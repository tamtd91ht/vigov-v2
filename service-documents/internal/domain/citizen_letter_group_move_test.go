package domain

import (
	"errors"
	"testing"
)

// Row 44: the status a display group stands for, from every status that has an arrow out. The table is
// the whole of C3 × ADR 0084 §2, so a new arrow that makes a group ambiguous turns this red.
func TestNextStatusInGroup(t *testing.T) {
	for _, tc := range []struct {
		from LetterStatus
		g    LetterStatusGroup
		want LetterStatus
		err  error
	}{
		{LetterStatusNew, LetterGroupInProgress, LetterStatusScreening, nil},
		{LetterStatusScreening, LetterGroupInProgress, LetterStatusAdmitted, nil},
		{LetterStatusScreening, LetterGroupForwarded, LetterStatusForwarded, nil},
		{LetterStatusScreening, LetterGroupNotProcessed, "", ErrLetterGroupAmbiguous},
		{LetterStatusAdmitted, LetterGroupInProgress, LetterStatusResolving, nil},
		{LetterStatusResolving, LetterGroupResolved, LetterStatusResolved, nil},
		{LetterStatusResolving, LetterGroupNotProcessed, LetterStatusDiscontinued, nil},
		{LetterStatusResolving, LetterGroupInProgress, "", ErrLetterGroupUnreachable},
		{LetterStatusNew, LetterGroupResolved, "", ErrLetterGroupUnreachable},
		{LetterStatusNew, LetterGroupAssigned, "", ErrLetterGroupUnreachable},
		{LetterStatusFiled, LetterGroupInProgress, "", ErrLetterGroupUnreachable},
		{LetterStatusNew, LetterStatusGroup("cho-phan-cong"), "", ErrLetterGroupInvalid},
	} {
		got, err := NextStatusInGroup(tc.from, tc.g)
		if got != tc.want || !errors.Is(err, tc.err) && !(err == nil && tc.err == nil) {
			t.Errorf("%s → %s = %q, %v; muốn %q, %v", tc.from, tc.g, got, err, tc.want, tc.err)
		}
	}
}
