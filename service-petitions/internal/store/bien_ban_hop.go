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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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

// cotBienBan IS READ BY POSITION in quetBienBan. It is the column list of the WRITE path's locking
// read (BienBanTheoIDDeSua) and is left exactly as that path uses it.
const cotBienBan = `id, ten_cuoc_hop, ngay_hop, so_hieu, dia_diem, chu_tri_ma,
	nguoi_tao_ma, tao_luc`

// cotBienBanDoc is what the REGISTER LIST reads: cotBienBan plus the lifecycle columns of migration
// 0012, APPENDED AT THE TAIL (quetBienBanDoc is positional).
//
// `noi_dung`, `thanh_phan` AND `dinh_kem` ARE STILL NOT IN IT: §2's card does not draw them, and the
// full text of every meeting on a page would be the largest part of the response by far. The detail
// read adds the first two (cotBienBanChiTiet); `dinh_kem` has no file store behind it yet.
const cotBienBanDoc = cotBienBan + `, trang_thai, ky_luc, ky_boi_ma, thu_ky_ma,
	tb_so_ky_hieu, tb_ngay, bo_sung_cho_id`

// cotBienBanChiTiet is what GET /api/v1/meetings/{id} reads: the list's columns plus the minutes body
// and the attendee list. `thanh_phan` is JSONB and is read as its text form.
const cotBienBanChiTiet = cotBienBanDoc + `, noi_dung, thanh_phan`

// SapXepBienBan is the closed set of sorts GET /api/v1/meetings offers.
//
// `held_on` IS THE DEFAULT (user decision 5, 25/09/2026): meeting day newest first, same day by entry
// time, `id` last — exactly the index `bien_ban_hop_theo_ngay_hop (tenant_id, ngay_hop DESC,
// tao_luc DESC, id)` of migration 0012.
//
// ⚠ `held_on` IS NOT SERVED BY store.QueryPage, and its declared Kind (text) is the CURSOR's shape,
// not the column's. QueryPage pages on ONE column plus `id` and binds the key with the column's own
// type; this order has THREE keys, and the first is a DATE that must be bound as a DATE (0012 header:
// a DATE compared with a bound instant is cast through the session time zone). trangTheoNgayHop owns
// that statement; the anchor travels as one opaque text key `YYYY-MM-DD|<tao_luc RFC 3339>` inside
// the same versioned, sort-bound cursor every other list uses (page.Encode / page.Decode).
//
// `created_at` STAYS ACCEPTED — it was the contract's only sort until this pass, so a client that
// sends it keeps working. It still pages through QueryPage exactly as before.
var SapXepBienBan = page.NewAllowlist(page.Desc,
	page.Col("held_on", "ngay_hop", page.KindText),
	page.Col("created_at", "tao_luc", page.KindTime),
)

// mocBienBan BINDS each allowlisted sort to the way its cursor value is read out of a scanned row.
// store.NewMoc compares the two lists AT CONSTRUCTION, so a sort added above without a reader here is
// a panic at startup rather than a wrong page order after release.
var mocBienBan = store.NewMoc[domain.BienBanHop](SapXepBienBan,
	map[string]func(domain.BienBanHop) page.Key{
		"held_on":    func(b domain.BienBanHop) page.Key { return page.TextKey(mocNgayHop(b)) },
		"created_at": func(b domain.BienBanHop) page.Key { return page.TimeKey(b.TaoLuc) },
	})

// dinhDangNgayHop is the calendar-day format of a DATE bound or read as text.
const dinhDangNgayHop = "2006-01-02"

// mocNgayHop is the held_on cursor key of one row: the meeting DAY as `YYYY-MM-DD` and the entry
// instant, joined by `|`. The day is formatted from the scanned DATE (midnight UTC from the driver),
// never converted through a local zone.
func mocNgayHop(b domain.BienBanHop) string {
	return b.NgayHop.Format(dinhDangNgayHop) + "|" + b.TaoLuc.UTC().Format(time.RFC3339Nano)
}

// tachMocNgayHop reverses mocNgayHop. EVERY FAILURE WRAPS page.ErrCursor, so the handler answers the
// same 400 it answers for any other bad cursor, and never echoes the value.
func tachMocNgayHop(s string) (ngay string, luc time.Time, err error) {
	phan := strings.Split(s, "|")
	if len(phan) != 2 {
		return "", time.Time{}, fmt.Errorf("%w: mốc ngày họp sai hình dạng", page.ErrCursor)
	}
	d, err := time.Parse(dinhDangNgayHop, phan[0])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: ngày họp trong mốc", page.ErrCursor)
	}
	luc, err = time.Parse(time.RFC3339Nano, phan[1])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: giờ nhập trong mốc", page.ErrCursor)
	}
	return d.Format(dinhDangNgayHop), luc, nil
}

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

	var (
		kq  page.Result[domain.BienBanHop]
		err error
	)
	if yc.Column().Param == "held_on" {
		kq, err = s.trangTheoNgayHop(ctx, yc)
	} else {
		kq, err = store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
			Columns: cotBienBanDoc,
			Table:   "bien_ban_hop",
			// `deleted_at IS NULL` FIRST AND ALWAYS (rule 7, invariant 2). The partial index
			// `bien_ban_hop_so` is built on exactly this predicate.
			Filter: `AND deleted_at IS NULL`,
		}, yc, mocBienBan, func(rows *sql.Rows) (domain.BienBanHop, string, error) {
			b, err := quetBienBanDoc(rows, false)
			if err != nil {
				return domain.BienBanHop{}, "", err
			}
			return b, b.ID, nil
		})
	}
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

// trangTheoNgayHop reads one page in the register's order: `ngay_hop DESC, tao_luc DESC, id ASC` —
// the column order AND directions of index `bien_ban_hop_theo_ngay_hop` (migration 0012), so the
// planner can walk it forwards (desc) or backwards (asc) without a sort.
//
// # WHY THE ANCHOR IS EXPANDED AND NOT A ROW COMPARISON
//
// `(a, b, c) < ($x, $y, $z)` only means "after this row" when every key runs in the SAME direction.
// Here `id` runs opposite to the other two (that is the index), so the predicate is written out:
//
//	ngay_hop < d OR (ngay_hop = d AND (tao_luc < t OR (tao_luc = t AND id > i)))      -- desc
//
// and every comparison flips for `order=asc`.
//
// # THE DAY IS BOUND AS `$2::date` FROM A 'YYYY-MM-DD' STRING
//
// Migration 0012's header warning: a DATE compared with a bound time.Time is cast through the
// session time zone, so a server in UTC+7 would compare against the previous day and the page would
// repeat or skip a whole day of meetings. A text value cast to DATE carries no zone at all.
//
// ALL THREE KEYS ARE NOT NULL (`ngay_hop` and `tao_luc` by 0007's schema, `id` as the key), which is
// what a keyset walk needs: a NULL key would drop its row from every page after the first.
func (s *BienBanHopStore) trangTheoNgayHop(ctx context.Context, yc page.Request) (
	page.Result[domain.BienBanHop], error) {

	out := page.NewResult[domain.BienBanHop]()

	// op compares the two DATE/TIME keys, opID the tie-break, which runs the other way.
	op, opID, huong, huongID := "<", ">", "DESC", "ASC"
	if yc.Dir() == page.Asc {
		op, opID, huong, huongID = ">", "<", "ASC", "DESC"
	}

	var tail strings.Builder
	tail.WriteString(`AND deleted_at IS NULL`)
	args := make([]any, 0, 4)
	if a, ok := yc.After(); ok {
		ngay, luc, err := tachMocNgayHop(a.Key.Text())
		if err != nil {
			return out, err
		}
		fmt.Fprintf(&tail, ` AND (ngay_hop %s $2::date OR (ngay_hop = $2::date AND (tao_luc %s $3 OR (tao_luc = $3 AND id %s $4))))`,
			op, op, opID)
		args = append(args, ngay, luc, a.ID)
	}
	fmt.Fprintf(&tail, ` ORDER BY ngay_hop %s, tao_luc %s, id %s LIMIT $%d`,
		huong, huong, huongID, len(args)+2)
	// limit+1: the extra row only says there is a next page; it is never returned.
	args = append(args, yc.Limit()+1)

	rows, err := s.db.For(ctx).Query(ctx, cotBienBanDoc, "bien_ban_hop", tail.String(), args...)
	if err != nil {
		return out, fmt.Errorf("bien_ban_hop: đọc trang theo ngày họp: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if len(out.Items) == yc.Limit() {
			out.HasMore = true
			break
		}
		b, err := quetBienBanDoc(rows, false)
		if err != nil {
			return page.NewResult[domain.BienBanHop](), err
		}
		out.Items = append(out.Items, b)
	}
	if err := rows.Err(); err != nil {
		return page.NewResult[domain.BienBanHop](), fmt.Errorf("bien_ban_hop: duyệt trang: %w", err)
	}
	if out.HasMore {
		cuoi := out.Items[len(out.Items)-1]
		out.NextCursor = page.Encode(yc.Column(), yc.Dir(),
			page.Anchor{Key: page.TextKey(mocNgayHop(cuoi)), ID: cuoi.ID})
	}
	return out, nil
}

// TheoID reads ONE live meeting of this commune with everything the detail screen shows: the list's
// columns, the minutes body, the attendees, its conclusions with their counters, and the ids of the
// live supplementary minutes that point at it.
//
// ErrBienBanKhongTonTai FOR ALL THREE CAUSES — unknown id, another commune's id, soft-deleted — and
// the handler answers one identical 404. Telling them apart tells a caller which minutes exist in a
// register they are not reading.
//
// THREE STATEMENTS, EACH BOUNDED, NONE PER CONCLUSION: the meeting, its conclusions with counters
// (the list's own aggregate, cauKetLuanKemDem), and its supplements.
func (s *BienBanHopStore) TheoID(ctx context.Context, id string) (domain.BienBanHop, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotBienBanChiTiet, "bien_ban_hop",
		`AND id = $2 AND deleted_at IS NULL`, id)
	if err != nil {
		return domain.BienBanHop{}, fmt.Errorf("bien_ban_hop: đọc một biên bản: %w", err)
	}
	if !rows.Next() {
		err := rows.Err()
		rows.Close()
		if err != nil {
			return domain.BienBanHop{}, fmt.Errorf("bien_ban_hop: đọc một biên bản: %w", err)
		}
		return domain.BienBanHop{}, ErrBienBanKhongTonTai
	}
	b, err := quetBienBanDoc(rows, true)
	rows.Close()
	if err != nil {
		return domain.BienBanHop{}, err
	}

	theo, err := s.ketLuanTheoBienBan(ctx, []string{b.ID})
	if err != nil {
		return domain.BienBanHop{}, err
	}
	b.KetLuan = theo[b.ID]

	if b.DuocBoSungBoi, err = s.boSungCua(ctx, b.ID); err != nil {
		return domain.BienBanHop{}, err
	}
	return b, nil
}

// TranBoSungMotBienBan bounds the supplements listed on one meeting. A meeting corrected more than a
// handful of times is already unusual; past this ceiling the data is wrong, and the read REFUSES
// rather than truncating — a short list would hide a correction of a signed record.
const TranBoSungMotBienBan = 100

// ErrQuaNhieuBoSung — the ceiling above was crossed.
var ErrQuaNhieuBoSung = errors.New("bien_ban_hop: vượt trần số biên bản bổ sung của một biên bản")

// boSungCua lists the LIVE supplementary minutes of one meeting, oldest first. NEVER nil — an empty
// slice is "nobody supplemented it", which is a different statement from "not read".
func (s *BienBanHopStore) boSungCua(ctx context.Context, id string) ([]string, error) {
	rows, err := s.db.For(ctx).Query(ctx, "id", "bien_ban_hop",
		`AND bo_sung_cho_id = $2 AND deleted_at IS NULL ORDER BY ngay_hop, tao_luc, id LIMIT $3`,
		id, TranBoSungMotBienBan+1)
	if err != nil {
		return nil, fmt.Errorf("bien_ban_hop: đọc biên bản bổ sung: %w", err)
	}
	defer rows.Close()

	ra := make([]string, 0, 2)
	for rows.Next() {
		var mot string
		if err := rows.Scan(&mot); err != nil {
			return nil, fmt.Errorf("bien_ban_hop: đọc dòng bổ sung: %w", err)
		}
		ra = append(ra, mot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bien_ban_hop: duyệt biên bản bổ sung: %w", err)
	}
	if len(ra) > TranBoSungMotBienBan {
		return nil, ErrQuaNhieuBoSung
	}
	return ra, nil
}

// TranNhiemVuMotKetLuan bounds GET /api/v1/meetings/{id}/conclusions/{stt}/tasks. The route returns
// the WHOLE list (a conclusion's tasks are one screen, §2's expanded row, not a register one pages
// through), so the bound is a ceiling the data is measured against. §3 lets a conclusion be split
// many times; 200 is far past "many" and short of anything a shared process should hand one caller.
const TranNhiemVuMotKetLuan = 200

// ErrQuaNhieuNhiemVuKetLuan — the ceiling was crossed. The caller REFUSES (500) rather than
// truncating: a silently short list is a task that vanished from its conclusion's row while the
// counter beside it still says it exists.
var ErrQuaNhieuNhiemVuKetLuan = errors.New("nhiem_vu: vượt trần số nhiệm vụ của một kết luận")

// cauKetLuanSong resolves ONE live conclusion of ONE live meeting by its ordinal. The meeting is
// joined so a conclusion of soft-deleted minutes answers "not found", exactly as the detail route
// answers for the minutes themselves. Both tables bound to $1 (QueryJoin's contract).
const cauKetLuanSong = `SELECT k.id FROM ket_luan_hop k
	JOIN bien_ban_hop b ON b.tenant_id = $1 AND b.id = k.bien_ban_id AND b.deleted_at IS NULL
	WHERE k.tenant_id = $1 AND k.bien_ban_id = $2 AND k.thu_tu = $3 AND k.deleted_at IS NULL`

// NhiemVuCuaKetLuan reads the LIVE tasks split from one conclusion, in the order they were split.
//
// TWO STATEMENTS: resolve the conclusion (ErrKetLuanKhongTonTai for an unknown meeting, another
// commune's meeting, removed minutes, an unknown or removed conclusion — one answer for all), then
// the tasks by the blurred pair `nguon_giao = 'ket-luan-hop' AND nguon_id = <conclusion id>`. The
// source code is a BOUND PARAMETER carrying the domain constant, for cauKetLuanKemDem's reason.
//
// The rows are the task register's own rows (cotNhiemVu / quetNhiemVu), so the response can reuse
// the register's row shape without a second mapping.
func (s *BienBanHopStore) NhiemVuCuaKetLuan(ctx context.Context, bienBanID string, thuTu int) (
	[]domain.NhiemVu, error) {

	ketLuanID, err := s.ketLuanSong(ctx, bienBanID, thuTu)
	if err != nil {
		return nil, err
	}

	// `deleted_at IS NULL` — rule 7, invariant 2: a removed task leaves the list, as it leaves `x/y`.
	rows, err := s.db.For(ctx).Query(ctx, cotNhiemVu, "nhiem_vu",
		`AND nguon_giao = $2 AND nguon_id = $3 AND deleted_at IS NULL ORDER BY tao_luc, id LIMIT $4`,
		string(domain.NguonKetLuanHop), ketLuanID, TranNhiemVuMotKetLuan+1)
	if err != nil {
		return nil, fmt.Errorf("nhiem_vu: đọc nhiệm vụ của kết luận: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.NhiemVu, 0, 4)
	for rows.Next() {
		n, err := quetNhiemVu(rows)
		if err != nil {
			return nil, err
		}
		ra = append(ra, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nhiem_vu: duyệt nhiệm vụ của kết luận: %w", err)
	}
	if len(ra) > TranNhiemVuMotKetLuan {
		// The rows are DROPPED rather than trimmed and returned — see ErrQuaNhieuNhiemVuKetLuan.
		return nil, ErrQuaNhieuNhiemVuKetLuan
	}
	return ra, nil
}

// ketLuanSong runs cauKetLuanSong and returns the conclusion's internal id.
func (s *BienBanHopStore) ketLuanSong(ctx context.Context, bienBanID string, thuTu int) (string, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, cauKetLuanSong, bienBanID, thuTu)
	if err != nil {
		return "", fmt.Errorf("ket_luan_hop: đọc kết luận: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("ket_luan_hop: đọc kết luận: %w", err)
		}
		return "", ErrKetLuanKhongTonTai
	}
	var id string
	if err := rows.Scan(&id); err != nil {
		return "", fmt.Errorf("ket_luan_hop: đọc dòng: %w", err)
	}
	return id, nil
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
//
// # THE THIRD COUNTER AND THE MARK (migration 0012, user decision 4)
//
// `so_nhiem_vu_tre_han` counts live tasks NOT in `$3` (`hoan-thanh`) that are late by
// dieuKienTreHan — the one SQL spelling of domain.NhiemVu.TreHan, shared with the task register's
// "Chỉ việc quá hạn" filter so the two can never disagree. `k.khong_phat_sinh` is the human-set
// mark. domain.KetLuanHop.TrangThai turns these four inputs into the derived status; nothing here
// decides a status.
const cauKetLuanKemDem = `SELECT k.id, k.bien_ban_id, k.thu_tu, k.noi_dung, k.tao_luc, k.khong_phat_sinh, n.so_nhiem_vu, n.so_nhiem_vu_xong, n.so_nhiem_vu_tre_han FROM ket_luan_hop k
	LEFT JOIN LATERAL (
		SELECT count(*) AS so_nhiem_vu,
		       count(*) FILTER (WHERE nv.trang_thai = $3) AS so_nhiem_vu_xong,
		       count(*) FILTER (WHERE nv.trang_thai <> $3 AND ` + dieuKienTreHan + `) AS so_nhiem_vu_tre_han
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
		if err := rows.Scan(&k.ID, &k.BienBanID, &k.ThuTu, &k.NoiDung, &k.TaoLuc, &k.KhongPhatSinh,
			&k.SoNhiemVu, &k.SoNhiemVuXong, &k.SoNhiemVuTreHan); err != nil {
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

// quetBienBanDoc reads one row of cotBienBanDoc — or of cotBienBanChiTiet when chiTiet is true.
//
// POSITIONAL, IN LOCKSTEP WITH THOSE TWO LISTS, and every column 0012 added is APPENDED after
// cotBienBan's eight, never inserted: a destination inserted in the middle silently shifts every
// column after it. Four of the new columns are nullable TEXT side by side (`ky_boi_ma`, `thu_ky_ma`,
// `tb_so_ky_hieu`, `bo_sung_cho_id`); the fake-driver suite gives each a visibly different kind of
// value so a shift comes back as wrong data.
func quetBienBanDoc(r quangKiem, chiTiet bool) (domain.BienBanHop, error) {
	var (
		b domain.BienBanHop

		soHieu, diaDiem, chuTri       sql.NullString
		kyBoi, thuKy, tbSo, boSungCho sql.NullString
		kyLuc, tbNgay                 sql.NullTime
		noiDung                       sql.NullString
		thanhPhan                     []byte
	)

	dich := []any{
		&b.ID, &b.TenCuocHop, &b.NgayHop, &soHieu, &diaDiem, &chuTri,
		&b.NguoiTaoMa, &b.TaoLuc,
		&b.TrangThai, &kyLuc, &kyBoi, &thuKy,
		&tbSo, &tbNgay, &boSungCho,
	}
	if chiTiet {
		dich = append(dich, &noiDung, &thanhPhan)
	}
	if err := r.Scan(dich...); err != nil {
		return domain.BienBanHop{}, fmt.Errorf("bien_ban_hop: đọc dòng: %w", err)
	}

	b.SoHieu = soHieu.String
	b.DiaDiem = diaDiem.String
	b.ChuTriMa = chuTri.String
	b.KyLuc = kyLuc.Time
	b.KyBoiMa = kyBoi.String
	b.ThuKyMa = thuKy.String
	b.TbSoKyHieu = tbSo.String
	b.TbNgay = tbNgay.Time
	b.BoSungChoID = boSungCho.String

	if chiTiet {
		b.NoiDung = noiDung.String
		// NEVER nil ON THE DETAIL READ: the column is NOT NULL DEFAULT '[]', and an empty list is
		// "nobody recorded attendees", which the response must be able to say.
		b.ThanhPhan = []string{}
		if len(thanhPhan) > 0 {
			if err := json.Unmarshal(thanhPhan, &b.ThanhPhan); err != nil {
				// NOT the value: the list names people (rule 3, forbidden #3).
				return domain.BienBanHop{}, fmt.Errorf("bien_ban_hop: đọc thành phần tham dự: %w", err)
			}
		}
	}
	return b, nil
}
