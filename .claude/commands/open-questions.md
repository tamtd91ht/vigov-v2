---
description: Check whether the code is silently deciding one of the customer's open questions
group: Yêu cầu & quyết định
argument-hint: "[question number, empty = all]"
allowed-tools: Read, Bash, Glob, Grep
---

# /open-questions

## The problem this solves

The agent **breaks no rule** when each line it writes is correct. It simply does not notice
that a chain of local decisions, each reasonable on its own, is adding up to an
**architectural decision it has no authority to make**.

This really happened: on an earlier ViGov project, "one commune or many" was an open question
the customer had not decided, and the estimate said "+1–2 days". The brain even carried the
rule *"never decide the customer's open questions"* — but that rule was one sentence of prose.
After 103 commits the code had decided, single-commune, in 12 places. Cost to reverse once
found: 20–28 days.

**Every line was innocent. The sum had already decided.**

## Steps

1. Read `kb/00-foundation/open-questions.json`
2. For each question with status `OPEN`: scan its `code_signals` across the service directories, `web-admin/`, `proto/`
3. Count signals per **direction**; report when one direction dominates
4. State the **reversal cost as of today** — that number grows over time, and watching it
   grow is what creates pressure to decide
5. For a question being silently decided: **raise it with the user** and propose asking the
   customer. **Never decide it yourself.**

## Report

| # | Question | Status | Signals | Reversal cost |
|---|---|---|---|---|

Conclude with: which question **must be asked before the work currently planned**.
