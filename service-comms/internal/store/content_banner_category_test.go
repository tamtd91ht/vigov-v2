package store

// Migration 0012 and ADR 0067 §3/§5 in SQL, over the fake driver of noi_dung_mini_app_test.go: the
// banner pair, the banner strip, the audio clearing on a type change, the share-locked category probes,
// and the category edit / soft delete / ancestor walk statements. What PostgreSQL decides (the CHECKs,
// the cover trigger, the advisory lock, the recursion) is the pg suite's.

import (
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

func TestBannerPairBindsInPositionAndReadsByName(t *testing.T) {
	k := &khoNDGia{}
	repo, ctx := khoNoiDung(t, k)
	order := 4
	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return repo.Chen(ctx, tx, domain.NoiDungMiniApp{
			ID: "nd-b", Loai: domain.LoaiBanner, TieuDe: "Banner", NgayDang: ngayMau,
			TrangThai: domain.TrangThaiAn, NguoiTaoMa: "CB-1", CoverImageFileID: "f1",
			LinkTo: "https://x.gov.vn", DisplayOrder: &order,
		})
	})
	if err != nil {
		t.Fatalf("chèn lỗi: %v", err)
	}
	args := k.lenh[0].args
	if args[17] != "https://x.gov.vn" || args[18] != int64(4) {
		t.Errorf("$18 link_to = %v, $19 display_order = %v", args[17], args[18])
	}

	row := dongNDMau()
	row.loai, row.linkTo, row.displayOrder = "banner", "/su-kien", int64(0)
	legacy := dongNDMau()
	legacy.id = "nd-legacy"
	k2 := &khoNDGia{dong: []dongNDMiniApp{row, legacy}}
	repo2, ctx2 := khoNoiDung(t, k2)
	result, err := repo2.DanhSach(ctx2, LocNoiDung{}, trangDauND(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	// display_order 0 is a real position, not NULL — the pointer is what keeps the two apart.
	if got := result.Items[0]; got.LinkTo != "/su-kien" || got.DisplayOrder == nil || *got.DisplayOrder != 0 {
		t.Errorf("cặp banner đọc sai: %+v", got)
	}
	if got := result.Items[1]; got.LinkTo != "" || got.DisplayOrder != nil {
		t.Errorf("NULL phải đọc thành rỗng / nil: %+v", got)
	}
}

func TestUpdateWritesBannerPairAndClearsAudioOffBroadcasts(t *testing.T) {
	k := &khoNDGia{soDongDoi: 1}
	repo, ctx := khoNoiDung(t, k)
	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return repo.CapNhat(ctx, tx, domain.NoiDungMiniApp{ID: "nd-001", Loai: domain.LoaiTinTuc, TieuDe: "T"})
	})
	if err != nil {
		t.Fatalf("cập nhật lỗi: %v", err)
	}
	stmt := k.lenh[0].sql
	for _, want := range []string{"link_to = $17", "display_order = $18",
		"audio_file_id = CASE WHEN $3 = 'truyen-thanh' THEN audio_file_id END",
		"audio_duration_seconds = CASE WHEN $3 = 'truyen-thanh' THEN audio_duration_seconds END"} {
		if !strings.Contains(stmt, want) {
			t.Errorf("câu cập nhật thiếu %q: %s", want, stmt)
		}
	}
}

func TestPublicBannersReadsPublishedBannersWithCoverInStripOrder(t *testing.T) {
	banner := dongNDMau()
	banner.id, banner.loai, banner.coverImageFileID = "nd-banner", "banner", "01JCOVER"
	banner.linkTo, banner.displayOrder = "/tin-tuc", int64(2)
	k := &khoNDGia{dong: []dongNDMiniApp{banner}}
	repo, ctx := khoNoiDung(t, k)

	got, err := repo.PublicBanners(ctx, 20)
	if err != nil {
		t.Fatalf("đọc dải banner lỗi: %v", err)
	}
	if len(got) != 1 || got[0].LinkTo != "/tin-tuc" || got[0].DisplayOrder == nil || *got[0].DisplayOrder != 2 {
		t.Fatalf("banner = %+v", got)
	}
	stmt := k.lenh[0]
	// MUTATIONS THAT MUST TURN THIS RED: drafts shown, coverless legacy banners drawn as empty slots,
	// another type mixed in, the order reversed or NULLs first, no bound.
	for _, want := range []string{"tenant_id = $1", "deleted_at IS NULL", "trang_thai = $2", "loai = $3",
		"cover_image_file_id IS NOT NULL", "ORDER BY display_order ASC NULLS LAST, id", "LIMIT $4"} {
		if !strings.Contains(stmt.sql, want) {
			t.Errorf("câu dải banner thiếu %q: %s", want, stmt.sql)
		}
	}
	if stmt.args[1] != "dang-hien" || stmt.args[2] != "banner" || stmt.args[3] != int64(21) {
		t.Errorf("tham số = %v, muốn [xã, dang-hien, banner, 21]", stmt.args)
	}
	if cols := stmt.sql[:strings.Index(stmt.sql, " FROM ")]; strings.Contains(cols, "noi_dung,") {
		t.Errorf("dải banner không mang toàn văn: %s", cols)
	}
}

func TestCategoryProbesTakeAShareLock(t *testing.T) {
	// A soft delete counting "no live items / children" must not be raced by a write filing under the row.
	k := &khoNDGia{coDong: true}
	repo, ctx := khoNoiDung(t, k)
	if _, err := chayTrongGiaoDichCoKetQua(t, k, ctx, func(tx *pkgstore.ScopedTx) (bool, error) {
		return repo.DanhMucCoThat(ctx, tx, "dm-001")
	}); err != nil {
		t.Fatal(err)
	}
	categories, ctx2 := khoDanhMucND(t, k)
	if _, err := chayTrongGiaoDichCoKetQua(t, k, ctx2, func(tx *pkgstore.ScopedTx) (bool, error) {
		return categories.ChaCoThat(ctx2, tx, "dm-001")
	}); err != nil {
		t.Fatal(err)
	}
	probes := 0
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "SELECT 1 FROM danh_muc_mini_app") {
			probes++
			if !strings.HasSuffix(l.sql, "FOR SHARE") {
				t.Errorf("thăm dò danh mục thiếu FOR SHARE: %s", l.sql)
			}
		}
	}
	if probes != 2 {
		t.Errorf("số câu thăm dò = %d, muốn 2", probes)
	}
}

func TestCategoryUpdateNeverTouchesSlugOrDeletion(t *testing.T) {
	k := &khoNDGia{soDongDoi: 1}
	repo, ctx := khoDanhMucND(t, k)
	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return repo.Update(ctx, tx, domain.DanhMucMiniApp{ID: "dm-1", Ten: "Y tế", Slug: "khong-duoc-ghi", ThuTu: 2, Hidden: true})
	})
	if err != nil {
		t.Fatalf("cập nhật danh mục lỗi: %v", err)
	}
	l := k.lenh[0]
	for _, banned := range []string{"slug", "deleted_at =", "deleted_by", "delete_reason", "tao_luc"} {
		if strings.Contains(l.sql, banned) {
			t.Errorf("câu sửa danh mục không được chạm %q: %s", banned, l.sql)
		}
	}
	if !strings.Contains(l.sql, "hidden = $6") || !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("câu sửa danh mục: %s", l.sql)
	}
	if l.args[0] != string(xaMotND) || l.args[3] != nil || l.args[5] != true {
		t.Errorf("tham số = %v — cha rỗng phải là NULL, xã từ giao dịch", l.args)
	}

	// Zero rows touched (deleted meanwhile) is "not found", never a silent success.
	k0 := &khoNDGia{soDongDoi: 0}
	repo0, ctx0 := khoDanhMucND(t, k0)
	err = chayTrongGiaoDich(t, k0, ctx0, func(tx *pkgstore.ScopedTx) error {
		return repo0.Update(ctx0, tx, domain.DanhMucMiniApp{ID: "dm-x", Ten: "x"})
	})
	if !errors.Is(err, ErrDanhMucKhongTonTaiMiniApp) {
		t.Errorf("lỗi = %v", err)
	}
}

func TestCategorySoftDeleteCarriesWhoAndWhyAndNeverHardDeletes(t *testing.T) {
	k := &khoNDGia{soDongDoi: 1}
	repo, ctx := khoDanhMucND(t, k)
	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return repo.SoftDelete(ctx, tx, "dm-1", "CB-2026-7K3M9Q", "Gộp vào danh mục Y tế")
	})
	if err != nil {
		t.Fatalf("xoá mềm lỗi: %v", err)
	}
	l := k.lenh[0]
	if strings.Contains(l.sql, "DELETE") || !strings.HasPrefix(l.sql, "UPDATE danh_muc_mini_app") {
		t.Fatalf("phải là UPDATE, không bao giờ DELETE: %s", l.sql)
	}
	for _, want := range []string{"deleted_at = now()", "deleted_by = $3", "delete_reason = $4",
		"tenant_id = $1 AND id = $2 AND deleted_at IS NULL"} {
		if !strings.Contains(l.sql, want) {
			t.Errorf("câu xoá mềm thiếu %q: %s", want, l.sql)
		}
	}
	if l.args[2] != "CB-2026-7K3M9Q" || l.args[3] != "Gộp vào danh mục Y tế" {
		t.Errorf("tham số = %v", l.args)
	}
}

func TestCategoryAncestorWalkIsScopedAndIncludesDeletedRows(t *testing.T) {
	k := &khoNDGia{}
	repo, ctx := khoDanhMucND(t, k)
	_ = chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		_ = repo.LockTree(ctx, tx)
		_, _ = repo.AncestorChain(ctx, tx, "dm-cha-moi")
		_, _ = repo.CountLiveChildren(ctx, tx, "dm-1")
		_, _ = repo.CountLiveItems(ctx, tx, "dm-1")
		return nil
	})
	var lock, walk, children, items string
	for _, l := range k.lenh {
		switch {
		case strings.Contains(l.sql, "pg_advisory_xact_lock"):
			lock = l.sql
		case strings.Contains(l.sql, "WITH RECURSIVE up"):
			walk = l.sql
		case strings.Contains(l.sql, "count(*) FROM danh_muc_mini_app"):
			children = l.sql
		case strings.Contains(l.sql, "count(*) FROM noi_dung_mini_app"):
			items = l.sql
		}
	}
	if !strings.Contains(lock, "'danh_muc_mini_app:cay:' || $1") {
		t.Errorf("khoá cây phải theo xã: %q", lock)
	}
	// Both halves scoped; deleted rows walked on purpose (a deleted row's cha_id is still an edge);
	// UNION, so a cycle already in the data terminates.
	if strings.Count(walk, "tenant_id = $1") != 2 || strings.Contains(walk, "deleted_at") ||
		strings.Contains(walk, "UNION ALL") {
		t.Errorf("câu đi lên tổ tiên: %s", walk)
	}
	if !strings.Contains(children, "cha_id = $2 AND deleted_at IS NULL") {
		t.Errorf("đếm con: %s", children)
	}
	if !strings.Contains(items, "danh_muc_id = $2 AND deleted_at IS NULL") || strings.Contains(items, "trang_thai") {
		t.Errorf("đếm nội dung phải tính MỌI trạng thái còn sống: %s", items)
	}
}

func TestCategoryHiddenIsReadByName(t *testing.T) {
	k := &khoNDGia{danhMuc: []dongDMMiniApp{
		{id: "dm-1", ten: "Y tế", slug: "y-te", thuTu: 1, taoLuc: lucMauND, hidden: true},
		{id: "dm-2", ten: "Giáo dục", slug: "giao-duc", thuTu: 2, taoLuc: lucMauND},
	}}
	repo, ctx := khoDanhMucND(t, k)
	list, err := repo.DanhSach(ctx)
	if err != nil {
		t.Fatalf("đọc danh mục lỗi: %v", err)
	}
	if !list[0].Hidden || list[1].Hidden {
		t.Errorf("hidden đọc sai: %+v", list)
	}
}
