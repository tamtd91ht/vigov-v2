# RULE 11 — The infrastructure configuration contract

One system, many communes, one set of k8s manifests. Every service that reads the same
dependency under a different variable name turns the cluster's environment into a pile nobody
can audit — two names for one cluster, and no way to tell which deployments were updated.

This rule is the **machine-decidable half** of `skills/infra-config`. The convention, the
worked examples and the k8s snippets live there; what cannot drift silently lives here.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | `core/config` is the **only** package that reads the environment. Everything else takes a typed `config.Config` |
| 2 | One canonical name per role, **identical in every service**. A second spelling for an existing meaning is a second name |
| 3 | The name says the **ROLE**, never the cluster: `KAFKA_LOG_ADDRESS`, never `KAFKA_02_ADDRESS`. Binding a role to a physical cluster happens in the k8s manifest, never in source |
| 4 | Two spellings of one name: **ConfigMap/Secret key `WITH-DASHES`**, **env var and Go `WITH_UNDERSCORES`**. `-` → `_` and nothing else |
| 5 | Address values are **cluster-shaped from the first line written** — a comma-separated `host:port` list, or a DSN whose host part may carry several hosts — even when the deployment has one node |
| 6 | Every variable has a line in `.env.example`, with a **placeholder**. Never a real value, never a plausible-looking one (rule 8) |
| 7 | Anything that would hurt in a log line is `secret.Secret` and comes from a **Secret**; everything else comes from a **ConfigMap** |
| 8 | Required vs optional is decided **per variable, with the reason written beside it**. Required means the service cannot serve one request without it |

## STRICTLY FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | `os.Getenv` outside `core/config` | A variable in no struct, no template and no review. It is discovered from a stack trace |
| 2 | A **cluster ordinal** in a name the code reads (`KAFKA_02_ADDRESS`, `REDIS_1_DSN`) | Welds a workload to one cluster. Moving it becomes a code change and a release of every service that reads it |
| 3 | A dash inside a name read from Go | Dashes are the ConfigMap key spelling. A dash in an env var name is not a valid shell identifier and will never be set |
| 4 | Reading a variable that has no `.env.example` line | The template is the registry. A variable outside it is one nobody can discover |
| 5 | Splitting an address list or a DSN to keep **one host** | Correct-looking for as long as there is one node, silently wrong on the day of the second |
| 6 | A commune-specific value in the environment | Rule 1, invariant 10: the environment carries PLATFORM-wide constants only. Per-commune values are read at runtime |

**Why invariant 3 is the one that pays for itself:** a role name in source and a cluster key in
the manifest means moving the log workload from kafka-02 to kafka-05 is one line in one file.
The same change with `KAFKA_02_ADDRESS` in Go is: edit source, review, rebuild every image that
reads it, release them together, and hope nobody missed one. The mechanism costs nothing on the
day it is written and cannot be retrofitted cheaply.

**Why invariant 5 cannot wait:** "single node today, we will add the list later" means the code
that keeps the first host survives review, because with one node it is indistinguishable from
correct. The defect ships, and it surfaces on the day the platform moves to HA — as a cluster
that mysteriously only ever talks to one member.

## STOP CONDITIONS — ask the user, never decide alone

1. A dependency that is **not already in `core/config`** — a new broker, a new store, a new
   search cluster. Which k8s object supplies it, and whether it is required, are the owner's
   calls, and ADR 0010 fixes the data infrastructure
2. A variable that must be **required in every environment**, because it stops every service
   from starting on any machine that has not set it
3. A value that differs **per commune** — that is rule 1 invariant 10 and does not belong in
   the environment at all

→ Enforcement: `hooks/env_contract_guard.py` (BLOCK)
→ Skill: `skills/infra-config`
→ Related: rule 8 (secrets and templates) · rule 1, invariant 10 (per-commune values)
