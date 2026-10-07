package domain

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func removableFile(uploader string) StoredFile {
	return StoredFile{ID: "f1", SubjectType: StoredFileSubjectTask, SubjectID: "nv-1", UploadedBy: uploader,
		Status: StoredFileStored, OriginalName: "Biên bản.pdf"}
}

// The user decision of 07/10/2026: the uploader OR a `task.update` holder, attached or not — on a file
// the caller can see. Everything invisible is None (404), never Refused (403).
func TestAttachmentRemovalRightFor(t *testing.T) {
	for _, c := range []struct {
		name   string
		f      StoredFile
		task   string
		linked string
		actor  string
		update bool
		want   AttachmentRemovalRight
	}{
		{"uploader, attached", removableFile("CB-1"), "nv-1", "nk-1", "CB-1", false, AttachmentRemovalUploader},
		{"uploader, draft", removableFile("CB-1"), "nv-1", "", "CB-1", false, AttachmentRemovalUploader},
		{"uploader holding task.update is recorded as uploader", removableFile("CB-1"), "nv-1", "nk-1", "CB-1", true,
			AttachmentRemovalUploader},
		{"task.update, attached", removableFile("CB-1"), "nv-1", "nk-1", "CB-2", true, AttachmentRemovalUpdater},
		{"reader without rights, attached", removableFile("CB-1"), "nv-1", "nk-1", "CB-2", false, AttachmentRemovalRefused},
		{"somebody else's draft, task.update", removableFile("CB-1"), "nv-1", "", "CB-2", true, AttachmentRemovalNone},
		{"another task", removableFile("CB-1"), "nv-2", "nk-1", "CB-1", true, AttachmentRemovalNone},
		{"no actor", removableFile("CB-1"), "nv-1", "nk-1", "", true, AttachmentRemovalNone},
		{"not stored yet", func() StoredFile { f := removableFile("CB-1"); f.Status = StoredFilePending; return f }(),
			"nv-1", "", "CB-1", true, AttachmentRemovalNone},
	} {
		if got := AttachmentRemovalRightFor(c.f, c.task, c.linked, c.actor, c.update); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCheckAttachmentRemovalReason(t *testing.T) {
	if got, err := CheckAttachmentRemovalReason("  tải nhầm  "); err != nil || got != "tải nhầm" {
		t.Errorf("trim: %q, %v", got, err)
	}
	if _, err := CheckAttachmentRemovalReason(" \t "); !errors.Is(err, ErrAttachmentRemovalReasonMissing) {
		t.Errorf("blank: %v", err)
	}
	if _, err := CheckAttachmentRemovalReason(strings.Repeat("ạ", MaxAttachmentRemovalReasonRunes)); err != nil {
		t.Errorf("at the limit (counted in runes, not bytes): %v", err)
	}
	if _, err := CheckAttachmentRemovalReason(strings.Repeat("a", MaxAttachmentRemovalReasonRunes+1)); !errors.Is(err,
		ErrAttachmentRemovalReasonTooLong) {
		t.Errorf("over: %v", err)
	}
}

// The longest reason still fits one timeline entry, so a valid removal is never refused by the log's
// own bound.
func TestAttachmentRemovalLogTextFitsTheLog(t *testing.T) {
	text := AttachmentRemovalLogText(strings.Repeat("r", MaxAttachmentRemovalReasonRunes))
	if n := utf8.RuneCountInString(text); n > NoiDungNhatKyToiDa {
		t.Errorf("log line is %d runes, the log takes %d", n, NoiDungNhatKyToiDa)
	}
	if _, err := KiemNoiDungNhatKy(text); err != nil {
		t.Errorf("KiemNoiDungNhatKy: %v", err)
	}
}
