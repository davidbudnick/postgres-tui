package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

func newTestConfig(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	cfg, err := NewConfig(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	return cfg
}

func reloadConfig(t *testing.T, cfg *Config) *Config {
	t.Helper()
	reloaded, err := NewConfig(cfg.path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	return reloaded
}

func TestConfig_AddListDelete(t *testing.T) {
	cfg := newTestConfig(t)
	conn, err := cfg.AddConnection(types.Connection{
		Name: "local", Host: "localhost", Port: 5432, Username: "postgres", Password: "secret",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if conn.ID == 0 {
		t.Fatal("expected id")
	}
	list, err := cfg.ListConnections()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if err := cfg.DeleteConnection(conn.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err = cfg.ListConnections()
	if err != nil || len(list) != 0 {
		t.Fatalf("after delete: %v len=%d", err, len(list))
	}
}

func TestConfig_EnsureDemoConnections(t *testing.T) {
	cfg := newTestConfig(t)
	if err := cfg.EnsureDemoConnections(); err != nil {
		t.Fatal(err)
	}
	list, err := cfg.ListConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d want 2 demos", len(list))
	}
	if list[0].Name != "Local Demo" || list[0].Database != "demo" {
		t.Fatalf("first: %+v", list[0])
	}
	if list[1].Name != "Analytics (RO)" || !list[1].ReadOnly {
		t.Fatalf("second: %+v", list[1])
	}
	// In-memory demo password for localhost connect flow.
	if list[0].Password != "postgres" {
		t.Fatalf("demo password not hydrated: %q", list[0].Password)
	}
	// Idempotent
	if err := cfg.EnsureDemoConnections(); err != nil {
		t.Fatal(err)
	}
	list, _ = cfg.ListConnections()
	if len(list) != 2 {
		t.Fatalf("re-ensure grew list: %d", len(list))
	}
	// Disk still strips password
	reloaded := reloadConfig(t, cfg)
	list, _ = reloaded.ListConnections()
	if list[0].Password != "postgres" {
		// hydrate runs on NewConfig
		t.Fatalf("reloaded should hydrate demo password, got %q", list[0].Password)
	}
	// Confirm on-disk file has no password
	raw, err := os.ReadFile(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"password": "postgres"`) {
		t.Fatal("password must not be written to disk")
	}
}

func TestConfig_Persistence_PasswordStripping(t *testing.T) {
	cfg := newTestConfig(t)
	_, err := cfg.AddConnection(types.Connection{
		Name: "local", Host: "localhost", Port: 5432, Username: "u", Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	reloaded := reloadConfig(t, cfg)
	list, err := reloaded.ListConnections()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d", len(list))
	}
	if list[0].Password != "" {
		t.Fatal("password must be stripped on disk")
	}
	if list[0].Name != "local" || list[0].Host != "localhost" {
		t.Fatalf("fields lost: %+v", list[0])
	}
}

func TestConfig_UpdatePreservesPassword(t *testing.T) {
	cfg := newTestConfig(t)
	conn, err := cfg.AddConnection(types.Connection{
		Name: "local", Host: "localhost", Port: 5432, Username: "u", Password: "keepme",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	conn.Name = "renamed"
	conn.Password = ""
	updated, err := cfg.UpdateConnection(conn)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Password != "keepme" {
		t.Fatalf("password not preserved: %q", updated.Password)
	}
}

func TestConfig_FavoritesAndRecent(t *testing.T) {
	cfg := newTestConfig(t)
	f, err := cfg.AddFavorite(types.Favorite{
		ConnectionID: 1, Database: "app", Schema: "public", Object: "users", Kind: "table",
	})
	if err != nil {
		t.Fatalf("fav: %v", err)
	}
	if f.Object != "users" {
		t.Fatal(f)
	}
	if !cfg.IsFavorite(1, "app", "public", "users") {
		t.Fatal("expected favorite")
	}
	cfg.AddRecentObject(types.RecentObject{
		ConnectionID: 1, Database: "app", Schema: "public", Object: "users",
	})
	if len(cfg.ListRecentObjects(1)) != 1 {
		t.Fatal("recent")
	}
	if err := cfg.RemoveFavorite(1, "app", "public", "users"); err != nil {
		t.Fatal(err)
	}
}

func TestConfig_SavedQueriesAndKeys(t *testing.T) {
	cfg := newTestConfig(t)
	_, err := cfg.AddSavedQuery(types.SavedQuery{Name: "all users", SQL: "select 1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ListSavedQueries()) != 1 {
		t.Fatal("expected query")
	}
	if err := cfg.DeleteSavedQuery("all users"); err != nil {
		t.Fatal(err)
	}
	kb := cfg.GetKeyBindings()
	if kb.Quit != "q" {
		t.Fatal(kb)
	}
	if err := cfg.ResetKeyBindings(); err != nil {
		t.Fatal(err)
	}
	if cfg.GetPageSize() != 100 {
		t.Fatal(cfg.GetPageSize())
	}
}
