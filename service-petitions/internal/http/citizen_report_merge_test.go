package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The three merge routes (routes_citizen_report_merge.go) and the link on the staff detail.
//
//	PROVED HERE   rule 5 invariant 7 on the three routes (401 · 403 wrong key · 401 right key WRONG
//	              COMMUNE — authz compares the commune before the key, the repository's answer · 200) with
//	              the use case NOT reached in the first three · the acts get the path code, the body, the
//	              BUSINESS code and the restricted fact · every refusal's status, code and fixed sentence,
//	              never err.Error() · the detail carries the main's CODE and the merged petitions' codes ·
//	              the candidates are the list's masked shape · no citizen response carries the link.
//	NOT PROVED    the rules and the transaction (internal/app/petition_merge_test.go), the SQL
//	              (internal/store/petition_merge_test.go).

// --- fakes -----------------------------------------------------------------------------------------

type mergeActsFake struct {
	calls      int
	tenant     tenant.ID
	code       string
	req        app.MergeRequest
	reason     string
	actor      audit.Actor
	restricted app.QuyenXemHanChe
	err        error
}

func (f *mergeActsFake) Merge(ctx context.Context, code string, req app.MergeRequest, actor audit.Actor,
	restricted app.QuyenXemHanChe) (domain.PhieuPhanAnh, error) {
	f.calls++
	f.tenant, f.code, f.req, f.actor, f.restricted = tenant.MustFrom(ctx), code, req, actor, restricted
	if f.err != nil {
		return domain.PhieuPhanAnh{}, f.err
	}
	return domain.PhieuPhanAnh{ID: "pa-child", MaTraCuu: code, TrangThai: domain.DaTiepNhan,
		MergedInto: "pa-main-internal-id", MergedAt: mergedAtHTTP, MergedBy: actor.ID}, nil
}

func (f *mergeActsFake) Unmerge(ctx context.Context, code, reason string, actor audit.Actor,
	restricted app.QuyenXemHanChe) (domain.PhieuPhanAnh, error) {
	f.calls++
	f.tenant, f.code, f.reason, f.actor, f.restricted = tenant.MustFrom(ctx), code, reason, actor, restricted
	if f.err != nil {
		return domain.PhieuPhanAnh{}, f.err
	}
	return domain.PhieuPhanAnh{ID: "pa-child", MaTraCuu: code, TrangThai: domain.DaTiepNhan}, nil
}

type mergeLinksFake struct {
	byID map[string]domain.MergeLinks
	err  error
}

func (f *mergeLinksFake) MergeLinks(_ context.Context, p domain.PhieuPhanAnh) (domain.MergeLinks, error) {
	if f.err != nil {
		return domain.MergeLinks{}, f.err
	}
	return f.byID[p.ID], nil
}

type duplicateCandidatesFake struct {
	calls      int
	code       string
	restricted app.QuyenXemHanChe
	result     app.DuplicateCandidateResult
	err        error
}

func (f *duplicateCandidatesFake) Candidates(_ context.Context, code string, restricted app.QuyenXemHanChe) (
	app.DuplicateCandidateResult, error) {
	f.calls++
	f.code, f.restricted = code, restricted
	if f.err != nil {
		return app.DuplicateCandidateResult{}, f.err
	}
	if f.result.Items == nil {
		return app.DuplicateCandidateResult{Items: []domain.PhieuPhanAnh{}, RadiusMeters: 50, WindowDays: 7}, nil
	}
	return f.result, nil
}

var mergedAtHTTP = time.Date(2026, 10, 9, 2, 0, 0, 0, time.UTC)

func mergePath(code string) string      { return duong(code) + "/merge" }
func unmergePath(code string) string    { return duong(code) + "/unmerge" }
func candidatesPath(code string) string { return duong(code) + "/duplicate-candidates" }

// --- the three routes, rule 5 invariant 7 ----------------------------------------------------------------

type mergeRoute struct {
	name, method, path string
	body               any
	key, wrong         string
	calls              func(m *mayChu) int
}

func mergeRoutes() []mergeRoute {
	return []mergeRoute{
		{"candidates", http.MethodGet, candidatesPath(maPhieuThuong), nil, "feedback.read", "task.read",
			func(m *mayChu) int { return m.dups.calls }},
		{"merge", http.MethodPost, mergePath(maPhieuThuong), citizenReportMergeIn{MainCode: "PA-MAIN-0000-0000"},
			"feedback.classify", "feedback.assign", func(m *mayChu) int { return m.merge.calls }},
		{"unmerge", http.MethodPost, unmergePath(maPhieuThuong), citizenReportUnmergeIn{Reason: "Chọn nhầm phiếu"},
			"feedback.classify", "feedback.resolve", func(m *mayChu) int { return m.merge.calls }},
	}
}

func TestMergeRoutesNoSession401(t *testing.T) {
	for _, r := range mergeRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.key)
			doiMa(t, m.goiGhiNV(t, r.method, hostA, r.path, nil, r.body), http.StatusUnauthorized)
			if r.calls(m) != 0 {
				t.Error("reached the use case without a session")
			}
		})
	}
}

func TestMergeRoutesWrongPermission403(t *testing.T) {
	for _, r := range mergeRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.wrong)
			doiMa(t, m.goiGhiNV(t, r.method, hostA, r.path, canBoCuaXa(xaA), r.body), http.StatusForbidden)
			if r.calls(m) != 0 {
				t.Error("reached the use case with the wrong key")
			}
		})
	}
}

// Right key, granted in BOTH communes, and a session of commune B at commune A's host: refused before
// anything is read (authz compares the commune first — 401, the repository's answer to this case).
func TestMergeRoutesRightPermissionWrongCommune(t *testing.T) {
	for _, r := range mergeRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.key)
			doiMa(t, m.goiGhiNV(t, r.method, hostA, r.path, canBoCuaXa(xaB), r.body), http.StatusUnauthorized)
			if r.calls(m) != 0 {
				t.Error("commune A reached with commune B's session")
			}
		})
	}
}

func TestMergeRoutesRightPermissionRightCommune(t *testing.T) {
	for _, r := range mergeRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.key)
			doiMa(t, m.goiGhiNV(t, r.method, hostA, r.path, canBoCuaXa(xaA), r.body), http.StatusOK)
			if r.calls(m) != 1 {
				t.Errorf("calls = %d, want 1", r.calls(m))
			}
		})
	}
}

// --- what the acts are handed, and what they answer --------------------------------------------------------

func TestMergeHandsDownCodeBodyBusinessCodeAndRestrictedFact(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.classify")
	w := m.goiGhiNV(t, http.MethodPost, hostA, mergePath(maPhieuThuong), canBoCuaXa(xaA),
		citizenReportMergeIn{MainCode: " PA-MAIN-0000-0000 ", Reason: "Cùng một ổ gà"})
	doiMa(t, w, http.StatusOK)
	f := m.merge
	if f.tenant != xaA || f.code != maPhieuThuong || f.req.MainCode != " PA-MAIN-0000-0000 " ||
		f.req.Reason != "Cùng một ổ gà" || f.actor.ID != maCanBo || f.restricted {
		t.Errorf("handed down %+v", f)
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["merged_into"] != "PA-MAIN-0000-0000" || out["merged_by"] != maCanBo || out["merged_at"] == nil {
		t.Errorf("reply = %v — the main petition's CODE, never its internal id", out)
	}
	if strings.Contains(w.Body.String(), "pa-main-internal-id") {
		t.Error("the internal id of the main petition is on the wire")
	}

	grantBoth(m, t, "feedback.classify", "feedback.restricted")
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, mergePath(maPhieuThuong), canBoCuaXa(xaA),
		citizenReportMergeIn{MainCode: "PA-MAIN-0000-0000"}), http.StatusOK)
	if !m.merge.restricted {
		t.Error("feedback.restricted held, yet the use case was told it is not")
	}
}

func TestUnmergeHandsDownTheReason(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.classify")
	w := m.goiGhiNV(t, http.MethodPost, hostA, unmergePath(maPhieuThuong), canBoCuaXa(xaA),
		citizenReportUnmergeIn{Reason: "Chọn nhầm phiếu chính"})
	doiMa(t, w, http.StatusOK)
	if m.merge.reason != "Chọn nhầm phiếu chính" || m.merge.actor.ID != maCanBo {
		t.Errorf("handed down %+v", m.merge)
	}
	if strings.Contains(w.Body.String(), "merged_into") {
		t.Errorf("an unmerged petition still carries a link: %s", w.Body.String())
	}
}

func TestMergeRefusalsAnswerFixedSentences(t *testing.T) {
	wrap := func(e error) error {
		return errors.Join(errors.New("xu_ly_phan_anh: gộp phiếu cho xã "+string(xaA)), e)
	}
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
		says   string
	}{
		"unknown / other commune": {wrap(petstore.ErrPhieuKhongTonTai), http.StatusNotFound, "not_found", "Không tìm thấy"},
		"can-bo without the key":  {wrap(app.ErrPhieuHanChe), http.StatusNotFound, "not_found", "Không tìm thấy"},
		"main missing":            {domain.ErrMergeMainMissing, http.StatusBadRequest, "invalid_request", "phiếu chính"},
		"into itself":             {domain.ErrMergeSelf, http.StatusBadRequest, "invalid_request", "chính nó"},
		"unmerge reason missing":  {domain.ErrUnmergeReasonMissing, http.StatusBadRequest, "invalid_request", "lý do tách"},
		"status":                  {wrap(domain.ErrMergeNotOpen), http.StatusConflict, "merge_state", "chưa xử lý xong"},
		"chain":                   {wrap(domain.ErrMergeTargetIsMerged), http.StatusConflict, "merge_state", "phiếu chính của nó"},
		"has children":            {wrap(domain.ErrMergeHasChildren), http.StatusConflict, "merge_state", "làm phiếu chính"},
		"can-bo with the key":     {wrap(domain.ErrMergeStaffConduct), http.StatusConflict, "merge_state", "không bao giờ được gộp"},
		"0004 conflict": {wrap(domain.ErrMergeDeadlineBeforeOrigin), http.StatusConflict, "merge_deadline_before_origin",
			"phiếu được phản ánh trước làm phiếu chính"},
		"moved meanwhile": {wrap(petstore.ErrPhieuDaChuyenTrang), http.StatusConflict, "petition_state", "tải lại"},
		"system failure":  {errors.New("pg: connection refused on db-07"), http.StatusInternalServerError, "internal", "Đã xảy ra lỗi"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.merge.err = c.err
			grantBoth(m, t, "feedback.classify")
			w := m.goiGhiNV(t, http.MethodPost, hostA, mergePath(maPhieuThuong), canBoCuaXa(xaA),
				citizenReportMergeIn{MainCode: "PA-MAIN-0000-0000"})
			doiMa(t, w, c.status)
			e := loiTra(t, w)
			if e.Code != c.code || !strings.Contains(e.Message, c.says) {
				t.Errorf("error = %+v", e)
			}
			for _, leak := range []string{string(xaA), "xu_ly_phan_anh", "phan_anh:", "db-07"} {
				if strings.Contains(w.Body.String(), leak) {
					t.Errorf("internals on the wire: %q in %s", leak, w.Body.String())
				}
			}
		})
	}
}

// --- the detail ----------------------------------------------------------------------------------------------

func TestDetailCarriesTheLinkCodes(t *testing.T) {
	m := dungMayChu(t)
	m.links.byID = map[string]domain.MergeLinks{"pa-001": {ChildCodes: []string{"PA-CHLD-0000-0001", "PA-CHLD-0000-0002"}}}
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var out phieuPhanAnhRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Join(out.MergedPetitions, ",") != "PA-CHLD-0000-0001,PA-CHLD-0000-0002" || out.MergedInto != "" {
		t.Errorf("main: merged_into=%q merged_petitions=%v", out.MergedInto, out.MergedPetitions)
	}

	// A merged petition: the main's CODE, who and when.
	p := m.phieu.theo[xaA][maPhieuThuong]
	p.MergedInto, p.MergedAt, p.MergedBy = "pa-main", mergedAtHTTP, "CB-00999"
	m.phieu.theo[xaA][maPhieuThuong] = p
	m.links.byID = map[string]domain.MergeLinks{"pa-001": {MainCode: "PA-MAIN-0000-0000"}}
	w = m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	out = phieuPhanAnhRa{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.MergedInto != "PA-MAIN-0000-0000" || out.MergedBy != "CB-00999" || out.MergedAt == nil ||
		!out.MergedAt.Equal(mergedAtHTTP) || len(out.MergedPetitions) != 0 {
		t.Errorf("merged: %+v", out)
	}
	if strings.Contains(w.Body.String(), `"pa-main"`) {
		t.Error("the main petition's internal id is on the wire")
	}

	// A petition nobody merged carries none of the four keys.
	m.links.byID = nil
	p.MergedInto, p.MergedAt, p.MergedBy = "", time.Time{}, ""
	m.phieu.theo[xaA][maPhieuThuong] = p
	w = m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	if strings.Contains(w.Body.String(), "merged") {
		t.Errorf("an unmerged petition carries link keys: %s", w.Body.String())
	}
}

func TestDetailRefusesWhenTheLinksCannotBeRead(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read", "feedback.unmask")
	m.links.err = errors.New("pg: connection refused on db-07")
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if len(m.vet.ghi) != 0 {
		t.Error("a disclosure was recorded for a read that never answered")
	}
}

// ADR 0087 §4: the link is staff-only — no citizen response type can carry it.
func TestCitizenResponsesNeverCarryTheLink(t *testing.T) {
	p := domain.PhieuPhanAnh{ID: "pa-child", MaTraCuu: "PA-CHLD-0000-0001", TrangThai: domain.DaTiepNhan,
		MergedInto: "pa-main", MergedAt: mergedAtHTTP, MergedBy: "CB-00999", GocDemHan: mocGui, VaoSoLuc: mocVaoSo}
	for name, v := range map[string]any{
		"my-citizen-reports":      phieuCuaToiRaNgoai(p, ""),
		"my-citizen-reports list": tomTatPhieuCuaToi(phieuCuaToiRaNgoai(p, "")),
		"public lookup":           accountlessLookupOf(p),
		"public receipt":          accountlessReceiptOf(p),
	} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, leak := range []string{"merged", "pa-main", "CB-00999"} {
			if strings.Contains(string(b), leak) {
				t.Errorf("%s carries %q: %s", name, leak, b)
			}
		}
	}
}

// --- the candidates ------------------------------------------------------------------------------------------

func TestCandidatesAreTheListShapeMasked(t *testing.T) {
	m := dungMayChu(t)
	lat, lng := 15.88, 108.33
	m.dups.result = app.DuplicateCandidateResult{RadiusMeters: 50, WindowDays: 7, Truncated: true,
		Items: []domain.PhieuPhanAnh{{ID: "pa-near", MaTraCuu: "PA-NEAR-0000-0001", TrangThai: domain.DaTiepNhan,
			NguoiGuiHoTen: "Nguyễn Văn An", NguoiGuiDienThoai: "0900000000", Lat: &lat, Lng: &lng,
			GocDemHan: mocGui, VaoSoLuc: mocVaoSo}}}
	w := m.goi(t, http.MethodGet, hostA, candidatesPath(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var out duplicateCandidatesOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0].Code != "PA-NEAR-0000-0001" || out.RadiusMeters != 50 ||
		out.WindowDays != 7 || !out.Truncated {
		t.Errorf("out = %+v", out)
	}
	if strings.Contains(w.Body.String(), "0900000000") || strings.Contains(w.Body.String(), "Nguyễn Văn An") {
		t.Error("the reporter is unmasked on a list")
	}
	if m.dups.code != maPhieuThuong || m.dups.restricted {
		t.Errorf("handed down %+v", m.dups)
	}

	// Nothing found: `items` is [], never null.
	m.dups.result = app.DuplicateCandidateResult{}
	w = m.goi(t, http.MethodGet, hostA, candidatesPath(maPhieuThuong), canBoCuaXa(xaA))
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("empty: %s", w.Body.String())
	}

	// `can-bo` without the key is the detail's 404.
	m.dups.err = app.ErrPhieuHanChe
	doiMa(t, m.goi(t, http.MethodGet, hostA, candidatesPath(maPhieuCanBo), canBoCuaXa(xaA)), http.StatusNotFound)
}

func TestRegisterRefusesMissingMergeDependencies(t *testing.T) {
	for name, drop := range map[string]func(d *Deps){
		"acts":       func(d *Deps) { d.CitizenReportMerge = nil },
		"links":      func(d *Deps) { d.MergeLinks = nil },
		"candidates": func(d *Deps) { d.DuplicateCandidates = nil },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("Register accepted a nil dependency")
				}
			}()
			d := depsDay()
			drop(&d)
			Register(http.NewServeMux(), d)
		})
	}
}
