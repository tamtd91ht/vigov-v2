package store

import (
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
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
	kq, err := kho.DanhSachCongKhai(ctxXa(tenant.ID(xa1)), "", "", trangDauNDThat(t))
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

// fileUnder files an item under a category, directly (themNoiDungThat leaves danh_muc_id NULL).
func fileUnder(t *testing.T, tenantID, itemID, categoryID string) {
	t.Helper()
	if _, err := moKetNoi(t).Exec(
		`UPDATE noi_dung_mini_app SET danh_muc_id = $3 WHERE tenant_id = $1 AND id = $2`,
		tenantID, itemID, categoryID); err != nil {
		t.Fatalf("xếp %s vào %s: %v", itemID, categoryID, err)
	}
}

func softDeleteCategory(t *testing.T, tenantID, id string) {
	t.Helper()
	if _, err := moKetNoi(t).Exec(
		`UPDATE danh_muc_mini_app SET deleted_at = now(), deleted_by = 'CB-TEST', delete_reason = 'thử'
		  WHERE tenant_id = $1 AND id = $2`, tenantID, id); err != nil {
		t.Fatalf("xoá mềm danh mục %s: %v", id, err)
	}
}

// The `category` filter and the chip read against a real database — the recursive CTE, the join and
// every predicate only PostgreSQL can evaluate. TREE (xã 1):
//
//	cha ─ con ─ chau        con-xoa (deleted, child of cha) ─ duoi-xoa (live, under the deleted one)
//	rieng (no item)         cha exists in xã 2 too, under the SAME id
//
// ITEMS: at-cha (cha), at-con (con, su-kien), at-chau (chau), at-duoi-xoa (duoi-xoa), nhap-con (con, an),
// at-rieng-khong (none filed), xa2-cha (xã 2's cha). EACH MAKES ONE DEFECT VISIBLE:
//
//	parent does not include descendants   → at-con / at-chau missing under cha
//	deleted category walked               → at-duoi-xoa appears under cha
//	seed row unscoped                     → xa2-cha appears in xã 1, or xã 1's items under xã 2's id
//	state dropped                         → nhap-con appears
func TestPgPublicCategoryFilterAndChips(t *testing.T) {
	c1, c2 := xaRieng(t)
	at := time.Now().UTC()
	for _, c := range []struct{ id, parent string }{
		{"cha", ""}, {"con", "cha"}, {"chau", "con"}, {"con-xoa", "cha"}, {"duoi-xoa", "con-xoa"}, {"rieng", ""},
	} {
		themDanhMucThat(t, c1, c.id, "Tên "+c.id, "slug-"+c.id, c.parent, 0)
	}
	themDanhMucThat(t, c2, "cha", "Cha xã 2", "slug-cha", "", 0)
	softDeleteCategory(t, c1, "con-xoa")

	for i, it := range []struct{ id, typ, state, cat string }{
		{"at-cha", "tin-tuc", "dang-hien", "cha"},
		{"at-con", "su-kien", "dang-hien", "con"},
		{"at-chau", "tin-tuc", "dang-hien", "chau"},
		{"at-duoi-xoa", "tin-tuc", "dang-hien", "duoi-xoa"},
		{"nhap-con", "tin-tuc", "an", "con"},
		{"at-rieng-khong", "tin-tuc", "dang-hien", ""},
	} {
		themNoiDungThat(t, c1, it.id, it.typ, "Tiêu đề "+it.id, it.state, "thu-cong", "", at.Add(time.Duration(i)*time.Second))
		if it.cat != "" {
			fileUnder(t, c1, it.id, it.cat)
		}
	}
	themNoiDungThat(t, c2, "xa2-cha", "tin-tuc", "Tin xã 2", "dang-hien", "thu-cong", "", at)
	fileUnder(t, c2, "xa2-cha", "cha")

	repo := khoNoiDungThat(t)
	list := func(ctxTenant, typ, cat string) string {
		t.Helper()
		res, err := repo.DanhSachCongKhai(ctxXa(tenant.ID(ctxTenant)), domain.LoaiNoiDung(typ), cat, trangDauNDThat(t))
		if err != nil {
			t.Fatalf("lọc danh mục %s/%s: %v", typ, cat, err)
		}
		var ids []string
		for _, n := range res.Items {
			ids = append(ids, n.ID)
		}
		sort.Strings(ids)
		return strings.Join(ids, ",")
	}
	for _, c := range []struct{ tenant, typ, cat, want string }{
		{c1, "", "cha", "at-cha,at-chau,at-con"},
		{c1, "", "con", "at-chau,at-con"},
		{c1, "su-kien", "cha", "at-con"},
		{c1, "", "con-xoa", ""},
		{c1, "", "rieng", ""},
		{c1, "", "khong-co", ""},
		{c2, "", "cha", "xa2-cha"},
	} {
		if got := list(c.tenant, c.typ, c.cat); got != c.want {
			t.Errorf("xã %s type=%q category=%s: %q, muốn %q", c.tenant, c.typ, c.cat, got, c.want)
		}
	}

	chips := func(typ string) string {
		t.Helper()
		ctx := ctxXa(tenant.ID(c1))
		ids, err := repo.PublishedCategoryIDs(ctx, domain.LoaiNoiDung(typ))
		if err != nil {
			t.Fatalf("danh mục có tin: %v", err)
		}
		live, err := khoDanhMucNDThat(t).DanhSach(ctx)
		if err != nil {
			t.Fatalf("cây danh mục: %v", err)
		}
		var out []string
		for _, c := range domain.CategoriesWithPublishedItems(live, ids) {
			out = append(out, c.ID)
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	// `duoi-xoa` holds an item but sits under a deleted category: cut. `rieng` holds nothing: hidden.
	if got := chips(""); got != "cha,chau,con" {
		t.Errorf("chip mọi loại = %q, muốn cha,chau,con", got)
	}
	if got := chips("su-kien"); got != "cha,con" {
		t.Errorf("chip su-kien = %q, muốn cha,con", got)
	}
}

// The `type` filter against a real database. EACH ROW MAKES ONE DEFECT VISIBLE:
//
//	su-kien       xã 1, su-kien, dang-hien  must appear
//	tin-tuc       xã 1, tin-tuc, dang-hien  drop the type clause → appears
//	su-kien-an    xã 1, su-kien, an         the filter replaced the state clause → appears
//	su-kien-xa-2  xã 2, su-kien, dang-hien  drop the commune → appears in xã 1
func TestPgPublicContentFiltersByType(t *testing.T) {
	c1, c2 := xaRieng(t)
	at := time.Now().UTC()
	themNoiDungThat(t, c1, "su-kien", "su-kien", "Sự kiện đã đăng", "dang-hien", "thu-cong", "", at)
	themNoiDungThat(t, c1, "tin-tuc", "tin-tuc", "Tin đã đăng", "dang-hien", "thu-cong", "", at.Add(time.Second))
	themNoiDungThat(t, c1, "su-kien-an", "su-kien", "Sự kiện nháp", "an", "thu-cong", "", at.Add(2*time.Second))
	themNoiDungThat(t, c2, "su-kien-xa-2", "su-kien", "Sự kiện xã 2", "dang-hien", "thu-cong", "", at)

	res, err := khoNoiDungThat(t).DanhSachCongKhai(ctxXa(tenant.ID(c1)), "su-kien", "", trangDauNDThat(t))
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

// The view increment against the real schema (ADR 0047, row 02/10/2026). EACH ROW MAKES ONE DEFECT
// VISIBLE, as above, plus the two only a real database can show: the immutability trigger lets an
// increase through, and concurrent increments are not lost (the UPDATE's row lock, not Go, serialises).
func TestPgIncrementPublicViewCount(t *testing.T) {
	xa1, xa2 := xaRieng(t)
	db := moKetNoi(t)
	luc := time.Now().UTC()
	themNoiDungThat(t, xa1, "cong-khai", "tin-tuc", "Tin đã đăng", "dang-hien", "thu-cong", "", luc)
	themNoiDungThat(t, xa1, "an", "tin-tuc", "Bản nháp", "an", "thu-cong", "", luc)
	themNoiDungThat(t, xa1, "da-xoa", "tin-tuc", "Đã xoá", "dang-hien", "thu-cong", "", luc)
	themNoiDungThat(t, xa2, "cong-khai", "tin-tuc", "Tin xã 2 cùng id", "dang-hien", "thu-cong", "", luc)
	if _, err := db.Exec(
		`UPDATE noi_dung_mini_app SET deleted_at = now(), deleted_by = 'CB-TEST', delete_reason = 'thử'
		  WHERE tenant_id = $1 AND id = 'da-xoa'`, xa1); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}
	read := func(tenantID, id string) (int, time.Time) {
		t.Helper()
		var n int
		var updated time.Time
		if err := db.QueryRow(`SELECT luot_xem, cap_nhat_luc FROM noi_dung_mini_app WHERE tenant_id = $1 AND id = $2`,
			tenantID, id).Scan(&n, &updated); err != nil {
			t.Fatalf("đọc %s: %v", id, err)
		}
		return n, updated
	}
	_, updatedBefore := read(xa1, "cong-khai")

	kho := khoNoiDungThat(t)
	ctx := ctxXa(tenant.ID(xa1))
	n, err := kho.IncrementPublicViewCount(ctx, "cong-khai")
	if err != nil || n != 1 {
		t.Fatalf("tăng lượt xem tin đã đăng: n = %d, lỗi = %v — muốn 1, nil (trigger bất biến phải cho tăng)", n, err)
	}
	for _, id := range []string{"an", "da-xoa"} {
		if _, err := kho.IncrementPublicViewCount(ctx, id); !errors.Is(err, ErrNoiDungKhongTonTai) {
			t.Errorf("tăng lượt xem %s: lỗi = %v, muốn ErrNoiDungKhongTonTai", id, err)
		}
		if got, _ := read(xa1, id); got != 0 {
			t.Errorf("%s bị đếm: luot_xem = %d", id, got)
		}
	}

	// Concurrent views: every one counted.
	const parallel = 8
	errs := make(chan error, parallel)
	for i := 0; i < parallel; i++ {
		go func() {
			_, err := kho.IncrementPublicViewCount(ctx, "cong-khai")
			errs <- err
		}()
	}
	for i := 0; i < parallel; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("tăng song song: %v", err)
		}
	}
	got, updatedAfter := read(xa1, "cong-khai")
	if got != 1+parallel {
		t.Fatalf("luot_xem = %d sau %d lượt, muốn %d — mất lượt khi đồng thời", got, 1+parallel, 1+parallel)
	}
	if !updatedAfter.Equal(updatedBefore) {
		t.Errorf("cap_nhat_luc đổi %v → %v — một lượt xem không phải một lần sửa", updatedBefore, updatedAfter)
	}
	// The same id in commune 2 is another row, untouched (rule 1).
	if other, _ := read(xa2, "cong-khai"); other != 0 {
		t.Errorf("tin cùng id của xã 2 bị đếm: luot_xem = %d", other)
	}
}
