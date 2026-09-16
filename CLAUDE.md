# ViGov v2 — conventions for the AI agent

---

# THIS IS A GOVERNMENT SYSTEM

ViGov is a **commune/ward-level digital administration platform**, running **one system for
many communes** on shared cloud infrastructure, told apart by **domain**. Every line of code
touches one of four things:

1. **Citizen personal data** — phone numbers, names, addresses, scene photographs, national
   ID data, petition contents. Governed by **Decree 13/2023/ND-CP**.
2. **Legally binding administrative records** — incoming/outgoing documents, petitions,
   feedback tickets, disbursement decisions. **Archival records**: no hard delete, no silent edits.
3. **The boundary between government bodies** — data leaking from one commune to another is
   not a software bug, it is a **breach between two public authorities**.
4. **The standing of a public authority** — a wrong commune name, a skewed figure, a
   misspelled Vietnamese sentence are all incidents somebody has to answer for.

| Principle | What it means in code |
|---|---|
| **Careful first, fast second** | Never guess the business rule. Never "temporary, fix later" |
| **Fail closed** | Cannot resolve commune / citizen / data owner → **refuse**. Never a default on the isolation path |
| **State assumptions** | Administrative practice varies by locality — choosing silently is choosing wrong |
| **Never destroy data** | Soft delete + audit trail. No lossy migrations |
| **Every write leaves a trail** | Who, what, when, from which IP, **in which commune** |
| **Closed by default** | New endpoints declare permission explicitly. New business files are private |
| **Verify, do not declare** | Report done once it has been run and is green. `stop_verify_guard` blocks otherwise |
| **Never decide the customer's open questions** | See `kb/00-foundation/open-questions.json`. `drift_guard` warns when the code is deciding silently |

**When forced to choose between fast and correct — choose correct, then tell the user what
it costs.**

---

## STACK

| Layer | Technology | Note |
|---|---|---|
| Backend | **Go microservices** | `.proto` is the source of truth for contracts |
| Admin web (staff) | **Next.js** | Configuration read **at runtime** by domain — never baked into the bundle |
| Citizen channel | **Zalo Mini App** | **No domain** — resolves commune differently, see `skills/zalo-miniapp-multi-tenant` |
| Deployment | Cloud, **multi-commune on shared infrastructure**, told apart by domain | |

---

## SESSION PROTOCOL

### Before reading any source code — in this order

1. `kb/INDEX.yaml` (~3 KB) — what exists · where · **what is deliberately not here**
2. The `always_load` entries (~14 KB) — what this system is, which invariants hold
3. "Where is X" → `kb/30-indexes/code-map.json` — **do NOT grep the repo**
4. "Who owns X" → `kb/30-indexes/data-ownership.json` — **do NOT read every schema**
5. "Why is it like this" → `kb/10-decisions/` — **do NOT infer it from the code**
6. **Only when you need to know HOW** → read the exact file step 3 pointed to

Not found in `kb/`: check the `not_here` section first.

- Listed there → read the code, **do not write documentation to compensate**
- Nowhere at all → this is **missing knowledge**: tell the user and propose the right tier.
  **Never create an `.md` wherever is convenient** (rule 9)

### Before adding any entity / query / endpoint

→ rule 1 (**which commune**) · rule 2 (**which service owns it**) · rule 5 (**what permission**)
→ any of the three unclear: **STOP CONDITION**, ask the user

### Before doing the work — route it

Read **`.claude/agents/ROUTING.md`** and dispatch. That file is the entry point to every
agent: it catches the event, names the entry agent, and lists the mandatory follow-up.

- Small, obvious, location already known → **do it directly**, no agent (ROUTING §6)
- Anything else → ROUTING §1 catches the event and names the agent
- A STOP CONDITION fires → **ask the user first**, dispatch nothing

**Routing is the main session's job.** A subagent cannot dispatch another subagent, so you
keep the thread: dispatch, take the result, dispatch the next. Never ask an agent to
coordinate other agents.

### Before saying it is done

→ run real verification (`make check`). `stop_verify_guard` blocks if it has not run.

---

## WORKING BEHAVIOUR

| Rule | Meaning |
|---|---|
| **Read before editing** | Read the real file and the neighbouring service. Comments explain *why* — read them first |
| **Stay in scope** | Touch only what the request needs. No drive-by refactors, no reformatting |
| **Simplest thing first** | The least code that solves the actual problem. No abstraction for a single call site |
| **Reuse before writing** | Check `kb/30-indexes/code-map.json` before writing a helper |
| **Comments explain WHY** | Give the reason and the consequence of getting it wrong; never restate the code |
| **Write short, technical, complete** | One idea per sentence, prefer tables, cite `file:line`. No preamble, no metaphor |
| **Plan multi-step work** | State `1. [step] → verified by: [how]` before starting |

Prose in documentation (`kb/`) is written in **Vietnamese** — the people supervising this
project read Vietnamese. Everything else — identifiers, file names, code comments, this
brain — is in **English**.

---

## GIT — `main` ONLY

Commit and push straight to `main`. Branch **only** when the user decides to, or when the
agent proposes it and the user **agrees**. "To be safe" is not a reason to branch on your
own: if you see a risk, **state the risk**, then do what was asked.

---

## THE BRAIN — `.claude/`

| Location | Contents |
|---|---|
| `rules/critical/` | **10 rules**, always loaded (below). Each names an enforcing hook |
| `hooks/` | **14 hooks**: 10 rule hooks + 4 cross-cutting |
| `skills/` | Skills, lazily loaded by keyword |
| `commands/` | Procedures invoked as `/command-name` |
| `agents/` | **9 agents** — 5 build, 4 review. Entry point: `agents/ROUTING.md` |
| `logs/guard.jsonl` | Guard log — evidence the enforcement layer actually ran |

**Brain invariants.** Every rule names at least one enforcing hook: a rule you cannot write a
hook for is a rule nothing checks, which means it will drift — it belongs in `skills/`.
(Rule 8 names two: `secret_scan` blocks, `session_start` warns on dangerous flags.) And every agent
listed in `agents/ROUTING.md` exists on disk, and every agent on disk is listed there.
`make brain` checks both.

When a hook blocks something, **do not look for another route** — every block message states
the correct equivalent. If the blocked action is genuinely required, state the consequences
to the user and wait for explicit confirmation.

Extending the brain: `.claude/README.md`.

---

## RULES — always loaded

@.claude/rules/critical/1-tenant-isolation.md
@.claude/rules/critical/2-service-boundary.md
@.claude/rules/critical/3-personal-data.md
@.claude/rules/critical/4-citizen-isolation.md
@.claude/rules/critical/5-rbac.md
@.claude/rules/critical/6-audit-log.md
@.claude/rules/critical/7-data-preservation.md
@.claude/rules/critical/8-secrets-config.md
@.claude/rules/critical/9-knowledge-single-source.md
@.claude/rules/critical/10-citizen-commitment.md
