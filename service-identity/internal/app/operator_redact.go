package app

import (
	"fmt"
	"io"
	"log/slog"
	"strconv"
)

// Rendering of the operator request/response structs (security review of TASK-03, finding 3).
//
// These structs carry credentials or personal data in PLAIN string fields — plain because a handler
// fills them from a request body and the use case compares them. One `log.Info("login", "req", req)`
// or `%+v` while debugging would then put a working credential into centralised logging (rule 3,
// rule 8). So every such type closes EVERY rendering path itself:
//
//	Format    — every fmt verb. Load-bearing: fmt only consults String for %v/%s/%q/%x, so without
//	            Format a `%d` walks the fields by reflection and prints them (the hole core/secret
//	            documents).
//	GoString  — %#v, for code that calls it directly.
//	LogValue  — slog, which does not go through fmt.
//
// What each renders is the type name, the fields that identify nothing and prove nothing (the IP,
// the operator code, the expiry, a count), and a single "withheld" marker. The withheld fields are
// not even NAMED: the rendering is a list of what is safe, never a list of what was hidden.

func writeRedacted(f fmt.State, verb rune, s string) {
	if verb == 'q' {
		_, _ = io.WriteString(f, strconv.Quote(s))
		return
	}
	_, _ = io.WriteString(f, s)
}

const withheld = " credentials:withheld}"

func (r OperatorLoginRequest) String() string {
	return "app.OperatorLoginRequest{ip:" + r.IP + withheld
}
func (r OperatorLoginRequest) GoString() string              { return r.String() }
func (r OperatorLoginRequest) Format(f fmt.State, verb rune) { writeRedacted(f, verb, r.String()) }
func (r OperatorLoginRequest) LogValue() slog.Value          { return slog.StringValue(r.String()) }

func (r OperatorSessionIssued) String() string {
	return "app.OperatorSessionIssued{account_code:" + r.AccountCode +
		" expires_at:" + r.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z") + withheld
}
func (r OperatorSessionIssued) GoString() string              { return r.String() }
func (r OperatorSessionIssued) Format(f fmt.State, verb rune) { writeRedacted(f, verb, r.String()) }
func (r OperatorSessionIssued) LogValue() slog.Value          { return slog.StringValue(r.String()) }

func (r OperatorPasswordChange) String() string {
	return "app.OperatorPasswordChange{ip:" + r.IP + withheld
}
func (r OperatorPasswordChange) GoString() string              { return r.String() }
func (r OperatorPasswordChange) Format(f fmt.State, verb rune) { writeRedacted(f, verb, r.String()) }
func (r OperatorPasswordChange) LogValue() slog.Value          { return slog.StringValue(r.String()) }

func (r OperatorEnrollmentRequest) String() string {
	return "app.OperatorEnrollmentRequest{ip:" + r.IP + withheld
}
func (r OperatorEnrollmentRequest) GoString() string              { return r.String() }
func (r OperatorEnrollmentRequest) Format(f fmt.State, verb rune) { writeRedacted(f, verb, r.String()) }
func (r OperatorEnrollmentRequest) LogValue() slog.Value          { return slog.StringValue(r.String()) }

// OperatorEnrollmentStart's two secrets are already secret.Secret; the type still closes its own
// rendering so the promise does not depend on which fields someone adds next.
func (r OperatorEnrollmentStart) String() string {
	return "app.OperatorEnrollmentStart{account_code:" + r.AccountCode + withheld
}
func (r OperatorEnrollmentStart) GoString() string              { return r.String() }
func (r OperatorEnrollmentStart) Format(f fmt.State, verb rune) { writeRedacted(f, verb, r.String()) }
func (r OperatorEnrollmentStart) LogValue() slog.Value          { return slog.StringValue(r.String()) }

func (r OperatorEnrollmentCompletion) String() string {
	return "app.OperatorEnrollmentCompletion{ip:" + r.IP + withheld
}
func (r OperatorEnrollmentCompletion) GoString() string { return r.String() }
func (r OperatorEnrollmentCompletion) Format(f fmt.State, verb rune) {
	writeRedacted(f, verb, r.String())
}
func (r OperatorEnrollmentCompletion) LogValue() slog.Value { return slog.StringValue(r.String()) }

// OperatorEnrollmentResult EMBEDS OperatorSessionIssued, whose methods would otherwise be promoted
// and describe only the session half. Declared here so the recovery codes are covered too.
func (r OperatorEnrollmentResult) String() string {
	return "app.OperatorEnrollmentResult{account_code:" + r.AccountCode +
		" expires_at:" + r.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z") +
		" recovery_code_count:" + strconv.Itoa(len(r.RecoveryCodes)) + withheld
}
func (r OperatorEnrollmentResult) GoString() string              { return r.String() }
func (r OperatorEnrollmentResult) Format(f fmt.State, verb rune) { writeRedacted(f, verb, r.String()) }
func (r OperatorEnrollmentResult) LogValue() slog.Value          { return slog.StringValue(r.String()) }
