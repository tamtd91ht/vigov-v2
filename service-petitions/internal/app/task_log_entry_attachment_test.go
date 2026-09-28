package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// A manual log entry carrying `attachments` (task_log_entry.go, migration 0021).
//
//	PROVED HERE   the files are read under lock and checked BEFORE the entry is written · the link is
//	              ONE statement, AFTER the entry, in the SAME transaction as the entry and its audit
//	              entry · the trail names the file ids, never the file names · the reply carries the
//	              files · another task's, another officer's, an unfinished, an already-attached, an
//	              unknown and another commune's file are all refused with ONE sentence and nothing is
//	              written · an empty or duplicate id never opens a transaction.
//	NOT PROVED    migration 0021's link trigger — stored_file_pg_test.go (SKIPS without VIGOV_TEST_DSN).

const (
	fileA = "01JTEPA0000000000000000001"
	fileB = "01JTEPB0000000000000000002"
)

func TestAddLogEntry_WithAttachmentsLinksInTheEntrysTransaction(t *testing.T) {
	k := khoNVMau()
	k.addStoredFile(storedFileRow(string(xaThu), fileA, idNVGoc, maNguoiThucHien, domain.StoredFileStored, mocTaoNV))
	k.addStoredFile(storedFileRow(string(xaThu), fileB, idNVGoc, maNguoiThucHien, domain.StoredFileReady, mocTaoNV))
	uc, ctx := dungGhiNhiemVu(t, k)

	row, files, err := uc.AddLogEntry(ctx, maNVGoc, logText, []string{fileA, fileB},
		staffActor(maNguoiThucHien), false)
	if err != nil {
		t.Fatalf("ghi nhật ký kèm tệp: %v", err)
	}
	if len(files) != 2 || files[0].FileID != fileA || files[1].FileID != fileB ||
		files[0].LogEntryID != row.ID || files[0].OriginalName != "Biên bản nghiệm thu.pdf" {
		t.Errorf("tệp trả về = %+v", files)
	}

	// ORDER: candidates (locked) → entry → link → audit, one transaction.
	var order []string
	for _, l := range k.lenh {
		switch {
		case strings.Contains(l.sql, "LEFT JOIN task_log_attachment"):
			order = append(order, "doc-tep")
			if !strings.Contains(l.sql, "FOR UPDATE OF f") || l.args[0] != string(xaThu) {
				t.Errorf("tệp không được đọc có khoá, trong xã: %s %v", l.sql, l.args)
			}
		case strings.Contains(l.sql, "INSERT INTO nhat_ky_nhiem_vu"):
			order = append(order, "nhat-ky")
		case strings.Contains(l.sql, "INSERT INTO task_log_attachment"):
			order = append(order, "gan-tep")
			if l.args[0] != string(xaThu) || l.args[1] != row.ID || l.args[2] != fileA || l.args[3] != fileB {
				t.Errorf("câu gắn tệp: %v", l.args)
			}
		case strings.Contains(l.sql, "INSERT INTO audit_log"):
			order = append(order, "vet")
		}
	}
	if strings.Join(order, ",") != "doc-tep,nhat-ky,gan-tep,vet" {
		t.Errorf("thứ tự = %v", order)
	}
	chiGhiTrongGiaoDich(t, k)
	if k.fileLinks[fileA] != row.ID || k.fileLinks[fileB] != row.ID {
		t.Errorf("liên kết = %v", k.fileLinks)
	}

	delta := auditDeltaText(t, k)
	if !strings.Contains(delta, `"tep_dinh_kem":["`+fileA+`","`+fileB+`"]`) {
		t.Errorf("vết không nêu mã tệp: %s", delta)
	}
	if strings.Contains(delta, "Biên bản") {
		t.Errorf("tên tệp lọt vào vết: %s", delta)
	}
}

func TestAddLogEntry_UnusableAttachmentRefusedWritesNothing(t *testing.T) {
	otherCommune := "01JB" + strings.Repeat("B", 22)
	for _, c := range []struct {
		name string
		row  map[string]any
		link bool
		ids  []string
	}{
		{"tệp của nhiệm vụ khác", map[string]any{"subject_id": idNVCon}, false, []string{fileA}},
		{"tệp của người khác tải", map[string]any{"uploaded_by": "CB-00412"}, false, []string{fileA}},
		{"tệp chưa quét xong", map[string]any{"status": "pending"}, false, []string{fileA}},
		{"tệp bị từ chối", map[string]any{"status": "rejected"}, false, []string{fileA}},
		{"tệp đã gắn vào dòng khác", nil, true, []string{fileA}},
		{"tệp của xã khác", map[string]any{"tenant_id": otherCommune}, false, []string{fileA}},
		{"mã tệp không có", nil, false, []string{fileA, "01JKHONGCO0000000000000000"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			r := storedFileRow(string(xaThu), fileA, idNVGoc, maNguoiThucHien, domain.StoredFileStored, mocTaoNV)
			for col, v := range c.row {
				r[col] = v
			}
			k.addStoredFile(r)
			if c.link {
				k.fileLinks = map[string]string{fileA: "nk-cu"}
			}
			uc, ctx := dungGhiNhiemVu(t, k)

			_, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, c.ids, staffActor(maNguoiThucHien), true)
			if !errors.Is(err, domain.ErrAttachmentNotUsable) {
				t.Fatalf("lỗi = %v, muốn ErrAttachmentNotUsable", err)
			}
			khongGhiGi(t, k)
		})
	}
}

func TestAddLogEntry_BadAttachmentListNeverOpensATransaction(t *testing.T) {
	for _, ids := range [][]string{{fileA, fileA}, {""}} {
		k := khoNVMau()
		uc, ctx := dungGhiNhiemVu(t, k)
		_, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, ids, staffActor(maNguoiThucHien), true)
		if !errors.Is(err, domain.ErrAttachmentListInvalid) {
			t.Fatalf("%q: lỗi = %v, muốn ErrAttachmentListInvalid", ids, err)
		}
		if k.batDau != 0 {
			t.Errorf("%q: mở %d giao dịch", ids, k.batDau)
		}
	}
}

// Wired without the file store: an entry WITH files refuses (never drops them and writes the text
// alone); an entry without files is unaffected.
func TestAddLogEntry_AttachmentsWithoutFileStoreRefuse(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.files = nil
	if _, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, []string{fileA}, staffActor(maNguoiThucHien), true); err == nil {
		t.Fatal("ghi nhật ký kèm tệp khi chưa nối kho tệp mà không lỗi")
	}
	khongGhiGi(t, k)
	if _, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, nil, staffActor(maNguoiThucHien), true); err != nil {
		t.Errorf("nhật ký không kèm tệp: %v", err)
	}
}
