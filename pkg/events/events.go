// Package events carries facts between services.
//
// Names are in the PAST TENSE and carry a version: "petitions.received.v1". An event
// describes something that has already happened, never a command — a command would make the
// publisher depend on the consumer's behaviour, which is the coupling events exist to avoid.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/pkg/tenant"
)

var ErrNoTenant = errors.New("events: message carries no commune")

// Envelope wraps every message.
//
// TenantID sits in the envelope and not in the payload because the payload is data the
// publisher declares, while the envelope is set by infrastructure. Consumers trust the
// envelope; they never trust a commune named inside a body.
type Envelope struct {
	ID       string          `json:"id"`   // stable, for deduplication
	Name     string          `json:"name"` // "petitions.received.v1"
	TenantID tenant.ID       `json:"tenant_id"`
	At       time.Time       `json:"at"`
	Payload  json.RawMessage `json:"payload"`
}

type Publisher interface {
	Publish(ctx context.Context, e Envelope) error
}

// Handler processes one message. It must be idempotent: queues deliver at least once, so the
// same message will arrive twice at some point.
type Handler func(ctx context.Context, e Envelope) error

// Dispatch validates the envelope and runs the handler with the commune in context.
//
// A message without a commune is REFUSED, never guessed. Guessing here would mean a
// background job writing into whichever commune happened to be convenient.
func Dispatch(ctx context.Context, e Envelope, h Handler) error {
	if !e.TenantID.Valid() {
		return fmt.Errorf("%w: %q", ErrNoTenant, e.Name)
	}
	return h(tenant.Into(ctx, e.TenantID), e)
}
