package app

// The LIFECYCLE acts on the meeting-minutes register — user decisions 25/09/2026 (ledger
// service-petitions/bien-ban-hop-tang-du-lieu), stored by migration 0012:
//
//	SuaBienBan            edit a DRAFT; on SIGNED minutes, record the conclusion notice ONCE
//	XoaBienBan            soft delete a DRAFT with no live task on any of its conclusions
//	KyBienBan             du-thao -> da-ky, stamping the instant and the signer's staff code
//	SuaKetLuan            reword one conclusion of a draft that has no live task
//	XoaKetLuan            soft delete one such conclusion; its ordinal is never reissued
//	DanhDauKhongPhatSinh  set "không phát sinh nhiệm vụ" on a draft's conclusion with no live task
//	BoDanhDauKhongPhatSinh clear it, draft only
//
// # THE SHAPE EVERY ACT SHARES
//
//  1. What the client sent is checked BEFORE the transaction opens — a refused form holds no lock.
//  2. The MEETING row is locked `FOR UPDATE` first, always. Every act on one meeting — these seven,
//     appending a conclusion, and the split's in-transaction check — serialises on that one row, so a
//     decision taken here ("no live task", "still a draft") is still true when the UPDATE lands.
//  3. A request that changes nothing WRITES NOTHING — no UPDATE, no audit entry. That is what makes
//     the routes' `idem.KhongCan` declarations a property rather than a hope.
//  4. The business write and its audit entry share the transaction (rule 6, invariant 3), the actor
//     is the STAFF BUSINESS CODE (rule 6, invariant 8), and the subject is the composed business
//     reference chuDeBienBan / chuDeKetLuan.
//  5. The delta never carries the minutes' body or a conclusion's text — LENGTHS only, the existing
//     convention (TaoBienBan): the ledger is append-only and a commune's minutes quote cases.
//
// # WHAT IS APP-ENFORCED HERE AND NOWHERE ELSE (migration 0012's header says why not a trigger)
//
//	a conclusion with a live task is locked even in draft          (decision 3)
//	the mark may be set only while the conclusion has no live task (decision 4)
//	supplementary minutes point at SIGNED minutes                  (decision 1) — in TaoBienBan

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The business verbs written into the trail, Vietnamese snake_case like every other action here.
const (
	HanhViSuaBienBanHop        = "sua_bien_ban_hop"
	HanhViGhiThongBaoKetLuan   = "ghi_thong_bao_ket_luan"
	HanhViKyBienBanHop         = "ky_bien_ban_hop"
	HanhViXoaBienBanHop        = "xoa_bien_ban_hop"
	HanhViSuaKetLuan           = "sua_ket_luan_hop"
	HanhViXoaKetLuan           = "xoa_ket_luan_hop"
	HanhViDanhDauKhongPhatSinh = "danh_dau_khong_phat_sinh"
	HanhViBoDauKhongPhatSinh   = "bo_dau_khong_phat_sinh"
)

// ThongBaoKetLuan is the conclusion notice (Thông báo kết luận), transcribed: number and day together.
type ThongBaoKetLuan struct {
	SoKyHieu string
	Ngay     time.Time
}

// --- 1. sửa biên bản (and the notice after signing) ------------------------------------------------

// YeuCauSuaBienBan is PATCH /api/v1/meetings/{id}. EVERY FIELD IS A POINTER: nil = not mentioned.
// For the optional texts an empty string CLEARS the value; the title and the day are required and
// refuse an empty one.
type YeuCauSuaBienBan struct {
	TenCuocHop *string
	NgayHop    *time.Time
	SoHieu     *string
	DiaDiem    *string
	ChuTriMa   *string
	ThuKyMa    *string
	NoiDung    *string
	ThanhPhan  *[]string

	// ThongBao sets the notice. On a draft it may be typed or replaced; on SIGNED minutes it is the
	// only thing that may still be written, and only while none is recorded. There is no way to CLEAR
	// a notice through this request — see the report of 25/09/2026.
	ThongBao *ThongBaoKetLuan
}

// SuaBienBan edits the minutes. Permission: `task.create` (route).
//
// # ON SIGNED MINUTES
//
// A field that CHANGES anything but the notice is refused (ErrBienBanDaKy) — a correction is
// supplementary minutes. A field sent with the value already stored is not a change and is ignored,
// so a client that sends the whole form back with only the notice filled in is not refused for
// repeating what it read. The notice is written once; the same notice again is a no-op, a different
// one is ErrDaCoThongBao.
//
// THE RETURNED RECORD carries no conclusions — the handler re-reads the detail for its response.
func (uc *GhiBienBanHop) SuaBienBan(ctx context.Context, id string, yc YeuCauSuaBienBan,
	nguoi audit.Actor) (domain.BienBanHop, error) {

	if err := chuanHoaSuaBienBan(&yc); err != nil {
		return domain.BienBanHop{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.BienBanHop{}, err
	}

	var sau domain.BienBanHop
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.BienBanDayDuDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		sau = apDungSuaBienBan(truoc, yc)
		doiNoiDung := khacNgoaiThongBao(truoc, sau)
		doiThongBao := truoc.TbSoKyHieu != sau.TbSoKyHieu || !truoc.TbNgay.Equal(sau.TbNgay)

		if truoc.TrangThai == domain.TrangThaiBienBanDaKy {
			if doiNoiDung {
				return domain.ErrBienBanDaKy
			}
			if !doiThongBao {
				return nil // nothing moved: no UPDATE, no entry
			}
			if truoc.TbSoKyHieu != "" || !truoc.TbNgay.IsZero() {
				return domain.ErrDaCoThongBao
			}
			if err := uc.kho.GhiThongBao(ctx, tx, truoc.ID, sau.TbSoKyHieu, sau.TbNgay); err != nil {
				return err
			}
			return ghiVetBienBan(ctx, tx, nguoi, HanhViGhiThongBaoKetLuan, sau, map[string]any{
				"truoc": map[string]any{"tb_so_ky_hieu": "", "tb_ngay": ""},
				"sau":   thongBaoVet(sau),
			})
		}

		if !doiNoiDung && !doiThongBao {
			return nil // nothing moved: no UPDATE, no entry
		}
		if err := uc.kho.SuaBienBan(ctx, tx, sau); err != nil {
			return err
		}
		return ghiVetBienBan(ctx, tx, nguoi, HanhViSuaBienBanHop, sau, map[string]any{
			"truoc": banChupBienBan(truoc),
			"sau":   banChupBienBan(sau),
		})
	})
	if err != nil {
		return domain.BienBanHop{}, bocBienBan(ctx, "sửa biên bản họp", err)
	}
	return sau, nil
}

// chuanHoaSuaBienBan trims and bounds what was sent; nil pointers are left alone.
func chuanHoaSuaBienBan(yc *YeuCauSuaBienBan) error {
	if yc.TenCuocHop != nil {
		s, err := domain.KiemTenCuocHop(*yc.TenCuocHop)
		if err != nil {
			return err
		}
		yc.TenCuocHop = &s
	}
	if yc.NgayHop != nil {
		d, err := domain.ChuanHoaNgayHop(*yc.NgayHop)
		if err != nil {
			return err
		}
		yc.NgayHop = &d
	}
	for _, ca := range []struct {
		truong **string
		tran   int
		loi    error
	}{
		{&yc.SoHieu, domain.SoHieuBienBanToiDa, domain.ErrSoHieuBienBanQuaDai},
		{&yc.DiaDiem, domain.DiaDiemBienBanToiDa, domain.ErrDiaDiemBienBanQuaDai},
		{&yc.ChuTriMa, domain.ChuTriMaToiDa, domain.ErrChuTriQuaDai},
		{&yc.ThuKyMa, domain.ChuTriMaToiDa, domain.ErrThuKyQuaDai},
		{&yc.NoiDung, domain.NoiDungBienBanToiDa, domain.ErrNoiDungBienBanQuaDai},
	} {
		if *ca.truong == nil {
			continue
		}
		s, err := domain.KiemVanBanTuyChon(**ca.truong, ca.tran, ca.loi)
		if err != nil {
			return err
		}
		*ca.truong = &s
	}
	if yc.ThanhPhan != nil {
		ds, err := domain.KiemThanhPhan(*yc.ThanhPhan)
		if err != nil {
			return err
		}
		yc.ThanhPhan = &ds
	}
	if yc.ThongBao != nil {
		so, ngay, err := domain.KiemThongBaoKetLuan(yc.ThongBao.SoKyHieu, yc.ThongBao.Ngay)
		if err != nil {
			return err
		}
		yc.ThongBao = &ThongBaoKetLuan{SoKyHieu: so, Ngay: ngay}
	}
	return nil
}

// apDungSuaBienBan is the row as it would be after the request.
func apDungSuaBienBan(b domain.BienBanHop, yc YeuCauSuaBienBan) domain.BienBanHop {
	datNeuCo := func(dich *string, gt *string) {
		if gt != nil {
			*dich = *gt
		}
	}
	datNeuCo(&b.TenCuocHop, yc.TenCuocHop)
	datNeuCo(&b.SoHieu, yc.SoHieu)
	datNeuCo(&b.DiaDiem, yc.DiaDiem)
	datNeuCo(&b.ChuTriMa, yc.ChuTriMa)
	datNeuCo(&b.ThuKyMa, yc.ThuKyMa)
	datNeuCo(&b.NoiDung, yc.NoiDung)
	if yc.NgayHop != nil {
		b.NgayHop = *yc.NgayHop
	}
	if yc.ThanhPhan != nil {
		b.ThanhPhan = *yc.ThanhPhan
	}
	if yc.ThongBao != nil {
		b.TbSoKyHieu, b.TbNgay = yc.ThongBao.SoKyHieu, yc.ThongBao.Ngay
	}
	return b
}

// khacNgoaiThongBao reports whether anything but the notice differs. The day is compared as a
// CALENDAR DAY (both sides are UTC midnight by then — the column is DATE, the request normalised).
func khacNgoaiThongBao(a, b domain.BienBanHop) bool {
	return a.TenCuocHop != b.TenCuocHop || !cungNgay(a.NgayHop, b.NgayHop) ||
		a.SoHieu != b.SoHieu || a.DiaDiem != b.DiaDiem || a.ChuTriMa != b.ChuTriMa ||
		a.ThuKyMa != b.ThuKyMa || a.NoiDung != b.NoiDung || !slices.Equal(a.ThanhPhan, b.ThanhPhan)
}

func cungNgay(a, b time.Time) bool {
	return a.Format("2006-01-02") == b.Format("2006-01-02")
}

// banChupBienBan is what the trail keeps of the minutes: references and staff codes as typed, the
// body and the attendee list as a LENGTH and a COUNT (rule 3; the convention TaoBienBan set).
func banChupBienBan(b domain.BienBanHop) map[string]any {
	m := map[string]any{
		"ten_cuoc_hop":    b.TenCuocHop,
		"ngay_hop":        b.NgayHop.Format("2006-01-02"),
		"so_hieu":         b.SoHieu,
		"dia_diem":        b.DiaDiem,
		"chu_tri_ma":      b.ChuTriMa,
		"thu_ky_ma":       b.ThuKyMa,
		"so_thanh_phan":   len(b.ThanhPhan),
		"do_dai_noi_dung": len([]rune(b.NoiDung)),
	}
	for k, v := range thongBaoVet(b) {
		m[k] = v
	}
	return m
}

func thongBaoVet(b domain.BienBanHop) map[string]any {
	ngay := ""
	if !b.TbNgay.IsZero() {
		ngay = b.TbNgay.Format("2006-01-02")
	}
	return map[string]any{"tb_so_ky_hieu": b.TbSoKyHieu, "tb_ngay": ngay}
}

// --- 2. xoá biên bản ----------------------------------------------------------------------------------

// XoaBienBan soft deletes DRAFT minutes. Permission: `task.create` (route).
//
// REFUSED WHILE ANY CONCLUSION HAS A LIVE TASK, with the count: the tasks point at the conclusions,
// and minutes removed from under them leave every one of those back-links pointing at nothing.
// SIGNED MINUTES ARE NEVER REMOVED (decision 1; the trigger refuses too).
func (uc *GhiBienBanHop) XoaBienBan(ctx context.Context, id, lyDoTho string, nguoi audit.Actor) error {
	lyDo, err := domain.KiemLyDoXoaBienBan(lyDoTho)
	if err != nil {
		return err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return err
	}
	bayGio := uc.nayHoac()

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bb, err := uc.kho.BienBanTheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		if bb.TrangThai == domain.TrangThaiBienBanDaKy {
			return domain.ErrBienBanDaKy
		}
		n, err := uc.kho.SoNhiemVuSongCuaBienBan(ctx, tx, bb.ID)
		if err != nil {
			return err
		}
		if n > 0 {
			return domain.LoiBienBanConNhiemVu(n)
		}
		// `deleted_by` IS THE STAFF BUSINESS CODE (rule 6, invariant 8).
		if err := uc.kho.XoaMemBienBan(ctx, tx, bb.ID, nguoi.ID, lyDo, bayGio); err != nil {
			return err
		}
		// THE REASON'S TEXT IS NOT IN THE DELTA, ITS LENGTH IS — it lives in `delete_reason` on a row
		// the archival trigger protects; a copy in the ledger would be a second permanent store.
		return ghiVetBienBan(ctx, tx, nguoi, HanhViXoaBienBanHop, bb, map[string]any{
			"truoc":        map[string]any{"trang_thai": bb.TrangThai, "da_xoa": false},
			"sau":          map[string]any{"trang_thai": bb.TrangThai, "da_xoa": true},
			"do_dai_ly_do": len([]rune(lyDo)),
			"xoa_luc":      lucRaVet(bayGio),
		})
	})
	if err != nil {
		return bocBienBan(ctx, "xoá biên bản họp", err)
	}
	return nil
}

// --- 3. ký biên bản ------------------------------------------------------------------------------------

// YeuCauKyBienBan is POST /api/v1/meetings/{id}/signature. The notice is OPTIONAL: it is routinely
// issued days after the signing, and may then be recorded once through SuaBienBan.
type YeuCauKyBienBan struct {
	ThongBao *ThongBaoKetLuan
}

// KyBienBan signs the minutes. Permission: `task.approve` (route) — "Duyệt hoàn thành", the key of
// the act that makes a record final; `task.create` alone is NOT enough.
//
// MINUTES WITH NO CONCLUSIONS MAY BE SIGNED (spec §7.3 — a meeting that concluded nothing is still a
// meeting). SIGNING TWICE IS REFUSED (ErrBienBanDaKy): the second request never records a second
// signer or instant.
//
// THE SIGNER IS THE ACTING PRINCIPAL'S STAFF CODE, never a value from the request (rule 6, invariant
// 8). The instant is this use case's clock.
func (uc *GhiBienBanHop) KyBienBan(ctx context.Context, id string, yc YeuCauKyBienBan,
	nguoi audit.Actor) (domain.BienBanHop, error) {

	var tbSo string
	var tbNgay time.Time
	if yc.ThongBao != nil {
		so, ngay, err := domain.KiemThongBaoKetLuan(yc.ThongBao.SoKyHieu, yc.ThongBao.Ngay)
		if err != nil {
			return domain.BienBanHop{}, err
		}
		tbSo, tbNgay = so, ngay
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.BienBanHop{}, err
	}
	bayGio := uc.nayHoac()

	var sau domain.BienBanHop
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bb, err := uc.kho.BienBanTheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		if bb.TrangThai == domain.TrangThaiBienBanDaKy {
			return domain.ErrBienBanDaKy
		}
		if err := uc.kho.KyBienBan(ctx, tx, bb.ID, bayGio, nguoi.ID, tbSo, tbNgay); err != nil {
			return err
		}
		sau = bb
		sau.TrangThai, sau.KyLuc, sau.KyBoiMa = domain.TrangThaiBienBanDaKy, bayGio, nguoi.ID
		if tbSo != "" {
			sau.TbSoKyHieu, sau.TbNgay = tbSo, tbNgay
		}
		daKy := map[string]any{
			"trang_thai": sau.TrangThai,
			"ky_luc":     lucRaVet(bayGio),
			"ky_boi_ma":  nguoi.ID,
		}
		for k, v := range thongBaoVet(sau) {
			daKy[k] = v
		}
		return ghiVetBienBan(ctx, tx, nguoi, HanhViKyBienBanHop, sau, map[string]any{
			"truoc": map[string]any{"trang_thai": bb.TrangThai},
			"sau":   daKy,
		})
	})
	if err != nil {
		return domain.BienBanHop{}, bocBienBan(ctx, "ký biên bản họp", err)
	}
	return sau, nil
}

// --- 4. one conclusion ---------------------------------------------------------------------------------

// YeuCauSuaKetLuan is PATCH /api/v1/meetings/{id}/conclusions/{stt}.
type YeuCauSuaKetLuan struct{ NoiDung string }

// SuaKetLuan rewords one conclusion. Permission: `task.create` (route).
//
// DRAFT ONLY, and REFUSED WHILE THE CONCLUSION HAS A LIVE TASK (decision 3) — even in draft: those
// tasks quote this sentence. The no-task mark does NOT lock the text (caller's decision 25/09/2026).
// The same text again is a no-op.
func (uc *GhiBienBanHop) SuaKetLuan(ctx context.Context, bienBanID string, thuTu int,
	yc YeuCauSuaKetLuan, nguoi audit.Actor) (domain.KetLuanHop, error) {

	noiDung, err := domain.KiemNoiDungKetLuan(yc.NoiDung)
	if err != nil {
		return domain.KetLuanHop{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.KetLuanHop{}, err
	}

	var sau domain.KetLuanHop
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bb, k, err := uc.khoaKetLuan(ctx, tx, bienBanID, thuTu)
		if err != nil {
			return err
		}
		if bb.TrangThai == domain.TrangThaiBienBanDaKy {
			return domain.ErrBienBanDaKy
		}
		sau = k
		if k.NoiDung == noiDung {
			return nil // nothing moved
		}
		if err := uc.khongCoNhiemVuSong(ctx, tx, k.ID); err != nil {
			return err
		}
		if err := uc.kho.SuaKetLuan(ctx, tx, k.ID, noiDung); err != nil {
			return err
		}
		sau.NoiDung = noiDung
		return ghiVetKetLuan(ctx, tx, nguoi, HanhViSuaKetLuan, bb, k.ThuTu, map[string]any{
			"truoc": map[string]any{"do_dai_noi_dung": len([]rune(k.NoiDung))},
			"sau":   map[string]any{"do_dai_noi_dung": len([]rune(noiDung))},
		})
	})
	if err != nil {
		return domain.KetLuanHop{}, bocBienBan(ctx, "sửa kết luận họp", err)
	}
	return sau, nil
}

// XoaKetLuan soft deletes one conclusion. Permission: `task.create` (route). Draft only, refused while
// a live task points at it, reason mandatory. ITS ORDINAL IS NEVER REISSUED — ThuTuLonNhat counts
// deleted rows (rule 7, invariant 3).
func (uc *GhiBienBanHop) XoaKetLuan(ctx context.Context, bienBanID string, thuTu int, lyDoTho string,
	nguoi audit.Actor) error {

	lyDo, err := domain.KiemLyDoXoaBienBan(lyDoTho)
	if err != nil {
		return err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return err
	}
	bayGio := uc.nayHoac()

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bb, k, err := uc.khoaKetLuan(ctx, tx, bienBanID, thuTu)
		if err != nil {
			return err
		}
		if bb.TrangThai == domain.TrangThaiBienBanDaKy {
			return domain.ErrBienBanDaKy
		}
		if err := uc.khongCoNhiemVuSong(ctx, tx, k.ID); err != nil {
			return err
		}
		if err := uc.kho.XoaMemKetLuan(ctx, tx, k.ID, nguoi.ID, lyDo, bayGio); err != nil {
			return err
		}
		return ghiVetKetLuan(ctx, tx, nguoi, HanhViXoaKetLuan, bb, k.ThuTu, map[string]any{
			"truoc":        map[string]any{"da_xoa": false, "do_dai_noi_dung": len([]rune(k.NoiDung))},
			"sau":          map[string]any{"da_xoa": true},
			"do_dai_ly_do": len([]rune(lyDo)),
			"xoa_luc":      lucRaVet(bayGio),
		})
	})
	if err != nil {
		return bocBienBan(ctx, "xoá kết luận họp", err)
	}
	return nil
}

// DanhDauKhongPhatSinh sets "không phát sinh nhiệm vụ" (decision 4). Permission: `task.create`.
//
// ALREADY SET IS A NO-OP (PUT is idempotent): nothing is written, whatever the meeting's state —
// the mark is what the caller asked for. Otherwise: draft only, and only while no live task points
// at the conclusion (a conclusion that produced work did not "produce no work").
func (uc *GhiBienBanHop) DanhDauKhongPhatSinh(ctx context.Context, bienBanID string, thuTu int,
	nguoi audit.Actor) (domain.KetLuanHop, error) {

	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.KetLuanHop{}, err
	}
	bayGio := uc.nayHoac()

	var sau domain.KetLuanHop
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bb, k, err := uc.khoaKetLuan(ctx, tx, bienBanID, thuTu)
		if err != nil {
			return err
		}
		sau = k
		if k.KhongPhatSinh {
			return nil
		}
		if bb.TrangThai == domain.TrangThaiBienBanDaKy {
			return domain.ErrBienBanDaKy
		}
		if err := uc.khongCoNhiemVuSong(ctx, tx, k.ID); err != nil {
			return err
		}
		if err := uc.kho.DatKhongPhatSinh(ctx, tx, k.ID, bayGio, nguoi.ID); err != nil {
			return err
		}
		sau.KhongPhatSinh = true
		return ghiVetKetLuan(ctx, tx, nguoi, HanhViDanhDauKhongPhatSinh, bb, k.ThuTu, map[string]any{
			"truoc": map[string]any{"khong_phat_sinh": false},
			"sau":   map[string]any{"khong_phat_sinh": true, "luc": lucRaVet(bayGio)},
		})
	})
	if err != nil {
		return domain.KetLuanHop{}, bocBienBan(ctx, "đánh dấu không phát sinh nhiệm vụ", err)
	}
	return sau, nil
}

// BoDanhDauKhongPhatSinh clears the mark. Permission: `task.create`. Not set is a no-op; otherwise
// draft only (after signing the mark is part of the signed record — the trigger freezes it too).
func (uc *GhiBienBanHop) BoDanhDauKhongPhatSinh(ctx context.Context, bienBanID string, thuTu int,
	nguoi audit.Actor) error {

	if err := coCanBoThucHien(nguoi); err != nil {
		return err
	}
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		bb, k, err := uc.khoaKetLuan(ctx, tx, bienBanID, thuTu)
		if err != nil {
			return err
		}
		if !k.KhongPhatSinh {
			return nil
		}
		if bb.TrangThai == domain.TrangThaiBienBanDaKy {
			return domain.ErrBienBanDaKy
		}
		if err := uc.kho.BoKhongPhatSinh(ctx, tx, k.ID); err != nil {
			return err
		}
		return ghiVetKetLuan(ctx, tx, nguoi, HanhViBoDauKhongPhatSinh, bb, k.ThuTu, map[string]any{
			"truoc": map[string]any{"khong_phat_sinh": true},
			"sau":   map[string]any{"khong_phat_sinh": false},
		})
	})
	if err != nil {
		return bocBienBan(ctx, "bỏ dấu không phát sinh nhiệm vụ", err)
	}
	return nil
}

// --- shared ---------------------------------------------------------------------------------------------

// khoaKetLuan locks the meeting, THEN the conclusion — the one lock order of this register (see the
// header). An unknown / removed meeting answers ErrBienBanKhongTonTai, an unknown / removed
// conclusion ErrKetLuanKhongTonTai; the handler answers 404 to both.
func (uc *GhiBienBanHop) khoaKetLuan(ctx context.Context, tx *store.ScopedTx, bienBanID string,
	thuTu int) (domain.BienBanHop, domain.KetLuanHop, error) {

	bb, err := uc.kho.BienBanTheoIDDeSua(ctx, tx, bienBanID)
	if err != nil {
		return domain.BienBanHop{}, domain.KetLuanHop{}, err
	}
	k, err := uc.kho.KetLuanTheoThuTuDeSua(ctx, tx, bienBanID, thuTu)
	if err != nil {
		return domain.BienBanHop{}, domain.KetLuanHop{}, err
	}
	return bb, k, nil
}

// khongCoNhiemVuSong is decision 3 and decision 4's precondition: no LIVE task may point at the
// conclusion.
func (uc *GhiBienBanHop) khongCoNhiemVuSong(ctx context.Context, tx *store.ScopedTx, ketLuanID string) error {
	n, err := uc.kho.SoNhiemVuSongCuaKetLuan(ctx, tx, ketLuanID)
	if err != nil {
		return err
	}
	if n > 0 {
		return domain.ErrKetLuanDaCoNhiemVu
	}
	return nil
}

func ghiVetBienBan(ctx context.Context, tx *store.ScopedTx, nguoi audit.Actor, hanhVi string,
	b domain.BienBanHop, delta map[string]any) error {
	return ghiVet(ctx, tx, nguoi, hanhVi, chuDeBienBan(b), delta)
}

func ghiVetKetLuan(ctx context.Context, tx *store.ScopedTx, nguoi audit.Actor, hanhVi string,
	b domain.BienBanHop, thuTu int, delta map[string]any) error {
	return ghiVet(ctx, tx, nguoi, hanhVi, chuDeKetLuan(b, thuTu), delta)
}

func ghiVet(ctx context.Context, tx *store.ScopedTx, nguoi audit.Actor, hanhVi, chuDe string,
	delta map[string]any) error {
	b, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("bien_ban_hop: mã hoá delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: nguoi, Action: hanhVi, Subject: chuDe, Delta: b})
}
