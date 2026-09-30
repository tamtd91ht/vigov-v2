package store

// The announcement book's SQL, over a FAKE DRIVER — the half that runs on every machine.
//
// THE OTHER HALF IS thong_bao_noi_bo_pg_test.go AND NEITHER REPLACES THE OTHER. This file proves
// the Go side: that the page read asks for the columns the scan reads, in that order; that the
// commune is bound and is never a parameter; that the counter read is ONE statement for a whole
// page; that a soft-deleted announcement is excluded. What it CANNOT prove is anything the
// database decides — that these columns and this table exist at all, the CHECK constraints, the
// immutability triggers, the foreign keys. That is what the pg suite is for, and it SKIPS unless
// VIGOV_TEST_DSN is set.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	pkgpage "github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

var (
	xaMotTB  = tenant.ID("01JTB" + strings.Repeat("A", 21))
	lucMauTB = time.Date(2026, 9, 7, 16, 35, 0, 0, time.UTC)
)

// dongTBNoiBo is one `thong_bao` row, addressed BY COLUMN NAME.
//
// THIS IS WHAT MAKES THE COLUMN CHECK REAL: the fake assembles each row from the names the store
// actually asked for, so reordering cotThongBaoNoiBo without reordering quetThongBaoNoiBo turns
// this suite red. A fake that returned a fixed tuple would agree with any order.
type dongTBNoiBo struct {
	id, tieuDe, noiDung, trangThai, trangThaiThu, nguoiSoanMa string
	ghim, batBuoc, guiThu                                     bool
	phatHanhLuc                                               any // nil for a draft
	taoLuc                                                    time.Time
}

func (d dongTBNoiBo) giaTri(cot string) driver.Value {
	switch cot {
	case "id":
		return d.id
	case "tieu_de":
		return d.tieuDe
	case "noi_dung":
		return d.noiDung
	case "trang_thai":
		return d.trangThai
	case "ghim":
		return d.ghim
	case "bat_buoc_xac_nhan":
		return d.batBuoc
	case "gui_thu_dien_tu":
		return d.guiThu
	case "trang_thai_thu":
		return d.trangThaiThu
	case "nguoi_soan_ma":
		return d.nguoiSoanMa
	case "phat_hanh_luc":
		return d.phatHanhLuc
	case "tao_luc":
		return d.taoLuc
	default:
		// LOUD, NOT ZERO. A silent zero here would let a column be added to cotThongBaoNoiBo and
		// never actually be read by anything, while this suite "passed" and proved nothing about it.
		panic("driver giả thông báo nội bộ: không có giá trị mẫu cho cột " + cot)
	}
}

// demTBGia is one row of the counter query.
type demTBGia struct {
	id            string
	tong, xacNhan int64
}

type khoTBNoiBoGia struct {
	lenh []lenhGhi
	dong []dongTBNoiBo
	dem  []demTBGia
	loi  error
}

func (k *khoTBNoiBoGia) Connect(context.Context) (driver.Conn, error) { return &connTBNoiBo{k: k}, nil }
func (k *khoTBNoiBoGia) Driver() driver.Driver                        { return trinhTBNoiBo{} }

type trinhTBNoiBo struct{}

func (trinhTBNoiBo) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả thông báo nội bộ: chỉ dùng Connector")
}

type connTBNoiBo struct{ k *khoTBNoiBoGia }

func (c *connTBNoiBo) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả thông báo nội bộ: không hỗ trợ Prepare")
}
func (c *connTBNoiBo) Close() error { return nil }
func (c *connTBNoiBo) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *connTBNoiBo) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return txTBNoiBo{}, nil
}

type txTBNoiBo struct{}

func (txTBNoiBo) Commit() error   { return nil }
func (txTBNoiBo) Rollback() error { return nil }

func (c *connTBNoiBo) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.lenh = append(c.k.lenh, lenhGhi{sql: q, args: ghiArgs(args)})
	if c.k.loi != nil {
		return nil, c.k.loi
	}
	return ketQuaGia{n: 1}, nil
}

func (c *connTBNoiBo) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.lenh = append(c.k.lenh, lenhGhi{sql: q, args: ghiArgs(args)})
	if c.k.loi != nil {
		return nil, c.k.loi
	}

	if strings.Contains(q, "GROUP BY thong_bao_id") {
		hang := make([][]driver.Value, 0, len(c.k.dem))
		for _, d := range c.k.dem {
			hang = append(hang, []driver.Value{d.id, d.tong, d.xacNhan})
		}
		return &rowsGhiGia{cot: []string{"thong_bao_id", "tong", "xac_nhan"}, hang: hang}, nil
	}

	cot, err := cotTrongCauLenhGhi(q)
	if err != nil {
		return nil, err
	}
	hang := make([][]driver.Value, 0, len(c.k.dong))
	for _, d := range c.k.dong {
		mot := make([]driver.Value, len(cot))
		for i, ten := range cot {
			mot[i] = d.giaTri(ten)
		}
		hang = append(hang, mot)
	}
	return &rowsGhiGia{cot: cot, hang: hang}, nil
}

func khoThongBaoNoiBo(t *testing.T, k *khoTBNoiBoGia) (*ThongBaoNoiBoStore, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewThongBaoNoiBoStore(pkgstore.New(db)), tenant.Into(context.Background(), xaMotTB)
}

func trangDau(t *testing.T) pkgpage.Request {
	t.Helper()
	yc, err := pkgpage.New(SapXepThongBaoNoiBo, "", "", "", "")
	if err != nil {
		t.Fatalf("dựng yêu cầu phân trang: %v", err)
	}
	return yc
}

// --- the read path -------------------------------------------------------------------------

func TestDanhSachThongBaoDocDungCotVaDungThuTu(t *testing.T) {
	// THE ASSERTION THIS FILE EXISTS FOR. The row is assembled BY NAME from the columns the store
	// asked for, so a swap between cotThongBaoNoiBo and quetThongBaoNoiBo lands the wrong value in
	// the wrong field — three adjacent booleans make that swap invisible to the compiler.
	k := &khoTBNoiBoGia{dong: []dongTBNoiBo{{
		id: "tb-001", tieuDe: "Thông báo về việc triển khai hệ thống an ninh",
		noiDung: "Toàn văn nội dung.", trangThai: "da-phat-hanh",
		ghim: true, batBuoc: true, guiThu: false,
		trangThaiThu: "chua-gui", nguoiSoanMa: "CB-2026-7K3M9Q",
		phatHanhLuc: lucMauTB, taoLuc: lucMauTB,
	}}}
	kho, ctx := khoThongBaoNoiBo(t, k)

	kq, err := kho.DanhSach(ctx, trangDau(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if len(kq.Items) != 1 {
		t.Fatalf("số thẻ = %d, muốn 1", len(kq.Items))
	}
	mot := kq.Items[0]
	if mot.ID != "tb-001" || mot.TieuDe != "Thông báo về việc triển khai hệ thống an ninh" {
		t.Errorf("thẻ = %+v", mot)
	}
	if mot.NoiDung != "Toàn văn nội dung." {
		t.Errorf("nội dung = %q — cột này PHẢI có trong danh sách: §2 vẽ panel chi tiết từ chính nó", mot.NoiDung)
	}
	if !mot.Ghim || !mot.BatBuocXacNhan || mot.GuiThuDienTu {
		t.Errorf("ba cờ bị hoán vị: ghim=%v bắt buộc=%v gửi thư=%v",
			mot.Ghim, mot.BatBuocXacNhan, mot.GuiThuDienTu)
	}
	if mot.TrangThai != domain.ThongBaoDaPhatHanh || mot.TrangThaiThu != domain.ThuChuaGui {
		t.Errorf("trạng thái = %q / %q", mot.TrangThai, mot.TrangThaiThu)
	}
	if mot.NguoiSoanMa != "CB-2026-7K3M9Q" {
		t.Errorf("người soạn = %q, muốn mã cán bộ", mot.NguoiSoanMa)
	}
}

func TestDanhSachThongBaoBanNhapCoPhatHanhLucRong(t *testing.T) {
	// `phat_hanh_luc` IS NULL FOR A DRAFT, and scanning a NULL straight into a time.Time is a
	// runtime error in some drivers and a zero value in others. The store reads it through
	// sql.NullTime; this is the case that proves it.
	k := &khoTBNoiBoGia{dong: []dongTBNoiBo{{
		id: "tb-nhap", tieuDe: "Nháp", noiDung: "Nội dung", trangThai: "nhap",
		trangThaiThu: "chua-gui", nguoiSoanMa: "CB-1", phatHanhLuc: nil, taoLuc: lucMauTB,
	}}}
	kho, ctx := khoThongBaoNoiBo(t, k)

	kq, err := kho.DanhSach(ctx, trangDau(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if !kq.Items[0].PhatHanhLuc.IsZero() {
		t.Errorf("bản nháp có phát hành lúc = %v, muốn rỗng", kq.Items[0].PhatHanhLuc)
	}
	if kq.Items[0].DaPhatHanh() {
		t.Error("bản nháp không được coi là đã phát hành")
	}
}

func TestDanhSachThongBaoLocDongDaXoaMemVaBuocXa(t *testing.T) {
	k := &khoTBNoiBoGia{dong: []dongTBNoiBo{{
		id: "tb-001", tieuDe: "T", noiDung: "N", trangThai: "da-phat-hanh",
		trangThaiThu: "chua-gui", nguoiSoanMa: "CB-1", phatHanhLuc: lucMauTB, taoLuc: lucMauTB,
	}}}
	kho, ctx := khoThongBaoNoiBo(t, k)

	if _, err := kho.DanhSach(ctx, trangDau(t)); err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}

	var cauTrang string
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "FROM thong_bao ") {
			cauTrang = l.sql
			if len(l.args) == 0 || l.args[0] != string(xaMotTB) {
				t.Fatalf("tham số $1 = %v, muốn xã %v — xã đến từ ngữ cảnh (luật 1, bất biến 5)",
					l.args, xaMotTB)
			}
		}
	}
	if cauTrang == "" {
		t.Fatal("không thấy câu đọc trang")
	}
	// RULE 7, INVARIANT 2 — everywhere, always. A soft-deleted announcement must not appear in a
	// list, and the partial index `thong_bao_so` is built on exactly this predicate.
	if !strings.Contains(cauTrang, "deleted_at IS NULL") {
		t.Errorf("câu đọc trang thiếu `deleted_at IS NULL`: %s", cauTrang)
	}
	if !strings.Contains(cauTrang, "tenant_id = $1") {
		t.Errorf("câu đọc trang thiếu `tenant_id = $1`: %s", cauTrang)
	}
	// §2: "mới nhất ở trên", and the `id` tie-break is what makes the order TOTAL — without it two
	// announcements written in the same millisecond let page two repeat one and drop the other.
	if !strings.Contains(cauTrang, "ORDER BY tao_luc DESC, id DESC") {
		t.Errorf("thứ tự sai: %s", cauTrang)
	}
}

func TestDanhSachThongBaoDemNguoiNhanMotCauChoCaTrang(t *testing.T) {
	// NEVER 1+N. A query per card is affordable on three sample notices and unaffordable on a
	// commune's third year, and the shape that behaves that way is the shape nobody notices.
	k := &khoTBNoiBoGia{
		dong: []dongTBNoiBo{
			{id: "tb-1", tieuDe: "T1", noiDung: "N", trangThai: "da-phat-hanh",
				trangThaiThu: "chua-gui", nguoiSoanMa: "CB-1", phatHanhLuc: lucMauTB, taoLuc: lucMauTB},
			{id: "tb-2", tieuDe: "T2", noiDung: "N", trangThai: "da-phat-hanh",
				trangThaiThu: "chua-gui", nguoiSoanMa: "CB-1", phatHanhLuc: lucMauTB, taoLuc: lucMauTB},
		},
		dem: []demTBGia{{id: "tb-1", tong: 12, xacNhan: 2}},
	}
	kho, ctx := khoThongBaoNoiBo(t, k)

	kq, err := kho.DanhSach(ctx, trangDau(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}

	var soCauDem int
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "GROUP BY thong_bao_id") {
			soCauDem++
			if !strings.Contains(l.sql, "tenant_id = $1") {
				t.Errorf("câu đếm thiếu `tenant_id = $1` — sẽ đếm người nhận của xã khác: %s", l.sql)
			}
			if len(l.args) != 3 || l.args[0] != string(xaMotTB) {
				t.Errorf("tham số câu đếm = %v, muốn [xã, tb-1, tb-2]", l.args)
			}
		}
	}
	if soCauDem != 1 {
		t.Fatalf("số câu đếm = %d, muốn 1 cho cả trang", soCauDem)
	}

	// §3's `2/12 đã xác nhận` on the first card, and `0/0` on the second — a card with no recipient
	// rows produces NO row in a GROUP BY, so the zero value is the correct answer rather than a
	// missing key to crash on.
	if kq.Items[0].SoNguoiNhan != 12 || kq.Items[0].SoDaXacNhan != 2 {
		t.Errorf("thẻ 1 = %d/%d, muốn 2/12", kq.Items[0].SoDaXacNhan, kq.Items[0].SoNguoiNhan)
	}
	if kq.Items[1].SoNguoiNhan != 0 || kq.Items[1].SoDaXacNhan != 0 {
		t.Errorf("thẻ 2 = %d/%d, muốn 0/0", kq.Items[1].SoDaXacNhan, kq.Items[1].SoNguoiNhan)
	}
}

func TestDanhSachThongBaoTrangRongKhongChayCauDem(t *testing.T) {
	// Not only a saving: the `IN (…)` list is built from the ids, and an empty list is not valid
	// SQL. A newly onboarded commune takes this branch on every request.
	k := &khoTBNoiBoGia{}
	kho, ctx := khoThongBaoNoiBo(t, k)

	kq, err := kho.DanhSach(ctx, trangDau(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if len(kq.Items) != 0 {
		t.Fatalf("số thẻ = %d, muốn 0", len(kq.Items))
	}
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "GROUP BY thong_bao_id") {
			t.Fatalf("trang rỗng mà vẫn chạy câu đếm: %s", l.sql)
		}
	}
}

func TestSapXepThongBaoChiNhanCotTaoLuc(t *testing.T) {
	// THE ALLOWLIST IS THE WHOLE SURFACE a client may sort on, and `phat_hanh_luc` is deliberately
	// not on it: it is NULL for a draft, and `(col, id) > (…)` is NULL for a NULL col — every draft
	// would vanish from every page after the first, silently.
	if _, err := pkgpage.New(SapXepThongBaoNoiBo, "issued_at", "", "", ""); !errors.Is(err, pkgpage.ErrSort) {
		t.Errorf("sắp xếp theo issued_at: lỗi = %v, muốn ErrSort", err)
	}
	if _, err := pkgpage.New(SapXepThongBaoNoiBo, "pinned", "", "", ""); !errors.Is(err, pkgpage.ErrSort) {
		t.Errorf("sắp xếp theo pinned: lỗi = %v, muốn ErrSort", err)
	}
}

// --- the write path -------------------------------------------------------------------------

func TestChenNguoiNhanMotCauChoCaDanhSach(t *testing.T) {
	k := &khoTBNoiBoGia{}
	kho, ctx := khoThongBaoNoiBo(t, k)

	err := pkgstore.New(sql.OpenDB(k)).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return kho.ChenNguoiNhan(ctx, tx, "tb-1", []domain.NguoiNhanThongBao{
			{NguoiNhanMa: "CB-A", DichDanh: true},
			{NguoiNhanMa: "CB-B", DichDanh: false},
		})
	})
	if err != nil {
		t.Fatalf("chèn người nhận lỗi: %v", err)
	}

	var cau string
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "INSERT INTO thong_bao_nguoi_nhan") {
			if cau != "" {
				t.Fatal("chèn người nhận phải là MỘT câu cho cả danh sách")
			}
			cau = l.sql
			if l.args[0] != string(xaMotTB) {
				t.Errorf("xã = %v, muốn %v", l.args[0], xaMotTB)
			}
		}
	}
	if cau == "" {
		t.Fatal("không thấy câu chèn người nhận")
	}
	// The overlap between "named explicitly" and "member of a chosen department" is legitimate, so
	// one person is ONE row — which is what makes §3's `{y}` count people rather than reasons.
	if !strings.Contains(cau, "ON CONFLICT DO NOTHING") {
		t.Errorf("thiếu ON CONFLICT DO NOTHING: %s", cau)
	}
	// The recipient is born having neither opened nor acknowledged anything: binding those columns
	// here would let a caller create a recipient already marked as having read something.
	for _, cam := range []string{"da_mo_luc", "da_xac_nhan_luc"} {
		if strings.Contains(cau, cam) {
			t.Errorf("câu chèn không được nhận %q: %s", cam, cau)
		}
	}
}

func TestChenNguoiNhanDanhSachRongKhongChayCauNao(t *testing.T) {
	k := &khoTBNoiBoGia{}
	kho, ctx := khoThongBaoNoiBo(t, k)

	err := pkgstore.New(sql.OpenDB(k)).For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return kho.ChenNguoiNhan(ctx, tx, "tb-1", nil)
	})
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "INSERT INTO thong_bao_nguoi_nhan") {
			t.Fatalf("danh sách rỗng mà vẫn chạy câu chèn: %s", l.sql)
		}
	}
}
