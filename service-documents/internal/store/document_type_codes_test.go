package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
)

// StatesByCode — the statement shape and the Scan, without a database (the fake driver of
// loai_van_ban_test.go, which builds rows BY COLUMN NAME from the SELECT list). What PostgreSQL
// matches is proved in document_type_codes_pg_test.go, which SKIPS without VIGOV_TEST_DSN.

func TestStatesByCodeStatement(t *testing.T) {
	k := &khoGia{hang: []hangGia{{ma: "cong-van", dangDung: false}}}
	got, err := dungKhoGia(k).StatesByCode(ctxXa(string(xaThu)), []string{"cong-van", "quyet-dinh"})
	if err != nil {
		t.Fatal(err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("%d statements, want 1", len(k.lenh))
	}
	l := k.lenh[0]
	want := "SELECT ma, dang_dung FROM loai_van_ban WHERE tenant_id = $1 AND deleted_at IS NULL AND ma IN ($2, $3)"
	if l.sql != want {
		t.Errorf("sql\n  %q\nwant\n  %q", l.sql, want)
	}
	wantArgs := []driver.Value{string(xaThu), "cong-van", "quyet-dinh"}
	if len(l.args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", l.args, wantArgs)
	}
	for i := range wantArgs {
		if l.args[i] != wantArgs[i] {
			t.Errorf("arg %d = %v, want %v", i, l.args[i], wantArgs[i])
		}
	}
	// active=false must survive the Scan: a switched-off type is ANSWERED, not dropped.
	if len(got) != 1 || got[0].Code != "cong-van" || got[0].Active {
		t.Errorf("= %+v, want one cong-van inactive", got)
	}
}

func TestStatesByCodeEmptyRunsNoStatement(t *testing.T) {
	k := &khoGia{hang: mauMotDong()}
	got, err := dungKhoGia(k).StatesByCode(ctxXa(string(xaThu)), nil)
	if err != nil || len(got) != 0 || len(k.lenh) != 0 {
		t.Errorf("got=%v err=%v statements=%d — empty must answer empty without a read", got, err, len(k.lenh))
	}
}

func TestStatesByCodeErrorIsWrapped(t *testing.T) {
	cause := errors.New("connection lost")
	if _, err := dungKhoGia(&khoGia{loi: cause}).StatesByCode(ctxXa(string(xaThu)), []string{"cong-van"}); !errors.Is(err, cause) {
		t.Errorf("not wrapped with %%w: %v", err)
	}
}

func TestStatesByCodeWithoutCommunePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("read with no commune in the context")
		}
	}()
	_, _ = dungKhoGia(&khoGia{}).StatesByCode(context.Background(), []string{"cong-van"})
}
