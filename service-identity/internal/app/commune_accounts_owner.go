package app

// The operator OWNER's acts on ONE commune's sign-in accounts (operator.proto ListCommuneStaffAccounts
// / CreateCommuneAdminAccount / ResetStaffPassword; ADR 0048 §"Sửa đổi 13/10/2026 — owner, …" #2, #4,
// #5). The commune is the "x-tenant-id" metadata the interceptor put in ctx (rule 1, invariant 4); every
// statement runs on a ScopedTx of it, so nothing of another commune is reachable.
//
// THE TRAIL IS THE COMMUNE'S OWN audit_log (amendment #5), actor_kind audit.KindOperator, "who" = the
// owner's VH- code, in the transaction of the act. The LIST is audited too: it hands staff names and
// sign-in emails — personal data — to somebody outside the commune (rule 6, invariant 7).
//
// WHY THIS IS NOT "VIHAT TOUCHING BUSINESS DATA" (ADR 0048 stop condition #2) and what it still costs:
// operator.proto and the amendment's "Rủi ro và giá" table. The owner decided it on 13/10/2026.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// Audit verbs (values Vietnamese, ADR 0011). The reset reuses HanhViDatLaiMatKhau — the same act as
// a commune administrator's reset, told apart by actor_kind (TaiKhoanCanBo.ResetByOperator).
const (
	// ActionStaffAccountsReadByOperator — the owner READ the commune's sign-in accounts (names,
	// emails). Subject is the reader's own VH- code: the read is about no single person, and "what did
	// this operator read" is the question an inspection asks of it (the ListActiveStaffCounts shape).
	ActionStaffAccountsReadByOperator = "xem_tai_khoan_can_bo_boi_van_hanh"
	// ActionCommuneAdminCreatedByOperator — the owner created an administrator account here. Distinct
	// from cap_tai_khoan_can_bo (a directory row turned into an account by the commune's own admin) and
	// from gieo_quan_tri_mac_dinh (the system's seed): a different person, a different authority.
	ActionCommuneAdminCreatedByOperator = "tao_quan_tri_xa_boi_van_hanh"
)

// Refusals of the commune-scoped owner acts.
var (
	// ErrCommuneNotLive — the platform does not know the commune, or it is inactive (CreateAdmin only).
	// FAILED_PRECONDITION: an account under a commune nobody can reach is an account nobody uses.
	ErrCommuneNotLive = errors.New("commune accounts: the target commune is not a live commune on the platform")
	// ErrCommuneAdminEmailTaken — the commune already holds this sign-in email, in ANY state (the
	// directory's `UNIQUE (tenant_id, email)` is not partial). ALREADY_EXISTS.
	ErrCommuneAdminEmailTaken = errors.New("commune accounts: this commune already has an account with this email")
	// ErrStaffEmailRequired — a blank email for an account: the email IS the sign-in name.
	ErrStaffEmailRequired = errors.New("commune accounts: an account needs a sign-in email")
)

// CommuneAccountStore is *idstore.CanBoStore's part used here, declared at the point of use.
type CommuneAccountStore interface {
	ListSignInAccounts(ctx context.Context, tx *store.ScopedTx) ([]domain.StaffSignInAccount, error)
	SignInAccountByID(ctx context.Context, tx *store.ScopedTx, id string) (domain.StaffSignInAccount, error)
	VaiTroQuanTriMacDinh(ctx context.Context, tx *store.ScopedTx, idMoi string) (string, bool, error) // vi-name-ok: existing store method (store/quan_tri_mac_dinh.go), reused not renamed
	CapMoiQuyen(ctx context.Context, tx *store.ScopedTx, vaiTroID, capBoi string) (int64, error)      // vi-name-ok: existing store method (store/quan_tri_mac_dinh.go), reused not renamed
	InsertCommuneAdmin(ctx context.Context, tx *store.ScopedTx, in idstore.NewCommuneAdmin) error
}

// StaffPasswordResetter is *TaiKhoanCanBo's operator reset.
type StaffPasswordResetter interface {
	ResetByOperator(ctx context.Context, id, temporaryPassword, reason, operatorID string, actor audit.Actor) (domain.CanBoTomTat, error)
}

// CommuneAccountsByOwner is the use case. Constructed once; the commune comes from each call's ctx.
type CommuneAccountsByOwner struct {
	db       *store.DB
	accounts CommuneAccountStore
	resetter StaffPasswordResetter
	communes tenant.CommuneLookup // UNCACHED: "re-read live from the platform" (operator.proto)

	// Injected so a test can pin them. Production: ulid.Moi, domain.SinhMaCanBo, password.Bam.
	newID   func() (string, error)
	newCode func(time.Time) (string, error)
	hash    func(string) (string, error)
	now     func() time.Time
}

func NewCommuneAccountsByOwner(db *store.DB, accounts CommuneAccountStore, resetter StaffPasswordResetter,
	communes tenant.CommuneLookup) *CommuneAccountsByOwner {
	if db == nil || accounts == nil || resetter == nil || communes == nil {
		panic("app.NewCommuneAccountsByOwner: db, accounts, resetter and communes are required")
	}
	return &CommuneAccountsByOwner{db: db, accounts: accounts, resetter: resetter, communes: communes,
		newID: ulid.Moi, newCode: domain.SinhMaCanBo, hash: password.Bam,
		now: func() time.Time { return time.Now().UTC() }}
}

func operatorAuditActor(op MiniAppOperator) (audit.Actor, error) {
	a, err := operatorActor(op)
	if err != nil {
		return audit.Actor{}, err
	}
	return audit.Actor{ID: a.code, Kind: a.kind, IP: a.ip}, nil
}

// CheckCommuneAdminShape is CreateCommuneAdminAccount's shape, from the values alone: the staff
// directory's own email and name rules (domain.ChuanHoaEmail / ChuanHoaHoTen), email required.
// Returns the normalised values. internal/grpc calls it before resolving the session.
func CheckCommuneAdminShape(email, fullName string) (string, string, error) {
	em, err := domain.ChuanHoaEmail(email)
	if err != nil {
		return "", "", err
	}
	if em == "" {
		return "", "", ErrStaffEmailRequired
	}
	name, err := domain.ChuanHoaHoTen(fullName)
	if err != nil {
		return "", "", err
	}
	return em, name, nil
}

// List returns the commune's sign-in accounts and writes ONE trail entry for the read, in the same
// transaction: the answer leaves only if the entry committed (fail closed — an unaudited read of
// personal data is the state rule 6, invariant 7 forbids).
func (u *CommuneAccountsByOwner) List(ctx context.Context, owner MiniAppOperator) ([]domain.StaffSignInAccount, error) {
	actor, err := operatorAuditActor(owner)
	if err != nil {
		return nil, err
	}
	var out []domain.StaffSignInAccount
	err = u.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		rows, err := u.accounts.ListSignInAccounts(ctx, tx)
		if err != nil {
			return err
		}
		out = rows
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionStaffAccountsReadByOperator, Subject: actor.ID,
			Delta: deltaCanBo(map[string]any{"so_tai_khoan": len(rows)}),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("commune accounts: list: %w", err)
	}
	return out, nil
}

// CreateAdmin creates ONE account holding the commune administrator role, with the owner's temporary
// password, in ONE transaction with its trail entry.
//
// THE ROLE: `quan-tri-he-thong`, through the SAME statement the first `admin` sign-in seeds it with
// (store.VaiTroQuanTriMacDinh) — created when the commune has none, refused when it was soft-deleted
// (ErrVaiTroQuanTriDaXoa: somebody deleted it; reviving it is their call). Every `quyen` key is
// granted (store.CapMoiQuyen, cap_boi = the owner's VH- code) ONLY WHEN THE ROLE IS CREATED HERE. An
// EXISTING role is used as the commune left it: re-filling it would silently undo a grant the commune's
// own administrator withdrew, which is the vendor deciding a commune's authorisation (ADR 0003).
//
// THE CODE: domain.SinhMaCanBo, retried on a collision exactly as DanhBaCanBo.Them and the seed do.
func (u *CommuneAccountsByOwner) CreateAdmin(ctx context.Context, owner MiniAppOperator, email, fullName,
	temporaryPassword string) (domain.StaffSignInAccount, error) {
	actor, err := operatorAuditActor(owner)
	if err != nil {
		return domain.StaffSignInAccount{}, err
	}
	email, fullName, err = CheckCommuneAdminShape(email, fullName)
	if err != nil {
		return domain.StaffSignInAccount{}, err
	}
	if err := domain.KiemTraMatKhauMoi(temporaryPassword); err != nil {
		return domain.StaffSignInAccount{}, err
	}
	// Live, uncached: a commune deactivated a minute ago must not receive an administrator.
	commune, found, err := u.communes.Current(ctx)
	if err != nil {
		return domain.StaffSignInAccount{}, fmt.Errorf("commune accounts: read the commune from the platform: %w", err)
	}
	if !found || !commune.Active {
		return domain.StaffSignInAccount{}, ErrCommuneNotLive
	}
	hash, err := u.hash(temporaryPassword)
	if err != nil {
		return domain.StaffSignInAccount{}, fmt.Errorf("commune accounts: hash temporary password: %w", err)
	}
	accountID, err := u.newID()
	if err != nil {
		return domain.StaffSignInAccount{}, fmt.Errorf("commune accounts: new account id: %w", err)
	}
	roleID, err := u.newID()
	if err != nil {
		return domain.StaffSignInAccount{}, fmt.Errorf("commune accounts: new role id: %w", err)
	}

	for attempt := 0; attempt < soLanThuMa; attempt++ {
		code, err := u.newCode(u.now())
		if err != nil {
			return domain.StaffSignInAccount{}, fmt.Errorf("commune accounts: new staff code: %w", err)
		}
		var created domain.StaffSignInAccount
		err = u.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
			rid, roleCreated, err := u.accounts.VaiTroQuanTriMacDinh(ctx, tx, roleID)
			if err != nil {
				return err
			}
			var granted int64
			if roleCreated {
				if granted, err = u.accounts.CapMoiQuyen(ctx, tx, rid, actor.ID); err != nil {
					return err
				}
			}
			if err := u.accounts.InsertCommuneAdmin(ctx, tx, idstore.NewCommuneAdmin{
				ID: accountID, Code: code, FullName: fullName, Email: email, PasswordHash: hash, RoleID: rid,
			}); err != nil {
				return err
			}
			if created, err = u.accounts.SignInAccountByID(ctx, tx, accountID); err != nil {
				return err
			}
			// No email, no name, no password, no hash (rule 6, forbidden #4): the code names the person.
			return audit.Write(ctx, tx, audit.Entry{
				Actor: actor, Action: ActionCommuneAdminCreatedByOperator, Subject: code,
				Delta: deltaCanBo(map[string]any{
					"vai_tro":           idstore.MaVaiTroQuanTriMacDinh,
					"vai_tro_tao_moi":   roleCreated,
					"so_quyen_cap_them": granted,
					"sau": map[string]any{
						"co_tai_khoan": true, "dang_hoat_dong": true, "phai_doi_mat_khau": true,
					},
					"cot_da_dat": []string{"mat_khau_hash"},
				}),
			})
		})
		switch {
		case errors.Is(err, idstore.ErrMaCanBoDaDung):
			continue // nothing committed; a new code, never a hunt for a free one
		case errors.Is(err, idstore.ErrEmailDaDung):
			return domain.StaffSignInAccount{}, ErrCommuneAdminEmailTaken
		case err != nil:
			return domain.StaffSignInAccount{}, fmt.Errorf("commune accounts: create admin: %w", err)
		}
		return created, nil
	}
	return domain.StaffSignInAccount{}, fmt.Errorf("%w sau %d lần", ErrKhongSinhDuocMa, soLanThuMa)
}

// ResetPassword is TaiKhoanCanBo.DatLai performed by the owner with the owner's typed temporary
// password (ResetByOperator): same refusals — idstore.ErrCanBoKhongTonTai / ErrChuaCoTaiKhoan (not a
// live sign-in account HERE) and ErrTaiKhoanDangKhoa (the commune's administrative lock, not lifted)
// — same session revocation, same verb, one transaction. operatorID is the owner's internal id; it
// never reaches the trail.
func (u *CommuneAccountsByOwner) ResetPassword(ctx context.Context, owner MiniAppOperator, operatorID, userID,
	temporaryPassword, reason string) (domain.StaffSignInAccount, error) {
	actor, err := operatorAuditActor(owner)
	if err != nil {
		return domain.StaffSignInAccount{}, err
	}
	if reason, err = CheckOwnerReason(reason); err != nil {
		return domain.StaffSignInAccount{}, err
	}
	if err := domain.KiemTraMatKhauMoi(temporaryPassword); err != nil {
		return domain.StaffSignInAccount{}, err
	}
	if _, err := u.resetter.ResetByOperator(ctx, userID, temporaryPassword, reason, operatorID, actor); err != nil {
		return domain.StaffSignInAccount{}, err
	}
	// Read back for the response — a read of what just committed, in its own scoped transaction.
	var after domain.StaffSignInAccount
	err = u.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		after, err = u.accounts.SignInAccountByID(ctx, tx, userID)
		return err
	})
	if err != nil {
		return domain.StaffSignInAccount{}, fmt.Errorf("commune accounts: read back after reset: %w", err)
	}
	return after, nil
}
