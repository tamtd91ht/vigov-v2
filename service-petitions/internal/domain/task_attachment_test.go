package domain

import (
	"errors"
	"strings"
	"testing"
)

// Tests for the attachment rules of §5.9 (stored_file.go). The trigger in migration 0021 is the
// floor; these are what the write path refuses with a sentence first.

func TestCleanAttachmentName(t *testing.T) {
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"bien-ban.pdf", "bien-ban.pdf", true},
		{"  Biên bản nghiệm thu.pdf  ", "Biên bản nghiệm thu.pdf", true},
		// A browser's fake path: only the last element is a name.
		{`C:\fakepath\anh-hien-truong.jpg`, "anh-hien-truong.jpg", true},
		{"thu-muc/con/tep.png", "tep.png", true},
		{"", "", false},
		{"   ", "", false},
		{`C:\fakepath\`, "", false},
		{"tep\x00.pdf", "", false},
		{"tep\n.pdf", "", false},
		{"\xff\xfe.pdf", "", false},
		{strings.Repeat("ă", MaxOriginalNameRunes), strings.Repeat("ă", MaxOriginalNameRunes), true},
		{strings.Repeat("ă", MaxOriginalNameRunes+1), "", false},
	} {
		got, err := CleanAttachmentName(c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Errorf("CleanAttachmentName(%q) = %q, %v; muốn %q", c.in, got, err, c.want)
			}
			continue
		}
		if !errors.Is(err, ErrAttachmentNameInvalid) {
			t.Errorf("CleanAttachmentName(%q) lỗi = %v, muốn ErrAttachmentNameInvalid", c.in, err)
		}
	}
}

func TestCheckAttachmentList(t *testing.T) {
	if err := CheckAttachmentList(nil); err != nil {
		t.Errorf("danh sách rỗng: %v", err)
	}
	if err := CheckAttachmentList([]string{"a", "b"}); err != nil {
		t.Errorf("hai tệp khác nhau: %v", err)
	}
	for _, bad := range [][]string{{"a", "a"}, {""}, {"a", "  "}} {
		if err := CheckAttachmentList(bad); !errors.Is(err, ErrAttachmentListInvalid) {
			t.Errorf("%q: lỗi = %v, muốn ErrAttachmentListInvalid", bad, err)
		}
	}
}

func attachableFile() StoredFile {
	return StoredFile{ID: "f1", SubjectType: StoredFileSubjectTask, SubjectID: "nv-1",
		UploadedBy: "CB-00311", Status: StoredFileStored}
}

func TestCheckAttachable(t *testing.T) {
	ok := AttachCandidate{File: attachableFile()}
	if err := CheckAttachable(ok, "nv-1", "CB-00311"); err != nil {
		t.Fatalf("tệp hợp lệ bị từ chối: %v", err)
	}
	ready := ok
	ready.File.Status = StoredFileReady
	if err := CheckAttachable(ready, "nv-1", "CB-00311"); err != nil {
		t.Errorf("tệp ready bị từ chối: %v", err)
	}

	for name, c := range map[string]struct {
		cand   AttachCandidate
		task   string
		author string
	}{
		"nhiệm vụ khác":      {ok, "nv-2", "CB-00311"},
		"người tải khác":     {ok, "nv-1", "CB-00412"},
		"tác giả rỗng":       {ok, "nv-1", ""},
		"đã gắn vào dòng":    {AttachCandidate{File: ok.File, LinkedTo: "nk-9"}, "nv-1", "CB-00311"},
		"chưa tải xong":      {withStatus(ok, StoredFilePending), "nv-1", "CB-00311"},
		"đã bị từ chối":      {withStatus(ok, StoredFileRejected), "nv-1", "CB-00311"},
		"đã huỷ theo hạn":    {withStatus(ok, StoredFilePurged), "nv-1", "CB-00311"},
		"chủ thể không phải": {withSubject(ok, "petition"), "nv-1", "CB-00311"},
	} {
		if err := CheckAttachable(c.cand, c.task, c.author); !errors.Is(err, ErrAttachmentNotUsable) {
			t.Errorf("%s: lỗi = %v, muốn ErrAttachmentNotUsable", name, err)
		}
	}
}

func withStatus(c AttachCandidate, s StoredFileStatus) AttachCandidate {
	c.File.Status = s
	return c
}

func withSubject(c AttachCandidate, s string) AttachCandidate {
	c.File.SubjectType = s
	return c
}

func TestMayDownload(t *testing.T) {
	f := attachableFile()
	if !MayDownload(f, "nv-1", "nk-1", "CB-09999") {
		t.Error("tệp đã gắn vào nhật ký phải tải được với mọi người đọc nhiệm vụ")
	}
	if !MayDownload(f, "nv-1", "", "CB-00311") {
		t.Error("người tải lên phải tải lại được tệp chưa gắn")
	}
	if MayDownload(f, "nv-1", "", "CB-09999") {
		t.Error("tệp chưa gắn vào nhật ký lộ cho người không tải lên")
	}
	if MayDownload(f, "nv-1", "", "") {
		t.Error("người đọc rỗng khớp người tải lên")
	}
	if MayDownload(f, "nv-2", "nk-1", "CB-00311") {
		t.Error("tệp của nhiệm vụ khác tải được qua đường của nhiệm vụ này")
	}
	pending := f
	pending.Status = StoredFilePending
	if MayDownload(pending, "nv-1", "", "CB-00311") {
		t.Error("tệp chưa quét tải được")
	}
}
