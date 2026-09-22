// Package ulid generates the opaque row identifier every business table in this system uses.
//
// WHY IT LIVES IN core/ AND NOT BESIDE ITS FIRST CALLER. There were two hand-written copies
// before this package existed — service-identity/internal/store/crosstenant/dinh_danh_cong_dan.go
// and service-comms/internal/app/so_thong_bao.go — and the second one says in its own comment that
// moving it to core/ is the right call "on the day a second caller appears". Five catalogue write
// paths arriving at once is that day several times over.
//
// The reason the duplication is dangerous rather than merely untidy: a ULID that differs in
// ALPHABET or in LENGTH between two services is an id that passes in one service and fails a CHECK
// or a length assumption in another — and it fails on the row that is already written, in a system
// where rows are archival records that are never rewritten.
//
// THIS IS NOT A SECRET AND MUST NEVER BE USED AS ONE. A ULID leads with a millisecond timestamp,
// so it is guessable by construction: two rows created in the same millisecond differ only in the
// 80 random bits, and the ordering of every row is readable from the id. Anything that must not be
// guessed — a session id, a citizen's lookup code — uses 256 bits of pure randomness instead
// (rule 4, invariant 4). See service-identity/internal/store/phien.go maNgauNhien.
package ulid

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Do is the length of the encoded form. 130 bits of 5-bit groups, exactly 26 characters.
const Do = 26

// chuCai is Crockford's base32 alphabet — no I, L, O or U, so an identifier read aloud or
// transcribed off a screen cannot turn into a DIFFERENT VALID identifier. In a system where the id
// appears in support conversations about a real administrative record, that property is the whole
// reason for choosing this alphabet over plain base32.
const chuCai = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Moi returns a ULID: 48 bits of millisecond timestamp followed by 80 bits of randomness.
//
// AN ERROR IS RETURNED RATHER THAN SWALLOWED, and callers must treat it as fatal to the write.
// crypto/rand failing means the operating system's entropy source is unavailable; continuing with
// a predictable id would hand out row identifiers an outsider can enumerate. Fail closed.
func Moi() (string, error) {
	var b [16]byte

	ms := uint64(time.Now().UTC().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	if _, err := rand.Read(b[6:]); err != nil {
		return "", fmt.Errorf("ulid: sinh mã ngẫu nhiên: %w", err)
	}

	// 128 bits do not divide into 5-bit groups, so what is encoded is a 130-bit field whose two
	// leading bits are zero: 130 / 5 = 26 characters exactly. `du` starts at 2 for those two bits.
	var out [Do]byte
	var goi uint32
	du := uint(2)
	vt := 0
	for _, by := range b {
		goi = goi<<8 | uint32(by)
		du += 8
		for du >= 5 {
			du -= 5
			out[vt] = chuCai[(goi>>du)&31]
			vt++
		}
	}
	return string(out[:]), nil
}
