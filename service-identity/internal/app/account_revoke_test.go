package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// TaiKhoanCanBo.Revoke — `admin.user.revoke`, user decision 2026-10-03. On the fake driver, which
// records which statement ran in which transaction; the real SQL is in account_revoke_pg_test.go.
//
// Every property here fails SILENTLY when broken:
//
//	sessions not revoked       the person keeps working until the token expires — "revoked" in the
//	                           ledger, signed in on the screen.
//	audit outside the tx       the account goes and the trail does not (rule 6, invariant 3).
//	actor = internal id        an entry nobody can read in an inspection (rule 6, invariant 8).
//	no #13                     the commune revokes its last administrator and nobody can administer it.

const revokeReason = "Dòng nhập trùng, cần xoá sau khi thu hồi"

// newRevokeBench is the bench with the target holding an account — the state revoke starts from.
func newRevokeBench(t *testing.T) *banThuTaiKhoan {
	t.Helper()
	b := dungBanThuTaiKhoan(t)
	b.kho.cb.CoTaiKhoan = true
	return b
}

// THE WHOLE ACT IN ONE COMMITTED TRANSACTION: account write, session revocation, audit entry.
//
// MUTATIONS THAT MUST TURN THIS RED: drop the ThuHoiCuaCanBo call; move audit.Write out of the Tx
// closure; build the actor from nguoi.ID.
func TestRevokeAccountSessionsAndAuditShareOneCommittedTransaction(t *testing.T) {
	b := newRevokeBench(t)

	result, err := b.uc.Revoke(ctxXa(xaThu), idNguoiKhac, "  "+revokeReason+"  ", nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Revoke lỗi: %v", err)
	}
	if result.CoTaiKhoan {
		t.Error("bản ghi trả về vẫn CoTaiKhoan = true")
	}
	if b.kho.revokedID != idNguoiKhac {
		t.Fatalf("RevokeAccount nhận id %q, muốn %q", b.kho.revokedID, idNguoiKhac)
	}
	if len(b.phien.lyDo) != 1 || b.phien.lyDo[0] != sessionRevokedByRevoke {
		t.Fatalf("thu hồi phiên = %v, muốn đúng một lần với lý do %q", b.phien.lyDo, sessionRevokedByRevoke)
	}

	write := b.ghi.tim("dau-hieu-thu-hoi-tai-khoan")
	sessions := b.ghi.tim("dau-hieu-thu-hoi)")
	entry := motVet(t, b.ghi)
	if write == nil || sessions == nil {
		t.Fatalf("thiếu câu ghi: tài khoản=%v phiên=%v", write != nil, sessions != nil)
	}
	if entry.tx == 0 || write.tx != entry.tx || sessions.tx != entry.tx {
		t.Fatalf("giao dịch: tài khoản=%d phiên=%d vết=%d — phải là MỘT (luật 6 bất biến 3)",
			write.tx, sessions.tx, entry.tx)
	}
	if b.ghi.ketThucCua(entry.tx) != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", b.ghi.ketThucCua(entry.tx))
	}

	if got := chuoiArg(t, entry, viTriHanhVi); got != ActionAccountRevoked {
		t.Errorf("hành vi = %q, muốn %q", got, ActionAccountRevoked)
	}
	if got := chuoiArg(t, entry, viTriActor); got != maCanBo {
		t.Errorf("actor = %q, muốn mã cán bộ %q (luật 6 bất biến 8)", got, maCanBo)
	}
	if got := chuoiArg(t, entry, viTriChuThe); got != maNguoiKhac {
		t.Errorf("chủ thể = %q, muốn mã cán bộ %q", got, maNguoiKhac)
	}

	delta := deltaCua(t, entry)
	beforeSide, _ := delta["truoc"].(map[string]any)
	afterSide, _ := delta["sau"].(map[string]any)
	if beforeSide["co_tai_khoan"] != true || afterSide["co_tai_khoan"] != false {
		t.Errorf("delta co_tai_khoan: truoc=%v sau=%v, muốn true -> false",
			beforeSide["co_tai_khoan"], afterSide["co_tai_khoan"])
	}
	if delta["ly_do"] != revokeReason {
		t.Errorf("delta.ly_do = %v, muốn lý do đã cắt khoảng trắng", delta["ly_do"])
	}
	raw := string(entry.args[viTriDelta].([]byte))
	if !strings.Contains(raw, "mat_khau_hash") {
		t.Error("delta không NÊU tên cột mat_khau_hash")
	}
	if strings.Contains(raw, idNguoiKhac) {
		t.Error("delta mang định danh nội bộ của người bị thu hồi")
	}

	// The security log: one line, staff codes, never the reason text.
	line := b.log.String()
	if !strings.Contains(line, EventStaffAccountRevoked) || !strings.Contains(line, maNguoiKhac) {
		t.Errorf("nhật ký an ninh thiếu sự kiện hoặc mã cán bộ: %s", line)
	}
	if strings.Contains(line, "nhập trùng") {
		t.Error("nhật ký an ninh chứa nội dung lý do (văn bản tự do)")
	}
}

// THE ROLE IS KEPT, so a re-issue restores the same authority (see Revoke's comment).
//
// MUTATION THAT MUST TURN THIS RED: clear VaiTroID on the returned row.
func TestRevokeKeepsRole(t *testing.T) {
	b := newRevokeBench(t)

	result, err := b.uc.Revoke(ctxXa(xaThu), idNguoiKhac, revokeReason, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Revoke lỗi: %v", err)
	}
	if result.VaiTroID != vaiTroYeu {
		t.Errorf("vai trò sau thu hồi = %q, muốn giữ %q", result.VaiTroID, vaiTroYeu)
	}
	afterSide := deltaCua(t, motVet(t, b.ghi))["sau"].(map[string]any)
	if afterSide["vai_tro_id"] != vaiTroYeu {
		t.Errorf("delta.sau.vai_tro_id = %v, muốn %q", afterSide["vai_tro_id"], vaiTroYeu)
	}
}

// #13 — THE SAME LOCKED READ, IN THE SAME ORDER, AS DatKhoa: the administrator set before the row.
//
// MUTATION THAT MUST TURN THIS RED: read the target first, or drop the administrator-set read.
func TestRevokeLocksAdminSetBeforeTargetRow(t *testing.T) {
	b := newRevokeBench(t)

	if _, err := b.uc.Revoke(ctxXa(xaThu), idNguoiKhac, revokeReason, nguoiThucHienGia()); err != nil {
		t.Fatalf("Revoke lỗi: %v", err)
	}
	b.ghi.mu.Lock()
	defer b.ghi.mu.Unlock()
	adminAt, rowAt := -1, -1
	for i, l := range b.ghi.lenh {
		switch {
		case strings.Contains(l.sql, "admin-set") && adminAt < 0:
			adminAt = i
		case strings.Contains(l.sql, "doc-de-ghi") && rowAt < 0:
			rowAt = i
		}
	}
	if adminAt < 0 || rowAt < 0 || adminAt > rowAt {
		t.Fatalf("thứ tự khoá: tập quản trị ở %d, dòng đích ở %d — tập quản trị phải đi trước", adminAt, rowAt)
	}
}

// #13 — revoking the LAST administrator is refused and writes nothing.
func TestRevokeLastAdminRefused(t *testing.T) {
	b := newRevokeBench(t)
	b.kho.admins = []string{idNguoiKhac}

	if _, err := b.uc.Revoke(ctxXa(xaThu), idNguoiKhac, revokeReason, nguoiThucHienGia()); !errors.Is(err, ErrQuanTriCuoiCung) {
		t.Fatalf("lỗi = %v, muốn ErrQuanTriCuoiCung", err)
	}
	khongCoGhi(t, b.ghi)
	if len(b.phien.lyDo) != 0 {
		t.Error("bị từ chối nhưng vẫn thu hồi phiên")
	}
}

// A sole administrator who is NOT the target does not block revoking somebody else.
func TestRevokeOrdinaryStaffWithSingleOtherAdminAllowed(t *testing.T) {
	b := newRevokeBench(t)
	b.kho.admins = []string{idQuanTri2}

	if _, err := b.uc.Revoke(ctxXa(xaThu), idNguoiKhac, revokeReason, nguoiThucHienGia()); err != nil {
		t.Fatalf("lỗi = %v, muốn thành công", err)
	}
}

// #14 — refused before a transaction opens.
func TestRevokeSelfRefused(t *testing.T) {
	b := newRevokeBench(t)

	if _, err := b.uc.Revoke(ctxXa(xaThu), idNoiBo, revokeReason, nguoiThucHienGia()); !errors.Is(err, ErrTuThaoTacChinhMinh) {
		t.Fatalf("lỗi = %v, muốn ErrTuThaoTacChinhMinh", err)
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Fatalf("mở %d giao dịch cho yêu cầu tự thu hồi", n)
	}
}

func TestRevokeWithoutAccountRefused(t *testing.T) {
	b := dungBanThuTaiKhoan(t) // directory-only row

	if _, err := b.uc.Revoke(ctxXa(xaThu), idNguoiKhac, revokeReason, nguoiThucHienGia()); !errors.Is(err, ErrChuaCoTaiKhoan) {
		t.Fatalf("lỗi = %v, muốn ErrChuaCoTaiKhoan", err)
	}
	khongCoGhi(t, b.ghi)
	if len(b.phien.lyDo) != 0 {
		t.Error("không có tài khoản nhưng vẫn thu hồi phiên")
	}
	if b.log.Len() != 0 {
		t.Errorf("bị từ chối nhưng vẫn ghi nhật ký an ninh: %s", b.log.String())
	}
}

// A missing or blank reason is refused before any transaction opens.
func TestRevokeReasonRequired(t *testing.T) {
	for _, raw := range []string{"", "   "} {
		b := newRevokeBench(t)
		if _, err := b.uc.Revoke(ctxXa(xaThu), idNguoiKhac, raw, nguoiThucHienGia()); !errors.Is(err, domain.ErrRevokeReasonMissing) {
			t.Errorf("%q: lỗi = %v, muốn ErrRevokeReasonMissing", raw, err)
		}
		if n := b.ghi.soGiaoDich(); n != 0 {
			t.Errorf("%q: mở %d giao dịch", raw, n)
		}
	}
}

// A LOCKED account can be revoked — otherwise a retired person's duplicated row could never reach
// the soft delete.
func TestRevokeLockedAccountAllowed(t *testing.T) {
	b := newRevokeBench(t)
	b.kho.cb.DangHoatDong = false

	if _, err := b.uc.Revoke(ctxXa(xaThu), idNguoiKhac, revokeReason, nguoiThucHienGia()); err != nil {
		t.Fatalf("lỗi = %v, muốn thành công", err)
	}
}

// Another commune's id, an invented one and a soft-deleted one are the store's single "not found".
func TestRevokeUnknownIDIsNotFound(t *testing.T) {
	b := newRevokeBench(t)

	if _, err := b.uc.Revoke(ctxXa(xaThu), "nd-khong-co", revokeReason, nguoiThucHienGia()); !errors.Is(err, idstore.ErrCanBoKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrCanBoKhongTonTai", err)
	}
	khongCoGhi(t, b.ghi)
}

// An actor without a staff code refuses the write — no fallback to the internal id.
func TestRevokeActorWithoutCodeRefused(t *testing.T) {
	b := newRevokeBench(t)
	actor := nguoiThucHienGia()
	actor.Vet.ID = ""

	if _, err := b.uc.Revoke(ctxXa(xaThu), idNguoiKhac, revokeReason, actor); err == nil {
		t.Fatal("người thực hiện không có mã cán bộ mà vẫn thu hồi được")
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Fatalf("mở %d giao dịch", n)
	}
}
