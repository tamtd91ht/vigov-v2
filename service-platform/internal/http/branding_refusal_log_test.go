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
	"github.com/vihat/vigov/service-platform/internal/app"
)

// Every 409 of the branding upload routes is logged with its code, the file id and the cause
// (09/10/2026: a missing temp bucket answered 409 with nothing in the log). INFO and `ma_loi`.
func TestBrandingUploadRefusalsAreLogged(t *testing.T) {
	const fileID = "01JFILE0000000000000000001"
	var buf bytes.Buffer
	h := &brandingHandlers{log: slog.New(slog.NewTextHandler(&buf, nil))}
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.SetPathValue("id", fileID)
	r = r.WithContext(tenant.Into(context.Background(), brCommuneA))
	w := httptest.NewRecorder()
	h.answerError(w, r, "hoàn tất ảnh nhận diện", app.ErrBrandingUploadNotReceived)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	for _, want := range []string{"level=INFO", "ma_loi=upload_not_received", "tep_id=" + fileID} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log lacks %q: %s", want, buf.String())
		}
	}
}
