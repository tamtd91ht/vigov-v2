package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// THE ZALO-ACCOUNT OWNER — ADR 0080, migration 0032 — at the layer that knows SQL.
//
// As in phieu_phan_anh_cong_dan_test.go, the fake drivers hand back rows REGARDLESS of the WHERE
// clause, so the isolation is asserted where it is decidable without PostgreSQL: the statement text
// and its bound arguments.
//
//	PROVED HERE   Tao binds `zalo_account_id` as $22 — the value on a Zalo-owned row, NULL (never '')
//	              on a citizen's · the owner read binds `zalo_account_id = $3` for an account and
//	              `cong_dan_id = $3` for a citizen, never both · an owner with no id or an undeclared
//	              kind is refused BEFORE any statement · the locking read carries FOR UPDATE · the
//	              ceiling count binds commune, account and day start and does NOT filter soft-deleted
//	              rows (on purpose) · the lock is keyed by commune and account · neither statement
//	              runs without an account.
//
//	NOT PROVED    PostgreSQL itself (the count's value, the lock's effect, 0032's CHECKs).

const zaloThu = "01JZALOACCOUNTSTORETHU000"

func TestCreateBindsZaloAccountOwner(t *testing.T) {
	p := createRow(nil, nil)
	p.CongDanID, p.ZaloAccountID = "", zaloThu
	st := runCreate(t, p)

	if !strings.Contains(st.sql, "publication_status, zalo_account_id)") {
		t.Fatalf("column list moved; $22 no longer means zalo_account_id: %s", st.sql)
	}
	if len(st.args) != 22 {
		t.Fatalf("%d arguments, want 22", len(st.args))
	}
	if st.args[21] != zaloThu {
		t.Errorf("$22 = %#v, want the account id", st.args[21])
	}
	if st.args[4] != nil {
		t.Errorf("$5 cong_dan_id = %#v, want NULL — a Zalo-owned petition has no citizen identity", st.args[4])
	}
}

func TestCreateCitizenRowBindsNullZaloAccount(t *testing.T) {
	st := runCreate(t, createRow(nil, nil))
	if st.args[21] != nil {
		t.Errorf("$22 = %#v, want NULL (never '') — migration 0032 refuses a blank owner", st.args[21])
	}
}

func TestOwnedByCodeBindsTheOwnersColumn(t *testing.T) {
	for _, c := range []struct {
		owner     domain.PetitionOwner
		want, cam string
	}{
		{domain.PetitionOwner{Kind: domain.OwnerZaloAccount, ID: zaloThu}, "zalo_account_id = $3", "cong_dan_id ="},
		{domain.PetitionOwner{Kind: domain.OwnerCitizen, ID: congDanThu}, "cong_dan_id = $3", "zalo_account_id ="},
	} {
		t.Run(string(c.owner.Kind), func(t *testing.T) {
			k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
			s := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k)))
			if _, err := s.OwnedByCode(ctxXa(xaThu), c.owner, maThu); err != nil {
				t.Fatalf("OwnedByCode: %v", err)
			}
			if len(k.lenh) != 1 {
				t.Fatalf("%d statements, want 1", len(k.lenh))
			}
			l := k.lenh[0]
			for _, w := range []string{"WHERE tenant_id = $1", "ma_tra_cuu = $2", c.want, "deleted_at IS NULL"} {
				if !strings.Contains(l.sql, w) {
					t.Errorf("statement lacks %q: %q", w, l.sql)
				}
			}
			if strings.Contains(l.sql, c.cam) {
				t.Errorf("statement filters on the OTHER owner column %q: %q", c.cam, l.sql)
			}
			if len(l.args) != 3 || l.args[0] != string(xaThu) || l.args[1] != maThu || l.args[2] != c.owner.ID {
				t.Errorf("args = %v, want [commune, code, owner id]", l.args)
			}
		})
	}
}

func TestOwnedByCodeRefusesAnInvalidOwnerBeforeAnyStatement(t *testing.T) {
	for name, o := range map[string]domain.PetitionOwner{
		"no id":           {Kind: domain.OwnerZaloAccount},
		"blank id":        {Kind: domain.OwnerZaloAccount, ID: "   "},
		"undeclared kind": {Kind: "staff", ID: "CB-00123"},
		"no kind":         {ID: zaloThu},
	} {
		t.Run(name, func(t *testing.T) {
			k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
			s := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k)))
			_, err := s.OwnedByCode(ctxXa(xaThu), o, maThu)
			if !errors.Is(err, ErrThieuDinhDanhCongDan) {
				t.Fatalf("err = %v, want ErrThieuDinhDanhCongDan (a wiring fault, never a filter switched off)", err)
			}
			if len(k.lenh) != 0 {
				t.Errorf("ran %d statements for an invalid owner", len(k.lenh))
			}
		})
	}
}

func TestOwnedForUpdateLocksTheAccountsPetition(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{dongPhieu(map[string]driver.Value{
		"cong_dan_id": nil, "zalo_account_id": zaloThu})}}
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	s := NewPhieuPhanAnhStore(h)
	var got domain.PhieuPhanAnh
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		got, err = s.OwnedForUpdate(ctx, tx, domain.PetitionOwner{Kind: domain.OwnerZaloAccount, ID: zaloThu}, maThu)
		return err
	}); err != nil {
		t.Fatalf("OwnedForUpdate: %v", err)
	}
	st := d.stmts[0]
	for _, w := range []string{"tenant_id = $1", "zalo_account_id = $3", "deleted_at IS NULL", "FOR UPDATE"} {
		if !strings.Contains(st.sql, w) {
			t.Errorf("statement lacks %q: %q", w, st.sql)
		}
	}
	if len(st.args) != 3 || st.args[2] != zaloThu {
		t.Errorf("args = %v", st.args)
	}
	// The scan reads the new column back, and the petition says it is unverified.
	if got.ZaloAccountID != zaloThu || got.CongDanID != "" || !got.ContactUnverified() {
		t.Errorf("scanned owner = (%q, %q)", got.CongDanID, got.ZaloAccountID)
	}
}

func TestCountZaloAccountPetitionsSinceCountsSoftDeletedToo(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{{"count(*)": int64(7)}}}
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	s := NewPhieuPhanAnhStore(h)
	since := time.Date(2026, 10, 7, 17, 0, 0, 0, time.UTC)
	var n int
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		n, err = s.CountZaloAccountPetitionsSince(ctx, tx, zaloThu, since)
		return err
	}); err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 7 {
		t.Errorf("count = %d, want 7", n)
	}
	st := d.stmts[0]
	for _, w := range []string{"tenant_id = $1", "zalo_account_id = $2", "vao_so_luc >= $3"} {
		if !strings.Contains(st.sql, w) {
			t.Errorf("statement lacks %q: %q", w, st.sql)
		}
	}
	// ON PURPOSE (petition_owner.go): a spam petition staff soft-deleted must not give its sender the
	// allowance back. A `deleted_at` predicate here is the regression this assertion exists for.
	if strings.Contains(st.sql, "deleted_at") {
		t.Errorf("the ceiling count excludes soft-deleted rows — deleting spam would refill the allowance: %q", st.sql)
	}
	if len(st.args) != 3 || st.args[0] != string(xaThu) || st.args[1] != zaloThu {
		t.Errorf("args = %v", st.args)
	}
}

func TestLockZaloAccountIntakeKeysByCommuneAndAccount(t *testing.T) {
	d := &sfDB{affected: 1}
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	s := NewPhieuPhanAnhStore(h)
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.LockZaloAccountIntake(ctx, tx, zaloThu)
	}); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, "pg_advisory_xact_lock") || len(st.args) != 2 ||
		st.args[0] != string(xaThu) || st.args[1] != zaloThu {
		t.Errorf("lock = %q %v, want a transaction lock on (commune, account)", st.sql, st.args)
	}
}

func TestCeilingStatementsRefuseAnEmptyAccount(t *testing.T) {
	d := &sfDB{affected: 1}
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	s := NewPhieuPhanAnhStore(h)
	_ = sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		if err := s.LockZaloAccountIntake(ctx, tx, " "); !errors.Is(err, ErrNoZaloAccount) {
			t.Errorf("lock err = %v", err)
		}
		if _, err := s.CountZaloAccountPetitionsSince(ctx, tx, "", time.Now()); !errors.Is(err, ErrNoZaloAccount) {
			t.Errorf("count err = %v", err)
		}
		return nil
	})
	if len(d.stmts) != 0 {
		t.Errorf("ran %d statements without an account", len(d.stmts))
	}
}
