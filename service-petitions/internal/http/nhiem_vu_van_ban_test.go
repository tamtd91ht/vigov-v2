package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for §5.4's document block on the wire.
//
//	PROVED HERE   the DETAIL read carries the block and the REGISTER LIST does not read it at all ·
//	              `null` and `[]` stay two different statements · a `DATE` column travels as
//	              YYYY-MM-DD and an RFC 3339 instant is REFUSED rather than truncated · the
//	              pointer-to-slice distinction on PATCH reaches the use case intact · a malformed
//	              date opens no use case call at all.
//
//	THE FOUR CASES OF RULE 5, INVARIANT 7 ARE NOT REPEATED HERE, and that is not an omission: this
//	pass adds NO ROUTE. GET /api/v1/tasks/{ma}, POST /api/v1/tasks and PATCH /api/v1/tasks/{ma}
//	already carry all four — 401 no session · 403 wrong permission · 401 right permission WRONG
//	COMMUNE · 2xx both correct — in TestDocNhiemVu* (nhiem_vu_test.go) and in the six-route table of
//	nhiem_vu_ghi_test.go, and those assertions cover the bodies this file changes because they run
//	against the same handlers.

var mocNgayVanBanRa = time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

// vanBanCuaNVA is commune A's task with a document block: two groups, and A GAP in the positions of
// the first — lines ② and ③ were removed at some point and keep their numbers.
func vanBanCuaNVA() map[string][]domain.NhiemVuVanBan {
	return map[string][]domain.NhiemVuVanBan{
		"nv-001": {
			{ID: "vb-1", NhiemVuID: "nv-001", Nhom: domain.VanBanCapTrenGiao, ThuTu: 1,
				SoKyHieu: "1742-CV/BTCTU", NgayVanBan: mocNgayVanBanRa,
				TrichYeu: "Công văn của Ban Tổ chức Thành uỷ"},
			{ID: "vb-4", NhiemVuID: "nv-001", Nhom: domain.VanBanSanPhamRa, ThuTu: 4,
				TrichYeu: "Báo cáo của Ban Thường vụ Đảng uỷ"},
		},
	}
}

// --- the detail read -------------------------------------------------------------------------------

func TestDocNhiemVu_TraKhoiVanBanTheoDungThuTuDaLuu(t *testing.T) {
	m := dungMayChu(t)
	m.nhiemVu.vanBan = vanBanCuaNVA()

	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	ra := docNhiemVu(t, w.Body.Bytes())

	if len(ra.Documents) != 2 {
		t.Fatalf("documents = %d dòng, muốn 2", len(ra.Documents))
	}
	mot := ra.Documents[0]
	if mot.ID != "vb-1" || mot.Group != string(domain.VanBanCapTrenGiao) {
		t.Errorf("dòng đầu: id=%q group=%q", mot.ID, mot.Group)
	}
	if mot.Reference != "1742-CV/BTCTU" {
		t.Errorf("reference = %q", mot.Reference)
	}
	// A `DATE` COLUMN TRAVELS AS A CALENDAR DAY. An RFC 3339 instant would let a browser's zone
	// decide which DAY a document was signed.
	if mot.Date != "2026-06-09" {
		t.Errorf("date = %q, muốn 2026-06-09 (ngày ghi trên văn bản, không giờ, không múi)", mot.Date)
	}
	if mot.Position != 1 {
		t.Errorf("position = %d, muốn 1", mot.Position)
	}
	// THE GAP SURVIVES ONTO THE WIRE. `position` is an ISSUED number, not an array index: 4 is
	// correct here and renumbering it to 2 would be the client inventing a fact.
	if ra.Documents[1].Position != 4 {
		t.Errorf("position dòng hai = %d, muốn 4 — số đã cấp, không phải chỉ số mảng",
			ra.Documents[1].Position)
	}
	if m.nhiemVu.goiVanBan != 1 {
		t.Errorf("đọc khối văn bản %d lần, muốn 1", m.nhiemVu.goiVanBan)
	}
}

// TestDocNhiemVu_KhongCoDongNaoThiTraMangRong — `[]` means "this task has no lines", and it is a
// DIFFERENT statement from the `null` the register list sends.
func TestDocNhiemVu_KhongCoDongNaoThiTraMangRong(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"documents":[]`) {
		t.Fatalf("thân không mang `\"documents\":[]`: %s", w.Body.String())
	}
}

// TestDocNhiemVu_LoiDocKhoiVanBanThi500ChuKhongPhaiKhoiRong.
//
// Answering 200 with an empty block would be a statement ABOUT THE RECORD made from a failure to
// read it — on the one surface where the block is the screen's content. The drawer would say this
// task references no documents, and nobody would report it as a fault.
func TestDocNhiemVu_LoiDocKhoiVanBanThi500ChuKhongPhaiKhoiRong(t *testing.T) {
	m := dungMayChu(t)
	m.nhiemVu.loiVanBan = errors.New("kho hỏng")

	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "kho hỏng") {
		t.Errorf("lộ lỗi hệ thống ra client: %s", w.Body.String())
	}
}

// --- the register list -----------------------------------------------------------------------------

// TestDanhSachNhiemVu_KhongDocKhoiVanBanVaTraNull is the other half of the `null` / `[]` rule.
//
// §4's card draws no documents, and a page that loaded three lists per row would be payload nobody
// renders. `null` on the wire says "NOT SENT ON THIS SURFACE" and says nothing about whether the
// task has lines — which is exactly why the list must not send `[]`.
func TestDanhSachNhiemVu_KhongDocKhoiVanBanVaTraNull(t *testing.T) {
	m := dungMayChu(t)
	m.nhiemVu.vanBan = vanBanCuaNVA()

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	if m.nhiemVu.goiVanBan != 0 {
		t.Errorf("sổ nhiệm vụ đọc khối văn bản %d lần — mỗi trang sẽ tải ba danh sách cho mỗi dòng "+
			"mà thẻ §4 không vẽ", m.nhiemVu.goiVanBan)
	}
	if !strings.Contains(w.Body.String(), `"documents":null`) {
		t.Fatalf("thân trang không mang `\"documents\":null`: %s", w.Body.String())
	}

	// AND THE PARSED SHAPE AGREES: nil, not an empty slice.
	trang := docTrangNhiemVu(t, w.Body.Bytes())
	if len(trang.Items) == 0 {
		t.Fatal("trang rỗng — không kiểm được gì")
	}
	if trang.Items[0].Documents != nil {
		t.Errorf("documents = %+v trên sổ, muốn nil", trang.Items[0].Documents)
	}
}

// --- the write routes --------------------------------------------------------------------------------

func TestTaoNhiemVu_KhoiVanBanDiNguyenVenXuongUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create")

	than := thanTaoNV()
	than.Documents = []vanBanNhiemVuVao{
		{Group: string(domain.VanBanCapTrenGiao), Reference: "1742-CV/BTCTU", Date: "2026-06-09",
			Summary: "Công văn của Ban Tổ chức Thành uỷ"},
		{Group: string(domain.VanBanSanPhamRa), Summary: "Báo cáo số 335-BC/ĐU ngày 29/6/2026"},
	}

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongTasks, canBoCuaXa(xaA), than)

	doiMa(t, w, http.StatusCreated)
	vb := m.ghiNhiemVu.ycTao.VanBan
	if len(vb) != 2 {
		t.Fatalf("use case nhận %d dòng văn bản, muốn 2", len(vb))
	}
	if vb[0].Nhom != domain.VanBanCapTrenGiao || vb[0].SoKyHieu != "1742-CV/BTCTU" {
		t.Errorf("dòng đầu sai: %+v", vb[0])
	}
	if !vb[0].NgayVanBan.Equal(mocNgayVanBanRa) {
		t.Errorf("ngay_van_ban = %v, muốn %v", vb[0].NgayVanBan, mocNgayVanBanRa)
	}
	// AN ABSENT DATE IS THE ZERO time.Time, which the store writes as SQL NULL — the ordinary case,
	// because §7.2 collects no date at all.
	if !vb[1].NgayVanBan.IsZero() {
		t.Errorf("dòng không khai ngày lại mang %v", vb[1].NgayVanBan)
	}
}

// TestTaoNhiemVu_KhongGuiDocumentsThiUseCaseNhanRong — rule 2, invariant 4. `taoNhiemVuVao` is a
// PUBLISHED contract with a real client behind it, and the field had to be OPTIONAL: a body that
// predates this pass must still be accepted, unchanged.
func TestTaoNhiemVu_KhongGuiDocumentsThiUseCaseNhanRong(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create")

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongTasks,
		canBoCuaXa(xaA), thanTaoNV())

	doiMa(t, w, http.StatusCreated)
	if len(m.ghiNhiemVu.ycTao.VanBan) != 0 {
		t.Errorf("use case nhận %d dòng văn bản cho một thân không khai `documents`",
			len(m.ghiNhiemVu.ycTao.VanBan))
	}
}

// TestTaoNhiemVu_NgayVanBanSaiDangThi400VaKhongGoiUseCase: refused at the edge, so a malformed date
// opens no transaction on a government register.
func TestTaoNhiemVu_NgayVanBanSaiDangThi400VaKhongGoiUseCase(t *testing.T) {
	for _, ngay := range []string{
		"2026-06-09T00:00:00Z", // RFC 3339 — REFUSED, not truncated: a zone must not pick the day
		"09/06/2026",
		"2026-6-9",
		"hôm qua",
	} {
		t.Run(ngay, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "task.create")

			than := thanTaoNV()
			than.Documents = []vanBanNhiemVuVao{{
				Group: string(domain.VanBanCapTrenGiao), Date: ngay, Summary: "x",
			}}

			w := m.goiGhiNV(t, http.MethodPost, hostA, duongTasks,
				canBoCuaXa(xaA), than)

			doiMa(t, w, http.StatusBadRequest)
			if m.ghiNhiemVu.goi != 0 {
				t.Errorf("gọi use case %d lần dù ngày không đọc được", m.ghiNhiemVu.goi)
			}
			// THE CLIENT'S STRING IS NOT ECHOED BACK: it is free text about an administrative
			// document, and an error message travels into centralised logging (rule 3, forbidden #3).
			if strings.Contains(w.Body.String(), ngay) {
				t.Errorf("thông điệp lỗi nhắc lại chuỗi máy khách gửi: %s", w.Body.String())
			}
		})
	}
}

// TestSuaNhiemVu_KhongGuiDocumentsThiUseCaseNhanNil IS THE POINTER DISTINCTION, half one.
func TestSuaNhiemVu_KhongGuiDocumentsThiUseCaseNhanNil(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.update")
	tieuDe := "Tiêu đề mới"

	w := m.goiGhiNV(t, http.MethodPatch, hostA, duongNV(maNVThu),
		canBoCuaXa(xaA), suaNhiemVuVao{Title: &tieuDe})

	doiMa(t, w, http.StatusOK)
	if m.ghiNhiemVu.ycSua.VanBan != nil {
		t.Fatalf("VanBan = %+v, muốn nil — thân không nhắc tới khối thì khối phải được để nguyên",
			m.ghiNhiemVu.ycSua.VanBan)
	}
}

// TestSuaNhiemVu_GuiDocumentsRongThiUseCaseNhanConTroToiSliceRong IS THE POINTER DISTINCTION, half
// two — and the half that matters: `[]` means the block is being EMPTIED. Collapsed into nil, three
// `✕` clicks followed by Save would silently keep every line.
func TestSuaNhiemVu_GuiDocumentsRongThiUseCaseNhanConTroToiSliceRong(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.update")

	// Written as raw JSON on purpose: `documents: []` and an absent key are two different bodies, and
	// marshalling a Go struct could not produce both from one shape.
	w := m.goiGhiNVTho(t, http.MethodPatch, hostA, duongNV(maNVThu),
		canBoCuaXa(xaA), `{"documents":[]}`)

	doiMa(t, w, http.StatusOK)
	if m.ghiNhiemVu.ycSua.VanBan == nil {
		t.Fatal("VanBan = nil cho thân `{\"documents\":[]}` — `gỡ hết` bị đọc thành `không nhắc tới`")
	}
	if len(*m.ghiNhiemVu.ycSua.VanBan) != 0 {
		t.Errorf("VanBan = %d dòng, muốn 0", len(*m.ghiNhiemVu.ycSua.VanBan))
	}
}

func TestSuaNhiemVu_GuiLaiDongCoIDThiIDDiXuongUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.update")

	w := m.goiGhiNVTho(t, http.MethodPatch, hostA, duongNV(maNVThu),
		canBoCuaXa(xaA),
		`{"documents":[{"id":"vb-1","group":"cap-tren-giao","summary":"Đã đính chính"}]}`)

	doiMa(t, w, http.StatusOK)
	if m.ghiNhiemVu.ycSua.VanBan == nil || len(*m.ghiNhiemVu.ycSua.VanBan) != 1 {
		t.Fatalf("VanBan = %+v, muốn một dòng", m.ghiNhiemVu.ycSua.VanBan)
	}
	v := (*m.ghiNhiemVu.ycSua.VanBan)[0]
	if v.ID != "vb-1" {
		t.Errorf("id = %q, muốn vb-1 — không mang id thì mỗi lần lưu lại sinh thêm một dòng mới", v.ID)
	}
	if v.TrichYeu != "Đã đính chính" {
		t.Errorf("trich_yeu = %q", v.TrichYeu)
	}
}

// --- the refusals, and which status code each gets ------------------------------------------------

// TestLoiKhoiVanBanAnhXaDungMa is the 400 / 409 line for this block.
//
//	400  the BODY is wrong — a group that is not one of the three, an empty line. The officer fixes
//	     the form.
//	409  the RECORD moved — the line is not on this task any more, or it sits in another group. The
//	     officer reloads the drawer. A 400 here would tell them to fix a form that was correct.
func TestLoiKhoiVanBanAnhXaDungMa(t *testing.T) {
	for _, tc := range []struct {
		ten  string
		loi  error
		muon int
		ma   string
	}{
		{"nhóm không hợp lệ", domain.ErrNhomVanBanKhongHopLe, http.StatusBadRequest, "invalid_request"},
		{"thiếu nội dung", domain.ErrThieuTrichYeuVanBan, http.StatusBadRequest, "invalid_request"},
		{"quá nhiều dòng", domain.ErrQuaNhieuVanBan, http.StatusBadRequest, "invalid_request"},
		{"gửi trùng một dòng", domain.ErrVanBanTrungTrongYeuCau, http.StatusBadRequest, "invalid_request"},
		{"dòng không thuộc nhiệm vụ", domain.ErrVanBanKhongThuocNhiemVu, http.StatusConflict, "task_document"},
		{"đổi nhóm một dòng đã lưu", domain.ErrDoiNhomVanBan, http.StatusConflict, "task_document"},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "task.update")
			m.ghiNhiemVu.loi = tc.loi

			w := m.goiGhiNV(t, http.MethodPatch, hostA, duongNV(maNVThu),
				canBoCuaXa(xaA), suaNhiemVuVao{})

			doiMa(t, w, tc.muon)
			if e := loiTra(t, w); e.Code != tc.ma {
				t.Errorf("mã lỗi = %q, muốn %q", e.Code, tc.ma)
			}
		})
	}
}

// --- the helper this file needs ---------------------------------------------------------------------

// goiGhiNVTho issues a write request with a RAW JSON body.
//
// IT EXISTS BECAUSE `documents: []` AND AN ABSENT `documents` ARE TWO DIFFERENT BODIES, and a Go
// struct marshalled through `goiGhiNV` cannot produce both: `*[]T` with `omitempty` drops the key
// when the pointer is nil and emits `[]` when it is not, so the two cases can only be written out as
// text. The distinction is the whole of "emptying the block" versus "not touching it".
func (m *mayChu) goiGhiNVTho(t *testing.T, method, host, path string, p *authz.Principal,
	than string) *httptest.ResponseRecorder {
	t.Helper()
	return m.goiGhiNV(t, method, host, path, p, json.RawMessage(than))
}
