package domain

import "time"

// StatusEntryTimes answers which of migration 0043's two status-entry columns a move from `from` to
// `to` at instant `at` writes (ADR 0047 G4, ADR 0091 §5). A zero result means "leave that column as it
// is"; the store spells it NULL and keeps the stored value with COALESCE — the shape `xu_ly_xong_luc`
// already has.
//
// ONLY A REAL ENTRY WRITES. `from == to` is no transition — a re-assignment that keeps a petition in
// `da-chuyen-xu-ly` (SauKhiPhanCong) must not move the instant it was handed over. A RE-ENTRY after a
// reopening is a real entry and DOES overwrite: G4 decided "mở lại lấy giờ mới nhất".
//
// ONE FUNCTION FOR EVERY WRITE PATH (assignment, the forward advance, a merged petition following its
// main), so no path can enter one of the two statuses without its time — the defect this exists to
// prevent is a citizen timeline that shows the time on one road into the status and not on another.
func StatusEntryTimes(from, to TrangThai, at time.Time) (forwardedAt, awaitingAt time.Time) {
	if from == to {
		return time.Time{}, time.Time{}
	}
	switch to {
	case DaChuyenXuLy:
		return at, time.Time{}
	case ChoDanXacNhan:
		return time.Time{}, at
	}
	return time.Time{}, time.Time{}
}
