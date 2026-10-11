package crosstenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
)

// ActiveStaffCountStore answers OperatorService.ListActiveStaffCounts: per commune, how many active
// staff accounts it has — the denominator of "Đã ghép nối" on platform-admin's Zalo Bot communes
// table. SHAPE 4 of doc.go: a COUNT across every commune, for the Vihat operator, answered for THIS
// read only by the owner on 11/10/2026 (operator.proto, the RPC and the service comment).
//
// COUNTS ONLY, BY CONSTRUCTION: the statement selects tenant_id and count(*) and nothing else, so no
// code, name, email or unit of any person can leave through this type (ADR 0003).
//
// IT HOLDS A RAW *sql.DB FOR THIS ONE READ: there is no commune to scope it to (core/store.For would
// panic), which is exactly why it lives in this package. The handle never leaves. The audit entry
// the read requires (rule 6, invariant 7) is the use case's — app.ActiveStaffCounts — because only
// it knows which operator read.
type ActiveStaffCountStore struct {
	raw *sql.DB
}

func NewActiveStaffCountStore(db *sql.DB) *ActiveStaffCountStore {
	if db == nil {
		panic("crosstenant: NewActiveStaffCountStore needs a database handle")
	}
	return &ActiveStaffCountStore{raw: db}
}

// CommuneStaffCount is one commune's count. Count is always >= 1: a commune with none is absent.
type CommuneStaffCount struct {
	TenantID string
	Count    uint32
}

// ActiveStaffAccountFilter is "an active staff account" — the owner's "not soft-deleted AND not
// locked" (11/10/2026) as identity's rows say it. It MUST equal store.locChonNguoi, the picker's
// predicate the contract names: a denominator that counted somebody the picker cannot offer would
// make 100% linked unreachable. It cannot be imported from there (package store's tests import this
// package — a cycle), so it is restated here and PINNED: store/active_staff_filter_pin_test.go fails
// the day the two differ.
const ActiveStaffAccountFilter = `AND deleted_at IS NULL AND co_tai_khoan AND dang_hoat_dong`

// activeStaffCountsQuery — COLLATE "C" so the order is byte order whatever the database locale: the
// contract promises a stable order (by tenant_id), not a meaningful one.
const activeStaffCountsQuery = `SELECT tenant_id, count(*)
FROM nguoi_dung
WHERE TRUE ` + ActiveStaffAccountFilter + `
GROUP BY tenant_id
ORDER BY tenant_id COLLATE "C"`

// ActiveStaffCounts counts active staff accounts per commune, every commune at once.
func (s *ActiveStaffCountStore) ActiveStaffCounts(ctx context.Context) ([]CommuneStaffCount, error) {
	// @cross-tenant: one operator screen (platform-admin, Zalo Bot communes table) shows every
	// commune's uptake of the Zalo Bot channel; owner's decision 11/10/2026 for this read only
	// (operator.proto ListActiveStaffCounts). Counts per tenant_id — no row of any person leaves.
	rows, err := s.raw.QueryContext(ctx, activeStaffCountsQuery)
	if err != nil {
		return nil, fmt.Errorf("active staff counts: query: %w", err)
	}
	defer rows.Close()
	out := make([]CommuneStaffCount, 0, 64)
	for rows.Next() {
		var (
			tenantID string
			n        int64
		)
		if err := rows.Scan(&tenantID, &n); err != nil {
			return nil, fmt.Errorf("active staff counts: scan: %w", err)
		}
		if n < 1 || n > math.MaxUint32 {
			// GROUP BY never yields 0, and 4 billion staff in one commune is a corrupt read, not a
			// figure to truncate into a screen.
			return nil, errors.New("active staff counts: count out of range for one commune")
		}
		out = append(out, CommuneStaffCount{TenantID: tenantID, Count: uint32(n)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("active staff counts: %w", err)
	}
	return out, nil
}
