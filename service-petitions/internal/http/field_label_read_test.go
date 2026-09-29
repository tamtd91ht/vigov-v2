package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/app"
)

// The four READ routes that render a field label — staff detail and list, citizen detail and list.
// Labels are the commune's wording else the platform default (app.EffectiveFieldLabels, proved in
// internal/app/intake_field_test.go). When platform cannot be read, a read route answers 503
// `field_catalogue_unavailable` rather than show a raw code (ADR 0060 §3); any other failure stays 500.

func platformDown() error { return fmt.Errorf("%w: unreachable", app.ErrFieldCatalogueUnavailable) }

func TestStaffPetitionReadsAnswer503WhenLabelsUnavailable(t *testing.T) {
	for name, path := range map[string]string{
		"detail": duong(maPhieuThuong),
		"list":   "/api/v1/citizen-reports",
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.nhan.loi = platformDown()
			w := m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA))
			doiMa(t, w, http.StatusServiceUnavailable)
			if e := loiTra(t, w); e.Code != "field_catalogue_unavailable" {
				t.Errorf("error key %q", e.Code)
			}
		})
	}
}

func TestStaffPetitionReadsOtherLabelFailureStays500(t *testing.T) {
	m := dungMayChu(t)
	m.nhan.loi = errors.New("pg: connection refused")
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "connection refused") {
		t.Error("internal error text leaked")
	}
}

func TestCitizenPetitionReadsAnswer503WhenLabelsUnavailable(t *testing.T) {
	m := dungMayChuGui(t)
	w := m.gui(t, bodyWithField(`"rac-thai"`), tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)
	var filed struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &filed); err != nil || filed.Code == "" {
		t.Fatalf("no code: %s", w.Body.String())
	}

	m.nhan.loi = platformDown()
	for name, path := range map[string]string{
		"detail": duongTapCongDan + "/" + filed.Code,
		"list":   duongTapCongDan,
	} {
		t.Run(name, func(t *testing.T) {
			r := m.doc(t, path, tokenCuaToi)
			doiMa(t, r, http.StatusServiceUnavailable)
			if e := loiTra(t, r); e.Code != "field_catalogue_unavailable" {
				t.Errorf("error key %q", e.Code)
			}
		})
	}
}
