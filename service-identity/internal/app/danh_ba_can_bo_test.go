package app

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: the five write routes of the staff register stand on three customer
// decisions taken on 2026-09-22, and EVERY ONE OF THEM FAILS SILENTLY.
//
//	#13  a commune that loses its last administrator keeps working perfectly — until somebody has
//	     to change a permission, and then nobody in the commune can, and neither can the vendor
//	     (ADR 0003). Nothing turns red on the day the guard is removed.
//	#14  a missing self-check or a missing "cannot grant what you do not hold" check produces
//	     requests that SUCCEED. The 35-key permission table then describes a flat model while the
//	     system runs a model with one key above all the others, and the audit trail records every
//	     escalation as a perfectly ordinary role change.
//	#6   an audit entry written outside the transaction, or naming the internal id instead of the
//	     staff code, is invisible until an inspection asks who did something.
//
// It runs on the fake database/sql driver from dang_nhap_giao_dich_test.go — no PostgreSQL, no
// Redis — because a test that needs infrastructure is a test that stops being run. What that
// driver records is which statement ran inside which transaction and how the transaction ended,
// which is exactly what rule 6, invariant 3 turns on.
//
// WHAT THIS FILE CANNOT PROVE, and is therefore proven in store/can_bo_ghi_pg_test.go instead:
// that `FOR UPDATE` really serialises two administrators locking each other. A fake store agrees
// with whatever it is told, and a race is precisely the thing it cannot have an opinion about.

// --- fixtures ---------------------------------------------------------------------------------

const (
	idNguoiKhac  = "nd-01JNGUOIKHAC0000000000"
	maNguoiKhac  = "CB-002"
	idQuanTri2   = "nd-01JQUANTRIHAI0000000000"
	vaiTroYeu    = "vt-chuyen-vien"
	vaiTroManh   = "vt-chu-tich"
	diDongGia    = "0900000001"
	diDongGiaSau = "0900000002"
)

// nguoiThucHienGia is the administrator making every change below. The two identifiers are
// DIFFERENT VALUES ON PURPOSE: half this file exists to catch them being swapped — the guards
// compare on the internal id, the trail records the business code, and a swap makes both look
// like they are working.
func nguoiThucHienGia() NguoiThucHien {
	return NguoiThucHien{
		ID:  idNoiBo,
		Vet: audit.Actor{ID: maCanBo, Kind: "staff", IP: ipGia},
	}
}

func canBoGia() domain.CanBoTomTat {
	return domain.CanBoTomTat{
		ID: idNguoiKhac, Ma: maNguoiKhac,
		HoTen: "Trần Thị B", Email: "canbo.b@example.gov.vn",
		ChucVu: "Công chức Văn phòng", BoPhanID: "bp-001", VaiTroID: vaiTroYeu,
		DienThoaiCoQuan: dienThoaiGia, DiDongCaNhan: diDongGia,
		CoTaiKhoan: true, DangHoatDong: true,
		TaoLuc: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

// --- the fake store ---------------------------------------------------------------------------

// khoGia stands in for *idstore.CanBoStore.
//
// EVERY MUTATING METHOD RUNS A REAL STATEMENT THROUGH THE TRANSACTION IT WAS HANDED, and that is
// not decoration: it is what lets the driver record the business write and the audit entry with
// their transaction ids, so "the two share one transaction" is an assertion about what actually
// ran rather than about what the code looks like.
type khoGia struct {
	cb      domain.CanBoTomTat
	coDong  bool // false: the row is not there — TheoIDDeGhi answers ErrCanBoKhongTonTai
	quanTri []string
	// quyenVaiTro maps a role id to the keys it grants. A role ABSENT from the map does not
	// exist — which is how a soft-deleted role behaves, and it must not be confused with a role
	// that grants nothing (present, empty slice).
	quyenVaiTro map[string][]string
	quyenToi    []string

	// maDaDung makes the first Chen fail with ErrMaCanBoDaDung, as the unique key would.
	maDaDung  bool
	soLanChen int
	maDaThu   []string

	// congKhaiCuoi is the row DatCongKhai was handed — what the UPDATE would write.
	congKhaiCuoi *domain.CanBoTomTat

	loi error
}

func (k *khoGia) TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CanBoTomTat, error) {
	if k.loi != nil {
		return domain.CanBoTomTat{}, k.loi
	}
	if !k.coDong || id != k.cb.ID {
		return domain.CanBoTomTat{}, idstore.ErrCanBoKhongTonTai
	}
	// A real statement, so the driver can show this read happened INSIDE the transaction.
	if _, err := tx.Exec(ctx, "SELECT doc-de-ghi FROM nguoi_dung", string(tx.TenantID()), id); err != nil {
		return domain.CanBoTomTat{}, err
	}
	return k.cb, nil
}

func (k *khoGia) Chen(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error {
	k.soLanChen++
	k.maDaThu = append(k.maDaThu, cb.Ma)
	if k.maDaDung && k.soLanChen == 1 {
		return idstore.ErrMaCanBoDaDung
	}
	_, err := tx.Exec(ctx, "INSERT INTO nguoi_dung (dau-hieu-chen)", cb.ID, cb.Ma, cb.HoTen)
	return err
}

func (k *khoGia) CapNhatHoSo(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error {
	_, err := tx.Exec(ctx, "UPDATE nguoi_dung SET ho-so", cb.ID, cb.HoTen, cb.DiDongCaNhan)
	return err
}

func (k *khoGia) DatKhoa(ctx context.Context, tx *store.ScopedTx, id string, dangHoatDong bool) error {
	_, err := tx.Exec(ctx, "UPDATE nguoi_dung SET dang_hoat_dong", id, dangHoatDong)
	return err
}

func (k *khoGia) DatVaiTro(ctx context.Context, tx *store.ScopedTx, id, vaiTroID string) error {
	_, err := tx.Exec(ctx, "UPDATE nguoi_dung SET vai_tro_id", id, vaiTroID)
	return err
}

func (k *khoGia) DatCongKhai(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error {
	k.congKhaiCuoi = &cb
	_, err := tx.Exec(ctx, "UPDATE nguoi_dung SET cong-khai", cb.ID, cb.HienTrenMiniApp,
		cb.DongYCongKhaiLuc, cb.DongYCongKhaiGhiBoi, cb.ThuTuDanhBa)
	return err
}

func (k *khoGia) QuanTriDangHoatDong(context.Context, *store.ScopedTx) ([]string, error) {
	return k.quanTri, nil
}

func (k *khoGia) QuyenCuaVaiTro(_ context.Context, _ *store.ScopedTx, vaiTroID string) ([]string, bool, error) {
	if vaiTroID == "" {
		return nil, true, nil
	}
	ds, co := k.quyenVaiTro[vaiTroID]
	return ds, co, nil
}

func (k *khoGia) QuyenDangGiu(context.Context, *store.ScopedTx, string) ([]string, error) {
	return k.quyenToi, nil
}

// banThuDanhBa builds the use case over the fake driver, with the code and id generators pinned so
// an assertion can name the value that was written.
type banThuDanhBa struct {
	uc  *DanhBaCanBo
	kho *khoGia
	ghi *ghiChep
}

func dungBanThuDanhBa(t *testing.T) *banThuDanhBa {
	t.Helper()
	db, g := moDB(t)

	kho := &khoGia{
		cb:     canBoGia(),
		coDong: true,
		// Two administrators by default: the ordinary case, where #13 permits the operation. Every
		// test that wants the refusal shortens this list, so the guard has to be READ rather than
		// assumed from a fixture that could never have passed.
		quanTri: []string{idNoiBo, idQuanTri2},
		quyenVaiTro: map[string][]string{
			vaiTroYeu:  {"document.read"},
			vaiTroManh: {"admin.user", "budget.confirm"},
		},
		quyenToi: []string{"admin.user", "budget.confirm", "document.read"},
	}

	uc := NewDanhBaCanBo(db, kho)
	uc.sinhID = func() (string, error) { return "nd-moi-0001", nil }
	lan := 0
	uc.sinhMa = func(time.Time) (string, error) {
		lan++
		return "CB-2026-THU" + string(rune('0'+lan)), nil
	}
	uc.bayGio = func() time.Time { return time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC) }

	return &banThuDanhBa{uc: uc, kho: kho, ghi: g}
}

func yeuCauThemGia() YeuCauThemCanBo {
	return YeuCauThemCanBo{
		HoTen:           "Trần Thị B",
		ChucVu:          "Công chức Văn phòng",
		Email:           "canbo.b@example.gov.vn",
		BoPhanID:        "bp-001",
		DienThoaiCoQuan: dienThoaiGia,
		DiDongCaNhan:    diDongGia,
	}
}

// --- helpers ------------------------------------------------------------------------------------

// vetDaGhi returns the audit statements the driver saw, in order.
func vetDaGhi(g *ghiChep) []lenhGhi {
	g.mu.Lock()
	defer g.mu.Unlock()
	var ra []lenhGhi
	for _, l := range g.lenh {
		if strings.Contains(l.sql, "audit_log") {
			ra = append(ra, l)
		}
	}
	return ra
}

// motVet asserts that exactly one audit entry was written and returns it.
func motVet(t *testing.T, g *ghiChep) lenhGhi {
	t.Helper()
	ds := vetDaGhi(g)
	if len(ds) != 1 {
		t.Fatalf("có %d vết kiểm toán, muốn đúng 1 — rule 6 bất biến 1", len(ds))
	}
	return ds[0]
}

// The positions of core/audit.Write's INSERT arguments:
// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
const (
	viTriActor  = 1
	viTriHanhVi = 4
	viTriChuThe = 5
	viTriDelta  = 7
)

func chuoiArg(t *testing.T, l lenhGhi, i int) string {
	t.Helper()
	if i >= len(l.args) {
		t.Fatalf("vết chỉ có %d tham số, cần vị trí %d", len(l.args), i)
	}
	s, ok := l.args[i].(string)
	if !ok {
		t.Fatalf("tham số %d không phải chuỗi: %T", i, l.args[i])
	}
	return s
}

func deltaCua(t *testing.T, l lenhGhi) map[string]any {
	t.Helper()
	b, ok := l.args[viTriDelta].([]byte)
	if !ok {
		t.Fatalf("delta không phải []byte: %T", l.args[viTriDelta])
	}
	var ra map[string]any
	if err := json.Unmarshal(b, &ra); err != nil {
		t.Fatalf("delta không phải JSON: %v — %s", err, string(b))
	}
	return ra
}

// khongCoGhi asserts that nothing was written: no business statement, no audit entry. It is the
// assertion behind every refusal — a guard that answers an error AFTER writing the row has not
// guarded anything.
func khongCoGhi(t *testing.T, g *ghiChep) {
	t.Helper()
	if n := len(vetDaGhi(g)); n != 0 {
		t.Errorf("thao tác bị từ chối nhưng vẫn ghi %d vết kiểm toán", n)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, l := range g.lenh {
		if strings.HasPrefix(l.sql, "UPDATE") || strings.HasPrefix(l.sql, "INSERT") {
			t.Errorf("thao tác bị từ chối nhưng vẫn chạy câu ghi: %s", l.sql)
		}
	}
}

// --- Them ---------------------------------------------------------------------------------------

// The insert and its audit entry share ONE transaction, and that transaction committed (rule 6,
// invariant 3).
//
// MUTATION THAT MUST TURN THIS RED: move the audit.Write call out of the Tx closure, or drop it.
func TestThemCanBoGhiVetTrongCUNGGiaoDich(t *testing.T) {
	b := dungBanThuDanhBa(t)

	cb, err := b.uc.Them(ctxXa(xaThu), yeuCauThemGia(), nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if cb.Ma == "" {
		t.Fatal("cán bộ mới không có mã — hệ thống phải sinh mã (câu #15)")
	}

	chen := b.ghi.tim("dau-hieu-chen")
	if chen == nil {
		t.Fatal("không có câu INSERT nào")
	}
	vet := motVet(t, b.ghi)
	if vet.tx == 0 || vet.tx != chen.tx {
		t.Fatalf("vết ở giao dịch %d, bản ghi nghiệp vụ ở giao dịch %d — luật 6 bất biến 3 đòi CÙNG một giao dịch",
			vet.tx, chen.tx)
	}
	if ket := b.ghi.ketThucCua(vet.tx); ket != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", ket)
	}
}

// THE SUBJECT OF THE TRAIL IS THE STAFF CODE, AND THE ACTOR IS THE ACTOR'S OWN STAFF CODE — never
// either internal id (rule 6, invariant 8).
//
// MUTATION THAT MUST TURN THIS RED: build the actor from p.ID instead of p.Ma. Nothing else
// notices — a ULID is a perfectly valid string in that column.
func TestThemCanBoChuTheVetLaMaCanBoChuKhongPhaiID(t *testing.T) {
	b := dungBanThuDanhBa(t)

	cb, err := b.uc.Them(ctxXa(xaThu), yeuCauThemGia(), nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	vet := motVet(t, b.ghi)

	if got := chuoiArg(t, vet, viTriActor); got != maCanBo {
		t.Errorf("actor_id = %q, muốn mã cán bộ %q", got, maCanBo)
	}
	if got := chuoiArg(t, vet, viTriActor); got == idNoiBo {
		t.Error("actor_id đang mang ĐỊNH DANH NỘI BỘ — luật 6 bất biến 8")
	}
	if got := chuoiArg(t, vet, viTriChuThe); got != cb.Ma {
		t.Errorf("subject = %q, muốn mã của người vừa tạo %q", got, cb.Ma)
	}
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViThemCanBo {
		t.Errorf("action = %q, muốn %q", got, HanhViThemCanBo)
	}
}

// THE DELTA CARRIES THE COLUMN NAME AND THE MASKED VALUE, NEVER THE RAW NUMBER (rule 6,
// forbidden #4). An audit log holding raw personal data is a personal-data store that no erasure
// or masking rule reaches.
func TestThemCanBoVetCheDuLieuCaNhan(t *testing.T) {
	b := dungBanThuDanhBa(t)

	if _, err := b.uc.Them(ctxXa(xaThu), yeuCauThemGia(), nguoiThucHienGia()); err != nil {
		t.Fatalf("Them: %v", err)
	}
	vet := motVet(t, b.ghi)
	tho := string(vet.args[viTriDelta].([]byte))

	if strings.Contains(tho, diDongGia) {
		t.Errorf("vết mang SỐ DI ĐỘNG TRẦN: %s", tho)
	}
	if !strings.Contains(tho, "di_dong_ca_nhan") {
		t.Errorf("vết không nêu TÊN CỘT di_dong_ca_nhan: %s", tho)
	}
	if strings.Contains(tho, "Trần Thị B") {
		t.Errorf("vết mang HỌ TÊN ĐẦY ĐỦ: %s", tho)
	}
	sau, _ := deltaCua(t, vet)["sau"].(map[string]any)
	if sau["ho_ten"] == "" || sau["ho_ten"] == nil {
		t.Errorf("vết không có trường ho_ten đã che: %v", sau)
	}
}

// A CODE COLLISION IS RETRIED WITH A NEW CODE — never resolved by looking for a free one, which
// is two statements with a gap in between (rule 7, invariant 3; migration 0009 §4).
func TestThemCanBoMaTrungThiSinhMaKhacChuKhongTimMaTrong(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.maDaDung = true

	cb, err := b.uc.Them(ctxXa(xaThu), yeuCauThemGia(), nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Them sau va chạm mã: %v", err)
	}
	if b.kho.soLanChen != 2 {
		t.Fatalf("chèn %d lần, muốn 2 (lần đầu va chạm, lần sau mã mới)", b.kho.soLanChen)
	}
	if b.kho.maDaThu[0] == b.kho.maDaThu[1] {
		t.Fatalf("lần thử thứ hai dùng LẠI mã cũ %q", b.kho.maDaThu[1])
	}
	if cb.Ma != b.kho.maDaThu[1] {
		t.Fatalf("trả về mã %q, nhưng mã ghi được là %q", cb.Ma, b.kho.maDaThu[1])
	}
	// The failed attempt rolled back: exactly one entry, in the transaction that committed.
	vet := motVet(t, b.ghi)
	if ket := b.ghi.ketThucCua(vet.tx); ket != "commit" {
		t.Fatalf("giao dịch của vết kết thúc bằng %q", ket)
	}
	if b.ghi.soGiaoDich() != 2 {
		t.Fatalf("mở %d giao dịch, muốn 2", b.ghi.soGiaoDich())
	}
	if ket := b.ghi.ketThucCua(1); ket != "rollback" {
		t.Fatalf("giao dịch va chạm kết thúc bằng %q, muốn rollback", ket)
	}
}

// A REQUEST THAT FAILS ITS SHAPE OPENS NO TRANSACTION AT ALL. A rejected body must never hold a
// row lock while being rejected.
func TestThemCanBoSaiDinhDangThiKhongMoGiaoDich(t *testing.T) {
	b := dungBanThuDanhBa(t)

	yc := yeuCauThemGia()
	yc.HoTen = "   "
	if _, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia()); !errors.Is(err, domain.ErrThieuHoTen) {
		t.Fatalf("lỗi = %v, muốn ErrThieuHoTen", err)
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Fatalf("mở %d giao dịch cho một yêu cầu sai định dạng", n)
	}
}

// AN ACTOR WITH NO STAFF CODE REFUSES THE WRITE AND NEVER FALLS BACK TO THE INTERNAL ID.
func TestThemCanBoThieuMaNguoiThucHienThiTuChoi(t *testing.T) {
	b := dungBanThuDanhBa(t)

	nguoi := nguoiThucHienGia()
	nguoi.Vet.ID = ""
	if _, err := b.uc.Them(ctxXa(xaThu), yeuCauThemGia(), nguoi); err == nil {
		t.Fatal("thiếu mã cán bộ của người thực hiện mà vẫn ghi — luật 6 bất biến 2")
	}
	khongCoGhi(t, b.ghi)
}

// --- Sua ----------------------------------------------------------------------------------------

// A NO-OP WRITES NOTHING AND AUDITS NOTHING — which is what makes `idem.KhongCan` on the route a
// statement of fact rather than a hope.
func TestSuaCanBoKhongDoiGiThiKhongGhiVet(t *testing.T) {
	b := dungBanThuDanhBa(t)
	ten := b.kho.cb.HoTen

	if _, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac, YeuCauSuaCanBo{HoTen: &ten}, nguoiThucHienGia()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if n := len(vetDaGhi(b.ghi)); n != 0 {
		t.Errorf("gửi lại đúng giá trị đang có mà vẫn ghi %d vết", n)
	}
	if l := b.ghi.tim("SET ho-so"); l != nil {
		t.Error("gửi lại đúng giá trị đang có mà vẫn chạy UPDATE")
	}
}

// THE DELTA HOLDS ONLY THE FIELDS THAT MOVED, AND THE PERSONAL ONES ARE MASKED.
func TestSuaCanBoVetChiMangTruongDaDoiVaDaChe(t *testing.T) {
	b := dungBanThuDanhBa(t)
	moi := diDongGiaSau

	if _, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac,
		YeuCauSuaCanBo{DiDongCaNhan: &moi}, nguoiThucHienGia()); err != nil {
		t.Fatalf("Sua: %v", err)
	}

	vet := motVet(t, b.ghi)
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViSuaCanBo {
		t.Errorf("action = %q, muốn %q", got, HanhViSuaCanBo)
	}
	if got := chuoiArg(t, vet, viTriChuThe); got != maNguoiKhac {
		t.Errorf("subject = %q, muốn mã của NGƯỜI BỊ SỬA %q", got, maNguoiKhac)
	}

	tho := string(vet.args[viTriDelta].([]byte))
	if strings.Contains(tho, diDongGia) || strings.Contains(tho, diDongGiaSau) {
		t.Errorf("vết mang số di động TRẦN: %s", tho)
	}
	d := deltaCua(t, vet)
	truoc, _ := d["truoc"].(map[string]any)
	sau, _ := d["sau"].(map[string]any)
	if _, co := truoc["di_dong_ca_nhan"]; !co {
		t.Errorf("vết thiếu giá trị TRƯỚC của cột đã đổi: %v", truoc)
	}
	if _, co := sau["ho_ten"]; co {
		t.Errorf("vết mang cả trường KHÔNG đổi (ho_ten) — luật 6 bất biến 5: %v", sau)
	}
}

// --- DatKhoa: #14, first constraint --------------------------------------------------------------

// MUTATION THAT MUST TURN THIS RED: delete the `id == nguoi.ID` check in DatKhoa.
func TestKhoaChinhMinhBiTuChoi(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.ID = idNoiBo // the target IS the caller

	_, err := b.uc.DatKhoa(ctxXa(xaThu), idNoiBo, true, nguoiThucHienGia())
	if !errors.Is(err, ErrTuThaoTacChinhMinh) {
		t.Fatalf("lỗi = %v, muốn ErrTuThaoTacChinhMinh (câu #14)", err)
	}
	khongCoGhi(t, b.ghi)
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Errorf("mở %d giao dịch cho một thao tác bị chặn trước mọi lượt đọc", n)
	}
}

// --- DatKhoa: #13 --------------------------------------------------------------------------------

// MUTATION THAT MUST TURN THIS RED: delete the laNguoiQuanTriCuoiCung check in DatKhoa. Nothing
// else in the suite notices — the lock succeeds, the row is written, the trail is correct, and the
// commune finds out the next time somebody has to change a permission.
func TestKhoaNguoiQuanTriCUOICUNGBiTuChoi(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.quanTri = []string{idNguoiKhac} // the target is the only administrator left

	_, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia())
	if !errors.Is(err, ErrQuanTriCuoiCung) {
		t.Fatalf("lỗi = %v, muốn ErrQuanTriCuoiCung (câu #13)", err)
	}
	khongCoGhi(t, b.ghi)
	if ket := b.ghi.ketThucCua(1); ket != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback", ket)
	}
}

// THE OTHER DIRECTION, and it is not symmetry for its own sake: a rule written as
// `len(quanTri) <= 1` would refuse an ordinary lock in a commune that happens to have one
// administrator, which is how a correct rule gets deleted for being unusable.
func TestKhoaNguoiKHONGPhaiQuanTriVanChayDuDuyNhatMotQuanTri(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.quanTri = []string{idQuanTri2} // one administrator, and it is NOT the target

	cb, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("khoá một cán bộ thường bị chặn: %v", err)
	}
	if cb.DangHoatDong {
		t.Error("đã khoá mà dang_hoat_dong vẫn true")
	}
	vet := motVet(t, b.ghi)
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViKhoaCanBo {
		t.Errorf("action = %q, muốn %q", got, HanhViKhoaCanBo)
	}
}

// Locking somebody who is already locked writes nothing — the property `idem.KhongCan` claims.
func TestKhoaNguoiDaKhoaThiKhongGhiGi(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.DangHoatDong = false

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia()); err != nil {
		t.Fatalf("DatKhoa: %v", err)
	}
	if n := len(vetDaGhi(b.ghi)); n != 0 {
		t.Errorf("khoá một tài khoản đã khoá mà vẫn ghi %d vết", n)
	}
}

// UNLOCKING carries no #13 check — it can only make the set bigger — and writes its own verb.
func TestMoKhoaGhiHanhViRieng(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.DangHoatDong = false
	b.kho.quanTri = []string{idNguoiKhac}

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatalf("mở khoá bị chặn: %v", err)
	}
	vet := motVet(t, b.ghi)
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViMoKhoaCanBo {
		t.Errorf("action = %q, muốn %q", got, HanhViMoKhoaCanBo)
	}
}

// --- DoiVaiTro: #14, second constraint ------------------------------------------------------------

// MUTATION THAT MUST TURN THIS RED: delete the khongCam check in DoiVaiTro. Every request then
// succeeds, and `admin.user` silently becomes the key that contains every other one — through the
// two-step route the first constraint cannot see (promote a colleague, then be promoted by them).
func TestDoiVaiTroKhongTraoDuocQuyenMinhKhongCam(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.quyenToi = []string{"admin.user"} // the caller does NOT hold budget.confirm

	_, err := b.uc.DoiVaiTro(ctxXa(xaThu), idNguoiKhac, vaiTroManh, nguoiThucHienGia())
	if !errors.Is(err, ErrTraoQuyenKhongCam) {
		t.Fatalf("lỗi = %v, muốn ErrTraoQuyenKhongCam (câu #14)", err)
	}

	var chiTiet *LoiTraoQuyenKhongCam
	if !errors.As(err, &chiTiet) {
		t.Fatal("lỗi không mang danh sách khoá quyền thiếu — người quản trị không biết phải xin quyền nào")
	}
	if len(chiTiet.Thieu) != 1 || chiTiet.Thieu[0] != "budget.confirm" {
		t.Errorf("khoá thiếu = %v, muốn [budget.confirm]", chiTiet.Thieu)
	}
	khongCoGhi(t, b.ghi)
}

func TestDoiVaiTroCamDuQuyenThiChayDuoc(t *testing.T) {
	b := dungBanThuDanhBa(t)

	cb, err := b.uc.DoiVaiTro(ctxXa(xaThu), idNguoiKhac, vaiTroManh, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("DoiVaiTro: %v", err)
	}
	if cb.VaiTroID != vaiTroManh {
		t.Errorf("vai trò sau = %q, muốn %q", cb.VaiTroID, vaiTroManh)
	}

	vet := motVet(t, b.ghi)
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViDoiVaiTro {
		t.Errorf("action = %q, muốn %q", got, HanhViDoiVaiTro)
	}
	d := deltaCua(t, vet)
	truoc, _ := d["truoc"].(map[string]any)
	sau, _ := d["sau"].(map[string]any)
	if truoc["vai_tro_id"] != vaiTroYeu || sau["vai_tro_id"] != vaiTroManh {
		t.Errorf("vết không ghi đúng trước/sau: %v -> %v", truoc, sau)
	}
}

func TestDoiVaiTroChinhMinhBiTuChoi(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.ID = idNoiBo

	_, err := b.uc.DoiVaiTro(ctxXa(xaThu), idNoiBo, vaiTroManh, nguoiThucHienGia())
	if !errors.Is(err, ErrTuThaoTacChinhMinh) {
		t.Fatalf("lỗi = %v, muốn ErrTuThaoTacChinhMinh", err)
	}
	khongCoGhi(t, b.ghi)
}

// --- DoiVaiTro: #13, the third path ---------------------------------------------------------------

// MUTATION THAT MUST TURN THIS RED: delete the laNguoiQuanTriCuoiCung check in DoiVaiTro. This is
// the path #13 calls the easiest to miss: nobody is locked and nobody is deleted, so the screen
// shows an ordinary role change.
func TestHaVaiTroNguoiQuanTriCUOICUNGBiTuChoi(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.VaiTroID = vaiTroManh        // the target currently holds admin.user
	b.kho.quanTri = []string{idNguoiKhac} // and is the only one who does

	_, err := b.uc.DoiVaiTro(ctxXa(xaThu), idNguoiKhac, vaiTroYeu, nguoiThucHienGia())
	if !errors.Is(err, ErrQuanTriCuoiCung) {
		t.Fatalf("lỗi = %v, muốn ErrQuanTriCuoiCung (câu #13, đường thứ ba)", err)
	}
	khongCoGhi(t, b.ghi)
}

// MOVING THE LAST ADMINISTRATOR BETWEEN TWO ROLES THAT BOTH CARRY admin.user IS ALLOWED. The
// commune never stops having an administrator, and a rule that fired here would fire on a safe
// operation — which is how rules get removed.
func TestChuyenQuanTriCuoiCungSangVaiTroKhacVanCoAdminThiChayDuoc(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.VaiTroID = vaiTroManh
	b.kho.quanTri = []string{idNguoiKhac}
	b.kho.quyenVaiTro["vt-chu-tich-2"] = []string{"admin.user", "budget.confirm"}

	if _, err := b.uc.DoiVaiTro(ctxXa(xaThu), idNguoiKhac, "vt-chu-tich-2", nguoiThucHienGia()); err != nil {
		t.Fatalf("chuyển sang một vai trò CŨNG có admin.user bị chặn: %v", err)
	}
}

// A ROLE THAT IS ABSENT OR SOFT-DELETED IS REFUSED. The foreign key would accept the second one —
// rule 7 keeps the row — and the person would hold a role granting nothing while the screen shows
// them as having one.
func TestDoiSangVaiTroDaXoaBiTuChoi(t *testing.T) {
	b := dungBanThuDanhBa(t)

	_, err := b.uc.DoiVaiTro(ctxXa(xaThu), idNguoiKhac, "vt-da-xoa", nguoiThucHienGia())
	if !errors.Is(err, ErrVaiTroKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrVaiTroKhongTonTai", err)
	}
	khongCoGhi(t, b.ghi)
}

// Assigning the role somebody already holds writes nothing — the property `idem.KhongCan` claims.
func TestDoiVaiTroSangChinhVaiTroDangCoThiKhongGhiGi(t *testing.T) {
	b := dungBanThuDanhBa(t)

	if _, err := b.uc.DoiVaiTro(ctxXa(xaThu), idNguoiKhac, vaiTroYeu, nguoiThucHienGia()); err != nil {
		t.Fatalf("DoiVaiTro: %v", err)
	}
	if n := len(vetDaGhi(b.ghi)); n != 0 {
		t.Errorf("gán đúng vai trò đang có mà vẫn ghi %d vết", n)
	}
}

// --- what every write shares ---------------------------------------------------------------------

// EVERY audit entry of this file names the commune of the transaction it ran in. The commune is
// never a parameter — audit.Write takes it from the transaction, which took it from the context
// (rule 1, invariant 4) — and this is the case that would notice a second source appearing.
func TestMoiVetMangDungXaCuaGiaoDich(t *testing.T) {
	b := dungBanThuDanhBa(t)

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia()); err != nil {
		t.Fatalf("DatKhoa: %v", err)
	}
	vet := motVet(t, b.ghi)
	if got := chuoiArg(t, vet, 0); got != string(xaThu) {
		t.Errorf("tenant_id của vết = %q, muốn %q", got, string(xaThu))
	}
	if got := chuoiArg(t, vet, 3); got != ipGia {
		t.Errorf("actor_ip = %q, muốn %q — luật 6 bất biến 2 đòi địa chỉ thật của yêu cầu", got, ipGia)
	}
}

// A store failure rolls the whole thing back: no row, no entry. The two states always agree.
func TestLoiKhoThiKhongConVetNaoSotLai(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.loi = errors.New("kho hỏng")

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia()); err == nil {
		t.Fatal("kho hỏng mà DatKhoa vẫn trả về thành công")
	}
	khongCoGhi(t, b.ghi)
	if ket := b.ghi.ketThucCua(1); ket != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback", ket)
	}
}

var _ driver.Value = driver.Value(nil) // giữ import driver cho chuoiArg/deltaCua
