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

**Read-only agents.** `isolation-reviewer` and `domain-expert` hold no `Write` or `Edit`
tool, so they can run alongside anything, including each other and a builder. This is the
cheapest parallelism available and it is usually the one worth taking.

**A read-only agent alongside asking the user.** Dispatch, then ask your question in the same
turn; the answer and the report arrive independently.

## NEVER SAFE

| Pair | Why |
|---|---|
| `test-designer` + any builder | It writes *test files anywhere* — its boundary crosses every other boundary by definition |
| Two builders on **one change** | ROUTING §3 says it outright: the output of each step is the input of the next |
| Any builder + the **verification gate** | `go build` / `go test` run while another agent is writing measures a tree that is halfway through a change. A green result means nothing |
| Two agents that both run a **repo-global command** | Below |

## SHARED STATE THAT PATHS DO NOT SHOW

Path disjointness is **necessary but not sufficient**. Two `go-service-builder` runs on
`services/identity/**` and `services/petitions/**` look independent and are not: both may add
a dependency, and both then write the same two files.

| Shared thing | Written by | Consequence |
|---|---|---|
| `go.mod` · `go.sum` | `go get`, `go mod tidy` | Two agents resolving dependencies at once corrupt each other's resolution |
| `gen/**` | `buf generate` | A regenerate mid-flight changes the code the other agent is compiling against |
| `kb/30-indexes/**` | `make kb` | Generated from a tree that is still moving |
| The build and test cache | `go build`, `go test` | Results describe a tree nobody will ever have again |

**These commands belong to the main session only.** Say so in the brief; do not assume an
agent will avoid them.

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
| Fixing `kb/` contradictions while fixing route examples in `services/` | `knowledge-keeper` parallel with main-session edits | `kb/` and `services/*/internal/**` are disjoint — and it still handed back two items that were in the main session's scope |
| `pkg/idem` then the login route, both `go-service-builder` | **Sequential** | Different files, but both need `pkg/config` and both would run `go mod tidy`. Question 3 fails |
| Review after a change touching data, permissions and files | `isolation-reviewer` parallel with `domain-expert` | Neither writes anything |

## CHECKLIST

```
[ ] boundaries disjoint per ROUTING §3
[ ] neither needs the other's output
[ ] no shared go.mod / gen/ / kb indexes / build cache
[ ] every brief self-contained: no question left for the agent to ask
[ ] repo-global commands reserved to the main session
[ ] verification runs alone, after all of them return
```

→ Write boundaries and conflicting pairs: `.claude/agents/ROUTING.md` §3
→ Mandatory follow-up after any change: `.claude/agents/ROUTING.md` §5
→ When not to use an agent at all: `.claude/agents/ROUTING.md` §6
