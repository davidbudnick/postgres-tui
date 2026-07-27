package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/davidbudnick/postgres-tui/internal/db"
	"github.com/davidbudnick/postgres-tui/internal/service"
	"github.com/davidbudnick/postgres-tui/internal/testutil"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestCommands_ConfigCRUD(t *testing.T) {
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	mock := testutil.NewMockPG()
	c := NewCommands(cfg, mock)

	msg := c.LoadConnections()()
	loaded, ok := msg.(types.ConnectionsLoadedMsg)
	if !ok || loaded.Err != nil {
		t.Fatalf("%T %v", msg, loaded.Err)
	}

	msg = c.AddConnection(types.Connection{Name: "x", Host: "localhost", Port: 5432})()
	added := msg.(types.ConnectionAddedMsg)
	if added.Err != nil {
		t.Fatal(added.Err)
	}

	added.Connection.Name = "y"
	msg = c.UpdateConnection(added.Connection)()
	if msg.(types.ConnectionUpdatedMsg).Err != nil {
		t.Fatal(msg)
	}

	msg = c.DeleteConnection(added.Connection.ID)()
	if msg.(types.ConnectionDeletedMsg).Err != nil {
		t.Fatal(msg)
	}
}

func TestCommands_ConnectAndBrowse(t *testing.T) {
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	mock := testutil.NewMockPG()
	c := NewCommandsFromContainer(service.NewContainer(cfg, mock))

	msg := c.Connect(types.Connection{Host: "localhost", Port: 5432})()
	cm := msg.(types.ConnectedMsg)
	if cm.Err != nil {
		t.Fatal(cm.Err)
	}
	if cm.Info.Version == "" {
		t.Fatal("expected version")
	}

	msg = c.LoadDatabases()()
	if msg.(types.DatabasesLoadedMsg).Err != nil {
		t.Fatal(msg)
	}
	msg = c.SelectDatabase("app")()
	if msg.(types.DatabaseSelectedMsg).Err != nil {
		t.Fatal(msg)
	}
	msg = c.LoadSchemas()()
	if msg.(types.SchemasLoadedMsg).Err != nil {
		t.Fatal(msg)
	}
	msg = c.LoadObjects("public", types.ObjectTable)()
	if msg.(types.ObjectsLoadedMsg).Err != nil {
		t.Fatal(msg)
	}
	msg = c.LoadTableDetail("public", "users")()
	if msg.(types.TableDetailLoadedMsg).Err != nil {
		t.Fatal(msg)
	}
	msg = c.LoadTableData("public", "users", 0, 50)()
	if msg.(types.TableDataLoadedMsg).Err != nil {
		t.Fatal(msg)
	}
	msg = c.RunQuery("SELECT 1", 10)()
	if msg.(types.QueryResultMsg).Err != nil {
		t.Fatal(msg)
	}
	msg = c.LoadActivity()()
	if msg.(types.ActivityLoadedMsg).Err != nil {
		t.Fatal(msg)
	}
	msg = c.LoadERD("public")()
	erd := msg.(types.ERDLoadedMsg)
	if erd.Err != nil {
		t.Fatal(erd.Err)
	}
	if erd.Graph.Schema != "public" || len(erd.Graph.Tables) == 0 {
		t.Fatalf("erd=%+v", erd.Graph)
	}
	msg = c.LoadServerInfo()()
	if msg.(types.ServerInfoLoadedMsg).Err != nil {
		t.Fatal(msg)
	}
	msg = c.TestConnection(types.Connection{Host: "localhost"})()
	tm := msg.(types.ConnectionTestMsg)
	if !tm.Success || tm.Latency < 0 {
		t.Fatal(tm)
	}
	msg = c.Disconnect()()
	if _, ok := msg.(types.DisconnectedMsg); !ok {
		t.Fatalf("%T", msg)
	}
}

func TestCommands_Export(t *testing.T) {
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	c := NewCommands(cfg, testutil.NewMockPG())
	path := filepath.Join(t.TempDir(), "out.csv")
	msg := c.ExportCSV(path, types.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "a,b"}, {"2", "x"}},
	})()
	em := msg.(types.ExportDoneMsg)
	if em.Err != nil || em.Rows != 2 {
		t.Fatalf("%+v", em)
	}
}

func TestCheckForUpdate(t *testing.T) {
	msg := CheckForUpdate("dev")()
	if msg != nil {
		t.Fatalf("%v", msg)
	}
	_ = time.Second
}
