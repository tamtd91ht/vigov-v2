package store

// THE PUBLIC READS OF THE MINI APP CONTENT REGISTER — what GET /api/v1/commune-news and
// GET /api/v1/commune-news/{id} serve to anybody who knows a commune's domain (owner decision
// 2026-09-27; docs/ui-ux/11-noi-dung-mini-app.md:189-190), and — since the user's decision of
// 2026-09-30 — which categories GET /api/v1/commune-news/categories may offer as chips.
//
// TWO METHODS NEXT TO DanhSach / TheoID, NOT A FLAG ON THEM, and the difference is the predicate: the
// staff register lists every state (§6's three chips), the public read lists ONE — `dang-hien`. A
// boolean parameter on the staff read would be one wrong `true` away from publishing every draft and
// everything waiting for approval.
//
// THE STATE IS A BOUND PARAMETER HOLDING domain.TrangThaiDangHien, not a literal re-spelt here, so the
// SQL and domain.NoiDungMiniApp.HienChoDan read the same constant. The handler checks HienChoDan on
// every row as well — a second wall, not a second definition.
//
// NO NEW INDEX: `noi_dung_mini_app_so` (tenant_id, tao_luc DESC, id) WHERE deleted_at IS NULL serves the
// order and the cursor, and `trang_thai` is filtered on the rows it walks. A commune whose drafts vastly
// outnumber its published items would make the walk longer; a partial index `WHERE trang_thai =
// 'dang-hien'` is the fix then, in a migration, after it is measured.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// DanhSachCongKhai reads ONE PAGE of the commune's PUBLISHED items, newest first (tao_luc DESC, id —
// SapXepNoiDungMiniApp's only column, and the cursor QueryPage pages on).
//
// THE BODY IS NOT IN THE RESULT, for the reason cotNoiDungMiniApp gives: a page of a hundred articles
// at the body's cap is twenty million runes.
//
// THE COMMUNE IS $1 FROM THE CONTEXT. On the public route the handler put it there after the PLATFORM
// resolved the `host` the caller named; this method cannot tell and does not need to.
//
// itemType "" = every type EXCEPT `banner` (ADR 0067 §5 decision 5: a banner is a promotional picture,
// not news, and only ever comes back on `?type=banner` — PublicBanners). Any other value is bound as the
// next placeholder; the handler has already refused a code outside the six. itemType `banner` here is
// not reached from the handler (it calls PublicBanners) and would list banners by date.
//
// categoryID "" = every category (and items filed nowhere). Otherwise only items filed under that
// category OR ANY LIVE DESCENDANT (user decision 2026-09-30: choosing a parent chip includes its
// children's items) — see publicCategorySubtree. Combined with itemType by AND. An id of another commune,
// a soft-deleted category or no category at all matches nothing: an empty page, the same answer.
func (s *NoiDungMiniAppStore) DanhSachCongKhai(ctx context.Context, itemType domain.LoaiNoiDung,
	categoryID string, yc page.Request) (page.Result[domain.NoiDungMiniApp], error) {

	filter := ` AND deleted_at IS NULL AND trang_thai = $2`
	args := []any{string(domain.TrangThaiDangHien)}
	if itemType != "" {
		args = append(args, string(itemType))
		filter += fmt.Sprintf(` AND loai = $%d`, len(args)+1) // +1: $1 is the commune
	} else {
		args = append(args, string(domain.LoaiBanner))
		filter += fmt.Sprintf(` AND loai <> $%d`, len(args)+1)
	}
	if categoryID != "" {
		args = append(args, categoryID)
		filter += fmt.Sprintf(publicCategorySubtree, len(args)+1)
	}
	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotNoiDungMiniApp,
		Table:   "noi_dung_mini_app",
		Filter:  filter,
		Args:    args,
	}, yc, mocNoiDungMiniApp, func(rows *sql.Rows) (domain.NoiDungMiniApp, string, error) {
		n, err := quetNoiDungMiniApp(rows, false)
		if err != nil {
			return domain.NoiDungMiniApp{}, "", err
		}
		return n, n.ID, nil
	})
}

// publicCategorySubtree is the `category=` filter: `danh_muc_id` in the chosen category's LIVE subtree.
// `%[1]d` is the placeholder holding the category id.
//
// BOTH HALVES OF THE RECURSION BIND `tenant_id = $1` AND `deleted_at IS NULL`. The self-referencing key
// is composite with `tenant_id` (0006:190), so a child is provably in its parent's commune — but the
// seed row is chosen by an id THE CLIENT SENT, and without `$1` on it an id of another commune would
// seed the walk. A soft-deleted category cuts its subtree (rule 7, invariant 2), the same edges
// domain.CategoriesWithPublishedItems walks, so a chip never selects items its row did not count.
//
// UNION AND NOT UNION ALL: it de-duplicates, so a cycle — impossible today (0006:223-228), possible the
// day a re-parenting route forgets its ancestor walk — terminates instead of hanging the request.
// Served by `danh_muc_mini_app_cay` (tenant_id, cha_id, thu_tu) WHERE deleted_at IS NULL.
const publicCategorySubtree = ` AND danh_muc_id IN (WITH RECURSIVE subtree(id) AS (` +
	`SELECT c.id FROM danh_muc_mini_app c WHERE c.tenant_id = $1 AND c.id = $%[1]d AND c.deleted_at IS NULL ` +
	`UNION SELECT c.id FROM danh_muc_mini_app c JOIN subtree s ON c.cha_id = s.id ` +
	`WHERE c.tenant_id = $1 AND c.deleted_at IS NULL) SELECT id FROM subtree)`

// publishedCategoryIDs is the read behind GET /api/v1/commune-news/categories: the LIVE categories of
// this commune that hold at least one published, not-deleted item (optionally of one type).
//
// THE JOIN TO THE LIVE CATEGORY TABLE IS WHAT BOUNDS THE RESULT. Without it the distinct ids could name
// soft-deleted categories too, and nothing caps those. With it the answer is a subset of the live tree,
// which TranDanhMucMiniApp caps — and the LIMIT below is that cap plus one, so exceeding it is a refusal
// (ErrQuaNhieuDanhMucMiniApp), never a silently short chip row.
//
// Every joined table is constrained to $1 in its ON clause (store.Scoped.QueryJoin's contract).
const publishedCategoryIDs = `SELECT DISTINCT nd.danh_muc_id FROM noi_dung_mini_app nd ` +
	`JOIN danh_muc_mini_app dm ON dm.tenant_id = $1 AND dm.id = nd.danh_muc_id AND dm.deleted_at IS NULL ` +
	`WHERE nd.tenant_id = $1 AND nd.deleted_at IS NULL AND nd.trang_thai = $2%s LIMIT $%d`

// PublishedCategoryIDs returns the ids of the commune's live categories that hold ≥1 published item
// DIRECTLY (not through a descendant — domain.CategoriesWithPublishedItems adds the ancestors).
// itemType "" = every type but `banner` — the SAME set the default list shows, so a chip never leads to
// a list that is empty because its only items are banners.
func (s *NoiDungMiniAppStore) PublishedCategoryIDs(ctx context.Context, itemType domain.LoaiNoiDung) ([]string, error) {
	args := []any{string(domain.TrangThaiDangHien)}
	var typeClause string
	if itemType != "" {
		args = append(args, string(itemType))
		typeClause = fmt.Sprintf(` AND nd.loai = $%d`, len(args)+1)
	} else {
		args = append(args, string(domain.LoaiBanner))
		typeClause = fmt.Sprintf(` AND nd.loai <> $%d`, len(args)+1)
	}
	args = append(args, TranDanhMucMiniApp+1)
	stmt := fmt.Sprintf(publishedCategoryIDs, typeClause, len(args)+1)

	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("noi_dung_mini_app: đọc danh mục có tin công khai: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0, 16)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("noi_dung_mini_app: đọc dòng danh mục có tin: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("noi_dung_mini_app: duyệt danh mục có tin: %w", err)
	}
	if len(ids) > TranDanhMucMiniApp {
		// Dropped, not trimmed — the same refusal DanhMucMiniAppStore.DanhSach makes.
		return nil, ErrQuaNhieuDanhMucMiniApp
	}
	return ids, nil
}

// CongKhaiTheoID reads ONE PUBLISHED item of this commune, body included.
//
// AN ITEM THAT EXISTS BUT IS NOT PUBLISHED IS ErrNoiDungKhongTonTai, exactly like an id of another
// commune or no id at all: the three are one answer outside (rule 4, forbidden #2). Telling a draft
// apart from nothing would let anybody learn which ids a commune is preparing.
func (s *NoiDungMiniAppStore) CongKhaiTheoID(ctx context.Context, id string) (domain.NoiDungMiniApp, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotNoiDungMiniAppChiTiet, "noi_dung_mini_app",
		`AND id = $2 AND deleted_at IS NULL AND trang_thai = $3`, id, string(domain.TrangThaiDangHien))
	if err != nil {
		return domain.NoiDungMiniApp{}, fmt.Errorf("noi_dung_mini_app: đọc bản ghi công khai: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.NoiDungMiniApp{}, fmt.Errorf("noi_dung_mini_app: đọc bản ghi công khai: %w", err)
		}
		return domain.NoiDungMiniApp{}, ErrNoiDungKhongTonTai
	}
	return quetNoiDungMiniApp(rows, true)
}

// incrementPublicViewCount is the ONE writer of `luot_xem` (ADR 0047, row 02/10/2026). One atomic
// statement: the row lock PostgreSQL takes for the UPDATE serialises concurrent readers, so two views
// are two increments, never a lost update — no read-modify-write in Go.
//
// THE SAME PREDICATE AS CongKhaiTheoID — this commune ($1), this id, not soft-deleted, `dang-hien` —
// so a row the public read would not return is never counted, even if it was unpublished between the
// read and this statement. It SETS NOTHING ELSE: not `cap_nhat_luc` (a view is not an edit, and the
// staff register sorts and shows "last updated" by it), not `da_sua_tay` (§10.4's flag means a member
// of staff changed the text). The triggers on this table allow it: noi_dung_mini_app_bat_bien refuses
// only a DECREASE of `luot_xem` (0011:503-507), and the cover/audio/banner/portal-category triggers
// fire only on UPDATE OF their own columns (0011:452, 0012:232,274, 0013:462).
const incrementPublicViewCount = `UPDATE noi_dung_mini_app SET luot_xem = luot_xem + 1 ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND trang_thai = $3 RETURNING luot_xem`

// IncrementPublicViewCount adds one view to a PUBLISHED item of this commune and returns the new count.
// ErrNoiDungKhongTonTai when the predicate matches no row (another commune's id, not published,
// soft-deleted, no such id) — nothing was written then.
//
// NO AUDIT ENTRY, BY THE OWNER'S EXCEPTION (ADR 0047, row 02/10/2026): a view is a resident reading
// what the commune published, not an act on a record; one audit row per read would make the trail a
// read log of the notice board. The commune comes from ctx (rule 1, invariant 4) through the scoped
// transaction, never from an argument.
func (s *NoiDungMiniAppStore) IncrementPublicViewCount(ctx context.Context, id string) (int, error) {
	var count int
	err := s.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return tx.Underlying().QueryRowContext(ctx, incrementPublicViewCount,
			string(tx.TenantID()), id, string(domain.TrangThaiDangHien)).Scan(&count)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNoiDungKhongTonTai
	}
	if err != nil {
		return 0, fmt.Errorf("noi_dung_mini_app: tăng lượt xem: %w", err)
	}
	return count, nil
}

// PublicBannerStripMax bounds the banner strip. Not a customer figure: it is page.MaxLimit, the bound
// every public list here already has. A commune with more published banners than this gets the first
// PublicBannerStripMax in display order, and the handler logs that it happened.
const PublicBannerStripMax = page.MaxLimit

// PublicBanners reads the commune's banner strip (ADR 0067 §5): PUBLISHED, not-deleted `banner` items
// WITH A COVER, in `display_order` ASC NULLS LAST, then `id` — migration 0012's order and its
// `noi_dung_mini_app_banner_strip` index. A legacy banner with no cover (0012 lets it exist) is skipped
// here, never drawn as an empty slot. Returns at most limit rows, plus one more when there are more —
// the caller trims and logs.
func (s *NoiDungMiniAppStore) PublicBanners(ctx context.Context, limit int) ([]domain.NoiDungMiniApp, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotNoiDungMiniApp, "noi_dung_mini_app",
		`AND deleted_at IS NULL AND trang_thai = $2 AND loai = $3 AND cover_image_file_id IS NOT NULL `+
			`ORDER BY display_order ASC NULLS LAST, id LIMIT $4`,
		string(domain.TrangThaiDangHien), string(domain.LoaiBanner), limit+1)
	if err != nil {
		return nil, fmt.Errorf("noi_dung_mini_app: đọc dải banner: %w", err)
	}
	defer rows.Close()
	var out []domain.NoiDungMiniApp
	for rows.Next() {
		n, err := quetNoiDungMiniApp(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("noi_dung_mini_app: duyệt dải banner: %w", err)
	}
	return out, nil
}
