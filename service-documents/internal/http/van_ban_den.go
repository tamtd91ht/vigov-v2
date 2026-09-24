package http

// The routes of SỔ VĂN BẢN ĐẾN (docs/ui-ux/05-van-ban-don-thu.md §3, §5, §7).
//
// FIVE ROUTES, THREE PERMISSIONS, AND THE SPLIT IS THE SPECIFICATION'S OWN (§7 rule 4):
// `document.create` books and corrects, `document.read` lists, `document.route` sends a document to
// a department. Those three keys are seeded at service-identity/migrations/0001_init.sql:287-289 and
// listed in the permission matrix at docs/ui-ux/14-cau-hinh.md:122-124 — no key was invented here
// (rule 5, invariant 3c).
//
// ⚠ FINDING, NOT A DECISION: THERE IS NO FOURTH KEY, AND TWO ROUTES ARE GUARDED BY THE NEAREST ONE.
// `PATCH` and `DELETE` carry `document.create`, because "Vào sổ văn bản" is the only write key the
// `quyen` table has for this subsystem. A commune may well want the clerk who books documents to be
// unable to REMOVE one — that is a different right, and it would need `document.update` /
// `document.delete` seeded by a migration. Inventing either here would produce a route that answers
// 403 to EVERY account forever while every test stayed green, because a fake checker grants any
// string (rule 5, invariant 3c). It is raised for open question #27 instead.
//
// THE URL RESOURCE IS `incoming-documents`, AND IT WAS NOT TRANSLATED ON THE SPOT: it is the noun
// kb/00-foundation/ubiquitous-language.md:142 already settled for `van_ban_den` (ADR 0011).
//
// ⚠ `routings` IS A NOUN THIS SESSION CHOSE. The mapping table has no row for "chuyển xử lý", and
// ADR 0011 says to ask rather than translate on the spot. `chuyen-xu-ly` (the specification's own
// path, §6) is Vietnamese and a verb, which rest_api_guard refuses; `routings` is the nominalised
// form of the act the permission itself is named after ("Phân luồng văn bản"). It is a finding in
// the hand-over, not a decision hidden in a route.

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// vanBanDenRa is one incoming document as it leaves the API.
//
// # THERE IS NO `overdue` FIELD, AND THAT IS THE POINT RATHER THAN AN OMISSION
//
// The first draft of this type carried one, computed on every render. `citizen_commitment_guard`
// blocked it, and the guard is right for a reason worth writing down: a boolean beside the deadline
// is a SECOND REPRESENTATION of one fact on the wire, and the two can disagree — a response cached
// for a minute carries `overdue: false` next to a deadline that has since passed, and the screen
// believes the boolean. `due_at` is the fact; being late is a comparison against it, and §3.1's own
// cell ("d/M/yyyy (trễ N ngày)") renders both halves from that single value.
//
// The server's own derivation exists and is the only one there will ever be: domain.VanBanDen.QuaHan
// (rule 10, invariant 3). It is what a report will call; it is not a column and not a field.
//
// `DueAt` IS RFC 3339 AND NOT A DATE. The commitment is counted in WORKING HOURS (ADR 0007); a date
// on the wire would round it to a day and widen it silently, and a client could never render
// "trễ 3 giờ".
//
// ⚠ `Summary` AND `IssuingBody` ARE FREE TEXT AND MAY NAME A CITIZEN (rule 3). They are returned
// unmasked to the commune's own staff, which is what the register is for — the §3.1 table renders
// `TRÍCH YẾU` in full. What this file guarantees is narrower and is stated rather than implied:
// neither value is ever logged, and neither appears in an error message.
type vanBanDenRa struct {
	ID string `json:"id"`

	// Number and Year are the ISSUED NUMBER. Output only: a request carrying either is refused with
	// 400, because the number is the register's to give.
	Number int `json:"number"`
	Year   int `json:"year"`

	ReceivedDate string `json:"received_date"`           // YYYY-MM-DD, the day it arrived
	ReferenceNo  string `json:"reference_no,omitempty"`  // "1742-CV/BTCTU"
	DocumentDate string `json:"document_date,omitempty"` // YYYY-MM-DD, the day it was signed
	IssuingBody  string `json:"issuing_body"`
	DocumentType string `json:"document_type"` // the catalogue CODE
	Summary      string `json:"summary"`
	Urgency      string `json:"urgency,omitempty"`

	HoldingUnit string `json:"holding_unit,omitempty"` // identity's `bo_phan.id`
	Assignee    string `json:"assignee,omitempty"`     // a STAFF BUSINESS CODE

	DueAt string `json:"due_at"` // RFC 3339 — the commitment, fixed once at booking

	Status    string `json:"status"` // `moi-vao-so` … — Vietnamese without diacritics (ADR 0011)
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func ngayRa(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.DateOnly)
}

func lucRa(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func vanBanDenRaNgoai(v domain.VanBanDen) vanBanDenRa {
	return vanBanDenRa{
		ID:           v.ID,
		Number:       v.SoVaoSo,
		Year:         v.Nam,
		ReceivedDate: ngayRa(v.NgayDen),
		ReferenceNo:  v.SoKyHieu,
		DocumentDate: ngayRa(v.NgayVanBan),
		IssuingBody:  v.CoQuanBanHanh,
		DocumentType: v.LoaiVanBan,
		Summary:      v.TrichYeu,
		Urgency:      string(v.DoKhan),
		HoldingUnit:  v.BoPhanDangGiu,
		Assignee:     v.CanBoXuLyMa,
		DueAt:        lucRa(v.HanXuLyXong),
		Status:       string(v.TrangThai),
		CreatedBy:    v.NguoiTaoMa,
		CreatedAt:    lucRa(v.TaoLuc),
		UpdatedAt:    lucRa(v.CapNhatLuc),
	}
}

// `omitempty` ON EVERY OPTIONAL INPUT FIELD, AND IT IS NOT COSMETIC. tools/apidoc marks a field
// REQUIRED in kb/20-contracts/openapi.json unless it carries `omitempty`, so without it these bodies
// would tell every generated client that `number` MUST be sent — on routes that answer 400 to
// exactly that.

// themVanBanDenVao is the body of POST /api/v1/incoming-documents.
//
// `Number`, `Status` AND `DueAt` ARE HERE ONLY SO THEY CAN BE REFUSED. None of them is written
// anywhere: the number comes from the counter under a row lock, the state is a literal in the
// INSERT, and the deadline is computed by identity from this commune's SLA. Declaring them and
// answering 400 is the difference between a clerk learning that these are not theirs to set and a
// clerk watching a value they typed disappear — and in the number's case, believing they had just
// reissued số 7.
//
// THERE IS NO `created_by` FIELD AND THERE MUST NEVER BE ONE. Who booked the document is the acting
// principal from the session; a field would be a client naming somebody else as the author of an
// archival record (rule 1, forbidden #2, applied to a person instead of a commune).
type themVanBanDenVao struct {
	ReceivedDate string `json:"received_date"` // YYYY-MM-DD
	ReferenceNo  string `json:"reference_no,omitempty"`
	DocumentDate string `json:"document_date,omitempty"`
	IssuingBody  string `json:"issuing_body"`
	DocumentType string `json:"document_type"`
	Summary      string `json:"summary"`
	Urgency      string `json:"urgency,omitempty"`

	Number *int    `json:"number,omitempty"`
	Status *string `json:"status,omitempty"`
	DueAt  *string `json:"due_at,omitempty"`
}

// suaVanBanDenVao is the body of PATCH /api/v1/incoming-documents/{id}.
//
// EVERY EDITABLE FIELD IS A POINTER, and that is the whole reason this is a PATCH and not a PUT:
// `reference_no`, `document_date` and `urgency` are optional, and their empty value is MEANINGFUL —
// "this document carries no number of its own" is a statement, not an absence of one. A body of
// plain values cannot tell "not mentioned" from "cleared", so a dialog editing only the summary
// would wipe the issuing body's own number off an archival record.
//
// THE HOLDING DEPARTMENT AND THE ASSIGNEE ARE NOT EDITABLE HERE, and that is not an oversight:
// moving a document between departments is an act with a reason and a timeline entry, which is what
// the routing route is for. An edit that could move a file would move it with no record of who sent
// it where.
type suaVanBanDenVao struct {
	ReceivedDate *string `json:"received_date,omitempty"`
	ReferenceNo  *string `json:"reference_no,omitempty"`
	DocumentDate *string `json:"document_date,omitempty"`
	IssuingBody  *string `json:"issuing_body,omitempty"`
	DocumentType *string `json:"document_type,omitempty"`
	Summary      *string `json:"summary,omitempty"`
	Urgency      *string `json:"urgency,omitempty"`

	Number *int    `json:"number,omitempty"`
	Status *string `json:"status,omitempty"`
	DueAt  *string `json:"due_at,omitempty"`
}

// goVanBanVao is the body of both DELETE routes.
//
// A DELETE WITH A BODY, and the alternative was worse. Rule 7, invariant 1 names three columns —
// `deleted_at`, `deleted_by`, `delete_reason` — so the reason is not optional, and the only other
// place to put it is the query string, where free text about a government record would land in every
// access log and proxy cache.
type goVanBanVao struct {
	Reason string `json:"reason"`
}

// chuyenVanBanVao is the body of POST /api/v1/incoming-documents/{id}/routings — the "Chuyển cho bộ
// phận khác" block of §3.5.
//
// `Reason` IS MANDATORY AND THAT IS THIS SESSION'S DECISION, not the specification's: §3.5 draws the
// field with a placeholder and does not mark it required. It is required because the timeline entry
// can never be edited afterwards (rule 7, forbidden #5) — a routing with no reason is an instruction
// nobody can account for, and the person who gave it will have moved on by the time anybody asks.
//
// `Assignee` IS OPTIONAL: "— Để bộ phận tự phân công —" is a real answer on that screen.
type chuyenVanBanVao struct {
	ToUnit   string `json:"to_unit"`
	Assignee string `json:"assignee,omitempty"`
	Reason   string `json:"reason"`
}

// ngayVao parses a YYYY-MM-DD date.
//
// `time.DateOnly` AND NOT RFC 3339: these are DATES — the day a document arrived, the day it was
// signed — and a client sending an instant would be choosing a timezone for a fact that has none.
// Both columns are `DATE`.
//
// THE ZERO VALUE IS RETURNED FOR AN EMPTY STRING rather than an error, so that PATCH can tell
// "absent" from "malformed" using its own pointer, and so that clearing an optional date is
// expressible. domain.KiemNgayDen refuses the zero where the field is required.
func ngayVao(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, true
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ThemVanBanDen books one incoming document. POST /api/v1/incoming-documents
func (h *Handler) ThemVanBanDen(w http.ResponseWriter, r *http.Request) {
	var vao themVanBanDenVao
	if !docThan(w, r, &vao) {
		return
	}
	// BEFORE ANYTHING ELSE. A body naming the number, the state or the deadline is refused outright,
	// so the caller learns which facts are not theirs to set rather than watching them disappear.
	if err := khongDoTuClient(vao.Number, vao.Status, vao.DueAt); err != nil {
		h.traLoiLoiVanBan(w, r, "vào sổ", err)
		return
	}

	ngayDen, ok := ngayVao(vao.ReceivedDate)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`received_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-22.", "")
		return
	}
	ngayVanBan, ok := ngayVao(vao.DocumentDate)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"`document_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-18.", "")
		return
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	moi, err := h.d.GhiVanBanDen.Them(r.Context(), app.YeuCauVaoSoVanBanDen{
		NgayDen:       ngayDen,
		SoKyHieu:      vao.ReferenceNo,
		NgayVanBan:    ngayVanBan,
		CoQuanBanHanh: vao.IssuingBody,
		LoaiVanBan:    vao.DocumentType,
		TrichYeu:      vao.Summary,
		DoKhan:        domain.DoKhan(vao.Urgency),
	}, nguoi)
	if err != nil {
		h.traLoiLoiVanBan(w, r, "vào sổ", err)
		return
	}

	// What a retry carrying the same Idempotency-Key is told about. THE ID AND NOT THE BODY: the
	// body would go into Redis, which is a cache and not a record store.
	idem.RecordCode(r.Context(), moi.ID)
	vietJSON(w, http.StatusCreated, vanBanDenRaNgoai(moi))
}

// SuaVanBanDen corrects one booked document. PATCH /api/v1/incoming-documents/{id}
func (h *Handler) SuaVanBanDen(w http.ResponseWriter, r *http.Request) {
	var vao suaVanBanDenVao
	if !docThan(w, r, &vao) {
		return
	}
	if err := khongDoTuClient(vao.Number, vao.Status, vao.DueAt); err != nil {
		h.traLoiLoiVanBan(w, r, "sửa", err)
		return
	}

	yc := app.YeuCauSuaVanBanDen{
		SoKyHieu:      vao.ReferenceNo,
		CoQuanBanHanh: vao.IssuingBody,
		LoaiVanBan:    vao.DocumentType,
		TrichYeu:      vao.Summary,
	}
	if vao.ReceivedDate != nil {
		t, ok := ngayVao(*vao.ReceivedDate)
		if !ok || t.IsZero() {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"`received_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-22.", "")
			return
		}
		yc.NgayDen = &t
	}
	if vao.DocumentDate != nil {
		// THE EMPTY STRING IS ACCEPTED HERE and clears the date: `document_date` is optional, so
		// "this document carries no date of its own" has to be expressible.
		t, ok := ngayVao(*vao.DocumentDate)
		if !ok {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"`document_date` phải theo dạng YYYY-MM-DD, ví dụ 2026-09-18.", "")
			return
		}
		yc.NgayVanBan = &t
	}
	if vao.Urgency != nil {
		d := domain.DoKhan(*vao.Urgency)
		yc.DoKhan = &d
	}

	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}

	sau, err := h.d.GhiVanBanDen.Sua(r.Context(), r.PathValue("id"), yc, nguoi)
	if err != nil {
		h.traLoiLoiVanBan(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, vanBanDenRaNgoai(sau))
}

// GoVanBanDen soft deletes one entry. DELETE /api/v1/incoming-documents/{id}
//
// 204 AND NO BODY. The row is still there — it carries `deleted_at`, `deleted_by` and
// `delete_reason`, and its NUMBER is still taken — but there is nothing the caller can do with it,
// and returning it would invite a client to display a document it has just taken off the screen.
func (h *Handler) GoVanBanDen(w http.ResponseWriter, r *http.Request) {
	var vao goVanBanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	if err := h.d.GhiVanBanDen.Go(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiVanBan(w, r, "gỡ", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ChuyenVanBanDen routes one document to a department.
// POST /api/v1/incoming-documents/{id}/routings
//
// IT RETURNS THE DOCUMENT (200), unlike the soft delete above, because the caller's next act is on
// the same row: the client needs the state it has landed in and the department now holding it to
// know which buttons to draw.
func (h *Handler) ChuyenVanBanDen(w http.ResponseWriter, r *http.Request) {
	var vao chuyenVanBanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuThe(w, r)
		return
	}
	sau, err := h.d.GhiVanBanDen.Chuyen(r.Context(), r.PathValue("id"), app.YeuCauChuyenVanBan{
		DenBoPhan:   vao.ToUnit,
		CanBoXuLyMa: vao.Assignee,
		LyDo:        vao.Reason,
	}, nguoi)
	if err != nil {
		h.traLoiLoiVanBan(w, r, "chuyển", err)
		return
	}
	vietJSON(w, http.StatusOK, vanBanDenRaNgoai(sau))
}

// DanhSachVanBanDen serves one page of the register. GET /api/v1/incoming-documents
func (h *Handler) DanhSachVanBanDen(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to
	// $1 from the context on every statement (rule 1, invariant 5). page.Parse reads only
	// limit/cursor/sort/order, and a cursor carries no commune by construction. NOTHING HERE READS
	// `tenant_id` FROM THE QUERY STRING — a client naming its own commune is a client granting
	// itself access (rule 1, forbidden #2).
	thamSo := r.URL.Query()

	// Parsed BEFORE the store is touched: a rejected page request must run no statement at all.
	yc, err := page.Parse(thamSo, docstore.SapXepVanBanDen)
	if err != nil {
		// page.HTTPError owns the mapping so every service answers a bad cursor the same way. It
		// never echoes what the client sent: a cursor is opaque, and a rejected sort key is often a
		// probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	loc, err := locVanBanDenTuQuery(thamSo)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}

	kq, err := h.d.VanBanDen.DanhSach(ctx, loc, yc)
	if err != nil {
		// The wrapped error carries the store failure. It does NOT carry a summary or an issuing
		// body, and it never reaches the client (rule 3, forbidden #3).
		h.d.Log.Error("danh sách văn bản đến: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// page.Result[T] DIRECTLY — tools/apidoc understands it, so there is no second three-field
	// struct copying it and no way for the two to drift.
	//
	// make(..., 0, ...) and not a nil slice: `items` must marshal as [] on a commune whose register
	// is empty, never as null. A newly onboarded commune has exactly that, and a client that has to
	// handle both shapes handles one of them wrong.
	ra := page.Result[vanBanDenRa]{
		Items:      make([]vanBanDenRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, v := range kq.Items {
		ra.Items = append(ra.Items, vanBanDenRaNgoai(v))
	}
	vietJSON(w, http.StatusOK, ra)
}

// ChiTietVanBanDen serves one document for the detail drawer. GET /api/v1/incoming-documents/{id}
//
// THE 404 GOES THROUGH traLoiLoiVanBan, the same sentence the write routes give for a missing
// document, so "removed", "another commune's" and "never existed" are byte-identical.
func (h *Handler) ChiTietVanBanDen(w http.ResponseWriter, r *http.Request) {
	v, err := h.d.ChiTietVanBanDen.ChiTiet(r.Context(), r.PathValue("id"))
	if err != nil {
		h.traLoiLoiVanBan(w, r, "đọc chi tiết", err)
		return
	}
	vietJSON(w, http.StatusOK, vanBanDenRaNgoai(v))
}

// lichSuChuyenRa is one line of the routing timeline as it leaves the API — every column
// `lich_su_chuyen_van_ban` stores except `tenant_id`, which the caller already is.
//
// FIELD NAMES MATCH chuyenVanBanVao (`to_unit`, `assignee`, `reason`), so the line a routing writes
// reads back under the names it was sent with. Units are identity's `bo_phan.id` and people are
// STAFF BUSINESS CODES; no display name is resolved here — the table holds none.
//
// ⚠ `Reason` IS FREE TEXT (a leader's instruction) and may name a citizen (rule 3). Returned to the
// commune's own staff unmasked, as the drawer renders it; never logged, never in an error.
type lichSuChuyenRa struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	RoutedAt   string `json:"routed_at"`           // RFC 3339 — the instant of the act
	RoutedBy   string `json:"routed_by"`           // staff business code of who routed it
	Status     string `json:"status"`              // the state the document moved INTO by this act
	FromUnit   string `json:"from_unit,omitempty"` // empty on the first routing: nobody held it
	ToUnit     string `json:"to_unit"`
	Assignee   string `json:"assignee,omitempty"` // empty = "Để bộ phận tự phân công"
	Reason     string `json:"reason"`
	CreatedAt  string `json:"created_at"` // RFC 3339 — when the row was written
}

// danhSachLichSuChuyenRa wraps the timeline in an object, like danhSachLoaiVanBanRa and for the same
// reason. NO next_cursor AND NO has_more: the route returns the whole timeline or refuses.
type danhSachLichSuChuyenRa struct {
	Items []lichSuChuyenRa `json:"items"`
}

func lichSuChuyenRaNgoai(c domain.ChuyenVanBan) lichSuChuyenRa {
	return lichSuChuyenRa{
		ID:         c.ID,
		DocumentID: c.VanBanDenID,
		RoutedAt:   lucRa(c.ThoiDiem),
		RoutedBy:   c.NguoiMa,
		Status:     string(c.TrangThaiTaiThoiDiem),
		FromUnit:   c.TuBoPhan,
		ToUnit:     c.DenBoPhan,
		Assignee:   c.CanBoXuLyMa,
		Reason:     c.NoiDung,
		CreatedAt:  lucRa(c.TaoLuc),
	}
}

// LichSuChuyenVanBanDen serves one document's routing timeline, oldest first.
// GET /api/v1/incoming-documents/{id}/routings
//
// THE ORDER IS THE STORE'S (ORDER BY thoi_diem, id) and is kept as-is: sorting again here would be
// a second copy of that decision.
func (h *Handler) LichSuChuyenVanBanDen(w http.ResponseWriter, r *http.Request) {
	ds, err := h.d.ChiTietVanBanDen.LichSuChuyen(r.Context(), r.PathValue("id"))
	if err != nil {
		h.traLoiLoiVanBan(w, r, "đọc lịch sử chuyển", err)
		return
	}
	// make(..., 0, ...): a document never routed has `items: []`, never `null`.
	ra := danhSachLichSuChuyenRa{Items: make([]lichSuChuyenRa, 0, len(ds))}
	for _, c := range ds {
		ra.Items = append(ra.Items, lichSuChuyenRaNgoai(c))
	}
	vietJSON(w, http.StatusOK, ra)
}

// locVanBanDenTuQuery validates the filters. EVERY VALUE BECOMES A BOUND PARAMETER in the store;
// nothing here builds SQL.
//
// AN UNKNOWN `status` IS REFUSED RATHER THAN IGNORED. A filter silently dropped returns the WHOLE
// register to a screen that asked for one slice of it, and the screen has no way to know — which on
// this surface means a clerk believing they are looking at every document of one kind.
func locVanBanDenTuQuery(q url.Values) (docstore.LocVanBanDen, error) {
	var loc docstore.LocVanBanDen
	if s := q.Get("year"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 2000 || n > 2200 {
			return loc, errNamKhongHopLe
		}
		loc.Nam = n
	}
	if s := q.Get("status"); s != "" {
		switch domain.TrangThaiVanBanDen(s) {
		case domain.VanBanMoiVaoSo, domain.VanBanDaPhanCong, domain.VanBanDangXuLy,
			domain.VanBanDaGiaiQuyet, domain.VanBanChuyenCapTren, domain.VanBanLuuKhongThuLy:
			loc.TrangThai = s
		default:
			return loc, errTrangThaiKhongHopLe
		}
	}
	loc.LoaiVanBan = q.Get("document_type")
	loc.BoPhan = q.Get("holding_unit")
	loc.Tim = q.Get("q")
	// RUNES, NOT BYTES. `len` counts UTF-8 bytes, and a Vietnamese letter with a tone mark is 2-3
	// bytes — so a byte cap of 200 refused a search of ~70 characters while the limit is meant in
	// characters, the unit domain.ChuanHoaChuoi counts in.
	if utf8.RuneCountInString(loc.Tim) > 200 {
		return loc, errTimQuaDai
	}
	return loc, nil
}

// khongDoTuClient refuses the three facts a client may never state.
//
// ONE FUNCTION FOR BOTH WRITE ROUTES, because two copies would drift and the copy that drifts is the
// one that stops refusing `number` — which is the field whose acceptance would let a client reissue
// a number that is already on a sealed document.
func khongDoTuClient(so *int, trangThai, han *string) error {
	if so != nil {
		return domain.ErrSoDoTuClient
	}
	if trangThai != nil {
		return domain.ErrTrangThaiDoTuClient
	}
	if han != nil {
		return domain.ErrHanDoTuClient
	}
	return nil
}

// traLoiLoiVanBan maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR ALL THE WRITE ROUTES OF BOTH REGISTERS, because copies of this mapping would
// drift and the copy that drifts is the one answering 500 where it meant 409 — which reads to an
// operator as a broken server rather than as a rule doing its job.
//
// WHY 409 AND NOT 403 FOR A STATE REFUSAL: the caller HOLDS the permission and is allowed to perform
// the operation. What is refused is this operation on THIS document, because of the state it is in.
// 403 would send a clerk to the Phân quyền screen to be granted a right they already have.
//
// THE DOMAIN'S OWN SENTENCE IS RETURNED ON EVERY REFUSAL, deliberately. It names the field and the
// rule, holds no personal data and no internal detail, and a second sentence written here would
// drift from it. What must NEVER reach a client is the PostgreSQL exception underneath — its text is
// English, it names a constraint, and it says nothing a clerk in a commune can act on.
func (h *Handler) traLoiLoiVanBan(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, docstore.ErrKhongThayVanBanDen):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy văn bản đến này.", "")
	case errors.Is(err, docstore.ErrKhongThayVanBanDi):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy văn bản đi này.", "")
	case errors.Is(err, docstore.ErrLoaiVanBanKhongDung),
		errors.Is(err, docstore.ErrLoaiVanBanDiKhongDung):
		// 409 AND NOT 400: the value is well formed and the caller is allowed to send it. What is
		// wrong is this code against the state of the commune's catalogue — the type may have been
		// retired between the form loading and the clerk pressing save.
		httpx.WriteError(w, http.StatusConflict, "document_type_unknown",
			"Loại văn bản này không còn trong danh mục đang dùng của xã. Hãy chọn lại loại văn bản.",
			"")
	case errors.Is(err, app.ErrChuaAnDinhDuocHan):
		// 409, AND THE SENTENCE NAMES THE SCREEN THAT FIXES IT. This is the ordinary answer in every
		// commune today: `sla` is empty everywhere and the onboarding step that fills it does not
		// exist in this repository. A fallback deadline is refused outright (rule 10, forbidden #3),
		// so the honest thing to do is say which configuration is missing.
		httpx.WriteError(w, http.StatusConflict, "sla_chua_cau_hinh",
			"Xã chưa cấu hình thời hạn xử lý cho văn bản đến, nên chưa vào sổ được. "+
				"Vào Cấu hình → Thời hạn xử lý để đặt số giờ xử lý, rồi vào sổ lại.", "")
	case errors.Is(err, docstore.ErrDaySoDayTran):
		httpx.WriteError(w, http.StatusConflict, "so_van_ban_day",
			"Dãy số của sổ năm nay đã đạt mức tối đa. Hãy báo quản trị hệ thống.", "")
	case errors.Is(err, domain.ErrVanBanDaKetThuc):
		httpx.WriteError(w, http.StatusConflict, "document_state", err.Error(), "")
	case laLoiDauVaoVanBan(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("văn bản: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// laLoiDauVaoVanBan reports whether this is a refusal of what the client sent, as opposed to a
// failure.
//
// LISTED EXPLICITLY RATHER THAN DEFAULTING TO 400. A default of "anything I do not recognise is the
// client's fault" turns a database outage into a 400, and a client that believes its input is wrong
// retries with different input forever while nobody is told the server is broken.
func laLoiDauVaoVanBan(err error) bool {
	for _, mot := range []error{
		domain.ErrThieuNgayDen, domain.ErrNgayDenTuongLai, domain.ErrNgayDenQuaXa,
		domain.ErrNgayVanBanSauDen, domain.ErrThieuNgayVanBan, domain.ErrNgayVanBanTuongLai,
		domain.ErrThieuCoQuanBanHanh, domain.ErrThieuNoiNhan,
		domain.ErrThieuLoaiVanBan, domain.ErrThieuTrichYeu, domain.ErrDoKhanKhongHopLe,
		domain.ErrThieuBoPhanNhan, domain.ErrThieuLyDoChuyen, domain.ErrThieuLyDoGo,
		domain.ErrChuoiQuaDai,
		domain.ErrSoDoTuClient, domain.ErrTrangThaiDoTuClient, domain.ErrHanDoTuClient,
	} {
		if errors.Is(err, mot) {
			return true
		}
	}
	return false
}

// thieuChuThe answers a request that reached a guarded write route with no principal.
//
// A 500 AND NOT AN ANONYMOUS WRITE. These routes sit behind authz.RequirePermission, so there is
// always one; arriving here without one means the route was mounted wrong, or identity is older than
// the `ma` field and sent a principal with no business code. Rule 6 does not permit a business write
// whose trail cannot name who made it, and a fallback to the internal id would put two kinds of
// identifier into `audit_log.actor_id` one deployment window at a time, with every test green.
func (h *Handler) thieuChuThe(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến ghi sổ văn bản chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal",
		"Đã xảy ra lỗi. Vui lòng thử lại.", "")
}
