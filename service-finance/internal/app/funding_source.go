package app

// The use cases behind the WRITE surface of the funding source catalogue (§6 "Quản lý nguồn vốn",
// migration 0013, user decisions 06/10/2026):
//
//	AddFundingSource  a new source of the commune, with — optionally — its granted amount for one year
//	SetGrantedAmount  record or correct the amount granted to one source for one year
//
// WHY THIS LAYER: rule 6, invariant 3 requires the audit entry to share a transaction with the
// business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes outside
// one. Opening that transaction is this layer's job; the handler translates HTTP and the store knows
// SQL. AddFundingSource is TWO writes (the catalogue row and the year's amount) that land together or not at all.
//
// WHAT IS DELIBERATELY NOT HERE: no rename and no remove of a source — the user decided neither exists.
// A name is therefore an identifier for the life of the commune, which is what lets the audit trail
// file every act under it (see the Subject notes below).

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// FundingSourceStore is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION,
// so the row and its audit entry cannot be written in two.
type FundingSourceStore interface {
	NameTaken(ctx context.Context, tx *store.ScopedTx, name string) (bool, error)
	CountLive(ctx context.Context, tx *store.ScopedTx) (int, error)
	InsertSource(ctx context.Context, tx *store.ScopedTx, id, name string) (int, error)
	SourceForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.NguonVon, error)
	AnnualAmount(ctx context.Context, tx *store.ScopedTx, sourceID string, year int) (domain.FundingSourceAnnualAmount, bool, error)
	InsertAnnualAmount(ctx context.Context, tx *store.ScopedTx, a domain.FundingSourceAnnualAmount) error
	UpdateAnnualAmount(ctx context.Context, tx *store.ScopedTx, id string, amount domain.Dong) error
}

// FundingSources owns adding a commune's funding sources and recording their granted amounts.
type FundingSources struct {
	db    *store.DB
	store FundingSourceStore

	// newID is injected so a test can pin the ids. In production it is ulid.Moi. Called once per row.
	newID func() (string, error)
}

func NewFundingSources(db *store.DB, s FundingSourceStore) *FundingSources {
	return &FundingSources{db: db, store: s, newID: ulid.Moi}
}

// The business verbs written into the trail — Vietnamese snake_case, like every other action this
// service writes (`them_du_an`, `chot_ky_ngan_sach`): an inspection reads these strings.
const (
	ActionFundingSourceCreate     = "them_nguon_von"
	ActionFundingSourceGrantedSet = "ghi_von_duoc_giao_nguon_von"
)

// FundingSourceCreateRequest is one new source as the form sends it.
//
// Year IS REQUIRED EVEN WHEN GrantedAmount IS ABSENT. The form is opened from a year's screen, and the
// reply describes the source AS READ FOR THAT YEAR — the same shape as an item of the year's list. A
// default year would be the one thing every read in this service refuses (store.ErrThieuNamNganSach).
//
// GrantedAmount nil = "left blank": NO year row is written, and every read shows 0 for the year (user
// decision 06/10/2026). A pointer to 0 is different — an explicit "granted nothing this year", which is
// written and audited like any other figure.
type FundingSourceCreateRequest struct {
	Name          string
	Year          int
	GrantedAmount *domain.Dong
}

// AddFundingSource records one new catalogue source and, when given, its granted amount for Year.
//
// RETURNS the source as read for Year: TongNguon is the amount just written, 0 when left blank.
func (uc *FundingSources) AddFundingSource(ctx context.Context, req FundingSourceCreateRequest,
	actor audit.Actor) (domain.NguonVon, error) {

	// Shape first, outside the transaction: a request that fails its shape must never hold a lock.
	name, err := domain.NormaliseFundingSourceName(req.Name)
	if err != nil {
		return domain.NguonVon{}, err
	}
	if err := domain.CheckFundingSourceYear(req.Year); err != nil {
		return domain.NguonVon{}, err
	}
	if req.GrantedAmount != nil {
		if err := domain.CheckGrantedAmount(*req.GrantedAmount); err != nil {
			return domain.NguonVon{}, err
		}
	}
	if err := coNguoiThucHien(actor); err != nil {
		return domain.NguonVon{}, err
	}

	// Ids minted OUTSIDE the transaction: a generator failure must not roll back a write for a reason
	// that has nothing to do with it.
	sourceID, err := uc.newID()
	if err != nil {
		return domain.NguonVon{}, fmt.Errorf("nguon_von: sinh mã: %w", err)
	}
	var amountRow domain.FundingSourceAnnualAmount
	if req.GrantedAmount != nil {
		amountID, err := uc.newID()
		if err != nil {
			return domain.NguonVon{}, fmt.Errorf("nguon_von: sinh mã vốn được giao: %w", err)
		}
		amountRow = domain.FundingSourceAnnualAmount{
			ID: amountID, FundingSourceID: sourceID, Year: req.Year, GrantedAmount: *req.GrantedAmount,
		}
	}

	created := domain.NguonVon{ID: sourceID, Ten: name, Nam: req.Year}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE CEILING BEFORE THE NAME: a full catalogue is a condition of the whole list, a taken name
		// of one value — the caller can act on the first without knowing anything about the second.
		n, err := uc.store.CountLive(ctx, tx)
		if err != nil {
			return err
		}
		if n >= fistore.TranNguonVonMotNam {
			return fistore.ErrFundingSourceCatalogueFull
		}
		taken, err := uc.store.NameTaken(ctx, tx, name)
		if err != nil {
			return err
		}
		if taken {
			return fistore.ErrFundingSourceNameTaken
		}

		order, err := uc.store.InsertSource(ctx, tx, sourceID, name)
		if err != nil {
			return err
		}
		created.ThuTu = order
		if req.GrantedAmount != nil {
			// THE YEAR'S AMOUNT LANDS IN THE SAME TRANSACTION AS THE SOURCE: a source committed without
			// the figure the clerk typed is a card reading "0" that the clerk believes says otherwise.
			if err := uc.store.InsertAnnualAmount(ctx, tx, amountRow); err != nil {
				return err
			}
			created.TongNguon = amountRow.GrantedAmount
		}

		after := map[string]any{
			"funding_source_id": created.ID,
			"name":              created.Ten,
			"order":             created.ThuTu,
			"year":              req.Year,
			// null = left blank (no row written), distinct from an explicit 0.
			"granted_amount": nil,
		}
		if req.GrantedAmount != nil {
			after["granted_amount"] = int64(amountRow.GrantedAmount)
		}
		delta, err := json.Marshal(map[string]any{"sau": after})
		if err != nil {
			return fmt.Errorf("nguon_von: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS BOTH INSERTS (rule 6, invariant 3); TenantID filled from the transaction.
		//
		// THE SUBJECT IS THE SOURCE'S NAME. §11 gives `nguon_von` no `ma`, so there is no issued code
		// (rule 6, invariant 8 wants a business identifier, never the ULID). The name is the next best
		// thing and here it is a sound one: unique within the commune, counting soft-deleted rows
		// (0013), and never renamed (user decision 06/10/2026) — so it names exactly one source for the
		// life of the commune. The id is in the delta for joins. Nothing here is personal data.
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionFundingSourceCreate, Subject: created.Ten, Delta: delta,
		})
	})
	if err != nil {
		// Nothing committed: no source, no amount, no trail.
		return domain.NguonVon{}, wrapFundingSource(ctx, "thêm", err)
	}
	return created, nil
}

// SetGrantedAmount records the amount granted to one source for one year — inserting the year's row
// the first time, correcting it afterwards. PUT semantics: the figure sent IS the figure.
//
// ⚠ ASSUMPTION, STATED BECAUSE NO RULE WAS DECIDED: correcting a PAST year's amount is ALLOWED, audited
// with before and after. Neither the user's decisions of 06/10/2026 nor 0013 lock a year's figure, and
// 0013 left the question to this write path ("NOT FROZEN AFTER ENTRY"). In particular a YEAR CLOSE of
// the budget board (0012) does NOT lock it here — that close is about thu-chi entries, and extending it
// to sources would be a rule nobody chose. Tightening later costs one guard in this function; the
// audit entries already written say exactly which past figures moved, by whom.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING: sending the figure already recorded is not an event, and
// that is what makes the route's idem.KhongCan declaration true. A FIRST write of 0 is NOT a no-op —
// it turns "nobody entered a figure" into "the commune declared 0", and that declaration is recorded.
func (uc *FundingSources) SetGrantedAmount(ctx context.Context, sourceID string, year int,
	amount domain.Dong, actor audit.Actor) (domain.FundingSourceAnnualAmount, error) {

	if sourceID == "" {
		return domain.FundingSourceAnnualAmount{}, fistore.ErrFundingSourceNotFound
	}
	if err := domain.CheckFundingSourceYear(year); err != nil {
		return domain.FundingSourceAnnualAmount{}, err
	}
	if err := domain.CheckGrantedAmount(amount); err != nil {
		return domain.FundingSourceAnnualAmount{}, err
	}
	if err := coNguoiThucHien(actor); err != nil {
		return domain.FundingSourceAnnualAmount{}, err
	}
	newRowID, err := uc.newID()
	if err != nil {
		return domain.FundingSourceAnnualAmount{}, fmt.Errorf("nguon_von: sinh mã vốn được giao: %w", err)
	}

	var result domain.FundingSourceAnnualAmount
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE SOURCE IS LOCKED FIRST — see store.SourceForUpdate: it is what serialises two writers of
		// the same (source, year), the second of which would otherwise insert into the unique key.
		src, err := uc.store.SourceForUpdate(ctx, tx, sourceID)
		if err != nil {
			return err
		}
		before, found, err := uc.store.AnnualAmount(ctx, tx, sourceID, year)
		if err != nil {
			return err
		}
		if found && before.GrantedAmount == amount {
			result = before
			return nil
		}

		var beforeAmount any // null = no figure had been entered for the year
		if found {
			beforeAmount = int64(before.GrantedAmount)
			if err := uc.store.UpdateAnnualAmount(ctx, tx, before.ID, amount); err != nil {
				return err
			}
			result = before
			result.GrantedAmount = amount
		} else {
			result = domain.FundingSourceAnnualAmount{
				ID: newRowID, FundingSourceID: sourceID, Year: year, GrantedAmount: amount,
			}
			if err := uc.store.InsertAnnualAmount(ctx, tx, result); err != nil {
				return err
			}
		}

		// BEFORE AND AFTER (rule 6, invariant 5) — the figure is the denominator of the year's three
		// bars, so a correction moves figures that may already have been reported, and this entry is
		// the only record of what they were measured against before.
		delta, err := json.Marshal(map[string]any{
			"funding_source_id": sourceID,
			"year":              year,
			"truoc":             map[string]any{"granted_amount": beforeAmount},
			"sau":               map[string]any{"granted_amount": int64(amount)},
		})
		if err != nil {
			return fmt.Errorf("nguon_von: mã hoá delta: %w", err)
		}
		// Subject = the source's name, for the reason AddFundingSource gives.
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionFundingSourceGrantedSet, Subject: src.Ten, Delta: delta,
		})
	})
	if err != nil {
		return domain.FundingSourceAnnualAmount{}, wrapFundingSource(ctx, "ghi vốn được giao", err)
	}
	return result, nil
}

// wrapFundingSource wraps a failure with the commune and the operation, and NOTHING ELSE — no name,
// no amount: an error travels into centralised logging across every commune. %w keeps the chain so
// the handler can tell a refusal from a failure. The handler NEVER returns this text to a client.
func wrapFundingSource(ctx context.Context, op string, err error) error {
	return fmt.Errorf("nguon_von: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}
