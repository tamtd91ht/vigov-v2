package app

// The use cases of the commune's CITIZEN CONTACT DIRECTORY ("Danh bạ người dân", ADR 0097): the
// list and detail reads, the one audited full-number read, and the three contact writes. Groups are
// citizen_group.go; the Excel import is citizen_directory_import.go. All three files share the one
// CitizenDirectory value below.
//
// WHAT THIS LAYER OWNS, and the three store files do not:
//
//   - THE MASK. Every number leaves this package as privacy.MaskPhone renders it (ADR 0097 D2) —
//     list items and the detail alike. The full number has ONE exit, RevealPhone, and that exit
//     writes its audit entry before the number leaves (rule 6, invariant 7).
//   - THE TRANSACTION. Every write and its audit entry share one *store.ScopedTx (rule 6,
//     invariant 3); the store files refuse a nil one.
//   - THE DELTA. Nothing personal enters the trail (rule 6, forbidden #4): a contact entry names
//     COLUMN NAMES, counts and booleans — never the name, the number, the address or the note. The
//     trail is read years later by people who are not the commune's staff; a trail that quotes the
//     row is a second directory nobody masks.
//
// THE COMMUNE IS NEVER A PARAMETER HERE (rule 1, invariant 4): every store method binds tenant_id
// from the context (store.DB.For(ctx)) or from the transaction (tx.TenantID()) as $1.
//
// THE SUBJECT OF A CONTACT ENTRY IS THE ROW'S ULID. core/audit asks for a business code, and a
// directory row has none (migration 0033 issues no code — the owner wanted a contact list, not a
// register); the one other identifier, the phone number, is personal data and would make the
// subject column a store of numbers. The id is what the detail route is addressed by, so an
// inspection can still open the row.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/search"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The verbs in the trail — Vietnamese snake_case like every other action this system writes
// (ADR 0011; the identifiers are English, rule 12). An inspection reads these strings.
const (
	// ActionRevealCitizenDirectoryPhone — "Xem số đầy đủ" (ADR 0097 §1): a READ, audited because
	// it discloses full personal data (rule 6, invariant 7). The shape of `xem_email_can_bo`. NOT
	// ActionRevealCitizenContactPhone (citizen_contact_phone_reveal.go): that is a citizen session's
	// verified number handed to another service — a different act, a different verb.
	ActionRevealCitizenDirectoryPhone = "xem_so_dien_thoai_nguoi_dan"
	ActionAddCitizenContact           = "them_nguoi_dan_danh_ba"
	ActionEditCitizenContact          = "sua_nguoi_dan_danh_ba"
	ActionRemoveCitizenContact        = "go_nguoi_dan_danh_ba"
	// ActionRevealCitizenDirectoryEmail — "Xem email đầy đủ" (ADR 0101 #6): the phone reveal's shape for
	// the e-mail column, a READ audited because it discloses full personal data (rule 6, invariant 7).
	ActionRevealCitizenDirectoryEmail = "xem_email_nguoi_dan"
)

// EventCitizenContactChanged is the fact every directory write owes the search index (ADR 0101 #11;
// proto/vigov/identity/v1/events.proto). Its topic is events.Topic(ENV, this name).
const EventCitizenContactChanged = "identity.citizen_contact_changed.v1"

// The two operation words of the event payload.
const (
	contactOpUpsert = "upsert"
	contactOpRemove = "remove"
)

// The search refusals (ADR 0101 #7; task card E-1).
var (
	// ErrCitizenSearchUnavailable — the request needs Elasticsearch (attribute or created-date filters, or
	// any search once the index is configured) and the index is off or failed. 503, never a silent
	// PostgreSQL answer with the filters dropped.
	ErrCitizenSearchUnavailable = errors.New("danh bạ người dân: tìm kiếm nâng cao tạm thời không dùng được — vui lòng thử lại sau")
	// ErrCitizenSearchFilterInvalid — a filter naming a field that is not a live attribute of the right
	// type, or an option value it does not define, or an empty range.
	ErrCitizenSearchFilterInvalid = errors.New("danh bạ người dân: bộ lọc thuộc tính không hợp lệ")
	// ErrCitizenSearchCombination — "Chỉ người có Zalo" is a live session join only PostgreSQL can answer;
	// it cannot be combined with the attribute and created-date filters only Elasticsearch can.
	ErrCitizenSearchCombination = errors.New("danh bạ người dân: không lọc \"có Zalo\" cùng lúc với lọc thuộc tính hoặc ngày tạo")
)

// revealCitizenContactPhonePermission is the key the reveal route declares, recorded in the entry so
// an inspection reads under which right the number was opened. The owner chose `citizen.update` on
// 16/10/2026 instead of a separate unmask key (ADR 0097 §1); seeded by migration 0028 — NO KEY WAS
// ADDED (rule 5, invariant 3c).
const revealCitizenContactPhonePermission = "citizen.update"

// CitizenContactRepo is the contact store, declared at the point of use. Every mutating method and
// the two reads that must see the same rows as a write (ReadPhone, PhoneTaken) take the transaction.
// *idstore.CitizenDirectoryStore satisfies it.
type CitizenContactRepo interface {
	List(ctx context.Context, f idstore.CitizenContactFilter, req page.Request) (page.Result[idstore.CitizenContactListItem], error)
	Count(ctx context.Context, f idstore.CitizenContactFilter) (int, error)
	TagCounts(ctx context.Context) ([]idstore.TagCount, error)
	Get(ctx context.Context, id string) (domain.CitizenContact, bool, error)
	GroupIDsOf(ctx context.Context, contactIDs []string) (map[string][]string, error)
	MemberCounts(ctx context.Context) (map[string]int, error)

	ReadPhone(ctx context.Context, tx *store.ScopedTx, id string) (string, bool, error)
	PhoneTaken(ctx context.Context, tx *store.ScopedTx, phone, exceptID string) (bool, error)
	Insert(ctx context.Context, tx *store.ScopedTx, c domain.CitizenContact) error
	Update(ctx context.Context, tx *store.ScopedTx, c domain.CitizenContact) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error
	SetGroups(ctx context.Context, tx *store.ScopedTx, contactID string, groupIDs []string, by string, at time.Time) error

	// ListByIDs hydrates the page the search index chose (ADR 0101 #7); ReadEmail is the audited reveal.
	ListByIDs(ctx context.Context, ids []string) ([]idstore.CitizenContactListItem, error)
	ReadEmail(ctx context.Context, tx *store.ScopedTx, id string) (string, bool, error)
}

// CitizenFieldReader reads the commune's attribute definitions. *idstore.CitizenFieldStore.
type CitizenFieldReader interface {
	List(ctx context.Context, includeRetired bool) ([]domain.CitizenField, error)
}

// EventRecorder writes owed facts inside the act's transaction. *idstore.EventOutboxStore.
type EventRecorder interface {
	Record(ctx context.Context, tx *store.ScopedTx, evs []idstore.OutboxEvent) error
}

// CitizenSearch is the directory's search index. *search.CitizenIndex.
type CitizenSearch interface {
	Ready() error
	Search(ctx context.Context, q search.Query, limit int, after *search.After) (search.Page, error)
	Count(ctx context.Context, q search.Query) (int, error)
}

// CitizenGroupRepo is the group store, declared at the point of use. *idstore.CitizenGroupStore
// satisfies it.
type CitizenGroupRepo interface {
	List(ctx context.Context) ([]domain.CitizenGroup, error)
	Get(ctx context.Context, id string) (domain.CitizenGroup, bool, error)
	IDsByNames(ctx context.Context, names []string) (map[string]string, error)

	NameTaken(ctx context.Context, tx *store.ScopedTx, name, exceptID string) (bool, error)
	Insert(ctx context.Context, tx *store.ScopedTx, g domain.CitizenGroup) error
	Update(ctx context.Context, tx *store.ScopedTx, g domain.CitizenGroup) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error
}

// CitizenDirectory owns the directory's use cases.
type CitizenDirectory struct {
	db       *store.DB
	contacts CitizenContactRepo
	groups   CitizenGroupRepo
	fields   CitizenFieldReader
	outbox   EventRecorder
	// index is nil when Elasticsearch is NOT CONFIGURED (dev): a keyword-only search then runs on
	// PostgreSQL, and the filters only the index can answer are 503. Configured but off (key mismatch,
	// cluster down) is index.Ready() != nil: every search is 503 (owner, 17/10/2026).
	index CitizenSearch

	// Injected so a test can pin them. In production: ulid.Moi and time.Now.
	newID func() (string, error)
	now   func() time.Time
}

// NewCitizenDirectory builds the use case. The outbox is REQUIRED: a directory write that did not record
// its event would leave the search index silently stale (ADR 0101 #11).
func NewCitizenDirectory(db *store.DB, contacts CitizenContactRepo, groups CitizenGroupRepo,
	fields CitizenFieldReader, outbox EventRecorder) *CitizenDirectory {
	if db == nil || contacts == nil || groups == nil || fields == nil || outbox == nil {
		panic("app.NewCitizenDirectory: db, contacts, groups, fields and outbox are required")
	}
	return &CitizenDirectory{
		db: db, contacts: contacts, groups: groups, fields: fields, outbox: outbox,
		newID: ulid.Moi,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// WithSearch attaches the configured search index. Not called when Elasticsearch is not configured.
func (uc *CitizenDirectory) WithSearch(idx CitizenSearch) *CitizenDirectory {
	uc.index = idx
	return uc
}

// contactChangedEvents builds one outbox row per contact id: the opaque id and the operation, nothing
// personal (rule 3; events.proto).
func (uc *CitizenDirectory) contactChangedEvents(ids []string, op string, at time.Time) ([]idstore.OutboxEvent, error) {
	out := make([]idstore.OutboxEvent, 0, len(ids))
	for _, id := range ids {
		evID, err := uc.newID()
		if err != nil {
			return nil, fmt.Errorf("danh bạ người dân: sinh mã sự kiện: %w", err)
		}
		payload, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(&identityv1.CitizenContactChanged{ContactId: id, Op: op})
		if err != nil {
			return nil, fmt.Errorf("danh bạ người dân: mã hoá sự kiện: %w", err)
		}
		out = append(out, idstore.OutboxEvent{ID: evID, Name: EventCitizenContactChanged, Payload: payload, OccurredAt: at})
	}
	return out, nil
}

// recordChanged writes the events of `ids` inside tx — the same transaction as the write (ADR 0101 #11).
func (uc *CitizenDirectory) recordChanged(ctx context.Context, tx *store.ScopedTx, ids []string, op string, at time.Time) error {
	evs, err := uc.contactChangedEvents(ids, op, at)
	if err != nil {
		return err
	}
	return uc.outbox.Record(ctx, tx, evs)
}

// CitizenContactListItem is one row of the directory page AS THE BROWSER MAY SEE IT: the number
// masked, no address, no note (ADR 0097 D13; rule 3, invariant 6). There is no field for the full
// number, so a handler cannot forward one by mistake.
type CitizenContactListItem struct {
	ID                  string
	FullName            string
	PhoneMasked         string
	ResidentialUnitID   string
	ResidentialUnitName string
	Tags                []string
	GroupIDs            []string
	IsActive            bool
	Source              domain.CitizenContactSource
	// HasOwnAppSession is the "Zalo" column (ADR 0097 §3): one boolean, derived by the store.
	HasOwnAppSession bool
	CreatedAt        time.Time
	// Attributes carries ONLY the live fields the commune marked "hiện ở danh sách" (ADR 0101 #1) — the
	// list's columns; a page must not carry every attribute of fifty people (rule 3, invariant 6).
	Attributes map[string]json.RawMessage
}

// CitizenContactDetail is the edit dialog's view of one row: address and note included, the number
// STILL MASKED — the dialog shows the mask until "Xem số đầy đủ" (RevealPhone) is pressed.
type CitizenContactDetail struct {
	ID                string
	FullName          string
	PhoneMasked       string
	ResidentialUnitID string
	Address           string
	Note              string
	Tags              []string
	GroupIDs          []string
	IsActive          bool
	Source            domain.CitizenContactSource
	ImportBatchID     string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	// EmailMasked is privacy.MaskEmail of the e-mail (ADR 0101 #6), "" when none; the full address has
	// its own audited route (RevealEmail), like the number.
	EmailMasked string
	// Attributes are the values of the LIVE fields (the edit form); values of retired fields stay stored
	// and hidden (ADR 0101 #4).
	Attributes map[string]json.RawMessage
}

// CitizenContactRequest is one contact as the add / edit form sends it: the row's fields and the
// WHOLE set of group ids the person belongs to (SetGroups semantics — absent means removed).
//
// Email and Attributes are OPTIONAL ON AN EDIT so the dialog can save without revealing the e-mail first:
// Email nil = keep the stored address, "" = clear it; AttributesSet false = keep every stored value, true =
// Attributes is the whole set of live values (a live code absent is cleared). On a create, absent = none.
type CitizenContactRequest struct {
	Contact       domain.CitizenContactInput
	GroupIDs      []string
	Email         *string
	Attributes    map[string]json.RawMessage
	AttributesSet bool
}

// List reads one page under `f` and masks every number before it leaves.
func (uc *CitizenDirectory) List(ctx context.Context, f idstore.CitizenContactFilter, req page.Request) (page.Result[CitizenContactListItem], error) {
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	kq, err := uc.contacts.List(ctx, f, req)
	if err != nil {
		return page.NewResult[CitizenContactListItem](), err
	}
	items, err := uc.maskListItems(ctx, kq.Items)
	if err != nil {
		return page.NewResult[CitizenContactListItem](), err
	}
	return page.Result[CitizenContactListItem]{Items: items, NextCursor: kq.NextCursor, HasMore: kq.HasMore}, nil
}

// maskListItems renders store rows as the browser may see them: number masked, attributes cut to the
// live list columns. One definition read per page.
func (uc *CitizenDirectory) maskListItems(ctx context.Context, rows []idstore.CitizenContactListItem) ([]CitizenContactListItem, error) {
	out := make([]CitizenContactListItem, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	defs, err := uc.fields.List(ctx, false)
	if err != nil {
		return nil, err
	}
	var columns []domain.CitizenField
	for _, d := range defs {
		if d.ShowInList {
			columns = append(columns, d)
		}
	}
	for _, r := range rows {
		out = append(out, CitizenContactListItem{
			ID: r.ID, FullName: r.FullName, PhoneMasked: privacy.MaskPhone(r.Phone),
			ResidentialUnitID: r.ResidentialUnitID, ResidentialUnitName: r.ResidentialUnitName,
			Tags: nonNilStrings(r.Tags), GroupIDs: nonNilStrings(r.GroupIDs),
			IsActive: r.IsActive, Source: r.Source, HasOwnAppSession: r.HasOwnAppSession, CreatedAt: r.CreatedAt,
			Attributes: domain.LiveAttributes(columns, r.Attributes),
		})
	}
	return out, nil
}

// Count is the separate count route (ADR 0097 D8) — the store's predicate, the list's filter.
func (uc *CitizenDirectory) Count(ctx context.Context, f idstore.CitizenContactFilter) (int, error) {
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	return uc.contacts.Count(ctx, f)
}

// TagCounts is the tag strip over the list.
func (uc *CitizenDirectory) TagCounts(ctx context.Context) ([]idstore.TagCount, error) {
	return uc.contacts.TagCounts(ctx)
}

// Get reads one live contact for the edit dialog. idstore.ErrCitizenContactNotFound for an unknown,
// removed or other-commune id alike (rule 4, forbidden #2).
func (uc *CitizenDirectory) Get(ctx context.Context, id string) (CitizenContactDetail, error) {
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	c, found, err := uc.contacts.Get(ctx, id)
	if err != nil {
		return CitizenContactDetail{}, err
	}
	if !found {
		return CitizenContactDetail{}, idstore.ErrCitizenContactNotFound
	}
	groups, err := uc.contacts.GroupIDsOf(ctx, []string{c.ID})
	if err != nil {
		return CitizenContactDetail{}, err
	}
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	defs, err := uc.fields.List(ctx, false)
	if err != nil {
		return CitizenContactDetail{}, err
	}
	d := CitizenContactDetail{
		ID: c.ID, FullName: c.FullName, PhoneMasked: privacy.MaskPhone(c.Phone),
		ResidentialUnitID: derefString(c.ResidentialUnitID), Address: c.Address, Note: c.Note,
		Tags: nonNilStrings(c.Tags), GroupIDs: nonNilStrings(groups[c.ID]),
		IsActive: c.IsActive, Source: c.Source, ImportBatchID: derefString(c.ImportBatchID),
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		Attributes: domain.LiveAttributes(defs, c.Attributes),
	}
	if c.Email != "" {
		d.EmailMasked = privacy.MaskEmail(c.Email)
	}
	return d, nil
}

// RevealEmail returns the FULL e-mail of one live contact — "Xem email đầy đủ" (ADR 0101 #6). The
// RevealPhone contract: one entry per call, written in the read's transaction, the address handed out only
// after the commit. "" when the row has no e-mail (still one entry: the act of looking is the disclosure).
func (uc *CitizenDirectory) RevealEmail(ctx context.Context, actor NguoiThucHien, id string) (string, error) {
	if err := actor.hopLe(); err != nil {
		return "", err
	}
	if id == "" {
		return "", idstore.ErrCitizenContactNotFound
	}
	delta := deltaSLA(map[string]any{"truong_da_mo": []string{"email"}, "quyen": revealCitizenContactPhonePermission})
	var email string
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		e, found, err := uc.contacts.ReadEmail(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return idstore.ErrCitizenContactNotFound
		}
		// TenantID left unset: audit.Write takes it from the transaction (rule 1, invariant 4).
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor: actor.Vet, Action: ActionRevealCitizenDirectoryEmail, Subject: id, Delta: delta,
		}); err != nil {
			return err
		}
		email = e
		return nil
	})
	if err != nil {
		// Never the address in the message (rule 3).
		return "", fmt.Errorf("xem_email_nguoi_dan: %w", err)
	}
	return email, nil
}

// RevealPhone returns the FULL number of one live contact — "Xem số đầy đủ" (ADR 0097 §1).
//
// EVERY CALL IS ONE ENTRY: two presses are two disclosures (the reading of xem_email_can_bo), so
// there is no idempotency key. The read and the entry share one transaction and the number leaves
// only after the entry has COMMITTED; a failed entry rolls back and the caller answers 500 with
// nothing disclosed — the one ordering rule 6 permits.
//
// idstore.ErrCitizenContactNotFound for an unknown, removed or other-commune id; nothing is audited
// then, because nothing was disclosed.
func (uc *CitizenDirectory) RevealPhone(ctx context.Context, actor NguoiThucHien, id string) (string, error) {
	if err := actor.hopLe(); err != nil {
		return "", err
	}
	if id == "" {
		return "", idstore.ErrCitizenContactNotFound
	}
	// The FIELD opened and the key it was opened under — never the value (rule 6, forbidden #4;
	// ADR 0097 §1). The field is a list, the shape of the two other reveal entries of this service.
	delta := deltaSLA(map[string]any{
		"truong_da_mo": []string{"so_dien_thoai"},
		"quyen":        revealCitizenContactPhonePermission,
	})

	var phone string
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, found, err := uc.contacts.ReadPhone(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return idstore.ErrCitizenContactNotFound
		}
		// TenantID left unset: audit.Write takes it from the transaction, which took it from the
		// context — one source for the commune (rule 1, invariant 4).
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionRevealCitizenDirectoryPhone,
			Subject: id,
			Delta:   delta,
		}); err != nil {
			return err
		}
		phone = p
		return nil
	})
	if err != nil {
		// Never the number in the message (rule 3).
		return "", fmt.Errorf("xem_so_dien_thoai_nguoi_dan: %w", err)
	}
	return phone, nil
}

// Create inserts one contact typed on the form (Source = nhap-tay), its memberships and ONE audit
// entry in one transaction. idstore.ErrCitizenPhoneTaken when the commune already has a live row
// with that number (ADR 0097 D5); idstore.ErrCitizenGroupNotFound for a group id that is not a live
// group of this commune. Returns the new row's id.
func (uc *CitizenDirectory) Create(ctx context.Context, actor NguoiThucHien, req CitizenContactRequest) (string, error) {
	if err := actor.hopLe(); err != nil {
		return "", err
	}
	if req.Email != nil {
		req.Contact.Email = *req.Email
	}
	in, err := domain.CheckCitizenContactInput(req.Contact)
	if err != nil {
		return "", err
	}
	// The store scopes this read by tenant_id through store.DB.For(ctx). Retired fields included: their
	// codes are recognised (and ignored) rather than refused as unknown.
	defs, err := uc.fields.List(ctx, true)
	if err != nil {
		return "", err
	}
	attrs, err := domain.CheckCitizenAttributes(defs, req.Attributes, nil)
	if err != nil {
		return "", err
	}
	groupIDs, err := uc.liveGroupIDs(ctx, req.GroupIDs)
	if err != nil {
		return "", err
	}
	id, err := uc.newID()
	if err != nil {
		return "", fmt.Errorf("danh bạ người dân: sinh id: %w", err)
	}
	at := uc.now()
	c := contactFromInput(id, in, domain.CitizenContactSourceManual, nil, at)
	c.Attributes = attrs

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Checked inside the transaction so the answer and the insert see the same rows; the
		// live-unique key is the backstop for the race that remains.
		taken, err := uc.contacts.PhoneTaken(ctx, tx, c.Phone, "")
		if err != nil {
			return err
		}
		if taken {
			return idstore.ErrCitizenPhoneTaken
		}
		// Insert binds tx.TenantID() as $1 (rule 1, invariant 5).
		if err := uc.contacts.Insert(ctx, tx, c); err != nil {
			return err
		}
		if len(groupIDs) > 0 {
			// created_by on memberships is the STAFF CODE (rule 6, invariant 8).
			if err := uc.contacts.SetGroups(ctx, tx, c.ID, groupIDs, actor.Vet.ID, at); err != nil {
				return err
			}
		}
		// The search index learns of the row through the outbox, in this transaction (ADR 0101 #11).
		if err := uc.recordChanged(ctx, tx, []string{c.ID}, contactOpUpsert, at); err != nil {
			return err
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). Non-personal fields only.
		trail := contactTrail(in, len(groupIDs))
		trail["co_email"] = in.Email != ""
		trail["thuoc_tinh"] = attributeCodes(attrs)
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionAddCitizenContact,
			Subject: c.ID,
			At:      at,
			Delta: deltaSLA(map[string]any{
				"sau":   trail,
				"nguon": string(c.Source),
			}),
		})
	})
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

// Update rewrites one live contact and its memberships, and audits WHICH FIELDS changed — by column
// name, never by value — plus is_active before and after (the one field an inspection asks about:
// "when did the commune stop contacting this person"). A NO-OP WRITES NOTHING AND AUDITS NOTHING,
// as every edit route of this service.
//
// The row and its memberships are read BEFORE the transaction (the store has no in-transaction
// detail read); the UPDATE's `deleted_at IS NULL` still refuses a row removed in between, and the
// phone check runs inside. Two staff editing the same row at once: the later write wins, which is
// what the form does today for every catalogue of this service.
func (uc *CitizenDirectory) Update(ctx context.Context, actor NguoiThucHien, id string, req CitizenContactRequest) error {
	if err := actor.hopLe(); err != nil {
		return err
	}
	if id == "" {
		return idstore.ErrCitizenContactNotFound
	}
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	before, found, err := uc.contacts.Get(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return idstore.ErrCitizenContactNotFound
	}
	// Email absent = keep the stored address (the dialog never had it unmasked); "" = clear.
	req.Contact.Email = before.Email
	if req.Email != nil {
		req.Contact.Email = *req.Email
	}
	in, err := domain.CheckCitizenContactInput(req.Contact)
	if err != nil {
		return err
	}
	attrs := before.Attributes
	if req.AttributesSet {
		// The store scopes this read by tenant_id through store.DB.For(ctx).
		defs, err := uc.fields.List(ctx, true)
		if err != nil {
			return err
		}
		if attrs, err = domain.CheckCitizenAttributes(defs, req.Attributes, before.Attributes); err != nil {
			return err
		}
	}
	groupIDs, err := uc.liveGroupIDs(ctx, req.GroupIDs)
	if err != nil {
		return err
	}
	beforeGroups, err := uc.contacts.GroupIDsOf(ctx, []string{id})
	if err != nil {
		return err
	}
	changed := changedContactFields(before, beforeGroups[id], in, groupIDs)
	changedAttrs := changedAttributeCodes(before.Attributes, attrs)
	if len(changedAttrs) > 0 {
		changed = append(changed, "attributes")
	}
	if len(changed) == 0 {
		return nil
	}
	at := uc.now()
	after := contactFromInput(before.ID, in, before.Source, before.ImportBatchID, before.CreatedAt)
	after.UpdatedAt = at
	after.Attributes = attrs

	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if in.Phone != before.Phone {
			taken, err := uc.contacts.PhoneTaken(ctx, tx, in.Phone, id)
			if err != nil {
				return err
			}
			if taken {
				return idstore.ErrCitizenPhoneTaken
			}
		}
		// Update binds tx.TenantID() as $1 (rule 1, invariant 5).
		if err := uc.contacts.Update(ctx, tx, after); err != nil {
			return err
		}
		if !sameSet(beforeGroups[id], groupIDs) {
			if err := uc.contacts.SetGroups(ctx, tx, id, groupIDs, actor.Vet.ID, at); err != nil {
				return err
			}
		}
		if err := uc.recordChanged(ctx, tx, []string{id}, contactOpUpsert, at); err != nil {
			return err
		}
		delta := map[string]any{
			"truong_da_doi": changed,
			"truoc":         map[string]any{"is_active": before.IsActive},
			"sau":           map[string]any{"is_active": in.IsActive},
		}
		if len(changedAttrs) > 0 {
			// Codes only — a value may be personal data (rule 6, forbidden #4).
			delta["thuoc_tinh_da_doi"] = changedAttrs
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionEditCitizenContact,
			Subject: id,
			At:      at,
			Delta:   deltaSLA(delta),
		})
	})
}

// attributeCodes lists the codes holding a value, sorted — what a create entry records.
func attributeCodes(a map[string]json.RawMessage) []string {
	out := make([]string, 0, len(a))
	for k := range a {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// changedAttributeCodes lists, sorted, the codes whose stored JSON differs between before and after.
func changedAttributeCodes(before, after map[string]json.RawMessage) []string {
	var out []string
	for k, v := range after {
		if string(before[k]) != string(v) {
			out = append(out, k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Remove soft deletes one contact with a MANDATORY reason (ADR 0097 D7): removing a person from the
// directory is the commune ceasing to contact a citizen, and "why" must be answerable.
func (uc *CitizenDirectory) Remove(ctx context.Context, actor NguoiThucHien, id, reason string) error {
	if err := actor.hopLe(); err != nil {
		return err
	}
	if id == "" {
		return idstore.ErrCitizenContactNotFound
	}
	reason, err := domain.NormalizeCitizenContactDeleteReason(reason)
	if err != nil {
		return err
	}
	at := uc.now()
	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// deleted_by is the STAFF CODE (rule 6, invariant 8) — the same value as the entry's actor.
		if err := uc.contacts.SoftDelete(ctx, tx, id, actor.Vet.ID, reason, at); err != nil {
			return err
		}
		if err := uc.recordChanged(ctx, tx, []string{id}, contactOpRemove, at); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionRemoveCitizenContact,
			Subject: id,
			At:      at,
			Delta:   deltaSLA(map[string]any{"ly_do": reason, "xoa_mem": true}),
		})
	})
}

// IsCitizenDirectoryInputError reports a refusal of what the client sent (400). Listed explicitly,
// so a database outage can never become a 400. Conflicts (phone taken, group name taken) and
// not-found are NOT here: they map to 409 and 404 by identity.
func IsCitizenDirectoryInputError(err error) bool {
	for _, e := range []error{
		domain.ErrCitizenContactFullNameMissing, domain.ErrCitizenContactFullNameTooLong,
		domain.ErrCitizenPhoneMissing, domain.ErrCitizenPhoneInvalid, domain.ErrCitizenPhoneTooLong,
		domain.ErrCitizenPhoneNotNational, domain.ErrCitizenContactAddressTooLong,
		domain.ErrCitizenContactNoteTooLong, domain.ErrCitizenContactTooManyTags,
		domain.ErrCitizenContactTagTooLong, domain.ErrCitizenContactTagHasComma,
		domain.ErrCitizenContactUnitRefInvalid, domain.ErrCitizenContactSourceUnknown,
		domain.ErrCitizenGroupNameMissing, domain.ErrCitizenGroupNameTooLong,
		domain.ErrCitizenGroupDescriptionTooLong, domain.ErrCitizenGroupSortOrderRange,
		domain.ErrCitizenDirectoryReasonMissing, domain.ErrCitizenDirectoryReasonTooLong,
		idstore.ErrCitizenContactUnitUnknown,
		domain.ErrCitizenContactEmail, domain.ErrCitizenAttributeUnknown, domain.ErrCitizenAttributeInvalid,
		domain.ErrCitizenAttributeRequired, ErrCitizenSearchFilterInvalid, ErrCitizenSearchCombination,
	} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// liveGroupIDs deduplicates the requested group ids and refuses any that is not a LIVE group of this
// commune — the composite key accepts a soft-deleted group, so the caller must (SetGroups' contract).
// One Get per id: a person is in a handful of groups.
func (uc *CitizenDirectory) liveGroupIDs(ctx context.Context, ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		// The store scopes this read by tenant_id through store.DB.For(ctx).
		_, found, err := uc.groups.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, idstore.ErrCitizenGroupNotFound
		}
		out = append(out, id)
	}
	return out, nil
}

// contactFromInput builds the row the store writes. UpdatedAt = CreatedAt for a new row; Update
// overrides it.
func contactFromInput(id string, in domain.CitizenContactInput, source domain.CitizenContactSource,
	batch *string, createdAt time.Time) domain.CitizenContact {
	c := domain.CitizenContact{
		ID: id, FullName: in.FullName, Phone: in.Phone, Address: in.Address, Note: in.Note,
		Tags: nonNilStrings(in.Tags), Source: source, IsActive: in.IsActive, Email: in.Email,
		ImportBatchID: batch, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	if in.ResidentialUnitID != "" {
		unit := in.ResidentialUnitID
		c.ResidentialUnitID = &unit
	}
	return c
}

// contactTrail is the non-personal payload of a create entry: where the person lives at the level
// of a hamlet (an id, hundreds of people), how many labels and groups, whether in contact. NOT the
// name, number, address, note or the labels themselves — a label is free text a person typed about
// another person (ADR 0097 §5).
func contactTrail(in domain.CitizenContactInput, groups int) map[string]any {
	return map[string]any{
		"residential_unit_id": in.ResidentialUnitID,
		"so_nhan":             len(in.Tags),
		"so_nhom":             groups,
		"is_active":           in.IsActive,
	}
}

// changedContactFields lists, by COLUMN NAME, what an edit changes. Order is fixed so two entries
// for the same edit read the same.
func changedContactFields(before domain.CitizenContact, beforeGroups []string, after domain.CitizenContactInput, afterGroups []string) []string {
	var out []string
	add := func(col string, changed bool) {
		if changed {
			out = append(out, col)
		}
	}
	add("full_name", before.FullName != after.FullName)
	add("phone", before.Phone != after.Phone)
	add("residential_unit_id", derefString(before.ResidentialUnitID) != after.ResidentialUnitID)
	add("address", before.Address != after.Address)
	add("note", before.Note != after.Note)
	add("tags", !sameList(before.Tags, after.Tags))
	add("is_active", before.IsActive != after.IsActive)
	add("email", before.Email != after.Email)
	add("groups", !sameSet(beforeGroups, afterGroups))
	return out
}

func sameList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameSet compares two id lists regardless of order: membership order carries no meaning.
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return sameList(x, y)
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// nonNilStrings makes an empty list marshal as `[]`, never `null`.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
