package http

// THE ECONOMIC MAP — the commune's asset register (migration 0015; ADR 0072; docs/ui-ux/
// 10-ban-do-kinh-te-so.md; staff menu `ban-do-kinh-te-so`) and the one write the catalogue needed for it.
//
//	POST   /api/v1/map-asset-types/defaults      admin.lookup  sow the eleven groups (ADR 0072 §3)
//	GET    /api/v1/map-asset-points              asset.read    GeoJSON of the filter, light properties
//	GET    /api/v1/map-assets                    asset.read    `Sổ địa điểm`, one page, masked
//	GET    /api/v1/map-assets/{id}               asset.read    one asset; unmasked for asset.update (audited)
//	GET    /api/v1/map-asset-summary             asset.read    counts per group, verified ratio
//	POST   /api/v1/map-assets                    asset.update  add one
//	PATCH  /api/v1/map-assets/{id}               asset.update  edit one
//	DELETE /api/v1/map-assets/{id}               asset.update  soft delete, reason mandatory
//	POST   /api/v1/map-assets/{id}/confirmation  asset.update  set / clear `verified`
//
// THE KEYS EXIST AND NONE WAS INVENTED (rule 5, invariant 3c): `asset.read` / `asset.update` are the
// spec's own (§12.5) and are seeded at service-identity/migrations/0001_init.sql:287-288; `admin.lookup`
// is the key every write of the type catalogue already declares (routes.go, POST /api/v1/map-asset-types).
//
// THE NOUN IS `asset` (kb/00-foundation/ubiquitous-language.md, row `MapAssetType`): the permissions
// fixed it, and `resource` / `poi` would give one concept two English words.
//
// A SEPARATE REGISTRATION WITH ITS OWN DEPS (RegisterMapAssets), the external contacts' shape: these
// routes share nothing with the rest of the service but the checker. cmd/server mounts them on the same
// staff mux.
//
// PERSONAL DATA (rule 3; 0015 §PERSONAL DATA). `representative`, `phone` and `tax_code` leave the API
// MASKED unless the caller holds `asset.update` AND reads one asset's detail — and that full read is
// itself audited (rule 6, invariant 7) before the body is written, the way petitions audit
// `xem_day_du_nguoi_gui`. The list is ALWAYS masked: it is a register many rows at a time, and an
// officer who needs a number opens the record. The map points carry no personal field at all. A WRITE
// reply (create, edit, confirmation) is unmasked ONLY when the use case actually wrote — its own entry,
// same transaction, records the act; a no-op wrote no entry, so it answers masked like the detail
// without a full view. `address` and the coordinates are never masked, for any group: ADR 0072 sửa đổi
// 04/10/2026 — chủ dự án chốt hộ kinh doanh hiển thị như doanh nghiệp.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// MapAssetReader is the READ half — store calls, no transaction. *commsstore.MapAssetStore satisfies it.
type MapAssetReader interface {
	Points(ctx context.Context, f domain.MapAssetFilter) ([]domain.MapAssetPoint, error)
	List(ctx context.Context, f domain.MapAssetFilter, req page.Request) (page.Result[domain.MapAsset], error)
	ByID(ctx context.Context, id string) (domain.MapAsset, error)
	Summary(ctx context.Context) (domain.MapAssetSummary, error)
}

// MapAssetWriter is the WRITE half: each method opens a TRANSACTION and writes its audit entry inside
// it (rule 6, invariant 3) — RecordFullView included. *app.MapAssets satisfies it.
type MapAssetWriter interface {
	Create(ctx context.Context, in app.MapAssetInput, actor audit.Actor) (domain.MapAsset, error)
	// Update and SetConfirmation report `written`: true only when the row changed AND its entry is in the
	// same transaction. It decides whether the reply may be unmasked.
	Update(ctx context.Context, id string, p app.MapAssetPatch, actor audit.Actor) (row domain.MapAsset, written bool, err error)
	Delete(ctx context.Context, id, reason string, actor audit.Actor) error
	SetConfirmation(ctx context.Context, id string, verified bool, actor audit.Actor) (row domain.MapAsset, written bool, err error)
	RecordFullView(ctx context.Context, a domain.MapAsset, actor audit.Actor) error
}

// MapAssetTypeSeeder sows the eleven default groups. *app.MapAssetTypeDefaults satisfies it.
type MapAssetTypeSeeder interface {
	SeedDefaults(ctx context.Context, actor audit.Actor) (app.MapAssetTypeSeedResult, error)
}

// MapAssetDeps is everything the nine routes touch.
type MapAssetDeps struct {
	Checker      authz.Checker
	Reader       MapAssetReader
	Writer       MapAssetWriter
	TypeDefaults MapAssetTypeSeeder
	Log          *slog.Logger
}

type mapAssetHandler struct {
	d MapAssetDeps
}

// --- response shapes -----------------------------------------------------------------------------

// mapAssetPointsOut is a GeoJSON FeatureCollection (RFC 7946), the shape MapLibre's GeoJSON source and
// its browser-side clustering read as is.
type mapAssetPointsOut struct {
	Type     string               `json:"type"` // always "FeatureCollection"
	Features []mapAssetFeatureOut `json:"features"`
}

type mapAssetFeatureOut struct {
	Type       string                `json:"type"` // always "Feature"
	ID         string                `json:"id"`
	Geometry   mapAssetGeometryOut   `json:"geometry"`
	Properties mapAssetPointPropsOut `json:"properties"`
}

// mapAssetGeometryOut — `coordinates` is [lng, lat], LONGITUDE FIRST (RFC 7946 §3.1.1). The opposite
// of how a Vietnamese address card writes it, and the swap draws every pin in the wrong ocean.
type mapAssetGeometryOut struct {
	Type        string    `json:"type"` // always "Point"
	Coordinates []float64 `json:"coordinates"`
}

// mapAssetPointPropsOut — LIGHT ON PURPOSE: what the marker colour, the layer toggle and the popup
// title need. No representative, phone, tax code or address (rule 3); the popup's "open" link reads the
// detail route.
type mapAssetPointPropsOut struct {
	ID            string `json:"id"`
	AssetTypeCode string `json:"asset_type_code"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Verified      bool   `json:"verified"`
}

// mapAssetRowOut is one row of the `Sổ địa điểm` table (spec §7). `representative` and `phone` are
// ALWAYS MASKED here — see the file header. Absent when the asset has none (`—` on the screen).
type mapAssetRowOut struct {
	ID                string `json:"id"`
	AssetTypeCode     string `json:"asset_type_code"`
	Name              string `json:"name"`
	Address           string `json:"address,omitempty"`
	ResidentialUnitID string `json:"residential_unit_id,omitempty"`
	Representative    string `json:"representative,omitempty"`
	Phone             string `json:"phone,omitempty"`
	Status            string `json:"status"`
	Verified          bool   `json:"verified"`
}

// mapAssetOut is one asset with every field (spec §8, §10).
type mapAssetOut struct {
	ID                string  `json:"id"`
	AssetTypeCode     string  `json:"asset_type_code"`
	Name              string  `json:"name"`
	Address           string  `json:"address,omitempty"`
	ResidentialUnitID string  `json:"residential_unit_id,omitempty"`
	Lat               float64 `json:"lat"`
	Lng               float64 `json:"lng"`
	// Representative, Phone and TaxCode are MASKED when `masked` is true.
	Representative string     `json:"representative,omitempty"`
	Phone          string     `json:"phone,omitempty"`
	Status         string     `json:"status"`
	Verified       bool       `json:"verified"`
	VerifiedAt     *time.Time `json:"verified_at,omitempty"`
	VerifiedBy     string     `json:"verified_by,omitempty"` // business code `CB-…`
	TaxCode        string     `json:"tax_code,omitempty"`
	IndustryCode   string     `json:"industry_code,omitempty"`
	EmployeeCount  *int       `json:"employee_count,omitempty"`
	EstablishedOn  string     `json:"established_on,omitempty"` // YYYY-MM-DD
	Description    string     `json:"description,omitempty"`
	// CustomValues is {"<field_code>": value} — every stored key, including those of a retired or
	// disabled field (spec §12.6); the form shows only the fields map-field-schemas lists as active.
	CustomValues map[string]any `json:"custom_values"`
	// Masked says whether the three personal fields above are masked — so a form never saves a mask
	// back as if it were the value.
	Masked    bool      `json:"masked"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type mapAssetTypeCountOut struct {
	AssetTypeCode string `json:"asset_type_code"`
	Count         int    `json:"count"`
	Verified      int    `json:"verified"`
}

// mapAssetSummaryOut — `total` and `verified_ratio` are the header meta line ("26 đối tượng · 42.3% đã
// xác minh"); `by_type` the layer counts. Groups with no live asset are absent from `by_type`.
type mapAssetSummaryOut struct {
	Total         int                    `json:"total"`
	Verified      int                    `json:"verified"`
	VerifiedRatio float64                `json:"verified_ratio"` // 0..1, four decimals; 0 when total is 0
	ByType        []mapAssetTypeCountOut `json:"by_type"`
}

type mapAssetTypeRefOut struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// seedMapAssetTypesOut is what one run did — THREE LISTS, always present, `[]` when empty.
type seedMapAssetTypesOut struct {
	Created         []mapAssetTypeRefOut `json:"created"`          // nhóm vừa thêm vào danh mục của xã
	SkippedExisting []mapAssetTypeRefOut `json:"skipped_existing"` // xã đã có mã này — giữ NGUYÊN
	SkippedDeleted  []mapAssetTypeRefOut `json:"skipped_deleted"`  // xã đã XOÁ mã này — không khôi phục
	CreatedCount    int                  `json:"created_count"`
}

// --- request shapes ------------------------------------------------------------------------------

// createMapAssetIn is the body of POST. `omitempty` on the optional fields so tools/apidoc does not mark
// them required. `lat` / `lng` are required (spec §12.1) — pointers so a missing one is told from 0.
// `verified` is declared ONLY TO BE REFUSED: confirmation is its own route with its own trail.
type createMapAssetIn struct {
	AssetTypeCode     string         `json:"asset_type_code"`
	Name              string         `json:"name"`
	Lat               *float64       `json:"lat"`
	Lng               *float64       `json:"lng"`
	Address           string         `json:"address,omitempty"`
	ResidentialUnitID string         `json:"residential_unit_id,omitempty"`
	Representative    string         `json:"representative,omitempty"`
	Phone             string         `json:"phone,omitempty"`
	Status            string         `json:"status,omitempty"` // default dang-hoat-dong
	TaxCode           string         `json:"tax_code,omitempty"`
	IndustryCode      string         `json:"industry_code,omitempty"`
	EmployeeCount     *int           `json:"employee_count,omitempty"`
	EstablishedOn     string         `json:"established_on,omitempty"`
	Description       string         `json:"description,omitempty"`
	CustomValues      map[string]any `json:"custom_values,omitempty"`

	Verified *bool `json:"verified,omitempty"`
}

// updateMapAssetIn is the body of PATCH. ABSENT = leave alone; "" CLEARS an optional text; `lat` and
// `lng` travel together. `custom_values` MERGES by key — a key set to null is removed, keys not sent
// stay. `employee_count` cannot be cleared once set (JSON null reads as absent through a pointer).
type updateMapAssetIn struct {
	AssetTypeCode     *string        `json:"asset_type_code,omitempty"`
	Name              *string        `json:"name,omitempty"`
	Lat               *float64       `json:"lat,omitempty"`
	Lng               *float64       `json:"lng,omitempty"`
	Address           *string        `json:"address,omitempty"`
	ResidentialUnitID *string        `json:"residential_unit_id,omitempty"`
	Representative    *string        `json:"representative,omitempty"`
	Phone             *string        `json:"phone,omitempty"`
	Status            *string        `json:"status,omitempty"`
	TaxCode           *string        `json:"tax_code,omitempty"`
	IndustryCode      *string        `json:"industry_code,omitempty"`
	EmployeeCount     *int           `json:"employee_count,omitempty"`
	EstablishedOn     *string        `json:"established_on,omitempty"`
	Description       *string        `json:"description,omitempty"`
	CustomValues      map[string]any `json:"custom_values,omitempty"`

	Verified *bool `json:"verified,omitempty"`
}

// deleteMapAssetIn — the reason is mandatory (rule 7, invariant 1) and travels in the body, never the
// query string, where free text would land in every access log.
type deleteMapAssetIn struct {
	Reason string `json:"reason"`
}

// mapAssetConfirmationIn is the body of POST …/confirmation. A pointer: a body without `verified` is a
// 400, never read as "clear".
type mapAssetConfirmationIn struct {
	Verified *bool `json:"verified"`
}

// --- registration --------------------------------------------------------------------------------

// RegisterMapAssets mounts the nine routes. Refuses incomplete wiring at construction, like Register.
func RegisterMapAssets(mux *http.ServeMux, d MapAssetDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — chín tuyến bản đồ kinh tế số sẽ không kiểm được quyền")
	case d.Reader == nil:
		panic("comms/http: thiếu kho đối tượng bản đồ — bốn tuyến đọc /api/v1/map-asset… sẽ panic khi có người gọi")
	case d.Writer == nil:
		panic("comms/http: thiếu use case ghi đối tượng bản đồ — POST/PATCH/DELETE /api/v1/map-assets sẽ panic khi có người gọi")
	case d.TypeDefaults == nil:
		panic("comms/http: thiếu use case nạp nhóm mặc định — POST /api/v1/map-asset-types/defaults sẽ panic khi có người gọi")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &mapAssetHandler{d: d}

	// THE ELEVEN GROUPS OF ADR 0072 §3, INTO THIS COMMUNE — the ADR 0055 / roles-defaults shape: pressed by
	// an administrator, idempotent, existing codes untouched, soft-deleted codes NOT revived, one trail.
	// The rows are `nguon = 'he-thong'` (tier 2: relabel or disable, never delete — migration 0003).
	//
	// `admin.lookup`, THE KEY OF EVERY OTHER WRITE OF THIS CATALOGUE (routes.go:436-501); no key invented.
	//
	// 200 AND NOT 201, INCLUDING ON THE FIRST RUN: the request brings the catalogue to a known state, it
	// does not create one addressable resource (roles/defaults' reasoning).
	//
	// idem.KhongCan: a second run finds every code present and writes and audits NOTHING; two runs at
	// once serialise on a per-commune advisory lock, so the second reports the first's rows as existing.
	//
	// 409 `catalogue_full` when adding the missing groups would pass the catalogue's ceiling.
	//
	// @summary  Nạp 11 nhóm tài nguyên mặc định (ADR 0072) vào danh mục của xã — mã đã có giữ nguyên, mã đã xoá không khôi phục
	// @screen   10-ban-do-kinh-te-so §3
	// @reply    200 seedMapAssetTypesOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error catalogue_full
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/map-asset-types/defaults",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("chạy lần hai thấy mọi mã đã có nên không ghi và không kiểm toán gì; hai lượt cùng lúc tuần tự hoá trên khoá tư vấn theo xã")(
				http.HandlerFunc(h.SeedMapAssetTypes))))

	// THE MAP: every live asset of the filter as a GeoJSON FeatureCollection, coordinates [lng, lat].
	// Filters: asset_type_code (repeat it, or comma-separate, ≤ 20), status, verified (true|false),
	// industry_code (2 digits), residential_unit_id, q (name or address contains).
	//
	// NOT PAGINATED, BOUNDED: the map draws the whole filter at once (browser clustering, ADR 0072 §4).
	// Past commsstore.MapAssetPointsCeiling (5000) it answers 422 `too_many_points` rather than drawing a
	// map with places silently missing — the commune narrows the filter (by group).
	//
	// NO idem.* DECLARATION: a GET changes no state. No audit entry: no personal field is returned, and
	// the read is inside the commune the request arrived in (rule 6, invariant 7 asks for neither case).
	//
	// @summary  Điểm tài nguyên trên bản đồ kinh tế số của xã — GeoJSON FeatureCollection, toạ độ [kinh độ, vĩ độ], chỉ thuộc tính nhẹ
	// @screen   10-ban-do-kinh-te-so §4, §6
	// @reply    200 mapAssetPointsOut
	// @reply    400 httpx.Error invalid_filter
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    422 httpx.Error too_many_points
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/map-asset-points",
		authz.RequirePermission(d.Checker, "asset.read")(
			http.HandlerFunc(h.MapAssetPoints)))

	// THE `Sổ địa điểm` TABLE (spec §7): one page, ordered by group, then name, then id — the cursor
	// carries only the anchor's id (the name may be a person's). Same filters as the points.
	// `representative` and `phone` ALWAYS MASKED here.
	//
	// @summary  Sổ địa điểm của xã — một trang, xếp theo nhóm rồi tên; người đại diện và số điện thoại luôn che
	// @screen   10-ban-do-kinh-te-so §7
	// @reply    200 page.Result[mapAssetRowOut]
	// @reply    400 httpx.Error invalid_filter invalid_cursor invalid_sort invalid_limit
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/map-assets",
		authz.RequirePermission(d.Checker, "asset.read")(
			http.HandlerFunc(h.ListMapAssets)))

	// ONE ASSET, every field. Masked for `asset.read`; UNMASKED for a caller who also holds `asset.update`
	// (ADR 0072 §5 "số đầy đủ chỉ cho người giữ asset.update") — and that disclosure is AUDITED first
	// (`xem_day_du_doi_tuong_ban_do`, rule 6 invariant 7); no business code on the principal = masked.
	//
	// 404 is ONE answer for no such id, another commune's id and a deleted asset.
	//
	// @summary  Một đối tượng trên bản đồ kinh tế số, đủ trường kèm giá trị tuỳ biến — che người đại diện, điện thoại, mã số thuế trừ khi có asset.update
	// @screen   10-ban-do-kinh-te-so §6, §8
	// @reply    200 mapAssetOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/map-assets/{id}",
		authz.RequirePermission(d.Checker, "asset.read")(
			http.HandlerFunc(h.GetMapAsset)))

	// THE HEADER META LINE AND THE LAYER COUNTS: live assets per group, total, verified count and ratio.
	//
	// @summary  Số đối tượng theo nhóm, tổng số và tỷ lệ đã xác minh của bản đồ kinh tế số
	// @screen   10-ban-do-kinh-te-so §1, §4.1
	// @reply    200 mapAssetSummaryOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/map-asset-summary",
		authz.RequirePermission(d.Checker, "asset.read")(
			http.HandlerFunc(h.MapAssetSummary)))

	// ADD ONE ASSET (spec §8). Required: asset_type_code, name, lat, lng.
	//
	// idem.Required(DongKhiHong), THE CONTENT-ITEM CHOICE: the only unique key underneath is the tax code,
	// and most rows have none — a cache outage with MoKhiHong would let a double-submitted form put the
	// same place on the map twice. The idempotency key is the ONLY layer, so it cannot fail open.
	//
	//	400 invalid_request         a shape refused (field named, value never quoted), or `verified` sent
	//	409 tax_code_taken          another live asset of this commune holds the tax code
	//	422 asset_type_unavailable  the group is not live and in use in this commune's catalogue
	//	422 invalid_custom_values   a key the group's field schema lacks / has disabled, a value of the
	//	                            wrong type or outside its options, a required field missing
	//
	// @summary  Thêm một đối tượng lên bản đồ kinh tế số của xã
	// @screen   10-ban-do-kinh-te-so §8
	// @request  createMapAssetIn
	// @reply    201 mapAssetOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error tax_code_taken
	// @reply    422 httpx.Error asset_type_unavailable invalid_custom_values
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/map-assets",
		authz.RequirePermission(d.Checker, "asset.update")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.CreateMapAsset))))

	// EDIT ONE ASSET. PATCH AND NOT PUT: most fields are optional and a dialog editing the phone must not
	// clear the rest. The row AFTER the merge must satisfy the group's CURRENT form — an asset filed before
	// a field became required is refused (422) until the value is supplied.
	//
	// idem.KhongCan: app.MapAssets.Update writes and audits nothing when nothing moved.
	//
	// @summary  Sửa một đối tượng trên bản đồ kinh tế số — chỉ trường được gửi; custom_values gộp theo khoá
	// @screen   10-ban-do-kinh-te-so §7, §8
	// @request  updateMapAssetIn
	// @reply    200 mapAssetOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error tax_code_taken
	// @reply    422 httpx.Error asset_type_unavailable invalid_custom_values
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/map-assets/{id}",
		authz.RequirePermission(d.Checker, "asset.update")(
			idem.KhongCan("sửa là ghi đè một trạng thái đã biết; use case không ghi gì khi không có trường nào đổi, nên lần gửi thứ hai để lại đúng một dòng và đúng một vết")(
				http.HandlerFunc(h.UpdateMapAsset))))

	// SOFT DELETE with a mandatory reason, IN THE BODY (`{ "reason": "…" }`, 204) — the shape of DELETE
	// /api/v1/external-contacts/{id}. The asset leaves the map, the table and the counts at once.
	//
	// idem.KhongCan — a second delete is a 404: the locked read and the UPDATE carry `AND deleted_at IS
	// NULL`, so it cannot overwrite who deleted it or why.
	//
	// @summary  Xoá mềm một đối tượng trên bản đồ kinh tế số, kèm lý do bắt buộc
	// @screen   10-ban-do-kinh-te-so §7
	// @request  deleteMapAssetIn
	// @reply    204 -
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/map-assets/{id}",
		authz.RequirePermission(d.Checker, "asset.update")(
			idem.KhongCan("xoá một đối tượng đã xoá trả 404: lượt đọc khoá dòng và câu UPDATE đều mang `AND deleted_at IS NULL`, nên lần thứ hai không ghi đè được người xoá và lý do")(
				http.HandlerFunc(h.DeleteMapAsset))))

	// VERIFY / UN-VERIFY (spec §10 `da_xac_minh` · `xac_minh_luc` · `nguoi_xac_minh`). `verified: true`
	// sets verified_at = now and verified_by = the caller's business code; `false` clears both — the trail
	// keeps who had verified and when. `confirmation`: a nominalised sub-resource (rest-api-design §3).
	//
	// THE NOUN IS `confirmation`, NOT `verification` (owner, 04/10/2026): `verification` already names
	// "Nghiệm thu" of a petition (kb/00-foundation/ubiquitous-language.md:155), and one English word for
	// two acts would make the trail and the contract ambiguous. The COLUMNS and JSON FIELDS stay
	// `verified` / `verified_at` / `verified_by` — migration 0015 is applied and is not edited.
	//
	// idem.KhongCan: asking for the state the asset is already in writes and audits nothing.
	//
	// @summary  Xác minh hoặc bỏ xác minh một đối tượng trên bản đồ kinh tế số
	// @screen   10-ban-do-kinh-te-so §7, §10
	// @request  mapAssetConfirmationIn
	// @reply    200 mapAssetOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/map-assets/{id}/confirmation",
		authz.RequirePermission(d.Checker, "asset.update")(
			idem.KhongCan("đặt về đúng trạng thái đang có thì không ghi và không kiểm toán gì, nên lần gửi thứ hai để lại đúng một trạng thái và đúng một vết")(
				http.HandlerFunc(h.SetMapAssetConfirmation))))
}

// --- reads ---------------------------------------------------------------------------------------

// parseMapAssetFilter reads the shared filter off the query string and normalises it. Nothing read here
// reaches the statement text: every value is bound (commsstore.mapAssetFilterSQL).
func parseMapAssetFilter(q url.Values) (domain.MapAssetFilter, error) {
	f := domain.MapAssetFilter{
		AssetTypeCodes:    q["asset_type_code"],
		Status:            firstQueryValue(q, "status"),
		IndustryCode:      firstQueryValue(q, "industry_code"),
		ResidentialUnitID: firstQueryValue(q, "residential_unit_id"),
		Query:             firstQueryValue(q, "q"),
	}
	switch firstQueryValue(q, "verified") {
	case "":
	case "true":
		v := true
		f.Verified = &v
	case "false":
		v := false
		f.Verified = &v
	default:
		return domain.MapAssetFilter{}, domain.ErrMapAssetFilterInvalid
	}
	return domain.NormalizeMapAssetFilter(f)
}

// firstQueryValue is url.Values.Get spelled as an index — tools/apidoc reads the parameter name at the
// call site (truyvan.go, package-level accessors), the same as it reads q.Get.
func firstQueryValue(q url.Values, key string) string {
	if v := q[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func writeInvalidFilter(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, "invalid_filter",
		"Bộ lọc không hợp lệ: nhóm là mã danh mục (tối đa 20), trạng thái là dang-hoat-dong / tam-ngung / da-giai-the, "+
			"verified là true hoặc false, ngành nghề là hai chữ số, từ khoá tối đa 100 ký tự.", "")
}

// MapAssetPoints — GET /api/v1/map-asset-points
func (h *mapAssetHandler) MapAssetPoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f, err := parseMapAssetFilter(r.URL.Query())
	if err != nil {
		writeInvalidFilter(w)
		return
	}
	// Scoped in the store: MapAssetStore.Points reads through db.For(ctx).Query, live rows only.
	pts, err := h.d.Reader.Points(ctx, f)
	if err != nil {
		if errors.Is(err, commsstore.ErrTooManyMapAssetPoints) {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "too_many_points",
				fmt.Sprintf("Bộ lọc này có hơn %d đối tượng — quá nhiều để vẽ cùng lúc. "+
					"Hãy chọn bớt nhóm tài nguyên hoặc thu hẹp bộ lọc.", commsstore.MapAssetPointsCeiling), "")
			return
		}
		h.internal(ctx, w, "đọc điểm bản đồ", err)
		return
	}
	out := mapAssetPointsOut{Type: "FeatureCollection", Features: make([]mapAssetFeatureOut, 0, len(pts))}
	for _, p := range pts {
		out.Features = append(out.Features, mapAssetFeatureOut{
			Type: "Feature",
			ID:   p.ID,
			// [lng, lat] — RFC 7946 order.
			Geometry: mapAssetGeometryOut{Type: "Point", Coordinates: []float64{p.Lng, p.Lat}},
			Properties: mapAssetPointPropsOut{
				ID: p.ID, AssetTypeCode: p.AssetTypeCode, Name: p.Name, Status: p.Status, Verified: p.Verified,
			},
		})
	}
	vietJSON(w, http.StatusOK, out)
}

// ListMapAssets — GET /api/v1/map-assets
func (h *mapAssetHandler) ListMapAssets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	// Validated BEFORE the database: a rejected request runs no statement.
	req, err := page.Parse(q, commsstore.MapAssetListSort)
	if err != nil {
		st, code, msg := page.HTTPError(err)
		httpx.WriteError(w, st, code, msg, "")
		return
	}
	f, err := parseMapAssetFilter(q)
	if err != nil {
		writeInvalidFilter(w)
		return
	}
	// Scoped in the store: MapAssetStore.List reads through db.For(ctx).Query, live rows only.
	res, err := h.d.Reader.List(ctx, f, req)
	if err != nil {
		h.internal(ctx, w, "đọc sổ địa điểm", err)
		return
	}
	out := page.Result[mapAssetRowOut]{Items: make([]mapAssetRowOut, 0, len(res.Items)),
		NextCursor: res.NextCursor, HasMore: res.HasMore}
	for _, a := range res.Items {
		out.Items = append(out.Items, mapAssetRowOut{
			ID: a.ID, AssetTypeCode: a.AssetTypeCode, Name: a.Name, Address: a.Address,
			ResidentialUnitID: a.ResidentialUnitID,
			Representative:    maskIfSet(a.Representative, privacy.MaskName),
			Phone:             maskIfSet(a.Phone, privacy.MaskPhone),
			Status:            a.Status, Verified: a.Verified,
		})
	}
	vietJSON(w, http.StatusOK, out)
}

// GetMapAsset — GET /api/v1/map-assets/{id}
//
// THE DISCLOSURE DECISION, LAST, AND THE TRAIL BEFORE THE BODY — petitions' order: everything above can
// still fail, and an entry recording a disclosure that never reached anybody is a false record.
func (h *mapAssetHandler) GetMapAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Scoped in the store: MapAssetStore.ByID reads through db.For(ctx).Query, live rows only.
	a, err := h.d.Reader.ByID(ctx, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, commsstore.ErrMapAssetNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy đối tượng này trên bản đồ của xã.", "")
			return
		}
		h.internal(ctx, w, "đọc một đối tượng", err)
		return
	}

	full := false
	// `Ma == ""` CHANGES WHAT THIS DOES, not just what it writes: with no business code to attribute the
	// disclosure to, the officer gets the masked view (fail closed — petitions' rule).
	if p, ok := authz.From(ctx); ok && p.Ma != "" && h.d.Checker.Allows(ctx, p, "asset.update") {
		actor := audit.Actor{ID: p.Ma, Kind: p.Kind, IP: httpx.ClientIP(r)}
		if err := h.d.Writer.RecordFullView(ctx, a, actor); err != nil {
			h.internal(ctx, w, "ghi vết xem đầy đủ đối tượng bản đồ", err)
			return
		}
		full = true
	}
	vietJSON(w, http.StatusOK, mapAssetToOut(a, full))
}

// MapAssetSummary — GET /api/v1/map-asset-summary
func (h *mapAssetHandler) MapAssetSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Scoped in the store: MapAssetStore.Summary reads through db.For(ctx).Query, live rows only.
	s, err := h.d.Reader.Summary(ctx)
	if err != nil {
		h.internal(ctx, w, "đếm đối tượng bản đồ", err)
		return
	}
	out := mapAssetSummaryOut{Total: s.Total, Verified: s.Verified, VerifiedRatio: s.VerifiedRatio(),
		ByType: make([]mapAssetTypeCountOut, 0, len(s.ByType))}
	for _, c := range s.ByType {
		out.ByType = append(out.ByType, mapAssetTypeCountOut{AssetTypeCode: c.AssetTypeCode, Count: c.Count, Verified: c.Verified})
	}
	vietJSON(w, http.StatusOK, out)
}

// --- writes --------------------------------------------------------------------------------------

// SeedMapAssetTypes — POST /api/v1/map-asset-types/defaults
func (h *mapAssetHandler) SeedMapAssetTypes(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.MapAssetTypeDefaults opens db.For(ctx).Tx.
	res, err := h.d.TypeDefaults.SeedDefaults(r.Context(), actor)
	if err != nil {
		if errors.Is(err, commsstore.ErrDanhMucDayTran) {
			httpx.WriteError(w, http.StatusConflict, "catalogue_full",
				"Danh mục loại tài nguyên bản đồ của xã sẽ vượt số mục tối đa nếu nạp thêm. Hãy xoá bớt mục không dùng.", "")
			return
		}
		h.internal(r.Context(), w, "nạp nhóm mặc định", err)
		return
	}
	refs := func(in []app.MapAssetTypeRef) []mapAssetTypeRefOut {
		out := make([]mapAssetTypeRefOut, 0, len(in))
		for _, x := range in {
			out = append(out, mapAssetTypeRefOut{Code: x.Code, Label: x.Label})
		}
		return out
	}
	vietJSON(w, http.StatusOK, seedMapAssetTypesOut{
		Created: refs(res.Created), SkippedExisting: refs(res.SkippedExisting), SkippedDeleted: refs(res.SkippedDeleted),
		CreatedCount: len(res.Created),
	})
}

// CreateMapAsset — POST /api/v1/map-assets
func (h *mapAssetHandler) CreateMapAsset(w http.ResponseWriter, r *http.Request) {
	var in createMapAssetIn
	if !decodeWithNumbers(w, r, &in) {
		return
	}
	if in.Verified != nil {
		writeVerifiedNotHere(w)
		return
	}
	custom, ok := customValuesIn(w, in.CustomValues)
	if !ok {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.MapAssets opens db.For(ctx).Tx.
	row, err := h.d.Writer.Create(r.Context(), app.MapAssetInput{
		AssetTypeCode: in.AssetTypeCode, Name: in.Name, Address: in.Address, ResidentialUnitID: in.ResidentialUnitID,
		Lat: in.Lat, Lng: in.Lng, Representative: in.Representative, Phone: in.Phone, Status: in.Status,
		TaxCode: in.TaxCode, IndustryCode: in.IndustryCode, EmployeeCount: in.EmployeeCount,
		EstablishedOn: in.EstablishedOn, Description: in.Description, CustomValues: custom,
	}, actor)
	if err != nil {
		h.writeError(w, r, "thêm", err)
		return
	}
	// The row's id, not the body: the body (a phone number) would go into Redis, a cache.
	idem.RecordCode(r.Context(), row.ID)
	// UNMASKED: the caller holds asset.update and has just typed these values; the create's own entry,
	// in the same transaction, records the act.
	vietJSON(w, http.StatusCreated, mapAssetToOut(row, true))
}

// UpdateMapAsset — PATCH /api/v1/map-assets/{id}
func (h *mapAssetHandler) UpdateMapAsset(w http.ResponseWriter, r *http.Request) {
	var in updateMapAssetIn
	if !decodeWithNumbers(w, r, &in) {
		return
	}
	if in.Verified != nil {
		writeVerifiedNotHere(w)
		return
	}
	custom, ok := customValuesIn(w, in.CustomValues)
	if !ok {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.MapAssets opens db.For(ctx).Tx.
	row, written, err := h.d.Writer.Update(r.Context(), r.PathValue("id"), app.MapAssetPatch{
		AssetTypeCode: in.AssetTypeCode, Name: in.Name, Address: in.Address, ResidentialUnitID: in.ResidentialUnitID,
		Lat: in.Lat, Lng: in.Lng, Representative: in.Representative, Phone: in.Phone, Status: in.Status,
		TaxCode: in.TaxCode, IndustryCode: in.IndustryCode, EmployeeCount: in.EmployeeCount,
		EstablishedOn: in.EstablishedOn, Description: in.Description, CustomValues: custom,
	}, actor)
	if err != nil {
		h.writeError(w, r, "sửa", err)
		return
	}
	// UNMASKED ONLY WHEN THE EDIT WROTE: its own entry, same transaction, names the officer. A no-op
	// (`{}`, or every value unchanged) wrote no entry — answering it unmasked would be a full read of the
	// representative, phone and tax code with no trail (rule 6, invariant 7), reachable by sending `{}`.
	// The form treats `masked: true` as "do not send these fields back", so a mask is never saved over
	// the real value.
	vietJSON(w, http.StatusOK, mapAssetToOut(row, written))
}

// DeleteMapAsset — DELETE /api/v1/map-assets/{id}. Soft delete; 204, no body.
func (h *mapAssetHandler) DeleteMapAsset(w http.ResponseWriter, r *http.Request) {
	var in deleteMapAssetIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.MapAssets opens db.For(ctx).Tx.
	if err := h.d.Writer.Delete(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeError(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetMapAssetConfirmation — POST /api/v1/map-assets/{id}/confirmation
func (h *mapAssetHandler) SetMapAssetConfirmation(w http.ResponseWriter, r *http.Request) {
	var in mapAssetConfirmationIn
	if !docThan(w, r, &in) {
		return
	}
	if in.Verified == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thiếu `verified` (true để xác minh, false để bỏ xác minh).", "")
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.MapAssets opens db.For(ctx).Tx.
	row, written, err := h.d.Writer.SetConfirmation(r.Context(), r.PathValue("id"), *in.Verified, actor)
	if err != nil {
		h.writeError(w, r, "xác minh", err)
		return
	}
	// Same rule as the PATCH: the same state twice wrote no entry, so it answers masked.
	vietJSON(w, http.StatusOK, mapAssetToOut(row, written))
}

// --- helpers -------------------------------------------------------------------------------------

func mapAssetToOut(a domain.MapAsset, full bool) mapAssetOut {
	out := mapAssetOut{
		ID: a.ID, AssetTypeCode: a.AssetTypeCode, Name: a.Name, Address: a.Address,
		ResidentialUnitID: a.ResidentialUnitID, Lat: a.Lat, Lng: a.Lng,
		Representative: a.Representative, Phone: a.Phone, Status: a.Status,
		Verified: a.Verified, VerifiedAt: a.VerifiedAt, VerifiedBy: a.VerifiedBy,
		TaxCode: a.TaxCode, IndustryCode: a.IndustryCode, EmployeeCount: a.EmployeeCount,
		EstablishedOn: a.EstablishedOn, Description: a.Description,
		CustomValues: make(map[string]any, len(a.CustomValues)),
		Masked:       !full, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
	for k, v := range a.CustomValues {
		out.CustomValues[k] = v
	}
	if !full {
		out.Representative = maskIfSet(a.Representative, privacy.MaskName)
		out.Phone = maskIfSet(a.Phone, privacy.MaskPhone)
		out.TaxCode = maskIfSet(a.TaxCode, privacy.MaskCccd)
	}
	return out
}

func maskIfSet(s string, mask func(string) string) string {
	if s == "" {
		return ""
	}
	return mask(s)
}

// decodeWithNumbers is docThan with numbers kept as json.Number inside `custom_values`, so an integer
// field never travels through float64 (2^53) on its way to its type check.
func decodeWithNumbers(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, thanToiDa)
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		// The decoder's own message quotes the input — never sent (rule 3, forbidden #3).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	return true
}

// customValuesIn re-encodes each custom value as raw JSON for the domain's type checks.
func customValuesIn(w http.ResponseWriter, in map[string]any) (map[string]json.RawMessage, bool) {
	if in == nil {
		return nil, true
	}
	out := make(map[string]json.RawMessage, len(in))
	for k, v := range in {
		b, err := json.Marshal(v)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "`custom_values` không hợp lệ.", "")
			return nil, false
		}
		out[k] = b
	}
	return out, true
}

func writeVerifiedNotHere(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
		"Không đặt `verified` ở đây — dùng POST /api/v1/map-assets/{id}/confirmation, để việc xác minh có vết riêng.", "")
}

func (h *mapAssetHandler) internal(ctx context.Context, w http.ResponseWriter, op string, err error) {
	// The wrapped error carries the store failure and never reaches the client (rule 3, forbidden #3);
	// it names a rule and a field, never a value.
	h.d.Log.Error("bản đồ kinh tế số: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// missingPrincipal — a write route reached with no principal carrying a business code. Rule 6 does not
// permit a write whose trail cannot name who made it — and there is NO fallback to the internal id.
func (h *mapAssetHandler) missingPrincipal(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến ghi bản đồ kinh tế số chạy mà không có chủ thể mang mã cán bộ",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// writeError maps a use-case failure onto a status. Listed explicitly, never a default 400: a database
// outage must not read as the client's fault.
func (h *mapAssetHandler) writeError(w http.ResponseWriter, r *http.Request, op string, err error) {
	var cv *domain.CustomValueError
	switch {
	case errors.Is(err, commsstore.ErrMapAssetNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy đối tượng này trên bản đồ của xã.", "")
	case errors.Is(err, commsstore.ErrMapAssetTaxCodeTaken):
		httpx.WriteError(w, http.StatusConflict, "tax_code_taken",
			"Mã số thuế này đã có ở một đối tượng khác trên bản đồ của xã. Hãy mở đối tượng đó để cập nhật.", "")
	case errors.Is(err, commsstore.ErrMapAssetTypeUnavailable):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "asset_type_unavailable",
			"Nhóm tài nguyên này không có trong danh mục của xã hoặc đã tắt. Hãy chọn nhóm khác, "+
				"hoặc nạp/bật nhóm ở Cấu hình → Danh mục.", "")
	case errors.As(err, &cv):
		h.d.Log.Info("bản đồ kinh tế số: từ chối "+op, "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_custom_values",
			fmt.Sprintf("Trường tuỳ biến `%s`: %s.", cv.FieldCode, cv.Err.Error()), "")
	default:
		if msg, ok := refusalMessage(mapAssetRefusals, err); ok {
			h.d.Log.Info("bản đồ kinh tế số: từ chối "+op, "xa", string(tenant.MustFrom(r.Context())), "err", err)
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", msg, "")
			return
		}
		h.internal(r.Context(), w, op, err)
	}
}

// mapAssetRefusals — the 400s of the write routes, one fixed sentence per domain sentinel. Bounds are
// read from domain, never retyped.
var mapAssetRefusals = []refusal{
	{domain.ErrMapAssetTypeEmpty, "Hãy chọn nhóm tài nguyên (`asset_type_code`)."},
	{domain.ErrMapAssetNameEmpty, "Tên đối tượng đang trống hoặc chứa ký tự không hợp lệ."},
	{domain.ErrMapAssetNameTooLong, fmt.Sprintf("Tên đối tượng quá dài (tối đa %d ký tự).", domain.MapAssetNameMaxLen)},
	{domain.ErrMapAssetAddressInvalid, "Địa chỉ chứa ký tự không hợp lệ."},
	{domain.ErrMapAssetAddressTooLong, fmt.Sprintf("Địa chỉ quá dài (tối đa %d ký tự).", domain.MapAssetAddressMaxLen)},
	{domain.ErrMapAssetResidentialUnitInvalid, "Mã thôn / tổ dân phố không đúng dạng."},
	{domain.ErrMapAssetLocationMissing, "Vị trí là bắt buộc: gửi cả `lat` và `lng` — không có toạ độ thì không lên bản đồ được."},
	{domain.ErrMapAssetLocationOutOfRange, "Toạ độ ngoài phạm vi: vĩ độ từ -90 đến 90, kinh độ từ -180 đến 180."},
	{domain.ErrMapAssetRepresentativeInvalid, "Tên người đại diện chứa ký tự không hợp lệ."},
	{domain.ErrMapAssetRepresentativeTooLong, fmt.Sprintf("Tên người đại diện quá dài (tối đa %d ký tự).", domain.MapAssetRepresentativeMaxLen)},
	{domain.ErrMapAssetPhoneShape, fmt.Sprintf("Số điện thoại chỉ gồm chữ số và các dấu + . - ( ) cùng dấu cách, "+
		"có từ %d đến %d chữ số, tối đa %d ký tự.",
		domain.ExternalContactPhoneMinDigits, domain.ExternalContactPhoneMaxDigits, domain.ExternalContactPhoneMaxLen)},
	{domain.ErrMapAssetStatusUnknown, "Trạng thái phải là dang-hoat-dong, tam-ngung hoặc da-giai-the."},
	{domain.ErrMapAssetTaxCodeShape, fmt.Sprintf("Mã số thuế chỉ gồm chữ số, có thể một dấu gạch nối (ví dụ 0101234567-001), tối đa %d ký tự.", domain.MapAssetTaxCodeMaxLen)},
	{domain.ErrMapAssetIndustryCodeShape, "Mã ngành nghề là mã VSIC cấp 2, đúng hai chữ số (ví dụ 47)."},
	{domain.ErrMapAssetEmployeeCountInvalid, "Số lao động phải là số nguyên không âm."},
	{domain.ErrMapAssetEstablishedOnInvalid, "Ngày thành lập phải có dạng YYYY-MM-DD."},
	{domain.ErrMapAssetDescriptionInvalid, "Mô tả chứa ký tự không hợp lệ."},
	{domain.ErrMapAssetDescriptionTooLong, fmt.Sprintf("Mô tả quá dài (tối đa %d ký tự).", domain.MapAssetDescriptionMaxLen)},
	{domain.ErrMapAssetCustomValuesTooLarge, "Giá trị tuỳ biến quá lớn."},
	{domain.ErrThieuLyDoXoa, "Hãy nhập lý do xoá. Đối tượng trên bản đồ là hồ sơ của xã, xoá phải ghi rõ vì sao."},
	{domain.ErrLyDoXoaQuaDai, fmt.Sprintf("Lý do xoá quá dài (tối đa %d ký tự).", domain.LyDoXoaToiDa)},
}
