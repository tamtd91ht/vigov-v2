package domain

// Merging duplicate petitions — ADR 0087, the ADR 0041 amendment of 09/10/2026, and the owner's
// answers of 09/10/2026 to ADR 0087's four open items (relayed by the main session and recorded in
// migration 0037's header). Standard library only.
//
// A MERGE IS A LINK, NOT A STATUS (ADR 0087 §1). The merged petition keeps its lookup code, its
// stored deadline and its own lifecycle; it points at the MAIN petition, and follows it only at the
// two moments ADR 0087 §2 names. Nothing here closes a petition or adds a status.
//
// WHAT THE DATABASE ALREADY REFUSES (migration 0037's trigger and CHECKs) IS REFUSED HERE FIRST, IN
// VIETNAMESE: the database is the floor, these functions are the sentence. A drift between the two is
// a worse error message, never a hole.

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

// MergeKind is the act a petition_merge_event row records. Enum VALUES are Vietnamese without
// diacritics (ADR 0011) — migration 0037's CHECK `petition_merge_event_kind_valid`.
type MergeKind string

const (
	MergeKindMerge   MergeKind = "gop-phieu"
	MergeKindUnmerge MergeKind = "tach-phieu"
)

// PetitionMergeEvent is one row of `petition_merge_event` (migration 0037): append-only, staff-internal.
//
// THE MAIN DEADLINE BEFORE AND AFTER is the main petition's `han_xu_ly_xong` around the act; zero is
// SQL NULL, "chưa có" (ADR 0028). The merged petition's own deadline never changes and is not copied.
type PetitionMergeEvent struct {
	ID                 string
	PetitionID         string // the merged petition — the one carrying merged_into
	MainPetitionID     string
	Kind               MergeKind
	PerformedAt        time.Time
	PerformedBy        string // staff BUSINESS code (rule 6, invariant 8)
	Reason             string // "" = none (merge only); mandatory on unmerge. MAY HOLD PERSONAL DATA
	MainDeadlineBefore time.Time
	MainDeadlineAfter  time.Time
}

// MergeLinks is the staff detail's view of one petition's links: the main petition's CODE when this one
// is merged, and the codes of the petitions merged into it when it is a main one. Codes, never ids.
// Both empty on a petition nobody merged. With no chains (owner, 09/10/2026) at most one is non-empty.
type MergeLinks struct {
	MainCode   string
	ChildCodes []string
}

// mergeOpen is the owner's set of 09/10/2026: a merge only while BOTH petitions are unresolved.
//
// `da-xu-ly` IS OUTSIDE IT — the owner's range was "da-tiep-nhan … dang-xu-ly" and "both not yet
// resolved", and `da-xu-ly` is resolved (migration 0037's header states this reading so it can be
// challenged). The SAME FOUR CODES as the trigger `phieu_phan_anh_merge_guard`; a drift would make the
// database refuse what this file allowed, which is a 500 instead of a sentence.
var mergeOpen = map[TrangThai]bool{
	DaTiepNhan:   true,
	DangPhanLoai: true,
	DaChuyenXuLy: true,
	DangXuLy:     true,
}

// MergeOpen reports whether a petition in status `t` may take part in a merge — either side.
func MergeOpen(t TrangThai) bool { return mergeOpen[t] }

// MergeReasonMax bounds the merge / unmerge reason — the database's number (CHECK
// `petition_merge_event_reason_max`), validated here in RUNES so an oversized reason is a 400, not a 500.
const MergeReasonMax = KetQuaToiDa

var (
	// ErrMergeMainMissing — the body named no main petition.
	ErrMergeMainMissing = errors.New("phan_anh: `main_code` trống — phải chọn phiếu chính")
	// ErrMergeSelf — a petition is not a duplicate of itself (CHECK `phieu_phan_anh_not_merged_into_self`).
	ErrMergeSelf = errors.New("phan_anh: không gộp một phiếu vào chính nó")
	// ErrMergeReasonTooLong — over MergeReasonMax.
	ErrMergeReasonTooLong = errors.New("phan_anh: `reason` quá dài")
	// ErrUnmergeReasonMissing — unmerge needs a reason (owner, 09/10/2026; CHECK
	// `petition_merge_event_reason_valid`).
	ErrUnmergeReasonMissing = errors.New("phan_anh: `reason` trống — tách phiếu phải ghi lý do")

	// The STATE refusals — about the records, answered 409.
	ErrMergeNotOpen        = errors.New("phan_anh: chỉ gộp được hai phiếu đều chưa xử lý xong")
	ErrMergeAlreadyMerged  = errors.New("phan_anh: phiếu này đã được gộp vào một phiếu chính")
	ErrMergeTargetIsMerged = errors.New("phan_anh: phiếu được chọn làm phiếu chính đã được gộp vào phiếu khác")
	ErrMergeHasChildren    = errors.New("phan_anh: phiếu này đang là phiếu chính của phiếu khác nên không gộp tiếp được")
	ErrMergeStaffConduct   = errors.New("phan_anh: phiếu lĩnh vực cán bộ không bao giờ được gộp")
	ErrNotMerged           = errors.New("phan_anh: phiếu này không được gộp vào phiếu nào")
	ErrUnmergeNotOpen      = errors.New("phan_anh: chỉ tách được phiếu chưa đóng")

	// ErrMergeDeadlineBeforeOrigin — "the main takes the earlier deadline" would put the main's
	// `han_xu_ly_xong` BEFORE its own `goc_dem_han`, which migration 0004's CHECK
	// `phieu_phan_anh_han_sau_goc` refuses. Migration 0037's header names it as a consequence the owner
	// has not seen; this file does NOT loosen it. FAIL CLOSED: the merge is refused, and the officer can
	// pick the EARLIER-reported petition as the main one instead.
	ErrMergeDeadlineBeforeOrigin = errors.New(
		"phan_anh: hạn sớm hơn của phiếu gộp nằm trước lúc phiếu chính được phản ánh")
)

// CheckMergeReason trims and bounds the OPTIONAL merge reason. Blank after trim is "no reason" — the
// CHECK refuses a blank one, so it is never stored as "".
func CheckMergeReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > MergeReasonMax {
		return "", ErrMergeReasonTooLong
	}
	return s, nil
}

// CheckUnmergeReason trims and bounds the MANDATORY unmerge reason.
//
// NO MINIMUM BEYOND NON-BLANK: this is a staff-internal record of why, not a sentence a citizen reads
// (that is KetQuaToiThieu's job), and a floor would refuse a short true answer ("chọn nhầm phiếu").
func CheckUnmergeReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrUnmergeReasonMissing
	case utf8.RuneCountInString(s) > MergeReasonMax:
		return "", ErrMergeReasonTooLong
	}
	return s, nil
}

// CheckMerge decides whether `child` may be merged into `main`, and returns the deadline the MAIN
// petition must carry before the link is written. `childHasChildren` is whether some petition is
// already merged into `child` — a fact of other rows the caller read under the same transaction.
//
// THE ORDER OF THE CHECKS IS THE ORDER OF THE SENTENCES, NOT OF IMPORTANCE: every one of them is also
// enforced by migration 0037. The restricted-field refusal for a caller without `feedback.restricted` is
// NOT here — it is a 404 decided before this runs (app.duocChamPhieuHanChe), so this function never tells
// such a caller that a `can-bo` petition exists.
//
// THE DEADLINE (ADR 0087 §1; owner 09/10/2026, open item #1): the EARLIER of the two; when one is "chưa
// có" (zero, ADR 0028) the other is taken — HanSomHon is exactly that rule. It only ever moves EARLIER,
// so no promise already made to the main petition's citizen is lengthened (ADR 0087 stop condition #2).
func CheckMerge(child, main PhieuPhanAnh, childHasChildren bool) (time.Time, error) {
	switch {
	case child.ID == main.ID:
		return time.Time{}, ErrMergeSelf
	case child.LinhVuc == LinhVucHanChe || main.LinhVuc == LinhVucHanChe:
		return time.Time{}, ErrMergeStaffConduct
	case child.MergedInto != "":
		return time.Time{}, ErrMergeAlreadyMerged
	case main.MergedInto != "":
		return time.Time{}, ErrMergeTargetIsMerged
	case childHasChildren:
		return time.Time{}, ErrMergeHasChildren
	case !MergeOpen(child.TrangThai) || !MergeOpen(main.TrangThai):
		return time.Time{}, ErrMergeNotOpen
	}
	han := HanSomHon(main.HanXuLyXong, child.HanXuLyXong)
	if !han.IsZero() && han.Before(main.GocDemHan) {
		return time.Time{}, ErrMergeDeadlineBeforeOrigin
	}
	return han, nil
}

// CheckUnmerge decides whether `child` may be taken out of its main petition, and returns the status it
// stands in AFTER the unmerge.
//
// ANY TIME BEFORE IT IS CLOSED (owner, 09/10/2026, decision (c)) — replacing the earlier "only while
// unresolved". Per status:
//
//	the four open ones, `da-xu-ly`   allowed; the status does not move — the link is all that changes
//	`cho-dan-xac-nhan`               allowed; the petition RETURNS TO `dang-xu-ly` along the lifecycle's
//	                                 existing edge (chuyenDuocSang — the reopen edge). It most likely got
//	                                 there by following its main (ADR 0087 §2), and once separated it is
//	                                 no longer finished: asking its citizen to confirm the MAIN's work
//	                                 would be a confirmation of somebody else's case. Its deadline stays
//	                                 as stored (rule 10, invariant 2)
//	`da-dong`                        refused — taking a closed record out of the incident it was closed
//	                                 with would edit an archival record (rule 7, forbidden #5)
//	`khong-tiep-nhan`,               refused, READ AS "closed" too: both are terminal (no way out of
//	`chuyen-cap-tren`                chuyenDuocSang), and the unmerge message promises "sẽ được xử lý
//	                                 riêng" — false for a petition the commune refused or passed on.
//	                                 Fail closed; an assumption stated in the card's report
//
// THE EDGE IS ASKED OF THE LIFECYCLE, NOT ASSUMED: if `cho-dan-xac-nhan -> dang-xu-ly` ever leaves
// chuyenDuocSang, this refuses rather than writing a move the lifecycle does not have (rule 10 stop #2).
//
// THE MAIN PETITION'S DEADLINE IS NOT LENGTHENED BACK (ADR 0087 §Hệ quả): the caller writes the event
// with the same deadline before and after.
func CheckUnmerge(child PhieuPhanAnh) (TrangThai, error) {
	switch {
	case child.MergedInto == "":
		return "", ErrNotMerged
	case MergeOpen(child.TrangThai), child.TrangThai == DaXuLy:
		return child.TrangThai, nil
	case child.TrangThai == ChoDanXacNhan && ChoDanXacNhan.ChuyenSangDuoc(DangXuLy):
		return DangXuLy, nil
	}
	return "", ErrUnmergeNotOpen
}

// FollowsMain reports whether a merged petition in status `child` moves along when its main petition
// enters `target` (ADR 0087 §2): the main entering `cho-dan-xac-nhan` or `da-dong`, and the merged one
// not already there and not ended. A merged petition that was refused or referred on its own, or is
// already closed, stays where it is — the main's act is not a second ending of a record that has one.
func FollowsMain(child, target TrangThai) bool {
	if target != ChoDanXacNhan && target != DaDong {
		return false
	}
	switch child {
	case target, DaDong, KhongTiepNhan, ChuyenCapTren:
		return false
	}
	return child.HopLe()
}

// IsMergeStateRefusal reports the refusals about the STATE of the records — answered 409, never 400.
func IsMergeStateRefusal(err error) bool {
	for _, e := range []error{ErrMergeNotOpen, ErrMergeAlreadyMerged, ErrMergeTargetIsMerged,
		ErrMergeHasChildren, ErrMergeStaffConduct, ErrNotMerged, ErrUnmergeNotOpen} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// --- what the citizen of the merged petition is told -------------------------------------------------

// The label and sentence ADR 0041 §Sửa đổi 09/10/2026 owes the citizen of the MERGED petition on a merge
// and on an unmerge — the owner's wording of 09/10/2026, decisions (a) and (d). Beside loiNhanChoDan
// rather than in it, because that table is keyed by the TARGET status and neither act is a status (ADR
// 0041 open item #3: extend the mechanism, no second list).
//
// CARRIED BY `petitions.merge_changed.v1` (PetitionMergeChanged.citizen_message), not by
// `status_changed`: comms deduplicates on (code, moc, occurrence, recipient, channel), and a merge sent
// under the petition's current status would collapse onto the notice already sent for that status.
//
// THE LABEL IS THE ACT'S, not a status's (events.proto, CitizenMessage.status_label): a software string,
// the same in every commune. NEITHER TEXT NAMES THE MAIN PETITION — its code, content, photos or reporter
// (ADR 0087 §4, rule 4) — and neither is the staff-written reason (rule 3).
const (
	mergeStatusLabel   = "Ghép với phản ánh cùng vụ việc"
	mergeMessage       = "Phản ánh của anh/chị đã được ghép với phản ánh cùng vụ việc. Kết quả sẽ được báo khi xử lý xong."
	unmergeStatusLabel = "Xử lý riêng"
	unmergeMessage     = "Phản ánh của anh/chị sẽ được xử lý riêng. Kết quả sẽ được báo khi xử lý xong."
)

// MergeCitizenMessage is the (label, sentence) owed to the citizen of `p` for act `kind`. Both "" for a
// kind this file does not know — the caller then has nothing to send and must refuse, not guess.
func MergeCitizenMessage(p PhieuPhanAnh, kind MergeKind) (label, nextStep string) {
	switch kind {
	case MergeKindMerge:
		return mergeStatusLabel, MergeNextStep(p)
	case MergeKindUnmerge:
		return unmergeStatusLabel, UnmergeNextStep(p)
	}
	return "", ""
}

// MergeNextStep is the sentence owed to the citizen of `p` when it is merged into a main petition.
func MergeNextStep(p PhieuPhanAnh) string {
	if p.LinhVuc == LinhVucHanChe { // unreachable — can-bo is never merged — and decided anyway
		return loiNhanHanChe
	}
	return mergeMessage
}

// UnmergeNextStep is the sentence owed to the citizen of `p` when it is taken out of its main petition.
func UnmergeNextStep(p PhieuPhanAnh) string {
	if p.LinhVuc == LinhVucHanChe {
		return loiNhanHanChe
	}
	return unmergeMessage
}

// --- the suspected-duplicate search (ADR 0087 §6) ------------------------------------------------------

// earthRadiusMeters is the IUGG mean radius. The search is a SUGGESTION within tens of metres; the
// spheroid's error at this scale is far below a phone's GPS error.
const earthRadiusMeters = 6371008.8

// DistanceMeters is the great-circle (haversine) distance between two points in degrees.
func DistanceMeters(lat1, lng1, lat2, lng2 float64) float64 {
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(a)))
}

// GeoBox is a latitude/longitude rectangle in degrees.
type GeoBox struct {
	MinLat, MaxLat, MinLng, MaxLng float64
}

// BoxAround is the rectangle that CONTAINS every point within `radius` metres of (lat, lng) — the
// index-friendly pre-filter the database runs (no PostGIS, ADR 0072); DistanceMeters refines it.
//
// IT ERRS WIDE, never narrow: a box that is too small drops a real duplicate silently, one that is too
// wide costs a few rows the refinement throws away. Hence the 1% pad and the clamp on cos(lat), which
// only matters near a pole — far from any commune.
func BoxAround(lat, lng, radius float64) GeoBox {
	dLat := radius / earthRadiusMeters * 180 / math.Pi * 1.01
	c := math.Cos(lat * math.Pi / 180)
	if c < 0.01 {
		c = 0.01
	}
	dLng := dLat / c
	return GeoBox{MinLat: lat - dLat, MaxLat: lat + dLat, MinLng: lng - dLng, MaxLng: lng + dLng}
}
