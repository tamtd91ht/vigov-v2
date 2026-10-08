package app

// The commune's own mail server — docs/ui-ux/14-cau-hinh.md §10 and §12.6.
//
// THE PASSWORD IS WRITE-ONLY AND SEALED AT REST (ADR 0009, and the one place v2 deliberately
// differs from ../vigov-require, which stores it in the clear). Three rules, all from the
// requirement repository (docs/spec/07-viec-nen-va-thong-bao.md:95-100, email_config.py):
//
//  1. No read ever returns it — MailSettingsView says only whether one is set.
//  2. A blank password on save means KEEP the stored one…
//  3. …UNLESS host, port or account changed: then a new password is required, and the save is
//     refused without one (domain.MailDestinationChanged says why).
//
// WITHOUT A KEK, NOTHING HERE WRITES OR SENDS. A process with no SECRET_ENCRYPTION_KEYS holds a nil
// *crypto.Envelope; Save and SendTestMessage then refuse with crypto.ErrNotConfigured BEFORE any
// statement runs — including a save that carries no new password, because the password it would
// keep could not be opened by this process either, and a screen that saves and then cannot send is
// worse than one that says why it cannot save. Reading still works: it involves no secret.
//
// SEAL FIRST, THEN THE TRANSACTION. Envelope.Seal may create the commune's data key through
// crypto.DEKStore, which opens its own short transaction (store/data_encryption_key.go). Sealing
// inside this transaction would hold a row lock while waiting for a second connection.
//
// NEVER LOGGED, NEVER AUDITED: the password, the sealed bytes, and the SMTP exchange. The audit delta
// records `password_changed: true|false` and nothing about the value.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/mail"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// MailSettingsRepo is the store, declared at the point of use. The two writes take the
// transaction, so there is no signature that writes the row outside the one its entry is in.
type MailSettingsRepo interface {
	Get(ctx context.Context) (domain.MailSettings, error)
	Account(ctx context.Context) (domain.MailSettings, []byte, error)
	ForUpdate(ctx context.Context, tx *store.ScopedTx) (domain.MailSettings, []byte, bool, error)
	Upsert(ctx context.Context, tx *store.ScopedTx, m domain.MailSettings, sealed []byte, by string) error
	RecordTest(ctx context.Context, tx *store.ScopedTx, r domain.MailTestResult) error
	ClearTest(ctx context.Context, tx *store.ScopedTx) error
}

// MailSender is the SMTP adapter (internal/mail), declared at the point of use so a test can stand
// in for the network.
type MailSender interface {
	Send(ctx context.Context, a mail.Account, m mail.Message) error
}

// The business verbs written into the trail — VALUES (ADR 0011), read by an inspection.
const (
	ActionSaveMailSettings = "luu_cau_hinh_may_chu_thu"
	ActionSendTestMail     = "gui_thu_thu_may_chu_thu"
	// ActionRecordTestMailResult — the result of a test send, written onto the row (0019) with this entry.
	ActionRecordTestMailResult = "ghi_ket_qua_thu_thu_may_chu_thu"
)

// The fixed test message. NO CITIZEN DATA, NO COMMUNE DATA: it proves the configuration works and
// says nothing else. Wording after ../vigov-require's email_config.send_test.
const (
	testMailSubject = "Thư thử cấu hình máy chủ thư"
	testMailBody    = "Đây là thư thử do hệ thống gửi.\n\n" +
		"Nhận được thư này nghĩa là cấu hình máy chủ thư của xã đã đúng, và thông báo nội bộ " +
		"sẽ gửi được tới hộp thư của cán bộ.\n"
)

var (
	// ErrMailPasswordRequired — nothing is stored yet (first save), so there is nothing to keep.
	ErrMailPasswordRequired = errors.New("may_chu_thu: lần lưu đầu tiên phải nhập mật khẩu")
	// ErrMailPasswordRequiredForNewDestination — rule 3 in the file header.
	ErrMailPasswordRequiredForNewDestination = errors.New("may_chu_thu: đổi máy chủ, cổng hoặc tài khoản thì phải nhập lại mật khẩu")
	// ErrMailMissingActor — no business code to name in the trail and in updated_by.
	ErrMailMissingActor = errors.New("may_chu_thu: thiếu người thực hiện")
)

// MailSettingsView is what the screen reads. No password field exists to fill.
type MailSettingsView struct {
	domain.MailSettings
	// Configured is false for a commune that has never saved — §10's orange "Chưa có máy chủ thư
	// nào" warning.
	Configured bool
	// EncryptionConfigured is false when the platform runs without SECRET_ENCRYPTION_KEYS; the screen
	// can say so BEFORE the administrator types a password that could not be saved.
	EncryptionConfigured bool
}

// SaveMailSettingsRequest is one save. Password empty means "keep the stored one".
type SaveMailSettingsRequest struct {
	Input    domain.MailSettingsInput
	Password secret.Secret
}

// MailSettingsAdmin owns reading, saving and test-sending one commune's mail server settings.
type MailSettingsAdmin struct {
	db       *store.DB
	repo     MailSettingsRepo
	envelope *crypto.Envelope // nil when SECRET_ENCRYPTION_KEYS is unset — see the file header
	sender   MailSender
	now      func() time.Time // nil = time.Now; tests pin it
}

func NewMailSettingsAdmin(db *store.DB, repo MailSettingsRepo, envelope *crypto.Envelope,
	sender MailSender) *MailSettingsAdmin {
	return &MailSettingsAdmin{db: db, repo: repo, envelope: envelope, sender: sender}
}

// passwordAAD binds the sealed bytes to THIS table, THIS column and THIS commune's row (the row's
// key is tenant_id). Copied onto another column or another commune's row, they do not open.
func passwordAAD(ctx context.Context) []byte {
	return []byte("mail_settings/password_sealed/" + string(tenant.MustFrom(ctx)))
}

// Get reads the commune's settings. An unconfigured commune gets §10's defaults and
// Configured=false, never a 404: the screen is the place where the first configuration is typed.
func (uc *MailSettingsAdmin) Get(ctx context.Context) (MailSettingsView, error) {
	view := MailSettingsView{EncryptionConfigured: uc.envelope != nil}
	// Scoped in the store: MailSettingsStore.Get reads through db.For(ctx).Query.
	m, err := uc.repo.Get(ctx)
	if errors.Is(err, commsstore.ErrMailSettingsNotFound) {
		view.MailSettings = domain.MailSettings{Port: domain.DefaultMailPort, Security: domain.DefaultMailSecurity}
		return view, nil
	}
	if err != nil {
		return MailSettingsView{}, fmt.Errorf("may_chu_thu: đọc cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	view.MailSettings = m
	view.Configured = true
	return view, nil
}

// Save validates, seals a new password if one was typed, and writes the row and its audit entry in
// ONE transaction. A save that changes nothing and carries no password writes and audits nothing.
func (uc *MailSettingsAdmin) Save(ctx context.Context, req SaveMailSettingsRequest,
	actor audit.Actor) (MailSettingsView, error) {

	if uc.envelope == nil {
		return MailSettingsView{}, crypto.ErrNotConfigured
	}
	if actor.ID == "" {
		return MailSettingsView{}, ErrMailMissingActor
	}
	after, err := domain.NormalizeMailSettings(req.Input)
	if err != nil {
		return MailSettingsView{}, err
	}

	var newSealed []byte
	if len(req.Password) > 0 {
		if err := domain.ValidateMailPassword(req.Password.Lo()); err != nil {
			return MailSettingsView{}, err
		}
		if newSealed, err = uc.envelope.Seal(ctx, req.Password, passwordAAD(ctx)); err != nil {
			return MailSettingsView{}, fmt.Errorf("may_chu_thu: niêm mật khẩu: %w", err)
		}
	}

	// The last test result the row still holds once this save commits — what the reply shows.
	var lastTest *domain.MailTestResult
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of every statement.
		before, oldSealed, found, err := uc.repo.ForUpdate(ctx, tx)
		if err != nil {
			return err
		}
		lastTest = before.LastTest
		final := newSealed
		if final == nil {
			switch {
			case !found || len(oldSealed) == 0:
				return ErrMailPasswordRequired
			case domain.MailDestinationChanged(before, after):
				return ErrMailPasswordRequiredForNewDestination
			case domain.SameMailSettings(before, after):
				return nil // nothing moved: no UPDATE, no entry
			}
			final = oldSealed
		}
		if err := uc.repo.Upsert(ctx, tx, after, final, actor.ID); err != nil {
			return err
		}
		// 0019: a result describing a destination no longer configured is forgotten, in the same
		// transaction as the save that moved it.
		testCleared := found && domain.MailTestTargetChanged(before, after)
		if testCleared {
			if err := uc.repo.ClearTest(ctx, tx); err != nil {
				return err
			}
			lastTest = nil
		}

		var delta map[string]any
		if found {
			delta = map[string]any{
				"truoc":             diffMailSettings(before, after, true),
				"sau":               diffMailSettings(before, after, false),
				"password_changed":  newSealed != nil,
				"last_test_cleared": testCleared,
			}
		} else {
			delta = map[string]any{"sau": summarizeMailSettings(after), "password_changed": true}
		}
		b, err := json.Marshal(delta)
		if err != nil {
			return fmt.Errorf("may_chu_thu: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE UPSERT (rule 6, invariant 3). The delta holds no password and no
		// sealed bytes; addresses are masked (rule 6, invariant 5).
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionSaveMailSettings,
			Subject: domain.MailSettingsSubject,
			Delta:   b,
		})
	})
	if err != nil {
		return MailSettingsView{}, fmt.Errorf("may_chu_thu: lưu cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	after.PasswordSet = true
	after.LastTest = lastTest
	return MailSettingsView{MailSettings: after, Configured: true, EncryptionConfigured: true}, nil
}

// SendTestMessage sends the fixed test message to one address the administrator typed, through the
// commune's stored settings — enabled or not: testing BEFORE switching the server on is the point.
//
// THE ATTEMPT IS AUDITED BEFORE THE SEND, in its own transaction. Sending is not a write to
// business data, but it USES the commune's credential towards an address a person chose, and "who
// sent what from our official mailbox, to whom" is exactly the question rule 6 exists to answer.
// Before, not after: an entry written after the send cannot be made atomic with it, and a send
// whose entry failed would be a use of the credential nobody can account for. So no entry, no send.
// The recipient is masked (rule 6, invariant 5).
//
// THE RESULT IS RECORDED AFTER THE SEND RETURNS (migration 0019): masked recipient, ok, error class —
// one UPDATE that leaves updated_at / updated_by alone, with its own entry, in one transaction. A
// refusal BEFORE the send (bad address, no row, no KEK, a password that does not open) records nothing.
func (uc *MailSettingsAdmin) SendTestMessage(ctx context.Context, rawRecipient string, actor audit.Actor) error {
	if uc.envelope == nil {
		return crypto.ErrNotConfigured
	}
	if actor.ID == "" {
		return ErrMailMissingActor
	}
	to, err := domain.NormalizeMailAddress(rawRecipient)
	if err != nil {
		return err
	}
	// The value 0019 stores. Refused HERE, before anything is read or sent, when its CHECK would refuse
	// it: a send whose result cannot be recorded is a send the screen can never report.
	masked := privacy.MaskEmail(to)
	if !domain.ValidMaskedTestRecipient(masked) {
		return domain.ErrMailRecipient
	}
	// Scoped in the store: MailSettingsStore.Account reads through db.For(ctx).Query.
	m, sealed, err := uc.repo.Account(ctx)
	if err != nil {
		return err
	}
	password, err := uc.envelope.Open(ctx, sealed, passwordAAD(ctx))
	if err != nil {
		return fmt.Errorf("may_chu_thu: mở mật khẩu cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	defer clear(password)

	delta, err := json.Marshal(map[string]any{
		"recipient": masked,
		"host":      m.Host,
		"port":      m.Port,
		"security":  m.Security,
	})
	if err != nil {
		return fmt.Errorf("may_chu_thu: mã hoá delta: %w", err)
	}
	if err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionSendTestMail,
			Subject: domain.MailSettingsSubject,
			Delta:   delta,
		})
	}); err != nil {
		return fmt.Errorf("may_chu_thu: ghi vết gửi thử cho xã %s: %w", tenant.MustFrom(ctx), err)
	}

	sendErr := uc.sender.Send(ctx, mail.Account{
		Host: m.Host, Port: m.Port, Security: m.Security, Username: m.Username, Password: password,
	}, mail.Message{
		FromAddress: m.FromAddress, FromName: m.FromName, To: to,
		Subject: testMailSubject, Body: testMailBody,
	})

	// THE RESULT, ONCE THE SEND RETURNED (0019): the four columns and their entry in ONE transaction.
	// Everything that failed BEFORE the send returned above and records nothing — no test reached a
	// server. The class is the sentinel's, never the server's words (internal/mail never returns them).
	result := domain.MailTestResult{At: uc.clock(), ToMasked: masked, OK: sendErr == nil}
	if sendErr != nil {
		result.ErrorClass = mailErrorClass(sendErr)
	}
	rdelta, err := json.Marshal(map[string]any{
		"recipient": masked, "ok": result.OK, "error_class": result.ErrorClass,
	})
	if err != nil {
		return fmt.Errorf("may_chu_thu: mã hoá delta: %w", err)
	}
	if err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.repo.RecordTest(ctx, tx, result); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionRecordTestMailResult,
			Subject: domain.MailSettingsSubject,
			At:      result.At,
			Delta:   rdelta,
		})
	}); err != nil {
		// The server answered; the record did not land. The screen is told it failed rather than shown a
		// result that is not on the row — the attempt itself is already on record (above).
		return fmt.Errorf("may_chu_thu: ghi kết quả gửi thử cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return sendErr
}

// mailTestErrorClasses — 0019's table, sentinel → class, one to one. A wrapped sentinel (with its SMTP
// reply code) still matches.
var mailTestErrorClasses = []struct {
	err   error
	class string
}{
	{mail.ErrConnect, domain.MailTestErrorConnect},
	{mail.ErrTimeout, domain.MailTestErrorTimeout},
	{mail.ErrCertificate, domain.MailTestErrorCertificate},
	{mail.ErrTLS, domain.MailTestErrorTLS},
	{mail.ErrStartTLSMissing, domain.MailTestErrorStartTLSMissing},
	{mail.ErrPlaintextRefused, domain.MailTestErrorPlaintextRefused},
	{mail.ErrAuthUnsupported, domain.MailTestErrorAuthUnsupported},
	{mail.ErrAuthRejected, domain.MailTestErrorAuthRejected},
	{mail.ErrRecipientRejected, domain.MailTestErrorRecipientRejected},
	{mail.ErrProtocol, domain.MailTestErrorProtocol},
}

// mailErrorClass is the stored class of a send failure; MailTestErrorOther for anything not a sentinel.
func mailErrorClass(err error) string {
	for _, c := range mailTestErrorClasses {
		if errors.Is(err, c.err) {
			return c.class
		}
	}
	return domain.MailTestErrorOther
}

func (uc *MailSettingsAdmin) clock() time.Time {
	now := time.Now
	if uc.now != nil {
		now = uc.now
	}
	return now().UTC().Truncate(time.Microsecond)
}

// summarizeMailSettings is the full non-secret picture, addresses masked.
func summarizeMailSettings(m domain.MailSettings) map[string]any {
	return map[string]any{
		"host":         m.Host,
		"port":         m.Port,
		"security":     m.Security,
		"username":     privacy.MaskEmail(m.Username),
		"from_address": privacy.MaskEmail(m.FromAddress),
		"from_name":    m.FromName,
		"is_enabled":   m.IsEnabled,
	}
}

// diffMailSettings returns only the fields that moved, from the side asked for.
func diffMailSettings(before, after domain.MailSettings, beforeSide bool) map[string]any {
	out := map[string]any{}
	if before.Host != after.Host {
		out["host"] = chon(beforeSide, before.Host, after.Host)
	}
	if before.Port != after.Port {
		out["port"] = chon(beforeSide, before.Port, after.Port)
	}
	if before.Security != after.Security {
		out["security"] = chon(beforeSide, before.Security, after.Security)
	}
	if before.Username != after.Username {
		out["username"] = chon(beforeSide, privacy.MaskEmail(before.Username), privacy.MaskEmail(after.Username))
	}
	if before.FromAddress != after.FromAddress {
		out["from_address"] = chon(beforeSide, privacy.MaskEmail(before.FromAddress), privacy.MaskEmail(after.FromAddress))
	}
	if before.FromName != after.FromName {
		out["from_name"] = chon(beforeSide, before.FromName, after.FromName)
	}
	if before.IsEnabled != after.IsEnabled {
		out["is_enabled"] = chon(beforeSide, before.IsEnabled, after.IsEnabled)
	}
	return out
}
