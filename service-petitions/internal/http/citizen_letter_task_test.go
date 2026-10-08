package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// POST /api/v1/citizen-letter-tasks. What the route does with a principal, a body and an answer; what the
// ACT decides (documents asked first, the derived source/deadline/holder, one transaction, nothing written
// on any refusal) is pinned on the real create path in internal/app/citizen_letter_task_test.go.

const (
	citizenLetterTaskPath = "/api/v1/citizen-letter-tasks"
	letterIDHTTP          = "01JDONTHU0000000000000000A"
)

// citizenLetterTaskFake records what reached it, with the commune FROM THE CONTEXT.
type citizenLetterTaskFake struct {
	calls    int
	commune  tenant.ID
	letterID string
	yc       app.YeuCauTaoNhiemVu
	actor    audit.Actor
	err      error
}

func (f *citizenLetterTaskFake) CreateTask(ctx context.Context, letterID string, yc app.YeuCauTaoNhiemVu,
	actor audit.Actor) (domain.NhiemVu, error) {
	f.calls++
	f.commune = tenant.MustFrom(ctx)
	f.letterID, f.yc, f.actor = letterID, yc, actor
	if f.err != nil {
		return domain.NhiemVu{}, f.err
	}
	return domain.NhiemVu{ID: "nv-02", Ma: "NV08", TieuDe: yc.TieuDe, Loai: yc.Loai,
		NguonGiao: domain.SourceCitizenLetter, NguonID: letterID, TrangThai: "moi-giao"}, nil
}

func citizenLetterTaskBody() citizenLetterTaskIn {
	return citizenLetterTaskIn{LetterID: letterIDHTTP, AutoCode: true, Type: "co-ban",
		Title: "Giải quyết đơn số 12/2026 về ranh giới đất"}
}

// --- rule 5, invariant 7 ---------------------------------------------------------------------------------

func TestCitizenLetterTask_NoSessionIs401(t *testing.T) {
	m := dungMayChu(t)
	w := m.goiGhiNV(t, http.MethodPost, hostA, citizenLetterTaskPath, nil, citizenLetterTaskBody())
	doiMa(t, w, http.StatusUnauthorized)
	if m.citizenLetterTasks.calls != 0 {
		t.Error("use case ran without a session")
	}
}

// BOTH keys are required: each one alone is a 403, and the use case is never reached.
func TestCitizenLetterTask_EachKeyAloneIs403(t *testing.T) {
	for name, keys := range map[string][]authz.Perm{
		"missing task.create":   {"petition.read", "feedback.read"},
		"missing petition.read": {"task.create", "task.read", "feedback.read"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, keys...)
			w := m.goiGhiNV(t, http.MethodPost, hostA, citizenLetterTaskPath, canBoCuaXa(xaA), citizenLetterTaskBody())
			doiMa(t, w, http.StatusForbidden)
			if m.citizenLetterTasks.calls != 0 {
				t.Error("use case ran with one of the two keys missing")
			}
		})
	}
}

// 401 AND NOT 403 — this package's convention (TestPetitionTask_RightKeysWrongCommuneIs401): the commune on
// the principal is compared with the one resolved from Host BEFORE either key is consulted.
func TestCitizenLetterTask_RightKeysWrongCommuneIs401(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create", "petition.read")
	w := m.goiGhiNV(t, http.MethodPost, hostB, citizenLetterTaskPath, canBoCuaXa(xaA), citizenLetterTaskBody())
	doiMa(t, w, http.StatusUnauthorized)
	if m.citizenLetterTasks.calls != 0 {
		t.Error("commune A's session reached commune B's letters — a breach between two public authorities")
	}
}

func TestCitizenLetterTask_BothKeysRightCommuneIs201(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create", "petition.read")
	w := m.goiGhiNV(t, http.MethodPost, hostA, citizenLetterTaskPath, canBoCuaXa(xaA), citizenLetterTaskBody())
	doiMa(t, w, http.StatusCreated)

	f := m.citizenLetterTasks
	if f.calls != 1 || f.commune != xaA || f.letterID != letterIDHTTP {
		t.Fatalf("use case: %d calls, commune %q, letter %q", f.calls, f.commune, f.letterID)
	}
	if f.actor.ID != maCanBo {
		t.Errorf("actor = %q, want the business code %q (rule 6, invariant 8)", f.actor.ID, maCanBo)
	}
	if f.yc.NguonGiao != "" || f.yc.NguonID != "" || !f.yc.HanXuLy.IsZero() {
		t.Errorf("handler set source/deadline (%q, %q, %v) — that is the use case's, from documents",
			f.yc.NguonGiao, f.yc.NguonID, f.yc.HanXuLy)
	}
	if f.yc.TieuDe != citizenLetterTaskBody().Title || f.yc.Loai != "co-ban" || !f.yc.TuSinhMa {
		t.Errorf("dialog reached the use case wrong: %+v", f.yc)
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body: %s", w.Body.String())
	}
	if out["code"] != "NV08" || out["source"] != "don-thu" {
		t.Errorf("201 body lacks the minted number or the source: %s", w.Body.String())
	}
}

// --- the source and the deadline are the server's ---------------------------------------------------------

func TestCitizenLetterTask_ClientSentSourceOrDeadlineIs400(t *testing.T) {
	for _, body := range []string{
		`{"letter_id":"` + letterIDHTTP + `","auto_code":true,"type":"co-ban","title":"x","source":"don-thu"}`,
		`{"letter_id":"` + letterIDHTTP + `","auto_code":true,"type":"co-ban","title":"x","source_id":"01JKHAC"}`,
		`{"letter_id":"` + letterIDHTTP + `","auto_code":true,"type":"co-ban","title":"x","due_at":"2030-01-01T00:00:00Z"}`,
		`{"letter_id":"` + letterIDHTTP + `","auto_code":true,"type":"co-ban","title":"x","Due_At":null}`,
		`{"letter_id":"` + letterIDHTTP + `","auto_code":true,"type":"co-ban","title":"x","lead_unit":"bp"}`,
	} {
		t.Run(body, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "task.create", "petition.read")
			w := m.goiGhiNVTho(t, http.MethodPost, hostA, citizenLetterTaskPath, canBoCuaXa(xaA), body)
			doiMa(t, w, http.StatusBadRequest)
			if m.citizenLetterTasks.calls != 0 {
				t.Error("use case ran although the body chose its own source/deadline")
			}
		})
	}
}

func TestCitizenLetterTask_BadLetterIDIs400BeforeTheUseCase(t *testing.T) {
	for name, id := range map[string]string{"missing": "", "blank": "  ", "too long": strings.Repeat("A", 65)} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "task.create", "petition.read")
			b := citizenLetterTaskBody()
			b.LetterID = id
			doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, citizenLetterTaskPath, canBoCuaXa(xaA), b), http.StatusBadRequest)
			if m.citizenLetterTasks.calls != 0 {
				t.Error("use case ran with an unusable letter id")
			}
		})
	}
}

func TestCitizenLetterTask_MissingIdempotencyKeyIs400(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create", "petition.read")
	w := m.goiThan(t, http.MethodPost, hostA, citizenLetterTaskPath, canBoCuaXa(xaA), citizenLetterTaskBody())
	doiMa(t, w, http.StatusBadRequest)
	if m.citizenLetterTasks.calls != 0 {
		t.Error("use case ran without an Idempotency-Key — a double click would make two tasks")
	}
}

// A replay with the same key reaches the use case ONCE and is told the task's register number.
func TestCitizenLetterTask_SameKeyReplaysTheTaskNumber(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create", "petition.read")
	h := idem.Middleware(khoIdemMoi(), nil)(m.h)

	send := func() *httptest.ResponseRecorder {
		b, _ := json.Marshal(citizenLetterTaskBody())
		r := httptest.NewRequest(http.MethodPost, "https://"+hostA+citizenLetterTaskPath, bytes.NewReader(b))
		r.Host = hostA
		r.RemoteAddr = "10.0.0.7:51000"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(idem.Header, "01JIDEMDONTHUTASK000000000")
		r = r.WithContext(authz.Into(r.Context(), *canBoCuaXa(xaA)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	first := send()
	doiMa(t, first, http.StatusCreated)
	second := send()
	doiMa(t, second, http.StatusCreated)
	if m.citizenLetterTasks.calls != 1 {
		t.Fatalf("use case ran %d times for one Idempotency-Key — one click, two tasks", m.citizenLetterTasks.calls)
	}
	if second.Header().Get(idem.HeaderPhatLai) != "true" || !strings.Contains(second.Body.String(), "NV08") {
		t.Errorf("replay not marked or lacks the task number: %v %s", second.Header(), second.Body.String())
	}
}

// --- the answers -----------------------------------------------------------------------------------------

func TestCitizenLetterTask_RefusalsMapToTheirAnswers(t *testing.T) {
	wrapped := func(e error) error { return fmt.Errorf("nhiem_vu: giao việc mới cho xã %s: %w", xaA, e) }
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
		msg    string
	}{
		"not found / other commune / deleted": {app.ErrCitizenLetterNotFound, http.StatusNotFound, "not_found", "Không tìm thấy đơn thư."},
		"denunciation": {app.ErrCitizenLetterDenunciation, http.StatusUnprocessableEntity, "denunciation_no_task",
			"Đơn tố cáo không chuyển thành nhiệm vụ để giữ bí mật người tố cáo."},
		"nobody to hold it": {domain.ErrCitizenLetterAssignmentRequired, http.StatusUnprocessableEntity,
			"assignment_required", "Chọn bộ phận hoặc người thực hiện."},
		"documents down": {fmt.Errorf("%w: %w", app.ErrCitizenLetterUnchecked, errors.New("UNAVAILABLE")),
			http.StatusServiceUnavailable, "citizen_letter_check_unavailable", ""},
		"assignee invalid":   {app.ErrAssignmentStaffInvalid, http.StatusBadRequest, "invalid_request", ""},
		"assignee unchecked": {app.ErrAssignmentStaffUnchecked, http.StatusServiceUnavailable, "assignee_check_unavailable", ""},
		"unit not live":      {app.ErrOrgUnitNotLive, http.StatusBadRequest, "invalid_request", ""},
		"code taken":         {wrapped(petstore.ErrMaNhiemVuDaTonTai), http.StatusConflict, "code_taken", ""},
		"missing title":      {domain.ErrThieuTieuDeNhiemVu, http.StatusBadRequest, "invalid_request", ""},
		"system fault":       {errors.New("kho hỏng"), http.StatusInternalServerError, "internal", ""},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "task.create", "petition.read")
			m.citizenLetterTasks.err = c.err
			w := m.goiGhiNV(t, http.MethodPost, hostA, citizenLetterTaskPath, canBoCuaXa(xaA), citizenLetterTaskBody())
			doiMa(t, w, c.status)
			e := loiTra(t, w)
			if e.Code != c.code {
				t.Errorf("code = %q, want %q", e.Code, c.code)
			}
			if c.msg != "" && e.Message != c.msg {
				t.Errorf("message = %q, want %q", e.Message, c.msg)
			}
			if strings.Contains(w.Body.String(), string(xaA)) || strings.Contains(w.Body.String(), "kho hỏng") ||
				strings.Contains(w.Body.String(), "UNAVAILABLE") {
				t.Errorf("body leaks the commune id or an internal error: %s", w.Body.String())
			}
		})
	}
}

// --- POST /api/v1/tasks refuses the source (ADR 0085 Hệ quả) ---------------------------------------------

func TestCreateTask_CitizenLetterSourceIs400AndNeverReachesUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.create"))

	vao := thanTaoNV()
	vao.Source = string(domain.SourceCitizenLetter)
	vao.SourceID = letterIDHTTP
	w := m.goiGhiNV(t, http.MethodPost, hostA, duongTasks, canBoCuaXa(xaA), vao)

	doiMa(t, w, http.StatusBadRequest)
	if e := loiTra(t, w); e.Message != domain.ErrCitizenLetterSourceNotDirect.Error() {
		t.Errorf("refusal = %q", e.Message)
	}
	if m.ghiNhiemVu.goi != 0 {
		t.Errorf("use case called %d times with source don-thu, want 0", m.ghiNhiemVu.goi)
	}
}
