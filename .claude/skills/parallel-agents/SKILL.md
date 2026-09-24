---
name: parallel-agents
description: Use when deciding whether several agents may be dispatched at once instead of one after another. Triggers on: parallel, song song, concurrent, đồng thời, fan out, batch dispatch, multiple agents, nhiều agent, speed up, tăng tốc, independent tasks, độc lập, at the same time, cùng lúc.
---

# Skill: Running agents in parallel

Only the main session dispatches — a subagent cannot spawn a subagent, so every parallel
group is one message from the main session carrying several Agent calls.

Parallelism is worth it when the work is genuinely independent. It is **not** a way to go
faster on work that is not.

## THE TEST

> Two agents may run at once only if **nothing one writes is something the other reads or
> writes** — files *and* shared repository state.

Write boundaries live in `.claude/agents/ROUTING.md` §3, with the pairs that conflict. That
file owns them; look them up there rather than remembering them.

## DECIDE IN FOUR QUESTIONS

| # | Question | If the answer is wrong |
|---|---|---|
| 1 | Are the **write boundaries disjoint**? (ROUTING §3) | Two agents edit one file; the second write silently erases the first |
| 2 | Does either need the **other's output** to start? | The second builds on a state that never existed. Cross-layer work is ordered for this reason — ROUTING §3 |
| 3 | Does either touch **shared repository state** (below)? | Paths can be disjoint while the repo is not |
| 4 | Is each brief **complete enough to finish without asking**? | A parallel agent has no way to ask mid-flight — see below |

All four clear → dispatch together, in one message. Any one unclear → sequential.

## ALWAYS SAFE

**Read-only agents.** `context-scout`, `cross-context-scout`, `isolation-reviewer`,
`domain-expert` and `progress-reviewer` hold no `Write` or `Edit` tool, so they can run
alongside anything, including each other and a builder. This is the cheapest parallelism
available and it is usually the one worth taking — which is why the discovery step of the
workflow (ROUTING §0.2) is always the two scouts in one message.

**Implementation fan-out (ROUTING §0.5)** is "2–3 builders" only where every question below
passes, and the Go cap in the next-but-one section always wins over that number.

**Several builders writing progress at once.** Each writes only
`kb/90-ephemeral/tien-do/<its own module>.json`, so the ledger adds no shared state — the
partition is in the path. The one thing that stays serial is `make kb`, which renders the read
surface: it belongs to the main session, like every other repo-global command below.

**A read-only agent alongside asking the user.** Dispatch, then ask your question in the same
turn; the answer and the report arrive independently.

## NEVER SAFE

| Pair | Why |
|---|---|
| `test-designer` + any builder | Not a path collision — a `_test.go` file is never a `.proto`, a migration or a `kb/` document. It runs `go test ./...` across the whole module, so a package another agent is halfway through writing shows up as a compile error in code it does not own, and it chases a defect that is not there |
| Two builders on **one change** | ROUTING §3 says it outright: the output of each step is the input of the next |
| Any builder + the **verification gate** | `go build` / `go test` run while another agent is writing measures a tree that is halfway through a change. A green result means nothing |
| Two agents that both run a **repo-global command** | Below |

## SHARED STATE THAT PATHS DO NOT SHOW

Path disjointness is **necessary but not sufficient**. Two `go-service-builder` runs on
`identity/**` and `petitions/**` look independent and are not: both may add
a dependency, and both then write the same two files.

| Shared thing | Written by | Consequence |
|---|---|---|
| `go.mod` · `go.sum` | `go get`, `go mod tidy` | Two agents resolving dependencies at once corrupt each other's resolution |
| `gen/**` | `buf generate` | A regenerate mid-flight changes the code the other agent is compiling against |
| `kb/30-indexes/**` | `make kb` | Generated from a tree that is still moving |
| The build and test cache | `go build`, `go test` | Results describe a tree nobody will ever have again |

**These commands belong to the main session only.** Say so in the brief; do not assume an
agent will avoid them.

## THE MACHINE IS SHARED STATE TOO — AND IT IS THE ONE THAT DESTROYS WORK

Everything above is about agents corrupting each other's *files*. This one is different: nothing
is corrupted, the agent simply **dies mid-sentence and its work is gone**.

**HARD CAP: at most TWO concurrent agents that compile Go. One is the safe number.**

Measured on this project, 22/09/2026, not reasoned from first principles:

| What was run | What happened |
|---|---|
| 3 × `go-service-builder`, disjoint modules, textbook-correct boundaries | All three killed. Two left **nothing at all**; the third left 818 lines that happened to compile |
| Same day, earlier | Two whole sessions killed with exit 137, terminal closed with them |

The boundaries were right. Question 3 of the four questions passed. **It still cost most of a
night**, because a Go toolchain compiling a module is expensive and three of them plus the main
session is more than this machine has.

Why it is worse than any conflict in the table above:

- **It gives no signal.** No error, no partial file, no hand-back. The task notification says the
  agent stopped; the transcript is saved but the work is not.
- **A dying agent has not committed.** The main session cannot recover what was in its head.
- **It looks like progress right up to the end.** Three agents "running" reads as three times the
  throughput until the moment all three are gone.

**Practical rules:**

```
[ ] Go-compiling agents: 1 concurrent, 2 at the absolute most
[ ] Read-only agents (the two scouts, isolation-reviewer, domain-expert, progress-reviewer): cheap, parallel is fine
[ ] Web agents (tsc/vitest): 2 is fine — the Node toolchain costs far less than Go's
[ ] NEVER run a Go build in the main session while a Go agent is working
[ ] Long agent + valuable partial work → tell it in the brief: "if a command is killed, COMMIT what
    exists first, then re-run"
```

**And when an agent does die: check the working tree before assuming the task is lost.** Partial
work that compiles is worth a `wip(...)` commit immediately — the alternative is losing it a
second time to the same cause.

## VERIFICATION IS ALWAYS SERIAL

Fan out to write, then come back to one thread to verify:

```
dispatch A, B, C in parallel   (disjoint boundaries)
        |
   all three return
        |
main session, alone:  go mod tidy -> make check -> make kb
```

Running the gate while an agent is still writing is the failure `stop_verify_guard` exists to
catch, arrived at a different way: a green gate that proves nothing.

## WHAT PARALLELISM COSTS

Worth knowing before reaching for it, because none of this shows up as an error.

| Cost | What it looks like |
|---|---|
| **Hand-backs** | An agent respecting its boundary finds a problem in *your* files and returns it instead of fixing it. The work moves to the main session; it does not disappear |
| **No mid-flight questions** | Agents here have no `SendMessage`. One that hits an unanswered question either stops or assumes — and under CLAUDE.md it must not assume. A thin brief spends the whole wall-clock saving and returns nothing |
| **Findings that need each other** | Two reviewers each see half of a cross-cutting problem and neither reports it whole |
| **Context** | Every report lands in the main session at once. Three large reports can cost more than the time saved |

A single agent on a well-scoped task beats three agents on a vague one.

## WORKED EXAMPLES

| Situation | Call | Why |
|---|---|---|
| Naming resources, while the user is still choosing a design | `domain-expert` parallel with the question | Read-only; its answer is needed either way |
| Fixing `kb/` contradictions while fixing route examples in the service directories | `knowledge-keeper` parallel with main-session edits | `kb/` and `*/internal/**` are disjoint — and it still handed back two items that were in the main session's scope |
| `core/idem` then the login route, both `go-service-builder` | **Sequential** | Different files, but both need `core/config` and both would run `go mod tidy`. Question 3 fails |
| Review after a change touching data, permissions and files | `isolation-reviewer` parallel with `domain-expert` | Neither writes anything |

## CHECKLIST

```
[ ] boundaries disjoint per ROUTING §3
[ ] neither needs the other's output
[ ] no shared go.mod / gen/ / kb indexes / build cache
[ ] AT MOST 2 Go-compiling agents — 1 is the safe number. This one is not theory
[ ] every brief self-contained: no question left for the agent to ask
[ ] repo-global commands reserved to the main session
[ ] verification runs alone, after all of them return
```

→ Write boundaries and conflicting pairs: `.claude/agents/ROUTING.md` §3
→ Mandatory follow-up after any change: `.claude/agents/ROUTING.md` §5
→ When not to use an agent at all: `.claude/agents/ROUTING.md` §6
