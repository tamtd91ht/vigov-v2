package store

// THE INTERNAL ANNOUNCEMENT BOOK — `thong_bao`, `thong_bao_bo_phan`, `thong_bao_nguoi_nhan`
// (migration 0005). SQL, and nothing else.
//
// EVERY IDENTIFIER HERE CARRIES `ThongBaoNoiBo` OR AN UNAMBIGUOUS PREFIX, and that is not a style
// choice: `ThongBaoGuiCongDanStore`, `cotThongBao` and `chenThongBao` already exist in this
// package and belong to the CITIZEN ledger of migration 0004. The two modules share one Vietnamese
// word and nothing else — one talks to staff inside the commune, the other leaves the commune to
// a citizen (rule 4, two trust levels). A name reused between them is one refactor away from
// putting an internal notice on the citizen channel.
//
// FOUR THINGS HOLD IN EVERY STATEMENT IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1, from the context (rule 1, invariants 4 and 5). It is never a parameter of
//     any method here, and the aggregate read below binds it on the aggregated table too.
//  2. NOTHING HERE OPENS A TRANSACTION. The write methods take a *store.ScopedTx, because the
//     audit entry has to share it (rule 6, invariant 3) and a method that opened its own would
//     make that impossible for its caller. internal/app opens it.
//  3. EVERY READ OF `thong_bao` EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — including the
//     counter read, which joins nothing and therefore has to carry the predicate itself.
//  4. EVERY VALUE IS A BOUND PARAMETER. The multi-row inserts generate `$n` placeholders from the
//     COUNT of their input and bind the values; nothing from a request is concatenated into SQL.
//
// NO audit.Write IN THIS FILE, AND THAT IS NOT THE OMISSION IT LOOKS LIKE — audit_guard warns on
// exactly this shape, so the answer is written down rather than rediscovered, and it is the same
// answer store/thong_bao_gui_cong_dan.go gives. Rule 6 requires the entry to share the
// TRANSACTION, which it does: app/SoanThongBao opens one, calls the methods below, and writes the
// entry inside the same *store.ScopedTx. What the entry cannot come from is here — `Actor` is who
// caused the write (a member of staff, an IP) and `Action` is a business verb, and neither exists
// at the SQL layer. Putting audit.Write in this file would make the store invent an actor, which
// is how an unattributable entry is born.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ThongBaoNoiBoStore is the only path to the three tables of migration 0005.
//
// It is built from *store.DB and reaches the database only through Scoped — there is no field here
// holding a *sql.DB, and adding one would reopen the hole core/store exists to close (rule 1,
// invariant 5).
type ThongBaoNoiBoStore struct {
	db *store.DB
}

func NewThongBaoNoiBoStore(db *store.DB) *ThongBaoNoiBoStore { return &ThongBaoNoiBoStore{db: db} }

// ErrThongBaoNoiBoKhongTonTai says the id addressed no live announcement OF THIS COMMUNE.
//
// The two causes are deliberately not distinguished: "no such announcement" and "that announcement
// belongs to another commune" must look identical from outside, or the error itself becomes a way
// to discover what another authority holds (rule 4, forbidden #2, applied to the commune axis).
var ErrThongBaoNoiBoKhongTonTai = errors.New("thong_bao: không có bản ghi")

// cotThongBaoNoiBo IS READ BY POSITION in quetThongBaoNoiBo.
//
// `noi_dung` IS IN THE LIST, WHICH IS THE OPPOSITE OF WHAT THE MEETING REGISTER DOES WITH ITS BODY
// COLUMN, and the difference is the screen rather than an inconsistency. §2 of chapter 08 draws
// TWO columns fed by ONE selection: the card on the left shows two truncated lines, and the detail
// panel on the right shows the SAME announcement in full. There is no detail route in this pass,
// so a list that omitted the body would leave the right-hand column with nothing to render.
//
// THE COST IS BOUNDED AND IS STATED RATHER THAN HOPED ABOUT: domain.NoiDungToiDaThongBao caps one
// body at 20.000 runes and page.MaxLimit caps a page at 100, so a maximal page is a few megabytes
// — reachable only by a commune whose every announcement is ten pages long. If that ever becomes
// real, the fix is a detail route plus an excerpt column in this list, not a silent truncation
// here: an excerpt computed in SQL would make the right-hand panel quietly show a cut-off notice.
//
// THREE ADJACENT BOOLEANS (`ghim`, `bat_buoc_xac_nhan`, `gui_thu_dien_tu`) MEAN A SWAP IN EITHER
// THIS LIST OR THE SCAN PRODUCES NO ERROR AT ALL. It produces an announcement that demands
// acknowledgement nobody asked for and is pinned nobody asked for.
// TestPgCotThongBaoNoiBoKhopVoiLuocDoThat asserts this list against the real schema BY NAME.
const cotThongBaoNoiBo = `id, tieu_de, noi_dung, trang_thai, ghim, bat_buoc_xac_nhan, ` +
	`gui_thu_dien_tu, trang_thai_thu, nguoi_soan_ma, phat_hanh_luc, tao_luc`

// SapXepThongBaoNoiBo is the closed set of sorts GET /api/v1/announcements offers.
//
// ONE COLUMN, AND `phat_hanh_luc` IS DELIBERATELY NOT THE SECOND although §3 prints it on the
// card. A sort column must be NOT NULL: core/page spells out that `(col, id) > ($n, $n+1)`
// evaluates to NULL for a NULL col, so every DRAFT would vanish from every page after the first —
// silently, which is the one failure mode a list route must not have. `tao_luc` is NOT NULL, is
// indexed (`thong_bao_so`, migration 0005) and is the key every other register in this repository
// pages on.
//
// `ghim` IS NOT AN ALTERNATIVE SORT AND PINNED-FIRST IS NOT IMPLEMENTED. §3 wants pinned
// announcements at the head of the list, and core/page carries exactly ONE sort column plus the
// `id` tie-break — `ORDER BY ghim DESC, tao_luc DESC, id DESC` is not a cursor this repository can
// express. Sorting by `ghim` alone would be worse than not offering it: a boolean sort key makes
// `id` the real order, so the second page would come back in an order nobody asked for. Reported
// rather than approximated; see the read route.
var SapXepThongBaoNoiBo = page.NewAllowlist(page.Desc,
	page.Col("created_at", "tao_luc", page.KindTime),
)

// mocThongBaoNoiBo BINDS the allowlisted sort column to the way its cursor value is read out of a
// scanned row. store.NewMoc compares the two lists AT CONSTRUCTION, so a sort added above without
// a reader here is a panic at startup rather than a wrong page order after release.
var mocThongBaoNoiBo = store.NewMoc[domain.ThongBaoNoiBo](SapXepThongBaoNoiBo,
	map[string]func(domain.ThongBaoNoiBo) page.Key{
		"created_at": func(t domain.ThongBaoNoiBo) page.Key { return page.TimeKey(t.TaoLuc) },
	})

// DanhSach reads ONE PAGE of the commune's announcement book, each card carrying §3's counter.
//
// # TWO STATEMENTS, NEVER 1+N
//
// The page is one query; the `{x}/{y} đã xác nhận` counters for every card on that page are a
// SECOND query taking the page's ids and aggregating in the database. A query per card is
// affordable on §10's three sample notices and unaffordable on a commune's third year, and the
// shape that behaves that way is the shape nobody notices until then.
//
// # WHY THE COUNTERS ARE NOT COMPUTED IN GO
//
// They are `count(*)` over a table this page never loads. Counting in Go would mean loading every
// recipient row of every announcement on the page — a few thousand rows to produce two integers
// per card — and holding the whole commune's staff codes in a handler that has no use for them.
func (s *ThongBaoNoiBoStore) DanhSach(ctx context.Context, yc page.Request) (
	page.Result[domain.ThongBaoNoiBo], error) {

	kq, err := store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotThongBaoNoiBo,
		Table:   "thong_bao",
		// `deleted_at IS NULL` FIRST AND ALWAYS (rule 7, invariant 2). The partial index
		// `thong_bao_so` is built on exactly this predicate. The commune is bound to $1 by
		// store.QueryPage through Scoped.Query — `tenant_id` is never a value this file supplies.
		Filter: `AND deleted_at IS NULL`,
	}, yc, mocThongBaoNoiBo, func(rows *sql.Rows) (domain.ThongBaoNoiBo, string, error) {
		t, err := quetThongBaoNoiBo(rows)
		if err != nil {
			return domain.ThongBaoNoiBo{}, "", err
		}
		return t, t.ID, nil
	})
	if err != nil {
		return kq, err
	}
	if len(kq.Items) == 0 {
		// NO SECOND STATEMENT ON AN EMPTY PAGE, and this is not only a saving: the `IN (…)` list
		// below is built from the ids, and an empty list is not valid SQL. A newly onboarded commune
		// takes this branch on every request until its first announcement is composed.
		return kq, nil
	}

	ids := make([]string, 0, len(kq.Items))
	for _, t := range kq.Items {
		ids = append(ids, t.ID)
	}
	dem, err := s.demNguoiNhan(ctx, ids)
	if err != nil {
		return page.NewResult[domain.ThongBaoNoiBo](), err
	}
	for i := range kq.Items {
		d := dem[kq.Items[i].ID]
		kq.Items[i].SoNguoiNhan = d.tong
		kq.Items[i].SoDaXacNhan = d.xacNhan
	}
	return kq, nil
}

// demHaiSo is the pair §3 prints: `{x}/{y} đã xác nhận`.
type demHaiSo struct{ tong, xacNhan int }

// cauDemNguoiNhan aggregates the recipient counters for several announcements at once.
//
// # WHY IT IS QueryJoin AND NOT Query, WHEN IT JOINS NOTHING
//
// store.Scoped.Query owns the whole statement — it appends `WHERE tenant_id = $1 ` and then the
// caller's tail — which leaves no room for the GROUP BY this read needs. QueryJoin's contract is
// that the CALLER writes the statement and binds the commune to $1, which is what this does:
// `tenant_id = $1` is in the WHERE clause below and there is no path here that omits it.
//
// # AN ANNOUNCEMENT WITH NO RECIPIENTS PRODUCES NO ROW
//
// A GROUP BY returns nothing for an announcement whose recipient list is empty, and a DRAFT
// composed before the list is generated is exactly that. The caller therefore reads the map with a
// zero-value default rather than expecting a key — `0/0` is the correct answer for a draft, and §3
// only draws the counter when acknowledgement was demanded anyway.
//
// # ONE FORMATTING CONSTRAINT, STATED SO IT IS NOT "TIDIED" AWAY
//
// The ` FROM ` stays on the same line as the last selected expression. The fake driver the unit
// suite runs on reads the SELECT list as the text between the first `SELECT ` and the first
// ` FROM `; with a newline in front of FROM it cannot find it, and the suite fails with a message
// about the column list rather than about this query.
const cauDemNguoiNhan = `SELECT thong_bao_id, count(*) AS tong, count(*) FILTER (WHERE da_xac_nhan_luc IS NOT NULL) AS xac_nhan FROM thong_bao_nguoi_nhan
	WHERE tenant_id = $1 AND thong_bao_id IN (`

// demNguoiNhan returns the two counters per announcement id.
//
// THE IDS ARE PLACEHOLDERS, NOT TEXT. `$2, $3, …` are generated from the COUNT of ids and the
// values are bound; nothing from a row or a request is ever concatenated into the statement. The
// count is bounded by the page size, which page.Parse has already capped at page.MaxLimit.
func (s *ThongBaoNoiBoStore) demNguoiNhan(ctx context.Context, ids []string) (
	map[string]demHaiSo, error) {

	var b strings.Builder
	b.WriteString(cauDemNguoiNhan)
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("$" + strconv.Itoa(i+2)) // $1 is the commune
		args = append(args, id)
	}
	b.WriteString(") GROUP BY thong_bao_id")

	// `tenant_id = $1` is inside cauDemNguoiNhan, and QueryJoin binds the commune from the context
	// as $1 — the caller cannot supply it and cannot omit it (rule 1, invariant 5).
	rows, err := s.db.For(ctx).QueryJoin(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("thong_bao_nguoi_nhan: đếm người nhận: %w", err)
	}
	defer rows.Close()

	ra := make(map[string]demHaiSo, len(ids))
	for rows.Next() {
		var (
			id  string
			mot demHaiSo
		)
		if err := rows.Scan(&id, &mot.tong, &mot.xacNhan); err != nil {
			return nil, fmt.Errorf("thong_bao_nguoi_nhan: đọc dòng đếm: %w", err)
		}
		ra[id] = mot
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("thong_bao_nguoi_nhan: duyệt kết quả đếm: %w", err)
	}
	return ra, nil
}

// quetMotDongThongBao is what both *sql.Rows and *sql.Row satisfy, so one scan serves the list read
// and any single-row read added later.
type quetMotDongThongBao interface{ Scan(...any) error }

// quetThongBaoNoiBo reads one row of cotThongBaoNoiBo.
//
// POSITIONAL, IN LOCKSTEP WITH cotThongBaoNoiBo. database/sql binds by POSITION, so a destination
// inserted or removed anywhere but the tail silently shifts every column after it — and the three
// adjacent booleans here would shift into each other without any error at all.
//
// `phat_hanh_luc` IS READ THROUGH sql.NullTime. Scanning a NULL straight into a time.Time is a
// runtime error in some drivers and a zero value in others, and a draft is precisely the row where
// it is NULL. In Go it lands as the zero time, which is what domain.ThongBaoNoiBo documents.
func quetThongBaoNoiBo(r quetMotDongThongBao) (domain.ThongBaoNoiBo, error) {
	var (
		t        domain.ThongBaoNoiBo
		trang    string
		trangThu string
		phatHanh sql.NullTime
	)
	if err := r.Scan(&t.ID, &t.TieuDe, &t.NoiDung, &trang, &t.Ghim, &t.BatBuocXacNhan,
		&t.GuiThuDienTu, &trangThu, &t.NguoiSoanMa, &phatHanh, &t.TaoLuc); err != nil {
		return domain.ThongBaoNoiBo{}, fmt.Errorf("thong_bao: đọc dòng: %w", err)
	}
	t.TrangThai = domain.TrangThaiThongBao(trang)
	t.TrangThaiThu = domain.TrangThaiThuThongBao(trangThu)
	t.PhatHanhLuc = phatHanh.Time
	return t, nil
}

// --- the write path -------------------------------------------------------------------------

// chenThongBaoNoiBo records one announcement.
//
// `trang_thai` AND `phat_hanh_luc` ARE BOUND PARAMETERS AND `trang_thai_thu` IS NOT. The first two
// are decided by the use case — composing a draft and issuing it are different acts — while the
// mail state is the OUTCOME of an attempt nothing in this repository makes yet. Passing it in
// would let a caller assert `da-gui` about mail no process ever sent, which is a claim about a
// colleague's inbox that nothing witnessed.
//
// `deleted_at`, `deleted_by`, `delete_reason` APPEAR NOWHERE IN THIS STATEMENT: a row is not born
// deleted, and a parameter for them is a parameter a request could eventually reach.
//
// The statement names `tenant_id` first, as every write in this system does: the commune is not an
// argument the caller chooses, it is bound from the transaction (rule 1, invariants 4 and 5).
const chenThongBaoNoiBo = `INSERT INTO thong_bao
	(tenant_id, id, tieu_de, noi_dung, trang_thai, ghim, bat_buoc_xac_nhan,
	 gui_thu_dien_tu, nguoi_soan_ma, phat_hanh_luc)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

// Chen writes one announcement row. The caller owns the transaction and the audit entry inside it.
func (s *ThongBaoNoiBoStore) Chen(ctx context.Context, tx *store.ScopedTx,
	tb domain.ThongBaoNoiBo) error {

	var phatHanh any
	if !tb.PhatHanhLuc.IsZero() {
		phatHanh = tb.PhatHanhLuc.UTC()
	}
	// tx.TenantID() is $1 — the `tenant_id` of chenThongBaoNoiBo, taken from the context.
	if _, err := tx.Exec(ctx, chenThongBaoNoiBo, string(tx.TenantID()), tb.ID,
		tb.TieuDe, tb.NoiDung, string(tb.TrangThai), tb.Ghim, tb.BatBuocXacNhan,
		tb.GuiThuDienTu, tb.NguoiSoanMa, phatHanh); err != nil {
		return fmt.Errorf("thong_bao: chèn: %w", err)
	}
	return nil
}

// THERE IS NO `ChenBoPhan` AND `thong_bao_bo_phan` HAS NO WRITER IN THIS PASS.
//
// Not an oversight, and not a method left out to be "added later when convenient": writing a row
// there would be recording that an announcement was addressed to a department, while nothing in
// this repository can turn a department into the people in it. `bo_phan` and its membership are
// service-identity's data (rule 2), and IdentityService publishes no RPC that lists the staff of
// an org unit — ResolveStaffPrincipal, BatchGetStaff, ResolveStaffNames, ListCitizenCommunes,
// AdvanceWorkingHours, ResolveDeadlines and ResolveCitizenSession are the whole contract.
//
// AN ANNOUNCEMENT ADDRESSED TO FIVE DEPARTMENTS AND DELIVERED TO NOBODY IS THE WORST OUTCOME THIS
// MODULE HAS. Nothing errors, the card appears in the book, §3's counter reads `0/0`, and the
// author believes the commune was told. So the write path refuses a department list outright
// instead — see app.ErrGuiTheoBoPhanChuaCo — and this method does not exist to make that refusal
// easy to forget. The table is in migration 0005 because §6 declares it and because the addressing
// decision is part of the record; its writer arrives with the contract.

// ChenNguoiNhan freezes the recipient list (§9.3).
//
// THE TIMESTAMP COLUMNS ARE NOT WRITTEN. A recipient is born having neither opened nor
// acknowledged the announcement, and `da_mo_luc` / `da_xac_nhan_luc` are set by the act of reading
// it — which is a route this pass does not build. Binding them here would let a caller create a
// recipient already marked as having acknowledged something they have never seen, and the trigger
// in 0005 would then refuse to correct it.
//
// `ON CONFLICT DO NOTHING` for the same reason as the departments, plus one that is specific to
// this table: a person named explicitly in §5 may ALSO be a member of a chosen department, so the
// two sources of a recipient legitimately overlap. One row per person is what makes §3's `{y}`
// count people rather than reasons.
func (s *ThongBaoNoiBoStore) ChenNguoiNhan(ctx context.Context, tx *store.ScopedTx,
	thongBaoID string, ds []domain.NguoiNhanThongBao) error {

	if len(ds) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(`INSERT INTO thong_bao_nguoi_nhan (tenant_id, thong_bao_id, nguoi_nhan_ma, dich_danh) VALUES `)
	args := make([]any, 0, 2*len(ds)+2)
	args = append(args, string(tx.TenantID()), thongBaoID)
	for i, n := range ds {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("($1, $2, $" + strconv.Itoa(2*i+3) + ", $" + strconv.Itoa(2*i+4) + ")")
		args = append(args, n.NguoiNhanMa, n.DichDanh)
	}
	b.WriteString(" ON CONFLICT DO NOTHING")

	// The generated statement writes `tenant_id` as $1 on EVERY tuple, bound from tx.TenantID()
	// above — a recipient here cannot be filed under another commune (rule 1, invariant 5).
	if _, err := tx.Exec(ctx, b.String(), args...); err != nil {
		return fmt.Errorf("thong_bao_nguoi_nhan: chèn: %w", err)
	}
	return nil
}

// --- what is deliberately absent ---------------------------------------------------------------
//
// NO `TheoID`, NO `CapNhat`, NO `XoaMem`, AND NO RECIPIENT-SCOPED READ. Each absence is a decision
// and each one is cheap to add later:
//
//	a detail read      §7 lists GET /api/thong-bao/:id, and the list already carries the body and
//	                   the counters, so the only thing a detail route would ADD is §4's recipient
//	                   list and department chips. Those need staff and org-unit NAMES, which this
//	                   service does not own (rule 2) — the join is an identity RPC, not SQL.
//	an update          §9.2 forbids editing an issued announcement, and migration 0005 enforces it
//	                   in a trigger. The only editable things left are `ghim` and the withdrawal,
//	                   and neither is in this pass.
//	a soft delete      nothing in chapter 08 deletes an announcement; §4's `🗑 Gỡ` WITHDRAWS it,
//	                   which is a state change rather than a deletion. The columns exist because
//	                   rule 7 makes them the shape of business data.
//	`Gửi cho tôi`      §2's default filter reads `thong_bao_nguoi_nhan` by `nguoi_nhan_ma`. It is
//	                   not shipped because the permission question behind the two filters is a stop
//	                   condition — see the read route — and an index for it would then be an index
//	                   with no reader.
