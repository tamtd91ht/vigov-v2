package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// GET /api/v1/staff's two URL filters and POST /api/v1/staff/searches (user decision 2026-09-24).
//
// What is proven HERE is the handler's half: permission, validation before the store, and that the
// store receives exactly the filter the request carried. What the rows look like under that filter
// is proven against the real predicate in store/can_bo_tim_test.go.

const duongTim = "/api/v1/staff/searches"

// --- rule 5, invariant 7 — the four cases for POST /searches ------------------------------------

func TestTimCanBo_401KhongToken(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, "POST", hostA, duongTim, `{"q":"Nguyễn"}`, ""), http.StatusUnauthorized)
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("chưa đăng nhập mà đã đọc danh bạ %d lần", m.danhBa.soLanGoi)
	}
}

// 403 WITH THE WRONG PERMISSION: `content.update` is the Mini App directory key — a real key, on the
// neighbouring screen — and it does not open the register search.
func TestTimCanBo_403KhongCoAdminUser(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("content.update"): true}},
			xaB: {},
		}}
	})
	doiMa(t, m.goi(t, "POST", hostA, duongTim, `{"q":"Nguyễn"}`, m.tokenCho(t, xaA, sidA)), http.StatusForbidden)
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("thiếu admin.user mà đã đọc danh bạ %d lần", m.danhBa.soLanGoi)
	}
}

// 403 WITH THE RIGHT PERMISSION IN THE WRONG COMMUNE: `admin.user` in A, signed in properly at B.
func TestTimCanBo_403DungQuyenSaiXa(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, "POST", hostB, duongTim, `{"q":"Nguyễn"}`, m.tokenCho(t, xaB, sidB)), http.StatusForbidden)
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("sai xã mà đã đọc danh bạ %d lần", m.danhBa.soLanGoi)
	}
}

func TestTimCanBo_200VaChuyenDungLoc(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, "POST", hostA, duongTim,
		`{"q":"  Nguyễn   Văn ","unit":"bp-001","published":false,"limit":2}`, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var ra page.Result[canBoTomTat]
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải page.Result: %q", w.Body.String())
	}
	if len(ra.Items) != 2 || !ra.HasMore || ra.NextCursor == "" {
		t.Errorf("không đúng hình trang của GET /staff: %d dòng, has_more=%v", len(ra.Items), ra.HasMore)
	}

	d := m.danhBa
	if d.xaCuoi != xaA {
		t.Errorf("kho được gọi với xã %q, muốn %q", d.xaCuoi, xaA)
	}
	if d.locCuoi.TuKhoa != "Nguyễn Văn" || d.locCuoi.BoPhanID != "bp-001" ||
		d.locCuoi.CongKhai == nil || *d.locCuoi.CongKhai {
		t.Errorf("bộ lọc tới kho = %+v", d.locCuoi)
	}
	if d.yeuCauCuoi.Limit() != 2 || d.yeuCauCuoi.Column().Param != "code" {
		t.Errorf("trang tới kho: limit=%d sort=%q", d.yeuCauCuoi.Limit(), d.yeuCauCuoi.Column().Param)
	}

	// The cursor this search handed out continues THIS search.
	than := `{"q":"Nguyễn Văn","unit":"bp-001","published":false,"limit":2,"cursor":"` + ra.NextCursor + `"}`
	doiMa(t, m.goi(t, "POST", hostA, duongTim, than, m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if _, ok := m.danhBa.yeuCauCuoi.After(); !ok {
		t.Error("con trỏ trong thân không tới kho")
	}
}

// --- search validation, all BEFORE the store ------------------------------------------------------

func TestTimCanBoDauVaoSaiTra400VaKhongChamKho(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	for ten, than := range map[string]string{
		"q thiếu":              `{}`,
		"q rỗng":               `{"q":""}`,
		"q toàn khoảng trắng":  `{"q":"   \t "}`,
		"q quá 200 ký tự":      `{"q":"` + strings.Repeat("ễ", domain.TuKhoaTimCanBoToiDa+1) + `"}`,
		"unit quá dài":         `{"q":"Nguyễn","unit":"` + strings.Repeat("x", 65) + `"}`,
		"published không bool": `{"q":"Nguyễn","published":"yes"}`,
		"limit 0":              `{"q":"Nguyễn","limit":0}`,
		"cursor hỏng":          `{"q":"Nguyễn","cursor":"khong-phai-con-tro"}`,
		"không phải JSON":      `q=Nguyễn`,
	} {
		w := m.goi(t, "POST", hostA, duongTim, than, tok)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: mã = %d, muốn 400. Thân: %s", ten, w.Code, w.Body.String())
		}
	}
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("yêu cầu bị từ chối mà vẫn đọc kho %d lần", m.danhBa.soLanGoi)
	}
}

// THE LIMIT IS IN CHARACTERS, end to end: 150 Vietnamese letters (well over 200 bytes) pass.
func TestTimCanBo150KyTuTiengVietDuocNhan(t *testing.T) {
	m := dungMayChu(t)
	tu := string([]rune(strings.Repeat("Nguyễn Thị Hồng ", 10))[:150])
	if len(tu) <= domain.TuKhoaTimCanBoToiDa {
		t.Fatalf("mẫu chỉ %d byte — không phân biệt được byte với ký tự", len(tu))
	}
	w := m.goi(t, "POST", hostA, duongTim, `{"q":"`+tu+`"}`, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if m.danhBa.locCuoi.TuKhoa != tu {
		t.Error("từ khoá tới kho bị đổi")
	}
}

// THE REFUSAL NEVER QUOTES WHAT WAS SENT: the text is likely a name or a number.
func TestTimCanBoLoiKhongTrichLaiTuKhoa(t *testing.T) {
	m := dungMayChu(t)
	tu := strings.Repeat("0900000000", 25)
	w := m.goi(t, "POST", hostA, duongTim, `{"q":"`+tu+`"}`, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusBadRequest)
	if strings.Contains(w.Body.String(), "0900000000") {
		t.Errorf("thông báo lỗi trích lại từ khoá: %s", w.Body.String())
	}
}

// `q` NEVER REACHES THE LOGGER — not on success, not on the store-failure path, which is the one
// path in this handler that logs at all (rule 3, invariant 1).
//
// MUTATION THAT MUST TURN THIS RED: add "loc", loc (or "q", than.Q) to the Log.Error call in
// traTrangCanBo.
func TestTimCanBoKhongGhiTuKhoaVaoLog(t *testing.T) {
	m, buf := mayChuLog(t)
	const tu = "Hoàng Bí Mật 0900000099"
	tok := m.tokenCho(t, xaA, sidA)

	doiMa(t, m.goi(t, "POST", hostA, duongTim, `{"q":"`+tu+`"}`, tok), http.StatusOK)
	m.danhBa.loi = errors.New("cơ sở dữ liệu không phản hồi")
	doiMa(t, m.goi(t, "POST", hostA, duongTim, `{"q":"`+tu+`"}`, tok), http.StatusInternalServerError)

	log := buf.String()
	if !strings.Contains(log, "tìm cán bộ") {
		t.Fatalf("lối lỗi không ghi log — bài kiểm không kiểm được gì: %q", log)
	}
	for _, manh := range []string{"Hoàng", "Bí Mật", "0900000099"} {
		if strings.Contains(log, manh) {
			t.Errorf("log chứa từ khoá tìm kiếm %q: %s", manh, log)
		}
	}
}

// --- GET /api/v1/staff: the two URL filters -------------------------------------------------------

func TestDanhSachCanBoLocUnitVaPublishedToiKho(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff?unit=bp-001&published=true", "", tok), http.StatusOK)
	if l := m.danhBa.locCuoi; l.BoPhanID != "bp-001" || l.CongKhai == nil || !*l.CongKhai || l.TuKhoa != "" {
		t.Errorf("bộ lọc tới kho = %+v", l)
	}

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff?published=false", "", tok), http.StatusOK)
	if l := m.danhBa.locCuoi; l.BoPhanID != "" || l.CongKhai == nil || *l.CongKhai {
		t.Errorf("published=false tới kho = %+v — false là một bộ lọc thật, không phải 'không lọc'", l)
	}

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", tok), http.StatusOK)
	if l := m.danhBa.locCuoi; l != (domain.LocCanBo{}) {
		t.Errorf("không có bộ lọc mà kho nhận %+v", l)
	}
}

// THE URL CANNOT CARRY THE TEXT SEARCH. `?q=` is not read: the GET route has no way to set TuKhoa,
// which is the point of the user's decision — a name in a URL is a name in an access log.
func TestDanhSachCanBoKhongDocQTuURL(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff?q=Nguy%E1%BB%85n", "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if m.danhBa.locCuoi.TuKhoa != "" {
		t.Errorf("GET /staff đọc từ khoá từ URL: %+v", m.danhBa.locCuoi)
	}
}

func TestDanhSachCanBoLocSaiTra400VaKhongChamKho(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, truyVan := range []string{
		"?published=yes", "?published=1", "?published=TRUE", "?published=t",
		"?unit=" + strings.Repeat("x", 65),
	} {
		w := m.goi(t, "GET", hostA, "/api/v1/staff"+truyVan, "", tok)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: mã = %d, muốn 400", truyVan, w.Code)
		}
		if strings.Contains(w.Body.String(), "yes") {
			t.Errorf("%s: lỗi trích lại giá trị client gửi: %s", truyVan, w.Body.String())
		}
	}
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("bộ lọc sai mà vẫn đọc kho %d lần", m.danhBa.soLanGoi)
	}
}
