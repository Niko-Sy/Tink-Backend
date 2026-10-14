package direct

import (
	"chatroombackend/db"
	"chatroombackend/events"
	social "chatroombackend/service/social"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var roomPattern = regexp.MustCompile(`^[0-9]{9}$`)
var userPattern = regexp.MustCompile(`^U[0-9]{9}$`)

type Service struct {
	DB      *sql.DB
	Q       *db.Queries
	Enabled bool
	Publish events.Publisher
	mu      sync.Mutex
	limits  map[string]bucket
}
type bucket struct {
	At    time.Time
	Count int
}

func New(database *sql.DB, q *db.Queries, enabled bool, p events.Publisher) *Service {
	return &Service{DB: database, Q: q, Enabled: enabled, Publish: p, limits: map[string]bucket{}}
}

type Quote struct {
	MessageID string         `json:"messageId"`
	Sender    social.Profile `json:"sender"`
	Text      string         `json:"text"`
	IsDeleted bool           `json:"isDeleted"`
}
type Message struct {
	MessageID        string         `json:"messageId"`
	RoomID           string         `json:"roomId"`
	ConversationKind string         `json:"conversationKind"`
	ClientMessageID  string         `json:"clientMessageId"`
	DirectSeq        string         `json:"directSeq"`
	Revision         string         `json:"revision"`
	UserID           string         `json:"userId"`
	Sender           social.Profile `json:"sender"`
	Type             string         `json:"type"`
	Text             string         `json:"text"`
	Time             time.Time      `json:"time"`
	QuotedMessageID  *string        `json:"quotedMessageId"`
	Quote            *Quote         `json:"quote"`
	EditedAt         *time.Time     `json:"editedAt"`
	DeletedAt        *time.Time     `json:"deletedAt"`
	IsDeleted        bool           `json:"isDeleted"`
}
type Conversation struct {
	RoomID           string         `json:"roomId"`
	ConversationKind string         `json:"conversationKind"`
	Peer             social.Profile `json:"peer"`
	CreatedAt        time.Time      `json:"createdAt"`
	LastActivityAt   time.Time      `json:"lastActivityAt"`
	LastMessage      *Message       `json:"lastMessage"`
	UnreadCount      int64          `json:"unreadCount"`
	LastReadSeq      string         `json:"lastReadSeq"`
	LatestSeq        string         `json:"latestSeq"`
	ChangeRevision   string         `json:"changeRevision"`
	CanSend          bool           `json:"canSend"`
	ReadOnlyReason   *string        `json:"readOnlyReason"`
}
type CreateResult struct {
	Conversation Conversation `json:"conversation"`
	Created      bool         `json:"created"`
}
type SendInput struct {
	Type            string `json:"type"`
	Text            string `json:"text"`
	ClientMessageID string `json:"clientMessageId"`
	QuotedMessageID string `json:"quotedMessageId,omitempty"`
}
type History struct {
	Messages       []Message `json:"messages"`
	HasMore        bool      `json:"hasMore"`
	NextCursor     string    `json:"nextCursor"`
	LatestSeq      string    `json:"latestSeq"`
	ChangeRevision string    `json:"changeRevision"`
}
type Change struct {
	Revision string  `json:"revision"`
	Action   string  `json:"action"`
	Message  Message `json:"message"`
}
type Changes struct {
	Changes      []Change `json:"changes"`
	HasMore      bool     `json:"hasMore"`
	NextCursor   string   `json:"nextCursor"`
	UpToRevision string   `json:"upToRevision"`
}
type ReadResult struct {
	RoomID      string `json:"roomId"`
	LastReadSeq string `json:"lastReadSeq"`
	UnreadCount int64  `json:"unreadCount"`
}
type List struct {
	List       []Conversation `json:"list"`
	HasMore    bool           `json:"hasMore"`
	NextCursor string         `json:"nextCursor"`
}

func fail(status int, code, msg string) *social.Error { return social.Failure(status, code, msg, nil) }
func (s *Service) tx(ctx context.Context, opts *sql.TxOptions, fn func(*db.Queries) error) error {
	for i := 0; i < 3; i++ {
		tx, err := s.DB.BeginTx(ctx, opts)
		if err != nil {
			return err
		}
		err = fn(s.Q.WithTx(tx))
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if err == nil {
			return nil
		}
		_ = tx.Rollback()
		var p *pq.Error
		if errors.As(err, &p) && (p.Code == "40P01" || p.Code == "40001") {
			continue
		}
		return err
	}
	return fmt.Errorf("direct transaction retry exhausted")
}
func (s *Service) rate(actor, kind string, limit int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := actor + ":" + kind
	now := time.Now()
	b := s.limits[key]
	if now.Sub(b.At) >= time.Minute {
		b = bucket{At: now}
	}
	if b.Count >= limit {
		return social.Failure(429, "RATE_LIMITED", "操作过于频繁", map[string]any{"retryAfterSeconds": int((time.Minute - now.Sub(b.At)).Seconds()) + 1})
	}
	b.Count++
	s.limits[key] = b
	if len(s.limits) > 10000 {
		for k, v := range s.limits {
			if now.Sub(v.At) > time.Minute {
				delete(s.limits, k)
			}
		}
	}
	return nil
}
func profile(ctx context.Context, q *db.Queries, id string) (social.Profile, error) {
	p, err := q.SocialProfile(ctx, id)
	if err != nil {
		return social.Profile{}, err
	}
	return social.PublicProfile(p.UserID, p.Username, p.Nickname, p.Avatar, p.Signature, p.AccountStatus, p.OnlineStatus, p.ShowOnlineStatus), nil
}
func authorize(ctx context.Context, q *db.Queries, actor, room string) (db.DirectPair, error) {
	if !roomPattern.MatchString(room) {
		return db.DirectPair{}, fail(404, "RESOURCE_NOT_FOUND", "会话不可访问")
	}
	pair, err := q.DirectPairByRoom(ctx, room)
	if errors.Is(err, sql.ErrNoRows) {
		return pair, fail(404, "RESOURCE_NOT_FOUND", "会话不可访问")
	}
	if err != nil {
		return pair, err
	}
	if actor != pair.UserID1 && actor != pair.UserID2 {
		return pair, fail(404, "RESOURCE_NOT_FOUND", "会话不可访问")
	}
	me, err := q.SocialProfile(ctx, actor)
	if err != nil {
		return pair, err
	}
	if me.AccountStatus != "active" {
		return pair, fail(403, "ACCOUNT_UNAVAILABLE", "账号不可用")
	}
	return pair, nil
}
func peerID(p db.DirectPair, actor string) string {
	if p.UserID1 == actor {
		return p.UserID2
	}
	return p.UserID1
}
func (s *Service) reason(ctx context.Context, q *db.Queries, p db.DirectPair, actor string) (*string, error) {
	reason := ""
	if !s.Enabled {
		reason = "feature_disabled"
	} else {
		peer, err := q.SocialProfile(ctx, peerID(p, actor))
		if err != nil {
			return nil, err
		}
		if peer.AccountStatus != "active" {
			reason = "peer_unavailable"
		} else {
			_, err = q.SocialFriendSince(ctx, db.SocialFriendSinceParams{User1: p.UserID1, User2: p.UserID2})
			if errors.Is(err, sql.ErrNoRows) {
				reason = "not_friends"
			} else if err != nil {
				return nil, err
			} else {
				muted, err := q.IsUserGloballyMuted(ctx, actor)
				if err != nil {
					return nil, err
				}
				if muted {
					reason = "globally_muted"
				}
			}
		}
	}
	if reason == "" {
		return nil, nil
	}
	return &reason, nil
}
func (s *Service) writable(ctx context.Context, q *db.Queries, p db.DirectPair, actor string) error {
	me, err := q.SocialProfile(ctx, actor)
	if err != nil {
		return err
	}
	if me.AccountStatus != "active" {
		return fail(403, "ACCOUNT_UNAVAILABLE", "账号不可用")
	}
	r, err := s.reason(ctx, q, p, actor)
	if err != nil {
		return err
	}
	if r != nil {
		code := "DIRECT_READ_ONLY"
		status := 403
		if *r == "feature_disabled" {
			status = 503
			code = "FEATURE_DISABLED"
		}
		if *r == "globally_muted" {
			code = "GLOBALLY_MUTED"
		}
		return social.Failure(status, code, "当前会话仅可查看历史", map[string]any{"reason": *r})
	}
	return nil
}
func lockUsers(ctx context.Context, q *db.Queries, p db.DirectPair) error {
	ids, err := q.LockSocialPair(ctx, db.LockSocialPairParams{User1: p.UserID1, User2: p.UserID2})
	if err != nil {
		return err
	}
	if len(ids) != 2 {
		return fail(404, "RESOURCE_NOT_FOUND", "用户不可访问")
	}
	return nil
}
func (s *Service) message(ctx context.Context, q *db.Queries, m db.Message) (Message, error) {
	sender, err := profile(ctx, q, m.SenderID.String)
	if err != nil {
		return Message{}, err
	}
	out := Message{MessageID: m.MessageID, RoomID: m.RoomID, ConversationKind: "direct", ClientMessageID: m.ClientMessageID.UUID.String(), DirectSeq: strconv.FormatInt(m.DirectSeq.Int64, 10), Revision: strconv.FormatInt(m.DirectRevision.Int64, 10), UserID: m.SenderID.String, Sender: sender, Type: "text", Text: m.Content, Time: m.SentAt, IsDeleted: m.DeletedAt.Valid}
	if m.EditedAt.Valid {
		out.EditedAt = &m.EditedAt.Time
	}
	if m.DeletedAt.Valid {
		out.DeletedAt = &m.DeletedAt.Time
		out.Text = ""
	}
	if m.QuotedMessageID.Valid {
		out.QuotedMessageID = &m.QuotedMessageID.String
		quoted, err := q.DirectMessageByID(ctx, db.DirectMessageByIDParams{RoomID: m.RoomID, MessageID: m.QuotedMessageID.String})
		if err == nil {
			p, err := profile(ctx, q, quoted.SenderID.String)
			if err != nil {
				return out, err
			}
			text := []rune(quoted.Content)
			if len(text) > 100 {
				text = text[:100]
			}
			out.Quote = &Quote{MessageID: quoted.MessageID, Sender: p, Text: string(text), IsDeleted: quoted.DeletedAt.Valid}
			if quoted.DeletedAt.Valid {
				out.Quote.Text = ""
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return out, err
		}
	}
	return out, nil
}
func (s *Service) conversation(ctx context.Context, q *db.Queries, p db.DirectPair, actor string) (Conversation, error) {
	peer, err := profile(ctx, q, peerID(p, actor))
	if err != nil {
		return Conversation{}, err
	}
	activity, err := q.DirectActivity(ctx, p.RoomID)
	if err != nil {
		return Conversation{}, err
	}
	read, err := q.DirectReadPosition(ctx, db.DirectReadPositionParams{RoomID: p.RoomID, UserID: actor})
	if err != nil {
		return Conversation{}, err
	}
	unread, err := q.DirectUnread(ctx, db.DirectUnreadParams{RoomID: p.RoomID, ViewerID: actor})
	if err != nil {
		return Conversation{}, err
	}
	reason, err := s.reason(ctx, q, p, actor)
	if err != nil {
		return Conversation{}, err
	}
	out := Conversation{RoomID: p.RoomID, ConversationKind: "direct", Peer: peer, CreatedAt: activity.CreatedAt, LastActivityAt: activity.ActivityAt, UnreadCount: unread, LastReadSeq: strconv.FormatInt(read, 10), LatestSeq: strconv.FormatInt(p.LastMessageSeq, 10), ChangeRevision: strconv.FormatInt(p.ChangeRevision, 10), CanSend: reason == nil, ReadOnlyReason: reason}
	m, err := q.LatestDirectMessage(ctx, p.RoomID)
	if err == nil {
		msg, err := s.message(ctx, q, m)
		if err != nil {
			return out, err
		}
		out.LastMessage = &msg
	} else if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	return out, nil
}
func (s *Service) emit(ids []string, kind, action string, data any) {
	if s.Publish != nil {
		s.Publish(ids, events.Event{Type: kind, Action: action, Data: data, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)})
	}
}
func (s *Service) Create(ctx context.Context, actor, peer string) (CreateResult, error) {
	if !userPattern.MatchString(peer) || actor == peer {
		return CreateResult{}, fail(400, "INVALID_ARGUMENT", "私信对象无效")
	}
	var out CreateResult
	err := s.tx(ctx, nil, func(q *db.Queries) error {
		out = CreateResult{}
		pair := db.DirectPair{UserID1: actor, UserID2: peer}
		if err := lockUsers(ctx, q, pair); err != nil {
			return err
		}
		for _, id := range []string{actor, peer} {
			p, err := q.SocialProfile(ctx, id)
			if err != nil {
				return err
			}
			if p.AccountStatus != "active" {
				return fail(403, "TARGET_UNAVAILABLE", "用户不可用")
			}
		}
		if !s.Enabled {
			return fail(503, "FEATURE_DISABLED", "私信功能未启用")
		}
		if _, err := q.SocialFriendSince(ctx, db.SocialFriendSinceParams{User1: actor, User2: peer}); errors.Is(err, sql.ErrNoRows) {
			return fail(403, "DIRECT_REQUIRES_FRIENDSHIP", "请先添加好友")
		} else if err != nil {
			return err
		}
		pair, err := q.GetDirectPair(ctx, db.GetDirectPairParams{User1: actor, User2: peer})
		if errors.Is(err, sql.ErrNoRows) {
			if err = s.rate(actor, "create", 10); err != nil {
				return err
			}
			room, err := q.CreateDirectRoom(ctx)
			if err != nil {
				return err
			}
			pair, err = q.CreateDirectPair(ctx, db.CreateDirectPairParams{RoomID: room.RoomID, User1: actor, User2: peer})
			if err != nil {
				return err
			}
			for _, id := range []string{actor, peer} {
				if _, err = q.JoinChatroom(ctx, db.JoinChatroomParams{UserID: id, RoomID: room.RoomID, MemberRole: db.MemberRoleMember}); err != nil {
					return err
				}
			}
			out.Created = true
		} else if err != nil {
			return err
		}
		out.Conversation, err = s.conversation(ctx, q, pair, actor)
		return err
	})
	if err != nil {
		return CreateResult{}, err
	}
	if out.Created {
		s.emit([]string{actor, peer}, "direct", "created", map[string]any{"roomId": out.Conversation.RoomID})
	}
	return out, nil
}
func (s *Service) Detail(ctx context.Context, actor, room string) (Conversation, error) {
	var out Conversation
	err := s.tx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(q *db.Queries) error {
		p, err := authorize(ctx, q, actor, room)
		if err != nil {
			return err
		}
		out, err = s.conversation(ctx, q, p, actor)
		return err
	})
	return out, err
}

type listCursor struct {
	Time time.Time `json:"time"`
	Room string    `json:"room"`
}

func (s *Service) List(ctx context.Context, actor, cursor string, limit int) (List, error) {
	out := List{List: []Conversation{}}
	var marker listCursor
	arg := db.DirectConversationListParams{ViewerID: actor, PageLimit: int32(limit + 1)}
	if cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(data, &marker) != nil || marker.Time.IsZero() || !roomPattern.MatchString(marker.Room) {
			return out, fail(400, "INVALID_ARGUMENT", "会话游标无效")
		}
		arg.CursorTime = sql.NullTime{Time: marker.Time, Valid: true}
		arg.CursorRoom = marker.Room
	}
	err := s.tx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(q *db.Queries) error {
		rows, err := q.DirectConversationList(ctx, arg)
		if err != nil {
			return err
		}
		out.HasMore = len(rows) > limit
		if out.HasMore {
			rows = rows[:limit]
		}
		for _, r := range rows {
			p, err := authorize(ctx, q, actor, r.RoomID)
			if err != nil {
				return err
			}
			c, err := s.conversation(ctx, q, p, actor)
			if err != nil {
				return err
			}
			out.List = append(out.List, c)
		}
		if out.HasMore && len(rows) > 0 {
			last := rows[len(rows)-1]
			data, _ := json.Marshal(listCursor{last.ActivityAt, last.RoomID})
			out.NextCursor = base64.RawURLEncoding.EncodeToString(data)
		}
		return nil
	})
	return out, err
}
func (s *Service) Send(ctx context.Context, actor, room string, in SendInput) (Message, error) {
	text := strings.TrimSpace(in.Text)
	key, err := uuid.Parse(in.ClientMessageID)
	if err != nil || key == uuid.Nil || in.Type != "text" || utf8.RuneCountInString(text) < 1 || utf8.RuneCountInString(text) > 2000 {
		return Message{}, fail(400, "INVALID_ARGUMENT", "消息或发送编号无效")
	}
	payload, _ := json.Marshal([]string{room, actor, "text", text, in.QuotedMessageID})
	hash := sha256.Sum256(payload)
	digest := hex.EncodeToString(hash[:])
	var out Message
	var pair db.DirectPair
	created := false
	err = s.tx(ctx, nil, func(q *db.Queries) error {
		created = false
		p, err := authorize(ctx, q, actor, room)
		if err != nil {
			return err
		}
		if err = lockUsers(ctx, q, p); err != nil {
			return err
		}
		p, err = q.LockDirectRoom(ctx, room)
		if err != nil {
			return err
		}
		pair = p
		if _, err := authorize(ctx, q, actor, room); err != nil {
			return err
		}
		m, err := q.DirectMessageByKey(ctx, db.DirectMessageByKeyParams{RoomID: room, SenderID: actor, ClientMessageID: key})
		if err == nil {
			if m.RequestHash.String != digest {
				return fail(409, "MESSAGE_IDEMPOTENCY_CONFLICT", "发送编号对应不同内容")
			}
			out, err = s.message(ctx, q, m)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err = s.writable(ctx, q, p, actor); err != nil {
			return err
		}
		quote := sql.NullString{}
		if in.QuotedMessageID != "" {
			m, err := q.DirectMessageByID(ctx, db.DirectMessageByIDParams{RoomID: room, MessageID: in.QuotedMessageID})
			if err != nil || m.DeletedAt.Valid {
				return fail(400, "INVALID_REFERENCE", "引用消息不可访问")
			}
			quote = sql.NullString{String: in.QuotedMessageID, Valid: true}
		}
		if err = s.rate(actor, "send", 30); err != nil {
			return err
		}
		p, err = q.AdvanceDirectCounters(ctx, room)
		if err != nil {
			return err
		}
		m, err = q.InsertDirectMessage(ctx, db.InsertDirectMessageParams{RoomID: room, SenderID: actor, Content: text, QuotedMessageID: quote, ClientMessageID: key, RequestHash: digest, DirectSeq: p.LastMessageSeq, DirectRevision: p.ChangeRevision})
		if err != nil {
			return err
		}
		if err = q.RecordDirectChange(ctx, db.RecordDirectChangeParams{RoomID: room, Revision: p.ChangeRevision, MessageID: m.MessageID, Action: "new"}); err != nil {
			return err
		}
		if err = q.TouchDirectRoom(ctx, room); err != nil {
			return err
		}
		out, err = s.message(ctx, q, m)
		created = err == nil
		return err
	})
	if err != nil {
		return Message{}, err
	}
	if created {
		s.emit([]string{pair.UserID1, pair.UserID2}, "message", "new", out)
		s.emit([]string{pair.UserID1, pair.UserID2}, "direct", "updated", map[string]any{"roomId": room})
	}
	return out, nil
}
func (s *Service) Mutate(ctx context.Context, actor, room, id, text, expected string, withdraw bool) (Message, error) {
	text = strings.TrimSpace(text)
	revision, err := strconv.ParseInt(expected, 10, 64)
	if !withdraw && (err != nil || revision <= 0 || utf8.RuneCountInString(text) < 1 || utf8.RuneCountInString(text) > 2000) {
		return Message{}, fail(400, "INVALID_ARGUMENT", "编辑内容或版本无效")
	}
	var out Message
	var pair db.DirectPair
	changed := false
	err = s.tx(ctx, nil, func(q *db.Queries) error {
		changed = false
		p, err := authorize(ctx, q, actor, room)
		if err != nil {
			return err
		}
		if err = lockUsers(ctx, q, p); err != nil {
			return err
		}
		p, err = q.LockDirectRoom(ctx, room)
		if err != nil {
			return err
		}
		pair = p
		if _, err := authorize(ctx, q, actor, room); err != nil {
			return err
		}
		m, err := q.DirectMessageByID(ctx, db.DirectMessageByIDParams{RoomID: room, MessageID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return fail(404, "RESOURCE_NOT_FOUND", "消息不可访问")
		}
		if err != nil {
			return err
		}
		if m.SenderID.String != actor {
			return fail(403, "FORBIDDEN", "只能操作本人消息")
		}
		if withdraw && m.DeletedAt.Valid {
			out, err = s.message(ctx, q, m)
			return err
		}
		if err = s.writable(ctx, q, p, actor); err != nil {
			return err
		}
		if m.DeletedAt.Valid {
			return fail(409, "MESSAGE_DELETED", "消息已撤回")
		}
		if !withdraw && m.DirectRevision.Int64 != revision {
			return fail(409, "STALE_MESSAGE_REVISION", "消息已更新，请重新编辑")
		}
		p, err = q.AdvanceDirectRevision(ctx, room)
		if err != nil {
			return err
		}
		action := "edited"
		if withdraw {
			action = "deleted"
			m, err = q.WithdrawDirectMessage(ctx, db.WithdrawDirectMessageParams{RoomID: room, MessageID: id, Revision: p.ChangeRevision})
		} else {
			m, err = q.EditDirectMessage(ctx, db.EditDirectMessageParams{RoomID: room, MessageID: id, Revision: p.ChangeRevision, Content: text})
		}
		if err != nil {
			return err
		}
		if err = q.RecordDirectChange(ctx, db.RecordDirectChangeParams{RoomID: room, Revision: p.ChangeRevision, MessageID: id, Action: action}); err != nil {
			return err
		}
		out, err = s.message(ctx, q, m)
		changed = err == nil
		return err
	})
	if err != nil {
		return Message{}, err
	}
	if changed {
		action := "edited"
		if withdraw {
			action = "deleted"
		}
		s.emit([]string{pair.UserID1, pair.UserID2}, "message", action, out)
		s.emit([]string{pair.UserID1, pair.UserID2}, "direct", "updated", map[string]any{"roomId": room})
	}
	return out, nil
}
func (s *Service) History(ctx context.Context, actor, room string, before, after int64, useAfter bool, limit int) (History, error) {
	out := History{Messages: []Message{}}
	err := s.tx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(q *db.Queries) error {
		p, err := authorize(ctx, q, actor, room)
		if err != nil {
			return err
		}
		out.LatestSeq = strconv.FormatInt(p.LastMessageSeq, 10)
		out.ChangeRevision = strconv.FormatInt(p.ChangeRevision, 10)
		var rows []db.Message
		if useAfter {
			rows, err = q.DirectHistoryAfter(ctx, db.DirectHistoryAfterParams{RoomID: room, AfterSeq: after, PageLimit: int32(limit + 1)})
		} else {
			rows, err = q.DirectHistoryBefore(ctx, db.DirectHistoryBeforeParams{RoomID: room, BeforeSeq: before, PageLimit: int32(limit + 1)})
		}
		if err != nil {
			return err
		}
		out.HasMore = len(rows) > limit
		if out.HasMore {
			rows = rows[:limit]
		}
		if len(rows) > 0 {
			out.NextCursor = strconv.FormatInt(rows[len(rows)-1].DirectSeq.Int64, 10)
		}
		if !useAfter {
			for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
		for _, m := range rows {
			msg, err := s.message(ctx, q, m)
			if err != nil {
				return err
			}
			out.Messages = append(out.Messages, msg)
		}
		return nil
	})
	return out, err
}
func (s *Service) Changes(ctx context.Context, actor, room string, after int64, limit int) (Changes, error) {
	out := Changes{Changes: []Change{}}
	err := s.tx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(q *db.Queries) error {
		p, err := authorize(ctx, q, actor, room)
		if err != nil {
			return err
		}
		if after > p.ChangeRevision {
			return fail(400, "INVALID_MESSAGE_CURSOR", "变更游标超出会话版本")
		}
		out.UpToRevision = strconv.FormatInt(p.ChangeRevision, 10)
		rows, err := q.DirectChanges(ctx, db.DirectChangesParams{RoomID: room, AfterRevision: after, UpToRevision: p.ChangeRevision, PageLimit: int32(limit + 1)})
		if err != nil {
			return err
		}
		out.HasMore = len(rows) > limit
		if out.HasMore {
			rows = rows[:limit]
		}
		out.NextCursor = out.UpToRevision
		for _, r := range rows {
			m, err := q.DirectMessageByID(ctx, db.DirectMessageByIDParams{RoomID: room, MessageID: r.MessageID})
			if err != nil {
				return err
			}
			msg, err := s.message(ctx, q, m)
			if err != nil {
				return err
			}
			out.Changes = append(out.Changes, Change{strconv.FormatInt(r.Revision, 10), r.Action, msg})
			if out.HasMore {
				out.NextCursor = strconv.FormatInt(r.Revision, 10)
			}
		}
		return nil
	})
	return out, err
}
func (s *Service) Read(ctx context.Context, actor, room, messageID string) (ReadResult, error) {
	out := ReadResult{RoomID: room}
	err := s.tx(ctx, nil, func(q *db.Queries) error {
		if _, err := authorize(ctx, q, actor, room); err != nil {
			return err
		}
		if _, err := q.LockDirectRoom(ctx, room); err != nil {
			return err
		}
		m, err := q.DirectMessageByID(ctx, db.DirectMessageByIDParams{RoomID: room, MessageID: messageID})
		if err != nil {
			return fail(400, "INVALID_READ_TARGET", "阅读目标不可访问")
		}
		seq, err := q.AdvanceDirectRead(ctx, db.AdvanceDirectReadParams{RoomID: room, UserID: actor, TargetSeq: m.DirectSeq.Int64})
		if err != nil {
			return err
		}
		out.LastReadSeq = strconv.FormatInt(seq, 10)
		out.UnreadCount, err = q.DirectUnread(ctx, db.DirectUnreadParams{RoomID: room, ViewerID: actor})
		return err
	})
	if err == nil {
		s.emit([]string{actor}, "direct", "read", out)
	}
	return out, err
}

// Access supports transport authorization without weakening group guards.
func (s *Service) Access(ctx context.Context, actor, room string, write bool) error {
	p, err := authorize(ctx, s.Q, actor, room)
	if err != nil {
		return err
	}
	if write {
		return s.writable(ctx, s.Q, p, actor)
	}
	return nil
}
