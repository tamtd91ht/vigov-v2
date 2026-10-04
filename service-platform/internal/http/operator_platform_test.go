package http

// Wave 2 (ADR 0073): the upload limits, the operator log, and the platform's display copy of a
// forwarded secret act. 401 / 401 staff token / 403 / 503 / 2xx for every route are in
// TestGuardedRoutes; this file defends what is specific to each.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// --- fakes -------------------------------------------------------------------------------------------

type policyFake struct {
	rows      map[string]domain.UploadPolicy
	err       error
	changed   []domain.UploadPolicy
	reasons   []string
	actors    []domain.OperatorActor
	listCalls int
}

func newPolicyFake() *policyFake {
	return &policyFake{rows: map[string]domain.UploadPolicy{
		"petition-photo": {Purpose: "petition-photo", MaxBytes: 10485760,
			AllowedMIMETypes: []string{"image/jpeg", "image/png", "image/webp"}, FileCountLimited: true,
			MaxFilesPerSubject: 5, UpdatedAt: nowFake, UpdatedBy: "system"},
		"content-video": {Purpose: "content-video", MaxBytes: 2147483648,
			AllowedMIMETypes: []string{"video/mp4", "video/quicktime"}, UpdatedAt: nowFake, UpdatedBy: "system"},
	}}
}

func (f *policyFake) ListUploadPolicies(context.Context) ([]domain.UploadPolicy, error) {
	f.listCalls++
	return []domain.UploadPolicy{f.rows["content-video"], f.rows["petition-photo"]}, f.err
}

func (f *policyFake) ChangeUploadPolicy(_ context.Context, next domain.UploadPolicy, reason string, by domain.OperatorActor) (domain.UploadPolicy, bool, error) {
	if f.err != nil {
		return domain.UploadPolicy{}, false, f.err
	}
	if _, ok := f.rows[next.Purpose]; !ok {
		return domain.UploadPolicy{}, false, store.ErrUploadPolicyNotFound
	}
	f.changed = append(f.changed, next)
	f.reasons = append(f.reasons, reason)
	f.actors = append(f.actors, by)
	next.UpdatedAt, next.UpdatedBy = nowFake, by.Code
	return next, true, nil
}

type opLogFake struct {
	reads   int
	queries []store.OperatorLogQuery
	readers []domain.OperatorActor
	err     error
}

func (f *opLogFake) Read(_ context.Context, q store.OperatorLogQuery, reader domain.OperatorActor) (page.Result[domain.OperatorLogEntry], error) {
	f.reads++
	f.queries = append(f.queries, q)
	f.readers = append(f.readers, reader)
	r := page.NewResult[domain.OperatorLogEntry]()
	if f.err != nil {
		return r, f.err
	}
	r.Items = append(r.Items,
		domain.OperatorLogEntry{At: nowFake, ActorCode: opCodeFake, Action: "sua_ten_xa", CommuneID: communeIDFake,
			CommuneName: "Xã Thăng Bình", Subject: "Xã Thăng Bình", Before: []byte(`{"ten":"Xã Thăng Bìn"}`),
			After: []byte(`{"ten":"Xã Thăng Bình"}`), Reason: "Sửa lỗi gõ"},
		domain.OperatorLogEntry{At: nowFake.Add(-time.Hour), ActorCode: "system", Action: "upload_policy.seeded",
			Subject: "content-video", After: []byte(`{"max_bytes":2147483648}`)})
	return r, nil
}

// --- upload policies ----------------------------------------------------------------------------------

func TestListUploadPoliciesShowsChoicesAndNullCount(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"} // any one key reads (ADR 0073 #1)
	rec := h.do("GET", "/api/v1/upload-policies", "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	var got uploadPolicyListView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items %+v", got.Items)
	}
	video, photo := got.Items[0], got.Items[1]
	if video.MaxFilesPerSubject != nil || !strings.Contains(rec.Body.String(), `"max_files_per_subject":null`) {
		t.Errorf("no count limit must be an explicit null: %s", rec.Body)
	}
	if photo.MaxFilesPerSubject == nil || *photo.MaxFilesPerSubject != 5 {
		t.Errorf("photo count %v", photo.MaxFilesPerSubject)
	}
	// The console is offered only what the pipeline handles: no HEIC for a re-encoded photo.
	if strings.Join(photo.MIMEChoices, ",") != "image/jpeg,image/png,image/webp" || photo.MaxBytesCap != 50<<20 {
		t.Errorf("photo choices %v cap %d", photo.MIMEChoices, photo.MaxBytesCap)
	}
	if video.MaxBytesCap != 5<<30 {
		t.Errorf("video cap %d", video.MaxBytesCap)
	}
}

const policyPath = "/api/v1/upload-policies/petition-photo"

func TestChangeUploadPolicyPassesValidatedValues(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.upload_policy.manage"}
	rec := h.do("PUT", policyPath,
		`{"max_bytes":5242880,"allowed_mime_types":["image/png","image/jpeg"],"max_files_per_subject":null,"reason":"  Bỏ giới hạn số ảnh  "}`,
		opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	if len(h.pol.changed) != 1 {
		t.Fatalf("store called %d times", len(h.pol.changed))
	}
	c := h.pol.changed[0]
	if c.Purpose != "petition-photo" || c.MaxBytes != 5242880 || c.FileCountLimited ||
		strings.Join(c.AllowedMIMETypes, ",") != "image/png,image/jpeg" {
		t.Errorf("stored %+v", c)
	}
	if h.pol.reasons[0] != "Bỏ giới hạn số ảnh" || h.pol.actors[0].Code != opCodeFake || h.pol.actors[0].IP == "" {
		t.Errorf("reason %q actor %+v", h.pol.reasons[0], h.pol.actors[0])
	}
	if !strings.Contains(rec.Body.String(), `"updated_by":"VH-00001"`) || !strings.Contains(rec.Body.String(), `"max_files_per_subject":null`) {
		t.Errorf("answer %s", rec.Body)
	}
}

func TestChangeUploadPolicyRefusals(t *testing.T) {
	good := func(over string) string {
		m := map[string]any{"max_bytes": 1048576, "allowed_mime_types": []string{"image/jpeg"},
			"max_files_per_subject": 5, "reason": "Đổi giới hạn"}
		if over != "" {
			var o map[string]any
			if err := json.Unmarshal([]byte(over), &o); err != nil {
				panic(err)
			}
			for k, v := range o {
				if v == "DROP" {
					delete(m, k)
				} else {
					m[k] = v
				}
			}
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	for _, c := range []struct {
		name, path, body string
		want             int
		code             string
	}{
		{"unknown purpose", "/api/v1/upload-policies/everything", good(""), 404, "upload_policy_not_found"},
		{"purpose with no live row", "/api/v1/upload-policies/tenant-logo", good(""), 404, "upload_policy_not_found"},
		{"max_bytes absent", policyPath, good(`{"max_bytes":"DROP"}`), 400, "invalid_body"},
		{"count absent", policyPath, good(`{"max_files_per_subject":"DROP"}`), 400, "invalid_body"},
		{"count not a number", policyPath, good(`{"max_files_per_subject":"5"}`), 400, "invalid_body"},
		{"body names a commune", policyPath, good(`{"tenant_id":"01JD8ZQK9M3NPXR7TVWYB2C4EH"}`), 400, "invalid_body"},
		{"HEIC on a re-encoded photo", policyPath, good(`{"allowed_mime_types":["image/jpeg","image/heic"]}`), 422, "invalid_mime_types"},
		{"PDF on a photo", policyPath, good(`{"allowed_mime_types":["application/pdf"]}`), 422, "invalid_mime_types"},
		{"no types", policyPath, good(`{"allowed_mime_types":[]}`), 422, "invalid_mime_types"},
		{"over the image cap", policyPath, good(`{"max_bytes":52428801}`), 422, "invalid_max_bytes"},
		{"zero bytes", policyPath, good(`{"max_bytes":0}`), 422, "invalid_max_bytes"},
		{"zero files", policyPath, good(`{"max_files_per_subject":0}`), 422, "invalid_max_files"},
		{"reason blank", policyPath, good(`{"reason":"   "}`), 422, "invalid_reason"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.keys = []string{"ops.upload_policy.manage"}
			rec := h.do("PUT", c.path, c.body, opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
			if len(h.pol.changed) != 0 {
				t.Error("a refused edit reached the store")
			}
		})
	}
}

// Reading the limits is any key; CHANGING them is ops.upload_policy.manage and nothing else.
func TestChangeUploadPolicyNeedsItsKey(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.tenant.manage", "ops.domain.manage", "ops.profile.manage", "ops.mini_app.manage", "ops.qr.issue"}
	rec := h.do("PUT", policyPath, `{"max_bytes":1,"allowed_mime_types":["image/jpeg"],"max_files_per_subject":1,"reason":"x"}`, opCookie(t))
	if rec.Code != 403 || len(h.pol.changed) != 0 {
		t.Fatalf("every other key: %d, store calls %d — want 403, 0", rec.Code, len(h.pol.changed))
	}
}

// --- the operator log ---------------------------------------------------------------------------------

func TestOperatorLogAnswersMetadataAndNamesTheReader(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.mini_app.manage"}
	rec := h.do("GET", "/api/v1/operator-audit-entries?from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z&limit=10", "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	q := h.oplog.queries[0]
	if q.CommuneID != "" || q.From.IsZero() || q.To.IsZero() || q.Page.Limit() != 10 {
		t.Errorf("query %+v", q)
	}
	if h.oplog.readers[0].Code != opCodeFake || h.oplog.readers[0].IP == "" {
		t.Errorf("the read must be attributed to the operator: %+v", h.oplog.readers[0])
	}
	var got operatorAuditPageView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || got.Items[0].Commune == nil || got.Items[0].Commune.ID != communeIDFake ||
		got.Items[0].Reason != "Sửa lỗi gõ" || got.Items[1].Commune != nil {
		t.Errorf("items %s", rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"commune":null`) || !strings.Contains(rec.Body.String(), `"before":null`) ||
		strings.Contains(rec.Body.String(), `"total"`) {
		t.Errorf("platform-wide entry must carry commune:null, absent before:null, no total: %s", rec.Body)
	}
}

func TestCommuneOperatorLogTakesTheCommuneFromThePath(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.domain.manage"}
	rec := h.do("GET", "/api/v1/communes/"+communeIDFake+"/operator-audit-entries", "", opCookie(t))
	if rec.Code != 200 || h.oplog.queries[0].CommuneID != communeIDFake {
		t.Fatalf("status %d, query %+v", rec.Code, h.oplog.queries)
	}
	for _, c := range []struct{ path, code string }{
		{"/api/v1/communes/nope/operator-audit-entries", "commune_not_found"},
		{"/api/v1/communes/01JD8ZQK9M3NPXR7TVWYB2C4EZ/operator-audit-entries", "commune_not_found"},
	} {
		h := newHarness(t)
		h.id.keys = []string{"ops.domain.manage"}
		if rec := h.do("GET", c.path, "", opCookie(t)); rec.Code != 404 || !strings.Contains(rec.Body.String(), c.code) || h.oplog.reads != 0 {
			t.Errorf("%s: %d %s, reads %d", c.path, rec.Code, rec.Body, h.oplog.reads)
		}
	}
}

func TestOperatorLogRefusesBadQueriesBeforeReading(t *testing.T) {
	for _, c := range []struct{ query, code string }{
		{"from=yesterday", "invalid_range"},
		{"from=2026-10-02T00:00:00Z&to=2026-10-01T00:00:00Z", "invalid_range"},
		{"from=2026-10-01T00:00:00Z&to=2026-10-01T00:00:00Z", "invalid_range"},
		{"cursor=garbage", "invalid_cursor"},
		{"limit=0", "invalid_limit"},
	} {
		h := newHarness(t)
		h.id.keys = []string{"ops.qr.issue"}
		rec := h.do("GET", "/api/v1/operator-audit-entries?"+c.query, "", opCookie(t))
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), c.code) || h.oplog.reads != 0 {
			t.Errorf("%s: %d %s, reads %d", c.query, rec.Code, rec.Body, h.oplog.reads)
		}
	}
	// A read whose own trail entry failed returns NO page.
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"}
	h.oplog.err = errors.New("trail of the read failed (fake)")
	if rec := h.do("GET", "/api/v1/operator-audit-entries", "", opCookie(t)); rec.Code != 500 || strings.Contains(rec.Body.String(), "items") {
		t.Errorf("failed read: %d %s", rec.Code, rec.Body)
	}
}

// --- the secret-forward display copy (ADR 0073 §Hệ quả) ------------------------------------------------

func TestSecretForwardTrailAfterAcceptedOnly(t *testing.T) {
	h := miniAppHarness(t)
	if rec := h.do("PUT", secretPath(communeIDFake, oldAppFake), secretBodyFake, opCookie(t)); rec.Code != 200 {
		t.Fatalf("set: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do("DELETE", secretPath(communeIDFake, newAppFake), `{"reason":"Thu hồi khoá cũ"}`, opCookie(t)); rec.Code != 200 {
		t.Fatalf("retire: %d %s", rec.Code, rec.Body)
	}
	want := []domain.SecretForward{
		{AppID: oldAppFake, Version: "01JDVERSIONFAKE00000000000", Reason: "Đặt khoá cho app riêng"},
		{Retired: true, AppID: newAppFake, Version: "01JDVERSIONFAKE00000000000", Reason: "Thu hồi khoá cũ"},
	}
	if len(h.w.forwards) != 2 || h.w.forwards[0] != want[0] || h.w.forwards[1] != want[1] {
		t.Fatalf("forwards %+v", h.w.forwards)
	}
	for i, tgt := range h.w.forwardTargets {
		if tgt != tenant.ID(communeIDFake) || h.w.forwardActors[i].Code != opCodeFake {
			t.Errorf("forward %d: target %q actor %+v", i, tgt, h.w.forwardActors[i])
		}
	}
	for _, f := range h.w.forwards {
		if strings.Contains(f.AppID+f.Version+f.Reason, appSecretFake) {
			t.Error("the display copy carries the secret")
		}
	}

	// Nothing accepted → nothing recorded: every non-ACCEPTED answer, and "nothing live to retire".
	for _, setup := range []func(*identityFake){
		func(f *identityFake) { f.setErr, f.retireErr = errors.New("down (fake)"), errors.New("down (fake)") },
		func(f *identityFake) {
			f.setRes = &operatorclient.SetMiniAppSecretResult{Outcome: operatorclient.OutcomePermissionDenied}
			f.retireRes = &operatorclient.RetireMiniAppSecretResult{Outcome: operatorclient.OutcomeSessionNotLive}
		},
		func(f *identityFake) {
			f.retireErr = operatorclient.ErrNoLiveSecret
			f.setErr = operatorclient.ErrMiniAppNotBound
		},
	} {
		h := miniAppHarness(t)
		setup(h.id)
		h.do("PUT", secretPath(communeIDFake, oldAppFake), secretBodyFake, opCookie(t))
		h.do("DELETE", secretPath(communeIDFake, oldAppFake), `{"reason":"x"}`, opCookie(t))
		if len(h.w.forwards) != 0 {
			t.Errorf("recorded an act identity did not accept: %+v", h.w.forwards)
		}
	}
}

func TestSecretForwardTrailOnAutomaticRetirement(t *testing.T) {
	h := miniAppHarness(t)
	rec := h.do("POST", replacementPath(communeIDFake, oldAppFake), `{"new_app_id":"`+newAppFake+`","reason":"Xã đổi App ID"}`, opCookie(t))
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(h.w.forwards) != 1 || !h.w.forwards[0].Retired || !h.w.forwards[0].Automatic ||
		h.w.forwards[0].AppID != oldAppFake || h.w.forwards[0].Reason != "Xã đổi App ID" {
		t.Fatalf("forwards %+v", h.w.forwards)
	}
}

// The display copy failing does not fail the act identity already sealed — it is logged instead.
func TestSecretForwardTrailFailureIsLoggedNotAnswered(t *testing.T) {
	h := miniAppHarness(t)
	h.w.forwardErr = errors.New("store down (fake)")
	rec := h.do("PUT", secretPath(communeIDFake, oldAppFake), secretBodyFake, opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d — the secret IS set in identity", rec.Code)
	}
	if !strings.Contains(h.logs.String(), "mini_app.secret_trail_missing") {
		t.Errorf("the missing display copy must be logged: %s", h.logs.String())
	}
	assertNoSecret(t, h, rec.Body.String(), nil)
}
