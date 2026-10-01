package http

// The handlers behind §3's `Cấu hình` modal and run history of the portal sync
// (docs/ui-ux/11-noi-dung-mini-app.md §3; ADR 0067 §2; migration 0013). The routes and their
// permission declarations are in routes_portal_sync.go.
//
// THE KEY NEVER LEAVES THIS SERVICE. portalSyncSettingsOut has no field that could hold it — only
// `api_key_set`. portalSyncSettingsIn carries it inward, and the handler turns it into a secret.Secret
// at once, so nothing that formats or logs the request value can print it.
//
// THE COMMUNE IS NEVER HANDLED HERE. Every call passes the request context; the store behind the use
// cases reaches the database only through db.For(ctx) (rule 1, invariants 4 and 5).
//
// NOTHING A PORTAL SAYS REACHES A CLIENT: a failed call is answered 502 with an error CLASS turned into
// a fixed sentence (portalCallFailures) — never the portal's body, never the URL, never the key.

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// portalSyncSettingsOut is the commune's portal sync configuration as the screen reads it.
type portalSyncSettingsOut struct {
	// Configured false: the commune never saved; the other fields are the defaults.
	Configured bool `json:"configured"`
	// EncryptionConfigured false: the platform has no SECRET_ENCRYPTION_KEYS; saving answers 503.
	EncryptionConfigured bool `json:"encryption_configured"`
	// Provider is the API dialect; one value today (`cttdt-danang`). Not settable.
	Provider string `json:"provider"`
	APIURL   string `json:"api_url"`
	// APIKeySet is the ONLY thing any response says about the key (ADR 0009 rule 6).
	APIKeySet bool `json:"api_key_set"`
	// PublishMode is `cho-duyet` (default) or `dang-thang`.
	PublishMode string `json:"publish_mode"`
	// IntervalHours 0..24; 0 = manual only.
	IntervalHours    int  `json:"interval_hours"`
	WindowDays       int  `json:"window_days"`
	MaxItemsPerRun   int  `json:"max_items_per_run"`
	KeepSourceCredit bool `json:"keep_source_credit"`
	IsEnabled        bool `json:"is_enabled"`
	// LastRunAt is when the last run STARTED; null when never. The run history says how it ended.
	LastRunAt *time.Time `json:"last_run_at"`
	// UpdatedBy is the staff business code of the last save, "" when never saved.
	UpdatedBy string `json:"updated_by"`
}

func portalSyncSettingsToOut(v app.PortalSyncSettingsView) portalSyncSettingsOut {
	s := v.Settings
	return portalSyncSettingsOut{
		Configured: v.Configured, EncryptionConfigured: v.EncryptionConfigured, Provider: s.Provider,
		APIURL: s.APIURL, APIKeySet: s.APIKeySet, PublishMode: s.PublishMode, IntervalHours: s.IntervalHours,
		WindowDays: s.WindowDays, MaxItemsPerRun: s.MaxItemsPerRun, KeepSourceCredit: s.KeepSourceCredit,
		IsEnabled: s.IsEnabled, LastRunAt: instantOut(s.LastRunAt), UpdatedBy: s.UpdatedBy,
	}
}

// portalSyncSettingsIn is the body of PUT — the whole form.
//
// `api_key` IS WRITE-ONLY. Omitted or "" means KEEP the stored key — except on the first save, and when
// `api_url` changed: both are 422 (`api_key_required`, `api_key_required_for_new_url`). An omitted
// number or flag keeps the stored value (the default on a first save).
type portalSyncSettingsIn struct {
	APIURL           string `json:"api_url"`
	APIKey           string `json:"api_key,omitempty"`
	PublishMode      string `json:"publish_mode,omitempty"`
	IntervalHours    *int   `json:"interval_hours,omitempty"`
	WindowDays       *int   `json:"window_days,omitempty"`
	MaxItemsPerRun   *int   `json:"max_items_per_run,omitempty"`
	KeepSourceCredit *bool  `json:"keep_source_credit,omitempty"`
	IsEnabled        *bool  `json:"is_enabled,omitempty"`
}

// portalCategoryNodeOut is one category of the portal's LIVE tree, with the commune's stored choice.
type portalCategoryNodeOut struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	ParentID   string `json:"parent_id"`
	ParentName string `json:"parent_name"`
	IsSelected bool   `json:"is_selected"`
	// TargetKind is `tin-tuc` · `su-kien` · `thong-bao`, "" when never selected.
	TargetKind string `json:"target_kind"`
}

// portalStoredCategoryOut is one stored selection row.
type portalStoredCategoryOut struct {
	ID         string `json:"id"`
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	TargetKind string `json:"target_kind"`
	IsSelected bool   `json:"is_selected"`
}

// portalCategoryTreeOut — `items` asked of the portal NOW (never stored); `missing` the categories the
// commune still has selected that the portal no longer lists.
type portalCategoryTreeOut struct {
	Items   []portalCategoryNodeOut   `json:"items"`
	Missing []portalStoredCategoryOut `json:"missing"`
}

// portalCategoryIn is one entry of PUT categories.
type portalCategoryIn struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	TargetKind string `json:"target_kind"`
	IsSelected bool   `json:"is_selected"`
}

// portalCategoriesIn is the body of PUT categories: what the modal shows, ticked or not.
type portalCategoriesIn struct {
	Categories []portalCategoryIn `json:"categories"`
}

// portalCategoriesOut is every stored selection row after the save.
type portalCategoriesOut struct {
	Items []portalStoredCategoryOut `json:"items"`
}

// portalRunErrorOut is one entry of a run's error summary: a category and an error CLASS, no article.
type portalRunErrorOut struct {
	CategoryExternalID string `json:"category_external_id"`
	CategoryName       string `json:"category_name"`
	Error              string `json:"error"`
	Count              int    `json:"count"`
}

// portalRunOut is one run. Counts are 0, `outcome` "" and `finished_at` null while it runs.
type portalRunOut struct {
	ID string `json:"id"`
	// TriggerKind is `theo-lich` (the scheduler) or `chay-tay` (`⟳ Đồng bộ ngay`).
	TriggerKind string `json:"trigger_kind"`
	// Actor is `system` or the staff business code who pressed the button.
	Actor     string     `json:"actor"`
	StartedAt *time.Time `json:"started_at"`
	// FinishedAt is null while the run is in progress.
	FinishedAt *time.Time `json:"finished_at"`
	// Outcome is `thanh-cong` · `mot-phan` · `that-bai`, "" while running.
	Outcome              string              `json:"outcome"`
	FetchedCount         int                 `json:"fetched_count"`
	ImportedCount        int                 `json:"imported_count"`
	SkippedExistingCount int                 `json:"skipped_existing_count"`
	SkippedDeletedCount  int                 `json:"skipped_deleted_count"`
	FailedCount          int                 `json:"failed_count"`
	ErrorSummary         []portalRunErrorOut `json:"error_summary"`
}

func portalRunToOut(r domain.PortalSyncRun) portalRunOut {
	out := portalRunOut{
		ID: r.ID, TriggerKind: r.TriggerKind, Actor: r.Actor, StartedAt: instantOut(r.StartedAt),
		FinishedAt: instantOut(r.FinishedAt), Outcome: r.Outcome, FetchedCount: r.Counts.Fetched,
		ImportedCount: r.Counts.Imported, SkippedExistingCount: r.Counts.SkippedExisting,
		SkippedDeletedCount: r.Counts.SkippedDeleted, FailedCount: r.Counts.Failed,
		ErrorSummary: make([]portalRunErrorOut, 0, len(r.Errors)),
	}
	for _, e := range r.Errors {
		out.ErrorSummary = append(out.ErrorSummary, portalRunErrorOut(e))
	}
	return out
}

func storedCategoriesOut(cs []domain.PortalCategory) []portalStoredCategoryOut {
	out := make([]portalStoredCategoryOut, 0, len(cs))
	for _, c := range cs {
		out = append(out, portalStoredCategoryOut{ID: c.ID, ExternalID: c.ExternalID, Name: c.Name,
			TargetKind: c.TargetKind, IsSelected: c.IsSelected})
	}
	return out
}

// GetPortalSyncSettings — GET /api/v1/portal-sync/settings
func (h *Handler) GetPortalSyncSettings(w http.ResponseWriter, r *http.Request) {
	v, err := h.d.PortalSync.Settings(r.Context())
	if err != nil {
		h.writePortalSyncError(w, r, "đọc cấu hình", err)
		return
	}
	vietJSON(w, http.StatusOK, portalSyncSettingsToOut(v))
}

// PutPortalSyncSettings — PUT /api/v1/portal-sync/settings
func (h *Handler) PutPortalSyncSettings(w http.ResponseWriter, r *http.Request) {
	var in portalSyncSettingsIn
	if !docThan(w, r, &in) {
		return
	}
	// The one conversion: from here on the key is a secret.Secret, which refuses to render.
	key := secret.Secret(in.APIKey)
	in.APIKey = ""
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	v, err := h.d.WritePortalSync.SaveSettings(r.Context(), app.SavePortalSyncSettingsRequest{
		Input: domain.PortalSyncSettingsInput{
			APIURL: in.APIURL, PublishMode: in.PublishMode, IntervalHours: in.IntervalHours,
			WindowDays: in.WindowDays, MaxItemsPerRun: in.MaxItemsPerRun,
			KeepSourceCredit: in.KeepSourceCredit, IsEnabled: in.IsEnabled,
		},
		APIKey: key,
	}, actor)
	clear(key)
	if err != nil {
		h.writePortalSyncError(w, r, "lưu cấu hình", err)
		return
	}
	vietJSON(w, http.StatusOK, portalSyncSettingsToOut(v))
}

// GetPortalCategories — GET /api/v1/portal-sync/categories (the LIVE tree)
func (h *Handler) GetPortalCategories(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	t, err := h.d.PortalSync.CategoryTree(r.Context(), actor)
	if err != nil {
		h.writePortalSyncError(w, r, "đọc chuyên mục Cổng", err)
		return
	}
	out := portalCategoryTreeOut{Items: make([]portalCategoryNodeOut, 0, len(t.Items)),
		Missing: storedCategoriesOut(t.Missing)}
	for _, n := range t.Items {
		out.Items = append(out.Items, portalCategoryNodeOut(n))
	}
	vietJSON(w, http.StatusOK, out)
}

// PutPortalCategories — PUT /api/v1/portal-sync/categories
func (h *Handler) PutPortalCategories(w http.ResponseWriter, r *http.Request) {
	var in portalCategoriesIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	sel := make([]domain.PortalCategorySelection, 0, len(in.Categories))
	for _, c := range in.Categories {
		sel = append(sel, domain.PortalCategorySelection(c))
	}
	saved, err := h.d.WritePortalSync.SaveCategories(r.Context(), sel, actor)
	if err != nil {
		h.writePortalSyncError(w, r, "lưu chuyên mục", err)
		return
	}
	vietJSON(w, http.StatusOK, portalCategoriesOut{Items: storedCategoriesOut(saved)})
}

// ListPortalSyncRuns — GET /api/v1/portal-sync/runs
func (h *Handler) ListPortalSyncRuns(w http.ResponseWriter, r *http.Request) {
	req, err := page.Parse(r.URL.Query(), commsstore.SortPortalSyncRuns)
	if err != nil {
		status, code, msg := page.HTTPError(err)
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	res, err := h.d.PortalSync.Runs(r.Context(), req)
	if err != nil {
		h.writePortalSyncError(w, r, "đọc lịch sử chạy", err)
		return
	}
	out := page.Result[portalRunOut]{Items: make([]portalRunOut, 0, len(res.Items)),
		NextCursor: res.NextCursor, HasMore: res.HasMore}
	for _, run := range res.Items {
		out.Items = append(out.Items, portalRunToOut(run))
	}
	vietJSON(w, http.StatusOK, out)
}

// StartPortalSyncRun — POST /api/v1/portal-sync/runs (`⟳ Đồng bộ ngay`). 202: the run row exists and
// the work continues in the background; GET runs shows how it ended.
func (h *Handler) StartPortalSyncRun(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	run, err := h.d.WritePortalSync.StartRun(r.Context(), actor)
	if err != nil {
		h.writePortalSyncError(w, r, "chạy đồng bộ", err)
		return
	}
	vietJSON(w, http.StatusAccepted, portalRunToOut(run))
}

// portalCallFailures turn a portal call's CLASS into what the administrator can check. Never the
// portal's words.
var portalCallFailures = map[string]string{
	"url-refused":        "Địa chỉ API đã lưu không còn đạt điều kiện (https, tên máy .gov.vn, cổng 443). Hãy sửa địa chỉ trong cấu hình.",
	"address-refused":    "Tên máy của Cổng trỏ tới một địa chỉ mạng nội bộ, nên hệ thống không gọi tới. Hãy kiểm tra địa chỉ API, hoặc báo đơn vị quản lý Cổng.",
	"redirect-refused":   "Cổng chuyển hướng sang một máy chủ khác với địa chỉ API đã khai (hoặc không phải https), nên hệ thống dừng lại. Hãy khai đúng địa chỉ API mà Cổng đang dùng.",
	"too-many-redirects": "Cổng chuyển hướng quá ba lần, nên hệ thống dừng lại. Hãy kiểm tra địa chỉ API.",
	"dns":                "Không tìm thấy tên máy của Cổng. Hãy kiểm tra địa chỉ API.",
	"connect":            "Không kết nối được tới Cổng. Hãy thử lại sau.",
	"timeout":            "Cổng không trả lời kịp. Hãy thử lại sau.",
	"tls":                "Chứng chỉ của Cổng không xác minh được với tên máy đã khai. Hệ thống không gọi qua kết nối chưa xác minh.",
	"too-large":          "Cổng trả về dữ liệu quá lớn.",
	"parse":              "Cổng trả về dữ liệu không đúng định dạng của API chia sẻ.",
	"http-401":           "Cổng từ chối mã bảo mật. Hãy nhập lại mã bảo mật và lưu cấu hình.",
	"http-403":           "Cổng từ chối mã bảo mật. Hãy nhập lại mã bảo mật và lưu cấu hình.",
	"http-404":           "Cổng không có API chia sẻ ở địa chỉ đã khai. Hãy kiểm tra địa chỉ API.",
}

const portalCallFallback = "Cổng trả lời lỗi. Hãy thử lại sau, hoặc báo đơn vị quản lý Cổng."

// writePortalSyncError maps a use-case failure onto a status. 503 when the platform cannot seal or run,
// 502 when the portal failed, 422 for the key rules and the category ceiling, 409 for the state of the
// data, 400 for the request, 500 for everything not listed — never a default 400.
func (h *Handler) writePortalSyncError(w http.ResponseWriter, r *http.Request, op string, err error) {
	xa := string(tenant.MustFrom(r.Context()))
	var call *app.PortalCallError
	switch {
	case errors.Is(err, crypto.ErrNotConfigured):
		httpx.WriteError(w, http.StatusServiceUnavailable, "encryption_not_configured",
			"Nền tảng chưa cấu hình khoá mã hoá bí mật (SECRET_ENCRYPTION_KEYS), nên chưa lưu hay dùng được mã "+
				"bảo mật của Cổng. Chưa có gì được ghi. Hãy báo đơn vị vận hành hệ thống.", "")
		return
	case errors.Is(err, app.ErrPortalRunnerStopped):
		httpx.WriteError(w, http.StatusServiceUnavailable, "portal_sync_unavailable",
			"Bộ chạy đồng bộ đang khởi động lại. Hãy thử lại sau ít phút.", "")
		return
	case errors.Is(err, app.ErrPortalKeyRequiredForNewURL):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "api_key_required_for_new_url",
			"Đã đổi địa chỉ API thì phải nhập lại mã bảo mật. Mã cũ không được gửi tới một địa chỉ chưa từng dùng nó.", "")
		return
	case errors.Is(err, app.ErrPortalKeyRequired):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "api_key_required",
			"Lần lưu đầu tiên phải nhập mã bảo mật của Cổng.", "")
		return
	case errors.Is(err, commsstore.ErrPortalSyncSettingsNotFound):
		httpx.WriteError(w, http.StatusConflict, "portal_sync_not_configured",
			"Xã chưa lưu cấu hình đồng bộ Cổng. Hãy lưu địa chỉ API và mã bảo mật trước.", "")
		return
	case errors.Is(err, app.ErrPortalRunInProgress):
		httpx.WriteError(w, http.StatusConflict, "portal_sync_in_progress",
			"Đang có một lượt đồng bộ của xã. Hãy chờ lượt ấy xong rồi chạy lại.", "")
		return
	case errors.Is(err, app.ErrPortalRunnerBusy):
		// The PROCESS is at its concurrent-run ceiling (R1, 02/10/2026) — a capacity condition, not the
		// state of this commune's data, so 503 and not 409. Retry-After: a run is minutes, not seconds.
		w.Header().Set("Retry-After", "60")
		httpx.WriteError(w, http.StatusServiceUnavailable, "portal_sync_busy",
			"Hệ thống đang chạy đồng bộ cho các xã khác. Hãy thử lại sau ít phút.", "")
		return
	case errors.Is(err, domain.ErrPortalTooManySelected):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "too_many_categories",
			"Mỗi xã chọn tối đa "+strconv.Itoa(domain.PortalSelectedCategoriesMax)+" chuyên mục Cổng. "+
				"Hãy bỏ chọn bớt rồi lưu lại.", "")
		return
	case errors.As(err, &call):
		// The class and the commune only — the adapter's errors carry no URL and no key.
		h.d.Log.Warn("đồng bộ Cổng: gọi Cổng không được", "xa", xa, "loai", call.Class)
		msg, ok := portalCallFailures[call.Class]
		if !ok {
			msg = portalCallFallback
		}
		httpx.WriteError(w, http.StatusBadGateway, "portal_"+strings.ReplaceAll(call.Class, "-", "_"), msg, "")
		return
	}
	if msg, ok := refusalMessage(portalSyncRefusals, err); ok {
		h.logRefusal(r, "đồng bộ Cổng: từ chối "+op, err)
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", msg, "")
		return
	}
	// The wrapped error never reaches the client (rule 3, forbidden #3), and never holds the key: nothing
	// in the chain formats one.
	h.d.Log.Error("đồng bộ Cổng: "+op+" lỗi hệ thống", "xa", xa, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// portalSyncRefusals — the 400s, one fixed sentence per sentinel. No bound is retyped as a number
// where the domain owns it unexported; the exported ones are named by constant.
var portalSyncRefusals = []refusal{
	{domain.ErrPortalAPIURLInvalid, "Địa chỉ API phải bắt đầu bằng https://, tên máy kết thúc bằng .gov.vn, " +
		"không kèm cổng khác 443, tài khoản, tham số (?…) hay địa chỉ IP."},
	{domain.ErrPortalPublishMode, "Hãy chọn Chờ duyệt hoặc Đăng thẳng."},
	{domain.ErrPortalInterval, "Nhịp đồng bộ phải từ 0 (chỉ chạy tay) tới 24 giờ."},
	{domain.ErrPortalWindow, "Số ngày lấy tin phải từ 1 tới " + strconv.Itoa(domain.PortalWindowDaysMax) + "."},
	{domain.ErrPortalMaxItems, "Số tin tối đa mỗi lượt phải từ 1 tới " + strconv.Itoa(domain.PortalMaxItemsMax) + "."},
	{domain.ErrPortalAPIKeyShape, "Mã bảo mật quá dài hoặc chứa khoảng trắng hay ký tự không hợp lệ."},
	{domain.ErrPortalSelectionEmpty, "Có chuyên mục Cổng không có mã."},
	{domain.ErrPortalSelectionName, "Có chuyên mục Cổng không có tên, tên quá dài hoặc chứa ký tự không hợp lệ."},
	{domain.ErrPortalSelectionKind, "Loại nội dung của chuyên mục Cổng phải là Tin tức, Sự kiện hoặc Thông báo."},
	{domain.ErrPortalSelectionDup, "Một chuyên mục Cổng xuất hiện hai lần."},
	{domain.ErrPortalSelectionTooBig, "Quá nhiều chuyên mục Cổng trong một lần lưu."},
}
