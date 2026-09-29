package app

// The use cases behind the WRITE surface of the disbursement voucher register
// (docs/ui-ux/06-giai-ngan.md §8.2, §13 rule 3).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes
// outside one. Opening that transaction is this layer's job. The handler translates HTTP and
// nothing else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THE LIFECYCLE. Every write here is read-decide-write: read the voucher under
// a row lock, ask the domain whether this operation is admissible from that state, refuse or apply.
// That decision needs the row AND the rules AND the transaction at once, which is exactly one
// place — here.
//
// ---------------------------------------------------------------------------
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE. Said once, here, because it is the
// thing most likely to be misread as duplication:
//
//	CHECK (so_tien > 0)               0004:305
//	the three states                  0004:298
//	a locked voucher is frozen        0004, trigger `chung_tu_da_khoa` (:141-168)
//	an unlock carries its reason      0005, `chung_tu_giai_ngan_mo_khoa_du_vet`
//	hard DELETE refused outright      0004, `ho_so_luu_tru_cam_xoa_cung`
//
// Those hold against every writer — this service, a psql prompt, an import job written next year.
// What they cannot do is explain themselves: PostgreSQL answers with an exception whose text is
// English, names a constraint, and tells an accountant in a commune nothing they can act on. This
// layer refuses FIRST, in Vietnamese, naming the operation and the way out. A drift between the two
// is therefore a worse error message, never a hole — the constraint still runs last, and the whole
// transaction rolls back with the audit entry inside it.
//
// ---------------------------------------------------------------------------
// WHAT IS NOT BUILT HERE, deliberately:
//
//	the refund (khoản hoàn)   ADR 0035 §B and open question #30. A refund is a SEPARATE voucher,
//	                          not a negative amount and not an edit of an old voucher down to a
//	                          smaller figure. `CHECK (so_tien > 0)` is untouched, and UpdateVoucher's audit
//	                          delta is what makes the tempting detour visible if anybody takes it.
//	a ceiling on unlocks      decision (3) of migration 0005: counted, never capped. A ceiling is
//	                          a number that belongs to the customer, and a count is what lets them
//	                          choose one later from real figures.
//	"which threshold was in   nothing in this repository persists a reported period's disbursement
//	force when we reported"   figures, so there is nothing that could disagree with itself yet.
//	                          The day a reported period IS stored, the threshold used has to be
//	                          stored in the same row — 0005's closing note says why.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// DisbursementVoucherStore is the store, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the voucher in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that the unlock rule compares the right two staff codes — provable
// without a PostgreSQL. There is none reachable from this repository's build environment, so a test
// that needed one would be a test that never runs.
type DisbursementVoucherStore interface {
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.DisbursementVoucher, error)
	LiveInvestmentProjectCode(ctx context.Context, tx *store.ScopedTx, investmentProjectID string) (string, error)
	// CheckLiveFundingSource takes the TRANSACTION for the reason every method here does: there is no
	// foreign key under `chung_tu_giai_ngan.nguon_von_id` (0007:102-113), so this check IS the
	// constraint — and a constraint asked outside the transaction that writes the row is a
	// constraint that can already be stale when the write lands.
	CheckLiveFundingSource(ctx context.Context, tx *store.ScopedTx, fundingSourceID string) error
	InsertVoucher(ctx context.Context, tx *store.ScopedTx, voucher domain.DisbursementVoucher) error
	UpdateVoucher(ctx context.Context, tx *store.ScopedTx, voucher domain.DisbursementVoucher) error
	Confirm(ctx context.Context, tx *store.ScopedTx, id, staffCode string) error
	ResetToEnteredAfterUpdate(ctx context.Context, tx *store.ScopedTx, id string) error
	Lock(ctx context.Context, tx *store.ScopedTx, id, staffCode string, at time.Time) error
	Unlock(ctx context.Context, tx *store.ScopedTx, id string,
		statusBack domain.VoucherStatus, staffCode, reason string, at time.Time) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
}

// DisbursementVoucherService owns entering, correcting, confirming, freezing, reopening and removing one
// commune's disbursement vouchers.
type DisbursementVoucherService struct {
	db   *store.DB
	repo DisbursementVoucherStore

	// newID is injected so a test can pin the id. In production it is ulid.Moi.
	newID func() (string, error)

	// now is the clock `thoi_diem_khoa` and `thoi_diem_mo_khoa` are stamped from.
	//
	// A SEAM AND NOT time.Now() AT THE CALL SITE, for a reason that is about evidence rather than
	// convenience: both columns are read years later beside an audit entry carrying its own `at`,
	// and a test that cannot pin the instant cannot assert that the two agree. In production this
	// is nil and nowUTC returns the real clock.
	now func() time.Time
}

func NewDisbursementVoucherService(db *store.DB, repo DisbursementVoucherStore) *DisbursementVoucherService {
	return &DisbursementVoucherService{db: db, repo: repo, newID: ulid.Moi}
}

// nowUTC is the clock, UTC. `TIMESTAMPTZ` stores an instant rather than a wall reading, so the
// location here changes nothing that is stored — it is fixed so that a value read back in a test
// compares equal without a location dance.
func (uc *DisbursementVoucherService) nowUTC() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `them_hang_muc_ke_hoach_von`): an inspection reads these
// strings, and a function name would tell them nothing.
//
// SIX VERBS AND NOT ONE `sua_chung_tu`, because they are six different administrative acts with six
// different consequences. `mo_khoa_chung_tu_giai_ngan` in particular is the one an inspection
// searches for by name — it is the record of a signed figure being reopened.
const (
	ActionCreateDisbursementVoucher  = "them_chung_tu_giai_ngan"
	ActionUpdateDisbursementVoucher  = "sua_chung_tu_giai_ngan"
	ActionRemoveDisbursementVoucher  = "go_chung_tu_giai_ngan"
	ActionConfirmDisbursementVoucher = "xac_nhan_chung_tu_giai_ngan"
	ActionLockDisbursementVoucher    = "khoa_chung_tu_giai_ngan"
	ActionUnlockDisbursementVoucher  = "mo_khoa_chung_tu_giai_ngan"
)

// CreateDisbursementVoucherRequest is one new voucher, as it arrives from the handler.
//
// THERE IS NO `Status` FIELD AND THERE MUST NEVER BE ONE. A new voucher is `Kế toán nhập`,
// always: the store writes the state as a LITERAL, so there is no value any layer above could pass.
// A field here is a field a handler can fill from a request body, and what that buys is a voucher
// created already `Đã khoá` — a figure nobody confirmed, frozen against editing, counting toward
// the commune's disbursement total.
//
// THERE IS NO `EnteredByID` FIELD EITHER. Who entered it is the acting principal, which arrives as
// the audit.Actor beside the request; a field would be a second, client-supplied answer to a
// question the session already answers.
type CreateDisbursementVoucherRequest struct {
	InvestmentProjectID string
	PaymentDate         time.Time
	Amount              domain.Dong
	Description         string
	Counterparty        string
	VoucherNo           string

	// FundingSourceID is OPTIONAL, and empty is a legitimate answer rather than an omission: §13 rule 6
	// says a voucher with no funding source still counts toward "đã giải ngân" and is reported
	// separately as "đã chi nhưng chưa ghi rút từ nguồn nào" — §6 prints that as a real figure on a
	// real commune. Nothing here may start requiring it: refusing the entry would refuse exactly the
	// operation the specification permits, at the moment a payment has already left the account.
	FundingSourceID string
}

// UpdateDisbursementVoucherRequest is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. `Counterparty` and `VoucherNo` are optional and their
// meaningful value includes the empty string — "this voucher has no treasury number" is a statement,
// not an absence of one. A struct of plain values cannot tell "the client did not mention this" from
// "the client cleared it", so a screen editing only the description would silently wipe the
// counterparty off a payment record.
//
// `InvestmentProjectID` IS ABSENT AND IS NOT AN OVERSIGHT: moving a voucher between projects moves money between
// two reported totals with nothing on either screen saying so. The operation for a voucher filed
// against the wrong project is to remove it with a reason and enter it again — two events, both
// audited, both visible.
type UpdateDisbursementVoucherRequest struct {
	PaymentDate  *time.Time
	Amount       *domain.Dong
	Description  *string
	Counterparty *string
	VoucherNo    *string

	// FundingSourceID follows the same convention as the two optional strings above, and it carries THREE
	// distinct meanings that a plain string could only carry two of:
	//
	//	nil    leave the voucher's funding source exactly as it is
	//	""     DETACH it — the column goes to NULL and the voucher rejoins §6's "đã chi nhưng chưa
	//	       ghi rút từ nguồn nào" warning. A real correction: the accountant attributed a payment
	//	       to the wrong source and the right one is not yet known.
	//	"01J…" attach it to that source, which must be a LIVE source OF THIS COMMUNE (rule 1).
	//
	// "" IS NORMALISED TO NULL IN THIS LAYER AND NEVER TRAVELS DOWNWARD AS A BLANK. The column
	// carries `CHECK (nguon_von_id IS NULL OR btrim(nguon_von_id) <> '')` (0007:274-277), so an empty
	// string is refused by the database — and if that CHECK were ever dropped, the voucher would fall
	// out of the warning while belonging to no source either: money missing from both sides of one
	// screen, with every row looking filled in.
	FundingSourceID *string
}

// CreateVoucher records one payment against one project.
//
// THE SHAPE IS VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never
// hold a row lock while doing so, and the caller needs the reason rather than a rollback.
func (uc *DisbursementVoucherService) CreateVoucher(ctx context.Context, req CreateDisbursementVoucherRequest,
	actor audit.Actor) (domain.DisbursementVoucher, error) {

	if req.InvestmentProjectID == "" {
		return domain.DisbursementVoucher{}, domain.ErrInvestmentProjectMissing
	}
	if err := domain.ValidateAmount(req.Amount); err != nil {
		return domain.DisbursementVoucher{}, err
	}
	if err := domain.ValidatePaymentDate(req.PaymentDate); err != nil {
		return domain.DisbursementVoucher{}, err
	}
	description, err := domain.NormalizeVoucherDescription(req.Description)
	if err != nil {
		return domain.DisbursementVoucher{}, err
	}
	counterparty, err := domain.NormalizeCounterparty(req.Counterparty)
	if err != nil {
		return domain.DisbursementVoucher{}, err
	}
	voucherNo, err := domain.NormalizeVoucherNo(req.VoucherNo)
	if err != nil {
		return domain.DisbursementVoucher{}, err
	}
	fundingSourceID, err := domain.NormalizeFundingSourceID(req.FundingSourceID)
	if err != nil {
		return domain.DisbursementVoucher{}, err
	}
	if err := requireActor(actor); err != nil {
		return domain.DisbursementVoucher{}, err
	}

	id, err := uc.newID()
	if err != nil {
		return domain.DisbursementVoucher{}, fmt.Errorf("chung_tu_giai_ngan: sinh mã: %w", err)
	}

	next := domain.DisbursementVoucher{
		ID:                  id,
		InvestmentProjectID: req.InvestmentProjectID,
		PaymentDate:         req.PaymentDate,
		Amount:              req.Amount,
		Description:         description,
		Counterparty:        counterparty,
		VoucherNo:           voucherNo,
		// EMPTY IS CARRIED THROUGH AS EMPTY and becomes NULL in the store (emptyToNil). It is the
		// value §6's "đã chi nhưng chưa ghi rút từ nguồn nào" counts, not an absence to be filled in.
		FundingSourceID: fundingSourceID,
		// Set here only so the value this function RETURNS describes the row that was written. The
		// store does not read it: it writes 'ke-toan-nhap' as a literal.
		Status: domain.VoucherEntered,
		// THE STAFF BUSINESS CODE, never the internal id — migration 0005:52-59 states the
		// convention for all four `nguoi_*_id` columns, and audit.Actor.ID already carries exactly
		// that value (the handler reads `Principal.Ma`, rule 6, invariant 8).
		EnteredByID: actor.ID,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE PROJECT IS READ FIRST, AND FOR TWO REASONS AT ONCE: it refuses a voucher filed
		// against a project this commune does not have (money that would total nowhere), and it
		// yields the BUSINESS CODE the audit entry is filed under. A voucher has no code of its
		// own — there is no `ma` column on the table — so `subject` is the project's, and the
		// voucher is named inside the delta.
		investmentProjectCode, err := uc.repo.LiveInvestmentProjectCode(ctx, tx, next.InvestmentProjectID)
		if err != nil {
			return err
		}
		// THE SOURCE IS CHECKED ONLY WHEN ONE WAS NAMED. A voucher with no source is the state §13
		// rule 6 defines, so an empty value has nothing to verify — asking anyway would turn "no
		// source" into an error and refuse the operation the specification permits.
		//
		// INSIDE THE TRANSACTION, because with no foreign key underneath (0007:102-113) this check IS
		// the constraint. An id naming nothing, or naming another commune's source, would put the
		// money on no card of §6 AND out of the "chưa ghi rút từ nguồn nào" warning at the same time.
		if next.FundingSourceID != "" {
			if err := uc.repo.CheckLiveFundingSource(ctx, tx, next.FundingSourceID); err != nil {
				return err
			}
		}
		if err := uc.repo.InsertVoucher(ctx, tx, next); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": voucherSummary(next)})
		if err != nil {
			return fmt.Errorf("chung_tu_giai_ngan: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides
		// which commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionCreateDisbursementVoucher,
			Subject: investmentProjectCode,
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no voucher, no trail. The two states agree.
		return domain.DisbursementVoucher{}, wrapVoucherErr(ctx, "thêm", err)
	}
	return next, nil
}

// UpdateVoucher corrects an UNLOCKED voucher.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending a voucher the figures it already has is not an
// event; recording it would fill a public authority's ledger with entries saying nothing changed,
// and those are the entries that bury the ones carrying legal weight. It is also what makes this
// route genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
//
// THE LOCKED CASE IS REFUSED BEFORE ANY UPDATE IS ATTEMPTED, and that is the whole point of the
// layer: the trigger would refuse it too, with an English exception naming a constraint. §13 rule 3
// is what an accountant is owed instead — "chứng từ đã khoá thì không sửa, không gỡ — phải mở khoá
// trước (quyền `budget.confirm`)".
func (uc *DisbursementVoucherService) UpdateVoucher(ctx context.Context, id string, req UpdateDisbursementVoucherRequest,
	actor audit.Actor) (domain.DisbursementVoucher, error) {

	if id == "" {
		return domain.DisbursementVoucher{}, fistore.ErrVoucherNotFound
	}
	// Shape first, outside the transaction, for the same reason as CreateVoucher.
	if req.Amount != nil {
		if err := domain.ValidateAmount(*req.Amount); err != nil {
			return domain.DisbursementVoucher{}, err
		}
	}
	if req.PaymentDate != nil {
		if err := domain.ValidatePaymentDate(*req.PaymentDate); err != nil {
			return domain.DisbursementVoucher{}, err
		}
	}
	var description, counterparty, voucherNo, fundingSourceID string
	var err error
	if req.Description != nil {
		if description, err = domain.NormalizeVoucherDescription(*req.Description); err != nil {
			return domain.DisbursementVoucher{}, err
		}
	}
	if req.Counterparty != nil {
		if counterparty, err = domain.NormalizeCounterparty(*req.Counterparty); err != nil {
			return domain.DisbursementVoucher{}, err
		}
	}
	if req.VoucherNo != nil {
		if voucherNo, err = domain.NormalizeVoucherNo(*req.VoucherNo); err != nil {
			return domain.DisbursementVoucher{}, err
		}
	}
	if req.FundingSourceID != nil {
		// TRIMMED HERE, SO "   " AND "" BECOME ONE ANSWER before anything compares them. Without it,
		// a client clearing the field with spaces would produce a value that differs from "" — so the
		// no-op comparison below would see a change, the voucher would be written, and a CONFIRMED
		// voucher would lose its confirmation over an edit that changed nothing (ADR 0036).
		if fundingSourceID, err = domain.NormalizeFundingSourceID(*req.FundingSourceID); err != nil {
			return domain.DisbursementVoucher{}, err
		}
	}
	if err := requireActor(actor); err != nil {
		return domain.DisbursementVoucher{}, err
	}

	var after domain.DisbursementVoucher
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := before.CanUpdate(); err != nil {
			return err
		}

		after = before
		if req.PaymentDate != nil {
			after.PaymentDate = *req.PaymentDate
		}
		if req.Amount != nil {
			after.Amount = *req.Amount
		}
		if req.Description != nil {
			after.Description = description
		}
		if req.Counterparty != nil {
			after.Counterparty = counterparty
		}
		if req.VoucherNo != nil {
			after.VoucherNo = voucherNo
		}
		if req.FundingSourceID != nil {
			after.FundingSourceID = fundingSourceID
		}

		if voucherUnchanged(before, after) {
			return nil
		}

		investmentProjectCode, err := uc.repo.LiveInvestmentProjectCode(ctx, tx, before.InvestmentProjectID)
		if err != nil {
			return err
		}
		// CHECKED ONLY WHEN THE SOURCE ACTUALLY MOVES TO A NEW ONE, and both halves of that are
		// deliberate:
		//
		//	unchanged     a correction of the description must not fail because the source this
		//	              voucher has always named was removed from the catalogue afterwards. The row
		//	              is the historical fact; refusing to edit anything else would strand it.
		//	moved to ""   detaching needs nothing to exist. It is the state §13 rule 6 defines.
		//
		// What IS refused is attaching money to a source that does not exist in this commune — the
		// case rule 1 is made for, and the case no foreign key is underneath to catch.
		if after.FundingSourceID != before.FundingSourceID && after.FundingSourceID != "" {
			if err := uc.repo.CheckLiveFundingSource(ctx, tx, after.FundingSourceID); err != nil {
				return err
			}
		}
		if err := uc.repo.UpdateVoucher(ctx, tx, after); err != nil {
			return err
		}

		// A CONFIRMED VOUCHER GOES BACK TO `Kế toán nhập` BECAUSE ITS FIGURES JUST MOVED.
		// *"Lãnh đạo xác nhận những con số kia, không phải những con số này."* The customer's rule,
		// measured in `../vigov-require` commit `c3f4d6a`; domain.StatusAfterUpdate owns the decision
		// and this is the only caller.
		//
		// IT RUNS ONLY WHEN SOMETHING REALLY CHANGED. The no-op branch above has already returned, so
		// a repeat of an identical PATCH cannot strip a confirmation off a voucher nobody edited —
		// which would make `idem.KhongCan` on that route a lie AND undo a leader's act for nothing.
		resetToEntered := domain.StatusAfterUpdate(before.Status) != before.Status
		if resetToEntered {
			if err := uc.repo.ResetToEnteredAfterUpdate(ctx, tx, before.ID); err != nil {
				return err
			}
			after.Status = domain.StatusAfterUpdate(before.Status)
			// The column is cleared with the state, so the row stops naming somebody as the confirmer
			// of figures they have not seen. Who HAD confirmed it is in the entry below, which is
			// append-only.
			after.ConfirmedByID = ""
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to
		// find in a ledger that is never deleted.
		//
		// THE AMOUNT IS THE FIELD THIS DELTA EXISTS FOR. Open question #30 says a refund is a
		// separate voucher, not a sign change — and the detour that would avoid ever recording a
		// refund is editing an old voucher DOWN to a smaller figure. That edit is legitimate as a
		// correction and indistinguishable from the detour on the row itself; the only thing that
		// tells them apart afterwards is this pair of numbers in an append-only ledger.
		body := map[string]any{
			"chung_tu_id": after.ID,
			"truoc":       voucherDiff(before, after, true),
			"sau":         voucherDiff(before, after, false),
		}
		if resetToEntered {
			// RECORDED AS ITS OWN FACT, NOT FOLDED INTO `truoc`/`sau`. Losing a confirmation is not a
			// field the accountant edited — it is a consequence of the edit, and it undoes a named
			// person's act. `nguoi_xac_nhan_id` is nulled on the row, so this entry is the ONLY place
			// that still answers "who had confirmed these figures before they were changed".
			body["mat_xac_nhan"] = map[string]any{
				"truoc_trang_thai":  string(before.Status),
				"sau_trang_thai":    string(after.Status),
				"nguoi_xac_nhan_cu": before.ConfirmedByID,
			}
		}
		delta, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("chung_tu_giai_ngan: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionUpdateDisbursementVoucher,
			Subject: investmentProjectCode,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.DisbursementVoucher{}, wrapVoucherErr(ctx, "sửa", err)
	}
	return after, nil
}

// Remove soft deletes one voucher — `🗑 Gỡ` on the §8.2 screen.
//
// THIS IS NOT A DELETE AND THE NAME IS THE ONLY PLACE THAT COULD SUGGEST OTHERWISE. The row stays,
// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1); its money leaves
// `da_giai_ngan` because every read path excludes soft-deleted rows, not because anything was
// destroyed. `ho_so_luu_tru_cam_xoa_cung` refuses a hard DELETE on this table outright.
//
// THE REASON IS MANDATORY. A voucher that vanished from a project's total with no reason attached
// is money nobody can account for — and the row is still there, so the question WILL be asked.
func (uc *DisbursementVoucherService) Remove(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return fistore.ErrVoucherNotFound
	}
	reason, err := domain.NormalizeRemoveReason(rawReason)
	if err != nil {
		return err
	}
	if err := requireActor(actor); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := before.CanRemove(); err != nil {
			return err
		}
		investmentProjectCode, err := uc.repo.LiveInvestmentProjectCode(ctx, tx, before.InvestmentProjectID)
		if err != nil {
			return err
		}
		// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds. Two
		// kinds of identifier in one column is a column nobody can query (rule 6, invariant 8).
		if err := uc.repo.SoftDelete(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication
		// rule 9 forbids: the column is the current state of the row and can only ever hold the
		// FIRST removal, while the entry is the append-only record of the act. They answer two
		// different questions and neither can be derived from the other.
		delta, err := json.Marshal(map[string]any{
			"chung_tu_id": before.ID,
			"truoc":       voucherSummary(before),
			"ly_do":       reason,
			"xoa_mem":     true,
		})
		if err != nil {
			return fmt.Errorf("chung_tu_giai_ngan: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionRemoveDisbursementVoucher,
			Subject: investmentProjectCode,
			Delta:   delta,
		})
	})
	if err != nil {
		return wrapVoucherErr(ctx, "gỡ", err)
	}
	return nil
}

// Confirm moves a voucher from `Kế toán nhập` to `Đã xác nhận` (§8.2, `budget.confirm`).
func (uc *DisbursementVoucherService) Confirm(ctx context.Context, id string,
	actor audit.Actor) (domain.DisbursementVoucher, error) {

	return uc.changeStatus(ctx, id, actor, ActionConfirmDisbursementVoucher,
		func(tx *store.ScopedTx, before domain.DisbursementVoucher) (domain.DisbursementVoucher, map[string]any, error) {
			if err := before.CanConfirm(); err != nil {
				return domain.DisbursementVoucher{}, nil, err
			}
			if err := uc.repo.Confirm(ctx, tx, before.ID, actor.ID); err != nil {
				return domain.DisbursementVoucher{}, nil, err
			}
			after := before
			after.Status = domain.VoucherConfirmed
			after.ConfirmedByID = actor.ID
			return after, map[string]any{
				"chung_tu_id": before.ID,
				"truoc":       map[string]any{"trang_thai": string(before.Status)},
				"sau":         map[string]any{"trang_thai": string(after.Status)},
				"so_tien":     int64(before.Amount),
			}, nil
		})
}

// Lock freezes a voucher (§8.2, `budget.confirm`). From `Đã xác nhận` only — see
// domain.ErrLockRequiresConfirmation for why the chain is required even though the screen draws
// both buttons on a `Kế toán nhập` row.
func (uc *DisbursementVoucherService) Lock(ctx context.Context, id string,
	actor audit.Actor) (domain.DisbursementVoucher, error) {

	at := uc.nowUTC()
	return uc.changeStatus(ctx, id, actor, ActionLockDisbursementVoucher,
		func(tx *store.ScopedTx, before domain.DisbursementVoucher) (domain.DisbursementVoucher, map[string]any, error) {
			if err := before.CanLock(); err != nil {
				return domain.DisbursementVoucher{}, nil, err
			}
			if err := uc.repo.Lock(ctx, tx, before.ID, actor.ID, at); err != nil {
				return domain.DisbursementVoucher{}, nil, err
			}
			after := before
			after.Status = domain.VoucherLocked
			after.LockedByID = actor.ID
			after.LockedAt = at
			return after, map[string]any{
				"chung_tu_id": before.ID,
				"truoc":       map[string]any{"trang_thai": string(before.Status)},
				"sau":         map[string]any{"trang_thai": string(after.Status)},
				"so_tien":     int64(before.Amount),
			}, nil
		})
}

// Unlock reopens a frozen voucher (ADR 0035 §B, `budget.confirm`).
//
// TWO RULES, BOTH DECIDED BY THIS PROJECT ON 2026-09-22 rather than by the customer, in the
// direction that can be loosened later with one line and cannot be tightened later at all
// (migration 0005's header sets both out in full):
//
//	the reason is MANDATORY        the trail of unlocks that have already happened cannot be
//	                              rebuilt from any source, and it is exactly the figure an
//	                              inspection asks about — one somebody signed and somebody changed.
//	the person who locked it       nobody acts alone on the act that gives themselves room. Same
//	  may NOT unlock it            shape the customer settled for the staff register in #13 and #14.
//
// THE SECOND ONE ANSWERS 409, NOT 403, and that is not a detail: the caller HOLDS `budget.confirm`
// and is allowed to unlock vouchers. What is refused is this person against THIS row. A 403 would
// send them to the Phân quyền screen to be granted a permission they already have.
//
// THE COUNT IS INCREMENTED, NEVER CAPPED — decision (3). A ceiling is a number that belongs to the
// customer; a count is what lets them pick one later from real figures instead of somebody's guess.
func (uc *DisbursementVoucherService) Unlock(ctx context.Context, id, rawReason string,
	actor audit.Actor) (domain.DisbursementVoucher, error) {

	if id == "" {
		return domain.DisbursementVoucher{}, fistore.ErrVoucherNotFound
	}
	// THE REASON IS VALIDATED BEFORE THE TRANSACTION OPENS, so an unlock with no reason never holds
	// a row lock and never reaches the constraint. `chung_tu_giai_ngan_mo_khoa_du_vet` refuses the
	// same thing underneath; this is the sentence that arrives first.
	reason, err := domain.NormalizeUnlockReason(rawReason)
	if err != nil {
		return domain.DisbursementVoucher{}, err
	}

	at := uc.nowUTC()
	return uc.changeStatus(ctx, id, actor, ActionUnlockDisbursementVoucher,
		func(tx *store.ScopedTx, before domain.DisbursementVoucher) (domain.DisbursementVoucher, map[string]any, error) {
			// `actor.ID` IS THE STAFF BUSINESS CODE and `before.LockedByID` holds the same kind of
			// value (migration 0005:52-59). Comparing anything else here — an internal id, a display
			// name — compares two things that are not the same kind of identifier, and the comparison
			// would simply never be true, which reads as "the rule is off" rather than as a bug.
			if err := before.CanUnlock(actor.ID); err != nil {
				return domain.DisbursementVoucher{}, nil, err
			}
			back := domain.StatusAfterUnlock()
			if err := uc.repo.Unlock(ctx, tx, before.ID, back, actor.ID, reason, at); err != nil {
				return domain.DisbursementVoucher{}, nil, err
			}
			after := before
			after.Status = back
			after.UnlockedByID = actor.ID
			after.UnlockedAt = at
			after.UnlockReason = reason
			after.UnlockCount = before.UnlockCount + 1
			return after, map[string]any{
				"chung_tu_id": before.ID,
				"truoc": map[string]any{
					"trang_thai":    string(before.Status),
					"nguoi_khoa_id": before.LockedByID,
				},
				"sau":            map[string]any{"trang_thai": string(after.Status)},
				"ly_do":          reason,
				"so_lan_mo_khoa": after.UnlockCount,
				"so_tien":        int64(before.Amount),
			}, nil
		})
}

// changeStatus is the read-decide-write shared by the three lifecycle moves.
//
// ONE FUNCTION AND NOT THREE COPIES, because the part that is identical is the part that is easy to
// get subtly wrong in one copy: the row lock, the project code for the subject, and the audit entry
// INSIDE the same transaction. What differs — which transition is admissible, what is written, what
// the delta says — is the closure, so a new lifecycle move cannot accidentally reuse another's
// decision.
func (uc *DisbursementVoucherService) changeStatus(ctx context.Context, id string, actor audit.Actor,
	action string,
	apply func(tx *store.ScopedTx, before domain.DisbursementVoucher) (domain.DisbursementVoucher, map[string]any, error),
) (domain.DisbursementVoucher, error) {

	if id == "" {
		return domain.DisbursementVoucher{}, fistore.ErrVoucherNotFound
	}
	if err := requireActor(actor); err != nil {
		return domain.DisbursementVoucher{}, err
	}

	var after domain.DisbursementVoucher
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		investmentProjectCode, err := uc.repo.LiveInvestmentProjectCode(ctx, tx, before.InvestmentProjectID)
		if err != nil {
			return err
		}
		next, body, err := apply(tx, before)
		if err != nil {
			return err
		}
		after = next

		delta, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("chung_tu_giai_ngan: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  action,
			Subject: investmentProjectCode,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.DisbursementVoucher{}, wrapVoucherErr(ctx, action, err)
	}
	return after, nil
}

// requireActor refuses a write whose trail cannot name who made it.
//
// core/audit refuses an entry with no actor for the same reason; refusing HERE, before the
// transaction opens, keeps `nguoi_nhap_id` / `deleted_by` / `nguoi_khoa_id` and the entry telling
// the same story — and avoids a rollback whose cause is a missing principal rather than anything
// about the voucher. Rule 6 does not permit a business write whose trail cannot name its author.
func requireActor(actor audit.Actor) error {
	if actor.ID == "" {
		return fmt.Errorf("chung_tu_giai_ngan: thiếu người thực hiện")
	}
	return nil
}

// wrapVoucherErr wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No amount, no counterparty, no description: an error travels into centralised logging across every
// commune at once, and a voucher's free-text description is somebody's typing about a public
// authority's spending. The commune is not personal data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is.
// A %v here would collapse "this voucher is locked" and "the database is down" into one 500.
func wrapVoucherErr(ctx context.Context, op string, err error) error {
	return fmt.Errorf("chung_tu_giai_ngan: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}

// voucherSummary is the audit delta's view of one voucher.
//
// NOTHING HERE IS PERSONAL DATA (rule 3). `doi_tac` is a company — 0004:283-284 says so outright,
// and the domain repeats it: a commune that starts typing an individual's name and national ID into
// that box turns a disbursement register into a store of personal data under Decree 13, which is a
// decision for the customer rather than something to accommodate here.
func voucherSummary(c domain.DisbursementVoucher) map[string]any {
	return map[string]any{
		"chung_tu_id":  c.ID,
		"du_an_id":     c.InvestmentProjectID,
		"ngay_chi":     c.PaymentDate.Format("2006-01-02"),
		"so_tien":      int64(c.Amount),
		"noi_dung":     c.Description,
		"doi_tac":      c.Counterparty,
		"so_chung_tu":  c.VoucherNo,
		"nguon_von_id": c.FundingSourceID,
		"trang_thai":   string(c.Status),
	}
}

// voucherDiff returns only the fields that actually moved, from whichever side is asked for.
func voucherDiff(before, after domain.DisbursementVoucher, useBefore bool) map[string]any {
	result := map[string]any{}
	if !before.PaymentDate.Equal(after.PaymentDate) {
		result["ngay_chi"] = pick(useBefore, before.PaymentDate, after.PaymentDate).Format("2006-01-02")
	}
	if before.Amount != after.Amount {
		result["so_tien"] = int64(pick(useBefore, before.Amount, after.Amount))
	}
	if before.Description != after.Description {
		result["noi_dung"] = pick(useBefore, before.Description, after.Description)
	}
	if before.Counterparty != after.Counterparty {
		result["doi_tac"] = pick(useBefore, before.Counterparty, after.Counterparty)
	}
	if before.VoucherNo != after.VoucherNo {
		result["so_chung_tu"] = pick(useBefore, before.VoucherNo, after.VoucherNo)
	}
	// *"AI CHUYỂN CHỨNG TỪ NÀY SANG NGUỒN KHÁC"* IS A QUESTION AN INSPECTION ASKS BY NAME, and the
	// row cannot answer it: the column holds only where the money is attributed NOW. Moving a payment
	// between two sources moves it between two cards of §6 — two figures already read off a screen —
	// and detaching it moves it into the "đã chi nhưng chưa ghi rút từ nguồn nào" warning. Both sides
	// of the pair are recorded, and an empty string on either side is the unattached state, not a
	// missing value.
	if before.FundingSourceID != after.FundingSourceID {
		result["nguon_von_id"] = pick(useBefore, before.FundingSourceID, after.FundingSourceID)
	}
	return result
}

// voucherUnchanged reports whether the edit would change nothing. The state and the four staff codes
// are not compared because no path through UpdateVoucher can change them.
//
// ⚠ EVERY EDITABLE FIELD MUST BE LISTED HERE, AND A MISSING ONE FAILS IN SILENCE. UpdateVoucher returns early
// when this says "nothing moved" — so a field that is editable but unlisted is a field whose edit
// writes NO row, leaves NO audit entry, and does NOT send a confirmed voucher back to `Kế toán nhập`
// (ADR 0036). The request answers 200 with the OLD values and nothing anywhere is red. `nguon_von_id`
// was the sixth field to be added and is the first one this note existed for.
func voucherUnchanged(before, after domain.DisbursementVoucher) bool {
	return before.PaymentDate.Equal(after.PaymentDate) &&
		before.Amount == after.Amount &&
		before.Description == after.Description &&
		before.Counterparty == after.Counterparty &&
		before.VoucherNo == after.VoucherNo &&
		before.FundingSourceID == after.FundingSourceID
}
