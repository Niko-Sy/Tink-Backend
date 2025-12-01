package member

import (
	"chatroombackend/api/websocketmsg"
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type MuteRequest struct {
	MemberID string `json:"memberid" binding:"required"`
	Duration int64  `json:"duration"` // seconds, -1 表示永久
	Reason   string `json:"reason"`
}

// HandleMuteRoomMember 管理员禁言成员
func HandleMuteRoomMember(c *gin.Context) {
	roomID := c.Param("roomid")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "roomId required"})
		return
	}

	var req MuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request", "error": err.Error()})
		return
	}

	currentUser := c.GetString("userId")
	if currentUser == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized"})
		return
	}

	queries, err := middleware.GetQueriesFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "db error", "error": err.Error()})
		return
	}

	// 检查房间存在
	if _, err := queries.GetChatroomByID(c.Request.Context(), roomID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "chatroom not found"})
		return
	}

	// 验证当前用户存在（外键约束需要）
	_, err = queries.GetUserByID(c.Request.Context(), currentUser)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "current user not found in database"})
		return
	}

	// 权限检查：必须是管理员或房主
	isAdmin, err := queries.IsUserAdminOrOwner(c.Request.Context(), sqlcdb.IsUserAdminOrOwnerParams{UserID: currentUser, RoomID: roomID})
	if err != nil || !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "permission denied"})
		return
	}

	// 获取 member 关系
	member, err := queries.GetMemberByRelID(c.Request.Context(), req.MemberID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "member not found", "error": err.Error()})
		return
	}
	if member.RoomID != roomID {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "member not in this room"})
		return
	}

	// 计算到期时间
	var expires sql.NullTime
	if req.Duration < 0 {
		// 永久禁言，保持 expires NULL 表示永久
		expires = sql.NullTime{Valid: false}
	} else if req.Duration == 0 {
		// duration 0 表示立即解除（不合理），视为错误
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid duration"})
		return
	} else {
		t := time.Now().Add(time.Duration(req.Duration) * time.Second)
		expires = sql.NullTime{Time: t, Valid: true}
	}

	// 更新 chatroom_members 表 mute_status
	if err := queries.MuteMember(c.Request.Context(), sqlcdb.MuteMemberParams{UserID: member.UserID, RoomID: roomID, MuteExpiresAt: expires}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "mute failed", "error": err.Error()})
		return
	}

	// 记录 mute_records（admin_id 是用户ID）
	if _, err := queries.CreateMuteRecord(c.Request.Context(), sqlcdb.CreateMuteRecordParams{
		MemberRelID: member.MemberRelID,
		ExpiresAt:   time.Now().Add(time.Duration(req.Duration) * time.Second),
		Reason:      sql.NullString{String: req.Reason, Valid: req.Reason != ""},
		AdminID:     sql.NullString{String: currentUser, Valid: true},
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "create mute record failed", "error": err.Error()})
		return
	}

	// 响应 muteUntil
	var muteUntil *time.Time
	if expires.Valid {
		muteUntil = &expires.Time
	}

	// WebSocket 通知: 通知被禁言用户
	var duration time.Duration
	if req.Duration < 0 {
		duration = 0 // 永久禁言
	} else {
		duration = time.Duration(req.Duration) * time.Second
	}
	websocketmsg.NotifyUserMuted(member.UserID, roomID, duration)

	// 获取被禁言用户的昵称用于系统消息
	mutedUser, err := queries.GetUserByID(c.Request.Context(), member.UserID)
	var displayName string
	if err == nil && mutedUser.Nickname.Valid && mutedUser.Nickname.String != "" {
		displayName = mutedUser.Nickname.String
	} else if err == nil {
		displayName = mutedUser.Username
	} else {
		displayName = "用户"
	}

	// WebSocket 通知: 向聊天室广播禁言消息
	var systemMsg string
	if req.Duration < 0 {
		systemMsg = fmt.Sprintf("%s已被永久禁言", displayName)
	} else if req.Duration > 0 {
		// 转换为分钟显示（如果大于等于60秒）
		if req.Duration >= 60 {
			minutes := req.Duration / 60
			systemMsg = fmt.Sprintf("%s已被禁言%d分钟", displayName, minutes)
		} else {
			systemMsg = fmt.Sprintf("%s已被禁言%d秒", displayName, req.Duration)
		}
	}
	if systemMsg != "" {
		_ = websocketmsg.SendSystemMessage(roomID, systemMsg, member.MemberRelID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "禁言成功", "data": gin.H{"muteUntil": muteUntil}})
}
