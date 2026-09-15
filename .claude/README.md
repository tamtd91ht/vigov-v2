# The ViGov v2 brain — how it works and how to extend it

This directory is the **framework** for all code written in this project. The code is
written 100% by AI; humans supervise the business, the inputs and the outputs. So `.claude/`
is not documentation — **it is the only control mechanism**.

---

## Brain invariant: 9 rules ↔ 9 hooks, one to one

| Rule | Invariant | Hook | Level |
|---|---|---|---|
| 1 `tenant-isolation` | A record belongs to exactly one commune | `tenant_scope_guard` | BLOCK |
| 2 `service-boundary` | An entity has exactly one owning service | `service_boundary_guard` | BLOCK |
| 3 `personal-data` | Personal data never enters logs, never leaves unmasked | `pii_guard` | BLOCK |
| 4 `citizen-isolation` | A citizen sees only their own data | `citizen_scope_guard` | BLOCK |
| 5 `rbac` | Every endpoint declares permission explicitly | `rbac_guard` | BLOCK |
| 6 `audit-log` | Every write leaves a trail | `audit_guard` | advisory |
| 7 `data-preservation` | Administrative files are archival records | `data_safety_guard` | BLOCK |
| 8 `secrets-config` | Secrets never reach code, bundle, git or docs | `secret_scan` | BLOCK |
| 9 `knowledge-single-source` | One fact one place; never hand-write what can be generated | `doc_guard` | BLOCK |

**Adding a rule you cannot write a hook for** means the rule cannot be checked, which means
it will drift — **it belongs in `skills/`, not in `rules/critical/`.**

That test already removed two things from the rule tier:

| Demoted to a skill | Why |
|---|---|
| `administrative-language` | Important, but a machine cannot judge "feedback ≠ complaint ≠ denunciation" in context. Keeping it in the rule tier only spends always-loaded budget with no deterrent |
| `no-hardcoding` | Under multi-tenancy it is no longer an independent rule — it is a **consequence** of rule 1. Two copies of one rule are two copies that will drift |

## Four cross-cutting hooks

| Hook | Event | Job |
|---|---|---|
| `session_start` | SessionStart | Context · branch · is the knowledge layer ready · **dangerous flags currently on** |
| `bash_content_guard` | PreToolUse Bash | **Closes the shell-write detour** — see below |
| `stop_verify_guard` | **Stop** | Code changed but never verified → block the session from ending |
| `drift_guard` | SessionStart | Detect the code **silently deciding a customer's open question** |

### `bash_content_guard` — the hole v1 left open

A brain that only hooks `Edit|Write` is trivially routed around:

```
the same line of code,  written via Write  → BLOCKED
                        written via sed -i → LANDS
```

Having to say in prose what the tooling should have locked is the signature of a leaky
enforcement layer. v2 closes it twice: `settings.json` denies `sed`/`awk`/`perl`/`tee` at
the permission layer, and this hook scans **the content about to be written** inside any
command with a redirect or heredoc.

### `stop_verify_guard` — guarding the OUTPUT

Every brain carries "verify, do not just declare", and almost none enforces it. Eight hooks
guard the input; this is the only one guarding the output. For a 100%-AI project, "declared
done while tests are red" is a **more common** failure than "wrote a secret into the code".

---

## Agents — `agents/ROUTING.md` is the entry point

Nine agents: five build, four review. The main session reads `agents/ROUTING.md`, catches the
event, dispatches, and runs the mandatory follow-up.

| Agent | Mode | Owns |
|---|---|---|
| `contract-designer` | write | Contracts between services, events, entity ownership, transaction boundaries |
| `go-service-builder` | write | Everything inside one Go service |
| `data-migration-builder` | write | Schema changes, migrations, backfills, archival safety |
| `admin-web-builder` | write | Staff-facing Next.js |
| `citizen-app-builder` | write | Citizen-facing Mini App |
| `test-designer` | write | Test strategy, prioritised by cost of being wrong |
| `knowledge-keeper` | write | `kb/` curated tiers, ADRs, documentation discipline |
| `isolation-reviewer` | **read only** | Three isolation dimensions, including cross-file relations |
| `domain-expert` | **read only** | Vietnamese public administration business correctness |

Two design decisions worth stating, both taken from measured failures of v1:

**Routing belongs to the main session, not to an orchestrator agent.** In Claude Code a
subagent cannot spawn another subagent. v1 shipped a "chief architect" agent whose whole job
was delegating to other agents — it could never have worked. `ROUTING.md` is therefore a
decision table the main session reads, not an agent.

**Every path has exactly one owning agent.** A builder that needs a contract change stops and
hands back; it never edits `proto/**` itself. This is the same rule as service ownership
(rule 2), applied to the agents themselves — and it is the direct answer to the measured
finding that the declared type source of truth had three hand-maintained copies.

`ROUTING.md` carries no frontmatter on purpose: it is a decision table, not an agent
definition. `make brain` knows this and exempts it, while checking that ROUTING and the agent
files on disk still describe the same set.

---

## Checking the brain itself

| Command | What it checks |
|---|---|
| `make hooks` / `/check-hooks` | **47 cases**: every hook gets a must-block case and a must-pass case |
| `/check-brain` | 7 structural invariants (below) |
| `/knowledge-health` | Dead links · orphans · expired · budget |
| `/review-isolation` | All three isolation dimensions across the source |
| `/review-compliance` | What no hook can enforce: terminology, Vietnamese, accessibility, legal duties |

**A hook with no test may already be dead** — one syntax error, one broken pattern, and the
protection is off in silence: everything still looks normal, it just stops blocking.

### Seven structural invariants (`/check-brain`)

| # | Invariant | Threshold |
|---|---|---|
| 1 | Every rule has exactly one hook, and vice versa | 9 ↔ 9 |
| 2 | Every hook has at least one block case and one pass case | 0 missing |
| 3 | Every path and `/<command>` referenced under `.claude/**` exists | 0 dead |
| 4 | Every file in `skills/`, `commands/`, `agents/` has valid frontmatter | 0 missing |
| 5 | Always-loaded budget | ≤ 20,000 tokens |
| 6 | No `.md` outside `kb/`, `services/*/README.md`, `.claude/` | 0 |
| 7 | `ROUTING.md` and the agent files describe the same set | 0 drift |

Invariant 3 is the anti-drift mechanism. Brain v1 once had a dedicated commit cleaning up
traces of a dropped module across 31 files — and still missed 11. Keeping dozens of Markdown
files in sync by human discipline does not scale; it has to be machine-checked.

---

## Extending

### Adding a RULE

Only when a violation causes **unrecoverable harm**. And it is mandatory to:

1. Write `rules/critical/<n>-<name>.md` — state the **INVARIANT**, never the solution (below)
2. Add the `@import` line to the root `CLAUDE.md` — **without it the rule is never loaded**
3. Write the enforcing hook plus **at least 2 cases** in `tools/test_hooks.py`
4. Update the table in this file

**Rules state INVARIANTS; skills state SOLUTIONS.** The lesson from v1: the `no-hardcoding`
rule diagnosed correctly (*"one codebase serves many communes"*) but prescribed *"move it to
an environment variable"* — right for one-deployment-per-commune, **completely wrong** for
multi-tenant on shared infrastructure. Because it was an enforced rule, it pushed the agent
toward the wrong architecture **very effectively**, for over a hundred commits. **The better
the hook, the faster you go the wrong way.**

### Adding a HOOK

1. `hooks/<name>.py` — import `_common`, use `c.block()` / `c.warn()`
2. Messages **must state the correct approach**; escape hatches must be **explicit and
   loggable** (`// @cross-tenant: <reason>`) — no escape hatch and the hook gets disabled;
   an implicit one and the hook is meaningless
3. **Measure on real code before enabling.** A noisy hook is a disabled hook, and then the
   whole layer is gone — worse than having no hook
4. Add cases to `tools/test_hooks.py`, run `make hooks`
5. Register it in `settings.json`

### Adding a SKILL

`skills/<name>/SKILL.md` with `name` + `description` frontmatter. The *"Triggers on"* part of
the description decides whether the skill ever activates — list the words people **actually
use** when talking about that work.

---

## The guard log — `logs/guard.jsonl`

Every block appends one line: timestamp, hook, tool, file, violation **label** — and never
the violating content (the guard log must not become a store of exactly what it blocked).

Why it matters: rule 6 requires **application code** to audit every write. Without this log,
the brain's **own enforcement layer** would be the one thing leaving no trail — and there
would be no way to prove it ever worked.

---

## What this brain CANNOT catch

Stated plainly, because believing you are protected is more dangerous than knowing you are not:

| Not caught | Covered instead by |
|---|---|
| Wrong business logic (miscalculated deadline, wrong status) | Tests + domain review |
| Wrong Vietnamese, wrong administrative terminology | `/review-compliance` + a human reader |
| Isolation that is syntactically right but semantically wrong (right commune, wrong person) | `/review-isolation` + a two-commune isolation test |
| A design wrong from the start | `kb/10-decisions/` (ADR) + stop conditions |
| Hooks disabled via `settings.local.json` | Nothing — that file never enters git |

> This brain **does not make the code correct**. It makes **some classes of mistake blockable
> at the moment of typing, and the rest visible** — because what stays invisible never gets fixed.
