package migrations

import (
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0013 (the petition processing logbook `nhat_ky_phan_anh`).
//
// WHAT THIS IS AND IS NOT — same standing as nhan_trang_thai_nhiem_vu_test.go. It reads the SQL this
// binary embeds and asserts the key, the CHECKs and the append-only triggers are WRITTEN. It does NOT
// prove PostgreSQL enforces them or applies the file; that needs VIGOV_TEST_DSN (the pg suites in
// internal/store SKIP without it). It exists so that a widened or deleted CHECK, or a removed
// trigger, goes red somewhere that always runs.

const tep0013 = "0013_nhat_ky_phan_anh.sql"

// TestMigration0013TrangThaiTrungKhopSoPhanAnh — the timeline's status list must be the SAME SET as
// the register's nine (0004). A timeline chip the register would refuse contradicts the row it hangs
// off; a register status the timeline refuses makes that act unloggable, and the write — which
// shares one transaction with the business change — rolls back entirely.
//
// THE MUTATION THAT MUST TURN THIS RED: add, drop or misspell one code in either list.
func TestMigration0013TrangThaiTrungKhopSoPhanAnh(t *testing.T) {
	so := danhSachMaTrongCheck(t, maChay(t, tep0004), "phieu_phan_anh_trang_thai_hop_le", "trang_thai")
	nk := danhSachMaTrongCheck(t, maChay(t, tep0013), "nhat_ky_phan_anh_trang_thai_hop_le", "trang_thai_tai_thoi_diem")
	if len(so) != 9 {
		t.Fatalf("0004 khai %d mã trạng thái, mong 9 (ADR 0027) — bộ phân tích đã mù hoặc 0004 đã đổi: %v", len(so), so)
	}
	if strings.Join(nk, ",") != strings.Join(so, ",") {
		t.Errorf("mã trạng thái trong 0013 KHÁC sổ phản ánh 0004:\n  0013: %v\n  0004: %v", nk, so)
	}
}

// TestMigration0013HanhViDongKin — the act list is exactly the seven the card fixed.
func TestMigration0013HanhViDongKin(t *testing.T) {
	got := danhSachMaTrongCheck(t, maChay(t, tep0013), "nhat_ky_phan_anh_hanh_vi_hop_le", "hanh_vi")
	want := "chuyen-cap-tren,chuyen-trang-thai,dong-phieu,ghi-chu,khong-tiep-nhan,phan-cong,phan-loai"
	if strings.Join(got, ",") != want {
		t.Errorf("danh sách hanh_vi = %v, mong %s", got, want)
	}
}

// TestMigration0013KhoaRangBuocVaTrigger — composite key, hash partitioning, the bindings, the caps,
// and the append-only guard on UPDATE/DELETE (parent, cloned to leaves) and TRUNCATE (per leaf).
//
// THE MUTATIONS THAT MUST TURN THIS RED: a single-column key; dropping the ELSE arm of the
// assignment binding (a stray assignee on a note row); dropping `is not null` before `btrim` on the
// note (a CHECK treats NULL as passed); removing either trigger.
func TestMigration0013KhoaRangBuocVaTrigger(t *testing.T) {
	sql := maChay(t, tep0013)

	for _, c := range []struct{ can, vi string }{
		{"primary key (tenant_id, id)", "khoá chính phải hợp thành với tenant_id (luật 1 bất biến 6)"},
		{") partition by hash (tenant_id);", "bảng phải phân mảnh hash theo tenant_id như các bảng anh em (ADR 0010)"},
		{"partition of nhat_ky_phan_anh ' 'for values with (modulus 32, remainder %s)",
			"thiếu vòng tạo 32 mảnh — bảng phân mảnh không mảnh từ chối mọi INSERT"},
		{"phieu_phan_anh_id text not null", "mục nhật ký phải gắn một phiếu"},
		{"thoi_diem timestamptz not null", "thiếu thời điểm"},
		{"nguoi_ma text not null", "người làm phải NOT NULL — CHECK coi NULL là đạt"},
		{"hanh_vi text not null", "hành vi phải NOT NULL"},
		{"trang_thai_tai_thoi_diem text not null", "trạng thái tại thời điểm phải NOT NULL"},
		{"dinh_kem jsonb not null default '[]'::jsonb", "đính kèm mặc định mảng rỗng"},
		{"check (btrim(nguoi_ma) <> '' and char_length(nguoi_ma) <= 64)",
			"mã cán bộ phải không rỗng và có trần = domain.CanBoToiDa (luật 6 bất biến 8)"},
		{"when 'phan-cong' then bo_phan_id is not null and btrim(bo_phan_id) <> '' " +
			"and (can_bo_xu_ly_ma is null or btrim(can_bo_xu_ly_ma) <> '') " +
			"else bo_phan_id is null and can_bo_xu_ly_ma is null end",
			"cặp phân công chỉ có trên phan-cong, bộ phận bắt buộc (domain.KiemPhanCong)"},
		{"check (char_length(bo_phan_id) <= 64)", "bộ phận phải có trần = domain.BoPhanToiDa"},
		{"check (char_length(can_bo_xu_ly_ma) <= 64)", "cán bộ phải có trần = domain.CanBoToiDa"},
		{"when 'ghi-chu' then noi_dung is not null and btrim(noi_dung) <> '' " +
			"else noi_dung is null or btrim(noi_dung) <> '' end",
			"ghi chú bắt buộc nội dung không rỗng; nơi khác không được là chuỗi rỗng"},
		{"check (char_length(noi_dung) <= 2000)", "nội dung phải có trần theo KÝ TỰ = domain.KetQuaToiDa"},
		{"on nhat_ky_phan_anh (tenant_id, phieu_phan_anh_id, thoi_diem desc, id desc)",
			"chỉ mục dòng thời gian mới-nhất-trước, bắt đầu bằng tenant_id"},
		{"before update or delete on nhat_ky_phan_anh for each row execute function nhat_ky_phan_anh_chi_them()",
			"trigger chặn sửa/xoá đã mất (luật 7 cấm #5)"},
		{"before truncate on %s ' 'for each statement execute function nhat_ky_phan_anh_chi_them()",
			"trigger chặn TRUNCATE từng mảnh đã mất"},
	} {
		if !strings.Contains(sql, c.can) {
			t.Errorf("0013 thiếu %q — %s", c.can, c.vi)
		}
	}
}

// TestMigration0013KhongPhaHuyKhongXoaMem — this file only ADDS; it touches nothing that exists, and
// the table has no soft-delete columns (a row that could be hidden is a timeline that can be made
// to say something else).
func TestMigration0013KhongPhaHuyKhongXoaMem(t *testing.T) {
	sql := maChay(t, tep0013)
	for _, c := range []struct{ cam, vi string }{
		{"deleted_at", "có cột xoá mềm — nhật ký chỉ thêm, không có trạng thái 'đã ẩn'"},
		{"alter table", "sửa một bảng có sẵn — tệp này chỉ thêm"},
		{"drop table", "xoá bảng"},
		{"drop column", "xoá cột"},
		{"function ho_so_luu_tru_bat_bien", "thay hàm canh lưu trữ của 0004"},
		{"function nhat_ky_nhiem_vu_chi_them", "thay hàm canh của 0006"},
		{"references ", "khoá ngoại tới phiếu — lý do không dùng nằm trên cột"},
	} {
		if strings.Contains(sql, c.cam) {
			t.Errorf("0013 chứa %q — %s", c.cam, c.vi)
		}
	}
}
