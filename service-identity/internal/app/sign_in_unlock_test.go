package app

import (
	"testing"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The administrator's EARLY unlock of #39, on the unlock direction of DatKhoa (DELETE
// /api/v1/staff/{id}/lockout, `admin.user`). The two locks are two facts: each is lifted only if it
// is set, each under its own verb, and locking never touches the automatic one.

func lockedAutomatically(b *banThuDanhBa, count int) {
	until := b.uc.bayGio().Add(6 * time.Hour)
	b.kho.signInLock = domain.SignInLock{FailedCount: count, LockedUntil: &until}
}

func TestAdminUnlockLiftsAutoLockAndResetsCounter(t *testing.T) {
	b := dungBanThuDanhBa(t) // the target is ACTIVE (dang_hoat_dong = true): only the auto-lock is set
	lockedAutomatically(b, 0)

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatalf("DatKhoa unlock: %v", err)
	}
	if !b.kho.signInLock.IsClear() {
		t.Fatalf("auto-lock after the admin unlock = %+v, want cleared", b.kho.signInLock)
	}
	e := motVet(t, b.ghi) // exactly one: the manual lock was not set, so only the auto one is lifted
	if chuoiArg(t, e, viTriHanhVi) != ActionSignInLockLifted {
		t.Errorf("verb %q, want %q — must not be the manual unlock's verb", chuoiArg(t, e, viTriHanhVi), ActionSignInLockLifted)
	}
	if chuoiArg(t, e, viTriActor) != maCanBo || chuoiArg(t, e, viTriChuThe) != maNguoiKhac {
		t.Error("the entry must name the administrator's CODE as actor and the target's CODE as subject")
	}
	d := deltaCua(t, e)
	if d["truoc"].(map[string]any)["sign_in_locked_until"] == nil || d["sau"].(map[string]any)["sign_in_locked_until"] != nil {
		t.Errorf("delta does not show the lock before and none after: %v", d)
	}
	update := b.ghi.tim("UPDATE nguoi_dung SET sign-in-lock")
	if update == nil || update.tx != e.tx || b.ghi.ketThucCua(e.tx) != "commit" {
		t.Fatal("the unlock and its entry are not one committed transaction")
	}
}

func TestAdminUnlockClearsAPendingRunOfFailures(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.signInLock = domain.SignInLock{FailedCount: 3}

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	if b.kho.signInLock.FailedCount != 0 {
		t.Fatalf("count = %d after the admin unlock, want 0", b.kho.signInLock.FailedCount)
	}
	motVet(t, b.ghi)
}

func TestAdminUnlockBothLocksTwoEntries(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.DangHoatDong = false // retired (#10) AND auto-locked
	lockedAutomatically(b, 0)

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	verbs := map[string]bool{}
	for _, e := range vetDaGhi(b.ghi) {
		verbs[chuoiArg(t, e, viTriHanhVi)] = true
	}
	if len(vetDaGhi(b.ghi)) != 2 || !verbs[ActionSignInLockLifted] || !verbs[HanhViMoKhoaCanBo] {
		t.Fatalf("entries %v — want one per lock, each under its own verb", verbs)
	}
}

func TestAdminUnlockOnCleanAccountWritesNothing(t *testing.T) {
	b := dungBanThuDanhBa(t) // active, no auto-lock: nothing to unlock

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	khongCoGhi(t, b.ghi)
}

func TestAdminUnlockIgnoresAnExpiredLockWithNoFailures(t *testing.T) {
	b := dungBanThuDanhBa(t)
	past := b.uc.bayGio().Add(-time.Hour)
	b.kho.signInLock = domain.SignInLock{LockedUntil: &past}

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	khongCoGhi(t, b.ghi) // "unlocked" for an account that was not locked would be a false record
}

func TestManualLockDoesNotTouchTheAutoLock(t *testing.T) {
	b := dungBanThuDanhBa(t)
	lockedAutomatically(b, 0)
	before := b.kho.signInLock

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	if b.kho.signInLock != before || b.ghi.tim("sign-in-lock") != nil {
		t.Error("locking (#10) read or changed the automatic lock (#39)")
	}
}
