package domain

import (
	"errors"
	"testing"
	"time"
)

// Tests for §5.4 / §7.2's document block — the three closed groups, the field checks, and the one
// rule that costs something: what a write request does to the lines already stored.
//
//	PROVED HERE   `nhom` outside the three codes is refused · `thu_tu` of an existing line is read
//	              from the STORED row and survives a request that sends the block in another order ·
//	              a line the request does not name is removed · an unknown line id is refused rather
//	              than created · a line does not move between the three groups · the create path and
//	              the edit path are ONE rule.
//
//	NOT PROVED    anything PostgreSQL does — the three CHECK constraints, `UNIQUE (tenant_id,
//	              nhiem_vu_id, nhom, thu_tu)` counting soft-deleted rows, and the hard-delete
//	              trigger of migration 0009 are the FLOOR under everything here. That half needs a
//	              real server and VIGOV_TEST_DSN is unset in this build environment.

var mocNgayVanBan = time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

// baDongDaLuu is one task's block as the store returns it: two lines in one group, one in another,
// with the positions ALREADY CARRYING A GAP — line ② of `cap-tren-giao` was removed at some point,
// so the live lines are ① and ③.
//
// THE GAP IS THE FIXTURE'S WHOLE POINT. A block numbered 1,2,3 cannot tell "kept the stored number"
// from "renumbered by array index" — both produce the same values — so a test built on one would
// stay green against exactly the defect this file exists to catch.
func baDongDaLuu() []NhiemVuVanBan {
	return []NhiemVuVanBan{
		{ID: "vb-1", NhiemVuID: "nv-1", Nhom: VanBanCapTrenGiao, ThuTu: 1,
			SoKyHieu: "1742-CV/BTCTU", NgayVanBan: mocNgayVanBan,
			TrichYeu: "Công văn của Ban Tổ chức Thành uỷ"},
		{ID: "vb-3", NhiemVuID: "nv-1", Nhom: VanBanCapTrenGiao, ThuTu: 3,
			TrichYeu: "Thông báo số 90-TB/TU ngày 30/01/2026 về ý kiến chỉ đạo"},
		{ID: "vb-9", NhiemVuID: "nv-1", Nhom: VanBanSanPhamRa, ThuTu: 1,
			TrichYeu: "Báo cáo của Ban Thường vụ Đảng uỷ"},
	}
}

// guiLai turns a stored line back into the request shape, the way a drawer re-sends a line it is
// keeping untouched.
func guiLai(v NhiemVuVanBan) VanBanNhiemVuVao {
	return VanBanNhiemVuVao{
		ID: v.ID, Nhom: v.Nhom, SoKyHieu: v.SoKyHieu, NgayVanBan: v.NgayVanBan, TrichYeu: v.TrichYeu,
	}
}

// --- the three groups ------------------------------------------------------------------------------

func TestNhomVanBan_BaMaHopLe(t *testing.T) {
	for _, n := range []NhomVanBanNhiemVu{VanBanCapTrenGiao, VanBanChiDaoDangUy, VanBanSanPhamRa} {
		if !n.HopLe() {
			t.Errorf("nhóm %q bị từ chối — đây là một trong ba ô cố định của §5.4", n)
		}
	}
}

// TestNhomVanBan_MaNgoaiBaGiaTriBiTuChoi is the FAIL-CLOSED case. A line filed under a group no box
// renders is a line nobody ever sees again — on a record that is never hard-deleted.
func TestNhomVanBan_MaNgoaiBaGiaTriBiTuChoi(t *testing.T) {
	for _, n := range []NhomVanBanNhiemVu{
		"", "cap-tren", "khac", "CAP-TREN-GIAO", "san-pham", "van-ban-den",
	} {
		if NhomVanBanNhiemVu(n).HopLe() {
			t.Errorf("nhóm %q được nhận — danh sách ba nhóm là ĐÓNG (§5.4, §7.2)", n)
		}
	}
}

// --- field checks ------------------------------------------------------------------------------------

func TestKiemVanBanNhiemVuVao_NhomSaiThiTuChoi(t *testing.T) {
	_, err := KiemVanBanNhiemVuVao(VanBanNhiemVuVao{Nhom: "khac", TrichYeu: "x"})
	if !errors.Is(err, ErrNhomVanBanKhongHopLe) {
		t.Fatalf("lỗi = %v, muốn ErrNhomVanBanKhongHopLe", err)
	}
}

func TestKiemVanBanNhiemVuVao_ThieuNoiDungThiTuChoi(t *testing.T) {
	// ONLY WHITESPACE IS EMPTY. The schema's `btrim(trich_yeu) <> ''` says the same thing; this layer
	// is the sentence and that one is the floor.
	_, err := KiemVanBanNhiemVuVao(VanBanNhiemVuVao{Nhom: VanBanCapTrenGiao, TrichYeu: "   \t\n "})
	if !errors.Is(err, ErrThieuTrichYeuVanBan) {
		t.Fatalf("lỗi = %v, muốn ErrThieuTrichYeuVanBan", err)
	}
}

// TestKiemVanBanNhiemVuVao_DoDaiTinhBangKyTuKhongPhaiByte pins the rune bound. A byte bound would cut
// a Vietnamese sentence at a third of the length it cuts an English one, and the person who hit it
// would have no way to tell why.
func TestKiemVanBanNhiemVuVao_DoDaiTinhBangKyTuKhongPhaiByte(t *testing.T) {
	dai := make([]rune, TrichYeuVanBanToiDa)
	for i := range dai {
		dai[i] = 'ề' // three bytes in UTF-8
	}
	if _, err := KiemVanBanNhiemVuVao(VanBanNhiemVuVao{
		Nhom: VanBanCapTrenGiao, TrichYeu: string(dai),
	}); err != nil {
		t.Fatalf("%d ký tự tiếng Việt bị từ chối: %v — giới hạn đang đếm byte chứ không đếm ký tự",
			TrichYeuVanBanToiDa, err)
	}
	if _, err := KiemVanBanNhiemVuVao(VanBanNhiemVuVao{
		Nhom: VanBanCapTrenGiao, TrichYeu: string(append(dai, 'a')),
	}); !errors.Is(err, ErrTrichYeuVanBanQuaDai) {
		t.Fatalf("lỗi = %v, muốn ErrTrichYeuVanBanQuaDai", err)
	}
}

func TestKiemVanBanNhiemVuVao_SoKyHieuQuaDaiThiTuChoi(t *testing.T) {
	dai := make([]byte, SoKyHieuVanBanToiDa+1)
	for i := range dai {
		dai[i] = 'A'
	}
	_, err := KiemVanBanNhiemVuVao(VanBanNhiemVuVao{
		Nhom: VanBanCapTrenGiao, TrichYeu: "x", SoKyHieu: string(dai),
	})
	if !errors.Is(err, ErrSoKyHieuVanBanQuaDai) {
		t.Fatalf("lỗi = %v, muốn ErrSoKyHieuVanBanQuaDai", err)
	}
}

func TestKiemVanBanNhiemVuVao_CatKhoangTrangHaiDau(t *testing.T) {
	v, err := KiemVanBanNhiemVuVao(VanBanNhiemVuVao{
		Nhom: VanBanCapTrenGiao, TrichYeu: "  Công văn của Ban Tổ chức  ", SoKyHieu: " 1742-CV/BTCTU ",
	})
	if err != nil {
		t.Fatalf("bị từ chối: %v", err)
	}
	if v.TrichYeu != "Công văn của Ban Tổ chức" || v.SoKyHieu != "1742-CV/BTCTU" {
		t.Errorf("chưa cắt khoảng trắng: trich_yeu=%q so_ky_hieu=%q", v.TrichYeu, v.SoKyHieu)
	}
}

// TestKiemVanBanNhiemVuVao_NgayKhongBiKiemTra states an ABSENCE as an assertion. A document dated
// years back is ordinary (a 2019 directive still being implemented) and one dated ahead of today
// happens too. Refusing either would refuse real configuration.
func TestKiemVanBanNhiemVuVao_NgayKhongBiKiemTra(t *testing.T) {
	for _, ngay := range []time.Time{
		time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC),
		{},
	} {
		if _, err := KiemVanBanNhiemVuVao(VanBanNhiemVuVao{
			Nhom: VanBanCapTrenGiao, TrichYeu: "x", NgayVanBan: ngay,
		}); err != nil {
			t.Errorf("ngày %v bị từ chối: %v", ngay, err)
		}
	}
}

func TestKiemDanhSachVanBanNhiemVu_QuaNhieuDongThiTuChoi(t *testing.T) {
	ds := make([]VanBanNhiemVuVao, VanBanNhiemVuToiDa+1)
	for i := range ds {
		ds[i] = VanBanNhiemVuVao{Nhom: VanBanCapTrenGiao, TrichYeu: "x"}
	}
	if _, err := KiemDanhSachVanBanNhiemVu(ds); !errors.Is(err, ErrQuaNhieuVanBan) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuVanBan", err)
	}
}

// TestKiemDanhSachVanBanNhiemVu_RongVanTraSliceKhongNil is the distinction the whole block rests on:
// an EMPTY list ("remove everything") must stay distinguishable from a list nobody sent, and the
// caller tells them apart by the POINTER it holds — never by the length of this result.
func TestKiemDanhSachVanBanNhiemVu_RongVanTraSliceKhongNil(t *testing.T) {
	ra, err := KiemDanhSachVanBanNhiemVu([]VanBanNhiemVuVao{})
	if err != nil {
		t.Fatalf("danh sách rỗng bị từ chối: %v", err)
	}
	if ra == nil {
		t.Error("trả nil cho một danh sách rỗng — người gọi không còn phân biệt được `gửi rỗng` với `không gửi`")
	}
}

// --- SoSanhVanBan: the rule that costs something -----------------------------------------------------

// TestSoSanhVanBan_DongMoiKhongMangSoThuTu proves the mint is NOT this function's job: a new line
// leaves here with `ThuTu` untouched, so the number can only come from the store's high-water mark
// inside the transaction.
func TestSoSanhVanBan_DongMoiKhongMangSoThuTu(t *testing.T) {
	td, err := SoSanhVanBan(nil, []VanBanNhiemVuVao{
		{Nhom: VanBanCapTrenGiao, TrichYeu: "Công văn 1742-CV/BTCTU"},
		{Nhom: VanBanSanPhamRa, TrichYeu: "Báo cáo 324-BC/ĐU"},
	})
	if err != nil {
		t.Fatalf("so sánh: %v", err)
	}
	if len(td.Them) != 2 || len(td.Sua) != 0 || len(td.Xoa) != 0 {
		t.Fatalf("them=%d sua=%d xoa=%d, muốn 2/0/0", len(td.Them), len(td.Sua), len(td.Xoa))
	}
	if !td.CoGiDoi() {
		t.Error("CoGiDoi = false dù thêm hai dòng")
	}
}

// TestSoSanhVanBan_TaoViecCoIDThiTuChoi is the CREATE path's refusal, and it is not written as a
// branch anywhere: a create diffs against an EMPTY stored block, so an item carrying an id simply
// names a line that is not there. One rule, two callers.
func TestSoSanhVanBan_TaoViecCoIDThiTuChoi(t *testing.T) {
	_, err := SoSanhVanBan(nil, []VanBanNhiemVuVao{
		{ID: "vb-toi-tu-chon", Nhom: VanBanCapTrenGiao, TrichYeu: "x"},
	})
	if !errors.Is(err, ErrVanBanKhongThuocNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrVanBanKhongThuocNhiemVu — máy khách không được tự chọn id nội bộ", err)
	}
}

// TestSoSanhVanBan_GuiLaiNguyenVenThiKhongCoGiDoi is what makes a re-Save write nothing: no
// statement, no audit entry. Remove it and every open-and-close of the drawer files an entry in an
// append-only ledger saying an act happened that changed nothing.
func TestSoSanhVanBan_GuiLaiNguyenVenThiKhongCoGiDoi(t *testing.T) {
	daLuu := baDongDaLuu()
	var yc []VanBanNhiemVuVao
	for _, v := range daLuu {
		yc = append(yc, guiLai(v))
	}

	td, err := SoSanhVanBan(daLuu, yc)
	if err != nil {
		t.Fatalf("so sánh: %v", err)
	}
	if td.CoGiDoi() {
		t.Fatalf("CoGiDoi = true dù không đổi gì: them=%d sua=%d xoa=%d",
			len(td.Them), len(td.Sua), len(td.Xoa))
	}
}

// TestSoSanhVanBan_GiuNguyenSoThuTuDaLuuDuMangDaoNguoc IS THE TEST THIS FILE EXISTS FOR.
//
// The client re-sends the block in a DIFFERENT ORDER and edits the text of one line. A write path
// that renumbered by array position would hand out 1,2,3 — which both loses a recorded fact and, on
// the way to the database, collides with `UNIQUE (tenant_id, nhiem_vu_id, nhom, thu_tu)` because the
// removed line ② still holds the number 2.
//
// The stored positions are 1 and 3 with a GAP, and they must come out as 1 and 3.
func TestSoSanhVanBan_GiuNguyenSoThuTuDaLuuDuMangDaoNguoc(t *testing.T) {
	daLuu := baDongDaLuu()

	// Reversed, and the text of `vb-3` corrected — so the line is in `Sua` and its number is visible.
	sua3 := guiLai(daLuu[1])
	sua3.TrichYeu = "Thông báo số 90-TB/TU ngày 30/01/2026 (đã đính chính)"

	td, err := SoSanhVanBan(daLuu, []VanBanNhiemVuVao{
		guiLai(daLuu[2]), sua3, guiLai(daLuu[0]),
	})
	if err != nil {
		t.Fatalf("so sánh: %v", err)
	}
	if len(td.Them) != 0 || len(td.Xoa) != 0 {
		t.Fatalf("them=%d xoa=%d, muốn 0/0 — không dòng nào được thêm hay gỡ", len(td.Them), len(td.Xoa))
	}
	if len(td.Sua) != 1 {
		t.Fatalf("sua=%d, muốn 1", len(td.Sua))
	}
	if td.Sua[0].ID != "vb-3" {
		t.Fatalf("sửa nhầm dòng %q, muốn vb-3", td.Sua[0].ID)
	}
	if td.Sua[0].ThuTu != 3 {
		t.Fatalf("thu_tu = %d, muốn 3 — SỐ ĐÃ LƯU, không phải vị trí trong mảng (dòng này đứng thứ 2 "+
			"trong yêu cầu, và số 2 vẫn do một dòng ĐÃ GỠ giữ)", td.Sua[0].ThuTu)
	}
	if td.Sua[0].Nhom != VanBanCapTrenGiao {
		t.Errorf("nhom = %q, muốn giữ nguyên nhóm đã lưu", td.Sua[0].Nhom)
	}
	if td.Sua[0].NhiemVuID != "nv-1" {
		t.Errorf("nhiem_vu_id = %q — một dòng không đổi được nhiệm vụ chủ của nó", td.Sua[0].NhiemVuID)
	}
}

// TestSoSanhVanBan_DongKhongDuocGuiLaiThiBiGo is how `✕` reaches the server: the block is sent WHOLE
// and the removed line is simply not in it.
func TestSoSanhVanBan_DongKhongDuocGuiLaiThiBiGo(t *testing.T) {
	daLuu := baDongDaLuu()

	td, err := SoSanhVanBan(daLuu, []VanBanNhiemVuVao{guiLai(daLuu[0]), guiLai(daLuu[2])})
	if err != nil {
		t.Fatalf("so sánh: %v", err)
	}
	if len(td.Xoa) != 1 || td.Xoa[0].ID != "vb-3" {
		t.Fatalf("xoa = %+v, muốn đúng một dòng vb-3", td.Xoa)
	}
	// THE REMOVED LINE CARRIES ITS NUMBER OUT WITH IT, which is what keeps the number taken.
	if td.Xoa[0].ThuTu != 3 {
		t.Errorf("thu_tu của dòng bị gỡ = %d, muốn 3", td.Xoa[0].ThuTu)
	}
}

func TestSoSanhVanBan_GuiRongThiGoHetVaKhongThemGi(t *testing.T) {
	daLuu := baDongDaLuu()

	td, err := SoSanhVanBan(daLuu, []VanBanNhiemVuVao{})
	if err != nil {
		t.Fatalf("so sánh: %v", err)
	}
	if len(td.Xoa) != len(daLuu) || len(td.Them) != 0 || len(td.Sua) != 0 {
		t.Fatalf("them=%d sua=%d xoa=%d, muốn 0/0/%d", len(td.Them), len(td.Sua), len(td.Xoa), len(daLuu))
	}
}

func TestSoSanhVanBan_IDLaKhongCoThiTuChoi(t *testing.T) {
	_, err := SoSanhVanBan(baDongDaLuu(), []VanBanNhiemVuVao{
		{ID: "vb-cua-nhiem-vu-khac", Nhom: VanBanCapTrenGiao, TrichYeu: "x"},
	})
	if !errors.Is(err, ErrVanBanKhongThuocNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrVanBanKhongThuocNhiemVu", err)
	}
}

func TestSoSanhVanBan_MotDongGuiHaiLanThiTuChoi(t *testing.T) {
	daLuu := baDongDaLuu()
	mot := guiLai(daLuu[0])
	hai := mot
	hai.TrichYeu = "Một câu khác hẳn"

	_, err := SoSanhVanBan(daLuu, []VanBanNhiemVuVao{mot, hai})
	if !errors.Is(err, ErrVanBanTrungTrongYeuCau) {
		t.Fatalf("lỗi = %v, muốn ErrVanBanTrungTrongYeuCau — nếu không, bản ghi sau âm thầm đè bản trước", err)
	}
}

// TestSoSanhVanBan_DoiNhomMotDongDaLuuThiTuChoi: no specification describes moving a line between
// the three boxes, and doing it silently would also land the line on a number already taken in the
// destination group.
func TestSoSanhVanBan_DoiNhomMotDongDaLuuThiTuChoi(t *testing.T) {
	daLuu := baDongDaLuu()
	v := guiLai(daLuu[0])
	v.Nhom = VanBanSanPhamRa

	_, err := SoSanhVanBan(daLuu, []VanBanNhiemVuVao{v})
	if !errors.Is(err, ErrDoiNhomVanBan) {
		t.Fatalf("lỗi = %v, muốn ErrDoiNhomVanBan", err)
	}
}

// TestSoSanhVanBan_KhongKhaiNhomTrenDongDaLuuThiNhanLaGiuNguyen — the shape a client sends when it
// is only correcting the text.
func TestSoSanhVanBan_KhongKhaiNhomTrenDongDaLuuThiNhanLaGiuNguyen(t *testing.T) {
	daLuu := baDongDaLuu()
	v := VanBanNhiemVuVao{ID: "vb-9", TrichYeu: "Báo cáo của Ban Thường vụ Đảng uỷ (bản chính thức)"}

	td, err := SoSanhVanBan(daLuu, []VanBanNhiemVuVao{v})
	if err != nil {
		t.Fatalf("so sánh: %v", err)
	}
	if len(td.Sua) != 1 || td.Sua[0].Nhom != VanBanSanPhamRa || td.Sua[0].ThuTu != 1 {
		t.Fatalf("sua = %+v, muốn giữ nguyên nhóm `san-pham-dau-ra` và số thứ tự 1", td.Sua)
	}
}

// TestSoSanhVanBan_XoaSoKyHieuVaNgayLaMotThayDOI: clearing a structured field is an edit, not "the
// client did not send it". The block is replace-by-set, so an omitted value on a line that IS sent
// means the value is gone.
func TestSoSanhVanBan_XoaSoKyHieuVaNgayLaMotThayDoi(t *testing.T) {
	daLuu := baDongDaLuu()
	v := VanBanNhiemVuVao{ID: "vb-1", Nhom: VanBanCapTrenGiao, TrichYeu: daLuu[0].TrichYeu}

	td, err := SoSanhVanBan(daLuu, []VanBanNhiemVuVao{v, guiLai(daLuu[1]), guiLai(daLuu[2])})
	if err != nil {
		t.Fatalf("so sánh: %v", err)
	}
	if len(td.Sua) != 1 {
		t.Fatalf("sua=%d, muốn 1 — bỏ trống số ký hiệu và ngày LÀ một thay đổi", len(td.Sua))
	}
	if td.Sua[0].SoKyHieu != "" || !td.Sua[0].NgayVanBan.IsZero() {
		t.Errorf("so_ky_hieu=%q ngay_van_ban=%v, muốn rỗng cả hai",
			td.Sua[0].SoKyHieu, td.Sua[0].NgayVanBan)
	}
}

// --- minting the position ---------------------------------------------------------------------------

func TestThuTuVanBanTiepTheo(t *testing.T) {
	// ZERO MEANS THE GROUP HAS NEVER HELD A LINE, and 1 is the first number the schema's
	// `nhiem_vu_van_ban_thu_tu_tu_mot` admits.
	if got := ThuTuVanBanTiepTheo(0); got != 1 {
		t.Errorf("nhóm rỗng -> %d, muốn 1", got)
	}
	// THE HIGH-WATER MARK COUNTS REMOVED LINES, so a gap never closes: after ① ② ③ with ② and ③
	// removed, the next line is ④ and not ②.
	if got := ThuTuVanBanTiepTheo(3); got != 4 {
		t.Errorf("số lớn nhất 3 -> %d, muốn 4", got)
	}
}
