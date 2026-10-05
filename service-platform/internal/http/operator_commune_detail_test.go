package http

// The commune detail's secret status per own App ID (ADR 0070 §Sửa đổi 05/10/2026 #3): identity's
// ListMiniAppSecretStatuses joined on app_id. What these tests defend: the commune asked about is the
// PATH's; a version without a secret is "chua_dat", never dated; identity's 401/403 hold on the read
// but never on the answer to a committed write; every other failure — an identity older than the
// RPC included — renders "khong_ro" instead of failing the detail; settings left live under an App ID
// the commune no longer binds are surfaced so they can be retired.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/operatorclient"
)

const detailPath = "/api/v1/communes/" + communeIDFake

func detailHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"} // any one key reads the detail (ADR 0073 #1)
	return h
}

func decodeDetail(t *testing.T, body []byte) communeDetailView {
	t.Helper()
	var v communeDetailView
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode: %v (%s)", err, body)
	}
	return v
}

func secretOf(t *testing.T, v communeDetailView, appID string) secretStatusView {
	t.Helper()
	for _, a := range v.MiniApps {
		if a.AppID == appID {
			if a.Secret == nil {
				t.Fatalf("app %s has no secret status", appID)
			}
			return *a.Secret
		}
	}
	t.Fatalf("app %s not in the detail", appID)
	return secretStatusView{}
}

func TestDetailJoinsSecretStatuses(t *testing.T) {
	h := detailHarness(t)
	h.id.statusRes = &operatorclient.ListMiniAppSecretStatusesResult{Outcome: operatorclient.OutcomeAccepted,
		Statuses: []operatorclient.MiniAppSecretStatus{
			{AppID: oldAppFake, SecretSet: true, SetAt: nowFake.In(time.FixedZone("ICT", 7*3600)), SetBy: opCodeFake},
			// A live version holding no secret: "chua_dat", never presented as set.
			{AppID: newAppFake, SecretSet: false},
			// Settings live under an App ID the commune no longer binds (a failed automatic retirement).
			{AppID: deletedAppFake, SecretSet: true, SetAt: nowFake, SetBy: "VH-00002"},
		}}
	rec := h.do("GET", detailPath, "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	v := decodeDetail(t, rec.Body.Bytes())
	if s := secretOf(t, v, oldAppFake); s.Status != "da_dat" || s.SetBy != opCodeFake || s.SetAt == nil || !s.SetAt.Equal(nowFake) {
		t.Errorf("set app: %+v", s)
	}
	if !strings.Contains(rec.Body.String(), `"set_at":"2026-10-01T08:00:00Z"`) {
		t.Errorf("set_at must be UTC: %s", rec.Body)
	}
	if s := secretOf(t, v, newAppFake); s.Status != "chua_dat" || s.SetAt != nil || s.SetBy != "" {
		t.Errorf("version without a secret: %+v", s)
	}
	if len(v.UnboundSecrets) != 1 || v.UnboundSecrets[0].AppID != deletedAppFake || v.UnboundSecrets[0].Secret.Status != "da_dat" {
		t.Errorf("unbound = %+v", v.UnboundSecrets)
	}
	if len(h.id.sawStatusTenant) != 1 || h.id.sawStatusTenant[0] != communeIDFake {
		t.Errorf("identity asked about %v, want the path's commune", h.id.sawStatusTenant)
	}

	// Absent from identity's list: "chua_dat"; nothing unbound is an empty list, not null.
	h = detailHarness(t)
	rec = h.do("GET", detailPath, "", opCookie(t))
	v = decodeDetail(t, rec.Body.Bytes())
	if s := secretOf(t, v, oldAppFake); s.Status != "chua_dat" {
		t.Errorf("absent: %+v", s)
	}
	if !strings.Contains(rec.Body.String(), `"unbound_secrets":[]`) {
		t.Errorf("want unbound_secrets [] when known: %s", rec.Body)
	}
}

func TestDetailSecretStatusFailuresRenderUnknown(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		res  *operatorclient.ListMiniAppSecretStatusesResult
	}{
		{"identity older than the RPC", operatorclient.ErrNotSupported, nil},
		{"contract fault (UNSPECIFIED)", operatorclient.ErrContract, nil},
		{"identity unreachable", errors.New("operatorclient: unavailable (fake)"), nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := detailHarness(t)
			h.id.statusErr, h.id.statusRes = c.err, c.res
			rec := h.do("GET", detailPath, "", opCookie(t))
			if rec.Code != 200 {
				t.Fatalf("the detail must still render: %d %s", rec.Code, rec.Body)
			}
			v := decodeDetail(t, rec.Body.Bytes())
			for _, a := range v.MiniApps {
				if a.Secret == nil || a.Secret.Status != "khong_ro" || a.Secret.SetAt != nil {
					t.Errorf("%s: %+v, want khong_ro", a.AppID, a.Secret)
				}
			}
			if !strings.Contains(rec.Body.String(), `"unbound_secrets":null`) {
				t.Errorf("unknown must be null, not an empty list: %s", rec.Body)
			}
		})
	}
}

func TestDetailSecretStatusOutcomes(t *testing.T) {
	for _, c := range []struct {
		outcome operatorclient.Outcome
		want    int
	}{
		{operatorclient.OutcomeSessionNotLive, 401},
		{operatorclient.OutcomePermissionDenied, 403},
	} {
		h := detailHarness(t)
		h.id.statusRes = &operatorclient.ListMiniAppSecretStatusesResult{Outcome: c.outcome}
		rec := h.do("GET", detailPath, "", opCookie(t))
		if rec.Code != c.want {
			t.Errorf("%v on the read: %d, want %d", c.outcome, rec.Code, c.want)
		}
		if strings.Contains(rec.Body.String(), `"mini_apps"`) {
			t.Errorf("%v: a refused read leaked the detail: %s", c.outcome, rec.Body)
		}

		// After a COMMITTED write the same outcome must not read as "your write was refused".
		h = newHarness(t)
		h.id.keys = []string{"ops.mini_app.manage"}
		h.id.statusRes = &operatorclient.ListMiniAppSecretStatusesResult{Outcome: c.outcome}
		rec = h.do("PUT", miniAppActivationPath(communeIDFake, oldAppFake), `{"active":false,"reason":"Gỡ"}`, opCookie(t))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"khong_ro"`) {
			t.Errorf("%v after a write: %d %s, want 200 with khong_ro", c.outcome, rec.Code, rec.Body)
		}
	}
}

// The secret status never carries anything of the value: the view has no field for it, and the
// JSON keys are exactly status / set_at / set_by.
func TestSecretStatusViewShape(t *testing.T) {
	at := nowFake
	b, _ := json.Marshal(secretStatusView{Status: "da_dat", SetAt: &at, SetBy: opCodeFake})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if len(m) != 3 {
		t.Fatalf("secret status keys %v, want status/set_at/set_by only", m)
	}
}
