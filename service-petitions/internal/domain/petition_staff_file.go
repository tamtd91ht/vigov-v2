package domain

import "errors"

// The two STAFF files on a petition — migration 0027, menu phan-anh-nguoi-dan, owner decisions of
// 02/10/2026 (ADR 0047 row "Ảnh 'sau xử lý' của cán bộ — THAY G8", ADR 0008 decision 3):
//
//	verification photo  "SAU KHI XỬ LÝ" (docs/ui-ux/09 §8.4). Staff upload it; the CITIZEN sees it on
//	                    their own petition; ≤ 5; the per-commune switch makes one mandatory to close.
//	log attachment      `📎 Đính kèm` on a processing-log entry (§8.7 :197). STAFF-ONLY, never shown
//	                    to a citizen, like the log it hangs off.
//
// The use cases are internal/app/petition_verification_photo.go and petition_log_attachment.go.

// Both purposes as plain strings (domain imports nothing outside the standard library — see
// stored_file.go). Pinned against core/storage by petition_staff_file_test.go in internal/app.
const (
	PurposePetitionVerificationPhoto = "petition-verification-photo"
	PurposePetitionLogAttachment     = "petition-log-attachment"
)

// VerificationPhotoName is the `original_name` every verification photo is recorded under, for
// PetitionPhotoName's reason: a phone's file name is meaningless or describes the case (rule 3), and
// the stored bytes are a re-encoded JPEG whatever was uploaded.
const VerificationPhotoName = "anh-sau-xu-ly.jpg"

// VerificationPhotoUploadOpen reports whether staff may still add a verification photo to a petition
// in this status: EVERY STATUS EXCEPT THE THREE ENDINGS (`da-dong`, `khong-tiep-nhan`,
// `chuyen-cap-tren`).
//
// WHY THE ENDINGS ARE CLOSED: a closed petition was closed on the evidence it held at that instant —
// for a verification photo, the very evidence the close gate checked. Adding a photo afterwards edits
// what the closing rested on, silently (rule 7, forbidden #5). A REOPENED petition (`da-dong` →
// `dang-xu-ly`) is not an ending, so staff can add new evidence before closing it again.
//
// WHY EVERY OTHER STATUS IS OPEN, the early ones included: nothing in the specification ties the photo
// to one step (§8.4 shows the upload button on the detail whatever the status), and refusing an
// officer who photographs a fix done the day the report arrived would only push the photo into a
// later, less truthful upload.
//
// A LOG ATTACHMENT HAS NO SUCH WINDOW, on purpose: it rides a note, and a note is allowed on every
// status, endings included, because it APPENDS to the record and moves nothing (app.GhiChuNoiBo) — the
// task twin allows attachments on `hoan-thanh` for the same reason. A verification photo is different:
// it is the evidence the close gate reads, so adding one after the close changes what the close meant.
func VerificationPhotoUploadOpen(s TrangThai) bool {
	switch s {
	case DaDong, KhongTiepNhan, ChuyenCapTren:
		return false
	}
	return true
}

// VerificationPhotosVisibleToCitizen reports whether the citizen may see the verification photos of
// their own petition in this status: `cho-dan-xac-nhan` and `da-dong`, AND NOTHING ELSE (owner decision
// (b), 02/10/2026, ADR 0047 row "Ảnh 'sau xử lý' của cán bộ — THAY G8": "từ khi phiếu sang
// `cho-dan-xac-nhan` trở đi (và sau khi đóng), không phải ngay lúc cán bộ tải").
//
// THE CURRENT STATUS DECIDES, NOT THE HISTORY. A petition reopened by a 1–2 star rating is back at
// `dang-xu-ly`, and from then until it again reaches `cho-dan-xac-nhan` the citizen sees NO verification
// photo — not even the ones shown before the reopening. Two reasons: the photos uploaded during the
// reopened work are exactly what (b) keeps from the citizen until staff present them, and the old ones
// are the result the citizen has just rejected (decision (c)); showing those alone would need a
// per-photo cut at the reopening instant that nobody decided, and would present rejected evidence as
// the current answer. `da-dong` is shown whether or not the petition passed through `cho-dan-xac-nhan`
// ("và sau khi đóng"). `da-xu-ly`, the early statuses and the two terminal branches are hidden —
// fail closed: a status not named by the owner shows nothing.
func VerificationPhotosVisibleToCitizen(s TrangThai) bool {
	return s == ChoDanXacNhan || s == DaDong
}

// IsVerificationPhotoOf reports whether f is a verification photo of the petition with internal id
// petitionID — subject `petition`, purpose verification, uploaded by a member of staff (never the
// citizen marker, which migration 0026 keeps off this purpose anyway).
func IsVerificationPhotoOf(f StoredFile, petitionID string) bool {
	return petitionID != "" && f.SubjectType == StoredFileSubjectPetition && f.SubjectID == petitionID &&
		f.Purpose == PurposePetitionVerificationPhoto && f.UploadedBy != "" && f.UploadedBy != CitizenLogActor
}

// IsPetitionLogAttachmentOf is the same test for a log attachment of the petition.
func IsPetitionLogAttachmentOf(f StoredFile, petitionID string) bool {
	return petitionID != "" && f.SubjectType == StoredFileSubjectPetition && f.SubjectID == petitionID &&
		f.Purpose == PurposePetitionLogAttachment && f.UploadedBy != "" && f.UploadedBy != CitizenLogActor
}

// PetitionLogAttachment is one file as a petition timeline shows it — the same shape as a task's,
// for the same reasons (no object key, no uploader). An alias, not a copy: two struct types with one
// meaning are two things to keep in step.
type PetitionLogAttachment = TaskLogAttachment

// ErrPetitionAttachmentNotUsable refuses linking a file to a petition log entry. ONE SENTENCE FOR
// EVERY CAUSE, for ErrAttachmentNotUsable's reason (rule 4, forbidden #2, applied to staff).
var ErrPetitionAttachmentNotUsable = errors.New(
	"tệp đính kèm: chỉ đính kèm được tệp chính bạn đã tải lên cho nhật ký của phiếu này, đã tải xong " +
		"và chưa gắn vào dòng nhật ký nào")

// CheckPetitionLogAttachable is the write path's copy of migration 0027's
// `petition_log_attachment_check`, plus "not yet on any entry" (the table's primary key). The trigger
// stays the floor; this turns a refusal into a sentence instead of a 500.
func CheckPetitionLogAttachable(c AttachCandidate, petitionID, author string) error {
	f := c.File
	if author == "" || !IsPetitionLogAttachmentOf(f, petitionID) || f.UploadedBy != author ||
		!f.Status.Attachable() || c.LinkedTo != "" {
		return ErrPetitionAttachmentNotUsable
	}
	return nil
}

// MayDownloadPetitionLogAttachment is MayDownload for a petition log attachment: the file belongs to
// THIS petition as a log attachment and reached the destination, AND it is on a log entry — part of
// the timeline every staff reader of the petition sees — OR the reader uploaded it (checking a file
// before pressing `➤ Ghi nhật ký`). A draft is nobody's business but its uploader's.
func MayDownloadPetitionLogAttachment(f StoredFile, petitionID, linkedTo, reader string) bool {
	if !IsPetitionLogAttachmentOf(f, petitionID) || !f.Status.Attachable() {
		return false
	}
	return linkedTo != "" || (reader != "" && f.UploadedBy == reader)
}

// ErrVerificationPhotoRequired refuses closing a petition that holds no stored verification photo in
// a commune whose switch requires one (ADR 0008 decision 3, `bat_buoc_anh_nghiem_thu`, default ON).
// Its wire sentence is the commune's "Lời hệ thống" wording of KeyFeedbackAfterPhotoRequired.
//
// A REOPENED petition is refused with the same sentinel when none of its stored photos was uploaded
// after the most recent reopening (owner decision (c), 02/10/2026) — wrapped, so errors.Is still holds
// and the wire answer is the same 409 `after_photo_required`.
var ErrVerificationPhotoRequired = errors.New(
	"phan_anh: xã bắt buộc có ảnh sau xử lý trước khi đóng phiếu, và phiếu chưa có ảnh nào")
