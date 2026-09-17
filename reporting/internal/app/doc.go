// Package app holds the use cases of the reporting service.
//
// Use cases take INTERFACES, declared here at the point of use, never concrete types. A use
// case that names a concrete repository cannot be tested without a database and cannot be
// reused when the storage changes.
//
// Every use case that writes must open a transaction and put the audit entry inside it —
// see pkg/audit for why that is not optional.
package app
