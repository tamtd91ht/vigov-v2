package domain

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
