package types

import "testing"

func TestConnectionDSN(t *testing.T) {
	c := Connection{
		Host:     "localhost",
		Port:     5432,
		Username: "postgres",
		Password: "secret",
		Database: "app",
		SSLMode:  SSLModeDisable,
	}
	dsn := c.DSN()
	if dsn == "" {
		t.Fatal("empty dsn")
	}
	if !containsAll(dsn, "localhost", "5432", "postgres", "app", "sslmode=disable") {
		t.Fatalf("unexpected dsn: %s", dsn)
	}
}

func TestConnectionDisplayHost(t *testing.T) {
	c := Connection{Host: "db.local", Port: 5433, Database: "main"}
	if got := c.DisplayHost(); got != "db.local:5433/main" {
		t.Fatalf("got %q", got)
	}
	if got := (Connection{Host: "h", Port: 1}).Address(); got != "h:1" {
		t.Fatalf("got %q", got)
	}
}

func TestSchemaObjectFullName(t *testing.T) {
	o := SchemaObject{Schema: "public", Name: "users"}
	if o.FullName() != "public.users" {
		t.Fatalf("got %q", o.FullName())
	}
	if (SchemaObject{Name: "x"}).FullName() != "x" {
		t.Fatal("expected bare name")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
