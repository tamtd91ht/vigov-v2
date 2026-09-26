package app

// The DEFAULT ADMINISTRATOR of a commune, created at that commune's first `admin` sign-in.
//
// OWNER'S DECISION, 2026-09-26 (ledger item `xa-moi-khong-co-vai-tro-va-quyen`): a new commune
// had no production path to its first role, first grant or first administrator, so every
// RequirePermission route answered 403 to every account and nothing inside the commune could
// undo it. The commune is the one the request's Host resolved to at the edge — nothing here
// lists or chooses communes, and nothing a client sends names one (rule 1).
//
// THE CONDITIONS, ALL FOUR, and each one is load-bearing:
//
//	email == "admin"                   the literal the owner chose; no other address ever seeds
//	no `admin` row in ANY state        soft-deleted, locked or directory-only all count: somebody
//	                                   made that choice and seeding would reverse it silently
//	IDENTITY_ADMIN_SEED_PASSWORD set   empty = off; shorter than password.DaiToiThieu = off
//	typed password == that value       constant-time; anything else is an ordinary wrong password
//
// NOTHING HERE ADDS A MESSAGE, A STATUS OR A LOG LINE A CALLER COULD TELL APART. A wrong password
// on a seedable commune costs exactly what an unknown email costs today — no database round trip,
// no argon2 — and answers ErrDangNhapThatBai with the one line thatBai writes. Only the person who
// typed the configured value reaches the transaction below.

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// HanhViGieoQuanTriMacDinh is the audit action of the seed.
const HanhViGieoQuanTriMacDinh = "gieo_quan_tri_mac_dinh"

// ErrMatKhauGieoNgan — IDENTITY_ADMIN_SEED_PASSWORD is set but shorter than password.DaiToiThieu.
// main logs it once, by name, and runs with seeding off. Never carries the value or its length.
var ErrMatKhauGieoNgan = errors.New("gieo_quan_tri: IDENTITY_ADMIN_SEED_PASSWORD ngắn hơn độ dài tối thiểu — không gieo")

// errKhongGieo ends the seed transaction without writing anything: the commune already has an
// `admin` in some state, or a concurrent first sign-in committed one first. Not a failure — the
// caller goes on to the ordinary sign-in, which either finds the winner or refuses as today.
var errKhongGieo = errors.New("gieo_quan_tri: không gieo")

// gieoQuanTri is the switched-on state. nil on DangNhap means off.
type gieoQuanTri struct {
	matKhau secret.Secret

	// Injected so a test can pin them. In production: ulid.Moi, domain.SinhMaCanBo, time.Now.
	sinhID func() (string, error)
	sinhMa func(time.Time) (string, error)
	bayGio func() time.Time
}

// BatGieoQuanTri switches the default-administrator seed on with the configured password.
//
// A value shorter than password.DaiToiThieu leaves it OFF and returns ErrMatKhauGieoNgan: the
// account it would create is the most privileged one in the commune, and password.Bam would
// refuse the value anyway — refusing here, once, at startup, is the place somebody is watching.
// An empty value is simply off and returns nil.
func (uc *DangNhap) BatGieoQuanTri(matKhau secret.Secret) error {
	uc.gieo = nil
	if matKhau.Rong() {
		return nil
	}
	if utf8.RuneCount(matKhau.Lo()) < password.DaiToiThieu {
		return ErrMatKhauGieoNgan
	}
	uc.gieo = &gieoQuanTri{
		matKhau: matKhau,
		sinhID:  ulid.Moi,
		sinhMa:  domain.SinhMaCanBo,
		bayGio:  func() time.Time { return time.Now().UTC() },
	}
	return nil
}

// coTheGieo is the decision taken BEFORE any database work: the literal email and the configured
// password, compared in constant time. Called only after TheoEmail found no live account.
func (uc *DangNhap) coTheGieo(yc YeuCauDangNhap) bool {
	if uc.gieo == nil || yc.Email != idstore.EmailQuanTriMacDinh {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(yc.MatKhau), uc.gieo.matKhau.Lo()) == 1
}

// gieoQuanTriMacDinh creates role + every grant + the `admin` account + the audit entry in ONE
// transaction of the request's commune. errKhongGieo means nothing was written, on purpose.
func (uc *DangNhap) gieoQuanTriMacDinh(ctx context.Context, ip string) error {
	g := uc.gieo

	// Hashed once, outside the loop and the transaction: argon2id is deliberately slow, and a
	// transaction holding the unique-key locks for tens of milliseconds longer than it must is a
	// concurrent first sign-in waiting for nothing.
	bam, err := password.Bam(string(g.matKhau.Lo()))
	if err != nil {
		return fmt.Errorf("gieo_quan_tri: băm mật khẩu: %w", err)
	}
	idNguoi, err := g.sinhID()
	if err != nil {
		return fmt.Errorf("gieo_quan_tri: sinh id tài khoản: %w", err)
	}
	idVaiTro, err := g.sinhID()
	if err != nil {
		return fmt.Errorf("gieo_quan_tri: sinh id vai trò: %w", err)
	}

	for lan := 0; lan < soLanThuMa; lan++ {
		ma, err := g.sinhMa(g.bayGio())
		if err != nil {
			return fmt.Errorf("gieo_quan_tri: sinh mã cán bộ: %w", err)
		}

		err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
			co, err := uc.canBo.CoEmailMoiTrangThai(ctx, tx, idstore.EmailQuanTriMacDinh)
			if err != nil {
				return err
			}
			if co {
				return errKhongGieo
			}

			vaiTroID, taoMoi, err := uc.canBo.VaiTroQuanTriMacDinh(ctx, tx, idVaiTro)
			if err != nil {
				return err
			}
			soQuyen, err := uc.canBo.CapMoiQuyen(ctx, tx, vaiTroID, audit.SystemActor)
			if err != nil {
				return err
			}
			chen, err := uc.canBo.ChenQuanTriMacDinh(ctx, tx, idNguoi, ma, bam, vaiTroID)
			if err != nil {
				return err
			}
			if !chen {
				// A concurrent first sign-in won the email key. Roll back what this transaction
				// did (at most some grant cells the winner also wrote) and sign in against its row.
				return errKhongGieo
			}

			// SAME TRANSACTION (rule 6, invariant 3). The actor is the SYSTEM (invariant 6): the
			// person at the keyboard has no staff code yet, and the account did not create
			// itself. The subject is the new account's business code (invariant 8). The delta
			// names what was created — no password, no hash, no personal data (forbidden #4).
			delta, err := json.Marshal(map[string]any{
				"vai_tro":           idstore.MaVaiTroQuanTriMacDinh,
				"vai_tro_tao_moi":   taoMoi,
				"so_quyen_cap_them": soQuyen,
				"tai_khoan": map[string]any{
					"ma":                ma,
					"email":             idstore.EmailQuanTriMacDinh,
					"phai_doi_mat_khau": true,
				},
			})
			if err != nil {
				return fmt.Errorf("gieo_quan_tri: dựng delta: %w", err)
			}
			return audit.Write(ctx, tx, audit.Entry{
				Actor:   audit.Actor{ID: audit.SystemActor, Kind: "system", IP: ip},
				Action:  HanhViGieoQuanTriMacDinh,
				Subject: ma,
				Delta:   delta,
			})
		})
		// A staff-code collision is retried with a new code, never by hunting for a free one —
		// the same rule as DanhBaCanBo.Them. Nothing was committed.
		if errors.Is(err, idstore.ErrMaCanBoDaDung) {
			continue
		}
		return err
	}
	return fmt.Errorf("%w sau %d lần", ErrKhongSinhDuocMa, soLanThuMa)
}
