package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// `color` on the two identity catalogues' write and read DTOs (migration 0026). The permission cases
// are the routes' existing ones (danh_muc_ghi_test.go); what is asserted here is the wire mapping.

func TestCatalogueColorCreatePassesValue(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goiIdem(t, "POST", hostA, duongLoaiDonViDanCu, `{"code":"khu-pho","label":"Khu phố","color":"#1F6FEB"}`, tok)
	if w.Code != http.StatusCreated {
		t.Fatalf("mã = %d — %s", w.Code, w.Body.String())
	}
	if c := m.ghiLoaiDonViDanCu.themCuoi.Color; c == nil || *c != "#1F6FEB" {
		t.Errorf("màu tới use case = %v", c)
	}
	w = m.goiIdem(t, "POST", hostA, duongKhoiNhiemVu, `{"code":"khoi-moi","label":"Khối mới"}`, tok)
	if w.Code != http.StatusCreated || m.ghiKhoiNhiemVu.themCuoi.Color != nil {
		t.Errorf("không gửi màu: mã %d, màu tới use case %v", w.Code, m.ghiKhoiNhiemVu.themCuoi.Color)
	}
}

// PATCH: absent = leave (nil change); null = clear (change with nil); a string = set.
func TestCatalogueColorPatchThreeStates(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaA, sidA)
	duong := duongKhoiNhiemVu + "/muc-001"

	m.goiIdem(t, "PATCH", hostA, duong, `{"label":"Nhãn"}`, tok)
	if m.ghiKhoiNhiemVu.suaCuoi.Color != nil {
		t.Errorf("vắng màu: change = %+v, muốn nil", m.ghiKhoiNhiemVu.suaCuoi.Color)
	}
	m.goiIdem(t, "PATCH", hostA, duong, `{"color":null}`, tok)
	if c := m.ghiKhoiNhiemVu.suaCuoi.Color; c == nil || c.Color != nil {
		t.Errorf("null: change = %+v, muốn xoá màu", c)
	}
	m.goiIdem(t, "PATCH", hostA, duong, `{"color":"#abcdef"}`, tok)
	if c := m.ghiKhoiNhiemVu.suaCuoi.Color; c == nil || c.Color == nil || *c.Color != "#abcdef" {
		t.Errorf("chuỗi: change = %+v", c)
	}
}

func TestCatalogueColorInvalidIs400(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	m.ghiLoaiDonViDanCu.loi = domain.ErrCatalogueColorInvalid
	w := m.goiIdem(t, "POST", hostA, duongLoaiDonViDanCu, `{"code":"khu-pho","label":"Khu phố","color":"red"}`,
		m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusBadRequest {
		t.Errorf("mã = %d, muốn 400 — %s", w.Code, w.Body.String())
	}
}

// THE ROW LEAVES WITH `color`: the value when set, null — never "" — when not.
func TestCatalogueColorOut(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	m.ghiLoaiDonViDanCu.ra.Color = "#112233"
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goiIdem(t, "PATCH", hostA, duongLoaiDonViDanCu+"/muc-001", `{"color":"#112233"}`, tok)
	var row map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil || row["color"] != "#112233" {
		t.Errorf("thân = %s", w.Body.String())
	}
	w = m.goiIdem(t, "PATCH", hostA, duongKhoiNhiemVu+"/muc-001", `{"label":"X"}`, tok)
	row = nil
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil {
		t.Fatal(err)
	}
	if v, ok := row["color"]; !ok || v != nil {
		t.Errorf("không màu: color = %v (có khoá: %v), muốn null", v, ok)
	}
}
