package app

// The use cases behind the WRITE surface of the commune's staff register.
//
// WHY THIS LAYER EXISTS FOR WHAT LOOKS LIKE FIVE SMALL STATEMENTS: rule 6, invariant 3 requires
// the audit entry to share a transaction with the business write, and core/audit.Write takes a
// *store.ScopedTx with no overload that writes outside one. Opening that transaction is this
// layer's job. The handler translates HTTP and nothing else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THAT EVERY WRITE HERE IS READ-DECIDE-WRITE, and three of the decisions are
// customer decisions taken on 2026-09-22 that no other layer can make:
//
//	#10  RETIRING SOMEBODY IS A LOCK, NOT A DELETE. Two situations, two operations, two
//	     permissions. A locked person stays in the directory, because their name is what makes
//	     years of administrative records readable.
//	#13  THE SYSTEM REFUSES any operation that would leave the commune with no administrator —
//	     locking, deleting or moving the last one to a weaker role. Not a warning: a refusal.
//	     ADR 0003 leaves the vendor no way back in, so this is a PROCEDURAL DEAD END rather than an
//	     incident with a hot fix.
//	#14  NOBODY ACTS ON THEIR OWN ACCOUNT, AND NOBODY HANDS OUT A PERMISSION THEY DO NOT HOLD.
//	     TWO constraints, not one: with only the first, the two-step route is still open — put
//	     somebody else into a strong role, then ask them to promote you.
//
// WHAT THIS FILE DELIBERATELY DOES NOT DO, and each absence is a decision rather than an omission:
//
//	REVOKING AN ACCOUNT      `admin.user.revoke` (ADR 0035 #27) is not seeded and not built. Xoa
//	                         below therefore REFUSES a row that has an account rather than revoking
//	                         it on the way: the soft delete of #10 is for a duplicated directory
//	                         row, under its own key `admin.user.delete` (migration 0010 §4).
//	ISSUING AN ACCOUNT      #9/#17/#18 — a separate flow (`cap-tai-khoan-can-bo`). Nothing here
//	                         writes `co_tai_khoan` or `mat_khau_hash`; the store has no parameter
//	                         for either.
//	A PUBLIC DIRECTORY READ  DatCongKhai below WRITES the Mini App publication (#12, migration
//	                         0010). Nothing in this service READS it for a citizen yet: a public
//	                         Mini App route is rule 4 stop condition #2 and has not been decided.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// KhoDanhBaCanBo is the store, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the row in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that the guards run before the write — provable without a PostgreSQL.
//
// WHAT CANNOT BE PROVEN WITHOUT ONE, and is therefore proven in store/can_bo_ghi_pg_test.go
// instead: that QuanTriDangHoatDong's `FOR UPDATE` actually serialises two administrators locking
// each other. A fake store agrees with whatever the test says, and a race is precisely the thing
// it cannot have an opinion about.
type KhoDanhBaCanBo interface {
	TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CanBoTomTat, error)
	Chen(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error
	CapNhatHoSo(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error
	DatKhoa(ctx context.Context, tx *store.ScopedTx, id string, dangHoatDong bool) error
	DatVaiTro(ctx context.Context, tx *store.ScopedTx, id, vaiTroID string) error
	DatCongKhai(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, xoaBoi, lyDo string, luc time.Time) error
	QuanTriDangHoatDong(ctx context.Context, tx *store.ScopedTx) ([]string, error)
	QuyenCuaVaiTro(ctx context.Context, tx *store.ScopedTx, vaiTroID string) ([]string, bool, error)
	QuyenDangGiu(ctx context.Context, tx *store.ScopedTx, canBoID string) ([]string, error)
}

// NguoiThucHien is the person making the change, carrying BOTH of their identifiers.
//
// TWO FIELDS BECAUSE THEY ANSWER TWO QUESTIONS AND NEITHER CAN ANSWER THE OTHER'S — the same split
// authz.Principal makes, and the reason it is repeated here rather than passing the whole
// principal is that a use case that could see `Roles` would eventually decide something from them,
// which is the authorisation model leaking out of core/authz.
//
//	ID   DECIDES.   `nguoi_dung.id`. Compared with the target (#14, first constraint) and used to
//	                read what the actor holds (#14, second constraint). NEVER written to the trail.
//	Vet  RECORDS.   audit.Actor, whose ID is the STAFF CODE `ma` (rule 6, invariant 8). A ULID on
//	                an archival record names nobody to the person reading it years later.
type NguoiThucHien struct {
	ID  string
	Vet audit.Actor
}

// hopLe refuses a half-built actor before anything is written. core/audit.Entry.validate would
// refuse the empty code too, but only AFTER the business write has already run inside the
// transaction — the rollback is correct and the error is unreadable.
func (n NguoiThucHien) hopLe() error {
	switch {
	case n.ID == "":
		return errors.New("danh_ba_can_bo: thiếu định danh người thực hiện")
	case n.Vet.ID == "":
		// NEVER FALL BACK TO n.ID. Empty happens only when this service is talking to an identity
		// older than the `ma` field; a substitute here puts internal ids back into
		// `audit_log.actor_id` silently, with every test still green.
		return errors.New("danh_ba_can_bo: thiếu mã cán bộ của người thực hiện")
	}
	return nil
}

// The refusals this layer owns. The handler maps them to status codes; nothing here knows HTTP.
var (
	// ErrTuThaoTacChinhMinh — #14, first constraint.
	ErrTuThaoTacChinhMinh = errors.New("danh_ba_can_bo: không tự thao tác lên chính mình")

	// ErrQuanTriCuoiCung — #13. The refusal is unconditional: there is no override flag, no
	// "confirm anyway" parameter and no vendor path, because ADR 0003 gives the vendor no
	// authority over a commune's business data. Adding one is a new ADR, not a new field.
	ErrQuanTriCuoiCung = errors.New("danh_ba_can_bo: thao tác này sẽ làm xã không còn người quản trị")

	// ErrTraoQuyenKhongCam — #14, second constraint. See LoiTraoQuyenKhongCam for the keys.
	ErrTraoQuyenKhongCam = errors.New("danh_ba_can_bo: không trao được quyền mình không cầm")

	// ErrVaiTroKhongTonTai is a role that is absent OR soft-deleted. The foreign key would accept
	// the second one — rule 7 keeps the row — and the person would then hold a role that grants
	// nothing while every screen shows them as having one.
	ErrVaiTroKhongTonTai = errors.New("danh_ba_can_bo: vai trò không tồn tại")

	// ErrKhongSinhDuocMa is the retry loop giving up. See Them.
	ErrKhongSinhDuocMa = errors.New("danh_ba_can_bo: không sinh được mã cán bộ chưa dùng")

	// ErrChuaXacNhanDongY — open question #12. Publishing a personal mobile to the public Mini App
	// needs the person's own consent, and the lean form decided on 2026-09-24 is that the
	// administrator CONFIRMS, per request, that they asked and the person agreed. No confirmation,
	// no publication — and nothing is written.
	ErrChuaXacNhanDongY = errors.New("danh_ba_can_bo: công khai lên Mini App cần xác nhận đã được người này đồng ý")

	// ErrCanBoCoTaiKhoan — the soft delete of #10 is for a DUPLICATED DIRECTORY ROW, and a row
	// carrying a sign-in account is not that (user decision 2026-09-24). Revoking the account is
	// its own act under its own key (`admin.user.revoke`, ADR 0035 — not built); retiring somebody
	// is the lock. Deleting the row here would leave a credential attached to a record every screen
	// has stopped showing.
	ErrCanBoCoTaiKhoan = errors.New("danh_ba_can_bo: dòng danh bạ này đang có tài khoản đăng nhập, không xoá được")
)

// LoiTraoQuyenKhongCam names the keys that were refused.
//
// THE LIST IS THE POINT. "You may not assign this role" sends an administrator to guess; "this
// role carries budget.confirm and document.route, which you do not hold" tells them exactly what
// to ask their own superior for. Permission keys are not personal data — they are the same strings
// the Phân quyền screen prints.
type LoiTraoQuyenKhongCam struct{ Thieu []string }

func (e *LoiTraoQuyenKhongCam) Error() string {
	return ErrTraoQuyenKhongCam.Error() + ": " + strings.Join(e.Thieu, ", ")
}

// Is lets every caller write errors.Is(err, ErrTraoQuyenKhongCam) without knowing this type
// exists. Only the handler that wants to print the list reaches for the type itself.
func (e *LoiTraoQuyenKhongCam) Is(target error) bool { return target == ErrTraoQuyenKhongCam }

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `them_loai_van_ban`): an inspection reads these strings, and
// a function name would tell them nothing.
//
// FOUR VERBS AND NOT ONE `sua_can_bo`, because the four are four different acts with four
// different consequences. "Somebody's telephone number was corrected" and "somebody was moved into
// the role that runs the commune" must not be one string in a ledger that is never deleted —
// anyone auditing would have to open and interpret every delta to tell them apart.
const (
	HanhViThemCanBo   = "them_can_bo"
	HanhViSuaCanBo    = "sua_ho_so_can_bo"
	HanhViKhoaCanBo   = "khoa_tai_khoan_can_bo"
	HanhViMoKhoaCanBo = "mo_khoa_tai_khoan_can_bo"
	HanhViDoiVaiTro   = "doi_vai_tro_can_bo"

	// THREE MORE FOR THE MINI APP PUBLICATION (#12), for the same reason: "this person's mobile was
	// put on a public channel", "it was taken off" and "their position in the list moved" are three
	// different facts, and the first is the one an inspection or a complaint will ask about.
	HanhViCongKhaiMiniApp    = "cong_khai_mini_app"
	HanhViRutCongKhaiMiniApp = "rut_cong_khai_mini_app"
	HanhViDoiThuTuDanhBa     = "doi_thu_tu_danh_ba"

	// The soft delete of #10. The verb names the ONLY situation it exists for, so an inspection
	// reading the ledger cannot mistake it for a retirement (which is `khoa_tai_khoan_can_bo`).
	HanhViXoaCanBoNhapTrung = "xoa_can_bo_nhap_trung"
)

// soLanThuMa is how many codes are minted before giving up.
//
// WHY A LOOP AT ALL: `UNIQUE (tenant_id, ma)` is NOT partial, so a soft-deleted person keeps their
// code for good (rule 7, invariant 3), and the generator deliberately knows nothing about the
// table. A collision is therefore possible — vanishingly unlikely at 30 bits against a few tens of
// staff, but possible — and the answer is to mint another and insert again. It is NEVER to look
// for a free code: a SELECT followed by an INSERT is two statements with a gap in between, and two
// staff members created in that gap receive the same code.
//
// THREE AND NOT "until it works": a loop with no ceiling turns a table that has genuinely run out
// of codes, or a constraint violation this code misread, into a request that never returns.
const soLanThuMa = 3

// DanhBaCanBo owns adding, editing, locking, reassigning, publishing and — for a duplicated row
// only — soft-deleting one commune's staff directory entries.
type DanhBaCanBo struct {
	db  *store.DB
	kho KhoDanhBaCanBo

	// Injected so a test can pin them. In production: ulid.Moi, domain.SinhMaCanBo, time.Now.
	sinhID func() (string, error)
	sinhMa func(time.Time) (string, error)
	bayGio func() time.Time
}

func NewDanhBaCanBo(db *store.DB, kho KhoDanhBaCanBo) *DanhBaCanBo {
	return &DanhBaCanBo{
		db: db, kho: kho,
		sinhID: ulid.Moi,
		sinhMa: domain.SinhMaCanBo,
		bayGio: func() time.Time { return time.Now().UTC() },
	}
}

// YeuCauThemCanBo is one new directory entry as it arrives from the handler.
//
// THERE IS NO `Ma` FIELD AND THERE MUST NEVER BE ONE — open question #15: the system mints the
// code and neither form in the specification draws a box for it. A field here is a field a client
// could fill, and a client-chosen code is a code somebody can point at an existing person's
// records.
//
// THERE IS NO `VaiTroID` FIELD EITHER, and that is the larger decision: assigning a role is the
// operation carrying #14's two guards, so a role on the create request would be the way around
// both of them. A new person holds nothing until somebody with the authority grants it.
//
// THERE IS NO ACCOUNT, NO PASSWORD AND NO `CoTaiKhoan` — #9/#17/#18, a different flow.
type YeuCauThemCanBo struct {
	HoTen           string
	ChucVu          string
	Email           string
	BoPhanID        string
	DienThoaiCoQuan string
	DiDongCaNhan    string
}

// Them adds one directory entry and mints its staff code.
func (uc *DanhBaCanBo) Them(ctx context.Context, yc YeuCauThemCanBo,
	nguoi NguoiThucHien) (domain.CanBoTomTat, error) {

	if err := nguoi.hopLe(); err != nil {
		return domain.CanBoTomTat{}, err
	}

	// Validated BEFORE the transaction opens. A request that fails its shape must never hold a row
	// lock while doing so, and the caller needs the reason rather than a rollback.
	moi, err := uc.chuanHoa(yc)
	if err != nil {
		return domain.CanBoTomTat{}, err
	}
	if moi.ID, err = uc.sinhID(); err != nil {
		return domain.CanBoTomTat{}, fmt.Errorf("danh_ba_can_bo: sinh id: %w", err)
	}

	for lan := 0; lan < soLanThuMa; lan++ {
		if moi.Ma, err = uc.sinhMa(uc.bayGio()); err != nil {
			return domain.CanBoTomTat{}, fmt.Errorf("danh_ba_can_bo: sinh mã: %w", err)
		}

		err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
			if err := uc.kho.Chen(ctx, tx, moi); err != nil {
				return err
			}
			// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID is left unset on
			// purpose: audit.Write fills it from the transaction, which took it from the context
			// (rule 1, invariant 4). Passing it here would be a second source for the one fact
			// that decides which commune the entry belongs to.
			return audit.Write(ctx, tx, audit.Entry{
				Actor:   nguoi.Vet,
				Action:  HanhViThemCanBo,
				Subject: moi.Ma, // the business code of the person created, never the internal id
				Delta:   deltaCanBo(map[string]any{"sau": tomTatCanBo(moi)}),
			})
		})
		// A COLLISION IS RETRIED WITH A NEW CODE — never by hunting for a free one. Nothing was
		// committed: no row, no trail, and the code that lost stays unused rather than being
		// consumed by a row that does not exist.
		if errors.Is(err, idstore.ErrMaCanBoDaDung) {
			continue
		}
		if err != nil {
			return domain.CanBoTomTat{}, err
		}
		return moi, nil
	}
	return domain.CanBoTomTat{}, fmt.Errorf("%w sau %d lần", ErrKhongSinhDuocMa, soLanThuMa)
}

func (uc *DanhBaCanBo) chuanHoa(yc YeuCauThemCanBo) (domain.CanBoTomTat, error) {
	var cb domain.CanBoTomTat
	var err error
	if cb.HoTen, err = domain.ChuanHoaHoTen(yc.HoTen); err != nil {
		return cb, err
	}
	if cb.ChucVu, err = domain.ChuanHoaChucVu(yc.ChucVu); err != nil {
		return cb, err
	}
	if cb.Email, err = domain.ChuanHoaEmail(yc.Email); err != nil {
		return cb, err
	}
	if err = domain.KiemTraIDThamChieu(yc.BoPhanID); err != nil {
		return cb, err
	}
	cb.BoPhanID = yc.BoPhanID
	if cb.DienThoaiCoQuan, err = domain.ChuanHoaSoDienThoai(yc.DienThoaiCoQuan); err != nil {
		return cb, err
	}
	if cb.DiDongCaNhan, err = domain.ChuanHoaSoDienThoai(yc.DiDongCaNhan); err != nil {
		return cb, err
	}
	return cb, nil
}

// YeuCauSuaCanBo is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. Four of the six fields have a meaningful empty value —
// clearing a position, a department, or either telephone number is a legitimate edit. A struct of
// plain strings cannot tell "the client did not mention this" from "the client cleared it", so a
// screen that edits only the position would silently wipe both telephone numbers off a government
// directory, and nothing would report it.
type YeuCauSuaCanBo struct {
	HoTen           *string
	ChucVu          *string
	Email           *string
	BoPhanID        *string
	DienThoaiCoQuan *string
	DiDongCaNhan    *string

	// CoZalo — "Có Zalo" (migration 0010 §1). A pointer for the same reason as the rest: false is
	// a meaningful value, so only nil can mean "not mentioned".
	CoZalo *bool
}

// Sua applies a partial edit to one person's profile.
//
// IT CHANGES NO AUTHORITY, AND THAT IS WHY IT CARRIES NONE OF THE THREE GUARDS. The role, the lock
// and the account are not reachable from here — the UPDATE statement does not name those columns —
// so nothing this route can do makes anybody more powerful, removes the commune's last
// administrator, or promotes the caller. #14 asks whether somebody may act on their own account;
// what it is about is privilege, and correcting one's own telephone number is not privilege.
// Refusing it would also be the only rule in this file that a person hits while doing something
// harmless, which is how a rule stops being believed.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending the values a row already has is not an event;
// recording it would fill a public authority's ledger with entries saying nothing changed, and
// those are the entries that bury the ones carrying legal weight. It is also what makes this route
// genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
func (uc *DanhBaCanBo) Sua(ctx context.Context, id string, yc YeuCauSuaCanBo,
	nguoi NguoiThucHien) (domain.CanBoTomTat, error) {

	if err := nguoi.hopLe(); err != nil {
		return domain.CanBoTomTat{}, err
	}
	if id == "" {
		return domain.CanBoTomTat{}, idstore.ErrCanBoKhongTonTai
	}

	// Shape first, outside the transaction, for the same reason as Them.
	dat, err := chuanHoaSua(yc)
	if err != nil {
		return domain.CanBoTomTat{}, err
	}

	var sau domain.CanBoTomTat
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}

		sau = truoc
		dat(&sau)

		if !doiHoSo(truoc, sau) {
			return nil
		}
		if err := uc.kho.CapNhatHoSo(ctx, tx, sau); err != nil {
			return err
		}

		// BEFORE AND AFTER, ONLY THE FIELDS THAT MOVED, PERSONAL DATA ALREADY MASKED (rule 6,
		// invariant 5 and forbidden #4). See tomTatDoiHoSo.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViSuaCanBo,
			Subject: truoc.Ma,
			Delta: deltaCanBo(map[string]any{
				"truoc": tomTatDoiHoSo(truoc, sau, true),
				"sau":   tomTatDoiHoSo(truoc, sau, false),
			}),
		})
	})
	if err != nil {
		return domain.CanBoTomTat{}, err
	}
	return sau, nil
}

// chuanHoaSua validates every field the request mentions and returns the assignment to apply.
// Returning a closure keeps the normalised values and the decision to apply them in one place —
// the alternative is six nullable locals carried into the transaction.
func chuanHoaSua(yc YeuCauSuaCanBo) (func(*domain.CanBoTomTat), error) {
	var dat []func(*domain.CanBoTomTat)

	if yc.HoTen != nil {
		v, err := domain.ChuanHoaHoTen(*yc.HoTen)
		if err != nil {
			return nil, err
		}
		dat = append(dat, func(cb *domain.CanBoTomTat) { cb.HoTen = v })
	}
	if yc.ChucVu != nil {
		v, err := domain.ChuanHoaChucVu(*yc.ChucVu)
		if err != nil {
			return nil, err
		}
		dat = append(dat, func(cb *domain.CanBoTomTat) { cb.ChucVu = v })
	}
	if yc.Email != nil {
		v, err := domain.ChuanHoaEmail(*yc.Email)
		if err != nil {
			return nil, err
		}
		dat = append(dat, func(cb *domain.CanBoTomTat) { cb.Email = v })
	}
	if yc.BoPhanID != nil {
		v := *yc.BoPhanID
		if err := domain.KiemTraIDThamChieu(v); err != nil {
			return nil, err
		}
		dat = append(dat, func(cb *domain.CanBoTomTat) { cb.BoPhanID = v })
	}
	if yc.DienThoaiCoQuan != nil {
		v, err := domain.ChuanHoaSoDienThoai(*yc.DienThoaiCoQuan)
		if err != nil {
			return nil, err
		}
		dat = append(dat, func(cb *domain.CanBoTomTat) { cb.DienThoaiCoQuan = v })
	}
	if yc.DiDongCaNhan != nil {
		v, err := domain.ChuanHoaSoDienThoai(*yc.DiDongCaNhan)
		if err != nil {
			return nil, err
		}
		dat = append(dat, func(cb *domain.CanBoTomTat) { cb.DiDongCaNhan = v })
	}
	if yc.CoZalo != nil {
		v := *yc.CoZalo
		dat = append(dat, func(cb *domain.CanBoTomTat) { cb.CoZalo = v })
	}

	return func(cb *domain.CanBoTomTat) {
		for _, f := range dat {
			f(cb)
		}
	}, nil
}

// DatKhoa locks or unlocks one account — #10's answer to "this person has retired".
//
// THE ORDER OF THE TWO LOCKS IS DELIBERATE AND MUST NOT BE SWAPPED: the administrator set is
// locked FIRST, in id order, and the target row after it. Reversed, two administrators locking
// each other take the same two rows in opposite orders and deadlock; PostgreSQL detects it and
// aborts one, which is safe but reaches a commune as an unexplained failure. Taking the set first
// means both transactions queue on the same first row instead.
//
// THE PRICE, STATED: an id that does not exist now runs the administrator query before it can
// answer 404. One extra indexed read on an operation a commune performs a few times a year.
func (uc *DanhBaCanBo) DatKhoa(ctx context.Context, id string, khoa bool,
	nguoi NguoiThucHien) (domain.CanBoTomTat, error) {

	if err := nguoi.hopLe(); err != nil {
		return domain.CanBoTomTat{}, err
	}
	if id == "" {
		return domain.CanBoTomTat{}, idstore.ErrCanBoKhongTonTai
	}
	// #14, FIRST CONSTRAINT — checked before anything is read. It applies to unlocking as well as
	// locking, although self-unlocking is unreachable in practice: a locked account cannot
	// authenticate, so its owner never gets as far as this route. One rule with no branch is one
	// rule that cannot be half-removed.
	if id == nguoi.ID {
		return domain.CanBoTomTat{}, ErrTuThaoTacChinhMinh
	}

	var sau domain.CanBoTomTat
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		quanTri, err := uc.kho.QuanTriDangHoatDong(ctx, tx)
		if err != nil {
			return err
		}
		truoc, err := uc.kho.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}

		if truoc.DangHoatDong == !khoa {
			// Already in the requested state. Nothing written, nothing audited — see Sua.
			sau = truoc
			return nil
		}
		// #13 — locking the last person who can administer this commune is REFUSED. The set was
		// read and LOCKED in this transaction, so "there is somebody else" cannot become false
		// between the check and the write.
		if khoa && laNguoiQuanTriCuoiCung(quanTri, truoc.ID) {
			return ErrQuanTriCuoiCung
		}

		if err := uc.kho.DatKhoa(ctx, tx, truoc.ID, !khoa); err != nil {
			return err
		}
		sau = truoc
		sau.DangHoatDong = !khoa

		hanhVi := HanhViMoKhoaCanBo
		if khoa {
			hanhVi = HanhViKhoaCanBo
		}
		// NO PERSONAL DATA IN THIS DELTA AT ALL: one boolean moved. The person is named by
		// Subject, which is their staff code.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  hanhVi,
			Subject: truoc.Ma,
			Delta: deltaCanBo(map[string]any{
				"truoc": map[string]any{"dang_hoat_dong": truoc.DangHoatDong},
				"sau":   map[string]any{"dang_hoat_dong": sau.DangHoatDong},
			}),
		})
	})
	if err != nil {
		return domain.CanBoTomTat{}, err
	}
	return sau, nil
}

// DoiVaiTro moves one person to a role, or to no role at all.
//
// THIS IS THE ROUTE ALL THREE DECISIONS MEET ON, which is why it is not a field on the profile
// edit:
//
//	#14 first    the caller may not move THEMSELVES. Without it, holding `admin.user` means
//	             holding every permission, since its holder can put themselves into the strongest
//	             role in the commune.
//	#14 second   the caller may not grant a key they do not hold. Without it, the first constraint
//	             is a formality: put a colleague into the strongest role, then ask them to promote
//	             you. It is also what makes the 35-key table describe what actually runs, instead
//	             of describing a flat model while one key sits above all the others (rule 5,
//	             forbidden #2).
//	#13          moving the last administrator to a role without `admin.user` is the third of the
//	             three paths to a commune with nobody who can administer it — and the one least
//	             likely to look dangerous to whoever is doing it.
func (uc *DanhBaCanBo) DoiVaiTro(ctx context.Context, id, vaiTroID string,
	nguoi NguoiThucHien) (domain.CanBoTomTat, error) {

	if err := nguoi.hopLe(); err != nil {
		return domain.CanBoTomTat{}, err
	}
	if id == "" {
		return domain.CanBoTomTat{}, idstore.ErrCanBoKhongTonTai
	}
	if err := domain.KiemTraIDThamChieu(vaiTroID); err != nil {
		return domain.CanBoTomTat{}, err
	}
	if id == nguoi.ID {
		return domain.CanBoTomTat{}, ErrTuThaoTacChinhMinh
	}

	var sau domain.CanBoTomTat
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Same lock order as DatKhoa: the administrator set first, in id order, then the target.
		quanTri, err := uc.kho.QuanTriDangHoatDong(ctx, tx)
		if err != nil {
			return err
		}
		truoc, err := uc.kho.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}
		if truoc.VaiTroID == vaiTroID {
			sau = truoc
			return nil
		}

		quyenMoi, coVaiTro, err := uc.kho.QuyenCuaVaiTro(ctx, tx, vaiTroID)
		if err != nil {
			return err
		}
		if !coVaiTro {
			return ErrVaiTroKhongTonTai
		}

		// #14, SECOND CONSTRAINT. Both halves are read INSIDE this transaction: a comparison whose
		// two sides come from two moments is a comparison of two different worlds, and the moment
		// that matters is the one the write lands in.
		quyenToi, err := uc.kho.QuyenDangGiu(ctx, tx, nguoi.ID)
		if err != nil {
			return err
		}
		if thieu := khongCam(quyenMoi, quyenToi); len(thieu) > 0 {
			return &LoiTraoQuyenKhongCam{Thieu: thieu}
		}

		// #13 — the role change must not take the commune's last administrator away. It is only a
		// loss when the NEW role does not carry the key: moving the last administrator between two
		// roles that both hold `admin.user` changes nothing about the commune's ability to
		// administer itself, and refusing it would be a rule that fires on a safe operation.
		if !coQuyen(quyenMoi, idstore.QuyenQuanTriNguoiDung) &&
			laNguoiQuanTriCuoiCung(quanTri, truoc.ID) {
			return ErrQuanTriCuoiCung
		}

		if err := uc.kho.DatVaiTro(ctx, tx, truoc.ID, vaiTroID); err != nil {
			return err
		}
		sau = truoc
		sau.VaiTroID = vaiTroID

		// ROLE IDS, NOT ROLE NAMES, AND NO PERSONAL DATA. A name would need a join, and the name a
		// role carries today is not the name it carried when the entry was written — the id is the
		// only value that still means the same thing years later.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViDoiVaiTro,
			Subject: truoc.Ma,
			Delta: deltaCanBo(map[string]any{
				"truoc": map[string]any{"vai_tro_id": truoc.VaiTroID},
				"sau":   map[string]any{"vai_tro_id": sau.VaiTroID},
			}),
		})
	})
	if err != nil {
		return domain.CanBoTomTat{}, err
	}
	return sau, nil
}

// YeuCauCongKhai is the WHOLE publication state of one person, as PUT carries it.
//
//	CongKhai        publish (true) or unpublish (false).
//	DaXacNhanDongY  the administrator confirms THIS request: "đã hỏi ý và người này đồng ý".
//	                Required when CongKhai is true; ignored when false.
//	ThuTu           explicit position in the directory; nil = no explicit order. PUT semantics:
//	                nil CLEARS a position that was set, it does not mean "unchanged".
type YeuCauCongKhai struct {
	CongKhai       bool
	DaXacNhanDongY bool
	ThuTu          *int
}

// DatCongKhai publishes one person to the public Zalo Mini App directory, or takes them off it,
// and sets their position in it — open question #12.
//
// WHAT #12 DECIDED, AND WHERE EACH PART IS ENFORCED:
//
//	PER PERSON, NEVER BULK       the signature takes one id. There is no list form.
//	RECORDED CONSENT             publishing without DaXacNhanDongY is refused BEFORE the
//	                             transaction opens. The marks written are WHEN (this service's
//	                             clock) and WHO (nguoi.Vet.ID — the STAFF CODE, rule 6 invariant 8;
//	                             an empty one is refused by hopLe, never replaced by the internal id).
//	UNPUBLISH CLEARS THE MARKS   in the SAME UPDATE as the flag (store.datCongKhaiCanBo). The next
//	                             publish must therefore confirm consent again.
//	HISTORY                      one audit entry per change, in the same transaction.
//
// PUBLISHING SOMEBODY ALREADY PUBLISHED KEEPS THE ORIGINAL CONSENT MARKS. Consent is still
// required on the request (one rule, no branch), but the evidence on the row is the record of the
// act that put the number on the public channel; overwriting it with every repeat would replace
// "who asked the person, and when" with "who last pressed the button". It is also what makes the
// route idempotent: the same PUT twice writes nothing the second time.
//
// NO #14 SELF-CHECK. #14 is about PRIVILEGE, and publishing a directory entry grants none. Whether a
// staff member may record their OWN consent is not decided anywhere; it is reported, not decided
// here.
func (uc *DanhBaCanBo) DatCongKhai(ctx context.Context, id string, yc YeuCauCongKhai,
	nguoi NguoiThucHien) (domain.CanBoTomTat, error) {

	if err := nguoi.hopLe(); err != nil {
		return domain.CanBoTomTat{}, err
	}
	if id == "" {
		return domain.CanBoTomTat{}, idstore.ErrCanBoKhongTonTai
	}
	if err := domain.KiemTraThuTuDanhBa(yc.ThuTu); err != nil {
		return domain.CanBoTomTat{}, err
	}
	// #12 — THE CONSENT GATE. Before the transaction, so a refusal holds no row lock and writes
	// nothing.
	if yc.CongKhai && !yc.DaXacNhanDongY {
		return domain.CanBoTomTat{}, ErrChuaXacNhanDongY
	}

	var sau domain.CanBoTomTat
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, err := uc.kho.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}

		sau = truoc
		sau.ThuTuDanhBa = saoChepSo(yc.ThuTu)
		switch {
		case yc.CongKhai && !truoc.HienTrenMiniApp:
			luc := uc.bayGio()
			sau.HienTrenMiniApp = true
			sau.DongYCongKhaiLuc = &luc
			sau.DongYCongKhaiGhiBoi = nguoi.Vet.ID
		case !yc.CongKhai:
			sau.HienTrenMiniApp = false
			sau.DongYCongKhaiLuc = nil
			sau.DongYCongKhaiGhiBoi = ""
		}

		hanhVi := hanhViCongKhai(truoc, sau)
		if hanhVi == "" {
			return nil // nothing moved: no UPDATE, no entry
		}
		if err := uc.kho.DatCongKhai(ctx, tx, sau); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  hanhVi,
			Subject: truoc.Ma,
			Delta: deltaCanBo(map[string]any{
				"truoc": tomTatCongKhai(truoc),
				"sau":   tomTatCongKhai(sau),
			}),
		})
	})
	if err != nil {
		return domain.CanBoTomTat{}, err
	}
	return sau, nil
}

// hanhViCongKhai names the change, or "" when there is none. A publication transition wins over an
// order change in the same request: one request, one entry, and the delta carries both.
func hanhViCongKhai(truoc, sau domain.CanBoTomTat) string {
	switch {
	case !truoc.HienTrenMiniApp && sau.HienTrenMiniApp:
		return HanhViCongKhaiMiniApp
	case truoc.HienTrenMiniApp && !sau.HienTrenMiniApp:
		return HanhViRutCongKhaiMiniApp
	case !cungSo(truoc.ThuTuDanhBa, sau.ThuTuDanhBa):
		return HanhViDoiThuTuDanhBa
	}
	return ""
}

// tomTatCongKhai is the audit view of one publication state.
//
// THE MOBILE IS NAMED, MASKED (rule 6, forbidden #4). "Which number went onto the public channel"
// is a question the entry must answer; the digits are not needed to answer it, and an audit log
// holding them would be a personal-data store outside every erasure rule. The recorder is a staff
// code, and the consent time is a time — neither is personal data of the person published.
func tomTatCongKhai(cb domain.CanBoTomTat) map[string]any {
	var luc any
	if cb.DongYCongKhaiLuc != nil {
		luc = cb.DongYCongKhaiLuc.UTC().Format(time.RFC3339)
	}
	var thuTu any
	if cb.ThuTuDanhBa != nil {
		thuTu = *cb.ThuTuDanhBa
	}
	return map[string]any{
		"hien_tren_mini_app":       cb.HienTrenMiniApp,
		"dong_y_cong_khai_luc":     luc,
		"dong_y_cong_khai_ghi_boi": cb.DongYCongKhaiGhiBoi,
		"thu_tu_danh_ba":           thuTu,
		"di_dong_ca_nhan":          privacy.MaskPhone(cb.DiDongCaNhan),
		"co_zalo":                  cb.CoZalo,
	}
}

func cungSo(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// saoChepSo copies the value so the returned record does not alias the caller's request.
func saoChepSo(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// Xoa soft-deletes one DUPLICATED directory row — open question #10's second situation.
//
// WHAT #10 AND THE USER'S 2026-09-24 DECISION FIXED, AND WHERE EACH PART IS ENFORCED:
//
//	ONLY A DUPLICATE          retiring or transferring somebody is the LOCK (DatKhoa), never this.
//	                          The route carries its own key, `admin.user.delete` (migration 0010 §4).
//	NO ACCOUNT ON THE ROW     a row with `co_tai_khoan = true` is refused with ErrCanBoCoTaiKhoan,
//	                          INSIDE the transaction on the row read FOR UPDATE — so an account
//	                          issued a second earlier cannot slip between the check and the write.
//	NOT ONESELF (#14)         refused before anything is read, like every other write here.
//	NEVER THE LAST ADMIN (#13) see below — defensive, and stated as such.
//	REASON REQUIRED           rule 7 invariant 1; shape-checked before the transaction opens.
//	UNPUBLISHED WITH IT       the store's UPDATE clears the Mini App flag and both consent marks
//	                          as literals (store.xoaMemCanBo), so a deleted row can never stay public.
//	TRAIL                     one entry, same transaction, actor = the staff code.
//
// #13 IS NATURALLY SATISFIED AND STILL CHECKED. An administrator is somebody who can EXERCISE
// `admin.user`, and `dieuKienGiuQuyen` requires `co_tai_khoan` for that — so the account refusal
// above already excludes every administrator. The check is kept because that reasoning spans two
// files and one shared SQL fragment: the day the predicate changes (an account-less holder, a new
// way to administer), this line is what still stands between a delete and a commune nobody can
// administer, and ADR 0003 leaves the vendor no way back in. It costs the same locked read DatKhoa
// already takes, in the same order (administrator set first, then the target), so the two
// operations cannot deadlock each other.
//
// ALREADY DELETED, ANOTHER COMMUNE'S ID, OR AN INVENTED ONE: ErrCanBoKhongTonTai, one answer for
// all three (rule 4, forbidden #2). TheoIDDeGhi is scoped and excludes deleted rows, and that is
// also what makes a second DELETE unable to overwrite who removed the row, or why.
func (uc *DanhBaCanBo) Xoa(ctx context.Context, id, lyDoTho string, nguoi NguoiThucHien) error {
	if err := nguoi.hopLe(); err != nil {
		return err
	}
	if id == "" {
		return idstore.ErrCanBoKhongTonTai
	}
	lyDo, err := domain.ChuanHoaLyDoXoa(lyDoTho)
	if err != nil {
		return err
	}
	if id == nguoi.ID {
		return ErrTuThaoTacChinhMinh
	}

	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		quanTri, err := uc.kho.QuanTriDangHoatDong(ctx, tx)
		if err != nil {
			return err
		}
		truoc, err := uc.kho.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}
		if truoc.CoTaiKhoan {
			return ErrCanBoCoTaiKhoan
		}
		if laNguoiQuanTriCuoiCung(quanTri, truoc.ID) {
			return ErrQuanTriCuoiCung
		}

		luc := uc.bayGio()
		// `deleted_by` IS THE STAFF CODE (rule 6, invariant 8) — nguoi.Vet.ID, which hopLe has
		// already refused when empty. NEVER nguoi.ID.
		if err := uc.kho.XoaMem(ctx, tx, truoc.ID, nguoi.Vet.ID, lyDo, luc); err != nil {
			return err
		}

		sau := truoc
		sau.HienTrenMiniApp = false
		sau.DongYCongKhaiLuc = nil
		sau.DongYCongKhaiGhiBoi = ""

		// THE REASON TEXT IS NOT IN THE DELTA, ITS LENGTH IS — the convention service-petitions'
		// task delete set. It is free text about a record; `audit_log` is append-only and never
		// deleted, so a copy there is a second permanent store of it. The reason itself lives in
		// `delete_reason` on the row, which is kept for ever too.
		truocVet := tomTatCanBo(truoc)
		for k, v := range tomTatCongKhai(truoc) {
			truocVet[k] = v
		}
		truocVet["da_xoa"] = false
		truocVet["co_tai_khoan"] = truoc.CoTaiKhoan
		sauVet := tomTatCongKhai(sau)
		sauVet["da_xoa"] = true

		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViXoaCanBoNhapTrung,
			Subject: truoc.Ma,
			Delta: deltaCanBo(map[string]any{
				"truoc":        truocVet,
				"sau":          sauVet,
				"do_dai_ly_do": len([]rune(lyDo)),
				"xoa_luc":      luc.UTC().Format(time.RFC3339),
			}),
		})
	})
}

// laNguoiQuanTriCuoiCung reports whether removing this person's administrative rights would leave
// the commune with nobody.
//
// IT ASKS TWO THINGS AND BOTH MATTER: is the person IN the set at all — somebody who is not an
// administrator can be locked freely, whatever the size of the set — and is the set of size one.
// A plain `len(quanTri) <= 1` would refuse the ordinary case of a small commune locking an
// ordinary member of staff, which is how a correct rule gets deleted for being annoying.
func laNguoiQuanTriCuoiCung(quanTri []string, id string) bool {
	if len(quanTri) != 1 {
		return false
	}
	return quanTri[0] == id
}

func coQuyen(ds []string, ma string) bool {
	for _, v := range ds {
		if v == ma {
			return true
		}
	}
	return false
}

// khongCam returns the keys `can` grants that `dangCam` does not hold, in the order they were
// declared. The empty result is the permitted case.
func khongCam(can, dangCam []string) []string {
	co := make(map[string]bool, len(dangCam))
	for _, v := range dangCam {
		co[v] = true
	}
	var thieu []string
	for _, v := range can {
		if !co[v] {
			thieu = append(thieu, v)
		}
	}
	return thieu
}

// doiHoSo reports whether the edit would change anything. `ma`, `vai_tro_id`, `co_tai_khoan` and
// `dang_hoat_dong` are not compared because no path in Sua can change them.
func doiHoSo(truoc, sau domain.CanBoTomTat) bool {
	return truoc.HoTen != sau.HoTen || truoc.ChucVu != sau.ChucVu ||
		truoc.Email != sau.Email || truoc.BoPhanID != sau.BoPhanID ||
		truoc.DienThoaiCoQuan != sau.DienThoaiCoQuan || truoc.DiDongCaNhan != sau.DiDongCaNhan ||
		truoc.CoZalo != sau.CoZalo
}

// tomTatCanBo is the audit delta's view of one whole row, WITH PERSONAL DATA ALREADY MASKED.
//
// RULE 6, FORBIDDEN #4 IS THE WHOLE REASON THIS FUNCTION EXISTS: an audit trail holding raw names
// and telephone numbers IS a personal-data store — one that is append-only, kept for years, and
// outside every erasure and masking rule that applies to the table it was copied from. The entry
// still answers the question it is for, because the person is named by `Subject`, the staff code.
//
//	ho_ten               MASKED. A full name is personal data (rule 3).
//	di_dong_ca_nhan      MASKED. Personal data outright since #16.
//	dien_thoai_co_quan   MASKED TOO, although #16 classes the office landline as duty information.
//	                     Masking it costs nothing here — the current value is on the screen — and
//	                     the two columns sit next to each other in every list in this service. One
//	                     rule for both is one rule that cannot be applied to the wrong one.
//	email                NOT masked. A work address issued by the authority, the same reading
//	                     app/dang_nhap.go takes; it is also what makes an entry traceable to a
//	                     person when the directory row has since been edited.
func tomTatCanBo(cb domain.CanBoTomTat) map[string]any {
	return map[string]any{
		"ho_ten":             privacy.MaskName(cb.HoTen),
		"email":              cb.Email,
		"chuc_vu":            cb.ChucVu,
		"bo_phan_id":         cb.BoPhanID,
		"dien_thoai_co_quan": privacy.MaskPhone(cb.DienThoaiCoQuan),
		"di_dong_ca_nhan":    privacy.MaskPhone(cb.DiDongCaNhan),
	}
}

// tomTatDoiHoSo returns only the fields that actually moved, from whichever side is asked for.
//
// ONLY THE FIELDS THAT MOVED (rule 6, invariant 5): a delta carrying every column on every edit
// makes the one field somebody actually changed impossible to find in a ledger that is never
// deleted. The NAME of the column is always there even when its value is masked — "the personal
// mobile was changed, by this person, at this time" is the fact an inspection needs, and it is
// answerable without the digits.
func tomTatDoiHoSo(truoc, sau domain.CanBoTomTat, ben bool) map[string]any {
	ra := map[string]any{}
	if truoc.HoTen != sau.HoTen {
		ra["ho_ten"] = privacy.MaskName(chon(ben, truoc.HoTen, sau.HoTen))
	}
	if truoc.ChucVu != sau.ChucVu {
		ra["chuc_vu"] = chon(ben, truoc.ChucVu, sau.ChucVu)
	}
	if truoc.Email != sau.Email {
		ra["email"] = chon(ben, truoc.Email, sau.Email)
	}
	if truoc.BoPhanID != sau.BoPhanID {
		ra["bo_phan_id"] = chon(ben, truoc.BoPhanID, sau.BoPhanID)
	}
	if truoc.DienThoaiCoQuan != sau.DienThoaiCoQuan {
		ra["dien_thoai_co_quan"] = privacy.MaskPhone(chon(ben, truoc.DienThoaiCoQuan, sau.DienThoaiCoQuan))
	}
	if truoc.DiDongCaNhan != sau.DiDongCaNhan {
		ra["di_dong_ca_nhan"] = privacy.MaskPhone(chon(ben, truoc.DiDongCaNhan, sau.DiDongCaNhan))
	}
	if truoc.CoZalo != sau.CoZalo {
		// A flag, not personal data in itself: recorded as is.
		if ben {
			ra["co_zalo"] = truoc.CoZalo
		} else {
			ra["co_zalo"] = sau.CoZalo
		}
	}
	return ra
}

func chon(truoc bool, a, b string) string {
	if truoc {
		return a
	}
	return b
}

// deltaCanBo encodes the delta.
//
// A MARSHALLING FAILURE PRODUCES `null`, NOT A DROPPED ENTRY, and the choice is deliberate: the
// input is a map of strings this file built itself, so an error is unreachable in practice, and
// the alternatives are both worse. Returning an error would make every call site carry a branch
// nobody can test; skipping the entry would let the business write commit without a trail, which
// is the one thing rule 6 does not permit. A `null` delta still records who, what, when and on
// which record.
func deltaCanBo(v map[string]any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}
