package domain

import (
	"testing"
	"time"
)

// The figures are the owner's (open questions #38 and #39, 2026-09-30). Pinned so an edit to a
// number that looks arbitrary has to argue with the decision instead of with a magic constant.
func TestDecidedFigures(t *testing.T) {
	if MaxConsecutiveFailedSignIns != 5 || SignInLockDuration != 12*time.Hour {
		t.Errorf("#39 decided 5 failures / 12 hours; code has %d / %v", MaxConsecutiveFailedSignIns, SignInLockDuration)
	}
	if StaffSessionIdleTimeout != 60*time.Minute || AdminSessionIdleTimeout != 60*time.Minute {
		t.Errorf("#38 changed 2026-10-07 to 60 / 60 minutes; code has %v / %v", StaffSessionIdleTimeout, AdminSessionIdleTimeout)
	}
	if AdminSessionPermission != "admin.user" {
		t.Errorf("admin session key = %q", AdminSessionPermission)
	}
}

func TestSignInLockWalk(t *testing.T) {
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	var l SignInLock
	for i := 1; i <= 4; i++ {
		var locked bool
		l, locked = l.AfterFailure(now)
		if locked || l.LockedAt(now) || l.FailedCount != i {
			t.Fatalf("failure %d: %+v locked=%v", i, l, locked)
		}
	}
	l, locked := l.AfterFailure(now)
	if !locked || !l.LockedAt(now) || l.FailedCount != 0 {
		t.Fatalf("5th failure: %+v locked=%v", l, locked)
	}
	// A failure during the lock neither counts nor extends it.
	again, lockedAgain := l.AfterFailure(now.Add(time.Hour))
	if lockedAgain || again != l {
		t.Errorf("failure during the lock changed it: %+v", again)
	}
	if !l.LockedAt(now.Add(12*time.Hour - time.Nanosecond)) {
		t.Error("lock ended before 12 hours")
	}
	if l.LockedAt(now.Add(12 * time.Hour)) {
		t.Error("lock still in force at 12 hours")
	}
	if !l.Clear().IsClear() {
		t.Error("Clear is not clear")
	}
}

func TestSessionIdleExpired(t *testing.T) {
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	asked := 0
	admin := func(v bool) func() bool { return func() bool { asked++; return v } }
	for _, c := range []struct {
		idle    time.Duration
		isAdmin bool
		want    bool
	}{
		{59 * time.Minute, false, false},
		{60 * time.Minute, false, true},
		{61 * time.Minute, false, true},
		{59 * time.Minute, true, false},
		{60 * time.Minute, true, true},
		{61 * time.Minute, true, true},
	} {
		if got := SessionIdleExpired(now.Add(-c.idle), now, admin(c.isAdmin)); got != c.want {
			t.Errorf("idle %v admin %v: expired = %v, want %v", c.idle, c.isAdmin, got, c.want)
		}
	}
	asked = 0
	SessionIdleExpired(now.Add(-time.Minute), now, admin(true))
	SessionIdleExpired(now.Add(-2*time.Hour), now, admin(true))
	if asked != 0 {
		t.Errorf("admin permission asked %d times outside the admin–staff window — a query per request for nothing", asked)
	}
}

// SignInLockedAt is what the staff register shows: the end of a lock IN FORCE, nil otherwise —
// derived against now, so a stored instant in the past (nobody clears it) shows nothing.
func TestCanBoTomTatSignInLockedAt(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	future, past := now.Add(time.Hour), now.Add(-time.Second)

	if got := (CanBoTomTat{}).SignInLockedAt(now); got != nil {
		t.Errorf("never locked: %v, want nil", got)
	}
	if got := (CanBoTomTat{SignInLockedUntil: &past}).SignInLockedAt(now); got != nil {
		t.Errorf("expired: %v, want nil", got)
	}
	if got := (CanBoTomTat{SignInLockedUntil: &now}).SignInLockedAt(now); got != nil {
		t.Errorf("at the end instant: %v, want nil (LockedAt is now.Before(until))", got)
	}
	cb := CanBoTomTat{SignInLockedUntil: &future}
	got := cb.SignInLockedAt(now)
	if got == nil || !got.Equal(future) {
		t.Fatalf("in force: %v, want %v", got, future)
	}
	if got == cb.SignInLockedUntil {
		t.Error("returned the row's own pointer — a caller editing it would edit the row")
	}
}
