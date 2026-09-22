package domain

import (
	"crypto/rand"
	"fmt"
	"time"
)

// The staff code — `nguoi_dung.ma`, the value the audit trail quotes.
//
// WHO MINTS IT: the system. Open question #15, decided 2026-09-22. Neither form in the
// specification draws a Mã box (docs/ui-ux/12-danh-ba-can-bo.md §5, 14-cau-hinh.md:79), and as of
// that decision the absence is a decision rather than an omission — there is no "the commune
// types its own code" path to build, and no second numbering system for a commune to remember.
//
// THREE REQUIREMENTS, AND THEY PULL AGAINST EACH OTHER. The shape below is what satisfies all
// three; each one on its own has an easier answer that breaks another.
//
//  1. NEVER REISSUED, INCLUDING AFTER A SOFT DELETE (rule 7, invariant 3). The code names the
//     person who handled an administrative file; handing a departed person's code to a new
//     arrival makes two people indistinguishable in a record that is kept for years.
//  2. READABLE BY A PERSON. #15's own reasoning: whoever reads the audit trail a year later has
//     to be able to look the person up from what they see. This is what rules out a ULID, which
//     satisfies every database constraint and fails exactly this.
//  3. UNIQUE WITHIN A COMMUNE, and only within one. `UNIQUE (tenant_id, ma)` is composite
//     (rule 1, invariant 6), so two communes may hold the same code and never collide — which
//     they will, and must be allowed to.
//
// HOW REQUIREMENT 1 IS ACTUALLY GUARANTEED — it is NOT this function on its own, and pretending
// otherwise is how the guarantee gets lost:
//
//	THE GENERATOR'S HALF   the value is read from crypto/rand. It is never counted, never
//	                       derived from MAX(ma), never taken from a sequence. That matters
//	                       specifically because of soft delete: EVERY counter hands the departed
//	                       person's code to the next arrival the moment their row stops being
//	                       counted, and a soft-deleted row is precisely a row that drops out of
//	                       most counts.
//	THE DATABASE'S HALF    `UNIQUE (tenant_id, ma)` (migration 0001) carries NO
//	                       `WHERE deleted_at IS NULL`. A soft-deleted person keeps their code
//	                       occupied for good, and an INSERT proposing it is refused by the
//	                       server. The caller retries with a fresh code; it does not "find a free
//	                       one", because looking one up and then inserting it is two statements
//	                       with a gap in between, and the gap is where two staff members created
//	                       at once get the same code.
//
// Pinned by internal/store/ma_can_bo_pg_test.go, which asserts the refusal on a real server and
// asserts that the same code IS accepted in a second commune.

// tienToMaCanBo — "cán bộ". The prefix is there so the value is self-describing in a log line, a
// support conversation or an audit entry read out of context, where "2026-7K3M9Q" alone says
// nothing about what kind of thing it names.
const tienToMaCanBo = "CB"

// soKyTuNgauNhien — 6 characters of the alphabet below, i.e. 30 bits, about 1.07e9 values.
//
// THE SIZE IS CHOSEN AGAINST READABILITY, NOT AGAINST AN ATTACKER, and that distinction is worth
// keeping straight. A staff code is not a secret and must never be used as one: it is shown on
// the detail screen and quoted in the audit trail. What the randomness buys is requirement 1 —
// that the value owes nothing to the contents of the table. Anything that must not be GUESSED —
// a session id, a citizen's lookup code — uses 256 bits of pure randomness instead (rule 4,
// invariant 4); see internal/store/phien.go maNgauNhien.
//
// At the specification's ceiling of a few tens of staff per commune per year, a collision is
// vanishingly unlikely, and it is not relied upon in any case: the unique key refuses it and the
// caller generates again.
const soKyTuNgauNhien = 6

// chuCaiMaCanBo is Crockford's base32 alphabet — no I, L, O or U, so a code read aloud over the
// telephone, or transcribed off a screen, cannot turn into a DIFFERENT VALID code.
//
// IT IS THE SAME ALPHABET AS core/ulid, AND THAT IS DELIBERATE RATHER THAN COPIED BY ACCIDENT.
// core/ulid keeps its own copy unexported, and importing it here would be wrong in a way that
// matters later: a staff code is not a ULID, must not become one (requirement 2), and must not
// inherit the length or layout of one. What is shared is the property — a value a person reads
// back to somebody over the phone — and for that property this is the right alphabet in both
// places. If either ever changes, they are allowed to differ.
const chuCaiMaCanBo = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// SinhMaCanBo mints one candidate staff code, of the form `CB-2026-7K3M9Q`.
//
// CANDIDATE, NOT ALLOCATION. This function does not know what the table contains and deliberately
// never asks. The caller inserts the row and, on a unique violation of
// `UNIQUE (tenant_id, ma)`, calls again. That is the only ordering in which the code a row ends
// up with is the code the database agreed to — see the note on the two halves above.
//
// `luc` IS THE CALLER'S CLOCK, NOT time.Now(), for two reasons: a test can pin it, and the year
// that ends up in the code is the commune's local year rather than whatever zone the server
// happens to run in. The year is A READING AID AND NOTHING ELSE — nothing parses it, no query
// filters on it, and a code minted seven hours either side of midnight on 1 January is not wrong,
// merely less helpful. Never derive a fact from it.
//
// AN ERROR IS RETURNED RATHER THAN SWALLOWED, and the caller must treat it as fatal to the write.
// crypto/rand failing means the operating system's entropy source is unavailable; carrying on
// with a predictable value would start minting codes that owe something to the clock or to the
// previous call, which is the one property this whole file exists to avoid. Fail closed.
func SinhMaCanBo(luc time.Time) (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("can_bo: sinh mã ngẫu nhiên: %w", err)
	}

	// 32 bits read, the top 2 discarded: 6 characters × 5 bits = 30.
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])

	var duoi [soKyTuNgauNhien]byte
	for i := soKyTuNgauNhien - 1; i >= 0; i-- {
		duoi[i] = chuCaiMaCanBo[v&31]
		v >>= 5
	}

	return fmt.Sprintf("%s-%04d-%s", tienToMaCanBo, luc.Year(), duoi), nil
}
