package http

// THE IMAGE-FETCH RATE LIMIT on POST /api/v1/content-items/body-images/from-url (owner, 03/10/2026, ADR
// 0067 K10): 30 attempts per hour per OFFICER, EVERY attempt counted (failures and refusals included),
// 429 `image_fetch_rate_limited` + Retry-After past it, and CLOSED (503) when the counter store is down.
// Through the REAL Register behind the REAL edge chain (dungMayChuND).

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
)

// fetchAttempt is m.goi with a FRESH Idempotency-Key per attempt: the harness's fixed key would have the
// edge replay the first answer, and a replay never reaches the route — it is not an attempt.
func (m *mayChuND) fetchAttempt(t *testing.T, host, body string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	m.attempt++
	r := httptest.NewRequest(http.MethodPost, "https://"+host+pathBodyImageFromURL, strings.NewReader(body))
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, fmt.Sprintf("01JFETCHKEY%015d", m.attempt))
	r = r.WithContext(authz.Into(r.Context(), *p))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

// The 31st attempt in the window is refused BEFORE the use case: 30 fetches, not 31.
func TestImageFetchThirtyPassTheThirtyFirstIs429(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	for i := 0; i < ratelimit.StaffImageFetchLimit; i++ {
		doiMa(t, m.fetchAttempt(t, hostA, bodyBodyImageFromURLOK, canBo(xaA)), http.StatusCreated)
	}
	w := m.fetchAttempt(t, hostA, bodyBodyImageFromURLOK, canBo(xaA))
	doiMa(t, w, http.StatusTooManyRequests)
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 without Retry-After")
	}
	body := w.Body.String()
	if !strings.Contains(body, `"code":"image_fetch_rate_limited"`) ||
		!strings.Contains(body, "Đã lấy ảnh từ liên kết quá nhiều lần trong một giờ") {
		t.Errorf("429 body: %s", body)
	}
	if m.covers.fetches != ratelimit.StaffImageFetchLimit {
		t.Errorf("use case ran %d times, want %d — a throttled attempt reached the fetch", m.covers.fetches,
			ratelimit.StaffImageFetchLimit)
	}
	// The key is the commune + the officer's BUSINESS CODE — nothing of the pasted URL (rule 3).
	want := "t:" + string(xaA) + ":rl:body-image-fetch:actor:" + canBo(xaA).Ma
	for _, k := range m.fetchCounter.keys {
		if k != want {
			t.Fatalf("counter key = %q, want %q", k, want)
		}
	}
	// The refusal is a security event carrying the commune and the actor code — never the URL.
	logs := m.logs.String()
	for _, s := range []string{"event=body_image_fetch.rate_limited", "xa=" + string(xaA), "actor=" + canBo(xaA).Ma} {
		if !strings.Contains(logs, s) {
			t.Errorf("security event lacks %q: %s", s, logs)
		}
	}
}

// FAILURES COUNT (K10): thirty refused or failed attempts — bad link, unreachable host, a malformed body —
// spend the budget exactly like thirty successes.
func TestImageFetchFailuresAndRefusalsCount(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.covers.err = app.ErrImageURLInvalid
	for i := 0; i < 10; i++ {
		doiMa(t, m.fetchAttempt(t, hostA, bodyBodyImageFromURLOK, canBo(xaA)), http.StatusBadRequest)
	}
	m.covers.err = &app.ImageFetchError{Class: "timeout"}
	for i := 0; i < 10; i++ {
		doiMa(t, m.fetchAttempt(t, hostA, bodyBodyImageFromURLOK, canBo(xaA)), http.StatusBadGateway)
	}
	m.covers.err = nil
	for i := 0; i < 10; i++ { // a body that never parses: refused before the use case, still counted
		doiMa(t, m.fetchAttempt(t, hostA, `{"url":`, canBo(xaA)), http.StatusBadRequest)
	}
	before := m.covers.fetches
	doiMa(t, m.fetchAttempt(t, hostA, bodyBodyImageFromURLOK, canBo(xaA)), http.StatusTooManyRequests)
	if m.covers.fetches != before {
		t.Error("the 31st attempt reached the use case after 30 failed ones")
	}
}

// Another officer of the same commune, and the same code in another commune, each have their own budget.
func TestImageFetchBudgetIsPerOfficerAndPerCommune(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.capQuyen(xaB, QuyenSuaNoiDung)
	for i := 0; i < ratelimit.StaffImageFetchLimit; i++ {
		m.fetchAttempt(t, hostA, bodyBodyImageFromURLOK, canBo(xaA))
	}
	doiMa(t, m.fetchAttempt(t, hostA, bodyBodyImageFromURLOK, canBo(xaA)), http.StatusTooManyRequests)

	other := *canBo(xaA)
	other.ID, other.Ma = "01JCANBOKHAC0000000000000", "CB-2026-OTHER1"
	doiMa(t, m.fetchAttempt(t, hostA, bodyBodyImageFromURLOK, &other), http.StatusCreated)

	// Same business code, commune B: `CB-…` is unique per commune only, so the key carries the commune.
	doiMa(t, m.fetchAttempt(t, hostB, bodyBodyImageFromURLOK, canBo(xaB)), http.StatusCreated)
}

// FAIL CLOSED: the counter store down is a 503 and the fetch never runs — unlike the public news read.
func TestImageFetchStoreDownIs503Closed(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.fetchCounter.fail = errors.New("dial tcp: connection refused")
	w := m.fetchAttempt(t, hostA, bodyBodyImageFromURLOK, canBo(xaA))
	doiMa(t, w, http.StatusServiceUnavailable)
	if !strings.Contains(w.Body.String(), `"code":"rate_limit_unavailable"`) {
		t.Errorf("503 body: %s", w.Body.String())
	}
	if m.covers.fetches != 0 {
		t.Fatal("store down, yet the fetch ran — the limiter failed open")
	}
	if ratelimit.StaffImageFetch.FailsOpen() {
		t.Fatal("ratelimit.StaffImageFetch fails open")
	}
}

// A missing limiter is a wiring fault answered 503 on THIS route — never an unbounded fetch.
func TestImageFetchWithoutLimiterIs503(t *testing.T) {
	covers := &fakeCovers{}
	h := NewHandler(Deps{ContentCovers: covers, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+pathBodyImageFromURL,
		strings.NewReader(bodyBodyImageFromURLOK))
	r.Header.Set("Content-Type", "application/json")
	ctx := tenant.Into(r.Context(), xaA)
	r = r.WithContext(authz.Into(ctx, *canBo(xaA)))
	w := httptest.NewRecorder()
	h.FetchBodyImageFromURL(w, r)
	doiMa(t, w, http.StatusServiceUnavailable)
	if covers.fetches != 0 {
		t.Fatal("no limiter, yet the fetch ran")
	}
}
