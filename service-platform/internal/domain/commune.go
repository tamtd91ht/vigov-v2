package domain

import (
	"errors"
	"time"
)

// Commune is a commune as the OPERATOR console sees it: registry metadata only — id, name, province,
// state, hosts (ADR 0003; ADR 0048 §30/09 #5: "the list carries only name/domain/status"). Adding a
// field that is not registry metadata takes the cross-commune list out of the exception that lets it
// go unaudited — ask again before adding one.
type Commune struct {
	ID       string
	Name     string
	Province string // display string, tenant.tinh_thanh
	Active   bool
	// Domains are the commune's hosts, the PRIMARY ONE FIRST, then alphabetical. Empty is a commune
	// with no host yet — a configuration state, not an error.
	Domains   []string
	CreatedAt time.Time
}

// CommuneMiniApp is one mini_app row bound to a commune (che_do 'rieng'), soft-deleted rows excluded.
type CommuneMiniApp struct {
	AppID     string
	Mode      CheDoMiniApp
	Active    bool
	CreatedAt time.Time
	// CreatedBy is the business code that entered the row (`VH-…`, `CB-…`, or the legacy
	// `van-hanh:<user>` of the Jenkins stage).
	CreatedBy string
}

// Province is one row of the tinh_thanh catalogue.
type Province struct {
	ID   string
	Name string
}

// OperatorActor is who performs an operator write: the business code and the client address, both
// written into the target commune's audit_log (rule 6 invariants 2, 8).
type OperatorActor struct {
	Code string // `VH-00001` — never the internal operator id
	IP   string
}

// AuditKindOperator is audit_log.actor_kind for an operator write. A FOURTH value beside staff,
// citizen and system (migration 0001's comment lists three; the column carries no CHECK, so no
// migration is needed). The `VH-` prefix of the code says the same thing to a person; the kind says
// it to a query.
const AuditKindOperator = "operator"

// ErrNoActor — an operator write with no business code. Refused: an empty "who" has no fallback to
// the internal id (rule 6, invariant 8).
var ErrNoActor = errors.New("operator: thiếu mã nghiệp vụ của người vận hành")

// Validate refuses an actor with no code.
func (a OperatorActor) Validate() error {
	if a.Code == "" {
		return ErrNoActor
	}
	return nil
}
