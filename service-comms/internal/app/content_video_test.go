package app

// Migration 0028 through the content use case: the typed video length is written, audited by value,
// cleared with the type, and refused on another type before anything is written.

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-comms/internal/domain"
)

func TestEditSetsVideoDurationAndAuditsIt(t *testing.T) {
	row := draftRow()
	row.Loai, row.VideoURL = domain.LoaiVideo, "https://video.example/v/1"
	k := &khoNDGia{dongHienCo: row}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	got, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{VideoDurationSeconds: ptr(312)}, nguoiSoanND())
	if err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if got.VideoDurationSeconds != 312 || updateArgs(t, k)[20] != int64(312) {
		t.Errorf("returned %d, $21 bound %v", got.VideoDurationSeconds, updateArgs(t, k)[20])
	}
	if d := auditDelta(t, k); !strings.Contains(d, `"video_duration_seconds":312`) {
		t.Errorf("the audit delta must carry the typed length: %s", d)
	}
}

func TestTypeChangeAwayFromVideoClearsTheDuration(t *testing.T) {
	row := draftRow()
	row.Loai, row.VideoURL, row.VideoDurationSeconds = domain.LoaiVideo, "https://video.example/v/1", 312
	k := &khoNDGia{dongHienCo: row}
	uc, _, ctx := dungUseCaseNoiDung(t, k)

	news := string(domain.LoaiTinTuc)
	if _, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{Loai: &news}, nguoiSoanND()); err != nil {
		t.Fatalf("sửa lỗi: %v", err)
	}
	if updateArgs(t, k)[20] != nil {
		t.Errorf("type change away from video kept video_duration_seconds = %v", updateArgs(t, k)[20])
	}
}

func TestVideoDurationOnNewsIsRefusedBeforeAnyWrite(t *testing.T) {
	k := &khoNDGia{dongHienCo: draftRow()}
	uc, _, ctx := dungUseCaseNoiDung(t, k)
	_, err := uc.Sua(ctx, "nd-001", domain.YeuCauSuaNoiDung{VideoDurationSeconds: ptr(60)}, nguoiSoanND())
	if !errors.Is(err, domain.ErrVideoDurationOnlyForVideo) {
		t.Fatalf("err = %v", err)
	}
	if k.coCau("UPDATE noi_dung_mini_app") || k.coCau("INSERT INTO audit_log") {
		t.Error("refused, yet something was written")
	}
}
