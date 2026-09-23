package app

// The use cases behind the WRITE surface of the disbursement voucher register
// (docs/ui-ux/06-giai-ngan.md §8.2, §13 rule 3).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes
// outside one. Opening that transaction is this layer's job. The handler translates HTTP and
// nothing else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THE LIFECYCLE. Every write here is read-decide-write: read the voucher under
// a row lock, ask the domain whether this operation is admissible from that state, refuse or apply.
// That decision needs the row AND the rules AND the transaction at once, which is exactly one
// place — here.
//
// ---------------------------------------------------------------------------
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE. Said once, here, because it is the
// thing most likely to be misread as duplication:
//
//	CHECK (so_tien > 0)               0004:305
//	the three states                  0004:298
//	a locked voucher is frozen        0004, trigger `chung_tu_da_khoa` (:141-168)
//	an unlock carries its reason      0005, `chung_tu_giai_ngan_mo_khoa_du_vet`
//	hard DELETE refused outright      0004, `ho_so_luu_tru_cam_xoa_cung`
//
// Those hold against every writer — this service, a psql prompt, an import job written next year.
// What they cannot do is explain themselves: PostgreSQL answers with an exception whose text is
// English, names a constraint, and tells an accountant in a commune nothing they can act on. This
// layer refuses FIRST, in Vietnamese, naming the operation and the way out. A drift between the two
// is therefore a worse error message, never a hole — the constraint still runs last, and the whole
// transaction rolls back with the audit entry inside it.
//
// ---------------------------------------------------------------------------
// WHAT IS NOT BUILT HERE, deliberately:
//
//	the refund (khoản hoàn)   ADR 0035 §B and open question #30. A refund is a SEPARATE voucher,
//	                          not a negative amount and not an edit of an old voucher down to a
//	                          smaller figure. `CHECK (so_tien > 0)` is untouched, and Sua's audit
//	                          delta is what makes the tempting detour visible if anybody takes it.
//	a ceiling on unlocks      decision (3) of migration 0005: counted, never capped. A ceiling is
//	                          a number that belongs to the customer, and a count is what lets them
//	                          choose one later from real figures.
//	"which threshold was in   nothing in this repository persists a reported period's disbursement
//	force when we reported"   figures, so there is nothing that could disagree with itself yet.
//	                          The day a reported period IS stored, the threshold used has to be
//	                          stored in the same row — 0005's closing note says why.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// KhoChungTu is the store, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the voucher in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that the unlock rule compares the right two staff codes — provable
// without a PostgreSQL. There is none reachable from this repository's build environment, so a test
// that needed one would be a test that never runs.
type KhoChungTu interface {
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.ChungTuGiaiNgan, error)
	MaDuAnConSong(ctx context.Context, tx *store.ScopedTx, duAnID string) (string, error)
	Chen(ctx context.Context, tx *store.ScopedTx, ct domain.ChungTuGiaiNgan) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, ct domain.ChungTuGiaiNgan) error
	XacNhan(ctx context.Context, tx *store.ScopedTx, id, maCanBo string) error
	VeNhapSauKhiSua(ctx context.Context, tx *store.ScopedTx, id string) error
	Khoa(ctx context.Context, tx *store.ScopedTx, id, maCanBo string, luc time.Time) error
	MoKhoa(ctx context.Context, tx *store.ScopedTx, id string,
		trangThaiVe domain.TrangThaiChungTu, maCanBo, lyDo string, luc time.Time) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
}

// ChungTuGiaiNgan owns entering, correcting, confirming, freezing, reopening and removing one
// commune's disbursement vouchers.
type ChungTuGiaiNgan struct {
	db  *store.DB
	kho KhoChungTu

	// sinhID is injected so a test can pin the id. In production it is ulid.Moi.
	sinhID func() (string, error)

	// nay is the clock `thoi_diem_khoa` and `thoi_diem_mo_khoa` are stamped from.
	//
	// A SEAM AND NOT time.Now() AT THE CALL SITE, for a reason that is about evidence rather than
	// convenience: both columns are read years later beside an audit entry carrying its own `at`,
	// and a test that cannot pin the instant cannot assert that the two agree. In production this
	// is nil and nayHoac returns the real clock.
	nay func() time.Time
}

func NewChungTuGiaiNgan(db *store.DB, kho KhoChungTu) *ChungTuGiaiNgan {
	return &ChungTuGiaiNgan{db: db, kho: kho, sinhID: ulid.Moi}
}

// nayHoac is the clock, UTC. `TIMESTAMPTZ` stores an instant rather than a wall reading, so the
// location here changes nothing that is stored — it is fixed so that a value read back in a test
// compares equal without a location dance.
func (uc *ChungTuGiaiNgan) nayHoac() time.Time {
	if uc.nay == nil {
		return time.Now().UTC()
	}
	return uc.nay().UTC()
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `them_hang_muc_ke_hoach_von`): an inspection reads these
// strings, and a function name would tell them nothing.
//
// SIX VERBS AND NOT ONE `sua_chung_tu`, because they are six different administrative acts with six
// different consequences. `mo_khoa_chung_tu_giai_ngan` in particular is the one an inspection
// searches for by name — it is the record of a signed figure being reopened.
const (
	HanhViThemChungTu    = "them_chung_tu_giai_ngan"
	HanhViSuaChungTu     = "sua_chung_tu_giai_ngan"
	HanhViGoChungTu      = "go_chung_tu_giai_ngan"
	HanhViXacNhanChungTu = "xac_nhan_chung_tu_giai_ngan"
	HanhViKhoaChungTu    = "khoa_chung_tu_giai_ngan"
	HanhViMoKhoaChungTu  = "mo_khoa_chung_tu_giai_ngan"
)

// YeuCauThemChungTu is one new voucher, as it arrives from the handler.
//
// THERE IS NO `TrangThai` FIELD AND THERE MUST NEVER BE ONE. A new voucher is `Kế toán nhập`,
// always: the store writes the state as a LITERAL, so there is no value any layer above could pass.
// A field here is a field a handler can fill from a request body, and what that buys is a voucher
// created already `Đã khoá` — a figure nobody confirmed, frozen against editing, counting toward
// the commune's disbursement total.
//
// THERE IS NO `NguoiNhapID` FIELD EITHER. Who entered it is the acting principal, which arrives as
// the audit.Actor beside the request; a field would be a second, client-supplied answer to a
// question the session already answers.
type YeuCauThemChungTu struct {
	DuAnID    string
	NgayChi   time.Time
	SoTien    domain.Dong
	NoiDung   string
	DoiTac    string
	SoChungTu string
}

// YeuCauSuaChungTu is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. `DoiTac` and `SoChungTu` are optional and their
// meaningful value includes the empty string — "this voucher has no treasury number" is a statement,
// not an absence of one. A struct of plain values cannot tell "the client did not mention this" from
// "the client cleared it", so a screen editing only the description would silently wipe the
// counterparty off a payment record.
//
// `DuAnID` IS ABSENT AND IS NOT AN OVERSIGHT: moving a voucher between projects moves money between
// two reported totals with nothing on either screen saying so. The operation for a voucher filed
// against the wrong project is to remove it with a reason and enter it again — two events, both
// audited, both visible.
type YeuCauSuaChungTu struct {
	NgayChi   *time.Time
	SoTien    *domain.Dong
	NoiDung   *string
	DoiTac    *string
	SoChungTu *string
}

// Them records one payment against one project.
//
// THE SHAPE IS VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never
// hold a row lock while doing so, and the caller needs the reason rather than a rollback.
func (uc *ChungTuGiaiNgan) Them(ctx context.Context, yc YeuCauThemChungTu,
	nguoi audit.Actor) (domain.ChungTuGiaiNgan, error) {

	if yc.DuAnID == "" {
		return domain.ChungTuGiaiNgan{}, domain.ErrThieuDuAn
	}
	if err := domain.KiemTraSoTien(yc.SoTien); err != nil {
		return domain.ChungTuGiaiNgan{}, err
	}
	if err := domain.KiemTraNgayChi(yc.NgayChi); err != nil {
		return domain.ChungTuGiaiNgan{}, err
	}
	noiDung, err := domain.ChuanHoaNoiDung(yc.NoiDung)
	if err != nil {
		return domain.ChungTuGiaiNgan{}, err
	}
	doiTac, err := domain.ChuanHoaDoiTac(yc.DoiTac)
	if err != nil {
		return domain.ChungTuGiaiNgan{}, err
	}
	soChungTu, err := domain.ChuanHoaSoChungTu(yc.SoChungTu)
	if err != nil {
		return domain.ChungTuGiaiNgan{}, err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.ChungTuGiaiNgan{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.ChungTuGiaiNgan{}, fmt.Errorf("chung_tu_giai_ngan: sinh mã: %w", err)
	}

	moi := domain.ChungTuGiaiNgan{
		ID:        id,
		DuAnID:    yc.DuAnID,
		NgayChi:   yc.NgayChi,
		SoTien:    yc.SoTien,
		NoiDung:   noiDung,
		DoiTac:    doiTac,
		SoChungTu: soChungTu,
		// Set here only so the value this function RETURNS describes the row that was written. The
		// store does not read it: it writes 'ke-toan-nhap' as a literal.
		TrangThai: domain.ChungTuKeToanNhap,
		// THE STAFF BUSINESS CODE, never the internal id — migration 0005:52-59 states the
		// convention for all four `nguoi_*_id` columns, and audit.Actor.ID already carries exactly
		// that value (the handler reads `Principal.Ma`, rule 6, invariant 8).
		NguoiNhapID: nguoi.ID,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE PROJECT IS READ FIRST, AND FOR TWO REASONS AT ONCE: it refuses a voucher filed
		// against a project this commune does not have (money that would total nowhere), and it
		// yields the BUSINESS CODE the audit entry is filed under. A voucher has no code of its
		// own — there is no `ma` column on the table — so `subject` is the project's, and the
		// voucher is named inside the delta.
		maDuAn, err := uc.kho.MaDuAnConSong(ctx, tx, moi.DuAnID)
		if err != nil {
			return err
		}
		if err := uc.kho.Chen(ctx, tx, moi); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": tomTatChungTu(moi)})
		if err != nil {
			return fmt.Errorf("chung_tu_giai_ngan: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides
		// which commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViThemChungTu,
			Subject: maDuAn,
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no voucher, no trail. The two states agree.
		return domain.ChungTuGiaiNgan{}, bocChungTu(ctx, "thêm", err)
	}
	return moi, nil
}

// Sua corrects an UNLOCKED voucher.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending a voucher the figures it already has is not an
// event; recording it would fill a public authority's ledger with entries saying nothing changed,
// and those are the entries that bury the ones carrying legal weight. It is also what makes this
// route genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
//
// THE LOCKED CASE IS REFUSED BEFORE ANY UPDATE IS ATTEMPTED, and that is the whole point of the
// layer: the trigger would refuse it too, with an English exception naming a constraint. §13 rule 3
// is what an accountant is owed instead — "chứng từ đã khoá thì không sửa, không gỡ — phải mở khoá
// trước (quyền `budget.confirm`)".
func (uc *ChungTuGiaiNgan) Sua(ctx context.Context, id string, yc YeuCauSuaChungTu,
	nguoi audit.Actor) (domain.ChungTuGiaiNgan, error) {

	if id == "" {
		return domain.ChungTuGiaiNgan{}, fistore.ErrKhongThayChungTu
	}
	// Shape first, outside the transaction, for the same reason as Them.
	if yc.SoTien != nil {
		if err := domain.KiemTraSoTien(*yc.SoTien); err != nil {
			return domain.ChungTuGiaiNgan{}, err
		}
	}
	if yc.NgayChi != nil {
		if err := domain.KiemTraNgayChi(*yc.NgayChi); err != nil {
			return domain.ChungTuGiaiNgan{}, err
		}
	}
	var noiDung, doiTac, soChungTu string
	var err error
	if yc.NoiDung != nil {
		if noiDung, err = domain.ChuanHoaNoiDung(*yc.NoiDung); err != nil {
			return domain.ChungTuGiaiNgan{}, err
		}
	}
	if yc.DoiTac != nil {
		if doiTac, err = domain.ChuanHoaDoiTac(*yc.DoiTac); err != nil {
			return domain.ChungTuGiaiNgan{}, err
		}
	}
	if yc.SoChungTu != nil {
		if soChungTu, err = domain.ChuanHoaSoChungTu(*yc.SoChungTu); err != nil {
			return domain.ChungTuGiaiNgan{}, err
		}
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.ChungTuGiaiNgan{}, err
	}

	var sau domain.ChungTuGiaiNgan
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := truoc.ChoSua(); err != nil {
			return err
		}

		sau = truoc
		if yc.NgayChi != nil {
			sau.NgayChi = *yc.NgayChi
		}
		if yc.SoTien != nil {
			sau.SoTien = *yc.SoTien
		}
		if yc.NoiDung != nil {
			sau.NoiDung = noiDung
		}
		if yc.DoiTac != nil {
			sau.DoiTac = doiTac
		}
		if yc.SoChungTu != nil {
			sau.SoChungTu = soChungTu
		}

		if khongDoiChungTu(truoc, sau) {
			return nil
		}

		maDuAn, err := uc.kho.MaDuAnConSong(ctx, tx, truoc.DuAnID)
		if err != nil {
			return err
		}
		if err := uc.kho.CapNhat(ctx, tx, sau); err != nil {
			return err
		}

		// A CONFIRMED VOUCHER GOES BACK TO `Kế toán nhập` BECAUSE ITS FIGURES JUST MOVED.
		// *"Lãnh đạo xác nhận những con số kia, không phải những con số này."* The customer's rule,
		// measured in `../vigov-require` commit `c3f4d6a`; domain.TrangThaiSauKhiSua owns the decision
		// and this is the only caller.
		//
		// IT RUNS ONLY WHEN SOMETHING REALLY CHANGED. The no-op branch above has already returned, so
		// a repeat of an identical PATCH cannot strip a confirmation off a voucher nobody edited —
		// which would make `idem.KhongCan` on that route a lie AND undo a leader's act for nothing.
		veNhap := domain.TrangThaiSauKhiSua(truoc.TrangThai) != truoc.TrangThai
		if veNhap {
			if err := uc.kho.VeNhapSauKhiSua(ctx, tx, truoc.ID); err != nil {
				return err
			}
			sau.TrangThai = domain.TrangThaiSauKhiSua(truoc.TrangThai)
			// The column is cleared with the state, so the row stops naming somebody as the confirmer
			// of figures they have not seen. Who HAD confirmed it is in the entry below, which is
			// append-only.
			sau.NguoiXacNhanID = ""
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to
		// find in a ledger that is never deleted.
		//
		// THE AMOUNT IS THE FIELD THIS DELTA EXISTS FOR. Open question #30 says a refund is a
		// separate voucher, not a sign change — and the detour that would avoid ever recording a
		// refund is editing an old voucher DOWN to a smaller figure. That edit is legitimate as a
		// correction and indistinguishable from the detour on the row itself; the only thing that
		// tells them apart afterwards is this pair of numbers in an append-only ledger.
		than := map[string]any{
			"chung_tu_id": sau.ID,
			"truoc":       tomTatDoiChungTu(truoc, sau, true),
			"sau":         tomTatDoiChungTu(truoc, sau, false),
		}
		if veNhap {
			// RECORDED AS ITS OWN FACT, NOT FOLDED INTO `truoc`/`sau`. Losing a confirmation is not a
			// field the accountant edited — it is a consequence of the edit, and it undoes a named
			// person's act. `nguoi_xac_nhan_id` is nulled on the row, so this entry is the ONLY place
			// that still answers "who had confirmed these figures before they were changed".
			than["mat_xac_nhan"] = map[string]any{
				"truoc_trang_thai":  string(truoc.TrangThai),
				"sau_trang_thai":    string(sau.TrangThai),
				"nguoi_xac_nhan_cu": truoc.NguoiXacNhanID,
			}
		}
		delta, err := json.Marshal(than)
		if err != nil {
			return fmt.Errorf("chung_tu_giai_ngan: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaChungTu,
			Subject: maDuAn,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.ChungTuGiaiNgan{}, bocChungTu(ctx, "sửa", err)
	}
	return sau, nil
}

// Go soft deletes one voucher — `🗑 Gỡ` on the §8.2 screen.
//
// THIS IS NOT A DELETE AND THE NAME IS THE ONLY PLACE THAT COULD SUGGEST OTHERWISE. The row stays,
// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1); its money leaves
// `da_giai_ngan` because every read path excludes soft-deleted rows, not because anything was
// destroyed. `ho_so_luu_tru_cam_xoa_cung` refuses a hard DELETE on this table outright.
//
// THE REASON IS MANDATORY. A voucher that vanished from a project's total with no reason attached
// is money nobody can account for — and the row is still there, so the question WILL be asked.
func (uc *ChungTuGiaiNgan) Go(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
	if id == "" {
		return fistore.ErrKhongThayChungTu
	}
	lyDo, err := domain.ChuanHoaLyDoGo(lyDoTho)
	if err != nil {
		return err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := truoc.ChoGo(); err != nil {
			return err
		}
		maDuAn, err := uc.kho.MaDuAnConSong(ctx, tx, truoc.DuAnID)
		if err != nil {
			return err
		}
		// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds. Two
		// kinds of identifier in one column is a column nobody can query (rule 6, invariant 8).
		if err := uc.kho.XoaMem(ctx, tx, truoc.ID, nguoi.ID, lyDo); err != nil {
			return err
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication
		// rule 9 forbids: the column is the current state of the row and can only ever hold the
		// FIRST removal, while the entry is the append-only record of the act. They answer two
		// different questions and neither can be derived from the other.
		delta, err := json.Marshal(map[string]any{
			"chung_tu_id": truoc.ID,
			"truoc":       tomTatChungTu(truoc),
			"ly_do":       lyDo,
			"xoa_mem":     true,
		})
		if err != nil {
			return fmt.Errorf("chung_tu_giai_ngan: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViGoChungTu,
			Subject: maDuAn,
			Delta:   delta,
		})
	})
	if err != nil {
		return bocChungTu(ctx, "gỡ", err)
	}
	return nil
}

// XacNhan moves a voucher from `Kế toán nhập` to `Đã xác nhận` (§8.2, `budget.confirm`).
func (uc *ChungTuGiaiNgan) XacNhan(ctx context.Context, id string,
	nguoi audit.Actor) (domain.ChungTuGiaiNgan, error) {

	return uc.doiTrangThai(ctx, id, nguoi, HanhViXacNhanChungTu,
		func(tx *store.ScopedTx, truoc domain.ChungTuGiaiNgan) (domain.ChungTuGiaiNgan, map[string]any, error) {
			if err := truoc.ChoXacNhan(); err != nil {
				return domain.ChungTuGiaiNgan{}, nil, err
			}
			if err := uc.kho.XacNhan(ctx, tx, truoc.ID, nguoi.ID); err != nil {
				return domain.ChungTuGiaiNgan{}, nil, err
			}
			sau := truoc
			sau.TrangThai = domain.ChungTuDaXacNhan
			sau.NguoiXacNhanID = nguoi.ID
			return sau, map[string]any{
				"chung_tu_id": truoc.ID,
				"truoc":       map[string]any{"trang_thai": string(truoc.TrangThai)},
				"sau":         map[string]any{"trang_thai": string(sau.TrangThai)},
				"so_tien":     int64(truoc.SoTien),
			}, nil
		})
}

// Khoa freezes a voucher (§8.2, `budget.confirm`). From `Đã xác nhận` only — see
// domain.ErrChuaXacNhanThiChuaKhoaDuoc for why the chain is required even though the screen draws
// both buttons on a `Kế toán nhập` row.
func (uc *ChungTuGiaiNgan) Khoa(ctx context.Context, id string,
	nguoi audit.Actor) (domain.ChungTuGiaiNgan, error) {

	luc := uc.nayHoac()
	return uc.doiTrangThai(ctx, id, nguoi, HanhViKhoaChungTu,
		func(tx *store.ScopedTx, truoc domain.ChungTuGiaiNgan) (domain.ChungTuGiaiNgan, map[string]any, error) {
			if err := truoc.ChoKhoa(); err != nil {
				return domain.ChungTuGiaiNgan{}, nil, err
			}
			if err := uc.kho.Khoa(ctx, tx, truoc.ID, nguoi.ID, luc); err != nil {
				return domain.ChungTuGiaiNgan{}, nil, err
			}
			sau := truoc
			sau.TrangThai = domain.ChungTuDaKhoa
			sau.NguoiKhoaID = nguoi.ID
			sau.ThoiDiemKhoa = luc
			return sau, map[string]any{
				"chung_tu_id": truoc.ID,
				"truoc":       map[string]any{"trang_thai": string(truoc.TrangThai)},
				"sau":         map[string]any{"trang_thai": string(sau.TrangThai)},
				"so_tien":     int64(truoc.SoTien),
			}, nil
		})
}

// MoKhoa reopens a frozen voucher (ADR 0035 §B, `budget.confirm`).
//
// TWO RULES, BOTH DECIDED BY THIS PROJECT ON 2026-09-22 rather than by the customer, in the
// direction that can be loosened later with one line and cannot be tightened later at all
// (migration 0005's header sets both out in full):
//
//	the reason is MANDATORY        the trail of unlocks that have already happened cannot be
//	                              rebuilt from any source, and it is exactly the figure an
//	                              inspection asks about — one somebody signed and somebody changed.
//	the person who locked it       nobody acts alone on the act that gives themselves room. Same
//	  may NOT unlock it            shape the customer settled for the staff register in #13 and #14.
//
// THE SECOND ONE ANSWERS 409, NOT 403, and that is not a detail: the caller HOLDS `budget.confirm`
// and is allowed to unlock vouchers. What is refused is this person against THIS row. A 403 would
// send them to the Phân quyền screen to be granted a permission they already have.
//
// THE COUNT IS INCREMENTED, NEVER CAPPED — decision (3). A ceiling is a number that belongs to the
// customer; a count is what lets them pick one later from real figures instead of somebody's guess.
func (uc *ChungTuGiaiNgan) MoKhoa(ctx context.Context, id, lyDoTho string,
	nguoi audit.Actor) (domain.ChungTuGiaiNgan, error) {

	if id == "" {
		return domain.ChungTuGiaiNgan{}, fistore.ErrKhongThayChungTu
	}
	// THE REASON IS VALIDATED BEFORE THE TRANSACTION OPENS, so an unlock with no reason never holds
	// a row lock and never reaches the constraint. `chung_tu_giai_ngan_mo_khoa_du_vet` refuses the
	// same thing underneath; this is the sentence that arrives first.
	lyDo, err := domain.ChuanHoaLyDoMoKhoa(lyDoTho)
	if err != nil {
		return domain.ChungTuGiaiNgan{}, err
	}

	luc := uc.nayHoac()
	return uc.doiTrangThai(ctx, id, nguoi, HanhViMoKhoaChungTu,
		func(tx *store.ScopedTx, truoc domain.ChungTuGiaiNgan) (domain.ChungTuGiaiNgan, map[string]any, error) {
			// `nguoi.ID` IS THE STAFF BUSINESS CODE and `truoc.NguoiKhoaID` holds the same kind of
			// value (migration 0005:52-59). Comparing anything else here — an internal id, a display
			// name — compares two things that are not the same kind of identifier, and the comparison
			// would simply never be true, which reads as "the rule is off" rather than as a bug.
			if err := truoc.ChoMoKhoa(nguoi.ID); err != nil {
				return domain.ChungTuGiaiNgan{}, nil, err
			}
			ve := domain.TrangThaiSauKhiMoKhoa()
			if err := uc.kho.MoKhoa(ctx, tx, truoc.ID, ve, nguoi.ID, lyDo, luc); err != nil {
				return domain.ChungTuGiaiNgan{}, nil, err
			}
			sau := truoc
			sau.TrangThai = ve
			sau.NguoiMoKhoaID = nguoi.ID
			sau.ThoiDiemMoKhoa = luc
			sau.LyDoMoKhoa = lyDo
			sau.SoLanMoKhoa = truoc.SoLanMoKhoa + 1
			return sau, map[string]any{
				"chung_tu_id": truoc.ID,
				"truoc": map[string]any{
					"trang_thai":    string(truoc.TrangThai),
					"nguoi_khoa_id": truoc.NguoiKhoaID,
				},
				"sau":            map[string]any{"trang_thai": string(sau.TrangThai)},
				"ly_do":          lyDo,
				"so_lan_mo_khoa": sau.SoLanMoKhoa,
				"so_tien":        int64(truoc.SoTien),
			}, nil
		})
}

// doiTrangThai is the read-decide-write shared by the three lifecycle moves.
//
// ONE FUNCTION AND NOT THREE COPIES, because the part that is identical is the part that is easy to
// get subtly wrong in one copy: the row lock, the project code for the subject, and the audit entry
// INSIDE the same transaction. What differs — which transition is admissible, what is written, what
// the delta says — is the closure, so a new lifecycle move cannot accidentally reuse another's
// decision.
func (uc *ChungTuGiaiNgan) doiTrangThai(ctx context.Context, id string, nguoi audit.Actor,
	hanhVi string,
	ap func(tx *store.ScopedTx, truoc domain.ChungTuGiaiNgan) (domain.ChungTuGiaiNgan, map[string]any, error),
) (domain.ChungTuGiaiNgan, error) {

	if id == "" {
		return domain.ChungTuGiaiNgan{}, fistore.ErrKhongThayChungTu
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.ChungTuGiaiNgan{}, err
	}

	var sau domain.ChungTuGiaiNgan
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		maDuAn, err := uc.kho.MaDuAnConSong(ctx, tx, truoc.DuAnID)
		if err != nil {
			return err
		}
		moi, than, err := ap(tx, truoc)
		if err != nil {
			return err
		}
		sau = moi

		delta, err := json.Marshal(than)
		if err != nil {
			return fmt.Errorf("chung_tu_giai_ngan: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  hanhVi,
			Subject: maDuAn,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.ChungTuGiaiNgan{}, bocChungTu(ctx, hanhVi, err)
	}
	return sau, nil
}

// coNguoiThucHien refuses a write whose trail cannot name who made it.
//
// core/audit refuses an entry with no actor for the same reason; refusing HERE, before the
// transaction opens, keeps `nguoi_nhap_id` / `deleted_by` / `nguoi_khoa_id` and the entry telling
// the same story — and avoids a rollback whose cause is a missing principal rather than anything
// about the voucher. Rule 6 does not permit a business write whose trail cannot name its author.
func coNguoiThucHien(nguoi audit.Actor) error {
	if nguoi.ID == "" {
		return fmt.Errorf("chung_tu_giai_ngan: thiếu người thực hiện")
	}
	return nil
}

// bocChungTu wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No amount, no counterparty, no description: an error travels into centralised logging across every
// commune at once, and a voucher's free-text description is somebody's typing about a public
// authority's spending. The commune is not personal data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is.
// A %v here would collapse "this voucher is locked" and "the database is down" into one 500.
func bocChungTu(ctx context.Context, viec string, err error) error {
	return fmt.Errorf("chung_tu_giai_ngan: %s cho xã %s: %w", viec, tenant.MustFrom(ctx), err)
}

// tomTatChungTu is the audit delta's view of one voucher.
//
// NOTHING HERE IS PERSONAL DATA (rule 3). `doi_tac` is a company — 0004:283-284 says so outright,
// and the domain repeats it: a commune that starts typing an individual's name and national ID into
// that box turns a disbursement register into a store of personal data under Decree 13, which is a
// decision for the customer rather than something to accommodate here.
func tomTatChungTu(c domain.ChungTuGiaiNgan) map[string]any {
	return map[string]any{
		"chung_tu_id": c.ID,
		"du_an_id":    c.DuAnID,
		"ngay_chi":    c.NgayChi.Format("2006-01-02"),
		"so_tien":     int64(c.SoTien),
		"noi_dung":    c.NoiDung,
		"doi_tac":     c.DoiTac,
		"so_chung_tu": c.SoChungTu,
		"trang_thai":  string(c.TrangThai),
	}
}

// tomTatDoiChungTu returns only the fields that actually moved, from whichever side is asked for.
func tomTatDoiChungTu(truoc, sau domain.ChungTuGiaiNgan, ben bool) map[string]any {
	ra := map[string]any{}
	if !truoc.NgayChi.Equal(sau.NgayChi) {
		ra["ngay_chi"] = chon(ben, truoc.NgayChi, sau.NgayChi).Format("2006-01-02")
	}
	if truoc.SoTien != sau.SoTien {
		ra["so_tien"] = int64(chon(ben, truoc.SoTien, sau.SoTien))
	}
	if truoc.NoiDung != sau.NoiDung {
		ra["noi_dung"] = chon(ben, truoc.NoiDung, sau.NoiDung)
	}
	if truoc.DoiTac != sau.DoiTac {
		ra["doi_tac"] = chon(ben, truoc.DoiTac, sau.DoiTac)
	}
	if truoc.SoChungTu != sau.SoChungTu {
		ra["so_chung_tu"] = chon(ben, truoc.SoChungTu, sau.SoChungTu)
	}
	return ra
}

// khongDoiChungTu reports whether the edit would change nothing. The state and the four staff codes
// are not compared because no path through Sua can change them.
func khongDoiChungTu(truoc, sau domain.ChungTuGiaiNgan) bool {
	return truoc.NgayChi.Equal(sau.NgayChi) &&
		truoc.SoTien == sau.SoTien &&
		truoc.NoiDung == sau.NoiDung &&
		truoc.DoiTac == sau.DoiTac &&
		truoc.SoChungTu == sau.SoChungTu
}
