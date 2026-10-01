package app

import (
	"encoding/json"
	"strings"
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

// --- the security log (skills/security-logging: account locked / unlocked, manual or automatic) ---

// securityLines returns the JSON log lines carrying an `event` field, in order.
func securityLines(t *testing.T, b *banThuDanhBa) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(b.log.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line is not JSON: %v — %s", err, line)
		}
		if _, ok := m["event"]; ok {
			out = append(out, m)
		}
	}
	return out
}

// assertLine checks the event shape every line must carry — the CODES of both people, the commune
// and the administrator's IP — and that no personal data of the target reached it (rule 3).
func assertLine(t *testing.T, b *banThuDanhBa, l map[string]any, event, outcome, level string) {
	t.Helper()
	if l["event"] != event || l["outcome"] != outcome || l["level"] != level {
		t.Errorf("line = event %v outcome %v level %v, want %s %s %s", l["event"], l["outcome"], l["level"], event, outcome, level)
	}
	if l["actor"] != maCanBo || l["subject"] != maNguoiKhac {
		t.Errorf("actor/subject = %v/%v, want the staff CODES %s/%s", l["actor"], l["subject"], maCanBo, maNguoiKhac)
	}
	if l["xa"] != string(xaThu) || l["ip"] != ipGia {
		t.Errorf("xa/ip = %v/%v, want %s/%s", l["xa"], l["ip"], xaThu, ipGia)
	}
	raw := b.log.String()
	cb := canBoGia()
	for _, pii := range []string{cb.HoTen, cb.Email, cb.DiDongCaNhan, cb.DienThoaiCoQuan, idNguoiKhac, idNoiBo} {
		if strings.Contains(raw, pii) {
			t.Errorf("the security log carries %q — personal data or an internal id (rule 3, rule 6 inv. 8)", pii)
		}
	}
}

func TestManualLockWritesSecurityLogLine(t *testing.T) {
	b := dungBanThuDanhBa(t)

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	lines := securityLines(t, b)
	if len(lines) != 1 {
		t.Fatalf("%d security lines, want 1: %v", len(lines), lines)
	}
	assertLine(t, b, lines[0], EventStaffAccountLocked, "locked", "INFO")
}

func TestManualUnlockWritesSecurityLogLine(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.DangHoatDong = false // retired (#10), no automatic lock

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	lines := securityLines(t, b)
	if len(lines) != 1 {
		t.Fatalf("%d security lines, want 1: %v", len(lines), lines)
	}
	assertLine(t, b, lines[0], EventStaffAccountUnlocked, "unlocked", "INFO")
}

// The early lift of a lock IN FORCE is Warn: it cancels the brute-force defence while it works.
// The reply row must no longer carry the lock it just lifted.
func TestEarlyAutoUnlockWritesWarnLineAndClearsTheRow(t *testing.T) {
	b := dungBanThuDanhBa(t)
	lockedAutomatically(b, 0)
	b.kho.cb.SignInLockedUntil = b.kho.signInLock.LockedUntil // what TheoIDDeGhi would have read

	sau, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia())
	if err != nil {
		t.Fatal(err)
	}
	if sau.SignInLockedUntil != nil {
		t.Errorf("reply row still carries SignInLockedUntil = %v after the lift", sau.SignInLockedUntil)
	}
	lines := securityLines(t, b)
	if len(lines) != 1 {
		t.Fatalf("%d security lines, want 1 (the manual lock was not set): %v", len(lines), lines)
	}
	assertLine(t, b, lines[0], EventStaffSignInUnlocked, "unlocked", "WARN")
	if lines[0]["was_locked"] != true || lines[0]["locked_until"] == "" {
		t.Errorf("line does not say a live lock was lifted: %v", lines[0])
	}
}

// Only a pending run of failures (no lock in force): still logged, at Info.
func TestClearingPendingFailuresLogsAtInfo(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.signInLock = domain.SignInLock{FailedCount: 3}

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	lines := securityLines(t, b)
	if len(lines) != 1 {
		t.Fatalf("%d security lines, want 1: %v", len(lines), lines)
	}
	assertLine(t, b, lines[0], EventStaffSignInUnlocked, "unlocked", "INFO")
	if lines[0]["was_locked"] != false || lines[0]["failed_sign_in_count"] != float64(3) {
		t.Errorf("line = %v, want was_locked=false failed_sign_in_count=3", lines[0])
	}
}

func TestUnlockOfBothLocksWritesTwoLines(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.DangHoatDong = false
	lockedAutomatically(b, 0)

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	events := map[any]bool{}
	for _, l := range securityLines(t, b) {
		events[l["event"]] = true
	}
	if len(events) != 2 || !events[EventStaffSignInUnlocked] || !events[EventStaffAccountUnlocked] {
		t.Fatalf("events %v, want one line per lock that moved", events)
	}
}

// A no-op writes no trail and no security line: "unlocked" for an account that was not locked would
// be a false record in either stream.
func TestNoOpLockAndUnlockWriteNoSecurityLine(t *testing.T) {
	b := dungBanThuDanhBa(t) // active, clean
	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	b.kho.cb.DangHoatDong = false
	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia()); err != nil {
		t.Fatal(err)
	}
	if lines := securityLines(t, b); len(lines) != 0 {
		t.Fatalf("no-op wrote security lines: %v", lines)
	}
}

// A refused lock (#13, last administrator) rolls back and must not log a lock that never landed.
func TestRefusedLockWritesNoSecurityLine(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.quanTri = []string{idNguoiKhac} // the target is the only administrator

	if _, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia()); err == nil {
		t.Fatal("locking the last administrator was not refused")
	}
	if lines := securityLines(t, b); len(lines) != 0 {
		t.Fatalf("a refused lock wrote security lines: %v", lines)
	}
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
