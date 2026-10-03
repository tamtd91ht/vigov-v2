package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// GET /api/v1/staff-counts. Which rows each number admits is proven against the real predicates in
// store/staff_counts_test.go and staff_counts_pg_test.go; here: the four cases of rule 5 invariant 7,
// the commune reaching the store from the context, and the shape on the wire.

const pathStaffCounts = "/api/v1/staff-counts"

// StaffCounts on the fake register folds the commune's rows by department. `published` here is the
// flag alone — the fake does not restate the publication predicate (that would be a third copy); it
// only has to produce DIFFERENT numbers per bucket so a transposition in the handler shows.
func (d *danhBaGia) StaffCounts(ctx context.Context) (domain.StaffCounts, error) {
	d.ghiNhan(ctx)
	if d.loi != nil {
		return domain.StaffCounts{}, d.loi
	}
	byDept := map[string]domain.StaffTally{}
	out := domain.StaffCounts{Departments: []domain.DepartmentStaffTally{}}
	for _, cb := range d.theo[tenant.MustFrom(ctx)] {
		tally := byDept[cb.BoPhanID]
		tally.Total++
		out.Total++
		if cb.HienTrenMiniApp {
			tally.Published++
			out.Published++
		}
		byDept[cb.BoPhanID] = tally
	}
	for id, tally := range byDept {
		if id == "" {
			out.NoDepartment = tally
			continue
		}
		out.Departments = append(out.Departments, domain.DepartmentStaffTally{DepartmentID: id, StaffTally: tally})
	}
	sort.Slice(out.Departments, func(i, j int) bool { return out.Departments[i].DepartmentID < out.Departments[j].DepartmentID })
	return out, nil
}

// --- four cases of rule 5, invariant 7 ----------------------------------------------------------

func TestStaffCounts_401NoToken(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, "GET", hostA, pathStaffCounts, "", ""), http.StatusUnauthorized)
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("chưa đăng nhập mà đã đếm danh bạ %d lần", m.danhBa.soLanGoi)
	}
}

func TestStaffCounts_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	// The REAL routes behind a checker granting another key: fails if the route loses its declaration.
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{xaA: {idNoiBo: {"task.read": true}}}}
	})
	doiMa(t, m.goi(t, "GET", hostA, pathStaffCounts, "", m.tokenCho(t, xaA, sidA)), http.StatusForbidden)
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("thiếu quyền mà đã đếm danh bạ %d lần", m.danhBa.soLanGoi)
	}
}

func TestStaffCounts_403RightPermissionWrongCommune(t *testing.T) {
	// admin.user is held in commune A; the same person signed in properly at commune B holds nothing
	// there (rule 5, invariant 3).
	m := dungMayChu(t)
	doiMa(t, m.goi(t, "GET", hostB, pathStaffCounts, "", m.tokenCho(t, xaB, sidB)), http.StatusForbidden)
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("sai xã mà đã đếm danh bạ %d lần — phép kiểm quyền phải chặn trước kho", m.danhBa.soLanGoi)
	}
}

func TestStaffCounts_200Both(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, "GET", hostA, pathStaffCounts, "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if m.danhBa.xaCuoi != xaA {
		t.Errorf("kho được gọi với xã %q, muốn %q", m.danhBa.xaCuoi, xaA)
	}
}

// --- the shape on the wire --------------------------------------------------------------------------

func TestStaffCountsShape(t *testing.T) {
	m := dungMayChu(t)
	rows := danhBaXaA() // bp-001 ×2, bp-002 ×1
	rows[0].HienTrenMiniApp = true
	rows = append(rows, domain.CanBoTomTat{ID: "nd-04", Ma: "CB-004"}) // no department
	m.danhBa.theo[xaA] = rows

	w := m.goi(t, "GET", hostA, pathStaffCounts, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var got staffCountsOut
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	want := staffCountsOut{
		Total: 4, Published: 1,
		NoDepartment: staffTallyOut{Total: 1, Published: 0},
		Departments: []departmentCountOut{
			{ID: "bp-001", Total: 2, Published: 1},
			{ID: "bp-002", Total: 1, Published: 0},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("staff-counts = %+v\nmuốn          %+v", got, want)
	}
	// Commune B's one row must not be counted in A.
	for _, d := range got.Departments {
		if d.ID == "" {
			t.Errorf("bucket không bộ phận lọt vào departments với id rỗng: %s", w.Body.String())
		}
	}
}

func TestStaffCountsEmptyCommuneIsEmptyArrayNotNull(t *testing.T) {
	m := dungMayChu(t)
	m.danhBa.theo[xaA] = nil
	w := m.goi(t, "GET", hostA, pathStaffCounts, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"departments":[]`) {
		t.Errorf("thân = %s, muốn departments: []", w.Body.String())
	}
}

func TestStaffCountsStoreFailureIs500WithoutDetail(t *testing.T) {
	m := dungMayChu(t)
	m.danhBa.loi = errors.New("cơ sở dữ liệu không phản hồi: nguoi_dung")
	w := m.goi(t, "GET", hostA, pathStaffCounts, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "nguoi_dung") {
		t.Errorf("để lộ lỗi nội bộ: %s", w.Body.String())
	}
}
