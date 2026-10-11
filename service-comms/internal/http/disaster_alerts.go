package http

// DISASTER ALERTS — `cảnh báo thiên tai` (migration 0026; owner decision 11/10/2026, ADR 0091 §2): a
// warning ONE commune posts for its residents. Staff post, correct, end early and retire it in a tab of
// "Nội dung Mini App"; residents read the ones in force on the Mini App, without an account.
//
//	GET    /api/v1/disaster-alerts                    content.read    one page, newest start first
//	GET    /api/v1/disaster-alerts/{id}               content.read    one alert
//	POST   /api/v1/disaster-alerts                    content.update  post one
//	PATCH  /api/v1/disaster-alerts/{id}               content.update  correct a live, not-ended alert
//	POST   /api/v1/disaster-alerts/{id}/ending        content.update  end it early
//	DELETE /api/v1/disaster-alerts/{id}               content.update  soft delete, reason mandatory
//	GET    /api/v1/commune-disaster-alerts?host=      Public          the alerts in force, residents
//
// THE NOUN `disaster-alerts` IS ADR 0047 G2's (ubiquitous-language.md §Kênh công dân). The public noun
// `commune-disaster-alerts` follows `commune-external-contacts` / `commune-news`. `ending` is the
// nominalised act (skills/rest-api-design §3) — what a GET of the row's `ended_at`/`ended_by` answers.
//
// THE KEYS ARE THE OWNER'S AND BOTH EXIST: `content.read` / `content.update`,
// service-identity/migrations/0001_init.sql:292-293 (ADR 0091 §2). NO KEY WAS INVENTED (rule 5, 3c).
//
// PERSONAL DATA (rule 3). The contact names and the phones in the two lists may be an officer's own
// (0026): never logged, never in an error, masked in the audit delta (app/disaster_alert.go). Returned
// UNMASKED on both surfaces — the commune typed them for residents to call, and staff edit what they see.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// PublicDisasterAlertsPath is the public collection — exported so cmd/server routes the SAME path to the
// public chain; the route spells it as a literal for tools/apidoc, and a test asserts the two agree.
const PublicDisasterAlertsPath = "/api/v1/commune-disaster-alerts"

// DisasterAlertsReader is the staff READ half. *commsstore.DisasterAlertStore satisfies it.
type DisasterAlertsReader interface {
	List(ctx context.Context, req page.Request) (page.Result[domain.DisasterAlert], error)
	ByID(ctx context.Context, id string) (domain.DisasterAlert, error)
}

// DisasterAlertsWriter is the WRITE half: each call opens a transaction and writes its audit entry in
// it (rule 6, invariant 3). *app.DisasterAlerts satisfies it.
type DisasterAlertsWriter interface {
	Create(ctx context.Context, in app.DisasterAlertInput, actor audit.Actor) (domain.DisasterAlert, error)
	Update(ctx context.Context, id string, p app.DisasterAlertPatch, actor audit.Actor) (domain.DisasterAlert, error)
	End(ctx context.Context, id string, actor audit.Actor) (domain.DisasterAlert, error)
	Delete(ctx context.Context, id, reason string, actor audit.Actor) error
}

// PublicDisasterAlertsReader is the public read. *commsstore.DisasterAlertStore satisfies it.
type PublicDisasterAlertsReader interface {
	InForce(ctx context.Context, now time.Time) ([]domain.DisasterAlert, error)
}

// DisasterAlertDeps is everything the staff routes touch.
type DisasterAlertDeps struct {
	Checker authz.Checker
	Reader  DisasterAlertsReader
	Writer  DisasterAlertsWriter
	Log     *slog.Logger
	// Now is the clock `in_force` is derived with. nil = time.Now.
	Now func() time.Time
}

// PublicDisasterAlertDeps is everything the public route touches — nothing else is reachable from it.
type PublicDisasterAlertDeps struct {
	// Xa is the platform lookup the public news routes use: an outage is a 503, never an empty list
	// that reads as "no warning in this commune".
	Xa     TraXaTheoHost
	Alerts PublicDisasterAlertsReader
	// Limiter is ratelimit.PublicNewsRead — the EXISTING policy (ADR 0091 §2: "giới hạn tần suất như
	// tin công khai") and in production the SAME instance the news and contact routes hold: one budget
	// per (host, client network).
	Limiter *ratelimit.Limiter
	Log     *slog.Logger
	// Now is the clock "in force" is decided with. nil = time.Now.
	Now func() time.Time
}

// disasterAlertOut is one alert on the STAFF surface.
type disasterAlertOut struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Title, Level, Headline, Detail — as typed. Level and Headline are ABSENT when not stated.
	Title    string `json:"title"`
	Level    string `json:"level,omitempty"`
	Headline string `json:"headline,omitempty"`
	Detail   string `json:"detail"`

	EffectiveFrom time.Time  `json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty"`
	// EndedAt / EndedBy — present once staff ended the alert early; EndedBy is a staff business code.
	EndedAt *time.Time `json:"ended_at,omitempty"`
	EndedBy string     `json:"ended_by,omitempty"`
	// InForce is DERIVED from the times at the moment of the reply — never stored.
	InForce bool `json:"in_force"`

	DangerZone        []domain.DangerZonePoint  `json:"danger_zone"`
	EvacuationPoints  []domain.EvacuationPoint  `json:"evacuation_points"`
	EmergencyContacts []domain.EmergencyContact `json:"emergency_contacts"`

	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

func disasterAlertToOut(a domain.DisasterAlert, now time.Time) disasterAlertOut {
	return disasterAlertOut{
		ID: a.ID, Kind: string(a.Kind), Title: a.Title, Level: a.Level, Headline: a.Headline, Detail: a.Detail,
		EffectiveFrom: a.EffectiveFrom.UTC(), EffectiveTo: instantOut(a.EffectiveTo),
		EndedAt: instantOut(a.EndedAt), EndedBy: a.EndedBy, InForce: a.InForce(now),
		DangerZone: nonNilZone(a.DangerZone), EvacuationPoints: nonNilEvac(a.EvacuationPoints),
		EmergencyContacts: nonNilContacts(a.EmergencyContacts),
		CreatedAt:         a.CreatedAt, CreatedBy: a.CreatedBy, UpdatedAt: a.UpdatedAt, UpdatedBy: a.UpdatedBy,
	}
}

// publicDisasterAlertOut is one alert as the Mini App draws it. Every text is PLAIN TEXT
// (domain.VanBanThuanChoDan), like every text on the public surface.
type publicDisasterAlertOut struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Title is the alert's name; Headline is the one-line red banner — the title when staff wrote none,
	// so the banner is never empty.
	Title    string `json:"title"`
	Headline string `json:"headline"`
	Level    string `json:"level,omitempty"`
	Detail   string `json:"detail"`

	EffectiveFrom time.Time  `json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty"`

	// DangerZone is a polygon over the commune picture, x and y in 0..100 — NOT map coordinates.
	DangerZone        []domain.DangerZonePoint  `json:"danger_zone"`
	EvacuationPoints  []domain.EvacuationPoint  `json:"evacuation_points"`
	EmergencyContacts []domain.EmergencyContact `json:"emergency_contacts"`
}

// disasterAlertListOut is a page of the staff list.
type disasterAlertListOut = page.Result[disasterAlertOut]

// publicDisasterAlertListOut wraps the public list; never paginated (bounded by
// commsstore.PublicDisasterAlertCeiling).
type publicDisasterAlertListOut struct {
	Items []publicDisasterAlertOut `json:"items"`
}

// createDisasterAlertIn is the body of POST. Times are RFC 3339 WITH an offset.
type createDisasterAlertIn struct {
	// Kind is one of bao · lu · sat-lo · mua-lon · dich-benh · chay.
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Level    string `json:"level,omitempty"`
	Headline string `json:"headline,omitempty"`
	Detail   string `json:"detail"`
	// EffectiveFrom is required; EffectiveTo, when given, must be AFTER it (absent = until ended).
	EffectiveFrom string `json:"effective_from"`
	EffectiveTo   string `json:"effective_to,omitempty"`
	// DangerZone: none, or at least 3 points (x, y in 0..100), at most 200.
	DangerZone []domain.DangerZonePoint `json:"danger_zone,omitempty"`
	// EvacuationPoints: at most 50; `name` required, `phone` a dial string, `capacity` ≥ 0.
	EvacuationPoints []domain.EvacuationPoint `json:"evacuation_points,omitempty"`
	// EmergencyContacts: at most 50; `role`, `name` and `phone` all required.
	EmergencyContacts []domain.EmergencyContact `json:"emergency_contacts,omitempty"`
}

// updateDisasterAlertIn is the body of PATCH. ABSENT = leave alone. `level` / `headline` / `effective_to`
// "" CLEAR them; a list REPLACES the stored one whole (`[]` clears it).
type updateDisasterAlertIn struct {
	Kind              *string                    `json:"kind,omitempty"`
	Title             *string                    `json:"title,omitempty"`
	Level             *string                    `json:"level,omitempty"`
	Headline          *string                    `json:"headline,omitempty"`
	Detail            *string                    `json:"detail,omitempty"`
	EffectiveFrom     *string                    `json:"effective_from,omitempty"`
	EffectiveTo       *string                    `json:"effective_to,omitempty"`
	DangerZone        *[]domain.DangerZonePoint  `json:"danger_zone,omitempty"`
	EvacuationPoints  *[]domain.EvacuationPoint  `json:"evacuation_points,omitempty"`
	EmergencyContacts *[]domain.EmergencyContact `json:"emergency_contacts,omitempty"`
}

// deleteDisasterAlertIn — the reason is mandatory and travels in the body, never the query string.
type deleteDisasterAlertIn struct {
	Reason string `json:"reason"`
}

type disasterAlertHandler struct {
	d DisasterAlertDeps
}

// RegisterDisasterAlerts mounts the six STAFF routes. Refuses incomplete wiring at construction.
func RegisterDisasterAlerts(mux *http.ServeMux, d DisasterAlertDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — tuyến cảnh báo thiên tai sẽ không kiểm được quyền")
	case d.Reader == nil:
		panic("comms/http: thiếu kho cảnh báo thiên tai — GET /api/v1/disaster-alerts sẽ panic khi có người gọi")
	case d.Writer == nil:
		panic("comms/http: thiếu use case ghi cảnh báo thiên tai — POST/PATCH/DELETE sẽ panic khi có người gọi")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	h := &disasterAlertHandler{d: d}

	// Cursor-paginated like every staff list (rest-api-design REQUIRED #7): alerts accumulate season
	// after season and are never hard deleted. Every state is listed; `in_force` says which are shown.
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Cảnh báo thiên tai của xã — mọi trạng thái (sắp tới, đang hiệu lực, đã qua, đã kết thúc), mới bắt đầu nhất trước, phân trang theo con trỏ
	// @screen   11-noi-dung-mini-app
	// @reply    200 disasterAlertListOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/disaster-alerts",
		authz.RequirePermission(d.Checker, "content.read")(
			http.HandlerFunc(h.ListDisasterAlerts)))

	// 404 is ONE answer for no such id, another commune's and a deleted one.
	//
	// @summary  Một cảnh báo thiên tai của xã, đủ vùng nguy hiểm, điểm sơ tán và số khẩn cấp
	// @screen   11-noi-dung-mini-app
	// @reply    200 disasterAlertOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/disaster-alerts/{id}",
		authz.RequirePermission(d.Checker, "content.read")(
			http.HandlerFunc(h.GetDisasterAlert)))

	// idem.Required(DongKhiHong) — the content-item choice (task brief): there is NO unique key under this
	// write, so a double-submitted form would show residents the same warning twice, and the key is the
	// ONLY layer. THE COST, STATED: during a Redis outage no alert can be posted — 503 — on a route whose
	// purpose is an emergency. MoKhiHong (serve on a cache outage, risk a duplicate) is the alternative a
	// reviewer may prefer; it is one word here.
	//
	// The alert is public the moment `effective_from` is reached — there is no publish flag (0026).
	// 400 is `invalid_request` with the sentence naming the field (and, for a list, the item's position).
	//
	// @summary  Đăng một cảnh báo thiên tai — hiện trên Zalo Mini App từ thời điểm bắt đầu hiệu lực (không đẩy ZNS)
	// @screen   11-noi-dung-mini-app
	// @request  createDisasterAlertIn
	// @reply    201 disasterAlertOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/disaster-alerts",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.CreateDisasterAlert))))

	// PATCH: every field optional. 409 `alert_ended` once the alert was ended early — it is then the
	// record of what residents were told (0026); post a new one instead.
	//
	// idem.KhongCan: app.DisasterAlerts.Update writes and audits nothing when nothing moved.
	//
	// @summary  Sửa một cảnh báo thiên tai đang còn (chưa kết thúc) — nội dung, thời gian, vùng nguy hiểm, điểm sơ tán, số khẩn cấp
	// @screen   11-noi-dung-mini-app
	// @request  updateDisasterAlertIn
	// @reply    200 disasterAlertOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error alert_ended
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/disaster-alerts/{id}",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; use case không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.UpdateDisasterAlert))))

	// Ending early: `ended_at` / `ended_by` set in one statement; the alert leaves the Mini App at once.
	// idem.KhongCan — a second request is 409 `alert_ended` and cannot overwrite who ended it (the locked
	// read and the UPDATE both carry `ended_at IS NULL`).
	//
	// @summary  Kết thúc sớm một cảnh báo thiên tai — gỡ khỏi Zalo Mini App ngay, ghi ai kết thúc và lúc nào
	// @screen   11-noi-dung-mini-app
	// @reply    200 disasterAlertOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    409 httpx.Error alert_ended
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/disaster-alerts/{id}/ending",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("kết thúc lần thứ hai trả 409: lượt đọc khoá dòng và câu UPDATE đều mang `ended_at IS NULL`, nên không ghi đè được người kết thúc")(
				http.HandlerFunc(h.EndDisasterAlert))))

	// Soft delete with a mandatory reason, in the body; 204. Allowed on an ended alert too.
	//
	// @summary  Xoá mềm một cảnh báo thiên tai, kèm lý do bắt buộc — gỡ khỏi Zalo Mini App trong cùng lần ghi
	// @screen   11-noi-dung-mini-app
	// @request  deleteDisasterAlertIn
	// @reply    204 -
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/disaster-alerts/{id}",
		authz.RequirePermission(d.Checker, "content.update")(
			idem.KhongCan("xoá một cảnh báo đã xoá trả 404: lượt đọc khoá dòng và câu UPDATE đều mang `AND deleted_at IS NULL`, nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.DeleteDisasterAlert))))
}

// RegisterPublicDisasterAlerts mounts the PUBLIC route onto the public mux.
func RegisterPublicDisasterAlerts(mux *http.ServeMux, d PublicDisasterAlertDeps) {
	switch {
	case d.Xa == nil:
		panic("comms/http: thiếu kho tra xã theo tên miền — tuyến cảnh báo thiên tai công khai sẽ panic khi có người gọi")
	case d.Alerts == nil:
		panic("comms/http: thiếu kho cảnh báo thiên tai — tuyến công khai sẽ panic khi có người gọi")
	case d.Limiter == nil:
		// Rule 13 invariant 7: an unauthenticated route without its rate limit is refused at startup.
		panic("comms/http: thiếu bộ giới hạn tần suất tuyến công khai (ratelimit.PublicNewsRead)")
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	// THE PUBLIC NEWS HANDLER's host resolution (xaTheoHost): host check, platform lookup, `t:<tenant_id>`
	// limiter key, 429, 503 — the same path the news and contact routes take, not a copy.
	h := newHandlerCongKhai(DepsCongKhai{Xa: d.Xa, Limiter: d.Limiter, Log: d.Log})
	p := &publicDisasterAlerts{h: h, alerts: d.Alerts, now: d.Now}

	// PUBLIC (owner, 11/10/2026 — ADR 0091 §2 opens this unauthenticated route, rule 13 stop condition):
	// a warning is public information; a resident opening the Mini App during a storm has no account.
	//
	// RATE LIMIT — ratelimit.PublicNewsRead, the existing policy: 120 per minute per (host, client
	// network), shared with the news and contact reads. Over it: 429 + Retry-After. A Redis outage SERVES.
	// NO SERVER-SIDE CACHE: comms has no content cache to copy; the host lookup is cached
	// (tenant.CachedDirectory) and the read is one indexed statement over a handful of rows.
	//
	// NO idem.* DECLARATION: a GET changes no state.
	//
	// @summary  Cảnh báo thiên tai ĐANG HIỆU LỰC của xã theo tên miền — cho màn cảnh báo và băng đỏ trang chủ Zalo Mini App
	// @consumer citizen-app
	//
	// 200 is the alerts in force now — not ended, not deleted, started, not past their end — newest start
	// first, never paginated (at most 20). `{"items":[]}` for a commune with none AND for a domain no
	// active commune holds — identical bytes. 400 is `host` missing, repeated or malformed. 500 a store
	// failure. 503 the platform registry unreachable. 429 the rate limit.
	//
	// @reply    200 publicDisasterAlertListOut
	// @reply    400 httpx.Error
	// @reply    429 httpx.Error rate_limited
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error
	mux.Handle("GET /api/v1/commune-disaster-alerts",
		authz.Public("cảnh báo thiên tai đang hiệu lực mà xã đăng cho người dân, hiện trên Zalo Mini App (chủ dự án mở tuyến này 11/10/2026, ADR 0091 §2): người dân đọc không cần tài khoản; chỉ trả cảnh báo chưa xoá, chưa kết thúc, đang trong khung hiệu lực của đúng xã mà nền tảng phân giải từ tên miền; giới hạn 120 lần/phút theo tên miền và mạng của máy gọi")(
			http.HandlerFunc(p.PublicDisasterAlerts)))
}

// ListDisasterAlerts — GET /api/v1/disaster-alerts
func (h *disasterAlertHandler) ListDisasterAlerts(w http.ResponseWriter, r *http.Request) {
	req, err := page.Parse(r.URL.Query(), commsstore.SortDisasterAlerts)
	if err != nil {
		status, code, msg := page.HTTPError(err)
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	// Scoped in the store: QueryPage reads through db.For(ctx).
	res, err := h.d.Reader.List(r.Context(), req)
	if err != nil {
		h.writeError(w, r, "đọc", err)
		return
	}
	now := h.d.Now()
	out := disasterAlertListOut{Items: make([]disasterAlertOut, 0, len(res.Items)),
		NextCursor: res.NextCursor, HasMore: res.HasMore}
	for _, a := range res.Items {
		out.Items = append(out.Items, disasterAlertToOut(a, now))
	}
	vietJSON(w, http.StatusOK, out)
}

// GetDisasterAlert — GET /api/v1/disaster-alerts/{id}
func (h *disasterAlertHandler) GetDisasterAlert(w http.ResponseWriter, r *http.Request) {
	// Scoped in the store: db.For(ctx).Query.
	a, err := h.d.Reader.ByID(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeError(w, r, "đọc", err)
		return
	}
	vietJSON(w, http.StatusOK, disasterAlertToOut(a, h.d.Now()))
}

// CreateDisasterAlert — POST /api/v1/disaster-alerts
func (h *disasterAlertHandler) CreateDisasterAlert(w http.ResponseWriter, r *http.Request) {
	var in createDisasterAlertIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	from, err := domain.ParseEventInstant("effective_from", in.EffectiveFrom)
	if err != nil {
		h.writeError(w, r, "đăng", err)
		return
	}
	to, err := domain.ParseEventInstant("effective_to", in.EffectiveTo)
	if err != nil {
		h.writeError(w, r, "đăng", err)
		return
	}
	// Scoped in the use case: app.DisasterAlerts opens db.For(ctx).Tx.
	row, err := h.d.Writer.Create(r.Context(), app.DisasterAlertInput{
		Kind: in.Kind, Title: in.Title, Level: in.Level, Headline: in.Headline, Detail: in.Detail,
		EffectiveFrom: from, EffectiveTo: to, DangerZone: in.DangerZone, EvacuationPoints: in.EvacuationPoints,
		EmergencyContacts: in.EmergencyContacts,
	}, actor)
	if err != nil {
		h.writeError(w, r, "đăng", err)
		return
	}
	// The row's id, not the body: the body (phone numbers) would go into Redis, a cache.
	idem.RecordCode(r.Context(), row.ID)
	vietJSON(w, http.StatusCreated, disasterAlertToOut(row, h.d.Now()))
}

// UpdateDisasterAlert — PATCH /api/v1/disaster-alerts/{id}
func (h *disasterAlertHandler) UpdateDisasterAlert(w http.ResponseWriter, r *http.Request) {
	var in updateDisasterAlertIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	p := app.DisasterAlertPatch{
		Kind: in.Kind, Title: in.Title, Level: in.Level, Headline: in.Headline, Detail: in.Detail,
		DangerZone: in.DangerZone, EvacuationPoints: in.EvacuationPoints, EmergencyContacts: in.EmergencyContacts,
	}
	if in.EffectiveFrom != nil {
		// "" is NOT "clear" here: the start is required; ParseEventInstant's zero time is refused by the
		// domain as a missing `effective_from`.
		t, err := domain.ParseEventInstant("effective_from", *in.EffectiveFrom)
		if err != nil {
			h.writeError(w, r, "sửa", err)
			return
		}
		p.EffectiveFrom = &t
	}
	if in.EffectiveTo != nil {
		t, err := domain.ParseEventInstant("effective_to", *in.EffectiveTo) // "" → zero → open-ended
		if err != nil {
			h.writeError(w, r, "sửa", err)
			return
		}
		p.EffectiveTo = &t
	}
	// Scoped in the use case: app.DisasterAlerts opens db.For(ctx).Tx.
	row, err := h.d.Writer.Update(r.Context(), r.PathValue("id"), p, actor)
	if err != nil {
		h.writeError(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, disasterAlertToOut(row, h.d.Now()))
}

// EndDisasterAlert — POST /api/v1/disaster-alerts/{id}/ending
func (h *disasterAlertHandler) EndDisasterAlert(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.DisasterAlerts opens db.For(ctx).Tx.
	row, err := h.d.Writer.End(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		h.writeError(w, r, "kết thúc", err)
		return
	}
	vietJSON(w, http.StatusOK, disasterAlertToOut(row, h.d.Now()))
}

// DeleteDisasterAlert — DELETE /api/v1/disaster-alerts/{id}. Soft delete; 204.
func (h *disasterAlertHandler) DeleteDisasterAlert(w http.ResponseWriter, r *http.Request) {
	var in deleteDisasterAlertIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.DisasterAlerts opens db.For(ctx).Tx.
	if err := h.d.Writer.Delete(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeError(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// missingPrincipal — a write reached with no principal carrying a business code: a wiring fault, and
// rule 6 does not permit a write whose trail cannot name who made it. No fallback to the internal id.
func (h *disasterAlertHandler) missingPrincipal(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến ghi cảnh báo thiên tai chạy mà không có chủ thể mang mã cán bộ",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// disasterAlertRefusals are the 400s. The domain sentence is returned without its package prefix: it
// names the field and, for a list, the item's position — never a value.
var disasterAlertRefusals = []error{
	domain.ErrDisasterKindInvalid, domain.ErrDisasterTitleEmpty, domain.ErrDisasterTitleTooLong,
	domain.ErrDisasterLevelInvalid, domain.ErrDisasterHeadlineInvalid, domain.ErrDisasterDetailEmpty,
	domain.ErrDisasterDetailInvalid, domain.ErrDisasterFromMissing, domain.ErrDisasterWindowInverted,
	domain.ErrDisasterDangerZone, domain.ErrDisasterEvacuationPoints, domain.ErrDisasterContacts,
	domain.ErrEventTimeInvalid, domain.ErrThieuLyDoXoa, domain.ErrLyDoXoaQuaDai,
}

// writeError maps a failure onto a status — never a default 400. The wrapped error is logged, never
// sent (rule 3, forbidden #3).
func (h *disasterAlertHandler) writeError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, commsstore.ErrDisasterAlertNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy cảnh báo này.", "")
		return
	case errors.Is(err, domain.ErrDisasterEnded):
		httpx.WriteError(w, http.StatusConflict, "alert_ended",
			"Cảnh báo đã kết thúc nên không sửa hay kết thúc lại được. Hãy đăng một cảnh báo mới.", "")
		return
	}
	for _, refused := range disasterAlertRefusals {
		if errors.Is(err, refused) {
			h.d.Log.Info("cảnh báo thiên tai: từ chối "+op, "xa", string(tenant.MustFrom(r.Context())), "loai", refused.Error())
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", refusalSentence(err), "")
			return
		}
	}
	h.d.Log.Error("cảnh báo thiên tai: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// refusalSentence strips the package prefixes the domain sentinels carry ("disaster_alert: ",
// "noi_dung_mini_app: "), so the client reads the sentence alone.
func refusalSentence(err error) string {
	s := err.Error()
	for _, prefix := range []string{"disaster_alert: ", "noi_dung_mini_app: ", "xoa_mem: "} {
		s = strings.TrimPrefix(s, prefix)
	}
	return s
}

// publicDisasterAlerts serves the public route through the news surface's host resolution.
type publicDisasterAlerts struct {
	h      *HandlerCongKhai
	alerts PublicDisasterAlertsReader
	now    func() time.Time
}

// PublicDisasterAlerts — GET /api/v1/commune-disaster-alerts?host=
//
// THE SAME NEGATIVES AS GET /api/v1/commune-external-contacts: `host` validated before the platform is
// asked; an unknown, reserved or inactive domain is `{"items":[]}` with 200, byte-identical to a commune
// with no alert; an unreachable platform is 503; past the limit 429. NO AUDIT ENTRY: nothing is written.
func (p *publicDisasterAlerts) PublicDisasterAlerts(w http.ResponseWriter, r *http.Request) {
	hosts := r.URL.Query()["host"]
	if len(hosts) != 1 || !domain.HopLeTenMienXa(hosts[0]) {
		viet400Host(w)
		return
	}
	xa, found, ok := p.h.xaTheoHost(w, r, hosts[0], "cảnh báo thiên tai của xã")
	if !ok {
		return
	}
	if !found {
		vietJSON(w, http.StatusOK, publicDisasterAlertListOut{Items: []publicDisasterAlertOut{}})
		return
	}

	ctx := tenant.Into(r.Context(), xa.ID)
	// Scoped in the store: db.For(ctx).Query, live, not ended, inside the window.
	list, err := p.alerts.InForce(ctx, p.now())
	if err != nil {
		if !errors.Is(err, commsstore.ErrTooManyAlertsInForce) {
			p.h.loi500(ctx, w, "cảnh báo thiên tai của xã", err)
			return
		}
		// SERVE THE NEWEST, LOG THE REST: an empty banner during a disaster is the worst answer.
		p.h.d.Log.ErrorContext(ctx, "cảnh báo thiên tai đang hiệu lực vượt trần — chỉ trả các cảnh báo mới nhất",
			"xa", string(xa.ID), "tran", commsstore.PublicDisasterAlertCeiling)
	}
	out := publicDisasterAlertListOut{Items: make([]publicDisasterAlertOut, 0, len(list))}
	for _, a := range list {
		headline := a.Headline
		if headline == "" {
			headline = a.Title
		}
		out.Items = append(out.Items, publicDisasterAlertOut{
			ID: a.ID, Kind: string(a.Kind),
			Title:         domain.VanBanThuanChoDan(a.Title),
			Headline:      domain.VanBanThuanChoDan(headline),
			Level:         domain.VanBanThuanChoDan(a.Level),
			Detail:        domain.VanBanThuanChoDan(a.Detail),
			EffectiveFrom: a.EffectiveFrom.UTC(), EffectiveTo: instantOut(a.EffectiveTo),
			DangerZone:        nonNilZone(a.DangerZone),
			EvacuationPoints:  plainEvacuation(a.EvacuationPoints),
			EmergencyContacts: plainContacts(a.EmergencyContacts),
		})
	}
	vietJSON(w, http.StatusOK, out)
}

func plainEvacuation(pts []domain.EvacuationPoint) []domain.EvacuationPoint {
	out := make([]domain.EvacuationPoint, 0, len(pts))
	for _, p := range pts {
		p.Name, p.Address = domain.VanBanThuanChoDan(p.Name), domain.VanBanThuanChoDan(p.Address)
		out = append(out, p) // the phone's character set is closed (dial characters only)
	}
	return out
}

func plainContacts(cs []domain.EmergencyContact) []domain.EmergencyContact {
	out := make([]domain.EmergencyContact, 0, len(cs))
	for _, c := range cs {
		c.Role, c.Name = domain.VanBanThuanChoDan(c.Role), domain.VanBanThuanChoDan(c.Name)
		out = append(out, c)
	}
	return out
}

func nonNilZone(z []domain.DangerZonePoint) []domain.DangerZonePoint {
	if z == nil {
		return []domain.DangerZonePoint{}
	}
	return z
}

func nonNilEvac(p []domain.EvacuationPoint) []domain.EvacuationPoint {
	if p == nil {
		return []domain.EvacuationPoint{}
	}
	return p
}

func nonNilContacts(c []domain.EmergencyContact) []domain.EmergencyContact {
	if c == nil {
		return []domain.EmergencyContact{}
	}
	return c
}
