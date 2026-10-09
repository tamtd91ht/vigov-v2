package app

// The use cases of SỔ ĐƠN THƯ CÔNG DÂN (ADR 0039, ADR 0078 #2–#4; migration 0006).
//
// WHY THIS LAYER: rule 6, invariant 3 — the business write, its log row and its audit entry share ONE
// transaction, and core/audit.Write takes a *store.ScopedTx with no overload outside one. Booking adds
// the number: allocate it under the counter's row lock, insert the letter, write the trail — one
// transaction, or the register holds a number nobody used.
//
// THE DATABASE IS THE FLOOR AND THIS LAYER IS THE SENTENCE (van_ban_den.go's header). 0006 closes the
// status set, binds `accepted_at` / `resolved_at`, refuses '' on optional columns, freezes the number
// and keeps the log append-only. What it cannot do is the C3 ARROWS and the C13/C14 "assignee or
// petition.create" rule — those live here and in domain/, and refuse FIRST, in Vietnamese.
//
// IDENTITY IS ASKED OUTSIDE THE TRANSACTION. A unit id and an assignee code arrive in a request body,
// so they are checked against identity (LiveOrgUnits, CanBoGiaoViecDuoc) before the row lock is taken:
// a network round trip inside the transaction would hold the letter (or the commune's counter) for its
// duration. An identity error is never "not live" and never "live" — the write is refused with 503.
//
// THE COMMUNE: every store call below runs on a *store.ScopedTx or a context whose tenant_id the store
// binds as $1 (store.DB.For(ctx) / tx.TenantID()). No method here takes a commune as an argument.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// CitizenLetterRepo is the register's storage, declared at the point of use. Every write takes the
// transaction, so no signature here could write the letter in one transaction and the trail in another.
// *docstore.CitizenLetterStore satisfies it.
type CitizenLetterRepo interface {
	ByID(ctx context.Context, tx *store.ScopedTx, id string) (domain.CitizenLetter, error)
	ForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.CitizenLetter, error)
	Insert(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter) error
	UpdateHolder(ctx context.Context, tx *store.ScopedTx, id, unitID, assigneeCode, actorCode string) error
	UpdateStatus(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter, actorCode string) error
	UpdateResult(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter, actorCode string) error
	UpdateSender(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter, actorCode string) error
	UpdateDeadline(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter, actorCode string) error
	InsertLog(ctx context.Context, tx *store.ScopedTx, e domain.LetterLogEntry) error
	Log(ctx context.Context, tx *store.ScopedTx, letterID string) ([]domain.LetterLogEntry, error)

	List(ctx context.Context, f docstore.CitizenLetterFilter, req page.Request) (page.Result[domain.CitizenLetter], error)
	Count(ctx context.Context, f docstore.CitizenLetterFilter) (int, error)
	DuplicateCandidates(ctx context.Context, senderName string, since time.Time) ([]domain.CitizenLetter, error)
	ReportRows(ctx context.Context, year int, yearStart time.Time) ([]domain.CitizenLetter, error)
}

// LetterDirectory is the slice of identity this register needs. *identityclient.Client satisfies it.
//
// LiveOrgUnits and CanBoGiaoViecDuoc DECIDE (an absent id is refused). StaffOrgUnits NARROWS the
// `related` tab and grants nothing — its contract forbids using it in a guard (identityclient).
type LetterDirectory interface {
	LiveOrgUnits(ctx context.Context, ids []string) (map[string]struct{}, error)
	// vi-name-ok: mirrors the existing core/identityclient.Client method so *Client satisfies this interface
	CanBoGiaoViecDuoc(ctx context.Context, ma []string) (map[string]struct{}, error)
	StaffOrgUnits(ctx context.Context, staffCode string) ([]string, error)
	// ResolveCitizenLetterDeadline is ADR 0085 B: (due, true, nil) a deadline; (nil, false, nil) the
	// commune has no rule — "Không đặt hạn"; any error REFUSES the act (letterDeadline below).
	ResolveCitizenLetterDeadline(ctx context.Context, letterType identityv1.CitizenLetterType,
		kind identityv1.CitizenLetterDeadlineKind, countFrom time.Time) (*time.Time, bool, error)
}

// ErrLetterDirectoryUnavailable: identity could not answer, so the check did not happen and nothing
// was written. The edge answers 503 (retryable). The cause rides in the wrapped chain.
var ErrLetterDirectoryUnavailable = errors.New("citizen_letter: không kiểm được bộ phận / cán bộ với identity")

// ErrLetterDeadlineUnavailable: identity could not say when the letter falls due, so the act (booking,
// or the move to `thu-ly`) was refused and nothing was written — no number taken, no status moved. The
// edge answers 503 (retryable). NEVER turned into "no deadline": a letter without a deadline counts as
// ON TIME in the report (ADR 0084 #4), so storing NULL on an outage would inflate the figure.
var ErrLetterDeadlineUnavailable = errors.New("citizen_letter: chưa hỏi được hạn đơn thư với identity")

// ErrLetterDeadlineUnusable: the commune HAS a deadline rule for this letter type but it cannot be used
// (identityclient.ErrCitizenLetterDeadlineUnusable). A configuration fault, not an outage: the edge
// answers 409 with a sentence sending the clerk to the configuration, and nothing was written.
var ErrLetterDeadlineUnusable = errors.New("citizen_letter: quy tắc hạn đơn thư của xã không dùng được")

// ErrLetterScopeInvalid — a `scope` outside all/mine/related. The edge validates first; this is the
// second wall.
var ErrLetterScopeInvalid = errors.New("citizen_letter: phạm vi không hợp lệ")

// The business verbs written into the trail — Vietnamese snake_case like every action this system
// already writes (`vao_so_van_ban_den`): an inspection reads these strings.
const (
	ActionBookCitizenLetter         = "vao_so_don_thu"
	ActionRouteCitizenLetter        = "chuyen_don_thu"
	ActionMoveCitizenLetter         = "doi_trang_thai_don_thu"
	ActionRecordCitizenLetterResult = "ghi_ket_qua_don_thu"
	ActionCorrectLetterSender       = "sua_nguoi_gui_don_thu"
	ActionNoteCitizenLetter         = "ghi_nhat_ky_don_thu"
	ActionSetLetterDeadline         = "dat_han_xu_ly_don_thu"
	// ActionViewDenunciation — rule 6, invariant 7: reading a whistleblower's identity or the letter's
	// content is itself recorded, in the transaction of the read.
	ActionViewDenunciation = "xem_don_to_cao"
)

// Fixed log sentences. A `sua-nguoi-gui` row records THAT the sender was corrected, never the values
// (0006:419-422; rule 6, forbidden #4).
//
// logAssignedAtBooking is the prototype's sentence for a unit chosen on the booking form (ADR 0084 #2:
// the letter stays `moi-vao-so` and its group derives "Đã phân công"). The row's from-unit stays EMPTY,
// so the timeline prints "Một cửa → <unit>" — the letter came from the reception desk, not a unit.
const (
	logSenderCorrected   = "Sửa thông tin người gửi"
	logAssignedAtBooking = "Phân công xử lý"
)

// LetterCaller is the acting member of staff. Actor.ID IS THE STAFF BUSINESS CODE (Principal.Ma —
// rule 6, invariant 8). CanBook says whether the caller holds `petition.create`, asked of the checker
// by the edge — the second half of C13/C14's "assignee OR petition.create".
type LetterCaller struct {
	Actor   audit.Actor
	CanBook bool
}

// BookLetterRequest is one letter as it arrives. NO number, year, status or due field: the number is
// the counter's, the year is the year of the act, the status a literal, and the processing deadline
// is identity's answer for the commune's rule of this letter type (ADR 0084 #3, ADR 0085 B) — never
// the client's. A clerk may change it afterwards, by its own act (SetDeadline).
//
// Source is NOT the client's: each booking PATH states its own (the booking route `nhap-tay`, the
// Excel import `nhap-excel`). Required — an empty one is refused, never defaulted.
type BookLetterRequest struct {
	Source          domain.LetterSource
	ReceivedDate    time.Time
	Type            domain.LetterType
	SenderName      string
	SenderPhone     string
	SenderAddress   string
	Summary         string
	RelatedLetterID string // C11: set only when the clerk CONFIRMED the duplicate warning
	HoldingUnitID   string // "Chuyển ngay cho bộ phận"
}

// RouteLetterRequest is "Chuyển xử lý".
type RouteLetterRequest struct {
	ToUnitID     string
	AssigneeCode string // optional — "để bộ phận tự phân công"
	Reason       string
}

// MoveLetterRequest is a status change.
type MoveLetterRequest struct {
	Status domain.LetterStatus
	Note   string
}

// LetterResultRequest is C10's result — a full replacement.
type LetterResultRequest struct {
	DocumentNo   string
	DocumentDate time.Time
	Signer       string
	Issuer       string
	Summary      string
}

// SenderCorrection is a PARTIAL edit: a nil pointer leaves the field alone, a pointer to "" clears it.
type SenderCorrection struct {
	Name    *string
	Phone   *string
	Address *string
}

// LetterListQuery is the list request after the edge validated it. Scope is "", "all", "mine" or
// "related"; the caller's code comes from the SESSION.
type LetterListQuery struct {
	Filter     docstore.CitizenLetterFilter
	Scope      string
	CallerCode string
}

// DuplicateQuery is the body of the duplicate check.
type DuplicateQuery struct {
	SenderName string
	Summary    string
}

// CitizenLetters owns every act on one commune's citizen-letter register.
type CitizenLetters struct {
	db     *store.DB
	repo   CitizenLetterRepo
	series KhoDaySo
	dir    LetterDirectory
	// notices is the staff-notice outbox: Route with a named officer writes its row in the routing's own
	// transaction (ADR 0086 A1). Required — writeStaffNotice fails the act closed without it.
	notices StaffNoticeOutbox

	newID func() (string, error) // ulid.Moi in production; a seam so a test can pin ids
	now   func() time.Time       // nil = the real clock, UTC
}

func NewCitizenLetters(db *store.DB, repo CitizenLetterRepo, series KhoDaySo, dir LetterDirectory,
	notices StaffNoticeOutbox) *CitizenLetters {
	return &CitizenLetters{db: db, repo: repo, series: series, dir: dir, notices: notices, newID: ulid.Moi}
}

func (uc *CitizenLetters) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

// --- booking ----------------------------------------------------------------------------------------

// Book allocates the next `don-thu` number of THIS commune for THE YEAR OF THE ACT, stores the letter,
// writes a `luan-chuyen` log row when it was routed at booking, and the audit entry — one transaction.
//
// `year` IS THE YEAR OF THE BOOKING ACT in Asia/Ho_Chi_Minh, not of `received_date` (C-list; 0006:32-36):
// a letter received 30/12 and booked 02/01 takes the new year's series.
//
// THE PROCESSING DEADLINE IS ASKED BEFORE THE TRANSACTION (ADR 0085 B): counted from 00:00
// Asia/Ho_Chi_Minh of `received_date`, fixed here once and stored (rule 10, invariant 2). Not
// configured → NULL, "Không đặt hạn". Any error → the booking is refused before a number is taken.
func (uc *CitizenLetters) Book(ctx context.Context, req BookLetterRequest, caller LetterCaller) (domain.CitizenLetter, error) {
	b, err := uc.prepareBooking(ctx, req, caller)
	if err != nil {
		return domain.CitizenLetter{}, err
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return uc.bookInTx(ctx, tx, &b, caller)
	})
	if err != nil {
		return domain.CitizenLetter{}, wrapLetter(ctx, "vào sổ đơn thư", err)
	}
	return b.letter, nil
}

// preparedBooking is one letter Book's first half made ready: shape checked, unit checked live,
// deadline fixed by identity, ids drawn — everything that happens OUTSIDE the transaction.
type preparedBooking struct {
	letter domain.CitizenLetter
	logID  string // "" unless a unit was chosen at booking
	at     time.Time
}

// prepareBooking is Book's first half, and the Excel import's (citizen_letter_import.go): ONE copy of
// the booking rules, so a letter booked from a file cannot differ from one typed at the counter. Every
// identity round trip happens here, before any transaction opens (the file header's reason).
func (uc *CitizenLetters) prepareBooking(ctx context.Context, req BookLetterRequest, caller LetterCaller) (preparedBooking, error) {
	now := uc.clock()
	l, err := normaliseBooking(req, now)
	if err != nil {
		return preparedBooking{}, err
	}
	if err := requireActor(caller.Actor); err != nil {
		return preparedBooking{}, err
	}
	if l.HoldingUnitID != "" {
		if err := uc.checkUnit(ctx, l.HoldingUnitID); err != nil {
			return preparedBooking{}, err
		}
	}
	// The received DATE carries no zone of its own (a DATE column, parsed as a calendar day): its
	// Y-M-D is the Vietnamese calendar day, so midnight is built in Vietnam's zone, never converted.
	y, m, d := l.ReceivedDate.Date()
	countFrom := time.Date(y, m, d, 0, 0, 0, 0, domain.AutomationZone)
	if l.ProcessingDueAt, err = uc.letterDeadline(ctx, l.Type,
		identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_PROCESSING, countFrom); err != nil {
		return preparedBooking{}, wrapLetter(ctx, "tính hạn xử lý đơn thư", err)
	}
	if l.ID, err = uc.newID(); err != nil {
		return preparedBooking{}, fmt.Errorf("citizen_letter: sinh mã: %w", err)
	}
	var logID string
	if l.HoldingUnitID != "" {
		if logID, err = uc.newID(); err != nil {
			return preparedBooking{}, fmt.Errorf("citizen_letter: sinh mã nhật ký: %w", err)
		}
	}
	l.Year = now.In(domain.AutomationZone).Year()
	l.Status = domain.LetterStatusNew
	l.CreatedByCode = caller.Actor.ID
	l.CreatedAt, l.UpdatedAt = now, now
	return preparedBooking{letter: l, logID: logID, at: now}, nil
}

// bookInTx is Book's second half, inside the caller's transaction: link check, number, row, log row,
// audit entry. b.letter carries the number when it returns nil.
func (uc *CitizenLetters) bookInTx(ctx context.Context, tx *store.ScopedTx, b *preparedBooking, caller LetterCaller) error {
	l := &b.letter
	// THE LINK IS CHECKED BEFORE THE NUMBER IS TAKEN — a booking that will be refused does not
	// touch the counter at all.
	if l.RelatedLetterID != "" {
		if _, err := uc.repo.ByID(ctx, tx, l.RelatedLetterID); err != nil {
			if errors.Is(err, docstore.ErrCitizenLetterNotFound) {
				return domain.ErrLetterRelatedNotFound
			}
			return err
		}
	}
	// The counter row is (tenant_id from tx.TenantID(), 'don-thu', year) — one series per commune.
	n, err := uc.series.CapSo(ctx, tx, docstore.SeriesCitizenLetter, l.Year)
	if err != nil {
		return err
	}
	l.Number = n
	// The store binds tenant_id = tx.TenantID() as $1.
	if err := uc.repo.Insert(ctx, tx, *l); err != nil {
		return err
	}
	if l.HoldingUnitID != "" {
		if err := uc.repo.InsertLog(ctx, tx, domain.LetterLogEntry{
			ID: b.logID, LetterID: l.ID, At: b.at, ActorCode: caller.Actor.ID,
			Kind: domain.LetterLogRouting, ToUnitID: l.HoldingUnitID, Content: logAssignedAtBooking,
		}); err != nil {
			return err
		}
	}
	return writeLetterAudit(ctx, tx, caller.Actor, ActionBookCitizenLetter, *l, map[string]any{
		"sau": bookingSummary(*l),
	})
}

func normaliseBooking(req BookLetterRequest, now time.Time) (domain.CitizenLetter, error) {
	var (
		l   domain.CitizenLetter
		err error
	)
	if !req.Source.Valid() {
		return l, domain.ErrLetterSourceInvalid
	}
	if err = domain.CheckReceivedDate(req.ReceivedDate, now); err != nil {
		return l, err
	}
	if !req.Type.Valid() {
		return l, domain.ErrLetterTypeInvalid
	}
	if l.Summary, err = domain.TrimRequired(req.Summary, domain.MaxLetterSummary, domain.ErrLetterSummaryMissing); err != nil {
		return l, err
	}
	if l.SenderName, err = domain.TrimOptional(req.SenderName, domain.MaxSenderName); err != nil {
		return l, err
	}
	if l.SenderPhone, err = domain.TrimPhone(req.SenderPhone); err != nil {
		return l, err
	}
	if l.SenderAddress, err = domain.TrimOptional(req.SenderAddress, domain.MaxSenderAddress); err != nil {
		return l, err
	}
	if l.RelatedLetterID, err = domain.TrimOptional(req.RelatedLetterID, domain.MaxLetterIDLen); err != nil {
		return l, err
	}
	if l.HoldingUnitID, err = domain.TrimOptional(req.HoldingUnitID, domain.MaxLetterUnitID); err != nil {
		return l, err
	}
	l.ReceivedDate = req.ReceivedDate
	l.Type = req.Type
	l.Source = req.Source
	return l, nil
}

// --- routing ----------------------------------------------------------------------------------------

// Route hands the letter to a unit (and optionally an officer). An ATTRIBUTE, never a status (C3): a
// `moi-vao-so` letter stays `moi-vao-so`. A finished letter is not routed (409) — that would reopen a
// closed record by the side effect of a routing form.
func (uc *CitizenLetters) Route(ctx context.Context, id string, req RouteLetterRequest, caller LetterCaller) (domain.CitizenLetter, error) {
	if id == "" {
		return domain.CitizenLetter{}, docstore.ErrCitizenLetterNotFound
	}
	unit, err := domain.TrimRequired(req.ToUnitID, domain.MaxLetterUnitID, domain.ErrLetterUnitMissing)
	if err != nil {
		return domain.CitizenLetter{}, err
	}
	reason, err := domain.TrimRequired(req.Reason, domain.MaxLetterReason, domain.ErrLetterReasonMissing)
	if err != nil {
		return domain.CitizenLetter{}, err
	}
	assignee, err := domain.TrimOptional(req.AssigneeCode, domain.MaxStaffCode)
	if err != nil {
		return domain.CitizenLetter{}, err
	}
	if err := requireActor(caller.Actor); err != nil {
		return domain.CitizenLetter{}, err
	}
	if err := uc.checkUnit(ctx, unit); err != nil {
		return domain.CitizenLetter{}, err
	}
	if assignee != "" {
		if err := uc.checkAssignee(ctx, assignee); err != nil {
			return domain.CitizenLetter{}, err
		}
	}
	now := uc.clock()
	logID, err := uc.newID()
	if err != nil {
		return domain.CitizenLetter{}, fmt.Errorf("citizen_letter: sinh mã nhật ký: %w", err)
	}

	var after domain.CitizenLetter
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if before.Status.Finished() {
			return domain.ErrLetterFinished
		}
		if err := uc.repo.UpdateHolder(ctx, tx, before.ID, unit, assignee, caller.Actor.ID); err != nil {
			return err
		}
		if err := uc.repo.InsertLog(ctx, tx, domain.LetterLogEntry{
			ID: logID, LetterID: before.ID, At: now, ActorCode: caller.Actor.ID,
			Kind: domain.LetterLogRouting, FromUnitID: before.HoldingUnitID, ToUnitID: unit,
			AssigneeCode: assignee, Content: reason,
		}); err != nil {
			return err
		}
		// THE NAMED OFFICER IS TOLD — from the outbox, in THIS transaction (ADR 0086 A1, kind 22), keyed
		// on the log row just written. Title = the register number only; the body stays EMPTY (no
		// summary, no sender, no type — a denunciation must not announce itself, ADR 0078 #4). Booking
		// with a unit and no officer never reaches here, and owes no notice.
		if assignee != "" {
			if err := writeStaffNotice(ctx, tx, uc.notices, uc.newID,
				domain.LetterAssignedNotice(before, logID, assignee), caller.Actor.ID, now); err != nil {
				return err
			}
		}
		after = before
		after.HoldingUnitID, after.AssigneeCode, after.UpdatedAt = unit, assignee, now
		// The reason is free text and may quote the letter: the entry records the move, not the words.
		return writeLetterAudit(ctx, tx, caller.Actor, ActionRouteCitizenLetter, before, map[string]any{
			"truoc": map[string]any{"bo_phan_giu": before.HoldingUnitID, "can_bo_xu_ly": before.AssigneeCode},
			"sau":   map[string]any{"bo_phan_giu": unit, "can_bo_xu_ly": assignee},
		})
	})
	if err != nil {
		return domain.CitizenLetter{}, wrapLetter(ctx, "chuyển đơn thư", err)
	}
	return after, nil
}

// --- status -----------------------------------------------------------------------------------------

// Move changes the status along C3's arrows. WHO: the assignee, or a holder of `petition.create`
// (C13/C14) — decided on the row read under the lock, so a re-assignment racing this request cannot
// let the previous assignee through. `thu-ly` stamps `accepted_at`; `da-giai-quyet` / `dinh-chi` stamp
// `resolved_at`; `da-giai-quyet` needs the result the TYPE requires recorded first (domain
// .CitizenLetter.HasResult — a complaint / denunciation its issued document, a feedback letter /
// request nothing, ADR 0084 #2).
func (uc *CitizenLetters) Move(ctx context.Context, id string, req MoveLetterRequest, caller LetterCaller) (domain.CitizenLetter, error) {
	if id == "" {
		return domain.CitizenLetter{}, docstore.ErrCitizenLetterNotFound
	}
	if !req.Status.Valid() {
		return domain.CitizenLetter{}, domain.ErrLetterStatusInvalid
	}
	note, err := domain.TrimOptional(req.Note, domain.MaxLetterNote)
	if err != nil {
		return domain.CitizenLetter{}, err
	}
	if err := requireActor(caller.Actor); err != nil {
		return domain.CitizenLetter{}, err
	}
	// Truncated to the database's precision: `accepted_at` is the count origin of the resolution
	// deadline, and the instant identity counts from must be the instant stored (ADR 0085 B2).
	now := uc.clock().Truncate(time.Microsecond)
	logID, err := uc.newID()
	if err != nil {
		return domain.CitizenLetter{}, fmt.Errorf("citizen_letter: sinh mã nhật ký: %w", err)
	}
	admission, err := uc.prepareAdmission(ctx, id, req.Status, caller, now)
	if err != nil {
		return domain.CitizenLetter{}, wrapLetter(ctx, "đổi trạng thái đơn thư", err)
	}

	var after domain.CitizenLetter
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := mayWorkOn(before, caller); err != nil {
			return err
		}
		if err := domain.CheckLetterTransition(before.Status, req.Status); err != nil {
			return err
		}
		if req.Status == domain.LetterStatusResolved && !before.HasResult() {
			return domain.ErrLetterNeedsResult
		}
		after = before
		after.Status, after.UpdatedAt = req.Status, now
		if req.Status == domain.LetterStatusAdmitted {
			after.AcceptedAt = now
			if before.Type.HasResolutionDeadline() {
				// The type is fixed at booking and no UPDATE names it; checked anyway, because a
				// deadline asked for one type and stored on another is a commitment nobody made.
				if !admission.asked || admission.letterType != before.Type {
					return fmt.Errorf("citizen_letter: loại đơn đổi giữa lúc hỏi hạn và lúc thụ lý")
				}
				after.ResolutionDueAt = admission.resolutionDue
			}
		}
		if req.Status.EndsResolution() {
			after.ResolvedAt, after.ClosedAt = now, now
		} else if req.Status.Finished() {
			after.ClosedAt = now
		}
		if err := uc.repo.UpdateStatus(ctx, tx, after, caller.Actor.ID); err != nil {
			return err
		}
		if err := uc.repo.InsertLog(ctx, tx, domain.LetterLogEntry{
			ID: logID, LetterID: before.ID, At: now, ActorCode: caller.Actor.ID,
			Kind: domain.LetterLogStatusChange, FromStatus: before.Status, ToStatus: req.Status, Content: note,
		}); err != nil {
			return err
		}
		beforeView := map[string]any{"trang_thai": string(before.Status)}
		afterView := map[string]any{"trang_thai": string(req.Status)}
		if req.Status == domain.LetterStatusAdmitted {
			// The resolution deadline fixed at this act ("" = none: a feedback letter / request, or a
			// commune with no rule). Recorded in full, as SetDeadline records a deadline.
			beforeView["han_giai_quyet"] = instantText(before.ResolutionDueAt)
			afterView["han_giai_quyet"] = instantText(after.ResolutionDueAt)
		}
		return writeLetterAudit(ctx, tx, caller.Actor, ActionMoveCitizenLetter, before, map[string]any{
			"truoc":      beforeView,
			"sau":        afterView,
			"co_ghi_chu": note != "",
		})
	})
	if err != nil {
		return domain.CitizenLetter{}, wrapLetter(ctx, "đổi trạng thái đơn thư", err)
	}
	return after, nil
}

// admissionDeadline is what prepareAdmission learned before Move's transaction.
type admissionDeadline struct {
	asked         bool
	letterType    domain.LetterType
	resolutionDue time.Time // zero = the commune has no rule ("Không đặt hạn")
}

// prepareAdmission asks identity for the RESOLUTION deadline of a complaint / denunciation moving to
// `thu-ly`, counted from `acceptedAt` (ADR 0085 B2) — BEFORE the transaction, so no network round trip
// holds the row lock. The letter's type is only known from the row, so it is read first in a short
// read transaction; the who-may-act and C3 checks run on that read too, so a refused move answers with
// its own sentence instead of a 503 when identity is down. Move re-checks all of it under the lock.
// Any other target status, or a feedback letter / request, asks nothing.
func (uc *CitizenLetters) prepareAdmission(ctx context.Context, id string, to domain.LetterStatus,
	caller LetterCaller, acceptedAt time.Time) (admissionDeadline, error) {
	if to != domain.LetterStatusAdmitted {
		return admissionDeadline{}, nil
	}
	var cur domain.CitizenLetter
	if err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		cur, err = uc.repo.ByID(ctx, tx, id)
		return err
	}); err != nil {
		return admissionDeadline{}, err
	}
	if err := mayWorkOn(cur, caller); err != nil {
		return admissionDeadline{}, err
	}
	if err := domain.CheckLetterTransition(cur.Status, to); err != nil {
		return admissionDeadline{}, err
	}
	if !cur.Type.HasResolutionDeadline() {
		return admissionDeadline{}, nil
	}
	due, err := uc.letterDeadline(ctx, cur.Type,
		identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_RESOLUTION, acceptedAt)
	if err != nil {
		return admissionDeadline{}, err
	}
	return admissionDeadline{asked: true, letterType: cur.Type, resolutionDue: due}, nil
}

// letterDeadline asks identity ONE deadline of a letter of this commune (ADR 0085 B). Zero = the
// commune has no rule for this type and kind ("Không đặt hạn", B3). EVERY error refuses the act —
// never zero on an error, because a letter without a deadline counts as on time (ADR 0084 #4):
//
//	identityclient.ErrCitizenLetterDeadlineUnusable   → ErrLetterDeadlineUnusable   (409, fix the rule)
//	anything else (ErrIdentityUnavailable included)   → ErrLetterDeadlineUnavailable (503, retry)
func (uc *CitizenLetters) letterDeadline(ctx context.Context, t domain.LetterType,
	kind identityv1.CitizenLetterDeadlineKind, countFrom time.Time) (time.Time, error) {
	wire, err := letterTypeOnWire(t)
	if err != nil {
		return time.Time{}, err
	}
	due, configured, err := uc.dir.ResolveCitizenLetterDeadline(ctx, wire, kind, countFrom)
	switch {
	case errors.Is(err, identityclient.ErrCitizenLetterDeadlineUnusable):
		return time.Time{}, fmt.Errorf("%w: %w", ErrLetterDeadlineUnusable, err)
	case err != nil:
		return time.Time{}, fmt.Errorf("%w: %w", ErrLetterDeadlineUnavailable, err)
	case !configured:
		if due != nil {
			// Outside the client's contract: refused rather than guessing which half is true.
			return time.Time{}, fmt.Errorf("%w: identity trả hạn kèm 'chưa cấu hình'", ErrLetterDeadlineUnavailable)
		}
		return time.Time{}, nil
	case due == nil || due.IsZero():
		// A zero deadline would read as "Không đặt hạn" — the very confusion B3 exists to prevent.
		return time.Time{}, fmt.Errorf("%w: identity trả 'đã cấu hình' mà không có hạn", ErrLetterDeadlineUnavailable)
	}
	return due.UTC(), nil
}

// letterTypeOnWire maps the register's letter type onto identity.proto's enum. An unknown type is
// refused, never sent as UNSPECIFIED.
func letterTypeOnWire(t domain.LetterType) (identityv1.CitizenLetterType, error) {
	switch t {
	case domain.LetterTypeFeedback:
		return identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_KIEN_NGHI_PHAN_ANH, nil
	case domain.LetterTypeComplaint:
		return identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_KHIEU_NAI, nil
	case domain.LetterTypeDenunciation:
		return identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_TO_CAO, nil
	case domain.LetterTypeRequest:
		return identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_DE_NGHI, nil
	}
	return identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_UNSPECIFIED, domain.ErrLetterTypeInvalid
}

// --- result -----------------------------------------------------------------------------------------

// RecordResult writes the result. WHAT IS REQUIRED DEPENDS ON THE LETTER'S TYPE (ADR 0084 #2,
// domain.CitizenLetter.WithResult): the issued document and a summary for a complaint or a
// denunciation; the reply alone (the one textarea) for a feedback letter or a request. The type is
// known only once the row is read, so the shape is checked here and the rule under the row lock.
// Allowed in `thu-ly` and `dang-giai-quyet` only. Same WHO as Move. A request carrying the values
// already stored writes nothing and audits nothing — which is what makes the PUT honestly idempotent.
func (uc *CitizenLetters) RecordResult(ctx context.Context, id string, req LetterResultRequest, caller LetterCaller) (domain.CitizenLetter, error) {
	if id == "" {
		return domain.CitizenLetter{}, docstore.ErrCitizenLetterNotFound
	}
	now := uc.clock()
	res, err := normaliseResult(req, now)
	if err != nil {
		return domain.CitizenLetter{}, err
	}
	if err := requireActor(caller.Actor); err != nil {
		return domain.CitizenLetter{}, err
	}
	logID, err := uc.newID()
	if err != nil {
		return domain.CitizenLetter{}, fmt.Errorf("citizen_letter: sinh mã nhật ký: %w", err)
	}

	var after domain.CitizenLetter
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := mayWorkOn(before, caller); err != nil {
			return err
		}
		if before.Status != domain.LetterStatusAdmitted && before.Status != domain.LetterStatusResolving {
			return domain.ErrLetterResultNotAllowed
		}
		if after, err = before.WithResult(res); err != nil {
			return err
		}
		if sameResult(before, after) {
			return nil
		}
		after.UpdatedAt = now
		if err := uc.repo.UpdateResult(ctx, tx, after, caller.Actor.ID); err != nil {
			return err
		}
		if err := uc.repo.InsertLog(ctx, tx, domain.LetterLogEntry{
			ID: logID, LetterID: before.ID, At: now, ActorCode: caller.Actor.ID, Kind: domain.LetterLogResult,
			Content: resultLogLine(res),
		}); err != nil {
			return err
		}
		return writeLetterAudit(ctx, tx, caller.Actor, ActionRecordCitizenLetterResult, before, map[string]any{
			"truoc":               resultFields(before),
			"sau":                 resultFields(after),
			"doi_tom_tat_ket_qua": before.ResultSummary != after.ResultSummary,
		})
	})
	if err != nil {
		return domain.CitizenLetter{}, wrapLetter(ctx, "ghi kết quả đơn thư", err)
	}
	return after, nil
}

// normaliseResult trims every field and checks its SHAPE — ceilings, and a document date that is not
// in the future — before any transaction. Which fields are REQUIRED is the type's rule, checked under
// the row lock (domain.CitizenLetter.WithResult).
func normaliseResult(req LetterResultRequest, now time.Time) (domain.LetterResult, error) {
	var (
		out domain.LetterResult
		err error
	)
	if out.DocumentNo, err = domain.TrimOptional(req.DocumentNo, domain.MaxResultNo); err != nil {
		return out, err
	}
	if !req.DocumentDate.IsZero() {
		if err = domain.CheckResultDocumentDate(req.DocumentDate, now); err != nil {
			return out, err
		}
		out.DocumentDate = req.DocumentDate
	}
	if out.Signer, err = domain.TrimOptional(req.Signer, domain.MaxResultSigner); err != nil {
		return out, err
	}
	if out.Issuer, err = domain.TrimOptional(req.Issuer, domain.MaxResultIssuer); err != nil {
		return out, err
	}
	if out.Summary, err = domain.TrimOptional(req.Summary, domain.MaxResultSummary); err != nil {
		return out, err
	}
	return out, nil
}

// resultLogLine is the log row of a recorded result. The document number and date name an ISSUED
// document, not a citizen; the reply text is never copied into the append-only log.
func resultLogLine(r domain.LetterResult) string {
	if r.DocumentNo == "" {
		return "Ghi kết quả giải quyết"
	}
	return "Ghi kết quả giải quyết: văn bản số " + r.DocumentNo + " ngày " + r.DocumentDate.Format("02/01/2006")
}

func sameResult(a, b domain.CitizenLetter) bool {
	return a.ResultDocumentNo == b.ResultDocumentNo && a.ResultDocumentDate.Equal(b.ResultDocumentDate) &&
		a.ResultSigner == b.ResultSigner && a.ResultIssuer == b.ResultIssuer && a.ResultSummary == b.ResultSummary
}

func resultFields(l domain.CitizenLetter) map[string]any {
	date := ""
	if !l.ResultDocumentDate.IsZero() {
		date = l.ResultDocumentDate.Format(time.DateOnly)
	}
	return map[string]any{
		"so_van_ban": l.ResultDocumentNo, "ngay_van_ban": date,
		"nguoi_ky": l.ResultSigner, "co_quan_ban_hanh": l.ResultIssuer,
	}
}

// --- sender correction ------------------------------------------------------------------------------

// CorrectSender edits the sender fields. A no-op writes nothing. The log row says THAT the sender was
// corrected and the audit entry says WHICH fields changed — never a value, before or after: both are
// append-only, and an append-only store of raw personal data is one nobody can ever anonymise (rule 6,
// forbidden #4; rule 3, invariant 7).
func (uc *CitizenLetters) CorrectSender(ctx context.Context, id string, req SenderCorrection, caller LetterCaller) (domain.CitizenLetter, error) {
	if id == "" {
		return domain.CitizenLetter{}, docstore.ErrCitizenLetterNotFound
	}
	if req.Name == nil && req.Phone == nil && req.Address == nil {
		return domain.CitizenLetter{}, domain.ErrLetterSenderEmpty
	}
	var name, phone, address string
	var err error
	if req.Name != nil {
		if name, err = domain.TrimOptional(*req.Name, domain.MaxSenderName); err != nil {
			return domain.CitizenLetter{}, err
		}
	}
	if req.Phone != nil {
		if phone, err = domain.TrimPhone(*req.Phone); err != nil {
			return domain.CitizenLetter{}, err
		}
	}
	if req.Address != nil {
		if address, err = domain.TrimOptional(*req.Address, domain.MaxSenderAddress); err != nil {
			return domain.CitizenLetter{}, err
		}
	}
	if err := requireActor(caller.Actor); err != nil {
		return domain.CitizenLetter{}, err
	}
	now := uc.clock()
	logID, err := uc.newID()
	if err != nil {
		return domain.CitizenLetter{}, fmt.Errorf("citizen_letter: sinh mã nhật ký: %w", err)
	}

	var after domain.CitizenLetter
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		after = before
		if req.Name != nil {
			after.SenderName = name
		}
		if req.Phone != nil {
			after.SenderPhone = phone
		}
		if req.Address != nil {
			after.SenderAddress = address
		}
		changed := map[string]bool{
			"sender_name":    before.SenderName != after.SenderName,
			"sender_phone":   before.SenderPhone != after.SenderPhone,
			"sender_address": before.SenderAddress != after.SenderAddress,
		}
		if !changed["sender_name"] && !changed["sender_phone"] && !changed["sender_address"] {
			return nil
		}
		after.UpdatedAt = now
		if err := uc.repo.UpdateSender(ctx, tx, after, caller.Actor.ID); err != nil {
			return err
		}
		if err := uc.repo.InsertLog(ctx, tx, domain.LetterLogEntry{
			ID: logID, LetterID: before.ID, At: now, ActorCode: caller.Actor.ID,
			Kind: domain.LetterLogSenderCorrection, Content: logSenderCorrected,
		}); err != nil {
			return err
		}
		return writeLetterAudit(ctx, tx, caller.Actor, ActionCorrectLetterSender, before, map[string]any{
			"truong_da_doi": changed,
		})
	})
	if err != nil {
		return domain.CitizenLetter{}, wrapLetter(ctx, "sửa người gửi đơn thư", err)
	}
	return after, nil
}

// --- deadline ---------------------------------------------------------------------------------------

// SetDeadline is the clerk's "Hạn xử lý" (ADR 0079 lô 5 Q18): set it to `due`, or clear it with the
// zero instant ("Không đặt"). domain.SetActiveDue decides the column and refuses a finished letter.
// The instant is stored exactly as given — fixed AT THIS ACT, never recomputed on read (rule 10,
// invariant 2). A request carrying the value already stored writes nothing and audits nothing.
//
// WHO: a holder of `petition.create` — the prototype's PATCH of a petition (router.py:430-433) and the
// key that books and routes letters here (C13). The route already demands it; CanBook is the checker's
// answer for the same key in this commune, refused here as a second wall so the use case is safe
// whatever edge calls it.
//
// NO LOG ROW: 0006's `citizen_letter_log_kind_valid` admits five kinds and none is a deadline change.
// The audit entry (same transaction) is the record; adding a log kind needs a migration widening that
// CHECK — reported, not done here.
func (uc *CitizenLetters) SetDeadline(ctx context.Context, id string, due time.Time, caller LetterCaller) (domain.CitizenLetter, error) {
	if id == "" {
		return domain.CitizenLetter{}, docstore.ErrCitizenLetterNotFound
	}
	if err := requireActor(caller.Actor); err != nil {
		return domain.CitizenLetter{}, err
	}
	if !caller.CanBook {
		return domain.CitizenLetter{}, domain.ErrLetterNotPermitted
	}
	now := uc.clock()

	var after domain.CitizenLetter
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if after, err = before.SetActiveDue(due); err != nil {
			return err
		}
		if before.ProcessingDueAt.Equal(after.ProcessingDueAt) && before.ResolutionDueAt.Equal(after.ResolutionDueAt) {
			return nil
		}
		after.UpdatedAt = now
		if err := uc.repo.UpdateDeadline(ctx, tx, after, caller.Actor.ID); err != nil {
			return err
		}
		// A deadline is an instant a clerk typed, not personal data: before and after are recorded in
		// full, so an inspection can see every commitment the commune made and moved.
		return writeLetterAudit(ctx, tx, caller.Actor, ActionSetLetterDeadline, before, map[string]any{
			"cot":   domain.LetterDueColumn(before.Status),
			"truoc": map[string]any{"han_xu_ly": instantText(before.ActiveDueAt())},
			"sau":   map[string]any{"han_xu_ly": instantText(after.ActiveDueAt())},
		})
	})
	if err != nil {
		return domain.CitizenLetter{}, wrapLetter(ctx, "đặt hạn xử lý đơn thư", err)
	}
	return after, nil
}

// instantText is an instant for the trail: RFC 3339 in UTC, "" for none.
func instantText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// --- notes ------------------------------------------------------------------------------------------

// AddNote writes a `ghi-chu` row. Same WHO as Move.
//
// ⚠ C13/C14 ALSO LETS "officers of the same unit" write the log. The principal carries no unit, and
// identity's StaffOrgUnits must never be used in a guard (its contract) — so those officers are
// refused (fail closed) until a unit-membership check identity stands behind exists.
func (uc *CitizenLetters) AddNote(ctx context.Context, id, content string, caller LetterCaller) (domain.LetterLogEntry, error) {
	if id == "" {
		return domain.LetterLogEntry{}, docstore.ErrCitizenLetterNotFound
	}
	text, err := domain.TrimRequired(content, domain.MaxLetterNote, domain.ErrLetterNoteMissing)
	if err != nil {
		return domain.LetterLogEntry{}, err
	}
	if err := requireActor(caller.Actor); err != nil {
		return domain.LetterLogEntry{}, err
	}
	now := uc.clock()
	logID, err := uc.newID()
	if err != nil {
		return domain.LetterLogEntry{}, fmt.Errorf("citizen_letter: sinh mã nhật ký: %w", err)
	}
	entry := domain.LetterLogEntry{ID: logID, At: now, ActorCode: caller.Actor.ID, Kind: domain.LetterLogNote, Content: text}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		l, err := uc.repo.ByID(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := mayWorkOn(l, caller); err != nil {
			return err
		}
		entry.LetterID = l.ID
		if err := uc.repo.InsertLog(ctx, tx, entry); err != nil {
			return err
		}
		// The note itself is free text and stays out of the trail.
		return writeLetterAudit(ctx, tx, caller.Actor, ActionNoteCitizenLetter, l, map[string]any{
			"nhat_ky_id": logID,
		})
	})
	if err != nil {
		return domain.LetterLogEntry{}, wrapLetter(ctx, "ghi nhật ký đơn thư", err)
	}
	return entry, nil
}

// --- reads ------------------------------------------------------------------------------------------

// Detail reads one letter and what this viewer may see of it. A DENUNCIATION READ THAT DISCLOSES THE
// SENDER OR THE SUMMARY IS AUDITED in the same transaction (rule 6, invariant 7): if the entry cannot
// be written, nothing is returned.
func (uc *CitizenLetters) Detail(ctx context.Context, id string, viewer LetterCaller) (domain.CitizenLetter, domain.LetterDisclosure, error) {
	if id == "" {
		return domain.CitizenLetter{}, domain.LetterDisclosure{}, docstore.ErrCitizenLetterNotFound
	}
	var (
		l    domain.CitizenLetter
		show domain.LetterDisclosure
	)
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		if l, err = uc.repo.ByID(ctx, tx, id); err != nil {
			return err
		}
		show = domain.DetailDisclosure(l, viewer.Actor.ID, viewer.CanBook)
		if !l.Type.ProtectsIdentity() || (!show.Identity && !show.Summary) {
			return nil
		}
		if err := requireActor(viewer.Actor); err != nil {
			return err
		}
		return writeLetterAudit(ctx, tx, viewer.Actor, ActionViewDenunciation, l, map[string]any{
			"xem_danh_tinh": show.Identity, "xem_noi_dung": show.Summary,
		})
	})
	if err != nil {
		return domain.CitizenLetter{}, domain.LetterDisclosure{}, wrapLetter(ctx, "đọc đơn thư", err)
	}
	return l, show, nil
}

// Log reads one letter's log, newest first. The letter is read first in the same transaction: that
// read is the isolation (a removed letter's log is not served, an unknown id is the same 404).
func (uc *CitizenLetters) Log(ctx context.Context, id string) ([]domain.LetterLogEntry, error) {
	if id == "" {
		return nil, docstore.ErrCitizenLetterNotFound
	}
	var out []domain.LetterLogEntry
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		l, err := uc.repo.ByID(ctx, tx, id)
		if err != nil {
			return err
		}
		out, err = uc.repo.Log(ctx, tx, l.ID)
		return err
	})
	if err != nil {
		return nil, wrapLetter(ctx, "đọc nhật ký đơn thư", err)
	}
	return out, nil
}

// List reads one page. `related` asks identity for the caller's units; an identity error is a 503,
// never the tab with the unit clause dropped (which would hide exactly what the tab is named for).
func (uc *CitizenLetters) List(ctx context.Context, q LetterListQuery, req page.Request) (page.Result[domain.CitizenLetter], error) {
	f, err := uc.scopedFilter(ctx, q)
	if err != nil {
		return page.NewResult[domain.CitizenLetter](), err
	}
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	res, err := uc.repo.List(ctx, f, req)
	if err != nil {
		return page.NewResult[domain.CitizenLetter](), wrapLetter(ctx, "đọc sổ đơn thư", err)
	}
	return res, nil
}

// Count is how many letters List would page through for the same query — the tab label "Đơn thư công
// dân (N)" (ADR 0084 #7). The SAME scope resolution and the same filter as List (scopedFilter, then
// one predicate builder in the store), so the number never disagrees with the register it labels. A
// number only: nothing in it is masked because nothing in it is personal.
func (uc *CitizenLetters) Count(ctx context.Context, q LetterListQuery) (int, error) {
	f, err := uc.scopedFilter(ctx, q)
	if err != nil {
		return 0, err
	}
	n, err := uc.repo.Count(ctx, f)
	if err != nil {
		return 0, wrapLetter(ctx, "đếm sổ đơn thư", err)
	}
	return n, nil
}

// scopedFilter adds the scope's clause to the filter. `related` asks identity for the caller's units;
// an identity error is a 503, never the tab with the unit clause dropped (which would hide exactly
// what the tab is named for).
func (uc *CitizenLetters) scopedFilter(ctx context.Context, q LetterListQuery) (docstore.CitizenLetterFilter, error) {
	f := q.Filter
	switch q.Scope {
	case "", "all":
	case "mine", "related":
		if q.CallerCode == "" {
			return f, fmt.Errorf("citizen_letter: thiếu mã cán bộ của phiên")
		}
		if q.Scope == "mine" {
			f.MineCode = q.CallerCode
			break
		}
		units, err := uc.dir.StaffOrgUnits(ctx, q.CallerCode)
		if err != nil {
			return f, fmt.Errorf("%w: %w", ErrLetterDirectoryUnavailable, err)
		}
		f.Related = &docstore.CitizenLetterRelated{StaffCode: q.CallerCode, OrgUnits: units}
	default:
		return f, ErrLetterScopeInvalid
	}
	return f, nil
}

// Duplicates is C11's warning: candidates received within the last year (Asia/Ho_Chi_Minh), same
// sender name when one was typed, ranked by summary similarity. It WARNS — it links nothing.
func (uc *CitizenLetters) Duplicates(ctx context.Context, q DuplicateQuery) ([]domain.DuplicateCandidate, error) {
	summary, err := domain.TrimRequired(q.Summary, domain.MaxLetterSummary, domain.ErrLetterSummaryMissing)
	if err != nil {
		return nil, err
	}
	name, err := domain.TrimOptional(q.SenderName, domain.MaxSenderName)
	if err != nil {
		return nil, err
	}
	y, m, d := uc.clock().In(domain.AutomationZone).Date()
	since := time.Date(y-1, m, d, 0, 0, 0, 0, time.UTC)
	rows, err := uc.repo.DuplicateCandidates(ctx, name, since)
	if err != nil {
		return nil, wrapLetter(ctx, "kiểm đơn trùng", err)
	}
	return domain.RankDuplicates(summary, rows), nil
}

// Report computes the year's figures (domain.BuildLetterReport states every definition).
func (uc *CitizenLetters) Report(ctx context.Context, year int) (domain.LetterReport, error) {
	start := time.Date(year, time.January, 1, 0, 0, 0, 0, domain.AutomationZone)
	rows, err := uc.repo.ReportRows(ctx, year, start)
	if err != nil {
		return domain.LetterReport{}, wrapLetter(ctx, "lập báo cáo đơn thư", err)
	}
	return domain.BuildLetterReport(year, rows, uc.clock()), nil
}

// --- shared -----------------------------------------------------------------------------------------

// mayWorkOn is C13/C14: the assignee of THIS letter, or a holder of `petition.create`.
func mayWorkOn(l domain.CitizenLetter, caller LetterCaller) error {
	if caller.CanBook || (caller.Actor.ID != "" && caller.Actor.ID == l.AssigneeCode) {
		return nil
	}
	return domain.ErrLetterNotPermitted
}

func (uc *CitizenLetters) checkUnit(ctx context.Context, unitID string) error {
	live, err := uc.dir.LiveOrgUnits(ctx, []string{unitID})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLetterDirectoryUnavailable, err)
	}
	if _, ok := live[unitID]; !ok {
		return domain.ErrLetterUnitNotLive
	}
	return nil
}

func (uc *CitizenLetters) checkAssignee(ctx context.Context, code string) error {
	ok, err := uc.dir.CanBoGiaoViecDuoc(ctx, []string{code})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLetterDirectoryUnavailable, err)
	}
	if _, yes := ok[code]; !yes {
		return domain.ErrLetterAssigneeNotLive
	}
	return nil
}

// requireActor refuses a write whose trail cannot name its author (rule 6) — before any transaction.
func requireActor(a audit.Actor) error {
	if a.ID == "" {
		return fmt.Errorf("citizen_letter: thiếu người thực hiện")
	}
	return nil
}

// writeLetterAudit files one entry under the letter's register code, in the caller's transaction.
// TenantID is left to audit.Write, which takes it from the transaction (rule 1, invariant 4).
func writeLetterAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action string,
	l domain.CitizenLetter, delta map[string]any) error {
	delta["don_thu_id"] = l.ID
	raw, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("citizen_letter: mã hoá delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		Actor:   actor,
		Action:  action,
		Subject: domain.LetterAuditSubject(l.Year, l.Number),
		Delta:   raw,
	})
}

// bookingSummary is the booking entry's view of the letter. NO summary and NO sender value: the
// summary quotes the letter and the sender is personal data, and audit_log is append-only (rule 6,
// forbidden #4). WHETHER each sender field was given is recorded — that is what an inspection asks.
func bookingSummary(l domain.CitizenLetter) map[string]any {
	return map[string]any{
		"so_vao_so": l.Number, "nam": l.Year, "loai_don": string(l.Type), "nguon": string(l.Source),
		"ngay_nhan":     l.ReceivedDate.Format(time.DateOnly),
		"co_ho_ten":     l.SenderName != "",
		"co_dien_thoai": l.SenderPhone != "",
		"co_dia_chi":    l.SenderAddress != "",
		"don_lien_quan": l.RelatedLetterID,
		"bo_phan_giu":   l.HoldingUnitID,
		"trang_thai":    string(l.Status),
		// The deadline identity fixed at this act ("" = the commune has no rule, "Không đặt hạn"). An
		// instant, not personal data: recorded in full as SetDeadline records it.
		"han_xu_ly": instantText(l.ProcessingDueAt),
	}
}

// wrapLetter adds the commune and the operation — NOTHING about the letter (rule 3). %w keeps the
// chain so the edge can still tell a refusal from a failure.
func wrapLetter(ctx context.Context, op string, err error) error {
	return fmt.Errorf("don_thu: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}
