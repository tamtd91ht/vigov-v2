package store

import (
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
)

// The public reads against a REAL PostgreSQL. Skipped without VIGOV_TEST_DSN (shared harness:
// loai_tai_nguyen_ban_do_pg_test.go). EACH ROW MAKES ONE DEFECT VISIBLE:
//
//	cong-khai     xã 1, dang-hien           must appear
//	an            xã 1, an                  drop the state clause → appears
//	cho-duyet     xã 1, cho-duyet           drop the state clause → appears
//	da-xoa        xã 1, dang-hien, deleted  drop `deleted_at IS NULL` → appears
//	cua-xa-2      xã 2, dang-hien           drop the commune → appears in xã 1
func TestPgNoiDungCongKhaiChiDangHienCuaMotXa(t *testing.T) {
	xa1, xa2 := xaRieng(t)
	luc := time.Now().UTC()
	themNoiDungThat(t, xa1, "cong-khai", "tin-tuc", "Tin đã đăng", "dang-hien", "thu-cong", "", luc)
	themNoiDungThat(t, xa1, "an", "tin-tuc", "Bản nháp", "an", "thu-cong", "", luc.Add(time.Second))
	themNoiDungThat(t, xa1, "cho-duyet", "tin-tuc", "Chờ duyệt", "cho-duyet", "thu-cong", "", luc.Add(2*time.Second))
	themNoiDungThat(t, xa1, "da-xoa", "tin-tuc", "Đã xoá", "dang-hien", "thu-cong", "", luc.Add(3*time.Second))
	themNoiDungThat(t, xa2, "cua-xa-2", "tin-tuc", "Tin của xã 2", "dang-hien", "thu-cong", "", luc)
	if _, err := moKetNoi(t).Exec(
		`UPDATE noi_dung_mini_app SET deleted_at = now(), deleted_by = 'CB-TEST', delete_reason = 'thử'
		  WHERE tenant_id = $1 AND id = 'da-xoa'`, xa1); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	kho := khoNoiDungThat(t)
	kq, err := kho.DanhSachCongKhai(ctxXa(tenant.ID(xa1)), trangDauNDThat(t))
	if err != nil {
		t.Fatalf("đọc trang công khai: %v", err)
	}
	if len(kq.Items) != 1 || kq.Items[0].ID != "cong-khai" {
		var id []string
		for _, n := range kq.Items {
			id = append(id, n.ID)
		}
		t.Fatalf("trang công khai xã 1 = %v, muốn đúng [cong-khai]", id)
	}

	if _, err := kho.CongKhaiTheoID(ctxXa(tenant.ID(xa1)), "cong-khai"); err != nil {
		t.Fatalf("chi tiết đã đăng: %v", err)
	}
	for _, id := range []string{"an", "cho-duyet", "da-xoa", "cua-xa-2"} {
		if _, err := kho.CongKhaiTheoID(ctxXa(tenant.ID(xa1)), id); !errors.Is(err, ErrNoiDungKhongTonTai) {
			t.Errorf("chi tiết %s ở xã 1: lỗi = %v, muốn ErrNoiDungKhongTonTai", id, err)
		}
	}
}
