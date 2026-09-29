package app

// The use case behind "⬆ Nhập từ Excel" on Cấu hình → Người dùng (14-cau-hinh.md §3; user decision
// 2026-09-29, ADR 0059 §1): each row CREATES a directory entry, optionally ASSIGNS a role, and — when
// it has a work address — ISSUES a sign-in account with a temporary password. ALL OR NOTHING, under
// `admin.user`; a file with ANY role cell additionally needs `admin.role`.
//
//	Preview  plan the file against the commune — WRITES NOTHING, audits nothing, mints nothing
//	Import   plan unlocked; mint + hash the passwords OUTSIDE any transaction; then, in ONE transaction
//	         under the import lock, re-plan and write every row with its entries (rule 6, invariant 3)
//
// IT REUSES THE THREE SINGLE-PERSON PATHS' INTERNALS rather than restating them: the rows go through
// domain.PlanStaffImport (the create form's shape rules); the writes are CanBoStore.Chen, DatVaiTro and
// CapTaiKhoan — the statements behind POST /api/v1/staff, PUT /api/v1/staff/{id}/role and
// POST /api/v1/staff/{id}/account; the entries carry the SAME THREE VERBS (them_can_bo,
// doi_vai_tro_can_bo, cap_tai_khoan_can_bo) and the same delta shapes, plus the batch. What an
// inspection reads for an imported person is what it reads for a person added by hand.
//
// THE GUARDS OF THE ROLE PATH, AND WHICH APPLY HERE:
//
//	`admin.role`   ADR 0059 §1: a non-empty Vai trò column needs it besides the route's `admin.user`.
//	               Checked on the actor's keys read INSIDE the transaction, before any plan.
//	#14 second     the role may not carry a key the actor does not hold — per row, in the plan.
//	#14 first      not reachable: every target is a row this transaction creates, never the actor.
//	#13            not reachable: an import only ADDS people; the administrator set can only grow.
//
// THE TEMPORARY PASSWORDS (#9). Minted by domain.SinhMatKhauTam and hashed with password.Bam BEFORE the
// transaction opens (the reason is on TaiKhoanCanBo.Cap: argon2id holds 19 MiB and tens of
// milliseconds; inside, it would hold the import lock for seconds). They leave this process EXACTLY
// ONCE, in StaffImportResult, for the one 201 response. They are not in any audit delta, any error, any
// log line, and not in the idempotency cache — core/idem never stores a response body, so a retry with
// the same key replays only the batch code (see the handler). The database holds only the argon2id hash.
//
// NO SESSION IS REVOKED, unlike TaiKhoanCanBo.mint: every account here belongs to a row created in this
// same transaction, which cannot have had a session. The call would be a statement that can match
// nothing.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// StaffImportRepo is the store, declared at the point of use. EVERY WRITE TAKES THE TRANSACTION.
// *idstore.CanBoStore satisfies it; four methods keep the existing store names (rule 12, invariant 3).
type StaffImportRepo interface {
	LockStaffImport(ctx context.Context, tx *store.ScopedTx) error
	StaffImportSnapshot(ctx context.Context, tx *store.ScopedTx) (domain.StaffImportSnapshot, error)
	QuyenCuaVaiTro(ctx context.Context, tx *store.ScopedTx, vaiTroID string) ([]string, bool, error) // vi-name-ok: existing CanBoStore method, reused by name
	QuyenDangGiu(ctx context.Context, tx *store.ScopedTx, canBoID string) ([]string, error)          // vi-name-ok: existing CanBoStore method, reused by name
	Chen(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error
	DatVaiTro(ctx context.Context, tx *store.ScopedTx, id, vaiTroID string) error // vi-name-ok: existing CanBoStore method, reused by name
	CapTaiKhoan(ctx context.Context, tx *store.ScopedTx, id, bam string) error    // vi-name-ok: existing CanBoStore method, reused by name

	// StaffTemplateChoices is the one read outside a transaction: the template's two dropdowns.
	StaffTemplateChoices(ctx context.Context) ([]domain.StaffImportOrgUnitChoice, []domain.StaffImportRoleChoice, error)
}

// ErrRoleAssignmentNotPermitted — the file carries a role cell and the actor does not hold
// `admin.role` (ADR 0059 §1). A refusal of the FILE, before anything is planned or written: 403.
var ErrRoleAssignmentNotPermitted = errors.New("nhap_can_bo: tệp có cột Vai trò không trống — cần thêm quyền admin.role")

// StaffImportRejected carries every error of a file that was refused. Nothing was written.
type StaffImportRejected struct {
	Errors []domain.StaffImportError
}

func (e *StaffImportRejected) Error() string {
	return fmt.Sprintf("nhap_can_bo: tệp nhập có %d lỗi — không ghi gì", len(e.Errors))
}

// ImportedStaff is one person planned (Preview: no ID, no code, no password) or created (Import).
//
// TemporaryPassword IS PLAINTEXT, set only by Import and only when AccountIssued. Do not add it to a
// log field, an audit delta, an error, or a second return path. See the file header.
type ImportedStaff struct {
	Row               int
	Staff             domain.CanBoTomTat
	OrgUnitCode       string
	OrgUnitName       string
	RoleID            string
	RoleCode          string
	RoleName          string
	AccountIssued     bool
	TemporaryPassword string
}

// StaffImportResult is what a run planned or created, in file order. Batch is the import's ULID —
// the `lo_nhap` every entry of the file carries — set by Import only.
type StaffImportResult struct {
	Batch  string
	People []ImportedStaff
	Errors []domain.StaffImportError // Preview only; Import returns *StaffImportRejected
}

// staffImportSource marks, in every delta, that the act came from a file.
const staffImportSource = "nhap_excel"

var errStaffPreviewRollback = errors.New("nhap_can_bo: xem trước — huỷ giao dịch")

// errStaffCodeCollision ends one attempt of the write transaction: a minted staff code was already
// taken. The whole transaction rolled back, so the attempt is retried with fresh codes — never by
// hunting for a free one (the reasoning at soLanThuMa).
var errStaffCodeCollision = errors.New("nhap_can_bo: mã cán bộ vừa sinh đã có — sinh lại")

// ErrStaffImportChanged — the locked re-plan accepted an address the unlocked plan had not seen, so no
// password was minted for it. Refused rather than issuing an account without a credential. 409.
var ErrStaffImportChanged = errors.New("nhap_can_bo: danh bạ của xã vừa thay đổi trong lúc nhập")

// StaffImporter owns the staff import.
type StaffImporter struct {
	db   *store.DB
	repo StaffImportRepo

	// Injected so a test can pin them. In production: ulid.Moi, domain.SinhMaCanBo,
	// domain.SinhMatKhauTam, password.Bam, time.Now.
	newID       func() (string, error)
	newCode     func(time.Time) (string, error)
	newPassword func() (string, error)
	hash        func(string) (string, error)
	now         func() time.Time
}

func NewStaffImporter(db *store.DB, repo StaffImportRepo) *StaffImporter {
	return &StaffImporter{
		db: db, repo: repo,
		newID: ulid.Moi, newCode: domain.SinhMaCanBo, newPassword: domain.SinhMatKhauTam, hash: password.Bam,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func staffImportCeilings() domain.StaffImportCeilings {
	return domain.StaffImportCeilings{Staff: idstore.StaffImportDirectoryCeiling, Accounts: idstore.TranDanhBaChonNguoi}
}

// plan reads the snapshot inside tx and plans. It returns ErrRoleAssignmentNotPermitted before planning
// when the file assigns roles and the actor lacks `admin.role`, and fills each role's MissingKeys (#14,
// second constraint) from the actor's keys read in the same transaction.
func (uc *StaffImporter) plan(ctx context.Context, tx *store.ScopedTx, rows []domain.StaffImportRow,
	actor NguoiThucHien) ([]domain.PlannedStaff, []domain.StaffImportError, error) {

	snap, err := uc.repo.StaffImportSnapshot(ctx, tx)
	if err != nil {
		return nil, nil, err
	}
	if domain.StaffImportAssignsRoles(rows) {
		held, err := uc.repo.QuyenDangGiu(ctx, tx, actor.ID)
		if err != nil {
			return nil, nil, err
		}
		if !coQuyen(held, idstore.QuyenPhanQuyen) {
			return nil, nil, ErrRoleAssignmentNotPermitted
		}
		for i := range snap.Roles {
			keys, ok, err := uc.repo.QuyenCuaVaiTro(ctx, tx, snap.Roles[i].ID)
			if err != nil {
				return nil, nil, err
			}
			if !ok {
				// Deleted between the two reads of this transaction: never handed out.
				snap.Roles[i].MissingKeys = []string{"vai trò không còn"}
				continue
			}
			snap.Roles[i].MissingKeys = khongCam(keys, held)
		}
	}
	plan, errs := domain.PlanStaffImport(rows, snap, staffImportCeilings())
	return plan, errs, nil
}

func plannedOut(plan []domain.PlannedStaff) []ImportedStaff {
	out := make([]ImportedStaff, 0, len(plan))
	for _, p := range plan {
		out = append(out, ImportedStaff{
			Row: p.Row, Staff: p.Staff, OrgUnitCode: p.OrgUnitCode, OrgUnitName: p.OrgUnitName,
			RoleID: p.RoleID, RoleCode: p.RoleCode, RoleName: p.RoleName, AccountIssued: p.IssueAccount,
		})
	}
	return out
}

// Preview plans the file and reports. The returned error is a system failure or
// ErrRoleAssignmentNotPermitted; a file with errors is a SUCCESSFUL preview whose Errors is non-empty.
// No lock is taken and nothing is minted.
func (uc *StaffImporter) Preview(ctx context.Context, rows []domain.StaffImportRow, actor NguoiThucHien) (StaffImportResult, error) {
	if err := actor.hopLe(); err != nil {
		return StaffImportResult{}, err
	}
	var res StaffImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		plan, errs, err := uc.plan(ctx, tx, rows, actor)
		if err != nil {
			return err
		}
		res.People, res.Errors = plannedOut(plan), errs
		return errStaffPreviewRollback
	})
	if err != nil && !errors.Is(err, errStaffPreviewRollback) {
		return StaffImportResult{}, err
	}
	return res, nil
}

// credential is one minted temporary password and its hash, keyed by spreadsheet row.
type credential struct{ plain, hash string }

// Import writes the file, or nothing.
func (uc *StaffImporter) Import(ctx context.Context, rows []domain.StaffImportRow, actor NguoiThucHien) (StaffImportResult, error) {
	if err := actor.hopLe(); err != nil {
		return StaffImportResult{}, err
	}

	// 1. AN UNLOCKED PLAN FIRST, so a file with a typo costs one read, not N argon2id hashes.
	var first []domain.PlannedStaff
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		plan, errs, err := uc.plan(ctx, tx, rows, actor)
		if err != nil {
			return err
		}
		if len(errs) > 0 {
			return &StaffImportRejected{Errors: errs}
		}
		first = plan
		return errStaffPreviewRollback
	})
	if err != nil && !errors.Is(err, errStaffPreviewRollback) {
		return StaffImportResult{}, err
	}

	// 2. MINT AND HASH, outside any transaction. A value minted for a row the locked re-plan below
	//    refuses is returned to nobody.
	creds := make(map[int]credential, len(first))
	for _, p := range first {
		if !p.IssueAccount {
			continue
		}
		plain, err := uc.newPassword()
		if err != nil {
			return StaffImportResult{}, fmt.Errorf("nhap_can_bo: sinh mật khẩu tạm: %w", err)
		}
		h, err := uc.hash(plain)
		if err != nil {
			// password.Bam's error names the rule, never the value.
			return StaffImportResult{}, fmt.Errorf("nhap_can_bo: băm mật khẩu tạm: %w", err)
		}
		creds[p.Row] = credential{plain: plain, hash: h}
	}

	batch, err := uc.newID()
	if err != nil {
		return StaffImportResult{}, fmt.Errorf("nhap_can_bo: sinh mã lô nhập: %w", err)
	}

	// 3. THE WRITE, retried as a whole on a staff-code collision (soLanThuMa attempts).
	for attempt := 0; attempt < soLanThuMa; attempt++ {
		res, err := uc.write(ctx, rows, actor, batch, creds)
		if errors.Is(err, errStaffCodeCollision) {
			continue
		}
		if err != nil {
			return StaffImportResult{}, err
		}
		return res, nil
	}
	return StaffImportResult{}, fmt.Errorf("%w sau %d lần", ErrKhongSinhDuocMa, soLanThuMa)
}

func (uc *StaffImporter) write(ctx context.Context, rows []domain.StaffImportRow, actor NguoiThucHien,
	batch string, creds map[int]credential) (StaffImportResult, error) {

	var res StaffImportResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		res = StaffImportResult{Batch: batch}
		if err := uc.repo.LockStaffImport(ctx, tx); err != nil {
			return err
		}
		plan, errs, err := uc.plan(ctx, tx, rows, actor)
		if err != nil {
			return err
		}
		if len(errs) > 0 {
			return &StaffImportRejected{Errors: errs}
		}

		codes := make(map[string]bool, len(plan))
		people := plannedOut(plan)
		for i := range plan {
			p := &plan[i]
			cb := p.Staff
			if cb.ID, err = uc.newID(); err != nil {
				return fmt.Errorf("nhap_can_bo: sinh id: %w", err)
			}
			// A code repeated WITHIN the batch is minted again here; one already in the table is the
			// unique key's refusal below, which retries the whole transaction.
			for n := 0; ; n++ {
				if cb.Ma, err = uc.newCode(uc.now()); err != nil {
					return fmt.Errorf("nhap_can_bo: sinh mã cán bộ: %w", err)
				}
				if !codes[cb.Ma] {
					break
				}
				if n >= soLanThuMa {
					return errStaffCodeCollision
				}
			}
			codes[cb.Ma] = true

			if err := uc.repo.Chen(ctx, tx, cb); err != nil {
				if errors.Is(err, idstore.ErrMaCanBoDaDung) {
					return errStaffCodeCollision
				}
				return err
			}
			if err := audit.Write(ctx, tx, audit.Entry{
				Actor: actor.Vet, Action: HanhViThemCanBo, Subject: cb.Ma,
				Delta: deltaCanBo(map[string]any{
					"sau":   tomTatCanBo(cb), // personal data masked (rule 6, forbidden #4)
					"nguon": staffImportSource, "lo_nhap": batch, "dong": p.Row, "so_dong": len(plan),
				}),
			}); err != nil {
				return err
			}

			if p.RoleID != "" {
				if err := uc.repo.DatVaiTro(ctx, tx, cb.ID, p.RoleID); err != nil {
					return err
				}
				cb.VaiTroID = p.RoleID
				// A privilege change, audited as its own act (rule 5, invariant 5) — the verb and the
				// delta of PUT /api/v1/staff/{id}/role.
				if err := audit.Write(ctx, tx, audit.Entry{
					Actor: actor.Vet, Action: HanhViDoiVaiTro, Subject: cb.Ma,
					Delta: deltaCanBo(map[string]any{
						"truoc": map[string]any{"vai_tro_id": ""},
						"sau":   map[string]any{"vai_tro_id": p.RoleID},
						"nguon": staffImportSource, "lo_nhap": batch, "dong": p.Row,
					}),
				}); err != nil {
					return err
				}
			}

			if p.IssueAccount {
				c, ok := creds[p.Row]
				if !ok {
					return ErrStaffImportChanged
				}
				if err := uc.repo.CapTaiKhoan(ctx, tx, cb.ID, c.hash); err != nil {
					return err
				}
				cb.CoTaiKhoan = true
				// The delta of POST /api/v1/staff/{id}/account: the credential column NAMED, never
				// valued (deltaMatKhau's reasoning).
				if err := audit.Write(ctx, tx, audit.Entry{
					Actor: actor.Vet, Action: HanhViCapTaiKhoan, Subject: cb.Ma,
					Delta: deltaCanBo(map[string]any{
						"truoc":      map[string]any{"co_tai_khoan": false},
						"sau":        map[string]any{"co_tai_khoan": true, "phai_doi_mat_khau": true},
						"cot_da_dat": []string{"mat_khau_hash"},
						"nguon":      staffImportSource, "lo_nhap": batch, "dong": p.Row,
					}),
				}); err != nil {
					return err
				}
				people[i].TemporaryPassword = c.plain
			}
			people[i].Staff = cb
		}
		res.People = people
		return nil
	})
	if err != nil {
		return StaffImportResult{}, err
	}
	return res, nil
}

// TemplateChoices returns the dropdown values of the import template. Writes nothing, audits nothing.
func (uc *StaffImporter) TemplateChoices(ctx context.Context) ([]domain.StaffImportOrgUnitChoice, []domain.StaffImportRoleChoice, error) {
	return uc.repo.StaffTemplateChoices(ctx)
}
