package domain

// THE PORTAL SYNC — one commune's settings, its choice of portal categories, and the record of each
// run (migration 0013; ADR 0067 §2). Standard library only (skills/go-service-pattern): the URL
// guard that decides whether an api_url may be called lives in internal/portal (CheckURL), and the app
// layer asks it. What is here is the shape of the values and the arithmetic of a run's outcome.
//
// ENUM VALUES ARE MIGRATION 0013's, character for character (ADR 0011: Vietnamese without diacritics,
// in the database and in the API, never translated).

import (
	"errors"
	"html"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// PortalProviderCityShared is the one API dialect: the Đà Nẵng city's shared commune portal
// (0013 `portal_sync_settings_provider_known`, value `cttdt-danang`). A second province's portal is a
// new value AND a new adapter — never a guess at this one's format.
const PortalProviderCityShared = "cttdt-danang"

// Publish modes (§10.2).
const (
	PortalPublishReview = "cho-duyet"  // imported items land `cho-duyet`: a person approves first (default)
	PortalPublishDirect = "dang-thang" // imported items land `dang-hien`: live at once
)

// What started a run.
const (
	PortalRunScheduled = "theo-lich" // the scheduler; actor is always the system principal
	PortalRunManual    = "chay-tay"  // `⟳ Đồng bộ ngay`; actor is the staff business code
)

// How a run ended.
const (
	PortalRunSucceeded = "thanh-cong"
	PortalRunPartial   = "mot-phan"
	PortalRunFailed    = "that-bai"
)

// The owner's DEFAULTS (01/10/2026, ADR 0067 §2 "Ghi" #3 and #4) and the schema's vendor BOUNDS (0013
// CHECKs). The bounds are guards against a bogus value, not customer figures.
const (
	PortalDefaultIntervalHours = 6
	PortalDefaultWindowDays    = 90
	PortalDefaultMaxItems      = 100

	PortalIntervalMax   = 24 // 0 = manual only
	PortalWindowDaysMax = 3650
	PortalMaxItemsMax   = 1000

	// PortalAPIURLMaxLen is 0013's `char_length(api_url) <= 2048`.
	PortalAPIURLMaxLen = 2048
	// PortalAPIKeyMaxLen bounds the key typed into the screen. The real one is a short token; 512 is a
	// guard against a paste of something else entirely.
	PortalAPIKeyMaxLen = 512

	// PortalCategoryExternalIDMax / PortalCategoryNameMax are 0013's shape CHECKs.
	PortalCategoryExternalIDMax = 200
	PortalCategoryNameMax       = 500
	// PortalSelectionMax bounds one selection save — §3's sample portal has ~60 categories.
	PortalSelectionMax = 1000
)

// PortalTargetKinds are the three `loai` a portal category may be imported as (0013, owner 01/10/2026).
var PortalTargetKinds = []LoaiNoiDung{LoaiTinTuc, LoaiSuKien, LoaiThongBao}

// IsPortalTargetKind reports whether k is one of PortalTargetKinds.
func IsPortalTargetKind(k string) bool {
	for _, t := range PortalTargetKinds {
		if string(t) == k {
			return true
		}
	}
	return false
}

// PortalSyncSettingsSubject is the audit locator of a commune's one settings row.
const PortalSyncSettingsSubject = "dong-bo-cong/cau-hinh"

// PortalSyncSettings is one commune's row, WITHOUT the key. The sealed key never enters this type;
// APIKeySet says only whether one is stored (ADR 0009 rule 6, ADR 0067 §2 decision 4).
type PortalSyncSettings struct {
	Provider         string
	APIURL           string
	PublishMode      string
	IntervalHours    int
	WindowDays       int
	MaxItemsPerRun   int
	KeepSourceCredit bool
	IsEnabled        bool

	APIKeySet bool
	LastRunAt time.Time // zero: never run
	UpdatedAt time.Time
	UpdatedBy string // staff business code (rule 6, invariant 8)
}

// DefaultPortalSyncSettings is what an unconfigured commune's screen starts from.
func DefaultPortalSyncSettings() PortalSyncSettings {
	return PortalSyncSettings{
		Provider: PortalProviderCityShared, PublishMode: PortalPublishReview,
		IntervalHours: PortalDefaultIntervalHours, WindowDays: PortalDefaultWindowDays,
		MaxItemsPerRun: PortalDefaultMaxItems, KeepSourceCredit: true,
	}
}

// PortalSyncSettingsInput is one save, as the screen sends it. Pointers: an omitted value keeps the
// default on a first save and the stored value afterwards (MergePortalSyncSettings).
type PortalSyncSettingsInput struct {
	APIURL           string
	PublishMode      string
	IntervalHours    *int
	WindowDays       *int
	MaxItemsPerRun   *int
	KeepSourceCredit *bool
	IsEnabled        *bool
}

var (
	ErrPortalAPIURLInvalid   = errors.New("dong_bo_cong: địa chỉ API phải là https, tên máy kết thúc bằng .gov.vn, cổng 443, không kèm tài khoản, tham số hay địa chỉ IP")
	ErrPortalPublishMode     = errors.New("dong_bo_cong: chế độ đăng phải là chờ duyệt hoặc đăng thẳng")
	ErrPortalInterval        = errors.New("dong_bo_cong: nhịp đồng bộ phải từ 0 (chỉ chạy tay) tới 24 giờ")
	ErrPortalWindow          = errors.New("dong_bo_cong: số ngày lấy tin không hợp lệ")
	ErrPortalMaxItems        = errors.New("dong_bo_cong: số tin tối đa mỗi lượt không hợp lệ")
	ErrPortalAPIKeyShape     = errors.New("dong_bo_cong: mã bảo mật quá dài hoặc chứa ký tự không hợp lệ")
	ErrPortalSelectionEmpty  = errors.New("dong_bo_cong: mã chuyên mục Cổng trống hoặc không hợp lệ")
	ErrPortalSelectionName   = errors.New("dong_bo_cong: tên chuyên mục Cổng trống, quá dài hoặc chứa ký tự không hợp lệ")
	ErrPortalSelectionKind   = errors.New("dong_bo_cong: loại nội dung của chuyên mục Cổng phải là tin tức, sự kiện hoặc thông báo")
	ErrPortalSelectionDup    = errors.New("dong_bo_cong: một chuyên mục Cổng xuất hiện hai lần")
	ErrPortalSelectionTooBig = errors.New("dong_bo_cong: quá nhiều chuyên mục Cổng trong một lần lưu")
)

// MergePortalSyncSettings applies an input onto a base (the stored row, or the defaults) and checks the
// result. The api_url is only shape-checked here (length, characters); whether it may be CALLED is
// internal/portal.CheckURL's question, asked by the app layer before anything is stored.
func MergePortalSyncSettings(base PortalSyncSettings, in PortalSyncSettingsInput) (PortalSyncSettings, error) {
	out := base
	out.Provider = PortalProviderCityShared
	out.APIURL = strings.TrimSpace(in.APIURL)
	if out.APIURL == "" || utf8.RuneCountInString(out.APIURL) > PortalAPIURLMaxLen ||
		strings.ContainsFunc(out.APIURL, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r) || r == '\\'
		}) {
		return PortalSyncSettings{}, ErrPortalAPIURLInvalid
	}
	if in.PublishMode != "" {
		out.PublishMode = in.PublishMode
	}
	if out.PublishMode != PortalPublishReview && out.PublishMode != PortalPublishDirect {
		return PortalSyncSettings{}, ErrPortalPublishMode
	}
	if in.IntervalHours != nil {
		out.IntervalHours = *in.IntervalHours
	}
	if out.IntervalHours < 0 || out.IntervalHours > PortalIntervalMax {
		return PortalSyncSettings{}, ErrPortalInterval
	}
	if in.WindowDays != nil {
		out.WindowDays = *in.WindowDays
	}
	if out.WindowDays < 1 || out.WindowDays > PortalWindowDaysMax {
		return PortalSyncSettings{}, ErrPortalWindow
	}
	if in.MaxItemsPerRun != nil {
		out.MaxItemsPerRun = *in.MaxItemsPerRun
	}
	if out.MaxItemsPerRun < 1 || out.MaxItemsPerRun > PortalMaxItemsMax {
		return PortalSyncSettings{}, ErrPortalMaxItems
	}
	if in.KeepSourceCredit != nil {
		out.KeepSourceCredit = *in.KeepSourceCredit
	}
	if in.IsEnabled != nil {
		out.IsEnabled = *in.IsEnabled
	}
	return out, nil
}

// ValidatePortalAPIKey checks the typed key: 1..PortalAPIKeyMaxLen bytes of printable non-space text.
func ValidatePortalAPIKey(b []byte) error {
	if len(b) == 0 || len(b) > PortalAPIKeyMaxLen || !utf8.Valid(b) {
		return ErrPortalAPIKeyShape
	}
	for _, r := range string(b) {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return ErrPortalAPIKeyShape
		}
	}
	return nil
}

// PortalAPIURLChanged is ADR 0067 §2 decision 5: changing the address requires the key again, so the
// stored key never follows the configuration to another host. Compared EXACTLY (after trimming): a
// different path is a different endpoint to the server that answers it, and asking for the key once
// more costs one paste.
func PortalAPIURLChanged(before, after PortalSyncSettings) bool {
	return strings.TrimSpace(before.APIURL) != strings.TrimSpace(after.APIURL)
}

// SamePortalSyncSettings reports whether a save would move nothing (ignoring the key, which the caller
// judges separately).
func SamePortalSyncSettings(a, b PortalSyncSettings) bool {
	return a.APIURL == b.APIURL && a.PublishMode == b.PublishMode && a.IntervalHours == b.IntervalHours &&
		a.WindowDays == b.WindowDays && a.MaxItemsPerRun == b.MaxItemsPerRun &&
		a.KeepSourceCredit == b.KeepSourceCredit && a.IsEnabled == b.IsEnabled
}

// PortalSyncDue reports whether the scheduler owes this commune a run at now. Interval 0 is manual
// only; a commune that never ran is due at once.
func PortalSyncDue(s PortalSyncSettings, now time.Time) bool {
	if !s.IsEnabled || s.IntervalHours <= 0 {
		return false
	}
	return s.LastRunAt.IsZero() || !s.LastRunAt.Add(time.Duration(s.IntervalHours)*time.Hour).After(now)
}

// --- categories --------------------------------------------------------------------------------------

// PortalCategory is one row of `portal_categories`: the commune's choice of a portal category and the
// `loai` it is imported as. Not a mirror of the portal's tree (0013).
type PortalCategory struct {
	ID         string
	ExternalID string
	Name       string
	TargetKind string
	IsSelected bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// PortalCategorySelection is one entry of a selection save.
type PortalCategorySelection struct {
	ExternalID string
	Name       string
	TargetKind string
	IsSelected bool
}

// NormalizePortalSelection trims and checks a selection save. Refusals, never truncation.
func NormalizePortalSelection(in []PortalCategorySelection) ([]PortalCategorySelection, error) {
	if len(in) > PortalSelectionMax {
		return nil, ErrPortalSelectionTooBig
	}
	seen := make(map[string]bool, len(in))
	out := make([]PortalCategorySelection, 0, len(in))
	for _, s := range in {
		s.ExternalID = strings.TrimSpace(s.ExternalID)
		s.Name = strings.TrimSpace(s.Name)
		if s.ExternalID == "" || s.ExternalID == "0" || len(s.ExternalID) > PortalCategoryExternalIDMax ||
			strings.ContainsFunc(s.ExternalID, unicode.IsControl) {
			return nil, ErrPortalSelectionEmpty
		}
		if s.Name == "" || utf8.RuneCountInString(s.Name) > PortalCategoryNameMax ||
			strings.ContainsFunc(s.Name, unicode.IsControl) {
			return nil, ErrPortalSelectionName
		}
		if !IsPortalTargetKind(s.TargetKind) {
			return nil, ErrPortalSelectionKind
		}
		if seen[s.ExternalID] {
			return nil, ErrPortalSelectionDup
		}
		seen[s.ExternalID] = true
		out = append(out, s)
	}
	return out, nil
}

// --- runs --------------------------------------------------------------------------------------------

// PortalRunError is one entry of `portal_sync_runs.error_summary`. 0013's rule on the writer: a
// category id and name, an error CLASS and a count — never an article's title, body, summary, URL or
// slug, never a portal response, never an exception message, never the api_url's query or the key.
type PortalRunError struct {
	CategoryExternalID string `json:"category_external_id,omitempty"`
	CategoryName       string `json:"category_name,omitempty"`
	Error              string `json:"error"`
	Count              int    `json:"count"`
}

// PortalSyncRun is one row of `portal_sync_runs`.
type PortalSyncRun struct {
	ID          string
	TriggerKind string
	Actor       string
	StartedAt   time.Time
	FinishedAt  time.Time // zero: still running (or crashed, until reaped)
	Outcome     string    // "" until finished
	Counts      PortalRunCounts
	Errors      []PortalRunError
}

// PortalRunCounts — what one run did. THE ARITHMETIC, stated (0013 leaves it to the writer):
//
//	Fetched          usable rows the portal returned across the selected categories (any date)
//	Imported         became a new item
//	SkippedExisting  its portal id is held by a live item (or a concurrent writer won the insert)
//	SkippedDeleted   its portal id is held by a SOFT-DELETED item — never brought back
//	Failed           an article inside the window that could not be imported
//
// Fetched − (Imported + Skipped* + Failed) is the rows outside the window, undated, repeated across
// categories, or past the run's ceiling (MaxItemsPerRun bounds the articles ATTEMPTED; the rest wait
// for the next run, which skips what this one imported).
type PortalRunCounts struct {
	Fetched         int
	Imported        int
	SkippedExisting int
	SkippedDeleted  int
	Failed          int
}

// Error classes the run itself writes (the adapter's classes are internal/portal's).
const (
	// PortalRunErrorInterrupted marks a run that stopped before its end (shutdown), or a crashed run
	// finished by the reaper.
	PortalRunErrorInterrupted = "interrupted"
	// PortalRunErrorNoCategory marks a run with nothing selected to read.
	PortalRunErrorNoCategory = "no-category-selected"
)

// PortalRunOutcome is the outcome of a run from what happened:
//
//	that-bai    nothing could be read: no category selected, every category failed, or interrupted
//	            before any category was read
//	mot-phan    some categories failed, some articles failed, or interrupted after some work
//	thanh-cong  every selected category was read and every attempted article landed
//
// Image problems (a foreign-host image dropped, an image that failed its checks) are noted in the
// summary and do NOT lower the outcome: the article itself was imported (ADR 0067 §2).
func PortalRunOutcome(categoriesRead, categoriesFailed int, c PortalRunCounts, interrupted bool) string {
	switch {
	case categoriesRead == 0:
		return PortalRunFailed
	case interrupted || categoriesFailed > 0 || c.Failed > 0:
		return PortalRunPartial
	default:
		return PortalRunSucceeded
	}
}

// PortalExternalItemID is the value written to `noi_dung_mini_app.nguon_id_ngoai` for a portal
// article: the provider and the portal's own id. NAMESPACED ON PURPOSE: `UNIQUE (tenant_id,
// nguon_id_ngoai)` is the whole deduplication (0006:343-358), and a second provider whose ids collide
// with this one's would otherwise be read as "already imported".
func PortalExternalItemID(provider, articleID string) string {
	return provider + ":" + articleID
}

// PortalSourceCredit is the closing paragraph an imported body carries when the commune keeps the
// credit (§2 "Chế độ đăng" #6). It goes INTO THE BODY, after the article — not the summary, which is
// §6's one-line preview — and is escaped here, then sanitised with the rest of the body.
func PortalSourceCredit(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	return "<p><em>Nguồn: " + html.EscapeString(label) + "</em></p>"
}
