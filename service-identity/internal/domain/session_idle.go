package domain

import "time"

// The idle lock of a STAFF session — open question #38, DECIDED 2026-09-30. Decision text: "Khoá
// phiên khi KHÔNG DÙNG: cán bộ 30 phút, phiên có quyền quản trị 15 phút — lệch mức gợi ý 15/5 của
// §5.5.2.2 có chủ ý, lý do: bộ phận một cửa vừa tiếp dân vừa thao tác, 15/5 là đăng nhập lại hàng
// chục lần mỗi ngày. So với cột dung_gan_nhat. Phiên công dân Mini App KHÔNG áp. Hạn tuyệt đối 12 giờ
// giữ nguyên."
//
// THE DEVIATION FROM THE STANDARD'S 15/5 IS THE OWNER'S, ON THE RECORD, WITH ITS REASON. Tightening
// these toward 15/5 is a new decision, not a fix; so is loosening them (rule 13, forbidden #4).
//
// PLATFORM-WIDE CONSTANTS, for the reason given on SignInLock's constants. They sit UNDER the absolute
// lifetime (store.ThoiHanPhien, 12 h, #18), never instead of it: activity keeps a session alive for at
// most 12 hours in total.
//
// CITIZEN SESSIONS ARE OUT OF SCOPE by the same decision — they live in `phien_cong_dan`, with their
// own lifetime (CITIZEN_SESSION_TTL), and nothing here is read on that path.
const (
	StaffSessionIdleTimeout = 30 * time.Minute

	// AdminSessionIdleTimeout applies to a session whose account holds AdminSessionPermission at the
	// moment of the request. Read LIVE, not stamped on the session at sign-in: a person granted the
	// key mid-session gets the shorter limit on the next request, and one who loses it gets the
	// longer one — the same "permissions are read on every request" rule the whole edge follows.
	AdminSessionIdleTimeout = 15 * time.Minute

	// AdminSessionPermission is what makes a session "phiên có quyền quản trị". It is `admin.user`
	// because that is the key that makes somebody the commune's administrator everywhere else in
	// this service (#13's "last administrator" is counted on it — store.QuyenQuanTriNguoiDung): it
	// is the key that can issue accounts, reset passwords and move people between roles, i.e. the
	// one whose session is worth more to steal. The other `admin.*` keys configure catalogues.
	AdminSessionPermission = "admin.user"
)

// SessionIdleExpired reports whether a staff session last active at lastActivity has gone idle at
// now. isAdmin is asked ONLY when the answer depends on it — idle between the two limits — so the
// permission read it costs is paid on a handful of requests, not on every one.
//
// Exactly at the limit counts as expired: "khoá sau 30 phút" is reached at the 30th minute.
func SessionIdleExpired(lastActivity, now time.Time, isAdmin func() bool) bool {
	idle := now.Sub(lastActivity)
	switch {
	case idle >= StaffSessionIdleTimeout:
		return true
	case idle >= AdminSessionIdleTimeout:
		return isAdmin()
	default:
		return false
	}
}
