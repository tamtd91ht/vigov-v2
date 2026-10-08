package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// POST /api/v1/staff-count-queries — the total under the register's CURRENT search (owner decision
// 08/10/2026). What the count admits is proven against the real predicate in
// store/staff_register_sort_test.go (CountMatching equals the walk of the list); here: the four cases
// of rule 5 invariant 7, validation before the store, the SAME filter the search hands the store, and
// the shape on the wire.

const pathStaffCountQueries = "/api/v1/staff-count-queries"

// CountMatching on the fake register RECORDS the filter and answers the commune's row count — it does
// not apply the filter, for the reason DanhSach states: applying it here would be a second copy of the
// predicate, proven nowhere.
func (d *danhBaGia) CountMatching(ctx context.Context, loc domain.LocCanBo) (int, error) {
	d.ghiNhan(ctx)
	d.locCuoi = loc
	if d.loi != nil {
		return 0, d.loi
	}
	return len(d.theo[tenant.MustFrom(ctx)]), nil
}

// --- four cases of rule 5, invariant 7 ----------------------------------------------------------

func TestStaffCountQueries_401NoToken(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, "POST", hostA, pathStaffCountQueries, `{"q":"Nguyễn"}`, ""), http.StatusUnauthorized)
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("counted %d times without a session", m.danhBa.soLanGoi)
	}
}

// `content.update` is a real key on the neighbouring Mini App screen; it does not open the register.
func TestStaffCountQueries_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("content.update"): true}},
			xaB: {},
		}}
	})
	doiMa(t, m.goi(t, "POST", hostA, pathStaffCountQueries, `{"q":"Nguyễn"}`, m.tokenCho(t, xaA, sidA)),
		http.StatusForbidden)
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("counted %d times without admin.user", m.danhBa.soLanGoi)
	}
}

// `admin.user` held in commune A; the same person signed in properly at commune B holds nothing there.
func TestStaffCountQueries_403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, "POST", hostB, pathStaffCountQueries, `{"q":"Nguyễn"}`, m.tokenCho(t, xaB, sidB)),
		http.StatusForbidden)
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("counted %d times in the wrong commune", m.danhBa.soLanGoi)
	}
}

func TestStaffCountQueries_200Both(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, "POST", hostA, pathStaffCountQueries, `{"q":"Nguyễn"}`, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if m.danhBa.xaCuoi != xaA {
		t.Errorf("store called for commune %q, want %q", m.danhBa.xaCuoi, xaA)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %q", w.Body.String())
	}
	if len(got) != 1 || got["total"] != float64(len(danhBaXaA())) {
		t.Errorf("body = %s, want exactly {\"total\":%d}", w.Body.String(), len(danhBaXaA()))
	}
}

// --- the same filter as the search --------------------------------------------------------------

// THE COUNT AND THE SEARCH RECEIVE THE SAME FILTER for the same body — normalised text, unit,
// published — so the total cannot head a list it does not describe. Paging fields are not part of it.
func TestStaffCountQueriesPassesTheSearchFilter(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	const body = `{"q":"  Nguyễn   Văn ","unit":"bp-001","published":false}`

	doiMa(t, m.goi(t, "POST", hostA, duongTim, body, tok), http.StatusOK)
	searched := m.danhBa.locCuoi
	doiMa(t, m.goi(t, "POST", hostA, pathStaffCountQueries, body, tok), http.StatusOK)
	counted := m.danhBa.locCuoi

	if counted.TuKhoa != "Nguyễn Văn" || counted.BoPhanID != "bp-001" || counted.CongKhai == nil || *counted.CongKhai {
		t.Errorf("filter reaching the count = %+v", counted)
	}
	if counted.TuKhoa != searched.TuKhoa || counted.BoPhanID != searched.BoPhanID ||
		(counted.CongKhai == nil) != (searched.CongKhai == nil) || *counted.CongKhai != *searched.CongKhai {
		t.Errorf("count got %+v, search got %+v — the two disagree on the filter", counted, searched)
	}

	// published absent = both.
	doiMa(t, m.goi(t, "POST", hostA, pathStaffCountQueries, `{"q":"Nguyễn"}`, tok), http.StatusOK)
	if m.danhBa.locCuoi.CongKhai != nil || m.danhBa.locCuoi.BoPhanID != "" {
		t.Errorf("absent filters reached the store as %+v", m.danhBa.locCuoi)
	}
}

// THE SEARCH'S OWN VALIDATION, refusals included, before any statement runs.
func TestStaffCountQueriesBadBodyIs400AndNeverReachesTheStore(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	for name, body := range map[string]string{
		"q missing":          `{}`,
		"q empty":            `{"q":""}`,
		"q blank":            `{"q":"   \t "}`,
		"q over 200 chars":   `{"q":"` + strings.Repeat("ễ", domain.TuKhoaTimCanBoToiDa+1) + `"}`,
		"unit too long":      `{"q":"Nguyễn","unit":"` + strings.Repeat("x", 65) + `"}`,
		"published not bool": `{"q":"Nguyễn","published":"yes"}`,
		"not JSON":           `q=Nguyễn`,
	} {
		w := m.goi(t, "POST", hostA, pathStaffCountQueries, body, tok)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400: %s", name, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "Nguyễn") || strings.Contains(w.Body.String(), "yes") {
			t.Errorf("%s: the refusal quotes what was sent: %s", name, w.Body.String())
		}
	}
	if m.danhBa.soLanGoi != 0 {
		t.Errorf("a refused body still reached the store %d times", m.danhBa.soLanGoi)
	}
}

// `q` NEVER REACHES THE LOGGER — the store-failure path is the one that logs (rule 3, invariant 1).
func TestStaffCountQueriesNeverLogsTheSearchText(t *testing.T) {
	m, buf := mayChuLog(t)
	const text = "Hoàng Bí Mật 0900000099"
	m.danhBa.loi = errors.New("cơ sở dữ liệu không phản hồi: nguoi_dung")

	w := m.goi(t, "POST", hostA, pathStaffCountQueries, `{"q":"`+text+`"}`, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "nguoi_dung") {
		t.Errorf("the store error reached the client: %s", w.Body.String())
	}
	log := buf.String()
	if !strings.Contains(log, "đếm cán bộ theo tìm kiếm") {
		t.Fatalf("the failure path logged nothing — the case proves nothing: %q", log)
	}
	for _, part := range []string{"Hoàng", "Bí Mật", "0900000099"} {
		if strings.Contains(log, part) {
			t.Errorf("the log carries the search text %q: %s", part, log)
		}
	}
}
