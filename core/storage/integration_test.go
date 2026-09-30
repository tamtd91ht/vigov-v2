package storage

// Integration test against a REAL MinIO. SKIPPED unless VIGOV_TEST_MINIO_ENDPOINT is set — and a
// skipped test prints `ok` exactly like a passing one (the VIGOV_TEST_DSN lesson, .env.example).
// Run with -v and look for SKIP. How to start a local MinIO: .env.example, section "tests".
//
// It creates three buckets under a random prefix (private versioned, as ADR 0052 §2 requires),
// runs the whole upload flow, and removes what it wrote.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"

	"github.com/vihat/vigov/core/secret"
)

func integrationClient(t *testing.T) (*Client, func()) {
	t.Helper()
	// @env-ok: test-only address of a throwaway MinIO, never configuration of a running service
	endpoint := os.Getenv("VIGOV_TEST_MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("VIGOV_TEST_MINIO_ENDPOINT unset — MinIO integration test SKIPPED (see .env.example)")
	}
	// @env-ok: test-only credentials of a throwaway MinIO
	access := os.Getenv("VIGOV_TEST_MINIO_ACCESS_KEY")
	// @env-ok: test-only credentials of a throwaway MinIO
	secretKey := os.Getenv("VIGOV_TEST_MINIO_SECRET_KEY")
	var rnd [4]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Endpoint:       endpoint,
		PublicEndpoint: endpoint, // the test process is the "browser"
		AccessKey:      secret.Secret(access),
		SecretKey:      secret.Secret(secretKey),
		BucketPrefix:   "vigov-it-" + hex.EncodeToString(rnd[:]),
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	for _, b := range []Bucket{BucketPrivate, BucketPublic, BucketTemp} {
		name, _ := c.BucketName(b)
		if err := c.api.MakeBucket(ctx, name, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			t.Fatalf("make bucket %s: %v", name, err)
		}
	}
	priv, _ := c.BucketName(BucketPrivate)
	if err := c.api.EnableVersioning(ctx, priv); err != nil {
		t.Fatalf("enable versioning: %v", err)
	}
	cleanup := func() {
		for _, b := range []Bucket{BucketPrivate, BucketPublic, BucketTemp} {
			name, _ := c.BucketName(b)
			for obj := range c.api.ListObjectsIter(ctx, name, minio.ListObjectsOptions{Recursive: true, WithVersions: true}) {
				if obj.Err == nil {
					_ = c.api.RemoveObject(ctx, name, obj.Key, minio.RemoveObjectOptions{VersionID: obj.VersionID})
				}
			}
			_ = c.api.RemoveBucket(ctx, name)
		}
	}
	return c, cleanup
}

// postForm performs the browser half: a multipart POST with every policy field, file last.
func postForm(t *testing.T, p PresignedPost, contentType string, body []byte) int {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range p.Fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	fw, err := w.CreateFormFile("file", "upload.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(p.URL, w.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestIntegrationUploadPromoteDownloadPurge(t *testing.T) {
	c, cleanup := integrationClient(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	objectID, err := NewObjectID()
	if err != nil {
		t.Fatal(err)
	}
	k := Key{
		Class: ClassContentSource, TenantID: testTenant, CreatedAt: time.Now(),
		Service: ServiceComms, Purpose: PurposeContentImage, ObjectID: objectID,
		Variant: VariantOriginal, Ext: "png",
	}
	up, err := k.UploadPath()
	if err != nil {
		t.Fatal(err)
	}
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{7}, 1000)...)

	// a. presign, with a size limit smaller than a second, oversized attempt.
	post, err := c.PresignUpload(ctx, up, 2000, MIMEPNG, 0)
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}
	if code := postForm(t, post, MIMEPNG, bytes.Repeat([]byte{1}, 3000)); code < 400 {
		t.Fatalf("oversized upload accepted (%d) — MinIO did not enforce content-length-range", code)
	}
	// b. the real upload.
	if code := postForm(t, post, MIMEPNG, png); code != http.StatusNoContent && code != http.StatusOK && code != http.StatusCreated {
		t.Fatalf("upload status %d", code)
	}

	// c. complete: stat → sniff → sha256 → promote, all bound to the ETag.
	st, err := c.Stat(ctx, BucketTemp, up)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if st.Size != int64(len(png)) {
		t.Fatalf("size %d", st.Size)
	}
	head, err := c.ReadHead(ctx, BucketTemp, up, st.ETag, SniffBytes)
	if err != nil {
		t.Fatalf("ReadHead: %v", err)
	}
	if mime, _, ok := SniffMIME(head); !ok || mime != MIMEPNG {
		t.Fatalf("sniffed %q, %v", mime, ok)
	}
	if _, err := c.ReadHead(ctx, BucketTemp, up, `"not-the-etag"`, SniffBytes); !errors.Is(err, ErrChanged) {
		t.Errorf("stale etag: %v, want ErrChanged", err)
	}
	// Open — the reader the malware scan streams — bound to the same ETag.
	rc, size, err := c.Open(ctx, BucketTemp, up, st.ETag)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	streamed, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || size != int64(len(png)) || !bytes.Equal(streamed, png) {
		t.Fatalf("Open streamed %d bytes (size %d), err %v", len(streamed), size, err)
	}
	if _, _, err := c.Open(ctx, BucketTemp, up, `"not-the-etag"`); !errors.Is(err, ErrChanged) {
		t.Errorf("Open with stale etag: %v, want ErrChanged", err)
	}
	sum, err := c.SHA256(ctx, BucketTemp, up, st.ETag)
	if err != nil {
		t.Fatalf("SHA256: %v", err)
	}
	want := sha256.Sum256(png)
	if sum != hex.EncodeToString(want[:]) {
		t.Fatalf("sha256 mismatch")
	}
	promoted, err := c.Promote(ctx, up, st.ETag, k, BucketPrivate)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	dstKey, _ := k.Path()
	if promoted.Key != dstKey || promoted.Size != int64(len(png)) || promoted.ContentType != MIMEPNG {
		t.Fatalf("promoted = %+v", promoted)
	}
	if _, err := c.Stat(ctx, BucketTemp, up); !errors.Is(err, ErrNotFound) {
		t.Errorf("temp object still there: %v", err)
	}
	final, err := c.Stat(ctx, BucketPrivate, dstKey)
	if err != nil || final.ContentType != MIMEPNG {
		t.Fatalf("final stat = %+v, %v", final, err)
	}

	// Download through the presigned GET.
	dl, err := c.PresignDownload(ctx, BucketPrivate, dstKey, time.Minute, "ảnh hiện trường.png")
	if err != nil {
		t.Fatalf("PresignDownload: %v", err)
	}
	resp, err := http.Get(dl.URL())
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Equal(got, png) {
		t.Fatalf("download status %d, %d bytes", resp.StatusCode, len(got))
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "inline;") {
		t.Errorf("Content-Disposition = %q", cd)
	}

	// Purge every version — twice, to prove idempotence.
	for i := 0; i < 2; i++ {
		if err := c.PurgeAllVersions(ctx, BucketPrivate, dstKey); err != nil {
			t.Fatalf("PurgeAllVersions #%d: %v", i+1, err)
		}
	}
	priv, _ := c.BucketName(BucketPrivate)
	for obj := range c.api.ListObjectsIter(ctx, priv, minio.ListObjectsOptions{Prefix: dstKey, WithVersions: true}) {
		t.Errorf("version left after purge: %q %q (err %v)", obj.Key, obj.VersionID, obj.Err)
	}
}

// PutServerProduced against a real MinIO, in the shape of both flows it serves:
// (b) a citizen photo uploaded to temp, re-encoded (here: a stand-in byte slice), stored as the
// private original, then the raw temp upload purged; (a) a content-source derivative that
// PublishDerivative then accepts.
func TestIntegrationPutServerProduced(t *testing.T) {
	c, cleanup := integrationClient(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	objectID, err := NewObjectID()
	if err != nil {
		t.Fatal(err)
	}
	photo := Key{
		Class: ClassCitizenMedia, TenantID: testTenant, CreatedAt: time.Now(),
		Service: ServicePetitions, Purpose: PurposePetitionPhoto, ObjectID: objectID,
		Variant: VariantOriginal, Ext: "jpg",
	}
	up, err := photo.UploadPath()
	if err != nil {
		t.Fatal(err)
	}
	raw := append([]byte{0xFF, 0xD8, 0xFF, 0xE1}, bytes.Repeat([]byte("exif"), 300)...)
	post, err := c.PresignUpload(ctx, up, 10_000, MIMEJPEG, 0)
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}
	if code := postForm(t, post, MIMEJPEG, raw); code >= 300 {
		t.Fatalf("upload status %d", code)
	}

	clean := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{9}, 700)...)
	got, err := c.PutServerProduced(ctx, photo, bytes.NewReader(clean), int64(len(clean)))
	if err != nil {
		t.Fatalf("PutServerProduced: %v", err)
	}
	photoKey, _ := photo.Path()
	sum, err := c.SHA256(ctx, BucketPrivate, photoKey, "")
	if err != nil {
		t.Fatalf("SHA256: %v", err)
	}
	want := sha256.Sum256(clean)
	if got.Key != photoKey || got.Size != int64(len(clean)) || got.ContentType != MIMEJPEG ||
		got.SHA256 != hex.EncodeToString(want[:]) || sum != got.SHA256 || got.VersionID == "" {
		t.Fatalf("produced = %+v, stored sha256 %s", got, sum)
	}
	if st, err := c.Stat(ctx, BucketPrivate, photoKey); err != nil || st.ContentType != MIMEJPEG {
		t.Fatalf("stat = %+v, %v", st, err)
	}
	if _, err := c.PutServerProduced(ctx, photo, bytes.NewReader(clean), int64(len(clean))); !errors.Is(err, ErrExists) {
		t.Errorf("second write: %v, want ErrExists", err)
	}
	// The raw upload never reached private; the caller removes it from temp.
	if err := c.PurgeAllVersions(ctx, BucketTemp, up); err != nil {
		t.Fatalf("purge temp: %v", err)
	}
	if _, err := c.Stat(ctx, BucketTemp, up); !errors.Is(err, ErrNotFound) {
		t.Errorf("temp upload still there: %v", err)
	}

	// Oversized body: refused, and nothing committed under the key.
	other := photo
	if other.ObjectID, err = NewObjectID(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PutServerProduced(ctx, other, bytes.NewReader(clean), int64(len(clean))-1); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("longer than declared: %v, want ErrInvalidArgument", err)
	}
	otherKey, _ := other.Path()
	if _, err := c.Stat(ctx, BucketPrivate, otherKey); !errors.Is(err, ErrNotFound) {
		t.Errorf("a refused body was stored: %v", err)
	}

	// (a) a cover derivative, then published.
	cover := Key{
		Class: ClassContentSource, TenantID: testTenant, CreatedAt: time.Now(),
		Service: ServiceComms, Purpose: PurposeContentImage, ObjectID: objectID,
		Variant: "thumb-1280", Ext: "jpg",
	}
	if _, err := c.PutServerProduced(ctx, cover, bytes.NewReader(clean), int64(len(clean))); err != nil {
		t.Fatalf("PutServerProduced cover: %v", err)
	}
	pub := cover
	pub.Class = ClassPublicMedia
	if err := c.PublishDerivative(ctx, cover, pub); err != nil {
		t.Fatalf("PublishDerivative: %v", err)
	}
}

// Publish / unpublish against a real MinIO. The private derivative is written directly with the
// raw client — the upload flow is proved above; this test is about the private → public copy.
func TestIntegrationPublishUnpublish(t *testing.T) {
	c, cleanup := integrationClient(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	objectID, err := NewObjectID()
	if err != nil {
		t.Fatal(err)
	}
	src := Key{
		Class: ClassContentSource, TenantID: testTenant, CreatedAt: time.Now(),
		Service: ServiceComms, Purpose: PurposeContentImage, ObjectID: objectID,
		Variant: "thumb-320", Ext: "png",
	}
	dst := src
	dst.Class = ClassPublicMedia
	srcKey, _ := src.Path()
	dstKey, _ := dst.Path()
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{7}, 100)...)
	priv, _ := c.BucketName(BucketPrivate)
	pub, _ := c.BucketName(BucketPublic)
	if _, err := c.api.PutObject(ctx, priv, srcKey, bytes.NewReader(png), int64(len(png)),
		minio.PutObjectOptions{ContentType: MIMEPNG}); err != nil {
		t.Fatalf("put private derivative: %v", err)
	}

	for i := 0; i < 2; i++ { // twice: idempotent
		if err := c.PublishDerivative(ctx, src, dst); err != nil {
			t.Fatalf("PublishDerivative #%d: %v", i+1, err)
		}
	}
	st, err := c.api.StatObject(ctx, pub, dstKey, minio.StatObjectOptions{})
	if err != nil {
		t.Fatalf("stat public: %v", err)
	}
	if st.Size != int64(len(png)) || st.ContentType != MIMEPNG {
		t.Errorf("public object = size %d type %q", st.Size, st.ContentType)
	}
	if cc := st.Metadata.Get("Cache-Control"); cc != PublicCacheControl {
		t.Errorf("Cache-Control = %q", cc)
	}

	for i := 0; i < 2; i++ { // twice: idempotent
		if err := c.UnpublishDerivative(ctx, dst); err != nil {
			t.Fatalf("UnpublishDerivative #%d: %v", i+1, err)
		}
	}
	if _, err := c.Stat(ctx, BucketPublic, dstKey); !errors.Is(err, ErrNotFound) {
		t.Errorf("public object still there: %v", err)
	}
	if _, err := c.Stat(ctx, BucketPrivate, srcKey); err != nil {
		t.Errorf("unpublish touched the private source: %v", err)
	}
}
