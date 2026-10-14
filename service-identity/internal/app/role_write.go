package app

// The use cases behind the commune's OWN role writes on the Phân quyền screen (14-cau-hinh.md §4):
//
//	Add     POST /api/v1/roles                 a new job title, holding NO permission yet
//	Edit    PUT  /api/v1/roles/{id}            rename it, change its leader flag
//	Remove  POST /api/v1/roles/{id}/deletion   soft-delete it, refused while anybody holds it
//
// OWNER DECISION 15/10/2026: a "vai trò" is a staff job title carrying a permission set (Chủ tịch
// UBND, Phó Chủ tịch, Bí thư…); a member of staff may hold several (migration 0031). This reverses the
// 28/09/2026 "no create-role route" note (domain/role_template.go).
//
// OPEN QUESTION #14 ("nobody grants a key they do not hold") IS NOT ENGAGED HERE, and that is checked,
// not assumed: Add writes no `vai_tro_quyen` row, Edit and Remove touch none. A role's keys are ticked
// afterwards through PUT /api/v1/roles/{id}/permissions, which carries #14 in full. Giving the role to
// a person goes through the staff routes, which carry #14 on the union of the roles' keys.
//
// `quan-tri-he-thong` IS NOT THE COMMUNE'S TO EDIT: the first `admin` sign-in and the owner's
// CreateCommuneAdminAccount find it BY ITS CODE and treat it as the administrator role. Renaming it
// would leave a "Bí thư" column holding every key; deleting it would make the next seed refuse
// (ErrVaiTroQuanTriDaXoa). Both refused with ErrSystemRole.
//
// THE METHOD NAMES Add / Edit / Remove are chosen for hooks/tenant_scope_guard.py, which reads the
// usual CRUD verbs as unscoped store calls at every call site (the reason SoDoToChuc.Remove gives).

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// RoleWriteRepo is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION: the
// write and its audit entry cannot land in two.
type RoleWriteRepo interface {
	LockCatalogue(ctx context.Context, tx *store.ScopedTx) error
	LiveRoles(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingRole, error)
	CodesWithBase(ctx context.Context, tx *store.ScopedTx, base string) ([]string, error)
	InsertRole(ctx context.Context, tx *store.ScopedTx, r domain.ExistingRole) error
	LockRole(ctx context.Context, tx *store.ScopedTx, id string) (domain.ExistingRole, error)
	UpdateRole(ctx context.Context, tx *store.ScopedTx, id, name string, isLeader bool) error
	CountHolders(ctx context.Context, tx *store.ScopedTx, roleID string) (int, error)
	SoftDeleteRole(ctx context.Context, tx *store.ScopedTx, id, deletedBy, reason string) (time.Time, error)
}

// The business verbs written into the trail — Vietnamese snake_case beside gieo_vai_tro_mau and
// luu_phan_quyen_vai_tro (ADR 0011); identifiers English (rule 12).
const (
	ActionAddRole    = "them_vai_tro"
	ActionEditRole   = "sua_vai_tro"
	ActionRemoveRole = "xoa_vai_tro"
)

var (
	// ErrSystemRole — the target is `quan-tri-he-thong`. 409 system_role.
	ErrSystemRole = errors.New("vai_tro: không sửa hay xoá được vai trò quản trị hệ thống")

	// ErrRoleCatalogueFull — the commune already has idstore.TranDanhMucVaiTro live roles. Refused
	// here because past that ceiling GET /api/v1/roles and the matrix REFUSE to answer (they never
	// truncate), so one more role would break the Phân quyền screen for the whole commune. 409.
	ErrRoleCatalogueFull = errors.New("vai_tro: xã đã có số vai trò tối đa")
)

// RoleInUseError is the 409 role_in_use: live staff still hold the role.
type RoleInUseError struct{ Holders int }

func (e *RoleInUseError) Error() string {
	return fmt.Sprintf("vai_tro: còn %d cán bộ đang giữ vai trò", e.Holders)
}

// RoleInput is the body of create and edit: both fields, always — an edit that forgot the leader flag
// must not silently clear it (the HTTP layer refuses a missing field).
type RoleInput struct {
	Name     string
	IsLeader bool
}

// RoleDeletion is what Remove wrote.
type RoleDeletion struct {
	Role      domain.VaiTro
	DeletedAt time.Time
	DeletedBy string // the remover's staff code
}

// RoleCatalogue owns the commune's own role writes.
type RoleCatalogue struct {
	db   *store.DB
	repo RoleWriteRepo

	// Injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

func NewRoleCatalogue(db *store.DB, repo RoleWriteRepo) *RoleCatalogue {
	if db == nil || repo == nil {
		panic("app.NewRoleCatalogue: db and repo are required")
	}
	return &RoleCatalogue{db: db, repo: repo, newID: ulid.Moi}
}

// normalizeRoleName is NFC then the domain's shape. NFC HERE because the domain is standard-library
// only, and a name typed in decomposed form ("Bí thư" as i + U+0301) would otherwise slip past the
// duplicate check and sit beside its composed twin as two roles that look identical.
func normalizeRoleName(raw string) (string, error) {
	return domain.NormalizeRoleName(norm.NFC.String(raw))
}

// nfcRoles normalises the names the duplicate check compares against: a row written before NFC was
// applied must still collide with its composed twin.
func nfcRoles(live []domain.ExistingRole) []domain.ExistingRole {
	out := make([]domain.ExistingRole, len(live))
	for i, r := range live {
		r.Name = norm.NFC.String(r.Name)
		out[i] = r
	}
	return out
}

func roleTrail(r domain.ExistingRole) map[string]any {
	return map[string]any{"ma": r.Code, "ten": r.Name, "la_lanh_dao": r.IsLeader, "thu_tu": r.Order}
}

func roleView(r domain.ExistingRole) domain.VaiTro {
	return domain.VaiTro{ID: r.ID, Ma: r.Code, Ten: r.Name, LaLanhDao: r.IsLeader}
}

// Add creates one role of the commune, holding no permission.
//
// THE CODE IS DERIVED, never typed: the name's slug, suffixed -2, -3 … past every code the commune has
// EVER issued (soft-deleted roles included) and never `quan-tri-he-thong` (domain.PickRoleCode).
// `thu_tu` is after the last live role, so the new column appears at the matrix's right edge.
func (uc *RoleCatalogue) Add(ctx context.Context, in RoleInput, actor NguoiThucHien) (domain.VaiTro, error) {
	if err := actor.hopLe(); err != nil {
		return domain.VaiTro{}, err
	}
	// Shape first, outside the transaction: a request that fails its shape holds no lock.
	name, err := normalizeRoleName(in.Name)
	if err != nil {
		return domain.VaiTro{}, err
	}
	base, err := domain.RoleCodeBase(name)
	if err != nil {
		return domain.VaiTro{}, err
	}

	var created domain.ExistingRole
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.repo.LockCatalogue(ctx, tx); err != nil {
			return err
		}
		live, err := uc.repo.LiveRoles(ctx, tx)
		if err != nil {
			return err
		}
		if len(live) >= idstore.TranDanhMucVaiTro {
			return ErrRoleCatalogueFull
		}
		if err := domain.CheckRoleNameFree(name, "", nfcRoles(live)); err != nil {
			return err
		}
		taken, err := uc.repo.CodesWithBase(ctx, tx, base)
		if err != nil {
			return err
		}
		code, ok := domain.PickRoleCode(base, taken)
		if !ok {
			return idstore.ErrRoleCodeTaken
		}
		id, err := uc.newID()
		if err != nil {
			return fmt.Errorf("vai_tro: sinh id: %w", err)
		}
		created = domain.ExistingRole{ID: id, Code: code, Name: name, IsLeader: in.IsLeader,
			Order: domain.NextRoleOrder(live)}
		if err := uc.repo.InsertRole(ctx, tx, created); err != nil {
			return err
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). SUBJECT IS THE ROLE ID, as
		// gieo_vai_tro_mau and luu_phan_quyen_vai_tro use, so one role's history reads as one
		// continuous trail. `quyen: []` states in the record that the role was born holding nothing.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionAddRole,
			Subject: id,
			Delta:   deltaCanBo(map[string]any{"sau": roleTrail(created), "quyen": []string{}}),
		})
	})
	if err != nil {
		return domain.VaiTro{}, err
	}
	return roleView(created), nil
}

// Edit renames one role or changes its leader flag. The code never changes.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — the same discipline as SoDoToChuc.Sua, and what makes
// the route's idem.KhongCan declaration true.
func (uc *RoleCatalogue) Edit(ctx context.Context, id string, in RoleInput, actor NguoiThucHien) (domain.VaiTro, error) {
	if err := actor.hopLe(); err != nil {
		return domain.VaiTro{}, err
	}
	if err := checkRoleID(id); err != nil {
		return domain.VaiTro{}, err
	}
	name, err := normalizeRoleName(in.Name)
	if err != nil {
		return domain.VaiTro{}, err
	}

	var after domain.ExistingRole
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// LOCK ORDER: catalogue, then row (store/role_write.go).
		if err := uc.repo.LockCatalogue(ctx, tx); err != nil {
			return err
		}
		before, err := uc.repo.LockRole(ctx, tx, id)
		if err != nil {
			return err
		}
		if domain.IsSystemRole(before.Code) {
			return ErrSystemRole
		}
		after = before
		after.Name, after.IsLeader = name, in.IsLeader
		if before.Name == after.Name && before.IsLeader == after.IsLeader {
			return nil
		}
		live, err := uc.repo.LiveRoles(ctx, tx)
		if err != nil {
			return err
		}
		if err := domain.CheckRoleNameFree(name, before.ID, nfcRoles(live)); err != nil {
			return err
		}
		if err := uc.repo.UpdateRole(ctx, tx, before.ID, after.Name, after.IsLeader); err != nil {
			return err
		}
		// Before and after, both significant fields; nothing personal (rule 6, invariant 5).
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionEditRole,
			Subject: before.ID,
			Delta: deltaCanBo(map[string]any{
				"ma":    before.Code,
				"truoc": map[string]any{"ten": before.Name, "la_lanh_dao": before.IsLeader},
				"sau":   map[string]any{"ten": after.Name, "la_lanh_dao": after.IsLeader},
			}),
		})
	})
	if err != nil {
		return domain.VaiTro{}, err
	}
	return roleView(after), nil
}

// Remove soft-deletes one role of this commune.
//
// REFUSED WHILE ANY LIVE MEMBER OF STAFF HOLDS IT (RoleInUseError), counted under the row lock — the
// lock that makes the count hold is argued on idstore.RoleWriteStore.LockRole. Reassigning those people
// first is an administrative act on the Người dùng screen, never something a deletion does for them.
//
// THE ROLE'S GRANTS ARE LEFT AS THEY ARE (rule 7): every read of grants drops a deleted role, and the
// rows record what the role could do when it was retired. Its code stays taken for ever, so a later
// role of the same name gets `<code>-2` and POST /api/v1/roles/defaults reports a deleted template as
// "đã xoá" rather than reviving it.
func (uc *RoleCatalogue) Remove(ctx context.Context, id, rawReason string, actor NguoiThucHien) (RoleDeletion, error) {
	if err := actor.hopLe(); err != nil {
		return RoleDeletion{}, err
	}
	if err := checkRoleID(id); err != nil {
		return RoleDeletion{}, err
	}
	reason, err := domain.NormalizeRoleDeleteReason(norm.NFC.String(rawReason))
	if err != nil {
		return RoleDeletion{}, err
	}

	var out RoleDeletion
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.repo.LockCatalogue(ctx, tx); err != nil {
			return err
		}
		before, err := uc.repo.LockRole(ctx, tx, id)
		if err != nil {
			return err
		}
		if domain.IsSystemRole(before.Code) {
			return ErrSystemRole
		}
		holders, err := uc.repo.CountHolders(ctx, tx, before.ID)
		if err != nil {
			return err
		}
		if holders > 0 {
			return &RoleInUseError{Holders: holders}
		}
		// `deleted_by` IS THE STAFF CODE (rule 6, invariant 8) — actor.Vet.ID, never actor.ID.
		at, err := uc.repo.SoftDeleteRole(ctx, tx, before.ID, actor.Vet.ID, reason)
		if err != nil {
			return err
		}
		out = RoleDeletion{Role: roleView(before), DeletedAt: at, DeletedBy: actor.Vet.ID}

		// The reason IS in the delta (unlike xoa_bo_phan, which keeps only its length): removing a job
		// title is a decision about the authority's structure, and "why" is what an inspection asks.
		// It is the administrator's sentence about a role, not about a person.
		trailBefore := roleTrail(before)
		trailBefore["da_xoa"] = false
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionRemoveRole,
			Subject: before.ID,
			Delta: deltaCanBo(map[string]any{
				"truoc":           trailBefore,
				"sau":             map[string]any{"da_xoa": true},
				"ly_do":           reason,
				"can_bo_dang_giu": holders,
			}),
		})
	})
	if err != nil {
		return RoleDeletion{}, err
	}
	return out, nil
}

// checkRoleID refuses an empty or over-long path id before SQL — as "role not found", because no
// role has it (one answer for every id that names no live role of this commune).
func checkRoleID(id string) error {
	if id == "" || domain.KiemTraIDThamChieu(id) != nil {
		return idstore.ErrVaiTroKhongTonTaiDeGhi
	}
	return nil
}
