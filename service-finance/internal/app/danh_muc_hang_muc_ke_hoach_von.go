package app

// The use cases behind the WRITE surface of the document-type catalogue.
//
// WHY THIS LAYER EXISTS FOR WHAT LOOKS LIKE THREE SMALL STATEMENTS — the same answer
// service-comms gives for its notification ledger, and it is worth repeating because it is the one
// structural rule this repository was built to keep: rule 6, invariant 3 requires the audit entry
// to share a transaction with the business write, and core/audit.Write takes a *store.ScopedTx with
// no overload that writes outside one. Opening that transaction is this layer's job. The handler
// translates HTTP and nothing else, and the store knows SQL and nothing else.
//
// THE SECOND REASON IS THE THREE-TIER MODEL. Every write here is read-decide-write: read the row
// under a lock, work out which tier it is in, refuse or apply. That decision needs the row AND the
// rules AND the transaction at once, which is exactly one place — here.
//
// WHAT THIS FILE DELIBERATELY DOES NOT DO: sow a commune's `nguon = 'he-thong'` rows. That step is
// COMMUNE ONBOARDING, it does not exist in this repository, and designing it decides who writes a
// commune's first rows and under which principal in the audit trail (rule 6, invariant 6). It is
// the customer's call, not a use case's — see migrations/0003_danh_muc_hang_muc_ke_hoach_von.sql:47-52. An
// empty catalogue is the correct state of every commune today.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	docstore "github.com/vihat/vigov/service-finance/internal/store"
)

// KhoHangMuc is the store, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the row in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that `nguon` is never written from a request — provable without a
// PostgreSQL. A test that needs infrastructure is a test that stops being run.
type KhoHangMuc interface {
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.HangMucKeHoachVon, error)
	MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error)
	DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error)
	Chen(ctx context.Context, tx *store.ScopedTx, hm domain.HangMucKeHoachVon) error
	BoMacDinhKhac(ctx context.Context, tx *store.ScopedTx, trongID string) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, hm domain.HangMucKeHoachVon) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
}

// DanhMucHangMuc owns adding, editing and retiring one commune's document types.
type DanhMucHangMuc struct {
	db  *store.DB
	kho KhoHangMuc

	// sinhID is injected so a test can pin the id. In production it is ulid.Moi.
	sinhID func() (string, error)
}

func NewDanhMucHangMuc(db *store.DB, kho KhoHangMuc) *DanhMucHangMuc {
	return &DanhMucHangMuc{db: db, kho: kho, sinhID: ulid.Moi}
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `ghi_so_thong_bao`, `xem_day_du_nguoi_gui`): an inspection
// reads these strings, and a function name would tell them nothing.
//
// THE VERB NAMES THE CATALOGUE, not just the operation. `sua_danh_muc` across five tables would
// make the trail unable to answer which list changed without joining to a row that may since have
// been edited again.
const (
	HanhViThemHangMuc = "them_hang_muc_ke_hoach_von"
	HanhViSuaHangMuc  = "sua_hang_muc_ke_hoach_von"
	HanhViXoaHangMuc  = "xoa_hang_muc_ke_hoach_von"
)

// YeuCauThemHangMuc is one new row, as it arrives from the handler.
//
// THERE IS NO `Nguon` FIELD AND THERE MUST NEVER BE ONE. Provenance decides the tier, so a field
// here is a field a handler can fill from a request body — and the migration says what follows:
// "every guard below could be stepped around by setting nguon = 'don-vi' first". The store writes
// the value as a LITERAL for the same reason.
//
// THERE IS NO `DangDung` FIELD EITHER, and that is a smaller decision said out loud: a row the
// commune has just added is in use. Creating one already disabled is two requests — POST then
// PATCH — and the second one is the one that leaves a trail saying somebody turned it off.
type YeuCauThemHangMuc struct {
	Ma        string
	Nhan      string
	ThuTu     int
	LaMacDinh bool

	// Color is the display colour (migration 0017); nil = none chosen.
	Color *string
}

// YeuCauSuaHangMuc is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. Three of the four fields have a meaningful zero —
// `thu_tu` 0 is the first position, `dang_dung` false is "taken out of use", `la_mac_dinh` false is
// "no longer the default". A struct of plain values cannot tell "the client did not mention this"
// from "the client set it to zero", so a screen that edits only the label would silently move the
// row to the top of the list and clear the commune's default.
type YeuCauSuaHangMuc struct {
	Nhan      *string
	ThuTu     *int
	DangDung  *bool
	LaMacDinh *bool

	// Color has THREE states: nil = leave it; a change with Color nil = clear it; a value = set it.
	// Editable on every tier — presentation only (ADR 0079 lô 2 Q1 #9).
	Color *CatalogueColorChange
}

// categoryCreateAttempts bounds how many TRANSACTIONS an auto-coded create may take. A retry happens
// only when another request committed the very code this one chose between its check and its
// insert; each retry re-reads the taken codes, so a third collision in a row would mean a burst of
// identical labels, and refusing then is better than looping.
const categoryCreateAttempts = 3

// Them adds one row the commune owns.
//
// ORDER OF THE THREE REFUSALS, and it is not arbitrary: shape first (cheap, no lock), then the
// ceiling, then the duplicate code. The ceiling before the duplicate because a full catalogue is a
// condition of the whole list while a duplicate is a condition of one value — and the caller can
// act on the first without knowing anything about the second.
//
// `Ma` BLANK → THE SERVER ISSUES THE CODE (user decision 07/10/2026; the prototype's dialog sends a
// label only, CategoryManagerDialog.tsx:64-80). The code is the label's slug, then -2, -3 … until
// one has NEVER been used in this commune — MaDaDung counts soft-deleted rows, so an issued code is
// never issued again (rule 7, invariant 3). NO COUNTER TABLE, unlike projects (migration 0014): a
// project number is a series the commune reads in order, a slug is not, and the race a counter
// would serialise is closed here by `UNIQUE (tenant_id, ma)` plus a retry in a fresh transaction.
func (uc *DanhMucHangMuc) Them(ctx context.Context, yc YeuCauThemHangMuc,
	nguoi audit.Actor) (domain.HangMucKeHoachVon, error) {

	// Validated BEFORE the transaction opens. A request that fails its shape must never hold a row
	// lock while doing so, and the caller needs the reason rather than a rollback.
	autoCode := strings.TrimSpace(yc.Ma) == ""
	var ma string
	if !autoCode {
		var err error
		if ma, err = domain.ChuanHoaMa(yc.Ma); err != nil {
			return domain.HangMucKeHoachVon{}, err
		}
	}
	nhan, err := domain.ChuanHoaNhan(yc.Nhan)
	if err != nil {
		return domain.HangMucKeHoachVon{}, err
	}
	if err := domain.KiemTraThuTu(yc.ThuTu); err != nil {
		return domain.HangMucKeHoachVon{}, err
	}
	if yc.Color != nil {
		color, err := domain.NormalizeCatalogueColor(*yc.Color)
		if err != nil {
			return domain.HangMucKeHoachVon{}, err
		}
		yc.Color = &color
	}

	attempts := 1
	if autoCode {
		attempts = categoryCreateAttempts
	}
	for attempt := 1; ; attempt++ {
		created, err := uc.createOnce(ctx, ma, nhan, yc, autoCode, nguoi)
		if err == nil {
			return created, nil
		}
		// ONLY the auto path retries, and only on the unique key: a TYPED code that collides is the
		// person's choice and is refused (409), never silently swapped for one they did not type.
		if !autoCode || attempt >= attempts || !errors.Is(err, docstore.ErrMaDaTonTai) {
			return domain.HangMucKeHoachVon{}, err
		}
	}
}

// createOnce is ONE transaction of Them. Split out so a retry is a whole new transaction: after
// `UNIQUE (tenant_id, ma)` refuses an INSERT, PostgreSQL has aborted the old one and nothing more can
// run in it.
func (uc *DanhMucHangMuc) createOnce(ctx context.Context, ma, nhan string, yc YeuCauThemHangMuc,
	autoCode bool, nguoi audit.Actor) (domain.HangMucKeHoachVon, error) {

	id, err := uc.sinhID()
	if err != nil {
		return domain.HangMucKeHoachVon{}, fmt.Errorf("danh_muc_hang_muc_ke_hoach_von: sinh mã: %w", err)
	}

	moi := domain.HangMucKeHoachVon{
		ID: id, Ma: ma, Nhan: nhan, ThuTu: yc.ThuTu,
		LaMacDinh: yc.LaMacDinh,
		Color:     colorValue(yc.Color), // normalised by Them
		DangDung:  true,
		// Set here only so the value this function RETURNS describes the row that was written. The
		// store does not read them: it writes 'don-vi' and false as literals.
		Nguon:          domain.NguonDonVi,
		MaNguonReNhanh: false,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.kho.DemDangSong(ctx, tx)
		if err != nil {
			return err
		}
		if n >= docstore.TranDanhMucHangMuc {
			return docstore.ErrDanhMucDayTran
		}

		if autoCode {
			if moi.Ma, err = uc.firstFreeCode(ctx, tx, domain.CategoryCodeFromLabel(nhan)); err != nil {
				return err
			}
		} else {
			daDung, err := uc.kho.MaDaDung(ctx, tx, ma)
			if err != nil {
				return err
			}
			if daDung {
				return docstore.ErrMaDaTonTai
			}
		}

		if moi.LaMacDinh {
			// BEFORE the insert, not after: `UNIQUE (tenant_id, moc_mac_dinh)` admits one live
			// default, and inserting the second one first is the statement that fails.
			if err := uc.kho.BoMacDinhKhac(ctx, tx, moi.ID); err != nil {
				return err
			}
		}
		if err := uc.kho.Chen(ctx, tx, moi); err != nil {
			return err
		}

		// `auto_code` says whether a person typed this code or the server issued it — the one fact
		// about the code an inspection cannot recover from the row (the same key `them_du_an` uses).
		delta, err := json.Marshal(map[string]any{"sau": tomTatHangMuc(moi), "auto_code": autoCode})
		if err != nil {
			return fmt.Errorf("danh_muc_hang_muc_ke_hoach_von: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides
		// which commune the entry belongs to.
		//
		// NOTHING IN THE DELTA IS PERSONAL DATA (rule 3): a document type is how the authority
		// classifies its paperwork, not anything about a person.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViThemHangMuc,
			Subject: moi.Ma, // the business code, never the internal id
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no row, no trail. The two states agree.
		return domain.HangMucKeHoachVon{}, boc(ctx, "thêm", err)
	}
	return moi, nil
}

// firstFreeCode walks the series base, base-2 … base-99 and returns the first code never used in
// this commune — soft-deleted rows included, because MaDaDung counts them (rule 7, invariant 3).
// Past the last suffix it refuses rather than inventing a code nobody can read back to the label.
func (uc *DanhMucHangMuc) firstFreeCode(ctx context.Context, tx *store.ScopedTx, base string) (string, error) {
	for n := 1; n <= domain.CategoryCodeSuffixLimit; n++ {
		candidate := domain.CategoryCodeCandidate(base, n)
		used, err := uc.kho.MaDaDung(ctx, tx, candidate)
		if err != nil {
			return "", err
		}
		if !used {
			return candidate, nil
		}
	}
	return "", domain.ErrCategoryCodeSeriesBlocked
}

// Sua applies a partial edit, refusing whatever this row's tier does not allow.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending the label a row already has is not an event;
// recording it would fill a public authority's ledger with entries saying nothing changed, and
// those are the entries that bury the ones carrying legal weight. It is also what makes this route
// genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
func (uc *DanhMucHangMuc) Sua(ctx context.Context, id string, yc YeuCauSuaHangMuc,
	nguoi audit.Actor) (domain.HangMucKeHoachVon, error) {

	if id == "" {
		return domain.HangMucKeHoachVon{}, docstore.ErrDanhMucKhongTonTai
	}
	// Shape first, outside the transaction, for the same reason as Them.
	var nhan string
	if yc.Nhan != nil {
		var err error
		if nhan, err = domain.ChuanHoaNhan(*yc.Nhan); err != nil {
			return domain.HangMucKeHoachVon{}, err
		}
	}
	if yc.ThuTu != nil {
		if err := domain.KiemTraThuTu(*yc.ThuTu); err != nil {
			return domain.HangMucKeHoachVon{}, err
		}
	}
	var color string
	if c := yc.Color; c != nil && c.Color != nil {
		var err error
		if color, err = domain.NormalizeCatalogueColor(*c.Color); err != nil {
			return domain.HangMucKeHoachVon{}, err
		}
	}

	var sau domain.HangMucKeHoachVon
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}

		sau = truoc
		if yc.Nhan != nil {
			sau.Nhan = nhan
		}
		if yc.ThuTu != nil {
			sau.ThuTu = *yc.ThuTu
		}
		if yc.DangDung != nil {
			sau.DangDung = *yc.DangDung
		}
		if yc.LaMacDinh != nil {
			sau.LaMacDinh = *yc.LaMacDinh
		}
		if yc.Color != nil {
			sau.Color = color // "" when the change clears it
		}

		// THE TIER CHECK IS ON THE TRANSITION, not on the requested value. Asking a tier-3 row to
		// stay enabled is not an attempt to disable it, and refusing that would make the ordinary
		// "save the whole form" request fail on exactly the rows a commune may not touch.
		if truoc.DangDung && !sau.DangDung {
			if err := truoc.Tang().ChoTat(); err != nil {
				return err
			}
		}

		if khongDoiHangMuc(truoc, sau) {
			return nil
		}

		if sau.LaMacDinh && !truoc.LaMacDinh {
			if err := uc.kho.BoMacDinhKhac(ctx, tx, sau.ID); err != nil {
				return err
			}
		}
		if err := uc.kho.CapNhat(ctx, tx, sau); err != nil {
			return err
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to
		// find in a ledger that is never deleted.
		delta, err := json.Marshal(map[string]any{
			"truoc": tomTatDoiHangMuc(truoc, sau, true),
			"sau":   tomTatDoiHangMuc(truoc, sau, false),
		})
		if err != nil {
			return fmt.Errorf("danh_muc_hang_muc_ke_hoach_von: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaHangMuc,
			Subject: sau.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.HangMucKeHoachVon{}, boc(ctx, "sửa", err)
	}
	return sau, nil
}

// Xoa soft deletes one row — tier 1 only.
//
// THIS IS NOT A DELETE AND THE NAME IS THE ONLY PLACE THAT COULD SUGGEST OTHERWISE. The row stays,
// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma` stays
// taken forever: an issued code is never reissued, because business records hold it as a value.
//
// THE REASON IS OPTIONAL (user decision 07/10/2026): blank → domain.CategoryRemovalDefaultReason, so
// `delete_reason` is never empty (domain.ChuanHoaLyDoXoa says why).
func (uc *DanhMucHangMuc) Xoa(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
	if id == "" {
		return docstore.ErrDanhMucKhongTonTai
	}
	lyDo, err := domain.ChuanHoaLyDoXoa(lyDoTho)
	if err != nil {
		return err
	}
	if nguoi.ID == "" {
		// `deleted_by` with nothing in it is a deletion nobody can be asked about. core/audit
		// refuses an entry with no actor for the same reason; refusing here keeps the column and
		// the entry telling the same story.
		return fmt.Errorf("danh_muc_hang_muc_ke_hoach_von: thiếu người xoá")
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := truoc.Tang().ChoXoaMem(); err != nil {
			return err
		}
		if err := uc.kho.XoaMem(ctx, tx, truoc.ID, nguoi.ID, lyDo); err != nil {
			return err
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication
		// rule 9 forbids: the column is the current state of the row and can only ever hold the
		// FIRST deletion, while the entry is the append-only record of the act. They answer two
		// different questions and neither can be derived from the other.
		delta, err := json.Marshal(map[string]any{
			"truoc":   tomTatHangMuc(truoc),
			"ly_do":   lyDo,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("danh_muc_hang_muc_ke_hoach_von: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViXoaHangMuc,
			Subject: truoc.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return boc(ctx, "xoá", err)
	}
	return nil
}

// tomTat is the audit delta's view of one row. `nguon` is in it BECAUSE it is the fact that decides
// what may later be done to this row, and an inspection reading the entry has no other way to know
// which tier the row was in at the time.
func tomTatHangMuc(l domain.HangMucKeHoachVon) map[string]any {
	return map[string]any{
		"ma":          l.Ma,
		"nhan":        l.Nhan,
		"thu_tu":      l.ThuTu,
		"dang_dung":   l.DangDung,
		"la_mac_dinh": l.LaMacDinh,
		"nguon":       l.Nguon,
		"color":       colorDelta(l.Color),
	}
}

// tomTatDoi returns only the fields that actually moved, from whichever side is asked for.
func tomTatDoiHangMuc(truoc, sau domain.HangMucKeHoachVon, ben bool) map[string]any {
	ra := map[string]any{}
	if truoc.Nhan != sau.Nhan {
		ra["nhan"] = chon(ben, truoc.Nhan, sau.Nhan)
	}
	if truoc.ThuTu != sau.ThuTu {
		ra["thu_tu"] = chon(ben, truoc.ThuTu, sau.ThuTu)
	}
	if truoc.DangDung != sau.DangDung {
		ra["dang_dung"] = chon(ben, truoc.DangDung, sau.DangDung)
	}
	if truoc.LaMacDinh != sau.LaMacDinh {
		ra["la_mac_dinh"] = chon(ben, truoc.LaMacDinh, sau.LaMacDinh)
	}
	if truoc.Color != sau.Color {
		ra["color"] = chon(ben, colorDelta(truoc.Color), colorDelta(sau.Color))
	}
	return ra
}

// khongDoi reports whether the edit would change nothing. `ma`, `nguon` and `ma_nguon_re_nhanh` are
// not compared because no path here can change them.
func khongDoiHangMuc(truoc, sau domain.HangMucKeHoachVon) bool {
	return truoc.Nhan == sau.Nhan && truoc.ThuTu == sau.ThuTu &&
		truoc.DangDung == sau.DangDung && truoc.LaMacDinh == sau.LaMacDinh && truoc.Color == sau.Color
}

// colorValue reads an already-normalised optional colour; nil is "" (none).
func colorValue(c *string) string {
	if c == nil {
		return ""
	}
	return *c
}
