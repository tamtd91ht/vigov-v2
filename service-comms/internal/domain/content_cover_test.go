package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckCoverUsableAdmitsOnlyThisArticlesReadyCover(t *testing.T) {
	ok := &StoredFile{SubjectType: StoredFileSubjectContentItem, SubjectID: "item-1", Purpose: "content-image",
		Status: StoredFileReady}
	if err := CheckCoverUsable(ok, "item-1", "content-image"); err != nil {
		t.Fatalf("a ready cover of this article refused: %v", err)
	}
	for name, mutate := range map[string]func(f *StoredFile){
		"another article": func(f *StoredFile) { f.SubjectID = "item-2" },
		"another purpose": func(f *StoredFile) { f.Purpose = "content-video" },
		// `stored` passes 0011's trigger but has no derivative yet — it could never be published.
		"stored, no derivative": func(f *StoredFile) { f.Status = StoredFileStored },
		"pending":               func(f *StoredFile) { f.Status = StoredFilePending },
		"rejected":              func(f *StoredFile) { f.Status = StoredFileRejected },
		"another subject type":  func(f *StoredFile) { f.SubjectType = "task" },
	} {
		f := *ok
		mutate(&f)
		if err := CheckCoverUsable(&f, "item-1", "content-image"); !errors.Is(err, ErrCoverNotUsable) {
			t.Errorf("%s: err = %v, want ErrCoverNotUsable", name, err)
		}
	}
	if err := CheckCoverUsable(nil, "item-1", "content-image"); !errors.Is(err, ErrCoverNotUsable) {
		t.Errorf("missing row: err = %v", err)
	}
}

func TestCheckBodyImageUsableRefusesEveryOtherFileWithOneError(t *testing.T) {
	ok := &StoredFile{SubjectType: StoredFileSubjectContentItem, SubjectID: "item-1", Purpose: "content-body-image",
		Status: StoredFileReady}
	if err := CheckBodyImageUsable(ok, "item-1", "content-body-image"); err != nil {
		t.Fatalf("a ready body image of this article refused: %v", err)
	}
	for name, mutate := range map[string]func(f *StoredFile){
		"another article":     func(f *StoredFile) { f.SubjectID = "item-2" },
		"the cover's purpose": func(f *StoredFile) { f.Purpose = "content-image" },
		"not finished":        func(f *StoredFile) { f.Status = StoredFileProcessing },
		"rejected":            func(f *StoredFile) { f.Status = StoredFileRejected },
	} {
		f := *ok
		mutate(&f)
		if err := CheckBodyImageUsable(&f, "item-1", "content-body-image"); !errors.Is(err, ErrBodyImageNotUsable) {
			t.Errorf("%s: err = %v, want ErrBodyImageNotUsable", name, err)
		}
	}
	if err := CheckBodyImageUsable(nil, "item-1", "content-body-image"); !errors.Is(err, ErrBodyImageNotUsable) {
		t.Errorf("unknown / another commune's (no row): err = %v", err)
	}
}

func TestCoverFileIDShapeOnTheWire(t *testing.T) {
	if v, err := cleanFileID("  01JFFFFFFFFFFFFFFFFFFFFFFF "); err != nil || v != "01JFFFFFFFFFFFFFFFFFFFFFFF" {
		t.Errorf("valid id: %q %v", v, err)
	}
	if v, err := cleanFileID(""); err != nil || v != "" {
		t.Errorf("empty means none/detach: %q %v", v, err)
	}
	for _, bad := range []string{"../../etc", "id with space", strings.Repeat("A", MaxFileIDLen+1), "x;DROP"} {
		if _, err := cleanFileID(bad); !errors.Is(err, ErrCoverFileIDInvalid) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestEditRequestCarriesDetachAsEmptyPointer(t *testing.T) {
	empty := ""
	got, err := YeuCauSuaNoiDung{CoverImageFileID: &empty}.KiemTra()
	if err != nil || got.CoverImageFileID == nil || *got.CoverImageFileID != "" {
		t.Fatalf("detach lost: %v %v", got.CoverImageFileID, err)
	}
	got, err = YeuCauSuaNoiDung{}.KiemTra()
	if err != nil || got.CoverImageFileID != nil {
		t.Fatalf("absent must stay nil: %v %v", got.CoverImageFileID, err)
	}
}

func TestCleanCoverFileNameKeepsTheLastElementOnly(t *testing.T) {
	if v, err := CleanCoverFileName(`C:\fakepath\trao-qua.jpg`); err != nil || v != "trao-qua.jpg" {
		t.Errorf("got %q %v", v, err)
	}
	for _, bad := range []string{"", "   ", "a\x00b.jpg", strings.Repeat("ả", MaxCoverFileNameRunes+1)} {
		if _, err := CleanCoverFileName(bad); !errors.Is(err, ErrCoverFileNameInvalid) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestStatusEdgesMatchStoredFileGuard(t *testing.T) {
	for _, e := range [][2]StoredFileStatus{
		{StoredFilePending, StoredFileScanning}, {StoredFileScanning, StoredFileStored},
		{StoredFileStored, StoredFileProcessing}, {StoredFileProcessing, StoredFileReady},
		{StoredFileProcessing, StoredFileFailed}, {StoredFilePending, StoredFileRejected},
	} {
		if !e[0].CanMoveTo(e[1]) {
			t.Errorf("%s -> %s must be an edge", e[0], e[1])
		}
	}
	for _, e := range [][2]StoredFileStatus{
		{StoredFilePending, StoredFileReady}, {StoredFileReady, StoredFilePending}, {StoredFileRejected, StoredFileReady},
	} {
		if e[0].CanMoveTo(e[1]) {
			t.Errorf("%s -> %s must not be an edge", e[0], e[1])
		}
	}
}
