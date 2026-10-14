package message

import (
	"chatroombackend/db"
	"chatroombackend/events"
	"chatroombackend/service/direct"
	social "chatroombackend/service/social"
	"context"
	"database/sql"
	"time"
)

type Service struct {
	DB          *sql.DB
	Q           *db.Queries
	Direct      *direct.Service
	PublishRoom func(string, events.Event)
}

func New(database *sql.DB, q *db.Queries, d *direct.Service, p func(string, events.Event)) *Service {
	return &Service{database, q, d, p}
}

// Send is the common HTTP/WS entrypoint. Kind is derived by the server.
func (s *Service) Send(ctx context.Context, actor, room string, in direct.SendInput) (any, error) {
	kind, err := s.Q.ConversationKind(ctx, room)
	if err != nil {
		return nil, social.Failure(404, "RESOURCE_NOT_FOUND", "会话不可访问", nil)
	}
	if kind == "direct" {
		return s.Direct.Send(ctx, actor, room, in)
	}
	if in.Type != "text" && in.Type != "image" && in.Type != "file" {
		return nil, social.Failure(400, "INVALID_ARGUMENT", "消息类型无效", nil)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	q := s.Q.WithTx(tx)
	if _, err = q.LockGroupRoom(ctx, room); err != nil {
		return nil, social.Failure(404, "RESOURCE_NOT_FOUND", "聊天室不可访问", nil)
	}
	active, err := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: actor, RoomID: room})
	if err != nil || !active.IsActive {
		return nil, social.Failure(403, "FORBIDDEN", "不属于该聊天室", nil)
	}
	p, err := q.SocialProfile(ctx, actor)
	if err != nil {
		return nil, err
	}
	if p.AccountStatus != "active" {
		return nil, social.Failure(403, "ACCOUNT_UNAVAILABLE", "账号不可用", nil)
	}
	canSend, err := q.CanUserSendMessageInRoom(ctx, db.CanUserSendMessageInRoomParams{MutedUserID: actor, RoomID: room})
	if err != nil {
		return nil, err
	}
	if !canSend.Valid || !canSend.Bool {
		return nil, social.Failure(403, "GLOBALLY_MUTED", "暂不能发送消息", nil)
	}
	quote := sql.NullString{}
	if in.QuotedMessageID != "" {
		m, err := q.GetMessageByID(ctx, in.QuotedMessageID)
		if err != nil || m.RoomID != room {
			return nil, social.Failure(400, "INVALID_REFERENCE", "引用不可访问", nil)
		}
		quote = sql.NullString{String: in.QuotedMessageID, Valid: true}
	}
	m, err := q.CreateMessage(ctx, db.CreateMessageParams{Content: in.Text, MessageType: db.MessageType(in.Type), QuotedMessageID: quote, SenderID: sql.NullString{String: actor, Valid: true}, RoomID: room})
	if err != nil {
		return nil, err
	}
	if err = q.UpdateChatroomLastActiveTime(ctx, room); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	out := map[string]any{"messageId": m.MessageID, "roomId": room, "conversationKind": "group", "userId": actor, "userName": p.Nickname, "type": in.Type, "text": m.Content, "time": m.SentAt.UTC().Format(time.RFC3339Nano), "isOwn": true, "quotedMessageId": in.QuotedMessageID}
	if s.PublishRoom != nil {
		s.PublishRoom(room, events.Event{Type: "message", Action: "new", Data: out})
	}
	return out, nil
}
