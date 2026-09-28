package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeS3 serves GetObject and HeadObject (minio's Object.Stat is a HEAD) for one object, honouring
// If-Match the way S3 does. Enough to prove Open maps precondition and existence to ErrChanged /
// ErrNotFound before any byte is streamed. A request WITHOUT If-Match is answered 400, so a test
// passing here also proves every request Open makes is conditional.
func fakeS3(t *testing.T, bucket, key, etag string, body []byte) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s3err := func(status int, code string) {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(status)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>x</Message></Error>`, code)
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			s3err(http.StatusMethodNotAllowed, "MethodNotAllowed")
			return
		}
		if r.URL.Path != "/"+bucket+"/"+key {
			s3err(http.StatusNotFound, "NoSuchKey")
			return
		}
		m := r.Header.Get("If-Match")
		if m == "" {
			s3err(http.StatusBadRequest, "UnconditionalRequest")
			return
		}
		if strings.Trim(m, `"`) != strings.Trim(etag, `"`) {
			s3err(http.StatusPreconditionFailed, "PreconditionFailed")
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)
	cfg := testConfig()
	cfg.Endpoint = srv.URL
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestOpenStreamsTheObject(t *testing.T) {
	up, _ := validKey().UploadPath()
	body := bytes.Repeat([]byte{0x42}, 100_000)
	c := fakeS3(t, "vigov-test-temp", up, `"abc123"`, body)
	rc, size, err := c.Open(context.Background(), BucketTemp, up, `"abc123"`)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rc.Close()
	if size != int64(len(body)) {
		t.Errorf("size = %d, want %d", size, len(body))
	}
	got, err := io.ReadAll(rc)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("read %d bytes, err %v", len(got), err)
	}
}

func TestOpenStaleETagIsChanged(t *testing.T) {
	up, _ := validKey().UploadPath()
	c := fakeS3(t, "vigov-test-temp", up, `"abc123"`, []byte("x"))
	if _, _, err := c.Open(context.Background(), BucketTemp, up, `"stale"`); !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want ErrChanged", err)
	}
}

func TestOpenMissingIsNotFound(t *testing.T) {
	up, _ := validKey().UploadPath()
	c := fakeS3(t, "vigov-test-temp", "upload/other", `"abc123"`, []byte("x"))
	if _, _, err := c.Open(context.Background(), BucketTemp, up, `"abc123"`); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// These refusals happen before any network call.
func TestOpenRefusals(t *testing.T) {
	c := testClient(t)
	up, _ := validKey().UploadPath()
	if _, _, err := c.Open(context.Background(), BucketTemp, up, ""); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("no etag: %v", err)
	}
	if _, _, err := c.Open(context.Background(), BucketTemp, "content-source/x", `"e"`); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("non-upload key in temp: %v", err)
	}
}
