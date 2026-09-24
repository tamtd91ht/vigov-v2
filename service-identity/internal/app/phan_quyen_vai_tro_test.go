package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: PUT /api/v1/roles/{id}/permissions stands on #13 and #14, and every one of
// its refusals FAILS SILENTLY when removed — the save succeeds, the matrix looks right, and the
// audit trail records an escalation or a lock-out as an ordinary save.
//
// It runs on the fake database/sql driver of dang_nhap_giao_dich_test.go, so "the grant change and
// its audit entry share one transaction" is an assertion about what actually ran.
//
// WHAT IT CANNOT PROVE: that the `FOR UPDATE` locks serialise two concurrent saves. No PostgreSQL
// harness runs here (the *_pg_test.go suites SKIP without VIGOV_TEST_DSN), and a fake store agrees
// with whatever it is told.
//
// A NOTE ON THE 409 TESTS, because a reader will ask: in a CONSISTENT database, #13 cannot fire on
// this route while #14 holds. Removing key K needs the actor to hold K (#14 second), the actor's role
// cannot be the target (#14 first), and the actor is active — so the actor IS a holder of K outside
// the target role. The 409 fixtures therefore hand the use case an actor key list that disagrees
// with the locked holder sets on purpose: that is the only way to test the #13 guard ALONE, and it
// is the guard that stands if #14's removal half is ever relaxed.

const (
	vaiTroQuanTri = "vt-quan-tri"
	vaiTroDich    = "vt-chuyen-vien"
)

// khoPhanQuyenGia stands in for *idstore.PhanQuyenStore. The two writes run a REAL statement
// through the transaction they were handed, so the driver records their transaction ids.
type khoPhanQuyenGia struct {
	quanTri   []domain.NguoiGiuQuyen
	phanQuyen []domain.NguoiGiuQuyen
	// vaiTro maps a role to its keys; ABSENT = no such role in this commune (or soft-deleted).
	vaiTro    map[string][]string
	vaiTroToi string
	quyenToi  []string
	// danhMuc is the platform catalogue `quyen`.
	danhMuc map[string]bool

	themCuoi, boCuoi []string
	capBoiCuoi       string
}

func (k *khoPhanQuyenGia) QuanTriDeGhi(context.Context, *store.ScopedTx) ([]domain.NguoiGiuQuyen, error) {
	return k.quanTri, nil
}
func (k *khoPhanQuyenGia) PhanQuyenDeGhi(context.Context, *store.ScopedTx) ([]domain.NguoiGiuQuyen, error) {
	return k.phanQuyen, nil
}
func (k *khoPhanQuyenGia) VaiTroDeGhi(_ context.Context, _ *store.ScopedTx, id string) ([]string, error) {
	ds, co := k.vaiTro[id]
	if !co {
		return nil, idstore.ErrVaiTroKhongTonTaiDeGhi
	}
	return append([]string{}, ds...), nil
}
func (k *khoPhanQuyenGia) VaiTroCuaNguoi(context.Context, *store.ScopedTx, string) (string, error) {
	return k.vaiTroToi, nil
}
func (k *khoPhanQuyenGia) QuyenDangGiu(context.Context, *store.ScopedTx, string) ([]string, error) {
	return k.quyenToi, nil
}
func (k *khoPhanQuyenGia) KhoaKhongTonTai(_ context.Context, _ *store.ScopedTx, ds []string) ([]string, error) {
	var thieu []string
	for _, v := range ds {
		if !k.danhMuc[v] {
			thieu = append(thieu, v)
		}
	}
	return thieu, nil
}
func (k *khoPhanQuyenGia) ThemCap(ctx context.Context, tx *store.ScopedTx, id string, them []string, capBoi string) error {
	if len(them) == 0 {
		return nil
	}
	k.themCuoi, k.capBoiCuoi = them, capBoi
	_, err := tx.Exec(ctx, "GHI-GIA them-cap vai_tro_quyen", string(tx.TenantID()), id, strings.Join(them, ","), capBoi)
	return err
}
func (k *khoPhanQuyenGia) BoCap(ctx context.Context, tx *store.ScopedTx, id string, bo []string) error {
	if len(bo) == 0 {
		return nil
	}
	k.boCuoi = bo
	_, err := tx.Exec(ctx, "GHI-GIA bo-cap vai_tro_quyen", string(tx.TenantID()), id, strings.Join(bo, ","))
	return err
}

type banThuPhanQuyen struct {
	uc  *PhanQuyenVaiTro
	kho *khoPhanQuyenGia
	ghi *ghiChep
}

// dungBanThuPhanQuyen: the actor (idNoiBo) sits in vt-quan-tri with admin.user, admin.role,
// document.read, task.read. A second administrator exists in the same role, so #13 permits by
// default; the target column vt-chuyen-vien holds document.read.
func dungBanThuPhanQuyen(t *testing.T) *banThuPhanQuyen {
	t.Helper()
	db, g := moDB(t)
	haiQuanTri := []domain.NguoiGiuQuyen{
		{CanBoID: idNoiBo, VaiTroID: vaiTroQuanTri},
		{CanBoID: idQuanTri2, VaiTroID: vaiTroQuanTri},
	}
	kho := &khoPhanQuyenGia{
		quanTri:   haiQuanTri,
		phanQuyen: haiQuanTri,
		vaiTro: map[string][]string{
			vaiTroQuanTri: {"admin.role", "admin.user", "document.read", "task.read"},
			vaiTroDich:    {"document.read"},
		},
		vaiTroToi: vaiTroQuanTri,
		quyenToi:  []string{"admin.role", "admin.user", "document.read", "task.read"},
		danhMuc: map[string]bool{
			"admin.role": true, "admin.user": true, "document.read": true, "task.read": true,
			"budget.confirm": true,
		},
	}
	return &banThuPhanQuyen{uc: NewPhanQuyenVaiTro(db, kho), kho: kho, ghi: g}
}

// khongGhiPhanQuyen: a refusal writes no grant and no audit entry.
func khongGhiPhanQuyen(t *testing.T, g *ghiChep) {
	t.Helper()
	if n := len(vetDaGhi(g)); n != 0 {
		t.Errorf("bị từ chối nhưng vẫn ghi %d vết kiểm toán", n)
	}
	if g.tim("GHI-GIA") != nil {
		t.Error("bị từ chối nhưng vẫn ghi vai_tro_quyen")
	}
}

// --- the ordinary save ----------------------------------------------------------------------------

// The grant change and its audit entry share ONE transaction, which committed (rule 6, inv 3).
//
// MUTATION THAT MUST TURN THIS RED: move audit.Write out of the Tx closure, or drop it.
func TestLuuPhanQuyenGhiVetCungGiaoDich(t *testing.T) {
	b := dungBanThuPhanQuyen(t)

	sau, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"task.read"}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Luu: %v", err)
	}
	if strings.Join(sau, ",") != "task.read" {
		t.Errorf("tập đã lưu = %v", sau)
	}
	if strings.Join(b.kho.themCuoi, ",") != "task.read" || strings.Join(b.kho.boCuoi, ",") != "document.read" {
		t.Errorf("thêm %v bỏ %v, muốn thêm [task.read] bỏ [document.read]", b.kho.themCuoi, b.kho.boCuoi)
	}

	them, bo := b.ghi.tim("them-cap"), b.ghi.tim("bo-cap")
	vet := motVet(t, b.ghi)
	if them == nil || bo == nil || vet.tx == 0 || vet.tx != them.tx || vet.tx != bo.tx {
		t.Fatalf("vết và hai câu ghi không cùng một giao dịch (luật 6 bất biến 3)")
	}
	if ket := b.ghi.ketThucCua(vet.tx); ket != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", ket)
	}
	// The commune of the entry is the context's, bound at position 0 by audit.Write.
	if got := chuoiArg(t, vet, 0); got != string(xaThu) {
		t.Errorf("tenant của vết = %q, muốn %q", got, xaThu)
	}
}

// The actor is the STAFF CODE, the subject the role id, the delta names truoc/sau/them/bo.
//
// MUTATION THAT MUST TURN THIS RED: write nguoi.ID into the actor (or into cap_boi).
func TestLuuPhanQuyenVetMangMaCanBoVaTapTruocSau(t *testing.T) {
	b := dungBanThuPhanQuyen(t)

	if _, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"task.read"}, nguoiThucHienGia()); err != nil {
		t.Fatalf("Luu: %v", err)
	}
	vet := motVet(t, b.ghi)
	if got := chuoiArg(t, vet, viTriActor); got != maCanBo {
		t.Errorf("actor_id = %q, muốn MÃ CÁN BỘ %q (luật 6 bất biến 8)", got, maCanBo)
	}
	if b.kho.capBoiCuoi != maCanBo {
		t.Errorf("cap_boi = %q, muốn mã cán bộ %q", b.kho.capBoiCuoi, maCanBo)
	}
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViLuuPhanQuyenVaiTro {
		t.Errorf("action = %q", got)
	}
	if got := chuoiArg(t, vet, viTriChuThe); got != vaiTroDich {
		t.Errorf("subject = %q, muốn id vai trò %q", got, vaiTroDich)
	}
	d := deltaCua(t, vet)
	for truong, muon := range map[string]string{
		"truoc": "document.read", "sau": "task.read", "them": "task.read", "bo": "document.read",
	} {
		ds, _ := d[truong].([]any)
		var s []string
		for _, v := range ds {
			str, _ := v.(string)
			s = append(s, str)
		}
		if strings.Join(s, ",") != muon {
			t.Errorf("delta[%s] = %v, muốn [%s]", truong, d[truong], muon)
		}
	}
}

// Duplicates collapse; order does not matter.
func TestLuuPhanQuyenKhoaTrungGopLai(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	sau, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich,
		[]string{"task.read", "document.read", "task.read"}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Luu: %v", err)
	}
	if strings.Join(sau, ",") != "document.read,task.read" {
		t.Errorf("tập = %v, muốn gộp trùng và sắp xếp", sau)
	}
	if strings.Join(b.kho.themCuoi, ",") != "task.read" || b.kho.boCuoi != nil {
		t.Errorf("thêm %v bỏ %v", b.kho.themCuoi, b.kho.boCuoi)
	}
}

// Saving the column as it stands writes nothing and audits nothing (the idem.KhongCan claim).
func TestLuuPhanQuyenKhongDoiThiKhongGhiGi(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	if _, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"document.read"}, nguoiThucHienGia()); err != nil {
		t.Fatalf("Luu: %v", err)
	}
	khongGhiPhanQuyen(t, b.ghi)
}

// An empty set is legitimate: the role is emptied.
func TestLuuPhanQuyenTapRongLaHopLe(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	sau, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Luu: %v", err)
	}
	if len(sau) != 0 || strings.Join(b.kho.boCuoi, ",") != "document.read" {
		t.Errorf("sau %v bỏ %v", sau, b.kho.boCuoi)
	}
	motVet(t, b.ghi)
}

// --- #14 ------------------------------------------------------------------------------------------

// MUTATION THAT MUST TURN THIS RED: remove the `vaiTroToi == vaiTroID` refusal.
func TestLuuPhanQuyenVaiTroCuaChinhMinhBiTuChoi(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	_, err := b.uc.Luu(ctxXa(xaThu), vaiTroQuanTri,
		[]string{"admin.role", "admin.user", "document.read"}, nguoiThucHienGia())
	if !errors.Is(err, ErrTuThaoTacChinhMinh) {
		t.Fatalf("err = %v, muốn ErrTuThaoTacChinhMinh (câu #14, ràng buộc một)", err)
	}
	khongGhiPhanQuyen(t, b.ghi)
}

// MUTATION THAT MUST TURN THIS RED: remove the khongCam subset check.
func TestLuuPhanQuyenThemKhoaMinhKhongCamBiTuChoi(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	_, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich,
		[]string{"document.read", "budget.confirm"}, nguoiThucHienGia())
	var loi *LoiTraoQuyenKhongCam
	if !errors.As(err, &loi) || strings.Join(loi.Thieu, ",") != "budget.confirm" {
		t.Fatalf("err = %v, muốn LoiTraoQuyenKhongCam{budget.confirm}", err)
	}
	khongGhiPhanQuyen(t, b.ghi)
}

// Removing a key the actor does not hold is refused too (user decision 2026-09-24).
func TestLuuPhanQuyenBoKhoaMinhKhongCamBiTuChoi(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	b.kho.vaiTro[vaiTroDich] = []string{"budget.confirm", "document.read"}
	_, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"document.read"}, nguoiThucHienGia())
	var loi *LoiTraoQuyenKhongCam
	if !errors.As(err, &loi) || strings.Join(loi.Thieu, ",") != "budget.confirm" {
		t.Fatalf("err = %v, muốn LoiTraoQuyenKhongCam{budget.confirm}", err)
	}
	khongGhiPhanQuyen(t, b.ghi)
}

// An UNCHANGED key the actor lacks is not an act and is not refused.
func TestLuuPhanQuyenKhoaGiuNguyenKhongCanCam(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	b.kho.vaiTro[vaiTroDich] = []string{"budget.confirm", "document.read"}
	if _, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich,
		[]string{"budget.confirm", "document.read", "task.read"}, nguoiThucHienGia()); err != nil {
		t.Fatalf("giữ nguyên khoá mình không cầm mà bị từ chối: %v", err)
	}
}

// Fail closed: an actor who lost `admin.role` between the route guard and the lock.
func TestLuuPhanQuyenNguoiMatAdminRoleGiuaChungBiTuChoi(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	b.kho.quyenToi = []string{"admin.user", "document.read", "task.read"}
	_, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"task.read"}, nguoiThucHienGia())
	if !errors.Is(err, ErrTraoQuyenKhongCam) {
		t.Fatalf("err = %v, muốn ErrTraoQuyenKhongCam", err)
	}
	khongGhiPhanQuyen(t, b.ghi)
}

// --- rule 5 invariant 3c, 404, shape --------------------------------------------------------------

func TestLuuPhanQuyenKhoaKhongCoTrongDanhMucBiTuChoi(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	_, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"document.read", "x.khong-co"}, nguoiThucHienGia())
	var loi *LoiKhoaQuyenKhongTonTai
	if !errors.As(err, &loi) || strings.Join(loi.Thieu, ",") != "x.khong-co" {
		t.Fatalf("err = %v, muốn LoiKhoaQuyenKhongTonTai{x.khong-co}", err)
	}
	khongGhiPhanQuyen(t, b.ghi)
}

func TestLuuPhanQuyenVaiTroKhongCoLa404(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	_, err := b.uc.Luu(ctxXa(xaThu), "vt-xa-khac", []string{"task.read"}, nguoiThucHienGia())
	if !errors.Is(err, idstore.ErrVaiTroKhongTonTaiDeGhi) {
		t.Fatalf("err = %v, muốn ErrVaiTroKhongTonTaiDeGhi", err)
	}
	khongGhiPhanQuyen(t, b.ghi)
}

func TestLuuPhanQuyenKhoaSaiDangTuChoiTruocGiaoDich(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	_, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"khongcodaucham"}, nguoiThucHienGia())
	if !errors.Is(err, domain.ErrKhoaQuyenSaiDangThuc) {
		t.Fatalf("err = %v", err)
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Errorf("thân sai dạng mà đã mở %d giao dịch", n)
	}
}

// An empty staff code refuses BEFORE the transaction — never falls back to the internal id.
func TestLuuPhanQuyenThieuMaCanBoBiTuChoi(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	nguoi := NguoiThucHien{ID: idNoiBo, Vet: audit.Actor{Kind: "staff", IP: ipGia}}
	if _, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"task.read"}, nguoi); err == nil {
		t.Fatal("thiếu mã cán bộ mà vẫn lưu")
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Errorf("thiếu mã cán bộ mà đã mở %d giao dịch", n)
	}
}

// --- #13, for admin.user and admin.role (see the note at the top on why the fixtures disagree) ----

func TestLuuPhanQuyenBoAdminUserKhoiNguoiGiuCuoiCungBiTuChoi(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	b.kho.vaiTro[vaiTroDich] = []string{"admin.user", "document.read"}
	b.kho.quanTri = []domain.NguoiGiuQuyen{{CanBoID: idNguoiKhac, VaiTroID: vaiTroDich}}

	_, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"document.read"}, nguoiThucHienGia())
	var loi *LoiKhongConNguoiGiu
	if !errors.As(err, &loi) || loi.Khoa != "admin.user" {
		t.Fatalf("err = %v, muốn LoiKhongConNguoiGiu{admin.user} (câu #13)", err)
	}
	khongGhiPhanQuyen(t, b.ghi)
}

// MUTATION THAT MUST TURN THIS RED: remove the admin.role ConNguoiGiuSauKhiLuu check.
func TestLuuPhanQuyenBoAdminRoleKhoiNguoiGiuCuoiCungBiTuChoi(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	b.kho.vaiTro[vaiTroDich] = []string{"admin.role", "document.read"}
	b.kho.phanQuyen = []domain.NguoiGiuQuyen{{CanBoID: idNguoiKhac, VaiTroID: vaiTroDich}}

	_, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"document.read"}, nguoiThucHienGia())
	var loi *LoiKhongConNguoiGiu
	if !errors.As(err, &loi) || loi.Khoa != "admin.role" {
		t.Fatalf("err = %v, muốn LoiKhongConNguoiGiu{admin.role} (câu #13 mở rộng)", err)
	}
	khongGhiPhanQuyen(t, b.ghi)
}

// A holder through ANOTHER role keeps the key: removing it here is safe and allowed.
func TestLuuPhanQuyenBoAdminUserKhiConNguoiGiuNoiKhacDuocPhep(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	b.kho.vaiTro[vaiTroDich] = []string{"admin.user", "document.read"}
	b.kho.quanTri = append(b.kho.quanTri, domain.NguoiGiuQuyen{CanBoID: idNguoiKhac, VaiTroID: vaiTroDich})

	if _, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"document.read"}, nguoiThucHienGia()); err != nil {
		t.Fatalf("còn quản trị viên ở vai trò khác mà vẫn bị từ chối: %v", err)
	}
	motVet(t, b.ghi)
}
