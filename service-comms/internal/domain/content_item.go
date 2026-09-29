package domain

// MINI APP CONTENT (`docs/ui-ux/11-noi-dung-mini-app.md`) — what a commune publishes to its own
// residents inside the Zalo Mini App.
//
// READ THIS BEFORE REACHING FOR A NAME IN THIS PACKAGE. Three files here now model something a
// Vietnamese speaker would call `thông báo`, and they are three different things with three
// different audiences (kb/00-foundation/ubiquitous-language.md:151):
//
//	announcement.go            a commune tells its OWN STAFF something. Table `thong_bao`.
//	citizen_notification.go    a message LEAVES the commune to ONE CITIZEN. Table `thong_bao_gui_cong_dan`.
//	this file                  an ARTICLE on a public channel, readable by anybody who opens the
//	                           Mini App — and `thong-bao` is one of its six `loai` values, §5.
//
// Every identifier here therefore carries `Content…` or an unambiguous prefix.
// A name collision resolved by "whichever compiles" is how an internal staff notice ends up on a
// public channel — which on this surface is a disclosure, not a bug.
//
// WHAT IS MODELLED HERE AND WHAT IS NOT. The types below carry an item of content as a RECORD: what
// it says, which category it sits in, where it came from, whether residents can see it. They do NOT
// carry the portal synchronisation of §3 and §10 — there is no `core/crypto` to hold the portal's
// API key (ADR 0009, decision 7), no scheduler for §3's `Mỗi 6 giờ`, and no outbound HTTP adapter.
// Migration 0006 names all three absences in full. What survives from that half is PROVENANCE, which
// is a property of the row rather than of the job: `Source`, `SourceURL`, `SourceRef`, `HandEdited`.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// ContentType is one of §5's six tabs. The codes are §5's own `Mã` column, character for character —
// they were read, not translated.
//
// THE LIST IS CLOSED AND THE CHECK CONSTRAINT IN MIGRATION 0006 ADMITS NO SEVENTH. A commune cannot
// add a type: §5 is a fixed set of six tabs in the interface, and a seventh value would be a tab no
// screen draws, holding rows nobody can find.
type ContentType string

const (
	ContentTypeNews      ContentType = "tin-tuc"      // bài viết thường
	ContentTypeEvent     ContentType = "su-kien"      // có ngày diễn ra
	ContentTypeNotice    ContentType = "thong-bao"    // thông báo cho DÂN — not the internal staff module
	ContentTypeBroadcast ContentType = "truyen-thanh" // bản tin audio của loa xã
	ContentTypeVideo     ContentType = "video"
	ContentTypeBanner    ContentType = "banner" // ảnh quảng bá trên đầu Mini App
)

// IsValidContentType reports whether a string is one of the six.
//
// A SWITCH AND NOT A MAP, deliberately: a map would be package-level mutable state, and the one
// thing worse than an unknown `loai` is an unknown `loai` that some other file added at runtime.
func IsValidContentType(t string) bool {
	switch ContentType(t) {
	case ContentTypeNews, ContentTypeEvent, ContentTypeNotice, ContentTypeBroadcast, ContentTypeVideo, ContentTypeBanner:
		return true
	}
	return false
}

// ContentStatus is §6's chip.
//
// `ContentStatusPending` HAS NO WRITER IN THIS REPOSITORY and that is deliberate rather than
// unfinished: §10.2 puts an item here only when the PORTAL SYNC runs in `chờ duyệt` mode, and the
// sync is not built. The value exists because §6 draws the chip, and because leaving it out of the
// schema would make the sync's first pass a migration on live data.
type ContentStatus string

const (
	// ContentStatusHidden — composed and not published. §7: "Chưa bật 'Đăng lên Mini App' thì bà con chưa
	// thấy — soạn trước, đăng sau được." This is the state a new item is born in.
	ContentStatusHidden ContentStatus = "an"
	// ContentStatusPending — waiting for approval. Sync only (§10.2).
	ContentStatusPending ContentStatus = "cho-duyet"
	// ContentStatusVisible — live on the Mini App.
	ContentStatusVisible ContentStatus = "dang-hien"
)

// ContentSource is §8's `nguon`: how this row came to exist. IT IS NEVER TAKEN FROM A REQUEST — see
// CreateContentItemRequest — and migration 0006 makes it immutable once written.
type ContentSource string

const (
	ContentSourceManual     ContentSource = "thu-cong"     // soạn tay trong ViGov
	ContentSourcePortalSync ContentSource = "dong-bo-cong" // đồng bộ từ Cổng thông tin điện tử của xã
)

// The bounds. Every one of them is a REFUSAL rather than a truncation: an article silently cut is an
// article whose meaning changed on the way into the database, on a channel residents read.
//
// WHY THEY ARE NOT `AnnouncementTitleMaxLen` AND `AnnouncementBodyMaxLen` FROM announcement.go, although two
// of the four numbers coincide: those bound an INTERNAL NOTICE typed by one member of staff into a
// textarea. These bound a PUBLISHED ARTICLE, most of which will arrive from a portal that has its
// own limits, and the day one of the two has to move it must move alone. Sharing the constant would
// couple a staff memo's length to a commune's news.
const (
	// ContentTitleMaxLen — §6 prints the title in bold on one line and §7 collects it in a text
	// input. 300 runes is roughly ten times any real headline.
	ContentTitleMaxLen = 300

	// ContentSummaryMaxLen — §6's second line under the title, "tóm tắt cắt 1 dòng". 2.000 runes is
	// far past a summary and short of an article.
	ContentSummaryMaxLen = 2000

	// ContentBodyMaxLen — §7's body, HTML per §8. 200.000 runes is about a hundred pages: past that
	// it is a document with attachments, which this chapter does not have.
	ContentBodyMaxLen = 200000

	// ContentURLMaxLen bounds `anh_dai_dien_url` and `nguon_url`. 2.000 is the length past which a
	// URL stops being one in every browser anybody uses.
	ContentURLMaxLen = 2000

	// CategoryNameMaxLen — §7's `Danh mục` select shows this. Same order as a catalogue label.
	CategoryNameMaxLen = 200

	// CategorySlugMaxLen bounds the category's business code.
	CategorySlugMaxLen = 64

	// CategorySortOrderMax bounds the display order of a category.
	CategorySortOrderMax = 9999
)

var (
	// ErrInvalidContentType — a `loai` outside §5's six. Refused HERE as well as by the CHECK
	// constraint, because the constraint answers with a PostgreSQL exception and this answers with a
	// sentence naming the field.
	ErrInvalidContentType = errors.New("noi_dung_mini_app: `type` không phải một trong sáu loại nội dung")

	// ErrContentTitleEmpty / ErrContentTitleTooLong — §7 marks the title required. An item with no
	// title is a row nobody can pick out of §6's table.
	ErrContentTitleEmpty   = errors.New("noi_dung_mini_app: thiếu `title`")
	ErrContentTitleTooLong = errors.New("noi_dung_mini_app: `title` quá dài")

	ErrContentSummaryTooLong = errors.New("noi_dung_mini_app: `summary` quá dài")
	ErrContentBodyTooLong    = errors.New("noi_dung_mini_app: `body` quá dài")

	// ErrInvalidURL — a URL that is not http(s), or is longer than any real one.
	//
	// THE SCHEME CHECK IS THE POINT AND IT IS NOT COSMETIC. `anh_dai_dien_url` is rendered as the
	// `src` of an image and `nguon_url` as an `href`, both inside an application residents open on
	// their phones. `javascript:…` in an href is script execution on the citizen channel, and
	// `data:…` is an arbitrary payload served from the commune's own screen. An allowlist of two
	// schemes is the only form of this check that cannot be worked around by spelling.
	ErrInvalidURL = errors.New("noi_dung_mini_app: địa chỉ phải bắt đầu bằng http:// hoặc https://")
	ErrURLTooLong = errors.New("noi_dung_mini_app: địa chỉ quá dài")

	// ErrCategoryNameEmpty / ErrCategoryNameTooLong — §8 gives the category a `ten`, and a category with
	// no name is an empty line in §7's select.
	ErrCategoryNameEmpty   = errors.New("danh_muc_mini_app: thiếu `name`")
	ErrCategoryNameTooLong = errors.New("danh_muc_mini_app: `name` quá dài")

	// ErrCategorySlugEmpty / ErrCategorySlugShape / ErrCategorySlugTooLong — the category's
	// business code, in the form ADR 0011 fixes.
	ErrCategorySlugEmpty = errors.New("danh_muc_mini_app: thiếu `slug`")
	ErrCategorySlugShape = errors.New(
		"danh_muc_mini_app: `slug` chỉ gồm chữ thường a-z, số và dấu gạch ngang, ví dụ `chuyen-doi-so`")
	ErrCategorySlugTooLong = errors.New("danh_muc_mini_app: `slug` quá dài")

	// ErrCategorySortOrderOutOfRange — the display order of a category.
	ErrCategorySortOrderOutOfRange = errors.New("danh_muc_mini_app: `order` ngoài khoảng cho phép")

	// ErrCategoryOwnParent — §8's tree. Refused here so the caller gets a sentence rather than the
	// CHECK constraint's exception.
	ErrCategoryOwnParent = errors.New("danh_muc_mini_app: một danh mục không thể là cha của chính nó")
)

// NormalizeContentTitle trims and validates an article title.
//
// IT CARRIES VIETNAMESE WITH DIACRITICS — it is a headline a resident reads — so nothing here
// restricts the character set beyond refusing control characters, which corrupt a screen and a log
// line alike.
func NormalizeContentTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	switch {
	case title == "":
		return "", ErrContentTitleEmpty
	case len([]rune(title)) > ContentTitleMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrContentTitleTooLong, ContentTitleMaxLen)
	}
	for _, r := range title {
		if unicode.IsControl(r) {
			return "", ErrContentTitleEmpty
		}
	}
	return title, nil
}

// NormalizeLongText trims and bounds a multi-line field — the summary and the body.
//
// NEWLINES, CARRIAGE RETURNS AND TABS ARE ALLOWED and every other control character is not. A
// browser on Windows sends CRLF, and refusing it would reject an ordinary form submission.
//
// ⚠ IT DOES NOT SANITISE HTML, AND THAT LIMIT IS WRITTEN HERE RATHER THAN LEFT TO BE ASSUMED. §8
// says `noi_dung` is HTML and §7 wants a rich-text editor, so the body is STORED AS GIVEN. There is
// no HTML sanitiser in this repository, and writing one from scratch is how sanitisers get written
// wrongly. What follows, stated so it is a known risk rather than a surprise: a member of staff with
// `content.update` can put arbitrary markup — including script — into something every resident of
// the commune opens. The permission is the only control today. The fix is a vetted sanitiser at the
// point the Mini App renders it, or at this boundary, and it is a decision with an owner rather than
// a line somebody adds here.
func NormalizeLongText(s string, limit int, tooLong error) (string, error) {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > limit {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", tooLong, limit)
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", fmt.Errorf("%w", tooLong)
		}
	}
	return s, nil
}

// NormalizeURL trims and validates a link. AN EMPTY STRING IS VALID and means "no link".
//
// See ErrInvalidURL for why the scheme allowlist is not cosmetic.
func NormalizeURL(u string) (string, error) {
	u = strings.TrimSpace(u)
	if u == "" {
		return "", nil
	}
	if len(u) > ContentURLMaxLen {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrURLTooLong, ContentURLMaxLen)
	}
	// LOWER-CASED FOR THE COMPARISON ONLY, never for storage: `HTTPS://…` is a valid URL somebody
	// may have pasted, and rewriting what they typed is what rule 7 keeps us from doing to stored
	// values. `url.Parse` is deliberately not used — it accepts a scheme-less string as a relative
	// path, which is exactly the shape this check exists to refuse.
	lower := strings.ToLower(u)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return "", ErrInvalidURL
	}
	for _, r := range u {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", ErrInvalidURL
		}
	}
	return u, nil
}

// NormalizeCategoryName trims and validates a category's display name.
func NormalizeCategoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", ErrCategoryNameEmpty
	case len([]rune(name)) > CategoryNameMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCategoryNameTooLong, CategoryNameMaxLen)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", ErrCategoryNameEmpty
		}
	}
	return name, nil
}

// NormalizeCategorySlug trims and validates a category's business code.
//
// THE RULE IS THE SAME AS NormalizeCode's AND THE FUNCTION IS NOT, deliberately. `NormalizeCode` answers
// with sentences naming `code`, which is the contract field of the map-asset-type catalogue; this
// contract calls the field `slug`, because §8 does. An error message naming a field the client never
// sent is a message that sends somebody looking at the wrong input. The repository already accepts
// this trade explicitly — see the note in three_tier_catalogue.go on why the same file exists in five
// services.
//
// NO CASE FOLDING. `Chuyen-Doi-So` is REFUSED rather than lower-cased: silently changing a code the
// person typed means the code stored is not the code they saw, and rule 7, invariant 3 never lets us
// correct it afterwards.
func NormalizeCategorySlug(slug string) (string, error) {
	slug = strings.TrimSpace(slug)
	switch {
	case slug == "":
		return "", ErrCategorySlugEmpty
	case len(slug) > CategorySlugMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrCategorySlugTooLong, CategorySlugMaxLen)
	}
	if slug[0] == '-' || slug[len(slug)-1] == '-' {
		return "", ErrCategorySlugShape
	}
	prevDash := false
	for i := 0; i < len(slug); i++ {
		c := slug[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			prevDash = false
		case c == '-':
			if prevDash {
				// `chuyen--doi-so` reads as one code and sorts as another. Refuse it while it is
				// still a typo rather than after every article is filed under it.
				return "", ErrCategorySlugShape
			}
			prevDash = true
		default:
			return "", ErrCategorySlugShape
		}
	}
	return slug, nil
}

// ValidateCategorySortOrder bounds a category's display order. NEGATIVE IS REFUSED, not clamped — see
// ValidateSortOrder next door for the argument.
func ValidateCategorySortOrder(order int) error {
	if order < 0 || order > CategorySortOrderMax {
		return fmt.Errorf("%w (0..%d)", ErrCategorySortOrderOutOfRange, CategorySortOrderMax)
	}
	return nil
}

// --- the records -------------------------------------------------------------------------------

// ContentCategory is ONE category of the commune's own Mini App filing tree — the `ContentCategory`
// entity of migration 0006.
type ContentCategory struct {
	ID string // ULID, internal

	// Name is a NAME the commune gave a category of its own, so the contract field is `name` and not
	// `label` — kb/00-foundation/ubiquitous-language.md draws that line at the schema, and this
	// column is `ten`, next to `bo_phan.ten`.
	Name string

	// Slug is the business code: the stable handle an audit entry and an export have on this row.
	Slug string

	// ParentID is "" at the root. §3's sample tree is two levels (`Danh mục › Chuyển đổi số`); nothing
	// in the chapter says two is the limit, so the type expresses a tree.
	ParentID string

	SortOrder int
	CreatedAt time.Time
}

// ContentItem is ONE item of content — the `ContentItem` entity of migration 0006.
//
// `attachments` IS NOT A FIELD HERE, although migration 0006 creates the column §8 declares. Nothing
// in this pass writes it, and §6's `Tệp đính kèm` column is `🔗 Có ảnh` / `—`, which is derived from
// `ImageURL` — see HasImage below. A field read from a column no path fills is a field every layer
// above has to carry and no screen can trust.
type ContentItem struct {
	ID string // ULID, internal

	Type ContentType

	// CategoryID is "" for §7's `— Chưa xếp danh mục —`. That is an ordinary state, not a missing
	// value: an article synchronised before the commune built its tree has nowhere to sit.
	CategoryID string

	Title   string
	Summary string
	Body    string // HTML, stored as given — see NormalizeLongText for the sanitisation gap

	ImageURL string

	// PublishedOn is §6's `Ngày đăng`, a DATE. It differs from CreatedAt for an article carried over from
	// the portal with the portal's own publication date.
	PublishedOn time.Time

	// ViewCount is §6's `👁 {n}`. NOTHING INCREMENTS IT IN THIS PASS: the only thing that legitimately
	// would is a resident opening the article, and the citizen-facing read of §9 is not built.
	ViewCount int

	Status ContentStatus

	// Source, SourceURL, SourceRef and HandEdited are the provenance half of §8. They describe the
	// ROW rather than the sync job, which is why they are here while the job is not — migration 0006
	// argues it in full.
	Source     ContentSource
	SourceURL  string
	SourceRef  string
	HandEdited bool

	// AuthorCode is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id — rule 6,
	// invariant 8. §8 models it as a uuid; the rule wins, and migration 0006 argues it.
	AuthorCode string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// HasImage answers §6's `Tệp đính kèm` column: `🔗 Có ảnh` when there is an image, `—` when there is
// not.
//
// DERIVED AND NOT STORED. A boolean column beside the URL would be two representations of one fact,
// and the stale one is what a screen would read (rule 9).
func (c ContentItem) HasImage() bool { return c.ImageURL != "" }

// IsVisibleToCitizens reports whether residents can see this item at all.
//
// IT IS NOT `Status == ContentStatusVisible` SPELLED OUT AT EVERY CALL SITE, and that is the point:
// `cho-duyet` and `an` are both invisible to a resident for different reasons, and a place that
// asked the narrow question by hand would eventually ask it wrongly — by testing for `an` alone and
// publishing everything waiting for approval.
func (c ContentItem) IsVisibleToCitizens() bool { return c.Status == ContentStatusVisible }

// --- the requests ------------------------------------------------------------------------------

// CreateContentItemRequest is one new item as the use case receives it — §7's modal, field for field.
//
// THERE IS NO `Status` FIELD AND THERE MUST NEVER BE ONE. §7 collects a CHECKBOX, `Đăng lên Mini
// App`, and the state is decided from it by the use case. A request that could name its own state
// could post an item already marked `cho-duyet` — a state §10.2 reserves for the sync — or publish
// one straight past whatever approval a commune later introduces.
//
// THERE IS NO `Source`, `SourceURL` OR `SourceRef` FIELD EITHER. Provenance decides what may later
// be done to the row: §10.4 protects a hand-edited synced article from the next run, and a client
// that could claim `dong-bo-cong` could take a row it composed out of the commune's own hands. The
// store writes the value as a LITERAL for the same reason.
//
// AND THERE IS NO `AuthorCode`. The author is the PRINCIPAL of the request, taken from the session at
// the handler (rule 6, invariant 8). A field here is a field a body could fill, which is one member
// of staff publishing over a colleague's name.
//
// THERE IS NO `PublishedOn` EITHER, and that one is simply §7: the modal has no date field. The use case
// dates a hand-composed item today. The column is editable in the schema because a future sync must
// carry the portal's publication date, not because a form may set it.
type CreateContentItemRequest struct {
	Type string

	// CategoryID is "" for `— Chưa xếp danh mục —`.
	CategoryID string

	Title    string
	Summary  string
	Body     string
	ImageURL string

	// Publish is §7's checkbox `Đăng lên Mini App`. true -> `dang-hien`, false -> `an`.
	Publish bool
}

// Validate normalises and validates the request, returning the cleaned values.
func (r CreateContentItemRequest) Validate() (CreateContentItemRequest, error) {
	var out CreateContentItemRequest
	var err error

	if !IsValidContentType(r.Type) {
		return CreateContentItemRequest{}, ErrInvalidContentType
	}
	out.Type = r.Type

	if out.Title, err = NormalizeContentTitle(r.Title); err != nil {
		return CreateContentItemRequest{}, err
	}
	if out.Summary, err = NormalizeLongText(r.Summary, ContentSummaryMaxLen, ErrContentSummaryTooLong); err != nil {
		return CreateContentItemRequest{}, err
	}
	if out.Body, err = NormalizeLongText(r.Body, ContentBodyMaxLen, ErrContentBodyTooLong); err != nil {
		return CreateContentItemRequest{}, err
	}
	if out.ImageURL, err = NormalizeURL(r.ImageURL); err != nil {
		return CreateContentItemRequest{}, err
	}
	// The category id is a ULID minted by this service. It is validated for SHAPE only — that it is
	// one token and not a paragraph; whether the category EXISTS is the foreign key's answer, and
	// asking it twice would be a check that can disagree with the constraint underneath it.
	out.CategoryID = strings.TrimSpace(r.CategoryID)
	if len(out.CategoryID) > StaffCodeMaxLen {
		return CreateContentItemRequest{}, fmt.Errorf("%w (tối đa %d ký tự)", ErrStaffCodeTooLong, StaffCodeMaxLen)
	}

	out.Publish = r.Publish
	return out, nil
}

// UpdateContentItemRequest is a PARTIAL edit — §6's `✎` reopening §7's modal. A nil pointer means "leave
// this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. Every field here has a meaningful zero: an empty summary
// is "no summary", an empty category is `— Chưa xếp danh mục —`, and `Publish` false is
// "take it off the Mini App". A struct of plain values cannot tell "the client did not mention this"
// from "the client cleared it", so a screen that edits only the title would silently unpublish the
// article and drop it out of its category.
//
// THE SAME THREE FIELDS ARE ABSENT AS ON THE CREATE, for the same reasons, plus one that only
// applies here: `HandEdited` is not a field. §10.4's flag records that a member of STAFF edited a
// SYNCED item, which is a fact about the act rather than a value in it — the use case sets it, and
// migration 0006 refuses to clear it.
type UpdateContentItemRequest struct {
	Type       *string
	CategoryID *string
	Title      *string
	Summary    *string
	Body       *string
	ImageURL   *string
	Publish    *bool
}

// Validate validates whatever the request actually mentioned, and returns the cleaned values in the
// same shape.
func (r UpdateContentItemRequest) Validate() (UpdateContentItemRequest, error) {
	out := UpdateContentItemRequest{Publish: r.Publish}

	if r.Type != nil {
		if !IsValidContentType(*r.Type) {
			return UpdateContentItemRequest{}, ErrInvalidContentType
		}
		t := *r.Type
		out.Type = &t
	}
	if r.Title != nil {
		v, err := NormalizeContentTitle(*r.Title)
		if err != nil {
			return UpdateContentItemRequest{}, err
		}
		out.Title = &v
	}
	if r.Summary != nil {
		v, err := NormalizeLongText(*r.Summary, ContentSummaryMaxLen, ErrContentSummaryTooLong)
		if err != nil {
			return UpdateContentItemRequest{}, err
		}
		out.Summary = &v
	}
	if r.Body != nil {
		v, err := NormalizeLongText(*r.Body, ContentBodyMaxLen, ErrContentBodyTooLong)
		if err != nil {
			return UpdateContentItemRequest{}, err
		}
		out.Body = &v
	}
	if r.ImageURL != nil {
		v, err := NormalizeURL(*r.ImageURL)
		if err != nil {
			return UpdateContentItemRequest{}, err
		}
		out.ImageURL = &v
	}
	if r.CategoryID != nil {
		v := strings.TrimSpace(*r.CategoryID)
		if len(v) > StaffCodeMaxLen {
			return UpdateContentItemRequest{}, fmt.Errorf("%w (tối đa %d ký tự)", ErrStaffCodeTooLong, StaffCodeMaxLen)
		}
		out.CategoryID = &v
	}
	return out, nil
}

// CreateContentCategoryRequest is one new category — §6's `⊞ Danh mục tin`.
type CreateContentCategoryRequest struct {
	Name      string
	Slug      string
	ParentID  string
	SortOrder int
}

// Validate normalises and validates the request.
func (r CreateContentCategoryRequest) Validate() (CreateContentCategoryRequest, error) {
	var out CreateContentCategoryRequest
	var err error

	if out.Name, err = NormalizeCategoryName(r.Name); err != nil {
		return CreateContentCategoryRequest{}, err
	}
	if out.Slug, err = NormalizeCategorySlug(r.Slug); err != nil {
		return CreateContentCategoryRequest{}, err
	}
	if err := ValidateCategorySortOrder(r.SortOrder); err != nil {
		return CreateContentCategoryRequest{}, err
	}
	out.SortOrder = r.SortOrder

	out.ParentID = strings.TrimSpace(r.ParentID)
	if len(out.ParentID) > StaffCodeMaxLen {
		return CreateContentCategoryRequest{}, fmt.Errorf("%w (tối đa %d ký tự)", ErrStaffCodeTooLong, StaffCodeMaxLen)
	}
	return out, nil
}
