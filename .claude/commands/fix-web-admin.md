---
description: Fix one bug on a business menu of the staff Web Admin — reproduce, find the root cause, regression test first, fix, validate, commit, ledger
group: Phát triển
argument-hint: "--menu=<menu> --des=<mô tả lỗi> — ví dụ: --menu=nhiem-vu --des=không áp dụng được bộ lọc ngày tháng ở màn hình danh sách"
allowed-tools: Read, Grep, Glob, Bash, Edit, Write, Agent, AskUserQuestion, mcp__claude-in-chrome
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

## 1b. Is it a UI defect? — decides whether §2b, §5b and the PASS verdict apply

A **UI defect** is anything about what the screen shows or how it presents it: layout, position,
size, spacing, a label or its wording, order, a missing/extra button, column or field, dialog vs
inline, an absent state (loading/empty/error/no permission), colour, icon. A defect that is
**only** data or behaviour (the filter sends the wrong date, the list is short) is not — but one
`--des` can be both; then both paths apply.

UI defect → read `.claude/skills/ui-ux-prototype-fidelity/SKILL.md` in full **now**. Its §1 decides
what "expected" means for the rest of this command: the prototype
`../vigov-require/apps/admin`, overridden only by the ADRs it ranks above it. Sibling repo
missing → STOP and say so (a UI fix without the prototype is a fix toward a guess).

**Screenshots.** The main session captures them — subagents cannot drive a browser. Before §2:
the user's image if they gave one (note its state and width), and one of the **current** rendered
screen in the same state (claude-in-chrome on `npm run dev` in `web-admin/` or the deployed site;
saved under the scratchpad). Cannot capture → say so and ask the user for one; carry on, but no
step may later claim a match.

## 2. Discovery — aimed at the defect, not the whole menu

`codegraph_status` with `projectPath` (ROUTING §0.1), then ONE message, in parallel:

| Agent | Brief must carry, beyond skill §2 |
|---|---|
| `context-scout` | the `--des` verbatim · "trace the failing behaviour end to end: control → state/URL params → `src/lib/api/` call → request actually sent → the route in `kb/20-contracts/openapi.json` → whether the server applies it. Name the **first** point where the value is lost or wrong, with `file:line`. Recent commits on that path". UI defect: "trace the **winning** CSS for every property in doubt — the component's classes, the shared component, and `src/app/globals.css` (a rule outside `@layer` beats every Tailwind class)" |
| `cross-context-scout` | the `--des` verbatim · "what is the **expected** behaviour and where is it written: the spec `docs/ui-ux/NN-<slug>.md`, `../vigov-require`, the ledger. Other menus sharing the same component/hook/API — they may carry the same bug or break with the fix" |
| `ui-ux-reviewer` | **UI defect only — mode BASELINE.** The `--des` verbatim · menu, route, the screen state · the screenshot paths from §1b · "write the prototype baseline (skill §2) with file:line in `../vigov-require/apps/admin`, the ADR exceptions, and the full difference table (skill §3) against the current screenshot. List every difference, not only the one in `--des`" |
| `domain-expert` | only when the expected behaviour itself is a business rule (which date a filter means, a status, a deadline, a figure) |

**Check the baseline before using it** (skill §5, step 1): every row cites file:line or an image;
every ALLOWED cites an ADR; nothing in it comes from the current `web-admin` code. A gap → send it
back to `ui-ux-reviewer` with the gap named, before §3.

## 3. Diagnosis — shown to the user, never written to a file

```
Symptom          the --des, verbatim
Reproduction     the steps, or why it could not be reproduced on this machine
Root cause       [EVIDENCE] file:line — the FIRST point where it goes wrong
Layer            web-admin · backend · contract (schema.gen.ts ↔ openapi.json drift)
Expected         [EVIDENCE] spec / require / ledger — or [RECOMMENDATION] with the reason
                 UI defect: the ui-ux-reviewer baseline — prototype file:line, never the current code
Differences      UI defect: the MISMATCH rows of the reviewer's table, each mapped to the root
                 cause that explains it; a row no cause explains is listed, not dropped
Fix              the smallest change at the root cause, not at the symptom
Regression test  which test, and that it fails before the fix
Also affected    other menus sharing the code (cross-context-scout)
```

For a UI defect, a regression test that asserts a class name is **not** enough on its own (skill
§4): it stays, but the proof is the §5b screenshot verdict.

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
| UI defect: a `ui-ux-reviewer` STOP CONDITION (prototype vs ADR, prototype vs user image, prototype silent on the state, prototype needs a backend change) | Follow the ranking of skill §1; when it does not settle it, ask — never design a third option |

None holds → state the diagnosis and proceed; restoring documented behaviour is what was asked.
Could not reproduce and no root cause with `file:line` → STOP and report; never fix a guess.

## 5. Fix — test first, one card

One card, owner `admin-web-builder` (ROUTING §3), regression test with `test-designer`:

1. The regression test is written **first** and run — it must fail for the reason in the
   diagnosis. A test that passes before the fix tests something else.
2. The fix, at the root cause only. Anything else found goes back as `OUT-OF-SCOPE`.
3. Validate (ROUTING §0.6): in `web-admin/`, `npm run typecheck` · `npm run lint` ·
   `npm run test`; then `make check` at the repo root. The new test now passes.
3b. **UI defect — validate against the prototype (§5b) when a screenshot can be taken.** No PASS,
   no claim of "khớp prototype" — but the code is still committed and pushed (owner, 09/10/2026).
4. Review the diff, commit by explicit path: `fix(web-admin): <menu> — <the defect, short>`, then
   `git push origin main` at once (ROUTING §0.7). Not verified by screenshot → say so in the
   message and the ledger ("chưa kiểm bằng ảnh render").
5. Ledger (skill §7): an item in the module's `kb/90-ephemeral/tien-do/<module>.json` with
   `"menu": "<slug>"`, the sha and the validation run in `bang_chung`; correct any `xong` item §1
   found to be wrong. Then `make kb`.

The `admin-web-builder` brief for a UI defect carries the reviewer's baseline and MISMATCH rows
verbatim, the skill path, and: "change only what closes these rows; anything the prototype does
not show is out of scope — no own design, no 'improvement'".

## 5b. UI defect — the prototype check, repeated until PASS

After the builder returns and step 3 is green:

1. The main session captures the rendered screen again — same state, same width as the baseline
   picture (`npm run dev` in `web-admin/`, claude-in-chrome). If the changed component or CSS is
   shared (§2 `cross-context-scout`), one more screenshot of another screen that uses it.
2. `ui-ux-reviewer`, **mode VALIDATE**: the new screenshot paths + the baseline. It returns
   PASS / FAIL / UNVERIFIABLE with the full skill §3 table.
3. **FAIL** → back to `admin-web-builder` with the MISMATCH rows verbatim, then step 3 and this
   section again. Three rounds without PASS → STOP and show the user the table; the cause is
   probably not where the diagnosis put it.
4. **UNVERIFIABLE** (no browser, dev server down, no picture) → commit + push anyway (step 4),
   marked unverified in the message and the ledger; ask the user to check on the build machine.
   Never word the result as "matches the prototype". Finished code never waits on a picture.
5. **PASS** → step 4 (commit + push). Put the verdict and the screenshot file names in the ledger item's
   `bang_chung`.

## 6. Final response

Skill §8, with `Root cause: file:line` and `Regression test: <file> — failed before, passes
after` added, and every DEPENDENCY or open question under `Remaining`. UI defect: add
`Prototype check: PASS (ui-ux-reviewer, round n)` and every ALLOWED row with its ADR, so the user
sees each place the screen deliberately differs from the prototype.
