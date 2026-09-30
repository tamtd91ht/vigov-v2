package domain

import "errors"

// Creating a task FROM a petition (POST /api/v1/citizen-reports/{maTraCuu}/tasks, user decision
// 30/09/2026). The task rules are the task register's own (nhiem_vu_ghi.go); this file holds only the
// rules that belong to the PETITION: which petitions may give rise to work.
//
// # THE RULE, AS THE USER DECIDED IT ON 30/09/2026
//
// A task may be booked only on a petition the commune has ACCEPTED and CLASSIFIED and not yet closed,
// and never on one in the restricted field `can-bo`. That replaced the builder's first assumption
// ("every status that is not closed"), which let a task be booked on a report nobody had yet read.
//
// # WHICH STATUSES THAT IS, READ OFF THE LIFECYCLE MAP (phieu_phan_anh.go chuyenDuocSang)
//
//	da-tiep-nhan     refused  nobody has classified it: the field is not settled by the commune and
//	                          `han_xu_ly_xong` may not be fixed yet (ADR 0028 decision E)
//	dang-phan-loai   refused  classified but NOT YET ACCEPTED: this is the step that decides whether the
//	                          commune takes the report at all — both terminal branches leave from here
//	                          and from nowhere else (KetThucNhanhDuoc)
//	da-chuyen-xu-ly  allowed  the first status after acceptance: SauKhiPhanCong moves `dang-phan-loai`
//	dang-xu-ly       allowed  to it, and it is the only edge in, so everything from here on has passed
//	da-xu-ly         allowed  both classification AND acceptance
//	cho-dan-xac-nhan allowed
//	da-dong          refused  closed (a 1–2 star rating reopens it to `dang-xu-ly` — ADR 0050 point 2 —
//	                          and that is the path by which it becomes eligible again, not this one)
//	khong-tiep-nhan  refused  terminal: the commune formally refused the report
//	chuyen-cap-tren  refused  terminal: the report was passed to another body
//
// # THE FIELD IS REQUIRED TOO, AS A SECOND WALL
//
// The status is the authority: `da-chuyen-xu-ly` cannot be reached without going through ChotLinhVuc,
// which settles `linh_vuc`. `linh_vuc` alone would NOT be enough the other way round — a citizen channel
// may carry a field at booking, so `da-tiep-nhan` can already hold one. A row at an allowed status with
// an EMPTY field can only come from data written outside the lifecycle, and it is refused as
// unclassified rather than let through (fail closed).
//
// `han_xu_ly_xong` IS DELIBERATELY NOT REQUIRED. It is fixed by the same act that settles the field, so
// on a row the lifecycle produced it is always set; requiring it as well would refuse, with a sentence
// telling the officer to "classify first", a petition that can no longer be classified (ChotLinhVuc
// accepts only `da-tiep-nhan`) — a refusal with no way out.

// ErrPetitionClosedForTask refuses a task on a petition the lifecycle has finished with: `da-dong`,
// `khong-tiep-nhan`, `chuyen-cap-tren`, and any code that is not one of the nine (fail closed).
var ErrPetitionClosedForTask = errors.New(
	"phan_anh: phiếu đã đóng hoặc đã kết thúc — không tạo nhiệm vụ mới từ phiếu này")

// ErrPetitionNotClassifiedForTask refuses a task on a petition the commune has not yet classified AND
// accepted: `da-tiep-nhan`, `dang-phan-loai`, or an allowed status with no settled field.
var ErrPetitionNotClassifiedForTask = errors.New(
	"phan_anh: phiếu chưa được phân loại và tiếp nhận xử lý — không tạo nhiệm vụ từ phiếu này")

// ErrRestrictedFieldNoTask refuses a task on a `can-bo` petition, to a caller who MAY see it. The same
// shape as the documents register's tố cáo (C9): a report about a member of staff is handled by the
// leadership directly on the petition, never handed out as a task — a task is visible to its assignee,
// its monitor and every `task.read` holder, none of whom the restricted field was opened to.
//
// A caller WITHOUT `feedback.restricted` never reaches this error: the use case answers that case with
// the 404 of an unknown code first, so the existence of the petition is not revealed.
var ErrRestrictedFieldNoTask = errors.New(
	"phan_anh: phiếu thuộc lĩnh vực hạn chế — không tạo nhiệm vụ từ phiếu này")

// PetitionAcceptsTask decides it on the row as it was read. ORDER: an unknown status first (fail
// closed), then the restricted field (so a key holder gets the reason that applies whatever the status),
// then the lifecycle.
func PetitionAcceptsTask(p PhieuPhanAnh) error {
	if !p.TrangThai.HopLe() {
		return ErrPetitionClosedForTask
	}
	if p.LinhVuc == LinhVucHanChe {
		return ErrRestrictedFieldNoTask
	}
	switch p.TrangThai {
	case DaChuyenXuLy, DangXuLy, DaXuLy, ChoDanXacNhan:
		if p.LinhVuc == "" {
			return ErrPetitionNotClassifiedForTask
		}
		return nil
	case DaTiepNhan, DangPhanLoai:
		return ErrPetitionNotClassifiedForTask
	default:
		// `da-dong`, `khong-tiep-nhan`, `chuyen-cap-tren`.
		return ErrPetitionClosedForTask
	}
}
