package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Handler serves the finance routes. It holds no business logic: the use cases in internal/app
// own that, including the transaction an audit entry has to share with its business write
// (rule 6, invariant 3). Nothing here writes, so nothing here opens a transaction.
type Handler struct {
	d Deps
}

func NewHandler(d Deps) *Handler {
	// slog.Default() rather than a nil check at every call site. A handler that logged nowhere
	// would swallow the one line an operator gets when a commune's catalogue is refused.
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Handler{d: d}
}

// nay is the clock every derived figure is computed against.
//
// WHY A SEAM AND NOT time.Now() AT THE CALL SITE: the disbursement screen's "điểm chậm" is a
// comparison between how much of the budget year has gone and how much money has moved (§3). That
// arithmetic is at its most fragile on the first and last days of a year, and a handler reading
// the wall clock directly cannot be tested on either of those days — so the one case that matters
// would be the one case never exercised. Deps.Nay is nil in production and this returns the real
// clock; a test sets it to a fixed instant.
func (h *Handler) nay() time.Time {
	if h.d.Nay == nil {
		return time.Now()
	}
	return h.d.Nay()
}

// vietJSON writes one JSON body. Same shape as the identity service's helper of the same name,
// deliberately COPIED rather than imported: reaching into service-identity/internal/ breaks the
// service boundary at compile time (rule 2, forbidden #1), and four lines are not what core/ is
// for.
func vietJSON(w http.ResponseWriter, status int, than any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(than)
}
