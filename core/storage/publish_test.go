package storage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// derivative is a private, approved-derivative key: the 720p transcode of validKey's video.
func derivative() Key {
	k := validKey()
	k.Variant = "mp4-720p"
	return k
}

// publicTwin is k with the one change PublishDerivative allows.
func publicTwin(k Key) Key {
	k.Class = ClassPublicMedia
	return k
}

// These refusals happen before any network call: testClient points at a host that does not exist,
// so reaching the server would fail with a different error than the one each case expects.
func TestPublishDerivativeRefusals(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	citizen := derivative()
	citizen.Class, citizen.Service, citizen.Purpose, citizen.Ext = ClassCitizenMedia, ServicePetitions, PurposePetitionPhoto, "jpg"
	citizen.Variant = "thumb-320"
	original := validKey()
	otherTenant := publicTwin(derivative())
	otherTenant.TenantID = "01J9ZK7Q3M5N8P2R4T6V8W0XY0"
	otherService := publicTwin(derivative())
	otherService.Service = ServicePetitions
	otherPurpose := publicTwin(derivative())
	otherPurpose.Purpose = PurposeContentImage
	otherObject := publicTwin(derivative())
	otherObject.ObjectID = "01JAB3CD4EF5GH6JK7MN8PQ9RT"
	otherVariant := publicTwin(derivative())
	otherVariant.Variant = VariantPoster
	otherExt := publicTwin(derivative())
	otherExt.Ext = "mov"
	otherMonth := publicTwin(derivative())
	otherMonth.CreatedAt = otherMonth.CreatedAt.AddDate(0, 1, 0)
	privateDst := derivative()
	privateDst.Class = ClassRecords
	badSrc := derivative()
	badSrc.TenantID = "not-a-ulid"

	cases := map[string]struct {
		src, dst Key
		want     error
	}{
		"citizen-media derivative":   {citizen, publicTwin(citizen), ErrNotPublishable},
		"citizen-media original":     {func() Key { k := citizen; k.Variant = VariantOriginal; return k }(), publicTwin(citizen), ErrNotPublishable},
		"original":                   {original, publicTwin(original), ErrNotPublishable},
		"records derivative":         {func() Key { k := derivative(); k.Class = ClassRecords; return k }(), publicTwin(derivative()), ErrNotPublishable},
		"source already public":      {publicTwin(derivative()), publicTwin(derivative()), ErrInvalidArgument},
		"destination not public":     {derivative(), privateDst, ErrInvalidArgument},
		"destination other commune":  {derivative(), otherTenant, ErrInvalidArgument},
		"destination other service":  {derivative(), otherService, ErrInvalidArgument},
		"destination other purpose":  {derivative(), otherPurpose, ErrInvalidArgument},
		"destination other object":   {derivative(), otherObject, ErrInvalidArgument},
		"destination other variant":  {derivative(), otherVariant, ErrInvalidArgument},
		"destination other ext":      {derivative(), otherExt, ErrInvalidArgument},
		"destination other month":    {derivative(), otherMonth, ErrInvalidArgument},
		"invalid source (same twin)": {badSrc, publicTwin(badSrc), ErrInvalidKey},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := c.PublishDerivative(ctx, tc.src, tc.dst); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestUnpublishDerivativeRefusals(t *testing.T) {
	c := testClient(t)
	if err := c.UnpublishDerivative(context.Background(), derivative()); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("private class: %v", err)
	}
	bad := publicTwin(derivative())
	bad.ObjectID = "x"
	if err := c.UnpublishDerivative(context.Background(), bad); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("invalid key: %v", err)
	}
}

// publishS3 is a fake MinIO for the publish/unpublish calls: HEAD and DELETE of the public object,
// the server-side copy (PUT with x-amz-copy-source) and a version listing. It records what it saw.
type publishS3 struct {
	mu        sync.Mutex
	exists    bool   // the public object exists
	copyFrom  string // x-amz-copy-source of the last copy
	copyHdr   http.Header
	copies    int
	deletes   []string // versionId of each DELETE
	pubPath   string
	otherReqs []string
}

func newPublishS3(t *testing.T, dstKey string) (*publishS3, *Client) {
	t.Helper()
	f := &publishS3{pubPath: "/vigov-test-public/" + dstKey}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		notFound := func() {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>x</Message></Error>`)
		}
		switch {
		case r.Method == http.MethodHead && r.URL.Path == f.pubPath:
			if !f.exists {
				notFound()
				return
			}
			w.Header().Set("ETag", `"e1"`)
			w.Header().Set("Content-Length", "10")
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == f.pubPath && r.Header.Get("x-amz-copy-source") != "":
			f.copies++
			f.copyFrom, _ = url.PathUnescape(r.Header.Get("x-amz-copy-source"))
			f.copyHdr = r.Header.Clone()
			f.exists = true
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><CopyObjectResult><ETag>"e1"</ETag><LastModified>%s</LastModified></CopyObjectResult>`,
				time.Now().UTC().Format(time.RFC3339))
		case r.Method == http.MethodGet && r.URL.Path == "/vigov-test-public/" && r.URL.Query().Has("versions"):
			w.Header().Set("Content-Type", "application/xml")
			body := `<?xml version="1.0" encoding="UTF-8"?><ListVersionsResult><Name>vigov-test-public</Name><IsTruncated>false</IsTruncated>`
			if f.exists {
				body += `<Version><Key>` + strings.TrimPrefix(f.pubPath, "/vigov-test-public/") +
					`</Key><VersionId>null</VersionId><IsLatest>true</IsLatest><ETag>"e1"</ETag><Size>10</Size></Version>`
			}
			fmt.Fprint(w, body+`</ListVersionsResult>`)
		case r.Method == http.MethodDelete && r.URL.Path == f.pubPath:
			f.deletes = append(f.deletes, r.URL.Query().Get("versionId"))
			f.exists = false
			w.WriteHeader(http.StatusNoContent)
		default:
			f.otherReqs = append(f.otherReqs, r.Method+" "+r.URL.String())
			notFound()
		}
	}))
	t.Cleanup(srv.Close)
	cfg := testConfig()
	cfg.Endpoint = srv.URL
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return f, c
}

func TestPublishDerivativeCopiesPrivateToPublic(t *testing.T) {
	src := derivative()
	dst := publicTwin(src)
	srcKey, _ := src.Path()
	dstKey, _ := dst.Path()
	f, c := newPublishS3(t, dstKey)
	ctx := context.Background()

	if err := c.PublishDerivative(ctx, src, dst); err != nil {
		t.Fatalf("PublishDerivative: %v", err)
	}
	if f.copies != 1 {
		t.Fatalf("copies = %d, want 1 (other requests: %v)", f.copies, f.otherReqs)
	}
	if want := "vigov-test-private/" + srcKey; strings.TrimPrefix(f.copyFrom, "/") != want {
		t.Errorf("copy source = %q, want %q", f.copyFrom, want)
	}
	for h, want := range map[string]string{
		"Cache-Control":            PublicCacheControl,
		"Content-Type":             MIMEMP4,
		"Content-Disposition":      ContentDisposition("", MIMEMP4),
		"x-amz-metadata-directive": "REPLACE",
	} {
		if got := f.copyHdr.Get(h); got != want {
			t.Errorf("%s = %q, want %q", h, got, want)
		}
	}
	if !strings.HasPrefix(f.copyHdr.Get("Content-Disposition"), "inline;") {
		t.Errorf("media must be inline: %q", f.copyHdr.Get("Content-Disposition"))
	}

	// Idempotent: the object exists now, so a retry copies nothing and succeeds.
	if err := c.PublishDerivative(ctx, src, dst); err != nil {
		t.Fatalf("second PublishDerivative: %v", err)
	}
	if f.copies != 1 {
		t.Errorf("retry copied again: copies = %d", f.copies)
	}

	// The published object has the anonymous URL PublicURL builds.
	if u, err := c.PublicURL(dstKey); err != nil || !strings.HasSuffix(u, "/"+dstKey) {
		t.Errorf("PublicURL = %q, %v", u, err)
	}
}

func TestUnpublishDerivativeDeletesEveryVersionAndIsIdempotent(t *testing.T) {
	dst := publicTwin(derivative())
	dstKey, _ := dst.Path()
	f, c := newPublishS3(t, dstKey)
	f.exists = true
	ctx := context.Background()

	if err := c.UnpublishDerivative(ctx, dst); err != nil {
		t.Fatalf("UnpublishDerivative: %v (other requests: %v)", err, f.otherReqs)
	}
	if len(f.deletes) != 1 || f.deletes[0] != "null" || f.exists {
		t.Fatalf("deletes = %v, exists = %v", f.deletes, f.exists)
	}
	if err := c.UnpublishDerivative(ctx, dst); err != nil {
		t.Fatalf("second UnpublishDerivative: %v", err)
	}
	if len(f.deletes) != 1 {
		t.Errorf("nothing left, yet deleted again: %v", f.deletes)
	}
}
