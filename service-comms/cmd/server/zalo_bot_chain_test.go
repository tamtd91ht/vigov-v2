package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	svchttp "github.com/vihat/vigov/service-comms/internal/http"
)

// What only this file can see: that the webhook chain is reached on ZALO_BOT_WEBHOOK_HOST and ONLY there
// (ADR 0074 #5) — the same path on a commune's host goes to the staff chain, and an empty host serves no
// webhook at all.

const testWebhookHost = "bot.api.example.vn"

func markers() (rest, zalo http.Handler, hit *string) {
	var who string
	rest = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { who = "rest"; w.WriteHeader(http.StatusNotFound) })
	zalo = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { who = "zalo"; w.WriteHeader(http.StatusOK) })
	return rest, zalo, &who
}

func serveHost(h http.Handler, host, path string) {
	r := httptest.NewRequest(http.MethodPost, "https://"+host+path, nil)
	r.Host = host
	h.ServeHTTP(httptest.NewRecorder(), r)
}

func TestZaloWebhookServedOnlyOnItsHost(t *testing.T) {
	rest, zalo, who := markers()
	h := withZaloBotWebhook(rest, zalo, testWebhookHost)

	cases := []struct{ host, path, want string }{
		{testWebhookHost, svchttp.ZaloBotUpdatesPath, "zalo"},
		{testWebhookHost + ":443", svchttp.ZaloBotUpdatesPath, "zalo"},
		{"xa-a.example.gov.vn", svchttp.ZaloBotUpdatesPath, "rest"},
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
	rest, zalo, who := markers()
	h := withZaloBotWebhook(rest, zalo, "")
	serveHost(h, testWebhookHost, svchttp.ZaloBotUpdatesPath)
	if *who != "rest" {
		t.Fatalf("with no ZALO_BOT_WEBHOOK_HOST the webhook was served (%q)", *who)
	}
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
