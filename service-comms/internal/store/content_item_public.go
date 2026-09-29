package store

// THE PUBLIC READS OF THE MINI APP CONTENT REGISTER — what GET /api/v1/commune-news and
// GET /api/v1/commune-news/{id} serve to anybody who knows a commune's domain (owner decision
// 2026-09-27; docs/ui-ux/11-noi-dung-mini-app.md:189-190).
//
// TWO METHODS NEXT TO List / ByID, NOT A FLAG ON THEM, and the difference is the predicate: the
// staff register lists every state (§6's three chips), the public read lists ONE — `dang-hien`. A
// boolean parameter on the staff read would be one wrong `true` away from publishing every draft and
// everything waiting for approval.
//
// THE STATE IS A BOUND PARAMETER HOLDING domain.ContentStatusVisible, not a literal re-spelt here, so the
// SQL and domain.ContentItem.IsVisibleToCitizens read the same constant. The handler checks IsVisibleToCitizens on
// every row as well — a second wall, not a second definition.
//
// NO NEW INDEX: `noi_dung_mini_app_so` (tenant_id, tao_luc DESC, id) WHERE deleted_at IS NULL serves the
// order and the cursor, and `trang_thai` is filtered on the rows it walks. A commune whose drafts vastly
// outnumber its published items would make the walk longer; a partial index `WHERE trang_thai =
// 'dang-hien'` is the fix then, in a migration, after it is measured.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ListPublic reads ONE PAGE of the commune's PUBLISHED items, newest first (tao_luc DESC, id —
// ContentItemSorts's only column, and the cursor QueryPage pages on).
//
// THE BODY IS NOT IN THE RESULT, for the reason contentItemColumns gives: a page of a hundred articles
// at the body's cap is twenty million runes.
//
// THE COMMUNE IS $1 FROM THE CONTEXT. On the public route the handler put it there after the PLATFORM
// resolved the `host` the caller named; this method cannot tell and does not need to.
//
// itemType "" = every type. Any other value is bound as $3; the handler has already refused a code
// outside the six, and an unknown one here would only match nothing.
func (s *ContentItemStore) ListPublic(ctx context.Context, itemType domain.ContentType, req page.Request) (
	page.Result[domain.ContentItem], error) {

	filter := ` AND deleted_at IS NULL AND trang_thai = $2`
	args := []any{string(domain.ContentStatusVisible)}
	if itemType != "" {
		filter += ` AND loai = $3`
		args = append(args, string(itemType))
	}
	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: contentItemColumns,
		Table:   "noi_dung_mini_app",
		Filter:  filter,
		Args:    args,
	}, req, contentItemCursor, func(rows *sql.Rows) (domain.ContentItem, string, error) {
		c, err := scanContentItem(rows, false)
		if err != nil {
			return domain.ContentItem{}, "", err
		}
		return c, c.ID, nil
	})
}

// PublicByID reads ONE PUBLISHED item of this commune, body included.
//
// AN ITEM THAT EXISTS BUT IS NOT PUBLISHED IS ErrContentItemNotFound, exactly like an id of another
// commune or no id at all: the three are one answer outside (rule 4, forbidden #2). Telling a draft
// apart from nothing would let anybody learn which ids a commune is preparing.
func (s *ContentItemStore) PublicByID(ctx context.Context, id string) (domain.ContentItem, error) {
	rows, err := s.db.For(ctx).Query(ctx, contentItemDetailColumns, "noi_dung_mini_app",
		`AND id = $2 AND deleted_at IS NULL AND trang_thai = $3`, id, string(domain.ContentStatusVisible))
	if err != nil {
		return domain.ContentItem{}, fmt.Errorf("noi_dung_mini_app: đọc bản ghi công khai: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.ContentItem{}, fmt.Errorf("noi_dung_mini_app: đọc bản ghi công khai: %w", err)
		}
		return domain.ContentItem{}, ErrContentItemNotFound
	}
	return scanContentItem(rows, true)
}
