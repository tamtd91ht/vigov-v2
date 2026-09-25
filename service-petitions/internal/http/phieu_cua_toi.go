package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The CITIZEN read path for one petition. GET /api/v1/my-citizen-reports/{maTraCuu}
//
// # THIS FILE IS A SEPARATE SURFACE FROM phieu_phan_anh.go, NOT A VARIANT OF IT
//
// Rule 4, invariant 5: citizen routes and staff routes are separated AT ROUTING LEVEL and never
// share a handler. That is kept here by TYPES rather than by discipline — HandlerCongDan holds
// DepsCongDan, DepsCongDan holds PhieuCuaCongDanDoc, and PhieuCuaCongDanDoc has exactly one
// method, the one that filters by citizen. There is no field on this handler through which the
// unfiltered staff read (petstore.PhieuPhanAnhStore.TheoMaTraCuu) can be reached, so "the citizen
// route cannot read somebody else's petition" is a fact about the type and not a rule somebody has
// to remember while editing.
//
// Sharing Handler would have cost nothing today and would have put both reads one dot away from
// each other, in a file where the two responses look almost identical.

// PhieuCuaCongDanDoc reads the petitions belonging to ONE citizen, in the commune of the session —
// one by its lookup code, or a page of them.
//
// DELIBERATELY NARROWER THAN PhieuPhanAnhDoc, and the narrowness is the isolation. BOTH methods take
// the citizen identifier as a required argument and neither has a value that switches the filter
// off, so adding the list did not add a way to reach the unfiltered register. See the note at the top
// of this file, and the long argument on petstore.CuaCongDanTheoMaTraCuu for why these are separate
// store methods rather than an optional filter on the staff ones.
type PhieuCuaCongDanDoc interface {
	CuaCongDanTheoMaTraCuu(ctx context.Context, congDanID, ma string) (domain.PhieuPhanAnh, error)
	DanhSachCuaCongDan(ctx context.Context, congDanID, trangThai string, yc page.Request) (
		page.Result[domain.PhieuPhanAnh], error)
}

// GuiPhanAnhCongDan receives ONE petition from the citizen filing it.
//
// A SECOND, SEPARATE INTERFACE RATHER THAN A METHOD ON PhieuCuaCongDanDoc, and the separation is
// the same one rule 6 turns on. The read is a store call; this one opens a TRANSACTION and writes
// an audit entry inside it (rule 6, invariant 3), which is why it is an app use case and not a
// store method. Behind one interface a future caller would reach for whichever method was nearest
// and could end up writing the petition outside a transaction — the exact defect core/audit was
// shaped to make impossible.
//
// IT TAKES THE ACTOR AND NO CITIZEN IDENTIFIER, and that is rule 4, invariant 2 at the type level:
// on a self-filed petition the person acting IS the owner, so one value serves both and there is no
// second parameter a handler could fill from a request body.
type GuiPhanAnhCongDan interface {
	Gui(ctx context.Context, yc app.YeuCauGuiPhanAnh, congDan audit.Actor) (domain.PhieuPhanAnh, error)
}

// DepsCongDan is everything the CITIZEN routes need — and nothing the staff routes need.
//
// NO authz.Checker FIELD, AND THAT ABSENCE IS RULE 5, INVARIANT 6: citizen routes do not use RBAC.
// A citizen holds no role and no permission key; they are isolated by identity (rule 4). A Checker
// here would be a field with nothing to answer, and the first person to find it would look for a
// permission to guard the route with — which is the wrong axis entirely.
type DepsCongDan struct {
	// Phieu is the identity-filtered read. Required; RegisterCongDan refuses a nil at startup.
	Phieu PhieuCuaCongDanDoc

	// GuiPhieu is the citizen intake — the one write on this surface. Required, for a harder reason
	// than the others: a nil here does not crash a screen, it crashes the act rule 10 exists for,
	// in front of a member of the public who has just typed out a complaint.
	GuiPhieu GuiPhanAnhCongDan

	// NhanLinhVuc is the commune's own wording for the field code, so the citizen reads
	// "Rác thải – Vệ sinh môi trường" rather than `rac-thai`. The SAME interface the staff route
	// uses, and sharing it is right here: it is a commune's public vocabulary, not a staff-only
	// fact, and two readers of one catalogue would be two things to keep in step.
	NhanLinhVuc NhanLinhVucDanhMuc

	Log *slog.Logger
}

// HandlerCongDan serves the citizen surface. It holds no business logic — see Handler.
type HandlerCongDan struct {
	d DepsCongDan
}

func NewHandlerCongDan(d DepsCongDan) *HandlerCongDan {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &HandlerCongDan{d: d}
}

// phieuCuaToiRa is one petition as it leaves the API TO THE CITIZEN WHO FILED IT.
//
// # WHAT IS NOT ON IT, AND WHY EACH ABSENCE IS DELIBERATE
//
// Rule 10, invariant 7 and rule 4, forbidden #5: the citizen sees their own petition's PROGRESS.
// Staff notes and routing history stay internal. Read this list before adding a field back — each
// one looks harmless on its own, and the screen will ask for some of them.
//
//	bo_phan_id / can_bo_xu_ly_id   WHO inside the authority is handling it. This is routing
//	                               history in its smallest form, and naming the officer on a
//	                               citizen's screen invites pressure on a named individual over a
//	                               report they did not choose to receive.
//	hien_cong_khai                 a decision STAFF make about the public page, not a fact about
//	                               this citizen's petition.
//	vao_so_luc                     the internal register instant. On the staff-booked channel it
//	                               differs from ClockFrom by up to a week, and explaining that gap
//	                               is an internal matter (ADR 0028).
//	so_lan_mo_lai                  a counter against a per-commune cap (ADR 0008). Nobody has
//	                               decided what a citizen is told about their remaining reopens.
//	id                             the internal ULID means nothing outside this service.
//
// THERE IS NO `overdue` FIELD, for the identical reason there is none on the staff response:
// overdue is DERIVED from a deadline against the current instant (rule 10, invariant 3). A boolean
// built when the response was written is stale the moment the screen keeps rendering, and the
// client needs the deadline anyway to say how long.
//
// # THE TWO NULLS STILL MEAN OPPOSITE THINGS
//
// AcknowledgeDue NULL = "không áp dụng". ResolveDue NULL = "chưa có". A citizen screen must not
// collapse them into one blank: the correct sentence while unclassified is "sẽ được xem trong N
// giờ làm việc", built from AcknowledgeDue, and never an invented resolve date.
type phieuCuaToiRa struct {
	// Code is `ma_tra_cuu` — the string the citizen was handed and the only name they have for
	// this petition.
	Code string `json:"code"`

	Channel string `json:"channel"` // `zalo-mini-app` | `zalo-oa` | `web-xa` | `can-bo-nhap-ho`
	Status  string `json:"status"`  // one of the nine (ADR 0027)

	// Field is the tier-1 code, EMPTY while unclassified. FieldLabel is the commune's own wording
	// and is EMPTY when the commune has not renamed the code — the ordinary case, not an error.
	Field      string `json:"field"`
	FieldLabel string `json:"field_label"`

	Content string `json:"content"`
	Address string `json:"address"`

	// ReporterName and ReporterPhone are ALWAYS MASKED ON THIS SURFACE — "Nguyễn V. A." and
	// "09****0000" — even though the citizen reading them is the person who typed them.
	//
	// TWO REASONS, AND THE SECOND IS THE ONE THAT MAKES IT MORE THAN A FORMALITY (chốt 22/09/2026):
	//
	//  1. Rule 3, invariant 3: anything leaving the API is masked UNLESS the caller holds an
	//     explicit full-view permission. Rule 5, invariant 6 says citizen routes do not use RBAC,
	//     so on this surface there is no permission that CAN be held — and "no permission to check"
	//     resolves to masked, never to open. That is the fail-closed direction; the other one is a
	//     default on the isolation path.
	//  2. A CITIZEN IDENTITY IS WEAK (rule 4). Phone numbers change hands and SIMs get taken over.
	//     Whoever holds the number next inherits the session path to these petitions — masking is
	//     what stops that person HARVESTING the full name and number the previous holder typed into
	//     an old petition. The record keeps the values; the screen does not hand them over.
	//
	// WHAT IT COSTS, STATED RATHER THAN GLOSSED: a citizen cannot proof-read the number they typed
	// beyond its last four digits. That was accepted with the decision.
	//
	// BOTH ARE EMPTY WHEN THE PETITION WAS FILED ANONYMOUSLY, and that holds on this surface too.
	// The record keeps the identity (ADR 0008); no screen shows it, including the filer's own.
	ReporterName  string `json:"reporter_name"`
	ReporterPhone string `json:"reporter_phone"`
	Anonymous     bool   `json:"anonymous"`

	// ClockFrom is `goc_dem_han`: the instant the citizen pressed send, which BOTH deadlines are
	// counted from (ADR 0027, decision D). Without it the two deadlines cannot be explained to the
	// person waiting on them.
	ClockFrom time.Time `json:"clock_from"`

	AcknowledgeDue *time.Time `json:"acknowledge_due"`
	ResolveDue     *time.Time `json:"resolve_due"`

	// Result is `ket_qua_xu_ly` — WHAT THE COMMUNE ACTUALLY DID, written when the petition was
	// closed. Empty until then.
	//
	// IT IS THE ONE PIECE OF STAFF-WRITTEN TEXT THIS SURFACE CARRIES, and it is here because rule 10,
	// invariant 6 requires it: "closing a petition records a result the citizen can read. Never close
	// silently." Everything else a member of staff types about a petition — notes, routing reasons —
	// stays internal (rule 4, forbidden #5); this one string was written FOR the person reading it.
	//
	// IT IS ALSO WHY THE NOTIFICATION DOES NOT CARRY IT. The message that leaves through `comms` says
	// only that the petition was closed and to look it up; the result itself stays behind this
	// authenticated read, because a queue is persisted, replicated and backed up and this text is
	// about one named case (proto/vigov/petitions/v1/events.proto, §CitizenMessage).
	Result string `json:"result"`

	// Reason and ReceivingBody are what the citizen is told when the commune REFUSED the petition
	// (`khong-tiep-nhan`) or PASSED IT ON (`chuyen-cap-tren`) — the branch's equivalent of Result, and
	// on this surface for the same rule-10 reason: a petition must never end silently. Written FOR the
	// citizen (migration 0011), so they are not staff notes (rule 4, forbidden #5).
	//
	// OMITEMPTY so they are absent — not `""` — on the seven statuses that never carry them, and so
	// tools/apidoc does not declare them required. The instant of the act is NOT here: nobody decided
	// the citizen screen shows it, and ClockFrom plus the status already explain the petition.
	Reason        string `json:"reason,omitempty"`
	ReceivingBody string `json:"receiving_body,omitempty"`
}

// phieuCuaToiRaNgoai builds the citizen response.
//
// IT TAKES NO `xemDayDu` PARAMETER, and that is the difference from phieuRaNgoai worth naming: the
// staff builder has a switch between masked and full because a staff permission can open it. This
// one has no switch at all, so there is no argument any call site could pass to unmask. The
// decision is in the type, not in the caller.
func phieuCuaToiRaNgoai(p domain.PhieuPhanAnh, nhan string) phieuCuaToiRa {
	ra := phieuCuaToiRa{
		Code:       p.MaTraCuu,
		Channel:    string(p.Kenh),
		Status:     string(p.TrangThai),
		Field:      p.LinhVuc,
		FieldLabel: nhan,
		Content:    p.NoiDung,
		Address:    p.DiaChi,
		Anonymous:  p.AnDanh,
		ClockFrom:  p.GocDemHan,
		Result:     p.KetQuaXuLy,
	}

	// An anonymous petition carries NEITHER field, not even masked. A masked name is still a name
	// in a commune of a few thousand people, and the promise made when the box was ticked was that
	// no interface shows it.
	if !p.AnDanh {
		ra.ReporterName = privacy.MaskName(p.NguoiGuiHoTen)
		ra.ReporterPhone = privacy.MaskPhone(p.NguoiGuiDienThoai)
	}

	// The two NULLs come from the two predicates that NAME the question being asked, never from a
	// bare IsZero() here.
	if !p.HanTiepNhanKhongApDung() {
		t := p.HanTiepNhan
		ra.AcknowledgeDue = &t
	}
	if !p.ChuaChotHanXuLy() {
		t := p.HanXuLyXong
		ra.ResolveDue = &t
	}
	// GATED ON THE STATUS HERE AS WELL AS BY THE DATABASE'S CHECK (migration 0011). This is the one
	// surface where staff-written text reaches a member of the public, so it states its own condition
	// rather than trusting that the columns are empty elsewhere: a row that ever violated the CHECK
	// (a restore, a manual fix) must not put a refusal reason on a petition that is being processed.
	if p.TrangThai == domain.KhongTiepNhan || p.TrangThai == domain.ChuyenCapTren {
		ra.Reason = p.LyDoKetThucNhanh
		ra.ReceivingBody = p.CoQuanNhan
	}
	return ra
}

// PhieuCuaToi serves ONE petition to the citizen who filed it.
// GET /api/v1/my-citizen-reports/{maTraCuu}
//
// # THE IDENTITY COMES FROM THE SESSION, AND THERE IS NO OTHER PATH INTO THIS FUNCTION
//
// Rule 4, invariant 2. The value is authz.Principal.ID, which core/authz.CitizenPrincipal copied
// from the httpx.CitizenSession the edge resolved from the bearer token (ADR 0022). This handler
// reads NOTHING from the query string, from a header or from the body — rule 4, forbidden #1 is
// the top entry of that rule's list because `?phone=` looks like an ordinary parameter in a diff.
//
// # THE COMMUNE ALSO COMES FROM THE SESSION, AND NOT FROM Host
//
// httpx.XaTuPhien put it in the context (ADR 0022); the Mini App has no domain, so there is no
// Host to derive one from. A request whose session carries commune B therefore queries commune B,
// and petstore.CuaCongDanTheoMaTraCuu binds that commune to $1 — commune A's petition matches no
// row and the citizen is told the same thing an unknown code is told.
//
// # ONE ANSWER, FOUR CAUSES — AND THE TEST THAT MATTERS IS THE ONE COMPARING TWO RESPONSES
//
// No such code · ANOTHER CITIZEN'S CODE · another commune's code · soft deleted all produce the
// identical 404 body (rule 4, forbidden #2). A 403 for the second one would confirm to somebody
// working through codes that they found a real petition — and the existence of a petition is
// itself information about a citizen.
//
// # NO AUDIT ENTRY IS WRITTEN HERE, AND THAT IS A CONCLUSION RATHER THAN AN OMISSION
//
// Rule 6, invariant 7 audits reading FULL personal data and reading ACROSS communes. This route
// does neither: the contact fields are masked on the way out, and the query cannot leave the
// commune of the session. Rule 6, invariant 1 audits every WRITE, and this route writes nothing.
//
// It follows that this route does NOT touch open question #25 — the unanswered question of where
// the trail of an act belonging to NO commune is kept. That question is reached by acts that
// happen before a commune is chosen; this one runs behind httpx.XaTuPhien, which answers 401 until
// a commune is in the session. Were the masking ever removed, this paragraph stops being true:
// the read becomes a full-personal-data read, it needs a trail, and the trail needs an audit.Actor
// of a kind nobody has defined for citizens.
func (h *HandlerCongDan) PhieuCuaToi(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	congDanID, ok := h.danhTinhTuPhien(ctx)
	if !ok {
		// A WIRING FAULT, ANSWERED AS 500 AND NOT AS 401 OR 404. authz.CitizenOnly refuses every
		// request without a citizen principal before this function runs, so reaching here means
		// the route was mounted without that guard. 401 would tell the citizen to sign in again,
		// which will not help and hides the fault; 404 would say their petition is gone. The same
		// reasoning is on petstore.ErrThieuDinhDanhCongDan.
		h.d.Log.Error("tuyến công dân chạy mà không có danh tính trong phiên — thiếu authz.CitizenOnly " +
			"hoặc authz.CitizenPrincipal trên chuỗi rìa")
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	ma := r.PathValue("maTraCuu")
	if ma == "" {
		// An empty path segment matches no code and has no business reaching the database.
		h.khongTimThay(w)
		return
	}

	p, err := h.d.Phieu.CuaCongDanTheoMaTraCuu(ctx, congDanID, ma)
	if err != nil {
		if errors.Is(err, petstore.ErrPhieuKhongTonTai) {
			h.khongTimThay(w)
			return
		}
		// NEITHER THE CODE NOR THE CITIZEN IDENTIFIER IS LOGGED. The code opens a citizen's
		// petition and the identifier ties a pile of lines together into one person's activity
		// (rule 3). The commune is not personal data and is what an operator needs.
		h.d.Log.Error("đọc phiếu của công dân: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// THE RESTRICTED FIELD `can-bo` IS NOT CHECKED HERE, AND THE ASYMMETRY IS CORRECT. That key
	// protects a report about a member of staff from being read by THAT PERSON'S COLLEAGUES
	// (feedback.restricted, ADR 0030). The person reading here is the one who FILED it. Hiding
	// their own report from them would protect nobody and would tell a citizen their complaint
	// about an officer had vanished — which is precisely how trust in the channel dies.
	nhan := ""
	if p.LinhVuc != "" {
		nhan, err = h.nhanCuaLinhVuc(ctx, p.LinhVuc)
		if err != nil {
			h.d.Log.Error("đọc nhãn lĩnh vực cho tuyến công dân: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
	}

	vietJSON(w, http.StatusOK, phieuCuaToiRaNgoai(p, nhan))
}

// danhTinhTuPhien reads the citizen identifier out of the principal the edge built.
//
// IT IS THE ONLY FUNCTION IN THIS FILE THAT PRODUCES A CITIZEN IDENTIFIER, so there is one place
// to read when asking "could this value have come from the client". It cannot: authz.Into is
// called by core/authz.CitizenPrincipal from httpx.CitizenSessionFrom, and the edge fills that
// from the bearer token alone (core/httpx/citizen.go).
//
// THE `Kind` CHECK IS NOT REDUNDANT WITH authz.CitizenOnly. That guard runs on the route and this
// runs in the handler; if the two are ever separated — a handler reused, a middleware reordered —
// a STAFF principal reaching here would otherwise be used as a citizen identifier, and the query
// would filter `cong_dan_id` by a staff id, match nothing, and look like a working route.
func (h *HandlerCongDan) danhTinhTuPhien(ctx context.Context) (string, bool) {
	p, ok := authz.From(ctx)
	if !ok || p.Kind != "citizen" || p.ID == "" {
		return "", false
	}
	return p.ID, true
}

// khongTimThay is the ONE answer for "no such code", "another citizen's code", "another commune's
// code" and "soft deleted".
//
// THE BODY IS BYTE-FOR-BYTE THE SAME IN ALL FOUR CASES, which is the property rule 4, forbidden #2
// actually requires — an identical status code with a differing message or error key leaks exactly
// what the identical status was hiding.
func (h *HandlerCongDan) khongTimThay(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy phiếu phản ánh.", "")
}

// nhanCuaLinhVuc finds the commune's wording for one code. The code set is CLOSED at twelve
// (ADR 0026), so reading the whole override set and scanning it costs nothing and avoids a second
// read path to keep in step with the list one — the same reasoning as Handler.nhanCuaLinhVuc.
func (h *HandlerCongDan) nhanCuaLinhVuc(ctx context.Context, ma string) (string, error) {
	ds, err := h.d.NhanLinhVuc.DanhSach(ctx)
	if err != nil {
		return "", err
	}
	for _, n := range ds {
		if n.Ma == ma {
			return n.Nhan, nil
		}
	}
	return "", nil
}
