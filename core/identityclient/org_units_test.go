package identityclient

// What these tests defend on THIS side of the wire: an outage is never an answer (not "not live",
// not "no unit"), a unit nobody asked for is a contract fault, the ceiling is refused rather than
// sent, and the commune travels in metadata.

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

// fakeOrgUnitServer is identity answering the two org-unit RPCs, and nothing else.
type fakeOrgUnitServer struct {
	identityv1.UnimplementedIdentityServiceServer

	live      []string
	units     []string
	err       error
	calls     int
	sawTenant []string
	sawLive   *identityv1.ResolveLiveOrgUnitsRequest
	sawStaff  *identityv1.ResolveStaffOrgUnitsRequest
}

func (s *fakeOrgUnitServer) record(ctx context.Context) {
	s.calls++
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTenant = md.Get(grpcx.MetadataTenantKey)
	}
}

func (s *fakeOrgUnitServer) ResolveLiveOrgUnits(ctx context.Context, in *identityv1.ResolveLiveOrgUnitsRequest) (
	*identityv1.ResolveLiveOrgUnitsResponse, error) {
	s.record(ctx)
	s.sawLive = in
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.ResolveLiveOrgUnitsResponse{LiveIds: s.live}, nil
}

func (s *fakeOrgUnitServer) ResolveStaffOrgUnits(ctx context.Context, in *identityv1.ResolveStaffOrgUnitsRequest) (
	*identityv1.ResolveStaffOrgUnitsResponse, error) {
	s.record(ctx)
	s.sawStaff = in
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.ResolveStaffOrgUnitsResponse{OrgUnitIds: s.units}, nil
}

func assertTenantOnWire(t *testing.T, got []string) {
	t.Helper()
	if len(got) != 1 || got[0] != string(xaA) {
		t.Errorf("x-tenant-id trên dây = %v, muốn đúng một giá trị %q — RPC này KHÔNG được miễn xã", got, string(xaA))
	}
}

// Neither RPC may be exempt from the commune: exemption turns "a unit of the caller's commune" into
// "a unit of some commune".
func TestOrgUnitRPCsAreNotTenantExempt(t *testing.T) {
	for _, m := range []string{
		identityv1.IdentityService_ResolveLiveOrgUnits_FullMethodName,
		identityv1.IdentityService_ResolveStaffOrgUnits_FullMethodName,
		identityv1.IdentityService_ResolveDueSoonCutoff_FullMethodName,
	} {
		if grpcx.ExemptFromTenant(m) {
			t.Errorf("%s nằm trong danh sách miễn xã", m)
		}
	}
}

// ---- LiveOrgUnits ----

func TestLiveOrgUnitsReturnsSetAndCarriesTenant(t *testing.T) {
	srv := &fakeOrgUnitServer{live: []string{"bp-live"}}
	c := moMay(t, srv)

	live, err := c.LiveOrgUnits(ngucCanh(), []string{"bp-live", "bp-removed"})
	if err != nil {
		t.Fatalf("LiveOrgUnits: %v", err)
	}
	if _, ok := live["bp-live"]; !ok {
		t.Error("bộ phận còn hiệu lực không có trong tập")
	}
	if _, ok := live["bp-removed"]; ok {
		t.Error("BỘ PHẬN KHÔNG ĐƯỢC TRẢ LẠI CÓ TRONG TẬP — việc sẽ giao cho một bộ phận không tồn tại")
	}
	assertTenantOnWire(t, srv.sawTenant)
}

func TestLiveOrgUnitsUnavailableIsRetryableSentinelNotAnAnswer(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		c := moMay(t, &fakeOrgUnitServer{err: status.Error(code, "identity đang xuống")})
		live, err := c.LiveOrgUnits(ngucCanh(), []string{"bp-a"})
		if err == nil {
			t.Fatalf("%v mà không báo lỗi", code)
		}
		if live != nil {
			t.Errorf("trả về tập %v kèm lỗi — bên gọi có thể quyết định từ nó", live)
		}
		if !errors.Is(err, ErrIdentityUnavailable) {
			t.Errorf("%v không mang ErrIdentityUnavailable: %v", code, err)
		}
		if status.Code(err) != code {
			t.Errorf("mã = %v, muốn giữ nguyên %v", status.Code(err), code)
		}
	}
}

// A non-transient failure is still an error, but NOT the retryable sentinel: retrying a wrong caller
// key changes nothing.
func TestLiveOrgUnitsOtherErrorsAreNotRetryable(t *testing.T) {
	for _, e := range []error{status.Error(codes.InvalidArgument, "quá trần"), errors.New("giả lập: hỏng")} {
		c := moMay(t, &fakeOrgUnitServer{err: e})
		live, err := c.LiveOrgUnits(ngucCanh(), []string{"bp-a"})
		if err == nil || live != nil {
			t.Fatalf("lỗi %v: err=%v live=%v", e, err, live)
		}
		if errors.Is(err, ErrIdentityUnavailable) {
			t.Errorf("lỗi không tạm thời %v bị đánh dấu thử lại được", e)
		}
	}
}

func TestLiveOrgUnitsUnaskedIDIsContractFault(t *testing.T) {
	for _, extra := range []string{"bp-nobody-asked", ""} {
		c := moMay(t, &fakeOrgUnitServer{live: []string{"bp-a", extra}})
		if _, err := c.LiveOrgUnits(ngucCanh(), []string{"bp-a"}); err == nil {
			t.Fatalf("phản hồi mang bộ phận chưa được hỏi %q mà không báo lỗi", extra)
		}
	}
}

func TestLiveOrgUnitsEmptyDoesNotCall(t *testing.T) {
	for _, ids := range [][]string{nil, {"", ""}} {
		srv := &fakeOrgUnitServer{live: []string{"bp-x"}}
		c := moMay(t, srv)
		live, err := c.LiveOrgUnits(ngucCanh(), ids)
		if err != nil {
			t.Fatalf("yêu cầu rỗng bị báo lỗi: %v", err)
		}
		if len(live) != 0 || srv.calls != 0 {
			t.Errorf("số bộ phận = %d, số lần gọi = %d — yêu cầu rỗng KHÔNG được thành 'tất cả'", len(live), srv.calls)
		}
	}
}

// Over the ceiling is REFUSED LOCALLY: not sent, not chunked, not truncated.
func TestLiveOrgUnitsOverCeilingRefusedLocally(t *testing.T) {
	srv := &fakeOrgUnitServer{}
	c := moMay(t, srv)

	ids := make([]string, MaxOrgUnitIDsPerCall+1)
	for i := range ids {
		ids[i] = "bp-over"
	}
	if _, err := c.LiveOrgUnits(ngucCanh(), ids); err == nil {
		t.Fatal("quá trần mà không báo lỗi")
	}
	if srv.calls != 0 {
		t.Errorf("đã gửi %d yêu cầu chỉ có thể thất bại", srv.calls)
	}

	// Exactly at the ceiling is sent.
	if _, err := c.LiveOrgUnits(ngucCanh(), ids[:MaxOrgUnitIDsPerCall]); err != nil {
		t.Fatalf("đúng trần bị từ chối: %v", err)
	}
	if srv.calls != 1 {
		t.Errorf("số lần gọi = %d, muốn 1", srv.calls)
	}
}

func TestLiveOrgUnitsWithoutTenantRefusedBeforeWire(t *testing.T) {
	srv := &fakeOrgUnitServer{}
	c := moMay(t, srv)
	if _, err := c.LiveOrgUnits(context.Background(), []string{"bp-a"}); err == nil {
		t.Fatal("gọi được ResolveLiveOrgUnits mà không có xã trong context")
	}
	if srv.calls != 0 {
		t.Errorf("lời gọi tới được máy chủ %d lần dù không mang xã", srv.calls)
	}
}

// ---- StaffOrgUnits ----

func TestStaffOrgUnitsReturnsUnitsAndCarriesTenantAndCode(t *testing.T) {
	srv := &fakeOrgUnitServer{units: []string{"bp-1", "bp-1"}}
	c := moMay(t, srv)

	units, err := c.StaffOrgUnits(ngucCanh(), "CB-00123")
	if err != nil {
		t.Fatalf("StaffOrgUnits: %v", err)
	}
	if len(units) != 1 || units[0] != "bp-1" {
		t.Errorf("units = %v, muốn [bp-1] (trùng lặp gộp lại)", units)
	}
	if srv.sawStaff.GetMa() != "CB-00123" {
		t.Errorf("ma trên dây = %q", srv.sawStaff.GetMa())
	}
	assertTenantOnWire(t, srv.sawTenant)
}

// Empty is an ordinary answer and stays empty — not nil-and-an-error, not "every unit".
func TestStaffOrgUnitsEmptyIsOrdinary(t *testing.T) {
	c := moMay(t, &fakeOrgUnitServer{})
	units, err := c.StaffOrgUnits(ngucCanh(), "CB-00123")
	if err != nil {
		t.Fatalf("trả rỗng bị báo lỗi: %v", err)
	}
	if units == nil || len(units) != 0 {
		t.Errorf("units = %#v, muốn slice rỗng khác nil", units)
	}
}

func TestStaffOrgUnitsUnavailableIsRetryableSentinel(t *testing.T) {
	c := moMay(t, &fakeOrgUnitServer{err: status.Error(codes.Unavailable, "identity đang xuống")})
	units, err := c.StaffOrgUnits(ngucCanh(), "CB-00123")
	if err == nil {
		t.Fatal("UNAVAILABLE mà không báo lỗi — tab sẽ mất vế bộ phận mà không ai biết")
	}
	if units != nil {
		t.Errorf("trả về %v kèm lỗi", units)
	}
	if !errors.Is(err, ErrIdentityUnavailable) {
		t.Errorf("không mang ErrIdentityUnavailable: %v", err)
	}
}

func TestStaffOrgUnitsBlankUnitIsContractFault(t *testing.T) {
	c := moMay(t, &fakeOrgUnitServer{units: []string{"bp-1", ""}})
	if _, err := c.StaffOrgUnits(ngucCanh(), "CB-00123"); err == nil {
		t.Fatal("mã bộ phận rỗng trong phản hồi mà không báo lỗi")
	}
}

func TestStaffOrgUnitsEmptyCodeRefusedLocally(t *testing.T) {
	srv := &fakeOrgUnitServer{}
	c := moMay(t, srv)
	if _, err := c.StaffOrgUnits(ngucCanh(), ""); err == nil {
		t.Fatal("mã cán bộ rỗng mà không báo lỗi")
	}
	if srv.calls != 0 {
		t.Errorf("đã gửi %d yêu cầu chỉ có thể thất bại", srv.calls)
	}
}
