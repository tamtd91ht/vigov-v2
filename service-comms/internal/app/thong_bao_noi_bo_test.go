package app

// The use case behind POST /api/v1/announcements, run over the REAL store over a FAKE DRIVER.
//
// WHY A DRIVER AND NOT A FAKE STORE — the same argument driver_gia_danh_muc_test.go makes, and it
// holds harder here because this act has TWO writes plus a trail. What this package is responsible
// for is not "the use case called a method", it is:
//
//	the announcement, its recipients AND the audit entry are in ONE transaction (rule 6, invariant 3)
//	a refusal writes NOTHING and commits nothing
//	the state and the author are decided here and never taken from the request
//	`trang_thai_thu` is `chua-gui` whatever the request asked for
//
// Every one of those is a property of the SQL and of the transaction boundaries, and a fake store
// erases exactly that.
//
// WHAT IT STILL DOES NOT PROVE, said plainly so nobody reads more into a green run than is there:
// nothing PostgreSQL does with these statements. Not the CHECK constraints, not the immutability
// triggers of migration 0005, not the foreign keys, not partition routing. That half needs a real
// server; the pg suite in internal/store is where it lands the day a DSN exists.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// khoTBGia records every statement and every transaction boundary. It answers no query, because
// this use case runs none — a fake that invented an answer to a query nobody makes would be a fake
// that keeps working after the code starts making one.
type khoTBGia struct {
	lenh []lenhGhi

	batDau, daCommit, daRollback int

	// loiSau fails the FIRST statement containing this substring, and only that one.
	//
	// WHY NOT A BLANKET ERROR: failing everything cannot tell "rolled back" from "never started".
	// The invariant worth proving is that a failure on the LAST statement of the transaction — the
	// audit entry — takes both business writes down with it, and that needs everything before it to
	// have already run.
	loiSau string
	daNo   bool
}

func (k *khoTBGia) Connect(context.Context) (driver.Conn, error) { return &connTBGia{k: k}, nil }
func (k *khoTBGia) Driver() driver.Driver                        { return trinhTBGia{} }

type trinhTBGia struct{}

func (trinhTBGia) Open(string) (driver.Conn, error) {
	return nil, errors.New("driver giả thông báo: chỉ dùng Connector")
}

type connTBGia struct{ k *khoTBGia }

func (c *connTBGia) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("driver giả thông báo: không hỗ trợ Prepare")
}
func (c *connTBGia) Close() error { return nil }

func (c *connTBGia) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connTBGia) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.batDau++
	return &txTBGia{k: c.k}, nil
}

func (c *connTBGia) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	gt := make([]driver.Value, 0, len(args))
	for _, a := range args {
		gt = append(gt, a.Value)
	}
	c.k.lenh = append(c.k.lenh, lenhGhi{sql: q, args: gt})
	if c.k.loiSau != "" && !c.k.daNo && strings.Contains(q, c.k.loiSau) {
		c.k.daNo = true
		return nil, errors.New("driver giả thông báo: câu lệnh này được dựng để hỏng")
	}
	return driver.RowsAffected(1), nil
}

func (c *connTBGia) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	return nil, errors.New("driver giả thông báo: use case này không truy vấn gì — " + q)
}

type txTBGia struct{ k *khoTBGia }

func (t *txTBGia) Commit() error   { t.k.daCommit++; return nil }
func (t *txTBGia) Rollback() error { t.k.daRollback++; return nil }

// cau returns every recorded statement containing `tu`, so an assertion names the statement it
// cares about rather than an index that shifts when a check is added.
func (k *khoTBGia) cau(tu string) []lenhGhi {
	var ra []lenhGhi
	for _, l := range k.lenh {
		if strings.Contains(l.sql, tu) {
			ra = append(ra, l)
		}
	}
	return ra
}

func (k *khoTBGia) coCau(tu string) bool { return len(k.cau(tu)) > 0 }

const (
	idThongBaoPinned = "01JTHONGBAOCODINH00000000"
	maCanBoSoan      = "CB-2026-7K3M9Q"
)

var lucPinned = time.Date(2026, 9, 23, 8, 30, 0, 0, time.UTC)

// dungUseCaseThongBao builds the REAL use case over the REAL store over the fake driver, with the
// id AND THE CLOCK pinned so assertions can name both.
func dungUseCaseThongBao(t *testing.T, k *khoTBGia) (*SoanThongBaoNoiBo, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	// ONE CONNECTION, so every statement of one transaction lands on the same fake and in order. A
	// pool would interleave them and the ordering assertions would be about the scheduler.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	kho := store.New(db)
	uc := NewSoanThongBaoNoiBo(kho, commsstore.NewThongBaoNoiBoStore(kho))
	uc.sinhID = func() (string, error) { return idThongBaoPinned, nil }
	uc.bayGio = func() time.Time { return lucPinned }
	return uc, tenant.Into(context.Background(), xaA)
}

func nguoiSoanMau() audit.Actor {
	return audit.Actor{ID: maCanBoSoan, Kind: "staff", IP: "10.0.0.7"}
}

func ycMau() domain.YeuCauSoanThongBao {
	return domain.YeuCauSoanThongBao{
		TieuDe:      "Mời họp giao ban tháng 9",
		NoiDung:     "Kính mời các đồng chí dự họp.",
		NguoiNhanMa: []string{"CB-2026-AAAA11", "CB-2026-BBBB22"},
	}
}

// --- the happy path ---------------------------------------------------------------------------

func TestPhatHanhGhiBaCauTrongMotGiaoDich(t *testing.T) {
	// RULE 6, INVARIANT 3, AS A PROPERTY OF THE TRANSACTION BOUNDARIES RATHER THAN A CLAIM. The
	// announcement, the recipients and the trail are three statements; if any of them could land in
	// a different transaction, a commune could end up with a notice nobody can be asked about.
	k := &khoTBGia{}
	uc, ctx := dungUseCaseThongBao(t, k)

	moi, err := uc.PhatHanh(ctx, ycMau(), nguoiSoanMau())
	if err != nil {
		t.Fatalf("phát hành lỗi: %v", err)
	}

	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở=%d chốt=%d huỷ=%d, muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}
	for _, muon := range []string{"INSERT INTO thong_bao\n", "INSERT INTO thong_bao_nguoi_nhan", "INSERT INTO audit_log"} {
		if !k.coCau(muon) {
			t.Errorf("thiếu câu lệnh %q", muon)
		}
	}
	if moi.ID != idThongBaoPinned {
		t.Errorf("id = %q, muốn %q", moi.ID, idThongBaoPinned)
	}
}

func TestPhatHanhQuyetDinhTrangThaiVaNguoiSoanTaiDay(t *testing.T) {
	// The state and the author are NOT in the request (domain.YeuCauSoanThongBao has no field for
	// either). This asserts what actually reaches the INSERT, which is the only place a future
	// "small" change could reintroduce a client-chosen state.
	k := &khoTBGia{}
	uc, ctx := dungUseCaseThongBao(t, k)

	yc := ycMau()
	yc.GuiThuDienTu = true // the checkbox was ticked
	if _, err := uc.PhatHanh(ctx, yc, nguoiSoanMau()); err != nil {
		t.Fatalf("phát hành lỗi: %v", err)
	}

	l := k.cau("INSERT INTO thong_bao\n")
	if len(l) != 1 {
		t.Fatalf("số câu chèn thông báo = %d, muốn 1", len(l))
	}
	// $1 tenant, $2 id, $3 tiêu đề, $4 nội dung, $5 trạng thái, $6 ghim, $7 bắt buộc xác nhận,
	// $8 gửi thư, $9 người soạn, $10 phát hành lúc.
	args := l[0].args
	if len(args) != 10 {
		t.Fatalf("số tham số = %d, muốn 10", len(args))
	}
	if args[0] != string(xaA) {
		t.Errorf("xã = %v, muốn %v — xã đến từ ngữ cảnh, không từ yêu cầu", args[0], xaA)
	}
	if args[4] != string(domain.ThongBaoDaPhatHanh) {
		t.Errorf("trạng thái = %v, muốn da-phat-hanh", args[4])
	}
	if args[8] != maCanBoSoan {
		t.Errorf("người soạn = %v, muốn mã cán bộ từ phiên (luật 6, bất biến 8)", args[8])
	}
	if args[9] != lucPinned {
		t.Errorf("phát hành lúc = %v, muốn %v — một lần đọc đồng hồ cho cả hành vi", args[9], lucPinned)
	}
	// `trang_thai_thu` IS NOT A PARAMETER OF THIS STATEMENT. The ticked checkbox is recorded in
	// `gui_thu_dien_tu`; the mail STATE stays at its default because nothing has sent anything.
	if strings.Contains(l[0].sql, "trang_thai_thu") {
		t.Error("câu chèn không được nhận trang_thai_thu: đó là kết quả của một lần gửi chưa xảy ra")
	}
}

func TestPhatHanhGhiMoiNguoiNhanLaDichDanh(t *testing.T) {
	k := &khoTBGia{}
	uc, ctx := dungUseCaseThongBao(t, k)

	if _, err := uc.PhatHanh(ctx, ycMau(), nguoiSoanMau()); err != nil {
		t.Fatalf("phát hành lỗi: %v", err)
	}
	l := k.cau("INSERT INTO thong_bao_nguoi_nhan")
	if len(l) != 1 {
		t.Fatalf("số câu chèn người nhận = %d, muốn 1 — một câu cho cả danh sách", len(l))
	}
	// $1 xã, $2 id thông báo, rồi từng cặp (mã, đích danh).
	args := l[0].args
	if len(args) != 2+2*2 {
		t.Fatalf("số tham số = %d, muốn 6", len(args))
	}
	if args[2] != "CB-2026-AAAA11" || args[4] != "CB-2026-BBBB22" {
		t.Errorf("mã người nhận = %v, %v", args[2], args[4])
	}
	if args[3] != true || args[5] != true {
		t.Error("mọi người nhận trong lượt này phải là đích danh: không có đường nào khác vào danh sách")
	}
}

func TestPhatHanhViDeKiemToanLaMaCanBoVaKhongMangNoiDung(t *testing.T) {
	// TWO RULES IN ONE ASSERTION, and both have a measured history in this repository:
	//   rule 6, invariant 8 — `actor_id` is `CB-…`, never a ULID. Six write paths got this wrong on
	//                          2026-09-22 and no test turned red.
	//   rule 3, forbidden #5 — the audit ledger is never deleted, so the title and the body (free
	//                          text that can name a person) must not enter it.
	k := &khoTBGia{}
	uc, ctx := dungUseCaseThongBao(t, k)

	if _, err := uc.PhatHanh(ctx, ycMau(), nguoiSoanMau()); err != nil {
		t.Fatalf("phát hành lỗi: %v", err)
	}
	l := k.cau("INSERT INTO audit_log")
	if len(l) != 1 {
		t.Fatalf("số vết kiểm toán = %d, muốn 1", len(l))
	}
	args := l[0].args
	// $1 xã, $2 actor_id, $3 actor_kind, $4 actor_ip, $5 action, $6 subject, $7 at, $8 delta.
	if args[1] != maCanBoSoan {
		t.Errorf("actor_id = %v, muốn mã cán bộ %q", args[1], maCanBoSoan)
	}
	if args[4] != HanhViPhatHanhThongBao {
		t.Errorf("action = %v, muốn %q", args[4], HanhViPhatHanhThongBao)
	}
	chuDe, _ := args[5].(string)
	if !strings.HasPrefix(chuDe, "thong-bao/2026-09-23/") || !strings.HasSuffix(chuDe, maCanBoSoan) {
		t.Errorf("subject = %q, muốn thong-bao/<ngày>/<mã cán bộ>", chuDe)
	}
	if strings.Contains(chuDe, "Mời họp") {
		t.Error("tiêu đề lọt vào chủ đề vết kiểm toán — sổ này không bao giờ bị xoá (luật 3)")
	}
	delta, _ := args[7].([]byte)
	for _, cam := range []string{"Mời họp", "Kính mời"} {
		if strings.Contains(string(delta), cam) {
			t.Errorf("delta chứa %q — tiêu đề và nội dung không được vào sổ kiểm toán", cam)
		}
	}
	// What the delta MUST carry: who was told. Without it the entry cannot answer the question an
	// inspection opens with.
	if !strings.Contains(string(delta), "CB-2026-AAAA11") {
		t.Error("delta thiếu mã người nhận — vết không trả lời được ai đã được báo")
	}
}

// --- the refusals ------------------------------------------------------------------------------

func TestPhatHanhTheoBoPhanKhongGhiGiVaKhongMoGiaoDich(t *testing.T) {
	// THE REFUSAL THIS PASS EXISTS TO MAKE LOUD, asserted as "nothing happened" rather than as an
	// error value: a version that recorded the departments and delivered to nobody would return the
	// same error here if the check were moved, and only the statement count catches that.
	k := &khoTBGia{}
	uc, ctx := dungUseCaseThongBao(t, k)

	yc := ycMau()
	yc.BoPhanIDs = []string{"01JBOPHAN0000000000000000"}
	_, err := uc.PhatHanh(ctx, yc, nguoiSoanMau())
	if !errors.Is(err, ErrGuiTheoBoPhanChuaCo) {
		t.Fatalf("lỗi = %v, muốn ErrGuiTheoBoPhanChuaCo", err)
	}
	if len(k.lenh) != 0 || k.batDau != 0 {
		t.Errorf("từ chối mà vẫn chạy %d câu lệnh và mở %d giao dịch", len(k.lenh), k.batDau)
	}
}

func TestPhatHanhKhongCoNguoiNhanBiTuChoi(t *testing.T) {
	// An announcement that reaches nobody is the one failure of this module that produces no error
	// anywhere: the card appears in the book and the commune is simply never told.
	k := &khoTBGia{}
	uc, ctx := dungUseCaseThongBao(t, k)

	yc := ycMau()
	yc.NguoiNhanMa = nil
	if _, err := uc.PhatHanh(ctx, yc, nguoiSoanMau()); !errors.Is(err, domain.ErrKhongCoNguoiNhan) {
		t.Fatalf("lỗi = %v, muốn ErrKhongCoNguoiNhan", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("từ chối mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

func TestPhatHanhBoPhanBaoTruocKhiBaoThieuNguoiNhan(t *testing.T) {
	// ORDER OF THE TWO REFUSALS. An author who ticked only departments must be told the capability
	// is missing — not "you addressed this to nobody", which is true, useless, and sends them
	// looking for a mistake in their own form.
	k := &khoTBGia{}
	uc, ctx := dungUseCaseThongBao(t, k)

	yc := ycMau()
	yc.NguoiNhanMa = nil
	yc.BoPhanIDs = []string{"01JBOPHAN0000000000000000"}
	if _, err := uc.PhatHanh(ctx, yc, nguoiSoanMau()); !errors.Is(err, ErrGuiTheoBoPhanChuaCo) {
		t.Fatalf("lỗi = %v, muốn ErrGuiTheoBoPhanChuaCo", err)
	}
}

func TestPhatHanhThieuMaCanBoSoanBiTuChoi(t *testing.T) {
	// `nguoi_soan_ma` with nothing in it is an announcement nobody can be asked about, and there is
	// NO FALLBACK to an internal id (rule 6, invariant 8).
	k := &khoTBGia{}
	uc, ctx := dungUseCaseThongBao(t, k)

	if _, err := uc.PhatHanh(ctx, ycMau(), audit.Actor{Kind: "staff", IP: "10.0.0.7"}); !errors.Is(err, ErrThieuNguoiSoan) {
		t.Fatalf("lỗi = %v, muốn ErrThieuNguoiSoan", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("từ chối mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

func TestPhatHanhVetKiemToanHongThiCuonLaiCaHaiCauGhi(t *testing.T) {
	// THE INVARIANT THAT ONLY A TRANSACTION TEST CAN SHOW. If the trail fails, the announcement and
	// its recipients must go with it: rule 6, invariant 3 does not permit a business write whose
	// entry failed, and rule 7 would make the orphaned announcement permanent.
	k := &khoTBGia{loiSau: "INSERT INTO audit_log"}
	uc, ctx := dungUseCaseThongBao(t, k)

	if _, err := uc.PhatHanh(ctx, ycMau(), nguoiSoanMau()); err == nil {
		t.Fatal("vết kiểm toán hỏng mà phát hành vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("giao dịch: chốt=%d huỷ=%d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestPhatHanhTieuDeRongBiTuChoiTruocKhiMoGiaoDich(t *testing.T) {
	// Shape first, outside the transaction. A request that fails its shape must never hold a
	// transaction open while doing so.
	k := &khoTBGia{}
	uc, ctx := dungUseCaseThongBao(t, k)

	yc := ycMau()
	yc.TieuDe = "   "
	if _, err := uc.PhatHanh(ctx, yc, nguoiSoanMau()); !errors.Is(err, domain.ErrTieuDeTrong) {
		t.Fatalf("lỗi = %v, muốn ErrTieuDeTrong", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch cho một yêu cầu sai hình dạng", k.batDau)
	}
}
