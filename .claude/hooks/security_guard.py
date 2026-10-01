"""PreToolUse — BLOCK code that downgrades transport, cryptography or browser safety.  [RULE 13]

WHY THIS HOOK EXISTS: TCVN 14423:2026 §5.4.2.4 (encrypt credentials and sensitive data in
transit), §5.17.2.3 (protect the application against common attacks) and §6.16.2.8 (strong,
standard algorithms only). Most of that standard is process, not code — but a handful of
lines are decidable on sight, and every one of them fails silently: the service still works,
every test is still green, and the only symptom is that the protection is gone.

  plaintext gRPC / TLS verification off / sslmode=disable -> credentials cross the wire readable
  md5 · sha1 · des · rc4 · math/rand                       -> a secret that is guessable
  dangerouslySetInnerHTML · innerHTML= · eval · Function() -> script injection into a staff session
  HttpOnly/Secure switched off · CORS "*"                  -> the session cookie leaves its host

WHAT IT DELIBERATELY DOES NOT CHECK: whether MFA, lockout, idle timeout or rate limiting
EXIST. Those are features; a pattern cannot see their absence. `tools/check_security.py`,
`/review-security` and the `security-reviewer` agent own them.

MEASURED BEFORE ENABLING (2026-09-28, commit 812804d): outside tests the patterns match
exactly three places — two `insecure.NewCredentials()` (core/identityclient, core/platformclient)
and one `math/rand/v2` used to spread load across ClamAV hosts. All three sit in
`tools/security_debt.json` with a reason and an expiry. Every mention of
`dangerouslySetInnerHTML` in the repo is inside a comment or a test, which is why comments are
stripped before matching — a hook that fires on the sentence explaining why the pattern is
banned is a hook somebody turns off.

The hook judges only the text an edit INTRODUCES (c.new_content), never lines already on disk.
The repo-wide view is `tools/check_security.py`, for the reason rule 5 needs two shapes.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "security_guard"

GO = (".go",)
WEB = (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs")
YAML = (".yaml", ".yml")

# (pattern, label, file kinds). Labels are English because they land in guard.jsonl and in
# tools/security_debt.json, where they are the key a debt entry is matched on.
PATTERNS: list[tuple[re.Pattern, str, tuple[str, ...]]] = [
    # --- transport (TCVN 5.4.2.4) ---
    (re.compile(r"\binsecure\.NewCredentials\s*\("), "plaintext-grpc", GO),
    (re.compile(r"\bgrpc\.WithInsecure\s*\("), "plaintext-grpc", GO),
    (re.compile(r"\bInsecureSkipVerify\s*:\s*true\b"), "tls-verify-off", GO),
    (re.compile(r"\brejectUnauthorized\s*:\s*false\b"), "tls-verify-off", WEB),
    (re.compile(r"NODE_TLS_REJECT_UNAUTHORIZED\W{0,4}0"), "tls-verify-off", WEB + YAML),
    (re.compile(r"\bsslmode=disable\b"), "db-tls-off", GO + WEB + YAML),
    # --- cryptography (TCVN 6.16.2.8) ---
    (re.compile(r"\"crypto/(?:md5|sha1|des|rc4)\""), "weak-crypto", GO),
    # math/rand is banned outright, not "near a token": deciding whether a value is a secret
    # needs the whole data flow, and a heuristic that guesses wrong in the permissive direction
    # is a guard that looks armed. One legitimate use exists; it carries an exception.
    (re.compile(r"\"math/rand(?:/v2)?\""), "non-crypto-random", GO),
    (re.compile(r"\bMath\.random\s*\("), "non-crypto-random", WEB),
    # --- browser (TCVN 5.17.2.3) ---
    (re.compile(r"\bdangerouslySetInnerHTML\b"), "raw-html", WEB),
    (re.compile(r"\.(?:inner|outer)HTML\s*="), "raw-html", WEB),
    (re.compile(r"(?<![\w.])eval\s*\("), "dynamic-code", WEB),
    (re.compile(r"\bnew\s+Function\s*\("), "dynamic-code", WEB),
    (re.compile(r"\bHttpOnly\s*:\s*false\b|\bSecure\s*:\s*false\b"), "cookie-flag-off", GO),
    (re.compile(r"\bhttpOnly\s*:\s*false\b|\bsecure\s*:\s*false\b"), "cookie-flag-off", WEB),
    (re.compile(r"Access-Control-Allow-Origin[\"'`]\s*,\s*[\"'`]\*[\"'`]"), "cors-wildcard", GO + WEB),
]

# `// @security-exception: <reason>` (or `#` in YAML) on the line or the line above. A reason
# is mandatory — same discipline as `@cross-tenant:` (rule 1) and `@env-ok:` (rule 11).
EXCEPTION = re.compile(r"(?://|#)\s*@security-exception:\s*\S+")
EXCEPTION_EMPTY = re.compile(r"(?://|#)\s*@security-exception:\s*(?:debt=\s*)?$")

# `// @security-exception: debt=<label> …` — the exception is NOT a judgement that the line is
# safe, it is a pointer to a promise in tools/security_debt.json (same file, that label). Needed
# because a plain marker reads as "no violation": a ledger entry written for the same line then
# looked FIXED, and nothing tracked its expiry (core/operatorclient, 2026-10-01). The hook only
# checks the SHAPE; tools/check_security.py checks the entry exists and has not expired.
DEBT_REF = re.compile(r"(?://|#)\s*@security-exception:\s*debt=([A-Za-z0-9][\w.-]*)")
# `debt=` with no usable label: a pointer to nothing. It must NOT excuse the line, or a typo
# would be a silent permanent exemption that no ledger entry ever covers.
DEBT_BROKEN = re.compile(r"(?://|#)\s*@security-exception:\s*debt=(?![A-Za-z0-9])")


def kind_of(path: str) -> tuple[str, ...] | None:
    p = path.lower()
    for kind in (GO, WEB, YAML):
        if p.endswith(kind):
            return kind
    return None


def in_scope(path: str) -> bool:
    # Rule 13 is a ViGov rule: another repo opened from this session has its own conventions
    # and must not be told to write ViGov's escape comment (CLAUDE.md, "the other hooks fire on
    # edits to vihat-miniapp too"). Relative paths stay IN scope — fail closed (c.ngoai_du_an).
    return (kind_of(path) is not None and not c.should_skip(path)
            and not c.is_generated("", path) and not c.ngoai_du_an(path))


def strip_comments(src: str, yaml: bool) -> str:
    """Blank out comments, keeping every newline so line numbers survive.

    Differs from rest_api_guard.strip_comments in one way that matters here: a ' or " string
    ends at the newline. TSX text carries apostrophes ("don't"), and a quote that never closes
    would swallow the rest of the file as "string" — leaving every later comment un-stripped,
    which is exactly the noise this function exists to remove.
    """
    if yaml:
        return "\n".join(re.sub(r"(^|\s)#.*$", r"\1", ln) for ln in src.split("\n"))
    out: list[str] = []
    i, n, quote = 0, len(src), ""
    while i < n:
        ch = src[i]
        if quote:
            if ch == "\n" and quote != "`":
                quote = ""
                out.append(ch)
                i += 1
                continue
            out.append(ch)
            if ch == "\\" and i + 1 < n:
                out.append(src[i + 1])
                i += 2
                continue
            if ch == quote:
                quote = ""
            i += 1
            continue
        if ch in "\"'`":
            quote = ch
            out.append(ch)
            i += 1
            continue
        if src.startswith("//", i):
            while i < n and src[i] != "\n":
                i += 1
            continue
        if src.startswith("/*", i):
            i += 2
            while i < n and not src.startswith("*/", i):
                if src[i] == "\n":
                    out.append("\n")
                i += 1
            i += 2
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def quet(noi_dung: str, path: str) -> list[tuple[int, str]]:
    """The pure half: [(line, label)] for every violation in `noi_dung`. No I/O.

    Kept pure so tools/test_hooks.py and tools/check_security.py call the SAME decision — two
    copies of a pattern list are two lists that drift, and the looser one is the one that runs.
    """
    return _scan(noi_dung, path)[0]


def scan_debt_markers(content: str, path: str) -> list[tuple[int, str, str]]:
    """[(line, pattern label, debt label)] for every violation excused by `debt=<label>`.

    Only markers that actually excuse a violation are returned: a `debt=` marker on a harmless
    line would otherwise keep a ledger entry alive after the code it covered was fixed.
    """
    return _scan(content, path)[1]


def _exception_on(line: str) -> tuple[bool, str | None]:
    """(excused?, debt label or None) for one raw line."""
    if DEBT_BROKEN.search(line):
        return False, None
    m = DEBT_REF.search(line)
    if m:
        return True, m.group(1)
    return bool(EXCEPTION.search(line)), None


def _scan(content: str, path: str) -> tuple[list[tuple[int, str]], list[tuple[int, str, str]]]:
    kind = kind_of(path)
    if kind is None:
        return [], []
    raw = content.split("\n")
    code = strip_comments(content, kind is YAML).split("\n")
    hits: list[tuple[int, str]] = []
    debts: list[tuple[int, str, str]] = []
    for idx, line in enumerate(code):
        for pat, label, kinds in PATTERNS:
            if kind[0] not in kinds or not pat.search(line):
                continue
            here = raw[idx] if idx < len(raw) else ""
            above = raw[idx - 1] if idx >= 1 else ""
            excused, debt = _exception_on(here)
            if not excused:
                excused, debt = _exception_on(above)
            if not excused:
                hits.append((idx + 1, label))
            elif debt:
                debts.append((idx + 1, label, debt))
    return hits, debts


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not in_scope(path):
        sys.exit(0)

    noi_dung = c.new_content(ti)
    if not noi_dung:
        sys.exit(0)

    hits = quet(noi_dung, path)
    empty = [i for i, ln in enumerate(noi_dung.split("\n"), 1) if EXCEPTION_EMPTY.search(ln)]
    if not hits and not empty:
        sys.exit(0)

    details = [f"line {ln}: {label}" for ln, label in hits]
    details += [f"line {ln}: @security-exception with no reason" for ln in empty]
    c.block(
        HOOK,
        f"security baseline downgraded — {os.path.basename(path)}",
        details,
        [
            "  Each of these keeps working and keeps every test green; the only symptom is that",
            "  the protection is gone. TCVN 14423:2026 §5.4.2.4 · §5.17.2.3 · §6.16.2.8.",
            "",
            "  Correct approach:",
            "    plaintext-grpc / tls-verify-off / db-tls-off -> TLS with verification",
            "                                                     (skills/infra-config)",
            "    weak-crypto        -> crypto/sha256, argon2id (core/password), AES-GCM",
            "    non-crypto-random  -> crypto/rand (Go) · crypto.getRandomValues (web)",
            "    raw-html / dynamic-code -> render text nodes; never build code from strings",
            "    cookie-flag-off / cors-wildcard -> host-only, HttpOnly, Secure; allow-list origins",
            "",
            "  Genuinely not a secret or not a trust boundary (load spreading, a test harness)?",
            "    // @security-exception: <specific reason>",
            "  on the line or the line above. Existing debt lives in tools/security_debt.json.",
            "  A known, user-agreed debt instead points at its ledger entry (same file + label):",
            "    // @security-exception: debt=<label> <short reason>",
            "  tools/check_security.py then tracks that entry's expiry.",
            "",
            "  → Rule 13: .claude/rules/critical/13-security-baseline.md",
            "  → Skill:   .claude/skills/security-baseline/SKILL.md",
        ],
        tool=tool,
        path=path,
    )


if __name__ == "__main__":
    main()
