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
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/richtext"
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
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, deletedBy, reason string, at time.Time) error
}

// KhoDanhMucMiniApp is the category store. A SECOND INTERFACE rather than four more methods on the
// first, because they guard two different tables with two different refusals — and a caller reaching
// for whichever method was nearest is how an article ends up checked against a category ceiling.
type KhoDanhMucMiniApp interface {
	SlugDaDung(ctx context.Context, tx *store.ScopedTx, slug string) (bool, error)
	DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error)
	ChaCoThat(ctx context.Context, tx *store.ScopedTx, chaID string) (bool, error)
	Chen(ctx context.Context, tx *store.ScopedTx, dm domain.DanhMucMiniApp) error

	// The edit and the soft delete (ADR 0067 §3).
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.DanhMucMiniApp, error)
	LockTree(ctx context.Context, tx *store.ScopedTx) error
	AncestorChain(ctx context.Context, tx *store.ScopedTx, startID string) ([]string, error)
	CountLiveChildren(ctx context.Context, tx *store.ScopedTx, id string) (int, error)
	CountLiveItems(ctx context.Context, tx *store.ScopedTx, id string) (int, error)
	Update(ctx context.Context, tx *store.ScopedTx, c domain.DanhMucMiniApp) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, deletedBy, reason string) error
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

	// covers keeps the cover's public copy in step with the article (content_cover.go). nil = no file
	// store wired: a request naming a cover is refused with ErrCoverUploadNotConfigured.
	covers *coverPublisher

	// audioFiles retires (soft-deletes) the file row of a broadcast audio the edit takes off its item
	// (content_audio.go says why the slot must be freed). nil = not wired: an edit that would remove
	// audio is refused with ErrAudioUploadNotConfigured rather than leaving the slot held for ever.
	audioFiles AudioFileRetirer
}

// AudioFileRetirer is the one stored_file write the edit path needs for audio. *store.StoredFileStore.
type AudioFileRetirer interface {
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error
}

// WithAudioFiles wires the file store the edit path retires removed audio files through.
func (uc *SoanNoiDungMiniApp) WithAudioFiles(files AudioFileRetirer) *SoanNoiDungMiniApp {
	uc.audioFiles = files
	return uc
}

// audioRetireReason is the `delete_reason` of a broadcast file the item stopped pointing at.
const audioRetireReason = "gỡ tệp âm thanh khỏi mục truyền thanh (sửa mục nội dung)"

func NewSoanNoiDungMiniApp(db *store.DB, kho KhoNoiDungMiniApp, khoDanh KhoDanhMucMiniApp) *SoanNoiDungMiniApp {
	return &SoanNoiDungMiniApp{db: db, kho: kho, khoDanh: khoDanh, sinhID: ulid.Moi, bayGio: time.Now}
}

// WithCovers wires the cover file store and the object store (UNTYPED nil when not configured — see
// NewContentCovers) so composing and editing can attach a cover and publish its derivative.
func (uc *SoanNoiDungMiniApp) WithCovers(files CoverFiles, objects CoverObjectStore, log *slog.Logger) *SoanNoiDungMiniApp {
	if log == nil {
		log = slog.Default()
	}
	uc.covers = &coverPublisher{files: files, objects: objects, log: log}
	return uc
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

	// ActionDeleteContentItem — the per-row delete of §6 (user decision 2026-10-02). English identifier,
	// Vietnamese value (rule 12 / ADR 0011), the table named in the verb like its neighbours.
	ActionDeleteContentItem = "xoa_noi_dung_mini_app"

	// The verbs of ADR 0067 §3, same Vietnamese snake_case as their neighbours: an inspection reads these
	// strings. English identifiers, Vietnamese values (rule 12 / ADR 0011).
	ActionUpdateContentCategory = "sua_danh_muc_mini_app"
	ActionDeleteContentCategory = "xoa_danh_muc_mini_app"
)

// ErrCategoryNotEmpty refuses deleting a category that still holds live items or live child
// categories (ADR 0067 §3 decision 3). The handler's sentence suggests hiding it instead.
var ErrCategoryNotEmpty = errors.New("danh_muc_mini_app: danh mục còn nội dung hoặc danh mục con")

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
//	no upload here        §7's `Ảnh đại diện` is uploaded by ContentCovers (content_cover.go); this
//	                      act only ATTACHES a completed upload (`CoverImageFileID`) and, when the item
//	                      is born published, publishes its derivative (coverPublisher). The size and
//	                      format rules are platform's policy, read by the uploader, never here.
//	few per-type fields   the event window and place (`su-kien`) and the external video link (`video`)
//	                      ARE handled — the user decided them on 30/09/2026 (ADR 0047 §6) and
//	                      migration 0011 added the columns. The banner's link and order (migration
//	                      0012, ADR 0067 §5) are handled too. The audio file of a `truyen-thanh` is
//	                      NOT set here: it is attached by its upload's completion (content_audio.go).
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

	// THE BODY IS SANITISED FIRST, on this and every other write path (ADR 0067 §1 decision 2), and
	// BEFORE KiemTra so the length bound is checked on what will actually be stored. STAFF policy: this
	// is staff composing (ADR 0067 §Sửa đổi 03/10/2026, K1) — body images by file id, quote, byline.
	yc.NoiDung = richtext.SanitizeStaff(yc.NoiDung)

	// VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never hold a
	// transaction open while doing so, and the caller needs the reason rather than a rollback.
	sach, err := yc.KiemTra()
	if err != nil {
		return domain.NoiDungMiniApp{}, err
	}
	if nguoi.ID == "" {
		return domain.NoiDungMiniApp{}, ErrThieuNguoiTaoNoiDung
	}
	// A banner is born with its cover or not at all (migration 0012's trigger, asked first).
	if err := domain.CheckBannerCover(nil, domain.NoiDungMiniApp{
		Loai: domain.LoaiNoiDung(sach.Loai), CoverImageFileID: sach.CoverImageFileID}); err != nil {
		return domain.NoiDungMiniApp{}, err
	}

	// THE BODY'S IMAGES (ADR 0067 §Sửa đổi 03/10/2026, K2): every file id the sanitised body references.
	// Each is checked in the transaction below; the first also names the article when there is no cover.
	bodyImages := richtext.ImageFileIDs(sach.NoiDung)
	if len(bodyImages) > 0 && uc.covers == nil {
		return domain.NoiDungMiniApp{}, ErrCoverUploadNotConfigured
	}

	// THE ID. With a cover — or, without one, with a body image — it is the id the upload minted
	// (migration 0011: §7 uploads before `Lưu`) — read from the FILE ROW, never from the request, so a
	// client cannot name the id of a new record. Every other file must then carry the same id
	// (checkAttach / checkBodyImages), so a body image of another draft is refused like any other.
	var id string
	fromUpload := false
	if sach.CoverImageFileID != "" {
		if uc.covers == nil {
			return domain.NoiDungMiniApp{}, ErrCoverUploadNotConfigured
		}
		f, err := uc.covers.files.ByID(ctx, sach.CoverImageFileID)
		if err != nil {
			return domain.NoiDungMiniApp{}, bocNoiDung(ctx, "thêm nội dung Mini App", err)
		}
		if f == nil {
			return domain.NoiDungMiniApp{}, domain.ErrCoverNotUsable
		}
		if err := domain.CheckCoverUsable(f, f.SubjectID, string(coverPurpose)); err != nil {
			return domain.NoiDungMiniApp{}, err
		}
		id, fromUpload = f.SubjectID, true
	} else if len(bodyImages) > 0 {
		f, err := uc.covers.files.ByID(ctx, bodyImages[0])
		if err != nil {
			return domain.NoiDungMiniApp{}, bocNoiDung(ctx, "thêm nội dung Mini App", err)
		}
		if f == nil {
			return domain.NoiDungMiniApp{}, domain.ErrBodyImageNotUsable
		}
		if err := domain.CheckBodyImageUsable(f, f.SubjectID, string(bodyImagePurpose)); err != nil {
			return domain.NoiDungMiniApp{}, err
		}
		id, fromUpload = f.SubjectID, true
	} else if id, err = uc.sinhID(); err != nil {
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
		EventStartsAt: sach.EventStartsAt,
		EventEndsAt:   sach.EventEndsAt,
		EventPlace:    sach.EventPlace,
		VideoURL:      sach.VideoURL,
		LinkTo:        sach.LinkTo,
		DisplayOrder:  sach.DisplayOrder,
		NgayDang:      ngayDang,
		LuotXem:       0,

		CoverImageFileID: sach.CoverImageFileID,
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
	// G1 (ADR 0047 §6), THE HALF MIGRATION 0011'S TRIGGER CANNOT ENFORCE: its trigger is UPDATE-only,
	// so a row born `dang-hien` gets its first-publish instant here — the same clock reading as the
	// rest of the act — and a row born `an` gets none until the edit that publishes it (Sua).
	if moi.TrangThai == domain.TrangThaiDangHien {
		moi.PublishedAt = luc
	}

	// PUBLISHED WITH A COVER: settle copies the derivative to the public bucket INSIDE this transaction
	// and records its key (coverPublisher says why); a rollback after the copy withdraws it again.
	var cover settlement
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
		if fromUpload {
			// The upload's id must still be FREE: an upload issued for an existing article cannot be
			// used to create a second one under its id.
			switch _, err := uc.kho.TheoIDDeSua(ctx, tx, moi.ID); {
			case err == nil && moi.CoverImageFileID != "":
				return domain.ErrCoverNotUsable
			case err == nil:
				return domain.ErrBodyImageNotUsable
			case !errors.Is(err, commsstore.ErrNoiDungKhongTonTai):
				return err
			}
		}
		if moi.CoverImageFileID != "" {
			if err := uc.covers.checkAttach(ctx, tx, moi.ID, moi.CoverImageFileID); err != nil {
				return err
			}
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3): every referenced file locked and checked.
		if len(bodyImages) > 0 {
			if err := uc.covers.checkBodyImages(ctx, tx, moi.ID, bodyImages); err != nil {
				return err
			}
		}
		if err := uc.kho.Chen(ctx, tx, moi); err != nil {
			return err
		}
		after := tomTatNoiDungMiniApp(moi)
		if len(bodyImages) > 0 {
			after["body_image_file_ids"] = bodyImages // internal handles, never a file name (rule 3)
		}
		if uc.covers != nil {
			// Ready body images uploaded for this (reserved) id that the saved body does not place: retired
			// here, recorded in this entry (retireUnreferencedBodyImages). Pending uploads stay.
			retired, err := uc.covers.retireUnreferencedBodyImages(ctx, tx, moi.ID, moi.NoiDung, nguoi.ID, luc)
			if err != nil {
				return err
			}
			if len(retired) > 0 {
				after["body_images_retired"] = retired
			}
			if cover, err = uc.covers.settle(ctx, tx, moi, luc); err != nil {
				return err
			}
			if cover.published != "" {
				after["cover_published_file_id"] = cover.published
			}
			if len(cover.bodyPublished) > 0 {
				after["body_images_published"] = cover.bodyPublished
			}
		}

		delta, err := json.Marshal(map[string]any{"sau": after})
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
		// Nothing was committed: no item, no trail. The two agree — and every copy made for it is withdrawn.
		if len(cover.copied) > 0 {
			uc.covers.undoCopies(ctx, cover)
		}
		return domain.NoiDungMiniApp{}, coverRefusal(ctx, "thêm nội dung Mini App", err)
	}
	if len(cover.withdraw) > 0 {
		uc.covers.withdrawAfterCommit(ctx, uc.db, moi, cover.withdraw, nguoi, luc)
	}
	return moi, nil
}

// coverRefusal returns a cover refusal UNWRAPPED (the handler answers it with its own sentence, which
// names no commune) and wraps every other failure with bocNoiDung.
func coverRefusal(ctx context.Context, viec string, err error) error {
	for _, r := range []error{domain.ErrCoverNotUsable, domain.ErrBodyImageNotUsable, ErrCoverPublishUnavailable,
		ErrCoverUploadNotConfigured} {
		if errors.Is(err, r) {
			return err
		}
	}
	return bocNoiDung(ctx, viec, err)
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
	// Sanitised before KiemTra, as on the create (ADR 0067 §1 decision 2). Only a body the request
	// SENDS is touched: an edit that does not mention the body leaves a legacy row's stored HTML exactly
	// as it is (rule 7) — the public read sanitises that one on the way out. STAFF policy, as on the
	// create (K1) — also when the row came from the portal: the edit is a member of staff writing.
	if yc.NoiDung != nil {
		clean := richtext.SanitizeStaff(*yc.NoiDung)
		yc.NoiDung = &clean
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
	var cover settlement
	// refusal carries a refusal decided INSIDE the transaction (it needs the stored row) back out
	// UNWRAPPED. bocNoiDung would prefix it with the commune id, and the handler returns a refusal's
	// own sentence to the client — which must name the field, not the commune.
	var refusal error
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
		if sach.EventStartsAt != nil {
			sau.EventStartsAt = *sach.EventStartsAt
		}
		if sach.EventEndsAt != nil {
			sau.EventEndsAt = *sach.EventEndsAt
		}
		if sach.EventPlace != nil {
			sau.EventPlace = *sach.EventPlace
		}
		if sach.VideoURL != nil {
			sau.VideoURL = *sach.VideoURL
		}
		if sach.CoverImageFileID != nil {
			sau.CoverImageFileID = *sach.CoverImageFileID // "" detaches; the file row stays (rule 7)
		}
		if sach.LinkTo != nil {
			sau.LinkTo = *sach.LinkTo
		}
		if sach.DisplayOrder != nil {
			sau.DisplayOrder = sach.DisplayOrder
		}
		// THE AUDIO PAIR (migration 0012, ADR 0067 §4): "" removes both; the id already attached is a
		// no-op; any OTHER id is refused — a file is attached by its upload's completion, which carries
		// the duration (ContentAudio.Complete). A duration alone corrects the one already typed.
		if sach.AudioFileID != nil {
			switch *sach.AudioFileID {
			case "":
				sau.AudioFileID, sau.AudioDurationSeconds = "", 0
			case truoc.AudioFileID:
			default:
				refusal = domain.ErrAudioNotUsable
				return refusal
			}
		}
		if sach.AudioDurationSeconds != nil {
			sau.AudioDurationSeconds = *sach.AudioDurationSeconds
		}

		// THE PER-TYPE FIELDS OF MIGRATION 0011, decided on the MERGED row because the type after the
		// edit may come from the request or from the stored row.
		//
		//   a VALUE sent for a field the resulting type cannot carry → refused, never dropped: the author
		//     believes that date or link is published;
		//   the type moved AWAY from su-kien / video → that type's columns are cleared in this same
		//     UPDATE, which is what 0011's CHECKs require and what its comment asks for;
		//   then the window rule on what remains (a PATCH of `event_ends_at` alone is judged against
		//     the stored start).
		if sau.Loai != domain.LoaiSuKien && sach.SetsEventField() {
			refusal = domain.ErrEventFieldsOnlyForEvent
			return refusal
		}
		if sau.Loai != domain.LoaiVideo && sach.SetsVideoURL() {
			refusal = domain.ErrVideoURLOnlyForVideo
			return refusal
		}
		if sau.Loai != domain.LoaiBanner && sach.SetsBannerField() {
			refusal = domain.ErrBannerFieldsOnlyForBanner
			return refusal
		}
		if sau.Loai != domain.LoaiTruyenThanh && sach.SetsAudioField() {
			refusal = domain.ErrAudioOnlyForBroadcast
			return refusal
		}
		sau = sau.WithoutOtherTypeFields()
		if err := sau.CheckTypeFields(); err != nil {
			refusal = err
			return refusal
		}
		// Migration 0012's cover trigger, on the merged row: no item becomes a banner without a cover,
		// and a banner's cover is replaced, never removed.
		if err := domain.CheckBannerCover(&truoc, sau); err != nil {
			refusal = err
			return refusal
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

		// A NEW cover is checked under lock, before anything is written: uploaded for THIS article,
		// `ready` (its derivative exists), not deleted. Re-sending the current cover checks nothing.
		if sau.CoverImageFileID != "" && sau.CoverImageFileID != truoc.CoverImageFileID {
			if uc.covers == nil {
				refusal = ErrCoverUploadNotConfigured
				return refusal
			}
			if err := uc.covers.checkAttach(ctx, tx, sau.ID, sau.CoverImageFileID); err != nil {
				return err
			}
		}

		// A BODY THE REQUEST SENDS (ADR 0067 §Sửa đổi 03/10/2026, K2): every image it references is
		// checked under lock — all of them, not only the new ones, so a body saved before this check
		// existed cannot carry an unverified id through its next edit. Every ready body image the saved
		// body does not reference is retired after the UPDATE below.
		if sach.NoiDung != nil {
			now := richtext.ImageFileIDs(sau.NoiDung)
			if len(now) > 0 {
				if uc.covers == nil {
					refusal = ErrCoverUploadNotConfigured
					return refusal
				}
				if err := uc.covers.checkBodyImages(ctx, tx, sau.ID, now); err != nil {
					return err
				}
			}
		}

		if khongDoiNoiDungMiniApp(truoc, sau) {
			return nil
		}

		// THE BROADCAST FILE THE ITEM STOPS POINTING AT — removed, or cleared by a type change away from
		// `truyen-thanh` — is soft-deleted in THIS transaction, so the one-file slot is free for its
		// replacement and no live audio row is left with no item to play it (rule 7: soft, the object
		// stays). Refused, not skipped, when the file store is not wired.
		retiredAudio := ""
		if truoc.AudioFileID != "" && sau.AudioFileID != truoc.AudioFileID {
			if uc.audioFiles == nil {
				refusal = ErrAudioUploadNotConfigured
				return refusal
			}
			retiredAudio = truoc.AudioFileID
		}

		// §10.4 — see the note on this function. AFTER the no-op check, deliberately.
		if truoc.Nguon == domain.NguonDongBoCong {
			sau.DaSuaTay = true
		}

		// G1 (ADR 0047 §6): the first-publish instant is set by THIS edit only when it moves the row
		// INTO `dang-hien` and none is recorded yet — the one case migration 0011's trigger admits. Once
		// set it is never touched again: unpublishing leaves it, republishing keeps the original.
		//
		// ⚠ A LEGACY ROW (published before 0011, `published_at` NULL — no backfill, on purpose) that is
		// unpublished and later republished gets THIS instant: its real first publish was never
		// recorded, and the republish is the first one the system witnessed. An edit that leaves such a
		// row `dang-hien` stamps nothing; readers fall back to `ngay_dang`.
		if sau.TrangThai == domain.TrangThaiDangHien && truoc.TrangThai != domain.TrangThaiDangHien &&
			truoc.PublishedAt.IsZero() {
			sau.PublishedAt = uc.bayGio().UTC()
		}

		if err := uc.kho.CapNhat(ctx, tx, sau); err != nil {
			return err
		}
		if retiredAudio != "" {
			// ErrStoredFileMoved: the row is already retired (or carries a public key, which an audio
			// row never does) — the slot is free either way, so the edit stands.
			err := uc.audioFiles.SoftDelete(ctx, tx, retiredAudio, nguoi.ID, audioRetireReason, uc.bayGio().UTC())
			if err != nil && !errors.Is(err, commsstore.ErrStoredFileMoved) {
				return err
			}
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to find
		// in a ledger that is never deleted.
		afterDelta := tomTatDoiNoiDungMiniApp(truoc, sau, false)
		if retiredAudio != "" {
			afterDelta["audio_file_retired"] = retiredAudio
		}
		if uc.covers != nil {
			// EVERY READY BODY IMAGE THE SAVED BODY DOES NOT REFERENCE (removed by this edit, or uploaded
			// and never placed), not public: retired here, in this transaction, recorded in this entry. A
			// public one is retired by withdrawAfterCommit once its copy is gone. Pending uploads stay.
			retired, err := uc.covers.retireUnreferencedBodyImages(ctx, tx, sau.ID, sau.NoiDung, nguoi.ID,
				uc.bayGio().UTC())
			if err != nil {
				return err
			}
			if len(retired) > 0 {
				afterDelta["body_images_retired"] = retired
			}
			// THE PUBLIC COPIES FOLLOW THE ROW JUST WRITTEN (coverPublisher.settle, per purpose):
			// published → the cover and every referenced body image copied and recorded here; anything
			// else still public → withdrawn after commit.
			if cover, err = uc.covers.settle(ctx, tx, sau, uc.bayGio().UTC()); err != nil {
				return err
			}
			if cover.published != "" {
				afterDelta["cover_published_file_id"] = cover.published
			}
			if len(cover.bodyPublished) > 0 {
				afterDelta["body_images_published"] = cover.bodyPublished
			}
		}
		delta, err := json.Marshal(map[string]any{
			"id":    sau.ID,
			"truoc": tomTatDoiNoiDungMiniApp(truoc, sau, true),
			"sau":   afterDelta,
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
	if err != nil && len(cover.copied) > 0 {
		// Rolled back after public copies were made: withdraw them (coverPublisher, PUBLISH).
		uc.covers.undoCopies(ctx, cover)
	}
	if refusal != nil {
		// Rolled back (the closure returned it); nothing was written, no trail.
		return domain.NoiDungMiniApp{}, refusal
	}
	if err != nil {
		return domain.NoiDungMiniApp{}, coverRefusal(ctx, "sửa nội dung Mini App", err)
	}
	if len(cover.withdraw) > 0 {
		uc.covers.withdrawAfterCommit(ctx, uc.db, sau, cover.withdraw, nguoi, uc.bayGio().UTC())
	}
	return sau, nil
}

// Delete soft-deletes one item — §6's per-row delete (user decision 2026-10-02: follow the requirement
// prototype, ../vigov-require/apps/api/app/modules/content/service.py:170-179, soft delete + unpublish).
//
// WHAT DIFFERS FROM THE PROTOTYPE: THE REASON IS REQUIRED. The prototype deletes without one; rule 7
// invariant 1 and migration 0006's CHECK `noi_dung_mini_app_xoa_mem_day_du` make `delete_reason` part
// of what a soft delete is. Shape-checked before the transaction opens.
//
// ONE TRANSACTION: the row read FOR UPDATE (live rows of THIS commune only — so another commune's id, an
// invented one and an already-deleted one are one ErrNoiDungKhongTonTai), the UPDATE that sets the three
// soft-delete columns AND `trang_thai = 'an'`, and the audit entry. `deleted_by` and the entry's actor
// are the same staff BUSINESS CODE (audit.Actor.ID is built from Principal.Ma — rule 6, invariant 8).
//
// THE COVER'S PUBLIC COPY GOES THE WAY AN UNPUBLISH SENDS IT (coverPublisher): settle runs on the row as
// deleted — no longer `dang-hien` — so every file of the item still carrying a public key is listed, and
// withdrawn after commit with its own entry. A failed withdrawal keeps the key; see the ⚠ below.
//
// THE AUDIO FILE IS NOT TOUCHED: it is never public (content_audio.go) — residents get a presigned GET
// of at most PublicAudioURLTTL, signed only for items a public read returns, and a deleted item is
// returned by none. The file row stays attached to the record (rule 7).
//
// ⚠ A WITHDRAWAL THAT FAILS AFTER COMMIT IS NOT RETRIED BY ANYTHING. For an edit, the next edit retries
// it (PublicForSubject finds the key); a deleted item has no next edit. The copy is then reachable only
// through its key — two random ULIDs, a public bucket that is not listable (ADR 0052 §2) — and no read
// of this service links it any more. Logged by withdrawAfterCommit; a sweep is owed (reported).
//
// THE REASON TEXT IS NOT IN THE DELTA, ITS LENGTH IS — service-identity's staff delete convention
// (app/danh_ba_can_bo.go:1271-1291). It is free text about a public record that can name a household;
// the ledger is never deleted, and the reason itself is kept for ever in `delete_reason`.
func (uc *SoanNoiDungMiniApp) Delete(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return commsstore.ErrNoiDungKhongTonTai
	}
	reason, err := domain.ChuanHoaLyDoXoa(rawReason)
	if err != nil {
		return err
	}
	if actor.ID == "" {
		return ErrThieuNguoiTaoNoiDung
	}

	at := uc.bayGio().UTC()
	var after domain.NoiDungMiniApp
	var cover settlement
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := uc.kho.SoftDelete(ctx, tx, before.ID, actor.ID, reason, at); err != nil {
			return err
		}
		after = before
		after.TrangThai = domain.TrangThaiAn

		afterDelta := map[string]any{"trang_thai": string(after.TrangThai), "da_xoa": true}
		if uc.covers != nil {
			var err error
			if cover, err = uc.covers.settle(ctx, tx, after, at); err != nil {
				return err
			}
			if len(cover.withdraw) > 0 {
				ids := make([]string, 0, len(cover.withdraw))
				for _, f := range cover.withdraw {
					ids = append(ids, f.ID)
				}
				afterDelta["cover_withdraw_file_ids"] = ids
			}
		}

		beforeDelta := tomTatNoiDungMiniApp(before)
		beforeDelta["da_xoa"] = false
		delta, err := json.Marshal(map[string]any{
			"id":           before.ID,
			"truoc":        beforeDelta,
			"sau":          afterDelta,
			"do_dai_ly_do": len([]rune(reason)),
			"xoa_luc":      at.Format(time.RFC3339),
		})
		if err != nil {
			return fmt.Errorf("noi_dung_mini_app: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE UPDATE (rule 6, invariant 3); TenantID filled by audit.Write from the tx.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionDeleteContentItem,
			Subject: chuDeNoiDungMiniApp(before),
			At:      at,
			Delta:   delta,
		})
	})
	if err != nil {
		// Rolled back: no deletion, no trail. settle never copies for a row that is not `dang-hien`, so
		// there is no public copy to undo. %w keeps ErrNoiDungKhongTonTai visible to the handler (404).
		return bocNoiDung(ctx, "xoá nội dung Mini App", err)
	}
	if len(cover.withdraw) > 0 {
		uc.covers.withdrawAfterCommit(ctx, uc.db, after, cover.withdraw, actor, at)
	}
	return nil
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
			// freshly minted ULID, so no existing row can already have it as an ancestor. The
			// re-parenting edit (Sua below) is where the ancestor walk lives.
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

// Update edits one category: name, parent, order, hidden (ADR 0067 §3). The slug is not editable — the
// handler refuses a body naming it (domain.ErrSlugImmutable).
//
// RE-PARENTING WALKS THE ANCESTORS OF THE PROPOSED PARENT, under the commune's tree lock, and refuses
// when the category itself is among them (migration 0006:223-228). The lock is what makes the walk
// true at commit: two re-parentings racing each other would otherwise each pass and leave a cycle.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — the reason the route can declare idem.KhongCan.
func (uc *DanhMucNoiDungMiniApp) Update(ctx context.Context, id string, yc domain.ContentCategoryUpdate,
	actor audit.Actor) (domain.DanhMucMiniApp, error) {

	if id == "" {
		return domain.DanhMucMiniApp{}, commsstore.ErrDanhMucKhongTonTaiMiniApp
	}
	clean, err := yc.KiemTra()
	if err != nil {
		return domain.DanhMucMiniApp{}, err
	}
	if actor.ID == "" {
		return domain.DanhMucMiniApp{}, ErrThieuNguoiTaoNoiDung
	}

	var after domain.DanhMucMiniApp
	// refusal carries a refusal decided inside the transaction back out UNWRAPPED (bocNoiDung would
	// prefix the commune id, and the handler returns a refusal's own sentence).
	var refusal error
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.kho.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		after = before
		if clean.Ten != nil {
			after.Ten = *clean.Ten
		}
		if clean.ChaID != nil {
			after.ChaID = *clean.ChaID
		}
		if clean.ThuTu != nil {
			after.ThuTu = *clean.ThuTu
		}
		if clean.Hidden != nil {
			after.Hidden = *clean.Hidden
		}

		if after.ChaID != before.ChaID && after.ChaID != "" {
			if after.ChaID == after.ID {
				refusal = domain.ErrDanhMucTuLamCha
				return refusal
			}
			if err := uc.kho.LockTree(ctx, tx); err != nil {
				return err
			}
			live, err := uc.kho.ChaCoThat(ctx, tx, after.ChaID)
			if err != nil {
				return err
			}
			if !live {
				refusal = ErrDanhMucChaKhongTonTai
				return refusal
			}
			chain, err := uc.kho.AncestorChain(ctx, tx, after.ChaID)
			if err != nil {
				return err
			}
			if domain.ReparentCreatesCycle(after.ID, chain) {
				refusal = domain.ErrCategoryCycle
				return refusal
			}
		}

		if after == before {
			return nil
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the UPDATE.
		if err := uc.kho.Update(ctx, tx, after); err != nil {
			return err
		}
		// BEFORE AND AFTER OF THE FIELDS THAT MOVED (rule 6, invariant 5). A category name is how the
		// authority files its own news, not personal data — the create already records it by value.
		delta, err := json.Marshal(map[string]any{
			"id":    after.ID,
			"truoc": categoryChanges(before, after, true),
			"sau":   categoryChanges(before, after, false),
		})
		if err != nil {
			return fmt.Errorf("danh_muc_mini_app: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionUpdateContentCategory,
			Subject: after.Slug,
			Delta:   delta,
		})
	})
	if refusal != nil {
		return domain.DanhMucMiniApp{}, refusal
	}
	if err != nil {
		return domain.DanhMucMiniApp{}, bocNoiDung(ctx, "sửa danh mục Mini App", err)
	}
	return after, nil
}

// Delete soft deletes one category, with a mandatory reason (rule 7, invariant 1).
//
// REFUSED WHILE IT HOLDS ANY LIVE ITEM (in any state) OR ANY LIVE CHILD CATEGORY — the owner chose
// refusal over un-filing (ADR 0067 §3): un-filing is a silent edit of many records, and hiding reaches
// the same goal without touching one. The row stays, its slug is never reissued, its items keep their
// `danh_muc_id`.
func (uc *DanhMucNoiDungMiniApp) Delete(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return commsstore.ErrDanhMucKhongTonTaiMiniApp
	}
	reason, err := domain.ChuanHoaLyDoXoa(rawReason)
	if err != nil {
		return err
	}
	if actor.ID == "" {
		return ErrThieuNguoiTaoNoiDung
	}

	var refusal error
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.kho.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		children, err := uc.kho.CountLiveChildren(ctx, tx, id)
		if err != nil {
			return err
		}
		items, err := uc.kho.CountLiveItems(ctx, tx, id)
		if err != nil {
			return err
		}
		if children > 0 || items > 0 {
			refusal = ErrCategoryNotEmpty
			return refusal
		}
		// deleted_by is the actor's BUSINESS CODE — audit.Actor.ID is built from Principal.Ma
		// (rule 6, invariant 8), so the column and the entry name the same person.
		if err := uc.kho.SoftDelete(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{
			"truoc":   tomTatDanhMucMiniApp(before),
			"ly_do":   reason,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("danh_muc_mini_app: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionDeleteContentCategory,
			Subject: before.Slug,
			Delta:   delta,
		})
	})
	if refusal != nil {
		return refusal
	}
	if err != nil {
		return bocNoiDung(ctx, "xoá danh mục Mini App", err)
	}
	return nil
}

// categoryChanges returns the fields that moved, from the before side (beforeSide) or the after side.
func categoryChanges(before, after domain.DanhMucMiniApp, beforeSide bool) map[string]any {
	out := map[string]any{}
	if before.Ten != after.Ten {
		out["ten"] = chon(beforeSide, before.Ten, after.Ten)
	}
	if before.ChaID != after.ChaID {
		out["cha_id"] = chon(beforeSide, before.ChaID, after.ChaID)
	}
	if before.ThuTu != after.ThuTu {
		out["thu_tu"] = chon(beforeSide, before.ThuTu, after.ThuTu)
	}
	if before.Hidden != after.Hidden {
		out["hidden"] = chon(beforeSide, before.Hidden, after.Hidden)
	}
	return out
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
		// Migration 0011 (keys in English, rule 12; the older keys above are not renamed). The two
		// instants are the authority's own schedule, not personal data. The PLACE and the VIDEO LINK are
		// free text that can name a household, so only their presence enters a ledger nothing deletes.
		"published_at":    instantOrEmpty(n.PublishedAt),
		"event_starts_at": instantOrEmpty(n.EventStartsAt),
		"event_ends_at":   instantOrEmpty(n.EventEndsAt),
		"has_event_place": n.EventPlace != "",
		"has_video_url":   n.VideoURL != "",
		// The cover's FILE ID — an internal handle, not personal data; its file name never enters here.
		"cover_image_file_id": n.CoverImageFileID,
		// Migration 0012's banner pair, split like the video link: position by value, link by presence.
		"display_order": orderOrNil(n.DisplayOrder),
		"has_link_to":   n.LinkTo != "",
		// Migration 0012's audio pair: the file ID (an internal handle) and the typed duration.
		"audio_file_id":          n.AudioFileID,
		"audio_duration_seconds": n.AudioDurationSeconds,
	}
}

// instantOrEmpty is an audit-delta value: RFC 3339 in UTC, or "" for NULL.
func instantOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
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
	// Migration 0011's fields, same split as tomTatNoiDungMiniApp: instants by value, place and link
	// as "changed" only. `published_at` moves at most once (G1), so its before side is always "".
	if !truoc.PublishedAt.Equal(sau.PublishedAt) {
		ra["published_at"] = instantOrEmpty(chon(ben, truoc.PublishedAt, sau.PublishedAt))
	}
	if !truoc.EventStartsAt.Equal(sau.EventStartsAt) {
		ra["event_starts_at"] = instantOrEmpty(chon(ben, truoc.EventStartsAt, sau.EventStartsAt))
	}
	if !truoc.EventEndsAt.Equal(sau.EventEndsAt) {
		ra["event_ends_at"] = instantOrEmpty(chon(ben, truoc.EventEndsAt, sau.EventEndsAt))
	}
	if truoc.EventPlace != sau.EventPlace {
		ra["event_place_changed"] = true
	}
	if truoc.VideoURL != sau.VideoURL {
		ra["video_url_changed"] = true
	}
	if truoc.CoverImageFileID != sau.CoverImageFileID {
		ra["cover_image_file_id"] = chon(ben, truoc.CoverImageFileID, sau.CoverImageFileID)
	}
	// Migration 0012's banner pair: the position by value (the authority's own arrangement), the link
	// as "changed" only, like video_url.
	if !sameOrder(truoc.DisplayOrder, sau.DisplayOrder) {
		ra["display_order"] = orderOrNil(chon(ben, truoc.DisplayOrder, sau.DisplayOrder))
	}
	if truoc.LinkTo != sau.LinkTo {
		ra["link_to_changed"] = true
	}
	// Migration 0012's audio pair, both by value: an internal file id and a number of seconds.
	if truoc.AudioFileID != sau.AudioFileID {
		ra["audio_file_id"] = chon(ben, truoc.AudioFileID, sau.AudioFileID)
	}
	if truoc.AudioDurationSeconds != sau.AudioDurationSeconds {
		ra["audio_duration_seconds"] = chon(ben, truoc.AudioDurationSeconds, sau.AudioDurationSeconds)
	}
	return ra
}

// sameOrder compares two nullable display orders by value.
func sameOrder(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// orderOrNil is an audit-delta value: the number, or nil (JSON null) for NULL.
func orderOrNil(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// tomTatDanhMucMiniApp is the audit delta's view of one category.
func tomTatDanhMucMiniApp(dm domain.DanhMucMiniApp) map[string]any {
	return map[string]any{
		"id":     dm.ID,
		"ten":    dm.Ten,
		"slug":   dm.Slug,
		"cha_id": dm.ChaID,
		"thu_tu": dm.ThuTu,
		"hidden": dm.Hidden,
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
		truoc.TrangThai == sau.TrangThai &&
		truoc.EventStartsAt.Equal(sau.EventStartsAt) &&
		truoc.EventEndsAt.Equal(sau.EventEndsAt) &&
		truoc.EventPlace == sau.EventPlace &&
		truoc.VideoURL == sau.VideoURL &&
		truoc.CoverImageFileID == sau.CoverImageFileID &&
		truoc.LinkTo == sau.LinkTo &&
		sameOrder(truoc.DisplayOrder, sau.DisplayOrder) &&
		truoc.AudioFileID == sau.AudioFileID &&
		truoc.AudioDurationSeconds == sau.AudioDurationSeconds
	// `PublishedAt` is NOT compared, for the reason `DaSuaTay` is not: it is derived from the act of
	// publishing, which a changed TrangThai already makes a change.
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
