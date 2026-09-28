# RULE 13 — Security baseline (TCVN 14423:2026)

The national cybersecurity baseline. Most of it is process and infrastructure; this rule holds
only what a machine can decide. The rest, and every threshold: `skills/security-baseline`.

| # | Invariant |
|---|---|
| 1 | Traffic to other services and data stores is encrypted and verified |
| 2 | Standard, strong cryptography only; secrets come from a CSPRNG |
| 3 | Browser code never renders raw HTML or builds code from strings |
| 4 | Session cookies stay host-only, `HttpOnly`, `Secure`; CORS is an allow-list |
| 5 | Every Next.js app sends CSP, HSTS, nosniff, Referrer-Policy and a framing policy |
| 6 | No third-party code with a known called or high-severity vulnerability ships |
| 7 | Every unauthenticated route declares a rate limit — enforced once `core/ratelimit` exists |

Breaches already on disk sit in `tools/security_debt.json` / `tools/vuln_exceptions.json`
with a reason and an expiry. Expired, or fixed but still listed = red.

| # | Forbidden |
|---|---|
| 1 | `insecure.NewCredentials` · `InsecureSkipVerify: true` · `sslmode=disable` |
| 2 | md5 · sha1 · des · rc4 · `math/rand` · `Math.random` |
| 3 | `dangerouslySetInnerHTML` · `innerHTML =` · `eval` · `new Function` |
| 4 | Silently loosening a threshold: password, lockout, session, log retention |

Escape: `// @security-exception: <reason>`, reviewed by `security-reviewer`.

**STOP — ask the user:** loosening or choosing any security threshold (open questions #35–#39)
· a new unauthenticated route · a new unencrypted channel · changing a hash or cipher ·
sending security logs to an outside service.

→ Enforcement: `hooks/security_guard.py` (BLOCK) · `tools/check_security.py` (`make check`) ·
`tools/check_vuln.py` (`make vuln`, Jenkins gate + nightly)
→ Skills: `skills/security-baseline` · `skills/security-logging` · Review: `/review-security`
