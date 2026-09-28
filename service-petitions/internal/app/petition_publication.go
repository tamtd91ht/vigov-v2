package app

// The use case behind PUT /api/v1/citizen-reports/{code}/publication — a member of staff decides
// whether a petition may appear on the public page (ADR 0050 point 8; requirement
// `router.py:349-361`, `service.py:764-793`; SRS M4.3.1).
//
// # ONE TRANSACTION, TWO WRITES — AND DELIBERATELY NOT THREE OR FOUR
//
//	the decision            `phieu_phan_anh.publication_status`   one UPDATE, nothing else on the row
//	the trail               `audit_log`                           rule 6, invariant 3
//
// NO TIMELINE ROW (`nhat_ky_phan_anh`). The requirement's moderate() writes no event
// (`service.py:784-793`, against the `add_event` every other act there calls); its only record is the
// field-level audit on `moderation_state` (`models.py:87-98`). Following it keeps migration 0013's
// closed `hanh_vi` list as it is.
//
// NO OUTBOX ROW, NO CITIZEN MESSAGE. A moderation is not a status transition and is not in ADR 0041's
// table; the event `petitions.status_changed.v1` is named after a status change, and the status does
// not change (user decision 28/09/2026 — the requirement's RECEIVED -> SCREENING side effect is not
// copied, so no deadline and no notification can move as a by-product).

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// AuditActionSetPublication is the business verb in the trail — THE EXACT STRING migration 0017's
// backfill wrote for the system's own decisions, so "every publication decision on this petition,
// by the system or by staff" is one query on one verb.
const AuditActionSetPublication = "dat_trang_thai_cong_khai"

// SetPublication sets the petition's publication status to `raw` (`cong-khai` or `an`).
// Route permission: `feedback.assign` (the requirement's key, `router.py:352`).
//
// # IDEMPOTENT: THE SAME DECISION TWICE WRITES NOTHING THE SECOND TIME
//
// A PUT states an absolute value. When the row already holds it, there is no change to record, so
// there is no UPDATE and no audit entry — the precedent of PUT …/no-task-marker. The petition is
// returned as it stands, 200. An entry saying "cong-khai -> cong-khai" would be an act that did not
// happen, and the trail records acts.
//
// # THE ORDER OF THE REFUSALS IS THE SAME AS ON EVERY OTHER WRITE ROUTE
//
// Restricted field first (404, identical to an unknown code — app.ErrPhieuHanChe), then the one rule
// of this act (a `can-bo` petition is never `cong-khai`, 409). A caller without `feedback.restricted`
// must never learn from a 409 that a staff-conduct report exists under this code.
func (uc *XuLyPhanAnh) SetPublication(ctx context.Context, ma, raw string, nguoi audit.Actor,
	hanChe QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	to, err := domain.CheckPublicationRequest(raw)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	var after domain.PhieuPhanAnh
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := duocChamPhieuHanChe(p, hanChe); err != nil {
			return err
		}
		if err := domain.PublicationAllowed(p, to); err != nil {
			return err
		}
		after = p
		if p.PublicationStatus == to {
			return nil
		}
		if err := uc.kho.SetPublicationStatus(ctx, tx, p.ID, p.PublicationStatus, to); err != nil {
			return err
		}
		after.PublicationStatus = to

		// BEFORE AND AFTER (rule 6, invariant 5), in the `truoc` / `sau` shape migration 0017's backfill
		// wrote. The lifecycle status is recorded on both sides, UNCHANGED, so an inspection reading
		// this entry can see the moderation moved no status. No personal data: neither the content nor
		// the reporter is in it (rule 6, forbidden #4).
		delta, err := json.Marshal(map[string]any{
			"truoc": map[string]any{
				"publication_status": string(p.PublicationStatus),
				"trang_thai":         string(p.TrangThai),
			},
			"sau": map[string]any{
				"publication_status": string(to),
				"trang_thai":         string(p.TrangThai),
			},
		})
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: mã hoá delta: %w", err)
		}
		// TenantID left unset: audit.Write takes it from the transaction (rule 1, invariant 4). Actor is
		// the staff BUSINESS CODE the handler built from Principal.Ma (rule 6, invariant 8).
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  AuditActionSetPublication,
			Subject: p.MaTraCuu,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "đặt trạng thái công khai", err)
	}
	return after, nil
}
