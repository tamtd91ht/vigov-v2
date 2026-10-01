package store

import (
	"errors"
	"strings"
	"testing"
)

// The public reads (noi_dung_cong_khai.go), over the SAME fake driver as the staff register. It proves
// the statement: commune bound to $1 from the context, soft-deleted rows excluded, the state bound to
// `dang-hien`, no body on the page. That the database actually returns only published rows of one
// commune is the pg suite's (noi_dung_cong_khai_pg_test.go).

func TestDanhSachCongKhaiChiDangHienBuocXa(t *testing.T) {
	k := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	kho, ctx := khoNoiDung(t, k)

	kq, err := kho.DanhSachCongKhai(ctx, "", "", trangDauND(t))
	if err != nil {
		t.Fatalf("đọc trang công khai lỗi: %v", err)
	}
	if len(kq.Items) != 1 || kq.Items[0].NoiDung != "" {
		t.Fatalf("trang công khai = %+v — một dòng, không mang toàn văn", kq.Items)
	}
	l := k.lenh[0]
	// MUTATIONS THAT MUST TURN THIS RED: drop the state clause (every draft is published), drop the
	// soft-delete clause, or bind the state to anything but `dang-hien`.
	for _, muon := range []string{"tenant_id = $1", "deleted_at IS NULL", "trang_thai = $2",
		"ORDER BY tao_luc DESC, id DESC"} {
		if !strings.Contains(l.sql, muon) {
			t.Errorf("câu đọc trang công khai thiếu %q: %s", muon, l.sql)
		}
	}
	if l.args[0] != string(xaMotND) || l.args[1] != "dang-hien" {
		t.Fatalf("tham số = %v, muốn [xã của ngữ cảnh, dang-hien, …]", l.args)
	}
	if cot := l.sql[:strings.Index(l.sql, " FROM ")]; strings.Contains(cot, "noi_dung") {
		t.Errorf("trang công khai chọn cả `noi_dung`: %s", cot)
	}
	if strings.Contains(l.sql, "loai =") {
		t.Errorf("không lọc loại mà câu vẫn có `loai =`: %s", l.sql)
	}
	// ADR 0067 §5 decision 5: the default list never carries banners.
	if !strings.Contains(l.sql, "loai <> $3") || l.args[2] != "banner" {
		t.Errorf("danh sách mặc định phải loại banner (`loai <> $3` = banner): %s %v", l.sql, l.args)
	}
}

func TestPublicContentListTypeFilterKeepsStateAndCommune(t *testing.T) {
	fake := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	repo, ctx := khoNoiDung(t, fake)

	if _, err := repo.DanhSachCongKhai(ctx, "su-kien", "", trangDauND(t)); err != nil {
		t.Fatalf("đọc trang công khai lọc loại lỗi: %v", err)
	}
	stmt := fake.lenh[0]
	// MUTATIONS THAT MUST TURN THIS RED: the type clause replaces the state clause, or binds the type
	// to the state's placeholder.
	for _, want := range []string{"tenant_id = $1", "deleted_at IS NULL", "trang_thai = $2", "loai = $3"} {
		if !strings.Contains(stmt.sql, want) {
			t.Errorf("câu đọc trang công khai lọc loại thiếu %q: %s", want, stmt.sql)
		}
	}
	if stmt.args[0] != string(xaMotND) || stmt.args[1] != "dang-hien" || stmt.args[2] != "su-kien" {
		t.Fatalf("tham số = %v, muốn [xã, dang-hien, su-kien, …]", stmt.args)
	}
}

func TestPublicContentListCategoryFilterWalksLiveSubtreeOfThisCommune(t *testing.T) {
	fake := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	repo, ctx := khoNoiDung(t, fake)

	kq, err := repo.DanhSachCongKhai(ctx, "su-kien", "dm-cha", trangDauND(t))
	if err != nil {
		t.Fatalf("đọc trang công khai lọc danh mục lỗi: %v", err)
	}
	// The fake read the CONTENT row — the subquery on danh_muc_mini_app did not make it a category read.
	if len(kq.Items) != 1 || kq.Items[0].ID != "nd-001" {
		t.Fatalf("trang = %+v", kq.Items)
	}
	stmt := fake.lenh[0]
	// MUTATIONS THAT MUST TURN THIS RED: the seed row unscoped (another commune's id seeds the walk), the
	// recursive step unscoped, deleted categories walked, UNION ALL (a cycle hangs), the category bound
	// to the type's placeholder, or the state clause lost.
	for _, want := range []string{
		"tenant_id = $1", "deleted_at IS NULL", "trang_thai = $2", "loai = $3",
		"danh_muc_id IN (WITH RECURSIVE subtree(id) AS (",
		"WHERE c.tenant_id = $1 AND c.id = $4 AND c.deleted_at IS NULL UNION SELECT",
		"JOIN subtree s ON c.cha_id = s.id WHERE c.tenant_id = $1 AND c.deleted_at IS NULL",
	} {
		if !strings.Contains(stmt.sql, want) {
			t.Errorf("câu lọc danh mục thiếu %q: %s", want, stmt.sql)
		}
	}
	if strings.Contains(stmt.sql, "UNION ALL") {
		t.Errorf("UNION ALL: một chu trình sẽ lặp mãi: %s", stmt.sql)
	}
	if stmt.args[0] != string(xaMotND) || stmt.args[1] != "dang-hien" || stmt.args[2] != "su-kien" || stmt.args[3] != "dm-cha" {
		t.Fatalf("tham số = %v, muốn [xã, dang-hien, su-kien, dm-cha, …]", stmt.args)
	}

	// Category alone: the banner exclusion takes $3, the category $4.
	fake.lenh = nil
	if _, err := repo.DanhSachCongKhai(ctx, "", "dm-cha", trangDauND(t)); err != nil {
		t.Fatalf("lọc chỉ danh mục lỗi: %v", err)
	}
	if s := fake.lenh[0].sql; !strings.Contains(s, "loai <> $3") || !strings.Contains(s, "c.id = $4") ||
		strings.Contains(s, "loai =") {
		t.Errorf("chỉ lọc danh mục: %s", s)
	}
}

func TestPublishedCategoryIDsJoinsLiveCategoriesOfThisCommune(t *testing.T) {
	fake := &khoNDGia{categoriesWithItems: []string{"dm-a", "dm-b"}}
	repo, ctx := khoNoiDung(t, fake)

	ids, err := repo.PublishedCategoryIDs(ctx, "su-kien")
	if err != nil {
		t.Fatalf("đọc danh mục có tin lỗi: %v", err)
	}
	if strings.Join(ids, ",") != "dm-a,dm-b" {
		t.Fatalf("ids = %v", ids)
	}
	stmt := fake.lenh[0]
	// MUTATIONS THAT MUST TURN THIS RED: the joined table not constrained to $1 (ids of another commune
	// match), deleted categories or items counted, drafts counted, the type dropped, no bound.
	for _, want := range []string{
		"JOIN danh_muc_mini_app dm ON dm.tenant_id = $1 AND dm.id = nd.danh_muc_id AND dm.deleted_at IS NULL",
		"WHERE nd.tenant_id = $1 AND nd.deleted_at IS NULL AND nd.trang_thai = $2",
		"AND nd.loai = $3", "LIMIT $4",
	} {
		if !strings.Contains(stmt.sql, want) {
			t.Errorf("câu danh mục có tin thiếu %q: %s", want, stmt.sql)
		}
	}
	if stmt.args[0] != string(xaMotND) || stmt.args[1] != "dang-hien" || stmt.args[2] != "su-kien" ||
		stmt.args[3] != int64(TranDanhMucMiniApp+1) {
		t.Fatalf("tham số = %v", stmt.args)
	}

	fake.lenh = nil
	if _, err := repo.PublishedCategoryIDs(ctx, ""); err != nil {
		t.Fatalf("không lọc loại lỗi: %v", err)
	}
	// No type: every type but banner, the same set the default list shows.
	if s := fake.lenh[0].sql; !strings.Contains(s, "nd.loai <> $3") || !strings.Contains(s, "LIMIT $4") ||
		fake.lenh[0].args[2] != "banner" {
		t.Errorf("không lọc loại: %s", s)
	}
}

func TestPublishedCategoryIDsOverCapRefusesRatherThanTruncates(t *testing.T) {
	many := make([]string, TranDanhMucMiniApp+1)
	for i := range many {
		many[i] = "dm"
	}
	repo, ctx := khoNoiDung(t, &khoNDGia{categoriesWithItems: many})
	if ids, err := repo.PublishedCategoryIDs(ctx, ""); !errors.Is(err, ErrQuaNhieuDanhMucMiniApp) || ids != nil {
		t.Fatalf("vượt trần: ids = %d, lỗi = %v — muốn nil, ErrQuaNhieuDanhMucMiniApp", len(ids), err)
	}
}

func TestCongKhaiTheoIDChiDangHienBuocXa(t *testing.T) {
	k := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	kho, ctx := khoNoiDung(t, k)

	n, err := kho.CongKhaiTheoID(ctx, "nd-001")
	if err != nil {
		t.Fatalf("đọc chi tiết công khai lỗi: %v", err)
	}
	if n.NoiDung != "<p>Toàn văn</p>" {
		t.Fatalf("toàn văn = %q", n.NoiDung)
	}
	l := k.lenh[0]
	for _, muon := range []string{"tenant_id = $1", "id = $2", "deleted_at IS NULL", "trang_thai = $3"} {
		if !strings.Contains(l.sql, muon) {
			t.Errorf("câu đọc chi tiết công khai thiếu %q: %s", muon, l.sql)
		}
	}
	if l.args[0] != string(xaMotND) || l.args[1] != "nd-001" || l.args[2] != "dang-hien" {
		t.Fatalf("tham số = %v, muốn [xã, nd-001, dang-hien]", l.args)
	}
}

func TestCongKhaiTheoIDKhongCoDongThiBaoKhongTonTai(t *testing.T) {
	// A draft, another commune's id and no id at all are ONE answer: the predicate returns no row.
	kho, ctx := khoNoiDung(t, &khoNDGia{})
	if _, err := kho.CongKhaiTheoID(ctx, "nd-ban-nhap"); !errors.Is(err, ErrNoiDungKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrNoiDungKhongTonTai", err)
	}
}
