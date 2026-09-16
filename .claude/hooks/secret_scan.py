"""PreToolUse — BLOCK hardcoded secrets, and real values in template files.  [RULE 8]

Principle: patterns must be HIGH PRECISION. False positives make people disable the hook,
and then the whole layer is gone. Only match when the real value sits in the code; indirect
access (os.Getenv, ${X}, process.env) is never blocked.

Different from v1: .md/.txt are NO LONGER skipped. A secret pasted into documentation gets
COMMITTED — more dangerous than .env.local, which gitignore already blocks. For docs, only
the two highest-precision patterns run.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "secret_scan"

SECRET_PATTERNS = [
    (r"""(?i)\b(password|passwd|pwd|mat_khau)\s*[:=]\s*["'`][^"'`$\{][^"'`]{3,}["'`]""",
     "hardcoded password"),
    (r"""(?i)\b(api[_\-]?key|access[_\-]?key|secret[_\-]?key|client[_\-]?secret|"""
     r"""app[_\-]?secret|jwt[_\-]?secret|server[_\-]?key|signing[_\-]?key)\s*[:=]\s*"""
     r"""["'`][^"'`$\{][^"'`]{7,}["'`]""",
     "hardcoded API key / secret"),
    (r"""-----BEGIN\s+(RSA\s+|EC\s+|DSA\s+|OPENSSH\s+)?PRIVATE\s+KEY-----""",
     "private key block embedded in code"),
    (r"""\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b""",
     "embedded JWT"),
    (r"""(?i)(postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|amqps?):\/\/"""
     r"""[^\s:@\/"']+:[^\s@\/"']{3,}@""",
     "connection string with credentials"),
    (r"""\b(AKIA|ASIA)[0-9A-Z]{16}\b""", "AWS access key"),
]

# For documentation: only the two patterns that essentially never false-positive
DOC_PATTERNS = [SECRET_PATTERNS[2], SECRET_PATTERNS[4]]
DOC_EXT = (".md", ".txt", ".adoc", ".rst")

ENV_KEY_SECRETISH = re.compile(
    r"(SECRET|PASSWORD|PASSWD|_PASS$|TOKEN|CREDENTIAL|PRIVATE_KEY|APIKEY|API_KEY|"
    r"ACCESS_KEY|SERVER_KEY|_KEY$|SALT|SIGNATURE|BYPASS|_DSN$|_URI$|_URL$)", re.I)

PLACEHOLDER = re.compile(
    r"(change[-_]?me|changeme|todo|xxx+|your[-_]|<[^>]*>|\.\.\.|example\.com|"
    r"guest:guest|mock|dummy|placeholder|thay[-_]?the|dien[-_]?vao)", re.I)

HARMLESS_HOST = re.compile(
    r"^(https?|amqps?|postgres(ql)?|mongodb(\+srv)?|redis)?:?/?/?"
    r"(localhost|127\.0\.0\.1|0\.0\.0\.0|db|cache|queue|api|web|nginx|minio)([:/]|$)", re.I)

PUBLIC_PREFIX = re.compile(r"^(NEXT_PUBLIC_|VITE_)", re.I)


def scan_env_example(content: str, path: str, tool: str) -> None:
    """Template files are committed — a real value here ends up in git history forever.

    Scanned by VARIABLE NAME, not by value length: a template legitimately holds long values
    like TZ=Asia/Ho_Chi_Minh, and length-based rules fire constantly until someone disables
    the hook.
    """
    bad = []
    for lineno, line in enumerate(content.splitlines(), 1):
        s = line.strip()
        if not s or s.startswith("#") or "=" not in s:
            continue
        key, _, value = s.partition("=")
        key, value = key.strip(), value.strip().strip('"').strip("'")
        if not value or PUBLIC_PREFIX.match(key):
            continue
        if not ENV_KEY_SECRETISH.search(key):
            continue
        if PLACEHOLDER.search(value) or HARMLESS_HOST.match(value) or " " in value:
            continue
        bad.append(f"line {lineno}: {key}=<{len(value)} chars, no placeholder marker — hidden>")
    if bad:
        c.block(HOOK, f"suspected real value in template {os.path.basename(path)}", bad,
                ["  Real values belong in the runtime environment or a secret store.",
                 "  Templates carry only NAMES + placeholders (empty or change-me-...) + a comment.",
                 "",
                 "  → Rule 8: .claude/rules/critical/8-secrets-config.md"],
                tool=tool, path=path)


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit", "NotebookEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path:
        sys.exit(0)
    content = c.new_content(ti)
    if not content:
        sys.exit(0)

    if path.endswith((".env.example", ".env.sample")):
        scan_env_example(content, path, tool)
        sys.exit(0)

    if c.should_skip(path) or c.is_generated(content, path):
        sys.exit(0)

    patterns = DOC_PATTERNS if path.endswith(DOC_EXT) else SECRET_PATTERNS
    hits = []
    for pattern, label in patterns:
        for m in re.finditer(pattern, content):
            head = m.group(0)[:24].replace("\n", " ")
            hits.append(f"{label}: {head}…")

    if hits:
        c.block(HOOK, f"hardcoded secret in {os.path.basename(path)}", hits,
                ["  A secret that reaches git CANNOT be recalled — history is already copied",
                 "  everywhere. In a multi-commune system, one leaked key affects EVERY commune.",
                 "",
                 "  Correct approach: os.Getenv(\"...\") or a secret store; declare the name in",
                 "  the template file. In documentation, reference the VARIABLE NAME, never a value.",
                 "",
                 "  → Rule 8: .claude/rules/critical/8-secrets-config.md"],
                tool=tool, path=path)

    sys.exit(0)


if __name__ == "__main__":
    main()
