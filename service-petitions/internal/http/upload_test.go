package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
)

// The envelope half every upload route shares (upload.go), proved once here; each route's suite proves
// its own wiring through the same helpers.
//
//	PROVED HERE   the multipart body the suites send · a busy pod answers 503 upload_busy + Retry-After
//	              and never asks the use case · the policy's cap bounds the body before a byte of the
//	              file is read (413) · not multipart is 415 · an undeclared field is 400 · every envelope
//	              failure is logged INFO with its code, never the file name · the declared type falls
//	              back to the part's header, the file name to the part's name.

// uploadThan is a multipart upload body for the suites' request helpers: `size` first (core/httpx
// requires it before the file), then the text fields, then the one `file` part.
type uploadThan struct {
	fields   [][2]string
	fileName string
	partType string
	data     []byte
	// size overrides the `size` field; "" sends len(data).
	size string
}

func (u uploadThan) encode(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	size := u.size
	if size == "" {
		size = strconv.Itoa(len(u.data))
	}
	if err := mw.WriteField(httpx.UploadSizeField, size); err != nil {
		t.Fatal(err)
	}
	for _, f := range u.fields {
		if err := mw.WriteField(f[0], f[1]); err != nil {
			t.Fatal(err)
		}
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+u.fileName+`"`)
	if u.partType != "" {
		h.Set("Content-Type", u.partType)
	}
	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(u.data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &b, mw.FormDataContentType()
}

// uploadSlotsThu is the per-pod cap every suite mounts: the real bound, a fresh value per server.
func uploadSlotsThu() *httpx.UploadSlots { return httpx.NewUploadSlots(httpx.UploadSlotsPerPod) }

// photoFile is a scene / verification photo upload. The file name looks like a person's — it must
// never reach the use case or a log line.
func photoFile() uploadThan {
	return uploadThan{fields: [][2]string{{uploadFieldContentType, storage.MIMEPNG}},
		fileName: "Nguyen Van A 0900000000.png", partType: storage.MIMEPNG, data: bytes.Repeat([]byte{7}, 2048)}
}

// attachmentFile is a log / task attachment upload with its display name.
func attachmentFile() uploadThan {
	return uploadThan{fields: [][2]string{{uploadFieldFileName, "Biên bản nghiệm thu.pdf"},
		{uploadFieldContentType, storage.MIMEPDF}}, fileName: "scan.pdf", partType: storage.MIMEPDF,
		data: bytes.Repeat([]byte{9}, 4096)}
}

// drainUpload is what every fake's Upload does with the body: read it all, then Finish — so the real
// envelope (httpx.ReadUpload) is exercised end to end.
func drainUpload(body app.UploadBody) (int, error) {
	n, err := io.Copy(io.Discard, body)
	if err != nil {
		return int(n), err
	}
	return int(n), body.Finish()
}

// --- the envelope, on a minimal handler -----------------------------------------------------------------

type limitFake struct {
	max   int64
	err   error
	calls int
}

func (l *limitFake) MaxUploadBytes(context.Context) (int64, error) { l.calls++; return l.max, l.err }

func serveEnvelope(t *testing.T, slots *httpx.UploadSlots, lim *limitFake, r *http.Request, log *bytes.Buffer) (
	*httptest.ResponseRecorder, *httpx.UploadRequest) {
	t.Helper()
	w := httptest.NewRecorder()
	var got *httpx.UploadRequest
	r = r.WithContext(tenant.Into(r.Context(), xaA))
	logger := slog.New(slog.NewTextHandler(log, nil))
	answer := func(err error) {
		refused := fileRefusalLog(r, logger, "thử: từ chối", "tải thử", err)
		if !writeUploadEnvelopeError(w, err, refused) {
			httpx.WriteError(w, http.StatusTeapot, "use_case", "x", "")
		}
	}
	up, release, ok := receiveUpload(w, r, slots, logger, lim, []string{uploadFieldFileName, uploadFieldContentType}, answer)
	defer release()
	if ok {
		if _, err := drainUpload(uploadBody{&up}); err != nil {
			answer(err)
		} else {
			got = &up
			w.WriteHeader(http.StatusCreated)
		}
	}
	return w, got
}

func uploadRequest(t *testing.T, u uploadThan) *http.Request {
	t.Helper()
	b, ct := u.encode(t)
	r := httptest.NewRequest(http.MethodPost, "/x", b)
	r.Header.Set("Content-Type", ct)
	return r
}

func TestUploadEnvelope(t *testing.T) {
	t.Run("accepted: fields, fallbacks to the part's own header and name", func(t *testing.T) {
		var log bytes.Buffer
		u := attachmentFile()
		u.fields = nil // no file_name, no content_type: the part's own are read
		w, up := serveEnvelope(t, uploadSlotsThu(), &limitFake{max: 1 << 20}, uploadRequest(t, u), &log)
		if w.Code != http.StatusCreated || up == nil {
			t.Fatalf("status %d", w.Code)
		}
		if declaredType(*up) != storage.MIMEPDF || declaredFileName(*up) != "scan.pdf" || up.Size != 4096 {
			t.Errorf("declared type %q name %q size %d", declaredType(*up), declaredFileName(*up), up.Size)
		}
	})
	t.Run("the fields win over the part", func(t *testing.T) {
		var log bytes.Buffer
		w, up := serveEnvelope(t, uploadSlotsThu(), &limitFake{max: 1 << 20}, uploadRequest(t, attachmentFile()), &log)
		if w.Code != http.StatusCreated || declaredFileName(*up) != "Biên bản nghiệm thu.pdf" {
			t.Fatalf("status %d name %q", w.Code, declaredFileName(*up))
		}
	})
	t.Run("busy: 503 upload_busy, Retry-After, use case never asked", func(t *testing.T) {
		var log bytes.Buffer
		slots := httpx.NewUploadSlots(1)
		release, _ := slots.Acquire(context.Background())
		defer release()
		lim := &limitFake{max: 1 << 20}
		w, _ := serveEnvelope(t, slots, lim, uploadRequest(t, attachmentFile()), &log)
		if w.Code != http.StatusServiceUnavailable || loiTra(t, w).Code != httpx.UploadBusyCode ||
			w.Header().Get("Retry-After") == "" {
			t.Fatalf("status %d %s %v", w.Code, w.Body.String(), w.Header())
		}
		if lim.calls != 0 {
			t.Error("the policy was asked although no slot was free")
		}
		if !strings.Contains(log.String(), "hết chỗ") {
			t.Errorf("busy not logged: %s", log.String())
		}
	})
	t.Run("the slot is released after the request", func(t *testing.T) {
		var log bytes.Buffer
		slots := httpx.NewUploadSlots(1)
		for i := 0; i < 3; i++ {
			if w, _ := serveEnvelope(t, slots, &limitFake{max: 1 << 20}, uploadRequest(t, attachmentFile()), &log); w.Code != http.StatusCreated {
				t.Fatalf("upload %d: %d — a slot leaked", i+1, w.Code)
			}
		}
	})
	for _, c := range []struct {
		name   string
		req    func(t *testing.T) *http.Request
		max    int64
		status int
		code   string
	}{
		{"declared size over the policy's cap: 413 before the file", func(t *testing.T) *http.Request {
			return uploadRequest(t, attachmentFile())
		}, 1024, http.StatusRequestEntityTooLarge, "file_too_large"},
		{"not multipart: 415", func(t *testing.T) *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"size":1}`))
			r.Header.Set("Content-Type", "application/json")
			return r
		}, 1 << 20, http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"a field the route does not declare: 400", func(t *testing.T) *http.Request {
			u := attachmentFile()
			u.fields = append(u.fields, [2]string{"tenant_id", "01JB"})
			return uploadRequest(t, u)
		}, 1 << 20, http.StatusBadRequest, "invalid_upload"},
		{"file shorter than declared: 400", func(t *testing.T) *http.Request {
			u := attachmentFile()
			u.size = "5000"
			return uploadRequest(t, u)
		}, 1 << 20, http.StatusBadRequest, "invalid_upload"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var log bytes.Buffer
			w, _ := serveEnvelope(t, uploadSlotsThu(), &limitFake{max: c.max}, c.req(t), &log)
			if w.Code != c.status || loiTra(t, w).Code != c.code {
				t.Fatalf("status %d body %s, want %d %s", w.Code, w.Body.String(), c.status, c.code)
			}
			for _, want := range []string{"level=INFO", "ma_loi=" + c.code} {
				if !strings.Contains(log.String(), want) {
					t.Errorf("log lacks %q: %s", want, log.String())
				}
			}
			if strings.Contains(log.String(), "Biên bản") || strings.Contains(log.String(), "scan.pdf") {
				t.Errorf("the file name reached the log: %s", log.String())
			}
		})
	}
	t.Run("the policy's own refusal goes through the route's answer", func(t *testing.T) {
		var log bytes.Buffer
		w, _ := serveEnvelope(t, uploadSlotsThu(), &limitFake{err: errors.New("not configured")},
			uploadRequest(t, attachmentFile()), &log)
		if w.Code != http.StatusTeapot {
			t.Fatalf("status %d", w.Code)
		}
	})
	t.Run("storage's size refusal is 413 too", func(t *testing.T) {
		w := httptest.NewRecorder()
		if !writeUploadEnvelopeError(w, errors.Join(errors.New("x"), storage.ErrTooLarge), func(string) {}) ||
			w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status %d", w.Code)
		}
	})
}
