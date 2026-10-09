package http

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
)

// Every 4xx refusal of a staff FILE route past the gate is logged with its code, the path's file id and
// the cause (09/10/2026: a missing temp bucket answered 409 with nothing in the log). INFO and `ma_loi`,
// as tuChoiXuLy.
func TestStaffFileRefusalsAreLogged(t *testing.T) {
	const fileID = "01JFILE0000000000000000001"
	mappers := map[string]func(h *Handler, w http.ResponseWriter, r *http.Request, err error){
		"verification photo": func(h *Handler, w http.ResponseWriter, r *http.Request, err error) {
			h.answerVerificationPhotoError(w, r, "hoàn tất ảnh sau xử lý", err)
		},
		"petition log attachment": func(h *Handler, w http.ResponseWriter, r *http.Request, err error) {
			h.answerPetitionLogAttachmentError(w, r, "hoàn tất tệp đính kèm nhật ký", err)
		},
		"task attachment": func(h *Handler, w http.ResponseWriter, r *http.Request, err error) {
			h.answerTaskAttachmentError(w, r, "hoàn tất tệp đính kèm", err)
		},
	}
	for name, answer := range mappers {
		for _, c := range []struct {
			err    error
			status int
			code   string
		}{
			{app.ErrUploadChanged, http.StatusConflict, "upload_changed"},
			// The upload's own failures (ADR 0052 §Sửa đổi 09/10/2026): what was invisible when the
			// bytes went straight to MinIO is now a logged 4xx of this service.
			{fmt.Errorf("tệp tải lên x: %w", httpx.ErrUploadTimeout), http.StatusRequestTimeout, "upload_timeout"},
			{fmt.Errorf("tệp tải lên x: %w", httpx.ErrUploadTooLarge), http.StatusRequestEntityTooLarge, "file_too_large"},
			{fmt.Errorf("tệp tải lên x: %w", httpx.ErrUploadMalformed), http.StatusBadRequest, "invalid_upload"},
		} {
			t.Run(name+"/"+c.code, func(t *testing.T) {
				var buf bytes.Buffer
				h := &Handler{d: Deps{Log: slog.New(slog.NewTextHandler(&buf, nil))}}
				r := httptest.NewRequest(http.MethodPost, "/x", nil)
				r.SetPathValue("id", fileID)
				r = r.WithContext(tenant.Into(context.Background(), xaA))
				w := httptest.NewRecorder()
				answer(h, w, r, c.err)
				if w.Code != c.status {
					t.Fatalf("status = %d, want %d", w.Code, c.status)
				}
				for _, want := range []string{"level=INFO", "ma_loi=" + c.code, "tep_id=" + fileID} {
					if !strings.Contains(buf.String(), want) {
						t.Errorf("log lacks %q: %s", want, buf.String())
					}
				}
			})
		}
	}
}
