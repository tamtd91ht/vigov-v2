---
name: go-tenant-context
description: Use when writing any query, repository, middleware, or inter-service call in the Go backend. Triggers on: tenant, tenant_id, multi-tenant, multi-commune, tenant scope, scoped repo, context, Host, domain, isolation, repository, query, store.
---

# Skill: Tenant context in Go

`tenant_id` is a **data dimension**, not a parameter. It travels in `context.Context` from
the edge down to the store, and **no business function takes it as an argument** — taking it
as an argument means it can be passed wrong, and eventually it will be.

## REQUIRED

| # | Practice |
|---|---|
| 1 | The outermost edge resolves the commune from `Host` into the context. Cannot resolve → **404** |
| 2 | The repository reads `tenant_id` **from the context** and adds it to every query itself |
| 3 | No path to the raw store. A `*sql.DB` is never exposed to the business layer |
| 4 | Every request compares the token's `tenant_id` with the commune from `Host`. Mismatch = 401 + alert |
| 5 | gRPC calls carry `tenant_id` in metadata; the receiver lifts it into its context |
| 6 | Queue messages carry `tenant_id`; a consumer without it **refuses** |

## FORBIDDEN

`tenantID string` as a business function argument · `?? "default"` · tenant from the client ·
cookies scoped to the parent domain · single-column unique keys.

## Shape

```go
// edge — the single place the commune is resolved
func TenantMiddleware(dir Directory) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            t, ok := dir.ByHost(r.Host)
            if !ok {
                http.NotFound(w, r)   // 404: reveal nothing about which communes exist
                return
            }
            next.ServeHTTP(w, r.WithContext(tenant.Into(r.Context(), t.ID)))
        })
    }
}

// store — tenant_id is added for you, so business code never has to remember
func (r *Repo) scoped(ctx context.Context) *scopedRepo {
    return &scopedRepo{db: r.db, tid: tenant.MustFrom(ctx)}  // missing → panic, never guess
}

func (s *scopedRepo) Find(ctx context.Context, f Filter) ([]Petition, error) {
    return s.db.Query(ctx, `SELECT ... WHERE tenant_id = $1 AND ...`, s.tid, ...)
}
```

The key design choice: **`MustFrom` panics when the commune is missing.** Failing loudly in
development beats failing silently in production, where silence means serving every
commune's data.

## The only escape hatch

```go
// @cross-tenant: district-level petition rollup, aggregates only, audited
```

Review them regularly with `/review-isolation`: is each one still correct.

→ Rule 1 · `kb/00-foundation/multi-tenant-model.md`
