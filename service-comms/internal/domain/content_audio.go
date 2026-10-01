package domain

// THE BROADCAST AUDIO OF A `truyen-thanh` ITEM (ADR 0067 §4, migration 0012): the rules for the two
// columns `audio_file_id` / `audio_duration_seconds` and for pointing an item at an uploaded file.
// The file itself is a `stored_file` row like a cover's (content_cover.go); what differs is the purpose
// (content-audio), that there is no derivative, and that nothing of it is ever public — it is delivered
// by a short-lived presigned GET of the PRIVATE original (ADR 0067 §4.2, ADR 0052 stop condition #2).

import "errors"

// The duration bounds are migration 0012's `noi_dung_mini_app_audio_duration_range`. 21600 s (6 h) is a
// VENDOR bound against a bogus value, not a customer figure (0012 says why).
const (
	AudioDurationMinSeconds = 1
	AudioDurationMaxSeconds = 21600
)

var (
	// ErrAudioDurationInvalid — outside 1 .. 21600 seconds.
	ErrAudioDurationInvalid = errors.New(
		"noi_dung_mini_app: `audio_duration_seconds` phải là số giây từ 1 đến 21600 (6 giờ)")

	// ErrAudioOnlyForBroadcast — `noi_dung_mini_app_audio_only_for_truyen_thanh`: audio on any other type.
	ErrAudioOnlyForBroadcast = errors.New("noi_dung_mini_app: tệp âm thanh chỉ dùng cho loại truyen-thanh")

	// ErrAudioAllOrNone — `noi_dung_mini_app_audio_all_or_none`: a file with no duration, or a duration
	// with no file.
	ErrAudioAllOrNone = errors.New(
		"noi_dung_mini_app: tệp âm thanh và thời lượng đi cùng nhau — có tệp thì phải có thời lượng, gỡ tệp thì gỡ cả thời lượng")

	// ErrAudioFileIDInvalid — `audio_file_id` that cannot be an id this service mints. Never echoed.
	ErrAudioFileIDInvalid = errors.New("noi_dung_mini_app: `audio_file_id` không hợp lệ")

	// ErrAudioNotUsable refuses pointing an item at a file. ONE SENTENCE FOR EVERY CAUSE — unknown id,
	// another commune's, uploaded for another item, not finished, refused, deleted — for ErrCoverNotUsable's
	// reason (rule 4, forbidden #2, applied to staff).
	ErrAudioNotUsable = errors.New(
		"truyền thanh: chỉ dùng được tệp âm thanh đã tải lên cho chính mục này và đã xử lý xong — tệp được gắn khi hoàn tất tải lên")
)

// CheckAudioDuration is the range CHECK on one typed value.
func CheckAudioDuration(seconds int) error {
	if seconds < AudioDurationMinSeconds || seconds > AudioDurationMaxSeconds {
		return ErrAudioDurationInvalid
	}
	return nil
}

// checkAudioFields is migration 0012's three audio CHECKs on one set of values ("" / 0 = NULL).
func checkAudioFields(loai LoaiNoiDung, fileID string, seconds int) error {
	if fileID == "" && seconds == 0 {
		return nil
	}
	if loai != LoaiTruyenThanh {
		return ErrAudioOnlyForBroadcast
	}
	if fileID == "" || seconds == 0 {
		return ErrAudioAllOrNone
	}
	return CheckAudioDuration(seconds)
}

// CheckAudioUsable is the write path's copy of migration 0012's `noi_dung_mini_app_audio_file_check`,
// ONE STEP STRICTER, as CheckCoverUsable is: the trigger admits `stored` / `processing` / `ready`, this
// admits `ready` only — the state a completion reaches after the type sniff, the whole-stream audio check
// and the malware scan all passed.
func CheckAudioUsable(f *StoredFile, itemID, purpose string) error {
	if f == nil || itemID == "" || f.SubjectType != StoredFileSubjectContentItem ||
		f.SubjectID != itemID || f.Purpose != purpose || f.Status != StoredFileReady {
		return ErrAudioNotUsable
	}
	return nil
}
