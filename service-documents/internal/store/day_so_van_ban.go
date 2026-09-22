package store

// THE NUMBER SERIES OF THE TWO REGISTERS. This is the most expensive file in the service, and the
// whole of it is about one statement.
//
// Rule 7, invariant 3 and forbidden #4: a document number that has been issued is NEVER reissued
// and NEVER renumbered. An outgoing number is printed on paper, sealed, and sent to a district
// office, a court or a citizen; two documents sharing one number are two documents nobody outside
// the commune can tell apart, and one of them has been signed.
//
// THREE MECHANISMS HOLD THAT PROMISE, and each one alone still leaks:
//
//	UNIQUE (tenant_id, nam, so_*)     migration 0004, WITHOUT `WHERE deleted_at IS NULL`. Soft
//	                                  deleting number 7 does NOT free number 7.
//	the counter row                   a high-water mark, not `max(so) + 1`. A number computed from
//	                                  rows goes DOWN when a row is removed; a counter does not.
//	`FOR UPDATE` on that row          THIS FILE. Two clerks pressing "Thêm văn bản đi" in the same
//	                                  second must not both read 11 and both write 12.
//
// WHY THE ROW LOCK IS THE ONLY THING THAT CLOSES THE GAP, and why no check in Go can replace it:
// under READ COMMITTED the second transaction BLOCKS on the row the first one holds, and when the
// first commits PostgreSQL re-reads the row — so the second allocation sees 12 and returns 13. Take
// the lock away and both transactions read the same value from the same snapshot, both write the
// same number, and the UNIQUE key rejects the second INSERT. That failure is SAFE — no duplicate
// number can exist — but it reaches a commune as "the software is broken" at the moment a clerk is
// registering a document, and it is indistinguishable from a real outage on the screen.
//
// THE SAME SHAPE service-identity's `truyVanQuanTriDeGhi` USES, for the same reason stated there:
// the lock has to cover the row the ANSWER depends on, not merely the row being written.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
)

// SoSach names one of the two registers. A VALUE, not a table name: văn bản đến số 7 and văn bản đi
// số 7 are two different documents in the same commune in the same year, and both are correct.
type SoSach string

const (
	SoSachDen SoSach = "den"
	SoSachDi  SoSach = "di"
)

// TranSoMotNam is the ceiling on one commune's series in one year.
//
// WHY A CEILING AT ALL: a number is allocated by a row lock and nothing else bounds it, so a loop
// that books documents — an import run twice, a client retrying without an idempotency key — would
// walk the series into five digits before anybody noticed. At "vài chục văn bản đến mỗi tuần" a
// commune reaches a few thousand a year, so 99999 is roughly twenty times any real figure: it is
// not an estimate of how many there might be, it is the point past which the register is no longer
// a register.
//
// REFUSING IS THE SAFE DIRECTION HERE, unlike almost everywhere else in this repository: refusing
// stops one commune's booking with a sentence somebody acts on, while continuing hands out numbers
// that can never be taken back.
const TranSoMotNam = 99999

// ErrDayySoDayTran says the year's series has reached its ceiling. The caller answers 409.
var ErrDaySoDayTran = errors.New("day_so_van_ban: dãy số của năm đã đạt trần")

// DaySoStore allocates register numbers. Built from *store.DB, which only hands out commune-scoped
// access: there is no constructor taking a raw *sql.DB, because a repository that can be built
// without a commune is a repository that can number another commune's register (rule 1, invariant 5).
type DaySoStore struct {
	db *store.DB
}

func NewDaySoStore(db *store.DB) *DaySoStore { return &DaySoStore{db: db} }

// moDayNeuChua creates the counter row for this (commune, register, year) if it is not there yet.
//
// `ON CONFLICT DO NOTHING` AND NOT "read, then insert if missing": two clerks booking the first
// document of a new year would both find no row and both INSERT, and the second would fail the
// primary key — turning the first booking of every January into an error. This statement is safe to
// run concurrently and safe to run twice.
//
// IT DOES NOT ALLOCATE ANYTHING. `so_cuoi` starts at 0, so the first document still takes 1 through
// the locked read below. Splitting creation from allocation is what lets the allocation be a plain
// read-modify-write under a lock, with no branch for "the first one".
const moDayNeuChua = `INSERT INTO day_so_van_ban (tenant_id, so_sach, nam, so_cuoi)
	VALUES ($1, $2, $3, 0)
	ON CONFLICT (tenant_id, so_sach, nam) DO NOTHING`

// docDayDeCap reads the counter AND HOLDS IT until the transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS STATEMENT AND NOT AN OPTIMISATION. Remove it and this file's
// entire purpose is gone, silently: every test that books one document at a time still passes, and
// the defect only appears when two clerks act in the same second — which is a Monday morning in a
// commune office, not a rare event.
const docDayDeCap = `SELECT so_cuoi FROM day_so_van_ban
	WHERE tenant_id = $1 AND so_sach = $2 AND nam = $3
	FOR UPDATE`

// tangDay moves the high-water mark. `so_cuoi = $4` and not `so_cuoi + 1` on purpose: the caller has
// just read the value under the lock and the trigger `day_so_van_ban_khong_lui` refuses any write
// that would lower it, so an explicit value keeps the number that was allocated and the number that
// was stored provably identical — there is no path where the row ends up one ahead of the document.
const tangDay = `UPDATE day_so_van_ban SET so_cuoi = $4, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND so_sach = $2 AND nam = $3`

// CapSo issues the next number of one register in one year, inside the caller's transaction.
//
// IT TAKES THE TRANSACTION AND NEVER OPENS ONE. That is what makes the number and the document
// inseparable: the row lock is released when the caller's transaction ends, so a booking that fails
// afterwards rolls the counter back with it and the number is not burned. A PostgreSQL SEQUENCE
// could not do this — `nextval` survives a rollback, and a register with holes in it is a register
// an inspector asks about.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: it comes from tx.TenantID(), which took it from
// the context (rule 1, invariants 4 and 5).
func (s *DaySoStore) CapSo(ctx context.Context, tx *store.ScopedTx, so SoSach, nam int) (int, error) {
	xa := string(tx.TenantID())

	if _, err := tx.Exec(ctx, moDayNeuChua, xa, string(so), nam); err != nil {
		return 0, fmt.Errorf("day_so_van_ban: mở dãy số: %w", err)
	}

	var soCuoi int
	if err := tx.Underlying().QueryRowContext(ctx, docDayDeCap, xa, string(so), nam).Scan(&soCuoi); err != nil {
		// sql.ErrNoRows cannot happen: the INSERT above ran in this same transaction. If it ever
		// does, the row was removed underneath us — which the database refuses — so it is wrapped as
		// an ordinary failure rather than translated into a business answer.
		return 0, fmt.Errorf("day_so_van_ban: đọc dãy số để cấp: %w", err)
	}

	moi := soCuoi + 1
	if moi > TranSoMotNam {
		return 0, ErrDaySoDayTran
	}
	if _, err := tx.Exec(ctx, tangDay, xa, string(so), nam, moi); err != nil {
		return 0, fmt.Errorf("day_so_van_ban: tăng dãy số: %w", err)
	}
	return moi, nil
}
