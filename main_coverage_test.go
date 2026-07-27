package main

import (
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/ui"
)

type fakeProgram struct {
	err error
}

func (f *fakeProgram) Send(msg tea.Msg) {}
func (f *fakeProgram) Run() (tea.Model, error) {
	return nil, f.err
}

func TestProdLogFatalAndRunApp(t *testing.T) {
	old := logFatalf
	called := false
	logFatalf = func(v ...any) { called = true }
	t.Cleanup(func() { logFatalf = old })
	prodLogFatal("x")
	if !called {
		t.Fatal("prodLogFatal")
	}

	// cover defaultLogFatalf without process exit
	oldExit := osExit
	osExit = func(int) {}
	t.Cleanup(func() { osExit = oldExit })
	defaultLogFatalf("x")

	// cover defaultNewProgram construction
	_ = defaultNewProgram(ui.NewModel())

	oldNP := newProgram
	newProgram = func(m ui.Model) teaProgram {
		return &fakeProgram{}
	}
	t.Cleanup(func() { newProgram = oldNP })

	m := ui.NewModel()
	send := func(tea.Msg) {}
	m.SendFunc = &send
	if err := prodRunApp(m); err != nil {
		t.Fatal(err)
	}

	newProgram = func(m ui.Model) teaProgram {
		return &fakeProgram{err: errors.New("run")}
	}
	if err := prodRunApp(m); err == nil {
		t.Fatal("expected error")
	}

	m.SendFunc = nil
	newProgram = func(m ui.Model) teaProgram { return &fakeProgram{} }
	if err := prodRunApp(m); err != nil {
		t.Fatal(err)
	}
}

func TestMainFunction(t *testing.T) {
	oldRun := runApp
	oldFatal := logFatal
	t.Cleanup(func() {
		runApp = oldRun
		logFatal = oldFatal
	})

	home := t.TempDir()
	t.Setenv("HOME", home)

	oldArgs := os.Args
	os.Args = []string{"postgres-tui"}
	t.Cleanup(func() { os.Args = oldArgs })

	ran := false
	runApp = func(m ui.Model) error {
		ran = true
		return nil
	}
	logFatal = func(v ...any) { t.Fatalf("unexpected fatal: %v", v) }
	main()
	if !ran {
		t.Fatal("runApp not called")
	}

	runApp = func(m ui.Model) error { return errors.New("boom") }
	fatalCalled := false
	logFatal = func(v ...any) { fatalCalled = true }
	main()
	if !fatalCalled {
		t.Fatal("expected fatal on runApp err")
	}

	// setup error path
	blocker := filepath.Join(t.TempDir(), "notdir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", blocker)
	ran = false
	fatalCalled = false
	runApp = func(m ui.Model) error { ran = true; return nil }
	logFatal = func(v ...any) { fatalCalled = true }
	main()
	if ran || !fatalCalled {
		t.Fatal("expected setup fatal")
	}
	t.Setenv("HOME", home)
}

func TestSetupAndInitConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	oldArgs := os.Args
	os.Args = []string{"postgres-tui"}
	t.Cleanup(func() { os.Args = oldArgs })

	m, err := setup()
	if err != nil {
		t.Fatal(err)
	}
	if m.Cmds == nil || m.Logs == nil {
		t.Fatal("incomplete model")
	}

	// CLI connection path
	os.Args = []string{"postgres-tui", "--host", "db.example", "--port", "5433"}
	m, err = setup()
	if err != nil {
		t.Fatal(err)
	}
	if m.CLIConnection == nil || m.CLIConnection.Host != "db.example" {
		t.Fatalf("%+v", m.CLIConnection)
	}

	// EnsureDemoConnections save fail (config dir not writable)
	locked := t.TempDir()
	cfgDir := filepath.Join(locked, ".config", "postgres-tui")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cfgDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cfgDir, 0o755) })
	t.Setenv("HOME", locked)
	os.Args = []string{"postgres-tui"}
	if _, err := setup(); err == nil {
		t.Fatal("expected ensure demo fail")
	}
	t.Setenv("HOME", home)

	// initConfig with UserHomeDir error falls back to TempDir
	oldHome := userHomeDir
	userHomeDir = func() (string, error) { return "", errors.New("no home") }
	t.Cleanup(func() { userHomeDir = oldHome })
	cfg, err := initConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("nil cfg")
	}
}

func TestParseCLIFlags(t *testing.T) {
	oldArgs := os.Args
	oldExit := osExit
	var exitCode int
	osExit = func(code int) { exitCode = code }
	t.Cleanup(func() {
		os.Args = oldArgs
		osExit = oldExit
	})

	os.Args = []string{"postgres-tui", "--host", "localhost", "--port", "5433", "--user", "u", "--database", "db", "--name", "n", "--sslmode", "disable", "--read-only", "--password", "secret"}
	conn := parseCLIFlags()
	if conn == nil || conn.Host != "localhost" || conn.Port != 5433 || !conn.ReadOnly || conn.Name != "n" {
		t.Fatalf("%+v", conn)
	}

	// default name from host:port when name empty; default user postgres
	os.Args = []string{"postgres-tui", "--host", "h"}
	conn = parseCLIFlags()
	if conn == nil || conn.Name != "h:5432" || conn.Username != "postgres" {
		t.Fatalf("%+v", conn)
	}

	// version flag
	exitCode = -1
	os.Args = []string{"postgres-tui", "--version"}
	if parseCLIFlags() != nil || exitCode != 0 {
		t.Fatalf("version exit=%d", exitCode)
	}

	// help
	exitCode = -1
	os.Args = []string{"postgres-tui", "-h"}
	// -h is host shorthand empty — actually -h alone may be host. Use --help
	os.Args = []string{"postgres-tui", "--help"}
	_ = parseCLIFlags()
	if exitCode != 0 {
		// flag.ErrHelp triggers osExit(0)
		t.Fatalf("help exit=%d", exitCode)
	}

	// bad flag
	exitCode = -1
	os.Args = []string{"postgres-tui", "--not-a-flag"}
	_ = parseCLIFlags()
	if exitCode != 2 {
		t.Fatalf("bad flag exit=%d", exitCode)
	}

	// update path with dev version fails then exits 1
	exitCode = -1
	os.Args = []string{"postgres-tui", "--update"}
	_ = parseCLIFlags()
	if exitCode != 1 {
		t.Fatalf("update exit=%d", exitCode)
	}

	// successful update (already up to date)
	oldVer := version
	oldExec := osExecutable
	oldClient := httpClient
	t.Cleanup(func() {
		version = oldVer
		osExecutable = oldExec
		httpClient = oldClient
	})
	version = "1.0.0"
	dir := t.TempDir()
	bin := filepath.Join(dir, "postgres-tui")
	_ = os.WriteFile(bin, []byte("x"), 0o755)
	osExecutable = func() (string, error) { return bin, nil }
	httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonOK(`{"tag_name":"v1.0.0"}`), nil
	})}
	exitCode = -1
	os.Args = []string{"postgres-tui", "--update"}
	_ = parseCLIFlags()
	if exitCode != 0 {
		t.Fatalf("update success exit=%d", exitCode)
	}
}

func TestParseFlagsPasswordWarning(t *testing.T) {
	conn, ver, up, err := parseFlags([]string{"--host", "h", "--password", "s", "-a", "also"})
	if err != nil || ver || up || conn == nil {
		t.Fatal(err)
	}
	// cover shorthand host/port/user/database
	conn, _, _, err = parseFlags([]string{"-h", "host2", "-p", "1", "-U", "me", "-d", "db"})
	if err != nil || conn.Host != "host2" || conn.Port != 1 || conn.Username != "me" || conn.Database != "db" {
		t.Fatalf("%+v %v", conn, err)
	}
	// ErrHelp
	_, _, _, err = parseFlags([]string{"--help"})
	if err != flag.ErrHelp {
		t.Fatalf("%v", err)
	}
}

func TestInitConfigMkdirFail(t *testing.T) {
	// Point home at a file so MkdirAll fails.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "notdir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := userHomeDir
	userHomeDir = func() (string, error) { return blocker, nil }
	t.Cleanup(func() { userHomeDir = old })
	if _, err := initConfig(); err == nil {
		t.Fatal("expected mkdir fail")
	}
}
