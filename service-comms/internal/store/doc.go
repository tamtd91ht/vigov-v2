// Package store is the ONLY place in the comms service that knows SQL.
//
// SQL leaking upward into app/ is infrastructure leaking into business logic: it makes the
// use cases untestable and ties them to one database.
//
// Every repository here is built from *store.Scoped, which carries the commune. There is
// deliberately no constructor taking a raw *sql.DB — a repository that can be built without a
// commune is a repository that can query across communes.
package store
