---
name: go-service-pattern
description: Use when adding or changing a Go service — directory layout, layers, dependencies, wiring, configuration. Triggers on: new service, Go, directory structure, internal, cmd, layer, dependency, wiring, main.go, handler, repository, package layout.
---

# Skill: Go service layout in ViGov

## Directory shape

```
services/<name>/
├── cmd/server/main.go        wiring only, no business logic
├── internal/
│   ├── domain/               business types and pure rules — imports NO infrastructure
│   ├── app/                  use cases; takes interfaces, never concrete types
│   ├── store/                repository implementation; the ONLY place that knows SQL
│   ├── http/                 routing + permission declarations
│   └── event/                publishing and consuming events
├── README.md                 one screen: what this service does, what data it owns
└── migrations/
```

## REQUIRED

| # | Practice |
|---|---|
| 1 | `domain/` imports nothing beyond the standard library — business rules testable without a database |
| 2 | `app/` takes **interfaces**, declared at the point of use |
| 3 | Only `store/` knows SQL. SQL leaking into `app/` is infrastructure leaking into business logic |
| 4 | `main.go` only wires. No `init()`, no global mutable state |
| 5 | Configuration is read once at startup; **per-commune** configuration is read at runtime (rule 1) |
| 6 | Errors are wrapped with `fmt.Errorf("...: %w", err)` — keep the cause chain |
| 7 | Every function touching I/O takes `ctx` as its first parameter |

## FORBIDDEN

Importing another service's `internal/` · opening another service's database · a `utils`
or `common` package that holds everything · returning `interface{}` from the business layer ·
swallowing errors with `_`.

## Before adding a new service — stop condition

Does this flow need **strong consistency** with an existing service?
**Yes → do not split.** Splitting means accepting eventual consistency and writing compensation.

→ Rule 2 · `kb/00-foundation/domain-boundaries.md`
