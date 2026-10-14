package app

// The use case behind POST /api/v1/roles/defaults — writing the eight template roles of
// docs/ui-ux/14-cau-hinh.md §4.1 (domain.RoleTemplates) into the request's commune.
//
// USER DECISION 2026-09-28 made this explicit, idempotent, per-commune act the ONLY way a commune got
// its working roles (precedent: POST /api/v1/sla/defaults, app/sla.go). OWNER DECISION 15/10/2026
// changed two things: a commune now also creates, renames and soft-deletes its own roles
// (app/role_write.go), and a BRAND-NEW commune receives the templates automatically — inside the
// owner's CreateCommuneAdminAccount transaction, when that act creates the administrator role
// (app/commune_accounts_owner.go, through writeRoleTemplates below). Nothing else calls it
// automatically — not the first `admin` sign-in (app/gieo_quan_tri.go), not startup.
//
// =================================================================================================
// THE FOUR RULES IT RUNS ON, and each one is a refusal a plausible implementation would skip:
//
//	HOLD EVERYTHING YOU GRANT   open question #14, decided: nobody grants a key they do not hold. The
//	                            caller must hold EVERY key any template grants — else the WHOLE run
//	                            is refused and nothing is written. Not "seed the roles I could grant":
//	                            a partial set is a commune configured by accident of who pressed it.
//	EXISTING MEANS UNTOUCHED    a live role with a template's code is left EXACTLY as it is, grants
//	                            included. Missing grants are NOT topped up — vigov-require's
//	                            provisioning.py:131-150 does that and it is deliberately not copied:
//	                            a commune that removed `report.export` from Kế toán would get it back
//	                            by somebody pressing a button, with nothing on screen saying so.
//	DELETED MEANS DECIDED       a soft-deleted role with a template's code is skipped and REPORTED.
//	                            `UNIQUE (tenant_id, ma)` still counts it (0001:120), and reviving it
//	                            would reverse somebody's decision without a trail of who reversed it.
//	ONE TRANSACTION             every role, every grant and every audit entry, or none of them.
//
// `quan-tri-he-thong` IS NOT A TEMPLATE and nothing here reads, writes or re-grants it.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// RoleTemplateRepo is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION:
// there is no signature that would let a role land in one transaction and its audit entry in another.
type RoleTemplateRepo interface {
	LockGrantorSets(ctx context.Context, tx *store.ScopedTx) error
	HeldPermissions(ctx context.Context, tx *store.ScopedTx, staffID string) ([]string, error)
	RoleCodeStates(ctx context.Context, tx *store.ScopedTx, codes []string) (map[string]bool, error)
	InsertRole(ctx context.Context, tx *store.ScopedTx, id string, t domain.RoleTemplate) error
	GrantPermissions(ctx context.Context, tx *store.ScopedTx, roleID string, keys []string, grantedBy string) error
}

// ActionSeedRoleTemplate is the verb in the trail. Vietnamese snake_case like every other action
// this system writes: an inspection reads the string, and a function name would tell them nothing.
const ActionSeedRoleTemplate = "gieo_vai_tro_mau"

// RoleTemplateRef names one role in the result: code and name only.
type RoleTemplateRef struct {
	Code string
	Name string
}

// RoleTemplateResult is what one run did. THREE LISTS AND NOT A COUNT: "already there" and "deleted
// by somebody" are two different things an administrator must be told apart — the second one is
// the commune's own earlier decision, and the screen should say so rather than stay silent.
type RoleTemplateResult struct {
	Created         []RoleTemplateRef
	SkippedExisting []RoleTemplateRef
	SkippedDeleted  []RoleTemplateRef
}

// RoleTemplateSeeder seeds the template roles.
type RoleTemplateSeeder struct {
	db   *store.DB
	repo RoleTemplateRepo

	// Injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

func NewRoleTemplateSeeder(db *store.DB, repo RoleTemplateRepo) *RoleTemplateSeeder {
	return &RoleTemplateSeeder{db: db, repo: repo, newID: ulid.Moi}
}

// SeedDefaults writes the templates this commune does not have yet.
//
// THE ORDER IS LOAD-BEARING: the locks first (store/role_template.go property 3), then the caller's
// keys, then the refusal — all before the first write, so a refused run writes nothing and audits
// nothing. A run that finds every template present is a SUCCESS with an empty `Created`, and writes
// and audits nothing, which is what makes the route's idem.KhongCan declaration true.
func (uc *RoleTemplateSeeder) SeedDefaults(ctx context.Context, actor NguoiThucHien) (RoleTemplateResult, error) {
	if err := actor.hopLe(); err != nil {
		return RoleTemplateResult{}, err
	}

	var res RoleTemplateResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Reset on every attempt: a result surviving into a retried closure would report a total.
		res = RoleTemplateResult{}

		if err := uc.repo.LockGrantorSets(ctx, tx); err != nil {
			return err
		}
		held, err := uc.repo.HeldPermissions(ctx, tx, actor.ID)
		if err != nil {
			return err
		}
		// #14: EVERY key any template grants, whether or not that template will be created on this
		// run. The set a caller may seed does not depend on what the commune happens to have already.
		if missing := khongCam(domain.RoleTemplatePermissions(), held); len(missing) > 0 {
			return &LoiTraoQuyenKhongCam{Thieu: missing}
		}

		res, err = writeRoleTemplates(ctx, tx, uc.repo, uc.newID, actor.Vet)
		return err
	})
	if err != nil {
		return RoleTemplateResult{}, err
	}
	return res, nil
}

// RoleTemplateWriter is the write half of RoleTemplateRepo — what writeRoleTemplates needs, and all
// the owner's CreateCommuneAdminAccount path is given (it takes no grantor locks and checks no #14).
type RoleTemplateWriter interface {
	RoleCodeStates(ctx context.Context, tx *store.ScopedTx, codes []string) (map[string]bool, error)
	InsertRole(ctx context.Context, tx *store.ScopedTx, id string, t domain.RoleTemplate) error
	GrantPermissions(ctx context.Context, tx *store.ScopedTx, roleID string, keys []string, grantedBy string) error
}

// writeRoleTemplates writes, inside the CALLER'S transaction, every template the commune has no row
// for (live or soft-deleted), with its grants and one audit entry each. EXISTING MEANS UNTOUCHED and
// DELETED MEANS DECIDED hold here; the locks and #14 are the caller's (SeedDefaults takes both; the
// owner's brand-new-commune path needs neither — commune_accounts_owner.go says why).
//
// `actor.ID` is the business code that goes into `cap_boi` and `audit_log.actor_id` (rule 6,
// invariant 8): a staff CB- code from the HTTP route, the owner's VH- code from the operator path.
func writeRoleTemplates(ctx context.Context, tx *store.ScopedTx, repo RoleTemplateWriter,
	newID func() (string, error), actor audit.Actor) (RoleTemplateResult, error) {

	var res RoleTemplateResult
	templates := domain.RoleTemplates()
	codes := make([]string, 0, len(templates))
	for _, t := range templates {
		codes = append(codes, t.Code)
	}
	states, err := repo.RoleCodeStates(ctx, tx, codes)
	if err != nil {
		return RoleTemplateResult{}, err
	}

	for _, t := range templates {
		ref := RoleTemplateRef{Code: t.Code, Name: t.Name}
		if deleted, exists := states[t.Code]; exists {
			if deleted {
				res.SkippedDeleted = append(res.SkippedDeleted, ref)
			} else {
				res.SkippedExisting = append(res.SkippedExisting, ref)
			}
			continue
		}

		id, err := newID()
		if err != nil {
			return RoleTemplateResult{}, fmt.Errorf("vai_tro_mau: sinh id vai trò %s: %w", t.Code, err)
		}
		if err := repo.InsertRole(ctx, tx, id, t); err != nil {
			return RoleTemplateResult{}, err
		}
		if err := repo.GrantPermissions(ctx, tx, id, t.Permissions, actor.ID); err != nil {
			return RoleTemplateResult{}, err
		}

		// ONE ENTRY PER ROLE CREATED, IN THIS SAME TRANSACTION (rule 6, invariant 3). Unlike the
		// SLA seed's single entry, each role is its own authority-bearing record with its own
		// grants, and PUT /api/v1/roles/{id}/permissions audits the same role under the same
		// subject later — so an inspection following one role reads one continuous history.
		//
		// SUBJECT IS THE ROLE ID, as HanhViLuuPhanQuyenVaiTro uses: the code and the name are in
		// the delta, and the name can be edited. Nothing here is personal data.
		delta, err := json.Marshal(map[string]any{
			"ma":          t.Code,
			"ten":         t.Name,
			"la_lanh_dao": t.IsLeader,
			"thu_tu":      t.Order,
			"quyen":       t.Permissions,
		})
		if err != nil {
			return RoleTemplateResult{}, fmt.Errorf("vai_tro_mau: dựng delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionSeedRoleTemplate,
			Subject: id,
			Delta:   delta,
		}); err != nil {
			return RoleTemplateResult{}, err
		}
		res.Created = append(res.Created, ref)
	}
	return res, nil
}
