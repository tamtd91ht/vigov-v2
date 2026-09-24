---
name: develop-menu
description: The shared procedure behind /develop-web-admin, /develop-backend-api, /develop-miniapp and /develop-feature (all three platforms in one run) — continue building one existing business menu on one platform, from recovered knowledge through discovery, a tagged proposal, a confirmation gate, per-task commits and per-menu documentation. Triggers on: develop menu, phát triển tiếp, menu nghiệp vụ, develop-web-admin, develop-backend-api, develop-miniapp, tiếp tục menu, làm tiếp nhiệm vụ, BACKEND DEPENDENCY, [EVIDENCE], [RECOMMENDATION].
---

# Skill: Continue developing one business menu

The three single-platform `/develop-*` commands differ **only** in platform scope;
`/develop-feature` runs all three for one feature and adds the platform decision, the phase
order and the backend→client handoff gate (its own file). Everything else is this
file — three copies of one procedure are three copies that drift (rule 9). The workflow itself
is `.claude/agents/ROUTING.md` §0; this file adds what a **menu** needs on top of it and never
relaxes a gate there.

## 0. Resolve the menu — or stop

| Situation | Action |
|---|---|
| No argument | STOP. Say exactly: *"Vui lòng truyền tên menu nghiệp vụ cần phát triển. Ví dụ: /develop-web-admin Nhiệm vụ"* |
| `python tools/tien_do.py --menu "<menu>"` exits **2** | STOP. Show the candidate list it printed and ask. **Never pick the closest** — a guessed menu is work done on the wrong business area |
| Exits **0** | The menu is its slug; the spec is the `docs/ui-ux/NN-<slug>.md` it printed |

The catalogue is `docs/ui-ux/NN-<slug>.md` (`hooks/_common.py` `cac_menu`), resolved in one
place for the commands, the ledger guard and the generator alike.

Then the platform check, from the command's own file. A menu with no surface on that platform
(`/develop-miniapp Nhiệm vụ` — citizens never see tasks) is a STOP, not an empty run.

## 1. Read the knowledge that already exists — before any discovery

A second run on the same menu must not start from zero. Read, in this order:

| # | Source | Gives |
|---|---|---|
| 1 | `python tools/tien_do.py --menu "<menu>"` | Every ledger item of every module tagged with this menu: done · in progress · parked · owed to the customer |
| 2 | the spec it points to, `docs/ui-ux/NN-<slug>.md` | Purpose, business workflow, actors, screens — the customer's words |
| 3 | `kb/90-ephemeral/ban-giao-phien.md` | Decisions settled, traps already paid for |
| 4 | `kb/50-doi-chieu/` notes + `neo.json` | What the requirement repo changed, and up to where it was read |
| 5 | `kb/00-foundation/open-questions.json` | Questions still owed — a hit on this menu is a gate item |

These go **into the scouts' briefs**, so the scouts extend what is known rather than
rediscover it.

**Why there is no `docs/ai-context/menus/<menu>.md`.** The proposal behind this command asked
for one, and asked in the same breath to reuse an equivalent store if one exists. Both halves
of it exist: the half nobody can regenerate (purpose, workflow, actors) is the spec; the
state half (done, pending, open questions) is the ledger, now cut by menu through the `menu`
key. APIs, data model and permissions are regenerated from source (`kb/30-indexes/`,
`kb/20-contracts/openapi.json`). A hand-written menu file would copy all three and drift from
each (rule 9; `doc_guard` blocks it).

## 2. codegraph, then discovery — ROUTING §0.1–§0.2

`codegraph_status` **with `projectPath`** first (ROUTING §0.1). Then ONE message, in parallel:

| Agent | Brief must carry |
|---|---|
| `context-scout` | menu · slug · spec path · platform · the §1 findings · "cover: screen/page, component, state, API, domain, model, table, service, workflow, permission, validation, tests, commits touching these" |
| `cross-context-scout` | the same, plus "walk the domain to the OTHER menus that share its entities, APIs, events, permissions; tag every requirement; recover previous-session items from the §1 ledger view" |
| `domain-expert` | **when** the change touches a status, a deadline, numbering, a role, or a figure (ROUTING §1): the commune's real workflow for this menu — *tiếp nhận → phân công → xử lý → phê duyệt → trả kết quả → lưu hồ sơ*, or the one that applies |

The scouts keep their **one** output contract. The proposal listed its own headings per agent
(`Existing APIs`, `Related Menus`…); those are sub-bullets under the scouts' standard headings,
not a second contract — two contracts for one agent are two things the next report ignores.

## 3. The proposal — every claim carries its source

Drafted per ROUTING §0.2 (`Plan` agent when large). Shown to the user, never written to a file.

```
## 1. Current State            ## 6. Risks
## 2. Requirement              ## 7. Recommended Development Steps
## 3. New/Changed Requirements ## 8. Task Decomposition
## 4. Related Context          ## 9. Parallelization Plan
## 5. Impact                   ## 10. Validation Plan
```

Every statement is tagged:

| Tag | Means | Must carry |
|---|---|---|
| `[EVIDENCE]` | from the spec, `../vigov-require`, the ledger, code or a commit | `file:line` or a sha |
| `[RECOMMENDATION]` | from domain knowledge or judgement — **not a requirement** | the reason |

A `[RECOMMENDATION]` is never built as if confirmed. If the plan depends on one, it is a
confirmation item. Requirement changes are shown as `R-nn · TAG · Old · New · Impact`, the tags
of `cross-context-scout`.

## 4. Confirmation — ROUTING §0.3, in this shape

Ask only what is ambiguous, risky, business-sensitive, architecture-impacting, or
requirement-conflicting. Small technical choices are decided and stated, not asked. A STOP
CONDITION of rules 1–11 is **never** a small technical choice.

Up to four questions → **AskUserQuestion**, recommended option first; its free-text *Other*
is the proposal's `Input: [ type something ]`. More than four, or when options do not fit, write
each as:

```
### Confirmation #n
Question: …
Context: …                    ([EVIDENCE] file:line)
Current behavior: …
New requirement: …
Recommendation: …             (the default if the user answers "yes")
Reason: …
Input: [ type something ]
```

`yes` = take the recommendation. Anything typed = take what was typed. Then **wait**.

## 5. Task cards — ROUTING §0.4, plus the platform line

Each card adds `Agent` (the owner from ROUTING §3) and `Platform: in scope | DEPENDENCY`.

A card outside the command's platform is **not implemented**. It is marked
`BACKEND DEPENDENCY` (or `WEB DEPENDENCY` / `MINIAPP DEPENDENCY` / `CONTRACT DEPENDENCY`),
written as a `chua_lam` ledger item in the module that owns it, tagged with this `menu`, and
reported. The commands' scope is a promise to the user about which tree changes.
Under `/develop-feature` such a card **is** built, and the mark becomes an ordering edge
(`develop-feature.md` §2).

Split by layer and never as one card per feature ("Implement Nhiệm vụ" is not a card).

## 6. Implement, validate, commit — per card

Parallel only within ROUTING §0.5 (≤2 Go-compiling agents; one feature's backend layers are
sequential). Then, **per card**, in the main session:

```
builder returns → validate (ROUTING §0.6) → review the diff → commit (ROUTING §0.7) → document (§7)
```

**Builders do not commit; the main session commits each card after validating it.** Two
agents committing at once race on `.git/index.lock`, and an agent cannot know whether the tree
it would commit still compiles with another agent's half-written work beside it. One card ·
one purpose · one commit, staged by explicit path, no debug code, no unneeded generated files.

## 7. Document after every commit — not at the end

Immediately after each commit:

| What | Where |
|---|---|
| The card's ledger item: `trang_thai`, `bang_chung` (the sha + the validation run), `tiep_theo`, **`"menu": "<slug>"`** | `kb/90-ephemeral/tien-do/<module>.json`, then `make kb` |
| Every pre-existing item the scouts found for this menu, still untagged | add `"menu": "<slug>"` — this is how the menu view fills in over runs |
| A decision that moved an invariant, boundary or contract | ADR, `/decision` |
| A business decision the user confirmed · a trap that cost time | `/handover` before the session ends (ROUTING §0.8) |

No `docs/ai-context/sessions/YYYY-MM-DD/`: a dated folder is a session log (rule 9, forbidden
#3). Its `summary`/`decisions`/`tasks` land in the ledger, the ADRs and the handover as above,
and the commit list is `git log`.

## 8. Final response

```
## Development Summary
Menu: <tên> (`<slug>`, docs/ui-ux/NN-<slug>.md)
Platform: …
Completed: TASK-nn …            Commits: <sha> <subject> …
Validation: <command> → <result>, and what did NOT run on this machine
Documentation: ledger items touched (module/id) · ADR · handover
Remaining: every card not done, every DEPENDENCY, every open confirmation
Next Recommended Steps: …
```

Never "completed" while a mandatory card is undone; ROUTING §0.9 decides what "done" means.

## Non-negotiable

Context before code · codegraph (with `projectPath`) before broad reads · recent commits and
cross-menu impact always · previous-session knowledge recovered first · requirement changes
detected, never assumed from the code · a recommendation is not a requirement · every
confirmation has a recommended option and a free-text override · no silent scope growth · one
card, one commit, one ledger update.
