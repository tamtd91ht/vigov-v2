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
//
// `implementing_unit` (0016) IS AFTER the three dates, then `at_risk` (0018), so no existing position
// moved. at_risk is NOT NULL DEFAULT false — no COALESCE needed, and none would be honest: a NULL there
// is a schema fault, and the scan into a bool fails loudly rather than reading "not at risk".
const cotDuAn = `da.id, da.ma, da.nam, da.hang_muc_id, da.ten, COALESCE(da.mo_ta, ''), ` +
	`da.ke_hoach_von_nam, COALESCE(da.tong_muc_duoc_duyet, 0), ` +
	`COALESCE(da.don_vi_thuc_hien_id, ''), COALESCE(da.can_bo_phu_trach_id, ''), ` +
	`da.ngay_khoi_cong, da.ngay_hoan_thanh, da.thoi_han_giai_ngan, ` +
	`COALESCE(da.implementing_unit, ''), da.at_risk`

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
		&khoiCong, &hoanThanh, &thoiHan, &t.DuAn.ImplementingUnit, &t.DuAn.AtRisk, &daGiaiNgan)
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

	// ImplementingUnit empty means every unit; otherwise an EXACT match on `implementing_unit` — the
	// prototype's filter (budget/repository.py:147-149), whose values come from
	// ImplementingUnitsOfYear. A filter inside the commune, not an isolation boundary.
	ImplementingUnit string
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
		  AND ($4 = '' OR da.implementing_unit = $4)
		ORDER BY da.ma
		LIMIT $5`

	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, loc.Nam, loc.HangMucID, loc.ImplementingUnit,
		TranDuAnMotNam+1)
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

// ErrTooManyImplementingUnits — more distinct units in one year than TranDuAnMotNam, which cannot
// happen without the project ceiling being broken too. REFUSED, NOT TRUNCATED: a short filter list is a
// unit nobody can select, with nothing saying so.
var ErrTooManyImplementingUnits = errors.New("du_an: vượt trần số đơn vị thực hiện một năm")

// ImplementingUnitsOfYear lists the distinct "Đơn vị thực hiện" typed on this commune's LIVE projects of
// one budget year — the options of the list's filter (prototype budget/repository.py:355-366).
//
// THE YEAR IS REQUIRED, unlike the prototype's (`budget_year` optional there): §13 rule 8 makes each
// year its own set of projects, and the list it filters is year-scoped, so an "every year" option list
// would offer units that match nothing on the screen.
//
// ONE TABLE, so Scoped.Query, which writes `WHERE tenant_id = $1` itself (rule 1, invariant 5). Blank
// values cannot be stored (0016's CHECK); the `btrim` predicate is kept so a row written before that
// CHECK by another writer still never offers an empty option.
//
// SORTED BY THE DATABASE'S COLLATION — this repository has no Vietnamese collation helper, and inventing
// one here would be a second sort order nobody else uses. Exact-match filter values, so the order is
// display only.
func (s *DuAnStore) ImplementingUnitsOfYear(ctx context.Context, year int) ([]string, error) {
	if year == 0 {
		return nil, ErrThieuNamNganSach
	}
	rows, err := s.db.For(ctx).Query(ctx, "DISTINCT implementing_unit", "du_an",
		`AND deleted_at IS NULL AND nam = $2
		   AND implementing_unit IS NOT NULL AND btrim(implementing_unit) <> ''
		 ORDER BY implementing_unit
		 LIMIT $3`, year, TranDuAnMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("du_an: đọc đơn vị thực hiện theo năm: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 16)
	for rows.Next() {
		var unit string
		if err := rows.Scan(&unit); err != nil {
			return nil, fmt.Errorf("du_an: đọc dòng đơn vị thực hiện: %w", err)
		}
		out = append(out, unit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("du_an: duyệt đơn vị thực hiện: %w", err)
	}
	if len(out) > TranDuAnMotNam {
		return nil, ErrTooManyImplementingUnits
	}
	return out, nil
}

// TranPhanBoMotNam bounds the allocation lines read for ONE budget year's project list.
//
// 20000 IS TEN SOURCES ON EVERY ONE OF TranDuAnMotNam PROJECTS. §14's commune carries 63 projects and
// §6's sample four sources, so a real year is a few hundred lines. Past this the data is no longer a
// commune's capital plan, and the read REFUSES rather than truncates — a short list here is a project
// whose chip says "Chưa gắn nguồn" while it has sources (ErrQuaNhieuPhanBo's reasoning).
const TranPhanBoMotNam = 20000

// allocationsOfYear lists the live allocation lines of one year's live projects, with each source's
// name, in the commune's own source order — ONE statement for the whole page (no read per project).
//
// THE SAME FILTERS AS DanhSach (`da.nam = $2`, the optional category `$3`), so the lines match the
// projects the list returns and nothing else.
//
// `nv.deleted_at IS NULL`: a line naming a source that is no longer in the catalogue is left out, the
// way §6's cards leave it out (tongPhanBoTheoNguon joins only live sources). The catalogue has no
// remove route today (decision 06/10/2026), so this excludes nothing in practice; it keeps the chip and
// the cards agreeing should one ever be added.
//
// tenant_id = $1 ON EVERY TABLE, for the reason tongChungTu gives (rule 1).
const allocationsOfYear = `SELECT pb.du_an_id, pb.nguon_von_id, nv.ten, pb.so_tien_phan_bo
	FROM phan_bo_nguon_von pb
	JOIN du_an da
	  ON da.tenant_id = pb.tenant_id AND da.tenant_id = $1 AND da.id = pb.du_an_id
	JOIN nguon_von nv
	  ON nv.tenant_id = pb.tenant_id AND nv.tenant_id = $1 AND nv.id = pb.nguon_von_id
	WHERE pb.tenant_id = $1 AND pb.deleted_at IS NULL
	  AND da.deleted_at IS NULL AND da.nam = $2
	  AND ($3 = '' OR da.hang_muc_id = $3)
	  AND nv.deleted_at IS NULL
	ORDER BY pb.du_an_id, nv.thu_tu, nv.ten, nv.id
	LIMIT $4`

// AllocationsOfYear reads the allocation lines behind the list's chips, keyed by project id. A project
// with no line is simply absent from the map — "Chưa gắn nguồn".
func (s *DuAnStore) AllocationsOfYear(ctx context.Context,
	loc LocDuAn) (map[string][]domain.ProjectAllocation, error) {

	if loc.Nam == 0 {
		return nil, ErrThieuNamNganSach
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, allocationsOfYear, loc.Nam, loc.HangMucID, TranPhanBoMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("phan_bo_nguon_von: đọc theo năm: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]domain.ProjectAllocation, 64)
	n := 0
	for rows.Next() {
		var (
			projectID string
			a         domain.ProjectAllocation
			amount    int64
		)
		if err := rows.Scan(&projectID, &a.FundingSourceID, &a.SourceName, &amount); err != nil {
			return nil, fmt.Errorf("phan_bo_nguon_von: đọc dòng theo năm: %w", err)
		}
		a.Amount = domain.Dong(amount)
		out[projectID] = append(out[projectID], a)
		n++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phan_bo_nguon_von: duyệt theo năm: %w", err)
	}
	if n > TranPhanBoMotNam {
		return nil, ErrQuaNhieuPhanBo
	}
	return out, nil
}

// allocationsOfProject lists one project's live allocation lines with what its vouchers drew FROM EACH
// SOURCE — the "Giải ngân theo nguồn vốn" block of §8 (prototype BudgetItemDetail.tsx:333-372).
//
// THE VOUCHER SUBQUERY IS tongChungTu's RULE restricted to one project and grouped by source: every
// state counts, soft-deleted vouchers do not. Vouchers naming NO source belong to no line (§13 rule 6);
// vouchers naming a source the project has no line for appear on no line either — the per-source bars
// show allocated sources only, as the prototype does.
const allocationsOfProject = `SELECT pb.nguon_von_id, nv.ten, pb.so_tien_phan_bo, COALESCE(ct.da_giai_ngan, 0)
	FROM phan_bo_nguon_von pb
	JOIN du_an da
	  ON da.tenant_id = pb.tenant_id AND da.tenant_id = $1 AND da.id = pb.du_an_id
	JOIN nguon_von nv
	  ON nv.tenant_id = pb.tenant_id AND nv.tenant_id = $1 AND nv.id = pb.nguon_von_id
	LEFT JOIN (SELECT c.tenant_id, c.nguon_von_id, SUM(c.so_tien) AS da_giai_ngan
	             FROM chung_tu_giai_ngan c
	            WHERE c.tenant_id = $1 AND c.du_an_id = $2 AND c.deleted_at IS NULL
	              AND c.nguon_von_id IS NOT NULL
	            GROUP BY c.tenant_id, c.nguon_von_id) ct
	  ON ct.tenant_id = pb.tenant_id AND ct.nguon_von_id = pb.nguon_von_id
	WHERE pb.tenant_id = $1 AND pb.du_an_id = $2 AND pb.deleted_at IS NULL
	  AND da.deleted_at IS NULL AND nv.deleted_at IS NULL
	ORDER BY nv.thu_tu, nv.ten, nv.id
	LIMIT $3`

// MaxVouchersPerProject bounds §8.2's voucher list of ONE project.
//
// A real project carries a handful to a few dozen payments (instalments, treasury lines); 5000 is
// past any of them — the point where the data is no longer one project's payments (an import run
// twice, a fixture on a live database). The read REFUSES rather than truncates: the screen sums what
// it shows, and a silently short list is a project that looks less disbursed than it is.
const MaxVouchersPerProject = 5000

// ErrTooManyVouchers says the ceiling was reached. The caller answers 500 and refuses.
var ErrTooManyVouchers = errors.New("chung_tu_giai_ngan: dự án vượt trần số chứng từ")

// vouchersOfProject lists one project's LIVE vouchers, newest payment date first.
//
// ORDER BY ngay_chi DESC, id DESC: the id is a ULID, so among vouchers paid on the same day the one
// entered last comes first, and the order is TOTAL — two reads cannot swap two rows.
//
// THE SOURCE NAME IS A CORRELATED SUBQUERY, NOT A JOIN, so cotChungTu's unqualified columns stay
// unambiguous. It is constrained to tenant_id = $1 like every other table here (rule 1). It does NOT
// filter `nv.deleted_at`: a voucher drawn from a source is drawn from it historically, and blanking the
// name would make the row read as "no source" — which §6 counts as something else entirely.
//
// EVERY STATE, soft-deleted excluded — the same set tongChungTu totals, so the tab and the project's
// "đã giải ngân" cannot disagree.
const vouchersOfProject = `SELECT ` + cotChungTu + `,
	COALESCE((SELECT nv.ten FROM nguon_von nv
	           WHERE nv.tenant_id = $1 AND nv.id = ct.nguon_von_id), '')
	FROM chung_tu_giai_ngan ct
	WHERE ct.tenant_id = $1 AND ct.du_an_id = $2 AND ct.deleted_at IS NULL
	ORDER BY ct.ngay_chi DESC, ct.id DESC
	LIMIT $3`

// VouchersOfProject reads the voucher tab of one LIVE project of this commune.
//
// THE PROJECT IS CHECKED FIRST, so "no such project" (404) and "a project with no voucher yet" ([])
// are two different answers. A project of another commune is ErrKhongThayDuAn — the statement cannot
// reach it.
func (s *DuAnStore) VouchersOfProject(ctx context.Context, projectID string) ([]domain.ProjectVoucher, error) {
	scoped := s.db.For(ctx)
	exists, err := scoped.Query(ctx, "1", "du_an", "AND id = $2 AND deleted_at IS NULL", projectID)
	if err != nil {
		return nil, fmt.Errorf("du_an: kiểm dự án của danh sách chứng từ: %w", err)
	}
	found := exists.Next()
	if err := exists.Err(); err != nil {
		exists.Close()
		return nil, fmt.Errorf("du_an: kiểm dự án của danh sách chứng từ: %w", err)
	}
	exists.Close()
	if !found {
		return nil, ErrKhongThayDuAn
	}

	rows, err := scoped.QueryJoin(ctx, vouchersOfProject, projectID, MaxVouchersPerProject+1)
	if err != nil {
		return nil, fmt.Errorf("chung_tu_giai_ngan: đọc theo dự án: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ProjectVoucher, 0, 16)
	for rows.Next() {
		var (
			v    domain.ProjectVoucher
			name string
		)
		v.Voucher, err = docMotDongChungTu(func(dest ...any) error {
			return rows.Scan(append(dest, &name)...)
		})
		if err != nil {
			return nil, fmt.Errorf("chung_tu_giai_ngan: đọc dòng theo dự án: %w", err)
		}
		v.FundingSourceName = name
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chung_tu_giai_ngan: duyệt theo dự án: %w", err)
	}
	if len(out) > MaxVouchersPerProject {
		return nil, ErrTooManyVouchers
	}
	return out, nil
}

// AllocationsOfProject reads one project's allocation lines for the detail screen. A project of
// another commune yields no line — the statement binds tenant_id = $1 on every table; the caller has
// already answered 404 for it through ChiTiet.
func (s *DuAnStore) AllocationsOfProject(ctx context.Context, projectID string) ([]domain.ProjectAllocation, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, allocationsOfProject, projectID, TranPhanBoMotDuAn+1)
	if err != nil {
		return nil, fmt.Errorf("phan_bo_nguon_von: đọc theo dự án: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ProjectAllocation, 0, 4)
	for rows.Next() {
		var (
			a                 domain.ProjectAllocation
			amount, disbursed int64
		)
		if err := rows.Scan(&a.FundingSourceID, &a.SourceName, &amount, &disbursed); err != nil {
			return nil, fmt.Errorf("phan_bo_nguon_von: đọc dòng theo dự án: %w", err)
		}
		a.Amount, a.Disbursed = domain.Dong(amount), domain.Dong(disbursed)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phan_bo_nguon_von: duyệt theo dự án: %w", err)
	}
	if len(out) > TranPhanBoMotDuAn {
		return nil, ErrQuaNhieuPhanBo
	}
	return out, nil
}

// disbursedByMonth buckets the live vouchers of ONE budget year's live projects by the calendar month
// of `ngay_chi` — §4's year curve, and §8.3's project curve when `$3` names a project.
//
// THE TWO EDGES ARE domain.DisbursedByMonth's RULE, expressed in SQL as buckets: a payment dated before
// 01/01 of the year lands in bucket 1 (January), one dated after 31/12 in bucket 13 (AfterYear). The
// statement only BUCKETS; the meaning of each bucket is decided once, in domain.
//
// THE VOUCHER SET IS tongChungTu's: every state counts, soft-deleted vouchers do not — so the curve and
// "đã giải ngân" total the same rows. A soft-deleted PROJECT's vouchers are excluded too, as DanhSach
// excludes the project.
//
// ONE STATEMENT, AT MOST 13 ROWS, whatever the number of projects or vouchers (no read per project).
//
// tenant_id = $1 ON BOTH TABLES, for the reason tongChungTu gives (rule 1).
const disbursedByMonth = `SELECT CASE
	         WHEN ct.ngay_chi < make_date($2::int, 1, 1) THEN 1
	         WHEN ct.ngay_chi >= make_date($2::int + 1, 1, 1) THEN 13
	         ELSE EXTRACT(MONTH FROM ct.ngay_chi)::int
	       END AS bucket,
	       SUM(ct.so_tien)::bigint
	  FROM chung_tu_giai_ngan ct
	  JOIN du_an da
	    ON da.tenant_id = ct.tenant_id AND da.tenant_id = $1 AND da.id = ct.du_an_id
	 WHERE ct.tenant_id = $1 AND ct.deleted_at IS NULL
	   AND da.deleted_at IS NULL AND da.nam = $2::int
	   AND ($3 = '' OR ct.du_an_id = $3)
	 GROUP BY 1`

// DisbursedByMonth reads the month buckets of one budget year — the whole commune when projectID is
// empty, one project otherwise. The caller of the project form has already answered 404 for a project
// not in this commune (ChiTiet); here such an id simply matches no row.
func (s *DuAnStore) DisbursedByMonth(ctx context.Context, year int, projectID string) (domain.DisbursedByMonth, error) {
	var out domain.DisbursedByMonth
	if year == 0 {
		return out, ErrThieuNamNganSach
	}
	rows, err := s.db.For(ctx).QueryJoin(ctx, disbursedByMonth, year, projectID)
	if err != nil {
		return out, fmt.Errorf("chung_tu_giai_ngan: đọc theo tháng: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			bucket int
			amount int64
		)
		if err := rows.Scan(&bucket, &amount); err != nil {
			return out, fmt.Errorf("chung_tu_giai_ngan: đọc dòng theo tháng: %w", err)
		}
		switch {
		case bucket >= 1 && bucket <= 12:
			out.Months[bucket-1] = domain.Dong(amount)
		case bucket == 13:
			out.AfterYear = domain.Dong(amount)
		default:
			// Unreachable by the CASE above. Refused rather than dropped: a bucket nobody adds is money
			// missing from the curve with nothing saying so.
			return domain.DisbursedByMonth{}, fmt.Errorf("chung_tu_giai_ngan: ô tháng lạ %d", bucket)
		}
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("chung_tu_giai_ngan: duyệt theo tháng: %w", err)
	}
	return out, nil
}
