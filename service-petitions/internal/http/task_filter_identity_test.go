package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
)

// Tests for `soon=true` and `scope=related` on GET /api/v1/tasks and GET /api/v1/task-counts — the
// two filters that ask identity before the store is reached.
//
//	PROVED HERE   `soon` asks for the TASK kind, the default row and each priority level (or only the
//	              `priority` named), with ONE instant that becomes both ends of the window the store
//	              receives · each level's threshold reaches the store · not-configured is 409 and names the
//	              missing configuration · any other failure is 503 · neither failure reaches the store ·
//	              `related` asks for the SESSION's code (never a query value), in the request's commune,
//	              and hands the store the code and the units · an empty unit list is a real answer ·
//	              a principal without a code is 500 with no identity call · the counts route shares all
//	              of it · a request naming neither filter makes no identity call.
//
//	NOT PROVED    the SQL those values become — store/task_list_scope_test.go (fake driver) and
//	              TestPgTaskListSoonAndRelated (skips without VIGOV_TEST_DSN).
//
// The four cases rule 5 invariant 7 requires are NOT repeated: no route was added, and both routes
// carry them in nhiem_vu_test.go / task_counts_test.go.

// taskFilterIdentityFake stands in for *identityclient.Client. It reads the commune from the
// context, as the real client lifts it into metadata, and records every question asked.
type taskFilterIdentityFake struct {
	cutoffAfter time.Duration // the cutoff is asOf + this, when no error is set
	cutoffErr   error
	// cutoffAfterByPriority overrides cutoffAfter for one priority code (ADR 0079 lô 2 Q4 b).
	cutoffAfterByPriority map[string]time.Duration
	// askedFields is every field / priority code DueSoonCutoff was asked about, in order.
	askedFields []string
	units       map[tenant.ID]map[string][]string
	unitsErr    error

	cutoffCalls int
	lastKind    identityv1.WorkKind
	lastField   string
	lastAsOf    time.Time
	lastCutoff  time.Time

	unitCalls     int
	lastStaffCode string
	lastTenant    tenant.ID
}

func taskFilterIdentitySample() *taskFilterIdentityFake {
	return &taskFilterIdentityFake{
		cutoffAfter: 17 * time.Hour, // deliberately not 72: nothing here may assume a number
		units: map[tenant.ID]map[string][]string{
			xaA: {maCanBo: {"bp-vpdu", "bp-tu-phap"}},
		},
	}
}

func (f *taskFilterIdentityFake) DueSoonCutoff(ctx context.Context, kind identityv1.WorkKind,
	field string, asOf time.Time) (time.Time, error) {

	f.cutoffCalls++
	f.lastKind, f.lastField, f.lastAsOf = kind, field, asOf
	f.askedFields = append(f.askedFields, field)
	f.lastTenant = tenant.MustFrom(ctx)
	if f.cutoffErr != nil {
		return time.Time{}, f.cutoffErr
	}
	if d, ok := f.cutoffAfterByPriority[field]; ok {
		return asOf.Add(d), nil
	}
	f.lastCutoff = asOf.Add(f.cutoffAfter)
	return f.lastCutoff, nil
}

func (f *taskFilterIdentityFake) StaffOrgUnits(ctx context.Context, staffCode string) ([]string, error) {
	f.unitCalls++
	f.lastStaffCode = staffCode
	f.lastTenant = tenant.MustFrom(ctx)
	if f.unitsErr != nil {
		return nil, f.unitsErr
	}
	return append([]string{}, f.units[f.lastTenant][staffCode]...), nil
}

// --- soon=true -------------------------------------------------------------------------------------

func TestTaskListSoonAsksIdentityPerPriorityAndBindsBothEnds(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?soon=true", canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	f := m.filterIdentity
	// The default row, then each level of commune A's scale (ADR 0079 lô 2 Q4 b) — never per row.
	if got := strings.Join(f.askedFields, "|"); got != "|khan|cao|thuong" {
		t.Fatalf("hỏi ngưỡng theo %q, muốn dòng mặc định rồi từng mức của xã", got)
	}
	if f.lastKind != identityv1.WorkKind_WORK_KIND_NHIEM_VU {
		t.Errorf("hỏi loại việc %v — muốn nhiệm vụ", f.lastKind)
	}
	if f.lastTenant != xaA {
		t.Errorf("hỏi ngưỡng của xã %q, muốn xã của Host", f.lastTenant)
	}
	loc := m.nhiemVu.locCuoi
	if !loc.DueSoonFrom.Equal(f.lastAsOf) || !loc.DueSoonUntil.Equal(f.lastCutoff) {
		t.Errorf("cửa sổ xuống kho = (%v, %v], muốn (%v, %v] — cùng một mốc `now` cho hai đầu",
			loc.DueSoonFrom, loc.DueSoonUntil, f.lastAsOf, f.lastCutoff)
	}
}

func TestTaskListSoonNotConfiguredIs409(t *testing.T) {
	for _, path := range []string{"/api/v1/tasks?soon=true", taskCountsPath + "?soon=true"} {
		m := dungMayChu(t)
		m.filterIdentity.cutoffErr = fmt.Errorf("identityclient: ResolveDueSoonCutoff: %w",
			identityclient.ErrDueSoonNotConfigured)

		w := m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA))

		doiMa(t, w, http.StatusConflict)
		e := loiTra(t, w)
		if e.Code != "due_soon_not_configured" || !strings.Contains(e.Message, "chưa cấu hình ngưỡng sắp đến hạn") {
			t.Errorf("%s: lỗi = %+v, muốn due_soon_not_configured và câu nói rõ xã chưa cấu hình", path, e)
		}
		if m.nhiemVu.goi != 0 {
			t.Errorf("%s: đã đọc kho dù không có ngưỡng — trang sẽ bị đọc là \"không có việc sắp đến hạn\"", path)
		}
	}
}

func TestTaskListSoonIdentityFailureIs503(t *testing.T) {
	for name, cause := range map[string]error{
		"không trả lời": fmt.Errorf("x: %w", identityclient.ErrIdentityUnavailable),
		"hợp đồng hỏng": errors.New("identityclient: ResolveDueSoonCutoff: INTERNAL"),
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.filterIdentity.cutoffErr = cause

			w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?soon=true", canBoCuaXa(xaA))

			doiMa(t, w, http.StatusServiceUnavailable)
			if loiTra(t, w).Code != "task_filter_unavailable" || m.nhiemVu.goi != 0 {
				t.Errorf("mã lỗi hoặc số lần đọc kho sai: %s, %d", w.Body.String(), m.nhiemVu.goi)
			}
		})
	}
}

func TestTaskListWithoutSoonOrRelatedAsksIdentityNothing(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?scope=mine&late=true", canBoCuaXa(xaA)), http.StatusOK)

	if c := m.filterIdentity; c.cutoffCalls+c.unitCalls != 0 {
		t.Errorf("hỏi identity %d+%d lần cho một trang không cần", c.cutoffCalls, c.unitCalls)
	}
	if l := m.nhiemVu.locCuoi; !l.DueSoonFrom.IsZero() || l.Related != nil {
		t.Errorf("bộ lọc xuống kho mang soon/related dù không được hỏi: %+v", l)
	}
}

// Each priority level's threshold reaches the store as that level's upper end, for the SAME `now`; a
// level answering the default's instant is left to the ELSE branch.
func TestTaskListSoonCarriesEachPriorityThreshold(t *testing.T) {
	m := dungMayChu(t)
	m.filterIdentity.cutoffAfterByPriority = map[string]time.Duration{"khan": 40 * time.Hour, "cao": 2 * time.Hour}

	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?soon=true", canBoCuaXa(xaA)), http.StatusOK)

	loc := m.nhiemVu.locCuoi
	p := loc.DueSoonByPriority
	if p == nil || len(p.Until) != 2 ||
		!p.Until["khan"].Equal(loc.DueSoonFrom.Add(40*time.Hour)) || !p.Until["cao"].Equal(loc.DueSoonFrom.Add(2*time.Hour)) {
		t.Fatalf("ngưỡng theo mức xuống kho = %+v (từ %v)", p, loc.DueSoonFrom)
	}
	if _, ok := p.Until["thuong"]; ok {
		t.Errorf("mức trả đúng ngưỡng mặc định vẫn bị đưa xuống kho: %+v", p.Until)
	}
	if !loc.DueSoonUntil.Equal(loc.DueSoonFrom.Add(17 * time.Hour)) {
		t.Errorf("ngưỡng mặc định = %v", loc.DueSoonUntil)
	}
}

// `priority=` names one level: one question, for that level, and no per-level overrides.
func TestTaskListSoonWithPriorityFilterAsksThatLevelOnly(t *testing.T) {
	m := dungMayChu(t)
	m.filterIdentity.cutoffAfterByPriority = map[string]time.Duration{"khan": 40 * time.Hour}

	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?soon=true&priority=khan", canBoCuaXa(xaA)), http.StatusOK)

	if got := strings.Join(m.filterIdentity.askedFields, "|"); got != "khan" {
		t.Errorf("hỏi ngưỡng theo %q, muốn đúng mức khan", got)
	}
	loc := m.nhiemVu.locCuoi
	if loc.DueSoonByPriority != nil || !loc.DueSoonUntil.Equal(loc.DueSoonFrom.Add(40*time.Hour)) {
		t.Errorf("cửa sổ = (%v, %v] theo mức %+v", loc.DueSoonFrom, loc.DueSoonUntil, loc.DueSoonByPriority)
	}
}

// The scale unreadable: refused, never "the default threshold for everything".
func TestTaskListSoonPriorityScaleUnreadableIs503(t *testing.T) {
	m := dungMayChu(t)
	m.uuTien.loi = errors.New("kho xuống")

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?soon=true", canBoCuaXa(xaA))

	doiMa(t, w, http.StatusServiceUnavailable)
	if loiTra(t, w).Code != "task_filter_unavailable" || m.nhiemVu.goi != 0 {
		t.Errorf("mã lỗi hoặc số lần đọc kho sai: %s, %d", w.Body.String(), m.nhiemVu.goi)
	}
}

// --- scope=related ---------------------------------------------------------------------------------

func TestTaskListRelatedUsesSessionCodeAndUnits(t *testing.T) {
	m := dungMayChu(t)

	// `assignee` is a separate picker and stays ANDed; it must NOT become the "tôi" of the scope.
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?scope=related&assignee=CB-99999", canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	f := m.filterIdentity
	if f.unitCalls != 1 || f.lastStaffCode != maCanBo || f.lastTenant != xaA {
		t.Fatalf("hỏi bộ phận %d lần cho %q ở xã %q — muốn 1 lần, mã của CHÍNH phiên, xã của Host",
			f.unitCalls, f.lastStaffCode, f.lastTenant)
	}
	loc := m.nhiemVu.locCuoi
	if loc.Related == nil || loc.Related.StaffCode != maCanBo ||
		strings.Join(loc.Related.OrgUnits, ",") != "bp-vpdu,bp-tu-phap" {
		t.Errorf("phạm vi liên quan xuống kho = %+v", loc.Related)
	}
	if loc.NguoiThucHienMa != "CB-99999" {
		t.Errorf("bộ lọc người thực hiện = %q, muốn giữ nguyên CB-99999 (AND với phạm vi)", loc.NguoiThucHienMa)
	}
}

func TestTaskListRelatedWithNoUnitIsStillServed(t *testing.T) {
	m := dungMayChu(t)
	m.filterIdentity.units = nil // an officer in no unit — an ordinary answer

	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?scope=related", canBoCuaXa(xaA)), http.StatusOK)

	r := m.nhiemVu.locCuoi.Related
	if r == nil || r.StaffCode != maCanBo || len(r.OrgUnits) != 0 {
		t.Errorf("phạm vi liên quan = %+v, muốn mã của phiên và KHÔNG bộ phận nào", r)
	}
}

func TestTaskListRelatedIdentityFailureIs503(t *testing.T) {
	for _, path := range []string{"/api/v1/tasks?scope=related", taskCountsPath + "?scope=related"} {
		m := dungMayChu(t)
		m.filterIdentity.unitsErr = fmt.Errorf("x: %w", identityclient.ErrIdentityUnavailable)

		w := m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA))

		doiMa(t, w, http.StatusServiceUnavailable)
		if m.nhiemVu.goi != 0 {
			t.Errorf("%s: đã đọc kho khi chưa biết bộ phận — tab sẽ thiếu đúng việc nó mang tên", path)
		}
	}
}

func TestTaskListRelatedWithoutStaffCodeIs500(t *testing.T) {
	m := dungMayChu(t)
	p := canBoCuaXa(xaA)
	p.Ma = ""

	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?scope=related", p), http.StatusInternalServerError)

	if m.filterIdentity.unitCalls != 0 || m.nhiemVu.goi != 0 {
		t.Errorf("đã hỏi identity (%d) hoặc đọc kho (%d) với một chủ thể không mã",
			m.filterIdentity.unitCalls, m.nhiemVu.goi)
	}
}

// TestTaskCountsSharesSoonAndRelated — the number over a Kanban column is counted under the SAME
// filter the column's cards are read with.
func TestTaskCountsSharesSoonAndRelated(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, http.MethodGet, hostA, taskCountsPath+"?soon=true&scope=related", canBoCuaXa(xaA)),
		http.StatusOK)

	loc := m.nhiemVu.locCuoi
	if loc.DueSoonUntil.IsZero() || loc.Related == nil || loc.Related.StaffCode != maCanBo {
		t.Errorf("bộ lọc đếm = %+v, muốn cùng soon và related như danh sách", loc)
	}
}
