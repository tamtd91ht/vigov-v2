package http

// ADR 0067 §1, §3, §5 on the wire. The four-case permission suite (rule 5, invariant 7) for the two
// new category routes is in noi_dung_mini_app_test.go's table (cacTuyenND), so it runs through the real
// Register behind the real edge chain with every other content route. What follows is the contract.

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// --- PATCH /api/v1/content-categories/{id} ---------------------------------------------------------

func TestCategoryPatchCarriesOnlyWhatWasSent(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goi(t, http.MethodPatch, hostA, duongDanhMucND+"/dm-001",
		`{"name":"Y tế","parent_id":"","hidden":true}`, canBo(xaA))
	doiMa(t, w, http.StatusOK)
	got := m.ghiDM.lastUpdate
	if m.ghiDM.lastID != "dm-001" || got.Ten == nil || *got.Ten != "Y tế" || got.ChaID == nil || *got.ChaID != "" ||
		got.Hidden == nil || !*got.Hidden || got.ThuTu != nil {
		t.Errorf("yêu cầu tới use case = id %q, %+v", m.ghiDM.lastID, got)
	}
	if m.ghiDM.nguoi.ID != "CB-2026-7K3M9Q" {
		t.Errorf("chủ thể = %q, muốn mã cán bộ", m.ghiDM.nguoi.ID)
	}
	if !strings.Contains(w.Body.String(), `"hidden"`) {
		t.Errorf("phản hồi phải mang `hidden`: %s", w.Body.String())
	}
}

func TestCategoryPatchRefusesSlugBeforeTheUseCase(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goi(t, http.MethodPatch, hostA, duongDanhMucND+"/dm-001", `{"name":"Y tế","slug":"y-te"}`, canBo(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if e := loiTra(t, w); e.Code != "invalid_request" || !strings.Contains(e.Message, "slug") {
		t.Errorf("lỗi = %+v", e)
	}
	if m.ghiDM.updates != 0 {
		t.Error("gửi slug mà vẫn chạm tới use case")
	}
}

func TestCategoryWriteRefusalsMapToStatusAndCode(t *testing.T) {
	cases := []struct {
		name, method, body string
		err                error
		status             int
		code               string
	}{
		{"patch missing", http.MethodPatch, `{"name":"x"}`, commsstore.ErrDanhMucKhongTonTaiMiniApp, http.StatusNotFound, "not_found"},
		{"patch cycle", http.MethodPatch, `{"parent_id":"dm-chau"}`, domain.ErrCategoryCycle, http.StatusConflict, "category_cycle"},
		{"patch self", http.MethodPatch, `{"parent_id":"dm-001"}`, domain.ErrDanhMucTuLamCha, http.StatusBadRequest, "invalid_request"},
		{"patch dead parent", http.MethodPatch, `{"parent_id":"dm-x"}`, app.ErrDanhMucChaKhongTonTai, http.StatusConflict, "parent_missing"},
		{"delete missing", http.MethodDelete, `{"reason":"r"}`, commsstore.ErrDanhMucKhongTonTaiMiniApp, http.StatusNotFound, "not_found"},
		{"delete not empty", http.MethodDelete, `{"reason":"r"}`, app.ErrCategoryNotEmpty, http.StatusConflict, "category_not_empty"},
		{"delete no reason", http.MethodDelete, `{}`, domain.ErrThieuLyDoXoa, http.StatusBadRequest, "invalid_request"},
		{"store failure", http.MethodDelete, `{"reason":"r"}`, errors.New("pq: boom"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		m := dungMayChuND(t)
		m.capQuyen(xaA, QuyenSuaNoiDung)
		m.ghiDM.loi = c.err
		w := m.goi(t, c.method, hostA, duongDanhMucND+"/dm-001", c.body, canBo(xaA))
		doiMa(t, w, c.status)
		e := loiTra(t, w)
		if e.Code != c.code {
			t.Errorf("%s: mã = %q, muốn %q", c.name, e.Code, c.code)
		}
		if c.code == "category_not_empty" && !strings.Contains(e.Message, "ẩn") {
			t.Errorf("%s: câu từ chối phải gợi ý ẩn danh mục: %q", c.name, e.Message)
		}
		if strings.Contains(w.Body.String(), "pq:") {
			t.Errorf("%s: lỗi kho lọt ra client: %s", c.name, w.Body.String())
		}
	}
}

func TestCategoryDeleteSendsTheReasonAndAnswers204(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goi(t, http.MethodDelete, hostA, duongDanhMucND+"/dm-001", `{"reason":"Gộp vào Y tế"}`, canBo(xaA))
	doiMa(t, w, http.StatusNoContent)
	if m.ghiDM.lastReason != "Gộp vào Y tế" || m.ghiDM.lastID != "dm-001" || m.ghiDM.nguoi.ID != "CB-2026-7K3M9Q" {
		t.Errorf("tới use case: id %q, lý do %q, chủ thể %q", m.ghiDM.lastID, m.ghiDM.lastReason, m.ghiDM.nguoi.ID)
	}
	if w.Body.Len() != 0 {
		t.Errorf("204 không mang thân: %q", w.Body.String())
	}
}

// --- banner fields on the staff write routes --------------------------------------------------------

func TestBannerFieldsReachTheUseCase(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	doiMa(t, m.goi(t, http.MethodPost, hostA, duongNoiDung,
		`{"type":"banner","title":"Lễ hội","cover_image_file_id":"01JCOVER","link_to":"/su-kien","display_order":3}`,
		canBo(xaA)), http.StatusCreated)
	if got := m.ghi.themCuoi; got.LinkTo != "/su-kien" || got.DisplayOrder == nil || *got.DisplayOrder != 3 {
		t.Errorf("thêm: %+v", got)
	}

	m2 := dungMayChuND(t)
	m2.capQuyen(xaA, QuyenSuaNoiDung)
	doiMa(t, m2.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"link_to":""}`, canBo(xaA)), http.StatusOK)
	if got := m2.ghi.suaCuoi; got.LinkTo == nil || *got.LinkTo != "" || got.DisplayOrder != nil {
		t.Errorf("sửa: link_to \"\" phải là XOÁ, display_order vắng phải nil: %+v", got)
	}
}

func TestBannerRefusalsAre422WithTheirCodes(t *testing.T) {
	for err, code := range map[error]string{
		domain.ErrLinkToInvalid:             "invalid_link_to",
		domain.ErrDisplayOrderInvalid:       "invalid_display_order",
		domain.ErrBannerFieldsOnlyForBanner: "banner_fields_only_for_banner",
		domain.ErrBannerCoverRequired:       "banner_cover_required",
	} {
		for _, method := range []string{http.MethodPost, http.MethodPatch} {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.ghi.loi = err
			path, body := duongNoiDung, thanThemNoiDungHopLe
			if method == http.MethodPatch {
				path, body = duongNoiDung+"/nd-001", thanSuaNoiDungHopLe
			}
			w := m.goi(t, method, hostA, path, body, canBo(xaA))
			doiMa(t, w, http.StatusUnprocessableEntity)
			if e := loiTra(t, w); e.Code != code || e.Message != err.Error() {
				t.Errorf("%s %v: lỗi = %+v", method, err, e)
			}
		}
	}
}

func TestStaffItemCarriesBannerFieldsOnlyWhenSet(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	order := 0
	b := noiDungMau()
	b.Loai, b.LinkTo, b.DisplayOrder = domain.LoaiBanner, "https://dichvucong.gov.vn", &order
	m.so.mot = b
	w := m.goi(t, http.MethodGet, hostA, duongNoiDung+"/nd-001", "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["link_to"] != "https://dichvucong.gov.vn" || got["display_order"] != float64(0) {
		t.Errorf("link_to = %v, display_order = %v — 0 là một vị trí thật, phải có mặt", got["link_to"], got["display_order"])
	}
	m.so.mot = noiDungMau()
	w = m.goi(t, http.MethodGet, hostA, duongNoiDung+"/nd-001", "", canBo(xaA))
	if strings.Contains(w.Body.String(), "link_to") || strings.Contains(w.Body.String(), "display_order") {
		t.Errorf("tin thường không mang trường banner: %s", w.Body.String())
	}
	// The staff detail still returns the STORED HTML: TestDocSoNoiDungKhongMangToanVanConTuyenChiTietThiCo.
}

// --- the public surface -------------------------------------------------------------------------------

func intPtr(n int) *int { return &n }

// bannerCommune is commune A with banners in every case the strip must separate, a news item, and
// commune B's banner (which A's strip must never show).
func bannerCommune() *ckNoiDung {
	banner := func(id string, order *int, cover string, state domain.TrangThaiNoiDung, link string) domain.NoiDungMiniApp {
		return domain.NoiDungMiniApp{ID: id, Loai: domain.LoaiBanner, TieuDe: "Banner " + id, NgayDang: ckNgay,
			TrangThai: state, CoverImageFileID: cover, DisplayOrder: order, LinkTo: link}
	}
	return &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{
		xaA: {
			banner("b-unordered", nil, "f-unordered", domain.TrangThaiDangHien, ""),
			banner("b-second", intPtr(2), "f-second", domain.TrangThaiDangHien, "https://dichvucong.gov.vn"),
			banner("b-first", intPtr(1), "f-first", domain.TrangThaiDangHien, "/su-kien"),
			banner("b-no-cover", intPtr(0), "", domain.TrangThaiDangHien, "/x"),
			banner("b-no-public-copy", intPtr(0), "f-unpublished-copy", domain.TrangThaiDangHien, ""),
			banner("b-draft", intPtr(0), "f-draft", domain.TrangThaiAn, ""),
			banner("b-bad-link", intPtr(3), "f-bad", domain.TrangThaiDangHien, "javascript:alert(1)"),
			{ID: "a-news", Loai: domain.LoaiTinTuc, TieuDe: "Tin A", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien},
		},
		xaB: {banner("b-of-b", intPtr(0), "f-b", domain.TrangThaiDangHien, "")},
	}}
}

func bannerServer(t *testing.T, nd *ckNoiDung, dm *ckDanhMuc) http.Handler {
	t.Helper()
	covers := &fakePublicCovers{byTenant: map[tenant.ID]map[string]string{
		xaA: {"f-unordered": "https://cdn/u.jpg", "f-second": "https://cdn/2.jpg", "f-first": "https://cdn/1.jpg",
			"f-draft": "https://cdn/d.jpg", "f-bad": "https://cdn/bad.jpg"},
		xaB: {"f-b": "https://cdn/b.jpg"},
	}}
	if dm == nil {
		dm = &ckDanhMuc{}
	}
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Limiter: ckLimiter(), Xa: &ckNenTang{}, NoiDung: nd, Views: nd, DanhMuc: dm, CoverImages: covers, Audio: &fakePublicAudio{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return mux
}

func TestPublicBannerStripOrderPictureAndLink(t *testing.T) {
	nd := bannerCommune()
	h := bannerServer(t, nd, nil)
	w := ckGoi(h, MauTinXa, ckHostA, "&type=banner")
	doiMa(t, w, http.StatusOK)
	page := ckDocTrang(t, w)
	var ids []string
	for _, it := range page.Items {
		ids = append(ids, it["id"].(string))
	}
	// display_order ASC NULLS LAST; no draft, no coverless banner, no banner whose cover has no public
	// copy, nothing of commune B, no news.
	if got := strings.Join(ids, ","); got != "b-first,b-second,b-bad-link,b-unordered" {
		t.Fatalf("dải banner = %s", got)
	}
	first := page.Items[0]
	if first["image_url"] != "https://cdn/1.jpg" || first["title"] != "Banner b-first" || first["link_to"] != "/su-kien" ||
		first["type"] != "banner" {
		t.Errorf("banner đầu = %v", first)
	}
	if page.Items[1]["link_to"] != "https://dichvucong.gov.vn" {
		t.Errorf("link_to https = %v", page.Items[1]["link_to"])
	}
	// A stored link_to that fails the rule never reaches a resident's tap; absent = not tappable.
	for _, i := range []int{2, 3} {
		if _, ok := page.Items[i]["link_to"]; ok {
			t.Errorf("%v: link_to phải vắng: %v", page.Items[i]["id"], page.Items[i])
		}
	}
	if page.HasMore || page.NextCursor != "" {
		t.Errorf("dải banner không phân trang: %+v", page)
	}
	if nd.bannerLimit != commsstore.PublicBannerStripMax {
		t.Errorf("trần = %d, muốn %d", nd.bannerLimit, commsstore.PublicBannerStripMax)
	}
	ckKhongCoHTML(t, w)
}

func TestPublicDefaultListExcludesBanners(t *testing.T) {
	h := bannerServer(t, bannerCommune(), nil)
	w := ckGoi(h, MauTinXa, ckHostA)
	doiMa(t, w, http.StatusOK)
	items := ckDocTrang(t, w).Items
	if len(items) != 1 || items[0]["id"] != "a-news" {
		t.Fatalf("danh sách mặc định = %v, muốn đúng [a-news]", items)
	}
}

func TestPublicBannerStripPlatformAndStoreFailures(t *testing.T) {
	nd := bannerCommune()
	nd.loi = errors.New("pq: boom")
	w := ckGoi(bannerServer(t, nd, nil), MauTinXa, ckHostA, "&type=banner")
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "pq:") {
		t.Errorf("lỗi kho lọt ra: %s", w.Body.String())
	}
	// Unknown domain: the same empty page as a commune with no banners.
	w = ckGoi(bannerServer(t, bannerCommune(), nil), MauTinXa, ckHostKhongCo, "&type=banner")
	doiMa(t, w, http.StatusOK)
	if len(ckDocTrang(t, w).Items) != 0 {
		t.Errorf("tên miền lạ phải là trang rỗng: %s", w.Body.String())
	}
}

func TestHiddenCategoryLosesItsChipButItsItemsStayListed(t *testing.T) {
	dm := &ckDanhMuc{theoXa: map[tenant.ID][]domain.DanhMucMiniApp{
		xaA: {{ID: "dm-shown", Ten: "Y tế"}, {ID: "dm-hidden", Ten: "Lưu trữ", Hidden: true}},
	}}
	nd := &ckNoiDung{tree: dm, theoXa: map[tenant.ID][]domain.NoiDungMiniApp{
		xaA: {
			{ID: "n-shown", Loai: domain.LoaiTinTuc, TieuDe: "A", DanhMucID: "dm-shown", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien},
			{ID: "n-hidden", Loai: domain.LoaiTinTuc, TieuDe: "B", DanhMucID: "dm-hidden", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien},
		},
	}}
	h := bannerServer(t, nd, dm)

	w := ckGoi(h, MauTinXa+"/categories", ckHostA)
	doiMa(t, w, http.StatusOK)
	if body := w.Body.String(); !strings.Contains(body, "dm-shown") || strings.Contains(body, "dm-hidden") {
		t.Errorf("chip: %s", body)
	}
	w = ckGoi(h, MauTinXa, ckHostA)
	doiMa(t, w, http.StatusOK)
	if items := ckDocTrang(t, w).Items; len(items) != 2 {
		t.Errorf("tin của danh mục ẩn vẫn phải có trong 'Tất cả': %v", items)
	}
	// And its own lookup still answers.
	doiMa(t, ckGoi(h, MauTinXa+"/n-hidden", ckHostA), http.StatusOK)
}

func TestPublicDetailBodyBlocksCarryStructureAndPlainBodyStays(t *testing.T) {
	nd := &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{xaA: {{
		ID: "n-rich", Loai: domain.LoaiThongBao, TieuDe: "Lịch tiêm", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien,
		NoiDung: `<h2>Lịch</h2><p>Mang <strong>sổ</strong> và <a href="https://dichvucong.gov.vn">đăng ký</a></p>` +
			`<ol><li>Sáng</li><li><em>Chiều</em></li></ol>`,
	}}}}
	w := ckGoi(bannerServer(t, nd, nil), MauTinXa+"/n-rich", ckHostA)
	doiMa(t, w, http.StatusOK)
	ckKhongCoHTML(t, w)
	var got struct {
		Body       string         `json:"body"`
		BodyBlocks []bodyBlockOut `json:"body_blocks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if got.Body != "Lịch\n\nMang sổ và đăng ký\n\nSáng\n\nChiều" {
		t.Errorf("body văn bản thuần = %q", got.Body)
	}
	want := []bodyBlockOut{
		{Kind: "heading", Level: 2, Runs: []inlineRunOut{{Text: "Lịch"}}},
		{Kind: "paragraph", Runs: []inlineRunOut{{Text: "Mang "}, {Text: "sổ", Bold: true}, {Text: " và "},
			{Text: "đăng ký", Href: "https://dichvucong.gov.vn"}}},
		{Kind: "ordered_list", Items: []listItemOut{{Runs: []inlineRunOut{{Text: "Sáng"}}},
			{Runs: []inlineRunOut{{Text: "Chiều", Italic: true}}}}},
	}
	a, _ := json.Marshal(got.BodyBlocks)
	b, _ := json.Marshal(want)
	if string(a) != string(b) {
		t.Errorf("body_blocks =\n%s\nmuốn\n%s", a, b)
	}

	// The list never carries body_blocks.
	w = ckGoi(bannerServer(t, nd, nil), MauTinXa, ckHostA)
	if strings.Contains(w.Body.String(), "body_blocks") {
		t.Errorf("trang danh sách không mang body_blocks: %s", w.Body.String())
	}
}
