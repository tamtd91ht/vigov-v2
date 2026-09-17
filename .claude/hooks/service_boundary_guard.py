"""PreToolUse — BLOCK crossing a service boundary.  [RULE 2]

Scope: *.go under services/

WHY BLOCK: without boundaries the system becomes a DISTRIBUTED MONOLITH — full cost of
microservices, none of the benefit. This decay has NO SYMPTOMS: tests green, features work,
it is just that nothing can be changed any more. By the time anyone notices, dozens of
violations are already in.

Four things blocked:
  1. Importing another service's internal/   — boundary crossed at compile time
  2. Opening a DB this service does not own  — a read path that bypasses the contract
  3. Hand-editing .proto-generated files     — silently lost on the next generate
  4. (advisory) breaking a published event contract
"""

from __future__ import annotations

import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "service_boundary_guard"

IMPORT_INTERNAL = re.compile(
    r"[\"']([a-z0-9_.\-/]*?/services/([a-z0-9_\-]+)/internal/[a-z0-9_/\-]+)[\"']"
)

DB_CONNECT = re.compile(
    r"(sql\.Open|pgxpool\.New|pgx\.Connect|mongo\.Connect|redis\.NewClient|"
    r"gorm\.Open|sqlx\.(?:Open|Connect))\s*\(",
)

# Database name inside a DSN — compared against the service being edited
DSN_DBNAME = re.compile(r"(?:dbname=|/)([a-z0-9_]{3,})(?:[\?\"'`\s]|$)")

PROTO_GEN = (".pb.go", "_grpc.pb.go", ".pb.gw.go", "_pb2.py", ".connect.go")


def service_of(path: str) -> str | None:
    # Normalise: a relative path ("services/x/...") has no leading slash
    m = re.search(r"/services/([a-z0-9_\-]+)/", "/" + path.lstrip("/"))
    return m.group(1) if m else None


def owner_map(root: str) -> dict:
    """Data-ownership map — GENERATED tier, source of truth for rule 2 invariant 1."""
    p = os.path.join(root, "kb", "30-indexes", "data-ownership.json")
    try:
        with open(p, encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return {}


def scan(content: str, path: str) -> list[str]:
    hits: list[str] = []
    me = service_of(path)
    lines = content.splitlines()

    for i, line in enumerate(lines):
        m = IMPORT_INTERNAL.search(line)
        if m and me and m.group(2) != me:
            hits.append(f"line {i+1}: imports internal/ of service '{m.group(2)}' (we are '{me}')")

        if DB_CONNECT.search(line):
            ctx = "\n".join(lines[max(0, i - 3): i + 4])
            for db in DSN_DBNAME.findall(ctx):
                if me and db != me and db not in ("postgres", "localhost", "127", "0"):
                    hits.append(f"line {i+1}: connects to database '{db}' — service '{me}' does not own it")
                    break

    return hits


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path:
        sys.exit(0)

    # 3. Hand-editing a .proto-generated file
    if path.endswith(PROTO_GEN):
        c.block(
            HOOK,
            f"hand-editing a GENERATED file — {os.path.basename(path)}",
            ["this file is produced by `buf generate`"],
            [
                "  Edits here are SILENTLY lost on the next generate, and the contract between",
                "  services drifts from what is actually running.",
                "",
                "  Correct approach: edit the .proto, then run `buf generate`.",
                "  Need custom logic? Put it in a normal file alongside, never in generated code.",
                "",
                "  → Rule 2: .claude/rules/critical/2-service-boundary.md",
                "  → Skill: .claude/skills/proto-contract/SKILL.md",
            ],
            tool=tool,
            path=path,
        )

    if not path.endswith(".go") or c.should_skip(path):
        sys.exit(0)
    if "/services/" not in ("/" + path.lstrip("/")):
        sys.exit(0)

    content = c.new_content(ti)
    if not content or c.is_generated(content, path):
        sys.exit(0)

    hits = scan(content, path)
    if not hits:
        sys.exit(0)

    c.block(
        HOOK,
        f"service boundary crossed — {os.path.basename(path)}",
        hits,
        [
            "  Every entity has EXACTLY ONE owning service. Others read it only through a",
            "  CONTRACT — gRPC (sync) or events (async). There is no third path.",
            "",
            "  Reading another service's database directly, ten times over, is a distributed",
            "  monolith: full microservice cost, and nothing can be changed any more.",
            "",
            "  Correct approach:",
            "    - need it now        → call gRPC via the contract in proto/",
            "    - can tolerate lag   → consume events, keep your own read model",
            "    - contract missing   → STOP CONDITION, ask the user",
            "",
            "  Who owns what: kb/30-indexes/data-ownership.json",
            "  → Rule 2: .claude/rules/critical/2-service-boundary.md",
        ],
        tool=tool,
        path=path,
    )


if __name__ == "__main__":
    main()
