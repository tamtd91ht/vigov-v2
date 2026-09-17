package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// VaiTroStore reads the commune's roles.
//
// IT DECIDES NOTHING. Access decisions are Checker's, one permission at a time, against
// `vai_tro_quyen`, on every request. This store answers "what is this role called" — a display
// question. The two must not merge: a caller that read a role here and branched on it would be
// deciding access from a description, which is rule 5, forbidden #3.
type VaiTroStore struct {
	db *store.DB
}

func NewVaiTroStore(db *store.DB) *VaiTroStore { return &VaiTroStore{db: db} }

// truyVanVaiTroCuaCanBo reads the role of ONE staff member.
//
// A LEFT JOIN, AND THAT IS THE WHOLE POINT OF THE QUERY. `nguoi_dung.vai_tro_id` is NULLABLE — a
// person can be in the commune's directory with no role assigned yet — and an inner join would
// return no row for them, which is indistinguishable from "this person does not exist". The caller
// has to tell those two apart: one is a normal state the screen renders, the other is a defect.
// With the left join, "the person exists but has no role" is a row full of NULLs, and "no such
// person" is no row at all.
//
// THE JOIN IS CONSTRAINED TO THE SAME COMMUNE, not merely to the matching id. Joining on id alone
// would match another commune's role wherever ids collide — the leak rule 1 exists to prevent, and
// one that no single-commune test would ever show. Same discipline as checker.go.
//
// vt.deleted_at IS NULL SITS IN THE JOIN, NOT IN THE WHERE. In the WHERE it would filter out the
// PERSON along with the deleted role, turning "their role was deleted" into "this person does not
// exist" — and the handler would answer 500 for a state that is merely untidy data. In the join it
// leaves the person and empties the role, which is the same answer as "no role assigned": both mean
// there is no role to name. The cost of that merge is stated where it is paid, on VaiTroCuaCanBo.
//
// IT DOES NOT FILTER co_tai_khoan OR dang_hoat_dong, unlike every query in checker.go and
// can_bo.go. Those two decide whether somebody may ACT, and they are enforced upstream — the
// session middleware has already refused a locked or account-less person before any handler runs.
// Repeating them here would add a predicate that can only be wrong in one direction: dropped from
// the middleware and left here, it looks like the check still happens.
const truyVanVaiTroCuaCanBo = `
SELECT vt.ma, vt.ten, vt.la_lanh_dao
FROM nguoi_dung nd
LEFT JOIN vai_tro vt
       ON vt.tenant_id  = nd.tenant_id
      AND vt.id         = nd.vai_tro_id
      AND vt.deleted_at IS NULL
WHERE nd.tenant_id = $1
  AND nd.id = $2
  AND nd.deleted_at IS NULL`

// VaiTroCuaCanBo reports the role of one staff member in the commune carried by ctx.
//
// THREE OUTCOMES, AND THE CALLER MUST BE ABLE TO TELL THEM APART:
//
//	(vt, true, nil)    the person holds a role
//	({}, false, nil)   the person exists and has NO role assigned — a normal state
//	({}, false, err)   the role could not be READ
//
// The second and the third must never collapse into one answer. "No role assigned" is a fact about
// a person and the screen renders it as such; "could not read" is a failure, and answering it with
// the same empty value would state, on a government screen, that somebody holds no role when the
// truth is that nobody knows. The bool exists so a caller cannot accidentally read one for the
// other — the same asymmetry Checker.QuyenCua argues for its list.
//
// A SOFT-DELETED ROLE COMES BACK AS "no role assigned", and that merge is deliberate: both mean
// there is no role to put on the screen, and the alternative — a fourth outcome for a role that was
// deleted while somebody still points at it — would be a state no screen has anything to say about.
// It is untidy data, not a failure: rule 7 keeps the deleted row, and reassigning the person is an
// administrative act, not something this read path invents.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.QueryJoin binds it to $1 from the
// context, so an id belonging to another commune matches no row and comes back as
// ErrCanBoKhongTonTai.
func (s *VaiTroStore) VaiTroCuaCanBo(ctx context.Context, canBoID string) (domain.VaiTro, bool, error) {
	if canBoID == "" {
		// Fail closed rather than run a query that would match whatever row has an empty id. Not an
		// error the caller can do anything about, but it is a defect, so it is named.
		return domain.VaiTro{}, false, fmt.Errorf("vai_tro: %w", ErrCanBoKhongTonTai)
	}

	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanVaiTroCuaCanBo, canBoID)
	if err != nil {
		return domain.VaiTro{}, false, fmt.Errorf("vai_tro: đọc vai trò của cán bộ: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.VaiTro{}, false, fmt.Errorf("vai_tro: duyệt kết quả: %w", err)
		}
		return domain.VaiTro{}, false, ErrCanBoKhongTonTai
	}

	// NULLABLE TARGETS, because the left join produces NULLs for a person with no role. A plain
	// string target turns that ordinary state into a scan error, which the caller would then have
	// to report as "could not read" — exactly the collapse this function exists to avoid.
	var ma, ten sql.NullString
	var laLanhDao sql.NullBool
	if err := rows.Scan(&ma, &ten, &laLanhDao); err != nil {
		return domain.VaiTro{}, false, fmt.Errorf("vai_tro: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.VaiTro{}, false, fmt.Errorf("vai_tro: duyệt kết quả: %w", err)
	}
	if !ma.Valid {
		return domain.VaiTro{}, false, nil
	}
	return domain.VaiTro{Ma: ma.String, Ten: ten.String, LaLanhDao: laLanhDao.Bool}, true, nil
}
