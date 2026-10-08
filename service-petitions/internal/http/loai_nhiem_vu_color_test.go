package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// `color` on the wire of /api/v1/task-types (migration 0033, ADR 0079 row 5). The permission cases are the
// routes' existing four-case suite; what is asserted here is the field mapping and its three states.

func TestColorWireTaskTypes(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, "admin.lookup")
	p := canBoGhi(xaA)

	w := m.goiThan(t, http.MethodPost, hostA, "/api/v1/task-types", p, `{"code":"mau-moi","label":"Màu mới","color":"#1F6FEB"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST = %d — %s", w.Code, w.Body.String())
	}
	if c := m.ghi.themCuoi.Color; c == nil || *c != "#1F6FEB" {
		t.Errorf("colour reaching the use case = %v", c)
	}

	one := "/api/v1/task-types/x-001"
	m.goiThan(t, http.MethodPatch, hostA, one, p, `{"label":"Nhãn"}`)
	if m.ghi.suaCuoi.Color != nil {
		t.Errorf("absent: change = %+v, want nil (leave it)", m.ghi.suaCuoi.Color)
	}
	m.goiThan(t, http.MethodPatch, hostA, one, p, `{"color":null}`)
	if c := m.ghi.suaCuoi.Color; c == nil || c.Color != nil {
		t.Errorf("null: change = %+v, want clear", c)
	}
	m.ghi.ra.Color = "#112233"
	w = m.goiThan(t, http.MethodPatch, hostA, one, p, `{"color":"#112233"}`)
	if c := m.ghi.suaCuoi.Color; c == nil || c.Color == nil || *c.Color != "#112233" {
		t.Errorf("string: change = %+v", c)
	}
	var row map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil || row["color"] != "#112233" {
		t.Errorf("row out = %s", w.Body.String())
	}

	// No colour leaves as null — never "" a client could hand to a style attribute.
	m.ghi.ra.Color = ""
	w = m.goiThan(t, http.MethodPatch, hostA, one, p, `{"label":"X"}`)
	row = nil
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil {
		t.Fatal(err)
	}
	if v, ok := row["color"]; !ok || v != nil {
		t.Errorf("no colour: color = %v (present %v), want null", v, ok)
	}

	m.ghi.loi = domain.ErrCatalogueColorInvalid
	if w := m.goiThan(t, http.MethodPatch, hostA, one, p, `{"color":"red"}`); w.Code != http.StatusBadRequest {
		t.Errorf("malformed colour = %d, want 400 — %s", w.Code, w.Body.String())
	}
}
