package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// CountOpenHeldByOrgUnit — the statement shape, without a database. What the predicate COUNTS is
// proved in org_unit_holdings_pg_test.go, which SKIPS without VIGOV_TEST_DSN.

const testUnitID = "01JBOPHAN0000000000000000A"

func TestPetitionHeldOpenStatement(t *testing.T) {
	d := &countDriver{row: []driver.Value{int64(3)}}
	n, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(d))).CountOpenHeldByOrgUnit(ctxXa(xaThu), testUnitID)
	if err != nil {
		t.Fatalf("CountOpenHeldByOrgUnit: %v", err)
	}
	if n != 3 {
		t.Errorf("n = %d, muốn 3", n)
	}
	if len(d.stmts) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(d.stmts))
	}
	l := d.stmts[0]
	want := "SELECT count(*) FROM phieu_phan_anh WHERE tenant_id = $1 AND deleted_at IS NULL AND bo_phan_id = $2 AND " +
		"trang_thai NOT IN ('da-dong', 'khong-tiep-nhan', 'chuyen-cap-tren')"
	if l.sql != want {
		t.Errorf("câu lệnh\n  %q\nmuốn\n  %q", l.sql, want)
	}
	if len(l.args) != 2 || l.args[0] != string(xaThu) || l.args[1] != testUnitID {
		t.Errorf("tham số = %v, muốn [xã từ context, mã bộ phận]", l.args)
	}
	// `da-xu-ly` and `cho-dan-xac-nhan` are OPEN (the citizen can send the petition back).
	for _, s := range []domain.TrangThai{domain.DaXuLy, domain.ChoDanXacNhan, domain.DangXuLy} {
		if strings.Contains(l.sql, "'"+string(s)+"'") {
			t.Errorf("trạng thái %s bị loại — nó vẫn là hồ sơ bộ phận đang giữ", s)
		}
	}
	// The history table is not the holder.
	if strings.Contains(l.sql, "nhat_ky") {
		t.Error("đọc nhật ký — lịch sử không phải người đang giữ")
	}
}

func TestTaskHeldOpenStatement(t *testing.T) {
	d := &countDriver{row: []driver.Value{int64(2)}}
	n, err := NewNhiemVuStore(store.New(sql.OpenDB(d))).CountOpenHeldByOrgUnit(ctxXa(xaThu), testUnitID)
	if err != nil {
		t.Fatalf("CountOpenHeldByOrgUnit: %v", err)
	}
	if n != 2 {
		t.Errorf("n = %d, muốn 2", n)
	}
	l := d.stmts[0]
	want := "SELECT count(*) FROM nhiem_vu WHERE tenant_id = $1 AND deleted_at IS NULL AND " +
		"bo_phan_id = $2 AND trang_thai <> 'hoan-thanh'"
	if l.sql != want {
		t.Errorf("câu lệnh\n  %q\nmuốn\n  %q", l.sql, want)
	}
	// ADR 0065 NV5: the lead unit IS bo_phan_id; the retired column is never read.
	if strings.Contains(l.sql, "co_quan_chu_tri_id") {
		t.Errorf("câu đếm còn đọc cột đã nghỉ co_quan_chu_tri_id: %s", l.sql)
	}
	if len(l.args) != 2 || l.args[0] != string(xaThu) || l.args[1] != testUnitID {
		t.Errorf("tham số = %v", l.args)
	}
	for _, s := range []domain.TrangThaiNhiemVu{domain.TamDung, domain.ChuyenTiep} {
		if strings.Contains(l.sql, "'"+string(s)+"'") {
			t.Errorf("trạng thái %s bị loại — nó có đường quay lại việc đang làm", s)
		}
	}
}

func TestHeldOpenRefusesBlankIDBeforeQuery(t *testing.T) {
	d := &countDriver{row: []driver.Value{int64(0)}}
	db := store.New(sql.OpenDB(d))
	if _, err := NewPhieuPhanAnhStore(db).CountOpenHeldByOrgUnit(ctxXa(xaThu), ""); !errors.Is(err, ErrOrgUnitIDBlank) {
		t.Errorf("phiếu: err = %v, muốn ErrOrgUnitIDBlank", err)
	}
	if _, err := NewNhiemVuStore(db).CountOpenHeldByOrgUnit(ctxXa(xaThu), ""); !errors.Is(err, ErrOrgUnitIDBlank) {
		t.Errorf("nhiệm vụ: err = %v, muốn ErrOrgUnitIDBlank", err)
	}
	if len(d.stmts) != 0 {
		t.Error("mã rỗng mà vẫn chạy câu đếm")
	}
}

// An error is never a zero: a zero lets the delete through.
func TestHeldOpenWrapsDriverErrorAndRefusesNoRow(t *testing.T) {
	cause := errors.New("mất kết nối")
	if _, err := NewNhiemVuStore(store.New(sql.OpenDB(&countDriver{err: cause}))).
		CountOpenHeldByOrgUnit(ctxXa(xaThu), testUnitID); !errors.Is(err, cause) {
		t.Errorf("lỗi không được bọc bằng %%w: %v", err)
	}
	if _, err := NewPhieuPhanAnhStore(store.New(sql.OpenDB(&countDriver{row: nil}))).
		CountOpenHeldByOrgUnit(ctxXa(xaThu), testUnitID); err == nil {
		t.Error("câu đếm không trả dòng nào mà đọc thành 0")
	}
}

// No commune in the context panics (store.DB.For → tenant.MustFrom) rather than counting across
// communes or defaulting to one.
func TestHeldOpenWithoutCommunePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("đếm được khi context không mang xã")
		}
	}()
	_, _ = NewNhiemVuStore(store.New(sql.OpenDB(&countDriver{row: []driver.Value{int64(0)}}))).
		CountOpenHeldByOrgUnit(context.Background(), testUnitID)
}
