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
)

// THE ACCOUNTLESS STATEMENTS — ADR 0083 (TEMPORARY), at the layer that knows SQL.
//
//	PROVED HERE   the lookup binds commune, the Mini App channel and the code, demands BOTH owner
//	              columns NULL and excludes soft-deleted rows · a scanned row that is not accountless (a
//	              citizen's petition the fake driver hands back regardless of WHERE) is ErrPhieuKhongTonTai
//	              · the ceiling count uses the same predicate, binds the day start and counts soft-deleted
//	              rows · the lock is per commune · an empty code runs no statement.
//
//	NOT PROVED    PostgreSQL itself.

func accountlessRow() map[string]driver.Value {
	return dongPhieu(map[string]driver.Value{"cong_dan_id": nil, "zalo_account_id": nil})
}

func TestAccountlessByCodeBindsChannelAndBothOwnersNull(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{accountlessRow()}}
	s := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k)))
	p, err := s.AccountlessByCode(ctxXa(xaThu), maThu)
	if err != nil {
		t.Fatalf("AccountlessByCode: %v", err)
	}
	if !p.Accountless() {
		t.Errorf("scanned petition is not accountless: %+v", p)
	}
	l := k.lenh[0]
	for _, w := range []string{"WHERE tenant_id = $1", "kenh_tiep_nhan = $2", "cong_dan_id IS NULL",
		"zalo_account_id IS NULL", "ma_tra_cuu = $3", "deleted_at IS NULL"} {
		if !strings.Contains(l.sql, w) {
			t.Errorf("statement lacks %q: %q", w, l.sql)
		}
	}
	if len(l.args) != 3 || l.args[0] != string(xaThu) || l.args[1] != "zalo-mini-app" || l.args[2] != maThu {
		t.Errorf("args = %v, want [commune, zalo-mini-app, code]", l.args)
	}
}

// The second wall: an OWNED petition with the code (the fake ignores WHERE) is the same not-found.
func TestAccountlessByCodeRefusesAnOwnedRow(t *testing.T) {
	for name, row := range map[string]map[string]driver.Value{
		"citizen":      dongPhieu(nil),
		"zalo account": dongPhieu(map[string]driver.Value{"cong_dan_id": nil, "zalo_account_id": "01JZALO"}),
		"staff-booked": dongPhieu(map[string]driver.Value{"cong_dan_id": nil, "kenh_tiep_nhan": "can-bo-nhap-ho"}),
	} {
		t.Run(name, func(t *testing.T) {
			k := &khoGia{hangTheoCot: []map[string]driver.Value{row}}
			s := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k)))
			if _, err := s.AccountlessByCode(ctxXa(xaThu), maThu); !errors.Is(err, ErrPhieuKhongTonTai) {
				t.Errorf("err = %v, want ErrPhieuKhongTonTai", err)
			}
		})
	}
}

func TestAccountlessByCodeEmptyCodeRunsNothing(t *testing.T) {
	k := &khoGia{}
	s := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k)))
	if _, err := s.AccountlessByCode(ctxXa(xaThu), " "); !errors.Is(err, ErrPhieuKhongTonTai) {
		t.Errorf("err = %v", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("ran %d statements for an empty code", len(k.lenh))
	}
}

func TestCountAccountlessUsesTheSamePredicateAndCountsSoftDeleted(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{{"count(*)": int64(199)}}}
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	s := NewPhieuPhanAnhStore(h)
	since := time.Date(2026, 10, 7, 17, 0, 0, 0, time.UTC)
	var n int
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		n, err = s.CountAccountlessPetitionsSince(ctx, tx, since)
		return err
	}); err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 199 {
		t.Errorf("count = %d", n)
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, accountlessWhere) || !strings.Contains(st.sql, "vao_so_luc >= $3") {
		t.Errorf("count does not use the shared predicate: %q", st.sql)
	}
	if strings.Contains(st.sql, "deleted_at") {
		t.Errorf("the ceiling count excludes soft-deleted rows — deleting spam would refill the allowance: %q", st.sql)
	}
	if len(st.args) != 3 || st.args[0] != string(xaThu) || st.args[1] != "zalo-mini-app" {
		t.Errorf("args = %v", st.args)
	}
}

func TestLockAccountlessIntakeIsPerCommune(t *testing.T) {
	d := &sfDB{affected: 1}
	h := pkgstore.New(sql.OpenDB(sfConnector{d: d}))
	s := NewPhieuPhanAnhStore(h)
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.LockAccountlessIntake(ctx, tx)
	}); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, "pg_advisory_xact_lock") || !strings.Contains(st.sql, "accountless") ||
		len(st.args) != 1 || st.args[0] != string(xaThu) {
		t.Errorf("lock = %q %v", st.sql, st.args)
	}
}
