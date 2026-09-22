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

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// A third fake database/sql driver in this package, for the three calendar tables.
//
// WHY THE REAL STORES RUN ON TOP OF IT INSTEAD OF HAND-WRITTEN FAKE STORES — the same argument
// driver_gia_sla_test.go makes. Every property worth proving here is a property OF THE SQL AND OF
// THE TRANSACTION BOUNDARIES, and a fake store erases exactly that:
//
//	the business write and its audit entry are in ONE transaction     (rule 6, invariant 3)
//	a refusal leaves the transaction with NOTHING committed
//	the seeding run's READ happens inside the same transaction as its writes
//	seeding a second time issues NO UPDATE of any kind                (the load-bearing one)
//	the overlap check reads the CALENDAR, inside the transaction, before any INSERT
//	the soft delete is an UPDATE carrying deleted_by and delete_reason, never a DELETE
//
// A SEPARATE DRIVER AND NOT AN EXTENSION OF khoSLAGia: that one serves one table with one row
// shape. Teaching it three more would make every case in either file depend on a branch written
// for the other, and the first surprising interaction would be silent.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not `UNIQUE (tenant_id, thu, bat_dau)`, not
// `CHECK (ket_thuc > bat_dau)`, not `make_time` rejecting an hour of 25, not `FOR UPDATE` actually
// serialising two administrators, not the partition routing. That half needs a real server —
// VIGOV_TEST_DSN is unset here, so the pg suites skip without running a statement — and it is the
// FLOOR under everything asserted below. The refusals tested here are the SENTENCE, not the
// enforcement.

// The rows the fake tables hold. One struct per table, each carrying the soft-delete flag the
// `…DeGhi` reads publish.
type (
	hangCaTuan struct {
		id      string
		thu     int
		batDau  int
		ketThuc int
		ghiChu  string
		daXoa   bool
	}
	hangNghiLe struct {
		id    string
		ngay  string
		ten   string
		daXoa bool
	}
	hangLamBu struct {
		id      string
		ngay    string
		batDau  int
		ketThuc int
		ten     string
		daXoa   bool
	}
)

// khoLichGia is the three fake tables plus the recording tape.
type khoLichGia struct {
	mu   sync.Mutex
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	caTuan []hangCaTuan
	nghiLe []hangNghiLe
	lamBu  []hangLamBu

	loi error

	// loiSau fails the FIRST statement containing this substring, and only that one. Failing
	// everything cannot tell "rolled back" from "never started"; the invariant worth proving is
	// that a failure on the LAST statement of a transaction — the audit entry — takes the business
	// write down with it.
	loiSau string
	daNo   bool
}

func (k *khoLichGia) ghi(q string, args []driver.NamedValue) {
	k.mu.Lock()
	k.lenh = append(k.lenh, lenhGhi{sql: q, args: giaTri(args)})
	k.mu.Unlock()
}

// cau returns every recorded statement containing `tu`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (k *khoLichGia) cau(tu string) []lenhGhi {
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

func (k *khoLichGia) soCau(tu string) int  { return len(k.cau(tu)) }
func (k *khoLichGia) coCau(tu string) bool { return k.soCau(tu) > 0 }

func (k *khoLichGia) kiemLoi(q string) error {
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

func (k *khoLichGia) Connect(context.Context) (driver.Conn, error) { return &connLichGia{k: k}, nil }
func (k *khoLichGia) Driver() driver.Driver                        { return trinhGia{} }

type connLichGia struct{ k *khoLichGia }

func (c *connLichGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connLichGia) Close() error { return nil }

func (c *connLichGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connLichGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	return &txLichGia{k: c.k}, nil
}

func (c *connLichGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. doiMotDongLich turns zero into "row not found", and a fake
	// returning zero would make every update look like a missing row — hiding the case that
	// actually tests.
	return driver.RowsAffected(1), nil
}

func (c *connLichGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}

	c.k.mu.Lock()
	defer c.k.mu.Unlock()

	// THE id AND THE YEAR ARE TAKEN FROM THE BOUND PARAMETERS AND NOT FROM THE FIXTURE, so a store
	// that stopped binding them, or bound them in the wrong position, hands back the wrong rows here
	// rather than quietly passing.
	motID := func() string {
		if len(args) >= 2 {
			s, _ := args[1].Value.(string)
			return s
		}
		return ""
	}
	nam := func() string {
		if len(args) >= 2 {
			switch v := args[1].Value.(type) {
			case int64:
				return fmt.Sprintf("%04d", v)
			case int:
				return fmt.Sprintf("%04d", v)
			}
		}
		return ""
	}

	switch {
	case strings.Contains(q, "FROM lich_lam_viec"):
		ra := make([][]driver.Value, 0, len(c.k.caTuan))
		for _, h := range c.k.caTuan {
			if strings.Contains(q, "FOR UPDATE") && (h.id != motID() || h.daXoa) {
				continue
			}
			ra = append(ra, []driver.Value{
				h.id, int64(h.thu), int64(h.batDau), int64(h.ketThuc), h.ghiChu, h.daXoa,
			})
		}
		return &rowsGia{cot: cotCaTuanGia(), hang: ra}, nil

	case strings.Contains(q, "FROM ngay_nghi_le"):
		ra := make([][]driver.Value, 0, len(c.k.nghiLe))
		for _, h := range c.k.nghiLe {
			if strings.Contains(q, "FOR UPDATE") {
				if h.id != motID() || h.daXoa {
					continue
				}
			} else if !strings.HasPrefix(h.ngay, nam()) {
				// The year window really is applied, so a store that dropped `make_date($2…)` shows
				// up here as a check reading rows it should never have seen.
				continue
			}
			ra = append(ra, []driver.Value{h.id, h.ngay, h.ten, h.daXoa})
		}
		return &rowsGia{cot: cotNghiLeGia(), hang: ra}, nil

	case strings.Contains(q, "FROM ngay_lam_bu"):
		ra := make([][]driver.Value, 0, len(c.k.lamBu))
		for _, h := range c.k.lamBu {
			if strings.Contains(q, "FOR UPDATE") {
				if h.id != motID() || h.daXoa {
					continue
				}
			} else if !strings.HasPrefix(h.ngay, nam()) {
				continue
			}
			ra = append(ra, []driver.Value{
				h.id, h.ngay, int64(h.batDau), int64(h.ketThuc), h.ten, h.daXoa,
			})
		}
		return &rowsGia{cot: cotLamBuGia(), hang: ra}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// The column lists mirror the stores' own ORDER. Written out here rather than imported so that
// reordering a store's list without reordering its Scan turns this red too — and two of these three
// lists hold ADJACENT SAME-TYPED COLUMNS whose transposition produces no error at all, only a week
// that runs from closing time to opening time.
func cotCaTuanGia() []string {
	return []string{"id", "thu", "bat_dau", "ket_thuc", "ghi_chu", "da_xoa"}
}
func cotNghiLeGia() []string { return []string{"id", "ngay", "ten", "da_xoa"} }
func cotLamBuGia() []string {
	return []string{"id", "ngay", "bat_dau", "ket_thuc", "ten", "da_xoa"}
}

type txLichGia struct{ k *khoLichGia }

func (t *txLichGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.mu.Unlock()
	return nil
}

func (t *txLichGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.mu.Unlock()
	return nil
}

// --- fixtures -----------------------------------------------------------------------------------

const (
	xaLich      = tenant.ID("01JLICH0000000000000000000")
	maCanBoLich = "CB-0042"
	idCaThu     = "01JCALAMVIECVUADOC0000000X"
	idNghiLeThu = "01JNGAYNGHILEVUADOC00000X"
	idLamBuThu  = "01JNGAYLAMBUVUADOC000000X"
)

// idGieoLich mints a distinct, recognisable id per seeded row, so an assertion can say WHICH row an
// INSERT was for without depending on randomness.
func idGieoLich(n int) string { return fmt.Sprintf("01JGLIC%019d", n) }

// nguoiLich is the actor. Vet.ID is the STAFF CODE — rule 6, invariant 8 — and ID is the internal id
// the use case decides on. They are DIFFERENT VALUES on purpose: a test whose two identifiers are
// equal cannot tell which one reached `audit_log.actor_id` or `deleted_by`.
func nguoiLich() NguoiThucHien {
	return NguoiThucHien{
		ID:  "nd-01JNOIBOCUANGUOISUALICH",
		Vet: audit.Actor{ID: maCanBoLich, Kind: "staff", IP: "10.0.0.7"},
	}
}

// dungUseCaseLich builds the REAL use case over the REAL stores over the fake driver, with the id
// generator pinned so assertions can name the rows it writes.
func dungUseCaseLich(t *testing.T, k *khoLichGia) (*Lich, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind all three stores, exactly as cmd/server wires it: the transaction the use
	// case opens is the transaction the stores write in, and separate handles would be separate
	// pools.
	kho := store.New(db)
	uc := NewLich(kho,
		idstore.NewLichLamViecStore(kho),
		idstore.NewNgayNghiLeStore(kho),
		idstore.NewNgayLamBuStore(kho))

	var n int
	uc.sinhID = func() (string, error) {
		n++
		return idGieoLich(n), nil
	}
	return uc, tenant.Into(context.Background(), xaLich)
}

// tuanDayDu is a commune whose week already holds every seed row, WITH ONE SESSION EDITED — Monday
// afternoon ends at 16:00 instead of the seed's 17:00.
//
// THAT EDIT IS THE WHOLE POINT OF THE FIXTURE. It is the value a second seeding run must not touch,
// and it is deliberately an hour a commune would plausibly choose.
func tuanDayDu() []hangCaTuan {
	var ra []hangCaTuan
	for i, g := range domain.BoGieoCaLamViec() {
		ketThuc := int(g.KetThuc)
		if g.Thu == 1 && g.BatDau == domain.GioMoChieu {
			ketThuc = gioXaDaSuaLich
		}
		ra = append(ra, hangCaTuan{
			id:      fmt.Sprintf("01JCOCA%019d", i),
			thu:     g.Thu,
			batDau:  int(g.BatDau),
			ketThuc: ketThuc,
			ghiChu:  g.GhiChu,
		})
	}
	return ra
}

// gioXaDaSuaLich is the closing time the commune typed over the seed's 17:00. Named rather than
// inlined, so the assertion that it survives a second seeding run reads as what it is.
const gioXaDaSuaLich = 16 * 3600
