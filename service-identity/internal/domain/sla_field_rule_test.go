package domain

import (
	"errors"
	"testing"
)

func TestNormalizeSLAField(t *testing.T) {
	if got, err := NormalizeSLAField("  an-ninh "); err != nil || got != "an-ninh" {
		t.Errorf("= %q, %v", got, err)
	}
	for in, want := range map[string]error{
		"":                       ErrSLAFieldMissing,
		"   ":                    ErrSLAFieldMissing,
		"an ninh":                ErrSLAFieldMalformed,
		"a\tb":                   ErrSLAFieldMalformed,
		string(make([]byte, 65)): ErrSLAFieldMalformed,
	} {
		if _, err := NormalizeSLAField(in); !errors.Is(err, want) {
			t.Errorf("%q: lỗi = %v, muốn %v", in, err, want)
		}
	}
}

func TestCheckSLAKindTakesFieldRows(t *testing.T) {
	for _, k := range []LoaiViec{LoaiViecPhanAnh, LoaiViecVanBanDen, LoaiViecNhiemVu} {
		if err := CheckSLAKindTakesFieldRows(k); err != nil {
			t.Errorf("%s: %v", k, err)
		}
	}
	if err := CheckSLAKindTakesFieldRows(LoaiViecDonThu); !errors.Is(err, ErrSLAKindHasNoFieldRows) {
		t.Errorf("don-thu: %v", err)
	}
	if err := CheckSLAKindTakesFieldRows("khac"); !errors.Is(err, ErrSLAKindUnknown) {
		t.Errorf("khac: %v", err)
	}
}

func TestCheckSLARowRemovable(t *testing.T) {
	if err := CheckSLARowRemovable(DongSLA{LinhVuc: ""}); !errors.Is(err, ErrSLADefaultRowNotRemovable) {
		t.Errorf("dòng mặc định: %v", err)
	}
	if err := CheckSLARowRemovable(DongSLA{LinhVuc: "an-ninh"}); err != nil {
		t.Errorf("dòng riêng: %v", err)
	}
}

func TestNormalizeSLADeleteReason(t *testing.T) {
	if got, err := NormalizeSLADeleteReason("  gộp "); err != nil || got != "gộp" {
		t.Errorf("= %q, %v", got, err)
	}
	// OPTIONAL ON THE WAY IN, NEVER EMPTY IN THE ROW (owner decision 10/10/2026; rule 7, invariant 1).
	for _, blank := range []string{"", " ", "\t\n"} {
		if got, err := NormalizeSLADeleteReason(blank); err != nil || got != SLAFieldRowDeleteDefaultReason {
			t.Errorf("rỗng %q: = %q, %v — muốn câu cố định", blank, got, err)
		}
	}
	if n := len([]rune(SLAFieldRowDeleteDefaultReason)); n == 0 || n > SLADeleteReasonMaxLen {
		t.Errorf("câu cố định dài %d ký tự", n)
	}
	long := make([]rune, SLADeleteReasonMaxLen+1)
	for i := range long {
		long[i] = 'ộ'
	}
	if _, err := NormalizeSLADeleteReason(string(long)); !errors.Is(err, ErrSLADeleteReasonTooLong) {
		t.Errorf("dài: %v", err)
	}
}

func TestNormalizeCatalogueColor(t *testing.T) {
	for in, want := range map[string]string{"#1F6FEB": "#1f6feb", " #abcdef ": "#abcdef", "#000000": "#000000"} {
		if got, err := NormalizeCatalogueColor(in); err != nil || got != want {
			t.Errorf("%q = %q, %v; muốn %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "#fff", "1f6feb", "#1f6febff", "#gggggg", "red", "#12345 "} {
		if _, err := NormalizeCatalogueColor(bad); !errors.Is(err, ErrCatalogueColorInvalid) {
			t.Errorf("%q: lỗi = %v", bad, err)
		}
	}
}
