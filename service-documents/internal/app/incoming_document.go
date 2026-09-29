package app

// The use cases behind the WRITE surface of SỔ VĂN BẢN ĐẾN
// (docs/ui-ux/05-van-ban-don-thu.md §3, §5, §7).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx with no overload that writes
// outside one. Opening that transaction is this layer's job. The handler translates HTTP and nothing
// else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THE NUMBER. Booking a document is: allocate the next number under a row lock,
// insert the row, record the trail — and all three have to be ONE transaction, or the register ends
// up with a number nobody used, or a document nobody can account for. That needs the lock AND the
// rules AND the transaction at once, which is exactly one place: here.
//
// ---------------------------------------------------------------------------
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE. Said once, here:
//
//	UNIQUE (tenant_id, nam, so_vao_so)   migration 0004 — counts soft-deleted rows
//	the six states                       0004, `van_ban_den_trang_thai_hop_le`
//	the issued number is immutable       0004, trigger `so_van_ban_bat_bien`
//	the counter only moves forward       0004, trigger `day_so_van_ban_khong_lui`
//	hard removal refused outright        0004, `ho_so_luu_tru_cam_xoa_cung`
//	the timeline is append-only          0004, trigger `lich_su_chuyen_chi_them`
//
// Those hold against every writer — this service, a psql prompt, an import job written next year.
// What they cannot do is explain themselves: PostgreSQL answers with an exception whose text is
// English and names a constraint, which tells a clerk in a commune nothing they can act on. This
// layer refuses FIRST, in Vietnamese. A drift between the two is therefore a worse error message,
// never a hole — the constraint still runs last, and the whole transaction rolls back with the audit
// entry inside it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// IncomingDocumentStore is the register's write half, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the document in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that the number is allocated under a lock — provable without a PostgreSQL.
// There is none reachable from this repository's build environment (VIGOV_TEST_DSN is unset), so a
// test that needed one would be a test that never runs.
type IncomingDocumentStore interface {
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.IncomingDocument, error)
	DocumentTypeInUse(ctx context.Context, tx *store.ScopedTx, code string) error
	Insert(ctx context.Context, tx *store.ScopedTx, d domain.IncomingDocument) error
	Update(ctx context.Context, tx *store.ScopedTx, d domain.IncomingDocument) error
	RouteToOrgUnit(ctx context.Context, tx *store.ScopedTx, id, toOrgUnitID, assigneeCode string,
		status domain.IncomingDocumentStatus) error
	InsertRouting(ctx context.Context, tx *store.ScopedTx, r domain.DocumentRouting) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error

	// The detail drawer's two reads. They take the transaction too, for the reason above: the
	// timeline must only be read after the document was found visible, in the same transaction.
	ByID(ctx context.Context, tx *store.ScopedTx, id string) (domain.IncomingDocument, error)
	RoutingHistory(ctx context.Context, tx *store.ScopedTx, incomingDocumentID string) ([]domain.DocumentRouting, error)
}

// NumberSeriesStore allocates register numbers. A SEPARATE INTERFACE from the one above, and not
// three more methods on it, because the two have different obligations: everything in
// IncomingDocumentStore writes the document, while this one writes the COUNTER and must be called
// exactly once per booking, inside the same transaction, under its row lock. Two interfaces, two
// obligations, visible at the point of use.
type NumberSeriesStore interface {
	IssueNumber(ctx context.Context, tx *store.ScopedTx, register docstore.Register, year int) (int, error)
}

// DeadlineReader asks identity for the instant this commune's commitment on a document falls due.
//
// THE FULL identityclient.Client SIGNATURE, DELIBERATELY, rather than a narrow `Due(ctx, t)` that
// would read better here. Three of the four arguments are decisions ADR 0028 makes about THIS act —
// which work kind's hours, which field code, which clock is being fixed — and a wrapper would move
// them out of this file into a piece of wiring nobody tests. Keeping them here is what lets the test
// assert that the document register asks for `van-ban-den`, for the DEFAULT row (`linh_vuc` ""), and
// for the processing clock ALONE.
//
// *identityclient.Client satisfies this as it is.
type DeadlineReader interface {
	// vi-name-ok: mirrors the existing exported method core/identityclient.Client.HanXuLy (core keeps its names)
	HanXuLy(ctx context.Context, workKind identityv1.WorkKind, fieldCode string, from time.Time,
		kinds []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error)
}

// ErrDeadlineNotEstablished means the commune's commitment could not be established, so the
// document was NOT booked.
//
// # THIS IS THE MOST CONSEQUENTIAL LINE IN THE SERVICE AND IT IS DELIBERATE
//
// `sla` is EMPTY FOR EVERY COMMUNE TODAY: migration 0008 of service-identity seeds nothing and the
// onboarding step that sows a commune's first rows does not exist in this repository. So until
// somebody fills in a commune's SLA, POST /api/v1/incoming-documents answers 409 and books nothing.
// That is the contract working, not a bug to route around (core/identityclient.HanXuLy states it in
// full):
//
//	DO NOT fall back to a number. Not 40 hours, not the specification's own table. §7 rule 3's
//	figures came from ONE commune's prototype; a commitment invented by software is still reported
//	upward as though the authority made it (rule 10, forbidden #3).
//
//	DO NOT book the document without a deadline. `han_xu_ly_xong` is NOT NULL precisely so that
//	this decision cannot be made quietly by an INSERT; a row with no deadline is a document nobody
//	counts while every overdue report counts it anyway.
//
//	DO NOT add hours here. hooks/citizen_commitment_guard.py blocks the shape outside
//	service-identity, and rule 10, forbidden #2 is why: adding a duration walks straight through
//	nights, weekends, `ngay_nghi_le` and `ngay_lam_bu` while looking like it respected the unit.
//
// ONE SENTINEL FOR EVERY CAUSE — no `sla` row, no working calendar, identity unreachable, wrong
// caller key. The caller's answer is the same for all of them: refuse and say which screen fixes it.
// The cause is not dropped: it rides in the wrapped chain, and core/identityclient has already
// logged the gRPC code, which is the value that tells an operator whether to open the configuration
// screen or the incident channel.
var ErrDeadlineNotEstablished = errors.New("van_ban_den: xã chưa cấu hình thời hạn xử lý nên chưa vào sổ được")

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `them_chung_tu_giai_ngan`): an inspection reads these strings,
// and a function name would tell them nothing.
//
// FOUR VERBS AND NOT ONE `sua_van_ban_den`, because they are four different administrative acts.
// `chuyen_van_ban_den` in particular is the one an inspection searches for by name — it is the
// record of who was made responsible for a document, and when.
//
// THE VALUES ARE STORED in the append-only `audit_log.action`; ADR 0061 §Ánh xạ hành vi maps them to
// their English successors, which only NEW entries will carry (X22, layer C).
const (
	ActionRegisterIncomingDocument = "vao_so_van_ban_den"
	ActionUpdateIncomingDocument   = "sua_van_ban_den"
	ActionRemoveIncomingDocument   = "go_van_ban_den"
	ActionRouteIncomingDocument    = "chuyen_van_ban_den"
)

// RegisterIncomingDocumentRequest is one document as it arrives from the handler.
//
// THERE IS NO `ArrivalNo` FIELD AND THERE MUST NEVER BE ONE. The number is the register's to give: it
// comes from the counter, under a row lock, inside the transaction. A field here is a field a
// handler can fill from a request body, and what that buys is a client reissuing a number that is
// already on a document somebody has acted on.
//
// THERE IS NO `Year` FIELD EITHER. The year is the year of the ACT — see Register.
//
// THERE IS NO `Status` FIELD: the store writes `moi-vao-so` as a literal, and the state moves by
// routing and by nothing else.
//
// THERE IS NO `DueAt` FIELD: the commitment is this commune's SLA and this commune's calendar,
// computed by identity. A client choosing its own deadline is a client choosing how long the
// authority may take.
type RegisterIncomingDocumentRequest struct {
	ReceivedDate time.Time
	ReferenceNo  string
	DocumentDate time.Time
	IssuingBody  string
	DocumentType string
	Summary      string
	Urgency      domain.Urgency
}

// UpdateIncomingDocumentRequest is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. `ReferenceNo`, `DocumentDate` and `Urgency` are optional
// and their meaningful value includes the empty one — "this document carries no number of its own"
// is a statement, not an absence of one. A struct of plain values cannot tell "the client did not
// mention this" from "the client cleared it", so a dialog editing only the summary would silently
// wipe the issuing body's number off an archival record.
type UpdateIncomingDocumentRequest struct {
	ReceivedDate *time.Time
	ReferenceNo  *string
	DocumentDate *time.Time
	IssuingBody  *string
	DocumentType *string
	Summary      *string
	Urgency      *domain.Urgency
}

// RouteDocumentRequest is the "Chuyển cho bộ phận khác" block of §3.5.
type RouteDocumentRequest struct {
	ToOrgUnitID  string
	AssigneeCode string // optional — "— Để bộ phận tự phân công —"
	Reason       string
}

// IncomingDocuments owns booking, correcting, routing and removing one commune's incoming documents.
type IncomingDocuments struct {
	db        *store.DB
	repo      IncomingDocumentStore
	series    NumberSeriesStore
	deadlines DeadlineReader

	// newID is injected so a test can pin the id. In production it is ulid.Moi.
	newID func() (string, error)

	// now is the clock the booking instant, the routing instant and the deadline's origin are all
	// taken from.
	//
	// A SEAM AND NOT time.Now() AT THE CALL SITE, for a reason about evidence rather than
	// convenience: the instant this returns becomes the ORIGIN the commitment is counted from, and a
	// test that cannot pin it cannot assert that the deadline asked for and the deadline stored are
	// about the same moment. In production this is nil and clock returns the real clock.
	now func() time.Time
}

func NewIncomingDocuments(db *store.DB, repo IncomingDocumentStore, series NumberSeriesStore,
	deadlines DeadlineReader) *IncomingDocuments {
	return &IncomingDocuments{db: db, repo: repo, series: series, deadlines: deadlines, newID: ulid.Moi}
}

// clock is the clock, UTC. `TIMESTAMPTZ` stores an instant rather than a wall reading, so the
// location here changes nothing that is stored — it is fixed so that a value read back in a test
// compares equal without a location dance.
func (uc *IncomingDocuments) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

// Register books one incoming document: allocates its number, stores it, records the trail.
//
// THE SHAPE IS VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never
// hold the counter's row lock while doing so — that lock serialises every booking in the commune —
// and the caller needs the reason rather than a rollback.
//
// THE DEADLINE IS FETCHED BEFORE THE TRANSACTION OPENS TOO, AND THAT IS NOT A STYLE CHOICE. It is a
// gRPC call to another service; made inside the transaction it would hold the counter row for the
// length of a network round trip, so every booking in the commune would queue behind identity's
// latency, and an identity outage would become a register that hangs rather than one that refuses.
//
// `nam` IS THE YEAR OF THE ACT, not the year on the document. A register is opened per year and a
// number is given when the document is BOOKED: a letter dated 31/12/2026 that reaches the office on
// 02/01/2027 takes số 1/2027, with `ngay_den` and `ngay_van_ban` saying the rest. Taking the year
// from `ngay_den` instead would insert a number into a series that was closed — an issued number
// appearing in a year's register after that year ended (rule 7). ⚠ THIS IS A READING OF
// ADMINISTRATIVE PRACTICE, not a line of the specification, and it is reported as such.
func (uc *IncomingDocuments) Register(ctx context.Context, req RegisterIncomingDocumentRequest,
	actor audit.Actor) (domain.IncomingDocument, error) {

	now := uc.clock()

	doc, err := normalizeRegistration(req, now)
	if err != nil {
		return domain.IncomingDocument{}, err
	}
	if err := requireDocumentActor(actor); err != nil {
		return domain.IncomingDocument{}, err
	}

	id, err := uc.newID()
	if err != nil {
		return domain.IncomingDocument{}, fmt.Errorf("van_ban_den: sinh mã: %w", err)
	}
	doc.ID = id
	doc.Year = now.Year()
	doc.Status = domain.IncomingStatusRegistered
	// THE STAFF BUSINESS CODE, never the internal id — audit.Actor.ID already carries exactly that
	// value, because the handler reads `Principal.Ma` (rule 6, invariant 8).
	doc.CreatedBy = actor.ID

	due, err := uc.resolveDueAt(ctx, now)
	if err != nil {
		return domain.IncomingDocument{}, err
	}
	doc.DueAt = due

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE TYPE IS CHECKED FIRST, BEFORE THE NUMBER IS TAKEN. A booking that is going to be
		// refused must not consume a number: the counter only moves forward, so a number taken by a
		// request that then fails would leave a permanent hole in the commune's register. The order
		// of these two statements is the whole of that property.
		if err := uc.repo.DocumentTypeInUse(ctx, tx, doc.DocumentType); err != nil {
			return err
		}
		no, err := uc.series.IssueNumber(ctx, tx, docstore.RegisterIncoming, doc.Year)
		if err != nil {
			return err
		}
		doc.ArrivalNo = no

		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		if err := uc.repo.Insert(ctx, tx, doc); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": summarizeIncoming(doc)})
		if err != nil {
			return fmt.Errorf("van_ban_den: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT AND AS THE NUMBER (rule 6, invariant 3). TenantID is left
		// unset on purpose: audit.Write fills it from the transaction, which took it from the
		// context (rule 1, invariant 4). Passing it here would be a second source for the one fact
		// that decides which commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionRegisterIncomingDocument,
			Subject: domain.IncomingDocumentCode(doc.Year, doc.ArrivalNo),
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no document, no number, no trail. The three states agree.
		return domain.IncomingDocument{}, wrapDocumentErr(ctx, "vào sổ văn bản đến", err)
	}
	return doc, nil
}

// resolveDueAt asks identity for the one clock this register fixes.
//
// `DEADLINE_KIND_XU_LY_XONG` ALONE, AND `DEADLINE_KIND_TIEP_NHAN` DELIBERATELY NOT ASKED FOR. The
// commune's `sla` row for `van-ban-den` carries both figures, and the acknowledge clock measures how
// long a record waits before a human reads it — but an incoming document is BOOKED BY a member of
// staff, so the act that creates the row IS the reading (ADR 0028 decision E, and the same reasoning
// kb/00-foundation/ubiquitous-language.md:75 states for staff-booked petitions). Asking for it would
// store a commitment that was met at the instant it was created.
//
// ⚠ IF THE CUSTOMER MEANS "8 working hours from the date on the document until somebody books it",
// THAT IS A DIFFERENT ORIGIN and a different act, and it is a stop condition (rule 10) rather than a
// second argument here: counting from a date a clerk types would let a document be booked already
// overdue.
//
// THE FIELD CODE IS "" — THE DEFAULT ROW. A document register has no `lĩnh vực` concept anywhere in
// the specification, so there is no per-field row to read. It is passed straight through, never
// substituted for some "general" code.
func (uc *IncomingDocuments) resolveDueAt(ctx context.Context, from time.Time) (time.Time, error) {
	due, err := uc.deadlines.HanXuLy(ctx, identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN, "", from,
		[]identityv1.DeadlineKind{identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG})
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %w", ErrDeadlineNotEstablished, err)
	}
	t, ok := due[identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG]
	if !ok || t.IsZero() {
		// identityclient already refuses a partial answer; this is the second wall, and it is here
		// because a zero time.Time reaching the column is a deadline in the year 1 — a document
		// overdue the moment it is booked.
		return time.Time{}, fmt.Errorf("%w: hạn xử lý xong rỗng", ErrDeadlineNotEstablished)
	}
	return t, nil
}

// Update corrects one booked document.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending a document the values it already has is not an
// event; recording it would fill a public authority's ledger with entries saying nothing changed,
// and those are the entries that bury the ones carrying legal weight. It is also what makes this
// route genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
//
// THE NUMBER, THE YEAR, THE STATE AND THE DEADLINE ARE NOT REACHABLE FROM HERE — see the store's
// updateIncomingDocument for which rule each absence serves.
func (uc *IncomingDocuments) Update(ctx context.Context, id string, req UpdateIncomingDocumentRequest,
	actor audit.Actor) (domain.IncomingDocument, error) {

	if id == "" {
		return domain.IncomingDocument{}, docstore.ErrIncomingDocumentNotFound
	}
	if err := requireDocumentActor(actor); err != nil {
		return domain.IncomingDocument{}, err
	}
	now := uc.clock()

	var after domain.IncomingDocument
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}

		after = before
		if err := applyIncomingUpdate(&after, req, now); err != nil {
			return err
		}
		if incomingUnchanged(before, after) {
			return nil
		}
		if after.DocumentType != before.DocumentType {
			if err := uc.repo.DocumentTypeInUse(ctx, tx, after.DocumentType); err != nil {
				return err
			}
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the UPDATE.
		if err := uc.repo.Update(ctx, tx, after); err != nil {
			return err
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to find
		// in a ledger that is never deleted.
		delta, err := json.Marshal(map[string]any{
			"van_ban_id": after.ID,
			"truoc":      summarizeIncomingChange(before, after, true),
			"sau":        summarizeIncomingChange(before, after, false),
		})
		if err != nil {
			return fmt.Errorf("van_ban_den: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionUpdateIncomingDocument,
			Subject: domain.IncomingDocumentCode(before.Year, before.ArrivalNo),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.IncomingDocument{}, wrapDocumentErr(ctx, "sửa văn bản đến", err)
	}
	return after, nil
}

// Remove soft deletes one entry.
//
// THIS IS NOT A DELETE AND THE NAME IS THE ONLY PLACE THAT COULD SUGGEST OTHERWISE. The row stays,
// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1); it leaves every
// screen because every read path excludes soft-deleted rows, not because anything was destroyed.
//
// THE NUMBER DOES NOT COME BACK. Nothing here touches the counter, and the unique key still counts
// this row — so the next document takes the NEXT number and the removed one leaves a visible gap.
// That gap IS the record: an archival register that silently closed its own holes would be a
// register nobody could audit.
//
// THE REASON IS MANDATORY. An entry that vanished from the register with no reason attached is a
// document nobody can account for — and the row is still there, so the question WILL be asked.
func (uc *IncomingDocuments) Remove(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return docstore.ErrIncomingDocumentNotFound
	}
	reason, err := domain.NormalizeRequired(rawReason, domain.MaxRemovalReasonLen, domain.ErrMissingRemovalReason)
	if err != nil {
		return err
	}
	if err := requireDocumentActor(actor); err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds. Two
		// kinds of identifier in one column is a column nobody can query (rule 6, invariant 8).
		if err := uc.repo.SoftDelete(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication rule 9
		// forbids: the column is the current state of the row and can only ever hold the FIRST
		// removal, while the entry is the append-only record of the act. They answer two different
		// questions and neither can be derived from the other.
		delta, err := json.Marshal(map[string]any{
			"van_ban_id": before.ID,
			"truoc":      summarizeIncoming(before),
			"ly_do":      reason,
			"xoa_mem":    true,
		})
		if err != nil {
			return fmt.Errorf("van_ban_den: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionRemoveIncomingDocument,
			Subject: domain.IncomingDocumentCode(before.Year, before.ArrivalNo),
			Delta:   delta,
		})
	})
	if err != nil {
		return wrapDocumentErr(ctx, "gỡ văn bản đến", err)
	}
	return nil
}

// Route routes a document to a department — the chairman's instruction and the office's handover in
// one act (§3.5, permission `document.route`).
//
// THREE WRITES, ONE TRANSACTION, AND THAT IS THE WHOLE DESIGN: the document moves, the timeline
// records the move, and the audit trail records who did it. Split across transactions, any window
// between them is a register whose "Đang giữ" column and whose timeline disagree — and the timeline
// is append-only, so there is no repair afterwards.
//
// THE TIMELINE ENTRY IS A BUSINESS RECORD, NOT A SECOND AUDIT LOG. A clerk reads it on the screen;
// `audit_log` is invisible to the commune and answers a different question. Both are written.
func (uc *IncomingDocuments) Route(ctx context.Context, id string, req RouteDocumentRequest,
	actor audit.Actor) (domain.IncomingDocument, error) {

	if id == "" {
		return domain.IncomingDocument{}, docstore.ErrIncomingDocumentNotFound
	}
	toOrgUnitID, err := domain.NormalizeRequired(req.ToOrgUnitID, domain.MaxOrgUnitIDLen, domain.ErrMissingTargetOrgUnit)
	if err != nil {
		return domain.IncomingDocument{}, err
	}
	// THE REASON IS MANDATORY, and that is a decision this session makes rather than one the
	// specification states: §3.5 draws the field with a placeholder and does not mark it required.
	// It is required here because the entry can never be edited afterwards (rule 7, forbidden #5) —
	// a routing with no reason is an instruction nobody can account for, and the person who gave it
	// will have moved on by the time anybody asks.
	reason, err := domain.NormalizeRequired(req.Reason, domain.MaxRoutingReasonLen, domain.ErrMissingRoutingReason)
	if err != nil {
		return domain.IncomingDocument{}, err
	}
	assigneeCode, err := domain.NormalizeOptional(req.AssigneeCode, domain.MaxStaffCodeLen)
	if err != nil {
		return domain.IncomingDocument{}, err
	}
	if err := requireDocumentActor(actor); err != nil {
		return domain.IncomingDocument{}, err
	}

	at := uc.clock()
	routingID, err := uc.newID()
	if err != nil {
		return domain.IncomingDocument{}, fmt.Errorf("van_ban_den: sinh mã lịch sử: %w", err)
	}

	var after domain.IncomingDocument
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := before.CanRoute(); err != nil {
			return err
		}

		status := domain.StatusAfterRouting(before.Status)
		if err := uc.repo.RouteToOrgUnit(ctx, tx, before.ID, toOrgUnitID, assigneeCode, status); err != nil {
			return err
		}
		if err := uc.repo.InsertRouting(ctx, tx, domain.DocumentRouting{
			ID:                 routingID,
			IncomingDocumentID: before.ID,
			RoutedAt:           at,
			RoutedBy:           actor.ID,
			StatusAtTime:       status,
			FromOrgUnitID:      before.HoldingOrgUnitID,
			ToOrgUnitID:        toOrgUnitID,
			AssigneeCode:       assigneeCode,
			Instruction:        reason,
		}); err != nil {
			return err
		}

		after = before
		after.HoldingOrgUnitID = toOrgUnitID
		after.AssigneeCode = assigneeCode
		after.Status = status

		delta, err := json.Marshal(map[string]any{
			"van_ban_id": before.ID,
			"truoc": map[string]any{
				"trang_thai":   string(before.Status),
				"bo_phan_giu":  before.HoldingOrgUnitID,
				"can_bo_xu_ly": before.AssigneeCode,
			},
			"sau": map[string]any{
				"trang_thai":   string(after.Status),
				"bo_phan_giu":  after.HoldingOrgUnitID,
				"can_bo_xu_ly": after.AssigneeCode,
			},
			"ly_do": reason,
		})
		if err != nil {
			return fmt.Errorf("van_ban_den: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionRouteIncomingDocument,
			Subject: domain.IncomingDocumentCode(before.Year, before.ArrivalNo),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.IncomingDocument{}, wrapDocumentErr(ctx, "chuyển văn bản đến", err)
	}
	return after, nil
}

// --- the detail drawer (§3.5) --------------------------------------------------------------------
//
// READS, SO NO AUDIT ENTRY: rule 6, invariant 7 audits reading FULL personal data or reading ACROSS
// communes, and these do neither — the register carries no masked citizen field, and the commune is
// bound from the context.

// Detail reads one live document of this commune.
//
// A TRANSACTION FOR ONE SELECT only so the store has one signature shape (IncomingDocumentStore). An
// empty id is the same not-found as any other: nothing distinguishes "no such id" from "not yours".
func (uc *IncomingDocuments) Detail(ctx context.Context, id string) (domain.IncomingDocument, error) {
	if id == "" {
		return domain.IncomingDocument{}, docstore.ErrIncomingDocumentNotFound
	}
	var d domain.IncomingDocument
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		d, err = uc.repo.ByID(ctx, tx, id)
		return err
	})
	if err != nil {
		return domain.IncomingDocument{}, wrapDocumentErr(ctx, "đọc văn bản đến", err)
	}
	return d, nil
}

// RoutingHistory reads one document's routing timeline, oldest first.
//
// THE DOCUMENT IS READ FIRST, AND THAT READ IS THE ISOLATION. `lich_su_chuyen_van_ban` has no
// soft-delete columns and no foreign key, so without it a removed document's timeline would still
// be served (rule 7, invariant 2), and an unknown id would answer 200 with an empty list instead of
// the 404 the document route gives — two answers for one fact.
func (uc *IncomingDocuments) RoutingHistory(ctx context.Context, id string) ([]domain.DocumentRouting, error) {
	if id == "" {
		return nil, docstore.ErrIncomingDocumentNotFound
	}
	var out []domain.DocumentRouting
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		d, err := uc.repo.ByID(ctx, tx, id)
		if err != nil {
			return err
		}
		out, err = uc.repo.RoutingHistory(ctx, tx, d.ID)
		return err
	})
	if err != nil {
		return nil, wrapDocumentErr(ctx, "đọc lịch sử chuyển văn bản đến", err)
	}
	return out, nil
}

// --- shape ---------------------------------------------------------------------------------------

// normalizeRegistration validates and trims one booking request. OUTSIDE THE TRANSACTION — see
// Register.
func normalizeRegistration(req RegisterIncomingDocumentRequest, now time.Time) (domain.IncomingDocument, error) {
	var d domain.IncomingDocument
	var err error

	if err = domain.ValidateReceivedDate(req.ReceivedDate, now); err != nil {
		return domain.IncomingDocument{}, err
	}
	if err = domain.ValidateIncomingDocumentDate(req.DocumentDate, req.ReceivedDate); err != nil {
		return domain.IncomingDocument{}, err
	}
	if err = domain.ValidateUrgency(req.Urgency); err != nil {
		return domain.IncomingDocument{}, err
	}
	if d.IssuingBody, err = domain.NormalizeRequired(req.IssuingBody, domain.MaxIssuingBodyLen,
		domain.ErrMissingIssuingBody); err != nil {
		return domain.IncomingDocument{}, err
	}
	if d.DocumentType, err = domain.NormalizeRequired(req.DocumentType, domain.MaxDocumentTypeLen,
		domain.ErrMissingDocumentType); err != nil {
		return domain.IncomingDocument{}, err
	}
	if d.Summary, err = domain.NormalizeRequired(req.Summary, domain.MaxSummaryLen,
		domain.ErrMissingSummary); err != nil {
		return domain.IncomingDocument{}, err
	}
	if d.ReferenceNo, err = domain.NormalizeOptional(req.ReferenceNo, domain.MaxReferenceNoLen); err != nil {
		return domain.IncomingDocument{}, err
	}
	d.ReceivedDate = req.ReceivedDate
	d.DocumentDate = req.DocumentDate
	d.Urgency = req.Urgency
	return d, nil
}

// applyIncomingUpdate applies a partial edit onto the row that was read under the lock.
func applyIncomingUpdate(d *domain.IncomingDocument, req UpdateIncomingDocumentRequest, now time.Time) error {
	if req.ReceivedDate != nil {
		if err := domain.ValidateReceivedDate(*req.ReceivedDate, now); err != nil {
			return err
		}
		d.ReceivedDate = *req.ReceivedDate
	}
	if req.DocumentDate != nil {
		if err := domain.ValidateIncomingDocumentDate(*req.DocumentDate, d.ReceivedDate); err != nil {
			return err
		}
		d.DocumentDate = *req.DocumentDate
	}
	if req.Urgency != nil {
		if err := domain.ValidateUrgency(*req.Urgency); err != nil {
			return err
		}
		d.Urgency = *req.Urgency
	}
	if req.IssuingBody != nil {
		s, err := domain.NormalizeRequired(*req.IssuingBody, domain.MaxIssuingBodyLen, domain.ErrMissingIssuingBody)
		if err != nil {
			return err
		}
		d.IssuingBody = s
	}
	if req.DocumentType != nil {
		s, err := domain.NormalizeRequired(*req.DocumentType, domain.MaxDocumentTypeLen, domain.ErrMissingDocumentType)
		if err != nil {
			return err
		}
		d.DocumentType = s
	}
	if req.Summary != nil {
		s, err := domain.NormalizeRequired(*req.Summary, domain.MaxSummaryLen, domain.ErrMissingSummary)
		if err != nil {
			return err
		}
		d.Summary = s
	}
	if req.ReferenceNo != nil {
		s, err := domain.NormalizeOptional(*req.ReferenceNo, domain.MaxReferenceNoLen)
		if err != nil {
			return err
		}
		d.ReferenceNo = s
	}
	return nil
}

// incomingUnchanged reports whether the edit would change nothing. The number, the year, the state,
// the deadline and the holding department are not compared because no path through Update can move
// them.
func incomingUnchanged(before, after domain.IncomingDocument) bool {
	return before.ReceivedDate.Equal(after.ReceivedDate) &&
		before.DocumentDate.Equal(after.DocumentDate) &&
		before.ReferenceNo == after.ReferenceNo &&
		before.IssuingBody == after.IssuingBody &&
		before.DocumentType == after.DocumentType &&
		before.Summary == after.Summary &&
		before.Urgency == after.Urgency
}

// summarizeIncoming is the audit delta's view of one entry.
//
// ⚠ `trich_yeu` IS FREE TEXT AND MAY NAME A CITIZEN (rule 3). An incoming document is ordinarily
// official correspondence between bodies, but a commune that types "Đơn của ông Nguyễn Văn A, số
// điện thoại …" into the summary has put personal data into an append-only ledger that is never
// deleted. THE SUMMARY IS STILL RECORDED, because an audit entry that cannot say WHICH document was
// removed is an entry nobody can use — and it is the field an edit is most likely to touch. What
// this layer can guarantee is narrower and is stated rather than implied: nothing here logs it, and
// no error message carries it out to a client.
//
// THE DELTA KEYS ARE THE OLD COLUMN NAMES AND STAY SO: `audit_log.delta` is append-only, and ADR 0061
// keeps them as JSON keys, read through the glossary's §Từ điển đổi tên.
func summarizeIncoming(d domain.IncomingDocument) map[string]any {
	return map[string]any{
		"van_ban_id":       d.ID,
		"so_vao_so":        d.ArrivalNo,
		"nam":              d.Year,
		"ngay_den":         d.ReceivedDate.Format("2006-01-02"),
		"so_ky_hieu":       d.ReferenceNo,
		"co_quan_ban_hanh": d.IssuingBody,
		"loai_van_ban":     d.DocumentType,
		"trich_yeu":        d.Summary,
		"do_khan":          string(d.Urgency),
		"trang_thai":       string(d.Status),
		"han_xu_ly_xong":   d.DueAt.UTC().Format(time.RFC3339),
	}
}

// summarizeIncomingChange returns only the fields that actually moved, from whichever side is asked
// for.
func summarizeIncomingChange(before, after domain.IncomingDocument, beforeSide bool) map[string]any {
	out := map[string]any{}
	if !before.ReceivedDate.Equal(after.ReceivedDate) {
		out["ngay_den"] = pickDate(beforeSide, before.ReceivedDate, after.ReceivedDate)
	}
	if !before.DocumentDate.Equal(after.DocumentDate) {
		out["ngay_van_ban"] = pickDate(beforeSide, before.DocumentDate, after.DocumentDate)
	}
	if before.ReferenceNo != after.ReferenceNo {
		out["so_ky_hieu"] = pickString(beforeSide, before.ReferenceNo, after.ReferenceNo)
	}
	if before.IssuingBody != after.IssuingBody {
		out["co_quan_ban_hanh"] = pickString(beforeSide, before.IssuingBody, after.IssuingBody)
	}
	if before.DocumentType != after.DocumentType {
		out["loai_van_ban"] = pickString(beforeSide, before.DocumentType, after.DocumentType)
	}
	if before.Summary != after.Summary {
		out["trich_yeu"] = pickString(beforeSide, before.Summary, after.Summary)
	}
	if before.Urgency != after.Urgency {
		out["do_khan"] = pickString(beforeSide, string(before.Urgency), string(after.Urgency))
	}
	return out
}

func pickString(beforeSide bool, a, b string) string {
	if beforeSide {
		return a
	}
	return b
}

// pickDate formats the chosen side, and renders the zero date as "" rather than as the year 1 — an
// audit entry saying a date was cleared must not say it was set to 0001-01-01.
func pickDate(beforeSide bool, a, b time.Time) string {
	t := b
	if beforeSide {
		t = a
	}
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// requireDocumentActor refuses a write whose trail cannot name who made it.
//
// core/audit refuses an entry with no actor for the same reason; refusing HERE, before the
// transaction opens, keeps `nguoi_tao_ma` / `deleted_by` / `nguoi_ma` and the entry telling the same
// story — and avoids a rollback whose cause is a missing principal rather than anything about the
// document. Rule 6 does not permit a business write whose trail cannot name its author.
func requireDocumentActor(actor audit.Actor) error {
	if actor.ID == "" {
		return fmt.Errorf("van_ban: thiếu người thực hiện")
	}
	return nil
}

// wrapDocumentErr wraps a failure with the commune and the operation, and NOTHING ELSE.
//
// No summary, no issuing body, no document number: an error travels into centralised logging across
// every commune at once, and the summary of an incoming document is free text about a government
// matter that may name a citizen (rule 3). The commune is not personal data and is the one thing an
// operator can act on.
//
// The chain is kept with %w so the handler can still tell a refusal from a failure with errors.Is. A
// %v here would collapse "this document is already settled" and "the database is down" into one 500.
func wrapDocumentErr(ctx context.Context, op string, err error) error {
	return fmt.Errorf("van_ban: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}
