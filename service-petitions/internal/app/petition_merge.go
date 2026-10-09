package app

// Merging duplicate petitions — ADR 0087, the ADR 0041 amendment of 09/10/2026, the owner's answers of
// 09/10/2026 (migration 0037's header). Three acts on XuLyPhanAnh, because they share its locked reads,
// its outbox and its timeline with the four lifecycle acts:
//
//	Merge      link a petition to a MAIN petition          POST …/{code}/merge     feedback.classify
//	Unmerge    take it out again, reason mandatory         POST …/{code}/unmerge   feedback.classify
//	followMain merged petitions follow their main into `cho-dan-xac-nhan` / `da-dong` (called by
//	           TienTrangThai and Dong, inside their transactions)
//
// EVERY ACT IS ONE TRANSACTION, and what it writes stands or falls together (rule 6, invariant 3; rule 2,
// invariant 6 — an administrative file is never half-processed):
//
//	merge     main's deadline (when it moves) · the link · the history row · two audit entries
//	unmerge   the link cleared · the history row · two audit entries
//	follow    per merged petition: its status · its audit entry · its timeline row · its own outbox row
//
// LOCK ORDER: THE MAIN PETITION FIRST, THEN THE MERGED ONE — the order TienTrangThai and Dong already take
// (main, then MergedPetitionsForUpdate), so a merge and a closing of the same main petition serialise
// instead of deadlocking. Two officers merging A into B and B into A at once is a lock cycle PostgreSQL
// detects and aborts (migration 0037's header); the loser answers 500 and nothing was written.
//
// ⚠ WHAT IS NOT WRITTEN, said where it would be:
//
//	the TIMELINE row of a merge / unmerge   `nhat_ky_phan_anh.hanh_vi` is a CLOSED list (0013, widened by
//	                                        0018/0023/0030) with no `gop-phieu` / `tach-phieu`; writing one
//	                                        would roll every merge back on PostgreSQL. Widening it is a
//	                                        migration on a populated table — not this card's. The act's
//	                                        record is petition_merge_event + audit_log, both in the same
//	                                        transaction.
//	the CITIZEN NOTIFICATION of a merge /   no contract can carry it (domain.mergeMessage says why); the
//	unmerge                                 obligation is recorded by the petition_merge_event row itself.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// PetitionMergeEvents is the append-only history (migration 0037). *petstore.PetitionMergeEventStore
// satisfies it. ONE METHOD AND IT TAKES THE TRANSACTION — no history row can be written outside the act.
type PetitionMergeEvents interface {
	Append(ctx context.Context, tx *store.ScopedTx, e domain.PetitionMergeEvent) error
}

// The business verbs written into the trail (rule 6). Vietnamese snake_case VALUES like every verb this
// service writes; one verb per SIDE, so an inspection asking "what was merged into PA-…" and "where did
// PA-… go" each searches one string.
const (
	ActionMergePetition         = "gop_phieu_phan_anh"          // on the merged petition
	ActionReceiveMergedPetition = "nhan_phieu_gop_phan_anh"     // on the main petition
	ActionUnmergePetition       = "tach_phieu_phan_anh"         // on the petition taken out
	ActionUnmergeFromMain       = "tach_phieu_khoi_phieu_chinh" // on the main petition
)

// MergeRequest is one merge as it arrives from the handler: the MAIN petition's lookup code, and an
// optional reason (staff-internal, may hold personal data — never logged, never in the audit delta).
type MergeRequest struct {
	MainCode string
	Reason   string
}

// errMergeHistoryNotWired: the history store is absent. A wiring fault (500) — a merge without its
// history row is the one pairing migration 0037 leaves to this layer.
var errMergeHistoryNotWired = errors.New("xu_ly_phan_anh: chưa nối dây sổ lịch sử gộp phiếu")

// Merge links petition `code` to the main petition `req.MainCode`. Permission: `feedback.classify`
// (ADR 0087 §5), same commune by construction — both codes are read in the commune the context carries,
// so another commune's code is the unknown code's 404.
func (uc *XuLyPhanAnh) Merge(ctx context.Context, code string, req MergeRequest, actor audit.Actor,
	restricted QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	mainCode := strings.TrimSpace(req.MainCode)
	if mainCode == "" {
		return domain.PhieuPhanAnh{}, domain.ErrMergeMainMissing
	}
	if mainCode == code {
		return domain.PhieuPhanAnh{}, domain.ErrMergeSelf
	}
	reason, err := domain.CheckMergeReason(req.Reason)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	if uc.mergeEvents == nil {
		return domain.PhieuPhanAnh{}, errMergeHistoryNotWired
	}

	now := uc.nayHoac()
	var after domain.PhieuPhanAnh
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		main, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, mainCode)
		if err != nil {
			return err
		}
		child, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, code)
		if err != nil {
			return err
		}
		// THE RESTRICTED FIELD FIRST, ON BOTH: a caller without `feedback.restricted` is told what an
		// unknown code is told, never "a can-bo petition cannot be merged" — which would confirm that a
		// report about a colleague exists under that code.
		if err := duocChamPhieuHanChe(child, restricted); err != nil {
			return err
		}
		if err := duocChamPhieuHanChe(main, restricted); err != nil {
			return err
		}
		hasChildren, err := uc.kho.HasMergedPetitionsTx(ctx, tx, child.ID)
		if err != nil {
			return err
		}
		deadline, err := domain.CheckMerge(child, main, hasChildren)
		if err != nil {
			return err
		}
		// THE DEADLINE BEFORE THE LINK — the trigger refuses a link whose main does not already carry the
		// earlier deadline (migration 0037). Skipped when nothing moves (main already earlier, or both
		// "chưa có"): an UPDATE that changes nothing would still be a write with nothing to audit.
		if !deadline.Equal(main.HanXuLyXong) {
			if err := uc.kho.MovePetitionDeadlineForMerge(ctx, tx, main.ID, main.HanXuLyXong, deadline); err != nil {
				return err
			}
		}
		if err := uc.kho.LinkMerge(ctx, tx, child.ID, main.ID, child.TrangThai, now, actor.ID); err != nil {
			return err
		}
		eventID, err := uc.sinhID()
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: sinh mã lịch sử gộp: %w", err)
		}
		if err := uc.mergeEvents.Append(ctx, tx, domain.PetitionMergeEvent{
			ID: eventID, PetitionID: child.ID, MainPetitionID: main.ID, Kind: domain.MergeKindMerge,
			PerformedAt: now, PerformedBy: actor.ID, Reason: reason,
			MainDeadlineBefore: main.HanXuLyXong, MainDeadlineAfter: deadline,
		}); err != nil {
			return err
		}
		if err := writeMergeAudit(ctx, tx, actor, child, main, main.HanXuLyXong, deadline, reason,
			ActionMergePetition, ActionReceiveMergedPetition); err != nil {
			return err
		}

		after = child
		after.MergedInto, after.MergedAt, after.MergedBy = main.ID, now, actor.ID
		// The CITIZEN of `child` is owed MergeNextStep — recorded by the history row above, not sent: see
		// domain.mergeMessage for the contract that does not exist yet.
		return nil
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "gộp phiếu", err)
	}
	return after, nil
}

// Unmerge takes petition `code` out of its main petition. Permission: `feedback.classify`. The reason is
// MANDATORY (owner, 09/10/2026). The main petition's deadline is NOT lengthened back (ADR 0087 §Hệ quả).
func (uc *XuLyPhanAnh) Unmerge(ctx context.Context, code, rawReason string, actor audit.Actor,
	restricted QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	reason, err := domain.CheckUnmergeReason(rawReason)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	if uc.mergeEvents == nil {
		return domain.PhieuPhanAnh{}, errMergeHistoryNotWired
	}

	// READ ONCE WITHOUT THE LOCK, only to learn WHICH main petition to lock first (the lock order above).
	// The decision is taken again on the locked rows.
	first, err := uc.kho.TheoMaTraCuu(ctx, code)
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "tách phiếu", err)
	}
	if err := duocChamPhieuHanChe(first, restricted); err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "tách phiếu", err)
	}
	if first.MergedInto == "" {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "tách phiếu", domain.ErrNotMerged)
	}

	now := uc.nayHoac()
	var after domain.PhieuPhanAnh
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		main, err := uc.kho.ByIDForUpdate(ctx, tx, first.MergedInto)
		if errors.Is(err, petstore.ErrPhieuKhongTonTai) {
			// The main petition was soft-deleted since — what to do with its merged petitions is decided by
			// nobody (migration 0037's header). A state refusal, never a guess.
			return petstore.ErrPhieuDaChuyenTrang
		}
		if err != nil {
			return err
		}
		child, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, code)
		if err != nil {
			return err
		}
		if err := duocChamPhieuHanChe(child, restricted); err != nil {
			return err
		}
		if child.MergedInto != "" && child.MergedInto != main.ID {
			return petstore.ErrPhieuDaChuyenTrang // re-linked between the two reads
		}
		if err := domain.CheckUnmerge(child); err != nil {
			return err
		}
		if err := uc.kho.UnlinkMerge(ctx, tx, child.ID, main.ID, child.TrangThai); err != nil {
			return err
		}
		eventID, err := uc.sinhID()
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: sinh mã lịch sử tách: %w", err)
		}
		// THE SAME DEADLINE BEFORE AND AFTER: unmerging never lengthens the main petition's commitment.
		if err := uc.mergeEvents.Append(ctx, tx, domain.PetitionMergeEvent{
			ID: eventID, PetitionID: child.ID, MainPetitionID: main.ID, Kind: domain.MergeKindUnmerge,
			PerformedAt: now, PerformedBy: actor.ID, Reason: reason,
			MainDeadlineBefore: main.HanXuLyXong, MainDeadlineAfter: main.HanXuLyXong,
		}); err != nil {
			return err
		}
		if err := writeMergeAudit(ctx, tx, actor, child, main, main.HanXuLyXong, main.HanXuLyXong, reason,
			ActionUnmergePetition, ActionUnmergeFromMain); err != nil {
			return err
		}
		after = child
		after.MergedInto, after.MergedAt, after.MergedBy = "", time.Time{}, ""
		// The CITIZEN of `child` is owed UnmergeNextStep — recorded by the history row, not sent (see Merge).
		return nil
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "tách phiếu", err)
	}
	return after, nil
}

// writeMergeAudit writes the two entries of one merge or unmerge — one per petition touched, each with
// the OTHER petition's code and the main petition's deadline around the act (rule 6, invariant 5).
//
// THE REASON IS NOT IN EITHER DELTA, ITS LENGTH IS: staff free text that may quote a report, and
// audit_log is append-only and never deleted (rule 6, forbidden #4). It lives on petition_merge_event.
// The actor is the staff BUSINESS code the handler put in audit.Actor.ID (rule 6, invariant 8).
func writeMergeAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, child, main domain.PhieuPhanAnh,
	deadlineBefore, deadlineAfter time.Time, reason, childAction, mainAction string) error {

	deadline := map[string]any{
		"truoc": map[string]any{"han_xu_ly_xong": lucRaVet(deadlineBefore)},
		"sau":   map[string]any{"han_xu_ly_xong": lucRaVet(deadlineAfter)},
	}
	childDelta := map[string]any{"main_petition": main.MaTraCuu, "main_deadline": deadline}
	mainDelta := map[string]any{"merged_petition": child.MaTraCuu, "truoc": deadline["truoc"], "sau": deadline["sau"]}
	if reason != "" {
		childDelta["reason_length"] = utf8.RuneCountInString(reason)
		mainDelta["reason_length"] = utf8.RuneCountInString(reason)
	}
	for _, e := range []struct {
		action, subject string
		delta           map[string]any
	}{
		{childAction, child.MaTraCuu, childDelta},
		{mainAction, main.MaTraCuu, mainDelta},
	} {
		body, err := json.Marshal(e.delta)
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: mã hoá delta gộp phiếu: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: e.action, Subject: e.subject, Delta: body}); err != nil {
			return err
		}
	}
	return nil
}

// followMain moves the petitions merged into `main` along with it (ADR 0087 §2), inside the caller's
// transaction, right after `main` itself moved. `main` is the petition AFTER its act.
//
// ONLY INTO `cho-dan-xac-nhan` AND `da-dong`, and only a petition not already there or ended
// (domain.FollowsMain). Into `da-dong` with THE MAIN PETITION'S RESULT — the same sentence; every merged
// petition's citizen then reads the result through their OWN lookup code and never sees the main
// petition's content, photos or reporter (ADR 0087 §4: the result is the only text copied).
//
// EACH MERGED PETITION IS ITS OWN RECORD: its own audit entry (the same verb as the main's act, with the
// main's code in the delta), its own timeline row, and its OWN `petitions.status_changed.v1` outbox row —
// so each citizen is told by the ADR 0041 table for THEIR petition, with their own code and occurrence.
// Each then confirms and rates their own petition (ADR 0087 §2) where the lifecycle still allows it.
//
// ONE READ for all of them (MergedPetitionsForUpdate, under lock); the writes are per petition by nature —
// each is a separate archival record with its own trail.
//
// A MERGED PETITION HAS NO MERGED PETITIONS (no chains), so a petition that is itself merged asks nothing.
func (uc *XuLyPhanAnh) followMain(ctx context.Context, tx *store.ScopedTx, main domain.PhieuPhanAnh,
	actor audit.Actor, at time.Time) error {

	target := main.TrangThai
	if (target != domain.ChoDanXacNhan && target != domain.DaDong) || main.MergedInto != "" {
		return nil
	}
	children, err := uc.kho.MergedPetitionsForUpdate(ctx, tx, main.ID)
	if err != nil {
		return err
	}
	action, logAction := HanhViChuyenTrangPhieu, domain.NhatKyChuyenTrangThai
	if target == domain.DaDong {
		action, logAction = HanhViDongPhanAnh, domain.NhatKyDongPhieu
	}
	for _, c := range children {
		if !domain.FollowsMain(c.TrangThai, target) {
			continue
		}
		result, closedAt := "", time.Time{}
		if target == domain.DaDong {
			result, closedAt = main.KetQuaXuLy, at
		}
		if err := uc.kho.FollowMain(ctx, tx, c.ID, main.ID, c.TrangThai, target, main.XuLyXongLuc,
			result, closedAt); err != nil {
			return err
		}
		after := c
		after.TrangThai = target
		if after.XuLyXongLuc.IsZero() {
			after.XuLyXongLuc = main.XuLyXongLuc
		}
		if target == domain.DaDong {
			after.KetQuaXuLy, after.DongLuc = result, at
		}
		delta := map[string]any{
			"truoc":        map[string]any{"trang_thai": string(c.TrangThai)},
			"sau":          map[string]any{"trang_thai": string(target)},
			"follows_main": main.MaTraCuu,
		}
		if target == domain.DaDong {
			// The result's LENGTH, never its text — Dong's own reasoning (rule 6, forbidden #4).
			delta["do_dai_ket_qua"] = utf8.RuneCountInString(result)
		}
		body, err := json.Marshal(delta)
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: mã hoá delta phiếu gộp: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: c.MaTraCuu, Delta: body}); err != nil {
			return err
		}
		if _, err := uc.writeTimelineRow(ctx, tx, after, logAction, at, actor, "", false); err != nil {
			return err
		}
		if err := uc.ghiSuKien(ctx, tx, after, target, at); err != nil {
			return err
		}
	}
	return nil
}
