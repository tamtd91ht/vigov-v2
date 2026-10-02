package domain

// A CITIZEN'S SCENE PHOTO ON THEIR OWN PETITION — menu phan-anh-nguoi-dan, docs/ui-ux/09 §8.4
// "TRƯỚC KHI XỬ LÝ"; owner decisions of 02/10/2026 recorded in ADR 0047 (row "Ảnh hiện trường khi gửi
// phản ánh", and G3); storage shape in migration 0026. The use case is internal/app/petition_photo.go.

// PetitionPhotoName is the `original_name` every citizen photo is recorded under, and the file name a
// signed link offers on download.
//
// A FIXED SERVER NAME, NOT THE CLIENT'S. A phone's file name is either meaningless (`IMG_2041.jpg`) or
// a description of the case, which is personal data (rule 3, ADR 0052 §3); nothing a reader needs is
// in it, and the stored bytes are a re-encoded JPEG anyway, so the client's extension would be false.
// Not taking it removes one personal-data field from the request, the row and the staff screen.
const PetitionPhotoName = "anh-hien-truong.jpg"

// PhotoUploadOpen reports whether a citizen may still attach photos to a petition in this status.
//
// ONLY `da-tiep-nhan` (owner, 02/10/2026): photos are attached AFTER the petition exists, by its
// lookup code, and may be retried from "Phản ánh của tôi" while it is still there. Once staff move it
// on, the set staff are looking at must not grow silently behind them. Checked when an upload slot is
// issued AND again, under the petition's row lock, when the photo is stored.
func PhotoUploadOpen(s TrangThai) bool { return s == DaTiepNhan }

// IsCitizenPhotoOf reports whether f is a citizen scene photo of the petition with internal id
// petitionID — the shape migration 0026 forces on every citizen row (subject `petition`, purpose
// `petition-photo`, uploader the fixed citizen marker). WHICH citizen is not on the row: it is the
// petition's `cong_dan_id`, which the caller has already matched against the session.
func IsCitizenPhotoOf(f StoredFile, petitionID string) bool {
	return petitionID != "" && f.SubjectType == StoredFileSubjectPetition && f.SubjectID == petitionID &&
		f.Purpose == PurposePetitionPhoto && f.UploadedBy == CitizenLogActor
}

// PurposePetitionPhoto is core/storage.PurposePetitionPhoto as a plain string (domain imports nothing
// outside the standard library — see the note at the top of stored_file.go). Pinned against
// core/storage by petition_photo_test.go in internal/app.
const PurposePetitionPhoto = "petition-photo"
