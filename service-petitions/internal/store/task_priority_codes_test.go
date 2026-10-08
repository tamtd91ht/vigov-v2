package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
)

// StatesByCode — the statement shape and the Scan, without a database (the fake driver of
// driver_gia_test.go, which builds rows BY COLUMN NAME from the SELECT list). What PostgreSQL matches
// is proved in task_priority_codes_pg_test.go, which SKIPS without VIGOV_TEST_DSN.

func TestTaskPriorityStatesByCodeStatement(t *testing.T) {
	// la_mac_dinh true, dang_dung false: if the Scan ever read the wrong boolean, Active comes back true.
	k := &khoGia{hang: []hangGia{{ma: "khan", macDinh: true, dangDung: false}}}
	got, err := dungKhoUuTien(k).StatesByCode(ctxXa(xaThu), []string{"khan", "cao"})
	if err != nil {
		t.Fatal(err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("%d statements, want 1", len(k.lenh))
	}
	l := k.lenh[0]
	want := "SELECT ma, dang_dung FROM muc_uu_tien_nhiem_vu WHERE tenant_id = $1 AND deleted_at IS NULL AND ma IN ($2, $3)"
	if l.sql != want {
		t.Errorf("sql\n  %q\nwant\n  %q", l.sql, want)
	}
	wantArgs := []driver.Value{string(xaThu), "khan", "cao"}
	if len(l.args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", l.args, wantArgs)
	}
	for i := range wantArgs {
		if l.args[i] != wantArgs[i] {
			t.Errorf("arg %d = %v, want %v", i, l.args[i], wantArgs[i])
		}
	}
	// active=false must survive the Scan: a switched-off priority is ANSWERED, not dropped.
	if len(got) != 1 || got[0].Code != "khan" || got[0].Active {
		t.Errorf("= %+v, want one khan inactive", got)
	}
}

func TestTaskPriorityStatesByCodeEmptyRunsNoStatement(t *testing.T) {
	k := &khoGia{hang: mauUuTien()}
	got, err := dungKhoUuTien(k).StatesByCode(ctxXa(xaThu), nil)
	if err != nil || len(got) != 0 || len(k.lenh) != 0 {
		t.Errorf("got=%v err=%v statements=%d — empty must answer empty without a read", got, err, len(k.lenh))
	}
}

func TestTaskPriorityStatesByCodeErrorIsWrapped(t *testing.T) {
	cause := errors.New("connection lost")
	if _, err := dungKhoUuTien(&khoGia{loi: cause}).StatesByCode(ctxXa(xaThu), []string{"khan"}); !errors.Is(err, cause) {
		t.Errorf("not wrapped with %%w: %v", err)
	}
}

func TestTaskPriorityStatesByCodeWithoutCommunePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("read with no commune in the context")
		}
	}()
	_, _ = dungKhoUuTien(&khoGia{}).StatesByCode(context.Background(), []string{"khan"})
}
