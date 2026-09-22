package http

import (
	"context"
	"encoding/json"
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

// THE DEFECT CLASS THIS FILE CLOSES: two different token.Signer instances wired into one
// service.
//
// TWO PLACES HOLD A SIGNER, and both are correct on their own. app.DangNhap signs the session
// token INSIDE its transaction, so that a signing failure rolls back the session row and its
// audit entry together — otherwise the archive of a public authority states that somebody signed
// in while no cookie was ever issued, in an append-only entry that cannot be corrected (rule 6,
// invariant 4). svchttp.Deps.Signer is what XacThuc verifies the incoming cookie with.
//
// Wire those two from two separate NewSigner calls and the token issued at sign-in cannot be
// read on the very next request: the person signs in successfully, the browser stores a cookie,
// and every following request is treated as signed out. Nothing turns red — each unit test
// passes, because each side is self-consistent. The seam between them is only visible end to
// end, which is what this file is.
//
// The fixtures (hostA, xaA, sidA, idNoiBo, canBoMau, the fakes) come from routes_test.go, same
// package.

// dangNhapKyThat is the sign-in use case reduced to the one thing that matters here: it signs
// with the signer it was GIVEN, exactly as app.DangNhap does, and returns the token in
// KetQuaDangNhap the way the handler expects to receive it.
//
// It takes app.KyToken — the same interface app.DangNhap takes — so the test wires the signing
// side the way production wires it, and the two signers can deliberately be made different.
type dangNhapKyThat struct {
	ky     app.KyToken
	sid    string
	hetHan time.Time
}

func (u *dangNhapKyThat) Chay(ctx context.Context, yc app.YeuCauDangNhap) (app.KetQuaDangNhap, error) {
	if yc.Email != emailDung || yc.MatKhau != matKhauDung {
		return app.KetQuaDangNhap{}, app.ErrDangNhapThatBai
	}
	// The commune comes from the context the edge established, never from the request body.
	tok, err := u.ky.Ky(token.Claims{
		TenantID:  tenant.MustFrom(ctx),
		Sid:       u.sid,
		ExpiresAt: u.hetHan,
	})
	if err != nil {
		return app.KetQuaDangNhap{}, err
	}
	return app.KetQuaDangNhap{
		Sid:       u.sid,
		Token:     tok,
		HetHanLuc: u.hetHan,
		CanBo:     canBoMau(),
	}, nil
}

func signerThu(t *testing.T, khoa string) *token.Signer {
	t.Helper()
	s, err := token.NewSigner([]secret.Secret{secret.Secret(khoa)})
	if err != nil {
		t.Fatalf("NewSigner lỗi: %v", err)
	}
	return s
}

// dungMayChuHaiSigner builds the REAL edge chain, the REAL routes and the REAL middleware, with
// the signing side and the verifying side given separately so a mis-wiring can be expressed.
//
// Production passes the same pointer to both — see identity/cmd/server/main.go, step 5.
func dungMayChuHaiSigner(t *testing.T, kySigner, giaiSigner *token.Signer) http.Handler {
	t.Helper()

	hetHan := time.Now().UTC().Add(idstore.ThoiHanPhien)
	d := Deps{
		Checker: checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {quyenThu: true}},
		}},
		Signer: giaiSigner,
		Phien: &phienGia{
			phien:  map[string]idstore.Phien{sidA: {ID: sidA, NguoiDungID: idNoiBo, HetHanLuc: hetHan}},
			thuHoi: map[string]bool{},
		},
		Quyen:     quyenMau(),
		VaiTro:    vaiTroMau(),
		BoPhan:    boPhanMau(),
		VaiTroMuc: vaiTroMucMau(),
		MaTran:    maTranMau(),
		// The three reference reads are wired here for one reason only: Register REFUSES incomplete
		// Deps at construction, so this harness cannot build the real route table without them.
		// Nothing in this file calls those routes — it is about the session staying readable across
		// two requests.
		ThonToDanPho:   thonToDanPhoMau(),
		LoaiDonViDanCu: loaiDonViDanCuMau(),
		KhoiNhiemVu:    khoiNhiemVuMau(),
		// The three calendar stores, for the same reason and with the same caveat: Register refuses
		// incomplete Deps whatever it mounts, and today it mounts no calendar route at all — the URL
		// resource names are being asked rather than guessed (ADR 0011).
		LichLamViec: lichLamViecMau(),
		NgayNghiLe:  ngayNghiLeMau(),
		NgayLamBu:   ngayLamBuMau(),
		// The deadline table, read and write. Unlike the calendar these ARE mounted — three routes
		// under `admin.sla` — so Register would refuse this Deps without them.
		SLA:    slaMau(),
		GhiSLA: ghiSLAMau(),
		CanBo:  &canBoGia{theo: map[string]domain.CanBo{idNoiBo: canBoMau()}},
		DanhBa: danhBaMau(),
		// Same reason again: the five write routes of the register are mounted by Register, so the
		// use case behind them has to be wired even though nothing in this file calls them.
		GhiDanhBa: ghiDanhBaMau(),
		TaiKhoan:  taiKhoanMau(),
		DangNhap:  &dangNhapKyThat{ky: kySigner, sid: sidA, hetHan: hetHan},
		DangXuat:  &dangXuatGia{},
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	mux := http.NewServeMux()
	Register(mux, d)
	mux.Handle("GET "+duongThu,
		authz.RequirePermission(d.Checker, quyenThu)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p, _ := authz.From(r.Context())
				vietJSON(w, http.StatusOK, map[string]string{"principal_id": p.ID})
			})))

	var h http.Handler = mux
	h = XacThuc(d)(h)
	h = httpx.TenantMiddleware(thuMucGia{hostA: {ID: xaA, Host: hostA, Active: true}})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return h
}

// dangNhapLay performs a real sign-in and returns the session cookie the browser would keep.
func dangNhapLay(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()

	r := httptest.NewRequest("POST", "https://"+hostA+"/api/v1/sessions",
		strings.NewReader(`{"email":"`+emailDung+`","password":"`+matKhauDung+`"}`))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("đăng nhập trả %d, muốn 201 — thân: %s", w.Code, w.Body.String())
	}
	c := cookiePhien(t, w)
	if c == nil || c.Value == "" {
		t.Fatal("đăng nhập thành công mà không đặt cookie phiên")
	}
	return c
}

func goiVoiCookie(t *testing.T, h http.Handler, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest("GET", "https://"+hostA+duongThu, nil)
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.AddCookie(&http.Cookie{Name: CookiePhien, Value: c.Value})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestDangNhapXongThiCookieDungDuocNgayORequestKeTiep(t *testing.T) {
	// THE END-TO-END CASE. Sign in through the real handler, take the cookie the real handler
	// set, and send it back through the real XacThuc: a Principal has to come out the other end.
	//
	// This is the only test that fails when the signing side and the verifying side are wired to
	// two different signers, and that failure is invisible everywhere else — the sign-in returns
	// 201 either way.
	signer := signerThu(t, khoaGia)
	h := dungMayChuHaiSigner(t, signer, signer)

	c := dangNhapLay(t, h)
	w := goiVoiCookie(t, h, c)

	if w.Code != http.StatusOK {
		t.Fatalf("request ngay sau khi đăng nhập trả %d, muốn 200 — cán bộ đăng nhập xong "+
			"lập tức bị coi là chưa đăng nhập. Thân: %s", w.Code, w.Body.String())
	}
	var ra map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if ra["principal_id"] != idNoiBo {
		t.Fatalf("Principal.ID = %q, muốn id nội bộ %q", ra["principal_id"], idNoiBo)
	}
}

func TestNoiHaiSignerKhacNhauThiTokenVuaPhatKhongGiaiDuoc(t *testing.T) {
	// THE MIS-WIRING, WRITTEN DOWN. This is what identity/cmd/server/main.go must never
	// be allowed to become: app.DangNhap given one signer, svchttp.Deps given another.
	//
	// Note what does NOT fail: the sign-in still answers 201 and still sets a cookie. Only the
	// next request tells the truth, which is why the test above has to exist and has to be end
	// to end.
	h := dungMayChuHaiSigner(t,
		signerThu(t, khoaGia),
		signerThu(t, khoaGia+"-mot-khoa-khac-hoan-toan"))

	c := dangNhapLay(t, h)
	w := goiVoiCookie(t, h, c)

	if w.Code == http.StatusOK {
		t.Fatal("hai signer khác nhau mà token vẫn giải được — bài test trên không còn bắt được lỗi này")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("mã trạng thái = %d, muốn 401", w.Code)
	}
}
