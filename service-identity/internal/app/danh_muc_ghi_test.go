package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The write use cases of the two catalogues, over the REAL store and a REAL transaction boundary
// (driver_gia_danh_muc_test.go). EVERY TEST RUNS OVER BOTH INSTANTIATIONS.

func chuoiDM(s string) *string { return &s }
func dungDM(b bool) *bool      { return &b }

// motVetDanhMuc asserts one audit entry, in the one committed transaction, attributed to the STAFF
// CODE (rule 6, invariant 8), in the commune of the context, and returns its delta.
func motVetDanhMuc(t *testing.T, ten string, k *khoDanhMucGia, hanhVi, chuDe string) map[string]any {
	t.Helper()
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("%s: có %d vết kiểm toán, muốn 1", ten, len(vet))
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("%s: giao dịch mở %d, commit %d, rollback %d — muốn đúng MỘT giao dịch cho cả ghi lẫn vết",
			ten, k.batDau, k.daCommit, k.daRollback)
	}
	a := vet[0].args
	if a[0] != string(xaDanhMuc) {
		t.Errorf("%s: vết mang xã %v, muốn %v", ten, a[0], xaDanhMuc)
	}
	if a[1] != maCanBoDanhMuc {
		t.Errorf("%s: actor_id = %v, muốn MÃ CÁN BỘ %q — không phải id nội bộ (luật 6 bất biến 8)", ten, a[1], maCanBoDanhMuc)
	}
	if a[3] != "10.0.0.7" {
		t.Errorf("%s: actor_ip = %v", ten, a[3])
	}
	if a[4] != hanhVi || a[5] != chuDe {
		t.Errorf("%s: hành vi/chủ thể = %v/%v, muốn %s/%s", ten, a[4], a[5], hanhVi, chuDe)
	}
	var delta map[string]any
	b, _ := a[7].([]byte)
	if err := json.Unmarshal(b, &delta); err != nil {
		t.Fatalf("%s: delta không đọc được: %v", ten, err)
	}
	return delta
}

// --- Them ---------------------------------------------------------------------------------------

// A COMMUNE'S ROW IS TIER 1, AND `nguon` IS A LITERAL. The INSERT goes to THIS catalogue's table,
// carries seven bound values (no $n for `nguon` / `ma_nguon_re_nhanh`), and shares its transaction
// with the audit entry.
func TestThemDanhMucGhiTang1VaVetCungGiaoDich(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]

		m, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: " khu-moi ", Nhan: " Khu mới ", ThuTu: 4})
		if err != nil {
			t.Fatalf("%s: Them: %v", c.ten, err)
		}
		if m.ID != idDanhMucMoi || m.Ma != "khu-moi" || m.Nhan != "Khu mới" || m.ThuTu != 4 ||
			!m.DangDung || m.Nguon != domain.NguonDonVi || m.tang() != domain.TangDonVi {
			t.Errorf("%s: kết quả sai: %+v", c.ten, m)
		}
		chen := k.cau("INSERT INTO " + c.bang + " ")
		if len(chen) != 1 {
			t.Fatalf("%s: có %d INSERT vào %s, muốn 1", c.ten, len(chen), c.bang)
		}
		if !strings.Contains(chen[0].sql, "'don-vi', false") || len(chen[0].args) != 7 {
			t.Errorf("%s: nguon/ma_nguon_re_nhanh phải là HẰNG trong câu INSERT, không phải tham số: %s %v",
				c.ten, chen[0].sql, chen[0].args)
		}
		motVetDanhMuc(t, c.ten, k, c.hanhViThem, "khu-moi")
	}
}

// A SOFT-DELETED ROW STILL OWNS ITS CODE (rule 7, invariant 3); another commune's code does not
// collide (codes are per commune).
func TestThemDanhMucMaDaDungKeCaDongDaXoa(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		if _, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "da-xoa", Nhan: "X"}); !errors.Is(err, idstore.ErrMaDaTonTai) {
			t.Errorf("%s: lỗi = %v, muốn ErrMaDaTonTai", c.ten, err)
		}
		if len(k.cau("INSERT")) != 0 || k.daCommit != 0 {
			t.Errorf("%s: bị từ chối mà vẫn ghi hoặc commit", c.ten)
		}
		if _, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "cua-xa-b", Nhan: "X"}); err != nil {
			t.Errorf("%s: mã của XÃ KHÁC không được chặn xã này: %v", c.ten, err)
		}
	}
}

// THE CEILING: at TranDanhMuc… live rows the create is refused — the read route would otherwise turn
// every picker in the commune into a 500.
func TestThemDanhMucDayTranThiTuChoi(t *testing.T) {
	for i, tran := range []int{idstore.TranDanhMucLoaiDonViDanCu, idstore.TranDanhMucKhoiNhiemVu} {
		bang := []string{"loai_don_vi_dan_cu", "khoi_nhiem_vu"}[i]
		var hang []hangDanhMuc
		for n := 0; n < tran; n++ {
			hang = append(hang, hangDanhMuc{bang: bang, xa: xaDanhMuc, id: fmt.Sprintf("r%d", n),
				ma: fmt.Sprintf("m%d", n), dangDung: true, nguon: "don-vi"})
		}
		k := &khoDanhMucGia{hang: hang}
		c := dungCatalogue(t, k)[i]
		if _, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "moi", Nhan: "Mới"}); !errors.Is(err, idstore.ErrDanhMucDayTran) {
			t.Errorf("%s: lỗi = %v, muốn ErrDanhMucDayTran", c.ten, err)
		}
	}
}

// SETTING A NEW DEFAULT CLEARS THE OLD ONE FIRST, in the same transaction.
func TestThemDanhMucMacDinhBoMacDinhCuTruoc(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		if _, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "moi", Nhan: "Mới", LaMacDinh: true}); err != nil {
			t.Fatal(err)
		}
		var thuTu []string
		for _, l := range k.lenh {
			switch {
			case strings.Contains(l.sql, "SET la_mac_dinh = false"):
				thuTu = append(thuTu, "bo")
			case strings.Contains(l.sql, "INSERT INTO "+c.bang):
				thuTu = append(thuTu, "chen")
			}
		}
		if strings.Join(thuTu, ",") != "bo,chen" {
			t.Errorf("%s: thứ tự = %v, muốn bỏ mặc định cũ RỒI mới chèn", c.ten, thuTu)
		}
	}
}

func TestThemDanhMucDauVaoSaiKhongMoGiaoDich(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		for _, yc := range []YeuCauThemDanhMuc{
			{Ma: "", Nhan: "X"}, {Ma: "Khu-Pho", Nhan: "X"}, {Ma: "khu-pho", Nhan: "  "}, {Ma: "khu-pho", Nhan: "X", ThuTu: -1},
		} {
			if _, err := c.them(ctxDanhMuc(), yc); !LaLoiDauVaoDanhMuc(err) {
				t.Errorf("%s %+v: lỗi = %v, muốn lỗi đầu vào", c.ten, yc, err)
			}
		}
		if k.batDau != 0 {
			t.Errorf("%s: đầu vào sai mà đã mở %d giao dịch", c.ten, k.batDau)
		}
	}
}

// AN AUDIT FAILURE TAKES THE INSERT DOWN WITH IT.
func TestThemDanhMucVetHongThiRollback(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau(), loiSau: "INSERT INTO audit_log"}
		c := dungCatalogue(t, k)[i]
		if _, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "moi", Nhan: "Mới"}); err == nil {
			t.Fatalf("%s: vết hỏng mà vẫn thành công", c.ten)
		}
		if k.daCommit != 0 || k.daRollback != 1 {
			t.Errorf("%s: commit %d rollback %d — muốn rollback cùng vết", c.ten, k.daCommit, k.daRollback)
		}
	}
}

// --- Sua ----------------------------------------------------------------------------------------

// TIER 3 CANNOT BE DISABLED, BUT CAN BE RELABELLED; TIER 2 CAN BE DISABLED.
//
// MUTATION THAT MUST TURN THIS RED: remove the ChoTat check from Sua. The fake has no trigger, so the
// UPDATE would then go through.
func TestSuaDanhMucTheoTang(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		if _, err := c.sua(ctxDanhMuc(), "m-t3", YeuCauSuaDanhMuc{DangDung: dungDM(false)}); !errors.Is(err, domain.ErrKhongTatDuocMucReNhanh) {
			t.Errorf("%s: tắt tầng 3: lỗi = %v, muốn ErrKhongTatDuocMucReNhanh", c.ten, err)
		}
		if len(k.cau("UPDATE "+c.bang)) != 0 || len(k.cau("audit_log")) != 0 {
			t.Fatalf("%s: tầng 3 bị từ chối mà vẫn UPDATE hoặc ghi vết", c.ten)
		}

		k = &khoDanhMucGia{hang: hangMau()}
		c = dungCatalogue(t, k)[i]
		m, err := c.sua(ctxDanhMuc(), "m-t3", YeuCauSuaDanhMuc{Nhan: chuoiDM("Nhãn mới"), DangDung: dungDM(true)})
		if err != nil || m.Nhan != "Nhãn mới" || m.Ma != "re-nhanh" {
			t.Errorf("%s: đổi nhãn tầng 3 (kèm active=true giữ nguyên): %+v, %v", c.ten, m, err)
		}
		delta := motVetDanhMuc(t, c.ten, k, c.hanhViSua, "re-nhanh")
		if truoc, _ := delta["truoc"].(map[string]any); truoc["nhan"] != "Rẽ nhánh" || len(truoc) != 1 {
			t.Errorf("%s: delta phải chỉ mang trường đã đổi: %v", c.ten, delta)
		}

		k = &khoDanhMucGia{hang: hangMau()}
		c = dungCatalogue(t, k)[i]
		if _, err := c.sua(ctxDanhMuc(), "m-t2", YeuCauSuaDanhMuc{DangDung: dungDM(false)}); err != nil {
			t.Errorf("%s: tắt tầng 2 phải được: %v", c.ten, err)
		}
	}
}

// THE UPDATE NEVER NAMES `ma`, `nguon` OR `ma_nguon_re_nhanh`.
func TestSuaDanhMucKhongChamMaVaNguon(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		if _, err := c.sua(ctxDanhMuc(), "m-t1", YeuCauSuaDanhMuc{Nhan: chuoiDM("Y"), ThuTu: so(9), LaMacDinh: dungDM(true)}); err != nil {
			t.Fatal(err)
		}
		for _, up := range k.cau("UPDATE " + c.bang) {
			set := up.sql[:strings.Index(up.sql, "WHERE")]
			for _, cam := range []string{" ma ", " ma=", "nguon"} {
				if strings.Contains(set, cam) {
					t.Errorf("%s: câu UPDATE chạm %q: %s", c.ten, cam, up.sql)
				}
			}
		}
	}
}

// A NO-OP WRITES NOTHING AND AUDITS NOTHING — what makes idem.KhongCan honest on PATCH.
func TestSuaDanhMucKhongDoiThiKhongGhi(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		if _, err := c.sua(ctxDanhMuc(), "m-t1", YeuCauSuaDanhMuc{ThuTu: so(3), DangDung: dungDM(true)}); err != nil {
			t.Fatal(err)
		}
		if len(k.cau("UPDATE "+c.bang+" SET")) != 0 || len(k.cau("audit_log")) != 0 {
			t.Errorf("%s: không đổi gì mà vẫn UPDATE hoặc ghi vết", c.ten)
		}
	}
}

// ANOTHER COMMUNE'S ROW, A SOFT-DELETED ONE AND AN INVENTED ID ARE ONE ANSWER.
//
// MUTATION THAT MUST TURN THIS RED: drop `tenant_id = $1 AND` from the lock read and bind the id at
// $1 — the fake then answers across communes and finds `m-khac`.
func TestSuaVaXoaDanhMucKhongTimThay(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		for _, id := range []string{"m-khac", "m-xoa", "khong-co", ""} {
			k := &khoDanhMucGia{hang: hangMau()}
			c := dungCatalogue(t, k)[i]
			if _, err := c.sua(ctxDanhMuc(), id, YeuCauSuaDanhMuc{Nhan: chuoiDM("X")}); !errors.Is(err, idstore.ErrDanhMucKhongTonTai) {
				t.Errorf("%s sửa %q: lỗi = %v", c.ten, id, err)
			}
			if err := c.xoa(ctxDanhMuc(), id, "lý do"); !errors.Is(err, idstore.ErrDanhMucKhongTonTai) {
				t.Errorf("%s xoá %q: lỗi = %v", c.ten, id, err)
			}
			if len(k.cau("UPDATE "+c.bang+" SET")) != 0 {
				t.Errorf("%s %q: vẫn UPDATE", c.ten, id)
			}
		}
	}
}

// --- Xoa ----------------------------------------------------------------------------------------

// A SYSTEM ROW (tier 2 or 3) IS NEVER DELETED.
//
// MUTATION THAT MUST TURN THIS RED: remove the ChoXoaMem check from Xoa.
func TestXoaDanhMucHeThongBiTuChoi(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		for _, id := range []string{"m-t2", "m-t3"} {
			k := &khoDanhMucGia{hang: hangMau()}
			c := dungCatalogue(t, k)[i]
			if err := c.xoa(ctxDanhMuc(), id, "không dùng"); !errors.Is(err, domain.ErrKhongXoaDuocMucHeThong) {
				t.Errorf("%s xoá %s: lỗi = %v, muốn ErrKhongXoaDuocMucHeThong", c.ten, id, err)
			}
			if len(k.cau("deleted_at = now()")) != 0 || len(k.cau("audit_log")) != 0 || k.daCommit != 0 {
				t.Errorf("%s xoá %s: bị từ chối mà vẫn xoá mềm, ghi vết hoặc commit", c.ten, id)
			}
		}
	}
}

// A COMMUNE ROW IS SOFT DELETED — deleted_by is the STAFF CODE, the reason is in the column and in
// the trail, and everything shares one transaction.
func TestXoaDanhMucTang1XoaMemVaVet(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		if err := c.xoa(ctxDanhMuc(), "m-t1", "  nhập nhầm "); err != nil {
			t.Fatalf("%s: %v", c.ten, err)
		}
		xoa := k.cau("UPDATE " + c.bang + " SET deleted_at = now()")
		if len(xoa) != 1 {
			t.Fatalf("%s: có %d câu xoá mềm, muốn 1", c.ten, len(xoa))
		}
		a := xoa[0].args
		if a[0] != string(xaDanhMuc) || a[1] != "m-t1" || a[2] != maCanBoDanhMuc || a[3] != "nhập nhầm" {
			t.Errorf("%s: tham số xoá mềm = %v — deleted_by phải là MÃ CÁN BỘ %q, không phải %q",
				c.ten, a, maCanBoDanhMuc, idCanBoDanhMuc)
		}
		if len(k.cau("DELETE")) != 0 {
			t.Errorf("%s: có câu DELETE — danh mục chỉ xoá mềm", c.ten)
		}
		delta := motVetDanhMuc(t, c.ten, k, c.hanhViXoa, "cua-xa")
		if delta["ly_do"] != "nhập nhầm" || delta["xoa_mem"] != true {
			t.Errorf("%s: delta xoá: %v", c.ten, delta)
		}
	}
}

func TestXoaDanhMucThieuLyDoKhongMoGiaoDich(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		if err := c.xoa(ctxDanhMuc(), "m-t1", "   "); !errors.Is(err, domain.ErrThieuLyDoXoaDanhMuc) {
			t.Errorf("%s: lỗi = %v", c.ten, err)
		}
		if k.batDau != 0 {
			t.Errorf("%s: thiếu lý do mà đã mở giao dịch", c.ten)
		}
	}
}

// --- cross-cutting ------------------------------------------------------------------------------

// EVERY STATEMENT BINDS THE COMMUNE OF THE CONTEXT AT $1 (rule 1, invariants 4 and 5).
func TestDanhMucMoiCauMangXaCuaNguCanh(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		_, _ = c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "moi", Nhan: "Mới", LaMacDinh: true})
		_, _ = c.sua(ctxDanhMuc(), "m-t2", YeuCauSuaDanhMuc{Nhan: chuoiDM("Z"), LaMacDinh: dungDM(true)})
		_ = c.xoa(ctxDanhMuc(), "m-t1", "lý do")
		if len(k.lenh) < 8 {
			t.Fatalf("%s: chỉ %d câu lệnh — phép kiểm không kiểm gì", c.ten, len(k.lenh))
		}
		for _, l := range k.lenh {
			if len(l.args) == 0 || l.args[0] != string(xaDanhMuc) {
				t.Errorf("%s: câu lệnh không mang xã ở $1: %s (args %v)", c.ten, l.sql, l.args)
			}
		}
	}
}

// NO FALLBACK TO THE INTERNAL ID: an empty staff code refuses every write before any transaction.
func TestDanhMucThieuMaCanBoThiTuChoi(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		c.nguoi.Vet.ID = ""
		if _, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "moi", Nhan: "Mới"}); err == nil {
			t.Errorf("%s: Them không có mã cán bộ mà vẫn ghi", c.ten)
		}
		if _, err := c.sua(ctxDanhMuc(), "m-t1", YeuCauSuaDanhMuc{Nhan: chuoiDM("Y")}); err == nil {
			t.Errorf("%s: Sua không có mã cán bộ mà vẫn ghi", c.ten)
		}
		if err := c.xoa(ctxDanhMuc(), "m-t1", "lý do"); err == nil {
			t.Errorf("%s: Xoa không có mã cán bộ mà vẫn ghi", c.ten)
		}
		if k.batDau != 0 {
			t.Errorf("%s: đã mở %d giao dịch", c.ten, k.batDau)
		}
	}
}
