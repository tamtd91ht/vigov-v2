package domain

// The shape rules of a staff password, and the generator behind the temporary one.
//
// THIS FILE IMPORTS NOTHING BUT THE STANDARD LIBRARY (rule 4 of the service pattern). It knows
// nothing about argon2, about SQL or about HTTP: what it owns is "is this string a thing we are
// willing to turn into a credential", which has to be answered BEFORE a transaction opens.
//
// NOTHING HERE EVER RETURNS, LOGS OR FORMATS THE VALUE IT WAS GIVEN. A password is not merely
// personal data, it IS the credential (rule 3, rule 8): an error message quoting it puts it in the
// response body, in the browser console and in whatever the client logs. Every sentence below names
// the RULE and never the input.
//
// OPEN QUESTION #9, decided 2026-09-22: the system MINTS a temporary password and the person must
// change it at the first sign-in. #17, the same day: there is NO self-service reset by e-mail — an
// administrator of the commune resets it for them. Both flows mint a value with SinhMatKhauTam, and
// both leave `phai_doi_mat_khau` true.

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// DaiMatKhauToiThieu is the shortest password a person may choose.
//
// IT MUST EQUAL core/password.DaiToiThieu, AND IT IS NOT ALLOWED TO IMPORT IT. This package may
// only reach for the standard library, so the two constants are two literals — and two literals
// are two values that drift. What holds them equal is a test, app.TestDaiMatKhauKhopVoiCorePassword,
// which fails the moment either moves.
//
// THE DIRECTION OF THE DRIFT THAT MATTERS: if this one ever became the SMALLER of the two, this
// file would accept a password that core/password.Bam then refuses — INSIDE the transaction,
// after the guards have run, reaching a member of staff as an internal error on a screen that
// told them their input was fine.
//
// Length beats composition rules, which is the argument core/password already makes: demanding a
// symbol produces "P@ssw0rd!" and a note stuck to the monitor, while length is what actually costs
// an attacker time.
const DaiMatKhauToiThieu = 12

// DaiMatKhauToiDa bounds what is accepted, and the bound is about the SERVER rather than about
// good passwords.
//
// argon2id hashes a 4 KiB input in about the time it hashes a 20-byte one — the cost is in the
// memory parameter, not in the length — so this is not protection against a slow hash. It is
// protection against a field with no ceiling on a process serving 200+ communes: 512 characters is
// far past any passphrase a person types and far short of anything worth streaming.
const DaiMatKhauToiDa = 512

// The refusals this file owns. Sentinels, so the use case decides the policy and the handler
// decides the status code.
var (
	ErrThieuMatKhau      = errors.New("mat_khau: chưa nhập mật khẩu")
	ErrMatKhauQuaNgan    = fmt.Errorf("mat_khau: mật khẩu phải có ít nhất %d ký tự", DaiMatKhauToiThieu)
	ErrMatKhauQuaDai     = fmt.Errorf("mat_khau: mật khẩu không được quá %d ký tự", DaiMatKhauToiDa)
	ErrMatKhauKhongDoc   = errors.New("mat_khau: mật khẩu chứa ký tự không đọc được")
	ErrMatKhauMoiTrungCu = errors.New("mat_khau: mật khẩu mới phải khác mật khẩu hiện tại")
)

// KiemTraMatKhauMoi refuses a new password this system will not store.
//
// IT DOES NOT NORMALISE, AND THE NAME SAYS SO ON PURPOSE. Every other write path in this service
// goes through a `ChuanHoa...` that trims and collapses whitespace; doing that to a password would
// CHANGE THE CREDENTIAL silently — the person types a trailing space, the server stores the value
// without it, and the sign-in they try next fails against a password nobody mistyped. A password is
// stored exactly as it was typed or it is refused.
//
// THE ONE THING IT LOOKS AT BESIDES LENGTH is whether the string is valid UTF-8. An invalid
// sequence is refused by PostgreSQL on the way in, and the failure would arrive INSIDE the
// transaction that also writes the audit entry (rule 6, invariant 3) — so the whole operation rolls
// back and reaches a person as "an error occurred" rather than as "this is not a usable password".
//
// COUNTED IN RUNES, NOT BYTES. A Vietnamese passphrase is two to three bytes per character, so a
// byte count would refuse "mậtkhẩucủatôi" for being too short in exactly the wrong direction.
func KiemTraMatKhauMoi(moi string) error {
	switch {
	case moi == "":
		return ErrThieuMatKhau
	case !utf8.ValidString(moi):
		return ErrMatKhauKhongDoc
	case utf8.RuneCountInString(moi) < DaiMatKhauToiThieu:
		return ErrMatKhauQuaNgan
	case utf8.RuneCountInString(moi) > DaiMatKhauToiDa:
		return ErrMatKhauQuaDai
	}
	return nil
}

// soKyTuMatKhauTam — 16 characters of the alphabet below, i.e. 80 bits.
//
// THE SIZE IS CHOSEN AGAINST AN ATTACKER, WHICH IS THE OPPOSITE OF WHY SinhMaCanBo CHOSE SIX. A
// staff code is a label, printed on a screen and quoted in the audit trail, and its randomness only
// buys independence from the table. This value IS a working credential for an account of a
// government system: until the person changes it, guessing it is signing in as them, and every
// action they then take is recorded under that person's name (rule 6, invariant 2).
//
// 80 bits also survives the thing that makes a temporary password different from a session id: it
// is read out loud, written on a slip of paper, and lives for as long as it takes somebody to come
// back to their desk. The margin is there so a shortened or partly-overheard value is still not a
// guess anybody completes.
const soKyTuMatKhauTam = 16

// nhomMatKhauTam groups the characters for reading aloud: `7K3M-9QRT-...`.
//
// THE HYPHENS ARE PART OF THE PASSWORD, not decoration a caller may strip. They are what makes an
// administrator able to dictate the value down a telephone line without losing their place, which
// is the ONLY channel #9 leaves: there is no activation e-mail, because a commune's mail server is
// per-commune configuration and may not be filled in at all.
const nhomMatKhauTam = 4

// SinhMatKhauTam mints one temporary password, of the form `7K3M-9QRT-B4XZ-V2NP`.
//
// IT IS RETURNED TO THE CALLER AND TO NOBODY ELSE, AND IT IS NEVER STORED. What reaches the
// database is the argon2id hash of it; the plaintext exists in one HTTP response and in the memory
// of the administrator reading it out. That is the whole point of #9: after the person changes it,
// the administrator does not know anybody's password, which is what rule 6, invariant 2 needs in
// order for "who did this" to mean anything.
//
// THE ALPHABET IS chuCaiMaCanBo — Crockford's base32, no I, L, O or U — and reusing it here is a
// decision rather than a convenience. A value that turns into a DIFFERENT VALID value when read
// aloud is worse for a password than for a code: the staff code merely names the wrong person on a
// screen, while a mistyped password locks somebody out of a government system and sends them back
// to the administrator who cannot tell them what it was.
//
// EVERY CHARACTER IS UNBIASED, and that is a property of the arithmetic rather than a hope: 32
// divides 256 exactly, so masking a uniform byte to its low 5 bits leaves each of the 32 characters
// with probability 8/256. Rejection sampling would be needed for an alphabet whose size does not
// divide 256, and writing `v % len(alphabet)` there is the classic way to make the first few
// characters more likely.
//
// AN ERROR IS FATAL TO THE CALLER'S WRITE AND MUST NOT BE SWALLOWED. crypto/rand failing means the
// operating system's entropy source is unavailable; carrying on with anything predictable would
// mint credentials somebody can guess. Fail closed.
func SinhMatKhauTam() (string, error) {
	var b [soKyTuMatKhauTam]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("mat_khau: sinh mật khẩu tạm: %w", err)
	}

	var ra strings.Builder
	ra.Grow(soKyTuMatKhauTam + soKyTuMatKhauTam/nhomMatKhauTam)
	for i, v := range b {
		if i > 0 && i%nhomMatKhauTam == 0 {
			ra.WriteByte('-')
		}
		ra.WriteByte(chuCaiMaCanBo[v&31])
	}
	return ra.String(), nil
}
