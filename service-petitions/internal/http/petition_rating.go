package http

// The citizen rates their own petition — POST /api/v1/my-citizen-reports/{maTraCuu}/rating
// (ADR 0050 point 2). A CITIZEN route: authz.CitizenOnly + httpx.XaTuPhien, no RBAC (rule 5,
// invariant 6), identity and commune from the session only (rule 4, invariant 2; ADR 0022).
//
// TWO VALUES ARE TAKEN FROM THE REQUEST, THE STARS AND THE COMMENT, and nothing else. Whose petition,
// which commune, whether it reopens and what it reopens to are all decided behind this handler: a
// body naming any of them is dropped by encoding/json because no field here names them.
//
// NO STAFF ROUTE RECORDS A RATING ON A CITIZEN'S BEHALF — the owner's decision of 28/09/2026. The
// rating is the citizen's own verdict on the commune's work; an officer typing it in would be the
// commune grading itself.

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// CitizenRating records the citizen's rating. A SEPARATE INTERFACE, for the reason GuiPhanAnhCongDan
// is one: it opens a transaction and writes an audit entry inside it, which is a use case and not a
// store call. It takes the actor and no citizen identifier — the actor IS the owner.
type CitizenRating interface {
	Rate(ctx context.Context, ma string, req app.RatingRequest, citizen audit.Actor) (domain.PhieuPhanAnh, error)
}

// ratingInput is the body of POST …/{maTraCuu}/rating.
//
// `stars` WITHOUT omitempty — it is the act, so the published contract marks it required. An absent
// value decodes to 0, which is outside 1..5 and refused with the same sentence as 6.
type ratingInput struct {
	Stars int `json:"stars"`

	// Comment is OPTIONAL (omitempty keeps it optional in the contract). ⚠ CITIZEN FREE TEXT (rule 3):
	// never logged, never echoed in an error, never in the audit delta or on the event. At most
	// domain.RatingCommentMaxLen characters; blank after trim is treated as absent.
	Comment string `json:"comment,omitempty"`
}

// RatePetition serves POST /api/v1/my-citizen-reports/{maTraCuu}/rating.
//
// 200 WITH THE SAME SHAPE THE GET ANSWERS (phieuCuaToiRa), so the Mini App re-renders one screen from
// one type and sees the new status — `dang-xu-ly` after a reopening — and its own rating.
//
// ONE 404 FOR FOUR CAUSES — another citizen's code, another commune's, soft deleted, unknown — byte for
// byte the body the GET answers (rule 4, forbidden #2), through the same khongTimThay.
func (h *HandlerCongDan) RatePetition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var in ratingInput
	if !docThan(w, r, &in) {
		return
	}
	citizen, ok := h.congDanThucHien(r)
	if !ok {
		// A WIRING FAULT, 500 — authz.CitizenOnly refuses every request without a citizen principal
		// before this runs. The same reasoning as GuiPhieu.
		h.d.Log.Error("tuyến đánh giá phản ánh chạy mà không có danh tính trong phiên — thiếu " +
			"authz.CitizenOnly hoặc authz.CitizenPrincipal trên chuỗi rìa")
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	ma := r.PathValue("maTraCuu")
	if ma == "" {
		h.khongTimThay(w)
		return
	}

	p, err := h.d.Rating.Rate(ctx, ma, app.RatingRequest{Stars: in.Stars, Comment: in.Comment}, citizen)
	if err != nil {
		h.answerRatingError(w, r, err)
		return
	}

	// The retry with the same Idempotency-Key is told the lookup code, never the body (core/idem).
	idem.RecordCode(ctx, p.MaTraCuu)

	// LOGGED: the commune, the code and whether it reopened — NOT the stars' comment, NOT the citizen
	// id (rule 3). The code is the business identifier an operator is quoted.
	h.d.Log.Info("người dân đã đánh giá phản ánh",
		"xa", string(tenant.MustFrom(ctx)), "ma_tra_cuu", p.MaTraCuu,
		"mo_lai", p.TrangThai == domain.DangXuLy)

	nhan := ""
	if p.LinhVuc != "" {
		nhan, err = h.nhanCuaLinhVuc(ctx, p.LinhVuc)
		if err != nil {
			// THE RATING ALREADY COMMITTED. A 500 would tell the citizen it failed, and they would rate
			// again — the second time a 409 once the first reopened it. The label is cosmetic: the
			// response goes out without it, the same call traPhieu makes.
			h.d.Log.Error("đọc nhãn lĩnh vực sau khi đánh giá: lỗi hệ thống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			nhan = ""
		}
	}
	vietJSON(w, http.StatusOK, phieuCuaToiRaNgoai(p, nhan))
}

// ratingRefusals is the sentence a CITIZEN reads for each refusal. Written here and never read from
// the sentinel — the same reason cacCauTuChoiPhieu gives: the domain's sentences carry a package prefix
// and, wrapped by the use case, the commune id. The limits are read from domain, never retyped.
//
// "ông/bà" AS IN THE NOTIFICATIONS (domain.loiNhanChoDan) — this surface serves the shared app and the
// commune apps alike, and the neutral form is the one both already use.
var ratingRefusals = []struct {
	cause error
	text  string
}{
	{domain.ErrRatingStarsOutOfRange, fmt.Sprintf("Vui lòng chọn từ %d đến %d sao.",
		domain.RatingMinStars, domain.RatingMaxStars)},
	{domain.ErrRatingCommentTooLong, fmt.Sprintf("Nhận xét quá dài (tối đa %d ký tự).",
		domain.RatingCommentMaxLen)},
	{domain.ErrRatingNotOpen, "Phản ánh này chưa thể đánh giá: chỉ đánh giá được khi xã đã xử lý xong " +
		"và phản ánh chưa được đóng. Vui lòng tải lại để xem trạng thái hiện tại."},
	{petstore.ErrPhieuDaChuyenTrang, "Trạng thái phản ánh vừa thay đổi. Vui lòng tải lại rồi đánh giá lại."},
}

func ratingRefusalText(err error) string {
	for _, c := range ratingRefusals {
		if errors.Is(err, c.cause) {
			return c.text
		}
	}
	return "Yêu cầu bị từ chối. Vui lòng tải lại rồi thử lại."
}

// answerRatingError maps one failure onto a status and a fixed sentence.
//
// THE DEFAULT IS THE NARROW BRANCH (500), for traLoiLoiGui's reason: "anything unrecognised is the
// sender's fault" would turn a database outage into a message telling a citizen to fix their rating.
func (h *HandlerCongDan) answerRatingError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	switch {
	case errors.Is(err, petstore.ErrPhieuKhongTonTai):
		// Identical to an unknown code on the GET — rule 4, forbidden #2.
		h.khongTimThay(w)
	case domain.IsRatingInputError(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", ratingRefusalText(err), "")
	case errors.Is(err, domain.ErrRatingNotOpen), errors.Is(err, petstore.ErrPhieuDaChuyenTrang):
		// 409 AND NOT 404: this branch is reached only AFTER the read matched the citizen's own
		// petition, so saying its state is not rateable discloses nothing about anybody else's record.
		// The code `petition_state` is the one the staff routes branch on for the same class.
		h.d.Log.Info("từ chối đánh giá phản ánh vì trạng thái",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusConflict, "petition_state", ratingRefusalText(err), "")
	default:
		h.d.Log.Error("đánh giá phản ánh: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
