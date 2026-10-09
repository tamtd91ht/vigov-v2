package migrations

import (
	"strings"
	"testing"
)

const file0025 = "0025_staff_notification_act_kinds.sql"

// actKinds are the nine phase-2 kinds 0025 admits (comms.proto StaffNotificationKind 18–25 and the
// announcement comms issues itself). internal/domain holds the same list as ZaloKind* constants.
var actKinds = []string{
	"'nhiem-vu.giao-moi'", "'nhiem-vu.de-nghi-lui-han'", "'nhiem-vu.cho-duyet'", "'nhiem-vu.nhac-ten'",
	"'van-ban.chuyen-toi'", "'phan-anh.phan-cong'", "'phan-anh.mo-lai'", "'bao-cao.san-sang'", "'thong-bao.moi'",
}

// THE MUTATIONS THAT MUST TURN THIS RED: one of the nine missing from any of the three CHECKs; a value the
// old CHECK admitted dropped (delivered notices and deliveries are immutable); the bell-only mention
// admitted on a Zalo table (ADR 0081 #5); an old constraint dropped before its replacement exists; any row
// written or rewritten.
func TestMigration0025ActKinds(t *testing.T) {
	sql := executableSQL(t, file0025)
	perDomain := []string{
		"'nhiem-vu.sap-den-han', 'nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang'",
		"'van-ban.sap-den-han', 'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang'",
		"'phan-anh.sap-den-han', 'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang'",
	}
	for _, c := range []struct {
		name, head, replaces string
		bellOnly             bool // must admit 'giai-ngan.nhac-ten'
	}{
		{"staff_notification_kind_with_act_notices", "check (kind in ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan',",
			"staff_notification_kind_with_bell_only", true},
		{"zalo_channel_setting_kinds_with_act_notices", "check (kinds <@ array['sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan',",
			"zalo_channel_setting_kinds_per_domain", false},
		{"zalo_delivery_kind_with_act_notices", "check (kind in ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan', 'thu-nghiem',",
			"zalo_delivery_kind_per_domain", false},
	} {
		add := strings.Index(sql, "add constraint "+c.name+" ")
		if add < 0 {
			t.Errorf("0025 does not add %s", c.name)
			continue
		}
		rest := sql[add:]
		if end := strings.Index(rest, "end if;"); end > 0 {
			rest = rest[:end]
		}
		if !strings.Contains(rest, c.head) {
			t.Errorf("%s does not keep the old values valid", c.name)
		}
		for _, want := range append(append([]string{}, perDomain...), actKinds...) {
			if !strings.Contains(rest, want) {
				t.Errorf("%s lacks %s", c.name, want)
			}
		}
		if got := strings.Contains(rest, "'giai-ngan.nhac-ten'"); got != c.bellOnly {
			t.Errorf("%s: admits the bell-only mention = %v, want %v (ADR 0081 #5)", c.name, got, c.bellOnly)
		}
		drop := strings.Index(sql, "drop constraint if exists "+c.replaces+";")
		if drop < 0 {
			t.Errorf("0025 does not drop %s", c.replaces)
		} else if drop < strings.LastIndex(sql, "add constraint") {
			t.Errorf("0025 drops %s before every new constraint is added", c.replaces)
		}
	}
	if n := strings.Count(sql, "add constraint"); n != 3 {
		t.Errorf("0025 adds %d constraints, want 3", n)
	}
	noDestruction(t, file0025, sql)
}
