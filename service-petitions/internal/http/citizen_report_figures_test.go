package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The /phan-anh statistics and the log-attachment removal — citizen_report_figures.go, plus the
// `attachments` field on the six act bodies.
//
//	PROVED HERE   rule 5 invariant 7 on the four routes (401 · 403 wrong key · 401 right key WRONG
//	              COMMUNE · 2xx) with the use case NOT reached in the first three · the breakdown needs
//	              BOTH keys · the count and the points take the LIST's filter (same parser, same refusals,
//	              `scope=mine` from the session, the restricted fact) · the points carry lat/lng/status
//	              and nothing else · 422 past the ceiling · the breakdown fails closed (503 / 409) when
//	              working hours cannot be measured, and carries no ratio · the removal's sentences and
//	              codes · the acts hand `attachments` down.
//	NOT PROVED    the SQL (internal/store, citizen_report_figures_test.go) and the transactions
//	              (internal/app).

// --- fakes -----------------------------------------------------------------------------------------

// CountCitizenReports is the list fake's count: THE SAME FILTERING DanhSach applies, so a test of the
// total fails when the handler hands down the wrong filter, not only when it hands down none.
func (d *danhSachPhieuGia) CountCitizenReports(ctx context.Context, loc petstore.LocPhieu) (int, error) {
	res, err := d.DanhSach(ctx, loc, page.Request{})
	if err != nil {
		return 0, err
	}
	return len(res.Items), nil
}

// CitizenReportPoints is the located rows of the same filtered set.
func (d *danhSachPhieuGia) CitizenReportPoints(ctx context.Context, loc petstore.LocPhieu) ([]domain.CitizenReportPoint, error) {
	res, err := d.DanhSach(ctx, loc, page.Request{})
	if err != nil {
		return nil, err
	}
	out := []domain.CitizenReportPoint{}
	for _, p := range res.Items {
		if p.Lat != nil && p.Lng != nil {
			out = append(out, domain.CitizenReportPoint{Lat: *p.Lat, Lng: *p.Lng, Status: p.TrangThai})
		}
	}
	return out, nil
}

// Remove records the file and the reason, then answers through the shared petition gate.
func (f *petitionLogAttachmentsFake) Remove(ctx context.Context, ma, id, reason string, actor audit.Actor,
	resolve app.QuyenXuLyCaXa, restricted app.QuyenXemHanChe) error {
	f.removedID, f.removedReason, f.removeResolve = id, reason, resolve
	return f.admit(ctx, ma, actor, restricted)
}

// citizenReportBreakdownFake answers per commune, from the context.
type citizenReportBreakdownFake struct {
	byTenant   map[tenant.ID]domain.CitizenReportBreakdown
	calls      int
	period     domain.Period
	restricted app.QuyenXemHanChe
	err        error
}

func (f *citizenReportBreakdownFake) Read(ctx context.Context, p domain.Period, restricted app.QuyenXemHanChe) (
	domain.CitizenReportBreakdown, error) {
	f.calls++
	f.period, f.restricted = p, restricted
	if f.err != nil {
		return domain.CitizenReportBreakdown{}, f.err
	}
	return f.byTenant[tenant.MustFrom(ctx)], nil
}

var breakdownAsOf = time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)

// Commune A and commune B carry DIFFERENT, distinct figures, so a leak and a field swap both show.
func citizenReportBreakdownSample() *citizenReportBreakdownFake {
	return &citizenReportBreakdownFake{byTenant: map[tenant.ID]domain.CitizenReportBreakdown{
		xaA: {
			AsOf:   breakdownAsOf,
			Totals: domain.CitizenReportFieldFigures{OnTimeSample: 9, OnTime: 6, Late: 3, Overdue: 4},
			Fields: []domain.CitizenReportFieldFigures{
				{FieldCode: "", Received: 2, Late: 1, OnTimeSample: 1, Overdue: 1},
				{FieldCode: "rac-thai", Received: 11, Finished: 8, OnTimeSample: 8, OnTime: 6, Late: 2,
					RatingSample: 5, RatingSum: 17, Overdue: 3},
			},
			Units: []domain.CitizenReportUnitFigures{
				{OrgUnitID: "bp-moi-truong", Finished: 7, HandlingSample: 6, HandlingWorkingSeconds: 97200},
				{OrgUnitID: "", Finished: 1},
			},
			ResidentialUnits: []domain.CitizenReportResidentialUnitFigures{
				{ResidentialUnitID: "", Received: 1, Overdue: 0},
				{ResidentialUnitID: "thon-ha-lam", Received: 12, Overdue: 4},
			},
		},
		xaB: {AsOf: breakdownAsOf, Fields: []domain.CitizenReportFieldFigures{{FieldCode: "xa-b-only", Received: 99}}},
	}}
}

// --- the four routes, rule 5 invariant 7 ---------------------------------------------------------------

const (
	countsPath    = "/api/v1/citizen-report-counts"
	pointsPath    = "/api/v1/citizen-report-points"
	breakdownPath = "/api/v1/citizen-report-breakdown"
)

func removeLogAttachmentPath(code string) string {
	return logAttachmentsPath(code) + "/" + staffFileIDHTTP
}

type figureRoute struct {
	name, method, path string
	body               any
	keys               []string // every key the route needs
	wrong              string   // a real petitions key that must NOT open it
	calls              func(m *mayChu) int
	ok                 int
}

func figureRoutes() []figureRoute {
	return []figureRoute{
		{"counts", http.MethodGet, countsPath, nil, []string{"feedback.read"}, "feedback.resolve",
			func(m *mayChu) int { return m.danhSach.goi }, http.StatusOK},
		{"points", http.MethodGet, pointsPath + "?status=dang-xu-ly", nil, []string{"feedback.read"}, "feedback.assign",
			func(m *mayChu) int { return m.danhSach.goi }, http.StatusOK},
		{"breakdown", http.MethodGet, breakdownPath + periodQuery, nil, []string{"feedback.read", "report.read"},
			"feedback.restricted", func(m *mayChu) int { return m.breakdown.calls }, http.StatusOK},
		{"remove log attachment", http.MethodDelete, removeLogAttachmentPath(maPhieuThuong),
			citizenReportLogAttachmentRemoveIn{Reason: "Tải nhầm tệp của phiếu khác"}, []string{"feedback.read"},
			"feedback.resolve", func(m *mayChu) int { return m.petitionLogFiles.calls }, http.StatusNoContent},
	}
}

func TestFigureRoutesNoSession401(t *testing.T) {
	for _, r := range figureRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.keys...)
			doiMa(t, m.goiGhiNV(t, r.method, hostA, r.path, nil, r.body), http.StatusUnauthorized)
			if r.calls(m) != 0 {
				t.Error("reached the store/use case without a session")
			}
		})
	}
}

func TestFigureRoutesWrongPermission403(t *testing.T) {
	for _, r := range figureRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.wrong, "task.read")
			doiMa(t, m.goiGhiNV(t, r.method, hostA, r.path, canBoCuaXa(xaA), r.body), http.StatusForbidden)
			if r.calls(m) != 0 {
				t.Error("reached the store/use case with the wrong key")
			}
		})
	}
}

// The breakdown is a REPORT over the register: one key without the other is 403, either way round.
func TestBreakdownNeedsBothKeys(t *testing.T) {
	for _, only := range []string{"feedback.read", "report.read"} {
		t.Run(only, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, only)
			doiMa(t, m.goi(t, http.MethodGet, hostA, breakdownPath+periodQuery, canBoCuaXa(xaA)), http.StatusForbidden)
			if m.breakdown.calls != 0 {
				t.Error("read with one key")
			}
		})
	}
}

// Right keys, granted in BOTH communes, and a session issued by commune B at commune A's host: refused
// before any read (authz compares the commune first, hence 401).
func TestFigureRoutesRightPermissionWrongCommune(t *testing.T) {
	for _, r := range figureRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.keys...)
			doiMa(t, m.goiGhiNV(t, r.method, hostA, r.path, canBoCuaXa(xaB), r.body), http.StatusUnauthorized)
			if r.calls(m) != 0 {
				t.Error("commune A reached with commune B's session")
			}
		})
	}
}

func TestFigureRoutesRightPermissionRightCommune(t *testing.T) {
	for _, r := range figureRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.keys...)
			doiMa(t, m.goiGhiNV(t, r.method, hostA, r.path, canBoCuaXa(xaA), r.body), r.ok)
			if r.calls(m) != 1 {
				t.Errorf("calls = %d, want 1", r.calls(m))
			}
		})
	}
}

// --- counts and points: the list's filter --------------------------------------------------------------

func TestCountsTakeTheListFilterAndRestrictedFact(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read")
	w := m.goi(t, http.MethodGet, hostA, countsPath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var out map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	// Commune A holds three petitions, one of them `can-bo`: two without `feedback.restricted`.
	if len(out) != 1 || out["total"] != 2 {
		t.Errorf("body = %v, want {total: 2}", out)
	}

	grantBoth(m, t, "feedback.read", "feedback.restricted")
	w = m.goi(t, http.MethodGet, hostA, countsPath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"total":3`) {
		t.Errorf("with feedback.restricted: %s", w.Body.String())
	}

	// The SAME LocPhieu the list receives for the same query — compared, not re-listed by hand.
	q := "?status=dang-phan-loai&field=rac-thai&hamlet=thon-1&unit=bp-1&channel=zalo-oa&q=r%C3%A1c&late=true&rating_max=2&scope=mine&limit=5"
	listed := dungMayChu(t)
	doiMa(t, listed.goi(t, http.MethodGet, hostA, duongDanhSach+q, canBoCuaXa(xaA)), http.StatusOK)
	counted := dungMayChu(t)
	doiMa(t, counted.goi(t, http.MethodGet, hostA, countsPath+q, canBoCuaXa(xaA)), http.StatusOK)
	if listed.danhSach.loc != counted.danhSach.loc {
		t.Errorf("count filter = %+v,\nlist filter  = %+v", counted.danhSach.loc, listed.danhSach.loc)
	}
	if counted.danhSach.loc.CanBoXuLyID != maCanBo {
		t.Errorf("scope=mine = %q, want the SESSION's code", counted.danhSach.loc.CanBoXuLyID)
	}
}

func TestCountsAndPointsRefuseWhatTheListRefuses(t *testing.T) {
	for _, path := range []string{countsPath, pointsPath} {
		for _, q := range []string{"?status=khong-co", "?channel=fax", "?late=1", "?rating_max=6", "?scope=related",
			"?metric=received", "?q=" + strings.Repeat("x", petstore.TimPhieuToiDa+1)} {
			t.Run(path+q, func(t *testing.T) {
				m := dungMayChu(t)
				doiMa(t, m.goi(t, http.MethodGet, hostA, path+q, canBoCuaXa(xaA)), http.StatusBadRequest)
				if m.danhSach.goi != 0 {
					t.Error("store reached although the filter was refused")
				}
			})
		}
	}
}

func TestPointsCarryCoordinatesAndStatusOnly(t *testing.T) {
	m := dungMayChu(t)
	lat, lng := 15.512345, 108.234567
	for i := range m.danhSach.theo[xaA] {
		m.danhSach.theo[xaA][i].Lat, m.danhSach.theo[xaA][i].Lng = &lat, &lng
	}
	w := m.goi(t, http.MethodGet, hostA, pointsPath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("items = %d, want commune A's two non-restricted petitions", len(out.Items))
	}
	for _, it := range out.Items {
		keys := []string{}
		for k := range it {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if got := strings.Join(keys, ","); got != "lat,lng,status" {
			t.Errorf("keys = %s — only lat, lng, status (rule 3)", got)
		}
	}
	for _, leak := range []string{maPhieuThuong, maPhieuNhapHo, "Đống rác", "Nguyễn"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Errorf("points carry %q", leak)
		}
	}
}

func TestPointsEmptyIsArrayAndCeilingIs422(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, pointsPath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if strings.TrimSpace(w.Body.String()) != `{"items":[]}` {
		t.Errorf("no located petition: %s", w.Body.String())
	}

	m.danhSach.loi = petstore.ErrTooManyCitizenReportPoints
	w = m.goi(t, http.MethodGet, hostA, pointsPath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusUnprocessableEntity)
	if e := loiTra(t, w); e.Code != "too_many_points" || !strings.Contains(e.Message, "thu hẹp bộ lọc") {
		t.Errorf("error = %+v", e)
	}
}

// --- the breakdown ---------------------------------------------------------------------------------------

func TestBreakdownShapePeriodAndRestrictedFact(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, breakdownPath+periodQuery, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if !m.breakdown.period.From.Equal(periodFrom) || !m.breakdown.period.To.Equal(periodTo) {
		t.Errorf("period = %+v", m.breakdown.period)
	}
	if m.breakdown.restricted {
		t.Error("no feedback.restricted, yet the use case was told to include can-bo")
	}
	var out citizenReportBreakdownOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Totals != (citizenReportBreakdownTotalsOut{OnTimeSample: 9, OnTime: 6, Late: 3, Overdue: 4}) {
		t.Errorf("totals = %+v", out.Totals)
	}
	if len(out.Fields) != 2 || out.Fields[0].FieldCode != "" || out.Fields[1].RatingSum != 17 {
		t.Errorf("fields = %+v — the unclassified row kept, figures per field", out.Fields)
	}
	if len(out.Units) != 2 || out.Units[0].HandlingWorkingSeconds != 97200 || out.Units[0].HandlingSample != 6 {
		t.Errorf("units = %+v", out.Units)
	}
	if len(out.ResidentialUnits) != 2 || out.ResidentialUnits[0].ResidentialUnitID != "" ||
		out.ResidentialUnits[1].Received != 12 {
		t.Errorf("residential units = %+v", out.ResidentialUnits)
	}
	body := w.Body.String()
	for _, cam := range []string{"xa-b-only", "rate", "ratio", "percent", "average", "avg"} {
		if strings.Contains(body, cam) {
			t.Errorf("body carries %q", cam)
		}
	}

	grantBoth(m, t, "feedback.read", "report.read", "feedback.restricted")
	doiMa(t, m.goi(t, http.MethodGet, hostA, breakdownPath+periodQuery, canBoCuaXa(xaA)), http.StatusOK)
	if !m.breakdown.restricted {
		t.Error("feedback.restricted held, yet can-bo excluded")
	}
}

func TestBreakdownEmptyListsAreArrays(t *testing.T) {
	m := dungMayChu(t)
	m.breakdown.byTenant[xaA] = domain.CitizenReportBreakdown{AsOf: breakdownAsOf}
	grantBoth(m, t, "feedback.read", "report.read")
	w := m.goi(t, http.MethodGet, hostA, breakdownPath+periodQuery, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	for _, k := range []string{`"fields":[]`, `"units":[]`, `"residential_units":[]`} {
		if !strings.Contains(w.Body.String(), k) {
			t.Errorf("missing %s: %s", k, w.Body.String())
		}
	}
}

func TestBreakdownFailsClosedWithoutWorkingHours(t *testing.T) {
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"identity down":           {errors.Join(app.ErrWorkingHoursUnavailable, errors.New("rpc Unavailable")), http.StatusServiceUnavailable, "working_hours_unavailable"},
		"calendar not configured": {app.ErrWorkingCalendarMissing, http.StatusConflict, "working_calendar_not_configured"},
		"store fails, no leak":    {errors.New("pg: connection refused on host db-07"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.breakdown.err = c.err
			grantBoth(m, t, "feedback.read", "report.read")
			w := m.goi(t, http.MethodGet, hostA, breakdownPath+periodQuery, canBoCuaXa(xaA))
			doiMa(t, w, c.status)
			if e := loiTra(t, w); e.Code != c.code {
				t.Errorf("code = %q, want %q", e.Code, c.code)
			}
			if strings.Contains(w.Body.String(), "units") || strings.Contains(w.Body.String(), "db-07") {
				t.Errorf("a refused read carries figures or infrastructure: %s", w.Body.String())
			}
		})
	}
}

func TestBreakdownPeriodRefused(t *testing.T) {
	for _, q := range []string{"", "?from=2026-09-22&to=2026-09-29", "?from=2026-09-29T00:00:00Z&to=2026-09-22T00:00:00Z"} {
		m := dungMayChu(t)
		grantBoth(m, t, "feedback.read", "report.read")
		doiMa(t, m.goi(t, http.MethodGet, hostA, breakdownPath+q, canBoCuaXa(xaA)), http.StatusBadRequest)
		if m.breakdown.calls != 0 {
			t.Errorf("%q: use case reached with a refused period", q)
		}
	}
}

func TestRegisterRefusesMissingBreakdown(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Register accepted a nil CitizenReportBreakdown")
		}
	}()
	d := depsDay()
	d.CitizenReportBreakdown = nil
	Register(http.NewServeMux(), d)
}

// --- the removal --------------------------------------------------------------------------------------

func TestRemoveLogAttachmentHandsDownFileReasonAndBusinessCode(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read")
	w := m.goiGhiNV(t, http.MethodDelete, hostA, removeLogAttachmentPath(maPhieuThuong), canBoCuaXa(xaA),
		citizenReportLogAttachmentRemoveIn{Reason: "Tải nhầm tệp"})
	doiMa(t, w, http.StatusNoContent)
	f := m.petitionLogFiles
	if f.removedID != staffFileIDHTTP || f.removedReason != "Tải nhầm tệp" || f.actor.ID != maCanBo || f.restricted ||
		bool(f.removeResolve) {
		t.Errorf("handed down id=%q reason=%q actor=%+v restricted=%v resolve=%v", f.removedID, f.removedReason,
			f.actor, f.restricted, f.removeResolve)
	}
	// The second door: `feedback.resolve` reaches the use case as a fact; the uploader rule is decided there.
	grantBoth(m, t, "feedback.read", "feedback.resolve")
	doiMa(t, m.goiGhiNV(t, http.MethodDelete, hostA, removeLogAttachmentPath(maPhieuThuong), canBoCuaXa(xaA),
		citizenReportLogAttachmentRemoveIn{Reason: "Tải nhầm tệp"}), http.StatusNoContent)
	if !bool(m.petitionLogFiles.removeResolve) {
		t.Error("feedback.resolve held, yet the use case was told it is not")
	}
	// The restricted field: `can-bo` without the key is the unknown code's 404.
	w = m.goiGhiNV(t, http.MethodDelete, hostA, removeLogAttachmentPath(maPhieuCanBo), canBoCuaXa(xaA),
		citizenReportLogAttachmentRemoveIn{Reason: "Tải nhầm tệp"})
	doiMa(t, w, http.StatusNotFound)
}

func TestRemoveLogAttachmentRefusals(t *testing.T) {
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
		says   string
	}{
		"no reason":       {domain.ErrAttachmentRemovalReasonMissing, http.StatusBadRequest, "invalid_request", "hồ sơ xử lý phản ánh"},
		"reason too long": {domain.ErrAttachmentRemovalReasonTooLong, http.StatusBadRequest, "invalid_request", "quá dài"},
		"legal hold":      {domain.ErrAttachmentUnderLegalHold, http.StatusConflict, "legal_hold", "khiếu nại"},
		"neither door":    {domain.ErrAttachmentRemovalNotAllowed, http.StatusForbidden, "forbidden", "người đã tải tệp lên"},
		"not visible":     {app.ErrAttachmentNotFound, http.StatusNotFound, "not_found", "Không tìm thấy"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.petitionLogFiles.err = c.err
			grantBoth(m, t, "feedback.read")
			w := m.goiGhiNV(t, http.MethodDelete, hostA, removeLogAttachmentPath(maPhieuThuong), canBoCuaXa(xaA),
				citizenReportLogAttachmentRemoveIn{Reason: "x"})
			doiMa(t, w, c.status)
			e := loiTra(t, w)
			if e.Code != c.code || !strings.Contains(e.Message, c.says) {
				t.Errorf("error = %+v", e)
			}
			if strings.Contains(e.Message, "nhiệm vụ") {
				t.Errorf("a petition refusal names a TASK record: %q", e.Message)
			}
		})
	}
}

// --- attachments on the acts ------------------------------------------------------------------------------

func TestActsHandDownAttachments(t *testing.T) {
	files := []string{"01JFILE0000000000000000001", "01JFILE0000000000000000002"}
	for _, c := range []struct {
		name string
		key  authz.Perm
		path string
		body any
		got  func(x *xuLyPhieuGia) []string
	}{
		{"classification", "feedback.classify", duongPhanLoai(maPhieuThuong),
			phanLoaiVao{Field: "rac-thai", Attachments: files}, func(x *xuLyPhieuGia) []string { return x.ycLinhVuc.Attachments }},
		{"assignment", "feedback.assign", duongPhanCong(maPhieuThuong),
			phanCongVao{Unit: "bp-001", Attachments: files}, func(x *xuLyPhieuGia) []string { return x.ycPhanCong.Attachments }},
		{"status", "feedback.read", duongTienTrang(maPhieuThuong),
			tienTrangThaiVao{Attachments: files}, func(x *xuLyPhieuGia) []string { return x.attachmentIDs }},
		{"closure", "feedback.resolve", duongDong(maPhieuThuong),
			dongPhieuVao{Result: ketQuaThat, Attachments: files}, func(x *xuLyPhieuGia) []string { return x.attachmentIDs }},
		{"rejection", "feedback.classify", duongKhongTiepNhan(maPhieuThuong),
			khongTiepNhanVao{Reason: "Nội dung không thuộc thẩm quyền của xã.", Attachments: files},
			func(x *xuLyPhieuGia) []string { return x.attachmentIDs }},
		{"referral", "feedback.classify", duongChuyenCapTren(maPhieuThuong),
			chuyenCapTrenVao{Reason: "Thuộc thẩm quyền điện lực.", ReceivingBody: "Điện lực huyện", Attachments: files},
			func(x *xuLyPhieuGia) []string { return x.ycChuyen.Attachments }},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, c.key, "feedback.read")
			doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, c.path, canBoCuaXa(xaA), c.body), http.StatusOK)
			if got := c.got(m.xuLy); strings.Join(got, ",") != strings.Join(files, ",") {
				t.Errorf("attachments handed down = %v", got)
			}
		})
	}
}

// A refused file is a 400 with the domain's one sentence — never err.Error(), which carries the commune.
func TestActRefusedAttachmentIs400WithoutInternals(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.resolve")
	m.xuLy.loi = errors.Join(errors.New("xu_ly_phan_anh: đóng phiếu cho xã "+string(xaA)), domain.ErrPetitionAttachmentNotUsable)
	w := m.goiGhiNV(t, http.MethodPost, hostA, duongDong(maPhieuThuong), canBoCuaXa(xaA),
		dongPhieuVao{Result: ketQuaThat, Attachments: []string{"01JFILE"}})
	doiMa(t, w, http.StatusBadRequest)
	if strings.Contains(w.Body.String(), string(xaA)) || strings.Contains(w.Body.String(), "xu_ly_phan_anh") {
		t.Errorf("internals on the wire: %s", w.Body.String())
	}
}
