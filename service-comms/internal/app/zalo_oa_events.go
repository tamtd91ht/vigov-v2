package app

// FORWARDING ZALO OA WEBHOOK EVENTS TO KAFKA (owner decision 17/10/2026, TASK W-1).
//
// Zalo posts every event of every OA authorised to the platform's shared app (ADR 0098) to one URL on the
// platform's callback host. This use case decides, for one posted body:
//
//	1. configured?   webhook secret, app id and publisher all present — else ErrZaloOAEventsNotConfigured
//	                 (503, dev only: staging/prod refuse to start without them)
//	2. authentic?    X-ZEvent-Signature verifies AND the body's app_id is ZALO_OA_APP_ID — else
//	                 ErrZaloOAEventForbidden (403), nothing published
//	3. whose?        the OA named by the owner's rule (domain.ZaloOAEvent.OAID) → the commune holding it
//	                 LIVE (crosstenant). No OA, or not live anywhere → Ignored (200, so Zalo stops
//	                 retrying) and NOTHING is published: fail closed, never a guessed commune
//	4. publish       core/events.Envelope{ID: ULID, Name: comms.zalo_oa_event_received.v1, TenantID, At,
//	                 Payload: the raw body, unchanged}, synchronously — the caller answers 200 only after
//	                 the broker acknowledged; a broker error is ErrZaloOAEventsUnavailable (503, Zalo retries)
//
// THE PAYLOAD IS THE WHOLE ZALO BODY, PERSONAL DATA INCLUDED — the owner's decision (17/10/2026), which
// departs from skills/events-and-queues ("messages must not carry personal data"); the topic's retention is
// operations' call. What this code still guarantees is rule 3 in the LOGS: the body, the user ids, the
// message text and the OA id are never written to a log line or an error.
//
// NO AUDIT ENTRY: nothing in this flow writes business data. The one cross-commune read returns a commune
// identifier for a signed event — the same shape as the authorization callback's state lookup, which is not
// audited either.
//
// AT LEAST ONCE: Zalo retries a delivery it saw fail, and each delivery gets a fresh envelope ID (owner:
// "ID (ULID)"), so a consumer deduplicates on the payload's own identifiers (e.g. message.msg_id), never on
// the envelope ID alone.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/events"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ZaloOAEventReceivedName is the event's name — and so its Kafka topic (core/events/kafka). Declared in
// proto/vigov/comms/v1/events.proto. Past tense, versioned (rule 2, invariant 4). In THIS package, beside
// the envelope it names: tools/kb (event-flows) follows a name through a constant of the same package only.
const ZaloOAEventReceivedName = "comms.zalo_oa_event_received.v1"

var (
	// ErrZaloOAEventsNotConfigured: the webhook secret, the app id or the broker is missing (dev only).
	ErrZaloOAEventsNotConfigured = errors.New("zalo oa events: webhook secret, app id or event broker not configured")
	// ErrZaloOAEventForbidden: the signature is missing or wrong, the body is unreadable, or the app is
	// not the platform's. Nothing was published.
	ErrZaloOAEventForbidden = errors.New("zalo oa events: signature or app refused")
	// ErrZaloOAEventsUnavailable: the commune lookup or the broker failed. Nothing was acknowledged, so Zalo
	// retries.
	ErrZaloOAEventsUnavailable = errors.New("zalo oa events: temporarily unavailable")
)

// ZaloOAEventOutcome is what happened to an authenticated event.
type ZaloOAEventOutcome string

const (
	// ZaloOAEventPublished: the broker acknowledged the envelope.
	ZaloOAEventPublished ZaloOAEventOutcome = "published"
	// ZaloOAEventNoOA: the event names no OA by the owner's rule. Not published.
	ZaloOAEventNoOA ZaloOAEventOutcome = "no-oa"
	// ZaloOAEventUnknownOA: the OA is live in no commune. Not published.
	ZaloOAEventUnknownOA ZaloOAEventOutcome = "unknown-oa"
)

// ZaloOAEventResult is the outcome plus what may be logged: the event name (shape-checked) and, once
// resolved, the commune.
type ZaloOAEventResult struct {
	Outcome   ZaloOAEventOutcome
	EventName string
	Tenant    tenant.ID
}

// ZaloOAEventTenantResolver is crosstenant.ZaloOA's live oa_id → commune lookup.
type ZaloOAEventTenantResolver interface {
	TenantByLiveOAID(ctx context.Context, oaID string) (tenant.ID, bool, error)
}

// ZaloOAEventsConfig is the shared app's id and its webhook "OA Secret Key".
type ZaloOAEventsConfig struct {
	AppID         string
	WebhookSecret secret.Secret
}

// ZaloOAEvents is the use case.
type ZaloOAEvents struct {
	resolver  ZaloOAEventTenantResolver
	publisher events.Publisher
	cfg       ZaloOAEventsConfig
	now       func() time.Time
	newID     func() (string, error)
}

// NewZaloOAEvents builds the use case. publisher may be nil — KAFKA_EVENT_ADDRESS unset (dev only) — and
// every event then answers ErrZaloOAEventsNotConfigured: refused at use, never dropped.
func NewZaloOAEvents(resolver ZaloOAEventTenantResolver, publisher events.Publisher, cfg ZaloOAEventsConfig) *ZaloOAEvents {
	if resolver == nil {
		panic("comms/app: ZaloOAEvents needs the live-OA resolver")
	}
	return &ZaloOAEvents{resolver: resolver, publisher: publisher, cfg: cfg,
		now: func() time.Time { return time.Now().UTC() }, newID: ulid.Moi}
}

// Configured reports whether every event can be verified and published.
func (z *ZaloOAEvents) Configured() bool {
	return z.publisher != nil && z.cfg.AppID != "" && !z.cfg.WebhookSecret.Rong()
}

// Receive handles one posted body. signature is the X-ZEvent-Signature header as sent.
func (z *ZaloOAEvents) Receive(ctx context.Context, signature string, body []byte) (ZaloOAEventResult, error) {
	if !z.Configured() {
		return ZaloOAEventResult{}, ErrZaloOAEventsNotConfigured
	}
	ev, err := domain.ParseZaloOAEvent(body)
	if err != nil {
		return ZaloOAEventResult{}, fmt.Errorf("%w: %w", ErrZaloOAEventForbidden, err)
	}
	// The signature first, then the app: both are 403, and neither says which failed.
	if !domain.VerifyZaloOAEventSignature(signature, ev, body, z.cfg.WebhookSecret.Lo()) || ev.AppID != z.cfg.AppID {
		return ZaloOAEventResult{}, ErrZaloOAEventForbidden
	}
	res := ZaloOAEventResult{EventName: domain.LoggableZaloOAEventName(ev.EventName)}
	if ev.OAID == "" {
		res.Outcome = ZaloOAEventNoOA
		return res, nil
	}
	t, found, err := z.resolver.TenantByLiveOAID(ctx, ev.OAID)
	if err != nil {
		return res, fmt.Errorf("%w: %w", ErrZaloOAEventsUnavailable, err)
	}
	if !found {
		res.Outcome = ZaloOAEventUnknownOA
		return res, nil
	}
	res.Tenant = t

	id, err := z.newID()
	if err != nil {
		return res, fmt.Errorf("%w: event id: %w", ErrZaloOAEventsUnavailable, err)
	}
	// A copy: the envelope must not alias a buffer the caller may clear after Receive returns.
	payload := append([]byte(nil), body...)
	env := events.Envelope{ID: id, Name: ZaloOAEventReceivedName, TenantID: t, At: z.now(), Payload: payload}
	if err := z.publisher.Publish(ctx, env); err != nil {
		return res, fmt.Errorf("%w: %w", ErrZaloOAEventsUnavailable, err)
	}
	res.Outcome = ZaloOAEventPublished
	return res, nil
}
