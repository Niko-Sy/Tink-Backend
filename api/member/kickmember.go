package member

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

type KickRequest struct {
	MemberID string `json:"memberid" binding:"required"`
	Reason   string `json:"reason"`
}

// HandleKickRoomMember 管理员踢出成员
func HandleKickRoomMember(c *gin.Context) {
	roomID := c.Param("roomid")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "roomId required"})
		return
	}

	var req KickRequest
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

	// 执行踢出（设置 is_active = false, left_at = NOW()）
	if err := queries.KickMember(c.Request.Context(), sqlcdb.KickMemberParams{UserID: member.UserID, RoomID: roomID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "kick failed", "error": err.Error()})
		return
	}

	// 同步成员计数（减少）
	_ = queries.DecrementChatroomMemberCount(c.Request.Context(), roomID)

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "踢出成功"})
}
