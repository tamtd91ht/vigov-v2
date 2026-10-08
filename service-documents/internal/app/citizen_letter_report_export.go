package app

// `Xuất Excel` on the Báo cáo tab (owner spec §5; prototype router.py:371-395 + report_sheet.py) —
// GET /api/v1/citizen-letter-report/exports?year=, `report.export` AND `petition.read`.
//
// THE SAME FIGURES AS GET /api/v1/citizen-letter-report: Report below is the very method that route
// calls, so the file and the screen cannot disagree. AGGREGATES ONLY — counts, a percentage, an
// average, unit names. The report carries no letter, no sender, no summary (domain.LetterReport), so
// nothing personal can reach the file.
//
// THE ORDER IS petitions' register export's (service-petitions app/task_register_export.go), and the
// order is the rule:
//
//  1. READ the figures, and the unit NAMES identity holds for the unit ids (removed units included,
//     flagged). An identity failure produces NO FILE (503): a sheet printing an id or a blank where a
//     unit's name belongs is a statement about the commune's register made from a failure to read it.
//  2. RENDER through the handler's callback (an .xlsx is presentation).
//  3. RECORD the export in `audit_log` — rule 3 invariant 4's "export is audited" — and only THEN hand
//     the bytes back. A render failure writes no entry; an audit failure returns no bytes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// ActionExportCitizenLetterReport is the trail's verb.
const ActionExportCitizenLetterReport = "xuat_bao_cao_don_thu"

// ErrLetterReportNamesUnavailable — identity could not name the units. No file. 503, retryable.
var ErrLetterReportNamesUnavailable = errors.New("don_thu: chưa tra được tên bộ phận cho báo cáo đơn thư")

// LetterUnitNames names org units by id, removed ones included. *identityclient.Client satisfies it.
type LetterUnitNames interface {
	OrgUnitNames(ctx context.Context, ids []string) (map[string]identityclient.OrgUnitName, error)
}

// LetterReportUnitName is how the sheet names one unit row. Known false = identity had no name for the
// id (unknown, or another commune's — indistinguishable by design); Removed = the unit has since been
// taken off the org chart.
type LetterReportUnitName struct {
	Name    string
	Known   bool
	Removed bool
}

// LetterReportSheet is what the render callback gets: the report and the names of its units. A unit row
// with UnitID "" is the letters held by no unit (`Chưa phân công`).
type LetterReportSheet struct {
	Report    domain.LetterReport
	UnitNames map[string]LetterReportUnitName
}

// CitizenLetterReportExport is the use case.
type CitizenLetterReportExport struct {
	letters *CitizenLetters
	names   LetterUnitNames
}

func NewCitizenLetterReportExport(letters *CitizenLetters, names LetterUnitNames) *CitizenLetterReportExport {
	return &CitizenLetterReportExport{letters: letters, names: names}
}

// Export reads, renders, records, returns — in that order (file header).
func (uc *CitizenLetterReportExport) Export(ctx context.Context, year int, actor audit.Actor,
	render func(LetterReportSheet) ([]byte, error)) ([]byte, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}

	// 1. READ.
	rep, err := uc.letters.Report(ctx, year)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, u := range rep.ByUnit {
		if u.UnitID != "" {
			ids = append(ids, u.UnitID)
		}
	}
	names := make(map[string]LetterReportUnitName, len(ids))
	// Chunked at the client's ceiling, never truncated (identityclient.MaxLookupKeysPerCall).
	for start := 0; start < len(ids); start += identityclient.MaxLookupKeysPerCall {
		end := min(start+identityclient.MaxLookupKeysPerCall, len(ids))
		got, err := uc.names.OrgUnitNames(ctx, ids[start:end])
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrLetterReportNamesUnavailable, err)
		}
		for id, n := range got {
			names[id] = LetterReportUnitName{Name: n.Name, Known: true,
				Removed: n.Standing == identityv1.RecordStanding_RECORD_STANDING_REMOVED}
		}
	}

	// 2. RENDER.
	file, err := render(LetterReportSheet{Report: rep, UnitNames: names})
	if err != nil {
		return nil, fmt.Errorf("don_thu: dựng tệp báo cáo đơn thư: %w", err)
	}

	// 3. RECORD. The figures that left are in the file; the trail says who took which year's report.
	delta, err := json.Marshal(map[string]any{
		"nam": year, "tiep_nhan": rep.Received, "da_giai_quyet": rep.Resolved, "dang_xu_ly": rep.InProgress,
		"qua_han": rep.PastDue, "so_bo_phan": len(rep.ByUnit),
	})
	if err != nil {
		return nil, fmt.Errorf("don_thu: mã hoá delta xuất báo cáo: %w", err)
	}
	if err := uc.letters.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionExportCitizenLetterReport,
			Subject: "bao-cao-don-thu/" + strconv.Itoa(year),
			At:      uc.letters.clock(),
			Delta:   delta,
		})
	}); err != nil {
		return nil, wrapLetter(ctx, "ghi vết xuất báo cáo đơn thư", err)
	}
	return file, nil
}
