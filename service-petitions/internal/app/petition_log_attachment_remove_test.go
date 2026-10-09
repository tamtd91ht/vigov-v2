package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// PetitionLogAttachments.Remove — the soft delete of one petition log attachment (09/10/2026).
//
//	PROVED HERE   only a file the caller can SEE is removed (this petition's log attachment, stored, on an
//	              entry or the caller's own draft) — everything else is the one ErrAttachmentNotFound and
//	              writes NOTHING · `can-bo` without the key is refused first · legal hold refuses · the
//	              reason is mandatory and checked before any transaction · the soft delete carries the
//	              BUSINESS CODE and the reason · ONE transaction holds the soft delete, the timeline line
//	              and the audit entry · the line and the trail carry neither the file name nor the reason's
//	              text in the trail · a second removal is the 404.
//	NOT PROVED    PostgreSQL (0021's stored_file_delete_complete, 0027's append-only link).

// --- the two fakes' removal halves -------------------------------------------------------------------

func (p *vpPetitions) GhiNhatKy(_ context.Context, tx *pkgstore.ScopedTx, e domain.NhatKyPhanAnh) error { // vi-name-ok: implements the existing store method
	if tx == nil {
		return errors.New("timeline row written outside a transaction")
	}
	p.logged = append(p.logged, e)
	return nil
}

func (f *vpFiles) LinkedPetitionLogEntryTx(_ context.Context, tx *pkgstore.ScopedTx, id string) (string, error) {
	if tx == nil {
		return "", errors.New("link read outside the transaction")
	}
	return f.links[id], nil
}

// SoftDelete hides the row from every later read, as `deleted_at IS NULL` does in the store.
func (f *vpFiles) SoftDelete(_ context.Context, tx *pkgstore.ScopedTx, id, by, reason string, _ time.Time) error {
	rows := f.all(tx.TenantID())
	if _, ok := rows[id]; !ok {
		return petstore.ErrStoredFileMoved
	}
	delete(rows, id)
	f.softDeleted = append(f.softDeleted, id+"|"+by+"|"+reason)
	return nil
}

// --- fixtures ------------------------------------------------------------------------------------------

const removeReason = "Tải nhầm biên bản của phiếu khác"

func (h *laHarness) seedLogFile(id, uploader string, linked string, mutate func(*domain.StoredFile)) {
	f := domain.StoredFile{ID: id, Bucket: domain.StoredFileBucketPrivate, ObjectKey: "records/k/" + id + "/original.pdf",
		Purpose: domain.PurposePetitionLogAttachment, SubjectType: domain.StoredFileSubjectPetition,
		SubjectID: ppPetition, Status: domain.StoredFileStored, UploadedBy: uploader, OriginalName: "bien-ban-nha-ong-hai.pdf"}
	if mutate != nil {
		mutate(&f)
	}
	h.files.all(xaThu)[id] = f
	if linked != "" {
		h.files.links[id] = linked
	}
}

func (h *laHarness) nothingWritten(t *testing.T) {
	t.Helper()
	if len(h.files.softDeleted) != 0 || len(h.pets.logged) != 0 || len(h.db.audits()) != 0 {
		t.Errorf("written although refused: deleted %v, logged %d, audits %d",
			h.files.softDeleted, len(h.pets.logged), len(h.db.audits()))
	}
}

// --- cases ---------------------------------------------------------------------------------------------

func TestRemoveLogAttachment_SoftDeleteLineAndTrailInOneTransaction(t *testing.T) {
	h := buildLogAttachments(t)
	h.uc.logID = func() (string, error) { return "nkpa-go-1", nil }
	h.seedLogFile("f1", "CB-00999", "nkpa-1", nil) // another officer's file, ON an entry: visible to all staff

	if err := h.uc.Remove(h.ctx, vpCode, "f1", "  "+removeReason+"  ", canBoThu(), coQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(h.files.softDeleted) != 1 || h.files.softDeleted[0] != "f1|"+maCanBoThu+"|"+removeReason {
		t.Errorf("soft delete = %v — the file, the remover's BUSINESS CODE, the trimmed reason", h.files.softDeleted)
	}
	if len(h.pets.logged) != 1 {
		t.Fatalf("timeline rows = %d, want 1", len(h.pets.logged))
	}
	line := h.pets.logged[0]
	if line.ID != "nkpa-go-1" || line.PhieuPhanAnhID != ppPetition || line.NguoiMa != maCanBoThu ||
		line.HanhVi != domain.LogActionAttachmentRemoved || line.TrangThai != domain.DaDong {
		t.Errorf("line = %+v", line)
	}
	if !strings.Contains(line.NoiDung, removeReason) || strings.Contains(line.NoiDung, "bien-ban-nha-ong-hai") {
		t.Errorf("line text = %q — the reason, never the file name", line.NoiDung)
	}
	a := h.db.audits()
	if len(a) != 1 || a[0][0] != maCanBoThu || a[0][2] != ActionPetitionLogAttachmentRemoved || a[0][3] != vpCode {
		t.Fatalf("trail = %v", a)
	}
	for _, want := range []string{`"tep_id":"f1"`, `"nhat_ky_id":"nkpa-go-1"`, `"gan_nhat_ky":"nkpa-1"`, `"do_dai_ly_do":`,
		`"quyen_go":"ket-thuc-xu-ly-phan-anh"`} {
		if !strings.Contains(a[0][4], want) {
			t.Errorf("delta %s lacks %s", a[0][4], want)
		}
	}
	if strings.Contains(a[0][4], removeReason) || strings.Contains(a[0][4], "bien-ban") {
		t.Errorf("delta carries the reason or the file name: %s", a[0][4])
	}
	if h.db.begun != 1 || h.db.rolled != 0 {
		t.Errorf("transactions begun %d rolled %d, want one committed", h.db.begun, h.db.rolled)
	}

	// THE SECOND REMOVAL is the one 404 — it cannot overwrite who removed the file or why.
	if err := h.uc.Remove(h.ctx, vpCode, "f1", removeReason, canBoThu(), khongQuyenCaXa, khongQuyenHanChe); !errors.Is(err, ErrAttachmentNotFound) {
		t.Errorf("second removal: %v", err)
	}
	if len(h.pets.logged) != 1 {
		t.Error("a second timeline line was written")
	}
}

func TestRemoveLogAttachment_OwnDraftIsRemovable(t *testing.T) {
	h := buildLogAttachments(t)
	h.seedLogFile("f1", maCanBoThu, "", nil)
	if err := h.uc.Remove(h.ctx, vpCode, "f1", removeReason, canBoThu(), khongQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatalf("own draft: %v", err)
	}
	if a := h.db.audits(); len(a) != 1 || !strings.Contains(a[0][4], `"gan_nhat_ky":""`) ||
		!strings.Contains(a[0][4], `"quyen_go":"nguoi-tai-len"`) {
		t.Errorf("trail = %v — a draft is on no entry", a)
	}
}

// THE TWO DOORS (owner decision 09/10/2026): somebody else's file on an entry is visible to every staff
// reader, and removable only with `feedback.resolve`; the uploader needs no key beyond the gate.
func TestRemoveLogAttachment_UploaderOrResolve(t *testing.T) {
	h := buildLogAttachments(t)
	h.seedLogFile("f1", "CB-00999", "nkpa-1", nil)
	if err := h.uc.Remove(h.ctx, vpCode, "f1", removeReason, canBoThu(), khongQuyenCaXa, khongQuyenHanChe); !errors.Is(err, domain.ErrAttachmentRemovalNotAllowed) {
		t.Fatalf("neither uploader nor resolve: %v", err)
	}
	h.nothingWritten(t)

	h = buildLogAttachments(t)
	h.seedLogFile("f1", maCanBoThu, "nkpa-1", nil)
	if err := h.uc.Remove(h.ctx, vpCode, "f1", removeReason, canBoThu(), khongQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatalf("the uploader without resolve: %v", err)
	}
}

func TestRemoveLogAttachment_InvisibleFileIsTheOne404AndWritesNothing(t *testing.T) {
	for name, seed := range map[string]func(h *laHarness){
		"unknown id":                         func(*laHarness) {},
		"another officer's unattached draft": func(h *laHarness) { h.seedLogFile("f1", "CB-00999", "", nil) },
		"another petition's file": func(h *laHarness) {
			h.seedLogFile("f1", maCanBoThu, "nkpa-1", func(f *domain.StoredFile) { f.SubjectID = "pa-other" })
		},
		"a verification photo": func(h *laHarness) {
			h.seedLogFile("f1", maCanBoThu, "", func(f *domain.StoredFile) { f.Purpose = domain.PurposePetitionVerificationPhoto })
		},
		"a citizen scene photo": func(h *laHarness) {
			h.seedLogFile("f1", domain.CitizenLogActor, "", func(f *domain.StoredFile) { f.Purpose = domain.PurposePetitionPhoto })
		},
		"not stored yet": func(h *laHarness) {
			h.seedLogFile("f1", maCanBoThu, "", func(f *domain.StoredFile) { f.Status = domain.StoredFilePending })
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := buildLogAttachments(t)
			seed(h)
			if err := h.uc.Remove(h.ctx, vpCode, "f1", removeReason, canBoThu(), khongQuyenCaXa, khongQuyenHanChe); !errors.Is(err, ErrAttachmentNotFound) {
				t.Fatalf("err = %v, want ErrAttachmentNotFound", err)
			}
			h.nothingWritten(t)
		})
	}
}

func TestRemoveLogAttachment_Refusals(t *testing.T) {
	t.Run("legal hold", func(t *testing.T) {
		h := buildLogAttachments(t)
		h.seedLogFile("f1", maCanBoThu, "nkpa-1", func(f *domain.StoredFile) { f.LegalHold = true })
		if err := h.uc.Remove(h.ctx, vpCode, "f1", removeReason, canBoThu(), khongQuyenCaXa, khongQuyenHanChe); !errors.Is(err, domain.ErrAttachmentUnderLegalHold) {
			t.Fatalf("err = %v", err)
		}
		h.nothingWritten(t)
	})
	t.Run("no reason", func(t *testing.T) {
		h := buildLogAttachments(t)
		h.seedLogFile("f1", maCanBoThu, "nkpa-1", nil)
		if err := h.uc.Remove(h.ctx, vpCode, "f1", " \t ", canBoThu(), khongQuyenCaXa, khongQuyenHanChe); !errors.Is(err, domain.ErrAttachmentRemovalReasonMissing) {
			t.Fatalf("err = %v", err)
		}
		if h.db.begun != 0 {
			t.Error("a transaction opened for a removal with no reason")
		}
	})
	t.Run("restricted field without the key", func(t *testing.T) {
		h := buildLogAttachments(t)
		h.pets.byCommune[xaThu]["PA-CANBO-0000-0001"] = domain.PhieuPhanAnh{ID: ppPetition, MaTraCuu: "PA-CANBO-0000-0001",
			LinhVuc: domain.LinhVucHanChe, TrangThai: domain.DangXuLy}
		h.seedLogFile("f1", maCanBoThu, "nkpa-1", nil)
		if err := h.uc.Remove(h.ctx, "PA-CANBO-0000-0001", "f1", removeReason, canBoThu(), khongQuyenCaXa, khongQuyenHanChe); !errors.Is(err, ErrPhieuHanChe) {
			t.Fatalf("err = %v", err)
		}
		h.nothingWritten(t)
	})
	t.Run("another commune", func(t *testing.T) {
		h := buildLogAttachments(t)
		h.seedLogFile("f1", maCanBoThu, "nkpa-1", nil)
		if err := h.uc.Remove(ctxXa(xaKia), vpCode, "f1", removeReason, canBoThu(), khongQuyenCaXa, khongQuyenHanChe); !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
			t.Fatalf("err = %v", err)
		}
		h.nothingWritten(t)
		if _, still := h.files.all(xaThu)["f1"]; !still {
			t.Error("commune A's file touched from commune B")
		}
	})
	t.Run("no business code", func(t *testing.T) {
		h := buildLogAttachments(t)
		h.seedLogFile("f1", maCanBoThu, "nkpa-1", nil)
		actor := canBoThu()
		actor.ID = ""
		if err := h.uc.Remove(h.ctx, vpCode, "f1", removeReason, actor, khongQuyenCaXa, khongQuyenHanChe); err == nil {
			t.Fatal("removed with no business code to put in deleted_by")
		}
		h.nothingWritten(t)
	})
}
