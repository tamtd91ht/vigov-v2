package app

// A SECOND fake database/sql driver, for the two document registers.
//
// WHY THE REAL STORE RUNS ON TOP OF IT INSTEAD OF A HAND-WRITTEN FAKE STORE — the same argument
// driver_gia_test.go makes, and it is heavier here because the property being protected is an
// ISSUED NUMBER:
//
//	the business write, the number and the audit entry are in ONE transaction  (rule 6, inv. 3)
//	a refusal leaves the transaction with NOTHING committed — and NO NUMBER CONSUMED
//	the number is read `FOR UPDATE`, so two clerks cannot take the same one  (rule 7, inv. 3)
//	a soft delete never returns the number to the series
//	a new year starts a new series
//	the routing UPDATE and its timeline row are one transaction              (rule 7, forbidden #5)
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// A SECOND DRIVER AND NOT AN EXTENSION OF THE FIRST: that one answers `count(*)` and one catalogue
// row shape. Teaching it three more tables would make every case in either file depend on a branch
// written for the other, and the first surprising interaction would be silent.
//
// # THIS ONE MODELS THE ROW LOCK, AND THAT IS THE POINT OF THE FILE
//
// The other fake drivers in this repository record statements. This one also SERIALISES: a
// `SELECT … FOR UPDATE` on `day_so_van_ban` takes a token that is held until the transaction ends,
// exactly as PostgreSQL holds a row lock under READ COMMITTED. That is what makes
// TestCapSo_HaiLuotSongSongRaHaiSoKhacNhau a real test rather than a description — remove
// `FOR UPDATE` from the store and it goes red, which is the mutation this file exists to survive.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not `UNIQUE (tenant_id, nam, so_vao_so)`, not the
// `so_van_ban_bat_bien` trigger, not `day_so_khong_lui`, not `lich_su_chuyen_chi_them`, not the
// partition routing. That half needs a real server (VIGOV_TEST_DSN is unset here), and it is the
// FLOOR under everything this file asserts — the application refusals tested below are the SENTENCE,
// not the enforcement.

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

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// hangVBD is the row the incoming register's FOR UPDATE read hands back. nil means "no such live
// entry".
type hangVBD struct {
	id           string
	soVaoSo, nam int
	ngayDen      time.Time
	soKyHieu     string
	ngayVanBan   *time.Time
	coQuan       string
	loai         string
	trichYeu     string
	doKhan       string
	boPhan       string
	canBo        string
	han          time.Time
	trangThai    string
	nguoiTao     string
	taoLuc       time.Time
	capNhatLuc   time.Time
}

// hangVBDi is the same for the outgoing register.
type hangVBDi struct {
	id         string
	soDi, nam  int
	ngayVanBan time.Time
	loai       string
	trichYeu   string
	noiNhan    string
	nguoiKy    string
	nguoiTao   string
	taoLuc     time.Time
	capNhatLuc time.Time
}

type khoVBGia struct {
	mu   sync.Mutex
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	hangDen *hangVBD
	hangDi  *hangVBDi

	// hangLichSu answers `SELECT … FROM lich_su_chuyen_van_ban`, IN THE ORDER GIVEN — the fake does
	// not sort, so the ORDER BY is asserted on the statement text, not on the result.
	hangLichSu [][]driver.Value

	// loaiCoTrongDanhMuc answers `SELECT 1 FROM loai_van_ban …`. FALSE IS A REAL CASE: a type the
	// commune retired between the form loading and the clerk pressing save.
	loaiCoTrongDanhMuc bool

	// soCuoi is the number series, keyed "<sổ>|<năm>" — the same key the primary key of
	// `day_so_van_ban` uses, minus the commune, which this fake serves only one of.
	soCuoi map[string]int

	// khoaDay MODELS THE ROW LOCK. Capacity one: whoever holds it is the transaction that ran
	// `SELECT … FOR UPDATE` on the series, and it is returned at Commit or Rollback.
	khoaDay chan struct{}

	// treDocDay is slept at the END of a series read, while the lock (if any) is still held. It
	// makes the race window DETERMINISTIC rather than a matter of scheduling: with `FOR UPDATE` the
	// second reader is still blocked and the delay costs nothing, and without it both readers get
	// the same value every single run. A test of a race that only sometimes races is a test that
	// goes green on the day it matters.
	treDocDay time.Duration

	loi    error
	loiSau string
	daNo   bool
}

func khoVBMau() *khoVBGia {
	return &khoVBGia{
		loaiCoTrongDanhMuc: true,
		soCuoi:             map[string]int{},
		khoaDay:            make(chan struct{}, 1),
	}
}

func (k *khoVBGia) ghi(q string, args []driver.NamedValue) {
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
func (k *khoVBGia) cau(tu string) []lenhGhi {
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

func (k *khoVBGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

func (k *khoVBGia) kiemLoi(q string) error {
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

func (k *khoVBGia) Connect(context.Context) (driver.Conn, error) { return &connVBGia{k: k}, nil }
func (k *khoVBGia) Driver() driver.Driver                        { return trinhGia{} }

type connVBGia struct {
	k  *khoVBGia
	tx *txVBGia // the transaction currently open on THIS connection, if any
}

func (c *connVBGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả: không hỗ trợ Prepare")
}
func (c *connVBGia) Close() error { return nil }

func (c *connVBGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connVBGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.mu.Lock()
	c.k.batDau++
	c.k.mu.Unlock()
	t := &txVBGia{k: c.k, c: c}
	c.tx = t
	return t, nil
}

func (c *connVBGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}
	// THE GUARD BELOW IS NOT DEFENSIVE PADDING. A statement against the series with a DIFFERENT
	// parameter list is exactly what the mutation "trả số về dãy" looks like, and without this the
	// fake panics on an index — which turns a test that CAUGHT a defect into a test that crashed.
	// A crash and a failure read the same in CI and do not read the same to a person.
	if len(args) < 4 && strings.Contains(q, "day_so_van_ban") {
		return driver.RowsAffected(1), nil
	}
	switch {
	case strings.Contains(q, "INSERT INTO day_so_van_ban"):
		// ON CONFLICT DO NOTHING — the row is created once and never overwritten.
		k := khoaDay(args[1].Value, args[2].Value)
		c.k.mu.Lock()
		if _, co := c.k.soCuoi[k]; !co {
			c.k.soCuoi[k] = 0
		}
		c.k.mu.Unlock()
	case strings.Contains(q, "UPDATE day_so_van_ban"):
		k := khoaDay(args[1].Value, args[2].Value)
		c.k.mu.Lock()
		c.k.soCuoi[k] = int(args[3].Value.(int64))
		c.k.mu.Unlock()
	}
	// ONE ROW AFFECTED, ALWAYS. store.doiMotDongVanBan turns zero into "not found", and a fake
	// returning zero would make every update look like a missing document — hiding the case this
	// actually tests.
	return driver.RowsAffected(1), nil
}

// khoaDay builds the series key from the two bound parameters, exactly as the primary key does.
func khoaDay(soSach, nam driver.Value) string {
	return fmt.Sprintf("%v|%v", soSach, nam)
}

func (c *connVBGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.ghi(q, args)
	if err := c.k.kiemLoi(q); err != nil {
		return nil, err
	}

	switch {
	case strings.Contains(q, "FROM day_so_van_ban"):
		// THE LOCK IS TAKEN ONLY IF THE STATEMENT ASKS FOR IT. That is the whole mechanism: remove
		// `FOR UPDATE` from the store and this branch stops serialising, which is what the
		// concurrency test observes.
		if strings.Contains(q, "FOR UPDATE") && c.tx != nil && !c.tx.giuKhoa {
			c.k.khoaDay <- struct{}{}
			c.tx.giuKhoa = true
		}
		k := khoaDay(args[1].Value, args[2].Value)
		c.k.mu.Lock()
		so := c.k.soCuoi[k]
		c.k.mu.Unlock()
		if c.k.treDocDay > 0 {
			time.Sleep(c.k.treDocDay)
		}
		return &rowsGia{cot: []string{"so_cuoi"}, hang: [][]driver.Value{{int64(so)}}}, nil

	case strings.Contains(q, "FROM loai_van_ban"):
		if !c.k.loaiCoTrongDanhMuc {
			return &rowsGia{cot: []string{"?column?"}}, nil
		}
		return &rowsGia{cot: []string{"?column?"}, hang: [][]driver.Value{{int64(1)}}}, nil

	case strings.Contains(q, "FROM lich_su_chuyen_van_ban"):
		return &rowsGia{cot: cotLSC(), hang: c.k.hangLichSu}, nil

	case strings.Contains(q, "FROM van_ban_den"):
		if c.k.hangDen == nil {
			return &rowsGia{cot: cotVBD()}, nil
		}
		h := *c.k.hangDen
		var ngayVB driver.Value
		if h.ngayVanBan != nil {
			ngayVB = *h.ngayVanBan
		}
		return &rowsGia{cot: cotVBD(), hang: [][]driver.Value{{
			h.id, int64(h.soVaoSo), int64(h.nam), h.ngayDen, h.soKyHieu, ngayVB,
			h.coQuan, h.loai, h.trichYeu, h.doKhan, h.boPhan, h.canBo,
			h.han, h.trangThai, h.nguoiTao, h.taoLuc, h.capNhatLuc,
		}}}, nil

	case strings.Contains(q, "FROM van_ban_di"):
		if c.k.hangDi == nil {
			return &rowsGia{cot: cotVBDi()}, nil
		}
		h := *c.k.hangDi
		return &rowsGia{cot: cotVBDi(), hang: [][]driver.Value{{
			h.id, int64(h.soDi), int64(h.nam), h.ngayVanBan, h.loai, h.trichYeu,
			h.noiNhan, h.nguoiKy, h.nguoiTao, h.taoLuc, h.capNhatLuc,
		}}}, nil
	}
	return nil, fmt.Errorf("driver giả: không biết trả gì cho %q", q)
}

// cotVBD mirrors docstore's cotVanBanDen ORDER. Written out here rather than imported so that
// reordering the store's list without reordering its Scan turns this red too — and that list holds
// `ngay_den` and `ngay_van_ban` side by side, a swap that produces no error and reads as a document
// received before it was signed.
func cotVBD() []string {
	return []string{
		"id", "so_vao_so", "nam", "ngay_den", "so_ky_hieu", "ngay_van_ban",
		"co_quan_ban_hanh", "loai_van_ban", "trich_yeu", "do_khan",
		"bo_phan_dang_giu_id", "can_bo_xu_ly_ma",
		"han_xu_ly_xong", "trang_thai", "nguoi_tao_ma", "tao_luc", "cap_nhat_luc",
	}
}

// cotLSC mirrors docstore's cotLichSuChuyen ORDER — `tu_bo_phan_id` and `den_bo_phan_id` are
// adjacent TEXT columns, and a swap reads as the file travelling backwards.
func cotLSC() []string {
	return []string{
		"id", "van_ban_den_id", "thoi_diem", "nguoi_ma", "trang_thai_tai_thoi_diem",
		"tu_bo_phan_id", "den_bo_phan_id", "can_bo_xu_ly_ma", "noi_dung", "tao_luc",
	}
}

// cotVBDi mirrors docstore's cotVanBanDi ORDER — `trich_yeu`, `noi_nhan` and `nguoi_ky` are three
// adjacent TEXT columns, and a swap there makes a document read as having been sent to the person
// who signed it.
func cotVBDi() []string {
	return []string{
		"id", "so_di", "nam", "ngay_van_ban", "loai_van_ban", "trich_yeu",
		"noi_nhan", "nguoi_ky", "nguoi_tao_ma", "tao_luc", "cap_nhat_luc",
	}
}

type txVBGia struct {
	k       *khoVBGia
	c       *connVBGia
	giuKhoa bool
}

func (t *txVBGia) ketThuc() {
	if t.giuKhoa {
		<-t.k.khoaDay
		t.giuKhoa = false
	}
	t.c.tx = nil
}

func (t *txVBGia) Commit() error {
	t.k.mu.Lock()
	t.k.daCommit++
	t.k.mu.Unlock()
	t.ketThuc()
	return nil
}

func (t *txVBGia) Rollback() error {
	t.k.mu.Lock()
	t.k.daRollback++
	t.k.mu.Unlock()
	t.ketThuc()
	return nil
}

// --- the identity stand-in ------------------------------------------------------------------------

// hanGia answers ResolveDeadlines. IT RECORDS WHAT WAS ASKED FOR, because the three arguments are
// decisions ADR 0028 makes about this act: which work kind, which field row, which clock.
type hanGia struct {
	// mu GIỮ BỐN TRƯỜNG GHI DƯỚI ĐÂY, và nó có mặt vì một ca kiểm thật chứ không phòng xa.
	//
	// `TestCapSo_HaiLuotSongSongRaHaiSoKhacNhau` cố ý chạy HAI goroutine qua cùng một use case
	// để chứng minh khoá dòng của sổ số hoạt động. Cả hai đi qua `HanXuLy`, nên bốn phép gán
	// dưới là ghi-ghi đồng thời trên một bộ đếm không khoá — `go test -race` bắt đúng chỗ ấy.
	//
	// ĐÂY LÀ LỖI CỦA BỘ ĐỒ THỬ, KHÔNG PHẢI CỦA MÃ NGHIỆP VỤ. Nhưng nó làm `make check` ĐỎ, và
	// một cổng đỏ vì lý do không ai sửa là cổng người ta học cách bỏ qua — rồi lần đỏ thật
	// tiếp theo cũng trôi qua cùng cách. Khoá ở đây rẻ hơn hẳn cái giá ấy.
	//
	// CHỈ KHOÁ ĐƯỜNG GHI. Các ca khác đọc thẳng `han.goi`, `han.loai`, … sau khi goroutine đã
	// kết thúc, nên chúng happens-after và không cần khoá; bọc thêm accessor chỉ để "cho đủ
	// đối xứng" sẽ bắt sửa mười chỗ đang đúng.
	mu   sync.Mutex
	tra  time.Time
	loi  error
	goi  int
	loai identityv1.WorkKind
	linh string
	tu   time.Time
	can  []identityv1.DeadlineKind
}

func (h *hanGia) HanXuLy(_ context.Context, loaiViec identityv1.WorkKind, linhVuc string,
	tuLuc time.Time, can []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error) {

	h.mu.Lock()
	h.goi++
	h.loai, h.linh, h.tu, h.can = loaiViec, linhVuc, tuLuc, can
	h.mu.Unlock()
	if h.loi != nil {
		return nil, h.loi
	}
	ra := map[identityv1.DeadlineKind]time.Time{}
	for _, k := range can {
		ra[k] = h.tra
	}
	return ra, nil
}

// --- wiring ---------------------------------------------------------------------------------------

// lucVaoSo is the instant every booking below is stamped with. FIXED rather than time.Now(), because
// it becomes the ORIGIN the commitment is counted from and the YEAR of the series — an assertion
// about either would otherwise be an assertion about when the suite ran.
var lucVaoSo = time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC)

// hanMau is what identity answers with: 40 working hours after lucVaoSo, as that commune's calendar
// would place them. The VALUE is arbitrary; what matters is that it comes from identity and is
// stored unchanged.
var hanMau = time.Date(2026, 9, 29, 3, 30, 0, 0, time.UTC)

const idMoiVanBan = "01JVANBANMOIVUATAO0000000"

// dungUseCaseVanBanDen builds the REAL use case over the REAL store over the fake driver.
//
// `soKetNoi` IS A PARAMETER because the concurrency test needs two connections while every other
// test needs exactly one: with one connection every statement of one transaction lands on the same
// fake in order, and a pool would make the ordering assertions about the scheduler instead.
func dungUseCaseVanBanDen(t *testing.T, k *khoVBGia, soKetNoi int) (*VanBanDen, *hanGia, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(soKetNoi)
	t.Cleanup(func() { db.Close() })

	// ONE *store.DB behind all of them, exactly as cmd/server wires it: the transaction the use case
	// opens is the transaction the stores write in, and two handles would be two pools.
	kho := store.New(db)
	han := &hanGia{tra: hanMau}
	uc := NewVanBanDen(kho, docstore.NewVanBanDenStore(kho), docstore.NewDaySoStore(kho), han)
	uc.sinhID = func() (string, error) { return idMoiVanBan, nil }
	uc.nay = func() time.Time { return lucVaoSo }
	return uc, han, tenant.Into(context.Background(), xaA)
}

func dungUseCaseVanBanDi(t *testing.T, k *khoVBGia, soKetNoi int) (*VanBanDi, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(soKetNoi)
	t.Cleanup(func() { db.Close() })

	kho := store.New(db)
	uc := NewVanBanDi(kho, docstore.NewVanBanDiStore(kho), docstore.NewDaySoStore(kho))
	uc.sinhID = func() (string, error) { return idMoiVanBan, nil }
	uc.nay = func() time.Time { return lucVaoSo }
	return uc, tenant.Into(context.Background(), xaA)
}
