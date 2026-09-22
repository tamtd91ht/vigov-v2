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

// A second fake database/sql driver, for the disbursement voucher register.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument
// driver_gia_danh_muc_test.go makes, and it is heavier here because the properties are about money:
//
//	the business write and its audit entry are in ONE transaction   (rule 6, invariant 3)
//	a refusal leaves the transaction with NOTHING committed
//	`trang_thai` is never written from anything a request could reach
//	an unlock writes its reason IN THE SAME STATEMENT as the state  (migration 0005, decision 1)
//	`so_lan_mo_khoa` is incremented IN SQL, never read-modify-written
//	a soft delete writes all three of rule 7 invariant 1's columns
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// A SECOND DRIVER AND NOT AN EXTENSION OF THE FIRST: that one answers `count(*)` and one catalogue
// row shape. Teaching it a second table would make every case in either file depend on a branch
// written for the other, and the first surprising interaction would be silent.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not the `chung_tu_da_khoa` trigger, not
// `chung_tu_giai_ngan_mo_khoa_du_vet`, not `CHECK (so_tien > 0)`, not the partition routing. That
// half needs a real server (VIGOV_TEST_DSN is unset here), and it is the FLOOR under everything
// this file asserts — the application refusals tested below are the SENTENCE, not the enforcement.

// hangCT is the row the FOR UPDATE read hands back. A nil *hangCT means "no such live voucher".
type hangCT struct {
	id, duAnID   string
	ngayChi      time.Time
	soTien       int64
	noiDung      string
	doiTac       string
	soChungTu    string
	trangThai    string
	nguoiNhap    string
	nguoiXacNhan string
	nguoiKhoa    string
	nguoiMoKhoa  string
	thoiDiemKhoa *time.Time
	thoiDiemMo   *time.Time
	lyDoMoKhoa   string
	soLanMoKhoa  int64
}

type khoCTGia struct {
	mu   sync.Mutex
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	hang *hangCT

	// maDuAn is what `SELECT ma FROM du_an` answers. EMPTY MEANS NO SUCH LIVE PROJECT, which is the
	// case ErrKhongThayDuAnCuaChungTu exists for — money filed against a project that totals nowhere.
	maDuAn string

	loi error

	// loiSau fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET `loi`: failing everything cannot tell "rolled back" from "never started".
	// The invariant worth proving is that a failure on the LAST statement of a transaction — the
	// audit entry — takes the business write down with it, and that needs everything before it to
	// have already run.
	loiSau string
	daNo   bool
}

func (k *khoCTGia) ghi(q string, args []driver.NamedValue) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	k.mu.Lock()
	k.lenh = append(k.lenh, lenhGhi{sql: q, args: gt})
	k.mu.Unlock()
}

// cau returns every recorded statement containing `tu`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (k *khoCTGia) cau(tu string) []lenhGhi {
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

func (k *khoCTGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

func (k *khoCTGia) kiemLoi(q string) error {
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

func (k *khoCTGia) Connect(context.Context) (driver.Conn, error) { return &connCTGia{k: k}, nil }
func (k *khoCTGia) Driver() driver.Driver                        { return trinhGia{} }

type connCTGia struct{ k *khoCTGia }

func (c *connCTGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connCTGia) Close() error { return nil }

func (c *connCTGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connCTGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	return &txCTGia{k: c.k}, nil
}

func (c *connCTGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. store.doiMotDongChungTu turns zero into "not found", and a fake
	// returning zero would make every update look like a missing voucher — hiding the case this
	// actually tests.
	return driver.RowsAffected(1), nil
}

func (c *connCTGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(q, "FROM du_an"):
		if c.k.maDuAn == "" {
			return &rowsGia{cot: []string{"ma"}}, nil
		}
		return &rowsGia{cot: []string{"ma"}, hang: [][]driver.Value{{c.k.maDuAn}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.k.hang == nil {
			return &rowsGia{cot: cotCT()}, nil
		}
		h := *c.k.hang
		return &rowsGia{cot: cotCT(), hang: [][]driver.Value{{
			h.id, h.duAnID, h.ngayChi, h.soTien, h.noiDung,
			h.doiTac, h.soChungTu, h.trangThai,
			h.nguoiNhap, h.nguoiXacNhan, h.nguoiKhoa, h.nguoiMoKhoa,
			gioHoacNil(h.thoiDiemKhoa), gioHoacNil(h.thoiDiemMo),
			h.lyDoMoKhoa, h.soLanMoKhoa,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

func gioHoacNil(t *time.Time) driver.Value {
	if t == nil {
		return nil
	}
	return *t
}

// cotCT mirrors fistore's cotChungTu ORDER. Written out here rather than imported so that
// reordering the store's list without reordering its Scan turns this red too — and that list has
// four adjacent staff-code columns whose swap would make "the person who locked it" name the person
// who unlocked it, which is the exact comparison the self-unlock rule is made of.
func cotCT() []string {
	return []string{
		"id", "du_an_id", "ngay_chi", "so_tien", "noi_dung",
		"doi_tac", "so_chung_tu", "trang_thai",
		"nguoi_nhap_id", "nguoi_xac_nhan_id", "nguoi_khoa_id", "nguoi_mo_khoa_id",
		"thoi_diem_khoa", "thoi_diem_mo_khoa", "ly_do_mo_khoa", "so_lan_mo_khoa",
	}
}

type txCTGia struct{ k *khoCTGia }

func (t *txCTGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.mu.Unlock()
	return nil
}

func (t *txCTGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.mu.Unlock()
	return nil
}

// lucCoDinh is the instant every lock and unlock below is stamped with. FIXED rather than
// time.Now(), so that an assertion can compare what reached `thoi_diem_khoa` with what the test
// asked for — a wall-clock read would make that assertion about scheduling.
var lucCoDinh = time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC)

// dungUseCaseChungTu builds the REAL use case over the REAL store over the fake driver, with the id
// and the clock pinned so assertions can name them.
func dungUseCaseChungTu(t *testing.T, k *khoCTGia) (*ChungTuGiaiNgan, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	kho := store.New(db)
	uc := NewChungTuGiaiNgan(kho, fistore.NewChungTuGiaiNganStore(kho))
	uc.sinhID = func() (string, error) { return idChungTuMoi, nil }
	uc.nay = func() time.Time { return lucCoDinh }
	return uc, tenant.Into(context.Background(), xaA)
}

const idChungTuMoi = "01JCHUNGTUMOIVUATAO000000"
