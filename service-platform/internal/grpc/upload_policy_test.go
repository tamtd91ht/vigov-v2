package grpc_test

// ListUploadPolicies over bufconn with the real commune interceptor, fake store.

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svcgrpc "github.com/vihat/vigov/service-platform/internal/grpc"
)

// policiesFake returns rows as the store would: already without soft-deleted ones.
type policiesFake struct {
	rows   []domain.UploadPolicy
	broken bool
}

func (p policiesFake) ListUploadPolicies(context.Context) ([]domain.UploadPolicy, error) {
	if p.broken {
		return nil, loiHaTang
	}
	return p.rows, nil
}

var policyUpdatedAt = time.Date(2026, 9, 28, 1, 2, 3, 0, time.UTC)

func samplePolicies() policiesFake {
	return policiesFake{rows: []domain.UploadPolicy{
		{Purpose: "content-video", MaxBytes: 2147483648,
			AllowedMIMETypes: []string{"video/mp4", "video/quicktime"}, UpdatedAt: policyUpdatedAt},
		{Purpose: "petition-photo", MaxBytes: 10485760,
			AllowedMIMETypes: []string{"image/jpeg", "image/png", "image/webp", "image/heic"},
			FileCountLimited: true, MaxFilesPerSubject: 5, UpdatedAt: policyUpdatedAt},
	}}
}

func policyClient(t *testing.T, p svcgrpc.UploadPolicies) platformv1.PlatformServiceClient {
	t.Helper()
	cli, _ := dungVoi(t, svcgrpc.Deps{Dir: danhBaMau(), Apps: soMiniAppMau(), HoSo: hoSoMau(), Policies: p,
		Fields: sampleFields()})
	return cli
}

func TestListUploadPoliciesMapsFieldByField(t *testing.T) {
	t.Parallel()
	res, err := policyClient(t, samplePolicies()).ListUploadPolicies(tenant.Into(ctxTest(t), xaTanPhu),
		&platformv1.ListUploadPoliciesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	ps := res.GetPolicies()
	if len(ps) != 2 {
		t.Fatalf("policies = %d, want 2", len(ps))
	}

	video := ps[0]
	if video.GetPurpose() != platformv1.UploadPurpose_UPLOAD_PURPOSE_CONTENT_VIDEO ||
		video.GetMaxBytes() != 2147483648 || len(video.GetAllowedMimeTypes()) != 2 ||
		!video.GetUpdatedAt().AsTime().Equal(policyUpdatedAt) {
		t.Errorf("video = %+v", video)
	}
	// No count limit is ABSENT, not 0 (platform.proto: presence is the signal).
	if video.MaxFilesPerSubject != nil {
		t.Errorf("video max_files_per_subject present (%d); want absent", video.GetMaxFilesPerSubject())
	}

	photo := ps[1]
	if photo.GetPurpose() != platformv1.UploadPurpose_UPLOAD_PURPOSE_PETITION_PHOTO ||
		photo.MaxFilesPerSubject == nil || photo.GetMaxFilesPerSubject() != 5 {
		t.Errorf("photo = %+v", photo)
	}
}

// A row whose purpose the enum cannot name is skipped — callers then read it as not configured —
// and the rest of the answer still arrives.
func TestListUploadPoliciesSkipsUnknownPurpose(t *testing.T) {
	t.Parallel()
	p := samplePolicies()
	p.rows = append(p.rows,
		domain.UploadPolicy{Purpose: "retired-purpose", MaxBytes: 1, AllowedMIMETypes: []string{"image/png"}},
		domain.UploadPolicy{Purpose: "Content-Image", MaxBytes: 1, AllowedMIMETypes: []string{"image/png"}})
	res, err := policyClient(t, p).ListUploadPolicies(tenant.Into(ctxTest(t), xaTanPhu),
		&platformv1.ListUploadPoliciesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GetPolicies()) != 2 {
		t.Fatalf("policies = %+v; want the two known ones only", res.GetPolicies())
	}
}

// Empty is an ordinary answer: OK with no entries, never NOT_FOUND.
func TestListUploadPoliciesEmptyIsOK(t *testing.T) {
	t.Parallel()
	res, err := policyClient(t, policiesFake{}).ListUploadPolicies(tenant.Into(ctxTest(t), xaTanPhu),
		&platformv1.ListUploadPoliciesRequest{})
	if err != nil || len(res.GetPolicies()) != 0 {
		t.Fatalf("res=%+v err=%v; want OK and empty", res, err)
	}
}

// The same answer for every commune today — the override is open, so there is nothing to vary.
func TestListUploadPoliciesSameForEveryCommune(t *testing.T) {
	t.Parallel()
	cli := policyClient(t, samplePolicies())
	a, errA := cli.ListUploadPolicies(tenant.Into(ctxTest(t), xaTanPhu), &platformv1.ListUploadPoliciesRequest{})
	b, errB := cli.ListUploadPolicies(tenant.Into(ctxTest(t), xaDaSapNhap), &platformv1.ListUploadPoliciesRequest{})
	if errA != nil || errB != nil || len(a.GetPolicies()) != len(b.GetPolicies()) {
		t.Fatalf("a=%v/%v b=%v/%v", a, errA, b, errB)
	}
}

// The commune is required: the raw client sends no "x-tenant-id" and the server's interceptor
// refuses.
func TestListUploadPoliciesWithoutCommuneRefused(t *testing.T) {
	t.Parallel()
	_, raw := dungVoi(t, svcgrpc.Deps{Dir: danhBaMau(), Apps: soMiniAppMau(), HoSo: hoSoMau(),
		Policies: samplePolicies(), Fields: sampleFields()})
	_, err := raw.ListUploadPolicies(ctxTest(t), &platformv1.ListUploadPoliciesRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}

// An outage is Internal — never an empty OK, which callers would read as "nothing allowed" and
// nobody would page for — and its cause does not cross the boundary.
func TestListUploadPoliciesStoreFailureIsInternal(t *testing.T) {
	t.Parallel()
	_, err := policyClient(t, policiesFake{broken: true}).ListUploadPolicies(tenant.Into(ctxTest(t), xaTanPhu),
		&platformv1.ListUploadPoliciesRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() == loiHaTang.Error() {
		t.Error("infrastructure error leaked to the caller")
	}
}
