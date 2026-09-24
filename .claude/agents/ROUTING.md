# Agent routing map — ViGov v2

> **This file is read by the MAIN session, not by an agent.**
> In Claude Code a subagent cannot spawn another subagent. There is therefore no orchestrator
> agent: the main session reads this table, dispatches, receives the result, and dispatches
> again. Any design where one agent "coordinates the others" does not run — v1 had exactly
> that design and it could never have worked.

Thirteen agents: **eight that write, five read-only** (`context-scout`, `cross-context-scout`,
`isolation-reviewer`, `domain-expert`, `progress-reviewer`). Each has a **write boundary**; two
agents never own the same path.

---

## 0. THE DEVELOPMENT WORKFLOW — every request that writes code

**Analyze → Discover (parallel) → Synthesize → Gate → Decompose → Implement (parallel) →
Validate → Commit → Document.** No code is edited before §0.3 has passed.

Continuing one existing business menu on one platform has its own entry points —
`/develop-web-admin` · `/develop-backend-api` · `/develop-miniapp` `<menu>` — which run this
workflow with menu resolution, per-menu knowledge and platform scope added
(`.claude/skills/develop-menu/SKILL.md`). `/develop-feature <menu>[: <chức năng>]` runs all
three for one feature, backend first, then both clients in parallel after a contract handoff
gate.

```
user request
   │
   ├─ 0.0 Exempt by the TEST below? ─── yes ──► do it directly; discovery = NOT APPLICABLE (say why)
   │
   ├─ 0.1 codegraph ready?  ── no ──► codegraph init (below), then continue
   │
   ├─ 0.2 DISCOVER — ONE message, two read-only agents in parallel
   │        context-scout        (straight down: files, symbols, recent commits, behaviour)
   │        cross-context-scout  (sideways: other modules, ../vigov-require, earlier sessions)
   │
   ├─ 0.3 SYNTHESIZE (Plan agent when large) + GATE (main session, always)
   │        any gate item below  ──► STOP · explain · concrete options · ask · WAIT
   │
   ├─ 0.4 DECOMPOSE into task cards · catch the EVENT (§1) per card · owner per §3
   │
   ├─ 0.5 IMPLEMENT — 2–3 builders in parallel where §3 + skills/parallel-agents allow
   │
   ├─ 0.6 VALIDATE — main session alone, after every builder has returned (§5.1)
   │
   ├─ 0.7 COMMIT — one commit per validated task
   │
   └─ 0.8 DOCUMENT — ledger · kb tier · handover (§0.8)
```

Dispatching is explicit and the main session keeps the thread; agents return findings and
diffs, never further dispatches. Several agents go out in **one message** only when §3 and
`.claude/skills/parallel-agents/SKILL.md` say their boundaries and shared state are disjoint.

### 0.0 The exemption is a TEST, not a judgement

"Small and obvious" judged by the agent making the change is the one judgement this workflow
exists not to trust: a two-line change that breaks a contract looks small to the agent that
wrote it. So the exemption is decided by what the change touches, and **both** must hold:

| # | Condition |
|---|---|
| 1 | Touches none of: `proto/**` · `*/migrations/**` · `core/**` · `*/internal/http/**` · `*/internal/grpc/**` · `*/internal/event/**` · anything under `authz/` · a type or field another service or client reads |
| 2 | Changes only comments, labels, user-visible text, or documentation — no control flow, no data shape |

Either fails → full workflow. Exempt → say so in one line, naming which of the two held.
`hooks/workflow_guard.py` checks condition 1 (and a ≥3-code-file edit) at Stop and speaks
once if neither scout ran; condition 2 is not machine-decidable and stays the agent's stated
reason.

### 0.1 codegraph first

**Every codegraph call passes `projectPath` = the repo root** (`git rev-parse --show-toplevel`).
Without it the MCP server answers from **whatever project it was started for** — measured on
2026-09-24: `codegraph_status` with no path returned 5 476 Java files of another company
project, with `projectPath` it returned this repo's 535 Go / 164 TS. It fails SILENTLY: the
queries succeed, the symbols are real, they are just not ours. The dry run of `context-scout`
is what caught it.

Sanity check before trusting any answer: `codegraph_status` must list **go and typescript**
and **no java**. Anything else → wrong index; say so and fall back.

**Freshness: never trust `codegraph status`** — measured 2026-09-24, it printed "Index is up
to date" while two functions added after init were missing. `hooks/codegraph_sync.py` syncs
at SessionStart and after every git command that moves the tree (commit, pull, merge,
checkout, rebase…), ~1 s, never blocking. What it cannot see is **uncommitted** edits: if this
session has changed code since its last git command, run `codegraph sync .` (Windows:
`cmd //c "codegraph.cmd sync ."`) in the main session before dispatching the scouts. Never in
a builder — the index is shared state, like `make kb`.

"Not initialized" → run `codegraph init .` in the repo root (Windows:
`cmd //c "codegraph.cmd init ."`), then `codegraph_status` again. `.codegraph/` is per machine
and gitignored. "Could grep instead" is not a reason to skip it; grep is the fallback for what
the graph does not index (SQL, YAML, string literals).

codegraph answers **what calls what** and **what breaks**. It does not answer **who owns** an
entity or **why** — those stay `kb/30-indexes/data-ownership.json` and `kb/10-decisions/`.

### 0.2 "Agent 3" is split in two: drafting, and deciding

The proposal's agent 3 does two different jobs, and only one of them needs the main session:

| Job | Who | Why |
|---|---|---|
| **Draft** the synthesis and the task cards from the two reports | built-in `Plan` agent, **when large** | A fresh context reads the reports without the main session's first reading of the request — the anchoring it has to catch. And the two long reports land in its context, not the main one |
| **Decide**: run the gate, ask the user, dispatch builders | **main session, always** | A subagent cannot dispatch and cannot ask the user mid-flight |

A drafting subagent is **not** the v1 orchestrator (§10): that one's job was to dispatch, which
cannot work. This one dispatches nothing and returns a document.

**Large** = the scouts report more than one module affected, or the draft would hold three or
more task cards. Otherwise the main session drafts it itself — a `Plan` round trip for one card
costs more than it saves. Brief the `Plan` agent with both reports **verbatim** and the output
headings below; it must list every `CONFLICT` and `UNKNOWN` unresolved, never pick a side.

### 0.3 Synthesis, and the gate

The synthesis (drafted per §0.2, always shown to the user, never written to a file):

```
Requirement Summary · Current System Behavior · Expected Behavior · Affected Components
Risks · Unknowns · Requirement Conflicts · Implementation Tasks · Dependencies
```

**STOP and ask before any code** when any of these holds — this list extends the STOP
CONDITIONS of rules 1–11 and §2; it never relaxes them:

| Gate | |
|---|---|
| Requirement unclear, or more than one reasonable implementation | |
| Conflict: requirement ↔ code · old requirement ↔ new · request ↔ `../vigov-require` | Never pick a side |
| Existing behaviour changes | |
| API contract · event/message contract · DB schema · data migration · backward compatibility | |
| Authentication / authorisation · money / disbursement logic | |
| Impact on another module · high production risk · a destructive operation | |
| A business assumption nobody has confirmed | |
| An open question in `kb/00-foundation/open-questions.json` is touched | §1 row 2 |

Ask with **concrete options**, recommended one first (AskUserQuestion). Then **wait** — a
gated request is not started "while waiting".

### 0.4 Task cards

Each independent task is one card, stated to the user before dispatch and passed verbatim as
the builder's brief:

```
TASK-NN · Title
Goal · Scope (in / out) · Files/Modules · Owner agent (§3)
Dependencies (TASK-xx) · Input · Expected Output · Validation (the exact command) · Risk
```

A card whose owner is unclear, or that needs another card's output, is not parallel.

### 0.5 Implementation agents stay in their card

A builder implements its card and nothing else: no scope growth, no unrelated fix, no large
refactor, no architecture or public-API change outside the card. Anything found outside it is
returned as `OUT-OF-SCOPE: <what> · <file:line>` for the main session to route — never fixed
in passing.

**Parallel cap:** 2–3 builders, but **at most 2 that compile Go, and 1 is the safe number**
(measured, `skills/parallel-agents`). Sequential whenever a card depends on another, two
cards touch the same critical file, or merge risk exists.

**What that leaves in practice — stated so nobody expects the proposal's picture.** Its
diagram fans one feature out as TASK-01 domain ‖ TASK-02 repository ‖ TASK-03 API. Here that
is forbidden (§3, cross-layer changes): each layer's output is the next one's input, so one
feature's backend cards are **sequential**. Add `go-service ∩ migration` and `test-designer`
being sequential with every writer, and real implementation parallelism is mostly:

| Runs in parallel | Why it is safe |
|---|---|
| `admin-web-builder` / `citizen-app-builder` ‖ one Go builder | Disjoint trees, and the Node toolchain is cheap — after the contract card has landed |
| Any builder ‖ read-only agents (scouts, reviewers, `domain-expert`) | They write nothing |
| Two Go builders on **unrelated** requests, different services, no `core/**`, no new dependency | The rare case; still ≤2 |

The Go cap was measured on **one** machine on 22/09/2026. It is a floor for that machine, not
a constant: on a machine with more memory it may be raised, but only by measuring there —
never by assuming.

### 0.6 Validation

`compile → unit tests → relevant integration tests → static/architecture checks (make check)
→ review the diff`. A failure goes back to the card's builder, then the gate re-runs. Never
commit what could have been validated and was not; if a step cannot run on this machine, say
so and name it.

### 0.7 Commit — one per validated task

Straight to `main` (GIT section of CLAUDE.md). Small, one scope, no unrelated hunks, staged
**by explicit path** — never `git add -A`, which sweeps up a parallel session's work. Message
`type(scope): …` (`feat` · `fix` · `refactor` · `test` · `docs` · `chore`). Independent tasks
are not folded into one commit.

### 0.8 Documentation — into the tiers that already exist

The proposal behind this section drew a `docs/` tree and a `sessions/YYYY-MM-DD/` folder. Both
already exist here under other names, and a second system beside them is forbidden by rule 9 —
so each question goes to its owning place:

| Question the session must leave answered | Owning place |
|---|---|
| What moved in this module · what remains · what is owed to the customer | `kb/90-ephemeral/tien-do/<module>.json` → `make kb` (`/progress`) |
| Why it changed · what was decided | ADR in `kb/10-decisions/` (`/decision`) — only when an invariant, boundary or contract moved |
| What assumptions were made · what the customer confirmed | ledger item + `ban-giao-phien.md` §1 |
| Traps · parallel sessions · next steps pointer | `kb/90-ephemeral/ban-giao-phien.md` (`/handover`), rewritten in full |
| Requirement repo moved | `kb/50-doi-chieu/` via `require-watcher` |
| **The requirement tags** `cross-context-scout` produced (below) | the module's ledger item |
| What changed, which commits, which tests | `git log` — never copied into a document |

**The scouts' reports are discarded on purpose — except their tags.** Most of a report is
rebuildable from the code (rule 9's test says: do not write it). The tags are not: that a
requirement is `PARTIALLY DONE`, or in `CONFLICT` with `../vigov-require`, took a whole agent
to establish. They go into the ledger item of the module they belong to, in the fields that
already exist:

| Tag | Where it lands |
|---|---|
| `PARTIALLY DONE` | `tiep_theo` — what exists, what is missing, with `file:line` |
| `CONFLICT` · `UNKNOWN` the user has not yet answered | `no_confirm` names the open question, or `tiep_theo` states the conflict with both sides quoted — never resolved in the ledger |
| `ALREADY DONE` | nothing new — the existing `xong` item is the record |
| `NEW` · `CHANGED` that become work | a new item, `trang_thai: "chua_lam"` |

**Only the ledger is enforced** (`progress_guard`). `ban-giao-phien.md` is written only when
`/handover` runs, and nothing forces it. So: a session that settled a decision with the user,
or hit a trap that cost real time, runs `/handover` before ending. A session with neither
leaves it alone — rewriting it for nothing is how it fills with stale rows.

A dated session folder is a session log (rule 9, forbidden #3): worthless after a day, never
deleted. The next session reads, in this order: `ban-giao-phien.md` → `tien-do.md` → the
related requirement note → recent commits → codegraph → source.

### 0.9 Done means every line is ticked or marked NOT APPLICABLE with a reason

```
[ ] codegraph checked (0.1)            [ ] recent commits checked (context-scout)
[ ] cross-context checked              [ ] ../vigov-require checked, if related
[ ] requirement confirmed (0.3 gate)   [ ] task cards stated (0.4)
[ ] independent cards parallelised, within the cap
[ ] implemented                        [ ] validation run — make check green (0.6)
[ ] diff reviewed                      [ ] each task committed (0.7)
[ ] ledger + kb tier updated (0.8)     [ ] remaining work recorded in the ledger
```

Priorities, when two pull apart: **correctness > speed · context > assumption · confirmation >
guessing · independent tasks > one monolithic task · reusable knowledge > session-only
knowledge.**

---

## 0b. How the rest of this file is used by §0

```
0.0 small?              → §6
0.4 owner per card      → §1 (event) · §3 (layer) · §4 (symptom)
0.3 gate                → also §2 (the three questions)
0.5 parallel            → §3 pair table · skills/parallel-agents
0.6–0.8 follow-up       → §5, never optional
```

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
| 14 | The progress ledger looks wrong — an item vanished, `xong` with nothing behind it, a module gone quiet | `progress-reviewer` (read-only) |
| 15 | **`../vigov-require` has moved** — BA/PM changed the requirement, or `require_sync_guard` blocked a write | `require-watcher` |

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
| Inside one Go service | `go-service-builder` | `<name>/**`, `core/**` |
| Schema and data | `data-migration-builder` | `*/migrations/**`, backfills |
| Staff web + platform console | `admin-web-builder` | `web-admin/**`, `platform-admin/**` |
| Citizen app | `citizen-app-builder` | `citizen-app/**` |
| Knowledge | `knowledge-keeper` | `kb/` curated tiers **except `kb/50-doi-chieu/`**, `*/README.md` |
| Requirement sync | `require-watcher` | `kb/50-doi-chieu/**` — **the one carve-out from `knowledge-keeper`** |
| Tests | `test-designer` | test files anywhere |
| Web work queue | `admin-web-builder` | `tasks/web/claimed/**`, `tasks/web/done/**` — **never `open/` or `stale/`** |
| Progress ledger | **the agent that owns the module** | `kb/90-ephemeral/tien-do/<its own module>.json` — that one file, never another module's |

**Generated paths (`kb/20-contracts/**`, `kb/30-indexes/**`, `tasks/web/{open,stale}/**`,
`kb/90-ephemeral/tien-do.md`, `*.pb.go`) have no owner** — they are produced by `make kb` and
`buf generate`. Any agent editing them is a bug.

### The progress ledger, and why it is one file per module

Every agent writes progress, so on paper it is the most contended file in the repository. It is
not contended at all, for the same reason the web queue is not: **the path itself carries the
partition.**

```
kb/90-ephemeral/tien-do/<module>.json   ← written, one owner each
kb/90-ephemeral/tien-do.md              ← generated by `make kb`, read by everyone
```

- **One file per module, and the module is the directory you are already working in.** Two
  agents never open the same file, so there is no lock to take — and no lock to leave jammed
  when an agent stops mid-flight.
- **The single file people asked for is the READ side.** It is generated, so it can be one
  file without ever being a write target. A single shared *write* target is the `pending.json`
  shape forbidden above: the later write silently erases the earlier one.
- **Nothing is ever deleted.** Work finished moves to `trang_thai: "xong"` with evidence; it
  does not leave the file. `hooks/progress_guard.py` blocks a write that makes an existing
  item disappear — the case that matters is not malice, it is an agent rewriting the whole file
  from a copy it read an hour ago.
- **Blocked is derived, not stored**: an item is blocked because `no_confirm` names an OPEN
  question, never because someone set a flag. Same reasoning as rule 10, invariant 3.

Procedure: `/progress` · Audit: `progress-reviewer` · Ledger is **mandatory**, not
informational — the Stop hook blocks a session that changed a module's code and left its
ledger untouched. (The web queue above is the opposite: it gates nothing.)

### The web queue, and why it cannot be fought over

`tasks/web/` exists so the Next.js admin apps can be built **at the same time** as the backend
rather than after it. The three directories ARE the status:

```
tasks/web/open/<id>.json   →   claimed/<id>.json   →   done/<id>.json
```

- **`open/` is generated.** One file appears per route the backend has actually registered. A
  new file never collides with anything, so the backend side can emit freely.
- **Claiming is `rename`, and that is the whole locking story.** Two agents reaching for the
  same task: one rename succeeds, the other gets `ENOENT` and knows to pick another. The
  filesystem is the lock — do not add a second one, and above all do not replace this with a
  single shared `pending.json`, where the later write silently erases the earlier one.
- **A route that disappears leaves an orphan, and the three directories are treated
  differently** — because they mean three different things. A task in `open/` moves to
  `stale/`: nobody had started it, and moving beats deleting because *"why did this vanish"*
  is a question somebody will ask, and a file that moved can answer it. A task in `claimed/` is
  **not touched** — someone is building a screen against a contract that just stopped existing,
  which is the case that matters most and therefore gets reported loudly rather than quietly
  tidied out from under them. A task in `done/` is left alone and not even reported: it is a
  true record of a screen that was built for a route that later changed.
- If the route comes back with the same id, `stale/` moves **back** to `open/` rather than a
  new file appearing — unless it is already in `done/`, which means it was built.
- `<id>` is a deterministic hash of `service|METHOD|path`, and a task is emitted only when that
  id is absent from **all three** directories. So "already done" is visible without a ledger.

The queue is **informational**. It blocks nothing and gates nothing.

Zalo Mini App work is deliberately **not** in here — `citizen-app-builder` follows different
rules (no domain, weak identity), and one queue holding both is an invitation to pick up the
wrong task. It gets its own when it is needed.

### Which pairs may run at the same time

Derived from the boundaries above. **CONFLICT means sequential**, always.

| | contract | go-service | migration | admin-web | citizen-app | knowledge | test |
|---|---|---|---|---|---|---|---|
| **contract-designer** | — | ok | ok | ok | ok | **CONFLICT** | **CONFLICT** |
| **go-service-builder** | ok | *see below* | **CONFLICT** | ok | ok | **CONFLICT** | **CONFLICT** |
| **data-migration-builder** | ok | **CONFLICT** | — | ok | ok | ok | **CONFLICT** |
| **admin-web-builder** | ok | ok | ok | — | ok | ok | **CONFLICT** |
| **citizen-app-builder** | ok | ok | ok | ok | — | ok | **CONFLICT** |
| **knowledge-keeper** | **CONFLICT** | **CONFLICT** | ok | ok | ok | — | **CONFLICT** |

Where each conflict comes from:

| Pair | Overlapping path |
|---|---|
| go-service ∩ migration | `*/migrations/**` sits inside `<name>/**` |
| go-service ∩ knowledge | `*/README.md` sits inside `<name>/**` |
| contract ∩ knowledge | both write under `kb/` |
| **test-designer ∩ every writer** | see below — the reason is not the paths |

**Why `test-designer` is sequential with every writer**, stated precisely because the loose
version ("its boundary is test files anywhere") gives the right answer for the wrong reason: a
`_test.go` file is never a `.proto`, a migration, a README or a `kb/` document, so on **paths**
it collides with almost nobody. What it collides with is **shared state** — it runs
`go test ./...` across the whole module, so a package another agent is halfway through writing
surfaces as a compile error in code `test-designer` does not own, and it goes hunting a defect
that does not exist. The same applies in reverse to anything that runs `make kb`.

So the pair is sequential, but knowing *why* tells you the exception: an agent that touches no
Go and runs no repo-wide command can still go alongside it.

**Two `go-service-builder` runs on different services** are the trap: `identity/**`
and `petitions/**` are disjoint, but both may write `go.mod` and `go.sum`. Parallel
only when neither touches `core/**` or adds a dependency.

The five **read-only** agents (`context-scout`, `cross-context-scout`, `isolation-reviewer`,
`domain-expert`, `progress-reviewer`) hold no write tool and conflict with nothing — they run
alongside anything, including each other.

→ Decision procedure, shared state, and why verification stays serial:
`.claude/skills/parallel-agents/SKILL.md`

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
| 4 | Update `kb/90-ephemeral/tien-do/<module>.json` (`/progress`), then `make kb` | **Never** — `progress_guard` blocks the session from ending otherwise |

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
progress-reviewer    → the ledger is true before anyone hands it over
main session         → /review-compliance · /review-isolation · make check
                     → two-commune isolation test (rule 1)
```

---

## 6. WHEN **NOT** TO USE AN AGENT

Dispatching costs a context switch and loses the thread. Do it directly when:

- the change passes the **§0.0 test** — both conditions, not "it feels small". A one-file
  change to a route or a migration is not exempt; it runs the scouts even if it then needs no
  builder
- it is a typo, a label, a comment (which is §0.0 condition 2)
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
| `require-watcher` | write | `kb/50-doi-chieu/**` only — what BA/PM changed in `../vigov-require`, and which module here must answer it |
| `isolation-reviewer` | **read only** | Three isolation dimensions, including cross-file relations |
| `domain-expert` | **read only** | Vietnamese public administration business correctness |
| `progress-reviewer` | **read only** | The progress ledger as a record: lost items, `xong` without evidence, modules gone quiet |
| `context-scout` | **read only** | Discovery 1 (§0.2): the request's own area — symbols, call paths, recent commits, current behaviour |
| `cross-context-scout` | **read only** | Discovery 2 (§0.2): other modules, `../vigov-require`, earlier sessions — tags each requirement NEW · CHANGED · DONE · PARTIAL · CONFLICT · UNKNOWN |

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
