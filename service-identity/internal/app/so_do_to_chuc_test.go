package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The two write use cases of the org chart, over the REAL store and a REAL transaction boundary
// (driver_gia_bo_phan_test.go).

// chuoi mirrors `so` in danh_ba_mini_app_test.go, which this file also uses.
func chuoi(s string) *string { return &s }

// motVetBoPhan asserts one audit entry, in the one committed transaction, attributed to the STAFF
// CODE (rule 6, invariant 8), and returns its bound parameters.
func motVetBoPhan(t *testing.T, k *khoBoPhanGia, hanhVi, chuDe string) map[string]any {
	t.Helper()
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết kiểm toán, muốn 1", len(vet))
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d, commit %d, rollback %d — muốn đúng MỘT giao dịch cho cả ghi lẫn vết",
			k.batDau, k.daCommit, k.daRollback)
	}
	a := vet[0].args
	if a[0] != string(xaBoPhan) {
		t.Errorf("vết mang xã %v, muốn %v", a[0], xaBoPhan)
	}
	if a[1] != maCanBoBoPhan {
		t.Errorf("actor_id = %v, muốn MÃ CÁN BỘ %q (luật 6 bất biến 8) — không phải id nội bộ", a[1], maCanBoBoPhan)
	}
	if a[3] != "10.0.0.7" {
		t.Errorf("actor_ip = %v", a[3])
	}
	if a[4] != hanhVi || a[5] != chuDe {
		t.Errorf("hành vi/chủ thể = %v/%v, muốn %s/%s", a[4], a[5], hanhVi, chuDe)
	}
	var delta map[string]any
	b, _ := a[7].([]byte)
	if err := json.Unmarshal(b, &delta); err != nil {
		t.Fatalf("delta không đọc được: %v", err)
	}
	return delta
}

// --- Them ---------------------------------------------------------------------------------------

func TestThemBoPhanSinhMaTuTenVaGhiVetCungGiaoDich(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	bp, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "  VĂN PHÒNG ĐẢNG ỦY ", ChaID: "bp-a", ThuTu: so(4)}, nguoiBoPhan())
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if bp.Ma != "van-phong-dang-uy" || bp.Ten != "VĂN PHÒNG ĐẢNG ỦY" || bp.ChaID != "bp-a" || bp.ThuTu != 4 || bp.ID != idBoPhanMoi {
		t.Fatalf("kết quả sai: %+v", bp)
	}
	chen := k.cau("INSERT INTO bo_phan")
	if len(chen) != 1 {
		t.Fatalf("có %d INSERT bo_phan, muốn 1", len(chen))
	}
	a := chen[0].args
	if a[0] != string(xaBoPhan) || a[1] != idBoPhanMoi || a[2] != "VĂN PHÒNG ĐẢNG ỦY" || a[3] != "van-phong-dang-uy" || a[4] != "bp-a" || a[5] != int64(4) {
		t.Errorf("tham số INSERT sai thứ tự/giá trị: %v", a)
	}
	delta := motVetBoPhan(t, k, HanhViThemBoPhan, "van-phong-dang-uy")
	if _, ok := delta["sau"]; !ok {
		t.Errorf("delta thiếu `sau`: %v", delta)
	}
}

// THE UNIQUE KEY IS NOT PARTIAL, so a SOFT-DELETED unit's code is still taken — the suffix must
// skip it, or the INSERT is refused by a key the screen cannot explain.
func TestThemBoPhanTrungMaThiThemHauToKeCaDongDaXoa(t *testing.T) {
	k := &khoBoPhanGia{hang: append(cayBaTang(),
		hangBoPhan{xa: xaBoPhan, id: "bp-cu", ma: "van-phong-dang-uy", ten: "CŨ", daXoa: true},
		hangBoPhan{xa: xaBoPhan, id: "bp-cu2", ma: "van-phong-dang-uy-2", ten: "CŨ 2"},
		// Another commune's `-3` must NOT push this commune to `-4`: codes are per commune.
		hangBoPhan{xa: xaBoPhanKhac, id: "bp-k3", ma: "van-phong-dang-uy-3", ten: "XÃ B"},
	)}
	uc, ctx := dungUseCaseBoPhan(t, k)

	bp, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "VĂN PHÒNG ĐẢNG ỦY"}, nguoiBoPhan())
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if bp.Ma != "van-phong-dang-uy-3" {
		t.Fatalf("mã = %q, muốn van-phong-dang-uy-3 (gốc đã xoá mềm vẫn giữ mã, -2 đã dùng)", bp.Ma)
	}
	if bp.ChaID != "" {
		t.Errorf("không gửi cha mà ChaID = %q", bp.ChaID)
	}
	if a := k.cau("INSERT INTO bo_phan")[0].args; a[4] != "" {
		t.Errorf("cha_id ở gốc phải là \"\" (store gập thành NULL), nhận %v", a[4])
	}
}

// A CODE THE PERSON TYPED IS USED EXACTLY OR REFUSED — never silently suffixed.
func TestThemBoPhanMaTuNhapTrungThiTuChoi(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	_, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "VĂN PHÒNG MỚI", Ma: "da-xoa"}, nguoiBoPhan())
	if !errors.Is(err, idstore.ErrMaBoPhanDaDung) {
		t.Fatalf("lỗi = %v, muốn ErrMaBoPhanDaDung — mã của dòng đã xoá mềm không được cấp lại", err)
	}
	if len(k.cau("INSERT")) != 0 || k.daCommit != 0 {
		t.Error("bị từ chối mà vẫn ghi hoặc commit")
	}

	bp, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "VĂN PHÒNG MỚI", Ma: "vp-moi"}, nguoiBoPhan())
	if err != nil || bp.Ma != "vp-moi" {
		t.Fatalf("mã tự nhập hợp lệ: %q, %v", bp.Ma, err)
	}
}

// A PARENT IN ANOTHER COMMUNE IS NOT FOUND, and the refusal names the parent, not the target.
//
// MUTATION THAT MUST TURN THIS RED: drop `tenant_id = $1 AND` from KhoaBoPhan and bind the id at $1.
// The fake then answers across every commune, as PostgreSQL would, and finds `bp-khac`.
func TestThemBoPhanChaOXaKhacBiTuChoi(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	_, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "TỔ MỚI", ChaID: "bp-khac"}, nguoiBoPhan())
	if !errors.Is(err, idstore.ErrBoPhanChaKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrBoPhanChaKhongTonTai — RÒ RỈ: bộ phận của xã khác được nhận làm cha", err)
	}
	if len(k.cau("INSERT")) != 0 {
		t.Error("cha của xã khác mà vẫn ghi")
	}
}

func TestThemBoPhanChaDaXoaMemBiTuChoi(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	if _, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "TỔ MỚI", ChaID: "bp-xoa"}, nguoiBoPhan()); !errors.Is(err, idstore.ErrBoPhanChaKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrBoPhanChaKhongTonTai", err)
	}
}

// EVERY STATEMENT BINDS THE COMMUNE OF THE CONTEXT AT $1 (rule 1, invariants 4 and 5).
func TestThemBoPhanMoiCauMangXaCuaNguCanh(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)
	if _, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "TỔ MỚI", ChaID: "bp-b"}, nguoiBoPhan()); err != nil {
		t.Fatal(err)
	}
	for _, l := range k.lenh {
		if len(l.args) == 0 || l.args[0] != string(xaBoPhan) {
			t.Errorf("câu lệnh không mang xã ở $1: %s (args %v)", l.sql, l.args)
		}
	}
}

// AN AUDIT FAILURE TAKES THE INSERT DOWN WITH IT — the two share one transaction.
func TestThemBoPhanVetHongThiKhongCommit(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang(), loiSau: "INSERT INTO audit_log"}
	uc, ctx := dungUseCaseBoPhan(t, k)

	if _, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "TỔ MỚI"}, nguoiBoPhan()); err == nil {
		t.Fatal("vết hỏng mà Them vẫn thành công")
	}
	if len(k.cau("INSERT INTO bo_phan")) != 1 || k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("INSERT %d, commit %d, rollback %d — muốn INSERT đã chạy rồi bị rollback cùng vết",
			len(k.cau("INSERT INTO bo_phan")), k.daCommit, k.daRollback)
	}
}

func TestThemBoPhanDauVaoSaiKhongMoGiaoDich(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)
	for _, yc := range []YeuCauThemBoPhan{
		{Ten: "   "},
		{Ten: "!!!"}, // nothing to derive a code from
		{Ten: "TỔ", Ma: "To-Mot"},
		{Ten: "TỔ", ThuTu: so(-1)},
	} {
		_, err := uc.Them(ctx, yc, nguoiBoPhan())
		if !LaLoiDauVaoBoPhan(err) {
			t.Errorf("%+v: lỗi = %v, muốn lỗi đầu vào (400)", yc, err)
		}
	}
	if k.batDau != 0 {
		t.Errorf("đầu vào sai mà đã mở %d giao dịch", k.batDau)
	}
}

// --- Sua ----------------------------------------------------------------------------------------

// A → B → C; moving A under C would make A its own ancestor.
//
// MUTATION THAT MUST TURN THIS RED: remove the kiemVongLap call from Sua.
func TestSuaBoPhanDoiChaTaoVongThiTuChoi(t *testing.T) {
	for ten, cha := range map[string]string{"dưới cháu": "bp-c", "dưới con": "bp-b", "dưới chính nó": "bp-a"} {
		k := &khoBoPhanGia{hang: cayBaTang()}
		uc, ctx := dungUseCaseBoPhan(t, k)

		_, err := uc.Sua(ctx, "bp-a", YeuCauSuaBoPhan{ChaID: chuoi(cha)}, nguoiBoPhan())
		if !errors.Is(err, ErrCayBoPhanVongLap) {
			t.Errorf("%s: lỗi = %v, muốn ErrCayBoPhanVongLap", ten, err)
		}
		if len(k.cau("UPDATE bo_phan")) != 0 || len(k.cau("audit_log")) != 0 || k.daCommit != 0 {
			t.Errorf("%s: vòng lặp mà vẫn ghi hoặc commit", ten)
		}
	}
}

// THE WALK LOCKS EVERY ANCESTOR, in the same transaction as the UPDATE. An unlocked walk is a check
// with a gap through which two crossing moves both pass.
func TestSuaBoPhanDoiChaHopLeKhoaCaChuoiToTien(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	// Move C (under B) directly under A: walk A → root.
	bp, err := uc.Sua(ctx, "bp-c", YeuCauSuaBoPhan{ChaID: chuoi("bp-a")}, nguoiBoPhan())
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if bp.ChaID != "bp-a" || bp.Ma != "to-mot-cua" {
		t.Fatalf("kết quả: %+v", bp)
	}
	khoa := k.cau("FOR UPDATE")
	if len(khoa) != 2 || khoa[0].args[1] != "bp-c" || khoa[1].args[1] != "bp-a" {
		t.Fatalf("các lượt khoá = %v, muốn bp-c rồi bp-a", khoa)
	}
	up := k.cau("UPDATE bo_phan")
	if len(up) != 1 || up[0].args[3] != "bp-a" {
		t.Fatalf("UPDATE: %v", up)
	}
	delta := motVetBoPhan(t, k, HanhViSuaBoPhan, "to-mot-cua")
	truoc, _ := delta["truoc"].(map[string]any)
	sau, _ := delta["sau"].(map[string]any)
	if truoc["cha_id"] != "bp-b" || sau["cha_id"] != "bp-a" {
		t.Errorf("delta không mang cha trước/sau: %v", delta)
	}
}

func TestSuaBoPhanDoiVeGoc(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	bp, err := uc.Sua(ctx, "bp-c", YeuCauSuaBoPhan{ChaID: chuoi("")}, nguoiBoPhan())
	if err != nil || bp.ChaID != "" {
		t.Fatalf("dời về gốc: %+v, %v", bp, err)
	}
	if n := len(k.cau("FOR UPDATE")); n != 1 {
		t.Errorf("dời về gốc mà khoá %d dòng, muốn 1 (chỉ chính nó — gốc không có tổ tiên)", n)
	}
}

// THE CODE IS NOT EDITABLE: the UPDATE names no `ma`, whatever else changes.
func TestSuaBoPhanKhongDungToiMa(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	bp, err := uc.Sua(ctx, "bp-b", YeuCauSuaBoPhan{Ten: chuoi("VĂN PHÒNG UBND"), ThuTu: so(7)}, nguoiBoPhan())
	if err != nil {
		t.Fatal(err)
	}
	if bp.Ma != "van-phong" {
		t.Errorf("mã đổi thành %q sau khi đổi tên — mã đã cấp không bao giờ đổi", bp.Ma)
	}
	up := k.cau("UPDATE bo_phan")[0]
	set := up.sql[:strings.Index(up.sql, "WHERE")]
	if strings.Contains(set, " ma ") || strings.Contains(set, " ma=") {
		t.Errorf("câu UPDATE chạm vào cột ma: %s", up.sql)
	}
	motVetBoPhan(t, k, HanhViSuaBoPhan, "van-phong")
}

// A NO-OP WRITES NOTHING AND AUDITS NOTHING — what makes idem.KhongCan honest on PATCH.
func TestSuaBoPhanKhongDoiGiThiKhongGhi(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	if _, err := uc.Sua(ctx, "bp-b", YeuCauSuaBoPhan{Ten: chuoi("VĂN PHÒNG"), ChaID: chuoi("bp-a"), ThuTu: so(2)}, nguoiBoPhan()); err != nil {
		t.Fatal(err)
	}
	if len(k.cau("UPDATE bo_phan")) != 0 || len(k.cau("audit_log")) != 0 {
		t.Error("không đổi gì mà vẫn UPDATE hoặc ghi vết")
	}
}

// ANOTHER COMMUNE'S UNIT, A SOFT-DELETED ONE AND AN INVENTED ID ARE ONE ANSWER.
func TestSuaBoPhanKhongTimThay(t *testing.T) {
	for _, id := range []string{"bp-khac", "bp-xoa", "bp-khong-co", ""} {
		k := &khoBoPhanGia{hang: cayBaTang()}
		uc, ctx := dungUseCaseBoPhan(t, k)
		if _, err := uc.Sua(ctx, id, YeuCauSuaBoPhan{Ten: chuoi("X")}, nguoiBoPhan()); !errors.Is(err, idstore.ErrKhongTimThayBoPhan) {
			t.Errorf("%q: lỗi = %v, muốn ErrKhongTimThayBoPhan", id, err)
		}
		if len(k.cau("UPDATE bo_phan")) != 0 {
			t.Errorf("%q: vẫn UPDATE", id)
		}
	}
}

func TestSuaBoPhanChaMoiOXaKhacHoacDaXoaBiTuChoi(t *testing.T) {
	for _, cha := range []string{"bp-khac", "bp-xoa", "bp-khong-co"} {
		k := &khoBoPhanGia{hang: cayBaTang()}
		uc, ctx := dungUseCaseBoPhan(t, k)
		if _, err := uc.Sua(ctx, "bp-c", YeuCauSuaBoPhan{ChaID: chuoi(cha)}, nguoiBoPhan()); !errors.Is(err, idstore.ErrBoPhanChaKhongTonTai) {
			t.Errorf("cha %q: lỗi = %v, muốn ErrBoPhanChaKhongTonTai", cha, err)
		}
	}
}

// A LOOP ALREADY IN THE DATA is a failure, never "no cycle" and never an infinite walk.
func TestSuaBoPhanVongLapSanCoKhongChayMai(t *testing.T) {
	k := &khoBoPhanGia{hang: []hangBoPhan{
		{xa: xaBoPhan, id: "bp-x", ma: "x", ten: "X"},
		{xa: xaBoPhan, id: "bp-p", ma: "p", ten: "P", cha: "bp-q"},
		{xa: xaBoPhan, id: "bp-q", ma: "q", ten: "Q", cha: "bp-p"},
	}}
	uc, ctx := dungUseCaseBoPhan(t, k)
	_, err := uc.Sua(ctx, "bp-x", YeuCauSuaBoPhan{ChaID: chuoi("bp-p")}, nguoiBoPhan())
	if err == nil || errors.Is(err, ErrCayBoPhanVongLap) || LaLoiDauVaoBoPhan(err) {
		t.Fatalf("lỗi = %v — vòng lặp sẵn có phải là lỗi hệ thống, không phải lỗi của người gọi", err)
	}
	if len(k.cau("UPDATE bo_phan")) != 0 {
		t.Error("vẫn UPDATE")
	}
}

func TestSuaBoPhanThieuMaNguoiThucHienThiTuChoi(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)
	n := nguoiBoPhan()
	n.Vet.ID = ""
	if _, err := uc.Sua(ctx, "bp-b", YeuCauSuaBoPhan{Ten: chuoi("Y")}, n); err == nil {
		t.Fatal("thiếu mã cán bộ mà vẫn ghi — luật 6 bất biến 8: không có đường lùi về ID")
	}
	if k.batDau != 0 {
		t.Error("đã mở giao dịch")
	}
}
