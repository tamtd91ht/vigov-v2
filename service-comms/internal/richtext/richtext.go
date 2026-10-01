// Package richtext is the ONE place a Mini App article body is made safe (ADR 0067 §1).
//
// TWO FUNCTIONS, ONE PIPELINE:
//
//	Sanitize  arbitrary HTML → HTML holding only the allow-list below. Run on EVERY write path (staff
//	          compose/edit today, the portal sync when it lands) and AGAIN on the public read, because
//	          rows written before this package existed are archival records that are never rewritten
//	          (rule 7) and still hold whatever markup was typed.
//	Blocks    sanitised HTML → a structure (paragraph · heading · list · inline runs). The Mini App
//	          renders THIS and never markup (rule 13 forbidden #3): a client that is never handed HTML
//	          cannot be updated into rendering it.
//
// WHY A PACKAGE OF ITS OWN AND NOT internal/domain, although the card that built it said "domain":
// internal/domain imports the standard library only (skills/go-service-pattern, rule 1 of that table),
// and both halves here stand on third-party parsers. A sanitiser written from scratch over the standard
// library is how sanitisers get written wrongly. The portal sync imports this package exactly as
// internal/app does — one function, so no write path can carry a second policy.
package richtext

import (
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// LinkRel is the `rel` every surviving link carries, whatever the input said. noopener: a page opened
// from the article cannot script the opener. noreferrer: the commune's admin URL is not leaked to the
// target. nofollow: a public authority's page does not lend its rank to whatever a link names.
const LinkRel = "noopener noreferrer nofollow"

// policy is built once: a bluemonday.Policy is safe for concurrent use AFTER construction, and building
// it per call would rebuild the same maps on every article.
var (
	policyOnce sync.Once
	policy     *bluemonday.Policy
)

// allowList is the policy of ADR 0067 §1 decision 1, EXACTLY: p, br, strong, em, ul, ol, li, h2, h3,
// and a with an https href. No other element, no other attribute, no other scheme. Widening it is the
// ADR's stop condition #1 — the customer's call, never a line added here.
func allowList() *bluemonday.Policy {
	policyOnce.Do(func() {
		p := bluemonday.NewPolicy()
		p.AllowElements("p", "br", "strong", "em", "ul", "ol", "li", "h2", "h3")
		p.AllowAttrs("href").OnElements("a")
		// https ONLY. Relative and protocol-relative links are refused (AllowRelativeURLs stays false):
		// `//host/x` leaves for whatever host it names, and a relative path means nothing inside a Mini
		// App that has no origin of its own.
		p.AllowURLSchemes("https")
		p.RequireParseableURLs(true)
		policy = p
	})
	return policy
}

// Sanitize returns html reduced to the allow-list. An `a` whose href does not survive loses the tag
// and keeps its text. The content of script/style is dropped whole (bluemonday's default), never
// printed as text.
//
// THE OUTPUT IS STILL HTML. It is what staff screens store and edit (Tiptap reads it); the Mini App
// gets Blocks(Sanitize(x)), never this string.
func Sanitize(s string) string {
	if s == "" {
		return ""
	}
	return fixLinks(allowList().Sanitize(s))
}

// fixLinks rewrites every `a` start tag to exactly `href` + LinkRel, on bluemonday's output.
//
// WHY A SECOND PASS: bluemonday adds `noopener` only together with `target="_blank"`, and `target` is
// an attribute the allow-list does not grant. Re-tokenising is safe because html.Token.String
// re-escapes every attribute value and every text node it emits. A link whose href is not https here
// — impossible after the policy, checked anyway — loses its tag, the same as in the policy.
func fixLinks(s string) string {
	if !strings.Contains(s, "<a") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 64)
	z := html.NewTokenizer(strings.NewReader(s))
	// openDropped counts `a` start tags removed here, so their matching end tags go too.
	openDropped := 0
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return b.String()
		}
		tok := z.Token()
		if tok.DataAtom == atom.A {
			switch tt {
			case html.StartTagToken, html.SelfClosingTagToken:
				href := attr(tok, "href")
				if !isHTTPS(href) {
					openDropped++
					continue
				}
				tok.Attr = []html.Attribute{{Key: "href", Val: href}, {Key: "rel", Val: LinkRel}}
			case html.EndTagToken:
				if openDropped > 0 {
					openDropped--
					continue
				}
			}
		}
		b.WriteString(tok.String())
	}
}

func attr(t html.Token, key string) string {
	for _, a := range t.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// isHTTPS is the last check on a link before it leaves this package, in either form.
func isHTTPS(href string) bool {
	h := strings.TrimSpace(href)
	return len(h) > len("https://") && strings.EqualFold(h[:len("https://")], "https://")
}
