package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The routes of "Danh bạ người dân" (chapter 16; ADR 0097) — the commune's contact list of citizens,
// and its groups. The Excel import is citizen_directory_import.go.
//
//	GET    /api/v1/citizen-contacts                  one page, filters in the query (no keyword)
//	POST   /api/v1/citizen-contact-searches          one page, keyword + filters in the BODY
//	POST   /api/v1/citizen-contact-count-queries     how many rows the same filter matches
//	GET    /api/v1/citizen-contact-tags              the tag strip: every tag in use and its count
//	POST   /api/v1/citizen-contacts                  add one person
//	GET    /api/v1/citizen-contacts/{id}             the edit dialog's view (address and note)
//	PATCH  /api/v1/citizen-contacts/{id}             save the whole form
//	POST   /api/v1/citizen-contacts/{id}/deletion    soft delete, reason mandatory
//	GET    /api/v1/citizen-contacts/{id}/phone       "Xem số đầy đủ" — one audit entry per call
//	GET    /api/v1/citizen-groups                    the groups and their member counts
//	POST   /api/v1/citizen-groups                    create a group
//	PATCH  /api/v1/citizen-groups/{id}               rename / describe / reorder a group
//	POST   /api/v1/citizen-groups/{id}/deletion      soft delete, reason optional
//
// TWO KEYS AND NO THIRD (ADR 0097 D2, §1; rule 5, invariant 3c): `citizen.read` on every read of the
// list, `citizen.update` on every write AND on the full number — both seeded by migration 0028. The
// reveal deliberately reuses the write key instead of an unmask key of its own.
//
// THE NUMBER IS MASKED ON EVERY ROUTE BUT ONE. The use case renders privacy.MaskPhone into the list
// item and the detail; the wire types below carry `phone_masked` and have NO field for the full
// number, so a handler cannot forward one by mistake. The one exit is GET …/{id}/phone, whose entry
// is written in the same transaction as the read (app.RevealPhone).
//
// THE KEYWORD TRAVELS IN THE BODY (ADR 0097 D11): it matches any part of a phone number, and a
// fragment of a number on a URL is personal data in every access log (rule 3, forbidden #4). GET
// /api/v1/citizen-contacts therefore takes the structural filters only; the search and the count
// are POSTs, the shape of POST /api/v1/staff/searches and /api/v1/staff-count-queries.
//
// THE COMMUNE IS NEVER PASSED. Every read and write goes through store.DB.For(ctx), bound to the
// Host-derived tenant_id (rule 1, invariant 4); another commune's id is simply not found.
//
// NOTHING PERSONAL IN A LOG LINE OR AN ERROR BODY (rule 3): the handlers log the commune and the
// wrapped error only; every refusal sentence names a rule, never a value. The one logged error on
// the search path carries the store failure, never the keyword.

// citizenContactBodyMax bounds a contact or group body: a name, a number, an address (500 runes),
// a note (1000), twenty tags, a handful of group ids. The staff register's bound.
const citizenContactBodyMax = thanCanBoToiDa

// CitizenDirectoryUseCases is what the thirteen routes here and the three import routes need from
// the use case — *app.CitizenDirectory in production. One interface and not a read/write pair,
// because every read of this screen is a use case too: the mask is applied there, not in a store.
// Every method reads its commune from the context (store.DB.For(ctx)); none takes a tenant_id.
type CitizenDirectoryUseCases interface {
	List(ctx context.Context, f idstore.CitizenContactFilter, req page.Request) (page.Result[app.CitizenContactListItem], error)
	Count(ctx context.Context, f idstore.CitizenContactFilter) (int, error)
	TagCounts(ctx context.Context) ([]idstore.TagCount, error)
	Get(ctx context.Context, id string) (app.CitizenContactDetail, error)
	RevealPhone(ctx context.Context, actor app.NguoiThucHien, id string) (string, error)
	Create(ctx context.Context, actor app.NguoiThucHien, req app.CitizenContactRequest) (string, error)
	Update(ctx context.Context, actor app.NguoiThucHien, id string, req app.CitizenContactRequest) error
	Remove(ctx context.Context, actor app.NguoiThucHien, id, reason string) error

	ListGroups(ctx context.Context) ([]app.CitizenGroupItem, error)
	CreateGroup(ctx context.Context, actor app.NguoiThucHien, in domain.CitizenGroupInput) (string, error)
	UpdateGroup(ctx context.Context, actor app.NguoiThucHien, id string, in domain.CitizenGroupInput) error
	RemoveGroup(ctx context.Context, actor app.NguoiThucHien, id, reason string) error

	ImportPreview(ctx context.Context, sheet [][]string, units []app.ImportResidentialUnit) (app.CitizenContactImportPreview, error)
	ImportCommit(ctx context.Context, actor app.NguoiThucHien, sheet [][]string, units []app.ImportResidentialUnit) (app.CitizenContactImportResult, error)
}

// --- wire shapes: contacts -------------------------------------------------------------------------

// citizenContactOut is one row of the list (chapter 16 §4). NO address, NO note (ADR 0097 D13) and
// NO full number — `phone_masked` is privacy.MaskPhone's rendering (09****0001).
type citizenContactOut struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
	// PhoneMasked is the ONLY phone field on the list; the full number has its own audited route.
	PhoneMasked string `json:"phone_masked"`
	// ResidentialUnitID and ResidentialUnitName: "" when none; the name is resolved in the page query.
	ResidentialUnitID   string   `json:"residential_unit_id"`
	ResidentialUnitName string   `json:"residential_unit_name"`
	Tags                []string `json:"tags"`
	GroupIDs            []string `json:"group_ids"`
	// IsActive false = the commune stopped contacting this person; the row still lists (§10 #8).
	IsActive bool `json:"is_active"`
	// Source is `nhap-tay` or `nhap-excel` (ADR 0011).
	Source string `json:"source"`
	// HasOwnAppSession is the "Zalo" column (ADR 0097 §3): the number is signed in to THIS commune's
	// Mini App right now. One boolean — never an id, never whether the number is known elsewhere.
	HasOwnAppSession bool      `json:"has_own_app_session"`
	CreatedAt        time.Time `json:"created_at"`
}

// citizenContactDetailOut is the edit dialog's view (chapter 16 §5): address and note included, the
// number STILL masked until "Xem số đầy đủ" is pressed.
type citizenContactDetailOut struct {
	ID                string    `json:"id"`
	FullName          string    `json:"full_name"`
	PhoneMasked       string    `json:"phone_masked"`
	ResidentialUnitID string    `json:"residential_unit_id"`
	Address           string    `json:"address"`
	Note              string    `json:"note"`
	Tags              []string  `json:"tags"`
	GroupIDs          []string  `json:"group_ids"`
	IsActive          bool      `json:"is_active"`
	Source            string    `json:"source"`
	ImportBatchID     string    `json:"import_batch_id"` // "" unless source is nhap-excel
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// citizenContactIn is the whole add / edit form (chapter 16 §5). PATCH sends the WHOLE form: the
// use case compares and audits what changed. `group_ids` is the full membership — absent means
// removed. `is_active` absent = true (the checkbox's default).
type citizenContactIn struct {
	FullName          string   `json:"full_name"`
	Phone             string   `json:"phone"` // any spelling; the server normalises to 0…
	ResidentialUnitID string   `json:"residential_unit_id"`
	Address           string   `json:"address"`
	Note              string   `json:"note"`
	Tags              []string `json:"tags"`
	GroupIDs          []string `json:"group_ids"`
	IsActive          *bool    `json:"is_active"`
}

// citizenContactCreatedOut is the 201 of POST: the new row's id, what the dialog opens next.
type citizenContactCreatedOut struct {
	ID string `json:"id"`
}

// citizenContactDeletionIn — the reason is MANDATORY (ADR 0097 D7): removing a person is the commune
// ceasing to contact a citizen. Kept out of the query string, where free text lands in access logs.
type citizenContactDeletionIn struct {
	Reason string `json:"reason"`
}

// citizenContactPhoneOut is the one body that carries a full number — the reply of the audited
// reveal, and nothing else reuses it.
type citizenContactPhoneOut struct {
	Phone string `json:"phone"`
}

// citizenContactSearchIn is the body of POST /api/v1/citizen-contact-searches.
//
//	q                      optional keyword: full name and address accent-insensitively, and ANY
//	                       PART of the phone number (ADR 0097 D11). Trimmed, ≤ 200 characters.
//	group_id               optional: only members of this group
//	residential_unit_id    optional: only this hamlet / street block
//	active                 optional: true = only people still contacted; absent/null = everyone
//	has_own_app_session    optional: true = only numbers signed in to this commune's Mini App
//	                       ("Chỉ người có Zalo"); false = only those not; absent = everyone
//	limit, cursor          as GET's `?limit=` / `?cursor=`; 50 per page is the screen's choice
type citizenContactSearchIn struct {
	Q                 string `json:"q"`
	GroupID           string `json:"group_id"`
	ResidentialUnitID string `json:"residential_unit_id"`
	Active            *bool  `json:"active"`
	HasOwnAppSession  *bool  `json:"has_own_app_session"`
	Limit             *int   `json:"limit"`
	Cursor            string `json:"cursor"`
}

// citizenContactCountQueryIn is the body of POST /api/v1/citizen-contact-count-queries: the search's
// filter without its paging.
type citizenContactCountQueryIn struct {
	Q                 string `json:"q"`
	GroupID           string `json:"group_id"`
	ResidentialUnitID string `json:"residential_unit_id"`
	Active            *bool  `json:"active"`
	HasOwnAppSession  *bool  `json:"has_own_app_session"`
}

// citizenContactCountOut — the shape of the separate count route (ADR 0097 D8, ADR 0071).
type citizenContactCountOut struct {
	Count int `json:"count"`
}

// citizenContactTagOut is one entry of the tag strip (chapter 16 §3).
type citizenContactTagOut struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type citizenContactTagsOut struct {
	Items []citizenContactTagOut `json:"items"`
}

// --- wire shapes: groups ---------------------------------------------------------------------------

// citizenGroupOut is one row of the Nhóm tab (chapter 16 §6). A group names a kind of person, not a
// person: nothing here is personal data.
type citizenGroupOut struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SortOrder   int       `json:"sort_order"`
	MemberCount int       `json:"member_count"` // live members
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type citizenGroupsOut struct {
	Items []citizenGroupOut `json:"items"`
}

// citizenGroupIn is the body of POST and PATCH — the whole group, every time.
type citizenGroupIn struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

type citizenGroupCreatedOut struct {
	ID string `json:"id"`
}

// citizenGroupDeletionIn — the reason is OPTIONAL (ADR 0097 D7): removing a group tidies a catalogue.
type citizenGroupDeletionIn struct {
	Reason string `json:"reason"`
}

// --- mapping ----------------------------------------------------------------------------------------

func citizenContactToOut(c app.CitizenContactListItem) citizenContactOut {
	return citizenContactOut{
		ID: c.ID, FullName: c.FullName, PhoneMasked: c.PhoneMasked,
		ResidentialUnitID: c.ResidentialUnitID, ResidentialUnitName: c.ResidentialUnitName,
		Tags: nonNilList(c.Tags), GroupIDs: nonNilList(c.GroupIDs),
		IsActive: c.IsActive, Source: string(c.Source), HasOwnAppSession: c.HasOwnAppSession,
		CreatedAt: c.CreatedAt.UTC(),
	}
}

func citizenContactDetailToOut(d app.CitizenContactDetail) citizenContactDetailOut {
	return citizenContactDetailOut{
		ID: d.ID, FullName: d.FullName, PhoneMasked: d.PhoneMasked,
		ResidentialUnitID: d.ResidentialUnitID, Address: d.Address, Note: d.Note,
		Tags: nonNilList(d.Tags), GroupIDs: nonNilList(d.GroupIDs),
		IsActive: d.IsActive, Source: string(d.Source), ImportBatchID: d.ImportBatchID,
		CreatedAt: d.CreatedAt.UTC(), UpdatedAt: d.UpdatedAt.UTC(),
	}
}

func citizenGroupToOut(g app.CitizenGroupItem) citizenGroupOut {
	return citizenGroupOut{
		ID: g.ID, Name: g.Name, Description: g.Description, SortOrder: g.SortOrder,
		MemberCount: g.MemberCount, CreatedAt: g.CreatedAt.UTC(), UpdatedAt: g.UpdatedAt.UTC(),
	}
}

// nonNilList makes an empty list marshal as `[]`, never `null`.
func nonNilList(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (in citizenContactIn) toRequest() app.CitizenContactRequest {
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	return app.CitizenContactRequest{
		Contact: domain.CitizenContactInput{
			FullName: in.FullName, Phone: in.Phone, ResidentialUnitID: in.ResidentialUnitID,
			Address: in.Address, Note: in.Note, Tags: in.Tags, IsActive: active,
		},
		GroupIDs: in.GroupIDs,
	}
}

// citizenContactFilterFrom builds the store filter from the five filter fields and answers 400
// itself. SHARED by the list, the search and the count, so a total and the list it heads are refused
// and normalised by one piece of code — a second copy is where "the count says 12, the list shows 11"
// comes from.
//
// The two ids are checked for SHAPE only (ULID, domain.NormalizeCitizenContactUnitRef — the same
// rule for a group id, which is minted by the same core/ulid); whether they name a live row of this
// commune is the query's question, and a well-formed id that names nothing matches nobody.
func citizenContactFilterFrom(w http.ResponseWriter, q, groupID, unitID string, active, hasOwnAppSession *bool) (idstore.CitizenContactFilter, bool) {
	var f idstore.CitizenContactFilter
	if strings.TrimSpace(q) != "" {
		// The staff register's normaliser: whitespace collapsed, 1..200 runes. The sentence names
		// the bound, never the keyword (it is likely a name or a number).
		text, err := domain.ChuanHoaTuKhoaTimCanBo(q)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"Từ khoá tìm kiếm quá dài (tối đa "+strconv.Itoa(domain.TuKhoaTimCanBoToiDa)+" ký tự).", "")
			return f, false
		}
		f.Keyword = text
	}
	var err error
	if f.GroupID, err = domain.NormalizeCitizenContactUnitRef(groupID); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Mã nhóm dùng để lọc không hợp lệ.", "")
		return f, false
	}
	if f.ResidentialUnitID, err = domain.NormalizeCitizenContactUnitRef(unitID); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Mã thôn / tổ dân phố dùng để lọc không hợp lệ.", "")
		return f, false
	}
	// `active: true` narrows to people still contacted; false/absent lists everyone, because a
	// switched-off row still belongs to the directory (chapter 16 §10 #8) and the store has no
	// "inactive only" predicate — nothing on the screen asks for one.
	f.ActiveOnly = active != nil && *active
	f.HasOwnAppSession = hasOwnAppSession
	return f, true
}

// queryBool reads a `true` / `false` query parameter — ONLY those two spellings, the reading of GET
// /api/v1/staff's `published`. Returns false on any other value, after answering 400.
func queryBool(w http.ResponseWriter, q url.Values, name string) (*bool, bool) {
	switch thamSoLoc(q, name) {
	case "":
		return nil, true
	case "true":
		return ptrBool(true), true
	case "false":
		return ptrBool(false), true
	default:
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tham số `"+name+"` chỉ nhận true hoặc false.", "")
		return nil, false
	}
}

// --- handlers: contacts -----------------------------------------------------------------------------

// ListCitizenContacts — GET /api/v1/citizen-contacts. No keyword here: that is the search's body.
//
// NO AUDIT ENTRY: the numbers are masked and the read stays inside the commune (rule 6, invariant 7
// does not reach it). The risk the masked list still carries through the search box is ADR 0097 §4.
func (h *Handler) ListCitizenContacts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	active, ok := queryBool(w, q, "active")
	if !ok {
		return
	}
	hasOwnAppSession, ok := queryBool(w, q, "has_own_app_session")
	if !ok {
		return
	}
	f, ok := citizenContactFilterFrom(w, "", thamSoLoc(q, "group_id"), thamSoLoc(q, "residential_unit_id"), active, hasOwnAppSession)
	if !ok {
		return
	}
	// Parsed BEFORE the store is touched: a rejected page request runs no statement at all.
	req, err := page.Parse(q, idstore.CitizenContactSorts)
	if err != nil {
		status, code, msg := page.HTTPError(err)
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	h.writeCitizenContactPage(w, r, "danh sách người dân", f, req)
}

// SearchCitizenContacts — POST /api/v1/citizen-contact-searches. The keyword is in the body and is
// NEVER LOGGED, on success or failure (rule 3, invariant 1).
func (h *Handler) SearchCitizenContacts(w http.ResponseWriter, r *http.Request) {
	var in citizenContactSearchIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	f, ok := citizenContactFilterFrom(w, in.Q, in.GroupID, in.ResidentialUnitID, in.Active, in.HasOwnAppSession)
	if !ok {
		return
	}
	// page.New and not page.Parse: the paging fields arrive in the body. One sort column
	// (created_at, newest first), so there is no sort/order field to read.
	limit := ""
	if in.Limit != nil {
		limit = strconv.Itoa(*in.Limit)
	}
	req, err := page.New(idstore.CitizenContactSorts, "", "", limit, in.Cursor)
	if err != nil {
		status, code, msg := page.HTTPError(err)
		httpx.WriteError(w, status, code, msg, "")
		return
	}
	h.writeCitizenContactPage(w, r, "tìm người dân", f, req)
}

// writeCitizenContactPage runs one page read and writes it — shared by the list and the search so the
// two cannot drift in the shape on the wire or in what a failure may say.
func (h *Handler) writeCitizenContactPage(w http.ResponseWriter, r *http.Request, what string, f idstore.CitizenContactFilter, req page.Request) {
	ctx := r.Context()
	// tenant_id: the context's, from Host; the store reads through store.DB.For(ctx).
	res, err := h.d.CitizenDirectory.List(ctx, f, req)
	if err != nil {
		// The filter is NOT logged: it may hold the keyword.
		h.d.Log.Error("danh bạ người dân: "+what+" lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := page.Result[citizenContactOut]{Items: make([]citizenContactOut, 0, len(res.Items)),
		NextCursor: res.NextCursor, HasMore: res.HasMore}
	for _, c := range res.Items {
		out.Items = append(out.Items, citizenContactToOut(c))
	}
	vietJSON(w, http.StatusOK, out)
}

// CountCitizenContacts — POST /api/v1/citizen-contact-count-queries: the "n người" of the tab label
// under the current filter. A count, no personal data, no audit entry.
func (h *Handler) CountCitizenContacts(w http.ResponseWriter, r *http.Request) {
	var in citizenContactCountQueryIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	f, ok := citizenContactFilterFrom(w, in.Q, in.GroupID, in.ResidentialUnitID, in.Active, in.HasOwnAppSession)
	if !ok {
		return
	}
	ctx := r.Context()
	// tenant_id: the context's, from Host; the store counts through store.DB.For(ctx).
	n, err := h.d.CitizenDirectory.Count(ctx, f)
	if err != nil {
		h.d.Log.Error("danh bạ người dân: đếm lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, citizenContactCountOut{Count: n})
}

// ListCitizenContactTags — GET /api/v1/citizen-contact-tags. The tags are free text staff typed about
// people (ADR 0097 §5); they are the commune's own labels, read inside the commune.
func (h *Handler) ListCitizenContactTags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// tenant_id: the context's, from Host; the store reads through store.DB.For(ctx).
	tags, err := h.d.CitizenDirectory.TagCounts(ctx)
	if err != nil {
		h.d.Log.Error("danh bạ người dân: đọc nhãn lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := citizenContactTagsOut{Items: make([]citizenContactTagOut, 0, len(tags))}
	for _, t := range tags {
		out.Items = append(out.Items, citizenContactTagOut{Tag: t.Tag, Count: t.Count})
	}
	vietJSON(w, http.StatusOK, out)
}

// GetCitizenContact — GET /api/v1/citizen-contacts/{id}. 404 for an unknown, removed or another
// commune's id alike (rule 4, forbidden #2).
func (h *Handler) GetCitizenContact(w http.ResponseWriter, r *http.Request) {
	// tenant_id: the context's, from Host; the store reads through store.DB.For(ctx), so another
	// commune's id is simply not found.
	d, err := h.d.CitizenDirectory.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeCitizenContactError(w, r, "đọc chi tiết", err)
		return
	}
	vietJSON(w, http.StatusOK, citizenContactDetailToOut(d))
}

// CreateCitizenContact — POST /api/v1/citizen-contacts. 201 with the id.
func (h *Handler) CreateCitizenContact(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenContactIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	id, err := h.d.CitizenDirectory.Create(r.Context(), actor, in.toRequest())
	if err != nil {
		h.writeCitizenContactError(w, r, "thêm người dân", err)
		return
	}
	// What a retry with the same Idempotency-Key is told: the id, never the body.
	idem.RecordCode(r.Context(), id)
	vietJSON(w, http.StatusCreated, citizenContactCreatedOut{ID: id})
}

// UpdateCitizenContact — PATCH /api/v1/citizen-contacts/{id}. 204: the dialog closes and the list
// reloads; nothing is echoed. A form identical to the row writes nothing and audits nothing.
func (h *Handler) UpdateCitizenContact(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenContactIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx, so another
	// commune's id is simply not found (404).
	if err := h.d.CitizenDirectory.Update(r.Context(), actor, r.PathValue("id"), in.toRequest()); err != nil {
		h.writeCitizenContactError(w, r, "sửa người dân", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveCitizenContact — POST /api/v1/citizen-contacts/{id}/deletion. 204; the row stays with its
// three soft-delete columns (rule 7). An empty reason is 400 (ADR 0097 D7).
func (h *Handler) RemoveCitizenContact(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenContactDeletionIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	if err := h.d.CitizenDirectory.Remove(r.Context(), actor, r.PathValue("id"), in.Reason); err != nil {
		h.writeCitizenContactError(w, r, "gỡ người dân", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RevealCitizenContactPhone — GET /api/v1/citizen-contacts/{id}/phone, "Xem số đầy đủ" (ADR 0097
// §1). `citizen.update`, declared in routes.go. EVERY CALL IS ONE AUDIT ENTRY, written by the use
// case in the same transaction as the read; a failed entry is a 500 with nothing disclosed. 404 for
// an unknown, removed or another commune's id — one answer. NEITHER THE NUMBER NOR THE ID IS LOGGED.
func (h *Handler) RevealCitizenContactPhone(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	ctx := r.Context()
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	phone, err := h.d.CitizenDirectory.RevealPhone(ctx, actor, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, idstore.ErrCitizenContactNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "citizen_contact_not_found", "Không tìm thấy người dân này trong danh bạ.", "")
			return
		}
		h.d.Log.Error("danh bạ người dân: xem số đầy đủ lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	// A full number: never cached between this process and the dialog.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusOK, citizenContactPhoneOut{Phone: phone})
}

// --- handlers: groups ------------------------------------------------------------------------------

// ListCitizenGroups — GET /api/v1/citizen-groups, with live member counts.
func (h *Handler) ListCitizenGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// tenant_id: the context's, from Host; the store reads through store.DB.For(ctx).
	groups, err := h.d.CitizenDirectory.ListGroups(ctx)
	if err != nil {
		h.d.Log.Error("nhóm người dân: đọc danh sách lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	out := citizenGroupsOut{Items: make([]citizenGroupOut, 0, len(groups))}
	for _, g := range groups {
		out.Items = append(out.Items, citizenGroupToOut(g))
	}
	vietJSON(w, http.StatusOK, out)
}

// CreateCitizenGroup — POST /api/v1/citizen-groups. 201 with the id; 409 `name_taken`.
func (h *Handler) CreateCitizenGroup(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenGroupIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	id, err := h.d.CitizenDirectory.CreateGroup(r.Context(), actor, domain.CitizenGroupInput{
		Name: in.Name, Description: in.Description, SortOrder: in.SortOrder})
	if err != nil {
		h.writeCitizenGroupError(w, r, "lập nhóm", err)
		return
	}
	// What a retry with the same Idempotency-Key is told: the id, never the body.
	idem.RecordCode(r.Context(), id)
	vietJSON(w, http.StatusCreated, citizenGroupCreatedOut{ID: id})
}

// UpdateCitizenGroup — PATCH /api/v1/citizen-groups/{id}. 204.
func (h *Handler) UpdateCitizenGroup(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenGroupIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx, so another
	// commune's id is simply not found (404).
	err := h.d.CitizenDirectory.UpdateGroup(r.Context(), actor, r.PathValue("id"), domain.CitizenGroupInput{
		Name: in.Name, Description: in.Description, SortOrder: in.SortOrder})
	if err != nil {
		h.writeCitizenGroupError(w, r, "sửa nhóm", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveCitizenGroup — POST /api/v1/citizen-groups/{id}/deletion. 204; the members stay in the
// directory. The body may be `{}`: the reason is optional (ADR 0097 D7).
func (h *Handler) RemoveCitizenGroup(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var in citizenGroupDeletionIn
	if !readCitizenDirectoryBody(w, r, &in) {
		return
	}
	// tenant_id: the context's, from Host; the use case opens store.DB.For(ctx).Tx.
	if err := h.d.CitizenDirectory.RemoveGroup(r.Context(), actor, r.PathValue("id"), in.Reason); err != nil {
		h.writeCitizenGroupError(w, r, "gỡ nhóm", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- shared ----------------------------------------------------------------------------------------

// readCitizenDirectoryBody decodes one JSON body under the bound — docThanCanBo's behaviour with
// this screen's bound. The decoder's message quotes the input and is not returned (rule 3,
// forbidden #3).
func readCitizenDirectoryBody(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, citizenContactBodyMax)
	return docThanCanBo(w, r, into)
}

// writeCitizenContactError maps one refusal of a CONTACT route onto a status — one function for
// every contact route, so the mapping cannot drift between them.
//
// A group id in a contact's body that names no live group is 400 `citizen_group_not_found`, not
// 404: the person being written exists (or is being created); it is the FORM that references
// something gone — the client re-reads GET /api/v1/citizen-groups and lets the person choose again.
func (h *Handler) writeCitizenContactError(w http.ResponseWriter, r *http.Request, what string, err error) {
	switch {
	case errors.Is(err, idstore.ErrCitizenContactNotFound):
		// One answer for an invented id, a removed row and another commune's row (rule 4, forbidden #2).
		httpx.WriteError(w, http.StatusNotFound, "citizen_contact_not_found", "Không tìm thấy người dân này trong danh bạ.", "")
	case errors.Is(err, idstore.ErrCitizenPhoneTaken):
		httpx.WriteError(w, http.StatusConflict, "phone_taken", "Số điện thoại này đã có trong danh bạ.", "")
	case errors.Is(err, idstore.ErrCitizenGroupNotFound):
		httpx.WriteError(w, http.StatusBadRequest, "citizen_group_not_found",
			"Một nhóm đã chọn không còn trong danh mục nhóm của xã. Hãy tải lại danh sách nhóm rồi chọn lại.", "")
	case app.IsCitizenDirectoryInputError(err):
		// The domain's own sentence: it names the rule and never echoes the value.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("danh bạ người dân: "+what+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// writeCitizenGroupError maps one refusal of a GROUP route. Here an unknown group id IS the resource
// addressed, so it is 404.
func (h *Handler) writeCitizenGroupError(w http.ResponseWriter, r *http.Request, what string, err error) {
	switch {
	case errors.Is(err, idstore.ErrCitizenGroupNotFound):
		httpx.WriteError(w, http.StatusNotFound, "citizen_group_not_found", "Không tìm thấy nhóm này.", "")
	case errors.Is(err, idstore.ErrCitizenGroupNameTaken):
		httpx.WriteError(w, http.StatusConflict, "name_taken", "Xã đã có một nhóm cùng tên. Hãy đặt tên khác.", "")
	case app.IsCitizenDirectoryInputError(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("nhóm người dân: "+what+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
