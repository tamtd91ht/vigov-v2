package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// Fake credentials, labelled as such (rule 8, forbidden #1).
func testConfig() Config {
	return Config{
		Endpoint:           "http://minio.internal.example:9000",
		PublicEndpoint:     "https://files.example.test",
		PublicMediaBaseURL: "https://cdn.example.test/vigov-test-public",
		AccessKey:          secret.Secret("FAKE-ACCESS-KEY-NOT-REAL"),
		SecretKey:          secret.Secret("fake-secret-key-NOT-A-REAL-SECRET"),
		BucketPrefix:       "vigov-test",
	}
}

func testClient(t *testing.T) *Client {
	t.Helper()
	c, err := New(testConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewNotConfigured(t *testing.T) {
	_, err := New(Config{})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
	cfg := testConfig()
	cfg.SecretKey = nil
	_, err = New(cfg)
	if !errors.Is(err, ErrNotConfigured) || !strings.Contains(err.Error(), "OBJECT_STORAGE_SECRET_KEY") {
		t.Fatalf("err = %v, want ErrNotConfigured naming the secret key", err)
	}
}

func TestNewRefusesMultipleHosts(t *testing.T) {
	cfg := testConfig()
	cfg.Endpoint = "http://a.example:9000,http://b.example:9000"
	if _, err := New(cfg); err == nil || !strings.Contains(err.Error(), "ONE endpoint") {
		t.Fatalf("err = %v, want single-endpoint refusal", err)
	}
}

func TestBucketNames(t *testing.T) {
	c := testClient(t)
	for b, want := range map[Bucket]string{
		BucketPrivate: "vigov-test-private", BucketPublic: "vigov-test-public", BucketTemp: "vigov-test-temp",
	} {
		if got, err := c.BucketName(b); err != nil || got != want {
			t.Errorf("BucketName(%d) = %q, %v", b, got, err)
		}
	}
	if _, err := c.BucketName(Bucket(0)); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("zero bucket accepted: %v", err)
	}
}

// decodePolicy returns the policy conditions as their JSON text, one per entry.
func decodePolicy(t *testing.T, b64 string) []string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("policy not base64: %v", err)
	}
	var p struct {
		Conditions []json.RawMessage `json:"conditions"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("policy not JSON: %v\n%s", err, raw)
	}
	out := make([]string, 0, len(p.Conditions))
	for _, c := range p.Conditions {
		var v []any
		if err := json.Unmarshal(c, &v); err != nil {
			t.Fatalf("condition: %v", err)
		}
		out = append(out, fmt.Sprint(v...))
	}
	return out
}

func TestPresignUploadIsSignedForThePublicHostOffline(t *testing.T) {
	c := testClient(t)
	up, err := validKey().UploadPath()
	if err != nil {
		t.Fatal(err)
	}
	// No server exists at either endpoint: this only passes if presigning is offline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	post, err := c.PresignUpload(ctx, up, 2<<30, MIMEMP4, 0)
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}
	u, err := url.Parse(post.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "files.example.test" || u.Scheme != "https" {
		t.Fatalf("signed for %s://%s, want the PUBLIC endpoint", u.Scheme, u.Host)
	}
	if u.Path != "/vigov-test-temp/" {
		t.Errorf("path = %q, want path-style temp bucket", u.Path)
	}
	if post.Fields["key"] != up || post.Fields["Content-Type"] != MIMEMP4 {
		t.Errorf("fields = key %q, type %q", post.Fields["key"], post.Fields["Content-Type"])
	}
	if !strings.Contains(post.Fields["x-amz-credential"], "/us-east-1/s3/") {
		t.Errorf("credential scope = %q", post.Fields["x-amz-credential"])
	}
	conds := strings.Join(decodePolicy(t, post.Fields["policy"]), "\n")
	for _, want := range []string{
		"eq$bucketvigov-test-temp",
		"eq$key" + up,
		"eq$Content-Type" + MIMEMP4,
		"content-length-range1 2.147483648e+09",
	} {
		if !strings.Contains(conds, want) {
			t.Errorf("policy lacks %q:\n%s", want, conds)
		}
	}
	if d := time.Until(post.ExpiresAt); d > UploadTTL || d < UploadTTL-time.Minute {
		t.Errorf("expiry in %s, want ~%s", d, UploadTTL)
	}
}

func TestPresignUploadRefusals(t *testing.T) {
	c := testClient(t)
	up, _ := validKey().UploadPath()
	dst, _ := validKey().Path()
	ctx := context.Background()
	cases := map[string]struct {
		key, mime string
		max       int64
		ttl       time.Duration
		want      error
	}{
		"destination key":     {dst, MIMEMP4, 10, 0, ErrInvalidKey},
		"html type":           {up, "text/html", 10, 0, ErrTypeNotAllowed},
		"svg type":            {up, "image/svg+xml", 10, 0, ErrTypeNotAllowed},
		"type mismatches ext": {up, MIMEJPEG, 10, 0, ErrInvalidArgument},
		"zero max":            {up, MIMEMP4, 0, 0, ErrInvalidArgument},
		"ttl above 15m":       {up, MIMEMP4, 10, 16 * time.Minute, ErrInvalidArgument},
		"negative ttl":        {up, MIMEMP4, 10, -time.Second, ErrInvalidArgument},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := c.PresignUpload(ctx, tc.key, tc.max, tc.mime, tc.ttl); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPresignDownloadOffline(t *testing.T) {
	c := testClient(t)
	k := validKey()
	k.Class, k.Service, k.Purpose, k.Ext = ClassRecords, ServiceDocuments, PurposeDocumentScan, "pdf"
	key, _ := k.Path()
	got, err := c.PresignDownload(context.Background(), BucketPrivate, key, 5*time.Minute, "Công văn\r\n\"số 1\".pdf")
	if err != nil {
		t.Fatalf("PresignDownload: %v", err)
	}
	u, err := url.Parse(got.URL())
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "files.example.test" {
		t.Fatalf("signed for %s, want the public endpoint", u.Host)
	}
	if u.Path != "/vigov-test-private/"+key {
		t.Errorf("path = %q", u.Path)
	}
	q := u.Query()
	if q.Get("response-content-type") != MIMEPDF {
		t.Errorf("response-content-type = %q", q.Get("response-content-type"))
	}
	cd := q.Get("response-content-disposition")
	if !strings.HasPrefix(cd, "attachment;") || strings.ContainsAny(cd, "\r\n") {
		t.Errorf("disposition = %q", cd)
	}
	if q.Get("X-Amz-Expires") != "300" || q.Get("X-Amz-Signature") == "" {
		t.Errorf("expires/signature = %q/%q", q.Get("X-Amz-Expires"), q.Get("X-Amz-Signature"))
	}
}

func TestPresignDownloadRefusals(t *testing.T) {
	c := testClient(t)
	key, _ := validKey().Path()
	up, _ := validKey().UploadPath()
	ctx := context.Background()
	if _, err := c.PresignDownload(ctx, BucketPrivate, key, 0, "a"); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("zero ttl: %v", err)
	}
	if _, err := c.PresignDownload(ctx, BucketPrivate, key, time.Hour, "a"); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("1h ttl: %v", err)
	}
	if _, err := c.PresignDownload(ctx, BucketPrivate, up, time.Minute, "a"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("upload key in private bucket: %v", err)
	}
	if _, err := c.PresignDownload(ctx, BucketTemp, key, time.Minute, "a"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("destination key in temp bucket: %v", err)
	}
	if _, err := c.PresignDownload(ctx, BucketPrivate, "../etc/passwd", time.Minute, "a"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("free-form key: %v", err)
	}
}

// Presigned URLs and POST forms are bearer credentials: no fmt or slog path may render them.
func TestPresignedValuesDoNotRender(t *testing.T) {
	post := PresignedPost{URL: "https://files.example.test/b/", Fields: map[string]string{"x-amz-signature": "SIG"}}
	u := PresignedURL("https://files.example.test/b/k?X-Amz-Signature=SIG")
	for _, s := range []string{
		fmt.Sprint(post), fmt.Sprintf("%+v", post), fmt.Sprintf("%#v", post), fmt.Sprintf("%v", []any{post}),
		fmt.Sprint(u), fmt.Sprintf("%s", u), fmt.Sprintf("%q", u), fmt.Sprintf("%#v", u),
		post.LogValue().String(), u.LogValue().String(),
	} {
		if strings.Contains(s, "SIG") || strings.Contains(s, "files.example.test") {
			t.Fatalf("rendered: %s", s)
		}
	}
	// JSON is the one path that must carry the value: that is how a handler hands it over.
	b, _ := json.Marshal(struct{ U PresignedURL }{u})
	if !strings.Contains(string(b), "SIG") {
		t.Errorf("JSON must carry the URL: %s", b)
	}
}

func TestPublicURL(t *testing.T) {
	c := testClient(t)
	k := validKey()
	k.Class, k.Variant = ClassPublicMedia, "mp4-720p"
	key, _ := k.Path()
	got, err := c.PublicURL(key)
	if err != nil || got != "https://cdn.example.test/vigov-test-public/"+key {
		t.Fatalf("PublicURL = %q, %v", got, err)
	}
	private, _ := validKey().Path()
	if _, err := c.PublicURL(private); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("non public-media class: %v", err)
	}
	cfg := testConfig()
	cfg.PublicMediaBaseURL = ""
	c2, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c2.PublicURL(key); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no base URL: %v", err)
	}
}

// These refusals happen before any network call, so they run without a server.
func TestPromoteAndPurgeRefuseBeforeTouchingTheServer(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	up, _ := validKey().UploadPath()
	dst := validKey()

	if _, err := c.Promote(ctx, up, "etag", dst, BucketPublic); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("promote into public: %v", err)
	}
	if _, err := c.Promote(ctx, up, "etag", dst, BucketTemp); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("promote into temp: %v", err)
	}
	if _, err := c.Promote(ctx, up, "", dst, BucketPrivate); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("promote without etag: %v", err)
	}
	other := dst
	other.TenantID = "01J9ZK7Q3M5N8P2R4T6V8W0XY0"
	if _, err := c.Promote(ctx, up, "etag", other, BucketPrivate); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("promote into another commune: %v", err)
	}
	otherService := dst
	otherService.Service, otherService.Purpose = ServicePetitions, PurposePetitionPhoto
	if _, err := c.Promote(ctx, up, "etag", otherService, BucketPrivate); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("promote into another service: %v", err)
	}

	rec := validKey()
	rec.Class = ClassRecords
	recKey, _ := rec.Path()
	if err := c.PurgeAllVersions(ctx, BucketPrivate, recKey); !errors.Is(err, ErrRecordsNotPurgeable) {
		t.Errorf("purge records: %v", err)
	}
	if err := c.PurgeAllVersions(ctx, BucketPrivate, "content-source/"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("purge a bare prefix: %v", err)
	}
}
