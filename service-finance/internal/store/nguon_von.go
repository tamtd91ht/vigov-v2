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
// READ-ONLY. Migration 0013 settled the two unique keys 0007 left open (user decision 06/10/2026),
// so a write path is no longer blocked on the customer; it is TASK-02's, and it does not live here.
type NguonVonStore struct {
	db *store.DB
}

func NewNguonVonStore(db *store.DB) *NguonVonStore { return &NguonVonStore{db: db} }

// TranNguonVonMotNam is the hard upper bound on one commune's funding sources.
//
// THE NAME SAYS "one year" AND THE BOUND IS NOW PER COMMUNE. Since 0013 a source is declared once
// and shared by every year, so the catalogue — not a year's slice of it — is what is counted. The
// identifier is kept (rule 12, invariant 3: existing names are not renamed); this line is the
// correction.
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
// THE SAME CEILING, and the reason is not laziness: since 0013 declared `UNIQUE (tenant_id, du_an_id,
// nguon_von_id)`, a project holds at most one line per source, so it cannot hold more lines than
// the commune has sources. It stays a separate constant because it bounds a different read, and a
// ceiling shared by name would silently move both the day one of them has to change.
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
	ErrQuaNhieuNguonVon = errors.New("nguon_von: vượt trần số nguồn vốn của một xã")
	ErrQuaNhieuPhanBo   = errors.New("phan_bo_nguon_von: vượt trần số dòng phân bổ một dự án")
)

// cotNguonVon IS READ BY POSITION in the scans below — the catalogue row (`nv`, nguon_von) and the
// amount granted for the year asked (`fa`, funding_source_annual_amounts, migration 0013). It is a
// constant rather than a column list written inline at each call site because the two reads below
// must scan the same four values in the same order.
//
// COALESCE(fa.granted_amount, 0) IS THE DECIDED MEANING, not a convenience: a source with no row for
// that year has nothing granted that year (0013's note on the table). The source still appears —
// the catalogue is the commune's list of sources, and hiding one because this year's figure has not
// been typed yet would make a declared source vanish from §6.
const cotNguonVon = `nv.id, nv.ten, nv.thu_tu, COALESCE(fa.granted_amount, 0)`

// joinSoTienNam attaches the granted amount of ONE year to each catalogue row.
//
// LEFT, so a source with no amount for the year stays (see cotNguonVon). `fa.tenant_id = $1` IS IN
// THE ON CLAUSE AS WELL AS `fa.tenant_id = nv.tenant_id`, and both are load-bearing — QueryJoin's
// contract (core/store/scoped.go): simplify the second to an id match and two communes whose source
// ids collide would read each other's amounts; the first keeps that edit from leaking (rule 1).
//
// `fa.year = $2` IS THE WHOLE OF DECISION 2: one figure per year, and only the year asked for. Read
// without it, a source with amounts in two years would produce two rows on one card.
const joinSoTienNam = `LEFT JOIN funding_source_annual_amounts fa
		       ON fa.tenant_id = nv.tenant_id AND fa.tenant_id = $1
		      AND fa.funding_source_id = nv.id AND fa.year = $2`

// DanhSach reads one commune's funding sources, each with the amount granted for one budget year,
// in the commune's own order (§6).
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: QueryJoin binds it to $1 from the context, so
// this can only ever read the sources of the commune the request arrived in (rule 1).
//
// EVERY LIVE SOURCE OF THE CATALOGUE IS RETURNED, whatever the year — since 0013 a source has no
// year; only its granted amount does. `Nam` on each row is the year that amount was read for.
//
// ORDER BY thu_tu, ten, id: `thu_tu` is the commune's own arrangement and `ten` breaks ties. Since
// 0013 `ten` is unique within a commune, so `id` can no longer decide anything; it stays as the last
// key so the order does not quietly depend on a constraint a later migration might loosen.
func (s *NguonVonStore) DanhSach(ctx context.Context, nam int) ([]domain.NguonVon, error) {
	// ErrThieuNamNganSach is shared with DuAnStore, and deliberately: there is no "all years" read
	// here either. The granted amount is per year, and a default to the current year would be a
	// wrong report nobody can see.
	if nam == 0 {
		return nil, ErrThieuNamNganSach
	}

	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling returns a full page indistinguishable from a complete list of
	// exactly that size — the truncation this read refuses to perform, performed by the bound meant
	// to prevent it.
	stmt := `SELECT ` + cotNguonVon + `
		FROM nguon_von nv
		` + joinSoTienNam + `
		WHERE nv.tenant_id = $1 AND nv.deleted_at IS NULL
		ORDER BY nv.thu_tu, nv.ten, nv.id
		LIMIT $3`

	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, nam, TranNguonVonMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("nguon_von: đọc danh sách: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.NguonVon, 0, 8)
	for rows.Next() {
		nv := domain.NguonVon{Nam: nam}
		var tong int64
		// POSITIONAL — in lockstep with cotNguonVon.
		if err := rows.Scan(&nv.ID, &nv.Ten, &nv.ThuTu, &tong); err != nil {
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
// the day a line is entered against the wrong year. 0013 did not change this: what moved to a
// per-year table is the source's GRANTED amount, which this read does not touch.
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

// tongPhanBoTheoNguon is the subquery behind "đã phân bổ" and "Số dự án" on §6's card, FOR ONE YEAR.
//
// THE YEAR COMES FROM THE PROJECT, `da.nam = $2`, and since 0013 that join is the only thing scoping
// this figure to a year at all. Under 0007 a source belonged to one year, so its lines were that
// year's lines by construction; now one source serves every year, and without the join a 2027
// project's allocation would land on the 2026 card — a card whose numbers all still look plausible.
// The project's year and not anything on the line: `du_an.nam` is the one home of that fact and is
// immutable (store/du_an_ghi.go, capNhatDuAn).
//
// `da.deleted_at IS NULL` beside `pb.deleted_at IS NULL`: lines are soft deleted with their project
// (xoaMemPhanBoCuaDuAn), so this is redundant today — and it is what keeps a removed project's plan
// off the card should a line ever outlive its project (rule 7, invariant 2).
//
// COUNT(DISTINCT du_an_id), NOT COUNT(*). Since 0013 `UNIQUE (tenant_id, du_an_id, nguon_von_id)`
// makes the two equal; DISTINCT stays because it costs nothing and does not depend on that key.
//
// tenant_id = $1 IS BOUND ON EVERY TABLE, INSIDE THE SUBQUERY AS WELL AS IN THE OUTER ON CLAUSE, and
// each is load-bearing: simplify any join to an id match and two communes whose ids collide would
// total each other's money. Constrained everywhere, that edit cannot produce a leak (rule 1).
const tongPhanBoTheoNguon = `(SELECT pb.tenant_id, pb.nguon_von_id,
	          SUM(pb.so_tien_phan_bo) AS da_phan_bo,
	          COUNT(DISTINCT pb.du_an_id) AS so_du_an
	     FROM phan_bo_nguon_von pb
	     JOIN du_an da
	       ON da.tenant_id = pb.tenant_id AND da.tenant_id = $1 AND da.id = pb.du_an_id
	    WHERE pb.tenant_id = $1 AND pb.deleted_at IS NULL
	      AND da.deleted_at IS NULL AND da.nam = $2
	    GROUP BY pb.tenant_id, pb.nguon_von_id)`

// tongChungTuTheoNguon is "đã giải ngân" per source, FOR ONE YEAR — the numerator of two of §6's
// three bars.
//
// THE YEAR COMES FROM THE VOUCHER'S PROJECT, for the reason tongPhanBoTheoNguon gives. It is the same
// year rule DuAnStore applies to the commune's disbursed total (a project's vouchers count in that
// project's year), so a source card and the commune's KPI agree about which year a payment is in.
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
// SOFT-DELETED VOUCHERS AND PROJECTS ARE EXCLUDED, everywhere and always (rule 7, invariant 2).
const tongChungTuTheoNguon = `(SELECT ct.tenant_id, ct.nguon_von_id, SUM(ct.so_tien) AS da_giai_ngan
	     FROM chung_tu_giai_ngan ct
	     JOIN du_an da
	       ON da.tenant_id = ct.tenant_id AND da.tenant_id = $1 AND da.id = ct.du_an_id
	    WHERE ct.tenant_id = $1 AND ct.deleted_at IS NULL AND ct.nguon_von_id IS NOT NULL
	      AND da.deleted_at IS NULL AND da.nam = $2
	    GROUP BY ct.tenant_id, ct.nguon_von_id)`

// TienDoTheoNguon reads §6's block for one budget year: every live source of the commune's
// catalogue, the amount granted to it that year, and the year's three figures its card is drawn
// from.
//
// THE RATIOS ARE NOT COMPUTED IN SQL. This returns amounts and counts; domain.TienDoNguonVon turns
// them into the three bars, including the case where a bar HAS NO VALUE because its denominator is
// zero (§6's "Chương trình mục tiêu quốc gia, đã phân bổ 0 đ" — and, since 0013, a source with no
// amount granted for the year). A ratio computed here would have to choose a number for that case,
// and every available number is a lie about a commune's work.
//
// A SOURCE WITH NO AMOUNT, NO ALLOCATION AND NO VOUCHER IN THE YEAR STILL APPEARS, which is why all
// three joins are LEFT. §6's sample table lists such a row; dropping it would make a source the
// commune declared vanish from the screen that exists to show the commune's sources.
func (s *NguonVonStore) TienDoTheoNguon(ctx context.Context, nam int) ([]domain.TienDoNguonVon, error) {
	if nam == 0 {
		return nil, ErrThieuNamNganSach
	}

	stmt := `SELECT ` + cotNguonVon + `,
			COALESCE(pb.da_phan_bo, 0), COALESCE(pb.so_du_an, 0), COALESCE(ct.da_giai_ngan, 0)
		FROM nguon_von nv
		` + joinSoTienNam + `
		LEFT JOIN ` + tongPhanBoTheoNguon + ` pb
		       ON pb.tenant_id = nv.tenant_id AND pb.nguon_von_id = nv.id
		LEFT JOIN ` + tongChungTuTheoNguon + ` ct
		       ON ct.tenant_id = nv.tenant_id AND ct.nguon_von_id = nv.id
		WHERE nv.tenant_id = $1 AND nv.deleted_at IS NULL
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
		t.NguonVon.Nam = nam
		// POSITIONAL — the first four in lockstep with cotNguonVon.
		if err := rows.Scan(&t.NguonVon.ID, &t.NguonVon.Ten, &t.NguonVon.ThuTu, &tong,
			&daPhanBo, &soDuAn, &daGiaiNgan); err != nil {
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
