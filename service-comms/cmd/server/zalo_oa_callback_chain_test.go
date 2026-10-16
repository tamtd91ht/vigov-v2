package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	svchttp "github.com/vihat/vigov/service-comms/internal/http"
)

// What only this file can see (ADR 0098 #3, #4): the OA callback is reached on ZALO_OA_CALLBACK_HOST and
// ONLY there; a commune's host never serves it; the callback host serves nothing else; it may share its
// host with ZALO_BOT_WEBHOOK_HOST; an empty host serves no callback; and its chain strips a client-named
// commune.

const testOACallbackHost = "oa.api.example.vn"

func oaMarkers() (rest, callback http.Handler, hit *string) {
	var who string
	rest = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { who = "rest"; w.WriteHeader(http.StatusNotFound) })
	callback = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { who = "callback"; w.WriteHeader(http.StatusFound) })
	return rest, callback, &who
}

func getHost(h http.Handler, host, path string) {
	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	h.ServeHTTP(httptest.NewRecorder(), r)
}

func TestZaloOACallbackServedOnlyOnItsHost(t *testing.T) {
	rest, callback, who := oaMarkers()
	h := withZaloOACallback(rest, callback, testOACallbackHost)
	cases := []struct{ host, path, want string }{
		{testOACallbackHost, svchttp.ZaloOAAuthorizationsPath, "callback"},
		{testOACallbackHost + ":443", svchttp.ZaloOAAuthorizationsPath, "callback"},
		// A commune's host: the path goes to the staff side, which has no such route.
		{"xa-a.example.gov.vn", svchttp.ZaloOAAuthorizationsPath, "rest"},
		// The callback host serves no staff route: rest, where TenantMiddleware 404s a platform host.
		{testOACallbackHost, "/api/v1/zalo-oas/current", "rest"},
		{testOACallbackHost, svchttp.ZaloOAAuthorizationsPath + "/x", "rest"},
	}
	for _, c := range cases {
		*who = ""
		getHost(h, c.host, c.path)
		if *who != c.want {
			t.Errorf("%s%s → %q, want %q", c.host, c.path, *who, c.want)
		}
	}
}

func TestZaloOACallbackAbsentWithoutHost(t *testing.T) {
	rest, callback, who := oaMarkers()
	h := withZaloOACallback(rest, callback, "")
	getHost(h, testOACallbackHost, svchttp.ZaloOAAuthorizationsPath)
	if *who != "rest" {
		t.Fatalf("with no ZALO_OA_CALLBACK_HOST the callback was served (%q)", *who)
	}
}

// ADR 0098 #3: the callback host may be the bot webhook's host. Both host patterns live on different
// muxes with different paths — each reaches its own chain, and the staff side gets the rest.
func TestZaloOACallbackSharesHostWithBotWebhook(t *testing.T) {
	var who string
	mark := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { who = name })
	}
	h := withZaloOACallback(withZaloBotWebhook(mark("staff"), mark("shared-bot"), mark("commune-bot"), testWebhookHost),
		mark("callback"), testWebhookHost)
	cases := []struct{ method, host, path, want string }{
		{http.MethodGet, testWebhookHost, svchttp.ZaloOAAuthorizationsPath, "callback"},
		{http.MethodPost, testWebhookHost, svchttp.ZaloBotUpdatesPath, "shared-bot"},
		{http.MethodPost, "xa-a.example.gov.vn", svchttp.ZaloBotUpdatesPath, "commune-bot"},
		{http.MethodGet, "xa-a.example.gov.vn", svchttp.ZaloOAAuthorizationsPath, "staff"},
		{http.MethodGet, testWebhookHost, "/api/v1/zalo-oas/current", "staff"},
	}
	for _, c := range cases {
		who = ""
		r := httptest.NewRequest(c.method, "https://"+c.host+c.path, nil)
		r.Host = c.host
		h.ServeHTTP(httptest.NewRecorder(), r)
		if who != c.want {
			t.Errorf("%s %s%s → %q, want %q", c.method, c.host, c.path, who, c.want)
		}
	}
}

func TestZaloOACallbackChainStripsTenantHeaders(t *testing.T) {
	var saw string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { saw = r.Header.Get("X-Tenant-Id") })
	r := httptest.NewRequest(http.MethodGet, "https://"+testOACallbackHost+svchttp.ZaloOAAuthorizationsPath+"?state=x", nil)
	r.Header.Set("X-Tenant-Id", string(testCommuneA))
	zaloOACallbackChain(inner).ServeHTTP(httptest.NewRecorder(), r)
	if saw != "" {
		t.Fatal("a client-named commune reached the OA callback")
	}
}
