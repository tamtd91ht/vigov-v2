package app

// The use cases behind the WRITE surface of SỔ VĂN BẢN ĐI.
//
// ⚠ NO SPECIFICATION EXISTS FOR THIS REGISTER — see migration 0004 and domain.OutgoingDocument. What
// is built here is the smallest register that is actually usable: issue a number, record what went
// out, correct a mistake, remove an entry with a reason. Nothing more was invented: there is no
// lifecycle, no approval chain and no deadline, because no source in this repository names one.
//
// THE STRUCTURE IS incoming_document.go's, AND THE SHARED HELPERS ARE ITS: `requireDocumentActor`,
// `wrapDocumentErr`, `pickString`, `pickDate`. One copy, because the two registers must answer the
// same question the same way — a second copy of "who is acting" or "what may leave in an error" is a
// second answer, and the drifted one is the one that leaks a document summary into a log line.
//
// WHAT IS DIFFERENT, AND IT IS ONE THING: THE DEADLINE. An incoming document carries a commitment
// the commune must meet, computed from its SLA. An outgoing document does not — it is the commune's
// own act, finished at the moment it is issued. So there is no identity call on this path, and
// `van_ban_di` has no deadline column. IF THE CUSTOMER LATER WANTS "văn bản đi phải phát hành trong
// N giờ sau khi ký", that is a new SLA work kind and a new column, not a number added here.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// OutgoingDocumentStore is the outgoing register's write half, declared at the point of use. Every
// method takes the transaction, for the reason stated on IncomingDocumentStore.
type OutgoingDocumentStore interface {
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.OutgoingDocument, error)
	DocumentTypeInUse(ctx context.Context, tx *store.ScopedTx, code string) error
	Insert(ctx context.Context, tx *store.ScopedTx, d domain.OutgoingDocument) error
	Update(ctx context.Context, tx *store.ScopedTx, d domain.OutgoingDocument) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
}

// The business verbs written into the trail. `cap_so_van_ban_di` rather than `them_van_ban_di`, and
// the difference is what an inspection searches for: the event worth naming is that THIS COMMUNE
// ISSUED A NUMBER, which is the act that cannot be undone.
//
// THE VALUES ARE STORED in the append-only `audit_log.action`; ADR 0061 §Ánh xạ hành vi maps them to
// their English successors, which only NEW entries will carry (X22, layer C).
const (
	ActionIssueOutgoingDocumentNumber = "cap_so_van_ban_di"
	ActionUpdateOutgoingDocument      = "sua_van_ban_di"
	ActionRemoveOutgoingDocument      = "go_van_ban_di"
)

// IssueOutgoingDocumentRequest is one outgoing document as it arrives from the handler.
//
// THERE IS NO `IssuedNo` FIELD AND THERE MUST NEVER BE ONE, and on this register that sentence is
// the heaviest one in the file: the number goes on paper, under a seal, out of the building. A client
// that could name it could put a number that is already on a signed document onto a second one.
//
// THERE IS NO `Year` FIELD: the year is the year of the act — see IssueNumber.
type IssueOutgoingDocumentRequest struct {
	DocumentDate time.Time
	DocumentType string
	Summary      string
	Recipient    string
	Signer       string
}

// UpdateOutgoingDocumentRequest is a PARTIAL edit: a nil pointer means "leave this alone". `Signer`
// is optional and its empty value is meaningful — "this entry does not record a signer" is a
// statement — so a dialog editing only the recipient must not wipe it.
type UpdateOutgoingDocumentRequest struct {
	DocumentDate *time.Time
	DocumentType *string
	Summary      *string
	Recipient    *string
	Signer       *string
}

// OutgoingDocuments owns issuing, correcting and removing one commune's outgoing documents.
type OutgoingDocuments struct {
	db     *store.DB
	repo   OutgoingDocumentStore
	series NumberSeriesStore

	newID func() (string, error)
	now   func() time.Time
}

func NewOutgoingDocuments(db *store.DB, repo OutgoingDocumentStore, series NumberSeriesStore) *OutgoingDocuments {
	return &OutgoingDocuments{db: db, repo: repo, series: series, newID: ulid.Moi}
}

func (uc *OutgoingDocuments) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

// IssueNumber issues one outgoing document number and records what it was issued for.
//
// THE SHAPE IS VALIDATED BEFORE THE TRANSACTION OPENS, so a malformed request never holds the
// counter's row lock — that lock serialises every issue in the commune for that year.
//
// `nam` IS THE YEAR OF THE ACT. A number is given when the document is issued; a clerk who types
// last year's date onto today's document does not thereby insert a number into last year's closed
// series. ⚠ Administrative practice, not a line of any specification — reported as such.
func (uc *OutgoingDocuments) IssueNumber(ctx context.Context, req IssueOutgoingDocumentRequest,
	actor audit.Actor) (domain.OutgoingDocument, error) {

	now := uc.clock()

	doc, err := normalizeIssue(req, now)
	if err != nil {
		return domain.OutgoingDocument{}, err
	}
	if err := requireDocumentActor(actor); err != nil {
		return domain.OutgoingDocument{}, err
	}

	id, err := uc.newID()
	if err != nil {
		return domain.OutgoingDocument{}, fmt.Errorf("van_ban_di: sinh mã: %w", err)
	}
	doc.ID = id
	doc.Year = now.Year()
	doc.CreatedBy = actor.ID

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// THE TYPE IS CHECKED BEFORE THE NUMBER IS TAKEN, for the reason stated on
		// IncomingDocuments.Register: a request that is going to be refused must not consume a
		// number, because the counter only ever moves forward and the hole would be permanent.
		if err := uc.repo.DocumentTypeInUse(ctx, tx, doc.DocumentType); err != nil {
			return err
		}
		no, err := uc.series.IssueNumber(ctx, tx, docstore.RegisterOutgoing, doc.Year)
		if err != nil {
			return err
		}
		doc.IssuedNo = no

		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		if err := uc.repo.Insert(ctx, tx, doc); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": summarizeOutgoing(doc)})
		if err != nil {
			return fmt.Errorf("van_ban_di: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionIssueOutgoingDocumentNumber,
			Subject: domain.OutgoingDocumentCode(doc.Year, doc.IssuedNo),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.OutgoingDocument{}, wrapDocumentErr(ctx, "cấp số văn bản đi", err)
	}
	return doc, nil
}

// Update corrects one issued entry. The number and the year are not reachable from here.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — same reasoning as the incoming register, and it is
// what makes this route's `idem.KhongCan` declaration true rather than hopeful.
func (uc *OutgoingDocuments) Update(ctx context.Context, id string, req UpdateOutgoingDocumentRequest,
	actor audit.Actor) (domain.OutgoingDocument, error) {

	if id == "" {
		return domain.OutgoingDocument{}, docstore.ErrOutgoingDocumentNotFound
	}
	if err := requireDocumentActor(actor); err != nil {
		return domain.OutgoingDocument{}, err
	}
	now := uc.clock()

	var after domain.OutgoingDocument
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}

		after = before
		if err := applyOutgoingUpdate(&after, req, now); err != nil {
			return err
		}
		if outgoingUnchanged(before, after) {
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

		delta, err := json.Marshal(map[string]any{
			"van_ban_id": after.ID,
			"truoc":      summarizeOutgoingChange(before, after, true),
			"sau":        summarizeOutgoingChange(before, after, false),
		})
		if err != nil {
			return fmt.Errorf("van_ban_di: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionUpdateOutgoingDocument,
			Subject: domain.OutgoingDocumentCode(before.Year, before.IssuedNo),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.OutgoingDocument{}, wrapDocumentErr(ctx, "sửa văn bản đi", err)
	}
	return after, nil
}

// Remove soft deletes one entry, with a mandatory reason.
//
// THE NUMBER DOES NOT COME BACK, and here that is not a technical statement: the document was issued
// under that number and has left the commune. Removing the row takes it off the screen; it cannot
// take it off the paper, and a register that reissued the number would give two documents in the
// outside world one identity.
func (uc *OutgoingDocuments) Remove(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return docstore.ErrOutgoingDocumentNotFound
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
		if err := uc.repo.SoftDelete(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{
			"van_ban_id": before.ID,
			"truoc":      summarizeOutgoing(before),
			"ly_do":      reason,
			"xoa_mem":    true,
		})
		if err != nil {
			return fmt.Errorf("van_ban_di: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionRemoveOutgoingDocument,
			Subject: domain.OutgoingDocumentCode(before.Year, before.IssuedNo),
			Delta:   delta,
		})
	})
	if err != nil {
		return wrapDocumentErr(ctx, "gỡ văn bản đi", err)
	}
	return nil
}

// --- shape ---------------------------------------------------------------------------------------

func normalizeIssue(req IssueOutgoingDocumentRequest, now time.Time) (domain.OutgoingDocument, error) {
	var d domain.OutgoingDocument
	var err error

	if err = domain.ValidateOutgoingDocumentDate(req.DocumentDate, now); err != nil {
		return domain.OutgoingDocument{}, err
	}
	if d.DocumentType, err = domain.NormalizeRequired(req.DocumentType, domain.MaxDocumentTypeLen,
		domain.ErrMissingDocumentType); err != nil {
		return domain.OutgoingDocument{}, err
	}
	if d.Summary, err = domain.NormalizeRequired(req.Summary, domain.MaxSummaryLen,
		domain.ErrMissingSummary); err != nil {
		return domain.OutgoingDocument{}, err
	}
	if d.Recipient, err = domain.NormalizeRequired(req.Recipient, domain.MaxRecipientLen,
		domain.ErrMissingRecipient); err != nil {
		return domain.OutgoingDocument{}, err
	}
	if d.Signer, err = domain.NormalizeOptional(req.Signer, domain.MaxSignerLen); err != nil {
		return domain.OutgoingDocument{}, err
	}
	d.DocumentDate = req.DocumentDate
	return d, nil
}

func applyOutgoingUpdate(d *domain.OutgoingDocument, req UpdateOutgoingDocumentRequest, now time.Time) error {
	if req.DocumentDate != nil {
		if err := domain.ValidateOutgoingDocumentDate(*req.DocumentDate, now); err != nil {
			return err
		}
		d.DocumentDate = *req.DocumentDate
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
	if req.Recipient != nil {
		s, err := domain.NormalizeRequired(*req.Recipient, domain.MaxRecipientLen, domain.ErrMissingRecipient)
		if err != nil {
			return err
		}
		d.Recipient = s
	}
	if req.Signer != nil {
		s, err := domain.NormalizeOptional(*req.Signer, domain.MaxSignerLen)
		if err != nil {
			return err
		}
		d.Signer = s
	}
	return nil
}

func outgoingUnchanged(before, after domain.OutgoingDocument) bool {
	return before.DocumentDate.Equal(after.DocumentDate) &&
		before.DocumentType == after.DocumentType &&
		before.Summary == after.Summary &&
		before.Recipient == after.Recipient &&
		before.Signer == after.Signer
}

// summarizeOutgoing is the audit delta's view of one entry.
//
// ⚠ `noi_nhan` MAY NAME A CITIZEN (rule 3) — an outgoing document is often a reply to one person,
// and "Ông Nguyễn Văn A, thôn Bình Trị" is what a commune writes there. It is recorded for the same
// reason the incoming summary is: an entry that cannot say WHICH document was issued or removed is
// an entry nobody can use. What is guaranteed is narrower and stated rather than implied: nothing
// here logs it, and no error message carries it out to a client.
func summarizeOutgoing(d domain.OutgoingDocument) map[string]any {
	return map[string]any{
		"van_ban_id":   d.ID,
		"so_di":        d.IssuedNo,
		"nam":          d.Year,
		"ngay_van_ban": d.DocumentDate.Format("2006-01-02"),
		"loai_van_ban": d.DocumentType,
		"trich_yeu":    d.Summary,
		"noi_nhan":     d.Recipient,
		"nguoi_ky":     d.Signer,
	}
}

func summarizeOutgoingChange(before, after domain.OutgoingDocument, beforeSide bool) map[string]any {
	out := map[string]any{}
	if !before.DocumentDate.Equal(after.DocumentDate) {
		out["ngay_van_ban"] = pickDate(beforeSide, before.DocumentDate, after.DocumentDate)
	}
	if before.DocumentType != after.DocumentType {
		out["loai_van_ban"] = pickString(beforeSide, before.DocumentType, after.DocumentType)
	}
	if before.Summary != after.Summary {
		out["trich_yeu"] = pickString(beforeSide, before.Summary, after.Summary)
	}
	if before.Recipient != after.Recipient {
		out["noi_nhan"] = pickString(beforeSide, before.Recipient, after.Recipient)
	}
	if before.Signer != after.Signer {
		out["nguoi_ky"] = pickString(beforeSide, before.Signer, after.Signer)
	}
	return out
}
