// Package richtext is the ONE place a Mini App article body is made safe (ADR 0067 §1, amended by
// §Sửa đổi 03/10/2026).
//
// ONE SANITISER PER WRITE SOURCE, ONE PIPELINE (K1 — replaced B1's "one function" on 03/10/2026):
//
//	Sanitize       arbitrary HTML → HTML holding only the NARROW allow-list (§1 decision 1). The PORTAL
//	               sync's write path, and only that one: portal images live on every host, and taking
//	               them is H4's later batch, never a widening here (amendment stop condition 2).
//	SanitizeStaff  arbitrary HTML → the narrow list PLUS the three staff shapes (body image by FILE ID,
//	               quote, byline). Staff compose/edit, and AGAIN on the public read (K7), because rows
//	               written before this package existed are archival records that are never rewritten
//	               (rule 7) and still hold whatever markup was typed. A portal row passes it unchanged:
//	               it was stored narrow, and narrow HTML is a fixed point of the staff policy.
//	Blocks         sanitised HTML → a structure (paragraph · heading · list · image · quote · byline ·
//	               inline runs). The Mini App renders THIS and never markup (rule 13 forbidden #3): a
//	               client that is never handed HTML cannot be updated into rendering it.
//
// WHY A PACKAGE OF ITS OWN AND NOT internal/domain, although the card that built it said "domain":
// internal/domain imports the standard library only (skills/go-service-pattern, rule 1 of that table),
// and both halves here stand on third-party parsers. A sanitiser written from scratch over the standard
// library is how sanitisers get written wrongly. Every write path names the policy of ITS source at the
// call site; a write path that skips the one of its source is the amendment's stop condition #1.
package richtext

import (
	"regexp"
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

// The policies are built once: a bluemonday.Policy is safe for concurrent use AFTER construction, and
// building one per call would rebuild the same maps on every article.
var (
	policyOnce sync.Once
	policy     *bluemonday.Policy

	staffOnce   sync.Once
	staffPolicy *bluemonday.Policy
)

// baseList is §1 decision 1, EXACTLY: p, br, strong, em, ul, ol, li, h2, h3, and a with an https href.
// Both policies start here, so the narrow list cannot drift between them.
func baseList() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "em", "ul", "ol", "li", "h2", "h3")
	p.AllowAttrs("href").OnElements("a")
	// https ONLY. Relative and protocol-relative links are refused (AllowRelativeURLs stays false):
	// `//host/x` leaves for whatever host it names, and a relative path means nothing inside a Mini
	// App that has no origin of its own.
	p.AllowURLSchemes("https")
	p.RequireParseableURLs(true)
	return p
}

// allowList is the PORTAL write policy: baseList and nothing more. Adding img, figure or any other tag
// or attribute here is the amendment's stop condition #2 (H4) — the customer's call, never a line
// added here.
func allowList() *bluemonday.Policy {
	policyOnce.Do(func() { policy = baseList() })
	return policy
}

// fileIDShape is a 26-character ULID in core/ulid's alphabet (Crockford base32, upper case: no I, L,
// O, U). The body stores the FILE ID of an image, never a URL (K2): the public URL is resolved by the
// server at read time, for this commune and this article only.
var fileIDShape = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

// bylineValue is the only value `data-role` may hold, and only on a top-level `p`.
const bylineValue = "byline"

// staffList is the STAFF policy (K1): baseList plus the element and attribute SET of the three staff
// shapes. bluemonday knows sets, not positions; WHERE each may stand (an img only inside a figure, a
// figcaption only inside one, a blockquote holding paragraphs) is enforceStaffShapes' job.
//
// Never taken (amendment stop condition 3): `src` or any URL on img, iframe, style, class, event
// attributes, any scheme but https.
func staffList() *bluemonday.Policy {
	staffOnce.Do(func() {
		p := baseList()
		p.AllowElements("figure", "figcaption", "blockquote")
		p.AllowAttrs("data-file-id").Matching(fileIDShape).OnElements("img")
		p.AllowAttrs("alt").OnElements("img")
		p.AllowAttrs("data-role").Matching(regexp.MustCompile(`^` + bylineValue + `$`)).OnElements("p")
		staffPolicy = p
	})
	return staffPolicy
}

// Sanitize is the PORTAL policy: html reduced to the narrow allow-list. An `a` whose href does not
// survive loses the tag and keeps its text. The content of script/style is dropped whole (bluemonday's
// default), never printed as text.
//
// THE OUTPUT IS STILL HTML. It is what is stored and what staff screens edit (Tiptap reads it); the
// Mini App gets Blocks(SanitizeStaff(x)), never this string.
func Sanitize(s string) string {
	if s == "" {
		return ""
	}
	return fixLinks(allowList().Sanitize(s))
}

// SanitizeStaff is the STAFF policy, for staff POST/PATCH and for the public read (K1, K7): Sanitize
// plus exactly these shapes — the contract web-admin's editor schema mirrors:
//
//	<figure><img data-file-id="<ULID>" alt="…"><figcaption>…inline…</figcaption></figure>
//	<blockquote><p>…inline…</p>…</blockquote>
//	<p data-role="byline">…inline…</p>
//
// figcaption and alt are optional (H3). A `src` on the img is removed, never kept: the stored body names
// a file, and a URL a client sent is a host every resident's phone would be sent to (K2). A figure
// without a valid img is dropped WHOLE, caption included; an img anywhere but directly inside a
// top-level figure is dropped. HTML holding none of the three shapes comes out byte-for-byte as
// Sanitize gives it.
func SanitizeStaff(s string) string {
	if s == "" {
		return ""
	}
	out := fixLinks(staffList().Sanitize(s))
	if !hasStaffShape(out) {
		return out
	}
	return enforceStaffShapes(out)
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

// isHTTPS is the last check on a link before it leaves this package, in either form: an https URL with
// a non-empty authority and NO USERINFO (R5, 02/10/2026 — the same rule as domain.NormalizeLinkTo).
// `https://thangbinh.danang.gov.vn@evil.example/` names evil.example; shown in an article of a public
// authority, the part before `@` reads as the government host the resident is about to visit. The
// authority ends at the first `/ ? #` — or `\`, which browsers read as `/` in an https URL, so a `\@`
// cannot hide an `@` from this check while the browser sees no userinfo.
func isHTTPS(href string) bool {
	const scheme = "https://"
	h := strings.TrimSpace(href)
	if len(h) <= len(scheme) || !strings.EqualFold(h[:len(scheme)], scheme) {
		return false
	}
	rest := h[len(scheme):]
	end := strings.IndexAny(rest, `/?#\`)
	if end < 0 {
		end = len(rest)
	}
	authority := rest[:end]
	return authority != "" && !strings.Contains(authority, "@")
}
