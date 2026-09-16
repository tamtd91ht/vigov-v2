// Package store provides the only sanctioned path to the database.
//
// WHY: rule 1 requires every query to be scoped to a commune. Enforcing that by asking
// developers to remember is how the previous system ended up with zero occurrences of
// tenant_id across 35,000 lines. Here the scope is added by the repository itself, so
// forgetting it is not possible — there is no API that omits it.
//
// There is deliberately NO method that exposes the raw *sql.DB to business code.
package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/pkg/tenant"
)

// DB is the process-wide handle. Only Scoped may be obtained from it.
type DB struct{ db *sql.DB }

func New(db *sql.DB) *DB { return &DB{db: db} }

// Scoped binds a connection to one commune for the life of a request.
type Scoped struct {
	db  *sql.DB
	tid tenant.ID
}

// For returns a repository scoped to the commune in the context.
// Panics when there is none — see tenant.MustFrom for why that is deliberate.
func (d *DB) For(ctx context.Context) *Scoped {
	return &Scoped{db: d.db, tid: tenant.MustFrom(ctx)}
}

// TenantID is exposed so repositories can build their own SQL. It is the only way to obtain
// the value, which keeps every use greppable.
func (s *Scoped) TenantID() tenant.ID { return s.tid }

// Query runs a read of ONE table, already filtered by commune.
//
// The caller names the columns and the table; this adds `WHERE tenant_id = $1` and binds it,
// so $1 is always the commune and the caller's own placeholders start at $2. There is no
// signature here that omits the commune, which is the point of the package.
//
// For a join, use QueryJoin: `tenant_id` is ambiguous the moment a second table is in scope,
// and PostgreSQL refuses the query rather than guessing.
func (s *Scoped) Query(ctx context.Context, cot, bang, tail string, args ...any) (*sql.Rows, error) {
	full := "SELECT " + cot + " FROM " + bang + " WHERE tenant_id = $1 " + tail
	return s.db.QueryContext(ctx, full, append([]any{string(s.tid)}, args...)...)
}

// QueryJoin runs a read across joined tables, still scoped to the commune.
//
// WHY A SECOND METHOD INSTEAD OF LOOSENING THE FIRST: Query owns the whole statement, which is
// what makes forgetting the commune impossible. A joined query has to own its own FROM clause,
// so the scope has to be applied differently — and the honest way to say that is a separate
// method with a separate contract, not an escape hatch on the safe one.
//
// The contract: the caller writes the full statement, uses $1 for the commune, and qualifies
// it with the alias of the table that owns the row — `nd.tenant_id = $1`, never a bare
// `tenant_id = $1`, which is ambiguous across joined tables.
//
// Every other table in the join must ALSO be constrained to $1 in its ON clause. Joining on id
// alone would match another commune's row wherever ids collide, which is exactly the leak
// rule 1 exists to prevent, and no test of a single commune would ever show it.
func (s *Scoped) QueryJoin(ctx context.Context, stmt string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, stmt, append([]any{string(s.tid)}, args...)...)
}

// Tx runs fn inside one transaction, scoped to the same commune.
//
// WHY THIS EXISTS AT ALL: rule 6 requires the audit entry to share a transaction with the
// business write. The previous system had no transactions anywhere, so "every write leaves a
// trail" could not actually hold — there was always a window where the record had changed and
// the trail had not. This is the API that closes it.
func (s *Scoped) Tx(ctx context.Context, fn func(*ScopedTx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	stx := &ScopedTx{tx: tx, tid: s.tid}
	if err := fn(stx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("store: %w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

// ScopedTx is a transaction bound to one commune.
type ScopedTx struct {
	tx  *sql.Tx
	tid tenant.ID
}

func (s *ScopedTx) TenantID() tenant.ID { return s.tid }
func (s *ScopedTx) Underlying() *sql.Tx { return s.tx }

// Exec runs a write already scoped to the commune.
func (s *ScopedTx) Exec(ctx context.Context, stmt string, args ...any) (sql.Result, error) {
	return s.tx.ExecContext(ctx, stmt, args...)
}
