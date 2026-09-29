package platformclient

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

const (
	fieldsCommuneA = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")
	fieldsCommuneB = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EG")
)

type fakeFieldsClient struct {
	mu    sync.Mutex
	res   *platformv1.ListPetitionFieldsResponse
	err   error
	calls int
}

func (f *fakeFieldsClient) ListPetitionFields(_ context.Context, _ *platformv1.ListPetitionFieldsRequest,
	_ ...grpc.CallOption) (*platformv1.ListPetitionFieldsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.res, nil
}

func (f *fakeFieldsClient) answer(res *platformv1.ListPetitionFieldsResponse, err error) {
	f.mu.Lock()
	f.res, f.err = res, err
	f.mu.Unlock()
}

type fieldsClock struct{ t time.Time }

func (c *fieldsClock) now() time.Time { return c.t }

func newFieldsReader(f *fakeFieldsClient) (*PetitionFields, *fieldsClock) {
	r := NewPetitionFields(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := &fieldsClock{t: time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)}
	r.now = c.now
	return r, c
}

func fieldsAnswer(fs ...*platformv1.PetitionField) *platformv1.ListPetitionFieldsResponse {
	return &platformv1.ListPetitionFieldsResponse{Fields: fs}
}

func racThai() *platformv1.PetitionField {
	return &platformv1.PetitionField{Code: "rac-thai", DefaultLabel: "Rác thải – Vệ sinh môi trường",
		SortOrder: 1, Icon: "Trash2", Tone: "orange", Active: true}
}

func khac() *platformv1.PetitionField {
	return &platformv1.PetitionField{Code: "khac", DefaultLabel: "Khác", SortOrder: 12, Active: true}
}

func inCommune(id tenant.ID) context.Context { return tenant.Into(context.Background(), id) }

// Active → accepted; retired → refused for intake but still looked up; absent → unknown.
func TestPetitionFieldsCheckForIntake(t *testing.T) {
	f := &fakeFieldsClient{}
	retired := &platformv1.PetitionField{Code: "ma-cu", DefaultLabel: "Mã cũ", SortOrder: 13}
	f.answer(fieldsAnswer(racThai(), khac(), retired), nil)
	r, _ := newFieldsReader(f)

	set, err := r.Set(inCommune(fieldsCommuneA))
	if err != nil {
		t.Fatal(err)
	}
	if err := set.CheckForIntake("rac-thai"); err != nil {
		t.Errorf("rac-thai: %v, want nil", err)
	}
	if err := set.CheckForIntake("ma-cu"); !errors.Is(err, ErrPetitionFieldRetired) {
		t.Errorf("ma-cu: %v, want ErrPetitionFieldRetired", err)
	}
	if err := set.CheckForIntake("ve-sinh-moi-truong"); !errors.Is(err, ErrPetitionFieldUnknown) {
		t.Errorf("unknown: %v, want ErrPetitionFieldUnknown", err)
	}
	// A retired code still has its label on read.
	if pf, ok := set.Lookup("ma-cu"); !ok || pf.DefaultLabel != "Mã cũ" || pf.Active {
		t.Errorf("Lookup(ma-cu) = %+v, %v", pf, ok)
	}
}

// Empty answer: every code is unknown — never "anything goes".
func TestPetitionFieldsEmptyRefusesEverything(t *testing.T) {
	f := &fakeFieldsClient{}
	f.answer(fieldsAnswer(), nil)
	r, _ := newFieldsReader(f)
	set, err := r.Set(inCommune(fieldsCommuneA))
	if err != nil {
		t.Fatal(err)
	}
	if err := set.CheckForIntake("khac"); !errors.Is(err, ErrPetitionFieldUnknown) {
		t.Errorf("khac on empty set: %v, want ErrPetitionFieldUnknown", err)
	}
}

// Unavailable is its own sentinel — never "unknown", never "valid" — and nothing is cached from it.
func TestPetitionFieldsUnavailable(t *testing.T) {
	f := &fakeFieldsClient{}
	f.answer(nil, status.Error(codes.Unavailable, "down"))
	r, _ := newFieldsReader(f)

	_, err := r.Set(inCommune(fieldsCommuneA))
	if !errors.Is(err, ErrPetitionFieldsUnavailable) {
		t.Fatalf("err = %v, want ErrPetitionFieldsUnavailable", err)
	}
	if errors.Is(err, ErrPetitionFieldUnknown) {
		t.Error("an outage reads as 'unknown code'")
	}
	// Errors are not cached: the next call asks again and succeeds.
	f.answer(fieldsAnswer(khac()), nil)
	if _, err := r.Set(inCommune(fieldsCommuneA)); err != nil {
		t.Fatalf("after recovery: %v", err)
	}
	if f.calls != 2 {
		t.Errorf("calls = %d, want 2", f.calls)
	}
}

// Within the TTL one answer serves every commune; past it the platform is asked again; a failed
// refresh after expiry is REFUSED, never served from the old answer.
func TestPetitionFieldsCacheTTLAndNoStaleServing(t *testing.T) {
	f := &fakeFieldsClient{}
	f.answer(fieldsAnswer(khac()), nil)
	r, c := newFieldsReader(f)

	if _, err := r.Set(inCommune(fieldsCommuneA)); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(PetitionFieldsTTL - time.Second)
	if _, err := r.Set(inCommune(fieldsCommuneB)); err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 {
		t.Fatalf("calls within TTL across two communes = %d, want 1 (one set for every commune)", f.calls)
	}

	c.t = c.t.Add(time.Second) // exactly at expiry: expired
	f.answer(nil, status.Error(codes.Unavailable, "down"))
	if _, err := r.Set(inCommune(fieldsCommuneA)); !errors.Is(err, ErrPetitionFieldsUnavailable) {
		t.Fatalf("expired + platform down: err = %v, want ErrPetitionFieldsUnavailable (no stale serving)", err)
	}
	if f.calls != 2 {
		t.Errorf("calls = %d, want 2", f.calls)
	}
}

// No commune in ctx: refused locally, on a cache hit too, and nothing is sent.
func TestPetitionFieldsRequireCommune(t *testing.T) {
	f := &fakeFieldsClient{}
	f.answer(fieldsAnswer(khac()), nil)
	r, _ := newFieldsReader(f)
	if _, err := r.Set(context.Background()); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("cold, no commune: err = %v, want ErrNoTenant", err)
	}
	if f.calls != 0 {
		t.Fatalf("calls = %d, want 0 — nothing may be sent without a commune", f.calls)
	}
	if _, err := r.Set(inCommune(fieldsCommuneA)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Set(context.Background()); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("warm, no commune: err = %v, want ErrNoTenant", err)
	}
}

// Entries that break the contract are dropped — they read as unknown codes — and the rest arrive.
func TestPetitionFieldsDropsUnusableEntries(t *testing.T) {
	f := &fakeFieldsClient{}
	f.answer(fieldsAnswer(
		khac(),
		&platformv1.PetitionField{Code: "", DefaultLabel: "x", SortOrder: 1, Active: true},
		&platformv1.PetitionField{Code: "Điện", DefaultLabel: "x", SortOrder: 1, Active: true},
		&platformv1.PetitionField{Code: "khong-nhan", SortOrder: 1, Active: true},
		&platformv1.PetitionField{Code: "thu-tu-0", DefaultLabel: "x", Active: true},
		&platformv1.PetitionField{Code: "trung", DefaultLabel: "A", SortOrder: 1, Active: true},
		&platformv1.PetitionField{Code: "trung", DefaultLabel: "B", SortOrder: 2, Active: false},
	), nil)
	r, _ := newFieldsReader(f)
	set, err := r.Set(inCommune(fieldsCommuneA))
	if err != nil {
		t.Fatal(err)
	}
	if all := set.All(); len(all) != 1 || all[0].Code != "khac" {
		t.Fatalf("All = %+v, want only khac", all)
	}
	for _, code := range []string{"khong-nhan", "thu-tu-0", "trung"} {
		if err := set.CheckForIntake(code); !errors.Is(err, ErrPetitionFieldUnknown) {
			t.Errorf("%s: %v, want ErrPetitionFieldUnknown", code, err)
		}
	}
}

// Order is SortOrder then Code whatever the wire order; an unknown tone becomes ""; All is a copy.
func TestPetitionFieldsOrderToneAndCopy(t *testing.T) {
	f := &fakeFieldsClient{}
	odd := &platformv1.PetitionField{Code: "b-ma", DefaultLabel: "B", SortOrder: 1, Tone: "pink", Active: true}
	f.answer(fieldsAnswer(khac(), odd, racThai()), nil)
	r, _ := newFieldsReader(f)
	set, err := r.Set(inCommune(fieldsCommuneA))
	if err != nil {
		t.Fatal(err)
	}
	all := set.All()
	if len(all) != 3 || all[0].Code != "b-ma" || all[1].Code != "rac-thai" || all[2].Code != "khac" {
		t.Fatalf("order = %+v", all)
	}
	if all[0].Tone != "" || all[1].Tone != "orange" || all[1].Icon != "Trash2" {
		t.Errorf("tone/icon = %+v / %+v", all[0], all[1])
	}
	all[0].Code = "sua"
	if set.All()[0].Code != "b-ma" {
		t.Error("All shares its backing array with the cache")
	}
}

func TestNewPetitionFieldsNilClientPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("built a reader with a nil client")
		}
	}()
	_ = NewPetitionFields(nil, nil)
}
