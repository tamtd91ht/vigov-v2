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
// Exactly two shapes, and both need a decision already written down in an ADR before the code
// is written:
//
//  1. A table that has NO tenant_id at all because a decision put it above the commune —
//     today that is dinh_danh_cong_dan alone (ADR 0002: one phone number is one record for the
//     whole platform).
//  2. A lookup that must find a row BEFORE the commune is known, where the commune is then
//     compared rather than assumed — ADR 0019, invariant 8: redeeming a pairing code has to
//     find the code first so that a code belonging to another commune produces 401 AND AN
//     ALERT, not a quiet "not found". That store is NOT here yet; it is the next slice.
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
