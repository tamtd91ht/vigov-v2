package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Handler serves the documents routes. It holds no business logic: the use cases in internal/app
// own that, including the transaction an audit entry shares with the write it records (rule 6).
// The routes here are reads, so none of them opens one.
type Handler struct {
	d Deps
}

func NewHandler(d Deps) *Handler {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Handler{d: d}
}

// vietJSON writes the one success shape. Failures go through httpx.WriteError, which is the single
// error shape for the whole system — a handler inventing a second one is what makes a client carry
// two branches for one outcome.
func vietJSON(w http.ResponseWriter, status int, than any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(than)
}
