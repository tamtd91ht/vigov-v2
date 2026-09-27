package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The read behind GET /api/v1/commune-staff — the public Zalo Mini App staff directory
// (docs/ui-ux/12-danh-ba-can-bo.md:117, owner decision 2026-09-27). The SIXTH staff read in this
// package, and the only one anybody on the internet can trigger.
//
// IT MUST NOT BE MERGED WITH ANY OF THE OTHER FIVE (see can_bo_chon_nguoi.go for their predicates).
// Its predicate is the strictest of all, and its column list is the only one chosen for a reader who
// is not staff.

// TranDanhBaCongKhai is the hard upper bound on one commune's published directory.
//
// NOT PAGINATED, for the reason TranDanhBaChonNguoi gives: a directory whose second page a Mini App
// forgets to fetch is a directory missing the person a citizen needed to ring, with nothing on screen
// saying so. The customer's own directory is 26 people (12-danh-ba-can-bo.md §7); 500 is the point past
// which the data is wrong, not large — and the bound is what keeps a public route on a process serving
// 200+ communes from being unbounded (skills/rest-api-design §5).
const TranDanhBaCongKhai = 500

// ErrQuaNhieuCanBoCongKhai says the ceiling was reached. The caller answers 500 and REFUSES: a
// silently short public directory is indistinguishable, to a citizen, from a complete one.
var ErrQuaNhieuCanBoCongKhai = errors.New("can_bo: danh bạ công khai vượt trần")

// truyVanDanhBaCongKhai reads one commune's published directory, unit name included, in ONE statement.
//
// WHAT IS SELECTED IS THE CONTRACT: name, position, unit name, office line, personal mobile, has-Zalo.
// No `id`, no `ma`, no `email`, no account or lock flag, no consent mark. `nd.ma` appears ONLY in the
// ORDER BY, as the tie-break that keeps two people of the same name from swapping places between two
// loads; it is never read into Go. POSITIONAL — in lockstep with the Scan in danhBaCongKhai; `ho_ten`
// and `chuc_vu` are adjacent TEXT columns, and so are the two numbers: a swap errors nowhere.
//
// WHO IS IN IT — every condition is a decision, stated where it is paid:
//
//	deleted_at IS NULL            a soft-deleted row is a duplicate kept for the trail (rule 7 inv 2).
//	dang_hoat_dong                a LOCKED person is somebody retired or moved (#10). A citizen must
//	                              not be handed the number of someone who no longer holds the post.
//	                              NOT `co_tai_khoan`: most of the published directory are people who
//	                              never sign in (12-danh-ba-can-bo.md §7 — 26 entries, 12 accounts);
//	                              requiring an account would publish almost nobody.
//	hien_tren_mini_app            the administrator published this person, per person (#12).
//	dong_y_cong_khai_luc IS NOT NULL  and the consent evidence is present. Migration 0010 §3 already
//	dong_y_cong_khai_ghi_boi <> ''    makes these follow from the flag; they are repeated HERE because
//	                              this is the one read where a CHECK dropped by a future migration would
//	                              publish a personal mobile nobody consented to. Two walls, each enough.
//
// EVERY TABLE IS BOUND TO THE COMMUNE. `nd.tenant_id = $1` is the Scoped.QueryJoin contract; the JOIN
// repeats `bp.tenant_id = nd.tenant_id`, because unit ids are per commune and a join on id alone would
// print commune B's unit name wherever ids collide (rule 1). A LEFT JOIN, with the unit conditions in
// the ON clause: a person in no unit, or in a unit since soft-deleted, stays in the directory with an
// empty unit name rather than vanishing from it.
//
// ORDER — the specification's: `thu_tu_danh_ba`, then unit name. NULLS LAST on both, because NULL
// order means "no explicit position" (0010 §1) and must not jump ahead of everybody who has one; then
// name and code as tie-breaks, so the order is total.
const truyVanDanhBaCongKhai = `
SELECT nd.ho_ten, nd.chuc_vu, coalesce(bp.ten, ''),
       nd.dien_thoai_co_quan, nd.di_dong_ca_nhan, nd.co_zalo
FROM nguoi_dung nd
LEFT JOIN bo_phan bp
       ON bp.tenant_id = nd.tenant_id
      AND bp.id        = nd.bo_phan_id
      AND bp.deleted_at IS NULL
WHERE nd.tenant_id = $1
` + locDanhBaCongKhai + `
ORDER BY nd.thu_tu_danh_ba ASC NULLS LAST, bp.ten ASC NULLS LAST, nd.ho_ten ASC, nd.ma ASC
LIMIT $2`

// locDanhBaCongKhai is the publication predicate, kept as its own constant so the test can assert it
// clause by clause against the statement that reached the driver.
const locDanhBaCongKhai = `  AND nd.deleted_at IS NULL
  AND nd.dang_hoat_dong
  AND nd.hien_tren_mini_app
  AND nd.dong_y_cong_khai_luc IS NOT NULL
  AND nd.dong_y_cong_khai_ghi_boi <> ''`

// DanhBaCongKhai reads the published directory of the commune in the context.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.QueryJoin binds it to $1 from the context
// (rule 1, invariant 5). On the public route the context's commune was resolved by the SERVER from
// the `host` the caller named — never a tenant_id the caller sent.
func (s *CanBoStore) DanhBaCongKhai(ctx context.Context) ([]domain.CanBoCongKhai, error) {
	return s.danhBaCongKhai(ctx, TranDanhBaCongKhai)
}

// danhBaCongKhai takes the ceiling as a parameter only so a test can reach the refusal with a handful
// of rows; production has exactly one caller, with TranDanhBaCongKhai.
func (s *CanBoStore) danhBaCongKhai(ctx context.Context, tran int) ([]domain.CanBoCongKhai, error) {
	// LIMIT is the ceiling PLUS ONE — the only way "too many" is distinguishable from "exactly the
	// ceiling". See BoPhanStore.DanhSach.
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanDanhBaCongKhai, tran+1)
	if err != nil {
		return nil, fmt.Errorf("can_bo: đọc danh bạ công khai: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.CanBoCongKhai, 0, 32)
	for rows.Next() {
		var cb domain.CanBoCongKhai
		// NEVER LOG A SCANNED ROW: a name and a personal mobile (rule 3, forbidden #1).
		if err := rows.Scan(&cb.HoTen, &cb.ChucVu, &cb.TenBoPhan,
			&cb.DienThoaiCoQuan, &cb.DiDongCaNhan, &cb.CoZalo); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng danh bạ công khai: %w", err)
		}
		ra = append(ra, cb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt danh bạ công khai: %w", err)
	}
	if len(ra) > tran {
		// DROPPED, not trimmed: a partial list handed back is a refusal one careless caller away from
		// being rendered.
		return nil, ErrQuaNhieuCanBoCongKhai
	}
	return ra, nil
}
