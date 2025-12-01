package websocketmsg

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/logger"
	"chatroombackend/middleware"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境请加强校验
	},
}

type Client struct {
	Conn   *websocket.Conn
	UserID string
	Send   chan []byte
}

type Hub struct {
	Clients    map[string]*Client // userId -> client
	ClientsMux sync.RWMutex
	Rooms      map[string]map[string]bool // roomId -> set of userIds
	RoomsMux   sync.RWMutex
}

var hub = &Hub{
	Clients: make(map[string]*Client),
	Rooms:   make(map[string]map[string]bool),
}

var queries *sqlcdb.Queries

// SetQueries 注入 sqlc 生成的 Queries 对象
func SetQueries(q *sqlcdb.Queries) {
	queries = q
}

// WebSocket 消息结构
type WSMessage struct {
	Type   string          `json:"type"`
	Action string          `json:"action,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// HandleWebSocket Gin 路由处理函数
func HandleWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		logger.Warn("WebSocket", "Connection attempt without token")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
		return
	}
	// 校验 JWT
	claims, err := middleware.ParseToken(token)
	if err != nil {
		logger.Warn("WebSocket", fmt.Sprintf("Connection attempt with invalid token: %v", err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	userId := claims.UserID

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to upgrade connection for user %s", userId), err)
		return
	}

	logger.Info("WebSocket", fmt.Sprintf("User %s connected from %s", userId, c.ClientIP()))

	client := &Client{
		Conn:   conn,
		UserID: userId,
		Send:   make(chan []byte, 256),
	}

	hub.ClientsMux.Lock()
	hub.Clients[userId] = client
	hub.ClientsMux.Unlock()

	go client.writePump()

	// 标记为在线并订阅用户加入的房间（断线重连支持）
	if queries != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		// 将用户设置为在线
		if err := queries.SetUserOnline(ctx, userId); err != nil {
			logger.Error("WebSocket", fmt.Sprintf("Failed to set user %s online", userId), err)
		} else {
			logger.Info("WebSocket", fmt.Sprintf("User %s set to online", userId))
		}
		// 列出用户的聊天室并加入 hub.rooms
		rooms, err := queries.ListUserChatrooms(ctx, sqlcdb.ListUserChatroomsParams{UserID: userId, Limit: 1000, Offset: 0})
		if err == nil {
			logger.Info("WebSocket", fmt.Sprintf("User %s auto-joining %d rooms", userId, len(rooms)))
			for _, r := range rooms {
				hub.joinRoom(userId, r.RoomID)
				// 尝试增加聊天室在线计数（容错）
				if err := queries.IncrementChatroomOnlineCount(ctx, r.RoomID); err != nil {
					logger.Error("WebSocket", fmt.Sprintf("Failed to increment online count for room %s", r.RoomID), err)
				}
			}
		} else {
			logger.Error("WebSocket", fmt.Sprintf("Failed to list rooms for user %s", userId), err)
		}
	}

	client.readPump()

	logger.Info("WebSocket", fmt.Sprintf("User %s disconnecting", userId))

	hub.ClientsMux.Lock()
	delete(hub.Clients, userId)
	hub.ClientsMux.Unlock()

	// 断开连接，取消所有房间订阅并设置离线
	if queries != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		// 将用户设置为离线
		if err := queries.SetUserOffline(ctx, userId); err != nil {
			logger.Error("WebSocket", fmt.Sprintf("Failed to set user %s offline", userId), err)
		} else {
			logger.Info("WebSocket", fmt.Sprintf("User %s set to offline", userId))
		}
		// 从所有房间移除并减少在线计数
		hub.RoomsMux.Lock()
		roomCount := 0
		for roomID := range hub.Rooms {
			if _, ok := hub.Rooms[roomID][userId]; ok {
				delete(hub.Rooms[roomID], userId)
				roomCount++
				if err := queries.DecrementChatroomOnlineCount(ctx, roomID); err != nil {
					logger.Error("WebSocket", fmt.Sprintf("Failed to decrement online count for room %s", roomID), err)
				}
			}
		}
		hub.RoomsMux.Unlock()
		logger.Info("WebSocket", fmt.Sprintf("User %s removed from %d rooms", userId, roomCount))
	}
	_ = conn.Close()
	logger.Info("WebSocket", fmt.Sprintf("User %s connection closed", userId))
}

// hub: 加入与广播辅助
func (h *Hub) joinRoom(userID, roomID string) {
	h.RoomsMux.Lock()
	defer h.RoomsMux.Unlock()
	set, ok := h.Rooms[roomID]
	if !ok {
		set = make(map[string]bool)
		h.Rooms[roomID] = set
	}
	set[userID] = true
}

func (h *Hub) leaveRoom(userID, roomID string) {
	h.RoomsMux.Lock()
	defer h.RoomsMux.Unlock()
	if set, ok := h.Rooms[roomID]; ok {
		delete(set, userID)
		if len(set) == 0 {
			delete(h.Rooms, roomID)
		}
	}
}

func (h *Hub) broadcastRoom(roomID string, msg WSMessage) {
	h.RoomsMux.RLock()
	members, ok := h.Rooms[roomID]
	h.RoomsMux.RUnlock()
	if !ok {
		return
	}
	b, _ := json.Marshal(msg)
	for uid := range members {
		h.ClientsMux.RLock()
		if client, ok := h.Clients[uid]; ok {
			select {
			case client.Send <- b:
			default:
				// 如果发送通道阻塞，跳过
			}
		}
		h.ClientsMux.RUnlock()
	}
}

func (c *Client) readPump() {
	defer func() {
		logger.Debug("WebSocket", fmt.Sprintf("ReadPump ended for user %s", c.UserID))
		_ = c.Conn.Close()
	}()
	c.Conn.SetReadLimit(512 * 1024) // Increased to 512KB for media messages
	_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warn("WebSocket", fmt.Sprintf("Unexpected close for user %s: %v", c.UserID, err))
			} else {
				logger.Debug("WebSocket", fmt.Sprintf("User %s read error: %v", c.UserID, err))
			}
			break
		}
		logger.Debug("WebSocket", fmt.Sprintf("User %s received %d bytes", c.UserID, len(message)))
		c.handleMessage(message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		logger.Debug("WebSocket", fmt.Sprintf("WritePump ended for user %s", c.UserID))
		_ = c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				logger.Debug("WebSocket", fmt.Sprintf("Send channel closed for user %s", c.UserID))
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				logger.Error("WebSocket", fmt.Sprintf("Failed to get writer for user %s", c.UserID), err)
				return
			}
			_, _ = w.Write(message)

			// Add queued messages to the current message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				logger.Error("WebSocket", fmt.Sprintf("Failed to close writer for user %s", c.UserID), err)
				return
			}
			logger.Debug("WebSocket", fmt.Sprintf("Sent message to user %s (%d bytes)", c.UserID, len(message)))
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Debug("WebSocket", fmt.Sprintf("Ping failed for user %s: %v", c.UserID, err))
				return
			}
			logger.Debug("WebSocket", fmt.Sprintf("Sent ping to user %s", c.UserID))
		}
	}
}

func (c *Client) handleMessage(message []byte) {
	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		logger.Warn("WebSocket", fmt.Sprintf("Failed to parse message from user %s: %v", c.UserID, err))
		return
	}

	// Log received message
	logger.Info("WebSocket", fmt.Sprintf("Received message from user %s - Type: %s, Action: %s", c.UserID, msg.Type, msg.Action))

	// 心跳
	if msg.Type == "ping" {
		pong := WSMessage{Type: "pong"}
		b, _ := json.Marshal(pong)
		c.Send <- b
		return
	}

	// 处理聊天消息发送
	if msg.Type == "message" && msg.Action == "send" {
		c.handleSendMessage(msg)
		return
	}

	// 处理加入房间
	if msg.Type == "room" && msg.Action == "join" {
		c.handleJoinRoom(msg)
		return
	}

	// 处理离开房间
	if msg.Type == "room" && msg.Action == "leave" {
		c.handleLeaveRoom(msg)
		return
	}

	// 处理用户状态更新
	if msg.Type == "user_status" && msg.Action == "update" {
		c.handleUserStatusUpdate(msg)
		return
	}

	// 处理消息已读
	if msg.Type == "message" && msg.Action == "read" {
		c.handleMessageRead(msg)
		return
	}

	// 处理正在输入状态
	if msg.Type == "typing" {
		c.handleTypingStatus(msg)
		return
	}

	logger.Warn("WebSocket", fmt.Sprintf("Unknown message type/action from user %s: %s/%s", c.UserID, msg.Type, msg.Action))
}

// handleSendMessage 处理发送消息
func (c *Client) handleSendMessage(msg WSMessage) {
	// 解析 data
	var d struct {
		RoomID          string  `json:"roomId"`
		MessageType     string  `json:"messageType"`
		Text            string  `json:"text"`
		QuotedMessageID *string `json:"quotedMessageId,omitempty"`
		MediaURL        *string `json:"mediaUrl,omitempty"`
	}
	if err := json.Unmarshal(msg.Data, &d); err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to parse send message data from user %s", c.UserID), err)
		c.sendError("invalid_data", "Invalid message data")
		return
	}

	logger.Info("WebSocket", fmt.Sprintf("User %s sending message to room %s - Type: %s", c.UserID, d.RoomID, d.MessageType))

	// 必要的 db 依赖
	if queries == nil {
		logger.Error("WebSocket", "Database queries not initialized", nil)
		c.sendError("internal_error", "Database not available")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 检查用户是否在房间中
	inRoom, err := queries.IsUserInChatroom(ctx, sqlcdb.IsUserInChatroomParams{UserID: c.UserID, RoomID: d.RoomID})
	if err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Error checking user %s in room %s", c.UserID, d.RoomID), err)
		c.sendError("internal_error", "Failed to verify room membership")
		return
	}
	if !inRoom {
		logger.Warn("WebSocket", fmt.Sprintf("User %s not in room %s", c.UserID, d.RoomID))
		c.sendError("not_in_room", "You are not a member of this room")
		return
	}

	// 检查是否被禁言（全局或房间）
	canSend, err := queries.CanUserSendMessageInRoom(ctx, sqlcdb.CanUserSendMessageInRoomParams{MutedUserID: c.UserID, RoomID: d.RoomID})
	if err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Error checking mute status for user %s in room %s", c.UserID, d.RoomID), err)
		c.sendError("internal_error", "Failed to check permissions")
		return
	}
	if !canSend.Valid || !canSend.Bool {
		logger.Warn("WebSocket", fmt.Sprintf("User %s is muted in room %s", c.UserID, d.RoomID))
		c.sendError("muted", "You are muted and cannot send messages")
		return
	}

	// 构建 CreateMessageParams
	var quoted sql.NullString
	if d.QuotedMessageID != nil && *d.QuotedMessageID != "" {
		quoted = sql.NullString{String: *d.QuotedMessageID, Valid: true}
	}
	sender := sql.NullString{String: c.UserID, Valid: true}

	// 验证消息类型
	mt := sqlcdb.MessageType(d.MessageType)
	validTypes := []sqlcdb.MessageType{
		sqlcdb.MessageTypeText,
		sqlcdb.MessageTypeImage,
		sqlcdb.MessageTypeFile,
		sqlcdb.MessageTypeSystemNotification,
	}
	isValidType := false
	for _, vt := range validTypes {
		if mt == vt {
			isValidType = true
			break
		}
	}
	if !isValidType {
		logger.Warn("WebSocket", fmt.Sprintf("Invalid message type from user %s: %s", c.UserID, d.MessageType))
		c.sendError("invalid_type", "Invalid message type")
		return
	}

	// 对于非文本消息，需要 mediaUrl
	if mt != sqlcdb.MessageTypeText && mt != sqlcdb.MessageTypeSystemNotification {
		if d.MediaURL == nil || *d.MediaURL == "" {
			logger.Warn("WebSocket", fmt.Sprintf("Missing media URL for type %s from user %s", d.MessageType, c.UserID))
			c.sendError("missing_media", "Media URL required for this message type")
			return
		}
	}

	createParams := sqlcdb.CreateMessageParams{
		Content:         d.Text,
		MessageType:     mt,
		QuotedMessageID: quoted,
		SenderID:        sender,
		RoomID:          d.RoomID,
	}

	// 创建消息
	m, err := queries.CreateMessage(ctx, createParams)
	if err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to create message from user %s in room %s", c.UserID, d.RoomID), err)
		c.sendError("internal_error", "Failed to create message")
		return
	}

	logger.Info("WebSocket", fmt.Sprintf("Message created: %s from user %s in room %s", m.MessageID, c.UserID, d.RoomID))

	// 获取带发送者信息的消息（包含昵称等）
	mm, err := queries.GetMessageWithSender(ctx, m.MessageID)
	if err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to get message with sender for message %s", m.MessageID), err)
		// 仍然继续，使用基础信息
	}

	// 组装广播消息
	out := map[string]interface{}{
		"messageId": m.MessageID,
		"roomId":    m.RoomID,
		"userId":    c.UserID,
		"type":      string(m.MessageType),
		"text":      m.Content,
		"time":      m.SentAt.UTC().Format(time.RFC3339),
	}

	if err == nil {
		out["userName"] = chooseDisplayName(mm.Username, mm.Nickname)
		if mm.AvatarUrl.Valid {
			out["avatarUrl"] = mm.AvatarUrl.String
		}
	}

	if d.QuotedMessageID != nil && *d.QuotedMessageID != "" {
		out["quotedMessageId"] = *d.QuotedMessageID
	}

	if d.MediaURL != nil && *d.MediaURL != "" {
		out["mediaUrl"] = *d.MediaURL
	}

	outMsg := WSMessage{Type: "message", Action: "new"}
	b, _ := json.Marshal(out)
	outMsg.Data = b

	// 更新房间最后活跃时间（异步）
	go func(roomID string) {
		ctx := context.Background()
		if err := queries.UpdateChatroomLastActiveTime(ctx, roomID); err != nil {
			logger.Error("WebSocket", fmt.Sprintf("Failed to update last active time for room %s", roomID), err)
		}
	}(d.RoomID)

	// 广播到房间
	logger.Info("WebSocket", fmt.Sprintf("Broadcasting message %s to room %s", m.MessageID, d.RoomID))
	hub.broadcastRoom(d.RoomID, outMsg)
}

// handleJoinRoom 处理用户加入房间
func (c *Client) handleJoinRoom(msg WSMessage) {
	var d struct {
		RoomID string `json:"roomId"`
	}
	if err := json.Unmarshal(msg.Data, &d); err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to parse join room data from user %s", c.UserID), err)
		c.sendError("invalid_data", "Invalid room data")
		return
	}

	logger.Info("WebSocket", fmt.Sprintf("User %s joining room %s via WebSocket", c.UserID, d.RoomID))

	if queries == nil {
		c.sendError("internal_error", "Database not available")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 检查用户是否在房间中
	inRoom, err := queries.IsUserInChatroom(ctx, sqlcdb.IsUserInChatroomParams{UserID: c.UserID, RoomID: d.RoomID})
	if err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Error checking user %s in room %s", c.UserID, d.RoomID), err)
		c.sendError("internal_error", "Failed to verify room membership")
		return
	}
	if !inRoom {
		logger.Warn("WebSocket", fmt.Sprintf("User %s not member of room %s", c.UserID, d.RoomID))
		c.sendError("not_in_room", "You are not a member of this room")
		return
	}

	// 加入房间的 WebSocket 订阅
	hub.joinRoom(c.UserID, d.RoomID)

	// 增加房间在线计数
	if err := queries.IncrementChatroomOnlineCount(ctx, d.RoomID); err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to increment online count for room %s", d.RoomID), err)
	}

	logger.Info("WebSocket", fmt.Sprintf("User %s successfully joined room %s", c.UserID, d.RoomID))

	// 发送确认消息
	resp := WSMessage{
		Type:   "room",
		Action: "joined",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","success":true}`, d.RoomID)),
	}
	b, _ := json.Marshal(resp)
	c.Send <- b

	// 广播用户加入消息到房间（可选）
	joinNotice := WSMessage{
		Type:   "room_member",
		Action: "joined",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","userId":"%s"}`, d.RoomID, c.UserID)),
	}
	hub.broadcastRoom(d.RoomID, joinNotice)
}

// handleLeaveRoom 处理用户离开房间
func (c *Client) handleLeaveRoom(msg WSMessage) {
	var d struct {
		RoomID string `json:"roomId"`
	}
	if err := json.Unmarshal(msg.Data, &d); err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to parse leave room data from user %s", c.UserID), err)
		return
	}

	logger.Info("WebSocket", fmt.Sprintf("User %s leaving room %s via WebSocket", c.UserID, d.RoomID))

	hub.leaveRoom(c.UserID, d.RoomID)

	if queries != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// 减少房间在线计数
		if err := queries.DecrementChatroomOnlineCount(ctx, d.RoomID); err != nil {
			logger.Error("WebSocket", fmt.Sprintf("Failed to decrement online count for room %s", d.RoomID), err)
		}
	}

	logger.Info("WebSocket", fmt.Sprintf("User %s successfully left room %s", c.UserID, d.RoomID))

	// 发送确认消息
	resp := WSMessage{
		Type:   "room",
		Action: "left",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","success":true}`, d.RoomID)),
	}
	b, _ := json.Marshal(resp)
	c.Send <- b

	// 广播用户离开消息到房间（可选）
	leaveNotice := WSMessage{
		Type:   "room_member",
		Action: "left",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","userId":"%s"}`, d.RoomID, c.UserID)),
	}
	hub.broadcastRoom(d.RoomID, leaveNotice)
}

// handleUserStatusUpdate 处理用户状态更新
func (c *Client) handleUserStatusUpdate(msg WSMessage) {
	var d struct {
		Status string `json:"status"` // online, away, busy, offline
	}
	if err := json.Unmarshal(msg.Data, &d); err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to parse status update from user %s", c.UserID), err)
		return
	}

	logger.Info("WebSocket", fmt.Sprintf("User %s updating status to: %s", c.UserID, d.Status))

	if queries == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 更新用户状态
	var err error
	switch d.Status {
	case "online":
		err = queries.SetUserOnline(ctx, c.UserID)
	case "offline":
		err = queries.SetUserOffline(ctx, c.UserID)
	default:
		// 对于其他状态，使用通用更新（如果有的话）
		logger.Warn("WebSocket", fmt.Sprintf("Unknown status from user %s: %s", c.UserID, d.Status))
		return
	}

	if err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to update status for user %s", c.UserID), err)
		return
	}

	logger.Info("WebSocket", fmt.Sprintf("User %s status updated to %s", c.UserID, d.Status))

	// 广播状态更新到所有相关房间
	hub.RoomsMux.RLock()
	userRooms := []string{}
	for roomID, members := range hub.Rooms {
		if members[c.UserID] {
			userRooms = append(userRooms, roomID)
		}
	}
	hub.RoomsMux.RUnlock()

	statusMsg := WSMessage{
		Type:   "user_status",
		Action: "updated",
		Data:   json.RawMessage(fmt.Sprintf(`{"userId":"%s","status":"%s"}`, c.UserID, d.Status)),
	}

	for _, roomID := range userRooms {
		hub.broadcastRoom(roomID, statusMsg)
	}
}

// handleMessageRead 处理消息已读确认
func (c *Client) handleMessageRead(msg WSMessage) {
	var d struct {
		RoomID    string `json:"roomId"`
		MessageID string `json:"messageId"`
	}
	if err := json.Unmarshal(msg.Data, &d); err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to parse read confirmation from user %s", c.UserID), err)
		return
	}

	logger.Info("WebSocket", fmt.Sprintf("User %s marked message %s as read in room %s", c.UserID, d.MessageID, d.RoomID))

	// 这里可以更新数据库中的已读状态
	// 目前只记录日志
}

// handleTypingStatus 处理正在输入状态
func (c *Client) handleTypingStatus(msg WSMessage) {
	var d struct {
		RoomID string `json:"roomId"`
		Typing bool   `json:"typing"`
	}
	if err := json.Unmarshal(msg.Data, &d); err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to parse typing status from user %s", c.UserID), err)
		return
	}

	logger.Debug("WebSocket", fmt.Sprintf("User %s typing status in room %s: %v", c.UserID, d.RoomID, d.Typing))

	// 广播输入状态到房间
	typingMsg := WSMessage{
		Type:   "typing",
		Action: "status",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","userId":"%s","typing":%v}`, d.RoomID, c.UserID, d.Typing)),
	}
	hub.broadcastRoom(d.RoomID, typingMsg)
}

// sendError 发送错误消息给客户端
func (c *Client) sendError(action string, message string) {
	errMsg := WSMessage{
		Type:   "error",
		Action: action,
		Data:   json.RawMessage(fmt.Sprintf(`{"message":"%s"}`, message)),
	}
	b, _ := json.Marshal(errMsg)
	c.Send <- b
}

func chooseDisplayName(username sql.NullString, nickname sql.NullString) string {
	if nickname.Valid && nickname.String != "" {
		return nickname.String
	}
	if username.Valid {
		return username.String
	}
	return ""
}

// SendToUser 广播消息给指定用户
func SendToUser(userId string, msg WSMessage) {
	hub.ClientsMux.RLock()
	client, ok := hub.Clients[userId]
	hub.ClientsMux.RUnlock()
	if ok {
		b, _ := json.Marshal(msg)
		client.Send <- b
	}
}

// BroadcastToRoom 广播消息到指定聊天室（供外部调用）
func BroadcastToRoom(roomID string, msg WSMessage) {
	logger.Info("WebSocket", fmt.Sprintf("Broadcasting to room %s - Type: %s, Action: %s", roomID, msg.Type, msg.Action))
	hub.broadcastRoom(roomID, msg)
}

// SendSystemMessage 发送系统消息到指定房间
func SendSystemMessage(roomID string, content string) error {
	if queries == nil {
		return fmt.Errorf("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建系统消息
	createParams := sqlcdb.CreateMessageParams{
		Content:     content,
		MessageType: sqlcdb.MessageTypeSystemNotification,
		SenderID:    sql.NullString{Valid: false}, // 系统消息无发送者
		RoomID:      roomID,
	}

	m, err := queries.CreateMessage(ctx, createParams)
	if err != nil {
		logger.Error("WebSocket", fmt.Sprintf("Failed to create system message for room %s", roomID), err)
		return err
	}

	logger.Info("WebSocket", fmt.Sprintf("System message created: %s for room %s", m.MessageID, roomID))

	// 组装广播消息
	out := map[string]interface{}{
		"messageId": m.MessageID,
		"roomId":    m.RoomID,
		"type":      "system_notification",
		"text":      m.Content,
		"time":      m.SentAt.UTC().Format(time.RFC3339),
	}

	outMsg := WSMessage{Type: "message", Action: "new"}
	b, _ := json.Marshal(out)
	outMsg.Data = b

	// 广播到房间
	hub.broadcastRoom(roomID, outMsg)
	return nil
}

// NotifyRoomMemberChange 通知房间成员变化
func NotifyRoomMemberChange(roomID, userID, action string) {
	msg := WSMessage{
		Type:   "room_member",
		Action: action,
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","userId":"%s","timestamp":"%s"}`, roomID, userID, time.Now().UTC().Format(time.RFC3339))),
	}
	logger.Info("WebSocket", fmt.Sprintf("Notifying room %s of member %s action: %s", roomID, userID, action))
	hub.broadcastRoom(roomID, msg)
}

// NotifyUserKicked 通知用户被踢出房间
func NotifyUserKicked(userID, roomID, reason string) {
	msg := WSMessage{
		Type:   "room_member",
		Action: "kicked",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","reason":"%s","timestamp":"%s"}`, roomID, reason, time.Now().UTC().Format(time.RFC3339))),
	}
	logger.Info("WebSocket", fmt.Sprintf("Notifying user %s kicked from room %s", userID, roomID))
	SendToUser(userID, msg)

	// Also remove them from the room
	hub.leaveRoom(userID, roomID)

	// Decrease online count if they're connected
	if queries != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := queries.DecrementChatroomOnlineCount(ctx, roomID); err != nil {
			logger.Error("WebSocket", fmt.Sprintf("Failed to decrement online count for room %s", roomID), err)
		}
	}
}

// NotifyUserMuted 通知用户被禁言
func NotifyUserMuted(userID, roomID string, duration time.Duration) {
	msg := WSMessage{
		Type:   "mute",
		Action: "muted",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","duration":"%s","timestamp":"%s"}`, roomID, duration.String(), time.Now().UTC().Format(time.RFC3339))),
	}
	logger.Info("WebSocket", fmt.Sprintf("Notifying user %s muted in room %s for %s", userID, roomID, duration))
	SendToUser(userID, msg)
}

// NotifyUserUnmuted 通知用户解除禁言
func NotifyUserUnmuted(userID, roomID string) {
	msg := WSMessage{
		Type:   "mute",
		Action: "unmuted",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","timestamp":"%s"}`, roomID, time.Now().UTC().Format(time.RFC3339))),
	}
	logger.Info("WebSocket", fmt.Sprintf("Notifying user %s unmuted in room %s", userID, roomID))
	SendToUser(userID, msg)
}

// NotifyMessageDeleted 通知消息被删除
func NotifyMessageDeleted(roomID, messageID string) {
	msg := WSMessage{
		Type:   "message",
		Action: "deleted",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","messageId":"%s","timestamp":"%s"}`, roomID, messageID, time.Now().UTC().Format(time.RFC3339))),
	}
	logger.Info("WebSocket", fmt.Sprintf("Notifying room %s of deleted message %s", roomID, messageID))
	hub.broadcastRoom(roomID, msg)
}

// NotifyMessageEdited 通知消息被编辑
func NotifyMessageEdited(roomID, messageID, newContent string) {
	msg := WSMessage{
		Type:   "message",
		Action: "edited",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s","messageId":"%s","text":"%s","timestamp":"%s"}`, roomID, messageID, newContent, time.Now().UTC().Format(time.RFC3339))),
	}
	logger.Info("WebSocket", fmt.Sprintf("Notifying room %s of edited message %s", roomID, messageID))
	hub.broadcastRoom(roomID, msg)
}

// GetOnlineUsersInRoom 获取房间内在线用户列表
func GetOnlineUsersInRoom(roomID string) []string {
	hub.RoomsMux.RLock()
	defer hub.RoomsMux.RUnlock()

	users := []string{}
	if members, ok := hub.Rooms[roomID]; ok {
		for userID := range members {
			users = append(users, userID)
		}
	}
	return users
}

// GetOnlineUserCount 获取在线用户数
func GetOnlineUserCount() int {
	hub.ClientsMux.RLock()
	defer hub.ClientsMux.RUnlock()
	return len(hub.Clients)
}

// IsUserOnline 检查用户是否在线
func IsUserOnline(userID string) bool {
	hub.ClientsMux.RLock()
	defer hub.ClientsMux.RUnlock()
	_, ok := hub.Clients[userID]
	return ok
}

// NullString 创建 sql.NullString 辅助函数
func NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
