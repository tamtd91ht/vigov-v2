package http

// The staff-recorded rating (owner rule 11/10/2026, bug sheet row 60) — the handler of
// routes_staff_recorded_rating.go and the shape the staff detail carries it in. HTTP translation and
// nothing else: the rules are domain/petition_staff_recorded_rating.go, the transaction and the trail
// app.StaffRecordedRatings'.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// StaffRecordedRatingActs is the act and the detail's read. *app.StaffRecordedRatings satisfies it.
type StaffRecordedRatingActs interface {
	Record(ctx context.Context, code string, req app.StaffRatingRequest, actor audit.Actor,
		restricted app.QuyenXemHanChe) (domain.PhieuPhanAnh, domain.StaffRecordedRating, error)
	ForPetition(ctx context.Context, petitionID string) (domain.StaffRecordedRating, bool, error)
}

// staffRecordedRatingIn is the body of POST …/{maTraCuu}/staff-recorded-rating.
//
// NO `source`, NO STAFF CODE, NO STATUS: the source is always `staff_recorded`, the author is the session's
// staff business code, and the act moves no status. `note` is optional staff free text — what the citizen
// said, and how; it may name people (rule 3), so it is never logged and only its length is audited.
type staffRecordedRatingIn struct {
	Stars   int    `json:"stars"`
	Channel string `json:"channel"`
	Note    string `json:"note,omitempty"`
}

// staffRecordedRatingOut is the record as the STAFF detail shows it — beside, never inside, the citizen's
// own `rating`. `source` is always `staff_recorded`, on the wire too, so a client cannot mistake it.
// Staff surface only: phieuCuaToiRa (the citizen's own view) does not carry it.
type staffRecordedRatingOut struct {
	Stars      int       `json:"stars"`
	Channel    string    `json:"channel"`
	Note       string    `json:"note,omitempty"`
	Source     string    `json:"source"`
	RecordedBy string    `json:"recorded_by"`
	RecordedAt time.Time `json:"recorded_at"`
}

func staffRecordedRatingOutOf(r domain.StaffRecordedRating) *staffRecordedRatingOut {
	return &staffRecordedRatingOut{
		Stars: r.Stars, Channel: string(r.Channel), Note: r.Note, Source: r.Source,
		RecordedBy: r.RecordedBy, RecordedAt: r.RecordedAt,
	}
}

// RecordStaffRating serves POST /api/v1/citizen-reports/{maTraCuu}/staff-recorded-rating. Replies with the
// petition (unchanged by the act) carrying the new record in `staff_recorded_rating`.
func (h *Handler) RecordStaffRating(w http.ResponseWriter, r *http.Request) {
	var in staffRecordedRatingIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	p, rec, err := h.d.StaffRecordedRatings.Record(ctx, r.PathValue("maTraCuu"),
		app.StaffRatingRequest{Stars: in.Stars, Channel: in.Channel, Note: in.Note}, actor, h.coQuyenHanChe(ctx))
	if err != nil {
		h.answerStaffRatingError(w, r, err)
		return
	}
	h.traPhieu(w, r, p, func(ra *phieuPhanAnhRa) { ra.StaffRecordedRating = staffRecordedRatingOutOf(rec) })
}

// staffRatingRefusals — FIXED SENTENCES, never err.Error(): a refusal raised inside the transaction arrives
// wrapped with the commune id (app.bocPhieu). None echoes the note.
var staffRatingRefusals = []struct {
	err    error
	status int
	code   string
	text   string
}{
	{domain.ErrRatingStarsOutOfRange, http.StatusBadRequest, "invalid_request", "Số sao phải từ 1 đến 5."},
	{domain.ErrStaffRatingChannelInvalid, http.StatusBadRequest, "invalid_request",
		"Chọn cách người dân đánh giá: qua điện thoại hoặc tại quầy."},
	{domain.ErrStaffRatingNoteTooLong, http.StatusBadRequest, "invalid_request", "Ghi chú quá dài (tối đa 1000 ký tự)."},
	{domain.ErrStaffRatingCitizenRated, http.StatusConflict, "citizen_rated",
		"Người dân đã tự đánh giá phiếu này nên không ghi nhận đánh giá thay được nữa."},
	{domain.ErrStaffRatingAlreadyRecorded, http.StatusConflict, "staff_rating_exists",
		"Phiếu này đã có đánh giá do cán bộ ghi nhận."},
}

// answerStaffRatingError maps the act's refusals, then falls through to traLoiLoiXuLy — the one mapping
// every petition act shares (404 for every cause that would disclose a record, 500 otherwise).
func (h *Handler) answerStaffRatingError(w http.ResponseWriter, r *http.Request, err error) {
	for _, m := range staffRatingRefusals {
		if errors.Is(err, m.err) {
			staff := ""
			if p, ok := authz.From(r.Context()); ok {
				staff = p.Ma
			}
			// INFO: the rule doing its job. No code, no note (app.bocPhieu); the officer by business code.
			h.d.Log.Info("ghi nhận đánh giá thay người dân: từ chối", "xa", string(tenant.MustFrom(r.Context())),
				"can_bo", staff, "ma_loi", m.code)
			httpx.WriteError(w, m.status, m.code, m.text, "")
			return
		}
	}
	h.traLoiLoiXuLy(w, r, "ghi nhận đánh giá thay người dân", err)
}
