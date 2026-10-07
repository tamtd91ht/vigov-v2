package app

// The Sổ theo dõi export — GET /api/v1/tasks/register-export, `task.read` (user decision 28/09/2026).
//
// # WHAT THIS LAYER OWNS, AND WHY IT IS NOT THE HANDLER'S
//
// Three things that must happen in one order, and the order is the rule:
//
//  1. READ the rows under the list's own filters and sort (the same store reads GET /api/v1/tasks
//     uses), their document blocks, and the NAMES identity holds for units, blocs and staff.
//  2. RENDER the file — through a callback the handler supplies, because an .xlsx is presentation.
//  3. RECORD the export in `audit_log`, in its own transaction, and only THEN hand the bytes back.
//
// Keeping 3 here, after 2 and before the return, is what makes "a file left the system with no trail"
// impossible to express: there is no exported path that returns bytes without having written the
// entry (the shape xem_nguoi_gui.go uses for a read that must be audited). A render failure writes no
// entry — nothing left. An audit failure returns no bytes — nothing leaves.
//
// # THE CAP
//
// RegisterExportRowCap rows, refused (never truncated) above it: a register printout that silently
// stops at row 5000 is a record a reader believes complete. Counted FIRST with the list's own count
// (one statement), so an oversized request reads no rows; checked AGAIN while paging, so rows added
// between the count and the read cannot slip past it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// RegisterExportRowCap bounds one export. The workbook is built in memory on a process every commune
// shares; 5000 rows is several years of one commune's register and a few MB of xlsx. Approved by the
// user on 28/09/2026.
const RegisterExportRowCap = 5000

// ActionRegisterExport is the verb in the trail (Vietnamese snake_case, like every action here).
const ActionRegisterExport = "xuat_so_theo_doi_nhiem_vu"

// registerExportSubject is the entry's subject. An export is about THE REGISTER, not one task, so the
// subject names the register — a business name an inspection can search for, never an internal id.
const registerExportSubject = "so-theo-doi-nhiem-vu"

// ErrRegisterExportTooLarge — the filters match more than RegisterExportRowCap rows. 422.
var ErrRegisterExportTooLarge = fmt.Errorf(
	"nhiem_vu: sổ theo dõi khớp quá %d nhiệm vụ — hãy thu hẹp bộ lọc rồi xuất lại", RegisterExportRowCap)

// ErrRegisterNamesUnavailable — identity could not supply the unit, bloc or staff names. The file is
// NOT produced: printing blanks or ids where names belong is a statement about an official register
// made from a failure to read it. 503, retryable.
var ErrRegisterNamesUnavailable = errors.New("nhiem_vu: chưa tra được tên bộ phận, khối hoặc cán bộ cho sổ theo dõi")

// TaskRegisterReader is the list's own read surface. *petstore.NhiemVuStore satisfies it.
type TaskRegisterReader interface {
	// vi-name-ok: the method name of the existing *petstore.NhiemVuStore this interface must match
	DanhSach(ctx context.Context, loc petstore.LocNhiemVu, yc page.Request) (page.Result[domain.NhiemVu], error)
	CountByStatus(ctx context.Context, loc petstore.LocNhiemVu) (map[domain.TrangThaiNhiemVu]int, error)
	DocumentsForTasks(ctx context.Context, taskIDs []string) (map[string][]domain.NhiemVuVanBan, error)
}

// RegisterNameResolver is identity's three name lookups. *identityclient.Client satisfies it.
//
// ⚠ EVERY ANSWER HERE IS FOR PRINTING ONLY. A name resolving says nothing about whether the unit may
// hold work or the person may be assigned it (identityclient states it on each method).
type RegisterNameResolver interface {
	// vi-name-ok: the method name of the existing core/identityclient.Client this interface must match
	TenCanBoTheoMa(ctx context.Context, ma []string) (map[string]identityclient.TenCanBo, error)
	OrgUnitNames(ctx context.Context, ids []string) (map[string]identityclient.OrgUnitName, error)
	TaskBlocLabels(ctx context.Context, codes []string) (map[string]identityclient.TaskBlocLabel, error)
}

// TaskRegisterData is what the renderer receives: the rows in the list's order, each with its document
// block loaded, and identity's names keyed as the rows store them. An ABSENT key is an ordinary answer
// (unknown or another commune's) the renderer shows as unknown — never as an empty cell.
type TaskRegisterData struct {
	Tasks []domain.NhiemVu
	Staff map[string]identityclient.TenCanBo
	Units map[string]identityclient.OrgUnitName
	Blocs map[string]identityclient.TaskBlocLabel
}

// TaskRegisterExport is the use case.
type TaskRegisterExport struct {
	db    *store.DB
	list  TaskRegisterReader
	names RegisterNameResolver
}

func NewTaskRegisterExport(db *store.DB, list TaskRegisterReader, names RegisterNameResolver) *TaskRegisterExport {
	return &TaskRegisterExport{db: db, list: list, names: names}
}

// RegisterExportRequest is the export as the handler hands it down: the list's filter (identity-backed
// parts already resolved) and the list's sort, validated by the handler with page.New.
type RegisterExportRequest struct {
	Filter petstore.LocNhiemVu
	Sort   string
	Order  string
}

// Export reads, renders, records, and returns the file and its row count.
func (uc *TaskRegisterExport) Export(ctx context.Context, req RegisterExportRequest, actor audit.Actor,
	render func(TaskRegisterData) ([]byte, error)) ([]byte, int, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return nil, 0, err
	}

	// 1a. THE COUNT FIRST — one statement, the list's own predicate.
	counts, err := uc.list.CountByStatus(ctx, req.Filter)
	if err != nil {
		return nil, 0, fmt.Errorf("nhiem_vu: đếm sổ theo dõi để xuất: %w", err)
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	if total > RegisterExportRowCap {
		return nil, 0, ErrRegisterExportTooLarge
	}

	// 1b. THE ROWS, page by page through the list's own read — same filter, same sort, same keyset.
	var tasks []domain.NhiemVu
	cursor := ""
	for {
		yc, err := page.New(petstore.SapXepNhiemVu, req.Sort, req.Order, strconv.Itoa(page.MaxLimit), cursor)
		if err != nil {
			return nil, 0, fmt.Errorf("nhiem_vu: dựng trang xuất sổ: %w", err)
		}
		res, err := uc.list.DanhSach(ctx, req.Filter, yc)
		if err != nil {
			return nil, 0, fmt.Errorf("nhiem_vu: đọc sổ theo dõi để xuất: %w", err)
		}
		tasks = append(tasks, res.Items...)
		if len(tasks) > RegisterExportRowCap {
			// Rows arrived between the count and the read. Refused, never cut at the cap.
			return nil, 0, ErrRegisterExportTooLarge
		}
		if !res.HasMore {
			break
		}
		cursor = res.NextCursor
	}

	// 1c. THE DOCUMENT BLOCKS, one statement per page-sized chunk.
	for start := 0; start < len(tasks); start += page.MaxLimit {
		end := min(start+page.MaxLimit, len(tasks))
		ids := make([]string, 0, end-start)
		for _, n := range tasks[start:end] {
			ids = append(ids, n.ID)
		}
		blocks, err := uc.list.DocumentsForTasks(ctx, ids)
		if err != nil {
			return nil, 0, fmt.Errorf("nhiem_vu: đọc văn bản cho sổ theo dõi: %w", err)
		}
		for i := start; i < end; i++ {
			b, ok := blocks[tasks[i].ID]
			if !ok || b == nil {
				// Same line the list draws: a missing key is a broken reader, not an empty block.
				return nil, 0, errors.New("nhiem_vu: kho không trả khối văn bản cho một nhiệm vụ của sổ")
			}
			tasks[i].VanBan = b
		}
	}

	// 1d. THE NAMES — each lookup de-duplicated and chunked to the contract's ceiling.
	data := TaskRegisterData{Tasks: tasks}
	var staff, units, blocs []string
	for _, n := range tasks {
		staff = append(staff, n.NguoiThucHienMa)
		units = append(units, n.BoPhanID)
		blocs = append(blocs, n.Khoi)
	}
	if data.Staff, err = lookupChunked(ctx, staff, uc.names.TenCanBoTheoMa); err != nil {
		return nil, 0, fmt.Errorf("%w: %w", ErrRegisterNamesUnavailable, err)
	}
	if data.Units, err = lookupChunked(ctx, units, uc.names.OrgUnitNames); err != nil {
		return nil, 0, fmt.Errorf("%w: %w", ErrRegisterNamesUnavailable, err)
	}
	if data.Blocs, err = lookupChunked(ctx, blocs, uc.names.TaskBlocLabels); err != nil {
		return nil, 0, fmt.Errorf("%w: %w", ErrRegisterNamesUnavailable, err)
	}

	// 2. RENDER.
	file, err := render(data)
	if err != nil {
		return nil, 0, fmt.Errorf("nhiem_vu: dựng tệp sổ theo dõi: %w", err)
	}

	// 3. RECORD, then return. See the top of this file for why the order is the rule.
	delta, err := json.Marshal(map[string]any{
		"so_dong": len(tasks),
		// THE FILE CARRIES STAFF FULL NAMES (§4.3 prints them; user decision 28/09/2026). Recorded so
		// an inspection can find every export that took names out of the system.
		"co_ho_ten_can_bo": true,
		"sap_xep":          map[string]string{"cot": req.Sort, "chieu": req.Order},
		"bo_loc":           registerFilterSummary(req.Filter),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("nhiem_vu: mã hoá delta xuất sổ: %w", err)
	}
	if err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionRegisterExport,
			Subject: registerExportSubject,
			Delta:   delta,
		})
	}); err != nil {
		return nil, 0, fmt.Errorf("nhiem_vu: ghi vết xuất sổ theo dõi: %w", err)
	}
	return file, len(tasks), nil
}

// lookupChunked calls one identity lookup over a de-duplicated key list, in chunks of the contract's
// ceiling, and merges the answers. Blank keys are dropped (a task with no unit asks nothing).
func lookupChunked[V any](ctx context.Context, keys []string,
	call func(context.Context, []string) (map[string]V, error)) (map[string]V, error) {
	return lookupChunkedAt(ctx, keys, identityclient.MaxLookupKeysPerCall, call)
}

// registerFilterSummary is the filter as the trail records it: WHICH filters were set and their CODED
// values — status, type, bloc, priority, unit, source, assignee (a staff code), parent, roots, metric,
// period, the checkboxes and the scope. The free-text search is recorded as PRESENT ONLY: `q` is
// whatever a clerk typed, which may quote a citizen's complaint, and `audit_log` is permanent (rule 3,
// forbidden #5).
func registerFilterSummary(loc petstore.LocNhiemVu) map[string]any {
	out := map[string]any{}
	set := func(k, v string) {
		if v != "" {
			out[k] = v
		}
	}
	set("status", loc.TrangThai)
	set("type", loc.Loai)
	set("bloc", loc.Khoi)
	set("priority", loc.MucUuTien)
	set("unit", loc.BoPhanID)
	set("source", loc.NguonGiao)
	set("assignee", loc.NguoiThucHienMa)
	set("parent", loc.ParentCode)
	if loc.Roots {
		out["roots"] = true
	}
	set("metric", string(loc.Metric))
	if !loc.Period.From.IsZero() || !loc.Period.To.IsZero() {
		out["from"], out["to"] = loc.Period.From, loc.Period.To
	}
	if loc.Tim != "" {
		out["q_present"] = true
	}
	if loc.ChiTreHan {
		out["late"] = true
	}
	if !loc.DueSoonFrom.IsZero() {
		out["soon"] = true
	}
	if loc.Related != nil {
		out["scope"] = "related"
	}
	// The scope's NAME and not the code behind it: the code is the actor's own, already on the entry.
	if loc.AssignedByStaffCode != "" {
		out["scope"] = "assigned-by-me"
	}
	if loc.Incomplete {
		out["incomplete"] = true
	}
	return out
}
