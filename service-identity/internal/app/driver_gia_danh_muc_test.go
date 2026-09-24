package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// A fake database/sql driver for the two reference catalogues, so the REAL use case runs over the
// REAL store over a REAL transaction boundary — the argument driver_gia_bo_phan_test.go makes.
//
// IT ANSWERS FROM THE STATEMENT IT IS GIVEN: a statement naming `FROM khoi_nhiem_vu` sees only the
// task-bloc rows, so a store passing the wrong table constant reads the wrong catalogue and turns a
// test red; a lock read without `tenant_id = $1 AND id = $2` is answered across EVERY commune, as
// PostgreSQL would answer `WHERE id = $1`.
//
// WHAT IT DOES NOT PROVE: the trigger, the unique keys, FOR UPDATE really serialising. It has no
// trigger at all — which is exactly what makes the app-level tier refusals provable here: remove one
// and the UPDATE goes through. The database floor is danh_muc_ghi_pg_test.go, skipped without
// VIGOV_TEST_DSN.

type hangDanhMuc struct {
	bang     string
	xa       tenant.ID
	id, ma   string
	nhan     string
	thuTu    int
	macDinh  bool
	dangDung bool
	nguon    string
	reNhanh  bool
	daXoa    bool
}

type khoDanhMucGia struct {
	mu   sync.Mutex
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	hang []hangDanhMuc

	// loiSau fails the FIRST statement containing this substring.
	loiSau string
	daNo   bool
}

func (k *khoDanhMucGia) ghi(q string, args []driver.NamedValue) {
	k.mu.Lock()
	k.lenh = append(k.lenh, lenhGhi{sql: q, args: giaTri(args)})
	k.mu.Unlock()
}

func (k *khoDanhMucGia) cau(tu string) []lenhGhi {
	k.mu.Lock()
	defer k.mu.Unlock()
	var ra []lenhGhi
	for _, l := range k.lenh {
		if strings.Contains(l.sql, tu) {
			ra = append(ra, l)
		}
	}
	return ra
}

func (k *khoDanhMucGia) kiemLoi(q string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.loiSau != "" && !k.daNo && strings.Contains(q, k.loiSau) {
		k.daNo = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *khoDanhMucGia) Connect(context.Context) (driver.Conn, error) {
	return &connDanhMucGia{k: k}, nil
}
func (k *khoDanhMucGia) Driver() driver.Driver { return trinhGia{} }

type connDanhMucGia struct{ k *khoDanhMucGia }

func (c *connDanhMucGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connDanhMucGia) Close() error { return nil }
func (c *connDanhMucGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connDanhMucGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	return &txDanhMucGia{k: c.k}, nil
}

func (c *connDanhMucGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

// bangCua names the catalogue table a statement reads from, or "".
func bangCua(q string) string {
	for _, b := range []string{"loai_don_vi_dan_cu", "khoi_nhiem_vu"} {
		if strings.Contains(q, "FROM "+b+" ") {
			return b
		}
	}
	return ""
}

func (c *connDanhMucGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	c.k.mu.Lock()
	defer c.k.mu.Unlock()
	bang := bangCua(q)
	xa := tenant.ID(chuoiThu(args, 0))

	switch {
	case strings.Contains(q, "FOR UPDATE"):
		cot := []string{"id", "ma", "nhan", "la_mac_dinh", "dang_dung", "thu_tu", "nguon", "ma_nguon_re_nhanh"}
		theoXa := strings.Contains(q, "tenant_id = $1 AND id = $2")
		id := chuoiThu(args, 1)
		if !theoXa {
			id = chuoiThu(args, 0)
		}
		for _, h := range c.k.hang {
			if h.bang == bang && h.id == id && (!theoXa || h.xa == xa) &&
				(!strings.Contains(q, "deleted_at IS NULL") || !h.daXoa) {
				return &rowsGia{cot: cot, hang: [][]driver.Value{
					{h.id, h.ma, h.nhan, h.macDinh, h.dangDung, int64(h.thuTu), h.nguon, h.reNhanh},
				}}, nil
			}
		}
		return &rowsGia{cot: cot}, nil

	case strings.Contains(q, "SELECT count(*)") && strings.Contains(q, "AND ma = $2"):
		n := 0
		for _, h := range c.k.hang {
			// Deleted rows COUNTED, as the unique key counts them.
			if h.bang == bang && h.xa == xa && h.ma == chuoiThu(args, 1) {
				n++
			}
		}
		return &rowsGia{cot: []string{"count"}, hang: [][]driver.Value{{int64(n)}}}, nil

	case strings.Contains(q, "SELECT count(*)") && strings.Contains(q, "deleted_at IS NULL"):
		n := 0
		for _, h := range c.k.hang {
			if h.bang == bang && h.xa == xa && !h.daXoa {
				n++
			}
		}
		return &rowsGia{cot: []string{"count"}, hang: [][]driver.Value{{int64(n)}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

type txDanhMucGia struct{ k *khoDanhMucGia }

func (t *txDanhMucGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.mu.Unlock()
	return nil
}

func (t *txDanhMucGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.mu.Unlock()
	return nil
}

// --- fixtures -----------------------------------------------------------------------------------

const (
	xaDanhMuc      = tenant.ID("01JDANHMUCXAA000000000000A")
	xaDanhMucKhac  = tenant.ID("01JDANHMUCXAB000000000000B")
	maCanBoDanhMuc = "CB-0042"
	idCanBoDanhMuc = "nd-01JNOIBODANHMUC"
	idDanhMucMoi   = "01JDANHMUCMOI0000000000000"
)

// nguoiDanhMuc — Ma and ID DIFFERENT on purpose, so a test can tell which reached `actor_id` and
// `deleted_by`.
func nguoiDanhMuc() NguoiThucHien {
	return NguoiThucHien{
		ID:  idCanBoDanhMuc,
		Vet: audit.Actor{ID: maCanBoDanhMuc, Kind: "staff", IP: "10.0.0.7"},
	}
}

// hangMau gives EACH table the three tiers in commune A, a soft-deleted row, and one row in commune
// B — the same ids in both tables, so a store reading the wrong table finds the wrong ROW.
func hangMau() []hangDanhMuc {
	var ra []hangDanhMuc
	for _, b := range []string{"loai_don_vi_dan_cu", "khoi_nhiem_vu"} {
		ra = append(ra,
			hangDanhMuc{bang: b, xa: xaDanhMuc, id: "m-t1", ma: "cua-xa", nhan: "Của xã " + b, thuTu: 3, dangDung: true, nguon: "don-vi"},
			hangDanhMuc{bang: b, xa: xaDanhMuc, id: "m-t2", ma: "he-thong", nhan: "Hệ thống", thuTu: 2, dangDung: true, nguon: "he-thong"},
			hangDanhMuc{bang: b, xa: xaDanhMuc, id: "m-t3", ma: "re-nhanh", nhan: "Rẽ nhánh", thuTu: 1, macDinh: true, dangDung: true, nguon: "he-thong", reNhanh: true},
			hangDanhMuc{bang: b, xa: xaDanhMuc, id: "m-xoa", ma: "da-xoa", nhan: "Đã xoá", nguon: "don-vi", daXoa: true},
			hangDanhMuc{bang: b, xa: xaDanhMucKhac, id: "m-khac", ma: "cua-xa-b", nhan: "Xã B", dangDung: true, nguon: "don-vi"},
		)
	}
	return ra
}

// catalogueThu builds one catalogue's use case behind the same generic surface, so every test runs
// over BOTH instantiations.
type catalogueThu struct {
	ten                   string
	bang                  string
	hanhViThem, hanhViSua string
	hanhViXoa             string
	them                  func(ctx context.Context, yc YeuCauThemDanhMuc) (mucDanhMuc, error)
	sua                   func(ctx context.Context, id string, yc YeuCauSuaDanhMuc) (mucDanhMuc, error)
	xoa                   func(ctx context.Context, id, lyDo string) error
	nguoi                 *NguoiThucHien
}

func dungCatalogue(t *testing.T, k *khoDanhMucGia) []catalogueThu {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)

	nguoi := nguoiDanhMuc()
	ldv := NewDanhMucLoaiDonViDanCu(kho, idstore.NewLoaiDonViDanCuStore(kho))
	ldv.sinhID = func() (string, error) { return idDanhMucMoi, nil }
	knv := NewDanhMucKhoiNhiemVu(kho, idstore.NewKhoiNhiemVuStore(kho))
	knv.sinhID = func() (string, error) { return idDanhMucMoi, nil }

	return []catalogueThu{
		{
			ten: "loại đơn vị dân cư", bang: "loai_don_vi_dan_cu",
			hanhViThem: HanhViThemLoaiDonViDanCu, hanhViSua: HanhViSuaLoaiDonViDanCu, hanhViXoa: HanhViXoaLoaiDonViDanCu,
			them: func(ctx context.Context, yc YeuCauThemDanhMuc) (mucDanhMuc, error) {
				r, err := ldv.Them(ctx, yc, nguoi)
				return ldv.mo.sangMuc(r), err
			},
			sua: func(ctx context.Context, id string, yc YeuCauSuaDanhMuc) (mucDanhMuc, error) {
				r, err := ldv.Sua(ctx, id, yc, nguoi)
				return ldv.mo.sangMuc(r), err
			},
			xoa:   func(ctx context.Context, id, lyDo string) error { return ldv.Xoa(ctx, id, lyDo, nguoi) },
			nguoi: &nguoi,
		},
		{
			ten: "khối nhiệm vụ", bang: "khoi_nhiem_vu",
			hanhViThem: HanhViThemKhoiNhiemVu, hanhViSua: HanhViSuaKhoiNhiemVu, hanhViXoa: HanhViXoaKhoiNhiemVu,
			them: func(ctx context.Context, yc YeuCauThemDanhMuc) (mucDanhMuc, error) {
				r, err := knv.Them(ctx, yc, nguoi)
				return knv.mo.sangMuc(r), err
			},
			sua: func(ctx context.Context, id string, yc YeuCauSuaDanhMuc) (mucDanhMuc, error) {
				r, err := knv.Sua(ctx, id, yc, nguoi)
				return knv.mo.sangMuc(r), err
			},
			xoa:   func(ctx context.Context, id, lyDo string) error { return knv.Xoa(ctx, id, lyDo, nguoi) },
			nguoi: &nguoi,
		},
	}
}

func ctxDanhMuc() context.Context { return tenant.Into(context.Background(), xaDanhMuc) }
