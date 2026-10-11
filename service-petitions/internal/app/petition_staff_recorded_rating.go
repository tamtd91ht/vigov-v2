package app

// The use case behind POST /api/v1/citizen-reports/{code}/staff-recorded-rating — a member of staff records
// the rating a citizen gave by phone or at the counter (the prototype's "Ghi nhận đánh giá của người dân",
// vigov-require FeedbackDetailDrawer.tsx:516-540 / router.py:391-405; owner rule 11/10/2026, bug sheet row
// 60). The rules are in domain/petition_staff_recorded_rating.go; why it is a separate table, in migration
// 0042's header.
//
// # ONE TRANSACTION, TWO WRITES
//
//	the record   `petition_staff_recorded_rating`   one INSERT (append-only)
//	the trail    `audit_log`                         rule 6, invariant 3
//
// NOTHING ELSE. No UPDATE of `phieu_phan_anh` (the citizen's `diem_hai_long` is never touched, the status
// never moves), no timeline row (`nhat_ky_phan_anh`'s closed action list has no code for it, and adding one
// is a two-step CHECK swap on a populated table — 0039/0040's shape — left for a later card), no citizen
// message and no staff notice (no status moved; ADR 0041 / 0086 name no row for it).

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

// AuditActionStaffRecordedRating is the verb in the trail. Vietnamese snake_case like every action this
// service writes; DISTINCT from the citizen's `cong_dan_danh_gia_phan_anh`, so "which ratings did citizens
// give themselves" and "which did staff record" are never one query by accident.
const AuditActionStaffRecordedRating = "can_bo_ghi_nhan_danh_gia_phan_anh"

// StaffRatingPetitions is the part of the register the act needs: the petition, read under its lock.
// *petstore.PhieuPhanAnhStore satisfies it.
type StaffRatingPetitions interface {
	TheoMaTraCuuDeSua(ctx context.Context, tx *store.ScopedTx, ma string) (domain.PhieuPhanAnh, error) // vi-name-ok: the existing store method, mirrored
}

// StaffRatingRecords is the record table. *petstore.PetitionStaffRatingStore satisfies it. Every method
// that writes takes the commune-scoped transaction.
type StaffRatingRecords interface {
	ForPetitionTx(ctx context.Context, tx *store.ScopedTx, petitionID string) (domain.StaffRecordedRating, bool, error)
	ForPetition(ctx context.Context, petitionID string) (domain.StaffRecordedRating, bool, error)
	Append(ctx context.Context, tx *store.ScopedTx, r domain.StaffRecordedRating) error
}

// StaffRatingRequest is the act as the handler hands it down. No citizen field, no status field.
type StaffRatingRequest struct {
	Stars   int
	Channel string
	Note    string
}

// StaffRecordedRatings owns the act and the detail's read.
type StaffRecordedRatings struct {
	db        *store.DB
	petitions StaffRatingPetitions
	records   StaffRatingRecords

	newID func() (string, error)
	now   func() time.Time
}

func NewStaffRecordedRatings(db *store.DB, petitions StaffRatingPetitions, records StaffRatingRecords) *StaffRecordedRatings {
	return &StaffRecordedRatings{
		db: db, petitions: petitions, records: records,
		newID: ulid.Moi,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// Record writes the staff record for petition `code` and returns the petition (unchanged) and the record.
// Route permission: `feedback.resolve` (the prototype's key, seeded in service-identity 0001_init.sql).
//
// THE ORDER OF THE REFUSALS IS EVERY PETITION WRITE'S: restricted field first (404, identical to an unknown
// code — ErrPhieuHanChe), then this act's two 409s. A caller without `feedback.restricted` must never learn
// from a 409 that a staff-conduct report exists under this code.
func (uc *StaffRecordedRatings) Record(ctx context.Context, code string, req StaffRatingRequest,
	actor audit.Actor, restricted QuyenXemHanChe) (domain.PhieuPhanAnh, domain.StaffRecordedRating, error) {

	stars, channel, note, err := domain.CheckStaffRecordedRating(req.Stars, req.Channel, req.Note)
	if err != nil {
		return domain.PhieuPhanAnh{}, domain.StaffRecordedRating{}, err
	}
	// The actor is a STAFF business code (rule 6, invariant 8) — no fallback.
	if err := coCanBoThucHien(actor); err != nil {
		return domain.PhieuPhanAnh{}, domain.StaffRecordedRating{}, err
	}
	id, err := uc.newID()
	if err != nil {
		return domain.PhieuPhanAnh{}, domain.StaffRecordedRating{}, fmt.Errorf("danh_gia_can_bo_ghi: sinh mã: %w", err)
	}

	at := uc.now().UTC()
	var (
		p   domain.PhieuPhanAnh
		rec domain.StaffRecordedRating
	)
	// db.For(ctx): the transaction is scoped to the commune the context carries (rule 1, invariant 5).
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE PETITION'S ROW LOCK — the same row the citizen's own rating locks (CitizenPetitionForUpdate),
		// so "the citizen rated" and "staff recorded" cannot race past each other's check.
		var err error
		p, err = uc.petitions.TheoMaTraCuuDeSua(ctx, tx, code)
		if err != nil {
			return err
		}
		if err := duocChamPhieuHanChe(p, restricted); err != nil {
			return err
		}
		if err := domain.CheckStaffRatingAllowed(p); err != nil {
			return err
		}
		_, exists, err := uc.records.ForPetitionTx(ctx, tx, p.ID)
		if err != nil {
			return err
		}
		if exists {
			return domain.ErrStaffRatingAlreadyRecorded
		}

		rec = domain.StaffRecordedRating{
			ID: id, PetitionID: p.ID, Stars: stars, Channel: channel, Note: note,
			Source:     domain.StaffRecordedSource,
			RecordedBy: actor.ID,
			RecordedAt: at,
		}
		if err := uc.records.Append(ctx, tx, rec); err != nil {
			return err
		}

		// THE NOTE'S LENGTH, NOT ITS TEXT (rule 6, forbidden #4): it is what a citizen said about their case,
		// typed by staff, and `audit_log` is permanent. `nguon` is restated so the entry alone says this was
		// NOT the citizen's own verdict; the petition's status and the citizen's (absent) rating are recorded,
		// so an inspection can see the act moved neither.
		delta := map[string]any{
			"sau": map[string]any{
				"so_sao":     stars,
				"kenh":       string(channel),
				"nguon":      domain.StaffRecordedSource,
				"ghi_luc":    lucRaVet(at),
				"ban_ghi_id": id,
			},
			"trang_thai_phieu":   string(p.TrangThai),
			"dan_da_tu_danh_gia": false,
		}
		if note != "" {
			delta["do_dai_ghi_chu"] = utf8.RuneCountInString(note)
		}
		body, err := json.Marshal(delta)
		if err != nil {
			return fmt.Errorf("danh_gia_can_bo_ghi: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  AuditActionStaffRecordedRating,
			Subject: p.MaTraCuu,
			Delta:   body,
		})
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, domain.StaffRecordedRating{}, bocPhieu(ctx, "ghi nhận đánh giá thay người dân", err)
	}
	return p, rec, nil
}

// ForPetition reads the staff record of a petition for the staff detail. `false` = none.
func (uc *StaffRecordedRatings) ForPetition(ctx context.Context, petitionID string) (domain.StaffRecordedRating, bool, error) {
	return uc.records.ForPetition(ctx, petitionID)
}

// Compile-time proof the real stores satisfy the interfaces.
var (
	_ StaffRatingPetitions = (*petstore.PhieuPhanAnhStore)(nil)
	_ StaffRatingRecords   = (*petstore.PetitionStaffRatingStore)(nil)
)
