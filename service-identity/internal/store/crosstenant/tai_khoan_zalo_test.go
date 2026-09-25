package crosstenant

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
)

// No-database checks. The SQL itself runs in internal/store/cau_phien_pg_test.go, which SKIPS
// without VIGOV_TEST_DSN — everything decidable without a database is decided here.

const maZaloGia = "zalo-user-GIA-0000000000" // obviously fake (rule 3, invariant 5)

func TestKhoaZaloTuChoiTruocKhiChamCSDL(t *testing.T) {
	s := &TaiKhoanZaloStore{} // no handle: any query would panic
	ctx := context.Background()
	for ten, c := range map[string]struct {
		app, ma string
		muon    error
	}{
		"thiếu app":    {"", maZaloGia, ErrThieuAppID},
		"app dấu cách": {"   ", maZaloGia, ErrThieuAppID},
		"thiếu mã":     {"123", "", ErrThieuMaZalo},
		"app quá dài":  {strings.Repeat("1", 200), maZaloGia, ErrKhoaZaloQuaDai},
		"mã quá dài":   {"123", strings.Repeat("9", 200), ErrKhoaZaloQuaDai},
	} {
		t.Run(ten, func(t *testing.T) {
			if _, _, err := s.Doc(ctx, c.app, c.ma); !errors.Is(err, c.muon) {
				t.Errorf("Doc: err = %v, muốn %v", err, c.muon)
			}
			if _, _, err := s.TimHoacTao(ctx, nil, c.app, c.ma); !errors.Is(err, c.muon) {
				t.Errorf("TimHoacTao: err = %v, muốn %v", err, c.muon)
			}
		})
	}
	if _, _, err := s.TimHoacTao(ctx, nil, "123", maZaloGia); !errors.Is(err, ErrThieuGiaoDichZalo) {
		t.Errorf("TimHoacTao không giao dịch: err = %v", err)
	}
}

func TestGhiTaiKhoanCanGiaoDichVaDinhDanh(t *testing.T) {
	s := &TaiKhoanZaloStore{}
	ctx := context.Background()
	if err := s.TroToiDinhDanh(ctx, nil, "", "CD"); !errors.Is(err, ErrThieuTaiKhoan) {
		t.Errorf("err = %v", err)
	}
	if err := s.TroToiDinhDanh(ctx, nil, "TK", " "); !errors.Is(err, ErrThieuDinhDanh) {
		t.Errorf("err = %v", err)
	}
	if err := s.TroToiDinhDanh(ctx, nil, "TK", "CD"); !errors.Is(err, ErrThieuGiaoDichZalo) {
		t.Errorf("err = %v", err)
	}
	if err := s.NhoXaCuaGiaoDich(ctx, nil, ""); !errors.Is(err, ErrThieuTaiKhoan) {
		t.Errorf("err = %v", err)
	}
	if err := s.NhoXaCuaGiaoDich(ctx, nil, "TK"); !errors.Is(err, ErrThieuGiaoDichZalo) {
		t.Errorf("err = %v", err)
	}
}

func TestBamMaZaloDungHinhDangRangBuoc(t *testing.T) {
	// CHECK tai_khoan_zalo_bam_la_sha256 in migration 0011 — the hash must satisfy it, and must
	// never be the raw id.
	b := bamMaZalo(maZaloGia)
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(b) {
		t.Fatalf("băm %q không khớp ràng buộc của migration 0011", b)
	}
	if strings.Contains(b, maZaloGia) || bamMaZalo(maZaloGia) != b {
		t.Fatal("băm không tất định hoặc chứa mã thô")
	}
	_, bam, err := khoaZalo(" 123 ", " "+maZaloGia+" ")
	if err != nil || bam != b {
		t.Fatalf("khoá Zalo không cắt khoảng trắng trước khi băm: %v", err)
	}
}

func TestLoiTaiKhoanZaloKhongMangGiaTri(t *testing.T) {
	for _, err := range []error{ErrThieuAppID, ErrThieuMaZalo, ErrKhoaZaloQuaDai, ErrThieuTaiKhoan,
		ErrThieuDinhDanh, ErrKhongCoTaiKhoan, ErrThieuGiaoDichZalo} {
		if strings.ContainsAny(err.Error(), "0123456789") {
			t.Errorf("lỗi dựng sẵn chứa chữ số: %v", err)
		}
	}
}
