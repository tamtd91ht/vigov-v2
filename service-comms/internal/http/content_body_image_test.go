package http

// The body-image routes and fields (content_body_image.go, ADR 0067 §Sửa đổi 03/10/2026). The four-case
// permission suite for the upload route runs in noi_dung_mini_app_test.go's table (cacTuyenND), through the
// REAL Register behind the REAL edge chain; this file holds the contract details: the reply, the error
// codes, the staff `body_images`, and the public resolver.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const (
	pathBodyImages = "/api/v1/content-items/body-images"

	bodyImgA = "01JBBBBBBBBBBBBBBBBBBBBBBB"
	bodyImgB = "01JCCCCCCCCCCCCCCCCCCCCCCC"
)

var bodyBodyImageUploadOK = multipartBody([][2]string{sizeOf(fileBytes), {"file_name", "anh-hien-truong.jpg"},
	{"content_type", "image/jpeg"}}, fileBytes)

func figureOf(id string) string {
	return `<figure><img data-file-id="` + id + `" alt="Ảnh"><figcaption>Chú thích</figcaption></figure>`
}

func TestBodyImageUploadAnswersTheStoredImageWithItsPreviewAndReservedItem(t *testing.T) {
	const fileID = "01JBODYFILE000000000000000"
	exp := time.Date(2026, 10, 3, 9, 15, 0, 0, time.UTC)

	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.covers.bodyView = map[string]app.BodyImageView{fileID: {FileID: fileID, Status: domain.StoredFileReady,
		PreviewURL: storage.PresignedURL("https://minio.example/signed-new"), PreviewExpiresAt: exp}}
	w := m.goiVoi(t, http.MethodPost, hostA, pathBodyImages, bodyBodyImageUploadOK, multipartCT, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	var out bodyImageFileOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if out.ID != fileID || out.Status != "ready" || out.ContentItemID != "01JRESERVEDITEM00000000000" ||
		out.PreviewURL != "https://minio.example/signed-new" || out.PreviewExpiresAt == nil || !out.PreviewExpiresAt.Equal(exp) {
		t.Fatalf("reply = %+v", out)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("a reply carrying a presigned preview must be no-store")
	}
	// The file name is personal data: never echoed. And never an upload form any more.
	for _, gone := range []string{"anh-hien-truong", `"upload"`} {
		if strings.Contains(w.Body.String(), gone) {
			t.Errorf("the reply carries %s: %s", gone, w.Body.String())
		}
	}
	// RULE 6, INVARIANT 8: the actor is the staff BUSINESS CODE from the session, never the id.
	if m.covers.actor.ID != canBo(xaA).Ma || m.covers.lastRequest.Size != int64(len(fileBytes)) ||
		string(m.covers.received) != string(fileBytes) {
		t.Errorf("use case got actor=%q req=%+v", m.covers.actor.ID, m.covers.lastRequest)
	}
	// The preview is asked for THIS file of ITS article, in the commune of the Host.
	if m.covers.lastID != "01JRESERVEDITEM00000000000" || len(m.covers.askedBodyIDs) != 1 ||
		m.covers.askedBodyIDs[0] != fileID || m.covers.tenantID != xaA {
		t.Errorf("previews asked for item %q files %v in %q", m.covers.lastID, m.covers.askedBodyIDs, m.covers.tenantID)
	}

	// A later upload of the same unsaved article names it. A FRESH harness: the harness sends one fixed
	// Idempotency-Key, and a second POST under it is the first one's replay, never the handler.
	m = dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w = m.goiVoi(t, http.MethodPost, hostA, pathBodyImages, multipartBody([][2]string{sizeOf(fileBytes),
		{"content_item_id", "01JRESERVEDITEM00000000000"}}, fileBytes), multipartCT, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	if m.covers.lastRequest.ContentItemID != "01JRESERVEDITEM00000000000" {
		t.Errorf("content_item_id did not reach the use case: %+v", m.covers.lastRequest)
	}
}

func TestBodyImageUploadNotReadyCarriesNoPreview(t *testing.T) {
	const fileID = "01JBODYFILE000000000000000"
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	m.covers.bodyCompleteStatus = domain.StoredFileProcessing
	m.covers.bodyView = map[string]app.BodyImageView{fileID: {FileID: fileID,
		PreviewURL: storage.PresignedURL("https://minio.example/never")}}
	w := m.goiVoi(t, http.MethodPost, hostA, pathBodyImages, bodyBodyImageUploadOK, multipartCT, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	if strings.Contains(w.Body.String(), "preview_url") || m.covers.bodyViews != 0 {
		t.Errorf("not ready, yet a preview: %s (views %d)", w.Body.String(), m.covers.bodyViews)
	}
}

func TestBodyImageErrorsMapToTheirStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"21st image", app.ErrCoverCountReached, http.StatusConflict, "body_image_limit"},
		{"not a live article nor own reservation", commsstore.ErrNoiDungKhongTonTai, http.StatusNotFound, "not_found"},
		{"declared type not allowed", app.ErrCoverTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		{"infected", &app.CoverRejection{Reason: app.CoverRejectMalware}, http.StatusUnprocessableEntity, "body_image_rejected"},
		{"over count at completion", &app.CoverRejection{Reason: app.CoverRejectCountReached}, http.StatusUnprocessableEntity, "body_image_rejected"},
		{"scanner down", app.ErrCoverScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{"not configured", app.ErrCoverUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{"moved under the completion", app.ErrCoverNotPending, http.StatusConflict, "body_image_state"},
		{"temp bucket down", fmt.Errorf("%w: storage: unreachable", app.ErrCoverUploadIncomplete),
			http.StatusServiceUnavailable, "upload_store_unavailable"},
		{"client stream cut", fmt.Errorf("%w: %w", app.ErrCoverUploadIncomplete, httpx.ErrUploadMalformed),
			http.StatusBadRequest, "invalid_upload"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.covers.err = tc.err
			w := m.goiVoi(t, http.MethodPost, hostA, pathBodyImages, bodyBodyImageUploadOK, multipartCT, canBo(xaA))
			doiMa(t, w, tc.want)
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Errorf("code: %s", w.Body.String())
			}
			if strings.Contains(w.Body.String(), "ảnh bìa") {
				t.Errorf("a body-image refusal speaks of the cover: %s", w.Body.String())
			}
		})
	}
}

func TestSavingAForeignBodyImageIs422WithOneCode(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.ghi.loi = domain.ErrBodyImageNotUsable
			path, body := duongNoiDung, `{"type":"tin-tuc","title":"Tin","body":"`+strings.ReplaceAll(figureOf(bodyImgA), `"`, `\"`)+`"}`
			if method == http.MethodPatch {
				path, body = duongNoiDung+"/nd-001", `{"body":"`+strings.ReplaceAll(figureOf(bodyImgA), `"`, `\"`)+`"}`
			}
			w := m.goi(t, method, hostA, path, body, canBo(xaA))
			doiMa(t, w, http.StatusUnprocessableEntity)
			if !strings.Contains(w.Body.String(), `"code":"invalid_body_image"`) || strings.Contains(w.Body.String(), bodyImgA) {
				t.Errorf("body: %s", w.Body.String())
			}
		})
	}
}

func TestContentDetailCarriesBodyImagePreviewsInBodyOrder(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	n := noiDungMau()
	n.AnhDaiDienURL, n.CoverImageFileID = "", ""
	n.NoiDung = "<p>Mở đầu</p>" + figureOf(bodyImgB) + "<p>giữa</p>" + figureOf(bodyImgA) + figureOf(bodyImgB)
	m.so.mot = n
	exp := time.Date(2026, 10, 3, 9, 15, 0, 0, time.UTC)
	m.covers.bodyView = map[string]app.BodyImageView{
		bodyImgB: {FileID: bodyImgB, Status: domain.StoredFileReady, Public: true,
			PreviewURL: storage.PresignedURL("https://minio.example/signed-b"), PreviewExpiresAt: exp},
	}

	w := m.goi(t, http.MethodGet, hostA, duongNoiDung+"/nd-001", "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	var got noiDungRa
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if len(got.BodyImages) != 2 || got.BodyImages[0].FileID != bodyImgB || got.BodyImages[1].FileID != bodyImgA {
		t.Fatalf("body_images = %+v, want [B, A] — distinct, body order", got.BodyImages)
	}
	if got.BodyImages[0].PreviewURL != "https://minio.example/signed-b" || !got.BodyImages[0].Public ||
		got.BodyImages[0].PreviewExpiresAt == nil {
		t.Errorf("B = %+v", got.BodyImages[0])
	}
	if got.BodyImages[1].PreviewURL != "" {
		t.Errorf("A has no ready file, yet carries a preview: %+v", got.BodyImages[1])
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("a reply carrying presigned previews must be no-store")
	}
	if m.covers.tenantID != xaA || m.covers.lastID != n.ID {
		t.Errorf("previews read in commune %q for item %q", m.covers.tenantID, m.covers.lastID)
	}
}

func TestContentListAndPlainBodyAskForNoBodyImagePreview(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	n := noiDungMau()
	n.NoiDung = figureOf(bodyImgA)
	m.so.ra.Items = append(m.so.ra.Items, n)
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongNoiDung, "", canBo(xaA)), http.StatusOK)

	m.so.mot = noiDungMau() // `<p>Toàn văn</p>`: no image
	w := m.goi(t, http.MethodGet, hostA, duongNoiDung+"/nd-001", "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	if m.covers.bodyViews != 0 || strings.Contains(w.Body.String(), `"body_images"`) {
		t.Errorf("views=%d body=%s — the list and an image-less body sign nothing", m.covers.bodyViews, w.Body.String())
	}
}

// --- the public detail resolver (K2/K7) ------------------------------------------------------------------

func TestPublicDetailResolvesOnlyThisArticlesPublishedBodyImagesInThisCommune(t *testing.T) {
	nd, dm := ckDuLieu()
	items := nd.theoXa[xaA]
	items[0].NoiDung = "<p>Đầu</p>" + figureOf(bodyImgA) + figureOf(bodyImgB) + "<p>Cuối</p>"
	nd.theoXa[xaA] = items
	const urlA = "https://media.example/vigov-prod-public/public-media/t_a/2026/10/comms/content-body-image/a/thumb-1280.jpg"
	covers := &fakePublicCovers{bodies: map[tenant.ID]map[string]map[string]string{
		// B is not published in xã A — but IS, under the same ids, in xã B and on another article of xã A.
		xaA: {"nd-a-1": {bodyImgA: urlA}, "nd-a-khac": {bodyImgB: "https://media.example/khac.jpg"}},
		xaB: {"nd-a-1": {bodyImgB: "https://media.example/xa-b.jpg"}},
	}}
	h := newPublicCoverServer(t, nd, dm, covers)

	w := ckGoi(h, MauTinXa+"/nd-a-1", ckHostA)
	doiMa(t, w, http.StatusOK)
	var one struct {
		BodyBlocks []map[string]any `json:"body_blocks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &one); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	var images []string
	for _, b := range one.BodyBlocks {
		if b["kind"] == "image" {
			images = append(images, b["src"].(string))
		}
	}
	if len(images) != 1 || images[0] != urlA {
		t.Fatalf("image srcs = %v, want only [%s]", images, urlA)
	}
	if strings.Contains(w.Body.String(), bodyImgA) || strings.Contains(w.Body.String(), "data-file-id") {
		t.Errorf("a file id reached the resident: %s", w.Body.String())
	}
	if covers.tenantID != xaA || len(covers.askedBodies) != 1 || covers.askedBodies[0] != "nd-a-1" {
		t.Errorf("resolved in commune %q for %v", covers.tenantID, covers.askedBodies)
	}
	ckKhongCoHTML(t, w)
}

func TestPublicListNeverAsksForBodyImages(t *testing.T) {
	nd, dm := ckDuLieu()
	covers := &fakePublicCovers{}
	h := newPublicCoverServer(t, nd, dm, covers)
	doiMa(t, ckGoi(h, MauTinXa, ckHostA), http.StatusOK)
	if len(covers.askedBodies) != 0 {
		t.Errorf("the list asked for body images of %v", covers.askedBodies)
	}
}
