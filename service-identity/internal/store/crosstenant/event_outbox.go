package crosstenant

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/core/tenant"
)

// EventOutboxCommunes answers two identifier-only questions that span communes, for identity's background
// work on the citizen-directory search (ADR 0101 #11):
//
//   - which communes still owe an event (the relay then works IN each commune's context), and
//   - which communes have a directory at all (the "reindex every commune" command, needed after
//     SEARCH_HASH_KEY changes).
//
// IDENTIFIERS ONLY, BY CONSTRUCTION: both statements select DISTINCT tenant_id and nothing else, so no
// row of any person can leave through this type. It holds the raw handle because there is no commune to
// scope these reads to — which is why it lives in this package (doc.go).
type EventOutboxCommunes struct {
	raw *sql.DB
}

func NewEventOutboxCommunes(db *sql.DB) *EventOutboxCommunes {
	if db == nil {
		panic("crosstenant: NewEventOutboxCommunes needs a database handle")
	}
	return &EventOutboxCommunes{raw: db}
}

// CommunesWithUnpublishedEvents lists the communes whose event_outbox holds an unpublished row.
func (s *EventOutboxCommunes) CommunesWithUnpublishedEvents(ctx context.Context) ([]tenant.ID, error) {
	// @cross-tenant: identity's outbox relay (ADR 0101 #11) needs the communes owing an event; it then
	// publishes in each commune's own context. tenant_id only — no event, no payload.
	rows, err := s.raw.QueryContext(ctx,
		`SELECT DISTINCT tenant_id FROM event_outbox WHERE published_at IS NULL ORDER BY tenant_id`)
	return readCommuneIDs(rows, err)
}

// CommunesWithContacts lists the communes that have at least one live directory row.
func (s *EventOutboxCommunes) CommunesWithContacts(ctx context.Context) ([]tenant.ID, error) {
	// @cross-tenant: the operator's "reindex every commune" command (ADR 0101 #11, owner 17/10/2026 —
	// a changed SEARCH_HASH_KEY needs every commune rebuilt). tenant_id only — no row of any person.
	rows, err := s.raw.QueryContext(ctx,
		`SELECT DISTINCT tenant_id FROM citizen_contact WHERE deleted_at IS NULL ORDER BY tenant_id`)
	return readCommuneIDs(rows, err)
}

func readCommuneIDs(rows *sql.Rows, err error) ([]tenant.ID, error) {
	if err != nil {
		return nil, fmt.Errorf("crosstenant: đọc danh sách xã: %w", err)
	}
	defer rows.Close()
	var out []tenant.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("crosstenant: đọc mã xã: %w", err)
		}
		// A malformed id is skipped, never guessed into another commune (rule 1).
		if t := tenant.ID(id); t.Valid() {
			out = append(out, t)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crosstenant: duyệt danh sách xã: %w", err)
	}
	return out, nil
}
