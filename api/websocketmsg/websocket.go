package websocketmsg

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/events"
	"chatroombackend/internal/telemetry"
	"chatroombackend/logger"
	"chatroombackend/middleware"
	directservice "chatroombackend/service/direct"
	messagecore "chatroombackend/service/message"
	socialservice "chatroombackend/service/social"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: middleware.IsAllowedOrigin,
}

type Client struct {
	Conn     *websocket.Conn
	UserID   string
	Send     chan []byte
	Done     chan struct{}
	stopOnce sync.Once
	Messages *messagecore.Service
	Rooms    map[string]bool // protected by Hub.RoomsMux
}

func (c *Client) stop() {
	c.stopOnce.Do(func() {
		close(c.Done)
		if c.Conn != nil {
			_ = c.Conn.Close()
		}
	})
}
func (c *Client) enqueue(b []byte) {
	select {
	case <-c.Done:
		return
	default:
	}
	select {
	case c.Send <- b:
	case <-c.Done:
	default:
		telemetry.Record("hub/queue_overflow")
		c.stop()
	}
}

type Hub struct {
	Clients    map[string]map[*Client]bool
	ClientsMux sync.RWMutex
	Rooms      map[string]map[string]bool // aggregate unique subscribed users
	RoomsMux   sync.RWMutex
	Closing    bool
}

var hub = &Hub{Clients: map[string]map[*Client]bool{}, Rooms: map[string]map[string]bool{}}
var queries *sqlcdb.Queries

func SetQueries(q *sqlcdb.Queries) { queries = q }
func SetRuntime(q *sqlcdb.Queries, database *sql.DB) {
	if queries == q {
		return
	}
	queries = q
	// A second developer process must not reset the shared database's room counts.
}

type WSMessage struct {
	Type      string          `json:"type"`
	Action    string          `json:"action,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
}

func PublishSocialEvent(ids []string, event events.Event) {
	if queries == nil {
		return
	}
	telemetry.Record(event.Type + "/" + event.Action)
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, _ := json.Marshal(event.Data)
	msg := WSMessage{Type: event.Type, Action: event.Action, Data: data, Timestamp: event.Timestamp}
	var scope struct {
		RoomID string `json:"roomId"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(data, &scope)
	if event.Type == "invitation" && scope.Status != "" {
		telemetry.Record("invitation/status/" + scope.Status)
		if scope.Reason != "" {
			telemetry.Record("invitation/reason/" + scope.Reason)
		}
	}
	for _, id := range ids {
		hub.ClientsMux.RLock()
		clients := []*Client{}
		for c := range hub.Clients[id] {
			clients = append(clients, c)
		}
		hub.ClientsMux.RUnlock()
		for _, c := range clients {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			actor, err := queries.SocialProfile(ctx, id)
			cancel()
			if err != nil || actor.AccountStatus != "active" {
				continue
			}
			if (event.Type == "direct" || event.Type == "direct/read" || (event.Type == "message" && scope.RoomID != "")) && c.Messages != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				err := c.Messages.Direct.Access(ctx, id, scope.RoomID, false)
				cancel()
				if err != nil {
					continue
				}
				if event.Type == "direct" && event.Action == "created" {
					hub.subscribe(c, scope.RoomID)
				}
			}
			b, _ := json.Marshal(msg)
			c.enqueue(b)
		}
	}
}
func HandleWithMessages(core *messagecore.Service, tickets *TicketStore) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set("messageService", core)
		ctx.Set("tickets", tickets)
		HandleWebSocket(ctx)
	}
}
func HandleWebSocket(ctx *gin.Context) {
	if !middleware.IsAllowedOrigin(ctx.Request) {
		ctx.AbortWithStatusJSON(403, gin.H{"code": 403, "message": "来源不允许"})
		return
	}
	claims, err := ctx.MustGet("tickets").(*TicketStore).Consume(ctx.Query("ticket"))
	if err != nil {
		ctx.JSON(401, gin.H{"code": 401, "message": "无效登录凭证"})
		return
	}
	if queries == nil {
		ctx.JSON(503, gin.H{"code": 503, "message": "服务不可用"})
		return
	}
	actor, err := queries.SocialProfile(ctx.Request.Context(), claims.UserID)
	if err != nil || actor.AccountStatus != "active" {
		ctx.JSON(403, gin.H{"code": 403, "message": "账号不可用"})
		return
	}
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	core := ctx.MustGet("messageService").(*messagecore.Service)
	c := &Client{Messages: core, Conn: conn, UserID: claims.UserID, Send: make(chan []byte, 256), Done: make(chan struct{}), Rooms: map[string]bool{}}
	hub.ClientsMux.Lock()
	if hub.Closing {
		hub.ClientsMux.Unlock()
		c.stop()
		return
	}
	first := len(hub.Clients[c.UserID]) == 0
	if first {
		hub.Clients[c.UserID] = map[*Client]bool{}
	}
	hub.Clients[c.UserID][c] = true
	if first {
		callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = queries.SetUserOnline(callCtx, c.UserID)
		cancel()
	}
	hub.ClientsMux.Unlock()
	go c.writePump()
	go func() {
		timer := time.NewTimer(time.Until(claims.ExpiresAt.Time))
		defer timer.Stop()
		select {
		case <-timer.C:
			c.stop()
		case <-c.Done:
		}
	}()
	if first {
		PublishPresence(c.UserID)
	}
	callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	for offset := int32(0); ; offset += 100 {
		rooms, err := queries.ListUserChatrooms(callCtx, sqlcdb.ListUserChatroomsParams{UserID: c.UserID, Limit: 100, Offset: offset})
		if err != nil {
			break
		}
		for _, room := range rooms {
			hub.subscribe(c, room.RoomID)
		}
		if len(rooms) < 100 {
			break
		}
	}
	cancel()
	// Direct subscriptions are bound to the same immutable router service as the connection.
	cursor := ""
	for {
		list, err := core.Direct.List(ctx.Request.Context(), c.UserID, cursor, 100)
		if err != nil {
			break
		}
		for _, room := range list.List {
			hub.subscribe(c, room.RoomID)
		}
		if !list.HasMore {
			break
		}
		cursor = list.NextCursor
	}
	c.readPump()
	c.stop()
	hub.ClientsMux.Lock()
	delete(hub.Clients[c.UserID], c)
	last := len(hub.Clients[c.UserID]) == 0
	hub.RoomsMux.Lock()
	for room := range c.Rooms {
		hub.unsubscribeLocked(c, room)
	}
	hub.RoomsMux.Unlock()
	if last {
		delete(hub.Clients, c.UserID)
		callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = queries.SetUserOffline(callCtx, c.UserID)
		cancel()
	}
	hub.ClientsMux.Unlock()
	if last {
		PublishPresence(c.UserID)
	}
}
func (h *Hub) subscribe(c *Client, roomID string) {
	h.RoomsMux.Lock()
	defer h.RoomsMux.Unlock()
	select {
	case <-c.Done:
		return
	default:
	}
	if c.Rooms[roomID] {
		return
	}
	c.Rooms[roomID] = true
	if h.Rooms[roomID] == nil {
		h.Rooms[roomID] = map[string]bool{}
	}
	if !h.Rooms[roomID][c.UserID] {
		h.Rooms[roomID][c.UserID] = true
		if queries != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = queries.IncrementChatroomOnlineCount(ctx, roomID)
			cancel()
		}
	}
}

// Caller holds ClientsMux and RoomsMux. Never reverse this lock order.
func (h *Hub) unsubscribeLocked(c *Client, roomID string) {
	if !c.Rooms[roomID] {
		return
	}
	delete(c.Rooms, roomID)
	for other := range h.Clients[c.UserID] {
		if other != c && other.Rooms[roomID] {
			return
		}
	}
	delete(h.Rooms[roomID], c.UserID)
	if len(h.Rooms[roomID]) == 0 {
		delete(h.Rooms, roomID)
	}
	if queries != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = queries.DecrementChatroomOnlineCount(ctx, roomID)
		cancel()
	}
}
func (h *Hub) unsubscribe(c *Client, roomID string) {
	h.ClientsMux.RLock()
	defer h.ClientsMux.RUnlock()
	h.RoomsMux.Lock()
	defer h.RoomsMux.Unlock()
	h.unsubscribeLocked(c, roomID)
}

// Remove all subscriptions when the real membership is revoked.
func (h *Hub) leaveRoom(userID, roomID string) {
	h.ClientsMux.RLock()
	defer h.ClientsMux.RUnlock()
	h.RoomsMux.Lock()
	defer h.RoomsMux.Unlock()
	for c := range h.Clients[userID] {
		h.unsubscribeLocked(c, roomID)
	}
}
func (h *Hub) broadcastRoom(roomID string, msg WSMessage) {
	if queries == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := queries.LockFreeGroupRoom(ctx, roomID)
	if err != nil {
		return
	}
	h.ClientsMux.RLock()
	h.RoomsMux.RLock()
	clients := []*Client{}
	for uid := range h.Rooms[roomID] {
		for c := range h.Clients[uid] {
			if c.Rooms[roomID] {
				clients = append(clients, c)
			}
		}
	}
	h.RoomsMux.RUnlock()
	h.ClientsMux.RUnlock()
	b, _ := json.Marshal(msg)
	for _, c := range clients {
		actor, e := queries.SocialProfile(ctx, c.UserID)
		if e != nil || actor.AccountStatus != "active" {
			c.stop()
			continue
		}
		active, err := queries.IsUserInChatroom(ctx, sqlcdb.IsUserInChatroomParams{UserID: c.UserID, RoomID: roomID})
		if err == nil && active {
			c.enqueue(b)
		}
	}
}

// Presence is private to friends, and every recipient refetches public visibility.
func PublishPresence(userID string) {
	if queries == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	p, err := queries.SocialProfile(ctx, userID)
	if err != nil {
		return
	}
	public := socialservice.PublicProfile(p.UserID, p.Username, p.Nickname, p.Avatar, p.Signature, p.AccountStatus, p.OnlineStatus, p.ShowOnlineStatus)
	for offset := int32(0); ; offset += 100 {
		friends, err := queries.SocialFriendList(ctx, sqlcdb.SocialFriendListParams{ViewerID: userID, Search: "", StatusFilter: "all", PageLimit: 100, PageOffset: offset})
		if err != nil {
			return
		}
		for _, f := range friends {
			PublishSocialEvent([]string{f.UserID}, events.Event{Type: "friend_presence", Action: "updated", Data: public, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)})
		}
		if len(friends) < 100 {
			return
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		logger.Debug("WebSocket", fmt.Sprintf("ReadPump ended for user %s", c.UserID))
		c.stop()
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
		c.stop()
	}()
	for {
		select {
		case <-c.Done:
			return
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
		c.enqueue(b)
		return
	}

	if msg.Type == "room" || msg.Type == "message" || msg.Type == "typing" {
		var data struct {
			RoomID          string `json:"roomId"`
			ClientMessageID string `json:"clientMessageId"`
		}
		if json.Unmarshal(msg.Data, &data) != nil || data.RoomID == "" {
			c.sendError("invalid_data", "Invalid room")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		kind, err := c.Messages.Q.ConversationKind(ctx, data.RoomID)
		if err != nil {
			cancel()
			c.sendError("not_in_room", "Room inaccessible")
			return
		}
		if kind == "direct" {
			err = c.Messages.Direct.Access(ctx, c.UserID, data.RoomID, false)
			cancel()
			if err != nil {
				c.sendDirectError(err, data.RoomID, data.ClientMessageID)
				return
			}
			switch {
			case msg.Type == "message" && msg.Action == "send":
				c.handleSendMessage(msg)
			case msg.Type == "room" && msg.Action == "join":
				hub.subscribe(c, data.RoomID)
				c.respondRoom("joined", data.RoomID)
			case msg.Type == "room" && msg.Action == "leave":
				hub.unsubscribe(c, data.RoomID)
				c.respondRoom("left", data.RoomID)
			case msg.Type == "message" && msg.Action == "read":
				var d struct {
					MessageID string `json:"messageId"`
				}
				_ = json.Unmarshal(msg.Data, &d)
				if _, err = c.Messages.Direct.Read(context.Background(), c.UserID, data.RoomID, d.MessageID); err != nil {
					c.sendDirectError(err, data.RoomID, "")
				}
			default:
				c.sendError("unsupported_action", "Action unavailable")
			}
			return
		}
		active, memberErr := c.Messages.Q.IsUserInChatroom(ctx, sqlcdb.IsUserInChatroomParams{UserID: c.UserID, RoomID: data.RoomID})
		cancel()
		if memberErr != nil || kind != "group" || !active {
			c.sendError("not_in_room", "Room inaccessible")
			return
		}
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
func (c *Client) respondRoom(action, room string) {
	data, _ := json.Marshal(gin.H{"roomId": room, "success": true})
	b, _ := json.Marshal(WSMessage{Type: "room", Action: action, Data: data})
	c.enqueue(b)
}
func (c *Client) sendDirectError(err error, room, key string) {
	var e *socialservice.Error
	if !errors.As(err, &e) {
		e = socialservice.Failure(500, "INTERNAL_ERROR", "服务暂不可用", nil)
	}
	data, _ := json.Marshal(gin.H{"roomId": room, "clientMessageId": key, "code": e.Status, "businessCode": e.Code, "message": e.Message, "details": e.Details})
	b, _ := json.Marshal(WSMessage{Type: "error", Action: "send_failed", Data: data})
	c.enqueue(b)
}
func (c *Client) handleSendMessage(msg WSMessage) {
	var input struct {
		RoomID string `json:"roomId"`
		Type   string `json:"messageType"`
		Text   string `json:"text"`
		Key    string `json:"clientMessageId"`
		Quote  string `json:"quotedMessageId"`
	}
	if json.Unmarshal(msg.Data, &input) != nil {
		c.sendError("invalid_data", "Invalid message")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := c.Messages.Send(ctx, c.UserID, input.RoomID, directservice.SendInput{Type: input.Type, Text: input.Text, ClientMessageID: input.Key, QuotedMessageID: input.Quote})
	if err != nil {
		c.sendDirectError(err, input.RoomID, input.Key)
		return
	}
	if input.Key != "" {
		data, _ := json.Marshal(out)
		b, _ := json.Marshal(WSMessage{Type: "message", Action: "ack", Data: data})
		c.enqueue(b)
	}
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

	hub.subscribe(c, d.RoomID)
	data, _ := json.Marshal(gin.H{"roomId": d.RoomID, "success": true})
	b, _ := json.Marshal(WSMessage{Type: "room", Action: "joined", Data: data})
	c.enqueue(b)
}
func (c *Client) handleLeaveRoom(msg WSMessage) {
	var d struct {
		RoomID string `json:"roomId"`
	}
	if json.Unmarshal(msg.Data, &d) != nil {
		return
	}
	hub.unsubscribe(c, d.RoomID)
	data, _ := json.Marshal(gin.H{"roomId": d.RoomID, "success": true})
	b, _ := json.Marshal(WSMessage{Type: "room", Action: "left", Data: data})
	c.enqueue(b)
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

	PublishPresence(c.UserID)
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
		Data:   json.RawMessage(fmt.Sprintf(`{"userId":"%s","onlineStatus":"%s"}`, c.UserID, d.Status)),
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
	c.enqueue(b)
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
func SendToUser(userID string, msg WSMessage) {
	b, _ := json.Marshal(msg)
	hub.ClientsMux.RLock()
	clients := []*Client{}
	for c := range hub.Clients[userID] {
		clients = append(clients, c)
	}
	hub.ClientsMux.RUnlock()
	for _, c := range clients {
		c.enqueue(b)
	}
}

// BroadcastToRoom 广播消息到指定聊天室（供外部调用）
func BroadcastToRoom(roomID string, msg WSMessage) {
	logger.Info("WebSocket", fmt.Sprintf("Broadcasting to room %s - Type: %s, Action: %s", roomID, msg.Type, msg.Action))
	hub.broadcastRoom(roomID, msg)
}

// SendSystemMessage 发送系统消息到指定房间
// memberID: 可选，相关成员的member_rel_id（如禁言/解禁通知）
func SendSystemMessage(roomID string, content string, memberID ...string) error {
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
	// 如果提供了 memberID，添加到消息中
	if len(memberID) > 0 && memberID[0] != "" {
		out["memberId"] = memberID[0]
	}

	b, _ := json.Marshal(out)
	outMsg.Data = b

	// 广播到房间
	logger.Info("WebSocket", fmt.Sprintf("Broadcasting system message %s to room %s", m.MessageID, roomID))
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
	data, _ := json.Marshal(map[string]any{"userId": userID, "roomId": roomID, "reason": reason})
	SendToUser(userID, WSMessage{Type: "room_member", Action: "kicked", Data: data})
	hub.leaveRoom(userID, roomID)
}

// NotifyUserMuted 通知用户被禁言
// duration: 禁言时长，0 或负数表示永久禁言
func NotifyUserMuted(userID, roomID string, duration time.Duration) {
	var data string
	if duration <= 0 {
		// 永久禁言
		data = fmt.Sprintf(`{"roomId":"%s","duration":-1}`, roomID)
	} else {
		// 定时禁言：duration 为秒数，muteUntil 为结束时间
		durationSeconds := int64(duration.Seconds())
		muteUntil := time.Now().Add(duration).UTC().Format(time.RFC3339)
		data = fmt.Sprintf(`{"roomId":"%s","duration":%d,"muteUntil":"%s"}`, roomID, durationSeconds, muteUntil)
	}
	msg := WSMessage{
		Type:   "notification",
		Action: "muted",
		Data:   json.RawMessage(data),
	}
	logger.Info("WebSocket", fmt.Sprintf("Notifying user %s muted in room %s for %s", userID, roomID, duration))
	SendToUser(userID, msg)
}

// NotifyUserUnmuted 通知用户解除禁言
func NotifyUserUnmuted(userID, roomID string) {
	msg := WSMessage{
		Type:   "notification",
		Action: "unmuted",
		Data:   json.RawMessage(fmt.Sprintf(`{"roomId":"%s"}`, roomID)),
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

// MembershipJoined is called only after a real membership write succeeds.
func MembershipJoined(userID, roomID string) {
	hub.ClientsMux.RLock()
	clients := []*Client{}
	for c := range hub.Clients[userID] {
		clients = append(clients, c)
	}
	hub.ClientsMux.RUnlock()
	for _, c := range clients {
		hub.subscribe(c, roomID)
	}
	data, _ := json.Marshal(gin.H{"roomId": roomID, "userId": userID})
	BroadcastToRoom(roomID, WSMessage{Type: "room_member", Action: "joined", Data: data})
}
func MembershipLeft(userID, roomID string) {
	hub.leaveRoom(userID, roomID)
	data, _ := json.Marshal(gin.H{"roomId": roomID, "userId": userID})
	BroadcastToRoom(roomID, WSMessage{Type: "room_member", Action: "left", Data: data})
}

func PublishRoomEvent(room string, event events.Event) {
	data, _ := json.Marshal(event.Data)
	BroadcastToRoom(room, WSMessage{Type: event.Type, Action: event.Action, Data: data, Timestamp: event.Timestamp})
}

// PublishGroupEvent adapts committed domain results to membership subscriptions.
func PublishGroupEvent(ids []string, e events.Event) {
	var data struct {
		RoomID string `json:"roomId"`
		UserID string `json:"userId"`
		Reason string `json:"reason"`
	}
	b, _ := json.Marshal(e.Data)
	_ = json.Unmarshal(b, &data)
	if e.Type == "group_membership" {
		switch e.Action {
		case "joined":
			MembershipJoined(data.UserID, data.RoomID)
			PublishSocialEvent(ids, events.Event{Type: "room", Action: "added", Data: map[string]any{"roomId": data.RoomID}})
		case "kicked":
			NotifyUserKicked(data.UserID, data.RoomID, data.Reason)
			payload, _ := json.Marshal(map[string]any{"userId": data.UserID, "roomId": data.RoomID})
			BroadcastToRoom(data.RoomID, WSMessage{Type: "room_member", Action: "kicked", Data: payload})
			PublishSocialEvent(ids, events.Event{Type: "room", Action: "removed", Data: map[string]any{"roomId": data.RoomID}})
		case "left":
			MembershipLeft(data.UserID, data.RoomID)
			PublishSocialEvent(ids, events.Event{Type: "room", Action: "removed", Data: map[string]any{"roomId": data.RoomID}})
		case "deleted":
			for _, id := range ids {
				hub.leaveRoom(id, data.RoomID)
			}
			PublishSocialEvent(ids, events.Event{Type: "room", Action: "removed", Data: map[string]any{"roomId": data.RoomID}})
		}
		return
	}
	PublishSocialEvent(ids, e)
}

// Shutdown closes hijacked sockets, which http.Server.Shutdown does not manage.
// Wait until read pumps release subscriptions/presence before closing the DB.
func Shutdown(ctx context.Context) error {
	hub.ClientsMux.Lock()
	hub.Closing = true
	clients := []*Client{}
	for _, set := range hub.Clients {
		for c := range set {
			clients = append(clients, c)
		}
	}
	hub.ClientsMux.Unlock()
	for _, c := range clients {
		c.stop()
	}
	timer := time.NewTicker(20 * time.Millisecond)
	defer timer.Stop()
	for {
		hub.ClientsMux.RLock()
		n := len(hub.Clients)
		hub.ClientsMux.RUnlock()
		if n == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
}
