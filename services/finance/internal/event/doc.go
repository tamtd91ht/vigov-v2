// Package event publishes and consumes facts for the finance service.
//
// Publishing happens AFTER the transaction commits (outbox), never inside it — otherwise the
// event can arrive before the data it describes.
//
// Every consumer is idempotent: queues deliver at least once, so the same message will arrive
// twice eventually. And a message with no commune is refused, never guessed — see
// pkg/events.Dispatch.
package event
