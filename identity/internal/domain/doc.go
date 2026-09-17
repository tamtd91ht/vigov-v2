// Package domain holds the business types and pure rules of the identity service.
//
// HARD CONSTRAINT: this package imports NOTHING but the standard library. No database driver,
// no HTTP, no logging framework. That is what makes the business rules testable without
// infrastructure, and it is the first thing to check when this package starts feeling awkward
// — the awkwardness is usually infrastructure trying to leak in.
package domain
