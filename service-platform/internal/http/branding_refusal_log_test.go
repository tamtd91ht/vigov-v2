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
	"github.com/vihat/vigov/service-platform/internal/app"
)

// Every refusal of the branding upload routes that is the client's or the file's is logged at INFO with
// its code and the cause (09/10/2026: a missing temp bucket answered 409 with nothing in the log). The
// file id rides in the cause — Upload wraps every error after the row exists with it — since the
// `…/{id}/completion` path that used to carry it is gone (ADR 0052 §Sửa đổi 09/10/2026).
func TestBrandingUploadRefusalsAreLogged(t *testing.T) {
	const fileID = "01JFILE0000000000000000001"
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"file limit", fmt.Errorf("nhận diện xã: tệp %s: %w", fileID, app.ErrBrandingCountReached), http.StatusConflict, "upload_limit"},
		{"client too slow", fmt.Errorf("nhận diện xã: tệp %s: storage: read: %w", fileID, httpx.ErrUploadTimeout), http.StatusRequestTimeout, "upload_timeout"},
		{"rejected image", fmt.Errorf("nhận diện xã: tệp %s: %w", fileID, &app.BrandingRejection{Reason: app.BrandingRejectUndecodable}), http.StatusUnprocessableEntity, "image_rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := &brandingHandlers{log: slog.New(slog.NewTextHandler(&buf, nil))}
			r := httptest.NewRequest(http.MethodPost, "/x", nil)
			r = r.WithContext(tenant.Into(context.Background(), brCommuneA))
			w := httptest.NewRecorder()
			h.answerError(w, r, "tải ảnh nhận diện", tc.err)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			for _, want := range []string{"level=INFO", "ma_loi=" + tc.code, fileID, "xa=" + string(brCommuneA)} {
				if !strings.Contains(buf.String(), want) {
					t.Errorf("log lacks %q: %s", want, buf.String())
				}
			}
		})
	}
}
