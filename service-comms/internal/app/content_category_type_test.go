package app

// Migration 0030 in the use cases (owner, 14/10/2026): ONE CATEGORY TREE PER CONTENT TYPE, and a banner
// has no category. Over the real stores and the fake driver of noi_dung_mini_app_test.go, so what is
// proven is what reaches the SQL and the transaction — a refusal writes nothing and commits nothing.

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// wroteNothing fails the test when a refused act left any write or commit behind.
func wroteNothing(t *testing.T, k *khoNDGia, what string) {
	t.Helper()
	for _, stmt := range []string{"INSERT INTO noi_dung_mini_app", "UPDATE noi_dung_mini_app",
		"INSERT INTO danh_muc_mini_app", "UPDATE danh_muc_mini_app", "INSERT INTO audit_log"} {
		if k.coCau(stmt) {
			t.Errorf("%s: refused, yet %q ran", what, stmt)
		}
	}
	if k.daCommit != 0 {
		t.Errorf("%s: refused, yet committed", what)
	}
}

// --- the category create -------------------------------------------------------------------------

func TestCreateCategoryWritesItsTypeAndAuditsIt(t *testing.T) {
	k := &khoNDGia{}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	got, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Type: "su-kien", Ten: "Lễ hội"}, nguoiSoanND())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.Type != domain.LoaiSuKien {
		t.Errorf("returned type = %q", got.Type)
	}
	ins := k.cau("INSERT INTO danh_muc_mini_app")
	// $1 xã, $2 id, $3 tên, $4 slug, $5 cha, $6 thứ tự, $7 loại (migration 0030 — no DEFAULT behind it).
	if len(ins) != 1 || len(ins[0].args) != 7 || ins[0].args[6] != "su-kien" {
		t.Fatalf("INSERT = %+v", ins)
	}
	delta := decodedAuditDelta(t, k, HanhViThemDanhMucMiniApp)
	// Migration 0030's REVERSAL check looks for `delta->'sau' ? 'loai'` on staff-written entries.
	if after, _ := delta["sau"].(map[string]any); after["loai"] != "su-kien" {
		t.Errorf("delta sau = %v, want loai su-kien", after)
	}
}

func TestCreateCategoryWithoutTypeIsRefusedBeforeAnyTransaction(t *testing.T) {
	for _, typ := range []string{"", "banner", "news"} {
		k := &khoNDGia{}
		_, ucDM, ctx := dungUseCaseNoiDung(t, k)
		if _, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Type: typ, Ten: "Lễ hội"}, nguoiSoanND()); !errors.Is(err, domain.ErrCategoryTypeInvalid) {
			t.Errorf("type %q: err = %v", typ, err)
		}
		if k.batDau != 0 || len(k.lenh) != 0 {
			t.Errorf("type %q: refused, yet a transaction opened", typ)
		}
	}
}

// The same name in two types is two categories (names are unique per (type, parent)): only the new
// category's own tree is read for the name check.
func TestSameCategoryNameInTwoTypesIsAllowed(t *testing.T) {
	k := &khoNDGia{liveNamesByType: map[domain.LoaiNoiDung][]domain.CategoryName{
		domain.LoaiTinTuc: {{ID: "dm-news", Ten: "Nông nghiệp"}},
	}}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	if _, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Type: "video", Ten: "Nông nghiệp"}, nguoiSoanND()); err != nil {
		t.Fatalf("same name, other type: %v", err)
	}
	reads := k.cau("SELECT id, ten, COALESCE(cha_id, '') FROM danh_muc_mini_app")
	if len(reads) != 1 || reads[0].args[1] != "video" {
		t.Fatalf("name check read %+v, want the video tree only", reads)
	}

	// …and within the SAME type it is still a collision.
	k = &khoNDGia{liveNamesByType: map[domain.LoaiNoiDung][]domain.CategoryName{
		domain.LoaiTinTuc: {{ID: "dm-news", Ten: "Nông nghiệp"}},
	}}
	_, ucDM, ctx = dungUseCaseNoiDung(t, k)
	if _, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Type: "tin-tuc", Ten: "nông nghiệp"}, nguoiSoanND()); !errors.Is(err, domain.ErrCategoryNameTaken) {
		t.Fatalf("same name, same type: err = %v", err)
	}
	wroteNothing(t, k, "same name, same type")
}

func TestCreateCategoryUnderAParentOfAnotherTypeIsRefused(t *testing.T) {
	k := &khoNDGia{categoryTypes: map[string]domain.LoaiNoiDung{"dm-news": domain.LoaiTinTuc}}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	_, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Type: "su-kien", Ten: "Lễ hội", ChaID: "dm-news"}, nguoiSoanND())
	if !errors.Is(err, domain.ErrCategoryTypeMismatch) {
		t.Fatalf("err = %v, want ErrCategoryTypeMismatch", err)
	}
	wroteNothing(t, k, "parent of another type")

	// A parent of the same type is accepted.
	k = &khoNDGia{categoryTypes: map[string]domain.LoaiNoiDung{"dm-ev": domain.LoaiSuKien}}
	_, ucDM, ctx = dungUseCaseNoiDung(t, k)
	if _, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Type: "su-kien", Ten: "Lễ hội", ChaID: "dm-ev"}, nguoiSoanND()); err != nil {
		t.Fatalf("same-type parent: %v", err)
	}
}

// --- the category edit ---------------------------------------------------------------------------

func TestMoveCategoryUnderAParentOfAnotherTypeIsRefused(t *testing.T) {
	moving := categoryNow()
	moving.Type = domain.LoaiSuKien
	k := &khoNDGia{category: moving, categoryTypes: map[string]domain.LoaiNoiDung{"dm-news": domain.LoaiTinTuc},
		ancestors: []string{"dm-news"}}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	parent := "dm-news"
	if _, err := ucDM.Update(ctx, "dm-1", domain.ContentCategoryUpdate{ChaID: &parent}, nguoiSoanND()); !errors.Is(err, domain.ErrCategoryTypeMismatch) {
		t.Fatalf("err = %v, want ErrCategoryTypeMismatch", err)
	}
	wroteNothing(t, k, "move across trees")
}

// A rename reads the category's OWN tree for siblings, never another type's.
func TestRenameCategoryChecksNamesInItsOwnTreeOnly(t *testing.T) {
	cat := categoryNow()
	cat.Type = domain.LoaiVideo
	k := &khoNDGia{category: cat, liveNamesByType: map[domain.LoaiNoiDung][]domain.CategoryName{
		domain.LoaiTinTuc: {{ID: "dm-news", Ten: "Giáo dục"}},
		domain.LoaiVideo:  {{ID: "dm-1", Ten: cat.Ten}},
	}}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	name := "Giáo dục"
	if _, err := ucDM.Update(ctx, "dm-1", domain.ContentCategoryUpdate{Ten: &name}, nguoiSoanND()); err != nil {
		t.Fatalf("rename to a name used only in another type: %v", err)
	}
	if !k.coCau("UPDATE danh_muc_mini_app") {
		t.Error("the rename was not written")
	}
}

// --- the item create -----------------------------------------------------------------------------

func TestCreateItemUnderACategoryOfAnotherTypeIsRefused(t *testing.T) {
	k := &khoNDGia{categoryTypes: map[string]domain.LoaiNoiDung{"dm-ev": domain.LoaiSuKien}}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	yc := ycThemMau() // tin-tuc
	yc.DanhMucID = "dm-ev"
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); !errors.Is(err, domain.ErrCategoryTypeMismatch) {
		t.Fatalf("err = %v, want ErrCategoryTypeMismatch", err)
	}
	wroteNothing(t, k, "item under another type's category")

	// The same item under a category of its own type is filed.
	k = &khoNDGia{categoryTypes: map[string]domain.LoaiNoiDung{"dm-news": domain.LoaiTinTuc}}
	uc, _, ctx = dungUseCaseNoiDung(t, k)
	yc.DanhMucID = "dm-news"
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); err != nil {
		t.Fatalf("same-type category: %v", err)
	}
	if ins := k.cau("INSERT INTO noi_dung_mini_app"); len(ins) != 1 || ins[0].args[3] != "dm-news" {
		t.Errorf("INSERT danh_muc_id = %+v", ins)
	}
}

func TestCreateBannerWithCategoryIsRefusedBeforeAnyTransaction(t *testing.T) {
	k := &khoNDGia{coDong: true}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	yc := ycThemMau()
	yc.Loai, yc.DanhMucID = "banner", "dm-news"
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); !errors.Is(err, domain.ErrBannerHasNoCategory) {
		t.Fatalf("err = %v, want ErrBannerHasNoCategory", err)
	}
	if k.batDau != 0 || len(k.lenh) != 0 {
		t.Error("refused, yet a transaction opened")
	}
}

// --- the item edit -------------------------------------------------------------------------------

func filedNews() *domain.NoiDungMiniApp {
	row := draftRow()
	row.DanhMucID = "dm-news"
	return row
}

// A type change with no category in the request leaves the item UNFILED in the same UPDATE, and the trail
// shows the category before and after.
func TestTypeChangeWithoutCategoryClearsTheCategory(t *testing.T) {
	k := &khoNDGia{dongHienCo: filedNews()}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	video := "video"
	got, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{Loai: &video}, nguoiSoanND())
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got.DanhMucID != "" {
		t.Errorf("category after type change = %q, want none", got.DanhMucID)
	}
	upd := k.cau("UPDATE noi_dung_mini_app")
	if len(upd) != 1 || upd[0].args[2] != "video" || upd[0].args[3] != nil {
		t.Fatalf("UPDATE loai/danh_muc_id = %v / %v, want video / NULL", upd[0].args[2], upd[0].args[3])
	}
	// The old category is not probed: clearing needs no category at all.
	if k.coCau("SELECT loai FROM danh_muc_mini_app") {
		t.Error("a cleared category was still probed")
	}
	delta := decodedAuditDelta(t, k, HanhViSuaNoiDungMiniApp)
	before, _ := delta["truoc"].(map[string]any)
	after, _ := delta["sau"].(map[string]any)
	if before["danh_muc_id"] != "dm-news" || after["danh_muc_id"] != "" || after["loai"] != "video" {
		t.Errorf("delta truoc/sau = %v / %v", before, after)
	}
}

// A type change naming a category of the NEW type files it there; naming the OLD type's category is refused.
func TestTypeChangeWithCategoryMustMatchTheNewType(t *testing.T) {
	types := map[string]domain.LoaiNoiDung{"dm-news": domain.LoaiTinTuc, "dm-vid": domain.LoaiVideo}
	video := "video"

	k := &khoNDGia{dongHienCo: filedNews(), categoryTypes: types}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	cat := "dm-vid"
	if _, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{Loai: &video, DanhMucID: &cat}, nguoiSoanND()); err != nil {
		t.Fatalf("new type, matching category: %v", err)
	}
	if upd := k.cau("UPDATE noi_dung_mini_app"); upd[0].args[3] != "dm-vid" {
		t.Errorf("UPDATE danh_muc_id = %v", upd[0].args[3])
	}

	k = &khoNDGia{dongHienCo: filedNews(), categoryTypes: types}
	uc, _, ctx = dungUseCaseNoiDung(t, k)
	old := "dm-news" // re-sent unchanged, but the type moved: it must now match the new type
	if _, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{Loai: &video, DanhMucID: &old}, nguoiSoanND()); !errors.Is(err, domain.ErrCategoryTypeMismatch) {
		t.Fatalf("new type, old category: err = %v", err)
	}
	wroteNothing(t, k, "type change keeping the old category")
}

func TestEditIntoAnotherTypesCategoryIsRefused(t *testing.T) {
	k := &khoNDGia{dongHienCo: filedNews(), categoryTypes: map[string]domain.LoaiNoiDung{"dm-ev": domain.LoaiSuKien}}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	cat := "dm-ev"
	if _, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{DanhMucID: &cat}, nguoiSoanND()); !errors.Is(err, domain.ErrCategoryTypeMismatch) {
		t.Fatalf("err = %v, want ErrCategoryTypeMismatch", err)
	}
	wroteNothing(t, k, "edit into another type's category")
}

func TestEditGivingABannerACategoryIsRefused(t *testing.T) {
	b := draftRow()
	b.Loai, b.CoverImageFileID = domain.LoaiBanner, ""
	k := &khoNDGia{dongHienCo: b, coDong: true}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	cat := "dm-news"
	if _, err := uc.Sua(ctx, b.ID, domain.YeuCauSuaNoiDung{DanhMucID: &cat}, nguoiSoanND()); !errors.Is(err, domain.ErrBannerHasNoCategory) {
		t.Fatalf("err = %v, want ErrBannerHasNoCategory", err)
	}
	wroteNothing(t, k, "banner given a category")
}

// Migration 0030 left rows filed across trees as they were (question 4). An edit that touches neither the
// type nor the category must not be refused because of it — even when the client re-sends the category.
func TestUnrelatedEditOfARowFiledAcrossTreesIsNotRefused(t *testing.T) {
	legacy := filedNews() // tin-tuc under dm-ev, a su-kien category
	legacy.DanhMucID = "dm-ev"
	k := &khoNDGia{dongHienCo: legacy, categoryTypes: map[string]domain.LoaiNoiDung{"dm-ev": domain.LoaiSuKien}}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	title, same := "Tiêu đề mới", "dm-ev"
	if _, err := uc.Sua(ctx, legacy.ID, domain.YeuCauSuaNoiDung{TieuDe: &title, DanhMucID: &same}, nguoiSoanND()); err != nil {
		t.Fatalf("unrelated edit refused: %v", err)
	}
	if upd := k.cau("UPDATE noi_dung_mini_app"); len(upd) != 1 || upd[0].args[3] != "dm-ev" {
		t.Errorf("UPDATE = %+v — the filing must be left as it was", upd)
	}

	// A legacy BANNER filed under a category: an unrelated edit leaves it filed, not refused.
	b := draftRow()
	b.Loai, b.CoverImageFileID, b.DanhMucID = domain.LoaiBanner, "01JCOVER", "dm-news"
	k = &khoNDGia{dongHienCo: b}
	uc, _, ctx = dungUseCaseNoiDung(t, k)
	if _, err := uc.Sua(ctx, b.ID, domain.YeuCauSuaNoiDung{TieuDe: &title}, nguoiSoanND()); err != nil {
		t.Fatalf("legacy banner edit refused: %v", err)
	}
	if upd := k.cau("UPDATE noi_dung_mini_app"); len(upd) != 1 || upd[0].args[3] != "dm-news" {
		t.Errorf("UPDATE = %+v — an unrelated edit must not refile the banner", upd)
	}
	if k.coCau("SELECT loai FROM danh_muc_mini_app") {
		t.Error("an unrelated edit still probed the category")
	}
}

// A category that is not live answers `category_missing` before its type is ever compared.
func TestCreateItemUnderAMissingCategoryIsStillCategoryMissing(t *testing.T) {
	k := &khoNDGia{}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	yc := ycThemMau()
	yc.DanhMucID = "dm-gone"
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); !errors.Is(err, commsstore.ErrDanhMucKhongTonTaiMiniApp) {
		t.Fatalf("err = %v, want ErrDanhMucKhongTonTaiMiniApp", err)
	}
}
