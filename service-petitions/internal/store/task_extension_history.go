package store

// The EXTENSION HISTORY of ONE task (§5.8, user decision 07/10/2026) — GET /api/v1/tasks/{ma}/extensions.
// SQL, and nothing else.
//
// A SEPARATE READ FROM THE APPROVAL QUEUE (de_nghi_lui_han_cho_duyet.go), deliberately: the queue is
// pending-only across the commune, joined to its task, oldest first, and its predicate builder is shared
// with the badge count (pendingExtensionFilter). This page is EVERY status of one task, newest first,
// with the decider's note. Folding it into the queue's builder would change the badge's set.
//
// ⚠ `ly_do` AND `decision_note` ARE STAFF FREE TEXT that may name people (rule 3). No error built here
// quotes either, and none quotes a task number or a staff code.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// TaskExtensionHistorySort is the ONE order the history offers: NEWEST REQUEST FIRST, `id` as the
// tie-break store.QueryPage always appends — the order of the task's own timeline (SapXepNhatKyNhiemVu),
// so the drawer reads both alike. Param `requested_at`, the wire name of `thoi_diem` on deNghiLuiHanRa.
//
// Served by `de_nghi_lui_han_theo_nhiem_vu` (tenant_id, nhiem_vu_id, thoi_diem DESC) of migration 0006.
// `thoi_diem` IS NOT NULL there, which the `(col, id) < (…)` keyset needs.
var TaskExtensionHistorySort = page.NewAllowlist(page.Desc,
	page.Col("requested_at", "thoi_diem", page.KindTime),
)

var taskExtensionHistoryAnchor = store.NewMoc[domain.DeNghiLuiHan](TaskExtensionHistorySort,
	map[string]func(domain.DeNghiLuiHan) page.Key{
		"requested_at": func(d domain.DeNghiLuiHan) page.Key { return page.TimeKey(d.ThoiDiem) },
	})

// taskExtensionHistoryColumns IS READ BY POSITION in the Scan below: cotDeNghi's nine, then the note.
// cotDeNghi is NOT widened instead, because the locked read of the decision path uses it and has no
// use for a note that is NULL on every pending row.
const taskExtensionHistoryColumns = cotDeNghi + `, decision_note`

// TaskHistory reads ONE PAGE of one task's extension requests — every status — by the task's INTERNAL
// id, which the caller has from the row TheoMa just read (that read is the soft-delete and commune
// check on the TASK). ONE STATEMENT PER PAGE.
//
// THE COMMUNE IS NOT A PARAMETER: $1 from the context (store.Scoped), so another commune's task id
// matches no row even if a caller had one (rule 1, invariant 5).
//
// `deleted_at IS NULL`: a soft-deleted request leaves every read path (rule 7, invariant 2). The one
// read of this table that keeps removed rows is ApprovedCount, for the reason it states.
func (s *DeNghiLuiHanStore) TaskHistory(ctx context.Context, taskID string, yc page.Request) (
	page.Result[domain.DeNghiLuiHan], error) {

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: taskExtensionHistoryColumns,
		Table:   "de_nghi_lui_han",
		Filter:  `AND nhiem_vu_id = $2 AND deleted_at IS NULL`,
		Args:    []any{taskID},
	}, yc, taskExtensionHistoryAnchor, func(rows *sql.Rows) (domain.DeNghiLuiHan, string, error) {
		var (
			d       domain.DeNghiLuiHan
			status  string
			decider sql.NullString
			decided sql.NullTime
			note    sql.NullString
		)
		if err := rows.Scan(&d.ID, &d.NhiemVuID, &d.NguoiDeNghiMa, &decider, &d.HanMoi, &d.LyDo,
			&status, &d.ThoiDiem, &decided, &note); err != nil {
			return domain.DeNghiLuiHan{}, "", fmt.Errorf("de_nghi_lui_han: quét dòng lịch sử lùi hạn: %w", err)
		}
		d.TrangThai = domain.TrangThaiDeNghi(status)
		d.NguoiDuyetMa = decider.String
		d.DuyetLuc = decided.Time
		d.DecisionNote = note.String
		return d, d.ID, nil
	})
}
