package http

// The routes behind `Cấu hình → Trường bản đồ` (docs/ui-ux/14-cau-hinh.md §6):
// GET · POST /api/v1/map-field-schemas, PATCH · DELETE /api/v1/map-field-schemas/{id}.
//
// `map-field-schemas` IS A VENDOR-CHOSEN NOUN. kb/00-foundation/ubiquitous-language.md §Tên tài
// nguyên trên URL has no row for this concept; the noun follows the entity (`MapFieldSchema`,
// migration 0007) and the sibling `map-asset-types`. Filling that row belongs to its owner.
//
// NO ASSET REGISTER EXISTS YET, so these fields describe values nobody can store. See
// internal/app/map_field_schema.go for what that changes about each rule.
//
// THE COMMUNE IS NEVER HANDLED HERE. Every call below passes the request context, and the store
// behind it reaches the database only through db.For(ctx) — tenant_id is $1 of every statement
// (rule 1, invariants 4 and 5).

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// fieldOptionOut / fieldOptionIn — one entry of a `chon` list.
type fieldOptionOut struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type fieldOptionIn struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// mapFieldSchemaOut is one field as it leaves the API. Nothing here is personal data (rule 3): it
// describes a form, not a person.
type mapFieldSchemaOut struct {
	ID            string `json:"id"`
	AssetTypeCode string `json:"asset_type_code"` // `code` of a row of GET /api/v1/map-asset-types
	// FieldCode is `Mã trường` ("legal_form"). NOT `field_key`, the name ../vigov-require uses:
	// tools/apidoc refuses a response field whose name holds the word `key` unless it is declared a
	// customer-approved credential, and this is no credential. `code` is also this repository's
	// word for an issued `ma` (map-asset-types answers `code`).
	FieldCode string `json:"field_code"`
	Label     string `json:"label"`
	// ValueType is one of van-ban · so-nguyen · so-thap-phan · dung-sai · ngay · chon (ADR 0011:
	// values Vietnamese without diacritics). The screen labels them Văn bản · Số nguyên ·
	// Số thập phân · Đúng/Sai · Ngày · Chọn trong danh sách.
	ValueType  string           `json:"value_type"`
	Options    []fieldOptionOut `json:"options"` // [] unless value_type is `chon`, never null
	IsRequired bool             `json:"is_required"`
	SortOrder  int              `json:"sort_order"`
	// IsActive false is `Tắt`: still listed, can be re-enabled. A deleted field is not listed.
	IsActive bool `json:"is_active"`
}

// mapFieldSchemaListOut wraps the list in an object; no cursor — the whole list or a refusal, like
// map-asset-types.
type mapFieldSchemaListOut struct {
	Items []mapFieldSchemaOut `json:"items"`
}

func mapFieldSchemaToOut(m domain.MapFieldSchema) mapFieldSchemaOut {
	opts := make([]fieldOptionOut, 0, len(m.Options))
	for _, o := range m.Options {
		opts = append(opts, fieldOptionOut{Value: o.Value, Label: o.Label})
	}
	return mapFieldSchemaOut{
		ID: m.ID, AssetTypeCode: m.AssetTypeCode, FieldCode: m.FieldCode, Label: m.Label,
		ValueType: m.ValueType, Options: opts, IsRequired: m.IsRequired,
		SortOrder: m.SortOrder, IsActive: m.IsActive,
	}
}

func optionsFromIn(in []fieldOptionIn) []domain.FieldOption {
	out := make([]domain.FieldOption, 0, len(in))
	for _, o := range in {
		out = append(out, domain.FieldOption{Value: o.Value, Label: o.Label})
	}
	return out
}

// createMapFieldSchemaIn is the body of POST. `omitempty` on the optional fields so tools/apidoc
// does not mark them required.
type createMapFieldSchemaIn struct {
	AssetTypeCode string          `json:"asset_type_code"`
	FieldCode     string          `json:"field_code"`
	Label         string          `json:"label"`
	ValueType     string          `json:"value_type"`
	Options       []fieldOptionIn `json:"options,omitempty"`
	IsRequired    bool            `json:"is_required,omitempty"`
	SortOrder     int             `json:"sort_order,omitempty"`
}

// updateMapFieldSchemaIn is the body of PATCH. Every editable field is a pointer: sort_order 0,
// is_required false and is_active false are real values, and a dialog editing only the label must
// not reset them.
//
// `options` IS THE WHOLE NEW LIST, not a delta: it must keep every value the row has (relabel and
// append only). A full list makes the PATCH idempotent — sending it twice changes nothing.
//
// `asset_type_code`, `field_code` AND `value_type` ARE HERE ONLY TO BE REFUSED — all three are
// immutable, and a client that sends them back is told so rather than left to assume it could.
type updateMapFieldSchemaIn struct {
	Label      *string          `json:"label,omitempty"`
	Options    *[]fieldOptionIn `json:"options,omitempty"`
	IsRequired *bool            `json:"is_required,omitempty"`
	SortOrder  *int             `json:"sort_order,omitempty"`
	IsActive   *bool            `json:"is_active,omitempty"`

	AssetTypeCode *string `json:"asset_type_code,omitempty"`
	FieldCode     *string `json:"field_code,omitempty"`
	ValueType     *string `json:"value_type,omitempty"`
}

// deleteMapFieldSchemaIn — the reason is mandatory (rule 7, invariant 1) and travels in the body,
// never the query string, where free text would land in every access log.
type deleteMapFieldSchemaIn struct {
	Reason string `json:"reason"`
}

// ListMapFieldSchemas — GET /api/v1/map-field-schemas[?asset_type_code=<code>]
//
// No audit entry: a configuration list read inside the commune it belongs to (rule 6, invariant 7
// audits full personal data and cross-commune reads, neither of which this is).
func (h *Handler) ListMapFieldSchemas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	typeCode, err := assetTypeFilter(r.URL.Query())
	if err != nil {
		// Not echoed: the message names the filter, not what was sent.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Mã nhóm tài nguyên dùng để lọc không đúng dạng.", "")
		return
	}

	// Scoped in the store: MapFieldSchemaStore.List reads through db.For(ctx).Query.
	list, err := h.d.MapFieldSchemas.List(ctx, typeCode)
	if err != nil {
		if errors.Is(err, commsstore.ErrTooManyMapFieldSchemas) {
			h.d.Log.Error("trường bản đồ vượt trần — TỪ CHỐI thay vì cắt bớt",
				"xa", string(tenant.MustFrom(ctx)), "tran", commsstore.MapFieldSchemaCeiling)
		} else {
			h.d.Log.Error("trường bản đồ: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		}
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	out := mapFieldSchemaListOut{Items: make([]mapFieldSchemaOut, 0, len(list))}
	for _, m := range list {
		out.Items = append(out.Items, mapFieldSchemaToOut(m))
	}
	writeJSON(w, http.StatusOK, out)
}

// assetTypeFilter reads the OPTIONAL `asset_type_code` filter: absent or empty means every group,
// present means it must be a well-formed code.
//
// A SEPARATE FUNCTION BECAUSE OF THE CONTRACT, not for reuse. tools/apidoc marks a query parameter
// `required` when it feeds the condition of an `if` that answers 400 in the handler
// (tools/apidoc/truyvan.go, docThanHam). Validated inline, this optional filter was published as
// REQUIRED, and a client built from openapi.json would have had to send a filter the server never
// asks for. Here the 400 branch depends on an error returned from a call, which the generator does
// not trace, so the parameter comes out `required: false` — which is what the handler does. That
// leans on a LIMIT of the generator, not on a rule it enforces: inline the check again and the
// contract goes back to `required: true` with no test turning red.
func assetTypeFilter(q url.Values) (string, error) {
	raw := queryParam(q, "asset_type_code")
	if raw == "" {
		return "", nil
	}
	return domain.NormalizeCode(raw)
}

// CreateMapFieldSchema — POST /api/v1/map-field-schemas
func (h *Handler) CreateMapFieldSchema(w http.ResponseWriter, r *http.Request) {
	var in createMapFieldSchemaIn
	if !decodeBody(w, r, &in) {
		return
	}
	actor, ok := actorFrom(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.MapFieldSchemas opens db.For(ctx).Tx.
	row, err := h.d.WriteMapFieldSchemas.Create(r.Context(), app.CreateMapFieldRequest{
		AssetTypeCode: in.AssetTypeCode,
		FieldCode:     in.FieldCode,
		Label:         in.Label,
		ValueType:     in.ValueType,
		Options:       optionsFromIn(in.Options),
		IsRequired:    in.IsRequired,
		SortOrder:     in.SortOrder,
	}, actor)
	if err != nil {
		h.writeMapFieldSchemaError(w, r, "thêm", err)
		return
	}
	// The business address, not the body: the body would go into Redis, a cache, not a record store.
	idem.RecordCode(r.Context(), row.Subject())
	writeJSON(w, http.StatusCreated, mapFieldSchemaToOut(row))
}

// UpdateMapFieldSchema — PATCH /api/v1/map-field-schemas/{id}
func (h *Handler) UpdateMapFieldSchema(w http.ResponseWriter, r *http.Request) {
	var in updateMapFieldSchemaIn
	if !decodeBody(w, r, &in) {
		return
	}
	switch {
	case in.AssetTypeCode != nil:
		h.writeMapFieldSchemaError(w, r, "sửa", domain.ErrAssetTypeImmutable)
		return
	case in.FieldCode != nil:
		h.writeMapFieldSchemaError(w, r, "sửa", domain.ErrFieldCodeImmutable)
		return
	case in.ValueType != nil:
		h.writeMapFieldSchemaError(w, r, "sửa", domain.ErrValueTypeImmutable)
		return
	}
	actor, ok := actorFrom(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	req := app.UpdateMapFieldRequest{
		Label: in.Label, IsRequired: in.IsRequired, SortOrder: in.SortOrder, IsActive: in.IsActive,
	}
	if in.Options != nil {
		opts := optionsFromIn(*in.Options)
		req.Options = &opts
	}
	// Scoped in the use case: app.MapFieldSchemas opens db.For(ctx).Tx.
	row, err := h.d.WriteMapFieldSchemas.Update(r.Context(), r.PathValue("id"), req, actor)
	if err != nil {
		h.writeMapFieldSchemaError(w, r, "sửa", err)
		return
	}
	writeJSON(w, http.StatusOK, mapFieldSchemaToOut(row))
}

// DeleteMapFieldSchema — DELETE /api/v1/map-field-schemas/{id}. Soft delete; 204, no body.
func (h *Handler) DeleteMapFieldSchema(w http.ResponseWriter, r *http.Request) {
	var in deleteMapFieldSchemaIn
	if !decodeBody(w, r, &in) {
		return
	}
	actor, ok := actorFrom(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.MapFieldSchemas opens db.For(ctx).Tx.
	if err := h.d.WriteMapFieldSchemas.Delete(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.writeMapFieldSchemaError(w, r, "xoá", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// missingPrincipal — a write route reached with no principal carrying a business code. The route
// sits behind RequirePermission, so this is a wiring fault or an identity older than `ma`; either
// way rule 6 does not permit a write whose trail cannot name who made it.
func (h *Handler) missingPrincipal(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến ghi trường bản đồ chạy mà không có chủ thể mang mã cán bộ",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// writeMapFieldSchemaError maps a use-case failure onto a status. 409 for a rule about the data
// (the caller holds the permission; this row or this state refuses), 400 for a rule about the
// request, 404 for a row not in this commune, 500 for everything not listed — never a default 400.
func (h *Handler) writeMapFieldSchemaError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, commsstore.ErrMapFieldSchemaNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy trường bản đồ này.", "")
	case errors.Is(err, commsstore.ErrAssetTypeMissing):
		httpx.WriteError(w, http.StatusConflict, "asset_type_missing",
			"Nhóm tài nguyên đã chọn không có trong danh mục loại tài nguyên bản đồ của xã. "+
				"Hãy thêm nhóm ở tab Danh mục trước.", "")
	case errors.Is(err, commsstore.ErrFieldCodeTaken):
		httpx.WriteError(w, http.StatusConflict, "field_code_taken",
			"Nhóm này đã có một trường mang mã này. Hãy chọn mã khác.", "")
	case errors.Is(err, commsstore.ErrFieldCodeRetired):
		// Says WHY a key nowhere on the screen is taken — without it this reads as a bug.
		httpx.WriteError(w, http.StatusConflict, "field_code_retired",
			"Mã này đã dùng cho một trường đã xoá của nhóm. Mã đã cấp thì không cấp lại, vì dữ liệu "+
				"cũ vẫn lưu theo mã đó. Hãy chọn mã khác.", "")
	case errors.Is(err, commsstore.ErrMapFieldSchemaFull):
		httpx.WriteError(w, http.StatusConflict, "catalogue_full",
			"Xã đã đạt số trường bản đồ tối đa. Hãy xoá bớt trường không dùng.", "")
	case errors.Is(err, domain.ErrOptionRemoved):
		h.logRefusal(r, "trường bản đồ: từ chối "+op, err)
		httpx.WriteError(w, http.StatusConflict, "option_removed",
			"Không bỏ được một lựa chọn đã có. Có thể đổi nhãn hoặc thêm lựa chọn mới.", "")
	default:
		if msg, ok := refusalMessage(mapFieldSchemaRefusals, err); ok {
			h.logRefusal(r, "trường bản đồ: từ chối "+op, err)
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", msg, "")
			return
		}
		// The wrapped error never reaches the client (rule 3, forbidden #3).
		h.d.Log.Error("trường bản đồ: "+op+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// refusal pairs one domain sentinel with the sentence staff read when it refuses their request.
//
// WRITTEN HERE AND NOT READ FROM THE SENTINEL. The domain's sentences are for the error chain and
// the log: they start with the package prefix (`truong_ban_do:`, `may_chu_thu:`), quote the JSON
// field name in backticks, and by the time they reach this layer may carry a wrapped detail. The
// screen prints `message` verbatim, so the wire sentence must be one nobody has to strip — the
// same split as service-petitions/internal/http/xu_ly_phan_anh.go cacCauTuChoiPhieu. The price is
// two sentences per refusal; each table sits beside the one switch that uses it, and every row is
// pinned by a test asserting no prefix and no backtick reaches the body.
//
// THE TABLE IS ALSO THE LIST OF 400s. A sentinel missing from it falls through to 500, never to a
// default 400 — see isInputError for why "unknown means the client's fault" is wrong.
type refusal struct {
	err     error
	message string
}

// refusalMessage returns the fixed sentence for the first row whose sentinel is in err's chain.
func refusalMessage(table []refusal, err error) (string, bool) {
	for _, row := range table {
		if errors.Is(err, row.err) {
			return row.message, true
		}
	}
	return "", false
}

// logRefusal records a refusal with its full chain — the chain goes to the log and nowhere else.
// INFO and not ERROR: the rule doing its job is not a fault anybody must be woken for. The chain
// names a rule and a field, never a value the person typed.
func (h *Handler) logRefusal(r *http.Request, msg string, err error) {
	h.d.Log.Info(msg, "xa", string(tenant.MustFrom(r.Context())), "err", err)
}

// mapFieldSchemaRefusals — the 400s of the four map-field routes. Field names are the labels of
// docs/ui-ux/14-cau-hinh.md §6 (`Nhóm tài nguyên`, `Mã trường`, `Nhãn hiển thị`, `Kiểu dữ liệu`,
// `Thứ tự`). The bounds are read from domain, never retyped, so a bound that changes changes the
// sentence too.
//
// ErrMa* HERE IS THE ASSET GROUP'S CODE: the only catalogue code these routes take is
// `asset_type_code` (app.MapFieldSchemas.Create normalises it with domain.ChuanHoaMa).
var mapFieldSchemaRefusals = []refusal{
	{domain.ErrCodeEmpty, "Chưa chọn nhóm tài nguyên."},
	{domain.ErrCodeShape, "Mã nhóm tài nguyên không đúng dạng: chỉ gồm chữ thường a-z, số và dấu gạch ngang."},
	{domain.ErrCodeTooLong, fmt.Sprintf("Mã nhóm tài nguyên quá dài (tối đa %d ký tự).", domain.CodeMaxLen)},

	{domain.ErrFieldCodeEmpty, "Chưa nhập mã trường."},
	{domain.ErrFieldCodeShape, "Mã trường không đúng dạng: bắt đầu bằng chữ thường a-z, chỉ gồm chữ thường a-z, " +
		"số và dấu gạch dưới, ví dụ legal_form."},
	{domain.ErrFieldCodeTooLong, fmt.Sprintf("Mã trường quá dài (tối đa %d ký tự).", domain.FieldCodeMaxLen)},
	// "trống hoặc chứa ký tự không hợp lệ": domain.normalizeText answers the EMPTY sentinel for a
	// control character too, so "chưa nhập" alone would be false for a label that has text.
	{domain.ErrFieldLabelEmpty, "Nhãn hiển thị đang trống hoặc chứa ký tự không hợp lệ."},
	{domain.ErrFieldLabelTooLong, fmt.Sprintf("Nhãn hiển thị quá dài (tối đa %d ký tự).", domain.FieldLabelMaxLen)},
	{domain.ErrValueTypeUnknown, "Kiểu dữ liệu không hợp lệ. Hãy chọn một trong: Văn bản, Số nguyên, " +
		"Số thập phân, Đúng/Sai, Ngày, Chọn trong danh sách."},
	{domain.ErrSortOrderRange, fmt.Sprintf("Thứ tự phải từ 0 đến %d.", domain.FieldSortOrderMax)},

	{domain.ErrOptionsRequired, "Kiểu Chọn trong danh sách cần ít nhất một lựa chọn."},
	{domain.ErrOptionsNotAllowed, "Chỉ kiểu Chọn trong danh sách mới có danh sách lựa chọn."},
	{domain.ErrOptionsTooMany, fmt.Sprintf("Quá nhiều lựa chọn (tối đa %d).", domain.FieldOptionsMax)},
	{domain.ErrOptionValueEmpty, "Một lựa chọn đang trống giá trị hoặc giá trị chứa ký tự không hợp lệ."},
	{domain.ErrOptionValueTooLong, fmt.Sprintf("Giá trị của một lựa chọn quá dài (tối đa %d ký tự).",
		domain.FieldOptionValueMaxLen)},
	{domain.ErrOptionLabelEmpty, "Một lựa chọn đang trống nhãn hoặc nhãn chứa ký tự không hợp lệ."},
	{domain.ErrOptionLabelTooLong, fmt.Sprintf("Nhãn của một lựa chọn quá dài (tối đa %d ký tự).",
		domain.FieldLabelMaxLen)},
	{domain.ErrOptionValueDup, "Có hai lựa chọn trùng giá trị. Mỗi lựa chọn cần một giá trị riêng."},

	{domain.ErrAssetTypeImmutable, "Không đổi được nhóm tài nguyên của trường đã có. Hãy thêm trường mới ở nhóm khác."},
	{domain.ErrFieldCodeImmutable, "Không đổi được mã trường. Hãy sửa nhãn hiển thị, hoặc thêm trường mới."},
	{domain.ErrValueTypeImmutable, "Không đổi được kiểu dữ liệu: dữ liệu đã ghi theo kiểu cũ sẽ không đọc " +
		"được. Hãy thêm trường mới."},

	{domain.ErrDeleteReasonMissing, "Chưa nhập lý do xoá."},
	{domain.ErrDeleteReasonTooLong, fmt.Sprintf("Lý do xoá quá dài (tối đa %d ký tự).", domain.DeleteReasonMaxLen)},
}
