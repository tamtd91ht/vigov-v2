package app

// The spreadsheet import of tasks — POST /api/v1/tasks/imports, `task.create` (§8; vigov-require
// 0053854 importer.py; user decisions 28/09/2026). The row rules are domain/task_import.go; this file
// owns the LOOKUPS and the ONE TRANSACTION.
//
// # ALL OR NOTHING
//
// Every row is checked BEFORE the transaction opens — the row rules, then identity (unit codes, staff
// codes, bloc codes) and this service's own catalogues (type, priority), each batched over the whole
// file. One refused row and NOTHING is written: a half-imported batch leaves nobody able to say which
// half landed (require's reasoning, and rule 2's "never half-processed"). Then ONE transaction books
// every row through createInTx — the very code POST /api/v1/tasks runs — so each task gets its number
// from the same generator (`SoLonNhatDaCap` read inside the transaction sees the rows just inserted),
// its document lines, its first timeline row and its own `tao_nhiem_vu` audit entry, plus ONE batch
// entry for the import itself. Any failure inside rolls all of it back.
//
// # WHAT IS CHECKED, AND AGAINST WHAT
//
//	unit codes               identity LiveOrgUnitIDsByCode — live units of THIS commune; the answer IS
//	                         the live predicate LiveOrgUnits applies, so the ids are not asked again
//	assignee                 identity ResolveAssignableStaff (CanBoGiaoViecDuoc), EVERY code in the
//	                         file — stricter than create, which checks only the assigner; typed codes
//	                         are more error-prone than a picker (user decision)
//	bloc codes               identity TaskBlocLabels, LIVE only — create does not check the bloc; a
//	                         typed code that names nothing would otherwise be stored as-is
//	type / priority codes    this service's catalogues, IN-USE rows only, by code or by a label that
//	                         names exactly one row (resolveImportCatalogue); a blank type takes the
//	                         commune's default type, and no default refuses the row
//
// An identity failure refuses the whole import as UNCHECKED (503), never rows as "unknown".
//
// NO LEAD UNIT, NO MONITOR (ADR 0065 NV5, user decision 30/09/2026): the template lost those two
// columns, and a file that still carries them is refused WHOLE (ErrTaskImportRetiredColumns) — never
// read with them skipped, because the clerk who typed a monitor there meant somebody to follow the
// task, and dropping the name silently would record an assignment nobody made.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ActionTaskImport is the batch entry's verb; each task still gets its own `tao_nhiem_vu`.
const ActionTaskImport = "nhap_nhiem_vu_tu_excel"

// ErrTaskImportUnchecked — identity could not be asked, so NOTHING was written and no row was judged.
// Retryable (503).
var ErrTaskImportUnchecked = errors.New("nhiem_vu: chưa kiểm được bộ phận, cán bộ hoặc khối cho tệp nhập")

// ErrTaskImportTooManyRows — more data rows than domain.TaskImportRowCap. Refused whole (422).
var ErrTaskImportTooManyRows = fmt.Errorf("nhiem_vu: tệp nhập quá %d dòng nhiệm vụ", domain.TaskImportRowCap)

// ErrTaskImportRetiredColumns — the heading row still names "Cơ quan chủ trì tham mưu" or "Chuyên viên
// theo dõi": the file was filled on the template from before ADR 0065 NV5. Refused whole (400), checked
// BEFORE ErrTaskImportLayout so the clerk is told the actual reason rather than "wrong layout".
var ErrTaskImportRetiredColumns = errors.New(
	"nhiem_vu: tệp theo mẫu cũ — còn cột cơ quan chủ trì hoặc chuyên viên theo dõi, hai vai đã gộp vào đơn vị và người thực hiện")

// ErrTaskImportLayout — the heading row is not the template's, or the sheet holds no task. Refused
// whole (400): reading another layout by position would put values into the wrong fields.
var ErrTaskImportLayout = errors.New("nhiem_vu: tệp không đúng mẫu nhập nhiệm vụ")

// TaskImportLayoutError is ErrTaskImportLayout carrying the sentence that says WHERE the file differs
// from the template ("ô F2 phải là tiêu đề …") — a bare "wrong layout" sends the clerk to compare
// fourteen headings by eye (bug sheet row 19). The sentence quotes only the template, never a cell.
type TaskImportLayoutError struct{ Sentence string }

func (e *TaskImportLayoutError) Error() string {
	return ErrTaskImportLayout.Error() + ": " + e.Sentence
}
func (e *TaskImportLayoutError) Unwrap() error { return ErrTaskImportLayout }

// TaskImportLookups is what the import asks identity. *identityclient.Client satisfies it.
type TaskImportLookups interface {
	LiveOrgUnitIDsByCode(ctx context.Context, codes []string) (map[string]string, error)
	TaskBlocLabels(ctx context.Context, codes []string) (map[string]identityclient.TaskBlocLabel, error)
	// vi-name-ok: the method name of the existing core/identityclient.Client this interface must match
	CanBoGiaoViecDuoc(ctx context.Context, ma []string) (map[string]struct{}, error)
}

// TaskTypeCatalogue and TaskPriorityCatalogue are this service's two task catalogues;
// *petstore.LoaiNhiemVuStore and *petstore.MucUuTienNhiemVuStore satisfy them.
type TaskTypeCatalogue interface {
	// vi-name-ok: the method name of the existing catalogue stores this interface must match
	DanhSach(ctx context.Context) ([]domain.LoaiNhiemVu, error)
}

type TaskPriorityCatalogue interface {
	// vi-name-ok: the method name of the existing catalogue stores this interface must match
	DanhSach(ctx context.Context) ([]domain.MucUuTienNhiemVu, error)
}

// TaskImport is the use case.
type TaskImport struct {
	create     *GhiNhiemVu
	lookups    TaskImportLookups
	types      TaskTypeCatalogue
	priorities TaskPriorityCatalogue
}

func NewTaskImport(create *GhiNhiemVu, lookups TaskImportLookups, types TaskTypeCatalogue,
	priorities TaskPriorityCatalogue) *TaskImport {
	return &TaskImport{create: create, lookups: lookups, types: types, priorities: priorities}
}

// TaskImportResult is the report. Errors are per row and per column; Codes are the numbers issued, in
// row order, when Committed.
type TaskImportResult struct {
	TotalRows int
	Created   int
	Committed bool
	Errors    []domain.TaskImportError
	Codes     []string
}

// preparedImportRow is one row ready for createInTx.
type preparedImportRow struct {
	row  int
	task domain.NhiemVu
	docs domain.ThayDoiVanBan
}

// Import checks every row and, when all pass and dryRun is false, books them all in one transaction.
// `sheet` is the first worksheet as text, heading row first.
func (uc *TaskImport) Import(ctx context.Context, sheet [][]string, dryRun bool, actor audit.Actor) (
	TaskImportResult, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return TaskImportResult{}, err
	}
	// The heading row may sit below title rows (domain.TaskImportHeadingSearchRows); the retired columns
	// are looked for over the same rows, so an old-template file is named as such wherever its heading is.
	for i := 0; i < len(sheet) && i < domain.TaskImportHeadingSearchRows; i++ {
		if domain.TaskImportCarriesRetiredColumns(sheet[i]) {
			return TaskImportResult{}, ErrTaskImportRetiredColumns
		}
	}
	head, ok := domain.LocateTaskImportHeadings(sheet)
	if !ok {
		return TaskImportResult{}, &TaskImportLayoutError{Sentence: domain.DiagnoseTaskImportHeadings(sheet).Sentence()}
	}

	// 0. THIS SERVICE'S OWN CATALOGUES, read before the row rules: a type/priority dropdown value
	// "code · label" is turned back into its code only when that code is the catalogue's
	// (importChoiceCells), and that needs the catalogue.
	types, err := uc.types.DanhSach(ctx)
	if err != nil {
		return TaskImportResult{}, fmt.Errorf("nhiem_vu: đọc danh mục loại nhiệm vụ cho tệp nhập: %w", err)
	}
	priorities, err := uc.priorities.DanhSach(ctx)
	if err != nil {
		return TaskImportResult{}, fmt.Errorf("nhiem_vu: đọc danh mục mức ưu tiên cho tệp nhập: %w", err)
	}
	typeEntries, defaultType := make([]importCatalogueEntry, 0, len(types)), ""
	for _, t := range types {
		typeEntries = append(typeEntries, importCatalogueEntry{code: t.Ma, label: t.Nhan, inUse: t.DangDung})
		if t.DangDung && t.LaMacDinh {
			defaultType = t.Ma
		}
	}
	priorityEntries := make([]importCatalogueEntry, 0, len(priorities))
	for _, p := range priorities {
		priorityEntries = append(priorityEntries, importCatalogueEntry{code: p.Ma, label: p.Nhan, inUse: p.DangDung})
	}

	// 1. THE ROW RULES.
	var (
		rows []domain.TaskImportRow
		errs []domain.TaskImportError
	)
	for i, cells := range sheet[head+1:] {
		if domain.TaskImportRowBlank(cells) {
			continue
		}
		if len(rows) == domain.TaskImportRowCap {
			return TaskImportResult{}, ErrTaskImportTooManyRows
		}
		// The SPREADSHEET row number the clerk sees: sheet[0] is row 1, the first data row is head+2.
		r, rowErrs := domain.ParseTaskImportRow(head+i+2, importChoiceCells(cells, typeEntries, priorityEntries))
		errs = append(errs, rowErrs...)
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return TaskImportResult{}, &TaskImportLayoutError{Sentence: fmt.Sprintf(
			"không có dòng nhiệm vụ nào dưới hàng tiêu đề (dòng %d).", head+1)}
	}
	res := TaskImportResult{TotalRows: len(rows)}

	// 2. THE LOOKUPS, batched over the whole file.
	var unitCodes, staffCodes, blocCodes []string
	for _, r := range rows {
		unitCodes = append(unitCodes, r.UnitCode)
		staffCodes = append(staffCodes, r.AssigneeCode)
		blocCodes = append(blocCodes, r.BlocCode)
	}
	units, err := lookupChunked(ctx, unitCodes, uc.lookups.LiveOrgUnitIDsByCode)
	if err != nil {
		return TaskImportResult{}, fmt.Errorf("%w: %w", ErrTaskImportUnchecked, err)
	}
	blocs, err := lookupChunked(ctx, blocCodes, uc.lookups.TaskBlocLabels)
	if err != nil {
		return TaskImportResult{}, fmt.Errorf("%w: %w", ErrTaskImportUnchecked, err)
	}
	staff, err := lookupChunkedAt(ctx, staffCodes, identityclient.TranMaGiaoViecMotLo, uc.lookups.CanBoGiaoViecDuoc)
	if err != nil {
		return TaskImportResult{}, fmt.Errorf("%w: %w", ErrTaskImportUnchecked, err)
	}

	// 3. EACH ROW AGAINST THE ANSWERS, then the create path's own normalisation. Every row's refusals
	// are collected; nothing is prepared for booking once any row has failed.
	now := uc.create.nayHoac()
	prepared := make([]preparedImportRow, 0, len(rows))
	for _, r := range rows {
		fail := func(col int, msg string) {
			errs = append(errs, domain.TaskImportError{Row: r.Row, Column: domain.TaskImportHeadings[col], Message: msg})
		}
		unitID := units[r.UnitCode]
		if r.UnitCode != "" && unitID == "" {
			fail(2, "không có bộ phận đang hoạt động mang mã này trong xã")
		}
		if _, ok := staff[r.AssigneeCode]; r.AssigneeCode != "" && !ok {
			fail(3, "cán bộ mang mã này không nhận được việc trong xã")
		}
		if b, ok := blocs[r.BlocCode]; r.BlocCode != "" &&
			(!ok || b.Standing != identityv1.RecordStanding_RECORD_STANDING_LIVE) {
			fail(7, "không có khối nhiệm vụ đang dùng mang mã này")
		}
		typeCode := r.TypeCode
		switch {
		case typeCode == "" && defaultType == "":
			fail(6, "xã chưa đặt loại nhiệm vụ mặc định — hãy ghi mã loại nhiệm vụ")
		case typeCode == "":
			typeCode = defaultType
		default:
			code, problem := resolveImportCatalogue(typeCode, typeEntries, "loại nhiệm vụ")
			if problem != "" {
				fail(6, problem)
			}
			typeCode = code
		}
		priorityCode := r.PriorityCode
		if priorityCode != "" {
			code, problem := resolveImportCatalogue(priorityCode, priorityEntries, "mức ưu tiên")
			if problem != "" {
				fail(4, problem)
			}
			priorityCode = code
		}
		if len(errs) > 0 {
			continue
		}

		moi, err := chuanHoaTaoNhiemVu(YeuCauTaoNhiemVu{
			TuSinhMa: true, Loai: typeCode, Khoi: r.BlocCode, TieuDe: r.Title, MoTa: r.Description,
			MucUuTien: priorityCode, GhiChu: r.Note, NguonGiao: string(domain.NguonTrucTiep),
			BoPhanID: unitID, NguoiThucHienMa: r.AssigneeCode,
			HanXuLy: r.Due,
		})
		if err != nil {
			// Unreachable once the row rules passed; kept as the floor, without the cell's value.
			fail(0, "dòng không hợp lệ theo luật giao việc")
			continue
		}
		docs, err := domain.KiemDanhSachVanBanNhiemVu(r.Documents)
		if err == nil {
			var change domain.ThayDoiVanBan
			if change, err = domain.SoSanhVanBan(nil, docs); err == nil {
				moi.LanhDaoPheDuyetHoanThanh = r.LeaderApproved
				moi.CapTrenCongNhanHoanThanh = r.SuperiorAcknowledged
				moi.NguoiTaoMa = actor.ID
				moi.TaoLuc = now
				moi.UpdatedAt = now.Truncate(time.Microsecond)
				prepared = append(prepared, preparedImportRow{row: r.Row, task: moi, docs: change})
				continue
			}
		}
		fail(8, "văn bản không hợp lệ") // "Văn bản cấp trên giao" — column 8 since ADR 0065 NV5
	}

	res.Errors = errs
	if len(errs) > 0 || dryRun {
		return res, nil
	}

	// Kind 18's fallback recipients for the rows naming a unit and no assignee — asked ONCE for every such
	// unit, before the transaction (resolveUnitHolders; never fails the import).
	var heldUnits []string
	for _, p := range prepared {
		if strings.TrimSpace(p.task.NguoiThucHienMa) == "" && p.task.BoPhanID != "" {
			heldUnits = append(heldUnits, p.task.BoPhanID)
		}
	}
	holders := resolveUnitHolders(ctx, uc.create.unitHolders, heldUnits)

	// 4. ONE TRANSACTION FOR THE WHOLE FILE.
	var codes []string
	err = uc.create.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		codes = make([]string, 0, len(prepared))
		for i := range prepared {
			p := &prepared[i]
			id, err := uc.create.sinhID()
			if err != nil {
				return fmt.Errorf("nhiem_vu: sinh mã nội bộ: %w", err)
			}
			p.task.ID = id
			if err := uc.create.createInTx(ctx, tx, &p.task, createInTxRequest{
				TuSinhMa: true, Documents: p.docs, LogPrefix: "Nhập từ Excel: ", UnitHolders: holders,
				ExtraDelta: map[string]any{
					"nguon_tao":          "nhap_excel",
					"dong_excel":         p.row,
					"lanh_dao_phe_duyet": p.task.LanhDaoPheDuyetHoanThanh,
					"cap_tren_cong_nhan": p.task.CapTrenCongNhanHoanThanh,
				},
			}, actor, now); err != nil {
				return err
			}
			codes = append(codes, p.task.Ma)
		}
		// THE BATCH ENTRY: one row an inspection can find for "who imported this file", naming the
		// range of numbers it issued. No file name — a file name may carry personal data (rule 3).
		delta, err := json.Marshal(map[string]any{
			"so_dong": len(codes),
			"ma_dau":  codes[0],
			"ma_cuoi": codes[len(codes)-1],
		})
		if err != nil {
			return fmt.Errorf("nhiem_vu: mã hoá delta nhập tệp: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionTaskImport, Subject: registerExportSubject, Delta: delta,
		})
	})
	if err != nil {
		return TaskImportResult{}, bocNhiemVu(ctx, "nhập nhiệm vụ từ tệp", err)
	}
	res.Codes = codes
	res.Created = len(codes)
	res.Committed = true
	return res, nil
}

// importCatalogueEntry is one row of a task catalogue as the import matches a typed cell against it.
type importCatalogueEntry struct {
	code, label string
	inUse       bool
}

// resolveImportCatalogue turns a typed type/priority cell into an IN-USE catalogue CODE, or a refusal
// sentence for the row report.
//
// THE CODE FIRST, THEN THE LABEL (bug sheet row 19): the Nhiệm vụ screen shows "Khẩn" and "Theo văn
// bản", never `khan`, and a clerk types what they see. A label is accepted only when it names EXACTLY
// ONE in-use row, and what is stored is that row's code — the label is a commune's to rename, the code
// never changes. A label two rows share is refused with a request for the code: picking one would file
// the task under a level the clerk may not have meant. `typed` arrives lower-cased by the row rules.
// Sentences never quote the cell (rule 3).
func resolveImportCatalogue(typed string, entries []importCatalogueEntry, what string) (string, string) {
	for _, e := range entries {
		if e.code == typed {
			if !e.inUse {
				return "", what + " mang mã này đã tắt trong danh mục của xã"
			}
			return e.code, ""
		}
	}
	want := domain.NormaliseImportText(typed)
	found, off := "", false
	for _, e := range entries {
		if domain.NormaliseImportText(e.label) != want {
			continue
		}
		if !e.inUse {
			off = true
			continue
		}
		if found != "" {
			return "", "tên này trùng nhiều " + what + " đang dùng — hãy ghi mã thay cho tên (xem mã ở Cấu hình › Danh mục)"
		}
		found = e.code
	}
	switch {
	case found != "":
		return found, ""
	case off:
		return "", what + " mang tên này đã tắt trong danh mục của xã"
	}
	return "", "không có " + what + " đang dùng mang mã hoặc tên này trong danh mục của xã"
}

// importChoiceCells returns a copy of one data row with the template's dropdown values ("code ·
// label", xlsx.ChoiceValue) turned back into the bare code the row rules read. A bare code typed by
// hand passes unchanged, so files filled before the template had dropdowns still import.
//
//	unit · assignee · bloc   always the part before the separator: none of these codes can carry it
//	                         (unit and bloc are slugs, staff codes `CB-…`), and what remains is checked
//	                         against identity exactly as a typed code is
//	type · priority          only when the part before the separator IS a code of the catalogue. These
//	                         two also accept a unique LABEL (resolveImportCatalogue), and a label a
//	                         commune wrote with " · " in it must still reach that match whole
//
// The cell is never trusted for having come from a dropdown: Excel's list is a convenience a person can
// type past, and every code still goes through the same lookups below.
func importChoiceCells(cells []string, types, priorities []importCatalogueEntry) []string {
	out := append([]string(nil), cells...)
	for _, col := range []int{domain.TaskImportColUnit, domain.TaskImportColAssignee, domain.TaskImportColBloc} {
		if col < len(out) {
			out[col] = xlsx.ChoiceCode(out[col])
		}
	}
	for _, c := range []struct {
		col     int
		entries []importCatalogueEntry
	}{{domain.TaskImportColType, types}, {domain.TaskImportColPriority, priorities}} {
		if c.col >= len(out) || !strings.Contains(out[c.col], xlsx.ChoiceSeparator) {
			continue
		}
		// Lower-cased as the row rules fold these codes (domain.ParseTaskImportRow).
		code := strings.ToLower(xlsx.ChoiceCode(out[c.col]))
		for _, e := range c.entries {
			if e.code == code {
				out[c.col] = code
				break
			}
		}
	}
	return out
}

// lookupChunkedAt is lookupChunked with an explicit ceiling — ResolveAssignableStaff's is 50, not the
// name lookups' 200.
func lookupChunkedAt[V any](ctx context.Context, keys []string, ceiling int,
	call func(context.Context, []string) (map[string]V, error)) (map[string]V, error) {

	seen := make(map[string]struct{}, len(keys))
	uniq := make([]string, 0, len(keys))
	for _, k := range keys {
		if _, dup := seen[k]; dup || k == "" {
			continue
		}
		seen[k] = struct{}{}
		uniq = append(uniq, k)
	}
	out := make(map[string]V, len(uniq))
	for start := 0; start < len(uniq); start += ceiling {
		got, err := call(ctx, uniq[start:min(start+ceiling, len(uniq))])
		if err != nil {
			return nil, err
		}
		for k, v := range got {
			out[k] = v
		}
	}
	return out, nil
}
