package domain

import (
	"regexp"
	"strings"
	"testing"
)

// tagLeft matches anything tag-shaped. THE PROPERTY every case below must hold: none survives.
var tagLeft = regexp.MustCompile(`<[A-Za-z/!?]`)

func mustBePlain(t *testing.T, in, out string) {
	t.Helper()
	if tagLeft.MatchString(out) {
		t.Fatalf("còn thẻ HTML sau khi lọc:\nvào: %q\nra:  %q", in, out)
	}
	// Attributes live inside tags; with every tag gone none can survive.
	lower := strings.ToLower(out)
	for _, banned := range []string{"javascript:", "onerror", "<script", "</script"} {
		if strings.Contains(lower, banned) {
			t.Fatalf("còn %q sau khi lọc:\nvào: %q\nra:  %q", banned, in, out)
		}
	}
}

func TestPlainTextForCitizenKeepsTextDropsTags(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"<p>Xin chào bà con</p><p>Lịch tiêm chủng tuần này.</p>", "Xin chào bà con\n\nLịch tiêm chủng tuần này."},
		{"Dòng một<br>Dòng hai<br/>Dòng ba", "Dòng một\nDòng hai\nDòng ba"},
		{"<b>UBND</b> xã <i>thông báo</i>", "UBND xã thông báo"},
		{"bà &amp; con &quot;xã&quot; &nbsp;nhé", `bà & con "xã" nhé`},
		{"giá < 5 triệu và 7 > 3", "giá < 5 triệu và 7 > 3"},
		{"Văn bản thường\n\n\n\nĐoạn hai", "Văn bản thường\n\nĐoạn hai"},
		{`<div title="a>b">nội dung</div>`, "nội dung"},
		{`<a href="javascript:alert(1)">bấm vào đây</a>`, "bấm vào đây"},
		{"<ul><li>một</li><li>hai</li></ul>", "một\n\nhai"},
		{"  \t khoảng   trắng \r\n thừa  ", "khoảng trắng\nthừa"},
		{"", ""},
	} {
		got := PlainTextForCitizen(tc.in)
		if got != tc.want {
			t.Errorf("PlainTextForCitizen(%q) = %q, muốn %q", tc.in, got, tc.want)
		}
		mustBePlain(t, tc.in, got)
	}
}

func TestPlainTextForCitizenDropsScriptEntirely(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"<p>trước</p><script>alert(1)</script><p>sau</p>", "trước\n\nsau"},
		{"<p>a</p><SCRIPT type='text/javascript'>alert('x')</ScRiPt ><p>b</p>", "a\n\nb"},
		{"<style>p{color:red}</style>chữ", "chữ"},
		{`<img src=x onerror="alert(1)">ảnh`, "ảnh"},
		{"<iframe src='https://x'></iframe>còn lại", "còn lại"},
		{"<svg><script>alert(1)</script></svg>hết", "hết"},
		{"<!-- <script>alert(1)</script> -->sau chú thích", "sau chú thích"},
		// Entity-encoded markup becomes markup after decoding — dropped by the second pass.
		{"&lt;script&gt;alert(1)&lt;/script&gt;tin", "tin"},
		{"&lt;b&gt;đậm&lt;/b&gt;", "đậm"},
		// Nested trick: `<scr<script>` is ONE tag (named `scr`, ending at the first `>`); what is left
		// is inert TEXT — no `<`, so nothing any renderer could read as a tag.
		{"<scr<script>ipt>alert(1)</script>", "ipt>alert(1)"},
		// Unterminated: drop the rest rather than guess where the script ends.
		{"đầu<script>alert(1)", "đầu"},
		{"đầu<img src=x onerror=alert(1)", "đầu"},
		{"đầu<!-- mãi không đóng", "đầu"},
		{"<scriptx>chữ</scriptx>", "chữ"},
	} {
		got := PlainTextForCitizen(tc.in)
		mustBePlain(t, tc.in, got)
		if got != tc.want {
			t.Errorf("PlainTextForCitizen(%q) = %q, muốn %q", tc.in, got, tc.want)
		}
	}
}

func TestPlainTextForCitizenDropsControlCharacters(t *testing.T) {
	got := PlainTextForCitizen("a\x00b\x1bc\x07")
	if got != "abc" {
		t.Fatalf("ký tự điều khiển còn lại: %q", got)
	}
}
