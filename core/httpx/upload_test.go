package httpx

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// part is one multipart part in the order it is written. file=true makes it a file part.
type part struct {
	name, value string
	file        bool
}

func multipartBody(t *testing.T, parts ...part) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		if p.file {
			fw, err := mw.CreateFormFile(p.name, "scene.jpg")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(fw, p.value); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := mw.WriteField(p.name, p.value); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func uploadRequest(body io.Reader, contentType string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", body)
	r.Header.Set(uploadHeaderContentType, contentType)
	return r
}

var testOpts = UploadOptions{MaxFileBytes: 1 << 10, Fields: []string{"purpose"}}

func TestReadUploadStreamsTheOneFile(t *testing.T) {
	content := strings.Repeat("x", 300)
	body, ct := multipartBody(t,
		part{name: UploadSizeField, value: "300"},
		part{name: "purpose", value: "scene"},
		part{name: UploadFileField, value: content, file: true})
	up, err := ReadUpload(httptest.NewRecorder(), uploadRequest(body, ct), testOpts)
	if err != nil {
		t.Fatalf("ReadUpload: %v", err)
	}
	if up.Size != 300 || up.Fields["purpose"] != "scene" || up.Filename != "scene.jpg" ||
		up.ContentType != "application/octet-stream" {
		t.Fatalf("unexpected request: size=%d fields=%v name=%q type=%q", up.Size, up.Fields, up.Filename, up.ContentType)
	}
	if _, ok := up.Fields[UploadSizeField]; ok {
		t.Fatal("size must be returned as Size, not left in Fields")
	}
	got, err := io.ReadAll(up.File)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != content {
		t.Fatalf("got %d bytes, want 300", len(got))
	}
	if err := up.Finish(); err != nil {
		t.Fatalf("Finish after a clean read: %v", err)
	}
	if up.Deadline.IsZero() || time.Until(up.Deadline) > UploadReadTimeout {
		t.Fatalf("deadline not set from the vendor bound: %v", up.Deadline)
	}
}

// The file is handed over before a byte of it is read: a reader the caller pulls from, not a buffer.
func TestReadUploadDoesNotReadTheFileBeforeReturning(t *testing.T) {
	head, ct := multipartBody(t, part{name: UploadSizeField, value: "5"}, part{name: UploadFileField, value: "hello", file: true})
	// Cut the body inside the file part: if ReadUpload consumed the file it would fail.
	cut := bytes.Index(head.Bytes(), []byte("hello"))
	r := uploadRequest(io.MultiReader(bytes.NewReader(head.Bytes()[:cut]), errReader{}), ct)
	up, err := ReadUpload(httptest.NewRecorder(), r, testOpts)
	if err != nil {
		t.Fatalf("ReadUpload read past the file header: %v", err)
	}
	if _, err := io.ReadAll(up.File); err == nil {
		t.Fatal("a broken file body must surface as a read error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func TestReadUploadRefusals(t *testing.T) {
	cases := []struct {
		name  string
		parts []part
		want  error
	}{
		{"missing size", []part{{name: UploadFileField, value: "abc", file: true}}, ErrUploadMalformed},
		{"size after file", []part{{name: UploadFileField, value: "abc", file: true}, {name: UploadSizeField, value: "3"}}, ErrUploadMalformed},
		{"no file", []part{{name: UploadSizeField, value: "3"}}, ErrUploadMalformed},
		{"size not a number", []part{{name: UploadSizeField, value: "3kb"}, {name: UploadFileField, value: "abc", file: true}}, ErrUploadMalformed},
		{"size zero", []part{{name: UploadSizeField, value: "0"}, {name: UploadFileField, value: "", file: true}}, ErrUploadMalformed},
		{"size negative", []part{{name: UploadSizeField, value: "-3"}, {name: UploadFileField, value: "abc", file: true}}, ErrUploadMalformed},
		{"size over the policy", []part{{name: UploadSizeField, value: "1025"}, {name: UploadFileField, value: "abc", file: true}}, ErrUploadTooLarge},
		{"unknown field", []part{{name: "tenant_id", value: "x"}, {name: UploadSizeField, value: "3"}, {name: UploadFileField, value: "abc", file: true}}, ErrUploadMalformed},
		{"field twice", []part{{name: "purpose", value: "a"}, {name: "purpose", value: "b"}, {name: UploadSizeField, value: "3"}, {name: UploadFileField, value: "abc", file: true}}, ErrUploadMalformed},
		{"field too long", []part{{name: "purpose", value: strings.Repeat("a", UploadMaxFieldBytes+1)}, {name: UploadSizeField, value: "3"}, {name: UploadFileField, value: "abc", file: true}}, ErrUploadMalformed},
		{"file under another name", []part{{name: UploadSizeField, value: "3"}, {name: "purpose", value: "abc", file: true}}, ErrUploadMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, ct := multipartBody(t, c.parts...)
			_, err := ReadUpload(httptest.NewRecorder(), uploadRequest(body, ct), testOpts)
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestReadUploadRefusesTooManyFields(t *testing.T) {
	names := make([]string, 0, UploadMaxFields+1)
	parts := make([]part, 0, UploadMaxFields+2)
	for i := range UploadMaxFields + 1 {
		n := "f" + strconv.Itoa(i)
		names = append(names, n)
		parts = append(parts, part{name: n, value: "v"})
	}
	parts = append(parts, part{name: UploadFileField, value: "abc", file: true})
	body, ct := multipartBody(t, parts...)
	_, err := ReadUpload(httptest.NewRecorder(), uploadRequest(body, ct),
		UploadOptions{MaxFileBytes: 1 << 10, Fields: names})
	if !errors.Is(err, ErrUploadMalformed) {
		t.Fatalf("got %v, want ErrUploadMalformed", err)
	}
}

// What follows the file is only visible once the file is read: the reader refuses at its end.
func TestReadUploadRefusesAnythingAfterTheFile(t *testing.T) {
	for name, trailer := range map[string]part{
		"second file":     {name: UploadFileField, value: "def", file: true},
		"text after file": {name: "purpose", value: "late"},
	} {
		t.Run(name, func(t *testing.T) {
			body, ct := multipartBody(t, part{name: UploadSizeField, value: "3"},
				part{name: UploadFileField, value: "abc", file: true}, trailer)
			up, err := ReadUpload(httptest.NewRecorder(), uploadRequest(body, ct), testOpts)
			if err != nil {
				t.Fatalf("ReadUpload: %v", err)
			}
			if _, err := io.ReadAll(up.File); !errors.Is(err, ErrUploadMalformed) {
				t.Fatalf("read: got %v, want ErrUploadMalformed", err)
			}
			if err := up.Finish(); !errors.Is(err, ErrUploadMalformed) {
				t.Fatalf("Finish: got %v, want ErrUploadMalformed", err)
			}
		})
	}
}

// A reader that stops at exactly Size bytes (as a storage write may) still gets the trailer checked.
func TestFinishChecksTheTrailerWhenTheCallerStoppedAtSize(t *testing.T) {
	body, ct := multipartBody(t, part{name: UploadSizeField, value: "3"},
		part{name: UploadFileField, value: "abc", file: true}, part{name: "purpose", value: "late"})
	up, err := ReadUpload(httptest.NewRecorder(), uploadRequest(body, ct), testOpts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(up.File, make([]byte, 3)); err != nil {
		t.Fatal(err)
	}
	if err := up.Finish(); !errors.Is(err, ErrUploadMalformed) {
		t.Fatalf("got %v, want ErrUploadMalformed", err)
	}
}

func TestReadUploadFileDisagreesWithDeclaredSize(t *testing.T) {
	for name, c := range map[string]struct{ size, content string }{
		"longer":  {"3", "abcd"},
		"shorter": {"5", "abc"},
	} {
		t.Run(name, func(t *testing.T) {
			body, ct := multipartBody(t, part{name: UploadSizeField, value: c.size},
				part{name: UploadFileField, value: c.content, file: true})
			up, err := ReadUpload(httptest.NewRecorder(), uploadRequest(body, ct), testOpts)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadAll(up.File); !errors.Is(err, ErrUploadMalformed) {
				t.Fatalf("got %v, want ErrUploadMalformed", err)
			}
		})
	}
}

func TestReadUploadNotMultipart(t *testing.T) {
	for _, ct := range []string{"", "application/json", "multipart/mixed; boundary=x", "multipart/form-data"} {
		_, err := ReadUpload(httptest.NewRecorder(), uploadRequest(strings.NewReader("{}"), ct), testOpts)
		want := ErrUploadNotMultipart
		if ct == "multipart/form-data" { // right type, no boundary: a broken envelope
			want = ErrUploadMalformed
		}
		if !errors.Is(err, want) {
			t.Fatalf("%q: got %v, want %v", ct, err, want)
		}
	}
}

// The body cap is the policy max + the envelope; bytes before the first part count too.
func TestReadUploadBodyOverTheCap(t *testing.T) {
	body, ct := multipartBody(t, part{name: UploadSizeField, value: "3"}, part{name: UploadFileField, value: "abc", file: true})
	preamble := strings.Repeat("p\r\n", int(testOpts.MaxFileBytes+UploadEnvelopeBytes)/3+1)
	r := uploadRequest(io.MultiReader(strings.NewReader(preamble), body), ct)
	_, err := ReadUpload(httptest.NewRecorder(), r, testOpts)
	if !errors.Is(err, ErrUploadTooLarge) {
		t.Fatalf("got %v, want ErrUploadTooLarge", err)
	}
	if UploadErrorStatus(err) != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", UploadErrorStatus(err))
	}
}

func TestReadUploadRequiresAPolicyMax(t *testing.T) {
	body, ct := multipartBody(t, part{name: UploadSizeField, value: "3"}, part{name: UploadFileField, value: "abc", file: true})
	defer func() {
		if recover() == nil {
			t.Fatal("a zero MaxFileBytes must panic: an uncapped upload is a wiring bug")
		}
	}()
	_, _ = ReadUpload(httptest.NewRecorder(), uploadRequest(body, ct), UploadOptions{})
}

func TestUploadErrorStatus(t *testing.T) {
	for err, want := range map[error]int{
		ErrUploadNotMultipart:                          http.StatusUnsupportedMediaType,
		ErrUploadTooLarge:                              http.StatusRequestEntityTooLarge,
		ErrUploadMalformed:                             http.StatusBadRequest,
		ErrUploadTimeout:                               http.StatusRequestTimeout,
		fmt.Errorf("store: %w", ErrUploadTimeout):      http.StatusRequestTimeout,
		errors.New("something else"):                   0,
		fmt.Errorf("wrapped: %w", io.ErrUnexpectedEOF): 0,
	} {
		if got := UploadErrorStatus(err); got != want {
			t.Fatalf("%v: got %d, want %d", err, got, want)
		}
	}
}

func TestParseUploadSize(t *testing.T) {
	if n, err := ParseUploadSize("1024", 1024); err != nil || n != 1024 {
		t.Fatalf("got %d %v", n, err)
	}
	for raw, want := range map[string]error{
		"": ErrUploadMalformed, "0": ErrUploadMalformed, "-1": ErrUploadMalformed, " 5": ErrUploadMalformed,
		"+5": ErrUploadMalformed, "1e3": ErrUploadMalformed, "99999999999999999999": ErrUploadMalformed,
		"1025": ErrUploadTooLarge,
	} {
		if _, err := ParseUploadSize(raw, 1024); !errors.Is(err, want) {
			t.Fatalf("%q: got %v, want %v", raw, err, want)
		}
	}
}

// A recorder supports no deadline (ResponseController answers ErrNotSupported): no panic, and the
// reader's own clock still refuses a read past the deadline.
func TestReadUploadDeadlineOnARecorder(t *testing.T) {
	body, ct := multipartBody(t, part{name: UploadSizeField, value: "3"}, part{name: UploadFileField, value: "abc", file: true})
	opts := testOpts
	opts.Timeout = time.Nanosecond
	up, err := ReadUpload(httptest.NewRecorder(), uploadRequest(body, ct), opts)
	if err != nil && !errors.Is(err, ErrUploadTimeout) {
		t.Fatalf("got %v", err)
	}
	if err == nil {
		time.Sleep(time.Millisecond)
		if _, err := io.ReadAll(up.File); !errors.Is(err, ErrUploadTimeout) {
			t.Fatalf("got %v, want ErrUploadTimeout", err)
		}
	}
}

// A real connection that stalls mid-file is cut by the read deadline — the slot is not held forever.
func TestReadUploadSlowBodyTimesOut(t *testing.T) {
	got := make(chan error, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		opts := testOpts
		opts.Timeout = 200 * time.Millisecond
		up, err := ReadUpload(w, r, opts)
		if err == nil {
			_, err = io.ReadAll(up.File)
		}
		got <- err
		if s := UploadErrorStatus(err); s != 0 {
			w.WriteHeader(s)
		}
	}))
	defer srv.Close()

	body, ct := multipartBody(t, part{name: UploadSizeField, value: "5"}, part{name: UploadFileField, value: "hello", file: true})
	cut := bytes.Index(body.Bytes(), []byte("hello")) + 2
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bw := bufio.NewWriter(conn)
	fmt.Fprintf(bw, "POST /api/v1/uploads HTTP/1.1\r\nHost: x\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n", ct, body.Len())
	bw.Write(body.Bytes()[:cut])
	if err := bw.Flush(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-got:
		if !errors.Is(err, ErrUploadTimeout) {
			t.Fatalf("got %v, want ErrUploadTimeout", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a stalled body held the handler past its deadline")
	}
}

func TestUploadSlotsFullThenReleased(t *testing.T) {
	s := NewUploadSlots(2)
	r1, ok1 := s.Acquire(context.Background())
	_, ok2 := s.Acquire(context.Background())
	if !ok1 || !ok2 {
		t.Fatal("two slots must be free")
	}
	if rel, ok := s.Acquire(context.Background()); ok || rel == nil {
		t.Fatal("third acquire must fail at once, with a callable release")
	}
	r1()
	r1() // twice is harmless: it must not free a slot it does not hold
	if _, ok := s.Acquire(context.Background()); !ok {
		t.Fatal("release must free its slot")
	}
	if _, ok := s.Acquire(context.Background()); ok {
		t.Fatal("a double release freed a second slot")
	}
}

func TestUploadSlotsRefuseADoneContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, ok := NewUploadSlots(1).Acquire(ctx); ok {
		t.Fatal("a gone client must not take a slot")
	}
}

func TestNewUploadSlotsRefusesZero(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("zero slots must panic: every upload would answer 503 forever")
		}
	}()
	NewUploadSlots(0)
}

func TestWriteUploadBusy(t *testing.T) {
	w := httptest.NewRecorder()
	WriteUploadBusy(w, "trace-1")
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") != strconv.Itoa(UploadRetryAfterSeconds) ||
		!strings.Contains(w.Body.String(), `"code":"`+UploadBusyCode+`"`) {
		t.Fatalf("got %d %v %s", w.Code, w.Header(), w.Body)
	}
}
