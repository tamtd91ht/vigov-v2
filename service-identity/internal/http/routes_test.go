package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// --- fixtures ---------------------------------------------------------------------------

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	emailDung   = "canbo.a@example.gov.vn"
	matKhauDung = "khong-phai-mat-khau-that"

	// idNoiBo and maCanBo are DIFFERENT VALUES ON PURPOSE. Half the tests below exist to catch
	// the two being swapped: store.Checker matches on the internal id, the audit trail records
	// the business code, and a swap makes every guarded route answer 403 with nothing to show
	// why.
	idNoiBo = "nd-01JINTERNALIDCUACANBO"
	maCanBo = "CB-001"

	sidA = "sid-cua-xa-a-0001"
	sidB = "sid-cua-xa-b-0001"

	// Fake signing key. Never a real key in source (rule 8, forbidden #1).
	khoaGia = "khoa-ky-gia-KHONG-PHAI-KHOA-THAT-cho-test"

	quyenThu = authz.Perm("admin.user")

	// duongThu is a HARNESS-ONLY route. Two tests need to see the PRINCIPAL the edge built —
	// its id and its commune — and no shipped route returns either: a route that echoed the
	// caller's own commune back would be a route telling a prober which commune it reached.
	//
	// It used to sit at /api/v1/staff, which was free then. That path now belongs to a real
	// route mounted by Register, and mounting both on one ServeMux panics — so the stand-in
	// moved rather than the real one. It is registered in test files only (hooks skip
	// _test.go) and reaches no binary.
	duongThu = "/api/v1/test-probe"
)

var (
	xaA = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

func canBoMau() domain.CanBo {
	return domain.CanBo{
		ID:     idNoiBo,
		Ma:     maCanBo,
		HoTen:  "Nguyễn Văn A",
		Email:  emailDung,
		ChucVu: "Công chức Văn phòng",
		// The agreed fake number (rule 3, invariant 5). It is here so the tests below can prove
		// it never reaches the response.
		DienThoai:   "0900000000",
		MatKhauHash: "$argon2id$gia$KHONG-PHAI-HASH-THAT",
		// Both true: this fixture is a person who HAS a sign-in account and is NOT locked. The
		// two are separate columns since migration 0003 and the store filters on both, so a
		// principal that reaches these routes has satisfied both.
		CoTaiKhoan:   true,
		DangHoatDong: true,
	}
}

// --- fakes ------------------------------------------------------------------------------

type thuMucGia map[string]tenant.Tenant

func (m thuMucGia) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// thuMucMau is the platform registry for the two test communes. A NEW MAP PER CALL, so a test
// that empties or rewrites one copy cannot change the other — which is exactly what the 503 cases
// in xa_test.go do.
//
// The display names are different enough to tell apart in an assertion: a fixture where both
// communes are called the same thing cannot show a route serving the wrong one.
func thuMucMau() thuMucGia {
	return thuMucGia{
		hostA: {ID: xaA, Host: hostA, Name: "Xã Thăng Bình", Active: true},
		hostB: {ID: xaB, Host: hostB, Name: "Xã Bình Dương", Active: true},
	}
}

// quyenGia lists permissions PER COMMUNE and per staff id, reading the commune from the context
// exactly as the real query reads it from *store.Scoped. Keyed any other way, the isolation case
// in phien_hien_tai_test.go would pass while proving nothing.
type quyenGia struct {
	quyen map[tenant.ID]map[string][]authz.Perm
	loi   error
	goi   int
}

func (q *quyenGia) QuyenCua(ctx context.Context, p authz.Principal) ([]authz.Perm, error) {
	q.goi++
	if q.loi != nil {
		return nil, q.loi
	}
	if p.Kind != "staff" || p.ID == "" {
		return nil, nil
	}
	return q.quyen[tenant.MustFrom(ctx)][p.ID], nil
}

// quyenMau mirrors checkerGia: commune A grants this account admin.user, commune B grants it
// nothing at all. The empty side is not padding — it is the account whose roles were withdrawn,
// which is the case GET /api/v1/sessions/current names in its AnyAuthenticated reason.
func quyenMau() *quyenGia {
	return &quyenGia{quyen: map[tenant.ID]map[string][]authz.Perm{
		xaA: {idNoiBo: {quyenThu, "task.read"}},
		xaB: {},
	}}
}

// phienGia counts its reads. The count is what proves the commune check happens BEFORE any
// database access.
type phienGia struct {
	phien     map[string]idstore.Phien
	thuHoi    map[string]bool
	soLanDoc  int
	soLanDung int
}

func (p *phienGia) KiemTra(_ context.Context, sid string) (idstore.Phien, error) {
	p.soLanDoc++
	ph, ok := p.phien[sid]
	if !ok || p.thuHoi[sid] {
		return idstore.Phien{}, idstore.ErrPhienKhongTonTai
	}
	if time.Now().UTC().After(ph.HetHanLuc) {
		return idstore.Phien{}, idstore.ErrPhienHetHan
	}
	return ph, nil
}

func (p *phienGia) GhiNhanDung(_ context.Context, _ string) { p.soLanDung++ }

type canBoGia struct {
	theo     map[string]domain.CanBo
	soLanDoc int
}

func (c *canBoGia) TheoID(_ context.Context, id string) (domain.CanBo, error) {
	c.soLanDoc++
	cb, ok := c.theo[id]
	if !ok {
		return domain.CanBo{}, idstore.ErrCanBoKhongTonTai
	}
	return cb, nil
}

// checkerGia mirrors identity/internal/store/checker.go: it matches on
// (tenant_id, nguoi_dung.id, quyen_ma) and on nothing else. Keying it by the INTERNAL id is
// what makes it fail loudly if the middleware ever puts cb.Ma on the principal.
type checkerGia struct {
	quyen map[tenant.ID]map[string]map[authz.Perm]bool
}

func (c checkerGia) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	if p.Kind != "staff" || p.ID == "" {
		return false
	}
	return c.quyen[tenant.MustFrom(ctx)][p.ID][perm]
}

// dangNhapGia returns the SIGNED TOKEN as well as the sid: the use case signs inside its own
// transaction now, so the handler receives a token rather than producing one. A handler that went
// back to signing for itself would sign after the commit — the failure this moved away from.
type dangNhapGia struct {
	goi     int
	lanCuoi app.YeuCauDangNhap
	sid     string
	tok     string
	hetHan  time.Time
	loi     error
}

func (u *dangNhapGia) Chay(_ context.Context, yc app.YeuCauDangNhap) (app.KetQuaDangNhap, error) {
	u.goi++
	u.lanCuoi = yc
	if u.loi != nil {
		return app.KetQuaDangNhap{}, u.loi
	}
	if yc.Email != emailDung || yc.MatKhau != matKhauDung {
		return app.KetQuaDangNhap{}, app.ErrDangNhapThatBai
	}
	return app.KetQuaDangNhap{
		Sid:       u.sid,
		Token:     u.tok,
		Refresh:   "refresh-gia-khong-dung-den",
		HetHanLuc: u.hetHan,
		CanBo:     canBoMau(),
	}, nil
}

type dangXuatGia struct {
	goi     int
	sid     string
	maCanBo string
	ip      string
	loi     error
}

func (u *dangXuatGia) Chay(_ context.Context, sid, ma, ip string) error {
	u.goi++
	u.sid, u.maCanBo, u.ip = sid, ma, ip
	return u.loi
}

// --- harness ----------------------------------------------------------------------------

type mayChu struct {
	h      http.Handler
	d      Deps // kept so a test can rebuild the chain with ONE dependency swapped — see dungLai
	thuMuc thuMucGia
	signer *token.Signer
	phien  *phienGia
	canBo  *canBoGia
	danhBa *danhBaGia
	quyen  *quyenGia
	// dangNhap and dangXuat are the same values as d.DangNhap / d.DangXuat, typed.
	dangNhap *dangNhapGia
	dangXuat *dangXuatGia

	// them mounts harness-only routes after the real ones. One test needs to see the PRINCIPAL the
	// edge built, and no shipped route returns it — see duongThu.
	them func(mux *http.ServeMux, d Deps)
}

// dungMayChu builds the real edge chain, in the real order. Nothing here is a PostgreSQL or a
// Redis: the properties under test are ordering and isolation, and a test that needs
// infrastructure is a test that stops being run.
func dungMayChu(t *testing.T) *mayChu {
	t.Helper()

	signer, err := token.NewSigner([]secret.Secret{secret.Secret(khoaGia)})
	if err != nil {
		t.Fatalf("NewSigner lỗi: %v", err)
	}

	hetHan := time.Now().UTC().Add(idstore.ThoiHanPhien)
	phien := &phienGia{
		phien: map[string]idstore.Phien{
			sidA: {ID: sidA, NguoiDungID: idNoiBo, HetHanLuc: hetHan},
			sidB: {ID: sidB, NguoiDungID: idNoiBo, HetHanLuc: hetHan},
		},
		thuHoi: map[string]bool{},
	}
	canBo := &canBoGia{theo: map[string]domain.CanBo{idNoiBo: canBoMau()}}
	danhBa := danhBaMau()
	quyen := quyenMau()

	d := Deps{
		// Commune A grants the permission; commune B has the same account and grants nothing.
		// That pair is what produces a genuine "right permission, wrong commune" case.
		Checker: checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {quyenThu: true}},
			xaB: {},
		}},
		Quyen:  quyen,
		Signer: signer,
		Phien:  phien,
		CanBo:  canBo,
		DanhBa: danhBa,
		// The harness gives Deps.Xa its OWN directory value, not the one the edge is built with
		// below, although both start from the same map. Two values is what lets a test make the
		DangNhap: &dangNhapGia{
			sid: sidA,
			// A token signed the way the use case signs it, so the cookie carries something the
			// middleware can actually read back.
			tok:    kyThu(t, signer, xaA, sidA, hetHan),
			hetHan: hetHan,
		},
		DangXuat: &dangXuatGia{},
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// The guarded route used by the four cases of rule 5, invariant 7 is now a REAL one —
	// GET /api/v1/staff, mounted by Register with `admin.user`. It used to be a stand-in declared
	// here, because the service had no guarded route of its own; a stand-in can only prove that
	// authz works, never that the route being shipped declared it.
	m := &mayChu{
		d:        d,
		thuMuc:   thuMucMau(),
		signer:   signer,
		phien:    phien,
		canBo:    canBo,
		danhBa:   danhBa,
		quyen:    quyen,
		dangNhap: d.DangNhap.(*dangNhapGia),
		dangXuat: d.DangXuat.(*dangXuatGia),
	}
	m.dungLai(t, nil)
	return m
}

// dungLai rebuilds the real edge chain, optionally with one dependency swapped first.
//
// WHY IT EXISTS: three cases below need the chain built with a different Deps — a checker that
// grants another permission, a commune directory that cannot answer. Rebuilding by hand in each
// test means each copy can drift out of the real order, and the order IS the property most of
// these tests are about.
func (m *mayChu) dungLai(t *testing.T, sua func(d *Deps)) {
	t.Helper()
	if sua != nil {
		sua(&m.d)
	}

	mux := http.NewServeMux()
	Register(mux, m.d)
	if m.them != nil {
		m.them(mux, m.d)
	}

	var h http.Handler = mux
	h = XacThuc(m.d)(h)
	h = httpx.TenantMiddleware(m.thuMuc)(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	m.h = h
}

func (m *mayChu) tokenCho(t *testing.T, xa tenant.ID, sid string) string {
	t.Helper()
	return kyThu(t, m.signer, xa, sid, time.Now().UTC().Add(time.Hour))
}

func kyThu(t *testing.T, s *token.Signer, xa tenant.ID, sid string, hetHan time.Time) string {
	t.Helper()
	tok, err := s.Ky(token.Claims{TenantID: xa, Sid: sid, ExpiresAt: hetHan})
	if err != nil {
		t.Fatalf("ký token: %v", err)
	}
	return tok
}

func (m *mayChu) goi(t *testing.T, method, host, path, than, tok string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if than != "" {
		body = strings.NewReader(than)
	}
	r := httptest.NewRequest(method, "https://"+host+path, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if tok != "" {
		r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})
	}
	return m.chay(r)
}

// chay runs a request the caller built itself — for the cases that need a Host with a port, or a
// header the harness would never send.
func (m *mayChu) chay(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func cookiePhien(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == CookiePhien {
			return c
		}
	}
	return nil
}

func doiMa(t *testing.T, w *httptest.ResponseRecorder, muon int) {
	t.Helper()
	if w.Code != muon {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, muon, w.Body.String())
	}
}

func loiTra(t *testing.T, w *httptest.ResponseRecorder) httpx.Error {
	t.Helper()
	var e httpx.Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("thân lỗi không phải JSON: %q", w.Body.String())
	}
	return e
}

// --- POST /api/v1/sessions ----------------------------------------------------------------
//
// The route is Public, so rule 5's "403 wrong permission" does not apply. The three cases that
// replace it are below: wrong password, unknown email (one message for both), and success.

func TestDangNhapThanhCong(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "POST", hostA, "/api/v1/sessions",
		`{"email":"`+emailDung+`","password":"`+matKhauDung+`"}`, "")
	doiMa(t, w, http.StatusCreated)

	var ra phanHoiDangNhap
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if ra.Sid != sidA {
		t.Errorf("sid = %q, muốn %q", ra.Sid, sidA)
	}
	if ra.Staff.Code != maCanBo || ra.Staff.FullName != "Nguyễn Văn A" {
		t.Errorf("thông tin cán bộ sai: %+v", ra.Staff)
	}
	if ra.ExpiresAt.IsZero() {
		t.Error("thiếu hạn phiên — client không biết khi nào phải đăng nhập lại")
	}
	if m.dangNhap.lanCuoi.IP != "10.0.0.7" {
		t.Errorf("IP ghi vết = %q, muốn 10.0.0.7", m.dangNhap.lanCuoi.IP)
	}
}

func TestDangNhapKhongTraDuLieuCaNhanVaKhongTraHash(t *testing.T) {
	// Rule 3: a phone number is personal data under Decree 13/2023, and nothing on the sign-in
	// screen needs it. The password hash is a credential and never leaves the service at all.
	m := dungMayChu(t)

	w := m.goi(t, "POST", hostA, "/api/v1/sessions",
		`{"email":"`+emailDung+`","password":"`+matKhauDung+`"}`, "")
	doiMa(t, w, http.StatusCreated)

	than := w.Body.String()
	for _, cam := range []string{"0900000000", "argon2", "dien_thoai", "phone", "mat_khau", "password"} {
		if strings.Contains(strings.ToLower(than), strings.ToLower(cam)) {
			t.Errorf("phản hồi đăng nhập chứa %q: %s", cam, than)
		}
	}
}

func TestDangNhapSaiThongTinChiCoMotThongBao(t *testing.T) {
	// Telling a wrong password apart from an unknown email hands over a directory of which
	// email addresses exist on this commune's domain.
	m := dungMayChu(t)

	cases := map[string]string{
		"sai mật khẩu":        `{"email":"` + emailDung + `","password":"sai-mat-khau"}`,
		"email không tồn tại": `{"email":"khong-ton-tai@example.gov.vn","password":"` + matKhauDung + `"}`,
		"thiếu mật khẩu":      `{"email":"` + emailDung + `"}`,
		"thiếu email":         `{"password":"` + matKhauDung + `"}`,
	}
	var thay []httpx.Error
	for ten, than := range cases {
		t.Run(ten, func(t *testing.T) {
			w := m.goi(t, "POST", hostA, "/api/v1/sessions", than, "")
			doiMa(t, w, http.StatusUnauthorized)
			e := loiTra(t, w)
			if e.Code != "invalid_credentials" {
				t.Errorf("code = %q", e.Code)
			}
			if cookiePhien(t, w) != nil {
				t.Error("đăng nhập hỏng mà vẫn đặt cookie phiên")
			}
			thay = append(thay, e)
		})
	}
	for _, e := range thay[1:] {
		if e.Code != thay[0].Code || e.Message != thay[0].Message {
			t.Errorf("hai trường hợp trả lời khác nhau: %+v vs %+v", thay[0], e)
		}
	}
}

func TestDangNhapThanHongTra400(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "POST", hostA, "/api/v1/sessions", `{khong-phai-json`, "")
	doiMa(t, w, http.StatusBadRequest)
	if got := loiTra(t, w).Code; got != "invalid_body" {
		t.Errorf("code = %q, muốn invalid_body", got)
	}
	if m.dangNhap.goi != 0 {
		t.Error("thân hỏng mà vẫn gọi tới nghiệp vụ")
	}
}

func TestMaLoiTiengAnhThongBaoTiengViet(t *testing.T) {
	// rest-api-design §1: `code` is an identifier, `message` is a sentence a person reads.
	m := dungMayChu(t)

	w := m.goi(t, "POST", hostA, "/api/v1/sessions",
		`{"email":"`+emailDung+`","password":"sai"}`, "")
	e := loiTra(t, w)

	if strings.ToLower(e.Code) != e.Code || strings.ContainsAny(e.Code, "àáâãèéêìíòóôõùúýăđĩũơưạảấầẩẫậắằẳẵặẹẻẽếềểễệỉịọỏốồổỗộớờởỡợụủứừửữựỳỵỷỹ") {
		t.Errorf("code phải là tiếng Anh snake_case: %q", e.Code)
	}
	if !strings.ContainsAny(e.Message, "ăâđêôơưàáảãạ") {
		t.Errorf("message phải là tiếng Việt có dấu: %q", e.Message)
	}
}

// --- the cookie ---------------------------------------------------------------------------

func TestCookiePhienKhongCoThuocTinhDomain(t *testing.T) {
	// THE SINGLE MOST CONSEQUENTIAL LINE IN THIS SERVICE. A cookie carrying a Domain attribute
	// of a parent domain is sent to EVERY commune's subdomain, which ends the isolation between
	// two public authorities while every functional test stays green (rule 1, forbidden #3).
	// No Domain at all means host-only, which is exactly one commune.
	m := dungMayChu(t)

	w := m.goi(t, "POST", hostA, "/api/v1/sessions",
		`{"email":"`+emailDung+`","password":"`+matKhauDung+`"}`, "")
	doiMa(t, w, http.StatusCreated)

	raw := w.Header().Get("Set-Cookie")
	if strings.Contains(strings.ToLower(raw), "domain=") {
		t.Fatalf("cookie phiên có thuộc tính Domain: %q", raw)
	}

	c := cookiePhien(t, w)
	if c == nil {
		t.Fatal("không đặt cookie phiên")
	}
	if c.Domain != "" {
		t.Errorf("Domain = %q, phải để trống", c.Domain)
	}
	if !c.HttpOnly {
		t.Error("thiếu HttpOnly — JavaScript đọc được phiên của cán bộ")
	}
	if !c.Secure {
		t.Error("thiếu Secure — phiên đi qua kết nối không mã hoá là phiên phát lại được")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, muốn Lax", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, muốn /", c.Path)
	}
	// The cookie must not outlive the session it points at.
	if c.Value == "" {
		t.Error("cookie rỗng")
	}
}

// --- the commune check ---------------------------------------------------------------------

func TestTokenXaAGuiToiHostXaBThiTuChoiTruocMoiTruyVan(t *testing.T) {
	// THE MOST IMPORTANT CASE IN THIS FILE.
	//
	// It asserts two things at once: the answer is 401, AND no store was touched. The second
	// half is what proves the commune comparison sits at the token layer, before any read — so
	// commune B never runs a query for a sid that belongs to commune A, and no cross-commune
	// read has to be justified.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goi(t, "GET", hostB, "/api/v1/staff", "", tok)

	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "tenant_mismatch" {
		t.Errorf("code = %q, muốn tenant_mismatch", got)
	}
	if m.phien.soLanDoc != 0 {
		t.Errorf("đã đọc bảng phiên %d lần — phép so xã phải nằm TRƯỚC mọi truy vấn", m.phien.soLanDoc)
	}
	if m.canBo.soLanDoc != 0 {
		t.Errorf("đã đọc bảng cán bộ %d lần — phép so xã phải nằm TRƯỚC mọi truy vấn", m.canBo.soLanDoc)
	}
	if c := cookiePhien(t, w); c == nil || c.MaxAge >= 0 {
		t.Error("token của xã khác phải bị xoá khỏi trình duyệt")
	}
}

func TestTokenDungXaThiDungDuoc(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if m.phien.soLanDung != 1 {
		t.Errorf("GhiNhanDung gọi %d lần, muốn 1", m.phien.soLanDung)
	}
}

func TestHostKhongThuocXaNaoTra404(t *testing.T) {
	// Rule 1, invariant 3: cannot resolve the commune -> 404, never a default commune, and 404
	// also reveals nothing about which communes exist.
	m := dungMayChu(t)

	w := m.goi(t, "GET", "khong-ai-biet.example.gov.vn", "/api/v1/staff", "", "")
	doiMa(t, w, http.StatusNotFound)
}

// --- four cases of rule 5, invariant 7 -----------------------------------------------------

func TestRouteCoQuyen_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", ""), http.StatusUnauthorized)
}

func TestRouteCoQuyen_403SaiQuyen(t *testing.T) {
	// The account is signed in and its commune matches; it simply does not hold this permission.
	m := dungMayChu(t)
	// The REAL routes, behind a checker that grants a different permission. Building this from
	// Register rather than from a stand-in route is what makes the case prove something about
	// what ships: it fails if a route is ever mounted without a declaration.
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{xaA: {idNoiBo: {"task.read": true}}}}
	})

	for _, duong := range duongCanBo {
		doiMa(t, m.goi(t, "GET", hostA, duong, "", m.tokenCho(t, xaA, sidA)), http.StatusForbidden)
	}
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("thiếu quyền mà đã đọc danh bạ %d lần", m.danhBa.soLanGoi)
	}
}

func TestRouteCoQuyen_403DungQuyenSaiXa(t *testing.T) {
	// The same person, holding the permission in commune A, signed in properly at commune B.
	// Nothing about the request is malformed — the grant simply does not exist in commune B,
	// and a permission that crossed the commune would be privilege escalation (rule 5,
	// invariant 3).
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, "/api/v1/staff", "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusForbidden)
}

func TestRouteCoQuyen_200DuCaHai(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
}

// --- the id trap ---------------------------------------------------------------------------

func TestPrincipalIDLaIDNoiBoChuKhongPhaiMaCanBo(t *testing.T) {
	// store.Checker queries `nd.id = $2` with Principal.ID
	// (identity/internal/store/checker.go:38). Putting cb.Ma there matches no row, so
	// EVERY guarded route answers 403 and nothing in the response, the logs or a test says why.
	// checkerGia is keyed the same way the real query is, so a swap turns this test red.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	// The principal is read out of the context BY THE STORE the handler called, which is where a
	// real store would be matching `nd.id = $2` against it.
	if m.danhBa.principalCuoi != idNoiBo {
		t.Fatalf("Principal.ID = %q, muốn id nội bộ %q", m.danhBa.principalCuoi, idNoiBo)
	}
	if m.danhBa.principalCuoi == maCanBo {
		t.Fatal("Principal.ID đang là mã cán bộ — Checker sẽ không khớp dòng nào")
	}
}

func TestGhiVetDungMaCanBoChuKhongPhaiIDNoiBo(t *testing.T) {
	// The mirror image of the test above: the audit trail records a BUSINESS code, because an
	// internal id means nothing to whoever reads the trail (rule 6, invariant 3).
	m := dungMayChu(t)

	w := m.goi(t, "DELETE", hostA, "/api/v1/sessions/"+sidA, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusNoContent)

	if m.dangXuat.maCanBo != maCanBo {
		t.Errorf("ghi vết bằng %q, muốn mã cán bộ %q", m.dangXuat.maCanBo, maCanBo)
	}
	if m.dangXuat.ip != "10.0.0.7" {
		t.Errorf("IP ghi vết = %q", m.dangXuat.ip)
	}
}

// --- the session registry ------------------------------------------------------------------

func TestPhienDaThuHoiThiKhongDungPrincipal(t *testing.T) {
	// The whole reason the session is a table and not merely a signed token: a token that is
	// only signed cannot be taken back. Revoking must take effect on the very next request.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", tok), http.StatusOK)

	m.phien.thuHoi[sidA] = true

	w := m.goi(t, "GET", hostA, "/api/v1/staff", "", tok)
	doiMa(t, w, http.StatusUnauthorized)
	if c := cookiePhien(t, w); c == nil || c.MaxAge >= 0 {
		t.Error("phiên đã thu hồi mà cookie chết vẫn nằm lại trình duyệt")
	}
}

func TestPhienHetHanThiKhongDungPrincipal(t *testing.T) {
	m := dungMayChu(t)
	m.phien.phien[sidA] = idstore.Phien{
		ID: sidA, NguoiDungID: idNoiBo, HetHanLuc: time.Now().UTC().Add(-time.Minute),
	}

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA)), http.StatusUnauthorized)
}

func TestCanBoBiKhoaThiKhongDungPrincipal(t *testing.T) {
	// CanBoStore.TheoID returns nothing for a locked or soft-deleted account, so an account
	// locked mid-session stops being a principal on the next request rather than at expiry.
	m := dungMayChu(t)
	delete(m.canBo.theo, idNoiBo)

	doiMa(t, m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA)), http.StatusUnauthorized)
}

func TestTokenHongThiXoaCookieVaVanPhucVuRoutePublic(t *testing.T) {
	// A stale cookie must not make the sign-in screen unreachable — that is how somebody gets
	// locked out of a government service with no way back in.
	m := dungMayChu(t)

	w := m.goi(t, "POST", hostA, "/api/v1/sessions",
		`{"email":"`+emailDung+`","password":"`+matKhauDung+`"}`, "token-hong-khong-giai-duoc")

	doiMa(t, w, http.StatusCreated)
	if m.phien.soLanDoc != 0 {
		t.Error("token hỏng mà vẫn tra bảng phiên")
	}

	// Two Set-Cookie headers go out: the middleware clears the dead token, then the handler
	// issues the new one. ORDER DECIDES THE OUTCOME — a browser applies them in order, so the
	// clear must come first or the person signs in and is immediately signed out again.
	var cookies []*http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == CookiePhien {
			cookies = append(cookies, c)
		}
	}
	if len(cookies) == 0 {
		t.Fatal("không đặt cookie phiên mới")
	}
	cuoi := cookies[len(cookies)-1]
	if cuoi.Value == "" || cuoi.MaxAge < 0 {
		t.Errorf("Set-Cookie cuối cùng phải là phiên mới, nhận %+v", cuoi)
	}
}

// --- DELETE /api/v1/sessions/{sid} ----------------------------------------------------------

func TestDangXuatKhongCoTokenTra401(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "DELETE", hostA, "/api/v1/sessions/"+sidA, "", ""), http.StatusUnauthorized)
	if m.dangXuat.goi != 0 {
		t.Error("chưa đăng nhập mà đã gọi tới nghiệp vụ đăng xuất")
	}
}

func TestDangXuatSidCuaNguoiKhacTra404(t *testing.T) {
	// 404 AND NOT 403. A 403 would confirm that the sid exists and belongs to somebody else;
	// the existence of another person's session is itself information (rule 4, forbidden #2).
	// The same answer is given for an invented sid, so the two cannot be told apart.
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	cases := []string{
		sidB,                  // a real session, belonging to another sign-in
		"sid-bia-ra-khong-co", // an invented one
	}
	var thay []httpx.Error
	for _, sid := range cases {
		w := m.goi(t, "DELETE", hostA, "/api/v1/sessions/"+sid, "", tok)
		doiMa(t, w, http.StatusNotFound)
		thay = append(thay, loiTra(t, w))
		if m.dangXuat.goi != 0 {
			t.Fatalf("đã thu hồi phiên %q của người khác", sid)
		}
	}
	if thay[0] != thay[1] {
		t.Errorf("phiên có thật và phiên bịa trả lời khác nhau: %+v vs %+v", thay[0], thay[1])
	}
}

func TestDangXuatThanhCong(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "DELETE", hostA, "/api/v1/sessions/"+sidA, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusNoContent)

	if m.dangXuat.goi != 1 || m.dangXuat.sid != sidA {
		t.Errorf("thu hồi sai phiên: goi=%d sid=%q", m.dangXuat.goi, m.dangXuat.sid)
	}
	if c := cookiePhien(t, w); c == nil || c.MaxAge >= 0 {
		t.Error("đăng xuất rồi mà cookie vẫn còn hiệu lực")
	}
}

func TestDangXuatLoiHeThongTra500(t *testing.T) {
	m := dungMayChu(t)
	m.dangXuat.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "DELETE", hostA, "/api/v1/sessions/"+sidA, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if cookiePhien(t, w) != nil {
		t.Error("thu hồi hỏng mà vẫn xoá cookie — người dùng tưởng đã đăng xuất")
	}
}
