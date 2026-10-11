package domain

// Zalo GROUP delivery of a staff notice (ADR 0092; comms.proto StaffNotification fields 8 and 9): which
// unit holds the record the notice is about, and whether the notice may enter a group at all. Decided
// HERE, per record, because comms cannot: the same kind is sent for an ordinary petition and for a
// restricted one (ADR 0030), and a group's members were never checked against `feedback.restricted`.

// GroupPosting mirrors comms' GroupPosting without importing it (domain imports only the standard
// library); app maps it onto the wire. The ZERO VALUE is Unspecified on purpose: a builder that does not
// decide posts to NO group (comms fails closed on it), so a notice forgotten here stays out of groups.
type GroupPosting int

const (
	// GroupPostingUnspecified — digests and per-person notices (due-soon, weekly digest, report ready,
	// mention, extension request): their key is per person or per period, so comms' per-group dedupe
	// would post the first recipient's version to the group (comms.proto, GROUP_POSTING_ALLOWED).
	GroupPostingUnspecified GroupPosting = iota
	// GroupPostingAllowed — ONE record, and title / body / link the same for every recipient.
	GroupPostingAllowed
	// GroupPostingRestricted — the record is restricted (a `can-bo` petition, ADR 0030). Never posted.
	GroupPostingRestricted
)

// recordGroupPosting is the decision for a notice about ONE record whose text is the same for every
// recipient: Allowed, unless the record is restricted — whose recipients the producer filters per
// person, so a group (members nobody checked) must never receive it.
func recordGroupPosting(restricted bool) GroupPosting {
	if restricted {
		return GroupPostingRestricted
	}
	return GroupPostingAllowed
}

// GroupUnit is the record's unit as comms accepts it: a ULID (26 characters of Crockford base32), or "".
//
// WHY A SHAPE CHECK HERE: comms refuses a malformed holding_org_unit_id with INVALID_ARGUMENT for the
// WHOLE batch — the bell rows of every other notice in the call with it, and an automation run whose
// first failed page stops delivery would fail the same way on every beat. "" is the contract's
// "unknown": the notice reaches commune-wide groups only, which it reaches with any unit anyway, so
// dropping a malformed id can never widen who sees it.
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
