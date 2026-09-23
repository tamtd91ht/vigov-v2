package app

// A fake database/sql driver for the WRITE path of the task register.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument
// driver_gia_phieu_test.go makes, and here it is heavier still, because what is being protected is
// a set of rules that ONLY EXIST AS RELATIONSHIPS BETWEEN ROWS:
//
//	the change, the timeline row and the audit entry are in ONE transaction   (rule 6, invariant 3)
//	a refusal leaves the transaction with NOTHING committed
//	every row the rules decide against is read `FOR UPDATE`
//	the completion check walks the WHOLE TREE, not one level                  (ADR 0037 decision 4)
//	a cycle is found by walking UPWARD, across three rows                     (the fifth rule)
//	moving a deadline touches `han_xu_ly` and NEVER `han_ban_dau`             (§5.8, §11.3)
//
// Every one of those is a property of the SQL and of the transaction boundaries. A fake store
// erases exactly that: it would let `duyetCaCay` be replaced by a single-level read and stay green.
//
// # THE FAKE HOLDS A REAL TREE
//
// Rows live in a map keyed by internal id, so a test can build `A → B → C`, hang a grandchild off
// it, or close a cycle — and the store's own statements are what read it back. A fake that answered
// one canned row could not express the three-level cases, which are exactly the ones that separate
// a correct recursion from a plausible one.
//
// # WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there
//
// Nothing PostgreSQL does with these statements: not the CHECK constraints of migration 0006, not
// `nhiem_vu_bat_bien` (the trigger that refuses a change to `han_ban_dau`), not
// `UNIQUE (tenant_id, ma)`, not `nhiem_vu_khong_tu_lam_cha` from 0008, and not the hash partition
// routing. That half needs a real server — VIGOV_TEST_DSN is unset here — and it is the FLOOR under
// everything asserted in nhiem_vu_test.go: the refusals tested there are the SENTENCE, not the
// enforcement.

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

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// khoNhiemVuGia serves the task register, its timeline and its extension requests.
type khoNhiemVuGia struct {
	mu   sync.Mutex
	lenh []lenhPhieu // reused from driver_gia_phieu_test.go — same shape, same `trongGiaoDich` field

	dangMo                       bool
	batDau, daCommit, daRollback int

	// nhiemVu is the register, keyed by INTERNAL id. Each value is a column -> value map, assembled
	// by NAME from the SELECT list the store itself wrote.
	nhiemVu map[string]map[string]driver.Value

	// nhatKy is each task's timeline, NEWEST FIRST, as the store's ORDER BY returns it.
	nhatKy map[string][]string

	// deNghi is the extension requests, keyed by request id.
	deNghi map[string]map[string]driver.Value

	// soLonNhat is what the minting query answers.
	soLonNhat int64

	// maDaDung answers the duplicate-code check.
	maDaDung int64

	doiDong int64

	loi    error
	loiSau string
	daNo   bool
}

// --- fixtures -------------------------------------------------------------------------------------

const (
	idNVGoc   = "01JNHIEMVUGOCTRONGTEST000"
	maNVGoc   = "NV19"
	idNVCon   = "01JNHIEMVUCONTRONGTEST000"
	maNVCon   = "NV20"
	idNVChau  = "01JNHIEMVUCHAUTRONGTEST00"
	maNVChau  = "NV21"
	idMoiSinh = "01JNHIEMVUMOISINHTEST0000"
	idDeNghi  = "01JDENGHILUIHANTEST000000"

	// The leader named ON THE RECORD — ADR 0038's second layer compares against this value. It is a
	// STAFF BUSINESS CODE and is DELIBERATELY NOT `maCanBoThu`: the acting officer in most cases is
	// somebody else, so a rule that compared the wrong pair would show up as a refusal rather than
	// pass by coincidence.
	maLanhDao = "CB-00007"

	maNguoiThucHien = "CB-00311"
)

var (
	// The instants on the fixture task. ALL DISTINCT AND NONE A ROUND OFFSET FROM ANOTHER, so a
	// local duration arithmetic slipped into the use case could not land on one by accident.
	mocHanNV     = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	mocHanGocNV  = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	mocHanMoiNV  = time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC)
	mocTaoNV     = time.Date(2026, 6, 1, 3, 30, 0, 0, time.UTC)
	mocThaoTacNV = time.Date(2026, 9, 23, 8, 5, 0, 0, time.UTC)
)

// dongNhiemVuGia is one `nhiem_vu` row as the driver hands it back.
//
// EVERY VALUE IS DISTINCT AND OF THE RIGHT TYPE, so a Scan wired to the wrong position comes back
// as visibly wrong data rather than as a zero that looks plausible. The two BOOLEANs are OPPOSITE
// for the same reason: they are adjacent columns of one type, and swapping them compiles and runs.
func dongNhiemVuGia(id, ma string, sua map[string]driver.Value) map[string]driver.Value {
	d := map[string]driver.Value{
		"id":                            id,
		"ma":                            ma,
		"loai":                          "theo-van-ban",
		"khoi":                          "khoi-dang",
		"tieu_de":                       "Báo cáo tổng kết việc thực hiện chủ trương về công tác cán bộ",
		"mo_ta":                         "Tổng hợp số liệu từ các chi bộ trực thuộc.",
		"trang_thai":                    "dang-thuc-hien",
		"muc_uu_tien":                   "cao",
		"nguon_giao":                    "ket-luan-hop",
		"nguon_id":                      "klh-007",
		"bo_phan_id":                    "bp-vpdu",
		"nguoi_thuc_hien_ma":            maNguoiThucHien,
		"lanh_dao_giao_viec_ma":         maLanhDao,
		"co_quan_chu_tri_id":            "bp-vpdu",
		"chuyen_vien_theo_doi_ma":       "CB-00412",
		"han_xu_ly":                     mocHanNV,
		"han_ban_dau":                   mocHanGocNV,
		"ngay_hoan_thanh":               nil,
		"tien_do":                       int64(40),
		"tom_tat_ket_qua":               nil,
		"ghi_chu":                       nil,
		"lanh_dao_phe_duyet_hoan_thanh": true,
		"cap_tren_cong_nhan_hoan_thanh": false,
		"nguoi_tao_ma":                  "CB-00123",
		"tao_luc":                       mocTaoNV,
		"nhiem_vu_cha_id":               nil,
	}
	for k, v := range sua {
		d[k] = v
	}
	return d
}

// dongDeNghiGia is one `de_nghi_lui_han` row.
func dongDeNghiGia(sua map[string]driver.Value) map[string]driver.Value {
	d := map[string]driver.Value{
		"id":               idDeNghi,
		"nhiem_vu_id":      idNVGoc,
		"nguoi_de_nghi_ma": maNguoiThucHien,
		"nguoi_duyet_ma":   nil,
		"han_moi":          mocHanMoiNV,
		"ly_do":            "Chờ số liệu từ ba chi bộ chưa gửi về.",
		"trang_thai":       "cho-duyet",
		"thoi_diem":        mocTaoNV,
		"duyet_luc":        nil,
	}
	for k, v := range sua {
		d[k] = v
	}
	return d
}

// khoNVMau is one root task, nothing else: no children, no timeline, no requests.
func khoNVMau() *khoNhiemVuGia {
	return &khoNhiemVuGia{
		doiDong: 1,
		nhiemVu: map[string]map[string]driver.Value{
			idNVGoc: dongNhiemVuGia(idNVGoc, maNVGoc, nil),
		},
		nhatKy: map[string][]string{},
		deNghi: map[string]map[string]driver.Value{},
	}
}

// themCon hangs a child off a parent, so a test can build a tree of any depth.
//
// IT SETS BOTH SIDES — the child's `nhiem_vu_cha_id` is what the downward walk reads, and it is ALSO
// what the upward cycle walk reads. One fake feeding both directions is what makes a three-level
// case mean the same thing to `duyetCaCay` and to `kiemChuTrinh`.
func (k *khoNhiemVuGia) themCon(id, ma, chaID string, trangThai domain.TrangThaiNhiemVu) {
	k.nhiemVu[id] = dongNhiemVuGia(id, ma, map[string]driver.Value{
		"nhiem_vu_cha_id": chaID,
		"trang_thai":      string(trangThai),
		// A CHILD WITH NO DEADLINE OF ITS OWN — ADR 0037 decision 2 says it inherits none, so the
		// ordinary child is exactly this shape.
		"han_xu_ly":   nil,
		"han_ban_dau": nil,
	})
}

// --- the driver -------------------------------------------------------------------------------------

func (k *khoNhiemVuGia) ghi(q string, args []driver.NamedValue) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	k.mu.Lock()
	k.lenh = append(k.lenh, lenhPhieu{sql: q, args: gt, trongGiaoDich: k.dangMo})
	k.mu.Unlock()
}

// cau returns every recorded statement containing `tu`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (k *khoNhiemVuGia) cau(tu string) []lenhPhieu {
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

func (k *khoNhiemVuGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

func (k *khoNhiemVuGia) kiemLoi(q string) error {
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

func (k *khoNhiemVuGia) Connect(context.Context) (driver.Conn, error) {
	return &connNVGia{k: k}, nil
}
func (k *khoNhiemVuGia) Driver() driver.Driver { return trinhNVGia{} }

type trinhNVGia struct{}

func (trinhNVGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type connNVGia struct{ k *khoNhiemVuGia }

func (c *connNVGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connNVGia) Close() error { return nil }

func (c *connNVGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connNVGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.dangMo = true
	c.k.mu.Unlock()
	return &txNVGia{k: c.k}, nil
}

func (c *connNVGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (
	driver.Result, error) {

	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	if strings.Contains(q, "UPDATE nhiem_vu") || strings.Contains(q, "UPDATE de_nghi_lui_han") {
		// THE ONLY STATEMENTS WHOSE ROW COUNT MEANS ANYTHING. The store turns zero into a refusal —
		// the race between two officers — so the count has to be settable; a fake that always
		// answered 1 would make that case untestable.
		return driver.RowsAffected(c.k.doiDong), nil
	}
	return driver.RowsAffected(1), nil
}

func (c *connNVGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (
	driver.Rows, error) {

	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	cot, err := cotTrongSelect(q)
	if err != nil {
		return nil, err
	}
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}

	switch {
	case strings.Contains(q, "FROM nhat_ky_nhiem_vu"):
		return c.k.doNhatKy(cot, gt)
	case strings.Contains(q, "FROM de_nghi_lui_han"):
		return c.k.doDeNghi(q, cot, gt)
	case strings.Contains(q, "FROM nhiem_vu"):
		return c.k.doNhiemVu(q, cot, gt)
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// doNhiemVu answers every read of the register. THE BRANCHES ARE ORDERED FROM NARROWEST STATEMENT
// TO WIDEST, because the wide one (`SELECT <all columns> … WHERE id = $2`) would otherwise swallow
// the parent lookup, which also matches on `id`.
func (k *khoNhiemVuGia) doNhiemVu(q string, cot []string, args []driver.Value) (driver.Rows, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	switch {
	// THE COLUMN NAME IS REPLACED FOR THESE TWO. `cotTrongSelect` splits the SELECT list on commas,
	// and `COALESCE(MAX(substring(ma from 3)::bigint), 0)` carries one — so the generic reader sees
	// two columns where the store scans one. An aggregate answers ONE value by construction, so the
	// honest fake names it rather than parsing SQL it does not need to understand.
	case strings.Contains(q, "MAX("):
		return &rowsNVGia{cot: []string{"so"}, hang: [][]driver.Value{{k.soLonNhat}}}, nil

	case strings.Contains(q, "count(*)"):
		return &rowsNVGia{cot: []string{"n"}, hang: [][]driver.Value{{k.maDaDung}}}, nil

	case strings.Contains(q, "nhiem_vu_cha_id = $2"):
		// The DOWNWARD step: children of one parent, in `ma` order exactly as the store asks.
		var con []map[string]driver.Value
		for _, r := range k.nhiemVu {
			if r["nhiem_vu_cha_id"] == args[1] {
				con = append(con, r)
			}
		}
		sapTheoMa(con)
		return dungRows(cot, con)

	case len(cot) == 1 && cot[0] == "nhiem_vu_cha_id":
		// The UPWARD step the cycle walk is built from.
		r, co := k.nhiemVu[fmt.Sprint(args[1])]
		if !co {
			return &rowsNVGia{cot: cot}, nil
		}
		return dungRows(cot, []map[string]driver.Value{r})

	case strings.Contains(q, "AND id = $2"):
		r, co := k.nhiemVu[fmt.Sprint(args[1])]
		if !co {
			return &rowsNVGia{cot: cot}, nil
		}
		return dungRows(cot, []map[string]driver.Value{r})

	case strings.Contains(q, "AND ma = $2"):
		for _, r := range k.nhiemVu {
			if r["ma"] == args[1] {
				return dungRows(cot, []map[string]driver.Value{r})
			}
		}
		return &rowsNVGia{cot: cot}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

func (k *khoNhiemVuGia) doNhatKy(cot []string, args []driver.Value) (driver.Rows, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	var hang [][]driver.Value
	for _, tt := range k.nhatKy[fmt.Sprint(args[1])] {
		hang = append(hang, []driver.Value{tt})
	}
	return &rowsNVGia{cot: cot, hang: hang}, nil
}

func (k *khoNhiemVuGia) doDeNghi(q string, cot []string, args []driver.Value) (driver.Rows, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if strings.Contains(q, "count(*)") {
		var n int64
		for _, r := range k.deNghi {
			if r["nhiem_vu_id"] == args[1] && r["trang_thai"] == "cho-duyet" {
				n++
			}
		}
		return &rowsNVGia{cot: cot, hang: [][]driver.Value{{n}}}, nil
	}

	// THE PAIR IS MATCHED, NOT JUST THE REQUEST ID. That is what stops a request filed on task A
	// being decided through task B's URL, where the leader named on B would answer for A.
	r, co := k.deNghi[fmt.Sprint(args[1])]
	if !co || r["nhiem_vu_id"] != args[2] {
		return &rowsNVGia{cot: cot}, nil
	}
	return dungRows(cot, []map[string]driver.Value{r})
}

// sapTheoMa puts rows in `ma` order — the store's own ORDER BY. Without it a map iteration would
// reorder the refusal message between runs, and an assertion on it would be flaky rather than wrong.
func sapTheoMa(rows []map[string]driver.Value) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && fmt.Sprint(rows[j-1]["ma"]) > fmt.Sprint(rows[j]["ma"]); j-- {
			rows[j-1], rows[j] = rows[j], rows[j-1]
		}
	}
}

// dungRows assembles rows BY NAME from the SELECT list the store itself wrote.
//
// THAT IS WHAT MAKES THE COLUMN CHECK WORK: reordering `cotNhiemVu` without reordering the matching
// Scan comes back as WRONG DATA rather than as a plausible zero — and the columns it would shift
// are the three adjacent TIMESTAMPTZs whose confusion produces a wrong figure rather than an error.
func dungRows(cot []string, rows []map[string]driver.Value) (driver.Rows, error) {
	var hang [][]driver.Value
	for _, r := range rows {
		mot := make([]driver.Value, len(cot))
		for i, ten := range cot {
			v, co := r[ten]
			if !co {
				// A column was added to a `cot…` constant and not to the fixture. Failing loudly
				// beats scanning a nil that "passes" while proving nothing.
				panic("driver giả: không có giá trị mẫu cho cột " + ten)
			}
			mot[i] = v
		}
		hang = append(hang, mot)
	}
	return &rowsNVGia{cot: cot, hang: hang}, nil
}

type txNVGia struct{ k *khoNhiemVuGia }

func (t *txNVGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.dangMo = false
	t.k.mu.Unlock()
	return nil
}

func (t *txNVGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.dangMo = false
	t.k.mu.Unlock()
	return nil
}

type rowsNVGia struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsNVGia) Columns() []string { return r.cot }
func (r *rowsNVGia) Close() error      { return nil }
func (r *rowsNVGia) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

// --- the harness --------------------------------------------------------------------------------

// dungGhiNhiemVu builds the REAL use case over the REAL stores over the fake driver, with the ids
// and the clock pinned so every assertion is about the decision and not about randomness.
func dungGhiNhiemVu(t *testing.T, k *khoNhiemVuGia) (*GhiNhiemVu, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and IN ORDER. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both stores, exactly as cmd/server wires it: the transaction the use case
	// opens is the transaction both stores write in, and two handles would be two pools.
	kho := pkgstore.New(db)
	uc := NewGhiNhiemVu(kho, petstore.NewNhiemVuStore(kho), petstore.NewDeNghiLuiHanStore(kho))

	// IDS ARE HANDED OUT IN ORDER so a test can name the one it expects. The first id of a create is
	// the task, the second is its timeline row.
	var n int
	uc.sinhID = func() (string, error) {
		n++
		if n == 1 {
			return idMoiSinh, nil
		}
		return fmt.Sprintf("01JSINHRATRONGTEST%06d", n), nil
	}
	uc.nay = func() time.Time { return mocThaoTacNV }
	return uc, ctxXa(xaThu)
}
