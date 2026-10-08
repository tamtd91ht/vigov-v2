package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for GET /api/v1/tasks/register-export.
//
//	PROVED HERE   rule 5 invariant 7's four cases, with NO export call on a refusal · the list's
//	              filters and sort reach the use case, the actor is the session's business code, the
//	              commune is the Host's · a bad sort is 400 and a not-configured `soon` 409 before any
//	              export · 422 / 503 / 500 map from the use case's sentinels, with nothing internal in
//	              the body · the reply is a real .xlsx whose header row is §4.3's thirteen columns IN
//	              ORDER · each cell renders as §4.3 draws it, names in place of ids, unknowns shown as
//	              unknown, never as a ULID or a blank.
//
//	NOT PROVED    the read / names / audit ordering — internal/app/task_register_export_test.go.

type registerExportFake struct {
	calls  int
	req    app.RegisterExportRequest
	actor  audit.Actor
	tenant tenant.ID
	err    error
	data   app.TaskRegisterData
}

func (f *registerExportFake) Export(ctx context.Context, req app.RegisterExportRequest, actor audit.Actor,
	render func(app.TaskRegisterData) ([]byte, error)) ([]byte, int, error) {

	f.calls++
	f.req, f.actor, f.tenant = req, actor, tenant.MustFrom(ctx)
	if f.err != nil {
		return nil, 0, f.err
	}
	b, err := render(f.data)
	return b, len(f.data.Tasks), err
}

const registerExportPath = "/api/v1/tasks/register-export"

var registerDue = time.Date(2026, 10, 5, 2, 30, 0, 0, time.UTC) // 09:30 in Vietnam

func registerSampleData() app.TaskRegisterData {
	return app.TaskRegisterData{
		Tasks: []domain.NhiemVu{{
			ID: "nv-001", Ma: "NV33", TieuDe: "Triển khai chỉ đạo về chuyển đổi số",
			MoTa: "Tổng hợp báo cáo quý.", Khoi: "khoi-uy-ban",
			BoPhanID: "bp-cu", NguoiThucHienMa: "CB-00311",
			HanXuLy: registerDue, TomTatKetQua: "Đã họp triển khai.", GhiChu: "Theo dõi hằng tuần",
			LanhDaoPheDuyetHoanThanh: true,
			VanBan: []domain.NhiemVuVanBan{
				{Nhom: domain.VanBanCapTrenGiao, SoKyHieu: "90-TB/TU", NgayVanBan: time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
					TrichYeu: "Thông báo kết luận"},
				{Nhom: domain.VanBanSanPhamRa, TrichYeu: "Báo cáo kết quả"},
			},
		}, {
			ID: "nv-002", Ma: "NV34", TieuDe: "Rà soát hồ sơ", BoPhanID: "bp-mat", NguoiThucHienMa: "CB-99999",
		}},
		Staff: map[string]identityclient.TenCanBo{
			"CB-00412": {HoTen: "Trần Văn Theo Dõi", TrangThai: identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_IN_DIRECTORY},
			"CB-00311": {HoTen: "Lê Thị Thực Hiện", TrangThai: identityv1.StaffRecordStanding_STAFF_RECORD_STANDING_REMOVED_FROM_DIRECTORY},
		},
		Units: map[string]identityclient.OrgUnitName{
			"bp-vp": {Name: "Văn phòng HĐND-UBND", Standing: identityv1.RecordStanding_RECORD_STANDING_LIVE},
			"bp-cu": {Name: "Phòng Kinh tế cũ", Standing: identityv1.RecordStanding_RECORD_STANDING_REMOVED},
		},
		Blocs: map[string]identityclient.TaskBlocLabel{
			"khoi-uy-ban": {Label: "Khối Uỷ ban", Standing: identityv1.RecordStanding_RECORD_STANDING_LIVE},
		},
	}
}

// --- rule 5, invariant 7 ------------------------------------------------------------------------

func TestRegisterExportNoSessionIs401(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, registerExportPath, nil), http.StatusUnauthorized)
	if m.registerExport.calls != 0 {
		t.Error("đã xuất sổ khi chưa có phiên")
	}
}

func TestRegisterExportWrongPermissionIs403(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{xaA: {}}}
	})
	doiMa(t, m.goi(t, http.MethodGet, hostA, registerExportPath, canBoCuaXa(xaA)), http.StatusForbidden)
	if m.registerExport.calls != 0 {
		t.Error("đã xuất sổ khi thiếu quyền task.read")
	}
}

// TestRegisterExportRightPermissionWrongCommuneIs401 — 401 rather than 403 for the reason
// nhiem_vu_test.go states: the commune comparison runs before the permission is read.
func TestRegisterExportRightPermissionWrongCommuneIs401(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.read"): true}},
			xaB: {idCanBo: {authz.Perm("task.read"): true}},
		}}
	})
	doiMa(t, m.goi(t, http.MethodGet, hostA, registerExportPath, canBoCuaXa(xaB)), http.StatusUnauthorized)
	if m.registerExport.calls != 0 {
		t.Error("đã xuất sổ cho một phiên của xã khác")
	}
}

func TestRegisterExportIs200WithARealWorkbook(t *testing.T) {
	m := dungMayChu(t)
	m.registerExport.data = registerSampleData()

	w := m.goi(t, http.MethodGet, hostA, registerExportPath+"?status=dang-thuc-hien&sort=code&order=asc",
		canBoCuaXa(xaA))

	doiMa(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != xlsxContentType {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); cd != `attachment; filename="so-theo-doi-nhiem-vu.xlsx"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, muốn no-store — tệp mang họ tên cán bộ", cc)
	}

	f := m.registerExport
	if f.calls != 1 || f.tenant != xaA || f.actor.ID != maCanBo {
		t.Fatalf("xuất %d lần, xã %q, người %q — muốn 1, xã của Host, MÃ cán bộ của phiên", f.calls, f.tenant, f.actor.ID)
	}
	if f.req.Filter.TrangThai != "dang-thuc-hien" || f.req.Sort != "code" || f.req.Order != "asc" {
		t.Errorf("bộ lọc/sắp xếp xuống use case = %+v", f.req)
	}

	x, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("thân phản hồi không phải xlsx: %v", err)
	}
	defer x.Close()
	rows, err := x.GetRows("So theo doi")
	if err != nil {
		t.Fatalf("đọc trang: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("%d dòng, muốn 1 tiêu đề + 2 nhiệm vụ", len(rows))
	}
	if got := strings.Join(rows[0], "|"); got != strings.Join(registerColumns, "|") {
		t.Errorf("hàng tiêu đề = %q\nmuốn %q", got, strings.Join(registerColumns, "|"))
	}
	if rows[1][0] != "NV33" || rows[2][0] != "NV34" {
		t.Errorf("thứ tự dòng không giữ thứ tự danh sách: %q, %q", rows[1][0], rows[2][0])
	}
}

// ADR 0082: the export follows the on-screen order, so `sort=status` (with `roots=true`) reaches the use
// case exactly as the list receives it.
func TestRegisterExportCarriesTheStatusSort(t *testing.T) {
	m := dungMayChu(t)
	m.registerExport.data = registerSampleData()

	doiMa(t, m.goi(t, http.MethodGet, hostA, registerExportPath+"?roots=true&sort=status&order=asc",
		canBoCuaXa(xaA)), http.StatusOK)
	f := m.registerExport
	if f.calls != 1 || f.req.Sort != "status" || f.req.Order != "asc" || !f.req.Filter.Roots {
		t.Errorf("xuất %d lần, yêu cầu %+v — muốn sort=status asc, roots", f.calls, f.req)
	}
}

// --- what is refused before any export --------------------------------------------------------------

func TestRegisterExportBadSortIs400(t *testing.T) {
	for _, q := range []string{"?sort=tieu_de", "?sort=code&order=ngang"} {
		m := dungMayChu(t)
		doiMa(t, m.goi(t, http.MethodGet, hostA, registerExportPath+q, canBoCuaXa(xaA)), http.StatusBadRequest)
		if m.registerExport.calls != 0 {
			t.Errorf("%s: đã xuất dù sắp xếp bị từ chối", q)
		}
	}
}

func TestRegisterExportSharesTheListFilterRefusals(t *testing.T) {
	m := dungMayChu(t)
	m.filterIdentity.cutoffErr = fmt.Errorf("x: %w", identityclient.ErrDueSoonNotConfigured)

	doiMa(t, m.goi(t, http.MethodGet, hostA, registerExportPath+"?soon=true", canBoCuaXa(xaA)), http.StatusConflict)
	doiMa(t, m.goi(t, http.MethodGet, hostA, registerExportPath+"?status=chua-thuc-hien", canBoCuaXa(xaA)),
		http.StatusBadRequest)
	if m.registerExport.calls != 0 {
		t.Error("đã xuất dù bộ lọc bị từ chối")
	}
}

func TestRegisterExportErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{app.ErrRegisterExportTooLarge, http.StatusUnprocessableEntity, "register_export_too_large"},
		{fmt.Errorf("%w: %w", app.ErrRegisterNamesUnavailable, identityclient.ErrIdentityUnavailable),
			http.StatusServiceUnavailable, "register_names_unavailable"},
		{errors.New("pg: connection refused on host db-07"), http.StatusInternalServerError, "internal"},
	} {
		m := dungMayChu(t)
		m.registerExport.err = tc.err
		w := m.goi(t, http.MethodGet, hostA, registerExportPath, canBoCuaXa(xaA))
		doiMa(t, w, tc.status)
		if e := loiTra(t, w); e.Code != tc.code || strings.Contains(e.Message, "db-07") {
			t.Errorf("%v: lỗi = %+v", tc.err, e)
		}
	}
}

// --- the cells ------------------------------------------------------------------------------------

// TestRegisterColumnsDropRetiredRoles — ADR 0065 NV5: "Cơ quan chủ trì tham mưu" and "Chuyên viên VP
// tham mưu / theo dõi" are not columns of the file any more; both are "Đơn vị thực hiện".
func TestRegisterColumnsDropRetiredRoles(t *testing.T) {
	if len(registerColumns) != 11 || len(registerColumnWidths) != len(registerColumns) {
		t.Fatalf("%d cột, %d độ rộng — muốn 11 và bằng nhau", len(registerColumns), len(registerColumnWidths))
	}
	for _, c := range registerColumns {
		if strings.Contains(c, "chủ trì") || strings.Contains(c, "theo dõi") {
			t.Errorf("sổ theo dõi còn cột đã gộp: %q", c)
		}
	}
}

func TestRegisterRowRendersSection43(t *testing.T) {
	d := registerSampleData()
	got := registerRow(d.Tasks[0], d)
	want := []string{
		"NV33",
		"Triển khai chỉ đạo về chuyển đổi số\nTổng hợp báo cáo quý.\nKhối Uỷ ban",
		"Phòng Kinh tế cũ (đã gỡ)\nLê Thị Thực Hiện (đã gỡ khỏi danh bạ)",
		"90-TB/TU · 30/11/2026\nThông báo kết luận",
		"",
		"Không số\nBáo cáo kết quả",
		"05/10/2026 09:30",
		"Đã họp triển khai.",
		"Theo dõi hằng tuần",
		"✓",
		"",
	}
	if len(got) != len(registerColumns) {
		t.Fatalf("%d ô, muốn %d", len(got), len(registerColumns))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cột %q = %q, muốn %q", registerColumns[i], got[i], want[i])
		}
	}
}

// TestRegisterRowUnknownsAreShownAsUnknown — an id identity did not answer is NEVER printed as the
// ULID, and nothing prints blank where a value was recorded.
func TestRegisterRowUnknownsAreShownAsUnknown(t *testing.T) {
	d := registerSampleData()
	got := registerRow(d.Tasks[1], d)
	if got[2] != "Không rõ bộ phận\nCB-99999" {
		t.Errorf("đơn vị thực hiện = %q, muốn bộ phận không rõ và MÃ cán bộ chưa tra được", got[2])
	}
	if strings.Contains(strings.Join(got, "|"), "bp-mat") {
		t.Errorf("mã định danh nội bộ của bộ phận lọt vào sổ: %q", got)
	}
	if got[6] != "—" {
		t.Errorf("không có hạn phải hiện —: %q", got[6])
	}
}
