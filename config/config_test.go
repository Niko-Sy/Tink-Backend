package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLIsRuntimeConfigurationSource(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://ignored.invalid/ignored")
	t.Setenv("BASE_URL", "http://ignored.invalid")
	t.Setenv("JWT_SECRET", "ignored-env-key")
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := []byte("database:\n  url: postgres://yaml.invalid/shared\nserver:\n  base_url: http://yaml.invalid:8080\n  allowed_origins: [http://yaml.invalid]\njwt:\n  secret: yaml-key\n  expiration: 1800\n  refresh_expiration: 7200\nsocial:\n  friends: true\n  direct_messages: false\n  room_invitations: true\n")
	if e := os.WriteFile(path, body, 0600); e != nil {
		t.Fatal(e)
	}
	c, e := LoadConfig(path)
	if e != nil {
		t.Fatal(e)
	}
	if c.Database.URL != "postgres://yaml.invalid/shared" || c.Server.BaseURL != "http://yaml.invalid:8080" || c.JWT.Secret != "yaml-key" {
		t.Fatal("environment overrode YAML")
	}
	if c.Social.DirectMessages || !c.Social.RoomInvitations || len(c.Server.AllowedOrigins) != 1 || c.JWT.Expiration != 1800 || c.JWT.RefreshExpiration != 7200 {
		t.Fatal("YAML settings ignored")
	}
}
func TestExistingYAMLDefaultsEnableDeliveredSocialFeatures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(path, []byte("database:\n  url: postgres://yaml.invalid/shared\n"), 0600)
	c, e := LoadConfig(path)
	if e != nil {
		t.Fatal(e)
	}
	if !c.Social.Friends || !c.Social.DirectMessages || !c.Social.RoomInvitations || c.JWT.Expiration != 86400 || c.JWT.RefreshExpiration != 604800 {
		t.Fatal("unexpected defaults")
	}
}
