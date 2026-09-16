// Package migrations carries the platform service's schema migrations INSIDE the binary.
//
// WHY EMBEDDED RATHER THAN READ FROM DISK: a binary that reads migrations from a path depends
// on the right version of those files being present next to it, and "the right version" is
// exactly what drifts — a container built from one commit, a volume mounted from another. An
// embedded file cannot disagree with the code that was compiled beside it.
//
// The runner is pkg/migrate. Read its package comment before adding a file here; in particular,
// a migration that has already been applied is NEVER edited — the runner stops the service when
// it is.
package migrations

import "embed"

// FS holds every *.sql in this directory, applied in filename order by pkg/migrate.Chay.
//
//go:embed *.sql
var FS embed.FS
