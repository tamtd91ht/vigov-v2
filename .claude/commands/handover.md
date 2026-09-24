---
description: Rewrite the single session-handover note so another session, on another machine, can carry on
group: Tiến độ & bàn giao
argument-hint: "(none) — this command reads the repository, not your memory"
allowed-tools: Read, Grep, Glob, Bash, Edit, Write
---

# /handover

AI has no memory between sessions, and the next session may open on a different machine with
nothing but this repository. Everything not written down is rediscovered — expensively, slowly,
and **reaching a different conclusion each time** (rule 9).

This command writes the one thing `git log` cannot: **what was decided, what is blocked and on
whom, and which traps have already cost somebody an hour.**

---

## THE FILE

**`kb/90-ephemeral/ban-giao-phien.md` — exactly one, rewritten IN FULL, every time.**

| | |
|---|---|
| Never append | An appended file becomes a session log, which rule 9 forbidden #3 names outright: worthless after a day, yet never deleted |
| Never a second dated copy | Two handover files are two files that disagree, and a reader cannot tell which is current. The date lives INSIDE the file, not in its name |
| Never a summary of the diff | `git log`, `git diff` and the generated indexes answer that, exactly and forever |

**The one-line test, applied to every line before writing it:** *if I delete this line, can
`git log`, `kb/30-indexes/`, `kb/20-contracts/openapi.json` or the source itself produce it
again?* **Yes → do not write it.** **No → this is the line worth writing.**

Concretely, DO NOT write: a commit list · a file tree · counts of services, routes or tests ·
a description of what a package does · a restatement of an ADR. All of those are cheaper and
truer at their own source, and each copy is a copy that will drift.

---

## PROCEDURE

### 1. Gather evidence from the repository, not from the conversation

Your recollection of this session is the least reliable input available — it is also the only
input that cannot be checked by whoever reads the file next. Run these first and write from
their output:

```bash
git log --oneline <base>..HEAD        # <base> = last commit named in the current handover
git status --short                    # uncommitted work is the most dangerous thing to lose
ls tasks/web/open tasks/web/claimed tasks/web/stale
git log --since="14 days" --name-only -- kb/90-ephemeral/tien-do/   # ledgers that moved
```

Then read `kb/90-ephemeral/tien-do.md`. A module whose code moved while its ledger stood still
is the first thing to chase — dispatch `progress-reviewer` before writing, not after.

Then read `kb/00-foundation/open-questions.json` — a question still open there is a **blocker
with a name**, and the single most valuable thing this file carries.

If other sessions are running, list them and record **which paths each one holds**. Two
sessions writing one directory is the failure this file exists to prevent.

### 2. Verify every claim you are about to make — **carried-forward ones first**

The riskiest lines in the file are not the new ones. They are the rows you keep from the
previous handover: they read as verified because they were, once, and nothing in the file says
when. A trap that has been fixed, a component recorded as missing that now exists, a blocker
the customer settled last week — each survives a rewrite unless it is checked against disk.

**Open the previous version, and for every row you intend to keep, check it on disk before
keeping it.** A row you cannot check goes out, or says plainly that it is unchecked.


A handover that says "green" when it is not is worse than no handover: the next session builds
on it. Run `make check` and record **its actual result**, including what it did NOT cover
(a linter missing from the machine, a test suite skipped for want of a DSN, an image never
built). Anything you did not run yourself is written under **CHƯA KIỂM**, never softened.

### 3. Write the file

Frontmatter — `tier: T5` and `expires` are enforced by `hooks/doc_guard.py`:

```yaml
---
id: ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: <short sha of HEAD>
expires: <today + 90 days>
owns_facts:
  - "trạng thái thi công tại <ngày> và việc kế tiếp phải làm"
---
```

Body, in **Vietnamese** (it is `kb/` prose — CLAUDE.md), in this order:

| § | Section | What belongs in it |
|---|---|---|
| 1 | **Đã làm** | Only what `git log` cannot say: decisions settled with the customer and where they are recorded; what a change was FOR |
| 2 | **Việc kế tiếp** | **A pointer only.** The work itself lives in `kb/90-ephemeral/tien-do.md`, one section per module — see below |
| 3 | **Đang bị chặn** | **A pointer only** to the generated *"Nợ khách chốt"* table, which derives from each item's `no_confirm` |
| 4 | **Phiên song song** | Which other sessions are live and which paths each holds. Omit the section entirely when there are none — never leave a stale one |
| 5 | **Cạm bẫy đã gặp** | Traps that cost real time, each with the symptom as it appeared and why the obvious reading was wrong. This is the section that saves the most hours |
| 6 | **Cổng kiểm** | How to verify, and **what the gate does not cover on this machine**. No numbers — they are wrong within days |
| 7 | **Việc treo** | **A pointer only.** A deferred item lives in its module's ledger with `trang_thai: "treo"` |

### §2, §3 and §7 are pointers — this is the whole division of labour

This file and `kb/90-ephemeral/tien-do.md` answer **different questions**, and the moment both
answer the same one, they start to disagree and the reader cannot tell which is current
(rule 9, invariant 2):

| Question | File |
|---|---|
| What is done, what is not, what does this module still owe | `kb/90-ephemeral/tien-do.md` — generated, per module, written by `/progress` |
| What was decided with the customer · which traps have already cost an hour · which sessions are live | **this file** |

So `/handover` **never writes a task list**. If, while writing, you find work that belongs in a
ledger, put it there with `/progress` — do not copy it here.

### 4. Keep `kb/INDEX.yaml` in step

Its `ban-giao-phien` entry carries `expires` and a `load_when`. If the expiry moved, update it
there too — a pointer with the wrong date is a pointer the reader stops trusting.

### 5. Commit and push

The next session may be on another machine, so a handover that is not pushed does not exist.
Stage **by explicit path** — never `git add -A`, which sweeps up whatever a parallel session
has in flight.

---

## REFUSALS

- **Do not invent continuity.** A step you did not verify is written as unverified, or left out.
- **Do not decide an open question by describing it as settled.** If §3 would record a decision
  nobody made, that is a STOP CONDITION: ask the user instead of writing it.
- **Do not carry a section forward because it was there last time.** Every section is rewritten
  from what is true now; a stale §4 pointing at a session that ended is worse than no §4.
- **Do not write personal data** — no real phone numbers, names or national IDs, not even in an
  example (rule 3, forbidden #5). Business codes only.
- **Do not write secrets, DSNs or credentialed connection strings.** Name the variable (rule 8,
  invariant 3).

## AFTERWARDS

Tell the user, in three lines at most: the file's path, the number of blockers in §3 and who
each one is waiting on, and whether it was pushed.
