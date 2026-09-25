package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the LIFECYCLE acts of the meeting-minutes register (user decisions 25/09/2026), over the
// REAL store on the fake driver.
//
//	PROVED HERE   draft vs signed for every write · the meeting row is locked FOR UPDATE before any
//	              decision · a request that changes nothing writes NOTHING · the notice after signing
//	              is written once · signing twice is refused and signs once · deletes refused while a
//	              live task points at a conclusion (with the count) · a conclusion's text locked by
//	              tasks, NOT by the mark · the mark refused with live tasks · a split refused while the
//	              mark is set, including when the mark lands between the split's first read and its
//	              transaction · supplementary minutes only for SIGNED originals · a trigger refusal
//	              that slips through maps to the 409 sentinel · no audit delta carries minutes or
//	              conclusion TEXT · the actor is the staff business code.
//
//	NOT PROVED    what PostgreSQL does — the triggers, CHECKs and FK of migration 0012 are proved by
//	              migrations/bien_ban_vong_doi_test.go and the pg suites (skipped without a DSN).

// --- helpers --------------------------------------------------------------------------------------

func kyFixture(k *khoBienBanGia) {
	r := k.bienBan[idBBGoc]
	r["trang_thai"] = domain.TrangThaiBienBanDaKy
	r["ky_luc"] = mocTaoBB
	r["ky_boi_ma"] = "CB-00009"
}

func khongGhiCauNaoBB(t *testing.T, k *khoBienBanGia) {
	t.Helper()
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "INSERT") || strings.HasPrefix(l.sql, "UPDATE") {
			t.Errorf("đã ghi: %q", l.sql)
		}
	}
}

func deltaBB(t *testing.T, k *khoBienBanGia) string {
	t.Helper()
	return string(vetKiemToanBB(t, k).args[7].([]byte))
}

func khongMangVanBan(t *testing.T, delta string, cam ...string) {
	t.Helper()
	for _, c := range cam {
		if c != "" && strings.Contains(delta, c) {
			t.Errorf("delta mang nguyên văn — sổ vết không xoá được, biên bản xã trích dẫn vụ việc: %s", delta)
		}
	}
}

func khoaBienBanTruoc(t *testing.T, k *khoBienBanGia) {
	t.Helper()
	iKhoa, iGhi := -1, -1
	for i, l := range k.lenh {
		if iKhoa < 0 && strings.Contains(l.sql, "FROM bien_ban_hop") && strings.Contains(l.sql, "FOR UPDATE") {
			iKhoa = i
		}
		if iGhi < 0 && strings.HasPrefix(l.sql, "UPDATE") {
			iGhi = i
		}
	}
	if iKhoa < 0 || (iGhi >= 0 && iKhoa > iGhi) {
		t.Errorf("không khoá dòng biên bản FOR UPDATE trước khi ghi (khoá %d, ghi %d)", iKhoa, iGhi)
	}
}

func chuoi(s string) *string { return &s }

// --- 1. PATCH the minutes ----------------------------------------------------------------------------

func TestSuaBienBan_BanNhapMotGiaoDichVetChiMangDoDai(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	const moi = "Toàn văn MỚI: bổ sung ý kiến của trưởng thôn về hộ ông Nguyễn Văn A."
	sau, err := uc.SuaBienBan(ctx, idBBGoc, YeuCauSuaBienBan{
		TenCuocHop: chuoi("Giao ban tháng 8 (bản sửa)"),
		NoiDung:    chuoi(moi),
		ThuKyMa:    chuoi("CB-00011"),
	}, canBoThu())
	if err != nil {
		t.Fatalf("sửa biên bản nháp: %v", err)
	}
	if sau.TenCuocHop != "Giao ban tháng 8 (bản sửa)" || sau.ThuKyMa != "CB-00011" {
		t.Errorf("bản ghi sau khi sửa = %+v", sau)
	}
	if n := len(k.cau("UPDATE bien_ban_hop")); n != 1 {
		t.Fatalf("ghi %d câu UPDATE biên bản, muốn 1", n)
	}
	chiGhiTrongGiaoDichBB(t, k)
	khoaBienBanTruoc(t, k)

	vet := vetKiemToanBB(t, k)
	if vet.args[1] != maCanBoThu || vet.args[4] != HanhViSuaBienBanHop {
		t.Errorf("vết: chủ thể %v, hành vi %v", vet.args[1], vet.args[4])
	}
	d := deltaBB(t, k)
	khongMangVanBan(t, d, noiDungBBGoc, moi)
	if !strings.Contains(d, `"do_dai_noi_dung"`) {
		t.Errorf("delta không ghi độ dài nội dung trước/sau: %s", d)
	}
}

func TestSuaBienBan_KhongDoiGiThiKhongGhiGi(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	// The same values the fixture holds — a client sending the form back unchanged.
	_, err := uc.SuaBienBan(ctx, idBBGoc, YeuCauSuaBienBan{
		TenCuocHop: chuoi("  " + tenCuocHopBB + " "),
		NoiDung:    chuoi(noiDungBBGoc),
		NgayHop:    &mocNgayHopBB,
		ThanhPhan:  &[]string{"CB-00007", "Đại diện Mặt trận Tổ quốc xã"},
	}, canBoThu())
	if err != nil {
		t.Fatalf("sửa không đổi gì: %v", err)
	}
	khongGhiCauNaoBB(t, k)
	if k.coCau("INSERT INTO audit_log") {
		t.Error("ghi vết cho một lần sửa không đổi gì — sổ vết đếm được một hành vi không xảy ra")
	}
}

func TestSuaBienBan_DaKyThiTuChoiSuaNoiDung(t *testing.T) {
	k := khoBBMau()
	kyFixture(k)
	uc, _, ctx := dungGhiBienBan(t, k)

	_, err := uc.SuaBienBan(ctx, idBBGoc, YeuCauSuaBienBan{NoiDung: chuoi("Sửa sau khi ký.")}, canBoThu())
	if !errors.Is(err, domain.ErrBienBanDaKy) {
		t.Fatalf("lỗi = %v, muốn ErrBienBanDaKy", err)
	}
	khongGhiCauNaoBB(t, k)
}

func TestSuaBienBan_DaKyGhiThongBaoDungMotLan(t *testing.T) {
	k := khoBBMau()
	kyFixture(k)
	uc, _, ctx := dungGhiBienBan(t, k)

	ngay := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	// The title is sent back UNCHANGED alongside the notice: not a change, so not refused.
	_, err := uc.SuaBienBan(ctx, idBBGoc, YeuCauSuaBienBan{
		TenCuocHop: chuoi(tenCuocHopBB),
		ThongBao:   &ThongBaoKetLuan{SoKyHieu: "12/TB-UBND", Ngay: ngay},
	}, canBoThu())
	if err != nil {
		t.Fatalf("ghi thông báo sau khi ký: %v", err)
	}
	ghi := k.cau("UPDATE bien_ban_hop")
	if len(ghi) != 1 || !strings.Contains(ghi[0].sql, "tb_so_ky_hieu IS NULL") {
		t.Fatalf("câu ghi thông báo phải mang điều kiện 'chưa có thông báo': %+v", ghi)
	}
	if strings.Contains(ghi[0].sql, "noi_dung") {
		t.Error("câu ghi thông báo sau khi ký chạm vào cột nội dung")
	}
	if vet := vetKiemToanBB(t, k); vet.args[4] != HanhViGhiThongBaoKetLuan {
		t.Errorf("hành vi = %v, muốn %q", vet.args[4], HanhViGhiThongBaoKetLuan)
	}
}

func TestSuaBienBan_DaKyThongBaoDaCo(t *testing.T) {
	ngay := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	for _, ca := range []struct {
		ten string
		so  string
		loi error
	}{
		{"thông báo khác thì từ chối", "13/TB-UBND", domain.ErrDaCoThongBao},
		{"gửi lại đúng thông báo đã ghi thì không ghi gì", "12/TB-UBND", nil},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			k := khoBBMau()
			kyFixture(k)
			k.bienBan[idBBGoc]["tb_so_ky_hieu"] = "12/TB-UBND"
			k.bienBan[idBBGoc]["tb_ngay"] = ngay
			uc, _, ctx := dungGhiBienBan(t, k)

			_, err := uc.SuaBienBan(ctx, idBBGoc, YeuCauSuaBienBan{
				ThongBao: &ThongBaoKetLuan{SoKyHieu: ca.so, Ngay: ngay}}, canBoThu())
			if !errors.Is(err, ca.loi) && !(ca.loi == nil && err == nil) {
				t.Fatalf("lỗi = %v, muốn %v", err, ca.loi)
			}
			khongGhiCauNaoBB(t, k)
		})
	}
}

// TestSuaBienBan_TriggerTuChoiThanhLoi409 — if a trigger refusal ever slips past the use case's
// checks, it must reach the client as the 409 sentinel, not as a 500.
func TestSuaBienBan_TriggerTuChoiThanhLoi409(t *testing.T) {
	for _, ca := range []struct {
		ten, msg string
		muon     error
	}{
		{"khoá biên bản đã ký", "archival record bien_ban_hop: signed minutes cannot be edited or removed",
			domain.ErrBienBanDaKy},
		{"thông báo chỉ ghi một lần",
			"archival record bien_ban_hop: the conclusion notice reference of signed minutes is recorded once",
			domain.ErrDaCoThongBao},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			k := khoBBMau()
			k.loiTrigger, k.loiTriggerMsg = "UPDATE bien_ban_hop", ca.msg
			uc, _, ctx := dungGhiBienBan(t, k)

			_, err := uc.SuaBienBan(ctx, idBBGoc, YeuCauSuaBienBan{NoiDung: chuoi("Một bản sửa.")}, canBoThu())
			if !errors.Is(err, ca.muon) {
				t.Fatalf("lỗi = %v, muốn %v", err, ca.muon)
			}
			if k.daCommit != 0 {
				t.Errorf("commit %d lần sau khi trigger từ chối", k.daCommit)
			}
		})
	}
}

// --- 2. DELETE the minutes ----------------------------------------------------------------------------

func TestXoaBienBan_BanNhapKhongNhiemVuThiXoaMem(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	const lyDo = "Nhập trùng với biên bản số 31 — hộ bà Trần Thị B khiếu nại."
	if err := uc.XoaBienBan(ctx, idBBGoc, lyDo, canBoThu()); err != nil {
		t.Fatalf("xoá biên bản nháp: %v", err)
	}
	ghi := k.cau("UPDATE bien_ban_hop")
	if len(ghi) != 1 || !strings.Contains(ghi[0].sql, "deleted_at") {
		t.Fatalf("không xoá mềm: %+v", ghi)
	}
	// `deleted_by` ($4) IS THE STAFF BUSINESS CODE.
	if ghi[0].args[3] != maCanBoThu {
		t.Errorf("deleted_by = %v, muốn %q", ghi[0].args[3], maCanBoThu)
	}
	chiGhiTrongGiaoDichBB(t, k)
	khoaBienBanTruoc(t, k)
	if vet := vetKiemToanBB(t, k); vet.args[4] != HanhViXoaBienBanHop {
		t.Errorf("hành vi = %v", vet.args[4])
	}
	khongMangVanBan(t, deltaBB(t, k), lyDo)
}

func TestXoaBienBan_ConNhiemVuThiTuChoiKemSoLuong(t *testing.T) {
	k := khoBBMau()
	k.soNVBienBan = 2
	uc, _, ctx := dungGhiBienBan(t, k)

	err := uc.XoaBienBan(ctx, idBBGoc, "Nhập trùng.", canBoThu())
	var lc *domain.LoiConNhiemVu
	if !errors.As(err, &lc) || lc.SoNhiemVu != 2 {
		t.Fatalf("lỗi = %v, muốn LoiConNhiemVu{2}", err)
	}
	khongGhiCauNaoBB(t, k)
}

func TestXoaBienBan_DaKyThiTuChoi(t *testing.T) {
	k := khoBBMau()
	kyFixture(k)
	uc, _, ctx := dungGhiBienBan(t, k)

	if err := uc.XoaBienBan(ctx, idBBGoc, "Nhập trùng.", canBoThu()); !errors.Is(err, domain.ErrBienBanDaKy) {
		t.Fatalf("lỗi = %v, muốn ErrBienBanDaKy", err)
	}
	khongGhiCauNaoBB(t, k)
}

func TestXoaBienBan_ThieuLyDoThiKhongMoGiaoDich(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	if err := uc.XoaBienBan(ctx, idBBGoc, "  ", canBoThu()); !errors.Is(err, domain.ErrThieuLyDoXoaBienBan) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoXoaBienBan", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch cho một yêu cầu thiếu lý do", k.batDau)
	}
}

// --- 3. signing ------------------------------------------------------------------------------------------

func TestKyBienBan_BanNhapThanhDaKyMangMaCanBoVaDongHo(t *testing.T) {
	k := khoBBMau()
	// A meeting with NO conclusions may be signed (§7.3).
	k.ketLuan = nil
	uc, _, ctx := dungGhiBienBan(t, k)

	sau, err := uc.KyBienBan(ctx, idBBGoc, YeuCauKyBienBan{}, canBoThu())
	if err != nil {
		t.Fatalf("ký biên bản: %v", err)
	}
	if sau.TrangThai != domain.TrangThaiBienBanDaKy || sau.KyBoiMa != maCanBoThu ||
		!sau.KyLuc.Equal(mocThaoTacBB) {
		t.Errorf("bản ghi sau khi ký = %+v", sau)
	}
	ghi := k.cau("UPDATE bien_ban_hop")
	if len(ghi) != 1 {
		t.Fatalf("ghi %d câu ký, muốn 1", len(ghi))
	}
	a := ghi[0].args
	if a[2] != domain.TrangThaiBienBanDaKy || a[4] != maCanBoThu || a[7] != domain.TrangThaiBienBanDuThao {
		t.Errorf("tham số câu ký = %v — phải chuyển du-thao → da-ky, người ký là mã cán bộ", a)
	}
	if luc, ok := a[3].(time.Time); !ok || !luc.Equal(mocThaoTacBB) {
		t.Errorf("ky_luc = %v, muốn đồng hồ của use case", a[3])
	}
	chiGhiTrongGiaoDichBB(t, k)
	khoaBienBanTruoc(t, k)
	if vet := vetKiemToanBB(t, k); vet.args[4] != HanhViKyBienBanHop || vet.args[1] != maCanBoThu {
		t.Errorf("vết ký: %v / %v", vet.args[4], vet.args[1])
	}
	if k.coCau("FROM ket_luan_hop") {
		t.Error("ký biên bản đọc kết luận — biên bản không có kết luận vẫn ký được, không cần đếm")
	}
}

func TestKyBienBan_KyHaiLanThiTuChoiVaKhongGhi(t *testing.T) {
	k := khoBBMau()
	kyFixture(k)
	uc, _, ctx := dungGhiBienBan(t, k)

	if _, err := uc.KyBienBan(ctx, idBBGoc, YeuCauKyBienBan{}, canBoThu()); !errors.Is(err, domain.ErrBienBanDaKy) {
		t.Fatalf("lỗi = %v, muốn ErrBienBanDaKy — lần ký thứ hai không phải một chữ ký mới", err)
	}
	khongGhiCauNaoBB(t, k)
}

func TestKyBienBan_KemThongBao(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	ngay := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	if _, err := uc.KyBienBan(ctx, idBBGoc, YeuCauKyBienBan{
		ThongBao: &ThongBaoKetLuan{SoKyHieu: " 12/TB-UBND ", Ngay: ngay}}, canBoThu()); err != nil {
		t.Fatalf("ký kèm thông báo: %v", err)
	}
	a := k.cau("UPDATE bien_ban_hop")[0].args
	if a[5] != "12/TB-UBND" {
		t.Errorf("tb_so_ky_hieu = %v", a[5])
	}
}

func TestKyBienBan_ThongBaoThieuNgayThiTuChoiTruocGiaoDich(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	_, err := uc.KyBienBan(ctx, idBBGoc, YeuCauKyBienBan{
		ThongBao: &ThongBaoKetLuan{SoKyHieu: "12/TB-UBND"}}, canBoThu())
	if !errors.Is(err, domain.ErrThieuNgayThongBao) {
		t.Fatalf("lỗi = %v, muốn ErrThieuNgayThongBao", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch", k.batDau)
	}
}

// --- 4. one conclusion ------------------------------------------------------------------------------------

func TestSuaKetLuan_BanNhapKhongNhiemVu(t *testing.T) {
	k := khoBBMau()
	uc, _, ctx := dungGhiBienBan(t, k)

	const moi = "Giao Địa chính rà soát lại hồ sơ đất của hộ ông Lê Văn C, báo cáo trước 25/8."
	kl, err := uc.SuaKetLuan(ctx, idBBGoc, int(thuTuKLGoc), YeuCauSuaKetLuan{NoiDung: moi}, canBoThu())
	if err != nil {
		t.Fatalf("sửa kết luận: %v", err)
	}
	if kl.NoiDung != moi || kl.ThuTu != int(thuTuKLGoc) {
		t.Errorf("kết luận sau khi sửa = %+v", kl)
	}
	if n := len(k.cau("UPDATE ket_luan_hop")); n != 1 {
		t.Fatalf("ghi %d câu sửa kết luận, muốn 1", n)
	}
	chiGhiTrongGiaoDichBB(t, k)
	khoaBienBanTruoc(t, k)
	vet := vetKiemToanBB(t, k)
	if vet.args[4] != HanhViSuaKetLuan || vet.args[5] != "bien-ban-hop/2026-08-05/31/BB-UBND/ket-luan/3" {
		t.Errorf("vết: %v / %v", vet.args[4], vet.args[5])
	}
	khongMangVanBan(t, deltaBB(t, k), noiDungKLGoc, moi)
}

func TestSuaKetLuan_CoNhiemVuThiKhoaKeCaKhiNhap(t *testing.T) {
	k := khoBBMau()
	k.soNVKetLuan = map[string]int64{idKLGoc: 1}
	uc, _, ctx := dungGhiBienBan(t, k)

	_, err := uc.SuaKetLuan(ctx, idBBGoc, int(thuTuKLGoc), YeuCauSuaKetLuan{NoiDung: "Câu khác."}, canBoThu())
	if !errors.Is(err, domain.ErrKetLuanDaCoNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrKetLuanDaCoNhiemVu — nhiệm vụ đang trích câu này", err)
	}
	khongGhiCauNaoBB(t, k)
}

// TestSuaKetLuan_DauKhongPhatSinhKhongKhoaNoiDung — only TASKS lock the text (caller's decision).
func TestSuaKetLuan_DauKhongPhatSinhKhongKhoaNoiDung(t *testing.T) {
	k := khoBBMau()
	k.ketLuan[idKLGoc]["khong_phat_sinh"] = true
	uc, _, ctx := dungGhiBienBan(t, k)

	if _, err := uc.SuaKetLuan(ctx, idBBGoc, int(thuTuKLGoc), YeuCauSuaKetLuan{NoiDung: "Câu khác."},
		canBoThu()); err != nil {
		t.Fatalf("kết luận mang dấu 'không phát sinh' bị khoá nội dung: %v", err)
	}
}

func TestSuaKetLuan_DaKyHoacKhongDoi(t *testing.T) {
	t.Run("biên bản đã ký", func(t *testing.T) {
		k := khoBBMau()
		kyFixture(k)
		uc, _, ctx := dungGhiBienBan(t, k)
		_, err := uc.SuaKetLuan(ctx, idBBGoc, int(thuTuKLGoc), YeuCauSuaKetLuan{NoiDung: "Câu khác."}, canBoThu())
		if !errors.Is(err, domain.ErrBienBanDaKy) {
			t.Fatalf("lỗi = %v, muốn ErrBienBanDaKy", err)
		}
		khongGhiCauNaoBB(t, k)
	})
	t.Run("cùng nội dung", func(t *testing.T) {
		k := khoBBMau()
		// Tasks exist, but nothing changes: a no-op is not refused, and writes nothing.
		k.soNVKetLuan = map[string]int64{idKLGoc: 1}
		uc, _, ctx := dungGhiBienBan(t, k)
		if _, err := uc.SuaKetLuan(ctx, idBBGoc, int(thuTuKLGoc), YeuCauSuaKetLuan{NoiDung: noiDungKLGoc},
			canBoThu()); err != nil {
			t.Fatalf("sửa không đổi gì: %v", err)
		}
		khongGhiCauNaoBB(t, k)
	})
	t.Run("kết luận không có", func(t *testing.T) {
		k := khoBBMau()
		uc, _, ctx := dungGhiBienBan(t, k)
		_, err := uc.SuaKetLuan(ctx, idBBGoc, 99, YeuCauSuaKetLuan{NoiDung: "Câu khác."}, canBoThu())
		if !errors.Is(err, petstore.ErrKetLuanKhongTonTai) {
			t.Fatalf("lỗi = %v, muốn ErrKetLuanKhongTonTai", err)
		}
	})
}

func TestXoaKetLuan(t *testing.T) {
	t.Run("nháp, không nhiệm vụ", func(t *testing.T) {
		k := khoBBMau()
		uc, _, ctx := dungGhiBienBan(t, k)
		if err := uc.XoaKetLuan(ctx, idBBGoc, int(thuTuKLGoc), "Ghi nhầm.", canBoThu()); err != nil {
			t.Fatalf("xoá kết luận: %v", err)
		}
		ghi := k.cau("UPDATE ket_luan_hop")
		if len(ghi) != 1 || !strings.Contains(ghi[0].sql, "deleted_at") {
			t.Fatalf("không xoá mềm kết luận: %+v", ghi)
		}
		if strings.Contains(ghi[0].sql, "thu_tu") {
			t.Error("xoá kết luận chạm vào số thứ tự — số đã cấp không bao giờ đổi")
		}
		if vet := vetKiemToanBB(t, k); vet.args[4] != HanhViXoaKetLuan {
			t.Errorf("hành vi = %v", vet.args[4])
		}
		khongMangVanBan(t, deltaBB(t, k), noiDungKLGoc)
	})
	t.Run("còn nhiệm vụ", func(t *testing.T) {
		k := khoBBMau()
		k.soNVKetLuan = map[string]int64{idKLGoc: 3}
		uc, _, ctx := dungGhiBienBan(t, k)
		if err := uc.XoaKetLuan(ctx, idBBGoc, int(thuTuKLGoc), "Ghi nhầm.", canBoThu()); !errors.Is(err, domain.ErrKetLuanDaCoNhiemVu) {
			t.Fatalf("lỗi = %v, muốn ErrKetLuanDaCoNhiemVu", err)
		}
		khongGhiCauNaoBB(t, k)
	})
	t.Run("đã ký", func(t *testing.T) {
		k := khoBBMau()
		kyFixture(k)
		uc, _, ctx := dungGhiBienBan(t, k)
		if err := uc.XoaKetLuan(ctx, idBBGoc, int(thuTuKLGoc), "Ghi nhầm.", canBoThu()); !errors.Is(err, domain.ErrBienBanDaKy) {
			t.Fatalf("lỗi = %v, muốn ErrBienBanDaKy", err)
		}
		khongGhiCauNaoBB(t, k)
	})
}

// --- 5. the no-task mark -----------------------------------------------------------------------------------

func TestDanhDauKhongPhatSinh(t *testing.T) {
	t.Run("nháp, không nhiệm vụ", func(t *testing.T) {
		k := khoBBMau()
		uc, _, ctx := dungGhiBienBan(t, k)
		kl, err := uc.DanhDauKhongPhatSinh(ctx, idBBGoc, int(thuTuKLGoc), canBoThu())
		if err != nil {
			t.Fatalf("đánh dấu: %v", err)
		}
		if !kl.KhongPhatSinh || kl.TrangThai() != domain.KetLuanHoanThanh {
			t.Errorf("kết luận sau khi đánh dấu = %+v (trạng thái %q)", kl, kl.TrangThai())
		}
		ghi := k.cau("UPDATE ket_luan_hop")
		if len(ghi) != 1 || ghi[0].args[3] != maCanBoThu {
			t.Fatalf("câu đánh dấu: %+v — người đánh dấu phải là mã cán bộ", ghi)
		}
		if vet := vetKiemToanBB(t, k); vet.args[4] != HanhViDanhDauKhongPhatSinh {
			t.Errorf("hành vi = %v", vet.args[4])
		}
	})
	t.Run("còn nhiệm vụ thì từ chối", func(t *testing.T) {
		k := khoBBMau()
		k.soNVKetLuan = map[string]int64{idKLGoc: 1}
		uc, _, ctx := dungGhiBienBan(t, k)
		if _, err := uc.DanhDauKhongPhatSinh(ctx, idBBGoc, int(thuTuKLGoc), canBoThu()); !errors.Is(err, domain.ErrKetLuanDaCoNhiemVu) {
			t.Fatalf("lỗi = %v, muốn ErrKetLuanDaCoNhiemVu", err)
		}
		khongGhiCauNaoBB(t, k)
	})
	t.Run("đã ký thì từ chối", func(t *testing.T) {
		k := khoBBMau()
		kyFixture(k)
		uc, _, ctx := dungGhiBienBan(t, k)
		if _, err := uc.DanhDauKhongPhatSinh(ctx, idBBGoc, int(thuTuKLGoc), canBoThu()); !errors.Is(err, domain.ErrBienBanDaKy) {
			t.Fatalf("lỗi = %v, muốn ErrBienBanDaKy", err)
		}
		khongGhiCauNaoBB(t, k)
	})
	t.Run("đã có dấu thì không ghi gì", func(t *testing.T) {
		k := khoBBMau()
		k.ketLuan[idKLGoc]["khong_phat_sinh"] = true
		uc, _, ctx := dungGhiBienBan(t, k)
		if _, err := uc.DanhDauKhongPhatSinh(ctx, idBBGoc, int(thuTuKLGoc), canBoThu()); err != nil {
			t.Fatalf("đánh dấu lại: %v", err)
		}
		khongGhiCauNaoBB(t, k)
	})
}

func TestBoDanhDauKhongPhatSinh(t *testing.T) {
	t.Run("nháp có dấu", func(t *testing.T) {
		k := khoBBMau()
		k.ketLuan[idKLGoc]["khong_phat_sinh"] = true
		uc, _, ctx := dungGhiBienBan(t, k)
		if err := uc.BoDanhDauKhongPhatSinh(ctx, idBBGoc, int(thuTuKLGoc), canBoThu()); err != nil {
			t.Fatalf("bỏ dấu: %v", err)
		}
		if n := len(k.cau("UPDATE ket_luan_hop")); n != 1 {
			t.Fatalf("ghi %d câu bỏ dấu", n)
		}
		if vet := vetKiemToanBB(t, k); vet.args[4] != HanhViBoDauKhongPhatSinh {
			t.Errorf("hành vi = %v", vet.args[4])
		}
	})
	t.Run("đã ký thì từ chối", func(t *testing.T) {
		k := khoBBMau()
		kyFixture(k)
		k.ketLuan[idKLGoc]["khong_phat_sinh"] = true
		uc, _, ctx := dungGhiBienBan(t, k)
		if err := uc.BoDanhDauKhongPhatSinh(ctx, idBBGoc, int(thuTuKLGoc), canBoThu()); !errors.Is(err, domain.ErrBienBanDaKy) {
			t.Fatalf("lỗi = %v, muốn ErrBienBanDaKy", err)
		}
		khongGhiCauNaoBB(t, k)
	})
	t.Run("chưa có dấu thì không ghi gì", func(t *testing.T) {
		k := khoBBMau()
		uc, _, ctx := dungGhiBienBan(t, k)
		if err := uc.BoDanhDauKhongPhatSinh(ctx, idBBGoc, int(thuTuKLGoc), canBoThu()); err != nil {
			t.Fatalf("bỏ dấu khi chưa có: %v", err)
		}
		khongGhiCauNaoBB(t, k)
	})
}

// --- 6. the split refuses while the mark is set ---------------------------------------------------------

func TestTachKetLuan_CoDauKhongPhatSinhThiTuChoi(t *testing.T) {
	k := khoBBMau()
	k.ketLuan[idKLGoc]["khong_phat_sinh"] = true
	uc, nv, ctx := dungGhiBienBan(t, k)

	_, err := uc.TachKetLuanThanhNhiemVu(ctx, idBBGoc, int(thuTuKLGoc), tachMau(), canBoThu())
	if !errors.Is(err, domain.ErrKetLuanKhongPhatSinh) {
		t.Fatalf("lỗi = %v, muốn ErrKetLuanKhongPhatSinh — bỏ dấu trước rồi mới tách", err)
	}
	if nv.daTao {
		t.Error("đã tạo nhiệm vụ từ một kết luận ghi 'không phát sinh nhiệm vụ'")
	}
}

// TestTachKetLuan_DauDatGiuaHaiLanDocThiVanTuChoi is the window the in-transaction check exists for:
// the first read (outside any transaction) sees no mark; another clerk sets it; the check INSIDE the
// task's transaction, under the meeting's lock, must see it and refuse.
func TestTachKetLuan_DauDatGiuaHaiLanDocThiVanTuChoi(t *testing.T) {
	k := khoBBMau()
	uc, nv, ctx := dungGhiBienBan(t, k)
	nv.truocKhiKiem = func() { k.ketLuan[idKLGoc]["khong_phat_sinh"] = true }

	_, err := uc.TachKetLuanThanhNhiemVu(ctx, idBBGoc, int(thuTuKLGoc), tachMau(), canBoThu())
	if !errors.Is(err, domain.ErrKetLuanKhongPhatSinh) {
		t.Fatalf("lỗi = %v, muốn ErrKetLuanKhongPhatSinh từ phép kiểm TRONG giao dịch", err)
	}
	if !nv.coKiem || nv.daTao {
		t.Errorf("phép kiểm nguồn trong giao dịch: có chạy = %v, đã tạo = %v", nv.coKiem, nv.daTao)
	}
	// THE CHECK LOCKED THE MEETING — the lock the delete and the mark take too.
	if !k.coCau("FOR UPDATE") {
		t.Error("phép kiểm trong giao dịch không khoá dòng biên bản")
	}
}

// TestTachKetLuan_BienBanBiXoaGiuaHaiLanDocThi404 — the meeting soft-deleted after the first read:
// the in-transaction check answers the one 404 sentence of this register.
func TestTachKetLuan_BienBanBiXoaGiuaHaiLanDocThi404(t *testing.T) {
	k := khoBBMau()
	uc, nv, ctx := dungGhiBienBan(t, k)
	nv.truocKhiKiem = func() { delete(k.bienBan, idBBGoc) }

	_, err := uc.TachKetLuanThanhNhiemVu(ctx, idBBGoc, int(thuTuKLGoc), tachMau(), canBoThu())
	if !errors.Is(err, petstore.ErrKetLuanKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrKetLuanKhongTonTai", err)
	}
	if nv.daTao {
		t.Error("đã tạo nhiệm vụ trỏ vào kết luận của một biên bản đã xoá")
	}
}

// TestTachKetLuan_BienBanDaKyVanTachDuoc — signing freezes the record, not the assignment of its work.
func TestTachKetLuan_BienBanDaKyVanTachDuoc(t *testing.T) {
	k := khoBBMau()
	kyFixture(k)
	uc, nv, ctx := dungGhiBienBan(t, k)

	if _, err := uc.TachKetLuanThanhNhiemVu(ctx, idBBGoc, int(thuTuKLGoc), tachMau(), canBoThu()); err != nil {
		t.Fatalf("tách kết luận của biên bản đã ký: %v", err)
	}
	if !nv.coKiem || !nv.daTao {
		t.Errorf("có kiểm = %v, đã tạo = %v", nv.coKiem, nv.daTao)
	}
}

// --- 7. supplementary minutes, and appending to signed minutes ---------------------------------------------

func TestTaoBienBan_BoSung(t *testing.T) {
	t.Run("gốc đã ký thì lập được, là bản nháp, kết luận đánh số từ 1", func(t *testing.T) {
		k := khoBBMau()
		kyFixture(k)
		uc, _, ctx := dungGhiBienBan(t, k)
		yc := taoBienBanMau()
		yc.BoSungChoID = idBBGoc
		bb, err := uc.TaoBienBan(ctx, yc, canBoThu())
		if err != nil {
			t.Fatalf("lập biên bản bổ sung: %v", err)
		}
		if bb.BoSungChoID != idBBGoc || bb.TrangThai != domain.TrangThaiBienBanDuThao ||
			bb.KetLuan[0].ThuTu != 1 {
			t.Errorf("biên bản bổ sung = %+v", bb)
		}
		chen := k.cau("INSERT INTO bien_ban_hop")
		if len(chen) != 1 || chen[0].args[11] != idBBGoc {
			t.Fatalf("cột bo_sung_cho_id không nhận id gốc: %+v", chen)
		}
		chiGhiTrongGiaoDichBB(t, k)
	})
	t.Run("gốc còn nháp thì từ chối", func(t *testing.T) {
		k := khoBBMau()
		uc, _, ctx := dungGhiBienBan(t, k)
		yc := taoBienBanMau()
		yc.BoSungChoID = idBBGoc
		if _, err := uc.TaoBienBan(ctx, yc, canBoThu()); !errors.Is(err, domain.ErrBoSungChoBanNhap) {
			t.Fatalf("lỗi = %v, muốn ErrBoSungChoBanNhap", err)
		}
		khongGhiCauNaoBB(t, k)
	})
	t.Run("gốc không có trong xã thì 404", func(t *testing.T) {
		k := khoBBMau()
		uc, _, ctx := dungGhiBienBan(t, k)
		yc := taoBienBanMau()
		yc.BoSungChoID = "bien-ban-cua-xa-khac"
		if _, err := uc.TaoBienBan(ctx, yc, canBoThu()); !errors.Is(err, ErrBienBanGocKhongTonTai) {
			t.Fatalf("lỗi = %v, muốn ErrBienBanGocKhongTonTai", err)
		}
		khongGhiCauNaoBB(t, k)
	})
}

func TestThemKetLuan_BienBanDaKyThiTuChoi(t *testing.T) {
	k := khoBBMau()
	kyFixture(k)
	uc, _, ctx := dungGhiBienBan(t, k)

	_, err := uc.ThemKetLuan(ctx, idBBGoc, YeuCauThemKetLuan{NoiDung: "Kết luận thêm sau khi ký."}, canBoThu())
	if !errors.Is(err, domain.ErrBienBanDaKy) {
		t.Fatalf("lỗi = %v, muốn ErrBienBanDaKy — kết luận mới thuộc biên bản bổ sung", err)
	}
	khongGhiCauNaoBB(t, k)
}
