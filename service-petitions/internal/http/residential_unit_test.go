package http

// The residential unit on the petition routes — ADR 0088, residential_unit.go. What the ROUTES do with
// `residential_unit_id` and the names; what the ACTS decide (identity asked before writing, the commune in
// the context, the audit before/after) is pinned in internal/app/residential_unit_test.go.
//
//	PROVED HERE   the body field reaches each use case (staff intake, citizen intake, classification) ·
//	              each refusal maps to its status and code, nothing written · the detail and the list
//	              carry id + name from ONE lookup, and refuse 503 when identity cannot name them · a
//	              write response omits the name rather than failing when the lookup fails after commit ·
//	              the breakdown carries the names and refuses 503 · the accountless path refuses the field
//	              (accountless_test.go). The four permission cases of each route are unchanged and live in
//	              that route's own suite.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
)

const (
	unitIDA   = "01JUNITAHTTPTEST000000000"
	unitIDB   = "01JUNITBHTTPTEST000000000"
	unitNameA = "Thôn Một"
)

// unitNamesFake is identity's ResolveResidentialUnitNames, batched. It records every call and the
// commune each one carried.
type unitNamesFake struct {
	names    map[string]identityclient.ResidentialUnitName
	err      error
	calls    int
	asked    [][]string
	communes []tenant.ID
}

func newUnitNamesFake() *unitNamesFake {
	return &unitNamesFake{names: map[string]identityclient.ResidentialUnitName{
		unitIDA: {Name: unitNameA, Standing: identityv1.RecordStanding_RECORD_STANDING_LIVE},
		unitIDB: {Name: "Tổ dân phố Hai", Standing: identityv1.RecordStanding_RECORD_STANDING_REMOVED},
	}}
}

func (f *unitNamesFake) ResidentialUnitNamesInBatches(ctx context.Context, ids []string) (
	map[string]identityclient.ResidentialUnitName, error) {
	f.calls++
	f.asked = append(f.asked, ids)
	c, _ := tenant.From(ctx)
	f.communes = append(f.communes, c)
	if f.err != nil {
		return nil, f.err
	}
	return f.names, nil
}

var errUnitNamesDown = errors.New("identityclient: ResolveResidentialUnitNames: Unavailable")

func withUnit(m *mayChu, xa tenant.ID, code, unit string) {
	p := m.phieu.theo[xa][code]
	p.ThonID = unit
	m.phieu.theo[xa][code] = p
	for i, q := range m.danhSach.theo[xa] {
		if q.MaTraCuu == code {
			m.danhSach.theo[xa][i].ThonID = unit
		}
	}
}

// --- detail -----------------------------------------------------------------------------------------

func TestDetailCarriesResidentialUnitIDAndName(t *testing.T) {
	m := dungMayChu(t)
	withUnit(m, xaA, maPhieuThuong, unitIDB)

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	ra := docPhieu(t, w.Body.Bytes())
	// A RETIRED unit still shows its name — the petition keeps the place it was received in.
	if ra.ResidentialUnitID != unitIDB || ra.ResidentialUnitName != "Tổ dân phố Hai" {
		t.Errorf("thôn = %q / %q", ra.ResidentialUnitID, ra.ResidentialUnitName)
	}
	if m.unitNames.calls != 1 || m.unitNames.communes[0] != xaA {
		t.Errorf("tra tên %d lần, xã %v", m.unitNames.calls, m.unitNames.communes)
	}
}

func TestDetailWithoutUnitAsksNobodyAndOmitsKeys(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if m.unitNames.calls != 0 {
		t.Error("phiếu không có thôn mà vẫn tra tên")
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	for _, k := range []string{"residential_unit_id", "residential_unit_name"} {
		if _, has := raw[k]; has {
			t.Errorf("khoá %q có mặt trên phiếu không có thôn", k)
		}
	}
}

func TestDetailUnitNamesDownIs503(t *testing.T) {
	m := dungMayChu(t)
	withUnit(m, xaA, maPhieuThuong, unitIDA)
	m.unitNames.err = errUnitNamesDown
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusServiceUnavailable)
	if loiTra(t, w).Code != "residential_unit_names_unavailable" {
		t.Errorf("mã lỗi = %q", loiTra(t, w).Code)
	}
}

// Unknown id (another commune's, or since removed from identity's answer): the name is absent, never
// guessed — the id is still there.
func TestDetailUnknownUnitHasNoName(t *testing.T) {
	m := dungMayChu(t)
	withUnit(m, xaA, maPhieuThuong, "01JUNITUNKNOWNHTTP0000000")
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if ra := docPhieu(t, w.Body.Bytes()); ra.ResidentialUnitName != "" {
		t.Errorf("tên đoán cho một mã identity không trả: %q", ra.ResidentialUnitName)
	}
}

// --- list -------------------------------------------------------------------------------------------

func TestListNamesUnitsInOneBatchedCall(t *testing.T) {
	m := dungMayChu(t)
	withUnit(m, xaA, maPhieuThuong, unitIDA)
	w := m.goi(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if m.unitNames.calls != 1 {
		t.Fatalf("tra tên %d lần cho một trang, muốn 1", m.unitNames.calls)
	}
	var out struct {
		Items []phieuPhanAnhRa `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range out.Items {
		if it.Code == maPhieuThuong {
			found = it.ResidentialUnitID == unitIDA && it.ResidentialUnitName == unitNameA
		} else if it.ResidentialUnitID != "" || it.ResidentialUnitName != "" {
			t.Errorf("phiếu %s mang thôn không có thật: %+v", it.Code, it)
		}
	}
	if !found {
		t.Errorf("phiếu %s thiếu thôn/tên trên danh sách: %s", maPhieuThuong, w.Body.String())
	}
}

func TestListUnitNamesDownIs503(t *testing.T) {
	m := dungMayChu(t)
	withUnit(m, xaA, maPhieuThuong, unitIDA)
	m.unitNames.err = errUnitNamesDown
	w := m.goi(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusServiceUnavailable)
	if loiTra(t, w).Code != "residential_unit_names_unavailable" {
		t.Errorf("mã lỗi = %q", loiTra(t, w).Code)
	}
}

// --- staff intake -----------------------------------------------------------------------------------

func TestStaffIntake_ResidentialUnitReachesUseCaseAndComesBackNamed(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	b := staffIntakeBody()
	b["residential_unit_id"] = unitIDA
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), b)
	doiMa(t, w, http.StatusCreated)
	if m.staffIntake.req.ResidentialUnitID != unitIDA {
		t.Errorf("thôn xuống use case = %q", m.staffIntake.req.ResidentialUnitID)
	}
	if ra := docPhieu(t, w.Body.Bytes()); ra.ResidentialUnitID != unitIDA || ra.ResidentialUnitName != unitNameA {
		t.Errorf("thân 201: thôn %q tên %q", ra.ResidentialUnitID, ra.ResidentialUnitName)
	}
}

// After the commit, a failed name lookup must not hide the code from the officer.
func TestStaffIntake_UnitNamesDownStillReturnsCode(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	m.unitNames.err = errUnitNamesDown
	b := staffIntakeBody()
	b["residential_unit_id"] = unitIDA
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), b)
	doiMa(t, w, http.StatusCreated)
	ra := docPhieu(t, w.Body.Bytes())
	if ra.Code != staffIntakeCode || ra.ResidentialUnitID != unitIDA || ra.ResidentialUnitName != "" {
		t.Errorf("thân 201 = %s", w.Body.String())
	}
}

// --- classification -----------------------------------------------------------------------------------

func TestClassification_ResidentialUnitReachesUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.classify")
	w := m.goiThan(t, http.MethodPost, hostA, duongPhanLoai(maPhieuThuong), canBoCuaXa(xaA),
		map[string]any{"field": "rac-thai", "residential_unit_id": unitIDA})
	doiMa(t, w, http.StatusOK)
	got := m.xuLy.ycLinhVuc.ResidentialUnitID
	if got == nil || *got != unitIDA {
		t.Fatalf("thôn xuống use case = %v", got)
	}
	if ra := docPhieu(t, w.Body.Bytes()); ra.ResidentialUnitName != unitNameA {
		t.Errorf("tên thôn trên phản hồi = %q", ra.ResidentialUnitName)
	}

	// Absent stays nil — "keep what the petition holds", never "remove".
	m2 := dungMayChu(t)
	m2.capQuyen(t, "feedback.classify")
	doiMa(t, m2.goiThan(t, http.MethodPost, hostA, duongPhanLoai(maPhieuThuong), canBoCuaXa(xaA),
		map[string]any{"field": "rac-thai"}), http.StatusOK)
	if m2.xuLy.ycLinhVuc.ResidentialUnitID != nil {
		t.Error("không gửi thôn mà use case nhận một giá trị")
	}
}

func TestResidentialUnitRefusalsOnStaffActs(t *testing.T) {
	wrapped := func(e error) error { return fmt.Errorf("x cho xã %s: %w", xaA, e) }
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"không thuộc xã": {app.ErrResidentialUnitNotActive, http.StatusBadRequest, "residential_unit_not_offered"},
		"identity xuống": {wrapped(app.ErrResidentialUnitCheckUnavailable), http.StatusServiceUnavailable, "residential_unit_check_unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "feedback.classify")
			m.xuLy.loi = c.err
			w := m.goiThan(t, http.MethodPost, hostA, duongPhanLoai(maPhieuThuong), canBoCuaXa(xaA),
				map[string]any{"field": "rac-thai", "residential_unit_id": unitIDA})
			doiMa(t, w, c.status)
			if e := loiTra(t, w); e.Code != c.code {
				t.Errorf("mã lỗi = %q, muốn %q", e.Code, c.code)
			}
			if strings.Contains(w.Body.String(), string(xaA)) || strings.Contains(w.Body.String(), unitIDA) {
				t.Errorf("thân lỗi lộ xã hoặc mã thôn: %s", w.Body.String())
			}
		})
	}
}

// --- citizen intake ---------------------------------------------------------------------------------------

func TestCitizenIntake_ResidentialUnitReachesUseCase(t *testing.T) {
	m := dungMayChuGui(t)
	than := strings.Replace(thanThu, `{`, `{"residential_unit_id":"`+unitIDA+`",`, 1)
	doiMa(t, m.gui(t, than, tokenCuaToi, khoaThu), http.StatusCreated)
	if got := m.so.thayYeuCau[0].ResidentialUnitID; got != unitIDA {
		t.Errorf("thôn xuống use case = %q", got)
	}

	m2 := dungMayChuGui(t)
	doiMa(t, m2.gui(t, thanThu, tokenCuaToi, khoaThu), http.StatusCreated)
	if got := m2.so.thayYeuCau[0].ResidentialUnitID; got != "" {
		t.Errorf("không gửi thôn mà use case nhận %q", got)
	}
}

func TestCitizenIntake_ResidentialUnitRefusals(t *testing.T) {
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"không thuộc xã": {app.ErrResidentialUnitNotActive, http.StatusBadRequest, "residential_unit_not_offered"},
		"identity xuống": {fmt.Errorf("gui: %w", app.ErrResidentialUnitCheckUnavailable), http.StatusServiceUnavailable, "residential_unit_check_unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuGui(t)
			m.so.loi = c.err
			than := strings.Replace(thanThu, `{`, `{"residential_unit_id":"`+unitIDA+`",`, 1)
			w := m.gui(t, than, tokenCuaToi, khoaThu)
			doiMa(t, w, c.status)
			if e := loiTra(t, w); e.Code != c.code {
				t.Errorf("mã lỗi = %q, muốn %q", e.Code, c.code)
			}
			if strings.Contains(w.Body.String(), "PA-") {
				t.Errorf("lượt hỏng mang mã tra cứu: %s", w.Body.String())
			}
		})
	}
}

// --- breakdown --------------------------------------------------------------------------------------------

func TestBreakdownCarriesResidentialUnitName(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read", "report.read")
	ra := m.breakdown.byTenant[xaA]
	ra.ResidentialUnits[1].Name = unitNameA
	m.breakdown.byTenant[xaA] = ra
	w := m.goi(t, http.MethodGet, hostA, breakdownPath+periodQuery, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var out citizenReportBreakdownOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.ResidentialUnits[0].ResidentialUnitName != "" || out.ResidentialUnits[1].ResidentialUnitName != unitNameA {
		t.Errorf("tên thôn trên bảng = %+v", out.ResidentialUnits)
	}
}

func TestBreakdownUnitNamesDownIs503(t *testing.T) {
	m := dungMayChu(t)
	grantBoth(m, t, "feedback.read", "report.read")
	m.breakdown.err = fmt.Errorf("x: %w", app.ErrResidentialUnitNamesUnavailable)
	w := m.goi(t, http.MethodGet, hostA, breakdownPath+periodQuery, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusServiceUnavailable)
	if loiTra(t, w).Code != "residential_unit_names_unavailable" {
		t.Errorf("mã lỗi = %q", loiTra(t, w).Code)
	}
}
