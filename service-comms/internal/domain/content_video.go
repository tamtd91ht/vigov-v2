package domain

// THE TYPED DURATION OF A `video` ITEM (migration 0028; owner decision 11/10/2026, ADR 0091 §7): the
// prototype's video list shows a length on every card, and staff type it — the link is external
// (ADR 0047 §6 (3)), so there is nothing here to measure.
//
// UNLIKE THE AUDIO PAIR (content_audio.go) THE DURATION IS NOT TIED TO `video_url`: rows written before
// 0028 have a link and no duration, and staff may leave the length unknown. 0 means NULL ("not given").

import "errors"

// The bounds are migration 0028's `noi_dung_mini_app_video_duration_range` — the same VENDOR bound as
// the audio one (0012), against a bogus value, not a customer figure.
const (
	VideoDurationMinSeconds = 1
	VideoDurationMaxSeconds = 21600
)

var (
	// ErrVideoDurationInvalid — outside 1 .. 21600 seconds.
	ErrVideoDurationInvalid = errors.New(
		"noi_dung_mini_app: `video_duration_seconds` phải là số giây từ 1 đến 21600 (6 giờ)")

	// ErrVideoDurationOnlyForVideo — `noi_dung_mini_app_video_duration_only_for_video`. REFUSED rather
	// than dropped when the client SENT a value, for ErrVideoURLOnlyForVideo's reason.
	ErrVideoDurationOnlyForVideo = errors.New(
		"noi_dung_mini_app: `video_duration_seconds` chỉ dùng cho loại video")
)

// CheckVideoDuration is the range CHECK on one typed value. 0 is NULL and valid.
func CheckVideoDuration(seconds int) error {
	if seconds == 0 {
		return nil
	}
	if seconds < VideoDurationMinSeconds || seconds > VideoDurationMaxSeconds {
		return ErrVideoDurationInvalid
	}
	return nil
}

// checkVideoDuration is 0028's two CHECKs on one set of values.
func checkVideoDuration(loai LoaiNoiDung, seconds int) error {
	if seconds == 0 {
		return nil
	}
	if loai != LoaiVideo {
		return ErrVideoDurationOnlyForVideo
	}
	return CheckVideoDuration(seconds)
}
