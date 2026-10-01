package app

// ADR 0067 §1, §3 and §5 in the use cases, over the same real stores and fake driver as
// noi_dung_mini_app_test.go: the body is sanitised on every write path, the banner rules are answered
// before anything is written, and the category edit / soft delete write the row and its trail in ONE
// transaction — or nothing at all.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-comms/internal/domain"
)

// decodedAuditDelta decodes the delta ($8) of the one audit entry the act wrote, and checks the actor ($2).
func decodedAuditDelta(t *testing.T, k *khoNDGia, action string) map[string]any {
	t.Helper()
	entries := k.cau("INSERT INTO audit_log")
	if len(entries) != 1 {
		t.Fatalf("số vết = %d, muốn 1", len(entries))
	}
	args := entries[0].args
	if args[1] != maCanBoSoanND {
		t.Errorf("chủ thể vết = %v, muốn mã cán bộ (luật 6, bất biến 8)", args[1])
	}
	if args[4] != action {
		t.Errorf("hành vi = %v, muốn %s", args[4], action)
	}
	raw, _ := args[7].([]byte)
	var delta map[string]any
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatalf("delta không phải JSON: %v (%q)", err, raw)
	}
	return delta
}

// --- §1: the body is sanitised on every write path ----------------------------------------------

func TestCreateStoresTheSanitisedBody(t *testing.T) {
	k := &khoNDGia{}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	yc := ycThemMau()
	yc.NoiDung = `<p onclick="x()">Bà con <strong>chú ý</strong></p><script>alert(1)</script>` +
		`<a href="javascript:alert(1)">bấm</a><a href="https://dichvucong.gov.vn">Cổng</a>`

	got, err := uc.Them(ctx, yc, nguoiSoanND())
	if err != nil {
		t.Fatalf("thêm lỗi: %v", err)
	}
	stored, _ := k.cau("INSERT INTO noi_dung_mini_app")[0].args[6].(string)
	for _, banned := range []string{"<script", "alert", "onclick", "javascript"} {
		if strings.Contains(stored, banned) {
			t.Errorf("thân lưu xuống còn %q: %s", banned, stored)
		}
	}
	for _, want := range []string{"<strong>chú ý</strong>", `href="https://dichvucong.gov.vn"`,
		`rel="noopener noreferrer nofollow"`} {
		if !strings.Contains(stored, want) {
			t.Errorf("thân lưu xuống thiếu %q: %s", want, stored)
		}
	}
	if got.NoiDung != stored {
		t.Errorf("giá trị trả về khác giá trị đã lưu")
	}
}

func TestEditSanitisesASentBodyAndLeavesAnUnsentOneAlone(t *testing.T) {
	legacy := *draftRow()
	legacy.NoiDung = `<p>Cũ</p><script>alert(1)</script>`

	// Body sent: sanitised.
	k := &khoNDGia{dongHienCo: &legacy}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	body := `<p>Mới</p><img src=x onerror=alert(1)>`
	if _, err := uc.Sua(ctx, legacy.ID, domain.YeuCauSuaNoiDung{NoiDung: &body}, nguoiSoanND()); err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	stored, _ := k.cau("UPDATE noi_dung_mini_app")[0].args[6].(string)
	if stored != "<p>Mới</p>" {
		t.Errorf("thân sau sửa = %q, muốn <p>Mới</p>", stored)
	}

	// Body not sent: the legacy row's stored HTML is written back exactly as it was (rule 7) — the
	// public read is what sanitises it.
	k2 := &khoNDGia{dongHienCo: &legacy}
	uc2, _, ctx2 := dungUseCaseNoiDung(t, k2)
	title := "Tiêu đề khác"
	if _, err := uc2.Sua(ctx2, legacy.ID, domain.YeuCauSuaNoiDung{TieuDe: &title}, nguoiSoanND()); err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if stored, _ := k2.cau("UPDATE noi_dung_mini_app")[0].args[6].(string); stored != legacy.NoiDung {
		t.Errorf("sửa không nhắc thân mà thân bị viết lại: %q", stored)
	}
}

// --- §5: banners -----------------------------------------------------------------------------------

func TestBannerWithoutCoverIsRefusedBeforeAnyTransaction(t *testing.T) {
	k := &khoNDGia{}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	yc := ycThemMau()
	yc.Loai = "banner"
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); !errors.Is(err, domain.ErrBannerCoverRequired) {
		t.Fatalf("lỗi = %v, muốn ErrBannerCoverRequired", err)
	}
	if k.batDau != 0 || len(k.lenh) != 0 {
		t.Errorf("từ chối mà vẫn mở giao dịch / chạy câu lệnh: %d, %d", k.batDau, len(k.lenh))
	}
}

func TestEditRefusesTurningIntoBannerWithoutCoverAndBannerFieldsElsewhere(t *testing.T) {
	news := *draftRow()
	news.CoverImageFileID = ""
	banner := "banner"
	link := "/tin-tuc"
	for name, yc := range map[string]domain.YeuCauSuaNoiDung{
		"into banner without cover": {Loai: &banner},
		"link_to on news":           {LinkTo: &link},
	} {
		k := &khoNDGia{dongHienCo: &news}
		uc, _, ctx := dungUseCaseNoiDung(t, k)
		_, err := uc.Sua(ctx, news.ID, yc, nguoiSoanND())
		if !errors.Is(err, domain.ErrBannerCoverRequired) && !errors.Is(err, domain.ErrBannerFieldsOnlyForBanner) {
			t.Errorf("%s: lỗi = %v", name, err)
		}
		if k.coCau("UPDATE noi_dung_mini_app") || k.coCau("INSERT INTO audit_log") || k.daCommit != 0 {
			t.Errorf("%s: từ chối mà vẫn ghi", name)
		}
	}
}

func TestTypeChangeAwayFromBannerClearsLinkAndOrderInTheSameUpdate(t *testing.T) {
	order := 2
	b := *draftRow()
	b.Loai, b.CoverImageFileID, b.LinkTo, b.DisplayOrder = domain.LoaiBanner, "", "/tin-tuc", &order
	// A legacy coverless banner: editing it is allowed (the trigger lets it through too).
	k := &khoNDGia{dongHienCo: &b}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	news := "tin-tuc"
	if _, err := uc.Sua(ctx, b.ID, domain.YeuCauSuaNoiDung{Loai: &news}, nguoiSoanND()); err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	args := k.cau("UPDATE noi_dung_mini_app")[0].args
	if args[16] != nil || args[17] != nil {
		t.Errorf("link_to = %v, display_order = %v — đổi khỏi banner phải xoá cả hai", args[16], args[17])
	}
	delta := decodedAuditDelta(t, k, HanhViSuaNoiDungMiniApp)
	if after, _ := delta["sau"].(map[string]any); after["link_to_changed"] != true || after["display_order"] != nil {
		t.Errorf("delta sau = %v", after)
	}
}

// --- §3: editing a category --------------------------------------------------------------------------

func categoryNow() *domain.DanhMucMiniApp {
	return &domain.DanhMucMiniApp{ID: "dm-1", Ten: "Y tế", Slug: "y-te", ChaID: "", ThuTu: 1}
}

func TestCategoryEditWritesRowAndTrailInOneTransaction(t *testing.T) {
	k := &khoNDGia{category: categoryNow(), coDong: true, ancestors: []string{"dm-goc"}}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	name, parent, hidden := "Y tế cộng đồng", "dm-goc", true

	got, err := ucDM.Update(ctx, "dm-1", domain.ContentCategoryUpdate{Ten: &name, ChaID: &parent, Hidden: &hidden}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa danh mục lỗi: %v", err)
	}
	if got.Ten != name || got.ChaID != "dm-goc" || !got.Hidden || got.Slug != "y-te" {
		t.Errorf("danh mục sau sửa = %+v", got)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở=%d chốt=%d huỷ=%d, muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}
	if !k.coCau("pg_advisory_xact_lock") {
		t.Error("đổi cha mà không khoá cây của xã")
	}
	if !k.coCau("UPDATE danh_muc_mini_app") {
		t.Fatal("không có câu cập nhật")
	}
	delta := decodedAuditDelta(t, k, ActionUpdateContentCategory)
	before, _ := delta["truoc"].(map[string]any)
	after, _ := delta["sau"].(map[string]any)
	if before["ten"] != "Y tế" || after["ten"] != name || before["cha_id"] != "" || after["cha_id"] != "dm-goc" ||
		before["hidden"] != false || after["hidden"] != true {
		t.Errorf("delta trước/sau = %v / %v", before, after)
	}
	if _, moved := after["thu_tu"]; moved {
		t.Error("thu_tu không đổi mà vẫn vào delta")
	}
	if subject := k.cau("INSERT INTO audit_log")[0].args[5]; subject != "y-te" {
		t.Errorf("đối tượng vết = %v, muốn slug y-te (mã nghiệp vụ, không phải ULID)", subject)
	}
}

func TestCategoryEditNoOpWritesNothing(t *testing.T) {
	k := &khoNDGia{category: categoryNow()}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	same := "Y tế"
	if _, err := ucDM.Update(ctx, "dm-1", domain.ContentCategoryUpdate{Ten: &same}, nguoiSoanND()); err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if k.coCau("UPDATE danh_muc_mini_app") || k.coCau("INSERT INTO audit_log") {
		t.Error("không đổi gì mà vẫn ghi / ghi vết")
	}
}

func TestCategoryReparentRefusals(t *testing.T) {
	cases := []struct {
		name      string
		parent    string
		live      bool
		ancestors []string
		want      error
	}{
		{"under itself", "dm-1", true, nil, domain.ErrDanhMucTuLamCha},
		{"under its grandchild", "dm-chau", true, []string{"dm-chau", "dm-con", "dm-1"}, domain.ErrCategoryCycle},
		{"under a dead parent", "dm-da-xoa", false, nil, ErrDanhMucChaKhongTonTai},
	}
	for _, c := range cases {
		k := &khoNDGia{category: categoryNow(), coDong: c.live, ancestors: c.ancestors}
		_, ucDM, ctx := dungUseCaseNoiDung(t, k)
		parent := c.parent
		if _, err := ucDM.Update(ctx, "dm-1", domain.ContentCategoryUpdate{ChaID: &parent}, nguoiSoanND()); !errors.Is(err, c.want) {
			t.Errorf("%s: lỗi = %v, muốn %v", c.name, err, c.want)
		}
		if k.coCau("UPDATE danh_muc_mini_app") || k.coCau("INSERT INTO audit_log") || k.daCommit != 0 {
			t.Errorf("%s: từ chối mà vẫn ghi", c.name)
		}
	}
}

func TestCategoryEditOfMissingIsNotFound(t *testing.T) {
	k := &khoNDGia{}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	name := "x"
	if _, err := ucDM.Update(ctx, "dm-cua-xa-khac", domain.ContentCategoryUpdate{Ten: &name}, nguoiSoanND()); err == nil ||
		!strings.Contains(err.Error(), "không có danh mục") {
		t.Errorf("lỗi = %v", err)
	}
}

// --- §3: retiring a category -------------------------------------------------------------------------

func TestCategoryDeleteIsRefusedWhileItHoldsLiveItemsOrChildren(t *testing.T) {
	for name, k := range map[string]*khoNDGia{
		"live item":  {category: categoryNow(), liveItems: 1},
		"live child": {category: categoryNow(), liveChildren: 1},
	} {
		_, ucDM, ctx := dungUseCaseNoiDung(t, k)
		if err := ucDM.Delete(ctx, "dm-1", "Không dùng nữa", nguoiSoanND()); !errors.Is(err, ErrCategoryNotEmpty) {
			t.Errorf("%s: lỗi = %v, muốn ErrCategoryNotEmpty", name, err)
		}
		if k.coCau("UPDATE danh_muc_mini_app") || k.coCau("INSERT INTO audit_log") || k.daCommit != 0 {
			t.Errorf("%s: từ chối mà vẫn ghi", name)
		}
	}
}

func TestCategoryDeleteIsSoftWithWhoWhyAndTrail(t *testing.T) {
	k := &khoNDGia{category: categoryNow()}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	if err := ucDM.Delete(ctx, "dm-1", "  Gộp vào Y tế cộng đồng ", nguoiSoanND()); err != nil {
		t.Fatalf("xoá lỗi: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("giao dịch: mở=%d chốt=%d", k.batDau, k.daCommit)
	}
	upd := k.cau("UPDATE danh_muc_mini_app")
	if len(upd) != 1 || upd[0].args[2] != maCanBoSoanND || upd[0].args[3] != "Gộp vào Y tế cộng đồng" {
		t.Fatalf("câu xoá mềm = %+v", upd)
	}
	if k.coCau("DELETE FROM") {
		t.Fatal("xoá cứng")
	}
	delta := decodedAuditDelta(t, k, ActionDeleteContentCategory)
	if delta["xoa_mem"] != true || delta["ly_do"] != "Gộp vào Y tế cộng đồng" {
		t.Errorf("delta = %v", delta)
	}
}

func TestCategoryDeleteNeedsAReason(t *testing.T) {
	k := &khoNDGia{category: categoryNow()}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	if err := ucDM.Delete(ctx, "dm-1", "   ", nguoiSoanND()); !errors.Is(err, domain.ErrThieuLyDoXoa) {
		t.Errorf("lỗi = %v, muốn ErrThieuLyDoXoa", err)
	}
	if k.batDau != 0 {
		t.Error("thiếu lý do mà vẫn mở giao dịch")
	}
}
