package domain

import (
	"testing"
	"time"
)

var entryAt = time.Date(2026, 10, 11, 2, 30, 0, 0, time.UTC)

func TestStatusEntryTimes(t *testing.T) {
	for _, c := range []struct {
		name               string
		from, to           TrangThai
		forwarded, waiting bool
	}{
		{"first routing", DangPhanLoai, DaChuyenXuLy, true, false},
		{"work done, citizen asked", DaXuLy, ChoDanXacNhan, false, true},
		// A merged petition following its main from wherever it stood (ADR 0087 §2).
		{"merged follows main", DangXuLy, ChoDanXacNhan, false, true},
		// Re-assignment mid-processing keeps the status, so the handed-over instant must not move.
		{"re-assignment, same status", DaChuyenXuLy, DaChuyenXuLy, false, false},
		{"re-assignment while waiting", ChoDanXacNhan, ChoDanXacNhan, false, false},
		{"other step", DaChuyenXuLy, DangXuLy, false, false},
		{"finishing step", DangXuLy, DaXuLy, false, false},
		{"closing", ChoDanXacNhan, DaDong, false, false},
	} {
		f, w := StatusEntryTimes(c.from, c.to, entryAt)
		if (!f.IsZero()) != c.forwarded || (!w.IsZero()) != c.waiting {
			t.Errorf("%s: forwarded=%v awaiting=%v, want %v/%v", c.name, f, w, c.forwarded, c.waiting)
		}
		if !f.IsZero() && !f.Equal(entryAt) || !w.IsZero() && !w.Equal(entryAt) {
			t.Errorf("%s: the instant is the act's, got %v / %v", c.name, f, w)
		}
	}
}

// G4: a reopened petition that enters the status AGAIN takes the new instant. The answer depends only
// on the move, never on what is stored — so the second entry is written exactly like the first, and
// the store's COALESCE(new, stored) keeps the new one.
func TestStatusEntryTimesReEntryOverwrites(t *testing.T) {
	later := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC) // after a reopening
	_, first := StatusEntryTimes(DaXuLy, ChoDanXacNhan, entryAt)
	_, second := StatusEntryTimes(DaXuLy, ChoDanXacNhan, later)
	if !first.Equal(entryAt) || !second.Equal(later) {
		t.Errorf("first=%v second=%v — a re-entry must carry its own instant", first, second)
	}
}
