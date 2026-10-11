package app

// "Loại đơn thư" use cases over the REAL core/audit on the fake driver (driver_gia_van_ban_test.go), rows
// in memory. WHAT IS PROVEN: each write and its audit entry commit in ONE transaction, the actor is the
// staff business code; a non-statutory code is refused before any transaction; a taken code writes
// nothing; a blank delete reason stores the fixed sentence; a no-op edit writes and audits nothing.

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

type letterTypeRepoFake struct {
	rows       map[string]domain.CitizenLetterTypeEntry
	deleted    map[string]string // id -> reason
	writes     int
	codesTaken map[domain.LetterType]bool
}

func newLetterTypeRepo(rows ...domain.CitizenLetterTypeEntry) *letterTypeRepoFake {
	r := &letterTypeRepoFake{rows: map[string]domain.CitizenLetterTypeEntry{}, deleted: map[string]string{},
		codesTaken: map[domain.LetterType]bool{}}
	for _, e := range rows {
		r.rows[e.ID] = e
		r.codesTaken[e.Code] = true
	}
	return r
}

func (r *letterTypeRepoFake) ByIDForUpdate(_ context.Context, _ *store.ScopedTx, id string) (domain.CitizenLetterTypeEntry, error) {
	e, ok := r.rows[id]
	if !ok {
		return e, docstore.ErrDanhMucKhongTonTai
	}
	return e, nil
}
func (r *letterTypeRepoFake) CodeTaken(_ context.Context, _ *store.ScopedTx, c domain.LetterType) (bool, error) {
	return r.codesTaken[c], nil
}
func (r *letterTypeRepoFake) AddEntry(_ context.Context, _ *store.ScopedTx, e domain.CitizenLetterTypeEntry) error {
	r.writes++
	r.rows[e.ID] = e
	return nil
}
func (r *letterTypeRepoFake) RewriteEntry(_ context.Context, _ *store.ScopedTx, e domain.CitizenLetterTypeEntry) error {
	r.writes++
	r.rows[e.ID] = e
	return nil
}
func (r *letterTypeRepoFake) SoftDelete(_ context.Context, _ *store.ScopedTx, id, _, reason string) error {
	r.writes++
	r.deleted[id] = reason
	return nil
}

func buildLetterTypes(t *testing.T, k *khoVBGia, repo *letterTypeRepoFake) (*CitizenLetterTypes, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	uc := NewCitizenLetterTypes(store.New(db), repo)
	uc.newID = func() (string, error) { return "01JLETTERTYPE0000000000001", nil }
	return uc, tenant.Into(context.Background(), xaA)
}

func TestAddLetterTypeAuditsInTheSameTransaction(t *testing.T) {
	k := khoVBMau()
	repo := newLetterTypeRepo()
	uc, ctx := buildLetterTypes(t, k, repo)
	e, err := uc.AddType(ctx, AddLetterTypeRequest{Code: "khieu-nai", Label: " Khiếu nại ", Order: 1}, clerk.Actor)
	if err != nil {
		t.Fatalf("AddType: %v", err)
	}
	if e.Code != domain.LetterTypeComplaint || e.Label != "Khiếu nại" || !e.Active {
		t.Fatalf("dòng: %+v", e)
	}
	a := auditEntries(k)
	if len(a) != 1 || a[0].args[1] != "CB-00123" || a[0].args[4] != ActionCreateCitizenLetterType || a[0].args[5] != "khieu-nai" {
		t.Fatalf("vết: %d", len(a))
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("giao dịch: mở %d, commit %d", k.batDau, k.daCommit)
	}
}

func TestAddLetterTypeRefusesAFifthTypeBeforeAnyTransaction(t *testing.T) {
	k := khoVBMau()
	repo := newLetterTypeRepo()
	uc, ctx := buildLetterTypes(t, k, repo)
	if _, err := uc.AddType(ctx, AddLetterTypeRequest{Code: "phan-anh", Label: "Phản ánh"}, clerk.Actor); !errors.Is(err, domain.ErrLetterTypeCodeNotStatutory) {
		t.Fatalf("mã ngoài bốn loại mà lỗi = %v", err)
	}
	if k.batDau != 0 || repo.writes != 0 {
		t.Fatal("mã sai mà đã mở giao dịch hoặc ghi")
	}
}

func TestAddLetterTypeTakenCodeWritesNothing(t *testing.T) {
	k := khoVBMau()
	repo := newLetterTypeRepo(domain.CitizenLetterTypeEntry{ID: "ldt-1", Code: domain.LetterTypeComplaint, Label: "Khiếu nại"})
	uc, ctx := buildLetterTypes(t, k, repo)
	if _, err := uc.AddType(ctx, AddLetterTypeRequest{Code: "khieu-nai", Label: "KN"}, clerk.Actor); !errors.Is(err, docstore.ErrMaDaTonTai) {
		t.Fatalf("mã đã có mà lỗi = %v", err)
	}
	if repo.writes != 0 || len(auditEntries(k)) != 0 || k.daRollback != 1 {
		t.Fatal("mã đã có mà vẫn ghi")
	}
}

func TestRemoveLetterTypeBlankReasonStoresTheFixedSentence(t *testing.T) {
	k := khoVBMau()
	repo := newLetterTypeRepo(domain.CitizenLetterTypeEntry{ID: "ldt-1", Code: domain.LetterTypeRequest, Label: "Đề nghị"})
	uc, ctx := buildLetterTypes(t, k, repo)
	if err := uc.RemoveType(ctx, "ldt-1", "  ", clerk.Actor); err != nil {
		t.Fatalf("RemoveType: %v", err)
	}
	if repo.deleted["ldt-1"] != domain.CitizenLetterTypeDeleteDefaultReason {
		t.Fatalf("lý do lưu = %q", repo.deleted["ldt-1"])
	}
	a := auditEntries(k)
	if len(a) != 1 || a[0].args[4] != ActionDeleteCitizenLetterType || !strings.Contains(auditDelta(k, 0), "không nhập lý do") {
		t.Fatal("xoá phải có đúng một vết mang câu cố định")
	}
}

func TestEditLetterTypeNoOpWritesNothing(t *testing.T) {
	k := khoVBMau()
	repo := newLetterTypeRepo(domain.CitizenLetterTypeEntry{ID: "ldt-1", Code: domain.LetterTypeRequest, Label: "Đề nghị", Active: true})
	uc, ctx := buildLetterTypes(t, k, repo)
	same := "Đề nghị"
	if _, err := uc.EditType(ctx, "ldt-1", EditLetterTypeRequest{Label: &same}, clerk.Actor); err != nil {
		t.Fatalf("EditType: %v", err)
	}
	if repo.writes != 0 || len(auditEntries(k)) != 0 {
		t.Fatal("không đổi gì mà vẫn ghi")
	}
	off := false
	if _, err := uc.EditType(ctx, "ldt-1", EditLetterTypeRequest{Active: &off}, clerk.Actor); err != nil {
		t.Fatalf("EditType: %v", err)
	}
	if repo.rows["ldt-1"].Active || len(auditEntries(k)) != 1 {
		t.Fatal("tắt dòng phải ghi và để một vết")
	}
}
