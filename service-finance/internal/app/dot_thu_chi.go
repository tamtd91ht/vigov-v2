package app

// The use cases behind the `⇄ Các đợt thu, chi` dialog (docs/ui-ux/07-thu-chi-ngan-sach.md §5,
// migration 0008): record one batch against a leaf line, and remove one with a reason. Same shape as
// every write in thu_chi_ngan_sach.go — lock the sheet, read the tree under the lock, ask
// internal/domain, write, and write the audit entry in the SAME transaction (rule 6, invariant 3).
//
// WRITING A BATCH DOES NOT SWITCH THE LINE'S MODE (user decision 25/09/2026, §4.2). A batch on a
// `manual` leaf is stored and counts nowhere until somebody switches that leaf to `entries` through
// PATCH /api/v1/budget-lines/{id}. The audit entry records the mode the line had, so "why did this
// batch not move the figure" has an answer.
//
// ⚠ migration 0008's header says the line "must be a LEAF in `entries` mode when the batch is
// written". The user's later decision (no auto-switch, the user chooses) is what is implemented: the
// LEAF half is enforced, the `entries` half is not.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// YeuCauGhiDot is one batch as the dialog's form sends it.
//
// THERE IS NO `ID`, NO `NguoiGhiMa` AND NO `TaoLuc`: the id is issued here, the author is the session's
// staff code, and the timestamp is the database's.
type YeuCauGhiDot struct {
	KhoanMucID  string
	Ngay        time.Time
	NoiDung     string
	DonViCaNhan string // optional; PERSONAL DATA when it names a person
	SoChungTu   string // optional

	// GiaTri is cotID -> amount. A nil value is "left empty" and writes no row — 0008: "a column with
	// NO row is empty too". At least one amount must be non-nil (domain.ErrDotKhongCoSoTienNao).
	GiaTri map[string]*domain.Dong
}

// GhiDot records one batch and its amounts against one leaf line.
//
// REFUSED, ALL BEFORE ANY WRITE: a line that is not a live line of a live sheet of this commune
// (404); a line WITH children (409, domain.ErrDotChiGhiVaoLa); an amount keyed by a column of another
// sheet or a removed column (400, ErrCotKhongThuocBang); an amount in a `phan_tram` column (400,
// ErrCotKhongPhaiCotSo). None of the three column checks exists in the database (0008 says why), so
// this function is the only thing standing between a batch and a figure filed on no screen.
func (uc *NganSach) GhiDot(ctx context.Context, yc YeuCauGhiDot,
	nguoi audit.Actor) (domain.DotThuChi, error) {

	if yc.KhoanMucID == "" {
		return domain.DotThuChi{}, domain.ErrKhongThayKhoanMuc
	}
	if err := domain.KiemTraNgayDot(yc.Ngay); err != nil {
		return domain.DotThuChi{}, err
	}
	noiDung, err := domain.ChuanHoaNoiDungDot(yc.NoiDung)
	if err != nil {
		return domain.DotThuChi{}, err
	}
	donVi, err := domain.ChuanHoaDonViCaNhan(yc.DonViCaNhan)
	if err != nil {
		return domain.DotThuChi{}, err
	}
	soCT, err := domain.ChuanHoaSoChungTuDot(yc.SoChungTu)
	if err != nil {
		return domain.DotThuChi{}, err
	}
	coSo := false
	for _, g := range yc.GiaTri {
		if g == nil {
			continue
		}
		coSo = true
		if err := domain.KiemTraGiaTri(*g); err != nil {
			return domain.DotThuChi{}, err
		}
	}
	if !coSo {
		return domain.DotThuChi{}, domain.ErrDotKhongCoSoTienNao
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return domain.DotThuChi{}, err
	}

	id, err := uc.sinhID()
	if err != nil {
		return domain.DotThuChi{}, fmt.Errorf("ngan_sach: sinh mã đợt: %w", err)
	}
	moi := domain.DotThuChi{
		ID: id, KhoanMucID: yc.KhoanMucID, Ngay: yc.Ngay, NoiDung: noiDung,
		DonViCaNhan: donVi, SoChungTu: soCT,
		// THE STAFF BUSINESS CODE — audit.Actor.ID is Principal.Ma (http.nguoiThucHien). 0008 named the
		// column `nguoi_ghi_ma` so it cannot be read as licence to store the internal id.
		NguoiGhiMa: nguoi.ID,
		GiaTri:     map[string]domain.Dong{},
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bangID, err := uc.kho.BangIDCuaKhoanMuc(ctx, tx, yc.KhoanMucID)
		if err != nil {
			return err
		}
		bang, day, err := uc.khoaVaDocCay(ctx, tx, bangID)
		if err != nil {
			return err
		}
		k, thay := day.TheoID(yc.KhoanMucID)
		if !thay {
			return domain.ErrKhongThayKhoanMuc
		}
		if day.CoCon(k.ID) {
			return domain.ErrDotChiGhiVaoLa
		}
		// THE LIST'S CEILING, AT THE WRITE. The `⇄` read refuses past TranDotMotKhoanMuc rather than
		// truncate; a batch accepted beyond it would count in the figure and never be listable, so it
		// could never be reconciled or removed. Counted under the sheet lock khoaVaDocCay just took.
		daCo, err := uc.kho.DemDotSong(ctx, tx, k.ID)
		if err != nil {
			return err
		}
		if daCo >= domain.TranDotMotKhoanMuc {
			return fmt.Errorf("%w (tối đa %d đợt)", domain.ErrKhoanMucDaDuDot, domain.TranDotMotKhoanMuc)
		}

		// EVERY MENTIONED COLUMN MUST BE A LIVE NUMBER COLUMN OF THIS SHEET — checked for all of
		// them before the first insert, in the sheet's column order so the delta reads the same way
		// twice.
		soTien := make([]map[string]any, 0, len(yc.GiaTri))
		daGap := 0
		for _, cot := range day.Cot {
			g, duocNhac := yc.GiaTri[cot.ID]
			if !duocNhac {
				continue
			}
			daGap++
			if cot.Kieu != domain.CotSo {
				return domain.ErrCotKhongPhaiCotSo
			}
			if g != nil {
				moi.GiaTri[cot.ID] = *g
				soTien = append(soTien, map[string]any{"cot_id": cot.ID, "cot": cot.Ten, "gia_tri": int64(*g)})
			}
		}
		if daGap != len(yc.GiaTri) {
			return domain.ErrCotKhongThuocBang
		}

		if err := uc.kho.ChenDot(ctx, tx, moi); err != nil {
			return err
		}
		for _, cot := range day.Cot {
			g, co := moi.GiaTri[cot.ID]
			if !co {
				continue
			}
			if err := uc.kho.ChenSoTienDot(ctx, tx, moi.ID, cot.ID, g); err != nil {
				return err
			}
		}

		delta, err := json.Marshal(map[string]any{
			"dot_id":       moi.ID,
			"khoan_muc_id": k.ID,
			// Whether this batch moves the displayed figure right now (§4.2): only in `entries` mode.
			"cach_tinh_khoan_muc": string(k.CachTinh),
			"sau":                 tomTatDot(moi, soTien),
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: HanhViGhiDotThuChi, Subject: bang.Ma, Delta: delta,
		})
	})
	if err != nil {
		return domain.DotThuChi{}, bocNganSach(ctx, "ghi đợt", err)
	}
	return moi, nil
}

// GoDot soft deletes one batch, once, with a mandatory reason. Its amounts drop out of every sum with
// it: every sum joins `dot_thu_chi.deleted_at IS NULL` (store.tongDotCuaBang).
//
// A SECOND REMOVAL IS A 404, never a rewrite: the UPDATE carries `AND deleted_at IS NULL`, and 0008's
// trigger would refuse a change of who removed it or why anyway.
func (uc *NganSach) GoDot(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
	if id == "" {
		return domain.ErrKhongThayDot
	}
	lyDo, err := domain.ChuanHoaLyDoXoaNganSach(lyDoTho)
	if err != nil {
		return err
	}
	if err := coNguoiThucHien(nguoi); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.DotTheoIDTrongGiaoDich(ctx, tx, id)
		if err != nil {
			return err
		}
		bangID, err := uc.kho.BangIDCuaKhoanMuc(ctx, tx, truoc.KhoanMucID)
		if errors.Is(err, domain.ErrKhongThayKhoanMuc) {
			// A batch of a removed line is off every screen — the same answer as no batch at all.
			return domain.ErrKhongThayDot
		}
		if err != nil {
			return err
		}
		bang, day, err := uc.khoaVaDocCay(ctx, tx, bangID)
		if err != nil {
			return err
		}
		if _, thay := day.TheoID(truoc.KhoanMucID); !thay {
			return domain.ErrKhongThayDot
		}

		// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds.
		if err := uc.kho.XoaMemDot(ctx, tx, truoc.ID, nguoi.ID, lyDo); err != nil {
			return err
		}

		var soTien []map[string]any
		for _, cot := range day.Cot {
			if g, co := truoc.GiaTri[cot.ID]; co {
				soTien = append(soTien, map[string]any{"cot_id": cot.ID, "cot": cot.Ten, "gia_tri": int64(g)})
			}
		}
		delta, err := json.Marshal(map[string]any{
			"dot_id":       truoc.ID,
			"khoan_muc_id": truoc.KhoanMucID,
			"truoc":        tomTatDot(truoc, soTien),
			"ly_do":        lyDo,
			"xoa_mem":      true,
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: nguoi, Action: HanhViGoDotThuChi, Subject: bang.Ma, Delta: delta,
		})
	})
	if err != nil {
		return bocNganSach(ctx, "gỡ đợt", err)
	}
	return nil
}

// tomTatDot is the audit delta's view of one batch.
//
// `don_vi_ca_nhan` IS NOT HERE — ONLY WHETHER IT WAS STATED AND HOW LONG IT IS. The column may hold a
// citizen's name (migration 0008), and an audit entry is append-only and kept for years: copying the
// text would make the ledger a personal-data store nobody can erase (rule 6 forbidden #4, rule 3).
// The batch row itself is immutable and carries the text; `dot_id` points at it.
func tomTatDot(d domain.DotThuChi, soTien []map[string]any) map[string]any {
	return map[string]any{
		"ngay":                  d.Ngay.Format(time.DateOnly),
		"noi_dung":              d.NoiDung,
		"so_chung_tu":           d.SoChungTu,
		"co_don_vi_ca_nhan":     d.DonViCaNhan != "",
		"do_dai_don_vi_ca_nhan": utf8.RuneCountInString(d.DonViCaNhan),
		"so_tien":               soTien,
	}
}
