package http

import (
	"net/http"

	"github.com/vihat/vigov/core/tenant"
)

// The read route behind the sign-in screen. GET /api/v1/communes/current
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
// THERE IS A `province` FIELD, AND THE PARAGRAPH THAT USED TO STAND HERE ARGUING AGAINST ONE IS
// GONE BECAUSE IT BECAME UNTRUE, not because anybody overruled it. It read "THE DATA DOES NOT
// EXIST": tenant.Tenant was {ID, Host, Name, Active} and `message Tenant` was {id, host,
// display_name}. Both now carry the province — `tenant.tinh_thanh` in the registry (service-
// platform/migrations/0001_init.sql:66), `Tenant.province` on the wire. A comment that describes a
// shape the code no longer has is worse than no comment: it is read as current and is believed.
//
// THE NAME IS THE DATA'S, NOT THE SCREEN'S. The admin header prints this value on the line the
// design calls "cơ quan cấp trên" and the web names its own field `parentAuthority`
// (docs/ui-ux/15 §3). The two coincide today and are not the same concept: a province is an
// administrative FACT about where the commune is; a superior authority is a claim about WHO it
// answers to. Vietnam reorganises commune-level units periodically, and the day those diverge, a
// field named after the screen label would be carrying something it no longer means. The argument
// is made in full in proto/vigov/platform/v1/platform.proto, which owns it — this is a pointer,
// not a second copy (rule 9).
//
// "" IS RETURNED FOR A COMMUNE THAT HAS NOT DECLARED A PROVINCE, and the old reasoning against
// that is replaced too. It said "" would be worse than nothing, because a reader could not tell
// "no parent" from "never filled in". That held while the field was ABSENT FROM THE CONTRACT; it
// does not hold now that the contract DECLARES "" with exactly one meaning — not declared. So the
// three ends agree and none of them improvises: the registry stores "", the server returns "", the
// screen renders NOTHING on that line. What no end may do is substitute a default province or
// guess one from the host — a guessed province printed under the name of a public authority is
// that authority stating something untrue about itself.
//
// IT IS SAFE ON A PUBLIC ROUTE, which on this route is a question that gets asked field by field.
// A commune's province is not personal data (rule 3), it is not an identifier anything is keyed by
// (rule 1, invariant 2 — only `id` is, and `id` is the field above that stays out), and it
// describes nothing about the commune's internal operation. It is also already implied by the
// domain the caller had to know to reach this route at all.
type thongTinXa struct {
	// Name is the display name AT THIS MOMENT, not an identifier — a commune can be renamed by
	// an administrative reorganisation while its tenant_id stays put, which is the whole point of
	// the id being opaque (rule 1, invariant 2).
	Name string `json:"name"`

	// Host is the commune's canonical domain as the registry holds it. It is what the caller
	// should be on; it is not necessarily the string the caller typed.
	Host string `json:"host"`

	// Province is the province or centrally-governed city the commune sits in. "" means the
	// commune has not declared one — render nothing, never a placeholder, never a guess.
	Province string `json:"province"`
}

// ThongTinXa serves the commune this request's Host resolves to. GET /api/v1/communes/current
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
	vietJSON(w, http.StatusOK, thongTinXa{Name: xa.Name, Host: xa.Host, Province: xa.Province})
}
