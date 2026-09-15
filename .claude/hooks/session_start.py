"""SessionStart — ViGov context reminder at the top of every session.

Goal: <= 10 lines. Say only what the person writing code needs RIGHT NOW; no lecturing.
Constraints: standard library only, fast (<200ms), fail silently.
"""

from __future__ import annotations

import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

MAIN_BRANCHES = ("main", "master", "prod")

# Flags that open a shortcut past real checks — must be off before production.
# Only presence is tested; the value is NEVER read or printed.
DANGEROUS_FLAGS = (
    "CITIZEN_OTP_BYPASS_CODE", "DISABLE_AUTH", "DEMO_MODE",
    "ALLOW_CROSS_TENANT", "SEED_ON_BOOT", "SKIP_TENANT_CHECK",
)


def git_branch(cwd: str) -> str | None:
    try:
        r = subprocess.run(["git", "symbolic-ref", "--short", "HEAD"],
                           capture_output=True, text=True, timeout=3, cwd=cwd)
        return r.stdout.strip() if r.returncode == 0 else None
    except Exception:
        return None


def flags_on(root: str) -> list[str]:
    on = []
    for rel in (".env", ".env.local", "deploy/.env"):
        p = os.path.join(root, rel)
        if not os.path.exists(p):
            continue
        try:
            with open(p, encoding="utf-8", errors="ignore") as f:
                for line in f:
                    line = line.strip()
                    for flag in DANGEROUS_FLAGS:
                        if line.startswith(flag + "="):
                            val = line.split("=", 1)[1].strip().strip('"').strip("'")
                            if val and val.lower() not in ("false", "0", "off", "no") \
                                    and flag not in on:
                                on.append(flag)
        except Exception:
            continue
    return on


def kb_ready(root: str) -> tuple[bool, str]:
    if not os.path.exists(os.path.join(root, "kb", "INDEX.yaml")):
        return False, "kb/INDEX.yaml missing — run `make kb`"
    if not os.path.exists(os.path.join(root, "kb", "30-indexes", "code-map.json")):
        return False, "generated indexes missing — run `make kb`"
    return True, ""


def main() -> None:
    c.utf8_streams()
    c.read_input()
    root = c.project_root()

    lines = [
        "[ViGov] GOVERNMENT system · MULTI-COMMUNE on shared infrastructure · citizen personal "
        "data + legally binding records. Careful first, fast second.",
        "[ViGov] Before reading code: kb/INDEX.yaml -> always_load -> indexes. Do NOT grep the repo.",
    ]

    branch = git_branch(root)
    if branch and branch not in MAIN_BRANCHES:
        lines.append(f"[ViGov] WARNING: on branch '{branch}', not main. This project uses main only.")

    ok, why = kb_ready(root)
    if not ok:
        lines.append(f"[ViGov] WARNING: knowledge layer not ready: {why}")

    on = flags_on(root)
    if on:
        lines.append(
            "[ViGov] WARNING: dangerous flags ON: " + ", ".join(on) +
            " — a shortcut past real checks. Must be off before production."
        )

    print("\n" + "\n".join(lines) + "\n")
    sys.exit(0)


if __name__ == "__main__":
    main()
