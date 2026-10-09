"""Stop — BLOCK ending the session while this session's work is uncommitted or unpushed.

WHY THIS HOOK EXISTS: the work runs on several machines (CLAUDE.md, the sibling-repo section),
and the owner builds and checks images from `origin/main`, not from this disk. Work that is
finished but sits uncommitted — or committed but unpushed — does not exist for anybody else:
the build machine cannot see it, the next session on another machine starts without it, and a
parallel session on this disk can overwrite it. On 09/10/2026 a finished screen sat uncommitted
for hours waiting for a screenshot that never came, until the owner asked; the owner's standing
order since: "bất cứ lúc nào code xong đều phải commit và push".

What it checks, at Stop:
  A. files THIS session (main transcript + its subagents' transcripts) wrote with Edit/Write
     that are still dirty in git                                  → "commit them"
  B. the current branch is ahead of its upstream                  → "push"

Why only THIS session's files: parallel sessions share the working tree, and their half-done
files must never be swept into this session's commit (ROUTING §0.7, never `git add -A`). So A
is limited to paths the transcripts prove this session wrote. B is branch-wide on purpose: an
unpushed commit is unpushed whoever made it, and pushing it is never harmful on `main`.

Not a verification gate: `stop_verify_guard` decides whether the work was verified. This hook
only says verified work must not stay local. A red check is reported and committed with the
failure stated in the message — never parked on disk.

Safety: `stop_hook_active` → let through once (no loop). Any git failure → silent (a broken git
call must not trap the session).
"""

from __future__ import annotations

import glob
import json
import os
import re
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "commit_push_guard"

_EDIT_TOOLS = ('"Edit"', '"Write"', '"MultiEdit"', '"NotebookEdit"')
_FILE_PATH = re.compile(r'"(?:file_path|notebook_path)"\s*:\s*"([^"]+)"')


def repo_root() -> str:
    return os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd()


def to_repo_path(path: str, root: str) -> str | None:
    """Absolute or relative path → repo-relative with forward slashes; None when outside."""
    p = path.replace("\\\\", "\\").replace("\\", "/")
    r = root.replace("\\", "/").rstrip("/")
    if re.match(r"^[A-Za-z]:/", p) or p.startswith("/"):
        if p.lower().startswith(r.lower() + "/"):
            p = p[len(r) + 1:]
        else:
            return None
    return p[2:] if p.startswith("./") else p


def transcripts(main_path: str) -> list[str]:
    """The main transcript plus every subagent transcript of the same session."""
    out = [main_path] if main_path and os.path.exists(main_path) else []
    base = main_path[:-6] if main_path.endswith(".jsonl") else main_path
    out += sorted(glob.glob(os.path.join(base, "subagents", "*.jsonl")))
    return out


def written_paths(paths: list[str], root: str) -> list[str]:
    seen: list[str] = []
    for tp in paths:
        try:
            with open(tp, encoding="utf-8", errors="ignore") as f:
                for line in f:
                    if not any(t in line for t in _EDIT_TOOLS):
                        continue
                    for m in _FILE_PATH.finditer(line):
                        rel = to_repo_path(m.group(1), root)
                        if rel and rel not in seen:
                            seen.append(rel)
        except Exception:
            continue
    return seen


def git(args: list[str], root: str) -> str | None:
    try:
        r = subprocess.run(["git", "-C", root, *args], capture_output=True, text=True,
                           encoding="utf-8", errors="ignore", timeout=20)
    except Exception:
        return None
    return r.stdout if r.returncode == 0 else None


def dirty_set(root: str) -> set[str] | None:
    out = git(["status", "--porcelain", "-uall"], root)
    if out is None:
        return None
    paths = set()
    for line in out.splitlines():
        if len(line) < 4:
            continue
        p = line[3:]
        if " -> " in p:
            p = p.split(" -> ", 1)[1]
        paths.add(p.strip().strip('"'))
    return paths


def ahead_count(root: str) -> int | None:
    out = git(["rev-list", "--count", "@{u}..HEAD"], root)
    try:
        return int(out.strip()) if out is not None else None
    except ValueError:
        return None


def problems(written: list[str], dirty: set[str], ahead: int) -> list[str]:
    """Pure decision — tested in tools/test_hooks.py. Empty list = let the session end."""
    out = [f"chưa commit: {p}" for p in written if p in dirty]
    if ahead > 0:
        out.append(f"{ahead} commit chưa push lên upstream")
    return out


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    if data.get("stop_hook_active"):
        sys.exit(0)

    root = repo_root()
    written = written_paths(transcripts(data.get("transcript_path") or ""), root)
    dirty = dirty_set(root)
    ahead = ahead_count(root)
    if dirty is None:
        dirty = set()
    found = problems(written, dirty, ahead or 0)
    if not found:
        sys.exit(0)

    c.block(
        HOOK,
        "this session's work is not on origin yet",
        found,
        [
            "  Owner's standing order (09/10/2026): code xong là commit và push — mọi lúc.",
            "  The build machine and every other session only see origin/main.",
            "",
            "  Do, in this order:",
            "    git add <each path above, explicitly>     # never `git add -A`: parallel sessions",
            "    git commit -m \"type(scope): …\"            # state any red check in the message",
            "    git push origin main",
            "",
            "  A file above belongs to another session? Leave it, and say so in your reply.",
            "  Not verified with a screenshot/PostgreSQL? Commit + push anyway and SAY what is",
            "  unverified — never park finished code on disk.",
            "",
            "  → CLAUDE.md, section 'GIT — main ONLY'",
        ],
        tool="Stop",
        path="",
    )


if __name__ == "__main__":
    main()
