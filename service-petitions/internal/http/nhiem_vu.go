package http

// The STAFF READ surface of the task register (docs/ui-ux/02-nhiem-vu.md §3, §4, §5).
//
//	GET /api/v1/tasks        task.read
//	GET /api/v1/tasks/{ma}   task.read
//
// `task.read` IS SEEDED at service-identity/migrations/0001_init.sql:304 ("Xem nhiệm vụ") and is
// NOT invented here (rule 5, invariant 3c). `tools/check_quyen.py` scans the whole repository
// against the `quyen` table on every `make check`.
//
// `tasks` IS THE SETTLED URL NOUN — kb/00-foundation/ubiquitous-language.md:144 maps `nhiem_vu` to
// `tasks`. Not translated on the spot (ADR 0011).
//
// # THERE IS NO WRITE ROUTE IN THIS FILE, AND THAT IS THIS PASS'S BOUNDARY, NOT AN OVERSIGHT
//
// Creating, assigning, moving and extending a task are the next pass. Two of them are additionally
// blocked on something nobody has decided, and each would look entirely reasonable if written
// anyway — which is what makes writing them expensive rather than merely premature:
//
//	POST /api/v1/tasks              creating a task FIXES its deadline, and §7.1 lets a leader type
//	                                a date while rule 10, invariant 4 counts the commitment in
//	                                WORKING HOURS from this commune's `sla` row
//	                                (loai_viec = 'nhiem-vu', service-identity/migrations/
//	                                0008_sla.sql:223). identity has AdvanceWorkingHours, which
//	                                answers "lúc nào" and not "bao lâu", and ADR 0029 §118 says the
//	                                shape of the reading contract is undecided. Same blocker the
//	                                petition intake sits behind.
//	POST …/{ma}/extension/…         who may APPROVE an extension. `task.extend` exists
//	                                (0001_init.sql:303) and `nhiem_vu.lanh_dao_giao_viec_ma` names a
//	                                person on the record. Whether holding the key is enough, or
//	                                being the named leader is also required, is the SAME holding-rule
//	                                question the petition path is stopped on
//	                                (kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md
//	                                §Mâu thuẫn) — and that note says in as many words not to pick a
//	                                side before somebody decides.

import (
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// nhiemVuRa is one task as it leaves the API to a member of staff.
//
// THERE IS NO `overdue` FIELD ON THIS RESPONSE, AND ITS ABSENCE IS THE DESIGN — read this before
// adding one back, because it looks like a convenience the screen is missing.
//
// Overdue is DERIVED from a deadline compared with the current instant (rule 10, invariant 3).
// Sending it alongside the deadline would put TWO REPRESENTATIONS OF ONE FACT on the wire (rule 9),
// and they do not stay in step: the boolean is frozen at the instant the response was built, while
// the chip that shows it is rendered later and stays on screen. The client has the deadline, so it
// can compute the state — and it needs to anyway, because §4.1 shows "Trễ 87 ngày", a duration the
// boolean cannot express.
//
// The canonical derivation for anything computed INSIDE this service is
// domain.NhiemVu.TreHan / HoanThanhDungHanBanDau. A second one is a second answer.
//
// THE FIELD NAMES SAY WHAT THE DATA IS, NOT WHAT THE SCREEN CALLS IT (ADR 0017). `assigner` is the
// leader who handed the work out and who decides an extension request; the drawer labels that box
// "Lãnh đạo giao việc", and the two names are allowed to differ.
//
// THE `Theo văn bản` DOCUMENT BLOCK OF §5.4 IS NOT ON THIS RESPONSE. `nhiem_vu_van_ban` does not
// exist yet (migration 0006 states the absence), so the drawer renders the task without the three
// document lists. A client must not read an empty field as "no documents" — there is no field.
type nhiemVuRa struct {
	// Code is `ma` — the number the commune issued, `NV19`. Not `id`: the internal ULID means
	// nothing outside this service, and every screen, every printed Sổ theo dõi and every audit
	// entry names the task by this number.
	Code string `json:"code"`

	// Type and Priority are CATALOGUE CODES of this service's own tables, and Bloc is a catalogue
	// code owned by service `identity` (ADR 0024). ALL THREE ARE CODES AND NOT LABELS: the client
	// already reads `GET /api/v1/task-types` and `GET /api/v1/task-priorities` to fill its pickers,
	// so joining the label in here would be a second copy of a string the commune may re-word.
	//
	// Priority and Bloc are EMPTY when the commune has not set one — the ordinary case for a bloc
	// ("— Chưa xác định —" is a real choice, §7.1), not an error.
	Type     string `json:"type"`
	Bloc     string `json:"bloc"`
	Priority string `json:"priority"`

	Title       string `json:"title"`
	Description string `json:"description"`

	// Status is one of the seven codes of §6. The LIST is closed (open question #21, ADR 0035 §C);
	// the LABEL and the ORDER belong to the commune, which is why neither travels here.
	Status string `json:"status"`

	// Source is one of the four codes of §3, and SourceID points at the record it came from — a
	// meeting conclusion, an incoming letter, a petition. SourceID is EMPTY for `truc-tiep`, which
	// the schema enforces.
	//
	// ⚠ IT IS AN ID AND NOTHING IS JOINED THROUGH IT. Two of the three targets live in other
	// services, and the third is a petition whose contents are citizen personal data (rule 3). A
	// screen that wants the letter's subject line asks the register that owns it.
	Source   string `json:"source"`
	SourceID string `json:"source_id"`

	// Unit is `bo_phan_id` and LeadUnit is `co_quan_chu_tri_id` — identity's department ids.
	// Assignee, Assigner and Monitor are STAFF BUSINESS CODES (`CB-2026-7K3M9Q`), never internal
	// ids (rule 6, invariant 8).
	//
	// Assignee EMPTY is the `Chưa phân công` state §4.1 renders, and it is a different state from
	// an empty Unit: §11.1 makes "handed to a department with nobody named, for too long" the case
	// the system has to report.
	Unit     string `json:"unit"`
	Assignee string `json:"assignee"`
	Assigner string `json:"assigner"`
	LeadUnit string `json:"lead_unit"`
	Monitor  string `json:"monitor"`

	// DueAt is the CURRENT commitment; OriginalDueAt is the commitment as FIRST made and never
	// moves (migration 0006 refuses it with a trigger).
	//
	// ⚠ THEY ARE NOT INTERCHANGEABLE AND A REPORT THAT PICKS THE WRONG ONE IS WRONG WITHOUT
	// FAILING. "Is this late today" is measured against DueAt; §11.3's on-time ratio is measured
	// against OriginalDueAt, and §5.8 promises the citizen-facing reason on the screen itself —
	// "Hạn gốc vẫn được giữ lại để báo cáo đúng hạn không bị lùi theo". Both are on the wire so the
	// drawer can show §5.6's two boxes side by side, which is the whole point of storing two.
	//
	// NULL ON THE WIRE MEANS NO DEADLINE WAS SET — the `Hạn —` of §4.1. It does not mean zero and
	// must never be rendered as a date in year 1. The two are null together; the schema enforces it.
	DueAt         *time.Time `json:"due_at"`
	OriginalDueAt *time.Time `json:"original_due_at"`

	// CompletedAt is set if and only if Status is `hoan-thanh` (the schema enforces the
	// biconditional). It is the instant §11.3's ratio compares with OriginalDueAt.
	CompletedAt *time.Time `json:"completed_at"`

	// Progress is the "% tiến độ ghi nhận" of §5.3 — a number the officer reports, NOT one derived
	// from the status. §6's lifecycle and this percentage are two statements about the same work and
	// collapsing them would make one of them a lie on every screen.
	Progress int `json:"progress"`

	ResultSummary string `json:"result_summary"`
	Note          string `json:"note"`

	// The two manual approval marks of §5.4, whose own caption says they "không làm đổi trạng thái
	// nhiệm vụ". They are on the wire because the drawer draws them; nothing derives anything from
	// them.
	LeaderApproved       bool `json:"leader_approved"`
	SuperiorAcknowledged bool `json:"superior_acknowledged"`

	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// nhiemVuRaNgoai builds the response.
//
// NO MASKING BRANCH AND NO PERMISSION ARGUMENT, unlike the petition register's equivalent, and the
// reason is what is on the record: every person named here is a MEMBER OF STAFF, by business code.
// There is no citizen name, no telephone number and no address on a task, and the day one appears
// this function needs the branch phieuRaNgoai has.
func nhiemVuRaNgoai(n domain.NhiemVu) nhiemVuRa {
	ra := nhiemVuRa{
		Code:                 n.Ma,
		Type:                 n.Loai,
		Bloc:                 n.Khoi,
		Priority:             n.MucUuTien,
		Title:                n.TieuDe,
		Description:          n.MoTa,
		Status:               string(n.TrangThai),
		Source:               string(n.NguonGiao),
		SourceID:             n.NguonID,
		Unit:                 n.BoPhanID,
		Assignee:             n.NguoiThucHienMa,
		Assigner:             n.LanhDaoGiaoViecMa,
		LeadUnit:             n.CoQuanChuTriID,
		Monitor:              n.ChuyenVienTheoDoiMa,
		Progress:             n.TienDo,
		ResultSummary:        n.TomTatKetQua,
		Note:                 n.GhiChu,
		LeaderApproved:       n.LanhDaoPheDuyetHoanThanh,
		SuperiorAcknowledged: n.CapTrenCongNhanHoanThanh,
		CreatedBy:            n.NguoiTaoMa,
		CreatedAt:            n.TaoLuc,
	}
	// A ZERO time.Time BECOMES JSON null, HERE AND IN ONE PLACE. Marshalled straight, it would
	// travel as `0001-01-01T00:00:00Z` — a date the screen would happily render and a comparison
	// would happily call overdue.
	if !n.HanXuLy.IsZero() {
		t := n.HanXuLy
		ra.DueAt = &t
	}
	if !n.HanBanDau.IsZero() {
		t := n.HanBanDau
		ra.OriginalDueAt = &t
	}
	if !n.NgayHoanThanh.IsZero() {
		t := n.NgayHoanThanh
		ra.CompletedAt = &t
	}
	return ra
}

// QuyenDocNhiemVu guards both read routes. The key exists in `quyen` —
// service-identity/migrations/0001_init.sql:304, label "Xem nhiệm vụ" — and is not invented here.
//
// It is a constant because both handlers below need the string for their own reasoning; the ROUTE
// declarations in routes.go spell it out as a literal, because tools/apidoc refuses anything there
// that is not one.
const QuyenDocNhiemVu authz.Perm = "task.read"

// --- the register list ---------------------------------------------------------------------------

// DanhSachNhiemVu serves one page of the commune's task register. GET /api/v1/tasks
//
// # THE SCOPE FILTER RESOLVES "TÔI" FROM THE SESSION AND NEVER FROM THE REQUEST
//
// §3's `Giao cho tôi` means "the task names ME as the officer". The code it compares against comes
// from authz.Principal.Ma, put there by the staff edge from the session — never from a query
// parameter. `?assignee=CB-00123` is a DIFFERENT filter (§3's "Người thực hiện" picker), it is
// allowed, and it is not an identity claim: an account that may read the register may read all of
// it, so naming a colleague narrows the page rather than widening any right.
//
// # TWO OF §3'S FILTERS ARE REFUSED RATHER THAN IGNORED, AND THAT IS THE POINT
//
// A filter silently dropped returns a page that answers a DIFFERENT question from the one the
// screen asked, and the screen has no way to know — which on this surface means an officer
// believing they are looking at every task of one kind. Both refusals name what is missing:
//
//	scope=related  needs "bộ phận tôi đang giữ", and the staff principal carries no department
//	soon=true      needs the commune's own `sla.gio_sap_den_han`, which identity exposes no RPC for
//
// See locNhiemVuTuQuery.
func (h *Handler) DanhSachNhiemVu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to
	// $1 from the context on every statement (rule 1, invariant 5). NOTHING HERE READS `tenant_id`
	// FROM THE QUERY STRING — a client naming its own commune is a client granting itself access
	// (rule 1, forbidden #2).
	thamSo := r.URL.Query()

	// Parsed BEFORE the store is touched: a rejected page request must run no statement at all.
	yc, err := page.Parse(thamSo, petstore.SapXepNhiemVu)
	if err != nil {
		// page.HTTPError owns the mapping so every service answers a bad cursor the same way. It
		// never echoes what the client sent: a cursor is opaque, and a rejected sort key is often a
		// probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	loc, canChuThe, err := locNhiemVuTuQuery(thamSo)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}

	if canChuThe {
		// FAIL CLOSED. With no business code to compare against, the honest answers are "refuse" and
		// "return the whole register" — and the second is a screen labelled `Giao cho tôi` showing
		// every task in the commune, which nobody would report as a fault. An empty `.Ma` on a staff
		// principal is a wiring fault, so it is a 500 an operator can find, exactly as the four write
		// routes next door treat the same condition.
		principal, ok := authz.From(ctx)
		if !ok || principal.Ma == "" {
			h.d.Log.Error("bộ lọc `scope=mine` chạy mà chủ thể không có mã cán bộ — SAI CẤU HÌNH ROUTE",
				"xa", string(tenant.MustFrom(ctx)), "duong", r.URL.Path)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		loc.NguoiThucHienMa = principal.Ma
	}

	kq, err := h.d.DanhSachNhiemVu.DanhSach(ctx, loc, yc)
	if err != nil {
		// The wrapped error carries the store failure. It does NOT reach the client.
		h.d.Log.Error("danh sách nhiệm vụ: lỗi hệ thống",
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
	ra := page.Result[nhiemVuRa]{
		Items:      make([]nhiemVuRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, n := range kq.Items {
		ra.Items = append(ra.Items, nhiemVuRaNgoai(n))
	}
	vietJSON(w, http.StatusOK, ra)
}

// locNhiemVuTuQuery validates the filters. EVERY VALUE BECOMES A BOUND PARAMETER in the store;
// nothing here builds SQL.
//
// The second return value says whether the caller asked for `Giao cho tôi` — the one filter whose
// value this function may NOT fill in, because it comes from the session and this function has no
// context.
//
// AN UNKNOWN `status` OR `source` IS REFUSED RATHER THAN IGNORED, for the reason the handler above
// states at length.
//
// `type`, `bloc`, `priority`, `unit` AND `assignee` ARE NOT VALIDATED AGAINST ANY LIST,
// deliberately: the first three are per-commune catalogues (two here, one in `identity`) and the
// last two name rows of identity's org chart and staff directory, none of which is readable from a
// handler (rule 2, forbidden #2). A code that matches nothing returns an empty page, which is a true
// answer — and the database already refuses an invalid `type` or `priority` on the WRITE path
// through a real foreign key, which is where that check belongs.
func locNhiemVuTuQuery(q map[string][]string) (petstore.LocNhiemVu, bool, error) {
	lay := func(k string) string {
		if v, ok := q[k]; ok && len(v) > 0 {
			return v[0]
		}
		return ""
	}

	var (
		loc       petstore.LocNhiemVu
		canChuThe bool
	)

	if s := lay("status"); s != "" {
		if !domain.TrangThaiNhiemVu(s).HopLe() {
			return loc, false, errTrangThaiNhiemVuKhongHopLe
		}
		loc.TrangThai = s
	}
	if s := lay("source"); s != "" {
		if !domain.NguonGiao(s).HopLe() {
			return loc, false, errNguonGiaoKhongHopLe
		}
		loc.NguonGiao = s
	}

	loc.Loai = lay("type")
	loc.Khoi = lay("bloc")
	loc.MucUuTien = lay("priority")
	loc.BoPhanID = lay("unit")
	loc.NguoiThucHienMa = lay("assignee")
	loc.Tim = lay("q")
	if len(loc.Tim) > petstore.TimNhiemVuToiDa {
		return loc, false, petstore.ErrTimNhiemVuQuaDai
	}

	// `late=true` IS THE ONLY ACCEPTED SPELLING, and anything else is refused rather than read as
	// false. A checkbox whose value arrived as `1` or `yes` and was silently dropped shows an officer
	// the whole register while the box on their screen is ticked.
	if s := lay("late"); s != "" {
		if s != "true" {
			return loc, false, errLocTreHanNhiemVuKhongHopLe
		}
		loc.ChiTreHan = true
	}

	// §3's "Sắp đến hạn" checkbox. REFUSED, WITH THE REASON, RATHER THAN IMPLEMENTED AGAINST 72.
	//
	// The threshold is per commune — `sla.gio_sap_den_han`, keyed by (tenant_id, loai_viec,
	// linh_vuc) at service-identity/migrations/0008_sla.sql:178, :218 — and identity publishes no
	// RPC that returns it (ADR 0029 §118). The "mặc định 72 giờ" in §3 is the DEFAULT A COMMUNE
	// OVERRIDES, and hard-coding it here would create the second source of one number that the
	// BA/PM note calls out by name: every screen showing "sắp đến hạn" must read that one column.
	if lay("soon") != "" {
		return loc, false, errLocSapDenHanChuaCo
	}

	// §3's scope tabs. `all` is the default and needs no predicate.
	switch s := lay("scope"); s {
	case "", "all":
	case "mine":
		canChuThe = true
	case "related":
		// REFUSED, WITH THE REASON. Three of §3's four clauses are expressible here; the fourth —
		// "bộ phận tôi đang giữ" — needs the caller's own department, and the staff principal does
		// not carry one: proto/vigov/identity/v1/identity.proto returns no `bo_phan` and explains at
		// §1283-1297 why that message is deliberately narrow. Rule 2, stop condition #2.
		//
		// ANSWERING WITH THE THREE CLAUSES WOULD BE WORSE THAN REFUSING: an officer whose department
		// holds a task would not see it on the tab named for exactly that, and a short list reports
		// nothing.
		return loc, false, errPhamViChuaHoTro
	default:
		return loc, false, errPhamViKhongHopLe
	}

	// `che_do_xem` (Kanban / Danh sách / Sổ theo dõi) IS DELIBERATELY NOT READ. It chooses a LAYOUT
	// over the same rows, so it changes no predicate and no sort; accepting it here would be a
	// parameter the server ignores, which the next reader has to prove is ignored.

	return loc, canChuThe, nil
}

var (
	errTrangThaiNhiemVuKhongHopLe = errors.New(
		"`status` không phải một trong bảy trạng thái của nhiệm vụ")
	errNguonGiaoKhongHopLe = errors.New(
		"`source` không phải một trong bốn nguồn giao việc")
	errLocTreHanNhiemVuKhongHopLe = errors.New(
		"`late` chỉ nhận giá trị `true`; bỏ hẳn tham số nếu không lọc theo trễ hạn")
	errPhamViKhongHopLe = errors.New(
		"`scope` chỉ nhận `all` hoặc `mine`")
	errPhamViChuaHoTro = errors.New(
		"`scope=related` chưa dùng được: bộ lọc này cần bộ phận của chính người đăng nhập, " +
			"mà hợp đồng phiên cán bộ hiện không trả về bộ phận")
	errLocSapDenHanChuaCo = errors.New(
		"`soon` chưa dùng được: ngưỡng `sắp đến hạn` là số giờ của từng xã trong bảng `sla`, " +
			"và chưa có đường đọc số ấy — máy chủ không tự đặt một con số thay xã")
)

// --- one task ------------------------------------------------------------------------------------

// DocNhiemVu serves one task to a member of staff. GET /api/v1/tasks/{ma}
//
// NO AUDIT ENTRY, AND THE BOUNDARY IS RULE 6, INVARIANT 7: what is audited is reading FULL personal
// data and reading ACROSS communes. This is neither — a task carries no citizen personal data at
// all, and the query cannot leave the commune the request arrived in. An audit ledger that grew a
// row per screen opened would bury the disclosures it exists to make findable.
//
// NO idem.* DECLARATION: a GET changes no state.
func (h *Handler) DocNhiemVu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ma := r.PathValue("ma")

	if ma == "" {
		// An empty path segment cannot match a number and has no business reaching the database.
		h.khongTimThayNhiemVu(w)
		return
	}

	n, err := h.d.NhiemVu.TheoMa(ctx, ma)
	if err != nil {
		if errors.Is(err, petstore.ErrNhiemVuKhongTonTai) {
			h.khongTimThayNhiemVu(w)
			return
		}
		h.d.Log.Error("đọc nhiệm vụ: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	vietJSON(w, http.StatusOK, nhiemVuRaNgoai(n))
}

// khongTimThayNhiemVu is the ONE answer for "no such number", "another commune's number" and "soft
// deleted".
//
// THREE CAUSES, ONE RESPONSE, ON PURPOSE. Telling them apart tells a caller which numbers exist in a
// register they are not reading.
func (h *Handler) khongTimThayNhiemVu(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy nhiệm vụ.", "")
}
