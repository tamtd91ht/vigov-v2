package domain

import "errors"

// Creating a task FROM a petition (POST /api/v1/citizen-reports/{maTraCuu}/tasks, user decision
// 30/09/2026). The task rules are the task register's own (nhiem_vu_ghi.go); this file holds only the
// one rule that belongs to the PETITION: which petitions may still give rise to work.

// ErrPetitionClosedForTask refuses a task on a petition the lifecycle has finished with.
//
// THE THREE STATUSES ARE `da-dong`, `khong-tiep-nhan` AND `chuyen-cap-tren`. Docs/ui-ux/09 §13 and
// 02 §57 name the route and the source, and say nothing about status; this is the ASSUMPTION the
// route was built on (stated in the report of 30/09/2026, not decided silently): a closed record is not
// reopened by a side effect. Booking work on a petition the citizen was told is finished would put a
// live obligation behind a record that reads "đã đóng", and on the two terminal branches it would be
// work on a report the commune formally refused or passed upward.
//
// `da-dong` CAN STILL MOVE — a 1–2 star rating reopens it (ADR 0050 point 2) — and that is the path
// by which it becomes eligible again, not this one.
var ErrPetitionClosedForTask = errors.New(
	"phan_anh: phiếu đã đóng hoặc đã kết thúc — không tạo nhiệm vụ mới từ phiếu này")

// PetitionAcceptsTask decides it on the row as it was read. FAIL CLOSED: a status outside the nine is
// refused, never let through as "probably open".
func PetitionAcceptsTask(p PhieuPhanAnh) error {
	if !p.TrangThai.HopLe() {
		return ErrPetitionClosedForTask
	}
	switch p.TrangThai {
	case DaDong, KhongTiepNhan, ChuyenCapTren:
		return ErrPetitionClosedForTask
	}
	return nil
}
