package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// Unit tests for the citizen notification ledger store. The fake driver and the list of what it
// can and cannot prove are in thong_bao_driver_gia_test.go.
//
// `xaThu` and `ctxXa` come from loai_tai_nguyen_ban_do_test.go. ONE way to put a commune into a
// context in this package, not two: a second copy is the one that ends up subtly different from
// what the edge really does.

const (
	maPhieuThu   = "PA-2026-7F3K9Q"
	maCongDanThu = "cd-01JCONGDANMAUTHUNGHIEM"
	maBanGhiThu  = "01JTHONGBAOMAUTHUNGHIEM01"
)

func thamSoThu() domain.ThamSoThongBao {
	return domain.ThamSoThongBao{
		MaTraCuu:     maPhieuThu,
		MocNhan:      "Đã chuyển xử lý",
		ViecTiepTheo: "Cán bộ phụ trách sẽ liên hệ với ông/bà trong 2 ngày làm việc.",
	}
}

func thongBaoThu() domain.ThongBaoGuiCongDan {
	return domain.ThongBaoGuiCongDan{
		ID:           maBanGhiThu,
		KhoaLanGui:   "phieu-phan-anh|" + maPhieuThu + "|da-chuyen-xu-ly|1|" + maCongDanThu + "|zalo-zns",
		DoiTuongLoai: domain.DoiTuongPhieuPhanAnh,
		DoiTuongMa:   maPhieuThu,
		Moc:          "da-chuyen-xu-ly",
		Lan:          1,
		Kenh:         domain.KenhZaloZNS,
		NguoiNhanMa:  maCongDanThu,
		MauMa:        "zns-phan-anh-doi-trang-thai",
		ThamSo:       thamSoThu(),
	}
}

// chayTrongGD runs fn inside a real *store.ScopedTx built over the fake driver. THE TRANSACTION IS
// REAL EVEN THOUGH THE DATABASE IS NOT: rule 6, invariant 3 is a statement about a transaction, so
// a suite that never opens one cannot check anything about it.
func chayTrongGD(t *testing.T, k *khoGhiGia, xa tenant.ID,
	fn func(tx *pkgstore.ScopedTx) error) error {
	t.Helper()
	ctx := ctxXa(xa)
	return pkgstore.New(moKhoGhi(k)).For(ctx).Tx(ctx, fn)
}

func dungKhoThongBao(k *khoGhiGia) *ThongBaoGuiCongDanStore {
	return NewThongBaoGuiCongDanStore(pkgstore.New(moKhoGhi(k)))
}

// --- Chen: the obligation is recorded, once ----------------------------------------------------

func TestChenBuocXaTuContextChuKhongPhaiTuThamSo(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not a parameter of Chen and cannot become one:
	// it is read off the transaction, which took it from the context. If a caller could pass one,
	// a consumer handling a message could write into whichever commune the payload named — which
	// is exactly the door rule 1 forbidden #2 closes.
	k := &khoGhiGia{soDongDoi: 1}
	kho := dungKhoThongBao(k)

	var moi bool
	err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		var err error
		moi, err = kho.Chen(context.Background(), tx, thongBaoThu())
		return err
	})
	if err != nil {
		t.Fatalf("Chen: %v", err)
	}
	if !moi {
		t.Error("dòng mới mà báo là đã có")
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]
	if !strings.Contains(l.sql, "INSERT INTO thong_bao_gui_cong_dan") {
		t.Errorf("ghi nhầm bảng: %q", l.sql)
	}
	if len(l.args) == 0 || l.args[0] != string(xaThu) {
		t.Fatalf("$1 = %v, muốn xã trong context %q", l.args, xaThu)
	}
}

func TestChenGiaoChoCSDLQuyetDinhTrungLap(t *testing.T) {
	// THE IDEMPOTENCY IS STRUCTURAL, NOT CHECKED. A read-then-write would pass every
	// single-threaded test and send a citizen two messages under concurrency — the only condition
	// it ever fails under. What makes that impossible is this clause, so it is asserted literally.
	//
	// `DO NOTHING` AND NOT `DO UPDATE`: the conflicting row is the same fact, possibly already
	// delivered, and the trigger in migration 0004 refuses to move a delivered row — so `DO
	// UPDATE` would turn a harmless duplicate into a failed transaction that rolls back a
	// business write.
	k := &khoGhiGia{soDongDoi: 1}
	kho := dungKhoThongBao(k)

	if err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.Chen(context.Background(), tx, thongBaoThu())
		return err
	}); err != nil {
		t.Fatalf("Chen: %v", err)
	}
	q := k.lenh[0].sql
	if !strings.Contains(q, "ON CONFLICT (tenant_id, khoa_lan_gui) DO NOTHING") {
		t.Fatalf("thiếu chống trùng ở tầng CSDL: %q", q)
	}
	if strings.Contains(q, "DO UPDATE") {
		t.Errorf("DO UPDATE sẽ khiến một lần giao lại ghi đè lên bản ghi đã gửi: %q", q)
	}
}

func TestChenGapTrungThiBaoDaCoChuKhongBaoLoi(t *testing.T) {
	// A REDELIVERY IS NORMAL OPERATION, NOT A FAILURE. Queues deliver at least once (rule 2,
	// invariant 5). Returning an error here would make a consumer retry forever, or give up and
	// dead-letter a message that was already handled correctly.
	//
	// The boolean is load-bearing: app/SoThongBao uses it to decide whether an audit entry is
	// owed, and auditing a redelivery would record a promise the commune never made twice.
	k := &khoGhiGia{soDongDoi: 0}
	kho := dungKhoThongBao(k)

	var moi bool
	err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		var err error
		moi, err = kho.Chen(context.Background(), tx, thongBaoThu())
		return err
	})
	if err != nil {
		t.Fatalf("giao lại cùng một sự kiện phải là no-op, nhận lỗi: %v", err)
	}
	if moi {
		t.Error("dòng đã có mà báo là mới — một bút toán nhật ký thứ hai sẽ được ghi")
	}
}

func TestChenLuonSinhDongOTrangThaiChoGui(t *testing.T) {
	// `trang_thai` IS NOT A PARAMETER OF Chen, and this pins it. Letting a producer name the state
	// would let it insert a row already marked `da-gui` — a claim about the citizen's phone that
	// nothing in this service witnessed. A row is born owing a message; the send decides the rest.
	k := &khoGhiGia{soDongDoi: 1}
	kho := dungKhoThongBao(k)

	tb := thongBaoThu()
	tb.TrangThai = domain.DaGui // a caller trying to skip ahead
	if err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.Chen(context.Background(), tx, tb)
		return err
	}); err != nil {
		t.Fatalf("Chen: %v", err)
	}
	args := k.lenh[0].args
	cuoi := args[len(args)-1]
	if cuoi != string(domain.ChoGui) {
		t.Fatalf("trạng thái ghi vào = %v, muốn %q", cuoi, domain.ChoGui)
	}
}

func TestChenNoiDungKhongDatThiKhongChayCauLenhNao(t *testing.T) {
	// RULE 10, INVARIANT 6 ENFORCED BEFORE THE ROW EXISTS. A notification that names a state and
	// no consequence tells the citizen nothing they can act on, and once the row is in the ledger
	// the adapter will happily send it. The CHECK constraint on `tham_so` would catch the missing
	// key, but it arrives as a database error at the bottom of a transaction with no explanation
	// of which rule was broken.
	k := &khoGhiGia{soDongDoi: 1}
	kho := dungKhoThongBao(k)

	tb := thongBaoThu()
	tb.ThamSo.ViecTiepTheo = ""

	err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.Chen(context.Background(), tx, tb)
		return err
	})
	if !errors.Is(err, domain.ErrThieuViecTiepTheo) {
		t.Fatalf("lỗi = %v, muốn ErrThieuViecTiepTheo", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("đã chạy %d câu lệnh dù nội dung không đạt", len(k.lenh))
	}
}

func TestChenLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := &khoGhiGia{soDongDoi: 1, loiGhi: goc}
	kho := dungKhoThongBao(k)

	err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.Chen(context.Background(), tx, thongBaoThu())
		return err
	})
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	// The transaction must not have been committed: the row and its audit entry live or die
	// together (rule 6, invariant 3).
	if k.daChot != 0 {
		t.Errorf("giao dịch đã chốt %d lần dù lệnh ghi hỏng", k.daChot)
	}
	if k.daHuy == 0 {
		t.Error("giao dịch không được huỷ sau lỗi ghi")
	}
}

// --- GhiKetQua: the outcome of a send ----------------------------------------------------------

func ketQuaDaGui() domain.KetQuaGui {
	return domain.KetQuaGui{
		TrangThai: domain.DaGui,
		GuiLuc:    time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC),
		// The agreed fake number, masked the way core/privacy.MaskPhone masks it.
		NguoiNhanChe: "09****0000",
		SoLanThu:     1,
	}
}

func TestGhiKetQuaLoaiTruDongDaGuiVaDongDaXoaMem(t *testing.T) {
	// TWO PREDICATES, TWO DIFFERENT JOBS, both easy to drop:
	//
	//	trang_thai <> 'da-gui'  the idempotency. A second report of the same success matches no
	//	                        row and changes nothing, instead of hitting the trigger that
	//	                        refuses to move a delivered notification and failing the caller's
	//	                        whole transaction.
	//	deleted_at IS NULL      rule 7, invariant 2 — everywhere, always, including writes.
	k := &khoGhiGia{soDongDoi: 1}
	kho := dungKhoThongBao(k)

	if err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.GhiKetQua(context.Background(), tx, maBanGhiThu, ketQuaDaGui())
		return err
	}); err != nil {
		t.Fatalf("GhiKetQua: %v", err)
	}
	q := k.lenh[0].sql
	if !strings.Contains(q, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không buộc xã: %q", q)
	}
	if !strings.Contains(q, "trang_thai <> 'da-gui'") {
		t.Errorf("thiếu điều kiện chống gửi lại: %q", q)
	}
	if !strings.Contains(q, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q)
	}
}

func TestGhiKetQuaDongDaGuiRoiThiKhongDoiVaKhongLoi(t *testing.T) {
	// "Already delivered" is the expected outcome of an at-least-once queue, not an error. The
	// `false` is what stops app/SoThongBao writing a second audit entry saying the message went
	// out twice.
	k := &khoGhiGia{soDongDoi: 0, coDong: true}
	kho := dungKhoThongBao(k)

	var doi bool
	err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		var err error
		doi, err = kho.GhiKetQua(context.Background(), tx, maBanGhiThu, ketQuaDaGui())
		return err
	})
	if err != nil {
		t.Fatalf("báo lại cùng một kết quả phải là no-op, nhận lỗi: %v", err)
	}
	if doi {
		t.Error("không dòng nào đổi mà vẫn báo là đã đổi")
	}
}

func TestGhiKetQuaKhongCoDongNaoThiBaoKhongTonTai(t *testing.T) {
	// THE TWO ZERO-ROW CASES ARE SEPARATED BY A SECOND READ, NOT BY GUESSING, because they lead an
	// operator to opposite conclusions: "already sent" is nothing to do, "no such notification" is
	// a consumer addressing a record that is not there.
	k := &khoGhiGia{soDongDoi: 0, coDong: false}
	kho := dungKhoThongBao(k)

	err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.GhiKetQua(context.Background(), tx, maBanGhiThu, ketQuaDaGui())
		return err
	})
	if !errors.Is(err, ErrThongBaoKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrThongBaoKhongTonTai", err)
	}
	// The probe is scoped to the commune as well: a row belonging to another commune must read as
	// absent, not as forbidden.
	probe := k.lenh[len(k.lenh)-1]
	if !strings.Contains(probe.sql, "tenant_id = $1") || probe.args[0] != string(xaThu) {
		t.Errorf("câu tra sự tồn tại không buộc xã: %q %v", probe.sql, probe.args)
	}
}

func TestGhiKetQuaTuChoiKetQuaKhongPhaiKetThuc(t *testing.T) {
	// `cho-gui` is where a row STARTS. Writing it back as a RESULT would erase a real outcome and
	// put a delivered notification back into the retry set.
	k := &khoGhiGia{soDongDoi: 1}
	kho := dungKhoThongBao(k)

	err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.GhiKetQua(context.Background(), tx, maBanGhiThu,
			domain.KetQuaGui{TrangThai: domain.ChoGui})
		return err
	})
	if !errors.Is(err, domain.ErrKetQuaTrangThaiKhongPhaiKetThuc) {
		t.Fatalf("lỗi = %v, muốn ErrKetQuaTrangThaiKhongPhaiKetThuc", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("đã chạy %d câu lệnh dù kết quả không hợp lệ", len(k.lenh))
	}
}

func TestGhiKetQuaTuChoiSoNguoiNhanChuaChe(t *testing.T) {
	// THE ONE THAT MATTERS MOST IN THIS FILE. A notification ledger is the classic place a whole
	// commune's phone numbers end up in one table, from where they reach every backup and every
	// log aggregator. The rule is also a CHECK constraint in migration 0004 — both layers, on
	// purpose: the constraint holds against a statement typed at a psql prompt, this one holds
	// against a caller and names the rule before a transaction is even open.
	//
	// The fake number is the agreed one (rule 3, invariant 5); what makes it fail here is that it
	// carries no mask.
	k := &khoGhiGia{soDongDoi: 1}
	kho := dungKhoThongBao(k)

	kq := ketQuaDaGui()
	kq.NguoiNhanChe = "0900000000"

	err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.GhiKetQua(context.Background(), tx, maBanGhiThu, kq)
		return err
	})
	if !errors.Is(err, domain.ErrNguoiNhanChuaChe) {
		t.Fatalf("lỗi = %v, muốn ErrNguoiNhanChuaChe", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("đã chạy %d câu lệnh dù số chưa được che", len(k.lenh))
	}
	// AND THE VALUE ITSELF MUST NOT BE IN THE ERROR. The error travels into logs, and the value is
	// the very thing suspected of being an unmasked number (rule 3, forbidden #3).
	if strings.Contains(err.Error(), "0900000000") {
		t.Errorf("số chưa che lọt vào thông điệp lỗi: %q", err.Error())
	}
}

func TestGhiKetQuaDaGuiMaThieuMocGioThiTuChoi(t *testing.T) {
	// "Sent" with no date answers nothing when an inspection asks when the citizen was told.
	k := &khoGhiGia{soDongDoi: 1}
	kho := dungKhoThongBao(k)

	kq := ketQuaDaGui()
	kq.GuiLuc = time.Time{}

	err := chayTrongGD(t, k, xaThu, func(tx *pkgstore.ScopedTx) error {
		_, err := kho.GhiKetQua(context.Background(), tx, maBanGhiThu, kq)
		return err
	})
	if !errors.Is(err, domain.ErrKetQuaThieuMocGio) {
		t.Fatalf("lỗi = %v, muốn ErrKetQuaThieuMocGio", err)
	}
}

// --- TheoDoiTuong: what did we tell this citizen -----------------------------------------------

func dongMau(id, taoLuc string) dongThongBao {
	t, _ := time.Parse(time.RFC3339, taoLuc)
	return dongThongBao{
		id: id, khoa: "phieu-phan-anh|" + maPhieuThu + "|da-dong|1|" + maCongDanThu + "|zalo-zns",
		loai: "phieu-phan-anh", doiTuongMa: maPhieuThu, moc: "da-dong", lan: 1,
		kenh: "zalo-zns", nguoiNhanMa: maCongDanThu, nguoiNhanChe: nil,
		mauMa:     "zns-phan-anh-doi-trang-thai",
		thamSo:    []byte(`{"ma_tra_cuu":"` + maPhieuThu + `","moc_nhan":"Đã đóng","viec_tiep_theo":"Ông/bà có thể đánh giá kết quả trong ứng dụng."}`),
		trangThai: "cho-gui", soLanThu: 0, guiLuc: nil, loiMa: nil, taoLuc: t,
	}
}

func TestTheoDoiTuongCauLenhBuocXaLocXoaMemVaSapXepOnDinh(t *testing.T) {
	k := &khoGhiGia{dong: []dongThongBao{dongMau("tb-1", "2026-09-20T08:00:00Z")}}
	kho := dungKhoThongBao(k)

	if _, err := kho.TheoDoiTuong(ctxXa(xaThu), domain.DoiTuongPhieuPhanAnh, maPhieuThu); err != nil {
		t.Fatalf("TheoDoiTuong: %v", err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]

	// RULE 1: the commune is bound from the context by Scoped.Query, never passed in.
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if l.args[0] != string(xaThu) {
		t.Fatalf("$1 = %v, muốn %q", l.args[0], xaThu)
	}
	// RULE 7, INVARIANT 2.
	if !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", l.sql)
	}
	// THE BUSINESS CODE IS MATCHED EXACTLY. A prefix or a pattern match would turn one lookup code
	// into a way to read the history of the codes around it — rule 4, invariant 4.
	if !strings.Contains(l.sql, "doi_tuong_ma = $3") {
		t.Errorf("mã hồ sơ không được so khớp chính xác: %q", l.sql)
	}
	if strings.Contains(l.sql, "LIKE") || strings.Contains(l.sql, "ILIKE") {
		t.Errorf("so khớp mờ trên mã hồ sơ: %q", l.sql)
	}
	// NEWEST FIRST, WITH A TOTAL ORDER. `id` carries UNIQUE (tenant_id, id), so two calls cannot
	// return the same rows in a different sequence.
	if !strings.Contains(l.sql, "ORDER BY tao_luc DESC, id DESC") {
		t.Errorf("thứ tự không ổn định: %q", l.sql)
	}
	// The LIMIT is the ceiling PLUS ONE — that single character is what makes "there are too many"
	// detectable rather than indistinguishable from a complete list.
	if len(l.args) < 4 || l.args[3] != int64(TranThongBaoMotHoSo+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", l.args[3:], TranThongBaoMotHoSo+1)
	}
}

func TestTheoDoiTuongDocDungTungCot(t *testing.T) {
	// The fake builds each row BY COLUMN NAME out of the statement, so reordering cotThongBao
	// without reordering the Scan in docMotThongBao turns this red. Read by position,
	// `doi_tuong_ma`/`moc` and `nguoi_nhan_ma`/`nguoi_nhan_che` are adjacent same-typed columns and
	// a swap produces no error at all — only a ledger claiming a message about record A carried
	// record B's code.
	d := dongMau("tb-1", "2026-09-20T08:00:00Z")
	d.nguoiNhanChe = "09****0000"
	d.trangThai = "da-gui"
	d.soLanThu = 2
	gui, _ := time.Parse(time.RFC3339, "2026-09-20T08:05:00Z")
	d.guiLuc = gui
	k := &khoGhiGia{dong: []dongThongBao{d}}

	ra, err := dungKhoThongBao(k).TheoDoiTuong(ctxXa(xaThu), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("TheoDoiTuong: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	mot := ra[0]
	if mot.ID != "tb-1" || mot.DoiTuongMa != maPhieuThu || mot.Moc != "da-dong" {
		t.Errorf("ba cột TEXT đọc sai chỗ: %+v", mot)
	}
	if mot.NguoiNhanMa != maCongDanThu || mot.NguoiNhanChe != "09****0000" {
		t.Errorf("người nhận đọc sai chỗ: %+v", mot)
	}
	if mot.Lan != 1 || mot.SoLanThu != 2 {
		t.Errorf("hai cột số đọc ngược: lan=%d so_lan_thu=%d", mot.Lan, mot.SoLanThu)
	}
	if mot.TrangThai != domain.DaGui || !mot.GuiLuc.Equal(gui) {
		t.Errorf("trạng thái / mốc gửi sai: %+v", mot)
	}
	// The template parameters survive the round trip — the ledger has to be able to say what was
	// actually said, and it says it as parameters rather than as rendered text (rule 3).
	if mot.ThamSo.MaTraCuu != maPhieuThu || mot.ThamSo.MocNhan != "Đã đóng" {
		t.Errorf("tham số đọc sai: %+v", mot.ThamSo)
	}
}

func TestTheoDoiTuongChuaGuiThiKhongCoMocGioVaKhongCoSoChe(t *testing.T) {
	// THREE NULLABLE COLUMNS, THREE DIFFERENT MEANINGS, and they land as zero values here. An
	// empty masked number means "not known yet", never "the empty number" — which is why the read
	// does not substitute anything for it.
	k := &khoGhiGia{dong: []dongThongBao{dongMau("tb-1", "2026-09-20T08:00:00Z")}}

	ra, err := dungKhoThongBao(k).TheoDoiTuong(ctxXa(xaThu), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("TheoDoiTuong: %v", err)
	}
	if ra[0].NguoiNhanChe != "" || ra[0].LoiMa != "" || !ra[0].GuiLuc.IsZero() {
		t.Errorf("dòng chưa gửi mà mang kết quả: %+v", ra[0])
	}
}

func TestTheoDoiTuongHoSoChuaBaoGiGiTraDanhSachRong(t *testing.T) {
	k := &khoGhiGia{}

	ra, err := dungKhoThongBao(k).TheoDoiTuong(ctxXa(xaThu), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("sổ rỗng phải là câu trả lời hợp lệ, nhận lỗi: %v", err)
	}
	if ra == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(ra) != 0 {
		t.Fatalf("nhận %d dòng từ một hồ sơ chưa được báo gì", len(ra))
	}
}

func TestTheoDoiTuongDungTranThiVanTraDu(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE history, not a refusal. An off-by-one here refuses a
	// record whose data is perfectly valid.
	k := &khoGhiGia{dong: nhieuDongThongBao(TranThongBaoMotHoSo)}

	ra, err := dungKhoThongBao(k).TheoDoiTuong(ctxXa(xaThu), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranThongBaoMotHoSo {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranThongBaoMotHoSo)
	}
}

func TestTheoDoiTuongVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// REFUSE, DO NOT TRUNCATE. This list answers "what did we tell this citizen, and when" — the
	// question a complaint opens with. A silently short answer is a commune stating it never sent
	// something it did send. The rows already read are DROPPED: a list handed back alongside an
	// error is a list that gets rendered.
	k := &khoGhiGia{dong: nhieuDongThongBao(TranThongBaoMotHoSo + 1)}

	ra, err := dungKhoThongBao(k).TheoDoiTuong(ctxXa(xaThu), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if !errors.Is(err, ErrQuaNhieuThongBao) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuThongBao", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

func TestTheoDoiTuongLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := &khoGhiGia{loiTruyVan: goc}

	ra, err := dungKhoThongBao(k).TheoDoiTuong(ctxXa(xaThu), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrQuaNhieuThongBao) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if ra != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
}

func TestTheoDoiTuongKhongCoXaTrongContextThiPanic(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A read that ran without a commune would either return every commune's
	// notifications or none, and both are silent. tenant.MustFrom panics by design; httpx.Recover
	// turns that into a traceable 500 at the edge. What must never happen is a default commune.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc sổ khi context không có xã mà không panic")
		}
	}()
	k := &khoGhiGia{dong: []dongThongBao{dongMau("tb-1", "2026-09-20T08:00:00Z")}}
	_, _ = dungKhoThongBao(k).TheoDoiTuong(context.Background(), domain.DoiTuongPhieuPhanAnh, maPhieuThu)
}

func nhieuDongThongBao(n int) []dongThongBao {
	ra := make([]dongThongBao, 0, n)
	for i := 0; i < n; i++ {
		ra = append(ra, dongMau(fmt.Sprintf("tb-%04d", i), "2026-09-20T08:00:00Z"))
	}
	return ra
}
