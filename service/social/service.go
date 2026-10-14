package social

import (
	"chatroombackend/config"
	db "chatroombackend/db"
	"chatroombackend/events"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"
)

type Capabilities struct {
	Friends         bool `json:"friends"`
	DirectMessages  bool `json:"directMessages"`
	RoomInvitations bool `json:"roomInvitations"`
}

func DefaultCapabilities() Capabilities {
	if c := config.GetConfig(); c != nil {
		return Capabilities{Friends: c.Social.Friends, DirectMessages: c.Social.DirectMessages, RoomInvitations: c.Social.RoomInvitations}
	}
	return Capabilities{Friends: true, DirectMessages: true, RoomInvitations: true}
}

type Error struct {
	Status  int
	Code    string
	Message string
	Details any
}

func (e *Error) Error() string { return e.Code }
func Failure(status int, code, message string, details any) *Error {
	return &Error{status, code, message, details}
}

var userIDPattern = regexp.MustCompile(`^U[0-9]{9}$`)
var requestIDPattern = regexp.MustCompile(`^FR[0-9]{10}$`)

type Profile struct {
	UserID       string  `json:"userId"`
	Username     string  `json:"username,omitempty"`
	Nickname     string  `json:"nickname"`
	Avatar       string  `json:"avatar"`
	Signature    string  `json:"signature"`
	OnlineStatus *string `json:"onlineStatus"`
	Availability string  `json:"availability"`
}

func PublicProfile(id, username, nickname, avatar, signature, account, status string, visible bool) Profile {
	p := Profile{UserID: id, Nickname: "用户不可用", Availability: "unavailable"}
	if account != "active" {
		return p
	}
	p.Username = username
	p.Nickname = nickname
	p.Avatar = avatar
	p.Signature = signature
	p.Availability = "active"
	if visible {
		if status == "do_not_disturb" {
			status = "busy"
		}
		p.OnlineStatus = &status
	}
	return p
}
func ProfileFromRow(r db.SocialProfileRow) Profile { return profile(r) }
func profile(r db.SocialProfileRow) Profile {
	return PublicProfile(r.UserID, r.Username, r.Nickname, r.Avatar, r.Signature, r.AccountStatus, r.OnlineStatus, r.ShowOnlineStatus)
}

type Relationship struct {
	State            string  `json:"state"`
	PendingRequestID *string `json:"pendingRequestId"`
	CanRequest       bool    `json:"canRequest"`
	CanMessage       bool    `json:"canMessage"`
}
type Contact struct {
	Profile      Profile   `json:"profile"`
	FriendSince  time.Time `json:"friendSince"`
	Relationship string    `json:"relationship"`
	CanMessage   bool      `json:"canMessage"`
}
type Request struct {
	RequestID  string     `json:"requestId"`
	SenderID   string     `json:"senderId"`
	ReceiverID string     `json:"receiverId"`
	Message    string     `json:"message"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	HandledAt  *time.Time `json:"handledAt"`
	Peer       Profile    `json:"peer"`
	Direction  string     `json:"direction"`
	CanAccept  bool       `json:"canAccept"`
	CanReject  bool       `json:"canReject"`
	CanCancel  bool       `json:"canCancel"`
}
type RequestResult struct {
	Request      Request      `json:"request"`
	Relationship Relationship `json:"relationship"`
	Created      bool         `json:"created"`
}
type Page[T any] struct {
	List       []T   `json:"list"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	TotalPages int64 `json:"totalPages"`
}

func paginated[T any](list []T, total int64, page, size int) Page[T] {
	if list == nil {
		list = []T{}
	}
	return Page[T]{list, total, page, size, (total + int64(size) - 1) / int64(size)}
}

type Summary struct {
	ReceivedPendingFriendRequests int64 `json:"receivedPendingFriendRequests"`
	ReceivedPendingInvitations    int64 `json:"receivedPendingInvitations"`
	DirectUnreadMessages          int64 `json:"directUnreadMessages"`
}
type bucket struct {
	start time.Time
	count int
}
type Service struct {
	DB           *sql.DB
	Q            *db.Queries
	Capabilities Capabilities
	Publish      events.Publisher
	Now          func() time.Time
	SearchLimit  int
	RequestLimit int
	DailyLimit   int64
	mu           sync.Mutex
	limits       map[string]bucket
}

func New(database *sql.DB, q *db.Queries, c Capabilities, publish events.Publisher) *Service {
	return &Service{DB: database, Q: q, Capabilities: c, Publish: publish, Now: time.Now, SearchLimit: 10, RequestLimit: 10, DailyLimit: 50, limits: map[string]bucket{}}
}
func (s *Service) Actor(ctx context.Context, id string) error {
	r, err := s.Q.SocialProfile(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Failure(401, "UNAUTHORIZED", "账号不存在", nil)
	}
	if err != nil {
		return err
	}
	if r.AccountStatus != "active" {
		return Failure(403, "ACCOUNT_UNAVAILABLE", "账号不可用", nil)
	}
	return nil
}
func (s *Service) Enabled() error {
	if !s.Capabilities.Friends {
		return Failure(503, "FEATURE_DISABLED", "好友功能未启用", nil)
	}
	return nil
}
func (s *Service) rate(id, action string, limit int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	key := id + ":" + action
	b := s.limits[key]
	if now.Sub(b.start) >= time.Minute {
		b = bucket{start: now}
	}
	if b.count >= limit {
		return Failure(429, "RATE_LIMITED", "操作过于频繁，请稍后重试", map[string]any{"retryAfterSeconds": int(time.Minute.Seconds()-now.Sub(b.start).Seconds()) + 1})
	}
	b.count++
	s.limits[key] = b
	if len(s.limits) > 10000 {
		for k, v := range s.limits {
			if now.Sub(v.start) > time.Minute {
				delete(s.limits, k)
			}
		}
	}
	return nil
}
func (s *Service) Public(ctx context.Context, id string) (Profile, error) {
	if !userIDPattern.MatchString(id) {
		return Profile{}, Failure(404, "RESOURCE_NOT_FOUND", "用户不可访问", nil)
	}
	r, err := s.Q.SocialProfile(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, Failure(404, "RESOURCE_NOT_FOUND", "用户不可访问", nil)
	}
	if err != nil {
		return Profile{}, err
	}
	if r.AccountStatus != "active" {
		return Profile{}, Failure(404, "RESOURCE_NOT_FOUND", "用户不可访问", nil)
	}
	return profile(r), nil
}
func relationship(ctx context.Context, q *db.Queries, viewer, target string) (Relationship, error) {
	rel := Relationship{State: "none"}
	p, err := q.SocialProfile(ctx, target)
	if errors.Is(err, sql.ErrNoRows) {
		rel.State = "unavailable"
		return rel, nil
	}
	if err != nil {
		return rel, err
	}
	if p.AccountStatus != "active" {
		rel.State = "unavailable"
		return rel, nil
	}
	if viewer == target {
		rel.State = "self"
		return rel, nil
	}
	_, err = q.SocialFriendSince(ctx, db.SocialFriendSinceParams{User1: viewer, User2: target})
	if err == nil {
		rel.State = "friends"
		return rel, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return rel, err
	}
	pending, err := q.GetPendingRequestBetweenUsers(ctx, db.GetPendingRequestBetweenUsersParams{FromUserID: viewer, ToUserID: target})
	if err == nil {
		rel.PendingRequestID = &pending.RequestID
		if pending.FromUserID == viewer {
			rel.State = "outgoing_pending"
		} else {
			rel.State = "incoming_pending"
		}
		return rel, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return rel, err
	}
	rel.CanRequest = p.EnableFriendRequest
	return rel, nil
}
func (s *Service) Relationship(ctx context.Context, viewer, target string) (Relationship, error) {
	if !userIDPattern.MatchString(target) {
		return Relationship{}, Failure(400, "INVALID_ARGUMENT", "用户ID无效", nil)
	}
	r, err := relationship(ctx, s.Q, viewer, target)
	r.CanRequest = r.CanRequest && s.Capabilities.Friends
	r.CanMessage = s.Capabilities.DirectMessages && r.State == "friends"
	return r, err
}

type SearchResult struct {
	Profile      Profile      `json:"profile"`
	Relationship Relationship `json:"relationship"`
}

func (s *Service) Search(ctx context.Context, viewer, id, username string) ([]SearchResult, error) {
	if (id == "") == (username == "") || id != "" && !userIDPattern.MatchString(id) || utf8.RuneCountInString(username) > 255 {
		return nil, Failure(400, "INVALID_ARGUMENT", "请提供完整用户ID或精确用户名", nil)
	}
	if err := s.rate(viewer, "search", s.SearchLimit); err != nil {
		return nil, err
	}
	users, err := s.Q.SearchSocialUser(ctx, db.SearchSocialUserParams{UserID: id, Username: username})
	if err != nil {
		return nil, err
	}
	out := []SearchResult{}
	for _, u := range users {
		p, err := s.Public(ctx, u)
		if err != nil {
			return nil, err
		}
		r, err := s.Relationship(ctx, viewer, u)
		if err != nil {
			return nil, err
		}
		out = append(out, SearchResult{p, r})
	}
	return out, nil
}
func requestDTO(r db.FriendRequest, viewer string, p Profile) Request {
	out := Request{RequestID: r.RequestID, SenderID: r.FromUserID, ReceiverID: r.ToUserID, Message: r.Message.String, Status: r.Status, CreatedAt: r.Createtime, Peer: p, Direction: "sent"}
	if r.HandledAt.Valid {
		t := r.HandledAt.Time
		out.HandledAt = &t
	}
	if r.ToUserID == viewer {
		out.Direction = "received"
		out.CanReject = r.Status == "pending"
		out.CanAccept = out.CanReject && p.Availability == "active"
	} else {
		out.CanCancel = r.Status == "pending"
	}
	return out
}
func requestResult(ctx context.Context, q *db.Queries, r db.FriendRequest, viewer string, created bool) (RequestResult, error) {
	peer := r.ToUserID
	if viewer == peer {
		peer = r.FromUserID
	}
	p, err := q.SocialProfile(ctx, peer)
	if err != nil {
		return RequestResult{}, err
	}
	rel, err := relationship(ctx, q, viewer, peer)
	if err != nil {
		return RequestResult{}, err
	}
	dto := requestDTO(r, viewer, profile(p))
	if dto.CanAccept {
		actor, err := q.SocialProfile(ctx, viewer)
		if err != nil {
			return RequestResult{}, err
		}
		dto.CanAccept = actor.EnableFriendRequest
	}
	return RequestResult{dto, rel, created}, nil
}
func (s *Service) transaction(ctx context.Context, fn func(*db.Queries) error) error {
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.DB.BeginTx(ctx, nil)
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
		var pg *pq.Error
		if errors.As(err, &pg) && (pg.Code == "40P01" || pg.Code == "40001") {
			continue
		}
		return err
	}
	return fmt.Errorf("transaction retry exhausted")
}
func lockPair(ctx context.Context, q *db.Queries, a, b string) error {
	rows, err := q.LockSocialPair(ctx, db.LockSocialPairParams{User1: a, User2: b})
	if err != nil {
		return err
	}
	if len(rows) != 2 {
		return Failure(403, "TARGET_NOT_ADDABLE", "用户不存在或不可添加", nil)
	}
	return nil
}
func (s *Service) emit(ids []string, kind, action string, data any) {
	if s.Publish != nil {
		s.Publish(ids, events.Event{Type: kind, Action: action, Data: data, Timestamp: s.Now().UTC().Format(time.RFC3339Nano)})
	}
}
func (s *Service) CreateRequest(ctx context.Context, viewer, target, message string) (RequestResult, error) {
	if !userIDPattern.MatchString(target) || viewer == target || utf8.RuneCountInString(message) > 200 {
		return RequestResult{}, Failure(400, "INVALID_ARGUMENT", "申请对象或留言无效", nil)
	}
	var result RequestResult
	err := s.transaction(ctx, func(q *db.Queries) error {
		if err := lockPair(ctx, q, viewer, target); err != nil {
			return err
		}
		me, err := q.SocialProfile(ctx, viewer)
		if err != nil {
			return err
		}
		peer, err := q.SocialProfile(ctx, target)
		if err != nil {
			return err
		}
		if me.AccountStatus != "active" || peer.AccountStatus != "active" || !peer.EnableFriendRequest {
			return Failure(403, "TARGET_NOT_ADDABLE", "用户不存在或不可添加", nil)
		}
		rel, err := relationship(ctx, q, viewer, target)
		if err != nil {
			return err
		}
		if rel.State == "friends" {
			return Failure(409, "ALREADY_FRIENDS", "已经是好友", nil)
		}
		if rel.PendingRequestID != nil {
			r, err := q.GetFriendRequestByID(ctx, *rel.PendingRequestID)
			if err != nil {
				return err
			}
			if r.FromUserID != viewer {
				return Failure(409, "INCOMING_REQUEST_PENDING", "对方已发送好友申请", map[string]any{"requestId": r.RequestID})
			}
			result, err = requestResult(ctx, q, r, viewer, false)
			return err
		}
		latest, err := q.LatestSocialRequest(ctx, db.LatestSocialRequestParams{User1: viewer, User2: target})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && latest.HandledAt.Valid {
			var cooldown time.Duration
			if latest.Status == "rejected" {
				cooldown = 24 * time.Hour
			}
			if latest.Status == "cancelled" {
				cooldown = time.Minute
			}
			remaining := cooldown - s.Now().Sub(latest.HandledAt.Time)
			if cooldown > 0 && remaining > 0 {
				return Failure(429, "REQUEST_COOLDOWN", "请稍后重新申请", map[string]any{"retryAfterSeconds": int(remaining.Seconds()) + 1})
			}
		}
		daily, err := q.CountSocialRequestsToday(ctx, viewer)
		if err != nil {
			return err
		}
		if daily >= s.DailyLimit {
			return Failure(429, "RATE_LIMITED", "今日好友申请已达上限", map[string]any{"retryAfterSeconds": 86400})
		}
		if err = s.rate(viewer, "request", s.RequestLimit); err != nil {
			return err
		}
		r, err := q.CreateFriendRequest(ctx, db.CreateFriendRequestParams{FromUserID: viewer, ToUserID: target, Message: sql.NullString{String: message, Valid: true}})
		if err != nil {
			return err
		}
		result, err = requestResult(ctx, q, r, viewer, true)
		return err
	})
	if err != nil {
		return RequestResult{}, err
	}
	if result.Created {
		s.emit([]string{viewer, target}, "friend_request", "created", map[string]any{"requestId": result.Request.RequestID, "status": "pending"})
	}
	return result, nil
}
func (s *Service) HandleRequest(ctx context.Context, viewer, id, action string) (RequestResult, error) {
	if !requestIDPattern.MatchString(id) {
		return RequestResult{}, Failure(404, "RESOURCE_NOT_FOUND", "申请不可访问", nil)
	}
	initial, err := s.Q.GetFriendRequestByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return RequestResult{}, Failure(404, "RESOURCE_NOT_FOUND", "申请不可访问", nil)
	}
	if err != nil {
		return RequestResult{}, err
	}
	if viewer != initial.FromUserID && viewer != initial.ToUserID {
		return RequestResult{}, Failure(404, "RESOURCE_NOT_FOUND", "申请不可访问", nil)
	}
	want := map[string]string{"accept": "accepted", "reject": "rejected", "cancel": "cancelled"}[action]
	if want == "" {
		return RequestResult{}, Failure(400, "INVALID_ARGUMENT", "操作无效", nil)
	}
	if action == "cancel" && viewer != initial.FromUserID || action != "cancel" && viewer != initial.ToUserID {
		return RequestResult{}, Failure(403, "FORBIDDEN", "无权处理该申请", nil)
	}
	var result RequestResult
	changed := false
	err = s.transaction(ctx, func(q *db.Queries) error {
		changed = false
		if err := lockPair(ctx, q, initial.FromUserID, initial.ToUserID); err != nil {
			return err
		}
		r, err := q.LockSocialRequest(ctx, id)
		if err != nil {
			return err
		}
		if r.Status != "pending" {
			if r.Status != want {
				return Failure(409, "REQUEST_STATE_CONFLICT", "申请已被其他操作处理", nil)
			}
			result, err = requestResult(ctx, q, r, viewer, false)
			return err
		}
		if action == "accept" {
			a, err := q.SocialProfile(ctx, r.FromUserID)
			if err != nil {
				return err
			}
			b, err := q.SocialProfile(ctx, r.ToUserID)
			if err != nil {
				return err
			}
			if a.AccountStatus != "active" || b.AccountStatus != "active" || !b.EnableFriendRequest {
				return Failure(403, "TARGET_NOT_ADDABLE", "无法接受该申请", nil)
			}
			if err = q.CreateFriendship(ctx, db.CreateFriendshipParams{Column1: r.FromUserID, Column2: r.ToUserID}); err != nil {
				return err
			}
		}
		r, err = q.SetSocialRequestStatus(ctx, db.SetSocialRequestStatusParams{RequestID: id, Status: want})
		if err != nil {
			return err
		}
		result, err = requestResult(ctx, q, r, viewer, false)
		changed = err == nil
		return err
	})
	if err != nil {
		return RequestResult{}, err
	}
	if changed {
		ids := []string{initial.FromUserID, initial.ToUserID}
		s.emit(ids, "friend_request", "updated", map[string]any{"requestId": id, "status": want})
		if want == "accepted" {
			s.emit(ids, "friendship", "created", map[string]any{"userIds": ids})
		}
	}
	return result, nil
}
func (s *Service) Requests(ctx context.Context, viewer, direction, status string, page, size int) (Page[Request], error) {
	if direction != "received" && direction != "sent" {
		return Page[Request]{}, Failure(400, "INVALID_ARGUMENT", "申请方向无效", nil)
	}
	if status != "" && status != "pending" && status != "accepted" && status != "rejected" && status != "cancelled" {
		return Page[Request]{}, Failure(400, "INVALID_ARGUMENT", "申请状态无效", nil)
	}
	rows, err := s.Q.SocialRequestList(ctx, db.SocialRequestListParams{ViewerID: viewer, Direction: direction, StatusFilter: status, PageLimit: int32(size), PageOffset: int32((page - 1) * size)})
	if err != nil {
		return Page[Request]{}, err
	}
	total, err := s.Q.CountSocialRequestList(ctx, db.CountSocialRequestListParams{ViewerID: viewer, Direction: direction, StatusFilter: status})
	if err != nil {
		return Page[Request]{}, err
	}
	list := []Request{}
	for _, r := range rows {
		p := PublicProfile(r.UserID, r.Username, r.Nickname, r.Avatar, r.Signature, r.AccountStatus, r.OnlineStatus, r.ShowOnlineStatus)
		list = append(list, requestDTO(db.FriendRequest{RequestID: r.RequestID, FromUserID: r.FromUserID, ToUserID: r.ToUserID, Message: sql.NullString{String: r.Message, Valid: true}, Status: r.Status, Createtime: r.Createtime, HandledAt: r.HandledAt}, viewer, p))
	}
	actor, err := s.Q.SocialProfile(ctx, viewer)
	if err != nil {
		return Page[Request]{}, err
	}
	if !actor.EnableFriendRequest {
		for i := range list {
			list[i].CanAccept = false
		}
	}
	return paginated(list, total, page, size), nil
}
func (s *Service) Friends(ctx context.Context, viewer, search, status string, page, size int) (Page[Contact], error) {
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "online" && status != "away" && status != "busy" && status != "offline" {
		return Page[Contact]{}, Failure(400, "INVALID_ARGUMENT", "在线状态无效", nil)
	}
	if status == "busy" {
		status = "do_not_disturb"
	}
	if utf8.RuneCountInString(search) > 255 {
		return Page[Contact]{}, Failure(400, "INVALID_ARGUMENT", "搜索内容过长", nil)
	}
	rows, err := s.Q.SocialFriendList(ctx, db.SocialFriendListParams{ViewerID: viewer, Search: search, StatusFilter: status, PageLimit: int32(size), PageOffset: int32((page - 1) * size)})
	if err != nil {
		return Page[Contact]{}, err
	}
	total, err := s.Q.CountSocialFriendList(ctx, db.CountSocialFriendListParams{ViewerID: viewer, Search: search, StatusFilter: status})
	if err != nil {
		return Page[Contact]{}, err
	}
	list := []Contact{}
	for _, r := range rows {
		list = append(list, Contact{Profile: PublicProfile(r.UserID, r.Username, r.Nickname, r.Avatar, r.Signature, r.AccountStatus, r.OnlineStatus, r.ShowOnlineStatus), FriendSince: r.EstablishedAt, Relationship: "friends", CanMessage: s.Capabilities.DirectMessages && r.AccountStatus == "active"})
	}
	return paginated(list, total, page, size), nil
}
func (s *Service) Contact(ctx context.Context, viewer, target string) (Contact, error) {
	since, err := s.Q.SocialFriendSince(ctx, db.SocialFriendSinceParams{User1: viewer, User2: target})
	if errors.Is(err, sql.ErrNoRows) {
		return Contact{}, Failure(404, "RESOURCE_NOT_FOUND", "联系人不存在", nil)
	}
	if err != nil {
		return Contact{}, err
	}
	p, err := s.Q.SocialProfile(ctx, target)
	if err != nil {
		return Contact{}, err
	}
	return Contact{Profile: profile(p), FriendSince: since, Relationship: "friends", CanMessage: s.Capabilities.DirectMessages && p.AccountStatus == "active"}, nil
}
func (s *Service) Delete(ctx context.Context, viewer, target string) error {
	if viewer == target || !userIDPattern.MatchString(target) {
		return Failure(400, "INVALID_ARGUMENT", "用户ID无效", nil)
	}
	changed := false
	var invalidated []db.ChatroomInvitation
	err := s.transaction(ctx, func(q *db.Queries) error {
		changed = false
		invalidated = nil
		if err := lockPair(ctx, q, viewer, target); err != nil {
			var e *Error
			if errors.As(err, &e) && e.Code == "TARGET_NOT_ADDABLE" {
				return nil
			}
			return err
		}
		_, err := q.SocialFriendSince(ctx, db.SocialFriendSinceParams{User1: viewer, User2: target})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if err = q.DeleteFriendship(ctx, db.DeleteFriendshipParams{Column1: viewer, Column2: target}); err != nil {
			return err
		}
		if invalidated, err = q.InvalidateFriendInvitations(ctx, db.InvalidateFriendInvitationsParams{User1: viewer, User2: target}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err == nil && changed {
		s.emit([]string{viewer, target}, "friendship", "removed", map[string]any{"userIds": []string{viewer, target}})
		s.emit([]string{viewer, target}, "invitation", "updated", map[string]any{"reason": "friendship_removed"})
		seen := map[string]bool{}
		for _, i := range invalidated {
			if !seen[i.RoomID] {
				ids, e := s.Q.GroupManagers(ctx, i.RoomID)
				if e == nil {
					s.emit(ids, "room_invitations", "updated", map[string]any{"roomId": i.RoomID})
				}
				seen[i.RoomID] = true
			}
		}
	}
	return err
}
func (s *Service) Summary(ctx context.Context, viewer string) (Summary, error) {
	if !s.Capabilities.Friends && !s.Capabilities.DirectMessages && !s.Capabilities.RoomInvitations {
		return Summary{}, nil
	}
	var out Summary
	if s.Capabilities.RoomInvitations {
		n, err := s.Q.CountPendingInvitations(ctx, viewer)
		if err != nil {
			return out, err
		}
		out.ReceivedPendingInvitations = n
	}
	if s.Capabilities.Friends {
		n, err := s.Q.CountPendingReceivedRequests(ctx, viewer)
		if err != nil {
			return out, err
		}
		out.ReceivedPendingFriendRequests = n
	}
	if s.Capabilities.DirectMessages {
		n, err := s.Q.TotalDirectUnread(ctx, viewer)
		if err != nil {
			return out, err
		}
		out.DirectUnreadMessages = n
	}
	return out, nil
}
