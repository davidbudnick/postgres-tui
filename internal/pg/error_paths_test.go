package pg

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/davidbudnick/postgres-tui/internal/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type cancelAfterNTracer struct {
	n     int64
	count atomic.Int64
}

func (t *cancelAfterNTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	if t.count.Add(1) > t.n {
		c, cancel := context.WithCancel(ctx)
		cancel()
		return c
	}
	return ctx
}

func (t *cancelAfterNTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func withPoolConfig(t *testing.T, fn func(*pgxpool.Config)) {
	t.Helper()
	prev := testConfigurePool
	testConfigurePool = fn
	t.Cleanup(func() { testConfigurePool = prev })
}

func connectWithConfig(t *testing.T, fn func(*pgxpool.Config)) *Client {
	t.Helper()
	withPoolConfig(t, fn)
	c := NewClient()
	if err := c.Connect(demoConn()); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { _ = c.Disconnect() })
	return c
}

func TestConnectNewPoolError(t *testing.T) {
	withPoolConfig(t, func(cfg *pgxpool.Config) {
		cfg.MaxConns = 0
	})
	c := NewClient()
	if err := c.Connect(demoConn()); err == nil {
		t.Fatal("expected NewWithConfig error")
	}
}

func TestCancelTracerQueryErrors(t *testing.T) {
	c := connectWithConfig(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = &cancelAfterNTracer{n: 0}
	})

	if _, err := c.ListDatabases(); err == nil {
		t.Fatal("ListDatabases")
	}
	if _, err := c.ListSchemas(); err == nil {
		t.Fatal("ListSchemas")
	}
	if _, err := c.ListObjects("public", types.ObjectTable); err == nil {
		t.Fatal("listRelations")
	}
	if _, err := c.ListObjects("public", types.ObjectSequence); err == nil {
		t.Fatal("listSequences")
	}
	if _, err := c.ListObjects("public", types.ObjectFunction); err == nil {
		t.Fatal("listFunctions")
	}
	if _, err := c.ListObjects("public", types.ObjectType); err == nil {
		t.Fatal("listTypes")
	}
	if _, err := c.ListObjects("public", types.ObjectExtension); err == nil {
		t.Fatal("listExtensionsAsObjects")
	}
	if _, err := c.ListActivity(); err == nil {
		t.Fatal("ListActivity")
	}
	if _, err := c.ListExtensions(); err == nil {
		t.Fatal("ListExtensions")
	}
	if _, err := c.ListERD("public"); err == nil {
		t.Fatal("ListERD tables")
	}
	if _, err := c.RunQuery("SELECT 1", 1); err == nil {
		t.Fatal("RunQuery select")
	}
	if _, err := c.RunQuery("SET application_name = 'x'", 1); err == nil {
		t.Fatal("RunQuery exec")
	}
}

func TestGetTableDetailPartialQueryFailures(t *testing.T) {
	c := connectWithConfig(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = &cancelAfterNTracer{n: 1}
	})
	if _, err := c.GetTableDetail("public", "users"); err == nil {
		t.Fatal("columns query")
	}

	c2 := connectWithConfig(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = &cancelAfterNTracer{n: 2}
	})
	if _, err := c2.GetTableDetail("public", "users"); err == nil {
		t.Fatal("indexes query")
	}

	c3 := connectWithConfig(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = &cancelAfterNTracer{n: 3}
	})
	if _, err := c3.GetTableDetail("public", "users"); err == nil {
		t.Fatal("constraints query")
	}
}

func TestListERDEdgeQueryFailure(t *testing.T) {
	c := connectWithConfig(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = &cancelAfterNTracer{n: 1}
	})
	if _, err := c.ListERD("public"); err == nil {
		t.Fatal("erd edges")
	}
}

func TestForcedScanAndValuesErrors(t *testing.T) {
	prevScan, prevVals, prevErr := scanRow, rowValues, rowsErr
	t.Cleanup(func() {
		scanRow, rowValues, rowsErr = prevScan, prevVals, prevErr
	})

	scanRow = func(s scannable, dest ...any) error {
		return errors.New("forced scan failure")
	}
	rowValues = func(r interface{ Values() ([]any, error) }) ([]any, error) {
		return nil, errors.New("forced values failure")
	}
	rowsErr = func(r interface{ Err() error }) error {
		return errors.New("forced rows err")
	}

	c := liveClient(t)
	ensureDemoObjects(t, c)

	if _, err := c.ListDatabases(); err == nil {
		t.Fatal("ListDatabases scan")
	}
	if _, err := c.ListSchemas(); err == nil {
		t.Fatal("ListSchemas scan")
	}
	if _, err := c.ListObjects("public", types.ObjectTable); err == nil {
		t.Fatal("listRelations scan")
	}
	if _, err := c.ListObjects("public", types.ObjectSequence); err == nil {
		t.Fatal("listSequences scan")
	}
	if _, err := c.ListObjects("public", types.ObjectFunction); err == nil {
		t.Fatal("listFunctions scan")
	}
	if _, err := c.ListObjects("public", types.ObjectType); err == nil {
		t.Fatal("listTypes scan")
	}
	if _, err := c.ListObjects("public", types.ObjectExtension); err == nil {
		t.Fatal("listExtensionsAsObjects scan")
	}
	if _, err := c.ListExtensions(); err == nil {
		t.Fatal("ListExtensions scan")
	}
	if _, err := c.ListActivity(); err == nil {
		t.Fatal("ListActivity scan")
	}
	if _, err := c.ListERD("public"); err == nil {
		t.Fatal("ListERD scan")
	}
	if _, err := c.GetTableDetail("public", "users"); err == nil {
		t.Fatal("GetTableDetail scan")
	}
	_, _ = c.GetObjectDetail("public", "address", types.ObjectType)
	if _, err := c.RunQuery("SELECT 1 AS n", 10); err == nil {
		t.Fatal("RunQuery Values")
	}
}

func TestForcedRowsErrAfterScanOK(t *testing.T) {
	prevScan, prevErr := scanRow, rowsErr
	t.Cleanup(func() {
		scanRow, rowsErr = prevScan, prevErr
	})

	scanRow = func(s scannable, dest ...any) error { return s.Scan(dest...) }
	rowsErr = func(r interface{ Err() error }) error { return errors.New("forced rows err") }

	c := liveClient(t)
	ensureDemoObjects(t, c)

	if _, err := c.ListDatabases(); err == nil {
		t.Fatal("ListDatabases rows.Err")
	}
	if _, err := c.ListSchemas(); err == nil {
		t.Fatal("ListSchemas rows.Err")
	}
	if _, err := c.ListObjects("public", types.ObjectTable); err == nil {
		t.Fatal("listRelations rows.Err")
	}
	if _, err := c.ListObjects("public", types.ObjectSequence); err == nil {
		t.Fatal("listSequences rows.Err")
	}
	if _, err := c.ListObjects("public", types.ObjectFunction); err == nil {
		t.Fatal("listFunctions rows.Err")
	}
	if _, err := c.ListObjects("public", types.ObjectType); err == nil {
		t.Fatal("listTypes rows.Err")
	}
	if _, err := c.ListObjects("public", types.ObjectExtension); err == nil {
		t.Fatal("listExtensionsAsObjects rows.Err")
	}
	if _, err := c.ListExtensions(); err == nil {
		t.Fatal("ListExtensions rows.Err")
	}
	if _, err := c.ListActivity(); err == nil {
		t.Fatal("ListActivity rows.Err")
	}
	if _, err := c.GetTableDetail("public", "users"); err == nil {
		t.Fatal("GetTableDetail colRows.Err")
	}
	if _, err := c.ListERD("public"); err == nil {
		t.Fatal("ListERD tblRows.Err")
	}
	if _, err := c.RunQuery("SELECT 1", 10); err == nil {
		t.Fatal("RunQuery rows.Err")
	}
}

func TestGetTableDetailIndexAndConstraintScanErr(t *testing.T) {
	prev := scanRow
	t.Cleanup(func() { scanRow = prev })

	var n atomic.Int64
	scanRow = func(s scannable, dest ...any) error {
		switch n.Add(1) {
		case 1:
			return s.Scan(dest...)
		case 2:
			return errors.New("index scan fail")
		default:
			return errors.New("constraint scan fail")
		}
	}

	c := liveClient(t)
	if _, err := c.GetTableDetail("public", "order_items"); err == nil {
		t.Fatal("expected index scan error")
	}

	n.Store(0)
	scanRow = func(s scannable, dest ...any) error {
		switch n.Add(1) {
		case 1, 2:
			return s.Scan(dest...)
		default:
			return errors.New("constraint scan fail")
		}
	}
	n.Store(0)
	var colScans, idxScans atomic.Int64
	scanRow = func(s scannable, dest ...any) error {
		switch len(dest) {
		case 8:
			colScans.Add(1)
			return s.Scan(dest...)
		case 5:
			if idxScans.Add(1) == 1 {
				return errors.New("index scan fail")
			}
			return s.Scan(dest...)
		default:
			return s.Scan(dest...)
		}
	}
	if _, err := c.GetTableDetail("public", "order_items"); err == nil {
		t.Fatal("expected index scan error")
	}

	scanRow = func(s scannable, dest ...any) error {
		switch len(dest) {
		case 8, 5:
			return s.Scan(dest...)
		case 3:
			return errors.New("constraint scan fail")
		default:
			return s.Scan(dest...)
		}
	}
	if _, err := c.GetTableDetail("public", "order_items"); err == nil {
		t.Fatal("expected constraint scan error")
	}
}

func TestGetTableDetailRowsErrPhases(t *testing.T) {
	prevScan, prevErr := scanRow, rowsErr
	t.Cleanup(func() {
		scanRow, rowsErr = prevScan, prevErr
	})

	scanRow = func(s scannable, dest ...any) error { return s.Scan(dest...) }

	phase := atomic.Int64{}
	rowsErr = func(r interface{ Err() error }) error {
		if phase.Add(1) == 1 {
			return errors.New("col err")
		}
		return r.Err()
	}
	c := liveClient(t)
	if _, err := c.GetTableDetail("public", "users"); err == nil {
		t.Fatal("colRows.Err")
	}

	phase.Store(0)
	rowsErr = func(r interface{ Err() error }) error {
		if phase.Add(1) == 2 {
			return errors.New("idx err")
		}
		return nil
	}
	if _, err := c.GetTableDetail("public", "users"); err == nil {
		t.Fatal("idxRows.Err")
	}

	phase.Store(0)
	rowsErr = func(r interface{ Err() error }) error {
		if phase.Add(1) == 3 {
			return errors.New("con err")
		}
		return nil
	}
	if _, err := c.GetTableDetail("public", "users"); err == nil {
		t.Fatal("conRows.Err")
	}
}

func TestListERDEdgeScanAndErr(t *testing.T) {
	prevScan, prevErr := scanRow, rowsErr
	t.Cleanup(func() {
		scanRow, rowsErr = prevScan, prevErr
	})

	c := liveClient(t)

	scanRow = func(s scannable, dest ...any) error {
		if len(dest) == 5 {
			return errors.New("edge scan fail")
		}
		return s.Scan(dest...)
	}
	rowsErr = func(r interface{ Err() error }) error { return r.Err() }
	if _, err := c.ListERD("public"); err == nil {
		t.Fatal("edge scan")
	}

	scanRow = func(s scannable, dest ...any) error { return s.Scan(dest...) }
	var errCalls atomic.Int64
	rowsErr = func(r interface{ Err() error }) error {
		if errCalls.Add(1) == 2 {
			return errors.New("edge rows err")
		}
		return nil
	}
	if _, err := c.ListERD("public"); err == nil {
		t.Fatal("edge rows.Err")
	}
}
