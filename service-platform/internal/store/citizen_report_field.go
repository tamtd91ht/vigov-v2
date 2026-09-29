package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// CitizenReportFieldStore is the only path to `citizen_report_field` (migrations 0011, 0012), tier 1 of the
// citizen report field catalogue (ADR 0026, ADR 0060).
//
// A RAW *sql.DB, LIKE UploadPolicyStore: the table has no tenant_id, so core/store.Scoped — which
// injects WHERE tenant_id = $1 — cannot read it. It holds one code set shared by every commune, no
// business content and no personal data, and this type only READS it. The write path (Vihat's
// operations zone, ADR 0048) is not built; when it is, it writes the row and its platform_audit_log
// entry in one transaction (rule 6, invariant 3).
type CitizenReportFieldStore struct {
	db *sql.DB
}

func NewCitizenReportFieldStore(db *sql.DB) *CitizenReportFieldStore {
	return &CitizenReportFieldStore{db: db}
}

// listCitizenReportFields reads EVERY code, retired ones included.
//
// THERE IS DELIBERATELY NO `WHERE is_active`: a retired code is still the code on every petition filed
// under it before retirement, and those petitions need its label for as long as they exist. Hiding
// retired codes is the caller's decision on its WRITE paths only (ADR 0060 §4; platform.proto).
//
// The order is total (sort_order, then the primary key) so two rows sharing a sort_order cannot swap
// places between two reads.
const listCitizenReportFields = `
	SELECT code, default_label, sort_order, icon, tone, is_active
	FROM citizen_report_field
	ORDER BY sort_order, code`

// ListCitizenReportFields returns every tier-1 code. An empty slice is an ordinary answer (no code is
// valid), not an error.
func (s *CitizenReportFieldStore) ListCitizenReportFields(ctx context.Context) ([]domain.CitizenReportField, error) {
	// No tenant filter: one code set for every commune (see the type comment).
	rows, err := s.db.QueryContext(ctx, listCitizenReportFields)
	if err != nil {
		return nil, fmt.Errorf("citizen_report_field: query: %w", err)
	}
	defer rows.Close()

	var out []domain.CitizenReportField
	for rows.Next() {
		var f domain.CitizenReportField
		if err := rows.Scan(&f.Code, &f.DefaultLabel, &f.SortOrder, &f.Icon, &f.Tone, &f.IsActive); err != nil {
			return nil, fmt.Errorf("citizen_report_field: scan: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("citizen_report_field: read: %w", err)
	}
	return out, nil
}
