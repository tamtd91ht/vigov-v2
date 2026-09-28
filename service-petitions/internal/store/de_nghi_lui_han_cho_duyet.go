package store

// The APPROVAL QUEUE of extension requests (§5.8) — GET /api/v1/task-extensions. SQL, and nothing
// else. The two write statements on `de_nghi_lui_han` live in nhiem_vu_ghi.go; this file is the one
// page a leader's queue renders.
//
// ⚠ `ly_do` IS FREE TEXT AN OFFICER TYPED, AND `tieu_de` OFTEN QUOTES A CITIZEN'S COMPLAINT (rule 3).
// No error built here quotes either, and none quotes a staff code.

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// SapXepDeNghiChoDuyet is the ONE order the queue offers: OLDEST PENDING FIRST, `id` as the
// tie-break store.QueryPage always appends.
//
// ASCENDING AND NOT THE HOUSE'S USUAL NEWEST-FIRST, because of what the list IS: an approval queue.
// The request waiting longest is the one whose task is closest to missing the deadline it asks to
// move, and newest-first would bury it below everything filed since. `order=desc` still works — the
// allowlist fixes the column, not the direction.
//
// `thoi_diem` IS NOT NULL (migration 0006), which is what the `(col, id) > (…)` keyset needs from a
// sort column.
var SapXepDeNghiChoDuyet = page.NewAllowlist(page.Asc,
	page.Col("requested_at", "thoi_diem", page.KindTime),
)

var mocDeNghiChoDuyet = store.NewMoc[domain.DeNghiLuiHanChoDuyet](SapXepDeNghiChoDuyet,
	map[string]func(domain.DeNghiLuiHanChoDuyet) page.Key{
		"requested_at": func(d domain.DeNghiLuiHanChoDuyet) page.Key { return page.TimeKey(d.DeNghi.ThoiDiem) },
	})

// LocDeNghiChoDuyet is the one filter the queue accepts, already resolved by the handler.
//
// LanhDaoGiaoViecMa is the CALLER'S OWN staff code for `approver=me`, taken from the session
// principal — never from the request (rule 4, invariant 2 in its staff form). "" = the whole commune.
type LocDeNghiChoDuyet struct {
	LanhDaoGiaoViecMa string

	// TaskCode narrows the queue to ONE task's pending requests, by its REGISTER NUMBER (`NV19`) —
	// §5.8's approve/reject block inside that task's drawer. "" = every task.
	//
	// Compared with the JOINED task's `ma`, so a number of another commune, of a soft-deleted task,
	// or of nothing at all yields an empty page — the same answer as a task with nothing pending.
	// Served by `de_nghi_lui_han_theo_nhiem_vu` (tenant_id, nhiem_vu_id, thoi_diem DESC) of migration
	// 0006 once `UNIQUE (tenant_id, ma)` has resolved the one task.
	TaskCode string
}

// bangDeNghiChoDuyet is the joined relation the page is cut from, as a DERIVED TABLE.
//
// # WHY A DERIVED TABLE HANDED TO store.QueryPage, AND NOT QueryJoin WITH A HAND-WRITTEN KEYSET
//
// QueryPage owns the ORDER BY, the anchor comparison and the limit+1 — the three things whose
// mistakes are silent (skipped or repeated rows). A hand-written keyset over QueryJoin would be a
// second copy of that logic in a service package. Wrapping the join in a derived table lets
// QueryPage keep owning all three over ONE relation whose sort column and `id` are unambiguous.
//
// # THE COMMUNE IS $1 ON BOTH TABLES (rule 1, invariant 5; store.Scoped.QueryJoin's contract)
//
//	d.tenant_id = $1   the request rows of this commune — and the partition key, so PostgreSQL
//	                   prunes to one of the 32 partitions
//	n.tenant_id = $1   IN THE ON CLAUSE. Joining on `nhiem_vu_id` alone would attach ANOTHER
//	                   COMMUNE'S task wherever two internal ids collide — a leak no single-commune
//	                   test could show
//	outer tenant_id    QueryPage adds `WHERE tenant_id = $1` over the derived table as well; it is
//	                   redundant with the two above and harmless, and it is the shape every page in
//	                   this repository has. ⚠ IT BINDS ONLY THE `d` SIDE — the derived table exposes
//	                   `d.tenant_id` alone. The `n` side is protected by the ON clause and nothing
//	                   else: "simplifying" that clause away because the outer filter "covers it"
//	                   attaches another commune's task (title, deadline, leader) to this page
//
// # WHAT IS EXCLUDED, AND WHERE
//
//	d.trang_thai = $2      pending only. BOUND to domain.ChoDuyetLuiHan rather than written as a
//	                       literal, so the code has one spelling (the reason cauKetLuanKemDem gives)
//	d.deleted_at IS NULL   rule 7, invariant 2 — the request table has soft delete
//	n.deleted_at IS NULL   a request on a soft-deleted task leaves the queue: the task is gone from
//	                       every read path, and a leader cannot open it to decide (the decision route
//	                       reads it through TheoMa, which excludes it too)
//
// INNER JOIN: a request whose task is missing or deleted yields no row. That is the answer, not an
// error — the decision route would 404 on it.
//
// FORMATTING CONSTRAINT (fake driver, see driver_gia_test.go): the fake reads the SELECT list between
// the FIRST `SELECT ` and the FIRST ` FROM `, which is the OUTER select that QueryPage builds; this
// text only ever comes after it.
const bangDeNghiChoDuyet = `(SELECT d.tenant_id, d.id, d.nhiem_vu_id, d.nguoi_de_nghi_ma, d.han_moi, d.ly_do, d.thoi_diem,
		n.ma AS nhiem_vu_ma, n.tieu_de AS nhiem_vu_tieu_de, n.han_xu_ly AS nhiem_vu_han_xu_ly,
		n.lanh_dao_giao_viec_ma
	FROM de_nghi_lui_han d
	JOIN nhiem_vu n ON n.tenant_id = $1 AND n.id = d.nhiem_vu_id AND n.deleted_at IS NULL
	WHERE d.tenant_id = $1 AND d.trang_thai = $2 AND d.deleted_at IS NULL) AS dn`

// cotDeNghiChoDuyet IS READ BY POSITION in the Scan below. `han_moi` and `nhiem_vu_han_xu_ly` are two
// TIMESTAMPTZs a swap between which produces no error — a leader shown the current deadline as the
// requested one — so the fake-driver suite gives them visibly different values.
const cotDeNghiChoDuyet = `id, nhiem_vu_id, nguoi_de_nghi_ma, han_moi, ly_do, thoi_diem, nhiem_vu_ma, nhiem_vu_tieu_de, nhiem_vu_han_xu_ly, lanh_dao_giao_viec_ma`

// ChoDuyet reads ONE PAGE of the commune's pending extension requests, each with its task's number,
// title, current deadline and named leader. ONE STATEMENT PER PAGE.
//
// THE COMMUNE IS NOT A PARAMETER. It is $1 from the context (store.Scoped), bound on both tables.
func (s *DeNghiLuiHanStore) ChoDuyet(ctx context.Context, loc LocDeNghiChoDuyet, yc page.Request) (
	page.Result[domain.DeNghiLuiHanChoDuyet], error) {

	args := []any{string(domain.ChoDuyetLuiHan)}
	dieuKien := ""
	if loc.LanhDaoGiaoViecMa != "" {
		args = append(args, loc.LanhDaoGiaoViecMa)
		dieuKien = ` AND lanh_dao_giao_viec_ma = $3`
	}
	if loc.TaskCode != "" {
		// NUMBERED FROM WHAT IS ALREADY BOUND: $3 when `approver=me` is absent, $4 when it is present.
		args = append(args, loc.TaskCode)
		dieuKien += ` AND nhiem_vu_ma = $` + strconv.Itoa(len(args)+1)
	}

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotDeNghiChoDuyet,
		Table:   bangDeNghiChoDuyet,
		Filter:  dieuKien,
		Args:    args,
	}, yc, mocDeNghiChoDuyet, func(rows *sql.Rows) (domain.DeNghiLuiHanChoDuyet, string, error) {
		var (
			d               domain.DeNghiLuiHanChoDuyet
			hanXuLy         sql.NullTime
			lanhDaoGiaoViec sql.NullString
		)
		if err := rows.Scan(&d.DeNghi.ID, &d.DeNghi.NhiemVuID, &d.DeNghi.NguoiDeNghiMa,
			&d.DeNghi.HanMoi, &d.DeNghi.LyDo, &d.DeNghi.ThoiDiem,
			&d.NhiemVuMa, &d.NhiemVuTieuDe, &hanXuLy, &lanhDaoGiaoViec); err != nil {
			return domain.DeNghiLuiHanChoDuyet{}, "", fmt.Errorf("de_nghi_lui_han: quét dòng hàng chờ duyệt: %w", err)
		}
		// Every row of this statement is pending by its WHERE clause; stated on the value rather than
		// left as the zero code, so no reader sees a request with an empty status.
		d.DeNghi.TrangThai = domain.ChoDuyetLuiHan
		d.HanXuLyHienTai = hanXuLy.Time
		d.LanhDaoGiaoViecMa = lanhDaoGiaoViec.String
		return d, d.DeNghi.ID, nil
	})
}
