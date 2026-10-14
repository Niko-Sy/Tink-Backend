package main

import (
	"bytes"
	"chatroombackend/internal/testenv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStartupWithoutMigrationMarkerUsesYAMLAndPreservesCounts(t *testing.T) {
	env, e := testenv.Start()
	if e != nil {
		t.Fatal(e)
	}
	defer env.Close()
	if e = testenv.Migrate(env.DB, 7); e != nil {
		t.Fatal(e)
	}
	if _, e = testenv.Seed(env.DB); e != nil {
		t.Fatal(e)
	}
	if _, e = env.DB.Exec("DROP TABLE schema_migrations;UPDATE chatrooms SET online_count=9 WHERE conversation_kind='group'"); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "backend")
	build := exec.Command("go", "build", "-o", bin, ".")
	if output, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build failed: %s", output)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	configPath := filepath.Join(dir, "config.yaml")
	body := fmt.Sprintf("database:\n  url: %q\n  max_open_conns: 10\n  max_idle_conns: 2\nserver:\n  host: 127.0.0.1\n  port: %d\n  base_url: http://127.0.0.1:%d\n  allowed_origins: [http://127.0.0.1:11451]\njwt:\n  secret: startup-test-key-at-least-32-bytes-long\n  expiration: 1800\n  refresh_expiration: 7200\nupload:\n  upload_dir: ./uploads\n  max_size: 5242880\n  url_prefix: /static/images\n  allowed_types: [image/png]\nlogger:\n  enable_file: false\n  enable_console: false\nsocial:\n  friends: true\n  direct_messages: true\n  room_invitations: true\n", env.DSN, port, port)
	if e = os.WriteFile(configPath, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(bin, "-config", configPath)
	cmd.Dir = dir
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "DATABASE_URL=") && !strings.HasPrefix(v, "JWT_SECRET=") && !strings.HasPrefix(v, "APP_ENV=") && !strings.HasPrefix(v, "ALLOWED_ORIGINS=") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "DATABASE_URL=postgres://ignored.invalid/ignored", "JWT_SECRET=ignored-env", "APP_ENV=production", "ALLOWED_ORIGINS=*")
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		if cmd.ProcessState == nil {
			cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(8 * time.Second):
				cmd.Process.Kill()
				<-done
			}
		}
	}()
	client := &http.Client{Timeout: time.Second}
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	ready := false
	for i := 0; i < 100; i++ {
		res, e := client.Get(base + "/health/ready")
		if e == nil {
			var out map[string]any
			json.NewDecoder(res.Body).Decode(&out)
			res.Body.Close()
			if res.StatusCode == 200 && out["ready"] == true {
				if _, has := out["schemaVersion"]; has {
					t.Fatal("version gate returned")
				}
				ready = true
				break
			}
		}
		select {
		case e := <-done:
			t.Fatalf("backend exited before ready: %v", e)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("backend with migrated tables but no marker did not start")
	}
	var n int
	if e = env.DB.QueryRow("SELECT count(*) FROM chatrooms WHERE online_count<>9").Scan(&n); e != nil || n != 0 {
		t.Fatal("startup reset shared online counts", n, e)
	}
	login, e := client.Post(base+"/api/v1/auth/login", "application/json", bytes.NewBufferString(`{"username":"alice","password":"Testpass123!"}`))
	if e != nil {
		t.Fatal(e)
	}
	var envelope struct {
		Data struct {
			Token string `json:"token"`
		}
	}
	json.NewDecoder(login.Body).Decode(&envelope)
	login.Body.Close()
	if envelope.Data.Token == "" {
		t.Fatal("YAML JWT not applied")
	}
	req, _ := http.NewRequest("GET", base+"/api/v1/users/me/social-capabilities", nil)
	req.Header.Set("Authorization", "Bearer "+envelope.Data.Token)
	res, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !bytes.Contains(raw, []byte(`"directMessages":true`)) || !bytes.Contains(raw, []byte(`"roomInvitations":true`)) {
		t.Fatal("YAML capabilities not applied", string(raw))
	}
	if e = cmd.Process.Signal(syscall.SIGTERM); e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("backend did not stop")
	}
}
