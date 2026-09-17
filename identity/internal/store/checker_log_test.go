package store

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// The one line Checker writes says a staff member was REFUSED because their permissions could
// not be read — fail closed, which is right, but it means somebody in a government office is
// being turned away from their own work. One process serves 200+ communes into one log stream,
// so without the commune on that line an operator knows only that "somebody, somewhere, is
// locked out", which is not something they can act on.
//
// It runs on a driver that always fails, so no PostgreSQL is needed: the pg tests next door skip
// themselves without VIGOV_TEST_DSN, and a check nobody runs is a check that does not exist.

type trinhLuonLoi struct{}

func (trinhLuonLoi) Open(string) (driver.Conn, error) {
	return nil, errors.New("chỉ dùng Connector")
}

type ketNoiLuonLoi struct{}

func (ketNoiLuonLoi) Connect(context.Context) (driver.Conn, error) { return connLuonLoi{}, nil }
func (ketNoiLuonLoi) Driver() driver.Driver                        { return trinhLuonLoi{} }

type connLuonLoi struct{}

func (connLuonLoi) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (connLuonLoi) Close() error { return nil }
func (connLuonLoi) Begin() (driver.Tx, error) {
	return nil, errors.New("không mở được giao dịch")
}
func (connLuonLoi) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return nil, errors.New("connection reset by peer")
}

func TestKiemQuyenHongThiTuChoiVaGhiLogCoXa(t *testing.T) {
	db := sql.OpenDB(ketNoiLuonLoi{})
	defer db.Close()

	var buf bytes.Buffer
	c := NewChecker(pkgstore.New(db),
		slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	xa := tenant.ID("01J0000000000000000000000A")
	p := authz.Principal{ID: "nd-01JINTERNALIDCUACANBO", Kind: "staff", TenantID: xa}

	if c.Allows(tenant.Into(context.Background(), xa), p, "admin.user") {
		t.Fatal("kiểm quyền hỏng mà vẫn cho qua — phải fail closed (luật 5 bất biến 2)")
	}

	ra := buf.String()
	if !strings.Contains(ra, "level=ERROR") {
		t.Fatalf("không ghi log mức ERROR cho một lần từ chối vì lỗi hệ thống:\n%s", ra)
	}
	if !strings.Contains(ra, "xa="+string(xa)) {
		t.Errorf("dòng log thiếu xã — không biết xã nào đang có cán bộ bị từ chối:\n%s", ra)
	}
	if !strings.Contains(ra, "quyen=admin.user") {
		t.Errorf("dòng log thiếu quyền bị từ chối:\n%s", ra)
	}
}
