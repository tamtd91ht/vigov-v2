package domain

// Zalo GROUP delivery of a staff notice (ADR 0092; comms.proto StaffNotification fields 8 and 9): which
// unit holds the record the notice is about, and whether the notice may enter a group at all. Same
// design as service-petitions/internal/domain/group_posting.go.
//
// THE RESTRICTED RECORD HERE IS A DENUNCIATION (`to-cao`, LetterType.ProtectsIdentity): the Law on
// Denunciation keeps the denunciation and its sender confidential, and a group has members nobody
// checked — so no notice about one ever enters a group, whatever its scope (main session decision
// 11/10/2026, fail closed; the owner may reverse it). ADR 0092 #6 names the `can-bo` petition; this is
// the same class of record on the letter register.

// GroupPosting mirrors comms' GroupPosting without importing it (domain imports only the standard
// library); app maps it onto the wire. The ZERO VALUE is Unspecified on purpose: a builder that does not
// decide posts to NO group (comms fails closed on it).
type GroupPosting int

const (
	// GroupPostingUnspecified — digests and per-person notices (due-soon, weekly digest, report ready):
	// their key is per person or per period, so comms' per-group dedupe would post the first
	// recipient's version to the group (comms.proto, GROUP_POSTING_ALLOWED).
	GroupPostingUnspecified GroupPosting = iota
	// GroupPostingAllowed — ONE record, and title / body / link the same for every recipient.
	GroupPostingAllowed
	// GroupPostingRestricted — the record is restricted (a denunciation letter). Never posted.
	GroupPostingRestricted
)

// recordGroupPosting is the decision for a notice about ONE record whose text is the same for every
// recipient: Allowed, unless the record is restricted.
func recordGroupPosting(restricted bool) GroupPosting {
	if restricted {
		return GroupPostingRestricted
	}
	return GroupPostingAllowed
}

// GroupUnit is the record's unit as comms accepts it: a ULID (26 characters of Crockford base32), or "".
//
// WHY A SHAPE CHECK HERE: comms refuses a malformed holding_org_unit_id with INVALID_ARGUMENT for the
// WHOLE batch — the bell rows of every other notice in the call with it, and a runner whose first failed
// page stops delivery would fail the same way on every beat. "" is the contract's "unknown": the notice
// reaches commune-wide groups only, which it reaches with any unit anyway, so dropping a malformed id
// can never widen who sees it.
func GroupUnit(id string) string {
	if len(id) != 26 {
		return ""
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'A' && c <= 'Z' && c != 'I' && c != 'L' && c != 'O' && c != 'U':
		default:
			return ""
		}
	}
	return id
}
