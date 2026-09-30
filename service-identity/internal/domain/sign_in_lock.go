package domain

import "time"

// The automatic sign-in lockout of a STAFF account — open question #39, DECIDED 2026-09-30, TCVN
// 14423 §5.5.2.2. Decision text: "5 lần đăng nhập sai LIÊN TIẾP → khoá tự động 12 giờ (mức tối thiểu
// §5.5.2.2); quản trị viên xã mở khoá sớm được ... Khác và không thay khoá thủ công (lockout)".
//
// PLATFORM-WIDE CONSTANTS, NOT PER-COMMUNE CONFIGURATION. Rule 1 invariant 10 sends LOCAL practice to
// runtime configuration; these are security thresholds the owner fixed for the platform, and a
// commune that loosened them would be loosening the national baseline for its own staff (rule 13,
// forbidden #4). The operator realm carries the same two figures (operator.go MaxFailedAttempts,
// LockoutDuration) on the same reasoning; they are separate constants because they are separate
// decisions that merely agree today.
const (
	// MaxConsecutiveFailedSignIns — the fifth consecutive failure locks.
	MaxConsecutiveFailedSignIns = 5

	// SignInLockDuration — 12 hours, the minimum §5.5.2.2 allows. Shorter would be below the
	// standard; longer turns five mistyped passwords into a day without the system.
	SignInLockDuration = 12 * time.Hour
)

// SignInLock is the automatic-lock state of one staff account (migration 0020). It is NOT
// `dang_hoat_dong`: that is the manual lock of #10, a business state an administrator sets; this one
// is a security state that expires by itself. See the migration for why they are two columns.
type SignInLock struct {
	FailedCount int
	LockedUntil *time.Time
}

// LockedAt reports whether the lock is in force at now. DERIVED from the stored instant, never a
// flag: a lock that has run out needs nobody to clear it.
func (l SignInLock) LockedAt(now time.Time) bool {
	return l.LockedUntil != nil && now.Before(*l.LockedUntil)
}

// Clear is the state after a successful sign-in or an administrator's early unlock.
func (l SignInLock) Clear() SignInLock { return SignInLock{} }

// IsClear reports whether there is anything to reset — so a success on a clean account writes
// nothing extra.
func (l SignInLock) IsClear() bool { return l.FailedCount == 0 && l.LockedUntil == nil }

// AfterFailure is the state after one more failed sign-in at now, and whether THIS failure locked.
//
// A FAILURE WHILE LOCKED IS NOT COUNTED and does not extend the lock: otherwise whoever is guessing
// keeps the real owner out indefinitely by guessing once every twelve hours. The caller refuses such
// an attempt without calling this; the guard here makes the rule hold even if it did.
//
// THE COUNT RESTARTS AT 0 WHEN IT LOCKS, so the lock's expiry opens a fresh window of five — not one
// more guess that re-locks at once.
func (l SignInLock) AfterFailure(now time.Time) (SignInLock, bool) {
	if l.LockedAt(now) {
		return l, false
	}
	n := l.FailedCount + 1
	if n >= MaxConsecutiveFailedSignIns {
		// Microsecond precision, PostgreSQL's, so the instant written is the instant read back.
		until := now.Add(SignInLockDuration).UTC().Truncate(time.Microsecond)
		return SignInLock{FailedCount: 0, LockedUntil: &until}, true
	}
	return SignInLock{FailedCount: n}, false
}
