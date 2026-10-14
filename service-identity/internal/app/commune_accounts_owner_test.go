package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: owner decision 15/10/2026 — a BRAND-NEW commune (the owner's act creates its
// `quan-tri-he-thong`) receives the eight template roles in the SAME transaction; a commune that
// already had an administrator role receives none. On the fake driver; the real statements are in
// commune_accounts_owner_pg_test.go.

type communeAccountStoreFake struct {
	roleCreated bool
}

func (f *communeAccountStoreFake) ListSignInAccounts(context.Context, *store.ScopedTx) ([]domain.StaffSignInAccount, error) {
	return nil, nil
}

func (f *communeAccountStoreFake) SignInAccountByID(_ context.Context, _ *store.ScopedTx, id string) (domain.StaffSignInAccount, error) {
	return domain.StaffSignInAccount{ID: id, RoleCode: domain.CommuneAdminRoleCode}, nil
}

func (f *communeAccountStoreFake) VaiTroQuanTriMacDinh(ctx context.Context, tx *store.ScopedTx, newID string) (string, bool, error) { // vi-name-ok: implements the existing store method
	_, err := tx.Exec(ctx, "GHI-GIA admin role", string(tx.TenantID()), newID)
	return newID, f.roleCreated, err
}

func (f *communeAccountStoreFake) CapMoiQuyen(ctx context.Context, tx *store.ScopedTx, roleID, by string) (int64, error) { // vi-name-ok: implements the existing store method
	_, err := tx.Exec(ctx, "GHI-GIA admin grants", string(tx.TenantID()), roleID, by)
	return 40, err
}

func (f *communeAccountStoreFake) InsertCommuneAdmin(ctx context.Context, tx *store.ScopedTx, in idstore.NewCommuneAdmin) error {
	_, err := tx.Exec(ctx, "GHI-GIA insert admin", string(tx.TenantID()), in.ID)
	return err
}

type resetterFake struct{}

func (resetterFake) ResetByOperator(context.Context, string, string, string, string, audit.Actor) (domain.CanBoTomTat, error) {
	return domain.CanBoTomTat{}, nil
}

func newOwnerAccountsBench(t *testing.T, roleCreated bool) (*CommuneAccountsByOwner, *roleTemplateRepoFake, *ghiChep) {
	t.Helper()
	db, g := moDB(t)
	tpl := &roleTemplateRepoFake{states: map[string]bool{}}
	uc := NewCommuneAccountsByOwner(db, &communeAccountStoreFake{roleCreated: roleCreated}, resetterFake{},
		communeLookupFake{found: true, active: true}, tpl)
	n := 0
	uc.newID = func() (string, error) {
		n++
		return fmt.Sprintf("id-%02d", n), nil
	}
	uc.newCode = func(time.Time) (string, error) { return "CB-2026-OWNER1", nil }
	uc.hash = func(string) (string, error) { return "hash-gia", nil }
	return uc, tpl, g
}

func TestOwnerCreateAdminSeedsTemplatesInTheSameTransactionForANewCommune(t *testing.T) {
	uc, tpl, g := newOwnerAccountsBench(t, true)
	owner := MiniAppOperator{Code: "VH-00001", IP: "203.0.113.11"}
	if _, err := uc.CreateAdmin(ctxXa(xaThu), owner, "quan.tri@xa.example.test", "Quản trị xã", "Tam-thoi-GIA-1"); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	if len(tpl.inserted) != len(domain.RoleTemplates()) {
		t.Fatalf("tạo %d vai trò mẫu, muốn %d", len(tpl.inserted), len(domain.RoleTemplates()))
	}
	// #14's check and the grantor locks are the HTTP route's, not this path's.
	for _, c := range tpl.calls {
		if c == "lock" || strings.HasPrefix(c, "held:") {
			t.Errorf("đường của chủ dự án không được gọi %q", c)
		}
	}
	// Granted by the owner's VH- code (cap_boi, rule 6 invariant 8).
	for _, by := range tpl.grantBy {
		if by != owner.Code {
			t.Errorf("cap_boi = %q, muốn %q", by, owner.Code)
		}
	}
	// Every statement — admin role, templates, account, the 8 template entries and the act's entry — in ONE
	// committed transaction.
	vet := vetDaGhi(g)
	if len(vet) != len(domain.RoleTemplates())+1 {
		t.Fatalf("có %d vết, muốn %d", len(vet), len(domain.RoleTemplates())+1)
	}
	tx := vet[0].tx
	g.mu.Lock()
	for _, l := range g.lenh {
		if (strings.Contains(l.sql, "GHI-GIA") || strings.Contains(l.sql, "audit_log")) && l.tx != tx {
			t.Errorf("câu %q ở giao dịch %d, muốn %d", l.sql, l.tx, tx)
		}
	}
	g.mu.Unlock()
	if g.ketThucCua(tx) != "commit" {
		t.Fatal("giao dịch không commit")
	}
	last := fmt.Sprint(vet[len(vet)-1].args)
	if !strings.Contains(last, ActionCommuneAdminCreatedByOperator) {
		t.Fatalf("vết cuối không phải của thao tác tạo quản trị: %s", last)
	}
}

func TestOwnerCreateAdminSeedsNothingWhenTheAdministratorRoleExisted(t *testing.T) {
	uc, tpl, _ := newOwnerAccountsBench(t, false)
	if _, err := uc.CreateAdmin(ctxXa(xaThu), MiniAppOperator{Code: "VH-00001"}, "b@xa.example.test", "B", "Tam-thoi-GIA-1"); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	if len(tpl.calls) != 0 {
		t.Fatalf("xã đã có vai trò quản trị mà vẫn gieo mẫu: %v", tpl.calls)
	}
}
