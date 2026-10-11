package http

// THE CITIZEN'S "NHẬN THÔNG BÁO" SWITCH (migration 0027; owner decision 11/10/2026, ADR 0091 §4) — the
// FIRST CITIZEN route of this service, on its own mux behind the citizen chain cmd/server builds
// (service-petitions/cmd/server/main.go §dungBien is the template).
//
//	GET  /api/v1/my-notification-preferences   CitizenOnly + XaTuPhien   the caller's own switch
//	PUT  /api/v1/my-notification-preferences   CitizenOnly + XaTuPhien   set it: {"zalo_enabled": bool}
//
// THE NOUN — `my-` + the resource, kb/00-foundation/ubiquitous-language.md §Tiền tố `my-` — IS THE
// BUILDER'S, NOT YET THE USER'S: that table lists CitizenNotificationPreference with the URL resource
// "chưa chốt". `notification-preferences` drops the `Citizen` the TYPE carries because `my-` already says
// whose (the type needed it only to stay clear of the staff bell, `notifications`). Renaming is free
// until a Mini App build calls it.
//
// IDENTITY AND COMMUNE COME FROM THE SESSION AND NOTHING ELSE (rule 4, invariant 2; ADR 0022): there is
// no citizen id, phone or commune in the path, the query or the body. XaTuPhien, not the Zalo-account
// class: a ZNS message goes to a VERIFIED citizen's number (ADR 0080 decision 4 — an unverified petition
// is never notified), so a session without a verified phone has no switch to flip and gets 403
// `chua_xac_thuc_so`, the same single cause the petitions citizen routes give.
//
// THE SWITCH CHANGES NO MESSAGE TODAY. The ZNS send path is not wired (ADR 0041 open #4); ADR 0091
// §Cái giá says the interface must not claim it already works. This file only records the choice.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// MyNotificationPreferencesPath is the citizen resource. Exported because cmd/server must route the SAME
// path to the citizen chain on the outer mux; the routes below spell it as a LITERAL because tools/apidoc
// reads the pattern from the source, and a test asserts the two agree.
const MyNotificationPreferencesPath = "/api/v1/my-notification-preferences"

// CitizenNotificationPreferenceUseCase is the read and the audited write. *app.CitizenNotificationPreferences
// satisfies it.
type CitizenNotificationPreferenceUseCase interface {
	Get(ctx context.Context, actor audit.Actor) (domain.CitizenNotificationPreference, error)
	Set(ctx context.Context, zaloEnabled bool, actor audit.Actor) (domain.CitizenNotificationPreference, error)
}

// CitizenDeps is everything the citizen routes of this service touch — nothing on it reaches a staff
// store (rule 4, invariant 5: separate routers, separate Deps).
type CitizenDeps struct {
	Preferences CitizenNotificationPreferenceUseCase
	Log         *slog.Logger
}

// notificationPreferenceOut is the caller's own switch.
type notificationPreferenceOut struct {
	// ZaloEnabled — whether this citizen receives Zalo messages about their own petitions in this
	// commune. TRUE until the citizen turns it off: a citizen who never touched the switch receives them.
	// Even when false, `khong-tiep-nhan` and `chuyen-cap-tren` are still sent (ADR 0091 §4).
	ZaloEnabled bool `json:"zalo_enabled"`
}

// notificationPreferenceIn is the body of PUT. A pointer so an absent key is a 400 rather than a silent
// "off" — a false nobody chose would stop a citizen's messages.
type notificationPreferenceIn struct {
	ZaloEnabled *bool `json:"zalo_enabled"`
}

type citizenHandler struct {
	d CitizenDeps
}

// RegisterCitizen mounts the citizen routes onto their OWN mux — the one behind the citizen chain.
// Handed the staff mux it compiles and 404s every citizen (TenantMiddleware has no commune for the
// reserved API host); cmd/server/main_test.go drives the real chain.
func RegisterCitizen(mux *http.ServeMux, d CitizenDeps) {
	if d.Preferences == nil {
		panic("comms/http: thiếu use case lựa chọn nhận thông báo — " +
			"/api/v1/my-notification-preferences sẽ panic khi có người dân mở màn Cá nhân")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &citizenHandler{d: d}

	// NO idem.* DECLARATION: a GET changes no state. No audit entry: the citizen reading their own
	// setting. NO @screen on either route: docs/ui-ux/ has no section for the Mini App's Cá nhân screen.
	//
	// @summary  Lựa chọn nhận thông báo Zalo CỦA CHÍNH công dân trong xã của phiên — chưa từng đổi thì là đang nhận
	// 401 folds the three situations every citizen route folds (ADR 0022): no bearer token · a token
	// that is not usable · a session that has not chosen a commune. 403 `chua_xac_thuc_so` only — a
	// session with no verified phone (never a permission refusal: citizens hold none, rule 5 inv 6).
	//
	// @reply    200 notificationPreferenceOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error chua_xac_thuc_so
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/my-notification-preferences",
		authz.CitizenOnly()(
			httpx.XaTuPhien()(
				http.HandlerFunc(h.GetNotificationPreference))))

	// PUT AND NOT PATCH: the resource is one boolean, replaced whole.
	//
	// idem.KhongCan: the use case writes and audits NOTHING when the stored value already equals the one
	// sent, so the same PUT twice leaves one state and one entry (a second tap is a no-op, not a second
	// trail line).
	//
	// @summary  Công dân bật / tắt nhận thông báo Zalo về phản ánh của mình — tắt vẫn nhận tin không tiếp nhận và chuyển cấp trên; mỗi lần đổi có vết kiểm toán
	// @request  notificationPreferenceIn
	// 400 is a body that is not JSON or has no boolean `zalo_enabled`.
	//
	// @reply    200 notificationPreferenceOut
	// @reply    400 httpx.Error
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error chua_xac_thuc_so
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/my-notification-preferences",
		authz.CitizenOnly()(
			httpx.XaTuPhien()(
				idem.KhongCan("PUT ghi đè một giá trị đã biết; use case không ghi và không ghi vết khi giá trị gửi lên trùng giá trị đang lưu, nên lần gửi thứ hai để lại đúng một trạng thái và một vết")(
					http.HandlerFunc(h.PutNotificationPreference)))))
}

// citizenActor is the audit actor of a citizen write — the opaque citizen id from the SESSION, Kind
// "citizen" (service-petitions/internal/http/gui_phan_anh.go congDanThucHien, the same shape and the
// same reason: a citizen has no staff code, so `.Ma` would be empty and the write refused). The Kind
// check is what makes `.ID` correct here.
func citizenActor(r *http.Request) (audit.Actor, bool) {
	p, ok := authz.From(r.Context())
	if !ok || p.Kind != "citizen" || p.ID == "" {
		return audit.Actor{}, false
	}
	return audit.Actor{ID: p.ID, Kind: p.Kind, IP: httpx.ClientIP(r)}, true
}

// GetNotificationPreference — GET /api/v1/my-notification-preferences
func (h *citizenHandler) GetNotificationPreference(w http.ResponseWriter, r *http.Request) {
	actor, ok := citizenActor(r)
	if !ok {
		h.missingCitizen(w, r)
		return
	}
	// Scoped in the store: db.For(ctx) — the commune XaTuPhien put on the context from the session.
	p, err := h.d.Preferences.Get(r.Context(), actor)
	if err != nil {
		h.fail(w, r, "đọc", err)
		return
	}
	vietJSON(w, http.StatusOK, notificationPreferenceOut{ZaloEnabled: p.ZaloEnabled})
}

// PutNotificationPreference — PUT /api/v1/my-notification-preferences
func (h *citizenHandler) PutNotificationPreference(w http.ResponseWriter, r *http.Request) {
	var in notificationPreferenceIn
	if !docThan(w, r, &in) {
		return
	}
	if in.ZaloEnabled == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thiếu `zalo_enabled` (true hoặc false).", "")
		return
	}
	actor, ok := citizenActor(r)
	if !ok {
		h.missingCitizen(w, r)
		return
	}
	// Scoped in the use case: app.CitizenNotificationPreferences opens db.For(ctx).Tx.
	p, err := h.d.Preferences.Set(r.Context(), *in.ZaloEnabled, actor)
	if err != nil {
		h.fail(w, r, "ghi", err)
		return
	}
	vietJSON(w, http.StatusOK, notificationPreferenceOut{ZaloEnabled: p.ZaloEnabled})
}

// missingCitizen — a route behind CitizenOnly + XaTuPhien reached without a verified citizen principal:
// a wiring fault. Nothing is read or written in someone else's name. The citizen id is NOT logged.
func (h *citizenHandler) missingCitizen(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến công dân lựa chọn nhận thông báo chạy mà không có chủ thể công dân — SAI CẤU HÌNH ROUTE",
		"duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

func (h *citizenHandler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	if errors.Is(err, app.ErrNotACitizen) {
		h.missingCitizen(w, r)
		return
	}
	// The wrapped error names the commune and the store failure — never the citizen (rule 3).
	h.d.Log.Error("lựa chọn nhận thông báo: "+op+" lỗi hệ thống",
		"xa", string(tenant.MustFrom(r.Context())), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}
