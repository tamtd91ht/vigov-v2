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

// Every 409 of the cover and audio upload routes is logged with its code, the file id and the cause
// (09/10/2026: a missing temp bucket answered 409 with nothing in the log). INFO and `ma_loi`.
func TestUploadRefusalsAreLogged(t *testing.T) {
	const fileID = "01JFILE0000000000000000001"
	for _, c := range []struct {
		name   string
		answer func(h *Handler, w http.ResponseWriter, r *http.Request)
		code   string
	}{
		{"cover", func(h *Handler, w http.ResponseWriter, r *http.Request) {
			h.answerCoverError(w, r, "hoàn tất ảnh bìa", app.ErrCoverUploadNotReceived)
		}, "upload_not_received"},
		{"audio", func(h *Handler, w http.ResponseWriter, r *http.Request) {
			h.answerAudioError(w, r, "hoàn tất âm thanh", app.ErrAudioUploadExpired)
		}, "upload_expired"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := &Handler{d: Deps{Log: slog.New(slog.NewTextHandler(&buf, nil))}}
			r := httptest.NewRequest(http.MethodPost, "/x", nil)
			r.SetPathValue("id", fileID)
			r = r.WithContext(tenant.Into(context.Background(), xaA))
			w := httptest.NewRecorder()
			c.answer(h, w, r)
			if w.Code != http.StatusConflict {
				t.Fatalf("status = %d, want 409", w.Code)
			}
			for _, want := range []string{"level=INFO", "ma_loi=" + c.code, "tep_id=" + fileID} {
				if !strings.Contains(buf.String(), want) {
					t.Errorf("log lacks %q: %s", want, buf.String())
				}
			}
		})
	}
}
