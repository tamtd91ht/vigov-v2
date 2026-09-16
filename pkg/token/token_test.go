package token

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/pkg/secret"
	"github.com/vihat/vigov/pkg/tenant"
)

// Fake key material. Never a real key in source (rule 8, forbidden #1); these strings say so
// in their own text so a scanner and a human both read them the same way.
//
// secret.Secret and not []byte: that is the type NewSigner takes now, so the tests hold the
// material the same way production does.
var (
	khoaMoi = secret.Secret("khoa-ky-gia-KHONG-PHAI-KHOA-THAT-moi-01")
	khoaCu  = secret.Secret("khoa-ky-gia-KHONG-PHAI-KHOA-THAT-cu-002")
	khoaLa  = secret.Secret("khoa-ky-gia-KHONG-PHAI-KHOA-THAT-ngoai1")
)

// xaGia builds a well-formed commune id: tenant.ID.Valid() requires ULID length.
func xaGia(dau string) tenant.ID {
	return tenant.ID(dau + strings.Repeat("A", 26-len(dau)))
}

func kyDuoc(t *testing.T, s *Signer, c Claims) string {
	t.Helper()
	tok, err := s.Ky(c)
	if err != nil {
		t.Fatalf("Ky lỗi: %v", err)
	}
	return tok
}

func claimMau() Claims {
	return Claims{
		TenantID:  xaGia("01JX"),
		Sid:       "phien-gia-0001",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
}

func TestKyRoiGiaiLaiDuoc(t *testing.T) {
	s, err := NewSigner([]secret.Secret{khoaMoi})
	if err != nil {
		t.Fatalf("NewSigner lỗi: %v", err)
	}
	muon := claimMau()

	got, err := s.Giai(kyDuoc(t, s, muon))
	if err != nil {
		t.Fatalf("Giai lỗi: %v", err)
	}
	if got.TenantID != muon.TenantID {
		t.Errorf("TenantID = %q, muốn %q", got.TenantID, muon.TenantID)
	}
	if got.Sid != muon.Sid {
		t.Errorf("Sid = %q, muốn %q", got.Sid, muon.Sid)
	}
	if !got.ExpiresAt.Equal(muon.ExpiresAt.Truncate(time.Second)) {
		t.Errorf("ExpiresAt = %v, muốn %v", got.ExpiresAt, muon.ExpiresAt.Truncate(time.Second))
	}
}

func TestTokenKhongChuaVaiTro(t *testing.T) {
	// The claim set is three fields and must stay three. Embedding a role means a privilege
	// change takes effect only when the token expires — up to 12 hours of authority somebody
	// has already had removed (skills/session-and-token, FORBIDDEN).
	s, _ := NewSigner([]secret.Secret{khoaMoi})
	tok := kyDuoc(t, s, claimMau())

	than := tok[:strings.LastIndexByte(tok, '.')]
	tho := giaiB64(t, than[len(Version)+1:])
	for _, cam := range []string{"role", "vai_tro", "perm", "quyen"} {
		if strings.Contains(strings.ToLower(tho), cam) {
			t.Errorf("token chứa %q — vai trò/quyền không được nằm trong token", cam)
		}
	}
}

func TestKhoaCuTrongDanhSachVanXacMinhDuoc(t *testing.T) {
	// The whole reason the key is a LIST: rotating it must not sign out every member of staff
	// of every commune at once (rule 8, invariant 6).
	cu, err := NewSigner([]secret.Secret{khoaCu})
	if err != nil {
		t.Fatalf("NewSigner lỗi: %v", err)
	}
	tok := kyDuoc(t, cu, claimMau())

	// After rotation the new key signs, the old key still verifies.
	sau, err := NewSigner([]secret.Secret{khoaMoi, khoaCu})
	if err != nil {
		t.Fatalf("NewSigner lỗi: %v", err)
	}
	if _, err := sau.Giai(tok); err != nil {
		t.Fatalf("token ký bằng khoá cũ phải còn xác minh được, nhận %v", err)
	}

	// And a token signed after rotation uses the FIRST key, so dropping the old one later
	// invalidates only tokens older than one session lifetime.
	chiKhoaMoi, _ := NewSigner([]secret.Secret{khoaMoi})
	if _, err := chiKhoaMoi.Giai(kyDuoc(t, sau, claimMau())); err != nil {
		t.Errorf("token mới phải ký bằng khoá đầu danh sách, nhận %v", err)
	}
}

func TestKhoaNgoaiDanhSachThiKhongXacMinhDuoc(t *testing.T) {
	la, _ := NewSigner([]secret.Secret{khoaLa})
	tok := kyDuoc(t, la, claimMau())

	s, _ := NewSigner([]secret.Secret{khoaMoi, khoaCu})
	if _, err := s.Giai(tok); !errors.Is(err, ErrKhongHopLe) {
		t.Fatalf("muốn ErrKhongHopLe cho token ký bằng khoá lạ, nhận %v", err)
	}
}

func TestTokenHongThiTuChoi(t *testing.T) {
	s, _ := NewSigner([]secret.Secret{khoaMoi})
	hopLe := kyDuoc(t, s, claimMau())
	i := strings.LastIndexByte(hopLe, '.')

	cases := []struct {
		ten string
		tok string
	}{
		{"rỗng", ""},
		{"không có dấu chấm", "khongcodauchamnaoca"},
		{"sai chữ ký", hopLe[:i+1] + "AAAA"},
		{"sửa payload", "v1.AAAA." + hopLe[i+1:]},
		{"sai phiên bản", "v9" + hopLe[2:]},
		{"chỉ có thân", hopLe[:i]},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if _, err := s.Giai(c.tok); !errors.Is(err, ErrKhongHopLe) {
				t.Errorf("muốn ErrKhongHopLe, nhận %v", err)
			}
		})
	}
}

func TestTokenHetHanThiTuChoi(t *testing.T) {
	s, _ := NewSigner([]secret.Secret{khoaMoi})
	c := claimMau()
	c.ExpiresAt = time.Now().UTC().Add(-time.Second)

	if _, err := s.Giai(kyDuoc(t, s, c)); !errors.Is(err, ErrHetHan) {
		t.Fatalf("muốn ErrHetHan, nhận %v", err)
	}
}

func TestKyThieuClaimThiTuChoi(t *testing.T) {
	s, _ := NewSigner([]secret.Secret{khoaMoi})
	cases := map[string]Claims{
		"thiếu xã":       {Sid: "phien-gia-0001", ExpiresAt: time.Now().Add(time.Hour)},
		"thiếu sid":      {TenantID: xaGia("01JX"), ExpiresAt: time.Now().Add(time.Hour)},
		"không hạn dùng": {TenantID: xaGia("01JX"), Sid: "phien-gia-0001"},
	}
	for ten, c := range cases {
		t.Run(ten, func(t *testing.T) {
			if _, err := s.Ky(c); !errors.Is(err, ErrThieuClaim) {
				t.Errorf("muốn ErrThieuClaim, nhận %v", err)
			}
		})
	}
}

func TestSignerTuChoiKhoaKhongDung(t *testing.T) {
	// Fail closed at startup: no key means forged tokens are indistinguishable from real ones,
	// and a short key means the signature can be attacked offline.
	if _, err := NewSigner(nil); !errors.Is(err, ErrKhongCoKhoa) {
		t.Errorf("muốn ErrKhongCoKhoa, nhận %v", err)
	}
	if _, err := NewSigner([]secret.Secret{[]byte("qua-ngan")}); !errors.Is(err, ErrKhoaQuaNgan) {
		t.Errorf("muốn ErrKhoaQuaNgan, nhận %v", err)
	}
}

func TestSignerKhongGiuThamChieuKhoaGoc(t *testing.T) {
	// A caller reusing its buffer must not silently change what this signer trusts.
	khoa := append([]byte(nil), khoaMoi...)
	s, _ := NewSigner([]secret.Secret{khoa})
	tok := kyDuoc(t, s, claimMau())

	for i := range khoa {
		khoa[i] = 'x'
	}
	if _, err := s.Giai(tok); err != nil {
		t.Fatalf("signer phải giữ bản sao khoá, nhận %v", err)
	}
}

func giaiB64(t *testing.T, s string) string {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("giải base64: %v", err)
	}
	return string(b)
}
