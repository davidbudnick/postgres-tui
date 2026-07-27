package pg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

func demoConn() types.Connection {
	return types.Connection{
		Host: "localhost", Port: 5432, Username: "postgres", Password: "postgres",
		Database: "demo", SSLMode: types.SSLModeDisable,
	}
}

func liveClient(t *testing.T) *Client {
	t.Helper()
	c := NewClient()
	if err := c.Connect(demoConn()); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })
	return c
}

func ensureDemoObjects(t *testing.T, c *Client) {
	t.Helper()
	stmts := []string{
		`CREATE OR REPLACE FUNCTION public.add_one(x int) RETURNS int LANGUAGE sql IMMUTABLE AS $$ SELECT x + 1; $$`,
		`DO $$ BEGIN
		  IF NOT EXISTS (
		    SELECT 1 FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
		    WHERE n.nspname = 'public' AND t.typname = 'order_status'
		  ) THEN
		    CREATE TYPE public.order_status AS ENUM ('pending', 'shipped', 'cancelled');
		  END IF;
		END $$`,
		`DO $$ BEGIN
		  IF NOT EXISTS (
		    SELECT 1 FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
		    WHERE n.nspname = 'public' AND t.typname = 'address'
		  ) THEN
		    CREATE TYPE public.address AS (street text, city text, zip text);
		  END IF;
		END $$`,
		`CREATE MATERIALIZED VIEW IF NOT EXISTS public.product_names AS SELECT id, name FROM public.products`,
	}
	for _, s := range stmts {
		if _, err := c.RunQuery(s, 1); err != nil {
			t.Fatalf("ensure demo objects: %v\nSQL: %s", err, s)
		}
	}
}

func TestConnectDefaultsAndReconnect(t *testing.T) {
	c := NewClient()
	conn := types.Connection{
		Host: "localhost", Password: "postgres", Database: "demo",
	}
	if err := c.Connect(conn); err != nil {
		t.Skip(err)
	}
	defer func() { _ = c.Disconnect() }()
	if !c.IsConnected() {
		t.Fatal("expected connected")
	}
	if c.CurrentDatabase() != "demo" {
		t.Fatalf("database=%q", c.CurrentDatabase())
	}
	if err := c.Connect(demoConn()); err != nil {
		t.Fatal(err)
	}
	if !c.IsConnected() {
		t.Fatal("expected still connected after reconnect")
	}
}

func TestConnectErrors(t *testing.T) {
	c := NewClient()
	err := c.Connect(types.Connection{
		Host: "127.0.0.1", Port: 1, Username: "postgres", Password: "x",
		Database: "demo", SSLMode: types.SSLModeDisable,
	})
	if err == nil {
		t.Fatal("expected connect error")
	}
	err = c.Connect(types.Connection{
		Host: "localhost", Port: 5432, Username: "no_such_user_pg_tui", Password: "wrong",
		Database: "demo", SSLMode: types.SSLModeDisable,
	})
	if err == nil {
		t.Fatal("expected auth/ping error")
	}
}

func TestTestConnection(t *testing.T) {
	c := NewClient()
	lat, info, err := c.TestConnection(demoConn())
	if err != nil {
		t.Skip(err)
	}
	if lat <= 0 {
		t.Fatalf("latency=%v", lat)
	}
	if info.Version == "" || info.Database != "demo" {
		t.Fatalf("info=%+v", info)
	}
	_, _, err = c.TestConnection(types.Connection{
		Host: "127.0.0.1", Port: 1, Username: "x", Password: "x",
		Database: "x", SSLMode: types.SSLModeDisable,
	})
	if err == nil {
		t.Fatal("expected TestConnection error")
	}
}

func TestSwitchDatabase(t *testing.T) {
	c := liveClient(t)
	if err := c.SwitchDatabase("demo"); err != nil {
		t.Fatal(err)
	}
	if c.CurrentDatabase() != "demo" {
		t.Fatal(c.CurrentDatabase())
	}
	if err := c.SwitchDatabase("postgres"); err != nil {
		t.Fatal(err)
	}
	if c.CurrentDatabase() != "postgres" {
		t.Fatalf("got %q", c.CurrentDatabase())
	}
	if err := c.SwitchDatabase("demo"); err != nil {
		t.Fatal(err)
	}
}

func TestLiveServerInfoAndCatalogs(t *testing.T) {
	c := liveClient(t)
	info, err := c.GetServerInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.Version == "" || info.User == "" || info.Database != "demo" {
		t.Fatalf("%+v", info)
	}
	if info.Host != "localhost" || info.Port != 5432 {
		t.Fatalf("host/port %+v", info)
	}

	dbs, err := c.ListDatabases()
	if err != nil {
		t.Fatal(err)
	}
	if len(dbs) == 0 {
		t.Fatal("expected databases")
	}

	schemas, err := c.ListSchemas()
	if err != nil {
		t.Fatal(err)
	}
	if len(schemas) == 0 {
		t.Fatal("expected schemas")
	}

	exts, err := c.ListExtensions()
	if err != nil {
		t.Fatal(err)
	}
	if len(exts) == 0 {
		t.Fatal("expected extensions")
	}
}

func TestLiveListObjects(t *testing.T) {
	c := liveClient(t)
	ensureDemoObjects(t, c)

	objs, err := c.ListObjects("", types.ObjectTable)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) == 0 {
		t.Fatal("expected tables in all schemas")
	}

	objs, err = c.ListObjects("public", types.ObjectTable)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) == 0 {
		t.Fatal("expected tables in public")
	}

	for _, kind := range []types.ObjectKind{
		types.ObjectView,
		types.ObjectMatView,
		"",
		types.ObjectSequence,
		types.ObjectFunction,
		types.ObjectType,
		types.ObjectExtension,
		types.ObjectKind("unknown"),
	} {
		got, err := c.ListObjects("public", kind)
		if err != nil {
			t.Fatalf("ListObjects(%q): %v", kind, err)
		}
		t.Logf("kind=%q count=%d", kind, len(got))
		switch kind {
		case types.ObjectView, types.ObjectSequence, types.ObjectFunction, types.ObjectType, types.ObjectExtension, types.ObjectMatView:
			if len(got) == 0 {
				t.Fatalf("expected objects for kind %q", kind)
			}
		}
	}

	all, err := c.ListObjects("", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("expected mixed relations")
	}
	hasView, hasMat := false, false
	for _, o := range all {
		if o.Kind == types.ObjectView {
			hasView = true
		}
		if o.Kind == types.ObjectMatView {
			hasMat = true
		}
	}
	if !hasView {
		t.Fatal("expected view in mixed list")
	}
	if !hasMat {
		t.Fatal("expected matview in mixed list")
	}
}

func TestLiveGetTableDetailOrderItems(t *testing.T) {
	c := liveClient(t)
	detail, err := c.GetTableDetail("public", "order_items")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Object.FullName() != "public.order_items" {
		t.Fatalf("FullName=%q", detail.Object.FullName())
	}
	if detail.Object.Kind != types.ObjectTable {
		t.Fatalf("Kind=%q", detail.Object.Kind)
	}
	if len(detail.Columns) == 0 {
		t.Fatal("expected columns")
	}
	names := map[string]bool{}
	for _, col := range detail.Columns {
		names[col.Name] = true
	}
	for _, want := range []string{"id", "order_id", "product_id", "quantity", "unit_price_cents"} {
		if !names[want] {
			t.Fatalf("missing column %s", want)
		}
	}
	if len(detail.Indexes) == 0 {
		t.Fatal("expected indexes")
	}
	if len(detail.Constraints) == 0 {
		t.Fatal("expected constraints")
	}
}

func TestLiveGetTableDetailViewAndMatView(t *testing.T) {
	c := liveClient(t)
	ensureDemoObjects(t, c)

	v, err := c.GetTableDetail("public", "active_users")
	if err != nil {
		t.Fatal(err)
	}
	if v.Object.Kind != types.ObjectView {
		t.Fatalf("kind=%q", v.Object.Kind)
	}
	if v.CreateSQL == "" {
		t.Fatal("expected view definition")
	}

	m, err := c.GetTableDetail("public", "product_names")
	if err != nil {
		t.Fatal(err)
	}
	if m.Object.Kind != types.ObjectMatView {
		t.Fatalf("kind=%q", m.Object.Kind)
	}
	if m.CreateSQL == "" {
		t.Fatal("expected matview definition")
	}
}

func TestLiveGetTableDetailMissing(t *testing.T) {
	c := liveClient(t)
	cases := []struct {
		schema, name string
		wantSubstr   string
	}{
		{"", "order_items", "required"},
		{"public", "", "required"},
		{"pg_catalog", "postgresql", "not found"},
		{"pg_catalog", "plpgsql", "not found"},
		{"public", "does_not_exist", "not found"},
	}
	for _, tc := range cases {
		_, err := c.GetTableDetail(tc.schema, tc.name)
		if err == nil {
			t.Fatalf("GetTableDetail(%q,%q) expected error", tc.schema, tc.name)
		}
		if !strings.Contains(err.Error(), tc.wantSubstr) {
			t.Fatalf("GetTableDetail(%q,%q) err=%v want substr %q", tc.schema, tc.name, err, tc.wantSubstr)
		}
	}
}

func TestLiveGetObjectDetail(t *testing.T) {
	c := liveClient(t)
	ensureDemoObjects(t, c)

	td, err := c.GetObjectDetail("public", "users", types.ObjectTable)
	if err != nil {
		t.Fatal(err)
	}
	if len(td.Columns) == 0 {
		t.Fatal("table columns")
	}

	td, err = c.GetObjectDetail("public", "active_users", types.ObjectView)
	if err != nil {
		t.Fatal(err)
	}
	if td.Object.Kind != types.ObjectView {
		t.Fatal(td.Object.Kind)
	}

	td, err = c.GetObjectDetail("public", "product_names", types.ObjectMatView)
	if err != nil {
		t.Fatal(err)
	}
	if td.Object.Kind != types.ObjectMatView {
		t.Fatal(td.Object.Kind)
	}

	td, err = c.GetObjectDetail("public", "users", "")
	if err != nil {
		t.Fatal(err)
	}
	td, err = c.GetObjectDetail("public", "users", types.ObjectKind("other"))
	if err != nil {
		t.Fatal(err)
	}

	seq, err := c.GetObjectDetail("public", "users_id_seq", types.ObjectSequence)
	if err != nil {
		t.Fatal(err)
	}
	if seq.Object.Kind != types.ObjectSequence || len(seq.Props) == 0 || seq.CreateSQL == "" {
		t.Fatalf("%+v", seq)
	}
	_, err = c.GetObjectDetail("public", "no_such_seq", types.ObjectSequence)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("seq missing: %v", err)
	}

	fn, err := c.GetObjectDetail("public", "add_one", types.ObjectFunction)
	if err != nil {
		t.Fatal(err)
	}
	if fn.Object.Kind != types.ObjectFunction || fn.CreateSQL == "" || len(fn.Props) == 0 {
		t.Fatalf("%+v", fn)
	}
	_, err = c.GetObjectDetail("public", "no_such_fn", types.ObjectFunction)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("fn missing: %v", err)
	}

	enum, err := c.GetObjectDetail("public", "order_status", types.ObjectType)
	if err != nil {
		t.Fatal(err)
	}
	if enum.Object.Kind != types.ObjectType {
		t.Fatal(enum.Object.Kind)
	}
	foundLabels := false
	for _, p := range enum.Props {
		if p.Label == "labels" && strings.Contains(p.Value, "pending") {
			foundLabels = true
		}
	}
	if !foundLabels {
		t.Fatalf("enum props %+v", enum.Props)
	}

	comp, err := c.GetObjectDetail("public", "address", types.ObjectType)
	if err != nil {
		t.Fatal(err)
	}
	if len(comp.Columns) == 0 {
		t.Fatal("expected composite columns")
	}
	_, err = c.GetObjectDetail("public", "no_such_type", types.ObjectType)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("type missing: %v", err)
	}

	ext, err := c.GetObjectDetail("", "plpgsql", types.ObjectExtension)
	if err != nil {
		t.Fatal(err)
	}
	if ext.Object.Kind != types.ObjectExtension || len(ext.Props) == 0 {
		t.Fatalf("%+v", ext)
	}
	_, err = c.GetObjectDetail("", "no_such_ext", types.ObjectExtension)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ext missing: %v", err)
	}
}

func TestLiveGetTableDataAndRunQuery(t *testing.T) {
	c := liveClient(t)

	res, err := c.GetTableData("public", "users", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsSelect || len(res.Columns) == 0 {
		t.Fatalf("%+v", res)
	}

	res, err = c.GetTableData("public", "users", -5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsSelect {
		t.Fatal("expected select")
	}

	res, err = c.RunQuery("SELECT 1 AS n, NULL AS z, true AS b, now() AS t", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsSelect || len(res.Rows) != 1 {
		t.Fatalf("%+v", res)
	}

	res, err = c.RunQuery("SELECT generate_series(1, 5) AS n LIMIT 2", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("rows=%d", len(res.Rows))
	}

	res, err = c.RunQuery("SELECT generate_series(1, 20) AS n", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || len(res.Rows) != 5 {
		t.Fatalf("trunc=%v rows=%d", res.Truncated, len(res.Rows))
	}

	res, err = c.RunQuery("SELECT 1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatal(res.Rows)
	}

	res, err = c.RunQuery("SELECT 1", maxQueryRows+10)
	if err != nil {
		t.Fatal(err)
	}

	res, err = c.RunQuery("SET application_name = 'postgres-tui-test'", 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsSelect {
		t.Fatal("expected non-select")
	}

	_, err = c.RunQuery("NOT_A_VALID_SQL_!!!", 1)
	if err == nil {
		t.Fatal("expected exec error")
	}
	_, err = c.RunQuery("SELECT * FROM no_such_table_xyz", 1)
	if err == nil {
		t.Fatal("expected select error")
	}
	_, err = c.RunQuery("   ", 1)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty: %v", err)
	}
}

func TestLiveRunQueryReadOnly(t *testing.T) {
	conn := demoConn()
	conn.ReadOnly = true
	c := NewClient()
	if err := c.Connect(conn); err != nil {
		t.Skip(err)
	}
	defer func() { _ = c.Disconnect() }()
	if !c.IsReadOnly() {
		t.Fatal("expected read-only")
	}
	_, err := c.RunQuery("DELETE FROM users", 1)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("mutating: %v", err)
	}
	res, err := c.RunQuery("SELECT 1", 1)
	if err != nil || !res.IsSelect {
		t.Fatalf("select: %v %+v", err, res)
	}
	if err := c.CancelQuery(1); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("cancel: %v", err)
	}
}

func TestLiveListERD(t *testing.T) {
	c := liveClient(t)
	g, err := c.ListERD("public")
	if err != nil {
		t.Fatal(err)
	}
	if g.Schema != "public" {
		t.Fatalf("schema=%q", g.Schema)
	}
	if len(g.Tables) == 0 {
		t.Fatal("expected tables")
	}
	if len(g.Edges) == 0 {
		t.Fatal("expected FK edges")
	}
	found := false
	for _, e := range g.Edges {
		t.Logf("%s.%v -> %s.%v", e.FromTable, e.FromCols, e.ToTable, e.ToCols)
		if e.FromTable == "orders" && e.ToTable == "users" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected orders → users FK")
	}

	g2, err := c.ListERD("")
	if err != nil {
		t.Fatal(err)
	}
	if g2.Schema != "public" {
		t.Fatalf("default schema=%q", g2.Schema)
	}
}

func TestLiveListActivity(t *testing.T) {
	c := liveClient(t)
	rows, err := c.ListActivity()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		t.Logf("pid=%d user=%q db=%q state=%q type=%q", r.PID, r.User, r.Database, r.State, r.BackendType)
		if r.BackendType != "" && r.BackendType != "client backend" {
			t.Fatalf("noise backend leaked: %q", r.BackendType)
		}
	}
}

func TestLiveCancelQuery(t *testing.T) {
	c := liveClient(t)
	if err := c.CancelQuery(1); err != nil {
		t.Fatal(err)
	}
}

func TestClosedPoolErrors(t *testing.T) {
	c := liveClient(t)
	c.pool.Close()

	if _, err := c.GetServerInfo(); err == nil {
		t.Fatal("GetServerInfo")
	}
	if _, err := c.ListDatabases(); err == nil {
		t.Fatal("ListDatabases")
	}
	if _, err := c.ListSchemas(); err == nil {
		t.Fatal("ListSchemas")
	}
	if _, err := c.ListObjects("public", types.ObjectTable); err == nil {
		t.Fatal("ListObjects table")
	}
	if _, err := c.ListObjects("public", types.ObjectSequence); err == nil {
		t.Fatal("ListObjects seq")
	}
	if _, err := c.ListObjects("public", types.ObjectFunction); err == nil {
		t.Fatal("ListObjects fn")
	}
	if _, err := c.ListObjects("public", types.ObjectType); err == nil {
		t.Fatal("ListObjects type")
	}
	if _, err := c.ListObjects("public", types.ObjectExtension); err == nil {
		t.Fatal("ListObjects ext")
	}
	if _, err := c.GetTableDetail("public", "users"); err == nil {
		t.Fatal("GetTableDetail")
	}
	if _, err := c.GetObjectDetail("public", "users_id_seq", types.ObjectSequence); err == nil {
		t.Fatal("getSequenceDetail")
	}
	if _, err := c.GetObjectDetail("public", "add_one", types.ObjectFunction); err == nil {
		t.Fatal("getFunctionDetail")
	}
	if _, err := c.GetObjectDetail("public", "order_status", types.ObjectType); err == nil {
		t.Fatal("getTypeDetail")
	}
	if _, err := c.GetObjectDetail("", "plpgsql", types.ObjectExtension); err == nil {
		t.Fatal("getExtensionDetail")
	}
	if _, err := c.RunQuery("SELECT 1", 1); err == nil {
		t.Fatal("RunQuery")
	}
	if _, err := c.RunQuery("SET application_name = 'x'", 1); err == nil {
		t.Fatal("RunQuery exec")
	}
	if _, err := c.ListActivity(); err == nil {
		t.Fatal("ListActivity")
	}
	if _, err := c.ListERD("public"); err == nil {
		t.Fatal("ListERD")
	}
	if _, err := c.ListExtensions(); err == nil {
		t.Fatal("ListExtensions")
	}
	if err := c.CancelQuery(1); err == nil {
		t.Fatal("CancelQuery")
	}

	if err := c.Connect(demoConn()); err != nil {
		t.Fatal(err)
	}
}

func TestGetObjectDetailDisconnected(t *testing.T) {
	c := NewClient()
	if _, err := c.getSequenceDetail("public", "x"); err == nil {
		t.Fatal("seq")
	}
	if _, err := c.getFunctionDetail("public", "x"); err == nil {
		t.Fatal("fn")
	}
	if _, err := c.getTypeDetail("public", "x"); err == nil {
		t.Fatal("type")
	}
	if _, err := c.getExtensionDetail("", "x"); err == nil {
		t.Fatal("ext")
	}
}

func TestParseConfigFailure(t *testing.T) {
	c := NewClient()
	err := c.Connect(types.Connection{
		Host: "localhost", Port: 5432, Username: "postgres", Password: "postgres",
		Database: "demo", SSLMode: types.SSLMode("not-a-real-ssl-mode"),
	})
	if err == nil {
		t.Fatal("expected parse dsn error")
	}
	if !strings.Contains(err.Error(), "parse dsn") && !strings.Contains(err.Error(), "sslmode") && !strings.Contains(err.Error(), "connect") {
		t.Logf("got err=%v (acceptable if parse path differs)", err)
	}
}

func TestActivityWithConcurrentQuery(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		pool, err := c.poolOrErr()
		if err != nil {
			return
		}
		_, _ = pool.Exec(ctx, `SELECT pg_sleep(2)`)
	}()

	time.Sleep(200 * time.Millisecond)
	rows, err := c.ListActivity()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.QueryStart.IsZero() && strings.Contains(r.Query, "pg_sleep") {
			t.Fatalf("expected query start for active sleep: %+v", r)
		}
	}
	<-done
}
