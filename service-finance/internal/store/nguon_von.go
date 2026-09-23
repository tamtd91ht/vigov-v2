package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// NguonVonStore reads a commune's funding sources, the allocation of projects across them, and the
// figures §6's cards are drawn from.
//
// It holds *store.DB and never a *sql.DB. The commune is taken from the context on every call
// through db.For(ctx), so there is no constructor, no field and no method here that could produce a
// query without one (rule 1, invariant 5).
//
// READ-ONLY, ON PURPOSE. Migration 0007 leaves two unique keys undeclared because they are open
// questions for the customer (see its header), and both stay free to answer only while no row
// exists. A write path in this slice would close that window before the question was asked.
type NguonVonStore struct {
	db *store.DB
}

func NewNguonVonStore(db *store.DB) *NguonVonStore { return &NguonVonStore{db: db} }

// TranNguonVonMotNam is the hard upper bound on one commune's funding sources in one budget year.
//
// WHY A BOUND AT ALL: one process serves 200+ communes, so every list route is a shared resource
// and an unbounded one is forbidden. §6 shows the whole set at once — one card per source — so the
// bound cannot come from a `limit` parameter; it has to be a ceiling the data is checked against.
//
// 200 IS FIFTY TIMES THE REAL FIGURE. §6's sample commune carries four sources. The number is not
// an estimate of how many there might be; it is the point past which the data is no longer a
// commune's funding plan: an import run twice, a test fixture on a live database.
const TranNguonVonMotNam = 200

// TranPhanBoMotDuAn bounds one project's allocation lines.
//
// THE SAME CEILING, and the reason is not laziness: a project cannot sensibly draw on more sources
// than the commune has declared for the year. It is a separate constant because the two can diverge
// — migration 0007 does NOT declare `UNIQUE (tenant_id, du_an_id, nguon_von_id)` (an open question),
// so a project may hold several lines naming one source, and a ceiling shared by name would then be
// wrong for a reason nobody could see.
const TranPhanBoMotDuAn = 200

// ErrQuaNhieuNguonVon and ErrQuaNhieuPhanBo say a ceiling was reached. The caller answers 500 and
// refuses.
//
// REFUSE RATHER THAN TRUNCATE, and here the cost of the alternative is money on a screen: these
// rows are totalled into "đã phân bổ" per source (§6) and into the `Đủ · N nguồn` chip per project
// (§11). A silently short list produces a source that looks under-committed and a project that
// looks under-funded — both entirely normal-looking, both reported upward. A refusal breaks one
// commune's screen loudly and names itself in the log (fail closed).
var (
	ErrQuaNhieuNguonVon = errors.New("nguon_von: vượt trần số nguồn vốn một năm")
	ErrQuaNhieuPhanBo   = errors.New("phan_bo_nguon_von: vượt trần số dòng phân bổ một dự án")
)

// cotNguonVon IS READ BY POSITION in the scans below. `nam` and `thu_tu` are two adjacent INTs and
// a swap between them is invisible — the list still renders, ordered by a year and filtered by a
// rank — which is why this is a constant rather than a column list written inline at each call site.
const cotNguonVon = `id, ten, nam, thu_tu, tong_nguon`

// DanhSach reads one commune's funding sources for one budget year, in the commune's own order (§6).
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the sources of the commune the request arrived in (rule 1).
//
// ORDER BY thu_tu, ten, id: `thu_tu` is the commune's own arrangement and `ten` breaks ties. `id`
// is appended because NEITHER of the first two is unique — migration 0007 leaves the uniqueness of
// a source name open — so without it two rows sharing a rank and a name could swap places between
// two calls. An unstable order makes a client-side diff flicker on every reload and makes any test
// of this compare sets by accident.
func (s *NguonVonStore) DanhSach(ctx context.Context, nam int) ([]domain.NguonVon, error) {
	// ErrThieuNamNganSach is shared with DuAnStore, and deliberately: there is no "all years" read
	// here either. §13 rule 8 makes each budget year its own set, and a list mixing two years would
	// total one commune's funding twice on the same screen. A default to the current year would be a
	// wrong report nobody can see.
	if nam == 0 {
		return nil, ErrThieuNamNganSach
	}

	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling returns a full page indistinguishable from a complete list of
	// exactly that size — the truncation this read refuses to perform, performed by the bound meant
	// to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotNguonVon, "nguon_von",
		`AND nam = $2 AND deleted_at IS NULL ORDER BY thu_tu, ten, id LIMIT $3`,
		nam, TranNguonVonMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("nguon_von: đọc danh sách: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.NguonVon, 0, 8)
	for rows.Next() {
		var nv domain.NguonVon
		var tong int64
		// POSITIONAL — in lockstep with cotNguonVon.
		if err := rows.Scan(&nv.ID, &nv.Ten, &nv.Nam, &nv.ThuTu, &tong); err != nil {
			return nil, fmt.Errorf("nguon_von: đọc dòng: %w", err)
		}
		nv.TongNguon = domain.Dong(tong)
		ra = append(ra, nv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nguon_von: duyệt kết quả: %w", err)
	}
	if len(ra) > TranNguonVonMotNam {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might total anyway is how a refusal turns back into a silent truncation one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuNguonVon
	}
	return ra, nil
}

// PhanBoTheoDuAn reads one project's funding allocation lines (§9, §11).
//
// NO YEAR FILTER AND THERE MUST NOT BE ONE: an allocation line belongs to a PROJECT, and the
// project already belongs to exactly one budget year (0004, `du_an.nam`). Filtering by year here
// would be a second, independent copy of that fact — and the copy would be the one that is wrong on
// the day a line is entered against the wrong year.
//
// THE CALLER TURNS THIS INTO THE CHIP with domain.GanNguon, which is where §11's rule lives. This
// method deliberately does not compute `Đủ`/`Chưa đủ` in SQL: that would be the rule written a
// second time, in a second language, and the two would drift.
func (s *NguonVonStore) PhanBoTheoDuAn(ctx context.Context, duAnID string) ([]domain.PhanBoNguonVon, error) {
	rows, err := s.db.For(ctx).Query(ctx, `id, du_an_id, nguon_von_id, so_tien_phan_bo`,
		"phan_bo_nguon_von",
		`AND du_an_id = $2 AND deleted_at IS NULL ORDER BY nguon_von_id, id LIMIT $3`,
		duAnID, TranPhanBoMotDuAn+1)
	if err != nil {
		return nil, fmt.Errorf("phan_bo_nguon_von: đọc theo dự án: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.PhanBoNguonVon, 0, 4)
	for rows.Next() {
		var pb domain.PhanBoNguonVon
		var soTien int64
		if err := rows.Scan(&pb.ID, &pb.DuAnID, &pb.NguonVonID, &soTien); err != nil {
			return nil, fmt.Errorf("phan_bo_nguon_von: đọc dòng: %w", err)
		}
		pb.SoTien = domain.Dong(soTien)
		ra = append(ra, pb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phan_bo_nguon_von: duyệt kết quả: %w", err)
	}
	if len(ra) > TranPhanBoMotDuAn {
		return nil, ErrQuaNhieuPhanBo
	}
	return ra, nil
}

// tongPhanBoTheoNguon is the subquery behind "đã phân bổ" and "Số dự án" on §6's card.
//
// COUNT(DISTINCT du_an_id), NOT COUNT(*). Migration 0007 leaves `UNIQUE (tenant_id, du_an_id,
// nguon_von_id)` undeclared while the question is with the customer, so two lines may name one
// source for one project. Counting rows would then report a source as funding three projects when
// it funds two — a number on a card that nothing else on the screen contradicts.
//
// tenant_id = $1 IS INSIDE THE SUBQUERY AS WELL AS IN THE ON CLAUSE, and both are load-bearing: the
// join matches on (tenant_id, nguon_von_id), so a subquery unconstrained by commune would still be
// correct — until somebody simplifies the ON clause to `nguon_von_id` alone, at which point two
// communes whose source ids collide would total each other's money. Constrained in both places,
// that edit cannot produce a leak (rule 1).
const tongPhanBoTheoNguon = `(SELECT pb.tenant_id, pb.nguon_von_id,
	          SUM(pb.so_tien_phan_bo) AS da_phan_bo,
	          COUNT(DISTINCT pb.du_an_id) AS so_du_an
	     FROM phan_bo_nguon_von pb
	    WHERE pb.tenant_id = $1 AND pb.deleted_at IS NULL
	    GROUP BY pb.tenant_id, pb.nguon_von_id)`

// tongChungTuTheoNguon is "đã giải ngân" per source — the numerator of two of §6's three bars.
//
// EVERY STATE COUNTS, INCLUDING `ke-toan-nhap`, exactly as tongChungTu does for a project (§11
// defines the total as the sum of vouchers "WHERE trạng thái ≥ kế toán nhập", and that is the FIRST
// state). Written out because a reader meeting `trang_thai` assumes the total waits for
// confirmation; it does not, and adding `AND trang_thai <> 'ke-toan-nhap'` here would report every
// source as further behind than it is.
//
// `nguon_von_id IS NOT NULL` IS NOT A FILTER OF CONVENIENCE. Vouchers with no source are §13 rule
// 6's case — they count toward the project's and the commune's disbursed total and are reported
// separately as "đã chi nhưng chưa ghi rút từ nguồn nào". They belong to NO card here, and the
// GROUP BY would otherwise produce a NULL-keyed group that joins to nothing and disappears
// silently. The predicate says so out loud instead. That warning total is a read this slice
// deliberately does not build.
//
// SOFT-DELETED VOUCHERS ARE EXCLUDED, everywhere and always (rule 7, invariant 2).
const tongChungTuTheoNguon = `(SELECT ct.tenant_id, ct.nguon_von_id, SUM(ct.so_tien) AS da_giai_ngan
	     FROM chung_tu_giai_ngan ct
	    WHERE ct.tenant_id = $1 AND ct.deleted_at IS NULL AND ct.nguon_von_id IS NOT NULL
	    GROUP BY ct.tenant_id, ct.nguon_von_id)`

// TienDoTheoNguon reads §6's block: every funding source of one budget year with the three figures
// its card is drawn from.
//
// THE RATIOS ARE NOT COMPUTED IN SQL. This returns amounts and counts; domain.TienDoNguonVon turns
// them into the three bars, including the case where a bar HAS NO VALUE because its denominator is
// zero (§6's "Chương trình mục tiêu quốc gia, đã phân bổ 0 đ"). A ratio computed here would have to
// choose a number for that case, and every available number is a lie about a commune's work.
//
// A SOURCE WITH NO ALLOCATION AND NO VOUCHER STILL APPEARS, which is why both joins are LEFT. §6's
// sample table lists exactly such a row; dropping it would make a source the commune declared
// vanish from the screen that exists to show the commune's sources.
func (s *NguonVonStore) TienDoTheoNguon(ctx context.Context, nam int) ([]domain.TienDoNguonVon, error) {
	if nam == 0 {
		return nil, ErrThieuNamNganSach
	}

	stmt := `SELECT nv.id, nv.ten, nv.nam, nv.thu_tu, nv.tong_nguon,
			COALESCE(pb.da_phan_bo, 0), COALESCE(pb.so_du_an, 0), COALESCE(ct.da_giai_ngan, 0)
		FROM nguon_von nv
		LEFT JOIN ` + tongPhanBoTheoNguon + ` pb
		       ON pb.tenant_id = nv.tenant_id AND pb.nguon_von_id = nv.id
		LEFT JOIN ` + tongChungTuTheoNguon + ` ct
		       ON ct.tenant_id = nv.tenant_id AND ct.nguon_von_id = nv.id
		WHERE nv.tenant_id = $1 AND nv.deleted_at IS NULL AND nv.nam = $2
		ORDER BY nv.thu_tu, nv.ten, nv.id
		LIMIT $3`

	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, nam, TranNguonVonMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("nguon_von: đọc tiến độ theo nguồn: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.TienDoNguonVon, 0, 8)
	for rows.Next() {
		var (
			t                          domain.TienDoNguonVon
			tong, daPhanBo, daGiaiNgan int64
			soDuAn                     int
		)
		if err := rows.Scan(&t.NguonVon.ID, &t.NguonVon.Ten, &t.NguonVon.Nam, &t.NguonVon.ThuTu,
			&tong, &daPhanBo, &soDuAn, &daGiaiNgan); err != nil {
			return nil, fmt.Errorf("nguon_von: đọc dòng tiến độ: %w", err)
		}
		t.NguonVon.TongNguon = domain.Dong(tong)
		t.DaPhanBo = domain.Dong(daPhanBo)
		t.DaGiaiNgan = domain.Dong(daGiaiNgan)
		t.SoDuAn = soDuAn
		ra = append(ra, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nguon_von: duyệt kết quả tiến độ: %w", err)
	}
	if len(ra) > TranNguonVonMotNam {
		return nil, ErrQuaNhieuNguonVon
	}
	return ra, nil
}
