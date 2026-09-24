package http

// The READ and WRITE routes of the commune's budget board (docs/ui-ux/07-thu-chi-ngan-sach.md).
//
// NINE ROUTES, THREE PERMISSIONS, and the split is the one §9 rule 7 names — `budget.read`,
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
// parsing. A percentage is `basis_points` (phần vạn) for the reason domain.PhanVan gives: the screen
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

	// Unit is the DISPLAY unit's code — `dong` | `nghin-dong` | `trieu-dong` (domain.DonViTinh). The
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
	// domain.VaiTroCot and ADR 0035 §A.
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

	// Method is `manual` | `children`. OUTPUT ONLY — it follows from the tree, and a request carrying
	// it is refused with 400 (domain.ErrCachTinhDoTuClient).
	Method string `json:"method"`

	// Level is the indent depth. OUTPUT ONLY, derived from the parent.
	Level int `json:"level"`

	// IsHeadline is the `☆`. OUTPUT ONLY on this shape; it moves through its own route, which is what
	// keeps "at most one marked row" true.
	IsHeadline bool `json:"is_headline"`

	Values map[string]*int64 `json:"values"` // columnID -> đồng, or null for an empty cell
}

// oTongRa is one summary cell: the marked row's figure in one numeric column (§2's "Hàng ô tóm tắt
// (sinh theo các cột của bảng)").
type oTongRa struct {
	ColumnID string `json:"column_id"`
	Name     string `json:"name"`
	Role     string `json:"role,omitempty"`
	Value    *int64 `json:"value"`
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

func bangRaNgoai(b domain.BangNganSach) bangRa {
	ra := bangRa{
		ID: b.ID, Code: b.Ma, Year: b.Nam, Kind: string(b.Loai), Revision: b.Lan,
		Title:        b.TieuDe,
		CumulativeTo: ngayRa(b.LuyKeDen), SourceFile: b.NguonTep, LoadedAt: lucRa(b.NapLuc),
	}
	if d, co := domain.DocDonViTinhDaLuu(b.DonViTinh); co {
		ra.Unit, ra.UnitLabel = string(d), d.Nhan()
	} else {
		ra.UnitLabel = b.DonViTinh
		ra.UnitWarning = "Đơn vị tính đang lưu không thuộc danh sách đồng / nghìn đồng / triệu đồng — " +
			"chọn lại đơn vị cho bảng. Số liệu vẫn lưu bằng đồng và không bị quy đổi."
	}
	return ra
}

func bangDayDuRaNgoai(d domain.BangDayDu) bangDayDuRa {
	ra := bangDayDuRa{Sheet: bangRaNgoai(d.Bang)}

	ra.Columns = make([]cotRa, 0, len(d.Cot))
	for _, c := range d.Cot {
		ra.Columns = append(ra.Columns, cotRa{
			ID: c.ID, Name: c.Ten, Order: c.ThuTu, Type: string(c.Kieu),
			Formula: c.CongThuc, Role: string(c.VaiTro),
		})
	}

	ra.Lines = make([]dongRa, 0, len(d.KhoanMuc))
	for _, k := range d.KhoanMuc {
		gia := make(map[string]*int64, len(d.Cot))
		for _, c := range d.Cot {
			if c.Kieu != domain.CotSo {
				// A percentage column carries no stored figure at all (§9 rule 3). Omitting it here
				// rather than sending null keeps "this column has no value" and "this cell is empty"
				// from looking the same on the wire.
				continue
			}
			if g, co := d.GiaTri(k.ID, c.ID); co {
				v := int64(g)
				gia[c.ID] = &v
			} else {
				gia[c.ID] = nil
			}
		}
		ra.Lines = append(ra.Lines, dongRa{
			ID: k.ID, ParentID: k.ChaID, No: k.TT, Name: k.Ten, Order: k.ThuTu,
			Method: string(k.CachTinh), Level: k.Cap, IsHeadline: k.LaDongTong, Values: gia,
		})
	}

	ra.Summary = tomTatRaNgoai(d)
	return ra
}

// tomTatRaNgoai builds the report card.
//
// THE MARKED ROW IS ASKED FOR FIRST AND ITS FAILURE ENDS THE CARD, because every cell on it is read
// from that row: with no row marked, or with two marked, there is nothing to read and the honest
// answer is the sentence saying so. Filling the cells from "the first row" instead is exactly the
// guess §5 rule 5 invites and `is_headline` exists to refuse.
func tomTatRaNgoai(d domain.BangDayDu) tomTatRa {
	ten, ty := domain.ChiSoDatDuToan(d)
	ra := tomTatRa{Cells: []oTongRa{}, Indicator: chiSoRaNgoai(ten, ty)}

	dong, err := d.DongTong()
	if err != nil {
		ra.UnavailableReason = err.Error()
		return ra
	}
	ra.HeadlineLineID = dong.ID
	for _, c := range d.Cot {
		if c.Kieu != domain.CotSo {
			continue
		}
		o := oTongRa{ColumnID: c.ID, Name: c.Ten, Role: string(c.VaiTro)}
		if g, co := d.GiaTri(dong.ID, c.ID); co {
			v := int64(g)
			o.Value = &v
		}
		ra.Cells = append(ra.Cells, o)
	}
	return ra
}

func chiSoRaNgoai(ten string, t domain.TyLe) chiSoRa {
	ra := chiSoRa{Name: ten, UnavailableReason: t.LyDo}
	if t.Co {
		v := int(t.Gia)
		ra.BasisPoints = &v
	}
	return ra
}

func soTienRaNgoai(s domain.SoTien) soTienRa {
	ra := soTienRa{UnavailableReason: s.LyDo}
	if s.Co {
		v := int64(s.Gia)
		ra.Amount = &v
	}
	return ra
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
// .ErrTruongBangKhongSua): the decoder does not reject unknown fields, so without them a client
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

// namVaLoai parses the two query parameters every read of this board needs.
//
// BOTH ARE REQUIRED AND NEITHER DEFAULTS. §13's reasoning for du_an applies unchanged: each budget
// year is its own set of figures, and a default to "this year" on a filter that decides WHICH MONEY
// is being reported is a wrong report nobody can see. `kind` has no sensible default at all — the
// two tabs are two different reports.
// THE COMMUNE IS NOT ONE OF THEM. It is fixed by httpx.TenantMiddleware from Host and reaches the
// store through the context; these two select a budget year and a tab INSIDE that commune, and
// nothing in this file reads tenant_id from a query string (rule 1, forbidden #2).
func namVaLoai(r *http.Request) (int, domain.LoaiBang, string, bool) {
	namTho := r.URL.Query().Get("year")
	nam, err := strconv.Atoi(namTho)
	if namTho == "" || err != nil {
		return 0, "", "year", false
	}
	if err := domain.KiemTraNamNganSach(nam); err != nil {
		return 0, "", "year", false
	}
	loai := domain.LoaiBang(r.URL.Query().Get("kind"))
	if err := domain.KiemTraLoaiBang(loai); err != nil {
		return 0, "", "kind", false
	}
	return nam, loai, "", true
}

// DocBangNganSach returns one whole sheet. GET /api/v1/budget-sheets?year=2026&kind=chi
func (h *Handler) DocBangNganSach(w http.ResponseWriter, r *http.Request) {
	nam, loai, truong, ok := namVaLoai(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cần `year` (2000..2100) và `kind` là `thu` hoặc `chi`.", truong)
		return
	}

	d, err := h.d.NganSach.BangDayDu(r.Context(), nam, loai)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "đọc bảng", err)
		return
	}
	vietJSON(w, http.StatusOK, bangDayDuRaNgoai(d))
}

// DocChiSoNganSach returns the three figures §9 rule 6 sends upward.
// GET /api/v1/budget-indicators?year=2026
//
// A MISSING SHEET IS A SENTENCE, NOT AN ERROR AND NOT A ZERO. A commune that has not entered its
// expenditure sheet has no `Chi đạt dự toán` and no `Cân đối`, and that is a fact about the data
// rather than a failure of the request — so this answers 200 with the reasons in place of the
// figures. Answering 404 would make the card disappear; answering 0 would make leadership read a
// balance the commune never claimed.
func (h *Handler) DocChiSoNganSach(w http.ResponseWriter, r *http.Request) {
	namTho := r.URL.Query().Get("year")
	nam, err := strconv.Atoi(namTho)
	if namTho == "" || err != nil || domain.KiemTraNamNganSach(nam) != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cần `year` trong khoảng 2000..2100.", "")
		return
	}

	thu, coThu, err := h.bangHoacVang(r, nam, domain.BangThu)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "đọc chỉ số", err)
		return
	}
	chi, coChi, err := h.bangHoacVang(r, nam, domain.BangChi)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "đọc chỉ số", err)
		return
	}

	ra := chiSoNamRa{Year: nam, RevenueTotals: []oTongRa{}}

	if coThu {
		ten, ty := domain.ChiSoDatDuToan(thu)
		ra.RevenueAchievement = chiSoRaNgoai(ten, ty)
		// ADR 0035 §A: BOTH revenue figures, each by its own name. The commune needs one to report
		// its revenue and the other to know what it may actually spend, and a screen showing one of
		// them answers a question nobody asked.
		for _, vai := range []domain.VaiTroCot{domain.VaiTroThuNSNN, domain.VaiTroThuXaHuong} {
			cot, coCot := thu.CotTheoVaiTro(vai)
			if !coCot {
				continue
			}
			o := oTongRa{ColumnID: cot.ID, Name: cot.Ten, Role: string(vai)}
			if g, co, _ := thu.SoTong(vai); co {
				v := int64(g)
				o.Value = &v
			}
			ra.RevenueTotals = append(ra.RevenueTotals, o)
		}
	} else {
		ra.RevenueAchievement = chiSoRa{Name: "Thu đạt dự toán",
			UnavailableReason: fistore.ErrKhongThayBangNganSach.Error()}
	}

	if coChi {
		ten, ty := domain.ChiSoDatDuToan(chi)
		ra.ExpenditureAchievement = chiSoRaNgoai(ten, ty)
	} else {
		ra.ExpenditureAchievement = chiSoRa{Name: "Chi đạt dự toán",
			UnavailableReason: fistore.ErrKhongThayBangNganSach.Error()}
	}

	ra.Balance = soTienRaNgoai(domain.CanDoiThuChi(thu, chi))
	vietJSON(w, http.StatusOK, ra)
}

// bangHoacVang reads one sheet and turns "this commune has no such sheet" into an ABSENCE rather
// than an error, leaving every other failure a failure.
//
// THE DISTINCTION IS THE POINT. "No expenditure sheet for 2026" is a state of the commune's data
// that the card has a sentence for; "the database is unreachable" is not, and collapsing the two
// would let an outage print as "xã chưa có bảng ngân sách" on a government screen.
func (h *Handler) bangHoacVang(r *http.Request, nam int,
	loai domain.LoaiBang) (domain.BangDayDu, bool, error) {

	d, err := h.d.NganSach.BangDayDu(r.Context(), nam, loai)
	if errors.Is(err, fistore.ErrKhongThayBangNganSach) {
		return domain.BangDayDu{}, false, nil
	}
	if err != nil {
		return domain.BangDayDu{}, false, err
	}
	return d, true, nil
}

// --- the writes ----------------------------------------------------------------------------------------

// TaoBangNganSach creates one sheet with its columns. POST /api/v1/budget-sheets
func (h *Handler) TaoBangNganSach(w http.ResponseWriter, r *http.Request) {
	var vao taoBangVao
	if !docThan(w, r, &vao) {
		return
	}
	if vao.Code != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`code` không do client đặt — mã bảng do hệ thống cấp và không bao giờ cấp lại.", "")
		return
	}
	luyKe, ok := ngayVao(vao.CumulativeTo)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`cumulative_to` phải theo dạng YYYY-MM-DD, ví dụ 2026-08-25.", "")
		return
	}

	cot := make([]domain.CotNganSach, 0, len(vao.Columns))
	for _, c := range vao.Columns {
		cot = append(cot, domain.CotNganSach{
			Ten: c.Name, ThuTu: c.Order, Kieu: domain.KieuCot(c.Type),
			CongThuc: c.Formula, VaiTro: domain.VaiTroCot(c.Role),
		})
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	moi, err := h.d.GhiNganSach.TaoBang(r.Context(), app.YeuCauTaoBang{
		Nam: vao.Year, Loai: domain.LoaiBang(vao.Kind), TieuDe: vao.Title,
		DonViTinh: vao.Unit, LuyKeDen: luyKe, Cot: cot,
	}, nguoi)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "tạo bảng", err)
		return
	}

	// What a retry carrying the same Idempotency-Key is told about. THE CODE AND NOT THE BODY: the
	// body would go into Redis, which is a cache and not a record store.
	idem.RecordCode(r.Context(), moi.Ma)
	vietJSON(w, http.StatusCreated, bangRaNgoai(moi))
}

// GoBangNganSach soft deletes one sheet — §6's `🗑 Gỡ`. DELETE /api/v1/budget-sheets/{id}
//
// 204 AND NO BODY. The sheet and its whole tree are still in the table carrying `deleted_at`,
// `deleted_by` and `delete_reason`, but there is nothing the caller can do with them, and returning
// the sheet would invite a client to display a year it has just taken off the screen.
func (h *Handler) GoBangNganSach(w http.ResponseWriter, r *http.Request) {
	var vao goVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	if err := h.d.GhiNganSach.GoBang(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiNganSach(w, r, "gỡ bảng", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SuaBangNganSach edits one sheet's title, display unit and cut-off date.
// PATCH /api/v1/budget-sheets/{id}
//
// Changing `unit` changes only how the figures are DISPLAYED; every stored figure is đồng and none
// is converted (domain.DonViTinh).
func (h *Handler) SuaBangNganSach(w http.ResponseWriter, r *http.Request) {
	var vao suaBangVao
	if !docThan(w, r, &vao) {
		return
	}
	if vao.Year != nil || vao.Kind != nil || vao.Code != nil || vao.Columns != nil {
		h.traLoiLoiNganSach(w, r, "sửa bảng", domain.ErrTruongBangKhongSua)
		return
	}

	yc := app.YeuCauSuaBang{TieuDe: vao.Title, DonViTinh: vao.Unit}
	if vao.CumulativeTo != nil {
		// ngayVao maps "" to the zero time, which is exactly "clear the cut-off date".
		luyKe, ok := ngayVao(*vao.CumulativeTo)
		if !ok {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"`cumulative_to` phải theo dạng YYYY-MM-DD, ví dụ 2026-08-25, hoặc chuỗi rỗng để bỏ mốc.", "")
			return
		}
		yc.LuyKeDen = &luyKe
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	sau, err := h.d.GhiNganSach.SuaBang(r.Context(), r.PathValue("id"), yc, nguoi)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "sửa bảng", err)
		return
	}
	vietJSON(w, http.StatusOK, bangRaNgoai(sau))
}

// ThemKhoanMucNganSach adds one line. POST /api/v1/budget-lines
func (h *Handler) ThemKhoanMucNganSach(w http.ResponseWriter, r *http.Request) {
	var vao themDongVao
	if !docThan(w, r, &vao) {
		return
	}
	// BEFORE ANYTHING ELSE, so the caller learns that these three are not theirs to set rather than
	// watching the fields disappear.
	if err := khongDuocDat(vao.Method != nil, vao.Level != nil, vao.IsHeadline != nil); err != nil {
		h.traLoiLoiNganSach(w, r, "thêm khoản mục", err)
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	moi, err := h.d.GhiNganSach.ThemKhoanMuc(r.Context(), app.YeuCauThemKhoanMuc{
		BangID: vao.SheetID, ChaID: vao.ParentID, TT: vao.No, Ten: vao.Name, ThuTu: vao.Order,
	}, nguoi)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "thêm khoản mục", err)
		return
	}

	idem.RecordCode(r.Context(), moi.ID)
	vietJSON(w, http.StatusCreated, dongRaMot(moi))
}

// SuaKhoanMucNganSach edits one line's text and its figures. PATCH /api/v1/budget-lines/{id}
func (h *Handler) SuaKhoanMucNganSach(w http.ResponseWriter, r *http.Request) {
	var vao suaDongVao
	if !docThan(w, r, &vao) {
		return
	}
	if err := khongDuocDat(vao.Method != nil, vao.Level != nil, vao.IsHeadline != nil); err != nil {
		h.traLoiLoiNganSach(w, r, "sửa khoản mục", err)
		return
	}
	if vao.SheetID != nil || vao.ParentID != nil {
		h.traLoiLoiNganSach(w, r, "sửa khoản mục", domain.ErrChaKhongCungBang)
		return
	}

	gia := map[string]*domain.Dong{}
	for cotID, v := range vao.Values {
		if v == nil {
			gia[cotID] = nil
			continue
		}
		g := domain.Dong(*v)
		gia[cotID] = &g
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	sau, err := h.d.GhiNganSach.SuaKhoanMuc(r.Context(), r.PathValue("id"), app.YeuCauSuaKhoanMuc{
		TT: vao.No, Ten: vao.Name, ThuTu: vao.Order, GiaTri: gia,
	}, nguoi)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "sửa khoản mục", err)
		return
	}
	vietJSON(w, http.StatusOK, dongRaMot(sau))
}

// GoKhoanMucNganSach soft deletes one line. DELETE /api/v1/budget-lines/{id}
func (h *Handler) GoKhoanMucNganSach(w http.ResponseWriter, r *http.Request) {
	var vao goVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	if err := h.d.GhiNganSach.GoKhoanMuc(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiNganSach(w, r, "gỡ khoản mục", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DatDongTongNganSach marks one line as the sheet's total row — the `☆`.
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
func (h *Handler) DatDongTongNganSach(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	sau, err := h.d.GhiNganSach.DatDongTong(r.Context(), r.PathValue("id"), nguoi)
	if err != nil {
		h.traLoiLoiNganSach(w, r, "đặt dòng tổng", err)
		return
	}
	vietJSON(w, http.StatusOK, dongRaMot(sau))
}

// dongRaMot is one line WITHOUT its figures.
//
// `values` IS NIL ON THIS SHAPE ON PURPOSE. A line's displayed figures depend on the whole tree — a
// parent's cell is the sum of its direct children — and a write route holds one row, not the sheet.
// Sending a `values` map built from the row alone would give a parent's cells the LOCKED figures
// underneath it rather than the ones on the screen, which is a wrong number arriving at the moment
// the client is most likely to trust it. The client refetches the sheet; the route confirms the act.
func dongRaMot(k domain.KhoanMucNganSach) dongRa {
	return dongRa{
		ID: k.ID, ParentID: k.ChaID, No: k.TT, Name: k.Ten, Order: k.ThuTu,
		Method: string(k.CachTinh), Level: k.Cap, IsHeadline: k.LaDongTong,
	}
}

// khongDuocDat turns "the client named a field that is not theirs" into the domain's own sentence.
func khongDuocDat(coMethod, coLevel, coHeadline bool) error {
	switch {
	case coMethod:
		return domain.ErrCachTinhDoTuClient
	case coLevel:
		return domain.ErrCapDoTuClient
	case coHeadline:
		// REFUSED RATHER THAN IGNORED. The flag is reachable from no layer above the store except
		// DatDongTong's own statement pair, so ignoring it would be safe and would leave the client
		// believing it had just created the commune's total row — a second marked row as far as the
		// client is concerned, and the summary card would then disagree with what it thinks it did.
		return domain.ErrDongTongQuaTuyenRieng
	}
	return nil
}

// --- errors ----------------------------------------------------------------------------------------------

// traLoiLoiNganSach maps one use-case failure onto a status and a sentence.
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
func (h *Handler) traLoiLoiNganSach(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, fistore.ErrKhongThayBangNganSach):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy bảng ngân sách này.", "")
	case errors.Is(err, domain.ErrKhongThayKhoanMuc):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy khoản mục này.", "")
	case errors.Is(err, fistore.ErrBangDaTonTai):
		httpx.WriteError(w, http.StatusConflict, "sheet_exists", err.Error(), "")
	case errors.Is(err, domain.ErrKhoanMucChaKhongGoThang),
		errors.Is(err, domain.ErrConGiuKhoanMucCon),
		errors.Is(err, domain.ErrNhieuDongTong):
		httpx.WriteError(w, http.StatusConflict, "budget_tree", err.Error(), "")
	case errors.Is(err, fistore.ErrQuaNhieuKhoanMuc):
		// 500 AND NOT 409, because this is not something the caller did: the sheet in the database is
		// past a bound this service refuses to truncate, and the person in front of the screen has no
		// act available. It names itself in the log, which is where the operator will look.
		h.d.Log.Error("ngân sách: bảng vượt trần khoản mục",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	case laLoiDauVaoNganSach(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3). The commune is logged because it is the only thing an operator can act on;
		// the figures and the line names are NOT, because a log line travels into centralised logging
		// across every commune at once.
		h.d.Log.Error("ngân sách thu chi: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// laLoiDauVaoNganSach reports whether this is a refusal of what the client sent, as opposed to a
// failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400. A default of "anything I do not recognise is the
// client's fault" turns a database outage into a 400, and a client that believes its input is wrong
// retries with different input forever while nobody is told the server is broken.
func laLoiDauVaoNganSach(err error) bool {
	for _, mot := range []error{
		domain.ErrThieuBang, domain.ErrLoaiBangSai, domain.ErrNamNgoaiLich,
		domain.ErrKieuCotSai, domain.ErrVaiTroSai, domain.ErrVaiTroSaiLoaiBang,
		domain.ErrVaiTroTrenCotPhanTram, domain.ErrThieuCongThuc, domain.ErrThuaCongThuc,
		domain.ErrVaiTroTrungTrongBang,
		domain.ErrThieuTieuDe, domain.ErrTieuDeQuaDai,
		domain.ErrThieuDonViTinh, domain.ErrDonViTinhSai, domain.ErrTruongBangKhongSua,
		domain.ErrThieuTenCot, domain.ErrTenCotQuaDai, domain.ErrCongThucQuaDai,
		domain.ErrKhongCoCotNao, domain.ErrQuaNhieuCot,
		domain.ErrThieuTenKhoanMuc, domain.ErrTenKhoanMucQuaDai, domain.ErrTTQuaDai,
		domain.ErrThuTuNgoaiKhoangNS, domain.ErrGiaTriQuaLon,
		domain.ErrThieuLyDoXoaNganSach, domain.ErrLyDoXoaNganSachQuaDai,
		domain.ErrCachTinhDoTuClient, domain.ErrCapDoTuClient, domain.ErrDongTongQuaTuyenRieng,
		domain.ErrChaKhongCungBang, domain.ErrChaLaChinhNo, domain.ErrChaTaoVongLap,
		domain.ErrCotKhongThuocBang, domain.ErrCotKhongPhaiCotSo,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}
