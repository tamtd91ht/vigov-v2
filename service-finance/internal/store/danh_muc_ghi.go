package store

// The parts of the catalogue WRITE path that belong to the whole service rather than to one
// catalogue table.
//
// WHY THEY ARE SHARED AND NOT COPIED PER TABLE: a service can own more than one reference
// catalogue — service-petitions owns two — and three refusals meaning the same thing under three
// names is three ways for the HTTP layer to map one of them wrongly. The error a caller sees would
// then depend on which catalogue they touched, which is a difference with no cause.

import (
	"database/sql"
	"errors"
	"fmt"
)

var (
	// ErrDanhMucKhongTonTai — no live row with that id in THIS commune. The handler answers 404
	// and deliberately does not distinguish "belongs to another commune" from "does not exist":
	// telling them apart confirms the existence of another commune's row.
	ErrDanhMucKhongTonTai = errors.New("danh_muc: không tồn tại trong xã này")

	// ErrMaDaTonTai — the code is already taken in this commune, SOFT-DELETED ROWS INCLUDED.
	//
	// Counting deleted rows is not strictness for its own sake: `UNIQUE (tenant_id, ma)` counts
	// them too, on purpose. A commune that could soft-delete `cong-van` and create a new,
	// unrelated `cong-van` would silently change the type shown on every document already
	// registered under the old one — and document numbering follows the type (ADR 0024).
	// Rule 7, invariant 3: an issued code is never reissued.
	ErrMaDaTonTai = errors.New("danh_muc: mã đã được dùng trong xã này")

	// ErrDanhMucDayTran — the commune is already at this catalogue's ceiling (the Tran… constant
	// beside its own store).
	//
	// WHY A CREATE HAS TO CARE ABOUT THE READ ROUTE'S CEILING: DanhSach REFUSES rather than
	// truncates past that number, so the row that crosses the line does not merely add itself —
	// it turns the whole catalogue into a 500 for every screen in the commune. Refusing the write
	// breaks one button; allowing it breaks the registration form.
	ErrDanhMucDayTran = errors.New("danh_muc: danh mục đã đầy")
)

// doiMotDong turns "nothing was updated" into ErrDanhMucKhongTonTai.
//
// WHY IT IS CHECKED AT ALL WHEN THE CALLER ALREADY READ THE ROW: the read and the write are two
// statements, and a method used without the read — a future caller, a retry path — would otherwise
// report success for a row that does not exist. An UPDATE touching zero rows is not an error to
// PostgreSQL; it is only an error to us.
func doiMotDong(kq sql.Result, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("danh_muc: %s: đọc số dòng: %w", viec, err)
	}
	if n == 0 {
		return ErrDanhMucKhongTonTai
	}
	return nil
}
