package domain

// The rules of the two registers that hold regardless of storage. Everything here runs with no
// database and no clock of its own: a validation whose outcome depends on when the suite runs is a
// validation nobody trusts, so every function under test takes its instant as a parameter.

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var (
	bayGio  = time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC)
	homNay  = time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	homQua  = time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	homSau  = time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	hanMau  = time.Date(2026, 9, 29, 3, 30, 0, 0, time.UTC)
	quaHanR = time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
)

// --- overdue is DERIVED --------------------------------------------------------------------------

func TestQuaHan_SuyRaTuHanChuKhongPhaiMotCot(t *testing.T) {
	// RULE 10, INVARIANT 3. There is no column and no field behind this — the whole computation is
	// one comparison against the stored deadline, so it cannot go stale the way a flag set by a
	// nightly job does when the job is late, the clock skews or a holiday is added.
	v := VanBanDen{HanXuLyXong: hanMau, TrangThai: VanBanDangXuLy}

	if v.QuaHan(bayGio) {
		t.Error("còn hạn mà báo quá hạn")
	}
	if !v.QuaHan(quaHanR) {
		t.Error("đã quá hạn mà không báo")
	}
	// EXACTLY AT THE DEADLINE IS NOT LATE. "Trong 40 giờ làm việc" means up to and including that
	// instant; an off-by-one here turns a commitment met into a figure reported as missed.
	if v.QuaHan(hanMau) {
		t.Error("đúng thời điểm hạn mà đã báo quá hạn")
	}
}

func TestQuaHan_VanBanDaKetThucKhongBaoGioQuaHan(t *testing.T) {
	// A document the register has finished with is not "still late": the commune did the work, and
	// whether it did so in time is a different question — one this register cannot answer yet,
	// because it does not store the instant the work finished. THAT GAP IS STATED rather than
	// papered over with a comparison against `now`, which would make every settled document drift
	// into "overdue" as time passed.
	for _, tt := range []TrangThaiVanBanDen{VanBanDaGiaiQuyet, VanBanChuyenCapTren, VanBanLuuKhongThuLy} {
		v := VanBanDen{HanXuLyXong: hanMau, TrangThai: tt}
		if v.QuaHan(quaHanR) {
			t.Errorf("trạng thái %q đã kết thúc mà vẫn bị tính quá hạn", tt)
		}
	}
}

func TestDaKetThuc_BaMaKetThucVaBaMaConChay(t *testing.T) {
	ketThuc := map[TrangThaiVanBanDen]bool{
		VanBanMoiVaoSo: false, VanBanDaPhanCong: false, VanBanDangXuLy: false,
		VanBanDaGiaiQuyet: true, VanBanChuyenCapTren: true, VanBanLuuKhongThuLy: true,
	}
	for tt, muon := range ketThuc {
		if tt.DaKetThuc() != muon {
			t.Errorf("%q.DaKetThuc() = %v, muốn %v", tt, tt.DaKetThuc(), muon)
		}
	}
}

// --- routing only ever moves the state forward ----------------------------------------------------

func TestTrangThaiSauKhiChuyen_ChiTienKhongLui(t *testing.T) {
	// A brand-new entry becomes `da-phan-cong` — that IS what assigning it to a department means.
	if got := TrangThaiSauKhiChuyen(VanBanMoiVaoSo); got != VanBanDaPhanCong {
		t.Errorf("mới vào sổ -> %q, muốn %q", got, VanBanDaPhanCong)
	}
	// A document already being worked on KEEPS ITS STATE. Re-routing a `dang-xu-ly` document back to
	// `da-phan-cong` would undo a fact somebody recorded, and the timeline would then show the work
	// going backwards.
	for _, tt := range []TrangThaiVanBanDen{VanBanDaPhanCong, VanBanDangXuLy} {
		if got := TrangThaiSauKhiChuyen(tt); got != tt {
			t.Errorf("%q -> %q, muốn giữ nguyên", tt, got)
		}
	}
}

func TestChoChuyen_TuChoiVanBanDaKetThuc(t *testing.T) {
	for _, tt := range []TrangThaiVanBanDen{VanBanMoiVaoSo, VanBanDaPhanCong, VanBanDangXuLy} {
		if err := (VanBanDen{TrangThai: tt}).ChoChuyen(); err != nil {
			t.Errorf("%q: ChoChuyen = %v, muốn nil", tt, err)
		}
	}
	for _, tt := range []TrangThaiVanBanDen{VanBanDaGiaiQuyet, VanBanChuyenCapTren, VanBanLuuKhongThuLy} {
		err := (VanBanDen{TrangThai: tt}).ChoChuyen()
		if !errors.Is(err, ErrVanBanDaKetThuc) {
			t.Errorf("%q: ChoChuyen = %v, muốn ErrVanBanDaKetThuc", tt, err)
		}
		// THE SENTENCE NAMES THE STATE, because a clerk reading "không chuyển tiếp được" with no
		// reason retries the same button.
		if !strings.Contains(err.Error(), string(tt)) {
			t.Errorf("thông báo không nói trạng thái hiện tại: %v", err)
		}
	}
}

// --- the dates -------------------------------------------------------------------------------------

func TestKiemNgayDen(t *testing.T) {
	for ten, tc := range map[string]struct {
		ngay time.Time
		muon error
	}{
		"hôm nay":                   {homNay, nil},
		"hôm qua":                   {homQua, nil},
		"để trống":                  {time.Time{}, ErrThieuNgayDen},
		"ngày mai":                  {homSau, ErrNgayDenTuongLai},
		"quá xa trong quá khứ":      {bayGio.Add(-NgayDenSomNhat - 24*time.Hour), ErrNgayDenQuaXa},
		"vừa trong khoảng cho phép": {bayGio.Add(-NgayDenSomNhat + 24*time.Hour), nil},
	} {
		t.Run(ten, func(t *testing.T) {
			if err := KiemNgayDen(tc.ngay, bayGio); !errors.Is(err, tc.muon) {
				t.Fatalf("KiemNgayDen = %v, muốn %v", err, tc.muon)
			}
		})
	}
}

func TestKiemNgayDen_SoSanhTheoNGAYChuKhongTheoGio(t *testing.T) {
	// `ngay_den` IS A DATE and arrives as midnight UTC. Compared as an instant, a document booked at
	// 08:30 this morning would be "in the future" — which would refuse the single most ordinary
	// action in the whole register, and only in the morning.
	if err := KiemNgayDen(homNay, bayGio); err != nil {
		t.Fatalf("vào sổ sáng nay cho ngày hôm nay bị từ chối: %v", err)
	}
}

func TestKiemNgayVanBanDen_KhongTheKySauKhiDen(t *testing.T) {
	// A document cannot arrive before it was signed. The reverse — signed long before it arrived — is
	// ordinary post and is accepted.
	som := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if err := KiemNgayVanBanDen(som, homNay); err != nil {
		t.Errorf("ký trước, đến sau: %v, muốn nil", err)
	}
	if err := KiemNgayVanBanDen(time.Time{}, homNay); err != nil {
		t.Errorf("để trống là hợp lệ (trường tuỳ chọn): %v", err)
	}
	if err := KiemNgayVanBanDen(homSau, homNay); !errors.Is(err, ErrNgayVanBanSauDen) {
		t.Errorf("ký sau ngày đến = %v, muốn ErrNgayVanBanSauDen", err)
	}
	// SAME DAY IS FINE: a document signed and hand-delivered this morning.
	if err := KiemNgayVanBanDen(homNay, homNay); err != nil {
		t.Errorf("ký và đến cùng ngày: %v, muốn nil", err)
	}
}

func TestKiemNgayVanBanDi_BatBuocVaKhongOTuongLai(t *testing.T) {
	if err := KiemNgayVanBanDi(time.Time{}, bayGio); !errors.Is(err, ErrThieuNgayVanBan) {
		t.Errorf("thiếu ngày = %v, muốn ErrThieuNgayVanBan", err)
	}
	if err := KiemNgayVanBanDi(homSau, bayGio); !errors.Is(err, ErrNgayVanBanTuongLai) {
		t.Errorf("ngày tương lai = %v, muốn ErrNgayVanBanTuongLai", err)
	}
	if err := KiemNgayVanBanDi(homNay, bayGio); err != nil {
		t.Errorf("hôm nay = %v, muốn nil", err)
	}
}

// --- the free-text fields --------------------------------------------------------------------------

func TestChuanHoaChuoi_CatKhoangTrangVaTuChoiRong(t *testing.T) {
	s, err := ChuanHoaChuoi("  Về việc rà soát hộ nghèo  ", TranTrichYeu, ErrThieuTrichYeu)
	if err != nil || s != "Về việc rà soát hộ nghèo" {
		t.Fatalf("ChuanHoaChuoi = %q, %v", s, err)
	}
	if _, err := ChuanHoaChuoi("   ", TranTrichYeu, ErrThieuTrichYeu); !errors.Is(err, ErrThieuTrichYeu) {
		t.Fatalf("chuỗi toàn khoảng trắng = %v, muốn ErrThieuTrichYeu", err)
	}
}

func TestChuanHoaChuoi_DemKY_TUKhongDemBYTE(t *testing.T) {
	// A VIETNAMESE SUMMARY IS TWO TO THREE BYTES PER CHARACTER. Counted in bytes, the ceiling would
	// refuse a legitimate sentence at roughly a third of its apparent length — and the clerk would
	// see "quá dài" on a box that is visibly not full.
	dai := strings.Repeat("ế", TranTrichYeu) // 3 bytes each, exactly at the ceiling in runes
	if _, err := ChuanHoaChuoi(dai, TranTrichYeu, ErrThieuTrichYeu); err != nil {
		t.Fatalf("đúng trần theo ký tự mà bị từ chối: %v", err)
	}
	if _, err := ChuanHoaChuoi(dai+"ế", TranTrichYeu, ErrThieuTrichYeu); !errors.Is(err, ErrChuoiQuaDai) {
		t.Fatalf("vượt trần = %v, muốn ErrChuoiQuaDai", err)
	}
}

func TestChuanHoaTuyChon_RongLaMotCauTraLoi(t *testing.T) {
	// The empty value of an optional field is MEANINGFUL — "this document carries no number of its
	// own" is a statement — and it must survive as "" so the store can write NULL. One absent value
	// with two spellings in one column makes `WHERE … IS NULL` silently miss half the rows.
	s, err := ChuanHoaTuyChon("   ", TranSoKyHieu)
	if err != nil || s != "" {
		t.Fatalf("ChuanHoaTuyChon = %q, %v, muốn \"\", nil", s, err)
	}
}

func TestKiemDoKhan(t *testing.T) {
	// The four values are the statutory list of Nghị định 30/2020/NĐ-CP, and the EMPTY one is a real
	// answer: "the commune did not record an urgency" is not the same statement as "Thường".
	for _, d := range []DoKhan{DoKhanKhongGhi, DoKhanThuong, DoKhanKhan, DoKhanThuongKhan, DoKhanHoaToc} {
		if err := KiemDoKhan(d); err != nil {
			t.Errorf("%q = %v, muốn nil", d, err)
		}
	}
	if err := KiemDoKhan("rat-khan"); !errors.Is(err, ErrDoKhanKhongHopLe) {
		t.Errorf("giá trị lạ = %v, muốn ErrDoKhanKhongHopLe", err)
	}
}

// --- the business code the trail is filed under ---------------------------------------------------

func TestMaVanBan_HaiSoKhongBaoGioDungChung(t *testing.T) {
	// `audit_log.subject` HOLDS THIS, and it is read years later by somebody handling a complaint or
	// an inspection (rule 6, invariant 8). Two things matter: the two registers can never collide,
	// and the strings sort the way the numbers do.
	if a, b := MaVanBanDen(2026, 7), MaVanBanDi(2026, 7); a == b {
		t.Fatalf("số đến 7 và số đi 7 cùng một mã %q — hai văn bản khác nhau trong cùng một xã, "+
			"cùng một năm, và cả hai đều đúng", a)
	}
	if got := MaVanBanDen(2026, 7); got != "VB-DEN-2026-0007" {
		t.Fatalf("MaVanBanDen = %q, muốn VB-DEN-2026-0007", got)
	}
	if got := MaVanBanDi(2026, 12); got != "VB-DI-2026-0012" {
		t.Fatalf("MaVanBanDi = %q, muốn VB-DI-2026-0012", got)
	}
	// ZERO-PADDED, so số 7 sorts before số 12 in a ledger nobody can re-sort.
	if MaVanBanDen(2026, 7) > MaVanBanDen(2026, 12) {
		t.Fatal("mã không sắp đúng thứ tự số — thiếu đệm số 0")
	}
}
