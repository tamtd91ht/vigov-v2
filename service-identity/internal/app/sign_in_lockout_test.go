package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The automatic sign-in lockout — open question #39, decided 2026-09-30: five CONSECUTIVE failures
// lock a staff account for twelve hours; a success restarts the count; the lock and its lifting are
// audited; a locked account is refused WITHOUT saying whether the password was right.
//
// Runs through the REAL DangNhap and the REAL store SQL, on the fake driver of
// dang_nhap_giao_dich_test.go, which keeps the two lock columns as state. What it cannot prove —
// that FOR UPDATE serialises two parallel guesses — needs a PostgreSQL.

var lockoutStart = time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)

func lockoutHarness(t *testing.T) (*banThu, *time.Time) {
	t.Helper()
	b := dungBanThu(t)
	now := lockoutStart
	b.dangNhap.now = func() time.Time { return now }
	return b, &now
}

func wrongPassword() YeuCauDangNhap {
	yc := yeuCauDung()
	yc.MatKhau = "mat-khau-sai-hoan-toan"
	return yc
}

func mustRefuse(t *testing.T, b *banThu, yc YeuCauDangNhap) {
	t.Helper()
	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yc); !errors.Is(err, ErrDangNhapThatBai) {
		t.Fatalf("want ErrDangNhapThatBai, got %v", err)
	}
}

func lockEntries(b *banThu) []lenhGhi {
	var out []lenhGhi
	for _, l := range vetDaGhi(b.ghi) {
		if l.args[4] == ActionSignInAutoLocked {
			out = append(out, l)
		}
	}
	return out
}

func TestFourFailuresDoNotLockTheFifthDoes(t *testing.T) {
	b, now := lockoutHarness(t)

	for i := 1; i <= 4; i++ {
		mustRefuse(t, b, wrongPassword())
		if b.ghi.failedCount != int64(i) || b.ghi.lockedUntil != nil {
			t.Fatalf("after %d failures: count %d, locked %v", i, b.ghi.failedCount, b.ghi.lockedUntil)
		}
	}
	if n := len(vetDaGhi(b.ghi)); n != 0 {
		t.Fatalf("%d audit entries below the threshold — a failed guess is a security-log event, not a record", n)
	}

	mustRefuse(t, b, wrongPassword())
	want := now.Add(12 * time.Hour)
	if b.ghi.lockedUntil == nil || !b.ghi.lockedUntil.Equal(want) {
		t.Fatalf("5th failure: locked until %v, want %v", b.ghi.lockedUntil, want)
	}
	if b.ghi.failedCount != 0 {
		t.Errorf("count after the lock = %d, want 0 — the lock's expiry must open a fresh window of five", b.ghi.failedCount)
	}

	entries := lockEntries(b)
	if len(entries) != 1 {
		t.Fatalf("lock audit entries = %d, want exactly 1", len(entries))
	}
	e := entries[0]
	// (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
	if e.args[0] != string(xaThu) || e.args[1] != audit.SystemActor || e.args[2] != "system" ||
		e.args[3] != ipGia || e.args[5] != maCanBo {
		t.Errorf("lock entry %v — want commune, SYSTEM actor, the attempt's IP, the staff CODE as subject", e.args[:6])
	}
	var lockWrite *lenhGhi
	for i := range b.ghi.lenh {
		l := b.ghi.lenh[i]
		if strings.Contains(l.sql, "SET failed_sign_in_count") && l.tx == e.tx {
			lockWrite = &l
		}
	}
	if lockWrite == nil {
		t.Fatal("the lock and its audit entry are not in ONE transaction (rule 6, invariant 3)")
	}
	if b.ghi.ketThucCua(e.tx) != "commit" {
		t.Fatalf("the locking transaction ended %q — the lock must commit although the sign-in fails", b.ghi.ketThucCua(e.tx))
	}
	var delta map[string]any
	if err := json.Unmarshal(e.args[7].([]byte), &delta); err != nil {
		t.Fatalf("delta: %v", err)
	}
	if delta["sau"].(map[string]any)["sign_in_locked_until"] == nil {
		t.Errorf("the entry does not say until when: %v", delta)
	}
	if !strings.Contains(b.log.String(), "event=staff.sign_in_locked") {
		t.Errorf("no security-log line for the lock:\n%s", b.log.String())
	}
	if b.ghi.tim("INSERT INTO phien") != nil {
		t.Fatal("a session was opened during the failures")
	}
}

func TestLockedAccountRefusedEvenWithRightPassword(t *testing.T) {
	b, _ := lockoutHarness(t)
	for i := 0; i < domain.MaxConsecutiveFailedSignIns; i++ {
		mustRefuse(t, b, wrongPassword())
	}
	lockedUntil := *b.ghi.lockedUntil
	b.log.Reset()

	// The RIGHT password, while locked.
	_, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung())
	if !errors.Is(err, ErrDangNhapThatBai) {
		t.Fatalf("locked account with the right password: %v, want ErrDangNhapThatBai", err)
	}
	if err.Error() != ErrDangNhapThatBai.Error() {
		t.Errorf("the refusal says something a wrong password does not: %q", err.Error())
	}
	if b.ghi.tim("INSERT INTO phien") != nil {
		t.Fatal("A LOCKED ACCOUNT SIGNED IN")
	}
	if !b.ghi.lockedUntil.Equal(lockedUntil) || b.ghi.failedCount != 0 {
		t.Error("an attempt during the lock changed it — guesses must neither count nor extend the lock")
	}
	// ONE LOG LINE, the same one a wrong password writes.
	wrong := dungBanThu(t)
	mustRefuse(t, wrong, wrongPassword())
	if a, c := boThoiGian(b.log.String()), boThoiGian(wrong.log.String()); a != c {
		t.Errorf("locked-with-right-password logs differently from a wrong password:\n  locked: %s\n  wrong:  %s", a, c)
	}
}

func TestLockExpiresAfterTwelveHours(t *testing.T) {
	b, now := lockoutHarness(t)
	for i := 0; i < domain.MaxConsecutiveFailedSignIns; i++ {
		mustRefuse(t, b, wrongPassword())
	}

	*now = lockoutStart.Add(12*time.Hour - time.Second)
	mustRefuse(t, b, yeuCauDung())

	*now = lockoutStart.Add(12 * time.Hour)
	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); err != nil {
		t.Fatalf("after 12 hours the right password is refused: %v", err)
	}
	if b.ghi.lockedUntil != nil || b.ghi.failedCount != 0 {
		t.Errorf("a success after the lock left count %d, locked %v", b.ghi.failedCount, b.ghi.lockedUntil)
	}
}

func TestSuccessResetsTheCounter(t *testing.T) {
	b, _ := lockoutHarness(t)
	for i := 0; i < 4; i++ {
		mustRefuse(t, b, wrongPassword())
	}
	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); err != nil {
		t.Fatalf("right password after 4 failures: %v", err)
	}
	if b.ghi.failedCount != 0 {
		t.Fatalf("count after a success = %d, want 0 — #39 counts CONSECUTIVE failures", b.ghi.failedCount)
	}
	// Four more: still not locked, because the run restarted.
	for i := 0; i < 4; i++ {
		mustRefuse(t, b, wrongPassword())
	}
	if b.ghi.lockedUntil != nil {
		t.Fatal("locked after 4+success+4 — the success did not restart the run")
	}
}

func TestCleanSuccessWritesNoExtraStatement(t *testing.T) {
	b, _ := lockoutHarness(t)
	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); err != nil {
		t.Fatal(err)
	}
	if b.ghi.tim("SET failed_sign_in_count") != nil {
		t.Error("an ordinary sign-in on a clean account rewrote the lock columns")
	}
}
