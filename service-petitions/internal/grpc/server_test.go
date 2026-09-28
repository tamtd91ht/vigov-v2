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

	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/tenant"
)

const testTenant = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")

// fakeCounter records what it was asked and in which commune, and answers n or err.
type fakeCounter struct {
	n         int
	err       error
	calls     int
	gotID     string
	gotTenant tenant.ID
	hadTenant bool
}

func (f *fakeCounter) CountOpenHeldByOrgUnit(ctx context.Context, id string) (int, error) {
	f.calls++
	f.gotID = id
	f.gotTenant, f.hadTenant = tenant.From(ctx)
	return f.n, f.err
}

func newTestServer(p, t *fakeCounter) *Server {
	return NewServer(Deps{Petitions: p, Tasks: t, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func TestCountOrgUnitHoldingsAnswersBothCountsInContextCommune(t *testing.T) {
	p, tk := &fakeCounter{n: 3}, &fakeCounter{n: 2}
	res, err := newTestServer(p, tk).CountOrgUnitHoldings(tenant.Into(context.Background(), testTenant),
		&petitionsv1.CountOrgUnitHoldingsRequest{OrgUnitId: " bp-1 "})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetOpenPetitions() != 3 || res.GetOpenTasks() != 2 {
		t.Errorf("= %+v, muốn 3 phiếu, 2 nhiệm vụ", res)
	}
	for name, f := range map[string]*fakeCounter{"petitions": p, "tasks": tk} {
		if f.gotID != "bp-1" || !f.hadTenant || f.gotTenant != testTenant {
			t.Errorf("%s hỏi id=%q xã=%q — muốn id đã cắt khoảng trắng và xã của context", name, f.gotID, f.gotTenant)
		}
	}
}

func TestCountOrgUnitHoldingsBlankIDIsInvalidArgument(t *testing.T) {
	for _, id := range []string{"", "   "} {
		p, tk := &fakeCounter{}, &fakeCounter{}
		_, err := newTestServer(p, tk).CountOrgUnitHoldings(tenant.Into(context.Background(), testTenant),
			&petitionsv1.CountOrgUnitHoldingsRequest{OrgUnitId: id})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("id %q: mã = %v, muốn InvalidArgument", id, status.Code(err))
		}
		if p.calls+tk.calls != 0 {
			t.Errorf("id %q: vẫn đọc kho", id)
		}
	}
}

// No commune in the context — the interceptor is missing from the chain. Refused, never counted
// against a default commune; the stores are never reached.
func TestCountOrgUnitHoldingsWithoutCommuneIsRefused(t *testing.T) {
	p, tk := &fakeCounter{n: 1}, &fakeCounter{n: 1}
	_, err := newTestServer(p, tk).CountOrgUnitHoldings(context.Background(),
		&petitionsv1.CountOrgUnitHoldingsRequest{OrgUnitId: "bp-1"})
	if err == nil || status.Code(err) == codes.OK {
		t.Fatal("đếm được khi không có xã")
	}
	if p.calls+tk.calls != 0 {
		t.Error("không có xã mà vẫn đọc kho")
	}
}

// A store failure is INTERNAL with no cause on the wire — never a zero count.
func TestCountOrgUnitHoldingsStoreErrorIsInternalNotZero(t *testing.T) {
	cause := errors.New("pq: SELECT … mật khẩu=bí-mật")
	for name, pair := range map[string][2]*fakeCounter{
		"petitions": {{err: cause}, {n: 0}},
		"tasks":     {{n: 0}, {err: cause}},
	} {
		res, err := newTestServer(pair[0], pair[1]).CountOrgUnitHoldings(
			tenant.Into(context.Background(), testTenant), &petitionsv1.CountOrgUnitHoldingsRequest{OrgUnitId: "bp-1"})
		if status.Code(err) != codes.Internal || res != nil {
			t.Errorf("%s: res=%v mã=%v, muốn Internal và không có câu trả lời", name, res, status.Code(err))
		}
		if st, _ := status.FromError(err); st != nil && st.Message() != "lỗi nội bộ, vui lòng thử lại" {
			t.Errorf("%s: nguyên nhân lọt ra ngoài: %q", name, st.Message())
		}
	}
}

func TestClampUint32NeverWraps(t *testing.T) {
	if clampUint32(-1) != 0 || clampUint32(7) != 7 {
		t.Error("clamp sai ở giá trị thường")
	}
	if clampUint32(math.MaxUint32+1) != math.MaxUint32 {
		t.Error("tràn số quay vòng — một số lớn đọc thành nhỏ sẽ cho phép xoá")
	}
}

func TestNewServerRefusesMissingDeps(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dựng được máy chủ thiếu phụ thuộc")
		}
	}()
	NewServer(Deps{Petitions: &fakeCounter{}})
}
