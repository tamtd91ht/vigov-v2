---
description: Check the 7 structural invariants of the .claude brain — anti-drift
group: Bộ não
allowed-tools: Read, Bash, Glob, Grep
---

# /check-brain

A brain drifting away from reality is something that **will** happen, not something that
might. Keeping dozens of Markdown files in sync by human discipline does not scale — it has
to be machine-checked.

## Seven invariants

| # | Invariant | How | Threshold |
|---|---|---|---|
| 1 | Every rule has exactly one hook and vice versa | compare `rules/critical/*.md`, the table in `README.md`, and `settings.json` | 9 ↔ 9 |
| 2 | Every hook has ≥1 block case and ≥1 pass case | count in `tools/test_hooks.py` | 0 missing |
| 3 | Every path and `/<command>` referenced under `.claude/**` exists | extract and resolve | 0 dead |
| 4 | `skills/`, `commands/`, `agents/` have valid frontmatter | parse YAML | 0 missing |
| 5 | Always-loaded budget | count `CLAUDE.md` + the rules + `kb` `always_load` | ≤ 25,000 tokens |
| 6 | No `.md` outside `kb/`, `*/README.md`, `.claude/` | list `**/*.md` | 0 |
| 7 | `agents/ROUTING.md` and the agent files describe the same set | compare both lists | 0 drift |

Run: `python tools/check_brain.py`

## Report

One line per invariant: `[PASS] / [FAIL] <invariant> — <numbers>`.
A failing invariant lists **each specific location** and how to fix it — never a vague summary.

Invariant 3 catches drift earliest: a dead path means the brain is pointing the agent at
something that no longer exists.

Invariant 7 guards the routing map specifically. An agent named in ROUTING but absent from
disk sends the session nowhere; an agent on disk that ROUTING never routes to is one nobody
will ever reach. v1 lost a module and still carried 11 stale references after a dedicated
cleanup commit — this is that failure, made impossible to miss.
