package http

// "Loại đơn thư" on the Danh mục screen (bug sheet row 33). WHAT IS PROVEN: rule 5 invariant 7's four
// cases on EACH of the four routes, against the literal seeded key `admin.lookup` (3c); the commune and
// the staff BUSINESS CODE reach the use case; `source` / `tier` / `is_default` / a PATCHed `code` are
// refused, not ignored; a DELETE with no body is the ordinary "no reason" case; a non-statutory code is
// the use case's 400 sentence.

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

const pathLetterTypes = "/api/v1/citizen-letter-types"

// letterTypesFake stands in for both the store read and the write use case. Rows are KEYED BY COMMUNE,
// read from the context as the store reads it.
type letterTypesFake struct {
	mu         sync.Mutex
	rows       map[tenant.ID][]domain.CitizenLetterTypeEntry
	calls      int
	lastTenant tenant.ID
	lastActor  audit.Actor
	lastAdd    app.AddLetterTypeRequest
	lastReason string
	err        error
}

func (f *letterTypesFake) note(ctx context.Context, a audit.Actor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastTenant = tenant.MustFrom(ctx)
	f.lastActor = a
}

func (f *letterTypesFake) find(ctx context.Context, id string) (domain.CitizenLetterTypeEntry, error) {
	for _, e := range f.rows[tenant.MustFrom(ctx)] {
		if e.ID == id {
			return e, nil
		}
	}
	return domain.CitizenLetterTypeEntry{}, docstore.ErrDanhMucKhongTonTai
}

func (f *letterTypesFake) ListEntries(ctx context.Context) ([]domain.CitizenLetterTypeEntry, error) {
	f.note(ctx, audit.Actor{})
	return f.rows[tenant.MustFrom(ctx)], f.err
}

func (f *letterTypesFake) AddType(ctx context.Context, req app.AddLetterTypeRequest, a audit.Actor) (domain.CitizenLetterTypeEntry, error) {
	f.note(ctx, a)
	f.lastAdd = req
	if f.err != nil {
		return domain.CitizenLetterTypeEntry{}, f.err
	}
	return domain.CitizenLetterTypeEntry{ID: "ldt-moi", Code: domain.LetterType(req.Code), Label: req.Label,
		Order: req.Order, Active: true}, nil
}

func (f *letterTypesFake) EditType(ctx context.Context, id string, _ app.EditLetterTypeRequest, a audit.Actor) (domain.CitizenLetterTypeEntry, error) {
	f.note(ctx, a)
	if f.err != nil {
		return domain.CitizenLetterTypeEntry{}, f.err
	}
	return f.find(ctx, id)
}

func (f *letterTypesFake) RemoveType(ctx context.Context, id, reason string, a audit.Actor) error {
	f.note(ctx, a)
	f.lastReason = reason
	if f.err != nil {
		return f.err
	}
	_, err := f.find(ctx, id)
	return err
}

func withLetterTypeRows(m *mayChu) {
	m.letterTypes.rows = map[tenant.ID][]domain.CitizenLetterTypeEntry{
		xaA: {{ID: "ldt-1", Code: domain.LetterTypeComplaint, Label: "Khiếu nại", Order: 1, Active: true}},
		xaB: {{ID: "ldt-b", Code: domain.LetterTypeComplaint, Label: "Khiếu nại (xã B)", Order: 1, Active: true}},
	}
}

func letterTypeRoutes() []motTuyen {
	return []motTuyen{
		{"GET", http.MethodGet, pathLetterTypes, "", http.StatusOK},
		{"POST", http.MethodPost, pathLetterTypes, `{"code":"to-cao","label":"Tố cáo","order":2}`, http.StatusCreated},
		{"PATCH", http.MethodPatch, pathLetterTypes + "/ldt-1", `{"label":"Đơn khiếu nại"}`, http.StatusOK},
		{"DELETE", http.MethodDelete, pathLetterTypes + "/ldt-1", `{"reason":"Trùng"}`, http.StatusNoContent},
	}
}

// The key each route ACTUALLY asks for, compared with the literal seeded key (rule 5, 3c).
func TestLetterTypes_KeyIsTheSeededLiteral(t *testing.T) {
	for _, tc := range letterTypeRoutes() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			withLetterTypeRows(m)
			m.goiThan(t, tc.method, hostA, tc.duong, canBoCua(xaA), tc.than)
			if got := m.checker.hoiKhoaCuoi(); got != "admin.lookup" {
				t.Fatalf("tuyến hỏi khoá %q, muốn \"admin.lookup\"", got)
			}
		})
	}
}

func TestLetterTypes_401WithoutSession(t *testing.T) {
	for _, tc := range letterTypeRoutes() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			withLetterTypeRows(m)
			m.capQuyen(xaA, QuyenDanhMuc)
			doiMa(t, m.goiThan(t, tc.method, hostA, tc.duong, nil, tc.than), http.StatusUnauthorized)
			if m.letterTypes.calls != 0 {
				t.Fatal("chưa đăng nhập mà use case đã chạy")
			}
		})
	}
}

func TestLetterTypes_403WrongPermission(t *testing.T) {
	for _, tc := range letterTypeRoutes() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			withLetterTypeRows(m)
			// A real key of the register itself: may book letters, may not rewrite the catalogue.
			m.capQuyen(xaA, "petition.create", "petition.read")
			doiMa(t, m.goiThan(t, tc.method, hostA, tc.duong, canBoCua(xaA), tc.than), http.StatusForbidden)
			if m.letterTypes.calls != 0 {
				t.Fatal("sai quyền mà use case vẫn chạy")
			}
		})
	}
}

func TestLetterTypes_403RightPermissionOtherCommune(t *testing.T) {
	for _, tc := range letterTypeRoutes() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			withLetterTypeRows(m)
			m.capQuyen(xaA, QuyenDanhMuc) // granted in A; the account is B's, signed in at B
			doiMa(t, m.goiThan(t, tc.method, hostB, tc.duong, canBoCua(xaB), tc.than), http.StatusForbidden)
			if m.letterTypes.calls != 0 {
				t.Fatal("quyền cấp ở xã khác mà vẫn chạy được ở xã này")
			}
		})
	}
}

func TestLetterTypes_OKRightPermissionRightCommune(t *testing.T) {
	for _, tc := range letterTypeRoutes() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			withLetterTypeRows(m)
			m.capQuyen(xaA, QuyenDanhMuc)
			doiMa(t, m.goiThan(t, tc.method, hostA, tc.duong, canBoCua(xaA), tc.than), tc.ok)
			if m.letterTypes.calls != 1 || m.letterTypes.lastTenant != xaA {
				t.Fatalf("use case chạy %d lần ở xã %q", m.letterTypes.calls, m.letterTypes.lastTenant)
			}
			if tc.method != http.MethodGet && m.letterTypes.lastActor.ID != maCanBo {
				t.Fatalf("người thực hiện = %q, muốn mã cán bộ %q", m.letterTypes.lastActor.ID, maCanBo)
			}
		})
	}
}

func TestLetterTypes_ListHasTheDocumentTypeShape(t *testing.T) {
	m := dungMayChu(t)
	withLetterTypeRows(m)
	m.capQuyen(xaA, QuyenDanhMuc)
	w := m.goi(t, http.MethodGet, hostA, pathLetterTypes, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	var out struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 1 {
		t.Fatalf("thân: %s", w.Body.String())
	}
	row := out.Items[0]
	for k, want := range map[string]string{"code": `"khieu-nai"`, "label": `"Khiếu nại"`, "source": `"don-vi"`,
		"tier": `1`, "is_default": `false`, "active": `true`, "color": `null`} {
		if string(row[k]) != want {
			t.Errorf("%s = %s, muốn %s", k, row[k], want)
		}
	}
}

func TestLetterTypes_ClientMayNotStateProvenanceDefaultOrCode(t *testing.T) {
	for _, tc := range []struct{ name, method, path, body string }{
		{"POST source", http.MethodPost, pathLetterTypes, `{"code":"to-cao","label":"Tố cáo","source":"he-thong"}`},
		{"POST is_default", http.MethodPost, pathLetterTypes, `{"code":"to-cao","label":"Tố cáo","is_default":true}`},
		{"PATCH tier", http.MethodPatch, pathLetterTypes + "/ldt-1", `{"tier":2}`},
		{"PATCH code", http.MethodPatch, pathLetterTypes + "/ldt-1", `{"code":"to-cao"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChu(t)
			withLetterTypeRows(m)
			m.capQuyen(xaA, QuyenDanhMuc)
			doiMa(t, m.goiThan(t, tc.method, hostA, tc.path, canBoCua(xaA), tc.body), http.StatusBadRequest)
			if m.letterTypes.calls != 0 {
				t.Fatal("trường bị từ chối mà use case vẫn chạy")
			}
		})
	}
}

func TestLetterTypes_DeleteWithoutBodyIsNoReason(t *testing.T) {
	m := dungMayChu(t)
	withLetterTypeRows(m)
	m.capQuyen(xaA, QuyenDanhMuc)
	doiMa(t, m.goiThan(t, http.MethodDelete, hostA, pathLetterTypes+"/ldt-1", canBoCua(xaA), ""), http.StatusNoContent)
	if m.letterTypes.calls != 1 || m.letterTypes.lastReason != "" {
		t.Fatalf("xoá không lý do: gọi %d lần, lý do %q", m.letterTypes.calls, m.letterTypes.lastReason)
	}
}

func TestLetterTypes_RefusalsMapToStatus(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{domain.ErrLetterTypeCodeNotStatutory, http.StatusBadRequest},
		{domain.ErrNhanTrong, http.StatusBadRequest},
		{docstore.ErrMaDaTonTai, http.StatusConflict},
		{docstore.ErrDanhMucKhongTonTai, http.StatusNotFound},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, QuyenDanhMuc)
			m.letterTypes.err = tc.err
			doiMa(t, m.goiThan(t, http.MethodPost, hostA, pathLetterTypes, canBoCua(xaA),
				`{"code":"phan-anh","label":"Phản ánh"}`), tc.want)
		})
	}
}
