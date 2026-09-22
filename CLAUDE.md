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

## THE MINI APP SPANS TWO REPOSITORIES

| Repository | Holds |
|---|---|
| `vihat-miniapp` — **sibling directory**, `github.com/tamtd91ht/vihat-miniapp` | The **app itself**: Zalo registration, App ID + app secret, login backend (`POST /api/v1/sessions`), Zalo webhook, the QR codes it issues. Separate Go repo, commercial, **not ViGov** |
| `vigov-v2/citizen-app` (here) | The **front-end running inside** that app. Ships into the App ID the other repo owns |

**Which repo a Zalo surface belongs to is decided by WHICH SECRET SIGNS IT** — not by "who
the vendor is". Answered wrongly once, and the wrong answer shipped (ADR 0032):

| Surface | Signed with | Repo |
|---|---|---|
| Mini App webhook · `accessToken`/`phoneToken` exchange | the app owner's app secret | `vihat-miniapp` |
| **ZNS from EACH COMMUNE's OA** | that commune's key | **here**, `service-comms` (ADR 0018) |

Do **not** generalise the second row away into "anything Zalo leaves ViGov".

A QR from `vihat-miniapp` carries parameters that load into `citizen-app`. They **steer the
interface and grant nothing**: client-supplied data (rule 1, forbidden #2), commune enters a
session only by the citizen's explicit act, matching check is **in the flow** → ADR 0005 ·
0019 · 0022. Who owns the app and which OA authenticates it: ADR 0031 · 0018.

### `vihat-miniapp` MUST sit beside `vigov-v2` — and must be there before Mini App work

Not a tidiness convention. The work runs on **several machines**, and "beside `vigov-v2`" is
the only location that is true on all of them; an absolute path is true on exactly one, and
the day it stops being true nobody notices.

**Missing it, while doing Mini App work, is an ERROR — stop and say so.** Do not reason about
the Mini App from the half that lives here: that conclusion compiles, passes its tests, and is
written into the wrong repository. It has already happened — the Zalo webhook lived a day
inside `service-platform` before it was removed (ADR 0032).

```sh
cd <parent of vigov-v2> && git clone https://github.com/tamtd91ht/vihat-miniapp
```

`hooks/miniapp_sibling_guard.py` (BLOCK) enforces both halves: **absent** while a
`citizen-app/**` or `vihat-miniapp/**` path — or a shell command naming the repo — is touched,
and **present but somewhere other than beside this repo**. Two copies in two places are two
copies that drift, and the forgotten one is the one somebody reads.

⚠ The other hooks fire on edits to `vihat-miniapp` too — per session, not per repo — and that
repo has no `core/authz` and no `kb/`. Read such a report, do **not** route around it, and do
**not** import ViGov conventions there to silence it.

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

### Before touching a module — read where it stands

→ `kb/90-ephemeral/tien-do.md` — one generated file, one section per module: done · not done ·
still owed to the customer. Read it instead of rediscovering the state from the source.

### Before saying it is done

→ run real verification (`make check`). `stop_verify_guard` blocks if it has not run.
→ record what moved in `kb/90-ephemeral/tien-do/<module>.json`, then `make kb` (`/progress`).
`progress_guard` blocks the session from ending if a module's code changed and its ledger
did not. Nothing worth recording? Say that sentence out loud — never write an empty item.

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
| `rules/critical/` | **11 rules**, always loaded (below). Each names an enforcing hook |
| `hooks/` | **20 hooks**: 13 rule hooks + 7 cross-cutting |
| `skills/` | Skills, lazily loaded by keyword |
| `commands/` | Procedures invoked as `/command-name` |
| `agents/` | **10 agents** — 5 build, 5 review. Entry point: `agents/ROUTING.md` |
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
@.claude/rules/critical/11-infra-config-contract.md
