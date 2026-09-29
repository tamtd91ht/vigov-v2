package app

// ISSUING AN INTERNAL ANNOUNCEMENT — the write half of `docs/ui-ux/08-thong-bao.md`.
//
// WHY THIS LAYER EXISTS FOR WHAT LOOKS LIKE TWO INSERTS: rule 6, invariant 3 requires the audit
// entry to share a transaction with the business write, and core/audit.Write takes a
// *store.ScopedTx with no overload that writes outside one. Opening that transaction is this
// layer's job. The handler translates HTTP and nothing else; the store knows SQL and nothing else.
//
// THE SECOND REASON IS THAT ISSUING IS ONE ACT WITH TWO WRITES. The announcement and the recipient
// list it is frozen against (§9.3) have to land together: an announcement with no recipients is a
// notice the commune believes it sent and nobody received, and a recipient list with no
// announcement is rows pointing at nothing. One transaction is what makes "both or neither" true.
//
// NAMES: `CitizenNotifications` and `CitizenNotificationStore` are already taken in this package by
// the CITIZEN notification ledger (0004). Everything here carries `Announcement`.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// AnnouncementStore is the store, declared at the point of use.
//
// BOTH METHODS TAKE THE TRANSACTION. That is what makes it impossible to write the announcement in
// one transaction and its recipients — or its audit entry — in another: there is no signature here
// that would let you. It is also what makes the properties worth proving provable without a
// PostgreSQL: that a refusal writes nothing, that the entry shares the transaction, that the
// author is never taken from a request body.
type AnnouncementStore interface {
	Insert(ctx context.Context, tx *store.ScopedTx, a domain.Announcement) error
	InsertRecipients(ctx context.Context, tx *store.ScopedTx, announcementID string,
		recipients []domain.AnnouncementRecipient) error
}

// Announcements owns composing and issuing one commune's internal announcements.
type Announcements struct {
	db   *store.DB
	repo AnnouncementStore

	// newID and now are injected so a test can pin both. In production they are ulid.Moi and
	// time.Now. The clock is injectable because `phat_hanh_luc` is the moment §3 prints and the
	// database refuses to change it afterwards — a test that could not fix it could only assert
	// that it is "roughly now", which is the assertion that passes when the value is wrong.
	newID func() (string, error)
	now   func() time.Time
}

func NewAnnouncements(db *store.DB, repo AnnouncementStore) *Announcements {
	return &Announcements{db: db, repo: repo, newID: ulid.Moi, now: time.Now}
}

// ActionPublishAnnouncement is the business verb written into the trail. Vietnamese snake_case, like
// every other action this system already writes (`dang_nhap`, `ghi_so_thong_bao`): an inspection
// reads these strings, and a function name would tell them nothing.
//
// IT NAMES THE ACT, NOT THE TABLE. `phat_hanh` and a future `go_thong_bao` are two different
// things that happened to one record, and an entry saying only "thong_bao changed" cannot tell
// them apart.
const ActionPublishAnnouncement = "phat_hanh_thong_bao"

var (
	// ErrOrgUnitDeliveryUnavailable refuses an announcement addressed to DEPARTMENTS.
	//
	// WHY A REFUSAL AND NOT A PARTIAL DELIVERY — this is the decision of the file. §5 lets an
	// author tick departments, and §9.3 says the recipient list is generated from them AT THE
	// MOMENT OF ISSUE. Generating it means turning a `bo_phan` into the staff inside it, and that
	// is service-identity's data: rule 2 allows gRPC or events and nothing else, and
	// IdentityService has no RPC for it (ResolveStaffPrincipal · ResolveCitizenSession ·
	// BatchGetStaff · ResolveStaffNames · ListCitizenCommunes · AdvanceWorkingHours ·
	// ResolveDeadlines is the whole service). `proto/` is the source of truth for that contract
	// and adding to it is a different piece of work with a different owner.
	//
	// THE ALTERNATIVE WAS TO RECORD THE DEPARTMENTS AND DELIVER TO NOBODY, and it is the one
	// outcome this module must not have: no error anywhere, a card in the book, §3's counter
	// reading `0/0`, and an author who believes the commune was told. A commune finds out when
	// somebody misses a meeting. Refusing breaks one button loudly and tells the person exactly
	// which capability is missing — fail closed, and visible.
	//
	// WHAT STILL WORKS TODAY: §5's `Gửi thêm đích danh`. An announcement to named colleagues is
	// issued, delivered and counted correctly.
	ErrOrgUnitDeliveryUnavailable = errors.New("thong_bao: chưa gửi được theo bộ phận")

	// ErrMissingAuthor refuses the write when the request carries no staff business code.
	//
	// `deleted_by` has the same rule for the same reason, and core/audit refuses an entry with no
	// actor: an announcement nobody can be asked about is an announcement with no author. There is
	// NO FALLBACK to an internal id — rule 6, invariant 8, and the fallback is what put ULIDs in
	// `audit_log.actor_id` six times on 2026-09-22.
	ErrMissingAuthor = errors.New("thong_bao: thiếu mã cán bộ soạn")
)

// Publish issues one announcement to the staff named on it.
//
// # WHAT IT DOES NOT DO, IN ORDER OF HOW LIKELY SOMEBODY IS TO LOOK FOR IT
//
//	no draft            §5's `Lưu nháp` is a second act with a second route. The state `nhap`
//	                    exists in the schema because §6 declares it; nothing here writes it, so an
//	                    announcement that exists has been issued.
//	no department fan-out see ErrOrgUnitDeliveryUnavailable.
//	no mail             §5's checkbox is recorded in `gui_thu_dien_tu` and nothing acts on it. There
//	                    is no SMTP adapter and no `Cấu hình → Máy chủ thư` in this repository, so
//	                    `trang_thai_thu` stays `chua-gui` and §3's mail chip never appears. Sending
//	                    mail from a commune is an external dependency and a separate decision.
//	no bell entry       §8's unified inbox (`hop_thu_thong_bao`) is fed by four modules, three of
//	                    them in service-petitions. That is an inter-service contract, not a row this
//	                    use case may invent (migration 0005 says the same).
//
// # THE AUTHOR COMES FROM THE SESSION AND FROM NOWHERE ELSE
//
// `actor.ID` carries the staff BUSINESS CODE — the handler builds it from `authz.Principal.Ma`
// (rule 6, invariant 8). There is no `AuthorCode` field on the request, because a field a body
// can fill is a member of staff issuing an announcement over a colleague's name.
func (uc *Announcements) Publish(ctx context.Context, req domain.CreateAnnouncementRequest,
	actor audit.Actor) (domain.Announcement, error) {

	// VALIDATED BEFORE THE TRANSACTION OPENS. A request that fails its shape must never hold a
	// transaction open while doing so, and the caller needs the reason rather than a rollback.
	clean, err := req.Validate()
	if err != nil {
		return domain.Announcement{}, err
	}
	if len(clean.OrgUnitIDs) > 0 {
		return domain.Announcement{}, ErrOrgUnitDeliveryUnavailable
	}
	// AFTER the department refusal, deliberately. An author who ticked only departments gets the
	// sentence that names the missing capability, not "you addressed this to nobody" — which would
	// be true and useless, and would send them looking for a bug in their own form.
	if len(clean.RecipientCodes) == 0 {
		return domain.Announcement{}, domain.ErrNoRecipients
	}
	if actor.ID == "" {
		return domain.Announcement{}, ErrMissingAuthor
	}

	id, err := uc.newID()
	if err != nil {
		return domain.Announcement{}, fmt.Errorf("thong_bao: sinh mã: %w", err)
	}

	// ONE CLOCK READING FOR THE WHOLE ACT. Reading it twice would let `phat_hanh_luc` and the audit
	// entry's own timestamp disagree about when the commune issued this, and the database refuses
	// to correct the first one afterwards.
	at := uc.now().UTC()

	a := domain.Announcement{
		ID:    id,
		Title: clean.Title,
		Body:  clean.Body,
		// ISSUED, NOT DRAFTED — this is the act §5's `➤ Phát hành` performs, and the state is
		// decided HERE rather than taken from the request. See domain.CreateAnnouncementRequest.
		Status:         domain.AnnouncementPublished,
		Pinned:         clean.Pinned,
		AckRequired:    clean.AckRequired,
		EmailRequested: clean.EmailRequested,
		// `chua-gui` WHATEVER THE CHECKBOX SAID. The flag records what was asked for; this records
		// what happened, and nothing has happened.
		EmailStatus:    domain.EmailNotSent,
		AuthorCode:     actor.ID,
		IssuedAt:       at,
		CreatedAt:      at,
		RecipientCount: len(clean.RecipientCodes),
		AckCount:       0,
	}

	// EVERY RECIPIENT IS `dich_danh` IN THIS PASS, and that is true rather than convenient: the
	// only way onto the list today is being named in §5's `Gửi thêm đích danh`. The day departments
	// expand, the rows they produce carry false, and §4 can still tell the two apart.
	recipients := make([]domain.AnnouncementRecipient, 0, len(clean.RecipientCodes))
	for _, code := range clean.RecipientCodes {
		recipients = append(recipients, domain.AnnouncementRecipient{RecipientCode: code, IsNamed: true})
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		if err := uc.repo.Insert(ctx, tx, a); err != nil {
			return err
		}
		if err := uc.repo.InsertRecipients(ctx, tx, a.ID, recipients); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": summarizeAnnouncement(a, recipients)})
		if err != nil {
			return fmt.Errorf("thong_bao: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS BOTH INSERTS (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides which
		// commune the entry belongs to.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionPublishAnnouncement,
			Subject: announcementSubject(a),
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no announcement, no recipients, no trail. The three agree.
		return domain.Announcement{}, wrapErr(ctx, "phát hành thông báo", err)
	}
	return a, nil
}

// announcementSubject is the audit subject.
//
// NOT A ULID, for the reason rule 6, invariant 8 gives for `actor_id`: an entry is read years later
// by somebody handling a complaint or an inspection, and a ULID names nothing to them. AN
// ANNOUNCEMENT HAS NO BUSINESS CODE OF ITS OWN — chapter 08 prints no reference number and mints no
// series, and inventing one here would be deciding a numbering scheme for a record the customer
// has not asked to number.
//
// So it is COMPOSED from the two facts that let a person find the notice in the book: the day it
// was issued and who issued it. Two announcements from one person on one day share a subject; the
// delta carries the id and the title-free summary that separates them, and a subject is a locator
// rather than a key.
//
// THE TITLE IS DELIBERATELY NOT IN IT. `tieu_de` is free text a member of staff types, and a
// commune's announcement can name a person ("Thông báo về việc ông Nguyễn Văn A…"). The audit
// ledger is never deleted (rule 6, invariant 4), so anything put in it is there permanently — and
// rule 3, forbidden #5 keeps personal data out of exactly this kind of durable record.
func announcementSubject(a domain.Announcement) string {
	return "thong-bao/" + a.IssuedAt.Format("2006-01-02") + "/" + a.AuthorCode
}

// summarizeAnnouncement is the audit delta's view of what was issued.
//
// WHAT IS IN IT AND WHY EACH THING IS: the id (the only handle on the record), the flags (they
// decide what the commune was asked to do — acknowledge, or merely read), the COUNT of recipients
// and their codes. WHAT IS NOT IN IT: the title and the body. See announcementSubject — both are
// free text that can name a person, and this ledger is never deleted.
//
// THE RECIPIENT CODES ARE IN IT, AND THAT IS NOT A CONTRADICTION. `CB-2026-7K3M9Q` is the business
// code of a member of STAFF acting in their official capacity — the same value rule 6, invariant 8
// requires in `actor_id` on every entry in this system. It is not personal data in the sense
// Decree 13 governs, and without it the entry cannot answer the question an inspection asks: who
// was told.
func summarizeAnnouncement(a domain.Announcement, recipients []domain.AnnouncementRecipient) map[string]any {
	codes := make([]string, 0, len(recipients))
	for _, r := range recipients {
		codes = append(codes, r.RecipientCode)
	}
	return map[string]any{
		"id":                a.ID,
		"trang_thai":        string(a.Status),
		"ghim":              a.Pinned,
		"bat_buoc_xac_nhan": a.AckRequired,
		"gui_thu_dien_tu":   a.EmailRequested,
		"so_nguoi_nhan":     len(recipients),
		"nguoi_nhan_ma":     codes,
	}
}
