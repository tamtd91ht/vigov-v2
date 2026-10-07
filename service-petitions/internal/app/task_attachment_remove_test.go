package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for TaskAttachments.Remove (`Gỡ`, user decision 07/10/2026) over the REAL stores on the fake
// driver.
//
//	PROVED HERE   who: the uploader without `task.update` may · a `task.update` holder may remove another
//	              officer's attached file · a reader who is neither gets ErrAttachmentRemovalNotAllowed ·
//	              somebody else's unattached draft, another commune's row, another task's path, an unknown
//	              id, a not-yet-stored file and an already-removed file all answer ErrAttachmentNotFound ·
//	              legal hold refuses · a blank reason refuses before any transaction · the soft delete
//	              (deleted_by = staff code, trimmed reason), the timeline line (file name + reason) and the
//	              audit entry are written in ONE transaction, the trail carrying the reason's LENGTH and
//	              neither the reason nor the name · the download answers 404 afterwards · a second removal
//	              writes nothing.
//	NOT PROVED    migration 0021's CHECK and trigger on the soft-delete columns — stored_file_pg_test.go
//	              (SKIPS without VIGOV_TEST_DSN).

const (
	removalReason = "Tải nhầm tệp, đã thay bằng bản đúng."
	logIDThu      = "01JNHATKYGOTEPTRONGTEST00"
)

func seedStoredAttached(k *khoNhiemVuGia, uploader string, linked bool, mod map[string]any) {
	r := storedFileRow(string(xaThu), fileIDThu, idNVGoc, uploader, domain.StoredFileStored, mocTaoNV)
	for col, v := range mod {
		r[col] = v
	}
	k.addStoredFile(r)
	if linked {
		if k.fileLinks == nil {
			k.fileLinks = map[string]string{}
		}
		k.fileLinks[fileIDThu] = "nk-1"
	}
}

func TestRemove_UploaderSoftDeletesLogsAndAuditsInOneTransaction(t *testing.T) {
	k := khoNVMau()
	uc, obj, _, _, ctx := dungTaskAttachments(t, k)
	uc.newLogID = func() (string, error) { return logIDThu, nil }
	// The uploader is a related officer, not the assignee, and holds no `task.update`.
	seedStoredAttached(k, maLanhDao, true, nil)

	if err := uc.Remove(ctx, maNVGoc, fileIDThu, "  "+removalReason+"  ", staffActor(maLanhDao), false); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	row := k.storedFile(fileIDThu)
	if row["deleted_at"] == nil || row["deleted_by"] != maLanhDao || row["delete_reason"] != removalReason {
		t.Errorf("soft delete = at %v by %v reason %q — want the staff code and the trimmed reason",
			row["deleted_at"], row["deleted_by"], row["delete_reason"])
	}
	if row["status"] != "stored" {
		t.Errorf("status moved to %v — a removal hides, it does not change the file's status", row["status"])
	}
	if len(obj.purged) != 0 {
		t.Error("the object was purged — a records file is never removed by a command (ADR 0052 §6)")
	}

	logs := k.cau("INSERT INTO nhat_ky_nhiem_vu")
	if len(logs) != 1 {
		t.Fatalf("%d timeline rows, want 1", len(logs))
	}
	text, _ := logs[0].args[8].(string)
	if logs[0].args[1] != logIDThu || logs[0].args[2] != idNVGoc || logs[0].args[4] != maLanhDao ||
		!strings.Contains(text, removalReason) || !strings.Contains(text, "gỡ") {
		t.Errorf("timeline row = %v", logs[0].args)
	}
	// Decree 13: the name lives only in stored_file.original_name, the column an anonymisation can
	// rewrite — never in the append-only log.
	if strings.Contains(text, "Biên bản") || strings.Contains(text, "nghiệm thu") {
		t.Errorf("the file name was copied into the append-only log: %q", text)
	}
	chiGhiTrongGiaoDich(t, k)

	vet := vetKiemToan(t, k)
	if vet.args[1] != maLanhDao || vet.args[4] != ActionTaskAttachmentRemoved || vet.args[5] != maNVGoc {
		t.Errorf("trail: actor=%v action=%v subject=%v", vet.args[1], vet.args[4], vet.args[5])
	}
	delta := auditDeltaText(t, k)
	for _, want := range []string{`"tep_id":"` + fileIDThu + `"`, `"nhat_ky_id":"` + logIDThu + `"`,
		`"gan_nhat_ky":"nk-1"`, `"quyen_go":"nguoi-tai-len"`, `"da_go":true`,
		fmt.Sprintf(`"do_dai_ly_do":%d`, utf8.RuneCountInString(removalReason))} {
		if !strings.Contains(delta, want) {
			t.Errorf("delta lacks %s: %s", want, delta)
		}
	}
	if strings.Contains(delta, "Biên bản") || strings.Contains(delta, "Tải nhầm") {
		t.Errorf("the name or the reason text leaked into the trail: %s", delta)
	}

	// Gone from the download, and a second removal writes nothing.
	if _, err := uc.DownloadLink(ctx, maNVGoc, fileIDThu, staffActor(outsiderCode)); !errors.Is(err, ErrAttachmentNotFound) {
		t.Errorf("download after removal: err = %v, want ErrAttachmentNotFound", err)
	}
	before := writeCount(k)
	if err := uc.Remove(ctx, maNVGoc, fileIDThu, removalReason, staffActor(maLanhDao), false); !errors.Is(err, ErrAttachmentNotFound) {
		t.Errorf("second removal: err = %v, want ErrAttachmentNotFound", err)
	}
	if after := writeCount(k); after != before {
		t.Errorf("a second removal wrote %d statements", after-before)
	}
	if k.daCommit != 1 {
		t.Errorf("%d commits, want 1 — the repeat must not commit", k.daCommit)
	}
}

// writeCount counts the recorded WRITES — statements that start with INSERT or UPDATE, the test
// khongGhiGi applies. `SELECT … FOR UPDATE` is a read and is not one.
func writeCount(k *khoNhiemVuGia) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	n := 0
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "INSERT") || strings.HasPrefix(l.sql, "UPDATE") {
			n++
		}
	}
	return n
}

func TestRemove_TaskUpdateHolderMayRemoveAnotherOfficersAttachedFile(t *testing.T) {
	k := khoNVMau()
	uc, _, _, _, ctx := dungTaskAttachments(t, k)
	seedStoredAttached(k, maNguoiThucHien, true, nil)

	if err := uc.Remove(ctx, maNVGoc, fileIDThu, removalReason, staffActor(outsiderCode), true); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := k.storedFile(fileIDThu)["deleted_by"]; got != outsiderCode {
		t.Errorf("deleted_by = %v", got)
	}
	if delta := auditDeltaText(t, k); !strings.Contains(delta, `"quyen_go":"cap-nhat-nhiem-vu"`) {
		t.Errorf("delta does not name the task.update door: %s", delta)
	}
	chiGhiTrongGiaoDich(t, k)
}

func TestRemove_UploaderMayRemoveOwnDraft(t *testing.T) {
	k := khoNVMau()
	uc, _, _, _, ctx := dungTaskAttachments(t, k)
	seedStoredAttached(k, maNguoiThucHien, false, nil)

	if err := uc.Remove(ctx, maNVGoc, fileIDThu, removalReason, staffActor(maNguoiThucHien), false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if delta := auditDeltaText(t, k); !strings.Contains(delta, `"gan_nhat_ky":""`) {
		t.Errorf("delta: %s", delta)
	}
}

func TestRemove_RefusalsWriteNothing(t *testing.T) {
	other := "01JB" + strings.Repeat("B", 22)
	for _, c := range []struct {
		name   string
		mod    map[string]any
		linked bool
		actor  string
		update bool
		ma     string
		reason string
		want   error
	}{
		{"đọc được nhưng không phải người tải, không có task.update", nil, true, outsiderCode, false, maNVGoc,
			removalReason, domain.ErrAttachmentRemovalNotAllowed},
		{"bản nháp của người khác, không có task.update", nil, false, outsiderCode, false, maNVGoc,
			removalReason, ErrAttachmentNotFound},
		{"bản nháp của người khác, có task.update", nil, false, outsiderCode, true, maNVGoc,
			removalReason, ErrAttachmentNotFound},
		{"đang giữ phục vụ khiếu nại/thanh tra", map[string]any{"legal_hold": true}, true, maNguoiThucHien, true,
			maNVGoc, removalReason, domain.ErrAttachmentUnderLegalHold},
		{"xã khác", map[string]any{"tenant_id": other}, true, maNguoiThucHien, true, maNVGoc,
			removalReason, ErrAttachmentNotFound},
		{"qua đường của nhiệm vụ khác", nil, true, maNguoiThucHien, true, maNVCon,
			removalReason, ErrAttachmentNotFound},
		{"mã tệp không có", map[string]any{"id": "01JKHONGCO0000000000000000"}, false, maNguoiThucHien, true,
			maNVGoc, removalReason, ErrAttachmentNotFound},
		{"chưa tải xong", map[string]any{"status": "pending", "mime_type": nil, "size_bytes": nil, "sha256": nil},
			false, maNguoiThucHien, true, maNVGoc, removalReason, ErrAttachmentNotFound},
		{"đã gỡ", map[string]any{"deleted_at": mocTaoNV}, true, maNguoiThucHien, true, maNVGoc,
			removalReason, ErrAttachmentNotFound},
		{"nhiệm vụ không có", nil, true, maNguoiThucHien, true, "NV999", removalReason, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			k.themCon(idNVCon, maNVCon, idNVGoc, domain.DangThucHien)
			uc, _, _, _, ctx := dungTaskAttachments(t, k)
			r := storedFileRow(string(xaThu), fileIDThu, idNVGoc, maNguoiThucHien, domain.StoredFileStored, mocTaoNV)
			for col, v := range c.mod {
				r[col] = v
			}
			k.addStoredFile(r)
			if c.linked {
				k.fileLinks = map[string]string{fileIDThu: "nk-1"}
			}

			err := uc.Remove(ctx, c.ma, fileIDThu, c.reason, staffActor(c.actor), TaskUpdateRight(c.update))
			if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			khongGhiGi(t, k)
		})
	}
}

func TestRemove_BlankReasonRefusedBeforeAnyTransaction(t *testing.T) {
	for _, reason := range []string{"", "   \n\t"} {
		k := khoNVMau()
		uc, _, _, _, ctx := dungTaskAttachments(t, k)
		seedStoredAttached(k, maNguoiThucHien, true, nil)
		err := uc.Remove(ctx, maNVGoc, fileIDThu, reason, staffActor(maNguoiThucHien), false)
		if !errors.Is(err, domain.ErrAttachmentRemovalReasonMissing) {
			t.Fatalf("reason %q: err = %v", reason, err)
		}
		if k.batDau != 0 {
			t.Errorf("reason %q: %d transactions opened", reason, k.batDau)
		}
		khongGhiGi(t, k)
	}
	k := khoNVMau()
	uc, _, _, _, ctx := dungTaskAttachments(t, k)
	long := strings.Repeat("x", domain.MaxAttachmentRemovalReasonRunes+1)
	if err := uc.Remove(ctx, maNVGoc, fileIDThu, long, staffActor(maNguoiThucHien), false); !errors.Is(err,
		domain.ErrAttachmentRemovalReasonTooLong) {
		t.Errorf("long reason: err = %v", err)
	}
}

// Rule 6, invariant 8: no principal code, no write — and no fallback.
func TestRemove_NoStaffCodeRefused(t *testing.T) {
	k := khoNVMau()
	uc, _, _, _, ctx := dungTaskAttachments(t, k)
	seedStoredAttached(k, maNguoiThucHien, true, nil)
	if err := uc.Remove(ctx, maNVGoc, fileIDThu, removalReason, staffActor(""), true); err == nil {
		t.Fatal("removal with no staff code was accepted")
	}
	khongGhiGi(t, k)
}
