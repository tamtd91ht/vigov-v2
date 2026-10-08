package idem

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// THE ACCOUNTLESS SCOPE — RequiredAccountless, ADR 0083 row 10 (TEMPORARY; removed with the route).
//
//	PROVED HERE   a request with NO principal is served (Required still refuses it — an_danh_test.go)
//	              · the key space is (commune, key): the same key in another commune is a fresh request
//	              · a different key in the same commune is a fresh request, never a replay · a retry with
//	              the same key replays the code the first attempt recorded, through the route's replayer
//	              when it has one, the standard body when it declines · a key too short to carry 128 bits
//	              is refused before the Store is touched · Required is untouched by the new scope.

// randomKey is a 128-bit key as the app mints it: 32 lowercase hex digits.
const randomKey = "9f86d081884c7d659a2feaa0c55ad015"

// sendAccountless sends one request with NO principal — the shape of the public chain.
func sendAccountless(t *testing.T, h http.Handler, s Store, tid, key string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, duongDan, strings.NewReader("{}"))
	if key != "" {
		r.Header.Set(Header, key)
	}
	ctx := Into(tenant.Into(r.Context(), tenant.ID(tid)), s, logIm())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r.WithContext(ctx))
	return w
}

func TestAccountlessServesWithoutPrincipalAndReplaysTheSameCode(t *testing.T) {
	s := newStoreGia()
	var runs int
	h := RequiredAccountless(DongKhiHong, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runs++
		RecordCode(r.Context(), "PA-ACCOUNTLESS-1")
		w.WriteHeader(http.StatusCreated)
	}))

	if w := sendAccountless(t, h, s, xaA, randomKey); w.Code != http.StatusCreated {
		t.Fatalf("first: %d, want 201", w.Code)
	}
	w := sendAccountless(t, h, s, xaA, randomKey)
	if runs != 1 {
		t.Fatalf("handler ran %d times for one key, want 1", runs)
	}
	if w.Code != http.StatusCreated || w.Header().Get(HeaderPhatLai) != "true" {
		t.Fatalf("replay: %d replay=%q", w.Code, w.Header().Get(HeaderPhatLai))
	}
	var body PhatLai
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Code != "PA-ACCOUNTLESS-1" {
		t.Errorf("replay body = %s, want the first attempt's code", w.Body.String())
	}
}

// TestAccountlessKeySpaceIsCommuneAndKey — the same key in ANOTHER commune, and another key in the
// SAME commune, are both fresh requests: neither is handed anybody's code.
func TestAccountlessKeySpaceIsCommuneAndKey(t *testing.T) {
	s := newStoreGia()
	codes := []string{"PA-A-1", "PA-B-1", "PA-A-2"}
	var runs int
	h := RequiredAccountless(DongKhiHong, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RecordCode(r.Context(), codes[runs])
		runs++
		w.WriteHeader(http.StatusCreated)
	}))

	sendAccountless(t, h, s, xaA, randomKey)
	wB := sendAccountless(t, h, s, xaB, randomKey)
	wA2 := sendAccountless(t, h, s, xaA, "0123456789abcdef0123456789abcdef")

	if runs != 3 {
		t.Fatalf("handler ran %d times, want 3 — a replay crossed a commune or a key", runs)
	}
	for name, w := range map[string]*httptest.ResponseRecorder{"other commune": wB, "other key": wA2} {
		if w.Header().Get(HeaderPhatLai) != "" || strings.Contains(w.Body.String(), "PA-A-1") {
			t.Errorf("%s was replayed another sender's code: %s", name, w.Body.String())
		}
	}
	// The stored key is commune-prefixed (rule 1, invariant 7).
	if s.doc(Key(tenant.ID(xaB), accountlessActor, http.MethodPost, duongDan, randomKey)) == "" {
		t.Error("the commune B key is not under t:<B>:")
	}
}

func TestAccountlessReplayUsesTheRoutesReplayer(t *testing.T) {
	s := newStoreGia()
	var gotStatus int
	var gotCode string
	replay := func(w http.ResponseWriter, _ *http.Request, status int, code string) bool {
		gotStatus, gotCode = status, code
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"code":"` + code + `","status":"da-tiep-nhan"}`))
		return true
	}
	h := RequiredAccountless(DongKhiHong, replay)(handlerTao("PA-ACCOUNTLESS-2"))

	sendAccountless(t, h, s, xaA, randomKey)
	w := sendAccountless(t, h, s, xaA, randomKey)
	if gotStatus != http.StatusCreated || gotCode != "PA-ACCOUNTLESS-2" {
		t.Fatalf("replayer got (%d, %q)", gotStatus, gotCode)
	}
	if !strings.Contains(w.Body.String(), `"status":"da-tiep-nhan"`) || w.Header().Get(HeaderPhatLai) != "true" {
		t.Errorf("replay = %s (replay header %q)", w.Body.String(), w.Header().Get(HeaderPhatLai))
	}

	// A replayer that declines leaves the standard body.
	s2 := newStoreGia()
	h2 := RequiredAccountless(DongKhiHong, func(http.ResponseWriter, *http.Request, int, string) bool { return false })(
		handlerTao("PA-ACCOUNTLESS-3"))
	sendAccountless(t, h2, s2, xaA, randomKey)
	w2 := sendAccountless(t, h2, s2, xaA, randomKey)
	if !strings.Contains(w2.Body.String(), `"replayed":true`) || !strings.Contains(w2.Body.String(), "PA-ACCOUNTLESS-3") {
		t.Errorf("declined replay = %s, want the standard PhatLai body", w2.Body.String())
	}
}

func TestAccountlessRefusesKeysThatCannotCarry128Bits(t *testing.T) {
	for name, key := range map[string]string{
		"missing":          "",
		"31 hex digits":    "0123456789abcdef0123456789abcde",
		"short base64url":  "AbCdEfGhIjKlMnOpQrStU",
		"forbidden symbol": "9f86d081884c7d659a2feaa0c55ad015!",
	} {
		t.Run(name, func(t *testing.T) {
			s := newStoreGia()
			w := sendAccountless(t, RequiredAccountless(DongKhiHong, nil)(handlerTao("PA-X")), s, xaA, key)
			if w.Code != http.StatusBadRequest {
				t.Errorf("code = %d, want 400", w.Code)
			}
			if s.soKhoa() != 0 {
				t.Errorf("a refused key touched the Store")
			}
		})
	}
	for name, key := range map[string]string{
		"32 hex":       randomKey,
		"22 base64url": "AbCdEfGhIjKlMnOpQrStUv",
		"uuid":         "3f2504e0-4f89-41d3-9a0c-0305e82c3301",
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateAccountlessKey(key); err != nil {
				t.Errorf("refused a 128-bit key: %v", err)
			}
		})
	}
}

// TestRequiredStillRefusesNoPrincipal — the new scope did not open the old door.
func TestRequiredStillRefusesNoPrincipal(t *testing.T) {
	w := sendAccountless(t, Required(DongKhiHong)(handlerTao("PA-X")), newStoreGia(), xaA, randomKey)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Required without a principal = %d, want 500", w.Code)
	}
}
