package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// ZaloBotDisplayLog writes platform's DISPLAY row for an act on the shared Zalo Bot that comms has
// already answered (domain.ZaloBotAct). A RAW HANDLE, like SharedMiniAppStore: the row goes to
// platform_audit_log, which has no commune by migration 0008 — the bot belongs to no commune.
//
// NOT the trail of the act. The act is comms' write, recorded by comms in the same transaction
// (zalo_bot_operator.proto, "THE TRAIL"); this row is written AFTER, in its own statement, and when
// the two disagree comms' entry is the record. That is why a failure here is the caller's to log, not
// to turn into a failed request.
type ZaloBotDisplayLog struct {
	db *sql.DB
}

// NewZaloBotDisplayLog binds the writer to this service's own database (rule 2 invariant 2).
func NewZaloBotDisplayLog(db *sql.DB) *ZaloBotDisplayLog { return &ZaloBotDisplayLog{db: db} }

// RecordZaloBotAct appends one platform_audit_log row: actor = the operator's business code (rule 6
// invariant 8 — an empty code refuses), before = NULL (comms holds the history), after = the act's
// metadata, reason = NULL (no Zalo write asks for one, owner 11/10/2026).
func (l *ZaloBotDisplayLog) RecordZaloBotAct(ctx context.Context, act domain.ZaloBotAct, by domain.OperatorActor) error {
	if err := by.Validate(); err != nil {
		return err
	}
	after, err := zaloBotActAfter(act)
	if err != nil {
		return err
	}
	if _, err := l.db.ExecContext(ctx, `
		INSERT INTO platform_audit_log (actor, actor_ip, action, subject, before, after, reason)
		VALUES ($1, $2, $3, $4, NULL, $5::jsonb, NULL)`,
		by.Code, by.IP, act.Action, domain.ZaloBotSubject, string(delta(after))); err != nil {
		return fmt.Errorf("zalo bot display row: %w", err)
	}
	return nil
}

// zaloBotActAfter is the `after` object of one verb — only the fields that verb has.
func zaloBotActAfter(act domain.ZaloBotAct) (map[string]any, error) {
	after := map[string]any{"outcome": act.Outcome}
	switch act.Action {
	case domain.ZaloBotActionTokenSet:
		after["bot_name"], after["chat_url"], after["ended_link_count"] = act.BotName, act.ChatURL, act.EndedLinkCount
	case domain.ZaloBotActionChecked:
	case domain.ZaloBotActionWebhookSet:
		if act.WebhookURL != "" {
			after["url"] = act.WebhookURL
		}
	default:
		return nil, fmt.Errorf("zalo bot display row: unknown action %q", act.Action)
	}
	return after, nil
}
