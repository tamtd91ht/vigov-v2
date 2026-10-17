package search

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/secret"
)

const (
	fakeUser = "vigov-test-citizen-contacts"
	fakePass = "mat-khau-GIA-KHONG-PHAI-THAT"
)

var (
	user = secret.Secret(fakeUser)
	pass = secret.Secret(fakePass)
)

func TestNewRefusesInsecureOrIncomplete(t *testing.T) {
	if _, err := New(nil, user, pass, false); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no address: %v", err)
	}
	if _, err := New([]string{"http://es:9200"}, user, pass, false); !errors.Is(err, ErrInsecureAddress) {
		t.Errorf("http outside dev: %v", err)
	}
	if _, err := New([]string{"https://user:pw@es:9200"}, user, pass, false); !errors.Is(err, ErrInsecureAddress) ||
		strings.Contains(err.Error(), "pw") {
		t.Errorf("credentials in the URL: %v", err)
	}
	if _, err := New([]string{"https://es:9200"}, nil, nil, false); !errors.Is(err, ErrNoCredentials) {
		t.Errorf("no credentials outside dev: %v", err)
	}
	if _, err := New([]string{"http://es:9200"}, nil, nil, true); err != nil {
		t.Errorf("dev may use plain http without credentials: %v", err)
	}
	// Exactly one of the pair is refused in EVERY environment, dev included: never a fall back to no auth.
	for _, allow := range []bool{false, true} {
		for _, half := range [][2]secret.Secret{{user, nil}, {nil, pass}} {
			_, err := New([]string{"https://es:9200"}, half[0], half[1], allow)
			if !errors.Is(err, ErrHalfCredentials) {
				t.Errorf("half credentials (dev=%v): %v", allow, err)
				continue
			}
			if strings.Contains(err.Error(), fakeUser) || strings.Contains(err.Error(), fakePass) {
				t.Errorf("error carries a credential: %v", err)
			}
		}
	}
}

// The certificate is verified: a self-signed node is refused, never trusted silently.
func TestTLSIsVerified(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c, err := New([]string{srv.URL}, user, pass, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Count(context.Background(), "i", "", map[string]any{}); err == nil {
		t.Fatal("an untrusted certificate must fail the request")
	}
}

func TestBasicAuthHeaderFailoverAndErrorWithoutReason(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.RequestURI()
		if strings.HasSuffix(r.URL.Path, "/_count") {
			_, _ = io.WriteString(w, `{"count":7}`)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"type":"mapper_parsing_exception","reason":"failed to parse field [full_name] value Nguyễn Văn A"}}`)
	}))
	defer srv.Close()
	// The first node is dead: the client fails over.
	c, err := New([]string{"http://127.0.0.1:1", srv.URL}, user, pass, true)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // both round-robin starting points
		n, err := c.Count(context.Background(), "vigov-dev-citizen-contacts", "T1", map[string]any{"match_all": map[string]any{}})
		if err != nil || n != 7 {
			t.Fatalf("count = %d, %v", n, err)
		}
	}
	// HTTP Basic, never an ApiKey header (owner 17/10/2026).
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(fakeUser+":"+fakePass))
	if gotAuth != wantAuth || strings.HasPrefix(gotAuth, "ApiKey") {
		t.Errorf("Authorization header is not HTTP Basic of the configured pair")
	}
	if !strings.Contains(gotPath, "routing=T1") {
		t.Errorf("routing not sent: %s", gotPath)
	}
	err = c.PutDoc(context.Background(), "i", "d", "T1", map[string]any{"full_name": "Nguyễn Văn A"})
	if !IsStatus(err, http.StatusBadRequest) || strings.Contains(err.Error(), "Nguyễn") || !strings.Contains(err.Error(), "mapper_parsing_exception") {
		t.Errorf("error must name the type, never the reason: %v", err)
	}
}

func TestEnsureIndexAndMeta(t *testing.T) {
	created := false
	var meta map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodHead:
			if created {
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPut && r.URL.Path == "/idx":
			var def map[string]any
			_ = json.NewDecoder(r.Body).Decode(&def)
			meta = def["mappings"].(map[string]any)["_meta"].(map[string]any)
			created = true
		case r.Method == http.MethodGet && r.URL.Path == "/idx/_mapping":
			_ = json.NewEncoder(w).Encode(map[string]any{"idx": map[string]any{"mappings": map[string]any{"_meta": meta}}})
		case r.Method == http.MethodPut && r.URL.Path == "/idx/_mapping":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			meta = body["_meta"].(map[string]any)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()
	c, _ := New([]string{srv.URL}, nil, nil, true)
	ctx := context.Background()
	ok, err := c.EnsureIndex(ctx, "idx", map[string]any{"mappings": map[string]any{"_meta": map[string]any{"fp": "a"}}})
	if err != nil || !ok {
		t.Fatalf("first EnsureIndex = %v, %v", ok, err)
	}
	if ok, err := c.EnsureIndex(ctx, "idx", nil); err != nil || ok {
		t.Fatalf("second EnsureIndex = %v, %v — must not recreate", ok, err)
	}
	m, err := c.Meta(ctx, "idx")
	if err != nil || m["fp"] != "a" {
		t.Fatalf("meta = %v, %v", m, err)
	}
	if err := c.PutMeta(ctx, "idx", map[string]any{"fp": "b"}); err != nil {
		t.Fatal(err)
	}
	if m, _ := c.Meta(ctx, "idx"); m["fp"] != "b" {
		t.Errorf("meta after PutMeta = %v", m)
	}
}

func TestBulkPartialFailureNamesCountOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/x-ndjson" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `{"errors":true,"items":[{"index":{"status":201}},{"index":{"status":400,"error":{"type":"strict_dynamic_mapping_exception","reason":"x Nguyễn"}}}]}`)
	}))
	defer srv.Close()
	c, _ := New([]string{srv.URL}, nil, nil, true)
	err := c.Bulk(context.Background(), "idx", []BulkIndex{{ID: "a", Doc: map[string]any{}}, {ID: "b", Doc: map[string]any{}}})
	if err == nil || !strings.Contains(err.Error(), "1 of 2") || strings.Contains(err.Error(), "Nguyễn") {
		t.Errorf("bulk error = %v", err)
	}
}

func TestDeleteDocAbsentIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"result":"not_found"}`)
	}))
	defer srv.Close()
	c, _ := New([]string{srv.URL}, nil, nil, true)
	if err := c.DeleteDoc(context.Background(), "idx", "a", "T"); err != nil {
		t.Errorf("deleting an absent document: %v", err)
	}
}
