package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/identity/internal/domain"
	idstore "github.com/vihat/vigov/identity/internal/store"
)

// WHAT THIS FILE IS FOR: the two read routes of the staff register are the first routes in this
// service that return somebody's personal data to somebody else. Three things have to hold on
// every response, and each of them fails silently if it stops holding:
//
//  1. the phone number is MASKED — open question #11 is still open, so masking is the fail-closed
//     default and nothing in the 33 seeded permission keys can lift it;
//  2. the password hash is ABSENT — not "empty", absent, at every one of the three layers;
//  3. an id from another commune is 404, never 403 — a 403 confirms the record exists.
//
// The four cases of rule 5, invariant 7 (401 · 403 wrong permission · 403 right permission wrong
// commune · 200) are asserted for BOTH routes, and the wrong-permission case lives in
// routes_test.go where the checker is rebuilt.

// duongCanBo is both routes, so a case written once cannot be added to one route and forgotten
// on the other — which is exactly how a list gets a permission and its detail route does not.
var duongCanBo = []string{"/api/v1/staff", "/api/v1/staff/" + idNoiBo}

// --- fixtures -----------------------------------------------------------------------------

// The register of commune A. Ordered by `ma` ascending, which is SapXepCanBo's default.
//
// The set is deliberately mixed: CB-001 is an account, CB-002 is a directory-only person who has
// never signed in, CB-003 is a locked account. A fixture of three identical rows cannot tell a
// register that returns everybody from one that quietly filters.
func danhBaXaA() []domain.CanBoTomTat {
	dangNhapLuc := time.Date(2026, 9, 16, 11, 10, 38, 0, time.UTC)
	tao := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return []domain.CanBoTomTat{
		{
			ID: idNoiBo, Ma: maCanBo, HoTen: "Nguyễn Văn A", Email: emailDung,
			ChucVu: "Công chức Văn phòng", BoPhanID: "bp-001", VaiTroID: "vt-001",
			// The agreed fake number (rule 3, invariant 5), here so the tests can prove the raw
			// value never reaches a response body.
			DienThoai: "0900000000", CoTaiKhoan: true, DangHoatDong: true,
			DangNhapGanNhat: &dangNhapLuc, TaoLuc: tao,
		},
		{
			ID: "nd-02", Ma: "CB-002", HoTen: "Trần Thị B", Email: "canbo.b@example.gov.vn",
			ChucVu: "Trưởng thôn", BoPhanID: "bp-002",
			DienThoai: "0900000002", CoTaiKhoan: false, DangHoatDong: true,
			TaoLuc: tao.Add(time.Hour),
		},
		{
			ID: "nd-03", Ma: "CB-003", HoTen: "Lê Văn C", Email: "canbo.c@example.gov.vn",
			ChucVu: "Kế toán", BoPhanID: "bp-001", VaiTroID: "vt-002",
			DienThoai: "0900000003", CoTaiKhoan: true, DangHoatDong: false,
			TaoLuc: tao.Add(2 * time.Hour),
		},
	}
}

const idCuaXaB = "nd-cua-xa-b-0001"

func danhBaMau() *danhBaGia {
	return &danhBaGia{theo: map[tenant.ID][]domain.CanBoTomTat{
		xaA: danhBaXaA(),
		// Commune B holds ONE record, and commune A never holds its id. That pair is what makes
		// "another commune's id answers 404" a real case rather than a lookup of something that
		// does not exist anywhere.
		xaB: {{
			ID: idCuaXaB, Ma: "CB-001", HoTen: "Phạm Thị D", Email: "canbo.d@example.gov.vn",
			DienThoai: "0900000004", CoTaiKhoan: true, DangHoatDong: true,
		}},
	}}
}

// --- the fake register --------------------------------------------------------------------

// danhBaGia is the staff register, KEYED BY COMMUNE, reading the commune from the context the
// same way *store.Scoped does. Keying it any other way would make every isolation case below
// pass without proving anything.
//
// IT PAGES ONLY BY THE DEFAULT SORT (`ma` ascending) and refuses anything else, rather than
// pretending. The keyset walk itself — the part where a missing tie-break silently skips a
// record — is proven against the REAL SQL in
// identity/internal/store/can_bo_danh_sach_test.go. What is proven here is that the
// handler hands the cursor back and forth without losing it.
type danhBaGia struct {
	theo          map[tenant.ID][]domain.CanBoTomTat
	loi           error
	soLanGoi      int
	yeuCauCuoi    page.Request
	principalCuoi string
	xaCuoi        tenant.ID
}

func (d *danhBaGia) ghiNhan(ctx context.Context) {
	d.soLanGoi++
	d.xaCuoi = tenant.MustFrom(ctx)
	if p, ok := authz.From(ctx); ok {
		d.principalCuoi = p.ID
	}
}

func (d *danhBaGia) DanhSach(ctx context.Context, yc page.Request) (page.Result[domain.CanBoTomTat], error) {
	d.ghiNhan(ctx)
	d.yeuCauCuoi = yc
	kq := page.NewResult[domain.CanBoTomTat]()
	if d.loi != nil {
		return kq, d.loi
	}
	if yc.Column().Param != "code" {
		return kq, errors.New("danhBaGia: chỉ phục vụ sắp xếp mặc định theo mã")
	}

	ds := d.theo[tenant.MustFrom(ctx)]
	if a, ok := yc.After(); ok {
		i := 0
		for i < len(ds) && !sauMoc(ds[i], a) {
			i++
		}
		ds = ds[i:]
	}

	var cuoi domain.CanBoTomTat
	for _, cb := range ds {
		if len(kq.Items) == yc.Limit() {
			kq.HasMore = true
			kq.NextCursor = page.Encode(yc.Column(), yc.Dir(),
				page.Anchor{Key: page.TextKey(cuoi.Ma), ID: cuoi.ID})
			break
		}
		kq.Items = append(kq.Items, cb)
		cuoi = cb
	}
	return kq, nil
}

// sauMoc is `(ma, id) > (key, id)` — the row comparison pkg/store builds, INCLUDING the id
// tie-break. Without the second half two people sharing a code would straddle a page boundary
// and one of them would never be shown.
func sauMoc(cb domain.CanBoTomTat, a page.Anchor) bool {
	if cb.Ma != a.Key.Text() {
		return cb.Ma > a.Key.Text()
	}
	return cb.ID > a.ID
}

func (d *danhBaGia) ChiTiet(ctx context.Context, id string) (domain.CanBoTomTat, error) {
	d.ghiNhan(ctx)
	if d.loi != nil {
		return domain.CanBoTomTat{}, d.loi
	}
	for _, cb := range d.theo[tenant.MustFrom(ctx)] {
		if cb.ID == id {
			return cb, nil
		}
	}
	return domain.CanBoTomTat{}, idstore.ErrCanBoKhongTonTai
}

// --- four cases of rule 5, invariant 7, for BOTH routes -------------------------------------
//
// The "403 wrong permission" case is in routes_test.go: it needs a different checker, which
// means rebuilding the whole chain.

func TestCanBo_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	for _, duong := range duongCanBo {
		doiMa(t, m.goi(t, "GET", hostA, duong, "", ""), http.StatusUnauthorized)
	}
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("chưa đăng nhập mà đã đọc danh bạ %d lần", m.danhBa.soLanGoi)
	}
}

func TestCanBo_403DungQuyenSaiXa(t *testing.T) {
	// The same person, holding admin.user in commune A, properly signed in at commune B. Nothing
	// about the request is malformed; the grant simply does not exist in commune B, and a
	// permission that crossed the commune would be privilege escalation (rule 5, invariant 3).
	m := dungMayChu(t)

	for _, duong := range duongCanBo {
		doiMa(t, m.goi(t, "GET", hostB, duong, "", m.tokenCho(t, xaB, sidB)), http.StatusForbidden)
	}
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("sai xã mà đã đọc danh bạ %d lần — phép kiểm quyền phải chặn trước kho", m.danhBa.soLanGoi)
	}
}

func TestCanBo_200DuCaHai(t *testing.T) {
	m := dungMayChu(t)

	for _, duong := range duongCanBo {
		doiMa(t, m.goi(t, "GET", hostA, duong, "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	}
	if m.danhBa.xaCuoi != xaA {
		t.Errorf("kho được gọi với xã %q, muốn %q", m.danhBa.xaCuoi, xaA)
	}
}

// --- rule 3: what leaves the API ------------------------------------------------------------

func TestCanBoCheSoDienThoai(t *testing.T) {
	// MUTATION THAT MUST TURN THIS RED: drop privacy.MaskPhone from raNgoai and return
	// cb.DienThoai. Nothing else in the suite notices — the field is still present, still a
	// string, still the right person's.
	//
	// Masking is the fail-closed default while open question #11 is unanswered: no seeded
	// permission key means "see a staff member's full details", so there is no explicit path
	// rule 3, invariant 3 could open.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, duong := range duongCanBo {
		w := m.goi(t, "GET", hostA, duong, "", tok)
		doiMa(t, w, http.StatusOK)
		than := w.Body.String()

		if strings.Contains(than, "0900000000") {
			t.Errorf("%s trả về số điện thoại đầy đủ: %s", duong, than)
		}
		if !strings.Contains(than, "09****0000") {
			t.Errorf("%s không có số đã che — mong 09****0000: %s", duong, than)
		}
	}
}

func TestCanBoKhongBaoGioTraMatKhauHash(t *testing.T) {
	// Three independent layers keep the hash in: the store does not SELECT it,
	// domain.CanBoTomTat has no field for it, and canBoTomTat has none either. This asserts the
	// observable end of all three at once — the bytes on the wire.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	for _, duong := range duongCanBo {
		w := m.goi(t, "GET", hostA, duong, "", tok)
		doiMa(t, w, http.StatusOK)
		than := strings.ToLower(w.Body.String())
		for _, cam := range []string{"argon2", "mat_khau", "password", "hash", "$2a$"} {
			if strings.Contains(than, cam) {
				t.Errorf("%s trả về %q trong thân: %s", duong, cam, w.Body.String())
			}
		}
	}
}

func TestCanBoTraDuTruongManHinhCan(t *testing.T) {
	// The screen (14-cau-hinh.md §3) shows name, position, department, phone, last sign-in and
	// status. Name, position and department are NOT masked: admin.user is the explicit permission
	// guarding this screen, and a staff register with masked names is not a register.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/staff/"+idNoiBo, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var ra canBoTomTat
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	muon := canBoTomTat{
		ID: idNoiBo, Code: maCanBo, FullName: "Nguyễn Văn A", Email: emailDung,
		Position: "Công chức Văn phòng", DepartmentID: "bp-001", RoleID: "vt-001",
		Phone: "09****0000", HasAccount: true, Active: true,
	}
	ra.LastLoginAt, ra.CreatedAt = nil, time.Time{} // compared separately, below
	if ra != muon {
		t.Errorf("chi tiết = %+v\nmuốn        %+v", ra, muon)
	}
}

func TestCanBoChuaDangNhapThiLastLoginLaNull(t *testing.T) {
	// A zero time.Time would serialise as year 1 and the screen would render it as a date. The
	// screen has a distinct label — "Chưa đăng nhập" — and null is what carries it.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/staff/nd-02", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var ra map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatal(err)
	}
	if v, co := ra["last_login_at"]; !co || v != nil {
		t.Errorf("last_login_at = %#v, muốn null", v)
	}
}

// --- rule 4, forbidden #2: 404 and not 403 ---------------------------------------------------

func TestCanBoIDCuaXaKhacTra404GiongHetIDBiaRa(t *testing.T) {
	// MUTATION THAT MUST TURN THIS RED: answer 403 (or any different body) for a record that
	// exists in another commune. A 403 confirms the record exists somewhere, and the existence of
	// another authority's staff record is itself information.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	var thay []string
	for _, id := range []string{
		idCuaXaB,                   // a real record — in commune B
		"nd-bia-ra-khong-co-o-dau", // one that exists nowhere
	} {
		w := m.goi(t, "GET", hostA, "/api/v1/staff/"+id, "", tok)
		doiMa(t, w, http.StatusNotFound)
		if got := loiTra(t, w).Code; got != "staff_not_found" {
			t.Errorf("code = %q, muốn staff_not_found", got)
		}
		thay = append(thay, w.Body.String())
	}
	if thay[0] != thay[1] {
		t.Errorf("bản ghi của xã khác và id bịa ra trả lời khác nhau:\n%s\n%s", thay[0], thay[1])
	}
}

// --- rule 1: the register of ONE commune -----------------------------------------------------

func TestCanBoChiTraDuLieuCuaXaDangGoi(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var ra trangCanBo
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatal(err)
	}
	for _, cb := range ra.Items {
		if cb.ID == idCuaXaB {
			t.Fatalf("xã A nhận được bản ghi %s của xã B — RÒ RỈ GIỮA HAI XÃ", cb.ID)
		}
	}
}

// --- the two kinds of record -----------------------------------------------------------------

func TestCanBoTraCaTaiKhoanLanNguoiChiCoTrongDanhBa(t *testing.T) {
	// One `nguoi_dung` table serves two screens since migration 0003. Filtering on co_tai_khoan
	// here would answer, on behalf of one screen, a question nobody has asked — and it would hide
	// the 26 people the staff directory exists to list.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var ra trangCanBo
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatal(err)
	}
	var coTK, khongTK, biKhoa int
	for _, cb := range ra.Items {
		switch {
		case !cb.HasAccount:
			khongTK++
		case !cb.Active:
			biKhoa++
		default:
			coTK++
		}
	}
	if coTK == 0 || khongTK == 0 || biKhoa == 0 {
		t.Fatalf("danh sách thiếu một loại bản ghi: có tài khoản=%d, chỉ danh bạ=%d, bị khoá=%d — %+v",
			coTK, khongTK, biKhoa, ra.Items)
	}
}

// --- pagination -------------------------------------------------------------------------------

func TestCanBoPhanTrangNoiDungChoKhongLapKhongSot(t *testing.T) {
	// MUTATION THAT MUST TURN THIS RED: hand back the anchor of the FIRST row of the page instead
	// of the last, or drop next_cursor from the response. Both leave a perfectly valid-looking
	// first page.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	var thay []string
	conTro := ""
	for i := 0; ; i++ {
		if i > 10 {
			t.Fatal("con trỏ không tiến — vòng lặp vô hạn, đây là cách nó biểu hiện trên sản phẩm")
		}
		duong := "/api/v1/staff?limit=1"
		if conTro != "" {
			duong += "&cursor=" + url.QueryEscape(conTro)
		}
		w := m.goi(t, "GET", hostA, duong, "", tok)
		doiMa(t, w, http.StatusOK)

		var trang trangCanBo
		if err := json.Unmarshal(w.Body.Bytes(), &trang); err != nil {
			t.Fatal(err)
		}
		if len(trang.Items) > 1 {
			t.Fatalf("xin 1 bản ghi, nhận %d", len(trang.Items))
		}
		for _, cb := range trang.Items {
			thay = append(thay, cb.ID)
		}
		if !trang.HasMore {
			if trang.NextCursor != "" {
				t.Error("has_more=false mà vẫn có next_cursor")
			}
			break
		}
		if trang.NextCursor == "" {
			t.Fatal("has_more=true mà không có next_cursor — client không đi tiếp được")
		}
		conTro = trang.NextCursor
	}

	var muon []string
	for _, cb := range danhBaXaA() {
		muon = append(muon, cb.ID)
	}
	if !reflect.DeepEqual(thay, muon) {
		t.Fatalf("đi hết các trang được %v, muốn %v — lặp hoặc sót bản ghi", thay, muon)
	}
}

func TestCanBoConTroHongTra400VaKhongChamKho(t *testing.T) {
	// A rejected page request must run no statement at all. Falling back to "start from the
	// beginning" would show page 1 to somebody who believes they are on page 4.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/staff?cursor=khong-phai-con-tro", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusBadRequest)
	if got := loiTra(t, w).Code; got != "invalid_cursor" {
		t.Errorf("code = %q, muốn invalid_cursor", got)
	}
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("con trỏ hỏng mà vẫn đọc kho %d lần", m.danhBa.soLanGoi)
	}
}

func TestCanBoSapXepNgoaiDanhSachTrangTra400(t *testing.T) {
	// ?sort=ho_ten would put a person's name in a URL, an access log and a browser history
	// (rule 3, forbidden #4). The allowlist is closed, and a column outside it is refused rather
	// than ignored.
	m := dungMayChu(t)

	for _, truyVan := range []string{"?sort=ho_ten", "?sort=dien_thoai", "?order=cheo", "?limit=0"} {
		w := m.goi(t, "GET", hostA, "/api/v1/staff"+truyVan, "", m.tokenCho(t, xaA, sidA))
		doiMa(t, w, http.StatusBadRequest)
	}
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("yêu cầu bị từ chối mà vẫn đọc kho %d lần", m.danhBa.soLanGoi)
	}
}

func TestCanBoLimitVuotTranThiBiCatChuKhongLoi(t *testing.T) {
	// §5 #2: a client asking for 10.000 is served the maximum and a cursor to continue with —
	// not an error, and not 10.000 rows out of a process shared by 200+ communes.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/staff?limit=10000", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if got := m.danhBa.yeuCauCuoi.Limit(); got != page.MaxLimit {
		t.Errorf("limit tới kho = %d, muốn trần %d", got, page.MaxLimit)
	}
}

func TestCanBoXaRongTraMangRongChuKhongPhaiNull(t *testing.T) {
	// A newly onboarded commune has an empty register. `null` and `[]` are two shapes, and a
	// client that has to handle both handles one of them wrong.
	m := dungMayChu(t)
	m.danhBa.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("thân = %s, muốn items: []", w.Body.String())
	}
}

// --- failures ----------------------------------------------------------------------------------

func TestCanBoLoiKhoTra500KhongLoDuLieu(t *testing.T) {
	m := dungMayChu(t)
	m.danhBa.loi = errors.New("cơ sở dữ liệu không phản hồi: nguoi_dung")

	for _, duong := range duongCanBo {
		w := m.goi(t, "GET", hostA, duong, "", m.tokenCho(t, xaA, sidA))
		doiMa(t, w, http.StatusInternalServerError)
		// The error the store produced never reaches the client: it can quote a statement, a
		// table, and one day a value from a row (rule 3, forbidden #3).
		if strings.Contains(w.Body.String(), "nguoi_dung") {
			t.Errorf("%s để lộ lỗi nội bộ: %s", duong, w.Body.String())
		}
	}
}

// --- the shape published in the contract --------------------------------------------------------

func TestTrangCanBoTrungHinhDangVoiPage(t *testing.T) {
	// trangCanBo restates page.Result because tools/apidoc cannot describe a generic
	// instantiation, and a reply type it cannot describe is a route silently missing from
	// kb/20-contracts/openapi.json. A copy is allowed only while something pins it to the
	// original — rule 9, forbidden #2 is about copies that DRIFT.
	//
	// MUTATION THAT MUST TURN THIS RED: rename `next_cursor` to `cursor` on either side.
	goc := reflect.TypeOf(page.Result[canBoTomTat]{})
	sao := reflect.TypeOf(trangCanBo{})

	if goc.NumField() != sao.NumField() {
		t.Fatalf("page.Result có %d trường, trangCanBo có %d", goc.NumField(), sao.NumField())
	}
	for i := 0; i < goc.NumField(); i++ {
		a, b := goc.Field(i), sao.Field(i)
		if a.Name != b.Name {
			t.Errorf("trường %d: page.Result gọi là %q, trangCanBo gọi là %q", i, a.Name, b.Name)
		}
		if a.Tag.Get("json") != b.Tag.Get("json") {
			t.Errorf("trường %q: thẻ json %q vs %q", a.Name, a.Tag.Get("json"), b.Tag.Get("json"))
		}
		if a.Type.Kind() != b.Type.Kind() {
			t.Errorf("trường %q: kiểu %v vs %v", a.Name, a.Type, b.Type)
		}
	}
}

func TestSapXepCanBoKhongMoCotDuLieuCaNhanVaKhongMoCotNULL(t *testing.T) {
	// The allowlist is what decides which column names may appear in a URL. Two kinds must never
	// be on it, and neither failure is visible from a passing screen:
	//
	//   ho_ten / dien_thoai   a sort key travels in the URL, the access log and the browser
	//                         history (rule 3, forbidden #4);
	//   dang_nhap_gan_nhat    NULLABLE — `(col, id) > ($2, $3)` is NULL for a NULL col, so every
	//                         person who has never signed in vanishes from every page after the
	//                         first.
	for _, cot := range []string{"ho_ten", "full_name", "dien_thoai", "phone",
		"dang_nhap_gan_nhat", "last_login_at", "email"} {
		if _, err := page.New(idstore.SapXepCanBo, cot, "", "", ""); err == nil {
			t.Errorf("sắp xếp theo %q được chấp nhận", cot)
		}
	}
	for _, cot := range []string{"code", "created_at"} {
		if _, err := page.New(idstore.SapXepCanBo, cot, "", "", ""); err != nil {
			t.Errorf("sắp xếp theo %q bị từ chối: %v", cot, err)
		}
	}
}
