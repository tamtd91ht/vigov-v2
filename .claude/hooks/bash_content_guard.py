"""PreToolUse (Bash) — BLOCK file writes that ROUTE AROUND the content hooks.

WHY THIS HOOK EXISTS — the biggest hole in any brain that only hooks Edit|Write:

    the same line of code,
      written via Write   → secret_scan / pii_guard BLOCK it
      written via sed -i  → no hook runs, it LANDS

An agent blocked at Write very naturally tries another route. Having to say in prose what
the tooling should have locked is the signature of a leaky enforcement layer.

This hook extracts the content about to be written out of the shell command and re-runs the
secret_scan and pii_guard patterns over it. `settings.json` already denies sed/awk/perl/tee
at the permission layer; this is the second line, catching routes no deny-list enumerates
(heredocs, redirection, python -c).
"""

from __future__ import annotations

import re
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "bash_content_guard"

# Ways a shell command writes a file
WRITE_FORMS = [
    (re.compile(r"<<-?\s*['\"]?(\w+)['\"]?"), "heredoc"),
    (re.compile(r"(?<![0-9<>])>>?\s*[\w./\-]+"), "redirect to file"),
    (re.compile(r"\btee\b"), "tee"),
    (re.compile(r"\bsed\b[^|]*\s-i\b|\bsed\b\s+-i"), "sed -i"),
    (re.compile(r"\bperl\b[^|]*\s-p?i\b"), "perl -i"),
    (re.compile(r"\bpython3?\b\s+-c\b"), "python -c"),
    (re.compile(r"\bnode\b\s+-e\b"), "node -e"),
    (re.compile(r"\bprintf\b[^|]*>"), "printf >"),
]

# Secrets — same set as secret_scan
SECRETS = [
    (re.compile(r"""(?i)\b(password|passwd|pwd|mat_khau)\s*[:=]\s*["'`][^"'`$\{][^"'`]{3,}["'`]"""),
     "hardcoded password"),
    (re.compile(r"""(?i)\b(api[_\-]?key|secret[_\-]?key|client[_\-]?secret|jwt[_\-]?secret|server[_\-]?key)\s*[:=]\s*["'`][^"'`$\{][^"'`]{7,}["'`]"""),
     "hardcoded API key / secret"),
    (re.compile(r"-----BEGIN\s+(RSA\s+|EC\s+|OPENSSH\s+)?PRIVATE\s+KEY-----"),
     "private key block"),
    (re.compile(r"""(?i)(postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|amqps?):\/\/[^\s:@\/"']+:[^\s@\/"']{3,}@"""),
     "connection string with credentials"),
    (re.compile(r"\b(AKIA|ASIA)[0-9A-Z]{16}\b"), "AWS access key"),
    (re.compile(r"\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b"),
     "embedded JWT"),
]

# Personal data — same set as pii_guard
PIIS = [
    (re.compile(rf"{c.LOG_CALL}\s*\([^)\n]{{0,200}}{c.PII_TOKEN}"), "logging personal data"),
    (re.compile(c.VN_PHONE), "hardcoded real phone number"),
    (re.compile(c.VN_CCCD), "hardcoded national ID"),
]

# Read-only commands — not a write path, skip cheaply
READ_ONLY_HEAD = re.compile(r"^\s*(cat|head|tail|grep|rg|ls|wc|git\s+(log|diff|show|status))\b")


def looks_like_write(cmd: str) -> str | None:
    if READ_ONLY_HEAD.match(cmd) and ">" not in cmd:
        return None
    for pat, name in WRITE_FORMS:
        if pat.search(cmd):
            return name
    return None


def scan(cmd: str) -> list[str]:
    hits: list[str] = []
    for pat, label in SECRETS:
        if pat.search(cmd):
            hits.append(f"{label} — in the content about to be written")
    for pat, label in PIIS:
        for m in pat.finditer(cmd):
            digits = re.sub(r"\D", "", m.group(0))
            if digits and c.SAFE_FAKE.match(digits):
                continue          # agreed fake number used in examples
            hits.append(f"{label} — in the content about to be written")
            break
    return hits


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    if c.tool_of(data) != "Bash":
        sys.exit(0)

    cmd = (c.input_of(data).get("command") or "")
    if not cmd:
        sys.exit(0)

    form = looks_like_write(cmd)
    if not form:
        sys.exit(0)

    hits = scan(cmd)
    if not hits:
        sys.exit(0)

    c.block(
        HOOK,
        f"shell write ({form}) carrying forbidden content",
        list(dict.fromkeys(hits)),
        [
            "  Writing files through the shell bypasses secret_scan and pii_guard. That detour",
            "  is precisely why this hook exists.",
            "",
            "  Correct approach:",
            "    - use Write / Edit so every check runs",
            "    - read secrets from the environment or a secret store, never inline",
            "    - use the agreed fake number in examples: 0900000000",
            "",
            "  → Rule 3: .claude/rules/critical/3-personal-data.md",
            "  → Rule 8: .claude/rules/critical/8-secrets-config.md",
        ],
        tool="Bash",
        path="",
    )


if __name__ == "__main__":
    main()
