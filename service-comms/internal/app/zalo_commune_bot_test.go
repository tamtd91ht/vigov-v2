package app

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/store/crosstenant"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// The commune's own bot (0022) over the recording fake driver, the REAL stores and the REAL commune
// envelope: what is proved is the transaction shape, what is sealed with which binding, what is ended
// and audited, and what never leaves. What PostgreSQL decides — the unique keys, the guard, the deferred
// checks — is 0022's and its migration test's.

var botClock = time.Date(2026, 10, 8, 2, 0, 0, 0, time.UTC)

const (
	botRowID     = "01JBOTROW000000000000000001"
	botRowNewID  = "01JBOTROW000000000000000002"
	communeHostA = "xa-a.example.gov.vn"
)

// communeBotCols mirrors store.communeBotColumns (+ the sealed three).
func communeBotCols(sealed bool) []string {
	c := []string{"id", "bot_account_id", "bot_name", "chat_url", "set_at", "set_by", "last_check_at",
		"last_check_result", "webhook_set_at", "webhook_set_by", "pending"}
	if sealed {
		c = append(c, "token_sealed", "webhook_secret_sealed", "webhook_secret_pending_sealed")
	}
	return c
}

// liveBot is the fake's one live zalo_commune_bot row. nil = the commune uses the shared bot.
type liveBot struct {
	id, account      string
	token            []byte
	webhook, pending []byte
}

func (b *liveBot) values(sealed bool) []driver.Value {
	v := []driver.Value{b.id, b.account, "Bot Xã A", "https://zalo.me/123", botClock, "CB-00099", nil, nil, nil, nil,
		b.pending != nil}
	if sealed {
		v = append(v, b.token, nilIfEmpty(b.webhook), nilIfEmpty(b.pending))
	}
	return v
}

func nilIfEmpty(b []byte) driver.Value {
	if b == nil {
		return nil
	}
	return b
}

type botRig struct {
	f      *sqlFake
	env    *crypto.Envelope
	zalo   *fakeZalo
	uc     *CommuneZaloBots
	router *ZaloBotRouter
	ctx    context.Context
	bot    *liveBot
	// endedLinks is what EndLiveLinksOfBot's RETURNING hands back.
	endedLinks [][]driver.Value
	// pairingCodeID answers the commune's by-hash lookup ("" = none).
	pairingCodeID string
	host          string
	linkCount     int
	// logs is the security log the use case writes (JSON lines).
	logs *bytes.Buffer
}

// rigRegistry reads the rig's host at call time, so a test may blank it.
type rigRegistry struct{ r *botRig }

func (g rigRegistry) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) { // vi-name-ok: the method of the existing PortalCommuneRegistry interface
	if g.r.host == "" {
		return tenant.Tenant{}, false, nil
	}
	return tenant.Tenant{ID: tenant.MustFrom(ctx), Host: g.r.host, Active: true}, true, nil
}

func newBotRig(t *testing.T) *botRig {
	t.Helper()
	r := &botRig{env: testEnvelope(t), zalo: newFakeZalo(), ctx: tenant.Into(context.Background(), xaA),
		host: communeHostA}
	r.f = &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "FROM zalo_commune_bot"):
			sealed := strings.Contains(q, "token_sealed")
			if r.bot == nil {
				return communeBotCols(sealed), nil, nil
			}
			return communeBotCols(sealed), [][]driver.Value{r.bot.values(sealed)}, nil
		case strings.Contains(q, "UPDATE zalo_link SET unlinked_at"):
			return []string{"id", "staff_code", "linked_at", "bot_ref"}, r.endedLinks, nil
		case strings.Contains(q, "count(*)") && strings.Contains(q, "FROM zalo_link"):
			return []string{"count"}, [][]driver.Value{{int64(r.linkCount)}}, nil
		case strings.Contains(q, "FROM zalo_link"):
			return []string{"id", "staff_code", "linked_at", "bot_ref"}, nil, nil
		case strings.Contains(q, "SELECT id FROM zalo_pairing_code"):
			if r.pairingCodeID == "" {
				return []string{"id"}, nil, nil
			}
			return []string{"id"}, [][]driver.Value{{r.pairingCodeID}}, nil
		case strings.Contains(q, "FROM zalo_pairing_code"):
			return []string{"id", "staff_code", "expires_at", "failed_attempts"},
				[][]driver.Value{{r.pairingCodeID, "CB-00123", botClock.Add(5 * time.Minute), int64(0)}}, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	db := sql.OpenDB(r.f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	r.logs = &bytes.Buffer{}
	r.uc = NewCommuneZaloBots(kho, docstore.NewZaloCommuneBotStore(kho), docstore.NewZaloLinkStore(kho), r.env, r.zalo,
		rigRegistry{r}, slog.New(slog.NewJSONHandler(r.logs, nil)))
	r.uc.newID = func() (string, error) { return botRowNewID, nil }
	r.uc.now = func() time.Time { return botClock }
	r.router = NewZaloBotRouter(fakeBot{}, docstore.NewZaloCommuneBotStore(kho), r.env)
	return r
}

// sealFor seals plain under a commune bot binding of row id in commune A.
func (r *botRig) sealFor(t *testing.T, plain, id string, aad func(context.Context, string) []byte) []byte {
	t.Helper()
	s, err := r.env.Seal(r.ctx, secret.Secret(plain), aad(r.ctx, id))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (r *botRig) withLiveBot(t *testing.T, account string) {
	t.Helper()
	r.bot = &liveBot{id: botRowID, account: account, token: r.sealFor(t, zbToken, botRowID, communeBotTokenAAD)}
}

func setInput(token string) SetCommuneZaloBotInput {
	return SetCommuneZaloBotInput{Token: secret.Secret(token), BotName: "Bot Xã A", ChatURL: "https://zalo.me/123"}
}

func entryActions(f *sqlFake) []string {
	var out []string
	for _, s := range f.with("INSERT INTO audit_log") {
		out = append(out, fmt.Sprint(s.args[4]))
	}
	return out
}

func indexOfStmt(f *sqlFake, sub string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, s := range f.stmts {
		if strings.Contains(s.sql, sub) {
			return i
		}
	}
	return -1
}

// --- Set ----------------------------------------------------------------------------------------------

func TestCommuneBotAdoptSealsEndsSharedLinksAndAuditsInOneTransaction(t *testing.T) {
	r := newBotRig(t)
	r.endedLinks = [][]driver.Value{{"01JLINKSHARED0000000000001", "CB-00007", botClock.Add(-time.Hour), "shared"}}

	out, err := r.uc.Set(r.ctx, setInput(zbToken), staffActor)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if r.f.begun != 1 || r.f.commits != 1 || r.f.rollbacks != 0 {
		t.Fatalf("tx %d/%d/%d, want one committed", r.f.begun, r.f.commits, r.f.rollbacks)
	}
	if !out.Adopted || out.EndedLinkCount != 1 || out.RetiredPrevious || out.Bot.BotAccountID != "bot-111" {
		t.Fatalf("out = %+v", out)
	}
	// Every live SHARED link ended in this commune (0022 adopt check).
	end := r.f.with("UPDATE zalo_link SET unlinked_at")
	if len(end) != 1 || end[0].args[0] != string(xaA) || end[0].args[1] != domain.SharedZaloBotRef || end[0].args[3] != "CB-00123" {
		t.Fatalf("link end = %v", end)
	}
	ins := r.f.with("INSERT INTO zalo_commune_bot")
	if len(ins) != 1 {
		t.Fatalf("%d inserts", len(ins))
	}
	a := ins[0].args
	// bot_account_id is what getMe said — never typed.
	if a[0] != string(xaA) || a[1] != botRowNewID || a[2] != "bot-111" || a[7] != "CB-00123" {
		t.Fatalf("insert args = %v", a)
	}
	sealed, _ := a[3].([]byte)
	if len(sealed) == 0 || bytes.Contains(sealed, []byte(zbToken)) {
		t.Fatal("the token column is not sealed")
	}
	// Opens with THIS commune + THIS row's binding, and with nothing else.
	if p, err := r.env.Open(r.ctx, sealed, communeBotTokenAAD(r.ctx, botRowNewID)); err != nil || string(p.Lo()) != zbToken {
		t.Fatalf("sealed token does not open back: %v", err)
	}
	if _, err := r.env.Open(r.ctx, sealed, communeBotTokenAAD(r.ctx, botRowID)); err == nil {
		t.Error("the token opened under another row's binding")
	}
	ctxB := tenant.Into(context.Background(), xaB)
	if _, err := r.env.Open(ctxB, sealed, communeBotTokenAAD(ctxB, botRowNewID)); err == nil {
		t.Error("commune A's token opened for commune B")
	}
	if got := strings.Join(entryActions(r.f), ","); got != domain.ActionEndZaloLink+","+domain.ActionSetCommuneZaloBot {
		t.Fatalf("entries = %s", got)
	}
	for _, e := range r.f.with("INSERT INTO audit_log") {
		d := string(e.args[7].([]byte))
		if strings.Contains(d, zbToken) || strings.Contains(d, "token_sealed") {
			t.Fatalf("an entry carries the token: %s", d)
		}
		if e.args[1] != "CB-00123" {
			t.Errorf("actor = %v, want the business code", e.args[1])
		}
	}
}

func TestCommuneBotNewTokenForTheSameAccountUpdatesInPlace(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	out, err := r.uc.Set(r.ctx, setInput(zbToken), staffActor)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if out.Adopted || out.EndedLinkCount != 0 {
		t.Fatalf("out = %+v — the same bot is not a switch", out)
	}
	if len(r.f.with("INSERT INTO zalo_commune_bot")) != 0 || len(r.f.with("UPDATE zalo_link")) != 0 {
		t.Fatal("a new token for the same bot inserted a row or ended links")
	}
	up := r.f.with("UPDATE zalo_commune_bot SET token_sealed")
	if len(up) != 1 || up[0].args[1] != botRowID {
		t.Fatalf("update = %v", up)
	}
	sealed, _ := up[0].args[2].([]byte)
	if _, err := r.env.Open(r.ctx, sealed, communeBotTokenAAD(r.ctx, botRowID)); err != nil {
		t.Fatal("the new token is not sealed to the SAME row")
	}
}

func TestCommuneBotAnotherAccountRetiresTheOldEndsItsLinksAndAdopts(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-222")
	r.endedLinks = [][]driver.Value{{"01JLINKOWN0000000000000001", "CB-00008", botClock.Add(-time.Hour), "commune:bot-222"}}
	out, err := r.uc.Set(r.ctx, setInput(zbToken), staffActor)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !out.Adopted || !out.RetiredPrevious || out.EndedLinkCount != 1 {
		t.Fatalf("out = %+v", out)
	}
	ret := r.f.with("SET retired_at")
	if len(ret) != 1 || ret[0].args[1] != botRowID || ret[0].args[4] != domain.CommuneZaloBotReplacedReason {
		t.Fatalf("retire = %v", ret)
	}
	if end := r.f.with("UPDATE zalo_link SET unlinked_at"); len(end) != 1 || end[0].args[1] != "commune:bot-222" {
		t.Fatalf("the retired bot's links were not ended: %v", end)
	}
	// Retire BEFORE insert: 0022's one-live-per-commune index is checked immediately.
	if iR, iI := indexOfStmt(r.f, "SET retired_at"), indexOfStmt(r.f, "INSERT INTO zalo_commune_bot"); iR > iI {
		t.Errorf("retire at %d after insert at %d", iR, iI)
	}
}

func TestCommuneBotRefusalsWriteNothing(t *testing.T) {
	cases := map[string]struct {
		in    SetCommuneZaloBotInput
		zalo  zalobot.Outcome
		noEnv bool
		want  func(error) bool
	}{
		"name without Bot": {in: SetCommuneZaloBotInput{Token: secret.Secret(zbToken), BotName: "Xã A", ChatURL: "https://zalo.me/1"},
			want: func(e error) bool { return errors.Is(e, domain.ErrCommuneZaloBotName) }},
		"http chat url": {in: SetCommuneZaloBotInput{Token: secret.Secret(zbToken), BotName: "Bot A", ChatURL: "http://zalo.me/1"},
			want: func(e error) bool { return errors.Is(e, domain.ErrCommuneZaloBotChatURL) }},
		"token with a slash": {in: setInput("abc/def"),
			want: func(e error) bool { return errors.Is(e, domain.ErrCommuneZaloBotToken) }},
		"no token, no bot": {in: setInput(""),
			want: func(e error) bool { return errors.Is(e, domain.ErrCommuneZaloBotTokenRequired) }},
		"zalo refuses": {in: setInput("777:unknown-token"),
			want: func(e error) bool {
				var tc *ZaloTokenCheckError
				return errors.As(e, &tc) && tc.Class == domain.ZaloCallTokenRejected
			}},
		"zalo down": {in: setInput(zbToken), zalo: zalobot.OutcomeUnavailable,
			want: func(e error) bool {
				var tc *ZaloTokenCheckError
				return errors.As(e, &tc) && tc.Class == domain.ZaloCallUnavailable
			}},
		"no KEK": {in: setInput(zbToken), noEnv: true,
			want: func(e error) bool { return errors.Is(e, crypto.ErrNotConfigured) }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := newBotRig(t)
			r.zalo.getMeOutcome = c.zalo
			if c.noEnv {
				r.uc.envelope = nil
			}
			_, err := r.uc.Set(r.ctx, c.in, staffActor)
			if !c.want(err) {
				t.Fatalf("err = %v", err)
			}
			if r.f.begun != 0 || len(r.f.with("INSERT")) != 0 || len(r.f.with("UPDATE")) != 0 {
				t.Fatalf("a refusal wrote: begun %d", r.f.begun)
			}
		})
	}
}

func TestCommuneBotAccountLiveElsewhereIsOneGenericRefusal(t *testing.T) {
	for _, code := range []string{"23505", "P0001"} {
		r := newBotRig(t)
		r.f.exec = func(q string, _ []driver.Value) (int64, error) {
			if strings.Contains(q, "INSERT INTO zalo_commune_bot") {
				return 0, &pgconn.PgError{Code: code, Message: "zalo_commune_bot: another commune's details"}
			}
			return 1, nil
		}
		_, err := r.uc.Set(r.ctx, setInput(zbToken), staffActor)
		if !errors.Is(err, docstore.ErrCommuneZaloBotAccountTaken) {
			t.Fatalf("%s → %v", code, err)
		}
		if strings.Contains(err.Error(), "another commune") || r.f.commits != 0 || r.f.rollbacks != 1 {
			t.Fatalf("%s: err %q commits %d rollbacks %d", code, err, r.f.commits, r.f.rollbacks)
		}
	}
}

func TestCommuneBotNameOnlyEditNeedsNoToken(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	in := setInput("")
	in.BotName = "Bot Xã A mới"
	out, err := r.uc.Set(r.ctx, in, staffActor)
	if err != nil || out.Bot.BotName != "Bot Xã A mới" {
		t.Fatalf("out %+v err %v", out, err)
	}
	if r.zalo.calls != 0 {
		t.Error("a name-only edit called Zalo")
	}
	if up := r.f.with("UPDATE zalo_commune_bot SET bot_name"); len(up) != 1 {
		t.Fatalf("update = %v", up)
	}
	if got := entryActions(r.f); len(got) != 1 || got[0] != domain.ActionSetCommuneZaloBot {
		t.Fatalf("entries = %v", got)
	}
}

// --- webhook --------------------------------------------------------------------------------------

func TestCommuneBotWebhookStoresPendingFirstThenPromotesAndShowsTheSecretOnce(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	out, err := r.uc.RegisterWebhook(r.ctx, staffActor)
	if err != nil {
		t.Fatalf("RegisterWebhook: %v", err)
	}
	if out.Outcome != domain.ZaloCallOK || out.URL != "https://"+communeHostA+"/api/v1/zalo-bot-updates" || out.SetBy != "CB-00123" {
		t.Fatalf("out = %+v", out)
	}
	if len(r.zalo.webhookCalls) != 1 || r.zalo.webhookCalls[0].url != out.URL {
		t.Fatalf("Zalo was asked %v", r.zalo.webhookCalls)
	}
	if string(out.Secret.Lo()) != r.zalo.webhookCalls[0].secret || len(out.Secret) < 32 {
		t.Fatal("the secret shown is not the one registered")
	}
	pend := r.f.with("SET webhook_secret_pending_sealed = $3")
	if len(pend) != 1 {
		t.Fatalf("pending writes = %d", len(pend))
	}
	sealed, _ := pend[0].args[2].([]byte)
	if bytes.Contains(sealed, out.Secret.Lo()) {
		t.Fatal("the pending secret is stored in the clear")
	}
	if p, err := r.env.Open(r.ctx, sealed, communeBotWebhookAAD(r.ctx, botRowID)); err != nil || !bytes.Equal(p.Lo(), out.Secret.Lo()) {
		t.Fatal("the pending secret does not open under the webhook binding")
	}
	if iP, iZ := indexOfStmt(r.f, "SET webhook_secret_pending_sealed = $3"), indexOfStmt(r.f, "SET webhook_secret_sealed"); iP > iZ {
		t.Error("promoted before the pending secret was stored")
	}
	if got := strings.Join(entryActions(r.f), ","); got != domain.ActionRequestCommuneZaloWebhook+","+domain.ActionSetCommuneZaloWebhook {
		t.Fatalf("entries = %s", got)
	}
	for _, e := range r.f.with("INSERT INTO audit_log") {
		if strings.Contains(string(e.args[7].([]byte)), string(out.Secret.Lo())) {
			t.Fatal("an entry carries the webhook secret")
		}
	}
}

func TestCommuneBotWebhookRefusedClearsPendingAndShowsNothing(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.zalo.setWebhook = []zalobot.Outcome{zalobot.OutcomeRejected}
	out, err := r.uc.RegisterWebhook(r.ctx, staffActor)
	if err != nil {
		t.Fatal(err)
	}
	if out.Outcome != domain.ZaloCallRejected || len(out.Secret) != 0 {
		t.Fatalf("out = %+v", out)
	}
	if len(r.f.with("SET webhook_secret_pending_sealed = NULL")) != 1 {
		t.Error("a refused secret stayed pending")
	}
}

func TestCommuneBotWebhookReusesAPendingSecretAndDoesNotShowItAgain(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.bot.pending = r.sealFor(t, "earlier-generated-secret-0123456789abcdef", botRowID, communeBotWebhookAAD)
	out, err := r.uc.RegisterWebhook(r.ctx, staffActor)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Secret) != 0 {
		t.Error("a reused secret was shown a second time")
	}
	if r.zalo.webhookCalls[0].secret != "earlier-generated-secret-0123456789abcdef" {
		t.Error("the pending secret was not reused")
	}
	if len(r.f.with("SET webhook_secret_pending_sealed = $3")) != 0 {
		t.Error("a second pending secret was written over the first")
	}
}

func TestCommuneBotWebhookWithoutHostOrBotRefuses(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.host = ""
	if _, err := r.uc.RegisterWebhook(r.ctx, staffActor); !errors.Is(err, ErrCommuneHostUnknown) {
		t.Fatalf("no host → %v", err)
	}
	r = newBotRig(t)
	if _, err := r.uc.RegisterWebhook(r.ctx, staffActor); !errors.Is(err, ErrCommuneZaloBotMissing) {
		t.Fatalf("no bot → %v", err)
	}
	if r.zalo.calls != 0 || r.f.begun != 0 {
		t.Error("a refusal called Zalo or opened a transaction")
	}
}

// --- check, retire, current -------------------------------------------------------------------------

func TestCommuneBotCheckRecordsTheClass(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	out, err := r.uc.Check(r.ctx, staffActor)
	if err != nil || out.Outcome != domain.ZaloCallOK || out.AccountName != "Bot ViGov" {
		t.Fatalf("out %+v err %v", out, err)
	}
	if up := r.f.with("SET last_check_at"); len(up) != 1 || up[0].args[3] != domain.ZaloCallOK {
		t.Fatalf("record = %v", up)
	}
}

func TestCommuneBotRetireEndsItsLinksWithTheReason(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.endedLinks = [][]driver.Value{
		{"01JLINKOWN0000000000000001", "CB-00008", botClock.Add(-time.Hour), "commune:bot-111"},
		{"01JLINKOWN0000000000000002", "CB-00009", botClock.Add(-time.Hour), "commune:bot-111"},
	}
	out, err := r.uc.Retire(r.ctx, "  Xã không dùng bot riêng nữa  ", staffActor)
	if err != nil || !out.Retired || out.EndedLinkCount != 2 {
		t.Fatalf("out %+v err %v", out, err)
	}
	ret := r.f.with("SET retired_at")
	if len(ret) != 1 || ret[0].args[3] != "CB-00123" || ret[0].args[4] != "Xã không dùng bot riêng nữa" {
		t.Fatalf("retire = %v", ret)
	}
	if !strings.Contains(ret[0].sql, "webhook_secret_pending_sealed = NULL") {
		t.Error("a retired row could keep a pending secret (0022 retired_has_no_pending)")
	}
	if end := r.f.with("UPDATE zalo_link SET unlinked_at"); len(end) != 1 || end[0].args[1] != "commune:bot-111" {
		t.Fatalf("link end = %v", end)
	}
	if got := strings.Join(entryActions(r.f), ","); got != strings.Join([]string{domain.ActionEndZaloLink,
		domain.ActionEndZaloLink, domain.ActionRetireCommuneZaloBot}, ",") {
		t.Fatalf("entries = %s", got)
	}
	if r.f.commits != 1 {
		t.Fatalf("commits = %d", r.f.commits)
	}
}

func TestCommuneBotRetireRefusalsAndIdempotence(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	if _, err := r.uc.Retire(r.ctx, "   ", staffActor); !errors.Is(err, domain.ErrCommuneZaloBotRetireReason) {
		t.Fatalf("blank reason → %v", err)
	}
	if r.f.begun != 0 {
		t.Fatal("a refused retirement opened a transaction")
	}
	r = newBotRig(t) // no own bot
	out, err := r.uc.Retire(r.ctx, "lý do", staffActor)
	if err != nil || out.Retired || len(r.f.with("UPDATE zalo")) != 0 || len(r.f.with("INSERT")) != 0 {
		t.Fatalf("no bot: out %+v err %v — must write nothing", out, err)
	}
}

func TestCommuneBotCurrentCarriesNoSealedColumn(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	v, err := r.uc.Current(r.ctx)
	if err != nil || !v.HasOwnBot || v.Bot.BotAccountID != "bot-111" || v.Bot.Ref() != "commune:bot-111" {
		t.Fatalf("view %+v err %v", v, err)
	}
	for _, s := range r.f.with("FROM zalo_commune_bot") {
		if strings.Contains(s.sql, "token_sealed") {
			t.Fatalf("the screen's read selected a sealed column: %s", s.sql)
		}
	}
}

// --- the router ---------------------------------------------------------------------------------------

func TestRouterPrefersTheCommunesOwnBotAndFailsClosed(t *testing.T) {
	r := newBotRig(t)
	a, err := r.router.Active(r.ctx)
	if err != nil || a.Own || a.Ref != domain.SharedZaloBotRef || a.Configured {
		t.Fatalf("no own bot, shared unconfigured → %+v %v", a, err)
	}
	r.withLiveBot(t, "bot-111")
	a, tok, err := r.router.ActiveToken(r.ctx)
	if err != nil || !a.Own || a.Ref != "commune:bot-111" || string(tok.Lo()) != zbToken {
		t.Fatalf("own bot → %+v %v", a, err)
	}
	r.router.envelope = nil
	if _, _, err := r.router.ActiveToken(r.ctx); !errors.Is(err, crypto.ErrNotConfigured) {
		t.Fatalf("own bot without a KEK → %v — must never fall back to the shared bot", err)
	}
}

// --- the commune bot's webhook ----------------------------------------------------------------------------

// recordingSend records which token and chat every send went to.
type recordingSend struct {
	tokens, chats []string
}

func (s *recordingSend) SendMessage(_ context.Context, token secret.Secret, chatID, _ string) zalobot.Outcome {
	s.tokens = append(s.tokens, string(token.Lo()))
	s.chats = append(s.chats, chatID)
	return zalobot.OutcomeOK
}

func newCommuneWebhook(t *testing.T, r *botRig, l ZaloPairingLimiter, send ZaloMessenger) *CommuneZaloWebhook {
	t.Helper()
	db := sql.OpenDB(r.f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	if send == nil {
		send = fakeSend{}
	}
	w := NewCommuneZaloWebhook(kho, docstore.NewZaloLinkStore(kho), docstore.NewZaloCommuneBotStore(kho), r.env, l,
		send, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w.now = func() time.Time { return botClock }
	w.newID = func() (string, error) { return "01JLINKNEW0000000000000001", nil }
	return w
}

const liveWebhookSecret = "current-secret-value"

// authed authenticates as the rig's live bot and returns the context Handle and Reply must be given.
func authed(t *testing.T, r *botRig, w *CommuneZaloWebhook) context.Context {
	t.Helper()
	r.bot.webhook = r.sealFor(t, liveWebhookSecret, r.bot.id, communeBotWebhookAAD)
	ctx, ok, err := w.Authenticate(r.ctx, liveWebhookSecret)
	if !ok || err != nil {
		t.Fatalf("authenticate: %v %v", ok, err)
	}
	return ctx
}

func TestCommuneWebhookAuthenticatesAgainstThisCommunesSecrets(t *testing.T) {
	r := newBotRig(t)
	w := newCommuneWebhook(t, r, fakeLimiter{allow: true}, nil)
	if _, ok, err := w.Authenticate(r.ctx, "anything"); ok || err != nil {
		t.Fatalf("no own bot → %v %v", ok, err)
	}
	r.withLiveBot(t, "bot-111")
	r.bot.webhook = r.sealFor(t, "current-secret-value", botRowID, communeBotWebhookAAD)
	r.bot.pending = r.sealFor(t, "pending-secret-value", botRowID, communeBotWebhookAAD)
	for presented, want := range map[string]bool{"current-secret-value": true, "pending-secret-value": true,
		"wrong": false, "": false} {
		ctx, ok, err := w.Authenticate(r.ctx, presented)
		if ok != want || err != nil {
			t.Errorf("%q → %v %v", presented, ok, err)
		}
		if b, carried := authenticatedBot(ctx); carried != want || (want && b.ID != botRowID) {
			t.Errorf("%q: the context carries %+v (%v)", presented, b, carried)
		}
	}
	// The same bytes read in commune B do not open: the binding names the commune.
	ctxB := tenant.Into(context.Background(), xaB)
	if _, ok, err := w.Authenticate(ctxB, "current-secret-value"); ok || err == nil {
		t.Errorf("commune A's secret accepted in commune B: %v %v", ok, err)
	}
}

// D2: a RETIRED row's secrets — in force or pending — authenticate nothing. The read that compares them
// selects the LIVE row only (retired_at IS NULL); a commune whose only row is retired has no live row.
func TestCommuneWebhookRetiredBotsSecretsAre403(t *testing.T) {
	r := newBotRig(t)
	w := newCommuneWebhook(t, r, fakeLimiter{allow: true}, nil)
	r.withLiveBot(t, "bot-111")
	r.bot.webhook = r.sealFor(t, "current-secret-value", botRowID, communeBotWebhookAAD)
	r.bot.pending = r.sealFor(t, "pending-secret-value", botRowID, communeBotWebhookAAD)
	r.bot = nil // retired: the live-only read returns no row
	for _, presented := range []string{"current-secret-value", "pending-secret-value"} {
		if _, ok, err := w.Authenticate(r.ctx, presented); ok || err != nil {
			t.Errorf("a retired bot's %q authenticated: %v %v", presented, ok, err)
		}
	}
	reads := r.f.with("FROM zalo_commune_bot")
	if len(reads) == 0 {
		t.Fatal("no read")
	}
	for _, s := range reads {
		if !strings.Contains(s.sql, "retired_at IS NULL") {
			t.Fatalf("the secret read is not live-only: %s", s.sql)
		}
	}
}

func TestCommuneWebhookHandleWithoutAuthenticatedBotDoesNothing(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.pairingCodeID = "01JCODE00000000000000000001"
	send := &recordingSend{}
	w := newCommuneWebhook(t, r, fakeLimiter{allow: true}, send)
	if got := w.Handle(r.ctx, text("ABCD2345"), ""); got != "" {
		t.Fatalf("→ %q", got)
	}
	w.replyNow(r.ctx, "chat-1", "x")
	if r.f.begun != 0 || len(send.chats) != 0 {
		t.Fatal("an update was acted on without the authenticated bot")
	}
}

// THE RACE SHAPE: X authenticates the update, then Y replaces X before Handle and Reply run. The pairing
// goes through X's ref (0022's trigger then refuses it, X being retired) and no reply goes out through Y.
func TestCommuneWebhookActsThroughTheBotThatAuthenticated(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.pairingCodeID = "01JCODE00000000000000000001"
	send := &recordingSend{}
	w := newCommuneWebhook(t, r, fakeLimiter{allow: true}, send)
	ctx := authed(t, r, w)

	// Y replaces X.
	r.bot = &liveBot{id: botRowNewID, account: "bot-222",
		token: r.sealFor(t, zbOtherToken, botRowNewID, communeBotTokenAAD)}

	w.Handle(ctx, text("ABCD2345"), "")
	ins := r.f.with("INSERT INTO zalo_link")
	if len(ins) != 1 || ins[0].args[3] != "commune:bot-111" {
		t.Fatalf("pairing went through %v — want the AUTHENTICATED bot's ref", ins)
	}
	w.replyNow(ctx, "chat-1", domain.ZaloReplyPaired)
	if len(send.chats) != 0 {
		t.Fatalf("a reply went out through the bot that replaced the one Zalo called (token %v)", send.tokens)
	}
	// Same bot still live: the reply goes out, with its token.
	r.withLiveBot(t, "bot-111")
	w.replyNow(ctx, "chat-1", domain.ZaloReplyPaired)
	if len(send.tokens) != 1 || send.tokens[0] != zbToken {
		t.Fatalf("tokens = %v", send.tokens)
	}
}

// keyCounter is a ratelimit.Counter recording the keys it is asked to count.
type keyCounter struct{ keys []string }

func (c *keyCounter) Incr(_ context.Context, key string, _ time.Duration) (int64, time.Duration, error) {
	c.keys = append(c.keys, key)
	return 1, time.Hour, nil
}

func TestCommuneWebhookPairingLimitIsPerCommuneAndBot(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	counter := &keyCounter{}
	lim, err := ratelimit.New(counter, ratelimit.ZaloBotPairing)
	if err != nil {
		t.Fatal(err)
	}
	w := newCommuneWebhook(t, r, lim, nil)
	w.Handle(authed(t, r, w), text("ABCD2345"), "")
	if len(counter.keys) != 1 || !strings.HasPrefix(counter.keys[0], "t:"+string(xaA)+":") ||
		!strings.Contains(counter.keys[0], "zalo-chat:commune:bot-111:") || strings.Contains(counter.keys[0], "chat-1") {
		t.Fatalf("keys = %v — want the commune and the bot in the key, the chat hashed", counter.keys)
	}
}

func TestCommuneWebhookPairsThroughItsOwnBotInItsOwnCommune(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.pairingCodeID = "01JCODE00000000000000000001"
	w := newCommuneWebhook(t, r, fakeLimiter{allow: true}, nil)
	got := w.Handle(authed(t, r, w), text("ABCD2345"), "203.0.113.9")
	if got != domain.ZaloReplyPaired {
		t.Fatalf("→ %q", got)
	}
	ins := r.f.with("INSERT INTO zalo_link")
	if len(ins) != 1 || ins[0].args[0] != string(xaA) || ins[0].args[3] != "commune:bot-111" {
		t.Fatalf("link = %v — must carry the commune bot's ref, in the Host's commune", ins)
	}
	// The lookup was this commune's: scoped by $1, no cross-commune statement.
	look := r.f.with("SELECT id FROM zalo_pairing_code")
	if len(look) != 1 || look[0].args[0] != string(xaA) {
		t.Fatalf("lookup = %v", look)
	}
	aud := r.f.with("INSERT INTO audit_log")
	if len(aud) != 1 || !strings.Contains(string(aud[0].args[7].([]byte)), `"bot_ref":"commune:bot-111"`) ||
		strings.Contains(string(aud[0].args[7].([]byte)), "chat-1") {
		t.Fatalf("entry = %v", aud)
	}
}

func TestCommuneWebhookRefusesACodeNotOfThisCommune(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	w := newCommuneWebhook(t, r, fakeLimiter{allow: true}, nil)
	if got := w.Handle(authed(t, r, w), text("ABCD2345"), ""); got != domain.ZaloReplyPairingRefused {
		t.Fatalf("→ %q", got)
	}
	if r.f.begun != 0 {
		t.Error("a transaction was opened for a code this commune does not hold")
	}
}

// --- the shared bot refuses pairing in a commune that has its own ------------------------------------------

func TestSharedWebhookRefusesPairingWhereTheCommuneHasItsOwnBot(t *testing.T) {
	f := &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "FROM zalo_commune_bot") {
			b := &liveBot{id: botRowID, account: "bot-111", token: []byte("x")}
			return communeBotCols(false), [][]driver.Value{b.values(false)}, nil
		}
		return nil, nil, errors.New("unexpected")
	}}
	db := sql.OpenDB(f)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	w := NewZaloWebhook(kho, docstore.NewZaloLinkStore(kho), docstore.NewZaloCommuneBotStore(kho),
		&fakeResolver{matches: []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: "c1"}}}, fakeLimiter{allow: true},
		fakeBot{}, fakeSend{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if got := w.Handle(context.Background(), text("ABCD2345"), ""); got != domain.ZaloReplyPairingRefused {
		t.Fatalf("→ %q", got)
	}
	if f.begun != 0 {
		t.Error("the shared bot opened a pairing transaction in a commune that uses its own bot")
	}
}

// --- the staff test message goes through the commune's bot ------------------------------------------------

type fakeSource struct {
	bot   ActiveZaloBot
	token string
}

func (s fakeSource) Active(context.Context) (ActiveZaloBot, error) { return s.bot, nil }
func (s fakeSource) ActiveToken(context.Context) (ActiveZaloBot, secret.Secret, error) {
	return s.bot, secret.Secret(s.token), nil
}

type noNames struct{}

func (noNames) TenCanBoTheoMa(context.Context, []string) (map[string]identityclient.TenCanBo, error) { // vi-name-ok: the method of the existing StaffNames interface (core/identityclient)
	return nil, nil
}

func TestZaloLinksUseTheBotThatServesTheCommune(t *testing.T) {
	f := &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "FROM zalo_channel_setting"):
			return []string{"e", "k", "qs", "qe", "sd", "rd", "ua", "ub"},
				[][]driver.Value{{true, "nhiem-vu.qua-han", "21:00", "06:00", int64(1), int64(1), botClock, "CB-1"}}, nil
		case strings.Contains(q, "FROM zalo_link"):
			return []string{"staff_code", "id", "chat_id"}, nil, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	src := fakeSource{bot: ActiveZaloBot{Ref: "commune:bot-111", Own: true, Configured: true, BotName: "Bot Xã A",
		ChatURL: "https://zalo.me/123"}, token: zbToken}
	uc := NewZaloLinks(kho, docstore.NewZaloLinkStore(kho), src, fakeSend{}, noNames{}, nil)
	ctx := tenant.Into(context.Background(), xaA)
	if err := uc.SendTestMessage(ctx, staffActor); !errors.Is(err, ErrZaloNotLinked) {
		t.Fatalf("err = %v", err)
	}
	chats := f.with("FROM zalo_link")
	if len(chats) != 1 || chats[0].args[1] != "commune:bot-111" {
		t.Fatalf("the chat was looked up through %v, want the commune's own bot", chats)
	}
	if ready, err := uc.BotReady(ctx); err != nil || !ready {
		t.Fatalf("BotReady = %v %v", ready, err)
	}
}

// --- live_link_count is a scoped COUNT of the serving bot's links (no list ceiling) -------------------------

func TestCommuneBotCurrentCountsTheServingBotsLinks(t *testing.T) {
	r := newBotRig(t)
	r.linkCount = 2501 // past MaxZaloLinksListed: a count, not a list
	v, err := r.uc.Current(r.ctx)
	if err != nil || v.LiveLinkCount != 2501 {
		t.Fatalf("shared: %+v %v", v, err)
	}
	r.withLiveBot(t, "bot-111")
	if _, err := r.uc.Current(r.ctx); err != nil {
		t.Fatal(err)
	}
	counts := r.f.with("count(*)")
	if len(counts) != 2 || counts[0].args[0] != string(xaA) || counts[0].args[1] != domain.SharedZaloBotRef ||
		counts[1].args[1] != "commune:bot-111" {
		t.Fatalf("counts = %v", counts)
	}
}

// --- G1: the security log (TCVN 14423 §6.8.2.1) -----------------------------------------------------------

func TestCommuneBotChangesReachTheSecurityLogWithoutSecrets(t *testing.T) {
	r := newBotRig(t)
	if _, err := r.uc.Set(r.ctx, setInput(zbToken), staffActor); err != nil {
		t.Fatal(err)
	}
	out, err := r.uc.RegisterWebhook(r.ctx, staffActor)
	if err == nil {
		t.Fatalf("no live bot in the fake yet — expected a refusal, got %+v", out)
	}
	r.withLiveBot(t, "bot-111")
	wh, err := r.uc.RegisterWebhook(r.ctx, staffActor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.uc.Check(r.ctx, staffActor); err != nil {
		t.Fatal(err)
	}
	if _, err := r.uc.Retire(r.ctx, "Quay về bot chung", staffActor); err != nil {
		t.Fatal(err)
	}
	r.zalo.getMeOutcome = zalobot.OutcomeTokenRejected
	_, _ = r.uc.Set(r.ctx, setInput(zbToken), staffActor)

	logs := r.logs.String()
	for _, want := range []string{
		`"event":"zalo_commune_bot.set","outcome":"adopted"`,
		`"event":"zalo_commune_bot.webhook","outcome":"confirmed"`,
		`"event":"zalo_commune_bot.checked","outcome":"thanh-cong"`,
		`"event":"zalo_commune_bot.retired","outcome":"retired"`,
		`"outcome":"token_refused:token-bi-tu-choi"`,
		`"xa":"` + string(xaA) + `"`, `"actor":"CB-00123"`, `"ip":"10.0.0.7"`,
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("security log lacks %s:\n%s", want, logs)
		}
	}
	for _, bad := range []string{zbToken, string(wh.Secret.Lo()), "https://", "zalo-bot-updates"} {
		if bad != "" && strings.Contains(logs, bad) {
			t.Fatalf("the security log carries %q", bad)
		}
	}
}

// --- the dispatcher sends each commune's notices through that commune's bot -------------------------------

type twoCommuneLocks struct{}

func (twoCommuneLocks) CommunesWithDueZaloDeliveries(context.Context, time.Time) ([]tenant.ID, error) {
	return []tenant.ID{xaA, xaB}, nil
}
func (twoCommuneLocks) TryLockScheduler(context.Context) (func(), bool, error) {
	return func() {}, true, nil
}

type activeRegistry struct{}

func (activeRegistry) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) { // vi-name-ok: the method of the existing PortalCommuneRegistry interface
	return tenant.Tenant{ID: tenant.MustFrom(ctx), Host: "xa.example.gov.vn", Active: true}, true, nil
}

// perCommuneSource: A has its own bot, B is on the shared bot.
type perCommuneSource struct{}

func (perCommuneSource) Active(ctx context.Context) (ActiveZaloBot, error) {
	a, _, err := perCommuneSource{}.ActiveToken(ctx)
	return a, err
}

func (perCommuneSource) ActiveToken(ctx context.Context) (ActiveZaloBot, secret.Secret, error) {
	if tenant.MustFrom(ctx) == xaA {
		return ActiveZaloBot{Ref: "commune:bot-a", Own: true, Configured: true}, secret.Secret("token-of-A"), nil
	}
	return ActiveZaloBot{Ref: domain.SharedZaloBotRef, Configured: true}, secret.Secret("token-shared"), nil
}

func TestDispatcherSendsEachCommuneThroughItsOwnBot(t *testing.T) {
	f := &sqlFake{query: func(q string, args []driver.Value) ([]string, [][]driver.Value, error) {
		xa := fmt.Sprint(args[0])
		switch {
		case strings.Contains(q, "FROM zalo_delivery d"):
			return []string{"id", "staff_code", "kind", "attempts", "title", "body", "link"},
				[][]driver.Value{{"01JDELIV" + xa[:6], "CB-00001", "qua-han", int64(0), "Việc quá hạn", "", ""}}, nil
		case strings.Contains(q, "FROM zalo_channel_setting"):
			// Per-domain selection; the notice is the OLD kind producers still send (0021's map).
			return []string{"e", "k", "qs", "qe", "sd", "rd", "ua", "ub"},
				[][]driver.Value{{true, "phan-anh.qua-han", "21:00", "06:00", int64(1), int64(1), botClock, "CB-1"}}, nil
		case strings.Contains(q, "FROM zalo_link"):
			return []string{"staff_code", "id", "chat_id"},
				[][]driver.Value{{"CB-00001", "01JLINK" + xa[:6], "chat-of-" + xa}}, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	send := &recordingSend{}
	z, err := NewZaloDispatcher(ZaloDispatcherDeps{DB: kho, Repo: docstore.NewZaloLinkStore(kho), Locks: twoCommuneLocks{},
		Registry: activeRegistry{}, Bots: perCommuneSource{}, Send: send,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	// Noon in Vietnam: outside the quiet window.
	z.now = func() time.Time { return time.Date(2026, 10, 8, 5, 0, 0, 0, time.UTC) }
	z.Tick(context.Background())

	if len(send.chats) != 2 {
		t.Fatalf("sends = %v / %v", send.chats, send.tokens)
	}
	for i, chat := range send.chats {
		want := map[string]string{"chat-of-" + string(xaA): "token-of-A", "chat-of-" + string(xaB): "token-shared"}[chat]
		if want == "" || send.tokens[i] != want {
			t.Errorf("chat %s went with token %q, want %q", chat, send.tokens[i], want)
		}
	}
	for _, s := range f.with("FROM zalo_link") {
		want := map[string]string{string(xaA): "commune:bot-a", string(xaB): domain.SharedZaloBotRef}[fmt.Sprint(s.args[0])]
		if s.args[1] != want {
			t.Errorf("commune %v read its chats through %v, want %s", s.args[0], s.args[1], want)
		}
	}
}
