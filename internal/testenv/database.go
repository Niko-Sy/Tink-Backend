// Package testenv owns disposable local databases. It never reads the app DSN.
package testenv

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

type Environment struct {
	DB       *sql.DB
	DSN      string
	postgres *embeddedpostgres.EmbeddedPostgres
	dir      string
}

func Start() (*Environment, error) {
	dir, err := os.MkdirTemp("", "tink-social-pg-")
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	secret := make([]byte, 20)
	if _, err = rand.Read(secret); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().Version(embeddedpostgres.V16).
		Port(uint32(port)).Database("tink_test").Username("tink_test").Password(hex.EncodeToString(secret)).
		RuntimePath(filepath.Join(dir, "runtime")).DataPath(filepath.Join(dir, "data")).
		CachePath(filepath.Join(os.TempDir(), "tink-postgres-cache")).BinaryRepositoryURL("https://repo.maven.apache.org/maven2").
		StartTimeout(45 * time.Second).StartParameters(map[string]string{"listen_addresses": "127.0.0.1", "timezone": "UTC"}).Logger(io.Discard))
	if err = pg.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("isolated PostgreSQL startup: %w", err)
	}
	dsn := fmt.Sprintf("postgres://tink_test:%s@127.0.0.1:%d/tink_test?sslmode=disable", hex.EncodeToString(secret), port)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		pg.Stop()
		os.RemoveAll(dir)
		return nil, err
	}
	db.SetMaxOpenConns(20)
	if err = db.Ping(); err != nil {
		db.Close()
		pg.Stop()
		os.RemoveAll(dir)
		return nil, err
	}
	return &Environment{DB: db, DSN: dsn, postgres: pg, dir: dir}, nil
}
func (e *Environment) Close() {
	if e.DB != nil {
		e.DB.Close()
	}
	if e.postgres != nil {
		_ = e.postgres.Stop()
	}
	_ = os.RemoveAll(e.dir)
}
func Root() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(f), "../.."))
}
func Migrate(db *sql.DB, target int) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations(version bigint NOT NULL, dirty boolean NOT NULL DEFAULT false)"); err != nil {
		return err
	}
	var current int
	err := db.QueryRow("SELECT COALESCE(max(version),0) FROM schema_migrations").Scan(&current)
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(Root(), "dbsql/migration/*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		var version int
		fmt.Sscanf(filepath.Base(f), "%d", &version)
		if version <= current || version > target {
			continue
		}
		if err = apply(db, f, version); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	}
	return nil
}
func Rollback(db *sql.DB, target int) error {
	var current int
	if err := db.QueryRow("SELECT COALESCE(max(version),0) FROM schema_migrations").Scan(&current); err != nil {
		return err
	}
	files, _ := filepath.Glob(filepath.Join(Root(), "dbsql/migration/*.down.sql"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, f := range files {
		var version int
		fmt.Sscanf(filepath.Base(f), "%d", &version)
		if version > current || version <= target {
			continue
		}
		if err := apply(db, f, version-1); err != nil {
			return err
		}
	}
	return nil
}
func apply(db *sql.DB, f string, version int) error {
	data, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(string(data)); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM schema_migrations"); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO schema_migrations(version,dirty) VALUES($1,false)", version); err != nil {
		return err
	}
	return tx.Commit()
}

// Seed contains only disposable development identities, not application secrets.
func Seed(db *sql.DB) (map[string]string, error) {
	ids := map[string]string{}
	hash, err := bcrypt.GenerateFromPassword([]byte("Testpass123!"), bcrypt.MinCost)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"alice", "bob", "carol", "disabled", "hidden", "closed"} {
		var id string
		err = db.QueryRow("INSERT INTO users(username,hashed_password,nickname,email,avatar_url) VALUES($1,$2,$1,$3,'/Tink.svg') RETURNING user_id", name, string(hash), name+"@example.test").Scan(&id)
		if err != nil {
			return nil, err
		}
		ids[name] = id
	}
	if _, err = db.Exec("UPDATE users SET account_status='suspended' WHERE user_id=$1", ids["disabled"]); err != nil {
		return nil, err
	}
	if _, err = db.Exec("INSERT INTO user_settings(user_id,show_online_status) VALUES($1,false)", ids["hidden"]); err != nil {
		return nil, err
	}
	if _, err = db.Exec("INSERT INTO user_settings(user_id,enable_friend_request) VALUES($1,false)", ids["closed"]); err != nil {
		return nil, err
	}
	for _, kind := range []string{"public", "private_invite_only", "private_password"} {
		var room string
		err = db.QueryRow("INSERT INTO chatrooms(room_name,room_type,access_password,member_count) VALUES($1,$2,$3,1) RETURNING room_id", "Test "+kind, kind, sql.NullString{String: "roompass", Valid: kind == "private_password"}).Scan(&room)
		if err != nil {
			return nil, err
		}
		if _, err = db.Exec("INSERT INTO chatroom_members(user_id,room_id,member_role) VALUES($1,$2,'owner')", ids["alice"], room); err != nil {
			return nil, err
		}
		ids[kind] = room
	}
	return ids, nil
}
