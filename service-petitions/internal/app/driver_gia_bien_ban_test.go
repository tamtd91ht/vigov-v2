package app

// A fake database/sql driver for the WRITE path of the meeting-minutes register.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument
// driver_gia_nhiem_vu_test.go makes, and here it protects three properties that ONLY EXIST AS
// RELATIONSHIPS BETWEEN STATEMENTS:
//
//	the minutes, EVERY conclusion on the form and the audit entry are in ONE transaction
//	a refusal leaves the transaction with NOTHING committed
//	the ordinal is minted from a high-water mark read under the PARENT's lock  (§7.2)
//
// A fake store erases exactly that: it would let the conclusions be written in a second transaction,
// or the ordinal be computed from `len(live)`, and stay green.
//
// # WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there
//
// Nothing PostgreSQL does with these statements: not `UNIQUE (tenant_id, bien_ban_id, thu_tu)`, not
// the foreign key from a conclusion to its minutes, not the CHECK constraints of migration 0007, not
// `ho_so_luu_tru_cam_xoa_cung` (the trigger that refuses a hard delete), and not the hash partition
// routing. That half needs a real server — VIGOV_TEST_DSN is unset here — and it is the FLOOR under
// everything asserted in bien_ban_hop_test.go: the refusals tested there are the SENTENCE, not the
// enforcement.

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

	"github.com/vihat/vigov/core/audit"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// khoBienBanGia serves the meeting register and its conclusions.
type khoBienBanGia struct {
	mu   sync.Mutex
	lenh []lenhPhieu // reused from driver_gia_phieu_test.go — same shape, same `trongGiaoDich` field

	dangMo                       bool
	batDau, daCommit, daRollback int

	// bienBan is the register, keyed by INTERNAL id; ketLuan likewise, keyed by conclusion id. Each
	// value is a column -> value map, assembled by NAME from the SELECT list the store itself wrote.
	bienBan map[string]map[string]driver.Value
	ketLuan map[string]map[string]driver.Value

	// thuTuLonNhat is what the high-water-mark query answers. IT IS SET INDEPENDENTLY OF `ketLuan`
	// ON PURPOSE: the real statement counts SOFT-DELETED conclusions, which by definition are not
	// among the rows a live read returns, so a fake deriving one from the other could never express
	// the case §7.2 exists for — a removed ② whose number is never handed out again.
	thuTuLonNhat int64

	// soNVKetLuan answers "how many LIVE tasks point at this conclusion", keyed by conclusion id;
	// soNVBienBan answers the same for the whole meeting. Set independently of any task table,
	// because the fake has none — what is under test is the DECISION taken on the count.
	soNVKetLuan map[string]int64
	soNVBienBan int64

	loi    error
	loiSau string
	daNo   bool

	// loiTrigger makes the first Exec containing this fragment fail the way migration 0012's triggers
	// do: SQLSTATE P0001 with the trigger's own message.
	loiTrigger    string
	loiTriggerMsg string
}

// loiPgGia is a driver error carrying a SQLSTATE, the one method the store reads (pgconn.PgError has
// the same one).
type loiPgGia struct{ ma, msg string }

func (e *loiPgGia) Error() string    { return "ERROR: " + e.msg + " (SQLSTATE " + e.ma + ")" }
func (e *loiPgGia) SQLState() string { return e.ma }

// --- fixtures -------------------------------------------------------------------------------------

const (
	idBBGoc      = "01JBIENBANGOCTRONGTEST000"
	idKLGoc      = "01JKETLUANGOCTRONGTEST000"
	idBBMoiSinh  = "01JBIENBANMOISINHTEST0000"
	soHieuBBGoc  = "31/BB-UBND"
	maChuTriBB   = "CB-00007"
	thuTuKLGoc   = int64(3)
	tenCuocHopBB = "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026"
	noiDungBBGoc = "Toàn văn biên bản: xã bàn về tiến độ tuyến đường thôn Bình Trị."
	noiDungKLGoc = "Giao bộ phận Địa chính rà soát tiến độ tuyến đường, báo cáo trước ngày 20/8."
)

var (
	// mocNgayHopBB is a CALENDAR DAY at UTC midnight — the shape the column holds and the wire
	// format produces. mocTaoBB is an instant, and the two are deliberately different kinds.
	mocNgayHopBB = time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	mocTaoBB     = time.Date(2026, 8, 6, 3, 30, 0, 0, time.UTC)
	// mocThaoTacBB is the use case's clock in these tests — the instant a signature, a soft delete or
	// the no-task mark records.
	mocThaoTacBB = time.Date(2026, 8, 7, 2, 15, 0, 0, time.UTC)
)

// dongBienBanGia is one `bien_ban_hop` row as the driver hands it back.
//
// EVERY VALUE IS DISTINCT AND OF THE RIGHT TYPE, so a Scan wired to the wrong position comes back as
// visibly wrong data rather than as a zero that looks plausible. The three adjacent nullable TEXT
// columns (`so_hieu`, `dia_diem`, `chu_tri_ma`) are the ones that would shift into each other
// without any error at all.
func dongBienBanGia(id string, sua map[string]driver.Value) map[string]driver.Value {
	d := map[string]driver.Value{
		"id":           id,
		"ten_cuoc_hop": tenCuocHopBB,
		"ngay_hop":     mocNgayHopBB,
		"so_hieu":      soHieuBBGoc,
		"dia_diem":     "Phòng họp UBND xã",
		"chu_tri_ma":   maChuTriBB,
		"nguoi_tao_ma": maCanBoThu,
		"tao_luc":      mocTaoBB,
		// migration 0012 — a DRAFT with no secretary, no notice, not a supplement.
		"trang_thai":     domain.TrangThaiBienBanDuThao,
		"ky_luc":         nil,
		"ky_boi_ma":      nil,
		"thu_ky_ma":      nil,
		"tb_so_ky_hieu":  nil,
		"tb_ngay":        nil,
		"bo_sung_cho_id": nil,
		"noi_dung":       noiDungBBGoc,
		"thanh_phan":     []byte(`["CB-00007","Đại diện Mặt trận Tổ quốc xã"]`),
	}
	for k, v := range sua {
		d[k] = v
	}
	return d
}

// dongKetLuanGia is one `ket_luan_hop` row.
func dongKetLuanGia(id, bienBanID string, thuTu int64) map[string]driver.Value {
	return map[string]driver.Value{
		"id":              id,
		"bien_ban_id":     bienBanID,
		"thu_tu":          thuTu,
		"noi_dung":        noiDungKLGoc,
		"tao_luc":         mocTaoBB,
		"khong_phat_sinh": false,
	}
}

// khoBBMau is one meeting carrying one conclusion, ③, with ③ also the highest number ever issued.
func khoBBMau() *khoBienBanGia {
	return &khoBienBanGia{
		bienBan: map[string]map[string]driver.Value{
			idBBGoc: dongBienBanGia(idBBGoc, nil),
		},
		ketLuan: map[string]map[string]driver.Value{
			idKLGoc: dongKetLuanGia(idKLGoc, idBBGoc, thuTuKLGoc),
		},
		thuTuLonNhat: thuTuKLGoc,
	}
}

// --- the driver -------------------------------------------------------------------------------------

func (k *khoBienBanGia) ghi(q string, args []driver.NamedValue) {
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
func (k *khoBienBanGia) cau(tu string) []lenhPhieu {
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

func (k *khoBienBanGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

func (k *khoBienBanGia) kiemLoi(q string) error {
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

func (k *khoBienBanGia) Connect(context.Context) (driver.Conn, error) {
	return &connBBGia{k: k}, nil
}
func (k *khoBienBanGia) Driver() driver.Driver { return trinhBBGia{} }

type trinhBBGia struct{}

func (trinhBBGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả: chỉ dùng Connector")
}

type connBBGia struct{ k *khoBienBanGia }

func (c *connBBGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connBBGia) Close() error { return nil }

func (c *connBBGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connBBGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.dangMo = true
	c.k.mu.Unlock()
	return &txBBGia{k: c.k}, nil
}

func (c *connBBGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (
	driver.Result, error) {

	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	c.k.mu.Lock()
	defer c.k.mu.Unlock()
	if c.k.loiTrigger != "" && strings.Contains(q, c.k.loiTrigger) {
		c.k.loiTrigger = ""
		return nil, &loiPgGia{ma: "P0001", msg: c.k.loiTriggerMsg}
	}
	return driver.RowsAffected(1), nil
}

func (c *connBBGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (
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
	// THE AGGREGATE IS NAMED RATHER THAN PARSED. `cotTrongSelect` splits the SELECT list on commas
	// and `COALESCE(MAX(thu_tu), 0)` carries one, so the generic reader would see two columns where
	// the store scans one. An aggregate answers ONE value by construction.
	case strings.Contains(q, "MAX("):
		return &rowsNVGia{cot: []string{"so"}, hang: [][]driver.Value{{c.k.thuTuLonNhat}}}, nil

	// THE TWO COUNTS, before the table routes: the meeting-wide one carries a subquery FROM
	// ket_luan_hop and would otherwise be answered as a conclusion read.
	case strings.Contains(q, "count(*) FROM nhiem_vu"):
		c.k.mu.Lock()
		defer c.k.mu.Unlock()
		n := c.k.soNVBienBan
		if !strings.Contains(q, "bien_ban_id = $3") {
			n = c.k.soNVKetLuan[fmt.Sprint(gt[2])]
		}
		return &rowsNVGia{cot: []string{"n"}, hang: [][]driver.Value{{n}}}, nil

	case strings.Contains(q, "FROM ket_luan_hop"):
		return c.k.doKetLuan(cot, gt)

	case strings.Contains(q, "FROM bien_ban_hop"):
		return c.k.doBienBan(cot, gt)
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

func (k *khoBienBanGia) doBienBan(cot []string, args []driver.Value) (driver.Rows, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	r, co := k.bienBan[fmt.Sprint(args[1])]
	if !co {
		return &rowsNVGia{cot: cot}, nil
	}
	return dungRows(cot, []map[string]driver.Value{r})
}

// doKetLuan answers the read of ONE conclusion BY THE PAIR `(bien_ban_id, thu_tu)`.
//
// THE PAIR IS MATCHED, NOT JUST THE ORDINAL, and the fake insists on it for the reason the store
// does: every meeting has a ①, so an ordinal alone would let a test pass while the statement read
// somebody else's conclusion.
//
// THE COLUMNS ARE THE ONES THE STATEMENT NAMED, assembled by name from the fixture — so a column the
// store adds and the fixture lacks panics in dungRows instead of scanning a plausible zero.
func (k *khoBienBanGia) doKetLuan(cot []string, args []driver.Value) (driver.Rows, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	for _, r := range k.ketLuan {
		if r["bien_ban_id"] == args[1] && fmt.Sprint(r["thu_tu"]) == fmt.Sprint(args[2]) {
			return dungRows(cot, []map[string]driver.Value{r})
		}
	}
	return &rowsNVGia{cot: cot}, nil
}

type txBBGia struct{ k *khoBienBanGia }

func (t *txBBGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.dangMo = false
	t.k.mu.Unlock()
	return nil
}

func (t *txBBGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.dangMo = false
	t.k.mu.Unlock()
	return nil
}

// --- the task use case this file borrows ------------------------------------------------------------

// taoNhiemVuGia stands in for the TASK register's create use case.
//
// A FAKE HERE AND NOT THE REAL ONE, deliberately: what the split has to prove at this layer is that
// it DELEGATES — with the source pair filled in and the body untouched — and a fake is the only
// thing that can show a call was made at all. That the real use case then writes the task, the
// timeline row and the audit entry in ONE transaction is proved over the real store in
// nhiem_vu_test.go, and proving it twice would mean two places to update when it changes.
//
// THE SOURCE CHECK IS RUN FOR REAL, inside a transaction on the same fake database, exactly where
// GhiNhiemVu.TaoTuNguon runs it (first thing in its transaction). A refusal there means NO task:
// `daTao` stays false. That the real use case writes nothing on such a refusal is proved over the
// task store in nhiem_vu_test.go.
type taoNhiemVuGia struct {
	db *pkgstore.DB

	goi     int
	coKiem  bool
	daTao   bool
	yc      YeuCauTaoNhiemVu
	nguoi   audit.Actor
	loi     error
	loiKiem error

	// truocKhiKiem runs AFTER the split's first read and BEFORE the in-transaction check — another
	// clerk's act landing in exactly the window that check exists for.
	truocKhiKiem func()
}

func (t *taoNhiemVuGia) TaoTuNguon(ctx context.Context, yc YeuCauTaoNhiemVu, nguoi audit.Actor,
	kiem KiemNguonTrongGiaoDich) (domain.NhiemVu, error) {
	t.goi++
	t.yc, t.nguoi = yc, nguoi
	if t.truocKhiKiem != nil {
		t.truocKhiKiem()
	}
	if kiem != nil {
		t.coKiem = true
		if err := t.db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error { return kiem(ctx, tx) }); err != nil {
			t.loiKiem = err
			return domain.NhiemVu{}, err
		}
	}
	if t.loi != nil {
		return domain.NhiemVu{}, t.loi
	}
	t.daTao = true
	return domain.NhiemVu{
		ID: "nv-moi", Ma: "NV20", Loai: yc.Loai, TieuDe: yc.TieuDe,
		TrangThai: domain.MoiGiao, NguonGiao: domain.NguonGiao(yc.NguonGiao), NguonID: yc.NguonID,
	}, nil
}

// --- the harness --------------------------------------------------------------------------------

// dungGhiBienBan builds the REAL use case over the REAL store over the fake driver, with the ids
// pinned so every assertion is about the decision and not about randomness.
func dungGhiBienBan(t *testing.T, k *khoBienBanGia) (*GhiBienBanHop, *taoNhiemVuGia, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and IN ORDER. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	kho := pkgstore.New(db)
	nv := &taoNhiemVuGia{db: kho}
	uc := NewGhiBienBanHop(kho, petstore.NewBienBanHopStore(kho), nv)
	uc.nay = func() time.Time { return mocThaoTacBB }

	// IDS ARE HANDED OUT IN ORDER so a test can name the one it expects: the first is the minutes,
	// the ones after it are the conclusions on the form, in the order they were typed.
	var n int
	uc.sinhID = func() (string, error) {
		n++
		if n == 1 {
			return idBBMoiSinh, nil
		}
		return fmt.Sprintf("01JSINHRATRONGTEST%06d", n), nil
	}
	return uc, nv, ctxXa(xaThu)
}
