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
// the customer's call, not a use case's — see migrations/0003_danh_muc_loai_van_ban.sql:47-52. An
// empty catalogue is the correct state of every commune today.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// KhoLoaiVanBan is the store, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the row in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that `nguon` is never written from a request — provable without a
// PostgreSQL. A test that needs infrastructure is a test that stops being run.
type KhoLoaiVanBan interface {
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.LoaiVanBan, error)
	MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error)
	DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error)
	Chen(ctx context.Context, tx *store.ScopedTx, lvb domain.LoaiVanBan) error
	BoMacDinhKhac(ctx context.Context, tx *store.ScopedTx, trongID string) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, lvb domain.LoaiVanBan) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
}

// DanhMucLoaiVanBan owns adding, editing and retiring one commune's document types.
type DanhMucLoaiVanBan struct {
	db  *store.DB
	kho KhoLoaiVanBan

	// sinhID is injected so a test can pin the id. In production it is ulid.Moi.
	sinhID func() (string, error)
}

func NewDanhMucLoaiVanBan(db *store.DB, kho KhoLoaiVanBan) *DanhMucLoaiVanBan {
	return &DanhMucLoaiVanBan{db: db, kho: kho, sinhID: ulid.Moi}
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `ghi_so_thong_bao`, `xem_day_du_nguoi_gui`): an inspection
// reads these strings, and a function name would tell them nothing.
//
// THE VERB NAMES THE CATALOGUE, not just the operation. `sua_danh_muc` across five tables would
// make the trail unable to answer which list changed without joining to a row that may since have
// been edited again.
const (
	HanhViThemLoaiVanBan = "them_loai_van_ban"
	HanhViSuaLoaiVanBan  = "sua_loai_van_ban"
	HanhViXoaLoaiVanBan  = "xoa_loai_van_ban"
)

// YeuCauThemLoaiVanBan is one new row, as it arrives from the handler.
//
// THERE IS NO `Nguon` FIELD AND THERE MUST NEVER BE ONE. Provenance decides the tier, so a field
// here is a field a handler can fill from a request body — and the migration says what follows:
// "every guard below could be stepped around by setting nguon = 'don-vi' first". The store writes
// the value as a LITERAL for the same reason.
//
// THERE IS NO `DangDung` FIELD EITHER, and that is a smaller decision said out loud: a row the
// commune has just added is in use. Creating one already disabled is two requests — POST then
// PATCH — and the second one is the one that leaves a trail saying somebody turned it off.
type YeuCauThemLoaiVanBan struct {
	Ma        string
	Nhan      string
	ThuTu     int
	LaMacDinh bool
}

// YeuCauSuaLoaiVanBan is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. Three of the four fields have a meaningful zero —
// `thu_tu` 0 is the first position, `dang_dung` false is "taken out of use", `la_mac_dinh` false is
// "no longer the default". A struct of plain values cannot tell "the client did not mention this"
// from "the client set it to zero", so a screen that edits only the label would silently move the
// row to the top of the list and clear the commune's default.
type YeuCauSuaLoaiVanBan struct {
	Nhan      *string
	ThuTu     *int
	DangDung  *bool
	LaMacDinh *bool
}

// Them adds one row the commune owns.
//
// ORDER OF THE THREE REFUSALS, and it is not arbitrary: shape first (cheap, no lock), then the
// ceiling, then the duplicate code. The ceiling before the duplicate because a full catalogue is a
// condition of the whole list while a duplicate is a condition of one value — and the caller can
// act on the first without knowing anything about the second.
func (uc *DanhMucLoaiVanBan) Them(ctx context.Context, yc YeuCauThemLoaiVanBan,
	nguoi audit.Actor) (domain.LoaiVanBan, error) {

	// Validated BEFORE the transaction opens. A request that fails its shape must never hold a row
	// lock while doing so, and the caller needs the reason rather than a rollback.
	ma, err := domain.ChuanHoaMa(yc.Ma)
	if err != nil {
		return domain.LoaiVanBan{}, err
	}
	nhan, err := domain.ChuanHoaNhan(yc.Nhan)
	if err != nil {
		return domain.LoaiVanBan{}, err
	}
	if err := domain.KiemTraThuTu(yc.ThuTu); err != nil {
		return domain.LoaiVanBan{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.LoaiVanBan{}, fmt.Errorf("danh_muc_loai_van_ban: sinh mã: %w", err)
	}

	moi := domain.LoaiVanBan{
		ID: id, Ma: ma, Nhan: nhan, ThuTu: yc.ThuTu,
		LaMacDinh: yc.LaMacDinh,
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
		if n >= docstore.TranDanhMucLoaiVanBan {
			return docstore.ErrDanhMucDayTran
		}

		daDung, err := uc.kho.MaDaDung(ctx, tx, ma)
		if err != nil {
			return err
		}
		if daDung {
			return docstore.ErrMaDaTonTai
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

		delta, err := json.Marshal(map[string]any{"sau": tomTatLoaiVanBan(moi)})
		if err != nil {
			return fmt.Errorf("danh_muc_loai_van_ban: mã hoá delta: %w", err)
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
			Action:  HanhViThemLoaiVanBan,
			Subject: moi.Ma, // the business code, never the internal id
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no row, no trail. The two states agree.
		return domain.LoaiVanBan{}, boc(ctx, "thêm", err)
	}
	return moi, nil
}

// Sua applies a partial edit, refusing whatever this row's tier does not allow.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending the label a row already has is not an event;
// recording it would fill a public authority's ledger with entries saying nothing changed, and
// those are the entries that bury the ones carrying legal weight. It is also what makes this route
// genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
func (uc *DanhMucLoaiVanBan) Sua(ctx context.Context, id string, yc YeuCauSuaLoaiVanBan,
	nguoi audit.Actor) (domain.LoaiVanBan, error) {

	if id == "" {
		return domain.LoaiVanBan{}, docstore.ErrDanhMucKhongTonTai
	}
	// Shape first, outside the transaction, for the same reason as Them.
	var nhan string
	if yc.Nhan != nil {
		var err error
		if nhan, err = domain.ChuanHoaNhan(*yc.Nhan); err != nil {
			return domain.LoaiVanBan{}, err
		}
	}
	if yc.ThuTu != nil {
		if err := domain.KiemTraThuTu(*yc.ThuTu); err != nil {
			return domain.LoaiVanBan{}, err
		}
	}

	var sau domain.LoaiVanBan
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

		// THE TIER CHECK IS ON THE TRANSITION, not on the requested value. Asking a tier-3 row to
		// stay enabled is not an attempt to disable it, and refusing that would make the ordinary
		// "save the whole form" request fail on exactly the rows a commune may not touch.
		if truoc.DangDung && !sau.DangDung {
			if err := truoc.Tang().ChoTat(); err != nil {
				return err
			}
		}

		if khongDoiLoaiVanBan(truoc, sau) {
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
			"truoc": tomTatDoiLoaiVanBan(truoc, sau, true),
			"sau":   tomTatDoiLoaiVanBan(truoc, sau, false),
		})
		if err != nil {
			return fmt.Errorf("danh_muc_loai_van_ban: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaLoaiVanBan,
			Subject: sau.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.LoaiVanBan{}, boc(ctx, "sửa", err)
	}
	return sau, nil
}

// Xoa soft deletes one row — tier 1 only.
//
// THIS IS NOT A DELETE AND THE NAME IS THE ONLY PLACE THAT COULD SUGGEST OTHERWISE. The row stays,
// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma` stays
// taken forever: an issued code is never reissued, because business records hold it as a value.
func (uc *DanhMucLoaiVanBan) Xoa(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
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
		return fmt.Errorf("danh_muc_loai_van_ban: thiếu người xoá")
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
			"truoc":   tomTatLoaiVanBan(truoc),
			"ly_do":   lyDo,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("danh_muc_loai_van_ban: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViXoaLoaiVanBan,
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
func tomTatLoaiVanBan(l domain.LoaiVanBan) map[string]any {
	return map[string]any{
		"ma":          l.Ma,
		"nhan":        l.Nhan,
		"thu_tu":      l.ThuTu,
		"dang_dung":   l.DangDung,
		"la_mac_dinh": l.LaMacDinh,
		"nguon":       l.Nguon,
	}
}

// tomTatDoi returns only the fields that actually moved, from whichever side is asked for.
func tomTatDoiLoaiVanBan(truoc, sau domain.LoaiVanBan, ben bool) map[string]any {
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
	return ra
}

// khongDoi reports whether the edit would change nothing. `ma`, `nguon` and `ma_nguon_re_nhanh` are
// not compared because no path here can change them.
func khongDoiLoaiVanBan(truoc, sau domain.LoaiVanBan) bool {
	return truoc.Nhan == sau.Nhan && truoc.ThuTu == sau.ThuTu &&
		truoc.DangDung == sau.DangDung && truoc.LaMacDinh == sau.LaMacDinh
}
