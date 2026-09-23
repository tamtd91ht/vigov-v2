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
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// A third fake database/sql driver, for the investment project register.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument the
// other two drivers make, and it is heavier here because creating a project is SEVERAL writes:
//
//	the project, its allocation lines and the audit entry are in ONE transaction  (rule 6, inv. 3)
//	a refusal leaves the transaction with NOTHING committed
//	`ma` and `nam` appear in NO update statement                    (rule 7 #4, §13 rule 8)
//	a soft delete writes all three of rule 7 invariant 1's columns
//	the code check counts SOFT-DELETED rows                         (§9, rule 7 invariant 3)
//	the funding source is checked against the PROJECT's budget year (§13 rule 8)
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// A THIRD DRIVER AND NOT AN EXTENSION OF THE OTHER TWO: they answer a catalogue row shape and a
// voucher row shape. Teaching either a third table would make every case in the other files depend
// on a branch written for this one, and the first surprising interaction would be silent.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not `UNIQUE (tenant_id, ma)`, not
// `ho_so_luu_tru_cam_xoa_cung`, not `CHECK (ke_hoach_von_nam >= 0)`, not the partition routing. That
// half needs a real server (VIGOV_TEST_DSN is unset here), and it is the FLOOR under everything this
// file asserts — the application refusals tested below are the SENTENCE, not the enforcement.

// hangDA is the row the FOR UPDATE read hands back. A nil *hangDA means "no such live project".
type hangDA struct {
	id, ma       string
	nam          int64
	hangMucID    string
	ten, moTa    string
	keHoach      int64
	tongMuc      *int64
	donVi, canBo string
	khoiCong     *time.Time
	hoanThanh    *time.Time
	hanGiaiNgan  time.Time
}

type khoDAGia struct {
	mu   sync.Mutex
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	hang *hangDA

	// maDaDung is what `SELECT count(*) FROM du_an WHERE tenant_id = $1 AND ma = $2` answers. It is a
	// COUNT rather than a boolean because the statement under test is a count, and the point of the
	// check is that it has NO `deleted_at` predicate: a withdrawn project still owns its code (§9).
	maDaDung int64

	// hangMucCo is the set of LIVE capital plan categories of this commune, a SET rather than a
	// boolean on purpose: the case worth separating is "this id names nothing here" from "no category
	// exists at all". The first is what a category of ANOTHER commune looks like from inside this one
	// — the query binds tenant_id = $1, so such a row is simply not there (rule 1).
	hangMucCo map[string]bool

	// nguonVonCo is keyed by "<id>|<nam>", because the statement binds BOTH: a source of the right
	// commune but the WRONG budget year must answer "no such source", or a 2026 project's allocation
	// lands on a card §6 draws for 2027 (§13 rule 8).
	nguonVonCo map[string]bool

	// soChungTu is what `count(*) FROM chung_tu_giai_ngan` answers — the live vouchers blocking a
	// removal (store.ErrDuAnConChungTu).
	soChungTu int64

	// soPhanBoXoa is what the allocation soft delete reports as RowsAffected, so a test can assert
	// the figure that reaches the audit delta.
	soPhanBoXoa int64

	loi error

	// loiSau fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET `loi`: failing everything cannot tell "rolled back" from "never started". The
	// invariant worth proving is that a failure on the LAST statement of a transaction — the audit
	// entry — takes the business write down with it, and that needs everything before it to have
	// already run.
	loiSau string
	daNo   bool
}

func (k *khoDAGia) ghi(q string, args []driver.NamedValue) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	k.mu.Lock()
	k.lenh = append(k.lenh, lenhGhi{sql: q, args: gt})
	k.mu.Unlock()
}

// cau returns every recorded statement containing `tu`, so an assertion names the statement it cares
// about rather than an index that shifts when a check is added.
func (k *khoDAGia) cau(tu string) []lenhGhi {
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

func (k *khoDAGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

func (k *khoDAGia) kiemLoi(q string) error {
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

func (k *khoDAGia) Connect(context.Context) (driver.Conn, error) { return &connDAGia{k: k}, nil }
func (k *khoDAGia) Driver() driver.Driver                        { return trinhGia{} }

type connDAGia struct{ k *khoDAGia }

func (c *connDAGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connDAGia) Close() error { return nil }

func (c *connDAGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connDAGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	return &txDAGia{k: c.k}, nil
}

func (c *connDAGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	// THE ALLOCATION SOFT DELETE REPORTS ITS OWN COUNT, because zero rows is the ORDINARY case there
	// (§9: a project with no allocation is normal) and the use case puts the figure into the audit
	// delta. Everything else reports one row: store.doiMotDongDuAn turns zero into "not found", and a
	// fake returning zero would make every update look like a missing project — hiding the case the
	// test is actually about.
	if strings.Contains(q, "UPDATE phan_bo_nguon_von") {
		return driver.RowsAffected(c.k.soPhanBoXoa), nil
	}
	return driver.RowsAffected(1), nil
}

func (c *connDAGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(q, "count(*) FROM du_an"):
		return &rowsGia{cot: []string{"count"}, hang: [][]driver.Value{{c.k.maDaDung}}}, nil
	case strings.Contains(q, "count(*) FROM chung_tu_giai_ngan"):
		return &rowsGia{cot: []string{"count"}, hang: [][]driver.Value{{c.k.soChungTu}}}, nil
	case strings.Contains(q, "FROM hang_muc_ke_hoach_von"):
		// NO ROW IS THE ANSWER FOR AN ID THIS COMMUNE DOES NOT HOLD, which is exactly what PostgreSQL
		// would return: the statement binds tenant_id = $1, so another commune's category and one that
		// never existed are one answer here (store.ErrKhongThayHangMuc).
		if len(args) < 2 || !c.k.hangMucCo[fmt.Sprint(args[1].Value)] {
			return &rowsGia{cot: []string{"?column?"}}, nil
		}
		return &rowsGia{cot: []string{"?column?"}, hang: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(q, "FROM nguon_von"):
		// KEYED ON id AND nam TOGETHER — $2 and $3 — because that is what the statement binds. A test
		// that only matched the id could not tell a right-year source from a wrong-year one, which is
		// the whole point of store.ErrKhongThayNguonVonPhanBo.
		if len(args) < 3 {
			return &rowsGia{cot: []string{"?column?"}}, nil
		}
		khoa := fmt.Sprintf("%v|%v", args[1].Value, args[2].Value)
		if !c.k.nguonVonCo[khoa] {
			return &rowsGia{cot: []string{"?column?"}}, nil
		}
		return &rowsGia{cot: []string{"?column?"}, hang: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.k.hang == nil {
			return &rowsGia{cot: cotDA()}, nil
		}
		h := *c.k.hang
		return &rowsGia{cot: cotDA(), hang: [][]driver.Value{{
			h.id, h.ma, h.nam, h.hangMucID, h.ten, h.moTa,
			h.keHoach, tongHoacKhong(h.tongMuc),
			h.donVi, h.canBo,
			gioHoacNil(h.khoiCong), gioHoacNil(h.hoanThanh), h.hanGiaiNgan,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// tongHoacKhong mirrors `COALESCE(tong_muc_duoc_duyet, 0)`: a NULL column reaches Go as 0, which is
// how domain.DuAn.TongMucHieuLuc recognises "the commune left it blank" (§9).
func tongHoacKhong(v *int64) driver.Value {
	if v == nil {
		return int64(0)
	}
	return *v
}

// cotDA mirrors fistore's cotDuAnGhi ORDER. Written out here rather than imported so that reordering
// the store's list without reordering its Scan turns this red too — and that list has two adjacent
// amounts and three adjacent dates whose swap changes every ratio on every screen while no row looks
// wrong.
func cotDA() []string {
	return []string{
		"id", "ma", "nam", "hang_muc_id", "ten", "mo_ta",
		"ke_hoach_von_nam", "tong_muc_duoc_duyet",
		"don_vi_thuc_hien_id", "can_bo_phu_trach_id",
		"ngay_khoi_cong", "ngay_hoan_thanh", "thoi_han_giai_ngan",
	}
}

type txDAGia struct{ k *khoDAGia }

func (t *txDAGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.mu.Unlock()
	return nil
}

func (t *txDAGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.mu.Unlock()
	return nil
}

// dungUseCaseDuAn builds the REAL use case over the REAL store over the fake driver, with the id
// generator pinned so assertions can name what it minted.
//
// THE GENERATOR COUNTS ITS CALLS, because the use case must mint a SEPARATE id for the project and
// for every allocation line. One id reused across `du_an.id` and `phan_bo_nguon_von.id` is something
// the separate PRIMARY KEYs permit and nothing would notice until somebody joined the two tables.
func dungUseCaseDuAn(t *testing.T, k *khoDAGia) (*DuAn, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	kho := store.New(db)
	uc := NewDuAn(kho, fistore.NewDuAnGhiStore(kho))
	n := 0
	uc.sinhID = func() (string, error) {
		n++
		if n == 1 {
			return idDuAnMoi, nil
		}
		return fmt.Sprintf("%s%02d", idPhanBoMoi, n-1), nil
	}
	return uc, tenant.Into(context.Background(), xaA)
}

const (
	idDuAnMoi   = "01JDUANMOIVUATAO000000000"
	idPhanBoMoi = "01JPHANBOMOI00000000000"
)
