package app

// COMPOSING AND EDITING MINI APP CONTENT — the write half of `docs/ui-ux/11-noi-dung-mini-app.md`.
//
// WHY THIS LAYER EXISTS FOR WHAT LOOKS LIKE ONE INSERT AND ONE UPDATE: rule 6, invariant 3 requires
// the audit entry to share a transaction with the business write, and core/audit.Write takes a
// *store.ScopedTx with no overload that writes outside one. Opening that transaction is this layer's
// job. The handler translates HTTP and nothing else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THAT EVERY WRITE HERE IS READ-DECIDE-WRITE. An edit has to read the row under
// a lock, work out whether §10.4's `da_sua_tay` has to be set, and refuse a category that is not
// there — and that needs the row AND the rules AND the transaction at once, which is exactly one
// place.
//
// NAMES: `SoanThongBaoNoiBo`, `KhoThongBaoNoiBo` and `DanhMucLoaiTaiNguyen` are already taken in this
// package by the internal announcement book and the map-asset-type catalogue. Everything here
// carries `NoiDungMiniApp` or `DanhMucMiniApp`.
//
// WHAT THIS FILE DELIBERATELY DOES NOT DO: run the portal synchronisation of §3 and §10.1–10.3. It
// cannot, and the reasons are not effort — migration 0006 lists all three (no `core/crypto` to hold
// the commune's portal key, ADR 0009 decision 7; no scheduler; no outbound HTTP adapter). What this
// file DOES carry from that half is §10.4, because that rule is about an EDIT rather than about the
// job: see Sua.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// KhoNoiDungMiniApp is the content store, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the row in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry shares
// the transaction, that `nguon` is never written from a request — provable without a PostgreSQL. A
// test that needs infrastructure is a test that stops being run.
type KhoNoiDungMiniApp interface {
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.NoiDungMiniApp, error)
	DanhMucCoThat(ctx context.Context, tx *store.ScopedTx, id string) (bool, error)
	Chen(ctx context.Context, tx *store.ScopedTx, n domain.NoiDungMiniApp) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, n domain.NoiDungMiniApp) error
}

// KhoDanhMucMiniApp is the category store. A SECOND INTERFACE rather than four more methods on the
// first, because they guard two different tables with two different refusals — and a caller reaching
// for whichever method was nearest is how an article ends up checked against a category ceiling.
type KhoDanhMucMiniApp interface {
	SlugDaDung(ctx context.Context, tx *store.ScopedTx, slug string) (bool, error)
	DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error)
	ChaCoThat(ctx context.Context, tx *store.ScopedTx, chaID string) (bool, error)
	Chen(ctx context.Context, tx *store.ScopedTx, dm domain.DanhMucMiniApp) error
}

// SoanNoiDungMiniApp owns composing and editing one commune's Mini App content.
type SoanNoiDungMiniApp struct {
	db      *store.DB
	kho     KhoNoiDungMiniApp
	khoDanh KhoDanhMucMiniApp

	// sinhID and bayGio are injected so a test can pin both. In production they are ulid.Moi and
	// time.Now. The clock is injectable because `ngay_dang` is the date §6 prints, and a test that
	// could not fix it could only assert that it is "roughly today" — the assertion that passes when
	// the value is wrong.
	sinhID func() (string, error)
	bayGio func() time.Time
}

func NewSoanNoiDungMiniApp(db *store.DB, kho KhoNoiDungMiniApp, khoDanh KhoDanhMucMiniApp) *SoanNoiDungMiniApp {
	return &SoanNoiDungMiniApp{db: db, kho: kho, khoDanh: khoDanh, sinhID: ulid.Moi, bayGio: time.Now}
}

// DanhMucNoiDungMiniApp owns adding categories to one commune's Mini App filing tree.
type DanhMucNoiDungMiniApp struct {
	db  *store.DB
	kho KhoDanhMucMiniApp

	sinhID func() (string, error)
}

func NewDanhMucNoiDungMiniApp(db *store.DB, kho KhoDanhMucMiniApp) *DanhMucNoiDungMiniApp {
	return &DanhMucNoiDungMiniApp{db: db, kho: kho, sinhID: ulid.Moi}
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `phat_hanh_thong_bao`): an inspection reads these strings, and
// a function name would tell them nothing.
//
// THE VERB NAMES THE TABLE AS WELL AS THE OPERATION. `them_noi_dung` across a service that also holds
// an announcement book and a catalogue would make the trail unable to answer WHICH register changed
// without joining to a row that may since have been edited again.
const (
	HanhViThemNoiDungMiniApp = "them_noi_dung_mini_app"
	HanhViSuaNoiDungMiniApp  = "sua_noi_dung_mini_app"
	HanhViThemDanhMucMiniApp = "them_danh_muc_mini_app"
)

var (
	// ErrThieuNguoiTaoNoiDung refuses the write when the request carries no staff business code.
	//
	// core/audit refuses an entry with no actor for the same reason: an article nobody can be asked
	// about is an article with no author, on a channel where every resident of the commune reads it.
	// There is NO FALLBACK to an internal id — rule 6, invariant 8, and the fallback is what put
	// ULIDs in `audit_log.actor_id` six times on 2026-09-22.
	ErrThieuNguoiTaoNoiDung = errors.New("noi_dung_mini_app: thiếu mã cán bộ soạn")

	// ErrDanhMucChaKhongTonTai refuses a category filed under a parent that is not a live category of
	// this commune. The foreign key refuses a parent that does not exist AT ALL; this also catches a
	// SOFT-DELETED parent, which is still a row and which the constraint therefore accepts.
	ErrDanhMucChaKhongTonTai = errors.New("danh_muc_mini_app: không có danh mục cha này trong xã")
)

// Them composes one new item of content.
//
// # WHAT IT DOES NOT DO, IN ORDER OF HOW LIKELY SOMEBODY IS TO LOOK FOR IT
//
//	no attachment upload  §7's `Ảnh đại diện · Chọn tệp từ máy · JPG, PNG hoặc WebP — tối đa 50MB`.
//	                      There is no `core/storage` in this repository, so nothing here can accept
//	                      bytes. `anh_dai_dien_url` holds a link to a file some other system serves,
//	                      and §10.6's size and format rules belong to the uploader that does not
//	                      exist yet — writing them here would be validating a request nobody sends.
//	no per-type fields    §7's closing note wants a video URL, an audio file and a duration, an event
//	                      time and place, a banner link and order. It says "nên bổ sung", §8's data
//	                      model carries none of them, and migration 0006 argues why a suggestion does
//	                      not become an applied migration.
//	no approval step      `cho-duyet` is reachable only from the sync (§10.2). A commune composing by
//	                      hand publishes or does not, which is exactly the one checkbox §7 offers.
//
// # THE AUTHOR COMES FROM THE SESSION AND FROM NOWHERE ELSE
//
// `nguoi.ID` carries the staff BUSINESS CODE — the handler builds it from `authz.Principal.Ma`
// (rule 6, invariant 8). There is no `NguoiTaoMa` field on the request, because a field a body can
// fill is one member of staff publishing over a colleague's name, on a public channel.
func (uc *SoanNoiDungMiniApp) Them(ctx context.Context, yc domain.YeuCauThemNoiDung,
	nguoi audit.Actor) (domain.NoiDungMiniApp, error) {

	// VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never hold a
	// transaction open while doing so, and the caller needs the reason rather than a rollback.
	sach, err := yc.KiemTra()
	if err != nil {
		return domain.NoiDungMiniApp{}, err
	}
	if nguoi.ID == "" {
		return domain.NoiDungMiniApp{}, ErrThieuNguoiTaoNoiDung
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.NoiDungMiniApp{}, fmt.Errorf("noi_dung_mini_app: sinh mã: %w", err)
	}

	// ONE CLOCK READING FOR THE WHOLE ACT, and `ngay_dang` is its DATE part in UTC. §7 collects no
	// date, so a hand-composed item is dated the day it was composed; the column stays editable in
	// the schema only because a future sync must carry the portal's own publication date.
	luc := uc.bayGio().UTC()
	ngayDang := time.Date(luc.Year(), luc.Month(), luc.Day(), 0, 0, 0, 0, time.UTC)

	moi := domain.NoiDungMiniApp{
		ID:            id,
		Loai:          domain.LoaiNoiDung(sach.Loai),
		DanhMucID:     sach.DanhMucID,
		TieuDe:        sach.TieuDe,
		TomTat:        sach.TomTat,
		NoiDung:       sach.NoiDung,
		AnhDaiDienURL: sach.AnhDaiDienURL,
		NgayDang:      ngayDang,
		LuotXem:       0,
		// THE STATE IS DECIDED HERE FROM §7'S CHECKBOX, never taken from the request. See
		// domain.YeuCauThemNoiDung: a request that could name its own state could publish past
		// whatever approval a commune later introduces, or claim `cho-duyet`, which §10.2 reserves
		// for the sync.
		TrangThai: trangThaiTu(sach.DangLenMiniApp),
		// Set here only so the value this function RETURNS describes the row that was written. The
		// store does not read them: it writes 'thu-cong' as a literal and binds nothing for the other
		// three (migration 0006 and store.chenNoiDungMiniApp both say why).
		Nguon:      domain.NguonThuCong,
		DaSuaTay:   false,
		NguoiTaoMa: nguoi.ID,
		TaoLuc:     luc,
		CapNhatLuc: luc,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if moi.DanhMucID != "" {
			co, err := uc.kho.DanhMucCoThat(ctx, tx, moi.DanhMucID)
			if err != nil {
				return err
			}
			if !co {
				return commsstore.ErrDanhMucKhongTonTaiMiniApp
			}
		}
		if err := uc.kho.Chen(ctx, tx, moi); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": tomTatNoiDungMiniApp(moi)})
		if err != nil {
			return fmt.Errorf("noi_dung_mini_app: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides which
		// commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViThemNoiDungMiniApp,
			Subject: chuDeNoiDungMiniApp(moi),
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no item, no trail. The two agree.
		return domain.NoiDungMiniApp{}, bocNoiDung(ctx, "thêm nội dung Mini App", err)
	}
	return moi, nil
}

// Sua applies a partial edit — §6's `✎` reopening §7's modal.
//
// # §10.4 IS ENFORCED HERE AND IT IS THE ONE RULE OF THIS FILE WORTH READING TWICE
//
// "Bài do cán bộ sửa tay thì lần đồng bộ sau KHÔNG ghi đè (đặt cờ `da_sua_tay`)." The flag is set by
// THIS act, on any item whose provenance is the portal, and migration 0006 refuses to clear it
// afterwards. It is a promise to a person: somebody corrected a portal article's title, and a sync
// running every six hours would otherwise undo that correction silently, over and over, while the
// person who made it has no way to tell.
//
// IT IS SET AFTER THE NO-OP CHECK, NOT BEFORE. A request that changes nothing is not an edit, so it
// must not claim the protection either — otherwise opening the modal and pressing save without
// touching anything would take an article permanently out of the sync's reach.
//
// # A NO-OP WRITES NOTHING AND AUDITS NOTHING
//
// Sending back the values a row already has is not an event; recording it would fill a public
// authority's ledger with entries saying nothing changed, and those are the entries that bury the
// ones carrying legal weight. It is also what makes this route genuinely idempotent, which is what
// its `idem.KhongCan` declaration claims.
func (uc *SoanNoiDungMiniApp) Sua(ctx context.Context, id string, yc domain.YeuCauSuaNoiDung,
	nguoi audit.Actor) (domain.NoiDungMiniApp, error) {

	if id == "" {
		return domain.NoiDungMiniApp{}, commsstore.ErrNoiDungKhongTonTai
	}
	// Shape first, outside the transaction. A request that fails its shape must never hold a row
	// lock while doing so.
	sach, err := yc.KiemTra()
	if err != nil {
		return domain.NoiDungMiniApp{}, err
	}
	if nguoi.ID == "" {
		return domain.NoiDungMiniApp{}, ErrThieuNguoiTaoNoiDung
	}

	var sau domain.NoiDungMiniApp
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}

		sau = truoc
		if sach.Loai != nil {
			sau.Loai = domain.LoaiNoiDung(*sach.Loai)
		}
		if sach.DanhMucID != nil {
			sau.DanhMucID = *sach.DanhMucID
		}
		if sach.TieuDe != nil {
			sau.TieuDe = *sach.TieuDe
		}
		if sach.TomTat != nil {
			sau.TomTat = *sach.TomTat
		}
		if sach.NoiDung != nil {
			sau.NoiDung = *sach.NoiDung
		}
		if sach.AnhDaiDienURL != nil {
			sau.AnhDaiDienURL = *sach.AnhDaiDienURL
		}
		if sach.DangLenMiniApp != nil {
			// FROM THE CHECKBOX AND NOT FROM A STATE STRING, same as on the create — and with one
			// extra consequence here: an item the sync left in `cho-duyet` becomes `dang-hien` or
			// `an` by a member of staff's explicit act, which is what approving or refusing it IS.
			sau.TrangThai = trangThaiTu(*sach.DangLenMiniApp)
		}

		// THE CATEGORY IS CHECKED ONLY WHEN IT MOVED. Re-sending the category a row already has must
		// not fail because that category was retired in the meantime — the article is already filed
		// there, and refusing the save would make every other field uneditable.
		if sau.DanhMucID != truoc.DanhMucID && sau.DanhMucID != "" {
			co, err := uc.kho.DanhMucCoThat(ctx, tx, sau.DanhMucID)
			if err != nil {
				return err
			}
			if !co {
				return commsstore.ErrDanhMucKhongTonTaiMiniApp
			}
		}

		if khongDoiNoiDungMiniApp(truoc, sau) {
			return nil
		}

		// §10.4 — see the note on this function. AFTER the no-op check, deliberately.
		if truoc.Nguon == domain.NguonDongBoCong {
			sau.DaSuaTay = true
		}

		if err := uc.kho.CapNhat(ctx, tx, sau); err != nil {
			return err
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to find
		// in a ledger that is never deleted.
		delta, err := json.Marshal(map[string]any{
			"id":    sau.ID,
			"truoc": tomTatDoiNoiDungMiniApp(truoc, sau, true),
			"sau":   tomTatDoiNoiDungMiniApp(truoc, sau, false),
		})
		if err != nil {
			return fmt.Errorf("noi_dung_mini_app: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaNoiDungMiniApp,
			Subject: chuDeNoiDungMiniApp(sau),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.NoiDungMiniApp{}, bocNoiDung(ctx, "sửa nội dung Mini App", err)
	}
	return sau, nil
}

// Them adds one category to the commune's filing tree — §6's `⊞ Danh mục tin`.
//
// ORDER OF THE THREE REFUSALS, and it is not arbitrary: the ceiling first (a condition of the whole
// list, which the caller can act on without knowing anything about this value), then the duplicate
// slug, then the parent. The parent last because it is the only one that depends on another row
// being alive.
func (uc *DanhMucNoiDungMiniApp) Them(ctx context.Context, yc domain.YeuCauThemDanhMuc,
	nguoi audit.Actor) (domain.DanhMucMiniApp, error) {

	sach, err := yc.KiemTra()
	if err != nil {
		return domain.DanhMucMiniApp{}, err
	}
	if nguoi.ID == "" {
		return domain.DanhMucMiniApp{}, ErrThieuNguoiTaoNoiDung
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.DanhMucMiniApp{}, fmt.Errorf("danh_muc_mini_app: sinh mã: %w", err)
	}

	moi := domain.DanhMucMiniApp{
		ID:    id,
		Ten:   sach.Ten,
		Slug:  sach.Slug,
		ChaID: sach.ChaID,
		ThuTu: sach.ThuTu,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.kho.DemDangSong(ctx, tx)
		if err != nil {
			return err
		}
		if n >= commsstore.TranDanhMucMiniApp {
			return commsstore.ErrQuaNhieuDanhMucMiniApp
		}

		daDung, err := uc.kho.SlugDaDung(ctx, tx, moi.Slug)
		if err != nil {
			return err
		}
		if daDung {
			return commsstore.ErrSlugDanhMucDaTonTai
		}

		if moi.ChaID != "" {
			co, err := uc.kho.ChaCoThat(ctx, tx, moi.ChaID)
			if err != nil {
				return err
			}
			if !co {
				return ErrDanhMucChaKhongTonTai
			}
			// NO CYCLE CHECK IS NEEDED ON A CREATE AND THAT IS WORTH SAYING OUT LOUD: `moi.ID` is a
			// freshly minted ULID, so no existing row can already have it as an ancestor. The day a
			// RE-PARENTING route is written, it must walk up from the proposed parent looking for
			// itself — migration 0006 records the same obligation beside the CHECK that only catches
			// the one-node case.
		}

		if err := uc.kho.Chen(ctx, tx, moi); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": tomTatDanhMucMiniApp(moi)})
		if err != nil {
			return fmt.Errorf("danh_muc_mini_app: mã hoá delta: %w", err)
		}
		// NOTHING IN THIS DELTA IS PERSONAL DATA (rule 3): a category is how the authority files its
		// own news, not anything about a person. That is why the category's NAME may go into the
		// ledger while an article's TITLE may not — see tomTatNoiDungMiniApp.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:  nguoi,
			Action: HanhViThemDanhMucMiniApp,
			// THE BUSINESS CODE, never the internal id (rule 6 and the contract of audit.Entry). A
			// category has one, which is why this subject is a value rather than something composed.
			Subject: moi.Slug,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.DanhMucMiniApp{}, bocNoiDung(ctx, "thêm danh mục Mini App", err)
	}
	return moi, nil
}

// trangThaiTu maps §7's one checkbox onto §6's chip.
//
// A FUNCTION AND NOT AN INLINE TERNARY AT TWO CALL SITES: the two acts — composing and editing —
// have to agree about what the checkbox means, and two inline expressions are two places for them to
// stop agreeing. `cho-duyet` is deliberately unreachable from here (§10.2 gives it to the sync).
func trangThaiTu(dangLen bool) domain.TrangThaiNoiDung {
	if dangLen {
		return domain.TrangThaiDangHien
	}
	return domain.TrangThaiAn
}

// chuDeNoiDungMiniApp is the audit subject.
//
// NOT A ULID, for the reason rule 6, invariant 8 gives for `actor_id`: an entry is read years later
// by somebody handling a complaint or an inspection, and a ULID names nothing to them. AN ITEM OF
// CONTENT HAS NO BUSINESS CODE OF ITS OWN — chapter 11 prints no reference number and mints no
// series, and inventing one here would be deciding a numbering scheme for a record the customer has
// not asked to number. A CATEGORY does have one (`slug`), which is why Them above uses it directly.
//
// So it is COMPOSED from the two facts that locate the item in §6's table: the tab it sits under and
// the day it was published. The delta carries the id, which is the handle; a subject is a locator
// rather than a key.
//
// THE TITLE IS DELIBERATELY NOT IN IT. `tieu_de` is free text and a commune's news routinely names
// people ("Trao quà cho gia đình ông Nguyễn Văn A…"). The audit ledger is never deleted (rule 6,
// invariant 4), so anything put in it is there permanently — and rule 3, forbidden #5 keeps personal
// data out of exactly this kind of durable record.
func chuDeNoiDungMiniApp(n domain.NoiDungMiniApp) string {
	return "noi-dung-mini-app/" + string(n.Loai) + "/" + n.NgayDang.Format("2006-01-02")
}

// tomTatNoiDungMiniApp is the audit delta's view of one item.
//
// WHAT IS IN IT AND WHY EACH THING IS: the id (the only handle on the record), the type and the
// category (where it was filed), the state (whether residents can see it), and the provenance (which
// decides what a later sync may do to it). WHAT IS NOT IN IT: the title, the summary and the body.
// See chuDeNoiDungMiniApp — all three are free text that can name a person, and this ledger is never
// deleted.
func tomTatNoiDungMiniApp(n domain.NoiDungMiniApp) map[string]any {
	return map[string]any{
		"id":          n.ID,
		"loai":        string(n.Loai),
		"danh_muc_id": n.DanhMucID,
		"trang_thai":  string(n.TrangThai),
		"nguon":       string(n.Nguon),
		"da_sua_tay":  n.DaSuaTay,
		"ngay_dang":   n.NgayDang.Format("2006-01-02"),
		"co_anh":      n.CoAnh(),
	}
}

// tomTatDoiNoiDungMiniApp returns only the fields that actually moved, from whichever side is asked
// for.
//
// THE THREE TEXT FIELDS ARE REPORTED AS "CHANGED" WITHOUT THEIR VALUES. That is deliberate and is
// the whole difficulty of auditing a CMS: an inspection has to be able to see THAT the body of an
// article was rewritten — otherwise the trail cannot answer the question it exists for — while rule
// 3 keeps the text itself out of a ledger nothing ever deletes. A boolean says the first without the
// second.
func tomTatDoiNoiDungMiniApp(truoc, sau domain.NoiDungMiniApp, ben bool) map[string]any {
	ra := map[string]any{}
	if truoc.Loai != sau.Loai {
		ra["loai"] = string(chon(ben, truoc.Loai, sau.Loai))
	}
	if truoc.DanhMucID != sau.DanhMucID {
		ra["danh_muc_id"] = chon(ben, truoc.DanhMucID, sau.DanhMucID)
	}
	if truoc.TrangThai != sau.TrangThai {
		ra["trang_thai"] = string(chon(ben, truoc.TrangThai, sau.TrangThai))
	}
	if truoc.DaSuaTay != sau.DaSuaTay {
		ra["da_sua_tay"] = chon(ben, truoc.DaSuaTay, sau.DaSuaTay)
	}
	if truoc.CoAnh() != sau.CoAnh() {
		ra["co_anh"] = chon(ben, truoc.CoAnh(), sau.CoAnh())
	}
	if truoc.TieuDe != sau.TieuDe {
		ra["tieu_de_da_doi"] = true
	}
	if truoc.TomTat != sau.TomTat {
		ra["tom_tat_da_doi"] = true
	}
	if truoc.NoiDung != sau.NoiDung {
		ra["noi_dung_da_doi"] = true
	}
	return ra
}

// tomTatDanhMucMiniApp is the audit delta's view of one category.
func tomTatDanhMucMiniApp(dm domain.DanhMucMiniApp) map[string]any {
	return map[string]any{
		"id":     dm.ID,
		"ten":    dm.Ten,
		"slug":   dm.Slug,
		"cha_id": dm.ChaID,
		"thu_tu": dm.ThuTu,
	}
}

// khongDoiNoiDungMiniApp reports whether the edit would change nothing.
//
// `Nguon`, `NguonURL`, `NguonIDNgoai`, `NguoiTaoMa`, `NgayDang` AND `LuotXem` ARE NOT COMPARED
// because no path here can change them — they are absent from the UPDATE statement and refused by
// the trigger. `DaSuaTay` is not compared either, and that one is load bearing: it is DERIVED from
// whether this edit happened, so including it would make every edit of a synced article look like a
// change even when nothing else moved, and the no-op guarantee above would be false.
func khongDoiNoiDungMiniApp(truoc, sau domain.NoiDungMiniApp) bool {
	return truoc.Loai == sau.Loai &&
		truoc.DanhMucID == sau.DanhMucID &&
		truoc.TieuDe == sau.TieuDe &&
		truoc.TomTat == sau.TomTat &&
		truoc.NoiDung == sau.NoiDung &&
		truoc.AnhDaiDienURL == sau.AnhDaiDienURL &&
		truoc.TrangThai == sau.TrangThai
}

// bocNoiDung wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No title, no body, no actor: an error travels into centralised logging across every commune at
// once, and an article title can name a person (rule 3, forbidden #1). The commune is not personal
// data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is. A
// %v here would collapse "that category is not in this commune" and "the database is down" into one
// 500.
func bocNoiDung(ctx context.Context, viec string, err error) error {
	return fmt.Errorf("noi_dung_mini_app: %s cho xã %s: %w", viec, tenant.MustFrom(ctx), err)
}
