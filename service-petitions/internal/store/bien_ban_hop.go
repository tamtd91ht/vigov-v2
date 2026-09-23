package store

// THE MEETING MINUTES REGISTER — one page of cards, each with its conclusions and the two task
// counters §2 puts in the badge. SQL, and nothing else.
//
// FOUR THINGS HOLD IN EVERY STATEMENT IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1, from the context (rule 1, invariants 4 and 5). It is never a parameter
//     here, so no caller can reach another commune's minutes — and in the JOINED read below it is
//     bound on BOTH tables, because a join constrained on one side matches another commune's rows
//     wherever ids collide.
//  2. NOTHING HERE OPENS A TRANSACTION, AND NOTHING HERE WRITES. This pass is the read path.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — the minutes, the conclusions
//     AND the tasks behind the counters. A removed task must leave both halves of `x/y`, or the
//     fraction stops being true the first time a commune removes one.
//  4. EVERY VALUE IS A BOUND PARAMETER, including the two ENUM CODES in the counting query. See
//     cauKetLuanKemDem for why a literal there would be worse than an injection risk.

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// BienBanHopStore is the only path to `bien_ban_hop` and `ket_luan_hop` (migration 0007).
//
// EVERY READ GOES THROUGH *store.Scoped, which binds `tenant_id` to $1 from the context. The commune
// is therefore not a parameter of any method here and cannot be made into one — a repository that
// can be built without a commune is a repository that can query across communes.
type BienBanHopStore struct {
	db *store.DB
}

func NewBienBanHopStore(db *store.DB) *BienBanHopStore { return &BienBanHopStore{db: db} }

// cotBienBan IS READ BY POSITION in the Scan below.
//
// THREE COLUMNS OF THE TABLE ARE DELIBERATELY NOT IN IT, and their absence is a decision rather than
// an oversight: `noi_dung` (the minutes in full), `thanh_phan` and `dinh_kem` are not on the card
// §2 draws, and the full text of every meeting on a page would be the largest part of the response
// by far. There is no detail route in this pass, so nothing reads them yet — a client must not read
// their absence as "this meeting has no minutes body": there is no field.
const cotBienBan = `id, ten_cuoc_hop, ngay_hop, so_hieu, dia_diem, chu_tri_ma,
	nguoi_tao_ma, tao_luc`

// SapXepBienBan is the closed set of sorts GET /api/v1/meetings offers.
//
// ONE COLUMN, AND `ngay_hop` IS DELIBERATELY NOT THE SECOND — although §2 says "mới nhất ở trên"
// about the MEETINGS and a reader will reach for it. Two independent reasons, either one sufficient:
//
//	it is a DATE      the cursor compares `(col, id) > ($n, $n+1)` with a bound time.Time, and a
//	                  DATE compared with a timestamptz is cast through the session's time zone. No
//	                  PostgreSQL is reachable from this environment (VIGOV_TEST_DSN unset), so that
//	                  comparison cannot be verified here — and an unverifiable cursor is a page that
//	                  repeats and skips rows with nothing turning red.
//	no index          0007 creates none on `ngay_hop`, on purpose: an index nothing reads is a write
//	                  cost with no reader.
//
// `tao_luc` is NOT NULL and indexed (`bien_ban_hop_so`, migration 0007), which is what
// page.QueryPage needs from a sort column. It is also the key every other register in this
// repository pages on. Offering `held_at` later is one line here, one index there.
var SapXepBienBan = page.NewAllowlist(page.Desc,
	page.Col("created_at", "tao_luc", page.KindTime),
)

// mocBienBan BINDS the allowlisted sort column to the way its cursor value is read out of a scanned
// row. store.NewMoc compares the two lists AT CONSTRUCTION, so a sort added above without a reader
// here is a panic at startup rather than a wrong page order after release.
var mocBienBan = store.NewMoc[domain.BienBanHop](SapXepBienBan,
	map[string]func(domain.BienBanHop) page.Key{
		"created_at": func(b domain.BienBanHop) page.Key { return page.TimeKey(b.TaoLuc) },
	})

// DanhSach reads ONE PAGE of the commune's meeting minutes, EACH WITH ITS CONCLUSIONS AND COUNTERS.
//
// # TWO STATEMENTS, NEVER 1+N
//
// The page of cards is one query; the conclusions of every card on that page, with their task
// counts already aggregated, are a SECOND query taking the page's ids. A query per card would be
// affordable on the prototype's three meetings and unaffordable on a commune's third year, and the
// shape that behaves that way is the shape nobody notices until then.
//
// # WHY THE COUNTERS ARE NOT COMPUTED HERE IN GO
//
// They are `count(*)` over a table this page never loads. Counting in Go would mean loading every
// task of every conclusion on the page just to discard all but two integers — and it would load
// tasks whose titles quote citizens' cases into a handler that has no business holding them
// (rule 3).
func (s *BienBanHopStore) DanhSach(ctx context.Context, yc page.Request) (
	page.Result[domain.BienBanHop], error) {

	kq, err := store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotBienBan,
		Table:   "bien_ban_hop",
		// `deleted_at IS NULL` FIRST AND ALWAYS (rule 7, invariant 2). The partial index
		// `bien_ban_hop_so` is built on exactly this predicate.
		Filter: `AND deleted_at IS NULL`,
	}, yc, mocBienBan, func(rows *sql.Rows) (domain.BienBanHop, string, error) {
		b, err := quetBienBan(rows)
		if err != nil {
			return domain.BienBanHop{}, "", err
		}
		return b, b.ID, nil
	})
	if err != nil {
		return kq, err
	}
	if len(kq.Items) == 0 {
		// NO SECOND STATEMENT ON AN EMPTY PAGE, and this is not only a saving: the `IN (…)` list
		// below is built from the ids, and an empty list is not valid SQL. A newly onboarded commune
		// takes this branch on every request until its first meeting is recorded.
		return kq, nil
	}

	ids := make([]string, 0, len(kq.Items))
	for _, b := range kq.Items {
		ids = append(ids, b.ID)
	}
	theoBienBan, err := s.ketLuanTheoBienBan(ctx, ids)
	if err != nil {
		return page.NewResult[domain.BienBanHop](), err
	}
	for i := range kq.Items {
		kq.Items[i].KetLuan = theoBienBan[kq.Items[i].ID]
	}
	return kq, nil
}

// cauKetLuanKemDem reads the conclusions of several meetings WITH the two task counters of §2.
//
// # THE STATEMENT, AND WHY EACH PIECE IS THERE
//
//	LEFT JOIN LATERAL      a conclusion with NO tasks must still come back — that is the `Chưa tách
//	                       thành nhiệm vụ nào` line of §2, and an inner join would silently drop
//	                       exactly the conclusions somebody still has to act on. The aggregate
//	                       subquery always returns one row, so `count(*)` is 0 and never NULL.
//	nv.tenant_id = $1      store.Scoped.QueryJoin's contract, and the reason it exists: joining on
//	                       `nguon_id` alone would count ANOTHER COMMUNE'S tasks into this commune's
//	                       badge wherever two ids collide — a cross-commune read no test of a single
//	                       commune could ever show (rule 1).
//	nv.deleted_at IS NULL  rule 7, invariant 2. A removed task leaves BOTH halves of `x/y`.
//	ORDER BY               `bien_ban_id, thu_tu` — the order the circles ① ② ③ are drawn in (§2),
//	                       fixed here so no layer above has to sort and no two of them can disagree.
//
// # THE TWO ENUM CODES ARE BOUND PARAMETERS, NOT LITERALS, AND THAT IS NOT ABOUT INJECTION
//
// `$2` is domain.NguonKetLuanHop and `$3` is domain.HoanThanh. Written as literals in this string
// they would be a SECOND SPELLING of two codes the domain package owns — and a typo in either
// produces no error at all: the badge reads `0/0` on every card, forever, and looks exactly like a
// commune that has not split any conclusions yet. Passing the constants makes a rename a compile
// error instead.
//
// # ONE FORMATTING CONSTRAINT, STATED SO IT IS NOT "TIDIED" AWAY
//
// The outer ` FROM ` stays on the same line as the last selected column. The fake driver the unit
// suite runs on reads the SELECT list as the text between the first `SELECT ` and the first ` FROM `
// (see driver_gia_test.go); with a newline in front of FROM it cannot find it, and the suite fails
// with a message about the column list rather than about this query.
const cauKetLuanKemDem = `SELECT k.id, k.bien_ban_id, k.thu_tu, k.noi_dung, k.tao_luc, n.so_nhiem_vu, n.so_nhiem_vu_xong FROM ket_luan_hop k
	LEFT JOIN LATERAL (
		SELECT count(*) AS so_nhiem_vu,
		       count(*) FILTER (WHERE nv.trang_thai = $3) AS so_nhiem_vu_xong
		FROM nhiem_vu nv
		WHERE nv.tenant_id = $1
		  AND nv.nguon_giao = $2
		  AND nv.nguon_id = k.id
		  AND nv.deleted_at IS NULL
	) n ON true
	WHERE k.tenant_id = $1 AND k.deleted_at IS NULL AND k.bien_ban_id IN (`

// ketLuanTheoBienBan returns the conclusions of the given meetings, keyed by meeting id and already
// in `thu_tu` order.
//
// THE IDS ARE PLACEHOLDERS, NOT TEXT. `$4, $5, …` are generated from the COUNT of ids and the values
// are bound; nothing from a row or a request is ever concatenated into the statement. The count is
// bounded by the page size, which page.Parse has already capped.
func (s *BienBanHopStore) ketLuanTheoBienBan(ctx context.Context, ids []string) (
	map[string][]domain.KetLuanHop, error) {

	var b strings.Builder
	b.WriteString(cauKetLuanKemDem)
	// $1 is the commune, $2 the source code, $3 the finished status — the ids start at $4.
	args := make([]any, 0, len(ids)+2)
	args = append(args, string(domain.NguonKetLuanHop), string(domain.HoanThanh))
	for i, id := range ids {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("$" + strconv.Itoa(i+4))
		args = append(args, id)
	}
	b.WriteString(") ORDER BY k.bien_ban_id, k.thu_tu")

	rows, err := s.db.For(ctx).QueryJoin(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("ket_luan_hop: đọc kết luận kèm bộ đếm: %w", err)
	}
	defer rows.Close()

	ra := make(map[string][]domain.KetLuanHop, len(ids))
	for rows.Next() {
		var k domain.KetLuanHop
		if err := rows.Scan(&k.ID, &k.BienBanID, &k.ThuTu, &k.NoiDung, &k.TaoLuc,
			&k.SoNhiemVu, &k.SoNhiemVuXong); err != nil {
			return nil, fmt.Errorf("ket_luan_hop: đọc dòng: %w", err)
		}
		ra[k.BienBanID] = append(ra[k.BienBanID], k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ket_luan_hop: duyệt kết quả: %w", err)
	}
	return ra, nil
}

// quetBienBan reads one row of cotBienBan.
//
// POSITIONAL, IN LOCKSTEP WITH cotBienBan. database/sql binds by POSITION, so a destination inserted
// or removed anywhere but the tail silently shifts every column after it — and the three adjacent
// nullable TEXT columns here (`so_hieu`, `dia_diem`, `chu_tri_ma`) would shift into each other
// without any error at all, putting a room name where a chairperson's code belongs.
// TestPgCotBienBanTrongMaKhopVoiLuocDoThat asserts the list against the real schema BY NAME.
func quetBienBan(r quangKiem) (domain.BienBanHop, error) {
	var (
		b domain.BienBanHop

		// EVERY NULLABLE COLUMN IS READ THROUGH AN EXPLICIT NULL TYPE. Scanning a NULL straight into
		// a string is a runtime error in some drivers and a zero value in others, and the second is
		// how "no reference number was recorded" quietly becomes an empty string nobody distrusts.
		soHieu, diaDiem, chuTri sql.NullString
	)

	dich := []any{
		&b.ID, &b.TenCuocHop, &b.NgayHop, &soHieu, &diaDiem, &chuTri,
		&b.NguoiTaoMa, &b.TaoLuc,
	}
	if err := r.Scan(dich...); err != nil {
		return domain.BienBanHop{}, fmt.Errorf("bien_ban_hop: đọc dòng: %w", err)
	}

	b.SoHieu = soHieu.String
	b.DiaDiem = diaDiem.String
	b.ChuTriMa = chuTri.String
	return b, nil
}
