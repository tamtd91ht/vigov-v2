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
	"fmt"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/identityclient"
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
// THE `Theo văn bản` DOCUMENT BLOCK OF §5.4 IS ON THIS RESPONSE SINCE MIGRATION 0009 — see the
// `documents` field and read its note before rendering it, because `null` and `[]` are two different
// statements on this one field.
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

	// AllowedTransitions lists the status codes this task may move to from Status — computed by the
	// server from THE SAME MAP POST …/status enforces (domain.TrangThaiNhiemVu.AllowedTransitions), in
	// the order a picker shows them. vigov-require 56cfc3b's `allowed_transitions`: the screen draws
	// these and keeps no second copy of the lifecycle. ADDED 28/09/2026, additive to a published reply.
	//
	// ⚠ IT IS THE LIFECYCLE'S SHAPE, THE SAME FOR EVERY CALLER. Signing off `cho-duyet` →
	// `hoan-thanh`, reopening, and returning `cho-duyet` → `dang-thuc-hien` are listed for everybody and
	// answer 403 without `task.approve`; the direct `dang-thuc-hien` → `hoan-thanh` needs no key since
	// ADR 0065 NV1, so the assignee's `hoan-thanh` choice is a real one. Completing a parent with
	// unfinished sub-tasks, or reopening a child under a finished parent, is listed and answers 409.
	// All are decided on the row under the lock, never from this list. `chuyen-tiep` is never listed —
	// forwarding is POST …/assignment.
	//
	// ALWAYS AN ARRAY, never null: `[]` is "no move from here" (a status no map knows).
	AllowedTransitions []string `json:"allowed_transitions"`

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

	// Parent is the REGISTER NUMBER (`NV19`) of the task this one hangs off (§5.10), EMPTY for a root
	// task.
	//
	// A REGISTER NUMBER SINCE 28/09/2026, AND NO LONGER THE PARENT'S INTERNAL id. The create and
	// update bodies take a number in their own `parent` (resolved in this commune, inside the
	// transaction), so this field is the same kind of value in both directions: a drawer can send
	// back exactly what it read, and "Thêm việc con" under NV19 sends `parent: "NV19"` — the `code`
	// the drawer already holds. It was an id before, and no client could ever form a request with it,
	// because no response FIELD carries a task's own id (the opaque `next_cursor` embeds one as its
	// tie-break, which is not something a request field accepts).
	//
	// Resolved for a whole page in one statement (store.attachTreeFacts), never one read per row.
	//
	// ⚠ NOTHING ABOUT THE PARENT'S DEADLINE IS ON THIS RESPONSE, and that is ADR 0037 decision 2: a
	// sub-task has a deadline OF ITS OWN or none, so a child may be overdue while its parent is not.
	// A screen showing one red dot on the parent would be showing a fact nobody recorded.
	Parent string `json:"parent"`

	// ChildCount is the number of LIVE direct children — §4.1's `{n} việc con` chip and the size of
	// §5.10's block. Soft-deleted children are not counted (rule 7, invariant 2). The children
	// themselves are GET /api/v1/tasks?parent=<code>.
	//
	// COUNTED BY THE SERVER, for the whole page in one statement, and never from the rows a client
	// happens to hold: the same task would read `2 việc con` on one page and `3` on the next.
	ChildCount int `json:"child_count"`

	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is `cap_nhat_luc` — the instant of the row's last write. ADDED 28/09/2026 as the
	// token for PATCH's optional `expected_updated_at` precondition: send it back unchanged. It is
	// fresh on every write reply (the use case reads it after its own write). NOT a business fact —
	// "last activity on the work" is the timeline's question; a log entry does not move this value.
	UpdatedAt time.Time `json:"updated_at"`

	// Documents is §5.4's "SỔ THEO DÕI VĂN BẢN CHỈ ĐẠO" — the three dynamic lists of §7.2, in the
	// order the block is drawn (group, then position).
	//
	// ⚠ `null` AND `[]` MEAN TWO DIFFERENT THINGS HERE, AND A CLIENT THAT FOLDS THEM IS WRONG ON ONE
	// OF THEM:
	//
	//	null  NOT SENT ON THIS SURFACE. The register LIST (GET /api/v1/tasks) does not carry the
	//	      block BY DEFAULT — §4.1's card and §4.2's table do not draw it, and three lists of free
	//	      text per row of every page is payload nobody renders. It says NOTHING about whether the
	//	      task has lines.
	//	[]    THIS TASK HAS NO LINES. The detail read, the two write routes, and the LIST UNDER
	//	      `include=documents` (§4.3's Sổ theo dõi, added 28/09/2026 — one batched read per page)
	//	      answer this, and they always answer with an array.
	//
	// Rendering `null` as "no documents" is how a card would report an empty block for a task with
	// three. The distinction is the same one `domain.NhiemVu.VanBan` carries inside the service.
	//
	// THE LINES ARE NOT MASKED AND DO NOT NEED TO BE. Every field here is text a member of staff
	// typed about an administrative document; no citizen name, number or address is on the record —
	// and the day one is, this needs the branch phieuRaNgoai has (rule 3).
	Documents *[]nhiemVuVanBanRa `json:"documents,omitempty"`

	// MeetingID, MeetingTitle and ConclusionNo are the drawer's back-link for a task split from a
	// meeting conclusion (`source` = `ket-luan-hop`): the minutes' internal id (what
	// GET /api/v1/meetings/{id} takes), their title, and the conclusion's ordinal (①②③).
	//
	// ABSENT for every other source, AND absent when the meeting or the conclusion has been
	// soft-deleted — the task keeps `source_id`; there is simply nothing live to link to. Resolved by
	// the register's two reads in one batch statement; the write replies do not carry it.
	//
	// All three are the MINUTES' data (this service owns both tables), not a join into another
	// service, and none is personal data: a title and an ordinal name a meeting.
	MeetingID    string `json:"meeting_id,omitempty"`
	MeetingTitle string `json:"meeting_title,omitempty"`
	ConclusionNo int    `json:"conclusion_no,omitempty"`
}

// nhiemVuVanBanRa is ONE line of the block.
//
// THE FIELD NAMES SAY WHAT THE DATA IS, NOT WHAT THE SCREEN CALLS IT (ADR 0017): `reference` is the
// document's number and symbol, which §5.4 labels nothing at all because it renders it inline.
type nhiemVuVanBanRa struct {
	// ID is the line's internal id, and it is ON THE WIRE because the edit route needs it: PATCH
	// sends the block back WHOLE, and a line without an id would arrive as a new one on every save —
	// so a task's three documents would become six, then nine.
	//
	// IT IS AN INTERNAL id AND NAMES NOTHING OUTSIDE THIS SERVICE, exactly like `parent` above and
	// unlike `code`. A line has no business number: it is a field value of the task, not a record.
	ID string `json:"id"`

	// Group is one of the three closed codes of §5.4 — `cap-tren-giao`, `chi-dao-dang-uy`,
	// `san-pham-dau-ra`. THE LABELS ARE NOT HERE: they are fixed captions on the screen, and a
	// second copy of a display string is a second copy that drifts.
	Group string `json:"group"`

	// Reference is `1742-CV/BTCTU` and Date is the day printed on the document. BOTH ARE EMPTY IN
	// THE ORDINARY CASE TODAY: §7.2 offers one textarea and does not split them out, so what the
	// clerk typed is in `summary` alone. The server does NOT parse a sentence to fill these — a
	// guessed reference number is a document number that does not exist.
	Reference string `json:"reference"`

	// Date is `YYYY-MM-DD`, or "" when the line records no document date.
	//
	// A DATE AND NOT AN INSTANT, and the column is `DATE` for the same reason: this is the day
	// printed on paper (`9/6/2026`), with no time and no zone. Sending it as RFC 3339 would let a
	// browser's zone decide which DAY a document was signed. The `9/6/2026` of §5.4 is a RENDERING
	// and belongs to the screen (ADR 0017).
	Date string `json:"date"`

	// Summary is the text of the line — the trích yếu, or today the whole sentence §7.2's textarea
	// carried.
	Summary string `json:"summary"`

	// Position is `thu_tu`, the line's place WITHIN ITS GROUP. It is an ISSUED NUMBER and not an
	// array index: it is stable across saves, and a gap in it is the correct trace of a line that
	// was removed (migration 0009). A client must render the array as it arrives — already ordered —
	// rather than sorting on this value and inventing meaning for the gaps.
	Position int `json:"position"`
}

// dinhDangNgay is `YYYY-MM-DD`: ISO 8601 with no time and no zone, which is what a `DATE` column
// means and what a JSON contract carries.
//
// ⚠ `ngayHopRa` / `ngayHopVao` (bien_ban_hop.go, bien_ban_hop_ghi.go) WRITE THE SAME LITERAL, and
// this pass deliberately did not fold the three together. Those two are for a MANDATORY date and
// refuse an empty string with the meeting register's own sentinel; these two accept "" because §7.2
// does not collect a document date at all. Merging them would mean one function with a flag deciding
// whether absence is legal — and the reformat would touch a register this task has no business in.
const dinhDangNgay = "2006-01-02"

// ngayVanBanRa renders a document date, and renders the ZERO as "" rather than as the year 1.
//
// Marshalled straight, a zero time.Time travels as `0001-01-01`, which a screen renders happily and
// a reader takes for a real date on a real document.
func ngayVanBanRa(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(dinhDangNgay)
}

// vanBanRaNgoai builds the block. NEVER nil FOR A NON-nil INPUT — an empty array and a MISSING FIELD
// are two different statements on this surface, and the pointer is what lets the CONTRACT say so.
//
// ⚠ TỪNG TRẢ `[]nhiemVuVanBanRa` VÀ DÙNG `null` LÀM DẤU VẮNG MẶT. Hình dạng ấy đúng trên dây và
// SAI TRONG HỢP ĐỒNG: một lát cắt Go nil marshal thành `null`, nhưng `tools/apidoc` khai trường ấy
// là mảng BẮT BUỘC, KHÔNG NULL. Nên `tsc` tin `nhiemVu.documents` luôn là một mảng và cho gọi
// `.map(...)` — lời gọi ấy nổ trên tuyến SỔ, đúng tuyến cố ý không phục vụ khối này. Đo 24/09/2026.
//
// Con trỏ + `omitempty` nói đúng cùng một điều bằng thứ hợp đồng diễn đạt được: VẮNG MẶT trên sổ,
// CÓ MẶT (có thể rỗng) ở tuyến chi tiết — và `tsc` bắt mọi lời gọi quên kiểm.
func vanBanRaNgoai(ds []domain.NhiemVuVanBan) *[]nhiemVuVanBanRa {
	if ds == nil {
		return nil
	}
	ra := make([]nhiemVuVanBanRa, 0, len(ds))
	for _, v := range ds {
		ra = append(ra, nhiemVuVanBanRa{
			ID:        v.ID,
			Group:     string(v.Nhom),
			Reference: v.SoKyHieu,
			Date:      ngayVanBanRa(v.NgayVanBan),
			Summary:   v.TrichYeu,
			Position:  v.ThuTu,
		})
	}
	return &ra
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
		AllowedTransitions:   allowedTransitionsOut(n.TrangThai),
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
		Parent:               n.ParentCode,
		ChildCount:           n.ChildCount,
		CreatedBy:            n.NguoiTaoMa,
		CreatedAt:            n.TaoLuc,
		UpdatedAt:            n.UpdatedAt,
		// nil IN, nil OUT — the register list loads the block only under `include=documents`, and
		// an absent field on the wire says exactly that. See the note on the field; it is the one place this response has two
		// meanings for one absence, and they are both needed.
		Documents: vanBanRaNgoai(n.VanBan),
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
	if n.NguonGiao == domain.NguonKetLuanHop && n.NguonHop != nil {
		ra.MeetingID = n.NguonHop.BienBanID
		ra.MeetingTitle = n.NguonHop.TenCuocHop
		ra.ConclusionNo = n.NguonHop.ThuTu
	}
	return ra
}

// allowedTransitionsOut renders the lifecycle's moves as codes. Computed from the status alone, so a
// page of fifty tasks costs no read at all — nothing here depends on history.
func allowedTransitionsOut(t domain.TrangThaiNhiemVu) []string {
	moves := t.AllowedTransitions()
	out := make([]string, 0, len(moves))
	for _, m := range moves {
		out = append(out, string(m))
	}
	return out
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
// # TWO OF §3'S FILTERS ASK IDENTITY FIRST, AND NEITHER FALLS BACK
//
// `scope=related` asks identity for the caller's own unit(s) (StaffOrgUnits) and `soon=true` for
// the commune's due-soon cutoff (ResolveDueSoonCutoff). A failure is never "drop the filter" — that
// returns a page answering a different question, with nothing on screen to say so. See
// taskFilterFromRequest for the status each failure answers.
func (h *Handler) DanhSachNhiemVu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE COMMUNE IS FIXED BY httpx.TenantMiddleware FROM Host, and the store binds `tenant_id` to
	// $1 from the context on every statement (rule 1, invariant 5). NOTHING HERE READS `tenant_id`
	// FROM THE QUERY STRING — a client naming its own commune is a client granting itself access
	// (rule 1, forbidden #2).
	thamSo := r.URL.Query()

	// Parsed BEFORE the store is touched: a rejected page request must run no statement at all.
	//
	// ONE PACKAGE-LEVEL ALLOWLIST, and it has to stay that shape: tools/apidoc finds the list by this
	// identifier and publishes its columns (`created_at`, `code`, `due_at`, `priority`, `title`) as the
	// `sort` enum. For `order=asc` on `due_at` or `priority` the store switches to the ascending key
	// itself (petstore.DanhSach), so tasks without a deadline / a priority come LAST in both directions.
	// `title` is a server-side-anchor sort: its cursor carries the row id only, never the title.
	yc, err := page.Parse(thamSo, petstore.SapXepNhiemVu)
	if err != nil {
		// page.HTTPError owns the mapping so every service answers a bad cursor the same way. It
		// never echoes what the client sent: a cursor is opaque, and a rejected sort key is often a
		// probe.
		status, ma, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, ma, thongBao, "")
		return
	}

	// `include=documents` is a PROJECTION, not a filter, so it is parsed here and never reaches
	// LocNhiemVu — GET /api/v1/task-counts shares that struct and has no rows to attach a block to.
	withDocuments, err := taskListIncludeFromQuery(thamSo)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}

	loc, ok := h.taskFilterFromRequest(w, r, thamSo)
	if !ok {
		return
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

	if withDocuments {
		// ONE STATEMENT FOR THE WHOLE PAGE (skills/load-data-once), keyed by the internal ids of the
		// rows just read — so the blocks cannot belong to another commune's tasks, and the store binds
		// `tenant_id` to $1 as well.
		//
		// A FAILURE IS A 500, NOT A PAGE OF EMPTY BLOCKS: `documents: []` on every row would state
		// that no task on the page references a document — a statement about the register made from
		// a failure to read it, on the one view (§4.3) whose columns ARE those documents.
		ids := make([]string, 0, len(kq.Items))
		for _, n := range kq.Items {
			ids = append(ids, n.ID)
		}
		blocks, err := h.d.DanhSachNhiemVu.DocumentsForTasks(ctx, ids)
		if err == nil {
			for i := range kq.Items {
				// A MISSING KEY IS A BROKEN READER, NOT AN EMPTY BLOCK. The store answers every asked
				// id with a non-nil slice; rendering an absent one as `[]` would be the very statement
				// the paragraph above refuses to make.
				b, asked := blocks[kq.Items[i].ID]
				if !asked || b == nil {
					err = errDocumentsBlockMissing
					break
				}
				kq.Items[i].VanBan = b
			}
		}
		if err != nil {
			h.d.Log.Error("danh sách nhiệm vụ: lỗi đọc văn bản của trang",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
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

// taskListIncludeFromQuery reads `include`, the list's one optional projection.
//
// `include=documents` IS THE ONLY ACCEPTED VALUE, AND IT IS OPT-IN ON PURPOSE. The Kanban card (§4.1)
// and the Danh sách table (§4.2) draw no document; only the Sổ theo dõi (§4.3) does. Sending three
// lists of free text on every row of every page by default would be payload two of the three views
// never render — and it would change the default shape of a published reply, where an opt-in adds a
// field only for the caller that names it (additive, rule 2 invariant 7's spirit on the REST side).
//
// ANYTHING ELSE IS REFUSED rather than ignored, for the reason the filters are: a misspelt
// `include=document` silently dropped is a Sổ theo dõi whose three document columns read empty.
func taskListIncludeFromQuery(q map[string][]string) (bool, error) {
	if s := q["include"]; len(s) > 0 {
		if len(s) != 1 || s[0] != "documents" {
			return false, errTaskIncludeInvalid
		}
		return true, nil
	}
	return false, nil
}

// taskFilterNeeds names the parts of the filter locNhiemVuTuQuery may NOT fill in, because they come
// from the session or from identity and that function has neither.
type taskFilterNeeds struct {
	mine    bool // `scope=mine`    — the caller's own code, from the session
	related bool // `scope=related` — the caller's own code AND units (identity StaffOrgUnits)
	soon    bool // `soon=true`     — the commune's cutoff (identity ResolveDueSoonCutoff)
}

// taskFilterFromRequest turns the query string into the store's filter, resolving `scope=mine` and
// `scope=related` from the SESSION and `soon=true` from identity. It writes the refusal itself and
// reports false when it did.
//
// ONE FUNCTION FOR BOTH ROUTES THAT TAKE THESE FILTERS — the register list and the per-status
// counts. The count over a Kanban column must be the count of the cards under it, so both routes
// must turn one query string into one filter; two copies of this would drift at the first new filter.
//
// # WHAT EACH IDENTITY FAILURE ANSWERS — NONE OF THEM IS AN EMPTY OR UNFILTERED PAGE
//
//	ErrDueSoonNotConfigured  409 `due_soon_not_configured` — the commune has set no threshold. A
//	                         configuration fact, not a fault; never 72 hours (rule 10, forbidden #3)
//	anything else            503 `task_filter_unavailable` — the answer was not obtained. Retryable
//	                         when identity was unreachable; the same 503 for a broken contract,
//	                         because the client can do nothing different either way
func (h *Handler) taskFilterFromRequest(w http.ResponseWriter, r *http.Request,
	q map[string][]string) (petstore.LocNhiemVu, bool) {

	ctx := r.Context()
	loc, needs, err := locNhiemVuTuQuery(q)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return loc, false
	}

	if needs.mine || needs.related {
		// FAIL CLOSED. With no business code to compare against, the honest answers are "refuse" and
		// "return the whole register" — and the second is a screen labelled `Giao cho tôi` showing
		// every task in the commune, which nobody would report as a fault. An empty `.Ma` on a staff
		// principal is a wiring fault, so it is a 500 an operator can find, exactly as the four write
		// routes next door treat the same condition.
		principal, ok := authz.From(ctx)
		if !ok || principal.Ma == "" {
			h.d.Log.Error("bộ lọc phạm vi của tôi chạy mà chủ thể không có mã cán bộ — SAI CẤU HÌNH ROUTE",
				"xa", string(tenant.MustFrom(ctx)), "duong", r.URL.Path)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return loc, false
		}
		if needs.mine {
			loc.NguoiThucHienMa = principal.Ma
		}
		if needs.related {
			// THE CALLER'S OWN CODE, FROM THE SESSION — StaffOrgUnits' contract (rule 4, invariant 2
			// in its staff form). The answer NARROWS this list and grants nothing (rule 5).
			units, err := h.d.TaskFilterIdentity.StaffOrgUnits(ctx, principal.Ma)
			if err != nil {
				h.taskFilterUnavailable(w, r, "không lấy được bộ phận của người đăng nhập", err)
				return loc, false
			}
			loc.Related = &petstore.TaskRelatedScope{StaffCode: principal.Ma, OrgUnits: units}
		}
	}

	if needs.soon {
		// ONE `now` FOR BOTH ENDS OF THE WINDOW: it is the asOf identity walks the working hours from,
		// and the lower bound the store compares against — so no task falls between the two.
		now := time.Now().UTC()
		cutoff, err := h.d.TaskFilterIdentity.DueSoonCutoff(ctx, identityv1.WorkKind_WORK_KIND_NHIEM_VU,
			// "" IS THE DEFAULT ROW, and a real request: a task carries no field (linh_vuc).
			"", now)
		switch {
		case errors.Is(err, identityclient.ErrDueSoonNotConfigured):
			httpx.WriteError(w, http.StatusConflict, "due_soon_not_configured",
				"Xã chưa cấu hình ngưỡng sắp đến hạn cho nhiệm vụ, nên chưa lọc được việc sắp đến hạn. "+
					"Vui lòng cấu hình tại Cấu hình → Thời hạn xử lý.", "")
			return loc, false
		case err != nil:
			h.taskFilterUnavailable(w, r, "không lấy được ngưỡng sắp đến hạn của xã", err)
			return loc, false
		}
		loc.DueSoonFrom, loc.DueSoonUntil = now, cutoff
	}
	return loc, true
}

// taskFilterUnavailable answers a filter identity could not resolve. WARN, like every identity
// refusal in this service — this process is healthy, and the wrapped chain carries only the gRPC
// status core/identityclient already logged (no staff code, no personal data).
func (h *Handler) taskFilterUnavailable(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.d.Log.Warn("CẢNH BÁO: từ chối lọc danh sách nhiệm vụ — "+what,
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path, "err", err)
	httpx.WriteError(w, http.StatusServiceUnavailable, "task_filter_unavailable",
		"Chưa áp dụng được bộ lọc này nên chưa hiện danh sách. Vui lòng thử lại sau ít phút.", "")
}

// locNhiemVuTuQuery validates the filters. EVERY VALUE BECOMES A BOUND PARAMETER in the store;
// nothing here builds SQL.
//
// The second return value names the filters whose values this function may NOT fill in — `Giao cho
// tôi`, `Liên quan đến tôi` and `Sắp đến hạn` — because they come from the session or from identity
// and this function has neither (taskFilterNeeds).
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
func locNhiemVuTuQuery(q map[string][]string) (petstore.LocNhiemVu, taskFilterNeeds, error) {
	lay := func(k string) string {
		if v, ok := q[k]; ok && len(v) > 0 {
			return v[0]
		}
		return ""
	}

	var (
		loc   petstore.LocNhiemVu
		needs taskFilterNeeds
	)

	if s := lay("status"); s != "" {
		if !domain.TrangThaiNhiemVu(s).HopLe() {
			return loc, needs, errTrangThaiNhiemVuKhongHopLe
		}
		loc.TrangThai = s
	}
	if s := lay("source"); s != "" {
		if !domain.NguonGiao(s).HopLe() {
			return loc, needs, errNguonGiaoKhongHopLe
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
		return loc, needs, petstore.ErrTimNhiemVuQuaDai
	}

	// §5.10: the DIRECT children of one task, named by its register number. NOT VALIDATED AGAINST
	// THE REGISTER, for the reason `type` and `unit` are not: a number that matches nothing is an
	// empty page, which is true — and the same page a task with no children gets, so the filter
	// cannot be used to learn which numbers exist. Only its LENGTH is bounded, at the length an
	// issued number can have, so the value cannot become a payload.
	loc.ParentCode = lay("parent")
	if len([]rune(loc.ParentCode)) > domain.MaNhiemVuToiDa {
		return loc, needs, errParentCodeTooLong
	}

	// `late=true` IS THE ONLY ACCEPTED SPELLING, and anything else is refused rather than read as
	// false. A checkbox whose value arrived as `1` or `yes` and was silently dropped shows an officer
	// the whole register while the box on their screen is ticked.
	if s := lay("late"); s != "" {
		if s != "true" {
			return loc, needs, errLocTreHanNhiemVuKhongHopLe
		}
		loc.ChiTreHan = true
	}

	// §3's "Sắp đến hạn" checkbox. `soon=true` IS THE ONLY ACCEPTED SPELLING, for the reason `late`
	// has one. The window itself is resolved by taskFilterFromRequest from identity, which walks the
	// commune's own `sla.gio_sap_den_han` in working hours — the "mặc định 72 giờ" of §3 is a
	// commune's default to override, and no copy of it lives in this service.
	if s := lay("soon"); s != "" {
		if s != "true" {
			return loc, needs, errTaskSoonInvalid
		}
		needs.soon = true
	}

	// §3's scope tabs. `all` is the default and needs no predicate.
	switch s := lay("scope"); s {
	case "", "all":
	case "mine":
		needs.mine = true
	case "related":
		// "tôi giao, tôi theo dõi, tôi đã xử lý, hoặc bộ phận tôi đang giữ" — all four clauses, the
		// last from identity's StaffOrgUnits (petstore.relatedCondition lists them). Never three of
		// them: a tab missing the department's work reports nothing.
		needs.related = true
	default:
		return loc, needs, errTaskScopeInvalid
	}

	// `che_do_xem` (Kanban / Danh sách / Sổ theo dõi) IS DELIBERATELY NOT READ. It chooses a LAYOUT
	// over the same rows, so it changes no predicate and no sort; accepting it here would be a
	// parameter the server ignores, which the next reader has to prove is ignored.

	// The overview drill-down (`metric`, plus `from`/`to` for a period figure) — summary.go. ANDed with
	// every filter above; absent, it changes nothing.
	if err := parseTaskMetric(q, &loc); err != nil {
		return loc, needs, err
	}

	return loc, needs, nil
}

var (
	errTrangThaiNhiemVuKhongHopLe = errors.New(
		"`status` không phải một trong bảy trạng thái của nhiệm vụ")
	errNguonGiaoKhongHopLe = errors.New(
		"`source` không phải một trong bốn nguồn giao việc")
	errLocTreHanNhiemVuKhongHopLe = errors.New(
		"`late` chỉ nhận giá trị `true`; bỏ hẳn tham số nếu không lọc theo trễ hạn")
	// errPhamViKhongHopLe is now the PETITION list's unknown-scope refusal only (xu_ly_phan_anh.go):
	// that list still takes `all` and `mine`. The task list accepts `related` too — errTaskScopeInvalid.
	errPhamViKhongHopLe = errors.New(
		"`scope` chỉ nhận `all` hoặc `mine`")
	errTaskScopeInvalid = errors.New(
		"`scope` chỉ nhận `all`, `mine` hoặc `related`")
	errTaskSoonInvalid = errors.New(
		"`soon` chỉ nhận giá trị `true`; bỏ hẳn tham số nếu không lọc việc sắp đến hạn")
	errParentCodeTooLong = fmt.Errorf(
		"`parent` là mã nhiệm vụ của việc cha (ví dụ NV19), tối đa %d ký tự", domain.MaNhiemVuToiDa)
	errTaskIncludeInvalid = errors.New(
		"`include` chỉ nhận giá trị `documents`; bỏ hẳn tham số nếu không cần khối văn bản")
	// errDocumentsBlockMissing never reaches a client (it becomes the generic 500).
	errDocumentsBlockMissing = errors.New(
		"danh sách nhiệm vụ: kho không trả khối văn bản cho một nhiệm vụ đã hỏi")
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

	// §5.4'S DOCUMENT BLOCK — A SECOND READ, AND ONLY ON THIS ROUTE.
	//
	// It is keyed by the task's INTERNAL id, which came from the row just read, so the block cannot
	// belong to a task of another commune even before the store binds `tenant_id` to $1.
	//
	// A FAILURE HERE IS A 500 AND NOT A TASK WITHOUT ITS BLOCK. Answering 200 with `documents: []`
	// would tell the drawer this task has no referenced documents — a statement about the record
	// made from a failure to read it, on the one surface where the block is the screen's content.
	if n.VanBan, err = h.d.NhiemVu.VanBanCuaNhiemVu(ctx, n.ID); err != nil {
		h.d.Log.Error("đọc sổ theo dõi văn bản của nhiệm vụ: lỗi hệ thống",
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
