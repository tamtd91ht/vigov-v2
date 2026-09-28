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
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// A fake database/sql driver for the org chart, so the REAL use case runs over the REAL store over a
// REAL transaction boundary — the argument driver_gia_sla_test.go makes, for the same reasons: the
// properties worth proving (one transaction for the write and its trail, the lock walk, which
// commune each statement binds) are properties of the SQL, and a fake store erases them.
//
// IT ANSWERS THE WAY A DATABASE WOULD, FROM THE STATEMENT IT IS GIVEN. A lock read that carries
// `tenant_id = $1 AND id = $2` is filtered by commune; one that does not is answered across EVERY
// commune, exactly as PostgreSQL would answer `WHERE id = $1`. That is what lets a test turn red when
// the commune is dropped from the parent lookup, rather than the fake quietly re-adding it.
//
// WHAT IT DOES NOT PROVE: the unique key, the foreign key, FOR UPDATE really serialising two
// sessions. That half is bo_phan_pg_test.go in the store package, skipped without VIGOV_TEST_DSN.

type hangBoPhan struct {
	xa    tenant.ID
	id    string
	ma    string
	ten   string
	cha   string
	thuTu int
	daXoa bool
}

type khoBoPhanGia struct {
	mu   sync.Mutex
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	hang []hangBoPhan

	// loiSau fails the FIRST statement containing this substring — see khoSLAGia.loiSau.
	loiSau string
	daNo   bool

	// staff is the live-staff count per unit id, as the delete's counts see it. staffUnderLock, when
	// set, replaces it for the count taken INSIDE the transaction — a member of staff added between
	// the first read and the lock. deletedAtLock makes the lock see the unit already soft-deleted.
	staff          map[string]int
	staffUnderLock map[string]int
	deletedAtLock  bool
}

// childrenOf counts the LIVE units of commune xa whose parent is id — the predicate the store's
// subquery states. Caller holds k.mu.
func (k *khoBoPhanGia) childrenOf(xa tenant.ID, id string) int {
	n := 0
	for _, h := range k.hang {
		if h.xa == xa && h.cha == id && !h.daXoa {
			n++
		}
	}
	return n
}

func (k *khoBoPhanGia) ghi(q string, args []driver.NamedValue) {
	k.mu.Lock()
	k.lenh = append(k.lenh, lenhGhi{sql: q, args: giaTri(args)})
	k.mu.Unlock()
}

func (k *khoBoPhanGia) cau(tu string) []lenhGhi {
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

func (k *khoBoPhanGia) kiemLoi(q string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.loiSau != "" && !k.daNo && strings.Contains(q, k.loiSau) {
		k.daNo = true
		return errors.New("driver giả: câu lệnh này được dựng để hỏng")
	}
	return nil
}

func (k *khoBoPhanGia) Connect(context.Context) (driver.Conn, error) {
	return &connBoPhanGia{k: k}, nil
}
func (k *khoBoPhanGia) Driver() driver.Driver { return trinhGia{} }

type connBoPhanGia struct{ k *khoBoPhanGia }

func (c *connBoPhanGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connBoPhanGia) Close() error { return nil }
func (c *connBoPhanGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connBoPhanGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	return &txBoPhanGia{k: c.k}, nil
}

func (c *connBoPhanGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func chuoiThu(args []driver.NamedValue, i int) string {
	if i >= len(args) {
		return ""
	}
	s, _ := args[i].Value.(string)
	return s
}

func (c *connBoPhanGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	c.k.mu.Lock()
	defer c.k.mu.Unlock()

	switch {
	case strings.Contains(q, "SELECT bp.id, bp.ma") && strings.Contains(q, "count(*)"):
		// The delete's first read (idstore.BoPhanStore.LiveForDelete): one LIVE unit of the commune
		// at $1, with its counts. Refused outright if the statement stops naming the commune.
		if !strings.Contains(q, "bp.tenant_id = $1 AND bp.id = $2 AND bp.deleted_at IS NULL") {
			return nil, fmt.Errorf("driver giả: lượt đọc để xoá không lọc theo xã hay theo dòng sống: %q", q)
		}
		xa, id := tenant.ID(chuoiThu(args, 0)), chuoiThu(args, 1)
		cot := []string{"id", "ma", "ten", "cha_id", "thu_tu", "staff", "children"}
		for _, h := range c.k.hang {
			if h.id == id && h.xa == xa && !h.daXoa {
				return &rowsGia{cot: cot, hang: [][]driver.Value{
					{h.id, h.ma, h.ten, h.cha, int64(h.thuTu), int64(c.k.staff[id]), int64(c.k.childrenOf(xa, id))},
				}}, nil
			}
		}
		return &rowsGia{cot: cot}, nil

	case strings.HasPrefix(q, "SELECT (SELECT count(*)"):
		// The re-count under the lock (idstore.BoPhanStore.LocalHoldings).
		if !strings.Contains(q, "bp.tenant_id = $1 AND bp.id = $2") {
			return nil, fmt.Errorf("driver giả: đếm lại không lọc theo xã: %q", q)
		}
		xa, id := tenant.ID(chuoiThu(args, 0)), chuoiThu(args, 1)
		staff := c.k.staff
		if c.k.staffUnderLock != nil {
			staff = c.k.staffUnderLock
		}
		return &rowsGia{cot: []string{"staff", "children"}, hang: [][]driver.Value{
			{int64(staff[id]), int64(c.k.childrenOf(xa, id))},
		}}, nil

	case strings.Contains(q, "FOR UPDATE") && strings.Contains(q, "FROM bo_phan"):
		cot := []string{"id", "ma", "ten", "cha_id", "thu_tu", "da_xoa"}
		// Answer as the statement asks: scoped when it names the commune at $1, unscoped otherwise.
		theoXa := strings.Contains(q, "tenant_id = $1 AND id = $2")
		xa, id := tenant.ID(chuoiThu(args, 0)), chuoiThu(args, 1)
		if !theoXa {
			id = chuoiThu(args, 0)
		}
		for _, h := range c.k.hang {
			if h.id == id && (!theoXa || h.xa == xa) {
				return &rowsGia{cot: cot, hang: [][]driver.Value{
					{h.id, h.ma, h.ten, h.cha, int64(h.thuTu), h.daXoa || c.k.deletedAtLock},
				}}, nil
			}
		}
		return &rowsGia{cot: cot}, nil

	case strings.Contains(q, "FROM bo_phan") && strings.Contains(q, "ORDER BY thu_tu, ten, id"):
		// The import's snapshot (idstore.BoPhanStore.ImportSnapshot): every row of the commune at $1,
		// soft-deleted ones included and flagged. Only the commune bound at $1 is answered, so a
		// snapshot that dropped the commune filter would see nothing of this one's chart.
		xa := tenant.ID(chuoiThu(args, 0))
		if !strings.Contains(q, "tenant_id = $1") {
			return nil, fmt.Errorf("driver giả: ảnh chụp không lọc theo xã: %q", q)
		}
		var ra [][]driver.Value
		for _, h := range c.k.hang {
			if h.xa == xa {
				ra = append(ra, []driver.Value{h.id, h.ma, h.ten, h.cha, h.daXoa})
			}
		}
		return &rowsGia{cot: []string{"id", "ma", "ten", "cha_id", "da_xoa"}, hang: ra}, nil

	case strings.Contains(q, "SELECT ma FROM bo_phan"):
		xa, goc := tenant.ID(chuoiThu(args, 0)), chuoiThu(args, 1)
		var ra [][]driver.Value
		for _, h := range c.k.hang {
			// Deleted rows included, as the unique key counts them.
			if h.xa == xa && (h.ma == goc || strings.HasPrefix(h.ma, goc+"-")) {
				ra = append(ra, []driver.Value{h.ma})
			}
		}
		return &rowsGia{cot: []string{"ma"}, hang: ra}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

type txBoPhanGia struct{ k *khoBoPhanGia }

func (t *txBoPhanGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.mu.Unlock()
	return nil
}

func (t *txBoPhanGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.mu.Unlock()
	return nil
}

// --- fixtures -----------------------------------------------------------------------------------

const (
	xaBoPhan      = tenant.ID("01JBOPHANXAA0000000000000A")
	xaBoPhanKhac  = tenant.ID("01JBOPHANXAB0000000000000B")
	maCanBoBoPhan = "CB-0077"
	idBoPhanMoi   = "01JBOPHANMOI00000000000000"
)

// nguoiBoPhan — Ma and ID DIFFERENT on purpose, so a test can tell which reached `actor_id`.
func nguoiBoPhan() NguoiThucHien {
	return NguoiThucHien{
		ID:  "nd-01JNOIBOBOPHAN",
		Vet: audit.Actor{ID: maCanBoBoPhan, Kind: "staff", IP: "10.0.0.7"},
	}
}

func dungUseCaseBoPhan(t *testing.T, k *khoBoPhanGia) (*SoDoToChuc, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	uc := NewSoDoToChuc(kho, idstore.NewBoPhanStore(kho))
	uc.sinhID = func() (string, error) { return idBoPhanMoi, nil }
	return uc, tenant.Into(context.Background(), xaBoPhan)
}

// cayBaTang is A → B → C in commune A, plus one unit in commune B and one soft-deleted unit in A.
func cayBaTang() []hangBoPhan {
	return []hangBoPhan{
		{xa: xaBoPhan, id: "bp-a", ma: "lanh-dao", ten: "LÃNH ĐẠO", thuTu: 1},
		{xa: xaBoPhan, id: "bp-b", ma: "van-phong", ten: "VĂN PHÒNG", cha: "bp-a", thuTu: 2},
		{xa: xaBoPhan, id: "bp-c", ma: "to-mot-cua", ten: "TỔ MỘT CỬA", cha: "bp-b", thuTu: 3},
		{xa: xaBoPhan, id: "bp-xoa", ma: "da-xoa", ten: "ĐÃ XOÁ", thuTu: 9, daXoa: true},
		{xa: xaBoPhanKhac, id: "bp-khac", ma: "van-phong-hdnd", ten: "VĂN PHÒNG HĐND XÃ B"},
	}
}
