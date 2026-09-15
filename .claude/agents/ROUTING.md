# Agent routing map — ViGov v2

> **This file is read by the MAIN session, not by an agent.**
> In Claude Code a subagent cannot spawn another subagent. There is therefore no orchestrator
> agent: the main session reads this table, dispatches, receives the result, and dispatches
> again. Any design where one agent "coordinates the others" does not run — v1 had exactly
> that design and it could never have worked.

Nine agents. Five build, four review. Each has a **write boundary**; two agents never own the
same path.

---

## 0. How to use this file

```
user request
   │
   ├─ 1. Is this small and obvious?  ──────────────► do it directly, no agent (§6)
   │
   ├─ 2. Catch the EVENT (§1)  ─────────────────────► the trigger decides the entry agent
   │
   ├─ 3. Check the THREE QUESTIONS (§2)  ───────────► any unanswered = STOP, ask the user
   │
   ├─ 4. Dispatch to the entry agent (§3, §4)
   │
   └─ 5. Run the MANDATORY FOLLOW-UP (§5)  ─────────► never optional
```

Dispatching is sequential and explicit. The main session keeps the thread; agents return
findings and diffs, never further dispatches.

---

## 1. EVENT → ENTRY AGENT

The trigger is the *shape of the request or the repository state*, not the words used. Read
top to bottom; **the first match wins**.

| # | Event caught | Entry agent |
|---|---|---|
| 1 | A **STOP CONDITION** fires (§2) — commune, owner, permission, or consistency undecided | **none — ask the user first** |
| 2 | `drift_guard` warned that an open question is being silently decided | **none — raise it with the user** |
| 3 | Adding or changing a **contract between services**, an event, or entity ownership | `contract-designer` |
| 4 | A service needs data **it does not own** | `contract-designer` |
| 5 | Changing a table that **already holds data**, a backfill, an index, soft delete | `data-migration-builder` |
| 6 | Adding or changing a **backend use case, endpoint, query, repository** | `go-service-builder` |
| 7 | Adding or changing a **staff-facing screen, form, table, subsystem** | `admin-web-builder` |
| 8 | Adding or changing a **citizen-facing screen or submission flow** | `citizen-app-builder` |
| 9 | Business behaviour: **status, SLA, workflow, terminology, figures** | `domain-expert` (read-only) → then the relevant builder |
| 10 | Suspected **data leak**, wrong permission, wrong 401/403 | `isolation-reviewer` |
| 11 | Tests red, or "what should this be tested with" | `test-designer` |
| 12 | An **invariant, boundary, or decision changed** | `knowledge-keeper` |
| 13 | Preparing a **release / UAT / handover** | §5.3 release sequence |

### Events that route to `domain-expert` FIRST, always

These look like code work and are not. Getting the business wrong here is more expensive than
any implementation detail:

- a **new status** or a change to which transitions are legal
- anything computing a **deadline** or an SLA
- anything **numbering** documents or files
- naming a business concept that will become a field name
- a figure or statistic shown to leadership

Sequence: `domain-expert` (decide the business) → builder (implement) → `test-designer`.

---

## 2. THE THREE QUESTIONS — checked before any dispatch that writes

| Question | Answer lives in | Unanswered means |
|---|---|---|
| Which **commune** does this data belong to? | Rule 1 — always `tenant_id` | STOP |
| Which **service owns** this entity? | `kb/30-indexes/data-ownership.json` | STOP → `contract-designer` decides, or ask the user |
| Which **permission** guards this path? | Rule 5 — explicit, never implicit | STOP |

For a multi-step flow, a fourth question applies: **strong or eventual consistency, and what
is the compensation?** Undecided → `contract-designer`, and if it is a business call, the user.

> A STOP CONDITION is not a blocker to work around. It is the moment the work would otherwise
> silently decide something it has no authority to decide.

---

## 3. ROUTING BY LAYER

| Layer | Owner agent | Write boundary |
|---|---|---|
| Contracts between services | `contract-designer` | `proto/**`, `kb/30-indexes/transaction-boundaries.json` |
| Inside one Go service | `go-service-builder` | `services/<name>/**`, `pkg/**` |
| Schema and data | `data-migration-builder` | `services/*/migrations/**`, backfills |
| Staff web + platform console | `admin-web-builder` | `apps/commune-admin/**`, `apps/platform-admin/**` |
| Citizen app | `citizen-app-builder` | `apps/citizen-app/**` |
| Knowledge | `knowledge-keeper` | `kb/` curated tiers, `services/*/README.md` |
| Tests | `test-designer` | test files anywhere |

**Generated paths (`kb/20-contracts/**`, `kb/30-indexes/**`, `*.pb.go`) have no owner** — they
are produced by `make kb` and `buf generate`. Any agent editing them is a bug.

### Cross-layer changes

A change touching more than one layer is **decomposed by the main session**, in this order:

```
contract-designer  →  data-migration-builder  →  go-service-builder  →  client builders
      (shape)              (storage)                  (behaviour)           (surface)
```

Reason for the order: each step's output is the next step's input, and reversing it produces
work that has to be redone. Never run two builders on the same change in parallel.

---

## 4. ROUTING BY SYMPTOM — when something is wrong

| Symptom | Agent |
|---|---|
| Data from another commune is visible | `isolation-reviewer` |
| A citizen can see someone else's record | `isolation-reviewer` |
| 401/403 wrong, or a role can do too much | `isolation-reviewer` |
| Figures wrong, status wrong, deadline wrong | `domain-expert` |
| A field is missing on screen but present in the API | `contract-designer` (contract drift) |
| A list is short, or an old record is missing | `data-migration-builder` (soft-delete filter) |
| Tests red | `test-designer` |
| An event is not reaching a consumer | `contract-designer` |
| Empty environment variable, CORS, deploy failure | main session — this is configuration, not an agent's job |

**Never start from the symptom's location.** Start from `kb/30-indexes/code-map.json` and
`data-ownership.json`; they tell you where to look without scanning the repository.

---

## 5. MANDATORY FOLLOW-UP

### 5.1 After every change that writes code

| Order | Agent | Skippable when |
|---|---|---|
| 1 | `test-designer` | **Never** |
| 2 | `isolation-reviewer` | The change touches no data, permission, file, queue, or realtime path |
| 3 | `knowledge-keeper` | The change altered no invariant, boundary, or decision |

Then, in the main session: `make check`. It runs `check-brain`, `check-hooks`, lint and tests.
`stop_verify_guard` blocks the session from ending if this has not run.

### 5.2 After any change to a contract or an entity

`make kb` — regenerate the indexes. Without it, `data-ownership.json`, `code-map.json` and
`event-flows.json` describe a system that no longer exists, and the next session is routed
with a stale map.

### 5.3 Release sequence

```
domain-expert        → business behaviour is correct
isolation-reviewer   → all three dimensions, full scope
test-designer        → priority 1-3 coverage exists
knowledge-keeper     → /knowledge-health clean
main session         → /review-compliance · /review-isolation · make check
                     → two-commune isolation test (rule 1)
```

---

## 6. WHEN **NOT** TO USE AN AGENT

Dispatching costs a context switch and loses the thread. Do it directly when:

- the change is one or two files and the location is already known
- it is a typo, a label, a constant, a comment
- the user asked a question rather than for a change
- you are reading to understand — `kb/` is for that, not an agent

> Agents are for work that needs **a large read budget** or **a distinct expertise**.
> Using one for a small known change spends context and returns less than doing it directly.

---

## 7. CONFLICT RULES

| Situation | Rule |
|---|---|
| Two agents want the same path | The path's owner in §3 wins. The other **requests** the change |
| A builder needs a contract change | It stops and hands back — it never edits `proto/**` itself |
| A reviewer finds something fixable | It **reports**; it does not fix, unless the user asked for fixes |
| An agent hits a STOP CONDITION | It stops and returns the question. It never picks a default |
| Two agents disagree | The main session decides, or asks the user. Agents do not negotiate |

---

## 8. WHAT NO AGENT MAY DO

| Forbidden | Why |
|---|---|
| Decide a customer open question (`kb/00-foundation/open-questions.json`) | Not within its authority — this is the failure mode that cost the previous project 20–28 days |
| Edit generated files | Silently lost on the next generate |
| Disable or route around a hook | The block message always states the correct equivalent |
| Delete business data or documentation | Destructive actions are confirmed by the user |
| Report "done" without verification | `stop_verify_guard` blocks it, and rightly |

---

## 9. AGENT INDEX

| Agent | Mode | Owns |
|---|---|---|
| `contract-designer` | write | Contracts between services, event schemas, entity ownership, transaction boundaries |
| `go-service-builder` | write | Everything inside one Go service |
| `data-migration-builder` | write | Schema changes, migrations, backfills, archival safety |
| `admin-web-builder` | write | Staff-facing Next.js |
| `citizen-app-builder` | write | Citizen-facing Mini App |
| `test-designer` | write | Test strategy and tests, prioritised by cost of being wrong |
| `knowledge-keeper` | write | `kb/` curated tiers, ADRs, documentation discipline |
| `isolation-reviewer` | **read only** | Three isolation dimensions, including cross-file relations |
| `domain-expert` | **read only** | Vietnamese public administration business correctness |

---

## 10. WHY THIS SHAPE

Three findings from the review of the previous project determined this design:

| Finding | Consequence here |
|---|---|
| The brain enforced what is visible **in one place** and almost nothing requiring **the relation between two places** | `isolation-reviewer` and `contract-designer` exist specifically to own relations |
| The declared type source of truth lived in the frontend, with three hand-maintained copies | `contract-designer` owns the seams alone, and `proto/**` has exactly one owner |
| Tests concentrated where defects are **visible**, absent where they are **expensive** — `audit` and the citizen channel had none | `test-designer` prioritises by cost of being wrong, not by ease |

And one structural fact about Claude Code determined the rest: **a subagent cannot spawn a
subagent**, so routing belongs to the main session. v1 shipped an orchestrator agent that
could never have orchestrated anything.
