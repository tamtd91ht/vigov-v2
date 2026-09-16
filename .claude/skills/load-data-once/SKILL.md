---
name: load-data-once
description: Use when writing any multi-step business flow, any handler that touches more than one entity, any loop over records, or any code that calls a repository or gRPC service. Triggers on: N+1, query in a loop, batch, preload, join, repository, gRPC call, use case, flow, handler, list, enrich, lookup, performance, slow query, round trip, fetch, load, cache.
---

# Skill: Load data once, carry it through the flow

A business flow passes through many steps. **Data already in hand at step 2 must not be
fetched again at step 5.** Each re-fetch is a network round trip that returns something the
process already has.

This is the single most common performance defect in a layered service, and it never shows
up in tests: every test passes, every feature works, the system is simply slow — and it gets
slower with each commune onboarded.

## The two shapes to recognise

### Shape 1 — N+1: a query inside a loop

```go
// WRONG — 1 + N round trips
phieus, _ := repo.ListPhanAnh(ctx)
for _, p := range phieus {
    canBo, _ := repo.GetNguoiDung(ctx, p.CanBoXuLyID)  // one query PER row
    out = append(out, present(p, canBo))
}
```

```go
// RIGHT — 2 round trips, whatever N is
phieus, _ := repo.ListPhanAnh(ctx)
ids := collectIDs(phieus, func(p PhanAnh) string { return p.CanBoXuLyID })
canBos, _ := repo.GetNguoiDungByIDs(ctx, ids)          // ONE batched query
for _, p := range phieus {
    out = append(out, present(p, canBos[p.CanBoXuLyID]))
}
```

At 21 petitions this is invisible. At a commune with 3.000 petitions in the list view it is
3.001 queries, and the page stops loading.

### Shape 2 — the same row fetched at several steps of one request

```go
// WRONG — the same staff row read three times in one request
func (uc *UseCase) DongPhieu(ctx context.Context, id string) error {
    if err := uc.checkQuyen(ctx); err != nil {          // reads nguoi_dung
        return err
    }
    p, _ := uc.repo.GetPhanAnh(ctx, id)
    uc.repo.CapNhatTrangThai(ctx, p, uc.currentUser(ctx))  // reads nguoi_dung again
    return uc.audit.Ghi(ctx, uc.currentUser(ctx))          // and again
}
```

Fetch once at the edge of the use case, pass it down. A function that needs the actor should
**take it as a parameter**, not go and find it.

## REQUIRED

| # | Practice |
|---|---|
| 1 | **No repository call inside a loop.** Collect the ids, make one batched call |
| 2 | Every repository that returns a list has a `...ByIDs(ctx, ids []string)` sibling |
| 3 | Data resolved once per request travels **down as a parameter**, never re-fetched |
| 4 | A gRPC call to another service is **batched by default** — `GetStaff` takes ids, plural |
| 5 | Joins that belong to ONE service's schema are done **in SQL**, not in Go |
| 6 | Before adding a lookup, ask: *does the caller already hold this?* If yes, pass it in |

## FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | A query or gRPC call inside a `for` loop | Cost grows with data; invisible in tests, fatal in production |
| 2 | A helper that re-reads the current actor instead of receiving it | Same row, several reads, one request |
| 3 | Joining two tables of the SAME service in Go instead of in SQL | The database does this better; Go does it over the network |
| 4 | Caching the result of a request-scoped fetch in a package variable | Leaks across communes — rule 1. Request scope only |

## Where this collides with other rules — read this before "optimising"

**Never batch across a service boundary by reaching into another schema.** Rule 2 stands:
`petitions` does not join `identity`'s tables to avoid a gRPC call. The right fix is a
*batched* gRPC call, not a shortcut around the contract.

**Never widen a query's tenant scope to save a round trip.** Rule 1 stands: a single query
returning several communes' rows is a data breach, not an optimisation. Batching happens
**inside one commune**.

**Per-request memoisation is not a cache.** Keep it in the request's own scope. A
process-level cache keyed without `tenant_id` serves one commune's data to another, silently,
with every test still green (rule 1, forbidden #1).

## Where the data usually already is

| Step | Already holds | Do not re-fetch |
|---|---|---|
| Edge middleware | `tenant.ID`, `authz.Principal` | The commune, the signed-in actor |
| Use case entry | The aggregate it just loaded | Its own row, its own status |
| After a permission check | The actor's roles | The staff row |
| Before `audit.Write` | Actor and subject | Both — pass them in |
| Inside a transaction | Every row read so far | Anything read earlier in the same `Tx` |

## The judgement call

This is an optimisation to **weigh**, not a rule to apply blindly:

- **Batch it** when N grows with data — list views, reports, exports, event consumers.
- **Leave it** when N is one, or bounded and tiny, and batching would obscure the flow.
- **Measure before restructuring** anything that is already clear and already fast enough.

Correct and slow beats fast and wrong: never drop a tenant filter, a permission check, or an
audit write to save a round trip.

→ Rule 1 (commune isolation): `.claude/rules/critical/1-tenant-isolation.md`
→ Rule 2 (service boundary): `.claude/rules/critical/2-service-boundary.md`
→ Skill: `go-service-pattern` · `go-tenant-context` · `transaction-boundary`
