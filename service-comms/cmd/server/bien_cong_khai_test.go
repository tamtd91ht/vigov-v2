package main

// THE PUBLIC EDGE OF THIS BINARY — the wiring, not the middleware (core/httpx proves that).
//
// What only this file can see: that dungBien routes /api/v1/commune-news and its subtree to the PUBLIC
// chain on the reserved API host — with no session and no Host resolution — and leaves every staff
// route on the staff chain; and that dungBienCongKhai really installs CORS. A pattern dropped or
// widened here leaves a service that starts and serves, and either 404s every resident or puts a staff
// route outside TenantMiddleware.

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	commsapp "github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
)

const (
	hostApiComms    = "comms.api.vigov.vn" // reserved, maps to no commune (ADR 0046)
	nguonMiniAppThu = "https://h5.zdn.vn"
)

type xaTheoHostThu struct{}

func (xaTheoHostThu) XaTheoHost(context.Context, string) (tenant.Tenant, bool, error) {
	return tenant.Tenant{ID: xaA, Name: "Xã A", Active: true}, true, nil
}

type noiDungCongKhaiThu struct{}

func (noiDungCongKhaiThu) DanhSachCongKhai(ctx context.Context, _ domain.LoaiNoiDung, _ string, _ page.Request) (page.Result[domain.NoiDungMiniApp], error) {
	tenant.MustFrom(ctx)
	return page.NewResult[domain.NoiDungMiniApp](), nil
}

func (noiDungCongKhaiThu) PublishedCategoryIDs(ctx context.Context, _ domain.LoaiNoiDung) ([]string, error) {
	tenant.MustFrom(ctx)
	return []string{"dm-a"}, nil
}

func (noiDungCongKhaiThu) PublicBanners(ctx context.Context, _ int) ([]domain.NoiDungMiniApp, error) {
	tenant.MustFrom(ctx)
	return nil, nil
}

func (noiDungCongKhaiThu) CongKhaiTheoID(ctx context.Context, id string) (domain.NoiDungMiniApp, error) {
	tenant.MustFrom(ctx)
	return domain.NoiDungMiniApp{ID: id, TieuDe: "Tin", TrangThai: domain.TrangThaiDangHien}, nil
}

type danhMucThu struct{}

func (danhMucThu) DanhSach(context.Context) ([]domain.DanhMucMiniApp, error) {
	return []domain.DanhMucMiniApp{{ID: "dm-a", Ten: "Y tế"}}, nil
}

// dungCongKhaiThu is the real public chain around the real public routes, with fake reads.
func dungCongKhaiThu(t *testing.T) http.Handler {
	t.Helper()
	nguon, err := config.PhanTichNguonCORS("https://h5.zdn.vn,https://*.zalo.me")
	if err != nil {
		t.Fatalf("PhanTichNguonCORS: %v", err)
	}
	mux := http.NewServeMux()
	// The dev wiring with no Redis: the counter that always fails, under the fail-OPEN public policy.
	lim, err := ratelimit.New(unavailableCounter{}, ratelimit.PublicNewsRead)
	if err != nil {
		t.Fatal(err)
	}
	svchttp.RegisterCongKhai(mux, svchttp.DepsCongKhai{
		Limiter: lim,
		Xa:      xaTheoHostThu{}, NoiDung: noiDungCongKhaiThu{}, DanhMuc: danhMucThu{},
		// The real use case with nothing configured: every image is absent, never an error.
		CoverImages: commsapp.NewContentCovers(nil, nil, nil, nil, nil, nil),
		Audio:       commsapp.NewContentAudio(nil, nil, nil, nil, nil, nil),
		Log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return dungBienCongKhai(mux, nguon)
}

// canBoDanhDauComms stands in for the staff chain: it only records that it was reached.
type canBoDanhDauComms struct{ goi int }

func (c *canBoDanhDauComms) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	c.goi++
	w.WriteHeader(http.StatusTeapot)
}

// ngoaiThu builds the outer split with the staff mux replaced by a marker. dungBien wraps the marker in
// the real staff chain, so a staff path reaching it on the reserved host is answered 404 by
// TenantMiddleware — which is itself the proof that it went down the staff chain.
func ngoaiThu(t *testing.T) (http.Handler, *canBoDanhDauComms) {
	t.Helper()
	cb := &canBoDanhDauComms{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return dungBien(cb, dungCongKhaiThu(t), thuMucGia{}, canBoXaA(), nil, log), cb
}

func goiApi(h http.Handler, method, path, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://"+hostApiComms+path, nil)
	r.Host = hostApiComms
	if origin != "" {
		r.Header.Set("Origin", origin)
		if method == http.MethodOptions {
			r.Header.Set("Access-Control-Request-Method", "GET")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestTinXaDiChuoiCongKhaiKhongCanPhien(t *testing.T) {
	for _, p := range []string{svchttp.MauTinXa, svchttp.MauTinXa + "/01JTIN0000000000000000000A"} {
		h, cb := ngoaiThu(t)
		if w := goiApi(h, http.MethodGet, p+"?host=xa-a.vigov.vn", ""); w.Code != http.StatusOK {
			t.Fatalf("%s trên host API dành riêng, không phiên: mã = %d, muốn 200 — thân: %s", p, w.Code, w.Body.String())
		}
		if cb.goi != 0 {
			t.Fatalf("%s: chuỗi cán bộ bị gọi", p)
		}
	}
}

// The chip route rides MauTinXa+"/" onto the public chain — no second outer-mux entry — and reaches
// the categories handler, not the `{id}` detail (whose body would carry `title`, not `items`).
func TestPublicNewsCategoriesOnPublicChain(t *testing.T) {
	h, cb := ngoaiThu(t)
	w := goiApi(h, http.MethodGet, svchttp.MauTinXa+"/categories?host=xa-a.vigov.vn", nguonMiniAppThu)
	if w.Code != http.StatusOK {
		t.Fatalf("categories trên host API dành riêng: mã = %d, muốn 200 — thân: %s", w.Code, w.Body.String())
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"items":[{"id":"dm-a","name":"Y tế","order":0}]}` {
		t.Fatalf("thân = %s", got)
	}
	if cb.goi != 0 {
		t.Fatal("categories: chuỗi cán bộ bị gọi")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != nguonMiniAppThu {
		t.Fatalf("categories thiếu ACAO: %v", w.Header())
	}
}

func TestTuyenCanBoKhongLotSangChuoiCongKhai(t *testing.T) {
	// On the reserved host the staff chain answers 404 at TenantMiddleware — the marker behind it is
	// never reached. A staff path answered 200 here would mean it went down the public chain.
	for _, p := range []string{"/api/v1/content-items", "/api/v1/content-items/x", "/api/v1/content-categories",
		"/api/v1/commune-newsx", "/api/v1/map-asset-types"} {
		h, _ := ngoaiThu(t)
		if w := goiApi(h, http.MethodGet, p+"?host=xa-a.vigov.vn", ""); w.Code != http.StatusNotFound {
			t.Errorf("%s: mã = %d, muốn 404 từ TenantMiddleware của chuỗi cán bộ", p, w.Code)
		}
	}
}

func TestPreflightTinXa204VaCORSChiTrenChuoiCongKhai(t *testing.T) {
	h, _ := ngoaiThu(t)
	w := goiApi(h, http.MethodOptions, svchttp.MauTinXa, "https://mini.zalo.me")
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "https://mini.zalo.me" {
		t.Fatalf("preflight: mã = %d, ACAO = %q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
	w = goiApi(h, http.MethodGet, svchttp.MauTinXa+"?host=xa-a.vigov.vn", nguonMiniAppThu)
	if w.Header().Get("Access-Control-Allow-Origin") != nguonMiniAppThu {
		t.Fatalf("GET công khai thiếu ACAO: %v", w.Header())
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("tuyến công khai cấp Allow-Credentials")
	}
	w = goiApi(h, http.MethodGet, "/api/v1/content-items", nguonMiniAppThu)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("tuyến cán bộ mang header CORS: %v", w.Header())
	}
}
