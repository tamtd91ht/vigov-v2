package app

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The default-administrator seed at a commune's first `admin` sign-in (gieo_quan_tri.go).
//
// A FAKE DRIVER OF ITS OWN, not the one in dang_nhap_giao_dich_test.go: that one answers every
// `nguoi_dung` read with the same row, and the property under test here is a STATE CHANGE — no
// account before the seed transaction commits, the seeded account after it, and nothing at all
// when it rolls back. The real SQL (ON CONFLICT on a partitioned table, the FK to `quyen`, the
// CHECK of migration 0009 §3, two concurrent first sign-ins) is in gieo_quan_tri_pg_test.go.

// matKhauGieoGia is a fake seed password; the text says so (rule 8, forbidden #1).
const matKhauGieoGia = "mat-khau-gieo-GIA-KHONG-PHAI-THAT"

// soQuyenGq is how many catalogue keys the fake grant statement reports as added.
const soQuyenGq = 7

type trangThaiAdmin int

const (
	adminKhong     trangThaiAdmin = iota // no row with email `admin`
	adminHoatDong                        // a live account
	adminBiKhoa                          // dang_hoat_dong = false
	adminDaXoa                           // soft-deleted
	adminChiDanhBa                       // co_tai_khoan = false
)

type lenhGq struct {
	sql  string
	args []driver.Value
	tx   int
}

type khoGq struct {
	mu      sync.Mutex
	lenh    []lenhGq
	soTx    int
	ketThuc map[int]string
	loiTheo map[string]error

	// loiMotLan fails the FIRST statement containing the key, then forgets it.
	loiMotLan map[string]error

	admin       trangThaiAdmin
	vaiTroDaCo  bool // a `quan-tri-he-thong` role already exists
	vaiTroDaXoa bool // …and it is soft-deleted
	thuaDua     bool // a concurrent first sign-in commits `admin` just before this insert

	// cho[tx] is the account an uncommitted seed transaction inserted; daGieo is the committed one.
	cho    map[int][]driver.Value
	daGieo []driver.Value
}

func (k *khoGq) ghi(l lenhGq) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.lenh = append(k.lenh, l)
	for manh, err := range k.loiMotLan {
		if strings.Contains(l.sql, manh) {
			delete(k.loiMotLan, manh)
			return err
		}
	}
	for manh, err := range k.loiTheo {
		if strings.Contains(l.sql, manh) {
			return err
		}
	}
	return nil
}

func (k *khoGq) tatCa(manh string) []lenhGq {
	k.mu.Lock()
	defer k.mu.Unlock()
	var ra []lenhGq
	for _, l := range k.lenh {
		if strings.Contains(l.sql, manh) {
			ra = append(ra, l)
		}
	}
	return ra
}

func (k *khoGq) mot(t *testing.T, manh string) lenhGq {
	t.Helper()
	ds := k.tatCa(manh)
	if len(ds) != 1 {
		t.Fatalf("muốn đúng 1 câu lệnh chứa %q, có %d", manh, len(ds))
	}
	return ds[0]
}

func (k *khoGq) ketThucCua(tx int) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.ketThuc[tx]
}

// vetTheoHanhVi returns the audit statements carrying the given action (args[4]).
func (k *khoGq) vetTheoHanhVi(hanhVi string) []lenhGq {
	var ra []lenhGq
	for _, l := range k.tatCa("INSERT INTO audit_log") {
		if len(l.args) > 4 && l.args[4] == hanhVi {
			ra = append(ra, l)
		}
	}
	return ra
}

type ketNoiGq struct{ k *khoGq }

func (c ketNoiGq) Connect(context.Context) (driver.Conn, error) { return &connGq{k: c.k}, nil }
func (c ketNoiGq) Driver() driver.Driver                        { return trinhGia{} }

type connGq struct {
	k  *khoGq
	tx int
}

func (c *connGq) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connGq) Close() error { return nil }
func (c *connGq) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connGq) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.soTx++
	c.tx = c.k.soTx
	id := c.tx
	c.k.mu.Unlock()
	return &txGq{c: c, id: id}, nil
}

func (c *connGq) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	gt := giaTri(args)
	if err := c.k.ghi(lenhGq{sql: q, args: gt, tx: c.tx}); err != nil {
		return nil, err
	}
	c.k.mu.Lock()
	defer c.k.mu.Unlock()
	switch {
	case strings.Contains(q, "INSERT INTO vai_tro_quyen"):
		return driver.RowsAffected(soQuyenGq), nil
	case strings.Contains(q, "INSERT INTO vai_tro "):
		if c.k.vaiTroDaCo {
			return driver.RowsAffected(0), nil
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(q, "INSERT INTO nguoi_dung"):
		if c.k.thuaDua {
			// The winner committed first, with the same configured password.
			c.k.daGieo = hangGieo(gt)
			return driver.RowsAffected(0), nil
		}
		if c.k.cho == nil {
			c.k.cho = map[int][]driver.Value{}
		}
		c.k.cho[c.tx] = hangGieo(gt)
		return driver.RowsAffected(1), nil
	}
	return driver.RowsAffected(1), nil
}

// hangGieo builds the row TheoEmail would read back from ChenQuanTriMacDinh's arguments
// ($1 tenant, $2 id, $3 ma, $4 ho_ten, $5 email, $6 vai_tro_id, $7 hash), in cotNguoiDung order.
func hangGieo(a []driver.Value) []driver.Value {
	return []driver.Value{a[1], a[2], a[3], a[4], "", "", a[5], "", true, a[6], true, true}
}

func (c *connGq) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.k.ghi(lenhGq{sql: q, args: giaTri(args), tx: c.tx}); err != nil {
		return nil, err
	}
	c.k.mu.Lock()
	defer c.k.mu.Unlock()
	switch {
	case strings.Contains(q, "SELECT 1 FROM nguoi_dung"):
		if c.k.admin == adminKhong && c.k.daGieo == nil {
			return &rowsGq{cot: []string{"?column?"}}, nil
		}
		return &rowsGq{cot: []string{"?column?"}, hang: [][]driver.Value{{int64(1)}}}, nil

	case strings.Contains(q, "FROM vai_tro WHERE"):
		return &rowsGq{cot: []string{"id", "?column?"},
			hang: [][]driver.Value{{"vt-quan-tri", c.k.vaiTroDaXoa}}}, nil

	case strings.Contains(q, "FROM nguoi_dung"):
		// TheoEmail / TheoID: only a LIVE account (not deleted, has an account, not locked).
		if c.k.daGieo != nil {
			return &rowsGq{cot: cotNguoiDung, hang: [][]driver.Value{c.k.daGieo}}, nil
		}
		if c.k.admin == adminHoatDong {
			return &rowsGq{cot: cotNguoiDung, hang: [][]driver.Value{{
				idNoiBo, maCanBo, "Nguyễn Văn A", "admin", "", "", "vt-001", "",
				false, bamGia(), true, true,
			}}}, nil
		}
		return &rowsGq{cot: cotNguoiDung}, nil
	}
	return &rowsGq{}, nil
}

type txGq struct {
	c  *connGq
	id int
}

func (t *txGq) Commit() error   { return t.dong("commit") }
func (t *txGq) Rollback() error { return t.dong("rollback") }

func (t *txGq) dong(sao string) error {
	k := t.c.k
	k.mu.Lock()
	k.ketThuc[t.id] = sao
	if hang, co := k.cho[t.id]; co {
		if sao == "commit" {
			k.daGieo = hang
		}
		delete(k.cho, t.id)
	}
	k.mu.Unlock()
	t.c.tx = 0
	return nil
}

type rowsGq struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsGq) Columns() []string { return r.cot }
func (r *rowsGq) Close() error      { return nil }
func (r *rowsGq) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

type kyGq struct{}

func (kyGq) Ky(c token.Claims) (string, error) { return "token-gia." + c.Sid, nil }

type banThuGq struct {
	uc  *DangNhap
	kho *khoGq
	log *bytes.Buffer
}

// dungGq builds the use case over the fake store. matKhau "" leaves the seed off.
func dungGq(t *testing.T, matKhau string) *banThuGq {
	t.Helper()
	k := &khoGq{ketThuc: map[int]string{}, loiTheo: map[string]error{}, loiMotLan: map[string]error{}}
	db := sql.OpenDB(ketNoiGq{k: k})
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	uc := NewDangNhap(kho, idstore.NewCanBoStore(kho), idstore.NewPhienStore(kho), kyGq{}, log)
	if err := uc.BatGieoQuanTri(secret.Secret(matKhau)); err != nil {
		t.Fatalf("BatGieoQuanTri: %v", err)
	}
	if uc.gieo != nil {
		uc.gieo.bayGio = func() time.Time { return time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC) }
	}
	return &banThuGq{uc: uc, kho: k, log: &buf}
}

func yeuCauAdmin(matKhau string) YeuCauDangNhap {
	return YeuCauDangNhap{Email: "admin", MatKhau: matKhau, IP: ipGia, ThietBi: "Mozilla/5.0"}
}

// khongGhiGi asserts the seed wrote nothing and opened no session.
func (b *banThuGq) khongGhiGi(t *testing.T) {
	t.Helper()
	for _, manh := range []string{"INSERT INTO vai_tro", "INSERT INTO nguoi_dung", "INSERT INTO phien", "INSERT INTO audit_log"} {
		if n := len(b.kho.tatCa(manh)); n != 0 {
			t.Errorf("%d câu lệnh %q — không được ghi gì", n, manh)
		}
	}
}

// --- off / wrong password / wrong email: exactly today's behaviour ---------------------------------

func TestGieoTatKhiKhongCauHinhMatKhau(t *testing.T) {
	b := dungGq(t, "")
	_, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia))
	if !errors.Is(err, ErrDangNhapThatBai) {
		t.Fatalf("muốn ErrDangNhapThatBai, nhận %v", err)
	}
	if b.kho.soTx != 0 {
		t.Errorf("mở %d giao dịch khi tính năng tắt", b.kho.soTx)
	}
	if n := len(b.kho.lenh); n != 1 {
		t.Errorf("muốn đúng 1 câu lệnh (TheoEmail) như hôm nay, có %d", n)
	}
	b.khongGhiGi(t)
}

func TestGieoSaiMatKhauTraLoiNhuMoiLanSaiMatKhau(t *testing.T) {
	// No database work beyond today's lookup, no argon2, the same sentinel and the same single
	// log line as an unknown email — nothing a caller or a log reader can tell apart.
	b := dungGq(t, matKhauGieoGia)
	_, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin("mat-khau-sai-hoan-toan"))
	if err != ErrDangNhapThatBai { //nolint:errorlint // the SAME value, not merely a wrap of it
		t.Fatalf("muốn đúng ErrDangNhapThatBai, nhận %v", err)
	}
	if b.kho.soTx != 0 || len(b.kho.lenh) != 1 {
		t.Errorf("sai mật khẩu mà có %d giao dịch, %d câu lệnh", b.kho.soTx, len(b.kho.lenh))
	}
	b.khongGhiGi(t)

	chuan := dungGq(t, "")
	yc := yeuCauAdmin("mat-khau-sai-hoan-toan")
	yc.Email = "khong.ton.tai@example.gov.vn"
	_, _ = chuan.uc.Chay(ctxXa(xaThu), yc)
	if boVanTay(b.log.String()) != boVanTay(chuan.log.String()) {
		t.Errorf("dòng log khác lần sai thường:\n%s\n---\n%s", b.log.String(), chuan.log.String())
	}
}

// boVanTay strips what legitimately differs between two refusal lines: the time and the email
// fingerprint.
func boVanTay(s string) string {
	var ra []string
	for _, tu := range strings.Fields(s) {
		if strings.HasPrefix(tu, "time=") || strings.HasPrefix(tu, "email_van_tay=") {
			continue
		}
		ra = append(ra, tu)
	}
	return strings.Join(ra, " ")
}

func TestGieoChiVoiEmailDungChuAdmin(t *testing.T) {
	for _, email := range []string{"Admin", " admin", "admin ", "ADMIN", "admin@example.gov.vn", "quantri"} {
		t.Run(email, func(t *testing.T) {
			b := dungGq(t, matKhauGieoGia)
			yc := yeuCauAdmin(matKhauGieoGia)
			yc.Email = email
			if _, err := b.uc.Chay(ctxXa(xaThu), yc); !errors.Is(err, ErrDangNhapThatBai) {
				t.Fatalf("muốn ErrDangNhapThatBai, nhận %v", err)
			}
			if b.kho.soTx != 0 {
				t.Errorf("email %q mở giao dịch gieo", email)
			}
			b.khongGhiGi(t)
		})
	}
}

func TestBatGieoQuanTriNganThiTat(t *testing.T) {
	const ngan = "ngan-11-kt" // 10 runes < password.DaiToiThieu
	b := dungGq(t, "")
	err := b.uc.BatGieoQuanTri(secret.Secret(ngan))
	if !errors.Is(err, ErrMatKhauGieoNgan) {
		t.Fatalf("muốn ErrMatKhauGieoNgan, nhận %v", err)
	}
	if strings.Contains(err.Error(), ngan) {
		t.Error("lỗi mang giá trị mật khẩu")
	}
	if _, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(ngan)); !errors.Is(err, ErrDangNhapThatBai) {
		t.Fatalf("muốn ErrDangNhapThatBai, nhận %v", err)
	}
	b.khongGhiGi(t)
}

// --- the seed itself -----------------------------------------------------------------------------

func TestGieoDungMatKhauTaoVaiTroQuyenTaiKhoanVetCungMotGiaoDich(t *testing.T) {
	b := dungGq(t, matKhauGieoGia)
	kq, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia))
	if err != nil {
		t.Fatalf("đăng nhập lần đầu lỗi: %v", err)
	}

	vaiTro := b.kho.mot(t, "INSERT INTO vai_tro ")
	cap := b.kho.mot(t, "INSERT INTO vai_tro_quyen")
	nguoi := b.kho.mot(t, "INSERT INTO nguoi_dung")
	vetGieo := b.kho.vetTheoHanhVi(HanhViGieoQuanTriMacDinh)
	if len(vetGieo) != 1 {
		t.Fatalf("muốn 1 vết gieo, có %d", len(vetGieo))
	}
	tx := vaiTro.tx
	if tx == 0 {
		t.Fatal("gieo chạy ngoài giao dịch")
	}
	for ten, l := range map[string]lenhGq{"cấp quyền": cap, "tài khoản": nguoi, "vết": vetGieo[0]} {
		if l.tx != tx {
			t.Errorf("%s ở giao dịch %d, vai trò ở %d — luật 6 bất biến 3 đòi CÙNG một giao dịch", ten, l.tx, tx)
		}
	}
	if b.kho.ketThucCua(tx) != "commit" {
		t.Fatalf("giao dịch gieo kết thúc bằng %q", b.kho.ketThucCua(tx))
	}

	// Role: the fixed code and name.
	if vaiTro.args[3] != idstore.MaVaiTroQuanTriMacDinh || vaiTro.args[2] != idstore.TenVaiTroQuanTriMacDinh {
		t.Errorf("vai trò sai: %v", vaiTro.args)
	}
	// Grants: from the catalogue table, by the system, to the role just read back.
	if !strings.Contains(cap.sql, "FROM quyen") {
		t.Errorf("cấp quyền không đọc từ bảng quyen: %s", cap.sql)
	}
	if cap.args[1] != "vt-quan-tri" || cap.args[2] != "system" {
		t.Errorf("cấp quyền sai vai trò/người cấp: %v", cap.args)
	}

	// Account: email `admin`, the forced-change flag, a hash that verifies the configured value.
	if nguoi.args[4] != "admin" || nguoi.args[5] != "vt-quan-tri" {
		t.Errorf("tài khoản sai email/vai trò: %v", nguoi.args[:6])
	}
	if !strings.Contains(nguoi.sql, "phai_doi_mat_khau, mat_khau_hash)") ||
		!strings.Contains(nguoi.sql, "VALUES ($1, $2, $3, $4, $5, $6, true, true, true, $7)") {
		t.Errorf("tài khoản không đặt co_tai_khoan/dang_hoat_dong/phai_doi_mat_khau = true:\n%s", nguoi.sql)
	}
	bam, _ := nguoi.args[6].(string)
	if err := password.KiemTra(matKhauGieoGia, bam); err != nil {
		t.Errorf("hash không khớp mật khẩu cấu hình: %v", err)
	}
	ma, _ := nguoi.args[2].(string)
	if !strings.HasPrefix(ma, "CB-2026-") {
		t.Errorf("mã cán bộ %q không do SinhMaCanBo sinh", ma)
	}
	for _, a := range nguoi.args {
		if a == dienThoaiGia {
			t.Error("tài khoản mặc định mang số điện thoại")
		}
	}

	// Seed audit: system actor, the new account's business code, no credential in the delta.
	v := vetGieo[0].args
	if v[0] != string(xaThu) || v[1] != "system" || v[2] != "system" || v[3] != ipGia || v[5] != ma {
		t.Errorf("vết gieo sai: %v", v[:6])
	}
	delta := string(v[7].([]byte))
	var d map[string]any
	if err := json.Unmarshal([]byte(delta), &d); err != nil {
		t.Fatalf("delta không phải JSON: %v", err)
	}
	if strings.Contains(delta, matKhauGieoGia) || strings.Contains(delta, "$argon2id") {
		t.Errorf("delta mang mật khẩu hoặc hash: %s", delta)
	}
	if d["vai_tro"] != idstore.MaVaiTroQuanTriMacDinh || d["so_quyen_cap_them"] != float64(soQuyenGq) ||
		d["vai_tro_tao_moi"] != true {
		t.Errorf("delta thiếu nội dung: %s", delta)
	}

	// Then the ORDINARY sign-in: its own transaction, its own session and trail, on the new row.
	phien := b.kho.mot(t, "INSERT INTO phien")
	if phien.tx == tx || b.kho.ketThucCua(phien.tx) != "commit" {
		t.Errorf("phiên ở giao dịch %d (%q), gieo ở %d", phien.tx, b.kho.ketThucCua(phien.tx), tx)
	}
	vetDN := b.kho.vetTheoHanhVi("dang_nhap")
	if len(vetDN) != 1 || vetDN[0].tx != phien.tx || vetDN[0].args[1] != ma {
		t.Errorf("vết đăng nhập sai: %+v", vetDN)
	}
	if !kq.CanBo.PhaiDoiMatKhau || kq.CanBo.Ma != ma || kq.Sid == "" {
		t.Errorf("kết quả đăng nhập sai: phai_doi=%v ma=%q sid=%q", kq.CanBo.PhaiDoiMatKhau, kq.CanBo.Ma, kq.Sid)
	}
}

func TestGieoDungLaiVaiTroDaCo(t *testing.T) {
	b := dungGq(t, matKhauGieoGia)
	b.kho.vaiTroDaCo = true
	if _, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia)); err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	cap := b.kho.mot(t, "INSERT INTO vai_tro_quyen")
	if cap.args[1] != "vt-quan-tri" {
		t.Errorf("không cấp quyền cho vai trò đã có: %v", cap.args)
	}
	v := b.kho.vetTheoHanhVi(HanhViGieoQuanTriMacDinh)
	if len(v) != 1 || !strings.Contains(string(v[0].args[7].([]byte)), `"vai_tro_tao_moi":false`) {
		t.Errorf("vết không ghi rằng vai trò đã có sẵn: %+v", v)
	}
}

func TestGieoVaiTroDaXoaMemThiTuChoi(t *testing.T) {
	b := dungGq(t, matKhauGieoGia)
	b.kho.vaiTroDaCo, b.kho.vaiTroDaXoa = true, true
	if _, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia)); !errors.Is(err, ErrDangNhapThatBai) {
		t.Fatalf("muốn ErrDangNhapThatBai, nhận %v", err)
	}
	if len(b.kho.tatCa("INSERT INTO nguoi_dung")) != 0 || len(b.kho.tatCa("INSERT INTO phien")) != 0 {
		t.Error("vai trò đã xoá mềm mà vẫn tạo tài khoản hoặc phiên")
	}
	if b.kho.ketThucCua(1) != "rollback" {
		t.Errorf("giao dịch kết thúc bằng %q", b.kho.ketThucCua(1))
	}
}

func TestGieoHongGiuaChungThiQuayLuiVaTuChoi(t *testing.T) {
	for _, manh := range []string{"INSERT INTO vai_tro ", "INSERT INTO vai_tro_quyen", "INSERT INTO nguoi_dung", "INSERT INTO audit_log"} {
		t.Run(manh, func(t *testing.T) {
			b := dungGq(t, matKhauGieoGia)
			b.kho.loiTheo[manh] = errors.New("hỏng giả")
			_, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia))
			if err != ErrDangNhapThatBai { //nolint:errorlint // the same refusal as every other
				t.Fatalf("muốn đúng ErrDangNhapThatBai, nhận %v", err)
			}
			if b.kho.soTx != 1 || b.kho.ketThucCua(1) != "rollback" {
				t.Fatalf("muốn 1 giao dịch rollback, có %d (%q)", b.kho.soTx, b.kho.ketThucCua(1))
			}
			if b.kho.daGieo != nil {
				t.Error("tài khoản còn lại sau rollback")
			}
			if len(b.kho.tatCa("INSERT INTO phien")) != 0 {
				t.Error("gieo hỏng mà vẫn mở phiên")
			}
		})
	}
}

func TestGieoTrungMaCanBoThiSinhMaKhac(t *testing.T) {
	b := dungGq(t, matKhauGieoGia)
	b.kho.loiMotLan["INSERT INTO nguoi_dung"] = errors.New(`duplicate key value violates unique constraint "nguoi_dung_p07_tenant_id_ma_key"`)
	lan := 0
	b.uc.gieo.sinhMa = func(time.Time) (string, error) {
		lan++
		return []string{"CB-2026-AAAAAA", "CB-2026-BBBBBB"}[lan-1], nil
	}
	kq, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia))
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if kq.CanBo.Ma != "CB-2026-BBBBBB" || b.kho.ketThucCua(1) != "rollback" || b.kho.ketThucCua(2) != "commit" {
		t.Errorf("không thử lại bằng mã mới: ma=%q tx1=%q tx2=%q", kq.CanBo.Ma, b.kho.ketThucCua(1), b.kho.ketThucCua(2))
	}
}

func TestGieoThuaDuaThiDangNhapVaoTaiKhoanThangCuoc(t *testing.T) {
	// A concurrent first sign-in committed `admin` between this one's check and its insert. This
	// one writes nothing — in particular no second seed entry — and signs in against the winner.
	b := dungGq(t, matKhauGieoGia)
	b.kho.thuaDua = true
	if _, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia)); err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if b.kho.ketThucCua(1) != "rollback" {
		t.Errorf("giao dịch gieo thua cuộc kết thúc bằng %q", b.kho.ketThucCua(1))
	}
	for _, v := range b.kho.vetTheoHanhVi(HanhViGieoQuanTriMacDinh) {
		if b.kho.ketThucCua(v.tx) == "commit" {
			t.Error("bên thua cuộc vẫn để lại vết gieo")
		}
	}
	if len(b.kho.tatCa("INSERT INTO phien")) != 1 {
		t.Error("không đăng nhập vào tài khoản thắng cuộc")
	}
}

func TestGieoKhongChayKhiXaDaCoAdmin(t *testing.T) {
	for ten, tt := range map[string]trangThaiAdmin{
		"đang hoạt động": adminHoatDong, "bị khoá": adminBiKhoa,
		"đã xoá mềm": adminDaXoa, "chỉ danh bạ": adminChiDanhBa,
	} {
		t.Run(ten, func(t *testing.T) {
			b := dungGq(t, matKhauGieoGia)
			b.kho.admin = tt
			_, err := b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia))
			if !errors.Is(err, ErrDangNhapThatBai) {
				t.Fatalf("muốn ErrDangNhapThatBai, nhận %v", err)
			}
			for _, manh := range []string{"INSERT INTO vai_tro", "INSERT INTO nguoi_dung", "UPDATE nguoi_dung SET mat_khau_hash"} {
				if n := len(b.kho.tatCa(manh)); n != 0 {
					t.Errorf("%d câu lệnh %q — không bao giờ gieo đè hay hồi sinh", n, manh)
				}
			}
			if len(b.kho.tatCa("INSERT INTO phien")) != 0 {
				t.Error("mở phiên")
			}
		})
	}
}

func TestGieoMoiCauLenhMangXaCuaContext(t *testing.T) {
	for _, xa := range []tenant.ID{xaThu, xaKhac} {
		b := dungGq(t, matKhauGieoGia)
		if _, err := b.uc.Chay(ctxXa(xa), yeuCauAdmin(matKhauGieoGia)); err != nil {
			t.Fatalf("lỗi: %v", err)
		}
		if len(b.kho.lenh) == 0 {
			t.Fatal("không có câu lệnh")
		}
		for _, l := range b.kho.lenh {
			if len(l.args) == 0 || l.args[0] != string(xa) {
				t.Errorf("câu lệnh không mang xã %q ở $1: %s %v", xa, l.sql, l.args)
			}
		}
	}
}

func TestGieoLogKhongLoMatKhauHayHash(t *testing.T) {
	b := dungGq(t, matKhauGieoGia)
	_, _ = b.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia)) // seeds
	_, _ = b.uc.Chay(ctxXa(xaThu), yeuCauAdmin("mat-khau-sai-hoan-toan"))

	hong := dungGq(t, matKhauGieoGia)
	hong.kho.loiTheo["INSERT INTO nguoi_dung"] = errors.New("hỏng giả")
	_, _ = hong.uc.Chay(ctxXa(xaThu), yeuCauAdmin(matKhauGieoGia))
	if !strings.Contains(hong.log.String(), "gieo quản trị mặc định thất bại") {
		t.Error("gieo hỏng mà không có dòng log nào để lần")
	}

	for _, ra := range []string{b.log.String(), hong.log.String()} {
		for _, cam := range []string{matKhauGieoGia, "mat-khau-sai-hoan-toan", "$argon2id"} {
			if strings.Contains(ra, cam) {
				t.Errorf("log chứa %q:\n%s", cam, ra)
			}
		}
	}
}
