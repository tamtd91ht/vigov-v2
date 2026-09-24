package store

import (
	"regexp"
	"strings"
	"testing"
)

// THE SOFT DELETE OF #10, read as the statement it is.
//
// WHY A TEST ON THE STATEMENT TEXT: this package's fake engines have no transactions, and the one
// property that matters here — "a deleted row can never stay published" — lives in LITERALS inside
// the SET list rather than in anything a caller passes. The behaviour against a real server (the
// 0010 CHECKs accepting the row, the columns read back) is proven in
// app/danh_ba_can_bo_xoa_pg_test.go, which runs when VIGOV_TEST_DSN is set.

// setCua returns the SET list of an UPDATE, whitespace collapsed.
func setCua(t *testing.T, stmt string) string {
	t.Helper()
	gon := strings.Join(strings.Fields(stmt), " ")
	m := regexp.MustCompile(`(?i)\bSET (.*) WHERE `).FindStringSubmatch(gon)
	if m == nil {
		t.Fatalf("không tìm thấy SET … WHERE trong: %s", gon)
	}
	return m[1]
}

// MUTATION THAT MUST TURN THIS RED: drop any of the three publication literals from xoaMemCanBo.
// The row would then keep `hien_tren_mini_app = true` after the delete, and the day a public Mini
// App read forgets `deleted_at IS NULL`, a removed person's personal mobile is on a public channel.
func TestXoaMemGoCongKhaiMiniAppTrongCungCauUpdate(t *testing.T) {
	set := setCua(t, xoaMemCanBo)
	for _, can := range []string{
		"hien_tren_mini_app = false",
		"dong_y_cong_khai_luc = NULL",
		"dong_y_cong_khai_ghi_boi = ''",
		"deleted_at = $3",
		"deleted_by = $4",
		"delete_reason = $5",
	} {
		if !strings.Contains(set, can) {
			t.Errorf("câu xoá mềm thiếu %q trong SET: %s", can, set)
		}
	}
}

// AN ISSUED CODE AND THE NAME STAY EXACTLY AS THEY WERE: `ma` is never reissued or renumbered (rule
// 7, invariant 3), and `ho_ten` is what ResolveStaffNames prints beside the old records that name
// this person (ADR 0034). Neither may be reachable from the delete.
func TestXoaMemKhongChamMaVaHoTen(t *testing.T) {
	set := setCua(t, xoaMemCanBo)
	for _, cam := range []string{"ma =", "ho_ten", "email", "co_tai_khoan", "mat_khau_hash", "vai_tro_id"} {
		if regexp.MustCompile(`(^|[ ,])` + regexp.QuoteMeta(cam)).MatchString(set) {
			t.Errorf("câu xoá mềm ghi cột %q: %s", cam, set)
		}
	}
}

// THE COMMUNE IS $1 AND A SECOND DELETE CANNOT OVERWRITE WHO REMOVED THE ROW, OR WHY.
func TestXoaMemTheoXaVaChiDongChuaXoa(t *testing.T) {
	gon := strings.Join(strings.Fields(xoaMemCanBo), " ")
	for _, can := range []string{"tenant_id = $1", "id = $2", "deleted_at IS NULL"} {
		if !strings.Contains(gon, "WHERE") || !strings.Contains(gon[strings.Index(gon, "WHERE"):], can) {
			t.Errorf("mệnh đề WHERE thiếu %q: %s", can, gon)
		}
	}
}
