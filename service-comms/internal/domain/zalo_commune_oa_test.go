package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var oaClock = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

func TestZaloOALifetimesAreZaloDocumentedOnes(t *testing.T) {
	// ADR 0098 §Kết quả tra tài liệu #4, and #6 for the lead.
	if AccessTokenTTL != 25*time.Hour || RefreshTokenTTL != 90*24*time.Hour ||
		AccessRefreshLead != 2*time.Hour || AuthorizationTTL != 10*time.Minute {
		t.Fatalf("lifetimes drifted from ADR 0098: %v %v %v %v", AccessTokenTTL, RefreshTokenTTL, AccessRefreshLead, AuthorizationTTL)
	}
}

func TestCommuneZaloOATokenState(t *testing.T) {
	oa := CommuneZaloOA{OAID: "123", AccessExpiresAt: oaClock.Add(3 * time.Hour), RefreshIssuedAt: oaClock.Add(-time.Hour)}
	if oa.AccessExpired(oaClock) || oa.AccessRefreshDue(oaClock) || oa.RefreshTokenExpired(oaClock) || oa.NeedsReauthorization() {
		t.Fatalf("a fresh row is reported stale: %+v", oa)
	}
	if !oa.AccessRefreshDue(oaClock.Add(time.Hour + time.Minute)) {
		t.Error("less than 2 hours left is not due for refresh")
	}
	if !oa.AccessExpired(oaClock.Add(3 * time.Hour)) {
		t.Error("the expiry instant itself is still 'valid'")
	}
	if !oa.RefreshTokenExpired(oaClock.Add(RefreshTokenTTL - time.Hour)) {
		t.Error("a refresh token issued 90 days ago is still presented to Zalo")
	}
	oa.NeedsReauthorizationSince = oaClock
	if !oa.NeedsReauthorization() {
		t.Error("needs_reauthorization_since set, not reported")
	}
	if oa.Subject() != "zalo-oa/123" {
		t.Errorf("subject = %q", oa.Subject())
	}
}

func TestValidZaloOAIDIsDigitsOnly(t *testing.T) {
	for _, ok := range []string{"1", "1234567890", strings.Repeat("9", 32)} {
		if !ValidZaloOAID(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "12a", "-1", " 12", strings.Repeat("9", 33), "１２"} {
		if ValidZaloOAID(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestClipZaloOAName(t *testing.T) {
	if got := ClipZaloOAName("  UBND Xã A\t\n"); got != "UBND Xã A" {
		t.Errorf("= %q", got)
	}
	long := strings.Repeat("ệ", MaxZaloOANameRunes+3)
	if got := ClipZaloOAName(long); len([]rune(got)) != MaxZaloOANameRunes {
		t.Errorf("clipped to %d characters", len([]rune(got)))
	}
	if got := ClipZaloOAName("a\xffb"); got != "a�b" {
		t.Errorf("invalid UTF-8 kept: %q", got)
	}
}

func TestAuthorizationOpenAndOutcomeMapping(t *testing.T) {
	a := ZaloOAAuthorization{ID: "01J", ExpiresAt: oaClock.Add(AuthorizationTTL)}
	if !a.Open(oaClock) || a.Open(oaClock.Add(AuthorizationTTL)) {
		t.Fatal("open window wrong")
	}
	a.UsedAt = oaClock
	if a.Open(oaClock) {
		t.Fatal("a used request is still open")
	}
	if a.Subject() != "zalo-oa-authorization/01J" {
		t.Errorf("subject = %q", a.Subject())
	}
	cases := map[string]string{
		ZaloCallOK:                AuthorizationSucceeded,
		ZaloCallTokenRejected:     AuthorizationZaloRefused,
		ZaloCallRejected:          AuthorizationZaloRefused,
		ZaloCallRateLimited:       AuthorizationZaloUnavailable,
		ZaloCallUnavailable:       AuthorizationZaloUnavailable,
		ZaloCallMalformedResponse: AuthorizationMalformedResponse,
		"something-else":          AuthorizationMalformedResponse,
	}
	for in, want := range cases {
		if got := AuthorizationOutcomeOf(in); got != want {
			t.Errorf("%s → %s, want %s", in, got, want)
		}
	}
}

func TestMissingTemplateParamsIsByExactName(t *testing.T) {
	full := []TemplateParam{{Name: "ma_tra_cuu"}, {Name: "moc_nhan"}, {Name: "viec_tiep_theo"}, {Name: "extra"}}
	if m := MissingTemplateParams(full); len(m) != 0 {
		t.Fatalf("a full template is missing %v", m)
	}
	// ADR 0098 #9: no mapping — a different spelling is a missing name.
	spelled := []TemplateParam{{Name: "MaTraCuu"}, {Name: "moc-nhan"}, {Name: "viec_tiep_theo"}}
	if m := MissingTemplateParams(spelled); strings.Join(m, ",") != "ma_tra_cuu,moc_nhan" {
		t.Fatalf("missing = %v", m)
	}
	if m := MissingTemplateParams(nil); strings.Join(m, ",") != strings.Join(RequiredTemplateParams, ",") {
		t.Fatalf("missing of nothing = %v", m)
	}
}

func TestTemplateAttachableNeedsEnableAndMatch(t *testing.T) {
	tpl := ZaloOATemplate{TemplateID: "77", ZaloStatus: ZaloTemplateStatusEnabled, ParamsMatch: true}
	if !tpl.Attachable() || tpl.Attached() {
		t.Fatal("an ENABLE, matching, unattached template")
	}
	tpl.ZaloStatus = ZaloTemplateStatusRejected
	if tpl.Attachable() {
		t.Error("REJECT is attachable")
	}
	tpl.ZaloStatus, tpl.ParamsMatch = ZaloTemplateStatusEnabled, false
	if tpl.Attachable() {
		t.Error("a non-matching template is attachable")
	}
	tpl.Milestone = MilestoneReceived
	if !tpl.Attached() || tpl.Subject() != "zalo-oa-template/77" {
		t.Errorf("attached %v subject %q", tpl.Attached(), tpl.Subject())
	}
}

func TestMilestonesAre0031sEight(t *testing.T) {
	want := []string{"da-tiep-nhan", "da-chuyen-xu-ly", "cho-dan-xac-nhan", "da-dong", "khong-tiep-nhan",
		"chuyen-cap-tren", "gop-phieu", "tach-phieu"}
	if got := Milestones(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("milestones = %v", got)
	}
	for _, m := range want {
		if !ValidMilestone(m) {
			t.Errorf("%s refused", m)
		}
	}
	for _, bad := range []string{"", "da_tiep_nhan", "DA-TIEP-NHAN", "moi"} {
		if ValidMilestone(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestZaloTemplateShapes(t *testing.T) {
	for _, s := range []string{"ENABLE", "PENDING_REVIEW", "REJECT", "DISABLE", "DELETE"} {
		if !ValidZaloTemplateStatus(s) {
			t.Errorf("status %s refused", s)
		}
	}
	if ValidZaloTemplateStatus("enable") || ValidZaloTemplateStatus("") {
		t.Error("a status outside 0031's CHECK accepted")
	}
	if !ValidZaloTemplateQuality("HIGH") || ValidZaloTemplateQuality("GOOD") {
		t.Error("quality check wrong")
	}
	if !ValidZaloTemplateID("abc_DEF-123") || ValidZaloTemplateID("") || ValidZaloTemplateID("a b") ||
		ValidZaloTemplateID(strings.Repeat("x", 65)) {
		t.Error("template id shape wrong")
	}
}

func TestTemplateParamsMissingErrorNamesTheParams(t *testing.T) {
	err := &TemplateParamsMissingError{Missing: []string{"ma_tra_cuu", "moc_nhan"}}
	if !errors.Is(err, ErrCommuneZaloOAInvalid) {
		t.Fatal("not a request fault")
	}
	if !strings.Contains(err.Error(), "ma_tra_cuu, moc_nhan") {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestNormalizeCommuneZaloOARetireReason(t *testing.T) {
	if got, err := NormalizeCommuneZaloOARetireReason("  Đổi OA  "); err != nil || got != "Đổi OA" {
		t.Fatalf("= %q, %v", got, err)
	}
	for _, bad := range []string{"", "   ", "a\x00b", strings.Repeat("x", 201)} {
		if _, err := NormalizeCommuneZaloOARetireReason(bad); !errors.Is(err, ErrCommuneZaloOARetireReason) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestRevokeNoticeNamesOAManager(t *testing.T) {
	if !strings.Contains(CommuneZaloOARevokeNotice, "OA Manager") || !strings.Contains(CommuneZaloOARevokeNotice, "thu hồi") {
		t.Fatalf("notice = %q", CommuneZaloOARevokeNotice)
	}
	if len([]rune(CommuneZaloOAReplacedReason)) > 200 {
		t.Fatal("replaced reason exceeds 0031's CHECK")
	}
}
