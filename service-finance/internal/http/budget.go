package http

// The READ and WRITE routes of the commune's budget board (docs/ui-ux/07-thu-chi-ngan-sach.md).
//
// TWELVE ROUTES (the last three — the batches — are in budget_entry.go), THREE PERMISSIONS, and the split is the one §9 rule 7 names — `budget.read`,
// `budget.update`, `budget.confirm`. Which act sits under which is set out at each route in
// routes.go; two of them are this session's decision rather than the specification's and are written
// up as findings rather than buried.
//
//	GET    /api/v1/budget-sheets?year=&kind=      the whole sheet: columns, tree, figures, summary
//	GET    /api/v1/budget-indicators?year=        the three figures §9 rule 6 sends to /tong-quan
//	POST   /api/v1/budget-sheets                  create one year+kind sheet with its columns
//	PATCH  /api/v1/budget-sheets/{id}             title, display unit, cut-off date
//	DELETE /api/v1/budget-sheets/{id}             §6's `🗑 Gỡ`, soft, with a mandatory reason
//	POST   /api/v1/budget-lines                   `＋` / `⊞ Thêm khoản mục cấp cao nhất`
//	PATCH  /api/v1/budget-lines/{id}              rename, renumber, and type figures in
//	DELETE /api/v1/budget-lines/{id}              `🗑 Gỡ khoản mục`, soft, with a mandatory reason
//	POST   /api/v1/budget-lines/{id}/headline     the `☆` — which row the reported total comes from
//	GET    /api/v1/budget-lines/{id}/entries      the `⇄` dialog's list of batches
//	POST   /api/v1/budget-lines/{id}/entries      `+ Ghi đợt`
//	DELETE /api/v1/budget-entries/{id}            remove one batch, soft, with a mandatory reason
//
// ---------------------------------------------------------------------------
// EVERY FIGURE THIS FILE EMITS IS EITHER A NUMBER OR A SENTENCE, NEVER A ZERO STANDING IN FOR ONE.
//
// ADR 0035 §A: "chưa đủ dữ liệu để ra số thì để trống KÈM LÝ DO, không đặt mặc định. Một ô trống kèm
// lý do là một việc cần làm; một con số sai là một văn bản phải đính chính." That is why every
// summary and indicator field below is a POINTER beside an `unavailable_reason` string, and why a
// missing figure is not `0`, not `-1` and not an absent field: a client that received 0 would print
// 0, and a commune reading 0 on a card acts on it.
//
// ---------------------------------------------------------------------------
// AMOUNTS. Every figure is a JSON NUMBER OF ĐỒNG, never a formatted string and never "triệu đồng" —
// the same contract chungTuRa states. The sheet's `unit` field carries the CODE of the unit the SCREEN
// prints (`trieu-dong`, §1; closed list since 25/09/2026); the wire carries đồng so that a consumer can add two of them up without
// parsing. A percentage is `basis_points` (phần vạn) for the reason domain.BasisPoints gives: the screen
// prints two decimals, and integers of that unit compare exactly.

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// --- what leaves the API ---------------------------------------------------------------------------

type bangRa struct {
	ID   string `json:"id"`
	Code string `json:"code"` // "NS-2026-CHI-01" — the business code the audit trail is filed under
	Year int    `json:"year"`
	Kind string `json:"kind"` // `thu` | `chi`

	// Revision is `lan`. EXPOSED rather than hidden: after a `🗑 Gỡ` and a reload the commune has a
	// second sheet for the same year, and "which load is this" is the first question asked when two
	// printouts of one year disagree.
	Revision int `json:"revision"`

	Title string `json:"title"`

	// Unit is the DISPLAY unit's code — `dong` | `nghin-dong` | `trieu-dong` (domain.SheetUnit). The
	// figures on the wire are đồng whatever it says; the client divides when it draws.
	//
	// EMPTY WHEN THE STORED TEXT IS NOT ONE OF THE THREE, and then UnitWarning says so. A sheet created
	// before the list was closed may hold free text; it is mapped when the mapping is unambiguous and
	// NEVER guessed otherwise, because a wrong guess displays every figure a thousand times off.
	Unit string `json:"unit"`

	// UnitLabel is what the screen prints beside "Đơn vị tính:" — the canonical label, or the stored
	// text verbatim when it is not recognised.
	UnitLabel   string `json:"unit_label"`
	UnitWarning string `json:"unit_warning,omitempty"`

	CumulativeTo string `json:"cumulative_to,omitempty"` // YYYY-MM-DD
	SourceFile   string `json:"source_file,omitempty"`
	LoadedAt     string `json:"loaded_at,omitempty"` // RFC 3339
}

type cotRa struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
	Type  string `json:"type"` // `so` | `phan_tram`

	// Formula is emitted and NOT evaluated on the server. §9 rule 3: a percentage column is computed
	// at render, so the figure never becomes a second home for something derivable.
	Formula string `json:"formula,omitempty"`

	// Role is empty for an ordinary column. The two indicators are read from the marked ones — see
	// domain.ColumnIndicator and ADR 0035 §A.
	Role string `json:"role,omitempty"`
}

// dongRa is one line of the tree.
//
// `values` HOLDS WHAT THE SCREEN DRAWS, WHICH FOR A PARENT IS THE SUM OF ITS DIRECT CHILDREN. There
// is deliberately no second field carrying what is stored underneath: a parent cannot be typed into
// at all (the customer's 06/09/2026 decision), so the stored figures behind one are locked history,
// and emitting both would give a client two numbers for one cell with nothing saying which to print.
//
// A `null` VALUE IS AN EMPTY CELL AND IS NOT `0` (§9 rule 4). The screen draws `—`.
type dongRa struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id,omitempty"`

	// No is `tt` — "A", "I", "1.1", "-". A STRING and never a number: parsing it loses "A" and orders
	// "1.10" before "1.2".
	No    string `json:"no"`
	Name  string `json:"name"`
	Order int    `json:"order"`

	// Method is `manual` | `entries` | `children`. `children` follows from the tree. A LEAF may be
	// switched between `manual` and `entries` through PATCH (user decision 25/09/2026); POST refuses
	// the field with 400 (domain.ErrMethodFromClient) — a new line is always a `manual` leaf.
	Method string `json:"method"`

	// Level is the indent depth. OUTPUT ONLY, derived from the parent.
	Level int `json:"level"`

	// IsHeadline is the `☆`. OUTPUT ONLY on this shape; it moves through its own route, which is what
	// keeps "at most one marked row" true.
	IsHeadline bool `json:"is_headline"`

	Values map[string]*int64 `json:"values"` // columnID -> đồng, or null for an empty cell

	// UnavailableReasons is columnID -> the sentence explaining why that cell's figure CANNOT BE
	// COMPUTED (a sum past the exact range, a stored value past the ceiling — domain.FullSheet.Value).
	// Such a cell is `null` in Values AND has a key here; an EMPTY cell is `null` with no key here.
	// Omitted when every cell of the line is a figure or empty — the ordinary case.
	UnavailableReasons map[string]string `json:"unavailable_reasons,omitempty"`
}

// oTongRa is one summary cell: the marked row's figure in one numeric column (§2's "Hàng ô tóm tắt
// (sinh theo các cột của bảng)").
type oTongRa struct {
	ColumnID string `json:"column_id"`
	Name     string `json:"name"`
	Role     string `json:"role,omitempty"`
	Value    *int64 `json:"value"`

	// UnavailableReason is set when Value is null because the figure cannot be computed (or, on
	// `revenue_totals`, cannot be read from the marked row) — never set beside a number.
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

// chiSoRa is one indicator: a ratio, or the reason there is not one.
type chiSoRa struct {
	Name string `json:"name"`

	// BasisPoints is phần vạn — 10811 reads as 108,11%. NIL when there is no figure, and then
	// UnavailableReason says why in a sentence the commune can act on.
	BasisPoints       *int   `json:"basis_points"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

// soTienRa is one money figure, or the reason there is not one.
type soTienRa struct {
	Amount            *int64 `json:"amount"` // đồng
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

// tomTatRa is the report card above the grid (§2).
//
// `headline_line_id` IS EMITTED SO THE SCREEN CAN DRAW THE FILLED STAR, and `unavailable_reason` is
// what it prints instead of the cells when no row is marked. Both are needed: a card that simply
// showed nothing would read as a broken screen rather than as a job to do.
type tomTatRa struct {
	HeadlineLineID    string    `json:"headline_line_id,omitempty"`
	UnavailableReason string    `json:"unavailable_reason,omitempty"`
	Cells             []oTongRa `json:"cells"`
	Indicator         chiSoRa   `json:"indicator"`
}

type bangDayDuRa struct {
	Sheet   bangRa   `json:"sheet"`
	Columns []cotRa  `json:"columns"`
	Lines   []dongRa `json:"lines"`
	Summary tomTatRa `json:"summary"`
}

// chiSoNamRa is the KPI block §9 rule 6 feeds to `/tong-quan` and `/bao-cao`.
//
// IT IS ITS OWN ROUTE BECAUSE `balance` NEEDS BOTH SHEETS. `Cân đối thu - chi` is the revenue sheet's
// `Thu xã hưởng` minus the expenditure sheet's `Chi ngân sách` (ADR 0035 #32), so it cannot be a
// field on either sheet's own response without that sheet's route quietly reading the other one.
//
// BOTH REVENUE FIGURES REACH THE SCREEN, AND ADR 0035 §A REQUIRES IT: "màn hình phải hiện CẢ HAI số
// và gọi đúng tên từng số. Xã cần cả hai — một để báo cáo thu ngân sách, một để biết mình còn bao
// nhiêu." They are in `revenue_totals` below, each named. The thing there is only ONE of is `balance`.
type chiSoNamRa struct {
	Year int `json:"year"`

	RevenueAchievement     chiSoRa  `json:"revenue_achievement"`     // Thu đạt dự toán
	ExpenditureAchievement chiSoRa  `json:"expenditure_achievement"` // Chi đạt dự toán
	Balance                soTienRa `json:"balance"`                 // Cân đối thu - chi

	// RevenueTotals carries both revenue figures by name — ADR 0035 §A's mandatory consequence.
	RevenueTotals []oTongRa `json:"revenue_totals"`
}

// --- mapping ---------------------------------------------------------------------------------------

func sheetToOut(b domain.BudgetSheet) bangRa {
	result := bangRa{
		ID: b.ID, Code: b.Code, Year: b.Year, Kind: string(b.Kind), Revision: b.Revision,
		Title:        b.Title,
		CumulativeTo: formatDate(b.CumulativeTo), SourceFile: b.SourceFile, LoadedAt: formatTime(b.ImportedAt),
	}
	if d, ok := domain.ParseStoredUnit(b.Unit); ok {
		result.Unit, result.UnitLabel = string(d), d.Label()
	} else {
		result.UnitLabel = b.Unit
		result.UnitWarning = "Đơn vị tính đang lưu không thuộc danh sách đồng / nghìn đồng / triệu đồng — " +
			"chọn lại đơn vị cho bảng. Số liệu vẫn lưu bằng đồng và không bị quy đổi."
	}
	return result
}

func fullSheetToOut(d domain.FullSheet) bangDayDuRa {
	result := bangDayDuRa{Sheet: sheetToOut(d.Sheet)}

	result.Columns = make([]cotRa, 0, len(d.Columns))
	for _, c := range d.Columns {
		result.Columns = append(result.Columns, cotRa{
			ID: c.ID, Name: c.Name, Order: c.SortOrder, Type: string(c.Format),
			Formula: c.Formula, Role: string(c.Indicator),
		})
	}

	result.Lines = make([]dongRa, 0, len(d.Lines))
	for _, k := range d.Lines {
		value := make(map[string]*int64, len(d.Columns))
		var reason map[string]string
		for _, c := range d.Columns {
			if c.Format != domain.ColumnFormatNumber {
				// A percentage column carries no stored figure at all (§9 rule 3). Omitting it here
				// rather than sending null keeps "this column has no value" and "this cell is empty"
				// from looking the same on the wire.
				continue
			}
			s := d.Value(k.ID, c.ID)
			switch {
			case s.Present:
				v := int64(s.Value)
				value[c.ID] = &v
			case s.Reason != "":
				value[c.ID] = nil
				if reason == nil {
					reason = map[string]string{}
				}
				reason[c.ID] = s.Reason
			default:
				value[c.ID] = nil
			}
		}
		result.Lines = append(result.Lines, dongRa{
			ID: k.ID, ParentID: k.ParentID, No: k.OrdinalLabel, Name: k.Name, Order: k.SortOrder,
			Method: string(k.Method), Level: k.Level, IsHeadline: k.IsHeadline, Values: value,
			UnavailableReasons: reason,
		})
	}

	result.Summary = summaryToOut(d)
	return result
}

// summaryToOut builds the report card.
//
// THE MARKED ROW IS ASKED FOR FIRST AND ITS FAILURE ENDS THE CARD, because every cell on it is read
// from that row: with no row marked, or with two marked, there is nothing to read and the honest
// answer is the sentence saying so. Filling the cells from "the first row" instead is exactly the
// guess §5 rule 5 invites and `is_headline` exists to refuse.
func summaryToOut(d domain.FullSheet) tomTatRa {
	name, ratio := domain.EstimateAttainment(d)
	result := tomTatRa{Cells: []oTongRa{}, Indicator: indicatorToOut(name, ratio)}

	headline, err := d.Headline()
	if err != nil {
		result.UnavailableReason = err.Error()
		return result
	}
	result.HeadlineLineID = headline.ID
	for _, c := range d.Columns {
		if c.Format != domain.ColumnFormatNumber {
			continue
		}
		o := oTongRa{ColumnID: c.ID, Name: c.Name, Role: string(c.Indicator)}
		if s := d.Value(headline.ID, c.ID); s.Present {
			v := int64(s.Value)
			o.Value = &v
		} else {
			o.UnavailableReason = s.Reason // "" for an empty cell
		}
		result.Cells = append(result.Cells, o)
	}
	return result
}

func indicatorToOut(name string, t domain.Ratio) chiSoRa {
	result := chiSoRa{Name: name, UnavailableReason: t.Reason}
	if t.Present {
		v := int(t.Value)
		result.BasisPoints = &v
	}
	return result
}

func moneyFigureToOut(s domain.MoneyFigure) soTienRa {
	result := soTienRa{UnavailableReason: s.Reason}
	if s.Present {
		v := int64(s.Value)
		result.Amount = &v
	}
	return result
}

// --- what arrives ------------------------------------------------------------------------------------
//
// `omitempty` ON EVERY OPTIONAL INPUT FIELD, AND IT IS NOT COSMETIC. tools/apidoc marks a field
// REQUIRED in kb/20-contracts/openapi.json unless it carries `omitempty`, so without it these bodies
// would tell every generated client that `method` MUST be sent — on routes that answer 400 to
// exactly that.

type cotVao struct {
	Name    string `json:"name"`
	Order   int    `json:"order"`
	Type    string `json:"type"`              // `so` | `phan_tram`
	Formula string `json:"formula,omitempty"` // required for `phan_tram`, refused on `so`
	Role    string `json:"role,omitempty"`    // one of the six; empty for an ordinary column
}

// taoBangVao is the body of POST /api/v1/budget-sheets.
//
// THE COLUMNS COME WITH IT. §3 makes columns DATA rather than schema, and ADR 0035 §A makes two of
// them load-bearing — a sheet that existed for a while with no `Dự toán TP giao` column is a sheet
// whose indicator was missing for that while, with figures typed into it meanwhile.
//
// THERE IS NO `code` AND NO `revision` FIELD AND THERE MUST NEVER BE ONE. Both are the system's:
// `UNIQUE (tenant_id, ma)` counts soft-deleted rows, so a client that could name a code could burn
// one permanently.
type taoBangVao struct {
	Year  int    `json:"year"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Unit  string `json:"unit"` // `dong` | `nghin-dong` | `trieu-dong` — display only, figures stay đồng

	CumulativeTo string   `json:"cumulative_to,omitempty"` // YYYY-MM-DD
	Columns      []cotVao `json:"columns"`

	Code *string `json:"code,omitempty"` // present only so it can be refused
}

// suaBangVao is the body of PATCH /api/v1/budget-sheets/{id}. The field names are the create body's
// own, so one client model serves both.
//
// EVERY FIELD IS AN `omitempty` POINTER: absent (or null) leaves the field alone, and tools/apidoc
// declares all of them optional. `cumulative_to: ""` CLEARS the cut-off date — "not stated" is a real
// state of that column — while an empty `title` or `unit` is refused, since neither may be blank.
//
// `year`, `kind`, `code` AND `columns` ARE HERE ONLY SO THEY CAN BE REFUSED (domain
// .ErrSheetFieldNotEditable): the decoder does not reject unknown fields, so without them a client
// sending `year` would watch it vanish and believe it had moved a year's budget.
type suaBangVao struct {
	Title        *string `json:"title,omitempty"`
	CumulativeTo *string `json:"cumulative_to,omitempty"` // YYYY-MM-DD, or "" to clear
	Unit         *string `json:"unit,omitempty"`          // `dong` | `nghin-dong` | `trieu-dong`

	Year    *int     `json:"year,omitempty"`
	Kind    *string  `json:"kind,omitempty"`
	Code    *string  `json:"code,omitempty"`
	Columns []cotVao `json:"columns,omitempty"`
}

// goVao is the body of both DELETE routes.
//
// A DELETE WITH A BODY, and the alternative was worse. Rule 7, invariant 1 names three columns —
// `deleted_at`, `deleted_by`, `delete_reason` — so the reason is not optional, and the only other
// place to put it is the query string, where free text about a public authority's budget would land
// in every access log and proxy cache.
type goVao struct {
	Reason string `json:"reason"`
}

// themDongVao is the body of POST /api/v1/budget-lines.
//
// `method`, `level` AND `is_headline` ARE HERE ONLY SO THEY CAN BE REFUSED. None reaches the store.
// Declaring them and answering 400 is the difference between a client learning that these are not
// theirs to set and a client believing it has just created a parent that accepts typed figures, or a
// second row claiming to be the commune's total.
type themDongVao struct {
	SheetID  string `json:"sheet_id"`
	ParentID string `json:"parent_id,omitempty"`
	No       string `json:"no,omitempty"`
	Name     string `json:"name"`
	Order    int    `json:"order"`

	Method     *string `json:"method,omitempty"`
	Level      *int    `json:"level,omitempty"`
	IsHeadline *bool   `json:"is_headline,omitempty"`
}

// suaDongVao is the body of PATCH /api/v1/budget-lines/{id}.
//
// EVERY EDITABLE FIELD IS A POINTER, and that is the whole reason this is a PATCH and not a PUT:
// `no` is optional and its empty string is a MEANINGFUL value — §4.1's own sample has a row with no
// reference at all — so a body of plain values cannot tell "not mentioned" from "cleared".
//
// `values` IS `map[string]*int64`: a column id present with a number writes it, present with `null`
// CLEARS the cell (§9 rule 4 — empty is a state, drawn `—`, and is not 0), and absent leaves it
// alone. A figure sent for a line that has children is REFUSED, not ignored (the customer's
// 06/09/2026 decision).
//
// `sheet_id` AND `parent_id` ARE REFUSED, NOT IGNORED. Moving a line moves its figure between two
// totals that have already been read off a screen; the operation for a line in the wrong place is to
// remove it with a reason and enter it again — two events, both audited.
//
// `method` IS ACCEPTED: `manual` or `entries`, for a leaf only (app.UpdateBudgetLineRequest.Method says
// what each direction does to the stored figures). `level` and `is_headline` stay refused.
type suaDongVao struct {
	No    *string `json:"no,omitempty"`
	Name  *string `json:"name,omitempty"`
	Order *int    `json:"order,omitempty"`

	Values map[string]*int64 `json:"values,omitempty"`

	SheetID    *string `json:"sheet_id,omitempty"`
	ParentID   *string `json:"parent_id,omitempty"`
	Method     *string `json:"method,omitempty"`
	Level      *int    `json:"level,omitempty"`
	IsHeadline *bool   `json:"is_headline,omitempty"`
}

// --- the reads -----------------------------------------------------------------------------------------

// yearAndKind parses the two query parameters every read of this board needs.
//
// BOTH ARE REQUIRED AND NEITHER DEFAULTS. §13's reasoning for du_an applies unchanged: each budget
// year is its own set of figures, and a default to "this year" on a filter that decides WHICH MONEY
// is being reported is a wrong report nobody can see. `kind` has no sensible default at all — the
// two tabs are two different reports.
// THE COMMUNE IS NOT ONE OF THEM. It is fixed by httpx.TenantMiddleware from Host and reaches the
// store through the context; these two select a budget year and a tab INSIDE that commune, and
// nothing in this file reads tenant_id from a query string (rule 1, forbidden #2).
func yearAndKind(r *http.Request) (int, domain.SheetKind, string, bool) {
	rawYear := r.URL.Query().Get("year")
	year, err := strconv.Atoi(rawYear)
	if rawYear == "" || err != nil {
		return 0, "", "year", false
	}
	if err := domain.ValidateBudgetYear(year); err != nil {
		return 0, "", "year", false
	}
	kind := domain.SheetKind(r.URL.Query().Get("kind"))
	if err := domain.ValidateSheetKind(kind); err != nil {
		return 0, "", "kind", false
	}
	return year, kind, "", true
}

// GetBudgetSheet returns one whole sheet. GET /api/v1/budget-sheets?year=2026&kind=chi
func (h *Handler) GetBudgetSheet(w http.ResponseWriter, r *http.Request) {
	year, kind, field, ok := yearAndKind(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cần `year` (2000..2100) và `kind` là `thu` hoặc `chi`.", field)
		return
	}

	d, err := h.d.Budget.FullSheet(r.Context(), year, kind)
	if err != nil {
		h.writeBudgetError(w, r, "đọc bảng", err)
		return
	}
	writeJSON(w, http.StatusOK, fullSheetToOut(d))
}

// GetBudgetIndicators returns the three figures §9 rule 6 sends upward.
// GET /api/v1/budget-indicators?year=2026
//
// A MISSING SHEET IS A SENTENCE, NOT AN ERROR AND NOT A ZERO. A commune that has not entered its
// expenditure sheet has no `Chi đạt dự toán` and no `Cân đối`, and that is a fact about the data
// rather than a failure of the request — so this answers 200 with the reasons in place of the
// figures. Answering 404 would make the card disappear; answering 0 would make leadership read a
// balance the commune never claimed.
func (h *Handler) GetBudgetIndicators(w http.ResponseWriter, r *http.Request) {
	rawYear := r.URL.Query().Get("year")
	year, err := strconv.Atoi(rawYear)
	if rawYear == "" || err != nil || domain.ValidateBudgetYear(year) != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cần `year` trong khoảng 2000..2100.", "")
		return
	}

	revenue, hasRevenue, err := h.sheetOrAbsent(r, year, domain.SheetKindRevenue)
	if err != nil {
		h.writeBudgetError(w, r, "đọc chỉ số", err)
		return
	}
	expenditure, hasExpenditure, err := h.sheetOrAbsent(r, year, domain.SheetKindExpenditure)
	if err != nil {
		h.writeBudgetError(w, r, "đọc chỉ số", err)
		return
	}

	result := chiSoNamRa{Year: year, RevenueTotals: []oTongRa{}}

	if hasRevenue {
		name, ratio := domain.EstimateAttainment(revenue)
		result.RevenueAchievement = indicatorToOut(name, ratio)
		// ADR 0035 §A: BOTH revenue figures, each by its own name. The commune needs one to report
		// its revenue and the other to know what it may actually spend, and a screen showing one of
		// them answers a question nobody asked.
		for _, indicator := range []domain.ColumnIndicator{domain.IndicatorStateBudgetRevenue, domain.IndicatorCommuneRetainedRevenue} {
			column, hasColumn := revenue.ColumnByIndicator(indicator)
			if !hasColumn {
				continue
			}
			o := oTongRa{ColumnID: column.ID, Name: column.Name, Role: string(indicator)}
			g, ok, err := revenue.HeadlineValue(indicator)
			switch {
			case err != nil:
				// The sentence and not a bare null: the figure is unavailable for a reason the
				// commune can act on (no marked row, an empty cell, a sum past the exact range).
				o.UnavailableReason = err.Error()
			case ok:
				v := int64(g)
				o.Value = &v
			}
			result.RevenueTotals = append(result.RevenueTotals, o)
		}
	} else {
		result.RevenueAchievement = chiSoRa{Name: "Thu đạt dự toán",
			UnavailableReason: fistore.ErrBudgetSheetNotFound.Error()}
	}

	if hasExpenditure {
		name, ratio := domain.EstimateAttainment(expenditure)
		result.ExpenditureAchievement = indicatorToOut(name, ratio)
	} else {
		result.ExpenditureAchievement = chiSoRa{Name: "Chi đạt dự toán",
			UnavailableReason: fistore.ErrBudgetSheetNotFound.Error()}
	}

	result.Balance = moneyFigureToOut(domain.RevenueExpenditureBalance(revenue, expenditure))
	writeJSON(w, http.StatusOK, result)
}

// sheetOrAbsent reads one sheet and turns "this commune has no such sheet" into an ABSENCE rather
// than an error, leaving every other failure a failure.
//
// THE DISTINCTION IS THE POINT. "No expenditure sheet for 2026" is a state of the commune's data
// that the card has a sentence for; "the database is unreachable" is not, and collapsing the two
// would let an outage print as "xã chưa có bảng ngân sách" on a government screen.
func (h *Handler) sheetOrAbsent(r *http.Request, year int,
	kind domain.SheetKind) (domain.FullSheet, bool, error) {

	d, err := h.d.Budget.FullSheet(r.Context(), year, kind)
	if errors.Is(err, fistore.ErrBudgetSheetNotFound) {
		return domain.FullSheet{}, false, nil
	}
	if err != nil {
		return domain.FullSheet{}, false, err
	}
	return d, true, nil
}

// --- the writes ----------------------------------------------------------------------------------------

// CreateBudgetSheet creates one sheet with its columns. POST /api/v1/budget-sheets
func (h *Handler) CreateBudgetSheet(w http.ResponseWriter, r *http.Request) {
	var in taoBangVao
	if !readBody(w, r, &in) {
		return
	}
	if in.Code != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`code` không do client đặt — mã bảng do hệ thống cấp và không bao giờ cấp lại.", "")
		return
	}
	cumulativeTo, ok := parseDate(in.CumulativeTo)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`cumulative_to` phải theo dạng YYYY-MM-DD, ví dụ 2026-08-25.", "")
		return
	}

	column := make([]domain.BudgetColumn, 0, len(in.Columns))
	for _, c := range in.Columns {
		column = append(column, domain.BudgetColumn{
			Name: c.Name, SortOrder: c.Order, Format: domain.ColumnFormat(c.Type),
			Formula: c.Formula, Indicator: domain.ColumnIndicator(c.Role),
		})
	}

	actor, ok := actorFrom(r)
	if !ok {
		h.noPrincipal(w, r)
		return
	}

	next, err := h.d.BudgetWriter.CreateSheet(r.Context(), app.CreateBudgetSheetRequest{
		Year: in.Year, Kind: domain.SheetKind(in.Kind), Title: in.Title,
		Unit: in.Unit, CumulativeTo: cumulativeTo, Columns: column,
	}, actor)
	if err != nil {
		h.writeBudgetError(w, r, "tạo bảng", err)
		return
	}

	// What a retry carrying the same Idempotency-Key is told about. THE CODE AND NOT THE BODY: the
	// body would go into Redis, which is a cache and not a record store.
	idem.RecordCode(r.Context(), next.Code)
	writeJSON(w, http.StatusCreated, sheetToOut(next))
}

// RemoveBudgetSheet soft deletes one sheet — §6's `🗑 Gỡ`. DELETE /api/v1/budget-sheets/{id}
//
// 204 AND NO BODY. The sheet and its whole tree are still in the table carrying `deleted_at`,
// `deleted_by` and `delete_reason`, but there is nothing the caller can do with them, and returning
// the sheet would invite a client to display a year it has just taken off the screen.
func (h *Handler) RemoveBudgetSheet(w http.ResponseWriter, r *http.Request) {
	var in goVao
	if !readBody(w, r, &in) {
		return
	}
	actor, ok := actorFrom(r)
	if !ok {
		h.noPrincipal(w, r)
		return
	}
	if err := h.d.BudgetWriter.RemoveSheet(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeBudgetError(w, r, "gỡ bảng", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateBudgetSheet edits one sheet's title, display unit and cut-off date.
// PATCH /api/v1/budget-sheets/{id}
//
// Changing `unit` changes only how the figures are DISPLAYED; every stored figure is đồng and none
// is converted (domain.SheetUnit).
func (h *Handler) UpdateBudgetSheet(w http.ResponseWriter, r *http.Request) {
	var in suaBangVao
	if !readBody(w, r, &in) {
		return
	}
	if in.Year != nil || in.Kind != nil || in.Code != nil || in.Columns != nil {
		h.writeBudgetError(w, r, "sửa bảng", domain.ErrSheetFieldNotEditable)
		return
	}

	req := app.UpdateBudgetSheetRequest{Title: in.Title, Unit: in.Unit}
	if in.CumulativeTo != nil {
		// parseDate maps "" to the zero time, which is exactly "clear the cut-off date".
		cumulativeTo, ok := parseDate(*in.CumulativeTo)
		if !ok {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"`cumulative_to` phải theo dạng YYYY-MM-DD, ví dụ 2026-08-25, hoặc chuỗi rỗng để bỏ mốc.", "")
			return
		}
		req.CumulativeTo = &cumulativeTo
	}

	actor, ok := actorFrom(r)
	if !ok {
		h.noPrincipal(w, r)
		return
	}
	after, err := h.d.BudgetWriter.UpdateSheet(r.Context(), r.PathValue("id"), req, actor)
	if err != nil {
		h.writeBudgetError(w, r, "sửa bảng", err)
		return
	}
	writeJSON(w, http.StatusOK, sheetToOut(after))
}

// CreateBudgetLine adds one line. POST /api/v1/budget-lines
func (h *Handler) CreateBudgetLine(w http.ResponseWriter, r *http.Request) {
	var in themDongVao
	if !readBody(w, r, &in) {
		return
	}
	// BEFORE ANYTHING ELSE, so the caller learns that these three are not theirs to set rather than
	// watching the fields disappear.
	if err := notClientSettable(in.Method != nil, in.Level != nil, in.IsHeadline != nil); err != nil {
		h.writeBudgetError(w, r, "thêm khoản mục", err)
		return
	}

	actor, ok := actorFrom(r)
	if !ok {
		h.noPrincipal(w, r)
		return
	}

	next, err := h.d.BudgetWriter.CreateLine(r.Context(), app.CreateBudgetLineRequest{
		SheetID: in.SheetID, ParentID: in.ParentID, OrdinalLabel: in.No, Name: in.Name, SortOrder: in.Order,
	}, actor)
	if err != nil {
		h.writeBudgetError(w, r, "thêm khoản mục", err)
		return
	}

	idem.RecordCode(r.Context(), next.ID)
	writeJSON(w, http.StatusCreated, lineToOut(next))
}

// UpdateBudgetLine edits one line's text and its figures. PATCH /api/v1/budget-lines/{id}
func (h *Handler) UpdateBudgetLine(w http.ResponseWriter, r *http.Request) {
	var in suaDongVao
	if !readBody(w, r, &in) {
		return
	}
	if err := notClientSettable(false, in.Level != nil, in.IsHeadline != nil); err != nil {
		h.writeBudgetError(w, r, "sửa khoản mục", err)
		return
	}
	if in.SheetID != nil || in.ParentID != nil {
		h.writeBudgetError(w, r, "sửa khoản mục", domain.ErrParentInOtherSheet)
		return
	}

	value := map[string]*domain.Dong{}
	for columnID, v := range in.Values {
		if v == nil {
			value[columnID] = nil
			continue
		}
		g := domain.Dong(*v)
		value[columnID] = &g
	}

	actor, ok := actorFrom(r)
	if !ok {
		h.noPrincipal(w, r)
		return
	}

	req := app.UpdateBudgetLineRequest{OrdinalLabel: in.No, Name: in.Name, SortOrder: in.Order, Values: value}
	if in.Method != nil {
		// Refused HERE, before the use case, as well as in it: `children` or any other value is the
		// client naming something that follows the tree, and the answer is the same 400 either way.
		method := domain.LineMethod(*in.Method)
		if err := domain.ValidateChosenMethod(method); err != nil {
			h.writeBudgetError(w, r, "sửa khoản mục", err)
			return
		}
		req.Method = &method
	}
	after, err := h.d.BudgetWriter.UpdateLine(r.Context(), r.PathValue("id"), req, actor)
	if err != nil {
		h.writeBudgetError(w, r, "sửa khoản mục", err)
		return
	}
	writeJSON(w, http.StatusOK, lineToOut(after))
}

// RemoveBudgetLine soft deletes one line. DELETE /api/v1/budget-lines/{id}
func (h *Handler) RemoveBudgetLine(w http.ResponseWriter, r *http.Request) {
	var in goVao
	if !readBody(w, r, &in) {
		return
	}
	actor, ok := actorFrom(r)
	if !ok {
		h.noPrincipal(w, r)
		return
	}
	if err := h.d.BudgetWriter.RemoveLine(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeBudgetError(w, r, "gỡ khoản mục", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetBudgetHeadline marks one line as the sheet's total row — the `☆`.
// POST /api/v1/budget-lines/{id}/headline
//
// `headline` IS A NOMINALISED SUB-RESOURCE, NOT THE VERB `mark`. A verb in a path is what
// skills/rest-api-design forbids and `rest_api_guard` reports; `headline` is the STATE, and POST
// creating it on one row is the same shape `lockout` already uses on a voucher.
//
// POST AND NO DELETE, DELIBERATELY. §4.1 draws the star as something that MOVES — "bấm ngôi sao ở
// đầu một dòng khác để đổi" — and a sheet that HAD a total and then deliberately had none is a state
// nothing on that screen asks for. A sheet loses its total only by the marked row being removed,
// which releases the flag; the summary card then says so in a sentence.
//
// NO REQUEST BODY: the act carries no information beyond which row and who, and both are already in
// the path and the session.
func (h *Handler) SetBudgetHeadline(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFrom(r)
	if !ok {
		h.noPrincipal(w, r)
		return
	}
	after, err := h.d.BudgetWriter.SetHeadline(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		h.writeBudgetError(w, r, "đặt dòng tổng", err)
		return
	}
	writeJSON(w, http.StatusOK, lineToOut(after))
}

// lineToOut is one line WITHOUT its figures.
//
// `values` IS NIL ON THIS SHAPE ON PURPOSE. A line's displayed figures depend on the whole tree — a
// parent's cell is the sum of its direct children — and a write route holds one row, not the sheet.
// Sending a `values` map built from the row alone would give a parent's cells the LOCKED figures
// underneath it rather than the ones on the screen, which is a wrong number arriving at the moment
// the client is most likely to trust it. The client refetches the sheet; the route confirms the act.
func lineToOut(k domain.BudgetLine) dongRa {
	return dongRa{
		ID: k.ID, ParentID: k.ParentID, No: k.OrdinalLabel, Name: k.Name, Order: k.SortOrder,
		Method: string(k.Method), Level: k.Level, IsHeadline: k.IsHeadline,
	}
}

// notClientSettable turns "the client named a field that is not theirs" into the domain's own sentence.
func notClientSettable(hasMethod, hasLevel, hasHeadline bool) error {
	switch {
	case hasMethod:
		return domain.ErrMethodFromClient
	case hasLevel:
		return domain.ErrLevelFromClient
	case hasHeadline:
		// REFUSED RATHER THAN IGNORED. The flag is reachable from no layer above the store except
		// SetHeadline's own statement pair, so ignoring it would be safe and would leave the client
		// believing it had just created the commune's total row — a second marked row as far as the
		// client is concerned, and the summary card would then disagree with what it thinks it did.
		return domain.ErrHeadlineHasOwnRoute
	}
	return nil
}

// --- errors ----------------------------------------------------------------------------------------------

// writeBudgetError maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR ALL NINE ROUTES, because nine copies of this mapping would drift and the copy
// that drifts is the one answering 500 where it meant 409 — which reads to an operator as a broken
// server rather than as a rule doing its job.
//
// WHY 409 AND NOT 403 FOR EVERY STRUCTURAL REFUSAL: the caller HOLDS the permission and is allowed
// to perform the operation. What is refused is this operation on THIS row, because of the shape of
// the sheet. 403 would send an accountant to the Phân quyền screen to be granted a right they
// already have — and in the "parent line" case, a right that would change nothing, because the rule
// is about the tree and not about the permission.
//
// THE DOMAIN'S OWN SENTENCE IS RETURNED ON EVERY REFUSAL, deliberately. It names the operation and
// the way out, holds no personal data and no internal detail, and a second sentence written here
// would drift from it. What must NEVER reach a client is the PostgreSQL exception underneath.
func (h *Handler) writeBudgetError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, fistore.ErrBudgetSheetNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy bảng ngân sách này.", "")
	case errors.Is(err, domain.ErrLineNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy khoản mục này.", "")
	case errors.Is(err, domain.ErrEntryNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy đợt thu, chi này.", "")
	case errors.Is(err, fistore.ErrSheetExists):
		httpx.WriteError(w, http.StatusConflict, "sheet_exists", err.Error(), "")
	case errors.Is(err, domain.ErrParentLineNoDirectValue),
		errors.Is(err, domain.ErrLineHasChildren),
		errors.Is(err, domain.ErrMultipleHeadlines),
		errors.Is(err, domain.ErrEntryLeafOnly),
		errors.Is(err, domain.ErrEntriesLineNoDirectValue),
		errors.Is(err, domain.ErrEntriesLineHasEntries),
		errors.Is(err, domain.ErrParentLineMethodFixed),
		errors.Is(err, domain.ErrLineEntriesFull),
		// Checked BEFORE the 400 list: on entries -> manual it is wrapped together with
		// ErrValueTooLarge, and the act that fixes it is removing a batch, not resending the body.
		errors.Is(err, domain.ErrEntryTotalOverflow):
		// The domain sentence names the rule and the way out. None of these carries a figure or
		// any batch text — `counterparty` never reaches an error (rule 3, forbidden #3).
		httpx.WriteError(w, http.StatusConflict, "budget_tree", err.Error(), "")
	case errors.Is(err, fistore.ErrTooManyLines), errors.Is(err, fistore.ErrTooManyEntries):
		// 500 AND NOT 409, because this is not something the caller did: the sheet in the database is
		// past a bound this service refuses to truncate, and the person in front of the screen has no
		// act available. It names itself in the log, which is where the operator will look.
		h.d.Log.Error("ngân sách: vượt trần khoản mục hoặc trần đợt",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	case isBudgetInputError(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3). The commune is logged because it is the only thing an operator can act on;
		// the figures and the line names are NOT, because a log line travels into centralised logging
		// across every commune at once.
		h.d.Log.Error("ngân sách thu chi: "+op+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// isBudgetInputError reports whether this is a refusal of what the client sent, as opposed to a
// failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400. A default of "anything I do not recognise is the
// client's fault" turns a database outage into a 400, and a client that believes its input is wrong
// retries with different input forever while nobody is told the server is broken.
func isBudgetInputError(err error) bool {
	for _, one := range []error{
		domain.ErrSheetMissing, domain.ErrInvalidSheetKind, domain.ErrYearOutOfRange,
		domain.ErrInvalidColumnFormat, domain.ErrInvalidIndicator, domain.ErrIndicatorWrongSheetKind,
		domain.ErrIndicatorOnPercentColumn, domain.ErrFormulaMissing, domain.ErrFormulaUnexpected,
		domain.ErrIndicatorDuplicateInSheet,
		domain.ErrTitleMissing, domain.ErrTitleTooLong,
		domain.ErrUnitMissing, domain.ErrInvalidUnit, domain.ErrSheetFieldNotEditable,
		domain.ErrColumnNameMissing, domain.ErrColumnNameTooLong, domain.ErrFormulaTooLong,
		domain.ErrNoColumns, domain.ErrTooManyColumns,
		domain.ErrLineNameMissing, domain.ErrLineNameTooLong, domain.ErrOrdinalLabelTooLong,
		domain.ErrBudgetSortOrderOutOfRange, domain.ErrValueTooLarge,
		domain.ErrBudgetRemoveReasonMissing, domain.ErrBudgetRemoveReasonTooLong,
		domain.ErrMethodFromClient, domain.ErrLevelFromClient, domain.ErrHeadlineHasOwnRoute,
		domain.ErrParentInOtherSheet, domain.ErrParentIsSelf, domain.ErrParentCycle,
		domain.ErrColumnInOtherSheet, domain.ErrColumnNotNumber,
		domain.ErrEntryDateMissing, domain.ErrEntryDateOutOfRange,
		domain.ErrEntryContentMissing, domain.ErrEntryContentTooLong,
		domain.ErrEntryCounterpartyTooLong, domain.ErrEntryVoucherNoTooLong, domain.ErrEntryHasNoAmount,
	} {
		if errors.Is(err, one) {
			return true
		}
	}
	return false
}
