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
CODE_DIR = ("/services/", "/web/", "/internal/", "/pkg/", "/cmd/", "/proto/", "/migrations/")

VERIFY_CMD = re.compile(
    r"\b(make\s+(check|test|verify)|go\s+test|go\s+vet|golangci-lint\s+run|"
    r"buf\s+lint|npm\s+(test|run\s+(test|typecheck|lint|build))|npx\s+tsc)\b"
)


def is_code(path: str) -> bool:
    p = (path or "").replace("\\", "/")
    if not p.endswith(CODE_EXT) or c.should_skip(p):
        return False
    return any(d in p for d in CODE_DIR)


def walk_transcript(path: str) -> tuple[int, int, list[str]]:
    """(step of last code edit, step of last verification, files touched)."""
    last_edit = last_verify = -1
    touched: list[str] = []
    if not path or not os.path.exists(path):
        return last_edit, last_verify, touched

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

    return last_edit, last_verify, touched


def main() -> None:
    c.utf8_streams()
    data = c.read_input()

    if data.get("stop_hook_active"):
        sys.exit(0)                      # already blocked once — do not loop

    last_edit, last_verify, touched = walk_transcript(data.get("transcript_path") or "")

    if last_edit < 0:
        sys.exit(0)                      # no code was changed
    if last_verify > last_edit:
        sys.exit(0)                      # verified AFTER the last edit

    reason = ("verification never ran" if last_verify < 0
              else "verification ran, but BEFORE the last code edit")

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
