package store

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/core/store"
)

// The shared read for this service's reference catalogues (ADR 0024, migration 0005).
//
// ONE READER FOR SEVERAL TABLES, ON PURPOSE, AND THE ARGUMENT IS THE MIGRATION'S OWN. Migration
// 0005 gives every catalogue in this service ONE trigger function with the reason spelled out at
// line 124: "the tiers are a property of the SHAPE, not of one catalogue. A copy per table is a
// copy that gets fixed in two places and forgotten in the third." The read side has exactly the
// same shape — same five columns, same soft-delete predicate, same order, same ceiling-plus-one
// refusal — so the same reasoning applies to it. The part that must not be forgotten in the third
// copy is `deleted_at IS NULL` and the refusal; both live here once.
//
// WHY IT IS NOT BoPhanStore / VaiTroStore's SHAPE, which duplicate a similar query between them:
// those two read DIFFERENT columns from differently-shaped tables (`cha_id` on one, `la_lanh_dao`
// on the other) and were written before there was a catalogue shape to share. These two read the
// same five columns from two tables the migration deliberately declared identically.
//
// WHEN TO SPLIT IT AGAIN, so the next person does not have to decide from scratch: the day one
// catalogue needs a column the other does not have. At that point this stops being one shape and
// becomes two, and a parameter added to keep them together would be the abstraction that hides the
// difference.
//
// EACH CATALOGUE KEEPS ITS OWN STORE TYPE, ITS OWN CEILING AND ITS OWN SENTINEL ERROR. What is
// shared is the statement, not the identity: a handler still cannot be handed the wrong catalogue,
// and a log line still names which one overflowed.

// dongDanhMuc is one catalogue row as it leaves the database. Unexported: it is the shape of the
// SQL, not a business concept — each store maps it into its own domain type, which is what keeps
// ResidentialUnitType and TaskBloc two things the compiler can tell apart.
type dongDanhMuc struct {
	ID        string
	Ma        string
	Nhan      string
	LaMacDinh bool
	DangDung  bool
}

// cotDanhMuc IS READ BY POSITION in docDanhMuc. `ma` and `nhan` are adjacent TEXT columns and
// `la_mac_dinh` and `dang_dung` are adjacent BOOLEANs: swapping either pair — here or in the Scan —
// produces no error at all. The first swap shows slugs where labels belong; the second pre-selects
// a row the commune has taken out of use.
const cotDanhMuc = `id, ma, nhan, la_mac_dinh, dang_dung`

// docDanhMuc reads one commune's whole catalogue from `bang`, ordered, bounded by `tran`.
//
// `bang` IS INTERPOLATED INTO SQL AND MUST STAY A COMPILE-TIME CONSTANT. Both call sites pass an
// untyped string constant declared next to their store; nothing here ever reaches a request value,
// and it must not start to. Scoped.Query has the same contract for its `bang` argument — the
// commune is the only thing it binds, and it binds it to $1 from the context.
//
// NOT PAGINATED, same decision and same reasons as BoPhanStore.DanhSach: a closed reference list of
// a handful of rows whose consumers need it whole, not a register that grows with use. The bound
// pagination would have provided is `tran`, enforced in SQL.
//
// ROWS OUT OF USE ARE RETURNED; ONLY SOFT-DELETED ROWS DROP OUT. That is the migration's own
// statement of what this list is (0005:283) and it is what lets one route serve both the catalogue
// screen, which must show a disabled row with its "Đã tắt" chip, and a picker, which filters on
// DangDung. Filtering here would leave the catalogue screen unable to display what it manages.
//
// ORDER BY thu_tu, ma — AND `ma` IS THE TIE-BREAK RATHER THAN `nhan` BECAUSE IT IS UNIQUE PER
// COMMUNE (UNIQUE (tenant_id, ma), migration 0005:265). That makes the order TOTAL: two rows
// sharing a `thu_tu` can never swap places between two calls. Breaking ties on the label would not
// guarantee it — two rows may carry the same label — and an unstable order makes a client-side diff
// flicker on every reload and makes any test of this compare sets by accident.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the catalogue of the commune the request arrived in (rule 1,
// invariant 5).
func docDanhMuc(ctx context.Context, db *store.DB, bang string, tran int, quaNhieu error) ([]dongDanhMuc, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page indistinguishable from a complete list
	// of that size — the truncation these routes refuse to perform, performed by the bound meant to
	// prevent it.
	rows, err := db.For(ctx).Query(ctx, cotDanhMuc, bang,
		`AND deleted_at IS NULL ORDER BY thu_tu, ma LIMIT $2`, tran+1)
	if err != nil {
		return nil, fmt.Errorf("%s: đọc danh mục: %w", bang, err)
	}
	defer rows.Close()

	ra := make([]dongDanhMuc, 0, 16)
	for rows.Next() {
		var d dongDanhMuc
		// POSITIONAL — in lockstep with cotDanhMuc. See the note there on the two adjacent pairs.
		if err := rows.Scan(&d.ID, &d.Ma, &d.Nhan, &d.LaMacDinh, &d.DangDung); err != nil {
			return nil, fmt.Errorf("%s: đọc dòng: %w", bang, err)
		}
		ra = append(ra, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: duyệt kết quả: %w", bang, err)
	}
	if len(ra) > tran {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation one
		// careless `if err != nil { log }` later.
		return nil, quaNhieu
	}
	return ra, nil
}
