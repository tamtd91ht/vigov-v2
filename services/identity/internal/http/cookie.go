package http

import (
	"net/http"
	"time"
)

// CookiePhien is the name of the session cookie. The browser side declares the same name in
// apps/commune-admin/src/lib/session.ts — one name, two ends, no translation layer.
const CookiePhien = "vigov_session"

// datCookiePhien writes the signed session token.
//
// THE ATTRIBUTE THAT IS MISSING HERE IS THE POINT — read this before "completing" the cookie:
//
//	There is deliberately NO Domain attribute. Without one a browser makes the cookie
//	HOST-ONLY: sent back only to the exact host that set it, which is exactly one commune's
//	domain. Setting Domain to a parent domain (".vigov.vn") would send this commune's session
//	to EVERY other commune's subdomain — one line of configuration, harmless-looking, written
//	once, and the whole isolation between two public authorities is gone while every
//	functional test stays green (rule 1, forbidden #3).
//
//	Writing `Domain: r.Host` would also be host-only, but it is worse: it invites the next
//	person to "generalise" it to the parent domain. The absent attribute cannot be widened by
//	accident.
//
// Secure is always true, including in development. A session cookie that travels in clear text
// once is a session that can be replayed; local development gets HTTPS or gets no session.
func datCookiePhien(w http.ResponseWriter, tok string, hetHan time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookiePhien,
		Value:    tok,
		Path:     "/",
		Expires:  hetHan.UTC(),
		MaxAge:   int(time.Until(hetHan).Seconds()),
		HttpOnly: true, // JavaScript must not be able to read a staff session
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// xoaCookiePhien clears a cookie the server has just refused.
//
// Leaving a dead token in the browser means every subsequent request pays the full token
// parse and session lookup to be refused again, and the person sees no sign that they are
// signed out. Same attributes as above — a Set-Cookie that does not match on name, path and
// domain does not delete anything.
func xoaCookiePhien(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookiePhien,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}
