package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: the credential flow of open questions #9 and #17. Every property it
// asserts fails SILENTLY when it is broken, which is the only reason any of them is worth a test.
//
//	THE TEMPORARY PASSWORD IN THE TRAIL   an audit entry carrying the plaintext works perfectly.
//	                                      Every screen is right, every status code is right, and a
//	                                      table nobody may delete, kept for years, now holds working
//	                                      credentials for a government system (rule 6, forbidden #4).
//	THE FLAG NOT SET                      an account minted with `phai_doi_mat_khau = false` signs in
//	                                      and works. The only symptom is that a second person knows
//	                                      that member of staff's password — for ever — and the audit
//	                                      trail states their name with complete confidence.
//	THE FLAG CLEARED BY THE WRONG PATH    if anything but the person's own change clears it, #9 is
//	                                      defeated while every test about "the flag exists" stays
//	                                      green.
//	#14 ON THE RESET ROUTE                without it, an unattended browser on the SHARED counter
//	                                      machine of #18 is a complete account takeover, and the
//	                                      trail records it as ordinary administration.
//
// It runs on the fake database/sql driver from dang_nhap_giao_dich_test.go, which records WHICH
// statement ran inside WHICH transaction and how that transaction ended — exactly what rule 6,
// invariant 3 turns on. The real SQL is covered in tai_khoan_can_bo_pg_test.go behind
// VIGOV_TEST_DSN.

// --- the fake store ---------------------------------------------------------------------------

// khoTaiKhoanGia stands in for *idstore.CanBoStore on the credential paths.
//
// IT RECORDS THE HASH IT WAS GIVEN, NOT THE PASSWORD, because that is all the real store ever sees.
// A fake that accepted a plaintext would quietly make the test suite prove the opposite of the
// property: that the value CAN travel this far.
type khoTaiKhoanGia struct {
	cb     domain.CanBoTomTat
	day    domain.CanBo // what TheoIDDeGhiMatKhau answers — carries the hash
	coDong bool

	bamDaCap  string
	bamDatLai string
	bamTuDoi  string

	loi error
}

func (k *khoTaiKhoanGia) TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CanBoTomTat, error) {
	if k.loi != nil {
		return domain.CanBoTomTat{}, k.loi
	}
	if !k.coDong || id != k.cb.ID {
		return domain.CanBoTomTat{}, idstore.ErrCanBoKhongTonTai
	}
	// A real statement, so the driver can show the read happened INSIDE the transaction.
	if _, err := tx.Exec(ctx, "SELECT doc-de-ghi FROM nguoi_dung", string(tx.TenantID()), id); err != nil {
		return domain.CanBoTomTat{}, err
	}
	return k.cb, nil
}

func (k *khoTaiKhoanGia) TheoIDDeGhiMatKhau(ctx context.Context, tx *store.ScopedTx, id string) (domain.CanBo, error) {
	if k.loi != nil {
		return domain.CanBo{}, k.loi
	}
	if !k.coDong || id != k.day.ID {
		return domain.CanBo{}, idstore.ErrCanBoKhongTonTai
	}
	if _, err := tx.Exec(ctx, "SELECT doc-de-ghi-mat-khau FROM nguoi_dung", string(tx.TenantID()), id); err != nil {
		return domain.CanBo{}, err
	}
	return k.day, nil
}

func (k *khoTaiKhoanGia) CapTaiKhoan(ctx context.Context, tx *store.ScopedTx, id, bam string) error {
	k.bamDaCap = bam
	_, err := tx.Exec(ctx, "UPDATE nguoi_dung (dau-hieu-cap-tai-khoan)", id, bam)
	return err
}

func (k *khoTaiKhoanGia) DatMatKhauTam(ctx context.Context, tx *store.ScopedTx, id, bam string) error {
	k.bamDatLai = bam
	_, err := tx.Exec(ctx, "UPDATE nguoi_dung (dau-hieu-dat-lai)", id, bam)
	return err
}

func (k *khoTaiKhoanGia) DoiMatKhauChinhMinh(ctx context.Context, tx *store.ScopedTx, id, bam string) error {
	k.bamTuDoi = bam
	_, err := tx.Exec(ctx, "UPDATE nguoi_dung (dau-hieu-tu-doi)", id, bam)
	return err
}

// phienGiaThuHoi records the session revocations, and runs a real statement so the test can assert
// the revocation shared the transaction with the credential write.
type phienGiaThuHoi struct {
	lyDo []string
	loi  error
}

func (p *phienGiaThuHoi) ThuHoiCuaCanBo(ctx context.Context, tx *store.ScopedTx, id, lyDo string) error {
	if p.loi != nil {
		return p.loi
	}
	p.lyDo = append(p.lyDo, lyDo)
	_, err := tx.Exec(ctx, "UPDATE phien (dau-hieu-thu-hoi)", id, lyDo)
	return err
}

// --- the bench --------------------------------------------------------------------------------

// banThuTaiKhoan builds the use case over the fake driver with the two argon2 functions replaced.
//
// THE HASH IS A SHA-256 OF THE VALUE, NOT A REAL argon2 ENCODING, and both halves of that are
// deliberate. It makes the suite fast — the real Bam costs 19 MiB and tens of milliseconds per call
// — and, crucially, IT DOES NOT CONTAIN THE PLAINTEXT. The central assertion here is that the
// password appears in NOTHING sent to the database; a stand-in hash built by concatenation
// (`"bam-gia:" + matKhau`) would make that assertion fire on the hash itself, and the obvious
// repair is to weaken the assertion. It is also still DERIVABLE, so a test can prove the hash of
// the returned value really did reach the store — which is what stops the first assertion passing
// vacuously.
//
// THE GENERATOR IS NOT PINNED TO A CONSTANT. Two calls must return two different values — that is
// what TestDatLaiSinhMatKhauKhacNhauMoiLan asserts — so it counts instead.
type banThuTaiKhoan struct {
	uc    *TaiKhoanCanBo
	kho   *khoTaiKhoanGia
	phien *phienGiaThuHoi
	ghi   *ghiChep
}

const matKhauCuGia = "mat-khau-cu-KHONG-PHAI-THAT"

func bamGiaTu(matKhau string) string {
	tong := sha256.Sum256([]byte("bam-gia:" + matKhau))
	return "$gia$" + hex.EncodeToString(tong[:])
}

func dungBanThuTaiKhoan(t *testing.T) *banThuTaiKhoan {
	t.Helper()
	db, g := moDB(t)

	cb := canBoGia()
	cb.CoTaiKhoan = false // a DIRECTORY ROW: the state the account route starts from
	kho := &khoTaiKhoanGia{
		cb:     cb,
		coDong: true,
		day: domain.CanBo{
			ID: idNoiBo, Ma: maCanBo, HoTen: "Nguyễn Văn A", Email: emailCB,
			MatKhauHash: bamGiaTu(matKhauCuGia),
			// The person is under the forced change — the ordinary state of somebody calling the
			// self route for the first time (#9).
			PhaiDoiMatKhau: true,
			CoTaiKhoan:     true, DangHoatDong: true,
		},
	}
	phien := &phienGiaThuHoi{}

	uc := NewTaiKhoanCanBo(db, kho, phien)
	lan := 0
	uc.sinhMatKhau = func() (string, error) {
		lan++
		return "MAT-KHAU-TAM-GIA-" + string(rune('0'+lan)), nil
	}
	uc.bam = func(m string) (string, error) { return bamGiaTu(m), nil }
	uc.kiemTraBam = func(matKhau, bam string) error {
		if bamGiaTu(matKhau) != bam {
			return password.ErrSaiMatKhau
		}
		return nil
	}

	return &banThuTaiKhoan{uc: uc, kho: kho, phien: phien, ghi: g}
}

// moiThamSoChuoi returns every string argument of every statement the driver saw.
//
// IT IS THE ASSERTION BEHIND "THE PASSWORD NEVER LEAVES", and it looks at ARGUMENTS rather than at
// the audit entry alone on purpose: a plaintext reaching `nguoi_dung` would be just as bad as one
// reaching `audit_log`, and a test that only read the trail would not see it.
func moiThamSoChuoi(g *ghiChep) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var ra []string
	for _, l := range g.lenh {
		ra = append(ra, l.sql)
		for _, a := range l.args {
			switch v := a.(type) {
			case string:
				ra = append(ra, v)
			case []byte:
				ra = append(ra, string(v))
			}
		}
	}
	return ra
}

// khongLoRa asserts the value appears in NOTHING the database was handed.
func khongLoRa(t *testing.T, g *ghiChep, biMat, ten string) {
	t.Helper()
	for _, s := range moiThamSoChuoi(g) {
		if strings.Contains(s, biMat) {
			// The value itself is NOT printed, even in a failure. A test output is a log line
			// somewhere (rule 3, rule 8); naming the field is enough to act on.
			t.Fatalf("%s ĐÃ LỌT vào một câu lệnh gửi xuống CSDL — chuỗi có độ dài %d chứa nó. "+
				"Mật khẩu tạm chỉ được sống trong giá trị trả về của use case (#9)", ten, len(s))
		}
	}
}

// --- Cap: issuing an account -------------------------------------------------------------------

// The credential write and its audit entry share ONE transaction, and that transaction committed
// (rule 6, invariant 3).
//
// MUTATION THAT MUST TURN THIS RED: move the audit.Write call out of the Tx closure, or drop it.
func TestCapTaiKhoanGhiVetTrongCUNGGiaoDich(t *testing.T) {
	b := dungBanThuTaiKhoan(t)

	kq, err := b.uc.Cap(ctxXa(xaThu), idNguoiKhac, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}
	if kq.MatKhauTam == "" {
		t.Fatal("không trả về mật khẩu tạm — quản trị viên không có gì để đọc cho người ta")
	}

	ghi := b.ghi.tim("dau-hieu-cap-tai-khoan")
	if ghi == nil {
		t.Fatal("không có câu lệnh cấp tài khoản nào chạy")
	}
	vet := motVet(t, b.ghi)
	if vet.tx == 0 || vet.tx != ghi.tx {
		t.Fatalf("vết ở giao dịch %d, câu ghi ở giao dịch %d — phải cùng một (luật 6 bất biến 3)",
			vet.tx, ghi.tx)
	}
	if b.ghi.ketThucCua(vet.tx) != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", b.ghi.ketThucCua(vet.tx))
	}
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViCapTaiKhoan {
		t.Fatalf("hành vi = %q, muốn %q", got, HanhViCapTaiKhoan)
	}
	// Rule 6, invariant 8: the trail names the BUSINESS CODE of the person acted on.
	if got := chuoiArg(t, vet, viTriChuThe); got != maNguoiKhac {
		t.Fatalf("chủ thể vết = %q, muốn mã cán bộ %q", got, maNguoiKhac)
	}
}

// THE CENTRAL CASE OF THIS TURN: the temporary password reaches the administrator and reaches
// NOTHING ELSE — not the audit entry, not the business statement, not any argument sent to the
// database. The hash derived from it did, which is what proves the assertion is looking in the
// right place.
//
// MUTATIONS THAT MUST TURN THIS RED: put `matKhau` into the audit delta; pass the plaintext to
// CapTaiKhoan instead of the hash; add the value to any log or statement argument.
func TestCapTaiKhoanKhongDeMatKhauTamVaoVetHayVaoCauLenh(t *testing.T) {
	b := dungBanThuTaiKhoan(t)

	kq, err := b.uc.Cap(ctxXa(xaThu), idNguoiKhac, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}

	khongLoRa(t, b.ghi, kq.MatKhauTam, "mật khẩu tạm")

	// ...and the HASH of it did reach the store, so the check above is not passing vacuously.
	if b.kho.bamDaCap != bamGiaTu(kq.MatKhauTam) {
		t.Fatalf("kho nhận chuỗi băm không khớp mật khẩu đã trả về — phép kiểm ở trên không chứng minh gì")
	}
	// The hash itself must not be in the trail either (rule 6, forbidden #4): an append-only table
	// holding the argon2 encoding of every credential ever issued is an offline-cracking target
	// that outlives every rotation.
	vet := motVet(t, b.ghi)
	delta, _ := json.Marshal(deltaCua(t, vet))
	if strings.Contains(string(delta), b.kho.bamDaCap) {
		t.Fatal("chuỗi băm lọt vào delta của vết kiểm toán — vết chỉ được ghi TÊN CỘT")
	}
	if !strings.Contains(string(delta), "mat_khau_hash") {
		t.Fatal("delta không nêu TÊN CỘT mat_khau_hash — không đọc ra được rằng thông tin đăng nhập đã đổi")
	}
}

// The account is created AND the flag is set, in the delta the ledger keeps.
//
// MUTATION THAT MUST TURN THIS RED: drop `phai_doi_mat_khau = true` from capTaiKhoanCanBo, or stop
// recording it — the account would then be minted with a password valid for ever (#9 defeated).
func TestCapTaiKhoanVetNoiRoBatDoiMatKhau(t *testing.T) {
	b := dungBanThuTaiKhoan(t)

	if _, err := b.uc.Cap(ctxXa(xaThu), idNguoiKhac, nguoiThucHienGia()); err != nil {
		t.Fatalf("Cap lỗi: %v", err)
	}
	delta := deltaCua(t, motVet(t, b.ghi))
	sau, ok := delta["sau"].(map[string]any)
	if !ok {
		t.Fatalf("delta không có nhánh `sau`: %v", delta)
	}
	if sau["co_tai_khoan"] != true {
		t.Fatalf("delta.sau.co_tai_khoan = %v, muốn true", sau["co_tai_khoan"])
	}
	if sau["phai_doi_mat_khau"] != true {
		t.Fatalf("delta.sau.phai_doi_mat_khau = %v, muốn true — #9 bắt đổi ở lần đăng nhập đầu",
			sau["phai_doi_mat_khau"])
	}
}

func TestCapTaiKhoanChoNguoiDaCoTaiKhoanBiTuChoi(t *testing.T) {
	b := dungBanThuTaiKhoan(t)
	b.kho.cb.CoTaiKhoan = true

	if _, err := b.uc.Cap(ctxXa(xaThu), idNguoiKhac, nguoiThucHienGia()); !errors.Is(err, ErrDaCoTaiKhoan) {
		t.Fatalf("lỗi = %v, muốn ErrDaCoTaiKhoan", err)
	}
	khongCoGhi(t, b.ghi)
}

// Issuing a credential to a locked account would produce one that cannot sign in, and the
// administrator would read the password down the telephone before finding out.
func TestCapTaiKhoanChoNguoiDangKhoaBiTuChoi(t *testing.T) {
	b := dungBanThuTaiKhoan(t)
	b.kho.cb.DangHoatDong = false

	if _, err := b.uc.Cap(ctxXa(xaThu), idNguoiKhac, nguoiThucHienGia()); !errors.Is(err, ErrTaiKhoanDangKhoa) {
		t.Fatalf("lỗi = %v, muốn ErrTaiKhoanDangKhoa", err)
	}
	khongCoGhi(t, b.ghi)
}

// --- DatLai: the administrator resetting somebody else's password (#17) -------------------------

// #14 ON THE ROUTE WHERE IT STOPS SOMETHING REAL. This route does not ask for the current password,
// so a self-reset from an unattended browser on #18's SHARED counter machine would be a complete
// account takeover with no credential needed.
//
// MUTATION THAT MUST TURN THIS RED: remove the `id == nguoi.ID` check from mint.
func TestDatLaiMatKhauLenChinhMinhBiTuChoi(t *testing.T) {
	b := dungBanThuTaiKhoan(t)

	// The actor IS the target: nguoiThucHienGia() carries ID = idNoiBo.
	if _, err := b.uc.DatLai(ctxXa(xaThu), idNoiBo, nguoiThucHienGia()); !errors.Is(err, ErrTuThaoTacChinhMinh) {
		t.Fatalf("lỗi = %v, muốn ErrTuThaoTacChinhMinh (#14)", err)
	}
	khongCoGhi(t, b.ghi)
	if b.kho.bamDatLai != "" {
		t.Fatal("bị từ chối nhưng vẫn đặt chuỗi băm mới")
	}
}

func TestCapTaiKhoanLenChinhMinhBiTuChoi(t *testing.T) {
	b := dungBanThuTaiKhoan(t)

	if _, err := b.uc.Cap(ctxXa(xaThu), idNoiBo, nguoiThucHienGia()); !errors.Is(err, ErrTuThaoTacChinhMinh) {
		t.Fatalf("lỗi = %v, muốn ErrTuThaoTacChinhMinh (#14)", err)
	}
	khongCoGhi(t, b.ghi)
}

func TestDatLaiChoNguoiChuaCoTaiKhoanBiTuChoi(t *testing.T) {
	b := dungBanThuTaiKhoan(t) // the fixture is a directory row: CoTaiKhoan = false

	if _, err := b.uc.DatLai(ctxXa(xaThu), idNguoiKhac, nguoiThucHienGia()); !errors.Is(err, ErrChuaCoTaiKhoan) {
		t.Fatalf("lỗi = %v, muốn ErrChuaCoTaiKhoan", err)
	}
	khongCoGhi(t, b.ghi)
}

// Two resets must produce two DIFFERENT passwords. A generator that returned a constant — or one
// seeded from the row — would hand every member of staff the same temporary value.
func TestDatLaiSinhMatKhauKhacNhauMoiLan(t *testing.T) {
	b := dungBanThuTaiKhoan(t)
	b.kho.cb.CoTaiKhoan = true

	mot, err := b.uc.DatLai(ctxXa(xaThu), idNguoiKhac, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("DatLai lần 1: %v", err)
	}
	hai, err := b.uc.DatLai(ctxXa(xaThu), idNguoiKhac, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("DatLai lần 2: %v", err)
	}
	if mot.MatKhauTam == hai.MatKhauTam {
		t.Fatal("hai lần đặt lại cho CÙNG một mật khẩu tạm")
	}
	if b.kho.bamDatLai != bamGiaTu(hai.MatKhauTam) {
		t.Fatal("lần đặt lại thứ hai không ghi đè chuỗi băm")
	}
}

// REQUIRED #7 of skills/session-and-token, in the SAME transaction as the credential write: a reset
// that leaves the old sessions open has taken nothing back.
//
// MUTATION THAT MUST TURN THIS RED: drop the ThuHoiCuaCanBo call, or move it outside the Tx closure.
func TestDatLaiThuHoiMoiPhienTrongCungGiaoDich(t *testing.T) {
	b := dungBanThuTaiKhoan(t)
	b.kho.cb.CoTaiKhoan = true

	if _, err := b.uc.DatLai(ctxXa(xaThu), idNguoiKhac, nguoiThucHienGia()); err != nil {
		t.Fatalf("DatLai lỗi: %v", err)
	}
	if len(b.phien.lyDo) != 1 {
		t.Fatalf("thu hồi %d lần, muốn đúng 1", len(b.phien.lyDo))
	}
	thu := b.ghi.tim("dau-hieu-thu-hoi")
	vet := motVet(t, b.ghi)
	if thu == nil || thu.tx == 0 || thu.tx != vet.tx {
		t.Fatalf("thu hồi phiên không nằm trong giao dịch của vết: %+v vs tx %d", thu, vet.tx)
	}
}

// --- DoiCuaChinhMinh: the person taking ownership (#9) ------------------------------------------

// THE ONE PATH THAT CLEARS THE FLAG, and the delta says so.
//
// MUTATION THAT MUST TURN THIS RED: write `phai_doi_mat_khau = true` (or leave the column alone) in
// doiMatKhauChinhMinh — the person would change their password and stay locked to one screen for
// ever.
func TestDoiMatKhauChinhMinhGoCoBatDoi(t *testing.T) {
	b := dungBanThuTaiKhoan(t)

	err := b.uc.DoiCuaChinhMinh(ctxXa(xaThu),
		YeuCauDoiMatKhau{HienTai: matKhauCuGia, Moi: "mat-khau-moi-KHONG-PHAI-THAT"},
		nguoiThucHienGia())
	if err != nil {
		t.Fatalf("DoiCuaChinhMinh lỗi: %v", err)
	}

	delta := deltaCua(t, motVet(t, b.ghi))
	truoc := delta["truoc"].(map[string]any)
	sau := delta["sau"].(map[string]any)
	if truoc["phai_doi_mat_khau"] != true || sau["phai_doi_mat_khau"] != false {
		t.Fatalf("delta bắt đổi: truoc=%v sau=%v, muốn true -> false", truoc, sau)
	}
	if b.kho.bamTuDoi == "" {
		t.Fatal("không ghi chuỗi băm mới")
	}
	if len(b.phien.lyDo) != 1 {
		t.Fatalf("thu hồi %d lần, muốn đúng 1 — đổi mật khẩu phải kết thúc mọi phiên", len(b.phien.lyDo))
	}
}

// The new password must not reach the database as plaintext either — the self route is the one
// place a person's OWN chosen password passes through this process.
func TestDoiMatKhauKhongDeMatKhauMoiVaoCauLenh(t *testing.T) {
	b := dungBanThuTaiKhoan(t)
	const moi = "mat-khau-moi-KHONG-PHAI-THAT"

	if err := b.uc.DoiCuaChinhMinh(ctxXa(xaThu),
		YeuCauDoiMatKhau{HienTai: matKhauCuGia, Moi: moi}, nguoiThucHienGia()); err != nil {
		t.Fatalf("DoiCuaChinhMinh lỗi: %v", err)
	}
	khongLoRa(t, b.ghi, moi, "mật khẩu mới")
	khongLoRa(t, b.ghi, matKhauCuGia, "mật khẩu hiện tại")
}

// A wrong current password writes NOTHING — and answers the same whatever the reason.
func TestDoiMatKhauSaiMatKhauHienTaiThiKhongGhiGi(t *testing.T) {
	b := dungBanThuTaiKhoan(t)

	err := b.uc.DoiCuaChinhMinh(ctxXa(xaThu),
		YeuCauDoiMatKhau{HienTai: "sai-mat-khau-KHONG-PHAI-THAT", Moi: "mat-khau-moi-KHONG-PHAI-THAT"},
		nguoiThucHienGia())
	if !errors.Is(err, ErrMatKhauHienTaiSai) {
		t.Fatalf("lỗi = %v, muốn ErrMatKhauHienTaiSai", err)
	}
	khongCoGhi(t, b.ghi)
	if b.kho.bamTuDoi != "" {
		t.Fatal("mật khẩu sai nhưng vẫn ghi chuỗi băm mới")
	}
	if len(b.phien.lyDo) != 0 {
		t.Fatal("mật khẩu sai nhưng vẫn thu hồi phiên")
	}
}

// "Changing" to the same value would clear the flag while the administrator's password stayed
// valid — #9 defeated in one request, with a trail that says the person changed their password.
func TestDoiMatKhauMoiTrungCuBiTuChoi(t *testing.T) {
	b := dungBanThuTaiKhoan(t)

	err := b.uc.DoiCuaChinhMinh(ctxXa(xaThu),
		YeuCauDoiMatKhau{HienTai: matKhauCuGia, Moi: matKhauCuGia}, nguoiThucHienGia())
	if !errors.Is(err, domain.ErrMatKhauMoiTrungCu) {
		t.Fatalf("lỗi = %v, muốn domain.ErrMatKhauMoiTrungCu", err)
	}
	khongCoGhi(t, b.ghi)
	if b.ghi.soGiaoDich() != 0 {
		t.Fatalf("mở %d giao dịch cho một yêu cầu bị từ chối vì hình dạng", b.ghi.soGiaoDich())
	}
}

// A password that fails its shape is refused BEFORE a transaction opens — it must never hold a row
// lock while being told it is too short.
func TestDoiMatKhauQuaNganKhongMoGiaoDich(t *testing.T) {
	b := dungBanThuTaiKhoan(t)

	err := b.uc.DoiCuaChinhMinh(ctxXa(xaThu),
		YeuCauDoiMatKhau{HienTai: matKhauCuGia, Moi: "ngan"}, nguoiThucHienGia())
	if !errors.Is(err, domain.ErrMatKhauQuaNgan) {
		t.Fatalf("lỗi = %v, muốn domain.ErrMatKhauQuaNgan", err)
	}
	if b.ghi.soGiaoDich() != 0 {
		t.Fatalf("mở %d giao dịch, muốn 0", b.ghi.soGiaoDich())
	}
}

// THE TWO LENGTH CONSTANTS ARE TWO LITERALS IN TWO PACKAGES, and they are not allowed to drift.
// `domain` may import nothing but the standard library (rule 4 of the service pattern), so the
// compiler cannot hold them equal — this test is what does.
//
// THE DIRECTION THAT HURTS: if domain's were the smaller, the use case would accept a password that
// core/password.Bam then refuses, INSIDE the transaction, reaching a member of staff as an internal
// error on a screen that had just told them their input was fine.
func TestDaiMatKhauKhopVoiCorePassword(t *testing.T) {
	if domain.DaiMatKhauToiThieu != password.DaiToiThieu {
		t.Fatalf("domain.DaiMatKhauToiThieu = %d nhưng password.DaiToiThieu = %d — hai hằng này PHẢI bằng nhau",
			domain.DaiMatKhauToiThieu, password.DaiToiThieu)
	}
}

// The minted password must satisfy the rule the self route applies, or the person is handed a value
// the system would refuse if they typed it themselves.
func TestMatKhauTamThatDatYeuCauDoDai(t *testing.T) {
	m, err := domain.SinhMatKhauTam()
	if err != nil {
		t.Fatalf("SinhMatKhauTam lỗi: %v", err)
	}
	if err := domain.KiemTraMatKhauMoi(m); err != nil {
		t.Fatalf("mật khẩu tạm không qua được chính phép kiểm của hệ thống: %v", err)
	}
	if _, err := password.Bam(m); err != nil {
		t.Fatalf("mật khẩu tạm không băm được: %v", err)
	}
}
