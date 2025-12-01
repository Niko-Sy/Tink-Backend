package messages

import (
	"chatroombackend/api/websocketmsg"
	sqlcdb "chatroombackend/db"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HandleEditMessage 处理编辑消息请求 POST /chatrooms/:roomid/messages/:messageid/edit
func HandleEditMessage(c *gin.Context) {
	roomID := c.Param("roomid")
	messageID := c.Param("messageid")
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未授权"})
		return
	}

	var req struct {
		Text string `json:"text" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求参数错误", "error": err.Error()})
		return
	}

	queries, ok := c.MustGet("queries").(*sqlcdb.Queries)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "数据库查询对象获取失败"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// 验证用户是否在聊天室中
	inRoom, err := queries.IsUserInChatroom(ctx, sqlcdb.IsUserInChatroomParams{
		UserID: userID.(string),
		RoomID: roomID,
	})
	if err != nil || !inRoom {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "您不在该聊天室中"})
		return
	}

	// 获取原消息信息
	originalMsg, err := queries.GetMessageByID(ctx, messageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "消息不存在"})
		return
	}

	// 检查消息是否属于该聊天室
	if originalMsg.RoomID != roomID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "消息不属于该聊天室"})
		return
	}

	// 检查权限：是否是消息发送者或管理员
	isOwner := originalMsg.SenderID.Valid && originalMsg.SenderID.String == userID.(string)
	isAdmin := false

	if !isOwner {
		// 检查是否是管理员或房主
		memberInfo, err := queries.GetUserChatroomMembership(ctx, sqlcdb.GetUserChatroomMembershipParams{
			UserID: userID.(string),
			RoomID: roomID,
		})
		if err == nil && (memberInfo.MemberRole == sqlcdb.MemberRoleAdmin || memberInfo.MemberRole == sqlcdb.MemberRoleOwner) {
			isAdmin = true
		}
	}

	if !isOwner && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "您没有权限编辑此消息"})
		return
	}

	// 更新消息
	updatedMsg, err := queries.UpdateMessage(ctx, sqlcdb.UpdateMessageParams{
		MessageID: messageID,
		Content:   req.Text,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "消息编辑失败", "error": err.Error()})
		return
	}

	// 构建响应数据
	responseData := gin.H{
		"messageId": updatedMsg.MessageID,
		"text":      updatedMsg.Content,
		"isEdited":  true,
		"editedAt":  updatedMsg.SentAt.UTC().Format(time.RFC3339),
	}

	// 通过 WebSocket 广播消息编辑事件
	wsMsg := websocketmsg.WSMessage{
		Type:   "message",
		Action: "edit",
	}
	wsData, _ := json.Marshal(gin.H{
		"roomId":    roomID,
		"messageId": messageID,
		"newText":   req.Text,
		"editedAt":  updatedMsg.SentAt.UTC().Format(time.RFC3339),
	})
	wsMsg.Data = wsData
	websocketmsg.BroadcastToRoom(roomID, wsMsg)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "消息编辑成功",
		"data":    responseData,
	})
}
