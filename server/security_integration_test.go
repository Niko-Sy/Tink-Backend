package server_test

import (
	"chatroombackend/middleware"
	"chatroombackend/server"
	social "chatroombackend/service/social"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTokenPurposeRefreshAndTicket(t *testing.T) {
	a := actor(t, "security")
	access, _ := middleware.GenerateToken(a, "security")
	refresh, _ := middleware.GenerateRefreshToken(a, "security")
	for _, test := range []struct {
		method, path, token string
		want                int
	}{{"GET", "/users/me/userinfo", refresh, 401}, {"POST", "/auth/refresh", access, 401}, {"POST", "/auth/refresh", refresh, 200}, {"GET", "/auth/refresh?refresh_Token=" + refresh, "", 401}} {
		req, _ := http.NewRequest(test.method, api.URL+"/api/v1"+test.path, nil)
		if test.token != "" {
			req.Header.Set("Authorization", "Bearer "+test.token)
		}
		res, e := api.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != test.want {
			t.Fatal(test.path, res.StatusCode)
		}
		if test.want == 200 {
			var out envelope
			json.NewDecoder(res.Body).Decode(&out)
			var data struct {
				Token, RefreshToken string
				ExpiresIn           int
			}
			json.Unmarshal(out.Data, &data)
			if _, e = middleware.ParseToken(data.Token); e != nil {
				t.Fatal(e)
			}
			if _, e = middleware.ParseTokenPurpose(data.RefreshToken, "refresh"); e != nil {
				t.Fatal(e)
			}
			if _, e = middleware.ParseToken(data.RefreshToken); e == nil {
				t.Fatal("refresh as access")
			}
		}
		res.Body.Close()
	}
	claims := middleware.CustomClaims{UserID: a, Purpose: "access", RegisteredClaims: jwt.RegisteredClaims{Subject: a, Issuer: "wrong", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	raw, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(middleware.JWTSecretKey)
	if _, e := middleware.ParseToken(raw); e == nil {
		t.Fatal("wrong issuer")
	}
	claims.Issuer = middleware.JWTIssuer
	claims.Purpose = ""
	raw, _ = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(middleware.JWTSecretKey)
	if _, e := middleware.ParseToken(raw); e == nil {
		t.Fatal("legacy token")
	}
	claims.Purpose = "access"
	raw, _ = jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(middleware.JWTSecretKey)
	if _, e := middleware.ParseToken(raw); e == nil {
		t.Fatal("wrong algorithm")
	}
	code, out := call(t, a, "POST", "/auth/ws-ticket", nil)
	if code != 200 {
		t.Fatal(code)
	}
	var data struct {
		Ticket string `json:"ticket"`
	}
	json.Unmarshal(out.Data, &data)
	url := strings.Replace(api.URL, "http", "ws", 1) + "/ws?ticket=" + data.Ticket
	h := http.Header{"Origin": []string{"https://evil.example"}}
	conn, res, e := websocket.DefaultDialer.Dial(url, h)
	if e == nil {
		conn.Close()
		t.Fatal("bad origin")
	}
	if res.StatusCode != 403 {
		t.Fatal(res.StatusCode)
	}
	conn, _, e = websocket.DefaultDialer.Dial(url, nil)
	if e != nil {
		t.Fatal(e)
	}
	conn.Close()
	conn, res, e = websocket.DefaultDialer.Dial(url, nil)
	if e == nil {
		conn.Close()
		t.Fatal("reused ticket")
	}
	if res.StatusCode != 401 {
		t.Fatal(res.StatusCode)
	}
	_, res, e = websocket.DefaultDialer.Dial(strings.Replace(api.URL, "http", "ws", 1)+"/ws?token="+access, nil)
	if e == nil || res.StatusCode != 401 {
		t.Fatal("legacy URL token accepted")
	}
}
func TestInvitationMultiConnections(t *testing.T) {
	old := api
	srv := httptest.NewServer(server.NewRouter(manager, social.Capabilities{Friends: true, RoomInvitations: true}))
	api = srv
	defer func() { api = old; srv.Close() }()
	a, b := actor(t, "a"), actor(t, "b")
	befriend(t, a, b)
	room := makeGroup(t, a, "private")
	one, two := dial(t, b), dial(t, b)
	defer one.Close()
	defer two.Close()
	i := inviteTo(t, a, b, room)
	for _, c := range []*websocket.Conn{one, two} {
		event := readKind(t, c, "invitation")
		data := event["data"].(map[string]any)
		if data["invitationId"] != i.Invitation.InvitationID {
			t.Fatal(data)
		}
	}
	one.Close()
	if code, _ := acceptInvite(t, i.Invitation.InvitationID, b); code != 200 {
		t.Fatal(code)
	}
	event := readKind(t, two, "room")
	if event["action"] != "added" {
		t.Fatal(event)
	}
	assertMembers(t, room, 2)
}
