package http

// A commune's DEFAULT map frame in the operator area (ADR 0072 §"Sửa đổi 04/10/2026 (lần 2)", K1–K2):
// read with any ops.* key (ADR 0073 #1 — a pure read of commune metadata), written under
// `ops.tenant.manage` (ADR 0072 amendment 2, open item #3, decided 04/10/2026: the centre and radius
// are commune metadata, the same kind as the name that key already guards — no eighth key).
//
// The commune's OWN frame is service-comms'; this is only the fallback comms reads over gRPC when the
// commune has set none (K3). Nothing here reaches comms.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// MapFrameDefaultEditor is the default-frame store (*store.MapFrameDefaultStore). Both methods take the
// TARGET commune from ctx (rule 1 invariant 4); the write commits its audit entry in the same
// transaction.
type MapFrameDefaultEditor interface {
	OperatorMapFrameDefault(ctx context.Context) (domain.MapFrameDefault, bool, error)
	SetMapFrameDefault(ctx context.Context, next domain.MapFrameDefault, acknowledgedUnusual bool, reason string,
		by domain.OperatorActor) (domain.MapFrameDefault, bool, error)
}

// mapFrameDefaultView — the default frame of one commune, plus the hints the console's form needs.
// Without a default: configured=false and the hints only. Bounds is [minLng, minLat, maxLng, maxLat] —
// MapLibre's LngLatBoundsLike, longitude first — rounded outward to 6 decimals, the same formula and
// order comms answers for its own frame, so the console's preview is the box web-admin will apply.
type mapFrameDefaultView struct {
	Configured bool       `json:"configured"`
	CenterLat  *float64   `json:"center_lat,omitempty"`
	CenterLng  *float64   `json:"center_lng,omitempty"`
	RadiusKm   *float64   `json:"radius_km,omitempty"`
	Bounds     []float64  `json:"bounds,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	UpdatedBy  string     `json:"updated_by,omitempty"`

	// The hints (ADR 0072 K2). recommended and usual are owner-adjustable proposals; max is the hard
	// ceiling. Read from domain, never restated here.
	RecommendedRadiusKm float64    `json:"recommended_radius_km"`
	UsualRadiusKm       [2]float64 `json:"usual_radius_km"`
	MaxRadiusKm         float64    `json:"max_radius_km"`
}

// mapFrameDefaultBody — every number explicit: an absent one is a 400, never a silent 0 (which would
// read as a centre off the coast of Africa, refused anyway, or a radius the box refuses — but with the
// wrong message). acknowledged_unusual is the operator's confirmation of a radius outside the usual
// band (K2); absent = false.
type mapFrameDefaultBody struct {
	CenterLat           *float64 `json:"center_lat"`
	CenterLng           *float64 `json:"center_lng"`
	RadiusKm            *float64 `json:"radius_km"`
	AcknowledgedUnusual bool     `json:"acknowledged_unusual"`
	Reason              string   `json:"reason"`
}

func toMapFrameDefaultView(f domain.MapFrameDefault, configured bool) mapFrameDefaultView {
	v := mapFrameDefaultView{
		RecommendedRadiusKm: domain.MapFrameRecommendedRadiusKm,
		UsualRadiusKm:       [2]float64{domain.MapFrameUsualMinKm, domain.MapFrameUsualMaxKm},
		MaxRadiusKm:         domain.MapFrameMaxRadiusKm,
	}
	if !configured {
		return v
	}
	lat, lng, r, at := f.CenterLat, f.CenterLng, f.RadiusKm, f.UpdatedAt.UTC()
	b := f.Bounds()
	v.Configured = true
	v.CenterLat, v.CenterLng, v.RadiusKm, v.UpdatedAt = &lat, &lng, &r, &at
	v.Bounds = []float64{b.MinLng, b.MinLat, b.MaxLng, b.MaxLat}
	v.UpdatedBy = f.UpdatedBy
	return v
}

func (h *operatorHandlers) writeMapFrameError(w http.ResponseWriter, r *http.Request, err error) {
	type m struct {
		status     int
		code, text string
	}
	for target, v := range map[error]m{
		store.ErrCommuneNotFound: {http.StatusNotFound, "commune_not_found", msgCommuneNotFound},
		store.ErrCommuneInactive: {http.StatusConflict, "commune_inactive", "Xã đã ngừng hoạt động."},
		domain.ErrMapFrameCenterOutsideMainland: {http.StatusUnprocessableEntity, "center_outside_mainland",
			"Tâm khung phải nằm trong khung đất liền Việt Nam (vĩ độ 8,4–23,4; kinh độ 102,1–109,5)."},
		domain.ErrMapFrameRadiusOutOfRange: {http.StatusUnprocessableEntity, "radius_out_of_range",
			"Bán kính phải lớn hơn 0 và không quá 50 km."},
		domain.ErrMapFrameRadiusUnusualUnconfirmed: {http.StatusUnprocessableEntity, "radius_unusual_unconfirmed",
			"Bán kính lệch xa mức khuyến nghị (3–20 km). Xác nhận để lưu; người đặt chịu trách nhiệm về giá trị này."},
		domain.ErrReasonInvalid: {http.StatusUnprocessableEntity, "invalid_reason", "Cần ghi lý do (tối đa 500 ký tự)."},
	} {
		if errors.Is(err, target) {
			httpx.WriteError(w, v.status, v.code, v.text, "")
			return
		}
	}
	h.d.Log.ErrorContext(r.Context(), "khu vận hành: đọc/ghi khung bản đồ mặc định thất bại", "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", msgInternal, "")
}

// getMapFrameDefault answers the commune's default frame, or configured=false. An inactive commune is
// read like any other (rule 7 keeps its configuration).
func (h *operatorHandlers) getMapFrameDefault(w http.ResponseWriter, r *http.Request) {
	ctx, _, ok := targetCommune(w, r)
	if !ok {
		return
	}
	f, configured, err := h.d.MapFrames.OperatorMapFrameDefault(ctx)
	if err != nil {
		h.writeMapFrameError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toMapFrameDefaultView(f, configured))
}

// setMapFrameDefault creates or replaces the commune's default frame. Refused before the store, in this
// order: absent number (400); centre outside the mainland box, radius outside (0, 50] — both after
// rounding to the stored scales (422); a radius outside the usual 3–20 km band without
// acknowledged_unusual=true (422, K2 — enforced HERE, not only in the console, rule 5 forbidden #1);
// blank reason (422). Setting the values it already has changes nothing and writes no entry.
func (h *operatorHandlers) setMapFrameDefault(w http.ResponseWriter, r *http.Request) {
	ctx, _, ok := targetCommune(w, r)
	if !ok {
		return
	}
	var b mapFrameDefaultBody
	if !decodeBody(w, r, &b) {
		return
	}
	if b.CenterLat == nil || b.CenterLng == nil || b.RadiusKm == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", msgInvalidBody, "")
		return
	}
	next, err := domain.NormalizeMapFrameDefault(*b.CenterLat, *b.CenterLng, *b.RadiusKm)
	if err != nil {
		h.writeMapFrameError(w, r, err)
		return
	}
	if err := domain.CheckUnusualRadius(next.RadiusKm, b.AcknowledgedUnusual); err != nil {
		h.writeMapFrameError(w, r, err)
		return
	}
	reason, err := domain.ValidateReason(norm.NFC.String(b.Reason))
	if err != nil {
		h.writeMapFrameError(w, r, err)
		return
	}
	// The trail records the confirmation only when it was NEEDED: a true flag on a usual radius
	// confirmed nothing.
	acknowledged := b.AcknowledgedUnusual && domain.IsUnusualRadius(next.RadiusKm)
	out, _, err := h.d.MapFrames.SetMapFrameDefault(ctx, next, acknowledged, reason, actorOf(r))
	if err != nil {
		h.writeMapFrameError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toMapFrameDefaultView(out, true))
}
