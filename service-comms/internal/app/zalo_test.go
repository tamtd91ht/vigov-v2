package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/store/crosstenant"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// The Zalo channel's use cases over the recording fake driver (staff_notification_test.go): what is
// proved here is the transaction shape and the refusals. What PostgreSQL decides — the partial unique
// indexes, the guards, the CASE of the enqueue — is store/zalo_link_pg_test.go's.

// CheckNamedValue lets the fake take what pgx takes — a []string bound to text[] (`= ANY($2)`) — instead
// of database/sql's default converter refusing it before the statement is even recorded.
func (c *sqlFakeConn) CheckNamedValue(*driver.NamedValue) error { return nil }

// --- the bell's outbox ----------------------------------------------------------------------------------

func TestDeliver_QueuesZaloCopiesInTheSameTransaction(t *testing.T) {
	f := &sqlFake{exec: func(q string, _ []driver.Value) (int64, error) {
		if strings.Contains(q, "INSERT INTO staff_notification") {
			return insertTuples(q), nil
		}
		return 1, nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)
	db := sql.OpenDB(f)
	t.Cleanup(func() { db.Close() })
	uc.WithZaloOutbox(docstore.NewZaloLinkStore(store.New(db)), slog.New(slog.NewTextHandler(io.Discard, nil)))

	if _, err := uc.Deliver(ctx, twoNotices(), jobActor); err != nil {
		t.Fatal(err)
	}
	if f.begun != 1 || f.commits != 1 {
		t.Fatalf("begun %d commits %d — the outbox must share the bell's transaction", f.begun, f.commits)
	}
	enq := f.with("INSERT INTO zalo_delivery")
	if len(enq) != 1 || enq[0].args[0] != string(xaA) {
		t.Fatalf("enqueue statements = %v", enq)
	}
	if len(f.with("SAVEPOINT zalo_delivery_enqueue")) == 0 {
		t.Error("the enqueue is not inside a savepoint — its failure would take the bell down")
	}
}

func TestDeliver_ZaloEnqueueFailureStillDeliversTheBell(t *testing.T) {
	f := &sqlFake{exec: func(q string, _ []driver.Value) (int64, error) {
		switch {
		case strings.Contains(q, "INSERT INTO staff_notification"):
			return insertTuples(q), nil
		case strings.Contains(q, "INSERT INTO zalo_delivery"):
			return 0, errors.New("zalo table broken")
		}
		return 1, nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)
	db := sql.OpenDB(f)
	t.Cleanup(func() { db.Close() })
	uc.WithZaloOutbox(docstore.NewZaloLinkStore(store.New(db)), slog.New(slog.NewTextHandler(io.Discard, nil)))

	out, err := uc.Deliver(ctx, twoNotices(), jobActor)
	if err != nil {
		t.Fatalf("a Zalo failure failed the bell: %v", err)
	}
	if out[0].Created != 2 || f.commits != 1 || f.rollbacks != 0 {
		t.Fatalf("out %+v commits %d rollbacks %d", out, f.commits, f.rollbacks)
	}
	if len(f.with("ROLLBACK TO SAVEPOINT zalo_delivery_enqueue")) != 1 {
		t.Error("the failed enqueue was not rolled back to its savepoint")
	}
	aud := f.with("INSERT INTO audit_log")
	if len(aud) != 1 || !strings.Contains(string(aud[0].args[7].([]byte)), "zalo_loi_xep_hang") {
		t.Error("the trail does not say the Zalo copies were not queued")
	}
}

// --- the webhook ------------------------------------------------------------------------------------

type fakeResolver struct {
	matches []crosstenant.PairingCodeMatch
	err     error
	asked   int
}

func (r *fakeResolver) OpenPairingCodesByHash(context.Context, []byte) ([]crosstenant.PairingCodeMatch, error) {
	r.asked++
	return r.matches, r.err
}
func (r *fakeResolver) CommuneOfLiveChat(context.Context, string) (tenant.ID, bool, error) {
	return "", false, nil
}
func (r *fakeResolver) EndChatLinksInOtherCommunes(context.Context, *store.ScopedTx, string, time.Time) (int, error) {
	return 0, nil
}

type fakeBot struct{}

func (fakeBot) Bot(context.Context) (domain.SharedZaloBot, bool, error) {
	return domain.SharedZaloBot{}, false, nil
}
func (fakeBot) Token(context.Context) (secret.Secret, bool, error) { return nil, false, nil }
func (fakeBot) WebhookSecretMatches(context.Context, []byte) (bool, error) {
	return false, nil
}

type fakeSend struct{}

func (fakeSend) SendMessage(context.Context, secret.Secret, string, string) zalobot.Outcome {
	return zalobot.OutcomeOK
}

type fakeLimiter struct {
	allow bool
	err   error
}

func (l fakeLimiter) Allow(context.Context, ratelimit.Key) (bool, time.Duration, error) {
	return l.allow, time.Minute, l.err
}

// noCommuneBot answers zalo_commune_bot reads with no row — the commune uses the shared bot — and passes
// every other statement to the test's own answers.
func noCommuneBot(f *sqlFake) {
	inner := f.query
	f.query = func(q string, args []driver.Value) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "FROM zalo_commune_bot") {
			return communeBotCols(false), nil, nil
		}
		if inner == nil {
			return nil, nil, errors.New("fake: no answer")
		}
		return inner(q, args)
	}
}

func newWebhook(t *testing.T, f *sqlFake, r *fakeResolver, l fakeLimiter) *ZaloWebhook {
	t.Helper()
	noCommuneBot(f)
	db := sql.OpenDB(f)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	return NewZaloWebhook(kho, docstore.NewZaloLinkStore(kho), docstore.NewZaloCommuneBotStore(kho), r, l, fakeBot{},
		fakeSend{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func text(s string) zalobot.Update {
	return zalobot.Update{EventName: domain.ZaloEventTextReceived, ChatID: "chat-1", Text: s}
}

func TestWebhook_CommandsAndOtherText(t *testing.T) {
	f := &sqlFake{}
	r := &fakeResolver{}
	w := newWebhook(t, f, r, fakeLimiter{allow: true})
	ctx := context.Background()
	cases := map[string]string{
		"/trogiup": domain.ZaloReplyHelp, "/start": domain.ZaloReplyHelp, "xin chào": domain.ZaloReplyHelp,
		"/dung": domain.ZaloReplyNotLinked,
	}
	for in, want := range cases {
		if got := w.Handle(ctx, text(in), "1.2.3.4"); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
	if got := w.Handle(ctx, zalobot.Update{EventName: domain.ZaloEventUnsupportedReceived, ChatID: "c"}, ""); got != domain.ZaloReplyUnsupported {
		t.Errorf("unsupported → %q", got)
	}
	if got := w.Handle(ctx, zalobot.Update{EventName: "user.joined", ChatID: "c"}, ""); got != "" {
		t.Errorf("an unhandled event got a reply %q", got)
	}
	if r.asked != 0 || f.begun != 0 {
		t.Error("non-code text looked up a code or opened a transaction")
	}
}

func TestWebhook_PairingRefusalsAreOneSentenceAndTouchNothing(t *testing.T) {
	ctx := context.Background()
	two := []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: "c1"}, {TenantID: xaA, CodeID: "c2"}}
	for name, matches := range map[string][]crosstenant.PairingCodeMatch{"none": nil, "two communes": two} {
		f := &sqlFake{}
		w := newWebhook(t, f, &fakeResolver{matches: matches}, fakeLimiter{allow: true})
		if got := w.Handle(ctx, text("abcd-2345"), ""); got != domain.ZaloReplyPairingRefused {
			t.Errorf("%s → %q", name, got)
		}
		if f.begun != 0 {
			t.Errorf("%s: a transaction was opened for a code that matched no single commune", name)
		}
	}
}

func TestWebhook_PairingRateLimitedBeforeAnyLookupAndClosed(t *testing.T) {
	ctx := context.Background()
	r := &fakeResolver{matches: []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: "c1"}}}
	w := newWebhook(t, &sqlFake{}, r, fakeLimiter{allow: false})
	if got := w.Handle(ctx, text("ABCD2345"), ""); got != domain.ZaloReplyPairingLimited {
		t.Errorf("over the limit → %q", got)
	}
	w = newWebhook(t, &sqlFake{}, r, fakeLimiter{err: errors.New("redis down")})
	if got := w.Handle(ctx, text("ABCD2345"), ""); got != domain.ZaloReplyUnavailable {
		t.Errorf("limiter down → %q", got)
	}
	if r.asked != 0 {
		t.Error("a code was looked up past (or without) the per-chat limit")
	}
}

func TestWebhook_ExpiredCodeIsRefusedAndRolledBack(t *testing.T) {
	expired := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	f := &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "FROM zalo_pairing_code") {
			return []string{"id", "staff_code", "expires_at", "failed_attempts"},
				[][]driver.Value{{"c1", "CB-00123", expired, int64(0)}}, nil
		}
		return nil, nil, errors.New("unexpected")
	}}
	w := newWebhook(t, f, &fakeResolver{matches: []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: "c1"}}},
		fakeLimiter{allow: true})
	w.now = func() time.Time { return expired.Add(time.Second) }
	if got := w.Handle(context.Background(), text("ABCD2345"), ""); got != domain.ZaloReplyPairingRefused {
		t.Fatalf("expired → %q", got)
	}
	if f.rollbacks != 1 || f.commits != 0 || len(f.with("UPDATE zalo_pairing_code")) != 0 || len(f.with("INSERT INTO zalo_link")) != 0 {
		t.Errorf("an expired code wrote something: commits %d rollbacks %d", f.commits, f.rollbacks)
	}
}

func TestWebhook_ValidCodePairsInTheCodesCommuneWithItsEntry(t *testing.T) {
	future := time.Date(2026, 10, 5, 1, 10, 0, 0, time.UTC)
	f := &sqlFake{query: func(q string, args []driver.Value) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "FROM zalo_pairing_code"):
			return []string{"id", "staff_code", "expires_at", "failed_attempts"},
				[][]driver.Value{{"c1", "CB-00123", future, int64(0)}}, nil
		case strings.Contains(q, "FROM zalo_link"):
			return []string{"id", "staff_code", "linked_at"}, nil, nil
		}
		return nil, nil, errors.New("unexpected")
	}}
	w := newWebhook(t, f, &fakeResolver{matches: []crosstenant.PairingCodeMatch{{TenantID: xaB, CodeID: "c1"}}},
		fakeLimiter{allow: true})
	w.now = func() time.Time { return future.Add(-time.Minute) }
	w.newID = func() (string, error) { return "01JLINK0000000000000000001", nil }
	if got := w.Handle(context.Background(), text("abcd2345"), "203.0.113.9"); got != domain.ZaloReplyPaired {
		t.Fatalf("→ %q", got)
	}
	if f.commits != 1 {
		t.Fatalf("commits %d", f.commits)
	}
	ins := f.with("INSERT INTO zalo_link")
	if len(ins) != 1 || ins[0].args[0] != string(xaB) || ins[0].args[2] != "CB-00123" {
		t.Fatalf("link insert = %+v — must be in the CODE's commune, for the code's owner", ins)
	}
	aud := f.with("INSERT INTO audit_log")
	if len(aud) != 1 || aud[0].args[0] != string(xaB) || aud[0].args[1] != "CB-00123" || aud[0].args[2] != "staff" ||
		aud[0].args[4] != domain.ActionPairZaloLink {
		t.Fatalf("entry = %v", aud)
	}
	if strings.Contains(string(aud[0].args[7].([]byte)), "chat-1") {
		t.Error("the chat id reached the audit trail")
	}
}
