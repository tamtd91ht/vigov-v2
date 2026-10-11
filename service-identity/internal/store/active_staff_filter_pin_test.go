package store

import (
	"testing"

	"github.com/vihat/vigov/service-identity/internal/store/crosstenant"
)

// OperatorService.ListActiveStaffCounts counts "active staff accounts" with the picker's predicate
// (operator.proto: "the same three conditions store.locChonNguoi uses"). crosstenant cannot import
// this package (this package's tests import crosstenant), so it restates the predicate; this test is
// what keeps the restatement honest. Edit locChonNguoi and the operator's denominator must move with
// it — or this turns red.
func TestActiveStaffAccountFilterIsThePickerPredicate(t *testing.T) {
	if crosstenant.ActiveStaffAccountFilter != locChonNguoi {
		t.Fatalf("crosstenant.ActiveStaffAccountFilter = %q, locChonNguoi = %q — one meaning, two spellings",
			crosstenant.ActiveStaffAccountFilter, locChonNguoi)
	}
}
