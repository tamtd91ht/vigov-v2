package app

// The use case behind PATCH /api/v1/task-statuses/{code} — a commune re-wording or re-ordering one
// of the seven task statuses (open question #21, DECIDED: ADR 0035 §C).
//
// WHAT IT DOES NOT DO, BY DECISION: add a status, remove one, or take one out of use. #21 closed the
// code list, and this group has no `Tắt`. There is no method here for any of the three.
//
// WHY A USE CASE FOR ONE UPSERT: rule 6, invariant 3 — the override and its audit entry share one
// transaction, and core/audit.Write only accepts a *store.ScopedTx. Opening it is this layer's job.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// KhoNhanTrangThaiNhiemVu is the store, declared at the point of use. EVERY METHOD TAKES THE
// TRANSACTION, so the override cannot be written in one transaction and the entry in another.
type KhoNhanTrangThaiNhiemVu interface {
	TheoMaDeSua(ctx context.Context, tx *store.ScopedTx, ma domain.TrangThaiNhiemVu) (
		domain.NhanTrangThaiNhiemVu, bool, error)
	Ghi(ctx context.Context, tx *store.ScopedTx, n domain.NhanTrangThaiNhiemVu) error
}

// NhanTrangThaiNhiemVu owns re-wording and re-ordering one commune's task statuses.
type NhanTrangThaiNhiemVu struct {
	db  *store.DB
	kho KhoNhanTrangThaiNhiemVu
}

func NewNhanTrangThaiNhiemVu(db *store.DB, kho KhoNhanTrangThaiNhiemVu) *NhanTrangThaiNhiemVu {
	return &NhanTrangThaiNhiemVu{db: db, kho: kho}
}

// HanhViSuaNhanTrangThaiNhiemVu is the business verb written into the trail. It names the catalogue,
// like `sua_loai_nhiem_vu`, so the trail can say WHICH list changed without a join.
const HanhViSuaNhanTrangThaiNhiemVu = "sua_nhan_trang_thai_nhiem_vu"

// YeuCauSuaNhanTrangThai is a PARTIAL edit: nil means "leave this alone". Pointers because a screen
// that edits only the label must not also move the status.
type YeuCauSuaNhanTrangThai struct {
	Nhan  *string
	ThuTu *int
}

// Sua applies the edit and returns the status as the commune now sees it.
//
// UNKNOWN CODE -> docstore.ErrDanhMucKhongTonTai (404), checked against the domain's closed set
// BEFORE any transaction: a code outside the seven never reaches SQL (fail closed; #21).
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — the property the route's idem.KhongCan declaration
// rests on. Sending the default wording for a status that has no row is also a no-op: there is
// nothing to override.
//
// A REORDER IS ONE CALL PER CODE. Swapping two positions is two requests; migration 0010 has no
// UNIQUE on `thu_tu`, so the intermediate tie is legal and the read breaks it by default order.
func (uc *NhanTrangThaiNhiemVu) Sua(ctx context.Context, maTho string, yc YeuCauSuaNhanTrangThai,
	nguoi audit.Actor) (domain.TrangThaiHienThi, error) {

	md, ok := domain.TimMacDinhTrangThai(maTho)
	if !ok {
		return domain.TrangThaiHienThi{}, docstore.ErrDanhMucKhongTonTai
	}
	var nhan string
	if yc.Nhan != nil {
		var err error
		if nhan, err = domain.ChuanHoaNhanTrangThai(*yc.Nhan); err != nil {
			return domain.TrangThaiHienThi{}, err
		}
	}
	if yc.ThuTu != nil {
		if err := domain.KiemTraThuTuTrangThai(*yc.ThuTu); err != nil {
			return domain.TrangThaiHienThi{}, err
		}
	}
	// `cap_nhat_boi` and the audit actor are the SAME business code. Empty refuses the write, never a
	// fallback: a setting nobody can be named for is a change nobody answers for (rule 6, inv. 8).
	if nguoi.ID == "" {
		return domain.TrangThaiHienThi{}, fmt.Errorf("nhan_trang_thai_nhiem_vu: thiếu người sửa")
	}

	var sau domain.TrangThaiHienThi
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		hienCo, coDong, err := uc.kho.TheoMaDeSua(ctx, tx, md.Ma)
		if err != nil {
			return err
		}
		var ghiDe *domain.NhanTrangThaiNhiemVu
		if coDong {
			ghiDe = &hienCo
		}
		truoc := domain.HienThiMot(md, ghiDe)

		moi := domain.NhanTrangThaiNhiemVu{Ma: md.Ma, Nhan: truoc.Nhan, ThuTu: truoc.ThuTu, CapNhatBoi: nguoi.ID}
		if yc.Nhan != nil {
			moi.Nhan = nhan
		}
		if yc.ThuTu != nil {
			moi.ThuTu = *yc.ThuTu
		}
		sau = domain.HienThiMot(md, &moi)

		if sau.Nhan == truoc.Nhan && sau.ThuTu == truoc.ThuTu {
			sau = truoc
			return nil
		}

		// THE FULL EFFECTIVE PAIR IS WRITTEN (both columns are NOT NULL), even when only one moved.
		if err := uc.kho.Ghi(ctx, tx, moi); err != nil {
			return err
		}

		// BEFORE AND AFTER, ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). Nothing here is personal
		// data (rule 3): a status label is how the authority names a step of its own work.
		truocDelta, sauDelta := map[string]any{}, map[string]any{}
		if truoc.Nhan != sau.Nhan {
			truocDelta["nhan"], sauDelta["nhan"] = truoc.Nhan, sau.Nhan
		}
		if truoc.ThuTu != sau.ThuTu {
			truocDelta["thu_tu"], sauDelta["thu_tu"] = truoc.ThuTu, sau.ThuTu
		}
		delta, err := json.Marshal(map[string]any{"truoc": truocDelta, "sau": sauDelta})
		if err != nil {
			return fmt.Errorf("nhan_trang_thai_nhiem_vu: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE UPSERT (rule 6, invariant 3). TenantID left unset: audit.Write
		// takes it from the transaction, which took it from the context — one source for the commune.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViSuaNhanTrangThaiNhiemVu,
			Subject: string(md.Ma), // the status CODE — the business key of the row
			Delta:   delta,
		})
	})
	if err != nil {
		// Only the commune and the operation: an error travels into centralised logging.
		return domain.TrangThaiHienThi{}, fmt.Errorf("nhan_trang_thai_nhiem_vu: sửa cho xã %s: %w",
			tenant.MustFrom(ctx), err)
	}
	return sau, nil
}
