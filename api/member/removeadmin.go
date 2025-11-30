package member

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RemoveAdminRequest struct {
	MemberID string `json:"memberid" binding:"required"`
}

// HandleRemoveAdminRoomMember 房主取消管理员权限
func HandleRemoveAdminRoomMember(c *gin.Context) {
	roomID := c.Param("roomid")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "roomId required"})
		return
	}

	var req RemoveAdminRequest
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

	// 权限检查：必须是房主
	isOwner, err := queries.IsUserOwner(c.Request.Context(), sqlcdb.IsUserOwnerParams{UserID: currentUser, RoomID: roomID})
	if err != nil || !isOwner {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "only owner can remove admin"})
		return
	}

	member, err := queries.GetMemberByRelID(c.Request.Context(), req.MemberID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "member not found", "error": err.Error()})
		return
	}
	if member.RoomID != roomID {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "member not in this room"})
		return
	}

	if err := queries.RemoveMemberAdmin(c.Request.Context(), sqlcdb.RemoveMemberAdminParams{UserID: member.UserID, RoomID: roomID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "remove admin failed", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "移除管理员权限成功"})
}
