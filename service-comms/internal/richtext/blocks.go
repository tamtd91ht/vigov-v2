package richtext

import (
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// BlockKind is one of the block shapes the Mini App draws. Closed: a kind outside this list is a shape
// no client knows, and an app build on a resident's phone would show nothing for it. The last three
// came with ADR 0067 §Sửa đổi 03/10/2026 (K4) — acceptable then only because no Mini App build was on
// Zalo yet; the next addition has no such excuse.
type BlockKind string

const (
	KindParagraph   BlockKind = "paragraph"
	KindHeading     BlockKind = "heading"
	KindBulletList  BlockKind = "bullet_list"
	KindOrderedList BlockKind = "ordered_list"
	KindImage       BlockKind = "image"
	KindQuote       BlockKind = "quote"
	KindByline      BlockKind = "byline"
)

// Run is a stretch of text with one formatting. Href is "" or an https URL (it came through Sanitize);
// a run with an Href is a link the client opens externally after asking (owner, 01/10/2026).
//
// "\n" inside Text is a line break (`<br>`). Every other whitespace run is one space.
type Run struct {
	Text   string
	Bold   bool
	Italic bool
	Href   string
}

// Block is one block:
//
//	paragraph, heading, byline   Runs. Level is 2 or 3 on a heading and 0 otherwise.
//	bullet_list, ordered_list    Items, one slice of runs per item.
//	quote                        Paragraphs, one slice of runs per paragraph of the quote.
//	image                        FileID (a ULID — NEVER a URL, and never sent to a resident: the public
//	                             read swaps it for the URL of this article's own published file, K2/K7),
//	                             Alt ("" when none), Caption (nil when none, H3).
type Block struct {
	Kind       BlockKind
	Level      int
	Runs       []Run
	Items      [][]Run
	Paragraphs [][]Run
	FileID     string
	Alt        string
	Caption    []Run
}

// Blocks builds the block structure from SANITISED HTML (call SanitizeStaff first — the public read
// does).
//
// WHAT IT DOES WITH STRUCTURE IT DOES NOT MODEL: flattens it to text. A list nested in a list item, a
// paragraph inside a list item, an `li` outside any list — the text survives, separated by a space,
// and the nesting does not. Losing a level of indentation is the fail-safe side; a block kind outside
// the closed list is not.
//
// LOOSE TEXT at the top level (a body typed into a plain textarea before the editor existed, or text
// between blocks) becomes paragraphs: a blank line separates two, a single newline is a line break —
// the same reading VanBanThuanChoDan gives the plain `body`, so the two forms of one article agree.
func Blocks(sanitised string) []Block {
	if strings.TrimSpace(sanitised) == "" {
		return nil
	}
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(sanitised), ctx)
	if err != nil {
		// ParseFragment fails only when its reader does; a strings.Reader does not. Nothing to show
		// beats a guess — the plain `body` still carries the text.
		return nil
	}
	b := &builder{}
	for _, n := range nodes {
		b.top(n)
	}
	b.flushLoose()
	return b.blocks
}

// ImageFileIDs returns the distinct file ids of the image blocks of a body, in body order — exactly the
// images Blocks would draw, because it IS Blocks over the staff policy. Any body may be passed (stored,
// legacy, already sanitised): SanitizeStaff runs first, so an id the policy would drop is never returned.
//
// ONE DEFINITION OF "THE BODY'S IMAGES" for the attach check on save, the retire of removed images, the
// publish of a published article's images and the staff preview — four readers that must agree on which
// files an article shows, or one publishes a file another never checked.
func ImageFileIDs(body string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, b := range Blocks(SanitizeStaff(body)) {
		if b.Kind == KindImage && b.FileID != "" && !seen[b.FileID] {
			seen[b.FileID] = true
			ids = append(ids, b.FileID)
		}
	}
	return ids
}

// strictOnce builds the no-tags policy decodedText uses.
var (
	strictOnce sync.Once
	strict     *bluemonday.Policy
)

// decodedText is a text node's content with any TAG-SHAPED SEQUENCE removed.
//
// WHY, WHEN THE TEXT IS ALREADY TEXT: markup that arrived entity-encoded (`&lt;script&gt;…`) is markup
// once the parser has decoded it. The plain `body` drops it (domain.VanBanThuanChoDan, step 3), and ADR
// 0067 says the two forms of one article must agree — so a tag-shaped sequence never survives here
// either, whichever spelling it came in, and script/style content goes with its tags. A lone `<` that
// starts no tag ("giá < 5 triệu", "<3") is text and is kept.
func decodedText(s string) string {
	if !strings.ContainsRune(s, '<') {
		return s
	}
	strictOnce.Do(func() { strict = bluemonday.StrictPolicy() })
	// StrictPolicy returns HTML-escaped text; unescape once to get back to text.
	return html.UnescapeString(strict.Sanitize(s))
}

// piece is one unit of inline content before whitespace is normalised.
type piece struct {
	text      string
	style     style
	lineBreak bool // <br>, or a single newline in loose text
	paraBreak bool // a blank line in loose text
}

type style struct {
	bold, italic bool
	href         string
}

type builder struct {
	blocks []Block
	loose  []piece
}

// blankLine is a newline, optional horizontal whitespace, and another newline: the paragraph break of
// loose text.
var blankLine = regexp.MustCompile(`\r?\n[ \t\f\r]*\n`)

func (b *builder) top(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		parts := blankLine.Split(decodedText(n.Data), -1)
		for i, p := range parts {
			if i > 0 {
				b.loose = append(b.loose, piece{paraBreak: true})
			}
			lines := strings.Split(p, "\n")
			for j, l := range lines {
				if j > 0 {
					b.loose = append(b.loose, piece{lineBreak: true})
				}
				b.loose = append(b.loose, piece{text: l})
			}
		}
	case html.ElementNode:
		switch n.DataAtom {
		case atom.P:
			b.flushLoose()
			kind := KindParagraph
			if attrOf(n, "data-role") == bylineValue {
				kind = KindByline
			}
			b.addRuns(Block{Kind: kind}, inline(n))
		case atom.Li:
			b.flushLoose()
			b.addRuns(Block{Kind: KindParagraph}, inline(n))
		case atom.Figure:
			b.flushLoose()
			b.figure(n)
		case atom.Blockquote:
			b.flushLoose()
			b.quote(n)
		case atom.Img:
			// Outside a figure: not a shape (SanitizeStaff removes it; only an unsanitised caller gets
			// here). An image is never inferred from a bare img — least of all from its src.
		case atom.H2, atom.H3:
			b.flushLoose()
			level := 2
			if n.DataAtom == atom.H3 {
				level = 3
			}
			b.addRuns(Block{Kind: KindHeading, Level: level}, inline(n))
		case atom.Ul, atom.Ol:
			b.flushLoose()
			b.list(n)
		case atom.Strong, atom.Em, atom.A, atom.Br, atom.B, atom.I:
			collect(n, style{}, &b.loose)
		default:
			// Not in the allow-list (Sanitize removes these, so only an unsanitised caller gets here):
			// its children are read as if they stood at the top level.
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				b.top(c)
			}
		}
	}
}

func (b *builder) flushLoose() {
	if len(b.loose) == 0 {
		return
	}
	start := 0
	for i := 0; i <= len(b.loose); i++ {
		if i == len(b.loose) || b.loose[i].paraBreak {
			b.addRuns(Block{Kind: KindParagraph}, normalise(b.loose[start:i]))
			start = i + 1
		}
	}
	b.loose = nil
}

// addRuns appends blk with runs, unless the runs hold no text: an empty paragraph is a gap on a
// resident's screen with nothing in it.
func (b *builder) addRuns(blk Block, runs []Run) {
	if len(runs) == 0 {
		return
	}
	blk.Runs = runs
	b.blocks = append(b.blocks, blk)
}

func (b *builder) list(n *html.Node) {
	kind := KindBulletList
	if n.DataAtom == atom.Ol {
		kind = KindOrderedList
	}
	blk := Block{Kind: kind}
	// Content of the list that is not inside an `li` (text between items, a stray inline) is gathered
	// into an item of its own rather than dropped.
	var stray []piece
	flushStray := func() {
		if runs := normalise(stray); len(runs) > 0 {
			blk.Items = append(blk.Items, runs)
		}
		stray = nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == atom.Li {
			flushStray()
			if runs := inline(c); len(runs) > 0 {
				blk.Items = append(blk.Items, runs)
			}
			continue
		}
		collect(c, style{}, &stray)
	}
	flushStray()
	if len(blk.Items) > 0 {
		b.blocks = append(b.blocks, blk)
	}
}

// figure adds an image block when n holds a direct-child img with a file id of ULID shape — checked
// again here, so a caller that skipped SanitizeStaff still gets no image from anything else. The first
// figcaption is the caption.
func (b *builder) figure(n *html.Node) {
	blk := Block{Kind: KindImage}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch {
		case c.DataAtom == atom.Img && blk.FileID == "":
			if id := attrOf(c, "data-file-id"); fileIDShape.MatchString(id) {
				blk.FileID = id
				blk.Alt = strings.Join(strings.Fields(decodedText(attrOf(c, "alt"))), " ")
			}
		case c.DataAtom == atom.Figcaption && blk.Caption == nil:
			blk.Caption = inline(c)
		}
	}
	if blk.FileID == "" {
		return
	}
	b.blocks = append(b.blocks, blk)
}

// quote adds a quote block: one paragraph per `p`, and loose content between them gathered into a
// paragraph of its own — the same rule as a list's stray content. No paragraph with text, no block.
func (b *builder) quote(n *html.Node) {
	blk := Block{Kind: KindQuote}
	var stray []piece
	flushStray := func() {
		if runs := normalise(stray); len(runs) > 0 {
			blk.Paragraphs = append(blk.Paragraphs, runs)
		}
		stray = nil
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == atom.P {
			flushStray()
			if runs := inline(c); len(runs) > 0 {
				blk.Paragraphs = append(blk.Paragraphs, runs)
			}
			continue
		}
		collect(c, style{}, &stray)
	}
	flushStray()
	if len(blk.Paragraphs) > 0 {
		b.blocks = append(b.blocks, blk)
	}
}

// inline reads n's children as inline content.
func inline(n *html.Node) []Run {
	var ps []piece
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collect(c, style{}, &ps)
	}
	return normalise(ps)
}

// collect walks one node as inline content under style s.
func collect(n *html.Node, s style, out *[]piece) {
	switch n.Type {
	case html.TextNode:
		*out = append(*out, piece{text: decodedText(n.Data), style: s})
		return
	case html.ElementNode:
	default:
		return
	}
	separator := false
	switch n.DataAtom {
	case atom.Br:
		*out = append(*out, piece{lineBreak: true})
		return
	case atom.Strong, atom.B:
		s.bold = true
	case atom.Em, atom.I:
		s.italic = true
	case atom.A:
		if h := attrOf(n, "href"); isHTTPS(h) {
			s.href = strings.TrimSpace(h)
		}
	default:
		// A block inside inline content (a nested list, a paragraph in a list item): flattened to its
		// text, kept apart from its neighbours by a space.
		separator = true
	}
	if separator {
		*out = append(*out, piece{text: " "})
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collect(c, s, out)
	}
	if separator {
		*out = append(*out, piece{text: " "})
	}
}

func attrOf(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// normalise turns pieces into runs: whitespace runs become one space (carrying the style of the
// piece the whitespace stood in), spaces at the start and end of a line go, control characters go,
// adjacent runs of one style merge, and no run is empty.
func normalise(ps []piece) []Run {
	var runs []Run
	emit := func(text string, s style) {
		if n := len(runs); n > 0 {
			last := &runs[n-1]
			if last.Bold == s.bold && last.Italic == s.italic && last.Href == s.href {
				last.Text += text
				return
			}
		}
		runs = append(runs, Run{Text: text, Bold: s.bold, Italic: s.italic, Href: s.href})
	}

	lineStart := true
	pending := false
	var pendingStyle style
	for _, p := range ps {
		if p.lineBreak {
			// A break at the very start is nothing; a space before a break is dropped.
			if len(runs) > 0 {
				runs[len(runs)-1].Text += "\n"
			}
			pending, lineStart = false, true
			continue
		}
		var b strings.Builder
		for _, r := range p.text {
			switch {
			case unicode.IsSpace(r):
				if !lineStart && !pending {
					pending, pendingStyle = true, p.style
				}
			case unicode.IsControl(r):
				// dropped: corrupts a screen and a log line alike
			default:
				if pending {
					if b.Len() > 0 {
						emit(b.String(), p.style)
						b.Reset()
					}
					emit(" ", pendingStyle)
					pending = false
				}
				b.WriteRune(r)
				lineStart = false
			}
		}
		if b.Len() > 0 {
			emit(b.String(), p.style)
		}
	}

	// Trailing line breaks go; then empty runs (a run that held only a break that was trimmed).
	for n := len(runs); n > 0; n = len(runs) {
		runs[n-1].Text = strings.TrimRight(runs[n-1].Text, "\n")
		if runs[n-1].Text != "" {
			break
		}
		runs = runs[:n-1]
	}
	return runs
}
