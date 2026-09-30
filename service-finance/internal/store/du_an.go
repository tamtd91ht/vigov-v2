package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// DuAnStore reads a commune's investment projects and the disbursement figures derived from their
// vouchers.
//
// It holds *store.DB and never a *sql.DB. The commune is taken from the context on every call
// through db.For(ctx), so there is no constructor, no field and no method here that could produce
// a query without one (rule 1, invariant 5).
type DuAnStore struct {
	db *store.DB
}

func NewDuAnStore(db *store.DB) *DuAnStore { return &DuAnStore{db: db} }

// TranDuAnMotNam is the hard upper bound on one commune's projects in one budget year.
//
// WHY A BOUND AT ALL: one process serves 200+ communes, so every list route is a shared resource
// and an unbounded one is forbidden. This list is not yet paginated — the screen of §7 shows a
// commune's whole year at once, grouped by category, and it totals what it shows.
//
// 2000 IS ROUGHLY THIRTY TIMES THE REAL FIGURE. The specification's own commune carries 63
// projects in 2026 (§14). The number is not an estimate of how many there might be; it is the
// point past which the data is no longer one commune's budget year: an import run twice, a test
// fixture on a live database.
//
// WHEN A REAL COMMUNE APPROACHES IT, the answer is pagination plus server-side totals, NOT a
// bigger constant — a list this long stops being readable long before it stops being loadable.
const TranDuAnMotNam = 2000

// ErrQuaNhieuDuAn says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE, and here the cost of the alternative is money on a screen: this
// list is totalled into "kế hoạch vốn năm" and "đã giải ngân" for the whole commune (§3, §5). A
// silently short list produces totals that are simply too small, look entirely normal, and are
// reported upward. A refusal breaks one commune's screen loudly and names itself in the log.
var ErrQuaNhieuDuAn = errors.New("du_an: vượt trần số dự án một năm")

// ErrKhongThayDuAn is "no such project IN THIS COMMUNE" — and the two halves are one answer on
// purpose. A project of another commune is indistinguishable from one that does not exist,
// because the query cannot reach it at all; the caller answers 404 either way and so leaks
// nothing about what another authority holds.
var ErrKhongThayDuAn = errors.New("du_an: không có dự án này")

// cotDuAn IS READ BY POSITION in the scans below, and the list has pairs that swap with no error
// whatsoever:
//
//	ke_hoach_von_nam, tong_muc_duoc_duyet   two amounts — a swap changes every ratio on every
//	                                        screen and no row looks wrong.
//	ngay_khoi_cong, ngay_hoan_thanh,        three dates — a swap puts the disbursement deadline on
//	  thoi_han_giai_ngan                    the wrong clock, which is the figure §9 warns is NOT
//	                                        the completion date.
//
// That is why this is a constant rather than a column list written inline at each call site.
// `da_giai_ngan` is NOT in it: it is not a column anywhere, it is the aggregate the statements
// below compute, and it is appended after this list.
const cotDuAn = `da.id, da.ma, da.nam, da.hang_muc_id, da.ten, COALESCE(da.mo_ta, ''), ` +
	`da.ke_hoach_von_nam, COALESCE(da.tong_muc_duoc_duyet, 0), ` +
	`COALESCE(da.don_vi_thuc_hien_id, ''), COALESCE(da.can_bo_phu_trach_id, ''), ` +
	`da.ngay_khoi_cong, da.ngay_hoan_thanh, da.thoi_han_giai_ngan`

// tongChungTu is the subquery that produces "đã giải ngân" for every project of one commune.
//
// THE TOTAL IS DERIVED ON EVERY READ AND IS STORED NOWHERE. There is no `da_giai_ngan` column and
// there must never be one: two homes for one number drift, and the stale one is what reaches the
// report going upward (the reasoning rule 10 gives for refusing an `is_overdue` column, applied to
// money). This subquery is the single definition, used by every read path in this file, so the
// list screen and the detail screen cannot disagree about one project's figure.
//
// EVERY STATE COUNTS, INCLUDING `ke-toan-nhap`. §11 defines the total as the sum of vouchers
// "WHERE trạng thái ≥ kế toán nhập", and that is the FIRST state, so the condition admits all
// three. Adding `AND trang_thai <> 'ke-toan-nhap'` here would report every commune as further
// behind than it is — which is why the condition is written out rather than left to be assumed.
//
// SOFT-DELETED VOUCHERS ARE EXCLUDED, everywhere and always (rule 7, invariant 2).
//
// tenant_id = $1 IS INSIDE THE SUBQUERY AS WELL AS IN THE OUTER WHERE, and both are load-bearing:
// the join below matches on (tenant_id, du_an_id), so a subquery unconstrained by commune would
// still be correct — until somebody simplifies the ON clause to `du_an_id` alone, at which point
// two communes whose project ids collide would total each other's money. Constrained in both
// places, that edit cannot produce a leak.
const tongChungTu = `(SELECT ct.tenant_id, ct.du_an_id, SUM(ct.so_tien) AS da_giai_ngan
	          FROM chung_tu_giai_ngan ct
	          WHERE ct.tenant_id = $1 AND ct.deleted_at IS NULL
	          GROUP BY ct.tenant_id, ct.du_an_id)`

// quetDuAn reads one row into a project plus its derived total. Positional, in lockstep with
// cotDuAn — see the note there on the pairs that swap silently.
func quetDuAn(rows *sql.Rows) (domain.TienDoDuAn, error) {
	var (
		t             domain.TienDoDuAn
		khoiCong      sql.NullTime
		hoanThanh     sql.NullTime
		thoiHan       time.Time
		keHoach, tong int64
		daGiaiNgan    int64
	)
	err := rows.Scan(&t.DuAn.ID, &t.DuAn.Ma, &t.DuAn.Nam, &t.DuAn.HangMucID, &t.DuAn.Ten, &t.DuAn.MoTa,
		&keHoach, &tong, &t.DuAn.DonViThucHienID, &t.DuAn.CanBoPhuTrachID,
		&khoiCong, &hoanThanh, &thoiHan, &daGiaiNgan)
	if err != nil {
		return domain.TienDoDuAn{}, err
	}
	t.DuAn.KeHoachVonNam = domain.Dong(keHoach)
	t.DuAn.TongMucDuocDuyet = domain.Dong(tong)
	t.DaGiaiNgan = domain.Dong(daGiaiNgan)
	t.DuAn.ThoiHanGiaiNgan = thoiHan
	if khoiCong.Valid {
		t.DuAn.NgayKhoiCong = khoiCong.Time
	}
	if hoanThanh.Valid {
		t.DuAn.NgayHoanThanh = hoanThanh.Time
	}
	return t, nil
}

// LocDuAn is the filter of §7.1, minus the parts that are not decided here.
//
// `chi_du_an_cham` IS DELIBERATELY NOT A STORE FILTER. Whether a project is behind is DERIVED
// from a clock and a threshold (domain.LaCham), so filtering it in SQL would mean re-implementing
// that arithmetic in a second language — the classic two-homes-for-one-rule failure. The caller
// applies domain.LaCham to what this returns.
//
// THE FREE-TEXT SEARCH `q` OF §7.1 IS NOT HERE EITHER, and that is an omission with a name rather
// than an oversight: matching a Vietnamese project name needs a decision about accent folding and
// about whether the code is matched as a prefix or a substring, and guessing it would produce a
// search box that silently misses rows.
type LocDuAn struct {
	// Nam is REQUIRED. There is no "all years" read: §13 rule 8 makes each budget year its own set
	// of projects, and a list that mixed two years would total one commune's plan twice. DanhSach
	// refuses a zero rather than defaulting to the current year — a default on a filter that
	// decides which money is being reported is a wrong report nobody can see.
	Nam int

	// HangMucID empty means every category. This is a filter, not an isolation boundary.
	HangMucID string
}

// ErrThieuNamNganSach is a caller that did not say which budget year it wants.
var ErrThieuNamNganSach = errors.New("du_an: thiếu năm ngân sách")

// DanhSach reads one commune's projects for one budget year, each with its derived disbursed
// total (§7).
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: QueryJoin binds it to $1 from the context, and
// every table in the statement is constrained to that same $1 — the project, and the voucher
// subquery behind the join. Joining on id alone would match another commune's rows wherever ids
// collide, and no test of a single commune would ever show it (rule 1).
//
// ORDER BY da.ma: the project code is unique per commune (UNIQUE (tenant_id, ma)), so the order is
// TOTAL and two calls cannot swap two rows. An unstable order makes a client-side diff flicker on
// every reload and makes any test of this compare sets by accident.
func (s *DuAnStore) DanhSach(ctx context.Context, loc LocDuAn) ([]domain.TienDoDuAn, error) {
	if loc.Nam == 0 {
		return nil, ErrThieuNamNganSach
	}

	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling returns a full page indistinguishable from a complete list of
	// exactly that size — the truncation this read refuses to perform, performed by the bound meant
	// to prevent it.
	stmt := `SELECT ` + cotDuAn + `, COALESCE(ct.da_giai_ngan, 0)
		FROM du_an da
		LEFT JOIN ` + tongChungTu + ` ct
		       ON ct.tenant_id = da.tenant_id AND ct.du_an_id = da.id
		WHERE da.tenant_id = $1 AND da.deleted_at IS NULL AND da.nam = $2
		  AND ($3 = '' OR da.hang_muc_id = $3)
		ORDER BY da.ma
		LIMIT $4`

	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, loc.Nam, loc.HangMucID, TranDuAnMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("du_an: đọc danh sách: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.TienDoDuAn, 0, 64)
	for rows.Next() {
		mot, err := quetDuAn(rows)
		if err != nil {
			return nil, fmt.Errorf("du_an: đọc dòng: %w", err)
		}
		ra = append(ra, mot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("du_an: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDuAnMotNam {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might total anyway is how a refusal turns back into a silent truncation one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuDuAn
	}
	return ra, nil
}

// ChiTiet reads one project of the commune in the context, with its derived disbursed total (§8).
//
// IT USES THE SAME `tongChungTu` SUBQUERY AS DanhSach, which is the point: the figure on the
// detail page and the figure on the list are the same expression, so they cannot disagree. A
// second, "simpler" SUM written here is how one screen starts excluding unconfirmed vouchers while
// the other counts them.
func (s *DuAnStore) ChiTiet(ctx context.Context, id string) (domain.TienDoDuAn, error) {
	stmt := `SELECT ` + cotDuAn + `, COALESCE(ct.da_giai_ngan, 0)
		FROM du_an da
		LEFT JOIN ` + tongChungTu + ` ct
		       ON ct.tenant_id = da.tenant_id AND ct.du_an_id = da.id
		WHERE da.tenant_id = $1 AND da.deleted_at IS NULL AND da.id = $2`

	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, id)
	if err != nil {
		return domain.TienDoDuAn{}, fmt.Errorf("du_an: đọc chi tiết: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.TienDoDuAn{}, fmt.Errorf("du_an: đọc chi tiết: %w", err)
		}
		return domain.TienDoDuAn{}, ErrKhongThayDuAn
	}
	mot, err := quetDuAn(rows)
	if err != nil {
		return domain.TienDoDuAn{}, fmt.Errorf("du_an: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.TienDoDuAn{}, fmt.Errorf("du_an: duyệt kết quả: %w", err)
	}
	return mot, nil
}
