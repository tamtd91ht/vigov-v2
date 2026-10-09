package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
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

// The startup line names BOTH doors and the three buckets (09/10/2026: two endpoints reaching two
// stores showed only as "not received"), and carries no secret.
func TestLogAttrsNamesBothEndpointsAndBuckets(t *testing.T) {
	got := fmt.Sprint(testClient(t).LogAttrs()...)
	for _, want := range []string{"http://minio.internal.example:9000", "https://files.example.test",
		"vigov-test-private", "vigov-test-temp", "vigov-test-public"} {
		if !strings.Contains(got, want) {
			t.Errorf("LogAttrs lacks %q: %s", want, got)
		}
	}
	if strings.Contains(got, "FAKE-ACCESS-KEY") || strings.Contains(got, "fake-secret-key") {
		t.Errorf("LogAttrs leaks a credential: %s", got)
	}
}

// Destination keeps where and which key, and drops the query and every other form field.
func TestPresignedPostDestinationDropsCredentials(t *testing.T) {
	p := PresignedPost{
		URL:    "https://files.example.test/vigov-test-temp/?X-Amz-Signature=SIG",
		Fields: map[string]string{"key": "upload/x/original.jpg", "policy": "POLICY", "x-amz-signature": "SIG"},
	}
	target, key := p.Destination()
	if target != "https://files.example.test/vigov-test-temp/" || key != "upload/x/original.jpg" {
		t.Fatalf("Destination = %q, %q", target, key)
	}
}

// Every refusal of PutUpload happens before any byte is read and before any network call:
// testClient points at a host that does not exist, so reaching it would surface another error.
func TestPutUploadRefusals(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	up, _ := scenePhoto().UploadPath()
	export, _ := scenePhoto().ExportPath()
	dst, _ := scenePhoto().Path()
	body := jpegBytes(100)
	cases := map[string]struct {
		key, mime string
		r         io.Reader
		size, max int64
		want      []error
	}{
		"export key":          {export, MIMEJPEG, bytes.NewReader(body), 100, 1000, []error{ErrInvalidArgument, ErrInvalidKey}},
		"destination key":     {dst, MIMEJPEG, bytes.NewReader(body), 100, 1000, []error{ErrInvalidArgument, ErrInvalidKey}},
		"free-form key":       {"upload/../etc/passwd", MIMEJPEG, bytes.NewReader(body), 100, 1000, []error{ErrInvalidArgument, ErrInvalidKey}},
		"empty key":           {"", MIMEJPEG, bytes.NewReader(body), 100, 1000, []error{ErrInvalidArgument, ErrInvalidKey}},
		"html type":           {up, "text/html", bytes.NewReader(body), 100, 1000, []error{ErrTypeNotAllowed}},
		"type mismatches ext": {up, MIMEPNG, bytes.NewReader(body), 100, 1000, []error{ErrInvalidArgument}},
		"zero max":            {up, MIMEJPEG, bytes.NewReader(body), 100, 0, []error{ErrInvalidArgument}},
		"negative max":        {up, MIMEJPEG, bytes.NewReader(body), 100, -1, []error{ErrInvalidArgument}},
		"zero size":           {up, MIMEJPEG, bytes.NewReader(body), 0, 1000, []error{ErrInvalidArgument}},
		"negative size":       {up, MIMEJPEG, bytes.NewReader(body), -1, 1000, []error{ErrInvalidArgument}},
		"above the limit":     {up, MIMEJPEG, bytes.NewReader(body), 1001, 1000, []error{ErrTooLarge}},
		"nil reader":          {up, MIMEJPEG, nil, 100, 1000, []error{ErrInvalidArgument}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := c.PutUpload(ctx, tc.key, tc.r, tc.size, tc.max, tc.mime)
			for _, want := range tc.want {
				if !errors.Is(err, want) {
					t.Fatalf("err = %v, want %v", err, want)
				}
			}
		})
	}
	// The size limit is the caller's, distinct from a malformed call: a handler maps it to 413.
	if _, err := c.PutUpload(ctx, up, bytes.NewReader(body), 1001, 1000, MIMEJPEG); errors.Is(err, ErrInvalidArgument) {
		t.Errorf("too large is also ErrInvalidArgument (%v): the handler could not tell 413 from 400", err)
	}
}

// newUploadS3 is newProduceS3 pointed at the temp bucket and the upload key.
func newUploadS3(t *testing.T, uploadKey string) (*produceS3, *Client) {
	t.Helper()
	f, c := newProduceS3(t, uploadKey)
	f.mu.Lock()
	f.path = "/vigov-test-temp/" + uploadKey
	f.mu.Unlock()
	return f, c
}

func TestPutUploadStoresExactBytes(t *testing.T) {
	for name, size := range map[string]int{
		"small": 1000,
		// Larger than the read-ahead buffer and the final window: every branch of exactReader runs.
		"multi-buffer body": 3*exactBufSize + 123,
	} {
		t.Run(name, func(t *testing.T) {
			up, _ := scenePhoto().UploadPath()
			f, c := newUploadS3(t, up)
			body := jpegBytes(size)
			got, err := c.PutUpload(context.Background(), up, bytes.NewReader(body), int64(size), int64(size), MIMEJPEG)
			if err != nil {
				t.Fatalf("PutUpload: %v (other requests: %v)", err, f.otherReqs)
			}
			if got.Key != up || got.Size != int64(size) || got.ETag != "e2" || got.ContentType != MIMEJPEG {
				t.Fatalf("got %+v", got)
			}
			if !bytes.Equal(f.stored, body) {
				t.Fatalf("stored %d bytes, want the %d sent", len(f.stored), len(body))
			}
			if f.puts != 1 || len(f.otherReqs) != 0 {
				t.Errorf("want exactly one PUT, got puts=%d other=%v", f.puts, f.otherReqs)
			}
			if ct := f.putHdr.Get("Content-Type"); ct != MIMEJPEG {
				t.Errorf("Content-Type = %q", ct)
			}
			// Known size, one request: no multipart upload id, and a declared length that matches.
			if f.putHdr.Get("x-amz-decoded-content-length") != "" && f.putHdr.Get("x-amz-decoded-content-length") != strconv.Itoa(size) {
				t.Errorf("decoded length = %q", f.putHdr.Get("x-amz-decoded-content-length"))
			}
			for h := range f.putHdr {
				if strings.HasPrefix(strings.ToLower(h), "x-amz-meta-") {
					t.Errorf("user metadata written: %s", h)
				}
			}
		})
	}
}

// A body that does not match its declared size fails with ErrSizeMismatch, and the store never
// receives a whole body — nothing is committed under the upload key.
func TestPutUploadRefusesWrongLength(t *testing.T) {
	cases := map[string]struct{ actual, declared int }{
		"longer, small":  {101, 100},
		"longer, large":  {3*exactBufSize + 1, 3 * exactBufSize},
		"shorter, small": {99, 100},
		"shorter, large": {3 * exactBufSize, 3*exactBufSize + 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			up, _ := scenePhoto().UploadPath()
			f, c := newUploadS3(t, up)
			_, err := c.PutUpload(context.Background(), up, bytes.NewReader(jpegBytes(tc.actual)), int64(tc.declared), 1<<30, MIMEJPEG)
			if !errors.Is(err, ErrSizeMismatch) {
				t.Fatalf("err = %v, want ErrSizeMismatch", err)
			}
			if f.exists {
				t.Fatalf("a %d-byte body declared as %d was stored", tc.actual, tc.declared)
			}
		})
	}
}
