package app

// The parts of the catalogue write use cases that belong to the whole service rather than to one
// catalogue. See store/danh_muc_ghi.go for why the split exists at all.

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/core/tenant"
)

// boc wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No code, no label, no actor: an error travels into centralised logging across every commune at
// once. The commune is not personal data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is.
// A %v here would collapse "this row is a system row" and "the database is down" into one 500.
func boc(ctx context.Context, viec string, err error) error {
	return fmt.Errorf("danh_muc_hang_muc_ke_hoach_von: %s cho xã %s: %w", viec, tenant.MustFrom(ctx), err)
}

func chon[T any](ben bool, truoc, sau T) T {
	if ben {
		return truoc
	}
	return sau
}
