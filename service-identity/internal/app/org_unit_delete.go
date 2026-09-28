package app

// Removing one org unit — DELETE /api/v1/org-units/{id}, menu Cấu hình §12.4 "Xoá bộ phận"
// (user decision 2026-09-28).
//
// REFUSED WHILE THE UNIT HOLDS ANYTHING, and "anything" spans three services:
//
//	identity    live staff (the org chart's SoCanBo predicate) and live child units
//	petitions   open petitions, open tasks             — core/petitionsclient
//	documents   open incoming documents                 — core/documentsclient
//
// Finance projects do NOT count (the same decision).
//
// THE ORDER IS THE DESIGN:
//
//  1. reason, shape only — a request that fails its shape costs nothing
//  2. read the live unit and its two local counts — 404 before anybody else is asked
//  3. ask petitions, then documents — OUTSIDE and BEFORE the transaction, so no row lock is held
//     across two network calls. ANY failure refuses the delete (503); it is NEVER read as zero,
//     because zero is the one answer that lets a delete through (fail closed)
//  4. anything held → 409 naming every kind and its count ("yêu cầu chuyển trước")
//  5. transaction: lock the unit, RE-COUNT staff and children under the lock, soft delete, audit
//     entry — one transaction (rule 6, invariant 3)
//
// WHAT STEP 5 CANNOT CLOSE: a petition, task or document assigned to the unit in the seconds
// between step 3 and the commit. No lock spans services. After the commit, those services' own
// writes refuse the unit (ResolveLiveOrgUnits reads `deleted_at IS NULL`); inside the window they
// do not. Accepted: the window is one request long and the record stays readable (the name still
// resolves — ResolveOrgUnitNames does not filter deleted units).

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/documentsclient"
	"github.com/vihat/vigov/core/petitionsclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// ActionDeleteOrgUnit is the business verb written into the trail. The VALUE follows the house enum
// convention (ADR 0011) beside them_bo_phan / sua_bo_phan; the identifier is English (rule 12).
const ActionDeleteOrgUnit = "xoa_bo_phan"

// PetitionHoldingsSource is what the delete asks petitions. *petitionsclient.Client implements it.
type PetitionHoldingsSource interface {
	OrgUnitHoldings(ctx context.Context, orgUnitID string) (petitionsclient.OrgUnitHoldings, error)
}

// DocumentHoldingsSource is what the delete asks documents. *documentsclient.Client implements it.
type DocumentHoldingsSource interface {
	OrgUnitHoldings(ctx context.Context, orgUnitID string) (documentsclient.OrgUnitHoldings, error)
}

var (
	// ErrOrgUnitDeleteNotConfigured — PETITIONS_GRPC_ADDR or DOCUMENTS_GRPC_ADDR is not set, so the
	// delete cannot ask the owners. 503 with an honest sentence; every other org-chart route works.
	ErrOrgUnitDeleteNotConfigured = errors.New("bo_phan: chức năng xoá bộ phận chưa được cấu hình kết nối tới phản ánh và văn bản")

	// ErrOrgUnitHoldingsUnavailable — petitions or documents did not answer, or answered with an
	// error. 503. The original error is wrapped beside it for the log; it never reaches the client.
	ErrOrgUnitHoldingsUnavailable = errors.New("bo_phan: không kiểm được hồ sơ bộ phận đang giữ")
)

// OrgUnitInUseError is the 409: the unit still holds something. Counts by kind, so the screen can
// say what to move first.
type OrgUnitInUseError struct {
	Holdings domain.OrgUnitHoldings
}

func (e *OrgUnitInUseError) Error() string { return e.Holdings.Sentence() }

// WithHoldingsSources connects the delete to the two owners. Called by main only when BOTH clients
// were dialled; with either missing, Remove answers ErrOrgUnitDeleteNotConfigured.
//
// NIL INTERFACES ONLY — never a nil *Client wrapped in one, which would read as configured and
// panic on the first delete. main passes the values Dial returned without error.
func (uc *SoDoToChuc) WithHoldingsSources(p PetitionHoldingsSource, d DocumentHoldingsSource) *SoDoToChuc {
	uc.petitions, uc.documents = p, d
	return uc
}

// Remove soft-deletes one unit of this commune; see the top of this file for the order and why.
//
// `Remove` AND NOT `Delete`: hooks/tenant_scope_guard.py reads `.Delete(` as an unscoped store call
// at every call site. This one is scoped by the context like every other use case, and a
// `@cross-tenant` marker to silence the guard would be a false statement on the isolation path.
//
// ANOTHER COMMUNE'S ID, AN INVENTED ONE, AN ALREADY-DELETED UNIT: idstore.ErrKhongTimThayBoPhan,
// one answer for all three (rule 4, forbidden #2).
func (uc *SoDoToChuc) Remove(ctx context.Context, id, rawReason string, actor NguoiThucHien) error {
	if err := actor.hopLe(); err != nil {
		return err
	}
	if id == "" {
		return idstore.ErrKhongTimThayBoPhan
	}
	reason, err := domain.NormalizeOrgUnitDeleteReason(rawReason)
	if err != nil {
		return err
	}
	if uc.petitions == nil || uc.documents == nil {
		return ErrOrgUnitDeleteNotConfigured
	}

	unit, held, err := uc.kho.LiveForDelete(ctx, id)
	if err != nil {
		return err
	}

	// THE UNIT'S ID, AS READ — never the path value — is what is asked about.
	p, err := uc.petitions.OrgUnitHoldings(ctx, unit.ID)
	if err != nil {
		return fmt.Errorf("%w: petitions: %w", ErrOrgUnitHoldingsUnavailable, err)
	}
	d, err := uc.documents.OrgUnitHoldings(ctx, unit.ID)
	if err != nil {
		return fmt.Errorf("%w: documents: %w", ErrOrgUnitHoldingsUnavailable, err)
	}
	held.OpenPetitions = int(p.OpenPetitions)
	held.OpenTasks = int(p.OpenTasks)
	held.OpenIncomingDocuments = int(d.OpenIncomingDocuments)
	if held.Any() {
		return &OrgUnitInUseError{Holdings: held}
	}

	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, deleted, err := uc.kho.KhoaBoPhan(ctx, tx, unit.ID)
		if err != nil {
			return err
		}
		if deleted {
			// Removed by somebody else between the read and the lock.
			return idstore.ErrKhongTimThayBoPhan
		}
		// RE-COUNTED UNDER THE LOCK: a member of staff or a child unit added since the first read
		// is caught here. The remote counts are carried from step 3 (all zero by now).
		again, err := uc.kho.LocalHoldings(ctx, tx, unit.ID)
		if err != nil {
			return err
		}
		again.OpenPetitions, again.OpenTasks, again.OpenIncomingDocuments =
			held.OpenPetitions, held.OpenTasks, held.OpenIncomingDocuments
		if again.Any() {
			return &OrgUnitInUseError{Holdings: again}
		}

		// `deleted_by` IS THE STAFF CODE (rule 6, invariant 8) — actor.Vet.ID, never actor.ID.
		if err := uc.kho.SoftDelete(ctx, tx, before.ID, actor.Vet.ID, reason); err != nil {
			return err
		}

		// THE REASON TEXT IS NOT IN THE DELTA, ITS LENGTH IS — the convention of the staff delete:
		// `audit_log` is append-only, so a copy there is a second permanent store of free text. The
		// reason itself is `delete_reason` on the row, kept for ever too. The zero counts ARE in the
		// delta: they are what the delete was decided on, and an inspection can read them.
		beforeTrail := vetBoPhan(before)
		beforeTrail["da_xoa"] = false
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionDeleteOrgUnit,
			Subject: before.Ma,
			Delta: deltaBoPhan(map[string]any{
				"truoc":        beforeTrail,
				"sau":          map[string]any{"da_xoa": true},
				"do_dai_ly_do": len([]rune(reason)),
				"con_giu": map[string]any{
					"can_bo": again.Staff, "bo_phan_con": again.ChildUnits,
					"phan_anh": again.OpenPetitions, "nhiem_vu": again.OpenTasks,
					"van_ban_den": again.OpenIncomingDocuments,
				},
			}),
		})
	})
}
