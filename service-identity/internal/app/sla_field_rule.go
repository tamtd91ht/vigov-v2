package app

// Adding and removing a FIELD'S OWN deadline row — ADR 0079 lô 2 Q4 ("Theo prototype"): a commune
// adds a row for one field of one kind of work, removes it with a reason, never removes the default
// row, and nobody approves either act. `admin.sla` + the audit entry are the control.
//
// =================================================================================================
// WHICH CODES ARE CHECKED.
//
// A field code is stored as a VALUE (migration 0008, sla.linh_vuc) and read back by every
// ResolveDeadlines call — which answers a misspelled code with the DEFAULT row, silently. So the code
// is checked HERE, on the write, against the service that owns the list (rule 2, invariant 3):
//
//	phan-anh     tier-1 petition fields, platform, ListPetitionFields (ADR 0060).           CHECKED.
//	van-ban-den  document types (`loai_van_ban`), service-documents,
//	             ResolveDocumentTypeCodes (ADR 0079 lô 2 Q4).                              CHECKED.
//	nhiem-vu     task priorities (`muc_uu_tien_nhiem_vu`), service-petitions,
//	             ResolveTaskPriorityCodes (ADR 0079 lô 2 Q4).                              CHECKED.
//
// All three share one rule, fail closed: an active code is accepted; a code the owner does not know,
// or has switched off, is ErrSLAFieldNotInList (400); the owner not answering — or not wired at all —
// is ErrSLAFieldListUnavailable (503), never "accept unchecked". `don-thu` takes no field rows at all
// (domain.CheckSLAKindTakesFieldRows refuses it before any owner is asked).
//
// ErrSLAFieldUnverifiable remains for a kind that takes field rows but has no owner check written
// here: unreachable today, and the refusal a future kind gets until somebody writes its check —
// rather than falling through to "accept unchecked".
//
// =================================================================================================
// NEITHER ACT TOUCHES A DEADLINE ALREADY ISSUED (rule 10, invariant 2; ADR 0028). A row added or
// removed changes what the NEXT act that fixes a deadline reads; stored deadlines stay as they were.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The verbs in the trail — Vietnamese snake_case like every other action this system writes.
const (
	ActionAddSLAFieldRow    = "them_thoi_han_xu_ly_rieng"
	ActionRemoveSLAFieldRow = "xoa_thoi_han_xu_ly_rieng"
)

var (
	// ErrSLAFieldUnverifiable — the kind of work takes field rows but has no owner check written here
	// (see the file comment; unreachable for the kinds that exist today). Nothing is written.
	ErrSLAFieldUnverifiable = errors.New("sla: chưa đối chiếu được mã lĩnh vực của loại việc này")

	// ErrSLAFieldNotInList — the owner of the list does not know the code, or has retired it.
	ErrSLAFieldNotInList = errors.New("sla: mã lĩnh vực không có trong danh mục, hoặc đã ngừng dùng")

	// ErrSLAFieldListUnavailable — the owner of the list could not be asked. Retryable (503); never
	// read as "valid" and never as "unknown".
	ErrSLAFieldListUnavailable = errors.New("sla: chưa hỏi được danh mục lĩnh vực")

	// ErrSLAFieldRowExists — this commune already has a live row for this kind and field.
	ErrSLAFieldRowExists = errors.New("sla: lĩnh vực này đã có thời hạn riêng")
)

// PetitionFieldSource is tier 1 of the petition-field catalogue. *platformclient.PetitionFields
// satisfies it in production.
type PetitionFieldSource interface {
	Set(ctx context.Context) (platformclient.PetitionFieldSet, error)
}

// DocumentTypeSource answers which document-type codes a live type of the commune in ctx carries, and
// whether each is switched on (present → `dang_dung`; absent → no live type). *documentsclient.Client
// satisfies it in production.
type DocumentTypeSource interface {
	DocumentTypeCodes(ctx context.Context, codes []string) (map[string]bool, error)
}

// TaskPrioritySource answers which task-priority codes a live priority of the commune in ctx carries,
// and whether each is switched on (present → `dang_dung`; absent → no live priority).
// *petitionsclient.Client satisfies it in production.
type TaskPrioritySource interface {
	TaskPriorityCodes(ctx context.Context, codes []string) (map[string]bool, error)
}

// WithTaskPriorities wires the task-priority check. Without it every `nhiem-vu` add answers
// ErrSLAFieldListUnavailable — fail closed, never "accept unchecked".
func (uc *SLA) WithTaskPriorities(src TaskPrioritySource) *SLA {
	uc.taskPriorities = src
	return uc
}

// WithDocumentTypes wires the document-type check. Without it every `van-ban-den` add answers
// ErrSLAFieldListUnavailable — fail closed, never "accept unchecked".
func (uc *SLA) WithDocumentTypes(src DocumentTypeSource) *SLA {
	uc.documentTypes = src
	return uc
}

// WithPetitionFields wires the tier-1 reader. Without it every `phan-anh` add answers
// ErrSLAFieldListUnavailable — fail closed, never "accept unchecked".
func (uc *SLA) WithPetitionFields(src PetitionFieldSource) *SLA {
	uc.petitionFields = src
	return uc
}

// AddFieldRuleRequest is one new field row. All five figures are required — a new row has nothing to
// inherit them from; the sixth is optional (0 = NULL, "do not report").
type AddFieldRuleRequest struct {
	Kind  domain.LoaiViec
	Field string

	AcknowledgeHours       int
	ResolveHours           int
	DueSoonHours           int
	EscalateLeaderHours    int
	EscalatePresidentHours int
	UnassignedHoldHours    int
}

// AddFieldRule inserts a field's own row.
//
// ORDER: shape (no I/O) → the list owner (network, OUTSIDE the transaction — a database transaction
// held open across a gRPC call is a lock held for as long as another service chooses) → insert +
// audit in one transaction. `UNIQUE (tenant_id, loai_viec, linh_vuc_khoa)` is the guard against a
// duplicate, including two administrators adding the same field at once.
func (uc *SLA) AddFieldRule(ctx context.Context, req AddFieldRuleRequest, nguoi NguoiThucHien) (domain.DongSLA, error) {
	if err := nguoi.hopLe(); err != nil {
		return domain.DongSLA{}, err
	}
	if err := domain.CheckSLAKindTakesFieldRows(req.Kind); err != nil {
		return domain.DongSLA{}, err
	}
	field, err := domain.NormalizeSLAField(req.Field)
	if err != nil {
		return domain.DongSLA{}, err
	}
	// A zero sixth figure means "do not report"; a NEGATIVE one is a typo, refused rather than read
	// as zero.
	if req.UnassignedHoldHours < 0 {
		return domain.DongSLA{}, domain.ErrGioPhaiDuong
	}
	row := domain.DongSLA{
		LoaiViec: req.Kind, LinhVuc: field,
		GioTiepNhan: req.AcknowledgeHours, GioXuLyXong: req.ResolveHours, GioSapDenHan: req.DueSoonHours,
		GioBaoLanhDao: req.EscalateLeaderHours, GioBaoChuTich: req.EscalatePresidentHours,
		UnassignedHoldHours: req.UnassignedHoldHours,
	}
	if err := domain.KiemTraDongSLA(row); err != nil {
		return domain.DongSLA{}, err
	}
	if err := uc.checkFieldWithOwner(ctx, req.Kind, field); err != nil {
		return domain.DongSLA{}, err
	}
	if row.ID, err = uc.sinhID(); err != nil {
		return domain.DongSLA{}, fmt.Errorf("sla: sinh id dòng riêng: %w", err)
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.kho.Chen(ctx, tx, row); err != nil {
			return err
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). The subject names the commitment
		// (`phan-anh/an-ninh`), not the ULID — see chuDeSLA.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  ActionAddSLAFieldRow,
			Subject: chuDeSLA(row),
			Delta:   deltaSLA(map[string]any{"sau": soGioCuaDong(row)}),
		})
	})
	if errors.Is(err, idstore.ErrDongSLATrungKhoa) {
		return domain.DongSLA{}, ErrSLAFieldRowExists
	}
	if err != nil {
		return domain.DongSLA{}, err
	}
	return row, nil
}

// checkFieldWithOwner asks the list's owner whether the code may be written. A kind with no check here
// is refused outright (see the file comment).
func (uc *SLA) checkFieldWithOwner(ctx context.Context, kind domain.LoaiViec, field string) error {
	switch kind {
	case domain.LoaiViecPhanAnh:
		return uc.checkPetitionField(ctx, field)
	case domain.LoaiViecVanBanDen:
		return uc.checkDocumentType(ctx, field)
	case domain.LoaiViecNhiemVu:
		return uc.checkTaskPriority(ctx, field)
	default:
		return ErrSLAFieldUnverifiable
	}
}

// checkDocumentType asks service-documents about ONE code, exactly as it will be stored (already
// normalised by the caller — the server matches exactly). Not cached: a cached "active" would let a
// row through for a type switched off a moment ago (documents.proto).
func (uc *SLA) checkDocumentType(ctx context.Context, field string) error {
	if uc.documentTypes == nil {
		return ErrSLAFieldListUnavailable
	}
	answered, err := uc.documentTypes.DocumentTypeCodes(ctx, []string{field})
	if err != nil {
		// Including ErrDocumentsUnavailable: documents could not be asked. The wrapped cause stays for
		// the log; the sentinel decides the status.
		return fmt.Errorf("%w: %w", ErrSLAFieldListUnavailable, err)
	}
	// Absent (unknown, soft-deleted, another commune's) and switched off are one refusal.
	if active, ok := answered[field]; !ok || !active {
		return ErrSLAFieldNotInList
	}
	return nil
}

// checkTaskPriority asks service-petitions about ONE code, exactly as it will be stored — the twin of
// checkDocumentType. Not cached, for the same reason.
func (uc *SLA) checkTaskPriority(ctx context.Context, field string) error {
	if uc.taskPriorities == nil {
		return ErrSLAFieldListUnavailable
	}
	answered, err := uc.taskPriorities.TaskPriorityCodes(ctx, []string{field})
	if err != nil {
		// Including ErrPetitionsUnavailable: petitions could not be asked. The wrapped cause stays for
		// the log; the sentinel decides the status.
		return fmt.Errorf("%w: %w", ErrSLAFieldListUnavailable, err)
	}
	// Absent (unknown, soft-deleted, another commune's) and switched off are one refusal.
	if active, ok := answered[field]; !ok || !active {
		return ErrSLAFieldNotInList
	}
	return nil
}

func (uc *SLA) checkPetitionField(ctx context.Context, field string) error {
	if uc.petitionFields == nil {
		return ErrSLAFieldListUnavailable
	}
	set, err := uc.petitionFields.Set(ctx)
	if err != nil {
		// Including ErrPetitionFieldsUnavailable: the platform could not be asked. The wrapped cause
		// stays for the log; the sentinel decides the status.
		return fmt.Errorf("%w: %w", ErrSLAFieldListUnavailable, err)
	}
	if err := set.CheckForIntake(field); err != nil {
		return fmt.Errorf("%w: %w", ErrSLAFieldNotInList, err)
	}
	return nil
}

// RemoveFieldRule soft deletes one field's own row, with a reason. The default row is refused.
//
// A ROW ALREADY REMOVED, AN INVENTED ID AND ANOTHER COMMUNE'S ROW ARE ONE ANSWER — not found — because
// every statement is scoped to the commune in the context (rule 1, invariant 5; rule 4, forbidden #2).
func (uc *SLA) RemoveFieldRule(ctx context.Context, id, reason string, nguoi NguoiThucHien) error {
	if err := nguoi.hopLe(); err != nil {
		return err
	}
	if id == "" {
		return idstore.ErrDongSLAKhongTonTai
	}
	reason, err := domain.NormalizeSLADeleteReason(reason)
	if err != nil {
		return err
	}

	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// FOR UPDATE: an edit of the same row racing this removal would otherwise write figures onto a
		// row that is gone, and audit it as an edit.
		before, err := uc.kho.TheoIDDeGhi(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := domain.CheckSLARowRemovable(before); err != nil {
			return err
		}
		// deleted_by is the STAFF CODE (rule 6, invariant 8), the same value as the entry's actor.
		if err := uc.kho.SoftDelete(ctx, tx, before.ID, nguoi.Vet.ID, reason); err != nil {
			return err
		}
		// The reason is in the entry as well as in the column: the column is the row's state, the entry
		// the append-only record of the act.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  ActionRemoveSLAFieldRow,
			Subject: chuDeSLA(before),
			Delta: deltaSLA(map[string]any{
				"truoc":   soGioCuaDong(before),
				"ly_do":   reason,
				"xoa_mem": true,
			}),
		})
	})
}

// IsSLAFieldRuleInputError reports a refusal of what the client sent (400), for the two routes in this
// file — listed explicitly, so a database outage can never become a 400.
func IsSLAFieldRuleInputError(err error) bool {
	for _, e := range []error{
		domain.ErrSLAFieldMissing, domain.ErrSLAFieldMalformed,
		domain.ErrSLAKindHasNoFieldRows, domain.ErrSLAKindUnknown,
		domain.ErrSLADeleteReasonMissing, domain.ErrSLADeleteReasonTooLong,
	} {
		if errors.Is(err, e) {
			return true
		}
	}
	return LaLoiDauVaoSLA(err)
}
