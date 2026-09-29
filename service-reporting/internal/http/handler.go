package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
)

// Handler serves the reporting routes. It holds no business logic: the use cases in internal/app
// own that, including the transaction an audit entry has to share with its business write
// (rule 6, invariant 3).
type Handler struct {
	d Deps
}

func NewHandler(d Deps) *Handler {
	// slog.Default() rather than a nil check at every call site. A handler that logged nowhere
	// would swallow the one line an operator gets when a commune's write fails.
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Handler{d: d}
}

// writeJSON writes one JSON body. The same four lines as finance's and petitions' helper, COPIED
// rather than imported: reaching into another service's internal/ breaks the boundary at compile
// time (rule 2, forbidden #1), and four lines are not what core/ is for.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// maxBodyBytes bounds the request body. 64 KiB is far past any body here and far short of anything
// worth streaming: an unbounded body is memory a client chooses.
const maxBodyBytes = 64 << 10

// decodeBody decodes a JSON body, answering 400 itself on failure.
//
// THE DECODER'S OWN MESSAGE NEVER REACHES THE CLIENT: it quotes the offending input (rule 3,
// forbidden #3). The sentence returned says what to fix without repeating what was sent.
func decodeBody(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	return true
}

// actorFrom builds the audit actor from the request.
//
// THE TRAIL RECORDS THE BUSINESS CODE `Ma`, NEVER THE INTERNAL `ID` (rule 6, invariant 8): `CB-00123`
// names a person to whoever reads the entry years later, a ULID names nobody. AN EMPTY `Ma` REFUSES
// THE WRITE and never falls back to p.ID — a fallback would put internal ids back into the column
// silently, one deployment window at a time.
//
// THE IP COMES FROM httpx.ClientIP: the socket address, or the client address a trusted proxy named
// (TRUSTED_PROXY_CIDRS, applied at the edge in cmd/server) — never one the caller chose.
func actorFrom(r *http.Request) (audit.Actor, bool) {
	p, ok := authz.From(r.Context())
	if !ok || p.Ma == "" {
		return audit.Actor{}, false
	}
	return audit.Actor{ID: p.Ma, Kind: p.Kind, IP: httpx.ClientIP(r)}, true
}
