package app

// The use case behind the citizen notification ledger.
//
// WHY THIS LAYER EXISTS AT ALL FOR WHAT LOOKS LIKE ONE INSERT: rule 6, invariant 3 requires the
// audit entry to share a transaction with the business write, and two things have to be decided
// together for that to be true — WHEN the transaction opens, and WHETHER an entry is owed at all.
// The second is the interesting one here. Queues deliver at least once (rule 2, invariant 5), so
// the same fact arrives more than once; the second arrival must write no row AND no entry.
// Auditing unconditionally would fill the ledger of a public authority with entries saying it
// promised the same citizen the same thing four times, which is not what happened.
//
// THIS FILE DEFINES NO EVENT AND SUBSCRIBES TO NOTHING. The fact that a petition changed status
// belongs to `petitions`, and how it crosses the boundary is an inter-service contract — rule 2,
// with `.proto` as its source of truth. What is here is the half `comms` owns: given the fact,
// what gets recorded and what the citizen is owed.

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// CitizenNotificationStore is the store, declared at the point of use.
//
// AN INTERFACE HERE AND NOT THE CONCRETE *store.CitizenNotificationStore, for the reason that
// decides most of this repository's test coverage: the properties worth proving — that a
// redelivery writes nothing, that the entry shares the transaction — must be provable without a
// PostgreSQL, or they get proved once and then never again.
//
// BOTH METHODS TAKE THE TRANSACTION. That is what makes it impossible to write the row in one
// transaction and the entry in another: there is no signature here that would let you.
type CitizenNotificationStore interface {
	Insert(ctx context.Context, tx *store.ScopedTx, n domain.CitizenNotification) (bool, error)
	RecordSendResult(ctx context.Context, tx *store.ScopedTx, id string, result domain.SendResult) (bool, error)
}

// CitizenNotifications records what this commune owes its citizens, and what came of it.
type CitizenNotifications struct {
	db   *store.DB
	repo CitizenNotificationStore

	// newID is injected so a test can pin the id. In production it is newULID below.
	newID func() (string, error)
}

func NewCitizenNotifications(db *store.DB, repo CitizenNotificationStore) *CitizenNotifications {
	return &CitizenNotifications{db: db, repo: repo, newID: newULID}
}

// RecordRequest is one obligation to notify, as it arrives from whatever produced the fact.
//
// THE COMMUNE IS NOT IN HERE AND MUST NOT BE. It travels in the context (rule 1, invariant 4);
// a field on this struct would be a commune the caller names, which for a message consumer means
// a commune taken from a payload. Background work carries the commune INSIDE the envelope
// (rule 1, invariant 9), and core/events.Dispatch is what puts it into the context.
type RecordRequest struct {
	SubjectType   domain.SubjectType
	SubjectCode   string // the citizen's lookup code — a BUSINESS code, never an internal id
	Milestone     string // the transition, opaque to this service
	Round         int    // which occurrence of it — see domain.SendKey
	Channel       domain.Channel
	RecipientCode string // opaque citizen identity id. NEVER a phone number
	TemplateCode  string
	Params        domain.NotificationParams

	// Actor is who caused this obligation to exist. An empty ID becomes the system principal:
	// rule 6, invariant 6 requires background work to be audited too, with a system principal,
	// and the producer of a status change is a consumer, not a person. It is deliberately not a
	// silent default on an isolation path — it is the principal rule 6 prescribes for exactly
	// this case, and it is recorded as such rather than left blank.
	Actor audit.Actor
}

// RecordResult is what happened. `Created` false means the fact had already been recorded.
type RecordResult struct {
	ID      string
	Created bool
}

var (
	// ErrMissingSubject — a notification about nothing cannot be looked up, cannot be shown next
	// to a record, and cannot be audited: core/audit refuses an entry with no subject.
	ErrMissingSubject = errors.New("so_thong_bao: thiếu mã hồ sơ")
	// ErrMissingMilestone — a notification about no transition says nothing changed.
	ErrMissingMilestone = errors.New("so_thong_bao: thiếu mốc")
	// ErrMissingRecipient — with no recipient there is nobody to be told, and the deduplication key
	// would be the same for every citizen of the commune.
	ErrMissingRecipient = errors.New("so_thong_bao: thiếu người nhận")
	// ErrMissingTemplate — with no template the adapter has nothing approved to send.
	ErrMissingTemplate = errors.New("so_thong_bao: thiếu mẫu tin")
)

// Record records that the commune owes this citizen one notification.
//
// IT IS IDEMPOTENT, AND THE IDEMPOTENCY IS STRUCTURAL RATHER THAN CHECKED. There is no "read,
// then decide, then write": the deduplication key goes into a UNIQUE constraint and the insert
// says ON CONFLICT DO NOTHING, so two consumers processing the same redelivery at the same moment
// cannot both win. A read-then-write would pass every single-threaded test and produce two
// messages to one citizen under concurrency, which is the only condition it ever fails under.
//
// THE ROW AND ITS AUDIT ENTRY SHARE ONE TRANSACTION (rule 6, invariant 3). If the entry fails,
// the obligation is not recorded either — which is the correct outcome, because the queue will
// redeliver and the whole thing will be attempted again. The opposite ordering leaves a
// commitment in the system that nobody can attribute.
//
// THE AUDIT ENTRY IS WRITTEN ONLY WHEN A ROW WAS ACTUALLY CREATED. See the note at the top of the
// file: auditing every redelivery would record promises that were never made a second time.
func (uc *CitizenNotifications) Record(ctx context.Context, req RecordRequest) (RecordResult, error) {
	if err := validateRecordRequest(req); err != nil {
		return RecordResult{}, err
	}
	// Validated before the transaction opens: a message that fails rule 10's content rule must
	// never hold a row lock while doing so, and the caller needs the reason, not a rollback.
	if err := req.Params.Validate(); err != nil {
		return RecordResult{}, fmt.Errorf("so_thong_bao: nội dung: %w", err)
	}

	round := req.Round
	if round == 0 {
		// The first occurrence. Written as an explicit normalisation rather than accepted as a
		// zero, because `lan` is part of the deduplication key and a 0 from one producer with a 1
		// from another would be two keys for one fact — a duplicate message to a real person.
		round = 1
	}

	key, err := domain.SendKey(req.SubjectType, req.SubjectCode, req.Milestone, round, req.RecipientCode, req.Channel)
	if err != nil {
		return RecordResult{}, fmt.Errorf("so_thong_bao: khoá chống trùng: %w", err)
	}

	id, err := uc.newID()
	if err != nil {
		return RecordResult{}, fmt.Errorf("so_thong_bao: sinh mã: %w", err)
	}

	n := domain.CitizenNotification{
		ID:            id,
		SendKey:       key,
		SubjectType:   req.SubjectType,
		SubjectCode:   req.SubjectCode,
		Milestone:     req.Milestone,
		Round:         round,
		Channel:       req.Channel,
		RecipientCode: req.RecipientCode,
		TemplateCode:  req.TemplateCode,
		Params:        req.Params,
		Status:        domain.SendStatusPending,
	}

	var created bool
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		created, err = uc.repo.Insert(ctx, tx, n)
		if err != nil {
			return err
		}
		if !created {
			// A redelivery. Nothing was written, so nothing is owed a trail — and returning nil
			// commits an empty transaction, which is cheaper and simpler than a rollback that the
			// caller would have to tell apart from a failure.
			return nil
		}
		// SAME TRANSACTION AS THE ROW. The delta carries the transition and the channel and
		// nothing else: `nguoi_nhan_ma` is opaque but it is still a link to a person, and an
		// audit ledger that accumulates one per notification becomes a map of who reported what
		// (rule 6, forbidden #4). The subject already names the record.
		delta, err := json.Marshal(map[string]any{
			"moc": req.Milestone, "lan": round, "kenh": string(req.Channel), "trang_thai": string(domain.SendStatusPending),
		})
		if err != nil {
			return fmt.Errorf("so_thong_bao: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actorOrSystem(req.Actor),
			Action:  "ghi_so_thong_bao",
			Subject: req.SubjectCode, // business code, never the internal id
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no obligation, no trail. The two states agree, and the queue
		// will redeliver.
		return RecordResult{}, fmt.Errorf("so_thong_bao: ghi sổ cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return RecordResult{ID: id, Created: created}, nil
}

// RecordSendResult records what the channel did with one notification.
//
// IDEMPOTENT FOR THE SAME REASON AND BY THE SAME MECHANISM: the UPDATE's predicate excludes rows
// already delivered, so reporting one success twice moves nothing and audits nothing. `false`
// means "already delivered", which on an at-least-once queue is normal operation. A missing id is
// an error and is reported as one.
func (uc *CitizenNotifications) RecordSendResult(ctx context.Context, id string, result domain.SendResult,
	actor audit.Actor, subjectCode string) (bool, error) {

	if id == "" {
		return false, fmt.Errorf("so_thong_bao: thiếu mã bản ghi")
	}
	if subjectCode == "" {
		// The audit entry's subject is the BUSINESS code, and core/audit refuses an entry without
		// one. Taking the internal id instead would produce a trail nobody reading it can match to
		// the piece of paper in front of them.
		return false, ErrMissingSubject
	}
	if err := result.Validate(); err != nil {
		return false, fmt.Errorf("so_thong_bao: kết quả gửi: %w", err)
	}

	var moved bool
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		moved, err = uc.repo.RecordSendResult(ctx, tx, id, result)
		if err != nil {
			return err
		}
		if !moved {
			return nil
		}
		// `nguoi_nhan_che` IS IN THE DELTA AND IT IS MASKED, which is precisely what rule 6
		// invariant 5 asks for: the trail has to be able to answer "which number was told", and
		// the masked form answers it without turning the audit ledger into a personal-data store
		// (rule 6, forbidden #4). domain.SendResult.Validate has already refused an unmasked value,
		// and so has the database.
		delta, err := json.Marshal(map[string]any{
			"trang_thai": string(result.Status),
			"so_lan_thu": result.AttemptCount,
			"nguoi_nhan": result.MaskedRecipient,
			"loi_ma":     result.ErrorCode,
		})
		if err != nil {
			return fmt.Errorf("so_thong_bao: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actorOrSystem(actor),
			Action:  "ghi_ket_qua_thong_bao",
			Subject: subjectCode,
			Delta:   delta,
		})
	})
	if err != nil {
		return false, fmt.Errorf("so_thong_bao: ghi kết quả gửi cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return moved, nil
}

func validateRecordRequest(req RecordRequest) error {
	switch {
	case req.SubjectCode == "":
		return ErrMissingSubject
	case req.Milestone == "":
		return ErrMissingMilestone
	case req.RecipientCode == "":
		return ErrMissingRecipient
	case req.TemplateCode == "":
		return ErrMissingTemplate
	}
	return nil
}

// actorOrSystem fills in the system principal when nobody was named.
//
// Rule 6, invariant 6: background work is audited too, with a system principal. The alternative —
// refusing an entry with no actor — would make a consumer unable to record anything at all, and
// the alternative after that is the one this exists to prevent: somebody passing a staff id that
// did not do the work.
func actorOrSystem(a audit.Actor) audit.Actor {
	if a.ID == "" {
		return audit.Actor{ID: audit.SystemActor, Kind: "system"}
	}
	return a
}

// ulidAlphabet is Crockford's base32 alphabet — no I, L, O or U, so a transcribed identifier cannot
// turn into a different valid one.
const ulidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newULID returns a ULID: 48 bits of millisecond timestamp followed by 80 bits of randomness,
// encoded as 26 characters.
//
// THIS IS THE SECOND COPY IN THE REPOSITORY, AND SAYING SO IS THE POINT. The first is
// service-identity/internal/store/crosstenant/dinh_danh_cong_dan.go:154, whose own comment says
// moving it to core/ is the right call "on the day a second caller appears". That day is today,
// and the move was not made here for a reason that has nothing to do with the code: lifting it
// into core/ and deleting identity's copy touches another service, which this piece of work was
// scoped out of. It is raised with the user instead. Until then the two copies are identical and
// must stay so — a ULID that differs in alphabet or in length between two services is an id that
// fails a CHECK constraint in one of them.
func newULID() (string, error) {
	var b [16]byte

	ms := uint64(time.Now().UTC().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	if _, err := rand.Read(b[6:]); err != nil {
		return "", fmt.Errorf("so_thong_bao: sinh mã ngẫu nhiên: %w", err)
	}

	// 128 bits do not divide into 5-bit groups, so the encoding is of a 130-bit field whose two
	// leading bits are zero: 130 / 5 = 26 characters exactly. `bits` starts at 2 for those bits.
	var out [26]byte
	var acc uint32
	bits := uint(2)
	pos := 0
	for _, by := range b {
		acc = acc<<8 | uint32(by)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out[pos] = ulidAlphabet[(acc>>bits)&31]
			pos++
		}
	}
	return string(out[:]), nil
}
