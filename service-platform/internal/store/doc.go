// Package store is the ONLY place in the platform service that knows SQL.
//
// SQL leaking upward into app/ is infrastructure leaking into business logic: it makes the
// use cases untestable and ties them to one database.
//
// Every repository over a commune's own content is built from core/store, which carries the
// commune. Only two types take a raw *sql.DB, each over tables with NO commune column: Directory
// (the registry — it is what establishes the commune) and UploadPolicyStore (platform-wide upload
// limits). A repository over commune data that can be built without a commune is a repository
// that can query across communes.
package store
