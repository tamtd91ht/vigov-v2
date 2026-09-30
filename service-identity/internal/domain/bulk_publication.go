package domain

// The shape of ONE bulk publication request — POST /api/v1/staff/publications (user decision
// 2026-09-30, option A: the administrator selects many people, and EACH row carries its own
// "đã hỏi ý" confirmation inside the request).
//
// SHAPE ONLY, like the rest of this package: whether a person exists, is locked, or was confirmed
// is decided per item by the use case. What is refused here is a request that is malformed AS A
// WHOLE — the client has a bug, and answering it per item would hide the bug behind a list of
// "skipped" rows.

import (
	"errors"
	"fmt"
)

// MaxBulkPublication caps one request.
//
// 200, THE SAME CEILING AS THE STAFF EXCEL IMPORT (MaxStaffImportRows): a commune's register is a
// few tens of people, so 200 covers the whole register of the largest commune in one request with
// room to spare. Past it the request is not an administrator ticking rows on a screen — it is a
// payload, and every item holds a row lock (FOR UPDATE) until the single transaction ends.
const MaxBulkPublication = 200

var (
	ErrBulkPublicationEmpty       = errors.New("can_bo: danh sách công khai hàng loạt rỗng")
	ErrBulkPublicationTooLarge    = errors.New("can_bo: danh sách công khai hàng loạt quá dài")
	ErrBulkPublicationMissingID   = errors.New("can_bo: một dòng trong danh sách công khai hàng loạt thiếu id")
	ErrBulkPublicationDuplicateID = errors.New("can_bo: một người xuất hiện hai lần trong danh sách công khai hàng loạt")
)

// CheckBulkPublication checks the ids of one bulk request, in the order they arrived.
//
// A DUPLICATE IS REFUSED, NOT COLLAPSED: two rows for one person may carry two different consent
// confirmations and two different positions, and picking one of them would be deciding for the
// administrator which of their two answers counts.
//
// No message quotes an id: it is not personal data, but the sentence names the rule, which is what
// the person fixing the request needs.
func CheckBulkPublication(ids []string) error {
	switch {
	case len(ids) == 0:
		return ErrBulkPublicationEmpty
	case len(ids) > MaxBulkPublication:
		return fmt.Errorf("%w (tối đa %d người một lần)", ErrBulkPublicationTooLarge, MaxBulkPublication)
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			return ErrBulkPublicationMissingID
		}
		if err := KiemTraIDThamChieu(id); err != nil {
			return err
		}
		if _, dup := seen[id]; dup {
			return ErrBulkPublicationDuplicateID
		}
		seen[id] = struct{}{}
	}
	return nil
}
