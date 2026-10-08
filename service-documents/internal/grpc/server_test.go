package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/tenant"
)

const testTenant = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")

type fakeCounter struct {
	n         int
	err       error
	calls     int
	gotID     string
	gotTenant tenant.ID
}

func (f *fakeCounter) CountOpenHeldByOrgUnit(ctx context.Context, id string) (int, error) {
	f.calls++
	f.gotID = id
	f.gotTenant, _ = tenant.From(ctx)
	return f.n, f.err
}

func newTestServer(f *fakeCounter) *Server {
	return NewServer(Deps{Incoming: f, DocumentTypes: &fakeTypes{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func TestCountOrgUnitHoldingsAnswersInContextCommune(t *testing.T) {
	f := &fakeCounter{n: 7}
	res, err := newTestServer(f).CountOrgUnitHoldings(tenant.Into(context.Background(), testTenant),
		&documentsv1.CountOrgUnitHoldingsRequest{OrgUnitId: " bp-1 "})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetOpenIncomingDocuments() != 7 {
		t.Errorf("= %+v, want 7", res)
	}
	if f.gotID != "bp-1" || f.gotTenant != testTenant {
		t.Errorf("asked id=%q tenant=%q", f.gotID, f.gotTenant)
	}
}

func TestCountOrgUnitHoldingsBlankIDIsInvalidArgument(t *testing.T) {
	for _, id := range []string{"", "  "} {
		f := &fakeCounter{}
		_, err := newTestServer(f).CountOrgUnitHoldings(tenant.Into(context.Background(), testTenant),
			&documentsv1.CountOrgUnitHoldingsRequest{OrgUnitId: id})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("id %q: code = %v, want InvalidArgument", id, status.Code(err))
		}
		if f.calls != 0 {
			t.Errorf("id %q: store was read", id)
		}
	}
}

func TestCountOrgUnitHoldingsWithoutCommuneIsRefused(t *testing.T) {
	f := &fakeCounter{n: 1}
	_, err := newTestServer(f).CountOrgUnitHoldings(context.Background(),
		&documentsv1.CountOrgUnitHoldingsRequest{OrgUnitId: "bp-1"})
	if status.Code(err) == codes.OK {
		t.Fatal("counted with no commune")
	}
	if f.calls != 0 {
		t.Error("store was read with no commune")
	}
}

func TestCountOrgUnitHoldingsStoreErrorIsInternalNotZero(t *testing.T) {
	res, err := newTestServer(&fakeCounter{err: errors.New("pq: SELECT … password=secret")}).CountOrgUnitHoldings(
		tenant.Into(context.Background(), testTenant), &documentsv1.CountOrgUnitHoldingsRequest{OrgUnitId: "bp-1"})
	if status.Code(err) != codes.Internal || res != nil {
		t.Errorf("res=%v code=%v, want Internal and no answer", res, status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() != "lỗi nội bộ, vui lòng thử lại" {
		t.Errorf("cause leaked: %q", st.Message())
	}
}

func TestClampUint32NeverWraps(t *testing.T) {
	if clampUint32(-1) != 0 || clampUint32(3) != 3 || clampUint32(math.MaxUint32+1) != math.MaxUint32 {
		t.Error("clamp wrong")
	}
}

func TestNewServerRefusesMissingDeps(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("built a server with missing deps")
		}
	}()
	NewServer(Deps{Incoming: &fakeCounter{}})
}

func TestNewServerRefusesMissingDocumentTypes(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("built a server without the document-type reader")
		}
	}()
	NewServer(Deps{Incoming: &fakeCounter{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}
