// Package store is the ONLY place in the identity service that knows SQL.
//
// SQL leaking upward into app/ is infrastructure leaking into business logic: it makes the
// use cases untestable and ties them to one database.
//
// Every repository here is built from *store.Scoped, which carries the commune. A repository
// that can be built without a commune is a repository that can query across communes.
//
// THERE IS EXACTLY ONE CONSTRUCTOR TAKING A RAW *sql.DB, AND IT IS NAMED HERE SO THAT A SECOND
// ONE STANDS OUT: NewPhienCongDanStore. The citizen session lookup is what ESTABLISHES the
// commune — the Mini App has no domain, so the edge derives the commune FROM the session
// (ADR 0022) — and core/store.For(ctx) panics when there is no commune yet. The exemption is
// argued in full on PhienCongDanStore.TraCuu, the same way service-platform's Directory argues
// its own. Anything else that reads without a commune belongs in store/crosstenant/ (ADR 0021),
// not here.
package store
