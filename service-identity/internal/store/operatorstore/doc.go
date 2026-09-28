// Package operatorstore is the identity service's store for the Vihat operator realm — and the
// only code in the repository that reads or writes its five tables (migration 0012).
//
// WHY IT IS UNSCOPED. core/store.For(ctx) is the sanctioned path to the database and it panics
// when the context carries no commune (core/store/scoped.go). The operator realm has NO commune BY
// DESIGN — ADR 0048 §Chốt của chủ dự án #1: operator accounts live in service-identity, in separate
// tables, with no tenant_id, because an operator manages platform metadata across every commune.
// Inventing a "platform" commune to satisfy core/store would be a default on the isolation path
// (rule 1, forbidden #1). So this package takes a raw *sql.DB, and every statement carries its own
// `// @cross-tenant:` mark on the lines above it (rule 1, forbidden #6).
//
// WHY A SIBLING OF crosstenant AND NOT A FILE INSIDE IT. crosstenant/doc.go lists exactly three
// admissible shapes, and all three are about CITIZENS and COMMUNES — a citizen identity above the
// commune, a lookup before the commune is known, a citizen's own rows across communes. Operator
// accounts are none of those: they are not about any commune at all. Adding them there would
// either stretch a list whose whole value is being short, or file platform staff under a package
// whose readers are looking for citizen paths. A separate directory keeps both lists countable.
//
// WHAT THIS PACKAGE DOES NOT GRANT. It never reads a commune's business data. Nothing here joins a
// table that carries tenant_id, and nothing here may: an operator act that targets one commune is
// written by the service that owns that commune's data, under that commune (ADR 0048 §Thiết kế #6).
//
// THE HANDLE DOES NOT LEAVE. Store holds the *sql.DB and hands out only *Tx, whose methods are the
// statements below. There is no accessor for the raw handle or the raw *sql.Tx.
//
// AUDIT IN THE SAME TRANSACTION (rule 6, invariant 3). Every write is a method on *Tx, and
// Tx.AppendAudit writes operator_audit_log inside that same transaction. The use case (TASK-03)
// decides WHICH event to write, because only it knows the business fact; this package guarantees
// the two cannot commit apart.
//
// INVARIANTS THIS PACKAGE ENFORCES ITSELF rather than trusting every caller to remember them:
//
//   - The sid is hashed (SHA-256 hex) HERE, before any statement: callers pass the raw sid and it
//     never reaches the database. Recovery codes arrive ALREADY hashed, as the 32-byte digest of
//     operatorauth.HashRecoveryCode, because they must be normalised before hashing and that rule
//     has one owner; anything but 32 bytes is refused.
//   - Password change, administrative disable, MFA reset and any grant change REVOKE EVERY LIVE
//     SESSION of the account in the same transaction (owner's decision 2026-09-28).
//   - There is no DELETE statement anywhere in this package (rule 7).
//
// PERSONAL DATA (rule 3): email and display name are read and written here and NEVER logged, never
// placed in an error, never placed in an audit entry. Errors name the operation, not the row.
package operatorstore
