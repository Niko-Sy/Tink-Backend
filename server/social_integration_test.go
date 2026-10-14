package server_test

import (
	"bytes"
	"chatroombackend/db"
	"chatroombackend/events"
	"chatroombackend/internal/testenv"
	"chatroombackend/middleware"
	"chatroombackend/server"
	social "chatroombackend/service/social"
	"chatroombackend/utils"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var environment *testenv.Environment
var manager *middleware.DBManager
var api *httptest.Server
var users map[string]string

func TestMain(m *testing.M) {
	gin.SetMode(gin.ReleaseMode)
	var err error
	environment, err = testenv.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = testenv.Migrate(environment.DB, 7); err != nil {
		environment.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	users, err = testenv.Seed(environment.DB)
	if err != nil {
		environment.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg := middleware.DefaultDBConfig()
	cfg.DSN = environment.DSN
	manager, err = middleware.NewDBManager(cfg)
	if err != nil {
		environment.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	middleware.JWTSecretKey = []byte("isolated-integration-key-only")
	uploadDir, err := os.MkdirTemp("", "tink-test-uploads-")
	if err != nil {
		environment.Close()
		panic(err)
	}
	utils.InitImageUploader(&utils.ImageConfig{UploadDir: uploadDir, MaxSize: 5 << 20, AllowedTypes: []string{"image/png"}, URLPrefix: "/static/images"})
	api = httptest.NewServer(server.NewRouter(manager, social.Capabilities{Friends: true}))
	code := m.Run()
	api.Close()
	manager.Close()
	environment.Close()
	os.RemoveAll(uploadDir)
	os.Exit(code)
}

type envelope struct {
	Code         int             `json:"code"`
	BusinessCode string          `json:"businessCode"`
	Data         json.RawMessage `json:"data"`
}

func call(t *testing.T, id, method, path string, body any) (int, envelope) {
	t.Helper()
	var encoded []byte
	if body != nil {
		encoded, _ = json.Marshal(body)
	}
	req, err := http.NewRequest(method, api.URL+"/api/v1"+path, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if id != "" {
		token, _ := middleware.GenerateToken(id, "test")
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := api.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var value envelope
	if err = json.NewDecoder(res.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, value
}
func actor(t *testing.T, name string) string {
	t.Helper()
	var id string
	err := environment.DB.QueryRow("INSERT INTO users(username,hashed_password,nickname) VALUES($1,'unused',$1) RETURNING user_id", t.Name()+"_"+name).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func request(t *testing.T, a, b string) social.RequestResult {
	t.Helper()
	status, out := call(t, a, "POST", "/friends/requests", map[string]string{"toUserId": b, "message": "hello"})
	if status != 201 {
		t.Fatalf("request status=%d code=%s body=%s", status, out.BusinessCode, out.Data)
	}
	var result social.RequestResult
	if err := json.Unmarshal(out.Data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := environment.DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestFriendLifecyclePrivacyAndPagination(t *testing.T) {
	a, b := actor(t, "a"), actor(t, "b")
	status, out := call(t, a, "GET", "/users/search?userId="+b, nil)
	if status != 200 {
		t.Fatal(status)
	}
	if strings.Contains(string(out.Data), "email") || strings.Contains(string(out.Data), "password") {
		t.Fatal("private fields returned")
	}
	r := request(t, a, b)
	status, out = call(t, a, "POST", "/friends/requests", map[string]string{"toUserId": b})
	if status != 200 {
		t.Fatal("same pending not idempotent")
	}
	var repeated social.RequestResult
	json.Unmarshal(out.Data, &repeated)
	if repeated.Request.RequestID != r.Request.RequestID {
		t.Fatal("new duplicate")
	}
	status, out = call(t, b, "POST", "/friends/requests", map[string]string{"toUserId": a})
	if status != 409 || out.BusinessCode != "INCOMING_REQUEST_PENDING" {
		t.Fatal(status, out)
	}
	status, _ = call(t, b, "GET", "/users/me/friend-requests?direction=received&status=pending", nil)
	if status != 200 {
		t.Fatal(status)
	}
	status, _ = call(t, b, "POST", "/friends/requests/"+r.Request.RequestID+"/accept", map[string]any{})
	if status != 200 {
		t.Fatal(status)
	}
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		status, out = call(t, pair[0], "GET", "/users/me/friends?pageSize=1", nil)
		if status != 200 {
			t.Fatal(status)
		}
		var page social.Page[social.Contact]
		json.Unmarshal(out.Data, &page)
		if page.Total != 1 || len(page.List) != 1 || page.List[0].Profile.UserID != pair[1] || page.List[0].CanMessage {
			t.Fatalf("wrong list %s", out.Data)
		}
	}
	status, _ = call(t, a, "GET", "/friends/"+b, nil)
	if status != 200 {
		t.Fatal(status)
	}
	status, out = call(t, a, "GET", "/users/me/friends?q=does-not-exist", nil)
	if status != 200 || !strings.Contains(string(out.Data), `"total":0`) {
		t.Fatal(string(out.Data))
	}
	status, _ = call(t, a, "DELETE", "/friends/"+b, nil)
	if status != 200 {
		t.Fatal(status)
	}
	status, _ = call(t, a, "DELETE", "/friends/"+b, nil)
	if status != 200 {
		t.Fatal(status)
	}
	status, _ = call(t, b, "POST", "/friends/requests/"+r.Request.RequestID+"/accept", map[string]any{})
	if status != 200 {
		t.Fatal(status)
	}
	if count(t, "SELECT count(*) FROM friends WHERE user_id_1=LEAST($1::varchar,$2::varchar) AND user_id_2=GREATEST($1::varchar,$2::varchar)", a, b) != 0 {
		t.Fatal("old accept resurrected friendship")
	}
	_ = request(t, a, b)
}
func TestRequestAuthorizationStatesAndCooldown(t *testing.T) {
	a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
	r := request(t, a, b)
	if status, _ := call(t, c, "POST", "/friends/requests/"+r.Request.RequestID+"/accept", nil); status != 404 {
		t.Fatal(status)
	}
	if status, _ := call(t, a, "POST", "/friends/requests/"+r.Request.RequestID+"/accept", nil); status != 403 {
		t.Fatal(status)
	}
	if status, _ := call(t, b, "POST", "/friends/requests/"+r.Request.RequestID+"/cancel", nil); status != 403 {
		t.Fatal(status)
	}
	if status, _ := call(t, b, "POST", "/friends/requests/"+r.Request.RequestID+"/reject", nil); status != 200 {
		t.Fatal(status)
	}
	if status, _ := call(t, b, "POST", "/friends/requests/"+r.Request.RequestID+"/accept", nil); status != 409 {
		t.Fatal(status)
	}
	if status, out := call(t, b, "POST", "/friends/requests", map[string]string{"toUserId": a}); status != 429 || out.BusinessCode != "REQUEST_COOLDOWN" {
		t.Fatal(status, out)
	}
	_, err := environment.DB.Exec("UPDATE friend_request SET handled_at=now()-interval '25 hours' WHERE request_id=$1", r.Request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	next := request(t, a, b)
	if next.Request.RequestID == r.Request.RequestID {
		t.Fatal("request history reused")
	}
	if status, _ := call(t, a, "POST", "/friends/requests/"+next.Request.RequestID+"/cancel", nil); status != 200 {
		t.Fatal(status)
	}
	if status, _ := call(t, a, "POST", "/friends/requests", map[string]string{"toUserId": b}); status != 429 {
		t.Fatal(status)
	}
	if count(t, "SELECT count(*) FROM friend_request WHERE from_user_id=$1 AND to_user_id=$2", a, b) != 2 {
		t.Fatal("history lost")
	}
}
func TestUnavailableAndHiddenProfiles(t *testing.T) {
	a := actor(t, "a")
	for _, b := range []string{users["disabled"], users["closed"]} {
		if status, _ := call(t, a, "POST", "/friends/requests", map[string]string{"toUserId": b}); status != 403 {
			t.Fatal(status)
		}
	}
	if status, _ := call(t, a, "POST", "/friends/requests", map[string]string{"toUserId": a}); status != 400 {
		t.Fatal(status)
	}
	if status, _ := call(t, "", "GET", "/users/me/friends", nil); status != 401 {
		t.Fatal(status)
	}
	r := request(t, a, users["hidden"])
	if status, _ := call(t, users["hidden"], "POST", "/friends/requests/"+r.Request.RequestID+"/accept", nil); status != 200 {
		t.Fatal(status)
	}
	_, out := call(t, a, "GET", "/friends/"+users["hidden"], nil)
	var c social.Contact
	json.Unmarshal(out.Data, &c)
	if c.Profile.OnlineStatus != nil {
		t.Fatal("hidden online status leaked")
	}
	_, err := environment.DB.Exec("UPDATE users SET account_status='suspended' WHERE user_id=$1", users["hidden"])
	if err != nil {
		t.Fatal(err)
	}
	defer environment.DB.Exec("UPDATE users SET account_status='active' WHERE user_id=$1", users["hidden"])
	_, out = call(t, a, "GET", "/users/me/friends", nil)
	var page social.Page[social.Contact]
	json.Unmarshal(out.Data, &page)
	if page.Total != 1 || page.List[0].Profile.Availability != "unavailable" || page.List[0].Profile.Username != "" {
		t.Fatal(string(out.Data))
	}
	for _, path := range []string{"/users/me/friends?pageSize=101", "/users/me/friend-requests?direction=unknown", "/users/search?username=a&userId=" + a} {
		if status, _ := call(t, a, "GET", path, nil); status != 400 {
			t.Fatal(path, status)
		}
	}
}
func TestConcurrentRequestsAndActions(t *testing.T) {
	a, b := actor(t, "a"), actor(t, "b")
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			from, to := a, b
			if i%2 == 0 {
				from, to = to, from
			}
			status, _ := call(t, from, "POST", "/friends/requests", map[string]string{"toUserId": to})
			if status != 200 && status != 201 && status != 409 {
				t.Errorf("create status %d", status)
			}
		}(i)
	}
	wg.Wait()
	if count(t, "SELECT count(*) FROM friend_request WHERE status='pending' AND from_user_id IN($1,$2) AND to_user_id IN($1,$2)", a, b) != 1 {
		t.Fatal("duplicate pending")
	}
	var id, receiver string
	if err := environment.DB.QueryRow("SELECT request_id,to_user_id FROM friend_request WHERE status='pending' AND from_user_id IN($1,$2) AND to_user_id IN($1,$2)", a, b).Scan(&id, &receiver); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"accept", "reject", "accept", "reject"} {
		wg.Add(1)
		go func(action string) {
			defer wg.Done()
			status, _ := call(t, receiver, "POST", "/friends/requests/"+id+"/"+action, nil)
			if status != 200 && status != 409 {
				t.Errorf("handle status %d", status)
			}
		}(action)
	}
	wg.Wait()
	var state string
	environment.DB.QueryRow("SELECT status FROM friend_request WHERE request_id=$1", id).Scan(&state)
	n := count(t, "SELECT count(*) FROM friends WHERE user_id_1=LEAST($1::varchar,$2::varchar) AND user_id_2=GREATEST($1::varchar,$2::varchar)", a, b)
	if state == "accepted" && n != 1 || state == "rejected" && n != 0 {
		t.Fatalf("state %s friendships %d", state, n)
	}
}
func TestAcceptFailureRollsBack(t *testing.T) {
	a, b := actor(t, "a"), actor(t, "b")
	r := request(t, a, b)
	_, err := environment.DB.Exec("CREATE FUNCTION fail_friend_test() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$ LANGUAGE plpgsql; CREATE TRIGGER fail_friend_test BEFORE INSERT ON friends FOR EACH ROW EXECUTE FUNCTION fail_friend_test()")
	if err != nil {
		t.Fatal(err)
	}
	defer environment.DB.Exec("DROP TRIGGER fail_friend_test ON friends; DROP FUNCTION fail_friend_test()")
	if status, _ := call(t, b, "POST", "/friends/requests/"+r.Request.RequestID+"/accept", nil); status != 500 {
		t.Fatal(status)
	}
	var state string
	environment.DB.QueryRow("SELECT status FROM friend_request WHERE request_id=$1", r.Request.RequestID).Scan(&state)
	if state != "pending" {
		t.Fatal("half accepted")
	}
	if count(t, "SELECT count(*) FROM friends WHERE user_id_1=$1 OR user_id_2=$1", a) != 0 {
		t.Fatal("half friendship")
	}
}
func directFixture(t *testing.T, a, b string) string {
	t.Helper()
	tx, err := environment.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var room string
	if err = tx.QueryRow("INSERT INTO chatrooms(room_name,room_type,conversation_kind,member_count) VALUES('internal','private_invite_only','direct',2) RETURNING room_id").Scan(&room); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("INSERT INTO direct_pairs(room_id,user_id_1,user_id_2) VALUES($1,LEAST($2::varchar,$3::varchar),GREATEST($2::varchar,$3::varchar))", room, a, b); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("INSERT INTO chatroom_members(user_id,room_id) VALUES($1,$3),($2,$3)", a, b, room); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return room
}
func dial(t *testing.T, id string) *websocket.Conn {
	t.Helper()
	status, result := call(t, id, "POST", "/auth/ws-ticket", nil)
	if status != 200 {
		t.Fatal(status, result)
	}
	var ticket struct {
		Ticket string `json:"ticket"`
	}
	json.Unmarshal(result.Data, &ticket)
	conn, _, err := websocket.DefaultDialer.Dial(strings.Replace(api.URL, "http", "ws", 1)+"/ws?ticket="+ticket.Ticket, nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}
func readKind(t *testing.T, c *websocket.Conn, kind string) map[string]any {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(4 * time.Second))
	for {
		var msg map[string]any
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg["type"] == kind {
			return msg
		}
	}
}
func TestDirectGuardsAndConstraints(t *testing.T) {
	a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
	room := directFixture(t, a, b)
	for _, id := range []string{a, c} {
		for _, route := range []struct {
			method, path string
			body         any
		}{{"GET", "/chatroom/" + room + "/info", nil}, {"GET", "/chatroom/" + room + "/messages", nil}, {"GET", "/chatroom/" + room + "/members/memberlist", nil}, {"POST", "/chatroom/joinroom", map[string]string{"roomId": room}}, {"POST", "/chatroom/leaveroom", map[string]string{"roomId": room}}, {"POST", "/chatroom/" + room + "/update", map[string]string{"name": "bad"}}, {"POST", "/chatroom/" + room + "/delete", nil}, {"POST", "/chatroom/" + room + "/uploadimage", nil}, {"POST", "/chatroom/" + room + "/members/mute", nil}} {
			if status, _ := call(t, id, route.method, route.path, route.body); status != 404 {
				t.Fatalf("guard %s status %d", route.path, status)
			}
		}
	}
	_, out := call(t, a, "GET", "/users/me/chatrooms", nil)
	if strings.Contains(string(out.Data), room) {
		t.Fatal("direct leaked in group list")
	}
	conn := dial(t, c)
	defer conn.Close()
	conn.WriteJSON(map[string]any{"type": "typing", "data": map[string]any{"roomId": room, "typing": true}})
	readKind(t, conn, "error")
	for _, query := range []string{"INSERT INTO chatroom_members(user_id,room_id) VALUES($1,$2)", "UPDATE chatroom_members SET member_role='owner' WHERE user_id=$1 AND room_id=$2"} {
		id := a
		if strings.HasPrefix(query, "INSERT") {
			id = c
		}
		if _, err := environment.DB.Exec(query, id, room); err == nil {
			t.Fatal("direct invariant bypassed")
		}
	}
	if _, err := environment.DB.Exec("INSERT INTO chatroom_invitations(invitation_id,room_id,inviter_id,invitee_id) VALUES('00000000-0000-0000-0000-000000000001',$1,$2,$3)", room, a, c); err == nil {
		t.Fatal("direct invitation allowed")
	}
}
func TestMultipleConnectionsAndRealGroupLifecycle(t *testing.T) {
	a, b := actor(t, "a"), actor(t, "b")
	one, two := dial(t, b), dial(t, b)
	defer one.Close()
	defer two.Close()
	r := request(t, a, b)
	readKind(t, one, "friend_request")
	readKind(t, two, "friend_request")
	if status, _ := call(t, b, "POST", "/friends/requests/"+r.Request.RequestID+"/accept", nil); status != 200 {
		t.Fatal(status)
	}
	readKind(t, one, "friendship")
	readKind(t, two, "friendship")
	one.Close()
	time.Sleep(100 * time.Millisecond)
	if status, _ := call(t, a, "DELETE", "/friends/"+b, nil); status != 200 {
		t.Fatal(status)
	}
	readKind(t, two, "friendship")
	var state string
	environment.DB.QueryRow("SELECT online_status FROM users WHERE user_id=$1", b).Scan(&state)
	if state != "online" {
		t.Fatal("second tab disconnected")
	}
	if status, _ := call(t, b, "POST", "/chatroom/joinroom", map[string]string{"roomId": users["public"]}); status != 200 {
		t.Fatal(status)
	}
	for i := 0; i < 2; i++ {
		two.WriteJSON(map[string]any{"type": "room", "action": "join", "data": map[string]any{"roomId": users["public"]}})
		readKind(t, two, "room")
	}
	if count(t, "SELECT online_count FROM chatrooms WHERE room_id=$1", users["public"]) != 1 {
		t.Fatal("subscription double counted")
	}
	status, out := call(t, b, "POST", "/chatroom/"+users["public"]+"/messages", map[string]string{"type": "text", "text": "regression"})
	if status != 200 {
		t.Fatal(status, out)
	}
	readKind(t, two, "message")
	status, out = call(t, b, "GET", "/chatroom/"+users["public"]+"/messages", nil)
	if status != 200 || !strings.Contains(string(out.Data), "regression") {
		t.Fatal(status, string(out.Data))
	}
	if status, _ := call(t, b, "POST", "/chatroom/leaveroom", map[string]string{"roomId": users["public"]}); status != 200 {
		t.Fatal(status)
	}
	if count(t, "SELECT online_count FROM chatrooms WHERE room_id=$1", users["public"]) != 0 {
		t.Fatal("revoked subscription retained")
	}
}
func TestCapabilitiesAndDisabledAPI(t *testing.T) {
	_, out := call(t, users["alice"], "GET", "/users/me/social-capabilities", nil)
	if !strings.Contains(string(out.Data), `"directMessages":false`) {
		t.Fatal(string(out.Data))
	}
	disabled := httptest.NewServer(server.NewRouter(manager, social.Capabilities{}))
	defer disabled.Close()
	token, _ := middleware.GenerateToken(users["alice"], "alice")
	req, _ := http.NewRequest("GET", disabled.URL+"/api/v1/users/me/friends", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 503 {
		t.Fatal(res.StatusCode)
	}
}
func TestLegacyMigrationAuditAndProtectedRollback(t *testing.T) {
	env, err := testenv.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	if err = testenv.Migrate(env.DB, 2); err != nil {
		t.Fatal(err)
	}
	var a, b string
	env.DB.QueryRow("INSERT INTO users(username,hashed_password) VALUES('old_a','unused') RETURNING user_id").Scan(&a)
	env.DB.QueryRow("INSERT INTO users(username,hashed_password) VALUES('old_b','unused') RETURNING user_id").Scan(&b)
	if _, err = env.DB.Exec("INSERT INTO friend_request(from_user_id,to_user_id) VALUES($1,$2),($2,$1),($1,$1)", a, b); err != nil {
		t.Fatal(err)
	}
	if err = testenv.Migrate(env.DB, 7); err != nil {
		t.Fatal(err)
	}
	var n int
	env.DB.QueryRow("SELECT count(*) FROM friend_request_migration_audit").Scan(&n)
	if n != 2 {
		t.Fatal("legacy repairs not audited", n)
	}
	if err = testenv.Rollback(env.DB, 2); err == nil {
		t.Fatal("cancelled history rollback permitted")
	}
	env.DB.Exec("DELETE FROM friend_request")
	if err = testenv.Rollback(env.DB, 2); err != nil {
		t.Fatal(err)
	}
	if err = testenv.Migrate(env.DB, 7); err != nil {
		t.Fatal(err)
	}
}

func TestServiceLimitsAndCommitFailure(t *testing.T) {
	a, b, c := actor(t, "a"), actor(t, "b"), actor(t, "c")
	var mu sync.Mutex
	published := 0
	svc := social.New(environment.DB, db.New(environment.DB), social.Capabilities{Friends: true}, func(_ []string, _ events.Event) { mu.Lock(); published++; mu.Unlock() })
	svc.SearchLimit = 1
	svc.DailyLimit = 1
	if _, err := svc.Search(context.Background(), a, b, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Search(context.Background(), a, b, ""); err == nil {
		t.Fatal("search limit missing")
	}
	r, err := svc.CreateRequest(context.Background(), a, b, strings.Repeat("界", 200))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.CreateRequest(context.Background(), a, b, ""); err != nil {
		t.Fatal("idempotent retry consumed daily limit", err)
	}
	if _, err = svc.CreateRequest(context.Background(), a, c, ""); err == nil {
		t.Fatal("daily limit missing")
	}
	if _, err = svc.CreateRequest(context.Background(), b, c, strings.Repeat("界", 201)); err == nil {
		t.Fatal("Unicode length not validated")
	}
	_, err = environment.DB.Exec("CREATE FUNCTION fail_commit_test() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'injected commit failure'; END $$ LANGUAGE plpgsql; CREATE CONSTRAINT TRIGGER fail_commit_test AFTER INSERT ON friends DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_commit_test()")
	if err != nil {
		t.Fatal(err)
	}
	defer environment.DB.Exec("DROP TRIGGER fail_commit_test ON friends; DROP FUNCTION fail_commit_test()")
	if _, err = svc.HandleRequest(context.Background(), b, r.Request.RequestID, "accept"); err == nil {
		t.Fatal("commit failure hidden")
	}
	mu.Lock()
	n := published
	mu.Unlock()
	if n != 1 {
		t.Fatal("success event emitted before commit", n)
	}
	var state string
	environment.DB.QueryRow("SELECT status FROM friend_request WHERE request_id=$1", r.Request.RequestID).Scan(&state)
	if state != "pending" {
		t.Fatal("commit failed but request changed")
	}
}

func TestAuthenticationGroupAdministrationAndAvatarRegression(t *testing.T) {
	status, out := call(t, "", "POST", "/auth/register", map[string]string{"username": "regression_user", "password": "Testpass123!", "nickname": "Regression", "email": "regression@example.test"})
	if status != 200 {
		t.Fatal("register", status, out)
	}
	status, out = call(t, "", "POST", "/auth/login", map[string]string{"username": "regression_user", "password": "Testpass123!"})
	if status != 200 {
		t.Fatal("login", status, out)
	}
	var login struct {
		User struct {
			UserID string `json:"userId"`
		} `json:"user"`
	}
	json.Unmarshal(out.Data, &login)
	owner := login.User.UserID
	refresh, _ := middleware.GenerateRefreshToken(owner, "regression_user")
	req, _ := http.NewRequest("GET", api.URL+"/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+refresh)
	response, err := api.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("refresh", response.StatusCode)
	}
	member := actor(t, "member")
	status, out = call(t, owner, "POST", "/chatroom/createroom", map[string]string{"name": "regression", "type": "public", "description": "original"})
	if status != 200 {
		t.Fatal("create", status, out)
	}
	var created struct {
		Chatroom struct {
			RoomID string `json:"roomId"`
		} `json:"chatroom"`
	}
	json.Unmarshal(out.Data, &created)
	room := created.Chatroom.RoomID
	if status, out = call(t, member, "POST", "/chatroom/joinroom", map[string]string{"roomId": room}); status != 200 {
		t.Fatal("join", status, out)
	}
	var memberID string
	environment.DB.QueryRow("SELECT member_rel_id FROM chatroom_members WHERE user_id=$1 AND room_id=$2", member, room).Scan(&memberID)
	for _, action := range []string{"setadmin", "removeadmin", "mute", "unmute"} {
		payload := map[string]any{"memberid": memberID}
		if action == "mute" {
			payload["duration"] = 60
		}
		if status, out = call(t, owner, "POST", "/chatroom/"+room+"/members/"+action, payload); status != 200 {
			t.Fatal(action, status, out)
		}
	}
	status, out = call(t, member, "POST", "/chatroom/"+room+"/messages", map[string]string{"type": "text", "text": "original text"})
	if status != 200 {
		t.Fatal(status, out)
	}
	var message struct {
		MessageID string `json:"messageId"`
	}
	json.Unmarshal(out.Data, &message)
	if status, _ = call(t, member, "POST", "/chatroom/"+room+"/messages", map[string]string{"type": "text", "text": "reply", "quotedMessageId": message.MessageID}); status != 200 {
		t.Fatal("reply", status)
	}
	for _, action := range []string{"edit", "delete"} {
		if status, _ = call(t, member, "POST", "/chatroom/"+room+"/messages/"+message.MessageID+"/"+action, map[string]string{"text": "edited text"}); status != 200 {
			t.Fatal(action, status)
		}
	}
	if status, out = call(t, owner, "POST", "/chatroom/"+room+"/update", map[string]string{"description": "updated"}); status != 200 {
		t.Fatal("update", status, out)
	}
	if status, _ = call(t, owner, "GET", "/chatroom/"+room+"/members/memberlist", nil); status != 200 {
		t.Fatal(status)
	}
	if status, _ = call(t, owner, "POST", "/chatroom/"+room+"/members/kick", map[string]string{"memberid": memberID}); status != 200 {
		t.Fatal("kick", status)
	}
	if count(t, "SELECT member_count FROM chatrooms WHERE room_id=$1", room) != 1 {
		t.Fatal("member count after kick")
	}
	for _, entry := range []struct {
		kind, password string
		want           int
	}{{"private", "", 403}, {"protected", "roompass", 200}} {
		status, out = call(t, owner, "POST", "/chatroom/createroom", map[string]string{"name": entry.kind, "type": entry.kind, "password": "roompass"})
		if status != 200 {
			t.Fatal(status, out)
		}
		json.Unmarshal(out.Data, &created)
		if status, _ = call(t, member, "POST", "/chatroom/joinroom", map[string]string{"roomId": created.Chatroom.RoomID, "password": entry.password}); status != entry.want {
			t.Fatal(entry.kind, status)
		}
		if status == 200 {
			if left, _ := call(t, member, "POST", "/chatroom/leaveroom", map[string]string{"roomId": created.Chatroom.RoomID}); left != 200 {
				t.Fatal(left)
			}
		}
	}
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="test.png"`)
	header.Set("Content-Type", "image/png")
	part, _ := form.CreatePart(header)
	part.Write(pngData.Bytes())
	form.Close()
	req, _ = http.NewRequest("POST", api.URL+"/api/v1/users/me/uploadavatar", &body)
	token, _ := middleware.GenerateToken(owner, "regression_user")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", form.FormDataContentType())
	response, err = api.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	json.NewDecoder(response.Body).Decode(&out)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("avatar", response.StatusCode, out)
	}
	var upload struct {
		URL string `json:"url"`
	}
	json.Unmarshal(out.Data, &upload)
	response, err = api.Client().Get(api.URL + upload.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("avatar static", response.StatusCode)
	}
	if status, _ = call(t, owner, "POST", "/users/me/update", map[string]string{"avatar": upload.URL}); status != 200 {
		t.Fatal("save avatar", status)
	}
	if status, _ = call(t, owner, "POST", "/chatroom/"+room+"/delete", nil); status != 200 {
		t.Fatal("delete room", status)
	}
}
