package store

// Migration 0028 in SQL, over the fake driver of noi_dung_mini_app_test.go: the typed video length is
// bound in position on INSERT and UPDATE (0 → NULL) and read back by name. What PostgreSQL decides (the
// two CHECKs) is the pg suite's.

import (
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

func TestVideoDurationBindsOnInsert(t *testing.T) {
	for _, c := range []struct {
		seconds int
		want    any
	}{{754, int64(754)}, {0, nil}} {
		k := &khoNDGia{}
		repo, ctx := khoNoiDung(t, k)
		err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
			return repo.Chen(ctx, tx, domain.NoiDungMiniApp{
				ID: "nd-v", Loai: domain.LoaiVideo, TieuDe: "Video", NgayDang: ngayMau,
				TrangThai: domain.TrangThaiAn, NguoiTaoMa: "CB-1", VideoDurationSeconds: c.seconds,
			})
		})
		if err != nil {
			t.Fatalf("chèn lỗi: %v", err)
		}
		if !strings.Contains(k.lenh[0].sql, "video_duration_seconds)") {
			t.Errorf("INSERT does not name video_duration_seconds: %s", k.lenh[0].sql)
		}
		if args := k.lenh[0].args; len(args) != 20 || args[19] != c.want {
			t.Errorf("seconds %d: $20 bound as %v (len %d), want %v", c.seconds, args[len(args)-1], len(args), c.want)
		}
	}
}

func TestVideoDurationBindsOnUpdate(t *testing.T) {
	k := &khoNDGia{soDongDoi: 1}
	repo, ctx := khoNoiDung(t, k)
	err := chayTrongGiaoDich(t, k, ctx, func(tx *pkgstore.ScopedTx) error {
		return repo.CapNhat(ctx, tx, domain.NoiDungMiniApp{ID: "nd-001", Loai: domain.LoaiVideo, TieuDe: "T",
			VideoDurationSeconds: 95})
	})
	if err != nil {
		t.Fatalf("cập nhật lỗi: %v", err)
	}
	if !strings.Contains(k.lenh[0].sql, "video_duration_seconds = $21") {
		t.Errorf("UPDATE does not write video_duration_seconds = $21: %s", k.lenh[0].sql)
	}
	if args := k.lenh[0].args; len(args) != 21 || args[20] != int64(95) {
		t.Errorf("$21 bound as %v", args[20])
	}
}

func TestVideoDurationReadsByName(t *testing.T) {
	row := dongNDMau()
	row.loai, row.videoDuration = "video", int64(312)
	k := &khoNDGia{dong: []dongNDMiniApp{row}}
	repo, ctx := khoNoiDung(t, k)
	result, err := repo.DanhSach(ctx, LocNoiDung{}, trangDauND(t))
	if err != nil {
		t.Fatalf("đọc sổ lỗi: %v", err)
	}
	// 312 must land in the VIDEO length and not in the adjacent audio one.
	if got := result.Items[0]; got.VideoDurationSeconds != 312 || got.AudioDurationSeconds != 0 {
		t.Errorf("video length read as %d, audio as %d", got.VideoDurationSeconds, got.AudioDurationSeconds)
	}
}
