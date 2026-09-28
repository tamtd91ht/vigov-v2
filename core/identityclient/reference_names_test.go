package identityclient

// What these tests defend on THIS side of the wire, for the three register lookups: the commune
// travels in metadata, an outage is never an answer (and the transient codes carry the retryable
// sentinel), a key nobody asked for is a contract fault, a malformed item is a contract fault, the
// ceiling is refused rather than sent, and an empty request never reaches the network.

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

type fakeLookupServer struct {
	identityv1.UnimplementedIdentityServiceServer

	names     []*identityv1.OrgUnitName
	labels    []*identityv1.TaskBlocLabel
	matches   []*identityv1.OrgUnitCodeMatch
	err       error
	calls     int
	sawTenant []string
}

func (s *fakeLookupServer) record(ctx context.Context) {
	s.calls++
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		s.sawTenant = md.Get(grpcx.MetadataTenantKey)
	}
}

func (s *fakeLookupServer) ResolveOrgUnitNames(ctx context.Context, _ *identityv1.ResolveOrgUnitNamesRequest) (
	*identityv1.ResolveOrgUnitNamesResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.ResolveOrgUnitNamesResponse{Items: s.names}, nil
}

func (s *fakeLookupServer) ResolveTaskBlocLabels(ctx context.Context, _ *identityv1.ResolveTaskBlocLabelsRequest) (
	*identityv1.ResolveTaskBlocLabelsResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.ResolveTaskBlocLabelsResponse{Items: s.labels}, nil
}

func (s *fakeLookupServer) ResolveLiveOrgUnitCodes(ctx context.Context, _ *identityv1.ResolveLiveOrgUnitCodesRequest) (
	*identityv1.ResolveLiveOrgUnitCodesResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.ResolveLiveOrgUnitCodesResponse{Items: s.matches}, nil
}

const (
	live    = identityv1.RecordStanding_RECORD_STANDING_LIVE
	removed = identityv1.RecordStanding_RECORD_STANDING_REMOVED
)

func TestLookupRPCsAreNotTenantExempt(t *testing.T) {
	for _, m := range []string{
		identityv1.IdentityService_ResolveOrgUnitNames_FullMethodName,
		identityv1.IdentityService_ResolveTaskBlocLabels_FullMethodName,
		identityv1.IdentityService_ResolveLiveOrgUnitCodes_FullMethodName,
	} {
		if grpcx.ExemptFromTenant(m) {
			t.Errorf("%s nằm trong danh sách miễn xã", m)
		}
	}
}

func TestOrgUnitNamesMapsByIDIncludingRemoved(t *testing.T) {
	srv := &fakeLookupServer{names: []*identityv1.OrgUnitName{
		{Id: "bp-1", Name: "VĂN PHÒNG", Standing: live},
		{Id: "bp-2", Name: "BỘ PHẬN CŨ", Standing: removed},
	}}
	c := moMay(t, srv)

	got, err := c.OrgUnitNames(ngucCanh(), []string{"bp-1", "bp-2", "bp-3"})
	if err != nil {
		t.Fatalf("OrgUnitNames: %v", err)
	}
	if got["bp-1"].Name != "VĂN PHÒNG" || got["bp-1"].Standing != live {
		t.Errorf("bp-1 = %+v", got["bp-1"])
	}
	if got["bp-2"].Standing != removed {
		t.Errorf("bp-2 = %+v, muốn REMOVED", got["bp-2"])
	}
	if _, ok := got["bp-3"]; ok {
		t.Error("id không được trả lời lại có trong bản đồ")
	}
	assertTenantOnWire(t, srv.sawTenant)
}

func TestTaskBlocLabelsMapsByCode(t *testing.T) {
	srv := &fakeLookupServer{labels: []*identityv1.TaskBlocLabel{
		{Ma: "khoi-uy-ban", Label: "Khối Uỷ ban", Standing: live},
	}}
	c := moMay(t, srv)

	got, err := c.TaskBlocLabels(ngucCanh(), []string{"khoi-uy-ban", "khac"})
	if err != nil {
		t.Fatalf("TaskBlocLabels: %v", err)
	}
	if len(got) != 1 || got["khoi-uy-ban"].Label != "Khối Uỷ ban" {
		t.Errorf("nhãn = %+v", got)
	}
	assertTenantOnWire(t, srv.sawTenant)
}

func TestLiveOrgUnitIDsByCodeMapsCodeToID(t *testing.T) {
	srv := &fakeLookupServer{matches: []*identityv1.OrgUnitCodeMatch{{Ma: "van-phong", Id: "bp-1"}}}
	c := moMay(t, srv)

	got, err := c.LiveOrgUnitIDsByCode(ngucCanh(), []string{"van-phong", "da-xoa"})
	if err != nil {
		t.Fatalf("LiveOrgUnitIDsByCode: %v", err)
	}
	if len(got) != 1 || got["van-phong"] != "bp-1" {
		t.Errorf("khớp = %v, muốn chỉ van-phong→bp-1", got)
	}
	assertTenantOnWire(t, srv.sawTenant)
}

// Every call through the three wrappers, so each case below exercises all of them.
func callAll(c *Client, keys []string) []error {
	_, e1 := c.OrgUnitNames(ngucCanh(), keys)
	_, e2 := c.TaskBlocLabels(ngucCanh(), keys)
	_, e3 := c.LiveOrgUnitIDsByCode(ngucCanh(), keys)
	return []error{e1, e2, e3}
}

func TestLookupsUnavailableIsRetryableSentinelNotAnAnswer(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		c := moMay(t, &fakeLookupServer{err: status.Error(code, "identity đang xuống")})
		for i, err := range callAll(c, []string{"k"}) {
			if !errors.Is(err, ErrIdentityUnavailable) {
				t.Errorf("%v, lời gọi %d: không mang ErrIdentityUnavailable: %v", code, i, err)
			}
			if status.Code(err) != code {
				t.Errorf("%v, lời gọi %d: mã = %v, muốn giữ nguyên", code, i, status.Code(err))
			}
		}
	}
}

func TestLookupsNonTransientFailureIsErrorButNotRetryable(t *testing.T) {
	c := moMay(t, &fakeLookupServer{err: status.Error(codes.Unauthenticated, "khoá sai")})
	for i, err := range callAll(c, []string{"k"}) {
		if err == nil || errors.Is(err, ErrIdentityUnavailable) {
			t.Errorf("lời gọi %d: err = %v, muốn lỗi thường, không phải lỗi thử lại được", i, err)
		}
	}
}

func TestLookupsEmptyRequestNeverReachesNetwork(t *testing.T) {
	srv := &fakeLookupServer{}
	c := moMay(t, srv)
	for _, keys := range [][]string{nil, {}, {"", ""}} {
		for i, err := range callAll(c, keys) {
			if err != nil {
				t.Errorf("lời gọi %d với %q: %v", i, keys, err)
			}
		}
	}
	if srv.calls != 0 {
		t.Errorf("yêu cầu rỗng đã lên mạng %d lần", srv.calls)
	}
}

func TestLookupsOverCeilingRefusedLocally(t *testing.T) {
	srv := &fakeLookupServer{}
	c := moMay(t, srv)
	keys := make([]string, MaxLookupKeysPerCall+1)
	for i := range keys {
		keys[i] = "k"
	}
	for i, err := range callAll(c, keys) {
		if err == nil {
			t.Errorf("lời gọi %d: vượt trần mà không báo lỗi", i)
		}
	}
	if srv.calls != 0 {
		t.Error("yêu cầu vượt trần đã được gửi")
	}
}

func TestLookupsUnaskedKeyIsContractFault(t *testing.T) {
	c := moMay(t, &fakeLookupServer{
		names:   []*identityv1.OrgUnitName{{Id: "khong-hoi", Name: "X", Standing: live}},
		labels:  []*identityv1.TaskBlocLabel{{Ma: "khong-hoi", Label: "X", Standing: live}},
		matches: []*identityv1.OrgUnitCodeMatch{{Ma: "khong-hoi", Id: "bp-9"}},
	})
	for i, err := range callAll(c, []string{"k"}) {
		if err == nil {
			t.Errorf("lời gọi %d: phản hồi mang khoá không được hỏi mà không báo lỗi", i)
		}
	}
}

func TestLookupsMalformedItemIsContractFault(t *testing.T) {
	cases := map[string]*fakeLookupServer{
		"tên rỗng":             {names: []*identityv1.OrgUnitName{{Id: "k", Standing: live}}},
		"tên thiếu trạng thái": {names: []*identityv1.OrgUnitName{{Id: "k", Name: "X"}}},
	}
	for ten, srv := range cases {
		if _, err := moMay(t, srv).OrgUnitNames(ngucCanh(), []string{"k"}); err == nil {
			t.Errorf("OrgUnitNames, %s: không báo lỗi hợp đồng", ten)
		}
	}
	for ten, srv := range map[string]*fakeLookupServer{
		"nhãn rỗng":             {labels: []*identityv1.TaskBlocLabel{{Ma: "k", Standing: live}}},
		"nhãn thiếu trạng thái": {labels: []*identityv1.TaskBlocLabel{{Ma: "k", Label: "X"}}},
	} {
		if _, err := moMay(t, srv).TaskBlocLabels(ngucCanh(), []string{"k"}); err == nil {
			t.Errorf("TaskBlocLabels, %s: không báo lỗi hợp đồng", ten)
		}
	}
	for ten, srv := range map[string]*fakeLookupServer{
		"id rỗng": {matches: []*identityv1.OrgUnitCodeMatch{{Ma: "k"}}},
		"hai id cho một mã": {matches: []*identityv1.OrgUnitCodeMatch{
			{Ma: "k", Id: "bp-1"}, {Ma: "k", Id: "bp-2"},
		}},
	} {
		if _, err := moMay(t, srv).LiveOrgUnitIDsByCode(ngucCanh(), []string{"k"}); err == nil {
			t.Errorf("LiveOrgUnitIDsByCode, %s: không báo lỗi hợp đồng", ten)
		}
	}
}
