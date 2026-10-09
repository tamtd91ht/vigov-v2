package identityclient

// What these tests defend on THIS side of the wire, for the two residential-unit lookups (ADR 0088):
// neither RPC is exempt from the commune, the commune travels in metadata, an outage is never an
// answer, a key nobody asked for or an empty name is a contract fault, the ceiling is refused rather
// than sent, an empty request never reaches the network, and the batching helper splits into calls
// within the ceiling and fails whole when one call fails.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
)

// fakeResidentialUnitServer reuses fakeLookupServer for record(), sawTenant, err and the embedded
// UnimplementedIdentityServiceServer.
type fakeResidentialUnitServer struct {
	fakeLookupServer

	active []*identityv1.ActiveResidentialUnit
	names  []*identityv1.ResidentialUnitName
	// echoNames answers every requested id as a live unit named after it — for the batching tests.
	echoNames bool
	failOn    int // fail the Nth call (1-based) with Unavailable; 0 = never
	sizes     []int
}

func (s *fakeResidentialUnitServer) ResolveActiveResidentialUnits(ctx context.Context,
	_ *identityv1.ResolveActiveResidentialUnitsRequest) (*identityv1.ResolveActiveResidentialUnitsResponse, error) {
	s.record(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &identityv1.ResolveActiveResidentialUnitsResponse{Items: s.active}, nil
}

func (s *fakeResidentialUnitServer) ResolveResidentialUnitNames(ctx context.Context,
	req *identityv1.ResolveResidentialUnitNamesRequest) (*identityv1.ResolveResidentialUnitNamesResponse, error) {
	s.record(ctx)
	s.sizes = append(s.sizes, len(req.GetIds()))
	if s.err != nil {
		return nil, s.err
	}
	if s.failOn != 0 && s.calls == s.failOn {
		return nil, status.Error(codes.Unavailable, "identity đang xuống")
	}
	if s.echoNames {
		items := make([]*identityv1.ResidentialUnitName, 0, len(req.GetIds()))
		for _, id := range req.GetIds() {
			items = append(items, &identityv1.ResidentialUnitName{Id: id, Name: "Thôn " + id, Standing: live})
		}
		return &identityv1.ResolveResidentialUnitNamesResponse{Items: items}, nil
	}
	return &identityv1.ResolveResidentialUnitNamesResponse{Items: s.names}, nil
}

func TestResidentialUnitRPCsAreNotTenantExempt(t *testing.T) {
	for _, m := range []string{
		identityv1.IdentityService_ResolveActiveResidentialUnits_FullMethodName,
		identityv1.IdentityService_ResolveResidentialUnitNames_FullMethodName,
	} {
		if grpcx.ExemptFromTenant(m) {
			t.Errorf("%s nằm trong danh sách miễn xã", m)
		}
	}
}

func TestActiveResidentialUnitsMapsIDToName(t *testing.T) {
	srv := &fakeResidentialUnitServer{active: []*identityv1.ActiveResidentialUnit{{Id: "tt-1", Name: "Thôn Bình An"}}}
	c := moMay(t, srv)

	got, err := c.ActiveResidentialUnits(ngucCanh(), []string{"tt-1", "tt-off", "tt-1", ""})
	if err != nil {
		t.Fatalf("ActiveResidentialUnits: %v", err)
	}
	if len(got) != 1 || got["tt-1"] != "Thôn Bình An" {
		t.Errorf("đơn vị đang dùng = %v, muốn chỉ tt-1", got)
	}
	assertTenantOnWire(t, srv.sawTenant)
}

func TestResidentialUnitNamesMapsByIDIncludingRemoved(t *testing.T) {
	srv := &fakeResidentialUnitServer{names: []*identityv1.ResidentialUnitName{
		{Id: "tt-1", Name: "Thôn Bình An", Standing: live},
		{Id: "tt-2", Name: "Thôn Cũ", Standing: removed},
	}}
	c := moMay(t, srv)

	got, err := c.ResidentialUnitNames(ngucCanh(), []string{"tt-1", "tt-2", "tt-3"})
	if err != nil {
		t.Fatalf("ResidentialUnitNames: %v", err)
	}
	if got["tt-1"].Name != "Thôn Bình An" || got["tt-1"].Standing != live {
		t.Errorf("tt-1 = %+v", got["tt-1"])
	}
	if got["tt-2"].Standing != removed {
		t.Errorf("tt-2 = %+v, muốn REMOVED", got["tt-2"])
	}
	if _, ok := got["tt-3"]; ok {
		t.Error("id không được trả lời lại có trong bản đồ")
	}
	assertTenantOnWire(t, srv.sawTenant)
}

func callUnitLookups(c *Client, ids []string) []error {
	_, e1 := c.ActiveResidentialUnits(ngucCanh(), ids)
	_, e2 := c.ResidentialUnitNames(ngucCanh(), ids)
	_, e3 := c.ResidentialUnitNamesInBatches(ngucCanh(), ids)
	return []error{e1, e2, e3}
}

func TestResidentialUnitLookupsFailedCallIsErrorNeverAbsent(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		srv := &fakeResidentialUnitServer{}
		srv.err = status.Error(code, "identity đang xuống")
		for i, err := range callUnitLookups(moMay(t, srv), []string{"tt-1"}) {
			if !errors.Is(err, ErrIdentityUnavailable) {
				t.Errorf("%v, lời gọi %d: không mang ErrIdentityUnavailable: %v", code, i, err)
			}
		}
	}
	srv := &fakeResidentialUnitServer{}
	srv.err = status.Error(codes.InvalidArgument, "vượt trần")
	for i, err := range callUnitLookups(moMay(t, srv), []string{"tt-1"}) {
		if err == nil || errors.Is(err, ErrIdentityUnavailable) {
			t.Errorf("lời gọi %d: err = %v, muốn lỗi thường, không thử lại được", i, err)
		}
	}
}

func TestResidentialUnitLookupsEmptyRequestNeverReachesNetwork(t *testing.T) {
	srv := &fakeResidentialUnitServer{}
	c := moMay(t, srv)
	for _, ids := range [][]string{nil, {}, {"", ""}} {
		for i, err := range callUnitLookups(c, ids) {
			if err != nil {
				t.Errorf("lời gọi %d với %q: %v", i, ids, err)
			}
		}
	}
	if srv.calls != 0 {
		t.Errorf("yêu cầu rỗng đã lên mạng %d lần", srv.calls)
	}
}

func TestResidentialUnitLookupsOverCeilingRefusedLocally(t *testing.T) {
	srv := &fakeResidentialUnitServer{}
	c := moMay(t, srv)
	ids := make([]string, MaxLookupKeysPerCall+1)
	for i := range ids {
		ids[i] = "tt-1"
	}
	if _, err := c.ActiveResidentialUnits(ngucCanh(), ids); err == nil {
		t.Error("ActiveResidentialUnits: vượt trần mà không báo lỗi")
	}
	if _, err := c.ResidentialUnitNames(ngucCanh(), ids); err == nil {
		t.Error("ResidentialUnitNames: vượt trần mà không báo lỗi")
	}
	if srv.calls != 0 {
		t.Error("yêu cầu vượt trần đã được gửi")
	}
}

func TestResidentialUnitLookupsContractFaults(t *testing.T) {
	activeCases := map[string][]*identityv1.ActiveResidentialUnit{
		"id không được hỏi": {{Id: "khong-hoi", Name: "X"}},
		"id rỗng":           {{Id: "", Name: "X"}},
		"tên rỗng":          {{Id: "tt-1"}},
	}
	for name, items := range activeCases {
		if _, err := moMay(t, &fakeResidentialUnitServer{active: items}).
			ActiveResidentialUnits(ngucCanh(), []string{"tt-1"}); err == nil {
			t.Errorf("ActiveResidentialUnits, %s: không báo lỗi hợp đồng", name)
		}
	}
	nameCases := map[string][]*identityv1.ResidentialUnitName{
		"id không được hỏi":    {{Id: "khong-hoi", Name: "X", Standing: live}},
		"tên rỗng":             {{Id: "tt-1", Standing: live}},
		"tên thiếu trạng thái": {{Id: "tt-1", Name: "X"}},
	}
	for name, items := range nameCases {
		if _, err := moMay(t, &fakeResidentialUnitServer{names: items}).
			ResidentialUnitNames(ngucCanh(), []string{"tt-1"}); err == nil {
			t.Errorf("ResidentialUnitNames, %s: không báo lỗi hợp đồng", name)
		}
		if _, err := moMay(t, &fakeResidentialUnitServer{names: items}).
			ResidentialUnitNamesInBatches(ngucCanh(), []string{"tt-1"}); err == nil {
			t.Errorf("ResidentialUnitNamesInBatches, %s: không báo lỗi hợp đồng", name)
		}
	}
}

// manyUnitIDs returns n distinct ids plus duplicates of half of them and a blank — neither of which
// may count toward a batch.
func manyUnitIDs(n int) []string {
	ids := make([]string, 0, n+n/2+1)
	for i := 0; i < n; i++ {
		ids = append(ids, fmt.Sprintf("tt-%03d", i))
	}
	return append(append(ids, ids[:n/2]...), "")
}

func TestResidentialUnitNamesInBatchesSplitsWithinCeilingAndMerges(t *testing.T) {
	srv := &fakeResidentialUnitServer{echoNames: true}
	c := moMay(t, srv)

	const n = 2*MaxLookupKeysPerCall + 50
	got, err := c.ResidentialUnitNamesInBatches(ngucCanh(), manyUnitIDs(n))
	if err != nil {
		t.Fatalf("ResidentialUnitNamesInBatches: %v", err)
	}
	if len(got) != n {
		t.Errorf("số tên = %d, muốn %d", len(got), n)
	}
	if len(srv.sizes) != 3 {
		t.Fatalf("số lời gọi = %d (%v), muốn 3", len(srv.sizes), srv.sizes)
	}
	total := 0
	for _, sz := range srv.sizes {
		if sz > MaxLookupKeysPerCall {
			t.Errorf("một lô gửi %d id, vượt trần %d", sz, MaxLookupKeysPerCall)
		}
		total += sz
	}
	if total != n {
		t.Errorf("tổng id gửi = %d, muốn %d (trùng / rỗng không được gửi)", total, n)
	}
	assertTenantOnWire(t, srv.sawTenant)
}

func TestResidentialUnitNamesInBatchesFailsWholeOnOneFailedCall(t *testing.T) {
	srv := &fakeResidentialUnitServer{echoNames: true, failOn: 2}
	got, err := moMay(t, srv).ResidentialUnitNamesInBatches(ngucCanh(), manyUnitIDs(2*MaxLookupKeysPerCall+1))
	if !errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("err = %v, muốn ErrIdentityUnavailable — một lô hỏng là cả lần tra hỏng", err)
	}
	if got != nil {
		t.Errorf("trả bản đồ một phần (%d mục) kèm lỗi", len(got))
	}
}
