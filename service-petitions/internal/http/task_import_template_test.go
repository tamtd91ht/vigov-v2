package http

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for the DROPDOWNS of GET /api/v1/tasks/import-template (owner 14/10/2026).
//
//	PROVED HERE   type and priority list this commune's IN-USE catalogue rows as "code · label"; the
//	              two "(x)" columns offer "x" and STOP other values, the rest only WARN · unit, staff
//	              and bloc list only the client's codes identity confirms for THIS commune (live unit,
//	              assignable staff, LIVE bloc), with identity's names, in the client's order, once ·
//	              more than 2000 values of one parameter is 400 `too_many_choices` with no identity
//	              call · identity failing or not wired is 200 without those three · no client codes, no
//	              identity call · an empty catalogue gives its column no dropdown · header and example
//	              row unchanged, data sheet first.
//
//	NOT PROVED    the real identity client (core/identityclient's own tests) and that the import
//	              accepts the dropdown values (app TestTaskImportAcceptsDropdownValues).

// importChoicesFake answers the template's identity lookups PER COMMUNE, from the context, as identity
// does: another commune's code is simply absent.
type importChoicesFake struct {
	units     map[tenant.ID]map[string]string // code → id, live units
	unitNames map[string]string               // id → name
	staff     map[tenant.ID]map[string]string // assignable code → full name (invented)
	blocs     map[tenant.ID]map[string]identityclient.TaskBlocLabel
	err       error
	calls     int
}

func (f *importChoicesFake) LiveOrgUnitIDsByCode(ctx context.Context, codes []string) (map[string]string, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]string{}
	for _, c := range codes {
		if id, ok := f.units[tenant.MustFrom(ctx)][c]; ok {
			out[c] = id
		}
	}
	return out, nil
}

func (f *importChoicesFake) OrgUnitNames(_ context.Context, ids []string) (map[string]identityclient.OrgUnitName, error) {
	f.calls++
	out := map[string]identityclient.OrgUnitName{}
	for _, id := range ids {
		if n, ok := f.unitNames[id]; ok {
			out[id] = identityclient.OrgUnitName{Name: n, Standing: identityv1.RecordStanding_RECORD_STANDING_LIVE}
		}
	}
	return out, nil
}

// vi-name-ok: the method name of core/identityclient.Client the interface under test must match
func (f *importChoicesFake) CanBoGiaoViecDuoc(ctx context.Context, ma []string) (map[string]struct{}, error) {
	f.calls++
	out := map[string]struct{}{}
	for _, c := range ma {
		if _, ok := f.staff[tenant.MustFrom(ctx)][c]; ok {
			out[c] = struct{}{}
		}
	}
	return out, nil
}

// vi-name-ok: the method name of core/identityclient.Client the interface under test must match
func (f *importChoicesFake) TenCanBoTheoMa(ctx context.Context, ma []string) (map[string]identityclient.TenCanBo, error) {
	f.calls++
	out := map[string]identityclient.TenCanBo{}
	for _, c := range ma {
		if n, ok := f.staff[tenant.MustFrom(ctx)][c]; ok {
			out[c] = identityclient.TenCanBo{HoTen: n}
		}
	}
	return out, nil
}

func (f *importChoicesFake) TaskBlocLabels(ctx context.Context, codes []string) (map[string]identityclient.TaskBlocLabel, error) {
	f.calls++
	out := map[string]identityclient.TaskBlocLabel{}
	for _, c := range codes {
		if l, ok := f.blocs[tenant.MustFrom(ctx)][c]; ok {
			out[c] = l
		}
	}
	return out, nil
}

// importChoicesSample: commune A has two live units, two assignable officers (invented names) and a
// live and a removed bloc; commune B has its own of each, which a commune-A request must never see.
func importChoicesSample() *importChoicesFake {
	live, removed := identityv1.RecordStanding_RECORD_STANDING_LIVE, identityv1.RecordStanding_RECORD_STANDING_REMOVED
	return &importChoicesFake{
		units: map[tenant.ID]map[string]string{
			xaA: {"dia-chinh": "bp-a-1", "van-phong": "bp-a-2"},
			xaB: {"tu-phap-b": "bp-b-1"},
		},
		unitNames: map[string]string{"bp-a-1": "Địa chính", "bp-a-2": "Văn phòng", "bp-b-1": "Tư pháp xã B"},
		staff: map[tenant.ID]map[string]string{
			xaA: {"CB-00311": "Cán bộ Mẫu Một", "CB-00312": "Cán bộ Mẫu Hai"},
			xaB: {"CB-00900": "Cán bộ Xã B"},
		},
		blocs: map[tenant.ID]map[string]identityclient.TaskBlocLabel{
			xaA: {
				"khoi-uy-ban": {Label: "Khối Uỷ ban", Standing: live},
				"khoi-cu":     {Label: "Khối cũ", Standing: removed},
			},
			xaB: {"khoi-b": {Label: "Khối xã B", Standing: live}},
		},
	}
}

// templateDropdown is one column's dropdown as Excel would show it: the values (read from the hidden
// list sheet) and whether Excel STOPS a value outside them.
type templateDropdown struct {
	values []string
	stop   bool
}

var listColumnRe = regexp.MustCompile(`!\$([A-Z]+)\$1:\$[A-Z]+\$(\d+)`)

// templateDropdowns opens a downloaded template, checks the heading and example rows are unchanged, and
// returns its dropdowns keyed by data column letter.
func templateDropdowns(t *testing.T, body []byte) map[string]templateDropdown {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("mẫu không phải xlsx: %v", err)
	}
	defer f.Close()
	if sheets := f.GetSheetList(); sheets[0] != taskImportSheetName {
		t.Fatalf("trang đầu = %q, muốn %q — bộ đọc lấy trang ĐẦU", sheets[0], taskImportSheetName)
	}
	rows, _ := f.GetRows(taskImportSheetName)
	if len(rows) != 2 || strings.Join(rows[0], "|") != strings.Join(domain.TaskImportHeadings, "|") ||
		strings.Join(rows[1], "|") != strings.Join(importTemplateExample, "|") {
		t.Fatalf("tiêu đề / dòng mẫu đổi: %v", rows)
	}
	dvs, err := f.GetDataValidations(taskImportSheetName)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]templateDropdown{}
	if len(dvs) == 0 {
		return out
	}
	if vis, _ := f.GetSheetVisible(xlsx.SheetChoices); vis {
		t.Error("trang danh sách chọn phải ẩn")
	}
	cols, err := f.GetCols(xlsx.SheetChoices)
	if err != nil {
		t.Fatal(err)
	}
	for _, dv := range dvs {
		col := strings.TrimRightFunc(strings.SplitN(dv.Sqref, ":", 2)[0], func(r rune) bool { return r >= '0' && r <= '9' })
		m := listColumnRe.FindStringSubmatch(dv.Formula1)
		if m == nil {
			t.Fatalf("cột %s: công thức danh sách lạ %q", col, dv.Formula1)
		}
		idx, _ := excelize.ColumnNameToNumber(m[1])
		var values []string
		for _, v := range cols[idx-1] {
			if v != "" {
				values = append(values, v)
			}
		}
		out[col] = templateDropdown{values: values, stop: dv.ErrorStyle != nil && *dv.ErrorStyle == "stop"}
	}
	return out
}

func templateQuery(params map[string][]string) string {
	return taskTemplatePath + "?" + url.Values(params).Encode()
}

func TestTaskImportTemplateDropdowns(t *testing.T) {
	m := dungMayChu(t)
	importCreatorGrant(m, t)
	fake := importChoicesSample()
	m.dungLai(t, func(d *Deps) { d.TaskImportChoices = fake })

	// The client's lists: a code of commune B, an unknown code, a duplicate, a removed bloc, blanks — and
	// commune A's codes in an order that is not alphabetical, which the file must keep.
	path := templateQuery(map[string][]string{
		"org_unit": {"van-phong", "tu-phap-b", "dia-chinh", "van-phong", " ", "khong-co"},
		"staff":    {"CB-00312", "CB-00900", "CB-00311", "CB-99999"},
		"bloc":     {"khoi-cu", "khoi-b", "khoi-uy-ban"},
	})
	w := m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	got := templateDropdowns(t, w.Body.Bytes())

	want := map[string]templateDropdown{
		"C": {values: []string{"van-phong · Văn phòng", "dia-chinh · Địa chính"}},
		"D": {values: []string{"CB-00312 · Cán bộ Mẫu Hai", "CB-00311 · Cán bộ Mẫu Một"}},
		"E": {values: []string{"khan · Khẩn", "cao · Cao", "thuong · Thường"}},
		"G": {values: []string{"theo-van-ban · Theo văn bản", "co-ban · Cơ bản"}}, // "viec-cu" is off
		"H": {values: []string{"khoi-uy-ban · Khối Uỷ ban"}},                      // "khoi-cu" is removed
		"L": {values: []string{"x"}, stop: true},
		"M": {values: []string{"x"}, stop: true},
	}
	if len(got) != len(want) {
		t.Errorf("cột có danh sách chọn = %v, muốn %d cột", got, len(want))
	}
	for col, wd := range want {
		g, ok := got[col]
		if !ok || strings.Join(g.values, "|") != strings.Join(wd.values, "|") || g.stop != wd.stop {
			t.Errorf("cột %s = %+v, muốn %+v", col, g, wd)
		}
	}
	for _, d := range got {
		for _, v := range d.values {
			if strings.Contains(v, "xã B") || strings.Contains(v, "CB-00900") || strings.Contains(v, "khong-co") {
				t.Errorf("mẫu xã A lộ mã/tên không thuộc xã A: %q", v)
			}
		}
	}
	// The positions the template and the import share.
	for col, word := range map[int]string{
		domain.TaskImportColUnit: "Đơn vị", domain.TaskImportColAssignee: "Người thực hiện",
		domain.TaskImportColPriority: "Mức ưu tiên", domain.TaskImportColType: "Loại nhiệm vụ",
		domain.TaskImportColBloc: "Khối nhiệm vụ", domain.TaskImportColLeaderApproved: "Lãnh đạo phê duyệt",
		domain.TaskImportColSuperiorAcknowledged: "Cấp trên đã công nhận",
	} {
		if !strings.HasPrefix(domain.TaskImportHeadings[col], word) {
			t.Errorf("cột %d là %q, không phải %q", col, domain.TaskImportHeadings[col], word)
		}
	}
}

func TestTaskImportTemplateTooManyChoices(t *testing.T) {
	for _, param := range []string{"org_unit", "staff", "bloc"} {
		t.Run(param, func(t *testing.T) {
			m := dungMayChu(t)
			importCreatorGrant(m, t)
			fake := importChoicesSample()
			m.dungLai(t, func(d *Deps) { d.TaskImportChoices = fake })

			codes := make([]string, taskImportChoiceCap+1)
			for i := range codes {
				codes[i] = "ma"
			}
			w := m.goi(t, http.MethodGet, hostA, templateQuery(map[string][]string{param: codes}), canBoCuaXa(xaA))
			doiMa(t, w, http.StatusBadRequest)
			if e := loiTra(t, w); e.Code != "too_many_choices" || !strings.Contains(e.Message, "2000") {
				t.Errorf("lỗi = %+v", e)
			}
			if fake.calls != 0 {
				t.Errorf("đã hỏi identity %d lần dù yêu cầu bị từ chối", fake.calls)
			}

			// Exactly the cap is accepted.
			w = m.goi(t, http.MethodGet, hostA, templateQuery(map[string][]string{param: codes[:taskImportChoiceCap]}), canBoCuaXa(xaA))
			doiMa(t, w, http.StatusOK)
		})
	}
}

func TestTaskImportTemplateIdentityDown(t *testing.T) {
	for name, set := range map[string]func(d *Deps){
		"identity lỗi": func(d *Deps) {
			f := importChoicesSample()
			f.err = errors.New("identity: unavailable")
			d.TaskImportChoices = f
		},
		"chưa nối identity": func(d *Deps) { d.TaskImportChoices = nil },
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			importCreatorGrant(m, t)
			m.dungLai(t, set)
			w := m.goi(t, http.MethodGet, hostA, templateQuery(map[string][]string{
				"org_unit": {"dia-chinh"}, "staff": {"CB-00311"}, "bloc": {"khoi-uy-ban"},
			}), canBoCuaXa(xaA))
			doiMa(t, w, http.StatusOK)
			got := templateDropdowns(t, w.Body.Bytes())
			for _, col := range []string{"C", "D", "H"} {
				if _, ok := got[col]; ok {
					t.Errorf("cột %s vẫn có danh sách chọn khi identity không trả lời", col)
				}
			}
			for _, col := range []string{"E", "G", "L", "M"} {
				if _, ok := got[col]; !ok {
					t.Errorf("cột %s mất danh sách chọn — cột này không phụ thuộc identity", col)
				}
			}
		})
	}
}

func TestTaskImportTemplateNoClientCodesNoIdentityCall(t *testing.T) {
	m := dungMayChu(t)
	importCreatorGrant(m, t)
	fake := importChoicesSample()
	m.dungLai(t, func(d *Deps) {
		d.TaskImportChoices = fake
		d.LoaiNhiemVu = &loaiNhiemVuGia{theo: map[tenant.ID][]domain.LoaiNhiemVu{}} // the commune has none
	})
	w := m.goi(t, http.MethodGet, hostA, taskTemplatePath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if fake.calls != 0 {
		t.Errorf("hỏi identity %d lần dù không có mã nào", fake.calls)
	}
	got := templateDropdowns(t, w.Body.Bytes())
	if _, ok := got["G"]; ok {
		t.Error("danh mục loại nhiệm vụ trống mà cột vẫn có danh sách chọn")
	}
	if _, ok := got["E"]; !ok {
		t.Error("cột mức ưu tiên thiếu danh sách chọn")
	}
}

func TestImportChoiceCodesFromQuery(t *testing.T) {
	got, ok := importChoiceCodesFromQuery([]string{" b ", "a", "b", "", strings.Repeat("x", taskImportChoiceCodeMax+1)})
	if !ok || strings.Join(got, ",") != "b,a" {
		t.Errorf("= %v %v, muốn [b a]", got, ok)
	}
	if _, ok := importChoiceCodesFromQuery(make([]string, taskImportChoiceCap+1)); ok {
		t.Error("vượt trần phải bị từ chối, kể cả khi toàn giá trị trống")
	}
}
