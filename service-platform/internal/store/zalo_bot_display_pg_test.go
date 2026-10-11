package store

import (
	"context"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-platform/internal/domain"
)

// Against a real PostgreSQL — skipped without VIGOV_TEST_DSN. What only the database can show: the row
// satisfies migration 0008's CHECKs (actor/action/subject not blank, after present) with a NULL
// reason, and lands with the operator's business code and address.
func TestPgRecordZaloBotAct(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	l := NewZaloBotDisplayLog(db)
	ctx := context.Background()
	const q = `SELECT count(*) FROM platform_audit_log WHERE subject = 'zalo-bot-shared' AND action = $1`
	n := count(t, db, q, domain.ZaloBotActionTokenSet)

	if err := l.RecordZaloBotAct(ctx, domain.ZaloBotAct{Action: domain.ZaloBotActionTokenSet, Outcome: "thanh-cong",
		BotName: "Bot nhắc việc", ChatURL: "https://zalo.me/bot-FAKE", EndedLinkCount: 2}, operatorFake); err != nil {
		t.Fatal(err)
	}
	if count(t, db, q, domain.ZaloBotActionTokenSet) != n+1 {
		t.Fatal("no row written")
	}
	var actor, ip, after string
	var reasonNull bool
	if err := db.QueryRow(`SELECT actor, actor_ip, after::text, reason IS NULL FROM platform_audit_log
		WHERE action = $1 ORDER BY id DESC LIMIT 1`, domain.ZaloBotActionTokenSet).Scan(&actor, &ip, &after, &reasonNull); err != nil {
		t.Fatal(err)
	}
	if actor != operatorFake.Code || ip != operatorFake.IP || !reasonNull || !strings.Contains(after, `"ended_link_count": 2`) {
		t.Errorf("row actor=%q ip=%q reasonNull=%v after=%s", actor, ip, reasonNull, after)
	}
}
