package ltest

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ErrPlanted is the database failure a Fault plants.
var ErrPlanted = errors.New("ltest: planted database failure")

// Fault makes a database fail on purpose: Arm(k) fails the k-th statement from
// now (a query, an exec, a begin or a commit), once. Sweeping k = 1, 2, … over
// one action reaches every `if err != nil` it has, in order, and shows whether a
// failure part-way leaves anything half written.
type Fault struct {
	left   atomic.Int64 // statements until the failure; 0 = disarmed
	mu     sync.Mutex
	failed string // what failed last
}

// Arm fails the k-th statement from now; 0 disarms.
func (f *Fault) Arm(k int) { f.Failed(); f.left.Store(int64(k)) }

// Fired reports whether the armed failure happened (and disarms either way).
func (f *Fault) Fired() bool { return f.left.Swap(0) < 0 }

// Failed is the statement the last planted failure stopped.
func (f *Fault) Failed() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.failed
	f.failed = ""
	return s
}

func (f *Fault) hit(what string) error {
	if f.left.Load() <= 0 {
		return nil
	}
	if f.left.Add(-1) != 0 {
		return nil
	}
	f.left.Store(-1) // fired; stays fired until Fired / Arm
	f.mu.Lock()
	f.failed = what
	f.mu.Unlock()
	return ErrPlanted
}

// NewFaultyDB is NewDBWithDSN on a connection that fails when its Fault says.
// The database is the same throwaway copy; only this pool can be made to fail.
func NewFaultyDB(t T) (*sqlx.DB, string, *Fault) {
	helper(t)
	_, dsn := NewDBWithDSN(t)
	c, err := pq.NewConnector(dsn)
	if err != nil {
		t.Fatal(err)
	}
	f := &Fault{}
	db := sqlx.NewDb(sql.OpenDB(faultConnector{c, f}), "postgres").Unsafe() // as db.Connect
	db.SetMaxOpenConns(20)
	t.Cleanup(func() { db.Close() })
	return db, dsn, f
}

type faultConnector struct {
	driver.Connector
	f *Fault
}

func (c faultConnector) Connect(ctx context.Context) (driver.Conn, error) {
	cn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &faultConn{cn, c.f}, nil
}

// faultConn passes everything to pq's connection, failing a statement when armed.
type faultConn struct {
	driver.Conn
	f *Fault
}

func (c *faultConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	if err := c.f.hit(q); err != nil {
		return nil, err
	}
	if p, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return p.PrepareContext(ctx, q)
	}
	return c.Conn.Prepare(q)
}

func (c *faultConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	e, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip // database/sql prepares instead, which counts there
	}
	if err := c.f.hit(q); err != nil {
		return nil, err
	}
	return e.ExecContext(ctx, q, args)
}

func (c *faultConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	qc, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	if err := c.f.hit(q); err != nil {
		return nil, err
	}
	return qc.QueryContext(ctx, q, args)
}

func (c *faultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := c.f.hit("BEGIN"); err != nil {
		return nil, err
	}
	var tx driver.Tx
	var err error
	if b, ok := c.Conn.(driver.ConnBeginTx); ok {
		tx, err = b.BeginTx(ctx, opts)
	} else {
		tx, err = c.Conn.Begin() //nolint:staticcheck // pq always has BeginTx; kept for the interface
	}
	if err != nil {
		return nil, err
	}
	return faultTx{tx, c.f}, nil
}

func (c *faultConn) CheckNamedValue(nv *driver.NamedValue) error {
	if n, ok := c.Conn.(driver.NamedValueChecker); ok {
		return n.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

func (c *faultConn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *faultConn) ResetSession(ctx context.Context) error {
	if r, ok := c.Conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *faultConn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

// faultTx fails a commit when armed: the transaction is rolled back instead, as
// a real failed commit leaves it.
type faultTx struct {
	driver.Tx
	f *Fault
}

func (t faultTx) Commit() error {
	if err := t.f.hit("COMMIT"); err != nil {
		t.Tx.Rollback() //nolint:errcheck
		return err
	}
	return t.Tx.Commit()
}
