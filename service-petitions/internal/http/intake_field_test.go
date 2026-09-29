package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/app"
)

// POST /api/v1/my-citizen-reports WITH `field` (ADR 0050 point 1), on the gui_phan_anh_test.go harness
// (real citizen chain, idem, per-commune offered fields in the use-case fake).
//
//	PROVED HERE   an offered field reaches the use case trimmed and comes back with its label and
//	              resolve deadline · every field the commune does not offer — unknown, `can-bo`,
//	              another commune's code, blank — gets ONE identical 400 body · `linh_vuc` stays
//	              refused · platform down is 503 field_catalogue_unavailable and no code leaves ·
//	              a commune with no SLA is still 503 intake_not_configured · a label read that fails
//	              AFTER commit never costs the citizen their lookup code.

func bodyWithField(field string) string {
	return `{"content":"Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.","field":` + field + `}`
}

func TestIntakeFieldOfferedIsFiledWithLabelAndResolveDeadline(t *testing.T) {
	m := dungMayChuGui(t)
	w := m.gui(t, bodyWithField(`"  rac-thai "`), tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)

	if len(m.so.thayYeuCau) != 1 || m.so.thayYeuCau[0].Field != "rac-thai" {
		t.Fatalf("use case got %+v, want Field \"rac-thai\" (trimmed)", m.so.thayYeuCau)
	}
	ra := docPhieuCuaToi(t, w.Body.Bytes())
	if ra.Field != "rac-thai" || ra.FieldLabel != "Rác thải – Vệ sinh môi trường" {
		t.Errorf("field=%q label=%q", ra.Field, ra.FieldLabel)
	}
	if !strings.Contains(w.Body.String(), `"resolve_due":"2026-09-25T03:17:00Z"`) {
		t.Errorf("resolve deadline fixed at intake not returned: %s", w.Body.String())
	}
}

func TestIntakeFieldNotOfferedOneIdenticalAnswer(t *testing.T) {
	cases := map[string]struct{ body, token string }{
		"unknown code":           {bodyWithField(`"khong-co"`), tokenCuaToi},
		"staff conduct":          {bodyWithField(`"can-bo"`), tokenCuaToi},
		"another commune's code": {bodyWithField(`"rac-thai"`), tokenXaB},
		"blank":                  {bodyWithField(`"   "`), tokenCuaToi},
		"empty":                  {bodyWithField(`""`), tokenCuaToi},
	}
	var first string
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuGui(t)
			w := m.gui(t, c.body, c.token, khoaThu)
			doiMa(t, w, http.StatusBadRequest)
			if e := loiTra(t, w); e.Code != "field_not_offered" {
				t.Errorf("error key %q", e.Code)
			}
			// BYTE-FOR-BYTE the same body: telling the causes apart would say which codes exist and
			// which this commune switched off.
			if first == "" {
				first = w.Body.String()
			} else if w.Body.String() != first {
				t.Errorf("answers differ:\n%s\n%s", first, w.Body.String())
			}
			if m.so.dem != 0 {
				t.Error("a refused field produced a petition")
			}
		})
	}
}

func TestIntakeVietnameseFieldSpellingStaysRefused(t *testing.T) {
	m := dungMayChuGui(t)
	doiMa(t, m.gui(t, `{"content":"x","linh_vuc":"rac-thai"}`, tokenCuaToi, khoaThu), http.StatusBadRequest)
	if m.so.demGui != 0 {
		t.Error("`linh_vuc` reached the use case")
	}
}

func TestIntakeFieldCatalogueUnavailableIs503AndNoCode(t *testing.T) {
	m := dungMayChuGui(t)
	m.so.loi = fmt.Errorf("%w: platform down", app.ErrFieldCatalogueUnavailable)
	w := m.gui(t, bodyWithField(`"rac-thai"`), tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusServiceUnavailable)
	if e := loiTra(t, w); e.Code != "field_catalogue_unavailable" {
		t.Errorf("error key %q", e.Code)
	}
	if strings.Contains(w.Body.String(), "PA-") {
		t.Error("a lookup code left on a refused intake")
	}
}

func TestIntakeWithFieldNoSLAStaysIntakeNotConfigured(t *testing.T) {
	m := dungMayChuGui(t)
	m.so.loi = fmt.Errorf("%w: no sla row", app.ErrChuaAnDinhDuocHan)
	w := m.gui(t, bodyWithField(`"rac-thai"`), tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusServiceUnavailable)
	if e := loiTra(t, w); e.Code != "intake_not_configured" {
		t.Errorf("error key %q", e.Code)
	}
}

func TestIntakeLabelFailureAfterCommitStillReturnsTheCode(t *testing.T) {
	// Rule 10, invariant 1: the petition exists, so its code must reach the citizen. The label is
	// cosmetic and goes out empty.
	m := dungMayChuGui(t)
	m.nhan.loi = errors.New("platform down")
	w := m.gui(t, bodyWithField(`"rac-thai"`), tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)
	ra := docPhieuCuaToi(t, w.Body.Bytes())
	if ra.Code == "" || ra.Field != "rac-thai" || ra.FieldLabel != "" {
		t.Errorf("code=%q field=%q label=%q", ra.Code, ra.Field, ra.FieldLabel)
	}
}

func TestIntakeWithoutFieldReadsNoLabel(t *testing.T) {
	m := dungMayChuGui(t)
	doiMa(t, m.gui(t, thanThu, tokenCuaToi, khoaThu), http.StatusCreated)
	if m.nhan.goi != 0 {
		t.Error("a petition with no field triggered a catalogue read")
	}
}
