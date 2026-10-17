package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/textfold"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// CitizenDirectoryStore reads and writes `citizen_contact` and `citizen_contact_group` (migration
// 0033) — the commune's citizen contact directory ("Danh bạ người dân", ADR 0097).
//
// THE SAME FOUR PROPERTIES AS CitizenLetterDeadlineRuleStore, for the same reasons:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, bound from the context or the scoped transaction (rule
//     1, invariants 4 and 5). No method takes a commune, so no caller can name another commune's row.
//  2. NOTHING HERE OPENS A TRANSACTION. Every write takes the *store.ScopedTx its audit entry is
//     written in (rule 6, invariant 3), and the timestamps it writes come from the caller, so the
//     row and the entry carry the same instant.
//  3. NO HARD DELETE, no DELETE statement at all (rule 7) — a membership removed is a row kept.
//  4. NOTHING HERE LOGS. Every column but the ids and the timestamps is personal data (rule 3).
//
// WHAT THIS STORE RETURNS UNMASKED: the phone number, on every read. Masking is the API layer's
// (privacy.MaskPhone, ADR 0097 D2); the only path that hands the FULL number to a person is
// ReadPhone, whose caller writes the audit entry in the same transaction.
type CitizenDirectoryStore struct {
	db *store.DB
}

func NewCitizenDirectoryStore(db *store.DB) *CitizenDirectoryStore {
	return &CitizenDirectoryStore{db: db}
}

var (
	// ErrCitizenContactNotFound — no live contact of THIS commune has this id. One answer for an
	// invented id, a removed row and another commune's row (rule 4, forbidden #2).
	ErrCitizenContactNotFound = errors.New("danh bạ người dân: không tìm thấy người dân")

	// ErrCitizenPhoneTaken — the live-unique key refused a second live row with this number (one
	// number, one live row per commune — ADR 0097 D5).
	ErrCitizenPhoneTaken = errors.New("danh bạ người dân: số điện thoại này đã có trong danh bạ")

	// ErrCitizenContactUnitUnknown — the composite foreign key refused the residential unit: not a
	// unit of this commune.
	ErrCitizenContactUnitUnknown = errors.New("danh bạ người dân: thôn / tổ dân phố không thuộc xã này")

	// ErrCitizenDirectoryActorMissing — a write with nobody to name for it (rule 6, invariant 8).
	ErrCitizenDirectoryActorMissing = errors.New("danh bạ người dân: thiếu mã cán bộ thực hiện")

	// ErrCitizenDirectoryInstantMissing — a write with no instant to stamp the row with. The caller
	// owns the clock so the audit entry and the row agree; a zero time would store year 1.
	ErrCitizenDirectoryInstantMissing = errors.New("danh bạ người dân: thiếu thời điểm ghi")
)

// CitizenContactSorts is the closed set of sorts the directory list offers: ONE, `created_at`
// descending.
//
// WHY `created_at` AND NOT THE NAME. core/page puts the sort key into the cursor, and the cursor is
// a URL, an access log and a browser history — a person's name must not travel there (rule 3,
// forbidden #4; core/page package comment). `created_at` is NOT NULL (migration 0033), so the keyset
// `(created_at, id)` is a total order and no row vanishes after page one. Newest first is also what
// the staff register does (owner decision 15/10/2026): whoever just added a person finds them at the
// top. A name sort, if the owner asks for one, is page.KindRef on `full_name_folded` — the cursor
// then carries only the anchor's id — and is one more page.Col here.
var CitizenContactSorts = page.NewAllowlist(page.Desc,
	page.Col("created_at", "created_at", page.KindTime),
)

// citizenContactAnchors binds the allowlist to the anchor reader of each column (store.NewMoc panics
// at wiring time if the two disagree).
var citizenContactAnchors = store.NewMoc(CitizenContactSorts, map[string]func(CitizenContactListItem) page.Key{
	"created_at": func(r CitizenContactListItem) page.Key { return page.TimeKey(r.CreatedAt) },
})

// CitizenContactFilter is the directory list's filter — the body of POST /api/v1/citizen-contacts/
// searches (ADR 0097 D11: the keyword travels in the body, never in a URL).
type CitizenContactFilter struct {
	// Keyword matches the full name and the address accent-insensitively, and — when it holds at
	// least one digit — ANY PART of the phone number (ADR 0097 D11; the risk is stated in §4 and the
	// owner accepted it on 16/10/2026). Not narrowed here: ADR 0097 stop condition #5.
	Keyword string
	// GroupID keeps rows with a live membership of this group.
	GroupID string
	// ResidentialUnitID keeps rows of this residential unit.
	ResidentialUnitID string
	// ActiveOnly drops rows switched off (is_active = false). Soft-deleted rows are never listed.
	ActiveOnly bool
	// HasOwnAppSession, when set, keeps rows whose number is (true) / is not (false) signed in to
	// THIS commune's Mini App — the same predicate as the Zalo column (ADR 0097 §3, invariant 5).
	HasOwnAppSession *bool
}

// CitizenContactListItem is one row of the directory page: what the list screen shows and nothing
// more. Address and note are NOT read by List (ADR 0097 D13; rule 3, invariant 6 — a page must not
// carry fifty home addresses); Get reads them for the detail / edit dialog.
//
// Phone is the FULL national number; the API layer masks it (privacy.MaskPhone).
type CitizenContactListItem struct {
	ID                  string
	FullName            string
	Phone               string
	ResidentialUnitID   string // "" when none
	ResidentialUnitName string // resolved for the page in one query; "" when none or unknown
	Tags                []string
	Source              domain.CitizenContactSource
	IsActive            bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
	// GroupIDs are the live memberships whose group is itself live, in membership order.
	GroupIDs []string
	// HasOwnAppSession is the "Zalo" column (ADR 0097 §3): a live session of THIS commune holds the
	// number. Derived in the page statement; never stored.
	HasOwnAppSession bool
	// Attributes are the stored dynamic attribute values (ADR 0101), retired fields included — the use
	// case keeps only the live fields shown as list columns.
	Attributes map[string]json.RawMessage
}

// citizenContactLiveFilter is the base predicate of every read: soft-deleted rows are excluded
// everywhere (rule 7, invariant 2).
const citizenContactLiveFilter = ` AND deleted_at IS NULL`

// citizenContactPageCols is read BY POSITION in scanCitizenContactListItem. The last column is the
// Zalo flag, computed by ownAppSessionPredicate on the row's own number in identity form
// (domain.CitizenPhoneIdentityForm: leading 0 → 84). `$1` inside it is the commune QueryPage binds.
var citizenContactPageCols = `id, full_name, phone, coalesce(residential_unit_id, ''), ` +
	`array_to_string(tags, ','), source, is_active, created_at, updated_at, attributes::text, ` +
	fmt.Sprintf(ownAppSessionPredicate, `'84' || substr(citizen_contact.phone, 2)`)

func scanCitizenContactListItem(rows *sql.Rows) (CitizenContactListItem, string, error) {
	var r CitizenContactListItem
	var tags, source, attrs string
	if err := rows.Scan(&r.ID, &r.FullName, &r.Phone, &r.ResidentialUnitID, &tags, &source,
		&r.IsActive, &r.CreatedAt, &r.UpdatedAt, &attrs, &r.HasOwnAppSession); err != nil {
		return CitizenContactListItem{}, "", fmt.Errorf("danh bạ người dân: đọc dòng danh sách: %w", err)
	}
	r.Tags = splitStoredTags(tags)
	var err error
	if r.Attributes, err = decodeAttributes(attrs); err != nil {
		return CitizenContactListItem{}, "", err
	}
	r.Source = domain.CitizenContactSource(source)
	return r, r.ID, nil
}

// splitStoredTags reverses `array_to_string(tags, ',')`. Safe because a stored tag never holds a
// comma (domain.NormalizeCitizenContactTags refuses one). Empty in, empty non-nil out.
func splitStoredTags(csv string) []string {
	if csv == "" {
		return []string{}
	}
	return strings.Split(csv, ",")
}

// List reads ONE page of the directory under `f`, newest first, and fills what the page shows from
// two more statements — never one per row (skills/load-data-once): unit names
// (ThonToDanPhoStore.UnitNamesByID) and group ids (GroupIDsOf). The Zalo flag needs none.
//
// THE ZALO FLAG IS A COLUMN OF THE PAGE STATEMENT, not a second fetch: the same predicate text
// (ownAppSessionPredicate) that filters on HasOwnAppSession also fills the column, so "filtered as
// signed in" and "shown as signed in" cannot disagree, and the page is cut from the filtered set —
// a filter applied in Go after the fetch would hand back short pages and a wrong has_more.
//
// THE COMMUNE IS NOT A PARAMETER: QueryPage binds it to $1 from the context. The keyword is bound,
// never concatenated; see citizenContactFilterClause.
func (s *CitizenDirectoryStore) List(ctx context.Context, f CitizenContactFilter, req page.Request) (page.Result[CitizenContactListItem], error) {
	clause, args := citizenContactFilterClause(f)
	// @cross-tenant: ADR 0097 §3 — the page's Zalo column and filter join phien_cong_dan (bound to
	// THIS commune by $1) to dinh_danh_cong_dan (no tenant_id, ADR 0002); one boolean per row of this
	// commune's directory, never an id, never another commune's session.
	kq, err := store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: citizenContactPageCols,
		Table:   "citizen_contact",
		Filter:  clause,
		Args:    args,
	}, req, citizenContactAnchors, scanCitizenContactListItem)
	if err != nil {
		return page.NewResult[CitizenContactListItem](), fmt.Errorf("danh bạ người dân: đọc trang: %w", err)
	}
	if err := s.fillListItems(ctx, kq.Items); err != nil {
		return page.NewResult[CitizenContactListItem](), err
	}
	return kq, nil
}

// ListByIDs reads the LIVE rows of this commune among `ids` — the page the search index chose (ADR 0101
// #7) — in the order of `ids`, filled like a List page. An id with no live row of THIS commune is simply
// absent (removed since it was indexed, or never this commune's): the index is a pointer, PostgreSQL is
// the truth. The commune is $1 (Scoped.Query), so another commune's row can never come back even if an
// index ever held its id.
func (s *CitizenDirectoryStore) ListByIDs(ctx context.Context, ids []string) ([]CitizenContactListItem, error) {
	if len(ids) == 0 {
		return []CitizenContactListItem{}, nil
	}
	// @cross-tenant: ADR 0097 §3 — the Zalo column joins this commune's citizen sessions (see List).
	rows, err := s.db.For(ctx).Query(ctx, citizenContactPageCols, "citizen_contact",
		citizenContactLiveFilter+` AND id = ANY($2)`, ids)
	if err != nil {
		return nil, fmt.Errorf("danh bạ người dân: đọc theo danh sách mã: %w", err)
	}
	defer rows.Close()
	byID := make(map[string]CitizenContactListItem, len(ids))
	for rows.Next() {
		r, _, err := scanCitizenContactListItem(rows)
		if err != nil {
			return nil, err
		}
		byID[r.ID] = r
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh bạ người dân: duyệt theo danh sách mã: %w", err)
	}
	out := make([]CitizenContactListItem, 0, len(byID))
	for _, id := range ids {
		if r, ok := byID[id]; ok {
			out = append(out, r)
		}
	}
	if err := s.fillListItems(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// fillListItems resolves unit names and group ids for a page in two statements (skills/load-data-once).
func (s *CitizenDirectoryStore) fillListItems(ctx context.Context, items []CitizenContactListItem) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	unitIDs := make([]string, 0, len(items))
	seenUnit := map[string]bool{}
	for _, r := range items {
		ids = append(ids, r.ID)
		if r.ResidentialUnitID != "" && !seenUnit[r.ResidentialUnitID] {
			seenUnit[r.ResidentialUnitID] = true
			unitIDs = append(unitIDs, r.ResidentialUnitID)
		}
	}

	// Out-of-use and soft-deleted units INCLUDED: a row that points at a unit the commune has since
	// retired still shows the hamlet it holds (the archival-record reading of UnitNamesByID).
	units, err := NewThonToDanPhoStore(s.db).UnitNamesByID(ctx, unitIDs)
	if err != nil {
		return fmt.Errorf("danh bạ người dân: tra tên thôn / tổ: %w", err)
	}
	unitName := make(map[string]string, len(units))
	for _, u := range units {
		unitName[u.ID] = u.Name
	}
	groups, err := s.GroupIDsOf(ctx, ids)
	if err != nil {
		return err
	}
	for i := range items {
		r := &items[i]
		r.ResidentialUnitName = unitName[r.ResidentialUnitID]
		r.GroupIDs = groups[r.ID]
		if r.GroupIDs == nil {
			r.GroupIDs = []string{}
		}
	}
	return nil
}

// decodeAttributes reads `attributes::text`. A row the CHECK admits is always an object.
func decodeAttributes(text string) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	if text == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		// Not the text: it may hold personal data.
		return nil, errors.New("danh bạ người dân: thuộc tính đã lưu không đọc được")
	}
	return out, nil
}

// encodeAttributes renders the object written to the JSONB column; nil writes '{}'.
func encodeAttributes(a map[string]json.RawMessage) (string, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return "", errors.New("danh bạ người dân: không mã hoá được thuộc tính")
	}
	return string(b), nil
}

// Count counts the rows List would walk under `f` — the separate count route (ADR 0071, ADR 0097 D8).
//
// THE LIST'S OWN PREDICATE, NOT A COPY: citizenContactFilterClause builds both, so a total and the
// list it heads cannot disagree about what a filter means.
func (s *CitizenDirectoryStore) Count(ctx context.Context, f CitizenContactFilter) (int, error) {
	clause, args := citizenContactFilterClause(f)
	// @cross-tenant: ADR 0097 §3 — the HasOwnAppSession filter is the Zalo predicate (see List);
	// a count of this commune's rows only.
	rows, err := s.db.For(ctx).Query(ctx, `count(*)`, "citizen_contact", clause, args...)
	if err != nil {
		return 0, fmt.Errorf("danh bạ người dân: đếm theo bộ lọc: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("danh bạ người dân: đếm theo bộ lọc: %w", err)
		}
		return 0, errors.New("danh bạ người dân: đếm theo bộ lọc: không có dòng kết quả")
	}
	var n int64
	if err := rows.Scan(&n); err != nil {
		return 0, fmt.Errorf("danh bạ người dân: đếm theo bộ lọc: đọc: %w", err)
	}
	return int(n), rows.Err()
}

// citizenContactFilterClause builds the predicate of one directory read and its arguments.
//
// PLACEHOLDERS START AT $2: $1 is the commune, bound by Scoped.Query / QueryPage. No value from `f`
// is ever concatenated into the statement — only placeholder numbers are.
//
// THE KEYWORD, three comparisons on one bound pattern:
//
//	full_name_folded LIKE fold($n)       the STORED fold (migration 0033) against the term folded by
//	                                     the SAME SQL expression (core/textfold: the term is folded by
//	                                     the database, never by Go — `lower()` differs by LC_CTYPE);
//	fold(address) LIKE fold($n)          there is no `address_folded` column (0033), so the address is
//	                                     folded at read time with that expression; no index serves a
//	                                     `LIKE '%…%'` anyway, and the scan is one commune's partition;
//	phone LIKE '%digits%'                only when the term holds a digit — the digits of the term, in
//	                                     order, anywhere in the number (ADR 0097 D11; the owner accepted
//	                                     §4's risk). Digits only, so there is nothing for LIKE to misread.
//
// LIKE's three special characters are escaped (thoatLike) BEFORE the fold: the fold touches letters
// only, so "50%" still finds "50%" and not every value starting with "50".
//
// GroupID is an EXISTS on the membership table that binds `m.tenant_id = $1` ITSELF: Scoped.Query
// binds only the outer relation, and ids alone collide across communes (rule 1). Live memberships
// only, as GroupIDsOf shows them.
func citizenContactFilterClause(f CitizenContactFilter) (string, []any) {
	var b strings.Builder
	var args []any
	bind := func(v any) int { // appends v and returns its placeholder number
		args = append(args, v)
		return len(args) + 1
	}
	b.WriteString(citizenContactLiveFilter)

	if f.ActiveOnly {
		b.WriteString(" AND is_active")
	}
	if f.ResidentialUnitID != "" {
		fmt.Fprintf(&b, " AND residential_unit_id = $%d", bind(f.ResidentialUnitID))
	}
	if f.GroupID != "" {
		fmt.Fprintf(&b, " AND EXISTS (SELECT 1 FROM citizen_contact_group m WHERE m.tenant_id = $1"+
			" AND m.contact_id = citizen_contact.id AND m.group_id = $%d AND m.deleted_at IS NULL)", bind(f.GroupID))
	}
	if term := strings.TrimSpace(f.Keyword); term != "" {
		p := fmt.Sprintf("$%d", bind("%"+thoatLike.Replace(term)+"%"))
		fmt.Fprintf(&b, " AND (full_name_folded LIKE %s OR %s LIKE %s",
			textfold.SQL(p), textfold.SQL("coalesce(address, '')"), textfold.SQL(p))
		if digits := digitsOf(term); digits != "" {
			fmt.Fprintf(&b, " OR phone LIKE $%d", bind("%"+digits+"%"))
		}
		// The e-mail column (ADR 0101 #6) is stored lower-case; the term is lowered by the database.
		fmt.Fprintf(&b, " OR coalesce(email, '') LIKE lower(%s)", p)
		b.WriteString(")")
	}
	if f.HasOwnAppSession != nil {
		pred := fmt.Sprintf(ownAppSessionPredicate, `'84' || substr(citizen_contact.phone, 2)`)
		if *f.HasOwnAppSession {
			b.WriteString(" AND " + pred)
		} else {
			b.WriteString(" AND NOT " + pred)
		}
	}
	return b.String(), args
}

// digitsOf keeps the ASCII digits of the term, in order; "" when there is none.
func digitsOf(term string) string {
	var b strings.Builder
	for i := 0; i < len(term); i++ {
		if term[i] >= '0' && term[i] <= '9' {
			b.WriteByte(term[i])
		}
	}
	return b.String()
}

// citizenContactCols is read BY POSITION in scanCitizenContact — the detail read (Get).
const citizenContactCols = `id, full_name, phone, residential_unit_id, coalesce(address, ''), ` +
	`coalesce(note, ''), array_to_string(tags, ','), source, is_active, household_code, ` +
	`import_batch_id, created_at, updated_at, coalesce(email, ''), attributes::text`

func scanCitizenContact(scan func(...any) error) (domain.CitizenContact, error) {
	var c domain.CitizenContact
	var unit, household, batch sql.NullString
	var tags, source, attrs string
	if err := scan(&c.ID, &c.FullName, &c.Phone, &unit, &c.Address, &c.Note, &tags, &source,
		&c.IsActive, &household, &batch, &c.CreatedAt, &c.UpdatedAt, &c.Email, &attrs); err != nil {
		return domain.CitizenContact{}, err
	}
	c.Tags = splitStoredTags(tags)
	var err error
	if c.Attributes, err = decodeAttributes(attrs); err != nil {
		return domain.CitizenContact{}, err
	}
	c.Source = domain.CitizenContactSource(source)
	if unit.Valid {
		c.ResidentialUnitID = &unit.String
	}
	if household.Valid {
		c.HouseholdCode = &household.String
	}
	if batch.Valid {
		c.ImportBatchID = &batch.String
	}
	return c, nil
}

// Get reads one LIVE contact of this commune. found == false for an invented id, a removed row and
// another commune's row alike (rule 4, forbidden #2).
func (s *CitizenDirectoryStore) Get(ctx context.Context, id string) (domain.CitizenContact, bool, error) {
	if id == "" {
		return domain.CitizenContact{}, false, nil
	}
	rows, err := s.db.For(ctx).Query(ctx, citizenContactCols, "citizen_contact",
		citizenContactLiveFilter+` AND id = $2`, id)
	if err != nil {
		return domain.CitizenContact{}, false, fmt.Errorf("danh bạ người dân: đọc người dân: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CitizenContact{}, false, fmt.Errorf("danh bạ người dân: đọc người dân: %w", err)
		}
		return domain.CitizenContact{}, false, nil
	}
	c, err := scanCitizenContact(rows.Scan)
	if err != nil {
		return domain.CitizenContact{}, false, fmt.Errorf("danh bạ người dân: đọc dòng người dân: %w", err)
	}
	return c, true, rows.Err()
}

// ReadPhone returns the FULL number of one live contact, inside the caller's transaction — the ONE
// read behind "Xem số đầy đủ" (ADR 0097 D2, §1).
//
// THE CALLER MUST WRITE THE AUDIT ENTRY IN THIS SAME TRANSACTION and hand the number out only after
// it commits (rule 6, invariants 3 and 7 — the ordering of app/staff_email_reveal.go:31-33). The
// entry names the field and the key, never the value (rule 6, forbidden #4). A caller that reads
// through here without an entry is a caller ADR 0097 stop condition #2 forbids; this method cannot
// see that, which is why it is the only full-number read outside the list and detail scans and why
// its name says what it does.
//
// found == false is the one answer for an unknown, removed or other-commune id.
func (s *CitizenDirectoryStore) ReadPhone(ctx context.Context, tx *store.ScopedTx, id string) (string, bool, error) {
	if tx == nil {
		return "", false, ErrThieuGiaoDich
	}
	if id == "" {
		return "", false, nil
	}
	// ScopedTx.Query binds tenant_id = $1 from the transaction's commune (rule 1, invariant 5).
	rows, err := tx.Query(ctx, `phone`, "citizen_contact", citizenContactLiveFilter+` AND id = $2`, id)
	if err != nil {
		return "", false, fmt.Errorf("danh bạ người dân: đọc số đầy đủ: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", false, fmt.Errorf("danh bạ người dân: đọc số đầy đủ: %w", err)
		}
		return "", false, nil
	}
	var phone string
	// NEVER LOG phone (rule 3, forbidden #1).
	if err := rows.Scan(&phone); err != nil {
		return "", false, fmt.Errorf("danh bạ người dân: đọc dòng số đầy đủ: %w", err)
	}
	return phone, true, rows.Err()
}

// ReadEmail returns the FULL e-mail of one live contact inside the caller's transaction — the one read
// behind "Xem email đầy đủ" (ADR 0101 #6; the ReadPhone contract: the caller writes the audit entry in
// this transaction and hands the address out only after it commits). "" with found = true: no e-mail.
func (s *CitizenDirectoryStore) ReadEmail(ctx context.Context, tx *store.ScopedTx, id string) (string, bool, error) {
	if tx == nil {
		return "", false, ErrThieuGiaoDich
	}
	if id == "" {
		return "", false, nil
	}
	// ScopedTx.Query binds tenant_id = $1 from the transaction's commune (rule 1, invariant 5).
	rows, err := tx.Query(ctx, `coalesce(email, '')`, "citizen_contact", citizenContactLiveFilter+` AND id = $2`, id)
	if err != nil {
		return "", false, fmt.Errorf("danh bạ người dân: đọc email đầy đủ: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", false, fmt.Errorf("danh bạ người dân: đọc email đầy đủ: %w", err)
		}
		return "", false, nil
	}
	var email string
	// Never written to a log line (rule 3, forbidden #1).
	if err := rows.Scan(&email); err != nil {
		return "", false, fmt.Errorf("danh bạ người dân: đọc dòng email: %w", err)
	}
	return email, true, rows.Err()
}

// LiveIDsAfter reads up to `limit` ids of this commune's LIVE contacts greater than `after`, in id order —
// the walk of the per-commune reindex (ADR 0101 #11). Ids only.
func (s *CitizenDirectoryStore) LiveIDsAfter(ctx context.Context, after string, limit int) ([]string, error) {
	rows, err := s.db.For(ctx).Query(ctx, `id`, "citizen_contact",
		citizenContactLiveFilter+` AND id > $2 ORDER BY id LIMIT $3`, after, limit)
	if err != nil {
		return nil, fmt.Errorf("danh bạ người dân: đọc mã để dựng chỉ mục: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("danh bạ người dân: đọc mã: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh bạ người dân: duyệt mã: %w", err)
	}
	return out, nil
}

// PhoneTaken reports whether a LIVE contact of this commune other than `exceptID` holds `phone`
// (national form). Read inside the transaction so the answer and the write that follows see the
// same rows; the live-unique key is the backstop for the race that remains (ErrCitizenPhoneTaken).
func (s *CitizenDirectoryStore) PhoneTaken(ctx context.Context, tx *store.ScopedTx, phone, exceptID string) (bool, error) {
	if tx == nil {
		return false, ErrThieuGiaoDich
	}
	if phone == "" {
		// "" is stored by nobody (CHECK phone_digits); answering "taken" would refuse every row.
		return false, nil
	}
	// ScopedTx.Query binds tenant_id = $1 from the transaction's commune (rule 1, invariant 5).
	rows, err := tx.Query(ctx, `1`, "citizen_contact",
		citizenContactLiveFilter+` AND phone = $2 AND id <> $3 LIMIT 1`, phone, exceptID)
	if err != nil {
		return false, fmt.Errorf("danh bạ người dân: kiểm số đã có: %w", err)
	}
	defer rows.Close()
	taken := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("danh bạ người dân: kiểm số đã có: %w", err)
	}
	return taken, nil
}

// Insert writes one contact. The id, the instant (CreatedAt) and the source are the caller's; the
// row is live and UpdatedAt = CreatedAt. HouseholdCode is NOT written (reserved, migration 0033).
// No ON CONFLICT: a duplicate number arrives as ErrCitizenPhoneTaken and the use case decides.
func (s *CitizenDirectoryStore) Insert(ctx context.Context, tx *store.ScopedTx, c domain.CitizenContact) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if c.ID == "" {
		return ErrCitizenContactNotFound
	}
	if c.CreatedAt.IsZero() {
		return ErrCitizenDirectoryInstantMissing
	}
	if !c.Source.Valid() {
		return domain.ErrCitizenContactSourceUnknown
	}
	attrs, err := encodeAttributes(c.Attributes)
	if err != nil {
		return err
	}
	const stmt = `INSERT INTO citizen_contact
			(tenant_id, id, full_name, phone, residential_unit_id, address, note, tags, source,
			 is_active, import_batch_id, created_at, updated_at, email, attributes)
		VALUES ($1, $2, $3, $4, $5, nullif($6, ''), nullif($7, ''), $8::text[], $9, $10, $11, $12, $12,
		        nullif($13, ''), $14::jsonb)`
	_, err = tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.FullName, c.Phone,
		nullableRef(c.ResidentialUnitID), c.Address, c.Note, nonNilTags(c.Tags), string(c.Source),
		c.IsActive, nullableRef(c.ImportBatchID), c.CreatedAt, c.Email, attrs)
	if err != nil {
		return fmt.Errorf("danh bạ người dân: chèn người dân: %w", translateCitizenDirectoryWriteError(err))
	}
	return nil
}

// Update rewrites the editable columns of one LIVE contact: full name, phone, residential unit,
// address, note, tags, is_active — and `updated_at` from c.UpdatedAt. NOT source, import_batch_id,
// household_code (reserved) or created_at: how a row entered the directory is fixed at creation.
func (s *CitizenDirectoryStore) Update(ctx context.Context, tx *store.ScopedTx, c domain.CitizenContact) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if c.ID == "" {
		return ErrCitizenContactNotFound
	}
	if c.UpdatedAt.IsZero() {
		return ErrCitizenDirectoryInstantMissing
	}
	attrs, err := encodeAttributes(c.Attributes)
	if err != nil {
		return err
	}
	const stmt = `UPDATE citizen_contact
			SET full_name = $3, phone = $4, residential_unit_id = $5, address = nullif($6, ''),
			    note = nullif($7, ''), tags = $8::text[], is_active = $9, updated_at = $10,
			    email = nullif($11, ''), attributes = $12::jsonb
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), c.ID, c.FullName, c.Phone,
		nullableRef(c.ResidentialUnitID), c.Address, c.Note, nonNilTags(c.Tags), c.IsActive, c.UpdatedAt,
		c.Email, attrs)
	if err != nil {
		return fmt.Errorf("danh bạ người dân: sửa người dân: %w", translateCitizenDirectoryWriteError(err))
	}
	return exactlyOneCitizenContact(res, "sửa người dân")
}

// SoftDelete writes the three columns rule 7 names on one LIVE contact. `by` is the remover's STAFF
// CODE (rule 6, invariant 8); `reason` is MANDATORY (ADR 0097 D7 — the 0033 CHECK refuses a blank
// one; domain.NormalizeCitizenContactDeleteReason refuses it first with a sentence). `deleted_at IS
// NULL` makes a second removal not-found rather than a rewrite of who removed it and why.
//
// MEMBERSHIPS ARE LEFT AS THEY ARE: "was this person in the group when that notice went out" is a
// question only a kept row answers (migration 0033 header); every read of them joins the live
// contact, so a removed person is in no group's count or list.
func (s *CitizenDirectoryStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if id == "" {
		return ErrCitizenContactNotFound
	}
	if strings.TrimSpace(by) == "" {
		return ErrCitizenDirectoryActorMissing
	}
	if strings.TrimSpace(reason) == "" {
		return domain.ErrCitizenDirectoryReasonMissing
	}
	if at.IsZero() {
		return ErrCitizenDirectoryInstantMissing
	}
	const stmt = `UPDATE citizen_contact
			SET deleted_at = $3, deleted_by = $4, delete_reason = $5, updated_at = $3
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, at, by, reason)
	if err != nil {
		return fmt.Errorf("danh bạ người dân: xoá mềm người dân: %w", err)
	}
	return exactlyOneCitizenContact(res, "xoá mềm người dân")
}

// TagCount is one distinct tag of the commune and how many live contacts carry it.
type TagCount struct {
	Tag   string
	Count int
}

// MaxCitizenTagCounts bounds the chip strip: the most-used 200 tags, by count then name. A strip
// cannot show more; a commune with more distinct free labels than that has a tagging problem the
// strip is not the place to surface. Stated here because it IS a truncation, unlike every other
// ceiling of this service — the strip is a navigation aid, not a record.
const MaxCitizenTagCounts = 200

// TagCounts counts, per distinct tag, the LIVE contacts of this commune carrying it — the tag strip
// over the list. Inactive rows are counted (they are listed too); removed rows are not.
//
// ONE statement: `unnest(tags)` as a lateral join on the row's own array. Tags are compared as
// stored (case kept — the domain already collapses case within one row, not across rows).
func (s *CitizenDirectoryStore) TagCounts(ctx context.Context) ([]TagCount, error) {
	const stmt = `SELECT t.tag, count(*)
		FROM citizen_contact c CROSS JOIN LATERAL unnest(c.tags) AS t(tag)
		WHERE c.tenant_id = $1 AND c.deleted_at IS NULL
		GROUP BY t.tag
		ORDER BY count(*) DESC, t.tag
		LIMIT $2`
	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, MaxCitizenTagCounts)
	if err != nil {
		return nil, fmt.Errorf("danh bạ người dân: đếm nhãn: %w", err)
	}
	defer rows.Close()
	out := make([]TagCount, 0, 32)
	for rows.Next() {
		var tc TagCount
		var n int64
		if err := rows.Scan(&tc.Tag, &n); err != nil {
			return nil, fmt.Errorf("danh bạ người dân: đọc dòng nhãn: %w", err)
		}
		tc.Count = int(n)
		out = append(out, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh bạ người dân: duyệt nhãn: %w", err)
	}
	return out, nil
}

// CitizenGroupMembershipRemovedReason is written on every membership row SetGroups soft-removes.
// One fixed sentence: the reason for the CHANGE is on the audit entry the caller writes beside it.
const CitizenGroupMembershipRemovedReason = "Gỡ khỏi nhóm khi sửa nhóm của người dân"

// SetGroups makes `groupIDs` the contact's WHOLE live membership set — the staff↔role shape of
// migration 0031 (SetStaffRoles), in the caller's transaction:
//
//	soft-remove   live memberships not in the set: deleted_at / deleted_by / delete_reason, never a
//	              DELETE (rule 7; data_safety_guard would refuse one on this table anyway).
//	add           groups in the set with no live row, as NEW rows stamped `at` — a person removed and
//	              added back later is a SECOND row, never an edit of the first (rule 7, forbidden #5;
//	              0033 keys created_at for exactly this).
//
// The composite foreign keys refuse another commune's group (ErrCitizenGroupNotFound) and an
// unknown contact (ErrCitizenContactNotFound); a SOFT-DELETED group is accepted by the key, so the
// caller refuses it first (CitizenGroupStore.Get). `by` is the actor's STAFF CODE.
func (s *CitizenDirectoryStore) SetGroups(ctx context.Context, tx *store.ScopedTx, contactID string, groupIDs []string, by string, at time.Time) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if contactID == "" {
		return ErrCitizenContactNotFound
	}
	if strings.TrimSpace(by) == "" {
		return ErrCitizenDirectoryActorMissing
	}
	if at.IsZero() {
		return ErrCitizenDirectoryInstantMissing
	}
	// Deduplicated: `unnest` of a repeated id would insert the same live membership twice and hit
	// the live-unique key.
	set := make([]string, 0, len(groupIDs))
	seen := make(map[string]bool, len(groupIDs))
	for _, g := range groupIDs {
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		set = append(set, g)
	}
	xa := string(tx.TenantID())

	const removeStmt = `UPDATE citizen_contact_group
			SET deleted_at = $5, deleted_by = $4, delete_reason = $6
		WHERE tenant_id = $1 AND contact_id = $2 AND deleted_at IS NULL
		  AND NOT (group_id = ANY($3::text[]))`
	if _, err := tx.Exec(ctx, removeStmt, xa, contactID, set, by, at, CitizenGroupMembershipRemovedReason); err != nil {
		return fmt.Errorf("danh bạ người dân: gỡ khỏi nhóm: %w", translateCitizenDirectoryWriteError(err))
	}

	const addStmt = `INSERT INTO citizen_contact_group (tenant_id, contact_id, group_id, created_at, created_by)
		SELECT $1, $2, g, $5, $4 FROM unnest($3::text[]) AS g
		WHERE NOT EXISTS (SELECT 1 FROM citizen_contact_group cur
		                  WHERE cur.tenant_id = $1 AND cur.contact_id = $2
		                    AND cur.group_id = g AND cur.deleted_at IS NULL)`
	if _, err := tx.Exec(ctx, addStmt, xa, contactID, set, by, at); err != nil {
		return fmt.Errorf("danh bạ người dân: thêm vào nhóm: %w", translateCitizenDirectoryWriteError(err))
	}
	return nil
}

// GroupIDsOf reads the live group ids of several contacts in ONE statement, keyed by contact id, in
// membership order. Only memberships whose GROUP IS LIVE are returned: the screen names groups from
// the live group list, and an id it cannot name would be a chip with no text. A contact with none
// is absent from the map.
func (s *CitizenDirectoryStore) GroupIDsOf(ctx context.Context, contactIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(contactIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.For(ctx).Query(ctx, `contact_id, group_id`, "citizen_contact_group",
		`AND contact_id = ANY($2) AND deleted_at IS NULL`+
			` AND EXISTS (SELECT 1 FROM citizen_group g WHERE g.tenant_id = $1 AND g.id = citizen_contact_group.group_id AND g.deleted_at IS NULL)`+
			` ORDER BY contact_id, created_at, group_id`, contactIDs)
	if err != nil {
		return nil, fmt.Errorf("danh bạ người dân: đọc nhóm theo lô: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var contact, group string
		if err := rows.Scan(&contact, &group); err != nil {
			return nil, fmt.Errorf("danh bạ người dân: đọc dòng nhóm: %w", err)
		}
		out[contact] = append(out[contact], group)
	}
	// Checked: a truncated answer shows a person as in fewer groups than they are.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh bạ người dân: duyệt nhóm theo lô: %w", err)
	}
	return out, nil
}

// MemberCounts counts, per group, the LIVE contacts (inactive included, removed excluded) with a
// live membership — the "Thành viên" figure of the group tab. One statement; a group with nobody is
// absent from the map. The contact join is bound to the SAME commune in its ON clause (rule 1).
func (s *CitizenDirectoryStore) MemberCounts(ctx context.Context) (map[string]int, error) {
	const stmt = `SELECT m.group_id, count(*)
		FROM citizen_contact_group m
		JOIN citizen_contact c ON c.tenant_id = m.tenant_id AND c.id = m.contact_id AND c.deleted_at IS NULL
		WHERE m.tenant_id = $1 AND m.deleted_at IS NULL
		GROUP BY m.group_id`
	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("danh bạ người dân: đếm thành viên nhóm: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var group string
		var n int64
		if err := rows.Scan(&group, &n); err != nil {
			return nil, fmt.Errorf("danh bạ người dân: đọc dòng đếm thành viên: %w", err)
		}
		out[group] = int(n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh bạ người dân: duyệt đếm thành viên: %w", err)
	}
	return out, nil
}

// nullableRef binds an optional id: nil → NULL, "" → NULL (a composite foreign key with one NULL
// column is not checked, which is the meaning of "no unit").
func nullableRef(p *string) any {
	if p == nil || *p == "" {
		return nil
	}
	return *p
}

// nonNilTags makes sure the NOT NULL array column always receives an array.
func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func exactlyOneCitizenContact(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("danh bạ người dân: %s: đếm dòng đã ghi: %w", what, err)
	}
	if n == 0 {
		return ErrCitizenContactNotFound
	}
	return nil
}

// translateCitizenDirectoryWriteError turns a constraint violation into the sentinel the use case
// decides on; an unrecognised error is returned unchanged.
//
// SUBSTRING MATCHES, for translateCitizenLetterRuleWriteError's reason: on a PARTITIONED table
// PostgreSQL reports the PARTITION's index for a unique violation
// (`citizen_contact_p03_tenant_id_phone_live_key_key`), the parent's name for a plain table and for
// foreign keys. Both spellings are matched. `citizen_contact_p` is not a prefix of
// `citizen_contact_group_p`, so the two tables' violations stay apart.
func translateCitizenDirectoryWriteError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "citizen_contact_phone_live_unique"),
		strings.Contains(msg, "citizen_contact_p") && strings.Contains(msg, "phone") && strings.Contains(msg, "duplicate key"):
		return ErrCitizenPhoneTaken
	case strings.Contains(msg, "citizen_contact_residential_unit_fkey"):
		return ErrCitizenContactUnitUnknown
	case strings.Contains(msg, "citizen_contact_group_group_fkey"):
		return ErrCitizenGroupNotFound
	case strings.Contains(msg, "citizen_contact_group_contact_fkey"):
		return ErrCitizenContactNotFound
	case strings.Contains(msg, "citizen_group_name_live_unique"):
		return ErrCitizenGroupNameTaken
	}
	return err
}
