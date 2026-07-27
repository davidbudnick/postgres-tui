package pg

import (
	"strings"
	"testing"
	"time"
)

func TestQuoteIdent(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`users`, `"users"`},
		{`a"b`, `"a""b"`},
		{``, `""`},
	}
	for _, tc := range cases {
		if got := quoteIdent(tc.in); got != tc.want {
			t.Fatalf("quoteIdent(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatCell(t *testing.T) {
	long := strings.Repeat("x", maxCellBytes+10)
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	type longStr string
	cases := []struct {
		name  string
		in    any
		check func(string) bool
	}{
		{"nil", nil, func(s string) bool { return s == "NULL" }},
		{"bytes", []byte("hi"), func(s string) bool { return s == "hi" }},
		{"bytes truncate", []byte(long), func(s string) bool { return strings.HasSuffix(s, "…") }},
		{"time", ts, func(s string) bool { return s == ts.Format(time.RFC3339) }},
		{"bool true", true, func(s string) bool { return s == "true" }},
		{"bool false", false, func(s string) bool { return s == "false" }},
		{"int32", int32(7), func(s string) bool { return s == "7" }},
		{"int64", int64(8), func(s string) bool { return s == "8" }},
		{"float32", float32(1.5), func(s string) bool { return s != "" }},
		{"float64", float64(2.5), func(s string) bool { return s != "" }},
		{"string", "ok", func(s string) bool { return s == "ok" }},
		{"string truncate", long, func(s string) bool { return strings.HasSuffix(s, "…") }},
		{"default", struct{ X int }{1}, func(s string) bool { return s != "" }},
		{"default truncate", longStr(long), func(s string) bool { return strings.HasSuffix(s, "…") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatCell(tc.in); !tc.check(got) {
				t.Fatalf("formatCell=%q", got)
			}
		})
	}
}

func TestLooksLikeSelect(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"SELECT 1", true},
		{"  (SELECT 1)", true},
		{"with x as (select 1) select * from x", true},
		{"SHOW server_version", true},
		{"EXPLAIN SELECT 1", true},
		{"VALUES (1)", true},
		{"TABLE users", true},
		{"INSERT INTO t", false},
		{"update t set a=1", false},
	}
	for _, tc := range cases {
		if got := looksLikeSelect(tc.sql); got != tc.want {
			t.Fatalf("%q got %v want %v", tc.sql, got, tc.want)
		}
	}
}

func TestIsMutatingSQL(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"INSERT", true},
		{"UPDATE t", true},
		{"DELETE", true},
		{"DROP", true},
		{"ALTER", true},
		{"CREATE", true},
		{"TRUNCATE", true},
		{"GRANT", true},
		{"REVOKE", true},
		{"VACUUM", true},
		{"REINDEX", true},
		{"CLUSTER", true},
		{"COMMENT", true},
		{"COPY", true},
		{"CALL", true},
		{"DO $$", true},
		{"SECURITY", true},
		{"SELECT 1", false},
	}
	for _, tc := range cases {
		if got := isMutatingSQL(tc.sql); got != tc.want {
			t.Fatalf("%q got %v want %v", tc.sql, got, tc.want)
		}
	}
}

func TestHasLimitClause(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		{"SELECT 1 LIMIT 10", true},
		{"SELECT 1", false},
		{"select 1 limit 5", true},
	}
	for _, tc := range cases {
		if got := hasLimitClause(tc.sql); got != tc.want {
			t.Fatalf("%q got %v want %v", tc.sql, got, tc.want)
		}
	}
}

func TestConstraintTypeName(t *testing.T) {
	cases := []struct {
		code, want string
	}{
		{"p", "PRIMARY KEY"},
		{"f", "FOREIGN KEY"},
		{"u", "UNIQUE"},
		{"c", "CHECK"},
		{"x", "EXCLUDE"},
		{"z", "z"},
	}
	for _, tc := range cases {
		if got := constraintTypeName(tc.code); got != tc.want {
			t.Fatalf("%s: %s want %s", tc.code, got, tc.want)
		}
	}
}

func TestClientDisconnectedHelpers(t *testing.T) {
	c := NewClient()
	if c.IsConnected() {
		t.Fatal("connected")
	}
	if c.IsReadOnly() {
		t.Fatal("ro")
	}
	if c.CurrentDatabase() != "" {
		t.Fatal(c.CurrentDatabase())
	}
	if _, err := c.poolOrErr(); err == nil {
		t.Fatal("pool")
	}
	if _, err := c.GetServerInfo(); err == nil {
		t.Fatal("info")
	}
	if _, err := c.ListDatabases(); err == nil {
		t.Fatal("dbs")
	}
	if _, err := c.ListSchemas(); err == nil {
		t.Fatal("schemas")
	}
	if _, err := c.ListObjects("public", ""); err == nil {
		t.Fatal("objects")
	}
	if _, err := c.GetTableDetail("public", "t"); err == nil {
		t.Fatal("detail")
	}
	if _, err := c.GetObjectDetail("public", "t", ""); err == nil {
		t.Fatal("obj detail")
	}
	if _, err := c.GetTableData("public", "t", 0, 10); err == nil {
		t.Fatal("data")
	}
	if _, err := c.RunQuery("SELECT 1", 10); err == nil {
		t.Fatal("query")
	}
	if _, err := c.ListActivity(); err == nil {
		t.Fatal("activity")
	}
	if _, err := c.ListERD("public"); err == nil {
		t.Fatal("erd")
	}
	if _, err := c.ListExtensions(); err == nil {
		t.Fatal("ext")
	}
	if err := c.CancelQuery(1); err == nil {
		t.Fatal("cancel")
	}
	if err := c.Disconnect(); err != nil {
		t.Fatal(err)
	}
}
