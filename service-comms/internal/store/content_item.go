package store

// THE MINI APP CONTENT REGISTER — `noi_dung_mini_app` and `danh_muc_mini_app` (migration 0006).
// SQL, and nothing else.
//
// EVERY IDENTIFIER HERE CARRIES `ContentItem` OR `ContentCategory`, and that is not a style choice.
// This package already holds `AnnouncementStore` (the internal staff book), `CitizenNotificationStore`
// (the citizen message ledger) and `MapAssetTypeStore` (a reference catalogue). Two of those
// share one Vietnamese word with a VALUE of `noi_dung_mini_app.loai`, and the third is also a
// `danh_muc`. A name reused between them is one refactor away from putting an internal notice on a
// public channel — which on this surface is a disclosure, not a bug.
//
// FOUR THINGS HOLD IN EVERY STATEMENT IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1, from the context (rule 1, invariants 4 and 5). It is never a parameter of
//     any method here.
//  2. NOTHING HERE OPENS A TRANSACTION. The write methods take a *store.ScopedTx, because the audit
//     entry has to share it (rule 6, invariant 3) and a method that opened its own would make that
//     impossible for its caller. internal/app opens it.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) except SlugTaken, which exists
//     precisely to see them — an issued code is never reissued.
//  4. EVERY VALUE IS A BOUND PARAMETER. The filter below generates `$n` placeholders from the
//     COUNT of the filters actually asked for and binds the values; nothing from a request is
//     concatenated into SQL.
//
// NO audit.Write IN THIS FILE, AND THAT IS NOT THE OMISSION IT LOOKS LIKE — audit_guard warns on
// exactly this shape, so the answer is written down rather than rediscovered, and it is the same
// answer the two files next door give. Rule 6 requires the entry to share the TRANSACTION, which it
// does: internal/app opens one, calls the methods below, and writes the entry inside the same
// *store.ScopedTx. What the entry cannot come from is here — `Actor` is who caused the write and
// `Action` is a business verb, and neither exists at the SQL layer.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// --- refusals shared by both tables of this module -----------------------------------------------

var (
	// ErrContentItemNotFound says the id addressed no live item OF THIS COMMUNE.
	//
	// The two causes are deliberately not distinguished: "no such item" and "that item belongs to
	// another commune" must look identical from outside, or the error itself becomes a way to
	// discover what another authority holds (rule 4, forbidden #2, applied to the commune axis).
	ErrContentItemNotFound = errors.New("noi_dung_mini_app: không có bản ghi")

	// ErrCategorySlugTaken — the slug is already taken in this commune, SOFT-DELETED ROWS INCLUDED.
	//
	// Counting deleted rows is not strictness for its own sake: `UNIQUE (tenant_id, slug)` counts
	// them too, on purpose. A commune that could soft-delete `chuyen-doi-so` and create a new,
	// unrelated `chuyen-doi-so` would silently refile every article already filed under the old one.
	// Rule 7, invariant 3: an issued code is never reissued.
	ErrCategorySlugTaken = errors.New("danh_muc_mini_app: slug đã được dùng trong xã này")

	// ErrContentCategoryNotFound — the category the item was filed under is not a live category of
	// this commune. The foreign key refuses it too; this turns the constraint violation into a
	// sentence naming the field.
	ErrContentCategoryNotFound = errors.New("danh_muc_mini_app: không có danh mục này trong xã")

	// ErrTooManyContentCategories — the commune is at the ceiling. See ContentCategoryCeiling.
	ErrTooManyContentCategories = errors.New("danh_muc_mini_app: vượt trần danh mục")
)

// ContentCategoryCeiling is the hard upper bound on one commune's Mini App category tree.
//
// WHY A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED. One process serves 200+ communes, so every list
// route is a shared resource and an unbounded one is forbidden (skills/rest-api-design §5,
// FORBIDDEN #4). The category read returns the WHOLE tree on purpose — a select and a filter need
// every option to be correct at all — so the bound cannot come from a `limit` parameter.
//
// THE SAME FIGURE AS MapAssetTypeCeiling, DELIBERATELY. §3's sample commune has 60 chuyên mục
// on its portal, which is the realistic ceiling of a commune's own tree as well, so 500 is roughly
// eight times it — the point past which the content is no longer a filing tree but an import run
// twice. Two ceilings in one service carrying two different numbers would invite the next reader to
// look for a meaning in the difference that is not there.
const ContentCategoryCeiling = 500

// SearchTermMaxLen bounds §6's `Tìm theo tiêu đề…` box.
//
// THE SEARCH IS `ILIKE '%…%'` AND NO INDEX CAN SERVE IT — see clause below. The bound is therefore
// not about the string, it is about the work: a 4.000-character pattern matched against every live
// title of a commune is a request one client can make and every other commune on the process pays
// for. 200 is longer than any real search somebody types.
const SearchTermMaxLen = 200

// --- the content register ------------------------------------------------------------------------

// ContentItemStore is the only path to `noi_dung_mini_app`.
//
// It is built from *store.DB and reaches the database only through Scoped — there is no field here
// holding a *sql.DB, and adding one would reopen the hole core/store exists to close (rule 1,
// invariant 5).
type ContentItemStore struct {
	db *store.DB
}

func NewContentItemStore(db *store.DB) *ContentItemStore {
	return &ContentItemStore{db: db}
}

// contentItemColumns IS READ BY POSITION in scanContentItem.
//
// `noi_dung` IS DELIBERATELY NOT IN IT, WHICH IS THE OPPOSITE OF WHAT THE ANNOUNCEMENT BOOK DOES
// WITH ITS BODY COLUMN — and the difference is the size, not an inconsistency. An announcement is
// capped at 20.000 runes; an article here is capped at 200.000 (domain.ContentBodyMaxLen), because
// §8 says HTML and a portal article carries markup. page.MaxLimit caps a page at 100, so a list
// carrying the body would have a worst case of twenty million runes in one response. §6's table
// shows a title and a one-line summary and needs neither.
//
// THE BODY IS READ BY THE DETAIL METHOD, AND IT IS APPENDED AT THE TAIL of the column list rather
// than inserted anywhere in it — see contentItemDetailColumns. That is what lets ONE scan function
// serve both reads, so the two cannot drift apart.
//
// `nguon`, `nguon_url` AND `nguon_id_ngoai` ARE THREE ADJACENT NULLABLE-ISH TEXT COLUMNS. A swap
// between any two of them in either this list or the scan produces no error at all: it produces an
// article that claims to come from somewhere it did not, which is exactly the fact §10.4 protects.
// TestPgContentItemColumnsMatchRealSchema asserts this list against the real schema BY NAME.
const contentItemColumns = `id, loai, danh_muc_id, tieu_de, tom_tat, anh_dai_dien_url, ` +
	`ngay_dang, luot_xem, trang_thai, nguon, nguon_url, nguon_id_ngoai, da_sua_tay, ` +
	`nguoi_tao_ma, tao_luc, cap_nhat_luc`

// contentItemDetailColumns adds the body AT THE END. Nowhere else: the shared scan appends one
// destination when it is asked for the body, and an insertion anywhere but the tail would shift every
// column after it, silently.
const contentItemDetailColumns = contentItemColumns + `, noi_dung`

// ContentItemSorts is the closed set of sorts GET /api/v1/content-items offers.
//
// ONE COLUMN, AND `ngay_dang` IS DELIBERATELY NOT THE SECOND although §6 prints it. A sort column has
// to be NOT NULL — both are — but it also has to SEPARATE rows, and `ngay_dang` is a DATE: one sync
// run importing four hundred articles published the same day gives four hundred rows the same sort
// value, so the `id` tie-break silently carries the whole order and the list stops being ordered by
// what its header says. `tao_luc` is a timestamp, is NOT NULL, and is indexed
// (`noi_dung_mini_app_so`, migration 0006).
//
// OFFERING BOTH WOULD COST A SECOND INDEX WITH NO READER: §6 states no sort order at all, so a
// second option today is a guess with a write cost attached. Reported rather than approximated.
var ContentItemSorts = page.NewAllowlist(page.Desc,
	page.Col("created_at", "tao_luc", page.KindTime),
)

// contentItemCursor BINDS the allowlisted sort column to the way its cursor value is read out of a
// scanned row. store.NewMoc compares the two lists AT CONSTRUCTION, so a sort added above without a
// reader here is a panic at startup rather than a wrong page order after release.
var contentItemCursor = store.NewMoc[domain.ContentItem](ContentItemSorts,
	map[string]func(domain.ContentItem) page.Key{
		"created_at": func(c domain.ContentItem) page.Key { return page.TimeKey(c.CreatedAt) },
	})

// ContentItemFilter is §6's filter bar: the six tabs, the category select and the title box.
//
// AN EMPTY FIELD MEANS "DO NOT FILTER ON THIS", never "match the empty value". That distinction is
// the whole reason this is a struct of strings rather than three arguments: `Tất cả danh mục` and
// `— Chưa xếp danh mục —` are two different requests, and the second one is NOT expressible here on
// purpose — §6 offers no such option, and inventing one would put a filter on the screen that the
// screen does not have.
type ContentItemFilter struct {
	// Type is one of §5's six codes. A value outside them is refused by the caller before it reaches
	// here; an unknown code would simply match nothing, which reads on screen as an empty tab.
	Type string

	// CategoryID is §6's `Tất cả danh mục ▾`.
	CategoryID string

	// Search is §6's `🔍 Tìm theo tiêu đề…`. Title only — §6 says so, and searching the BODY would put
	// the HTML of every article through a pattern match on every keystroke.
	Search string
}

// likeEscaper escapes the three characters PostgreSQL's LIKE treats specially.
//
// WITHOUT IT, `%` TYPED IN THE SEARCH BOX MATCHES EVERYTHING and `_` matches any character — so a
// resident's name containing an underscore, or a careless paste, silently returns rows the person
// did not ask for. `\` goes first: escaping it after the other two would escape the backslashes this
// function just added. PostgreSQL's default LIKE escape character is `\`, so no ESCAPE clause is
// needed.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// clause builds the filter tail and its arguments.
//
// THE PLACEHOLDERS START AT $2 BECAUSE $1 IS ALWAYS THE COMMUNE — core/store.Scoped.Query owns $1
// and store.QueryPage numbers the caller's own arguments from $2 in the order they appear in
// PageSpec.Args. The counter below is therefore not cosmetic: a mismatch between the numbering here
// and the order of the slice binds the title pattern to the category and returns an empty list for
// every request, with no error anywhere.
//
// `deleted_at IS NULL` IS FIRST AND UNCONDITIONAL (rule 7, invariant 2). Both partial indexes of
// migration 0006 are built on exactly this predicate.
//
// THE TITLE SEARCH CANNOT USE AN INDEX AND THAT IS STATED RATHER THAN HOPED ABOUT: `ILIKE '%x%'` is
// a leading wildcard, so it is a scan of the commune's live rows however the columns are indexed.
// The partition restricts it to one commune and SearchTermMaxLen bounds the pattern; if a commune's
// register ever makes that too slow, the answer is a trigram index or a text-search column in a new
// migration, not a silently truncated result here.
func (f ContentItemFilter) clause() (string, []any) {
	var b strings.Builder
	b.WriteString(` AND deleted_at IS NULL`)

	args := make([]any, 0, 3)
	n := 2 // $1 is the commune
	if f.Type != "" {
		fmt.Fprintf(&b, " AND loai = $%d", n)
		args = append(args, f.Type)
		n++
	}
	if f.CategoryID != "" {
		fmt.Fprintf(&b, " AND danh_muc_id = $%d", n)
		args = append(args, f.CategoryID)
		n++
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		fmt.Fprintf(&b, " AND tieu_de ILIKE $%d", n)
		args = append(args, "%"+likeEscaper.Replace(q)+"%")
		n++
	}
	return b.String(), args
}

// List reads ONE PAGE of §6's table.
//
// ONE STATEMENT AND NO SECOND ONE. Unlike the announcement book next door, nothing on this row is an
// aggregate: §6's `👁 {n}` is a column, and `🔗 Có ảnh` is derived in Go from `anh_dai_dien_url`
// (domain.ContentItem.HasImage). A counter query here would be work with no reader.
//
// THE BODY IS NOT IN THE RESULT. Every item comes back with `Body` empty — see contentItemColumns
// for why, and ByID for where the body is read. A caller that renders the body from a list item
// renders nothing, which is visible; a list that carried it would be a response nobody notices until
// a commune has real articles in it.
func (s *ContentItemStore) List(ctx context.Context, filter ContentItemFilter, req page.Request) (
	page.Result[domain.ContentItem], error) {

	tail, args := filter.clause()

	res, err := store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: contentItemColumns,
		Table:   "noi_dung_mini_app",
		// The commune is bound to $1 by store.QueryPage through Scoped.Query — `tenant_id` is never
		// a value this file supplies.
		Filter: tail,
		Args:   args,
	}, req, contentItemCursor, func(rows *sql.Rows) (domain.ContentItem, string, error) {
		c, err := scanContentItem(rows, false)
		if err != nil {
			return domain.ContentItem{}, "", err
		}
		return c, c.ID, nil
	})
	if err != nil {
		return res, err
	}
	return res, nil
}

// ByID reads ONE live item of THIS COMMUNE, body included — §7's modal reopened by §6's `✎`.
//
// IT IS THE ONLY READ THAT RETURNS `noi_dung`. A detail route exists precisely because the list does
// not carry it (see contentItemColumns), so the two are not two ways of asking one question.
func (s *ContentItemStore) ByID(ctx context.Context, id string) (domain.ContentItem, error) {
	rows, err := s.db.For(ctx).Query(ctx, contentItemDetailColumns, "noi_dung_mini_app",
		`AND id = $2 AND deleted_at IS NULL`, id)
	if err != nil {
		return domain.ContentItem{}, fmt.Errorf("noi_dung_mini_app: đọc bản ghi: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.ContentItem{}, fmt.Errorf("noi_dung_mini_app: đọc bản ghi: %w", err)
		}
		return domain.ContentItem{}, ErrContentItemNotFound
	}
	c, err := scanContentItem(rows, true)
	if err != nil {
		return domain.ContentItem{}, err
	}
	return c, nil
}

// contentRowScanner is what both *sql.Rows and *sql.Row satisfy, so one scan serves every read.
type contentRowScanner interface{ Scan(...any) error }

// scanContentItem reads one row of contentItemColumns, plus the body when it was selected.
//
// POSITIONAL, IN LOCKSTEP WITH contentItemColumns. database/sql binds by POSITION, so a destination
// inserted or removed anywhere but the tail silently shifts every column after it — and the three
// adjacent provenance columns would shift into each other without any error at all.
//
// EVERY NULLABLE COLUMN GOES THROUGH sql.NullString. Scanning a NULL straight into a string is a
// runtime error, and five of these columns are NULL on the commonest row there is: a hand-composed
// article with no category, no summary, no image and no portal origin.
func scanContentItem(r contentRowScanner, withBody bool) (domain.ContentItem, error) {
	var (
		c                        domain.ContentItem
		typ, status, source      string
		category, summary, image sql.NullString
		sourceURL, sourceRef     sql.NullString
		body                     sql.NullString
	)
	dest := []any{
		&c.ID, &typ, &category, &c.Title, &summary, &image,
		&c.PublishedOn, &c.ViewCount, &status, &source, &sourceURL, &sourceRef, &c.HandEdited,
		&c.AuthorCode, &c.CreatedAt, &c.UpdatedAt,
	}
	if withBody {
		dest = append(dest, &body)
	}
	if err := r.Scan(dest...); err != nil {
		return domain.ContentItem{}, fmt.Errorf("noi_dung_mini_app: đọc dòng: %w", err)
	}

	c.Type = domain.ContentType(typ)
	c.Status = domain.ContentStatus(status)
	c.Source = domain.ContentSource(source)
	c.CategoryID = category.String
	c.Summary = summary.String
	c.ImageURL = image.String
	c.SourceURL = sourceURL.String
	c.SourceRef = sourceRef.String
	c.Body = body.String
	return c, nil
}

// --- the content write path ------------------------------------------------------------------------

// ByIDForUpdate reads one live item inside the transaction and holds it until the transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. The edit is a read-decide-write:
// read the row, work out whether §10.4's `da_sua_tay` has to be set, apply. Without the lock two
// members of staff editing the same article both read the old state and the second write silently
// overwrites the first — including the case where one of them was unpublishing it.
//
// IT READS THE BODY TOO, because a partial edit that does not mention `noi_dung` has to write back
// the body that is already there.
func (s *ContentItemStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (
	domain.ContentItem, error) {

	const stmt = `SELECT ` + contentItemDetailColumns + ` FROM noi_dung_mini_app ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	c, err := scanContentItem(
		tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id), true)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ContentItem{}, ErrContentItemNotFound
	}
	if err != nil {
		// scanContentItem already wrapped anything that is not ErrNoRows.
		return domain.ContentItem{}, err
	}
	return c, nil
}

// CategoryExists reports whether the id names a LIVE category of this commune.
//
// THE FOREIGN KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. The constraint never lets an
// article point at a category that does not exist, and it would answer with a driver's exception and
// a 500. This turns the ordinary case — a stale select on a screen somebody left open — into a
// sentence naming the field.
//
// IT EXCLUDES SOFT-DELETED CATEGORIES, WHICH THE FOREIGN KEY DOES NOT. Filing a new article under a
// category the commune retired last week is a mistake the constraint cannot see, because the row is
// still there.
func (s *ContentItemStore) CategoryExists(ctx context.Context, tx *store.ScopedTx, id string) (bool, error) {
	const stmt = `SELECT 1 FROM danh_muc_mini_app ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	var one int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("danh_muc_mini_app: kiểm tra danh mục: %w", err)
	}
	return true, nil
}

// insertContentItem records one item.
//
// `nguon` IS THE LITERAL 'thu-cong' AND `nguon_id_ngoai` / `nguon_url` / `da_sua_tay` APPEAR NOWHERE
// IN THIS STATEMENT. Read that as the property it is, not as a shortcut: there is no $n for any of
// them, so there is no value any layer above could pass and no field a client could fill. Every item
// this service composes is the commune's own. The sync's writer, when it exists, is a DIFFERENT
// statement — and it has to be, because §10.4's protection is exactly the difference between the
// two.
//
// `luot_xem` IS ABSENT TOO: an item is born with nobody having read it, and a parameter there is a
// parameter a request could eventually reach.
//
// `deleted_at`, `deleted_by`, `delete_reason` APPEAR NOWHERE EITHER: a row is not born deleted.
//
// The statement names `tenant_id` first, as every write in this system does: the commune is not an
// argument the caller chooses, it is bound from the transaction (rule 1, invariants 4 and 5).
const insertContentItem = `INSERT INTO noi_dung_mini_app
	(tenant_id, id, loai, danh_muc_id, tieu_de, tom_tat, noi_dung, anh_dai_dien_url,
	 ngay_dang, trang_thai, nguon, nguoi_tao_ma)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'thu-cong',$11)`

// Insert writes one item. The caller owns the transaction and the audit entry inside it.
func (s *ContentItemStore) Insert(ctx context.Context, tx *store.ScopedTx,
	c domain.ContentItem) error {

	// EMPTY STRINGS BECOME NULL, and that is not tidiness: `danh_muc_id` has a foreign key, and ''
	// is not a category id — it is a value the constraint would refuse. The other three are nullable
	// text where '' and NULL would be two spellings of "nothing", and a read path that had to handle
	// both would handle one of them wrongly.
	if _, err := tx.Exec(ctx, insertContentItem, string(tx.TenantID()), c.ID,
		string(c.Type), emptyToNull(c.CategoryID), c.Title, emptyToNull(c.Summary),
		emptyToNull(c.Body), emptyToNull(c.ImageURL), c.PublishedOn.UTC(),
		string(c.Status), c.AuthorCode); err != nil {
		return fmt.Errorf("noi_dung_mini_app: chèn: %w", err)
	}
	return nil
}

// updateContentItem — `nguon`, `nguon_id_ngoai`, `nguoi_tao_ma`, `tao_luc`, `luot_xem` AND THE
// SOFT-DELETE COLUMNS APPEAR NOWHERE IN THIS STATEMENT.
//
// All six are also refused by the trigger in migration 0006, and both layers are meant: the trigger
// is the floor that holds against every writer, and their absence here is what makes the floor
// unreachable from this service in the first place.
//
// `da_sua_tay` IS THE ONE PROVENANCE COLUMN THIS STATEMENT DOES WRITE, and only ever upward — the
// caller computes it as `before.HandEdited OR nguon = 'dong-bo-cong'`, and the trigger refuses to clear
// it. §10.4 is a promise to a member of staff that their correction survives the next sync, and this
// is where the promise is recorded.
const updateContentItem = `UPDATE noi_dung_mini_app
	SET loai = $3, danh_muc_id = $4, tieu_de = $5, tom_tat = $6, noi_dung = $7,
	    anh_dai_dien_url = $8, trang_thai = $9, da_sua_tay = $10, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// Update writes the fields §7's modal may change. The caller has already read the row with
// ByIDForUpdate and merged the partial request onto it.
func (s *ContentItemStore) Update(ctx context.Context, tx *store.ScopedTx,
	c domain.ContentItem) error {

	res, err := tx.Exec(ctx, updateContentItem, string(tx.TenantID()), c.ID,
		string(c.Type), emptyToNull(c.CategoryID), c.Title, emptyToNull(c.Summary),
		emptyToNull(c.Body), emptyToNull(c.ImageURL), string(c.Status), c.HandEdited)
	if err != nil {
		return fmt.Errorf("noi_dung_mini_app: cập nhật: %w", err)
	}
	return expectOneContentRow(res, "cập nhật")
}

// emptyToNull turns "" into a NULL bind. See the note on Insert for why the distinction is load
// bearing on `danh_muc_id` in particular.
func emptyToNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// expectOneContentRow turns "nothing was updated" into ErrContentItemNotFound.
//
// WHY IT IS CHECKED AT ALL WHEN THE CALLER ALREADY READ THE ROW UNDER A LOCK: the read and the write
// are two statements, and a method used without the read — a future caller, a retry path — would
// otherwise report success for a row that does not exist. An UPDATE touching zero rows is not an
// error to PostgreSQL; it is only an error to us.
func expectOneContentRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("noi_dung_mini_app: %s: đọc số dòng: %w", op, err)
	}
	if n == 0 {
		return ErrContentItemNotFound
	}
	return nil
}

// --- the category tree ------------------------------------------------------------------------------

// ContentCategoryStore is the only path to `danh_muc_mini_app`.
type ContentCategoryStore struct {
	db *store.DB
}

func NewContentCategoryStore(db *store.DB) *ContentCategoryStore {
	return &ContentCategoryStore{db: db}
}

// contentCategoryColumns IS READ BY POSITION in scanContentCategory. `ten` and `slug` are adjacent TEXT
// columns and `id` and `cha_id` are two more: swapping either pair compiles, runs, and produces a
// tree whose every node is its own parent or whose every label is a slug.
const contentCategoryColumns = `id, ten, slug, cha_id, thu_tu, tao_luc`

// List reads the commune's WHOLE category tree, ordered.
//
// NOT PAGINATED, AND THAT IS A DECISION. The same three reasons hold as for the map-asset-type
// catalogue next door: it is a closed filing tree of tens of rows, its consumers (§7's select, §6's
// filter) need the whole thing to be correct at all, and a client that must follow cursors to fill a
// dropdown will not. The bound pagination would have provided is provided by ContentCategoryCeiling.
//
// ORDER BY thu_tu, slug: `thu_tu` is the order the commune arranged its own tree in, and `slug`
// breaks ties. The tie-break is the slug rather than the name because `UNIQUE (tenant_id, slug)`
// makes it a TOTAL order — two categories can share a name, and an order that is not total lets two
// calls return the same rows in a different sequence.
//
// THE TREE IS RETURNED FLAT, with `cha_id` on each row. Assembling it is the client's job: §7 draws a
// select and §3 draws an indented list, and the two want different shapes of the same data.
func (s *ContentCategoryStore) List(ctx context.Context) ([]domain.ContentCategory, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page indistinguishable from a complete list
	// of exactly that size — the truncation this route refuses to perform, performed by the bound
	// meant to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, contentCategoryColumns, "danh_muc_mini_app",
		`AND deleted_at IS NULL ORDER BY thu_tu, slug LIMIT $2`, ContentCategoryCeiling+1)
	if err != nil {
		return nil, fmt.Errorf("danh_muc_mini_app: đọc danh mục: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ContentCategory, 0, 16)
	for rows.Next() {
		c, err := scanContentCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh_muc_mini_app: duyệt kết quả: %w", err)
	}
	if len(out) > ContentCategoryCeiling {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list the
		// caller might render anyway is how a refusal turns back into a silent truncation, one
		// careless `if err != nil { log }` later.
		return nil, ErrTooManyContentCategories
	}
	return out, nil
}

func scanContentCategory(r contentRowScanner) (domain.ContentCategory, error) {
	var (
		c      domain.ContentCategory
		parent sql.NullString
	)
	if err := r.Scan(&c.ID, &c.Name, &c.Slug, &parent, &c.SortOrder, &c.CreatedAt); err != nil {
		return domain.ContentCategory{}, fmt.Errorf("danh_muc_mini_app: đọc dòng: %w", err)
	}
	c.ParentID = parent.String
	return c, nil
}

// SlugTaken reports whether this slug is already taken in this commune — INCLUDING soft-deleted
// rows.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two concurrent creates of the same
// slug can both pass this check and the second will then hit `UNIQUE (tenant_id, slug)` and roll the
// whole transaction back — no duplicate row, an unhelpful 500. That is the correct trade: the
// constraint never lets the duplicate exist, and this turns the ordinary case into a sentence
// somebody can act on.
func (s *ContentCategoryStore) SlugTaken(ctx context.Context, tx *store.ScopedTx, slug string) (bool, error) {
	// `deleted_at` DELIBERATELY ABSENT FROM THE PREDICATE. See ErrCategorySlugTaken: an issued code
	// is never reissued (rule 7, invariant 3), so a deleted row still owns its slug.
	const stmt = `SELECT count(*) FROM danh_muc_mini_app WHERE tenant_id = $1 AND slug = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), slug).Scan(&n); err != nil {
		return false, fmt.Errorf("danh_muc_mini_app: kiểm tra slug trùng: %w", err)
	}
	return n > 0, nil
}

// CountLive counts the commune's live categories, for the ceiling check. Soft-deleted rows are
// excluded because List excludes them: the two numbers have to mean the same thing or the check
// guards the wrong quantity.
func (s *ContentCategoryStore) CountLive(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM danh_muc_mini_app WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt).Scan(&n); err != nil {
		return 0, fmt.Errorf("danh_muc_mini_app: đếm danh mục: %w", err)
	}
	return n, nil
}

// ParentExists reports whether the parent id names a LIVE category of this commune.
//
// THE FOREIGN KEY DOES NOT COVER THIS CASE COMPLETELY: it refuses a parent that does not exist, but
// a SOFT-DELETED parent is still a row, so without this check a commune could file a new category
// under one it retired — and §7's select, which excludes deleted rows, would then show a child whose
// parent is nowhere on the screen.
func (s *ContentCategoryStore) ParentExists(ctx context.Context, tx *store.ScopedTx, parentID string) (bool, error) {
	const stmt = `SELECT 1 FROM danh_muc_mini_app ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	var one int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), parentID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("danh_muc_mini_app: kiểm tra danh mục cha: %w", err)
	}
	return true, nil
}

// insertContentCategory records one category.
//
// THE SOFT-DELETE COLUMNS APPEAR NOWHERE: a category is not born deleted.
const insertContentCategory = `INSERT INTO danh_muc_mini_app
	(tenant_id, id, ten, slug, cha_id, thu_tu)
	VALUES ($1,$2,$3,$4,$5,$6)`

// Insert writes one category. The caller owns the transaction and the audit entry inside it.
func (s *ContentCategoryStore) Insert(ctx context.Context, tx *store.ScopedTx,
	c domain.ContentCategory) error {

	if _, err := tx.Exec(ctx, insertContentCategory, string(tx.TenantID()), c.ID,
		c.Name, c.Slug, emptyToNull(c.ParentID), c.SortOrder); err != nil {
		return fmt.Errorf("danh_muc_mini_app: chèn: %w", err)
	}
	return nil
}

// --- what is deliberately absent ---------------------------------------------------------------
//
// NO `SoftDelete` ON EITHER TABLE, AND NO CATEGORY UPDATE. Each absence is a decision:
//
//	deleting an item      §6's action column offers `✎` and NOTHING ELSE. §9 proposes a DELETE, but
//	                      rule 7 makes that a soft delete with a MANDATORY reason (`delete_reason`),
//	                      and no screen in chapter 11 collects one. Building the route would mean
//	                      deciding what the reason says, which is the customer's sentence to write.
//	                      Taking an item off the Mini App is `trang_thai = 'an'`, which the edit
//	                      route already does.
//	editing a category    §6's `⊞ Danh mục tin` button implies management, and §9 lists only
//	                      GET/POST. Renaming is harmless; RE-PARENTING is not — it is the one write
//	                      that can create a cycle through two or more rows, which no CHECK can refuse
//	                      (migration 0006 says so) and which makes every tree walk in this module
//	                      loop forever. That route arrives with its ancestor walk, not before it.
//	deleting a category   a category holding articles cannot simply go: the foreign key refuses it,
//	                      and what should happen to the articles is a question §11 does not answer.
//	the sync writer       see migration 0006. Nothing in this repository can hold the portal's API
//	                      key (ADR 0009, decision 7 — `core/crypto` does not exist), schedule a run,
//	                      or make the call.
//	a citizen read        §9's `/api/cong/mini-app/noi-dung`. See internal/http for the three
//	                      separate reasons it is not built.
