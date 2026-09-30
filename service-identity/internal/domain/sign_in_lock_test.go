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
	if StaffSessionIdleTimeout != 30*time.Minute || AdminSessionIdleTimeout != 15*time.Minute {
		t.Errorf("#38 decided 30 / 15 minutes; code has %v / %v", StaffSessionIdleTimeout, AdminSessionIdleTimeout)
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
		{29 * time.Minute, false, false},
		{30 * time.Minute, false, true},
		{31 * time.Minute, false, true},
		{14 * time.Minute, true, false},
		{15 * time.Minute, true, true},
		{16 * time.Minute, true, true},
		{16 * time.Minute, false, false},
	} {
		if got := SessionIdleExpired(now.Add(-c.idle), now, admin(c.isAdmin)); got != c.want {
			t.Errorf("idle %v admin %v: expired = %v, want %v", c.idle, c.isAdmin, got, c.want)
		}
	}
	asked = 0
	SessionIdleExpired(now.Add(-time.Minute), now, admin(true))
	SessionIdleExpired(now.Add(-time.Hour), now, admin(true))
	if asked != 0 {
		t.Errorf("admin permission asked %d times outside the 15–30 minute window — a query per request for nothing", asked)
	}
}
