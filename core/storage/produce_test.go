package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// jpegBytes is n bytes that sniff as JPEG, with a varying tail so a hash over the wrong slice
// would not collide with the right one by accident.
func jpegBytes(n int) []byte {
	b := make([]byte, n)
	copy(b, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	for i := 4; i < n; i++ {
		b[i] = byte(i * 31)
	}
	return b
}

// coverThumb is flow (a): the re-encoded cover derivative of a comms news image.
func coverThumb() Key {
	k := validKey()
	k.Purpose, k.Variant, k.Ext = PurposeContentImage, "thumb-1280", "jpg"
	return k
}

// scenePhoto is flow (b): the clean re-encode of a citizen scene photo, stored as its original.
func scenePhoto() Key {
	k := validKey()
	k.Class, k.Service, k.Purpose, k.Variant, k.Ext = ClassCitizenMedia, ServicePetitions, PurposePetitionPhoto, VariantOriginal, "jpg"
	return k
}

// Every refusal here happens before any network call: testClient points at a host that does not
// exist, so reaching the server would surface a different error than the one each case expects.
func TestPutServerProducedRefusals(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()
	img := jpegBytes(100)
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 100)...)
	html := []byte("<!doctype html><script>alert(1)</script>")

	records := coverThumb()
	records.Class = ClassRecords
	public := coverThumb()
	public.Class = ClassPublicMedia
	contentOriginal := coverThumb()
	contentOriginal.Variant = VariantOriginal
	badKey := coverThumb()
	badKey.TenantID = "not-a-ulid"
	openVariant := coverThumb()
	openVariant.Variant = "nguyen-van-a" // a slugified name is not a variant (rule 3)

	cases := map[string]struct {
		dst  Key
		r    io.Reader
		size int64
		want error
	}{
		"records":                  {records, bytes.NewReader(img), 100, ErrInvalidArgument},
		"records original":         {func() Key { k := records; k.Variant = VariantOriginal; return k }(), bytes.NewReader(img), 100, ErrInvalidArgument},
		"public-media":             {public, bytes.NewReader(img), 100, ErrInvalidArgument},
		"content-source original":  {contentOriginal, bytes.NewReader(img), 100, ErrInvalidArgument},
		"invalid key":              {badKey, bytes.NewReader(img), 100, ErrInvalidKey},
		"variant off closed list":  {openVariant, bytes.NewReader(img), 100, ErrInvalidKey},
		"nil reader":               {coverThumb(), nil, 100, ErrInvalidArgument},
		"zero size":                {coverThumb(), bytes.NewReader(img), 0, ErrInvalidArgument},
		"negative size":            {coverThumb(), bytes.NewReader(img), -1, ErrInvalidArgument},
		"above the cap":            {coverThumb(), bytes.NewReader(img), MaxServerProducedBytes + 1, ErrInvalidArgument},
		"sniffed type not allowed": {scenePhoto(), bytes.NewReader(html), int64(len(html)), ErrTypeNotAllowed},
		"sniff differs from ext":   {scenePhoto(), bytes.NewReader(png), int64(len(png)), ErrInvalidArgument},
		"empty reader":             {scenePhoto(), bytes.NewReader(nil), 100, ErrTypeNotAllowed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := c.PutServerProduced(ctx, tc.dst, tc.r, tc.size); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// produceS3 is a fake MinIO for PutServerProduced: HEAD of the destination and a plain PUT. It
// stores a body only when it arrived whole, as S3 does, so a test can tell "refused mid-stream"
// from "stored".
type produceS3 struct {
	mu        sync.Mutex
	path      string
	exists    bool
	stored    []byte
	putHdr    http.Header
	puts      int
	otherReqs []string
}

func newProduceS3(t *testing.T, dstKey string) (*produceS3, *Client) {
	t.Helper()
	f := &produceS3{path: "/vigov-test-private/" + dstKey}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		fail := func(status int, code string) {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(status)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>x</Message></Error>`, code)
		}
		switch {
		case r.Method == http.MethodHead && r.URL.Path == f.path:
			if !f.exists {
				fail(http.StatusNotFound, "NoSuchKey")
				return
			}
			w.Header().Set("ETag", `"e1"`)
			w.Header().Set("Content-Length", "10")
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == f.path && r.Header.Get("x-amz-copy-source") == "":
			f.puts++
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				fail(http.StatusBadRequest, "IncompleteBody")
				return
			}
			body, err := decodeAWSChunked(r.Header, raw)
			if err != nil {
				fail(http.StatusBadRequest, "IncompleteBody")
				return
			}
			if want, _ := strconv.Atoi(r.Header.Get("x-amz-decoded-content-length")); want != 0 && want != len(body) {
				fail(http.StatusBadRequest, "IncompleteBody")
				return
			}
			f.stored, f.putHdr, f.exists = body, r.Header.Clone(), true
			w.Header().Set("ETag", `"e2"`)
			w.Header().Set("x-amz-version-id", "v1")
			w.WriteHeader(http.StatusOK)
		default:
			f.otherReqs = append(f.otherReqs, r.Method+" "+r.URL.String())
			fail(http.StatusNotFound, "NoSuchKey")
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

// decodeAWSChunked undoes the streaming-signature body encoding minio-go uses over plain HTTP:
// `<hex size>;chunk-signature=…\r\n<data>\r\n`, ending with a zero-size chunk.
func decodeAWSChunked(h http.Header, b []byte) ([]byte, error) {
	if !strings.Contains(h.Get("x-amz-content-sha256"), "STREAMING") {
		return b, nil
	}
	var out []byte
	for {
		i := bytes.Index(b, []byte("\r\n"))
		if i < 0 {
			return nil, errors.New("truncated chunk header")
		}
		sizeHex, _, _ := strings.Cut(string(b[:i]), ";")
		n, err := strconv.ParseInt(sizeHex, 16, 64)
		if err != nil {
			return nil, err
		}
		b = b[i+2:]
		if n == 0 {
			return out, nil
		}
		if int64(len(b)) < n+2 {
			return nil, errors.New("truncated chunk")
		}
		out = append(out, b[:n]...)
		b = b[n+2:]
	}
}

func TestPutServerProducedStoresExactBytes(t *testing.T) {
	cases := map[string]struct {
		dst  Key
		size int
	}{
		"cover derivative (a)":       {coverThumb(), 1000},
		"citizen clean original (b)": {scenePhoto(), 1000},
		// Larger than the read-ahead buffer and the final window, so every branch of
		// exactReader runs: bounded reads, then the peek before the last bytes.
		"multi-buffer body":    {scenePhoto(), 3*exactBufSize + 123},
		"shorter than a sniff": {coverThumb(), SniffBytes / 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dstKey, _ := tc.dst.Path()
			f, c := newProduceS3(t, dstKey)
			body := jpegBytes(tc.size)
			got, err := c.PutServerProduced(context.Background(), tc.dst, bytes.NewReader(body), int64(len(body)))
			if err != nil {
				t.Fatalf("PutServerProduced: %v (other requests: %v)", err, f.otherReqs)
			}
			sum := sha256.Sum256(body)
			want := Produced{
				Key: dstKey, Size: int64(len(body)), ETag: "e2", VersionID: "v1",
				ContentType: MIMEJPEG, SHA256: hex.EncodeToString(sum[:]),
			}
			if got != want {
				t.Fatalf("got %+v\nwant %+v", got, want)
			}
			if !bytes.Equal(f.stored, body) {
				t.Fatalf("stored %d bytes, want the %d sent", len(f.stored), len(body))
			}
			if ct := f.putHdr.Get("Content-Type"); ct != MIMEJPEG {
				t.Errorf("Content-Type = %q, want the sniffed %q", ct, MIMEJPEG)
			}
			for h := range f.putHdr {
				if strings.HasPrefix(strings.ToLower(h), "x-amz-meta-") {
					t.Errorf("user metadata written: %s", h)
				}
			}
		})
	}
}

func TestPutServerProducedRefusesExisting(t *testing.T) {
	dst := scenePhoto()
	dstKey, _ := dst.Path()
	f, c := newProduceS3(t, dstKey)
	f.exists = true
	body := jpegBytes(100)
	if _, err := c.PutServerProduced(context.Background(), dst, bytes.NewReader(body), 100); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	if f.puts != 0 {
		t.Errorf("wrote over an existing object: puts = %d", f.puts)
	}
}

// A reader that does not yield exactly `size` bytes is refused, and the store never receives a
// whole body — so nothing is committed under dst.
func TestPutServerProducedRefusesWrongLength(t *testing.T) {
	cases := map[string]struct{ actual, declared int }{
		"longer, small":  {101, 100},
		"longer, large":  {3*exactBufSize + 1, 3 * exactBufSize},
		"shorter, small": {99, 100},
		"shorter, large": {3 * exactBufSize, 3*exactBufSize + 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dst := coverThumb()
			dstKey, _ := dst.Path()
			f, c := newProduceS3(t, dstKey)
			_, err := c.PutServerProduced(context.Background(), dst, bytes.NewReader(jpegBytes(tc.actual)), int64(tc.declared))
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("err = %v, want ErrInvalidArgument", err)
			}
			if f.exists {
				t.Fatalf("a %d-byte body declared as %d was stored", tc.actual, tc.declared)
			}
		})
	}
}
