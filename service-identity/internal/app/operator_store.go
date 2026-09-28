package app

import (
	"context"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/store/operatorstore"
)

// OperatorTx is the part of operatorstore.Tx the operator use cases call. Declared here, at the
// point of use (doc.go), so the use cases are tested against a fake that records the ORDER of what
// happened inside one transaction — which is how the tests prove an audit entry shares the
// transaction of the change it describes (rule 6, invariant 3).
//
// *operatorstore.Tx satisfies it as it stands; operatorStoreAdapter below is the only glue.
type OperatorTx interface {
	ByID(ctx context.Context, id string) (domain.OperatorAccount, error)
	ByCode(ctx context.Context, code string) (domain.OperatorAccount, error)
	ByEmail(ctx context.Context, email string) (domain.OperatorAccount, error)
	// The two ForUpdate reads LOCK the account row until the transaction ends. Every credential
	// check starts with one, so parallel attempts on one account run in sequence and the lockout
	// holds under concurrency (operatorstore forUpdate).
	ByEmailForUpdate(ctx context.Context, email string) (domain.OperatorAccount, error)
	ByIDForUpdate(ctx context.Context, id string) (domain.OperatorAccount, error)
	Credentials(ctx context.Context, accountID string) (operatorstore.Credentials, error)

	CreateAccount(ctx context.Context, in operatorstore.NewAccount, now time.Time) (domain.OperatorAccount, error)
	SetPendingTOTP(ctx context.Context, accountID string, sealed []byte, now time.Time) error
	ActivateTOTP(ctx context.Context, accountID, passwordHash string, step int64, provedSealed []byte, now time.Time) error
	RecordTOTPStep(ctx context.Context, accountID string, step int64, now time.Time) (bool, error)
	RegisterFailure(ctx context.Context, accountID string, now time.Time) (bool, time.Time, error)
	ResetFailures(ctx context.Context, accountID string, now time.Time) error
	SetPassword(ctx context.Context, accountID, passwordHash string, mustChange bool, now time.Time) error
	UpgradePasswordHash(ctx context.Context, accountID, oldHash, newHash string, now time.Time) error
	Disable(ctx context.Context, accountID, by, reason string, now time.Time) error
	Enable(ctx context.Context, accountID string, now time.Time) error
	ResetMFA(ctx context.Context, accountID string, now time.Time) error

	CreateSession(ctx context.Context, sid, accountID, ip, userAgent string, now time.Time) (time.Time, error)
	CheckSession(ctx context.Context, sid string, now time.Time) (operatorstore.OperatorSession, error)
	RevokeSession(ctx context.Context, sid, reason string, now time.Time) error
	TouchSession(ctx context.Context, sid string, now time.Time) error

	Grant(ctx context.Context, accountID string, key domain.OperatorPermission, by, reason string, now time.Time) error
	Revoke(ctx context.Context, accountID string, key domain.OperatorPermission, by, reason string, now time.Time) error
	ActivePermissions(ctx context.Context, accountID string) ([]domain.OperatorPermission, error)

	ReplaceRecoveryCodes(ctx context.Context, accountID string, digests [][]byte, now time.Time) (string, error)
	UseRecoveryCode(ctx context.Context, accountID string, digest []byte, now time.Time) (bool, error)

	AppendAudit(ctx context.Context, e domain.OperatorAuditEntry) error
}

// OperatorStore opens transactions and serves the two reads that need none.
type OperatorStore interface {
	InTx(ctx context.Context, fn func(tx OperatorTx) error) error
	ListAccounts(ctx context.Context) ([]domain.OperatorAccount, error)
	ActivePermissions(ctx context.Context, accountID string) ([]domain.OperatorPermission, error)
}

// NewOperatorStore adapts the real store to OperatorStore.
func NewOperatorStore(s *operatorstore.Store) OperatorStore { return operatorStoreAdapter{s: s} }

type operatorStoreAdapter struct{ s *operatorstore.Store }

func (a operatorStoreAdapter) InTx(ctx context.Context, fn func(tx OperatorTx) error) error {
	return a.s.InTx(ctx, func(tx *operatorstore.Tx) error { return fn(tx) })
}

func (a operatorStoreAdapter) ListAccounts(ctx context.Context) ([]domain.OperatorAccount, error) {
	return a.s.ListAccounts(ctx)
}

func (a operatorStoreAdapter) ActivePermissions(ctx context.Context, accountID string) ([]domain.OperatorPermission, error) {
	return a.s.ActivePermissions(ctx, accountID)
}
