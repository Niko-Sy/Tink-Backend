package member

import (
	"chatroombackend/api/websocketmsg"
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UnmuteRequest struct {
	MemberID string `json:"memberid" binding:"required"`
}

// HandleUnmuteRoomMember 解除禁言（管理员）
func HandleUnmuteRoomMember(c *gin.Context) {
	roomID := c.Param("roomid")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "roomId required",
		})
		return
	}

	var req UnmuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request",
			"error":   err.Error(),
		})
		return
	}

	currentUser := c.GetString("userId")
	if currentUser == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "unauthorized",
		})
		return
	}

	queries, err := middleware.GetQueriesFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "db error",
			"error":   err.Error(),
		})
		return
	}

	// 检查房间存在
	if _, err := queries.GetChatroomByID(c.Request.Context(), roomID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "chatroom not found",
		})
		return
	}

	// 权限检查：必须是管理员或房主
	isAdmin, err := queries.IsUserAdminOrOwner(c.Request.Context(), sqlcdb.IsUserAdminOrOwnerParams{UserID: currentUser, RoomID: roomID})
	if err != nil || !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "permission denied",
		})
		return
	}

	// 获取 member 关系
	member, err := queries.GetMemberByRelID(c.Request.Context(), req.MemberID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "member not found",
			"error":   err.Error(),
		})
		return
	}
	if member.RoomID != roomID {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "member not in this room",
		})
		return
	}

	// 解除 chatroom_members 中的禁言状态
	if err := queries.UnmuteMember(c.Request.Context(), sqlcdb.UnmuteMemberParams{UserID: member.UserID, RoomID: roomID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "unmute failed",
			"error":   err.Error(),
		})
		return
	}

	// 使相关的 mute_records 失效
	if err := queries.DeactivateMuteRecord(c.Request.Context(), member.MemberRelID); err != nil {
		// 记录失败不阻断流程
	}

	// WebSocket 通知: 通知被解禁用户
	websocketmsg.NotifyUserUnmuted(member.UserID, roomID)

	// 获取被解禁用户的昵称用于系统消息
	unmutedUser, err := queries.GetUserByID(c.Request.Context(), member.UserID)
	var displayName string
	if err == nil && unmutedUser.Nickname.Valid && unmutedUser.Nickname.String != "" {
		displayName = unmutedUser.Nickname.String
	} else if err == nil {
		displayName = unmutedUser.Username
	} else {
		displayName = "用户"
	}

	// WebSocket 通知: 向聊天室广播解禁消息
	_ = websocketmsg.SendSystemMessage(roomID, fmt.Sprintf("%s已被解除禁言", displayName), member.MemberRelID)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "解除禁言成功",
	})
}
