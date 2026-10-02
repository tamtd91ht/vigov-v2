package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
)

// THE 403 EVERY CITIZEN ROUTE DECLARES, proved on every citizen route.
//
// routes_cong_dan.go declares `@reply 403` on all four routes with exactly one cause: a usable session
// in a commune that carries NO verified phone (ADR 0045), refused by httpx.XaTuPhien with the key
// `chua_xac_thuc_so`. core/httpx proves the layer in isolation; this file proves each route is actually
// wrapped in it. A route re-registered with XaTuPhienChiXem by mistake would hand a session with an
// empty CitizenID to a handler that filters by that identity — the "empty citizen" ADR 0045 stop
// condition #6 names — and only a per-route assertion turns red for it.
//
// The fakes are counted as well as the status: a refusal that came after the store or use case ran
// would have acted for a request with no citizen identity at all.

// sessionWithoutPhone is a usable session, already in commune A, with no citizen identity yet.
type sessionWithoutPhone struct{}

// vi-name-ok: implements the existing httpx.CitizenSessions interface method.
func (sessionWithoutPhone) TraCuu(context.Context, string) (httpx.CitizenSession, bool, error) {
	return httpx.CitizenSession{ID: "sid-no-phone", TenantID: xaA}, true, nil
}

func TestCitizenRoutesRefuseSessionWithoutVerifiedPhone(t *testing.T) {
	routes := []struct {
		name, method, path, body string
	}{
		{"read own petition", http.MethodGet, duongCuaToi(maCuaToi), ""},
		{"list own petitions", http.MethodGet, "/api/v1/my-citizen-reports", ""},
		{"file a petition", http.MethodPost, "/api/v1/my-citizen-reports",
			`{"content":"Đống rác ở đầu ngõ đã ba ngày chưa ai dọn."}`},
		{"rate own petition", http.MethodPost, duongCuaToi(maCuaToi) + "/rating", `{"stars":5}`},
	}

	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			petitions := phieuCuaToiMau()
			intake := soPhieuMoi()
			rating := newRatingFake()

			mux := http.NewServeMux()
			RegisterCongDan(mux, DepsCongDan{
				Phieu:       petitions,
				GuiPhieu:    intake,
				Rating:      rating,
				NhanLinhVuc: nhanLinhVucMau(), CitizenFields: newFieldCatalogueFake(), Photos: newCitizenPhotosFake(), VerificationPhotos: newCitizenVerificationPhotosFake(), PhotoLimiter: photoLimiterThu(),
				Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
			})
			// The chain in the order cmd/server builds it (same as dungMayChuCongDan).
			var h http.Handler = mux
			h = authz.CitizenPrincipal()(h)
			h = httpx.CitizenEdge(sessionWithoutPhone{})(h)
			h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
			h = httpx.StripTenantHeaders(h)

			r := httptest.NewRequest(rt.method, "https://"+hostMiniApp+rt.path, strings.NewReader(rt.body))
			r.Host = hostMiniApp
			r.RemoteAddr = "10.0.0.9:51000"
			r.Header.Set("Authorization", "Bearer token-session-no-phone-FAKE")
			if rt.body != "" {
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("Idempotency-Key", "01JKEYNOPHONE00000000000000")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			doiMa(t, w, http.StatusForbidden)
			if !strings.Contains(w.Body.String(), `"chua_xac_thuc_so"`) {
				t.Errorf("body = %s — want key chua_xac_thuc_so so the Mini App knows to ask for the phone",
					w.Body.String())
			}
			if petitions.goi != 0 || intake.demGui != 0 || rating.calls != 0 {
				t.Errorf("a later layer ran (read %d · intake %d · rating %d) for a session with no phone",
					petitions.goi, intake.demGui, rating.calls)
			}
		})
	}
}
