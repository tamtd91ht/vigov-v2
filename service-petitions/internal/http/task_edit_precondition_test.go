package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/service-petitions/internal/app"
)

// Optimistic locking on PATCH /api/v1/tasks/{ma} at the HTTP boundary (28/09/2026): the optional
// `expected_updated_at` reaches the use case exactly, its absence reaches it as nil, the conflict is
// 409 `task_changed`, and every task reply carries `updated_at`.

// patchRaw sends a raw JSON body, so the test controls the exact wire spelling of the instant.
func (m *mayChu) patchRaw(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPatch, "https://"+hostA+duongNV(maNVThu), bytes.NewReader([]byte(body)))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYCUATEST")
	r = r.WithContext(authz.Into(r.Context(), *canBoCuaXa(xaA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func TestPatchPrecondition_ReachesUseCaseExactly(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.update"))

	w := m.patchRaw(t, `{"title":"Tên mới","expected_updated_at":"2026-09-21T06:40:12.345678Z"}`)
	doiMa(t, w, http.StatusOK)
	got := m.ghiNhiemVu.ycSua.ExpectedUpdatedAt
	want := time.Date(2026, 9, 21, 6, 40, 12, 345678000, time.UTC)
	if got == nil || !got.Equal(want) {
		t.Errorf("mốc tới use case = %v, muốn %v", got, want)
	}
}

func TestPatchPrecondition_AbsentIsNil(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.update"))

	w := m.patchRaw(t, `{"title":"Tên mới"}`)
	doiMa(t, w, http.StatusOK)
	if m.ghiNhiemVu.ycSua.ExpectedUpdatedAt != nil {
		t.Errorf("không gửi mốc mà use case nhận %v", m.ghiNhiemVu.ycSua.ExpectedUpdatedAt)
	}
}

func TestPatchPrecondition_ConflictIs409TaskChanged(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.update"))
	m.ghiNhiemVu.loi = bocNhuApp(app.ErrTaskEditConflict)

	w := m.patchRaw(t, `{"title":"Tên mới","expected_updated_at":"2026-09-21T06:40:12.345678Z"}`)
	doiMa(t, w, http.StatusConflict)
	e := loiTra(t, w)
	if e.Code != "task_changed" || !strings.Contains(e.Message, "tải lại") {
		t.Errorf("lỗi = %q / %q", e.Code, e.Message)
	}
	if strings.Contains(w.Body.String(), string(xaBocThu)) {
		t.Errorf("thân lộ mã xã: %s", w.Body.String())
	}
}

// TestPatchPrecondition_MalformedInstantIs400: a token that is not RFC 3339 is refused by the body
// decoder, never read as "no precondition" — that would silently turn the check off.
func TestPatchPrecondition_MalformedInstantIs400(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.update"))

	w := m.patchRaw(t, `{"title":"Tên mới","expected_updated_at":"hôm qua"}`)
	doiMa(t, w, http.StatusBadRequest)
	if m.ghiNhiemVu.goi != 0 {
		t.Errorf("mốc hỏng mà vẫn gọi use case %d lần", m.ghiNhiemVu.goi)
	}
}

func TestTaskReplyCarriesUpdatedAt(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if raw["updated_at"] != "2026-09-21T06:40:12.345678Z" {
		t.Errorf("updated_at = %v", raw["updated_at"])
	}
}
