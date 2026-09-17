"""Stop — BLOCK ending the session when code was changed but never verified.

WHY THIS HOOK EXISTS: every brain carries the line "verify, do not just declare" — and
almost none ENFORCES it. Eight hooks guard the INPUT (do not write the wrong thing); none
guards the OUTPUT (do not say done while it is red).

For a 100%-AI-written project, "agent declares done while tests fail" is a MORE COMMON
failure than "agent writes a secret into the code". This hook patches exactly that.

How: read the session transcript and compare two facts —
  A. was any Edit/Write applied to source code
  B. was any verification run AFTER the last code edit
A without B  →  block.

Safety: `stop_hook_active` true means this hook already blocked once — let it through so
the session cannot loop.
"""

from __future__ import annotations

import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "stop_verify_guard"

CODE_EXT = (".go", ".ts", ".tsx", ".js", ".jsx", ".proto", ".sql")
CODE_DIR = ("/internal/", "/core/", "/cmd/", "/proto/", "/migrations/", "/src/",
            "/migrations/")

# Signals that a verification run FAILED. Matching the command alone let a red `go test`
# satisfy the gate -- the exact failure this hook exists to stop.
FAIL_SIGNAL = re.compile(
    r'("is_error"\s*:\s*true|FAIL|--- FAIL|fail(?:ed|ure)|'
    r'exit (?:code|status) [1-9]|npm ERR!|error:)',
    re.IGNORECASE,
)

VERIFY_CMD = re.compile(
    r"\b(make\s+(check|test|verify)|go\s+test|go\s+vet|golangci-lint\s+run|"
    r"buf\s+lint|npm\s+(test|run\s+(test|typecheck|lint|build))|npx\s+tsc)\b"
)


def is_code(path: str) -> bool:
    # Normalise to a leading slash: transcripts record RELATIVE paths ("apps/x/y.tsx"),
    # while CODE_DIR entries are written "/apps/". Without this, a relative top-level dir
    # never matched and edits there were invisible to the gate.
    p = (path or "").replace("\\", "/")
    if not p.endswith(CODE_EXT) or c.should_skip(p):
        return False
    if not p.startswith("/"):
        p = "/" + p
    return any(d in p for d in CODE_DIR)


def walk_transcript(path: str) -> tuple[int, int, list[str], int]:
    """(last code edit, last PASSING verification, files touched, last FAILING verification)."""
    last_edit = last_verify = last_verify_failed = -1
    touched: list[str] = []
    if not path or not os.path.exists(path):
        return last_edit, last_verify, touched, last_verify_failed

    step = 0
    try:
        with open(path, encoding="utf-8", errors="ignore") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    rec = json.loads(line)
                except Exception:
                    continue
                step += 1
                blob = json.dumps(rec, ensure_ascii=False)

                if '"Bash"' in blob and VERIFY_CMD.search(blob):
                    # Only a PASSING run counts as verification.
                    if FAIL_SIGNAL.search(blob):
                        last_verify_failed = step
                    else:
                        last_verify = step

                if any(t in blob for t in ('"Edit"', '"Write"', '"MultiEdit"')):
                    for m in re.finditer(r'"file_path"\s*:\s*"([^"]+)"', blob):
                        p = m.group(1).replace("\\\\", "/")
                        if is_code(p):
                            last_edit = step
                            base = os.path.basename(p)
                            if base not in touched:
                                touched.append(base)
    except Exception:
        pass

    return last_edit, last_verify, touched, last_verify_failed


def main() -> None:
    c.utf8_streams()
    data = c.read_input()

    if data.get("stop_hook_active"):
        sys.exit(0)                      # already blocked once — do not loop

    last_edit, last_verify, touched, last_failed = walk_transcript(
        data.get("transcript_path") or "")

    if last_edit < 0:
        sys.exit(0)                      # no code was changed
    if last_verify > last_edit:
        sys.exit(0)                      # verified AFTER the last edit

    if last_failed > last_edit:
        reason = "verification ran after the last edit and FAILED"
    elif last_verify < 0:
        reason = "verification never ran"
    else:
        reason = "verification ran, but BEFORE the last code edit"

    c.warn(
        HOOK,
        f"this session changed code but {reason}",
        [f"touched: {t}" for t in touched[:8]],
        [
            "  ViGov is a government system. Declaring done without verifying moves the risk",
            "  onto the operators, who have no way to know.",
            "",
            "  Run before finishing:",
            "    make check          # gofmt + go vet + golangci-lint + go test + buf lint",
            "    make test           # tests only",
            "",
            "  Tests red and you still must stop: SAY WHICH test is red and why, then stop.",
            "  Never pass over it in silence.",
            "",
            "  → CLAUDE.md, section 'Before saying it is done'",
        ],
        tool="Stop",
        path="",
    )


if __name__ == "__main__":
    main()
