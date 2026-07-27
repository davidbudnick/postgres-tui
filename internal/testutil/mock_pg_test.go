package testutil

import (
	"testing"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestMockPG_AllMethods(t *testing.T) {
	m := NewMockPG()
	conn := types.Connection{Host: "localhost", Port: 5432, Database: "app", ReadOnly: true}

	if err := m.Connect(conn); err != nil {
		t.Fatal(err)
	}
	if !m.IsConnected() || !m.IsReadOnly() || m.CurrentDatabase() != "app" {
		t.Fatalf("state connected=%v ro=%v db=%s", m.IsConnected(), m.IsReadOnly(), m.CurrentDatabase())
	}

	// Connect without database falls back to postgres
	m2 := NewMockPG()
	if err := m2.Connect(types.Connection{Host: "h", Port: 0}); err != nil {
		t.Fatal(err)
	}
	if m2.CurrentDatabase() != "postgres" || m2.Info.Port != 5432 {
		t.Fatalf("db=%s port=%d", m2.CurrentDatabase(), m2.Info.Port)
	}

	lat, info, err := m.TestConnection(conn)
	if err != nil || lat <= 0 || info.Version == "" {
		t.Fatalf("test: %v lat=%v info=%+v", err, lat, info)
	}

	if err := m.SwitchDatabase("analytics"); err != nil {
		t.Fatal(err)
	}
	if m.CurrentDatabase() != "analytics" {
		t.Fatal(m.CurrentDatabase())
	}

	if _, err := m.GetServerInfo(); err != nil {
		t.Fatal(err)
	}
	if dbs, err := m.ListDatabases(); err != nil || len(dbs) == 0 {
		t.Fatal(err, dbs)
	}
	if schemas, err := m.ListSchemas(); err != nil || len(schemas) == 0 {
		t.Fatal(err, schemas)
	}

	objs, err := m.ListObjects("public", types.ObjectTable)
	if err != nil || len(objs) == 0 {
		t.Fatal(err, objs)
	}
	views, err := m.ListObjects("public", types.ObjectView)
	if err != nil {
		t.Fatal(err)
	}
	_ = views
	all, err := m.ListObjects("", "")
	if err != nil || len(all) == 0 {
		t.Fatal(err, all)
	}
	// schema filter miss
	if filtered, err := m.ListObjects("missing", types.ObjectTable); err != nil || len(filtered) != 0 {
		t.Fatalf("filtered=%v err=%v", filtered, err)
	}

	detail, err := m.GetTableDetail("public", "users")
	if err != nil || detail.Object.Name != "users" {
		t.Fatal(err, detail)
	}
	od, err := m.GetObjectDetail("public", "seq", types.ObjectSequence)
	if err != nil || len(od.Props) == 0 {
		t.Fatal(err, od)
	}
	// table kind should not invent props when already empty of non-table
	m.Detail.Props = nil
	od2, err := m.GetObjectDetail("public", "users", types.ObjectTable)
	if err != nil {
		t.Fatal(err)
	}
	_ = od2

	data, err := m.GetTableData("public", "users", 0, 10)
	if err != nil || len(data.Rows) == 0 {
		t.Fatal(err, data)
	}
	qr, err := m.RunQuery("SELECT 1", 10)
	if err != nil || qr.SQL != "SELECT 1" {
		t.Fatal(err, qr)
	}
	act, err := m.ListActivity()
	if err != nil || len(act) == 0 {
		t.Fatal(err, act)
	}
	erd, err := m.ListERD("billing")
	if err != nil || erd.Schema != "billing" {
		t.Fatal(err, erd)
	}
	// empty schema keeps default
	erd2, err := m.ListERD("")
	if err != nil {
		t.Fatal(err)
	}
	_ = erd2
	exts, err := m.ListExtensions()
	if err != nil || len(exts) == 0 {
		t.Fatal(err, exts)
	}
	if err := m.CancelQuery(1); err != nil {
		t.Fatal(err)
	}
	if err := m.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if m.IsConnected() {
		t.Fatal("still connected")
	}
}

func TestMockPG_FailNext(t *testing.T) {
	ops := []struct {
		name string
		call func(*MockPG) error
	}{
		{"connect", func(m *MockPG) error { return m.Connect(types.Connection{}) }},
		{"test", func(m *MockPG) error { _, _, err := m.TestConnection(types.Connection{}); return err }},
		{"switch", func(m *MockPG) error { return m.SwitchDatabase("x") }},
		{"info", func(m *MockPG) error { _, err := m.GetServerInfo(); return err }},
		{"databases", func(m *MockPG) error { _, err := m.ListDatabases(); return err }},
		{"schemas", func(m *MockPG) error { _, err := m.ListSchemas(); return err }},
		{"objects", func(m *MockPG) error { _, err := m.ListObjects("public", types.ObjectTable); return err }},
		{"detail", func(m *MockPG) error { _, err := m.GetTableDetail("public", "t"); return err }},
		{"object_detail", func(m *MockPG) error {
			_, err := m.GetObjectDetail("public", "t", types.ObjectType)
			return err
		}},
		{"data", func(m *MockPG) error { _, err := m.GetTableData("public", "t", 0, 1); return err }},
		{"query", func(m *MockPG) error { _, err := m.RunQuery("SELECT 1", 1); return err }},
		{"activity", func(m *MockPG) error { _, err := m.ListActivity(); return err }},
		{"erd", func(m *MockPG) error { _, err := m.ListERD("public"); return err }},
		{"ext", func(m *MockPG) error { _, err := m.ListExtensions(); return err }},
		{"cancel", func(m *MockPG) error { return m.CancelQuery(1) }},
		{"*", func(m *MockPG) error { return m.Connect(types.Connection{}) }},
	}
	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			m := NewMockPG()
			m.FailNext = op.name
			if err := op.call(m); err == nil {
				t.Fatal("expected fail")
			}
		})
	}
}
