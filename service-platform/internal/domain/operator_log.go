package domain

import "time"

// OperatorLogEntry is one row of the operator log screen (ADR 0073 #2): an operator's act on a
// commune, from this service's audit_log (actor_kind = AuditKindOperator), or a change to
// platform-wide configuration, from platform_audit_log. Read-only; nothing here is ever written back.
//
// METADATA ONLY. Every field comes from a trail entry an operator write path composed — registry
// fields, App IDs, version ids, limits — never a citizen's data and never a secret. Reason is the text
// the operator typed, shown as written.
type OperatorLogEntry struct {
	At        time.Time
	ActorCode string // `VH-…`, or 'system' for a migration's platform-wide change
	Action    string
	// CommuneID / CommuneName are empty for a platform-wide entry (no commune by definition).
	// CommuneName is the commune's CURRENT name, not the name at the time of the act.
	CommuneID   string
	CommuneName string
	Subject     string
	// Before / After are raw JSON objects (nil = absent). For an audit_log entry whose delta has no
	// truoc/sau pair (a creation, an attachment), After is the whole delta minus the reason.
	Before, After []byte
	Reason        string
}

// SecretForward is what the platform records after identity ACCEPTED a Mini App secret act it
// forwarded (ADR 0073 §Hệ quả; ADR 0070 bổ sung #7). Never the secret: an App ID and version ids.
type SecretForward struct {
	Retired bool   // false = a secret was set; true = one was retired
	AppID   string // a public Zalo identifier
	Version string // the version set, or the version retired
	Reason  string
	// Automatic is true for the retirement that follows an App ID change or detach (ADR 0070 #4).
	Automatic bool
}
