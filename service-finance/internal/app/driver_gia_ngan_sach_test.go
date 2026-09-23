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

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// A third fake `database/sql` driver, for the budget board.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument the
// other two drivers make, and it is heaviest here because the properties are about the figures that
// go into a document sent to a higher authority:
//
//	the business write and its audit entry are in ONE transaction   (rule 6, invariant 3)
//	a refusal leaves the transaction with NOTHING committed
//	`is_headline` is written by EXACTLY TWO statements, and marking one row CLEARS the others
//	a soft delete writes all three of rule 7 invariant 1's columns AND releases the star
//	a cell is cleared with NULL, never with a DELETE
//	`cach_tinh` is never written from anything a request could reach
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not `ho_so_luu_tru_cam_xoa_cung`, not the CHECK
// constraints on `loai` / `kieu` / `vai_tro` / `cach_tinh`, not `UNIQUE (tenant_id, nam, loai, lan)`,
// not `ON CONFLICT` actually conflicting, and not the partition routing. That half needs a real
// server (VIGOV_TEST_DSN is unset here and Docker is not running), and it is the FLOOR under
// everything this file asserts — the application refusals tested below are the SENTENCE, not the
// enforcement.

// hangBang is the sheet the FOR UPDATE read hands back. nil means "no such live sheet".
type hangBang struct {
	id, ma            string
	nam               int
	loai              string
	lan               int
	tieuDe, donViTinh string
}

type khoNSGia struct {
	mu   sync.Mutex
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	bang     *hangBang
	cot      []domain.CotNganSach
	khoanMuc []domain.KhoanMucNganSach
	gia      map[string]map[string]domain.Dong

	// bangIDCuaKhoanMuc is what `SELECT bang_id FROM khoan_muc_ngan_sach` answers. EMPTY MEANS NO
	// SUCH LIVE LINE, which is the case domain.ErrKhongThayKhoanMuc exists for.
	bangIDCuaKhoanMuc string

	// daCoBangConSong is what CoBangConSong answers, and lanKeTiep what LanKeTiep answers.
	daCoBangConSong bool
	lanKeTiep       int

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

func (k *khoNSGia) ghi(q string, args []driver.NamedValue) {
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
func (k *khoNSGia) cau(tu string) []lenhGhi {
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

func (k *khoNSGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

func (k *khoNSGia) kiemLoi(q string) error {
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

func (k *khoNSGia) Connect(context.Context) (driver.Conn, error) { return &connNSGia{k: k}, nil }
func (k *khoNSGia) Driver() driver.Driver                        { return trinhGia{} }

type connNSGia struct{ k *khoNSGia }

func (c *connNSGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connNSGia) Close() error { return nil }

func (c *connNSGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connNSGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	return &txNSGia{k: c.k}, nil
}

func (c *connNSGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. The store turns zero into "not found", and a fake returning zero
	// would make every update look like a missing row — hiding the case this actually tests.
	return driver.RowsAffected(1), nil
}

// QueryContext dispatches on the statement, and THE ORDER OF THE CASES IS LOAD-BEARING: three of
// these statements name `khoan_muc_ngan_sach`, and the value join names it as well as
// `gia_tri_khoan_muc`. Matched in the wrong order, the tree read would answer the value query and
// every figure on the sheet would silently be empty.
func (c *connNSGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	switch {
	case strings.Contains(q, "FROM gia_tri_khoan_muc"):
		var hang [][]driver.Value
		for _, k := range c.k.khoanMuc {
			for _, cot := range c.k.cot {
				if g, co := c.k.gia[k.ID][cot.ID]; co {
					hang = append(hang, []driver.Value{k.ID, cot.ID, int64(g)})
				}
			}
		}
		return &rowsGia{cot: []string{"khoan_muc_id", "cot_id", "gia_tri"}, hang: hang}, nil

	case strings.Contains(q, "SELECT bang_id FROM khoan_muc_ngan_sach"):
		if c.k.bangIDCuaKhoanMuc == "" {
			return &rowsGia{cot: []string{"bang_id"}}, nil
		}
		return &rowsGia{cot: []string{"bang_id"},
			hang: [][]driver.Value{{c.k.bangIDCuaKhoanMuc}}}, nil

	case strings.Contains(q, "FROM khoan_muc_ngan_sach"):
		var hang [][]driver.Value
		for _, k := range c.k.khoanMuc {
			hang = append(hang, []driver.Value{
				k.ID, k.BangID, k.ChaID, k.TT, k.Ten, int64(k.ThuTu),
				string(k.CachTinh), int64(k.Cap), k.LaDongTong,
			})
		}
		return &rowsGia{cot: cotKM(), hang: hang}, nil

	case strings.Contains(q, "FROM cot_ngan_sach"):
		var hang [][]driver.Value
		for _, cot := range c.k.cot {
			hang = append(hang, []driver.Value{
				cot.ID, cot.BangID, cot.Ten, int64(cot.ThuTu), string(cot.Kieu),
				cot.CongThuc, string(cot.VaiTro),
			})
		}
		return &rowsGia{cot: cotCotNS(), hang: hang}, nil

	case strings.Contains(q, "SELECT EXISTS"):
		return &rowsGia{cot: []string{"exists"},
			hang: [][]driver.Value{{c.k.daCoBangConSong}}}, nil

	case strings.Contains(q, "MAX(lan)"):
		return &rowsGia{cot: []string{"lan"},
			hang: [][]driver.Value{{int64(c.k.lanKeTiep)}}}, nil

	case strings.Contains(q, "FROM bang_ngan_sach"):
		if c.k.bang == nil {
			return &rowsGia{cot: cotBangNS()}, nil
		}
		b := c.k.bang
		return &rowsGia{cot: cotBangNS(), hang: [][]driver.Value{{
			b.id, b.ma, int64(b.nam), b.loai, int64(b.lan), b.tieuDe, b.donViTinh,
			nil, "", nil,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// cotBangNS / cotCotNS / cotKM mirror the store's column lists IN ORDER. Written out here rather
// than imported so that reordering one of the store's lists without reordering its Scan turns this
// red too — and `khoan_muc_ngan_sach` has `tt`, `ten` and `cach_tinh` all TEXT and adjacent, a swap
// among which is invisible on any row where they happen to look alike.
func cotBangNS() []string {
	return []string{"id", "ma", "nam", "loai", "lan", "tieu_de", "don_vi_tinh",
		"luy_ke_den", "nguon_tep", "nap_luc"}
}

func cotCotNS() []string {
	return []string{"id", "bang_id", "ten", "thu_tu", "kieu", "cong_thuc", "vai_tro"}
}

func cotKM() []string {
	return []string{"id", "bang_id", "cha_id", "tt", "ten", "thu_tu",
		"cach_tinh", "cap", "is_headline"}
}

type txNSGia struct{ k *khoNSGia }

func (t *txNSGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.mu.Unlock()
	return nil
}

func (t *txNSGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.mu.Unlock()
	return nil
}

// dungUseCaseNganSach builds the REAL use case over the REAL store over the fake driver, with the id
// pinned so assertions can name it.
func dungUseCaseNganSach(t *testing.T, k *khoNSGia) (*NganSach, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	kho := store.New(db)
	uc := NewNganSach(kho, fistore.NewNganSachStore(kho))
	n := 0
	uc.sinhID = func() (string, error) {
		n++
		if n == 1 {
			return idMoiNganSach, nil
		}
		return fmt.Sprintf("%s-%d", idMoiNganSach, n), nil
	}
	return uc, tenant.Into(context.Background(), xaA)
}

const idMoiNganSach = "01JNGANSACHMOIVUATAO000000"
