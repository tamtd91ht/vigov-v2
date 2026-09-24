package app

// The use case behind PUT /api/v1/roles/{id}/permissions — saving ONE column of the Phân quyền
// matrix (docs/ui-ux/14-cau-hinh.md §4, §12.5). The body is the WHOLE set of keys the role is to
// hold afterwards: replace-by-set.
//
// THIS IS THE ROUTE THAT DECIDES WHO MAY GRANT WHAT, so every decision the customer took on #13 and
// #14 (2026-09-22) and the user refined for this route (2026-09-24) meets here:
//
//	#14 first    the actor may not save the column of a role THEY CURRENTLY HOLD. Otherwise
//	             `admin.role` subsumes every other key: tick it on your own column and you hold it.
//	#14 second   every key ADDED and every key REMOVED must be one the actor holds. Adding a key you
//	             lack is escalation by proxy (grant it to a colleague's role, ask them to act);
//	             removing one you lack is acting on a right you have no authority over. The
//	             conservative reading of "cannot touch rights you do not have" — user decision
//	             2026-09-24. Keys left UNCHANGED are not checked: an unchanged cell is not an act.
//	#13          the save is refused if afterwards nobody active holds `admin.user`, OR nobody active
//	             holds `admin.role`. The first locks the commune out of managing people, the second
//	             out of managing permissions — both are procedural dead ends ADR 0003 gives the vendor
//	             no way to reopen.
//
// SESSIONS NEED NOTHING AFTER A SAVE. Permissions are read from the database on every request
// (store/checker.go; internal/http/middleware.go leaves Principal.Roles empty on purpose;
// core/staffauth reads per request with no cache), so a changed column takes effect on the next
// request of every holder. Nothing is cached that would need revoking.

import (
	"context"
	"errors"
	"strings"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// KhoPhanQuyen is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION, for
// the reason given on KhoDanhBaCanBo: there is no signature that would let the grant change and its
// audit entry land in two transactions.
type KhoPhanQuyen interface {
	QuanTriDeGhi(ctx context.Context, tx *store.ScopedTx) ([]domain.NguoiGiuQuyen, error)
	PhanQuyenDeGhi(ctx context.Context, tx *store.ScopedTx) ([]domain.NguoiGiuQuyen, error)
	VaiTroDeGhi(ctx context.Context, tx *store.ScopedTx, vaiTroID string) ([]string, error)
	VaiTroCuaNguoi(ctx context.Context, tx *store.ScopedTx, canBoID string) (string, error)
	QuyenDangGiu(ctx context.Context, tx *store.ScopedTx, canBoID string) ([]string, error)
	KhoaKhongTonTai(ctx context.Context, tx *store.ScopedTx, ds []string) ([]string, error)
	ThemCap(ctx context.Context, tx *store.ScopedTx, vaiTroID string, them []string, capBoi string) error
	BoCap(ctx context.Context, tx *store.ScopedTx, vaiTroID string, bo []string) error
}

// HanhViLuuPhanQuyenVaiTro is the verb in the trail. One verb for the whole save: the delta names
// every key added (`them`) and removed (`bo`), so an inspection reads one entry per press of Lưu.
const HanhViLuuPhanQuyenVaiTro = "luu_phan_quyen_vai_tro"

var (
	// ErrPhanQuyenKhongConNguoiGiu — #13 extended to `admin.role` for this route. The key that would
	// be lost is on the error so the sentence can name it.
	ErrPhanQuyenKhongConNguoiGiu = errors.New("phan_quyen: thao tác này sẽ làm xã không còn ai giữ quyền")

	// ErrKhoaQuyenKhongTonTai — rule 5, invariant 3c. See LoiKhoaQuyenKhongTonTai for the keys.
	ErrKhoaQuyenKhongTonTai = errors.New("phan_quyen: khoá quyền không có trong danh mục")
)

// LoiKhongConNguoiGiu names the key (`admin.user` or `admin.role`) the save would leave unheld.
type LoiKhongConNguoiGiu struct{ Khoa string }

func (e *LoiKhongConNguoiGiu) Error() string {
	return ErrPhanQuyenKhongConNguoiGiu.Error() + ": " + e.Khoa
}
func (e *LoiKhongConNguoiGiu) Is(target error) bool { return target == ErrPhanQuyenKhongConNguoiGiu }

// LoiKhoaQuyenKhongTonTai names the unknown keys. Permission keys are not personal data.
type LoiKhoaQuyenKhongTonTai struct{ Thieu []string }

func (e *LoiKhoaQuyenKhongTonTai) Error() string {
	return ErrKhoaQuyenKhongTonTai.Error() + ": " + strings.Join(e.Thieu, ", ")
}
func (e *LoiKhoaQuyenKhongTonTai) Is(target error) bool { return target == ErrKhoaQuyenKhongTonTai }

// PhanQuyenVaiTro saves one role's column.
type PhanQuyenVaiTro struct {
	db  *store.DB
	kho KhoPhanQuyen
}

func NewPhanQuyenVaiTro(db *store.DB, kho KhoPhanQuyen) *PhanQuyenVaiTro {
	return &PhanQuyenVaiTro{db: db, kho: kho}
}

// Luu replaces role `vaiTroID`'s key set with `dsQuyen` and returns the set as saved, sorted.
//
// THE ORDER BELOW IS LOAD-BEARING in two ways: the locks are taken in the order stated in
// store/vai_tro_quyen_ghi.go, and every refusal is decided BEFORE the first write, so a refused save
// writes nothing and audits nothing.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — saving the column as it already stands is not an
// event, and it is what makes the route's idem.KhongCan declaration true.
func (uc *PhanQuyenVaiTro) Luu(ctx context.Context, vaiTroID string, dsQuyen []string,
	nguoi NguoiThucHien) ([]string, error) {

	if err := nguoi.hopLe(); err != nil {
		return nil, err
	}
	if vaiTroID == "" {
		return nil, idstore.ErrVaiTroKhongTonTaiDeGhi
	}
	if err := domain.KiemTraIDThamChieu(vaiTroID); err != nil {
		return nil, err
	}
	// Shape first, outside the transaction: a malformed body must never hold a row lock.
	sau, err := domain.ChuanHoaTapQuyen(dsQuyen)
	if err != nil {
		return nil, err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// LOCK ORDER 1 and 2 — the two holder sets #13 is decided on.
		quanTri, err := uc.kho.QuanTriDeGhi(ctx, tx)
		if err != nil {
			return err
		}
		phanQuyen, err := uc.kho.PhanQuyenDeGhi(ctx, tx)
		if err != nil {
			return err
		}
		// LOCK ORDER 3 — the target column. 404 for absent, soft-deleted or another commune's.
		truoc, err := uc.kho.VaiTroDeGhi(ctx, tx, vaiTroID)
		if err != nil {
			return err
		}

		// #14, FIRST CONSTRAINT. The actor's row is locked by step 2 (they hold `admin.role`), so
		// this answer cannot move before the write.
		vaiTroToi, err := uc.kho.VaiTroCuaNguoi(ctx, tx, nguoi.ID)
		if err != nil {
			return err
		}
		if vaiTroToi == vaiTroID {
			return ErrTuThaoTacChinhMinh
		}

		// Unknown keys before anything about authority: a key that does not exist is a malformed
		// request, and naming it as "you do not hold it" would send the administrator the wrong way.
		thieuDanhMuc, err := uc.kho.KhoaKhongTonTai(ctx, tx, sau)
		if err != nil {
			return err
		}
		if len(thieuDanhMuc) > 0 {
			return &LoiKhoaQuyenKhongTonTai{Thieu: thieuDanhMuc}
		}

		them, bo := domain.HieuTapQuyen(truoc, sau)
		if len(them) == 0 && len(bo) == 0 {
			return nil
		}

		// #14, SECOND CONSTRAINT — on the ADDED and the REMOVED keys, both read in this transaction.
		quyenToi, err := uc.kho.QuyenDangGiu(ctx, tx, nguoi.ID)
		if err != nil {
			return err
		}
		// FAIL CLOSED: an actor who no longer holds `admin.role` under the lock (withdrawn between
		// the route guard and here) has no authority over any column.
		if !coQuyen(quyenToi, idstore.QuyenPhanQuyen) {
			return &LoiTraoQuyenKhongCam{Thieu: []string{idstore.QuyenPhanQuyen}}
		}
		if thieu := khongCam(append(append([]string{}, them...), bo...), quyenToi); len(thieu) > 0 {
			return &LoiTraoQuyenKhongCam{Thieu: thieu}
		}

		// #13, on the state AFTER the save, for both keys, against the sets locked above.
		if !domain.ConNguoiGiuSauKhiLuu(quanTri, vaiTroID, sau, idstore.QuyenQuanTriNguoiDung) {
			return &LoiKhongConNguoiGiu{Khoa: idstore.QuyenQuanTriNguoiDung}
		}
		if !domain.ConNguoiGiuSauKhiLuu(phanQuyen, vaiTroID, sau, idstore.QuyenPhanQuyen) {
			return &LoiKhongConNguoiGiu{Khoa: idstore.QuyenPhanQuyen}
		}

		if err := uc.kho.BoCap(ctx, tx, vaiTroID, bo); err != nil {
			return err
		}
		if err := uc.kho.ThemCap(ctx, tx, vaiTroID, them, nguoi.Vet.ID); err != nil {
			return err
		}

		// SAME TRANSACTION (rule 6, invariant 3). Subject is the role id — the only value that still
		// means the same role years later (a name can be edited). Keys only: nothing personal.
		// `them` and `bo` are stated explicitly so nobody has to diff `truoc` against `sau` by eye.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViLuuPhanQuyenVaiTro,
			Subject: vaiTroID,
			Delta: deltaCanBo(map[string]any{
				"truoc": khongNil(truoc),
				"sau":   khongNil(sau),
				"them":  khongNil(them),
				"bo":    khongNil(bo),
			}),
		})
	})
	if err != nil {
		return nil, err
	}
	return sau, nil
}

// khongNil makes an empty set serialise as [] rather than null, in the trail and in the response:
// "held nothing" and "unknown" must not look alike to the person reading either.
func khongNil(ds []string) []string {
	if ds == nil {
		return []string{}
	}
	return ds
}
