package pg

import (
	"strings"
	"testing"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

func liveClient(t *testing.T) *Client {
	t.Helper()
	c := NewClient()
	err := c.Connect(types.Connection{
		Host: "localhost", Port: 5432, Username: "postgres", Password: "postgres",
		Database: "demo", SSLMode: types.SSLModeDisable,
	})
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })
	return c
}

func TestLiveListObjects(t *testing.T) {
	c := liveClient(t)
	objs, err := c.ListObjects("", types.ObjectTable)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("empty schema count=%d", len(objs))
	for _, o := range objs {
		t.Logf("%s.%s kind=%s", o.Schema, o.Name, o.Kind)
	}
	objs, err = c.ListObjects("public", types.ObjectTable)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("public count=%d", len(objs))
	if len(objs) == 0 {
		t.Fatal("expected tables in public")
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
