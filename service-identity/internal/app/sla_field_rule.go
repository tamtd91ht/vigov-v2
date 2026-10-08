package app

// Adding and removing a FIELD'S OWN deadline row — ADR 0079 lô 2 Q4 ("Theo prototype"): a commune
// adds a row for one field of one kind of work, removes it with a reason, never removes the default
// row, and nobody approves either act. `admin.sla` + the audit entry are the control.
//
// =================================================================================================
// WHICH CODES ARE CHECKED, AND WHY TWO OF THE THREE KINDS ARE REFUSED TODAY.
//
// A field code is stored as a VALUE (migration 0008, sla.linh_vuc) and read back by every
// ResolveDeadlines call — which answers a misspelled code with the DEFAULT row, silently. So the code
// is checked HERE, on the write, against the service that owns the list (rule 2, invariant 3):
//
//	phan-anh     tier-1 petition fields, platform, ListPetitionFields (ADR 0060). CHECKED.
//	van-ban-den  document types (`loai_van_ban`), owned by service-documents.   NO CONTRACT.
//	nhiem-vu     task priorities (`muc_uu_tien_nhiem_vu`), service-petitions.  NO CONTRACT.
//
// For the last two there is no RPC identity can ask (proto/vigov/documents/v1/documents.proto and
// petitions/v1/petitions.proto expose only Health and CountOrgUnitHoldings), and identity may not read
// their databases (rule 2, forbidden #2). That is rule 2 STOP CONDITION #2, so those kinds are REFUSED
// with ErrSLAFieldUnverifiable rather than accepted on trust. Accepting them unchecked would store a
// code that a typo makes permanently wrong — falling back to the default deadline with nothing on any
// screen to say so. Lifting this refusal needs a contract (contract-designer), not a change here.
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
	// ErrSLAFieldUnverifiable — the kind of work's field list has no contract identity can ask (see the
	// file comment). Nothing is written.
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

// checkFieldWithOwner asks the list's owner whether the code may be written. See the file comment for
// why two kinds are refused outright.
func (uc *SLA) checkFieldWithOwner(ctx context.Context, kind domain.LoaiViec, field string) error {
	if kind != domain.LoaiViecPhanAnh {
		return ErrSLAFieldUnverifiable
	}
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
