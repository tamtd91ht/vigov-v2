package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The REAL use case over the REAL store over a fake database/sql driver that records every statement
// and every transaction boundary — the same approach as driver_gia_loai_nhiem_vu_test.go, for the
// same reason: "one transaction", "a refusal writes nothing", "the commune is $1" are properties of
// the SQL and the transaction, and a hand-written fake store erases exactly that.
//
// NOT PROVED HERE: anything PostgreSQL does — the CHECK on `ma`, the PK conflict, the trigger. That is
// store/nhan_trang_thai_nhiem_vu_pg_test.go (skips without VIGOV_TEST_DSN).

var xaNhanTT = tenant.ID("01JA" + strings.Repeat("A", 22))

type lenhNhanTT struct {
	sql  string
	args []driver.Value
}

type khoNhanTTGia struct {
	mu   sync.Mutex
	lenh []lenhNhanTT

	batDau, daCommit, daRollback int

	// hang is the row the FOR UPDATE read returns; nil = the commune has no override for the code.
	hang *domain.NhanTrangThaiNhiemVu

	// loiSau fails the first statement containing this substring.
	loiSau string
}

func (k *khoNhanTTGia) ghi(q string, args []driver.NamedValue) error {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.lenh = append(k.lenh, lenhNhanTT{sql: q, args: gt})
	if k.loiSau != "" && strings.Contains(q, k.loiSau) {
		k.loiSau = ""
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *khoNhanTTGia) cau(tu string) []lenhNhanTT {
	k.mu.Lock()
	defer k.mu.Unlock()
	var ra []lenhNhanTT
	for _, l := range k.lenh {
		if strings.Contains(l.sql, tu) {
			ra = append(ra, l)
		}
	}
	return ra
}

func (k *khoNhanTTGia) Connect(context.Context) (driver.Conn, error) { return &connNhanTT{k: k}, nil }
func (k *khoNhanTTGia) Driver() driver.Driver                        { return trinhNhanTT{} }

type trinhNhanTT struct{}

func (trinhNhanTT) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type connNhanTT struct{ k *khoNhanTTGia }

func (c *connNhanTT) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("không hỗ trợ Prepare")
}
func (c *connNhanTT) Close() error { return nil }
func (c *connNhanTT) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *connNhanTT) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	return &txNhanTT{k: c.k}, nil
}

func (c *connNhanTT) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.k.ghi(q, args); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *connNhanTT) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.k.ghi(q, args); err != nil {
		return nil, err
	}
	if !strings.Contains(q, "FOR UPDATE") {
		return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
	}
	r := &rowsNhanTT{}
	if h := c.k.hang; h != nil {
		r.hang = [][]driver.Value{{string(h.Ma), h.Nhan, int64(h.ThuTu), h.CapNhatBoi}}
	}
	return r, nil
}

type txNhanTT struct{ k *khoNhanTTGia }

func (t *txNhanTT) Commit() error   { t.k.mu.Lock(); t.k.daCommit++; t.k.mu.Unlock(); return nil }
func (t *txNhanTT) Rollback() error { t.k.mu.Lock(); t.k.daRollback++; t.k.mu.Unlock(); return nil }

type rowsNhanTT struct {
	hang [][]driver.Value
	i    int
}

// Columns mirror cotNhanTrangThai's ORDER, written out so a reorder there turns this red.
func (r *rowsNhanTT) Columns() []string { return []string{"ma", "nhan", "thu_tu", "cap_nhat_boi"} }
func (r *rowsNhanTT) Close() error      { return nil }
func (r *rowsNhanTT) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

func dungUseCaseNhanTT(t *testing.T, k *khoNhanTTGia) (*NhanTrangThaiNhiemVu, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	return NewNhanTrangThaiNhiemVu(kho, docstore.NewNhanTrangThaiNhiemVuStore(kho)),
		tenant.Into(context.Background(), xaNhanTT)
}

// nguoiNhanTT carries the BUSINESS code, as nguoiThucHien builds it from p.Ma (rule 6, inv. 8).
func nguoiNhanTT() audit.Actor { return audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"} }

func conTroChuoi(s string) *string { return &s }
func conTroSo(n int) *int          { return &n }

// --- rule 6, invariant 3: one transaction, with the actor's business code ----------------------

func TestSuaNhanTrangThaiGhiVaVetCungMotGiaoDich(t *testing.T) {
	k := &khoNhanTTGia{}
	uc, ctx := dungUseCaseNhanTT(t, k)

	sau, err := uc.Sua(ctx, "moi-giao", YeuCauSuaNhanTrangThai{Nhan: conTroChuoi("Chưa thực hiện")}, nguoiNhanTT())
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("giao dịch: mở=%d commit=%d rollback=%d, muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}
	ghi := k.cau("INSERT INTO nhan_trang_thai_nhiem_vu")
	vet := k.cau("INSERT INTO audit_log")
	if len(ghi) != 1 || len(vet) != 1 {
		t.Fatalf("upsert=%d vết=%d, muốn 1/1", len(ghi), len(vet))
	}
	// The upsert: tenant $1 from the context, the code, the new label, the DEFAULT position (both
	// columns are NOT NULL, so a label-only first write still stores the effective position), and the
	// actor's business code in `cap_nhat_boi`.
	a := ghi[0].args
	if a[0] != string(xaNhanTT) || a[1] != "moi-giao" || a[2] != "Chưa thực hiện" || a[3] != int64(1) || a[4] != "CB-00123" {
		t.Errorf("tham số upsert = %v", a)
	}
	if !strings.Contains(ghi[0].sql, "ON CONFLICT (tenant_id, ma) DO UPDATE") {
		t.Errorf("không phải upsert theo khoá chính: %q", ghi[0].sql)
	}
	// The trail: commune, business-code actor, verb, subject = status code, before/after.
	v := vet[0].args
	if v[0] != string(xaNhanTT) || v[1] != "CB-00123" || v[2] != "staff" || v[3] != "10.0.0.7" ||
		v[4] != HanhViSuaNhanTrangThaiNhiemVu || v[5] != "moi-giao" {
		t.Errorf("vết = %v", v[:6])
	}
	var delta map[string]map[string]any
	if err := json.Unmarshal(v[7].([]byte), &delta); err != nil {
		t.Fatalf("delta: %v", err)
	}
	if delta["truoc"]["nhan"] != "Mới giao" || delta["sau"]["nhan"] != "Chưa thực hiện" {
		t.Errorf("delta trước/sau sai: %v", delta)
	}
	if _, co := delta["sau"]["thu_tu"]; co {
		t.Errorf("delta chứa trường không đổi: %v", delta)
	}
	if sau.Nhan != "Chưa thực hiện" || sau.ThuTu != 1 || !sau.DaTuyChinh || sau.NhanMacDinh != "Mới giao" {
		t.Errorf("kết quả = %+v", sau)
	}
}

func TestSuaNhanTrangThaiVetHongThiKhongCommit(t *testing.T) {
	k := &khoNhanTTGia{loiSau: "INSERT INTO audit_log"}
	uc, ctx := dungUseCaseNhanTT(t, k)
	if _, err := uc.Sua(ctx, "cho-duyet", YeuCauSuaNhanTrangThai{ThuTu: conTroSo(2)}, nguoiNhanTT()); err == nil {
		t.Fatal("vết hỏng mà vẫn báo thành công")
	}
	if len(k.cau("INSERT INTO nhan_trang_thai_nhiem_vu")) != 1 {
		t.Fatal("upsert phải đã chạy trước vết — không thì phép thử không phân biệt được rollback với chưa chạy")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestSuaNhanTrangThaiThieuMaCanBoThiTuChoiTruocGiaoDich(t *testing.T) {
	k := &khoNhanTTGia{}
	uc, ctx := dungUseCaseNhanTT(t, k)
	if _, err := uc.Sua(ctx, "moi-giao", YeuCauSuaNhanTrangThai{Nhan: conTroChuoi("X")},
		audit.Actor{Kind: "staff"}); err == nil {
		t.Fatal("không có người sửa mà vẫn ghi")
	}
	if k.batDau != 0 {
		t.Error("mở giao dịch dù không có người sửa")
	}
}

// --- #21: the code list is closed -------------------------------------------------------------

func TestSuaNhanTrangThaiMaNgoaiBayMaLaKhongTonTai(t *testing.T) {
	for _, ma := range []string{"da-huy", "", "MOI-GIAO", "moi-giao/1"} {
		k := &khoNhanTTGia{}
		uc, ctx := dungUseCaseNhanTT(t, k)
		_, err := uc.Sua(ctx, ma, YeuCauSuaNhanTrangThai{Nhan: conTroChuoi("Đã huỷ")}, nguoiNhanTT())
		if !errors.Is(err, docstore.ErrDanhMucKhongTonTai) {
			t.Errorf("%q: lỗi = %v, muốn ErrDanhMucKhongTonTai (404)", ma, err)
		}
		if k.batDau != 0 || len(k.lenh) != 0 {
			t.Errorf("%q: mã ngoài vòng đời mà vẫn chạm cơ sở dữ liệu", ma)
		}
	}
}

// --- shape of what a client may send ------------------------------------------------------------

func TestSuaNhanTrangThaiNhanDemTheoKyTu(t *testing.T) {
	// 100 runes of a 2-byte letter (200 bytes): accepted. 101: refused before any transaction.
	k := &khoNhanTTGia{}
	uc, ctx := dungUseCaseNhanTT(t, k)
	if _, err := uc.Sua(ctx, "tam-dung", YeuCauSuaNhanTrangThai{Nhan: conTroChuoi(strings.Repeat("ơ", 100))},
		nguoiNhanTT()); err != nil {
		t.Fatalf("100 ký tự bị từ chối — đang đếm byte: %v", err)
	}

	k2 := &khoNhanTTGia{}
	uc2, ctx2 := dungUseCaseNhanTT(t, k2)
	_, err := uc2.Sua(ctx2, "tam-dung", YeuCauSuaNhanTrangThai{Nhan: conTroChuoi(strings.Repeat("ơ", 101))}, nguoiNhanTT())
	if !errors.Is(err, domain.ErrNhanQuaDai) || k2.batDau != 0 {
		t.Fatalf("101 ký tự: lỗi = %v, giao dịch = %d", err, k2.batDau)
	}
}

func TestSuaNhanTrangThaiDauVaoSai(t *testing.T) {
	for ten, tc := range map[string]struct {
		yc  YeuCauSuaNhanTrangThai
		loi error
	}{
		"nhãn trống":      {YeuCauSuaNhanTrangThai{Nhan: conTroChuoi("   ")}, domain.ErrNhanTrong},
		"thứ tự 0":        {YeuCauSuaNhanTrangThai{ThuTu: conTroSo(0)}, domain.ErrThuTuNgoaiKhoang},
		"thứ tự âm":       {YeuCauSuaNhanTrangThai{ThuTu: conTroSo(-3)}, domain.ErrThuTuNgoaiKhoang},
		"nhãn 150 ký tự":  {YeuCauSuaNhanTrangThai{Nhan: conTroChuoi(strings.Repeat("ệ", 150))}, domain.ErrNhanQuaDai},
		"thứ tự quá trần": {YeuCauSuaNhanTrangThai{ThuTu: conTroSo(domain.ThuTuToiDa + 1)}, domain.ErrThuTuNgoaiKhoang},
	} {
		t.Run(ten, func(t *testing.T) {
			k := &khoNhanTTGia{}
			uc, ctx := dungUseCaseNhanTT(t, k)
			if _, err := uc.Sua(ctx, "moi-giao", tc.yc, nguoiNhanTT()); !errors.Is(err, tc.loi) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.loi)
			}
			if k.batDau != 0 {
				t.Error("đầu vào sai mà vẫn mở giao dịch")
			}
		})
	}
}

// --- idempotency: a no-op writes nothing -----------------------------------------------------------

func TestSuaNhanTrangThaiKhongDoiThiKhongGhiKhongVet(t *testing.T) {
	for ten, tc := range map[string]struct {
		hang *domain.NhanTrangThaiNhiemVu
		yc   YeuCauSuaNhanTrangThai
	}{
		"gửi lại đúng nhãn đang có": {
			&domain.NhanTrangThaiNhiemVu{Ma: domain.ChoDuyet, Nhan: "Chờ lãnh đạo duyệt", ThuTu: 4, CapNhatBoi: "CB-00999"},
			YeuCauSuaNhanTrangThai{Nhan: conTroChuoi("Chờ lãnh đạo duyệt"), ThuTu: conTroSo(4)},
		},
		"gửi giá trị mặc định khi chưa có dòng": {
			nil, YeuCauSuaNhanTrangThai{Nhan: conTroChuoi("Chờ duyệt"), ThuTu: conTroSo(4)},
		},
		"thân rỗng": {nil, YeuCauSuaNhanTrangThai{}},
	} {
		t.Run(ten, func(t *testing.T) {
			k := &khoNhanTTGia{hang: tc.hang}
			uc, ctx := dungUseCaseNhanTT(t, k)
			if _, err := uc.Sua(ctx, "cho-duyet", tc.yc, nguoiNhanTT()); err != nil {
				t.Fatal(err)
			}
			if len(k.cau("INSERT INTO nhan_trang_thai_nhiem_vu")) != 0 || len(k.cau("INSERT INTO audit_log")) != 0 {
				t.Error("không có gì đổi mà vẫn ghi hoặc để vết — idem.KhongCan của tuyến thành lời nói dối")
			}
		})
	}
}

func TestSuaNhanTrangThaiCoDongChiDoiThuTuGiuNhanCua_Xa(t *testing.T) {
	// A position-only edit on a code the commune already re-worded keeps THE COMMUNE'S label — not
	// the default — in the upsert.
	k := &khoNhanTTGia{hang: &domain.NhanTrangThaiNhiemVu{Ma: domain.TamDung, Nhan: "Đang treo", ThuTu: 6, CapNhatBoi: "CB-00999"}}
	uc, ctx := dungUseCaseNhanTT(t, k)
	sau, err := uc.Sua(ctx, "tam-dung", YeuCauSuaNhanTrangThai{ThuTu: conTroSo(1)}, nguoiNhanTT())
	if err != nil {
		t.Fatal(err)
	}
	a := k.cau("INSERT INTO nhan_trang_thai_nhiem_vu")[0].args
	if a[2] != "Đang treo" || a[3] != int64(1) || a[4] != "CB-00123" {
		t.Errorf("upsert = %v, muốn giữ nhãn của xã và ghi người sửa mới", a)
	}
	// FOR UPDATE read carried the commune as $1 too.
	if doc := k.cau("FOR UPDATE"); len(doc) != 1 || doc[0].args[0] != string(xaNhanTT) || doc[0].args[1] != "tam-dung" {
		t.Errorf("đọc để sửa = %v", doc)
	}
	if sau.Nhan != "Đang treo" || sau.ThuTu != 1 {
		t.Errorf("kết quả = %+v", sau)
	}
}
