"""PreToolUse — BLOCK logging personal data and hardcoded real personal data.  [RULE 3]

WHY BLOCK RATHER THAN WARN: process logs flow into centralised logging, into backups, into
third-party monitoring. One log.Info(phone) written while debugging stays there and pushes
an entire commune's phone numbers somewhere nobody controls. Decree 13/2023 treats that as
processing personal data without a lawful basis.

Different from v1: .json is NO LONGER skipped — seed data, fixtures and manifests are all
.json and all committed. Skipping is by PATH (/testdata/, /fixtures/), not by extension.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "pii_guard"

PATTERNS = [
    (rf"{c.LOG_CALL}\s*\([^)\n]{{0,200}}{c.PII_TOKEN}",
     "logging personal data / auth secret"),
    (rf"{c.LOG_CALL}\s*\(\s*[`'\"][^`'\"\n]{{0,200}}%[sv][^`'\"\n]{{0,80}}[`'\"]\s*,[^)\n]{{0,80}}{c.PII_TOKEN}",
     "logging personal data through a format string"),
    (rf"{c.LOG_CALL}\s*\(\s*(?:req\.Body|r\.Body|req\.user|dto|payload|body|user|citizen)\s*[,)]",
     "logging a whole body / user — drags in every field"),
    (rf"fmt\.(?:Printf|Sprintf)\s*\(\s*[\"'`][^\"'`\n]{{0,80}}%\+?v",
     "printing a whole struct — drags in every field, personal data included"),
    (c.VN_PHONE, "hardcoded real phone number"),
    (c.VN_CCCD, "hardcoded 12-digit national ID"),
]

WATCH_EXT = (".go", ".ts", ".tsx", ".js", ".jsx", ".json", ".sql", ".yaml", ".yml")

# The first four patterns inspect VARIABLES passed to a log call.
# The last two inspect literal VALUES.
LOG_PATTERN_COUNT = 4

_STR_LIT = re.compile(r"\"(?:\\.|[^\"\\])*\"|`[^`]*`|'(?:\\.|[^'\\])*'")


def in_plain_string(line: str, pos: int) -> bool:
    """True when `pos` falls inside a PLAIN text literal (not a format string).

    WHY THIS MATTERS: `log.Info("otp sent", "code", dt.Code)` logs no personal data at all —
    the word "otp" merely happens to sit in the message. Blocking that is a false positive,
    and a noisy hook is a disabled hook. Conversely `log.Info("%s", phone)` has `phone`
    OUTSIDE the literal, and format strings stay in scope because their arguments are the
    dangerous part.
    """
    for m in _STR_LIT.finditer(line):
        if m.start() < pos < m.end():
            return "%" not in m.group(0)
    return False


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path or not path.endswith(WATCH_EXT) or c.should_skip(path):
        sys.exit(0)

    content = c.new_content(ti)
    if not content or c.is_generated(content):
        sys.exit(0)

    hits = []
    pii_re = re.compile(c.PII_TOKEN)
    for idx, (pattern, label) in enumerate(PATTERNS):
        is_log_pattern = idx < LOG_PATTERN_COUNT
        for m in re.finditer(pattern, content):
            span = m.group(0)

            if is_log_pattern:
                tok = pii_re.search(span)
                if tok:
                    line_start = content.rfind("\n", 0, m.start()) + 1
                    line_end = content.find("\n", m.start())
                    line = content[line_start: line_end if line_end > 0 else len(content)]
                    if in_plain_string(line, m.start() + tok.start() - line_start):
                        continue

            snippet = span.replace("\n", " ").strip()
            digits = re.sub(r"\D", "", snippet)
            if digits and c.SAFE_FAKE.match(digits):
                continue
            if len(snippet) > 90:
                snippet = snippet[:87] + "…"
            hits.append(f"{label}: {snippet}")

    if not hits:
        sys.exit(0)

    c.block(HOOK, f"personal data in {os.path.basename(path)}", hits,
            ["  Process logs flow into centralised logging, backups, and third-party",
             "  monitoring. A line written while debugging stays there.",
             "",
             "  Correct approach:",
             "    - to trace, log a BUSINESS CODE (code, arrival_no) or a MASKED value",
             "    - everything leaving the API goes through MaskPhone / MaskCccd",
             "    - examples and tests use the fake number: 0900000000",
             "",
             "  → Rule 3: .claude/rules/critical/3-personal-data.md",
             "  → Skill: .claude/skills/mask-personal-data/SKILL.md"],
            tool=tool, path=path)


if __name__ == "__main__":
    main()
