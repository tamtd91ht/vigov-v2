package store

// THE MINI APP CONTENT REGISTER — `noi_dung_mini_app` and `danh_muc_mini_app` (migration 0006).
// SQL, and nothing else.
//
// EVERY IDENTIFIER HERE CARRIES `NoiDungMiniApp` OR `DanhMucMiniApp`, and that is not a style choice.
// This package already holds `ThongBaoNoiBoStore` (the internal staff book), `ThongBaoGuiCongDanStore`
// (the citizen message ledger) and `LoaiTaiNguyenBanDoStore` (a reference catalogue). Two of those
// share one Vietnamese word with a VALUE of `noi_dung_mini_app.loai`, and the third is also a
// `danh_muc`. A name reused between them is one refactor away from putting an internal notice on a
// public channel — which on this surface is a disclosure, not a bug.
//
// FOUR THINGS HOLD IN EVERY STATEMENT IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1, from the context (rule 1, invariants 4 and 5). It is never a parameter of
//     any method here.
//  2. NOTHING HERE OPENS A TRANSACTION. The write methods take a *store.ScopedTx, because the audit
//     entry has to share it (rule 6, invariant 3) and a method that opened its own would make that
//     impossible for its caller. internal/app opens it.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) except SlugDaDung, which exists
//     precisely to see them — an issued code is never reissued.
//  4. EVERY VALUE IS A BOUND PARAMETER. The filter below generates `$n` placeholders from the
//     COUNT of the filters actually asked for and binds the values; nothing from a request is
//     concatenated into SQL.
//
// NO audit.Write IN THIS FILE, AND THAT IS NOT THE OMISSION IT LOOKS LIKE — audit_guard warns on
// exactly this shape, so the answer is written down rather than rediscovered, and it is the same
// answer the two files next door give. Rule 6 requires the entry to share the TRANSACTION, which it
// does: internal/app opens one, calls the methods below, and writes the entry inside the same
// *store.ScopedTx. What the entry cannot come from is here — `Actor` is who caused the write and
// `Action` is a business verb, and neither exists at the SQL layer.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// --- refusals shared by both tables of this module -----------------------------------------------

var (
	// ErrNoiDungKhongTonTai says the id addressed no live item OF THIS COMMUNE.
	//
	// The two causes are deliberately not distinguished: "no such item" and "that item belongs to
	// another commune" must look identical from outside, or the error itself becomes a way to
	// discover what another authority holds (rule 4, forbidden #2, applied to the commune axis).
	ErrNoiDungKhongTonTai = errors.New("noi_dung_mini_app: không có bản ghi")

	// ErrSlugDanhMucDaTonTai — the slug is already taken in this commune, SOFT-DELETED ROWS INCLUDED.
	//
	// Counting deleted rows is not strictness for its own sake: `UNIQUE (tenant_id, slug)` counts
	// them too, on purpose. A commune that could soft-delete `chuyen-doi-so` and create a new,
	// unrelated `chuyen-doi-so` would silently refile every article already filed under the old one.
	// Rule 7, invariant 3: an issued code is never reissued.
	ErrSlugDanhMucDaTonTai = errors.New("danh_muc_mini_app: slug đã được dùng trong xã này")

	// ErrDanhMucKhongTonTaiMiniApp — the category the item was filed under is not a live category of
	// this commune. The foreign key refuses it too; this turns the constraint violation into a
	// sentence naming the field.
	ErrDanhMucKhongTonTaiMiniApp = errors.New("danh_muc_mini_app: không có danh mục này trong xã")

	// ErrQuaNhieuDanhMucMiniApp — the commune is at the ceiling. See TranDanhMucMiniApp.
	ErrQuaNhieuDanhMucMiniApp = errors.New("danh_muc_mini_app: vượt trần danh mục")
)

// TranDanhMucMiniApp is the hard upper bound on one commune's Mini App category tree.
//
// WHY A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED. One process serves 200+ communes, so every list
// route is a shared resource and an unbounded one is forbidden (skills/rest-api-design §5,
// FORBIDDEN #4). The category read returns the WHOLE tree on purpose — a select and a filter need
// every option to be correct at all — so the bound cannot come from a `limit` parameter.
//
// THE SAME FIGURE AS TranDanhMucLoaiTaiNguyen, DELIBERATELY. §3's sample commune has 60 chuyên mục
// on its portal, which is the realistic ceiling of a commune's own tree as well, so 500 is roughly
// eight times it — the point past which the content is no longer a filing tree but an import run
// twice. Two ceilings in one service carrying two different numbers would invite the next reader to
// look for a meaning in the difference that is not there.
const TranDanhMucMiniApp = 500

// TuKhoaTimToiDa bounds §6's `Tìm theo tiêu đề…` box.
//
// THE SEARCH IS `ILIKE '%…%'` AND NO INDEX CAN SERVE IT — see menhDe below. The bound is therefore
// not about the string, it is about the work: a 4.000-character pattern matched against every live
// title of a commune is a request one client can make and every other commune on the process pays
// for. 200 is longer than any real search somebody types.
const TuKhoaTimToiDa = 200

// --- the content register ------------------------------------------------------------------------

// NoiDungMiniAppStore is the only path to `noi_dung_mini_app`.
//
// It is built from *store.DB and reaches the database only through Scoped — there is no field here
// holding a *sql.DB, and adding one would reopen the hole core/store exists to close (rule 1,
// invariant 5).
type NoiDungMiniAppStore struct {
	db *store.DB
}

func NewNoiDungMiniAppStore(db *store.DB) *NoiDungMiniAppStore {
	return &NoiDungMiniAppStore{db: db}
}

// cotNoiDungMiniApp IS READ BY POSITION in quetNoiDungMiniApp.
//
// `noi_dung` IS DELIBERATELY NOT IN IT, WHICH IS THE OPPOSITE OF WHAT THE ANNOUNCEMENT BOOK DOES
// WITH ITS BODY COLUMN — and the difference is the size, not an inconsistency. An announcement is
// capped at 20.000 runes; an article here is capped at 200.000 (domain.ThanNoiDungToiDa), because
// §8 says HTML and a portal article carries markup. page.MaxLimit caps a page at 100, so a list
// carrying the body would have a worst case of twenty million runes in one response. §6's table
// shows a title and a one-line summary and needs neither.
//
// THE BODY IS READ BY THE DETAIL METHOD, AND IT IS APPENDED AT THE TAIL of the column list rather
// than inserted anywhere in it — see cotNoiDungMiniAppChiTiet. That is what lets ONE scan function
// serve both reads, so the two cannot drift apart.
//
// `nguon`, `nguon_url` AND `nguon_id_ngoai` ARE THREE ADJACENT NULLABLE-ISH TEXT COLUMNS. A swap
// between any two of them in either this list or the scan produces no error at all: it produces an
// article that claims to come from somewhere it did not, which is exactly the fact §10.4 protects.
// TestPgCotNoiDungMiniAppKhopVoiLuocDoThat asserts this list against the real schema BY NAME.
const cotNoiDungMiniApp = `id, loai, danh_muc_id, tieu_de, tom_tat, anh_dai_dien_url, ` +
	`ngay_dang, luot_xem, trang_thai, nguon, nguon_url, nguon_id_ngoai, da_sua_tay, ` +
	`nguoi_tao_ma, tao_luc, cap_nhat_luc`

// cotNoiDungMiniAppChiTiet adds the body AT THE END. Nowhere else: the shared scan appends one
// destination when it is asked for the body, and an insertion anywhere but the tail would shift every
// column after it, silently.
const cotNoiDungMiniAppChiTiet = cotNoiDungMiniApp + `, noi_dung`

// SapXepNoiDungMiniApp is the closed set of sorts GET /api/v1/content-items offers.
//
// ONE COLUMN, AND `ngay_dang` IS DELIBERATELY NOT THE SECOND although §6 prints it. A sort column has
// to be NOT NULL — both are — but it also has to SEPARATE rows, and `ngay_dang` is a DATE: one sync
// run importing four hundred articles published the same day gives four hundred rows the same sort
// value, so the `id` tie-break silently carries the whole order and the list stops being ordered by
// what its header says. `tao_luc` is a timestamp, is NOT NULL, and is indexed
// (`noi_dung_mini_app_so`, migration 0006).
//
// OFFERING BOTH WOULD COST A SECOND INDEX WITH NO READER: §6 states no sort order at all, so a
// second option today is a guess with a write cost attached. Reported rather than approximated.
var SapXepNoiDungMiniApp = page.NewAllowlist(page.Desc,
	page.Col("created_at", "tao_luc", page.KindTime),
)

// mocNoiDungMiniApp BINDS the allowlisted sort column to the way its cursor value is read out of a
// scanned row. store.NewMoc compares the two lists AT CONSTRUCTION, so a sort added above without a
// reader here is a panic at startup rather than a wrong page order after release.
var mocNoiDungMiniApp = store.NewMoc[domain.NoiDungMiniApp](SapXepNoiDungMiniApp,
	map[string]func(domain.NoiDungMiniApp) page.Key{
		"created_at": func(n domain.NoiDungMiniApp) page.Key { return page.TimeKey(n.TaoLuc) },
	})

// LocNoiDung is §6's filter bar: the six tabs, the category select and the title box.
//
// AN EMPTY FIELD MEANS "DO NOT FILTER ON THIS", never "match the empty value". That distinction is
// the whole reason this is a struct of strings rather than three arguments: `Tất cả danh mục` and
// `— Chưa xếp danh mục —` are two different requests, and the second one is NOT expressible here on
// purpose — §6 offers no such option, and inventing one would put a filter on the screen that the
// screen does not have.
type LocNoiDung struct {
	// Loai is one of §5's six codes. A value outside them is refused by the caller before it reaches
	// here; an unknown code would simply match nothing, which reads on screen as an empty tab.
	Loai string

	// DanhMucID is §6's `Tất cả danh mục ▾`.
	DanhMucID string

	// Tu is §6's `🔍 Tìm theo tiêu đề…`. Title only — §6 says so, and searching the BODY would put
	// the HTML of every article through a pattern match on every keystroke.
	Tu string
}

// thoatLike escapes the three characters PostgreSQL's LIKE treats specially.
//
// WITHOUT IT, `%` TYPED IN THE SEARCH BOX MATCHES EVERYTHING and `_` matches any character — so a
// resident's name containing an underscore, or a careless paste, silently returns rows the person
// did not ask for. `\` goes first: escaping it after the other two would escape the backslashes this
// function just added. PostgreSQL's default LIKE escape character is `\`, so no ESCAPE clause is
// needed.
var thoatLike = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// menhDe builds the filter tail and its arguments.
//
// THE PLACEHOLDERS START AT $2 BECAUSE $1 IS ALWAYS THE COMMUNE — core/store.Scoped.Query owns $1
// and store.QueryPage numbers the caller's own arguments from $2 in the order they appear in
// PageSpec.Args. The counter below is therefore not cosmetic: a mismatch between the numbering here
// and the order of the slice binds the title pattern to the category and returns an empty list for
// every request, with no error anywhere.
//
// `deleted_at IS NULL` IS FIRST AND UNCONDITIONAL (rule 7, invariant 2). Both partial indexes of
// migration 0006 are built on exactly this predicate.
//
// THE TITLE SEARCH CANNOT USE AN INDEX AND THAT IS STATED RATHER THAN HOPED ABOUT: `ILIKE '%x%'` is
// a leading wildcard, so it is a scan of the commune's live rows however the columns are indexed.
// The partition restricts it to one commune and TuKhoaTimToiDa bounds the pattern; if a commune's
// register ever makes that too slow, the answer is a trigram index or a text-search column in a new
// migration, not a silently truncated result here.
func (l LocNoiDung) menhDe() (string, []any) {
	var b strings.Builder
	b.WriteString(` AND deleted_at IS NULL`)

	args := make([]any, 0, 3)
	n := 2 // $1 is the commune
	if l.Loai != "" {
		fmt.Fprintf(&b, " AND loai = $%d", n)
		args = append(args, l.Loai)
		n++
	}
	if l.DanhMucID != "" {
		fmt.Fprintf(&b, " AND danh_muc_id = $%d", n)
		args = append(args, l.DanhMucID)
		n++
	}
	if tu := strings.TrimSpace(l.Tu); tu != "" {
		fmt.Fprintf(&b, " AND tieu_de ILIKE $%d", n)
		args = append(args, "%"+thoatLike.Replace(tu)+"%")
		n++
	}
	return b.String(), args
}

// DanhSach reads ONE PAGE of §6's table.
//
// ONE STATEMENT AND NO SECOND ONE. Unlike the announcement book next door, nothing on this row is an
// aggregate: §6's `👁 {n}` is a column, and `🔗 Có ảnh` is derived in Go from `anh_dai_dien_url`
// (domain.NoiDungMiniApp.CoAnh). A counter query here would be work with no reader.
//
// THE BODY IS NOT IN THE RESULT. Every item comes back with `NoiDung` empty — see cotNoiDungMiniApp
// for why, and TheoID for where the body is read. A caller that renders the body from a list item
// renders nothing, which is visible; a list that carried it would be a response nobody notices until
// a commune has real articles in it.
func (s *NoiDungMiniAppStore) DanhSach(ctx context.Context, loc LocNoiDung, yc page.Request) (
	page.Result[domain.NoiDungMiniApp], error) {

	tail, args := loc.menhDe()

	kq, err := store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotNoiDungMiniApp,
		Table:   "noi_dung_mini_app",
		// The commune is bound to $1 by store.QueryPage through Scoped.Query — `tenant_id` is never
		// a value this file supplies.
		Filter: tail,
		Args:   args,
	}, yc, mocNoiDungMiniApp, func(rows *sql.Rows) (domain.NoiDungMiniApp, string, error) {
		n, err := quetNoiDungMiniApp(rows, false)
		if err != nil {
			return domain.NoiDungMiniApp{}, "", err
		}
		return n, n.ID, nil
	})
	if err != nil {
		return kq, err
	}
	return kq, nil
}

// TheoID reads ONE live item of THIS COMMUNE, body included — §7's modal reopened by §6's `✎`.
//
// IT IS THE ONLY READ THAT RETURNS `noi_dung`. A detail route exists precisely because the list does
// not carry it (see cotNoiDungMiniApp), so the two are not two ways of asking one question.
func (s *NoiDungMiniAppStore) TheoID(ctx context.Context, id string) (domain.NoiDungMiniApp, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotNoiDungMiniAppChiTiet, "noi_dung_mini_app",
		`AND id = $2 AND deleted_at IS NULL`, id)
	if err != nil {
		return domain.NoiDungMiniApp{}, fmt.Errorf("noi_dung_mini_app: đọc bản ghi: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.NoiDungMiniApp{}, fmt.Errorf("noi_dung_mini_app: đọc bản ghi: %w", err)
		}
		return domain.NoiDungMiniApp{}, ErrNoiDungKhongTonTai
	}
	n, err := quetNoiDungMiniApp(rows, true)
	if err != nil {
		return domain.NoiDungMiniApp{}, err
	}
	return n, nil
}

// quetMotDongNoiDung is what both *sql.Rows and *sql.Row satisfy, so one scan serves every read.
type quetMotDongNoiDung interface{ Scan(...any) error }

// quetNoiDungMiniApp reads one row of cotNoiDungMiniApp, plus the body when it was selected.
//
// POSITIONAL, IN LOCKSTEP WITH cotNoiDungMiniApp. database/sql binds by POSITION, so a destination
// inserted or removed anywhere but the tail silently shifts every column after it — and the three
// adjacent provenance columns would shift into each other without any error at all.
//
// EVERY NULLABLE COLUMN GOES THROUGH sql.NullString. Scanning a NULL straight into a string is a
// runtime error, and five of these columns are NULL on the commonest row there is: a hand-composed
// article with no category, no summary, no image and no portal origin.
func quetNoiDungMiniApp(r quetMotDongNoiDung, coThan bool) (domain.NoiDungMiniApp, error) {
	var (
		n                      domain.NoiDungMiniApp
		loai, trang, nguon     string
		danhMuc, tomTat, anh   sql.NullString
		nguonURL, nguonIDNgoai sql.NullString
		than                   sql.NullString
	)
	dich := []any{
		&n.ID, &loai, &danhMuc, &n.TieuDe, &tomTat, &anh,
		&n.NgayDang, &n.LuotXem, &trang, &nguon, &nguonURL, &nguonIDNgoai, &n.DaSuaTay,
		&n.NguoiTaoMa, &n.TaoLuc, &n.CapNhatLuc,
	}
	if coThan {
		dich = append(dich, &than)
	}
	if err := r.Scan(dich...); err != nil {
		return domain.NoiDungMiniApp{}, fmt.Errorf("noi_dung_mini_app: đọc dòng: %w", err)
	}

	n.Loai = domain.LoaiNoiDung(loai)
	n.TrangThai = domain.TrangThaiNoiDung(trang)
	n.Nguon = domain.NguonNoiDung(nguon)
	n.DanhMucID = danhMuc.String
	n.TomTat = tomTat.String
	n.AnhDaiDienURL = anh.String
	n.NguonURL = nguonURL.String
	n.NguonIDNgoai = nguonIDNgoai.String
	n.NoiDung = than.String
	return n, nil
}

// --- the content write path ------------------------------------------------------------------------

// TheoIDDeSua reads one live item inside the transaction and holds it until the transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. The edit is a read-decide-write:
// read the row, work out whether §10.4's `da_sua_tay` has to be set, apply. Without the lock two
// members of staff editing the same article both read the old state and the second write silently
// overwrites the first — including the case where one of them was unpublishing it.
//
// IT READS THE BODY TOO, because a partial edit that does not mention `noi_dung` has to write back
// the body that is already there.
func (s *NoiDungMiniAppStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (
	domain.NoiDungMiniApp, error) {

	const stmt = `SELECT ` + cotNoiDungMiniAppChiTiet + ` FROM noi_dung_mini_app ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	n, err := quetNoiDungMiniApp(
		tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id), true)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NoiDungMiniApp{}, ErrNoiDungKhongTonTai
	}
	if err != nil {
		// quetNoiDungMiniApp already wrapped anything that is not ErrNoRows.
		return domain.NoiDungMiniApp{}, err
	}
	return n, nil
}

// DanhMucCoThat reports whether the id names a LIVE category of this commune.
//
// THE FOREIGN KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. The constraint never lets an
// article point at a category that does not exist, and it would answer with a driver's exception and
// a 500. This turns the ordinary case — a stale select on a screen somebody left open — into a
// sentence naming the field.
//
// IT EXCLUDES SOFT-DELETED CATEGORIES, WHICH THE FOREIGN KEY DOES NOT. Filing a new article under a
// category the commune retired last week is a mistake the constraint cannot see, because the row is
// still there.
func (s *NoiDungMiniAppStore) DanhMucCoThat(ctx context.Context, tx *store.ScopedTx, id string) (bool, error) {
	const stmt = `SELECT 1 FROM danh_muc_mini_app ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	var mot int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan(&mot)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("danh_muc_mini_app: kiểm tra danh mục: %w", err)
	}
	return true, nil
}

// chenNoiDungMiniApp records one item.
//
// `nguon` IS THE LITERAL 'thu-cong' AND `nguon_id_ngoai` / `nguon_url` / `da_sua_tay` APPEAR NOWHERE
// IN THIS STATEMENT. Read that as the property it is, not as a shortcut: there is no $n for any of
// them, so there is no value any layer above could pass and no field a client could fill. Every item
// this service composes is the commune's own. The sync's writer, when it exists, is a DIFFERENT
// statement — and it has to be, because §10.4's protection is exactly the difference between the
// two.
//
// `luot_xem` IS ABSENT TOO: an item is born with nobody having read it, and a parameter there is a
// parameter a request could eventually reach.
//
// `deleted_at`, `deleted_by`, `delete_reason` APPEAR NOWHERE EITHER: a row is not born deleted.
//
// The statement names `tenant_id` first, as every write in this system does: the commune is not an
// argument the caller chooses, it is bound from the transaction (rule 1, invariants 4 and 5).
const chenNoiDungMiniApp = `INSERT INTO noi_dung_mini_app
	(tenant_id, id, loai, danh_muc_id, tieu_de, tom_tat, noi_dung, anh_dai_dien_url,
	 ngay_dang, trang_thai, nguon, nguoi_tao_ma)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'thu-cong',$11)`

// Chen writes one item. The caller owns the transaction and the audit entry inside it.
func (s *NoiDungMiniAppStore) Chen(ctx context.Context, tx *store.ScopedTx,
	n domain.NoiDungMiniApp) error {

	// EMPTY STRINGS BECOME NULL, and that is not tidiness: `danh_muc_id` has a foreign key, and ''
	// is not a category id — it is a value the constraint would refuse. The other three are nullable
	// text where '' and NULL would be two spellings of "nothing", and a read path that had to handle
	// both would handle one of them wrongly.
	if _, err := tx.Exec(ctx, chenNoiDungMiniApp, string(tx.TenantID()), n.ID,
		string(n.Loai), rongThanhNull(n.DanhMucID), n.TieuDe, rongThanhNull(n.TomTat),
		rongThanhNull(n.NoiDung), rongThanhNull(n.AnhDaiDienURL), n.NgayDang.UTC(),
		string(n.TrangThai), n.NguoiTaoMa); err != nil {
		return fmt.Errorf("noi_dung_mini_app: chèn: %w", err)
	}
	return nil
}

// capNhatNoiDungMiniApp — `nguon`, `nguon_id_ngoai`, `nguoi_tao_ma`, `tao_luc`, `luot_xem` AND THE
// SOFT-DELETE COLUMNS APPEAR NOWHERE IN THIS STATEMENT.
//
// All six are also refused by the trigger in migration 0006, and both layers are meant: the trigger
// is the floor that holds against every writer, and their absence here is what makes the floor
// unreachable from this service in the first place.
//
// `da_sua_tay` IS THE ONE PROVENANCE COLUMN THIS STATEMENT DOES WRITE, and only ever upward — the
// caller computes it as `truoc.DaSuaTay OR nguon = 'dong-bo-cong'`, and the trigger refuses to clear
// it. §10.4 is a promise to a member of staff that their correction survives the next sync, and this
// is where the promise is recorded.
const capNhatNoiDungMiniApp = `UPDATE noi_dung_mini_app
	SET loai = $3, danh_muc_id = $4, tieu_de = $5, tom_tat = $6, noi_dung = $7,
	    anh_dai_dien_url = $8, trang_thai = $9, da_sua_tay = $10, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// CapNhat writes the fields §7's modal may change. The caller has already read the row with
// TheoIDDeSua and merged the partial request onto it.
func (s *NoiDungMiniAppStore) CapNhat(ctx context.Context, tx *store.ScopedTx,
	n domain.NoiDungMiniApp) error {

	kq, err := tx.Exec(ctx, capNhatNoiDungMiniApp, string(tx.TenantID()), n.ID,
		string(n.Loai), rongThanhNull(n.DanhMucID), n.TieuDe, rongThanhNull(n.TomTat),
		rongThanhNull(n.NoiDung), rongThanhNull(n.AnhDaiDienURL), string(n.TrangThai), n.DaSuaTay)
	if err != nil {
		return fmt.Errorf("noi_dung_mini_app: cập nhật: %w", err)
	}
	return doiMotDongNoiDung(kq, "cập nhật")
}

// rongThanhNull turns "" into a NULL bind. See the note on Chen for why the distinction is load
// bearing on `danh_muc_id` in particular.
func rongThanhNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// doiMotDongNoiDung turns "nothing was updated" into ErrNoiDungKhongTonTai.
//
// WHY IT IS CHECKED AT ALL WHEN THE CALLER ALREADY READ THE ROW UNDER A LOCK: the read and the write
// are two statements, and a method used without the read — a future caller, a retry path — would
// otherwise report success for a row that does not exist. An UPDATE touching zero rows is not an
// error to PostgreSQL; it is only an error to us.
func doiMotDongNoiDung(kq sql.Result, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("noi_dung_mini_app: %s: đọc số dòng: %w", viec, err)
	}
	if n == 0 {
		return ErrNoiDungKhongTonTai
	}
	return nil
}

// --- the category tree ------------------------------------------------------------------------------

// DanhMucMiniAppStore is the only path to `danh_muc_mini_app`.
type DanhMucMiniAppStore struct {
	db *store.DB
}

func NewDanhMucMiniAppStore(db *store.DB) *DanhMucMiniAppStore {
	return &DanhMucMiniAppStore{db: db}
}

// cotDanhMucMiniApp IS READ BY POSITION in quetDanhMucMiniApp. `ten` and `slug` are adjacent TEXT
// columns and `id` and `cha_id` are two more: swapping either pair compiles, runs, and produces a
// tree whose every node is its own parent or whose every label is a slug.
const cotDanhMucMiniApp = `id, ten, slug, cha_id, thu_tu, tao_luc`

// DanhSach reads the commune's WHOLE category tree, ordered.
//
// NOT PAGINATED, AND THAT IS A DECISION. The same three reasons hold as for the map-asset-type
// catalogue next door: it is a closed filing tree of tens of rows, its consumers (§7's select, §6's
// filter) need the whole thing to be correct at all, and a client that must follow cursors to fill a
// dropdown will not. The bound pagination would have provided is provided by TranDanhMucMiniApp.
//
// ORDER BY thu_tu, slug: `thu_tu` is the order the commune arranged its own tree in, and `slug`
// breaks ties. The tie-break is the slug rather than the name because `UNIQUE (tenant_id, slug)`
// makes it a TOTAL order — two categories can share a name, and an order that is not total lets two
// calls return the same rows in a different sequence.
//
// THE TREE IS RETURNED FLAT, with `cha_id` on each row. Assembling it is the client's job: §7 draws a
// select and §3 draws an indented list, and the two want different shapes of the same data.
func (s *DanhMucMiniAppStore) DanhSach(ctx context.Context) ([]domain.DanhMucMiniApp, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page indistinguishable from a complete list
	// of exactly that size — the truncation this route refuses to perform, performed by the bound
	// meant to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotDanhMucMiniApp, "danh_muc_mini_app",
		`AND deleted_at IS NULL ORDER BY thu_tu, slug LIMIT $2`, TranDanhMucMiniApp+1)
	if err != nil {
		return nil, fmt.Errorf("danh_muc_mini_app: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.DanhMucMiniApp, 0, 16)
	for rows.Next() {
		dm, err := quetDanhMucMiniApp(rows)
		if err != nil {
			return nil, err
		}
		ra = append(ra, dm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh_muc_mini_app: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucMiniApp {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list the
		// caller might render anyway is how a refusal turns back into a silent truncation, one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuDanhMucMiniApp
	}
	return ra, nil
}

func quetDanhMucMiniApp(r quetMotDongNoiDung) (domain.DanhMucMiniApp, error) {
	var (
		dm  domain.DanhMucMiniApp
		cha sql.NullString
	)
	if err := r.Scan(&dm.ID, &dm.Ten, &dm.Slug, &cha, &dm.ThuTu, &dm.TaoLuc); err != nil {
		return domain.DanhMucMiniApp{}, fmt.Errorf("danh_muc_mini_app: đọc dòng: %w", err)
	}
	dm.ChaID = cha.String
	return dm, nil
}

// SlugDaDung reports whether this slug is already taken in this commune — INCLUDING soft-deleted
// rows.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two concurrent creates of the same
// slug can both pass this check and the second will then hit `UNIQUE (tenant_id, slug)` and roll the
// whole transaction back — no duplicate row, an unhelpful 500. That is the correct trade: the
// constraint never lets the duplicate exist, and this turns the ordinary case into a sentence
// somebody can act on.
func (s *DanhMucMiniAppStore) SlugDaDung(ctx context.Context, tx *store.ScopedTx, slug string) (bool, error) {
	// `deleted_at` DELIBERATELY ABSENT FROM THE PREDICATE. See ErrSlugDanhMucDaTonTai: an issued code
	// is never reissued (rule 7, invariant 3), so a deleted row still owns its slug.
	const stmt = `SELECT count(*) FROM danh_muc_mini_app WHERE tenant_id = $1 AND slug = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), slug).Scan(&n); err != nil {
		return false, fmt.Errorf("danh_muc_mini_app: kiểm tra slug trùng: %w", err)
	}
	return n > 0, nil
}

// DemDangSong counts the commune's live categories, for the ceiling check. Soft-deleted rows are
// excluded because DanhSach excludes them: the two numbers have to mean the same thing or the check
// guards the wrong quantity.
func (s *DanhMucMiniAppStore) DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM danh_muc_mini_app WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt).Scan(&n); err != nil {
		return 0, fmt.Errorf("danh_muc_mini_app: đếm danh mục: %w", err)
	}
	return n, nil
}

// ChaCoThat reports whether the parent id names a LIVE category of this commune.
//
// THE FOREIGN KEY DOES NOT COVER THIS CASE COMPLETELY: it refuses a parent that does not exist, but
// a SOFT-DELETED parent is still a row, so without this check a commune could file a new category
// under one it retired — and §7's select, which excludes deleted rows, would then show a child whose
// parent is nowhere on the screen.
func (s *DanhMucMiniAppStore) ChaCoThat(ctx context.Context, tx *store.ScopedTx, chaID string) (bool, error) {
	const stmt = `SELECT 1 FROM danh_muc_mini_app ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	var mot int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), chaID).Scan(&mot)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("danh_muc_mini_app: kiểm tra danh mục cha: %w", err)
	}
	return true, nil
}

// chenDanhMucMiniApp records one category.
//
// THE SOFT-DELETE COLUMNS APPEAR NOWHERE: a category is not born deleted.
const chenDanhMucMiniApp = `INSERT INTO danh_muc_mini_app
	(tenant_id, id, ten, slug, cha_id, thu_tu)
	VALUES ($1,$2,$3,$4,$5,$6)`

// Chen writes one category. The caller owns the transaction and the audit entry inside it.
func (s *DanhMucMiniAppStore) Chen(ctx context.Context, tx *store.ScopedTx,
	dm domain.DanhMucMiniApp) error {

	if _, err := tx.Exec(ctx, chenDanhMucMiniApp, string(tx.TenantID()), dm.ID,
		dm.Ten, dm.Slug, rongThanhNull(dm.ChaID), dm.ThuTu); err != nil {
		return fmt.Errorf("danh_muc_mini_app: chèn: %w", err)
	}
	return nil
}

// --- what is deliberately absent ---------------------------------------------------------------
//
// NO `XoaMem` ON EITHER TABLE, AND NO CATEGORY UPDATE. Each absence is a decision:
//
//	deleting an item      §6's action column offers `✎` and NOTHING ELSE. §9 proposes a DELETE, but
//	                      rule 7 makes that a soft delete with a MANDATORY reason (`delete_reason`),
//	                      and no screen in chapter 11 collects one. Building the route would mean
//	                      deciding what the reason says, which is the customer's sentence to write.
//	                      Taking an item off the Mini App is `trang_thai = 'an'`, which the edit
//	                      route already does.
//	editing a category    §6's `⊞ Danh mục tin` button implies management, and §9 lists only
//	                      GET/POST. Renaming is harmless; RE-PARENTING is not — it is the one write
//	                      that can create a cycle through two or more rows, which no CHECK can refuse
//	                      (migration 0006 says so) and which makes every tree walk in this module
//	                      loop forever. That route arrives with its ancestor walk, not before it.
//	deleting a category   a category holding articles cannot simply go: the foreign key refuses it,
//	                      and what should happen to the articles is a question §11 does not answer.
//	the sync writer       see migration 0006. Nothing in this repository can hold the portal's API
//	                      key (ADR 0009, decision 7 — `core/crypto` does not exist), schedule a run,
//	                      or make the call.
//	a citizen read        §9's `/api/cong/mini-app/noi-dung`. See internal/http for the three
//	                      separate reasons it is not built.
