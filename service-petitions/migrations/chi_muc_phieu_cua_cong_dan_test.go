package migrations

import (
	"os"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0014 (the index behind `GET /api/v1/my-citizen-reports`).
//
// WHAT THIS IS AND IS NOT — same standing as nhat_ky_phan_anh_test.go. It reads the SQL this binary
// embeds and asserts the index is WRITTEN with the right columns, direction and predicate. It does
// NOT prove the planner picks it; that needs VIGOV_TEST_DSN and an EXPLAIN. It exists so that a
// reordered column, a lost DESC or a drifted predicate goes red somewhere that always runs.

const tep0014 = "0014_chi_muc_phieu_cua_cong_dan.sql"

// TestMigration0014ChiMucDungHinh — tenant first (rule 1, the agent's non-negotiable #4), then the
// citizen equality, then the sort column and tie-break in QueryPage's direction, partial on live rows.
//
// THE MUTATIONS THAT MUST TURN THIS RED: dropping `tenant_id` or moving it off the front; putting
// `trang_thai` before `goc_dem_han`; losing either DESC; dropping or widening the WHERE.
func TestMigration0014ChiMucDungHinh(t *testing.T) {
	sql := maChay(t, tep0014)
	can := "create index if not exists phieu_phan_anh_cua_cong_dan " +
		"on phieu_phan_anh (tenant_id, cong_dan_id, goc_dem_han desc, id desc) " +
		"where deleted_at is null;"
	if !strings.Contains(sql, can) {
		t.Errorf("0014 thiếu %q — chỉ mục danh sách của công dân phải bắt đầu bằng tenant_id, "+
			"khớp thứ tự sắp (goc_dem_han, id) DESC của QueryPage và chỉ gồm dòng chưa xoá mềm", can)
	}
}

// TestMigration0014ViTuKhopStore — the partial predicate is only usable if the store's query implies
// it, and the index only avoids a sort if the store sorts by `goc_dem_han`. Both facts live in
// internal/store/phieu_phan_anh.go; this reads that file so a change there that orphans the index
// turns red here rather than surfacing as a slow list in production.
func TestMigration0014ViTuKhopStore(t *testing.T) {
	b, err := os.ReadFile("../internal/store/phieu_phan_anh.go")
	if err != nil {
		t.Fatalf("đọc store: %v", err)
	}
	src := string(b)
	for _, c := range []struct{ can, vi string }{
		{"loc := `AND cong_dan_id = $2 AND deleted_at IS NULL`",
			"vị từ của DanhSachCuaCongDan đã đổi — vị từ một phần của 0014 có thể không còn được suy ra"},
		{`page.Col("received_at", "goc_dem_han", page.KindTime)`,
			"cột sắp của danh sách công dân không còn là goc_dem_han — chỉ mục 0014 không còn tránh được bước sắp"},
	} {
		if !strings.Contains(src, c.can) {
			t.Errorf("store thiếu %q — %s", c.can, c.vi)
		}
	}
}

// TestMigration0014ChiThemChiMuc — this file creates one index and nothing else: no row, no column,
// no constraint, no trigger of an archival table is touched (rule 7).
func TestMigration0014ChiThemChiMuc(t *testing.T) {
	sql := maChay(t, tep0014)
	for _, c := range []struct{ cam, vi string }{
		{"concurrently", "CONCURRENTLY bị từ chối trên bảng cha phân mảnh và trong giao dịch (core/migrate)"},
		{"alter table", "sửa một bảng có sẵn — tệp này chỉ thêm chỉ mục"},
		{"drop ", "xoá một đối tượng — lệnh đảo ngược chỉ nằm trong chú thích REVERSAL"},
		{"insert ", "ghi dòng"},
		{"update ", "sửa dòng"},
		{"delete ", "xoá dòng"},
		{"trigger", "đụng trigger canh lưu trữ"},
		{"function", "đụng hàm canh lưu trữ"},
	} {
		if strings.Contains(sql, c.cam) {
			t.Errorf("0014 chứa %q — %s", c.cam, c.vi)
		}
	}
}
