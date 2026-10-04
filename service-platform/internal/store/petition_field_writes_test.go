package store

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A tier-1 code is never removed and never renamed (ADR 0060 §4, stop condition #5): no statement in
// the store may do either. Migration 0011's trigger is the database's copy; this is the source's,
// red at `go test` without a database. Literals are split so they do not appear here.
func TestPetitionFieldStoreNeverRenamesOrRemoves(t *testing.T) {
	src, err := os.ReadFile("petition_field.go")
	if err != nil {
		t.Fatal(err)
	}
	upper := strings.ToUpper(string(src))
	if strings.Contains(upper, "DELETE"+" FROM") {
		t.Error("petition_field.go deletes a row — retire with active = false")
	}
	if regexp.MustCompile(`SET\s+(?:[A-Z_]+\s*=\s*[^,]+,\s*)*CODE\s*=`).MatchString(upper) {
		t.Error("petition_field.go assigns the code column — a rename orphans archival petitions")
	}
}
