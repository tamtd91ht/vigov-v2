package http

// The display names on task responses (owner decision 15/10/2026, task_names.go):
//
//	list     ONE staff lookup and ONE unit lookup for the whole page, codes de-duplicated; names land on
//	         the right rows; a code identity does not answer leaves its field ABSENT (the code stays)
//	chunks   a page with more staff codes than one call may carry is split, never truncated
//	detail   the single read carries the names too
//	outage   identity failing = 200 with the codes and no names — never a failed list
//	commune  the lookup runs under the request's commune (the fake answers per commune)

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// taskNamesFake answers identity's two name lookups PER COMMUNE, reading the commune from the context
// as the real client's metadata does. Every call is recorded with what it was asked.
type taskNamesFake struct {
	staff map[tenant.ID]map[string]string
	units map[tenant.ID]map[string]string
	fail  error

	staffCalls [][]string
	unitCalls  [][]string
}

// vi-name-ok: implements the existing core/identityclient.Client method TaskNameLookups mirrors
func (f *taskNamesFake) TenCanBoTheoMa(ctx context.Context, codes []string) (map[string]identityclient.TenCanBo, error) {
	f.staffCalls = append(f.staffCalls, append([]string(nil), codes...))
	if f.fail != nil {
		return nil, f.fail
	}
	out := map[string]identityclient.TenCanBo{}
	for _, c := range codes {
		if name, ok := f.staff[tenant.MustFrom(ctx)][c]; ok {
			out[c] = identityclient.TenCanBo{HoTen: name}
		}
	}
	return out, nil
}

func (f *taskNamesFake) OrgUnitNames(ctx context.Context, ids []string) (map[string]identityclient.OrgUnitName, error) {
	f.unitCalls = append(f.unitCalls, append([]string(nil), ids...))
	if f.fail != nil {
		return nil, f.fail
	}
	out := map[string]identityclient.OrgUnitName{}
	for _, id := range ids {
		if name, ok := f.units[tenant.MustFrom(ctx)][id]; ok {
			out[id] = identityclient.OrgUnitName{Name: name}
		}
	}
	return out, nil
}

// taskNamesSample names what nhiemVuMau's commune-A tasks carry — EXCEPT maCanBo (NV88's assignee),
// left unanswered so the absent case is real. Commune B's table names the SAME codes differently, so a
// lookup run under the wrong commune would show B's names.
func taskNamesSample() *taskNamesFake {
	return &taskNamesFake{
		staff: map[tenant.ID]map[string]string{
			xaA: {"CB-00311": "Cán Bộ Thực Hiện", "CB-00007": "Lãnh Đạo Giao Việc"},
			xaB: {"CB-00311": "Tên Ở Xã B", "CB-00007": "Tên Ở Xã B"},
		},
		units: map[tenant.ID]map[string]string{
			xaA: {"bp-vpdu": "Văn phòng Đảng uỷ (đã gỡ)"},
			xaB: {"bp-vpdu": "Bộ phận Ở Xã B"},
		},
	}
}

func withTaskNames(t *testing.T, m *mayChu, f *taskNamesFake) {
	t.Helper()
	m.dungLai(t, func(d *Deps) { d.TaskNames = f })
}

func TestTaskListNamesOneLookupPerKind(t *testing.T) {
	m := dungMayChu(t)
	names := taskNamesSample()
	withTaskNames(t, m, names)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if len(names.staffCalls) != 1 || len(names.unitCalls) != 1 {
		t.Fatalf("staff calls %d, unit calls %d — muốn đúng 1 mỗi loại cho cả trang",
			len(names.staffCalls), len(names.unitCalls))
	}
	// NV19 carries CB-00311 + CB-00007, NV88 carries maCanBo: three distinct codes, each asked once.
	if got := len(names.staffCalls[0]); got != 3 {
		t.Errorf("hỏi %d mã cán bộ (%v), muốn 3 mã không trùng", got, names.staffCalls[0])
	}
	if got := names.unitCalls[0]; len(got) != 1 || got[0] != "bp-vpdu" {
		t.Errorf("hỏi bộ phận %v, muốn [bp-vpdu] — mã rỗng không được hỏi", got)
	}

	res := docTrangNhiemVu(t, w.Body.Bytes())
	byCode := map[string]nhiemVuRa{}
	for _, it := range res.Items {
		byCode[it.Code] = it
	}
	nv := byCode[maNhiemVuA]
	if nv.AssigneeName != "Cán Bộ Thực Hiện" || nv.AssignerName != "Lãnh Đạo Giao Việc" ||
		nv.UnitName != "Văn phòng Đảng uỷ (đã gỡ)" {
		t.Errorf("NV19 tên = %q / %q / %q", nv.AssigneeName, nv.AssignerName, nv.UnitName)
	}
	if nv.Assignee != "CB-00311" || nv.Unit != "bp-vpdu" {
		t.Errorf("mã bị thay bằng tên: assignee=%q unit=%q", nv.Assignee, nv.Unit)
	}
	// NV88: its assignee is not answered → the field is ABSENT on the wire, the code stays.
	if xo := byCode[maNhiemVuXo]; xo.AssigneeName != "" || xo.Assignee != maCanBo {
		t.Errorf("NV88: assignee_name = %q (muốn vắng), assignee = %q", xo.AssigneeName, xo.Assignee)
	}
	if n := strings.Count(w.Body.String(), `"assignee_name"`); n != 1 {
		t.Errorf("assignee_name xuất hiện %d lần, muốn 1 (chỉ NV19)", n)
	}
}

func TestTaskListNamesChunkedNeverTruncated(t *testing.T) {
	m := dungMayChu(t)
	names := taskNamesSample()
	withTaskNames(t, m, names)

	// 150 tasks × (assignee + assigner) = 300 distinct codes, over the 200-code ceiling of one call.
	var many []domain.NhiemVu
	for i := 0; i < 150; i++ {
		many = append(many, domain.NhiemVu{
			ID: fmt.Sprintf("nv-%03d", i), Ma: fmt.Sprintf("NV%03d", i), TrangThai: domain.MoiGiao,
			NguonGiao:       domain.NguonTrucTiep,
			NguoiThucHienMa: fmt.Sprintf("CB-A%03d", i), LanhDaoGiaoViecMa: fmt.Sprintf("CB-L%03d", i),
		})
	}
	m.nhiemVu.theo[xaA] = many

	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA)), http.StatusOK)

	total := 0
	for _, c := range names.staffCalls {
		if len(c) > identityclient.TranMaMotLo {
			t.Errorf("một lần hỏi %d mã, vượt trần %d", len(c), identityclient.TranMaMotLo)
		}
		total += len(c)
	}
	if len(names.staffCalls) != 2 || total != 300 {
		t.Errorf("staff calls = %d, tổng mã = %d — muốn 2 lần, đủ 300 mã", len(names.staffCalls), total)
	}
	if len(names.unitCalls) != 0 {
		t.Errorf("hỏi bộ phận %d lần cho trang không có bộ phận nào", len(names.unitCalls))
	}
}

func TestTaskDetailCarriesNames(t *testing.T) {
	m := dungMayChu(t)
	withTaskNames(t, m, taskNamesSample())

	w := m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	ra := docNhiemVu(t, w.Body.Bytes())
	if ra.AssigneeName != "Cán Bộ Thực Hiện" || ra.AssignerName != "Lãnh Đạo Giao Việc" ||
		ra.UnitName != "Văn phòng Đảng uỷ (đã gỡ)" {
		t.Errorf("chi tiết: tên = %q / %q / %q", ra.AssigneeName, ra.AssignerName, ra.UnitName)
	}
}

// IDENTITY DOWN = THE LIST WITHOUT NAMES (owner decision 15/10/2026), never a 5xx.
func TestTaskListIdentityDownServesCodesWithoutNames(t *testing.T) {
	m := dungMayChu(t)
	names := taskNamesSample()
	names.fail = fmt.Errorf("%w: unavailable", identityclient.ErrIdentityUnavailable)
	withTaskNames(t, m, names)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	res := docTrangNhiemVu(t, w.Body.Bytes())
	if len(res.Items) != 2 {
		t.Fatalf("trả %d nhiệm vụ, muốn 2", len(res.Items))
	}
	for _, it := range res.Items {
		if it.AssigneeName != "" || it.AssignerName != "" || it.UnitName != "" {
			t.Errorf("%s có tên dù identity lỗi", it.Code)
		}
		if it.Assignee == "" {
			t.Errorf("%s mất mã người thực hiện", it.Code)
		}
	}
	if strings.Contains(w.Body.String(), "_name\"") {
		t.Error("thân có trường *_name dù identity lỗi")
	}
}

// A WRITE REPLY after a committed act: a failed lookup must not turn into an error the officer retries.
func TestTaskWriteReplyIdentityDownKeepsCodes(t *testing.T) {
	m := dungMayChu(t)
	names := taskNamesSample()
	names.fail = errors.New("identity down")
	withTaskNames(t, m, names)
	h := &Handler{d: m.d}

	ra := h.namedTask(tenant.Into(context.Background(), xaA),
		domain.NhiemVu{Ma: "NV01", NguoiThucHienMa: "CB-00311", BoPhanID: "bp-vpdu"})
	if ra.Code != "NV01" || ra.Assignee != "CB-00311" || ra.AssigneeName != "" || ra.UnitName != "" {
		t.Errorf("reply: code=%q assignee=%q — muốn mã còn, tên vắng", ra.Code, ra.Assignee)
	}
}

// THE LOOKUP RUNS UNDER THE REQUEST'S COMMUNE: commune B's context gets B's names, never A's.
func TestTaskNamesResolvedInRequestCommune(t *testing.T) {
	m := dungMayChu(t)
	withTaskNames(t, m, taskNamesSample())
	h := &Handler{d: m.d}

	ra := h.namedTask(tenant.Into(context.Background(), xaB),
		domain.NhiemVu{Ma: "NV07", NguoiThucHienMa: "CB-00311", BoPhanID: "bp-vpdu"})
	if ra.AssigneeName != "Tên Ở Xã B" || ra.UnitName != "Bộ phận Ở Xã B" {
		t.Errorf("xã B nhận tên = %q / %q", ra.AssigneeName, ra.UnitName)
	}
}
