package member

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// MemberInfoResponse 成员信息响应
type MemberInfoResponse struct {
	MemberId   string     `json:"memberId"`
	RoomId     string     `json:"roomId"`
	UserId     string     `json:"userId"`
	RoomRole   string     `json:"roomRole"`
	IsMuted    bool       `json:"isMuted"`
	MuteUntil  *time.Time `json:"muteUntil"`
	JoinedAt   time.Time  `json:"joinedAt"`
	LastReadAt *time.Time `json:"lastReadAt"`
	IsActive   bool       `json:"isActive"`
}

// HandleGetRoomMemberInfo 获取用户在聊天室的成员信息
// GET /chatroom/:roomid/members/:userid/info
func HandleGetRoomMemberInfo(c *gin.Context) {
	roomId := c.Param("roomid")
	targetUserId := c.Param("userid")

	if roomId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "聊天室ID不能为空",
		})
		return
	}

	if targetUserId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "用户ID不能为空",
		})
		return
	}

	// 从JWT中间件获取当前用户ID
	currentUserID := c.GetString("userId")
	if currentUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录，请先登录获取Token",
		})
		return
	}

	// 获取数据库查询对象
	queries, err := middleware.GetQueriesFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取数据库连接失败",
			"error":   err.Error(),
		})
		return
	}

	// 检查聊天室是否存在
	_, err = queries.GetChatroomByID(c.Request.Context(), roomId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "聊天室不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询聊天室失败",
			"error":   err.Error(),
		})
		return
	}

	// 检查当前用户是否是聊天室成员（鉴权）
	isCurrentUserInRoom, err := queries.IsUserInChatroom(c.Request.Context(), sqlcdb.IsUserInChatroomParams{
		UserID: currentUserID,
		RoomID: roomId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "检查成员状态失败",
			"error":   err.Error(),
		})
		return
	}
	if !isCurrentUserInRoom {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "您不是该聊天室成员，无法查看成员信息",
		})
		return
	}

	// 获取目标用户在聊天室的成员信息
	membership, err := queries.GetUserChatroomMembership(c.Request.Context(), sqlcdb.GetUserChatroomMembershipParams{
		UserID: targetUserId,
		RoomID: roomId,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "该用户不是聊天室成员",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取成员信息失败",
			"error":   err.Error(),
		})
		return
	}

	// 检查成员是否处于活跃状态
	if !membership.IsActive {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "该用户已不是聊天室成员",
		})
		return
	}

	// 判断是否被禁言（考虑过期时间）
	isMuted := membership.MuteStatus == sqlcdb.MemberMuteStatusMuted
	if isMuted && membership.MuteExpiresAt.Valid && membership.MuteExpiresAt.Time.Before(time.Now()) {
		// 禁言已过期
		isMuted = false
	}

	// 构建响应
	response := MemberInfoResponse{
		MemberId: membership.MemberRelID,
		RoomId:   membership.RoomID,
		UserId:   membership.UserID,
		RoomRole: string(membership.MemberRole),
		IsMuted:  isMuted,
		MuteUntil: func() *time.Time {
			if membership.MuteExpiresAt.Valid && isMuted {
				return &membership.MuteExpiresAt.Time
			}
			return nil
		}(),
		JoinedAt: membership.JoinedAt,
		LastReadAt: func() *time.Time {
			if membership.LastReadAt.Valid {
				return &membership.LastReadAt.Time
			}
			return nil
		}(),
		IsActive: membership.IsActive,
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": response,
	})
}
