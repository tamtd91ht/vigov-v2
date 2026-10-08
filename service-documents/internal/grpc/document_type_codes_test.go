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

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// fakeTypes holds live document types PER COMMUNE, so "another commune's code" is a real row the
// handler must not answer. Soft-deleted types are simply not in the map — the store's predicate
// (`deleted_at IS NULL`) is proved in store/document_type_codes_*_test.go; here the point is that a
// code the store does not return is ABSENT, not an error and not "active".
type fakeTypes struct {
	byTenant map[tenant.ID]map[string]bool
	extra    []domain.DocumentTypeCodeState // returned whatever was asked — a misbehaving store
	err      error
	calls    int
	gotCodes []string
}

func (f *fakeTypes) StatesByCode(ctx context.Context, codes []string) ([]domain.DocumentTypeCodeState, error) {
	f.calls++
	f.gotCodes = append([]string(nil), codes...)
	if f.err != nil {
		return nil, f.err
	}
	live := f.byTenant[tenant.MustFrom(ctx)]
	var out []domain.DocumentTypeCodeState
	for _, c := range codes {
		if active, ok := live[c]; ok {
			out = append(out, domain.DocumentTypeCodeState{Code: c, Active: active})
		}
	}
	return append(out, f.extra...), nil
}

const otherTenant = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EG")

func catalogue() *fakeTypes {
	return &fakeTypes{byTenant: map[tenant.ID]map[string]bool{
		testTenant:  {"cong-van": true, "to-trinh": false},
		otherTenant: {"quyet-dinh": true},
	}}
}

func typesServer(f *fakeTypes) *Server {
	return NewServer(Deps{Incoming: &fakeCounter{}, DocumentTypes: f, Letters: &fakeLetters{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func resolve(f *fakeTypes, ctx context.Context, codes ...string) (map[string]bool, error) {
	res, err := typesServer(f).ResolveDocumentTypeCodes(ctx, &documentsv1.ResolveDocumentTypeCodesRequest{Codes: codes})
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
func TestResolveDocumentTypeCodesAnswers(t *testing.T) {
	f := catalogue()
	got, err := resolve(f, inTenant(), "cong-van", "to-trinh", "khong-co", "quyet-dinh", "cong-van")
	if err != nil {
		t.Fatal(err)
	}
	if active, ok := got["cong-van"]; !ok || !active {
		t.Errorf("cong-van: present=%v active=%v, want active", ok, active)
	}
	if active, ok := got["to-trinh"]; !ok || active {
		t.Errorf("to-trinh: present=%v active=%v, want present and inactive", ok, active)
	}
	for _, absent := range []string{"khong-co", "quyet-dinh"} {
		if _, ok := got[absent]; ok {
			t.Errorf("%q answered — unknown and another commune's code must be absent", absent)
		}
	}
	if len(f.gotCodes) != 4 {
		t.Errorf("store asked %v — duplicates must be collapsed before the read", f.gotCodes)
	}
}

// The other commune sees its own code and not ours — the commune is the context's, nothing else.
func TestResolveDocumentTypeCodesScopedToContextCommune(t *testing.T) {
	got, err := resolve(catalogue(), tenant.Into(context.Background(), otherTenant), "cong-van", "quyet-dinh")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got["quyet-dinh"] {
		t.Errorf("= %v, want only quyet-dinh", got)
	}
}

// EXACT MATCH: no trim, no lowercase on a non-blank code.
func TestResolveDocumentTypeCodesExactMatch(t *testing.T) {
	f := catalogue()
	got, err := resolve(f, inTenant(), " cong-van", "Cong-Van")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("= %v, want nothing — a normalised match approves a string other than the one stored", got)
	}
	if len(f.gotCodes) != 2 || f.gotCodes[0] != " cong-van" {
		t.Errorf("store asked %q — the code must reach it unchanged", f.gotCodes)
	}
}

// An empty request answers empty, never "every type", and reads nothing.
func TestResolveDocumentTypeCodesEmptyRequest(t *testing.T) {
	f := catalogue()
	got, err := resolve(f, inTenant())
	if err != nil || len(got) != 0 || f.calls != 0 {
		t.Errorf("got=%v err=%v reads=%d — want empty, no read", got, err, f.calls)
	}
}

func TestResolveDocumentTypeCodesOverCeilingIsInvalidArgument(t *testing.T) {
	sent := make([]string, MaxDocumentTypeCodesPerCall+1)
	for i := range sent {
		sent[i] = "cong-van" // duplicates: the ceiling counts what was SENT
	}
	f := catalogue()
	if _, err := resolve(f, inTenant(), sent...); status.Code(err) != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", status.Code(err))
	}
	if f.calls != 0 {
		t.Error("store read over the ceiling")
	}
	if _, err := resolve(catalogue(), inTenant(), sent[:MaxDocumentTypeCodesPerCall]...); err != nil {
		t.Errorf("exactly the ceiling refused: %v", err)
	}
}

func TestResolveDocumentTypeCodesBlankIsInvalidArgument(t *testing.T) {
	for _, blank := range []string{"", "   "} {
		f := catalogue()
		if _, err := resolve(f, inTenant(), "cong-van", blank); status.Code(err) != codes.InvalidArgument {
			t.Errorf("blank %q: code = %v, want InvalidArgument", blank, status.Code(err))
		}
		if f.calls != 0 {
			t.Errorf("blank %q: store was read", blank)
		}
	}
}

func TestResolveDocumentTypeCodesWithoutCommuneIsRefused(t *testing.T) {
	f := catalogue()
	if _, err := resolve(f, context.Background(), "cong-van"); status.Code(err) == codes.OK {
		t.Fatal("answered with no commune")
	}
	if f.calls != 0 {
		t.Error("store read with no commune")
	}
}

func TestResolveDocumentTypeCodesStoreErrorIsInternal(t *testing.T) {
	_, err := resolve(&fakeTypes{err: errors.New("pq: SELECT … password=secret")}, inTenant(), "cong-van")
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() != "lỗi nội bộ, vui lòng thử lại" {
		t.Errorf("cause leaked: %q", st.Message())
	}
}

// A row the store returns for a code NOT asked, or a second row for one code, never reaches the answer.
func TestResolveDocumentTypeCodesAnswerIsSubsetOfQuestion(t *testing.T) {
	f := catalogue()
	f.extra = []domain.DocumentTypeCodeState{{Code: "quyet-dinh", Active: true}, {Code: "cong-van", Active: false}}
	got, err := resolve(f, inTenant(), "cong-van")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got["cong-van"] {
		t.Errorf("= %v, want only cong-van, active (first answer kept)", got)
	}
}
