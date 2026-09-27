package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// The public directory read (danh_ba_cong_khai.go) — the STATEMENT, checked where it reaches the
// driver. This fake records and returns canned rows; it does NOT execute the predicate, so the
// row-level exclusions (unpublished, no consent, locked, soft-deleted, the other commune) are proven
// against a real PostgreSQL in danh_ba_cong_khai_pg_test.go. What is proven here without a database:
// the commune is $1 and comes from the context, every clause of the publication predicate is present,
// the JOIN repeats the commune, and no column outside the contract is selected.

type ckLenh struct {
	sql  string
	args []driver.Value
}

type ckGhi struct {
	lenh []ckLenh
	hang [][]driver.Value // returned for every query
}

type ckConnector struct{ g *ckGhi }

func (k ckConnector) Connect(context.Context) (driver.Conn, error) { return &ckConn{g: k.g}, nil }
func (k ckConnector) Driver() driver.Driver                        { return ckDriver{} }

type ckDriver struct{}

func (ckDriver) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type ckConn struct{ g *ckGhi }

func (c *ckConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("không hỗ trợ Prepare")
}
func (c *ckConn) Close() error              { return nil }
func (c *ckConn) Begin() (driver.Tx, error) { return nil, errors.New("không hỗ trợ giao dịch") }

func (c *ckConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	v := make([]driver.Value, len(args))
	for i, a := range args {
		v[i] = a.Value
	}
	c.g.lenh = append(c.g.lenh, ckLenh{sql: q, args: v})
	return &ckRows{hang: c.g.hang}, nil
}

type ckRows struct {
	hang [][]driver.Value
	i    int
}

func (r *ckRows) Columns() []string {
	return []string{"ho_ten", "chuc_vu", "ten", "dien_thoai_co_quan", "di_dong_ca_nhan", "co_zalo"}
}
func (r *ckRows) Close() error { return nil }
func (r *ckRows) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

func moKhoCongKhai(t *testing.T, hang [][]driver.Value) (*CanBoStore, *ckGhi) {
	t.Helper()
	g := &ckGhi{hang: hang}
	db := sql.OpenDB(ckConnector{g: g})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewCanBoStore(pkgstore.New(db)), g
}

// ckXa is a ULID-shaped commune id.
const ckXa = "01JCK00000000000000000000A"

func dongCongKhai(ten string) []driver.Value {
	return []driver.Value{ten, "Chủ tịch UBND xã", "LÃNH ĐẠO UBND XÃ", "0900000000", "0900000000", true}
}

func TestDanhBaCongKhaiBuocXaTuNguCanhVaDuMenhDe(t *testing.T) {
	kho, g := moKhoCongKhai(t, [][]driver.Value{dongCongKhai("Nguyễn Văn A")})

	ds, err := kho.DanhBaCongKhai(tenant.Into(context.Background(), ckXa))
	if err != nil {
		t.Fatalf("DanhBaCongKhai: %v", err)
	}
	if len(ds) != 1 || ds[0].HoTen != "Nguyễn Văn A" || ds[0].TenBoPhan != "LÃNH ĐẠO UBND XÃ" || !ds[0].CoZalo {
		t.Fatalf("đọc dòng sai: %+v", ds)
	}
	if len(g.lenh) != 1 {
		t.Fatalf("số câu lệnh = %d, muốn 1 (một lượt đọc, kèm tên bộ phận)", len(g.lenh))
	}
	l := g.lenh[0]
	if l.args[0] != ckXa {
		t.Fatalf("$1 = %v, muốn xã của ngữ cảnh %s", l.args[0], ckXa)
	}
	if l.args[1] != int64(TranDanhBaCongKhai+1) {
		t.Fatalf("LIMIT = %v, muốn trần + 1 = %d", l.args[1], TranDanhBaCongKhai+1)
	}

	// MUTATIONS THAT MUST TURN THIS RED: drop any one clause of the publication predicate, or the
	// commune from the JOIN.
	for _, menhDe := range []string{
		"WHERE nd.tenant_id = $1",
		"AND nd.deleted_at IS NULL",
		"AND nd.dang_hoat_dong\n",
		"AND nd.hien_tren_mini_app\n",
		"AND nd.dong_y_cong_khai_luc IS NOT NULL",
		"AND nd.dong_y_cong_khai_ghi_boi <> ''",
		"ON bp.tenant_id = nd.tenant_id",
		"AND bp.deleted_at IS NULL",
		"ORDER BY nd.thu_tu_danh_ba ASC NULLS LAST, bp.ten ASC NULLS LAST",
	} {
		if !strings.Contains(l.sql, menhDe) {
			t.Errorf("câu lệnh thiếu %q:\n%s", menhDe, l.sql)
		}
	}
	// `hien_tren_mini_app` must be required TRUE, never compared with a bound value a caller could flip.
	if strings.Contains(l.sql, "hien_tren_mini_app =") {
		t.Errorf("cờ công khai so với một tham số thay vì bắt buộc true:\n%s", l.sql)
	}
}

func TestDanhBaCongKhaiKhongChonCotNgoaiHopDong(t *testing.T) {
	// The column list IS the privacy guarantee of a PUBLIC route: what is never selected cannot leave.
	kho, g := moKhoCongKhai(t, nil)
	if _, err := kho.DanhBaCongKhai(tenant.Into(context.Background(), ckXa)); err != nil {
		t.Fatalf("DanhBaCongKhai: %v", err)
	}
	s := g.lenh[0].sql
	cot := s[strings.Index(s, "SELECT"):strings.Index(s, "FROM nguoi_dung")]
	for _, cam := range []string{"email", "nd.id", "nd.ma", "bo_phan_id", "vai_tro_id", "mat_khau_hash",
		"co_tai_khoan", "dong_y_cong_khai", "dang_nhap", "tenant_id", "bp.id", "bp.ma"} {
		if strings.Contains(cot, cam) {
			t.Errorf("danh bạ công khai chọn cột %q: %s", cam, cot)
		}
	}
}

func TestDanhBaCongKhaiVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	kho, _ := moKhoCongKhai(t, [][]driver.Value{dongCongKhai("A"), dongCongKhai("B")})
	ctx := tenant.Into(context.Background(), ckXa)

	ds, err := kho.danhBaCongKhai(ctx, 1)
	if !errors.Is(err, ErrQuaNhieuCanBoCongKhai) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuCanBoCongKhai", err)
	}
	if ds != nil {
		t.Fatalf("từ chối mà vẫn trả %d dòng — cắt bớt trá hình", len(ds))
	}
	if _, err := kho.danhBaCongKhai(ctx, 2); err != nil {
		t.Fatalf("đúng bằng trần mà bị từ chối: %v", err)
	}
}

func TestDanhBaCongKhaiKhongCoXaThiPanic(t *testing.T) {
	kho, g := moKhoCongKhai(t, nil)
	defer func() {
		if recover() == nil {
			t.Fatal("đọc danh bạ công khai không có xã trong ngữ cảnh mà không panic")
		}
		if len(g.lenh) != 0 {
			t.Fatalf("không có xã mà vẫn gửi %d câu lệnh", len(g.lenh))
		}
	}()
	kho.DanhBaCongKhai(context.Background()) //nolint:errcheck // must panic before returning
}
