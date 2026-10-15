package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// POST /api/v1/citizen-reports/{maTraCuu}/photos — the officer's scene photo at intake (owner decision
// 15/10/2026).
//
//	PROVED HERE   rule 5 invariant 7's four cases: 401 no session · 403 a REAL neighbouring key (the read
//	              key of the same path, the closing key) · 401 right key WRONG COMMUNE (this package's
//	              convention: authz compares the commune before the key) · 201 — the use case NOT reached in
//	              the first three · the business code and the restricted fact reach the use case · `can-bo`
//	              without `feedback.restricted` is the detail's 404 byte for byte · the refusals map to
//	              fixed sentences without the commune · the GET on the same path keeps `feedback.read`.
//	NOT PROVED    the flow — internal/app/staff_scene_photo_test.go.

const staffScenePhotoIDHTTP = "01JSTAFFSCENEPHOTO00000001"

// staffScenePhotosFake applies the commune and the restricted fact itself, so the isolation cases fail if
// the handler hands down the wrong ones.
type staffScenePhotosFake struct {
	calls      int
	seenTenant tenant.ID
	actor      audit.Actor
	req        app.PhotoUploadRequest
	restricted bool
	err        error
}

func (f *staffScenePhotosFake) MaxUploadBytes(context.Context) (int64, error) { return 10 << 20, nil }

func (f *staffScenePhotosFake) Upload(ctx context.Context, ma string, req app.PhotoUploadRequest,
	body app.UploadBody, actor audit.Actor, restricted app.QuyenXemHanChe) (domain.StoredFile, error) {
	f.calls++
	f.seenTenant, f.actor, f.req, f.restricted = tenant.MustFrom(ctx), actor, req, bool(restricted)
	if f.err != nil {
		return domain.StoredFile{}, f.err
	}
	switch {
	case tenant.MustFrom(ctx) == xaA && ma == maPhieuThuong:
	case tenant.MustFrom(ctx) == xaA && ma == maPhieuCanBo && bool(restricted):
	default:
		return domain.StoredFile{}, petstore.ErrPhieuKhongTonTai
	}
	if _, err := drainUpload(body); err != nil {
		return domain.StoredFile{}, err
	}
	return domain.StoredFile{ID: staffScenePhotoIDHTTP, MIMEType: storage.MIMEJPEG, SizeBytes: 300_000,
		Status: domain.StoredFileStored, CreatedAt: photoAtHTTP}, nil
}

func staffScenePhotosPath(code string) string { return duong(code) + "/photos" }

// --- rule 5, invariant 7 --------------------------------------------------------------------------

func TestStaffScenePhoto_NoSessionIs401(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, staffScenePhotosPath(maPhieuThuong), nil, photoFile()),
		http.StatusUnauthorized)
	if m.staffScenePhotos.calls != 0 {
		t.Error("use case reached without a session")
	}
}

// `feedback.read` opens the GET on the same path, never this write; neither does the closing key.
func TestStaffScenePhoto_WrongKeyIs403(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.read", "feedback.resolve", "feedback.classify", "feedback.assign")
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, staffScenePhotosPath(maPhieuThuong), canBoCuaXa(xaA), photoFile()),
		http.StatusForbidden)
	if m.staffScenePhotos.calls != 0 {
		t.Error("use case reached without feedback.create")
	}
}

func TestStaffScenePhoto_RightKeyWrongCommuneIs401(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostB, staffScenePhotosPath(maPhieuXaB), canBoCuaXa(xaA), photoFile()),
		http.StatusUnauthorized)
	if m.staffScenePhotos.calls != 0 {
		t.Error("commune B reached with commune A's session — rò rỉ giữa hai cơ quan nhà nước")
	}
}

func TestStaffScenePhoto_RightKeyRightCommuneIs201(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffScenePhotosPath(maPhieuThuong), canBoCuaXa(xaA), photoFile())
	doiMa(t, w, http.StatusCreated)
	f := m.staffScenePhotos
	if f.calls != 1 || f.seenTenant != xaA {
		t.Fatalf("calls %d, commune %q", f.calls, f.seenTenant)
	}
	if f.actor.ID != maCanBo || f.actor.Kind != "staff" || f.actor.IP == "" {
		t.Errorf("actor = %+v, want the BUSINESS CODE %q, kind staff, an IP (rule 6, invariants 2, 8)", f.actor, maCanBo)
	}
	if f.req != (app.PhotoUploadRequest{ContentType: storage.MIMEPNG, Size: 2048}) {
		t.Errorf("declared = %+v", f.req)
	}
	if f.restricted {
		t.Error("restricted fact handed down true without feedback.restricted")
	}
	var p photoOut
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.ID != staffScenePhotoIDHTTP || p.Status != "stored" ||
		p.ContentType != storage.MIMEJPEG {
		t.Errorf("reply = %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), maPhieuThuong) {
		t.Errorf("reply echoes the lookup code: %s", w.Body.String())
	}
	// The same path's GET is untouched: still `feedback.read`, still the read use case.
	m.capQuyen(t, "feedback.create")
	doiMa(t, m.goi(t, http.MethodGet, hostA, staffScenePhotosPath(maPhieuThuong), canBoCuaXa(xaA)), http.StatusForbidden)
}

// A `can-bo` petition without `feedback.restricted` is the detail's 404, byte for byte; with it, 201.
func TestStaffScenePhoto_RestrictedFieldIsTheDetails404(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create", "feedback.read")
	unknown := m.goi(t, http.MethodGet, hostA, duong(maKhongTonTai), canBoCuaXa(xaA))
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffScenePhotosPath(maPhieuCanBo), canBoCuaXa(xaA), photoFile())
	doiMa(t, w, http.StatusNotFound)
	if w.Body.String() != unknown.Body.String() {
		t.Errorf("restricted 404 differs from the unknown-code 404:\n %s\n %s", w.Body.String(), unknown.Body.String())
	}
	m.capQuyen(t, "feedback.create", "feedback.read", QuyenHanChe)
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, staffScenePhotosPath(maPhieuCanBo), canBoCuaXa(xaA), photoFile()),
		http.StatusCreated)
}

func TestStaffScenePhoto_RefusalMapping(t *testing.T) {
	wrapped := func(e error) error { return fmt.Errorf("x cho xã %s: %w", xaA, e) }
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"not staff-booked":  {wrapped(app.ErrScenePhotoNotStaffBooked), http.StatusConflict, "petition_not_staff_booked"},
		"window closed":     {wrapped(app.ErrPhotoWindowClosed), http.StatusConflict, "petition_state"},
		"count reached":     {wrapped(app.ErrPhotoCountReached), http.StatusConflict, "photo_limit"},
		"type not allowed":  {app.ErrPhotoTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		"too large":         {app.ErrPhotoTooLarge, http.StatusRequestEntityTooLarge, "file_too_large"},
		"malware":           {&app.AttachmentRejection{Reason: app.RejectMalware}, http.StatusUnprocessableEntity, "photo_rejected"},
		"not configured":    {app.ErrUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		"scanner down":      {wrapped(app.ErrScanUnavailable), http.StatusServiceUnavailable, "malware_scan_unavailable"},
		"restricted":        {wrapped(app.ErrPhieuHanChe), http.StatusNotFound, ""},
		"another's file id": {app.ErrPhotoNotFound, http.StatusNotFound, "not_found"},
		"system":            {errors.New("kho hỏng"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "feedback.create")
			m.staffScenePhotos.err = c.err
			w := m.goiGhiNV(t, http.MethodPost, hostA, staffScenePhotosPath(maPhieuThuong), canBoCuaXa(xaA), photoFile())
			doiMa(t, w, c.status)
			if c.code != "" {
				if e := loiTra(t, w); e.Code != c.code {
					t.Errorf("code = %q, want %q", e.Code, c.code)
				}
			}
			for _, banned := range []string{string(xaA), "kho hỏng", maPhieuThuong} {
				if strings.Contains(w.Body.String(), banned) {
					t.Errorf("body leaks %q: %s", banned, w.Body.String())
				}
			}
		})
	}
}
