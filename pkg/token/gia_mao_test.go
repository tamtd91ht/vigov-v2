package token

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// Forgery and expiry. THE DEFECT CLASS THESE COVER: a token that can be ALTERED and still
// accepted. Nothing reports it — the request looks ordinary, the response is a 200, and the
// only trace is that somebody acted as a commune or a session they were never issued. The
// existing tests check a handful of malformed shapes; these check that NO alteration passes.
//
// Companion file: token_test.go covers the happy path, key rotation and the key-length floor.

// tachToken splits a token into its body ("v1.<payload>") and its base64 signature.
func tachToken(t *testing.T, tok string) (than, chuKy string) {
	t.Helper()
	i := strings.LastIndexByte(tok, '.')
	if i <= 0 {
		t.Fatalf("token không đúng dạng: %q", tok)
	}
	return tok[:i], tok[i+1:]
}

func TestSuaMotBitTrongThanTokenLuonBiTuChoi(t *testing.T) {
	// Every byte of the body, every bit that matters: the commune, the sid and the expiry all
	// live in there. One accepted alteration is one request acting as another commune.
	s, _ := NewSigner([][]byte{khoaMoi})
	tok := kyDuoc(t, s, claimMau())
	than, _ := tachToken(t, tok)

	for i := 0; i < len(than); i++ {
		for _, bit := range []byte{0x01, 0x02, 0x10, 0x40} {
			hong := []byte(tok)
			hong[i] ^= bit
			if string(hong) == tok {
				continue
			}
			if _, err := s.Giai(string(hong)); !errors.Is(err, ErrKhongHopLe) {
				t.Fatalf("sửa byte %d (bit %#x) mà vẫn qua: err = %v, token = %q",
					i, bit, err, string(hong))
			}
		}
	}
}

func TestSuaChuKyLuonBiTuChoi(t *testing.T) {
	// hmac.Equal compares the whole value in constant time. A prefix comparison, or a length
	// mismatch treated as "close enough", would accept a truncated MAC — which is a forgery
	// that costs an attacker 2^8 tries, not 2^256.
	s, _ := NewSigner([][]byte{khoaMoi})
	tok := kyDuoc(t, s, claimMau())
	than, chuKy := tachToken(t, tok)

	mac, err := base64.RawURLEncoding.DecodeString(chuKy)
	if err != nil {
		t.Fatalf("chữ ký không phải base64url: %v", err)
	}
	if len(mac) != 32 {
		t.Fatalf("chữ ký dài %d byte, muốn 32 (HMAC-SHA256)", len(mac))
	}

	var bien []string
	for i := range mac {
		hong := append([]byte(nil), mac...)
		hong[i] ^= 0x01
		bien = append(bien, base64.RawURLEncoding.EncodeToString(hong))
	}
	bien = append(bien,
		"", // no signature at all
		base64.RawURLEncoding.EncodeToString(mac[:16]),          // truncated
		base64.RawURLEncoding.EncodeToString(append(mac, 0x00)), // padded
		base64.RawURLEncoding.EncodeToString(make([]byte, 32)),  // all zero
		"khong-phai-base64-vi-co-dau-gach!!",                    // not base64
		strings.Repeat("A", 43),                                 // right length, wrong value
	)
	for _, ck := range bien {
		if _, err := s.Giai(than + "." + ck); !errors.Is(err, ErrKhongHopLe) {
			t.Fatalf("chữ ký %q được chấp nhận: err = %v", ck, err)
		}
	}
}

func TestDoiXaRoiKyLaiBangKhoaNgoaiDanhSachThiBiTuChoi(t *testing.T) {
	// The shape of a real cross-commune attempt: take a valid token, rewrite `tid` to another
	// commune, re-sign with whatever key the attacker has. It must fail at the signature, before
	// the commune is ever read — otherwise commune B serves a token it never issued.
	that, _ := NewSigner([][]byte{khoaMoi})
	la, _ := NewSigner([][]byte{khoaLa})

	c := claimMau()
	c.TenantID = xaGia("01JB")
	gia := kyDuoc(t, la, c)

	// The test would be vacuous if the two communes happened to be equal.
	if c.TenantID == claimMau().TenantID {
		t.Fatal("hai xã trong test phải khác nhau")
	}
	if _, err := that.Giai(gia); !errors.Is(err, ErrKhongHopLe) {
		t.Fatalf("token ký bằng khoá ngoài danh sách phải bị từ chối, nhận %v", err)
	}
}

func TestGhepThanTokenNayVoiChuKyTokenKia(t *testing.T) {
	// Two legitimate tokens of two communes, signed by the SAME key. Swapping their halves must
	// not produce a third valid token — the MAC covers the body, so it cannot be transplanted.
	s, _ := NewSigner([][]byte{khoaMoi})

	cA := claimMau()
	cB := claimMau()
	cB.TenantID = xaGia("01JB")
	cB.Sid = "phien-gia-0002"

	tokA := kyDuoc(t, s, cA)
	tokB := kyDuoc(t, s, cB)
	thanA, _ := tachToken(t, tokA)
	_, chuKyB := tachToken(t, tokB)

	if _, err := s.Giai(thanA + "." + chuKyB); !errors.Is(err, ErrKhongHopLe) {
		t.Fatalf("thân xã A + chữ ký xã B phải bị từ chối, nhận %v", err)
	}
}

func TestCatCutVaThemDauChamDeuBiTuChoi(t *testing.T) {
	s, _ := NewSigner([][]byte{khoaMoi})
	tok := kyDuoc(t, s, claimMau())
	than, chuKy := tachToken(t, tok)

	// Every prefix of a valid token. A split done on the FIRST dot instead of the last, or an
	// index computed one byte off, shows up here and nowhere else.
	for i := 0; i < len(tok); i++ {
		if _, err := s.Giai(tok[:i]); err == nil {
			t.Fatalf("token cắt còn %d ký tự vẫn được chấp nhận", i)
		}
	}

	cases := map[string]string{
		"thêm dấu chấm cuối":     tok + ".",
		"thêm dấu chấm đầu":      "." + tok,
		"nhân đôi dấu chấm giữa": than + ".." + chuKy,
		"chèn đoạn giữa":         than + ".chen-them." + chuKy,
		"chỉ có dấu chấm":        ".",
		"phiên bản rỗng":         tok[2:],
		"phiên bản khác":         "v2" + tok[2:],
		"khoảng trắng đầu":       " " + tok,
		"khoảng trắng cuối":      tok + " ",
		"chữ hoa phiên bản":      "V1" + tok[2:],
	}
	for ten, hong := range cases {
		if _, err := s.Giai(hong); !errors.Is(err, ErrKhongHopLe) {
			t.Errorf("%s: phải bị từ chối, nhận %v", ten, err)
		}
	}
}

// kyThan signs an arbitrary body with the test key. It exists to reach the code path AFTER the
// signature check — the parser — which no external attacker can reach without the key, and
// which must therefore still refuse to panic or to accept an incomplete claim set.
func kyThan(s *Signer, than string) string {
	return than + "." + base64.RawURLEncoding.EncodeToString(s.mac(s.khoa[0], than))
}

func TestThanDungChuKyNhungPayloadHongThiTuChoiChuKhongPanic(t *testing.T) {
	s, _ := NewSigner([][]byte{khoaMoi})

	b64 := func(v string) string { return base64.RawURLEncoding.EncodeToString([]byte(v)) }

	cases := map[string]string{
		"payload không phải base64": Version + ".khong-phai-base64!!!",
		"payload rỗng":              Version + ".",
		"payload không phải JSON":   Version + "." + b64("khong-phai-json"),
		"JSON nhưng là mảng":        Version + "." + b64(`["tid","sid"]`),
		"thiếu tid":                 Version + "." + b64(`{"sid":"s","exp":99999999999}`),
		"thiếu sid":                 Version + "." + b64(`{"tid":"01JAAAAAAAAAAAAAAAAAAAAAAA","exp":99999999999}`),
		"exp bằng 0":                Version + "." + b64(`{"tid":"01JAAAAAAAAAAAAAAAAAAAAAAA","sid":"s","exp":0}`),
		"tid rỗng":                  Version + "." + b64(`{"tid":"","sid":"s","exp":99999999999}`),
	}
	for ten, than := range cases {
		t.Run(ten, func(t *testing.T) {
			if _, err := s.Giai(kyThan(s, than)); !errors.Is(err, ErrKhongHopLe) {
				t.Errorf("phải bị từ chối, nhận %v", err)
			}
		})
	}
}

func TestExpOBienVaTrongQuaKhu(t *testing.T) {
	// There is deliberately NO grace window: a token one second past its expiry is refused. A
	// leeway added "for clock skew" is a leeway an attacker also gets, on a credential whose
	// whole purpose is to stop working at a known moment.
	s, _ := NewSigner([][]byte{khoaMoi})
	bayGio := time.Now().UTC()

	hetHan := map[string]time.Time{
		"đúng thời điểm hết hạn": bayGio.Truncate(time.Second),
		"quá hạn 1 giây":         bayGio.Add(-time.Second),
		"quá hạn rất lâu":        time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for ten, exp := range hetHan {
		t.Run(ten, func(t *testing.T) {
			c := claimMau()
			c.ExpiresAt = exp
			if _, err := s.Giai(kyDuoc(t, s, c)); !errors.Is(err, ErrHetHan) {
				t.Errorf("muốn ErrHetHan, nhận %v", err)
			}
		})
	}

	// And the other side of the boundary, so the check is not simply refusing everything.
	c := claimMau()
	c.ExpiresAt = bayGio.Add(5 * time.Second)
	if _, err := s.Giai(kyDuoc(t, s, c)); err != nil {
		t.Errorf("token còn 5 giây phải dùng được, nhận %v", err)
	}
}

func TestExpAmBiCoiLaHetHan(t *testing.T) {
	// A negative exp is not a valid instant a signer would ever produce; it must land in the
	// expired branch rather than sliding through as "not zero, therefore fine".
	s, _ := NewSigner([][]byte{khoaMoi})
	than := Version + "." + base64.RawURLEncoding.EncodeToString(
		[]byte(`{"tid":"01JAAAAAAAAAAAAAAAAAAAAAAA","sid":"phien-gia-0001","exp":-1}`))

	if _, err := s.Giai(kyThan(s, than)); !errors.Is(err, ErrHetHan) {
		t.Errorf("exp âm phải bị coi là hết hạn, nhận %v", err)
	}
}

func TestSignerRongTuChoiMoiToken(t *testing.T) {
	// Fail closed. A Signer built by struct literal instead of NewSigner has no keys; it must
	// verify nothing rather than verify everything.
	that, _ := NewSigner([][]byte{khoaMoi})
	tok := kyDuoc(t, that, claimMau())

	var rong Signer
	if _, err := rong.Giai(tok); !errors.Is(err, ErrKhongHopLe) {
		t.Fatalf("signer không có khoá phải từ chối mọi token, nhận %v", err)
	}
	if rong.SoKhoa() != 0 {
		t.Errorf("SoKhoa = %d, muốn 0", rong.SoKhoa())
	}
}

func TestPayloadChiCoDungBaTruong(t *testing.T) {
	// The claim set is three fields and must stay three. A fourth — an `alg`, a role, a name —
	// is either a forgery surface or a privilege cached past its revocation. This fails the
	// moment somebody adds one, which is the only moment it is cheap to discuss.
	s, _ := NewSigner([][]byte{khoaMoi})
	than, _ := tachToken(t, kyDuoc(t, s, claimMau()))

	tho, err := base64.RawURLEncoding.DecodeString(than[len(Version)+1:])
	if err != nil {
		t.Fatal(err)
	}
	var truong map[string]any
	if err := json.Unmarshal(tho, &truong); err != nil {
		t.Fatal(err)
	}
	muon := map[string]bool{"tid": true, "sid": true, "exp": true}
	for k := range truong {
		if !muon[k] {
			t.Errorf("payload có trường lạ %q — claim set phải đúng ba trường", k)
		}
	}
	if len(truong) != len(muon) {
		t.Errorf("payload có %d trường, muốn %d: %v", len(truong), len(muon), truong)
	}
}
