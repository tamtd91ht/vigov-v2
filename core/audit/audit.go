// Package audit records who did what, in the same transaction as the change.
//
// WHY THIS IS A LIBRARY AND NOT A SERVICE — the single most consequential decision in the
// backend architecture:
//
//	Rule 6 requires the audit entry to share a transaction with the business write. Two
//	services cannot share a transaction. Making audit a service would therefore LOCK INTO THE
//	ARCHITECTURE the exact defect measured on the previous system: zero transactions across
//	the whole backend, so there was always a window where the record had changed and the trail
//	had not. When an inspection asks "who changed this file", the answer was: nobody knows.
//
// See kb/10-decisions/0001-service-decomposition.md.
//
// An audit trail is NOT a technical log. Logs rotate by size and may be lost. This is
// business data: retained at least 12 months, never edited, never deleted.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Actor identifies who performed the action.
type Actor struct {
	ID   string // staff code, citizen id, operator code (VH-…), or SystemActor
	Kind string // "staff" | "citizen" | "system" | KindOperator | KindZaloAccount
	IP   string
}

// KindZaloAccount is the actor_kind of an act by a Mini App session WITHOUT a verified phone, owned by
// its Zalo account (ADR 0080 #9): actor_id is `tai_khoan_zalo.id`, an opaque ULID — rule 6 invariant 8
// (business code) binds staff only. Like a citizen's entry, its actor id and IP are WITHHELD from the
// commune's audit screen (Log.Read, ADR 0054 §4): the id names a person's Zalo account, and an exact
// actor filter on it would let a guessed id confirm that person's activity. No CHECK on the column in
// any service, so the value needs no migration. Equal to authz.KindZaloAccount, pinned by a test.
const KindZaloAccount = "zalo-account"

// SystemActor is used for background jobs and migrations. System actions are audited too;
// an unattributed change is the thing this package exists to prevent.
const SystemActor = "system"

// KindOperator is the actor_kind of a Vihat OPERATOR acting on a commune's data (ADR 0048, ADR 0070
// §"Bổ sung 02/10/2026" #7). The entry is written in the commune's audit_log, in the same transaction
// as the change, with the operator's VH- code as actor — and it is WITHHELD from the commune's own
// audit screen (Log.Read): the owner decided an operator's act on a commune shows in the operator log
// only. The column carries no CHECK in any service, so the fourth value needs no migration.
const KindOperator = "operator"

// Entry is one immutable record of a write.
type Entry struct {
	TenantID tenant.ID
	Actor    Actor
	Action   string // a BUSINESS verb: "accept_petition", never a function name
	Subject  string // a BUSINESS code: "VB-2026-0142", never an internal id
	At       time.Time
	Delta    json.RawMessage // before/after, personal data ALREADY masked
}

func (e Entry) validate() error {
	switch {
	case e.TenantID == "":
		return fmt.Errorf("audit: no commune — an entry that cannot be attributed to a commune is useless")
	case e.Actor.ID == "":
		return fmt.Errorf("audit: no actor — use SystemActor for background work, never empty")
	case e.Action == "":
		return fmt.Errorf("audit: no action")
	case e.Subject == "":
		return fmt.Errorf("audit: no subject — use the business code, not the internal id")
	}
	return nil
}

// Write appends an entry inside an existing transaction.
//
// It takes *store.ScopedTx and not a context or a DB handle ON PURPOSE: the type system then
// makes it impossible to audit outside a transaction. There is no overload that writes
// standalone, because that overload is exactly the bug.
func Write(ctx context.Context, tx *store.ScopedTx, e Entry) error {
	if e.TenantID == "" {
		e.TenantID = tx.TenantID()
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	if err := e.validate(); err != nil {
		return err
	}
	if e.TenantID != tx.TenantID() {
		// Auditing one commune's action inside another commune's transaction is either a bug
		// or an attack. Both must stop here.
		return fmt.Errorf("audit: entry commune %q does not match transaction commune %q",
			e.TenantID, tx.TenantID())
	}

	const stmt = `INSERT INTO audit_log
		(tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := tx.Exec(ctx, stmt,
		string(e.TenantID), e.Actor.ID, e.Actor.Kind, e.Actor.IP,
		e.Action, e.Subject, e.At, []byte(e.Delta))
	if err != nil {
		return fmt.Errorf("audit: write: %w", err)
	}
	return nil
}
