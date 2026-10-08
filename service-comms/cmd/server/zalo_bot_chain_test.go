package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
)

// What only this file can see: that the SHARED webhook chain is reached on ZALO_BOT_WEBHOOK_HOST and ONLY
// there (ADR 0074 #5); that the same path on a commune's host goes to the COMMUNE bot's chain (ADR 0079
// Q1 #2), and every other path to the staff chain; and that an empty host serves no shared webhook.

const testWebhookHost = "bot.api.example.vn"

func markers() (rest, zalo, commune http.Handler, hit *string) {
	var who string
	rest = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { who = "rest"; w.WriteHeader(http.StatusNotFound) })
	zalo = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { who = "zalo"; w.WriteHeader(http.StatusOK) })
	commune = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { who = "commune"; w.WriteHeader(http.StatusOK) })
	return rest, zalo, commune, &who
}

func serveHost(h http.Handler, host, path string) {
	r := httptest.NewRequest(http.MethodPost, "https://"+host+path, nil)
	r.Host = host
	h.ServeHTTP(httptest.NewRecorder(), r)
}

func TestZaloWebhookServedOnlyOnItsHost(t *testing.T) {
	rest, zalo, commune, who := markers()
	h := withZaloBotWebhook(rest, zalo, commune, testWebhookHost)

	cases := []struct{ host, path, want string }{
		{testWebhookHost, svchttp.ZaloBotUpdatesPath, "zalo"},
		{testWebhookHost + ":443", svchttp.ZaloBotUpdatesPath, "zalo"},
		{"xa-a.example.gov.vn", svchttp.ZaloBotUpdatesPath, "commune"},
		{"xa-a.example.gov.vn", "/api/v1/zalo-channel-settings", "rest"},
		{testWebhookHost, "/api/v1/notifications", "rest"},
		{testWebhookHost, svchttp.ZaloBotUpdatesPath + "/x", "rest"},
	}
	for _, c := range cases {
		*who = ""
		serveHost(h, c.host, c.path)
		if *who != c.want {
			t.Errorf("%s%s → %q, want %q", c.host, c.path, *who, c.want)
		}
	}
}

func TestZaloWebhookAbsentWithoutHost(t *testing.T) {
	rest, zalo, commune, who := markers()
	h := withZaloBotWebhook(rest, zalo, commune, "")
	serveHost(h, testWebhookHost, svchttp.ZaloBotUpdatesPath)
	if *who == "zalo" {
		t.Fatalf("with no ZALO_BOT_WEBHOOK_HOST the shared webhook was served (%q)", *who)
	}
}

// The commune chain resolves the commune from Host and answers 404 for a host no commune holds — the
// webhook host included — before the handler runs (rule 1, invariant 3).
func TestCommuneZaloBotChainResolvesTheCommuneFromHost(t *testing.T) {
	var saw tenant.ID
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw, _ = tenant.From(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	h := communeZaloBotChain(inner, oneCommune{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "https://xa-a.example.gov.vn"+svchttp.ZaloBotUpdatesPath, nil)
	r.Header.Set("X-Tenant-Id", "01JBBBBBBBBBBBBBBBBBBBBBBB")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || saw != testCommuneA {
		t.Fatalf("commune host → %d in %q, want 200 in %q (never the header's)", w.Code, saw, testCommuneA)
	}
	saw = ""
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "https://"+testWebhookHost+svchttp.ZaloBotUpdatesPath, nil))
	if w.Code != http.StatusNotFound || saw != "" {
		t.Fatalf("a host no commune holds → %d, reached handler in %q", w.Code, saw)
	}
}

const testCommuneA = tenant.ID("01JAAAAAAAAAAAAAAAAAAAAAAA")

type oneCommune struct{}

func (oneCommune) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	if host == "xa-a.example.gov.vn" {
		return tenant.Tenant{ID: testCommuneA, Host: host, Active: true}, true
	}
	return tenant.Tenant{}, false
}

func TestZaloBotChainStripsTenantHeaders(t *testing.T) {
	var saw string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { saw = r.Header.Get("X-Tenant-Id") })
	r := httptest.NewRequest(http.MethodPost, "https://"+testWebhookHost+svchttp.ZaloBotUpdatesPath, nil)
	r.Header.Set("X-Tenant-Id", "01JAAAAAAAAAAAAAAAAAAAAAAA")
	zaloBotChain(inner).ServeHTTP(httptest.NewRecorder(), r)
	if saw != "" {
		t.Fatal("a client-named commune reached the webhook")
	}
}
