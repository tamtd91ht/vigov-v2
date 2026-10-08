package app

// The write surface of a commune's citizen-letter deadline rules (migration 0027; ADR 0084 #3,
// ADR 0085 B, câu 2–4): create a rule for one (letter type, deadline kind), change its amount or
// unit, remove it with a reason. `admin.sla` at the route and the audit entry are the control; there
// is no approval step, as for `sla` (ADR 0079 lô 2 Q4).
//
// THE UNIT LOCK IS CHECKED ON THE RESULT, NOT THE REQUEST (domain.CheckCitizenLetterDeadlineRule): a
// stored row that violates it — restored from elsewhere, or written before the legal review moved a
// cell — cannot be committed again by an edit that only meant to change its number.
//
// NOTHING HERE TOUCHES A DEADLINE ALREADY STORED on a letter (rule 10, invariant 2; ADR 0007 decision
// 6). A rule created, changed or removed changes what the NEXT booking or `thu-ly` reads. Removing a
// rule means "Không đặt hạn" from that moment on (ADR 0085 B3).

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The verbs in the trail — Vietnamese snake_case like every other action this system writes; an
// inspection reads these strings.
const (
	ActionAddCitizenLetterDeadlineRule    = "them_han_don_thu"
	ActionEditCitizenLetterDeadlineRule   = "sua_han_don_thu"
	ActionRemoveCitizenLetterDeadlineRule = "xoa_han_don_thu"
)

// CitizenLetterDeadlineRuleWriter is the store, declared at the point of use. Every mutating method
// takes the transaction the audit entry is written in.
type CitizenLetterDeadlineRuleWriter interface {
	GetForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.CitizenLetterDeadlineRule, error)
	Insert(ctx context.Context, tx *store.ScopedTx, r domain.CitizenLetterDeadlineRule) error
	UpdateAmountUnit(ctx context.Context, tx *store.ScopedTx, r domain.CitizenLetterDeadlineRule) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
}

// CitizenLetterDeadlineRules owns the write surface.
type CitizenLetterDeadlineRules struct {
	db    *store.DB
	rules CitizenLetterDeadlineRuleWriter

	// newID is injected so a test can pin it. ulid.Moi in production.
	newID func() (string, error)
}

func NewCitizenLetterDeadlineRules(db *store.DB, rules CitizenLetterDeadlineRuleWriter) *CitizenLetterDeadlineRules {
	return &CitizenLetterDeadlineRules{db: db, rules: rules, newID: ulid.Moi}
}

// CreateCitizenLetterDeadlineRuleRequest is one new rule. Every field is required.
type CreateCitizenLetterDeadlineRuleRequest struct {
	LetterType domain.CitizenLetterType
	Kind       domain.CitizenLetterDeadlineKind
	Amount     int
	Unit       domain.CitizenLetterDeadlineUnit
}

// UpdateCitizenLetterDeadlineRuleRequest is a PARTIAL edit: nil leaves the value alone. There is no
// letter type and no kind — what a rule applies to is fixed at creation.
type UpdateCitizenLetterDeadlineRuleRequest struct {
	Amount *int
	Unit   *domain.CitizenLetterDeadlineUnit
}

// ErrCitizenLetterDeadlineRuleExists — this commune already has a live rule for this pair (409).
var ErrCitizenLetterDeadlineRuleExists = errors.New("hạn đơn thư: loại đơn này đã có quy tắc cho loại hạn này — hãy sửa quy tắc sẵn có")

// Create inserts a rule and its audit entry in one transaction. The live-unique key is the guard
// against a duplicate, including two administrators creating the same rule at once.
func (uc *CitizenLetterDeadlineRules) Create(ctx context.Context, req CreateCitizenLetterDeadlineRuleRequest,
	actor NguoiThucHien) (domain.CitizenLetterDeadlineRule, error) {

	if err := actor.hopLe(); err != nil {
		return domain.CitizenLetterDeadlineRule{}, err
	}
	r := domain.CitizenLetterDeadlineRule{
		LetterType: req.LetterType, Kind: req.Kind, Amount: req.Amount, Unit: req.Unit,
	}
	if err := domain.CheckCitizenLetterDeadlineRule(r); err != nil {
		return domain.CitizenLetterDeadlineRule{}, err
	}
	var err error
	if r.ID, err = uc.newID(); err != nil {
		return domain.CitizenLetterDeadlineRule{}, fmt.Errorf("hạn đơn thư: sinh id: %w", err)
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.rules.Insert(ctx, tx, r); err != nil {
			return err
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). The subject names the rule by what it
		// governs (`khieu-nai/giai-quyet`), not by its ULID.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionAddCitizenLetterDeadlineRule,
			Subject: citizenLetterRuleSubject(r),
			Delta:   deltaSLA(map[string]any{"sau": citizenLetterRuleFigures(r)}),
		})
	})
	if errors.Is(err, idstore.ErrCitizenLetterDeadlineRuleExists) {
		return domain.CitizenLetterDeadlineRule{}, ErrCitizenLetterDeadlineRuleExists
	}
	if err != nil {
		return domain.CitizenLetterDeadlineRule{}, err
	}
	return r, nil
}

// Update applies a partial edit. A NO-OP WRITES NOTHING AND AUDITS NOTHING — as app.SLA.Sua, which
// is what makes the PATCH route idempotent.
func (uc *CitizenLetterDeadlineRules) Update(ctx context.Context, id string, req UpdateCitizenLetterDeadlineRuleRequest,
	actor NguoiThucHien) (domain.CitizenLetterDeadlineRule, error) {

	if err := actor.hopLe(); err != nil {
		return domain.CitizenLetterDeadlineRule{}, err
	}
	if id == "" {
		return domain.CitizenLetterDeadlineRule{}, idstore.ErrCitizenLetterDeadlineRuleNotFound
	}

	var after domain.CitizenLetterDeadlineRule
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.rules.GetForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		after = before
		if req.Amount != nil {
			after.Amount = *req.Amount
		}
		if req.Unit != nil {
			after.Unit = *req.Unit
		}
		if err := domain.CheckCitizenLetterDeadlineRule(after); err != nil {
			return err
		}
		if after == before {
			return nil
		}
		if err := uc.rules.UpdateAmountUnit(ctx, tx, after); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionEditCitizenLetterDeadlineRule,
			Subject: citizenLetterRuleSubject(before),
			Delta: deltaSLA(map[string]any{
				"truoc": citizenLetterRuleFigures(before),
				"sau":   citizenLetterRuleFigures(after),
			}),
		})
	})
	if err != nil {
		return domain.CitizenLetterDeadlineRule{}, err
	}
	return after, nil
}

// Remove soft deletes a rule, with a reason. From then on that letter type and kind is "not
// configured": letters booked afterwards carry no deadline (ADR 0085 B3).
func (uc *CitizenLetterDeadlineRules) Remove(ctx context.Context, id, reason string, actor NguoiThucHien) error {
	if err := actor.hopLe(); err != nil {
		return err
	}
	if id == "" {
		return idstore.ErrCitizenLetterDeadlineRuleNotFound
	}
	reason, err := domain.NormalizeCitizenLetterDeleteReason(reason)
	if err != nil {
		return err
	}
	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.rules.GetForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		// deleted_by is the STAFF CODE (rule 6, invariant 8) — the same value as the entry's actor.
		if err := uc.rules.SoftDelete(ctx, tx, before.ID, actor.Vet.ID, reason); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionRemoveCitizenLetterDeadlineRule,
			Subject: citizenLetterRuleSubject(before),
			Delta: deltaSLA(map[string]any{
				"truoc":   citizenLetterRuleFigures(before),
				"ly_do":   reason,
				"xoa_mem": true,
			}),
		})
	})
}

// IsCitizenLetterDeadlineRuleInputError reports a refusal of what the client sent (400). Listed
// explicitly, so a database outage can never become a 400.
func IsCitizenLetterDeadlineRuleInputError(err error) bool {
	for _, e := range []error{
		domain.ErrCitizenLetterTypeUnknown, domain.ErrCitizenLetterDeadlineKindUnknown,
		domain.ErrCitizenLetterNoResolutionDeadline, domain.ErrCitizenLetterDeadlineUnitUnknown,
		domain.ErrCitizenLetterDeadlineUnitLocked, domain.ErrCitizenLetterAmountNotPositive,
		domain.ErrCitizenLetterAmountTooLarge, domain.ErrCitizenLetterDeleteReasonMissing,
		domain.ErrCitizenLetterDeleteReasonTooLong,
	} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// citizenLetterRuleSubject is `<letter_type>/<deadline_kind>` — the commitment the rule governs, as
// the configuration screen prints it. Neither part is personal data.
func citizenLetterRuleSubject(r domain.CitizenLetterDeadlineRule) string {
	return string(r.LetterType) + "/" + string(r.Kind)
}

// citizenLetterRuleFigures is the before/after payload: the two values that can change. No personal
// data (rule 6, forbidden #4).
func citizenLetterRuleFigures(r domain.CitizenLetterDeadlineRule) map[string]any {
	return map[string]any{"amount": r.Amount, "unit": string(r.Unit)}
}
