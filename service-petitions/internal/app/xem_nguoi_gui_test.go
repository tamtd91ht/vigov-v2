package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/migrate"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/migrations"
)

// What this file proves, and what it does not — stated first, because a package that prints `ok`
// while asserting nothing is this repository's worst known trap.
//
//	PROVED HERE   the entry reaches the database as an INSERT INTO audit_log · it runs INSIDE a
//	              transaction and that transaction is COMMITTED · the commune bound into it comes
//	              from the CONTEXT and never from an argument · the six things rule 6 invariant 2
//	              requires are all in the bound arguments · the delta names COLUMNS and carries no
//	              personal value · a driver failure rolls back, is wrapped with %w, and carries
//	              neither the lookup code nor the actor into the message.
//
//	NOT PROVED    ANYTHING PostgreSQL DOES WITH THE STATEMENT. The fake accepts any SQL, so the
//	              append-only trigger of 0002_audit_log_append_only.sql, the hash partition routing
//	              and the column types are invisible here. TestPgVetXemDayDuDocLaiDuocTuBang below
//	              is what reads the row back out of the real table, and it SKIPS without
//	              VIGOV_TEST_DSN.

// --- a fake driver that has transactions ---------------------------------------------------------
//
// The catalogue suites in internal/store share a fake driver whose Begin deliberately fails: those
// stores only read. This one exists because the ONE thing under test here is a write that must be
// inside a transaction, so a driver without transactions could not fail for the right reason.

type lenhGia struct {
	sql  string
	args []driver.Value
	// trongGiaoDich records whether a transaction was open when the statement ran. It is the whole
	// point of the fake: an audit write outside a transaction is rule 6, forbidden #2, and it looks
	// identical to a correct one in every other respect.
	trongGiaoDich bool
}

type khoGia struct {
	lenh     []lenhGia
	dangMo   bool
	commit   int
	rollback int
	loi      error
}

func (k *khoGia) Connect(context.Context) (driver.Conn, error) { return &connGia{k: k}, nil }
func (k *khoGia) Driver() driver.Driver                        { return trinhGia{} }

type trinhGia struct{}

func (trinhGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type connGia struct{ k *khoGia }

func (c *connGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connGia) Close() error { return nil }

func (c *connGia) Begin() (driver.Tx, error) {
	c.k.dangMo = true
	return &txGia{k: c.k}, nil
}

func (c *connGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.k.lenh = append(c.k.lenh, lenhGia{sql: q, args: gt, trongGiaoDich: c.k.dangMo})
	if c.k.loi != nil {
		return nil, c.k.loi
	}
	return driver.RowsAffected(1), nil
}

type txGia struct{ k *khoGia }

func (t *txGia) Commit() error {
	t.k.commit++
	t.k.dangMo = false
	return nil
}

func (t *txGia) Rollback() error {
	t.k.rollback++
	t.k.dangMo = false
	return nil
}

// --- fixtures -------------------------------------------------------------------------------------

var (
	xaThu = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaKia = tenant.ID("01JB" + strings.Repeat("B", 22))
)

// No personal data in a fixture (rule 3, invariant 5). The actor is an internal staff id, the
// subject is a lookup code of the unguessable shape SinhMaTraCuu really produces.
const (
	maThu   = "PA-4K7M-92XR-BTVD"
	idCanBo = "nd-01JCANBONOIBOCUAXA"
)

func ctxXa(xa tenant.ID) context.Context { return tenant.Into(context.Background(), xa) }

func nguoiThu() audit.Actor {
	return audit.Actor{ID: idCanBo, Kind: "staff", IP: "10.0.0.7"}
}

func dungUC(k *khoGia) *XemNguoiGui { return NewXemNguoiGui(pkgstore.New(sql.OpenDB(k))) }

// --- the trail is a real row, in a real transaction --------------------------------------------

func TestGhiVetChayTrongGiaoDichVaDuocCommit(t *testing.T) {
	k := &khoGia{}

	if err := dungUC(k).GhiVet(ctxXa(xaThu), maThu, nguoiThu()); err != nil {
		t.Fatalf("GhiVet: %v", err)
	}

	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]

	if !strings.Contains(l.sql, "INSERT INTO audit_log") {
		t.Errorf("không ghi vào bảng vết: %q", l.sql)
	}
	// RULE 6, FORBIDDEN #1: the trail is business data in durable storage, never a log line. This
	// assertion is the difference between the two, and it is the reason this test reads the
	// STATEMENT rather than the logger.
	if !l.trongGiaoDich {
		t.Error("ghi vết NGOÀI giao dịch — luật 6 bất biến 3 và cấm #2")
	}
	if k.commit != 1 {
		t.Errorf("commit %d lần, muốn 1 — một vết chưa commit là một vết không tồn tại", k.commit)
	}
	if k.rollback != 0 {
		t.Errorf("rollback %d lần trên đường thành công", k.rollback)
	}
}

// TestGhiVetMangDuSauThuocCuaLuat6 checks the bound arguments one by one.
//
// BY POSITION, against the statement audit.Write builds:
//
//	$1 tenant_id · $2 actor_id · $3 actor_kind · $4 actor_ip · $5 action · $6 subject · $7 at · $8 delta
func TestGhiVetMangDuSauThuocCuaLuat6(t *testing.T) {
	k := &khoGia{}
	truoc := time.Now().UTC()

	if err := dungUC(k).GhiVet(ctxXa(xaThu), maThu, nguoiThu()); err != nil {
		t.Fatalf("GhiVet: %v", err)
	}
	args := k.lenh[0].args
	if len(args) != 8 {
		t.Fatalf("%d tham số, muốn 8: %v", len(args), args)
	}

	// TRONG XÃ NÀO — and it came from the context. GhiVet takes no commune parameter and cannot:
	// a commune that can be passed in is a commune a caller can get wrong (rule 1, invariant 4).
	if args[0] != string(xaThu) {
		t.Errorf("tenant_id = %v, muốn %q", args[0], xaThu)
	}
	// AI
	if args[1] != idCanBo {
		t.Errorf("actor_id = %v, muốn %q", args[1], idCanBo)
	}
	if args[2] != "staff" {
		t.Errorf("actor_kind = %v, muốn %q", args[2], "staff")
	}
	// TỪ IP NÀO
	if args[3] != "10.0.0.7" {
		t.Errorf("actor_ip = %v, muốn %q", args[3], "10.0.0.7")
	}
	// LÀM GÌ — a business verb, not a function name.
	if args[4] != HanhViXemDayDu {
		t.Errorf("action = %v, muốn %q", args[4], HanhViXemDayDu)
	}
	// TRÊN BẢN GHI NÀO — the business code the citizen holds, not the internal ULID.
	if args[5] != maThu {
		t.Errorf("subject = %v, muốn %q", args[5], maThu)
	}
	// KHI NÀO
	luc, ok := args[6].(time.Time)
	if !ok {
		t.Fatalf("at = %T, muốn time.Time", args[6])
	}
	if luc.Before(truoc.Add(-time.Minute)) || luc.After(time.Now().UTC().Add(time.Minute)) {
		t.Errorf("at = %v, không nằm quanh lúc chạy", luc)
	}
}

// TestGhiVetDeltaChiNoiTENCOT, không nói giá trị.
//
// Rule 6, forbidden #4: keeping the before/after of a personal-data field turns the ledger itself
// into a store of personal data — append-only, never deleted, which is the worst possible place for
// it to accumulate. The entry has to answer "what was seen", and the column names answer it.
func TestGhiVetDeltaChiNoiTenCot(t *testing.T) {
	k := &khoGia{}

	if err := dungUC(k).GhiVet(ctxXa(xaThu), maThu, nguoiThu()); err != nil {
		t.Fatalf("GhiVet: %v", err)
	}

	tho, ok := k.lenh[0].args[7].([]byte)
	if !ok {
		t.Fatalf("delta = %T, muốn []byte", k.lenh[0].args[7])
	}
	var d struct {
		TruongDaMo []string `json:"truong_da_mo"`
		Quyen      string   `json:"quyen"`
	}
	if err := json.Unmarshal(tho, &d); err != nil {
		t.Fatalf("delta không phải JSON: %q", string(tho))
	}
	if len(d.TruongDaMo) != 2 || d.TruongDaMo[0] != "nguoi_gui_ho_ten" || d.TruongDaMo[1] != "nguoi_gui_dien_thoai" {
		t.Errorf("truong_da_mo = %v — vết phải nói RÕ hai trường nào đã mở", d.TruongDaMo)
	}
	// THE KEY IS IN THE ENTRY because an inspection asks under which authority the number was read,
	// and a commune may later withdraw that tick box — after which the grant is gone and only this
	// row still says it was there.
	if d.Quyen != "feedback.unmask" {
		t.Errorf("quyen = %q, muốn %q", d.Quyen, "feedback.unmask")
	}
}

// TestGhiVetTheoXaTrongContext is the isolation case: the SAME use case object, two communes, two
// entries, each bound to the commune of its own request. A use case that cached the commune from
// the first call would pass every test above and attribute one commune's disclosure to another.
func TestGhiVetTheoXaTrongContext(t *testing.T) {
	k := &khoGia{}
	uc := dungUC(k)

	for _, xa := range []tenant.ID{xaThu, xaKia} {
		if err := uc.GhiVet(ctxXa(xa), maThu, nguoiThu()); err != nil {
			t.Fatalf("GhiVet cho %s: %v", xa, err)
		}
	}
	if len(k.lenh) != 2 {
		t.Fatalf("chạy %d câu lệnh, muốn 2", len(k.lenh))
	}
	if k.lenh[0].args[0] != string(xaThu) || k.lenh[1].args[0] != string(xaKia) {
		t.Errorf("tenant_id của hai vết = %v, %v — muốn %q rồi %q",
			k.lenh[0].args[0], k.lenh[1].args[0], xaThu, xaKia)
	}
}

// --- failure fails closed, and says nothing ------------------------------------------------------

func TestGhiVetKhoHongThiRollbackVaBocLoi(t *testing.T) {
	k := &khoGia{loi: errors.New("pg: relation audit_log is read only")}

	err := dungUC(k).GhiVet(ctxXa(xaThu), maThu, nguoiThu())

	if err == nil {
		t.Fatal("GhiVet nuốt lỗi — người gọi sẽ mở dữ liệu cá nhân mà không có vết")
	}
	if k.commit != 0 {
		t.Errorf("commit %d lần dù ghi hỏng", k.commit)
	}
	if k.rollback != 1 {
		t.Errorf("rollback %d lần, muốn 1", k.rollback)
	}
	// Rule 6, forbidden #6 of the working rules: wrapped with %w, never swallowed.
	if !errors.Is(err, k.loi) {
		t.Errorf("lỗi không bọc lỗi gốc bằng %%w: %v", err)
	}
	// RULE 3: the error travels into centralised logging. `ma_tra_cuu` is the one string that opens
	// a citizen's petition, and the actor id names a member of staff.
	if strings.Contains(err.Error(), maThu) || strings.Contains(err.Error(), idCanBo) {
		t.Errorf("thông điệp lỗi mang mã tra cứu hoặc người gây: %v", err)
	}
	// The commune IS allowed and IS wanted: it is not personal data, and it is what an operator
	// needs to find the database that refused.
	if !strings.Contains(err.Error(), string(xaThu)) {
		t.Errorf("thông điệp lỗi không nói xã nào: %v", err)
	}
}

func TestGhiVetThieuMaTraCuuThiTuChoi(t *testing.T) {
	k := &khoGia{}

	if err := dungUC(k).GhiVet(ctxXa(xaThu), "", nguoiThu()); err == nil {
		t.Fatal("ghi được vết không trỏ bản ghi nào")
	}
	if len(k.lenh) != 0 {
		t.Errorf("chạm cơ sở dữ liệu %d lần dù đã từ chối", len(k.lenh))
	}
}

// TestGhiVetKhongCoXaThiPanicChuKhongMacDinh: no commune in the context is a programming fault at
// the edge, and store.For panics rather than defaulting. A default on the isolation path is rule 1,
// forbidden #1 — one line that collapses the isolation silently, with tests still green.
func TestGhiVetKhongCoXaThiPanicChuKhongMacDinh(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("ghi được vết mà không có xã trong context")
		}
	}()
	k := &khoGia{}
	_ = dungUC(k).GhiVet(context.Background(), maThu, nguoiThu())
}

// --- the half a fake driver cannot reach ----------------------------------------------------------

// The schema is built ONCE for the whole package. THE HARNESS IS A SECOND COPY of the one in
// internal/store and that is not an oversight: Go test helpers do not cross a package boundary, and
// the alternative — an exported testing package whose only purpose is to be imported by tests — is a
// production symbol that exists for tests. The copy is 40 lines of harness and asserts nothing, so
// the two cannot drift in a way that makes a test wrong; they can only drift in a way that makes one
// of them fail to start, loudly.
var dbChung *sql.DB

func TestMain(m *testing.M) {
	// os.Getenv IN A _test.go FILE. Rule 11, invariant 1 binds the SERVICE to core/config; a test
	// harness is not the service, and this is the same line internal/store's harness uses. The DSN
	// carries a password and lives only in the environment (rule 8).
	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		os.Exit(m.Run()) // the pg test skips itself; the fake-driver tests still run
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mở kết nối:", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schema := fmt.Sprintf("vigov_app_test_%d", time.Now().UnixNano())
	// ONE physical connection: `SET search_path` is SESSION state, so on a pool it applies to
	// whichever connection served that statement and to no other.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		fmt.Fprintln(os.Stderr, "tạo schema:", err)
		os.Exit(1)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		fmt.Fprintln(os.Stderr, "đặt search_path:", err)
		os.Exit(1)
	}
	if _, err := migrate.Chay(ctx, db, migrations.FS, "petitions"); err != nil {
		fmt.Fprintln(os.Stderr, "chạy migration:", err)
		os.Exit(1)
	}

	dbChung = db
	ma := m.Run()

	// A schema this suite created, in a test database. Rule 7 protects archival business data; this
	// holds none.
	if _, err := db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
		fmt.Fprintln(os.Stderr, "dọn schema:", err)
	}
	db.Close()
	os.Exit(ma)
}

// TestPgVetXemDayDuDocLaiDuocTuBang reads the entry BACK OUT of `audit_log`.
//
// READ THIS BEFORE BELIEVING A GREEN RUN: it SKIPS without VIGOV_TEST_DSN and the package still
// prints `ok`. It is the only assertion here that touches what the fake driver cannot see — that
// `audit_log` really has the columns audit.Write names, that the row survives the commit, and that
// it can be found again by commune and subject, which is exactly what an inspection does.
func TestPgVetXemDayDuDocLaiDuocTuBang(t *testing.T) {
	if dbChung == nil {
		t.Skip("VIGOV_TEST_DSN chưa đặt — bỏ qua phép kiểm cần PostgreSQL thật")
	}

	// A commune id of this test's own, so the row it looks for cannot be another test's.
	xa := tenant.ID("01JT" + strings.Repeat("T", 22))
	if err := NewXemNguoiGui(pkgstore.New(dbChung)).GhiVet(ctxXa(xa), maThu, nguoiThu()); err != nil {
		t.Fatalf("GhiVet: %v", err)
	}

	var (
		actorID, actorKind, actorIP string
		delta                       []byte
	)
	err := dbChung.QueryRow(
		`SELECT actor_id, actor_kind, actor_ip, delta
		   FROM audit_log WHERE tenant_id = $1 AND subject = $2 AND action = $3`,
		string(xa), maThu, HanhViXemDayDu).
		Scan(&actorID, &actorKind, &actorIP, &delta)
	if err != nil {
		t.Fatalf("đọc lại vết từ bảng audit_log: %v", err)
	}
	if actorID != idCanBo || actorKind != "staff" || actorIP != "10.0.0.7" {
		t.Errorf("vết đọc lại: actor = %q / %q / %q", actorID, actorKind, actorIP)
	}
	if !strings.Contains(string(delta), "nguoi_gui_dien_thoai") {
		t.Errorf("delta đọc lại không nói trường nào đã mở: %s", delta)
	}
}
