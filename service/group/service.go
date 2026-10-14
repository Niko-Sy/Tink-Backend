// Package group serializes real membership changes with room mutations.
package group

import (
	"chatroombackend/db"
	"chatroombackend/events"
	social "chatroombackend/service/social"
	"context"
	"database/sql"
	"errors"
	"github.com/lib/pq"
	"time"
)

type Service struct {
	DB      *sql.DB
	Q       *db.Queries
	Publish events.Publisher
}

func New(d *sql.DB, q *db.Queries, p events.Publisher) *Service { return &Service{d, q, p} }

type Room struct {
	RoomID          string    `json:"roomId"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Icon            string    `json:"icon"`
	Type            string    `json:"type"`
	PeopleCount     int32     `json:"peopleCount"`
	OnlineCount     int32     `json:"onlineCount"`
	CreatedTime     time.Time `json:"createdTime"`
	LastMessageTime time.Time `json:"lastMessageTime"`
	CreatorID       string    `json:"creatorId,omitempty"`
}
type Member struct {
	MemberID string    `json:"memberId"`
	RoomID   string    `json:"roomId"`
	UserID   string    `json:"userId"`
	RoomRole string    `json:"roomRole"`
	IsMuted  bool      `json:"isMuted"`
	JoinedAt time.Time `json:"joinedAt"`
	IsActive bool      `json:"isActive"`
}
type Joined struct {
	Chatroom      Room   `json:"chatroom"`
	MemberInfo    Member `json:"memberInfo"`
	AlreadyMember bool   `json:"alreadyMember"`
}

func RoomDTO(r db.Chatroom) Room {
	typ := "public"
	if r.RoomType == db.ChatroomTypePrivateInviteOnly {
		typ = "private"
	}
	if r.RoomType == db.ChatroomTypePrivatePassword {
		typ = "protected"
	}
	at := r.CreatedAt
	if r.LastActiveAt.Valid {
		at = r.LastActiveAt.Time
	}
	return Room{RoomID: r.RoomID, Name: r.RoomName, Description: r.Description.String, Icon: r.IconUrl.String, Type: typ, PeopleCount: r.MemberCount, OnlineCount: r.OnlineCount, CreatedTime: r.CreatedAt, LastMessageTime: at}
}
func Actor(ctx context.Context, q *db.Queries, id string) error {
	p, e := q.SocialProfile(ctx, id)
	if e != nil || p.AccountStatus != "active" {
		return social.Failure(403, "ACCOUNT_UNAVAILABLE", "账号不可用", nil)
	}
	return nil
}
func CanInvite(ctx context.Context, q *db.Queries, actor string, r db.Chatroom) bool {
	m, e := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: actor, RoomID: r.RoomID})
	return e == nil && m.IsActive && (r.RoomType == db.ChatroomTypePublic || m.MemberRole == db.MemberRoleOwner || m.MemberRole == db.MemberRoleAdmin)
}
func Retryable(err error) bool {
	var e *pq.Error
	return errors.As(err, &e) && (e.Code == "40P01" || e.Code == "40001")
}
func (s *Service) Emit(ids []string, typ, action string, data any) {
	if s.Publish != nil {
		s.Publish(ids, events.Event{Type: typ, Action: action, Data: data, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)})
	}
}
func (s *Service) InvitationsChanged(rows []db.ChatroomInvitation) {
	seen := map[string]bool{}
	for _, i := range rows {
		if !seen[i.RoomID] {
			s.EmitRoomInvitationRefresh(i.RoomID)
			seen[i.RoomID] = true
		}
		s.Emit([]string{i.InviterID, i.InviteeID}, "invitation", "updated", map[string]any{"invitationId": i.InvitationID, "roomId": i.RoomID, "status": i.Status})
	}
}

// JoinTx assumes the caller has locked an active group room. It never publishes.
func JoinTx(ctx context.Context, q *db.Queries, user, room string) (Joined, error) {
	m, e := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: user, RoomID: room})
	already := e == nil && m.IsActive
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return Joined{}, e
	}
	if !already {
		if _, e = q.JoinChatroom(ctx, db.JoinChatroomParams{UserID: user, RoomID: room, MemberRole: db.MemberRoleMember}); e != nil {
			return Joined{}, e
		}
		if e = q.IncrementChatroomMemberCount(ctx, room); e != nil {
			return Joined{}, e
		}
		m, e = q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: user, RoomID: room})
		if e != nil {
			return Joined{}, e
		}
	}
	r, e := q.LockGroupRoom(ctx, room)
	if e != nil {
		return Joined{}, e
	}
	return Joined{RoomDTO(r), Member{m.MemberRelID, m.RoomID, m.UserID, string(m.MemberRole), m.MuteStatus == db.MemberMuteStatusMuted, m.JoinedAt, m.IsActive}, already}, nil
}
func (s *Service) run(ctx context.Context, actor, room string, fn func(*db.Queries, db.Chatroom) (any, []db.ChatroomInvitation, []string, error)) (any, error) {
	for n := 0; n < 3; n++ {
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return nil, e
		}
		q := s.Q.WithTx(tx)
		e = Actor(ctx, q, actor)
		var r db.Chatroom
		if e == nil {
			r, e = q.LockGroupRoom(ctx, room)
			if errors.Is(e, sql.ErrNoRows) {
				e = social.Failure(404, "RESOURCE_NOT_FOUND", "聊天室不可访问", nil)
			}
		}
		var out any
		var changed []db.ChatroomInvitation
		var recipients []string
		if e == nil {
			out, changed, recipients, e = fn(q, r)
		}
		if e == nil {
			e = tx.Commit()
		} else {
			tx.Rollback()
		}
		if e != nil {
			tx.Rollback()
			if Retryable(e) && n < 2 {
				continue
			}
			return nil, e
		}
		s.InvitationsChanged(changed)
		if recipients != nil {
			s.Emit(recipients, "room", "updated", map[string]any{"roomId": room})
		}
		return out, nil
	}
	return nil, social.Failure(503, "SERVICE_UNAVAILABLE", "请稍后重试", nil)
}
func (s *Service) Join(ctx context.Context, actor, room, password string) (Joined, error) {
	v, e := s.run(ctx, actor, room, func(q *db.Queries, r db.Chatroom) (any, []db.ChatroomInvitation, []string, error) {
		if r.RoomType == db.ChatroomTypePrivateInviteOnly {
			return nil, nil, nil, social.Failure(403, "INVITE_REQUIRED", "该聊天室需要邀请", nil)
		}
		if r.RoomType == db.ChatroomTypePrivatePassword && (!r.AccessPassword.Valid || r.AccessPassword.String != password) {
			return nil, nil, nil, social.Failure(403, "INVALID_PASSWORD", "聊天室密码错误", nil)
		}
		j, e := JoinTx(ctx, q, actor, room)
		return j, nil, nil, e
	})
	if e != nil {
		return Joined{}, e
	}
	j := v.(Joined)
	if !j.AlreadyMember {
		s.Emit([]string{actor}, "group_membership", "joined", map[string]any{"roomId": room, "userId": actor})
	}
	return j, nil
}
func (s *Service) Leave(ctx context.Context, actor, room string) error {
	changed := false
	_, e := s.run(ctx, actor, room, func(q *db.Queries, r db.Chatroom) (any, []db.ChatroomInvitation, []string, error) {
		changed = false
		m, e := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: actor, RoomID: room})
		if errors.Is(e, sql.ErrNoRows) {
			return nil, nil, nil, nil
		}
		if e != nil {
			return nil, nil, nil, e
		}
		if m.MemberRole == db.MemberRoleOwner {
			return nil, nil, nil, social.Failure(403, "OWNER_CANNOT_LEAVE", "房主不能退出聊天室", nil)
		}
		if e = q.LeaveChatroom(ctx, db.LeaveChatroomParams{UserID: actor, RoomID: room}); e != nil {
			return nil, nil, nil, e
		}
		if e = q.DecrementChatroomMemberCount(ctx, room); e != nil {
			return nil, nil, nil, e
		}
		rows, e := q.InvalidateMemberInvitations(ctx, db.InvalidateMemberInvitationsParams{RoomID: room, InviterID: actor})
		changed = true
		return nil, rows, nil, e
	})
	if e == nil && changed {
		s.Emit([]string{actor}, "group_membership", "left", map[string]any{"roomId": room, "userId": actor})
	}
	return e
}
func (s *Service) MemberAction(ctx context.Context, actor, room, memberID, action, reason string) error {
	target := ""
	changed := false
	_, e := s.run(ctx, actor, room, func(q *db.Queries, r db.Chatroom) (any, []db.ChatroomInvitation, []string, error) {
		changed = false
		a, e := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: actor, RoomID: room})
		if e != nil || (a.MemberRole != db.MemberRoleOwner && a.MemberRole != db.MemberRoleAdmin) {
			return nil, nil, nil, social.Failure(403, "FORBIDDEN", "没有管理权限", nil)
		}
		m, e := q.GetMemberByRelID(ctx, memberID)
		if e != nil || m.RoomID != room {
			return nil, nil, nil, social.Failure(404, "RESOURCE_NOT_FOUND", "成员不可访问", nil)
		}
		target = m.UserID
		if m.MemberRole == db.MemberRoleOwner || a.UserID == m.UserID || (a.MemberRole != db.MemberRoleOwner && (m.MemberRole == db.MemberRoleAdmin || action != "kick")) {
			return nil, nil, nil, social.Failure(403, "FORBIDDEN", "不能执行此成员操作", nil)
		}
		if !m.IsActive {
			if action == "kick" {
				return nil, nil, nil, nil
			}
			return nil, nil, nil, social.Failure(409, "MEMBER_INACTIVE", "成员已离开", nil)
		}
		switch action {
		case "kick":
			e = q.KickMember(ctx, db.KickMemberParams{UserID: target, RoomID: room})
			if e == nil {
				e = q.DecrementChatroomMemberCount(ctx, room)
			}
		case "setadmin":
			e = q.SetMemberAsAdmin(ctx, db.SetMemberAsAdminParams{UserID: target, RoomID: room})
		case "removeadmin":
			e = q.RemoveMemberAdmin(ctx, db.RemoveMemberAdminParams{UserID: target, RoomID: room})
		default:
			return nil, nil, nil, social.Failure(400, "INVALID_ARGUMENT", "操作无效", nil)
		}
		if e != nil {
			return nil, nil, nil, e
		}
		var rows []db.ChatroomInvitation
		if action == "kick" {
			rows, e = q.InvalidateMemberInvitations(ctx, db.InvalidateMemberInvitationsParams{RoomID: room, InviterID: target})
		} else {
			rows, e = q.InvalidateUnauthorizedInvitations(ctx, room)
		}
		changed = true
		ids, e2 := q.GroupActiveMembers(ctx, room)
		if e == nil {
			e = e2
		}
		return nil, rows, ids, e
	})
	if e == nil && changed && action == "kick" {
		s.Emit([]string{target}, "group_membership", "kicked", map[string]any{"roomId": room, "userId": target, "reason": reason})
	}
	return e
}

type Update struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Type        *string `json:"type"`
	Password    *string `json:"password"`
}

func (s *Service) Update(ctx context.Context, actor, room string, in Update) (Room, error) {
	v, e := s.run(ctx, actor, room, func(q *db.Queries, r db.Chatroom) (any, []db.ChatroomInvitation, []string, error) {
		a, e := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: actor, RoomID: room})
		if e != nil || (a.MemberRole != db.MemberRoleOwner && a.MemberRole != db.MemberRoleAdmin) {
			return nil, nil, nil, social.Failure(403, "FORBIDDEN", "没有管理权限", nil)
		}
		if in.Name != nil {
			if *in.Name == "" {
				return nil, nil, nil, social.Failure(400, "INVALID_ARGUMENT", "名称不能为空", nil)
			}
			r.RoomName = *in.Name
		}
		if in.Description != nil {
			r.Description = sql.NullString{String: *in.Description, Valid: true}
		}
		if in.Icon != nil {
			r.IconUrl = sql.NullString{String: *in.Icon, Valid: true}
		}
		if in.Password != nil {
			r.AccessPassword = sql.NullString{String: *in.Password, Valid: true}
		}
		if in.Type != nil {
			switch *in.Type {
			case "public":
				r.RoomType = db.ChatroomTypePublic
			case "private":
				r.RoomType = db.ChatroomTypePrivateInviteOnly
			case "protected":
				r.RoomType = db.ChatroomTypePrivatePassword
			default:
				return nil, nil, nil, social.Failure(400, "INVALID_ARGUMENT", "聊天室类型无效", nil)
			}
		}
		if r.RoomType == db.ChatroomTypePrivatePassword && r.AccessPassword.String == "" {
			return nil, nil, nil, social.Failure(400, "INVALID_ARGUMENT", "密码不能为空", nil)
		}
		_, e = q.UpdateChatroom(ctx, db.UpdateChatroomParams{Column1: room, RoomName: r.RoomName, Description: r.Description, IconUrl: r.IconUrl, RoomType: r.RoomType, AccessPassword: r.AccessPassword})
		if e != nil {
			return nil, nil, nil, e
		}
		rows, e := q.InvalidateUnauthorizedInvitations(ctx, room)
		if e != nil {
			return nil, nil, nil, e
		}
		r, e = q.LockGroupRoom(ctx, room)
		if e != nil {
			return nil, nil, nil, e
		}
		ids, e := q.GroupActiveMembers(ctx, room)
		return RoomDTO(r), rows, ids, e
	})
	if e != nil {
		return Room{}, e
	}
	return v.(Room), nil
}
func (s *Service) Delete(ctx context.Context, actor, room string) error {
	var ids []string
	_, e := s.run(ctx, actor, room, func(q *db.Queries, r db.Chatroom) (any, []db.ChatroomInvitation, []string, error) {
		a, e := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{UserID: actor, RoomID: room})
		if e != nil || a.MemberRole != db.MemberRoleOwner {
			return nil, nil, nil, social.Failure(403, "FORBIDDEN", "仅房主可删除聊天室", nil)
		}
		ids, e = q.GroupActiveMembers(ctx, room)
		if e != nil {
			return nil, nil, nil, e
		}
		if e = q.DeleteChatroom(ctx, room); e != nil {
			return nil, nil, nil, e
		}
		if e = q.DeactivateGroupMembers(ctx, room); e != nil {
			return nil, nil, nil, e
		}
		if e = q.ResetDeletedGroupCounts(ctx, room); e != nil {
			return nil, nil, nil, e
		}
		rows, e := q.InvalidateRoomInvitations(ctx, room)
		return nil, rows, nil, e
	})
	if e == nil {
		s.Emit(ids, "group_membership", "deleted", map[string]any{"roomId": room})
	}
	return e
}
func (s *Service) EmitRoomInvitationRefresh(room string) {
	ids, e := s.Q.GroupManagers(context.Background(), room)
	if e == nil {
		s.Emit(ids, "room_invitations", "updated", map[string]any{"roomId": room})
	}
}

type CreateInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Type        string `json:"type"`
	Password    string `json:"password"`
}

func (s *Service) Create(ctx context.Context, actor string, in CreateInput) (Joined, error) {
	if in.Name == "" {
		return Joined{}, social.Failure(400, "INVALID_ARGUMENT", "名称不能为空", nil)
	}
	kind := db.ChatroomTypePublic
	switch in.Type {
	case "public":
	case "private":
		kind = db.ChatroomTypePrivateInviteOnly
	case "protected":
		kind = db.ChatroomTypePrivatePassword
		if in.Password == "" {
			return Joined{}, social.Failure(400, "INVALID_ARGUMENT", "密码不能为空", nil)
		}
	default:
		return Joined{}, social.Failure(400, "INVALID_ARGUMENT", "类型无效", nil)
	}
	for n := 0; n < 3; n++ {
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return Joined{}, e
		}
		q := s.Q.WithTx(tx)
		_, e = q.LockSocialPair(ctx, db.LockSocialPairParams{User1: actor, User2: actor})
		if e == nil {
			e = Actor(ctx, q, actor)
		}
		var r db.CreateChatroomRow
		if e == nil {
			r, e = q.CreateChatroom(ctx, db.CreateChatroomParams{RoomName: in.Name, Description: sql.NullString{String: in.Description, Valid: true}, IconUrl: sql.NullString{String: in.Icon, Valid: true}, RoomType: kind, AccessPassword: sql.NullString{String: in.Password, Valid: kind == db.ChatroomTypePrivatePassword}})
		}
		var m db.JoinChatroomRow
		if e == nil {
			m, e = q.JoinChatroom(ctx, db.JoinChatroomParams{UserID: actor, RoomID: r.RoomID, MemberRole: db.MemberRoleOwner})
		}
		if e == nil {
			e = q.IncrementChatroomMemberCount(ctx, r.RoomID)
		}
		var room db.Chatroom
		if e == nil {
			room, e = q.LockGroupRoom(ctx, r.RoomID)
		}
		if e == nil {
			e = tx.Commit()
		} else {
			tx.Rollback()
		}
		if e != nil {
			tx.Rollback()
			if Retryable(e) && n < 2 {
				continue
			}
			return Joined{}, e
		}
		j := Joined{Chatroom: RoomDTO(room), MemberInfo: Member{m.MemberRelID, m.RoomID, m.UserID, string(m.MemberRole), false, m.JoinedAt, true}}
		j.Chatroom.CreatorID = actor
		s.Emit([]string{actor}, "group_membership", "joined", map[string]any{"roomId": room.RoomID, "userId": actor})
		return j, nil
	}
	return Joined{}, social.Failure(503, "SERVICE_UNAVAILABLE", "请稍后重试", nil)
}
