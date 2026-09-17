package privacy

import "testing"

func TestMaskPhone(t *testing.T) {
	cases := map[string]string{
		"0912345678": "09****5678",
		"0900000000": "09****0000", // the agreed fake number still masks
		"123":        "***",
		"":           "",
	}
	for in, want := range cases {
		if got := MaskPhone(in); got != want {
			t.Errorf("MaskPhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskCccd(t *testing.T) {
	if got, want := MaskCccd("079123456789"), "079******789"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMaskName(t *testing.T) {
	if got, want := MaskName("Nguyễn Văn An"), "Nguyễn V. A."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
