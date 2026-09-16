package domain

// CanBo is a staff account.
//
// Staff identity is ACCOUNTABLE: every action traces back to a person (rule 4). That is the
// opposite of citizen identity — a phone number plus an OTP — which is deliberately weak and
// never enough on its own for an act with legal consequences. The two never share a type.
type CanBo struct {
	ID           string // ULID, internal
	Ma           string // business code, the one that appears in the audit trail
	HoTen        string
	Email        string
	ChucVu       string
	BoPhanID     string
	VaiTroID     string
	DienThoai    string
	MatKhauHash  string // argon2id. NEVER the password, never logged (rule 3, rule 8)
	DangHoatDong bool
}
