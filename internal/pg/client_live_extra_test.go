package pg

import (
	"strings"
	"testing"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestLiveConnectLifecycle(t *testing.T) {
	c := NewClient()
	conn := types.Connection{
		Host: "localhost", Port: 5432, Username: "postgres", Password: "postgres",
		Database: "demo", SSLMode: types.SSLModeDisable,
	}
	if err := c.Connect(conn); err != nil {
		t.Skip(err)
	}
	if !c.IsConnected() || c.CurrentDatabase() != "demo" {
		t.Fatal("state")
	}
	if err := c.Connect(conn); err != nil {
		t.Fatal(err)
	}
	if err := c.Connect(types.Connection{
		Host: "localhost", Username: "", Password: "postgres", SSLMode: "",
	}); err != nil {
		t.Skip(err)
	}
	info, err := c.GetServerInfo()
	if err != nil || info.Version == "" {
		t.Fatal(err, info)
	}
	if err := c.SwitchDatabase("demo"); err != nil {
		t.Fatal(err)
	}
	if err := c.SwitchDatabase("demo"); err != nil {
		t.Fatal(err)
	}
	lat, sinfo, err := c.TestConnection(conn)
	if err != nil || lat <= 0 || sinfo.Version == "" {
		t.Fatal(err, lat, sinfo)
	}
	if err := c.Disconnect(); err != nil {
		t.Fatal(err)
	}
}

func TestLiveConnectBad(t *testing.T) {
	c := NewClient()
	err := c.Connect(types.Connection{
		Host: "127.0.0.1", Port: 1, Username: "postgres", Password: "x",
		Database: "postgres", SSLMode: types.SSLModeDisable,
	})
	if err == nil {
		t.Fatal("expected connect fail")
	}
	_, _, err = c.TestConnection(types.Connection{
		Host: "127.0.0.1", Port: 1, Username: "x", Password: "x", SSLMode: types.SSLModeDisable,
	})
	if err == nil {
		t.Fatal("expected test fail")
	}
}

func TestLiveListDatabasesSchemasObjects(t *testing.T) {
	c := liveClient(t)
	dbs, err := c.ListDatabases()
	if err != nil || len(dbs) == 0 {
		t.Fatal(err, dbs)
	}
	schemas, err := c.ListSchemas()
	if err != nil || len(schemas) == 0 {
		t.Fatal(err, schemas)
	}
	for _, kind := range []types.ObjectKind{
		types.ObjectTable, types.ObjectView, types.ObjectMatView, types.ObjectSequence,
		types.ObjectFunction, types.ObjectType, types.ObjectExtension, "",
		types.ObjectKind("unknown"),
	} {
		objs, err := c.ListObjects("public", kind)
		if err != nil {
			t.Fatalf("kind %s: %v", kind, err)
		}
		t.Logf("kind=%s count=%d", kind, len(objs))
	}
	if _, err := c.ListObjects("", types.ObjectTable); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListObjects("", types.ObjectSequence); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListObjects("", types.ObjectFunction); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListObjects("", types.ObjectType); err != nil {
		t.Fatal(err)
	}
}

func TestLiveGetObjectDetails(t *testing.T) {
	c := liveClient(t)
	d, err := c.GetObjectDetail("public", "users", types.ObjectTable)
	if err != nil || len(d.Columns) == 0 {
		t.Fatal(err, d)
	}
	d, err = c.GetObjectDetail("public", "users", "")
	if err != nil {
		t.Fatal(err)
	}
	d, err = c.GetObjectDetail("public", "users", types.ObjectKind("other"))
	if err != nil {
		t.Fatal(err)
	}

	seqs, _ := c.ListObjects("public", types.ObjectSequence)
	if len(seqs) > 0 {
		sd, err := c.GetObjectDetail(seqs[0].Schema, seqs[0].Name, types.ObjectSequence)
		if err != nil {
			t.Fatal(err)
		}
		if len(sd.Props) == 0 {
			t.Fatal("seq props")
		}
	} else {
		if _, err := c.GetObjectDetail("public", "no_such_seq", types.ObjectSequence); err == nil {
			t.Fatal("expected seq miss")
		}
	}

	fns, _ := c.ListObjects("public", types.ObjectFunction)
	if len(fns) > 0 {
		if _, err := c.GetObjectDetail(fns[0].Schema, fns[0].Name, types.ObjectFunction); err != nil {
			t.Fatal(err)
		}
	} else if _, err := c.GetObjectDetail("public", "no_fn", types.ObjectFunction); err == nil {
		t.Fatal("fn miss")
	}

	typesList, _ := c.ListObjects("public", types.ObjectType)
	if len(typesList) > 0 {
		if _, err := c.GetObjectDetail(typesList[0].Schema, typesList[0].Name, types.ObjectType); err != nil {
			t.Fatal(err)
		}
	} else if _, err := c.GetObjectDetail("public", "no_type", types.ObjectType); err == nil {
		t.Fatal("type miss")
	}

	exts, err := c.ListExtensions()
	if err != nil {
		t.Fatal(err)
	}
	if len(exts) > 0 {
		ed, err := c.GetObjectDetail(exts[0].Schema, exts[0].Name, types.ObjectExtension)
		if err != nil {
			t.Fatal(err)
		}
		_ = ed
	}
	if _, err := c.GetObjectDetail("public", "no_ext", types.ObjectExtension); err == nil {
		t.Fatal("ext miss")
	}

	views, _ := c.ListObjects("public", types.ObjectView)
	if len(views) > 0 {
		vd, err := c.GetTableDetail(views[0].Schema, views[0].Name)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("view def len=%d", len(vd.CreateSQL))
	}
}

func TestLiveQueryAndData(t *testing.T) {
	c := liveClient(t)
	res, err := c.GetTableData("public", "users", 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Columns) == 0 {
		t.Fatal("cols")
	}
	if _, err := c.GetTableData("public", "users", -1, 0); err != nil {
		t.Fatal(err)
	}

	qr, err := c.RunQuery("SELECT 1 AS n", 10)
	if err != nil || !qr.IsSelect || len(qr.Rows) != 1 {
		t.Fatal(err, qr)
	}
	if _, err := c.RunQuery("SELECT 1 LIMIT 1", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RunQuery("   ", 10); err == nil {
		t.Fatal("empty")
	}
	if _, err := c.RunQuery("SET application_name = 'postgres-tui-test'", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RunQuery("SELECT * FROM no_such_table_zzz", 10); err == nil {
		t.Fatal("bad")
	}
	if _, err := c.RunQuery("SELECT 1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RunQuery("SELECT 1", maxQueryRows+10); err != nil {
		t.Fatal(err)
	}

	ro := NewClient()
	if err := ro.Connect(types.Connection{
		Host: "localhost", Port: 5432, Username: "postgres", Password: "postgres",
		Database: "demo", SSLMode: types.SSLModeDisable, ReadOnly: true,
	}); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { _ = ro.Disconnect() })
	if !ro.IsReadOnly() {
		t.Fatal("ro")
	}
	if _, err := ro.RunQuery("DELETE FROM users WHERE false", 10); err == nil {
		t.Fatal("expected ro block")
	}
	if err := ro.CancelQuery(1); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatal(err)
	}
	_ = c.CancelQuery(0)
}

func TestLiveListERDDefaultSchema(t *testing.T) {
	c := liveClient(t)
	g, err := c.ListERD("")
	if err != nil {
		t.Fatal(err)
	}
	if g.Schema != "public" {
		t.Fatal(g.Schema)
	}
}
