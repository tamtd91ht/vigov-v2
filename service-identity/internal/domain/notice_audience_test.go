package domain

import (
	"errors"
	"testing"
)

func TestCheckNoticeAudienceNormalisesAndRefuses(t *testing.T) {
	a, err := CheckNoticeAudience(NoticeAudience{Kind: NoticeAudienceCitizenTag, Tags: []string{"  hộ   nghèo ", "", "hộ nghèo", "cựu chiến binh"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Tags) != 2 || a.Tags[0] != "cựu chiến binh" || a.Tags[1] != "hộ nghèo" {
		t.Errorf("tags = %q, want whitespace collapsed, duplicates and blanks gone, sorted", a.Tags)
	}
	b, _ := CheckNoticeAudience(NoticeAudience{Kind: NoticeAudienceCitizenTag, Tags: []string{"hộ nghèo", "cựu chiến binh"}})
	if a.Fingerprint() != b.Fingerprint() {
		t.Error("one audience spelled two ways has two fingerprints")
	}
	c, _ := CheckNoticeAudience(NoticeAudience{Kind: NoticeAudienceCitizenTag, Tags: []string{"hộ nghèo"}})
	if a.Fingerprint() == c.Fingerprint() {
		t.Error("two audiences share a fingerprint")
	}

	for name, bad := range map[string]NoticeAudience{
		"unspecified":       {},
		"blank group ids":   {Kind: NoticeAudienceCitizenGroup, GroupIDs: []string{" "}},
		"unit without ids":  {Kind: NoticeAudienceStaffUnit},
		"staff all w/ tags": {Kind: NoticeAudienceStaffAll, Tags: []string{"x"}},
	} {
		if _, err := CheckNoticeAudience(bad); !errors.Is(err, ErrNoticeAudienceInvalid) {
			t.Errorf("%s: err = %v, want ErrNoticeAudienceInvalid", name, err)
		}
	}
}

func TestSelectNoticeRecipients(t *testing.T) {
	sel := SelectNoticeRecipients([]NoticeCandidate{
		{Ref: "B", Phone: "0900000000"},
		{Ref: "A", Phone: "+84900000000"}, // same number, smaller ref — keeps it
		{Ref: "C", Phone: "0900000001", Inactive: true},
		{Ref: "D", Phone: "abc"},
		{Ref: "E", Phone: ""},
		{Ref: "F", Phone: "0900000002"},
		{Ref: "F", Phone: "0900000002"}, // the same row twice — one recipient, not a skip
	})
	if len(sel.Recipients) != 2 || sel.Recipients[0].Ref != "A" || sel.Recipients[0].Phone != "0900000000" ||
		sel.Recipients[1].Ref != "F" {
		t.Errorf("recipients = %+v, want A (national form) then F", sel.Recipients)
	}
	if sel.SkippedInactive != 1 || sel.SkippedNoPhone != 2 {
		t.Errorf("skipped = %d inactive, %d no phone — want 1, 2", sel.SkippedInactive, sel.SkippedNoPhone)
	}
}
