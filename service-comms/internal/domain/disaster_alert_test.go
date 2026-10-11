package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC)

func validAlert() DisasterAlert {
	cap0 := 0
	return DisasterAlert{
		Kind: DisasterKindFlood, Title: "  Lũ sông Hồng ", Detail: "Sơ tán\nngay", EffectiveFrom: t0,
		DangerZone:        []DangerZonePoint{{0, 0}, {100, 0}, {50, 100}},
		EvacuationPoints:  []EvacuationPoint{{Name: "Trường tiểu học", Phone: " 0900000000 ", Capacity: &cap0}},
		EmergencyContacts: []EmergencyContact{{Role: "Trực ban", Name: "Nguyễn Văn A", Phone: "0900000000"}},
	}
}

func TestNormalizeDisasterAlertAcceptsAndCleans(t *testing.T) {
	a, err := NormalizeDisasterAlert(validAlert())
	if err != nil {
		t.Fatalf("valid alert refused: %v", err)
	}
	if a.Title != "Lũ sông Hồng" || a.EvacuationPoints[0].Phone != "0900000000" || a.Level != "" {
		t.Errorf("cleaned = %+v", a)
	}
	empty := validAlert()
	empty.DangerZone, empty.EvacuationPoints, empty.EmergencyContacts = nil, nil, nil
	if a, err := NormalizeDisasterAlert(empty); err != nil || a.DangerZone == nil || a.EvacuationPoints == nil ||
		a.EmergencyContacts == nil {
		t.Errorf("nil lists must become empty lists (NOT NULL arrays): %+v %v", a, err)
	}
}

func TestNormalizeDisasterAlertRefusals(t *testing.T) {
	neg := -1
	for _, c := range []struct {
		name string
		mut  func(*DisasterAlert)
		want error
	}{
		{"english kind", func(a *DisasterAlert) { a.Kind = "flood" }, ErrDisasterKindInvalid},
		{"no title", func(a *DisasterAlert) { a.Title = " " }, ErrDisasterTitleEmpty},
		{"long title", func(a *DisasterAlert) { a.Title = strings.Repeat("a", 256) }, ErrDisasterTitleTooLong},
		{"long level", func(a *DisasterAlert) { a.Level = strings.Repeat("a", 101) }, ErrDisasterLevelInvalid},
		{"control in headline", func(a *DisasterAlert) { a.Headline = "a\x07" }, ErrDisasterHeadlineInvalid},
		{"no detail", func(a *DisasterAlert) { a.Detail = "" }, ErrDisasterDetailEmpty},
		{"no start", func(a *DisasterAlert) { a.EffectiveFrom = time.Time{} }, ErrDisasterFromMissing},
		// 0026's CHECK is STRICT: an end equal to the start is refused, not only an earlier one.
		{"end equals start", func(a *DisasterAlert) { a.EffectiveTo = t0 }, ErrDisasterWindowInverted},
		{"end before start", func(a *DisasterAlert) { a.EffectiveTo = t0.Add(-time.Hour) }, ErrDisasterWindowInverted},
		{"two-point zone", func(a *DisasterAlert) { a.DangerZone = a.DangerZone[:2] }, ErrDisasterDangerZone},
		{"x over 100", func(a *DisasterAlert) { a.DangerZone[0].X = 100.5 }, ErrDisasterDangerZone},
		{"y negative", func(a *DisasterAlert) { a.DangerZone[1].Y = -1 }, ErrDisasterDangerZone},
		{"NaN", func(a *DisasterAlert) { a.DangerZone[2].X = math.NaN() }, ErrDisasterDangerZone},
		{"201 points", func(a *DisasterAlert) { a.DangerZone = make([]DangerZonePoint, 201) }, ErrDisasterDangerZone},
		{"nameless shelter", func(a *DisasterAlert) { a.EvacuationPoints[0].Name = "" }, ErrDisasterEvacuationPoints},
		{"negative capacity", func(a *DisasterAlert) { a.EvacuationPoints[0].Capacity = &neg }, ErrDisasterEvacuationPoints},
		{"shelter phone with letters", func(a *DisasterAlert) { a.EvacuationPoints[0].Phone = "máy lẻ 12" }, ErrDisasterEvacuationPoints},
		{"51 shelters", func(a *DisasterAlert) { a.EvacuationPoints = make([]EvacuationPoint, 51) }, ErrDisasterEvacuationPoints},
		{"contact without role", func(a *DisasterAlert) { a.EmergencyContacts[0].Role = "" }, ErrDisasterContacts},
		{"contact without name", func(a *DisasterAlert) { a.EmergencyContacts[0].Name = "" }, ErrDisasterContacts},
		{"contact without phone", func(a *DisasterAlert) { a.EmergencyContacts[0].Phone = "" }, ErrDisasterContacts},
	} {
		a := validAlert()
		c.mut(&a)
		_, err := NormalizeDisasterAlert(a)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
			continue
		}
		// The sentence never echoes a value — a phone or a name typed in a list.
		if strings.Contains(err.Error(), "0900000000") || strings.Contains(err.Error(), "Nguyễn") {
			t.Errorf("%s: the refusal quotes a value: %v", c.name, err)
		}
	}
}

func TestDisasterAlertInForce(t *testing.T) {
	a := DisasterAlert{EffectiveFrom: t0, EffectiveTo: t0.Add(2 * time.Hour)}
	for _, c := range []struct {
		at   time.Time
		want bool
	}{{t0.Add(-time.Second), false}, {t0, true}, {t0.Add(time.Hour), true}, {t0.Add(2 * time.Hour), false}} {
		if got := a.InForce(c.at); got != c.want {
			t.Errorf("at %v: %v, want %v", c.at, got, c.want)
		}
	}
	open := DisasterAlert{EffectiveFrom: t0}
	if !open.InForce(t0.Add(1000 * time.Hour)) {
		t.Error("an open-ended alert stays in force")
	}
	ended := DisasterAlert{EffectiveFrom: t0, EndedAt: t0.Add(time.Minute)}
	if ended.InForce(t0.Add(time.Hour)) || !ended.Ended() {
		t.Error("an ended alert is not in force")
	}
}
