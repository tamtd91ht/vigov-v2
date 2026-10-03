package richtext

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// hasStaffShape reports whether bluemonday's staff output holds anything whose POSITION must be
// checked. Without one, the output is already final — and identical to Sanitize's.
func hasStaffShape(s string) bool {
	for _, tag := range []string{"<figure", "<figcaption", "<img", "<blockquote", "data-role"} {
		if strings.Contains(s, tag) {
			return true
		}
	}
	return false
}

// enforceStaffShapes rebuilds the staff shapes in the exact form SanitizeStaff documents, on
// bluemonday's output (so every element and attribute here is already in the staff SET). It decides
// only where each may stand:
//
//	figure      top level only; kept only with a direct-child img carrying a valid data-file-id. Rebuilt
//	            from scratch: img with data-file-id and alt and nothing else, then the first figcaption,
//	            inline content only. Any other content of the figure is dropped.
//	img         only as rebuilt above. Anywhere else — loose, in a paragraph, in a list — dropped.
//	blockquote  top level only; rebuilt as paragraphs of inline content (loose text and inline marks
//	            between them become a paragraph). Nested elsewhere: unwrapped, its text stays.
//	figcaption  only inside a rebuilt figure. Anywhere else: unwrapped, its text stays.
//	data-role   only on a top-level p, only `byline` (the policy already refused other values).
//
// Re-rendering with html.Render is safe for the same reason fixLinks' re-tokenising is: it re-escapes
// every attribute value and every text node it writes.
func enforceStaffShapes(s string) string {
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), ctx)
	if err != nil {
		// ParseFragment fails only when its reader does; a strings.Reader does not. Fail closed: an
		// empty body is refused by the write path's own check, never stored as a guess.
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, n := range nodes {
		for _, out := range staffTop(n) {
			if err := html.Render(&b, out); err != nil {
				// Render fails only when its writer does; a strings.Builder does not.
				return ""
			}
		}
	}
	return b.String()
}

// staffTop returns what one top-level node becomes: nothing, itself cleaned, or (for a stray
// figcaption) its inline content.
func staffTop(n *html.Node) []*html.Node {
	if n.Type != html.ElementNode {
		if n.Type == html.TextNode {
			return []*html.Node{n}
		}
		return nil
	}
	switch n.DataAtom {
	case atom.Figure:
		if f := buildFigure(n); f != nil {
			return []*html.Node{f}
		}
		return nil
	case atom.Img:
		return nil
	case atom.Blockquote:
		if q := buildQuote(n); q != nil {
			return []*html.Node{q}
		}
		return nil
	case atom.Figcaption:
		inlineOnly(n)
		return detachChildren(n)
	case atom.P:
		byline := attrOf(n, "data-role") == bylineValue
		n.Attr = nil
		if byline {
			n.Attr = []html.Attribute{{Key: "data-role", Val: bylineValue}}
		}
		inlineOnly(n)
		return []*html.Node{n}
	default:
		cleanNested(n)
		return []*html.Node{n}
	}
}

// buildFigure returns the canonical figure for n, or nil when n holds no valid img.
func buildFigure(n *html.Node) *html.Node {
	var img, caption *html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch {
		case c.DataAtom == atom.Img && img == nil && fileIDShape.MatchString(attrOf(c, "data-file-id")):
			img = c
		case c.DataAtom == atom.Figcaption && caption == nil:
			caption = c
		}
	}
	if img == nil {
		return nil
	}
	fig := element(atom.Figure)
	attrs := []html.Attribute{{Key: "data-file-id", Val: attrOf(img, "data-file-id")}}
	if alt := strings.TrimSpace(attrOf(img, "alt")); alt != "" {
		attrs = append(attrs, html.Attribute{Key: "alt", Val: alt})
	}
	im := element(atom.Img)
	im.Attr = attrs
	fig.AppendChild(im)
	if caption != nil {
		inlineOnly(caption)
		if hasText(caption) {
			fc := element(atom.Figcaption)
			for _, c := range detachChildren(caption) {
				fc.AppendChild(c)
			}
			fig.AppendChild(fc)
		}
	}
	return fig
}

// buildQuote returns the canonical blockquote for n — paragraphs of inline content only — or nil when
// no paragraph holds text.
func buildQuote(n *html.Node) *html.Node {
	q := element(atom.Blockquote)
	var pending *html.Node
	flush := func() {
		if pending != nil && hasText(pending) {
			q.AppendChild(pending)
		}
		pending = nil
	}
	for _, c := range detachChildren(n) {
		switch {
		case c.Type == html.TextNode || (c.Type == html.ElementNode && isInline(c.DataAtom)):
			if pending == nil {
				pending = element(atom.P)
			}
			pending.AppendChild(c)
		case c.Type == html.ElementNode && (c.DataAtom == atom.Img || c.DataAtom == atom.Figure):
			// An image inside a quote is not a staff shape: dropped, caption included.
		case c.Type == html.ElementNode:
			// A paragraph — or any other block (a heading, a list, a nested quote), flattened to one
			// paragraph of its inline content.
			flush()
			p := element(atom.P)
			inlineOnly(c)
			for _, gc := range detachChildren(c) {
				p.AppendChild(gc)
			}
			pending = p
			flush()
		}
	}
	flush()
	if q.FirstChild == nil {
		return nil
	}
	inlineOnlyEach(q)
	return q
}

// inlineOnlyEach runs inlineOnly on each paragraph of a rebuilt quote — the loose inline marks gathered
// into a pending paragraph may still hold an img.
func inlineOnlyEach(q *html.Node) {
	for p := q.FirstChild; p != nil; p = p.NextSibling {
		inlineOnly(p)
	}
}

// inlineOnly reduces n's content to text, br, strong, em and a: images and figures are dropped whole,
// every other element is unwrapped with a space on each side so words of two blocks do not run together.
func inlineOnly(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type != html.ElementNode {
			if c.Type != html.TextNode {
				n.RemoveChild(c)
			}
			c = next
			continue
		}
		switch {
		case c.DataAtom == atom.Img || c.DataAtom == atom.Figure:
			n.RemoveChild(c)
		case isInline(c.DataAtom):
			c.Attr = keepOnly(c)
			inlineOnly(c)
		default:
			inlineOnly(c)
			n.InsertBefore(&html.Node{Type: html.TextNode, Data: " "}, c)
			for _, gc := range detachChildren(c) {
				n.InsertBefore(gc, c)
			}
			n.InsertBefore(&html.Node{Type: html.TextNode, Data: " "}, c)
			n.RemoveChild(c)
		}
		c = next
	}
}

// cleanNested applies the position rules inside a block that is not a staff shape (a heading, a list,
// loose inline marks): images and figures dropped, quotes and figcaptions unwrapped, data-role removed.
func cleanNested(n *html.Node) {
	if n.DataAtom == atom.P {
		n.Attr = nil
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type != html.ElementNode {
			c = next
			continue
		}
		switch c.DataAtom {
		case atom.Img, atom.Figure:
			n.RemoveChild(c)
		case atom.Blockquote, atom.Figcaption:
			cleanNested(c)
			for _, gc := range detachChildren(c) {
				n.InsertBefore(gc, c)
			}
			n.RemoveChild(c)
		default:
			cleanNested(c)
		}
		c = next
	}
}

// isInline reports the inline elements of the allow-list.
func isInline(a atom.Atom) bool {
	return a == atom.Strong || a == atom.Em || a == atom.A || a == atom.Br
}

// keepOnly returns the attributes an inline element keeps: `a` its href and rel (fixLinks already set
// both), every other inline element none.
func keepOnly(n *html.Node) []html.Attribute {
	if n.DataAtom != atom.A {
		return nil
	}
	var out []html.Attribute
	for _, a := range n.Attr {
		if a.Key == "href" || a.Key == "rel" {
			out = append(out, a)
		}
	}
	return out
}

// hasText reports whether n holds any non-whitespace text.
func hasText(n *html.Node) bool {
	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data) != ""
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if hasText(c) {
			return true
		}
	}
	return false
}

// detachChildren removes n's children and returns them in order.
func detachChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		out = append(out, c)
		c = next
	}
	return out
}

func element(a atom.Atom) *html.Node {
	return &html.Node{Type: html.ElementNode, Data: a.String(), DataAtom: a}
}
