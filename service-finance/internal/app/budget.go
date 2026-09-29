package app

// The use cases behind the WRITE surface of the commune's budget board
// (docs/ui-ux/07-thu-chi-ngan-sach.md §4, §6, §7).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes
// outside one. Opening that transaction is this layer's job. The handler translates HTTP and nothing
// else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THE TREE. Every decision on this screen is about the SHAPE of the sheet, not
// about the row being written: whether the line has children (the customer's 06/09/2026 rule),
// what its depth is, whether marking it leaves exactly one marked row, whether its parent lost its
// last child. Those need the whole tree AND the transaction at once, which is exactly one place —
// here. Each write therefore locks the sheet, reads the tree under the lock, asks internal/domain,
// and applies or refuses.
//
// ---------------------------------------------------------------------------
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE — said once, here:
//
//	the two sheet kinds, the six roles   migration 0006's CHECK constraints
//	`manual` | `entries` | `children`    migration 0008 (widened 0006's CHECK)
//	one live sheet per year+kind         `UNIQUE (tenant_id, nam, loai, lan)` plus HasLiveSheet
//	hard DELETE refused outright         `ho_so_luu_tru_cam_xoa_cung`
//
// AND ONE PROPERTY WHOSE FLOOR IS MISSING, said plainly rather than implied: "at most one row is
// marked as the total" has NO database constraint (migration 0006 sets out why a partial unique
// index cannot be verified in this build environment). What holds it is SetHeadline's statement
// pair under the sheet's row lock — and, because this will not be the table's only writer forever,
// domain.FullSheet.Headline REFUSES to produce a total when it finds more than one rather than
// summing or choosing.
//
// ---------------------------------------------------------------------------
// WHAT IS NOT BUILT HERE, deliberately:
//
//	the Excel import (§6)   `nguon_tep` / `nap_luc` are the columns it will fill, and CreateSheet takes
//	                        the column set as data for exactly that reason — the parser is a turn of
//	                        its own and it is what fills them.
//	editing a COLUMN       a sheet's columns come from the Phòng Tài chính's file. Renaming one, or
//	                        moving a role from one column to another, changes which figure an
//	                        indicator reads — which is ADR 0035 §A territory and needs its own
//	                        decision about what happens to the periods already reported.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// BudgetStore is the store, declared at the point of use.
//
// EVERY WRITE METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the sheet in
// one transaction and the audit entry in another: there is no signature here that would let you. It
// is also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that the star is a radio — provable without a PostgreSQL. There is none
// reachable from this repository's build environment, so a test that needed one would never run.
type BudgetStore interface {
	SheetByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.BudgetSheet, error)
	FullSheetInTx(ctx context.Context, tx *store.ScopedTx,
		sheet domain.BudgetSheet) (domain.FullSheet, error)
	SheetIDOfLine(ctx context.Context, tx *store.ScopedTx, lineID string) (string, error)
	HasLiveSheet(ctx context.Context, tx *store.ScopedTx, year int, kind domain.SheetKind) (bool, error)
	NextRevision(ctx context.Context, tx *store.ScopedTx, year int, kind domain.SheetKind) (int, error)

	InsertSheet(ctx context.Context, tx *store.ScopedTx, b domain.BudgetSheet) error
	InsertColumn(ctx context.Context, tx *store.ScopedTx, c domain.BudgetColumn) error
	SoftDeleteSheet(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
	UpdateSheet(ctx context.Context, tx *store.ScopedTx, b domain.BudgetSheet) error

	InsertLine(ctx context.Context, tx *store.ScopedTx, k domain.BudgetLine) error
	UpdateLine(ctx context.Context, tx *store.ScopedTx, k domain.BudgetLine) error
	SetMethod(ctx context.Context, tx *store.ScopedTx, id string, method domain.LineMethod) error
	SetHeadline(ctx context.Context, tx *store.ScopedTx, sheetID, id string) error
	SoftDeleteLine(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
	WriteValue(ctx context.Context, tx *store.ScopedTx, lineID, columnID string, value *domain.Dong) error

	// The batches (migration 0008) — budget_entry.go in this package.
	HasLiveEntries(ctx context.Context, tx *store.ScopedTx, lineID string) (bool, error)
	CountLiveEntries(ctx context.Context, tx *store.ScopedTx, lineID string) (int, error)
	EntryByIDInTx(ctx context.Context, tx *store.ScopedTx, id string) (domain.BudgetEntry, error)
	InsertEntry(ctx context.Context, tx *store.ScopedTx, d domain.BudgetEntry) error
	InsertEntryAmount(ctx context.Context, tx *store.ScopedTx, entryID, columnID string, value domain.Dong) error
	SoftDeleteEntry(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
}

// BudgetService owns creating, removing, and editing one commune's budget board.
type BudgetService struct {
	db   *store.DB
	repo BudgetStore

	// newID is injected so a test can pin the id. In production it is ulid.Moi.
	newID func() (string, error)
}

func NewBudgetService(db *store.DB, repo BudgetStore) *BudgetService {
	return &BudgetService{db: db, repo: repo, newID: ulid.Moi}
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes: an inspection reads these strings, and a function name would tell them
// nothing.
//
// SIX VERBS AND NOT ONE `sua_ngan_sach`, because they are six administrative acts with six
// consequences. `dat_dong_tong_ngan_sach` in particular is the one worth searching for by name — it
// is the record of somebody choosing which row the commune's reported total comes from, and ADR
// 0035 §A is the reason that choice is a person's and not the software's.
const (
	ActionCreateBudgetSheet = "tao_bang_ngan_sach"
	ActionRemoveBudgetSheet = "go_bang_ngan_sach"
	ActionUpdateBudgetSheet = "sua_bang_ngan_sach"
	ActionCreateBudgetLine  = "them_khoan_muc_ngan_sach"
	ActionUpdateBudgetLine  = "sua_khoan_muc_ngan_sach"
	ActionRemoveBudgetLine  = "go_khoan_muc_ngan_sach"
	ActionSetBudgetHeadline = "dat_dong_tong_ngan_sach"
	ActionRecordBudgetEntry = "ghi_dot_thu_chi"
	ActionRemoveBudgetEntry = "go_dot_thu_chi"
)

// --- creating a sheet ------------------------------------------------------------------------------

// CreateBudgetSheetRequest is one new sheet together with its columns.
//
// THE COLUMNS ARRIVE WITH THE SHEET AND NOT AFTERWARDS, and that is not a convenience. §3's closing
// note makes columns DATA, and ADR 0035 §A makes two of them load-bearing: a sheet that existed for
// a while with no `Dự toán TP giao` column is a sheet whose indicator was absent for that while,
// with figures typed into it meanwhile. One act, one transaction, one audit entry describing the
// shape the commune actually created.
//
// THERE IS NO `Code`, NO `Revision` AND NO `Id` FIELD. All three are the system's: `lan` comes from what is
// already in the table and `ma` is built from it. A client-supplied code on a table whose uniqueness
// counts soft-deleted rows is a client that can permanently burn a code.
type CreateBudgetSheetRequest struct {
	Year         int
	Kind         domain.SheetKind
	Title        string
	Unit         string    // one of the three wire codes — domain.ValidateUnit
	CumulativeTo time.Time // zero when the commune has not stated it

	// Columns carries Name, SortOrder, Format, Formula and Indicator. ID and SheetID are filled here.
	Columns []domain.BudgetColumn
}

// CreateSheet creates one sheet and its columns.
//
// THE SHAPE IS VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never
// hold a row lock while doing so, and the caller needs the reason rather than a rollback.
func (uc *BudgetService) CreateSheet(ctx context.Context, req CreateBudgetSheetRequest,
	actor audit.Actor) (domain.BudgetSheet, error) {

	if err := domain.ValidateSheetKind(req.Kind); err != nil {
		return domain.BudgetSheet{}, err
	}
	if err := domain.ValidateBudgetYear(req.Year); err != nil {
		return domain.BudgetSheet{}, err
	}
	title, err := domain.NormalizeSheetTitle(req.Title)
	if err != nil {
		return domain.BudgetSheet{}, err
	}
	unit, err := domain.ValidateUnit(req.Unit)
	if err != nil {
		return domain.BudgetSheet{}, err
	}

	column := make([]domain.BudgetColumn, 0, len(req.Columns))
	for _, c := range req.Columns {
		name, err := domain.NormalizeColumnName(c.Name)
		if err != nil {
			return domain.BudgetSheet{}, err
		}
		formula, err := domain.NormalizeFormula(c.Formula)
		if err != nil {
			return domain.BudgetSheet{}, err
		}
		if err := domain.ValidateBudgetSortOrder(c.SortOrder); err != nil {
			return domain.BudgetSheet{}, err
		}
		column = append(column, domain.BudgetColumn{
			Name: name, SortOrder: c.SortOrder, Format: c.Format, Formula: formula, Indicator: c.Indicator,
		})
	}
	// THE WHOLE SET AT ONCE, because the duplicate-role check cannot be made per column: two columns
	// both marked `Thu xã hưởng` is a sheet where the `Cân đối` cell has two candidate answers.
	if err := domain.ValidateColumnSet(req.Kind, column); err != nil {
		return domain.BudgetSheet{}, err
	}
	if err := requireActor(actor); err != nil {
		return domain.BudgetSheet{}, err
	}

	id, err := uc.newID()
	if err != nil {
		return domain.BudgetSheet{}, fmt.Errorf("ngan_sach: sinh mã bảng: %w", err)
	}

	next := domain.BudgetSheet{
		ID: id, Year: req.Year, Kind: req.Kind,
		// The LABEL is what the column stores — see domain.SheetUnit for why code and stored text differ.
		Title: title, Unit: unit.Label(), CumulativeTo: req.CumulativeTo,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// REFUSED RATHER THAN STACKED. §6's `🗑 Gỡ` is how a year's sheet is replaced; a second live
		// sheet for one year and kind would give that year two answers with nothing on either screen
		// saying which the report was built from.
		exists, err := uc.repo.HasLiveSheet(ctx, tx, req.Year, req.Kind)
		if err != nil {
			return err
		}
		if exists {
			return fistore.ErrSheetExists
		}

		revision, err := uc.repo.NextRevision(ctx, tx, req.Year, req.Kind)
		if err != nil {
			return err
		}
		next.Revision = revision
		next.Code = sheetCode(req.Year, req.Kind, revision)

		if err := uc.repo.InsertSheet(ctx, tx, next); err != nil {
			return err
		}
		for i := range column {
			columnID, err := uc.newID()
			if err != nil {
				return fmt.Errorf("ngan_sach: sinh mã cột: %w", err)
			}
			column[i].ID = columnID
			column[i].SheetID = next.ID
			if err := uc.repo.InsertColumn(ctx, tx, column[i]); err != nil {
				return err
			}
		}

		delta, err := json.Marshal(map[string]any{"sau": sheetSummary(next, column)})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERTS (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4).
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionCreateBudgetSheet, Subject: next.Code, Delta: delta,
		})
	})
	if err != nil {
		// Nothing was committed: no sheet, no columns, no trail. The states agree.
		return domain.BudgetSheet{}, wrapBudgetErr(ctx, "tạo bảng", err)
	}
	return next, nil
}

// sheetCode builds the sheet's business code — `NS-2026-CHI-01`.
//
// IT IS BUILT AND NOT TYPED, and the two halves are why. It has to be UNIQUE FOREVER, counting
// removed sheets (rule 7, invariant 3), which a person cannot be asked to guarantee; and it is what
// `audit_log.subject` holds for every write on this board, so it has to be readable years later by
// somebody handling an inspection (rule 6, invariant 8). `NS-2026-CHI-01` names the thing; a ULID
// names nobody.
func sheetCode(year int, kind domain.SheetKind, revision int) string {
	return fmt.Sprintf("NS-%d-%s-%02d", year, strings.ToUpper(string(kind)), revision)
}

// RemoveSheet soft deletes one whole sheet — §6's `🗑 Gỡ`, the button whose own dialog warns it cannot be
// undone.
//
// "CANNOT BE UNDONE" IS TRUE ON THE SCREEN AND FALSE IN THE DATABASE, on purpose. The row stays with
// `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1) and its whole tree stays with
// it; what the commune loses is the sheet being ON any screen or in any total, which is what they
// asked for. Nothing is destroyed, and `ho_so_luu_tru_cam_xoa_cung` refuses a hard delete outright.
//
// THE REASON IS MANDATORY. A year's budget figures that vanished from every report with no reason
// attached is a question somebody will ask and nobody can answer — and the rows are still there, so
// it WILL be asked.
func (uc *BudgetService) RemoveSheet(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return fistore.ErrBudgetSheetNotFound
	}
	reason, err := domain.NormalizeBudgetRemoveReason(rawReason)
	if err != nil {
		return err
	}
	if err := requireActor(actor); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.SheetByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds. Two
		// kinds of identifier in one column is a column nobody can query (rule 6, invariant 8).
		if err := uc.repo.SoftDeleteSheet(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{
			"bang_id": before.ID,
			"truoc":   sheetSummary(before, nil),
			"ly_do":   reason,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionRemoveBudgetSheet, Subject: before.Code, Delta: delta,
		})
	})
	if err != nil {
		return wrapBudgetErr(ctx, "gỡ bảng", err)
	}
	return nil
}

// UpdateBudgetSheetRequest is a PARTIAL edit of a sheet's header: a nil pointer means "leave this alone".
//
// THREE FIELDS AND NO MORE. `Year`, `Kind`, `Code`, `Revision` decide which report the figures belong to and
// the code the trail is filed under; the columns decide which figure an indicator reads (ADR 0035
// §A). None of them is an edit of this act.
type UpdateBudgetSheetRequest struct {
	Title *string

	// CumulativeTo non-nil and ZERO clears the cut-off date — "the commune has not stated it" is a real
	// state of this column (store.timeOrNil writes it as NULL).
	CumulativeTo *time.Time

	// Unit is one of the three wire codes. CHANGING IT CHANGES ONLY HOW THE FIGURES ARE
	// DISPLAYED: every stored figure is đồng and none is rescaled here (domain.SheetUnit).
	Unit *string
}

// UpdateSheet edits one sheet's title, display unit and cut-off date.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING, the same property UpdateLine holds and for the same
// two reasons: an entry saying nothing changed buries the ones carrying legal weight, and it is what
// makes the route's idem.KhongCan declaration true.
//
// THE UNIT IS COMPARED AS THE STORED LABEL. A legacy sheet holding free text ("tr.đồng") that is set
// to `trieu-dong` DOES change — its column becomes the canonical label — and that is recorded, with
// the old text in `truoc`, because the screen will print its figures differently from that moment.
func (uc *BudgetService) UpdateSheet(ctx context.Context, id string, req UpdateBudgetSheetRequest,
	actor audit.Actor) (domain.BudgetSheet, error) {

	if id == "" {
		return domain.BudgetSheet{}, fistore.ErrBudgetSheetNotFound
	}
	// Shape first, outside the transaction: a request that fails its shape must never hold the
	// sheet's row lock while doing so.
	var title, unit string
	var err error
	if req.Title != nil {
		if title, err = domain.NormalizeSheetTitle(*req.Title); err != nil {
			return domain.BudgetSheet{}, err
		}
	}
	if req.Unit != nil {
		d, err := domain.ValidateUnit(*req.Unit)
		if err != nil {
			return domain.BudgetSheet{}, err
		}
		unit = d.Label()
	}
	if err := requireActor(actor); err != nil {
		return domain.BudgetSheet{}, err
	}

	var after domain.BudgetSheet
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// FOR UPDATE, and it excludes soft-deleted rows: a removed sheet and another commune's sheet
		// both answer ErrBudgetSheetNotFound, which the handler turns into one 404 body.
		before, err := uc.repo.SheetByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		after = before
		if req.Title != nil {
			after.Title = title
		}
		if req.Unit != nil {
			after.Unit = unit
		}
		if req.CumulativeTo != nil {
			after.CumulativeTo = *req.CumulativeTo
		}

		if before.Title == after.Title && before.Unit == after.Unit &&
			before.CumulativeTo.Equal(after.CumulativeTo) {
			return nil
		}
		if err := uc.repo.UpdateSheet(ctx, tx, after); err != nil {
			return err
		}

		// BEFORE AND AFTER OF ALL THREE FIELDS (rule 6, invariant 5). Three short values, none of
		// them personal data; recording all three rather than only the moved ones lets an inspector
		// read the sheet's whole header at that instant from one entry.
		delta, err := json.Marshal(map[string]any{
			"bang_id": before.ID,
			"truoc":   sheetHeader(before),
			"sau":     sheetHeader(after),
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionUpdateBudgetSheet, Subject: before.Code, Delta: delta,
		})
	})
	if err != nil {
		return domain.BudgetSheet{}, wrapBudgetErr(ctx, "sửa bảng", err)
	}
	return after, nil
}

// sheetHeader is the audit delta's view of a sheet's editable header.
func sheetHeader(b domain.BudgetSheet) map[string]any {
	var cumulativeTo any
	if !b.CumulativeTo.IsZero() {
		cumulativeTo = b.CumulativeTo.Format(time.DateOnly)
	}
	return map[string]any{"tieu_de": b.Title, "don_vi_tinh": b.Unit, "luy_ke_den": cumulativeTo}
}

// --- the lines -------------------------------------------------------------------------------------

// CreateBudgetLineRequest is one new line of the tree.
//
// THERE IS NO `Method`, NO `Level` AND NO `IsHeadline` FIELD, AND THERE MUST NEVER BE ONE.
//
//	Method      follows from the tree (domain.MethodFromTree). A field here is a field a request
//	            body can fill, and what that buys is a PARENT marked `manual` — the direct entry the
//	            customer's 06/09/2026 decision exists to forbid.
//	Level       is the parent's depth plus one. A field is a second answer to a question the tree
//	            already answers, and the screen indents by it.
//	IsHeadline  is the star. Creating a line already marked would be a second marked row arriving
//	            with no act recorded, which is exactly the double count ADR 0035 §A turns on.
type CreateBudgetLineRequest struct {
	SheetID      string
	ParentID     string // "" at the top level
	OrdinalLabel string
	Name         string
	SortOrder    int
}

// CreateLine adds one line — `＋ Thêm khoản mục con` and `⊞ Thêm khoản mục cấp cao nhất` (§4.3).
//
// IT MAY CHANGE ITS PARENT TOO, and that is the customer's rule taking effect: a leaf that gains its
// first child stops being typed into and starts summing. The flip is written in the SAME
// transaction, because a tree in which a parent still says `manual` while having a child is a tree
// whose figures are read from a locked cell.
//
// THE PARENT'S OWN CELLS ARE LEFT WHERE THEY ARE. §9 rule 1: switching to a computed mode LOCKS the
// typed figures, it does not erase them — and that is what makes the reverse move (RemoveLine's last
// child) able to give the commune something back rather than a blank row.
func (uc *BudgetService) CreateLine(ctx context.Context, req CreateBudgetLineRequest,
	actor audit.Actor) (domain.BudgetLine, error) {

	if req.SheetID == "" {
		return domain.BudgetLine{}, domain.ErrSheetMissing
	}
	name, err := domain.NormalizeLineName(req.Name)
	if err != nil {
		return domain.BudgetLine{}, err
	}
	ordinalLabel, err := domain.NormalizeOrdinalLabel(req.OrdinalLabel)
	if err != nil {
		return domain.BudgetLine{}, err
	}
	if err := domain.ValidateBudgetSortOrder(req.SortOrder); err != nil {
		return domain.BudgetLine{}, err
	}
	if err := requireActor(actor); err != nil {
		return domain.BudgetLine{}, err
	}

	id, err := uc.newID()
	if err != nil {
		return domain.BudgetLine{}, fmt.Errorf("ngan_sach: sinh mã khoản mục: %w", err)
	}

	next := domain.BudgetLine{
		ID: id, SheetID: req.SheetID, ParentID: req.ParentID, OrdinalLabel: ordinalLabel, Name: name, SortOrder: req.SortOrder,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		sheet, full, err := uc.lockAndReadTree(ctx, tx, req.SheetID)
		if err != nil {
			return err
		}

		var parent domain.BudgetLine
		hasParent := req.ParentID != ""
		if hasParent {
			var found bool
			parent, found = full.ByID(req.ParentID)
			if !found {
				// A parent that is not in THIS sheet's live tree: it does not exist, it was removed,
				// or it belongs to another sheet. One answer for all three — accepting it would put
				// one year's line under another year's tree and total them together.
				return domain.ErrParentInOtherSheet
			}
			// A PARENT IN `entries` MODE WITH LIVE BATCHES IS REFUSED, not flipped (decided 25/09/2026
			// under the user's "follow the recommendations"; migration 0008 question 4b left it open).
			// Flipped, it would become `children` and every batch in its dialog would silently stop
			// counting. Without live batches it flips below exactly like a `manual` leaf.
			if parent.Method == domain.MethodEntries {
				hasEntries, err := uc.repo.HasLiveEntries(ctx, tx, parent.ID)
				if err != nil {
					return err
				}
				if hasEntries {
					return domain.ErrEntriesLineHasEntries
				}
			}
		}
		// A NEW LINE IS ALWAYS A LEAF, so its calculation mode is `manual` and its depth is the
		// parent's plus one. Both derived here and never taken from the request.
		next.Level = domain.LevelUnder(parent, hasParent)
		next.Method = domain.MethodFromTree(false)

		if err := uc.repo.InsertLine(ctx, tx, next); err != nil {
			return err
		}

		parentMethodChange := false
		if hasParent && parent.Method != domain.MethodChildren {
			if err := uc.repo.SetMethod(ctx, tx, parent.ID, domain.MethodChildren); err != nil {
				return err
			}
			parentMethodChange = true
		}

		body := map[string]any{"sau": lineSummary(next)}
		if parentMethodChange {
			// RECORDED BECAUSE IT IS A CHANGE THE COMMUNE DID NOT ASK FOR. A parent that silently
			// stopped accepting typed figures is the single most confusing thing this screen can do,
			// and the entry is what lets somebody answer "why did that row stop being editable".
			body["cha_chuyen_sang_cong_tu_dong_con"] = map[string]any{
				"khoan_muc_id": parent.ID, "ten": parent.Name, "cach_tinh_truoc": string(parent.Method),
			}
		}
		delta, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionCreateBudgetLine, Subject: sheet.Code, Delta: delta,
		})
	})
	if err != nil {
		return domain.BudgetLine{}, wrapBudgetErr(ctx, "thêm khoản mục", err)
	}
	return next, nil
}

// UpdateBudgetLineRequest is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. `OrdinalLabel` is optional and its empty string is a MEANINGFUL
// value — §4.1's own sample has a row with no reference at all — so a struct of plain values cannot
// tell "the client did not mention this" from "the client cleared it", and a dialog editing only the
// name would wipe the reference off a budget line.
//
// `SheetID`, `ParentID`, `Level` AND `IsHeadline` ARE ABSENT AND NONE IS AN OVERSIGHT. Moving a line
// between sheets or under another parent moves its figure between two totals that have already been
// read off a screen; the other two are derived or have their own act.
type UpdateBudgetLineRequest struct {
	OrdinalLabel *string
	Name         *string
	SortOrder    *int

	// Method is the LEAF's choice between `manual` and `entries` (user decision 25/09/2026, §4.2).
	// `children` is never accepted — it follows the tree — and a line WITH children refuses both.
	//
	// WHAT A SWITCH DOES TO THE STORED FIGURES (§9.1), stated because the two directions differ:
	//
	//	manual  -> entries   the typed cells are LEFT UNTOUCHED and simply stop being displayed.
	//	entries -> manual    every number cell is OVERWRITTEN with the batch sum that was on the
	//	                     screen (NULL where the sum was empty), in the same transaction, audited.
	//
	// So the typed figures from before an entries period never come back: switching back to manual
	// shows what the batches added up to, which is §9.1's "giữ giá trị vừa tính làm giá trị khởi đầu".
	Method *domain.LineMethod

	// Values is columnID -> figure, and a nil VALUE means "clear this cell" (§9 rule 4: empty is a state
	// the screen draws as `—`, not the absence of a record). A columnID that is absent from the map is
	// a cell this request does not mention.
	Values map[string]*domain.Dong
}

// UpdateLine edits one line's text and its figures.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending a line the values it already has is not an
// event; recording it would fill a public authority's ledger with entries saying nothing changed,
// and those are the entries that bury the ones carrying legal weight. It is also what makes this
// route genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
//
// A FIGURE TYPED INTO A PARENT IS REFUSED — the customer's decision of 06/09/2026, enforced here and
// not only on the screen, because a screen-only rule holds until somebody calls the API directly.
// domain.ErrParentLineNoDirectValue carries the sentence, and migration 0006 carries the price the
// customer accepted with it.
func (uc *BudgetService) UpdateLine(ctx context.Context, id string, req UpdateBudgetLineRequest,
	actor audit.Actor) (domain.BudgetLine, error) {

	if id == "" {
		return domain.BudgetLine{}, domain.ErrLineNotFound
	}
	// Shape first, outside the transaction, for the same reason as CreateLine.
	var name, ordinalLabel string
	var err error
	if req.Name != nil {
		if name, err = domain.NormalizeLineName(*req.Name); err != nil {
			return domain.BudgetLine{}, err
		}
	}
	if req.OrdinalLabel != nil {
		if ordinalLabel, err = domain.NormalizeOrdinalLabel(*req.OrdinalLabel); err != nil {
			return domain.BudgetLine{}, err
		}
	}
	if req.SortOrder != nil {
		if err := domain.ValidateBudgetSortOrder(*req.SortOrder); err != nil {
			return domain.BudgetLine{}, err
		}
	}
	for _, g := range req.Values {
		if g == nil {
			continue
		}
		if err := domain.ValidateValue(*g); err != nil {
			return domain.BudgetLine{}, err
		}
	}
	if req.Method != nil {
		if err := domain.ValidateChosenMethod(*req.Method); err != nil {
			return domain.BudgetLine{}, err
		}
	}
	if err := requireActor(actor); err != nil {
		return domain.BudgetLine{}, err
	}

	var after domain.BudgetLine
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		sheetID, err := uc.repo.SheetIDOfLine(ctx, tx, id)
		if err != nil {
			return err
		}
		sheet, full, err := uc.lockAndReadTree(ctx, tx, sheetID)
		if err != nil {
			return err
		}
		before, found := full.ByID(id)
		if !found {
			return domain.ErrLineNotFound
		}

		after = before
		if req.OrdinalLabel != nil {
			after.OrdinalLabel = ordinalLabel
		}
		if req.Name != nil {
			after.Name = name
		}
		if req.SortOrder != nil {
			after.SortOrder = *req.SortOrder
		}

		// THE MODE, DECIDED BEFORE ANY WRITE so every refusal below leaves nothing behind.
		methodChanged := req.Method != nil && *req.Method != before.Method
		if req.Method != nil && full.HasChildren(before.ID) {
			return domain.ErrParentLineMethodFixed
		}
		if methodChanged {
			after.Method = *req.Method
		}
		// §4.2: an `entries` line's cells are read-only. Judged on the mode AFTER this request, so
		// "switch to manual and type a figure" in one PATCH is accepted and "switch to entries and
		// type a figure" is not.
		if len(req.Values) > 0 && after.Method == domain.MethodEntries && !full.HasChildren(before.ID) {
			return domain.ErrEntriesLineNoDirectValue
		}

		var copiedFromEntries []map[string]any
		if methodChanged {
			if before.Method == domain.MethodEntries && after.Method == domain.MethodManual {
				if full.Values == nil {
					full.Values = map[string]map[string]domain.Dong{}
				}
				if copiedFromEntries, err = uc.copyEntryTotalsIntoCells(ctx, tx, full, before.ID); err != nil {
					return err
				}
			}
			if err := uc.repo.SetMethod(ctx, tx, before.ID, after.Method); err != nil {
				return err
			}
		}

		// THE FIGURES, AND THE CUSTOMER'S RULE IN FRONT OF THEM. The check is on the LINE, not on
		// each cell: a parent is a parent for every column at once.
		changedCells, err := uc.writeCells(ctx, tx, full, before, req.Values)
		if err != nil {
			return err
		}

		textChanged := before.OrdinalLabel != after.OrdinalLabel || before.Name != after.Name || before.SortOrder != after.SortOrder
		if !textChanged && len(changedCells) == 0 && !methodChanged {
			return nil
		}
		if textChanged {
			if err := uc.repo.UpdateLine(ctx, tx, after); err != nil {
				return err
			}
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column of every cell on every edit makes the one figure somebody actually changed
		// impossible to find in a ledger that is never deleted — and on this screen the figure IS the
		// record: it is what goes into the document sent to the higher authority.
		body := map[string]any{"khoan_muc_id": before.ID}
		if textChanged {
			body["truoc"] = lineDiff(before, after, true)
			body["sau"] = lineDiff(before, after, false)
		}
		if len(changedCells) > 0 {
			body["o"] = changedCells
		}
		if methodChanged {
			// THE SWITCH IS RECORDED WITH WHAT IT DID TO THE CELLS. entries -> manual rewrites the
			// line's figures from the batch sums, and "why did these cells change" is answered only here.
			body["cach_tinh"] = map[string]any{
				"truoc": string(before.Method), "sau": string(after.Method),
			}
			if len(copiedFromEntries) > 0 {
				body["o_lay_tu_tong_dot"] = copiedFromEntries
			}
		}
		delta, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionUpdateBudgetLine, Subject: sheet.Code, Delta: delta,
		})
	})
	if err != nil {
		return domain.BudgetLine{}, wrapBudgetErr(ctx, "sửa khoản mục", err)
	}
	return after, nil
}

// copyEntryTotalsIntoCells is §9.1's entries -> manual hand-over: every NUMBER cell of the line becomes the
// batch sum that was on the screen a moment ago — the sum, or NULL where the sum was empty.
//
// NULL AND NOT "LEAVE THE OLD TYPED FIGURE": a typed figure from before the entries period would
// otherwise reappear in a column the screen showed as `—`, and the commune would see a number nobody
// entered for this period. Only cells that actually change are written.
//
// It UPDATES full.Values for the line, so a `values` map in the same request is compared against the
// figures the line holds after the hand-over, not before.
func (uc *BudgetService) copyEntryTotalsIntoCells(ctx context.Context, tx *store.ScopedTx, full domain.FullSheet,
	lineID string) ([]map[string]any, error) {

	// full.Values is non-nil (the caller guarantees it), so this inner map is shared with the caller's copy.
	if full.Values[lineID] == nil {
		full.Values[lineID] = map[string]domain.Dong{}
	}
	var changes []map[string]any
	for _, column := range full.Columns {
		if column.Format != domain.ColumnFormatNumber {
			continue
		}
		if full.EntryTotalOverflow[lineID][column.ID] {
			// The sum does not even fit int64. It cannot become a typed figure; the way out is to
			// remove the batch recorded in error, which RemoveEntry still allows.
			return nil, domain.ErrEntryTotalOverflow
		}
		total, hasTotal := full.EntryTotals[lineID][column.ID]
		var next *domain.Dong
		if hasTotal {
			// The typo guard every typed cell passes. A batch sum beyond it is not a budget figure,
			// and copying it would put into a manual cell what no person could have typed there.
			// Wrapped in BOTH sentinels: it is the batch sum that is wrong (409, remove a batch), and
			// the reason is the same bound a typed cell meets.
			if err := domain.ValidateValue(total); err != nil {
				return nil, fmt.Errorf("%w: %w", domain.ErrEntryTotalOverflow, err)
			}
			g := total
			next = &g
		}
		old, hadOld := full.Values[lineID][column.ID]
		if cellUnchanged(old, hadOld, next) {
			continue
		}
		if err := uc.repo.WriteValue(ctx, tx, lineID, column.ID, next); err != nil {
			return nil, err
		}
		changes = append(changes, map[string]any{
			"cot_id": column.ID, "cot": column.Name,
			"truoc": numberOrNil(old, hadOld), "sau": numberPtrOrNil(next),
		})
		if hasTotal {
			full.Values[lineID][column.ID] = total
		} else {
			delete(full.Values[lineID], column.ID)
		}
	}
	return changes, nil
}

// writeCells writes the cells this request mentions and returns a before/after record of the ones that
// actually moved.
//
// THREE REFUSALS, ALL BEFORE ANY WRITE:
//
//	the line has children     domain.CanWriteValue — the customer's 06/09/2026 decision.
//	the column is not this     a figure filed against another sheet's column is a figure on no
//	  sheet's                  screen, sitting in the table looking healthy.
//	the column is `phan_tram`  §9 rule 3: a percentage is computed at render, never stored. A stored
//	                           one is a second home for a derivable number, and the stale copy is the
//	                           one that reaches the report.
func (uc *BudgetService) writeCells(ctx context.Context, tx *store.ScopedTx, full domain.FullSheet,
	k domain.BudgetLine, o map[string]*domain.Dong) ([]map[string]any, error) {

	if len(o) == 0 {
		return nil, nil
	}
	if err := domain.CanWriteValue(full.HasChildren(k.ID)); err != nil {
		return nil, err
	}

	// SORTED BY THE SHEET'S OWN COLUMN ORDER rather than by map iteration, so the audit delta of one
	// edit reads the same way twice and two runs of a test compare equal.
	var changes []map[string]any
	for _, column := range full.Columns {
		next, mentioned := o[column.ID]
		if !mentioned {
			continue
		}
		if column.Format != domain.ColumnFormatNumber {
			return nil, domain.ErrColumnNotNumber
		}
		old, hadOld := full.Values[k.ID][column.ID]
		if cellUnchanged(old, hadOld, next) {
			continue
		}
		if err := uc.repo.WriteValue(ctx, tx, k.ID, column.ID, next); err != nil {
			return nil, err
		}
		changes = append(changes, map[string]any{
			"cot_id": column.ID,
			"cot":    column.Name,
			"truoc":  numberOrNil(old, hadOld),
			"sau":    numberPtrOrNil(next),
		})
	}
	// EVERY MENTIONED COLUMN MUST BE ONE OF THIS SHEET'S. Counting what was matched is how a column
	// id belonging to another sheet is caught — the loop above simply never sees it, and a silent
	// "wrote nothing" would tell the client its figure was saved.
	if len(o) != countSheetCells(full, o) {
		return nil, domain.ErrColumnInOtherSheet
	}
	return changes, nil
}

func countSheetCells(full domain.FullSheet, o map[string]*domain.Dong) int {
	n := 0
	for _, column := range full.Columns {
		if _, ok := o[column.ID]; ok {
			n++
		}
	}
	return n
}

func cellUnchanged(old domain.Dong, hadOld bool, next *domain.Dong) bool {
	if next == nil {
		return !hadOld
	}
	return hadOld && old == *next
}

func numberOrNil(g domain.Dong, ok bool) any {
	if !ok {
		return nil
	}
	return int64(g)
}

func numberPtrOrNil(g *domain.Dong) any {
	if g == nil {
		return nil
	}
	return int64(*g)
}

// RemoveLine soft deletes one line — `🗑 Gỡ khoản mục` (§4.1).
//
// A LINE WITH CHILDREN IS REFUSED rather than cascaded (domain.ErrLineHasChildren). A cascade
// takes a whole branch off every total in one click, and the rows are archival: they stay in the
// table carrying `deleted_at`, so "undo" is not a button, it is re-entering them one by one. Making
// the commune remove the children first makes the size of the act visible while it is still being
// decided.
//
// REMOVING THE LAST CHILD GIVES THE PARENT ITS FIGURES BACK, and that is §9 rule 1's second half:
// "đổi ngược lại thì giữ giá trị vừa tính làm giá trị khởi đầu". The parent returns to `manual` and
// each of its numeric cells is written with the total that was on the screen a moment ago —
// otherwise the row would go blank, and blank on this screen means "nobody has entered this", which
// is not what happened.
func (uc *BudgetService) RemoveLine(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return domain.ErrLineNotFound
	}
	reason, err := domain.NormalizeBudgetRemoveReason(rawReason)
	if err != nil {
		return err
	}
	if err := requireActor(actor); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		sheetID, err := uc.repo.SheetIDOfLine(ctx, tx, id)
		if err != nil {
			return err
		}
		sheet, full, err := uc.lockAndReadTree(ctx, tx, sheetID)
		if err != nil {
			return err
		}
		before, found := full.ByID(id)
		if !found {
			return domain.ErrLineNotFound
		}
		if err := domain.CanRemove(full.HasChildren(id)); err != nil {
			return err
		}

		// COMPUTED BEFORE THE REMOVAL, because after it the parent has no children and the figure
		// this is trying to preserve no longer exists anywhere.
		var restored []map[string]any
		parentBackToManual := before.ParentID != "" && len(full.DirectChildren(before.ParentID)) == 1
		kept := map[string]domain.Dong{}
		// uncomputable lists the columns whose parent figure was UNAVAILABLE (domain.FullSheet.Value)
		// at the moment of removal. Nothing is written for them — there is no honest number to keep —
		// and the entry says so, so an empty parent cell afterwards is explained rather than looking
		// like "nobody entered this". The removal itself is NOT refused: removing the offending child
		// may be exactly the fix.
		var uncomputable []string
		if parentBackToManual {
			for _, column := range full.Columns {
				if column.Format != domain.ColumnFormatNumber {
					continue
				}
				s := full.Value(before.ParentID, column.ID)
				switch {
				case s.Present:
					kept[column.ID] = s.Value
				case s.Reason != "":
					uncomputable = append(uncomputable, column.ID)
				}
			}
		}

		if err := uc.repo.SoftDeleteLine(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}

		if parentBackToManual {
			if err := uc.repo.SetMethod(ctx, tx, before.ParentID, domain.MethodManual); err != nil {
				return err
			}
			for _, column := range full.Columns {
				g, ok := kept[column.ID]
				if !ok {
					continue
				}
				keptValue := g
				if err := uc.repo.WriteValue(ctx, tx, before.ParentID, column.ID, &keptValue); err != nil {
					return err
				}
				restored = append(restored, map[string]any{"cot_id": column.ID, "gia_tri": int64(g)})
			}
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication rule 9
		// forbids: the column is the current state of the row and can only ever hold the FIRST
		// removal, while the entry is the append-only record of the act.
		body := map[string]any{
			"khoan_muc_id": before.ID,
			"truoc":        lineSummary(before),
			"ly_do":        reason,
			"xoa_mem":      true,
		}
		if parentBackToManual {
			parentBack := map[string]any{
				"khoan_muc_id": before.ParentID,
				"giu_lai":      restored,
			}
			if len(uncomputable) > 0 {
				parentBack["cot_khong_tinh_duoc"] = uncomputable
			}
			body["cha_ve_nhap_tay"] = parentBack
		}
		delta, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionRemoveBudgetLine, Subject: sheet.Code, Delta: delta,
		})
	})
	if err != nil {
		return wrapBudgetErr(ctx, "gỡ khoản mục", err)
	}
	return nil
}

// SetHeadline marks one line as the sheet's total row — the `☆` of §4.1.
//
// THE MOST CONSEQUENTIAL ROUTE ON THIS SCREEN, and the shortest. It decides which row every summary
// cell and both indicators are read from, which decides the figures the commune puts in a document
// sent upward. ADR 0035 §A is why it is a person's act at all: no ordering, no depth and no name
// pattern can tell the thu sheet's two nested top-level rows from the chi sheet's `Tổng số` sitting
// beside A…E, and getting it wrong double-counts on one form while looking right on the other.
//
// IT IS A RADIO AND THE STORE MAKES IT ONE: SetHeadline clears every other row of the sheet and sets
// this one, both inside this transaction and under the sheet's row lock. THERE IS NO "UNMARK"
// ROUTE — §4.1 draws the star as something that MOVES ("bấm ngôi sao ở đầu một dòng khác để đổi"),
// and a sheet that had a total and then deliberately had none is a state nothing on that screen
// asks for. A sheet loses its total only by the marked row being removed, which releases the flag.
func (uc *BudgetService) SetHeadline(ctx context.Context, id string,
	actor audit.Actor) (domain.BudgetLine, error) {

	if id == "" {
		return domain.BudgetLine{}, domain.ErrLineNotFound
	}
	if err := requireActor(actor); err != nil {
		return domain.BudgetLine{}, err
	}

	var after domain.BudgetLine
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		sheetID, err := uc.repo.SheetIDOfLine(ctx, tx, id)
		if err != nil {
			return err
		}
		sheet, full, err := uc.lockAndReadTree(ctx, tx, sheetID)
		if err != nil {
			return err
		}
		before, found := full.ByID(id)
		if !found {
			return domain.ErrLineNotFound
		}

		// WHICH ROW HELD IT BEFORE, read BEFORE the write and recorded in the entry. "The commune's
		// reported total moved from this row to that one" is the whole content of this act, and after
		// the write the previous answer is gone from the table.
		var previousHeadline string
		for _, k := range full.Lines {
			if k.IsHeadline {
				previousHeadline = k.ID
			}
		}

		if err := uc.repo.SetHeadline(ctx, tx, sheetID, before.ID); err != nil {
			return err
		}
		after = before
		after.IsHeadline = true

		delta, err := json.Marshal(map[string]any{
			"khoan_muc_id": before.ID,
			"truoc":        map[string]any{"dong_tong_id": previousHeadline},
			"sau":          map[string]any{"dong_tong_id": before.ID, "ten": before.Name, "tt": before.OrdinalLabel},
		})
		if err != nil {
			return fmt.Errorf("ngan_sach: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionSetBudgetHeadline, Subject: sheet.Code, Delta: delta,
		})
	})
	if err != nil {
		return domain.BudgetLine{}, wrapBudgetErr(ctx, "đặt dòng tổng", err)
	}
	return after, nil
}

// lockAndReadTree is the opening move of every line write: lock the sheet, then read its tree under the
// lock.
//
// ONE FUNCTION AND NOT FOUR COPIES, because the part that is identical is the part that is easy to
// get subtly wrong in one copy — reading the tree BEFORE taking the lock, which gives a decision
// made against a sheet that has since moved.
func (uc *BudgetService) lockAndReadTree(ctx context.Context, tx *store.ScopedTx,
	sheetID string) (domain.BudgetSheet, domain.FullSheet, error) {

	sheet, err := uc.repo.SheetByIDForUpdate(ctx, tx, sheetID)
	if err != nil {
		return domain.BudgetSheet{}, domain.FullSheet{}, err
	}
	full, err := uc.repo.FullSheetInTx(ctx, tx, sheet)
	if err != nil {
		return domain.BudgetSheet{}, domain.FullSheet{}, err
	}
	return sheet, full, nil
}

// wrapBudgetErr wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No figure, no line name, no title: an error travels into centralised logging across every commune
// at once, and a budget line's text is a public authority's own wording about its spending. The
// commune is not personal data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is.
// A %v here would collapse "this line has children" and "the database is down" into one 500.
func wrapBudgetErr(ctx context.Context, op string, err error) error {
	return fmt.Errorf("ngan_sach: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}

// sheetSummary is the audit delta's view of one sheet. NOTHING HERE IS PERSONAL DATA (rule 3): a
// budget sheet carries figures, a title and column headings, and no person appears on it.
func sheetSummary(b domain.BudgetSheet, column []domain.BudgetColumn) map[string]any {
	result := map[string]any{
		"bang_id":     b.ID,
		"ma":          b.Code,
		"nam":         b.Year,
		"loai":        string(b.Kind),
		"lan":         b.Revision,
		"tieu_de":     b.Title,
		"don_vi_tinh": b.Unit,
	}
	if len(column) == 0 {
		return result
	}
	// THE COLUMN SET IS IN THE ENTRY FOR THE CREATE, and the roles with it. Which column carries
	// `Thu xã hưởng` decides the `Cân đối` cell (ADR 0035 #32), so "which columns did this sheet have
	// when it was created" is a question an inspection can genuinely need — and the columns are
	// editable by nothing, so this entry is the only record of the answer.
	var summary []map[string]any
	for _, c := range column {
		summary = append(summary, map[string]any{
			"cot_id": c.ID, "ten": c.Name, "thu_tu": c.SortOrder,
			"kieu": string(c.Format), "vai_tro": string(c.Indicator),
		})
	}
	result["cot"] = summary
	return result
}

// lineSummary is the audit delta's view of one line.
func lineSummary(k domain.BudgetLine) map[string]any {
	return map[string]any{
		"khoan_muc_id": k.ID,
		"bang_id":      k.SheetID,
		"cha_id":       k.ParentID,
		"tt":           k.OrdinalLabel,
		"ten":          k.Name,
		"thu_tu":       k.SortOrder,
		"cach_tinh":    string(k.Method),
		"cap":          k.Level,
		"la_dong_tong": k.IsHeadline,
	}
}

// lineDiff returns only the text fields that actually moved, from whichever side is asked
// for. The figures are not here: they travel in their own `o` list, with the column named, because a
// cell is identified by a pair and not by a field name.
func lineDiff(before, after domain.BudgetLine, useBefore bool) map[string]any {
	result := map[string]any{}
	if before.OrdinalLabel != after.OrdinalLabel {
		result["tt"] = pick(useBefore, before.OrdinalLabel, after.OrdinalLabel)
	}
	if before.Name != after.Name {
		result["ten"] = pick(useBefore, before.Name, after.Name)
	}
	if before.SortOrder != after.SortOrder {
		result["thu_tu"] = pick(useBefore, before.SortOrder, after.SortOrder)
	}
	return result
}
