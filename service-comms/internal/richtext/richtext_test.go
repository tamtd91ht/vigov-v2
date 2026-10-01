package richtext

import (
	"reflect"
	"strings"
	"testing"
)

// --- Sanitize: the allow-list of ADR 0067 §1 decision 1, exactly ---------------------------------

func TestSanitizeKeepsEveryAllowedTag(t *testing.T) {
	in := `<h2>Đầu mục</h2><h3>Mục con</h3><p>Một <strong>đậm</strong> và <em>nghiêng</em><br>dòng mới</p>` +
		`<ul><li>một</li></ul><ol><li>hai</li></ol>`
	got := Sanitize(in)
	for _, want := range []string{"<h2>", "<h3>", "<p>", "<strong>", "<em>", "<br", "<ul>", "<ol>", "<li>"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q bị gỡ: %s", want, got)
		}
	}
}

func TestSanitizeStripsEverythingElse(t *testing.T) {
	cases := map[string]string{
		"script":        `<p>a</p><script>alert(1)</script>`,
		"img":           `<img src="https://x/a.png" onerror="alert(1)">`,
		"iframe":        `<iframe src="https://evil.example"></iframe>`,
		"style element": `<style>p{color:red}</style><p>a</p>`,
		"style attr":    `<p style="color:red">a</p>`,
		"on* attr":      `<p onclick="alert(1)">a</p>`,
		"class attr":    `<strong class="x" id="y">a</strong>`,
		"h1":            `<h1>a</h1>`,
		"div":           `<div>a</div>`,
		"svg":           `<svg onload="alert(1)"><circle/></svg>`,
	}
	banned := []string{"<script", "alert(", "<img", "<iframe", "<style", "style=", "onclick", "onerror",
		"onload", "class=", "id=", "<h1", "<div", "<svg", "color:red"}
	for name, in := range cases {
		got := Sanitize(in)
		for _, b := range banned {
			if strings.Contains(strings.ToLower(got), b) {
				t.Errorf("%s: %q còn sau khi làm sạch: %s", name, b, got)
			}
		}
	}
}

func TestSanitizeLinksHTTPSOnlyWithFixedRel(t *testing.T) {
	got := Sanitize(`<p><a href="https://thangbinh.danang.gov.vn/tin" target="_blank" rel="opener" title="t">Cổng</a></p>`)
	want := `<p><a href="https://thangbinh.danang.gov.vn/tin" rel="noopener noreferrer nofollow">Cổng</a></p>`
	if got != want {
		t.Errorf("liên kết https:\n got %s\nwant %s", got, want)
	}

	for _, href := range []string{
		"javascript:alert(1)", "JaVaScRiPt:alert(1)", "data:text/html,<script>alert(1)</script>",
		"http://x.example/a", "//evil.example/a", "/duong-trong-app", "vbscript:x", "mailto:a@b.vn",
		"&#106;avascript:alert(1)", " javascript:alert(1)",
	} {
		got := Sanitize(`<p><a href="` + href + `">chữ</a></p>`)
		if strings.Contains(got, "<a") || strings.Contains(got, "href") {
			t.Errorf("href %q phải bị gỡ: %s", href, got)
		}
		if !strings.Contains(got, "chữ") {
			t.Errorf("href %q: chữ của liên kết phải giữ lại: %s", href, got)
		}
	}
}

func TestSanitizeEntityEncodedMarkupStaysText(t *testing.T) {
	got := Sanitize(`<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>`)
	if strings.Contains(got, "<script") {
		t.Fatalf("markup mã hoá thực thể thành thẻ thật: %s", got)
	}
}

func TestSanitizeIsIdempotent(t *testing.T) {
	in := `<p>a <a href="https://x.gov.vn">b</a> &amp; c</p><ul><li><strong>d</strong></li></ul>`
	once := Sanitize(in)
	if twice := Sanitize(once); twice != once {
		t.Errorf("làm sạch hai lần khác một lần:\n%s\n%s", once, twice)
	}
}

// --- Blocks -------------------------------------------------------------------------------------

func TestBlocksParagraphWithNestedInline(t *testing.T) {
	got := Blocks(Sanitize(`<p>Bà con <strong>chú ý <em>lịch</em></strong> tiêm   chủng</p>`))
	want := []Block{{Kind: KindParagraph, Runs: []Run{
		{Text: "Bà con "},
		{Text: "chú ý ", Bold: true},
		{Text: "lịch", Bold: true, Italic: true},
		{Text: " tiêm chủng"},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestBlocksHeadingsListsAndLinks(t *testing.T) {
	in := `<h2>Thông báo</h2><h3>Chi tiết</h3>` +
		`<ul><li>Mang <a href="https://dichvucong.gov.vn">giấy tờ</a></li><li>  Đúng giờ </li></ul>` +
		`<ol><li>Một</li><li><em>Hai</em></li></ol>`
	got := Blocks(Sanitize(in))
	want := []Block{
		{Kind: KindHeading, Level: 2, Runs: []Run{{Text: "Thông báo"}}},
		{Kind: KindHeading, Level: 3, Runs: []Run{{Text: "Chi tiết"}}},
		{Kind: KindBulletList, Items: [][]Run{
			{{Text: "Mang "}, {Text: "giấy tờ", Href: "https://dichvucong.gov.vn"}},
			{{Text: "Đúng giờ"}},
		}},
		{Kind: KindOrderedList, Items: [][]Run{{{Text: "Một"}}, {{Text: "Hai", Italic: true}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestBlocksLineBreaksAndEmptyBlocksDropped(t *testing.T) {
	got := Blocks(Sanitize(`<p></p><p>  </p><p>Dòng một  <br>  dòng hai<br></p><ul><li></li></ul>`))
	want := []Block{{Kind: KindParagraph, Runs: []Run{{Text: "Dòng một\ndòng hai"}}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestBlocksNestedListFlattenedToText(t *testing.T) {
	got := Blocks(Sanitize(`<ul><li>Cha<ul><li>con</li></ul></li></ul>`))
	want := []Block{{Kind: KindBulletList, Items: [][]Run{{{Text: "Cha con"}}}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestBlocksLoosePlainTextBecomesParagraphs(t *testing.T) {
	// A body typed into a plain textarea before the editor: blank line = paragraph, newline = break.
	got := Blocks(Sanitize("Đoạn một\ndòng hai\n\n  Đoạn hai  "))
	want := []Block{
		{Kind: KindParagraph, Runs: []Run{{Text: "Đoạn một\ndòng hai"}}},
		{Kind: KindParagraph, Runs: []Run{{Text: "Đoạn hai"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestBlocksLegacyUnsanitisedRowAfterSanitize(t *testing.T) {
	// The public read: Blocks(Sanitize(stored)) over a row written before the sanitiser existed.
	legacy := `<div class="x"><h1>Tiêu đề</h1><p onclick="x()">Nội dung <b>đậm</b></p>` +
		`<script>alert(1)</script><img src=x onerror=alert(1)><a href="javascript:alert(1)">bấm</a></div>`
	got := Blocks(Sanitize(legacy))
	want := []Block{
		{Kind: KindParagraph, Runs: []Run{{Text: "Tiêu đề"}}},
		{Kind: KindParagraph, Runs: []Run{{Text: "Nội dung đậm"}}},
		{Kind: KindParagraph, Runs: []Run{{Text: "bấm"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
	for _, b := range got {
		for _, r := range b.Runs {
			if strings.Contains(r.Text, "alert") || r.Href != "" {
				t.Errorf("dòng cũ còn mã hoặc liên kết: %#v", r)
			}
		}
	}
}

func TestBlocksTextIsDecodedNotEscaped(t *testing.T) {
	got := Blocks(Sanitize(`<p>Bà con &amp; các cháu &lt;3, giá &lt; 5 triệu</p>`))
	if len(got) != 1 || got[0].Runs[0].Text != "Bà con & các cháu <3, giá < 5 triệu" {
		t.Errorf("got %#v", got)
	}
}

func TestBlocksDropEntityEncodedMarkupLikeThePlainBody(t *testing.T) {
	// `&lt;script&gt;` is markup once decoded; the plain `body` drops it (VanBanThuanChoDan step 3), and
	// the two forms of one article must agree (ADR 0067).
	got := Blocks(Sanitize(`<p>Trước &lt;script&gt;alert(1)&lt;/script&gt; sau &lt;b&gt;đậm&lt;/b&gt;</p>`))
	want := []Block{{Kind: KindParagraph, Runs: []Run{{Text: "Trước sau đậm"}}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestBlocksEmptyIsNil(t *testing.T) {
	for _, in := range []string{"", "   ", "<p></p>", "<script>x</script>"} {
		if got := Blocks(Sanitize(in)); got != nil {
			t.Errorf("%q: got %#v, want nil", in, got)
		}
	}
}
