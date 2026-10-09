package http

// THE ACCOUNTLESS PETITION SURFACE — ADR 0083, A TEMPORARY PATH.
//
// While App ViHAT is not approved by Zalo, `getAccessToken` fails (-1401) and no Mini App session can be
// opened, so no petition can be sent through any session route. ADR 0083 opens three PUBLIC routes —
// send, lookup by code, field catalogue — and nothing else (stop condition #1). REMOVE THIS FILE,
// routes_accountless.go and their wiring in cmd/server when the app passes review and `getAccessToken`
// works on a real device (ADR 0083 §Gỡ bỏ); the petitions already received stay (rule 7).
//
// # THE COMMUNE COMES FROM A CLIENT VALUE HERE, AND ONLY HERE
//
// Rule 1 forbidden #2 is relaxed by ADR 0083 (§"Vì sao luật 1 cấm #2 được nới ở đây"), for these routes
// alone: the client sends the DOMAIN printed on the commune's QR (`host`), never a tenant_id; the SERVER
// asks the platform which commune holds it, and serves only an ACTIVE one. Sending a petition to a
// commune grants the sender nothing to read there — like walking into a commune office to file a
// complaint. An unknown or inactive domain writes nothing (404).
//
// # WHAT EACH ROUTE IS BOUNDED BY (rule 13, invariant 7)
//
//	send     ratelimit.AccountlessSend, 5/hour per (host, network) + domain.AccountlessDailyCeiling per
//	         commune per day in the database + idem.RequiredAccountless per (commune, 128-bit key)
//	lookup   ratelimit.AccountlessLookup, 30/hour per network, counted before anything else
//	fields   ratelimit.AccountlessFieldRead, per (host, network)
//
// All three limiters FAIL CLOSED: a Redis outage answers 503 (ADR 0083 row 12).

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// AccountlessPetitionReader is the ONE read the public lookup may make: a live accountless petition of
// the context's commune by its code (petstore.PhieuPhanAnhStore.AccountlessByCode). No other read is
// reachable from this surface — the same type-level isolation phieu_cua_toi.go uses.
type AccountlessPetitionReader interface {
	AccountlessByCode(ctx context.Context, ma string) (domain.PhieuPhanAnh, error)
}

// DepsAccountless is everything the accountless routes need. Every field is required; RegisterAccountless
// refuses a nil at startup.
type DepsAccountless struct {
	// Communes resolves a domain to the commune holding it, telling an outage (error) from an unknown
	// host (false). ITS OWN tenant.CachedDirectory instance — never the staff edge's, which caches an
	// outage as a miss (core/tenant/cache.go, XaTheoHost).
	Communes tenant.HostResolver

	// Intake is the SAME use case the session intake uses (app.GuiPhanAnh): same validation, same
	// deadlines, same trail in the same transaction — only the sender differs (ADR 0083 row 8).
	Intake GuiPhanAnhCongDan

	// Petitions is the accountless-only read.
	Petitions AccountlessPetitionReader

	// Fields is the SAME filtered catalogue the session route serves (ADR 0083 row 8).
	Fields CitizenFieldCatalogue

	SendLimiter   *ratelimit.Limiter // ratelimit.AccountlessSend
	LookupLimiter *ratelimit.Limiter // ratelimit.AccountlessLookup
	FieldsLimiter *ratelimit.Limiter // ratelimit.AccountlessFieldRead

	Log *slog.Logger
}

// HandlerAccountless serves the accountless routes. No business logic — see Handler.
type HandlerAccountless struct {
	d DepsAccountless
}

func newHandlerAccountless(d DepsAccountless) *HandlerAccountless {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &HandlerAccountless{d: d}
}

// accountlessReportIn is the body of POST /api/v1/public-citizen-reports: the session body
// (guiPhanAnhVao) field for field, plus `host`. Spelled out rather than embedded because tools/apidoc
// does not read embedded structs; toSession converts it so the refuse-list check is the session route's.
//
// `host` IS THE DOMAIN ON THE QR, never a tenant_id — see the file comment.
type accountlessReportIn struct {
	Host string `json:"host"`

	Content   string  `json:"content"`
	Address   string  `json:"address"`
	Reporter  string  `json:"reporter_name"`
	Phone     string  `json:"reporter_phone"`
	Anonymous bool    `json:"anonymous"`
	Field     *string `json:"field,omitempty"`

	// REFUSED ON THIS PATH (400), unlike the session body: the scene location reaches the app only through
	// a Zalo-signed getLocation token exchange (ADR 0050, vihat-miniapp), which needs the Zalo access this
	// path exists because it does not have — so a coordinate here was not obtained the way the form's
	// location is. Refused rather than dropped, so a client sending one learns it was not stored.
	Lat *float64 `json:"lat"`
	Lng *float64 `json:"lng"`

	// ResidentialUnitID is REFUSED ON THIS PATH (400), like the location: the session intake accepts it
	// (ADR 0088), but this TEMPORARY path has no route a sender could have read the commune's list from
	// (ADR 0088 open item 2) and no owner to answer for it. Refused rather than dropped, so a client
	// sending one learns it was not stored.
	ResidentialUnitID *string `json:"residential_unit_id"`

	// --- refused, every one of them: the session body's list (guiPhanAnhVao), same JSON names -----
	CitizenID       *string `json:"citizen_id"`
	CitizenIDLegacy *string `json:"cong_dan_id"`
	FieldLegacy     *string `json:"linh_vuc"`
	Channel         *string `json:"channel"`
	Code            *string `json:"code"`
	Status          *string `json:"status"`
	ClockFrom       *string `json:"clock_from"`
	AcknowledgeDue  *string `json:"acknowledge_due"`
	ResolveDue      *string `json:"resolve_due"`
}

func (v accountlessReportIn) toSession() guiPhanAnhVao {
	return guiPhanAnhVao{
		Content: v.Content, Address: v.Address, Reporter: v.Reporter, Phone: v.Phone,
		Anonymous: v.Anonymous, Field: v.Field,
		CitizenID: v.CitizenID, CongDanID: v.CitizenIDLegacy, LinhVuc: v.FieldLegacy, Channel: v.Channel,
		Code: v.Code, Status: v.Status, ClockFrom: v.ClockFrom,
		AcknowledgeDue: v.AcknowledgeDue, ResolveDue: v.ResolveDue,
	}
}

// accountlessReceiptOut is the 201 of the send — and of its replay (ADR 0083 row 12: "Thân 201 chỉ gồm
// mã tra cứu và các trường của tra cứu"). Names and formats are phieuCuaToiRa's. NO contact details, no
// content: the sender typed them, and a response is one more place for them to be logged.
type accountlessReceiptOut struct {
	Code           string     `json:"code"`
	Status         string     `json:"status"`
	AcknowledgeDue *time.Time `json:"acknowledge_due"`
	ResolveDue     *time.Time `json:"resolve_due"`
}

// accountlessLookupOut is the public lookup (ADR 0083 rows 4 and 12): status, the two deadlines, the
// result and the reason — and NOTHING ELSE (stop condition #1). No name, phone, content, address,
// coordinates, field, channel or clock origin: whoever holds the code sees the progress only (cost #4).
// Names and semantics are phieuCuaToiRa's: `result` "" until closed; `reason` only on the two branches.
type accountlessLookupOut struct {
	Code           string     `json:"code"`
	Status         string     `json:"status"`
	AcknowledgeDue *time.Time `json:"acknowledge_due"`
	ResolveDue     *time.Time `json:"resolve_due"`
	Result         string     `json:"result"`
	Reason         string     `json:"reason,omitempty"`
}

// accountlessDeadlines are the two deadlines from the predicates that NAME the question — the
// phieuCuaToiRaNgoai rule, never a bare IsZero().
func accountlessDeadlines(p domain.PhieuPhanAnh) (ack, resolve *time.Time) {
	if !p.HanTiepNhanKhongApDung() {
		t := p.HanTiepNhan
		ack = &t
	}
	if !p.ChuaChotHanXuLy() {
		t := p.HanXuLyXong
		resolve = &t
	}
	return ack, resolve
}

func accountlessReceiptOf(p domain.PhieuPhanAnh) accountlessReceiptOut {
	ack, resolve := accountlessDeadlines(p)
	return accountlessReceiptOut{Code: p.MaTraCuu, Status: string(p.TrangThai), AcknowledgeDue: ack, ResolveDue: resolve}
}

func accountlessLookupOf(p domain.PhieuPhanAnh) accountlessLookupOut {
	ack, resolve := accountlessDeadlines(p)
	out := accountlessLookupOut{Code: p.MaTraCuu, Status: string(p.TrangThai),
		AcknowledgeDue: ack, ResolveDue: resolve, Result: p.KetQuaXuLy}
	// Gated on the status, as phieuCuaToiRaNgoai gates it: staff-written text reaching the public.
	if p.TrangThai == domain.KhongTiepNhan || p.TrangThai == domain.ChuyenCapTren {
		out.Reason = p.LyDoKetThucNhanh
	}
	return out
}

// writeCommuneNotFound is the ONE answer of send and fields for a domain no ACTIVE commune holds —
// unknown, reserved or inactive (ADR 0083 row 2). Nothing is written.
func writeCommuneNotFound(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "commune_not_found",
		"Không tìm thấy xã theo mã QR đã quét. Vui lòng quét lại mã QR của xã.", "")
}

func writeInvalidHost(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, "invalid_host", "Tên miền của xã không hợp lệ.", "")
}

// writeAccountlessNotFound is the ONE 404 of the lookup: malformed or unknown domain, inactive commune,
// no such code, another commune's code, a citizen's or a Zalo account's petition, soft deleted.
// Byte-identical for all of them (rule 4, forbidden #2) — the same body as the session lookup's.
func writeAccountlessNotFound(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy phiếu phản ánh.", "")
}

// communeOf resolves an ALREADY-VALIDATED host.
//
//	ok=false           503 already written: the platform could not be asked. Never "no commune".
//	ok=true, co=false  unknown, reserved or inactive — the caller writes its own "not here".
//	ok=true, co=true   the commune's id; the caller puts it in the context (tenant.Into).
//
// The host is logged on an outage: it is a domain, not a person, and it passed ValidCommuneHost.
func (h *HandlerAccountless) communeOf(w http.ResponseWriter, r *http.Request, host, what string) (tenant.ID, bool, bool) {
	xa, co, err := h.d.Communes.XaTheoHost(r.Context(), host)
	if err != nil {
		h.d.Log.WarnContext(r.Context(), what+": không hỏi được dịch vụ nền tảng", "host", host, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "platform_unavailable",
			"Hệ thống đang bận. Vui lòng thử lại sau ít phút.", "")
		return "", false, false
	}
	if !co || !xa.Active || !xa.ID.Valid() {
		return "", false, true
	}
	return xa.ID, true, true
}

// gateHost counts one request under `l`, keyed (host, network) and `t:<tenant>`-prefixed when the host
// resolved (rule 1, invariant 7). False = the response (429/503/500) is already written.
func (h *HandlerAccountless) gateHost(w http.ResponseWriter, r *http.Request, l *ratelimit.Limiter,
	resolved bool, host string) bool {
	key, err := ratelimit.PublicHostIPKey(r.Context(), resolved, host, httpx.ClientIP(r))
	if err != nil {
		// Unreachable — resolved implies the context carries the commune — and refused if reached.
		h.d.Log.ErrorContext(r.Context(), "không dựng được khoá giới hạn tần suất", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return false
	}
	return ratelimit.Gate(w, r, l, key, h.d.Log)
}

// --- fields --------------------------------------------------------------------------------------

// ListAccountlessFields serves the fields the commune offers on the form, for the commune the QR's
// domain names. GET /api/v1/public-citizen-report-fields?host=
//
// THE SAME BODY as GET /api/v1/my-citizen-report-fields, from the same filtered catalogue: the form is
// the ordinary one (ADR 0083 row 8). Configuration, not anybody's record — no audit entry.
func (h *HandlerAccountless) ListAccountlessFields(w http.ResponseWriter, r *http.Request) {
	hosts := r.URL.Query()["host"]
	if len(hosts) != 1 || !domain.ValidCommuneHost(hosts[0]) {
		writeInvalidHost(w)
		return
	}
	host := hosts[0]
	id, co, ok := h.communeOf(w, r, host, "danh mục lĩnh vực không tài khoản")
	if !ok {
		return
	}
	if co {
		r = r.WithContext(tenant.Into(r.Context(), id))
	}
	if !h.gateHost(w, r, h.d.FieldsLimiter, co, host) {
		return
	}
	if !co {
		writeCommuneNotFound(w)
		return
	}
	offered, err := h.d.Fields.CitizenCatalogue(r.Context())
	if err != nil {
		writeFieldCatalogueError(w, r, h.d.Log.Error, "đọc danh mục lĩnh vực cho phiếu không tài khoản", err)
		return
	}
	out := citizenFieldListOut{Items: make([]citizenFieldOut, 0, len(offered))}
	for _, v := range offered {
		out.Items = append(out.Items, citizenFieldOut{Code: v.Code, Label: v.Label,
			Icon: nullIfEmpty(v.Icon), Tone: nullIfEmpty(v.Tone)})
	}
	vietJSON(w, http.StatusOK, out)
}

// --- send ----------------------------------------------------------------------------------------

type ctxKeyAccountlessBody struct{}

// withBodyCommune is the edge of the send route: it reads the body, refuses what the client does not
// decide, resolves the commune from `host` and puts it in the context — so idem.RequiredAccountless,
// mounted AFTER it, prefixes its key with the commune the SERVER resolved (rule 1, invariant 7).
//
// A MIDDLEWARE AND NOT HANDLER CODE because the commune is in the body, and idem must run between "the
// commune is known" and "the use case runs". Nothing is claimed in Redis for a malformed body, a field
// the client does not decide or an unknown domain.
//
// AN UNKNOWN DOMAIN IS STILL COUNTED (ratelimit.AccountlessSend, keyed by host with no tenant prefix):
// a flood of invented domains is bounded like a flood of sends. A KNOWN commune's send is counted in
// SendAccountless, after idem, so a retry replayed with the same key does not spend the sender's budget.
func (h *HandlerAccountless) withBodyCommune(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in accountlessReportIn
		if !docThan(w, r, &in) {
			return
		}
		if in.toSession().truongKhongPhaiCuaClient() {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"Yêu cầu chứa thông tin do hệ thống tự xác định (người gửi, kênh, mã tra cứu, "+
					"trạng thái hoặc thời hạn), hoặc ghi lĩnh vực bằng `linh_vuc` thay vì `field`. "+
					"Hãy gửi lại chỉ với nội dung phản ánh.", "")
			return
		}
		if in.Lat != nil || in.Lng != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"Phản ánh gửi không qua tài khoản không nhận toạ độ hiện trường. Hãy ghi vị trí vào ô địa chỉ.", "")
			return
		}
		if in.ResidentialUnitID != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"Phản ánh gửi không qua tài khoản không nhận thôn, tổ dân phố. Hãy ghi vị trí vào ô địa chỉ.", "")
			return
		}
		if !domain.ValidCommuneHost(in.Host) {
			writeInvalidHost(w)
			return
		}
		id, co, ok := h.communeOf(w, r, in.Host, "gửi phản ánh không tài khoản")
		if !ok {
			return
		}
		if !co {
			if h.gateHost(w, r, h.d.SendLimiter, false, in.Host) {
				writeCommuneNotFound(w)
			}
			return
		}
		ctx := context.WithValue(tenant.Into(r.Context(), id), ctxKeyAccountlessBody{}, in)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SendAccountless receives one petition with no owner. POST /api/v1/public-citizen-reports
//
// THE SESSION INTAKE'S USE CASE, with app.AccountlessSender: same validation, same deadlines (fixed
// once, here, rule 10 invariant 2), same trail in the same transaction — actor Kind anonymous, IP from
// this socket (rule 6). No photos (ADR 0083 row 5): there is no photo route on this surface, and the
// petition has no owner a photo route could check. No ZNS: there is no recipient.
func (h *HandlerAccountless) SendAccountless(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	in, ok := ctx.Value(ctxKeyAccountlessBody{}).(accountlessReportIn)
	if !ok {
		// A WIRING FAULT: the route was mounted without withBodyCommune.
		h.d.Log.Error("tuyến gửi phản ánh không tài khoản chạy mà không qua withBodyCommune — SAI NỐI DÂY")
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	if !h.gateHost(w, r, h.d.SendLimiter, true, in.Host) {
		return
	}
	field := ""
	if in.Field != nil {
		if field = strings.TrimSpace(*in.Field); field == "" {
			writeFieldNotOffered(w)
			return
		}
	}
	p, err := h.d.Intake.Gui(ctx, app.YeuCauGuiPhanAnh{
		NoiDung: in.Content, DiaChi: in.Address, HoTen: in.Reporter, DienThoai: in.Phone,
		AnDanh: in.Anonymous, Field: field,
	}, app.AccountlessSender(httpx.ClientIP(r)))
	if err != nil {
		writeIntakeError(w, r, h.d.Log, err)
		return
	}
	// Handed to idem BEFORE the sender: a retry with the same key is answered with this code.
	idem.RecordCode(ctx, p.MaTraCuu)
	// The code and the commune, and nothing else (rule 3).
	h.d.Log.Info("đã tiếp nhận phản ánh không tài khoản", "xa", string(tenant.MustFrom(ctx)), "ma_tra_cuu", p.MaTraCuu)
	vietJSON(w, http.StatusCreated, accountlessReceiptOf(p))
}

// replayReceipt is the idem.Replayer of the send: a retry with the same key receives the SAME 201 shape,
// rebuilt from the database (the body is never kept in Redis). Declines, writing nothing, when the
// petition cannot be read — idem then answers its standard body, which still carries the code.
func (h *HandlerAccountless) replayReceipt(w http.ResponseWriter, r *http.Request, status int, code string) bool {
	p, err := h.d.Petitions.AccountlessByCode(r.Context(), code)
	if err != nil {
		return false
	}
	vietJSON(w, status, accountlessReceiptOf(p))
	return true
}

// --- lookup --------------------------------------------------------------------------------------

// LookupAccountless serves the progress of one accountless petition to whoever holds its code.
// GET /api/v1/public-citizen-reports/{maTraCuu}?host=
//
// COUNTED FIRST, per network, before the host is even validated: the bound is on the prober (ADR 0083
// row 9), and an answer that differed by host validity before counting would be free probing.
//
// NO AUDIT ENTRY: nothing is written, nothing personal is returned, and the read cannot leave the commune
// the domain names (rule 6, invariant 7 asks for neither case).
func (h *HandlerAccountless) LookupAccountless(w http.ResponseWriter, r *http.Request) {
	if !ratelimit.Gate(w, r, h.d.LookupLimiter, ratelimit.AccountlessLookupKey(httpx.ClientIP(r)), h.d.Log) {
		return
	}
	hosts := r.URL.Query()["host"]
	ma := r.PathValue("maTraCuu")
	if len(hosts) != 1 || !domain.ValidCommuneHost(hosts[0]) || ma == "" {
		writeAccountlessNotFound(w)
		return
	}
	id, co, ok := h.communeOf(w, r, hosts[0], "tra cứu phiếu không tài khoản")
	if !ok {
		return
	}
	if !co {
		writeAccountlessNotFound(w)
		return
	}
	ctx := tenant.Into(r.Context(), id)
	p, err := h.d.Petitions.AccountlessByCode(ctx, ma)
	if err != nil {
		if errors.Is(err, petstore.ErrPhieuKhongTonTai) {
			writeAccountlessNotFound(w)
			return
		}
		// The code is not logged (rule 3); the commune is.
		h.d.Log.Error("tra cứu phiếu không tài khoản: lỗi hệ thống", "xa", string(id), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, accountlessLookupOf(p))
}
