package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The optional scene location on the citizen intake (ADR 0050).
//
//	PROVED HERE   sent coordinates reach the row, rounded to the column's precision · not sent
//	              leaves both nil · one alone is refused BEFORE identity is asked and before any SQL
//	              · the audit delta says only `has_scene_location` true/false and holds no digit of
//	              either coordinate (rule 6, forbidden #4).
//	NOT PROVED    the INSERT's placement of the two values — internal/store's Tao test owns that.

func sceneCoord(v float64) *float64 { return &v }

func TestIntakeStoresSceneLocation(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	yc := ycThu()
	yc.Lat, yc.Lng = sceneCoord(16.05440049), sceneCoord(108.2022)
	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), yc, congDanThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}
	row := kho.thay[0]
	if row.Lat == nil || *row.Lat != 16.0544 || row.Lng == nil || *row.Lng != 108.2022 {
		t.Errorf("toạ độ trên dòng = %v/%v, muốn 16.0544/108.2022 (làm tròn 6 chữ số)", row.Lat, row.Lng)
	}
}

func TestIntakeWithoutSceneLocationLeavesBothNil(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycThu(), congDanThu()); err != nil {
		t.Fatalf("Gui: %v", err)
	}
	if row := kho.thay[0]; row.Lat != nil || row.Lng != nil {
		t.Errorf("toạ độ = %v/%v dù công dân không gửi — một ghim bịa ra trên bản đồ", row.Lat, row.Lng)
	}
}

func TestIntakeRefusesLoneCoordinateBeforeAnything(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	yc := ycThu()
	yc.Lat = sceneCoord(16.0544)
	_, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), yc, congDanThu())

	if !errors.Is(err, domain.ErrSceneLocationIncomplete) {
		t.Fatalf("lỗi = %v, muốn ErrSceneLocationIncomplete", err)
	}
	if han.goi != 0 || len(k.lenh) != 0 {
		t.Errorf("đã hỏi identity %d lần và chạy %d câu lệnh cho một yêu cầu sai hình dạng",
			han.goi, len(k.lenh))
	}
}

func TestIntakeDeltaFlagsSceneLocationWithoutCoordinates(t *testing.T) {
	for name, c := range map[string]struct {
		lat, lng *float64
		want     bool
	}{
		"có vị trí":    {sceneCoord(16.0544), sceneCoord(108.2022), true},
		"không vị trí": {nil, nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
			yc := ycThu()
			yc.Lat, yc.Lng = c.lat, c.lng
			if _, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), yc, congDanThu()); err != nil {
				t.Fatalf("Gui: %v", err)
			}
			raw, ok := k.lenh[1].args[7].([]byte)
			if !ok {
				t.Fatalf("delta = %T, muốn []byte", k.lenh[1].args[7])
			}
			for _, leak := range []string{"16.05", "108.20", "16054", "108202", `"lat"`, `"lng"`} {
				if strings.Contains(string(raw), leak) {
					t.Errorf("delta mang toạ độ (%q) — sổ vết append-only không được là kho vị trí: %s", leak, raw)
				}
			}
			var d map[string]any
			if err := json.Unmarshal(raw, &d); err != nil {
				t.Fatalf("delta không phải JSON: %q", raw)
			}
			if got, ok := d["has_scene_location"].(bool); !ok || got != c.want {
				t.Errorf("has_scene_location = %v, muốn %v: %s", d["has_scene_location"], c.want, raw)
			}
		})
	}
}
