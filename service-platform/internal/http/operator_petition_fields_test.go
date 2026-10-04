package http

// Tier-1 petition field codes in the operator area (ADR 0073 #3). 401 / 401 staff token / 403 / 503 /
// 2xx for every route are in TestGuardedRoutes; this file defends what is specific to each.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

type fieldsFake struct {
	rows      map[string]domain.PetitionField
	err       error
	writes    int
	sawCode   string
	sawField  domain.PetitionField
	sawActive *bool
	sawReason string
	sawActor  domain.OperatorActor
}

func newFieldsFake() *fieldsFake {
	return &fieldsFake{rows: map[string]domain.PetitionField{
		"dien": {Code: "dien", DefaultLabel: "Điện", SortOrder: 4, Icon: "Zap", Tone: "orange", Active: true},
		"khac": {Code: "khac", DefaultLabel: "Khác", SortOrder: 12, Icon: "MessageSquare", Tone: "blue", Active: false},
	}}
}

func (f *fieldsFake) ListPetitionFields(context.Context) ([]domain.PetitionField, error) {
	return []domain.PetitionField{f.rows["dien"], f.rows["khac"]}, f.err
}

func (f *fieldsFake) record(code string, reason string, by domain.OperatorActor) {
	f.writes++
	f.sawCode, f.sawReason, f.sawActor = code, reason, by
}

func (f *fieldsFake) CreatePetitionField(_ context.Context, in domain.PetitionField, reason string, by domain.OperatorActor) (domain.PetitionField, error) {
	f.record(in.Code, reason, by)
	f.sawField = in
	if f.err != nil {
		return domain.PetitionField{}, f.err
	}
	if _, ok := f.rows[in.Code]; ok {
		return domain.PetitionField{}, store.ErrPetitionFieldCodeTaken
	}
	in.Active = true
	return in, nil
}

func (f *fieldsFake) EditPetitionField(_ context.Context, code string, next domain.PetitionField, reason string, by domain.OperatorActor) (domain.PetitionField, bool, error) {
	f.record(code, reason, by)
	f.sawField = next
	row, ok := f.rows[code]
	if !ok {
		return domain.PetitionField{}, false, store.ErrPetitionFieldNotFound
	}
	next.Code, next.Active = row.Code, row.Active
	return next, true, f.err
}

func (f *fieldsFake) SetPetitionFieldActive(_ context.Context, code string, active bool, reason string, by domain.OperatorActor) (domain.PetitionField, bool, error) {
	f.record(code, reason, by)
	f.sawActive = &active
	row, ok := f.rows[code]
	if !ok {
		return domain.PetitionField{}, false, store.ErrPetitionFieldNotFound
	}
	row.Active = active
	return row, true, f.err
}

func TestListPetitionFieldsIncludesRetiredAndTones(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.qr.issue"} // any one key reads (ADR 0073 #1)
	rec := h.do("GET", "/api/v1/petition-fields", "", opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d (%s)", rec.Code, rec.Body)
	}
	var got petitionFieldListView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || got.Items[1].Code != "khac" || got.Items[1].Active {
		t.Errorf("a retired code must still be listed (it labels old petitions): %+v", got.Items)
	}
	if strings.Join(got.Tones, ",") != "blue,green,orange,purple,cyan,red" {
		t.Errorf("tones %v", got.Tones)
	}
}

func TestCreatePetitionField(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.petition_field.manage"}
	body := func(code, icon string) string {
		return `{"code":"` + code + `","default_label":"Cây xanh","sort_order":13,"icon":` + icon + `,"tone":"green","reason":"Xã đề nghị"}`
	}
	for _, c := range []struct {
		name, body string
		want       int
		code       string
	}{
		{"absent icon", `{"code":"cay-xanh","default_label":"Cây xanh","sort_order":13,"tone":"green","reason":"x"}`, 400, "invalid_body"},
		{"absent order", `{"code":"cay-xanh","default_label":"Cây xanh","icon":"","tone":"","reason":"x"}`, 400, "invalid_body"},
		{"active in body", `{"code":"cay-xanh","default_label":"a","sort_order":1,"icon":"","tone":"","reason":"x","active":false}`, 400, "invalid_body"},
		{"diacritics in code", body("cây-xanh", `"Trees"`), 422, "invalid_code"},
		{"upper case code", body("Cay-Xanh", `"Trees"`), 422, "invalid_code"},
		{"bad icon", body("cay-xanh", `"<svg>"`), 422, "invalid_icon"},
		{"no reason", `{"code":"cay-xanh","default_label":"a","sort_order":1,"icon":"","tone":"","reason":" "}`, 422, "invalid_reason"},
	} {
		if rec := h.do("POST", "/api/v1/petition-fields", c.body, opCookie(t)); rec.Code != c.want ||
			!strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
			t.Errorf("%s: %d %s; want %d %s", c.name, rec.Code, rec.Body, c.want, c.code)
		}
	}
	if h.fields.writes != 0 {
		t.Fatal("a refused request reached the store")
	}
	// An issued code — active or retired — is never issued again (rule 7 invariant 3).
	if rec := h.do("POST", "/api/v1/petition-fields", body("khac", `""`), opCookie(t)); rec.Code != 409 ||
		!strings.Contains(rec.Body.String(), "petition_field_code_taken") {
		t.Errorf("retired code re-issued: %d %s", rec.Code, rec.Body)
	}
	rec := h.do("POST", "/api/v1/petition-fields", body("cay-xanh", `"Trees"`), opCookie(t))
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"active":true`) {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if h.fields.sawActor.Code != opCodeFake || h.fields.sawReason != "Xã đề nghị" || h.fields.sawField.DefaultLabel != "Cây xanh" {
		t.Errorf("store saw %+v reason %q actor %+v", h.fields.sawField, h.fields.sawReason, h.fields.sawActor)
	}
}

// The code is the PATH's and never changes: a body naming a code is refused before the store.
func TestEditPetitionFieldNeverTouchesTheCode(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.petition_field.manage"}
	rename := `{"code":"dien-luc","default_label":"Điện","sort_order":4,"icon":"Zap","tone":"orange","reason":"x"}`
	if rec := h.do("PUT", "/api/v1/petition-fields/dien", rename, opCookie(t)); rec.Code != 400 {
		t.Errorf("a body carrying a code: %d, want 400 (renaming orphans archival petitions)", rec.Code)
	}
	for path, want := range map[string]int{
		"/api/v1/petition-fields/Dien":        404, // malformed = unknown
		"/api/v1/petition-fields/khong-co-ma": 404,
		"/api/v1/petition-fields/dien":        200,
	} {
		rec := h.do("PUT", path, `{"default_label":" Điện lực ","sort_order":4,"icon":"Zap","tone":"","reason":"Sửa nhãn"}`, opCookie(t))
		if rec.Code != want {
			t.Errorf("%s: %d %s, want %d", path, rec.Code, rec.Body, want)
		}
	}
	if h.fields.sawCode != "dien" && h.fields.sawCode != "khong-co-ma" {
		t.Errorf("store saw code %q", h.fields.sawCode)
	}
	if rec := h.do("PUT", "/api/v1/petition-fields/dien", `{"default_label":"Điện","sort_order":4,"icon":"Zap","reason":"x"}`, opCookie(t)); rec.Code != 400 {
		t.Errorf("absent tone: %d, want 400 (never a silent clear)", rec.Code)
	}
}

func TestPetitionFieldActivation(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.petition_field.manage"}
	path := "/api/v1/petition-fields/dien/activation"
	if rec := h.do("PUT", path, `{"reason":"x"}`, opCookie(t)); rec.Code != 400 {
		t.Errorf("absent active: %d, want 400 (never a silent retirement in every commune)", rec.Code)
	}
	if rec := h.do("PUT", path, `{"active":false,"reason":""}`, opCookie(t)); rec.Code != 422 {
		t.Errorf("blank reason: %d, want 422", rec.Code)
	}
	if h.fields.writes != 0 {
		t.Fatal("a refused request reached the store")
	}
	rec := h.do("PUT", path, `{"active":false,"reason":"Gộp vào mã khác"}`, opCookie(t))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"active":false`) || h.fields.sawActive == nil || *h.fields.sawActive {
		t.Fatalf("retire: %d %s", rec.Code, rec.Body)
	}
}
