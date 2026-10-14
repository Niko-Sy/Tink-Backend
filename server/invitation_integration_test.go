package server_test

import (
	"chatroombackend/events"
	"chatroombackend/server"
	"chatroombackend/service/group"
	invite "chatroombackend/service/invitation"
	social "chatroombackend/service/social"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func makeGroup(t *testing.T, owner, kind string) string {
	t.Helper()
	code, out := call(t, owner, "POST", "/chatroom/createroom", map[string]any{"name": t.Name(), "type": kind, "password": "secret-room-pass"})
	if code != 200 {
		t.Fatal(code, out)
	}
	var v struct {
		Chatroom group.Room `json:"chatroom"`
	}
	json.Unmarshal(out.Data, &v)
	return v.Chatroom.RoomID
}
func inviteTo(t *testing.T, a, b, r string) invite.Result {
	t.Helper()
	code, out := call(t, a, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": b, "message": "hello"})
	if code != 201 && code != 200 {
		t.Fatal(code, out)
	}
	var v invite.Result
	json.Unmarshal(out.Data, &v)
	if v.Invitation.InvitationID == "" {
		t.Fatal(string(out.Data))
	}
	return v
}
func acceptInvite(t *testing.T, id, actor string) (int, envelope) {
	return call(t, actor, "POST", "/chatroom-invitations/"+id+"/accept", nil)
}
func assertMembers(t *testing.T, room string, n int) {
	t.Helper()
	if got := count(t, "SELECT member_count FROM chatrooms WHERE room_id=$1", room); got != n {
		t.Fatalf("counter got %d expected %d", got, n)
	}
	if got := count(t, "SELECT count(*) FROM chatroom_members WHERE room_id=$1 AND is_active", room); got != n {
		t.Fatalf("active got %d expected %d", got, n)
	}
}
func TestInvitationsBatch(t *testing.T) {
	old := api
	srv := httptest.NewServer(server.NewRouter(manager, social.Capabilities{Friends: true, DirectMessages: true, RoomInvitations: true}))
	api = srv
	defer func() { api = old; srv.Close() }()
	t.Run("permissions lifecycle privacy", func(t *testing.T) {
		for _, kind := range []string{"public", "private", "protected"} {
			t.Run(kind, func(t *testing.T) {
				a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
				befriend(t, a, b)
				r := makeGroup(t, a, kind)
				i := inviteTo(t, a, b, r)
				raw, _ := json.Marshal(i)
				if strings.Contains(string(raw), "secret-room-pass") {
					t.Fatal("password leaked")
				}
				if code, _ := acceptInvite(t, i.Invitation.InvitationID, c); code != 404 {
					t.Fatal(code)
				}
				if code, _ := acceptInvite(t, i.Invitation.InvitationID, b); code != 200 {
					t.Fatal(code)
				}
				assertMembers(t, r, 2)
				befriend(t, b, c)
				code, _ := call(t, b, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": c})
				if kind == "public" && code != 201 || kind != "public" && code != 403 {
					t.Fatal(code)
				}
				if code, _ := call(t, c, "POST", "/chatroom/joinroom", map[string]string{"roomId": r}); kind != "public" && code != 403 {
					t.Fatal(code)
				}
				call(t, b, "POST", "/chatroom/leaveroom", map[string]string{"roomId": r})
				if code, out := acceptInvite(t, i.Invitation.InvitationID, b); code != 409 || out.BusinessCode != "INVITATION_ALREADY_USED" {
					t.Fatal(code, out)
				}
				expected := 1
				if kind == "public" {
					expected = 2
				}
				assertMembers(t, r, expected)
			})
		}
	})
	t.Run("pending and join concurrency", func(t *testing.T) {
		a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
		befriend(t, a, b)
		befriend(t, c, b)
		r := makeGroup(t, a, "public")
		call(t, c, "POST", "/chatroom/joinroom", map[string]string{"roomId": r})
		var wg sync.WaitGroup
		for j := 0; j < 8; j++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				from := a
				if j%2 == 0 {
					from = c
				}
				code, _ := call(t, from, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": b})
				if code != 200 && code != 201 && code != 409 {
					t.Error(code)
				}
			}(j)
		}
		wg.Wait()
		if count(t, "SELECT count(*) FROM chatroom_invitations WHERE room_id=$1 AND status='pending'", r) != 1 {
			t.Fatal("duplicate")
		}
		var id string
		environment.DB.QueryRow("SELECT invitation_id FROM chatroom_invitations WHERE room_id=$1 AND status='pending'", r).Scan(&id)
		for j := 0; j < 8; j++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				if j%2 == 0 {
					acceptInvite(t, id, b)
				} else {
					call(t, b, "POST", "/chatroom/joinroom", map[string]string{"roomId": r})
				}
			}(j)
		}
		wg.Wait()
		assertMembers(t, r, 3)
	})
	t.Run("expiry cooldown and cleanup independence", func(t *testing.T) {
		a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
		befriend(t, a, b)
		befriend(t, c, b)
		r := makeGroup(t, a, "public")
		call(t, c, "POST", "/chatroom/joinroom", map[string]string{"roomId": r})
		i := inviteTo(t, a, b, r)
		environment.DB.Exec("UPDATE chatroom_invitations SET created_at=now()-interval '8 days',expires_at=now()-interval '1 day' WHERE invitation_id=$1", i.Invitation.InvitationID)
		if code, out := acceptInvite(t, i.Invitation.InvitationID, b); code != 410 || out.BusinessCode != "INVITATION_EXPIRED" {
			t.Fatal(code, out)
		}
		if count(t, "SELECT count(*) FROM chatroom_invitations WHERE invitation_id=$1 AND status='expired'", i.Invitation.InvitationID) != 1 {
			t.Fatal("rollback expiry")
		}
		i = inviteTo(t, a, b, r)
		call(t, b, "POST", "/chatroom-invitations/"+i.Invitation.InvitationID+"/reject", nil)
		if code, out := call(t, c, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": b}); code != 429 || out.BusinessCode != "REQUEST_COOLDOWN" {
			t.Fatal(code, out)
		}
		environment.DB.Exec("UPDATE chatroom_invitations SET handled_at=now()-interval '25 hours' WHERE invitation_id=$1", i.Invitation.InvitationID)
		i = inviteTo(t, c, b, r)
		call(t, c, "POST", "/chatroom-invitations/"+i.Invitation.InvitationID+"/cancel", nil)
		if code, _ := call(t, a, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": b}); code != 429 {
			t.Fatal(code)
		}
	})
	t.Run("invalidation hooks", func(t *testing.T) {
		for _, mutation := range []string{"friend", "leave", "type", "delete", "account"} {
			t.Run(mutation, func(t *testing.T) {
				a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
				befriend(t, a, b)
				befriend(t, c, b)
				r := makeGroup(t, a, "public")
				call(t, c, "POST", "/chatroom/joinroom", map[string]string{"roomId": r})
				from := a
				if mutation == "leave" || mutation == "type" {
					from = c
				}
				i := inviteTo(t, from, b, r)
				switch mutation {
				case "friend":
					call(t, a, "DELETE", "/friends/"+b, nil)
				case "leave":
					call(t, c, "POST", "/chatroom/leaveroom", map[string]string{"roomId": r})
				case "type":
					call(t, a, "POST", "/chatroom/"+r+"/update", map[string]string{"type": "private"})
				case "delete":
					call(t, a, "POST", "/chatroom/"+r+"/delete", nil)
				case "account":
					environment.DB.Exec("UPDATE users SET account_status='suspended' WHERE user_id=$1", a)
				}
				if code, out := acceptInvite(t, i.Invitation.InvitationID, b); code != 410 || out.BusinessCode != "INVITATION_INVALIDATED" {
					t.Fatal(code, out)
				}
			})
		}
	})
	t.Run("transaction failure rollback", func(t *testing.T) {
		a, b := actor(t, "a"), actor(t, "b")
		befriend(t, a, b)
		r := makeGroup(t, a, "private")
		i := inviteTo(t, a, b, r)
		_, e := environment.DB.Exec("ALTER TABLE chatrooms ADD CONSTRAINT fail_invitation_count CHECK(room_id<>'" + r + "' OR member_count<2)")
		if e != nil {
			t.Fatal(e)
		}
		defer environment.DB.Exec("ALTER TABLE chatrooms DROP CONSTRAINT fail_invitation_count")
		if code, _ := acceptInvite(t, i.Invitation.InvitationID, b); code != 500 {
			t.Fatal(code)
		}
		assertMembers(t, r, 1)
		if count(t, "SELECT count(*) FROM chatroom_invitations WHERE invitation_id=$1 AND status='pending'", i.Invitation.InvitationID) != 1 {
			t.Fatal("half accepted")
		}
	})
	t.Run("off access candidates summary and direct isolation", func(t *testing.T) {
		a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
		befriend(t, a, b)
		r := makeGroup(t, a, "private")
		i := inviteTo(t, a, b, r)
		if code, out := call(t, b, "GET", "/users/me/social-summary", nil); code != 200 || !strings.Contains(string(out.Data), `"receivedPendingInvitations":1`) {
			t.Fatal(code, out)
		}
		if code, _ := call(t, c, "GET", "/chatroom/"+r+"/invitations", nil); code != 404 {
			t.Fatal(code)
		}
		if code, _ := call(t, a, "GET", "/chatroom/"+r+"/invite-candidates?pageSize=1", nil); code != 200 {
			t.Fatal(code)
		}
		if code, _ := call(t, a, "GET", "/users/me/invitable-chatrooms?inviteeId="+b, nil); code != 200 {
			t.Fatal(code)
		}
		direct := createDirect(t, a, b)
		if code, _ := call(t, a, "POST", "/chatroom/"+direct.RoomID+"/invitations", map[string]string{"inviteeId": c}); code != 404 {
			t.Fatal(code)
		}
		g := group.New(environment.DB, manager.GetQueries(), nil)
		off := invite.New(environment.DB, manager.GetQueries(), false, g, nil)
		if _, e := off.Handle(context.Background(), b, i.Invitation.InvitationID, "accept"); e == nil {
			t.Fatal("off accepted")
		}
		if _, e := off.List(context.Background(), b, "received", "", "", 1, 20); e != nil {
			t.Fatal(e)
		}
		if _, e := off.Handle(context.Background(), a, i.Invitation.InvitationID, "cancel"); e != nil {
			t.Fatal(e)
		}
	})
}

func TestInvitationRolesCompetitionAndCommitFailure(t *testing.T) {
	old := api
	srv := httptest.NewServer(server.NewRouter(manager, social.Capabilities{Friends: true, RoomInvitations: true}))
	api = srv
	defer func() { api = old; srv.Close() }()
	t.Run("admin invite owner cancel demotion and kick", func(t *testing.T) {
		a, b, c, d := actor(t, "owner"), actor(t, "admin"), actor(t, "target"), actor(t, "other")
		befriend(t, a, b)
		befriend(t, b, c)
		befriend(t, a, c)
		r := makeGroup(t, a, "public")
		call(t, b, "POST", "/chatroom/joinroom", map[string]string{"roomId": r})
		var rel string
		environment.DB.QueryRow("SELECT member_rel_id FROM chatroom_members WHERE user_id=$1 AND room_id=$2", b, r).Scan(&rel)
		if code, _ := call(t, a, "POST", "/chatroom/"+r+"/members/setadmin", map[string]string{"memberid": rel}); code != 200 {
			t.Fatal(code)
		}
		call(t, a, "POST", "/chatroom/"+r+"/update", map[string]string{"type": "private"})
		i := inviteTo(t, b, c, r)
		if code, _ := call(t, d, "POST", "/chatroom-invitations/"+i.Invitation.InvitationID+"/cancel", nil); code != 404 {
			t.Fatal(code)
		}
		if code, _ := call(t, a, "POST", "/chatroom-invitations/"+i.Invitation.InvitationID+"/cancel", nil); code != 200 {
			t.Fatal(code)
		}
		environment.DB.Exec("UPDATE chatroom_invitations SET handled_at=now()-interval '2 minutes' WHERE invitation_id=$1", i.Invitation.InvitationID)
		i = inviteTo(t, b, c, r)
		call(t, a, "POST", "/chatroom/"+r+"/members/removeadmin", map[string]string{"memberid": rel})
		if code, _ := acceptInvite(t, i.Invitation.InvitationID, c); code != 410 {
			t.Fatal(code)
		}
		if code, _ := call(t, b, "POST", "/chatroom/"+r+"/members/kick", map[string]string{"memberid": rel}); code != 403 {
			t.Fatal(code)
		}
		if code, _ := call(t, a, "POST", "/chatroom/"+r+"/members/kick", map[string]string{"memberid": rel}); code != 200 {
			t.Fatal(code)
		}
		call(t, a, "POST", "/chatroom/"+r+"/members/kick", map[string]string{"memberid": rel})
		assertMembers(t, r, 1)
	})
	t.Run("accept reject cancel competition", func(t *testing.T) {
		a, b := actor(t, "a"), actor(t, "b")
		befriend(t, a, b)
		r := makeGroup(t, a, "public")
		i := inviteTo(t, a, b, r)
		var wg sync.WaitGroup
		for _, action := range []string{"accept", "reject", "cancel", "accept"} {
			wg.Add(1)
			go func(action string) {
				defer wg.Done()
				user := b
				if action == "cancel" {
					user = a
				}
				code, _ := call(t, user, "POST", "/chatroom-invitations/"+i.Invitation.InvitationID+"/"+action, nil)
				if code != 200 && code != 409 {
					t.Error(code)
				}
			}(action)
		}
		wg.Wait()
		var state string
		environment.DB.QueryRow("SELECT status FROM chatroom_invitations WHERE invitation_id=$1", i.Invitation.InvitationID).Scan(&state)
		n := 1
		if state == "accepted" {
			n = 2
		}
		assertMembers(t, r, n)
	})
	t.Run("deferred commit failure emits nothing", func(t *testing.T) {
		a, b := actor(t, "a"), actor(t, "b")
		befriend(t, a, b)
		r := makeGroup(t, a, "private")
		i := inviteTo(t, a, b, r)
		_, e := environment.DB.Exec("CREATE FUNCTION fail_invitation_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.room_id='" + r + "' AND NEW.status='accepted' THEN RAISE EXCEPTION 'commit failure'; END IF; RETURN NEW; END $$; CREATE CONSTRAINT TRIGGER reject_invitation_commit AFTER UPDATE ON chatroom_invitations DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_invitation_commit()")
		if e != nil {
			t.Fatal(e)
		}
		defer environment.DB.Exec("DROP TRIGGER reject_invitation_commit ON chatroom_invitations;DROP FUNCTION fail_invitation_commit()")
		var emitted int
		publisher := func(_ []string, _ events.Event) { emitted++ }
		g := group.New(environment.DB, manager.GetQueries(), publisher)
		s := invite.New(environment.DB, manager.GetQueries(), true, g, publisher)
		if _, e = s.Handle(context.Background(), b, i.Invitation.InvitationID, "accept"); e == nil {
			t.Fatal("commit did not fail")
		}
		if emitted != 0 {
			t.Fatal("precommit event")
		}
		assertMembers(t, r, 1)
		if count(t, "SELECT count(*) FROM chatroom_invitations WHERE invitation_id=$1 AND status='pending'", i.Invitation.InvitationID) != 1 {
			t.Fatal("half commit")
		}
	})
}

func TestInvitationLimitsAndIndependentCapability(t *testing.T) {
	old := api
	srv := httptest.NewServer(server.NewRouter(manager, social.Capabilities{Friends: true, RoomInvitations: true}))
	api = srv
	defer func() { api = old; srv.Close() }()
	a := actor(t, "quota_owner")
	r := makeGroup(t, a, "public")
	targets := []string{}
	for j := 0; j < 11; j++ {
		b := actor(t, fmt.Sprintf("target%d", j))
		befriend(t, b, a)
		targets = append(targets, b)
	}
	var first invite.Result
	for j, b := range targets {
		if j < 10 {
			i := inviteTo(t, a, b, r)
			if j == 0 {
				first = i
			}
		} else {
			if code, out := call(t, a, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": b}); code != 429 || out.BusinessCode != "RATE_LIMITED" {
				t.Fatal(code, out)
			}
		}
	}
	reused := inviteTo(t, a, targets[0], r)
	if reused.Created || reused.Invitation.InvitationID != first.Invitation.InvitationID {
		t.Fatal("retry used quota")
	}
	// Move successful records earlier in the UTC day and fill the daily budget
	// with terminal history. A new target must be rejected without any new row.
	environment.DB.Exec("UPDATE chatroom_invitations SET status='accepted',handled_at=now(),created_at=(date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') WHERE inviter_id=$1", a)
	for j := 0; j < 90; j++ {
		_, e := environment.DB.Exec("INSERT INTO chatroom_invitations(invitation_id,room_id,inviter_id,invitee_id,status,handled_at,created_at) VALUES($1,$2,$3,$4,'accepted',now(),date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')", uuid.NewString(), r, a, targets[0])
		if e != nil {
			t.Fatal(e)
		}
	}
	if code, _ := call(t, a, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": targets[10]}); code != 429 {
		t.Fatal(code)
	}
	if count(t, "SELECT count(*) FROM chatroom_invitations WHERE inviter_id=$1", a) != 100 {
		t.Fatal("daily over budget")
	}
	other, b := actor(t, "other"), actor(t, "friend")
	befriend(t, other, b)
	groupID := makeGroup(t, other, "private")
	independent := httptest.NewServer(server.NewRouter(manager, social.Capabilities{RoomInvitations: true}))
	api = independent
	defer independent.Close()
	if code, out := call(t, other, "GET", "/users/"+b+"/relationship", nil); code != 200 || !strings.Contains(string(out.Data), `"state":"friends"`) {
		t.Fatal(code, out)
	}
	inviteTo(t, other, b, groupID)
}

func TestInvitationNegativeMatrixStaleRecreationAndMemberRollback(t *testing.T) {
	old := api
	srv := httptest.NewServer(server.NewRouter(manager, social.Capabilities{Friends: true, RoomInvitations: true}))
	api = srv
	defer func() { api = old; srv.Close() }()
	a, b, c := actor(t, "owner"), actor(t, "friend"), actor(t, "stranger")
	befriend(t, a, b)
	r := makeGroup(t, a, "public")
	for _, v := range []struct {
		id     string
		status int
		code   string
	}{{a, 400, "INVALID_ARGUMENT"}, {c, 403, "NOT_FRIENDS"}} {
		if code, out := call(t, a, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": v.id}); code != v.status || out.BusinessCode != v.code {
			t.Fatal(code, out)
		}
	}
	call(t, b, "POST", "/chatroom/joinroom", map[string]string{"roomId": r})
	if code, out := call(t, a, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": b}); code != 409 || out.BusinessCode != "ALREADY_MEMBER" {
		t.Fatal(code, out)
	}
	call(t, b, "POST", "/chatroom/leaveroom", map[string]string{"roomId": r})
	environment.DB.Exec("UPDATE users SET account_status='suspended' WHERE user_id=$1", b)
	if code, out := call(t, a, "POST", "/chatroom/"+r+"/invitations", map[string]string{"inviteeId": b}); code != 403 || out.BusinessCode != "ACCOUNT_UNAVAILABLE" {
		t.Fatal(code, out)
	}
	environment.DB.Exec("UPDATE users SET account_status='active' WHERE user_id=$1", b)
	environment.DB.Exec("INSERT INTO user_settings(user_id,enable_friend_request,show_online_status) VALUES($1,false,false)", b)
	oldInvite := inviteTo(t, a, b, r)
	if oldInvite.Invitation.Invitee.OnlineStatus != nil {
		t.Fatal("hidden presence leaked")
	}
	environment.DB.Exec("UPDATE chatroom_invitations SET created_at=now()-interval '8 days',expires_at=now()-interval '1 day' WHERE invitation_id=$1", oldInvite.Invitation.InvitationID)
	fresh := inviteTo(t, a, b, r)
	if fresh.Invitation.InvitationID == oldInvite.Invitation.InvitationID || count(t, "SELECT count(*) FROM chatroom_invitations WHERE invitation_id=$1 AND status='expired'", oldInvite.Invitation.InvitationID) != 1 {
		t.Fatal("stale pending was not replaced with history")
	}
	_, e := environment.DB.Exec("ALTER TABLE chatroom_members ADD CONSTRAINT fail_invitation_member CHECK(room_id<>'" + r + "' OR user_id<>'" + b + "' OR NOT is_active)")
	if e != nil {
		t.Fatal(e)
	}
	defer environment.DB.Exec("ALTER TABLE chatroom_members DROP CONSTRAINT fail_invitation_member")
	if code, _ := acceptInvite(t, fresh.Invitation.InvitationID, b); code != 500 {
		t.Fatal(code)
	}
	assertMembers(t, r, 1)
	if count(t, "SELECT count(*) FROM chatroom_invitations WHERE invitation_id=$1 AND status='pending'", fresh.Invitation.InvitationID) != 1 {
		t.Fatal("member failure half committed")
	}
}
