package app

// COMPOSING AND EDITING MINI APP CONTENT — the write half of `docs/ui-ux/11-noi-dung-mini-app.md`.
//
// WHY THIS LAYER EXISTS FOR WHAT LOOKS LIKE ONE INSERT AND ONE UPDATE: rule 6, invariant 3 requires
// the audit entry to share a transaction with the business write, and core/audit.Write takes a
// *store.ScopedTx with no overload that writes outside one. Opening that transaction is this layer's
// job. The handler translates HTTP and nothing else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THAT EVERY WRITE HERE IS READ-DECIDE-WRITE. An edit has to read the row under
// a lock, work out whether §10.4's `da_sua_tay` has to be set, and refuse a category that is not
// there — and that needs the row AND the rules AND the transaction at once, which is exactly one
// place.
//
// NAMES: `Announcements`, `AnnouncementStore` and `MapAssetTypeCatalogue` are already taken in this
// package by the internal announcement book and the map-asset-type catalogue. Everything here
// carries `ContentItem` or `ContentCategory`.
//
// WHAT THIS FILE DELIBERATELY DOES NOT DO: run the portal synchronisation of §3 and §10.1–10.3. It
// cannot, and the reasons are not effort — migration 0006 lists all three (no `core/crypto` to hold
// the commune's portal key, ADR 0009 decision 7; no scheduler; no outbound HTTP adapter). What this
// file DOES carry from that half is §10.4, because that rule is about an EDIT rather than about the
// job: see Update.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// ContentItemStore is the content store, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the row in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry shares
// the transaction, that `nguon` is never written from a request — provable without a PostgreSQL. A
// test that needs infrastructure is a test that stops being run.
type ContentItemStore interface {
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.ContentItem, error)
	CategoryExists(ctx context.Context, tx *store.ScopedTx, id string) (bool, error)
	Insert(ctx context.Context, tx *store.ScopedTx, c domain.ContentItem) error
	Update(ctx context.Context, tx *store.ScopedTx, c domain.ContentItem) error
}

// ContentCategoryStore is the category store. A SECOND INTERFACE rather than four more methods on the
// first, because they guard two different tables with two different refusals — and a caller reaching
// for whichever method was nearest is how an article ends up checked against a category ceiling.
type ContentCategoryStore interface {
	SlugTaken(ctx context.Context, tx *store.ScopedTx, slug string) (bool, error)
	CountLive(ctx context.Context, tx *store.ScopedTx) (int, error)
	ParentExists(ctx context.Context, tx *store.ScopedTx, parentID string) (bool, error)
	Insert(ctx context.Context, tx *store.ScopedTx, c domain.ContentCategory) error
}

// ContentItems owns composing and editing one commune's Mini App content.
type ContentItems struct {
	db         *store.DB
	repo       ContentItemStore
	categories ContentCategoryStore

	// newID and now are injected so a test can pin both. In production they are ulid.Moi and
	// time.Now. The clock is injectable because `ngay_dang` is the date §6 prints, and a test that
	// could not fix it could only assert that it is "roughly today" — the assertion that passes when
	// the value is wrong.
	newID func() (string, error)
	now   func() time.Time
}

func NewContentItems(db *store.DB, repo ContentItemStore, categories ContentCategoryStore) *ContentItems {
	return &ContentItems{db: db, repo: repo, categories: categories, newID: ulid.Moi, now: time.Now}
}

// ContentCategories owns adding categories to one commune's Mini App filing tree.
type ContentCategories struct {
	db   *store.DB
	repo ContentCategoryStore

	newID func() (string, error)
}

func NewContentCategories(db *store.DB, repo ContentCategoryStore) *ContentCategories {
	return &ContentCategories{db: db, repo: repo, newID: ulid.Moi}
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `phat_hanh_thong_bao`): an inspection reads these strings, and
// a function name would tell them nothing.
//
// THE VERB NAMES THE TABLE AS WELL AS THE OPERATION. `them_noi_dung` across a service that also holds
// an announcement book and a catalogue would make the trail unable to answer WHICH register changed
// without joining to a row that may since have been edited again.
const (
	ActionCreateContentItem     = "them_noi_dung_mini_app"
	ActionUpdateContentItem     = "sua_noi_dung_mini_app"
	ActionCreateContentCategory = "them_danh_muc_mini_app"
)

var (
	// ErrMissingContentAuthor refuses the write when the request carries no staff business code.
	//
	// core/audit refuses an entry with no actor for the same reason: an article nobody can be asked
	// about is an article with no author, on a channel where every resident of the commune reads it.
	// There is NO FALLBACK to an internal id — rule 6, invariant 8, and the fallback is what put
	// ULIDs in `audit_log.actor_id` six times on 2026-09-22.
	ErrMissingContentAuthor = errors.New("noi_dung_mini_app: thiếu mã cán bộ soạn")

	// ErrParentCategoryNotFound refuses a category filed under a parent that is not a live category of
	// this commune. The foreign key refuses a parent that does not exist AT ALL; this also catches a
	// SOFT-DELETED parent, which is still a row and which the constraint therefore accepts.
	ErrParentCategoryNotFound = errors.New("danh_muc_mini_app: không có danh mục cha này trong xã")
)

// Create composes one new item of content.
//
// # WHAT IT DOES NOT DO, IN ORDER OF HOW LIKELY SOMEBODY IS TO LOOK FOR IT
//
//	no attachment upload  §7's `Ảnh đại diện · Chọn tệp từ máy · JPG, PNG hoặc WebP — tối đa 50MB`.
//	                      There is no `core/storage` in this repository, so nothing here can accept
//	                      bytes. `anh_dai_dien_url` holds a link to a file some other system serves,
//	                      and §10.6's size and format rules belong to the uploader that does not
//	                      exist yet — writing them here would be validating a request nobody sends.
//	no per-type fields    §7's closing note wants a video URL, an audio file and a duration, an event
//	                      time and place, a banner link and order. It says "nên bổ sung", §8's data
//	                      model carries none of them, and migration 0006 argues why a suggestion does
//	                      not become an applied migration.
//	no approval step      `cho-duyet` is reachable only from the sync (§10.2). A commune composing by
//	                      hand publishes or does not, which is exactly the one checkbox §7 offers.
//
// # THE AUTHOR COMES FROM THE SESSION AND FROM NOWHERE ELSE
//
// `actor.ID` carries the staff BUSINESS CODE — the handler builds it from `authz.Principal.Ma`
// (rule 6, invariant 8). There is no `AuthorCode` field on the request, because a field a body can
// fill is one member of staff publishing over a colleague's name, on a public channel.
func (uc *ContentItems) Create(ctx context.Context, req domain.CreateContentItemRequest,
	actor audit.Actor) (domain.ContentItem, error) {

	// VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never hold a
	// transaction open while doing so, and the caller needs the reason rather than a rollback.
	clean, err := req.Validate()
	if err != nil {
		return domain.ContentItem{}, err
	}
	if actor.ID == "" {
		return domain.ContentItem{}, ErrMissingContentAuthor
	}

	id, err := uc.newID()
	if err != nil {
		return domain.ContentItem{}, fmt.Errorf("noi_dung_mini_app: sinh mã: %w", err)
	}

	// ONE CLOCK READING FOR THE WHOLE ACT, and `ngay_dang` is its DATE part in UTC. §7 collects no
	// date, so a hand-composed item is dated the day it was composed; the column stays editable in
	// the schema only because a future sync must carry the portal's own publication date.
	at := uc.now().UTC()
	publishedOn := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)

	created := domain.ContentItem{
		ID:          id,
		Type:        domain.ContentType(clean.Type),
		CategoryID:  clean.CategoryID,
		Title:       clean.Title,
		Summary:     clean.Summary,
		Body:        clean.Body,
		ImageURL:    clean.ImageURL,
		PublishedOn: publishedOn,
		ViewCount:   0,
		// THE STATE IS DECIDED HERE FROM §7'S CHECKBOX, never taken from the request. See
		// domain.CreateContentItemRequest: a request that could name its own state could publish past
		// whatever approval a commune later introduces, or claim `cho-duyet`, which §10.2 reserves
		// for the sync.
		Status: statusFromPublish(clean.Publish),
		// Set here only so the value this function RETURNS describes the row that was written. The
		// store does not read them: it writes 'thu-cong' as a literal and binds nothing for the other
		// three (migration 0006 and store.insertContentItem both say why).
		Source:     domain.ContentSourceManual,
		HandEdited: false,
		AuthorCode: actor.ID,
		CreatedAt:  at,
		UpdatedAt:  at,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if created.CategoryID != "" {
			found, err := uc.repo.CategoryExists(ctx, tx, created.CategoryID)
			if err != nil {
				return err
			}
			if !found {
				return commsstore.ErrContentCategoryNotFound
			}
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		if err := uc.repo.Insert(ctx, tx, created); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": summarizeContentItem(created)})
		if err != nil {
			return fmt.Errorf("noi_dung_mini_app: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides which
		// commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionCreateContentItem,
			Subject: contentItemSubject(created),
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no item, no trail. The two agree.
		return domain.ContentItem{}, wrapContentErr(ctx, "thêm nội dung Mini App", err)
	}
	return created, nil
}

// Update applies a partial edit — §6's `✎` reopening §7's modal.
//
// # §10.4 IS ENFORCED HERE AND IT IS THE ONE RULE OF THIS FILE WORTH READING TWICE
//
// "Bài do cán bộ sửa tay thì lần đồng bộ sau KHÔNG ghi đè (đặt cờ `da_sua_tay`)." The flag is set by
// THIS act, on any item whose provenance is the portal, and migration 0006 refuses to clear it
// afterwards. It is a promise to a person: somebody corrected a portal article's title, and a sync
// running every six hours would otherwise undo that correction silently, over and over, while the
// person who made it has no way to tell.
//
// IT IS SET AFTER THE NO-OP CHECK, NOT BEFORE. A request that changes nothing is not an edit, so it
// must not claim the protection either — otherwise opening the modal and pressing save without
// touching anything would take an article permanently out of the sync's reach.
//
// # A NO-OP WRITES NOTHING AND AUDITS NOTHING
//
// Sending back the values a row already has is not an event; recording it would fill a public
// authority's ledger with entries saying nothing changed, and those are the entries that bury the
// ones carrying legal weight. It is also what makes this route genuinely idempotent, which is what
// its `idem.KhongCan` declaration claims.
func (uc *ContentItems) Update(ctx context.Context, id string, req domain.UpdateContentItemRequest,
	actor audit.Actor) (domain.ContentItem, error) {

	if id == "" {
		return domain.ContentItem{}, commsstore.ErrContentItemNotFound
	}
	// Shape first, outside the transaction. A request that fails its shape must never hold a row
	// lock while doing so.
	clean, err := req.Validate()
	if err != nil {
		return domain.ContentItem{}, err
	}
	if actor.ID == "" {
		return domain.ContentItem{}, ErrMissingContentAuthor
	}

	var after domain.ContentItem
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}

		after = before
		if clean.Type != nil {
			after.Type = domain.ContentType(*clean.Type)
		}
		if clean.CategoryID != nil {
			after.CategoryID = *clean.CategoryID
		}
		if clean.Title != nil {
			after.Title = *clean.Title
		}
		if clean.Summary != nil {
			after.Summary = *clean.Summary
		}
		if clean.Body != nil {
			after.Body = *clean.Body
		}
		if clean.ImageURL != nil {
			after.ImageURL = *clean.ImageURL
		}
		if clean.Publish != nil {
			// FROM THE CHECKBOX AND NOT FROM A STATE STRING, same as on the create — and with one
			// extra consequence here: an item the sync left in `cho-duyet` becomes `dang-hien` or
			// `an` by a member of staff's explicit act, which is what approving or refusing it IS.
			after.Status = statusFromPublish(*clean.Publish)
		}

		// THE CATEGORY IS CHECKED ONLY WHEN IT MOVED. Re-sending the category a row already has must
		// not fail because that category was retired in the meantime — the article is already filed
		// there, and refusing the save would make every other field uneditable.
		if after.CategoryID != before.CategoryID && after.CategoryID != "" {
			found, err := uc.repo.CategoryExists(ctx, tx, after.CategoryID)
			if err != nil {
				return err
			}
			if !found {
				return commsstore.ErrContentCategoryNotFound
			}
		}

		if contentItemUnchanged(before, after) {
			return nil
		}

		// §10.4 — see the note on this function. AFTER the no-op check, deliberately.
		if before.Source == domain.ContentSourcePortalSync {
			after.HandEdited = true
		}

		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the UPDATE.
		if err := uc.repo.Update(ctx, tx, after); err != nil {
			return err
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to find
		// in a ledger that is never deleted.
		delta, err := json.Marshal(map[string]any{
			"id":    after.ID,
			"truoc": summarizeContentItemChange(before, after, true),
			"sau":   summarizeContentItemChange(before, after, false),
		})
		if err != nil {
			return fmt.Errorf("noi_dung_mini_app: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionUpdateContentItem,
			Subject: contentItemSubject(after),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.ContentItem{}, wrapContentErr(ctx, "sửa nội dung Mini App", err)
	}
	return after, nil
}

// Create adds one category to the commune's filing tree — §6's `⊞ Danh mục tin`.
//
// ORDER OF THE THREE REFUSALS, and it is not arbitrary: the ceiling first (a condition of the whole
// list, which the caller can act on without knowing anything about this value), then the duplicate
// slug, then the parent. The parent last because it is the only one that depends on another row
// being alive.
func (uc *ContentCategories) Create(ctx context.Context, req domain.CreateContentCategoryRequest,
	actor audit.Actor) (domain.ContentCategory, error) {

	clean, err := req.Validate()
	if err != nil {
		return domain.ContentCategory{}, err
	}
	if actor.ID == "" {
		return domain.ContentCategory{}, ErrMissingContentAuthor
	}

	id, err := uc.newID()
	if err != nil {
		return domain.ContentCategory{}, fmt.Errorf("danh_muc_mini_app: sinh mã: %w", err)
	}

	created := domain.ContentCategory{
		ID:        id,
		Name:      clean.Name,
		Slug:      clean.Slug,
		ParentID:  clean.ParentID,
		SortOrder: clean.SortOrder,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.repo.CountLive(ctx, tx)
		if err != nil {
			return err
		}
		if n >= commsstore.ContentCategoryCeiling {
			return commsstore.ErrTooManyContentCategories
		}

		taken, err := uc.repo.SlugTaken(ctx, tx, created.Slug)
		if err != nil {
			return err
		}
		if taken {
			return commsstore.ErrCategorySlugTaken
		}

		if created.ParentID != "" {
			found, err := uc.repo.ParentExists(ctx, tx, created.ParentID)
			if err != nil {
				return err
			}
			if !found {
				return ErrParentCategoryNotFound
			}
			// NO CYCLE CHECK IS NEEDED ON A CREATE AND THAT IS WORTH SAYING OUT LOUD: `created.ID` is a
			// freshly minted ULID, so no existing row can already have it as an ancestor. The day a
			// RE-PARENTING route is written, it must walk up from the proposed parent looking for
			// itself — migration 0006 records the same obligation beside the CHECK that only catches
			// the one-node case.
		}

		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		if err := uc.repo.Insert(ctx, tx, created); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": summarizeContentCategory(created)})
		if err != nil {
			return fmt.Errorf("danh_muc_mini_app: mã hoá delta: %w", err)
		}
		// NOTHING IN THIS DELTA IS PERSONAL DATA (rule 3): a category is how the authority files its
		// own news, not anything about a person. That is why the category's NAME may go into the
		// ledger while an article's TITLE may not — see summarizeContentItem.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:  actor,
			Action: ActionCreateContentCategory,
			// THE BUSINESS CODE, never the internal id (rule 6 and the contract of audit.Entry). A
			// category has one, which is why this subject is a value rather than something composed.
			Subject: created.Slug,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.ContentCategory{}, wrapContentErr(ctx, "thêm danh mục Mini App", err)
	}
	return created, nil
}

// statusFromPublish maps §7's one checkbox onto §6's chip.
//
// A FUNCTION AND NOT AN INLINE TERNARY AT TWO CALL SITES: the two acts — composing and editing —
// have to agree about what the checkbox means, and two inline expressions are two places for them to
// stop agreeing. `cho-duyet` is deliberately unreachable from here (§10.2 gives it to the sync).
func statusFromPublish(publish bool) domain.ContentStatus {
	if publish {
		return domain.ContentStatusVisible
	}
	return domain.ContentStatusHidden
}

// contentItemSubject is the audit subject.
//
// NOT A ULID, for the reason rule 6, invariant 8 gives for `actor_id`: an entry is read years later
// by somebody handling a complaint or an inspection, and a ULID names nothing to them. AN ITEM OF
// CONTENT HAS NO BUSINESS CODE OF ITS OWN — chapter 11 prints no reference number and mints no
// series, and inventing one here would be deciding a numbering scheme for a record the customer has
// not asked to number. A CATEGORY does have one (`slug`), which is why ContentCategories.Create uses
// it directly.
//
// So it is COMPOSED from the two facts that locate the item in §6's table: the tab it sits under and
// the day it was published. The delta carries the id, which is the handle; a subject is a locator
// rather than a key.
//
// THE TITLE IS DELIBERATELY NOT IN IT. `tieu_de` is free text and a commune's news routinely names
// people ("Trao quà cho gia đình ông Nguyễn Văn A…"). The audit ledger is never deleted (rule 6,
// invariant 4), so anything put in it is there permanently — and rule 3, forbidden #5 keeps personal
// data out of exactly this kind of durable record.
func contentItemSubject(c domain.ContentItem) string {
	return "noi-dung-mini-app/" + string(c.Type) + "/" + c.PublishedOn.Format("2006-01-02")
}

// summarizeContentItem is the audit delta's view of one item.
//
// WHAT IS IN IT AND WHY EACH THING IS: the id (the only handle on the record), the type and the
// category (where it was filed), the state (whether residents can see it), and the provenance (which
// decides what a later sync may do to it). WHAT IS NOT IN IT: the title, the summary and the body.
// See contentItemSubject — all three are free text that can name a person, and this ledger is never
// deleted.
//
// THE KEYS ARE THE OLD COLUMN NAMES AND STAY SO: `audit_log.delta` keeps the names it was written
// with (ADR 0061 §Dòng chỉ-thêm).
func summarizeContentItem(c domain.ContentItem) map[string]any {
	return map[string]any{
		"id":          c.ID,
		"loai":        string(c.Type),
		"danh_muc_id": c.CategoryID,
		"trang_thai":  string(c.Status),
		"nguon":       string(c.Source),
		"da_sua_tay":  c.HandEdited,
		"ngay_dang":   c.PublishedOn.Format("2006-01-02"),
		"co_anh":      c.HasImage(),
	}
}

// summarizeContentItemChange returns only the fields that actually moved, from whichever side is asked
// for.
//
// THE THREE TEXT FIELDS ARE REPORTED AS "CHANGED" WITHOUT THEIR VALUES. That is deliberate and is
// the whole difficulty of auditing a CMS: an inspection has to be able to see THAT the body of an
// article was rewritten — otherwise the trail cannot answer the question it exists for — while rule
// 3 keeps the text itself out of a ledger nothing ever deletes. A boolean says the first without the
// second.
func summarizeContentItemChange(before, after domain.ContentItem, side bool) map[string]any {
	out := map[string]any{}
	if before.Type != after.Type {
		out["loai"] = string(pick(side, before.Type, after.Type))
	}
	if before.CategoryID != after.CategoryID {
		out["danh_muc_id"] = pick(side, before.CategoryID, after.CategoryID)
	}
	if before.Status != after.Status {
		out["trang_thai"] = string(pick(side, before.Status, after.Status))
	}
	if before.HandEdited != after.HandEdited {
		out["da_sua_tay"] = pick(side, before.HandEdited, after.HandEdited)
	}
	if before.HasImage() != after.HasImage() {
		out["co_anh"] = pick(side, before.HasImage(), after.HasImage())
	}
	if before.Title != after.Title {
		out["tieu_de_da_doi"] = true
	}
	if before.Summary != after.Summary {
		out["tom_tat_da_doi"] = true
	}
	if before.Body != after.Body {
		out["noi_dung_da_doi"] = true
	}
	return out
}

// summarizeContentCategory is the audit delta's view of one category.
func summarizeContentCategory(c domain.ContentCategory) map[string]any {
	return map[string]any{
		"id":     c.ID,
		"ten":    c.Name,
		"slug":   c.Slug,
		"cha_id": c.ParentID,
		"thu_tu": c.SortOrder,
	}
}

// contentItemUnchanged reports whether the edit would change nothing.
//
// `Source`, `SourceURL`, `SourceRef`, `AuthorCode`, `PublishedOn` AND `ViewCount` ARE NOT COMPARED
// because no path here can change them — they are absent from the UPDATE statement and refused by
// the trigger. `HandEdited` is not compared either, and that one is load bearing: it is DERIVED from
// whether this edit happened, so including it would make every edit of a synced article look like a
// change even when nothing else moved, and the no-op guarantee above would be false.
func contentItemUnchanged(before, after domain.ContentItem) bool {
	return before.Type == after.Type &&
		before.CategoryID == after.CategoryID &&
		before.Title == after.Title &&
		before.Summary == after.Summary &&
		before.Body == after.Body &&
		before.ImageURL == after.ImageURL &&
		before.Status == after.Status
}

// wrapContentErr wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No title, no body, no actor: an error travels into centralised logging across every commune at
// once, and an article title can name a person (rule 3, forbidden #1). The commune is not personal
// data and is the one thing an operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is. A
// %v here would collapse "that category is not in this commune" and "the database is down" into one
// 500.
func wrapContentErr(ctx context.Context, op string, err error) error {
	return fmt.Errorf("noi_dung_mini_app: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}
