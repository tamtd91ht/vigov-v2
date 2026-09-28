---
description: Break the outstanding work in the progress ledger into task cards grouped in waves an agent fleet can run — blocked items listed apart, collision risks stated
group: Tiến độ & bàn giao
argument-hint: "[module | menu slug] — omit for the whole backlog"
allowed-tools: Read, Grep, Glob, Bash
---

# /plan-backlog

Turns *"what is still owed"* into *"what can be dispatched now, in which order, by whom"* —
without rescanning the source tree. It is **read-only**: it prints cards for the user to approve
and dispatches nothing. Dispatch stays with the main session (ROUTING §0, "routing is the main
session's job").

**What it does not promise, stated first:** full independence. A feature crossing layers is
sequential by construction (ROUTING §3, cross-layer changes), Go builders are capped, and
`test-designer` is serial with every writer. The command **minimises** collisions and **names**
the ones it could not remove. It reads free-text ledger prose, so two runs may split work
slightly differently — the user accepted that cost on 2026-09-28 rather than adding ledger fields.

---

## INPUTS — in this order, and nothing else unless a step says so

| # | Source | Used for |
|---|---|---|
| 1 | `kb/90-ephemeral/tien-do/*.json` — every item whose `trang_thai` is `chua_lam` or `dang_lam` | The backlog. `treo` is excluded: it means *we chose not to* |
| 2 | `kb/00-foundation/open-questions.json` | Which questions are still `OPEN` |
| 3 | `.claude/agents/ROUTING.md` §3 (write boundaries, pair table) + `.claude/skills/parallel-agents/SKILL.md` | Owner per card, what may share a wave |
| 4 | `kb/30-indexes/code-map.json`, `data-ownership.json`, `api-surface.json` | Resolving a vague `tiep_theo` to a module or file |
| 5 | `kb/90-ephemeral/ban-giao-phien.md` (if not expired) and `tasks/web/open/` | Decisions and traps not yet in the ledger; web tasks already queued |

`$ARGUMENTS` filters step 1: a module directory name (`citizen-app`) keeps that file; a menu slug
keeps items whose `menu` matches (`python tools/tien_do.py --menu "<slug>"` prints them).

**Source code is opened only in step 3 below**, and only the files the item itself names. Never
grep the repository to "get a feel" — that is the rediscovery this command exists to avoid.

---

## PROCEDURE

### 1. Classify every item — READY or BLOCKED

An item is **BLOCKED** when any of these holds; record *what* it waits for, quoting the words:

| Waits for | Evidence |
|---|---|
| The customer | `no_confirm` non-empty, **or** `tiep_theo` cites an open question `#N` that step 2 shows `OPEN` |
| Infrastructure | `tiep_theo` says something is not deployed / not configured (storage, broker, scanner, DSN, a secret) |
| Another item | `tiep_theo` says it needs another item, service, route or contract first — resolve it to that item's `id` if one exists |
| The user | `tiep_theo` says a decision is pending with the user, or a STOP CONDITION of rules 1–13 fires |

A question cited as `#N` that step 2 shows `DECIDED` does **not** block. Anything else is READY.
When unsure, classify BLOCKED and say why — a card that stalls a builder mid-flight costs more
than a card left for later.

### 2. Map each READY item to a layer and an owner

Use ROUTING §3: the ledger file's module gives the first guess (`service-x` → `go-service-builder`,
`web-admin` → `admin-web-builder`, `citizen-app` → `citizen-app-builder`, `proto` →
`contract-designer`, migrations → `data-migration-builder`, `_chung` → read the item). An item that
mentions a contract, a migration **and** a screen is several layers — go to step 3.

Business-behaviour items (status, SLA, deadline, numbering, figures shown to leadership) get a
`domain-expert` card **before** the builder card (ROUTING §1, "route to domain-expert first").

### 3. Split items that are too big for one card

Split when an item holds several deliverables (a list of routes, several screens) or spans
layers. Split **by layer first** (contract → migration → service → client), then by disjoint
file set inside one layer. Open only the files the item names, plus `code-map.json` entries for
the symbols it names. If splitting would need wider reading, emit it as one card marked
`SPLIT-NEEDED` and say what must be read — do not read it now.

### 4. Build waves

Place each card in the earliest wave where all of these hold:

1. Every card it depends on is in an **earlier** wave.
2. No other card in the wave has the **same owner on the same module** — two builders in one
   service collide on its files and its `go.mod`.
3. At most **2 cards that compile Go** in the wave (1 is the safe number — `skills/parallel-agents`).
   Two Go cards in one wave: neither touches `core/**` nor adds a dependency.
4. Pairs marked CONFLICT in ROUTING §3 never share a wave.
5. `test-designer` and `knowledge-keeper` cards never share a wave with a writer.

What remains as collision risk after these rules — two cards likely to touch one shared file, a
generated tier both will regenerate, a ledger file two cards write — goes in the card's `Risk`.

### 5. Print — in chat only

Never write the plan to a file: a dated plan is a session log (rule 9, forbidden #3). The
ledger stays the one record; the cards are its current reading.

---

## OUTPUT

Prose for the user in Vietnamese (CLAUDE.md, WORKING BEHAVIOUR); card fields may stay as below.

```
## Tóm tắt
<N> mục còn nợ · <R> sẵn sàng → <C> thẻ trong <W> đợt · <B> đang chặn

## Đợt 1 — chạy song song
TASK-01 · <title>
Goal · Scope (in / out) · Files/Modules · Owner agent
Dependencies: — · Source item: <module>/<id>
Input · Expected Output · Validation: <exact command> · Risk: <collision risk, or "—">

TASK-02 · …

## Đợt 2 — sau đợt 1
…

## Đang chặn — chờ gì
| Mục | Chờ | Trích nguyên văn |
|---|---|---|
| <module>/<id> | câu hỏi #N · hạ tầng · mục <id> · người dùng | "<words from tiep_theo>" |

## Cần tách thêm (SPLIT-NEEDED)
| Mục | Phải đọc gì để tách |
```

Every card carries its **source item** (`<module>/<id>`) so the builder updates that ledger item
and nothing else. Validation is a real command — `go test ./...` inside the service with
`GOCACHE`/`GOTMPDIR` set per the Windows note, `npx vitest run --maxWorkers=1` for a web app —
never "verify it works".

---

## REFUSALS

- **Never dispatch.** Print the cards and stop; the user approves, the main session dispatches.
- **Never decide an open question** or pick a side of a conflict to make an item READY — list it
  under "Đang chặn" (ROUTING §8).
- **Never edit the ledger** here, not even to fix an item that looks wrong — report it; the
  module's owner edits it (`/progress`).
- **Never invent a card** the ledger does not support. Work discovered while reading goes in the
  report as "chưa có trong sổ", for `/progress` to record.
- **No personal data, no secrets** in a card, not even quoted from a ledger item (rules 3 and 8).
