package grpc

// What these tests defend: the BOUNDARY around the two org-unit reads — ceilings refused before any
// read, "absent" as the one answer for every refusal, a response that is a subset of the question,
// an outage that is never an answer. The predicate itself is defended in internal/store.

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

type orgUnitFake struct {
	live      map[string]bool
	byStaff   map[string][]string
	extra     []string // ids the store returns WITHOUT being asked — a contract fault to be dropped
	err       error
	calls     int
	askedIDs  []string
	askedCode string

	byCode     map[string]string         // live unit code → id, for LiveIDsByCode
	extraCodes []domain.OrgUnitCodeMatch // matches returned WITHOUT being asked — a contract fault
}

func (f *orgUnitFake) LiveIDs(_ context.Context, ids []string) ([]string, error) {
	f.calls++
	f.askedIDs = ids
	if f.err != nil {
		return nil, f.err
	}
	var out []string
	for _, id := range ids {
		if f.live[id] {
			out = append(out, id)
		}
	}
	return append(out, f.extra...), nil
}

func (f *orgUnitFake) LiveIDsByCode(_ context.Context, codes []string) ([]domain.OrgUnitCodeMatch, error) {
	f.calls++
	f.askedIDs = codes
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.OrgUnitCodeMatch
	for _, c := range codes {
		if id, ok := f.byCode[c]; ok {
			out = append(out, domain.OrgUnitCodeMatch{Ma: c, ID: id})
		}
	}
	return append(out, f.extraCodes...), nil
}

func (f *orgUnitFake) UnitsOfStaff(_ context.Context, ma string) ([]string, error) {
	f.calls++
	f.askedCode = ma
	if f.err != nil {
		return nil, f.err
	}
	return f.byStaff[ma], nil
}

func TestLiveOrgUnitsOverCeilingRefusedBeforeRead(t *testing.T) {
	fake := &orgUnitFake{}
	s, _ := may(t, func(d *Deps) { d.OrgUnits = fake })

	ids := make([]string, MaxOrgUnitsPerCall+1)
	for i := range ids {
		ids[i] = "bp-1" // all duplicates: the ceiling counts what was SENT
	}
	_, err := s.ResolveLiveOrgUnits(ctxXa(xaA), &identityv1.ResolveLiveOrgUnitsRequest{Ids: ids})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("mã = %v, muốn InvalidArgument", status.Code(err))
	}
	if fake.calls != 0 {
		t.Errorf("đã đọc kho %d lần cho một lời gọi vượt trần", fake.calls)
	}
}

// Empty is NOT "every live unit": it answers empty and reads nothing.
func TestLiveOrgUnitsEmptyAnswersEmptyWithoutRead(t *testing.T) {
	fake := &orgUnitFake{live: map[string]bool{"bp-1": true}}
	s, _ := may(t, func(d *Deps) { d.OrgUnits = fake })

	for _, ids := range [][]string{nil, {""}, {"", ""}} {
		ra, err := s.ResolveLiveOrgUnits(ctxXa(xaA), &identityv1.ResolveLiveOrgUnitsRequest{Ids: ids})
		if err != nil {
			t.Fatalf("%q: %v", ids, err)
		}
		if len(ra.GetLiveIds()) != 0 {
			t.Fatalf("%q: trả %v — yêu cầu rỗng không được thành danh sách mọi bộ phận", ids, ra.GetLiveIds())
		}
	}
	if fake.calls != 0 {
		t.Errorf("đã đọc kho %d lần cho yêu cầu rỗng", fake.calls)
	}
}

// A SUBSET OF THE QUESTION, EACH ONCE — and an id the store hands back unasked is dropped, because
// the caller decides from this set.
func TestLiveOrgUnitsAnswerIsDedupedSubset(t *testing.T) {
	fake := &orgUnitFake{live: map[string]bool{"bp-1": true, "bp-2": true}, extra: []string{"bp-leak", "bp-1"}}
	s, _ := may(t, func(d *Deps) { d.OrgUnits = fake })

	ra, err := s.ResolveLiveOrgUnits(ctxXa(xaA), &identityv1.ResolveLiveOrgUnitsRequest{
		Ids: []string{"bp-1", "bp-1", "bp-gone", "bp-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := append([]string(nil), ra.GetLiveIds()...)
	sort.Strings(got)
	if strings.Join(got, ",") != "bp-1,bp-2" {
		t.Fatalf("live_ids = %v, muốn bp-1,bp-2 (không lặp, không mã chưa hỏi)", got)
	}
	if len(fake.askedIDs) != 3 {
		t.Errorf("kho được hỏi %v — trùng phải được gộp trước khi đọc", fake.askedIDs)
	}
}

// An outage is NOT "not live" — the caller would refuse a real unit — and NOT "live" either.
func TestLiveOrgUnitsStoreFailureIsInternal(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.OrgUnits = &orgUnitFake{err: errors.New("mất kết nối")} })
	ra, err := s.ResolveLiveOrgUnits(ctxXa(xaA), &identityv1.ResolveLiveOrgUnitsRequest{Ids: []string{"bp-1"}})
	if status.Code(err) != codes.Internal || ra != nil {
		t.Fatalf("mã = %v, phản hồi = %v; muốn Internal và không phản hồi", status.Code(err), ra)
	}
	if strings.Contains(err.Error(), "mất kết nối") {
		t.Error("nguyên nhân nội bộ lọt sang bên gọi")
	}
}

func TestOrgUnitRPCsWithoutCommuneAreInternalNotPanic(t *testing.T) {
	fake := &orgUnitFake{}
	s, _ := may(t, func(d *Deps) { d.OrgUnits = fake })
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panic khi thiếu xã: %v", r)
		}
	}()
	_, err := s.ResolveLiveOrgUnits(context.Background(), &identityv1.ResolveLiveOrgUnitsRequest{Ids: []string{"bp-1"}})
	if status.Code(err) != codes.Internal {
		t.Errorf("ResolveLiveOrgUnits: mã = %v, muốn Internal", status.Code(err))
	}
	_, err = s.ResolveStaffOrgUnits(context.Background(), &identityv1.ResolveStaffOrgUnitsRequest{Ma: "CB-001"})
	if status.Code(err) != codes.Internal {
		t.Errorf("ResolveStaffOrgUnits: mã = %v, muốn Internal", status.Code(err))
	}
	if fake.calls != 0 {
		t.Errorf("đã đọc kho %d lần khi không có xã", fake.calls)
	}
}

// An empty code is a wiring fault in the caller — its principal always carries one.
func TestStaffOrgUnitsEmptyCodeIsInvalidArgument(t *testing.T) {
	fake := &orgUnitFake{}
	s, _ := may(t, func(d *Deps) { d.OrgUnits = fake })
	for _, ma := range []string{"", "   "} {
		_, err := s.ResolveStaffOrgUnits(ctxXa(xaA), &identityv1.ResolveStaffOrgUnitsRequest{Ma: ma})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("ma %q: mã = %v, muốn InvalidArgument", ma, status.Code(err))
		}
	}
	if fake.calls != 0 {
		t.Errorf("đã đọc kho %d lần cho mã rỗng", fake.calls)
	}
}

// Known code → its unit; unknown code → empty, never NOT_FOUND; blanks from the store dropped.
func TestStaffOrgUnitsAnswers(t *testing.T) {
	fake := &orgUnitFake{byStaff: map[string][]string{"CB-001": {"bp-1", ""}}}
	s, _ := may(t, func(d *Deps) { d.OrgUnits = fake })

	ra, err := s.ResolveStaffOrgUnits(ctxXa(xaA), &identityv1.ResolveStaffOrgUnitsRequest{Ma: " CB-001 "})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ra.GetOrgUnitIds(), ",") != "bp-1" {
		t.Fatalf("org_unit_ids = %v, muốn bp-1", ra.GetOrgUnitIds())
	}
	if fake.askedCode != "CB-001" {
		t.Errorf("kho được hỏi mã %q, muốn CB-001 đã cắt khoảng trắng", fake.askedCode)
	}

	ra, err = s.ResolveStaffOrgUnits(ctxXa(xaA), &identityv1.ResolveStaffOrgUnitsRequest{Ma: "CB-khong-co"})
	if err != nil || len(ra.GetOrgUnitIds()) != 0 {
		t.Fatalf("mã không tồn tại: lỗi %v, đơn vị %v — muốn OK rỗng", err, ra.GetOrgUnitIds())
	}
}

// An outage is NOT "no unit": the tab would hide exactly the tasks it is named for.
func TestStaffOrgUnitsStoreFailureIsInternal(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.OrgUnits = &orgUnitFake{err: errors.New("mất kết nối")} })
	ra, err := s.ResolveStaffOrgUnits(ctxXa(xaA), &identityv1.ResolveStaffOrgUnitsRequest{Ma: "CB-001"})
	if status.Code(err) != codes.Internal || ra != nil {
		t.Fatalf("mã = %v, phản hồi = %v; muốn Internal và không phản hồi", status.Code(err), ra)
	}
}
