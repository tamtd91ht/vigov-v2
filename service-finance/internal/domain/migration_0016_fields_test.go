package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The domain half of migration 0016: the free-text "Đơn vị thực hiện" and an issue's owner and due date.

func TestNormaliseImplementingUnit(t *testing.T) {
	for _, c := range []struct {
		in, want string
		err      error
	}{
		{"  Công ty Xây dựng Thành Long  ", "Công ty Xây dựng Thành Long", nil},
		{"", "", nil},    // not named — the store writes NULL
		{"   ", "", nil}, // blank is "not named", never '' in the row (0016's CHECK)
		{strings.Repeat("ạ", ImplementingUnitMax), strings.Repeat("ạ", ImplementingUnitMax), nil}, // 255 CHARACTERS, not bytes
		{strings.Repeat("ạ", ImplementingUnitMax+1), "", ErrImplementingUnitTooLong},
		{"Công ty\nABC", "", ErrImplementingUnitInvalid},
		{"Công ty\tABC", "", ErrImplementingUnitInvalid},
	} {
		got, err := NormaliseImplementingUnit(c.in)
		if got != c.want || !errors.Is(err, c.err) || (c.err == nil && err != nil) {
			t.Errorf("NormaliseImplementingUnit(%q) = %q, %v; want %q, %v", c.in, got, err, c.want, c.err)
		}
	}
}

func TestImplementingUnitErrorsNeverQuoteTheValue(t *testing.T) {
	// Free text that may name a person (rule 3): the sentence names the field and the bound only.
	_, err := NormaliseImplementingUnit("Nguyễn Văn A\x00")
	if err == nil || strings.Contains(err.Error(), "Nguyễn") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(ErrImplementingUnitTooLong.Error(), "255") {
		t.Fatalf("the bound must be in the sentence: %q", ErrImplementingUnitTooLong)
	}
}

func TestNormaliseIssueOwnerCode(t *testing.T) {
	for _, c := range []struct {
		in, want string
		err      error
	}{
		{" CB-00042 ", "CB-00042", nil},
		{"", "", nil},
		{"   ", "", nil},
		{"CB 00042", "", ErrIssueOwnerInvalid},
		{"CB-\x0042", "", ErrIssueOwnerInvalid},
		{strings.Repeat("C", StaffCodeMax+1), "", ErrIssueOwnerInvalid},
	} {
		got, err := NormaliseIssueOwnerCode(c.in)
		if got != c.want || !errors.Is(err, c.err) || (c.err == nil && err != nil) {
			t.Errorf("NormaliseIssueOwnerCode(%q) = %q, %v; want %q, %v", c.in, got, err, c.want, c.err)
		}
	}
}

func TestCheckIssueDueOn(t *testing.T) {
	day := func(y int) time.Time { return time.Date(y, 6, 1, 0, 0, 0, 0, time.UTC) }
	for _, c := range []struct {
		in   time.Time
		want error
	}{
		{time.Time{}, nil},
		{day(2020), nil}, // in the past is accepted — the prototype has no such rule
		{day(2100), nil},
		{day(1999), ErrIssueDueOnInvalid},
		{day(2101), ErrIssueDueOnInvalid},
	} {
		if err := CheckIssueDueOn(c.in); !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("CheckIssueDueOn(%v) = %v, want %v", c.in, err, c.want)
		}
	}
}

func TestMentionsStillRefuseWhatOwnerRefuses(t *testing.T) {
	// One shape predicate behind both: a code a mention refuses is a code an owner refuses.
	for _, bad := range []string{"CB 1", "CB\x001", strings.Repeat("C", StaffCodeMax+1)} {
		if _, err := NormaliseMentions([]string{bad}); !errors.Is(err, ErrMentionInvalid) {
			t.Errorf("mention %q: err = %v", bad, err)
		}
		if _, err := NormaliseIssueOwnerCode(bad); !errors.Is(err, ErrIssueOwnerInvalid) {
			t.Errorf("owner %q: err = %v", bad, err)
		}
	}
	if _, err := NormaliseMentions([]string{"  "}); !errors.Is(err, ErrMentionInvalid) {
		t.Errorf("a blank mention must still be refused: %v", err)
	}
}
