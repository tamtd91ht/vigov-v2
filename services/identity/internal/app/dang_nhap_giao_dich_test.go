package app

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/pkg/password"
	"github.com/vihat/vigov/pkg/store"
	"github.com/vihat/vigov/pkg/tenant"
	idstore "github.com/vihat/vigov/services/identity/internal/store"
)

// THE GAP THIS FILE CLOSES: nothing tested that signing in and signing out actually WRITE AN
// AUDIT ENTRY, and that the entry shares the transaction with the change (rule 6, invariants 1
// and 3). services/identity/internal/http tests the routes with the use cases replaced by
// fakes, so the transaction boundary — the single thing rule 6 turns on — was never executed.
//
// WHY THIS IS THE FIRST PRIORITY: the failure is SILENT. A session opens, the staff member
// works, the response is a 201, and only an inspection asking "who signed in on 14 March"
// discovers there is no answer. Nothing turns red, no alert fires, no citizen complains. It is
// also unrepairable after the fact: an audit entry cannot be reconstructed from a row that has
// already changed.
//
// It runs on a fake database/sql driver — no PostgreSQL, no Redis — because a test that needs
// infrastructure is a test that stops being run. The driver records which statement ran inside
// which transaction and how that transaction ended, which is exactly the property under test.
// The real SQL is covered by services/identity/internal/store/*_pg_test.go behind
// VIGOV_TEST_DSN.

const (
	xaThu   = tenant.ID("01J0000000000000000000000A")
	xaKhac  = tenant.ID("01J0000000000000000000000B")
	idNoiBo = "nd-01JINTERNALIDCUACANBO"
	maCanBo = "CB-001"
	emailCB = "canbo.a@example.gov.vn"

	// The agreed fake number (rule 3, invariant 5) and a password that says in its own text that
	// it is not real (rule 8, forbidden #1). Both are here so the assertions below can prove
	// neither ever reaches a statement, an audit entry or a log line.
	dienThoaiGia = "0900000000"
	matKhauGia   = "mat-khau-gia-KHONG-PHAI-THAT"
	ipGia        = "10.0.0.7"
)

// bamGia is computed once: argon2id is deliberately slow, and the cost is the same for every
// test in this file.
var bamGia = sync.OnceValue(func() string {
	b, err := password.Bam(matKhauGia)
	if err != nil {
		panic(err)
	}
	return b
})

// --- fake driver ---------------------------------------------------------------------------

type lenhGhi struct {
	sql  string
	args []driver.Value
	tx   int // 0 = outside any transaction
}

type ghiChep struct {
	mu               sync.Mutex
	lenh             []lenhGhi
	soTx             int
	ketThuc          map[int]string // tx id -> "commit" | "rollback"
	loiTheo          map[string]error
	khongCoNguoiDung bool
}

func moDB(t *testing.T) (*store.DB, *ghiChep) {
	t.Helper()
	g := &ghiChep{ketThuc: map[int]string{}, loiTheo: map[string]error{}}
	db := sql.OpenDB(ketNoiGia{g: g})
	// More than one connection on purpose: a nested transaction (the defect shape this file
	// hunts — an audit entry written in its OWN transaction) must be able to run and be caught,
	// not deadlock waiting for the single connection.
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { db.Close() })
	return store.New(db), g
}

func (g *ghiChep) ghi(l lenhGhi) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lenh = append(g.lenh, l)
	for manh, err := range g.loiTheo {
		if strings.Contains(l.sql, manh) {
			return err
		}
	}
	return nil
}

// tim returns the single statement containing manh, or nil.
func (g *ghiChep) tim(manh string) *lenhGhi {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := range g.lenh {
		if strings.Contains(g.lenh[i].sql, manh) {
			return &g.lenh[i]
		}
	}
	return nil
}

func (g *ghiChep) ketThucCua(tx int) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ketThuc[tx]
}

func (g *ghiChep) soGiaoDich() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.soTx
}

// moiThamSo walks every argument of every statement, so an assertion can state that a value
// never reached the database at all.
func (g *ghiChep) moiThamSo() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var ra []string
	for _, l := range g.lenh {
		ra = append(ra, l.sql)
		for _, a := range l.args {
			if s, ok := a.(string); ok {
				ra = append(ra, s)
			}
		}
	}
	return ra
}

type ketNoiGia struct{ g *ghiChep }

func (c ketNoiGia) Connect(context.Context) (driver.Conn, error) { return &connGia{g: c.g}, nil }
func (c ketNoiGia) Driver() driver.Driver                        { return trinhGia{} }

type trinhGia struct{}

func (trinhGia) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type connGia struct {
	g  *ghiChep
	tx int
}

func (c *connGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connGia) Close() error { return nil }
func (c *connGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.g.mu.Lock()
	c.g.soTx++
	c.tx = c.g.soTx
	id := c.tx
	c.g.mu.Unlock()
	return &txGia{c: c, id: id}, nil
}

func (c *connGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.g.ghi(lenhGhi{sql: q, args: giaTri(args), tx: c.tx}); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *connGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.g.ghi(lenhGhi{sql: q, args: giaTri(args), tx: c.tx}); err != nil {
		return nil, err
	}
	if !strings.Contains(q, "nguoi_dung") {
		return &rowsGia{}, nil
	}
	c.g.mu.Lock()
	trong := c.g.khongCoNguoiDung
	c.g.mu.Unlock()
	if trong {
		return &rowsGia{cot: cotNguoiDung}, nil
	}
	return &rowsGia{cot: cotNguoiDung, hang: [][]driver.Value{{
		idNoiBo, maCanBo, "Nguyễn Văn A", emailCB, "Công chức Văn phòng",
		"bp-001", "vt-001", dienThoaiGia, bamGia(), true,
	}}}, nil
}

var cotNguoiDung = []string{
	"id", "ma", "ho_ten", "email", "chuc_vu", "bo_phan_id", "vai_tro_id",
	"dien_thoai", "mat_khau_hash", "dang_hoat_dong",
}

func giaTri(args []driver.NamedValue) []driver.Value {
	ra := make([]driver.Value, 0, len(args))
	for _, a := range args {
		ra = append(ra, a.Value)
	}
	return ra
}

type txGia struct {
	c  *connGia
	id int
}

func (t *txGia) Commit() error   { return t.dong("commit") }
func (t *txGia) Rollback() error { return t.dong("rollback") }

func (t *txGia) dong(sao string) error {
	t.c.g.mu.Lock()
	t.c.g.ketThuc[t.id] = sao
	t.c.g.mu.Unlock()
	t.c.tx = 0
	return nil
}

type rowsGia struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsGia) Columns() []string { return r.cot }
func (r *rowsGia) Close() error      { return nil }
func (r *rowsGia) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

// --- harness ---------------------------------------------------------------------------------

type banThu struct {
	dangNhap *DangNhap
	dangXuat *DangXuat
	ghi      *ghiChep
	log      *bytes.Buffer
}

func dungBanThu(t *testing.T) *banThu {
	t.Helper()
	db, g := moDB(t)
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	canBo := idstore.NewCanBoStore(db)
	phien := idstore.NewPhienStore(db)
	return &banThu{
		dangNhap: NewDangNhap(db, canBo, phien, log),
		dangXuat: NewDangXuat(db, phien),
		ghi:      g,
		log:      &buf,
	}
}

func ctxXa(xa tenant.ID) context.Context {
	return tenant.Into(context.Background(), xa)
}

func yeuCauDung() YeuCauDangNhap {
	return YeuCauDangNhap{Email: emailCB, MatKhau: matKhauGia, IP: ipGia, ThietBi: "Mozilla/5.0"}
}

// --- rule 6: the entry, and the transaction it shares ------------------------------------------

func TestDangNhapGhiVetCungGiaoDichVoiPhien(t *testing.T) {
	b := dungBanThu(t)

	kq, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung())
	if err != nil {
		t.Fatalf("đăng nhập lỗi: %v", err)
	}
	if kq.Sid == "" {
		t.Fatal("không có sid")
	}

	phien := b.ghi.tim("INSERT INTO phien")
	vet := b.ghi.tim("INSERT INTO audit_log")
	stamp := b.ghi.tim("UPDATE nguoi_dung SET dang_nhap_gan_nhat")

	if phien == nil {
		t.Fatal("không tạo phiên")
	}
	if vet == nil {
		t.Fatal("ĐĂNG NHẬP KHÔNG ĐỂ LẠI VẾT — luật 6 bất biến 1")
	}
	if phien.tx == 0 {
		t.Fatal("tạo phiên chạy ngoài giao dịch")
	}
	if vet.tx != phien.tx {
		t.Fatalf("vết ở giao dịch %d, phiên ở giao dịch %d — luật 6 bất biến 3 đòi CÙNG một giao dịch",
			vet.tx, phien.tx)
	}
	if stamp == nil || stamp.tx != phien.tx {
		t.Error("mốc đăng nhập gần nhất phải nằm trong cùng giao dịch")
	}
	if got := b.ghi.ketThucCua(phien.tx); got != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", got)
	}
}

func TestDangNhapGhiVetHongThiKhongConPhienNao(t *testing.T) {
	// THE CASE THAT PROVES THE TWO ARE ONE. If the audit insert fails, the session insert must go
	// with it. Writing the entry "afterwards, if it works" would leave a session that exists with
	// no trail — a signed-in staff member nobody can account for, which is the state the records
	// rules do not permit (rule 6, forbidden #2).
	b := dungBanThu(t)
	b.ghi.loiTheo["audit_log"] = errors.New("ghi vết hỏng")

	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); err == nil {
		t.Fatal("ghi vết hỏng mà đăng nhập vẫn báo thành công")
	}

	phien := b.ghi.tim("INSERT INTO phien")
	if phien == nil {
		t.Fatal("không có câu lệnh tạo phiên để kiểm")
	}
	if got := b.ghi.ketThucCua(phien.tx); got != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback — phiên phải bị huỷ cùng vết", got)
	}
}

func TestDangNhapTaoPhienHongThiKhongCoVetMoCoi(t *testing.T) {
	// The mirror image: the business write fails, so there must be no entry claiming it happened.
	b := dungBanThu(t)
	b.ghi.loiTheo["INSERT INTO phien"] = errors.New("không ghi được phiên")

	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); err == nil {
		t.Fatal("tạo phiên hỏng mà đăng nhập vẫn báo thành công")
	}
	if b.ghi.tim("INSERT INTO audit_log") != nil {
		t.Fatal("có vết cho một lần đăng nhập không xảy ra")
	}
	if got := b.ghi.ketThucCua(1); got != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback", got)
	}
}

func TestVetDangNhapMangDuNguoiViecXaVaIP(t *testing.T) {
	// Rule 6, invariant 2: who · what · on which record · when · from which IP · in which commune.
	// The actor is the BUSINESS code, not the internal id: an internal id means nothing to the
	// person reading the trail during an inspection.
	b := dungBanThu(t)

	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); err != nil {
		t.Fatal(err)
	}
	vet := b.ghi.tim("INSERT INTO audit_log")
	if vet == nil {
		t.Fatal("không có vết")
	}
	// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
	if len(vet.args) != 8 {
		t.Fatalf("vết có %d cột, muốn 8: %v", len(vet.args), vet.args)
	}
	muon := map[int]any{
		0: string(xaThu), // in which commune
		1: maCanBo,       // who — business code
		2: "staff",
		3: ipGia, // from which IP
		4: "dang_nhap",
		5: maCanBo,
	}
	ten := map[int]string{0: "tenant_id", 1: "actor_id", 2: "actor_kind", 3: "actor_ip", 4: "action", 5: "subject"}
	for i, v := range muon {
		if vet.args[i] != v {
			t.Errorf("%s = %v, muốn %v", ten[i], vet.args[i], v)
		}
	}
	if vet.args[1] == idNoiBo {
		t.Error("vết ghi id nội bộ thay vì mã cán bộ — người đọc vết không tra được id nội bộ")
	}
	if vet.args[6] == nil {
		t.Error("vết không có thời điểm")
	}
}

func TestVetMangXaCuaContextChuKhongPhaiXaKhac(t *testing.T) {
	// Rule 1: the commune travels on the context. The same use case, the same account, two
	// communes — each entry must carry the commune the request actually arrived for. An entry
	// filed under the wrong commune is a record in another authority's archive.
	for _, xa := range []tenant.ID{xaThu, xaKhac} {
		b := dungBanThu(t)
		if _, err := b.dangNhap.Chay(ctxXa(xa), yeuCauDung()); err != nil {
			t.Fatal(err)
		}
		vet := b.ghi.tim("INSERT INTO audit_log")
		if vet == nil || vet.args[0] != string(xa) {
			t.Fatalf("vết ghi xã %v, muốn %q", vet.args[0], xa)
		}
		phien := b.ghi.tim("INSERT INTO phien")
		if phien.args[0] != string(xa) {
			t.Fatalf("phiên ghi xã %v, muốn %q", phien.args[0], xa)
		}
	}
}

func TestVetKhongChuaDuLieuCaNhanHoacMatKhau(t *testing.T) {
	// Rule 6, forbidden #4 and rule 3: the audit trail must not become a second store of personal
	// data. Nothing that reaches any statement may carry the phone number, the password or the
	// hash — the hash is a credential, and a trail retained 12 months is a long time to hold one.
	b := dungBanThu(t)

	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); err != nil {
		t.Fatal(err)
	}
	cam := map[string]string{
		dienThoaiGia: "số điện thoại là dữ liệu cá nhân (Nghị định 13/2023)",
		matKhauGia:   "mật khẩu thô",
		"$argon2id":  "chuỗi băm mật khẩu",
	}
	for _, v := range b.ghi.moiThamSo() {
		for xau, vi := range cam {
			// The SELECT that reads the account legitimately returns the hash; only what is
			// WRITTEN is under test here.
			if strings.HasPrefix(v, "SELECT") {
				continue
			}
			if strings.Contains(v, xau) {
				t.Errorf("%s lọt vào câu lệnh ghi: %q", vi, v)
			}
		}
	}
}

// --- failed sign-in ---------------------------------------------------------------------------

func TestDangNhapThatBaiThiKhongMoGiaoDichNaoCa(t *testing.T) {
	// A wrong password must leave no session, no stamp and no entry. It must also answer the same
	// way as an unknown email: telling them apart hands over a directory of which addresses exist
	// on this commune's domain.
	cases := map[string]func(*banThu) YeuCauDangNhap{
		"sai mật khẩu": func(*banThu) YeuCauDangNhap {
			yc := yeuCauDung()
			yc.MatKhau = "mat-khau-sai-hoan-toan"
			return yc
		},
		"email không tồn tại": func(b *banThu) YeuCauDangNhap {
			b.ghi.khongCoNguoiDung = true
			return yeuCauDung()
		},
	}
	for ten, dung := range cases {
		t.Run(ten, func(t *testing.T) {
			b := dungBanThu(t)
			yc := dung(b)

			_, err := b.dangNhap.Chay(ctxXa(xaThu), yc)
			if !errors.Is(err, ErrDangNhapThatBai) {
				t.Fatalf("muốn ErrDangNhapThatBai, nhận %v", err)
			}
			if n := b.ghi.soGiaoDich(); n != 0 {
				t.Errorf("mở %d giao dịch cho một lần đăng nhập thất bại", n)
			}
			if b.ghi.tim("INSERT INTO phien") != nil {
				t.Error("đăng nhập thất bại mà vẫn mở phiên")
			}
			if b.ghi.tim("INSERT INTO audit_log") != nil {
				t.Error("có vết cho một lần đăng nhập không thành")
			}
		})
	}
}

func TestLogDangNhapKhongChuaMatKhauSoDienThoaiHayHash(t *testing.T) {
	// Rule 3, invariant 1. Process logs flow into centralised logging, into backups and into
	// third-party monitoring; one value written there cannot be recalled from any of them. This
	// is the defect nobody sees until somebody reads the log pipeline.
	b := dungBanThu(t)

	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); err != nil {
		t.Fatal(err)
	}
	sai := yeuCauDung()
	sai.MatKhau = "mat-khau-sai-hoan-toan"
	_, _ = b.dangNhap.Chay(ctxXa(xaThu), sai)

	b2 := dungBanThu(t)
	b2.ghi.khongCoNguoiDung = true
	_, _ = b2.dangNhap.Chay(ctxXa(xaThu), yeuCauDung())

	for _, ra := range []string{b.log.String(), b2.log.String()} {
		for _, cam := range []string{matKhauGia, "mat-khau-sai-hoan-toan", dienThoaiGia, "$argon2id"} {
			if strings.Contains(ra, cam) {
				t.Errorf("log chứa %q:\n%s", cam, ra)
			}
		}
	}
}

// --- sign-out ----------------------------------------------------------------------------------

func TestDangXuatGhiVetCungGiaoDichVoiThuHoi(t *testing.T) {
	b := dungBanThu(t)

	if err := b.dangXuat.Chay(ctxXa(xaThu), "sid-0001", maCanBo, ipGia); err != nil {
		t.Fatalf("đăng xuất lỗi: %v", err)
	}

	thuHoi := b.ghi.tim("UPDATE phien SET thu_hoi_luc")
	vet := b.ghi.tim("INSERT INTO audit_log")
	if thuHoi == nil {
		t.Fatal("không thu hồi phiên")
	}
	if vet == nil {
		t.Fatal("ĐĂNG XUẤT KHÔNG ĐỂ LẠI VẾT — luật 6 bất biến 1")
	}
	if thuHoi.tx == 0 || vet.tx != thuHoi.tx {
		t.Fatalf("vết ở giao dịch %d, thu hồi ở giao dịch %d — phải cùng một giao dịch", vet.tx, thuHoi.tx)
	}
	if got := b.ghi.ketThucCua(thuHoi.tx); got != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", got)
	}
	if vet.args[0] != string(xaThu) || vet.args[1] != maCanBo || vet.args[4] != "dang_xuat" {
		t.Errorf("vết đăng xuất sai nội dung: %v", vet.args)
	}
	if thuHoi.args[0] != string(xaThu) {
		t.Errorf("thu hồi chạy với xã %v, muốn %q", thuHoi.args[0], xaThu)
	}
}

func TestDangXuatGhiVetHongThiPhienVanConHieuLuc(t *testing.T) {
	// If the entry cannot be written, the revocation must not stand either: a session that
	// disappears with no trail is a revocation nobody can account for, and the staff member is
	// told the sign-out failed — which is true.
	b := dungBanThu(t)
	b.ghi.loiTheo["audit_log"] = errors.New("ghi vết hỏng")

	if err := b.dangXuat.Chay(ctxXa(xaThu), "sid-0001", maCanBo, ipGia); err == nil {
		t.Fatal("ghi vết hỏng mà đăng xuất vẫn báo thành công")
	}
	thuHoi := b.ghi.tim("UPDATE phien SET thu_hoi_luc")
	if thuHoi == nil {
		t.Fatal("không có câu lệnh thu hồi để kiểm")
	}
	if got := b.ghi.ketThucCua(thuHoi.tx); got != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback", got)
	}
}

func TestDangXuatKhongCoXaThiPanicChuKhongThuHoiMoXa(t *testing.T) {
	// Fail closed on the isolation path: with no commune in the context there is no safe
	// revocation to perform, and guessing one would revoke a session in whichever commune the
	// query happened to match.
	b := dungBanThu(t)

	defer func() {
		if recover() == nil {
			t.Fatal("thiếu xã trong context mà vẫn chạy — phải panic (tenant.MustFrom)")
		}
		if b.ghi.soGiaoDich() != 0 {
			t.Error("đã mở giao dịch dù không biết xã nào")
		}
	}()
	_ = b.dangXuat.Chay(context.Background(), "sid-0001", maCanBo, ipGia)
}
