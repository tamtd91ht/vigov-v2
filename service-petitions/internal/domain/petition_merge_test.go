package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// The pure half of ADR 0087: which merges are allowed, which deadline the main petition takes, which
// merged petitions follow their main, and the duplicate search's geometry.

var (
	mergeOrigin   = time.Date(2026, 10, 1, 2, 13, 0, 0, time.UTC)
	mergeEarlier  = time.Date(2026, 10, 3, 9, 41, 0, 0, time.UTC)
	mergeLater    = time.Date(2026, 10, 6, 4, 7, 0, 0, time.UTC)
	mergeTooEarly = time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC) // before mergeOrigin
)

func mergePair() (PhieuPhanAnh, PhieuPhanAnh) {
	child := PhieuPhanAnh{ID: "c", MaTraCuu: "PA-CHILD", TrangThai: DaTiepNhan, GocDemHan: mergeOrigin}
	main := PhieuPhanAnh{ID: "m", MaTraCuu: "PA-MAIN", TrangThai: DangXuLy, GocDemHan: mergeOrigin,
		HanXuLyXong: mergeLater, LinhVuc: "rac-thai"}
	return child, main
}

func TestMergeOpenIsTheOwnersFourStatuses(t *testing.T) {
	want := map[TrangThai]bool{DaTiepNhan: true, DangPhanLoai: true, DaChuyenXuLy: true, DangXuLy: true}
	for _, s := range []TrangThai{DaTiepNhan, DangPhanLoai, DaChuyenXuLy, DangXuLy, DaXuLy, ChoDanXacNhan,
		DaDong, KhongTiepNhan, ChuyenCapTren, "khong-co"} {
		if MergeOpen(s) != want[s] {
			t.Errorf("MergeOpen(%q) = %v", s, MergeOpen(s))
		}
	}
}

func TestCheckMergeRefusals(t *testing.T) {
	for name, c := range map[string]struct {
		edit        func(child, main *PhieuPhanAnh)
		hasChildren bool
		want        error
	}{
		"self":                       {func(c, m *PhieuPhanAnh) { m.ID = c.ID }, false, ErrMergeSelf},
		"child can-bo":               {func(c, m *PhieuPhanAnh) { c.LinhVuc = LinhVucHanChe }, false, ErrMergeStaffConduct},
		"main can-bo":                {func(c, m *PhieuPhanAnh) { m.LinhVuc = LinhVucHanChe }, false, ErrMergeStaffConduct},
		"child already merged":       {func(c, m *PhieuPhanAnh) { c.MergedInto = "x" }, false, ErrMergeAlreadyMerged},
		"main is merged (chain)":     {func(c, m *PhieuPhanAnh) { m.MergedInto = "x" }, false, ErrMergeTargetIsMerged},
		"child has children (chain)": {func(c, m *PhieuPhanAnh) {}, true, ErrMergeHasChildren},
		"child resolved":             {func(c, m *PhieuPhanAnh) { c.TrangThai = DaXuLy }, false, ErrMergeNotOpen},
		"main awaiting citizen":      {func(c, m *PhieuPhanAnh) { m.TrangThai = ChoDanXacNhan }, false, ErrMergeNotOpen},
		"main closed":                {func(c, m *PhieuPhanAnh) { m.TrangThai = DaDong }, false, ErrMergeNotOpen},
		"child refused":              {func(c, m *PhieuPhanAnh) { c.TrangThai = KhongTiepNhan }, false, ErrMergeNotOpen},
	} {
		t.Run(name, func(t *testing.T) {
			child, main := mergePair()
			c.edit(&child, &main)
			if _, err := CheckMerge(child, main, c.hasChildren); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

// THE DEADLINE: the earlier of the two; NULL ("chưa có") takes the other's; never later.
func TestCheckMergeDeadline(t *testing.T) {
	for name, c := range map[string]struct {
		child, main, want time.Time
	}{
		"child earlier -> main takes it": {mergeEarlier, mergeLater, mergeEarlier},
		"main earlier -> unchanged":      {mergeLater, mergeEarlier, mergeEarlier},
		"child NULL -> main unchanged":   {time.Time{}, mergeLater, mergeLater},
		"main NULL -> takes child's":     {mergeEarlier, time.Time{}, mergeEarlier},
		"both NULL -> stays chưa có":     {time.Time{}, time.Time{}, time.Time{}},
		"equal -> unchanged":             {mergeLater, mergeLater, mergeLater},
	} {
		t.Run(name, func(t *testing.T) {
			child, main := mergePair()
			child.HanXuLyXong, main.HanXuLyXong = c.child, c.main
			got, err := CheckMerge(child, main, false)
			if err != nil || !got.Equal(c.want) {
				t.Errorf("deadline = %v, %v; want %v", got, err, c.want)
			}
			if !main.HanXuLyXong.IsZero() && got.After(main.HanXuLyXong) {
				t.Error("the main petition's deadline moved LATER")
			}
		})
	}
}

// Migration 0004's `han_xu_ly_xong >= goc_dem_han`: inheriting a deadline earlier than the MAIN's own
// origin is refused, never written (and never clamped).
func TestCheckMergeDeadlineBeforeMainOriginRefused(t *testing.T) {
	child, main := mergePair()
	child.GocDemHan = mergeTooEarly.Add(-48 * time.Hour)
	child.HanXuLyXong = mergeTooEarly
	if _, err := CheckMerge(child, main, false); !errors.Is(err, ErrMergeDeadlineBeforeOrigin) {
		t.Errorf("err = %v", err)
	}
	// Swapped roles — the earlier-reported petition as the main one — passes.
	child2, main2 := main, child
	child2.ID, main2.ID = "c", "m"
	if got, err := CheckMerge(child2, main2, false); err != nil || !got.Equal(mergeTooEarly) {
		t.Errorf("earlier-reported main: %v, %v", got, err)
	}
}

func TestCheckUnmerge(t *testing.T) {
	child, _ := mergePair()
	if _, err := CheckUnmerge(child); !errors.Is(err, ErrNotMerged) {
		t.Errorf("not merged: %v", err)
	}
	child.MergedInto = "m"
	// Owner, 09/10/2026 (c): any time before the petition is closed. Only `cho-dan-xac-nhan` moves —
	// back to `dang-xu-ly`, along the lifecycle's existing edge.
	for from, want := range map[TrangThai]TrangThai{
		DaTiepNhan: DaTiepNhan, DangPhanLoai: DangPhanLoai, DaChuyenXuLy: DaChuyenXuLy, DangXuLy: DangXuLy,
		DaXuLy: DaXuLy, ChoDanXacNhan: DangXuLy,
	} {
		child.TrangThai = from
		if got, err := CheckUnmerge(child); err != nil || got != want {
			t.Errorf("%s: %s, %v — want %s", from, got, err, want)
		}
	}
	for _, s := range []TrangThai{DaDong, KhongTiepNhan, ChuyenCapTren, "khong-co"} {
		child.TrangThai = s
		if _, err := CheckUnmerge(child); !errors.Is(err, ErrUnmergeNotOpen) {
			t.Errorf("%s: %v", s, err)
		}
	}
}

// The (label, sentence) pair is the owner's wording of 09/10/2026 (d), and an unknown kind sends nothing.
func TestMergeCitizenMessage(t *testing.T) {
	p := PhieuPhanAnh{MaTraCuu: "PA-CHILD", LinhVuc: "rac-thai"}
	if l, s := MergeCitizenMessage(p, MergeKindMerge); l != "Ghép với phản ánh cùng vụ việc" ||
		s != "Phản ánh của anh/chị đã được ghép với phản ánh cùng vụ việc. Kết quả sẽ được báo khi xử lý xong." {
		t.Errorf("merge = %q / %q", l, s)
	}
	if l, s := MergeCitizenMessage(p, MergeKindUnmerge); l != "Xử lý riêng" ||
		s != "Phản ánh của anh/chị sẽ được xử lý riêng. Kết quả sẽ được báo khi xử lý xong." {
		t.Errorf("unmerge = %q / %q", l, s)
	}
	if l, s := MergeCitizenMessage(p, "khong-co"); l != "" || s != "" {
		t.Errorf("unknown kind = %q / %q", l, s)
	}
}

func TestMergeReasons(t *testing.T) {
	if r, err := CheckMergeReason("   "); err != nil || r != "" {
		t.Errorf("blank merge reason = %q, %v — optional, stored as none", r, err)
	}
	if _, err := CheckUnmergeReason(" \n "); !errors.Is(err, ErrUnmergeReasonMissing) {
		t.Errorf("blank unmerge reason: %v", err)
	}
	long := strings.Repeat("ạ", MergeReasonMax+1)
	if _, err := CheckMergeReason(long); !errors.Is(err, ErrMergeReasonTooLong) {
		t.Errorf("long merge reason: %v", err)
	}
	if _, err := CheckUnmergeReason(long); !errors.Is(err, ErrMergeReasonTooLong) {
		t.Errorf("long unmerge reason: %v", err)
	}
	if r, err := CheckUnmergeReason(strings.Repeat("ạ", MergeReasonMax)); err != nil || utf8Len(r) != MergeReasonMax {
		t.Errorf("max in RUNES refused: %v", err)
	}
}

func utf8Len(s string) int { return len([]rune(s)) }

func TestFollowsMain(t *testing.T) {
	for _, c := range []struct {
		child, target TrangThai
		want          bool
	}{
		{DaTiepNhan, ChoDanXacNhan, true},
		{DangXuLy, ChoDanXacNhan, true},
		{DaXuLy, ChoDanXacNhan, true},
		{ChoDanXacNhan, ChoDanXacNhan, false},
		{ChoDanXacNhan, DaDong, true},
		{DaChuyenXuLy, DaDong, true},
		{DaDong, DaDong, false},
		{KhongTiepNhan, DaDong, false},
		{ChuyenCapTren, ChoDanXacNhan, false},
		{DaTiepNhan, DangXuLy, false},
		{DaTiepNhan, DaXuLy, false},
		{"khong-co", DaDong, false},
	} {
		if got := FollowsMain(c.child, c.target); got != c.want {
			t.Errorf("FollowsMain(%s, %s) = %v", c.child, c.target, got)
		}
	}
}

// The merge sentences name the citizen's OWN case only — never the main petition (ADR 0087 §4).
func TestMergeSentences(t *testing.T) {
	p := PhieuPhanAnh{MaTraCuu: "PA-CHILD", LinhVuc: "rac-thai"}
	if !strings.Contains(MergeNextStep(p), "ghép với phản ánh cùng vụ việc") {
		t.Errorf("merge = %q", MergeNextStep(p))
	}
	if !strings.Contains(UnmergeNextStep(p), "sẽ được xử lý riêng") {
		t.Errorf("unmerge = %q", UnmergeNextStep(p))
	}
	p.LinhVuc = LinhVucHanChe
	if MergeNextStep(p) != loiNhanHanChe || UnmergeNextStep(p) != loiNhanHanChe {
		t.Error("can-bo carries more than code + status")
	}
}

func TestDistanceAndBox(t *testing.T) {
	lat, lng := 15.880123, 108.335456 // Hội An, a commune-sized place
	// ~0.00045° of latitude is ~50 m.
	d := DistanceMeters(lat, lng, lat+0.00045, lng)
	if math.Abs(d-50.04) > 0.5 {
		t.Errorf("distance = %.2f m, want ~50 m", d)
	}
	if DistanceMeters(lat, lng, lat, lng) != 0 {
		t.Error("a point is not at distance 0 from itself")
	}
	box := BoxAround(lat, lng, 50)
	// Every point exactly 50 m away in the four directions and on the diagonal lies inside the box.
	for _, p := range [][2]float64{
		{lat + 50/111195.0, lng}, {lat - 50/111195.0, lng},
		{lat, lng + 50/(111195.0*math.Cos(lat*math.Pi/180))}, {lat, lng - 50/(111195.0*math.Cos(lat*math.Pi/180))},
	} {
		if p[0] < box.MinLat || p[0] > box.MaxLat || p[1] < box.MinLng || p[1] > box.MaxLng {
			t.Errorf("point %v at 50 m is outside the box %+v", p, box)
		}
	}
}
