package app

// `Nhập từ Excel` on the citizen contact directory (ADR 0097 D3, D13; chapter 16 §7) — the use case
// behind the preview and the write route, both `citizen.update`. The sheet's shape rules are
// domain/citizen_directory_import.go and are not restated here (rule 9).
//
//	ImportPreview  plan the file against the commune and report — WRITES NOTHING, audits nothing
//	ImportCommit   the same plan inside ONE transaction; any error = nothing written; else every row
//	               with its memberships, ONE audit entry for the run (rule 6, invariant 3)
//
// ALL OR NOTHING, the shape of every import of this repository (staff_import.go,
// residential_unit_import.go; chapter 16 §7 names the choice): a half-written file leaves the
// clerk unable to tell which half landed, and re-importing it refuses the first half as duplicates.
//
// WHAT THIS LAYER RESOLVES THAT THE SHEET CANNOT: the residential unit (by code, then by name,
// against the live units the caller lists), the group names (one store statement for the whole
// file), and whether a number is ALREADY in the commune's directory (PhoneTaken, inside the
// transaction so the answer and the inserts see the same rows).
//
// THE FILE IS NOT STORED (ADR 0097 D13): a copy of a sheet of names and numbers is a second
// personal-data store nobody cleans. What remains is `import_batch_id` on every row and one entry
// whose delta holds the row count and the batch — no cell of the file.
//
// NO MESSAGE ECHOES A CELL (rule 3, forbidden #3): a row error names the row and the column.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ActionImportCitizenContacts is the ONE entry of a committed file (ADR 0059 §1 precedent: N rows,
// one entry). The subject is the batch id every imported row carries.
const ActionImportCitizenContacts = "nhap_danh_ba_nguoi_dan_tu_excel"

// CitizenContactImportRejected carries every error of a file that was refused by ImportCommit.
// Nothing was written.
type CitizenContactImportRejected struct {
	Errors []domain.CitizenContactImportError
}

func (e *CitizenContactImportRejected) Error() string {
	return fmt.Sprintf("danh bạ người dân: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// ImportResidentialUnit is one live residential unit of the commune a file may name, as the HTTP
// layer lists it for the template's dropdown. A cell is matched by CODE first, then by NAME (case
// folded, the staff import's rule) — never by id, which nobody types.
type ImportResidentialUnit struct {
	ID   string
	Code string
	Name string
}

// CitizenContactImportPreview is what a preview found. RowCount is the number of data rows the
// sheet holds (0 when the file itself was refused); Errors is empty when the file would be written
// as is. NO PERSONAL DATA: the person who uploaded the file already has the file.
type CitizenContactImportPreview struct {
	RowCount int
	Errors   []domain.CitizenContactImportError
}

// CitizenContactImportResult is a committed import: the batch every row carries and how many rows.
type CitizenContactImportResult struct {
	Batch    string
	RowCount int
}

// errCitizenImportPreviewRollback ends the preview's transaction: a preview must not COMMIT
// anything, even by accident, so it always rolls back.
var errCitizenImportPreviewRollback = errors.New("danh bạ người dân: xem trước — huỷ giao dịch")

// ImportPreview plans the file and reports. The returned error is a system failure only; a file with
// row errors is a SUCCESSFUL preview whose Errors is non-empty.
//
// The duplicate check needs a transaction (PhoneTaken reads inside one); the transaction is rolled
// back on purpose, so a preview leaves no row, no entry and no lock behind.
func (uc *CitizenDirectory) ImportPreview(ctx context.Context, sheet [][]string, units []ImportResidentialUnit) (CitizenContactImportPreview, error) {
	var res CitizenContactImportPreview
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		plan, errs, err := uc.planCitizenImport(ctx, tx, sheet, units)
		if err != nil {
			return err
		}
		res = CitizenContactImportPreview{RowCount: plan.rowCount, Errors: errs}
		return errCitizenImportPreviewRollback
	})
	if err != nil && !errors.Is(err, errCitizenImportPreviewRollback) {
		return CitizenContactImportPreview{}, err
	}
	return res, nil
}

// ImportCommit writes the file, or nothing. Every row: Source = nhap-excel, the run's batch id, its
// memberships; then ONE entry for the run. A row error anywhere returns
// *CitizenContactImportRejected from inside the transaction, which rolls it back.
func (uc *CitizenDirectory) ImportCommit(ctx context.Context, actor NguoiThucHien, sheet [][]string, units []ImportResidentialUnit) (CitizenContactImportResult, error) {
	if err := actor.hopLe(); err != nil {
		return CitizenContactImportResult{}, err
	}
	batch, err := uc.newID()
	if err != nil {
		return CitizenContactImportResult{}, fmt.Errorf("danh bạ người dân: sinh mã lô nhập: %w", err)
	}
	at := uc.now()
	var res CitizenContactImportResult
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		plan, errs, err := uc.planCitizenImport(ctx, tx, sheet, units)
		if err != nil {
			return err
		}
		if len(errs) > 0 {
			return &CitizenContactImportRejected{Errors: errs}
		}
		for _, p := range plan.rows {
			id, err := uc.newID()
			if err != nil {
				return fmt.Errorf("danh bạ người dân: sinh id: %w", err)
			}
			c := contactFromInput(id, p.contact, domain.CitizenContactSourceExcel, &batch, at)
			// Insert binds tx.TenantID() as $1 (rule 1, invariant 5). A unique-key refusal here is
			// the race PhoneTaken could not see; the row number says where, the number never does.
			if err := uc.contacts.Insert(ctx, tx, c); err != nil {
				return fmt.Errorf("dòng %d: %w", p.row, err)
			}
			if len(p.groupIDs) > 0 {
				if err := uc.contacts.SetGroups(ctx, tx, c.ID, p.groupIDs, actor.Vet.ID, at); err != nil {
					return fmt.Errorf("dòng %d: %w", p.row, err)
				}
			}
		}
		// ONE ENTRY FOR THE ACT OF IMPORTING — "who imported this file, when, how many rows, which
		// batch" is the question an inspection asks; each row answers "which file" by its batch id.
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionImportCitizenContacts,
			Subject: batch,
			At:      at,
			Delta: deltaSLA(map[string]any{
				"so_dong": len(plan.rows), "ma_lo": batch, "nguon": string(domain.CitizenContactSourceExcel),
			}),
		}); err != nil {
			return err
		}
		res = CitizenContactImportResult{Batch: batch, RowCount: len(plan.rows)}
		return nil
	})
	if err != nil {
		return CitizenContactImportResult{}, err
	}
	return res, nil
}

// plannedCitizenContact is one row ready to insert: the unit resolved to an id, the groups to ids.
type plannedCitizenContact struct {
	row      int
	contact  domain.CitizenContactInput
	groupIDs []string
}

type citizenImportPlan struct {
	rowCount int
	rows     []plannedCitizenContact
}

// planCitizenImport reads the sheet through the domain, then resolves what only the commune can
// answer. Returns the plan, or every row error (rows nil then), or a system error. The group lookup
// is ONE statement for the whole file; the duplicate check is one PhoneTaken per row, inside the
// caller's transaction (at most domain.MaxCitizenContactImportRows).
func (uc *CitizenDirectory) planCitizenImport(ctx context.Context, tx *store.ScopedTx, sheet [][]string,
	units []ImportResidentialUnit) (citizenImportPlan, []domain.CitizenContactImportError, error) {

	rows, errs := domain.ReadCitizenContactImportSheet(sheet)
	if len(errs) > 0 {
		return citizenImportPlan{}, errs, nil
	}
	plan := citizenImportPlan{rowCount: len(rows)}

	var names []string
	for _, r := range rows {
		names = append(names, r.GroupNames...)
	}
	groupID, err := uc.groups.IDsByNames(ctx, names)
	if err != nil {
		return citizenImportPlan{}, nil, err
	}

	fail := func(r domain.CitizenContactImportRow, col, msg string) {
		errs = append(errs, domain.CitizenContactImportError{Row: r.Row, Column: col, Message: msg})
	}
	for _, r := range rows {
		p := plannedCitizenContact{row: r.Row, contact: r.Contact}
		ok := true

		if r.ResidentialUnit != "" {
			id, n := resolveImportResidentialUnit(units, r.ResidentialUnit, r.ResidentialUnitIsCode)
			switch n {
			case 1:
				p.contact.ResidentialUnitID = id
			case 0:
				ok = false
				fail(r, domain.CitizenContactImportColResidentialUnit,
					"Không có thôn / tổ dân phố nào của xã khớp với ô này. Hãy chọn trong danh sách của tệp mẫu hoặc để trống.")
			default:
				ok = false
				fail(r, domain.CitizenContactImportColResidentialUnit,
					"Có nhiều thôn / tổ dân phố cùng tên này. Hãy chọn trong danh sách của tệp mẫu (có kèm mã).")
			}
		}

		for _, name := range r.GroupNames {
			id, found := groupID[name]
			if !found {
				ok = false
				fail(r, domain.CitizenContactImportColGroups,
					"Xã chưa lập nhóm mang tên này. Lập nhóm ở ngăn Nhóm trước, hoặc sửa ô này.")
				break
			}
			p.groupIDs = append(p.groupIDs, id)
		}

		// Inside the caller's transaction: the inserts that follow see the same rows. The message
		// does not repeat the number (rule 3, forbidden #3).
		taken, err := uc.contacts.PhoneTaken(ctx, tx, r.Contact.Phone, "")
		if err != nil {
			return citizenImportPlan{}, nil, err
		}
		if taken {
			ok = false
			fail(r, domain.CitizenContactImportColPhone, "Số điện thoại này đã có trong danh bạ của xã.")
		}

		if ok {
			plan.rows = append(plan.rows, p)
		}
	}
	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Row < errs[j].Row })
		plan.rows = nil
		return plan, errs, nil
	}
	return plan, nil, nil
}

// resolveImportResidentialUnit matches one cell against the commune's units: by code when the cell
// came from the dropdown, else by code first and then by folded name (org_unit_import.go's
// foldName: trimmed, lower-cased — not Unicode-normalised, so a mismatch refuses, never matches
// the wrong unit). Returns the id and HOW MANY units matched; two units with one name is a cell
// the person must disambiguate with the code.
func resolveImportResidentialUnit(units []ImportResidentialUnit, cell string, byCode bool) (string, int) {
	var id string
	n := 0
	for _, u := range units {
		if cell == u.Code {
			return u.ID, 1
		}
		if !byCode && foldUnitName(cell) == foldUnitName(u.Name) {
			id = u.ID
			n++
		}
	}
	return id, n
}

func foldUnitName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
