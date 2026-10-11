package domain

import (
	"errors"
	"testing"
)

// The table in NormaliseLocationCitizenAdjusted's doc, row for row (migration 0044).
func TestNormaliseLocationCitizenAdjusted(t *testing.T) {
	lat, lng := 16.0544, 108.2022
	yes, no := true, false
	for name, c := range map[string]struct {
		lat, lng *float64
		sent     *bool
		want     *bool
		err      error
	}{
		"absent, coordinates":    {&lat, &lng, nil, nil, nil},
		"absent, none":           {nil, nil, nil, nil, nil},
		"true, coordinates":      {&lat, &lng, &yes, &yes, nil},
		"true, none":             {nil, nil, &yes, nil, ErrLocationAdjustedWithoutCoordinates},
		"false, coordinates":     {&lat, &lng, &no, &no, nil},
		"false, none":            {nil, nil, &no, nil, nil},
		"true, lat only (guard)": {&lat, nil, &yes, nil, ErrLocationAdjustedWithoutCoordinates},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := NormaliseLocationCitizenAdjusted(c.lat, c.lng, c.sent)
			if !errors.Is(err, c.err) {
				t.Fatalf("err = %v, want %v", err, c.err)
			}
			switch {
			case c.want == nil && got != nil:
				t.Errorf("got %v, want nil — not recorded must never become a value", *got)
			case c.want != nil && (got == nil || *got != *c.want):
				t.Errorf("got %v, want %v", got, *c.want)
			}
			// The result must not alias the caller's pointer: the request value is the client's.
			if got != nil && c.sent != nil && got == c.sent {
				t.Error("result aliases the request pointer")
			}
		})
	}
}

func TestLocationAdjustedRefusalIsASenderError(t *testing.T) {
	if !LaLoiGuiPhanAnh(ErrLocationAdjustedWithoutCoordinates) {
		t.Error("ErrLocationAdjustedWithoutCoordinates is not listed as the sender's fault")
	}
}
