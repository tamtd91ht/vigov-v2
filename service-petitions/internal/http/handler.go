package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Handler serves the petitions routes. It holds NO business logic: the use cases in internal/app
// own that, including the transaction a business write shares with its audit entry (rule 6,
// invariant 3). What lives here is HTTP translation and nothing else.
type Handler struct {
	d Deps
}

func NewHandler(d Deps) *Handler {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Handler{d: d}
}

// vietJSON writes the response body. One function, so the content type and the encoder cannot
// differ between two routes of one service.
func vietJSON(w http.ResponseWriter, status int, than any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(than)
}
