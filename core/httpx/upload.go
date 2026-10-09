package httpx

// One upload = ONE multipart request to the service that owns the business record, streamed into the
// temp bucket (ADR 0052 §Sửa đổi 09/10/2026). Before that amendment devices POSTed straight to MinIO
// with a presigned form; a failure there was invisible to ViGov (no log, no error code), and MinIO's
// write path had to be open to the Internet. Now the bytes cross a pod, so this file bounds what one
// pod can be made to hold:
//
//	UploadSlots  how many uploads run at once   (extra → 503 upload_busy + Retry-After)
//	ReadUpload   how long, how big, what shape   (408 · 413 · 400 · 415)
//
// THE FILE IS NEVER BUFFERED. ReadUpload reads the small text fields, then hands back the `file` part
// itself as a reader; the caller pipes it into core/storage PutUpload. ParseMultipartForm is never
// used: it holds up to 32 MB in memory per request and spills the rest to a temporary file on the
// node (precedent: service-petitions/internal/http/catalogue_import.go:19).
//
// NOTHING A CLIENT SENT IS EVER IN AN ERROR OR A LOG LINE from here — not a field value, not a field
// name the client chose, not the file name (rule 3: a file name is often a person's name or phone).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// Vendor bounds, approved by the project owner as one package on 2026-10-09 — ADR 0052 §Sửa đổi
// 09/10/2026, row "Ngưỡng nhà cung cấp". Changing either is a decision for the owner, not a tuning
// knob: they are sized against the pod memory limits (petitions 384 MiB, GOMEMLIMIT 300 MiB).
const (
	// UploadSlotsPerPod: at most 4 uploads in flight per process. Each holds a connection, a MinIO
	// multipart buffer and — once complete — an inspection (decode, re-encode). Without a cap, a burst
	// of large files is how a 384 MiB pod gets OOM-killed, taking every other route down with it.
	UploadSlotsPerPod = 4
	// UploadReadTimeout: one upload may take at most 180 s. Without it a client that sends one byte a
	// minute holds a slot forever; four of them and the pod refuses every upload of every commune.
	UploadReadTimeout = 180 * time.Second
)

const (
	// UploadEnvelopeBytes is room for the multipart framing and the text fields on top of the file's
	// own cap. UploadMaxFields × UploadMaxFieldBytes (8 KiB) plus boundaries and part headers fits.
	UploadEnvelopeBytes = 64 << 10
	// UploadMaxFields bounds the text fields of one upload, `size` included.
	UploadMaxFields = 8
	// UploadMaxFieldBytes bounds one text field's value.
	UploadMaxFieldBytes = 1 << 10

	// UploadFileField is the one file part. UploadSizeField is the declared byte count, which must come
	// BEFORE the file: multipart does not carry a part's length, and the storage write needs it.
	UploadFileField = "file"
	UploadSizeField = "size"

	// UploadBusyCode is the fixed error code of the 503 when every slot is taken.
	UploadBusyCode = "upload_busy"
	// UploadRetryAfterSeconds is the Retry-After hint on that 503. A hint only — not an owner-set
	// bound; the client may wait longer.
	UploadRetryAfterSeconds = 5

	// Constants rather than literals: rbac_guard reads a header lookup by string literal as a route
	// declaration.
	uploadHeaderContentType = "Content-Type"
	uploadHeaderRetryAfter  = "Retry-After"
)

// The sentinels a handler maps to a status (UploadErrorStatus). Every error from ReadUpload, from
// the returned File, and from UploadRequest.Finish wraps exactly one of them.
var (
	ErrUploadNotMultipart = errors.New("upload: request is not multipart/form-data") // 415
	ErrUploadTooLarge     = errors.New("upload: over the size cap")                  // 413
	ErrUploadMalformed    = errors.New("upload: malformed multipart upload")         // 400
	ErrUploadTimeout      = errors.New("upload: not received within the time limit") // 408
)

// UploadErrorStatus is the status for an upload sentinel, or 0 when err is not one — then the caller
// decides (a storage failure is the caller's 5xx, never a 400 blamed on the client).
func UploadErrorStatus(err error) int {
	switch {
	case errors.Is(err, ErrUploadNotMultipart):
		return http.StatusUnsupportedMediaType
	case errors.Is(err, ErrUploadTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, ErrUploadTimeout):
		return http.StatusRequestTimeout
	case errors.Is(err, ErrUploadMalformed):
		return http.StatusBadRequest
	}
	return 0
}

// UploadSlots is the per-process cap on uploads in flight. One per service process, built at wiring
// with NewUploadSlots(UploadSlotsPerPod) and shared by every upload route of that service — a cap
// per route would let 8 routes hold 32 uploads.
type UploadSlots struct{ ch chan struct{} }

// NewUploadSlots panics when n < 1: zero slots would answer 503 to every upload forever, and that
// is a wiring bug to see at start, not in production.
func NewUploadSlots(n int) *UploadSlots {
	if n < 1 {
		panic("httpx.NewUploadSlots: n must be at least 1")
	}
	return &UploadSlots{ch: make(chan struct{}, n)}
}

// Acquire takes a slot WITHOUT WAITING. ok=false when every slot is taken (answer WriteUploadBusy) or
// the client is already gone. release is never nil, and calling it more than once frees one slot
// only — so `defer release()` is always safe.
//
// Not a queue on purpose: a waiting request still holds its connection and its unread body, which
// is the memory the cap exists to bound. The client retries after Retry-After instead.
func (s *UploadSlots) Acquire(ctx context.Context) (release func(), ok bool) {
	if ctx.Err() != nil {
		return func() {}, false
	}
	select {
	case s.ch <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-s.ch }) }, true
	default:
		return func() {}, false
	}
}

// WriteUploadBusy answers 503 upload_busy with Retry-After.
func WriteUploadBusy(w http.ResponseWriter, traceID string) {
	w.Header().Set(uploadHeaderRetryAfter, strconv.Itoa(UploadRetryAfterSeconds))
	WriteError(w, http.StatusServiceUnavailable, UploadBusyCode,
		"Hệ thống đang nhận nhiều tệp cùng lúc. Vui lòng thử lại sau ít giây.", traceID)
}

// UploadOptions is what one route declares.
type UploadOptions struct {
	// MaxFileBytes is the purpose's cap (platform policy, ADR 0052 §10). Required: zero panics — a
	// route with no cap is a wiring bug, and a default here would be a default nobody chose.
	MaxFileBytes int64
	// Fields are the text field names this route accepts besides `size`. Any other name is refused.
	Fields []string
	// Timeout overrides UploadReadTimeout. Zero = UploadReadTimeout. Only tests set it.
	Timeout time.Duration
}

// UploadRequest is a validated upload whose file has NOT been read yet.
type UploadRequest struct {
	// Fields holds the declared text fields that were sent (`size` is in Size, not here).
	Fields map[string]string
	// Size is the declared byte count: positive and ≤ MaxFileBytes. File yields exactly this many.
	Size int64
	// File streams the file part. It yields exactly Size bytes, then io.EOF only if nothing follows
	// the file; otherwise it fails with a wrapped sentinel. It is bound to the request: read it inside
	// the handler, before Deadline.
	File io.Reader
	// ContentType is the part's DECLARED type — client-supplied, so a hint, never a verdict. The
	// completion step sniffs the bytes.
	ContentType string
	// Filename is the client's file name. Client-supplied and possibly personal data: never log it,
	// never put it in a key. A route that has no use for it (the citizen photo) ignores it.
	Filename string
	// Deadline is when the read is cut. Use it for the storage write too:
	// context.WithDeadline(r.Context(), up.Deadline).
	Deadline time.Time

	file *uploadFile
}

// Finish confirms the whole file was read and nothing followed it. Call it after the storage write
// succeeds and before recording anything: a writer that stops at exactly Size bytes never reaches
// the end of the part, so a second file or a late field would otherwise go unnoticed.
func (u *UploadRequest) Finish() error {
	f := u.file
	if f.done {
		if errors.Is(f.err, io.EOF) {
			return nil
		}
		return f.err
	}
	if f.remaining > 0 {
		return fmt.Errorf("%w: file not read to its declared size", ErrUploadMalformed)
	}
	var one [1]byte
	if _, err := f.Read(one[:]); !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// ReadUpload validates the envelope of one upload and returns before reading a byte of the file.
//
// Wire shape: multipart/form-data; text fields first (`size` required, the route's Fields optional,
// each at most once, ≤ UploadMaxFieldBytes, ≤ UploadMaxFields in all); then exactly one part named
// `file`; then nothing.
//
// It caps the body at MaxFileBytes + UploadEnvelopeBytes and sets the connection's read deadline to
// now + UploadReadTimeout. Hold an UploadSlots slot BEFORE calling it.
func ReadUpload(w http.ResponseWriter, r *http.Request, opts UploadOptions) (UploadRequest, error) {
	if opts.MaxFileBytes <= 0 {
		panic("httpx.ReadUpload: UploadOptions.MaxFileBytes must be set")
	}
	mt, _, err := mime.ParseMediaType(r.Header.Get(uploadHeaderContentType))
	if err != nil || mt != "multipart/form-data" {
		return UploadRequest{}, ErrUploadNotMultipart
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = UploadReadTimeout
	}
	deadline := time.Now().Add(timeout)
	// The connection deadline is what unblocks a read stuck on a silent client. A writer that cannot
	// take one (httptest's recorder, some wrappers) answers ErrNotSupported; any other failure means
	// the connection is already unusable. Either way deadlineBody below still refuses every read past
	// the deadline — weaker (it cannot interrupt a blocked read), never absent.
	if err := http.NewResponseController(w).SetReadDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return UploadRequest{}, fmt.Errorf("%w: set read deadline: %w", ErrUploadMalformed, err)
	}
	r.Body = http.MaxBytesReader(w, &deadlineBody{rc: r.Body, deadline: deadline}, opts.MaxFileBytes+UploadEnvelopeBytes)

	mr, err := r.MultipartReader()
	if err != nil {
		return UploadRequest{}, fmt.Errorf("%w: %w", ErrUploadMalformed, err)
	}
	allowed := make(map[string]bool, len(opts.Fields)+1)
	for _, f := range opts.Fields {
		allowed[f] = true
	}
	allowed[UploadSizeField] = true

	fields := make(map[string]string, len(opts.Fields))
	size := int64(-1)
	count := 0
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return UploadRequest{}, fmt.Errorf("%w: no file part", ErrUploadMalformed)
		}
		if err != nil {
			return UploadRequest{}, classifyUploadRead(err, deadline)
		}
		name := part.FormName()
		if name == UploadFileField {
			if size < 0 {
				return UploadRequest{}, fmt.Errorf("%w: size must precede the file", ErrUploadMalformed)
			}
			f := &uploadFile{part: part, mr: mr, remaining: size, deadline: deadline}
			return UploadRequest{
				Fields:      fields,
				Size:        size,
				File:        f,
				ContentType: part.Header.Get(uploadHeaderContentType),
				Filename:    part.FileName(),
				Deadline:    deadline,
				file:        f,
			}, nil
		}
		// The client's name is not echoed: it is client-supplied text.
		if part.FileName() != "" {
			return UploadRequest{}, fmt.Errorf("%w: a file part under a name other than %q", ErrUploadMalformed, UploadFileField)
		}
		if !allowed[name] {
			return UploadRequest{}, fmt.Errorf("%w: a field this route does not declare", ErrUploadMalformed)
		}
		if _, dup := fields[name]; dup || (name == UploadSizeField && size >= 0) {
			return UploadRequest{}, fmt.Errorf("%w: a field sent twice", ErrUploadMalformed)
		}
		if count++; count > UploadMaxFields {
			return UploadRequest{}, fmt.Errorf("%w: too many fields", ErrUploadMalformed)
		}
		b, err := io.ReadAll(io.LimitReader(part, UploadMaxFieldBytes+1))
		if err != nil {
			return UploadRequest{}, classifyUploadRead(err, deadline)
		}
		if len(b) > UploadMaxFieldBytes {
			return UploadRequest{}, fmt.Errorf("%w: a field over %d bytes", ErrUploadMalformed, UploadMaxFieldBytes)
		}
		if name == UploadSizeField {
			if size, err = ParseUploadSize(string(b), opts.MaxFileBytes); err != nil {
				return UploadRequest{}, err
			}
			continue
		}
		fields[name] = string(b)
	}
}

// ParseUploadSize parses a declared byte count: plain decimal digits, > 0. Over max is
// ErrUploadTooLarge (413, refused before a byte of the file is read); anything else wrong is
// ErrUploadMalformed. ReadUpload calls it on `size`; it is exported for a route that receives the
// size another way.
func ParseUploadSize(raw string, max int64) (int64, error) {
	if raw == "" || len(raw) > 19 {
		return 0, fmt.Errorf("%w: size is not a byte count", ErrUploadMalformed)
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] < '0' || raw[i] > '9' {
			return 0, fmt.Errorf("%w: size is not a byte count", ErrUploadMalformed)
		}
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%w: size is not a positive byte count", ErrUploadMalformed)
	}
	if n > max {
		return 0, fmt.Errorf("%w: declared size over the cap", ErrUploadTooLarge)
	}
	return n, nil
}

// uploadFile yields exactly `remaining` bytes of the file part, then checks nothing follows it.
// Once it fails or ends it keeps returning the same result.
type uploadFile struct {
	part      *multipart.Part
	mr        *multipart.Reader
	remaining int64
	deadline  time.Time
	done      bool
	err       error
}

func (f *uploadFile) finish(err error) (int, error) {
	f.done, f.err = true, err
	return 0, err
}

func (f *uploadFile) Read(p []byte) (int, error) {
	if f.done {
		return 0, f.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	// Checked here too, not only under the body: the multipart reader buffers ahead, so the bytes of
	// a small file may already be in memory and never touch the connection again.
	if !time.Now().Before(f.deadline) {
		return f.finish(ErrUploadTimeout)
	}
	if f.remaining == 0 {
		return f.finish(f.checkEnd())
	}
	if int64(len(p)) > f.remaining {
		p = p[:f.remaining]
	}
	n, err := f.part.Read(p)
	f.remaining -= int64(n)
	switch {
	case errors.Is(err, io.EOF) && f.remaining > 0:
		f.done, f.err = true, fmt.Errorf("%w: file shorter than its declared size", ErrUploadMalformed)
		return n, f.err
	case errors.Is(err, io.EOF), err == nil:
		return n, nil
	default:
		f.done, f.err = true, classifyUploadRead(err, f.deadline)
		return n, f.err
	}
}

// checkEnd runs once Size bytes are out: the part must end there, and no part may follow it.
func (f *uploadFile) checkEnd() error {
	var one [1]byte
	for {
		n, err := f.part.Read(one[:])
		if n > 0 {
			return fmt.Errorf("%w: file longer than its declared size", ErrUploadMalformed)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return classifyUploadRead(err, f.deadline)
		}
	}
	_, err := f.mr.NextPart()
	switch {
	case errors.Is(err, io.EOF):
		return io.EOF
	case err == nil:
		return fmt.Errorf("%w: a part after the file", ErrUploadMalformed)
	default:
		return classifyUploadRead(err, f.deadline)
	}
}

// classifyUploadRead turns a failed read of the body into exactly one sentinel. A broken envelope
// or a client that hung up is 400; nothing read from the body is ever the server's fault.
func classifyUploadRead(err error, deadline time.Time) error {
	var tooBig *http.MaxBytesError
	var netErr net.Error
	switch {
	case errors.Is(err, ErrUploadTimeout), errors.Is(err, ErrUploadTooLarge), errors.Is(err, ErrUploadMalformed):
		return err
	case errors.As(err, &tooBig):
		return fmt.Errorf("%w: %w", ErrUploadTooLarge, err)
	case errors.Is(err, os.ErrDeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout(), !time.Now().Before(deadline):
		return fmt.Errorf("%w: %w", ErrUploadTimeout, err)
	default:
		return fmt.Errorf("%w: %w", ErrUploadMalformed, err)
	}
}

// deadlineBody refuses any read that starts after the deadline. It is the floor under the
// connection deadline: it holds where SetReadDeadline is not supported, and it stops a client that
// trickles bytes just fast enough to never block a single read.
type deadlineBody struct {
	rc       io.ReadCloser
	deadline time.Time
}

func (b *deadlineBody) Read(p []byte) (int, error) {
	if !time.Now().Before(b.deadline) {
		return 0, ErrUploadTimeout
	}
	return b.rc.Read(p)
}

func (b *deadlineBody) Close() error { return b.rc.Close() }
