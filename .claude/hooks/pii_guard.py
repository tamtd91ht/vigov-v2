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

# `[^)]` AND NOT `[^)\n]` — the newline exclusion was the whole defect, and it is the shape
# this repository keeps calling "green by luck": the pattern went quiet because the statement
# was longer than its own limit, not because the statement was clean.
#
# A structured log call with three key/value pairs is written across lines, and that is the
# normal way, not an exotic one:
#
#     slog.Error("khong gui duoc",
#         "so_dien_thoai", ct.DienThoai,     <- invisible to `[^)\n]`
#         "err", err)
#
# The bound does not get looser by allowing the newline: `[^)]` still cannot run past the first
# `)`, so the window is still exactly the argument list of one call. What changes is that the
# window now reaches the arguments that were pushed onto the next line.
PATTERNS = [
    (rf"{c.LOG_CALL}\s*\([^)]{{0,200}}{c.PII_TOKEN}",
     "logging personal data / auth secret"),
    (rf"{c.LOG_CALL}\s*\(\s*[`'\"][^`'\"\n]{{0,200}}%[sv][^`'\"\n]{{0,80}}[`'\"]\s*,[^)]{{0,80}}{c.PII_TOKEN}",
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


def mien_tru(line: str, vi_tri: int) -> bool:
    """The two places a PII WORD is not a PII VALUE: plain prose in a literal, and a comment."""
    return in_plain_string(line, vi_tri) or trong_chu_thich(line, vi_tri)


def trong_chu_thich(line: str, vi_tri: int) -> bool:
    """True when `vi_tri` sits after a `//` that is not itself inside a string literal.

    A COMMENT IS PROSE, NOT AN ARGUMENT — the same reasoning as `in_plain_string`, one level
    out. It became necessary the moment the log-call window was allowed to cross a newline,
    and the measurement said so immediately: 710 real files, and two of the three new reports
    were this, verbatim from `service-identity/cmd/server/main.go:368`:

        log.Info("khởi động", "service", "identity", "addr", cfg.ListenAddr,
            // secret.DSN redacts the password on every rendering path and keeps the host, so
            // this line still says which database was opened (rule 8).
            "dsn", cfg.DatabaseDSN)

    The hook would have accused the line for EXPLAINING WHY IT IS SAFE. This repository has
    the precedent written down already — drift_guard did exactly that to the most carefully
    reasoned migration in the tree, and the lesson recorded was "hook nhiễu là hook bị tắt".

    It opens no hole: a comment does not execute, so no personal data leaves through one. A
    real phone number or national ID written into a comment is still blocked, by the VALUE
    patterns (VN_PHONE / VN_CCCD), which do not go through this exemption.
    """
    vung = [m.span() for m in _STR_LIT.finditer(line)]
    i = line.find("//")
    while i != -1:
        if not any(a <= i < b for a, b in vung):
            return vi_tri > i
        i = line.find("//", i + 2)
    return False


def dong_quanh(content: str, vi_tri: int) -> tuple[str, int]:
    """(the line holding `vi_tri`, the offset of `vi_tri` inside it).

    Computed from the TOKEN, not from the start of the log call. Now that a match may span
    several lines, taking the call's line would hand `in_plain_string` a line the token is not
    even on — a literal on one line would then decide for a bare variable on another.
    """
    dau = content.rfind("\n", 0, vi_tri) + 1
    cuoi = content.find("\n", vi_tri)
    return content[dau: cuoi if cuoi > 0 else len(content)], vi_tri - dau


def da_che(span: str, vi_tri: int) -> bool:
    """True when the token at `vi_tri` is an argument of a Mask… call.

    Rule 3 invariant 3 names masking as the CORRECT way to let a value out, and this hook's own
    block message tells the author to "log a MASKED value". Without this exemption the widened
    token list in `_common.PII_TOKEN` reports the very line the rule asks for — and a guard that
    is wrong about the right answer is a guard somebody switches off within the week.

    Narrow by construction: the mask call must open BEFORE the token and must not have closed in
    between, so `MaskPhone(a), cb.HoTen` still reports `HoTen`. A variable merely named `masked`
    proves nothing and is not accepted — the claim has to be visible at the call site, the same
    discipline `// @cross-tenant:` follows.
    """
    truoc = span[:vi_tri]
    cuoi = None
    for m in c.MASK_CALL.finditer(truoc):
        cuoi = m.end()
    return cuoi is not None and ")" not in truoc[cuoi:]


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
    if not content or c.is_generated(content, path):
        sys.exit(0)

    hits = []
    pii_re = re.compile(c.PII_TOKEN)
    for idx, (pattern, label) in enumerate(PATTERNS):
        is_log_pattern = idx < LOG_PATTERN_COUNT
        for m in re.finditer(pattern, content):
            span = m.group(0)

            if is_log_pattern:
                # EVERY token in the window, not just the first one. Judging only
                # `pii_re.search(span)` meant the FIRST token decided for all of them — and in
                # structured logging the first token is the KEY, written as a plain literal:
                #
                #     slog.Error("khong gui duoc", "so_dien_thoai", ct.DienThoai, "err", err)
                #                                   ^ exempt, correctly   ^ never looked at
                #
                # So the key name shielded the value beside it, which is the one arrangement
                # that carries real personal data into the log.
                #
                # `toks and` is load-bearing: patterns 3 and 4 ("a whole body / user", "%+v")
                # carry no PII token at all, and an empty `all()` is True — which would have
                # switched those two off in silence.
                toks = list(pii_re.finditer(span))
                if toks and all(
                        da_che(span, tok.start())
                        or mien_tru(*dong_quanh(content, m.start() + tok.start()))
                        for tok in toks):
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
