package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The READ paths behind GET /api/v1/staff and GET /api/v1/staff/{id}.
//
// WHY THESE ARE NOT TheoID / TheoEmail, AND MUST NOT BE MERGED WITH THEM. Those two serve the
// SIGN-IN path and filter three conditions — not deleted, has an account, not locked. These two
// serve the STAFF REGISTER and filter one: not deleted. The predicates differ because the
// questions differ, and unifying them breaks whichever side loses its condition:
//
//   - widen the sign-in query and a public directory entry, or a locked-out former employee,
//     signs in;
//   - narrow the register and the 26 people of the commune's directory (12-danh-ba-can-bo.md §7)
//     disappear from the screen that exists to list them.
//
// Neither failure is loud. Two functions with two stated predicates is the cheap way to keep
// them from drifting into one.

// SapXepCanBo is the closed set of sorts GET /api/v1/staff offers. Declared here, next to the
// SQL, because it names real columns of `nguoi_dung` — only store/ knows column names.
//
// `ma` IS THE DEFAULT, ascending: it is stable, unique within the commune (UNIQUE (tenant_id,
// ma), migration 0001), and it is the value the audit trail quotes, so "the third row on page 2"
// means the same thing to the screen and to whoever later reads the trail.
//
// WHAT IS DELIBERATELY ABSENT, although the screen shows a ⇅ arrow on all six of its columns
// (14-cau-hinh.md §3). Every one of these is excluded for a reason pkg/page states, not for
// convenience:
//
//	ho_ten, dien_thoai   a sort key travels in a URL, an access log and a browser history.
//	                     Sorting on a person's own attributes puts personal data in all three
//	                     (rule 3, forbidden #4) — pkg/page says so in its package comment.
//	dang_nhap_gan_nhat   NULLABLE. `(col, id) > ($2, $3)` is NULL when col is NULL, so every
//	                     person who has never signed in would vanish from every page after the
//	                     first — silently, and they are exactly the rows an administrator is
//	                     looking for on this screen.
//	co_tai_khoan,        two distinct values. The id then does the real ordering, which is
//	dang_hoat_dong       slower than sorting by id and is not what the column header promised.
//
// Sorting by those is a client-side sort of the page in hand, or a decision to add a filter
// (`?has_account=`) — which is a route change, not a sort, and is not in this turn's scope.
var SapXepCanBo = page.NewAllowlist(page.Asc,
	page.Col("code", "ma", page.KindText),
	page.Col("created_at", "tao_luc", page.KindTime),
)

// cotTomTat IS READ BY POSITION in motTomTat, exactly like cotCanBo. Same trap, same discipline:
// `co_tai_khoan` and `dang_hoat_dong` are adjacent BOOLEANs and swapping them produces no error
// anywhere — the driver scans a bool into a bool and the two values trade places for good.
//
// mat_khau_hash IS NOT IN THIS LIST, and leaving it out is the point: a credential that is never
// selected cannot leak through a handler that forgot to drop it. domain.CanBoTomTat has no field
// to put it in either, so adding it here would not even compile.
const cotTomTat = `id, ma, ho_ten, email, chuc_vu,
                   coalesce(bo_phan_id,''), coalesce(vai_tro_id,''),
                   dien_thoai, co_tai_khoan, dang_hoat_dong,
                   dang_nhap_gan_nhat, tao_luc`

// locTomTat is the ONE predicate both read paths share.
//
// deleted_at IS NULL, and NOT `da_xoa = false`: rows predate any flag column, and a direct
// equality comparison silently drops all of them. A soft-deleted person is kept for the audit
// trail (rule 7, invariant 1) and must not appear on a screen; both halves of that sentence are
// this clause.
const locTomTat = `AND deleted_at IS NULL`

// DanhSach reads ONE page of the commune's staff register.
//
// BOTH KINDS OF RECORD COME BACK — people who can sign in (co_tai_khoan) and people who exist
// only in the public directory. One `nguoi_dung` table serves two screens (migration 0003), and
// `co_tai_khoan` is returned as a FIELD so each screen decides for itself. Filtering here would
// answer, on behalf of one screen, a question the customer has not been asked.
func (s *CanBoStore) DanhSach(ctx context.Context, yc page.Request) (page.Result[domain.CanBoTomTat], error) {
	// The sort column is read ONCE, outside the scan callback: the callback runs per row and the
	// column cannot change between rows.
	col := yc.Column()

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotTomTat,
		Table:   "nguoi_dung",
		Filter:  locTomTat,
	}, yc, func(rows *sql.Rows) (domain.CanBoTomTat, page.Anchor, error) {
		// quetTomTat, NOT motTomTat: QueryPage has already advanced the cursor to this row, and a
		// second Next() here would hand back every other row and drop the ones in between.
		cb, err := quetTomTat(rows)
		if err != nil {
			return domain.CanBoTomTat{}, page.Anchor{}, err
		}
		moc, err := mocCanBo(col, cb)
		if err != nil {
			return domain.CanBoTomTat{}, page.Anchor{}, err
		}
		return cb, page.Anchor{Key: moc, ID: cb.ID}, nil
	})
}

// ChiTiet reads one staff record by internal id.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context.
// An id belonging to another commune therefore matches no row and comes back as
// ErrCanBoKhongTonTai — which the handler answers with 404, not 403. A 403 would confirm that the
// record exists somewhere, and the existence of another authority's record is itself information
// (rule 4, forbidden #2).
func (s *CanBoStore) ChiTiet(ctx context.Context, id string) (domain.CanBoTomTat, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotTomTat, "nguoi_dung", locTomTat+` AND id = $2`, id)
	if err != nil {
		return domain.CanBoTomTat{}, fmt.Errorf("can_bo: đọc chi tiết: %w", err)
	}
	defer rows.Close()
	return motTomTat(rows)
}

// mocCanBo maps the sort column back onto the field that was sorted on.
//
// WHY THIS SWITCH EXISTS AND WHY IT MUST STAY IN STEP WITH SapXepCanBo: store.QueryPage asks the
// caller to state each row's anchor, because only the caller knows which scanned field the sort
// key is. An anchor built from the wrong field yields a cursor that walks the list in an order
// nobody asked for — the next page repeats records and skips others, with no error.
//
// QueryPage catches a KIND mismatch (text anchor against a time column). It cannot catch a
// same-kind mismatch, and it checks nothing at all on a list short enough to fit one page. So the
// default branch is an error rather than a fallback: a sort added to the allowlist and forgotten
// here fails loudly on the first request instead of quietly reordering a register.
func mocCanBo(col page.Column, cb domain.CanBoTomTat) (page.Key, error) {
	switch col.SQL {
	case "ma":
		return page.TextKey(cb.Ma), nil
	case "tao_luc":
		return page.TimeKey(cb.TaoLuc), nil
	default:
		return page.Key{}, fmt.Errorf(
			"can_bo: cột sắp xếp %q không có mốc tương ứng — SapXepCanBo và mocCanBo lệch nhau", col.SQL)
	}
}

// motTomTat reads the FIRST row of a single-row read, or reports that there is none.
func motTomTat(rows quetDuoc) (domain.CanBoTomTat, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CanBoTomTat{}, fmt.Errorf("can_bo: đọc: %w", err)
		}
		return domain.CanBoTomTat{}, ErrCanBoKhongTonTai
	}
	return quetTomTat(rows)
}

// quetTomTat scans the row the cursor is ALREADY on. POSITIONAL — this list of Scan targets must
// stay in lockstep with cotTomTat. See the note there on the two adjacent booleans.
//
// &cb.DangNhapGanNhat is a **time.Time, which is how database/sql is told that SQL NULL is a
// legitimate answer: it sets the pointer to nil instead of failing the scan. A plain time.Time
// target would make every person who has never signed in an error, and the register would stop
// listing precisely the accounts an administrator opened it to find.
func quetTomTat(rows quetDuoc) (domain.CanBoTomTat, error) {
	var cb domain.CanBoTomTat
	err := rows.Scan(&cb.ID, &cb.Ma, &cb.HoTen, &cb.Email, &cb.ChucVu,
		&cb.BoPhanID, &cb.VaiTroID, &cb.DienThoai,
		&cb.CoTaiKhoan, &cb.DangHoatDong,
		&cb.DangNhapGanNhat, &cb.TaoLuc)
	if err != nil {
		return domain.CanBoTomTat{}, fmt.Errorf("can_bo: đọc dòng: %w", err)
	}
	return cb, nil
}
