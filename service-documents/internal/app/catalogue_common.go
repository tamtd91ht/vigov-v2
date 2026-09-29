package app

// The parts of the catalogue write use cases that belong to the whole service rather than to one
// catalogue. See store/catalogue_write.go for why the split exists at all.

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/core/tenant"
)

// wrapCatalogueErr wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No code, no label, no actor: an error travels into centralised logging across every commune at
// once. The commune is not personal data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is.
// A %v here would collapse "this row is a system row" and "the database is down" into one 500.
func wrapCatalogueErr(ctx context.Context, op string, err error) error {
	return fmt.Errorf("danh_muc_loai_van_ban: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}

func pick[T any](beforeSide bool, before, after T) T {
	if beforeSide {
		return before
	}
	return after
}
