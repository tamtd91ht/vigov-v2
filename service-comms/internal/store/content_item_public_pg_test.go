package store

import (
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
)

// The public reads against a REAL PostgreSQL. Skipped without VIGOV_TEST_DSN (shared harness:
// map_asset_type_pg_test.go). EACH ROW MAKES ONE DEFECT VISIBLE:
//
//	cong-khai     xã 1, dang-hien           must appear
//	an            xã 1, an                  drop the state clause → appears
//	cho-duyet     xã 1, cho-duyet           drop the state clause → appears
//	da-xoa        xã 1, dang-hien, deleted  drop `deleted_at IS NULL` → appears
//	cua-xa-2      xã 2, dang-hien           drop the commune → appears in xã 1
func TestPgPublicContentOnlyVisibleOfOneTenant(t *testing.T) {
	t1, t2 := uniqueTenants(t)
	at := time.Now().UTC()
	insertContentRow(t, t1, "cong-khai", "tin-tuc", "Tin đã đăng", "dang-hien", "thu-cong", "", at)
	insertContentRow(t, t1, "an", "tin-tuc", "Bản nháp", "an", "thu-cong", "", at.Add(time.Second))
	insertContentRow(t, t1, "cho-duyet", "tin-tuc", "Chờ duyệt", "cho-duyet", "thu-cong", "", at.Add(2*time.Second))
	insertContentRow(t, t1, "da-xoa", "tin-tuc", "Đã xoá", "dang-hien", "thu-cong", "", at.Add(3*time.Second))
	insertContentRow(t, t2, "cua-xa-2", "tin-tuc", "Tin của xã 2", "dang-hien", "thu-cong", "", at)
	if _, err := openTestDB(t).Exec(
		`UPDATE noi_dung_mini_app SET deleted_at = now(), deleted_by = 'CB-TEST', delete_reason = 'thử'
		  WHERE tenant_id = $1 AND id = 'da-xoa'`, t1); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	s := newRealContentItemStore(t)
	res, err := s.ListPublic(tenantCtx(tenant.ID(t1)), "", realContentFirstPage(t))
	if err != nil {
		t.Fatalf("đọc trang công khai: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "cong-khai" {
		var ids []string
		for _, c := range res.Items {
			ids = append(ids, c.ID)
		}
		t.Fatalf("trang công khai xã 1 = %v, muốn đúng [cong-khai]", ids)
	}

	if _, err := s.PublicByID(tenantCtx(tenant.ID(t1)), "cong-khai"); err != nil {
		t.Fatalf("chi tiết đã đăng: %v", err)
	}
	for _, id := range []string{"an", "cho-duyet", "da-xoa", "cua-xa-2"} {
		if _, err := s.PublicByID(tenantCtx(tenant.ID(t1)), id); !errors.Is(err, ErrContentItemNotFound) {
			t.Errorf("chi tiết %s ở xã 1: lỗi = %v, muốn ErrContentItemNotFound", id, err)
		}
	}
}

// The `type` filter against a real database. EACH ROW MAKES ONE DEFECT VISIBLE:
//
//	su-kien       xã 1, su-kien, dang-hien  must appear
//	tin-tuc       xã 1, tin-tuc, dang-hien  drop the type clause → appears
//	su-kien-an    xã 1, su-kien, an         the filter replaced the state clause → appears
//	su-kien-xa-2  xã 2, su-kien, dang-hien  drop the commune → appears in xã 1
func TestPgPublicContentFiltersByType(t *testing.T) {
	c1, c2 := uniqueTenants(t)
	at := time.Now().UTC()
	insertContentRow(t, c1, "su-kien", "su-kien", "Sự kiện đã đăng", "dang-hien", "thu-cong", "", at)
	insertContentRow(t, c1, "tin-tuc", "tin-tuc", "Tin đã đăng", "dang-hien", "thu-cong", "", at.Add(time.Second))
	insertContentRow(t, c1, "su-kien-an", "su-kien", "Sự kiện nháp", "an", "thu-cong", "", at.Add(2*time.Second))
	insertContentRow(t, c2, "su-kien-xa-2", "su-kien", "Sự kiện xã 2", "dang-hien", "thu-cong", "", at)

	res, err := newRealContentItemStore(t).ListPublic(tenantCtx(tenant.ID(c1)), "su-kien", realContentFirstPage(t))
	if err != nil {
		t.Fatalf("đọc trang công khai lọc loại: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "su-kien" {
		var ids []string
		for _, it := range res.Items {
			ids = append(ids, it.ID)
		}
		t.Fatalf("trang công khai su-kien xã 1 = %v, muốn đúng [su-kien]", ids)
	}
}
