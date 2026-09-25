// Package crosstenant holds the identity service's reads and writes that do NOT carry a
// commune — and nothing else.
//
// WHY A PACKAGE AND NOT A FILE. ADR 0021 places it here so that "reads across communes" is a
// COUNTABLE LIST rather than a habit spread through the repository. The danger is never a type
// without a tenant_id field; it is a QUERY without `WHERE tenant_id = $1`. core/store.For(ctx)
// is the only sanctioned path to the database and it panics when the context carries no
// commune (core/store/scoped.go), so anything unscoped must step outside it — and that step
// has to be visible, in a directory named after what it does, instead of sitting between two
// ordinary repositories.
//
// WHAT THIS PACKAGE DOES NOT GRANT: it does not make a cross-commune query legitimate. It only
// makes it countable. Rule 1, forbidden #6 still applies to every statement in here — each one
// carries its own `// @cross-tenant: <reason>` on the line above it.
//
// # WHAT MAY BE ADDED HERE
//
// Exactly three shapes, and every one of them needs a decision already written down in an ADR
// before the code is written:
//
//  1. A table that has NO tenant_id at all because a decision put it above the commune —
//     dinh_danh_cong_dan (ADR 0002: one phone number is one record for the whole platform)
//     → dinh_danh_cong_dan.go; and since 2026-09-25 tai_khoan_zalo (ADR 0045: the Zalo account
//     behind a Mini App session, which answers "which commune" for the main app, so it cannot
//     belong to one) → tai_khoan_zalo.go. Every statement there is keyed on ONE account.
//  2. A lookup that must find a row BEFORE the commune is known, where the commune is then
//     compared rather than assumed — ADR 0019, invariant 8: redeeming a pairing code has to
//     find the code first so that a code belonging to another commune produces 401 AND AN
//     ALERT, not a quiet "not found". → ghep_phien.go
//  3. A read of a table that DOES have tenant_id, keyed on ONE CITIZEN IDENTITY taken from the
//     session, returning that citizen's own rows across the communes they deal with — ADR 0002
//     makes a citizen many-to-many with communes and the Mini App has to show them the list.
//     → quan_he_cong_dan_xa.go
//
// SHAPE 3 WAS ADDED ON 2026-09-20, AND THE PARAGRAPH BELOW IS WHY IT IS NOT A LOOSENING. This
// file used to say "exactly two shapes" while migration 0004 already named this exact query as
// belonging here (see the comment above index quan_he_cong_dan_xa_theo_cong_dan, which is
// deliberately not prefixed with tenant_id). One of the two had to give; the migration is the
// one carrying the argument, so the list grew rather than the query going somewhere quieter.
//
// What separates shape 3 from the rollup forbidden below is not how many communes appear in
// the result — it is what the query is KEYED ON. Shape 3 is keyed on one citizen, from the
// session, returning that citizen's own rows: it is rule 4, invariant 1 read across the
// commune boundary ADR 0002 put between a citizen and the communes they deal with. A rollup is
// keyed on a commune, or on nothing, and returns rows belonging to OTHER PEOPLE. A shape-3
// store that ever stops being keyed on one citizen — a COUNT, a GROUP BY, a list of everybody
// — has become the thing this package forbids, and it will not look like a new query when it
// happens. It will look like one more method on a type that was already here.
//
// # WHAT MAY NOT BE ADDED HERE, EVER
//
//   - Any read of a commune's business data that returns rows of more than one commune —
//     district or province rollups. Open question #4 is still open: nobody has decided how far
//     up the administrative ladder aggregate reading goes. Writing it now decides it silently.
//   - Any query whose commune comes from something the client sent (a header, a query
//     parameter, a body field). That is rule 1, forbidden #2, and this package is exactly where
//     it would look reasonable.
//   - A convenience wrapper that hands a raw *sql.DB to code outside this package. The handle
//     is the whole risk; it does not leave.
//   - The citizen session lookup by bearer token. It is unscoped too, and it carries its own
//     `// @cross-tenant:` mark like everything in here — but it is not a read ACROSS communes:
//     it returns at most ONE session, and the commune of the request is DERIVED from it
//     (ADR 0022), the way service-platform's Directory derives a commune from a Host. It lives
//     with the rest of the session code, in the parent package, documenting its own exemption
//     there. The migration that created the index for it says the same thing in the same words.
//
// # AUDIT ENTRIES ARE NOT WRITTEN HERE
//
// Every write in this package takes the caller's *sql.Tx and runs inside it, so the use case
// can write the audit entry in the SAME transaction (rule 6, invariant 3). This layer does not
// write the entry itself because it does not know the business fact: creating a
// dinh_danh_cong_dan row is "a citizen signed in for the first time", and only the use case
// knows who, from which IP, and in which commune — which is what the trail has to answer.
package crosstenant
