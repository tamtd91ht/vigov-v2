package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// WHAT THIS FILE IS FOR: the arithmetic and the refusals of the budget board, checked against
// docs/ui-ux/07-thu-chi-ngan-sach.md's OWN SAMPLE FIGURES rather than against numbers invented here.
//
// A test written from made-up numbers proves the code agrees with itself. The specification prints
// 108,1% and 91,3% beside the inputs that produce them (§3.1, §3.2, §10), so those are the figures
// below — and ADR 0035 §A turns on one of them being reachable only from one of the two candidate
// denominators.
//
// THE UNIT. Every stored figure is in ĐỒNG (migration 0006's MONEY block); the sheet DISPLAYS
// "Triệu đồng". `trieu` converts one printed figure to what is stored, so a reader can compare the
// constants below with the specification line by line.

// trieu turns a figure printed in triệu đồng — with at most one decimal, as §3 prints them — into
// đồng. TAKEN AS TENTHS rather than as a float: 3_463_459.2 cannot be written exactly in binary
// floating point, and the whole point of Dong being an integer is that no figure on this screen ever
// passes through one.
func trieu(phanMuoi int64) Dong { return Dong(phanMuoi * 100_000) }

const (
	// §3.1, the chi tab.
	chiDuToanNam  = 37_947_400 // 3.794.740,0
	chiNganSachSo = 34_634_592 // 3.463.459,2
	// §3.2, the thu tab.
	thuDuToanTPGiao = 39_930_100 // 3.993.010,0
	thuDuToanXaGiao = 46_812_900 // 4.681.290,0
	thuNSNNSo       = 43_167_643 // 4.316.764,3
	thuXaHuongSo    = 33_008_005 // 3.300.800,5
)

// --- fixtures -------------------------------------------------------------------------------------

// bangChiMau is the chi sheet of §3.1 and §10, cut down to what the figures need: `Tổng số` as the
// MARKED total row, and `A. CHI NGÂN SÁCH NHÀ NƯỚC` beside it as a SIBLING.
//
// THE SIBLING IS THE WHOLE POINT OF THE FIXTURE, not decoration. §10 describes the chi sheet as
// `Tổng số` → `A. CHI NGÂN SÁCH NHÀ NƯỚC`, and `../vigov-require` measured that those two are in
// fact at the SAME level: adding every top-level row counts the same money twice. A fixture with one
// top-level row could not tell a marked-row lookup from "sum the roots" or from "take the first one".
func bangChiMau() BangDayDu {
	return BangDayDu{
		Bang: BangNganSach{ID: "b-chi", Ma: "NS-2026-CHI-01", Nam: 2026, Loai: BangChi, Lan: 1},
		Cot: []CotNganSach{
			{ID: "c-dt", BangID: "b-chi", Ten: "Dự toán năm", ThuTu: 1, Kieu: CotSo, VaiTro: VaiTroDuToanNam},
			{ID: "c-chi", BangID: "b-chi", Ten: "Chi ngân sách", ThuTu: 2, Kieu: CotSo, VaiTro: VaiTroChiNganSach},
			{ID: "c-ty", BangID: "b-chi", Ten: "So sánh TH/DT (%)", ThuTu: 3, Kieu: CotPhanTram,
				CongThuc: "col_2 / col_1 * 100"},
		},
		KhoanMuc: []KhoanMucNganSach{
			{ID: "k-tong", BangID: "b-chi", TT: "", Ten: "Tổng số", ThuTu: 1,
				CachTinh: TinhTay, LaDongTong: true},
			{ID: "k-a", BangID: "b-chi", TT: "A", Ten: "CHI NGÂN SÁCH NHÀ NƯỚC", ThuTu: 2,
				CachTinh: TinhTay},
		},
		Gia: map[string]map[string]Dong{
			"k-tong": {"c-dt": trieu(chiDuToanNam), "c-chi": trieu(chiNganSachSo)},
			// A. carries FIGURES OF ITS OWN, larger than the total row's (§4.1 prints 5.502.660 /
			// 3.401.673,3 for it). Anything that summed top-level rows would produce a figure larger
			// than the sheet's own total — which is exactly the double count being guarded against.
			"k-a": {"c-dt": trieu(55_026_600), "c-chi": trieu(34_016_733)},
		},
	}
}

// bangThuMau is the thu sheet of §3.2 and §10: `A. TỔNG THU NỘI ĐỊA PHÁT SINH TRÊN ĐỊA BÀN` with
// `B. THU NGÂN SÁCH ĐỊA PHƯƠNG` NESTED under it — the second shape `../vigov-require` measured.
//
// THE MARKED ROW IS `A`, and all four columns of §3.2 are present because ADR 0035 turns on the
// DIFFERENCE between two of them: `Thu ngân sách NSNN` (4.316.764,3) and `Thu xã hưởng`
// (3.300.800,5). A fixture carrying only one could not tell #32 from its opposite.
func bangThuMau() BangDayDu {
	return BangDayDu{
		Bang: BangNganSach{ID: "b-thu", Ma: "NS-2026-THU-01", Nam: 2026, Loai: BangThu, Lan: 1},
		Cot: []CotNganSach{
			{ID: "c-tp", BangID: "b-thu", Ten: "Dự toán 2026 TP giao", ThuTu: 1, Kieu: CotSo,
				VaiTro: VaiTroDuToanTPGiao},
			{ID: "c-xa", BangID: "b-thu", Ten: "Dự toán 2026 Xã giao", ThuTu: 2, Kieu: CotSo,
				VaiTro: VaiTroDuToanXaGiao},
			{ID: "c-nsnn", BangID: "b-thu", Ten: "Thu ngân sách NSNN", ThuTu: 3, Kieu: CotSo,
				VaiTro: VaiTroThuNSNN},
			{ID: "c-huong", BangID: "b-thu", Ten: "Thu ngân sách Thu xã hưởng", ThuTu: 4, Kieu: CotSo,
				VaiTro: VaiTroThuXaHuong},
		},
		KhoanMuc: []KhoanMucNganSach{
			{ID: "k-a", BangID: "b-thu", TT: "A", Ten: "TỔNG THU NỘI ĐỊA PHÁT SINH TRÊN ĐỊA BÀN",
				ThuTu: 1, CachTinh: TinhTay, LaDongTong: true},
		},
		Gia: map[string]map[string]Dong{
			"k-a": {
				"c-tp":    trieu(thuDuToanTPGiao),
				"c-xa":    trieu(thuDuToanXaGiao),
				"c-nsnn":  trieu(thuNSNNSo),
				"c-huong": trieu(thuXaHuongSo),
			},
		},
	}
}

// --- (1) the marked row, and nothing else ----------------------------------------------------------

func TestDongTongLayDongDuocDanhDauChuKhongPhaiDongDau(t *testing.T) {
	// THE ASSERTION THAT MATTERS IS THE SECOND ONE. The chi fixture marks `Tổng số`, which happens to
	// be first; the thu fixture below marks a row that is not, and the mutation this file is written
	// against is "take the first row" — see TestDongTongKhongSuyTheoThuTuDong.
	b := bangChiMau()
	dong, err := b.DongTong()
	if err != nil {
		t.Fatalf("DongTong lỗi: %v", err)
	}
	if dong.ID != "k-tong" {
		t.Fatalf("dòng tổng = %q, muốn %q", dong.ID, "k-tong")
	}
}

func TestDongTongKhongSuyTheoThuTuDong(t *testing.T) {
	// THE MUTATION GUARD. `is_headline` exists because "mặc định dòng đầu tiên" (§5 rule 5) is right
	// on one form and wrong on the next, with nothing reporting it. Here the FIRST row is not the
	// marked one, so any implementation that falls back to ordering — first row, lowest `thu_tu`,
	// shallowest `cap` — answers `k-a` and this turns red.
	b := bangChiMau()
	b.KhoanMuc[0].LaDongTong = false
	b.KhoanMuc[1].LaDongTong = true

	dong, err := b.DongTong()
	if err != nil {
		t.Fatalf("DongTong lỗi: %v", err)
	}
	if dong.ID != "k-a" {
		t.Fatalf("dòng tổng = %q, muốn %q — dòng tổng phải là dòng ĐƯỢC ĐÁNH DẤU, "+
			"không phải dòng đầu tiên/nông nhất", dong.ID, "k-a")
	}
}

func TestChuaDanhDauDongNaoThiKHONGCoSoTongVaCoMotCAU(t *testing.T) {
	// ADR 0035 §A: "chưa đủ dữ liệu để ra số thì để trống KÈM LÝ DO, không đặt mặc định". Answering 0
	// here would put a zero into a card leadership reads.
	b := bangChiMau()
	for i := range b.KhoanMuc {
		b.KhoanMuc[i].LaDongTong = false
	}

	if _, err := b.DongTong(); !errors.Is(err, ErrChuaDanhDauDongTong) {
		t.Fatalf("DongTong = %v, muốn ErrChuaDanhDauDongTong", err)
	}

	ten, ty := ChiSoDatDuToan(b)
	if ty.Co {
		t.Fatalf("%s vẫn ra số %d khi chưa đánh dấu dòng tổng", ten, ty.Gia)
	}
	if ty.Gia != 0 || ty.LyDo == "" {
		t.Fatalf("chỉ số phải KHÔNG có giá trị và PHẢI có lý do; nhận Gia=%d LyDo=%q", ty.Gia, ty.LyDo)
	}
	if !strings.Contains(ty.LyDo, "ngôi sao") {
		t.Errorf("lý do không nói cho xã phải làm gì: %q", ty.LyDo)
	}
}

func TestHaiDongCungDanhDauThiTUCHOI_khongCongVaKhongChonBua(t *testing.T) {
	// THE DOUBLE COUNT, STATED AS A TEST. `../vigov-require` measured that the thu sheet has two
	// nested top-level rows and the chi sheet has `Tổng số` beside A…E; summing marked rows, or
	// silently taking the first of them, is exactly the failure `is_headline` was introduced to end.
	//
	// The application makes the mark a RADIO so this state is unreachable through any route here.
	// It is asserted anyway: the database carries no partial unique index for it (migration 0006 says
	// why it cannot be verified in this environment), so the state can arrive from an import or a
	// psql session, and the read path must refuse rather than answer plausibly.
	b := bangChiMau()
	b.KhoanMuc[1].LaDongTong = true // now BOTH are marked

	_, err := b.DongTong()
	if !errors.Is(err, ErrNhieuDongTong) {
		t.Fatalf("DongTong = %v, muốn ErrNhieuDongTong", err)
	}

	ten, ty := ChiSoDatDuToan(b)
	if ty.Co {
		t.Fatalf("%s vẫn ra số %d khi có hai dòng cùng đánh dấu — đó đúng là phép ĐẾM ĐÔI", ten, ty.Gia)
	}
	if !strings.Contains(ty.LyDo, "đếm đôi") {
		t.Errorf("lý do không nói ra vì sao từ chối: %q", ty.LyDo)
	}
}

// --- (2) the two figures ADR 0035 §A fixed -----------------------------------------------------------

func TestThuDatDuToanChiaChoDuToanTPGiao(t *testing.T) {
	// ADR 0035 #33. The specification contradicts itself: the summary cells of §3.2 show 108,1% and
	// the prose at §9 rule 6 says to divide by `Dự toán Xã giao`, which gives 92,2%. The ADR settles
	// it on the city target — the figure the commune is judged against and the only one comparable
	// between communes.
	//
	// BOTH NUMBERS ARE ASSERTED, the wanted one and the rejected one, because asserting only "10811"
	// would stay green the day somebody made the two denominators the same column.
	ten, ty := ChiSoDatDuToan(bangThuMau())
	if ten != "Thu đạt dự toán" {
		t.Fatalf("tên chỉ số = %q", ten)
	}
	if !ty.Co {
		t.Fatalf("không ra số: %s", ty.LyDo)
	}
	const muon = PhanVan(10811) // 108,11% — §3.2 prints 108,1%
	if ty.Gia != muon {
		t.Fatalf("Thu đạt dự toán = %d phần vạn, muốn %d", ty.Gia, muon)
	}
	const neuChiaChoXaGiao = PhanVan(9221) // 92,21% — the figure §9 rule 6 would produce
	if ty.Gia == neuChiaChoXaGiao {
		t.Fatal("đang chia cho `Dự toán Xã giao` — ADR 0035 #33 chốt `Dự toán TP giao`")
	}
}

func TestChiDatDuToanChiaChoDuToanNam(t *testing.T) {
	ten, ty := ChiSoDatDuToan(bangChiMau())
	if ten != "Chi đạt dự toán" {
		t.Fatalf("tên chỉ số = %q", ten)
	}
	if !ty.Co {
		t.Fatalf("không ra số: %s", ty.LyDo)
	}
	const muon = PhanVan(9127) // 91,27% — §3.1 prints 91,3%
	if ty.Gia != muon {
		t.Fatalf("Chi đạt dự toán = %d phần vạn, muốn %d", ty.Gia, muon)
	}
}

func TestCanDoiLayThuXaHuongChuKhongLayThuNSNN(t *testing.T) {
	// ADR 0035 #32, AND THE SIGN IS THE ASSERTION. On the specification's own figures the commune is
	// SHORT: 3.300.800,5 of retained revenue against 3.463.459,2 of expenditure. Built on
	// `Thu ngân sách NSNN` (4.316.764,3) the same cell would read as a surplus of +853.305,1 — a
	// commune told it has money it may not spend.
	kq := CanDoiThuChi(bangThuMau(), bangChiMau())
	if !kq.Co {
		t.Fatalf("không ra số: %s", kq.LyDo)
	}
	muon := trieu(thuXaHuongSo) - trieu(chiNganSachSo)
	if kq.Gia != muon {
		t.Fatalf("Cân đối = %d đồng, muốn %d", kq.Gia, muon)
	}
	if kq.Gia >= 0 {
		t.Fatalf("Cân đối = %d, phải ÂM trên số liệu mẫu — dấu dương nghĩa là đang lấy "+
			"`Thu ngân sách NSNN` chứ không phải `Thu xã hưởng` (ADR 0035 #32)", kq.Gia)
	}
	neuLayNSNN := trieu(thuNSNNSo) - trieu(chiNganSachSo)
	if kq.Gia == neuLayNSNN {
		t.Fatal("Cân đối đang lấy `Thu ngân sách NSNN` — ADR 0035 #32 chốt `Thu xã hưởng`")
	}
}

func TestCotDuToanTPGiaoTrongThiChiSoBienMatKemMotCAU(t *testing.T) {
	// ADR 0035 §A's mandatory consequence: `Dự toán TP giao` becomes a column that MUST have data,
	// and a commune that leaves it out loses the indicator — "và điều đó phải hiện ra thành một CÂU,
	// không thành một ô trống". Never 0, never NaN.
	//
	// BOTH HALVES OF "trống" ARE COVERED: the column is not declared at all, and the column exists but
	// the marked row has no figure in it. They need two different acts from the commune, so they get
	// two different sentences.
	t.Run("không có cột nào mang vai trò", func(t *testing.T) {
		b := bangThuMau()
		b.Cot = b.Cot[1:] // drop `Dự toán 2026 TP giao`

		ten, ty := ChiSoDatDuToan(b)
		if ty.Co {
			t.Fatalf("%s vẫn ra %d khi không có cột `Dự toán TP giao`", ten, ty.Gia)
		}
		if !errors.Is(chiSoLoi(b), ErrChuaGanVaiTroCot) {
			t.Errorf("lý do không phải ErrChuaGanVaiTroCot: %q", ty.LyDo)
		}
	})

	t.Run("có cột nhưng dòng tổng chưa có số", func(t *testing.T) {
		b := bangThuMau()
		delete(b.Gia["k-a"], "c-tp")

		ten, ty := ChiSoDatDuToan(b)
		if ty.Co {
			t.Fatalf("%s vẫn ra %d khi dòng tổng bỏ trống cột `Dự toán TP giao`", ten, ty.Gia)
		}
		if !errors.Is(chiSoLoi(b), ErrDongTongChuaCoSo) {
			t.Errorf("lý do không phải ErrDongTongChuaCoSo: %q", ty.LyDo)
		}
	})
}

// chiSoLoi re-runs the lookup the indicator performs, so a test can compare against the SENTINEL
// rather than against the rendered sentence. Comparing strings is a test that breaks when somebody
// fixes a typo and stays green when somebody changes the rule.
func chiSoLoi(b BangDayDu) error {
	var mau VaiTroCot
	if b.Bang.Loai == BangThu {
		mau = VaiTroDuToanTPGiao
	} else {
		mau = VaiTroDuToanNam
	}
	_, _, err := b.SoTong(mau)
	return err
}

func TestMauSoBangKhongThiKhongCoTyLe(t *testing.T) {
	// §9 rule 3 for a percentage column: denominator 0 ⇒ shown BLANK. The same holds for an indicator
	// — a division by zero is not 0%, not 100%, and not infinity; it is an absent measurement.
	b := bangThuMau()
	b.Gia["k-a"]["c-tp"] = 0

	_, ty := ChiSoDatDuToan(b)
	if ty.Co {
		t.Fatalf("mẫu số 0 mà vẫn ra %d phần vạn", ty.Gia)
	}
	if !strings.Contains(ty.LyDo, "mẫu số") {
		t.Errorf("lý do không nói mẫu số bằng 0: %q", ty.LyDo)
	}
}

func TestThieuHanMotBangThiCanDoiKhongRaSo(t *testing.T) {
	// "Xã chưa nhập bảng chi" and "xã chi 0 đồng" are different statements and only one of them is
	// ever true. A zero here is the second statement made on the evidence of the first.
	kq := CanDoiThuChi(bangThuMau(), BangDayDu{})
	if kq.Co {
		t.Fatalf("thiếu bảng chi mà Cân đối vẫn ra %d", kq.Gia)
	}
	if kq.LyDo == "" {
		t.Fatal("thiếu bảng chi mà không có lý do nào hiện ra")
	}
}

// --- (3) the tree, and the parent that is never typed into ---------------------------------------------

func TestKhoanMucChaCongTuDongCon_chiConTrucTiep(t *testing.T) {
	// §9 rule 2: a `children` line sums its DIRECT children only. Each child may itself be a
	// `children` line that has already summed its own, so reaching down to the grandchildren counts
	// every level below twice.
	b := BangDayDu{
		Bang: BangNganSach{ID: "b", Loai: BangChi},
		Cot:  []CotNganSach{{ID: "c", BangID: "b", Ten: "Chi ngân sách", Kieu: CotSo}},
		KhoanMuc: []KhoanMucNganSach{
			{ID: "ong", BangID: "b", Ten: "A", ThuTu: 1},
			{ID: "cha", BangID: "b", ChaID: "ong", Ten: "I", ThuTu: 2, Cap: 1},
			{ID: "con1", BangID: "b", ChaID: "cha", Ten: "1.1", ThuTu: 3, Cap: 2},
			{ID: "con2", BangID: "b", ChaID: "cha", Ten: "1.2", ThuTu: 4, Cap: 2},
		},
		Gia: map[string]map[string]Dong{
			"con1": {"c": 30},
			"con2": {"c": 12},
		},
	}

	if g, co := b.GiaTri("cha", "c"); !co || g != 42 {
		t.Fatalf("cha = (%d,%v), muốn (42,true)", g, co)
	}
	// The grandparent has ONE direct child, whose value is 42. A walk that also added the
	// grandchildren would produce 84.
	if g, co := b.GiaTri("ong", "c"); !co || g != 42 {
		t.Fatalf("ông = (%d,%v), muốn (42,true) — 84 nghĩa là đang cộng cả cháu", g, co)
	}
}

func TestChaKhongCoConNaoCoSoThiTRONGChuKhongPhaiKhongDong(t *testing.T) {
	// §9 rule 4: empty shows `—`, never `0`. A section the commune has not entered yet must not be
	// reported as a section it spent nothing on.
	b := BangDayDu{
		Bang: BangNganSach{ID: "b", Loai: BangChi},
		Cot:  []CotNganSach{{ID: "c", BangID: "b", Kieu: CotSo}},
		KhoanMuc: []KhoanMucNganSach{
			{ID: "cha", BangID: "b", Ten: "I", ThuTu: 1},
			{ID: "con", BangID: "b", ChaID: "cha", Ten: "1.1", ThuTu: 2, Cap: 1},
		},
		Gia: map[string]map[string]Dong{},
	}
	if g, co := b.GiaTri("cha", "c"); co {
		t.Fatalf("cha = (%d,true), muốn TRỐNG", g)
	}
}

func TestSoKHONGLaMotGiaTriThatChuKhongPhaiOTrong(t *testing.T) {
	// The other side of the same rule, and the reason `Gia` is a map of present keys rather than a
	// map with a zero default: a commune that typed 0 said something.
	b := BangDayDu{
		Bang:     BangNganSach{ID: "b", Loai: BangChi},
		Cot:      []CotNganSach{{ID: "c", BangID: "b", Kieu: CotSo}},
		KhoanMuc: []KhoanMucNganSach{{ID: "k", BangID: "b", Ten: "1.1", ThuTu: 1}},
		Gia:      map[string]map[string]Dong{"k": {"c": 0}},
	}
	if g, co := b.GiaTri("k", "c"); !co || g != 0 {
		t.Fatalf("ô ghi 0 đọc ra (%d,%v), muốn (0,true)", g, co)
	}
}

func TestGoThangVaoKhoanMucChaBiTuChoi(t *testing.T) {
	// The customer's decision of 06/09/2026, enforced in the business layer.
	if err := ChoGhiGiaTri(true); !errors.Is(err, ErrKhoanMucChaKhongGoThang) {
		t.Fatalf("ChoGhiGiaTri(cóCon) = %v, muốn ErrKhoanMucChaKhongGoThang", err)
	}
	if err := ChoGhiGiaTri(false); err != nil {
		t.Fatalf("ChoGhiGiaTri(lá) = %v, muốn nil", err)
	}
}

func TestCachTinhSuyTuCayChuKhongDoClientDat(t *testing.T) {
	if got := CachTinhTheoCay(true); got != TinhTheoCon {
		t.Errorf("có con -> %q, muốn %q", got, TinhTheoCon)
	}
	if got := CachTinhTheoCay(false); got != TinhTay {
		t.Errorf("không con -> %q, muốn %q", got, TinhTay)
	}
}

func TestVongLapChaConKhongTreoPhepDoc(t *testing.T) {
	// Nothing in the database stops a cycle in `cha_id` (migration 0006 states that cost), so a read
	// has to survive one. A wrong figure is something the screen can show; a request that never
	// returns is a worker held until the client gives up.
	b := BangDayDu{
		Bang: BangNganSach{ID: "b", Loai: BangChi},
		Cot:  []CotNganSach{{ID: "c", BangID: "b", Kieu: CotSo}},
		KhoanMuc: []KhoanMucNganSach{
			{ID: "x", BangID: "b", ChaID: "y", Ten: "X", ThuTu: 1},
			{ID: "y", BangID: "b", ChaID: "x", Ten: "Y", ThuTu: 2},
		},
		Gia: map[string]map[string]Dong{},
	}
	xong := make(chan struct{})
	go func() {
		defer close(xong)
		b.GiaTri("x", "c")
	}()
	select {
	case <-xong:
	case <-time.After(2 * time.Second):
		t.Fatal("phép đọc treo trên một vòng cha-con")
	}
}

// --- (4) the column declarations -------------------------------------------------------------------

func TestVaiTroPhaiDungLoaiBang(t *testing.T) {
	// A `thu` role on a `chi` sheet is an indicator reading a figure that is not what it is named
	// after. Its own sentence, because the correction is "you are on the wrong tab" rather than
	// "you mistyped".
	if err := KiemTraVaiTro(BangChi, VaiTroThuXaHuong); !errors.Is(err, ErrVaiTroSaiLoaiBang) {
		t.Fatalf("thu-xa-huong trên bảng chi = %v, muốn ErrVaiTroSaiLoaiBang", err)
	}
	if err := KiemTraVaiTro(BangThu, "khong-co-vai-tro-nay"); !errors.Is(err, ErrVaiTroSai) {
		t.Fatalf("vai trò lạ = %v, muốn ErrVaiTroSai", err)
	}
	if err := KiemTraVaiTro(BangThu, VaiTroThuXaHuong); err != nil {
		t.Fatalf("thu-xa-huong trên bảng thu = %v, muốn nil", err)
	}
}

func TestHaiCotCungMotVaiTroTrongMotBangBiTuChoi(t *testing.T) {
	// Two columns both marked `Thu xã hưởng` is a sheet where the `Cân đối` cell has two candidate
	// answers and no way to choose. The UNIQUE key in migration 0006 is the floor; this is the
	// sentence that arrives first.
	err := KiemTraBoCot(BangThu, []CotNganSach{
		{Ten: "Thu xã hưởng", Kieu: CotSo, VaiTro: VaiTroThuXaHuong},
		{Ten: "Thu xã hưởng (điều chỉnh)", Kieu: CotSo, VaiTro: VaiTroThuXaHuong},
	})
	if !errors.Is(err, ErrVaiTroTrungTrongBang) {
		t.Fatalf("= %v, muốn ErrVaiTroTrungTrongBang", err)
	}
}

func TestCotPhanTramKhongMangVaiTroVaPhaiCoCongThuc(t *testing.T) {
	if err := KiemTraCot(BangChi, CotNganSach{Kieu: CotPhanTram}); !errors.Is(err, ErrThieuCongThuc) {
		t.Errorf("cột %% không công thức = %v, muốn ErrThieuCongThuc", err)
	}
	if err := KiemTraCot(BangChi, CotNganSach{Kieu: CotSo, CongThuc: "a/b"}); !errors.Is(err, ErrThuaCongThuc) {
		t.Errorf("cột số có công thức = %v, muốn ErrThuaCongThuc", err)
	}
	err := KiemTraCot(BangChi, CotNganSach{Kieu: CotPhanTram, CongThuc: "a/b", VaiTro: VaiTroChiNganSach})
	if !errors.Is(err, ErrVaiTroTrenCotPhanTram) {
		t.Errorf("vai trò trên cột %% = %v, muốn ErrVaiTroTrenCotPhanTram", err)
	}
}

// --- (5) the bounds -----------------------------------------------------------------------------------

func TestGiaTriAmDuocChapNhan(t *testing.T) {
	// §9 rule 4 — and this is the one rule most likely to be "fixed" toward `> 0` by somebody who has
	// just read `chung_tu_giai_ngan`. That constraint belongs to a voucher, where a negative amount is
	// a refund pretending to be a payment (ADR 0035 §B). A budget line legitimately carries one.
	if err := KiemTraGiaTri(-5_000_000); err != nil {
		t.Fatalf("giá trị âm bị từ chối: %v", err)
	}
	if err := KiemTraGiaTri(GiaTriToiDa + 1); !errors.Is(err, ErrGiaTriQuaLon) {
		t.Fatalf("vượt trần = %v, muốn ErrGiaTriQuaLon", err)
	}
	if err := KiemTraGiaTri(-GiaTriToiDa - 1); !errors.Is(err, ErrGiaTriQuaLon) {
		t.Fatalf("vượt trần âm = %v, muốn ErrGiaTriQuaLon", err)
	}
}

func TestTyLeDongLamTronVeGanNhatVaKhongTran(t *testing.T) {
	if got := TyLeDong(2, 3); got != 6667 { // 66,666…% -> 66,67%
		t.Errorf("2/3 = %d phần vạn, muốn 6667", got)
	}
	if got := TyLeDong(0, 5); got != 0 {
		t.Errorf("0/5 = %d, muốn 0", got)
	}
	if got := TyLeDong(5, 0); got != 0 {
		t.Errorf("chia 0 = %d, muốn 0 (người gọi phải hỏi Co trước)", got)
	}
	// The overflow path: a figure at the typo-guard ceiling. `tu * 10000` would be 10^21 and wrap.
	if got := TyLeDong(GiaTriToiDa, GiaTriToiDa); got != 10_000 {
		t.Errorf("trần/trần = %d phần vạn, muốn 10000 (100%%) — số âm ở đây là tràn int64", got)
	}
}

// --- the sheet's unit: a closed list, and legacy text read back without guessing ------------------

func TestDonViTinhGhiChiNhanDungBaMa(t *testing.T) {
	for _, ma := range []string{"dong", "nghin-dong", "trieu-dong"} {
		d, err := KiemTraDonViTinh(ma)
		if err != nil || string(d) != ma || d.Nhan() == "" {
			t.Errorf("%q: = %q, %v — muốn nhận và có nhãn", ma, d, err)
		}
	}
	// A LABEL IS REFUSED ON A WRITE even though the read path recognises it: one input vocabulary.
	for _, sai := range []string{"Triệu đồng", "trieu_dong", "TRIEU-DONG", "tỷ đồng", "nghin"} {
		if _, err := KiemTraDonViTinh(sai); !errors.Is(err, ErrDonViTinhSai) {
			t.Errorf("%q: = %v, muốn ErrDonViTinhSai", sai, err)
		}
	}
	if _, err := KiemTraDonViTinh("  "); !errors.Is(err, ErrThieuDonViTinh) {
		t.Errorf("rỗng: = %v, muốn ErrThieuDonViTinh", err)
	}
}

func TestNhanTrieuDongTrungMacDinhCuaMigration0006(t *testing.T) {
	// A legacy row holding the column default and a new row written as `trieu-dong` must be the
	// same bytes, or the column would hold two shapes for one unit.
	if DonViTrieuDong.Nhan() != "Triệu đồng" {
		t.Fatalf("nhãn = %q, muốn đúng mặc định 'Triệu đồng' của migration 0006", DonViTrieuDong.Nhan())
	}
}

func TestDocDonViTinhCuKhongDoan(t *testing.T) {
	for luu, muon := range map[string]DonViTinh{
		"Triệu đồng":      DonViTrieuDong,
		"  TRIỆU   ĐỒNG ": DonViTrieuDong,
		"trđ":             DonViTrieuDong,
		"Nghìn đồng":      DonViNghinDong,
		"ngàn đồng":       DonViNghinDong,
		"1.000 đồng":      DonViNghinDong,
		"Đồng":            DonViDong,
		"VNĐ":             DonViDong,
		"trieu-dong":      DonViTrieuDong,
	} {
		if d, co := DocDonViTinhDaLuu(luu); !co || d != muon {
			t.Errorf("%q: = %q/%v, muốn %q", luu, d, co, muon)
		}
	}
	// NOT GUESSED: a scale outside the list, a bare word, a typo. Each would make the whole sheet
	// display a thousand times off if mapped wrongly.
	for _, luu := range []string{"Tỷ đồng", "triệu", "Triệu đòng", "", "USD"} {
		if d, co := DocDonViTinhDaLuu(luu); co {
			t.Errorf("%q: đoán thành %q — phải báo không nhận ra", luu, d)
		}
	}
}
