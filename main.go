package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/davidbudnick/postgres-tui/internal/cmd"
	"github.com/davidbudnick/postgres-tui/internal/db"
	"github.com/davidbudnick/postgres-tui/internal/pg"
	"github.com/davidbudnick/postgres-tui/internal/service"
	"github.com/davidbudnick/postgres-tui/internal/types"
	"github.com/davidbudnick/postgres-tui/internal/ui"

	tea "charm.land/bubbletea/v2"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var (
	osExit   = os.Exit
	logFatal = prodLogFatal
	runApp   = prodRunApp
)

func defaultLogFatalf(v ...any) {
	log.Println(v...)
	osExit(1)
}

var logFatalf = defaultLogFatalf

func prodLogFatal(v ...any) { logFatalf(v...) }

func defaultNewProgram(m ui.Model) teaProgram {
	return tea.NewProgram(m)
}

var newProgram = defaultNewProgram

type teaProgram interface {
	Send(msg tea.Msg)
	Run() (tea.Model, error)
}

func prodRunApp(m ui.Model) error {
	p := newProgram(m)
	if m.SendFunc != nil {
		*m.SendFunc = p.Send
	}
	_, err := p.Run()
	return err
}

func main() {
	m, err := setup()
	if err != nil {
		logFatal(err)
		return
	}
	if err := runApp(m); err != nil {
		logFatal(err)
	}
}

func setup() (ui.Model, error) {
	opts := parseCLIFlags()

	logWriter := types.NewLogWriter()

	m := ui.NewModel()
	m.Logs = logWriter
	m.Version = version

	if opts != nil {
		m.CLIConnection = opts
	}

	sendFunc := func(msg tea.Msg) {}
	m.SendFunc = &sendFunc

	handler := slog.NewJSONHandler(logWriter, nil)
	slog.SetDefault(slog.New(handler))

	config, err := initConfig()
	if err != nil {
		return m, fmt.Errorf("failed to initialize config: %w", err)
	}

	pgClient := pg.NewClient()
	container := &service.Container{Config: config, PG: pgClient}
	m.Cmds = cmd.NewCommandsFromContainer(container)
	m.PageSize = config.GetPageSize()

	return m, nil
}

func parseCLIFlags() *types.Connection {
	conn, showVersion, doUpdate, err := parseFlags(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			osExit(0)
			return nil
		}
		osExit(2)
		return nil
	}
	if showVersion {
		fmt.Printf("postgres-tui %s (commit: %s, built: %s)\n", version, commit, date)
		osExit(0)
		return nil
	}
	if doUpdate {
		if err := runUpdate(version); err != nil {
			fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
			osExit(1)
			return nil
		}
		osExit(0)
		return nil
	}
	return conn
}

func parseFlags(args []string) (conn *types.Connection, showVersion bool, doUpdate bool, err error) {
	fs := flag.NewFlagSet("postgres-tui", flag.ContinueOnError)

	host := fs.String("host", "", "PostgreSQL hostname (required for quick-connect)")
	port := fs.Int("port", 5432, "Server port")
	username := fs.String("user", "", "Username")
	password := fs.String("password", "", "Password")
	database := fs.String("database", "postgres", "Database name")
	name := fs.String("name", "", "Connection display name")
	sslMode := fs.String("sslmode", "prefer", "SSL mode: disable|allow|prefer|require|verify-ca|verify-full")
	readOnly := fs.Bool("read-only", false, "Read-only mode (block mutations)")
	versionFlag := fs.Bool("version", false, "Print version and exit")
	update := fs.Bool("update", false, "Update to the latest version")

	fs.StringVar(host, "h", "", "Hostname (shorthand)")
	fs.IntVar(port, "p", 5432, "Port (shorthand)")
	fs.StringVar(password, "a", "", "Password (shorthand)")
	fs.StringVar(database, "d", "postgres", "Database (shorthand)")
	fs.StringVar(username, "U", "", "Username (shorthand)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: postgres-tui [flags]\n\n")
		fmt.Fprintf(os.Stderr, "A terminal UI for PostgreSQL.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fmt.Fprintf(os.Stderr, "  -h, --host string       Server hostname (required for quick-connect)\n")
		fmt.Fprintf(os.Stderr, "  -p, --port int          Server port (default 5432)\n")
		fmt.Fprintf(os.Stderr, "  -U, --user string       Username\n")
		fmt.Fprintf(os.Stderr, "  -a, --password string   Password\n")
		fmt.Fprintf(os.Stderr, "  -d, --database string   Database name (default postgres)\n")
		fmt.Fprintf(os.Stderr, "      --name string       Connection display name\n")
		fmt.Fprintf(os.Stderr, "      --sslmode string    SSL mode (default prefer)\n")
		fmt.Fprintf(os.Stderr, "      --read-only         Block write statements\n")
		fmt.Fprintf(os.Stderr, "      --version           Print version and exit\n")
		fmt.Fprintf(os.Stderr, "      --update            Update to the latest version\n")
	}

	if err := fs.Parse(args); err != nil {
		return nil, false, false, err
	}

	if *versionFlag {
		return nil, true, false, nil
	}
	if *update {
		return nil, false, true, nil
	}
	if *host == "" {
		return nil, false, false, nil
	}

	fs.Visit(func(f *flag.Flag) {
		if f.Name == "password" || f.Name == "a" {
			fmt.Fprintln(os.Stderr, "Warning: Passing secrets on the command line exposes them in the process list. Prefer the interactive connection form.")
		}
	})

	conn = &types.Connection{
		Host:     *host,
		Port:     *port,
		Username: *username,
		Password: *password,
		Database: *database,
		SSLMode:  types.SSLMode(*sslMode),
		ReadOnly: *readOnly,
	}
	if *name != "" {
		conn.Name = *name
	} else {
		conn.Name = fmt.Sprintf("%s:%d", *host, *port)
	}
	if conn.Username == "" {
		conn.Username = "postgres"
	}
	return conn, false, false, nil
}

var userHomeDir = os.UserHomeDir

func initConfig() (*db.Config, error) {
	homeDir, err := userHomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}

	configDir := filepath.Join(homeDir, ".config", "postgres-tui")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, "config.json")
	return db.NewConfig(configPath)
}

// parsePort is a small helper used by tests.
func parsePort(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 5432
	}
	return n
}
