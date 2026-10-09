package http

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
)

// Every refusal of the cover and audio upload routes is logged with its code and the cause (09/10/2026:
// a missing temp bucket answered with nothing in the log). INFO and `ma_loi`; never a client value.
func TestUploadRefusalsAreLogged(t *testing.T) {
	for _, c := range []struct {
		name   string
		answer func(h *Handler, w http.ResponseWriter, r *http.Request)
		status int
		code   string
	}{
		{"cover", func(h *Handler, w http.ResponseWriter, r *http.Request) {
			h.answerCoverError(w, r, "tải ảnh bìa", app.ErrCoverCountReached)
		}, http.StatusConflict, "cover_limit"},
		{"audio", func(h *Handler, w http.ResponseWriter, r *http.Request) {
			h.answerAudioError(w, r, "tải âm thanh", &app.AudioRejection{Reason: app.AudioRejectNotAudio})
		}, http.StatusUnprocessableEntity, "audio_rejected"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := &Handler{d: Deps{Log: slog.New(slog.NewTextHandler(&buf, nil))}}
			r := httptest.NewRequest(http.MethodPost, "/x", nil)
			r = r.WithContext(tenant.Into(context.Background(), xaA))
			w := httptest.NewRecorder()
			c.answer(h, w, r)
			if w.Code != c.status {
				t.Fatalf("status = %d, want %d", w.Code, c.status)
			}
			for _, want := range []string{"level=INFO", "ma_loi=" + c.code, "xa=" + string(xaA)} {
				if !strings.Contains(buf.String(), want) {
					t.Errorf("log lacks %q: %s", want, buf.String())
				}
			}
		})
	}
}

// A temp bucket that refuses the write is the operator's line: WARN, with the cause, 503.
func TestUploadStoreFailureIsAWarningWithItsCause(t *testing.T) {
	var buf bytes.Buffer
	h := &Handler{d: Deps{Log: slog.New(slog.NewTextHandler(&buf, nil))}}
	r := httptest.NewRequest(http.MethodPost, "/x", nil).WithContext(tenant.Into(context.Background(), xaA))
	w := httptest.NewRecorder()
	h.answerCoverError(w, r, "tải ảnh bìa", app.ErrCoverUploadIncomplete)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	for _, want := range []string{"level=WARN", "ma_loi=upload_store_unavailable", "chưa nhận đủ tệp"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log lacks %q: %s", want, buf.String())
		}
	}
}
