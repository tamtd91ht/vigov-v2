package richtext

import (
	"reflect"
	"strings"
	"testing"
)

// fakeFileID is a made-up id of ULID shape — no file anywhere carries it.
const fakeFileID = "01JABCDEFGHJKMNPQRSTVWXYZ0"

// --- SanitizeStaff: ADR 0067 §Sửa đổi 03/10/2026, K1 ---------------------------------------------

func TestSanitizeStaffKeepsEachStaffShapeExactly(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"image with caption and alt",
			`<figure><img data-file-id="` + fakeFileID + `" alt="Lễ ra quân"><figcaption>Ảnh: <em>UBND xã</em></figcaption></figure>`,
			`<figure><img data-file-id="` + fakeFileID + `" alt="Lễ ra quân"/><figcaption>Ảnh: <em>UBND xã</em></figcaption></figure>`},
		{"image without caption or alt",
			`<figure><img data-file-id="` + fakeFileID + `"></figure>`,
			`<figure><img data-file-id="` + fakeFileID + `"/></figure>`},
		{"quote",
			`<blockquote><p>Câu <strong>trích</strong></p><p>đoạn hai</p></blockquote>`,
			`<blockquote><p>Câu <strong>trích</strong></p><p>đoạn hai</p></blockquote>`},
		{"byline",
			`<p data-role="byline">Văn phòng <em>UBND xã</em></p>`,
			`<p data-role="byline">Văn phòng <em>UBND xã</em></p>`},
		{"quote loose text becomes a paragraph",
			`<blockquote>lời <em>nói</em></blockquote>`,
			`<blockquote><p>lời <em>nói</em></p></blockquote>`},
	}
	for _, tc := range cases {
		if got := SanitizeStaff(tc.in); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeStaffDropsWhatIsNotAShape(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"img with src only", `<figure><img src="https://evil.example/a.png"><figcaption>chú thích</figcaption></figure><p>a</p>`, `<p>a</p>`},
		{"img without file id", `<figure><img alt="x"></figure><p>a</p>`, `<p>a</p>`},
		{"file id not a ULID", `<figure><img data-file-id="../../etc/passwd"></figure><p>a</p>`, `<p>a</p>`},
		{"file id lower case", `<figure><img data-file-id="` + strings.ToLower(fakeFileID) + `"></figure><p>a</p>`, `<p>a</p>`},
		{"file id with I/L/O/U", `<figure><img data-file-id="01JABCDEFGHIJKLMNOPQRSTUVW"></figure><p>a</p>`, `<p>a</p>`},
		{"figure without img", `<figure><figcaption>chú thích</figcaption></figure><p>a</p>`, `<p>a</p>`},
		{"img outside figure", `<p>a<img data-file-id="` + fakeFileID + `">b</p><img data-file-id="` + fakeFileID + `">`, `<p>ab</p>`},
		{"figure inside list", `<ul><li>a<figure><img data-file-id="` + fakeFileID + `"></figure></li></ul>`, `<ul><li>a</li></ul>`},
		{"image inside quote", `<blockquote><p>a</p><figure><img data-file-id="` + fakeFileID + `"></figure></blockquote>`, `<blockquote><p>a</p></blockquote>`},
		{"data-role other than byline", `<p data-role="author">a</p>`, `<p>a</p>`},
		{"data-role byline on li", `<ul><li data-role="byline">a</li></ul>`, `<ul><li>a</li></ul>`},
		{"empty quote", `<blockquote>  </blockquote><p>a</p>`, `<p>a</p>`},
		{"stray figcaption keeps its text", `<figcaption>chữ</figcaption>`, `chữ`},
	}
	for _, tc := range cases {
		if got := SanitizeStaff(tc.in); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeStaffStripsSrcAndForeignAttributes(t *testing.T) {
	in := `<figure class="x" style="color:red" onclick="alert(1)"><img src="https://evil.example/a.png" ` +
		`data-file-id="` + fakeFileID + `" alt="a" onerror="alert(1)" width="9" srcset="https://evil.example/b.png 2x">` +
		`<figcaption style="x" onmouseover="alert(1)">c</figcaption></figure>` +
		`<blockquote cite="https://evil.example" class="q"><p style="x">q</p></blockquote>` +
		`<p data-role="byline" class="b" onclick="alert(1)">b</p><iframe src="https://evil.example"></iframe>`
	got := strings.ToLower(SanitizeStaff(in))
	for _, banned := range []string{"src", "evil", "style", "class", "onclick", "onerror", "onmouseover",
		"alert", "width", "cite", "iframe", "color"} {
		if strings.Contains(got, banned) {
			t.Errorf("%q còn sau khi làm sạch: %s", banned, got)
		}
	}
	for _, kept := range []string{`data-file-id="` + strings.ToLower(fakeFileID) + `"`, `<figcaption>c</figcaption>`,
		`<blockquote><p>q</p></blockquote>`, `<p data-role="byline">b</p>`} {
		if !strings.Contains(got, kept) {
			t.Errorf("thiếu %q: %s", kept, got)
		}
	}
}

func TestSanitizeStaffLinksStillHTTPSOnly(t *testing.T) {
	got := SanitizeStaff(`<blockquote><p><a href="javascript:alert(1)">x</a> <a href="https://dichvucong.gov.vn" target="_blank">y</a></p></blockquote>`)
	want := `<blockquote><p>x <a href="https://dichvucong.gov.vn" rel="noopener noreferrer nofollow">y</a></p></blockquote>`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

// Narrow HTML is a fixed point: a portal row read through the staff policy is what the portal stored.
func TestSanitizeStaffEqualsSanitizeOnNarrowHTML(t *testing.T) {
	for _, in := range []string{
		`<h2>Đầu mục</h2><p>Một <strong>đậm</strong> &amp; <em>nghiêng</em><br>dòng</p><ul><li>a</li></ul>`,
		`<p><a href="https://x.gov.vn">b</a></p><div onclick="x()">c</div><script>alert(1)</script>`,
		"Đoạn một\n\nĐoạn hai",
	} {
		if a, b := Sanitize(in), SanitizeStaff(in); a != b {
			t.Errorf("%q:\nportal %s\nstaff  %s", in, a, b)
		}
	}
}

func TestSanitizeStaffIsIdempotent(t *testing.T) {
	in := `<p>Mở &amp; "đầu" 'x'</p><figure><img data-file-id="` + fakeFileID + `" alt="a &amp; b"><figcaption>c<br>d</figcaption></figure>` +
		`<blockquote>lời <strong>nói</strong><p>đoạn</p></blockquote><p data-role="byline">Tác giả</p>`
	once := SanitizeStaff(in)
	if twice := SanitizeStaff(once); twice != once {
		t.Errorf("làm sạch hai lần khác một lần:\n%s\n%s", once, twice)
	}
}

// Amendment stop condition 2: the PORTAL write policy takes none of the staff shapes.
func TestSanitizePortalStillStripsStaffShapes(t *testing.T) {
	in := `<figure><img data-file-id="` + fakeFileID + `" src="https://cong.example/a.png" alt="a"><figcaption>chú thích</figcaption></figure>` +
		`<blockquote><p>trích</p></blockquote><p data-role="byline">Tác giả</p>`
	got := Sanitize(in)
	for _, banned := range []string{"<figure", "<img", "<figcaption", "<blockquote", "data-role", "data-file-id", "src="} {
		if strings.Contains(got, banned) {
			t.Errorf("chính sách Cổng giữ %q: %s", banned, got)
		}
	}
	if want := `chú thích<p>trích</p><p>Tác giả</p>`; got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

// --- Blocks: the three new kinds -----------------------------------------------------------------

func TestBlocksStaffShapes(t *testing.T) {
	in := `<p>Mở đầu</p>` +
		`<figure><img data-file-id="` + fakeFileID + `" alt="  Lễ   ra quân "><figcaption>Ảnh: <strong>xã</strong></figcaption></figure>` +
		`<figure><img data-file-id="` + fakeFileID + `"></figure>` +
		`<blockquote><p>Câu một</p>lời <em>ngoài</em><p>Câu hai</p></blockquote>` +
		`<p data-role="byline">Văn phòng UBND xã</p>`
	got := Blocks(SanitizeStaff(in))
	want := []Block{
		{Kind: KindParagraph, Runs: []Run{{Text: "Mở đầu"}}},
		{Kind: KindImage, FileID: fakeFileID, Alt: "Lễ ra quân", Caption: []Run{{Text: "Ảnh: "}, {Text: "xã", Bold: true}}},
		{Kind: KindImage, FileID: fakeFileID},
		{Kind: KindQuote, Paragraphs: [][]Run{
			{{Text: "Câu một"}},
			{{Text: "lời "}, {Text: "ngoài", Italic: true}},
			{{Text: "Câu hai"}},
		}},
		{Kind: KindByline, Runs: []Run{{Text: "Văn phòng UBND xã"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %#v\nwant %#v", got, want)
	}
}

// A legacy row typed before any sanitiser: a raw `<img src>` never becomes an image block, whichever
// policy the caller forgot.
func TestBlocksLegacyRawImgIsNoImage(t *testing.T) {
	legacy := `<p>Trước</p><img src="https://anh.example/a.png" alt="x"><figure><img src="https://anh.example/b.png"><figcaption>c</figcaption></figure><p>Sau</p>`
	for name, html := range map[string]string{"staff": SanitizeStaff(legacy), "unsanitised": legacy} {
		for _, b := range Blocks(html) {
			if b.Kind == KindImage {
				t.Errorf("%s: một <img src> thô thành khối ảnh: %#v", name, b)
			}
		}
	}
}

func TestBlocksEmptyQuoteAndInvalidFigureAreNothing(t *testing.T) {
	// Unsanitised input, so Blocks' own checks are what is tested.
	got := Blocks(`<blockquote> </blockquote><figure><img data-file-id="nope"><figcaption>c</figcaption></figure>`)
	if got != nil {
		t.Errorf("got %#v, want nil", got)
	}
}
