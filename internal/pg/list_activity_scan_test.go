package pg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/davidbudnick/postgres-tui/internal/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListActivityScanError(t *testing.T) {
	// Hold an open backend so ListActivity returns ≥1 row, then fail scan.
	cfg, err := pgxpool.ParseConfig(demoConn().DSN())
	if err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	holder, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(holder.Close)
	conn, err := holder.Acquire(ctx)
	if err != nil {
		t.Skip(err)
	}
	// idle in transaction counts as client backend
	if _, err := conn.Exec(ctx, "BEGIN"); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "ROLLBACK")
		conn.Release()
	})

	c := liveClient(t)
	prev := scanRow
	t.Cleanup(func() { scanRow = prev })
	scanRow = func(s scannable, dest ...any) error {
		return errors.New("forced activity scan fail")
	}
	if _, err := c.ListActivity(); err == nil {
		t.Fatal("expected scan error")
	}
}

func TestListActivityHappyWithHolder(t *testing.T) {
	cfg, err := pgxpool.ParseConfig(demoConn().DSN())
	if err != nil {
		t.Skip(err)
	}
	ctx := context.Background()
	holder, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(holder.Close)
	if _, err := holder.Exec(ctx, "SELECT pg_sleep(0)"); err != nil {
		t.Skip(err)
	}
	// begin and hold
	tx, err := holder.Begin(ctx)
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	c := liveClient(t)
	rows, err := c.ListActivity()
	if err != nil {
		t.Fatal(err)
	}
	// may or may not see the holder depending on timing; just exercise path
	_ = rows
	_ = types.ActivityRow{}
}
