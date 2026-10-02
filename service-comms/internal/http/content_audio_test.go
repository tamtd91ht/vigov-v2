package http

// The broadcast audio routes (content_audio.go) and the audio fields on the staff and public reads.
// The four-case permission suite for both new routes runs in noi_dung_mini_app_test.go's table
// (cacTuyenND), through the REAL Register behind the REAL edge chain.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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

type fakeAudio struct {
	requests, completions, views int
	tenantID                     tenant.ID
	actor                        audit.Actor
	lastRequest                  app.AudioUploadRequest
	lastID                       string
	lastDuration                 int
	view                         app.AudioView
	err                          error
}

func (f *fakeAudio) RequestUpload(ctx context.Context, req app.AudioUploadRequest, actor audit.Actor) (
	app.AudioUpload, error) {
	f.requests++
	f.tenantID, f.actor, f.lastRequest = tenant.MustFrom(ctx), actor, req
	if f.err != nil {
		return app.AudioUpload{}, f.err
	}
	return app.AudioUpload{
		File: domain.StoredFile{ID: "01JAUDIOFILE00000000000000", Status: domain.StoredFilePending,
			SubjectID: req.ContentItemID, OriginalName: "ban-tin-sang.mp3"},
		Post: storage.PresignedPost{URL: "https://minio.example/vigov-prod-temp",
			Fields: map[string]string{"key": "upload/x"}, ExpiresAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)},
	}, nil
}

func (f *fakeAudio) Complete(ctx context.Context, id string, durationSeconds int, actor audit.Actor) (
	app.AudioCompletion, error) {
	f.completions++
	f.tenantID, f.actor, f.lastID, f.lastDuration = tenant.MustFrom(ctx), actor, id, durationSeconds
	if f.err != nil {
		return app.AudioCompletion{}, f.err
	}
	return app.AudioCompletion{
		File: domain.StoredFile{ID: id, SubjectID: "nd-tt-1", Status: domain.StoredFileReady,
			MIMEType: "audio/mpeg", SizeBytes: 4096},
		Item: domain.NoiDungMiniApp{ID: "nd-tt-1", Loai: domain.LoaiTruyenThanh, AudioFileID: id,
			AudioDurationSeconds: durationSeconds},
	}, nil
}

func (f *fakeAudio) View(ctx context.Context, fileID string) (app.AudioView, error) {
	f.views++
	f.tenantID = tenant.MustFrom(ctx)
	return f.view, nil
}

// fakePublicAudio signs, per commune, the ids it holds — and records what it was asked.
type fakePublicAudio struct {
	byTenant map[tenant.ID]map[string]app.PublicAudio
	asked    []string
	tenantID tenant.ID
	err      error
}

func (a *fakePublicAudio) PublicAudioURLs(ctx context.Context, ids []string) (map[string]app.PublicAudio, error) {
	a.tenantID = tenant.MustFrom(ctx)
	a.asked = append(a.asked, ids...)
	if a.err != nil {
		return nil, a.err
	}
	out := map[string]app.PublicAudio{}
	for _, id := range ids {
		if u, ok := a.byTenant[a.tenantID][id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

const (
	pathAudioFiles        = "/api/v1/content-items/audio-files"
	bodyAudioUploadOK     = `{"file_name":"ban-tin-sang.mp3","content_type":"audio/mpeg","size":2048,"content_item_id":"nd-tt-1"}`
	pathAudioCompletion   = "/api/v1/content-items/audio-files/01JAUDIOFILE00000000000000/completion"
	bodyAudioCompletionOK = `{"audio_duration_seconds":754}`
)

func TestAudioUploadReplyCarriesTheFormAndNoStore(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goi(t, http.MethodPost, hostA, pathAudioFiles, bodyAudioUploadOK, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("a presigned form must not be cached: Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	var out audioUploadOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if out.Upload.URL == "" || out.AudioFile.Status != "pending" || out.AudioFile.ContentItemID != "nd-tt-1" {
		t.Errorf("reply = %+v", out)
	}
	if strings.Contains(w.Body.String(), "ban-tin-sang") {
		t.Error("the reply echoes the file name")
	}
	// RULE 6, INVARIANT 8: the actor is the staff BUSINESS CODE from the session, never the id.
	if m.audio.actor.ID != canBo(xaA).Ma || m.audio.lastRequest.ContentItemID != "nd-tt-1" ||
		m.audio.lastRequest.ContentType != "audio/mpeg" || m.audio.lastRequest.Size != 2048 {
		t.Errorf("use case got actor=%q req=%+v", m.audio.actor.ID, m.audio.lastRequest)
	}
}

func TestAudioCompletionPassesTheTypedDuration(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goi(t, http.MethodPost, hostA, pathAudioCompletion, bodyAudioCompletionOK, canBo(xaA))
	doiMa(t, w, http.StatusOK)
	if m.audio.lastDuration != 754 || m.audio.lastID != "01JAUDIOFILE00000000000000" {
		t.Errorf("use case got id=%q duration=%d", m.audio.lastID, m.audio.lastDuration)
	}
	var out audioFileOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if out.Status != "ready" || out.DurationSeconds != 754 || out.MIMEType != "audio/mpeg" || out.SizeBytes != 4096 {
		t.Errorf("reply = %+v", out)
	}
}

func TestAudioErrorsMapToTheirStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"declared too large (policy)", app.ErrAudioTooLarge, http.StatusBadRequest, "invalid_request"},
		{"declared type not allowed", app.ErrAudioTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		{"no item named", app.ErrAudioItemRequired, http.StatusBadRequest, "invalid_request"},
		{"infected", &app.AudioRejection{Reason: app.AudioRejectMalware}, http.StatusUnprocessableEntity, "audio_rejected"},
		{"not audio", &app.AudioRejection{Reason: app.AudioRejectNotAudio}, http.StatusUnprocessableEntity, "audio_rejected"},
		{"duration out of range", domain.ErrAudioDurationInvalid, http.StatusUnprocessableEntity, "invalid_audio_duration"},
		{"item not truyen-thanh", domain.ErrAudioOnlyForBroadcast, http.StatusUnprocessableEntity, "audio_only_for_truyen_thanh"},
		{"already has audio", app.ErrAudioCountReached, http.StatusConflict, "audio_limit"},
		{"scanner down", app.ErrAudioScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{"not configured", app.ErrAudioUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{"limits unavailable", app.ErrAudioLimitsUnavailable, http.StatusServiceUnavailable, "upload_limits_unavailable"},
		{"not the caller's", app.ErrAudioFileNotFound, http.StatusNotFound, "not_found"},
		{"not received", app.ErrAudioUploadNotReceived, http.StatusConflict, "upload_not_received"},
		{"expired", app.ErrAudioUploadExpired, http.StatusConflict, "upload_expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.audio.err = tc.err
			w := m.goi(t, http.MethodPost, hostA, pathAudioCompletion, bodyAudioCompletionOK, canBo(xaA))
			doiMa(t, w, tc.want)
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Errorf("code: %s", w.Body.String())
			}
		})
	}
}

func TestContentDetailCarriesAudioBlock(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	n := noiDungMau()
	n.Loai, n.AudioFileID, n.AudioDurationSeconds = domain.LoaiTruyenThanh, "01JAUDIOFILE00000000000000", 754
	m.so.mot = n
	exp := time.Date(2026, 10, 1, 9, 15, 0, 0, time.UTC)
	m.audio.view = app.AudioView{FileID: n.AudioFileID, Status: domain.StoredFileReady, MIMEType: "audio/mp4",
		SizeBytes: 5_000_000, PreviewURL: storage.PresignedURL("https://minio.example/signed-audio"), PreviewExpiresAt: exp}

	w := m.goi(t, http.MethodGet, hostA, duongNoiDung+"/nd-001", "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	var got noiDungRa
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	a := got.Audio
	if a == nil || a.FileID != n.AudioFileID || a.Status != "ready" || a.MIMEType != "audio/mp4" ||
		a.SizeBytes != 5_000_000 || a.DurationSeconds != 754 || a.PreviewURL != "https://minio.example/signed-audio" {
		t.Fatalf("audio block = %+v", a)
	}
	if got.AudioFileID != n.AudioFileID || got.AudioDurationSeconds != 754 {
		t.Errorf("audio fields = %q %d", got.AudioFileID, got.AudioDurationSeconds)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("a reply carrying a presigned listening link must be no-store")
	}
	if m.audio.tenantID != xaA {
		t.Errorf("audio view read in commune %q, want %q", m.audio.tenantID, xaA)
	}
}

func TestContentListCarriesAudioFieldsButNoLink(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	n := noiDungMau()
	n.Loai, n.AudioFileID, n.AudioDurationSeconds = domain.LoaiTruyenThanh, "01JAUDIOFILE00000000000000", 754
	m.so.ra.Items = append(m.so.ra.Items, n)
	w := m.goi(t, http.MethodGet, hostA, duongNoiDung, "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	if !strings.Contains(body, `"audio_file_id":"01JAUDIOFILE00000000000000"`) ||
		!strings.Contains(body, `"audio_duration_seconds":754`) || strings.Contains(body, `"audio":`) {
		t.Errorf("list: %s", body)
	}
	if m.audio.views != 0 {
		t.Error("the list must not sign a listening link per row")
	}
}

func TestPatchPassesAudioRemovalAndDuration(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"audio_file_id":""}`,
		canBo(xaA)), http.StatusOK)
	if p := m.ghi.suaCuoi.AudioFileID; p == nil || *p != "" {
		t.Errorf("removal must reach the use case as a pointer to \"\": %v", p)
	}
	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"audio_duration_seconds":300}`,
		canBo(xaA)), http.StatusOK)
	if p := m.ghi.suaCuoi.AudioDurationSeconds; p == nil || *p != 300 || m.ghi.suaCuoi.AudioFileID != nil {
		t.Errorf("duration edit reached the use case as %v / %v", p, m.ghi.suaCuoi.AudioFileID)
	}
}

func TestPatchAudioRefusalsMapTo4xx(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
		code string
	}{
		{domain.ErrAudioNotUsable, http.StatusConflict, "audio_not_usable"},
		{domain.ErrAudioAllOrNone, http.StatusUnprocessableEntity, "audio_all_or_none"},
		{domain.ErrAudioOnlyForBroadcast, http.StatusUnprocessableEntity, "audio_only_for_truyen_thanh"},
		{domain.ErrAudioDurationInvalid, http.StatusUnprocessableEntity, "invalid_audio_duration"},
		{domain.ErrAudioFileIDInvalid, http.StatusBadRequest, "invalid_request"},
	} {
		m := dungMayChuND(t)
		m.capQuyen(xaA, QuyenSuaNoiDung)
		m.ghi.loi = tc.err
		w := m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"audio_file_id":"01JOTHER"}`, canBo(xaA))
		doiMa(t, w, tc.want)
		if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
			t.Errorf("%v: body %s", tc.err, w.Body.String())
		}
	}
}

// --- the public surface ------------------------------------------------------------------------------

const publicAudioURL = "https://minio.example/vigov-prod-private/content-source/t_x/2026/10/comms/content-audio/y/original.mp3?X-Amz-Signature=abc"

func publicAudioFixture() (*ckNoiDung, *ckDanhMuc, *fakePublicAudio, time.Time) {
	nd, dm := ckDuLieu()
	items := nd.theoXa[xaA]
	items[0].Loai, items[0].AudioFileID, items[0].AudioDurationSeconds = domain.LoaiTruyenThanh, "AUDIOPUBLISHED", 754
	// A DRAFT broadcast with audio: its id must never even be asked about on the public surface.
	items[1].Loai, items[1].AudioFileID, items[1].AudioDurationSeconds = domain.LoaiTruyenThanh, "AUDIODRAFT", 60
	nd.theoXa[xaA] = items
	exp := time.Date(2026, 10, 1, 9, 15, 0, 0, time.UTC)
	audio := &fakePublicAudio{byTenant: map[tenant.ID]map[string]app.PublicAudio{
		xaA: {"AUDIOPUBLISHED": {URL: publicAudioURL, ExpiresAt: exp},
			"AUDIODRAFT": {URL: "https://minio.example/draft.mp3", ExpiresAt: exp}},
	}}
	return nd, dm, audio, exp
}

func newPublicAudioServer(t *testing.T, nd *ckNoiDung, dm *ckDanhMuc, audio *fakePublicAudio) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Limiter: ckLimiter(), Xa: &ckNenTang{}, NoiDung: nd, Views: nd, DanhMuc: dm, CoverImages: &fakePublicCovers{},
		Audio: audio, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return mux
}

func TestPublicNewsAudioURLOnlyForPublishedBroadcasts(t *testing.T) {
	nd, dm, audio, exp := publicAudioFixture()
	h := newPublicAudioServer(t, nd, dm, audio)

	w := ckGoi(h, MauTinXa, ckHostA)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	got := ckDocTrang(t, w)
	if len(got.Items) != 1 || got.Items[0]["audio_url"] != publicAudioURL ||
		got.Items[0]["audio_duration_seconds"] != float64(754) ||
		got.Items[0]["audio_url_expires_at"] != exp.Format(time.RFC3339) {
		t.Fatalf("the published broadcast must carry its signed link and duration: %v", got.Items)
	}
	for _, id := range audio.asked {
		if id == "AUDIODRAFT" {
			t.Fatal("an UNPUBLISHED broadcast's audio was signed on the public surface")
		}
	}
	if audio.tenantID != xaA {
		t.Errorf("audio signed in commune %q, want %q", audio.tenantID, xaA)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("a public reply carrying a signed link must be no-store")
	}
	ckKhongCoHTML(t, w)
}

func TestPublicNewsDetailAudioAndOmittedWithoutLink(t *testing.T) {
	nd, dm, audio, _ := publicAudioFixture()
	h := newPublicAudioServer(t, nd, dm, audio)

	w := ckGoi(h, MauTinXa+"/nd-a-1", ckHostA)
	var one map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &one); err != nil || w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	if one["audio_url"] != publicAudioURL || one["audio_duration_seconds"] != float64(754) {
		t.Errorf("detail must carry the audio: %v", one)
	}

	// Not ready / storage not configured: no link — and then no duration either, and the reply may be cached.
	audio.byTenant[xaA] = map[string]app.PublicAudio{}
	w = ckGoi(h, MauTinXa+"/nd-a-1", ckHostA)
	one = map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &one)
	for _, k := range []string{"audio_url", "audio_duration_seconds", "audio_url_expires_at"} {
		if _, ok := one[k]; ok {
			t.Errorf("no signed link, yet %s is present: %s", k, w.Body.String())
		}
	}
	if w.Header().Get("Cache-Control") == "no-store" {
		t.Error("no link signed, so nothing forces no-store")
	}
}

func TestPublicNewsAudioReadFailureIs500(t *testing.T) {
	nd, dm, audio, _ := publicAudioFixture()
	audio.err = errors.New("store down")
	h := newPublicAudioServer(t, nd, dm, audio)
	if w := ckGoi(h, MauTinXa, ckHostA); w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestAudioIDsOnlyPublishedBroadcasts(t *testing.T) {
	ids := audioIDs([]domain.NoiDungMiniApp{
		{Loai: domain.LoaiTruyenThanh, AudioFileID: "A", TrangThai: domain.TrangThaiDangHien},
		{Loai: domain.LoaiTruyenThanh, AudioFileID: "A", TrangThai: domain.TrangThaiDangHien},
		{Loai: domain.LoaiTruyenThanh, AudioFileID: "B", TrangThai: domain.TrangThaiAn},
		{Loai: domain.LoaiTruyenThanh, AudioFileID: "C", TrangThai: domain.TrangThaiChoDuyet},
		// A non-broadcast row carrying an id (impossible under 0012's CHECK): never signed.
		{Loai: domain.LoaiTinTuc, AudioFileID: "D", TrangThai: domain.TrangThaiDangHien},
		{Loai: domain.LoaiTruyenThanh, TrangThai: domain.TrangThaiDangHien},
	})
	if len(ids) != 1 || ids[0] != "A" {
		t.Errorf("audioIDs = %v, want [A]", ids)
	}
}
