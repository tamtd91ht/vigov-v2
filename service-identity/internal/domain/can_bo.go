package domain

import "time"

// CanBo is a staff account.
//
// Staff identity is ACCOUNTABLE: every action traces back to a person (rule 4). That is the
// opposite of citizen identity — a phone number plus an OTP — which is deliberately weak and
// never enough on its own for an act with legal consequences. The two never share a type.
type CanBo struct {
	ID          string // ULID, internal
	Ma          string // business code, the one that appears in the audit trail
	HoTen       string
	Email       string
	ChucVu      string
	BoPhanID    string
	VaiTroID    string
	DienThoai   string
	MatKhauHash string // argon2id. NEVER the password, never logged (rule 3, rule 8)

	// CoTaiKhoan and DangHoatDong ANSWER TWO DIFFERENT QUESTIONS, and reading one for the other
	// is the defect migration 0003 exists to repair.
	//
	//   CoTaiKhoan   — "this person has a sign-in account at all". One `nguoi_dung` table holds
	//                  BOTH the public staff directory (26 people in the specification's seed)
	//                  and the accounts that can sign in (12). Directory-only people are false
	//                  here. Granted explicitly, never by default: DEFAULT false in the schema.
	//   DangHoatDong — "this account is not locked out". Meaningful ONLY where CoTaiKhoan is
	//                  true; on a directory-only person it says nothing, because there is no
	//                  account to lock. Defaults to true.
	//
	// Both must hold before anyone signs in or holds a permission. Dropping either widens
	// access: without CoTaiKhoan a directory entry becomes an account, without DangHoatDong a
	// locked-out former employee gets back in.
	CoTaiKhoan   bool
	DangHoatDong bool
}

// CanBoTomTat is one row of the commune's staff register as the Cấu hình → Người dùng screen
// and the staff directory read it (docs/ui-ux/14-cau-hinh.md §3, 12-danh-ba-can-bo.md).
//
// WHY A SECOND TYPE NEXT TO CanBo INSTEAD OF REUSING IT: CanBo carries MatKhauHash, because the
// sign-in path has to compare against it. This type HAS NO PASSWORD FIELD AT ALL, and that
// absence is the guarantee — a credential that is never read cannot be returned by accident,
// while a field merely left empty by convention is one careless SELECT away from being filled
// again, and the read path that would fill it is the one nobody re-reads.
//
// DienThoai IS HELD RAW HERE, and must be masked before it leaves the API (rule 3, invariant 3).
// The masking lives at the edge, in internal/http, because "what a caller is allowed to see" is
// a question about the caller — not about the record. Masking in the store would also mask it
// for a future internal consumer that has a legitimate need (sending an SMS, for instance), and
// a store that hands back an unusable value invites somebody to add a second, unmasked read path.
type CanBoTomTat struct {
	ID     string // ULID, internal — the value the {id} route segment carries
	Ma     string // business code, the one the audit trail shows
	HoTen  string
	Email  string
	ChucVu string

	// Ids, not names. Resolving them to "LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ" is a join, and the paged
	// read walks ONE table (see pkg/store.QueryPage). The screen already loads the department and
	// role lists for its own filter boxes, so it resolves the label without a second round trip.
	BoPhanID string
	VaiTroID string

	DienThoai string // RAW. Masked at the edge — see the note above.

	// The two flags answer two different questions; see CanBo for why they are not one column.
	// BOTH are returned, unfiltered, because ONE table serves TWO screens: the directory shows
	// everybody, Cấu hình → Người dùng shows accounts. Choosing one here would be deciding, on
	// behalf of a screen, a question nobody asked.
	CoTaiKhoan   bool
	DangHoatDong bool

	// Nil means "Chưa đăng nhập" — never signed in. A zero time.Time would render as year 1 in
	// whatever the browser does with it, and the screen has a distinct label for this case.
	DangNhapGanNhat *time.Time

	TaoLuc time.Time
}
