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

// A second fake database/sql driver in this package, for the deadline table.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument
// service-finance/internal/app/driver_gia_chung_tu_test.go makes. Every property worth proving here
// is a property OF THE SQL AND OF THE TRANSACTION BOUNDARIES, and a fake store erases exactly that:
//
//	the business write and its audit entry are in ONE transaction        (rule 6, invariant 3)
//	a refusal leaves the transaction with NOTHING committed
//	the seeding run's READ happens inside the same transaction as its writes
//	seeding a second time issues NO UPDATE of any kind                   (the load-bearing one)
//	the edit's UPDATE names the five hour columns AND NOTHING ELSE
//	`linh_vuc` goes in as NULL for the default row, never as ''
//
// A SEPARATE DRIVER AND NOT AN EXTENSION OF ghiChep: that one serves the sign-in path and answers
// one staff row shape. Teaching it a second table would make every case in either file depend on a
// branch written for the other, and the first surprising interaction would be silent.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not `UNIQUE (tenant_id, loai_viec, linh_vuc_khoa)`,
// not the `linh_vuc_khoa` GENERATED column, not `CHECK (sla_gio_phai_duong)`, not `FOR UPDATE`
// actually serialising two administrators, not the partition routing. That half needs a real server
// — VIGOV_TEST_DSN is unset here, so the pg suites skip without running a statement — and it is the
// FLOOR under everything asserted below. The refusals tested here are the SENTENCE, not the
// enforcement.

// hangSLA is one row the fake table holds.
type hangSLA struct {
	id       string
	loaiViec string
	linhVuc  any // nil = the default row, as PostgreSQL hands back a NULL
	gio      [5]int
}

// khoSLAGia is the fake table plus the recording tape.
type khoSLAGia struct {
	mu   sync.Mutex
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	// hang is what a SELECT answers. Tests seed it to model a commune that is empty, partly
	// configured, or fully configured with EDITED figures.
	hang []hangSLA

	loi error

	// loiSau fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET `loi`: failing everything cannot tell "rolled back" from "never started".
	// The invariant worth proving is that a failure on the LAST statement of a transaction — the
	// audit entry — takes the business writes down with it, and that needs everything before it to
	// have already run.
	loiSau string
	daNo   bool
}

func (k *khoSLAGia) ghi(q string, args []driver.NamedValue) {
	k.mu.Lock()
	k.lenh = append(k.lenh, lenhGhi{sql: q, args: giaTri(args)})
	k.mu.Unlock()
}

// cau returns every recorded statement containing `tu`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (k *khoSLAGia) cau(tu string) []lenhGhi {
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

func (k *khoSLAGia) soCau(tu string) int  { return len(k.cau(tu)) }
func (k *khoSLAGia) coCau(tu string) bool { return k.soCau(tu) > 0 }

func (k *khoSLAGia) kiemLoi(q string) error {
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

func (k *khoSLAGia) Connect(context.Context) (driver.Conn, error) { return &connSLAGia{k: k}, nil }
func (k *khoSLAGia) Driver() driver.Driver                        { return trinhGia{} }

type connSLAGia struct{ k *khoSLAGia }

func (c *connSLAGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connSLAGia) Close() error { return nil }

func (c *connSLAGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connSLAGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	return &txSLAGia{k: c.k}, nil
}

func (c *connSLAGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	// ONE ROW AFFECTED, ALWAYS. store.doiMotDongSLA turns zero into "row not found", and a fake
	// returning zero would make every update look like a missing row — hiding the case this
	// actually tests.
	return driver.RowsAffected(1), nil
}

func (c *connSLAGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	if !strings.Contains(q, "FROM sla") {
		return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
	}

	c.k.mu.Lock()
	defer c.k.mu.Unlock()

	// FOR UPDATE reads ONE row by id — $2. Everything else reads the whole table.
	//
	// THE id IS TAKEN FROM THE BOUND PARAMETER AND NOT FROM THE FIXTURE, so a store that stopped
	// binding it, or bound it in the wrong position, hands back the wrong row here rather than
	// quietly passing.
	if strings.Contains(q, "FOR UPDATE") {
		var id string
		if len(args) >= 2 {
			id, _ = args[1].Value.(string)
		}
		for _, h := range c.k.hang {
			if h.id == id {
				return &rowsGia{cot: cotSLAGia(), hang: [][]driver.Value{hangRaSLA(h)}}, nil
			}
		}
		return &rowsGia{cot: cotSLAGia()}, nil
	}

	ra := make([][]driver.Value, 0, len(c.k.hang))
	for _, h := range c.k.hang {
		ra = append(ra, hangRaSLA(h))
	}
	return &rowsGia{cot: cotSLAGia(), hang: ra}, nil
}

func hangRaSLA(h hangSLA) []driver.Value {
	return []driver.Value{
		h.id, h.loaiViec, h.linhVuc,
		int64(h.gio[0]), int64(h.gio[1]), int64(h.gio[2]), int64(h.gio[3]), int64(h.gio[4]),
	}
}

// cotSLAGia mirrors idstore's cotSLA ORDER. Written out here rather than imported so that
// reordering the store's list without reordering its Scan turns this red too — and that list has
// FIVE ADJACENT INTEGER COLUMNS whose transposition produces no error at all, only a different
// promise to a citizen (migration 0008 names it as this table's defect class).
func cotSLAGia() []string {
	return []string{
		"id", "loai_viec", "linh_vuc",
		"gio_tiep_nhan", "gio_xu_ly_xong", "gio_sap_den_han",
		"gio_bao_lanh_dao", "gio_bao_chu_tich",
	}
}

type txSLAGia struct{ k *khoSLAGia }

func (t *txSLAGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.mu.Unlock()
	return nil
}

func (t *txSLAGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.mu.Unlock()
	return nil
}

// --- fixtures -----------------------------------------------------------------------------------

const (
	xaSLA      = tenant.ID("01JSLA00000000000000000000")
	maCanBoSLA = "CB-0042"
	idDongSLA  = "01JDONGSLAVUADOC000000000X"
)

// idGieo mints a distinct, recognisable id per seeded row, so an assertion can say WHICH row an
// INSERT was for without depending on randomness.
func idGieo(n int) string { return fmt.Sprintf("01JGIEO%019d", n) }

// nguoiSLA is the actor. Ma is the STAFF CODE — rule 6, invariant 8 — and ID is the internal id the
// use case would decide on. They are DIFFERENT VALUES on purpose: a test whose two identifiers are
// equal cannot tell which one reached `audit_log.actor_id`.
func nguoiSLA() NguoiThucHien {
	return NguoiThucHien{
		ID:  "nd-01JNOIBOCUANGUOISUA",
		Vet: audit.Actor{ID: maCanBoSLA, Kind: "staff", IP: "10.0.0.7"},
	}
}

// dungUseCaseSLA builds the REAL use case over the REAL store over the fake driver, with the id
// generator pinned so assertions can name the rows it writes.
func dungUseCaseSLA(t *testing.T, k *khoSLAGia) (*SLA, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind both, exactly as cmd/server wires it: the transaction the use case opens
	// is the transaction the store writes in, and two handles would be two pools.
	kho := store.New(db)
	uc := NewSLA(kho, idstore.NewSLAStore(kho))

	var n int
	uc.sinhID = func() (string, error) {
		n++
		return idGieo(n), nil
	}
	return uc, tenant.Into(context.Background(), xaSLA)
}

// bangDayDu is a commune whose table already holds every seed row, with ONE FIGURE EDITED — the
// `van-ban-den` row's `gio_xu_ly_xong` changed from the seed's 40 to 24.
//
// THAT EDIT IS THE WHOLE POINT OF THE FIXTURE. It is the value a second seeding run must not touch,
// and it is deliberately a number a commune would plausibly choose.
func bangDayDu() []hangSLA {
	var ra []hangSLA
	for i, g := range domain.BoGieoSLA() {
		var lv any
		if g.LinhVuc != "" {
			lv = g.LinhVuc
		}
		gio := [5]int{g.GioTiepNhan, g.GioXuLyXong, g.GioSapDenHan, g.GioBaoLanhDao, g.GioBaoChuTich}
		if g.LoaiViec == domain.LoaiViecVanBanDen && g.LinhVuc == "" {
			gio[1] = gioXaDaSua
		}
		ra = append(ra, hangSLA{
			id:       fmt.Sprintf("01JCODONG%017d", i),
			loaiViec: string(g.LoaiViec),
			linhVuc:  lv,
			gio:      gio,
		})
	}
	return ra
}

// gioXaDaSua is the figure the commune typed over the seed's 40. Named rather than inlined, so the
// assertion that it survives a second seeding run reads as what it is.
const gioXaDaSua = 24
