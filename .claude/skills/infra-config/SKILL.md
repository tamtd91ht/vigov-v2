---
name: infra-config
description: Use when adding or changing any infrastructure dependency — PostgreSQL, Redis, RabbitMQ, Kafka, Elasticsearch — or any environment variable that names a host, a port, a credential or a cluster. Triggers on: env, environment variable, os.Getenv, config, cấu hình, ConfigMap, Secret, k8s, kubernetes, DSN, host, port, cluster, cụm, kafka, rabbit, rabbitmq, elastic, elasticsearch, redis, postgres, postgresql, broker, address, addrs, endpoint.
---

# Infrastructure configuration — one name, one shape, one place

One system serves many communes from one set of manifests. Every service that reads the same
dependency under a different variable name turns the cluster's environment into a pile nobody
can audit: two names for one thing, and no way to tell whether they point at the same cluster.

This skill is the convention. The machine-decidable half is **rule 11**, enforced by
`hooks/env_contract_guard.py`.

---

## THE FOUR RULES, IN ONE TABLE

| # | Rule | The failure it prevents |
|---|---|---|
| 1 | **One canonical name per role**, identical in every service | Service A reads `PG_HOST`, service B reads `POSTGRES_ADDR`. Two names, one cluster, and nobody can tell which deployments were updated |
| 2 | **The name says the ROLE, never the cluster** | `KAFKA_02_ADDRESS` in source code welds a workload to one cluster. Moving it becomes a code change, a review, a build and a release |
| 3 | **Values are always cluster-shaped**, even for one node | A single-host parse looks correct for a year, then the platform moves to HA and the second and third hosts are silently dropped |
| 4 | **Read in exactly one place**: `core/config` | An `os.Getenv` in a handler is a variable that appears in no template, no manifest and no review |

---

## 1. ONE SPELLING EVERYWHERE — `UPPER_SNAKE_CASE` (owner, 11/10/2026)

**A ConfigMap/Secret key is spelled EXACTLY like the environment variable the code reads.** One
token from the cluster to the binary: ConfigMap key = Secret key = env var = `.env.example` line =
the name in `core/config` = the row in the operations job's key catalogue.

| # | Rule | Example |
|---|---|---|
| 1 | Only `A–Z`, `0–9`, `_`; starts with a letter. **No dash, no lower case** | `OBJECT_STORAGE_BUCKET_PREFIX` |
| 2 | The key IS the env var name — no renaming, no `-`→`_` mapping anywhere on the way | key `BASEMAP_URL` → env `BASEMAP_URL` |
| 3 | A pod loads its objects WHOLE with `envFrom` (`common-config` + `<service>-secrets`). No per-key `configMapKeyRef`/`secretKeyRef` for renaming, no `kubectl set env --from … --keys=…` | — |
| 4 | Group prefix by dependency, suffix by value shape: `DB_` `REDIS_` `OBJECT_STORAGE_` `KAFKA_` `RABBITMQ_` `ELASTIC_` … / `_URL` `_ADDRESS` `_ADDRS` `_DSN` `_KEY` `_SECRET` | `REDIS_ADDRESS`, `OBJECT_STORAGE_SECRET_KEY` |
| 5 | A name lives in ONE object — Secret if it would hurt in a log, ConfigMap otherwise (§5) | — |
| 6 | Every key has a `.env.example` placeholder line and a row in `deploy/van-hanh/ops.py`'s catalogue | — |

**Why not dashes** (the convention before 11/10/2026, which the cluster never actually followed):

- **`envFrom` silently drops any key that is not a valid env var name.** A dashed key that nobody
  remembered to map one by one is a variable the pod simply does not have — no error at apply, no
  error at start unless the service happens to require it. That is how `BASEMAP-URL` needed a
  dedicated `kubectl set env --keys` step in a Jenkinsfile and a hand-written `configMapKeyRef`.
- Two spellings of one name is the drift itself: the operator types `BASEMAP_URL`, the job looks for
  `BASEMAP-URL`, and the map has no basemap. One spelling cannot be mistyped into the other.
- Nearly every key on the cluster was already `UPPER_SNAKE_CASE`, loaded with `envFrom`. The rule
  follows the system that runs, not the other way round.
- Adding a variable is adding a key. No Deployment changes, no wiring to forget.

---

## 2. THE NAME IS THE ROLE, THE MANIFEST PICKS THE CLUSTER

This is the part that buys flexibility, and it is the part most often got wrong.

**In source code**, a variable names what the dependency is FOR — the role it plays in this
system:

```
KAFKA_LOG_ADDRESS        the Kafka this system writes its logs to
KAFKA_EVENT_ADDRESS      the Kafka that carries business events
ELASTICSEARCH_ADDRS      the search cluster
```

**In the cluster**, the operator decides WHICH physical cluster each role lands on — by the VALUE
of the role's key, never by renaming it:

```yaml
# ConfigMap common-config — loaded whole by envFrom
KAFKA_LOG_ADDRESS:   kafka-02-a:9092,kafka-02-b:9092   # the CLUSTER is the value
KAFKA_EVENT_ADDRESS: kafka-01-a:9092,kafka-01-b:9092
```

Moving the log workload from kafka-02 to kafka-05 is then one value in one ConfigMap and a restart.
No code change, no rebuild, no release, no Deployment edit, and no service left pointing at the old
cluster because somebody missed a file.

**A cluster ordinal must never appear in a name the code reads.** `KAFKA_02_ADDRESS` inside
Go is the whole mechanism thrown away: the binding has moved from the manifest into the
binary, where changing it costs a deploy of every service that reads it.

---

## 3. ALWAYS CLUSTER-SHAPED, EVEN WHEN THERE IS ONE NODE

Single node today, HA tomorrow, a real cluster the year after — and **none of those may be a
code change**. So every address-bearing value is cluster-shaped from the first line written:

| Dependency | Shape | One node looks like |
|---|---|---|
| Kafka, Elasticsearch, any broker | comma-separated `host:port` list | `kafka-0:9092` |
| PostgreSQL, Redis | a DSN whose host part may carry several hosts | `postgres://u:p@pg-0:5432/db` |

**The code must never keep the first host and drop the rest.** That defect is invisible while
there is one node — which is exactly how long it takes for everybody to forget it is there.
Hand the whole value to a driver that understands multiple hosts (`pgx` does); never split it
yourself to "get the host".

---

## 4. ONE PLACE READS THE ENVIRONMENT

`core/config` is the only package that calls `os.Getenv`. Everything else takes a typed
`config.Config`.

Three things follow, and all three are lost the moment a second package reads the environment
directly:

- a variable that exists is **visible** — it is in one struct, in one template, in one review
- a variable that is missing is **refused at startup**, by name, where somebody is watching —
  not three hours later on the one code path that happens to read it
- a credential has a **type that cannot be printed** (`secret.Secret`), instead of a `string`
  one `%+v` away from a log line that cannot be recalled (rule 8)

---

## 5. WHICH k8s OBJECT SUPPLIES WHAT

| Object | Holds | Go type |
|---|---|---|
| **ConfigMap** | host, port, address list, queue name, index prefix, database number, TTL | `string`, `[]string`, `time.Duration` |
| **Secret** | password, token, API key, signing key, **any DSN with a password in it** | `secret.Secret` / `secret.DSN` |

The test is not "is it sensitive", it is **"would it hurt in a log line"**. A DSN with a
password is a Secret even though it looks like an address.

---

## 6. REQUIRED OR OPTIONAL — DECIDE PER VARIABLE, AND SAY WHY

Marking every new variable required is the mistake that stops all eight services from starting
on any machine that has not set four more variables, in order to protect code that does not
exist yet.

| Call it | When |
|---|---|
| **Required at `config.Load`** | The service cannot serve a single request without it — `DATABASE_DSN`, `GRPC_CALLER_KEY` |
| **Optional, refused by name at use** | Nothing calls it yet, or only some services need it — `IDENTITY_GRPC_ADDR`, and today RabbitMQ and Elasticsearch |

An optional variable still gets its `.env.example` line. A variable with no template line is a
variable the next person discovers from a stack trace.

---

## 7. ADDING ONE — THE ORDER THAT AVOIDS REWORK

1. **Name the ROLE**, not the cluster. Check no name already means the same thing.
2. Add the field to `config.Config` with the right type — `secret.Secret` if it would hurt in
   a log.
3. Read it in `config.Load`, `strings.TrimSpace` it. A trailing newline pasted out of a
   Secret becomes a connection error that names the wrong cause.
4. Decide required vs optional, and write the reason beside it.
5. Add the placeholder line to `.env.example`. **Never a real value** — a template that looks
   like a working config is a template somebody ships (rule 8).
6. Add its row to the operations job's catalogue (`python deploy/van-hanh/ops.py catalog --write`).
7. Tell whoever owns the cluster which object supplies it — the key is the SAME `UPPER_SNAKE_CASE`
   name, added to `common-config` or `<service>-secrets`; `envFrom` picks it up on the next restart.

**Never** write a k8s manifest as part of this: the cluster belongs to the project owner.
Name the variable and say which object should hold it.

---

## 8. ENCRYPTED AND VERIFIED BY DEFAULT (rule 13)

Every dependency above is reached over TLS **with verification**: `sslmode=verify-full`,
`rediss://`, Kafka `SASL_SSL`, an `https://` MinIO endpoint, gRPC with `credentials.NewTLS`.
The CA bundle is a ConfigMap, a client key is a Secret (§5). `security_guard` blocks the
plaintext spellings; the two gRPC clients still dialling in plaintext are dated debt in
`tools/security_debt.json`. Details: `skills/security-baseline` §3.

→ Rule: `.claude/rules/critical/11-infra-config-contract.md` · rule 13
→ Enforcement: `hooks/env_contract_guard.py` (BLOCK)
→ Related: rule 8 (secrets), rule 1 invariant 10 (commune values are read at runtime, never
  from the environment — the environment carries PLATFORM-wide constants only)
