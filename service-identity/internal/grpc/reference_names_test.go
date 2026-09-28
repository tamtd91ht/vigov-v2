package grpc

// What these tests defend: the BOUNDARY around the three register reads — ceilings refused before
// any read, an empty request answered without one, removed rows ANSWERED (flagged) by the two display
// reads, a response that is a subset of the question, a store failure that is Internal and never an
// empty answer, and no commune meaning refusal. The predicates themselves are defended in
// internal/store (reference_names_pg_test.go).

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

type orgUnitNameFake struct {
	rows  map[string]domain.OrgUnitName
	extra []domain.OrgUnitName // rows returned WITHOUT being asked — must be dropped
	err   error
	calls int
	asked []string
}

func (f *orgUnitNameFake) NamesByID(_ context.Context, ids []string) ([]domain.OrgUnitName, error) {
	f.calls++
	f.asked = ids
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.OrgUnitName
	for _, id := range ids {
		if r, ok := f.rows[id]; ok {
			out = append(out, r)
		}
	}
	return append(out, f.extra...), nil
}

type taskBlocLabelFake struct {
	rows  map[string]domain.TaskBlocLabel
	extra []domain.TaskBlocLabel
	err   error
	calls int
	asked []string
}

func (f *taskBlocLabelFake) LabelsByCode(_ context.Context, codes []string) ([]domain.TaskBlocLabel, error) {
	f.calls++
	f.asked = codes
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.TaskBlocLabel
	for _, c := range codes {
		if r, ok := f.rows[c]; ok {
			out = append(out, r)
		}
	}
	return append(out, f.extra...), nil
}

func overCeiling() []string {
	keys := make([]string, TranIDMotLo+1)
	for i := range keys {
		keys[i] = "k" // all duplicates: the ceiling counts what was SENT
	}
	return keys
}

func TestRegisterReadsOverCeilingRefusedBeforeRead(t *testing.T) {
	names, labels, units := &orgUnitNameFake{}, &taskBlocLabelFake{}, &orgUnitFake{}
	s, _ := may(t, func(d *Deps) { d.OrgUnitNames, d.TaskBlocLabels, d.OrgUnits = names, labels, units })

	_, err1 := s.ResolveOrgUnitNames(ctxXa(xaA), &identityv1.ResolveOrgUnitNamesRequest{Ids: overCeiling()})
	_, err2 := s.ResolveTaskBlocLabels(ctxXa(xaA), &identityv1.ResolveTaskBlocLabelsRequest{Ma: overCeiling()})
	_, err3 := s.ResolveLiveOrgUnitCodes(ctxXa(xaA), &identityv1.ResolveLiveOrgUnitCodesRequest{Ma: overCeiling()})
	for i, err := range []error{err1, err2, err3} {
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("RPC %d: mã = %v, muốn InvalidArgument — không bao giờ cắt bớt âm thầm", i, status.Code(err))
		}
	}
	if names.calls+labels.calls+units.calls != 0 {
		t.Error("vượt trần mà vẫn đọc kho")
	}

	// Exactly at the ceiling is accepted.
	at := overCeiling()[:TranIDMotLo]
	if _, err := s.ResolveOrgUnitNames(ctxXa(xaA), &identityv1.ResolveOrgUnitNamesRequest{Ids: at}); err != nil {
		t.Errorf("đúng trần bị từ chối: %v", err)
	}
}

func TestRegisterReadsEmptyAnswersEmptyWithoutRead(t *testing.T) {
	names, labels, units := &orgUnitNameFake{}, &taskBlocLabelFake{}, &orgUnitFake{}
	s, _ := may(t, func(d *Deps) { d.OrgUnitNames, d.TaskBlocLabels, d.OrgUnits = names, labels, units })

	for _, keys := range [][]string{nil, {}, {""}, {"", ""}} {
		r1, err := s.ResolveOrgUnitNames(ctxXa(xaA), &identityv1.ResolveOrgUnitNamesRequest{Ids: keys})
		if err != nil || len(r1.GetItems()) != 0 {
			t.Errorf("ResolveOrgUnitNames(%q) = (%v, %v), muốn rỗng", keys, r1, err)
		}
		r2, err := s.ResolveTaskBlocLabels(ctxXa(xaA), &identityv1.ResolveTaskBlocLabelsRequest{Ma: keys})
		if err != nil || len(r2.GetItems()) != 0 {
			t.Errorf("ResolveTaskBlocLabels(%q) = (%v, %v), muốn rỗng", keys, r2, err)
		}
		r3, err := s.ResolveLiveOrgUnitCodes(ctxXa(xaA), &identityv1.ResolveLiveOrgUnitCodesRequest{Ma: keys})
		if err != nil || len(r3.GetItems()) != 0 {
			t.Errorf("ResolveLiveOrgUnitCodes(%q) = (%v, %v), muốn rỗng", keys, r3, err)
		}
	}
	if names.calls+labels.calls+units.calls != 0 {
		t.Error("yêu cầu rỗng mà vẫn đọc kho — rỗng không bao giờ là \"tất cả\"")
	}
}

func TestOrgUnitNamesAnswersRemovedFlaggedAndOnlyWhatWasAsked(t *testing.T) {
	fake := &orgUnitNameFake{
		rows: map[string]domain.OrgUnitName{
			"bp-live": {ID: "bp-live", Name: "VĂN PHÒNG", Live: true},
			"bp-gone": {ID: "bp-gone", Name: "BỘ PHẬN CŨ", Live: false},
		},
		extra: []domain.OrgUnitName{{ID: "bp-not-asked", Name: "X", Live: true}},
	}
	s, _ := may(t, func(d *Deps) { d.OrgUnitNames = fake })

	ra, err := s.ResolveOrgUnitNames(ctxXa(xaA), &identityv1.ResolveOrgUnitNamesRequest{
		Ids: []string{"bp-live", "bp-gone", "bp-live", "bp-none"},
	})
	if err != nil {
		t.Fatalf("ResolveOrgUnitNames: %v", err)
	}
	if len(fake.asked) != 3 {
		t.Errorf("kho được hỏi %v, muốn 3 id đã gộp trùng", fake.asked)
	}
	got := map[string]*identityv1.OrgUnitName{}
	for _, it := range ra.GetItems() {
		if _, dup := got[it.GetId()]; dup {
			t.Errorf("%s trả hai lần", it.GetId())
		}
		got[it.GetId()] = it
	}
	if got["bp-live"].GetStanding() != identityv1.RecordStanding_RECORD_STANDING_LIVE {
		t.Errorf("bp-live standing = %v", got["bp-live"].GetStanding())
	}
	if g := got["bp-gone"]; g == nil || g.GetStanding() != identityv1.RecordStanding_RECORD_STANDING_REMOVED || g.GetName() == "" {
		t.Errorf("bộ phận đã gỡ = %v, muốn CÓ TÊN và đánh dấu REMOVED — sổ in phải ghi đúng điều hồ sơ giữ", g)
	}
	if _, ok := got["bp-not-asked"]; ok {
		t.Error("PHẢN HỒI MANG MỘT ID KHÔNG ĐƯỢC HỎI — tra cứu đã thành liệt kê")
	}
	if len(got) != 2 {
		t.Errorf("số mục = %d, muốn 2", len(got))
	}
}

func TestTaskBlocLabelsAnswersRemovedFlaggedAndOnlyWhatWasAsked(t *testing.T) {
	fake := &taskBlocLabelFake{
		rows: map[string]domain.TaskBlocLabel{
			"khoi-uy-ban": {Ma: "khoi-uy-ban", Label: "Khối Uỷ ban", Live: true},
			"khoi-cu":     {Ma: "khoi-cu", Label: "Khối cũ", Live: false},
		},
		extra: []domain.TaskBlocLabel{{Ma: "khoi-dang", Label: "Khối Đảng", Live: true}},
	}
	s, _ := may(t, func(d *Deps) { d.TaskBlocLabels = fake })

	ra, err := s.ResolveTaskBlocLabels(ctxXa(xaA), &identityv1.ResolveTaskBlocLabelsRequest{
		Ma: []string{"khoi-uy-ban", "khoi-cu", "khong-co"},
	})
	if err != nil {
		t.Fatalf("ResolveTaskBlocLabels: %v", err)
	}
	got := map[string]*identityv1.TaskBlocLabel{}
	for _, it := range ra.GetItems() {
		got[it.GetMa()] = it
	}
	if got["khoi-uy-ban"].GetLabel() != "Khối Uỷ ban" ||
		got["khoi-uy-ban"].GetStanding() != identityv1.RecordStanding_RECORD_STANDING_LIVE {
		t.Errorf("khoi-uy-ban = %v", got["khoi-uy-ban"])
	}
	if got["khoi-cu"].GetStanding() != identityv1.RecordStanding_RECORD_STANDING_REMOVED {
		t.Errorf("khối đã xoá = %v, muốn REMOVED", got["khoi-cu"])
	}
	if _, ok := got["khoi-dang"]; ok {
		t.Error("PHẢN HỒI MANG MỘT MÃ KHÔNG ĐƯỢC HỎI")
	}
	if len(got) != 2 {
		t.Errorf("số mục = %d, muốn 2", len(got))
	}
}

func TestLiveOrgUnitCodesAnswersOnlyAskedLiveMatches(t *testing.T) {
	fake := &orgUnitFake{
		byCode: map[string]string{"van-phong": "bp-1"},
		extraCodes: []domain.OrgUnitCodeMatch{
			{Ma: "khong-hoi", ID: "bp-9"}, // not asked
			{Ma: "rong-id", ID: ""},       // asked below, but an empty id is never an answer
		},
	}
	s, _ := may(t, func(d *Deps) { d.OrgUnits = fake })

	ra, err := s.ResolveLiveOrgUnitCodes(ctxXa(xaA), &identityv1.ResolveLiveOrgUnitCodesRequest{
		Ma: []string{"van-phong", "van-phong", "rong-id", "da-xoa"},
	})
	if err != nil {
		t.Fatalf("ResolveLiveOrgUnitCodes: %v", err)
	}
	items := ra.GetItems()
	if len(items) != 1 || items[0].GetMa() != "van-phong" || items[0].GetId() != "bp-1" {
		t.Fatalf("khớp = %v, muốn chỉ van-phong→bp-1", items)
	}
}

func TestRegisterReadsStoreFailureIsInternalNotEmpty(t *testing.T) {
	boom := errors.New("mất kết nối")
	s, _ := may(t, func(d *Deps) {
		d.OrgUnitNames = &orgUnitNameFake{err: boom}
		d.TaskBlocLabels = &taskBlocLabelFake{err: boom}
		d.OrgUnits = &orgUnitFake{err: boom}
	})
	r1, err1 := s.ResolveOrgUnitNames(ctxXa(xaA), &identityv1.ResolveOrgUnitNamesRequest{Ids: []string{"bp-1"}})
	r2, err2 := s.ResolveTaskBlocLabels(ctxXa(xaA), &identityv1.ResolveTaskBlocLabelsRequest{Ma: []string{"k"}})
	r3, err3 := s.ResolveLiveOrgUnitCodes(ctxXa(xaA), &identityv1.ResolveLiveOrgUnitCodesRequest{Ma: []string{"k"}})
	for i, err := range []error{err1, err2, err3} {
		if status.Code(err) != codes.Internal {
			t.Errorf("RPC %d: mã = %v, muốn Internal — sự cố không bao giờ là \"không có tên\"", i, status.Code(err))
		}
	}
	if r1 != nil || r2 != nil || r3 != nil {
		t.Error("trả phản hồi kèm lỗi")
	}
}

func TestRegisterReadsWithoutCommuneRefused(t *testing.T) {
	names, labels, units := &orgUnitNameFake{}, &taskBlocLabelFake{}, &orgUnitFake{}
	s, _ := may(t, func(d *Deps) { d.OrgUnitNames, d.TaskBlocLabels, d.OrgUnits = names, labels, units })

	_, err1 := s.ResolveOrgUnitNames(context.Background(), &identityv1.ResolveOrgUnitNamesRequest{Ids: []string{"bp-1"}})
	_, err2 := s.ResolveTaskBlocLabels(context.Background(), &identityv1.ResolveTaskBlocLabelsRequest{Ma: []string{"k"}})
	_, err3 := s.ResolveLiveOrgUnitCodes(context.Background(), &identityv1.ResolveLiveOrgUnitCodesRequest{Ma: []string{"k"}})
	for i, err := range []error{err1, err2, err3} {
		if status.Code(err) != codes.Internal {
			t.Errorf("RPC %d: mã = %v, muốn Internal (thiếu xã trong context)", i, status.Code(err))
		}
	}
	if names.calls+labels.calls+units.calls != 0 {
		t.Error("không có xã mà vẫn đọc kho")
	}
}
