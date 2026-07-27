package main

import (
	"flag"
	"testing"
)

func TestParseFlags_NoHost(t *testing.T) {
	conn, ver, up, err := parseFlags([]string{})
	if err != nil || conn != nil || ver || up {
		t.Fatalf("conn=%v ver=%v up=%v err=%v", conn, ver, up, err)
	}
}

func TestParseFlags_Host(t *testing.T) {
	conn, ver, up, err := parseFlags([]string{"--host", "localhost", "--port", "5433", "--user", "u", "--database", "db"})
	if err != nil || ver || up {
		t.Fatal(err)
	}
	if conn == nil || conn.Host != "localhost" || conn.Port != 5433 || conn.Database != "db" {
		t.Fatalf("%+v", conn)
	}
}

func TestParseFlags_Version(t *testing.T) {
	_, ver, _, err := parseFlags([]string{"--version"})
	if err != nil || !ver {
		t.Fatal(err)
	}
}

func TestParsePort(t *testing.T) {
	if parsePort("5432") != 5432 {
		t.Fatal()
	}
	if parsePort("x") != 5432 {
		t.Fatal()
	}
}

func TestIsSemver(t *testing.T) {
	if !isSemver("v1.2.3") || !isSemver("1.2.3") {
		t.Fatal("expected release tags to match")
	}
	if isSemver("dev") || isSemver("v1.2.3-2-gabc") || isSemver("v1.2.3-dirty") {
		t.Fatal("expected non-release forms to reject")
	}
}

func TestArchiveName(t *testing.T) {
	if archiveName("1.0.0", "darwin", "arm64") != "postgres-tui_1.0.0_Darwin_arm64.tar.gz" {
		t.Fatal(archiveName("1.0.0", "darwin", "arm64"))
	}
	if archiveName("1.0.0", "linux", "amd64") != "postgres-tui_1.0.0_Linux_x86_64.tar.gz" {
		t.Fatal(archiveName("1.0.0", "linux", "amd64"))
	}
}

// prevent unused import in case flag only used in parseFlags
var _ = flag.ErrHelp
