package app

// The use cases behind the Mini App content write routes, run over the REAL store over a FAKE
// DRIVER.
//
// WHY A DRIVER AND NOT A FAKE STORE — the same argument driver_gia_danh_muc_test.go and
// thong_bao_noi_bo_test.go both make, and it holds hardest here because §10.4 is a rule about what
// a WRITE puts in a column. What this package is responsible for is not "the use case called a
// method", it is:
//
//	the row AND the audit entry are in ONE transaction (rule 6, invariant 3)
//	a refusal writes NOTHING and commits nothing
//	the state, the author and the provenance are decided here and never taken from the request
//	§10.4 — editing a SYNCED item sets `da_sua_tay`, and a no-op edit does not
//	the audit actor is the staff BUSINESS CODE and the article's text never enters the ledger
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not the CHECK constraints, not the immutability
// trigger of migration 0006, not the foreign key, not the unique key on `nguon_id_ngoai`, not
// partition routing. That half needs a real server; the pg suite in internal/store is where it lands
// the day a DSN exists.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// khoNDGia records every statement and every transaction boundary, and answers the four reads these
// use cases make: the locked row, the category probe, the parent probe and the two counts.
type khoNDGia struct {
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	// dongHienCo is what TheoIDDeSua finds. nil means "no such row".
	dongHienCo *domain.NoiDungMiniApp

	// coDong answers both existence probes (`SELECT 1 …`).
	coDong bool

	// TWO COUNTERS FOR TWO `count(*)` STATEMENTS, and merging them is exactly the mistake this
	// comment exists to stop. `DemDangSong` counts the commune's LIVE categories for the ceiling,
	// while `SlugDaDung` counts rows carrying one slug INCLUDING soft-deleted ones. A single field
	// makes "the commune has three categories" also mean "this slug is taken three times", so the
	// happy path of ThemDanhMuc can never be reached and the ceiling case passes for the wrong reason.
	dem     int64 // DemDangSong
	demSlug int64 // SlugDaDung

	// loiSau fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET ERROR: failing everything cannot tell "rolled back" from "never started". The
	// invariant worth proving is that a failure on the LAST statement of the transaction — the audit
	// entry — takes the business write down with it, and that needs everything before it to have
	// already run.
	loiSau string
	daNo   bool

	// ADR 0067 §3 — what the category edit / delete reads find. category is the locked row (nil = none);
	// ancestors is what the recursive walk returns; liveChildren / liveItems answer the two refusal counts.
	category                *domain.DanhMucMiniApp
	ancestors               []string
	liveChildren, liveItems int64
}

func (k *khoNDGia) Connect(context.Context) (driver.Conn, error) { return &connNDGia{k: k}, nil }
func (k *khoNDGia) Driver() driver.Driver                        { return trinhNDGia{} }

type trinhNDGia struct{}

func (trinhNDGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả nội dung Mini App: chỉ dùng Connector")
}

type connNDGia struct{ k *khoNDGia }

func (c *connNDGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả nội dung Mini App: không hỗ trợ Prepare")
}
func (c *connNDGia) Close() error { return nil }

func (c *connNDGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connNDGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.batDau++
	return &txNDGia{k: c.k}, nil
}

func (c *connNDGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.k.lenh = append(c.k.lenh, lenhGhi{sql: q, args: gt})
	if c.k.loiSau != "" && !c.k.daNo && strings.Contains(q, c.k.loiSau) {
		c.k.daNo = true
		return nil, errors.New("driver giả nội dung Mini App: câu lệnh này được dựng để hỏng")
	}
	return driver.RowsAffected(1), nil
}

func (c *connNDGia) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.k.lenh = append(c.k.lenh, lenhGhi{sql: q, args: gt})

	switch {
	case strings.Contains(q, "count(*) FROM noi_dung_mini_app"):
		return &rowsNDGia{cot: []string{"count"}, hang: [][]driver.Value{{c.k.liveItems}}}, nil
	case strings.Contains(q, "count(*)") && strings.Contains(q, "cha_id = $2"):
		return &rowsNDGia{cot: []string{"count"}, hang: [][]driver.Value{{c.k.liveChildren}}}, nil
	case strings.Contains(q, "WITH RECURSIVE up"):
		hang := make([][]driver.Value, 0, len(c.k.ancestors))
		for _, id := range c.k.ancestors {
			hang = append(hang, []driver.Value{id})
		}
		return &rowsNDGia{cot: []string{"id"}, hang: hang}, nil
	case strings.Contains(q, "FROM danh_muc_mini_app") && strings.Contains(q, "FOR UPDATE"):
		cot := []string{"id", "ten", "slug", "cha_id", "thu_tu", "tao_luc", "hidden"}
		if c.k.category == nil {
			return &rowsNDGia{cot: cot}, nil
		}
		dm := *c.k.category
		var cha driver.Value
		if dm.ChaID != "" {
			cha = dm.ChaID
		}
		return &rowsNDGia{cot: cot, hang: [][]driver.Value{{dm.ID, dm.Ten, dm.Slug, cha,
			int64(dm.ThuTu), lucNDPinned, dm.Hidden}}}, nil
	case strings.Contains(q, "count(*)") && strings.Contains(q, "slug = $2"):
		return &rowsNDGia{cot: []string{"count"}, hang: [][]driver.Value{{c.k.demSlug}}}, nil
	case strings.Contains(q, "count(*)"):
		return &rowsNDGia{cot: []string{"count"}, hang: [][]driver.Value{{c.k.dem}}}, nil
	case strings.Contains(q, "SELECT 1 FROM"):
		if !c.k.coDong {
			return &rowsNDGia{cot: []string{"?column?"}}, nil
		}
		return &rowsNDGia{cot: []string{"?column?"}, hang: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(q, "FOR UPDATE"):
		if c.k.dongHienCo == nil {
			return &rowsNDGia{cot: cotChiTietND()}, nil
		}
		return &rowsNDGia{cot: cotChiTietND(),
			hang: [][]driver.Value{hangTu(*c.k.dongHienCo)}}, nil
	}
	return nil, errors.New("driver giả nội dung Mini App: truy vấn ngoài dự kiến — " + q)
}

// cotChiTietND is the detail column list, IN THE STORE'S OWN ORDER. It is written out rather than
// parsed out of the statement so that a reorder in the store without a matching reorder in the scan
// lands the wrong value in the wrong field HERE, which is where a test can see it.
func cotChiTietND() []string {
	return []string{"id", "loai", "danh_muc_id", "tieu_de", "tom_tat", "anh_dai_dien_url",
		"ngay_dang", "luot_xem", "trang_thai", "nguon", "nguon_url", "nguon_id_ngoai",
		"da_sua_tay", "nguoi_tao_ma", "tao_luc", "cap_nhat_luc",
		"published_at", "event_starts_at", "event_ends_at", "event_place", "video_url",
		"cover_image_file_id", "link_to", "display_order", "noi_dung"}
}

func hangTu(n domain.NoiDungMiniApp) []driver.Value {
	rong := func(s string) driver.Value {
		if s == "" {
			return nil
		}
		return s
	}
	instant := func(t time.Time) driver.Value {
		if t.IsZero() {
			return nil
		}
		return t
	}
	return []driver.Value{
		n.ID, string(n.Loai), rong(n.DanhMucID), n.TieuDe, rong(n.TomTat), rong(n.AnhDaiDienURL),
		n.NgayDang, int64(n.LuotXem), string(n.TrangThai), string(n.Nguon),
		rong(n.NguonURL), rong(n.NguonIDNgoai), n.DaSuaTay, n.NguoiTaoMa,
		n.TaoLuc, n.CapNhatLuc,
		instant(n.PublishedAt), instant(n.EventStartsAt), instant(n.EventEndsAt),
		rong(n.EventPlace), rong(n.VideoURL), rong(n.CoverImageFileID),
		rong(n.LinkTo), order(n.DisplayOrder), rong(n.NoiDung),
	}
}

func order(p *int) driver.Value {
	if p == nil {
		return nil
	}
	return int64(*p)
}

type rowsNDGia struct {
	cot  []string
	hang [][]driver.Value
	i    int
}

func (r *rowsNDGia) Columns() []string { return r.cot }
func (r *rowsNDGia) Close() error      { return nil }

// Next MUST RETURN io.EOF AND NOTHING THAT MERELY READS LIKE IT. database/sql compares the driver's
// error against io.EOF by identity to decide "no more rows"; an `errors.New("EOF")` is a different
// value, so the read fails with that error instead of yielding sql.ErrNoRows — and every "row not
// found" refusal in this file would then be reported as a database failure. Cost the first run of
// this suite exactly that.
func (r *rowsNDGia) Next(dest []driver.Value) error {
	if r.i >= len(r.hang) {
		return io.EOF
	}
	copy(dest, r.hang[r.i])
	r.i++
	return nil
}

type txNDGia struct{ k *khoNDGia }

func (t *txNDGia) Commit() error   { t.k.daCommit++; return nil }
func (t *txNDGia) Rollback() error { t.k.daRollback++; return nil }

func (k *khoNDGia) cau(tu string) []lenhGhi {
	var ra []lenhGhi
	for _, l := range k.lenh {
		if strings.Contains(l.sql, tu) {
			ra = append(ra, l)
		}
	}
	return ra
}

func (k *khoNDGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

const (
	idNDPinned    = "01JNOIDUNGMOI00000000000"
	maCanBoSoanND = "CB-2026-7K3M9Q"
)

var lucNDPinned = time.Date(2026, 9, 23, 8, 30, 0, 0, time.UTC)

// dungUseCaseNoiDung builds the REAL use cases over the REAL stores over the fake driver, with the
// id AND THE CLOCK pinned so assertions can name both.
func dungUseCaseNoiDung(t *testing.T, k *khoNDGia) (*SoanNoiDungMiniApp, *DanhMucNoiDungMiniApp, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	kho := store.New(db)
	nd := commsstore.NewNoiDungMiniAppStore(kho)
	dm := commsstore.NewDanhMucMiniAppStore(kho)

	uc := NewSoanNoiDungMiniApp(kho, nd, dm)
	uc.sinhID = func() (string, error) { return idNDPinned, nil }
	uc.bayGio = func() time.Time { return lucNDPinned }

	ucDM := NewDanhMucNoiDungMiniApp(kho, dm)
	ucDM.sinhID = func() (string, error) { return "01JDANHMUCMOI00000000000", nil }

	return uc, ucDM, tenant.Into(context.Background(), xaA)
}

func nguoiSoanND() audit.Actor {
	return audit.Actor{ID: maCanBoSoanND, Kind: "staff", IP: "10.0.0.7"}
}

func ycThemMau() domain.YeuCauThemNoiDung {
	return domain.YeuCauThemNoiDung{
		Loai:    "tin-tuc",
		TieuDe:  "Xã Thăng Bình khai giảng năm học mới",
		TomTat:  "Sáng nay, xã tổ chức lễ khai giảng.",
		NoiDung: "<p>Toàn văn bài viết.</p>",
	}
}

// --- composing ---------------------------------------------------------------------------------

func TestThemNoiDungGhiHaiCauTrongMotGiaoDich(t *testing.T) {
	// RULE 6, INVARIANT 3, AS A PROPERTY OF THE TRANSACTION BOUNDARIES RATHER THAN A CLAIM. If the
	// item and the trail could land in two transactions, a commune could end up publishing something
	// nobody can be asked about.
	k := &khoNDGia{}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	moi, err := uc.Them(ctx, ycThemMau(), nguoiSoanND())
	if err != nil {
		t.Fatalf("thêm lỗi: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở=%d chốt=%d huỷ=%d, muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}
	for _, muon := range []string{"INSERT INTO noi_dung_mini_app", "INSERT INTO audit_log"} {
		if !k.coCau(muon) {
			t.Errorf("thiếu câu lệnh %q", muon)
		}
	}
	if moi.ID != idNDPinned {
		t.Errorf("id = %q, muốn %q", moi.ID, idNDPinned)
	}
	// §7: the checkbox was NOT ticked, so the item is invisible to residents. THE DEFAULT DIRECTION
	// IS THE SAFE ONE: a commune that published something nobody decided to publish has an incident,
	// not a bug.
	if moi.TrangThai != domain.TrangThaiAn {
		t.Errorf("trạng thái = %q, muốn an khi chưa bật `Đăng lên Mini App`", moi.TrangThai)
	}
}

func TestThemNoiDungQuyetDinhTrangThaiNguonVaNguoiTaoTaiDay(t *testing.T) {
	// The state, the provenance and the author are NOT in the request (domain.YeuCauThemNoiDung has
	// no field for any of them). This asserts what actually reaches the INSERT, which is the only
	// place a future "small" change could reintroduce a client-chosen value.
	k := &khoNDGia{}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	yc := ycThemMau()
	yc.DangLenMiniApp = true
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); err != nil {
		t.Fatalf("thêm lỗi: %v", err)
	}

	l := k.cau("INSERT INTO noi_dung_mini_app")
	if len(l) != 1 {
		t.Fatalf("số câu chèn = %d, muốn 1", len(l))
	}
	// $1 xã, $2 id, $3 loại, $4 danh mục, $5 tiêu đề, $6 tóm tắt, $7 nội dung, $8 ảnh,
	// $9 ngày đăng, $10 trạng thái, $11 người tạo, $12–$17 cột của migration 0011 ($17 ảnh bìa).
	// `nguon` là hằng trong câu lệnh.
	args := l[0].args
	if len(args) != 19 { // $18 link_to, $19 display_order (migration 0012)
		t.Fatalf("số tham số = %d, muốn 19", len(args))
	}
	if args[16] != nil {
		t.Errorf("không có ảnh bìa mà cover_image_file_id = %v, muốn NULL", args[16])
	}
	if args[0] != string(xaA) {
		t.Errorf("xã = %v, muốn %v — xã đến từ ngữ cảnh, không từ yêu cầu", args[0], xaA)
	}
	if args[9] != string(domain.TrangThaiDangHien) {
		t.Errorf("trạng thái = %v, muốn dang-hien khi ô `Đăng lên Mini App` được tích", args[9])
	}
	if args[10] != maCanBoSoanND {
		t.Errorf("người tạo = %v, muốn mã cán bộ từ phiên (luật 6, bất biến 8)", args[10])
	}
	// `ngay_dang` IS ONE CLOCK READING FOR THE WHOLE ACT, as a DATE. §7 collects no date field.
	if ngay, _ := args[8].(time.Time); !ngay.Equal(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ngày đăng = %v, muốn 2026-09-23 (phần ngày của một lần đọc đồng hồ)", args[8])
	}
	// THE PROVENANCE IS A LITERAL AND NOT A PARAMETER — the one edit that would reopen §10.4.
	if !strings.Contains(l[0].sql, "'thu-cong'") {
		t.Errorf("`nguon` phải là hằng trong câu chèn: %s", l[0].sql)
	}
}

func TestThemNoiDungVetKiemToanLaMaCanBoVaKhongMangVanBan(t *testing.T) {
	// TWO RULES IN ONE ASSERTION, and both have a measured history in this repository:
	//   rule 6, invariant 8 — `actor_id` is `CB-…`, never a ULID. Six write paths got this wrong on
	//                          2026-09-22 and no test turned red.
	//   rule 3, forbidden #5 — the audit ledger is never deleted, so the title, the summary and the
	//                          body (free text that routinely names residents) must not enter it.
	k := &khoNDGia{}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	if _, err := uc.Them(ctx, ycThemMau(), nguoiSoanND()); err != nil {
		t.Fatalf("thêm lỗi: %v", err)
	}
	l := k.cau("INSERT INTO audit_log")
	if len(l) != 1 {
		t.Fatalf("số vết kiểm toán = %d, muốn 1", len(l))
	}
	args := l[0].args
	// $1 xã, $2 actor_id, $3 actor_kind, $4 actor_ip, $5 action, $6 subject, $7 at, $8 delta.
	if args[1] != maCanBoSoanND {
		t.Errorf("actor_id = %v, muốn mã cán bộ %q", args[1], maCanBoSoanND)
	}
	if args[4] != HanhViThemNoiDungMiniApp {
		t.Errorf("action = %v, muốn %q", args[4], HanhViThemNoiDungMiniApp)
	}
	chuDe, _ := args[5].(string)
	if chuDe != "noi-dung-mini-app/tin-tuc/2026-09-23" {
		t.Errorf("subject = %q, muốn noi-dung-mini-app/<loại>/<ngày đăng>", chuDe)
	}
	delta, _ := args[7].([]byte)
	for _, cam := range []string{"Thăng Bình", "khai giảng", "Toàn văn"} {
		if strings.Contains(string(delta), cam) {
			t.Errorf("delta chứa %q — tiêu đề, tóm tắt và toàn văn không được vào sổ kiểm toán", cam)
		}
	}
	// What the delta MUST carry: the handle on the record, and the facts that decide what may later
	// be done to it.
	for _, phai := range []string{idNDPinned, "thu-cong", "tin-tuc"} {
		if !strings.Contains(string(delta), phai) {
			t.Errorf("delta thiếu %q", phai)
		}
	}
}

func TestThemNoiDungDanhMucKhongCoThiTuChoiVaCuonLai(t *testing.T) {
	// The foreign key would refuse it too, with a driver's exception and a 500. This turns the
	// ordinary case — a screen somebody left open while a colleague retired the category — into a
	// refusal with a sentence, AND nothing is committed.
	k := &khoNDGia{coDong: false}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	yc := ycThemMau()
	yc.DanhMucID = "dm-da-xoa"
	_, err := uc.Them(ctx, yc, nguoiSoanND())
	if !errors.Is(err, commsstore.ErrDanhMucKhongTonTaiMiniApp) {
		t.Fatalf("lỗi = %v, muốn ErrDanhMucKhongTonTaiMiniApp", err)
	}
	if k.coCau("INSERT INTO noi_dung_mini_app") || k.coCau("INSERT INTO audit_log") {
		t.Error("từ chối mà vẫn ghi")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("giao dịch: chốt=%d huỷ=%d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestThemNoiDungVetKiemToanHongThiCuonLaiCauGhi(t *testing.T) {
	// THE INVARIANT THAT ONLY A TRANSACTION TEST CAN SHOW. If the trail fails, the article must go
	// with it: rule 6, invariant 3 does not permit a business write whose entry failed, and rule 7
	// would make the orphaned article permanent.
	k := &khoNDGia{loiSau: "INSERT INTO audit_log"}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	if _, err := uc.Them(ctx, ycThemMau(), nguoiSoanND()); err == nil {
		t.Fatal("vết kiểm toán hỏng mà thêm vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("giao dịch: chốt=%d huỷ=%d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestThemNoiDungThieuMaCanBoBiTuChoiTruocKhiMoGiaoDich(t *testing.T) {
	// `nguoi_tao_ma` with nothing in it is an article nobody can be asked about, on a channel every
	// resident reads. There is NO FALLBACK to an internal id (rule 6, invariant 8).
	k := &khoNDGia{}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	_, err := uc.Them(ctx, ycThemMau(), audit.Actor{Kind: "staff", IP: "10.0.0.7"})
	if !errors.Is(err, ErrThieuNguoiTaoNoiDung) {
		t.Fatalf("lỗi = %v, muốn ErrThieuNguoiTaoNoiDung", err)
	}
	if k.batDau != 0 || len(k.lenh) != 0 {
		t.Errorf("từ chối mà vẫn mở %d giao dịch và chạy %d câu lệnh", k.batDau, len(k.lenh))
	}
}

func TestThemNoiDungSaiHinhDangBiTuChoiTruocKhiMoGiaoDich(t *testing.T) {
	k := &khoNDGia{}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	yc := ycThemMau()
	yc.AnhDaiDienURL = "javascript:alert(1)"
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); !errors.Is(err, domain.ErrURLKhongHopLe) {
		t.Fatalf("lỗi = %v, muốn ErrURLKhongHopLe", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch cho một yêu cầu sai hình dạng", k.batDau)
	}
}

// --- editing, and §10.4 --------------------------------------------------------------------------

func dongDongBo() *domain.NoiDungMiniApp {
	return &domain.NoiDungMiniApp{
		ID: "nd-001", Loai: domain.LoaiTinTuc, TieuDe: "Tiêu đề từ Cổng",
		NgayDang: lucNDPinned, TrangThai: domain.TrangThaiDangHien,
		Nguon: domain.NguonDongBoCong, NguonURL: "https://cong/a", NguonIDNgoai: "cong-42",
		DaSuaTay: false, NguoiTaoMa: "CB-2026-AAAA11",
		TaoLuc: lucNDPinned, CapNhatLuc: lucNDPinned, NoiDung: "<p>Từ Cổng</p>",
	}
}

func TestSuaBaiDongBoVeThiDatCoDaSuaTay(t *testing.T) {
	// §10.4, THE ONE RULE OF THIS FILE WORTH READING TWICE. A member of staff corrected a portal
	// article's title; the next sync runs in six hours and must not undo it. The flag is the whole
	// mechanism, and migration 0006 then refuses to clear it.
	k := &khoNDGia{dongHienCo: dongDongBo()}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	tieuDe := "Tiêu đề đã sửa tay"
	sau, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{TieuDe: &tieuDe}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if !sau.DaSuaTay {
		t.Fatal("sửa tay một bài đồng bộ về mà KHÔNG đặt cờ da_sua_tay — lượt đồng bộ sau sẽ ghi đè (§10.4)")
	}
	l := k.cau("UPDATE noi_dung_mini_app")
	if len(l) != 1 {
		t.Fatalf("số câu cập nhật = %d, muốn 1", len(l))
	}
	// $1 xã, $2 id, $3 loại, $4 danh mục, $5 tiêu đề, $6 tóm tắt, $7 nội dung, $8 ảnh,
	// $9 trạng thái, $10 da_sua_tay.
	if l[0].args[9] != true {
		t.Errorf("tham số da_sua_tay = %v, muốn true", l[0].args[9])
	}
}

func TestSuaBaiSoanTayKhongDatCoDaSuaTay(t *testing.T) {
	// THE OTHER HALF, AND IT IS NOT DECORATION: the CHECK constraint of migration 0006 refuses
	// `da_sua_tay` on a `thu-cong` row, so setting it here would make every edit of a hand-composed
	// article fail at the database — and the flag would claim a protection that protects nothing.
	thuCong := dongDongBo()
	thuCong.Nguon = domain.NguonThuCong
	thuCong.NguonURL, thuCong.NguonIDNgoai = "", ""

	k := &khoNDGia{dongHienCo: thuCong}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	tieuDe := "Tiêu đề đã sửa"
	sau, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{TieuDe: &tieuDe}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if sau.DaSuaTay {
		t.Error("bài soạn tay không được mang cờ da_sua_tay — ràng buộc CHECK của 0006 từ chối đúng hình dạng ấy")
	}
}

func TestSuaKhongDoiGiThiKhongGhiKhongVetVaKhongDatCo(t *testing.T) {
	// THREE PROPERTIES IN ONE CASE, and the third is the one that would be missed. A no-op must write
	// nothing and audit nothing — otherwise the ledger fills with entries saying nothing changed, and
	// those are the entries that bury the ones carrying legal weight. AND it must not set
	// `da_sua_tay`: opening the modal and pressing save without touching anything would otherwise
	// take a portal article permanently out of the sync's reach.
	k := &khoNDGia{dongHienCo: dongDongBo()}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	nguyen := "Tiêu đề từ Cổng"
	sau, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{TieuDe: &nguyen}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if k.coCau("UPDATE noi_dung_mini_app") {
		t.Error("không đổi gì mà vẫn chạy câu cập nhật")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Error("không đổi gì mà vẫn ghi vết kiểm toán")
	}
	if sau.DaSuaTay {
		t.Error("lần lưu không đổi gì mà vẫn đặt cờ da_sua_tay — bài sẽ thoát khỏi đồng bộ vì một lần bấm Lưu")
	}
	if k.daCommit != 1 {
		t.Errorf("giao dịch không đổi gì: chốt=%d, muốn 1", k.daCommit)
	}
}

func TestSuaGiuNguyenTruongKhongDuocNhac(t *testing.T) {
	// THE WHOLE REASON THE REQUEST IS A STRUCT OF POINTERS. A screen editing only the title must not
	// clear the summary, drop the article out of its category, or unpublish it.
	goc := dongDongBo()
	goc.DanhMucID = "dm-001"
	goc.TomTat = "Tóm tắt cũ"

	k := &khoNDGia{dongHienCo: goc}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	tieuDe := "Tiêu đề mới"
	sau, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{TieuDe: &tieuDe}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if sau.TomTat != "Tóm tắt cũ" || sau.DanhMucID != "dm-001" ||
		sau.TrangThai != domain.TrangThaiDangHien {
		t.Errorf("trường không được nhắc bị đổi: %+v", sau)
	}
}

func TestSuaBoBaiKhoiMiniAppBangOTich(t *testing.T) {
	// §7's checkbox is also how an item comes OFF the Mini App. There is no DELETE route in chapter
	// 11, and §6's action column offers only `✎`.
	k := &khoNDGia{dongHienCo: dongDongBo()}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	tat := false
	sau, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{DangLenMiniApp: &tat}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if sau.TrangThai != domain.TrangThaiAn {
		t.Errorf("trạng thái = %q, muốn an", sau.TrangThai)
	}
	if sau.HienChoDan() {
		t.Error("bài đã tắt mà vẫn hiện cho dân")
	}
}

func TestSuaKhongCoDongThiBaoKhongTonTai(t *testing.T) {
	k := &khoNDGia{dongHienCo: nil}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	tieuDe := "Tiêu đề"
	_, err := uc.Sua(ctx, "nd-cua-xa-khac", domain.YeuCauSuaNoiDung{TieuDe: &tieuDe}, nguoiSoanND())
	if !errors.Is(err, commsstore.ErrNoiDungKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrNoiDungKhongTonTai", err)
	}
	if k.coCau("UPDATE noi_dung_mini_app") {
		t.Error("không tìm thấy dòng mà vẫn chạy câu cập nhật")
	}
}

func TestSuaDocDongDuoiKhoaFORUPDATE(t *testing.T) {
	// `FOR UPDATE` IS NOT AN OPTIMISATION. Without it two members of staff editing the same article
	// both read the old state and the second write silently overwrites the first — including the case
	// where one of them was unpublishing it.
	k := &khoNDGia{dongHienCo: dongDongBo()}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	tieuDe := "Tiêu đề mới"
	if _, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{TieuDe: &tieuDe}, nguoiSoanND()); err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	l := k.cau("FOR UPDATE")
	if len(l) != 1 {
		t.Fatalf("số câu đọc có khoá = %d, muốn 1", len(l))
	}
	if l[0].args[0] != string(xaA) {
		t.Errorf("xã = %v, muốn %v", l[0].args[0], xaA)
	}
}

// --- migration 0011: G1 first-publish instant, event and video fields ------------------------------

// updateArgs returns the bound values of the one UPDATE of the edit. Positions after $10 (0-based
// index): 10 event_starts_at, 11 event_ends_at, 12 event_place, 13 video_url, 14 published_at.
func updateArgs(t *testing.T, k *khoNDGia) []driver.Value {
	t.Helper()
	stmts := k.cau("UPDATE noi_dung_mini_app")
	if len(stmts) != 1 {
		t.Fatalf("số câu cập nhật = %d, muốn 1", len(stmts))
	}
	if len(stmts[0].args) != 18 { // $16 cover_image_file_id, $17 link_to, $18 display_order
		t.Fatalf("số tham số cập nhật = %d, muốn 18", len(stmts[0].args))
	}
	return stmts[0].args
}

func auditDelta(t *testing.T, k *khoNDGia) string {
	t.Helper()
	stmts := k.cau("INSERT INTO audit_log")
	if len(stmts) != 1 {
		t.Fatalf("số vết kiểm toán = %d, muốn 1", len(stmts))
	}
	delta, _ := stmts[0].args[7].([]byte)
	return string(delta)
}

func draftRow() *domain.NoiDungMiniApp {
	row := dongDongBo()
	row.Nguon, row.NguonURL, row.NguonIDNgoai = domain.NguonThuCong, "", ""
	row.TrangThai = domain.TrangThaiAn
	return row
}

func TestCreatePublishedFixesFirstPublishInstantAtInsert(t *testing.T) {
	// Migration 0011's trigger is UPDATE-only, so G1 at INSERT is owed by this write path.
	for _, publish := range []bool{true, false} {
		k := &khoNDGia{}
		uc, _, ctx := dungUseCaseNoiDung(t, k)
		req := ycThemMau()
		req.DangLenMiniApp = publish

		created, err := uc.Them(ctx, req, nguoiSoanND())
		if err != nil {
			t.Fatalf("publish=%v: thêm lỗi: %v", publish, err)
		}
		bound := k.cau("INSERT INTO noi_dung_mini_app")[0].args[11]
		if publish {
			if got, _ := bound.(time.Time); !got.Equal(lucNDPinned) || !created.PublishedAt.Equal(lucNDPinned) {
				t.Errorf("đăng ngay: published_at = %v / %v, muốn %v", bound, created.PublishedAt, lucNDPinned)
			}
			if !strings.Contains(auditDelta(t, k), `"published_at":"2026-09-23T08:30:00Z"`) {
				t.Errorf("vết thiếu published_at: %s", auditDelta(t, k))
			}
		} else if bound != nil || !created.PublishedAt.IsZero() {
			t.Errorf("chưa đăng mà published_at = %v / %v, muốn NULL", bound, created.PublishedAt)
		}
	}
}

func TestEditThatFirstPublishesStampsTheInstant(t *testing.T) {
	k := &khoNDGia{dongHienCo: draftRow()}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	on := true
	after, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{DangLenMiniApp: &on}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if got, _ := updateArgs(t, k)[14].(time.Time); !got.Equal(lucNDPinned) || !after.PublishedAt.Equal(lucNDPinned) {
		t.Errorf("lần đăng đầu: published_at = %v / %v, muốn %v", updateArgs(t, k)[14], after.PublishedAt, lucNDPinned)
	}
	delta := auditDelta(t, k)
	if !strings.Contains(delta, `"published_at":"2026-09-23T08:30:00Z"`) || !strings.Contains(delta, `"published_at":""`) {
		t.Errorf("vết phải ghi published_at trước (rỗng) và sau: %s", delta)
	}
}

func TestRepublishKeepsTheOriginalInstant(t *testing.T) {
	// Unpublished once, published again: G1 says the FIRST instant stays. The bound value is the stored
	// one (and COALESCE in the statement would keep it anyway).
	first := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	row := draftRow()
	row.PublishedAt = first
	k := &khoNDGia{dongHienCo: row}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	on := true
	after, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{DangLenMiniApp: &on}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if !after.PublishedAt.Equal(first) {
		t.Errorf("đăng lại: published_at = %v, muốn giữ %v", after.PublishedAt, first)
	}
	if got, _ := updateArgs(t, k)[14].(time.Time); !got.Equal(first) {
		t.Errorf("tham số published_at = %v, muốn %v", updateArgs(t, k)[14], first)
	}
	if strings.Contains(auditDelta(t, k), "published_at") {
		t.Errorf("published_at không đổi mà vẫn vào vết: %s", auditDelta(t, k))
	}
}

func TestEditOfLegacyPublishedRowStampsNothing(t *testing.T) {
	// Published before 0011 (NULL, no backfill) and edited while staying `dang-hien`: the trigger would
	// refuse a stamp here, and the use case must not try.
	row := draftRow()
	row.TrangThai = domain.TrangThaiDangHien
	k := &khoNDGia{dongHienCo: row}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	title := "Tiêu đề mới"
	after, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{TieuDe: &title}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if updateArgs(t, k)[14] != nil || !after.PublishedAt.IsZero() {
		t.Errorf("bài cũ đang hiện bị đóng giờ đăng: %v / %v", updateArgs(t, k)[14], after.PublishedAt)
	}
}

func eventRow() *domain.NoiDungMiniApp {
	row := draftRow()
	row.Loai = domain.LoaiSuKien
	row.EventStartsAt = time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC)
	row.EventEndsAt = time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)
	row.EventPlace = "Sân nhà ông Nguyễn Văn A"
	return row
}

func TestTypeChangeAwayFromEventClearsItsColumnsInTheSameUpdate(t *testing.T) {
	k := &khoNDGia{dongHienCo: eventRow()}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	news := string(domain.LoaiTinTuc)
	after, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{Loai: &news}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	args := updateArgs(t, k)
	for i := 10; i <= 13; i++ {
		if args[i] != nil {
			t.Errorf("đổi loại khỏi su-kien mà tham số $%d = %v, muốn NULL", i+1, args[i])
		}
	}
	if !after.EventStartsAt.IsZero() || after.EventPlace != "" {
		t.Errorf("dòng trả về còn trường sự kiện: %+v", after)
	}
	// The place can name a household: only THAT it changed enters the ledger.
	delta := auditDelta(t, k)
	if strings.Contains(delta, "Nguyễn Văn A") || !strings.Contains(delta, `"event_place_changed":true`) {
		t.Errorf("vết về địa điểm sai: %s", delta)
	}
	if !strings.Contains(delta, `"event_starts_at":"2026-10-05T01:00:00Z"`) {
		t.Errorf("vết thiếu event_starts_at trước khi xoá: %s", delta)
	}
}

func TestTypeChangeAwayFromVideoClearsTheLink(t *testing.T) {
	row := draftRow()
	row.Loai, row.VideoURL = domain.LoaiVideo, "https://video.example/v/1"
	k := &khoNDGia{dongHienCo: row}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	news := string(domain.LoaiTinTuc)
	if _, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{Loai: &news}, nguoiSoanND()); err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if updateArgs(t, k)[13] != nil {
		t.Errorf("đổi loại khỏi video mà video_url = %v, muốn NULL", updateArgs(t, k)[13])
	}
}

func TestEditRefusalsOnTheMergedRowWriteNothingAndNameNoCommune(t *testing.T) {
	early := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	place := "Nhà văn hoá"
	link := "https://video.example/v/1"
	cases := []struct {
		name string
		row  *domain.NoiDungMiniApp
		req  domain.YeuCauSuaNoiDung
		want error
	}{
		{"event field on news", draftRow(), domain.YeuCauSuaNoiDung{EventPlace: &place},
			domain.ErrEventFieldsOnlyForEvent},
		{"video link on news", draftRow(), domain.YeuCauSuaNoiDung{VideoURL: &link},
			domain.ErrVideoURLOnlyForVideo},
		// Only the end is sent; it is judged against the STORED start.
		{"end before stored start", eventRow(), domain.YeuCauSuaNoiDung{EventEndsAt: &early},
			domain.ErrEventEndsBeforeStart},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := &khoNDGia{dongHienCo: tc.row}
			uc, _, ctx := dungUseCaseNoiDung(t, k)

			_, err := uc.Sua(ctx, "nd-001", tc.req, nguoiSoanND())
			if !errors.Is(err, tc.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.want)
			}
			// The handler returns this sentence to the client: it must be the domain's own, not
			// wrapped with the commune id.
			if strings.Contains(err.Error(), string(xaA)) {
				t.Errorf("thông điệp từ chối lộ mã xã: %q", err.Error())
			}
			if k.coCau("UPDATE noi_dung_mini_app") || k.coCau("INSERT INTO audit_log") {
				t.Error("từ chối mà vẫn ghi")
			}
			if k.daCommit != 0 || k.daRollback != 1 {
				t.Errorf("giao dịch: chốt=%d huỷ=%d, muốn 0/1", k.daCommit, k.daRollback)
			}
		})
	}
}

func TestCreateTypeFieldRefusalsBeforeAnyTransaction(t *testing.T) {
	starts := time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		shape func(*domain.YeuCauThemNoiDung)
		want  error
	}{
		{"event on news", func(r *domain.YeuCauThemNoiDung) { r.EventStartsAt = starts },
			domain.ErrEventFieldsOnlyForEvent},
		{"video on event", func(r *domain.YeuCauThemNoiDung) {
			r.Loai, r.VideoURL = "su-kien", "https://video.example/v/1"
		}, domain.ErrVideoURLOnlyForVideo},
		{"end without start", func(r *domain.YeuCauThemNoiDung) {
			r.Loai, r.EventEndsAt = "su-kien", starts
		}, domain.ErrEventEndsWithoutStart},
		{"non-http video", func(r *domain.YeuCauThemNoiDung) {
			r.Loai, r.VideoURL = "video", "javascript:alert(1)"
		}, domain.ErrURLKhongHopLe},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := &khoNDGia{}
			uc, _, ctx := dungUseCaseNoiDung(t, k)
			req := ycThemMau()
			tc.shape(&req)
			if _, err := uc.Them(ctx, req, nguoiSoanND()); !errors.Is(err, tc.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.want)
			}
			if k.batDau != 0 {
				t.Errorf("mở %d giao dịch cho một yêu cầu sai", k.batDau)
			}
		})
	}
}

func TestCreateEventStoresItsFields(t *testing.T) {
	k := &khoNDGia{}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	req := ycThemMau()
	req.Loai = "su-kien"
	req.EventStartsAt = time.Date(2026, 10, 5, 8, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	req.EventPlace = "  Nhà văn hoá thôn 3  "

	created, err := uc.Them(ctx, req, nguoiSoanND())
	if err != nil {
		t.Fatalf("thêm lỗi: %v", err)
	}
	args := k.cau("INSERT INTO noi_dung_mini_app")[0].args
	if got, _ := args[12].(time.Time); !got.Equal(req.EventStartsAt) {
		t.Errorf("event_starts_at = %v, muốn %v", args[12], req.EventStartsAt)
	}
	if args[14] != "Nhà văn hoá thôn 3" || created.EventPlace != "Nhà văn hoá thôn 3" {
		t.Errorf("event_place = %v, muốn đã cắt khoảng trắng", args[14])
	}
	if strings.Contains(auditDelta(t, k), "Nhà văn hoá") {
		t.Errorf("địa điểm lọt vào vết kiểm toán: %s", auditDelta(t, k))
	}
}

// --- the category tree -----------------------------------------------------------------------------

func TestThemDanhMucChuDeVetLaSlug(t *testing.T) {
	// A CATEGORY HAS A BUSINESS CODE, so its audit subject is a VALUE rather than something composed
	// — which is the shape rule 6 and audit.Entry's own contract ask for, and the reason the content
	// item's subject has to be composed instead.
	k := &khoNDGia{dem: 3}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)

	moi, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{
		Ten: "Chuyển đổi số", Slug: "chuyen-doi-so", ThuTu: 2,
	}, nguoiSoanND())
	if err != nil {
		t.Fatalf("thêm danh mục lỗi: %v", err)
	}
	if moi.Slug != "chuyen-doi-so" {
		t.Errorf("slug = %q", moi.Slug)
	}
	l := k.cau("INSERT INTO audit_log")
	if len(l) != 1 {
		t.Fatalf("số vết = %d, muốn 1", len(l))
	}
	if l[0].args[1] != maCanBoSoanND {
		t.Errorf("actor_id = %v, muốn mã cán bộ", l[0].args[1])
	}
	if l[0].args[5] != "chuyen-doi-so" {
		t.Errorf("subject = %v, muốn slug", l[0].args[5])
	}
	if k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("giao dịch: chốt=%d huỷ=%d, muốn 1/0", k.daCommit, k.daRollback)
	}
}

func TestThemDanhMucSlugTrungBiTuChoi(t *testing.T) {
	// The unique key is the real guard; this is the readable message. `dem` = 1 makes SlugDaDung say
	// yes — including for a SOFT-DELETED row, which is the case rule 7, invariant 3 is about.
	k := &khoNDGia{demSlug: 1}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)

	_, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Ten: "Chuyển đổi số", Slug: "chuyen-doi-so"},
		nguoiSoanND())
	if !errors.Is(err, commsstore.ErrSlugDanhMucDaTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrSlugDanhMucDaTonTai", err)
	}
	if k.coCau("INSERT INTO danh_muc_mini_app") {
		t.Error("slug trùng mà vẫn chèn")
	}
}

func TestThemDanhMucVuotTranBiTuChoiTruocKhiKiemSlug(t *testing.T) {
	// ORDER OF THE REFUSALS: the ceiling is a condition of the WHOLE list, which the caller can act on
	// without knowing anything about this value. A full tree reported as "slug đã dùng" would send
	// somebody renaming their category forever.
	k := &khoNDGia{dem: int64(commsstore.TranDanhMucMiniApp)}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)

	_, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Ten: "Mục mới", Slug: "muc-moi"}, nguoiSoanND())
	if !errors.Is(err, commsstore.ErrQuaNhieuDanhMucMiniApp) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuDanhMucMiniApp", err)
	}
}

func TestThemDanhMucChaKhongCoThiTuChoi(t *testing.T) {
	// The self-referencing foreign key refuses a parent that does not exist AT ALL; this also catches
	// a SOFT-DELETED parent, which is still a row and which the constraint therefore accepts — and
	// which §7's select, excluding deleted rows, would draw as a child whose parent is nowhere.
	k := &khoNDGia{dem: 0, coDong: false}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)

	_, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{
		Ten: "Mục con", Slug: "muc-con", ChaID: "dm-da-xoa",
	}, nguoiSoanND())
	if !errors.Is(err, ErrDanhMucChaKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrDanhMucChaKhongTonTai", err)
	}
	if k.coCau("INSERT INTO danh_muc_mini_app") {
		t.Error("cha không tồn tại mà vẫn chèn")
	}
}
