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
//	merge     main's deadline (when it moves) · the link · the history row · two audit entries ·
//	          a timeline row on EACH petition · the merged petition's `merge_changed` outbox row
//	unmerge   the link cleared (and, from `cho-dan-xac-nhan`, the status back to `dang-xu-ly`) · the
//	          history row · two audit entries · a timeline row on EACH petition · the outbox row(s)
//	follow    per merged petition: its status · its audit entry · its timeline row · its own outbox row
//
// LOCK ORDER: THE MAIN PETITION FIRST, THEN THE MERGED ONE — the order TienTrangThai and Dong already take
// (main, then MergedPetitionsForUpdate), so a merge and a closing of the same main petition serialise
// instead of deadlocking. Two officers merging A into B and B into A at once is a lock cycle PostgreSQL
// detects and aborts (migration 0037's header); the loser answers 500 and nothing was written.
//
// THE CITIZEN OF THE MERGED PETITION IS TOLD, on a merge AND on an unmerge (owner, 09/10/2026 (a)), by
// `petitions.merge_changed.v1` — never `status_changed`, which would collapse onto the notice already
// sent for the current status (events.proto, PetitionMergeChanged). The main petition's citizen is owed
// nothing by either act (ADR 0041 §Sửa đổi 09/10/2026), and the message never names the main petition
// (ADR 0087 §4). Declared boundary: transaction-boundaries.json `thong_bao_cong_dan_khi_gop_tach_phieu`.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/audit"
	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// PetitionMergeEvents is the append-only history (migration 0037). *petstore.PetitionMergeEventStore
// satisfies it. EVERY METHOD TAKES THE TRANSACTION — no history row can be written outside the act, and
// the count is read on the connection that just appended, so it includes the act's own row.
type PetitionMergeEvents interface {
	Append(ctx context.Context, tx *store.ScopedTx, e domain.PetitionMergeEvent) error
	CountTx(ctx context.Context, tx *store.ScopedTx, petitionID string, kind domain.MergeKind) (int, error)
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
			ActionMergePetition, ActionReceiveMergedPetition, child.TrangThai); err != nil {
			return err
		}

		after = child
		after.MergedInto, after.MergedAt, after.MergedBy = main.ID, now, actor.ID
		if err := uc.writeMergeTimelineRows(ctx, tx, domain.MergeKindMerge, after, main, now, actor); err != nil {
			return err
		}
		return uc.writeMergeChangedEvent(ctx, tx, after, domain.MergeKindMerge, now)
	})
	if err != nil {
		return domain.PhieuPhanAnh{}, bocPhieu(ctx, "gộp phiếu", err)
	}
	return after, nil
}

// Unmerge takes petition `code` out of its main petition. Permission: `feedback.classify`. The reason is
// MANDATORY (owner, 09/10/2026). The main petition's deadline is NOT lengthened back (ADR 0087 §Hệ quả).
//
// ANY TIME BEFORE THE MERGED PETITION IS CLOSED (owner, 09/10/2026 (c); domain.CheckUnmerge). From
// `cho-dan-xac-nhan` it returns to `dang-xu-ly` in the same statement as the unlink — a TRANSITION, so
// it also gets its `status_changed` outbox row (silent: entering `dang-xu-ly` owes no sentence, ADR 0041
// §Không báo — the unmerge's own message is the citizen's word) and its before/after in the trail.
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
		statusAfter, err := domain.CheckUnmerge(child)
		if err != nil {
			return err
		}
		if err := uc.kho.UnlinkMerge(ctx, tx, child.ID, main.ID, child.TrangThai, statusAfter); err != nil {
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
			ActionUnmergePetition, ActionUnmergeFromMain, statusAfter); err != nil {
			return err
		}
		after = child
		after.MergedInto, after.MergedAt, after.MergedBy = "", time.Time{}, ""
		if statusAfter != child.TrangThai {
			// The store cleared xu_ly_xong_luc with the move (UnlinkMerge says why).
			after.TrangThai, after.XuLyXongLuc = statusAfter, time.Time{}
		}
		if err := uc.writeMergeTimelineRows(ctx, tx, domain.MergeKindUnmerge, after, main, now, actor); err != nil {
			return err
		}
		if statusAfter != child.TrangThai {
			if err := uc.ghiSuKien(ctx, tx, after, statusAfter, now); err != nil {
				return err
			}
		}
		return uc.writeMergeChangedEvent(ctx, tx, after, domain.MergeKindUnmerge, now)
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
//
// `childStatusAfter` differs from child.TrangThai only on an unmerge out of `cho-dan-xac-nhan`; the
// merged petition's entry then carries its status before and after — the transition is audited with
// the act that made it (rule 10, invariant 5).
func writeMergeAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, child, main domain.PhieuPhanAnh,
	deadlineBefore, deadlineAfter time.Time, reason, childAction, mainAction string,
	childStatusAfter domain.TrangThai) error {

	deadline := map[string]any{
		"truoc": map[string]any{"han_xu_ly_xong": lucRaVet(deadlineBefore)},
		"sau":   map[string]any{"han_xu_ly_xong": lucRaVet(deadlineAfter)},
	}
	childDelta := map[string]any{"main_petition": main.MaTraCuu, "main_deadline": deadline}
	if childStatusAfter != child.TrangThai {
		childDelta["truoc"] = map[string]any{"trang_thai": string(child.TrangThai)}
		childDelta["sau"] = map[string]any{"trang_thai": string(childStatusAfter)}
	}
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

// writeMergeTimelineRows writes the act's row on BOTH petitions' timelines (migration 0039): each row
// carries that petition's own status after the act, no assignment pair, and a fixed sentence naming the
// OTHER petition's lookup code (domain.MergeLogText). NOT THE REASON — staff free text that may quote a
// report; it stays on petition_merge_event alone, one copy.
func (uc *XuLyPhanAnh) writeMergeTimelineRows(ctx context.Context, tx *store.ScopedTx, kind domain.MergeKind,
	child, main domain.PhieuPhanAnh, at time.Time, actor audit.Actor) error {

	action := domain.LogActionMerge
	if kind == domain.MergeKindUnmerge {
		action = domain.LogActionUnmerge
	}
	if _, err := uc.writeTimelineRow(ctx, tx, child, action, at, actor,
		domain.MergeLogText(kind, false, main.MaTraCuu), false); err != nil {
		return err
	}
	_, err := uc.writeTimelineRow(ctx, tx, main, action, at, actor,
		domain.MergeLogText(kind, true, child.MaTraCuu), false)
	return err
}

// errMergeOccurrenceZero: the history count came back 0 right after the act appended its row — the
// append and the count did not see the same transaction. A wiring fault (500): an occurrence of 0 is
// malformed by contract, and defaulting it to 1 would collapse a second merge onto the first in comms.
var errMergeOccurrenceZero = errors.New("xu_ly_phan_anh: đếm lịch sử gộp ra 0 ngay sau khi ghi — không phát được tin gộp/tách")

// writeMergeChangedEvent records `petitions.merge_changed.v1` for the MERGED petition `child` (after the
// act), in the act's transaction (events.proto, PetitionMergeChanged).
//
// NO RECIPIENT, NO ROW — writeStatusChangedEvent's rule, for its reason: a staff-booked petition has no
// citizen account, and a message to nobody is guaranteed to dead-letter.
//
// THE PAYLOAD IS FIVE THINGS: the child's code, the kind, the occurrence, the opaque citizen id, and the
// software-composed (label, sentence). NOT the main petition's code (ADR 0087 §4), NOT the reason (rule 3),
// not the commune (the envelope's).
func (uc *XuLyPhanAnh) writeMergeChangedEvent(ctx context.Context, tx *store.ScopedTx, child domain.PhieuPhanAnh,
	kind domain.MergeKind, at time.Time) error {

	if child.CongDanID == "" {
		return nil
	}
	n, err := uc.mergeEvents.CountTx(ctx, tx, child.ID, kind)
	if err != nil {
		return err
	}
	if n <= 0 {
		return errMergeOccurrenceZero
	}
	label, nextStep := domain.MergeCitizenMessage(child, kind)
	if label == "" || nextStep == "" {
		return fmt.Errorf("xu_ly_phan_anh: không có lời báo công dân cho %q", kind)
	}
	msg := &petitionsv1.PetitionMergeChanged{
		LookupCode:     child.MaTraCuu,
		Kind:           string(kind),
		Occurrence:     uint32(n),
		CitizenId:      child.CongDanID,
		CitizenMessage: &petitionsv1.CitizenMessage{StatusLabel: label, NextStep: nextStep},
	}
	// UseProtoNames — the field names the .proto declares, as on status_changed.
	body, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(msg)
	if err != nil {
		return fmt.Errorf("xu_ly_phan_anh: mã hoá sự kiện gộp/tách: %w", err)
	}
	id, err := uc.sinhID()
	if err != nil {
		return fmt.Errorf("xu_ly_phan_anh: sinh mã sự kiện gộp/tách: %w", err)
	}
	return uc.suKien.Chen(ctx, tx, petstore.SuKienDi{
		ID: id, Ten: eventMergeChanged, DoiTuong: child.MaTraCuu, Than: body, XayRaLuc: at,
	})
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
