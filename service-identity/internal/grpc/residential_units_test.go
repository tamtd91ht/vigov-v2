package grpc

// What these tests defend: the BOUNDARY around the two residential-unit reads (ADR 0088) — the
// ceiling refused before any read, an empty request answered without one, another commune's unit
// absent, an out-of-use unit absent from the decision read but LIVE in the display read, a removed
// unit absent from the decision read but REMOVED in the display read, a response that is a subset of
// the question, a store failure that is Internal and never empty, and no commune meaning refusal.
// The SQL predicates themselves are defended in internal/store (residential_unit_lookup_pg_test.go).

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// fakeResidentialUnit is one stored row as the fake holds it.
type fakeResidentialUnit struct {
	name    string
	inUse   bool // dang_dung
	removed bool // deleted_at IS NOT NULL
}

// residentialUnitFake models the commune scope and both predicates, so the handler tests read like
// the contract: rows are keyed by COMMUNE then id, and the commune comes from the context exactly as
// store.Scoped binds it.
type residentialUnitFake struct {
	rows  map[tenant.ID]map[string]fakeResidentialUnit
	extra []domain.ResidentialUnitName // returned WITHOUT being asked — must be dropped
	err   error
	calls int
	asked []string
}

func (f *residentialUnitFake) scope(ctx context.Context, ids []string) map[string]fakeResidentialUnit {
	f.calls++
	f.asked = ids
	xa, _ := tenant.From(ctx)
	return f.rows[xa]
}

func (f *residentialUnitFake) ActiveUnitsByID(ctx context.Context, ids []string) ([]domain.ActiveResidentialUnit, error) {
	rows := f.scope(ctx, ids)
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.ActiveResidentialUnit
	for _, id := range ids {
		if r, ok := rows[id]; ok && r.inUse && !r.removed {
			out = append(out, domain.ActiveResidentialUnit{ID: id, Name: r.name})
		}
	}
	for _, e := range f.extra {
		out = append(out, domain.ActiveResidentialUnit{ID: e.ID, Name: e.Name})
	}
	return out, nil
}

func (f *residentialUnitFake) UnitNamesByID(ctx context.Context, ids []string) ([]domain.ResidentialUnitName, error) {
	rows := f.scope(ctx, ids)
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.ResidentialUnitName
	for _, id := range ids {
		if r, ok := rows[id]; ok {
			out = append(out, domain.ResidentialUnitName{ID: id, Name: r.name, Live: !r.removed})
		}
	}
	return append(out, f.extra...), nil
}

// fourUnits: one active, one out of use, one removed in commune A; one in commune B.
func fourUnits() *residentialUnitFake {
	return &residentialUnitFake{rows: map[tenant.ID]map[string]fakeResidentialUnit{
		xaA: {
			"tt-active": {name: "Thôn Bình An", inUse: true},
			"tt-off":    {name: "Thôn Cũ", inUse: false},
			"tt-gone":   {name: "Thôn Đã Xoá", inUse: true, removed: true},
		},
		xaB: {
			"tt-other": {name: "Thôn Xã Khác", inUse: true},
		},
	}}
}

var askAllUnits = []string{"tt-active", "tt-off", "tt-gone", "tt-other", "tt-none", "tt-active", ""}

func withUnits(f *residentialUnitFake) func(*Deps) {
	return func(d *Deps) { d.ResidentialUnits, d.ResidentialUnitNames = f, f }
}

func TestActiveResidentialUnitsOnlyActiveUnitsOfThisCommune(t *testing.T) {
	f := fourUnits()
	s, _ := may(t, withUnits(f))

	ra, err := s.ResolveActiveResidentialUnits(ctxXa(xaA),
		&identityv1.ResolveActiveResidentialUnitsRequest{Ids: askAllUnits})
	if err != nil {
		t.Fatalf("ResolveActiveResidentialUnits: %v", err)
	}
	if len(f.asked) != 5 {
		t.Errorf("kho được hỏi %v, muốn 5 id đã gộp trùng và bỏ rỗng", f.asked)
	}
	items := ra.GetItems()
	if len(items) != 1 || items[0].GetId() != "tt-active" || items[0].GetName() != "Thôn Bình An" {
		t.Fatalf("đơn vị đang dùng = %v, muốn chỉ tt-active — ngưng dùng / đã xoá / XÃ KHÁC / không có phải vắng", items)
	}
}

func TestResidentialUnitNamesOutOfUseLiveRemovedFlaggedOtherCommuneAbsent(t *testing.T) {
	f := fourUnits()
	f.extra = []domain.ResidentialUnitName{{ID: "tt-not-asked", Name: "X", Live: true}}
	s, _ := may(t, withUnits(f))

	ra, err := s.ResolveResidentialUnitNames(ctxXa(xaA),
		&identityv1.ResolveResidentialUnitNamesRequest{Ids: askAllUnits})
	if err != nil {
		t.Fatalf("ResolveResidentialUnitNames: %v", err)
	}
	got := map[string]*identityv1.ResidentialUnitName{}
	for _, it := range ra.GetItems() {
		if _, dup := got[it.GetId()]; dup {
			t.Errorf("%s trả hai lần", it.GetId())
		}
		got[it.GetId()] = it
	}
	if g := got["tt-active"]; g.GetStanding() != identityv1.RecordStanding_RECORD_STANDING_LIVE || g.GetName() != "Thôn Bình An" {
		t.Errorf("tt-active = %v", g)
	}
	if g := got["tt-off"]; g == nil || g.GetStanding() != identityv1.RecordStanding_RECORD_STANDING_LIVE {
		t.Errorf("thôn ngưng dùng = %v, muốn LIVE — ngưng dùng lọc ở ô chọn, không lọc ở nhãn (ADR 0059 §2)", g)
	}
	if g := got["tt-gone"]; g == nil || g.GetStanding() != identityv1.RecordStanding_RECORD_STANDING_REMOVED || g.GetName() == "" {
		t.Errorf("thôn đã xoá mềm = %v, muốn CÓ TÊN và REMOVED", g)
	}
	if _, ok := got["tt-other"]; ok {
		t.Error("THÔN CỦA XÃ KHÁC có trong câu trả lời — rò giữa hai xã (luật 1)")
	}
	if _, ok := got["tt-none"]; ok {
		t.Error("id không có lại có tên")
	}
	if _, ok := got["tt-not-asked"]; ok {
		t.Error("PHẢN HỒI MANG MỘT ID KHÔNG ĐƯỢC HỎI — tra cứu đã thành liệt kê")
	}
	if len(got) != 3 {
		t.Errorf("số mục = %d, muốn 3", len(got))
	}
}

func TestActiveResidentialUnitsDropsUnaskedAndNameless(t *testing.T) {
	f := &residentialUnitFake{
		rows:  map[tenant.ID]map[string]fakeResidentialUnit{xaA: {"tt-blank": {name: "", inUse: true}}},
		extra: []domain.ResidentialUnitName{{ID: "tt-not-asked", Name: "X", Live: true}},
	}
	s, _ := may(t, withUnits(f))

	ra, err := s.ResolveActiveResidentialUnits(ctxXa(xaA),
		&identityv1.ResolveActiveResidentialUnitsRequest{Ids: []string{"tt-blank"}})
	if err != nil {
		t.Fatalf("ResolveActiveResidentialUnits: %v", err)
	}
	if len(ra.GetItems()) != 0 {
		t.Fatalf("mục = %v, muốn rỗng — id không được hỏi và mục không tên đều phải vắng", ra.GetItems())
	}
}

func TestResidentialUnitReadsOverCeilingRefusedBeforeRead(t *testing.T) {
	f := fourUnits()
	s, _ := may(t, withUnits(f))

	over := make([]string, TranIDMotLo+1)
	for i := range over {
		over[i] = "tt-active" // all duplicates: the ceiling counts what was SENT
	}
	_, err1 := s.ResolveActiveResidentialUnits(ctxXa(xaA), &identityv1.ResolveActiveResidentialUnitsRequest{Ids: over})
	_, err2 := s.ResolveResidentialUnitNames(ctxXa(xaA), &identityv1.ResolveResidentialUnitNamesRequest{Ids: over})
	for i, err := range []error{err1, err2} {
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("RPC %d: mã = %v, muốn InvalidArgument — không bao giờ cắt bớt âm thầm", i, status.Code(err))
		}
	}
	if f.calls != 0 {
		t.Error("vượt trần mà vẫn đọc kho")
	}

	at := over[:TranIDMotLo]
	if _, err := s.ResolveActiveResidentialUnits(ctxXa(xaA), &identityv1.ResolveActiveResidentialUnitsRequest{Ids: at}); err != nil {
		t.Errorf("đúng trần bị từ chối: %v", err)
	}
	if _, err := s.ResolveResidentialUnitNames(ctxXa(xaA), &identityv1.ResolveResidentialUnitNamesRequest{Ids: at}); err != nil {
		t.Errorf("đúng trần bị từ chối: %v", err)
	}
}

func TestResidentialUnitReadsEmptyAnswersEmptyWithoutRead(t *testing.T) {
	f := fourUnits()
	s, _ := may(t, withUnits(f))

	for _, ids := range [][]string{nil, {}, {""}, {"", ""}} {
		r1, err := s.ResolveActiveResidentialUnits(ctxXa(xaA), &identityv1.ResolveActiveResidentialUnitsRequest{Ids: ids})
		if err != nil || len(r1.GetItems()) != 0 {
			t.Errorf("ResolveActiveResidentialUnits(%q) = (%v, %v), muốn rỗng", ids, r1, err)
		}
		r2, err := s.ResolveResidentialUnitNames(ctxXa(xaA), &identityv1.ResolveResidentialUnitNamesRequest{Ids: ids})
		if err != nil || len(r2.GetItems()) != 0 {
			t.Errorf("ResolveResidentialUnitNames(%q) = (%v, %v), muốn rỗng", ids, r2, err)
		}
	}
	if f.calls != 0 {
		t.Error("yêu cầu rỗng mà vẫn đọc kho — rỗng không bao giờ là \"tất cả\"")
	}
}

func TestResidentialUnitReadsStoreFailureIsInternalNotEmpty(t *testing.T) {
	s, _ := may(t, withUnits(&residentialUnitFake{err: errors.New("mất kết nối")}))

	r1, err1 := s.ResolveActiveResidentialUnits(ctxXa(xaA), &identityv1.ResolveActiveResidentialUnitsRequest{Ids: []string{"tt-1"}})
	r2, err2 := s.ResolveResidentialUnitNames(ctxXa(xaA), &identityv1.ResolveResidentialUnitNamesRequest{Ids: []string{"tt-1"}})
	for i, err := range []error{err1, err2} {
		if status.Code(err) != codes.Internal {
			t.Errorf("RPC %d: mã = %v, muốn Internal — sự cố không bao giờ là \"vắng\"", i, status.Code(err))
		}
	}
	if r1 != nil || r2 != nil {
		t.Error("trả phản hồi kèm lỗi")
	}
}

func TestResidentialUnitReadsWithoutCommuneRefused(t *testing.T) {
	f := fourUnits()
	s, _ := may(t, withUnits(f))

	_, err1 := s.ResolveActiveResidentialUnits(context.Background(), &identityv1.ResolveActiveResidentialUnitsRequest{Ids: []string{"tt-active"}})
	_, err2 := s.ResolveResidentialUnitNames(context.Background(), &identityv1.ResolveResidentialUnitNamesRequest{Ids: []string{"tt-active"}})
	for i, err := range []error{err1, err2} {
		if status.Code(err) != codes.Internal {
			t.Errorf("RPC %d: mã = %v, muốn Internal (thiếu xã trong context)", i, status.Code(err))
		}
	}
	if f.calls != 0 {
		t.Error("không có xã mà vẫn đọc kho")
	}
}
