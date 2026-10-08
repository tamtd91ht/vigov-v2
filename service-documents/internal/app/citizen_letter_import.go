package app

// `Nhập từ Excel` on the citizen-letter register (ADR 0084 #6; owner spec §4.1) — the use case behind
// POST /api/v1/citizen-letters/import-previews and …/imports, `petition.create` (the booking key).
// The sheet's shape rules are domain/citizen_letter_import.go.
//
// # ALL OR NOTHING — THE PRECEDENT'S CALL, AND THE PROTOTYPE'S
//
// Every repository import is all or nothing (service-petitions app/task_import.go, this service's
// document-type import), and so is the prototype ("one bad row and nothing is written", vigov-require
// book.py:429). Here it matters more than for a catalogue: every booked row TAKES A REGISTER NUMBER
// that is never given back (rule 7, invariant 3). A half-imported batch would leave numbers issued for
// the first half and a clerk unable to tell which half landed, and re-importing the file would book the
// first half twice.
//
//  1. EVERY ROW IS PREPARED BEFORE ANY TRANSACTION — through prepareBooking, THE SAME code Book runs:
//     the unit is checked live, and identity fixes the processing deadline from the commune's rule
//     (ADR 0085 B). A row the booking rules refuse is a ROW ERROR; nothing is written.
//  2. IDENTITY FAILING ANYWHERE IN 1 REFUSES THE WHOLE FILE AS UNCHECKED (503) — never "row refused",
//     never "booked without a deadline": a letter stored with no deadline because identity was down
//     counts as ON TIME in the report (ADR 0084 #4). Nothing was written, no number was taken.
//  3. ONE TRANSACTION books every row through bookInTx — the very code Book runs inside its own — so
//     each letter gets the next number of the commune's `don-thu` series, its `Phân công xử lý` log row
//     when a unit was named, and its own `vao_so_don_thu` audit entry; then ONE batch entry records the
//     import itself. Any failure inside rolls ALL of it back, numbers included.
//
// SOURCE IS `nhap-excel`, SET HERE: the client never states it (ADR 0084 #7).
//
// NOTHING ABOUT A ROW ENTERS A LOG OR THE TRAIL BEYOND WHAT BOOKING ALREADY RECORDS (rule 3): the batch
// entry lists the numbers issued and the row count — no name, phone, address or summary.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// ActionImportCitizenLetters is the batch entry's verb; each letter still gets its own `vao_so_don_thu`.
const ActionImportCitizenLetters = "nhap_don_thu_tu_excel"

// ErrLetterImportUnchecked — identity could not be asked (a unit code, a live unit, a deadline), so no
// row was judged and NOTHING was written. Retryable (503).
var ErrLetterImportUnchecked = errors.New("don_thu: chưa kiểm được bộ phận hoặc hạn xử lý cho tệp nhập")

// LetterImportRejected carries every row error of a file that was refused. Nothing was written.
type LetterImportRejected struct {
	Errors []domain.LetterImportError
}

func (e *LetterImportRejected) Error() string {
	return fmt.Sprintf("don_thu: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// LetterUnitCodes translates typed org-unit codes into the ids of LIVE units of the commune in the
// context. *identityclient.Client satisfies it (LiveOrgUnitIDsByCode: an absent code is "no live
// unit"; an error is never "unknown").
type LetterUnitCodes interface {
	LiveOrgUnitIDsByCode(ctx context.Context, codes []string) (map[string]string, error)
}

// ImportedLetter is one row as the import planned (preview) or booked it. NO PERSONAL DATA: this is
// what goes back to the browser, and the person who uploaded the file already has the file.
type ImportedLetter struct {
	Row             int
	ID              string // only once booked
	Number          int    // only once booked
	Year            int
	Type            domain.LetterType
	ReceivedDate    time.Time
	HoldingUnitID   string
	ProcessingDueAt time.Time // zero = the commune has no rule for this type, "Không đặt hạn"
	SenderUnknown   bool
}

// LetterImportResult is a preview (Errors may be non-empty) or a committed import (Errors empty).
type LetterImportResult struct {
	Letters []ImportedLetter
	Errors  []domain.LetterImportError
}

// CitizenLetterImport is the use case.
type CitizenLetterImport struct {
	letters *CitizenLetters
	units   LetterUnitCodes
}

func NewCitizenLetterImport(letters *CitizenLetters, units LetterUnitCodes) *CitizenLetterImport {
	return &CitizenLetterImport{letters: letters, units: units}
}

// Preview prepares every row exactly as Import would and reports. WRITES NOTHING, AUDITS NOTHING, takes
// no number (the number is drawn inside the transaction Import opens; Year is shown, Number is not).
// The returned error is a system failure or ErrLetterImportUnchecked; a file with row errors is a
// SUCCESSFUL preview whose Errors is non-empty.
func (uc *CitizenLetterImport) Preview(ctx context.Context, rows []domain.LetterImportRow, caller LetterCaller) (
	LetterImportResult, error) {
	prepared, errs, err := uc.prepare(ctx, rows, caller)
	if err != nil {
		return LetterImportResult{}, err
	}
	res := LetterImportResult{Errors: errs, Letters: make([]ImportedLetter, 0, len(prepared))}
	for _, p := range prepared {
		il := importedOut(p)
		il.ID, il.Number = "", 0
		res.Letters = append(res.Letters, il)
	}
	return res, nil
}

// Import books the whole file, or nothing (file header).
func (uc *CitizenLetterImport) Import(ctx context.Context, rows []domain.LetterImportRow, caller LetterCaller) (
	LetterImportResult, error) {
	if err := requireActor(caller.Actor); err != nil {
		return LetterImportResult{}, err
	}
	prepared, errs, err := uc.prepare(ctx, rows, caller)
	if err != nil {
		return LetterImportResult{}, err
	}
	if len(errs) > 0 {
		return LetterImportResult{}, &LetterImportRejected{Errors: errs}
	}

	l := uc.letters
	err = l.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		issued := make([]string, 0, len(prepared))
		for i := range prepared {
			if err := l.bookInTx(ctx, tx, &prepared[i].booking, caller); err != nil {
				return fmt.Errorf("dòng %d: %w", prepared[i].row, err)
			}
			b := prepared[i].booking.letter
			issued = append(issued, domain.LetterAuditSubject(b.Year, b.Number))
		}
		// ONE ENTRY FOR THE ACT OF IMPORTING, beside each letter's own: "who imported this file, when,
		// and which numbers did it issue" is one question an inspection asks. Numbers and a count only.
		delta, err := json.Marshal(map[string]any{
			"nguon": string(domain.LetterSourceExcel), "so_dong": len(prepared), "so_da_cap": issued,
		})
		if err != nil {
			return fmt.Errorf("don_thu: mã hoá delta nhập Excel: %w", err)
		}
		at := l.clock()
		// SAME TRANSACTION AS EVERY BOOKING (rule 6, invariant 3); TenantID filled from the transaction.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   caller.Actor,
			Action:  ActionImportCitizenLetters,
			Subject: "so-don-thu/nhap-excel/" + at.In(domain.AutomationZone).Format("2006-01-02"),
			At:      at,
			Delta:   delta,
		})
	})
	if err != nil {
		return LetterImportResult{}, wrapLetter(ctx, "nhập đơn thư từ Excel", err)
	}
	res := LetterImportResult{Letters: make([]ImportedLetter, 0, len(prepared))}
	for _, p := range prepared {
		res.Letters = append(res.Letters, importedOut(p))
	}
	return res, nil
}

type preparedImportLetter struct {
	row     int
	booking preparedBooking
}

func importedOut(p preparedImportLetter) ImportedLetter {
	l := p.booking.letter
	return ImportedLetter{Row: p.row, ID: l.ID, Number: l.Number, Year: l.Year, Type: l.Type,
		ReceivedDate: l.ReceivedDate, HoldingUnitID: l.HoldingUnitID, ProcessingDueAt: l.ProcessingDueAt,
		SenderUnknown: l.SenderUnknown()}
}

// prepare resolves the unit codes (one identity call), then runs prepareBooking on every row with a
// directory that remembers identity's answers for the length of this one file. Returns the prepared
// rows, or the row errors, or ErrLetterImportUnchecked / a system error.
func (uc *CitizenLetterImport) prepare(ctx context.Context, rows []domain.LetterImportRow, caller LetterCaller) (
	[]preparedImportLetter, []domain.LetterImportError, error) {

	if len(rows) == 0 || len(rows) > domain.MaxLetterImportRows {
		// The domain reader already refuses both; a second wall, because a caller skipping it would send
		// identity a batch over its ceiling.
		return nil, []domain.LetterImportError{{Message: fmt.Sprintf(
			"Tệp phải có từ 1 đến %d dòng đơn thư.", domain.MaxLetterImportRows)}}, nil
	}
	codes := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.UnitCode != "" {
			codes = append(codes, r.UnitCode)
		}
	}
	unitIDs := map[string]string{}
	if len(codes) > 0 {
		var err error
		if unitIDs, err = uc.units.LiveOrgUnitIDsByCode(ctx, codes); err != nil {
			return nil, nil, fmt.Errorf("%w: %w", ErrLetterImportUnchecked, err)
		}
	}

	memo := newMemoDirectory(uc.letters.dir)
	for _, id := range unitIDs {
		// LiveOrgUnitIDsByCode's predicate IS LiveOrgUnits' (its contract): the ids are not asked again.
		memo.live[id] = true
	}
	scoped := *uc.letters
	scoped.dir = memo

	var (
		out  []preparedImportLetter
		errs []domain.LetterImportError
	)
	for _, r := range rows {
		req := BookLetterRequest{
			Source:       domain.LetterSourceExcel,
			ReceivedDate: r.ReceivedDate, Type: r.Type,
			SenderName: r.SenderName, SenderPhone: r.SenderPhone, SenderAddress: r.SenderAddress,
			Summary: r.Summary,
		}
		if r.UnitCode != "" {
			id, ok := unitIDs[r.UnitCode]
			if !ok {
				errs = append(errs, domain.LetterImportError{Row: r.Row, Column: domain.LetterImportColUnit,
					Message: "Không có bộ phận đang dùng nào của xã mang mã này. Hãy kiểm tra lại mã bộ phận, hoặc để trống ô này."})
				continue
			}
			req.HoldingUnitID = id
		}
		b, err := scoped.prepareBooking(ctx, req, caller)
		if err != nil {
			if errors.Is(err, ErrLetterDirectoryUnavailable) || errors.Is(err, ErrLetterDeadlineUnavailable) {
				return nil, nil, fmt.Errorf("%w: %w", ErrLetterImportUnchecked, err)
			}
			if rowErr, ok := importRowError(r.Row, err); ok {
				errs = append(errs, rowErr)
				continue
			}
			return nil, nil, err
		}
		out = append(out, preparedImportLetter{row: r.Row, booking: b})
	}
	if len(errs) > 0 {
		return nil, errs, nil
	}
	return out, nil, nil
}

// importRowError turns a booking refusal into the row error the file report shows — the domain's own
// sentence, under the column it is about. false = not a refusal of the row (a system failure).
func importRowError(row int, err error) (domain.LetterImportError, bool) {
	e := domain.LetterImportError{Row: row}
	switch {
	case errors.Is(err, ErrLetterDeadlineUnusable):
		e.Column = domain.LetterImportColType
		e.Message = "Quy tắc thời hạn đơn thư của xã cho loại đơn này đang không dùng được. " +
			"Hãy báo quản trị xã kiểm tra lại cấu hình thời hạn đơn thư, rồi nhập lại."
		return e, true
	case errors.Is(err, domain.ErrLetterReceivedDateMissing), errors.Is(err, domain.ErrLetterReceivedDateFuture),
		errors.Is(err, domain.ErrLetterDateTooOld):
		e.Column = domain.LetterImportColReceived
	case errors.Is(err, domain.ErrLetterTypeInvalid):
		e.Column = domain.LetterImportColType
	case errors.Is(err, domain.ErrLetterSummaryMissing):
		e.Column = domain.LetterImportColSummary
	case errors.Is(err, domain.ErrLetterPhoneInvalid):
		e.Column = domain.LetterImportColPhone
	case errors.Is(err, domain.ErrLetterUnitNotLive):
		e.Column = domain.LetterImportColUnit
	}
	var le *domain.LetterError
	if !errors.As(err, &le) {
		return e, false
	}
	e.Message = le.Msg
	return e, true
}

// memoDirectory remembers identity's answers for ONE file: up to 200 rows would otherwise be 200 deadline
// round trips for what are usually a handful of (type, day) pairs. It never outlives the request, so a
// rule changed between two imports is read fresh. An ERROR is never remembered as an answer.
type memoDirectory struct {
	LetterDirectory
	live      map[string]bool
	deadlines map[deadlineKey]deadlineAnswer
}

type deadlineKey struct {
	letterType identityv1.CitizenLetterType
	kind       identityv1.CitizenLetterDeadlineKind
	countFrom  int64
}

type deadlineAnswer struct {
	due        *time.Time
	configured bool
}

func newMemoDirectory(dir LetterDirectory) *memoDirectory {
	return &memoDirectory{LetterDirectory: dir, live: map[string]bool{}, deadlines: map[deadlineKey]deadlineAnswer{}}
}

func (m *memoDirectory) LiveOrgUnits(ctx context.Context, ids []string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	var ask []string
	for _, id := range ids {
		if m.live[id] {
			out[id] = struct{}{}
		} else {
			ask = append(ask, id)
		}
	}
	if len(ask) == 0 {
		return out, nil
	}
	got, err := m.LetterDirectory.LiveOrgUnits(ctx, ask)
	if err != nil {
		return nil, err
	}
	for id := range got {
		m.live[id] = true
		out[id] = struct{}{}
	}
	return out, nil
}

func (m *memoDirectory) ResolveCitizenLetterDeadline(ctx context.Context, letterType identityv1.CitizenLetterType,
	kind identityv1.CitizenLetterDeadlineKind, countFrom time.Time) (*time.Time, bool, error) {
	k := deadlineKey{letterType, kind, countFrom.UnixNano()}
	if a, ok := m.deadlines[k]; ok {
		return copyInstant(a.due), a.configured, nil
	}
	due, configured, err := m.LetterDirectory.ResolveCitizenLetterDeadline(ctx, letterType, kind, countFrom)
	if err != nil {
		return nil, false, err
	}
	m.deadlines[k] = deadlineAnswer{due: copyInstant(due), configured: configured}
	return due, configured, nil
}

func copyInstant(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}
