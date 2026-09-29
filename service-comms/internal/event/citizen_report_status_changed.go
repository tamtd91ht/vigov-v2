package event

// The consumer of `petitions.status_changed.v1` — the first thing in this repository that turns a
// fact from another service into a line of the citizen notification ledger.
//
// WHY IT LIVES HERE AND NOT IN app/: app.CitizenNotifications answers "given an obligation, what is
// recorded and in which transaction". It knows nothing about queues, envelopes or wire formats, and it
// must stay that way — that is what lets its properties be proved without a broker. This file is the
// other half: it reads the envelope, refuses what the contract does not allow, and hands app/ a
// `RecordRequest`. There is deliberately NO second write path to the ledger here; everything goes
// through the use case that already puts the row and its audit entry in one transaction (rule 6,
// invariant 3).
//
// WHAT IS NOT WIRED, and saying so is the point (ADR 0010): Kafka carries events between services,
// but no broker client exists in this repository yet — `core/events.Publisher` is still a bare
// interface with no implementation. So this consumer is a FUNCTION THAT TAKES AN ENVELOPE. The day
// the Kafka consumer group is written, it calls Receive and nothing here changes. Building a broker
// client now would be inventing the half nobody has decided, inside the half that is decided.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/events"
	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// CitizenReportStatusChangedEvent is the ONE name this consumer accepts.
//
// The version is part of the name (rule 2, invariant 4) and the check against it is not
// defensive noise: two versions of an event run side by side during a migration, and a `.v2`
// body decoded by a `.v1` consumer produces fields that are silently absent rather than an
// error. Refusing an unexpected name keeps that failure loud on the day it can still be fixed.
const CitizenReportStatusChangedEvent = "petitions.status_changed.v1"

// CitizenNotificationRecorder is the ledger use case, declared here at the point of use
// (`skills/go-service-pattern` #2). The consumer names an interface rather than
// *app.CitizenNotifications so that what this file decides — what it refuses, and what it passes
// through untouched — is provable without a database.
type CitizenNotificationRecorder interface {
	Record(ctx context.Context, req app.RecordRequest) (app.RecordResult, error)
}

// TemplateResolver answers: which APPROVED ZNS template does THIS commune send for THIS transition.
//
// IT IS AN INTERFACE WITH NO IMPLEMENTATION IN THIS REPOSITORY, AND THAT IS A STOP CONDITION MADE
// VISIBLE RATHER THAN A DEFAULT WRITTEN QUIETLY. Onboarding a commune is "register the OA + get the
// ZNS templates approved" (ADR 0018, consequence 1), so the template code is per-commune
// configuration that must be read at RUNTIME (rule 1, invariant 10) — and no table, no adapter and
// no customer answer for it exists yet. A constant here would be one commune's template id
// hardcoded for 200+ communes (rule 10, forbidden #3), and it would be invisible until the first
// message reached the wrong OA.
//
// The commune is NOT a parameter: it travels in the context (rule 1, invariant 4).
type TemplateResolver interface {
	TemplateCode(ctx context.Context, milestone string) (string, error)
}

var (
	// ErrChannelNotConfigured is what a TemplateResolver implementation returns for a commune whose OA
	// is not configured yet. It is declared here, next to the interface it belongs to, so both sides
	// compare against one value instead of matching on an error string.
	//
	// It is NOT a failure to retry — see ErrNoRetry.
	ErrChannelNotConfigured = errors.New("su_kien: xã chưa cấu hình kênh gửi cho công dân")

	// ErrNoRetry marks an outcome that WILL NOT become correct by being delivered again: a
	// malformed body, an unknown event name, a commune with no channel. The queue adapter checks
	// `errors.Is(err, ErrNoRetry)` and sends the message to the dead-letter queue with a flag
	// staff can see, instead of spinning.
	//
	// WHY IT MATTERS FOR THE COMMUNE WITH NO OA: retrying that one can never succeed, so the retry
	// loop turns a real signal into noise and ADR 0006 consequence 3 — degrade VISIBLY — is exactly
	// what gets lost.
	ErrNoRetry = errors.New("su_kien: không thử lại")

	// ErrWrongEventName — the envelope carries another event, or another version of this one.
	ErrWrongEventName = errors.New("su_kien: tên sự kiện không thuộc consumer này")

	// ErrPayloadDecode — the body is not the protojson of PetitionStatusChanged.
	ErrPayloadDecode = errors.New("su_kien: không giải mã được payload")

	// ErrMissingLookupCode — no lookup code. The citizen's only handle on their case (rule 10,
	// invariant 1), and the ledger's business key.
	ErrMissingLookupCode = errors.New("su_kien: thiếu mã tra cứu")

	// ErrMissingStatus — no status. Nothing says what changed.
	ErrMissingStatus = errors.New("su_kien: thiếu trạng thái")

	// ErrMissingRecipient — no citizen id. Nobody is owed the message, and the deduplication key
	// would be the same for every citizen of the commune.
	ErrMissingRecipient = errors.New("su_kien: thiếu người nhận")

	// ErrInvalidOccurrence — `occurrence` is 0.
	//
	// REFUSED, NEVER DEFAULTED TO 1, and this is the subtle one. `occurrence` says WHICH time this
	// petition reached this status; ADR 0008 lets a citizen reopen a closed petition, so the second
	// closing is a different commitment owing a different message. A producer that sends 0 has not
	// told us which occurrence this is, and quietly calling it the first rebuilds the collapse the
	// field exists to prevent: the citizen is told about the first closing and never about the
	// second, while the system looks like it is deduplicating correctly.
	ErrInvalidOccurrence = errors.New("su_kien: lần chuyển trạng thái không hợp lệ")
)

// CitizenReportStatusChanged consumes `petitions.status_changed.v1`.
type CitizenReportStatusChanged struct {
	ledger    CitizenNotificationRecorder
	templates TemplateResolver
	log       *slog.Logger
}

func NewCitizenReportStatusChanged(ledger CitizenNotificationRecorder, templates TemplateResolver,
	log *slog.Logger) *CitizenReportStatusChanged {
	if log == nil {
		log = slog.Default()
	}
	return &CitizenReportStatusChanged{ledger: ledger, templates: templates, log: log}
}

// Receive is the ONLY way in, and it is the only way in ON PURPOSE.
//
// It runs the body through core/events.Dispatch, which REFUSES an envelope with no commune rather
// than guessing one (rule 1, invariant 9) and puts the commune from the envelope into the context
// (rule 1, invariant 4). The handler below is unexported precisely so there is no second entry that
// skips that step: a consumer writing into whichever commune happened to be convenient is a breach
// between two public authorities, and it would leave no trace that says so.
func (c *CitizenReportStatusChanged) Receive(ctx context.Context, e events.Envelope) error {
	return events.Dispatch(ctx, e, c.handle)
}

func (c *CitizenReportStatusChanged) handle(ctx context.Context, e events.Envelope) error {
	if e.Name != CitizenReportStatusChangedEvent {
		return fmt.Errorf("%w: %w: %q", ErrNoRetry, ErrWrongEventName, e.Name)
	}

	var msg petitionsv1.PetitionStatusChanged
	// DiscardUnknown IS REQUIRED, not a convenience. Rule 2, invariant 4 allows the publisher to
	// add an OPTIONAL field to a live event — `previous_status` is named in the contract as exactly
	// that — and strict decoding would turn that permitted change into every message failing here.
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(e.Payload, &msg); err != nil {
		// THE DECODER'S MESSAGE IS DELIBERATELY NOT WRAPPED IN. It quotes the offending part of the
		// body, and a body that broke the contract is precisely the one that may carry what the
		// contract forbids (rule 3): this error travels into logs, into alerts and into a ticket.
		// The envelope id is opaque and is what an operator needs to find the message itself.
		return fmt.Errorf("%w: %w: phong bì %q", ErrNoRetry, ErrPayloadDecode, e.ID)
	}

	if err := validateMessage(&msg); err != nil {
		return fmt.Errorf("%w: %w", ErrNoRetry, err)
	}

	// ABSENCE OF citizen_message IS A FACT, NOT A FAILURE. The event is published on all nine
	// transitions because the fact is that the status moved; whether the citizen is also owed a
	// message is said by this field being present. Classify, assign and field acceptance are
	// internal steps (`skills/petition-lifecycle`) and owe nobody anything, so: no ledger row, no
	// send — and NO LOG LINE either. Logging it would print a normal, frequent, correct outcome as
	// though it were an anomaly, and an alert channel that cries at correct behaviour is one people
	// learn to skip past.
	if msg.GetCitizenMessage() == nil {
		return nil
	}

	templateCode, err := c.templates.TemplateCode(ctx, msg.GetStatus())
	if err != nil {
		if errors.Is(err, ErrChannelNotConfigured) {
			// ADR 0006 consequence 3: a commune with no OA must degrade VISIBLY. Visible here is a
			// warning plus a non-retryable outcome the queue adapter turns into a dead letter and a
			// flag, not an endless retry.
			//
			// WHAT IS LOGGED IS A BUSINESS CODE AND NOTHING ELSE (rule 3, invariant 2): the lookup
			// code identifies a record, the transition is a code, the commune is a ULID. The
			// recipient — `citizen_id` — is NOT logged: it is opaque, but it is still a handle on a
			// person, and a log line pairing it with a record code is the first half of a map of
			// who reported what.
			c.log.Warn("chưa gửi được thông báo: xã chưa cấu hình kênh",
				"su_kien", e.Name, "xa", string(tenant.MustFrom(ctx)),
				"ma_tra_cuu", msg.GetLookupCode(), "moc", msg.GetStatus())
			return fmt.Errorf("%w: %w", ErrNoRetry, err)
		}
		// Anything else — the configuration store being unreachable, for instance — IS worth
		// retrying, so it is returned without the non-retryable marker.
		return fmt.Errorf("su_kien: mẫu tin của xã: %w", err)
	}

	// IDEMPOTENCY IS NOT IMPLEMENTED HERE, AND THAT IS THE DESIGN. app.CitizenNotifications.Record
	// builds domain.SendKey from the business fact and lets a UNIQUE constraint decide, so a
	// redelivery — and a republication under a fresh envelope id, which deduplicating on `e.ID`
	// would miss — both land on the same row. What this function must not do is lose a component of
	// that key, which is why `occurrence` is passed through and refused when absent.
	if _, err := c.ledger.Record(ctx, app.RecordRequest{
		SubjectType:   domain.SubjectCitizenReport,
		SubjectCode:   msg.GetLookupCode(),
		Milestone:     msg.GetStatus(),
		Round:         int(msg.GetOccurrence()),
		Channel:       domain.ChannelZaloZNS,
		RecipientCode: msg.GetCitizenId(),
		TemplateCode:  templateCode,
		Params: domain.NotificationParams{
			LookupCode:  msg.GetLookupCode(),
			StatusLabel: msg.GetCitizenMessage().GetStatusLabel(),
			NextStep:    msg.GetCitizenMessage().GetNextStep(),
		},
		// Actor is left zero ON PURPOSE: app.CitizenNotifications.Record turns that into the system
		// principal, which is what rule 6, invariant 6 prescribes for background work. The actor who
		// moved the status is deliberately absent from the contract — `petitions` already audited
		// that act, in the same transaction as the change, and a staff identity arriving on the
		// citizen notification path is a step toward a staff name leaving the commune.
	}); err != nil {
		return fmt.Errorf("su_kien: ghi sổ thông báo: %w", err)
	}
	return nil
}

// validateMessage refuses a body the contract says cannot happen.
//
// It refuses rather than repairs. Every one of these is a producer defect, and the repair that
// looks reasonable at each site — an empty code, a defaulted occurrence — produces a ledger row
// that is wrong in a way nothing downstream can detect.
func validateMessage(msg *petitionsv1.PetitionStatusChanged) error {
	switch {
	case msg.GetLookupCode() == "":
		return ErrMissingLookupCode
	case msg.GetStatus() == "":
		return ErrMissingStatus
	case msg.GetCitizenId() == "":
		return ErrMissingRecipient
	case msg.GetOccurrence() == 0:
		return ErrInvalidOccurrence
	}
	return nil
}
