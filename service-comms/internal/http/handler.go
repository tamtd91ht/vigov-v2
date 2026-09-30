package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Handler serves the comms routes. It holds no business logic: the use cases in internal/app own
// that, including the transaction an audit entry shares with its write (rule 6).
//
// There is no use case behind the one route mounted today — it is a read of a reference catalogue,
// so the handler talks to the store interface declared in routes.go and does nothing else.
type Handler struct {
	d Deps
}

func NewHandler(d Deps) *Handler {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Handler{d: d}
}

func vietJSON(w http.ResponseWriter, status int, than any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(than)
}
