package event

// The consumer of `petitions.merge_changed.v1` — the citizen notice owed when a petition is linked
// to, or unlinked from, a main petition (ADR 0087; ADR 0041 §Sửa đổi 09/10/2026).
//
// IT IS A SECOND CONSUMER AND NOT A BRANCH OF PhieuDoiTrangThai, for the reason the contract gives
// for the event being a second event: a merge changes no status. Read through the status consumer,
// it would have to be recorded under the petition's CURRENT status, landing on the deduplication key
// of a notice the citizen already received — and be dropped as a duplicate, silently, looking
// exactly like correct deduplication. Here `moc` is the act (`gop-phieu` / `tach-phieu`), which is
// not one of the nine status codes of ADR 0027, so the two families of key cannot meet.
//
// EVERYTHING ELSE IS THE STATUS CONSUMER'S SHAPE, ON PURPOSE, and its comments are the reasoning:
// the same refusals, the same non-retryable marker, the same ledger use case, the same single entry
// through core/events.Dispatch. The shared sentinels (ErrKhongThuLai, ErrThieuMaTraCuu, ...) are
// reused rather than redeclared so a queue adapter has ONE set of values to classify, whichever
// event failed.
//
// NOT WIRED to a broker, for the same reason as the status consumer (cmd/server/main.go, step 5):
// no Kafka client and no event.MauTinXa implementation exist yet.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/events"
	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// EventPetitionMergeChanged is the ONE name this consumer accepts — version included, for the
// reason written on TenSuKienPhieuDoiTrangThai.
const EventPetitionMergeChanged = "petitions.merge_changed.v1"

// The two acts the contract names (`PetitionMergeChanged.kind`, owned by `petitions` as
// `domain.MergeKind`). They are ALSO the template keys MauTinXa is asked for: each commune gets a
// separate approved ZNS template per act (ADR 0018), because "your report was merged" and "your
// report was separated again" are different sentences to a citizen.
const (
	MergeKindMerge   = "gop-phieu"
	MergeKindUnmerge = "tach-phieu"
)

// ErrUnknownMergeKind — `kind` is neither of the two acts.
//
// THIS IS THE ONE CHECK THE STATUS CONSUMER DOES NOT HAVE, and the asymmetry is deliberate. The
// status list belongs to `petitions` and this service must not copy it (migration 0004, `moc`). The
// merge kinds are different: the whole reason this event exists is that its `moc` CANNOT COLLIDE
// with a status code. An unvalidated kind of, say, `da-dong` would write a row under the key of the
// citizen's closing notice — and the real closing notice, arriving later, would be dropped as its
// duplicate. Two values are a contract this consumer depends on for correctness, not a copy of a
// list it merely displays.
var ErrUnknownMergeKind = errors.New("su_kien: loại gộp/tách phiếu không hợp lệ")

// PetitionMergeChangedConsumer consumes `petitions.merge_changed.v1`.
type PetitionMergeChangedConsumer struct {
	ledger    SoThongBao
	templates MauTinXa
	log       *slog.Logger
}

func NewPetitionMergeChangedConsumer(ledger SoThongBao, templates MauTinXa, log *slog.Logger) *PetitionMergeChangedConsumer {
	if log == nil {
		log = slog.Default()
	}
	return &PetitionMergeChangedConsumer{ledger: ledger, templates: templates, log: log}
}

// Nhan is the ONLY way in: core/events.Dispatch refuses an envelope with no commune and puts the
// envelope's commune into the context (rule 1, invariants 4 and 9). The handler is unexported so
// nothing can skip that step. The name matches PhieuDoiTrangThai.Nhan so one queue adapter calls
// both the same way.
func (c *PetitionMergeChangedConsumer) Nhan(ctx context.Context, e events.Envelope) error {
	return events.Dispatch(ctx, e, c.handle)
}

func (c *PetitionMergeChangedConsumer) handle(ctx context.Context, e events.Envelope) error {
	if e.Name != EventPetitionMergeChanged {
		return fmt.Errorf("%w: %w: %q", ErrKhongThuLai, ErrSaiTenSuKien, e.Name)
	}

	var msg petitionsv1.PetitionMergeChanged
	// DiscardUnknown: the publisher may add an optional field to a live event (rule 2, invariant 4).
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(e.Payload, &msg); err != nil {
		// The decoder's error is NOT wrapped: it quotes the body, and a body that broke the
		// contract is the one that may carry what the contract forbids (rule 3). The envelope id is
		// what an operator needs.
		return fmt.Errorf("%w: %w: phong bì %q", ErrKhongThuLai, ErrGiaiMaPayload, e.ID)
	}

	if err := validateMergeChanged(&msg); err != nil {
		return fmt.Errorf("%w: %w", ErrKhongThuLai, err)
	}

	// No citizen_message: the act owes this citizen nothing (ADR 0041 §Sửa đổi 09/10/2026 decides
	// which acts do). A normal outcome — no row, no send, no log line.
	if msg.GetCitizenMessage() == nil {
		return nil
	}

	templateCode, err := c.templates.MauTin(ctx, msg.GetKind())
	if err != nil {
		if errors.Is(err, ErrXaChuaCauHinhKenh) {
			// ADR 0006 consequence 3: degrade VISIBLY, and do not retry what cannot succeed.
			// Logged: the lookup code (a record), the kind (a code), the commune (a ULID). NOT the
			// citizen id — paired with a record code it starts a map of who reported what (rule 3).
			c.log.Warn("chưa gửi được thông báo: xã chưa cấu hình kênh",
				"su_kien", e.Name, "xa", string(tenant.MustFrom(ctx)),
				"ma_tra_cuu", msg.GetLookupCode(), "moc", msg.GetKind())
			return fmt.Errorf("%w: %w", ErrKhongThuLai, err)
		}
		// Anything else (configuration store unreachable) is worth retrying.
		return fmt.Errorf("su_kien: mẫu tin của xã: %w", err)
	}

	// Idempotency is the ledger's: app.GhiNo builds domain.KhoaLanGui from these fields and a
	// UNIQUE (tenant_id, khoa_lan_gui) decides. A republication under a fresh envelope id hands
	// over the same fields and lands on the same row; `occurrence` is what lets a second merge of
	// the same petition owe a second row.
	if _, err := c.ledger.GhiNo(ctx, app.YeuCauGhiNo{
		DoiTuongLoai: domain.DoiTuongPhieuPhanAnh,
		DoiTuongMa:   msg.GetLookupCode(),
		Moc:          msg.GetKind(),
		Lan:          int(msg.GetOccurrence()),
		Kenh:         domain.KenhZaloZNS,
		NguoiNhanMa:  msg.GetCitizenId(),
		MauMa:        templateCode,
		ThamSo: domain.ThamSoThongBao{
			MaTraCuu: msg.GetLookupCode(),
			// The label of the ACT, carried in the field named status_label (see its comment in
			// events.proto). To the citizen it is "what just happened to my petition" either way.
			MocNhan:      msg.GetCitizenMessage().GetStatusLabel(),
			ViecTiepTheo: msg.GetCitizenMessage().GetNextStep(),
		},
		// NguoiGay left zero: app.GhiNo records the system principal (rule 6, invariant 6). The
		// staff member who merged was audited by `petitions`, in the merge's own transaction.
	}); err != nil {
		return fmt.Errorf("su_kien: ghi sổ thông báo: %w", err)
	}
	return nil
}

// validateMergeChanged refuses, never repairs, a body the contract says cannot happen.
func validateMergeChanged(msg *petitionsv1.PetitionMergeChanged) error {
	switch {
	case msg.GetLookupCode() == "":
		return ErrThieuMaTraCuu
	case msg.GetKind() == "":
		return ErrThieuMoc
	case msg.GetKind() != MergeKindMerge && msg.GetKind() != MergeKindUnmerge:
		// The value is not quoted back: it is producer-controlled text on a path that reaches logs.
		return ErrUnknownMergeKind
	case msg.GetCitizenId() == "":
		return ErrThieuNguoiNhan
	case msg.GetOccurrence() == 0:
		// Never defaulted to 1: the second merge of a petition would collapse onto the first.
		return ErrLanKhongHopLe
	}
	return nil
}
