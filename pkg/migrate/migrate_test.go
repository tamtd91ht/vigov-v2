package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

// THE GAP THIS FILE CLOSES: nothing applied migrations at all, so every property a migration
// runner is bought for — order, exactly-once, a checksum that catches an edited file, a
// transaction that leaves no half-applied schema, a lock that two replicas cannot both pass —
// had no test because it had no code.
//
// Every failure this covers is SILENT in production. A file applied twice, a file skipped, a
// file applied out of order: the service starts, requests succeed, and the fault surfaces later
// as a query against a column that is not there — or, worse, as a constraint everyone believes
// is enforced and is not. `0002_audit_log_append_only.sql` is exactly that case: it was
// committed, never applied, and until it runs "append-only" is a comment.
//
// It runs on a fake database/sql driver — no PostgreSQL — because a test that needs
// infrastructure is a test that stops being run. The fake keeps enough real behaviour to make
// the assertions mean something: transactions buffer their writes and lose them on rollback,
// the progress table is genuinely read back, and pg_advisory_lock genuinely blocks.

// --- fake driver ---------------------------------------------------------------------------

type lenhGhi struct {
	sql  string
	args []driver.Value
	tx   int // 0 = outside any transaction
}

type hangTienDo struct {
	ten      string
	checksum string
}

// luuTru is the fake database: statements seen, transactions and how they ended, the committed
// contents of the progress table, and one advisory lock.
type luuTru struct {
	mu      sync.Mutex
	lenh    []lenhGhi
	soTx    int
	ketThuc map[int]string // tx id -> "commit" | "rollback"
	daAp    []hangTienDo   // COMMITTED rows of schema_migration
	loiTheo map[string]error

	khoa           chan struct{} // capacity 1 — pg_advisory_lock, and it really blocks
	soLanKhoa      int
	soLanNha       int
	nhaKhiChuaKhoa int // unlock without a matching lock — a defect, not a hang
}

func moDB(t *testing.T, soKetNoi int) (*sql.DB, *luuTru) {
	t.Helper()
	g := &luuTru{
		ketThuc: map[int]string{},
		loiTheo: map[string]error{},
		khoa:    make(chan struct{}, 1),
	}
	db := sql.OpenDB(ketNoiGia{g: g})
	db.SetMaxOpenConns(soKetNoi)
	t.Cleanup(func() { db.Close() })
	return db, g
}

func (g *luuTru) ghi(l lenhGhi) error {
	g.mu.Lock()
	g.lenh = append(g.lenh, l)
	var loi error
	for manh, err := range g.loiTheo {
		if strings.Contains(l.sql, manh) {
			loi = err
			break
		}
	}
	g.mu.Unlock()
	return loi
}

func (g *luuTru) dem(manh string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, l := range g.lenh {
		if strings.Contains(l.sql, manh) {
			n++
		}
	}
	return n
}

func (g *luuTru) tim(manh string) *lenhGhi {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := range g.lenh {
		if strings.Contains(g.lenh[i].sql, manh) {
			return &g.lenh[i]
		}
	}
	return nil
}

// timTheoThamSo finds the statement containing manh whose arguments include thamSo — the only
// way to tell four INSERTs into one table apart.
func (g *luuTru) timTheoThamSo(manh, thamSo string) *lenhGhi {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := range g.lenh {
		if !strings.Contains(g.lenh[i].sql, manh) {
			continue
		}
		for _, a := range g.lenh[i].args {
			if s, ok := a.(string); ok && s == thamSo {
				return &g.lenh[i]
			}
		}
	}
	return nil
}

// tenDaAp lists the progress table as it stands after every transaction has ended, which is the
// only view that answers "did this file leave a row behind".
func (g *luuTru) tenDaAp() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var ra []string
	for _, h := range g.daAp {
		ra = append(ra, h.ten)
	}
	return ra
}

func (g *luuTru) datTienDo(hang ...hangTienDo) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.daAp = append(g.daAp, hang...)
}

func (g *luuTru) ketThucCua(tx int) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ketThuc[tx]
}

type ketNoiGia struct{ g *luuTru }

func (c ketNoiGia) Connect(context.Context) (driver.Conn, error) { return &connGia{g: c.g}, nil }
func (c ketNoiGia) Driver() driver.Driver                        { return trinhGia{} }

type trinhGia struct{}

func (trinhGia) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type connGia struct {
	g     *luuTru
	tx    int
	choXu []hangTienDo // writes of the OPEN transaction — visible to nobody until commit
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
	c.choXu = nil
	return &txGia{c: c, id: id}, nil
}

func (c *connGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	v := giaTri(args)
	if err := c.g.ghi(lenhGhi{sql: q, args: v, tx: c.tx}); err != nil {
		return nil, err
	}

	switch {
	// The lock is taken OUTSIDE g.mu on purpose: holding the store's mutex while blocking would
	// deadlock the very concurrency this models.
	case strings.Contains(q, "pg_advisory_unlock"):
		// NON-BLOCKING on purpose. A runner that unlocks without having locked is a defect a test
		// must be able to REPORT; a blocking receive would instead hang the suite for the full
		// go-test timeout, and a suite that hangs gets its assertions deleted rather than read.
		select {
		case <-c.g.khoa:
			c.g.mu.Lock()
			c.g.soLanNha++
			c.g.mu.Unlock()
		default:
			c.g.mu.Lock()
			c.g.nhaKhiChuaKhoa++
			c.g.mu.Unlock()
		}

	case strings.Contains(q, "pg_advisory_lock"):
		c.g.khoa <- struct{}{}
		c.g.mu.Lock()
		c.g.soLanKhoa++
		c.g.mu.Unlock()

	case strings.Contains(q, "INSERT INTO "+BangTienDo):
		h := hangTienDo{ten: chuoi(v, 0), checksum: chuoi(v, 1)}
		if c.tx != 0 {
			c.choXu = append(c.choXu, h)
		} else {
			c.g.datTienDo(h)
		}
	}
	return driver.RowsAffected(1), nil
}

func (c *connGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.g.ghi(lenhGhi{sql: q, args: giaTri(args), tx: c.tx}); err != nil {
		return nil, err
	}
	if !strings.Contains(q, "FROM "+BangTienDo) {
		return &rowsGia{}, nil
	}
	c.g.mu.Lock()
	hang := make([][]driver.Value, 0, len(c.g.daAp))
	for _, h := range c.g.daAp {
		hang = append(hang, []driver.Value{h.ten, h.checksum})
	}
	c.g.mu.Unlock()
	return &rowsGia{cot: []string{"ten", "checksum"}, hang: hang}, nil
}

type txGia struct {
	c  *connGia
	id int
}

func (t *txGia) Commit() error {
	t.c.g.datTienDo(t.c.choXu...)
	return t.dong("commit")
}

// Rollback DISCARDS the buffered writes. That is the whole point of the fake: a migration that
// fails half-way must leave no progress row, and a fake that kept the row would agree with a
// runner that wrote it outside the transaction.
func (t *txGia) Rollback() error { return t.dong("rollback") }

func (t *txGia) dong(sao string) error {
	t.c.g.mu.Lock()
	t.c.g.ketThuc[t.id] = sao
	t.c.g.mu.Unlock()
	t.c.choXu = nil
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

func giaTri(args []driver.NamedValue) []driver.Value {
	ra := make([]driver.Value, 0, len(args))
	for _, a := range args {
		ra = append(ra, a.Value)
	}
	return ra
}

func chuoi(v []driver.Value, i int) string {
	if i >= len(v) {
		return ""
	}
	s, _ := v[i].(string)
	return s
}

// --- migration sources ------------------------------------------------------------------------

// Each body carries a marker unique to its file, so "did this file run, and how many times" is
// answerable by counting statements.
func than(ten string) string { return "CREATE TABLE bang_" + ten + " (id text)" }

func bam(s string) string {
	t := sha256.Sum256([]byte(s))
	return hex.EncodeToString(t[:])
}

func nguonBa() fstest.MapFS {
	return fstest.MapFS{
		"0001_init.sql":   {Data: []byte(than("0001"))},
		"0002_them.sql":   {Data: []byte(than("0002"))},
		"0003_them_2.sql": {Data: []byte(than("0003"))},
	}
}

// fsDaoNguoc hands back directory entries in REVERSE name order.
//
// WHY IT IS HERE: fs.ReadDir only sorts on the generic path. embed.FS implements ReadDirFS, so
// the sort inside the runner is the only thing putting 0001 before 0002 — and a test fed
// already-sorted entries would pass with that sort deleted. This makes the ordering property
// actually load-bearing.
type fsDaoNguoc struct{ fs.FS }

func (f fsDaoNguoc) ReadDir(ten string) ([]fs.DirEntry, error) {
	muc, err := fs.ReadDir(f.FS, ten)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(muc)-1; i < j; i, j = i+1, j-1 {
		muc[i], muc[j] = muc[j], muc[i]
	}
	return muc, nil
}

func chay(t *testing.T, db *sql.DB, nguon fs.FS) (KetQua, error) {
	t.Helper()
	return Chay(context.Background(), db, nguon, "identity")
}

// --- order --------------------------------------------------------------------------------

func TestApTheoDungThuTuTenTep(t *testing.T) {
	// Out of order, a migration lands on a schema the previous one had not built yet. The error
	// then reads as a broken migration, and the file that gets "fixed" is the innocent one.
	db, g := moDB(t, 1)

	kq, err := chay(t, db, fsDaoNguoc{nguonBa()})
	if err != nil {
		t.Fatalf("chạy migration: %v", err)
	}

	muon := []string{"0001_init.sql", "0002_them.sql", "0003_them_2.sql"}
	if !bangNhau(kq.DaAp, muon) {
		t.Fatalf("áp theo thứ tự %v, muốn %v", kq.DaAp, muon)
	}
	if got := g.tenDaAp(); !bangNhau(got, muon) {
		t.Fatalf("bảng tiến độ ghi %v, muốn %v", got, muon)
	}

	// The order the statements actually reached the driver, not merely what the result says.
	var thuTu []string
	g.mu.Lock()
	for _, l := range g.lenh {
		for _, ma := range []string{"0001", "0002", "0003"} {
			if strings.Contains(l.sql, "CREATE TABLE bang_"+ma) {
				thuTu = append(thuTu, ma)
			}
		}
	}
	g.mu.Unlock()
	if !bangNhau(thuTu, []string{"0001", "0002", "0003"}) {
		t.Fatalf("câu lệnh chạy theo thứ tự %v", thuTu)
	}
}

// --- exactly once --------------------------------------------------------------------------

func TestTepDaApThiLuotSauBoQua(t *testing.T) {
	// Re-running must apply nothing. `0002_audit_log_append_only.sql` drops and recreates a
	// trigger, so a second application is harmless there — but a future migration that inserts a
	// row is not, and the runner is what has to make that impossible.
	db, g := moDB(t, 1)

	if _, err := chay(t, db, nguonBa()); err != nil {
		t.Fatalf("lượt đầu: %v", err)
	}
	kq, err := chay(t, db, nguonBa())
	if err != nil {
		t.Fatalf("lượt hai: %v", err)
	}

	if len(kq.DaAp) != 0 {
		t.Errorf("lượt hai áp lại %v", kq.DaAp)
	}
	if muon := []string{"0001_init.sql", "0002_them.sql", "0003_them_2.sql"}; !bangNhau(kq.BoQua, muon) {
		t.Errorf("bỏ qua %v, muốn %v", kq.BoQua, muon)
	}
	for _, ma := range []string{"0001", "0002", "0003"} {
		if n := g.dem("CREATE TABLE bang_" + ma); n != 1 {
			t.Errorf("tệp %s chạy %d lần, muốn đúng 1", ma, n)
		}
	}
	if n := len(g.tenDaAp()); n != 3 {
		t.Errorf("bảng tiến độ có %d dòng, muốn 3", n)
	}
}

func TestMoiTepMotGiaoDichCungVoiDongTienDo(t *testing.T) {
	// Rule 7 invariant 5 wants progress recorded; recording it OUTSIDE the transaction records a
	// progress that may not have happened. A crash in the gap leaves the schema changed and the
	// table saying it was not — or the reverse.
	db, g := moDB(t, 1)

	if _, err := chay(t, db, nguonBa()); err != nil {
		t.Fatal(err)
	}
	ten := map[string]string{"0001": "0001_init.sql", "0002": "0002_them.sql", "0003": "0003_them_2.sql"}
	for _, ma := range []string{"0001", "0002", "0003"} {
		ddl := g.tim("CREATE TABLE bang_" + ma)
		if ddl == nil {
			t.Fatalf("không thấy câu lệnh của %s", ma)
		}
		if ddl.tx == 0 {
			t.Fatalf("%s chạy NGOÀI giao dịch", ma)
		}
		if got := g.ketThucCua(ddl.tx); got != "commit" {
			t.Fatalf("giao dịch của %s kết thúc bằng %q, muốn commit", ma, got)
		}

		// The progress row must be in THAT transaction, not merely in some transaction.
		ghi := g.timTheoThamSo("INSERT INTO "+BangTienDo, ten[ma])
		if ghi == nil {
			t.Fatalf("không thấy dòng tiến độ của %s", ten[ma])
		}
		if ghi.tx != ddl.tx {
			t.Fatalf("dòng tiến độ của %s ở giao dịch %d, câu lệnh ở giao dịch %d — phải cùng một giao dịch",
				ten[ma], ghi.tx, ddl.tx)
		}
	}
	// Each file gets its OWN transaction: one giant transaction over sixteen files means a
	// failure in the last one undoes fifteen that were fine.
	g.mu.Lock()
	soTx := g.soTx
	g.mu.Unlock()
	if soTx != 3 {
		t.Errorf("mở %d giao dịch cho 3 tệp, muốn 3", soTx)
	}
}

// --- an applied file that changed ------------------------------------------------------------

func TestChecksumDoiThiDungVaKhongApTepNaoSauDo(t *testing.T) {
	// Editing an applied migration leaves two databases both claiming version 0002 with two
	// different schemas, and nothing afterwards reports the difference. Stopping is the only
	// answer: continuing would apply 0002 to a database whose 0001 is not this 0001.
	db, g := moDB(t, 1)
	g.datTienDo(hangTienDo{ten: "0001_init.sql", checksum: bam("nội dung CŨ của 0001")})

	kq, err := chay(t, db, nguonBa())
	if !errors.Is(err, ErrChecksumLech) {
		t.Fatalf("lỗi = %v, muốn ErrChecksumLech", err)
	}
	if !strings.Contains(err.Error(), "0001_init.sql") {
		t.Errorf("thông báo không nêu tên tệp: %v", err)
	}
	if len(kq.DaAp) != 0 {
		t.Errorf("vẫn áp %v dù đã phát hiện lệch", kq.DaAp)
	}
	// The files AFTER the mismatched one are the ones that must not slip through.
	for _, ma := range []string{"0002", "0003"} {
		if n := g.dem("CREATE TABLE bang_" + ma); n != 0 {
			t.Errorf("tệp %s vẫn chạy %d lần sau khi phát hiện checksum lệch", ma, n)
		}
	}
	if got := g.tenDaAp(); len(got) != 1 {
		t.Errorf("bảng tiến độ = %v, muốn giữ nguyên một dòng", got)
	}
}

func TestCoSoDuLieuMoiHonMaNguonThiDung(t *testing.T) {
	// The database has applied a file this binary does not carry: the schema is AHEAD of the
	// code. Fail closed — the code would otherwise query tables whose shape it does not know.
	db, g := moDB(t, 1)
	g.datTienDo(hangTienDo{ten: "0009_tuong_lai.sql", checksum: bam("bất kỳ")})

	_, err := chay(t, db, nguonBa())
	if !errors.Is(err, ErrThieuTep) {
		t.Fatalf("lỗi = %v, muốn ErrThieuTep", err)
	}
	if !strings.Contains(err.Error(), "0009_tuong_lai.sql") {
		t.Errorf("thông báo không nêu tên tệp: %v", err)
	}
	if n := g.dem("CREATE TABLE bang_"); n != 0 {
		t.Errorf("vẫn chạy %d câu lệnh migration", n)
	}
}

// --- a file that fails half-way ----------------------------------------------------------------

func TestTepHongThiRollbackVaKhongCoDongTienDo(t *testing.T) {
	// The failure this rules out: a progress row for a migration that did not finish. The next
	// start would then skip it, and the schema would be missing a change forever, silently.
	db, g := moDB(t, 1)
	g.loiTheo["CREATE TABLE bang_0002"] = errors.New("cú pháp hỏng")

	kq, err := chay(t, db, nguonBa())
	if err == nil {
		t.Fatal("migration hỏng mà vẫn báo thành công")
	}
	if !strings.Contains(err.Error(), "0002_them.sql") {
		t.Errorf("thông báo không nêu tên tệp hỏng: %v", err)
	}
	if !bangNhau(kq.DaAp, []string{"0001_init.sql"}) {
		t.Errorf("DaAp = %v, muốn chỉ 0001", kq.DaAp)
	}

	hong := g.tim("CREATE TABLE bang_0002")
	if hong == nil {
		t.Fatal("không có câu lệnh hỏng để kiểm")
	}
	if got := g.ketThucCua(hong.tx); got != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback", got)
	}
	if got := g.tenDaAp(); !bangNhau(got, []string{"0001_init.sql"}) {
		t.Fatalf("bảng tiến độ = %v — 0002 hỏng không được để lại dòng nào", got)
	}
	// Stop at the first failure: 0003 on a database that never got 0002 is a schema nobody has.
	if n := g.dem("CREATE TABLE bang_0003"); n != 0 {
		t.Errorf("vẫn chạy tệp sau tệp hỏng %d lần", n)
	}
}

func TestMigrationHongVanNhaKhoa(t *testing.T) {
	// A lock kept after a failed start blocks every other replica, and the symptom is a service
	// that hangs with no error anywhere — the hardest kind of outage to diagnose.
	db, g := moDB(t, 1)
	g.loiTheo["CREATE TABLE bang_0001"] = errors.New("cú pháp hỏng")

	if _, err := chay(t, db, nguonBa()); err == nil {
		t.Fatal("muốn lỗi")
	}
	g.mu.Lock()
	khoa, nha, thua := g.soLanKhoa, g.soLanNha, g.nhaKhiChuaKhoa
	g.mu.Unlock()
	if khoa != 1 || nha != 1 {
		t.Fatalf("lấy khoá %d lần, nhả %d lần — phải bằng nhau", khoa, nha)
	}
	if thua != 0 {
		t.Errorf("nhả khoá %d lần mà chưa từng lấy", thua)
	}
}

// --- two replicas starting at once ---------------------------------------------------------

func TestHaiLuotChayDongThoiChiMotLuotAp(t *testing.T) {
	// Two replicas deployed together start within milliseconds of each other. Without the lock
	// both read an empty progress table and both apply the same file; the second one fails on a
	// constraint that already exists, and the deploy is half up.
	db, g := moDB(t, 4)
	nguon := nguonBa()

	var wg sync.WaitGroup
	kq := make([]KetQua, 2)
	loi := make([]error, 2)
	for i := range kq {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			kq[i], loi[i] = Chay(context.Background(), db, nguon, "identity")
		}(i)
	}
	wg.Wait()

	for i, err := range loi {
		if err != nil {
			t.Fatalf("lượt %d lỗi: %v", i, err)
		}
	}
	soAp := 0
	for _, k := range kq {
		if len(k.DaAp) > 0 {
			soAp++
			if len(k.DaAp) != 3 {
				t.Errorf("một lượt áp %v — phải áp cả ba hoặc không áp gì", k.DaAp)
			}
		}
	}
	if soAp != 1 {
		t.Fatalf("%d/2 lượt áp migration, muốn đúng 1", soAp)
	}
	for _, ma := range []string{"0001", "0002", "0003"} {
		if n := g.dem("CREATE TABLE bang_" + ma); n != 1 {
			t.Errorf("tệp %s chạy %d lần, muốn đúng 1", ma, n)
		}
	}
	if n := len(g.tenDaAp()); n != 3 {
		t.Errorf("bảng tiến độ có %d dòng, muốn 3", n)
	}
}

func TestKhoaDuocLayTruocKhiChamVaoSchema(t *testing.T) {
	db, g := moDB(t, 1)
	if _, err := chay(t, db, nguonBa()); err != nil {
		t.Fatal(err)
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	var iKhoa, iDauTien, iNha = -1, -1, -1
	for i, l := range g.lenh {
		switch {
		case strings.Contains(l.sql, "pg_advisory_unlock"):
			iNha = i
		case strings.Contains(l.sql, "pg_advisory_lock"):
			iKhoa = i
		case strings.Contains(l.sql, "CREATE TABLE bang_0001") && iDauTien < 0:
			iDauTien = i
		}
	}
	if iKhoa < 0 || iNha < 0 {
		t.Fatalf("không thấy lấy/nhả khoá: khoá=%d nhả=%d", iKhoa, iNha)
	}
	if iKhoa > iDauTien {
		t.Error("lấy khoá SAU khi đã chạy migration — cửa sổ hai bản sao cùng áp vẫn mở")
	}
	if iNha < iDauTien {
		t.Error("nhả khoá trước khi chạy xong — khoá không bao cả lượt chạy")
	}
}

// --- degenerate cases ---------------------------------------------------------------------

func TestKhongCoMigrationThiKhongLoi(t *testing.T) {
	// A service that has not written its first migration yet must still start.
	db, g := moDB(t, 1)

	kq, err := chay(t, db, fstest.MapFS{})
	if err != nil {
		t.Fatalf("danh sách rỗng mà báo lỗi: %v", err)
	}
	if len(kq.DaAp) != 0 || len(kq.BoQua) != 0 {
		t.Errorf("KetQua = %+v, muốn rỗng", kq)
	}
	g.mu.Lock()
	soTx := g.soTx
	g.mu.Unlock()
	if soTx != 0 {
		t.Errorf("mở %d giao dịch cho 0 tệp", soTx)
	}
}

func TestChiDocTepSQL(t *testing.T) {
	db, g := moDB(t, 1)
	nguon := fstest.MapFS{
		"0001_init.sql": {Data: []byte(than("0001"))},
		"README.txt":    {Data: []byte("không phải migration")},
		"embed.go":      {Data: []byte("package migrations")},
	}
	kq, err := chay(t, db, nguon)
	if err != nil {
		t.Fatal(err)
	}
	if !bangNhau(kq.DaAp, []string{"0001_init.sql"}) {
		t.Fatalf("DaAp = %v, muốn chỉ tệp .sql", kq.DaAp)
	}
	if g.dem("package migrations") != 0 {
		t.Error("đã gửi tệp không phải .sql xuống cơ sở dữ liệu")
	}
}

func TestThieuTenDichVuThiTuChoi(t *testing.T) {
	// No default on the lock key: two services guessing the same key would each believe they
	// hold the migration lock alone.
	db, _ := moDB(t, 1)
	if _, err := Chay(context.Background(), db, nguonBa(), "  "); err == nil {
		t.Fatal("thiếu tên dịch vụ mà vẫn chạy")
	}
}

func TestKhoaKhacNhauTheoDichVu(t *testing.T) {
	// Deterministic across processes — two replicas must compute the same number without talking
	// to each other — and different between services, so identity does not queue behind finance.
	// PINNED VALUES, not two calls compared to each other. Calling a pure function twice in one
	// expression is an equality that cannot fail, so it tested nothing — and the stability that
	// matters is not within one process but ACROSS processes and across binary versions. If the
	// hash ever changes, an old replica and a new one take DIFFERENT locks and both migrate at
	// once, which is the exact failure the lock exists to prevent. Only a pinned constant
	// catches that. Computed independently (fnv-1a of "vigov.migrate:<service>", as int64).
	for ten, muon := range map[string]int64{
		"identity": -4371719473242908139,
		"platform": -7775282374755666188,
		"finance":  4727339668149605191,
	} {
		if got := khoaTuTen(ten); got != muon {
			t.Fatalf("khoá của %q đổi: %d, muốn %d — hai phiên bản binary sẽ lấy hai khoá khác nhau "+
				"và cùng migrate một lúc", ten, got, muon)
		}
	}
	if khoaTuTen("identity") == khoaTuTen("finance") {
		t.Fatal("hai dịch vụ dùng chung khoá — mỗi lần deploy là một hàng đợi")
	}
}

// --- helpers --------------------------------------------------------------------------------

func bangNhau(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
