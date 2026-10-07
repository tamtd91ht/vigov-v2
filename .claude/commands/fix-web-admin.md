---
description: Fix one bug on a business menu of the staff Web Admin — reproduce, find the root cause, regression test first, fix, validate, commit, ledger
group: Phát triển
argument-hint: "--menu=<menu> --des=<mô tả lỗi> — ví dụ: --menu=nhiem-vu --des=không áp dụng được bộ lọc ngày tháng ở màn hình danh sách"
allowed-tools: Read, Grep, Glob, Bash, Edit, Write, Agent, AskUserQuestion
---

# /fix-web-admin `--menu=<menu> --des=<mô tả lỗi>`

Input: **$ARGUMENTS**

A bug fix is still code that changes control flow, so it is **never** ROUTING §0.0-exempt. This
file is `/develop-web-admin` narrowed to one defect: **read `.claude/skills/develop-menu/SKILL.md`
in full** — menu resolution, knowledge recovery, confirmation shape, per-card commit and ledger
come from there. The platform check and the scope table come from
`.claude/commands/develop-web-admin.md`. This file adds only what a **bug** changes.

## 0. Parse the two parameters — or stop

| Parameter | Rule |
|---|---|
| `--menu=` | Up to the next ` --` or the end. A slug (`nhiem-vu`) or a display name (`Nhiệm vụ`) — both resolve through skill §0 (`python tools/tien_do.py --menu "<menu>"`) |
| `--des=` | **Everything** after `--des=` to the end, verbatim, spaces included. Goes word for word into every brief — never paraphrased, never "improved" |

Either missing → STOP. Say exactly: *"Vui lòng truyền đủ hai tham số. Ví dụ: /fix-web-admin
--menu=nhiem-vu --des=không áp dụng được bộ lọc ngày tháng ở màn hình danh sách"*

Menu ambiguous (exit 2) → show the candidates and ask; never pick the closest. No row in
`web-admin/src/components/muc-menu.ts` → STOP (develop-web-admin platform check).

## 1. Known already? — before any discovery

Skill §1 in full, then look for **this** defect specifically:

- the ledger view of the menu — an item already describing it (`chua_lam`, `tiep_theo`) or a
  `xong` item that claims the behaviour works (that claim is now suspect — cite it);
- `git log --oneline -- web-admin/src/app/**/<menu route>/**` — the commit that last touched the
  failing behaviour is the first suspect, not proof.

## 2. Discovery — aimed at the defect, not the whole menu

`codegraph_status` with `projectPath` (ROUTING §0.1), then ONE message, in parallel:

| Agent | Brief must carry, beyond skill §2 |
|---|---|
| `context-scout` | the `--des` verbatim · "trace the failing behaviour end to end: control → state/URL params → `src/lib/api/` call → request actually sent → the route in `kb/20-contracts/openapi.json` → whether the server applies it. Name the **first** point where the value is lost or wrong, with `file:line`. Recent commits on that path" |
| `cross-context-scout` | the `--des` verbatim · "what is the **expected** behaviour and where is it written: the spec `docs/ui-ux/NN-<slug>.md`, `../vigov-require`, the ledger. Other menus sharing the same component/hook/API — they may carry the same bug or break with the fix" |
| `domain-expert` | only when the expected behaviour itself is a business rule (which date a filter means, a status, a deadline, a figure) |

## 3. Diagnosis — shown to the user, never written to a file

```
Symptom          the --des, verbatim
Reproduction     the steps, or why it could not be reproduced on this machine
Root cause       [EVIDENCE] file:line — the FIRST point where it goes wrong
Layer            web-admin · backend · contract (schema.gen.ts ↔ openapi.json drift)
Expected         [EVIDENCE] spec / require / ledger — or [RECOMMENDATION] with the reason
Fix              the smallest change at the root cause, not at the symptom
Regression test  which test, and that it fails before the fix
Also affected    other menus sharing the code (cross-context-scout)
```

A fix applied at the symptom (re-filtering on the client what the server returned unfiltered,
hiding a field) is not a fix — it hides the defect from the next screen that calls the same API.

## 4. Gate — STOP and ask (AskUserQuestion, recommended option first) when

| Gate | Default recommendation |
|---|---|
| Root cause is **outside `web-admin/**`** (server ignores the parameter, contract lacks it) | Record a `BACKEND DEPENDENCY` / `CONTRACT DEPENDENCY` ledger item tagged with the menu and propose `/develop-backend-api <menu>` — this command never edits the server |
| Expected behaviour is written nowhere (only `[RECOMMENDATION]`) | Ask which behaviour is right; never infer it from the code |
| The fix changes behaviour beyond restoring what the spec states | Ask |
| The fix touches shared code another menu uses | Ask, naming the other menus |
| Any ROUTING §0.3 gate item or rule 1–13 STOP CONDITION | Ask |

None holds → state the diagnosis and proceed; restoring documented behaviour is what was asked.
Could not reproduce and no root cause with `file:line` → STOP and report; never fix a guess.

## 5. Fix — test first, one card

One card, owner `admin-web-builder` (ROUTING §3), regression test with `test-designer`:

1. The regression test is written **first** and run — it must fail for the reason in the
   diagnosis. A test that passes before the fix tests something else.
2. The fix, at the root cause only. Anything else found goes back as `OUT-OF-SCOPE`.
3. Validate (ROUTING §0.6): in `web-admin/`, `npm run typecheck` · `npm run lint` ·
   `npm run test`; then `make check` at the repo root. The new test now passes.
4. Review the diff, commit by explicit path: `fix(web-admin): <menu> — <the defect, short>`.
5. Ledger (skill §7): an item in the module's `kb/90-ephemeral/tien-do/<module>.json` with
   `"menu": "<slug>"`, the sha and the validation run in `bang_chung`; correct any `xong` item §1
   found to be wrong. Then `make kb`.

## 6. Final response

Skill §8, with `Root cause: file:line` and `Regression test: <file> — failed before, passes
after` added, and every DEPENDENCY or open question under `Remaining`.
