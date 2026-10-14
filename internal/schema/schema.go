package schema

import (
	"chatroombackend/dbsql/migration"
	"context"
	"database/sql"
	"fmt"
)

const Latest = 7

func Status(ctx context.Context, d *sql.DB) (int, bool, error) {
	var version int
	var rows int
	if err := d.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&rows); err != nil {
		return 0, false, err
	}
	if rows != 1 {
		return 0, false, fmt.Errorf("ambiguous schema marker")
	}
	var dirty bool
	err := d.QueryRowContext(ctx, "SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty)
	return version, dirty, err
}

// Apply uses an advisory transaction lock and atomically updates the schema marker.
// No operation reads application configuration or silently drops social data.
func Apply(ctx context.Context, d *sql.DB, target int) error {
	if target < 0 || target > Latest {
		return fmt.Errorf("unsupported target")
	}
	tx, e := d.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(741862103)"); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL DEFAULT false)"); e != nil {
		return e
	}
	var version int
	var dirty bool
	var rows int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&rows); e != nil {
		return e
	}
	if rows > 1 {
		return fmt.Errorf("ambiguous schema marker")
	}
	if rows == 1 {
		if e = tx.QueryRowContext(ctx, "SELECT version,dirty FROM schema_migrations").Scan(&version, &dirty); e != nil {
			return e
		}
	}
	if dirty {
		return fmt.Errorf("dirty schema requires manual recovery")
	}
	if version > Latest {
		return fmt.Errorf("schema newer than binary")
	}
	if target < version && target < 5 {
		var n int
		if version >= 5 {
			if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM chatroom_invitations").Scan(&n); e != nil {
				return e
			}
			if n > 0 {
				return fmt.Errorf("refusing rollback that removes invitation history")
			}
		}
	}
	for version != target {
		next := version + 1
		direction := "up"
		fileVersion := next
		if version > target {
			direction = "down"
			fileVersion = version
			next = version - 1
		}
		files, e := migration.Files.ReadDir(".")
		if e != nil {
			return e
		}
		var source []byte
		for _, f := range files {
			var num int
			fmt.Sscanf(f.Name(), "%d", &num)
			if num == fileVersion && len(f.Name()) >= len(direction)+4 && f.Name()[len(f.Name())-len(direction)-4:] == direction+".sql" {
				source, e = migration.Files.ReadFile(f.Name())
				break
			}
		}
		if e != nil || source == nil {
			return fmt.Errorf("missing migration %d %s", fileVersion, direction)
		}
		if _, e = tx.ExecContext(ctx, string(source)); e != nil {
			return fmt.Errorf("migration %d %s failed (%T)", fileVersion, direction, e)
		}
		version = next
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM schema_migrations"); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,dirty) VALUES($1,false)", version); e != nil {
		return e
	}
	return tx.Commit()
}

type Check struct {
	Name       string `json:"name"`
	Violations int    `json:"violations"`
}

func Inspect(ctx context.Context, d *sql.DB) ([]Check, error) {
	checks := []struct{ name, query string }{
		{"member_count", "SELECT count(*) FROM chatrooms r WHERE conversation_kind='group' AND room_status='active' AND member_count<>(SELECT count(*) FROM chatroom_members m WHERE m.room_id=r.room_id AND is_active)"},
		{"direct_members", "SELECT count(*) FROM direct_pairs p WHERE (SELECT count(*) FROM chatroom_members m WHERE m.room_id=p.room_id AND m.is_active)<>2 OR EXISTS(SELECT 1 FROM chatroom_members m WHERE m.room_id=p.room_id AND m.user_id NOT IN(p.user_id_1,p.user_id_2))"},
		{"direct_room_kind", "SELECT count(*) FROM direct_pairs p JOIN chatrooms r USING(room_id) WHERE r.conversation_kind<>'direct'"},
		{"pending_invitations", "SELECT count(*) FROM (SELECT room_id,invitee_id FROM chatroom_invitations WHERE status='pending' GROUP BY room_id,invitee_id HAVING count(*)>1) t"},
		{"pending_friend_requests", "SELECT count(*) FROM (SELECT LEAST(from_user_id,to_user_id),GREATEST(from_user_id,to_user_id) FROM friend_request WHERE status='pending' GROUP BY 1,2 HAVING count(*)>1) t"},
		{"direct_read_position", "SELECT count(*) FROM chatroom_members m JOIN direct_pairs p USING(room_id) WHERE m.last_read_seq>p.last_message_seq"},
	}
	out := []Check{}
	tx, e := d.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	for _, c := range checks {
		var n int
		if e = tx.QueryRowContext(ctx, c.query).Scan(&n); e != nil {
			return nil, fmt.Errorf("inspection %s failed (%T)", c.name, e)
		}
		out = append(out, Check{c.name, n})
	}
	return out, tx.Commit()
}
