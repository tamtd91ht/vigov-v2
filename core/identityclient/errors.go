package identityclient

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrIdentityUnavailable marks an error meaning "identity did not answer, and asking again may
// work": the gRPC code was UNAVAILABLE or DEADLINE_EXCEEDED.
//
// WHY A SENTINEL AND NOT status.Code AT EVERY CALL SITE: the caller is an HTTP handler that has to
// pick a 503 with a retry sentence, and the only place that can see a gRPC code is this package. A
// handler importing google.golang.org/grpc/status to decide its own response is a second reading of
// what a failure means, in every service that does it.
//
// IT IS NEVER AN ANSWER. Not "not live", not "no unit", not "nothing is due soon". A caller that
// folds it into an empty result writes an unchecked id, hides the tasks a tab is named for, or
// drops a filter silently — every contract in identity.proto names that as the forbidden reading.
//
// ONLY THE TWO TRANSIENT CODES CARRY IT. INVALID_ARGUMENT, UNAUTHENTICATED (the caller key) and
// INTERNAL are also "the call did not happen" and also answer 503, but retrying them changes
// nothing — they are fixed by a deploy, not by waiting — so they stay plain errors. The original
// status is still wrapped, so status.Code(err) keeps working for the operator's log line.
var ErrIdentityUnavailable = errors.New("identityclient: identity tạm thời không trả lời — thử lại sau")

// ErrDueSoonNotConfigured marks FAILED_PRECONDITION from ResolveDueSoonCutoff: this commune has no
// usable `sla` row (or no usable working calendar) for the kind of work asked about.
//
// THE CALLER MAPS IT TO A CLEAR REFUSAL OF THE FILTER (a 409-class sentence such as "xã chưa cấu
// hình ngưỡng sắp đến hạn"), and NEVER to a number. Not 72 hours, not the specification's table:
// the threshold is the commune's, and a figure invented by software filters an officer's register
// on a basis nobody decided (rule 10, forbidden #3). Nor is it "nothing is due soon" — an empty
// page there reads as "all clear".
//
// It is the ORDINARY answer today: migration 0008 seeds no `sla` row for any commune.
var ErrDueSoonNotConfigured = errors.New("identityclient: xã chưa cấu hình ngưỡng sắp đến hạn")

// wrapCallError turns a failed identity call into an error carrying, where one applies, the
// sentinel a caller branches on — AND the original gRPC error, so status.Code still reads it.
//
// notConfigured is the sentinel FAILED_PRECONDITION maps to for this RPC, or nil when the RPC's
// contract does not produce that code as a configuration answer.
func wrapCallError(method string, err error, notConfigured error) error {
	switch code := status.Code(err); {
	case code == codes.Unavailable || code == codes.DeadlineExceeded:
		return fmt.Errorf("identityclient: %s: %w: %w", method, ErrIdentityUnavailable, err)
	case code == codes.FailedPrecondition && notConfigured != nil:
		return fmt.Errorf("identityclient: %s: %w: %w", method, notConfigured, err)
	default:
		return fmt.Errorf("identityclient: %s: %w", method, err)
	}
}
