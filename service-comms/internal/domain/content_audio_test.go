package domain

import (
	"errors"
	"testing"
)

func TestCheckAudioDurationBounds(t *testing.T) {
	for _, ok := range []int{1, 60, 21600} {
		if err := CheckAudioDuration(ok); err != nil {
			t.Errorf("%d refused: %v", ok, err)
		}
	}
	for _, bad := range []int{0, -1, 21601} {
		if err := CheckAudioDuration(bad); !errors.Is(err, ErrAudioDurationInvalid) {
			t.Errorf("%d: err = %v", bad, err)
		}
	}
}

// Migration 0012's three audio CHECKs, as CheckTypeFields asks them of the merged row.
func TestCheckTypeFieldsAudio(t *testing.T) {
	for _, tc := range []struct {
		name string
		n    NoiDungMiniApp
		want error
	}{
		{"broadcast with both", NoiDungMiniApp{Loai: LoaiTruyenThanh, AudioFileID: "F", AudioDurationSeconds: 60}, nil},
		{"broadcast with neither", NoiDungMiniApp{Loai: LoaiTruyenThanh}, nil},
		{"file without duration", NoiDungMiniApp{Loai: LoaiTruyenThanh, AudioFileID: "F"}, ErrAudioAllOrNone},
		{"duration without file", NoiDungMiniApp{Loai: LoaiTruyenThanh, AudioDurationSeconds: 60}, ErrAudioAllOrNone},
		{"audio on news", NoiDungMiniApp{Loai: LoaiTinTuc, AudioFileID: "F", AudioDurationSeconds: 60}, ErrAudioOnlyForBroadcast},
		{"duration out of range", NoiDungMiniApp{Loai: LoaiTruyenThanh, AudioFileID: "F", AudioDurationSeconds: 21601},
			ErrAudioDurationInvalid},
	} {
		if err := tc.n.CheckTypeFields(); !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestWithoutOtherTypeFieldsClearsAudioOffBroadcasts(t *testing.T) {
	n := NoiDungMiniApp{Loai: LoaiTinTuc, AudioFileID: "F", AudioDurationSeconds: 60}.WithoutOtherTypeFields()
	if n.AudioFileID != "" || n.AudioDurationSeconds != 0 {
		t.Errorf("audio kept on a news item: %+v", n)
	}
	b := NoiDungMiniApp{Loai: LoaiTruyenThanh, AudioFileID: "F", AudioDurationSeconds: 60}.WithoutOtherTypeFields()
	if b.AudioFileID != "F" || b.AudioDurationSeconds != 60 {
		t.Errorf("a broadcast lost its audio: %+v", b)
	}
}

func TestEditRequestAudioValidation(t *testing.T) {
	empty, bad, sixty, tooLong := "", "not/a/ulid", 60, 21601
	if _, err := (YeuCauSuaNoiDung{AudioFileID: &bad}).KiemTra(); !errors.Is(err, ErrAudioFileIDInvalid) {
		t.Errorf("bad id: %v", err)
	}
	if _, err := (YeuCauSuaNoiDung{AudioDurationSeconds: &tooLong}).KiemTra(); !errors.Is(err, ErrAudioDurationInvalid) {
		t.Errorf("too long: %v", err)
	}
	if _, err := (YeuCauSuaNoiDung{AudioFileID: &empty, AudioDurationSeconds: &sixty}).KiemTra(); !errors.Is(err, ErrAudioAllOrNone) {
		t.Errorf("remove + duration: %v", err)
	}
	got, err := (YeuCauSuaNoiDung{AudioFileID: &empty}).KiemTra()
	if err != nil || got.AudioFileID == nil || *got.AudioFileID != "" || got.SetsAudioField() {
		t.Errorf("removal: %+v, %v", got, err)
	}
	got, err = (YeuCauSuaNoiDung{AudioDurationSeconds: &sixty}).KiemTra()
	if err != nil || got.AudioDurationSeconds == nil || *got.AudioDurationSeconds != 60 || !got.SetsAudioField() {
		t.Errorf("duration: %+v, %v", got, err)
	}
}

func TestCheckAudioUsable(t *testing.T) {
	ok := &StoredFile{SubjectType: StoredFileSubjectContentItem, SubjectID: "I", Purpose: "content-audio",
		Status: StoredFileReady}
	if err := CheckAudioUsable(ok, "I", "content-audio"); err != nil {
		t.Fatalf("ready file refused: %v", err)
	}
	for name, mut := range map[string]func(*StoredFile){
		"another item":    func(f *StoredFile) { f.SubjectID = "J" },
		"a cover":         func(f *StoredFile) { f.Purpose = "content-image" },
		"not ready":       func(f *StoredFile) { f.Status = StoredFileStored },
		"another subject": func(f *StoredFile) { f.SubjectType = "task" },
	} {
		f := *ok
		mut(&f)
		if err := CheckAudioUsable(&f, "I", "content-audio"); !errors.Is(err, ErrAudioNotUsable) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := CheckAudioUsable(nil, "I", "content-audio"); !errors.Is(err, ErrAudioNotUsable) {
		t.Errorf("nil: %v", err)
	}
}
