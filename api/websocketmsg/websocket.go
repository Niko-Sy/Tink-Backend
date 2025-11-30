package websocketmsg

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"context"
	"database/sql"
	"encoding/json"
	"log"
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
// 这里只定义通用结构，具体 type/action/data 由协议决定

type WSMessage struct {
	Type   string          `json:"type"`
	Action string          `json:"action,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// Gin 路由处理函数
func HandleWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
		return
	}
	// 校验 JWT
	claims, err := middleware.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	userId := claims.UserID

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
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
		_ = queries.SetUserOnline(c.Request.Context(), userId)
		// 列出用户的聊天室并加入 hub.rooms
		rooms, err := queries.ListUserChatrooms(ctx, sqlcdb.ListUserChatroomsParams{UserID: userId, Limit: 1000, Offset: 0})
		if err == nil {
			for _, r := range rooms {
				hub.joinRoom(userId, r.RoomID)
				// 尝试增加聊天室在线计数（容错）
				_ = queries.IncrementChatroomOnlineCount(ctx, r.RoomID)
			}
		}
	}

	client.readPump()

	hub.ClientsMux.Lock()
	delete(hub.Clients, userId)
	hub.ClientsMux.Unlock()
	// 断开连接，取消所有房间订阅并设置离线
	if queries != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		// 将用户设置为离线
		_ = queries.SetUserOffline(ctx, userId)
		// 从所有房间移除并减少在线计数
		hub.RoomsMux.Lock()
		for roomID := range hub.Rooms {
			if _, ok := hub.Rooms[roomID][userId]; ok {
				delete(hub.Rooms[roomID], userId)
				_ = queries.DecrementChatroomOnlineCount(ctx, roomID)
			}
		}
		hub.RoomsMux.Unlock()
	}
	conn.Close()
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
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}
		c.handleMessage(message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			w.Close()
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(message []byte) {
	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}
	// 心跳
	if msg.Type == "ping" {
		pong := WSMessage{Type: "pong"}
		b, _ := json.Marshal(pong)
		c.Send <- b
		return
	}
	// 其他类型消息处理
	if msg.Type == "message" && msg.Action == "send" {
		// 解析 data
		var d struct {
			RoomID        string  `json:"roomId"`
			MessageType   string  `json:"messageType"`
			Text          string  `json:"text"`
			QuotedMessage *string `json:"quotedMessageId,omitempty"`
		}
		if err := json.Unmarshal(msg.Data, &d); err != nil {
			return
		}

		// 必要的 db 依赖
		if queries == nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// 检查用户是否在房间中
		inRoom, err := queries.IsUserInChatroom(ctx, sqlcdb.IsUserInChatroomParams{UserID: c.UserID, RoomID: d.RoomID})
		if err != nil || !inRoom {
			// 发送错误提示（简化）
			errMsg := WSMessage{Type: "error", Action: "not_in_room", Data: json.RawMessage([]byte(`{"message":"not in room"}`))}
			b, _ := json.Marshal(errMsg)
			c.Send <- b
			return
		}

		// 检查是否被禁言（全局或房间）
		canSend, err := queries.CanUserSendMessageInRoom(ctx, sqlcdb.CanUserSendMessageInRoomParams{MutedUserID: c.UserID, RoomID: d.RoomID})
		if err != nil || !canSend.Valid || !canSend.Bool {
			errMsg := WSMessage{Type: "error", Action: "muted", Data: json.RawMessage([]byte(`{"message":"muted"}`))}
			b, _ := json.Marshal(errMsg)
			c.Send <- b
			return
		}

		// 构建 CreateMessageParams
		var quoted sql.NullString
		if d.QuotedMessage != nil && *d.QuotedMessage != "" {
			quoted = sql.NullString{String: *d.QuotedMessage, Valid: true}
		}
		sender := sql.NullString{String: c.UserID, Valid: true}

		mt := sqlcdb.MessageType(d.MessageType)

		createParams := sqlcdb.CreateMessageParams{
			Content:         d.Text,
			MessageType:     mt,
			QuotedMessageID: quoted,
			SenderID:        sender,
			RoomID:          d.RoomID,
		}

		m, err := queries.CreateMessage(ctx, createParams)
		if err != nil {
			log.Printf("CreateMessage error: %v", err)
			return
		}

		// 获取带发送者信息的消息（包含昵称等）
		mm, err := queries.GetMessageWithSender(ctx, m.MessageID)
		if err != nil {
			log.Printf("GetMessageWithSender error: %v", err)
		}

		// 组装广播消息
		out := map[string]interface{}{
			"messageId": mm.MessageID,
			"roomId":    mm.RoomID,
			"userId":    mm.SenderID.String,
			"userName":  chooseDisplayName(mm.Username, mm.Nickname),
			"type":      mm.MessageType,
			"text":      mm.Content,
			"time":      mm.SentAt.UTC().Format(time.RFC3339),
		}

		outMsg := WSMessage{Type: "message", Action: "new"}
		b, _ := json.Marshal(out)
		outMsg.Data = b

		// 更新房间最后活跃时间（异步）
		go func(roomID string) {
			_ = queries.UpdateChatroomLastActiveTime(context.Background(), roomID)
		}(d.RoomID)

		// 广播到房间
		hub.broadcastRoom(d.RoomID, outMsg)
		return
	}

	// TODO: 处理其他类型（user_status, room_member, mute, ...）
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

// 广播消息给指定用户
func SendToUser(userId string, msg WSMessage) {
	hub.ClientsMux.RLock()
	client, ok := hub.Clients[userId]
	hub.ClientsMux.RUnlock()
	if ok {
		b, _ := json.Marshal(msg)
		client.Send <- b
	}
}
