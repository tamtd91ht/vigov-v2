package http

// Display names on the task responses (owner decision 15/10/2026, bug sheet web-admin-141026, menu
// Nhiệm vụ): `assignee_name`, `assigner_name` and `unit_name` beside the codes the task stores.
//
// WHY THE SERVER RESOLVES THEM. The screen used to look names up in /staff-directory, which lists only
// ACTIVE accounts — so a task held by a locked or removed member of staff, or by somebody with no
// account, showed `CB-2026-…`, and a task held by a removed unit showed a ULID. identity's two "name
// behind a stored key" lookups answer exactly those cases (removed records included, ADR 0034):
// ResolveStaffNames and ResolveOrgUnitNames, the pair the Sổ theo dõi export already uses
// (app.TaskRegisterExport). No new RPC.
//
// THE NAMES ARE PERSONAL DATA OF STAFF (rule 3) shown to staff on staff routes only. Nothing here logs
// a name or a code; no citizen route builds nhiemVuRa.

import (
	"context"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// TaskNameLookups is identity's two name lookups for keys a STORED task carries.
// *identityclient.Client satisfies it as it is.
//
// ⚠ A RESOLVED NAME DECIDES NOTHING: it may resolve precisely because the person or unit was removed.
// "May this person be assigned" and "may this unit hold work" are other contracts' questions.
type TaskNameLookups interface {
	// vi-name-ok: the method name of the existing core/identityclient.Client this interface must match
	TenCanBoTheoMa(ctx context.Context, ma []string) (map[string]identityclient.TenCanBo, error)
	OrgUnitNames(ctx context.Context, ids []string) (map[string]identityclient.OrgUnitName, error)
}

// taskNameBook holds the names for one response — one page, or one task.
type taskNameBook struct {
	staff map[string]identityclient.TenCanBo
	units map[string]identityclient.OrgUnitName
}

// taskNames resolves every staff code and unit id the tasks carry: de-duplicated, ONE lookup per kind
// per response, chunked to the contracts' ceilings (skills/load-data-once — never one call per row).
//
// # IDENTITY UNREACHABLE = THE RESPONSE WITHOUT NAMES, NEVER A FAILED LIST (owner decision 15/10/2026)
//
// The names are a convenience over the codes the response already carries, and the screen falls back
// to the code when a name is absent. Failing the register because identity is slow would take away the
// work list itself — a worse outcome than a code in place of a name. This is the OPPOSITE of the
// residential-unit names on petitions (writeResidentialUnitNamesUnavailable answers 503), and the
// difference is deliberate: there the name is the only place statement the record makes; here the
// code next to it still identifies the person. Each lookup fails on its own — a staff outage leaves the
// unit names in place. Logged at Warn with the commune and the error chain only (no code, no name).
//
// Deps.TaskNames nil (a harness that does not wire it) answers the same way: no names.
func (h *Handler) taskNames(ctx context.Context, tasks []domain.NhiemVu) taskNameBook {
	book := taskNameBook{}
	if h.d.TaskNames == nil || len(tasks) == 0 {
		return book
	}

	var staff, units []string
	seenStaff, seenUnit := map[string]struct{}{}, map[string]struct{}{}
	addStaff := func(code string) {
		if _, dup := seenStaff[code]; code != "" && !dup {
			seenStaff[code] = struct{}{}
			staff = append(staff, code)
		}
	}
	for _, n := range tasks {
		addStaff(n.NguoiThucHienMa)
		addStaff(n.LanhDaoGiaoViecMa)
		if _, dup := seenUnit[n.BoPhanID]; n.BoPhanID != "" && !dup {
			seenUnit[n.BoPhanID] = struct{}{}
			units = append(units, n.BoPhanID)
		}
	}

	if len(staff) > 0 {
		got, err := lookupInChunks(ctx, staff, identityclient.TranMaMotLo, h.d.TaskNames.TenCanBoTheoMa)
		if err != nil {
			h.d.Log.Warn("nhiệm vụ: không tra được tên cán bộ — trả danh sách không kèm tên",
				"xa", string(tenant.MustFrom(ctx)), "so_ma", len(staff), "err", err)
		} else {
			book.staff = got
		}
	}
	if len(units) > 0 {
		got, err := lookupInChunks(ctx, units, identityclient.MaxLookupKeysPerCall, h.d.TaskNames.OrgUnitNames)
		if err != nil {
			h.d.Log.Warn("nhiệm vụ: không tra được tên bộ phận — trả danh sách không kèm tên",
				"xa", string(tenant.MustFrom(ctx)), "so_bo_phan", len(units), "err", err)
		} else {
			book.units = got
		}
	}
	return book
}

// name fills the three name fields of one response from the book. An absent key leaves the field
// empty (omitted on the wire): unknown, another commune's, or the lookup failed — the code is still there.
func (b taskNameBook) name(ra *nhiemVuRa, n domain.NhiemVu) {
	ra.AssigneeName = b.staff[n.NguoiThucHienMa].HoTen
	ra.AssignerName = b.staff[n.LanhDaoGiaoViecMa].HoTen
	ra.UnitName = b.units[n.BoPhanID].Name
}

// namedTask is nhiemVuRaNgoai plus the names, for a response carrying ONE task — the detail read and
// the write replies. After a committed write a failed lookup must not become an error the officer
// would retry: the act happened, and the reply goes out without names (taskNames never fails).
func (h *Handler) namedTask(ctx context.Context, n domain.NhiemVu) nhiemVuRa {
	ra := nhiemVuRaNgoai(n)
	h.taskNames(ctx, []domain.NhiemVu{n}).name(&ra, n)
	return ra
}

// namedTasks is the same for a page: one book for every row.
func (h *Handler) namedTasks(ctx context.Context, tasks []domain.NhiemVu) []nhiemVuRa {
	book := h.taskNames(ctx, tasks)
	out := make([]nhiemVuRa, 0, len(tasks))
	for _, n := range tasks {
		ra := nhiemVuRaNgoai(n)
		book.name(&ra, n)
		out = append(out, ra)
	}
	return out
}
