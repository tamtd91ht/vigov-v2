package platformstore

// platformstore against REAL PostgreSQL and the REAL migrations (0018's guards, CHECKs and the deferred
// account-change trigger are what this file is about — no fake can make those assertions).
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET. A green `go test` without it means this file COMPILES, nothing
// more. The DSN carries a password and lives only in the environment (rule 8).
//
// ONE ORDERED SCENARIO, NOT INDEPENDENT TESTS: zalo_bot_shared is a single row by construction and no
// table here accepts a DELETE, so tests sharing a schema cannot reset it. The steps run in order on one
// fresh schema.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/migrations"
)

func openPG(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		t.Skip("VIGOV_TEST_DSN chưa đặt — bỏ qua test tích hợp")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// ONE connection: `SET search_path` is session state (the comms store pg tests' lesson).
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	schema := fmt.Sprintf("vigov_cm_platform_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	if _, err := migrate.Chay(ctx, db, migrations.FS, "comms"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// A schema this test created in a test database; it holds no archival data.
		_, _ = db.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		_ = db.Close()
	})
	return db
}

const (
	pgCommuneA = "01JTESTCOMMUNEAAAAAAAAAAAA"
	pgCommuneB = "01JTESTCOMMUNEBBBBBBBBBBBB"
)

var pgOp = Operator{Code: "VH-00001", IP: "10.0.0.5"}

func addLink(t *testing.T, db *sql.DB, tid, id, staff, chat string) {
	t.Helper()
	// bot_ref named explicitly: migration 0022 dropped its 'shared' default, so every writer says which bot.
	if _, err := db.Exec(`INSERT INTO zalo_link (tenant_id, id, staff_code, bot_ref, chat_id) VALUES ($1,$2,$3,'shared',$4)`,
		tid, id, staff, chat); err != nil {
		t.Fatalf("insert link: %v", err)
	}
}

const (
	pgCommuneC = "01JTESTCOMMUNECCCCCCCCCCCC"
	pgCommuneD = "01JTESTCOMMUNEDDDDDDDDDDDD"
	// ownBotTokenMarker sits inside the fake sealed token: CommuneStats must never surface it.
	ownBotTokenMarker = "OWN-BOT-FAKE-TOKEN"
)

type execFunc func(query string, args ...any) (sql.Result, error)

// addOwnBot inserts a LIVE zalo_commune_bot row through exec (the db, or a transaction).
func addOwnBot(t *testing.T, exec execFunc, tid, id, account, name string) {
	t.Helper()
	sealed := []byte(ownBotTokenMarker + "-padding-past-29-bytes")
	if _, err := exec(`INSERT INTO zalo_commune_bot (tenant_id, id, bot_account_id, token_sealed, bot_name, set_at, set_by)
		VALUES ($1,$2,$3,$4,$5,now(),'CB-001')`, tid, id, account, sealed, name); err != nil {
		t.Fatalf("insert own bot: %v", err)
	}
}

// addOwnLink inserts a live link to the commune's own bot (bot_ref = 'commune:' || account).
func addOwnLink(t *testing.T, exec execFunc, tid, id, staff, account, chat string) {
	t.Helper()
	if _, err := exec(`INSERT INTO zalo_link (tenant_id, id, staff_code, bot_ref, chat_id) VALUES ($1,$2,$3,$4,$5)`,
		tid, id, staff, domain.CommuneZaloBotRefPrefix+account, chat); err != nil {
		t.Fatalf("insert own-bot link: %v", err)
	}
}

func count(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func TestPlatformStoreAgainstPostgres(t *testing.T) {
	db := openPG(t)
	s := New(db)
	ctx := context.Background()
	kek := secret.Secret("kek-P-FAKE-NOT-A-REAL-KEY-32byt!")

	// 1. The platform DEK: refused without an operator, created and audited with one, never twice.
	env, err := crypto.NewPlatform([]secret.Secret{kek}, s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.SealPlatform(ctx, secret.Secret("x"), []byte("aad")); !errors.Is(err, ErrNoOperator) {
		t.Fatalf("a first seal without an operator: %v", err)
	}
	sealed, err := env.SealPlatform(WithOperator(ctx, pgOp), secret.Secret("123456789:FAKE-NOT-REAL"), []byte("zalo_bot_shared/token_sealed/shared"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if n := count(t, db, `SELECT count(*) FROM platform_operator_audit_log WHERE action = $1`, domain.ActionPlatformDataKeyCreated); n != 1 {
		t.Fatalf("DEK creation entries = %d", n)
	}
	if err := s.CreatePlatformDEK(WithOperator(ctx, pgOp), crypto.WrappedDEK{KEKID: "00000000", Wrapped: make([]byte, 65)}); !errors.Is(err, crypto.ErrPlatformDEKExists) {
		t.Fatalf("second DEK: %v", err)
	}
	if got, err := env.OpenPlatform(ctx, sealed, []byte("zalo_bot_shared/token_sealed/shared")); err != nil || string(got) != "123456789:FAKE-NOT-REAL" {
		t.Fatalf("open through the real table: %v", err)
	}

	// 2. A write without its trail entry never commits.
	err = s.InTx(ctx, func(tx *Tx) error {
		_, err := tx.SaveToken(ctx, SavedToken{BotAccountID: "bot-1", BotName: "Bot", ChatURL: "https://zalo.me/1", SetBy: pgOp.Code, Sealed: sealed})
		return err
	})
	if !errors.Is(err, ErrWriteWithoutTrail) {
		t.Fatalf("unaudited write: %v", err)
	}
	if _, found, _ := s.SharedBot(ctx); found {
		t.Fatal("an unaudited write committed")
	}

	// 3. The first save, audited.
	save := func(account string) error {
		return s.InTx(ctx, func(tx *Tx) error {
			if _, _, _, err := tx.LockSharedBot(ctx); err != nil {
				return err
			}
			if _, err := tx.SaveToken(ctx, SavedToken{BotAccountID: account, BotName: "Bot", ChatURL: "https://zalo.me/1",
				SetBy: pgOp.Code, Sealed: sealed}); err != nil {
				return err
			}
			return tx.AppendOperatorAudit(ctx, OperatorAudit{Operator: pgOp, Action: domain.ActionZaloBotTokenSet,
				Target: domain.ZaloBotTarget, After: map[string]string{"bot_account_id": account}})
		})
	}
	if err := save("bot-1"); err != nil {
		t.Fatal(err)
	}
	b, found, err := s.SharedBot(ctx)
	if err != nil || !found || b.BotAccountID != "bot-1" || b.LastCheckResult != domain.ZaloCallOK || b.SetBy != pgOp.Code {
		t.Fatalf("= %+v %v %v", b, found, err)
	}

	// 4. Links in two communes; a change of account WITHOUT ending them is refused AT COMMIT by 0018's
	// deferred trigger.
	addLink(t, db, pgCommuneA, "01JLINKA1", "CB-001", "chat-a1")
	addLink(t, db, pgCommuneA, "01JLINKA2", "CB-002", "chat-a2")
	addLink(t, db, pgCommuneB, "01JLINKB1", "CB-001", "chat-b1")
	if err := save("bot-2"); err == nil {
		t.Fatal("a change of bot account left links live and committed")
	}
	if b, _, _ := s.SharedBot(ctx); b.BotAccountID != "bot-1" {
		t.Fatal("the refused change was partly written")
	}

	// 5. Counted and ended in the same transaction: commits; each link audited in ITS commune.
	err = s.InTx(ctx, func(tx *Tx) error {
		if _, _, _, err := tx.LockSharedBot(ctx); err != nil {
			return err
		}
		n, err := tx.CountLiveLinks(ctx)
		if err != nil || n != 3 {
			return fmt.Errorf("count = %d, %v", n, err)
		}
		by, total, err := tx.EndAllLiveLinks(ctx, pgOp)
		if err != nil || total != 3 || by[pgCommuneA] != 2 || by[pgCommuneB] != 1 {
			return fmt.Errorf("ended = %v %d %v", by, total, err)
		}
		if _, err := tx.SaveToken(ctx, SavedToken{BotAccountID: "bot-2", BotName: "Bot 2", ChatURL: "https://zalo.me/2",
			SetBy: pgOp.Code, Sealed: sealed}); err != nil {
			return err
		}
		return tx.AppendOperatorAudit(ctx, OperatorAudit{Operator: pgOp, Action: domain.ActionZaloBotTokenSet,
			Target: domain.ZaloBotTarget, After: map[string]any{"ended_link_count": total}})
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT count(*) FROM zalo_link WHERE unlinked_at IS NULL`); n != 0 {
		t.Fatalf("%d links still live", n)
	}
	if n := count(t, db, `SELECT count(*) FROM zalo_link WHERE unlinked_by = $1 AND unlink_reason = $2`, pgOp.Code, domain.ZaloLinkEndedByBotChange); n != 3 {
		t.Fatalf("%d links ended by the operator", n)
	}
	for tid, want := range map[string]int{pgCommuneA: 2, pgCommuneB: 1} {
		if n := count(t, db, `SELECT count(*) FROM audit_log WHERE tenant_id = $1 AND action = $2 AND actor_id = $3 AND actor_kind = 'operator'`,
			tid, domain.ActionEndZaloLink, pgOp.Code); n != want {
			t.Errorf("commune %s: %d link-end entries, want %d", tid, n, want)
		}
	}

	// 6. The check is recorded only against the token it checked.
	err = s.InTx(ctx, func(tx *Tx) error {
		if _, err := tx.RecordCheck(ctx, []byte("not the stored token"), domain.ZaloCallRateLimited); !errors.Is(err, ErrChanged) {
			return fmt.Errorf("CAS on another token: %v", err)
		}
		if _, err := tx.RecordCheck(ctx, sealed, domain.ZaloCallRateLimited); err != nil {
			return err
		}
		return tx.AppendOperatorAudit(ctx, OperatorAudit{Operator: pgOp, Action: domain.ActionZaloBotChecked, Target: domain.ZaloBotTarget})
	})
	if err != nil {
		t.Fatal(err)
	}
	if b, _, _ := s.SharedBot(ctx); b.LastCheckResult != domain.ZaloCallRateLimited {
		t.Fatalf("check not recorded: %+v", b)
	}

	// 7. Pending, then promote (compare-and-swap), then a pending cleared.
	pending := bytes.Repeat([]byte{1}, 40)
	auditOnly := func(tx *Tx) error {
		return tx.AppendOperatorAudit(ctx, OperatorAudit{Operator: pgOp, Action: domain.ActionZaloBotWebhookRequested, Target: domain.ZaloBotTarget})
	}
	if err := s.InTx(ctx, func(tx *Tx) error {
		if _, err := tx.SetPendingWebhookSecret(ctx, pending); err != nil {
			return err
		}
		return auditOnly(tx)
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.PendingWebhookSecretRead(ctx); !bytes.Equal(got, pending) {
		t.Fatal("pending not stored")
	}
	if err := s.InTx(ctx, func(tx *Tx) error {
		if _, err := tx.PromotePendingWebhookSecret(ctx, bytes.Repeat([]byte{2}, 40), pgOp.Code); !errors.Is(err, ErrChanged) {
			return fmt.Errorf("promote of another secret: %v", err)
		}
		if _, err := tx.PromotePendingWebhookSecret(ctx, pending, pgOp.Code); err != nil {
			return err
		}
		return auditOnly(tx)
	}); err != nil {
		t.Fatal(err)
	}
	if b, _, _ := s.SharedBot(ctx); b.WebhookSetBy != pgOp.Code || b.HasPendingWebhook || b.WebhookSetAt.IsZero() {
		t.Fatalf("not promoted: %+v", b)
	}
	pending2 := bytes.Repeat([]byte{3}, 40)
	if err := s.InTx(ctx, func(tx *Tx) error {
		if _, err := tx.SetPendingWebhookSecret(ctx, pending2); err != nil {
			return err
		}
		if err := tx.ClearPendingWebhookSecret(ctx, pending2); err != nil {
			return err
		}
		return auditOnly(tx)
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.PendingWebhookSecretRead(ctx); got != nil {
		t.Fatal("pending not cleared")
	}

	// 8. The cross-commune read: counts and a switch.
	if _, err := db.Exec(`INSERT INTO zalo_channel_setting (tenant_id, is_enabled, kinds, updated_by)
		VALUES ($1, true, ARRAY['sap-den-han'], 'CB-001')`, pgCommuneB); err != nil {
		t.Fatal(err)
	}
	addLink(t, db, pgCommuneA, "01JLINKA3", "CB-003", "chat-a3")
	var stats []domain.ZaloBotCommuneStat
	if err := s.InTx(ctx, func(tx *Tx) error {
		var err error
		if stats, err = tx.CommuneStats(ctx); err != nil {
			return err
		}
		return tx.AppendOperatorAudit(ctx, OperatorAudit{Operator: pgOp, Action: domain.ActionZaloBotCommunesRead, Target: domain.ZaloBotTarget})
	}); err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 || stats[0].TenantID != pgCommuneA || stats[0].LinkedStaffCount != 1 || stats[0].ChannelEnabled ||
		stats[1].TenantID != pgCommuneB || !stats[1].ChannelEnabled || stats[1].LinkedStaffCount != 0 {
		t.Fatalf("stats = %+v", stats)
	}

	// 8b. Own bots (ADR 0079): a commune with only an own bot is listed; shared and own links counted
	// apart; a live link to a RETIRED own bot is never counted; nothing of the bot leaves but its name.
	addLink(t, db, pgCommuneB, "01JLINKB2", "CB-004", "chat-b2")
	addOwnBot(t, db.Exec, pgCommuneC, "01JBOTC", "cbot-c", "Bot Xã C")
	addOwnBot(t, db.Exec, pgCommuneD, "01JBOTX", "cbot-x", "Bot X")
	addOwnLink(t, db.Exec, pgCommuneD, "01JLINKD1", "CB-010", "cbot-x", "chat-d1")
	readStats := func(tx *Tx) ([]domain.ZaloBotCommuneStat, error) {
		st, err := tx.CommuneStats(ctx)
		if err != nil {
			return nil, err
		}
		return st, tx.AppendOperatorAudit(ctx, OperatorAudit{Operator: pgOp, Action: domain.ActionZaloBotCommunesRead, Target: domain.ZaloBotTarget})
	}
	if err := s.InTx(ctx, func(tx *Tx) error {
		var err error
		stats, err = readStats(tx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	want := []domain.ZaloBotCommuneStat{
		{TenantID: pgCommuneA, LinkedStaffCount: 1},
		{TenantID: pgCommuneB, ChannelEnabled: true, LinkedStaffCount: 1},
		{TenantID: pgCommuneC, UsesOwnBot: true, OwnBotName: "Bot Xã C"},
		{TenantID: pgCommuneD, UsesOwnBot: true, OwnBotName: "Bot X", OwnBotLinkedStaffCount: 1},
	}
	if fmt.Sprint(stats) != fmt.Sprint(want) {
		t.Fatalf("stats = %+v\nwant    %+v", stats, want)
	}
	for _, st := range stats {
		if strings.Contains(st.OwnBotName, "cbot-") || strings.Contains(st.OwnBotName, ownBotTokenMarker) {
			t.Fatalf("own_bot_name carries the account id or the token: %+v", st)
		}
	}
	// Retire X while its link is still live — a state 0022's deferred retire check refuses AT COMMIT, so
	// it exists only inside this transaction, which is rolled back. Read here, the count must already
	// ignore it: only the link to the CURRENT bot Y counts.
	errRollback := errors.New("rollback")
	err = s.InTx(ctx, func(tx *Tx) error {
		if _, err := tx.tx.ExecContext(ctx, `UPDATE zalo_commune_bot SET retired_at = now(), retired_by = 'CB-001',
			retire_reason = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
			pgCommuneD, "01JBOTX", domain.CommuneZaloBotReplacedReason); err != nil {
			return err
		}
		addOwnBot(t, tx.tx.Exec, pgCommuneD, "01JBOTY", "cbot-y", "Bot Y")
		addOwnLink(t, tx.tx.Exec, pgCommuneD, "01JLINKD2", "CB-011", "cbot-y", "chat-d2")
		st, err := readStats(tx)
		if err != nil {
			return err
		}
		d := st[len(st)-1]
		if d.TenantID != pgCommuneD || !d.UsesOwnBot || d.OwnBotName != "Bot Y" || d.OwnBotLinkedStaffCount != 1 || d.LinkedStaffCount != 0 {
			return fmt.Errorf("after the switch, D = %+v", d)
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}

	// 9.The trail refuses anybody but an operator, and no entry ever holds a sealed value.
	if err := s.InTx(ctx, func(tx *Tx) error {
		return tx.AppendOperatorAudit(ctx, OperatorAudit{Operator: Operator{Code: "system", IP: "x"}, Action: "a", Target: "t"})
	}); !errors.Is(err, ErrNoOperator) {
		t.Fatalf("a non-operator entry: %v", err)
	}
	var leaked int
	if err := db.QueryRow(`SELECT count(*) FROM platform_operator_audit_log
		WHERE coalesce(before::text,'') LIKE '%FAKE-NOT-REAL%' OR coalesce(after::text,'') LIKE '%FAKE-NOT-REAL%'`).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("token in the trail: %d %v", leaked, err)
	}
}
