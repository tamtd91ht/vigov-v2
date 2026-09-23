package store

// The Mini App content register's SQL, over a FAKE DRIVER — the half that runs on every machine.
//
// THE OTHER HALF IS noi_dung_mini_app_pg_test.go AND NEITHER REPLACES THE OTHER. This file proves the
// Go side: that the page read asks for the columns the scan reads, in that order; that the commune is
// bound and is never a parameter; that §6's three filters become bound placeholders in the right
// order; that a soft-deleted item is excluded everywhere and a soft-deleted SLUG is deliberately not;
// that the write statements cannot carry the values they must not carry. What it CANNOT prove is
// anything the database decides — that these columns and these tables exist at all, the CHECK
// constraints, the immutability trigger, the foreign key, the unique key. That is what the pg suite
// is for, and it SKIPS unless VIGOV_TEST_DSN is set.

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
	xaMotND  = tenant.ID("01JND" + strings.Repeat("C", 21))
	lucMauND = time.Date(2026, 9, 14, 8, 9, 0, 0, time.UTC)
	ngayMau  = time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
)

// dongNDMiniApp is one `noi_dung_mini_app` row, addressed BY COLUMN NAME.
//
// THIS IS WHAT MAKES THE COLUMN CHECK REAL: the fake assembles each row from the names the store
// actually asked for, so reordering cotNoiDungMiniApp without reordering quetNoiDungMiniApp turns
// this suite red. A fake that returned a fixed tuple would agree with any order — and three of these
// columns (`nguon`, `nguon_url`, `nguon_id_ngoai`) are adjacent text, where a swap compiles, runs,
// and produces an article claiming to come from somewhere it did not.
type dongNDMiniApp struct {
	id, loai, tieuDe, trangThai, nguon, nguoiTaoMa string
	danhMucID, tomTat, anh, nguonURL, nguonIDNgoai any // nil where the column is NULL
	than                                           any // `noi_dung`, nil on a row with no body
	ngayDang                                       time.Time
	luotXem                                        int64
	daSuaTay                                       bool
	taoLuc, capNhatLuc                             time.Time
}

func (d dongNDMiniApp) giaTri(cot string) driver.Value {
	switch cot {
	case "id":
		return d.id
	case "loai":
		return d.loai
	case "danh_muc_id":
		return d.danhMucID
	case "tieu_de":
		return d.tieuDe
	case "tom_tat":
		return d.tomTat
	case "anh_dai_dien_url":
		return d.anh
	case "ngay_dang":
		return d.ngayDang
	case "luot_xem":
		return d.luotXem
	case "trang_thai":
		return d.trangThai
	case "nguon":
		return d.nguon
	case "nguon_url":
		return d.nguonURL
	case "nguon_id_ngoai":
		return d.nguonIDNgoai
	case "da_sua_tay":
		return d.daSuaTay
	case "nguoi_tao_ma":
		return d.nguoiTaoMa
	case "tao_luc":
		return d.taoLuc
	case "cap_nhat_luc":
		return d.capNhatLuc
	case "noi_dung":
		return d.than
	default:
		// LOUD, NOT ZERO. A silent zero here would let a column be added to cotNoiDungMiniApp and
		// never actually be read by anything, while this suite "passed" and proved nothing about it.
		panic("driver giả nội dung Mini App: không có giá trị mẫu cho cột " + cot)
	}
}

// dongDMMiniApp is one `danh_muc_mini_app` row, addressed BY COLUMN NAME for the same reason.
type dongDMMiniApp struct {
	id, ten, slug string
	chaID         any
	thuTu         int64
	taoLuc        time.Time
}

func (d dongDMMiniApp) giaTri(cot string) driver.Value {
	switch cot {
	case "id":
		return d.id
	case "ten":
		return d.ten
	case "slug":
		return d.slug
	case "cha_id":
		return d.chaID
	case "thu_tu":
		return d.thuTu
	case "tao_luc":
		return d.taoLuc
	default:
		panic("driver giả danh mục Mini App: không có giá trị mẫu cho cột " + cot)
	}
}

type khoNDGia struct {
	lenh []lenhGhi

	dong     []dongNDMiniApp
	danhMuc  []dongDMMiniApp
	demTraVe int64 // what a `SELECT count(*)` answers
	coDong   bool  // what a `SELECT 1 …` existence probe answers

	soDongDoi int64 // what Exec reports as RowsAffected; 1 unless a test says otherwise
	loi       error
}

func (k *khoNDGia) Connect(context.Context) (driver.Conn, error) { return &connND{k: k}, nil }
func (k *khoNDGia) Driver() driver.Driver                        { return trinhND{} }

type trinhND struct{}

func (trinhND) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả nội dung Mini App: chỉ dùng Connector")
}

type connND struct{ k *khoNDGia }

func (c *connND) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả nội dung Mini App: không hỗ trợ Prepare")
}
func (c *connND) Close() error { return nil }
func (c *connND) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *connND) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) { return txND{}, nil }

type txND struct{}

func (txND) Commit() error   { return nil }
func (txND) Rollback() error { return nil }

func (c *connND) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.k.lenh = append(c.k.lenh, lenhGhi{sql: q, args: ghiArgs(args)})
	if c.k.loi != nil {
		return nil, c.k.loi
	}
	n := c.k.soDongDoi
	if n == 0 && !strings.Contains(q, "UPDATE") {
		n = 1
	}
	return ketQuaGia{n: n}, nil
}

func (c *connND) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.k.lenh = append(c.k.lenh, lenhGhi{sql: q, args: ghiArgs(args)})
	if c.k.loi != nil {
		return nil, c.k.loi
	}

	// The aggregate and the existence probe are answered BEFORE the column reader, because their
	// SELECT lists are not column names and the row assembler would panic on them. Keeping the two
	// outcomes separable is the whole reason the store distinguishes them.
	if strings.Contains(q, "count(*)") {
		return &rowsGhiGia{cot: []string{"count"}, hang: [][]driver.Value{{c.k.demTraVe}}}, nil
	}
	if strings.Contains(q, "SELECT 1 FROM") {
		if !c.k.coDong {
			return &rowsGhiGia{cot: []string{"?column?"}}, nil
		}
		return &rowsGhiGia{cot: []string{"?column?"}, hang: [][]driver.Value{{int64(1)}}}, nil
	}

	cot, err := cotTrongCauLenhGhi(q)
	if err != nil {
		return nil, err
	}
	if strings.Contains(q, "FROM danh_muc_mini_app") {
		hang := make([][]driver.Value, 0, len(c.k.danhMuc))
		for _, d := range c.k.danhMuc {
			mot := make([]driver.Value, len(cot))
			for i, ten := range cot {
				mot[i] = d.giaTri(ten)
			}
			hang = append(hang, mot)
		}
		return &rowsGhiGia{cot: cot, hang: hang}, nil
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

func khoNoiDung(t *testing.T, k *khoNDGia) (*NoiDungMiniAppStore, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewNoiDungMiniAppStore(pkgstore.New(db)), tenant.Into(context.Background(), xaMotND)
}

func khoDanhMucND(t *testing.T, k *khoNDGia) (*DanhMucMiniAppStore, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return NewDanhMucMiniAppStore(pkgstore.New(db)), tenant.Into(context.Background(), xaMotND)
}

func trangDauND(t *testing.T) pkgpage.Request {
	t.Helper()
	yc, err := pkgpage.New(SapXepNoiDungMiniApp, "", "", "", "")
	if err != nil {
		t.Fatalf("dựng yêu cầu phân trang: %v", err)
	}
	return yc
}

func dongNDMau() dongNDMiniApp {
	return dongNDMiniApp{
		id: "nd-001", loai: "tin-tuc", tieuDe: "Xã Thăng Bình khai giảng năm học mới",
		danhMucID: "dm-001", tomTat: "Sáng nay…", anh: "https://x/a.png",
		ngayDang: ngayMau, luotXem: 7, trangThai: "dang-hien",
		nguon: "dong-bo-cong", nguonURL: "https://cong/a", nguonIDNgoai: "cong-42",
		daSuaTay: true, nguoiTaoMa: "CB-2026-7K3M9Q",
		taoLuc: lucMauND, capNhatLuc: lucMauND, than: "<p>Toàn văn</p>",
	}
}

// --- the read path -------------------------------------------------------------------------

func TestDanhSachNoiDungDocDungCotVaDungThuTu(t *testing.T) {
	// THE ASSERTION THIS FILE EXISTS FOR. The row is assembled BY NAME from the columns the store
	// asked for, so a swap between cotNoiDungMiniApp and quetNoiDungMiniApp lands the wrong value in
	// the wrong field — and `nguon` / `nguon_url` / `nguon_id_ngoai` are three adjacent text columns
	// the compiler cannot tell apart.
	k := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	kho, ctx := khoNoiDung(t, k)

	kq, err := kho.DanhSach(ctx, LocNoiDung{}, trangDauND(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if len(kq.Items) != 1 {
		t.Fatalf("số dòng = %d, muốn 1", len(kq.Items))
	}
	mot := kq.Items[0]
	if mot.ID != "nd-001" || mot.TieuDe != "Xã Thăng Bình khai giảng năm học mới" {
		t.Errorf("dòng = %+v", mot)
	}
	if mot.Loai != domain.LoaiTinTuc || mot.TrangThai != domain.TrangThaiDangHien {
		t.Errorf("loại = %q, trạng thái = %q", mot.Loai, mot.TrangThai)
	}
	if mot.Nguon != domain.NguonDongBoCong || mot.NguonURL != "https://cong/a" || mot.NguonIDNgoai != "cong-42" {
		t.Errorf("ba cột xuất xứ bị hoán vị: nguon=%q url=%q ma_ngoai=%q",
			mot.Nguon, mot.NguonURL, mot.NguonIDNgoai)
	}
	if !mot.DaSuaTay {
		t.Error("cờ §10.4 `da_sua_tay` không đọc được")
	}
	if mot.LuotXem != 7 || !mot.NgayDang.Equal(ngayMau) {
		t.Errorf("lượt xem = %d, ngày đăng = %v", mot.LuotXem, mot.NgayDang)
	}
	if mot.NguoiTaoMa != "CB-2026-7K3M9Q" {
		t.Errorf("người tạo = %q, muốn mã cán bộ", mot.NguoiTaoMa)
	}
}

func TestDanhSachNoiDungKhongMangToanVan(t *testing.T) {
	// THE PROPERTY THAT MAKES THE DETAIL ROUTE NECESSARY. A hundred articles at
	// domain.ThanNoiDungToiDa is twenty million runes in one response, so the page must not select
	// `noi_dung` — and the fake driver PANICS on a column it has no sample for, which is how this
	// assertion also catches the column being added to the list by accident.
	k := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	kho, ctx := khoNoiDung(t, k)

	kq, err := kho.DanhSach(ctx, LocNoiDung{}, trangDauND(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	if kq.Items[0].NoiDung != "" {
		t.Errorf("trang danh sách mang toàn văn: %q", kq.Items[0].NoiDung)
	}
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "FROM noi_dung_mini_app") && strings.Contains(l.sql, "noi_dung,") {
			t.Errorf("câu đọc trang chọn cả `noi_dung`: %s", l.sql)
		}
	}
}

func TestDanhSachNoiDungBuocXaVaLocDongDaXoaMem(t *testing.T) {
	k := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	kho, ctx := khoNoiDung(t, k)

	if _, err := kho.DanhSach(ctx, LocNoiDung{}, trangDauND(t)); err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}

	var cau string
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "FROM noi_dung_mini_app") {
			cau = l.sql
			if len(l.args) == 0 || l.args[0] != string(xaMotND) {
				t.Fatalf("tham số $1 = %v, muốn xã %v — xã đến từ ngữ cảnh (luật 1, bất biến 5)",
					l.args, xaMotND)
			}
		}
	}
	if cau == "" {
		t.Fatal("không thấy câu đọc trang")
	}
	if !strings.Contains(cau, "tenant_id = $1") {
		t.Errorf("câu đọc trang thiếu `tenant_id = $1`: %s", cau)
	}
	// RULE 7, INVARIANT 2 — everywhere, always. Both partial indexes of migration 0006 are built on
	// exactly this predicate.
	if !strings.Contains(cau, "deleted_at IS NULL") {
		t.Errorf("câu đọc trang thiếu `deleted_at IS NULL`: %s", cau)
	}
	// §6 shows the newest first, and the `id` tie-break is what makes the order TOTAL — without it
	// two items written in the same millisecond let page two repeat one and drop the other.
	if !strings.Contains(cau, "ORDER BY tao_luc DESC, id DESC") {
		t.Errorf("thứ tự sai: %s", cau)
	}
}

func TestDanhSachNoiDungBaBoLocThanhThamSoDungThuTu(t *testing.T) {
	// §6's filter bar: the tab, the category select and the title box. THE NUMBERING IS THE
	// ASSERTION: store.QueryPage binds PageSpec.Args from $2 in slice order, so a mismatch between
	// the `$n` written into the predicate and the position in the slice binds the title pattern to
	// the category — which returns an empty list for every request, with no error anywhere.
	k := &khoNDGia{}
	kho, ctx := khoNoiDung(t, k)

	_, err := kho.DanhSach(ctx, LocNoiDung{Loai: "banner", DanhMucID: "dm-9", Tu: "khai giảng"},
		trangDauND(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}

	l := k.lenh[0]
	for _, muon := range []string{"loai = $2", "danh_muc_id = $3", "tieu_de ILIKE $4"} {
		if !strings.Contains(l.sql, muon) {
			t.Errorf("câu thiếu %q: %s", muon, l.sql)
		}
	}
	if len(l.args) < 5 {
		t.Fatalf("số tham số = %d, muốn ít nhất 5 (xã, loại, danh mục, tiêu đề, limit)", len(l.args))
	}
	if l.args[0] != string(xaMotND) || l.args[1] != "banner" || l.args[2] != "dm-9" {
		t.Errorf("tham số = %v", l.args[:3])
	}
	if l.args[3] != "%khai giảng%" {
		t.Errorf("mẫu tìm = %v, muốn %q", l.args[3], "%khai giảng%")
	}
}

func TestDanhSachNoiDungThoatKyTuDaiDienTrongTuKhoa(t *testing.T) {
	// WITHOUT THIS, A `%` TYPED IN THE SEARCH BOX MATCHES EVERYTHING and `_` matches any character.
	// The box is the commune's own search, so the failure is not an attack — it is a member of staff
	// who pasted a title containing an underscore and got back rows they did not ask for.
	k := &khoNDGia{}
	kho, ctx := khoNoiDung(t, k)

	if _, err := kho.DanhSach(ctx, LocNoiDung{Tu: `100%_ke\hoach`}, trangDauND(t)); err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	mau, _ := k.lenh[0].args[1].(string)
	if mau != `%100\%\_ke\\hoach%` {
		t.Errorf("mẫu tìm = %q, muốn ba ký tự đại diện đã được thoát", mau)
	}
}

func TestDanhSachNoiDungKhongLocThiKhongThemThamSoNao(t *testing.T) {
	k := &khoNDGia{}
	kho, ctx := khoNoiDung(t, k)

	if _, err := kho.DanhSach(ctx, LocNoiDung{Tu: "   "}, trangDauND(t)); err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	l := k.lenh[0]
	// A BLANK SEARCH BOX IS NOT A FILTER. `ILIKE '%%'` would match every row and cost a scan for
	// nothing, on the request every screen makes when it opens.
	if strings.Contains(l.sql, "ILIKE") {
		t.Errorf("từ khoá toàn khoảng trắng mà vẫn sinh mệnh đề ILIKE: %s", l.sql)
	}
	if len(l.args) != 2 { // the commune and the limit
		t.Errorf("số tham số = %d, muốn 2 (xã, limit)", len(l.args))
	}
}

func TestSapXepNoiDungChiNhanCotTaoLuc(t *testing.T) {
	// THE ALLOWLIST IS THE WHOLE SURFACE a client may sort on. `ngay_dang` is deliberately not on it:
	// it is a DATE, so one sync run importing four hundred articles published the same day gives four
	// hundred rows the same sort value and the `id` tie-break silently carries the whole order.
	for _, cot := range []string{"published_on", "ngay_dang", "view_count", "title"} {
		if _, err := pkgpage.New(SapXepNoiDungMiniApp, cot, "", "", ""); !errors.Is(err, pkgpage.ErrSort) {
			t.Errorf("sắp xếp theo %q: lỗi = %v, muốn ErrSort", cot, err)
		}
	}
}

func TestTheoIDDocToanVanVaLocDongDaXoaMem(t *testing.T) {
	k := &khoNDGia{dong: []dongNDMiniApp{dongNDMau()}}
	kho, ctx := khoNoiDung(t, k)

	n, err := kho.TheoID(ctx, "nd-001")
	if err != nil {
		t.Fatalf("đọc chi tiết lỗi: %v", err)
	}
	if n.NoiDung != "<p>Toàn văn</p>" {
		t.Errorf("toàn văn = %q — tuyến chi tiết tồn tại chính vì trang danh sách không mang nó", n.NoiDung)
	}
	cau := k.lenh[0].sql
	for _, muon := range []string{"tenant_id = $1", "id = $2", "deleted_at IS NULL", "noi_dung"} {
		if !strings.Contains(cau, muon) {
			t.Errorf("câu đọc chi tiết thiếu %q: %s", muon, cau)
		}
	}
}

func TestTheoIDKhongCoDongThiBaoKhongTonTai(t *testing.T) {
	// 404 AND NOT 403 FOR ANOTHER COMMUNE'S ITEM, indistinguishably: the predicate binds the commune
	// to $1, so an id belonging to another authority is simply not there. Telling the two apart would
	// confirm what that authority holds.
	k := &khoNDGia{}
	kho, ctx := khoNoiDung(t, k)

	if _, err := kho.TheoID(ctx, "nd-cua-xa-khac"); !errors.Is(err, ErrNoiDungKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrNoiDungKhongTonTai", err)
	}
}

// --- the write path -------------------------------------------------------------------------

func chayTrongGiaoDich(t *testing.T, k *khoNDGia, ctx context.Context,
	fn func(tx *pkgstore.ScopedTx) error) error {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return pkgstore.New(db).For(ctx).Tx(ctx, fn)
}

func TestChenNoiDungGhiNguonLaHangSoVaKhongNhanMaNgoai(t *testing.T) {
	// THE SECURITY PROPERTY OF THIS STATEMENT, not a shortcut: there is no `$n` for `nguon`,
	// `nguon_id_ngoai`, `nguon_url` or `da_sua_tay`, so no layer above can pass one and no client can
	// fill one. That is what keeps §10.4's protection — a hand-edited portal article survives the next
	// sync — out of reach of a request body.
	k := &khoNDGia{}
	kho, ctx := khoNoiDung(t, k)

	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return kho.Chen(ctx, tx, domain.NoiDungMiniApp{
			ID: "nd-moi", Loai: domain.LoaiTinTuc, TieuDe: "Tiêu đề",
			NgayDang: ngayMau, TrangThai: domain.TrangThaiAn, NguoiTaoMa: "CB-2026-7K3M9Q",
		})
	})
	if err != nil {
		t.Fatalf("chèn lỗi: %v", err)
	}

	var cau lenhGhi
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "INSERT INTO noi_dung_mini_app") {
			cau = l
		}
	}
	if cau.sql == "" {
		t.Fatal("không thấy câu chèn")
	}
	if !strings.Contains(cau.sql, "'thu-cong'") {
		t.Errorf("`nguon` phải là hằng trong câu lệnh: %s", cau.sql)
	}
	for _, cam := range []string{"nguon_id_ngoai", "nguon_url", "da_sua_tay", "luot_xem",
		"deleted_at", "deleted_by", "delete_reason"} {
		if strings.Contains(cau.sql, cam) {
			t.Errorf("câu chèn không được nhận %q: %s", cam, cau.sql)
		}
	}
	if cau.args[0] != string(xaMotND) {
		t.Errorf("xã = %v, muốn %v — xã đến từ giao dịch, không từ tham số", cau.args[0], xaMotND)
	}
}

func TestChenNoiDungBienChuoiRongThanhNull(t *testing.T) {
	// `danh_muc_id` HAS A FOREIGN KEY AND '' IS NOT A CATEGORY ID — the constraint would refuse it,
	// and the ordinary row is precisely the one with no category: §7's `— Chưa xếp danh mục —`.
	k := &khoNDGia{}
	kho, ctx := khoNoiDung(t, k)

	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return kho.Chen(ctx, tx, domain.NoiDungMiniApp{
			ID: "nd-moi", Loai: domain.LoaiTinTuc, TieuDe: "Tiêu đề",
			NgayDang: ngayMau, TrangThai: domain.TrangThaiAn, NguoiTaoMa: "CB-1",
		})
	})
	if err != nil {
		t.Fatalf("chèn lỗi: %v", err)
	}
	// $1 xã, $2 id, $3 loại, $4 danh mục, $5 tiêu đề, $6 tóm tắt, $7 nội dung, $8 ảnh,
	// $9 ngày đăng, $10 trạng thái, $11 người tạo.
	args := k.lenh[0].args
	if len(args) != 11 {
		t.Fatalf("số tham số = %d, muốn 11", len(args))
	}
	for _, i := range []int{3, 5, 6, 7} {
		if args[i] != nil {
			t.Errorf("tham số $%d = %v, muốn NULL cho chuỗi rỗng", i+1, args[i])
		}
	}
}

func TestCapNhatNoiDungKhongChamDuocXuatXuVaNguoiTao(t *testing.T) {
	// SIX COLUMNS ARE REFUSED BY THIS STATEMENT'S SHAPE, and the trigger in migration 0006 refuses
	// them again. Both layers are meant: the trigger is the floor that holds against every writer, and
	// their absence here is what makes the floor unreachable from this service in the first place.
	k := &khoNDGia{soDongDoi: 1}
	kho, ctx := khoNoiDung(t, k)

	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return kho.CapNhat(ctx, tx, domain.NoiDungMiniApp{
			ID: "nd-001", Loai: domain.LoaiTinTuc, TieuDe: "Tiêu đề mới",
			TrangThai: domain.TrangThaiDangHien, DaSuaTay: true,
		})
	})
	if err != nil {
		t.Fatalf("cập nhật lỗi: %v", err)
	}
	cau := k.lenh[0].sql
	for _, cam := range []string{"nguon =", "nguon_id_ngoai", "nguoi_tao_ma", "tao_luc =",
		"luot_xem", "deleted_at =", "ngay_dang ="} {
		if strings.Contains(cau, cam) {
			t.Errorf("câu cập nhật không được ghi %q: %s", cam, cau)
		}
	}
	// `da_sua_tay` IS THE ONE PROVENANCE COLUMN THIS STATEMENT DOES WRITE — §10.4 is recorded here.
	if !strings.Contains(cau, "da_sua_tay = $10") {
		t.Errorf("câu cập nhật phải ghi `da_sua_tay` (§10.4): %s", cau)
	}
	// `AND deleted_at IS NULL` IS WHAT MAKES EDITING A DELETED ITEM A 404 rather than a resurrection.
	if !strings.Contains(cau, "deleted_at IS NULL") {
		t.Errorf("câu cập nhật thiếu `deleted_at IS NULL`: %s", cau)
	}
}

func TestCapNhatKhongDongNaoThiBaoKhongTonTai(t *testing.T) {
	// An UPDATE touching zero rows is not an error to PostgreSQL; it is only an error to us. Without
	// this check a method used without the locked read would report success for a row that is gone.
	k := &khoNDGia{soDongDoi: 0}
	kho, ctx := khoNoiDung(t, k)

	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return kho.CapNhat(ctx, tx, domain.NoiDungMiniApp{ID: "nd-da-xoa", Loai: domain.LoaiTinTuc})
	})
	if !errors.Is(err, ErrNoiDungKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrNoiDungKhongTonTai", err)
	}
}

func TestDanhMucCoThatLocDongDaXoaMem(t *testing.T) {
	// THE FOREIGN KEY DOES NOT COVER THIS CASE: a soft-deleted category is still a row, so the
	// constraint accepts it. Filing a new article under a category the commune retired last week is a
	// mistake only this predicate catches.
	k := &khoNDGia{coDong: true}
	kho, ctx := khoNoiDung(t, k)

	_, err := chayTrongGiaoDichCoKetQua(t, k, ctx, func(tx *pkgstore.ScopedTx) (bool, error) {
		return kho.DanhMucCoThat(ctx, tx, "dm-001")
	})
	if err != nil {
		t.Fatalf("kiểm danh mục lỗi: %v", err)
	}
	cau := k.lenh[0].sql
	if !strings.Contains(cau, "deleted_at IS NULL") || !strings.Contains(cau, "tenant_id = $1") {
		t.Errorf("câu kiểm danh mục = %s", cau)
	}
}

// chayTrongGiaoDichCoKetQua is the same helper with a value coming back out of the closure.
func chayTrongGiaoDichCoKetQua[T any](t *testing.T, k *khoNDGia, ctx context.Context,
	fn func(tx *pkgstore.ScopedTx) (T, error)) (T, error) {
	t.Helper()
	var ra T
	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		var err error
		ra, err = fn(tx)
		return err
	})
	return ra, err
}

// --- the category tree ---------------------------------------------------------------------------

func TestDanhSachDanhMucDocDungCotVaDungThuTu(t *testing.T) {
	k := &khoNDGia{danhMuc: []dongDMMiniApp{
		{id: "dm-001", ten: "Chuyển đổi số", slug: "chuyen-doi-so", chaID: "dm-goc",
			thuTu: 2, taoLuc: lucMauND},
		{id: "dm-goc", ten: "Danh mục", slug: "danh-muc", chaID: nil, thuTu: 1, taoLuc: lucMauND},
	}}
	kho, ctx := khoDanhMucND(t, k)

	ds, err := kho.DanhSach(ctx)
	if err != nil {
		t.Fatalf("đọc danh mục lỗi: %v", err)
	}
	if len(ds) != 2 {
		t.Fatalf("số danh mục = %d, muốn 2", len(ds))
	}
	// `ten` and `slug` are adjacent TEXT columns, as are `id` and `cha_id`: swapping either pair
	// compiles, runs, and produces a tree whose every label is a slug or whose every node is its own
	// parent.
	if ds[0].Ten != "Chuyển đổi số" || ds[0].Slug != "chuyen-doi-so" {
		t.Errorf("tên/slug bị hoán vị: %+v", ds[0])
	}
	if ds[0].ChaID != "dm-goc" {
		t.Errorf("cha = %q, muốn dm-goc", ds[0].ChaID)
	}
	// A ROOT CATEGORY HAS A NULL PARENT and it must land as "" rather than crash the scan.
	if ds[1].ChaID != "" {
		t.Errorf("danh mục gốc có cha = %q, muốn rỗng", ds[1].ChaID)
	}

	cau := k.lenh[0].sql
	if !strings.Contains(cau, "ORDER BY thu_tu, slug") {
		t.Errorf("thứ tự sai — `slug` là thứ làm cho thứ tự TOÀN PHẦN: %s", cau)
	}
	if !strings.Contains(cau, "deleted_at IS NULL") {
		t.Errorf("câu đọc danh mục thiếu `deleted_at IS NULL`: %s", cau)
	}
	// LIMIT IS THE CEILING PLUS ONE, which is what makes "there are too many" detectable at all.
	if l := k.lenh[0].args; len(l) != 2 || l[1] != int64(TranDanhMucMiniApp+1) {
		t.Errorf("tham số = %v, muốn [xã, %d]", l, TranDanhMucMiniApp+1)
	}
}

func TestSlugDaDungDemCaDongDaXoaMem(t *testing.T) {
	// RULE 7, INVARIANT 3, AS A PREDICATE: `deleted_at` is deliberately ABSENT here. A commune that
	// could soft-delete `chuyen-doi-so` and create a new, unrelated `chuyen-doi-so` would silently
	// refile every article already filed under the old one.
	k := &khoNDGia{demTraVe: 1}
	kho, ctx := khoDanhMucND(t, k)

	co, err := chayTrongGiaoDichCoKetQua(t, k, ctx, func(tx *pkgstore.ScopedTx) (bool, error) {
		return kho.SlugDaDung(ctx, tx, "chuyen-doi-so")
	})
	if err != nil {
		t.Fatalf("kiểm slug lỗi: %v", err)
	}
	if !co {
		t.Error("slug đã dùng mà báo chưa")
	}
	cau := k.lenh[0].sql
	if strings.Contains(cau, "deleted_at") {
		t.Errorf("câu kiểm slug KHÔNG được loại dòng đã xoá mềm — mã đã cấp không cấp lại: %s", cau)
	}
	if !strings.Contains(cau, "tenant_id = $1") {
		t.Errorf("câu kiểm slug thiếu `tenant_id = $1`: %s", cau)
	}
}

func TestChenDanhMucBuocXaVaBienChaRongThanhNull(t *testing.T) {
	k := &khoNDGia{}
	kho, ctx := khoDanhMucND(t, k)

	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return kho.Chen(ctx, tx, domain.DanhMucMiniApp{
			ID: "dm-moi", Ten: "Chuyển đổi số", Slug: "chuyen-doi-so", ChaID: "", ThuTu: 3,
		})
	})
	if err != nil {
		t.Fatalf("chèn danh mục lỗi: %v", err)
	}
	args := k.lenh[0].args
	if args[0] != string(xaMotND) {
		t.Errorf("xã = %v, muốn %v", args[0], xaMotND)
	}
	// A ROOT CATEGORY HAS NO PARENT, and '' would fail the self-referencing foreign key.
	if args[4] != nil {
		t.Errorf("cha rỗng = %v, muốn NULL", args[4])
	}
}
