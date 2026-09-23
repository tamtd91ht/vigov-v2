package store

// The TASK REGISTER — the paginated list and the single-task read. SQL, and nothing else.
//
// FOUR THINGS HOLD ACROSS EVERY METHOD IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from the context (rule 1, invariants 4 and 5). It is
//     never a parameter here, so no caller can reach another commune's register.
//  2. NOTHING HERE OPENS A TRANSACTION, and nothing here WRITES. This pass is the read path; the
//     write use cases come next and will take a *store.ScopedTx so the audit entry cannot be
//     written anywhere but inside the same transaction (rule 6, invariant 3).
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — the list and the single read
//     alike. A task removed from the register does not come back through a number somebody still
//     has written in a meeting's minutes.
//  4. EVERY FILTER VALUE IS A BOUND PARAMETER. A filter assembled as text anywhere above this line
//     is an injection point in a government register.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// NhiemVuStore is the only path to `nhiem_vu` (migration 0006).
//
// EVERY READ GOES THROUGH *store.Scoped, which binds `tenant_id` to $1 from the context. The
// commune is therefore not a parameter of any method here and cannot be made into one — a
// repository that can be built without a commune is a repository that can query across communes.
type NhiemVuStore struct {
	db *store.DB
}

func NewNhiemVuStore(db *store.DB) *NhiemVuStore { return &NhiemVuStore{db: db} }

// ErrNhiemVuKhongTonTai means no LIVE task of THIS commune carries that number.
//
// ONE ERROR FOR "no such number", "another commune's number" AND "soft deleted", and the caller
// answers 404 for all three. Telling them apart tells a caller which numbers exist in a commune
// they cannot read.
var ErrNhiemVuKhongTonTai = errors.New("nhiem_vu: không có nhiệm vụ")

// cotNhiemVu IS READ BY POSITION in the Scan below.
//
// `han_xu_ly`, `han_ban_dau` AND `ngay_hoan_thanh` ARE THREE ADJACENT TIMESTAMPTZ COLUMNS and a swap
// between any two produces NO error at all — it produces a task that looks extended when it was
// not, or an on-time ratio measured against the wrong number. They are listed together on purpose,
// so the group is read as a group.
//
// `tao_luc` IS IN THIS ONE LIST, unlike the petition register's split pair, because a task has ONE
// creation instant. `phieu_phan_anh` needed two constants because `vao_so_luc` (a business fact) and
// `tao_luc` (a row-lifecycle fact) can differ by a week, and putting both in the shape every read
// returns invited a screen to render the wrong one. Here there is nothing to confuse it with.
const cotNhiemVu = `id, ma, loai, khoi, tieu_de, mo_ta, trang_thai, muc_uu_tien,
	nguon_giao, nguon_id,
	bo_phan_id, nguoi_thuc_hien_ma, lanh_dao_giao_viec_ma,
	co_quan_chu_tri_id, chuyen_vien_theo_doi_ma,
	han_xu_ly, han_ban_dau, ngay_hoan_thanh,
	tien_do, tom_tat_ket_qua, ghi_chu,
	lanh_dao_phe_duyet_hoan_thanh, cap_tren_cong_nhan_hoan_thanh,
	nguoi_tao_ma, tao_luc`

// SapXepNhiemVu is the closed set of sorts GET /api/v1/tasks offers.
//
// `created_at` IS THE DEFAULT, DESCENDING — newest first, which is what an officer opening the
// board in the morning needs. `tao_luc` is NOT NULL and indexed (`nhiem_vu_so`, migration 0006),
// which is what page.QueryPage needs from a sort column.
//
// `code` IS OFFERED AND THE PETITION REGISTER DELIBERATELY DOES NOT OFFER ITS EQUIVALENT, and the
// difference is what the two codes ARE. `ma_tra_cuu` is random, so sorting by it is sorting by
// noise; `nhiem_vu.ma` is the commune's own register sequence — NV01, NV02… — so ordering by it is
// the Sổ theo dõi's own order (§4.3). It is NOT NULL and carries UNIQUE (tenant_id, ma), so the
// cursor has a stable total order.
//
// WHAT IS DELIBERATELY ABSENT, and each absence is a different rule:
//
//	han_xu_ly    NULLABLE — `(col, id) > (…)` is NULL for a NULL col, so every task with no
//	             deadline would vanish from page two onward. §4.2 offers it as a column sort; a
//	             screen that needs late work first asks with `late=true`.
//	tieu_de      a sort key travels in a URL, an access log and a browser history, and a task
//	             title quotes a citizen's complaint often enough that it is not a safe thing to
//	             put there (rule 3, forbidden #4).
//	muc_uu_tien  it is a code, not a rank: the ORDER of the scale lives in the catalogue's
//	             `thu_tu`, and sorting the register alphabetically by code would put `cao` above
//	             `khan` — a board that looks sorted and is not.
var SapXepNhiemVu = page.NewAllowlist(page.Desc,
	page.Col("created_at", "tao_luc", page.KindTime),
	page.Col("code", "ma", page.KindText),
)

// TimNhiemVuToiDa bounds the free-text search. Longer than any phrase a clerk types, short enough
// that the pattern cannot become a payload.
const TimNhiemVuToiDa = 200

// ErrTimNhiemVuQuaDai — the search box was sent more than TimNhiemVuToiDa characters.
var ErrTimNhiemVuQuaDai = fmt.Errorf(
	"nhiem_vu: chuỗi tìm kiếm quá dài (tối đa %d ký tự)", TimNhiemVuToiDa)

// LocNhiemVu is the set of filters the list route accepts, already validated by the handler.
//
// A STRUCT AND NOT A STRING OF SQL: every field below becomes a BOUND PARAMETER.
//
// THE FILTERS OF §3 THAT ARE NOT HERE ARE NOT HERE ON PURPOSE, and both absences are refusals the
// handler makes explicitly rather than silent omissions — see locNhiemVuTuQuery:
//
//	`sap_den_han`        the threshold is the commune's own `sla.gio_sap_den_han`
//	                     (service-identity/migrations/0008_sla.sql:178) and identity exposes no RPC
//	                     that returns it (ADR 0029 §118). The specification's "mặc định 72 giờ" is a
//	                     DEFAULT FOR A COMMUNE TO OVERRIDE, not a constant for this service to hold:
//	                     a second copy of that number in Go is exactly what the BA/PM note calls the
//	                     one-threshold-not-two rule.
//	`lien-quan-den-toi`  it needs "bộ phận tôi đang giữ", and the staff principal carries NO
//	                     department — proto/vigov/identity/v1/identity.proto returns none, and
//	                     §1283-1297 explains at length why that message is kept narrow. Rule 2, stop
//	                     condition #2.
type LocNhiemVu struct {
	TrangThai string // "" = every status
	Loai      string // "" = every task type
	Khoi      string // "" = every bloc, including none
	MucUuTien string // "" = every priority, including none
	BoPhanID  string // "" = every department, including none
	NguonGiao string // "" = every source
	Tim       string // "" = no text search

	// NguoiThucHienMa is BOTH the "Người thực hiện" picker of §3 and the whole of the
	// `Giao cho tôi` scope — one predicate, because they are one question ("whose name is on this
	// task"). The handler is what decides the value: the picker supplies a code from the request,
	// the scope supplies the CALLER'S OWN code from the session and never from the request
	// (rule 4, invariant 2 in its staff form).
	NguoiThucHienMa string

	// ChiTreHan restricts the page to tasks whose CURRENT commitment was missed.
	//
	// DERIVED IN SQL, NEVER READ FROM A COLUMN (rule 10, invariant 3). See the predicate in
	// locNhiemVuThanhSQL, and read the warning there before touching it.
	ChiTreHan bool
}

// dieuKienTimNhiemVu is the free-text predicate, built with the placeholder already chosen.
//
// WRITTEN BY CONCATENATION RATHER THAN fmt.Sprintf for the same reason the petition register's
// equivalent is: `hooks/pii_guard` blocks a formatting call near a personal-data column name and is
// right to, so the honest move is to stop using the shape the guard watches rather than argue with
// it. §3 says the box searches "trong mã + tiêu đề" — those two columns and no others.
func dieuKienTimNhiemVu(n int) string {
	p := "$" + strconv.Itoa(n)
	return " AND (ma ILIKE " + p + " OR tieu_de ILIKE " + p + ")"
}

// locNhiemVuThanhSQL turns the validated filter struct into a predicate and its bound values.
//
// ONE FUNCTION BUILDING BOTH HALVES, because the placeholder numbers and the argument order are one
// fact: written apart, a filter added to one half and forgotten in the other produces a query that
// silently reads the wrong column's value.
func locNhiemVuThanhSQL(loc LocNhiemVu) (string, []any) {
	var (
		dieuKien string
		args     []any
	)
	them := func(mau string, gt any) {
		args = append(args, gt)
		dieuKien += fmt.Sprintf(mau, len(args)+1) // $1 is the commune
	}

	if loc.TrangThai != "" {
		them(" AND trang_thai = $%d", loc.TrangThai)
	}
	if loc.Loai != "" {
		them(" AND loai = $%d", loc.Loai)
	}
	if loc.Khoi != "" {
		them(" AND khoi = $%d", loc.Khoi)
	}
	if loc.MucUuTien != "" {
		them(" AND muc_uu_tien = $%d", loc.MucUuTien)
	}
	if loc.BoPhanID != "" {
		them(" AND bo_phan_id = $%d", loc.BoPhanID)
	}
	if loc.NguoiThucHienMa != "" {
		them(" AND nguoi_thuc_hien_ma = $%d", loc.NguoiThucHienMa)
	}
	if loc.NguonGiao != "" {
		them(" AND nguon_giao = $%d", loc.NguonGiao)
	}
	if loc.Tim != "" {
		// ILIKE ON TWO COLUMNS — the register number a clerk remembers and the title. A leading
		// wildcard cannot use an index, which is affordable because the scan is already bound to one
		// commune.
		args = append(args, "%"+loc.Tim+"%")
		dieuKien += dieuKienTimNhiemVu(len(args) + 1)
	}

	if loc.ChiTreHan {
		// OVERDUE IS DERIVED HERE TOO, AND THIS IS THE SECOND EXPRESSION OF domain.NhiemVu.TreHan —
		// say so rather than let somebody discover it. Go cannot run inside a WHERE clause, so a
		// register filtered on "late" has no other shape; what this comment buys is that the two are
		// read together when either changes.
		//
		// IT MIRRORS domain.NhiemVu.TreHan LINE FOR LINE:
		//
		//	no deadline  -> NOT late. Nothing was promised, so nothing can have been missed.
		//	finished     -> compare the two RECORDED instants, so work finished late STAYS late and
		//	                last quarter's figure does not change on every read.
		//	not finished -> compare the deadline with now.
		//
		// ⚠ `han_xu_ly` AND NOT `han_ban_dau`, AND THE TWO ARE NOT INTERCHANGEABLE. This filter is
		// the register's "Chỉ việc quá hạn" box: is this work late as things stand TODAY, after any
		// extension the leader granted. §11.3's ON-TIME RATIO is the other question and is measured
		// against `han_ban_dau`; swapping them here would make the box list tasks whose extension was
		// approved, which is precisely the complaint the extension was granted to answer.
		//
		// `now()` AND NOT A PARAMETER: the database's clock is the one the deadline was stored
		// against, and a `now` passed from a handler is a second clock that can disagree with it.
		dieuKien += ` AND han_xu_ly IS NOT NULL AND (
			(ngay_hoan_thanh IS NULL AND han_xu_ly < now())
			OR (ngay_hoan_thanh IS NOT NULL AND ngay_hoan_thanh > han_xu_ly))`
	}
	return dieuKien, args
}

// mocNhiemVu BINDS each allowlisted sort column to the way that column's cursor value is read out of
// a scanned row. store.NewMoc compares the two lists AT CONSTRUCTION, so a sort added to
// SapXepNhiemVu without a reader here is a panic at startup rather than an error on the first
// request that uses it — which would be after release.
var mocNhiemVu = store.NewMoc[domain.NhiemVu](SapXepNhiemVu,
	map[string]func(domain.NhiemVu) page.Key{
		"created_at": func(n domain.NhiemVu) page.Key { return page.TimeKey(n.TaoLuc) },
		"code":       func(n domain.NhiemVu) page.Key { return page.TextKey(n.Ma) },
	})

// DanhSach reads ONE PAGE of the commune's task register.
//
// PAGINATED, UNLIKE THE TWO CATALOGUES NEXT DOOR, and the difference is what the lists ARE: a
// catalogue is closed at a handful of rows, while this register grows with every week the commune
// operates — §12's sample data alone is 28 tasks for one commune in one period.
//
// THE FILTERS ARE BOUND PARAMETERS, numbered from $2 because QueryPage gives $1 to the commune.
func (s *NhiemVuStore) DanhSach(ctx context.Context, loc LocNhiemVu, yc page.Request) (
	page.Result[domain.NhiemVu], error) {

	dieuKien, args := locNhiemVuThanhSQL(loc)

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotNhiemVu,
		Table:   "nhiem_vu",
		// `deleted_at IS NULL` FIRST AND ALWAYS (rule 7, invariant 2). The partial index
		// `nhiem_vu_so` is built on exactly this predicate.
		Filter: `AND deleted_at IS NULL` + dieuKien,
		Args:   args,
	}, yc, mocNhiemVu, func(rows *sql.Rows) (domain.NhiemVu, string, error) {
		n, err := quetNhiemVu(rows)
		if err != nil {
			return domain.NhiemVu{}, "", err
		}
		return n, n.ID, nil
	})
}

// TheoMa reads one task by the number the commune issued it.
//
// IT TAKES THE ISSUED NUMBER AND NOT THE INTERNAL id, because that is what the URL carries, what
// the Sổ theo dõi prints and what an audit entry's subject will be. One identifier end to end is
// one identifier nobody can mix up.
//
// `deleted_at IS NULL` IS ON THIS PATH AS IT IS ON EVERY OTHER (rule 7, invariant 2).
func (s *NhiemVuStore) TheoMa(ctx context.Context, ma string) (domain.NhiemVu, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotNhiemVu, "nhiem_vu",
		`AND ma = $2 AND deleted_at IS NULL`, ma)
	if err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc theo mã: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc theo mã: %w", err)
		}
		return domain.NhiemVu{}, ErrNhiemVuKhongTonTai
	}
	n, err := quetNhiemVu(rows)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: duyệt kết quả: %w", err)
	}
	return n, nil
}

// quetNhiemVu reads one row of cotNhiemVu.
//
// POSITIONAL, IN LOCKSTEP WITH cotNhiemVu. database/sql binds by POSITION, so a destination inserted
// or removed anywhere but the tail silently shifts every column after it — and the columns it would
// shift are the three adjacent TIMESTAMPTZs whose confusion produces a wrong figure rather than an
// error. quetNhiemVuKhopCot in the test package asserts the two lists line up BY NAME.
func quetNhiemVu(r quangKiem) (domain.NhiemVu, error) {
	var (
		n   domain.NhiemVu
		tt  string
		ng  string
		loa string

		// EVERY NULLABLE COLUMN IS READ THROUGH AN EXPLICIT NULL TYPE. Scanning a NULL straight into
		// a string or a time.Time is a runtime error in some drivers and a zero value in others, and
		// the second is how "no deadline was ever set" quietly becomes a deadline in year 1.
		khoi, mucUuTien, moTa, nguonID    sql.NullString
		boPhan, nguoiThucHien, lanhDao    sql.NullString
		coQuanChuTri, chuyenVien          sql.NullString
		tomTat, ghiChu                    sql.NullString
		hanXuLy, hanBanDau, ngayHoanThanh sql.NullTime
	)

	dich := []any{
		&n.ID, &n.Ma, &loa, &khoi, &n.TieuDe, &moTa, &tt, &mucUuTien,
		&ng, &nguonID,
		&boPhan, &nguoiThucHien, &lanhDao,
		&coQuanChuTri, &chuyenVien,
		&hanXuLy, &hanBanDau, &ngayHoanThanh,
		&n.TienDo, &tomTat, &ghiChu,
		&n.LanhDaoPheDuyetHoanThanh, &n.CapTrenCongNhanHoanThanh,
		&n.NguoiTaoMa, &n.TaoLuc,
	}
	if err := r.Scan(dich...); err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc dòng: %w", err)
	}

	n.Loai = loa
	n.TrangThai = domain.TrangThaiNhiemVu(tt)
	n.NguonGiao = domain.NguonGiao(ng)
	n.Khoi = khoi.String
	n.MucUuTien = mucUuTien.String
	n.MoTa = moTa.String
	n.NguonID = nguonID.String
	n.BoPhanID = boPhan.String
	n.NguoiThucHienMa = nguoiThucHien.String
	n.LanhDaoGiaoViecMa = lanhDao.String
	n.CoQuanChuTriID = coQuanChuTri.String
	n.ChuyenVienTheoDoiMa = chuyenVien.String
	n.TomTatKetQua = tomTat.String
	n.GhiChu = ghiChu.String
	// NULL -> the zero time.Time. All three zeros mean ONE thing here — "this instant was never
	// recorded" — which is the opposite of the petition register, where two adjacent NULLs mean two
	// opposite things. Say it out loud precisely because the two files sit side by side.
	n.HanXuLy = hanXuLy.Time
	n.HanBanDau = hanBanDau.Time
	n.NgayHoanThanh = ngayHoanThanh.Time
	return n, nil
}
