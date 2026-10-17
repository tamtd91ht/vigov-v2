package app

// A COMMUNE'S SMS ACCOUNTS (migration 0032; ADR 0099 #2, #3) — the administration behind
// /api/v1/sms-accounts (admin.sms): add, edit, retire, check an eSMS account, and load its CSKH
// templates. Sending through one is the campaign's (block D-5), not here.
//
// THE COMMUNE IS THE CONTEXT'S, always (rule 1, invariant 4): the routes resolve it from Host; nothing
// here takes a commune as an argument, and nothing reads another commune's account.
//
// THE ApiKey AND SecretKey ARE WRITE-ONLY. Sealed under the COMMUNE's data key (ADR 0009; the mail
// password's and the commune bot's discipline), each with additional data naming the SECRET, the commune
// and the ROW (0032 "AAD OWED BY GO"). No return value carries a key; no audit delta carries one; eSMS
// failures are CLASSES, never eSMS's text (internal/esms).
//
// THE ORDER OF EVERY ACT is the commune bot's (zalo_commune_bot.go): validate the request alone → the
// eSMS call OUTSIDE any transaction → seal OUTSIDE the transaction (a first seal may create the commune's
// DEK in its own short transaction) → ONE transaction with the change and its audit entry. Keys eSMS
// does not accept are never saved.
//
// "KIỂM TRA KẾT NỐI" IS TWO READ-ONLY CALLS: get-balance (ADR 0099 #2 — are the keys good, and the
// balance), then the template list for the Brandname. The second is what makes 0032's
// 'brandname-chua-kich-hoat' class reachable: get-balance does not look at the Brandname at all, so an
// account whose Brandname eSMS has not activated would otherwise check green and fail on the first send.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/esms"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// smsAPIKeyAAD / smsSecretKeyAAD — 0032's bindings, verbatim: bound to the ROW (bytes copied from a
// retired account onto a live one do not open), to the commune (they do not open in another), and to
// the SECRET (an ApiKey does not open as a SecretKey).
func smsAPIKeyAAD(ctx context.Context, rowID string) []byte {
	return []byte("sms_account/api_key/" + string(tenant.MustFrom(ctx)) + "/" + rowID)
}

func smsSecretKeyAAD(ctx context.Context, rowID string) []byte {
	return []byte("sms_account/secret_key/" + string(tenant.MustFrom(ctx)) + "/" + rowID)
}

// SmsGateway is what this use case asks eSMS. *esms.Client satisfies it; tests fake it.
type SmsGateway interface {
	GetBalance(ctx context.Context, creds esms.Credentials) (string, esms.Outcome)
	ListCSKHTemplates(ctx context.Context, creds esms.Credentials, brandname string) ([]esms.Template, esms.Outcome)
}

// smsCheckClass maps an eSMS outcome onto 0032's check vocabulary. Every refusal that is not about the
// keys or the Brandname — a template mismatch, a 99, an account locked, an IP not whitelisted — is
// bi-tu-choi: what eSMS said exactly is not kept (internal/esms).
func smsCheckClass(o esms.Outcome) string {
	switch o {
	case esms.OutcomeOK:
		return domain.SmsCheckOK
	case esms.OutcomeKeyRejected:
		return domain.SmsCheckKeyRejected
	case esms.OutcomeBrandnameInactive:
		return domain.SmsCheckBrandnameInactive
	case esms.OutcomeUnavailable:
		return domain.SmsCheckUnavailable
	case esms.OutcomeMalformedResponse:
		return domain.SmsCheckMalformed
	}
	return domain.SmsCheckRejected
}

// ErrSmsAccountNotFound — no LIVE account with that id in this commune (another commune's id reads the
// same: rule 1 — the reply never says it exists elsewhere).
var ErrSmsAccountNotFound = errors.New("sms_account: no such live account in the commune")

// SmsProviderError — eSMS did not accept the keys / Brandname, or could not be asked. Class only (a
// domain.SmsCheck* value). Nothing was written.
type SmsProviderError struct{ Class string }

func (e *SmsProviderError) Error() string { return "sms_account: eSMS check failed: " + e.Class }

// CreateSmsAccountInput is one POST. Both keys are required.
type CreateSmsAccountInput struct {
	Label     string
	APIKey    secret.Secret
	SecretKey secret.Secret
	Brandname string
	IsDefault bool
}

// UpdateSmsAccountInput is one PATCH. nil / empty = keep: a key left empty keeps the stored one.
type UpdateSmsAccountInput struct {
	Label     *string
	Brandname *string
	APIKey    secret.Secret
	SecretKey secret.Secret
	IsDefault *bool
}

// SmsCheckOutput is POST check. Balance only when Result is thanh-cong.
type SmsCheckOutput struct {
	Result    string
	CheckedAt time.Time
	Balance   string
}

// SmsTemplateNetwork is one network's registration of a template text.
type SmsTemplateNetwork struct {
	NetworkID          int
	ProviderTemplateID string
}

// SmsTemplateGroup is one template TEXT as the screen shows it: the rows eSMS lists per network, grouped
// by content (0032 VENDOR CHOICES: "the picker groups rows by content").
type SmsTemplateGroup struct {
	// TemplateRowID is the sms_template.id of the group's FIRST row (by network, then TempId) — the
	// reference a campaign names the text by (notice_campaign.template_ref). Any row of the group names the
	// same text; the first is chosen so the reference is stable across reads.
	TemplateRowID string
	Content       string
	Params        []domain.SmsTemplateParam
	Networks      []SmsTemplateNetwork
	SyncedAt      time.Time
}

// SmsTemplateSyncOutput is POST templates/sync.
type SmsTemplateSyncOutput struct {
	Added, Updated, Vanished int
	// Skipped: templates eSMS listed that 0032 cannot store (a TempId of another shape, content over 1000
	// characters, an unreadable placeholder). Counted, never written, never quoted.
	Skipped   int
	Templates []SmsTemplateGroup
}

// SmsAccounts owns the acts on a commune's SMS accounts.
type SmsAccounts struct {
	db        *store.DB
	accounts  *docstore.SmsAccountStore
	templates *docstore.SmsTemplateStore
	envelope  *crypto.Envelope // the COMMUNE envelope; nil without SECRET_ENCRYPTION_KEYS
	gateway   SmsGateway
	log       *slog.Logger // the SECURITY LOG (skills/security-logging) — never the audit trail

	newID func() (string, error)
	now   func() time.Time
}

// NewSmsAccounts panics on a missing dependency — at construction, not on the first request.
func NewSmsAccounts(db *store.DB, accounts *docstore.SmsAccountStore, templates *docstore.SmsTemplateStore,
	envelope *crypto.Envelope, gateway SmsGateway, log *slog.Logger) *SmsAccounts {

	if db == nil || accounts == nil || templates == nil || gateway == nil {
		panic("app: NewSmsAccounts thiếu phụ thuộc")
	}
	if log == nil {
		log = slog.Default()
	}
	return &SmsAccounts{db: db, accounts: accounts, templates: templates, envelope: envelope, gateway: gateway,
		log: log, newID: ulid.Moi, now: time.Now}
}

func (uc *SmsAccounts) clock() time.Time { return uc.now().UTC().Truncate(time.Microsecond) }

// The security-log events of the SMS accounts (TCVN 14423 §6.8.2.1, "configuration changed"): an SMS
// account is a credential that spends the commune's money and speaks in its name.
const (
	SecEventSmsAccountCreated = "sms_account.created"
	SecEventSmsAccountUpdated = "sms_account.updated"
	SecEventSmsAccountRetired = "sms_account.retired"
	SecEventSmsAccountChecked = "sms_account.checked"
)

// securityEvent writes one security-log line: event, outcome, commune, the actor's BUSINESS CODE, the
// client IP and the account's row id. NEVER a key or anything eSMS said.
func (uc *SmsAccounts) securityEvent(ctx context.Context, event, outcome string, actor audit.Actor, accountID string) {
	uc.log.InfoContext(ctx, "NHẬT KÝ AN NINH: thay đổi cấu hình tài khoản SMS của xã",
		"event", event, "outcome", outcome, "xa", string(tenant.MustFrom(ctx)), "actor", actor.ID, "ip", actor.IP,
		"sms_account_id", accountID, "at", uc.clock())
}

// ListAccounts reads the commune's live accounts — no sealed column.
func (uc *SmsAccounts) ListAccounts(ctx context.Context) ([]domain.SmsAccount, error) {
	return uc.accounts.ListLive(ctx)
}

// verify asks eSMS whether creds and brandname work: get-balance, then the Brandname's template list (see
// the file comment). Returns the balance and thanh-cong, or "" and the failing class.
func (uc *SmsAccounts) verify(ctx context.Context, creds esms.Credentials, brandname string) (string, string) {
	balance, o := uc.gateway.GetBalance(ctx, creds)
	if o != esms.OutcomeOK {
		return "", smsCheckClass(o)
	}
	if _, o := uc.gateway.ListCSKHTemplates(ctx, creds, brandname); o != esms.OutcomeOK {
		return "", smsCheckClass(o)
	}
	return balance, domain.SmsCheckOK
}

// CreateAccount adds an account: eSMS is asked FIRST, with the given keys and Brandname; keys it refuses are
// never stored (*SmsProviderError). is_default moves the default mark to the new account in the same
// transaction.
func (uc *SmsAccounts) CreateAccount(ctx context.Context, in CreateSmsAccountInput, actor audit.Actor) (domain.SmsAccount, error) {
	if actor.ID == "" {
		return domain.SmsAccount{}, ErrNoActor
	}
	label, err := domain.NormalizeSmsAccountLabel(in.Label)
	if err != nil {
		return domain.SmsAccount{}, err
	}
	brandname, err := domain.NormalizeSmsBrandname(in.Brandname)
	if err != nil {
		return domain.SmsAccount{}, err
	}
	if in.APIKey.Rong() || in.SecretKey.Rong() {
		return domain.SmsAccount{}, domain.ErrSmsKeysRequired
	}
	if !domain.ValidSmsKey(in.APIKey.Lo()) || !domain.ValidSmsKey(in.SecretKey.Lo()) {
		return domain.SmsAccount{}, domain.ErrSmsKeyShape
	}
	if uc.envelope == nil {
		return domain.SmsAccount{}, crypto.ErrNotConfigured
	}

	// 1. eSMS, outside any transaction.
	balance, class := uc.verify(ctx, esms.Credentials{APIKey: in.APIKey, SecretKey: in.SecretKey}, brandname)
	if class != domain.SmsCheckOK {
		uc.securityEvent(ctx, SecEventSmsAccountCreated, "refused:"+class, actor, "")
		return domain.SmsAccount{}, &SmsProviderError{Class: class}
	}

	// 2. The row id decides the additional data, so it is chosen before sealing.
	id, err := uc.newID()
	if err != nil {
		return domain.SmsAccount{}, fmt.Errorf("sms_account: new id: %w", err)
	}
	apiSealed, err := uc.envelope.Seal(ctx, in.APIKey, smsAPIKeyAAD(ctx, id))
	if err != nil {
		return domain.SmsAccount{}, fmt.Errorf("sms_account: seal the ApiKey: %w", err)
	}
	secretSealed, err := uc.envelope.Seal(ctx, in.SecretKey, smsSecretKeyAAD(ctx, id))
	if err != nil {
		return domain.SmsAccount{}, fmt.Errorf("sms_account: seal the SecretKey: %w", err)
	}

	// 3. One transaction: the row and its entry.
	at := uc.clock()
	a := domain.SmsAccount{ID: id, Provider: domain.SmsProviderEsms, Label: label, Brandname: brandname,
		IsDefault: in.IsDefault, LastCheckAt: at, LastCheckResult: domain.SmsCheckOK, LastBalance: balance,
		SetAt: at, SetBy: actor.ID, CreatedAt: at}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if a.IsDefault {
			if err := uc.accounts.ClearDefault(ctx, tx, at); err != nil {
				return err
			}
		}
		if err := uc.accounts.InsertLive(ctx, tx, a, apiSealed, secretSealed); err != nil {
			return err
		}
		return writeSmsAccountAudit(ctx, tx, actor, domain.ActionCreateSmsAccount, a.Subject(), at, nil, smsAccountDelta(a), nil)
	})
	if err != nil {
		return domain.SmsAccount{}, fmt.Errorf("sms_account: save: %w", err)
	}
	uc.securityEvent(ctx, SecEventSmsAccountCreated, "created", actor, a.ID)
	return a, nil
}

// UpdateAccount edits a live account. Keys left empty are kept. New keys or a new Brandname are verified with
// eSMS FIRST (the resulting pair — a kept key is opened and presented with a new one) and refused, with
// nothing written, when eSMS does not accept them. Nothing changed = nothing written, no entry.
func (uc *SmsAccounts) UpdateAccount(ctx context.Context, id string, in UpdateSmsAccountInput, actor audit.Actor) (domain.SmsAccount, error) {
	if actor.ID == "" {
		return domain.SmsAccount{}, ErrNoActor
	}
	seen, found, err := uc.accounts.Sealed(ctx, id)
	if err != nil {
		return domain.SmsAccount{}, err
	}
	if !found {
		return domain.SmsAccount{}, ErrSmsAccountNotFound
	}
	label, brandname := seen.Account.Label, seen.Account.Brandname
	if in.Label != nil {
		if label, err = domain.NormalizeSmsAccountLabel(*in.Label); err != nil {
			return domain.SmsAccount{}, err
		}
	}
	if in.Brandname != nil {
		if brandname, err = domain.NormalizeSmsBrandname(*in.Brandname); err != nil {
			return domain.SmsAccount{}, err
		}
	}
	for _, k := range []secret.Secret{in.APIKey, in.SecretKey} {
		if !k.Rong() && !domain.ValidSmsKey(k.Lo()) {
			return domain.SmsAccount{}, domain.ErrSmsKeyShape
		}
	}
	newAPI, newSecret := !in.APIKey.Rong(), !in.SecretKey.Rong()
	credentials := newAPI || newSecret || brandname != seen.Account.Brandname
	defaultMoves := in.IsDefault != nil && *in.IsDefault != seen.Account.IsDefault
	if !credentials && !defaultMoves && label == seen.Account.Label {
		return seen.Account, nil // nothing moved: no UPDATE, no entry
	}

	apiSealed, secretSealed := seen.APIKeySealed, seen.SecretKeySealed
	var balance string
	if credentials {
		if uc.envelope == nil {
			return domain.SmsAccount{}, crypto.ErrNotConfigured
		}
		creds, err := uc.resultingCredentials(ctx, seen, in)
		if err != nil {
			return domain.SmsAccount{}, err
		}
		var class string
		balance, class = uc.verify(ctx, creds, brandname)
		clear(creds.APIKey)
		clear(creds.SecretKey)
		if class != domain.SmsCheckOK {
			uc.securityEvent(ctx, SecEventSmsAccountUpdated, "refused:"+class, actor, id)
			return domain.SmsAccount{}, &SmsProviderError{Class: class}
		}
		if newAPI {
			if apiSealed, err = uc.envelope.Seal(ctx, in.APIKey, smsAPIKeyAAD(ctx, id)); err != nil {
				return domain.SmsAccount{}, fmt.Errorf("sms_account: seal the ApiKey: %w", err)
			}
		}
		if newSecret {
			if secretSealed, err = uc.envelope.Seal(ctx, in.SecretKey, smsSecretKeyAAD(ctx, id)); err != nil {
				return domain.SmsAccount{}, fmt.Errorf("sms_account: seal the SecretKey: %w", err)
			}
		}
	}

	at := uc.clock()
	var after domain.SmsAccount
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.accounts.LockLive(ctx, tx, id)
		if err != nil {
			return err
		}
		// What eSMS verified is what was seen: keys or Brandname saved by someone else meanwhile would be
		// overwritten by a pair nobody checked together.
		if !found || !cur.Account.SetAt.Equal(seen.Account.SetAt) {
			return docstore.ErrSmsAccountChanged
		}
		after = cur.Account
		after.Label = label
		if credentials {
			setAt := at
			if setAt.Equal(cur.Account.SetAt) {
				setAt = setAt.Add(time.Microsecond) // 0032's guard needs a NEW set_at
			}
			if err := uc.accounts.UpdateCredentials(ctx, tx, id, label, brandname, apiSealed, secretSealed, actor.ID,
				domain.SmsCheckOK, balance, setAt); err != nil {
				return err
			}
			after.Brandname, after.SetAt, after.SetBy = brandname, setAt, actor.ID
			after.LastCheckAt, after.LastCheckResult, after.LastBalance = setAt, domain.SmsCheckOK, balance
		} else if label != cur.Account.Label {
			if err := uc.accounts.UpdateLabel(ctx, tx, id, label, at); err != nil {
				return err
			}
		}
		if in.IsDefault != nil && *in.IsDefault != cur.Account.IsDefault {
			move := uc.accounts.UnsetDefault
			if *in.IsDefault {
				move = uc.accounts.SetDefault
			}
			if err := move(ctx, tx, id, at); err != nil {
				return err
			}
			after.IsDefault = *in.IsDefault
		}
		return writeSmsAccountAudit(ctx, tx, actor, domain.ActionUpdateSmsAccount, after.Subject(), at,
			smsAccountDelta(cur.Account), smsAccountDelta(after),
			map[string]any{"api_key_replaced": newAPI, "secret_key_replaced": newSecret})
	})
	if err != nil {
		return domain.SmsAccount{}, fmt.Errorf("sms_account: save: %w", err)
	}
	outcome := "updated"
	if newAPI || newSecret {
		outcome = "keys_replaced"
	}
	uc.securityEvent(ctx, SecEventSmsAccountUpdated, outcome, actor, id)
	return after, nil
}

// resultingCredentials is the pair an update would leave: the new key where one was given, the stored
// one opened where not. The caller clears both.
func (uc *SmsAccounts) resultingCredentials(ctx context.Context, seen docstore.SmsAccountSecrets,
	in UpdateSmsAccountInput) (esms.Credentials, error) {

	creds := esms.Credentials{APIKey: append(secret.Secret(nil), in.APIKey...),
		SecretKey: append(secret.Secret(nil), in.SecretKey...)}
	var err error
	if creds.APIKey.Rong() {
		if creds.APIKey, err = uc.envelope.Open(ctx, seen.APIKeySealed, smsAPIKeyAAD(ctx, seen.Account.ID)); err != nil {
			clear(creds.SecretKey)
			return esms.Credentials{}, fmt.Errorf("sms_account: open the ApiKey: %w", err)
		}
	}
	if creds.SecretKey.Rong() {
		if creds.SecretKey, err = uc.envelope.Open(ctx, seen.SecretKeySealed, smsSecretKeyAAD(ctx, seen.Account.ID)); err != nil {
			clear(creds.APIKey)
			return esms.Credentials{}, fmt.Errorf("sms_account: open the SecretKey: %w", err)
		}
	}
	return creds, nil
}

// RetireAccount retires ("ngừng dùng") a live account with the administrator's reason — rule 7's soft delete.
// Retiring the default leaves the commune WITHOUT a default: no other account is chosen in its place.
func (uc *SmsAccounts) RetireAccount(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if actor.ID == "" {
		return ErrNoActor
	}
	reason, err := domain.NormalizeSmsRetireReason(rawReason)
	if err != nil {
		return err
	}
	at := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.accounts.LockLive(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return ErrSmsAccountNotFound
		}
		if err := uc.accounts.Retire(ctx, tx, id, actor.ID, reason, at); err != nil {
			return err
		}
		return writeSmsAccountAudit(ctx, tx, actor, domain.ActionRetireSmsAccount, cur.Account.Subject(), at,
			smsAccountDelta(cur.Account), map[string]any{"retired_at": at, "retired_by": actor.ID, "retire_reason": reason},
			map[string]any{"was_default": cur.Account.IsDefault})
	})
	if err != nil {
		return fmt.Errorf("sms_account: retire: %w", err)
	}
	uc.securityEvent(ctx, SecEventSmsAccountRetired, "retired", actor, id)
	return nil
}

// Check — "Kiểm tra kết nối" with the stored keys; the class (and the balance on success) recorded with
// its entry in one transaction. A failing check is a RESULT, not an error.
func (uc *SmsAccounts) Check(ctx context.Context, id string, actor audit.Actor) (SmsCheckOutput, error) {
	if actor.ID == "" {
		return SmsCheckOutput{}, ErrNoActor
	}
	s, creds, err := uc.openLive(ctx, id)
	if err != nil {
		return SmsCheckOutput{}, err
	}
	balance, result := uc.verify(ctx, creds, s.Account.Brandname)
	clear(creds.APIKey)
	clear(creds.SecretKey)
	at := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.accounts.LockLive(ctx, tx, id)
		if err != nil {
			return err
		}
		// Keys replaced meanwhile: this result describes the old ones and must not be recorded on the new.
		if !found || !cur.Account.SetAt.Equal(s.Account.SetAt) {
			return docstore.ErrSmsAccountChanged
		}
		if err := uc.accounts.RecordCheck(ctx, tx, id, result, balance, at); err != nil {
			return err
		}
		facts := map[string]any{"result": result, "checked_at": at}
		if balance != "" {
			facts["balance"] = balance
		}
		return writeSmsAccountAudit(ctx, tx, actor, domain.ActionCheckSmsAccount, s.Account.Subject(), at, nil, facts, nil)
	})
	if err != nil {
		return SmsCheckOutput{}, fmt.Errorf("sms_account: record the check: %w", err)
	}
	uc.securityEvent(ctx, SecEventSmsAccountChecked, result, actor, id)
	return SmsCheckOutput{Result: result, CheckedAt: at, Balance: balance}, nil
}

// openLive reads a live account and opens both keys. ErrSmsAccountNotFound when there is none here.
// The caller clears both keys.
func (uc *SmsAccounts) openLive(ctx context.Context, id string) (docstore.SmsAccountSecrets, esms.Credentials, error) {
	s, found, err := uc.accounts.Sealed(ctx, id)
	if err != nil {
		return docstore.SmsAccountSecrets{}, esms.Credentials{}, err
	}
	if !found {
		return docstore.SmsAccountSecrets{}, esms.Credentials{}, ErrSmsAccountNotFound
	}
	if uc.envelope == nil {
		return docstore.SmsAccountSecrets{}, esms.Credentials{}, crypto.ErrNotConfigured
	}
	api, err := uc.envelope.Open(ctx, s.APIKeySealed, smsAPIKeyAAD(ctx, id))
	if err != nil {
		return docstore.SmsAccountSecrets{}, esms.Credentials{}, fmt.Errorf("sms_account: open the ApiKey: %w", err)
	}
	sec, err := uc.envelope.Open(ctx, s.SecretKeySealed, smsSecretKeyAAD(ctx, id))
	if err != nil {
		clear(api)
		return docstore.SmsAccountSecrets{}, esms.Credentials{}, fmt.Errorf("sms_account: open the SecretKey: %w", err)
	}
	return s, esms.Credentials{APIKey: api, SecretKey: sec}, nil
}

// ---- the campaign's two reads (ADR 0099 #8, block D-5) -----------------------------------------------------

var (
	// ErrSmsTemplateMissing — no template row with that id under that account of this commune (404).
	ErrSmsTemplateMissing = errors.New("sms_template: no such template under the account")
	// ErrSmsTemplateNotSendable — the row exists but eSMS no longer lists it (vanished): sending its text
	// would come back "sai mẫu" (409).
	ErrSmsTemplateNotSendable = errors.New("sms_template: eSMS no longer lists this template")
)

// TemplateForSend reads a LIVE account of this commune and one of its templates that eSMS still lists —
// what a campaign is composed from. No key is opened.
//
//	account not live here        ErrSmsAccountNotFound
//	no such template row         ErrSmsTemplateMissing
//	template vanished at eSMS    ErrSmsTemplateNotSendable
func (uc *SmsAccounts) TemplateForSend(ctx context.Context, accountID, templateRowID string) (domain.SmsAccount, domain.SmsTemplate, error) {
	a, found, err := uc.accounts.Live(ctx, accountID)
	if err != nil {
		return domain.SmsAccount{}, domain.SmsTemplate{}, err
	}
	if !found {
		return domain.SmsAccount{}, domain.SmsTemplate{}, ErrSmsAccountNotFound
	}
	t, found, err := uc.templates.TemplateOfAccount(ctx, accountID, templateRowID)
	if err != nil {
		return domain.SmsAccount{}, domain.SmsTemplate{}, err
	}
	if !found {
		return domain.SmsAccount{}, domain.SmsTemplate{}, ErrSmsTemplateMissing
	}
	if !t.VanishedAt.IsZero() {
		return domain.SmsAccount{}, domain.SmsTemplate{}, ErrSmsTemplateNotSendable
	}
	return a, t, nil
}

// OpenForSend opens both keys of a LIVE account of this commune for a send or a price estimate. THE CALLER
// CLEARS BOTH (clear(creds.APIKey), clear(creds.SecretKey)) as soon as eSMS answers, and never logs,
// stores or returns them.
//
//	account not live here       ErrSmsAccountNotFound (a retired account sends nothing)
//	no SECRET_ENCRYPTION_KEYS   crypto.ErrNotConfigured
func (uc *SmsAccounts) OpenForSend(ctx context.Context, accountID string) (domain.SmsAccount, esms.Credentials, error) {
	s, creds, err := uc.openLive(ctx, accountID)
	if err != nil {
		return domain.SmsAccount{}, esms.Credentials{}, err
	}
	return s.Account, creds, nil
}

// ListTemplates reads the templates eSMS still lists for a live account of this commune, one entry per
// TEXT with the networks it is registered on.
func (uc *SmsAccounts) ListTemplates(ctx context.Context, accountID string) ([]SmsTemplateGroup, error) {
	_, found, err := uc.accounts.Live(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrSmsAccountNotFound
	}
	rows, err := uc.templates.ListCurrentByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return groupSmsTemplates(rows), nil
}

// groupSmsTemplates folds rows into one group per content, first appearance first.
func groupSmsTemplates(rows []domain.SmsTemplate) []SmsTemplateGroup {
	out := []SmsTemplateGroup{}
	index := map[string]int{}
	for _, t := range rows {
		i, ok := index[t.Content]
		if !ok {
			i = len(out)
			index[t.Content] = i
			params := t.Params
			if params == nil {
				params = []domain.SmsTemplateParam{}
			}
			out = append(out, SmsTemplateGroup{TemplateRowID: t.ID, Content: t.Content, Params: params})
		}
		g := &out[i]
		g.Networks = append(g.Networks, SmsTemplateNetwork{NetworkID: t.NetworkID, ProviderTemplateID: t.ProviderTemplateID})
		if t.SyncedAt.After(g.SyncedAt) {
			g.SyncedAt = t.SyncedAt
		}
	}
	return out
}

// smsTemplateKey is eSMS's identity of a template under one account (0032's unique key, less the
// commune and the account).
type smsTemplateKey struct {
	tempID  string
	network int
}

// SyncTemplates — "Nạp mẫu": eSMS's CSKH template list for the account's Brandname, written as one
// upsert per (TempId, network); rows eSMS no longer lists are marked vanished (kept — campaigns name
// them). eSMS refusing or unreachable = nothing written (*SmsProviderError). One entry for the sync.
func (uc *SmsAccounts) SyncTemplates(ctx context.Context, accountID string, actor audit.Actor) (SmsTemplateSyncOutput, error) {
	if actor.ID == "" {
		return SmsTemplateSyncOutput{}, ErrNoActor
	}
	s, creds, err := uc.openLive(ctx, accountID)
	if err != nil {
		return SmsTemplateSyncOutput{}, err
	}
	listed, o := uc.gateway.ListCSKHTemplates(ctx, creds, s.Account.Brandname)
	clear(creds.APIKey)
	clear(creds.SecretKey)
	if o != esms.OutcomeOK {
		return SmsTemplateSyncOutput{}, &SmsProviderError{Class: smsCheckClass(o)}
	}

	var out SmsTemplateSyncOutput
	incoming := map[smsTemplateKey]domain.SmsTemplate{}
	order := []smsTemplateKey{}
	for _, t := range listed {
		if domain.ValidateSmsTemplate(t.ID, t.NetworkID, t.Content) != nil {
			out.Skipped++
			continue
		}
		params, err := domain.ParseSmsTemplateParams(t.Content)
		if err != nil {
			out.Skipped++
			continue
		}
		k := smsTemplateKey{t.ID, t.NetworkID}
		if _, dup := incoming[k]; !dup {
			order = append(order, k)
		}
		incoming[k] = domain.SmsTemplate{AccountID: accountID, ProviderTemplateID: t.ID, NetworkID: t.NetworkID,
			Content: t.Content, Params: params}
	}

	at := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		out.Added, out.Updated, out.Vanished = 0, 0, 0
		cur, found, err := uc.accounts.LockLive(ctx, tx, accountID)
		if err != nil {
			return err
		}
		// The Brandname asked about must still be the account's.
		if !found || !cur.Account.SetAt.Equal(s.Account.SetAt) {
			return docstore.ErrSmsAccountChanged
		}
		existing, err := uc.templates.LockByAccount(ctx, tx, accountID)
		if err != nil {
			return err
		}
		have := map[smsTemplateKey]domain.SmsTemplate{}
		for _, t := range existing {
			have[smsTemplateKey{t.ProviderTemplateID, t.NetworkID}] = t
		}
		for _, k := range order {
			t := incoming[k]
			if old, ok := have[k]; ok {
				t.ID = old.ID
				out.Updated++
			} else {
				if t.ID, err = uc.newID(); err != nil {
					return fmt.Errorf("sms_template: new id: %w", err)
				}
				out.Added++
			}
			if err := uc.templates.UpsertFromSync(ctx, tx, t, at); err != nil {
				return err
			}
		}
		for _, t := range existing {
			k := smsTemplateKey{t.ProviderTemplateID, t.NetworkID}
			if _, still := incoming[k]; still || !t.VanishedAt.IsZero() {
				continue
			}
			if err := uc.templates.MarkVanished(ctx, tx, t.ID, at); err != nil {
				return err
			}
			out.Vanished++
		}
		return writeSmsAccountAudit(ctx, tx, actor, domain.ActionSyncSmsTemplates, s.Account.Subject(), at, nil,
			map[string]any{"added": out.Added, "updated": out.Updated, "vanished": out.Vanished, "skipped": out.Skipped,
				"synced_at": at}, nil)
	})
	if err != nil {
		return SmsTemplateSyncOutput{}, fmt.Errorf("sms_template: sync: %w", err)
	}
	groups, err := uc.ListTemplates(ctx, accountID)
	if err != nil {
		return SmsTemplateSyncOutput{}, err
	}
	out.Templates = groups
	return out, nil
}

// smsAccountDelta is the audit view of an account — NEVER a sealed column (0032: "the audit trail never
// carries a *_sealed column"). Nothing here is personal data: a Brandname is an organisation's name.
func smsAccountDelta(a domain.SmsAccount) map[string]any {
	return map[string]any{
		"provider": a.Provider, "label": a.Label, "brandname": a.Brandname, "is_default": a.IsDefault,
		"set_at": a.SetAt, "set_by": a.SetBy,
	}
}

func writeSmsAccountAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action, subject string,
	at time.Time, before, after any, facts map[string]any) error {

	m := map[string]any{"truoc": before, "sau": after}
	for k, v := range facts {
		m[k] = v
	}
	delta, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("sms_account: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: subject, At: at, Delta: delta})
}
