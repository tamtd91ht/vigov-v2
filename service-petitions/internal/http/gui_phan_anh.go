package http

// The CITIZEN intake — POST /api/v1/my-citizen-reports.
//
// # THIS IS THE ONE WRITE A MEMBER OF THE PUBLIC PERFORMS ON THIS SYSTEM
//
// Everything else in this repository is written by staff, whose identity is issued by an
// administrator and whose every act is attributable. This one is written by whoever is holding a
// phone that received an OTP (rule 4). That difference is why nothing on this path is taken from
// the request except the four values a citizen genuinely types.
//
// FIVE FACTS ABOUT THE RECORD COME FROM SOMEWHERE ELSE, and each of them is a lever if it does not:
//
//	cong_dan_id      the SESSION (rule 4, invariant 2) — a body field here reads as another person
//	the commune      the SESSION (rule 1, ADR 0022)    — a body field here files into another commune
//	kenh_tiep_nhan   a CONSTANT in the use case        — the channel decides which SLA rules apply
//	ma_tra_cuu       crypto/rand, in the use case      — a chosen code is a code somebody can guess
//	han_tiep_nhan    identity, in the use case         — a chosen deadline is a chosen commitment
//
// A body mentioning one of them is refused outright rather than having the field quietly ignored,
// so an integrator learns these are not theirs to set instead of watching a value disappear.
//
// # THE COMMUNE IS THE ONE EXCEPTION TO THAT, AND THE EXCEPTION IS DELIBERATE
//
// There is NO commune field on the struct below — not even one declared in order to be refused. A
// struct tag naming the commune on a REQUEST type is the exact shape rule 1, forbidden #2 is about,
// and hooks/tenant_scope_guard.py blocks it on sight. It cannot tell "declared so I can refuse it"
// from "declared so I can read it", and that is the correct call: the two are one edit apart and
// the second one is silent.
//
// Not declaring it is the STRONGER answer in any case. encoding/json drops fields the struct does
// not name, so such a value cannot reach anything at all — whereas a refusal is a branch somebody
// could later delete. The commune comes from httpx.XaTuPhien and there is no code path here through
// which it could come from elsewhere.

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// guiPhanAnhVao is the body of a citizen's submission.
//
// THE SECOND BLOCK OF FIELDS EXISTS ONLY TO BE REFUSED. They are pointers so "the client mentioned
// this" is distinguishable from "the client sent a zero" — the same shape themLoaiNhiemVuVao uses
// for `source` and `tier`, and for the same reason: silently dropping a field a client believed in
// is how a client ships against a contract that does not exist.
//
// THE SPELLINGS ARE DOUBLED ON PURPOSE (`citizen_id` and `cong_dan_id`, `field` and `linh_vuc`). A
// refusal that only catches the English spelling catches only the integrator who read the contract.
type guiPhanAnhVao struct {
	Content   string `json:"content"`
	Address   string `json:"address"`
	Reporter  string `json:"reporter_name"`
	Phone     string `json:"reporter_phone"`
	Anonymous bool   `json:"anonymous"`

	// --- refused, every one of them ----------------------------------------------------------

	// WHOSE PETITION IT IS. Rule 4, forbidden #1 — the top entry of that rule's list, because a
	// parameter like this looks entirely ordinary in a diff.
	CitizenID *string `json:"citizen_id"`
	CongDanID *string `json:"cong_dan_id"`

	// WHICH FIELD, i.e. WHICH SLA. Open question #23, closed in the other direction (ADR 0028):
	// letting the citizen pick would let every pothole be filed as `An ninh trật tự` and take that
	// field's 2-hour commitment.
	Field   *string `json:"field"`
	LinhVuc *string `json:"linh_vuc"`

	// THE CHANNEL, THE CODE, THE STATUS AND THE THREE INSTANTS. Each is either immutable in the
	// schema or a commitment the authority makes, and none is an input.
	Channel        *string `json:"channel"`
	Code           *string `json:"code"`
	Status         *string `json:"status"`
	ClockFrom      *string `json:"clock_from"`
	AcknowledgeDue *string `json:"acknowledge_due"`
	ResolveDue     *string `json:"resolve_due"`
}

// truongKhongPhaiCuaClient reports whether the body claimed a fact the client does not decide.
func (v guiPhanAnhVao) truongKhongPhaiCuaClient() bool {
	return v.CitizenID != nil || v.CongDanID != nil ||
		v.Field != nil || v.LinhVuc != nil ||
		v.Channel != nil || v.Code != nil || v.Status != nil ||
		v.ClockFrom != nil || v.AcknowledgeDue != nil || v.ResolveDue != nil
}

// GuiPhieu receives one petition from the citizen filing it.
// POST /api/v1/my-citizen-reports
//
// # WHAT THE CITIZEN GETS BACK, AND WHY IT IS THE SAME SHAPE THE GET RETURNS
//
// Rule 10, invariant 1: the lookup code is returned THE MOMENT the petition is received — it is the
// whole of the commitment, and the only name the citizen will ever have for this record. The body
// is `phieuCuaToiRa`, identical in shape to what GET .../{maTraCuu} answers, so the Mini App renders
// one screen from one type and a citizen who submits and then refreshes sees the same thing. One
// fact, one representation (rule 9).
//
// The contact details come back MASKED even though the citizen just typed them — the two reasons
// are on phieuCuaToiRa.ReporterPhone, and they hold identically here.
//
// # THE COMMUNE HAS NOT CONFIGURED ITS SLA: 503, AND THE INTAKE FAILS
//
// This is TODAY'S ANSWER FOR EVERY COMMUNE — the deadline table is seeded by nothing and the
// onboarding step does not exist — so this route currently refuses every submission in the system.
// That is the correct behaviour and not a gap: a deadline invented by software is a promise made on
// behalf of a public authority (rule 10, forbidden #3), and a petition stored without one is a
// commitment nobody counts while every on-time report counts it anyway.
//
// 503 AND NOT 500: nothing is broken. The service is healthy, the request was well formed, and what
// is missing is a configuration a HUMAN enters on a screen. 500 would send an operator looking for
// a fault in the software. NO `Retry-After` HEADER, deliberately: retrying in a second will not
// help, and a number here would be a second promise nobody can keep.
//
// THE SENTENCE THE CITIZEN READS NAMES NO INTERNAL DETAIL (rule 3, forbidden #3) — not a table, not
// identity, not gRPC. It says the channel is not open, says plainly that the report was NOT
// recorded, and names what to do instead. The commune id goes to the LOG, where an operator can act
// on it; it is not personal data.
//
// # NO 403 ON THIS ROUTE AND THERE CANNOT BE
//
// Citizens hold no permissions (rule 5, invariant 6). Every refusal is "not a usable session" (401),
// "your request is malformed" (400), "the channel is not open" (503) or a failure (500).
func (h *HandlerCongDan) GuiPhieu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var vao guiPhanAnhVao
	if !docThan(w, r, &vao) {
		return
	}
	// BEFORE ANYTHING ELSE, and before any work is done on the request.
	if vao.truongKhongPhaiCuaClient() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Yêu cầu chứa thông tin do hệ thống tự xác định (người gửi, lĩnh vực, kênh, mã tra cứu, "+
				"trạng thái hoặc thời hạn). Hãy gửi lại chỉ với nội dung phản ánh.", "")
		return
	}

	congDan, ok := h.congDanThucHien(r)
	if !ok {
		// A WIRING FAULT, ANSWERED 500 — the same reasoning as PhieuCuaToi. authz.CitizenOnly
		// refuses every request without a citizen principal before this function runs, so reaching
		// here means the route was mounted without that guard. 401 would tell the citizen to sign in
		// again, which will not help and hides the fault.
		h.d.Log.Error("tuyến gửi phản ánh chạy mà không có danh tính trong phiên — thiếu " +
			"authz.CitizenOnly hoặc authz.CitizenPrincipal trên chuỗi rìa")
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	p, err := h.d.GuiPhieu.Gui(ctx, app.YeuCauGuiPhanAnh{
		NoiDung:   vao.Content,
		DiaChi:    vao.Address,
		HoTen:     vao.Reporter,
		DienThoai: vao.Phone,
		AnDanh:    vao.Anonymous,
	}, congDan)
	if err != nil {
		h.traLoiLoiGui(w, r, err)
		return
	}

	// THE CODE IS HANDED TO idem BEFORE IT IS HANDED TO THE CITIZEN. A retry with the same
	// Idempotency-Key is then answered with this code rather than a bare 409 — without it the
	// citizen would never learn the code while their petition sits in the system, which is rule 10,
	// invariant 1 broken by the very mechanism meant to protect it (core/idem.RecordCode).
	//
	// A no-op on a request that carried no claim, so it is called unconditionally.
	idem.RecordCode(ctx, p.MaTraCuu)

	// LOGGED: the code and the commune, and NOTHING ELSE. The code is a business identifier and is
	// what an operator or a citizen quotes; the name, the number and the text of the report are
	// citizen personal data and never enter a log line (rule 3, invariants 1 and 2).
	h.d.Log.Info("đã tiếp nhận phản ánh của công dân",
		"xa", string(tenant.MustFrom(ctx)), "ma_tra_cuu", p.MaTraCuu)

	// 201, and no Location header. The lookup code is the one string that opens this petition, and a
	// Location would put it into every proxy access log on the way back — for a client that already
	// has it in the body and already knows the path.
	//
	// The label is "" because the field code is empty until an officer classifies it. There is no
	// catalogue read on this path, and there must not be: a field that is not set has no label.
	vietJSON(w, http.StatusCreated, phieuCuaToiRaNgoai(p, ""))
}

// congDanThucHien builds the audit actor from the SESSION, and from nothing else.
//
// IT IS THE ONLY PRODUCER OF A CITIZEN IDENTIFIER ON THE WRITE PATH, the twin of danhTinhTuPhien on
// the read path, so there is exactly one place to read when asking "could this value have come from
// the client". It cannot: authz.Principal is filled by core/authz.CitizenPrincipal from
// httpx.CitizenSessionFrom, which the edge fills from the bearer token alone.
//
// THE `Kind` CHECK IS NOT REDUNDANT WITH authz.CitizenOnly — same argument as danhTinhTuPhien. It
// also feeds `Actor.Kind`, which the ledger stores: a citizen's act recorded as a staff one makes
// the whole trail unable to answer "who did this".
//
// THE IP COMES FROM THIS PROCESS'S OWN SOCKET. httpx.ClientIP does not trust X-Forwarded-For, and
// rule 6, invariant 2 wants the address the request really arrived from — not one the caller named.
func (h *HandlerCongDan) congDanThucHien(r *http.Request) (audit.Actor, bool) {
	p, ok := authz.From(r.Context())
	if !ok || p.Kind != "citizen" || p.ID == "" {
		return audit.Actor{}, false
	}
	return audit.Actor{ID: p.ID, Kind: p.Kind, IP: httpx.ClientIP(r)}, true
}

// traLoiLoiGui maps one intake failure onto a status and a sentence a citizen can act on.
//
// THREE BRANCHES, AND THE DEFAULT IS THE NARROW ONE. Listing what counts as the sender's fault
// rather than defaulting to 400 is the same discipline as laLoiDauVao: a default of "anything I do
// not recognise is the client's fault" turns an identity outage into a 400, and the Mini App then
// tells a citizen to fix their report forever while nobody is told the server is broken.
func (h *HandlerCongDan) traLoiLoiGui(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	switch {
	case domain.LaLoiGuiPhanAnh(err):
		// The domain's own sentence is returned: it names the field and the rule, holds no personal
		// data (proved in domain/gui_phan_anh_test.go) and a second sentence written here would
		// drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")

	case errors.Is(err, app.ErrChuaAnDinhDuocHan):
		// WARN AND NOT ERROR: this service is healthy and answering. The commune is the whole of the
		// actionable information — it names which commune's configuration screen is empty — and the
		// wrapped chain carries the gRPC code core/identityclient already logged.
		h.d.Log.Warn("CẢNH BÁO: từ chối tiếp nhận phản ánh vì chưa ấn định được hạn của xã",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "intake_not_configured",
			"Xã chưa mở kênh tiếp nhận phản ánh trực tuyến nên chưa nhận được phiếu của bạn. "+
				"Phản ánh của bạn CHƯA được ghi nhận. Vui lòng liên hệ trực tiếp UBND xã.", "")

	default:
		// The wrapped error carries the failure and never reaches the citizen (rule 3, forbidden #3).
		h.d.Log.Error("gửi phản ánh: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
