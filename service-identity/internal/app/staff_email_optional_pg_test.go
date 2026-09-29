package app

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// THE HALF ONLY A REAL POSTGRESQL CAN DECIDE: that a blank address really lands as NULL, and that
// two such people in ONE commune both fit under `UNIQUE (tenant_id, email)` — the collision
// migration 0019 exists to remove. A fake store agrees with whatever it is told.
//
// Skipped unless VIGOV_TEST_DSN is set (see danh_ba_can_bo_pg_test.go).
func TestPgStaffTwoWithoutEmailSameCommuneBothStoredNull(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucThat(db)
	actor := nguoiPg("nd-quan-tri")

	var ids []string
	for _, blank := range []string{"", "   "} {
		cb, err := uc.Them(ctx, YeuCauThemCanBo{HoTen: "Trần Thị B", Email: blank}, actor)
		if err != nil {
			t.Fatalf("Them với email %q: %v — hai người không có thư điện tử phải cùng vào được", blank, err)
		}
		ids = append(ids, cb.ID)
	}

	for _, id := range ids {
		var email sql.NullString
		if err := db.QueryRow(`SELECT email FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`, xa, id).
			Scan(&email); err != nil {
			t.Fatalf("đọc lại: %v", err)
		}
		if email.Valid {
			t.Errorf("email lưu = %q, muốn NULL", email.String)
		}
	}
}

// CLEARING AN ACCOUNT HOLDER'S ADDRESS IS REFUSED ON THE REAL ROW, AND THE ROW KEEPS IT. Without an
// account, the same edit goes through and stores NULL.
func TestPgStaffClearEmailOnlyWithoutAccount(t *testing.T) {
	db := moPool(t)
	xa := xaRiengPg(t)
	withAccount := "nd-co-tai-khoan"
	dungXaPg(t, db, xa, "vt-thuong", []string{"document.read"}, withAccount)

	ctx := tenant.Into(context.Background(), tenant.ID(xa))
	uc := ucThat(db)
	actor := nguoiPg("nd-quan-tri")
	blank := ""

	if _, err := uc.Sua(ctx, withAccount, YeuCauSuaCanBo{Email: &blank}, actor); !errors.Is(err, ErrStaffEmailIsLogin) {
		t.Fatalf("xoá email người có tài khoản: lỗi = %v, muốn ErrStaffEmailIsLogin", err)
	}
	var email sql.NullString
	if err := db.QueryRow(`SELECT email FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`, xa, withAccount).
		Scan(&email); err != nil {
		t.Fatal(err)
	}
	if !email.Valid || email.String == "" {
		t.Fatal("bị từ chối mà thư điện tử vẫn bị xoá")
	}

	cb, err := uc.Them(ctx, YeuCauThemCanBo{HoTen: "Lê Văn C", Email: "levanc@xa.danang.gov.vn"}, actor)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if _, err := uc.Sua(ctx, cb.ID, YeuCauSuaCanBo{Email: &blank}, actor); err != nil {
		t.Fatalf("xoá email người KHÔNG có tài khoản: %v", err)
	}
	if err := db.QueryRow(`SELECT email FROM nguoi_dung WHERE tenant_id=$1 AND id=$2`, xa, cb.ID).
		Scan(&email); err != nil {
		t.Fatal(err)
	}
	if email.Valid {
		t.Errorf("email sau khi xoá = %q, muốn NULL", email.String)
	}
}
