package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	svchttp "github.com/vihat/vigov/service-petitions/internal/http"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// THE THIRD CHAIN — ADR 0083 (TEMPORARY). The accountless routes must reach their handlers through
// dungBien on the RESERVED API host (which no commune holds), with no session and no staff cookie, and
// must never ask the staff resolver. Delete any of the three `ngoai.Handle(svchttp.Accountless…, a)`
// lines and the matching case goes red (404 from the staff chain).

const accountlessCode = "PA-ACCT-0001-TEST"

// hostDirectoryStub resolves hostA to commune A, and nothing else.
type hostDirectoryStub struct{}

// vi-name-ok: implements tenant.HostResolver, whose method name is platformclient's
func (hostDirectoryStub) XaTheoHost(_ context.Context, host string) (tenant.Tenant, bool, error) {
	if host == hostA {
		return tenant.Tenant{ID: xaA, Host: hostA, Active: true}, true, nil
	}
	return tenant.Tenant{}, false, nil
}

// accountlessIntakeStub asserts what only the public chain + withBodyCommune can have set up: commune A
// in the context, and the accountless sender.
type accountlessIntakeStub struct{}

func (accountlessIntakeStub) Gui(ctx context.Context, _ app.YeuCauGuiPhanAnh, s app.IntakeSender) (domain.PhieuPhanAnh, error) {
	if tenant.MustFrom(ctx) != xaA || !s.Owner.IsAccountless() {
		return domain.PhieuPhanAnh{}, errors.New("intake stub: wrong commune or sender")
	}
	return domain.PhieuPhanAnh{MaTraCuu: accountlessCode, Kenh: domain.KenhZaloMiniApp,
		TrangThai: domain.DaTiepNhan, HanTiepNhan: mocGui}, nil
}

type accountlessReaderStub struct{}

func (accountlessReaderStub) AccountlessByCode(ctx context.Context, ma string) (domain.PhieuPhanAnh, error) {
	if tenant.MustFrom(ctx) != xaA || ma != accountlessCode {
		return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
	}
	return domain.PhieuPhanAnh{MaTraCuu: accountlessCode, Kenh: domain.KenhZaloMiniApp,
		TrangThai: domain.DaTiepNhan, HanTiepNhan: mocGui}, nil
}

// allowCounter always counts 1 — the limiters let everything through here; their thresholds are
// core/ratelimit's and internal/http's tests.
type allowCounter struct{}

func (allowCounter) Incr(context.Context, string, time.Duration) (int64, time.Duration, error) {
	return 1, time.Minute, nil
}

func accountlessMuxForTest(t *testing.T, log *slog.Logger) http.Handler {
	t.Helper()
	l, err := newAccountlessLimiters(allowCounter{})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	svchttp.RegisterAccountless(mux, svchttp.DepsAccountless{
		Communes: hostDirectoryStub{}, Intake: accountlessIntakeStub{}, Petitions: accountlessReaderStub{},
		Fields: fieldCatalogueStub{}, SendLimiter: l.send, LookupLimiter: l.lookup, FieldsLimiter: l.fields,
		Log: log,
	})
	return mux
}

func TestAccountlessRoutesRideThePublicChain(t *testing.T) {
	m := dungMayChu(t, canBoXaA())
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://"+hostMiniApp+path, strings.NewReader(body))
		r.Host = hostMiniApp
		r.RemoteAddr = "10.0.0.9:51000"
		if method == http.MethodPost {
			r.Header.Set("Idempotency-Key", "9f86d081884c7d659a2feaa0c55ad015")
		}
		w := httptest.NewRecorder()
		m.h.ServeHTTP(w, r)
		return w
	}

	if w := do(http.MethodGet, svchttp.AccountlessFieldsPath+"?host="+hostA, ""); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"code":"rac-thai"`) {
		t.Errorf("fields: %d %s", w.Code, w.Body.String())
	}
	// dungMayChu wires NO idem Store (dev without Redis), so the route's DongKhiHong answers 503
	// `idempotency_unavailable` — which only the accountless route on this chain can produce: the staff
	// chain would answer 404 for the reserved host. The 201 path is internal/http's accountless_test.go.
	if w := do(http.MethodPost, svchttp.AccountlessReportsPath,
		`{"host":"`+hostA+`","content":"Đống rác ở đầu ngõ."}`); w.Code != http.StatusServiceUnavailable ||
		!strings.Contains(w.Body.String(), "idempotency_unavailable") {
		t.Errorf("send: %d %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodGet, svchttp.AccountlessReportsPath+"/"+accountlessCode+"?host="+hostA, ""); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"status":"da-tiep-nhan"`) {
		t.Errorf("lookup: %d %s", w.Code, w.Body.String())
	}
	if m.pg.goi != 0 || m.so.goi != 0 {
		t.Errorf("the public chain asked the staff resolver %d times / the session registry %d times", m.pg.goi, m.so.goi)
	}
}

// The public chain ignores Host: an unknown commune domain is the accountless handler's 404, not the
// staff chain's.
func TestAccountlessUnknownHostIs404FromTheAccountlessHandler(t *testing.T) {
	m := dungMayChu(t, canBoXaA())
	r := httptest.NewRequest(http.MethodGet, "https://"+hostMiniApp+svchttp.AccountlessFieldsPath+"?host=unknown.example.vn", nil)
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "commune_not_found") {
		t.Errorf("unknown host = %d %s, want 404 commune_not_found", w.Code, w.Body.String())
	}
}

// The limiters fail CLOSED (no REDIS_DSN in dev): never served unbounded.
func TestAccountlessLimitersFailClosedWithoutRedis(t *testing.T) {
	l, err := newAccountlessLimiters(unavailableCounter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, lim := range []*ratelimit.Limiter{l.send, l.lookup, l.fields} {
		if lim.Policy().FailsOpen() {
			t.Error("an accountless limiter fails open")
		}
	}
}
