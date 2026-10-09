package app

// The use case behind POST /api/v1/my-citizen-reports/{code}/rating — the citizen rates the handling
// of their own petition, and a rating of 1 or 2 stars reopens it (ADR 0050 point 2).
//
// # THE SECOND WRITE A MEMBER OF THE PUBLIC PERFORMS, AND THE FIRST THAT MOVES A PETITION
//
// Intake creates a row; this changes one that staff have worked on. The actor is the CITIZEN — a weak
// identity (rule 4) — so everything that decides anything is read from the locked row and from the
// session, and the only inputs taken from the request are the stars and the comment.
//
// # ONE TRANSACTION, FOUR WRITES, AND THE FOURTH ONLY ON A REOPENING
//
//	the rating (and on 1–2 stars the reopening)   `phieu_phan_anh`    one UPDATE statement
//	the trail                                      `audit_log`         rule 6, invariant 3
//	the staff drawer's timeline row                `nhat_ky_phan_anh`  "Người dân đánh giá n sao"
//	the citizen is owed a word                     `su_kien_di`        rule 10, invariant 5 — reopen only
//
// A rating of 3–5 moves no status, so it publishes no `status_changed` fact and owes no message: the
// event is named after a status change, and a consumer counting time-in-status must not count one
// that did not happen (the reasoning PhanCong gives for a re-assignment).
//
// THE TIMELINE ROW follows the requirement repository (`service.py:826-838`: one event per rating,
// with the sentence it writes) — except WHO: `nguoi_ma` is domain.CitizenLogActor, never the citizen
// id (see that constant). ⚠ Its two action codes need migration 0013's CHECK widened; until that
// migration lands, every rating rolls back on the CHECK (TestLogActionsAreAllowedByTheSchema is red).

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// AuditActionCitizenRating is the business verb in the trail. Vietnamese snake_case, like every action
// this system writes (`cong_dan_gui_phan_anh`, `dong_phan_anh`) — an inspection reads the string.
//
// ONE VERB FOR BOTH OUTCOMES, WITH `mo_lai` ALWAYS IN THE DELTA, true or false — the precedent of
// `dong_khong_qua_xac_nhan` on the closing: "which petitions did citizens reopen" is then one key
// queried on one verb, and "which did citizens rate" does not have to union two.
const AuditActionCitizenRating = "cong_dan_danh_gia_phan_anh"

// PetitionRatingStore is the part of the register the rating needs, declared at the point of use.
// EVERY METHOD TAKES THE TRANSACTION. *petstore.PhieuPhanAnhStore satisfies it.
type PetitionRatingStore interface {
	CitizenPetitionForUpdate(ctx context.Context, tx *store.ScopedTx, congDanID, ma string) (
		domain.PhieuPhanAnh, error)
	RecordRating(ctx context.Context, tx *store.ScopedTx, id string, tuTrangThai domain.TrangThai,
		stars int, comment string, at time.Time) error
	ReopenByRating(ctx context.Context, tx *store.ScopedTx, id string, tuTrangThai domain.TrangThai,
		stars int, comment string, at time.Time) error

	// GhiNhatKy appends the timeline row (migration 0013) — the same append-only method the six staff
	// acts write through.
	GhiNhatKy(ctx context.Context, tx *store.ScopedTx, e domain.NhatKyPhanAnh) error
}

// RatingRequest is one rating as a citizen sends it.
//
// NO CITIZEN FIELD AND NO STATUS FIELD. Whose petition it is comes from the session (rule 4,
// invariant 2); whether it reopens is decided by the stars and the lifecycle, never by the client.
type RatingRequest struct {
	Stars int

	// Comment is OPTIONAL citizen free text (rule 3). Stored in `rating_comment` and NOWHERE ELSE: its
	// length is in the audit delta, never the text; it is not on the event and not logged.
	Comment string
}

// RatePetition owns the citizen's rating.
type RatePetition struct {
	db     *store.DB
	store  PetitionRatingStore
	events KhoSuKien

	// newID mints the outbox row's id; now is the clock `danh_gia_luc` and the event's `occurred_at`
	// come from. Both are seams so a test can pin them.
	newID func() (string, error)
	now   func() time.Time

	// notices is the STAFF-notice outbox (ADR 0086 kind 24) — not the citizen channel `events`. Built
	// from `db` by the constructor; noticeID is its own id seam.
	notices  StaffNoticeOutbox
	noticeID func() (string, error)

	// mergeHistory is read for the reopening's occurrence: a petition that was unmerged has started a
	// round the reopen counter does not see (statusOccurrence). Built from `db` like `notices`.
	mergeHistory PetitionMergeEvents
}

func NewRatePetition(db *store.DB, s PetitionRatingStore, events KhoSuKien) *RatePetition {
	return &RatePetition{
		db: db, store: s, events: events,
		newID:        ulid.Moi,
		now:          func() time.Time { return time.Now().UTC() },
		notices:      petstore.NewStaffNoticeOutboxStore(db),
		noticeID:     ulid.Moi,
		mergeHistory: petstore.NewPetitionMergeEventStore(db),
	}
}

// Rate records the citizen's rating of their own petition `ma`, reopening it on 1–2 stars, and returns
// the petition as it stands afterwards.
//
// `citizen` IS BOTH THE ACTOR AND THE OWNER, as on intake: the query filters by `citizen.ID`, and the
// trail names the same value. There is no second parameter a handler could fill from a request body.
func (uc *RatePetition) Rate(ctx context.Context, ma string, req RatingRequest, citizen audit.Actor) (
	domain.PhieuPhanAnh, error) {

	// FAIL CLOSED, BEFORE ANYTHING ELSE — the same wall as GuiPhanAnh.Gui. authz.CitizenOnly runs on the
	// route, so reaching here without a citizen means the route was mounted wrong.
	if citizen.ID == "" || citizen.Kind != "citizen" {
		return domain.PhieuPhanAnh{}, fmt.Errorf(
			"danh_gia_phan_anh: chủ thể không phải công dân (kind=%q) — tuyến thiếu authz.CitizenOnly "+
				"hoặc authz.CitizenPrincipal", citizen.Kind)
	}

	// Validated BEFORE the transaction opens: a misshapen request must not hold a row lock.
	stars, comment, err := domain.CheckRating(req.Stars, req.Comment)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	at := uc.now().UTC()
	var after domain.PhieuPhanAnh

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// BOTH AXES AND THE LOCK. Another citizen's code, another commune's code, a soft-deleted
		// petition and an unknown code are all ErrPhieuKhongTonTai here — one 404 body at the route.
		p, err := uc.store.CitizenPetitionForUpdate(ctx, tx, citizen.ID, ma)
		if err != nil {
			return err
		}
		if !domain.RatingOpen(p.TrangThai) {
			return domain.ErrRatingNotOpen
		}

		reopens := domain.RatingReopens(stars)
		after = p
		after.Rating, after.RatingComment, after.RatedAt = stars, comment, at

		if reopens {
			// The lifecycle map is consulted as well as RatingOpen, so the two declarations can never
			// disagree in the permissive direction (domain.ReopenByRatingAllowed).
			if !domain.ReopenByRatingAllowed(p.TrangThai) {
				return domain.ErrRatingNotOpen
			}
			if err := uc.store.ReopenByRating(ctx, tx, p.ID, p.TrangThai, stars, comment, at); err != nil {
				return err
			}
			after.TrangThai = domain.DangXuLy
			after.SoLanMoLai = p.SoLanMoLai + 1
			// Mirrors the statement: the resolved and closed instants are cleared, THE DEADLINES ARE NOT
			// TOUCHED (rule 10, invariant 2) — see store.ReopenByRating.
			after.XuLyXongLuc = time.Time{}
			after.DongLuc = time.Time{}
		} else if err := uc.store.RecordRating(ctx, tx, p.ID, p.TrangThai, stars, comment, at); err != nil {
			return err
		}

		// BEFORE AND AFTER (rule 6, invariant 5) — THE COMMENT IS NOT IN IT, ITS LENGTH IS.
		//
		// The comment is free text a citizen typed about their own case and will eventually name
		// somebody; `audit_log` is append-only and never deleted, so a copy there is a second permanent
		// store of personal data (rule 6, forbidden #4). `co_nhan_xet` says whether one was present
		// before and after, so a replaced comment is still visible as an event in the trail.
		//
		// THE DEADLINES ARE RECORDED ON A REOPENING, unchanged on both sides, so an inspection reading
		// this entry can see the promise was not moved (ADR 0050: "không tính lại hạn").
		delta := map[string]any{
			"truoc":  ratingSnapshot(p),
			"sau":    ratingSnapshot(after),
			"mo_lai": reopens,
			// DERIVED AT THE INSTANT OF THE ACT AND RECORDED — not the stored flag rule 10, invariant 3
			// forbids. On a reopening this is the petition measured against its unchanged deadline now.
			"tre_han": after.QuaHan(at),
		}
		if comment != "" {
			delta["do_dai_nhan_xet"] = utf8.RuneCountInString(comment)
		}
		if reopens {
			delta["han_xu_ly_xong"] = lucRaVet(p.HanXuLyXong)
			delta["han_tiep_nhan"] = lucRaVet(p.HanTiepNhan)
		}
		body, err := json.Marshal(delta)
		if err != nil {
			return fmt.Errorf("danh_gia_phan_anh: mã hoá delta: %w", err)
		}
		// TenantID left unset: audit.Write takes it from the transaction (rule 1, invariant 4). Subject
		// is the BUSINESS CODE the citizen holds.
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   citizen,
			Action:  AuditActionCitizenRating,
			Subject: p.MaTraCuu,
			Delta:   body,
		}); err != nil {
			return err
		}

		// THE TIMELINE ROW — the status AFTER the act (migration 0013), the requirement's sentence, the
		// fixed citizen marker as the author. No assignment pair: that is `phan-cong`'s alone (0013 CHECK).
		logID, err := uc.newID()
		if err != nil {
			return fmt.Errorf("danh_gia_phan_anh: sinh mã nhật ký: %w", err)
		}
		action := domain.LogActionCitizenRating
		if reopens {
			action = domain.LogActionReopenByRating
		}
		if err := uc.store.GhiNhatKy(ctx, tx, domain.NhatKyPhanAnh{
			ID:             logID,
			PhieuPhanAnhID: p.ID,
			ThoiDiem:       at,
			NguoiMa:        domain.CitizenLogActor,
			HanhVi:         action,
			TrangThai:      after.TrangThai,
			NoiDung:        domain.RatingLogText(stars, reopens),
		}); err != nil {
			return err
		}

		if !reopens {
			return nil
		}
		// THE REOPENING OWES THE CITIZEN A WORD (ADR 0041:53, ADR 0050 point 2; rule 10, invariant 5),
		// in the SAME transaction. `after.SoLanMoLai` is already incremented and the unmerges are added
		// (statusOccurrence), so Occurrence is a round no earlier entry into `dang-xu-ly` used — not the
		// first one's, and not the one an unmerge back into processing already took.
		if after.CongDanID != "" {
			unmerges, err := countUnmerges(ctx, tx, uc.mergeHistory, after)
			if err != nil {
				return err
			}
			if err := writeStatusChangedEvent(ctx, tx, uc.events, uc.newID, after, domain.DangXuLy, at,
				domain.ReopenNextStep(after), unmerges); err != nil {
				return err
			}
		}
		// AND THE OFFICER HOLDING IT A NOTICE (ADR 0086 kind 24), keyed by this rating's timeline row. No
		// officer on the petition (a unit-only assignment) → no row. The actor is a CITIZEN, so there is
		// no staff code to drop. NEVER the comment: the title carries the lookup code alone.
		return writeStaffNotice(ctx, tx, uc.notices, uc.noticeID,
			domain.PetitionReopenedNotice(after, logID), "", at)
	})
	if err != nil {
		// NOT the code, NOT the comment, NOT the citizen id — the commune and the act only (rule 3).
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "đánh giá", err)
	}
	return after, nil
}

// ratingSnapshot is the part of the petition the rating changes, for the audit delta. NO FREE TEXT.
func ratingSnapshot(p domain.PhieuPhanAnh) map[string]any {
	var stars any
	if p.Rating != 0 {
		stars = p.Rating
	}
	return map[string]any{
		"trang_thai":     string(p.TrangThai),
		"diem_hai_long":  stars,
		"co_nhan_xet":    p.RatingComment != "",
		"danh_gia_luc":   lucRaVet(p.RatedAt),
		"so_lan_mo_lai":  p.SoLanMoLai,
		"xu_ly_xong_luc": lucRaVet(p.XuLyXongLuc),
	}
}

// Compile-time proof the real store satisfies the interface — a method renamed there fails the build
// here, not at wiring.
var _ PetitionRatingStore = (*petstore.PhieuPhanAnhStore)(nil)
