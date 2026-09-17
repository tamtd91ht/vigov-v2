package http

import (
	"net/http"

	"github.com/vihat/vigov/core/tenant"
)

// The read route behind the sign-in screen. GET /api/v1/commune
//
// It is the ONE public route in this service that returns data, and every decision below exists
// because of that. Anything it returns is readable by anybody who can reach the domain.

// thongTinXa is the commune as the sign-in screen sees it.
//
// THERE IS NO `id` FIELD, AND ADDING ONE IS THE MISTAKE THIS COMMENT EXISTS TO PREVENT. The
// value the handler reads it from — tenant.Tenant — carries `ID` right next to `Name`, so the
// field will be sitting there in front of whoever edits this next. It stays out because:
//
//   - the web does not need it. The commune is derived server-side from `Host` on EVERY request
//     (rule 1, invariant 3), so a client has nothing to do with the value;
//   - a client that sent one back would be ignored anyway — naming your own commune is granting
//     yourself access (rule 1, forbidden #2). So it can only ever be decoration;
//   - it is the opaque identifier the whole isolation model rests on, and it is what prefixes
//     cache keys, queue messages, realtime rooms and file paths (rule 1, invariant 7).
//     Publishing it on an UNAUTHENTICATED route hands that prefix to anyone with curl, in
//     exchange for nothing.
//
// THERE IS NO `active` FIELD EITHER, and that is a different reason: it could only ever be
// `true` here. httpx.TenantMiddleware answers 404 for a deactivated commune before any handler
// runs (pkg/httpx/edge.go:25), so this code is unreachable for one. A field that is constant by
// construction invites the web to build a "this commune has merged" branch that never executes —
// and a merged commune IS a real state (rule 1, invariant 6: marked inactive, never deleted).
// Answering it properly means the edge saying something other than a bare 404, which is a change
// in pkg/httpx that every service shares. STATED, not silently half-done here.
//
// THERE IS NO `parent_authority` FIELD, although the admin web already asks for one
// (web-admin/src/components/cau-hinh-xa.tsx reads `parentAuthority` — "Thành phố Đà
// Nẵng" under the commune name, docs/ui-ux/15 §3). THE DATA DOES NOT EXIST: pkg/tenant.Tenant is
// {ID, Host, Name, Active} and `message Tenant` in proto/vigov/platform/v1 is {id, host,
// display_name}. Adding it is a change to the contract BETWEEN services, which is the
// contract-designer's surface and is generated from .proto, never written by hand (rule 2,
// invariant 7). Returning "" would be worse than returning nothing: the screen would print an
// empty line under the commune name and nobody would know whether the authority has no parent or
// the field was never filled in.
type thongTinXa struct {
	// Name is the display name AT THIS MOMENT, not an identifier — a commune can be renamed by
	// an administrative reorganisation while its tenant_id stays put, which is the whole point of
	// the id being opaque (rule 1, invariant 2).
	Name string `json:"name"`

	// Host is the commune's canonical domain as the registry holds it. It is what the caller
	// should be on; it is not necessarily the string the caller typed.
	Host string `json:"host"`
}

// ThongTinXa serves the commune this request's Host resolves to. GET /api/v1/commune
//
// THE COMMUNE IS NEVER TAKEN FROM THE REQUEST. It is the one the EDGE resolved from `Host` and
// put in the context; the query string, the body and any client header are not read here at all,
// and pkg/httpx.StripTenantHeaders has already removed the latter (rule 1, forbidden #2).
//
// NO AUDIT ENTRY: nothing is written, nothing personal is read, and nothing crosses a commune —
// the two cases rule 6, invariant 7 asks for. An entry per hit on the sign-in screen would bury
// the entries that carry legal weight under noise from unauthenticated traffic.
func (h *Handler) ThongTinXa(w http.ResponseWriter, r *http.Request) {
	// MỘT LẦN PHÂN GIẢI, VÀ NÓ ĐÃ XẢY RA Ở BIÊN.
	//
	// Bản trước của hàm này hỏi thư mục xã lần thứ hai, vì context chỉ mang `tenant_id` còn
	// tuyến này cần TÊN xã. Nó phải chép lại phép chuẩn hoá Host của biên, rồi phải so hai
	// kết quả và trả 503 khi chúng lệch — ba thứ chỉ tồn tại để rào một câu hỏi lẽ ra không
	// nên được hỏi hai lần. Biên nay mang cả tenant.Tenant (core/httpx/edge.go), nên cả ba
	// biến mất cùng lúc: không còn bản sao quy tắc, không còn hai câu trả lời để lệch nhau,
	// và không còn một nhánh 503 mà không bài test nào chạm tới trong đời thật.
	//
	// MustCurrent panic khi thiếu, và đó là chủ ý: nếu handler này chạy ngoài biên HTTP thì
	// nó sẽ hiển thị tên rỗng — một cơ quan nhà nước hiện sai tên mình, không có gì đỏ.
	xa := tenant.MustCurrent(r.Context())
	vietJSON(w, http.StatusOK, thongTinXa{Name: xa.Name, Host: xa.Host})
}
