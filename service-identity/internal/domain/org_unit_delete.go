package domain

// The rules of removing one org-chart node (menu Cấu hình §12.4 "Xoá bộ phận"; user decision
// 2026-09-28). Standard library only.
//
// A UNIT IS REMOVED ONLY WHEN IT HOLDS NOTHING. §12.4 says "yêu cầu chuyển trước": the person moves
// the staff, the child units and the open records elsewhere first, then deletes. The refusal names
// every kind and its count, because "still in use" alone sends somebody hunting across four screens.
//
// WHAT COUNTS (the user's decision, not this file's): live staff, live child units, open petitions,
// open tasks, open incoming documents. Finance projects do NOT count. The last three are owned by
// petitions and documents; their predicates live in the proto contracts, not here.

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrOrgUnitDeleteReasonTooLong — a reason that IS typed, past MaxOrgUnitDeleteReason.
var ErrOrgUnitDeleteReasonTooLong = errors.New("bo_phan: lý do xoá bộ phận quá dài")

// MaxOrgUnitDeleteReason bounds the reason, counted in runes — the same 500 every other soft delete
// in this service uses (ChuanHoaLyDoXoa, ChuanHoaLyDoXoaDanhMuc).
const MaxOrgUnitDeleteReason = 500

// OrgUnitDeleteDefaultReason is `delete_reason` when the person removing a unit types none.
//
// OPTIONAL ON THE WAY IN (owner decision 10/10/2026: the "Xoá bộ phận" confirmation drops the reason
// box), NEVER EMPTY IN THE ROW — rule 7, invariant 1 names `delete_reason`; the shape of ADR 0075 #4a
// and ADR 0077 #3. The unit could only be removed once it held nothing (§12.4), and the zero counts
// that decision rested on are in the audit delta, so the sentence need only say where it came from.
const OrgUnitDeleteDefaultReason = "Xoá bộ phận khỏi sơ đồ tổ chức khi không còn cán bộ, bộ phận con hay hồ sơ đang xử lý (người xoá không nhập lý do)"

// NormalizeOrgUnitDeleteReason trims and bounds the reason; a blank one becomes
// OrgUnitDeleteDefaultReason.
func NormalizeOrgUnitDeleteReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	switch {
	case reason == "":
		return OrgUnitDeleteDefaultReason, nil
	case utf8.RuneCountInString(reason) > MaxOrgUnitDeleteReason:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrOrgUnitDeleteReasonTooLong, MaxOrgUnitDeleteReason)
	}
	return reason, nil
}

// OrgUnitHoldings is everything one unit still holds, by kind.
//
// THE FIRST TWO ARE COUNTED HERE (identity's own tables), THE LAST THREE ARE ANSWERED BY THEIR
// OWNERS over gRPC. A kind added to either contract is added here, to Any and to Sentence in the
// same change: a kind not read is a kind that reads as zero, which is the one direction that lets a
// delete through silently.
type OrgUnitHoldings struct {
	Staff                 int // live staff sitting in the unit — the org chart's own SoCanBo predicate
	ChildUnits            int // live units whose parent is this one
	OpenPetitions         int
	OpenTasks             int
	OpenIncomingDocuments int
}

// Any reports whether the delete must be refused.
func (h OrgUnitHoldings) Any() bool {
	return h.Staff > 0 || h.ChildUnits > 0 || h.OpenPetitions > 0 || h.OpenTasks > 0 ||
		h.OpenIncomingDocuments > 0
}

// Sentence is the refusal a person reads: "Bộ phận còn 3 cán bộ, 1 bộ phận con — chuyển trước khi
// xoá." Only the kinds that are non-zero are named; the order is the order the person must work in
// (people and sub-units first, then the records).
func (h OrgUnitHoldings) Sentence() string {
	var parts []string
	add := func(n int, label string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, label))
		}
	}
	add(h.Staff, "cán bộ")
	add(h.ChildUnits, "bộ phận con")
	add(h.OpenPetitions, "phản ánh chưa xử lý xong")
	add(h.OpenTasks, "nhiệm vụ chưa hoàn thành")
	add(h.OpenIncomingDocuments, "văn bản đến chưa xử lý xong")
	if len(parts) == 0 {
		return ""
	}
	return "Bộ phận còn " + strings.Join(parts, ", ") + " — chuyển trước khi xoá."
}
