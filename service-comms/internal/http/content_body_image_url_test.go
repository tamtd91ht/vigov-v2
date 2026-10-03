package http

// POST /api/v1/content-items/body-images/from-url (content_body_image.go, ADR 0067 §Sửa đổi 03/10/2026,
// H5, K6, K8). The four-case permission suite runs in noi_dung_mini_app_test.go's table (cacTuyenND),
// through the REAL Register behind the REAL edge chain; this file holds the reply, the error codes, and
// what the log line may and may not carry.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const (
	pathBodyImageFromURL = "/api/v1/content-items/body-images/from-url"
	// The query and path carry markers a leak would be found by.
	pastedURL              = "https://img.news.example.com/2026/PATHMARK.jpg?token=QUERYMARK"
	bodyBodyImageFromURLOK = `{"url":"` + pastedURL + `"}`
)

func TestBodyImageFromURLReplyIsAReadyImageWithPreviewAndReservedItem(t *testing.T) {
	const fileID = "01JFETCHEDFILE000000000000"
	exp := time.Date(2026, 10, 3, 9, 15, 0, 0, time.UTC)
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.covers.bodyView = map[string]app.BodyImageView{fileID: {FileID: fileID, Status: domain.StoredFileReady,
		PreviewURL: storage.PresignedURL("https://minio.example/signed-fetched"), PreviewExpiresAt: exp}}
	w := m.goi(t, http.MethodPost, hostA, pathBodyImageFromURL, bodyBodyImageFromURLOK, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	var out bodyImageFileOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if out.ID != fileID || out.Status != "ready" || out.ContentItemID != "01JRESERVEDITEM00000000000" ||
		out.PreviewURL != "https://minio.example/signed-fetched" || out.PreviewExpiresAt == nil {
		t.Fatalf("reply = %+v", out)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("a reply carrying a presigned preview must be no-store")
	}
	// The URL reaches the use case untouched; the actor is the BUSINESS CODE (rule 6, invariant 8).
	if m.covers.lastFetch.URL != pastedURL || m.covers.actor.ID != canBo(xaA).Ma || m.covers.tenantID != xaA {
		t.Errorf("use case got %+v actor=%q xa=%q", m.covers.lastFetch, m.covers.actor.ID, m.covers.tenantID)
	}
	// Never echoed (rule 3).
	if strings.Contains(w.Body.String(), "PATHMARK") || strings.Contains(w.Body.String(), "QUERYMARK") {
		t.Errorf("the reply echoes the pasted URL: %s", w.Body.String())
	}
	logs := m.logs.String()
	if !strings.Contains(logs, "host=img.news.example.com") || !strings.Contains(logs, "outcome=stored") {
		t.Errorf("the log line must name the host and the outcome: %s", logs)
	}
	if strings.Contains(logs, "PATHMARK") || strings.Contains(logs, "QUERYMARK") {
		t.Errorf("the log carries the URL's path or query: %s", logs)
	}

	// A named article reaches the use case.
	m = dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w = m.goi(t, http.MethodPost, hostA, pathBodyImageFromURL,
		`{"url":"`+pastedURL+`","content_item_id":"01JRESERVEDITEM00000000000"}`, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	if m.covers.lastFetch.ContentItemID != "01JRESERVEDITEM00000000000" {
		t.Errorf("content_item_id did not reach the use case: %+v", m.covers.lastFetch)
	}
}

func TestBodyImageFromURLErrorsMapToTheirStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"bad shape", app.ErrImageURLInvalid, http.StatusBadRequest, "invalid_image_url"},
		{"refused address", &app.ImageFetchError{Class: "address-refused", Refused: true}, http.StatusBadGateway, "image_fetch_failed"},
		{"dns", &app.ImageFetchError{Class: "dns"}, http.StatusBadGateway, "image_fetch_failed"},
		{"non-200", &app.ImageFetchError{Class: "http-404"}, http.StatusBadGateway, "image_fetch_failed"},
		{"too many redirects", &app.ImageFetchError{Class: "too-many-redirects", Refused: true}, http.StatusBadGateway, "image_fetch_failed"},
		{"21st image", app.ErrCoverCountReached, http.StatusConflict, "body_image_limit"},
		{"not an image", &app.CoverRejection{Reason: app.CoverRejectTypeNotAllowed}, http.StatusUnprocessableEntity, "body_image_rejected"},
		{"too big", &app.CoverRejection{Reason: app.CoverRejectTooLarge}, http.StatusUnprocessableEntity, "body_image_rejected"},
		{"infected", &app.CoverRejection{Reason: app.CoverRejectMalware}, http.StatusUnprocessableEntity, "body_image_rejected"},
		{"undecodable", &app.CoverRejection{Reason: app.CoverRejectUndecodable}, http.StatusUnprocessableEntity, "body_image_rejected"},
		{"unknown article", commsstore.ErrNoiDungKhongTonTai, http.StatusNotFound, "not_found"},
		{"scanner down", app.ErrCoverScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{"not configured", app.ErrCoverUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{"limits unavailable", app.ErrCoverLimitsUnavailable, http.StatusServiceUnavailable, "upload_limits_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.covers.err = tc.err
			w := m.goi(t, http.MethodPost, hostA, pathBodyImageFromURL, bodyBodyImageFromURLOK, canBo(xaA))
			doiMa(t, w, tc.want)
			body := w.Body.String()
			if !strings.Contains(body, `"code":"`+tc.code+`"`) {
				t.Errorf("code: %s", body)
			}
			// One sentence, no URL, no remote class (a caller must not map internal addresses).
			for _, leak := range []string{"PATHMARK", "QUERYMARK", "img.news", "address-refused", "dns", "http-404", "ảnh bìa"} {
				if strings.Contains(body, leak) {
					t.Errorf("the reply carries %q: %s", leak, body)
				}
			}
			if logs := m.logs.String(); strings.Contains(logs, "PATHMARK") || strings.Contains(logs, "QUERYMARK") {
				t.Errorf("the log carries the URL's path or query: %s", logs)
			}
		})
	}
}

func TestBodyImageFromURLRefusalIsTheSecurityEvent(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.covers.err = &app.ImageFetchError{Class: "address-refused", Refused: true}
	m.goi(t, http.MethodPost, hostA, pathBodyImageFromURL, bodyBodyImageFromURLOK, canBo(xaA))
	logs := m.logs.String()
	for _, want := range []string{"event=outbound_url_refused", "class=address-refused", "host=img.news.example.com",
		"actor=" + canBo(xaA).Ma} {
		if !strings.Contains(logs, want) {
			t.Errorf("security event lacks %q: %s", want, logs)
		}
	}
}
