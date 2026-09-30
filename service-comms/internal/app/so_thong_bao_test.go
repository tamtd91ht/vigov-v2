package app

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
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// WHAT THIS FILE PROVES, and it is the half no store test can reach.
//
// The store suite proves the statements. This one proves the two decisions that sit above them,
// both of which fail SILENTLY:
//
//  1. THE ROW AND ITS AUDIT ENTRY SHARE ONE TRANSACTION (rule 6, invariant 3). That is a claim
//     about a transaction, so the fake below records Begin / Commit / Rollback and every
//     statement in order. A version that audited "afterwards, if it works" passes every test that
//     only checks both rows exist.
//  2. A REDELIVERY WRITES NEITHER. Queues deliver at least once (rule 2, invariant 5). Auditing
//     unconditionally would fill a public authority's ledger with entries saying it promised the
//     same citizen the same thing four times — every one of them false, and none of them
//     removable, because entries are append-only.
//
// NOT PROVED HERE: anything PostgreSQL does. The UNIQUE constraint that makes ON CONFLICT mean
// something, the CHECK constraints, and the immutability trigger all live in migration 0004 and
// are exercised only by the integration suite, which skips without VIGOV_TEST_DSN.

var xaThu = tenant.ID("01JA" + strings.Repeat("A", 22))

func ctxXa(xa tenant.ID) context.Context { return tenant.Into(context.Background(), xa) }

const (
	maPhieuThu   = "PA-2026-7F3K9Q"
	maCongDanThu = "cd-01JCONGDANMAUTHUNGHIEM"
	maBanGhiThu  = "01JTHONGBAOMAUTHUNGHIEM01"
)

// --- a database that records the order of things ------------------------------------------------

type khoGD struct {
	// buoc is the ORDER of everything that happened: "begin", "commit", "rollback", and the first
	// words of each statement. Order is the whole point — "insert, audit, commit" and "insert,
	// commit, audit" are the difference rule 6 invariant 3 is about, and a test that only counted
	// statements could not tell them apart.
	buoc []string
	loi  error
}

func (k *khoGD) Connect(context.Context) (driver.Conn, error) { return &connGD{k: k}, nil }
func (k *khoGD) Driver() driver.Driver                        { return trinhGD{} }

type trinhGD struct{}

func (trinhGD) Open(string) (driver.Conn, error) { return nil, errors.New("chỉ dùng Connector") }

type connGD struct{ k *khoGD }

func (c *connGD) Prepare(string) (driver.Stmt, error) { return nil, errors.New("không hỗ trợ") }
func (c *connGD) Close() error                        { return nil }
func (c *connGD) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *connGD) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.k.buoc = append(c.k.buoc, "begin")
	return &gdGD{k: c.k}, nil
}

type gdGD struct{ k *khoGD }

func (g *gdGD) Commit() error   { g.k.buoc = append(g.k.buoc, "commit"); return nil }
func (g *gdGD) Rollback() error { g.k.buoc = append(g.k.buoc, "rollback"); return nil }

type ketQuaGD struct{}

func (ketQuaGD) LastInsertId() (int64, error) { return 0, errors.New("không dùng") }
func (ketQuaGD) RowsAffected() (int64, error) { return 1, nil }

func (c *connGD) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	c.k.buoc = append(c.k.buoc, tomTat(q))
	if c.k.loi != nil {
		return nil, c.k.loi
	}
	return ketQuaGD{}, nil
}

func (c *connGD) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	c.k.buoc = append(c.k.buoc, tomTat(q))
	return &rowsRong{}, nil
}

type rowsRong struct{}

func (rowsRong) Columns() []string         { return []string{"x"} }
func (rowsRong) Close() error              { return nil }
func (rowsRong) Next([]driver.Value) error { return io.EOF }

// tomTat names a statement by what it touches, so the recorded sequence reads like the story it
// is meant to tell.
func tomTat(q string) string {
	switch {
	case strings.Contains(q, "audit_log"):
		return "audit"
	case strings.Contains(q, "INSERT INTO thong_bao_gui_cong_dan"):
		return "chen"
	case strings.Contains(q, "UPDATE thong_bao_gui_cong_dan"):
		return "capnhat"
	default:
		return "khac"
	}
}

// --- a store that answers what the test wants ---------------------------------------------------

type khoThongBaoGia struct {
	moi     bool
	doi     bool
	loiChen error
	loiKQ   error

	daChen   int
	daGhiKQ  int
	nhanDuoc domain.ThongBaoGuiCongDan
}

func (k *khoThongBaoGia) Chen(ctx context.Context, tx *pkgstore.ScopedTx,
	tb domain.ThongBaoGuiCongDan) (bool, error) {
	k.daChen++
	k.nhanDuoc = tb
	if k.loiChen != nil {
		return false, k.loiChen
	}
	// A real write inside the transaction, so the recorded sequence carries it.
	if _, err := tx.Exec(ctx, "INSERT INTO thong_bao_gui_cong_dan (tenant_id) VALUES ($1)",
		string(tx.TenantID())); err != nil {
		return false, err
	}
	return k.moi, nil
}

func (k *khoThongBaoGia) GhiKetQua(ctx context.Context, tx *pkgstore.ScopedTx, _ string,
	_ domain.KetQuaGui) (bool, error) {
	k.daGhiKQ++
	if k.loiKQ != nil {
		return false, k.loiKQ
	}
	if _, err := tx.Exec(ctx, "UPDATE thong_bao_gui_cong_dan SET trang_thai = $2 WHERE tenant_id = $1",
		string(tx.TenantID()), "da-gui"); err != nil {
		return false, err
	}
	return k.doi, nil
}

func dungSo(t *testing.T, k *khoGD, kho *khoThongBaoGia) *SoThongBao {
	t.Helper()
	uc := NewSoThongBao(pkgstore.New(sql.OpenDB(k)), kho)
	// The id is pinned so assertions can name it. In production it is maULID.
	uc.sinhID = func() (string, error) { return maBanGhiThu, nil }
	return uc
}

func yeuCauThu() YeuCauGhiNo {
	return YeuCauGhiNo{
		DoiTuongLoai: domain.DoiTuongPhieuPhanAnh,
		DoiTuongMa:   maPhieuThu,
		Moc:          "da-chuyen-xu-ly",
		Lan:          1,
		Kenh:         domain.KenhZaloZNS,
		NguoiNhanMa:  maCongDanThu,
		MauMa:        "zns-phan-anh-doi-trang-thai",
		ThamSo: domain.ThamSoThongBao{
			MaTraCuu:     maPhieuThu,
			MocNhan:      "Đã chuyển xử lý",
			ViecTiepTheo: "Cán bộ phụ trách sẽ liên hệ với ông/bà trong 2 ngày làm việc.",
		},
	}
}

// --- (1) the row and its entry share one transaction ---------------------------------------------

func TestGhiNoGhiSoVaNhatKyTrongCUNGMotGiaoDich(t *testing.T) {
	// THE INVARIANT THIS WHOLE LAYER EXISTS FOR. The measured defect on the previous project was
	// zero transactions across the whole backend, so "every write leaves a trail" could not hold:
	// there was always a window where the record had changed and the trail had not.
	//
	// The SEQUENCE is asserted, not the presence of both statements. "chen, audit, commit" and
	// "chen, commit, audit" both contain the same two writes; only the first one is rule 6.
	k := &khoGD{}
	kho := &khoThongBaoGia{moi: true}

	kq, err := dungSo(t, k, kho).GhiNo(ctxXa(xaThu), yeuCauThu())
	if err != nil {
		t.Fatalf("GhiNo: %v", err)
	}
	if !kq.Moi || kq.ID != maBanGhiThu {
		t.Errorf("kết quả sai: %+v", kq)
	}

	muon := []string{"begin", "chen", "audit", "commit"}
	if !bangNhau(k.buoc, muon) {
		t.Fatalf("thứ tự = %v, muốn %v", k.buoc, muon)
	}
}

func TestGhiNoNhatKyHongThiKHONGCoDongNaoDuocChot(t *testing.T) {
	// THE OTHER HALF OF THE SAME INVARIANT, and the half a "belt and braces" instinct usually
	// removes. If the audit entry fails, the obligation must not be recorded either: a commitment
	// nobody can attribute is the state the records rules do not permit. The queue will redeliver
	// and the whole thing will be attempted again — which is safe precisely because the insert is
	// idempotent.
	k := &khoGD{loi: errors.New("audit_log không ghi được")}
	kho := &khoThongBaoGia{moi: true}

	if _, err := dungSo(t, k, kho).GhiNo(ctxXa(xaThu), yeuCauThu()); err == nil {
		t.Fatal("nhật ký hỏng mà GhiNo vẫn báo thành công")
	}
	for _, b := range k.buoc {
		if b == "commit" {
			t.Fatalf("đã chốt giao dịch dù nhật ký hỏng: %v", k.buoc)
		}
	}
	if k.buoc[len(k.buoc)-1] != "rollback" {
		t.Errorf("không huỷ giao dịch: %v", k.buoc)
	}
}

// --- (2) a redelivery writes neither --------------------------------------------------------------

func TestGhiNoGiaoLaiThiKhongGhiNhatKyThuHai(t *testing.T) {
	// THE DECISION THIS PINS. `Chen` reports that nothing was inserted, so no entry is owed.
	// Auditing anyway would record that the commune promised the same citizen the same thing
	// twice — and audit entries are append-only (rule 6, invariant 4), so the false statement
	// could never be corrected, only added to.
	//
	// It must also NOT be an error: an at-least-once queue makes this normal operation, and a
	// consumer that treats it as failure retries forever or dead-letters a message that was
	// handled correctly.
	k := &khoGD{}
	kho := &khoThongBaoGia{moi: false}

	kq, err := dungSo(t, k, kho).GhiNo(ctxXa(xaThu), yeuCauThu())
	if err != nil {
		t.Fatalf("giao lại phải là no-op, nhận lỗi: %v", err)
	}
	if kq.Moi {
		t.Error("giao lại mà báo là dòng mới")
	}
	for _, b := range k.buoc {
		if b == "audit" {
			t.Fatalf("giao lại vẫn ghi nhật ký: %v", k.buoc)
		}
	}
}

func TestGhiNoHaiLanLienTiepChiSinhMotVetVaMotDong(t *testing.T) {
	// THE SAME PROPERTY SEEN FROM THE CALLER'S SIDE, because that is how it will actually happen:
	// the same envelope delivered twice, seconds apart, to the same consumer. The first call
	// inserts and audits; the second must do neither.
	k := &khoGD{}
	kho := &khoThongBaoGia{moi: true}
	uc := dungSo(t, k, kho)

	if _, err := uc.GhiNo(ctxXa(xaThu), yeuCauThu()); err != nil {
		t.Fatalf("lần một: %v", err)
	}
	kho.moi = false // the UNIQUE constraint swallowed the second insert
	if _, err := uc.GhiNo(ctxXa(xaThu), yeuCauThu()); err != nil {
		t.Fatalf("lần hai: %v", err)
	}

	if n := dem(k.buoc, "audit"); n != 1 {
		t.Fatalf("ghi %d vết cho một cam kết, muốn 1 — sổ của cơ quan sẽ nói xã hứa hai lần", n)
	}
}

// --- what the ledger and the trail must not carry ---------------------------------------------------

func TestGhiNoKhoaChongTrungMangDuMocVaLanChuKhongMangDuLieuCaNhan(t *testing.T) {
	// THE KEY IS WHAT DECIDES WHETHER A CITIZEN IS TOLD TWICE, so what goes into it is asserted
	// rather than assumed. `lan` is the component that is easy to drop as redundant: without it a
	// petition closed, reopened and closed AGAIN (ADR 0008) owes a second notification that would
	// be swallowed as a duplicate — a failure that looks exactly like correct deduplication.
	k := &khoGD{}
	kho := &khoThongBaoGia{moi: true}
	uc := dungSo(t, k, kho)

	yc := yeuCauThu()
	yc.Lan = 2
	if _, err := uc.GhiNo(ctxXa(xaThu), yc); err != nil {
		t.Fatalf("GhiNo: %v", err)
	}
	khoa := kho.nhanDuoc.KhoaLanGui
	if !strings.Contains(khoa, "|2|") {
		t.Errorf("khoá không phân biệt lần thứ mấy: %q", khoa)
	}
	if !strings.Contains(khoa, "da-chuyen-xu-ly") {
		t.Errorf("khoá không phân biệt mốc: %q", khoa)
	}
}

func TestGhiNoKhongCoLanThiCoiLaLanMot(t *testing.T) {
	// NORMALISED EXPLICITLY, not accepted as a zero. `lan` is part of the deduplication key, so a
	// producer sending 0 and another sending 1 would make two keys for one fact — and the citizen
	// receives the message twice.
	k := &khoGD{}
	kho := &khoThongBaoGia{moi: true}

	yc := yeuCauThu()
	yc.Lan = 0
	if _, err := dungSo(t, k, kho).GhiNo(ctxXa(xaThu), yc); err != nil {
		t.Fatalf("GhiNo: %v", err)
	}
	if kho.nhanDuoc.Lan != 1 {
		t.Fatalf("lan = %d, muốn 1", kho.nhanDuoc.Lan)
	}
	if !strings.Contains(kho.nhanDuoc.KhoaLanGui, "|1|") {
		t.Errorf("khoá không mang lần 1: %q", kho.nhanDuoc.KhoaLanGui)
	}
}

func TestGhiNoNoiDungKhongDatThiKhongMoGiaoDich(t *testing.T) {
	// RULE 10, INVARIANT 6, ENFORCED BEFORE A ROW LOCK IS TAKEN. A message that names a state and
	// no consequence tells the citizen nothing they can act on, and refusing it inside the
	// transaction would hold locks while doing so and hand the caller a rollback instead of a
	// reason.
	k := &khoGD{}
	kho := &khoThongBaoGia{moi: true}

	yc := yeuCauThu()
	yc.ThamSo.ViecTiepTheo = ""

	if _, err := dungSo(t, k, kho).GhiNo(ctxXa(xaThu), yc); !errors.Is(err, domain.ErrThieuViecTiepTheo) {
		t.Fatalf("lỗi = %v, muốn ErrThieuViecTiepTheo", err)
	}
	if len(k.buoc) != 0 {
		t.Errorf("đã chạm cơ sở dữ liệu: %v", k.buoc)
	}
	if kho.daChen != 0 {
		t.Errorf("đã gọi kho %d lần", kho.daChen)
	}
}

func TestGhiNoThieuTungTruongBatBuocThiTuChoi(t *testing.T) {
	ca := []struct {
		ten  string
		sua  func(*YeuCauGhiNo)
		muon error
	}{
		{"thiếu mã hồ sơ", func(y *YeuCauGhiNo) { y.DoiTuongMa = "" }, ErrThieuDoiTuong},
		{"thiếu mốc", func(y *YeuCauGhiNo) { y.Moc = "" }, ErrThieuMoc},
		{"thiếu người nhận", func(y *YeuCauGhiNo) { y.NguoiNhanMa = "" }, ErrThieuNguoiNhan},
		{"thiếu mẫu tin", func(y *YeuCauGhiNo) { y.MauMa = "" }, ErrThieuMauTin},
	}
	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			k := &khoGD{}
			kho := &khoThongBaoGia{moi: true}
			yc := yeuCauThu()
			c.sua(&yc)
			if _, err := dungSo(t, k, kho).GhiNo(ctxXa(xaThu), yc); !errors.Is(err, c.muon) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.muon)
			}
			if len(k.buoc) != 0 {
				t.Errorf("đã chạm cơ sở dữ liệu: %v", k.buoc)
			}
		})
	}
}

func TestGhiNoKhongCoXaTrongContextThiPanic(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A consumer whose message carried no commune must refuse, never guess
	// (rule 1, invariant 9). core/events.Dispatch refuses first; this is the second line, and it
	// is a panic rather than an error because a write that ran without a commune would land in
	// whichever commune happened to be convenient.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ghi sổ khi context không có xã mà không panic")
		}
	}()
	k := &khoGD{}
	kho := &khoThongBaoGia{moi: true}
	_, _ = dungSo(t, k, kho).GhiNo(context.Background(), yeuCauThu())
}

// --- the outcome path -------------------------------------------------------------------------------

func TestGhiKetQuaGhiKetQuaVaVetTrongCungMotGiaoDich(t *testing.T) {
	k := &khoGD{}
	kho := &khoThongBaoGia{doi: true}

	doi, err := dungSo(t, k, kho).GhiKetQua(ctxXa(xaThu), maBanGhiThu, domain.KetQuaGui{
		TrangThai:    domain.DaGui,
		GuiLuc:       time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC),
		NguoiNhanChe: "09****0000",
		SoLanThu:     1,
	}, audit.Actor{}, maPhieuThu)
	if err != nil {
		t.Fatalf("GhiKetQua: %v", err)
	}
	if !doi {
		t.Error("dòng đã đổi mà báo là không")
	}
	muon := []string{"begin", "capnhat", "audit", "commit"}
	if !bangNhau(k.buoc, muon) {
		t.Fatalf("thứ tự = %v, muốn %v", k.buoc, muon)
	}
}

func TestGhiKetQuaBaoLaiKetQuaCuThiKhongGhiVetThuHai(t *testing.T) {
	k := &khoGD{}
	kho := &khoThongBaoGia{doi: false}

	doi, err := dungSo(t, k, kho).GhiKetQua(ctxXa(xaThu), maBanGhiThu, domain.KetQuaGui{
		TrangThai: domain.ThatBai, LoiMa: "zns-429", SoLanThu: 3,
	}, audit.Actor{}, maPhieuThu)
	if err != nil {
		t.Fatalf("báo lại phải là no-op, nhận lỗi: %v", err)
	}
	if doi {
		t.Error("không dòng nào đổi mà báo là đã đổi")
	}
	if dem(k.buoc, "audit") != 0 {
		t.Errorf("vẫn ghi vết cho một thay đổi không xảy ra: %v", k.buoc)
	}
}

func TestGhiKetQuaThieuMaHoSoThiTuChoiVaKhongMoGiaoDich(t *testing.T) {
	// The audit entry's subject is the BUSINESS code, and core/audit refuses an entry without one.
	// Failing here, before the transaction, means the caller gets the reason instead of a rollback
	// from inside a closure.
	k := &khoGD{}
	kho := &khoThongBaoGia{doi: true}

	_, err := dungSo(t, k, kho).GhiKetQua(ctxXa(xaThu), maBanGhiThu,
		domain.KetQuaGui{TrangThai: domain.ThatBai}, audit.Actor{}, "")
	if !errors.Is(err, ErrThieuDoiTuong) {
		t.Fatalf("lỗi = %v, muốn ErrThieuDoiTuong", err)
	}
	if len(k.buoc) != 0 {
		t.Errorf("đã chạm cơ sở dữ liệu: %v", k.buoc)
	}
}

// --- who the trail attributes it to -----------------------------------------------------------------

func TestNguoiGayMacDinhLaHeThongChuKhongPhaiRong(t *testing.T) {
	// RULE 6, INVARIANT 6: system actions are audited too, WITH A SYSTEM PRINCIPAL. core/audit
	// refuses an entry with no actor, so leaving it empty would make a consumer unable to record
	// anything — and the fix somebody reaches for next is passing a staff id that did no work.
	if a := nguoiGay(audit.Actor{}); a.ID != audit.SystemActor || a.Kind != "system" {
		t.Fatalf("người gây mặc định = %+v, muốn nguyên tắc hệ thống", a)
	}
	// A named actor is never overwritten: when a member of staff causes a notification, the trail
	// must say so.
	nguoi := audit.Actor{ID: "CB-007", Kind: "staff", IP: "10.0.0.7"}
	if a := nguoiGay(nguoi); a != nguoi {
		t.Fatalf("người gây = %+v, muốn %+v", a, nguoi)
	}
}

// --- the id -------------------------------------------------------------------------------------------

func TestMaULIDDung26KyTuVaChiDungBangChuCrockford(t *testing.T) {
	// The alphabet matters beyond neatness: identity's copy (and the CHECK constraint beside it)
	// uses exactly this one, and an id that differs in alphabet or length between two services is
	// an id that fails a constraint in one of them. See the comment on maULID about the second
	// copy.
	thay := map[string]bool{}
	for i := 0; i < 64; i++ {
		ma, err := maULID()
		if err != nil {
			t.Fatalf("maULID: %v", err)
		}
		if len(ma) != 26 {
			t.Fatalf("độ dài = %d, muốn 26: %q", len(ma), ma)
		}
		for _, r := range ma {
			if !strings.ContainsRune(chuCaiULID, r) {
				t.Fatalf("ký tự ngoài bảng chữ Crockford: %q trong %q", r, ma)
			}
		}
		if thay[ma] {
			t.Fatalf("hai mã trùng nhau: %q", ma)
		}
		thay[ma] = true
	}
}

// --- helpers -------------------------------------------------------------------------------------------

func bangNhau(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func dem(a []string, x string) int {
	n := 0
	for _, v := range a {
		if v == x {
			n++
		}
	}
	return n
}
