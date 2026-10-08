package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// fakePriorities holds live task priorities PER COMMUNE, so "another commune's code" is a real row the
// handler must not answer. Soft-deleted rows are simply not in the map — the store's predicate
// (`deleted_at IS NULL`) is proved in store/task_priority_codes_*_test.go; here the point is that a
// code the store does not return is ABSENT, not an error and not "active".
type fakePriorities struct {
	byTenant map[tenant.ID]map[string]bool
	extra    []domain.TaskPriorityCodeState // returned whatever was asked — a misbehaving store
	err      error
	calls    int
	gotCodes []string
}

func (f *fakePriorities) StatesByCode(ctx context.Context, codes []string) ([]domain.TaskPriorityCodeState, error) {
	f.calls++
	f.gotCodes = append([]string(nil), codes...)
	if f.err != nil {
		return nil, f.err
	}
	live := f.byTenant[tenant.MustFrom(ctx)]
	var out []domain.TaskPriorityCodeState
	for _, c := range codes {
		if active, ok := live[c]; ok {
			out = append(out, domain.TaskPriorityCodeState{Code: c, Active: active})
		}
	}
	return append(out, f.extra...), nil
}

const otherTenant = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EG")

func priorityScale() *fakePriorities {
	return &fakePriorities{byTenant: map[tenant.ID]map[string]bool{
		testTenant:  {"khan": true, "cao": false},
		otherTenant: {"rat-khan": true},
	}}
}

func resolvePriorities(f *fakePriorities, ctx context.Context, codes ...string) (map[string]bool, error) {
	srv := NewServer(Deps{Petitions: &fakeCounter{}, Tasks: &fakeCounter{}, TaskPriorities: f,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	res, err := srv.ResolveTaskPriorityCodes(ctx, &petitionsv1.ResolveTaskPriorityCodesRequest{Codes: codes})
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, it := range res.GetItems() {
		if _, dup := out[it.GetCode()]; dup {
			return nil, fmt.Errorf("code %q answered twice", it.GetCode())
		}
		out[it.GetCode()] = it.GetActive()
	}
	return out, nil
}

func inTenant() context.Context { return tenant.Into(context.Background(), testTenant) }

// Active answered active, switched-off answered INACTIVE (present, not dropped), unknown and another
// commune's code absent, duplicates answered once and asked once.
func TestResolveTaskPriorityCodesAnswers(t *testing.T) {
	f := priorityScale()
	got, err := resolvePriorities(f, inTenant(), "khan", "cao", "khong-co", "rat-khan", "khan")
	if err != nil {
		t.Fatal(err)
	}
	if active, ok := got["khan"]; !ok || !active {
		t.Errorf("khan: present=%v active=%v, want active", ok, active)
	}
	if active, ok := got["cao"]; !ok || active {
		t.Errorf("cao: present=%v active=%v, want present and inactive", ok, active)
	}
	for _, absent := range []string{"khong-co", "rat-khan"} {
		if _, ok := got[absent]; ok {
			t.Errorf("%q answered — unknown and another commune's code must be absent", absent)
		}
	}
	if len(f.gotCodes) != 4 {
		t.Errorf("store asked %v — duplicates must be collapsed before the read", f.gotCodes)
	}
}

// The other commune sees its own code and not ours — the commune is the context's, nothing else.
func TestResolveTaskPriorityCodesScopedToContextCommune(t *testing.T) {
	got, err := resolvePriorities(priorityScale(), tenant.Into(context.Background(), otherTenant), "khan", "rat-khan")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got["rat-khan"] {
		t.Errorf("= %v, want only rat-khan", got)
	}
}

// EXACT MATCH: no trim, no lowercase on a non-blank code.
func TestResolveTaskPriorityCodesExactMatch(t *testing.T) {
	f := priorityScale()
	got, err := resolvePriorities(f, inTenant(), " khan", "Khan")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("= %v, want nothing — a normalised match approves a string other than the one stored", got)
	}
	if len(f.gotCodes) != 2 || f.gotCodes[0] != " khan" {
		t.Errorf("store asked %q — the code must reach it unchanged", f.gotCodes)
	}
}

// An empty request answers empty, never "every priority", and reads nothing.
func TestResolveTaskPriorityCodesEmptyRequest(t *testing.T) {
	f := priorityScale()
	got, err := resolvePriorities(f, inTenant())
	if err != nil || len(got) != 0 || f.calls != 0 {
		t.Errorf("got=%v err=%v reads=%d — want empty, no read", got, err, f.calls)
	}
}

func TestResolveTaskPriorityCodesOverCeilingIsInvalidArgument(t *testing.T) {
	sent := make([]string, MaxTaskPriorityCodesPerCall+1)
	for i := range sent {
		sent[i] = "khan" // duplicates: the ceiling counts what was SENT
	}
	f := priorityScale()
	if _, err := resolvePriorities(f, inTenant(), sent...); status.Code(err) != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", status.Code(err))
	}
	if f.calls != 0 {
		t.Error("store read over the ceiling")
	}
	if _, err := resolvePriorities(priorityScale(), inTenant(), sent[:MaxTaskPriorityCodesPerCall]...); err != nil {
		t.Errorf("exactly the ceiling refused: %v", err)
	}
}

func TestResolveTaskPriorityCodesBlankIsInvalidArgument(t *testing.T) {
	for _, blank := range []string{"", "   "} {
		f := priorityScale()
		if _, err := resolvePriorities(f, inTenant(), "khan", blank); status.Code(err) != codes.InvalidArgument {
			t.Errorf("blank %q: code = %v, want InvalidArgument", blank, status.Code(err))
		}
		if f.calls != 0 {
			t.Errorf("blank %q: store was read", blank)
		}
	}
}

func TestResolveTaskPriorityCodesWithoutCommuneIsRefused(t *testing.T) {
	f := priorityScale()
	if _, err := resolvePriorities(f, context.Background(), "khan"); status.Code(err) == codes.OK {
		t.Fatal("answered with no commune")
	}
	if f.calls != 0 {
		t.Error("store read with no commune")
	}
}

func TestResolveTaskPriorityCodesStoreErrorIsInternal(t *testing.T) {
	_, err := resolvePriorities(&fakePriorities{err: errors.New("pq: SELECT … password=secret")}, inTenant(), "khan")
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() != "lỗi nội bộ, vui lòng thử lại" {
		t.Errorf("cause leaked: %q", st.Message())
	}
}

// A row the store returns for a code NOT asked, or a second row for one code, never reaches the answer.
func TestResolveTaskPriorityCodesAnswerIsSubsetOfQuestion(t *testing.T) {
	f := priorityScale()
	f.extra = []domain.TaskPriorityCodeState{{Code: "rat-khan", Active: true}, {Code: "khan", Active: false}}
	got, err := resolvePriorities(f, inTenant(), "khan")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got["khan"] {
		t.Errorf("= %v, want only khan, active (first answer kept)", got)
	}
}
