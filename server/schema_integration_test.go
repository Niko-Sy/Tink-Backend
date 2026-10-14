package server_test

import (
	"chatroombackend/internal/schema"
	"chatroombackend/internal/testenv"
	"context"
	"github.com/google/uuid"
	"testing"
)

func TestReleaseSchemaUpgradeInspectionAndProtectedRollback(t *testing.T) {
	ctx := context.Background()
	e, err := testenv.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err = schema.Apply(ctx, e.DB, 6); err != nil {
		t.Fatal(err)
	}
	ids, err := testenv.Seed(e.DB)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	if _, err = e.DB.Exec("INSERT INTO chatroom_invitations(invitation_id,room_id,inviter_id,invitee_id,status,handled_at) VALUES($1,$2,$3,$4,'accepted',now())", id, ids["private_invite_only"], ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}
	if err = schema.Apply(ctx, e.DB, 7); err != nil {
		t.Fatal(err)
	}
	v, dirty, err := schema.Status(ctx, e.DB)
	if err != nil || v != 7 || dirty {
		t.Fatal(v, dirty, err)
	}
	checks, err := schema.Inspect(ctx, e.DB)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range checks {
		if c.Violations != 0 {
			t.Fatal(c)
		}
	}
	if err = schema.Apply(ctx, e.DB, 6); err != nil {
		t.Fatal(err)
	}
	if err = schema.Apply(ctx, e.DB, 7); err != nil {
		t.Fatal(err)
	}
	if err = schema.Apply(ctx, e.DB, 4); err == nil {
		t.Fatal("destroyed invitation history")
	}
	v, _, _ = schema.Status(ctx, e.DB)
	if v != 7 {
		t.Fatal("partial rollback")
	}
	e.DB.Exec("UPDATE schema_migrations SET dirty=true")
	if err = schema.Apply(ctx, e.DB, 7); err == nil {
		t.Fatal("dirty schema accepted")
	}
}
