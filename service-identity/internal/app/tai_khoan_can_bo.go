package app

// The three use cases that issue, reset and change a staff credential — the flow that turns a
// DIRECTORY ROW into an ACCOUNT, and the flow that lets the person take ownership of it.
//
// THREE CUSTOMER DECISIONS OF 2026-09-22 ARE WHAT THIS FILE IMPLEMENTS. Each is quoted where it
// decides something; none of them is an agent's reading of what was probably meant.
//
//	#9   THE SYSTEM MINTS A TEMPORARY PASSWORD and forces a change at the FIRST sign-in. Not an
//	     activation e-mail: a commune's mail server is per-commune configuration and may not be
//	     filled in at all, so the flow that depends on it abandons exactly the commune that has
//	     just been onboarded. The consequence that matters is the last sentence of the decision:
//	     AFTER THE FIRST SIGN-IN THE ADMINISTRATOR NO LONGER KNOWS ANYBODY'S PASSWORD — which is
//	     what rule 6, invariant 2 needs in order for "who did this" to name a person.
//	#17  NO SELF-SERVICE RESET. The commune's administrator resets it for them, through
//	     DatLai below. The two screens the specification draws — `/quen-mat-khau` and
//	     `/dat-lai-mat-khau/:token` — are NOT built, and their absence is the decision.
//	#14  NOBODY ACTS ON THEIR OWN ACCOUNT through an administrator route. Applied here to both
//	     administrator paths — see the note on DatLai, where it stops something specific.
//
// THE TEMPORARY PASSWORD LEAVES THIS PROCESS EXACTLY ONCE, IN THE RETURN VALUE, AND THAT IS THE
// WHOLE OF ITS HANDLING. It is not written to the database (only its argon2id hash is), not put in
// the audit delta, not logged at any level, and not recoverable afterwards by anybody including the
// administrator who minted it. Rule 3 forbids personal data in a log line; a credential is the
// harder case, because a leaked one is not merely readable — it is USABLE, under the name of a
// member of staff whose name then appears on archival records (rule 6, invariant 2).
//
// WHAT THIS FILE DELIBERATELY DOES NOT DO:
//
//	WITHDRAW AN ACCOUNT   there is no `HuyTaiKhoan`. Taking a sign-in account away from somebody
//	                      who keeps their directory entry is a third act with a third consequence,
//	                      and #10's reading — a retirement is a LOCK, not a removal — already
//	                      covers the situation a commune actually meets. Nothing in the `quyen`
//	                      table distinguishes it from `admin.user` either; adding a key is rule 5,
//	                      invariant 3c and a finding for #27.
//	MINT A PASSWORD ON    POST /api/v1/staff writes `co_tai_khoan = false` and an empty hash as
//	CREATE                LITERALS (store/can_bo_ghi.go). Adding an account there would put the
//	                      one-time plaintext into the response of a route whose body a commune
//	                      fills in from a form — and would give an account to the 26-person
//	                      directory, of whom the specification expects 12 to have one.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// KhoTaiKhoanCanBo is the store, declared at the point of use.
//
// TWO READS AND NOT ONE, AND THE SPLIT IS A SECURITY PROPERTY RATHER THAN A CONVENIENCE.
// TheoIDDeGhi comes back as domain.CanBoTomTat, WHICH HAS NO PASSWORD FIELD AT ALL, and it is what
// the two administrator paths use: an administrator issuing or resetting a credential never needs
// to see the one being replaced, so the column is never selected and cannot leak through a handler
// that forgot to drop it. TheoIDDeGhiMatKhau returns domain.CanBo, hash included, and exactly one
// caller has a reason for it — the person proving they know their current password.
//
// EVERY MUTATING METHOD TAKES THE TRANSACTION, so there is no signature in which the credential
// write and its audit entry could land in two transactions (rule 6, invariant 3).
type KhoTaiKhoanCanBo interface {
	TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CanBoTomTat, error)
	TheoIDDeGhiMatKhau(ctx context.Context, tx *store.ScopedTx, id string) (domain.CanBo, error)
	CapTaiKhoan(ctx context.Context, tx *store.ScopedTx, id, bam string) error
	DatMatKhauTam(ctx context.Context, tx *store.ScopedTx, id, bam string) error
	DoiMatKhauChinhMinh(ctx context.Context, tx *store.ScopedTx, id, bam string) error
}

// KhoPhienTaiKhoan revokes the open sessions of one account.
//
// A SEPARATE INTERFACE FROM KhoTaiKhoanCanBo, although *idstore.PhienStore and *idstore.CanBoStore
// are two different types anyway: what it declares is that these use cases may END sessions and do
// nothing else with the registry. They cannot open one, cannot read one, cannot see a sid.
//
// WHY IT IS HERE AT ALL — skills/session-and-token, REQUIRED #7: changing a password revokes every
// open session. A password change that leaves the old sessions working has not actually taken
// anything back, which is the entire reason an administrator is resetting it.
type KhoPhienTaiKhoan interface {
	ThuHoiCuaCanBo(ctx context.Context, tx *store.ScopedTx, nguoiDungID, lyDo string) error
}

// The refusals this layer owns. ErrTuThaoTacChinhMinh is NOT redeclared — it is the one from
// danh_ba_can_bo.go, because it is the same rule (#14) and two sentinels for one rule are two
// things a handler has to remember to map.
var (
	// ErrDaCoTaiKhoan — the person already has a sign-in account, so this is a RESET and must go
	// through the route that says so. Two acts, two audit verbs: "this person was given access to
	// the system" and "this person's password was replaced" are not the same entry in a ledger
	// that is never deleted.
	ErrDaCoTaiKhoan = errors.New("tai_khoan_can_bo: cán bộ này đã có tài khoản")

	// ErrChuaCoTaiKhoan — the mirror. Resetting the password of a directory-only row would be
	// minting an account through the reset route, and the account flow carries decisions the reset
	// route does not.
	ErrChuaCoTaiKhoan = errors.New("tai_khoan_can_bo: cán bộ này chưa có tài khoản")

	// ErrTaiKhoanDangKhoa — issuing a credential to a locked person produces an account that
	// cannot sign in, and the administrator would find that out by reading a password down the
	// telephone to somebody it does not work for. `dang_hoat_dong` says nothing on a directory row
	// (domain.CanBo), but the moment this operation succeeds it starts to mean something, so it is
	// checked at the one act that gives it meaning.
	ErrTaiKhoanDangKhoa = errors.New("tai_khoan_can_bo: tài khoản đang bị khoá")

	// ErrMatKhauHienTaiSai — the person did not prove they know the password they are replacing.
	//
	// THE PROOF IS NOT A FORMALITY IN THIS ENVIRONMENT. Open question #18's own reasoning: a
	// machine at the one-stop-shop counter is SHARED, and several people sit at it during a day.
	// Without this check, anybody who finds an unattended signed-in browser owns that account
	// permanently — they set a password its owner does not know, and every action taken afterwards
	// is recorded under the owner's name.
	ErrMatKhauHienTaiSai = errors.New("tai_khoan_can_bo: mật khẩu hiện tại không đúng")
)

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// service already writes (`dang_nhap`, `them_can_bo`): an inspection reads these strings.
//
// THREE VERBS AND NOT ONE `doi_mat_khau`, because the three are three different facts about
// authority, and a ledger that spells them the same cannot answer the question anybody actually
// asks of it:
//
//	cap_tai_khoan_can_bo    somebody was GIVEN ACCESS to a government system. Read years later,
//	                        this is the entry that says when a person could first act at all.
//	dat_lai_mat_khau_can_bo an administrator replaced SOMEBODY ELSE'S credential (#17). Between
//	                        this entry and that person's next `doi_mat_khau`, two people could
//	                        sign in as them — which is exactly the window an inspection asks about.
//	doi_mat_khau            the person took ownership of their own account. It is what closes the
//	                        window above, and #9's guarantee is precisely that it happened.
const (
	HanhViCapTaiKhoan   = "cap_tai_khoan_can_bo"
	HanhViDatLaiMatKhau = "dat_lai_mat_khau_can_bo"
	HanhViDoiMatKhau    = "doi_mat_khau"
)

// The reasons written onto a revoked session row. They are read by an administrator looking at why
// somebody was signed out, so they are sentences rather than codes.
const (
	lyDoThuHoiCapTaiKhoan = "cấp tài khoản"
	lyDoThuHoiDatLai      = "quản trị viên đặt lại mật khẩu"
	lyDoThuHoiDoiMatKhau  = "đổi mật khẩu"
)

// TaiKhoanCanBo owns issuing, resetting and changing one commune's staff credentials.
type TaiKhoanCanBo struct {
	db    *store.DB
	kho   KhoTaiKhoanCanBo
	phien KhoPhienTaiKhoan

	// Injected so a test can pin them. In production: domain.SinhMatKhauTam, password.Bam,
	// password.KiemTra.
	//
	// THE TWO argon2 FUNCTIONS ARE INJECTED FOR A SECOND REASON BESIDES PINNING: the real ones
	// cost 19 MiB and tens of milliseconds per call, and a table-driven test of these three use
	// cases would spend minutes hashing values nothing asserts about. A test that is slow is a
	// test somebody stops running.
	sinhMatKhau func() (string, error)
	bam         func(string) (string, error)
	kiemTraBam  func(matKhau, bam string) error
}

func NewTaiKhoanCanBo(db *store.DB, kho KhoTaiKhoanCanBo, phien KhoPhienTaiKhoan) *TaiKhoanCanBo {
	return &TaiKhoanCanBo{
		db: db, kho: kho, phien: phien,
		sinhMatKhau: domain.SinhMatKhauTam,
		bam:         password.Bam,
		kiemTraBam:  password.KiemTra,
	}
}

// KetQuaCapMatKhau carries the ONE-TIME plaintext back to the handler, and nothing else does.
//
// THERE IS NO PLACE THIS VALUE IS PUT OTHER THAN THIS STRUCT. Do not add it to a log field, to the
// audit delta, to an error, or to a second return path "for the tests". The administrator reads it
// to the person and it is gone; the database holds only the argon2id hash, which cannot be turned
// back into it. That irreversibility IS #9's guarantee, and it is one line of carelessness wide.
type KetQuaCapMatKhau struct {
	CanBo domain.CanBoTomTat

	// MatKhauTam is the temporary password, in plaintext, for exactly one HTTP response.
	MatKhauTam string
}

// Cap turns a directory row into an account carrying a temporary password (#9).
//
// THE PASSWORD IS MINTED AND HASHED BEFORE THE TRANSACTION OPENS, and that ordering is deliberate
// in both directions. argon2id costs 19 MiB and tens of milliseconds; doing it inside would hold a
// row lock for that long on a table every sign-in reads. And nothing is risked by doing it outside,
// because the hash is an ARGUMENT to the write: a minting failure means no transaction ever starts,
// and a write failure means the minted value was returned to nobody.
//
// #14 IS APPLIED ALTHOUGH IT IS UNREACHABLE HERE: the caller is signed in, so the caller has an
// account, so the caller is never a row this route accepts — ErrDaCoTaiKhoan would refuse them one
// line later. The check stays because a rule with no branch is a rule that cannot be half-removed,
// and because the reason it is unreachable is a property of another route's predicate rather than
// of this one.
//
// #13 IS NOT CHECKED, AND ITS ABSENCE IS REASONED RATHER THAN FORGOTTEN: the refusal is about the
// set of people who can administer the commune becoming EMPTY, and this operation can only make
// that set bigger — `dieuKienGiuQuyen` requires `co_tai_khoan`, which this write turns on and never
// off. A rule that fires on an operation which cannot cause the harm is a rule people learn to
// route around.
func (uc *TaiKhoanCanBo) Cap(ctx context.Context, id string, nguoi NguoiThucHien) (KetQuaCapMatKhau, error) {
	return uc.mint(ctx, id, nguoi, true)
}

// DatLai is the administrator resetting somebody else's password (#17).
//
// #14 STOPS SOMETHING REAL HERE, unlike on Cap. Without it, an administrator could reset their OWN
// password through this route — and this route does not ask for the current one, because its whole
// purpose is to help somebody who cannot supply it. Combined with #18's shared counter machine,
// that is a complete account takeover from an unattended browser: no credential needed, no trail
// that distinguishes it from ordinary administration. An administrator changing their own password
// uses DoiCuaChinhMinh, which demands the current one.
//
// THE CONSEQUENCE, STATED BECAUSE A COMMUNE WILL MEET IT: an administrator who has forgotten their
// own password needs ANOTHER administrator to reset it. That is the same shape #13 already forces
// on this commune — there is always more than one — and #17 chose the administrator path precisely
// because the self-service alternative depends on a mail server that may not exist.
func (uc *TaiKhoanCanBo) DatLai(ctx context.Context, id string, nguoi NguoiThucHien) (KetQuaCapMatKhau, error) {
	return uc.mint(ctx, id, nguoi, false)
}

// mint is both administrator paths. ONE FUNCTION because they differ in exactly one predicate —
// whether the row must already be an account — and two copies would be two places for the #14
// refusal, the session revocation or the audit entry to be edited out of one at a time.
//
// The BOOLEAN IS NOT A MODE FLAG ON A PUBLIC SURFACE: the two exported methods above are what
// callers see, each with its own name, its own audit verb and its own reasoning. This is the shared
// body, unexported.
func (uc *TaiKhoanCanBo) mint(ctx context.Context, id string, nguoi NguoiThucHien,
	capMoi bool) (KetQuaCapMatKhau, error) {

	if err := nguoi.hopLe(); err != nil {
		return KetQuaCapMatKhau{}, err
	}
	if id == "" {
		return KetQuaCapMatKhau{}, idstore.ErrCanBoKhongTonTai
	}
	// #14, checked before anything is read — same placement as DatKhoa and DoiVaiTro.
	if id == nguoi.ID {
		return KetQuaCapMatKhau{}, ErrTuThaoTacChinhMinh
	}

	matKhau, err := uc.sinhMatKhau()
	if err != nil {
		return KetQuaCapMatKhau{}, fmt.Errorf("tai_khoan_can_bo: sinh mật khẩu tạm: %w", err)
	}
	bam, err := uc.bam(matKhau)
	if err != nil {
		// The error from password.Bam never carries the value — it names the rule. Wrapping it is
		// safe; printing the input would not be.
		return KetQuaCapMatKhau{}, fmt.Errorf("tai_khoan_can_bo: băm mật khẩu tạm: %w", err)
	}

	var sau domain.CanBoTomTat
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}

		switch {
		case capMoi && truoc.CoTaiKhoan:
			return ErrDaCoTaiKhoan
		case !capMoi && !truoc.CoTaiKhoan:
			return ErrChuaCoTaiKhoan
		case !truoc.DangHoatDong:
			// Both directions. Resetting the password of a locked account is as pointless as
			// issuing one: the sign-in read filters `dang_hoat_dong`, so the value the
			// administrator reads out would not work, and they would have no way of telling why.
			return ErrTaiKhoanDangKhoa
		}

		if capMoi {
			err = uc.kho.CapTaiKhoan(ctx, tx, truoc.ID, bam)
		} else {
			err = uc.kho.DatMatKhauTam(ctx, tx, truoc.ID, bam)
		}
		if err != nil {
			return err
		}

		// REQUIRED #7 of skills/session-and-token, IN THE SAME TRANSACTION as the credential write.
		// On a reset it takes back the sessions opened with the password being replaced — which is
		// the point of resetting it. On an issue it is normally a no-op, because a row with
		// `co_tai_khoan = false` could never have signed in; it is called anyway rather than
		// branched around, because the branch would be the thing to get wrong the day a withdrawal
		// path exists, and a no-op UPDATE costs one indexed scan.
		lyDo := lyDoThuHoiCapTaiKhoan
		if !capMoi {
			lyDo = lyDoThuHoiDatLai
		}
		if err := uc.phien.ThuHoiCuaCanBo(ctx, tx, truoc.ID, lyDo); err != nil {
			return err
		}

		sau = truoc
		sau.CoTaiKhoan = true

		hanhVi := HanhViCapTaiKhoan
		if !capMoi {
			hanhVi = HanhViDatLaiMatKhau
		}
		// SAME TRANSACTION AS THE WRITE (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context.
		//
		// `phai_doi_mat_khau` APPEARS ONLY ON THE `sau` SIDE, AND THE ASYMMETRY IS HONESTY RATHER
		// THAN AN OVERSIGHT. domain.CanBoTomTat does not carry that column — cotTomTat does not
		// select it, deliberately, because that is the read which also avoids the credential — so
		// these two operations never learned what it was before. Writing a before-value here would
		// be inventing one, in a record that is never deleted.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  hanhVi,
			Subject: truoc.Ma, // the business code of the person acted on, never the internal id
			Delta: deltaMatKhau(
				map[string]any{"co_tai_khoan": truoc.CoTaiKhoan},
				map[string]any{"co_tai_khoan": true, "phai_doi_mat_khau": true}),
		})
	})
	if err != nil {
		return KetQuaCapMatKhau{}, err
	}
	return KetQuaCapMatKhau{CanBo: sau, MatKhauTam: matKhau}, nil
}

// YeuCauDoiMatKhau is one person replacing their own password.
//
// THE CURRENT PASSWORD IS MANDATORY AND HAS NO "SKIP IF phai_doi_mat_khau" BRANCH. It is tempting
// to drop it for somebody who is on the forced-change screen — they have just signed in, after all
// — and it would undo the one thing that makes the temporary password safe to read down a telephone
// line: that it has to be TYPED to be used. Knowing the session is not knowing the password, and on
// #18's shared counter machine those are routinely two different people.
type YeuCauDoiMatKhau struct {
	HienTai string
	Moi     string
}

// DoiCuaChinhMinh is the person setting their own password — the ONE act that clears
// `phai_doi_mat_khau` (#9), and the only route by which it is ever cleared.
//
// #14 DOES NOT APPLY AND MUST NOT BE COPIED HERE. That rule is about somebody granting themselves
// authority through an administrator route; this route grants nothing, takes nothing, and is the
// only way anybody ever stops carrying a password a second person chose. Refusing a self-target
// would leave the flag set forever.
//
// IT REVOKES EVERY SESSION, INCLUDING THE ONE MAKING THE REQUEST, and the cost is stated rather
// than hidden: the person is signed out and signs in again with the new password. That is
// skills/session-and-token REQUIRED #7 read literally, and the literal reading is the right one
// here — a password is usually changed BECAUSE somebody may have it, and the session opened with
// the old one is exactly what an attacker would be sitting in. Keeping the current session alive
// would mean the one session that survives a password change is the one nobody re-authenticated.
func (uc *TaiKhoanCanBo) DoiCuaChinhMinh(ctx context.Context, yc YeuCauDoiMatKhau,
	nguoi NguoiThucHien) error {

	if err := nguoi.hopLe(); err != nil {
		return err
	}
	// Shape first, outside the transaction: a request that fails its shape must not hold a row lock
	// while doing so, and the caller needs the reason rather than a rollback.
	if yc.HienTai == "" {
		return ErrMatKhauHienTaiSai
	}
	if err := domain.KiemTraMatKhauMoi(yc.Moi); err != nil {
		return err
	}
	if yc.Moi == yc.HienTai {
		// Refused BEFORE any hashing, so the comparison never touches a credential store. A
		// "change" that changes nothing would clear `phai_doi_mat_khau` while the administrator's
		// value stays valid — #9 defeated in one request, with a green trail saying the person
		// changed their password.
		return domain.ErrMatKhauMoiTrungCu
	}

	bam, err := uc.bam(yc.Moi)
	if err != nil {
		return fmt.Errorf("tai_khoan_can_bo: băm mật khẩu mới: %w", err)
	}

	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeGhiMatKhau(ctx, tx, nguoi.ID)
		if err != nil {
			return err
		}
		if !truoc.CoTaiKhoan || !truoc.DangHoatDong {
			// Unreachable through the edge — the middleware builds no principal for such a row —
			// so this is the second half of that check rather than a duplicate of it. Fail closed:
			// the person is refused, not handed a credential write on an account that is gone.
			return idstore.ErrCanBoKhongTonTai
		}

		// VERIFIED INSIDE THE TRANSACTION, AGAINST THE ROW BEING REPLACED. Read outside, the hash
		// compared against could be one another administrator replaced a millisecond ago, and the
		// person would overwrite a reset that was made for a reason.
		if err := uc.kiemTraBam(yc.HienTai, truoc.MatKhauHash); err != nil {
			// The reason is deliberately flattened: a parse failure of the stored hash and a wrong
			// password answer identically. Telling them apart tells whoever is guessing whether
			// this account's credential is even readable.
			return ErrMatKhauHienTaiSai
		}

		if err := uc.kho.DoiMatKhauChinhMinh(ctx, tx, truoc.ID, bam); err != nil {
			return err
		}
		if err := uc.phien.ThuHoiCuaCanBo(ctx, tx, truoc.ID, lyDoThuHoiDoiMatKhau); err != nil {
			return err
		}

		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViDoiMatKhau,
			Subject: truoc.Ma, // the person acted on is the person acting; both are the staff code
			Delta: deltaMatKhau(
				map[string]any{"phai_doi_mat_khau": truoc.PhaiDoiMatKhau},
				map[string]any{"phai_doi_mat_khau": false}),
		})
	})
}

// deltaMatKhau is the audit delta for every credential write in this file.
//
// IT RECORDS THE NAME OF THE COLUMN THAT CHANGED AND NEVER ITS VALUE — not the password, not the
// argon2 encoding, not a prefix of either, not a fingerprint. Rule 6, invariant 5 asks for before
// and after with personal data masked; a credential is the case where even a masked value is wrong,
// because the audit trail is append-only, kept for years, and outside every rotation the password
// itself is subject to. `"mat_khau_hash"` appearing in `cot_da_dat` says everything an inspection
// needs: THIS field was set, by THIS person, at THIS time, on THIS record.
//
// THE TWO BOOLEANS IT DOES CARRY ARE THE FACTS WORTH KEEPING. `co_tai_khoan` false → true is the
// moment somebody could first act in a government system at all. `phai_doi_mat_khau` true → false is
// the moment they stopped carrying a password a second person knew — the window between those two
// entries is what an inspection asks about, and it is answerable from the ledger alone.
//
// IT TAKES THE TWO SIDES AS MAPS RATHER THAN AS A FIXED SET OF BOOLEANS, because the three call
// sites genuinely know different things: only one of them ever reads the previous value of
// `phai_doi_mat_khau`. A fixed signature would force the other two to pass something for it, and
// the only values available would be a guess.
func deltaMatKhau(truoc, sau map[string]any) json.RawMessage {
	return deltaCanBo(map[string]any{
		"truoc": truoc,
		"sau":   sau,
		// The credential column is NAMED, never valued. See above.
		"cot_da_dat": []string{"mat_khau_hash"},
	})
}
