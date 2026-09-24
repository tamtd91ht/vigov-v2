"""SessionStart + PostToolUse(shell) — keep the codegraph index of THIS repo current.

WHY THIS HOOK EXISTS — measured 2026-09-24, not reasoned:
  * the codegraph MCP server is configured GLOBALLY with `--path` pointing at another project,
    so its file watcher never sees this repo; nothing updated the index here at all
  * two functions added after `codegraph init` were missing from it, while
    `codegraph status` printed "Index is up to date" — the status cannot be trusted
  * `codegraph sync` took ~1.1 s and fixed it; two concurrent syncs both exited 0
So instead of CHECKING freshness (which lies), it simply syncs, at the two moments that matter:

  SessionStart   the scouts of ROUTING §0.2 read the graph right after — and code from another
                 machine only ever arrives by `git pull`, which no post-commit hook would catch
  after a git command that moves the tree (commit, pull, merge, checkout, rebase, …)

SAFETY — this is a SIDE-EFFECT hook, not a guard, so it inverts the usual contract:
  * it NEVER blocks and never exits 2 on the success path
  * no `.codegraph/` (not initialised) or no `codegraph` binary (Jenkins, a fresh machine)
    → silent no-op; SessionStart prints one hint line, nothing more
  * bounded by TIMEOUT; a hang or a failure is reported ONCE (PostToolUse: exit 2 only puts the
    message in front of the agent, the tool call already happened) and logged to guard.jsonl
  * uncommitted edits between git commands are NOT synced here — ROUTING §0.1 covers that
"""

from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "codegraph_sync"

# ~1 s measured; 30 s leaves room for a large pull while never holding a session hostage.
TIMEOUT = 30

# Git subcommands after which the working tree may hold different code. Read-only ones
# (status, log, diff, show, fetch) are deliberately absent: syncing after them is pure cost.
DOI_CAY = re.compile(
    r"\bgit\s+(?:-C\s+\S+\s+)?"
    r"(commit|pull|merge|checkout|switch|rebase|reset|cherry-pick|revert|am|restore|"
    r"stash\s+(?:pop|apply))\b"
)


def can_sync_sau(lenh: str) -> bool:
    """True when this shell command may have changed the code on disk."""
    return bool(DOI_CAY.search(lenh or ""))


def tim_codegraph() -> str | None:
    # `shutil.which` honours PATHEXT, so it finds `codegraph.cmd` on Windows as well.
    return shutil.which("codegraph")


def dong_bo(root: str) -> str | None:
    """Run the sync. None = done or nothing to do; a string = what went wrong (no content)."""
    if not os.path.isdir(os.path.join(root, ".codegraph")):
        return None
    exe = tim_codegraph()
    if not exe:
        return None
    try:
        r = subprocess.run([exe, "sync", "-q", root], capture_output=True, timeout=TIMEOUT,
                           cwd=root)
    except subprocess.TimeoutExpired:
        return f"hết {TIMEOUT}s mà chưa xong"
    except Exception as ex:
        return f"không chạy được: {type(ex).__name__}"
    return None if r.returncode == 0 else f"exit {r.returncode}"


def bao_loi(loi: str, tool: str) -> None:
    c.log_guard(HOOK, tool, "", f"sync hỏng — {loi}")
    c.warn(
        HOOK,
        f"codegraph sync thất bại ({loi}) — chỉ mục codegraph có thể đã CŨ",
        [],
        [
            "  Không chặn gì. Nhưng scout đọc codegraph sẽ đọc một bản đồ cũ, không báo lỗi.",
            "  Chạy tay:  codegraph sync .     (Windows: cmd //c \"codegraph.cmd sync .\")",
            "  Kẹt khoá: codegraph unlock .    rồi sync lại.",
            "  → ROUTING §0.1",
        ],
        tool=tool,
        path="",
    )


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    try:
        root = c.project_root()
        ev = data.get("hook_event_name") or ("SessionStart" if not c.input_of(data) else "PostToolUse")
        if ev == "SessionStart":
            if not os.path.isdir(os.path.join(root, ".codegraph")):
                if tim_codegraph():
                    print("[ViGov] codegraph chưa init ở kho này — `codegraph init .` (ROUTING §0.1)")
                sys.exit(0)
            loi = dong_bo(root)
            if loi:
                print(f"[ViGov] codegraph sync thất bại ({loi}) — chỉ mục có thể CŨ. ROUTING §0.1")
                c.log_guard(HOOK, "SessionStart", "", f"sync hỏng — {loi}")
            sys.exit(0)

        tool = c.tool_of(data)
        if not c.la_vo_shell(tool) or not can_sync_sau(c.input_of(data).get("command", "")):
            sys.exit(0)
        loi = dong_bo(root)
        if loi:
            bao_loi(loi, tool)
    except SystemExit:
        raise
    except Exception:
        sys.exit(0)          # a side-effect hook must never cost the session anything
    sys.exit(0)


if __name__ == "__main__":
    main()
