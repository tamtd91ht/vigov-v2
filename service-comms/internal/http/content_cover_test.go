package http

// The cover upload route (content_cover.go) and the envelope every upload route shares (upload.go). The
// four-case permission suite runs in noi_dung_mini_app_test.go's table (cacTuyenND), through the REAL
// Register behind the REAL edge chain; this file holds the fake, the multipart wire helpers and the
// contract details: the reply, the envelope's refusals, the shared slots, the error codes.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// fakeCovers records the commune and the actor it was called with — the two facts a handler can get
// wrong (rule 1, rule 6 invariant 8) — and READS the stream it is handed, as the use case does.
type fakeCovers struct {
	requests, views int
	tenantID        tenant.ID
	actor           audit.Actor
	lastRequest     app.CoverUploadRequest
	lastID          string
	view            app.CoverView
	err             error
	// limit is the policy cap the handler asks for (0 = 1 MiB); limitErr refuses it.
	limit    int64
	limitErr error
	// received are the bytes of the last file part; finishErr what Finish said about what followed it.
	received  []byte
	finishErr error

	// completeSubject is the article the stored cover row names; "" = the reserved id.
	completeSubject string

	bodyRequests, bodyViews int
	askedBodyIDs            []string
	bodyView                map[string]app.BodyImageView
	bodyCompleteStatus      domain.StoredFileStatus // "" = ready

	// FetchBodyImage (content_body_image_url_test.go).
	fetches   int
	lastFetch app.BodyImageFromURLRequest
}

func (f *fakeCovers) FetchBodyImage(ctx context.Context, req app.BodyImageFromURLRequest, actor audit.Actor) (
	domain.StoredFile, error) {
	f.fetches++
	f.tenantID, f.actor, f.lastFetch = tenant.MustFrom(ctx), actor, req
	if f.err != nil {
		return domain.StoredFile{}, f.err
	}
	subject := req.ContentItemID
	if subject == "" {
		subject = "01JRESERVEDITEM00000000000"
	}
	return domain.StoredFile{ID: "01JFETCHEDFILE000000000000", Status: domain.StoredFileReady, MIMEType: "image/jpeg",
		SizeBytes: 5555, SubjectID: subject}, nil
}

func (f *fakeCovers) cap() (int64, error) {
	if f.limitErr != nil {
		return 0, f.limitErr
	}
	if f.limit == 0 {
		return 1 << 20, nil
	}
	return f.limit, nil
}

func (f *fakeCovers) CoverUploadLimit(context.Context) (int64, error)     { return f.cap() }
func (f *fakeCovers) BodyImageUploadLimit(context.Context) (int64, error) { return f.cap() }

// receive drains the stream like PutUpload, then calls Finish like the use case.
func (f *fakeCovers) receive(ctx context.Context, req app.CoverUploadRequest, actor audit.Actor) error {
	f.tenantID, f.actor, f.lastRequest = tenant.MustFrom(ctx), actor, req
	b, err := io.ReadAll(req.File)
	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrCoverUploadIncomplete, err)
	}
	f.received = b
	if f.finishErr = req.Finish(); f.finishErr != nil {
		return fmt.Errorf("%w: %w", app.ErrCoverUploadIncomplete, f.finishErr)
	}
	return f.err
}

func (f *fakeCovers) UploadCover(ctx context.Context, req app.CoverUploadRequest, actor audit.Actor) (
	domain.StoredFile, error) {
	f.requests++
	if err := f.receive(ctx, req, actor); err != nil {
		return domain.StoredFile{}, err
	}
	subject := req.ContentItemID
	if f.completeSubject != "" {
		subject = f.completeSubject
	}
	if subject == "" {
		subject = "01JRESERVEDITEM00000000000"
	}
	return domain.StoredFile{ID: "01JCOVERFILE00000000000000", Status: domain.StoredFileReady, MIMEType: "image/jpeg",
		SizeBytes: req.Size, SubjectID: subject, OriginalName: req.FileName}, nil
}

func (f *fakeCovers) View(ctx context.Context, fileID string) (app.CoverView, error) {
	f.views++
	f.tenantID = tenant.MustFrom(ctx)
	return f.view, nil
}

// The body-image half (content_body_image.go) — counted apart, so the permission table can tell which
// route reached the use case.
func (f *fakeCovers) UploadBodyImage(ctx context.Context, req app.CoverUploadRequest, actor audit.Actor) (
	domain.StoredFile, error) {
	f.bodyRequests++
	if err := f.receive(ctx, req, actor); err != nil {
		return domain.StoredFile{}, err
	}
	subject := req.ContentItemID
	if subject == "" {
		subject = "01JRESERVEDITEM00000000000"
	}
	status := domain.StoredFileReady
	if f.bodyCompleteStatus != "" {
		status = f.bodyCompleteStatus
	}
	return domain.StoredFile{ID: "01JBODYFILE000000000000000", Status: status, MIMEType: "image/jpeg",
		SizeBytes: req.Size, SubjectID: subject, OriginalName: req.FileName}, nil
}

func (f *fakeCovers) BodyImageViews(ctx context.Context, itemID string, fileIDs []string) ([]app.BodyImageView, error) {
	f.bodyViews++
	f.tenantID, f.lastID, f.askedBodyIDs = tenant.MustFrom(ctx), itemID, fileIDs
	out := make([]app.BodyImageView, 0, len(fileIDs))
	for _, id := range fileIDs {
		v, ok := f.bodyView[id]
		if !ok {
			v = app.BodyImageView{FileID: id}
		}
		out = append(out, v)
	}
	return out, nil
}

// --- the multipart wire shape (ADR 0052 §Sửa đổi 09/10/2026) ------------------------------------------

// multipartBoundary is fixed so a body can be a constant and its Content-Type a constant beside it.
const (
	multipartBoundary = "vigovtestboundary7d1"
	multipartCT       = "multipart/form-data; boundary=" + multipartBoundary
)

// multipartBody is one upload as a browser's FormData sends it: the text fields in order, then the `file`
// part (omitted when file is nil), then the closing boundary. The file part is named `x.bin` — never a
// name a test asserts on (rule 3 is the use case's; this is wire framing).
func multipartBody(fields [][2]string, file []byte) string {
	var b strings.Builder
	for _, f := range fields {
		fmt.Fprintf(&b, "--%s\r\nContent-Disposition: form-data; name=%q\r\n\r\n%s\r\n", multipartBoundary, f[0], f[1])
	}
	if file != nil {
		fmt.Fprintf(&b, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"x.bin\"\r\n"+
			"Content-Type: application/octet-stream\r\n\r\n", multipartBoundary)
		b.Write(file)
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", multipartBoundary)
	return b.String()
}

// fileBytes is a small file part; its size is what `size` declares.
var fileBytes = []byte("JPEGDATA")

func sizeOf(b []byte) [2]string { return [2]string{"size", strconv.Itoa(len(b))} }

const pathCoverImages = "/api/v1/content-items/cover-images"

var bodyCoverUploadOK = multipartBody([][2]string{sizeOf(fileBytes), {"file_name", "trao-qua.jpg"},
	{"content_type", "image/jpeg"}}, fileBytes)

func TestCoverUploadStreamsTheFileAndAnswersTheStoredFile(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goiVoi(t, http.MethodPost, hostA, pathCoverImages, multipartBody([][2]string{sizeOf(fileBytes),
		{"file_name", "trao-qua.jpg"}, {"content_type", "image/jpeg"}, {"content_item_id", "nd-001"}}, fileBytes),
		multipartCT, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	var out coverFileOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if out.ID != "01JCOVERFILE00000000000000" || out.Status != "ready" || out.ContentItemID != "nd-001" {
		t.Errorf("reply = %+v", out)
	}
	// NEVER AN UPLOAD FORM: the presigned POST is retired (ADR 0052 §Sửa đổi 09/10/2026).
	for _, gone := range []string{`"upload"`, `"url"`, `"fields"`, "trao-qua"} {
		if strings.Contains(w.Body.String(), gone) {
			t.Errorf("reply carries %s: %s", gone, w.Body.String())
		}
	}
	got := m.covers.lastRequest
	if string(m.covers.received) != string(fileBytes) || got.Size != int64(len(fileBytes)) ||
		got.FileName != "trao-qua.jpg" || got.ContentType != "image/jpeg" || got.ContentItemID != "nd-001" ||
		got.Deadline.IsZero() {
		t.Errorf("use case got %q / %+v", m.covers.received, got)
	}
	// RULE 6, INVARIANT 8: the actor is the staff BUSINESS CODE from the session, never the id.
	if m.covers.actor.ID != canBo(xaA).Ma {
		t.Errorf("actor = %q", m.covers.actor.ID)
	}
}

// Absent `file_name` / `content_type` fields fall back to the part's own filename and Content-Type.
func TestCoverUploadFallsBackToThePartsNameAndType(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	body := strings.Replace(multipartBody([][2]string{sizeOf(fileBytes)}, fileBytes),
		"Content-Type: application/octet-stream", "Content-Type: image/png; charset=binary", 1)
	doiMa(t, m.goiVoi(t, http.MethodPost, hostA, pathCoverImages, body, multipartCT, canBo(xaA)), http.StatusCreated)
	if got := m.covers.lastRequest; got.FileName != "x.bin" || got.ContentType != "image/png" {
		t.Errorf("use case got name %q type %q", got.FileName, got.ContentType)
	}
}

// THE COVER REPLY NAMES THE ARTICLE (TASK-03c): an officer who uploads the cover FIRST on a new article
// must learn the reserved id, or a later body image reserves a second one and the save answers 422.
func TestCoverUploadReplyCarriesContentItemID(t *testing.T) {
	for _, tc := range []struct {
		name, item, want string
	}{
		{"fresh reservation", "", "01JRESERVEDITEM00000000000"},
		{"existing article", "nd-001", "nd-001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			fields := [][2]string{sizeOf(fileBytes)}
			if tc.item != "" {
				fields = append(fields, [2]string{"content_item_id", tc.item})
			}
			w := m.goiVoi(t, http.MethodPost, hostA, pathCoverImages, multipartBody(fields, fileBytes), multipartCT, canBo(xaA))
			doiMa(t, w, http.StatusCreated)
			var out coverFileOut
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatalf("body is not JSON: %v", err)
			}
			if out.ContentItemID != tc.want {
				t.Errorf("content_item_id = %q, want %q", out.ContentItemID, tc.want)
			}
		})
	}
}

// The ENVELOPE's refusals: answered before the use case is reached, with a fixed code — the same on
// every upload route of this service.
func TestCoverUploadEnvelopeRefusals(t *testing.T) {
	big := make([]byte, 64)
	for _, tc := range []struct {
		name  string
		limit int64
		body  string
		ct    string
		want  int
		code  string
	}{
		{"JSON is no longer accepted", 0, `{"file_name":"a.jpg","content_type":"image/jpeg","size":8}`,
			"application/json", http.StatusUnsupportedMediaType, "upload_not_multipart"},
		{"declared size over the policy cap", 16, multipartBody([][2]string{sizeOf(big)}, big), multipartCT,
			http.StatusRequestEntityTooLarge, "file_too_large"},
		{"size after the file", 0, multipartBody(nil, fileBytes), multipartCT, http.StatusBadRequest, "invalid_upload"},
		{"no file part", 0, multipartBody([][2]string{sizeOf(fileBytes)}, nil), multipartCT,
			http.StatusBadRequest, "invalid_upload"},
		{"a field this route does not declare", 0, multipartBody([][2]string{sizeOf(fileBytes), {"phone", "0900000000"}},
			fileBytes), multipartCT, http.StatusBadRequest, "invalid_upload"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.covers.limit = tc.limit
			w := m.goiVoi(t, http.MethodPost, hostA, pathCoverImages, tc.body, tc.ct, canBo(xaA))
			doiMa(t, w, tc.want)
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Errorf("body: %s", w.Body.String())
			}
			if m.covers.requests != 0 {
				t.Error("a refused envelope reached the use case")
			}
			if strings.Contains(m.logs.String(), "0900000000") {
				t.Error("a field value reached the log")
			}
		})
	}
}

// A file longer or shorter than its declared size reaches the use case as a stream that fails: the use
// case (here, its fake) wraps the httpx sentinel and the handler answers its 400 — never a 5xx.
func TestCoverUploadStreamMismatchIs400(t *testing.T) {
	for name, body := range map[string]string{
		"longer than declared":  multipartBody([][2]string{{"size", "3"}}, fileBytes),
		"shorter than declared": multipartBody([][2]string{{"size", "99"}}, fileBytes),
		"a part after the file": strings.TrimSuffix(multipartBody([][2]string{sizeOf(fileBytes)}, fileBytes),
			"--"+multipartBoundary+"--\r\n") + multipartBody([][2]string{{"x", "y"}}, nil),
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			w := m.goiVoi(t, http.MethodPost, hostA, pathCoverImages, body, multipartCT, canBo(xaA))
			doiMa(t, w, http.StatusBadRequest)
			if !strings.Contains(w.Body.String(), `"code":"invalid_upload"`) {
				t.Errorf("body: %s", w.Body.String())
			}
			if !strings.Contains(m.logs.String(), "ma_loi=invalid_upload") {
				t.Errorf("the refusal is not logged: %s", m.logs.String())
			}
		})
	}
}

// EVERY SLOT TAKEN → 503 upload_busy + Retry-After, before a byte of the body is read; the slot of a
// finished upload is given back.
func TestUploadBusyIs503AndSlotsAreSharedAndReleased(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	var held []func()
	for i := 0; i < httpx.UploadSlotsPerPod; i++ {
		release, ok := m.slots.Acquire(context.Background())
		if !ok {
			t.Fatalf("slot %d not free", i)
		}
		held = append(held, release)
	}
	for _, path := range []string{pathCoverImages, pathBodyImages, pathAudioFiles} {
		w := m.goiVoi(t, http.MethodPost, hostA, path, bodyAudioUploadOK, multipartCT, canBo(xaA))
		doiMa(t, w, http.StatusServiceUnavailable)
		if !strings.Contains(w.Body.String(), `"code":"upload_busy"`) || w.Header().Get("Retry-After") == "" {
			t.Errorf("%s: %s (Retry-After %q)", path, w.Body.String(), w.Header().Get("Retry-After"))
		}
	}
	if m.covers.requests+m.covers.bodyRequests+m.audio.requests != 0 {
		t.Error("a busy upload reached a use case")
	}
	held[0]()
	w := m.goiVoi(t, http.MethodPost, hostA, pathCoverImages, bodyCoverUploadOK, multipartCT, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	// The upload above gave its slot back: one more fits.
	if release, ok := m.slots.Acquire(context.Background()); !ok {
		t.Error("the finished upload kept its slot")
	} else {
		release()
	}
}

func TestCoverErrorsMapToTheirStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"declared too large (policy)", app.ErrCoverTooLarge, http.StatusRequestEntityTooLarge, "file_too_large"},
		{"declared type not allowed", app.ErrCoverTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		{"infected", &app.CoverRejection{Reason: app.CoverRejectMalware}, http.StatusUnprocessableEntity, "cover_rejected"},
		{"too many pixels", &app.CoverRejection{Reason: app.CoverRejectTooManyPixels}, http.StatusUnprocessableEntity, "cover_rejected"},
		{"scanner down", app.ErrCoverScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{"not configured", app.ErrCoverUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{"limits unavailable", app.ErrCoverLimitsUnavailable, http.StatusServiceUnavailable, "upload_limits_unavailable"},
		{"temp bucket down", fmt.Errorf("%w: storage: put upload: bucket missing", app.ErrCoverUploadIncomplete),
			http.StatusServiceUnavailable, "upload_store_unavailable"},
		{"write over the cap", fmt.Errorf("%w: %w", app.ErrCoverUploadIncomplete, storage.ErrTooLarge),
			http.StatusRequestEntityTooLarge, "file_too_large"},
		{"write timed out", fmt.Errorf("%w: %w", app.ErrCoverUploadIncomplete, httpx.ErrUploadTimeout),
			http.StatusRequestTimeout, "upload_timeout"},
		{"not the caller's", app.ErrCoverFileNotFound, http.StatusNotFound, "not_found"},
		{"no such article", commsstore.ErrNoiDungKhongTonTai, http.StatusNotFound, "not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.covers.err = tc.err
			w := m.goiVoi(t, http.MethodPost, hostA, pathCoverImages, bodyCoverUploadOK, multipartCT, canBo(xaA))
			doiMa(t, w, tc.want)
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Errorf("code: %s", w.Body.String())
			}
		})
	}
}

// The policy read that caps the body is the use case's: its refusals answer as the upload's would.
func TestCoverUploadLimitRefusalsAnswerBeforeTheBody(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.covers.limitErr = app.ErrCoverLimitsUnavailable
	w := m.goiVoi(t, http.MethodPost, hostA, pathCoverImages, bodyCoverUploadOK, multipartCT, canBo(xaA))
	doiMa(t, w, http.StatusServiceUnavailable)
	if !strings.Contains(w.Body.String(), `"code":"upload_limits_unavailable"`) || m.covers.requests != 0 {
		t.Errorf("body: %s (requests %d)", w.Body.String(), m.covers.requests)
	}
}

func TestCoverRejectionSentenceNamesMalwareAndNotTheFile(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.covers.err = &app.CoverRejection{Reason: app.CoverRejectMalware}
	w := m.goiVoi(t, http.MethodPost, hostA, pathCoverImages, bodyCoverUploadOK, multipartCT, canBo(xaA))
	doiMa(t, w, http.StatusUnprocessableEntity)
	if !strings.Contains(w.Body.String(), "mã độc") || strings.Contains(w.Body.String(), "trao-qua") {
		t.Errorf("sentence: %s", w.Body.String())
	}
	if strings.Contains(m.logs.String(), "trao-qua") {
		t.Errorf("the file name reached the log: %s", m.logs.String())
	}
}

func TestContentDetailCarriesCoverPreview(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	n := noiDungMau()
	n.CoverImageFileID = "01JCOVERFILE00000000000000"
	m.so.mot = n
	exp := time.Date(2026, 10, 1, 9, 15, 0, 0, time.UTC)
	m.covers.view = app.CoverView{FileID: n.CoverImageFileID, Status: domain.StoredFileReady, Public: true,
		PreviewURL: storage.PresignedURL("https://minio.example/signed"), PreviewExpiresAt: exp}

	w := m.goi(t, http.MethodGet, hostA, duongNoiDung+"/nd-001", "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	var got noiDungRa
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if got.CoverImageFileID != n.CoverImageFileID || got.CoverImage == nil ||
		got.CoverImage.PreviewURL != "https://minio.example/signed" || !got.CoverImage.Public ||
		got.CoverImage.Status != "ready" {
		t.Fatalf("cover block = %+v", got.CoverImage)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("a reply carrying a presigned preview must be no-store")
	}
	if !got.HasImage {
		t.Error("has_image must be true with an uploaded cover")
	}
	if m.covers.tenantID != xaA {
		t.Errorf("cover view read in commune %q, want %q", m.covers.tenantID, xaA)
	}
}

func TestContentListCarriesCoverIDButNoPreview(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	n := noiDungMau()
	n.AnhDaiDienURL, n.CoverImageFileID = "", "01JCOVERFILE00000000000000"
	m.so.ra.Items = append(m.so.ra.Items, n)
	w := m.goi(t, http.MethodGet, hostA, duongNoiDung, "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	if !strings.Contains(body, `"cover_image_file_id":"01JCOVERFILE00000000000000"`) ||
		!strings.Contains(body, `"has_image":true`) || strings.Contains(body, `"cover_image":`) {
		t.Errorf("list: %s", body)
	}
	if m.covers.views != 0 {
		t.Error("the list must not sign a preview per row")
	}
}

func TestCreateAndPatchPassCoverFileID(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	doiMa(t, m.goi(t, http.MethodPost, hostA, duongNoiDung,
		`{"type":"tin-tuc","title":"Tin","publish":true,"cover_image_file_id":"01JCOVERFILE00000000000000"}`,
		canBo(xaA)), http.StatusCreated)
	if m.ghi.themCuoi.CoverImageFileID != "01JCOVERFILE00000000000000" {
		t.Errorf("create got cover %q", m.ghi.themCuoi.CoverImageFileID)
	}
	// "" DETACHES — a pointer to "", never nil.
	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"cover_image_file_id":""}`,
		canBo(xaA)), http.StatusOK)
	if p := m.ghi.suaCuoi.CoverImageFileID; p == nil || *p != "" {
		t.Errorf("detach must reach the use case as a pointer to \"\": %v", p)
	}
	// ABSENT leaves it alone.
	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"title":"Khác"}`,
		canBo(xaA)), http.StatusOK)
	if m.ghi.suaCuoi.CoverImageFileID != nil {
		t.Error("an absent cover_image_file_id must leave the cover alone")
	}
}

func TestCoverNotUsableIs409(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.ghi.loi = domain.ErrCoverNotUsable
	w := m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001",
		`{"cover_image_file_id":"01JOTHERFILE00000000000000"}`, canBo(xaA))
	doiMa(t, w, http.StatusConflict)
	if !strings.Contains(w.Body.String(), `"code":"cover_not_usable"`) {
		t.Errorf("body: %s", w.Body.String())
	}
}

func TestCoverPublishUnavailableIs503(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.ghi.loi = app.ErrCoverPublishUnavailable
	w := m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"publish":true}`, canBo(xaA))
	doiMa(t, w, http.StatusServiceUnavailable)
}
