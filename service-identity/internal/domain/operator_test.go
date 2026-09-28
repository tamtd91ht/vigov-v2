package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFormatOperatorCode(t *testing.T) {
	for _, c := range []struct {
		n    int64
		want string
	}{
		{1, "VH-00001"},
		{42, "VH-00042"},
		{99999, "VH-99999"},
		// Past five digits the number grows; it is never truncated into somebody else's code.
		{100000, "VH-100000"},
		{1234567, "VH-1234567"},
	} {
		got, err := FormatOperatorCode(c.n)
		if err != nil {
			t.Fatalf("FormatOperatorCode(%d): %v", c.n, err)
		}
		if got != c.want {
			t.Errorf("FormatOperatorCode(%d) = %q, want %q", c.n, got, c.want)
		}
		if !ValidOperatorCode(got) {
			t.Errorf("FormatOperatorCode(%d) = %q does not pass ValidOperatorCode", c.n, got)
		}
	}
}

func TestFormatOperatorCodeRefusesNonPositive(t *testing.T) {
	for _, n := range []int64{0, -1} {
		if _, err := FormatOperatorCode(n); err == nil {
			t.Errorf("FormatOperatorCode(%d) accepted a number no sequence starting at 1 yields", n)
		}
	}
}

func TestValidOperatorCode(t *testing.T) {
	for _, s := range []string{"", "VH-", "VH-1234", "vh-00001", "CB-00001", "VH-0000A", " VH-00001",
		"VH-00001 ", "CB-2026-7K3M9Q", "01JD9A0000000000000000000A"} {
		if ValidOperatorCode(s) {
			t.Errorf("ValidOperatorCode(%q) = true", s)
		}
	}
}

// The closed list is exactly the six keys of ADR 0048 §Chốt #3. Growing it is an owner decision.
func TestOperatorPermissionsClosedList(t *testing.T) {
	want := []string{"ops.tenant.manage", "ops.domain.manage", "ops.profile.manage",
		"ops.mini_app.manage", "ops.upload_policy.manage", "ops.qr.issue"}
	got := OperatorPermissions()
	if len(got) != len(want) {
		t.Fatalf("closed list has %d keys, want %d", len(got), len(want))
	}
	for i, k := range want {
		if string(got[i]) != k {
			t.Errorf("key %d = %q, want %q", i, got[i], k)
		}
		if !strings.HasPrefix(k, "ops.") {
			t.Errorf("%q lacks the ops. prefix — it could collide with a commune key", k)
		}
	}
	// A fresh slice per call: mutating one must not change the next.
	got[0] = "tampered"
	if OperatorPermissions()[0] != OperatorPermissionTenantManage {
		t.Error("OperatorPermissions shares its backing array with callers")
	}
}

func TestParseOperatorPermission(t *testing.T) {
	for _, k := range OperatorPermissions() {
		p, err := ParseOperatorPermission(string(k))
		if err != nil || p != k {
			t.Errorf("Parse(%q) = %q, %v", k, p, err)
		}
	}
	for _, s := range []string{"", "ops.tenant.manage ", "OPS.TENANT.MANAGE", "task.extend", "ops.everything"} {
		if _, err := ParseOperatorPermission(s); !errors.Is(err, ErrUnknownOperatorPermission) {
			t.Errorf("Parse(%q) err = %v, want ErrUnknownOperatorPermission", s, err)
		}
	}
}

func TestSecurityConstants(t *testing.T) {
	if SessionLifetime != 8*time.Hour {
		t.Errorf("SessionLifetime = %v, owner decided 8h", SessionLifetime)
	}
	if MaxFailedAttempts != 5 {
		t.Errorf("MaxFailedAttempts = %d, owner decided 5", MaxFailedAttempts)
	}
	if LockoutDuration != 12*time.Hour {
		t.Errorf("LockoutDuration = %v, owner decided 12h (28/09/2026, TCVN 14423 §5.5.2.2)", LockoutDuration)
	}
	if SessionIdleTimeout != 5*time.Minute {
		t.Errorf("SessionIdleTimeout = %v, owner decided 5m", SessionIdleTimeout)
	}
	if TemporaryPasswordLifetime != 24*time.Hour {
		t.Errorf("TemporaryPasswordLifetime = %v, owner decided 24h", TemporaryPasswordLifetime)
	}
	if PendingTOTPLifetime != 10*time.Minute {
		t.Errorf("PendingTOTPLifetime = %v, owner decided 10m", PendingTOTPLifetime)
	}
	if RecoveryCodeCount != 10 {
		t.Errorf("RecoveryCodeCount = %d, owner decided 10", RecoveryCodeCount)
	}
}

func TestOperatorAccountLockExpiresByItself(t *testing.T) {
	now := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	until := now.Add(LockoutDuration)
	a := OperatorAccount{LockedUntil: &until}
	if !a.LockedAt(now) {
		t.Error("not locked inside the window")
	}
	if a.LockedAt(until) {
		t.Error("still locked at the instant the lock ends")
	}
	if (OperatorAccount{}).LockedAt(now) {
		t.Error("an account with no lock reports locked")
	}
}

func TestOperatorAuditEntryValidate(t *testing.T) {
	ok := OperatorAuditEntry{Actor: "VH-00001", Action: OperatorAuditLoginSucceeded, Subject: "VH-00001"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid entry refused: %v", err)
	}
	sys := OperatorAuditEntry{Actor: SystemActor, Action: OperatorAuditPermissionGranted,
		Subject: "VH-00002", Reason: "ticket OPS-123"}
	if err := sys.Validate(); err != nil {
		t.Fatalf("system entry with ticket refused: %v", err)
	}

	for name, c := range map[string]struct {
		e    OperatorAuditEntry
		want error
	}{
		"empty actor — no fallback to system": {OperatorAuditEntry{Action: OperatorAuditLoggedOut, Subject: "VH-00001"}, ErrOperatorAuditActor},
		"internal id as actor":                {OperatorAuditEntry{Actor: "01JD9A0000000000000000000A", Action: OperatorAuditLoggedOut, Subject: "VH-00001"}, ErrOperatorAuditActor},
		"staff code as actor":                 {OperatorAuditEntry{Actor: "CB-2026-7K3M9Q", Action: OperatorAuditLoggedOut, Subject: "VH-00001"}, ErrOperatorAuditActor},
		"unknown action":                      {OperatorAuditEntry{Actor: "VH-00001", Action: "operator.anything", Subject: "VH-00001"}, ErrOperatorAuditAction},
		"email as subject":                    {OperatorAuditEntry{Actor: "VH-00001", Action: OperatorAuditLoggedOut, Subject: "a@example.test"}, ErrOperatorAuditSubject},
		"system without ticket":               {OperatorAuditEntry{Actor: SystemActor, Action: OperatorAuditMFAReset, Subject: "VH-00001", Reason: "  "}, ErrOperatorAuditReason},
	} {
		if err := c.e.Validate(); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}

// NIL IS EXPIRED on both helpers: no recorded instant must never mean "works forever".
func TestOperatorCredentialExpiryHelpers(t *testing.T) {
	now := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	later, earlier := now.Add(time.Minute), now.Add(-time.Minute)
	cases := []struct {
		name string
		a    OperatorAccount
		want bool
	}{
		{"own password", OperatorAccount{}, false},
		{"temporary, valid", OperatorAccount{MustChangePassword: true, TemporaryPasswordExpiresAt: &later}, false},
		{"temporary, at expiry", OperatorAccount{MustChangePassword: true, TemporaryPasswordExpiresAt: &now}, true},
		{"temporary, past", OperatorAccount{MustChangePassword: true, TemporaryPasswordExpiresAt: &earlier}, true},
		{"temporary, no expiry", OperatorAccount{MustChangePassword: true}, true},
	}
	for _, c := range cases {
		if got := c.a.TemporaryPasswordExpiredAt(now); got != c.want {
			t.Errorf("%s: expired=%v, want %v", c.name, got, c.want)
		}
	}
	created := now.Add(-PendingTOTPLifetime + time.Second)
	if (OperatorAccount{TOTPPending: true, PendingTOTPCreatedAt: &created}).PendingTOTPExpiredAt(now) {
		t.Error("pending 1 s inside its lifetime reported expired")
	}
	old := now.Add(-PendingTOTPLifetime)
	if !(OperatorAccount{TOTPPending: true, PendingTOTPCreatedAt: &old}).PendingTOTPExpiredAt(now) {
		t.Error("pending at its lifetime reported live")
	}
	if !(OperatorAccount{TOTPPending: true}).PendingTOTPExpiredAt(now) || !(OperatorAccount{}).PendingTOTPExpiredAt(now) {
		t.Error("an undated or absent pending secret must count as expired")
	}
	for _, a := range []OperatorAuditAction{OperatorAuditUnlocked, OperatorAuditTOTPEnrollmentStarted} {
		if !a.Valid() {
			t.Errorf("%s is not on the closed list", a)
		}
	}
}
