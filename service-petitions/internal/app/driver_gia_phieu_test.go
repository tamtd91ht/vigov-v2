package app

// A THIRD fake database/sql driver, for the STAFF processing path of the petition register.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument
// driver_gia_loai_nhiem_vu_test.go makes, and it is heavier here because what is being protected is a
// COMMITMENT MADE TO A CITIZEN:
//
//	the change, the audit entry and the outbox row are in ONE transaction  (rule 6 inv. 3; rule 10 inv. 5)
//	a refusal leaves the transaction with NOTHING committed
//	the petition is read `FOR UPDATE`, so two officers cannot both classify it
//	the deadline identity computed is what reaches the UPDATE, unchanged
//	no UPDATE on this path touches `han_tiep_nhan`, `han_phan_loai`, `goc_dem_han` or `ma_tra_cuu`
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// A THIRD DRIVER AND NOT AN EXTENSION OF THE OTHER TWO: khoGia (xem_nguoi_gui_test.go) has no
// QueryContext at all, and khoLoaiNhiemVuGia answers a catalogue row shape. Teaching either one the
// petition register would make every case in the other file depend on a branch written for this one.
//
// # WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there
//
// Nothing PostgreSQL does with these statements. Not the CHECK constraints of migrations 0004 and
// 0005, not the `ho_so_luu_tru_bat_bien` trigger, not the hash partition routing, not
// `UNIQUE (tenant_id, ma_tra_cuu)`. That half needs a real server (VIGOV_TEST_DSN is unset here), and
// it is the FLOOR under everything asserted in xu_ly_phan_anh_test.go — the application refusals
// tested there are the SENTENCE, not the enforcement.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// lenhPhieu is one statement the driver saw, and whether a transaction was open at the time.
//
// `trongGiaoDich` IS THE FIELD THE WHOLE FILE EXISTS FOR. A business write outside a transaction and
// one inside it are indistinguishable in every other respect, and rule 6, forbidden #2 is precisely
// the difference.
type lenhPhieu struct {
	sql           string
	args          []driver.Value
	trongGiaoDich bool
}

// khoPhieuXuLyGia serves ONE petition row, by name, from the column list the store itself asked for.
//
// BY NAME AND NOT BY POSITION: the row is assembled from the SELECT list the store wrote, so
// reordering cotPhieu without reordering the matching Scan comes back as WRONG DATA rather than as a
// plausible zero — and the columns most likely to be reordered are the three adjacent deadline
// columns whose NULLs mean two opposite things.
type khoPhieuXuLyGia struct {
	mu   sync.Mutex
	lenh []lenhPhieu

	dangMo                       bool
	batDau, daCommit, daRollback int

	// hang is the petition the FOR UPDATE read hands back. nil means "no such live petition".
	hang map[string]driver.Value

	// doiDong is what every UPDATE reports as rows affected. ZERO IS A REAL CASE: it is what the
	// database answers when another officer moved the petition between the locking read and the
	// write, and it is what the store turns into ErrPhieuDaChuyenTrang.
	doiDong int64

	loi error

	// loiSau fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET `loi`: failing everything cannot tell "rolled back" from "never started". The
	// invariant worth proving is that a failure on the LAST statement of the transaction — the outbox
	// row — takes the business write AND the audit entry down with it.
	loiSau string
	daNo   bool
}

func khoPhieuMau() *khoPhieuXuLyGia {
	return &khoPhieuXuLyGia{doiDong: 1, hang: dongPhieuMau(nil)}
}

// dongPhieuMau is one petition as the driver hands it back: a citizen-filed report that has been
// received and NOT YET CLASSIFIED — the state every act in this file starts from or moves past.
//
// EVERY INSTANT IS DISTINCT, so a Scan wired to the wrong position comes back as visibly wrong data.
// `han_xu_ly_xong` IS NIL, which is the "CHƯA CÓ" NULL; `han_tiep_nhan` and `han_phan_loai` are both
// set, which is the shape ADR 0028 and ADR 0035 §C produce together at intake.
// THE OVERRIDE MAP IS `map[string]any` AND THE RESULT IS `map[string]driver.Value`, which are two
// distinct Go types even though driver.Value is an alias for any. Written this way so a case can put
// a plain literal in the override without naming the driver package at every row.
func dongPhieuMau(sua map[string]any) map[string]driver.Value {
	d := map[string]driver.Value{
		"id":                   idPhieuThu,
		"ma_tra_cuu":           maPhieuThu,
		"kenh_tiep_nhan":       "zalo-mini-app",
		"cong_dan_id":          idCongDanThu,
		"noi_dung":             "Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
		"linh_vuc":             nil,
		"dia_chi":              "Đầu ngõ thôn Hà Lam",
		"thon_id":              "thon-001",
		"lat":                  nil,
		"lng":                  nil,
		"nguoi_gui_ho_ten":     "Nguyễn Văn An",
		"nguoi_gui_dien_thoai": "0900000000", // the agreed fake number (rule 3, invariant 5)
		"an_danh":              false,
		"trang_thai":           string(domain.DaTiepNhan),
		"bo_phan_id":           nil,
		"can_bo_xu_ly_id":      nil,
		"goc_dem_han":          mocGocThu,
		"vao_so_luc":           mocGocThu,
		"han_tiep_nhan":        mocTiepNhanThu,
		"han_xu_ly_xong":       nil,
		"han_phan_loai":        mocTranPhanLoaiThu,
		"phan_loai_luc":        nil,
		"xu_ly_xong_luc":       nil,
		"dong_luc":             nil,
		"ket_qua_xu_ly":        nil,
		"hien_cong_khai":       false,
		"so_lan_mo_lai":        int64(0),
		"tao_luc":              mocGocThu,
	}
	for k, v := range sua {
		d[k] = v
	}
	return d
}

func (k *khoPhieuXuLyGia) ghi(q string, args []driver.NamedValue) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	k.mu.Lock()
	k.lenh = append(k.lenh, lenhPhieu{sql: q, args: gt, trongGiaoDich: k.dangMo})
	k.mu.Unlock()
}

// cau returns every recorded statement containing `tu`, so an assertion names the statement it cares
// about rather than an index that shifts when a check is added.
func (k *khoPhieuXuLyGia) cau(tu string) []lenhPhieu {
	k.mu.Lock()
	defer k.mu.Unlock()
	var ra []lenhPhieu
	for _, l := range k.lenh {
		if strings.Contains(l.sql, tu) {
			ra = append(ra, l)
		}
	}
	return ra
}

func (k *khoPhieuXuLyGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

func (k *khoPhieuXuLyGia) kiemLoi(q string) error {
	if k.loi != nil {
		return k.loi
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.loiSau != "" && !k.daNo && strings.Contains(q, k.loiSau) {
		k.daNo = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *khoPhieuXuLyGia) Connect(context.Context) (driver.Conn, error) {
	return &connPhieuGia{k: k}, nil
}
func (k *khoPhieuXuLyGia) Driver() driver.Driver { return trinhPhieuGia{} }

type trinhPhieuGia struct{}

func (trinhPhieuGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type connPhieuGia struct{ k *khoPhieuXuLyGia }

func (c *connPhieuGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connPhieuGia) Close() error { return nil }

func (c *connPhieuGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connPhieuGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.dangMo = true
	c.k.mu.Unlock()
	return &txPhieuGia{k: c.k}, nil
}

func (c *connPhieuGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (
	driver.Result, error) {

	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	if strings.Contains(q, "UPDATE phieu_phan_anh") {
		// THE ONLY STATEMENT WHOSE ROW COUNT MEANS ANYTHING. The store turns zero into
		// ErrPhieuDaChuyenTrang, which is the race between two officers, so the count has to be
		// settable — a fake that always answered 1 would make that case untestable.
		return driver.RowsAffected(c.k.doiDong), nil
	}
	return driver.RowsAffected(1), nil
}

func (c *connPhieuGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (
	driver.Rows, error) {

	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	if !strings.Contains(q, "FROM phieu_phan_anh") {
		return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
	}

	cot, err := cotTrongSelect(q)
	if err != nil {
		return nil, err
	}
	if c.k.hang == nil {
		return &rowsPhieuGia{cot: cot}, nil
	}
	mot := make([]driver.Value, len(cot))
	for i, ten := range cot {
		v, co := c.k.hang[ten]
		if !co {
			// A column was added to cotPhieu and not to the fixture. Failing loudly beats scanning a
			// nil that "passes" while proving nothing — and on this table a silently nil deadline is a
			// commitment that reads as "không áp dụng".
			panic("driver giả: không có giá trị mẫu cho cột " + ten)
		}
		mot[i] = v
	}
	return &rowsPhieuGia{cot: cot, hang: [][]driver.Value{mot}}, nil
}

// cotTrongSelect reads the SELECT list out of the statement. THIS IS WHAT MAKES THE COLUMN CHECK
// WORK: the row is assembled by NAME from the columns the store asked for, so reordering cotPhieu
// without reordering the matching Scan turns this suite red.
func cotTrongSelect(q string) ([]string, error) {
	i := strings.Index(q, "SELECT ")
	j := strings.Index(q, " FROM ")
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("driver giả: không đọc được danh sách cột từ %q", q)
	}
	var ra []string
	for _, c := range strings.Split(q[i+len("SELECT "):j], ",") {
		ra = append(ra, strings.TrimSpace(c))
	}
	return ra, nil
}

type txPhieuGia struct{ k *khoPhieuXuLyGia }

func (t *txPhieuGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.dangMo = false
	t.k.mu.Unlock()
	return nil
}

func (t *txPhieuGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.dangMo = false
	t.k.mu.Unlock()
	return nil
}

type rowsPhieuGia struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsPhieuGia) Columns() []string { return r.cot }
func (r *rowsPhieuGia) Close() error      { return nil }
func (r *rowsPhieuGia) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

// --- fixtures -------------------------------------------------------------------------------------

const (
	idPhieuThu   = "01JPHIEUDANGXULYTRONGTEST"
	maPhieuThu   = "PA-9WDN-3HQK-72FM"
	idCongDanThu = "cd-01JCONGDANMODINHDANH"
	idSuKienThu  = "01JSUKIENDICODINHTEST0000"

	// A result of the shape rule 10, invariant 6 asks for: it says what was DONE, not that something
	// was done.
	ketQuaThat = "Đội vệ sinh đã thu gom toàn bộ rác tại đầu ngõ ngày 24/9 và dựng biển cấm đổ rác."
)

var (
	// The instants on the fixture petition. ALL DISTINCT AND NONE A ROUND OFFSET FROM ANOTHER: a
	// deadline in working hours lands wherever the commune's calendar puts it, and fixtures a round
	// number of hours apart would let a local `.Add(24 * time.Hour)` pass every assertion here.
	mocGocThu          = time.Date(2026, 9, 22, 7, 14, 3, 0, time.UTC)
	mocTiepNhanThu     = time.Date(2026, 9, 23, 2, 30, 0, 0, time.UTC)
	mocTranPhanLoaiThu = time.Date(2026, 9, 23, 9, 45, 0, 0, time.UTC)

	// mocXuLyXongThu is what identity answers when asked for the RESOLVE deadline of the field the
	// officer just settled. It is SEVEN DAYS AND A BIT after the origin, which no calendar-day
	// arithmetic in the use case could produce by accident.
	mocXuLyXongThu = time.Date(2026, 9, 30, 4, 20, 0, 0, time.UTC)

	// mocThaoTac is when the officer acted. Pinned, because it becomes `phan_loai_luc` — the instant
	// BOTH the acknowledge clock and the classification ceiling are measured against.
	mocThaoTac = time.Date(2026, 9, 22, 8, 5, 0, 0, time.UTC)
)

// hanXuLyGia is identity's answer to the RESOLVE question, and it RECORDS WHAT IT WAS ASKED. The
// three arguments are ADR 0028 decision E expressed as a call, so asserting on them is asserting on
// the decision.
//
// A SECOND FAKE ALONGSIDE hanGia, and not a reuse of it: that one implements HanTiepNhanDoc, which
// also carries TienGioLamViec. This one implements DocHanXuLyXong, which does not — and the fact that
// the classification path CANNOT reach the working-hours RPC is part of what is under test. A fake
// that offered both would let a future edit call it here with nothing turning red.
type hanXuLyGia struct {
	tra map[identityv1.DeadlineKind]time.Time
	loi error

	goi        int
	thayLoai   []identityv1.WorkKind
	thayLinh   []string
	thayTuLuc  []time.Time
	thayCanMoc [][]identityv1.DeadlineKind
}

func (h *hanXuLyGia) HanXuLy(_ context.Context, loai identityv1.WorkKind, linhVuc string,
	tuLuc time.Time, can []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error) {

	h.goi++
	h.thayLoai = append(h.thayLoai, loai)
	h.thayLinh = append(h.thayLinh, linhVuc)
	h.thayTuLuc = append(h.thayTuLuc, tuLuc)
	h.thayCanMoc = append(h.thayCanMoc, can)
	if h.loi != nil {
		return nil, h.loi
	}
	return h.tra, nil
}

func hanXuLyThu() *hanXuLyGia {
	return &hanXuLyGia{tra: map[identityv1.DeadlineKind]time.Time{
		identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG: mocXuLyXongThu,
	}}
}

// dungXuLy builds the REAL use case over the REAL store over the fake driver, with the id and the
// clock pinned so every assertion is about the decision and not about randomness.
func dungXuLy(t *testing.T, k *khoPhieuXuLyGia, han DocHanXuLyXong) (*XuLyPhanAnh, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and IN ORDER. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind all of them, exactly as cmd/server wires it: the transaction the use case
	// opens is the transaction the stores write in, and two handles would be two pools.
	kho := pkgstore.New(db)
	uc := NewXuLyPhanAnh(kho, petstore.NewPhieuPhanAnhStore(kho), petstore.NewSuKienDiStore(kho), han)
	uc.sinhID = func() (string, error) { return idSuKienThu, nil }
	uc.nay = func() time.Time { return mocThaoTac }
	return uc, ctxXa(xaThu)
}

// canBoThu is the acting officer. THE ID IS A BUSINESS CODE, never an internal ULID (rule 6,
// invariant 8) — the trail is read years later by somebody handling a complaint, and a ULID there
// names nobody.
func canBoThu() audit.Actor { return audit.Actor{ID: maCanBoThu, Kind: "staff", IP: "10.0.0.7"} }
