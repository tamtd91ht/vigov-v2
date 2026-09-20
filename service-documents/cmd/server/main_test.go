package main

// What this test defends: THE WIRING, not the middleware.
//
// core/staffauth proves the four behaviours of the authentication middleware and core/httpx proves
// what TenantMiddleware does. Neither of them can see whether THIS binary installs them. A
// middleware deleted from dungBien leaves a service that starts, serves and answers — and answers
// 401 to every member of staff holding a perfectly valid session, which is the exact state this
// service shipped in until today. Nothing else in this repository turns red for that.
//
// So this test drives the chain dungBien actually builds, with the REAL route mounted behind it.

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

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/staffauth"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
	svchttp "github.com/vihat/vigov/service-documents/internal/http"
)

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	idCanBo  = "nd-01JINTERNALIDCUACANBO"
	phieuGia = "phieu-phien-GIA-KHONG-PHAI-PHIEU-THAT"

	tuyen = "/api/v1/document-types"
)

var (
	xaA = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

// --- fakes ----------------------------------------------------------------------------------

type thuMucGia map[string]tenant.Tenant

func (m thuMucGia) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// khoGia is the document-type catalogue, KEYED BY COMMUNE and COUNTING ITS READS.
//
// Both halves are assertions. Keyed by commune, because a store keyed by nothing would let the
// wrong-commune test pass while proving nothing. Counting, because "the store was never touched" is
// the only way to show a refused request stopped at the edge rather than at the handler.
type khoGia struct {
	theo map[tenant.ID][]domain.LoaiVanBan
	doc  int
}

func (k *khoGia) DanhSach(ctx context.Context) ([]domain.LoaiVanBan, error) {
	k.doc++
	return k.theo[tenant.MustFrom(ctx)], nil
}

// phanGiaiGia is identity, absent, counting its calls.
type phanGiaiGia struct {
	goi int
	tra staffauth.StaffPrincipal
	co  bool
	loi error
}

func (p *phanGiaiGia) ResolveStaff(_ context.Context, _, _ string) (staffauth.StaffPrincipal, bool, error) {
	p.goi++
	return p.tra, p.co, p.loi
}

// --- the binary's own chain -------------------------------------------------------------------

type mayChu struct {
	h   http.Handler
	kho *khoGia
	pg  *phanGiaiGia
}

func dungMayChu(t *testing.T, pg *phanGiaiGia) *mayChu {
	t.Helper()

	kho := &khoGia{theo: map[tenant.ID][]domain.LoaiVanBan{
		xaA: {{ID: "lvb-001", Ma: "quyet-dinh", Nhan: "Quyết định", DangDung: true, LaMacDinh: true}},
		xaB: {{ID: "lvb-b-001", Ma: "to-trinh", Nhan: "Tờ trình xã B", DangDung: true}},
	}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// THE REAL ROUTE, REGISTERED THE WAY run() REGISTERS IT. A test route would prove the chain
	// passes requests through and nothing about the declaration this service actually ships.
	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:    staffauth.Checker{},
		LoaiVanBan: kho,
		Log:        log,
	})

	return &mayChu{
		h:   dungBien(mux, thuMucGia{hostA: {ID: xaA, Host: hostA, Active: true}, hostB: {ID: xaB, Host: hostB, Active: true}}, pg, log),
		kho: kho,
		pg:  pg,
	}
}

func (m *mayChu) goi(t *testing.T, host, path, phieu string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if phieu != "" {
		r.AddCookie(&http.Cookie{Name: staffauth.CookieName, Value: phieu})
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func doiMa(t *testing.T, w *httptest.ResponseRecorder, muon int) {
	t.Helper()
	if w.Code != muon {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, muon, w.Body.String())
	}
}

func canBoXaA() *phanGiaiGia {
	return &phanGiaiGia{
		co: true,
		tra: staffauth.StaffPrincipal{
			StaffID:        idCanBo,
			PermissionKeys: []authz.Perm{"document.read"},
		},
	}
}

// --- the first request this service has ever served ----------------------------------------------

func TestCanBoXaAO_HostXaA_DuocPhucVu(t *testing.T) {
	// FIRST TIME A ROUTE IN THIS SERVICE SERVES A REQUEST. Until this chain existed, the route
	// answered 401 to everybody — including a member of staff holding a perfectly valid session —
	// because nothing in the binary built an authz.Principal.
	m := dungMayChu(t, canBoXaA())

	w := m.goi(t, hostA, tuyen, phieuGia)
	doiMa(t, w, http.StatusOK)

	var than struct {
		Items []struct {
			Code string `json:"code"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &than); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	// The commune's OWN catalogue, read with the commune from the context — never commune B's.
	if len(than.Items) != 1 || than.Items[0].Code != "quyet-dinh" {
		t.Errorf("danh mục trả về = %+v, muốn danh mục của xã A", than.Items)
	}
	if m.kho.doc != 1 {
		t.Errorf("kho được đọc %d lần, muốn 1", m.kho.doc)
	}
	if m.pg.goi != 1 {
		t.Errorf("gọi dịch vụ định danh %d lần cho 1 yêu cầu, muốn 1", m.pg.goi)
	}
}

func TestCungGiayToOHostXaB_KhongCoChuThe_VaKhoKhongBiChamToi(t *testing.T) {
	// The same credential at commune B's host. The RPC makes the comparison — the commune in
	// "x-tenant-id" against the commune inside the credential — and answers OK with NO PRINCIPAL,
	// which is what this fake reproduces. The route's own guard then refuses.
	//
	// THE STORE MUST NOT BE TOUCHED. A refusal that still ran the query would mean the isolation
	// rested on the handler rather than on the edge, and a query is where a commune boundary is
	// actually crossed.
	m := dungMayChu(t, &phanGiaiGia{co: false})

	doiMa(t, m.goi(t, hostB, tuyen, phieuGia), http.StatusUnauthorized)
	if m.kho.doc != 0 {
		t.Errorf("kho bị đọc %d lần cho một yêu cầu bị từ chối", m.kho.doc)
	}
}

func TestKhongCoCookieThiKhongGoiDinhDanh(t *testing.T) {
	// The count is the assertion. A chain that called anyway would answer this request correctly
	// and pay a LAN round trip on every anonymous hit, forever, with nothing reporting it.
	m := dungMayChu(t, canBoXaA())

	doiMa(t, m.goi(t, hostA, tuyen, ""), http.StatusUnauthorized)
	if m.pg.goi != 0 {
		t.Errorf("gọi dịch vụ định danh %d lần khi không có cookie — phải là 0", m.pg.goi)
	}
	if m.kho.doc != 0 {
		t.Errorf("kho bị đọc %d lần cho một yêu cầu không có phiên", m.kho.doc)
	}
}

func TestDinhDanhChetThiTra503ChuKhongPhai401(t *testing.T) {
	// If identity is down and this chain answered "no principal", every member of staff would be
	// told to sign in again — through the service that is down.
	m := dungMayChu(t, &phanGiaiGia{loi: errors.New("rpc error: code = Unavailable desc = refused")})

	w := m.goi(t, hostA, tuyen, phieuGia)
	if w.Code == http.StatusUnauthorized {
		t.Fatal("trả 401 khi dịch vụ định danh chết")
	}
	doiMa(t, w, http.StatusServiceUnavailable)
	if m.kho.doc != 0 {
		t.Errorf("kho bị đọc %d lần trong lúc không xác thực được", m.kho.doc)
	}
}

func TestHostKhongPhanGiaiDuocTra404VaKhongGoiAi(t *testing.T) {
	m := dungMayChu(t, canBoXaA())

	doiMa(t, m.goi(t, "khong-co-xa.example.gov.vn", tuyen, phieuGia), http.StatusNotFound)
	if m.pg.goi != 0 || m.kho.doc != 0 {
		t.Errorf("Host không phân giải được vẫn gọi định danh %d lần và đọc kho %d lần", m.pg.goi, m.kho.doc)
	}
}

func TestHealthzNamNgoaiChuoiXa(t *testing.T) {
	// It answers whether this process is alive, which is true or false regardless of which commune
	// is asking. Behind Host resolution it would fail whenever the platform service does, and an
	// orchestrator would restart a healthy process during somebody else's outage.
	m := dungMayChu(t, canBoXaA())

	doiMa(t, m.goi(t, "khong-co-xa.example.gov.vn", "/healthz", ""), http.StatusOK)
	if m.pg.goi != 0 {
		t.Error("/healthz gọi tới dịch vụ định danh")
	}
}
