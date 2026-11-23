package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Queryer is the common surface of *sqlx.DB and *sqlx.Tx that repositories
// depend on. A repository accepts this interface so the same code runs against
// the pool (reads) or inside a transaction (writes) without duplication.
type Queryer interface {
	sqlx.ExtContext
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	NamedExecContext(ctx context.Context, query string, arg any) (sql.Result, error)
	PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)
}

// Ensure both concrete types satisfy the interface at compile time.
var (
	_ Queryer = (*sqlx.DB)(nil)
	_ Queryer = (*sqlx.Tx)(nil)
)

// A transaction can ride in a context, so code reached through a plugin hook —
// written on its own *sqlx.DB — joins its caller's transaction instead of
// committing on its own: a caller that must be all-or-nothing across several
// plugins (the AI Assistant applying a job through others' hooks) puts its tx in
// the context with InTx; stores read and write through On(ctx, db) and open
// transactions with WithTx, which then joins. Nothing else changes: a context
// without a tx behaves exactly as before.
type txKey struct{}

// InTx returns ctx carrying tx.
func InTx(ctx context.Context, tx *sqlx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TxFrom is the transaction ctx carries, nil when none.
func TxFrom(ctx context.Context) *sqlx.Tx {
	tx, _ := ctx.Value(txKey{}).(*sqlx.Tx)
	return tx
}

// On is the transaction ctx carries, else db.
func On(ctx context.Context, db *sqlx.DB) Queryer {
	if tx := TxFrom(ctx); tx != nil {
		return tx
	}
	return db
}

// WithTx runs fn inside a transaction, rolling back on error or panic and
// committing otherwise. When ctx carries a transaction (InTx), fn joins it inside
// a savepoint instead: an error rolls back fn's part only, and the caller's
// transaction commits or not.
func WithTx(ctx context.Context, db *sqlx.DB, fn func(tx *sqlx.Tx) error) error {
	if outer := TxFrom(ctx); outer != nil {
		return joined(ctx, outer, fn)
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// undoSavepoint rolls back to the newest ks_joined and releases it. PostgreSQL keeps a
// savepoint after ROLLBACK TO, and with two of one name the newest answers: without the
// release, an outer savepoint's rollback would only reach this inner one and keep what
// the outer did before it (IMP-3: a sheet line's new category outliving the line).
func undoSavepoint(ctx context.Context, tx *sqlx.Tx) error {
	if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT ks_joined`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `RELEASE SAVEPOINT ks_joined`)
	return err
}

func joined(ctx context.Context, tx *sqlx.Tx, fn func(tx *sqlx.Tx) error) (err error) {
	if _, err := tx.ExecContext(ctx, `SAVEPOINT ks_joined`); err != nil {
		return fmt.Errorf("savepoint: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = undoSavepoint(ctx, tx)
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		if rb := undoSavepoint(ctx, tx); rb != nil {
			return fmt.Errorf("%w (and its rollback: %v)", err, rb)
		}
		return err
	}
	if _, err = tx.ExecContext(ctx, `RELEASE SAVEPOINT ks_joined`); err != nil {
		// fn swallowed a failed statement (the transaction is aborted): undo this
		// savepoint so the caller's transaction can go on, and report the failure.
		if rb := undoSavepoint(ctx, tx); rb != nil {
			return fmt.Errorf("%w (and its rollback: %v)", err, rb)
		}
	}
	return err
}

// Tx is a transaction from Begin: its own, or a savepoint inside the one ctx
// carries. Commit and Rollback act on whichever it is; Rollback after Commit is a
// no-op, so `defer tx.Rollback()` keeps working.
type Tx struct {
	*sqlx.Tx
	ctx          context.Context
	joined, done bool
}

// Begin opens a transaction on db — or, when ctx carries one (InTx), a savepoint
// in it, so a store's own transaction joins its caller's.
func Begin(ctx context.Context, db *sqlx.DB) (*Tx, error) {
	if outer := TxFrom(ctx); outer != nil {
		if _, err := outer.ExecContext(ctx, `SAVEPOINT ks_joined`); err != nil {
			return nil, fmt.Errorf("savepoint: %w", err)
		}
		return &Tx{Tx: outer, ctx: ctx, joined: true}, nil
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx, ctx: ctx}, nil
}

func (t *Tx) Commit() error {
	if !t.joined {
		return t.Tx.Commit()
	}
	if t.done {
		return sql.ErrTxDone
	}
	t.done = true
	_, err := t.Tx.ExecContext(t.ctx, `RELEASE SAVEPOINT ks_joined`)
	return err
}

func (t *Tx) Rollback() error {
	if !t.joined {
		return t.Tx.Rollback()
	}
	if t.done {
		return sql.ErrTxDone
	}
	t.done = true
	return undoSavepoint(t.ctx, t.Tx)
}
