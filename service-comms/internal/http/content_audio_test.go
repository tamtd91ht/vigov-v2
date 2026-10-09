package http

// The broadcast audio routes (content_audio.go) and the audio fields on the staff and public reads.
// The four-case permission suite for the upload route runs in noi_dung_mini_app_test.go's table
// (cacTuyenND), through the REAL Register behind the REAL edge chain.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

type fakeAudio struct {
	requests, views int
	tenantID        tenant.ID
	actor           audit.Actor
	lastRequest     app.AudioUploadRequest
	received        []byte
	view            app.AudioView
	err             error
	limitErr        error
}

func (f *fakeAudio) UploadLimit(context.Context) (int64, error) {
	if f.limitErr != nil {
		return 0, f.limitErr
	}
	return 1 << 20, nil
}

// Upload drains the stream like PutUpload, then calls Finish, like the use case.
func (f *fakeAudio) Upload(ctx context.Context, req app.AudioUploadRequest, actor audit.Actor) (
	app.AudioCompletion, error) {
	f.requests++
	f.tenantID, f.actor, f.lastRequest = tenant.MustFrom(ctx), actor, req
	b, err := io.ReadAll(req.File)
	if err != nil {
		return app.AudioCompletion{}, fmt.Errorf("%w: %w", app.ErrAudioUploadIncomplete, err)
	}
	f.received = b
	if err := req.Finish(); err != nil {
		return app.AudioCompletion{}, fmt.Errorf("%w: %w", app.ErrAudioUploadIncomplete, err)
	}
	if f.err != nil {
		return app.AudioCompletion{}, f.err
	}
	const id = "01JAUDIOFILE00000000000000"
	return app.AudioCompletion{
		File: domain.StoredFile{ID: id, SubjectID: req.ContentItemID, Status: domain.StoredFileReady,
			MIMEType: "audio/mpeg", SizeBytes: req.Size},
		Item: domain.NoiDungMiniApp{ID: req.ContentItemID, Loai: domain.LoaiTruyenThanh, AudioFileID: id,
			AudioDurationSeconds: req.DurationSeconds},
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

const pathAudioFiles = "/api/v1/content-items/audio-files"

var bodyAudioUploadOK = multipartBody([][2]string{sizeOf(fileBytes), {"content_item_id", "nd-tt-1"},
	{"audio_duration_seconds", "754"}, {"file_name", "ban-tin-sang.mp3"}, {"content_type", "audio/mpeg"}}, fileBytes)

func TestAudioUploadStreamsTheFileAndAnswersItAttached(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goiVoi(t, http.MethodPost, hostA, pathAudioFiles, bodyAudioUploadOK, multipartCT, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	var out audioFileOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if out.ID != "01JAUDIOFILE00000000000000" || out.Status != "ready" || out.ContentItemID != "nd-tt-1" ||
		out.DurationSeconds != 754 || out.MIMEType != "audio/mpeg" || out.SizeBytes != int64(len(fileBytes)) {
		t.Errorf("reply = %+v", out)
	}
	for _, gone := range []string{"ban-tin-sang", `"upload"`} {
		if strings.Contains(w.Body.String(), gone) {
			t.Errorf("the reply carries %s: %s", gone, w.Body.String())
		}
	}
	// RULE 6, INVARIANT 8: the actor is the staff BUSINESS CODE from the session, never the id.
	got := m.audio.lastRequest
	if m.audio.actor.ID != canBo(xaA).Ma || got.ContentItemID != "nd-tt-1" || got.ContentType != "audio/mpeg" ||
		got.Size != int64(len(fileBytes)) || got.DurationSeconds != 754 || got.FileName != "ban-tin-sang.mp3" ||
		string(m.audio.received) != string(fileBytes) {
		t.Errorf("use case got actor=%q req=%+v", m.audio.actor.ID, got)
	}
}

// THE DURATION IS A FIELD BEFORE THE FILE, checked before the use case runs: missing, not a whole number
// or out of range is one 422, and nothing is written or read.
func TestAudioUploadDurationIsCheckedBeforeTheUseCase(t *testing.T) {
	for name, fields := range map[string][][2]string{
		"missing":      {sizeOf(fileBytes), {"content_item_id", "nd-tt-1"}},
		"not a number": {sizeOf(fileBytes), {"content_item_id", "nd-tt-1"}, {"audio_duration_seconds", "12 phút"}},
		"zero":         {sizeOf(fileBytes), {"content_item_id", "nd-tt-1"}, {"audio_duration_seconds", "0"}},
		"over 6 hours": {sizeOf(fileBytes), {"content_item_id", "nd-tt-1"}, {"audio_duration_seconds", "21601"}},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			w := m.goiVoi(t, http.MethodPost, hostA, pathAudioFiles, multipartBody(fields, fileBytes), multipartCT, canBo(xaA))
			doiMa(t, w, http.StatusUnprocessableEntity)
			if !strings.Contains(w.Body.String(), `"code":"invalid_audio_duration"`) || m.audio.requests != 0 {
				t.Errorf("body: %s (requests %d)", w.Body.String(), m.audio.requests)
			}
		})
	}
	// SENT AFTER THE FILE it does not count: the handler decides at the file part, before reading it, so
	// the duration is "missing" — the same 422, and the file is never read.
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	body := strings.TrimSuffix(multipartBody([][2]string{sizeOf(fileBytes), {"content_item_id", "nd-tt-1"}}, fileBytes),
		"--"+multipartBoundary+"--\r\n") + multipartBody([][2]string{{"audio_duration_seconds", "60"}}, nil)
	w := m.goiVoi(t, http.MethodPost, hostA, pathAudioFiles, body, multipartCT, canBo(xaA))
	doiMa(t, w, http.StatusUnprocessableEntity)
	if m.audio.requests != 0 {
		t.Error("a duration sent after the file reached the use case")
	}
}

func TestAudioErrorsMapToTheirStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"declared too large (policy)", app.ErrAudioTooLarge, http.StatusRequestEntityTooLarge, "file_too_large"},
		{"declared type not allowed", app.ErrAudioTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		{"no item named", app.ErrAudioItemRequired, http.StatusBadRequest, "invalid_request"},
		{"infected", &app.AudioRejection{Reason: app.AudioRejectMalware}, http.StatusUnprocessableEntity, "audio_rejected"},
		{"not audio", &app.AudioRejection{Reason: app.AudioRejectNotAudio}, http.StatusUnprocessableEntity, "audio_rejected"},
		{"item not truyen-thanh", domain.ErrAudioOnlyForBroadcast, http.StatusUnprocessableEntity, "audio_only_for_truyen_thanh"},
		{"already has audio", app.ErrAudioCountReached, http.StatusConflict, "audio_limit"},
		{"scanner down", app.ErrAudioScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{"not configured", app.ErrAudioUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{"limits unavailable", app.ErrAudioLimitsUnavailable, http.StatusServiceUnavailable, "upload_limits_unavailable"},
		{"no such item", commsstore.ErrNoiDungKhongTonTai, http.StatusNotFound, "not_found"},
		{"temp bucket down", fmt.Errorf("%w: storage: unreachable", app.ErrAudioUploadIncomplete),
			http.StatusServiceUnavailable, "upload_store_unavailable"},
		{"write over the cap", fmt.Errorf("%w: %w", app.ErrAudioUploadIncomplete, storage.ErrTooLarge),
			http.StatusRequestEntityTooLarge, "file_too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.audio.err = tc.err
			w := m.goiVoi(t, http.MethodPost, hostA, pathAudioFiles, bodyAudioUploadOK, multipartCT, canBo(xaA))
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
