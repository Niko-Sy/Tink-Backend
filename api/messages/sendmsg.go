package messages

import (
	"chatroombackend/api/websocketmsg"
	sqlcdb "chatroombackend/db"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HandleSendMessage 处理发送消息请求 POST /chatrooms/:roomid/messages
func HandleSendMessage(c *gin.Context) {
	roomID := c.Param("roomid")
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未授权"})
		return
	}

	var req struct {
		Type             string  `json:"type" binding:"required"` // text, image, file
		Text             string  `json:"text" binding:"required"`
		ReplyToMessageID *string `json:"replyToMessageId,omitempty"`
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

	// 检查是否被禁言
	canSend, err := queries.CanUserSendMessageInRoom(ctx, sqlcdb.CanUserSendMessageInRoomParams{
		MutedUserID: userID.(string),
		RoomID:      roomID,
	})
	if err != nil || !canSend.Valid || !canSend.Bool {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "您已被禁言，无法发送消息"})
		return
	}

	// 构建消息参数
	var quotedMsgID sql.NullString
	if req.ReplyToMessageID != nil && *req.ReplyToMessageID != "" {
		quotedMsgID = sql.NullString{String: *req.ReplyToMessageID, Valid: true}
	}

	msgType := sqlcdb.MessageType(req.Type)
	senderID := sql.NullString{String: userID.(string), Valid: true}

	// 创建消息
	message, err := queries.CreateMessage(ctx, sqlcdb.CreateMessageParams{
		Content:         req.Text,
		MessageType:     msgType,
		QuotedMessageID: quotedMsgID,
		SenderID:        senderID,
		RoomID:          roomID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "消息发送失败", "error": err.Error()})
		return
	}

	// 获取带发送者信息的消息
	msgWithSender, err := queries.GetMessageWithSender(ctx, message.MessageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取消息信息失败"})
		return
	}

	// 构建响应数据
	userName := msgWithSender.Nickname.String
	if userName == "" && msgWithSender.Username.Valid {
		userName = msgWithSender.Username.String
	}

	responseData := gin.H{
		"messageId": msgWithSender.MessageID,
		"roomId":    msgWithSender.RoomID,
		"userId":    msgWithSender.SenderID.String,
		"userName":  userName,
		"type":      string(msgWithSender.MessageType),
		"text":      msgWithSender.Content,
		"time":      msgWithSender.SentAt.UTC().Format(time.RFC3339),
		"isOwn":     true,
	}

	// 通过 WebSocket 广播消息
	wsMsg := websocketmsg.WSMessage{
		Type:   "message",
		Action: "new",
	}
	wsData, _ := json.Marshal(gin.H{
		"messageId": msgWithSender.MessageID,
		"roomId":    msgWithSender.RoomID,
		"userId":    msgWithSender.SenderID.String,
		"userName":  userName,
		"type":      string(msgWithSender.MessageType),
		"text":      msgWithSender.Content,
		"time":      msgWithSender.SentAt.UTC().Format(time.RFC3339),
	})
	wsMsg.Data = wsData
	websocketmsg.BroadcastToRoom(roomID, wsMsg)

	// 异步更新房间最后活跃时间
	go func(roomID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = queries.UpdateChatroomLastActiveTime(ctx, roomID)
	}(roomID)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "消息发送成功",
		"data":    responseData,
	})
}
