package invitation

import (
	"chatroombackend/db"
	"chatroombackend/events"
	"chatroombackend/service/group"
	social "chatroombackend/service/social"
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	DB      *sql.DB
	Q       *db.Queries
	Enabled bool
	Group   *group.Service
	Publish events.Publisher
}

func New(d *sql.DB, q *db.Queries, enabled bool, g *group.Service, p events.Publisher) *Service {
	return &Service{d, q, enabled, g, p}
}

type Invitation struct {
	CanEnter           bool           `json:"canEnter"`
	InvitationID       string         `json:"invitationId"`
	RoomID             string         `json:"roomId"`
	Direction          string         `json:"direction"`
	Status             string         `json:"status"`
	Inviter            social.Profile `json:"inviter"`
	Invitee            social.Profile `json:"invitee"`
	Room               group.Room     `json:"room"`
	Message            string         `json:"message"`
	CreatedAt          time.Time      `json:"createdAt"`
	ExpiresAt          time.Time      `json:"expiresAt"`
	HandledAt          *time.Time     `json:"handledAt"`
	InvalidationReason *string        `json:"invalidationReason"`
	CanAccept          bool           `json:"canAccept"`
	CanReject          bool           `json:"canReject"`
	CanCancel          bool           `json:"canCancel"`
}
type Result struct {
	Invitation Invitation `json:"invitation"`
	Created    bool       `json:"created"`
	*group.Joined
}
type Candidate struct {
	Profile  social.Profile `json:"profile"`
	Eligible bool           `json:"eligible"`
	Reason   string         `json:"reason"`
}
type RoomCandidate struct {
	group.Room
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason"`
}

func Page[T any](list []T, n int64, page, size int) social.Page[T] {
	if list == nil {
		list = []T{}
	}
	return social.Page[T]{List: list, Total: n, Page: page, PageSize: size, TotalPages: (n + int64(size) - 1) / int64(size)}
}
func (s *Service) actor(ctx context.Context, q *db.Queries, id string) error {
	return group.Actor(ctx, q, id)
}
func (s *Service) enabled() error {
	if !s.Enabled {
		return social.Failure(403, "INVITATIONS_DISABLED", "邀请功能暂未启用", nil)
	}
	return nil
}
func (s *Service) profile(ctx context.Context, q *db.Queries, id string) (social.Profile, error) {
	p, e := q.SocialProfile(ctx, id)
	if e != nil {
		return social.Profile{}, e
	}
	return social.ProfileFromRow(p), nil
}
func (s *Service) dto(ctx context.Context, q *db.Queries, i db.ChatroomInvitation, viewer, status string) (Invitation, error) {
	a, e := s.profile(ctx, q, i.InviterID)
	if e != nil {
		return Invitation{}, e
	}
	b, e := s.profile(ctx, q, i.InviteeID)
	if e != nil {
		return Invitation{}, e
	}
	out := Invitation{InvitationID: i.InvitationID.String(), RoomID: i.RoomID, Direction: "sent", Status: status, Inviter: a, Invitee: b, Message: i.Message, CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt}
	if viewer == i.InviteeID {
		out.Direction = "received"
	}
	if i.HandledAt.Valid {
		v := i.HandledAt.Time
		out.HandledAt = &v
	}
	if i.InvalidationReason.Valid {
		v := i.InvalidationReason.String
		out.InvalidationReason = &v
	} else if status == "invalidated" {
		v := "authorization_changed"
		out.InvalidationReason = &v
	}
	r, e := q.GetChatroomByID(ctx, i.RoomID)
	if e == nil {
		out.Room = group.Room{RoomID: r.RoomID, Name: r.RoomName, Description: r.Description.String, Icon: r.IconUrl.String, Type: "public", CreatedTime: r.CreatedAt}
		if r.RoomType == db.ChatroomTypePrivateInviteOnly {
			out.Room.Type = "private"
		}
		if r.RoomType == db.ChatroomTypePrivatePassword {
			out.Room.Type = "protected"
		}
	} else if errors.Is(e, sql.ErrNoRows) {
		out.Room = group.Room{RoomID: i.RoomID, Name: "不可用聊天室", Type: "unavailable"}
	} else {
		return Invitation{}, e
	}
	member, memberErr := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: viewer, RoomID: i.RoomID})
	out.CanEnter = memberErr == nil && member.IsActive && out.Room.Type != "unavailable"
	pending := status == "pending"
	out.CanAccept = pending && viewer == i.InviteeID && s.Enabled
	out.CanReject = pending && viewer == i.InviteeID
	out.CanCancel = pending && viewer == i.InviterID
	if pending && !out.CanCancel {
		m, e := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: viewer, RoomID: i.RoomID})
		out.CanCancel = e == nil && m.MemberRole == db.MemberRoleOwner
	}
	return out, nil
}
func pair(ctx context.Context, q *db.Queries, a, b string) error {
	ids, e := q.LockSocialPair(ctx, db.LockSocialPairParams{User1: a, User2: b})
	if e != nil {
		return e
	}
	if len(ids) != 2 {
		return social.Failure(404, "RESOURCE_NOT_FOUND", "用户不可访问", nil)
	}
	return nil
}
func (s *Service) emit(i db.ChatroomInvitation, action string) {
	s.Group.Emit([]string{i.InviterID, i.InviteeID}, "invitation", action, map[string]any{"invitationId": i.InvitationID.String(), "roomId": i.RoomID, "status": i.Status, "reason": i.InvalidationReason.String})
	s.Group.EmitRoomInvitationRefresh(i.RoomID)
}
func normalize(ctx context.Context, q *db.Queries, i db.ChatroomInvitation) (db.ChatroomInvitation, bool, error) {
	if i.Status != "pending" {
		return i, false, nil
	}
	status, e := q.EffectiveInvitationStatus(ctx, i.InvitationID)
	if e != nil {
		return i, false, e
	}
	if status == "pending" {
		return i, false, nil
	}
	reason := sql.NullString{}
	if status == "invalidated" {
		reason = sql.NullString{String: "authorization_changed", Valid: true}
	}
	i, e = q.TransitionInvitation(ctx, db.TransitionInvitationParams{InvitationID: i.InvitationID, Status: status, InvalidationReason: reason})
	return i, e == nil, e
}
func (s *Service) eligibility(ctx context.Context, q *db.Queries, actor, target, room string) (string, error) {
	if actor == target {
		return "SELF", nil
	}
	p, e := q.SocialProfile(ctx, target)
	if errors.Is(e, sql.ErrNoRows) {
		return "ACCOUNT_UNAVAILABLE", nil
	}
	if e != nil {
		return "", e
	}
	if p.AccountStatus != "active" {
		return "ACCOUNT_UNAVAILABLE", nil
	}
	_, e = q.SocialFriendSince(ctx, db.SocialFriendSinceParams{User1: actor, User2: target})
	if errors.Is(e, sql.ErrNoRows) {
		return "NOT_FRIENDS", nil
	}
	if e != nil {
		return "", e
	}
	_, e = q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: target, RoomID: room})
	if e == nil {
		return "ALREADY_MEMBER", nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", e
	}
	i, e := q.LatestInvitation(ctx, db.LatestInvitationParams{RoomID: room, InviteeID: target})
	if errors.Is(e, sql.ErrNoRows) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	effective, e := q.EffectiveInvitationStatus(ctx, i.InvitationID)
	if e != nil {
		return "", e
	}
	if effective == "pending" {
		return "INVITATION_ALREADY_PENDING", nil
	}
	now, e := q.InvitationClock(ctx)
	if e != nil {
		return "", e
	}
	duration := time.Duration(0)
	if i.Status == "rejected" {
		duration = 24 * time.Hour
	}
	if i.Status == "cancelled" {
		duration = time.Minute
	}
	if i.HandledAt.Valid && now.Before(i.HandledAt.Time.Add(duration)) {
		return "REQUEST_COOLDOWN", nil
	}
	return "", nil
}
func (s *Service) Create(ctx context.Context, actor, room, target, message string) (Result, error) {
	if e := s.enabled(); e != nil {
		return Result{}, e
	}
	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) > 200 || actor == target {
		return Result{}, social.Failure(400, "INVALID_ARGUMENT", "邀请内容无效", nil)
	}
	for n := 0; n < 3; n++ {
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return Result{}, e
		}
		q := s.Q.WithTx(tx)
		e = pair(ctx, q, actor, target)
		if e == nil {
			e = s.actor(ctx, q, actor)
		}
		var r db.Chatroom
		if e == nil {
			r, e = q.LockGroupRoom(ctx, room)
			if errors.Is(e, sql.ErrNoRows) {
				e = social.Failure(404, "RESOURCE_NOT_FOUND", "聊天室不可访问", nil)
			}
		}
		if e == nil && !group.CanInvite(ctx, q, actor, r) {
			e = social.Failure(403, "INVITE_FORBIDDEN", "没有邀请权限", nil)
		}
		var old db.ChatroomInvitation
		var stale bool
		if e == nil {
			old, e = q.PendingInvitation(ctx, db.PendingInvitationParams{RoomID: room, InviteeID: target})
			if errors.Is(e, sql.ErrNoRows) {
				e = nil
			} else if e == nil {
				old, stale, e = normalize(ctx, q, old)
			}
		}
		var i db.ChatroomInvitation
		created := false
		if e == nil && old.Status == "pending" {
			if old.InviterID != actor {
				e = social.Failure(409, "INVITATION_ALREADY_PENDING", "对方已有待处理邀请", nil)
			} else {
				i = old
			}
		} else if e == nil {
			var reason string
			reason, e = s.eligibility(ctx, q, actor, target, room)
			if e == nil && reason != "" {
				retryAfter := 60
				code := 403
				if reason == "REQUEST_COOLDOWN" {
					code = 429
					previous, err := q.LatestInvitation(ctx, db.LatestInvitationParams{RoomID: room, InviteeID: target})
					if err != nil {
						e = err
					} else {
						clock, err := q.InvitationClock(ctx)
						if err != nil {
							e = err
						} else {
							duration := time.Minute
							if previous.Status == "rejected" {
								duration = 24 * time.Hour
							}
							retryAfter = int(previous.HandledAt.Time.Add(duration).Sub(clock).Seconds()) + 1
						}
					}
				}
				if reason == "ALREADY_MEMBER" || reason == "INVITATION_ALREADY_PENDING" {
					code = 409
				}
				if e == nil {
					e = social.Failure(code, reason, "当前不能邀请该用户", map[string]any{"retryAfterSeconds": retryAfter})
				}
			}
			if e == nil {
				minute, e2 := q.CountInvitationsMinute(ctx, actor)
				if e2 != nil {
					e = e2
				}
				day, e2 := q.CountInvitationsToday(ctx, actor)
				if e2 != nil {
					e = e2
				}
				if e == nil && (minute >= 10 || day >= 100) {
					e = social.Failure(429, "RATE_LIMITED", "邀请过于频繁，请稍后重试", map[string]any{"retryAfterSeconds": 60})
				}
			}
			if e == nil {
				i, e = q.CreateInvitation(ctx, db.CreateInvitationParams{InvitationID: uuid.New(), RoomID: room, InviterID: actor, InviteeID: target, Message: message})
				created = e == nil
			}
		}
		var out Invitation
		if e == nil {
			out, e = s.dto(ctx, q, i, actor, i.Status)
		}
		// A stale terminal must be committed even if the next operation is rejected.
		business := e
		var domain *social.Error
		if e == nil || (stale && errors.As(e, &domain)) {
			e = tx.Commit()
		} else {
			tx.Rollback()
		}
		if e != nil {
			tx.Rollback()
			if group.Retryable(e) && n < 2 {
				continue
			}
			return Result{}, e
		}
		if stale {
			s.emit(old, "updated")
		}
		if business != nil {
			return Result{}, business
		}
		if created {
			s.emit(i, "created")
		}
		return Result{Invitation: out, Created: created}, nil
	}
	return Result{}, social.Failure(503, "SERVICE_UNAVAILABLE", "请稍后重试", nil)
}
func (s *Service) Handle(ctx context.Context, actor, id, action string) (Result, error) {
	parsed, e := uuid.Parse(id)
	if e != nil {
		return Result{}, social.Failure(404, "RESOURCE_NOT_FOUND", "邀请不可访问", nil)
	}
	base, e := s.Q.GetChatroomInvitation(ctx, parsed)
	if e != nil {
		return Result{}, social.Failure(404, "RESOURCE_NOT_FOUND", "邀请不可访问", nil)
	}
	for n := 0; n < 3; n++ {
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return Result{}, e
		}
		q := s.Q.WithTx(tx)
		e = pair(ctx, q, base.InviterID, base.InviteeID)
		if e == nil {
			e = s.actor(ctx, q, actor)
		}
		// Deleted rooms can still expose authorized terminal history/rejection/cancel.
		_, roomErr := q.LockGroupRoom(ctx, base.RoomID)
		if roomErr != nil && !errors.Is(roomErr, sql.ErrNoRows) {
			e = roomErr
		}
		var i db.ChatroomInvitation
		if e == nil {
			i, e = q.LockInvitation(ctx, parsed)
		}
		if e == nil {
			allowed := actor == i.InviteeID && (action == "accept" || action == "reject") || actor == i.InviterID && action == "cancel"
			if !allowed && action == "cancel" {
				m, err := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: actor, RoomID: i.RoomID})
				allowed = err == nil && m.MemberRole == db.MemberRoleOwner
			}
			if !allowed {
				e = social.Failure(404, "RESOURCE_NOT_FOUND", "邀请不可访问", nil)
			}
		}
		changed := false
		var business error
		var joined *group.Joined
		if e == nil {
			i, changed, e = normalize(ctx, q, i)
		}
		if e == nil {
			switch {
			case i.Status == "expired":
				business = social.Failure(410, "INVITATION_EXPIRED", "邀请已过期", nil)
			case i.Status == "invalidated":
				business = social.Failure(410, "INVITATION_INVALIDATED", "邀请已失效", nil)
			case i.Status == "accepted" && action == "accept":
				_, err := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: actor, RoomID: i.RoomID})
				if err != nil || roomErr != nil {
					business = social.Failure(409, "INVITATION_ALREADY_USED", "邀请已使用，请重新取得邀请", nil)
				} else {
					j, err := group.JoinTx(ctx, q, actor, i.RoomID)
					e = err
					joined = &j
				}
			case i.Status != "pending":
				expected := map[string]string{"reject": "rejected", "cancel": "cancelled"}[action]
				if i.Status != expected {
					business = social.Failure(409, "INVITATION_STATE_CONFLICT", "邀请已处理", nil)
				}
			default:
				if action == "accept" {
					e = s.enabled()
					if e == nil {
						j, err := group.JoinTx(ctx, q, actor, i.RoomID)
						e = err
						joined = &j
					}
				}
				status := map[string]string{"accept": "accepted", "reject": "rejected", "cancel": "cancelled"}[action]
				if status == "" {
					e = social.Failure(400, "INVALID_ARGUMENT", "操作无效", nil)
				}
				if e == nil {
					i, e = q.TransitionInvitation(ctx, db.TransitionInvitationParams{InvitationID: parsed, Status: status})
					changed = e == nil
				}
			}
		}
		var out Invitation
		if e == nil {
			out, e = s.dto(ctx, q, i, actor, i.Status)
		}
		if e == nil {
			e = tx.Commit()
		} else {
			tx.Rollback()
		}
		if e != nil {
			tx.Rollback()
			if group.Retryable(e) && n < 2 {
				continue
			}
			return Result{}, e
		}
		if changed {
			s.emit(i, "updated")
		}
		if joined != nil && !joined.AlreadyMember {
			s.Group.Emit([]string{actor}, "group_membership", "joined", map[string]any{"roomId": i.RoomID, "userId": actor})
		}
		if business != nil {
			return Result{}, business
		}
		return Result{Invitation: out, Joined: joined}, nil
	}
	return Result{}, social.Failure(503, "SERVICE_UNAVAILABLE", "请稍后重试", nil)
}
func (s *Service) Cleanup(ctx context.Context, limit int32) error {
	rows, e := s.Q.StaleInvitations(ctx, limit)
	if e != nil {
		return e
	}
	for _, i := range rows {
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		q := s.Q.WithTx(tx)
		e = pair(ctx, q, i.InviterID, i.InviteeID)
		if e == nil {
			_, err := q.LockGroupRoom(ctx, i.RoomID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				e = err
			}
		}
		var changed bool
		if e == nil {
			i, e = q.LockInvitation(ctx, i.InvitationID)
		}
		if e == nil {
			i, changed, e = normalize(ctx, q, i)
		}
		if e == nil {
			e = tx.Commit()
		} else {
			tx.Rollback()
		}
		if e != nil {
			return e
		}
		if changed {
			s.emit(i, "updated")
		}
	}
	return nil
}
func (s *Service) RunCleanup(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if e := s.Cleanup(ctx, 100); e != nil && ctx.Err() == nil {
				s.Group.Emit(nil, "operation", "cleanup_failed", map[string]any{"operation": "invitation_cleanup"})
			}
		}
	}
}
func (s *Service) List(ctx context.Context, actor, scope, room, status string, page, size int) (social.Page[Invitation], error) {
	if scope != "received" && scope != "sent" && scope != "room" {
		return social.Page[Invitation]{}, social.Failure(400, "INVALID_ARGUMENT", "方向无效", nil)
	}
	valid := map[string]bool{"": true, "pending": true, "accepted": true, "rejected": true, "cancelled": true, "expired": true, "invalidated": true}
	if !valid[status] {
		return social.Page[Invitation]{}, social.Failure(400, "INVALID_ARGUMENT", "状态无效", nil)
	}
	if e := s.actor(ctx, s.Q, actor); e != nil {
		return social.Page[Invitation]{}, e
	}
	if e := s.Cleanup(ctx, 100); e != nil {
		return social.Page[Invitation]{}, e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return social.Page[Invitation]{}, e
	}
	defer tx.Rollback()
	q := s.Q.WithTx(tx)
	manage := false
	if scope == "room" {
		m, err := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: actor, RoomID: room})
		_, roomErr := q.GetChatroomByID(ctx, room)
		if err != nil || roomErr != nil {
			return social.Page[Invitation]{}, social.Failure(404, "RESOURCE_NOT_FOUND", "聊天室不可访问", nil)
		}
		manage = m.MemberRole == db.MemberRoleOwner || m.MemberRole == db.MemberRoleAdmin
	}
	rows, e := q.InvitationList(ctx, db.InvitationListParams{Scope: scope, Viewer: actor, Room: room, Manage: manage, StatusFilter: status, PageLimit: int32(size), PageOffset: int32((page - 1) * size)})
	if e != nil {
		return social.Page[Invitation]{}, e
	}
	total, e := q.CountInvitationList(ctx, db.CountInvitationListParams{Scope: scope, Viewer: actor, Room: room, Manage: manage, StatusFilter: status})
	if e != nil {
		return social.Page[Invitation]{}, e
	}
	list := []Invitation{}
	for _, v := range rows {
		i := db.ChatroomInvitation{InvitationID: v.InvitationID, RoomID: v.RoomID, InviterID: v.InviterID, InviteeID: v.InviteeID, Message: v.Message, Status: v.Status, CreatedAt: v.CreatedAt, ExpiresAt: v.ExpiresAt, HandledAt: v.HandledAt, InvalidationReason: v.InvalidationReason}
		out, e := s.dto(ctx, q, i, actor, v.EffectiveStatus)
		if e != nil {
			return social.Page[Invitation]{}, e
		}
		list = append(list, out)
	}
	return Page(list, total, page, size), tx.Commit()
}
func (s *Service) Candidates(ctx context.Context, actor, room, search string, page, size int) (social.Page[Candidate], error) {
	if e := s.enabled(); e != nil {
		return social.Page[Candidate]{}, e
	}
	if e := s.actor(ctx, s.Q, actor); e != nil {
		return social.Page[Candidate]{}, e
	}
	r, e := s.Q.LockFreeGroupRoom(ctx, room)
	if e != nil {
		return social.Page[Candidate]{}, social.Failure(404, "RESOURCE_NOT_FOUND", "聊天室不可访问", nil)
	}
	if !group.CanInvite(ctx, s.Q, actor, r) {
		return social.Page[Candidate]{}, social.Failure(403, "INVITE_FORBIDDEN", "没有邀请权限", nil)
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return social.Page[Candidate]{}, e
	}
	defer tx.Rollback()
	q := s.Q.WithTx(tx)
	rows, e := q.SocialFriendList(ctx, db.SocialFriendListParams{ViewerID: actor, Search: search, StatusFilter: "all", PageLimit: int32(size), PageOffset: int32((page - 1) * size)})
	if e != nil {
		return social.Page[Candidate]{}, e
	}
	total, e := q.CountSocialFriendList(ctx, db.CountSocialFriendListParams{ViewerID: actor, Search: search, StatusFilter: "all"})
	if e != nil {
		return social.Page[Candidate]{}, e
	}
	list := []Candidate{}
	for _, v := range rows {
		p, e := s.profile(ctx, q, v.UserID)
		if e != nil {
			return social.Page[Candidate]{}, e
		}
		reason, e := s.eligibility(ctx, q, actor, v.UserID, room)
		if e != nil {
			return social.Page[Candidate]{}, e
		}
		list = append(list, Candidate{p, reason == "", reason})
	}
	return Page(list, total, page, size), tx.Commit()
}
func (s *Service) Rooms(ctx context.Context, actor, target, search string, page, size int) (social.Page[RoomCandidate], error) {
	if e := s.enabled(); e != nil {
		return social.Page[RoomCandidate]{}, e
	}
	if e := s.actor(ctx, s.Q, actor); e != nil {
		return social.Page[RoomCandidate]{}, e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return social.Page[RoomCandidate]{}, e
	}
	defer tx.Rollback()
	q := s.Q.WithTx(tx)
	rows, e := q.InvitableRooms(ctx, db.InvitableRoomsParams{Viewer: actor, Search: search, PageLimit: int32(size), PageOffset: int32((page - 1) * size)})
	if e != nil {
		return social.Page[RoomCandidate]{}, e
	}
	total, e := q.CountInvitableRooms(ctx, db.CountInvitableRoomsParams{Viewer: actor, Search: search})
	if e != nil {
		return social.Page[RoomCandidate]{}, e
	}
	list := []RoomCandidate{}
	for _, r := range rows {
		reason := ""
		if target != "" {
			reason, e = s.eligibility(ctx, q, actor, target, r.RoomID)
			if e != nil {
				return social.Page[RoomCandidate]{}, e
			}
		}
		list = append(list, RoomCandidate{group.RoomDTO(r), reason == "", reason})
	}
	return Page(list, total, page, size), tx.Commit()
}
