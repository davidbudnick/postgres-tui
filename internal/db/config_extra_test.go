package db

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestConfig_Close(t *testing.T) {
	cfg := newTestConfig(t)
	if err := cfg.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConfig_IsFavoriteMiss(t *testing.T) {
	cfg := newTestConfig(t)
	if cfg.IsFavorite(1, "d", "s", "o") {
		t.Fatal("expected miss")
	}
}

func TestConfig_ListFavorites(t *testing.T) {
	cfg := newTestConfig(t)
	_, _ = cfg.AddFavorite(types.Favorite{ConnectionID: 1, Database: "d", Schema: "s", Object: "a"})
	_, _ = cfg.AddFavorite(types.Favorite{ConnectionID: 2, Database: "d", Schema: "s", Object: "b"})
	if len(cfg.ListFavorites(1)) != 1 {
		t.Fatal(cfg.ListFavorites(1))
	}
	if len(cfg.ListFavorites(0)) != 2 {
		t.Fatal(cfg.ListFavorites(0))
	}
	// duplicate favorite returns existing
	f, err := cfg.AddFavorite(types.Favorite{ConnectionID: 1, Database: "d", Schema: "s", Object: "a"})
	if err != nil || f.Object != "a" {
		t.Fatal(err, f)
	}
	if len(cfg.ListFavorites(1)) != 1 {
		t.Fatal("duplicate added")
	}
}

func TestConfig_NotExistPaths(t *testing.T) {
	cfg := newTestConfig(t)
	if _, err := cfg.UpdateConnection(types.Connection{ID: 999}); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := cfg.DeleteConnection(999); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := cfg.RemoveFavorite(1, "d", "s", "o"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := cfg.DeleteSavedQuery("nope"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestConfig_DefaultsAndUpdateFields(t *testing.T) {
	cfg := newTestConfig(t)
	conn, err := cfg.AddConnection(types.Connection{
		Name: "x", Host: "h", Username: "u",
		Group: "g", Color: "red",
		TLSConfig: &types.TLSConfig{CAFile: "/ca"},
		Password:  "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if conn.Port != 5432 || conn.SSLMode != types.SSLModePrefer {
		t.Fatalf("%+v", conn)
	}

	upd := types.Connection{ID: conn.ID, Name: "y", Host: "h2", Username: "u2"}
	updated, err := cfg.UpdateConnection(upd)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Group != "g" || updated.Color != "red" || updated.TLSConfig == nil || updated.Password != "secret" {
		t.Fatalf("%+v", updated)
	}
}

func TestConfig_GetPageSizeAndMaxRecent(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.PageSize = 0
	if cfg.GetPageSize() != 100 {
		t.Fatal(cfg.GetPageSize())
	}
	cfg.PageSize = 50
	if cfg.GetPageSize() != 50 {
		t.Fatal(cfg.GetPageSize())
	}

	cfg.MaxRecent = 2
	cfg.AddRecentObject(types.RecentObject{ConnectionID: 1, Object: "a"})
	cfg.AddRecentObject(types.RecentObject{ConnectionID: 1, Object: "b"})
	cfg.AddRecentObject(types.RecentObject{ConnectionID: 1, Object: "c"})
	if len(cfg.ListRecentObjects(1)) != 2 {
		t.Fatal(cfg.ListRecentObjects(1))
	}
	// re-add moves to front
	cfg.AddRecentObject(types.RecentObject{ConnectionID: 1, Object: "b"})
	recent := cfg.ListRecentObjects(1)
	if recent[0].Object != "b" {
		t.Fatal(recent)
	}
	if len(cfg.ListRecentObjects(0)) != 2 {
		t.Fatal(cfg.ListRecentObjects(0))
	}
}

func TestConfig_KeyBindingsSet(t *testing.T) {
	cfg := newTestConfig(t)
	kb := types.DefaultKeyBindings()
	kb.Quit = "Q"
	if err := cfg.SetKeyBindings(kb); err != nil {
		t.Fatal(err)
	}
	if cfg.GetKeyBindings().Quit != "Q" {
		t.Fatal(cfg.GetKeyBindings())
	}
}

func TestConfig_SaveErrorPaths(t *testing.T) {
	cfg := newTestConfig(t)
	old := jsonMarshalIndent
	t.Cleanup(func() { jsonMarshalIndent = old })
	jsonMarshalIndent = func(any, string, string) ([]byte, error) {
		return nil, errors.New("marshal")
	}
	if _, err := cfg.AddConnection(types.Connection{Name: "x", Host: "h"}); err == nil {
		t.Fatal("expected marshal fail on add")
	}
	// restore for next ops
	jsonMarshalIndent = old
	conn, err := cfg.AddConnection(types.Connection{Name: "x", Host: "h", Username: "u"})
	if err != nil {
		t.Fatal(err)
	}
	jsonMarshalIndent = func(any, string, string) ([]byte, error) {
		return nil, errors.New("marshal")
	}
	conn.Name = "z"
	if _, err := cfg.UpdateConnection(conn); err == nil {
		t.Fatal("update marshal")
	}
	if err := cfg.DeleteConnection(conn.ID); err == nil {
		t.Fatal("delete marshal")
	}
	if _, err := cfg.AddFavorite(types.Favorite{ConnectionID: 1, Object: "o", Database: "d", Schema: "s"}); err == nil {
		t.Fatal("fav marshal")
	}
	// clear favorites and add without marshal fail first
	jsonMarshalIndent = old
	_, _ = cfg.AddFavorite(types.Favorite{ConnectionID: 1, Object: "o", Database: "d", Schema: "s"})
	jsonMarshalIndent = func(any, string, string) ([]byte, error) {
		return nil, errors.New("marshal")
	}
	if err := cfg.RemoveFavorite(1, "d", "s", "o"); err == nil {
		t.Fatal("remove fav")
	}
	jsonMarshalIndent = old
	_, _ = cfg.AddSavedQuery(types.SavedQuery{Name: "q", SQL: "select 1"})
	jsonMarshalIndent = func(any, string, string) ([]byte, error) {
		return nil, errors.New("marshal")
	}
	if _, err := cfg.AddSavedQuery(types.SavedQuery{Name: "q2", SQL: "s"}); err == nil {
		t.Fatal("add query")
	}
	if err := cfg.DeleteSavedQuery("q"); err == nil {
		t.Fatal("del query")
	}
	// EnsureDemoConnections save fail
	cfg2 := newTestConfig(t)
	jsonMarshalIndent = func(any, string, string) ([]byte, error) {
		return nil, errors.New("marshal")
	}
	if err := cfg2.EnsureDemoConnections(); err == nil {
		t.Fatal("demo")
	}
	jsonMarshalIndent = old
}

func TestConfig_IsLocalDemoConn(t *testing.T) {
	cases := []struct {
		c    types.Connection
		want bool
	}{
		{types.Connection{Host: "localhost", Port: 5432, Username: "postgres"}, true},
		{types.Connection{Host: "127.0.0.1", Port: 0, Username: "postgres"}, true},
		{types.Connection{Host: "remote", Port: 5432, Username: "postgres"}, false},
		{types.Connection{Host: "localhost", Port: 5433, Username: "postgres"}, false},
		{types.Connection{Host: "localhost", Port: 5432, Username: "other"}, false},
	}
	for _, tc := range cases {
		if got := isLocalDemoConn(tc.c); got != tc.want {
			t.Fatalf("%+v got %v want %v", tc.c, got, tc.want)
		}
	}
}

func TestConfig_LoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConfig(path); err == nil {
		t.Fatal("expected load error")
	}
}

func TestConfig_NewConfigDefaultsFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// page size 0 and empty quit binding get fixed on load
	raw := `{"connections":[{"id":5,"name":"x","host":"localhost","port":5432,"username":"postgres"}],"page_size":0,"key_bindings":{"quit":""}}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PageSize != 100 {
		t.Fatal(cfg.PageSize)
	}
	if cfg.GetKeyBindings().Quit != "q" {
		t.Fatal(cfg.GetKeyBindings())
	}
	if cfg.nextID != 6 {
		t.Fatal(cfg.nextID)
	}
	list, _ := cfg.ListConnections()
	if list[0].Password != "postgres" {
		t.Fatalf("hydrate: %q", list[0].Password)
	}
}

func TestConfig_NewConfigMkdirFail(t *testing.T) {
	// Use a path under a file (not a directory) to force MkdirAll failure.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConfig(filepath.Join(blocker, "nested", "config.json")); err == nil {
		t.Fatal("expected mkdir fail")
	}
}

func TestConfig_IsFavoriteFalse(t *testing.T) {
	cfg := newTestConfig(t)
	if cfg.IsFavorite(1, "d", "s", "o") {
		t.Fatal("expected false")
	}
	_, _ = cfg.AddFavorite(types.Favorite{ConnectionID: 1, Database: "d", Schema: "s", Object: "o"})
	if cfg.IsFavorite(1, "d", "s", "other") {
		t.Fatal("expected false for different object")
	}
}
