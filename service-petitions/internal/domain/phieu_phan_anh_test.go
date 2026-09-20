package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// These tests hold the rules ADR 0027 and ADR 0028 settled with the customer on 2026-09-20.
// They need no database, no network and no commune configuration, which is the point: the
// properties below are the ones a later session is most likely to "simplify" while believing it
// is obeying rule 10.

var (
	gui     = time.Date(2026, 9, 9, 7, 20, 0, 0, time.UTC)
	vaoSo   = time.Date(2026, 9, 9, 7, 20, 3, 0, time.UTC)
	hanNhan = time.Date(2026, 9, 9, 15, 20, 0, 0, time.UTC)
	hanXong = time.Date(2026, 9, 16, 9, 20, 0, 0, time.UTC)
)

// --- the nine statuses ---------------------------------------------------------------------

func TestChinTrangThaiVaKhongCoCaiThuMuoi(t *testing.T) {
	// The customer approved these nine strings VERBATIM. Changing one is now a migration of
	// archival records (rule 7), not a rename — so the strings themselves are pinned here, not
	// just the count.
	muon := []TrangThai{
		"da-tiep-nhan", "dang-phan-loai", "da-chuyen-xu-ly", "dang-xu-ly", "da-xu-ly",
		"cho-dan-xac-nhan", "da-dong", "khong-tiep-nhan", "chuyen-cap-tren",
	}
	if len(chuyenDuocSang) != len(muon) {
		t.Fatalf("có %d trạng thái, muốn %d — danh sách này ĐÓNG (ADR 0027)", len(chuyenDuocSang), len(muon))
	}
	for _, m := range muon {
		if !m.HopLe() {
			t.Errorf("thiếu trạng thái %q", m)
		}
	}
	for _, khong := range []TrangThai{"", "received", "screening", "da_tiep_nhan", "Da-Tiep-Nhan"} {
		if khong.HopLe() {
			// `received` and `screening` are the ENGLISH strings the specification carried before
			// 2026-09-20. They are a name that was REPLACED, not a second spelling still in use.
			t.Errorf("%q được nhận là hợp lệ — phải từ chối", khong)
		}
	}
}

func TestChuyenTrangThaiChiTheoDungDuongDaChot(t *testing.T) {
	duoc := []struct{ tu, den TrangThai }{
		{DaTiepNhan, DangPhanLoai},
		{DangPhanLoai, DaChuyenXuLy},
		{DangPhanLoai, KhongTiepNhan},
		{DangPhanLoai, ChuyenCapTren},
		{DaChuyenXuLy, DangXuLy},
		{DangXuLy, DaXuLy},
		{DaXuLy, ChoDanXacNhan},
		{ChoDanXacNhan, DaDong},
		{ChoDanXacNhan, DangXuLy}, // mở lại vì đánh giá thấp
		{DaDong, DangXuLy},        // mở lại vì đánh giá thấp
	}
	for _, c := range duoc {
		if !c.tu.ChuyenSangDuoc(c.den) {
			t.Errorf("%s -> %s bị từ chối, lẽ ra được", c.tu, c.den)
		}
	}

	khong := []struct {
		tu, den TrangThai
		vi      string
	}{
		{DaTiepNhan, KhongTiepNhan,
			"từ chối một phiếu CHƯA ai đọc — hai nhánh rẽ chỉ rời khỏi dang-phan-loai (09 §6)"},
		{DaTiepNhan, DaChuyenXuLy,
			"bỏ qua bước phân loại là bỏ qua đúng hành vi ấn định han_xu_ly_xong (ADR 0028)"},
		{DangXuLy, KhongTiepNhan,
			"từ chối một phiếu đã phân công và đã xử lý là một vòng đời khác, luật 10 điều kiện dừng #2"},
		{KhongTiepNhan, DangPhanLoai, "khong-tiep-nhan là trạng thái kết thúc"},
		{ChuyenCapTren, DangXuLy, "chuyen-cap-tren là trạng thái kết thúc"},
		{DaDong, DaDong, "đóng lại một phiếu đã đóng"},
		{"received", DangPhanLoai, "mã đã bị thay, không phải cách gọi thứ hai"},
	}
	for _, c := range khong {
		if c.tu.ChuyenSangDuoc(c.den) {
			t.Errorf("%s -> %s được nhận, lẽ ra từ chối: %s", c.tu, c.den, c.vi)
		}
	}
}

func TestHaiNhanhReLaTrangThaiKetThuc(t *testing.T) {
	for _, t2 := range []TrangThai{KhongTiepNhan, ChuyenCapTren} {
		if !t2.KetThuc() {
			t.Errorf("%s không phải trạng thái kết thúc", t2)
		}
	}
	// And a status in the main flow must NOT be terminal: a petition with no way out is a
	// commitment to a citizen that stops moving, silently, forever.
	for _, t2 := range []TrangThai{DaTiepNhan, DangPhanLoai, DaChuyenXuLy, DangXuLy, DaXuLy, ChoDanXacNhan, DaDong} {
		if t2.KetThuc() {
			t.Errorf("%s bị coi là kết thúc — phiếu rơi vào đó nằm lại vĩnh viễn", t2)
		}
	}
}

// --- the channels ------------------------------------------------------------------------------

func TestKenhNaoChotLinhVucLucVaoSo(t *testing.T) {
	// The rule is written by FORM SHAPE: only the staff-booked modal carries the field at
	// booking (docs/ui-ux/09 §11), so only it fixes both deadlines at once.
	if !KenhCanBoNhapHo.CoLinhVucLucVaoSo() {
		t.Error("kênh nhập hộ phải chốt lĩnh vực lúc vào sổ — biểu mẫu bắt buộc chọn lĩnh vực")
	}
	for _, k := range []KenhTiepNhan{KenhZaloMiniApp, KenhZaloOA, KenhWebXa} {
		if k.CoLinhVucLucVaoSo() {
			t.Errorf("kênh %s chốt lĩnh vực lúc vào sổ — dựng lại trần 56 giờ mà ADR 0028 vừa tháo", k)
		}
	}
}

func TestChiKenhNhapHoMoiKhongApDungHanTiepNhan(t *testing.T) {
	if KenhCanBoNhapHo.HanTiepNhanApDung() {
		t.Error("kênh nhập hộ áp dụng hạn tiếp nhận — chính cán bộ là người đọc, khoảng ấy không tồn tại")
	}
	for _, k := range []KenhTiepNhan{KenhZaloMiniApp, KenhZaloOA, KenhWebXa} {
		if !k.HanTiepNhanApDung() {
			t.Errorf("kênh %s bỏ hạn tiếp nhận — đó là đồng hồ duy nhất canh khoảng chờ phân loại", k)
		}
	}
}

// --- the two clocks, derived -------------------------------------------------------------------

func TestQuaHanSuyRaTuHanDaLuu(t *testing.T) {
	goc := PhieuPhanAnh{
		Kenh: KenhZaloMiniApp, TrangThai: DangPhanLoai,
		GocDemHan: gui, VaoSoLuc: vaoSo,
		HanTiepNhan: hanNhan, HanXuLyXong: hanXong,
	}

	truoc := hanXong.Add(-time.Minute)
	sau := hanXong.Add(time.Minute)

	if goc.QuaHan(truoc) {
		t.Error("quá hạn trước khi tới hạn")
	}
	if !goc.QuaHan(sau) {
		t.Error("không quá hạn sau khi đã qua hạn")
	}

	t.Run("xong trước hạn thì mãi mãi đúng hạn", func(t *testing.T) {
		p := goc
		p.XuLyXongLuc = truoc
		// A year later, still not overdue: the answer stops depending on `now` once the work is
		// recorded as done, so last quarter's figures do not change every time somebody opens the
		// screen.
		if p.QuaHan(sau.AddDate(1, 0, 0)) {
			t.Error("phiếu xong trước hạn lại thành quá hạn khi thời gian trôi")
		}
	})

	t.Run("xong sau hạn thì mãi mãi trễ", func(t *testing.T) {
		p := goc
		p.XuLyXongLuc = sau
		if !p.QuaHan(sau) {
			t.Error("phiếu xong muộn không còn bị tính là trễ — công việc trễ lặng lẽ hết trễ khi làm xong")
		}
	})

	t.Run("chưa có hạn xử lý thì KHÔNG quá hạn", func(t *testing.T) {
		p := goc
		p.HanXuLyXong = time.Time{}
		// A statement about the COMMITMENT, not about the work: nothing was promised, so nothing
		// was missed. What is running meanwhile is the acknowledge clock.
		if p.QuaHan(sau.AddDate(1, 0, 0)) {
			t.Error("phiếu chưa phân loại bị tính quá hạn — chưa ai hứa với dân một ngày nào")
		}
	})
}

func TestQuaHanTiepNhanDungLaiKhiCoNguoiDongVao(t *testing.T) {
	goc := PhieuPhanAnh{
		Kenh: KenhZaloMiniApp, TrangThai: DaTiepNhan,
		GocDemHan: gui, VaoSoLuc: vaoSo, HanTiepNhan: hanNhan,
	}

	if !goc.QuaHanTiepNhan(hanNhan.Add(time.Minute)) {
		t.Error("phiếu chưa ai đọc, đã quá hạn tiếp nhận, mà không bị tính")
	}

	t.Run("dừng ở hành vi của con người, không ở lúc sinh phiếu", func(t *testing.T) {
		p := goc
		p.PhanLoaiLuc = hanNhan.Add(-time.Hour)
		if p.QuaHanTiepNhan(hanNhan.AddDate(0, 1, 0)) {
			t.Error("đồng hồ tiếp nhận vẫn chạy sau khi cán bộ đã phân loại")
		}
		// And measuring from the send to the INSERT would be measuring the software against
		// itself: VaoSoLuc is three seconds after GocDemHan in every fixture here, so an
		// implementation that stopped the clock there would score every commune perfectly.
		p.PhanLoaiLuc = hanNhan.Add(time.Hour)
		if !p.QuaHanTiepNhan(hanNhan.AddDate(0, 1, 0)) {
			t.Error("cán bộ đọc phiếu MUỘN hơn hạn mà không bị tính trễ tiếp nhận")
		}
	})

	t.Run("phiếu nhập hộ không bao giờ trễ tiếp nhận", func(t *testing.T) {
		p := PhieuPhanAnh{Kenh: KenhCanBoNhapHo, GocDemHan: gui, VaoSoLuc: vaoSo}
		if !p.HanTiepNhanKhongApDung() {
			t.Fatal("hạn tiếp nhận của phiếu nhập hộ phải là KHÔNG ÁP DỤNG")
		}
		if p.QuaHanTiepNhan(vaoSo.AddDate(1, 0, 0)) {
			t.Error("phiếu nhập hộ bị tính trễ tiếp nhận — khoảng 'bao lâu thì có người đọc' không tồn tại ở kênh này")
		}
	})
}

// TestHaiNullNguocNghiaNhau is the property the whole design turns on, asserted where it is
// cheapest: two petitions, two NULL deadlines, two opposite meanings.
func TestHaiNullNguocNghiaNhau(t *testing.T) {
	nhapHo := PhieuPhanAnh{Kenh: KenhCanBoNhapHo, LinhVuc: "dien",
		GocDemHan: gui, VaoSoLuc: vaoSo, HanXuLyXong: hanXong}
	chuaPhanLoai := PhieuPhanAnh{Kenh: KenhZaloMiniApp,
		GocDemHan: gui, VaoSoLuc: vaoSo, HanTiepNhan: hanNhan}

	if !nhapHo.HanTiepNhanKhongApDung() || nhapHo.ChuaChotHanXuLy() {
		t.Error("phiếu nhập hộ: hạn tiếp nhận phải KHÔNG ÁP DỤNG, hạn xử lý phải ĐÃ CÓ")
	}
	if chuaPhanLoai.HanTiepNhanKhongApDung() || !chuaPhanLoai.ChuaChotHanXuLy() {
		t.Error("phiếu chưa phân loại: hạn tiếp nhận phải ĐANG CHẠY, hạn xử lý phải CHƯA CÓ")
	}
}

// --- the deadline only ever shortens -------------------------------------------------------------

func TestHanSomHonChiRutNganKhongBaoGioKeoDai(t *testing.T) {
	som := hanXong.Add(-48 * time.Hour)
	muon := hanXong.Add(48 * time.Hour)

	if got := HanSomHon(hanXong, som); !got.Equal(som) {
		t.Errorf("đổi sang lĩnh vực gấp hơn: hạn = %v, muốn %v", got, som)
	}
	if got := HanSomHon(hanXong, muon); !got.Equal(hanXong) {
		// The customer's own reason: the deadline is a thing already SAID to a citizen who holds
		// a lookup code and can read it back.
		t.Errorf("đổi sang lĩnh vực dài hơn lại KÉO DÀI hạn: %v — luật đã hứa là %v", got, hanXong)
	}

	t.Run("lần chốt ĐẦU TIÊN là ấn định, không phải đổi", func(t *testing.T) {
		// daHua is zero: nothing has been promised. Taking the min against a zero time.Time would
		// return year 1 — a petition overdue the instant it was classified — which is the shape
		// that silently re-creates the 56-hour ceiling ADR 0028 removed.
		if got := HanSomHon(time.Time{}, hanXong); !got.Equal(hanXong) {
			t.Errorf("ấn định lần đầu = %v, muốn %v", got, hanXong)
		}
	})
}

// --- the seven-day window --------------------------------------------------------------------

func TestGocDemHanChanHaiDauVaTuChoiChuKhongCatVeBien(t *testing.T) {
	ok := []struct {
		ten string
		goc time.Time
	}{
		{"đúng lúc vào sổ", vaoSo},
		{"ba ngày trước", vaoSo.AddDate(0, 0, -3)},
		{"đúng bảy ngày trước", vaoSo.AddDate(0, 0, -SoNgayNhoLai)},
	}
	for _, c := range ok {
		if err := KiemGocDemHan(c.goc, vaoSo); err != nil {
			t.Errorf("%s: bị từ chối (%v)", c.ten, err)
		}
	}

	for _, c := range []struct {
		ten string
		goc time.Time
	}{
		{"muộn hơn lúc vào sổ", vaoSo.Add(time.Second)},
		{"tám ngày trước", vaoSo.AddDate(0, 0, -8)},
		{"rỗng", time.Time{}},
	} {
		err := KiemGocDemHan(c.goc, vaoSo)
		if err == nil {
			t.Errorf("%s: được nhận — một mốc quá khứ tuỳ ý là quyền chế ra một phiếu đã quá hạn cho người khác", c.ten)
			continue
		}
		if !errors.Is(err, ErrGocDemHanNgoaiKhoang) {
			t.Errorf("%s: lỗi %v không bọc ErrGocDemHanNgoaiKhoang", c.ten, err)
		}
	}
}

// --- the lookup code ----------------------------------------------------------------------------

func TestMaTraCuuKhongDoanDuoc(t *testing.T) {
	const soLan = 2000

	daThay := make(map[string]bool, soLan)
	for i := 0; i < soLan; i++ {
		ma, err := SinhMaTraCuu()
		if err != nil {
			t.Fatalf("sinh mã: %v", err)
		}
		if daThay[ma] {
			t.Fatalf("sinh trùng mã %q trong %d lần — mã đã cấp không bao giờ cấp lại (luật 7 bất biến 3)", ma, soLan)
		}
		daThay[ma] = true

		if !strings.HasPrefix(ma, TienToMaTraCuu+"-") {
			t.Fatalf("mã %q không mang tiền tố %q", ma, TienToMaTraCuu)
		}

		than := strings.TrimPrefix(ma, TienToMaTraCuu+"-")
		nhom := strings.Split(than, "-")
		if len(nhom) != 3 {
			t.Fatalf("mã %q không chia thành ba nhóm — người đọc qua điện thoại sẽ lạc chỗ", ma)
		}
		for _, n := range nhom {
			if len(n) != 4 {
				t.Fatalf("mã %q có nhóm %q dài %d, muốn 4", ma, n, len(n))
			}
		}

		// THE SIX MISSING CHARACTERS ARE THE POINT. `0`/`O` and `1`/`I`/`L` in one code produce a
		// lookup failure the citizen reads as "the commune lost my report".
		for _, r := range than {
			if r == '-' {
				continue
			}
			if !strings.ContainsRune(chuCaiMaTraCuu, r) {
				t.Fatalf("mã %q chứa ký tự %q ngoài bảng chữ cái — dễ đọc nhầm", ma, string(r))
			}
		}
	}
}

func TestMaTraCuuKhongTuanTu(t *testing.T) {
	// A sequential code on a lookup path reads every other citizen's petition one increment at a
	// time (rule 4, invariant 4). Two consecutive codes sharing a long prefix is the cheapest
	// observable symptom of that.
	a, err := SinhMaTraCuu()
	if err != nil {
		t.Fatal(err)
	}
	b, err := SinhMaTraCuu()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("hai mã liên tiếp giống hệt nhau")
	}
	chung := 0
	for i := 0; i < len(a) && i < len(b) && a[i] == b[i]; i++ {
		chung++
	}
	// `PA-` is three shared characters by construction; anything much beyond that suggests a
	// counter rather than randomness.
	if chung > 6 {
		t.Errorf("hai mã liên tiếp trùng %d ký tự đầu (%q, %q) — trông như đếm tăng dần", chung, a, b)
	}
}

// TestMaTraCuuPhanBoDeu is the assertion that catches `b % 30` used WITHOUT rejection sampling.
//
// THE FIRST VERSION OF THIS TEST DID NOT CATCH IT, and the reason is worth keeping, because it is
// the trap the repository has already been bitten by eight times: a check that looks like it is
// guarding something while being unable to fail. It compared each character's count against a
// ±25% band. The bias is structural but SMALL — 256 = 8×30 + 16, so the first sixteen characters
// of the alphabet come up with probability 9/256 and the last fourteen with 8/256, which is only
// +5.5% and −6.25% around uniform. Every character sat comfortably inside ±25%, the mutation
// survived, and the test printed `ok`.
//
// WHAT ACTUALLY DETECTS IT is the SHAPE of the bias rather than its size: it always favours
// exactly the first `256 mod len(alphabet)` characters, so the two groups can be compared with
// each other instead of each character against an average. The gap between the group means is
// then ~11.8%, roughly thirteen standard deviations at this sample size, while the band below is
// four. A biased generator cannot pass; an unbiased one cannot flake.
//
// WHY IT MATTERS AT ALL: the bias shrinks the effective search space of every lookup code the
// system will ever issue, and it is invisible by eye — biased codes look exactly as random as
// unbiased ones.
func TestMaTraCuuPhanBoDeu(t *testing.T) {
	const soMa = 6000

	dem := map[rune]int{}
	tong := 0
	for i := 0; i < soMa; i++ {
		ma, err := SinhMaTraCuu()
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range strings.TrimPrefix(ma, TienToMaTraCuu+"-") {
			if r == '-' {
				continue
			}
			dem[r]++
			tong++
		}
	}
	if len(dem) != len(chuCaiMaTraCuu) {
		t.Fatalf("chỉ %d/%d ký tự từng xuất hiện trong %d mã", len(dem), len(chuCaiMaTraCuu), soMa)
	}

	// The split a modulo bias would produce, computed from the alphabet rather than written as a
	// number: if the alphabet ever changes length, the test keeps pointing at the right group.
	const soGiaTriByte = 256
	duocUuTien := soGiaTriByte % len(chuCaiMaTraCuu)
	if duocUuTien == 0 {
		t.Skip("bảng chữ cái chia hết 256 — phép modulo không còn lệch, phép kiểm này không còn nghĩa")
	}

	var tongUuTien, tongConLai int
	for i, r := range chuCaiMaTraCuu {
		if i < duocUuTien {
			tongUuTien += dem[r]
		} else {
			tongConLai += dem[r]
		}
	}
	trungBinhUuTien := float64(tongUuTien) / float64(duocUuTien)
	trungBinhConLai := float64(tongConLai) / float64(len(chuCaiMaTraCuu)-duocUuTien)

	ty := trungBinhUuTien / trungBinhConLai
	if ty < 0.96 || ty > 1.04 {
		t.Errorf("%d ký tự đầu bảng xuất hiện trung bình %.1f lần, %d ký tự còn lại %.1f lần (tỷ lệ %.4f) — "+
			"đúng hình dạng lệch của phép %% khi thiếu rejection sampling",
			duocUuTien, trungBinhUuTien, len(chuCaiMaTraCuu)-duocUuTien, trungBinhConLai, ty)
	}

	// A loose per-character band as well, for a different failure: a generator that never emits
	// one character at all, which the group ratio above could average away.
	mongDoi := float64(tong) / float64(len(chuCaiMaTraCuu))
	for r, n := range dem {
		if float64(n) < mongDoi*0.75 || float64(n) > mongDoi*1.25 {
			t.Errorf("ký tự %q xuất hiện %d lần, mong đợi ~%.0f", string(r), n, mongDoi)
		}
	}
}
