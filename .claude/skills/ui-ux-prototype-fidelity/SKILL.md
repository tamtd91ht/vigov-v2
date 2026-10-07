---
name: ui-ux-prototype-fidelity
description: Use when designing, fixing or reviewing any staff Web Admin screen against the prototype — layout, labels, order, buttons, columns, fields, dialog vs inline, states, spacing, colours — and whenever someone is about to say a UI change "matches the prototype". Triggers on: UI, UX, giao diện, prototype, khớp prototype, giống prototype, lệch prototype, bố cục, layout, popup, dialog, hộp thoại, drawer, ngăn, modal, kanban, bảng, cột, nút, nhãn, label, spacing, căn lề, padding, màu, CSS, Tailwind, screenshot, ảnh chụp màn hình, ảnh prototype, ui-ux-reviewer, fix-web-admin.
---

# UI/UX — faithful to the prototype, proven by a rendered picture

A screen is "done" when a **rendered screenshot** of it, set beside the prototype, shows no
unlisted difference. Not when the CSS reads right, not when a test finds a class name.

**Why this is a skill and not a rule:** "looks like the prototype" is a judgement on a picture.
No hook can decide it, so it is reviewed — by `ui-ux-reviewer`, at every step.

**Why it exists at all:** on 07/10/2026 three successive fixes of the *Giao việc mới* popup and the
Kanban (9898a14d, bbcaf3c7, b4726b15) were reported "matches the prototype" from reading CSS and a
"has class X" test. Production showed the popup still glued to the top edge and too narrow: a rule
in `web-admin/src/app/globals.css` sitting **outside `@layer`** beat every Tailwind class, and no
class-level test could see it. The owner had supplied the prototype before the first fix.

---

## 1. Where the prototype is — and which source wins

| Rank | Source | Decides |
|---|---|---|
| 1 | Hard rules and ADRs listed in ADR 0068 *lần 5* **#4** | Always win, whatever the prototype shows (commune from `Host`, masking, soft delete with reason, issued codes immutable, Vietnamese status codes + two deadline clocks, permissions per rule 5, …) |
| 2 | An owner decision recorded in `kb/10-decisions/` **after** 06/10/2026 that names the screen (e.g. ADR 0076 for the task drawer) | Wins over the prototype for the points it names |
| 3 | **`../vigov-require/apps/admin/src/app/**`** — the prototype's source, route by route | **Structure**: layout, labels (verbatim), order, buttons, columns, fields, dialog vs inline, states |
| 4 | A screenshot the user supplied in the conversation | Same as 3 — and it shows the **state** the user cares about. Read the size and the state it was taken in |
| 5 | `docs/ui-ux/NN-<slug>.md` (+ `15-phu-luc-giao-dien-chung.md` for shared parts) | Text transcribed from the running prototype — verbatim strings, behaviour. Below 3 when they disagree |
| 6 | ADR 0068 *lần 2* (OMICALL CRM tokens) + *lần 4* (left sidebar) | **CSS only**: colours, sizes (control 36px, header 68px, row 64px), radius, shadow. "Prototype = frame, CSS = new UI kit" |

**Not a source:** `../vigov-require/vigov-prototype.html` (the first static mock — ADR 0068 lần 5 #1),
the current `web-admin` code (it is what is being judged), your own taste.

A presentation **preference** written before 06/10/2026 that the prototype contradicts is
**replaced** by the prototype (lần 5 #3). A **hard rule** (rank 1) is never replaced.

The prototype has something that needs a backend route that does not exist → draw it **in the
prototype's position** as a disabled control with the "?" marker (ADR 0068 §14). Never fake it,
never drop it silently (lần 5 #5).

Prototype silent, spec silent, no ADR → **ask**. Never design a third option.

---

## 2. The baseline — written BEFORE any code changes

For the screen and the state in question, list from rank 3/4 (file:line or image):

| Aspect | What to record |
|---|---|
| Container | Page / dialog / drawer; its width, max-height, distance from each viewport edge, scroll owner |
| Layout | Columns, grid, order of blocks top → bottom, left → right |
| Text | Every visible label, title, placeholder, button text, empty-state sentence — **verbatim** |
| Controls | Every button (primary vs secondary vs ⋯ menu), field, filter, column — in order |
| Interaction | What opens what (dialog vs inline vs navigate), what closes it, keyboard, URL |
| States | Loading · empty · empty-by-filter · error + Tải lại · no permission · disabled "?" |
| Exceptions | Every point where rank 1 or 2 overrides the prototype, with the ADR |

This baseline is the **expected** of the diagnosis. Anything not in it is not a requirement.

---

## 3. The fidelity check — one row per difference, never a summary

| # | Aspect | Prototype (file:line / image) | Rendered now (screenshot) | Verdict |
|---|---|---|---|---|
| 1 | Dialog top gap | `<prototype component>:<line>` — centered, gap from top | touches the top edge | **MISMATCH** |
| 2 | Status badge | icon + text | text only | **MISMATCH** — also ADR 0068 (never colour/text alone) |
| 3 | "Xoá" in header tabs | present | absent | **ALLOWED** — ADR 0076 #1 |

Verdicts: **MATCH** · **MISMATCH** · **ALLOWED** (cite rank 1/2) · **UNVERIFIABLE** (say what
evidence is missing). List **every** difference you see, not the first three — a partial list is
how the second and third fixes of 07/10 happened.

---

## 4. Evidence that counts — and evidence that does not

| Counts | Does not count |
|---|---|
| A screenshot of the **rendered** screen (local `npm run dev` or the deployed site), same state and similar width as the prototype image | "The class `mt-12` is there" |
| The **winning** CSS rule for a property: the built stylesheet or computed style, including anything in `globals.css` outside `@layer` | Reading the component's Tailwind classes |
| The prototype's source at file:line, or the user's image | Memory of what the prototype looked like |

Subagents cannot drive a browser. **The main session captures the screenshots** (claude-in-chrome
on the running app, or the user provides them) and passes the file paths; `ui-ux-reviewer` reads
them with `Read`.

No rendered screenshot → the verdict is **UNVERIFIABLE**, and nobody may say "matches the
prototype". Say instead: *"chưa kiểm bằng ảnh render — chưa được báo là khớp"*.

---

## 5. Validate at every step, not once at the end

| Step | Check | Who |
|---|---|---|
| Before the fix | Baseline (§2) complete; every exception cites its ADR | `ui-ux-reviewer` |
| Diagnosis | The root cause explains **every** MISMATCH row, or the rest are listed as separate | main session |
| After each change | New screenshot → §3 table again. Any MISMATCH left → back to the builder with the rows | main session + `ui-ux-reviewer` |
| Before commit | §3 table has no MISMATCH and no UNVERIFIABLE on the rows the defect covers | `ui-ux-reviewer` verdict **PASS** |
| Regression | Other screens sharing the changed component/CSS: screenshot at least one | main session |

---

## 6. Things that look like improvements and are not

- Adding a feature "to look modern" (Ctrl+K, AI, dark mode, charts) — ROADMAP_PHASE2, not now
- blur, gradient, decorative shapes — ADR 0068 §11
- Renaming a label to "better" Vietnamese — the prototype's string is the product
- Moving a control because it "flows better" — order is part of the prototype
- "Sắp có" / "Chưa có" badges — replaced by the disabled "?" control (§14)

→ Agent: `ui-ux-reviewer` · Builder: `admin-web-builder` · Decisions: ADR 0068 (lần 2–5), ADR 0076
→ Related skills: `accessibility-elderly` (contrast, targets) · `administrative-language`
