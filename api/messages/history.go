package messages

import (
	sqlcdb "chatroombackend/db"
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// HandleGetMessageHistory 获取聊天室消息历史 GET /chatrooms/:roomid/messages
//
// 分页设计说明：
// 1. 传统分页（?page=1&pageSize=50）：
//   - page=1 返回最新的 50 条消息（降序：最新 -> 较早）
//   - page=2 返回接下来 50 条更早的消息
//   - 适用于首次加载和跳页查看
//
// 2. 游标分页（?before=messageId&pageSize=50）：
//   - 返回指定消息之前（更早）的消息
//   - 适用于"向上滚动加载更多历史消息"场景
//   - 避免了传统分页在有新消息插入时的数据偏移问题
//
// 前端使用建议：
// - 首次加载：GET /messages?page=1&pageSize=50
// - 向上滚动加载历史：GET /messages?before=<最早消息ID>&pageSize=50
// - 前端需要将返回的消息列表反转显示（最早的在上，最新的在下）
func HandleGetMessageHistory(c *gin.Context) {
	roomID := c.Param("roomid")
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未授权"})
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

	// 解析查询参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "50")
	beforeMsgID := c.Query("before") // 游标分页：获取此消息之前的历史消息

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	var messages []sqlcdb.GetMessagesByRoomRow
	var total int64

	// 如果指定了 before 参数，使用游标分页（推荐用于加载历史消息）
	if beforeMsgID != "" {
		beforeMessages, err := queries.GetMessagesBefore(ctx, sqlcdb.GetMessagesBeforeParams{
			Column1: sql.NullString{String: roomID, Valid: true},
			Column2: sql.NullString{String: beforeMsgID, Valid: true},
			Column3: sql.NullInt64{Int64: int64(pageSize), Valid: true},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取消息失败", "error": err.Error()})
			return
		}

		// 转换类型
		for _, msg := range beforeMessages {
			messages = append(messages, sqlcdb.GetMessagesByRoomRow{
				MessageID:       msg.MessageID,
				SentAt:          msg.SentAt,
				Content:         msg.Content,
				MessageType:     msg.MessageType,
				QuotedMessageID: msg.QuotedMessageID,
				SenderID:        msg.SenderID,
				RoomID:          msg.RoomID,
				Username:        msg.Username,
				Nickname:        msg.Nickname,
				AvatarUrl:       msg.AvatarUrl,
			})
		}
	} else {
		// 使用传统分页（page=1 返回最新消息）
		offset := int64((page - 1) * pageSize)
		messages, err = queries.GetMessagesByRoom(ctx, sqlcdb.GetMessagesByRoomParams{
			RoomID: roomID,
			Limit:  int64(pageSize),
			Offset: offset,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取消息失败", "error": err.Error()})
			return
		}
	}

	// 获取总消息数
	total, err = queries.CountMessagesInRoom(ctx, roomID)
	if err != nil {
		total = 0
	}

	// 构建响应数据
	messageList := make([]gin.H, 0, len(messages))
	for _, msg := range messages {
		userName := msg.Nickname.String
		if userName == "" && msg.Username.Valid {
			userName = msg.Username.String
		}

		isOwn := false
		if msg.SenderID.Valid && msg.SenderID.String == userID.(string) {
			isOwn = true
		}

		messageData := gin.H{
			"messageId": msg.MessageID,
			"roomId":    msg.RoomID,
			"userId":    msg.SenderID.String,
			"userName":  userName,
			"type":      string(msg.MessageType),
			"text":      msg.Content,
			"time":      msg.SentAt.UTC().Format(time.RFC3339),
			"isOwn":     isOwn,
			"isEdited":  false,
			"editedAt":  nil,
		}

		if msg.QuotedMessageID.Valid && msg.QuotedMessageID.String != "" {
			messageData["replyToMessageId"] = msg.QuotedMessageID.String
		}

		messageList = append(messageList, messageData)
	}

	hasMore := false
	if beforeMsgID == "" {
		hasMore = int64(page*pageSize) < total
	} else {
		hasMore = len(messages) >= pageSize
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"messages": messageList,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
			"hasMore":  hasMore,
		},
	})
}
