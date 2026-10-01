---
name: security-baseline
description: Use when touching anything TCVN 14423:2026 (national cybersecurity baseline) constrains — transport encryption, TLS, cryptography, random values, security headers, CSP, cookies, CORS, raw HTML, third-party dependencies, vulnerability scanning, rate limiting, security thresholds. Triggers on: TCVN, 14423, an ninh mạng, an toàn thông tin, bảo mật, security, cấp độ, TLS, mTLS, gRPC credentials, sslmode, InsecureSkipVerify, crypto, md5, sha1, math/rand, CSP, HSTS, header, cookie, CORS, XSS, dangerouslySetInnerHTML, eval, govulncheck, npm audit, CVE, dependency, thư viện, rate limit, brute force, dò mật khẩu, security_debt, vuln_exceptions, @security-exception.
---

# Skill: Security baseline (TCVN 14423:2026)

Rule 13 holds what a machine can decide. This skill holds **how** to do it right, and the
thresholds the standard sets — which are **not decisions yet**: open questions #35–#39.

## 1. Who owns what — do not ask code to solve an organisation's problem

About one third of the standard touches source code. The rest is infrastructure (network
zones, WAF, backup, EDR) or organisation (risk process, training, incident team, supplier
list). When a requirement is not code, say so and name the owner; never write code or
documentation to "cover" it.

| Group (level 3 §) | Code here | Infra / ops | Organisation |
|---|---|---|---|
| 5.4 information assets — classify, encrypt, integrity, signatures | ✔ | ✔ | ✔ |
| 5.5 secure configuration — session lock, login lockout | ✔ | ✔ | |
| 5.6 accounts — password, MFA, 45-day inactive, least privilege | ✔ | | ✔ |
| 5.7 vulnerabilities · 5.17 secure development | ✔ | ✔ | ✔ |
| 5.8 security logs | ✔ (emit) | ✔ (ship, keep) | ✔ (review) |
| 5.11 backup · 5.12 network · 5.13 monitoring | | ✔ | |
| 5.1 risk · 5.14 staff · 5.15 suppliers · 5.16 incidents · 5.18 pentest | | | ✔ |

## 2. Thresholds — the standard's numbers, NOT this project's decisions

Write none of these into code before the matching open question is DECIDED. When one is,
it becomes ONE constant or per-commune setting read from one place — never a literal repeated
in handlers.

| Threshold | Level 3 | Level 4 | Open question |
|---|---|---|---|
| Idle session lock — business software handling important data | ≤ 15 min | same | #38 |
| Idle session lock — administration sessions | ≤ 5 min | same | #38 |
| Failed logins before lockout (business software) | ≤ 5 | same | #39 |
| Lockout duration | 12 h – 30 days, plus emergency admin unlock | same | #39 |
| Password length with MFA / without MFA | ≥ 8 / ≥ 14 + 4 character classes | same | #36 |
| Admin password rotation · history | every 2 months · last 10 | same | #36 |
| MFA | from Internet, third parties, admin accounts | same | #37 |
| Disable inactive accounts | 45 days | same | — (standard is explicit) |
| Account list review · data-permission review | 6 months | quarterly | #35 |
| Security log retention (central) | ≥ 3 months | ≥ 6 months, tamper-proof | #35 |
| Vulnerability scan | 6 months | 6 months + quarterly for critical assets | #35 |
| Pentest | programme | external + internal, yearly | #35 |

## 3. Transport — encrypted AND verified (rule 13 inv. 1)

| Channel | Right | Wrong |
|---|---|---|
| gRPC between services | `credentials.NewTLS(&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})`, or a mesh doing mTLS | `insecure.NewCredentials()` |
| PostgreSQL | `sslmode=verify-full` (`require` encrypts but does not verify the host) | `sslmode=disable` |
| Redis · Kafka · MinIO | `rediss://` · SASL_SSL · `https://` endpoint | plaintext, or TLS with verification off |

Addresses stay cluster-shaped and are read in `core/config` only (rule 11,
`skills/infra-config`). A CA bundle is a ConfigMap; a client key is a Secret.

## 4. Cryptography (inv. 2)

| Need | Use | Never |
|---|---|---|
| Password | argon2id via `core/password` | any fast hash |
| Token, lookup code, OTP, nonce, salt | `crypto/rand` (Go) · `crypto.getRandomValues` (web) | `math/rand`, `Math.random` |
| Integrity of a stored file | SHA-256 (ADR 0052) | md5, sha1 |
| Encrypting a per-commune secret | AES-256-GCM envelope, ADR 0009 | a home-made scheme |
| Signing an exchanged official document | a licensed digital-signature provider (TCVN §5.4.2.8) — needs an ADR | — |

A non-security use of `math/rand` (load spreading, jitter) takes
`// @security-exception: <why no secret derives from it>`.

## 5. Browser (inv. 3–5)

- Render user text as React text nodes. Formatting a note → split lines into elements.
- Headers, in `next.config.ts` `headers()` or at the ingress, for **every** route:

```ts
async headers() {
  return [{ source: "/:path*", headers: [
    { key: "Content-Security-Policy", value: "default-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'" },
    { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
    { key: "X-Content-Type-Options", value: "nosniff" },
    { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  ]}];
}
```

  Next.js inline scripts need a nonce issued per request; start with
  `Content-Security-Policy-Report-Only`, read the reports, then enforce. `citizen-app` is
  exempt: Zalo serves its pages, and no file here can set their headers.
- State-changing requests: SameSite=Lax is the floor; also refuse a foreign `Origin`.

## 6. Third-party code (inv. 6)

Before adding a module or package: is it maintained, is its licence compatible, does
`make vuln` stay green. Lockfiles are committed. A finding you cannot fix yet goes into
`tools/vuln_exceptions.json` with reason, mitigation and expiry — the user sees it in the diff.
Upgrading the Go toolchain is the fix for standard-library findings.

## 7. Rate limiting (inv. 7 — enforced from phase 2)

Redis, key `t:<tenant_id>:rl:<route>:<ip-or-account>` (rule 1 invariant 7). Limit by source
address AND by target account on login and password endpoints — by address alone lets a
botnet guess one chair's password; by account alone lets anyone lock the chair out. Routes
will declare it next to `idem.*`, the same way duplicate-request protection is declared.

## 8. The debt ledger

`tools/security_debt.json` lists what was already broken when rule 13 arrived. Fixing one:
remove the violation AND its entry in the same commit. Adding one needs the user's agreement
and an expiry — it is a promise with a date, not a place to park findings.

A debt the user agreed to **at a specific line** is marked there with a pointer, not a plain
exception: `// @security-exception: debt=<label> <short reason>`, plus an entry with the same
`file` and `label` in the ledger. `check_security.py` then treats the entry as live (never
FIXED while the marker stands), turns red when it expires, and turns red (MISSING) when the
marker names no entry. A plain `@security-exception: <reason>` means "not a violation" and is
never tracked — do not use it for a debt.

→ Rule 13 · `skills/security-logging` · `skills/session-and-token` · `/review-security`
