package domain

import "testing"

// WHAT THIS FILE PROVES: the two lookups that stand between a commune's configuration and a
// promise made to a citizen. Both can fail in the SAME SILENT DIRECTION — returning A row rather
// than NO row — and that direction is the dangerous one: a refusal is visible, a wrong number is
// a commitment the authority never made (rule 10).
//
// The fixtures therefore give every row a DIFFERENT set of hours, and no two kinds of work share
// a figure. A lookup that crossed `loai_viec` or fell back when it should not would otherwise
// come back with a plausible number and every assertion would pass.

// motBangMau is a commune that has configured two kinds of work:
//
//	phan-anh     a default row, and a row for `an-ninh-trat-tu` that is much tighter
//	van-ban-den  a default row only
//
// `nhiem-vu` is absent on purpose — a commune that does not use the task module, which
// VanDeCuaSLA must NOT report as a defect.
func motBangMau() []DongSLA {
	return []DongSLA{
		{
			ID: "sla-pa-mac-dinh", LoaiViec: LoaiViecPhanAnh, LinhVuc: "",
			GioTiepNhan: 8, GioXuLyXong: 56, GioSapDenHan: 24,
			GioBaoLanhDao: 25, GioBaoChuTich: 49,
		},
		{
			ID: "sla-pa-an-ninh", LoaiViec: LoaiViecPhanAnh, LinhVuc: "an-ninh-trat-tu",
			GioTiepNhan: 2, GioXuLyXong: 16, GioSapDenHan: 4,
			GioBaoLanhDao: 9, GioBaoChuTich: 17,
		},
		{
			ID: "sla-vb-mac-dinh", LoaiViec: LoaiViecVanBanDen, LinhVuc: "",
			GioTiepNhan: 7, GioXuLyXong: 40, GioSapDenHan: 23,
			GioBaoLanhDao: 26, GioBaoChuTich: 50,
		},
	}
}

func TestLoaiViecChiNhanBaGiaTri(t *testing.T) {
	// ADR 0029, stop condition #1: a fourth value is a decision, not a deployment. This is the
	// Go half of the CHECK constraint in migration 0008, and it exists so a row the database
	// should have refused cannot be read as if it were ordinary.
	for _, v := range []LoaiViec{LoaiViecVanBanDen, LoaiViecPhanAnh, LoaiViecNhiemVu} {
		if !v.HopLe() {
			t.Errorf("%q phải hợp lệ", v)
		}
	}
	for _, v := range []LoaiViec{"", "PHAN-ANH", "phan_anh", "giai-ngan", "van-ban-di"} {
		if v.HopLe() {
			t.Errorf("%q KHÔNG được coi là hợp lệ", v)
		}
	}
}

func TestDongMacDinhLaDongCoLinhVucRong(t *testing.T) {
	if !(DongSLA{LinhVuc: ""}).LaDongMacDinh() {
		t.Error("dòng linh_vuc rỗng phải là dòng mặc định")
	}
	if (DongSLA{LinhVuc: "dien"}).LaDongMacDinh() {
		t.Error("dòng có mã lĩnh vực không phải dòng mặc định")
	}
}

// --- what the table cannot answer ---------------------------------------------------------------

func TestBangRongDuocNeuTenChuKhongImLang(t *testing.T) {
	// TODAY'S ANSWER FOR EVERY COMMUNE: migration 0008 creates the table and seeds nothing, and
	// the onboarding step that would sow a commune's first rows does not exist.
	//
	// The empty table must be a NAMED problem rather than an empty list the caller shrugs at:
	// falling back to "24 hours", or to the sixteen rows of the specification, is a commitment
	// invented by software and then told to a citizen (rule 10, forbidden #3).
	vd := VanDeCuaSLA(nil)
	if len(vd) != 1 || vd[0].Loai != VanDeSLATrong {
		t.Fatalf("bảng rỗng phải được nêu tên là vấn đề, nhận: %+v", vd)
	}
	if vd[0].LoaiViec != "" {
		t.Errorf("bảng rỗng không thuộc về một loại việc nào: %+v", vd[0])
	}
}

func TestThieuDongMacDinhLaMotLoHongDuocNeuTen(t *testing.T) {
	// THE HOLE THAT IS INVISIBLE ON THE SCREEN. A commune that configured `an-ninh-trat-tu` and
	// nothing else looks configured. Every petition in any other field has nothing to fall back
	// on — and on the citizen channels every petition at all, because ADR 0028 decision E reads
	// `han_tiep_nhan` from the DEFAULT row while the field is still unknown.
	ds := []DongSLA{{
		ID: "sla-pa-an-ninh", LoaiViec: LoaiViecPhanAnh, LinhVuc: "an-ninh-trat-tu",
		GioTiepNhan: 2, GioXuLyXong: 16, GioSapDenHan: 4, GioBaoLanhDao: 9, GioBaoChuTich: 17,
	}}

	vd := VanDeCuaSLA(ds)
	if len(vd) != 1 {
		t.Fatalf("nhận %d vấn đề, muốn 1: %+v", len(vd), vd)
	}
	if vd[0].Loai != VanDeThieuDongMacDinh || vd[0].LoaiViec != LoaiViecPhanAnh {
		t.Errorf("vấn đề sai: %+v", vd[0])
	}
}

func TestLoaiViecKhongCauHinhThiKHONGPhaiLoi(t *testing.T) {
	// DELIBERATE OMISSION, NOT AN OVERSIGHT: `nhiem-vu` is absent from motBangMau and must stay
	// absent from the problem list. A commune that does not use the task module has no row for
	// it, and reporting that as a defect would be this function deciding a commune's
	// administrative practice — the one thing CLAUDE.md says never to do silently.
	if vd := VanDeCuaSLA(motBangMau()); len(vd) != 0 {
		t.Fatalf("bảng đầy đủ mà vẫn báo vấn đề: %+v", vd)
	}
}

func TestDanhSachVanDeSapXepOnDinh(t *testing.T) {
	// Map iteration order is random in Go. A problem list that reorders between two loads of the
	// same screen reads as "something changed" when nothing did — and the person reading it is
	// the one trying to work out what is misconfigured.
	ds := []DongSLA{
		{ID: "a", LoaiViec: LoaiViecPhanAnh, LinhVuc: "dien"},
		{ID: "b", LoaiViec: LoaiViecVanBanDen, LinhVuc: "cong-van"},
		{ID: "c", LoaiViec: LoaiViecNhiemVu, LinhVuc: "khac"},
	}
	muon := []LoaiViec{LoaiViecNhiemVu, LoaiViecPhanAnh, LoaiViecVanBanDen} // nhiem-vu < phan-anh < van-ban-den

	for lan := 0; lan < 20; lan++ {
		vd := VanDeCuaSLA(ds)
		if len(vd) != 3 {
			t.Fatalf("nhận %d vấn đề, muốn 3", len(vd))
		}
		for i := range muon {
			if vd[i].LoaiViec != muon[i] {
				t.Fatalf("lần %d: thứ tự = %+v, muốn %v", lan, vd, muon)
			}
		}
	}
}

// --- choosing the row a promise is computed from ------------------------------------------------

func TestDongMacDinhKhongLayNhamLoaiViecKhac(t *testing.T) {
	// THE SILENT FAILURE THIS PINS. `van-ban-den` promises 40 working hours, `phan-anh` promises
	// 56. A lookup that ignored `loai_viec` would return a row, the caller would compute a
	// deadline, and nothing anywhere would report that a citizen's petition had just been given
	// the incoming-document deadline.
	ds := motBangMau()

	d, co := DongMacDinh(ds, LoaiViecPhanAnh)
	if !co {
		t.Fatal("không tìm thấy dòng mặc định của phản ánh")
	}
	if d.ID != "sla-pa-mac-dinh" || d.GioXuLyXong != 56 {
		t.Errorf("dòng mặc định sai: %+v", d)
	}

	d, co = DongMacDinh(ds, LoaiViecVanBanDen)
	if !co || d.GioXuLyXong != 40 {
		t.Errorf("dòng mặc định của văn bản đến sai: %+v (tìm thấy=%v)", d, co)
	}
}

func TestDongMacDinhVangMatThiTraFalseChuKhongTraDongRong(t *testing.T) {
	// FALSE MEANS REFUSE. A zero DongSLA would be five zero-hour deadlines — a commitment
	// breached at the instant it is made — and it would flow onward looking like a number.
	d, co := DongMacDinh(motBangMau(), LoaiViecNhiemVu)
	if co {
		t.Fatalf("loại việc chưa cấu hình mà vẫn tìm ra dòng: %+v", d)
	}
	if d != (DongSLA{}) {
		t.Errorf("không tìm thấy mà vẫn trả dữ liệu: %+v", d)
	}
}

func TestDongTheoLinhVucUuTienDongRiengCuaLinhVuc(t *testing.T) {
	// The whole point of ADR 0028 decision E: once the field is settled, the field's OWN row is
	// what fixes `han_xu_ly_xong`. Falling back here would rebuild the 56-hour ceiling that ADR
	// removed, and the three urgent fields would become unreachable.
	d, co := DongTheoLinhVuc(motBangMau(), LoaiViecPhanAnh, "an-ninh-trat-tu")
	if !co {
		t.Fatal("không tìm thấy dòng của an-ninh-trat-tu")
	}
	if d.ID != "sla-pa-an-ninh" {
		t.Fatalf("lấy nhầm dòng: %+v", d)
	}
	if d.GioTiepNhan != 2 || d.GioXuLyXong != 16 {
		t.Errorf("số giờ sai: tiếp nhận=%d xử lý xong=%d, muốn 2 và 16",
			d.GioTiepNhan, d.GioXuLyXong)
	}
}

func TestDongTheoLinhVucRoiVeDongMacDinhKhiLinhVucChuaCauHinh(t *testing.T) {
	// The fallback is the SPECIFICATION's, not this function's: `linh_vuc = null` is labelled
	// "Mặc định cho mọi lĩnh vực" (14-cau-hinh.md:311, :316).
	d, co := DongTheoLinhVuc(motBangMau(), LoaiViecPhanAnh, "rac-thai-ve-sinh-moi-truong")
	if !co {
		t.Fatal("lĩnh vực chưa cấu hình phải rơi về dòng mặc định")
	}
	if d.ID != "sla-pa-mac-dinh" {
		t.Fatalf("rơi về nhầm dòng: %+v", d)
	}
}

func TestDongTheoLinhVucKhongBaoGioRoiSangLoaiViecKhac(t *testing.T) {
	// The fallback goes DOWN to the default row of the SAME kind of work, never ACROSS. A
	// commune that configured only `van-ban-den` has no petition deadline at all, and that must
	// come back as a refusal rather than as the document deadline wearing a petition's name.
	chiVanBan := []DongSLA{{
		ID: "sla-vb-mac-dinh", LoaiViec: LoaiViecVanBanDen, LinhVuc: "",
		GioTiepNhan: 7, GioXuLyXong: 40, GioSapDenHan: 23, GioBaoLanhDao: 26, GioBaoChuTich: 50,
	}}

	if d, co := DongTheoLinhVuc(chiVanBan, LoaiViecPhanAnh, "an-ninh-trat-tu"); co {
		t.Fatalf("RÒ SANG LOẠI VIỆC KHÁC: %+v", d)
	}
}

func TestDongTheoLinhVucKhongLayDongCungMaCuaLoaiViecKhac(t *testing.T) {
	// FOUND BY MUTATION, NOT BY READING THE CODE. Dropping `d.LoaiViec == loai` from the
	// exact-match branch of DongTheoLinhVuc left every other test in this file GREEN: the only
	// field code in the fixtures existed under one kind of work, so ignoring the kind still
	// landed on the right row. The test that would have caught it did not exist.
	//
	// THE CODES REALLY DO OVERLAP. `linh_vuc` holds a value, not a key (ADR 0026), and nothing
	// stops two kinds of work carrying the same string — `khac` is in the shipped set and the
	// specification's own table gives `phan-anh` a `Khác` row. A lookup blind to `loai_viec`
	// answers whichever row it meets first, and the difference between the two here is 16
	// working hours against 168: the difference between "today" and "next week", promised to a
	// citizen holding a lookup code.
	cungMa := []DongSLA{
		{
			ID: "sla-vb-khac", LoaiViec: LoaiViecVanBanDen, LinhVuc: "khac",
			GioTiepNhan: 8, GioXuLyXong: 168, GioSapDenHan: 24,
			GioBaoLanhDao: 26, GioBaoChuTich: 50,
		},
		{
			ID: "sla-pa-khac", LoaiViec: LoaiViecPhanAnh, LinhVuc: "khac",
			GioTiepNhan: 2, GioXuLyXong: 16, GioSapDenHan: 4,
			GioBaoLanhDao: 9, GioBaoChuTich: 17,
		},
	}

	// The petition row comes SECOND in the slice on purpose: a lookup that ignores `loai_viec`
	// returns the first match, so this ordering is what makes the defect visible.
	d, co := DongTheoLinhVuc(cungMa, LoaiViecPhanAnh, "khac")
	if !co {
		t.Fatal("không tìm thấy dòng `khac` của phản ánh")
	}
	if d.ID != "sla-pa-khac" || d.GioXuLyXong != 16 {
		t.Fatalf("lấy dòng cùng mã của LOẠI VIỆC KHÁC: %+v", d)
	}

	d, co = DongTheoLinhVuc(cungMa, LoaiViecVanBanDen, "khac")
	if !co || d.ID != "sla-vb-khac" || d.GioXuLyXong != 168 {
		t.Fatalf("chiều ngược lại cũng sai: %+v (tìm thấy=%v)", d, co)
	}
}

func TestLinhVucRongHoiDungDongMacDinh(t *testing.T) {
	// "" is what the default row's field code IS (LaDongMacDinh), so asking with "" asks for the
	// default row. This is the shape ADR 0028 decision E uses on every citizen channel, where
	// the field is not known when `han_tiep_nhan` is fixed.
	d, co := DongTheoLinhVuc(motBangMau(), LoaiViecPhanAnh, "")
	if !co || d.ID != "sla-pa-mac-dinh" {
		t.Fatalf("lĩnh vực rỗng phải cho ra dòng mặc định, nhận %+v (tìm thấy=%v)", d, co)
	}
}

func TestBangRongThiKhongTraDongNao(t *testing.T) {
	// Fail closed, on both lookups. This is the state EVERY commune is in today.
	if d, co := DongMacDinh(nil, LoaiViecPhanAnh); co {
		t.Errorf("bảng rỗng mà vẫn ra dòng mặc định: %+v", d)
	}
	if d, co := DongTheoLinhVuc(nil, LoaiViecPhanAnh, "dien"); co {
		t.Errorf("bảng rỗng mà vẫn ra dòng theo lĩnh vực: %+v", d)
	}
}
