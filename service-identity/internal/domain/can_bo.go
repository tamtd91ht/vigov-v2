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

	// PhaiDoiMatKhau — "this account is still carrying a password somebody else chose".
	//
	// Open question #9, decided 2026-09-22: an administrator mints a TEMPORARY password and the
	// person must change it at the FIRST sign-in. Until they do, a second person knows the
	// credentials of a staff member whose name goes into the audit trail — which is the state
	// rule 6, invariant 2 exists to prevent, and the reason this is not a nicety.
	//
	// It is cleared ONLY by the person themselves setting a new password. An administrator reset
	// sets it back to true, because a reset produces exactly the same situation.
	//
	// DEFAULTS TO true IN THE SCHEMA (migration 0009 §1), which is the opposite direction from
	// CoTaiKhoan below and fail-closed for the same reason: here `true` means LESS authority.
	// A write path that forgets this field mints an account nobody is ever asked to secure.
	//
	// MEANINGLESS WHERE CoTaiKhoan IS FALSE — there is no account to force a change on.
	PhaiDoiMatKhau bool

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
// DienThoai AND DiDong ARE BOTH HELD RAW HERE, and must be masked before they leave the API
// (rule 3, invariant 3) — but NOT by the same rule, and that is the whole reason they are two
// fields. The masking lives at the edge, in internal/http, because "what a caller is allowed to
// see" is a question about the caller — not about the record. Masking in the store would also
// mask it for a future internal consumer that has a legitimate need (sending an SMS, for
// instance), and a store that hands back an unusable value invites somebody to add a second,
// unmasked read path.
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

	// TWO PHONE FIELDS, BECAUSE THEY ARE TWO KINDS OF DATA IN LAW. Open question #16, decided
	// 2026-09-22. Merging them would force ONE masking, export and publication rule onto both,
	// and the level that is right for one is always wrong for the other.
	//
	//	DienThoai  the OFFICE LANDLINE of a public office. Duty information. A commune puts it on
	//	           its own notice board; redacting it protects nobody.
	//	DiDong     the person's OWN MOBILE. PERSONAL DATA under Decree 13/2023/NĐ-CP. Shown
	//	           unmasked to staff of the same commune — they have to ring each other, and
	//	           hiding it just moves the number into a private channel the commune cannot
	//	           audit (#11) — but ALWAYS masked in an Excel export (rule 3, invariant 4), and
	//	           never published to the Mini App without that person's own recorded consent
	//	           (#12).
	//
	// Both RAW here. Masked at the edge — see the note above. Reading one for the other is the
	// mistake this comment exists to prevent: they are adjacent TEXT columns in the SELECT list,
	// and swapping them produces no error anywhere (see store.cotTomTat).
	DienThoai string
	DiDong    string

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

// CanBoVaiTro is one staff member as the INTER-SERVICE contract sees them, and it is
// deliberately the narrowest of the three staff types in this file.
//
// WHY A THIRD TYPE AND NOT CanBoTomTat. CanBoTomTat carries HoTen, DiDong, Email and ChucVu —
// four citizen-grade personal data of a member of staff (rule 3), DiDong being the one Decree
// 13/2023/NĐ-CP names outright since #16 separated it from the office landline. `message Staff` in
// proto/vigov/identity/v1/identity.proto declares NONE of them, so a read path that loads them
// would be a read path that has the values in hand at the moment somebody adds a field to the
// wire type. Having nothing to send is a stronger guarantee than remembering not to send it.
//
// The gap that creates is REAL and is stated on the RPC that returns this — BatchGetStaff in
// service-identity/internal/grpc/server.go. It is not closed here.
type CanBoVaiTro struct {
	ID string // ULID, internal — the value the caller maps its rows by

	// VaiTroMa is the role SLUG (`vai_tro.ma`, "chu-tich-ubnd"), not `vai_tro_id`.
	//
	// "" MEANS THE PERSON HOLDS NO ROLE, and it is an ordinary answer: `nguoi_dung.vai_tro_id`
	// is nullable, and a person can sit in the commune's org chart holding nothing. It is never
	// "we could not find out" — a read failure is an error, never an empty string.
	VaiTroMa string
}
