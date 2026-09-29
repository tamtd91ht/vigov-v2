package grpc_test

// ListPetitionFields over bufconn with the real commune interceptor, fake store.

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-platform/internal/grpc"
)

// fieldsFake returns rows as the store would: every code, retired ones included, in read order.
type fieldsFake struct {
	rows   []domain.CitizenReportField
	broken bool
}

func (f fieldsFake) ListCitizenReportFields(context.Context) ([]domain.CitizenReportField, error) {
	if f.broken {
		return nil, errInfra
	}
	return f.rows, nil
}

func sampleFields() fieldsFake {
	return fieldsFake{rows: []domain.CitizenReportField{
		{Code: "rac-thai", DefaultLabel: "Rác thải – Vệ sinh môi trường", SortOrder: 1, Icon: "Trash2",
			Tone: "orange", IsActive: true},
		{Code: "ma-da-ngung", DefaultLabel: "Mã đã ngừng", SortOrder: 13, IsActive: false},
	}}
}

func fieldsClient(t *testing.T, f svcgrpc.CitizenReportFields) (platformv1.PlatformServiceClient, platformv1.PlatformServiceClient) {
	t.Helper()
	return startWith(t, svcgrpc.Deps{Dir: sampleDirectory(), Apps: sampleMiniApps(), Profiles: sampleProfiles(),
		Policies: samplePolicies(), Fields: f})
}

// Every column crosses, and a RETIRED code is returned with active=false — old petitions still need
// its label (platform.proto; ADR 0060 §4).
func TestListPetitionFieldsMapsFieldByFieldIncludingRetired(t *testing.T) {
	t.Parallel()
	cli, _ := fieldsClient(t, sampleFields())
	res, err := cli.ListPetitionFields(tenant.Into(testCtx(t), tenantActive), &platformv1.ListPetitionFieldsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	fs := res.GetFields()
	if len(fs) != 2 {
		t.Fatalf("fields = %d, want 2 (retired included)", len(fs))
	}
	f := fs[0]
	if f.GetCode() != "rac-thai" || f.GetDefaultLabel() != "Rác thải – Vệ sinh môi trường" ||
		f.GetSortOrder() != 1 || f.GetIcon() != "Trash2" || f.GetTone() != "orange" || !f.GetActive() {
		t.Errorf("rac-thai = %+v", f)
	}
	if r := fs[1]; r.GetCode() != "ma-da-ngung" || r.GetActive() || r.GetIcon() != "" || r.GetTone() != "" {
		t.Errorf("retired = %+v; want present, active=false, icon/tone empty", r)
	}
}

// Empty is OK with no entries — never NOT_FOUND.
func TestListPetitionFieldsEmptyIsOK(t *testing.T) {
	t.Parallel()
	cli, _ := fieldsClient(t, fieldsFake{})
	res, err := cli.ListPetitionFields(tenant.Into(testCtx(t), tenantActive), &platformv1.ListPetitionFieldsRequest{})
	if err != nil || len(res.GetFields()) != 0 {
		t.Fatalf("res=%+v err=%v; want OK and empty", res, err)
	}
}

// Tier 1 is one set for every commune.
func TestListPetitionFieldsSameForEveryCommune(t *testing.T) {
	t.Parallel()
	cli, _ := fieldsClient(t, sampleFields())
	a, errA := cli.ListPetitionFields(tenant.Into(testCtx(t), tenantActive), &platformv1.ListPetitionFieldsRequest{})
	b, errB := cli.ListPetitionFields(tenant.Into(testCtx(t), tenantMerged), &platformv1.ListPetitionFieldsRequest{})
	if errA != nil || errB != nil || len(a.GetFields()) != len(b.GetFields()) {
		t.Fatalf("a=%v/%v b=%v/%v", a, errA, b, errB)
	}
}

// The commune is required: the raw client sends no "x-tenant-id" and the interceptor refuses.
func TestListPetitionFieldsWithoutCommuneRefused(t *testing.T) {
	t.Parallel()
	_, raw := fieldsClient(t, sampleFields())
	_, err := raw.ListPetitionFields(testCtx(t), &platformv1.ListPetitionFieldsRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}

// An outage is Internal — never an empty OK, which callers read as "no code is valid" — and its
// cause does not cross the boundary.
func TestListPetitionFieldsStoreFailureIsInternal(t *testing.T) {
	t.Parallel()
	cli, _ := fieldsClient(t, fieldsFake{broken: true})
	_, err := cli.ListPetitionFields(tenant.Into(testCtx(t), tenantActive), &platformv1.ListPetitionFieldsRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() == errInfra.Error() {
		t.Error("infrastructure error leaked to the caller")
	}
}

// A server built without the field store is refused at construction, not at the first call.
func TestNewServerWithoutCitizenReportFieldsPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("built a server without Fields")
		}
	}()
	_ = svcgrpc.NewServer(svcgrpc.Deps{Dir: sampleDirectory(), Apps: sampleMiniApps(), Profiles: sampleProfiles(),
		Policies: samplePolicies()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
