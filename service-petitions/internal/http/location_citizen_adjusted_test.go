package http

import (
	"encoding/json"
	"net/http"
	"testing"
)

// `location_citizen_adjusted` (migration 0044, ADR 0072 §Sửa đổi 11/10/2026 (Mini App) M3) on the
// citizen intake and the staff read.
//
//	PROVED HERE   true / false / absent reach the use case as true / false / nil — absent is NEVER
//	              turned into false · true without lat/lng is 422 `location_adjusted_without_coordinates`
//	              and creates no petition · the citizen's 201 does not carry the key · the staff read
//	              carries true and false as themselves and OMITS the key when not recorded.
//	NOT PROVED    the INSERT and the Scan — internal/store owns those against its fake driver.

const adjustedPre = `{"content":"Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.","lat":16.0544,"lng":108.2022`

func TestIntakeLocationCitizenAdjustedReachesUseCase(t *testing.T) {
	yes, no := true, false
	for name, c := range map[string]struct {
		body string
		want *bool
	}{
		"true":   {adjustedPre + `,"location_citizen_adjusted":true}`, &yes},
		"false":  {adjustedPre + `,"location_citizen_adjusted":false}`, &no},
		"absent": {adjustedPre + `}`, nil},
		"null":   {adjustedPre + `,"location_citizen_adjusted":null}`, nil},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuGui(t)
			w := m.gui(t, c.body, tokenCuaToi, khoaThu)
			doiMa(t, w, http.StatusCreated)

			got := m.so.thayYeuCau[0].LocationCitizenAdjusted
			switch {
			case c.want == nil && got != nil:
				t.Errorf("use case got %v, want nil — an unsaid flag must stay unsaid, never false", *got)
			case c.want != nil && (got == nil || *got != *c.want):
				t.Errorf("use case got %v, want %v", got, *c.want)
			}

			// The citizen's own receipt does not carry it (staff surface only).
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
				t.Fatalf("body is not JSON: %s", w.Body.String())
			}
			if _, has := raw["location_citizen_adjusted"]; has {
				t.Errorf("citizen 201 carries location_citizen_adjusted: %s", w.Body.String())
			}
		})
	}
}

func TestIntakeLocationAdjustedWithoutCoordinatesIs422(t *testing.T) {
	m := dungMayChuGui(t)
	w := m.gui(t, `{"content":"Đống rác ở đầu ngõ.","location_citizen_adjusted":true}`, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusUnprocessableEntity)
	if e := loiTra(t, w); e.Code != "location_adjusted_without_coordinates" {
		t.Errorf("error key %q, want location_adjusted_without_coordinates", e.Code)
	}
	if m.so.dem != 0 {
		t.Errorf("created %d petitions for a contradictory location", m.so.dem)
	}
}

// TestIntakeLocationAdjustedFalseWithoutCoordinatesIsNotRecorded: false says "Zalo's coordinates,
// untouched" — about no coordinates it says nothing, so nothing is stored (NULL), and it is not refused.
func TestIntakeLocationAdjustedFalseWithoutCoordinatesIsNotRecorded(t *testing.T) {
	m := dungMayChuGui(t)
	w := m.gui(t, `{"content":"Đống rác ở đầu ngõ.","location_citizen_adjusted":false}`, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)
	if m.so.dem != 1 {
		t.Fatalf("created %d petitions, want 1", m.so.dem)
	}
	for _, p := range m.so.theo[m.so.thayXa[0]] {
		if p.LocationCitizenAdjusted != nil {
			t.Errorf("stored %v without coordinates, want nil", *p.LocationCitizenAdjusted)
		}
	}
}

// --- staff read --------------------------------------------------------------------------------

func TestStaffReadCarriesLocationCitizenAdjusted(t *testing.T) {
	yes, no := true, false
	for name, c := range map[string]struct {
		set  *bool
		want string // the raw JSON value, "" = key absent
	}{
		"citizen moved the pin": {&yes, "true"},
		"zalo untouched":        {&no, "false"},
		"not recorded":          {nil, ""},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t) // grants feedback.read
			withSceneLocation(m, xaA, maPhieuThuong, 16.0544, 108.2022)
			p := m.phieu.theo[xaA][maPhieuThuong]
			p.LocationCitizenAdjusted = c.set
			m.phieu.theo[xaA][maPhieuThuong] = p

			w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
			doiMa(t, w, http.StatusOK)

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
				t.Fatalf("body is not JSON: %s", w.Body.String())
			}
			got, has := raw["location_citizen_adjusted"]
			switch {
			case c.want == "" && has:
				t.Errorf("key present (%s) on a petition where it was not recorded — absent must not read as a value", got)
			case c.want != "" && string(got) != c.want:
				t.Errorf("location_citizen_adjusted = %q, want %s", got, c.want)
			}
		})
	}
}
