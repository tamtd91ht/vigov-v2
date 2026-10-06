package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestSplitIssueTextFirstLineIsTitle(t *testing.T) {
	cases := []struct{ in, title, desc string }{
		{"Chưa bàn giao mặt bằng", "Chưa bàn giao mặt bằng", ""},
		{"Chưa bàn giao mặt bằng\r\nĐang chờ huyện duyệt.\nDự kiến tháng 11.",
			"Chưa bàn giao mặt bằng", "Đang chờ huyện duyệt.\nDự kiến tháng 11."},
		{"\n\n  Nhà thầu chậm  \n\n  thiếu vật tư  ", "Nhà thầu chậm", "thiếu vật tư"},
	}
	for _, c := range cases {
		title, desc, err := SplitIssueText(c.in)
		if err != nil || title != c.title || desc != c.desc {
			t.Errorf("SplitIssueText(%q) = %q, %q, %v; muốn %q, %q", c.in, title, desc, err, c.title, c.desc)
		}
	}
}

func TestSplitIssueTextRefusals(t *testing.T) {
	cases := map[string]error{
		"":                                   ErrIssueTextMissing,
		"  \n\t ":                            ErrIssueTextMissing,
		strings.Repeat("a", IssueTitleMax+1): ErrIssueTitleTooLong,
		"t\n" + strings.Repeat("b", IssueDescriptionMax+1): ErrIssueDescriptionTooLong,
		"tiêu đề\x00":          ErrIssueTextInvalid,
		"tiêu đề\nphần\x07sau": ErrIssueTextInvalid,
	}
	for in, want := range cases {
		if _, _, err := SplitIssueText(in); !errors.Is(err, want) {
			t.Errorf("SplitIssueText(len %d) err = %v, muốn %v", len(in), err, want)
		}
	}
	// Exactly at the bound is accepted (runes, not bytes — Vietnamese is multi-byte).
	if _, _, err := SplitIssueText(strings.Repeat("ạ", IssueTitleMax)); err != nil {
		t.Errorf("đúng %d ký tự có dấu bị từ chối: %v", IssueTitleMax, err)
	}
}

func TestNormaliseCommentBody(t *testing.T) {
	got, err := NormaliseCommentBody("  Đề nghị kế toán\r\nkiểm lại đợt 2  ")
	if err != nil || got != "Đề nghị kế toán\nkiểm lại đợt 2" {
		t.Fatalf("= %q, %v", got, err)
	}
	for in, want := range map[string]error{
		" ":                                   ErrCommentBodyMissing,
		strings.Repeat("x", CommentBodyMax+1): ErrCommentBodyTooLong,
		"a\x1bb":                              ErrCommentBodyInvalid,
	} {
		if _, err := NormaliseCommentBody(in); !errors.Is(err, want) {
			t.Errorf("err = %v, muốn %v", err, want)
		}
	}
}

func TestNormaliseMentions(t *testing.T) {
	got, err := NormaliseMentions([]string{" CB-00011 ", "CB-00012", "CB-00011"})
	if err != nil || len(got) != 2 || got[0] != "CB-00011" || got[1] != "CB-00012" {
		t.Fatalf("= %v, %v — muốn cắt khoảng trắng, bỏ trùng, giữ thứ tự", got, err)
	}
	empty, err := NormaliseMentions(nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("nil → %v, %v; muốn slice rỗng khác nil (lưu `[]`, không phải NULL)", empty, err)
	}
	for name, in := range map[string][]string{
		"rỗng":        {""},
		"có dấu cách": {"CB 00011"},
		"quá dài":     {strings.Repeat("x", StaffCodeMax+1)},
	} {
		if _, err := NormaliseMentions(in); !errors.Is(err, ErrMentionInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	many := make([]string, MentionsMax+1)
	for i := range many {
		many[i] = "CB-" + strings.Repeat("1", i+1)
	}
	if _, err := NormaliseMentions(many); !errors.Is(err, ErrMentionsTooMany) {
		t.Errorf("%d người: err = %v", len(many), err)
	}
}

func TestProjectIssueResolvedIsDerivedFromTheTimestamp(t *testing.T) {
	var i ProjectIssue
	if i.Resolved() {
		t.Fatal("chưa có thời điểm gỡ mà đã là đã gỡ")
	}
}
