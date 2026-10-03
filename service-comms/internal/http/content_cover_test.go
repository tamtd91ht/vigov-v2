package http

// The cover upload routes (content_cover.go). The four-case permission suite for both routes runs in
// noi_dung_mini_app_test.go's table (cacTuyenND), through the REAL Register behind the REAL edge
// chain; this file holds the fake and the contract details specific to covers.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// fakeCovers records the commune and the actor it was called with — the two facts a handler can get
// wrong (rule 1, rule 6 invariant 8).
type fakeCovers struct {
	requests, completions, views int
	tenantID                     tenant.ID
	actor                        audit.Actor
	lastRequest                  app.CoverUploadRequest
	lastID                       string
	view                         app.CoverView
	err                          error

	bodyRequests, bodyCompletions, bodyViews int
	askedBodyIDs                             []string
	bodyView                                 map[string]app.BodyImageView
	bodyCompleteStatus                       domain.StoredFileStatus // "" = ready
}

func (f *fakeCovers) RequestUpload(ctx context.Context, req app.CoverUploadRequest, actor audit.Actor) (
	app.CoverUpload, error) {
	f.requests++
	f.tenantID, f.actor, f.lastRequest = tenant.MustFrom(ctx), actor, req
	if f.err != nil {
		return app.CoverUpload{}, f.err
	}
	return app.CoverUpload{
		File: domain.StoredFile{ID: "01JCOVERFILE00000000000000", Status: domain.StoredFilePending,
			OriginalName: "trao-qua.jpg"},
		Post: storage.PresignedPost{URL: "https://minio.example/vigov-prod-temp",
			Fields: map[string]string{"key": "upload/x"}, ExpiresAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)},
	}, nil
}

func (f *fakeCovers) Complete(ctx context.Context, id string, actor audit.Actor) (domain.StoredFile, error) {
	f.completions++
	f.tenantID, f.actor, f.lastID = tenant.MustFrom(ctx), actor, id
	if f.err != nil {
		return domain.StoredFile{}, f.err
	}
	return domain.StoredFile{ID: id, Status: domain.StoredFileReady, MIMEType: "image/jpeg", SizeBytes: 1234}, nil
}

func (f *fakeCovers) View(ctx context.Context, fileID string) (app.CoverView, error) {
	f.views++
	f.tenantID = tenant.MustFrom(ctx)
	return f.view, nil
}

// The body-image half (content_body_image.go) — counted apart, so the permission table can tell which
// route reached the use case.
func (f *fakeCovers) RequestBodyImageUpload(ctx context.Context, req app.CoverUploadRequest, actor audit.Actor) (
	app.CoverUpload, error) {
	f.bodyRequests++
	f.tenantID, f.actor, f.lastRequest = tenant.MustFrom(ctx), actor, req
	if f.err != nil {
		return app.CoverUpload{}, f.err
	}
	subject := req.ContentItemID
	if subject == "" {
		subject = "01JRESERVEDITEM00000000000"
	}
	return app.CoverUpload{
		File: domain.StoredFile{ID: "01JBODYFILE000000000000000", Status: domain.StoredFilePending,
			SubjectID: subject, OriginalName: "anh-hien-truong.jpg"},
		Post: storage.PresignedPost{URL: "https://minio.example/vigov-prod-temp",
			Fields: map[string]string{"key": "upload/y"}, ExpiresAt: time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)},
	}, nil
}

func (f *fakeCovers) CompleteBodyImageUpload(ctx context.Context, id string, actor audit.Actor) (domain.StoredFile, error) {
	f.bodyCompletions++
	f.tenantID, f.actor, f.lastID = tenant.MustFrom(ctx), actor, id
	if f.err != nil {
		return domain.StoredFile{}, f.err
	}
	status := domain.StoredFileReady
	if f.bodyCompleteStatus != "" {
		status = f.bodyCompleteStatus
	}
	return domain.StoredFile{ID: id, Status: status, MIMEType: "image/jpeg", SizeBytes: 4321,
		SubjectID: "01JRESERVEDITEM00000000000"}, nil
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

const (
	pathCoverImages     = "/api/v1/content-items/cover-images"
	bodyCoverUploadOK   = `{"file_name":"trao-qua.jpg","content_type":"image/jpeg","size":2048}`
	pathCoverCompletion = "/api/v1/content-items/cover-images/01JCOVERFILE00000000000000/completion"
)

func TestCoverUploadReplyCarriesTheFormAndNoStore(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goi(t, http.MethodPost, hostA, pathCoverImages,
		`{"file_name":"trao-qua.jpg","content_type":"image/jpeg","size":2048,"content_item_id":"nd-001"}`, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("a presigned form must not be cached: Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	var out coverUploadOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if out.Upload.URL == "" || out.Upload.Fields["key"] == "" || out.CoverImage.Status != "pending" {
		t.Errorf("reply = %+v", out)
	}
	// RULE 6, INVARIANT 8: the actor is the staff BUSINESS CODE from the session, never the id.
	if m.covers.actor.ID != canBo(xaA).Ma || m.covers.lastRequest.ContentItemID != "nd-001" ||
		m.covers.lastRequest.Size != 2048 {
		t.Errorf("use case got actor=%q req=%+v", m.covers.actor.ID, m.covers.lastRequest)
	}
}

func TestCoverErrorsMapToTheirStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"declared too large (policy)", app.ErrCoverTooLarge, http.StatusBadRequest, "invalid_request"},
		{"declared type not allowed", app.ErrCoverTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		{"infected", &app.CoverRejection{Reason: app.CoverRejectMalware}, http.StatusUnprocessableEntity, "cover_rejected"},
		{"too many pixels", &app.CoverRejection{Reason: app.CoverRejectTooManyPixels}, http.StatusUnprocessableEntity, "cover_rejected"},
		{"scanner down", app.ErrCoverScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{"not configured", app.ErrCoverUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{"limits unavailable", app.ErrCoverLimitsUnavailable, http.StatusServiceUnavailable, "upload_limits_unavailable"},
		{"not the caller's", app.ErrCoverFileNotFound, http.StatusNotFound, "not_found"},
		{"not received", app.ErrCoverUploadNotReceived, http.StatusConflict, "upload_not_received"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.covers.err = tc.err
			w := m.goi(t, http.MethodPost, hostA, pathCoverCompletion, "", canBo(xaA))
			doiMa(t, w, tc.want)
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Errorf("code: %s", w.Body.String())
			}
		})
	}
}

func TestCoverRejectionSentenceNamesMalwareAndNotTheFile(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.covers.err = &app.CoverRejection{Reason: app.CoverRejectMalware}
	w := m.goi(t, http.MethodPost, hostA, pathCoverCompletion, "", canBo(xaA))
	doiMa(t, w, http.StatusUnprocessableEntity)
	if !strings.Contains(w.Body.String(), "mã độc") || strings.Contains(w.Body.String(), "trao-qua") {
		t.Errorf("sentence: %s", w.Body.String())
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
