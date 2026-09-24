---
description: Develop one feature of a business menu end to end in ONE run — backend API, Web Admin and Mini App, each only if the feature touches it
group: Phát triển
argument-hint: "<menu>[: <chức năng>] — ví dụ: Nhiệm vụ: gắn văn bản vào nhiệm vụ"
allowed-tools: Read, Grep, Glob, Bash, Edit, Write, Agent, AskUserQuestion
---

# /develop-feature `<menu>[: <chức năng>]`

Input: **$ARGUMENTS**

The part before the first `:` is the **menu** — resolved exactly as in skill §0. The part
after it, if any, is the **feature** in the user's words, and goes verbatim into every brief.
No `:` → the whole input is the menu and discovery proposes the features. Menu names contain
spaces and `&` (`Văn bản & Đơn thư`), which is why the separator is `:` and not a space.

**Read `.claude/skills/develop-menu/SKILL.md` in full and follow it.** This file adds only what
changes when one run spans all three platforms — the single-platform commands
`/develop-backend-api` · `/develop-web-admin` · `/develop-miniapp` keep their scope rules, and
this one applies **each of them to its own cards**.

## 1. Which platforms are involved — decided by evidence, confirmed by the user

Discovery (skill §2) runs once for the whole feature; the scouts' briefs say *"feature spans
backend, Web Admin and Mini App — report which of the three it actually touches"*. Then:

| Platform | Involved when | Precondition — fails → that platform is a STOP |
|---|---|---|
| Backend API | a route, use case, table, event or permission changes | owning service known (`data-ownership.json`) |
| Web Admin | the menu has a row in `web-admin/src/components/muc-menu.ts` **and** staff see or do the feature | — |
| Mini App | the spec or `citizen-app/src/` has a citizen screen for it | `../vihat-miniapp` beside this repo (CLAUDE.md) |

The proposal (skill §3) opens with a **platform table** — involved / not involved / blocked,
each row `[EVIDENCE]`-tagged. Which platforms are in the run is **confirmation #1**, always,
even when it looks obvious: it is the one decision that sets the size of everything after it.
A blocked platform does not stop the others; the user chooses to go ahead without it, or wait.

## 2. The cards form ONE plan, in dependency order

In the single-platform commands a card on another platform is a `DEPENDENCY` and is not built.
Here it **is** built — the `DEPENDENCY` marks become ordering edges between cards:

```
phase A  contract   contract-designer        proto / events / ownership         (if any)
phase B  storage    data-migration-builder   migrations                          (if any)
phase C  behaviour  go-service-builder       routes · use cases · authz · audit
         ── HANDOFF GATE (main session, alone) ─────────────────────────────────────
phase D  surfaces   admin-web-builder  ‖  citizen-app-builder                  (parallel)
phase E  tests      test-designer, per platform, after its builders — never alongside
```

A → B → C is sequential: one feature's backend layers are each other's input (ROUTING §3,
§0.5). D is the one place this command runs builders in parallel — two Node toolchains,
disjoint trees, an allowed pair in ROUTING §3. No backend card → start at D. No client card →
stop after C.

## 3. The handoff gate — between the backend and the screens

The clients must build against the contract **as it is now**, not as it was planned. After the
last phase-C card is validated and committed, in the main session, alone:

```
make kb                          # tools/apidoc → kb/20-contracts/openapi.json
cd web-admin && npm run gen:api  # → src/lib/api/schema.gen.ts   (only if Web Admin is in the run)
make check                       # check:api fails if schema.gen.ts and openapi.json disagree
codegraph sync .                 # the scouts' graph must see the new routes
```

Commit the regenerated files with the last backend card, or as their own `chore(kb)` commit —
never inside a client card. Then brief the phase-D builders with the **real** route shapes from
`openapi.json`: method, path, permission key, request and response fields. The Mini App has no
generated types (`citizen-app/src/api/` is hand-written), so its brief quotes the shapes
verbatim.

A phase-D builder that finds the contract short of what its screen needs **stops and hands
back** — it never edits the backend. The main session then opens a new phase-C card, reruns
the gate, and re-briefs. That loop is the cost of doing three platforms in one run; it is
cheaper than a screen built on a guessed field.

## 4. Commits, ledger, final response

Unchanged from the skill: validate → review diff → one commit per card → ledger item with
`menu` right after. Each card's ledger item goes in **the module that card changed**
(`service-petitions`, `web-admin`, `citizen-app`…), all tagged with the same `menu`, so
`python tools/tien_do.py --menu "<menu>"` shows the whole feature across platforms.

The final response (skill §8) adds, per platform: involved or not, cards done, commits, and
what is left. A platform the user deferred at confirmation #1 is listed under **Remaining**,
with its ledger item — never silently absent.

## When NOT to use this command

- The feature touches one platform → use that platform's command; this one only adds a gate
  and a confirmation you do not need.
- The backend change is a **contract other services consume** (a published event, a gRPC
  method) → run `/develop-backend-api` first and let it land; consumers outside this menu are
  not in this run's view.
- Several unrelated features at once → one run per feature. One run is one plan and one
  confirmation; two features in it are two sets of decisions confirmed as one.
