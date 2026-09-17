// Package password hashes and verifies staff passwords with argon2id.
//
// WHY ARGON2ID AND NOT A FAST HASH: a fast hash (MD5, SHA-256) is designed to be quick, which
// is exactly the wrong property here — it lets a leaked table be brute-forced at millions of
// guesses per second. Argon2id is deliberately slow and memory-hard, so the same leak costs an
// attacker real hardware and real time (skills/session-and-token, required #6).
//
// The parameters are stored INSIDE each hash. When they are raised later, existing hashes
// still verify with the parameters they were made with, and NeedsRehash reports which ones to
// upgrade on the next successful sign-in. Without that, raising the cost would lock out every
// existing account.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Current parameters. OWASP's 2024 baseline for argon2id: 19 MiB, 2 iterations, 1 lane.
const (
	thoiGian uint32 = 2
	boNho    uint32 = 19 * 1024 // KiB
	songSong uint8  = 1
	daiMuoi  uint32 = 16
	daiBam   uint32 = 32
)

var (
	ErrSaiMatKhau    = errors.New("password: mật khẩu không đúng")
	ErrBamKhongHopLe = errors.New("password: chuỗi băm không đọc được")
	ErrMatKhauNgan   = errors.New("password: mật khẩu quá ngắn")
)

// DaiToiThieu is the shortest password accepted.
//
// Length beats composition rules: forcing symbols produces "P@ssw0rd!" and a sticky note,
// while length is what actually costs an attacker time.
const DaiToiThieu = 12

// Bam returns an encoded argon2id hash, in the standard PHC string format so the parameters
// travel with it.
func Bam(matKhau string) (string, error) {
	if len([]rune(matKhau)) < DaiToiThieu {
		return "", fmt.Errorf("%w: cần ít nhất %d ký tự", ErrMatKhauNgan, DaiToiThieu)
	}

	muoi := make([]byte, daiMuoi)
	if _, err := rand.Read(muoi); err != nil {
		return "", fmt.Errorf("password: sinh muối: %w", err)
	}

	bam := argon2.IDKey([]byte(matKhau), muoi, thoiGian, boNho, songSong, daiBam)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, boNho, thoiGian, songSong,
		base64.RawStdEncoding.EncodeToString(muoi),
		base64.RawStdEncoding.EncodeToString(bam)), nil
}

// KiemTra verifies a password against an encoded hash.
//
// The comparison is constant-time: a byte-by-byte comparison that returns early leaks, through
// timing, how much of the hash matched.
func KiemTra(matKhau, maHoa string) error {
	ts, err := phanTich(maHoa)
	if err != nil {
		return err
	}

	tinhLai := argon2.IDKey([]byte(matKhau), ts.muoi, ts.thoiGian, ts.boNho, ts.songSong,
		uint32(len(ts.bam)))

	if subtle.ConstantTimeCompare(tinhLai, ts.bam) != 1 {
		return ErrSaiMatKhau
	}
	return nil
}

// CanBamLai reports whether a hash was made with weaker parameters than the current ones.
//
// Call it after a SUCCESSFUL sign-in — that is the only moment the plaintext is available to
// rehash with. Raising the cost without this would either lock everyone out or leave old
// hashes weak forever.
func CanBamLai(maHoa string) bool {
	ts, err := phanTich(maHoa)
	if err != nil {
		return true // không đọc được thì băm lại
	}
	return ts.thoiGian < thoiGian || ts.boNho < boNho || ts.songSong < songSong
}

type thamSo struct {
	thoiGian uint32
	boNho    uint32
	songSong uint8
	muoi     []byte
	bam      []byte
}

func phanTich(maHoa string) (thamSo, error) {
	phan := strings.Split(maHoa, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", "<muối>", "<băm>"]
	if len(phan) != 6 || phan[1] != "argon2id" {
		return thamSo{}, ErrBamKhongHopLe
	}

	var phienBan int
	if _, err := fmt.Sscanf(phan[2], "v=%d", &phienBan); err != nil {
		return thamSo{}, ErrBamKhongHopLe
	}
	if phienBan != argon2.Version {
		return thamSo{}, fmt.Errorf("%w: phiên bản argon2 %d", ErrBamKhongHopLe, phienBan)
	}

	var ts thamSo
	if _, err := fmt.Sscanf(phan[3], "m=%d,t=%d,p=%d", &ts.boNho, &ts.thoiGian, &ts.songSong); err != nil {
		return thamSo{}, ErrBamKhongHopLe
	}

	muoi, err := base64.RawStdEncoding.Strict().DecodeString(phan[4])
	if err != nil {
		return thamSo{}, ErrBamKhongHopLe
	}
	bam, err := base64.RawStdEncoding.Strict().DecodeString(phan[5])
	if err != nil {
		return thamSo{}, ErrBamKhongHopLe
	}
	ts.muoi, ts.bam = muoi, bam
	return ts, nil
}
