package server_test

import (
	"chatroombackend/db"
	"chatroombackend/events"
	"chatroombackend/internal/testenv"
	"chatroombackend/server"
	direct "chatroombackend/service/direct"
	social "chatroombackend/service/social"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func befriend(t *testing.T, a, b string) {
	t.Helper()
	r := request(t, a, b)
	if code, out := call(t, b, "POST", "/friends/requests/"+r.Request.RequestID+"/accept", nil); code != 200 {
		t.Fatal(code, out)
	}
}
func createDirect(t *testing.T, a, b string) direct.Conversation {
	t.Helper()
	code, out := call(t, a, "POST", "/direct-conversations", map[string]string{"peerUserId": b})
	if code != 200 && code != 201 {
		t.Fatal(code, out)
	}
	var result direct.CreateResult
	json.Unmarshal(out.Data, &result)
	return result.Conversation
}
func sendDirect(t *testing.T, actor, room, text, key string) direct.Message {
	t.Helper()
	code, out := call(t, actor, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: text, ClientMessageID: key})
	if code != 200 {
		t.Fatal(code, out)
	}
	var msg direct.Message
	json.Unmarshal(out.Data, &msg)
	return msg
}
func TestDirectBatch(t *testing.T) {
	original := api
	enabled := httptest.NewServer(server.NewRouter(manager, social.Capabilities{Friends: true, DirectMessages: true}))
	api = enabled
	defer func() { api = original; enabled.Close() }()
	t.Run("unique pair and concurrent transport", func(t *testing.T) {
		a, b := actor(t, "a"), actor(t, "b")
		befriend(t, a, b)
		var wg sync.WaitGroup
		ids := make(chan string, 10)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				from, to := a, b
				if i%2 == 0 {
					from, to = to, from
				}
				ids <- createDirect(t, from, to).RoomID
			}(i)
		}
		wg.Wait()
		close(ids)
		room := ""
		for id := range ids {
			if room == "" {
				room = id
			}
			if id != room {
				t.Fatal("duplicate rooms")
			}
		}
		if count(t, "SELECT count(*) FROM chatroom_members WHERE room_id=$1", room) != 2 {
			t.Fatal("bad pair")
		}
		key := uuid.NewString()
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); sendDirect(t, a, room, "same", key) }()
		}
		wg.Wait()
		if count(t, "SELECT count(*) FROM messages WHERE room_id=$1", room) != 1 {
			t.Fatal("idempotency failed")
		}
		sendDirect(t, a, room, "same", uuid.NewString())
		if count(t, "SELECT count(*) FROM messages WHERE room_id=$1", room) != 2 {
			t.Fatal("same text was deduplicated")
		}
		if code, out := call(t, a, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: "changed", ClientMessageID: key}); code != 409 || out.BusinessCode != "MESSAGE_IDEMPOTENCY_CONFLICT" {
			t.Fatal(code, out)
		}
	})
	t.Run("history changes edits withdrawal and reading", func(t *testing.T) {
		a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
		if _, err := environment.DB.Exec("UPDATE users SET system_role='admin' WHERE user_id=$1", c); err != nil {
			t.Fatal(err)
		}
		befriend(t, a, b)
		room := createDirect(t, a, b).RoomID
		m := sendDirect(t, a, room, "original", uuid.NewString())
		if code, _ := call(t, c, "GET", "/direct-conversations/"+room, nil); code != 404 {
			t.Fatal("third reader", code)
		}
		if code, _ := call(t, b, "POST", "/direct-conversations/"+room+"/messages/"+m.MessageID+"/edit", map[string]string{"text": "bad", "expectedRevision": m.Revision}); code != 403 {
			t.Fatal("edited other author", code)
		}
		code, out := call(t, a, "POST", "/direct-conversations/"+room+"/messages/"+m.MessageID+"/edit", map[string]string{"text": "edited", "expectedRevision": m.Revision})
		if code != 200 {
			t.Fatal(code, out)
		}
		var edited direct.Message
		json.Unmarshal(out.Data, &edited)
		if edited.Time != m.Time || edited.EditedAt == nil {
			t.Fatal("sent time moved")
		}
		if code, _ = call(t, a, "POST", "/direct-conversations/"+room+"/messages/"+m.MessageID+"/edit", map[string]string{"text": "stale", "expectedRevision": m.Revision}); code != 409 {
			t.Fatal("stale edit accepted")
		}
		key := uuid.NewString()
		code, out = call(t, b, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: "reply", ClientMessageID: key, QuotedMessageID: m.MessageID})
		if code != 200 {
			t.Fatal(code, out)
		}
		var reply direct.Message
		json.Unmarshal(out.Data, &reply)
		if reply.Quote == nil || reply.Quote.Text != "edited" {
			t.Fatal("missing quote")
		}
		if code, _ = call(t, a, "POST", "/direct-conversations/"+room+"/messages/"+m.MessageID+"/delete", nil); code != 200 {
			t.Fatal(code)
		}
		_, out = call(t, b, "GET", "/direct-conversations/"+room+"/changes?afterRevision=1&pageSize=1", nil)
		var changes direct.Changes
		json.Unmarshal(out.Data, &changes)
		if !changes.HasMore || changes.Changes[0].Message.Text != "" || !changes.Changes[0].Message.IsDeleted {
			t.Fatal("old replay resurrected deleted text", string(out.Data))
		}
		_, out = call(t, b, "GET", "/direct-conversations/"+room+"/messages", nil)
		var history direct.History
		json.Unmarshal(out.Data, &history)
		if len(history.Messages) != 2 || history.Messages[1].Quote == nil || !history.Messages[1].Quote.IsDeleted {
			t.Fatal(string(out.Data))
		}
		if code, _ = call(t, a, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: "original", ClientMessageID: m.ClientMessageID}); code != 200 {
			t.Fatal("withdraw retry", code)
		}
		if code, _ = call(t, a, "POST", "/direct-conversations/"+room+"/read", map[string]string{"lastReadMessageId": reply.MessageID}); code != 200 {
			t.Fatal(code)
		}
		_, out = call(t, a, "GET", "/direct-conversations/"+room, nil)
		var detail direct.Conversation
		json.Unmarshal(out.Data, &detail)
		if detail.UnreadCount != 0 || detail.LastReadSeq != reply.DirectSeq {
			t.Fatal(string(out.Data))
		}
		call(t, a, "POST", "/direct-conversations/"+room+"/read", map[string]string{"lastReadMessageId": m.MessageID})
		_, out = call(t, a, "GET", "/direct-conversations/"+room, nil)
		json.Unmarshal(out.Data, &detail)
		if detail.LastReadSeq != reply.DirectSeq {
			t.Fatal("read moved back")
		}
	})
	t.Run("friend removal readonly and original reuse", func(t *testing.T) {
		a, b := actor(t, "a"), actor(t, "b")
		befriend(t, a, b)
		room := createDirect(t, a, b).RoomID
		m := sendDirect(t, a, room, "kept", uuid.NewString())
		call(t, a, "DELETE", "/friends/"+b, nil)
		if code, _ := call(t, a, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: "new", ClientMessageID: uuid.NewString()}); code != 403 {
			t.Fatal(code)
		}
		if code, _ := call(t, a, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: "kept", ClientMessageID: m.ClientMessageID}); code != 200 {
			t.Fatal("committed result lost", code)
		}
		if code, _ := call(t, b, "GET", "/direct-conversations/"+room+"/messages", nil); code != 200 {
			t.Fatal(code)
		}
		befriend(t, a, b)
		if createDirect(t, b, a).RoomID != room {
			t.Fatal("history forked")
		}
	})
	t.Run("ws ack replay and own read delivery", func(t *testing.T) {
		a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
		befriend(t, a, b)
		room := createDirect(t, a, b).RoomID
		one, two, peer := dial(t, a), dial(t, a), dial(t, b)
		defer one.Close()
		defer two.Close()
		defer peer.Close()
		key := uuid.NewString()
		one.WriteJSON(map[string]any{"type": "message", "action": "send", "data": map[string]any{"roomId": room, "messageType": "text", "text": "ws", "clientMessageId": key}})
		ack := readKind(t, one, "message")
		for ack["action"] != "ack" {
			ack = readKind(t, one, "message")
		}
		readKind(t, two, "message")
		readKind(t, peer, "message")
		msg := sendDirect(t, a, room, "ws", key)
		if count(t, "SELECT count(*) FROM messages WHERE room_id=$1", room) != 1 {
			t.Fatal("ws/http duplicate")
		}
		call(t, a, "POST", "/direct-conversations/"+room+"/read", map[string]string{"lastReadMessageId": msg.MessageID})
		readKind(t, two, "direct")
		bad := dial(t, c)
		defer bad.Close()
		bad.WriteJSON(map[string]any{"type": "message", "action": "send", "data": map[string]any{"roomId": room, "messageType": "text", "text": "attack", "clientMessageId": uuid.NewString()}})
		readKind(t, bad, "error")
		one.WriteJSON(map[string]any{"type": "room", "action": "join", "data": map[string]string{"roomId": room}})
		readKind(t, one, "room")
		if count(t, "SELECT online_count FROM chatrooms WHERE room_id=$1", room) != 0 {
			t.Fatal("direct touched group presence")
		}
	})
	t.Run("limits errors snapshot cursor and failed commit", func(t *testing.T) {
		a, b := actor(t, "a"), actor(t, "b")
		befriend(t, a, b)
		room := createDirect(t, a, b).RoomID
		if code, _ := call(t, a, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: strings.Repeat("界", 2001), ClientMessageID: uuid.NewString()}); code != 400 {
			t.Fatal(code)
		}
		for _, path := range []string{"/direct-conversations/" + room + "/messages?beforeSeq=1&afterSeq=1", "/direct-conversations/" + room + "/changes?afterRevision=999", "/users/me/direct-conversations?cursor=bad"} {
			if code, _ := call(t, a, "GET", path, nil); code != 400 {
				t.Fatal(path, code)
			}
		}
		_, err := environment.DB.Exec("CREATE FUNCTION direct_fail_test() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'commit fault'; END $$ LANGUAGE plpgsql; CREATE CONSTRAINT TRIGGER direct_fail_test AFTER INSERT ON direct_message_changes DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION direct_fail_test()")
		if err != nil {
			t.Fatal(err)
		}
		if code, _ := call(t, a, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: "rollback", ClientMessageID: uuid.NewString()}); code != 500 {
			t.Fatal(code)
		}
		environment.DB.Exec("DROP TRIGGER direct_fail_test ON direct_message_changes; DROP FUNCTION direct_fail_test()")
		if count(t, "SELECT last_message_seq FROM direct_pairs WHERE room_id=$1", room) != 0 || count(t, "SELECT count(*) FROM messages WHERE room_id=$1", room) != 0 {
			t.Fatal("half message")
		}
	})
	t.Run("disabled peer cross room and group mute", func(t *testing.T) {
		a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
		befriend(t, a, b)
		befriend(t, a, c)
		room := createDirect(t, a, b).RoomID
		other := createDirect(t, a, c).RoomID
		original := sendDirect(t, c, other, "private", uuid.NewString())
		if code, _ := call(t, a, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: "cross", ClientMessageID: uuid.NewString(), QuotedMessageID: original.MessageID}); code != 400 {
			t.Fatal(code)
		}
		if code, _ := call(t, a, "POST", "/direct-conversations/"+room+"/read", map[string]string{"lastReadMessageId": original.MessageID}); code != 400 {
			t.Fatal(code)
		}
		environment.DB.Exec("UPDATE users SET account_status='suspended' WHERE user_id=$1", b)
		if code, _ := call(t, a, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: "no", ClientMessageID: uuid.NewString()}); code != 403 {
			t.Fatal(code)
		}
		if code, out := call(t, a, "GET", "/direct-conversations/"+room, nil); code != 200 || !strings.Contains(string(out.Data), "peer_unavailable") {
			t.Fatal(code, out)
		}
		environment.DB.Exec("UPDATE users SET account_status='active' WHERE user_id=$1", b)
		// A mute in an unrelated group cannot stop a direct message.
		environment.DB.Exec("INSERT INTO chatroom_members(user_id,room_id,mute_status) VALUES($1,$2,'muted')", a, users["public"])
		sendDirect(t, a, room, "not group muted", uuid.NewString())
		environment.DB.Exec("INSERT INTO global_mute_records(muted_user_id,expires_at) VALUES($1,now()+interval '1 hour')", a)
		if code, _ := call(t, a, "POST", "/direct-conversations/"+room+"/messages", direct.SendInput{Type: "text", Text: "globally blocked", ClientMessageID: uuid.NewString()}); code != 403 {
			t.Fatal(code)
		}
	})
	time.Sleep(100 * time.Millisecond)
}
func TestDirectFeatureOffServiceAndListPagination(t *testing.T) {
	a, b := actor(t, "a"), actor(t, "b")
	befriend(t, a, b)
	svc := direct.New(environment.DB, db.New(environment.DB), true, func([]string, events.Event) {})
	room, err := svc.Create(context.Background(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := svc.Send(context.Background(), b, room.Conversation.RoomID, direct.SendInput{Type: "text", Text: "history", ClientMessageID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	off := direct.New(environment.DB, db.New(environment.DB), false, nil)
	if _, err = off.Create(context.Background(), a, b); err == nil {
		t.Fatal("off create")
	}
	if _, err = off.Send(context.Background(), a, room.Conversation.RoomID, direct.SendInput{Type: "text", Text: "no", ClientMessageID: uuid.NewString()}); err == nil {
		t.Fatal("off send")
	}
	detail, err := off.Detail(context.Background(), a, room.Conversation.RoomID)
	if err != nil || detail.CanSend {
		t.Fatal(detail, err)
	}
	if _, err = off.Read(context.Background(), a, room.Conversation.RoomID, msg.MessageID); err != nil {
		t.Fatal(err)
	}
	list, err := off.List(context.Background(), a, "", 1)
	if err != nil || len(list.List) != 1 {
		t.Fatal(list, err)
	}
}

func TestDirectUpgradeRecoveryBaseline(t *testing.T) {
	env, err := testenv.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	if err = testenv.Migrate(env.DB, 5); err != nil {
		t.Fatal(err)
	}
	var a, b, room string
	env.DB.QueryRow("INSERT INTO users(username,hashed_password) VALUES('upgrade_a','unused') RETURNING user_id").Scan(&a)
	env.DB.QueryRow("INSERT INTO users(username,hashed_password) VALUES('upgrade_b','unused') RETURNING user_id").Scan(&b)
	tx, err := env.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = tx.QueryRow("INSERT INTO chatrooms(room_name,room_type,conversation_kind,member_count) VALUES('upgrade','private_invite_only','direct',2) RETURNING room_id").Scan(&room); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("INSERT INTO direct_pairs(room_id,user_id_1,user_id_2) VALUES($1,$2,$3)", room, a, b); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("INSERT INTO chatroom_members(room_id,user_id) VALUES($1,$2),($1,$3)", room, a, b); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("INSERT INTO messages(room_id,sender_id,content,message_type,client_message_id,request_hash,direct_seq) VALUES($1,$2,'old','text',$3,$4,7)", room, a, uuid.NewString(), strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = testenv.Migrate(env.DB, 6); err != nil {
		t.Fatal(err)
	}
	svc := direct.New(env.DB, db.New(env.DB), true, nil)
	history, err := svc.History(context.Background(), a, room, 0, 0, false, 50)
	if err != nil || history.LatestSeq != "7" || history.ChangeRevision != "7" {
		t.Fatal(history, err)
	}
	changes, err := svc.Changes(context.Background(), a, room, 0, 50)
	if err != nil || len(changes.Changes) != 1 || changes.NextCursor != "7" {
		t.Fatal(changes, err)
	}
	if err = testenv.Rollback(env.DB, 5); err == nil {
		t.Fatal("recovery history discarded by rollback")
	}
}

func TestDirectSendLimitAndIdempotentAllowance(t *testing.T) {
	a, b := actor(t, "a"), actor(t, "b")
	befriend(t, a, b)
	svc := direct.New(environment.DB, db.New(environment.DB), true, nil)
	created, err := svc.Create(context.Background(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	var last direct.SendInput
	for i := 0; i < 30; i++ {
		last = direct.SendInput{Type: "text", Text: "limited", ClientMessageID: uuid.NewString()}
		if _, err = svc.Send(context.Background(), a, created.Conversation.RoomID, last); err != nil {
			t.Fatal(i, err)
		}
	}
	if _, err = svc.Send(context.Background(), a, created.Conversation.RoomID, direct.SendInput{Type: "text", Text: "over", ClientMessageID: uuid.NewString()}); err == nil {
		t.Fatal("send limit not enforced")
	}
	if _, err = svc.Send(context.Background(), a, created.Conversation.RoomID, last); err != nil {
		t.Fatal("idempotent retry rejected at limit", err)
	}
	if count(t, "SELECT count(*) FROM messages WHERE room_id=$1", created.Conversation.RoomID) != 30 {
		t.Fatal("limit or retry changed stored messages")
	}
}

func TestDirectCreationLimitAndPairReuse(t *testing.T) {
	a := actor(t, "creator")
	svc := direct.New(environment.DB, db.New(environment.DB), true, nil)
	firstRoom := ""
	firstPeer := ""
	for i := 0; i < 11; i++ {
		p := actor(t, fmt.Sprintf("peer_%d", i))
		if _, err := environment.DB.Exec("INSERT INTO friends(user_id_1,user_id_2) VALUES($1,$2)", a, p); err != nil {
			t.Fatal(err)
		}
		result, err := svc.Create(context.Background(), a, p)
		if i < 10 && err != nil {
			t.Fatal(i, err)
		}
		if i == 10 && err == nil {
			t.Fatal("creation limit missing")
		}
		if i == 0 {
			firstRoom = result.Conversation.RoomID
			firstPeer = p
		}
	}
	again, err := svc.Create(context.Background(), a, firstPeer)
	if err != nil || again.Created || again.Conversation.RoomID != firstRoom {
		t.Fatal("pair reuse counted as creation", again, err)
	}
}
