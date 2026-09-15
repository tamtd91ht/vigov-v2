# RULE 8 — Secrets and configuration

A secret that reaches git **cannot be recalled** — the history is already copied to every
clone. In a multi-commune system, one leaked signing key affects **every commune**, not one.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | Secrets live only in a **secret store** or the runtime environment. Never in source, never in documentation |
| 2 | Adding a variable means updating **both** the real config and the template. Templates hold placeholders only |
| 3 | Never read a real config file and print it into chat, a commit message, or documentation. Reference the **variable name** |
| 4 | Variables with a public prefix (`NEXT_PUBLIC_*`) **ship inside the browser bundle** — they never hold secrets |
| 5 | Environment variables are for **platform-wide constants** only. Commune-specific values are read at runtime (rule 1, invariant 10) |
| 6 | Signing and session keys have a documented **lifetime and rotation procedure** |
| 7 | Dangerous flags (auth bypass, demo mode) must be **off before production**, and are reported every session |

## STRICTLY FORBIDDEN

| # | Forbidden |
|---|---|
| 1 | Passwords, API keys, credentialed connection strings or private key blocks hardcoded in source |
| 2 | Real values in a template file |
| 3 | `git add -f` on any env file other than the template |
| 4 | Secrets in documentation, including security documentation |
| 5 | A secret behind a public prefix |

## STOP CONDITIONS

1. Needing a new third-party secret
2. Suspecting a secret has leaked
3. Needing a dangerous flag enabled in a real environment

→ Enforcement: `hooks/secret_scan.py` (BLOCK) · `hooks/session_start.py` (flag warning)
