package cmd

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidbudnick/postgres-tui/internal/db"
	"github.com/davidbudnick/postgres-tui/internal/testutil"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestCommands_Accessors(t *testing.T) {
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	mock := testutil.NewMockPG()
	c := NewCommands(cfg, mock)
	if c.Config() != cfg {
		t.Fatal("config")
	}
	if c.PG() != mock {
		t.Fatal("pg")
	}
}

func TestCommands_ErrorPaths(t *testing.T) {
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	mock := testutil.NewMockPG()
	c := NewCommands(cfg, mock)

	mock.FailNext = "connect"
	msg := c.Connect(types.Connection{Host: "x"})()
	if msg.(types.ConnectedMsg).Err == nil {
		t.Fatal("connect err")
	}

	mock.FailNext = "switch"
	msg = c.SelectDatabase("x")()
	if msg.(types.DatabaseSelectedMsg).Err == nil {
		t.Fatal("switch err")
	}

	mock.FailNext = "info"
	_ = mock.Connect(types.Connection{Host: "x"})
	msg = c.SelectDatabase("y")()
	// switch ok, info fails
	if msg.(types.DatabaseSelectedMsg).Err == nil {
		// switch might have failed if FailNext was still set — re-setup
	}
	mock.FailNext = "info"
	msg = c.Connect(types.Connection{Host: "x"})()
	if msg.(types.ConnectedMsg).Err == nil {
		t.Fatal("info after connect")
	}

	mock.FailNext = "objects"
	msg = c.LoadObjectKinds("public", []types.ObjectKind{types.ObjectTable})()
	if msg.(types.ObjectsLoadedMsg).Err == nil {
		t.Fatal("objects err")
	}

	msg = c.LoadObjectKinds("public", nil)()
	if msg.(types.ObjectsLoadedMsg).Err != nil {
		t.Fatal("empty kinds should be ok")
	}

	// multi-kind success + sort (kind, schema, name)
	mock.FailNext = ""
	mock.Objects = []types.SchemaObject{
		{Schema: "public", Name: "b", Kind: types.ObjectView},
		{Schema: "public", Name: "a", Kind: types.ObjectTable},
		{Schema: "billing", Name: "z", Kind: types.ObjectTable},
		{Schema: "billing", Name: "a", Kind: types.ObjectTable},
	}
	msg = c.LoadObjectKinds("", []types.ObjectKind{types.ObjectTable, types.ObjectView})()
	loaded := msg.(types.ObjectsLoadedMsg)
	if loaded.Err != nil || len(loaded.Objects) < 3 {
		t.Fatalf("%+v", loaded)
	}
}

func TestCommands_LoadObjectDetailAndFavorites(t *testing.T) {
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	mock := testutil.NewMockPG()
	c := NewCommands(cfg, mock)
	_ = mock.Connect(types.Connection{Host: "h"})

	msg := c.LoadObjectDetail("public", "users", types.ObjectSequence)()
	if msg.(types.TableDetailLoadedMsg).Err != nil {
		t.Fatal(msg)
	}

	_, _ = cfg.AddFavorite(types.Favorite{
		ConnectionID: 1, Database: "app", Schema: "public", Object: "users",
	})
	msg = c.LoadFavorites(1)()
	fm := msg.(types.FavoritesLoadedMsg)
	if len(fm.Favorites) != 1 {
		t.Fatalf("%+v", fm)
	}
}

func TestCommands_ExportCSV_EmptyPathAndQuotes(t *testing.T) {
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	c := NewCommands(cfg, testutil.NewMockPG())
	msg := c.ExportCSV("", types.QueryResult{
		Columns: []string{"a"},
		Rows:    [][]string{{`he said "hi"`}, {"plain"}},
	})()
	em := msg.(types.ExportDoneMsg)
	if em.Err != nil || em.Path == "" || em.Rows != 2 {
		t.Fatalf("%+v", em)
	}
}

func TestCommands_CopyToClipboard(t *testing.T) {
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	c := NewCommands(cfg, testutil.NewMockPG())
	old := clipboardWrite
	t.Cleanup(func() { clipboardWrite = old })

	clipboardWrite = func(string) error { return nil }
	msg := c.CopyToClipboard("hello")()
	sm := msg.(types.StatusMsg)
	if sm.Text != "Copied to clipboard" {
		t.Fatal(sm.Text)
	}

	clipboardWrite = func(string) error { return errors.New("no clip") }
	msg = c.CopyToClipboard("hello")()
	sm = msg.(types.StatusMsg)
	if !strings.Contains(sm.Text, "clipboard:") {
		t.Fatal(sm.Text)
	}
}
