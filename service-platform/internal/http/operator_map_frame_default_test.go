package http

// A commune's default map frame in the operator area (ADR 0072 amendment 2, K1–K2). 401 / 401 staff
// token / 403 / 503 / 2xx for both routes are in TestGuardedRoutes; this file defends what is specific:
// the hard bounds, the server-side confirmation of an unusual radius, 404 / 409, and what reaches the
// audited store (target commune from the path, VH- code, IP, reason, confirmation).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

const inactiveCommuneIDFake = "01JD8ZQK9M3NPXR7TVWYB2C4EH"

// mapFramesFake is the store: communeIDFake active, inactiveCommuneIDFake inactive, any other id
// unknown. It records every write's target (from ctx), actor, reason and confirmation.
type mapFramesFake struct {
	frames       map[tenant.ID]domain.MapFrameDefault
	writes       int
	target       tenant.ID
	actor        domain.OperatorActor
	reason       string
	acknowledged bool
	saw          domain.MapFrameDefault
}

func newMapFramesFake() *mapFramesFake {
	return &mapFramesFake{frames: map[tenant.ID]domain.MapFrameDefault{}}
}

func (f *mapFramesFake) known(ctx context.Context) (tenant.ID, error) {
	id := tenant.MustFrom(ctx)
	if id != communeIDFake && id != inactiveCommuneIDFake {
		return id, store.ErrCommuneNotFound
	}
	return id, nil
}

func (f *mapFramesFake) OperatorMapFrameDefault(ctx context.Context) (domain.MapFrameDefault, bool, error) {
	id, err := f.known(ctx)
	if err != nil {
		return domain.MapFrameDefault{}, false, err
	}
	v, ok := f.frames[id]
	return v, ok, nil
}

func (f *mapFramesFake) SetMapFrameDefault(ctx context.Context, next domain.MapFrameDefault, acknowledged bool,
	reason string, by domain.OperatorActor) (domain.MapFrameDefault, bool, error) {
	f.writes++
	f.target, f.actor, f.reason, f.acknowledged, f.saw = tenant.MustFrom(ctx), by, reason, acknowledged, next
	id, err := f.known(ctx)
	if err != nil {
		return domain.MapFrameDefault{}, false, err
	}
	if id == inactiveCommuneIDFake {
		return domain.MapFrameDefault{}, false, store.ErrCommuneInactive
	}
	next.UpdatedBy, next.UpdatedAt = by.Code, nowFake
	f.frames[id] = next
	return next, true, nil
}

func mapFramePath(id string) string { return "/api/v1/communes/" + id + "/map-frame-default" }

func TestGetMapFrameDefault(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"} // any one key reads (ADR 0073 #1)

	rec := h.do("GET", mapFramePath(communeIDFake), "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	// Not configured: no centre, no bounds — never a built-in default — but the form's hints.
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"center_lat", "center_lng", "radius_km", "bounds", "updated_at", "updated_by"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("not configured but %q present: %s", absent, rec.Body)
		}
	}
	if raw["configured"] != false || raw["recommended_radius_km"] != 10.0 || raw["max_radius_km"] != 50.0 {
		t.Errorf("hints: %s", rec.Body)
	}
	if u, _ := raw["usual_radius_km"].([]any); len(u) != 2 || u[0] != 3.0 || u[1] != 20.0 {
		t.Errorf("usual_radius_km: %v", raw["usual_radius_km"])
	}

	h.frames.frames[communeIDFake] = domain.MapFrameDefault{CenterLat: 16, CenterLng: 108, RadiusKm: 11.132,
		UpdatedAt: time.Date(2026, 10, 4, 9, 0, 0, 0, time.FixedZone("ICT", 7*3600)), UpdatedBy: "VH-00002"}
	rec = h.do("GET", mapFramePath(communeIDFake), "", opCookie(t))
	var got mapFrameDefaultView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Configured || got.RadiusKm == nil || *got.RadiusKm != 11.132 || got.UpdatedBy != "VH-00002" ||
		got.UpdatedAt == nil || got.UpdatedAt.Location() != time.UTC {
		t.Fatalf("configured view: %s", rec.Body)
	}
	// [minLng, minLat, maxLng, maxLat], longitude first, enclosing the circle.
	if len(got.Bounds) != 4 || !(got.Bounds[0] < 107.9 && got.Bounds[1] <= 15.9 && got.Bounds[2] > 108.1 && got.Bounds[3] >= 16.1) {
		t.Errorf("bounds %v", got.Bounds)
	}

	for path, want := range map[string]int{
		mapFramePath("01JD8ZQK9M3NPXR7TVWYB2C4EZ"): 404, // unknown
		mapFramePath("not-a-ulid"):                 404, // malformed = unknown
		mapFramePath(inactiveCommuneIDFake):        200, // rule 7: an inactive commune's configuration is still readable
	} {
		if rec := h.do("GET", path, "", opCookie(t)); rec.Code != want {
			t.Errorf("%s: %d %s, want %d", path, rec.Code, rec.Body, want)
		}
	}
}

// The hard bounds (K2) and the server-side confirmation of an unusual radius. Every refusal is decided
// before the store.
func TestSetMapFrameDefaultValidation(t *testing.T) {
	body := func(lat, lng, r string, ack string) string {
		b := `{"center_lat":` + lat + `,"center_lng":` + lng + `,"radius_km":` + r
		if ack != "" {
			b += `,"acknowledged_unusual":` + ack
		}
		return b + `,"reason":"Đặt khung mặc định"}`
	}
	const lat, lng = "15.730507", "108.37811"
	for _, c := range []struct {
		name, body string
		want       int
		code       string
	}{
		{"radius 0", body(lat, lng, "0", "true"), 422, "radius_out_of_range"},
		{"radius rounds to 0", body(lat, lng, "0.04", "true"), 422, "radius_out_of_range"},
		{"radius 0.1 unconfirmed", body(lat, lng, "0.1", ""), 422, "radius_unusual_unconfirmed"},
		{"radius 0.1 confirmed", body(lat, lng, "0.1", "true"), 200, ""},
		{"radius 50 unconfirmed", body(lat, lng, "50", "false"), 422, "radius_unusual_unconfirmed"},
		{"radius 50 confirmed", body(lat, lng, "50", "true"), 200, ""},
		{"radius 50.1 confirmed", body(lat, lng, "50.1", "true"), 422, "radius_out_of_range"},
		{"radius negative", body(lat, lng, "-5", "true"), 422, "radius_out_of_range"},
		{"radius 2.9 unconfirmed", body(lat, lng, "2.9", ""), 422, "radius_unusual_unconfirmed"},
		{"radius 2.9 confirmed", body(lat, lng, "2.9", "true"), 200, ""},
		{"radius 20.1 unconfirmed", body(lat, lng, "20.1", ""), 422, "radius_unusual_unconfirmed"},
		{"radius 20.1 confirmed", body(lat, lng, "20.1", "true"), 200, ""},
		{"radius 3 needs no confirmation", body(lat, lng, "3", ""), 200, ""},
		{"radius 20 needs no confirmation", body(lat, lng, "20", ""), 200, ""},
		{"centre east of the box (Hoàng Sa)", body("16.5", "111.6", "10", ""), 422, "center_outside_mainland"},
		{"centre north of the box", body("23.5", lng, "10", ""), 422, "center_outside_mainland"},
		{"centre on the NE corner", body("23.4", "109.5", "10", ""), 200, ""},
		{"absent radius", `{"center_lat":15.7,"center_lng":108.3,"reason":"x"}`, 400, "invalid_body"},
		{"absent centre", `{"center_lng":108.3,"radius_km":10,"reason":"x"}`, 400, "invalid_body"},
		{"radius as text", `{"center_lat":15.7,"center_lng":108.3,"radius_km":"10","reason":"x"}`, 400, "invalid_body"},
		{"commune in body", `{"center_lat":15.7,"center_lng":108.3,"radius_km":10,"reason":"x","tenant_id":"` +
			inactiveCommuneIDFake + `"}`, 400, "invalid_body"},
		{"blank reason", `{"center_lat":15.7,"center_lng":108.3,"radius_km":10,"reason":"  "}`, 422, "invalid_reason"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.keys = []string{"ops.tenant.manage"}
			rec := h.do("PUT", mapFramePath(communeIDFake), c.body, opCookie(t))
			if rec.Code != c.want || (c.code != "" && !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`)) {
				t.Fatalf("%d %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
			if c.want != 200 && h.frames.writes != 0 {
				t.Fatal("a refused request reached the store")
			}
			if c.want == 200 && h.frames.writes != 1 {
				t.Fatalf("store writes %d, want 1", h.frames.writes)
			}
		})
	}
}

// What reaches the audited store: the commune of the PATH (never the body), the operator's VH- code and
// address, the NFC-trimmed reason, the rounded values, and the confirmation — recorded only when the
// radius actually needed one.
func TestSetMapFrameDefaultReachesTheAuditedStore(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.tenant.manage"}
	rec := h.do("PUT", mapFramePath(communeIDFake),
		`{"center_lat":15.7305071,"center_lng":108.37811,"radius_km":25.04,"acknowledged_unusual":true,"reason":"  Xã đề nghị mở rộng  "}`,
		opCookie(t))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"configured":true`) ||
		!strings.Contains(rec.Body.String(), `"updated_by":"VH-00001"`) {
		t.Fatalf("set: %d %s", rec.Code, rec.Body)
	}
	f := h.frames
	if f.target != communeIDFake || f.actor.Code != opCodeFake || f.actor.IP == "" || f.reason != "Xã đề nghị mở rộng" ||
		!f.acknowledged || f.saw.RadiusKm != 25 || f.saw.CenterLat != 15.730507 {
		t.Errorf("store saw target=%s actor=%+v reason=%q ack=%v frame=%+v", f.target, f.actor, f.reason, f.acknowledged, f.saw)
	}

	// A usual radius with a stray `true`: saved, but the trail does not claim a confirmation.
	if rec := h.do("PUT", mapFramePath(communeIDFake),
		`{"center_lat":15.73,"center_lng":108.37,"radius_km":10,"acknowledged_unusual":true,"reason":"Về 10 km"}`,
		opCookie(t)); rec.Code != 200 || h.frames.acknowledged {
		t.Errorf("usual radius: %d ack=%v", rec.Code, h.frames.acknowledged)
	}
}

func TestSetMapFrameDefaultUnknownAndInactiveCommune(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.tenant.manage"}
	ok := `{"center_lat":15.73,"center_lng":108.37,"radius_km":10,"reason":"Đặt"}`
	for path, want := range map[string]struct {
		status int
		code   string
	}{
		mapFramePath("01JD8ZQK9M3NPXR7TVWYB2C4EZ"): {404, "commune_not_found"},
		mapFramePath("not-a-ulid"):                 {404, "commune_not_found"},
		mapFramePath(inactiveCommuneIDFake):        {409, "commune_inactive"},
	} {
		rec := h.do("PUT", path, ok, opCookie(t))
		if rec.Code != want.status || !strings.Contains(rec.Body.String(), `"code":"`+want.code+`"`) {
			t.Errorf("%s: %d %s, want %d %s", path, rec.Code, rec.Body, want.status, want.code)
		}
	}
}
