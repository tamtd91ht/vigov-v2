package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/store/platformstore"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// Fake values — the text says so (rule 8, forbidden #1).
const (
	zbToken      = "123456789:FAKE-zalo-bot-token-NOT-REAL"
	zbOtherToken = "555555555:FAKE-other-bot-token-NOT-REAL"
	zbActor      = "VH-00001"
	zbHost       = "bot.api.vigov.vn"
	zbChatURL    = "https://zalo.me/1234567890"
	communeX     = "01JTESTCOMMUNEXXXXXXXXXXXX"
	communeY     = "01JTESTCOMMUNEYYYYYYYYYYYY"
)

var zbKEK = secret.Secret("kek-Z-FAKE-NOT-A-REAL-KEY-32byt!")

// ---- fake store: one committed state, a copy per transaction, swapped in on commit ----------------

type zbState struct {
	bot          *domain.SharedZaloBot
	tokenSealed  []byte
	current      []byte
	pending      []byte
	links        map[string][]string // live links: commune -> staff codes
	ended        []string            // "commune/staff/by"
	channelOn    map[string]bool
	trail        []platformstore.OperatorAudit
	communeTrail []string // "commune/staff/actor"
}

func (s zbState) clone() zbState {
	c := s
	if s.bot != nil {
		b := *s.bot
		c.bot = &b
	}
	c.links = map[string][]string{}
	for k, v := range s.links {
		c.links[k] = slices.Clone(v)
	}
	c.channelOn = maps.Clone(s.channelOn)
	c.ended = slices.Clone(s.ended)
	c.trail = slices.Clone(s.trail)
	c.communeTrail = slices.Clone(s.communeTrail)
	return c
}

type fakeZaloStore struct {
	mu         sync.Mutex
	st         zbState
	failCommit bool
	clock      time.Time
}

func newFakeZaloStore() *fakeZaloStore {
	return &fakeZaloStore{st: zbState{links: map[string][]string{}, channelOn: map[string]bool{}},
		clock: time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)}
}

func (f *fakeZaloStore) now() time.Time { f.clock = f.clock.Add(time.Second); return f.clock }

func (f *fakeZaloStore) SharedBot(context.Context) (domain.SharedZaloBot, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.st.bot == nil {
		return domain.SharedZaloBot{}, false, nil
	}
	b := *f.st.bot
	b.HasPendingWebhook = f.st.pending != nil
	return b, true, nil
}

func (f *fakeZaloStore) SharedBotToken(context.Context) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return bytes.Clone(f.st.tokenSealed), f.st.bot != nil, nil
}

func (f *fakeZaloStore) PendingWebhookSecretRead(context.Context) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return bytes.Clone(f.st.pending), nil
}

var errFakeCommit = errors.New("fake: commit failed")

func (f *fakeZaloStore) InTx(ctx context.Context, fn func(ZaloBotTx) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	tx := &fakeZaloTx{f: f, st: f.st.clone()}
	if err := fn(tx); err != nil {
		return err
	}
	if tx.wrote && !tx.audited {
		return platformstore.ErrWriteWithoutTrail
	}
	if f.failCommit {
		return errFakeCommit
	}
	f.st = tx.st
	return nil
}

type fakeZaloTx struct {
	f              *fakeZaloStore
	st             zbState
	wrote, audited bool
}

func (t *fakeZaloTx) LockSharedBot(context.Context) (domain.SharedZaloBot, []byte, bool, error) {
	if t.st.bot == nil {
		return domain.SharedZaloBot{}, nil, false, nil
	}
	return *t.st.bot, bytes.Clone(t.st.tokenSealed), true, nil
}

func (t *fakeZaloTx) SaveToken(_ context.Context, in platformstore.SavedToken) (time.Time, error) {
	t.wrote = true
	at := t.f.now()
	b := domain.SharedZaloBot{}
	if t.st.bot != nil {
		b = *t.st.bot
	}
	b.BotAccountID, b.BotName, b.ChatURL, b.SetAt, b.SetBy = in.BotAccountID, in.BotName, in.ChatURL, at, in.SetBy
	b.LastCheckAt, b.LastCheckResult = at, domain.ZaloCallOK
	t.st.bot, t.st.tokenSealed = &b, bytes.Clone(in.Sealed)
	return at, nil
}

func (t *fakeZaloTx) RecordCheck(_ context.Context, checked []byte, result string) (time.Time, error) {
	t.wrote = true
	if t.st.bot == nil || !bytes.Equal(checked, t.st.tokenSealed) {
		return time.Time{}, platformstore.ErrChanged
	}
	at := t.f.now()
	t.st.bot.LastCheckAt, t.st.bot.LastCheckResult = at, result
	return at, nil
}

func (t *fakeZaloTx) PendingWebhookSecret(context.Context) ([]byte, error) {
	return bytes.Clone(t.st.pending), nil
}

func (t *fakeZaloTx) SetPendingWebhookSecret(_ context.Context, sealed []byte) (time.Time, error) {
	t.wrote = true
	t.st.pending = bytes.Clone(sealed)
	return t.f.now(), nil
}

func (t *fakeZaloTx) PromotePendingWebhookSecret(_ context.Context, p []byte, by string) (time.Time, error) {
	t.wrote = true
	if !bytes.Equal(p, t.st.pending) || p == nil {
		return time.Time{}, platformstore.ErrChanged
	}
	at := t.f.now()
	t.st.current, t.st.pending = t.st.pending, nil
	t.st.bot.WebhookSetAt, t.st.bot.WebhookSetBy = at, by
	return at, nil
}

func (t *fakeZaloTx) ClearPendingWebhookSecret(_ context.Context, p []byte) error {
	t.wrote = true
	if !bytes.Equal(p, t.st.pending) {
		return platformstore.ErrChanged
	}
	t.st.pending = nil
	return nil
}

func (t *fakeZaloTx) CountLiveLinks(context.Context) (int, error) {
	n := 0
	for _, v := range t.st.links {
		n += len(v)
	}
	return n, nil
}

func (t *fakeZaloTx) EndAllLiveLinks(_ context.Context, op platformstore.Operator) (map[string]int, int, error) {
	t.wrote = true
	out, total := map[string]int{}, 0
	for c, staff := range t.st.links {
		for _, s := range staff {
			t.st.ended = append(t.st.ended, c+"/"+s+"/"+op.Code)
			t.st.communeTrail = append(t.st.communeTrail, c+"/"+s+"/"+op.Code)
		}
		out[c], total = len(staff), total+len(staff)
	}
	t.st.links = map[string][]string{}
	return out, total, nil
}

func (t *fakeZaloTx) CommuneStats(context.Context) ([]domain.ZaloBotCommuneStat, error) {
	ids := map[string]bool{}
	for c := range t.st.links {
		ids[c] = true
	}
	for c := range t.st.channelOn {
		ids[c] = true
	}
	var out []domain.ZaloBotCommuneStat
	for _, c := range slices.Sorted(maps.Keys(ids)) {
		out = append(out, domain.ZaloBotCommuneStat{TenantID: c, ChannelEnabled: t.st.channelOn[c], LinkedStaffCount: len(t.st.links[c])})
	}
	return out, nil
}

func (t *fakeZaloTx) AppendOperatorAudit(_ context.Context, e platformstore.OperatorAudit) error {
	if !domain.ValidOperatorCode(e.Operator.Code) || e.Operator.IP == "" {
		return platformstore.ErrNoOperator
	}
	t.audited = true
	t.st.trail = append(t.st.trail, e)
	return nil
}

// ---- fake Zalo --------------------------------------------------------------------------------------

type fakeZalo struct {
	accounts     map[string]zalobot.BotInfo // token -> who getMe says it is
	getMeOutcome zalobot.Outcome            // 0 = OK when the token is known, TokenRejected otherwise
	setWebhook   []zalobot.Outcome          // answered in order; OK when exhausted
	calls        int
	webhookCalls []struct{ url, secret string }
	info         zalobot.WebhookInfo
}

func newFakeZalo() *fakeZalo {
	return &fakeZalo{accounts: map[string]zalobot.BotInfo{
		zbToken:      {AccountID: "bot-111", AccountName: "Bot ViGov"},
		zbOtherToken: {AccountID: "bot-222", AccountName: "Bot Khác"},
	}}
}

func (z *fakeZalo) GetMe(_ context.Context, token secret.Secret) (zalobot.BotInfo, zalobot.Outcome) {
	z.calls++
	if z.getMeOutcome != 0 {
		return zalobot.BotInfo{}, z.getMeOutcome
	}
	info, ok := z.accounts[string(token)]
	if !ok {
		return zalobot.BotInfo{}, zalobot.OutcomeTokenRejected
	}
	return info, zalobot.OutcomeOK
}

func (z *fakeZalo) SetWebhook(_ context.Context, _ secret.Secret, u string, sec secret.Secret) zalobot.Outcome {
	z.calls++
	z.webhookCalls = append(z.webhookCalls, struct{ url, secret string }{u, string(sec)})
	if len(z.setWebhook) == 0 {
		return zalobot.OutcomeOK
	}
	o := z.setWebhook[0]
	z.setWebhook = z.setWebhook[1:]
	return o
}

func (z *fakeZalo) GetWebhookInfo(context.Context, secret.Secret) (zalobot.WebhookInfo, zalobot.Outcome) {
	z.calls++
	return z.info, zalobot.OutcomeOK
}

type memPlatformDEK struct{ row *crypto.WrappedDEK }

func (m *memPlatformDEK) GetPlatformDEK(context.Context) (crypto.WrappedDEK, error) {
	if m.row == nil {
		return crypto.WrappedDEK{}, crypto.ErrPlatformDEKNotFound
	}
	return *m.row, nil
}

func (m *memPlatformDEK) CreatePlatformDEK(_ context.Context, w crypto.WrappedDEK) error {
	if m.row != nil {
		return crypto.ErrPlatformDEKExists
	}
	m.row = &w
	return nil
}

func (m *memPlatformDEK) ReplacePlatformDEK(_ context.Context, _, n crypto.WrappedDEK) error {
	m.row = &n
	return nil
}

type zbRig struct {
	uc    *ZaloBotOperator
	store *fakeZaloStore
	zalo  *fakeZalo
	env   *crypto.PlatformEnvelope
}

func newZBRig(t *testing.T) zbRig {
	t.Helper()
	env, err := crypto.NewPlatform([]secret.Secret{zbKEK}, &memPlatformDEK{})
	if err != nil {
		t.Fatal(err)
	}
	st, z := newFakeZaloStore(), newFakeZalo()
	return zbRig{uc: NewZaloBotOperator(st, env, z, zbHost), store: st, zalo: z, env: env}
}

func setIn(token string, expected int) SetSharedZaloBotInput {
	return SetSharedZaloBotInput{Token: secret.Secret(token), BotName: " Bot ViGov ", ChatURL: zbChatURL,
		ActorCode: zbActor, ActorIP: "10.0.0.5", ExpectedRelinkCount: expected}
}

// noSecretIn fails when any trail entry, or text, carries a secret.
func noSecretIn(t *testing.T, st zbState, secrets ...string) {
	t.Helper()
	b, err := json.Marshal(st.trail)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range secrets {
		if s != "" && (bytes.Contains(b, []byte(s)) || strings.Contains(strings.Join(st.communeTrail, "|"), s)) {
			t.Fatalf("a secret reached the trail: %s", b)
		}
	}
}

// ---- SetSharedZaloBot ---------------------------------------------------------------------------------

func TestSetSharedZaloBotRefusesBadRequestsBeforeAnything(t *testing.T) {
	r := newZBRig(t)
	cases := map[string]SetSharedZaloBotInput{}
	for _, code := range []string{"", "CB-00001", "system", "VH-0001", "01JD8ZQK9M3NPXR7TVWYB2C4EF"} {
		in := setIn(zbToken, 0)
		in.ActorCode = code
		cases["actor "+code] = in
	}
	in := setIn("a/b", 0)
	cases["token with slash"] = in
	in = setIn(zbToken, 0)
	in.ChatURL = "http://zalo.me/x"
	cases["http chat url"] = in
	in = setIn(zbToken, 0)
	in.BotName = ""
	cases["empty name"] = in
	in = setIn(zbToken, -1)
	cases["negative count"] = in
	in = setIn(zbToken, 0)
	in.ActorIP = "not-an-ip"
	cases["bad ip"] = in
	for name, in := range cases {
		_, err := r.uc.SetSharedZaloBot(context.Background(), in)
		if !errors.Is(err, domain.ErrZaloBotInvalidArgument) {
			t.Errorf("%s: err = %v", name, err)
		}
		if strings.Contains(errString(err), zbToken) {
			t.Errorf("%s: the error quotes the token", name)
		}
	}
	if r.zalo.calls != 0 || r.store.st.bot != nil || len(r.store.st.trail) != 0 {
		t.Fatalf("a refused request reached Zalo or the store: calls=%d trail=%d", r.zalo.calls, len(r.store.st.trail))
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestSetSharedZaloBotTokenZaloRefusesIsNeverSaved(t *testing.T) {
	r := newZBRig(t)
	r.zalo.getMeOutcome = zalobot.OutcomeTokenRejected
	out, err := r.uc.SetSharedZaloBot(context.Background(), setIn(zbToken, 0))
	if err != nil || out.Result != SetTokenCheckFailed || out.ZaloOutcome != domain.ZaloCallTokenRejected {
		t.Fatalf("= %+v, %v", out, err)
	}
	if r.store.st.bot != nil || len(r.store.st.trail) != 0 {
		t.Fatal("a token Zalo refused was saved, or audited")
	}
}

func TestSetSharedZaloBotFirstSaveSealsAndAudits(t *testing.T) {
	r := newZBRig(t)
	out, err := r.uc.SetSharedZaloBot(context.Background(), setIn(zbToken, 0))
	if err != nil || out.Result != SetSaved {
		t.Fatalf("= %+v, %v", out, err)
	}
	st := r.store.st
	if st.bot.BotAccountID != "bot-111" || st.bot.BotName != "Bot ViGov" || st.bot.SetBy != zbActor ||
		st.bot.LastCheckResult != domain.ZaloCallOK {
		t.Fatalf("row = %+v", st.bot)
	}
	if bytes.Contains(st.tokenSealed, []byte(zbToken)) {
		t.Fatal("the token is stored in the clear")
	}
	got, err := r.env.OpenPlatform(context.Background(), st.tokenSealed, tokenAAD)
	if err != nil || string(got) != zbToken {
		t.Fatalf("the sealed token does not open to the token: %v", err)
	}
	if len(st.trail) != 1 || st.trail[0].Action != domain.ActionZaloBotTokenSet || st.trail[0].Operator.Code != zbActor ||
		st.trail[0].Operator.IP != "10.0.0.5" {
		t.Fatalf("trail = %+v", st.trail)
	}
	noSecretIn(t, st, zbToken, "FAKE-zalo")
}

func TestSetSharedZaloBotSameAccountKeepsLinks(t *testing.T) {
	r := newZBRig(t)
	if _, err := r.uc.SetSharedZaloBot(context.Background(), setIn(zbToken, 0)); err != nil {
		t.Fatal(err)
	}
	r.store.st.links[communeX] = []string{"CB-001", "CB-002"}
	// A rotated token of the SAME bot account: the count is ignored, every link kept.
	r.zalo.accounts["123456789:FAKE-rotated-NOT-REAL"] = zalobot.BotInfo{AccountID: "bot-111"}
	out, err := r.uc.SetSharedZaloBot(context.Background(), setIn("123456789:FAKE-rotated-NOT-REAL", 99))
	if err != nil || out.Result != SetSaved || out.EndedLinkCount != 0 {
		t.Fatalf("= %+v, %v", out, err)
	}
	if len(r.store.st.links[communeX]) != 2 || len(r.store.st.ended) != 0 {
		t.Fatal("links of the same bot were ended")
	}
}

func TestSetSharedZaloBotOtherAccountNeedsTheConfirmedCount(t *testing.T) {
	r := newZBRig(t)
	if _, err := r.uc.SetSharedZaloBot(context.Background(), setIn(zbToken, 0)); err != nil {
		t.Fatal(err)
	}
	r.store.st.links[communeX] = []string{"CB-001", "CB-002"}
	r.store.st.links[communeY] = []string{"CB-009"}
	before := r.store.st.clone()

	// The default 0 against a bot with links: the dialog, nothing written.
	out, err := r.uc.SetSharedZaloBot(context.Background(), setIn(zbOtherToken, 0))
	if err != nil || out.Result != SetRelinkConfirmationRequired || out.LiveLinkCount != 3 ||
		out.ZaloOutcome != domain.ZaloCallOK {
		t.Fatalf("= %+v, %v", out, err)
	}
	if r.store.st.bot.BotAccountID != "bot-111" || len(r.store.st.trail) != len(before.trail) ||
		!bytes.Equal(r.store.st.tokenSealed, before.tokenSealed) || len(r.store.st.ended) != 0 {
		t.Fatal("an unconfirmed change of bot account wrote something")
	}

	// Resubmitted with the number shown: every link ended, each audited in its own commune.
	out, err = r.uc.SetSharedZaloBot(context.Background(), setIn(zbOtherToken, 3))
	if err != nil || out.Result != SetSaved || out.EndedLinkCount != 3 {
		t.Fatalf("= %+v, %v", out, err)
	}
	st := r.store.st
	if st.bot.BotAccountID != "bot-222" || len(st.links) != 0 || len(st.ended) != 3 {
		t.Fatalf("bot=%s live=%v ended=%v", st.bot.BotAccountID, st.links, st.ended)
	}
	for _, e := range st.communeTrail {
		if !strings.HasSuffix(e, "/"+zbActor) {
			t.Errorf("a link end attributed to someone else: %s", e)
		}
	}
	last := st.trail[len(st.trail)-1].After.(map[string]any)
	if last["ended_link_count"] != 3 || last["account_changed"] != true {
		t.Fatalf("trail after = %v", last)
	}
	if byCommune := last["ended_links_by_commune"].(map[string]int); byCommune[communeX] != 2 || byCommune[communeY] != 1 {
		t.Fatalf("per-commune counts = %v", byCommune)
	}
	noSecretIn(t, st, zbToken, zbOtherToken)
}

// Nothing stored while links exist: "unknown" is "different" (fail closed).
func TestSetSharedZaloBotUnknownAccountIsTreatedAsDifferent(t *testing.T) {
	r := newZBRig(t)
	r.store.st.links[communeX] = []string{"CB-001"}
	out, err := r.uc.SetSharedZaloBot(context.Background(), setIn(zbToken, 0))
	if err != nil || out.Result != SetRelinkConfirmationRequired || out.LiveLinkCount != 1 {
		t.Fatalf("= %+v, %v", out, err)
	}
}

func TestSetSharedZaloBotWithoutKEKIsNotConfigured(t *testing.T) {
	st, z := newFakeZaloStore(), newFakeZalo()
	uc := NewZaloBotOperator(st, nil, z, zbHost)
	if _, err := uc.SetSharedZaloBot(context.Background(), setIn(zbToken, 0)); !errors.Is(err, crypto.ErrNotConfigured) {
		t.Fatalf("err = %v", err)
	}
	if z.calls != 0 || st.st.bot != nil {
		t.Fatal("without a KEK, Zalo was asked or something was written")
	}
}

func TestSetSharedZaloBotCommitFailureWritesNothing(t *testing.T) {
	r := newZBRig(t)
	r.store.failCommit = true
	if _, err := r.uc.SetSharedZaloBot(context.Background(), setIn(zbToken, 0)); !errors.Is(err, errFakeCommit) {
		t.Fatalf("err = %v", err)
	}
	if r.store.st.bot != nil || len(r.store.st.trail) != 0 {
		t.Fatal("a failed commit left a row or an entry")
	}
}

// ---- Get / Check ----------------------------------------------------------------------------------------

func TestCheckSharedZaloBot(t *testing.T) {
	r := newZBRig(t)
	out, err := r.uc.CheckSharedZaloBot(context.Background(), zbActor, "")
	if err != nil || out.Outcome != domain.ZaloCallNotConfigured || !out.CheckedAt.IsZero() || r.zalo.calls != 0 {
		t.Fatalf("not configured: = %+v, %v (calls %d)", out, err, r.zalo.calls)
	}
	if len(r.store.st.trail) != 0 {
		t.Fatal("NOT_CONFIGURED recorded something")
	}

	if _, err := r.uc.SetSharedZaloBot(context.Background(), setIn(zbToken, 0)); err != nil {
		t.Fatal(err)
	}
	out, err = r.uc.CheckSharedZaloBot(context.Background(), zbActor, "")
	if err != nil || out.Outcome != domain.ZaloCallOK || out.AccountName != "Bot ViGov" || out.CheckedAt.IsZero() {
		t.Fatalf("= %+v, %v", out, err)
	}
	last := r.store.st.trail[len(r.store.st.trail)-1]
	if last.Action != domain.ActionZaloBotChecked || last.Operator.IP != domain.UnknownActorIP {
		t.Fatalf("trail = %+v", last)
	}

	r.zalo.getMeOutcome = zalobot.OutcomeRateLimited
	out, err = r.uc.CheckSharedZaloBot(context.Background(), zbActor, "")
	if err != nil || out.Outcome != domain.ZaloCallRateLimited || out.AccountName != "" {
		t.Fatalf("= %+v, %v", out, err)
	}
	b, _, _ := r.uc.GetSharedZaloBot(context.Background())
	if b.LastCheckResult != domain.ZaloCallRateLimited || !b.LastCheckAt.Equal(out.CheckedAt) {
		t.Fatalf("last check = %+v", b)
	}

	r.store.failCommit = true
	if _, err := r.uc.CheckSharedZaloBot(context.Background(), zbActor, ""); err == nil {
		t.Fatal("a failed record must fail the RPC")
	}
	if _, err := r.uc.CheckSharedZaloBot(context.Background(), "", ""); !errors.Is(err, domain.ErrZaloBotActorCode) {
		t.Fatalf("no actor: %v", err)
	}
}

// ---- the webhook ----------------------------------------------------------------------------------------

func configured(t *testing.T) zbRig {
	t.Helper()
	r := newZBRig(t)
	if _, err := r.uc.SetSharedZaloBot(context.Background(), setIn(zbToken, 0)); err != nil {
		t.Fatal(err)
	}
	return r
}

func actionsOf(st zbState) []string {
	var out []string
	for _, e := range st.trail {
		out = append(out, e.Action)
	}
	return out
}

func TestSetWebhookOKPromotesAFreshSecret(t *testing.T) {
	r := configured(t)
	out, err := r.uc.SetSharedZaloBotWebhook(context.Background(), zbActor, "10.0.0.5")
	if err != nil || out.Outcome != domain.ZaloCallOK || out.URL != "https://bot.api.vigov.vn/api/v1/zalo-bot-updates" ||
		out.SetBy != zbActor || out.SetAt.IsZero() {
		t.Fatalf("= %+v, %v", out, err)
	}
	call := r.zalo.webhookCalls[0]
	if call.url != out.URL || len(call.secret) < 43 {
		t.Fatalf("Zalo got url %q and a %d-character secret", call.url, len(call.secret))
	}
	st := r.store.st
	if st.pending != nil || st.current == nil {
		t.Fatal("the secret was not promoted")
	}
	if cur, err := r.env.OpenPlatform(context.Background(), st.current, webhookSecretAAD); err != nil || string(cur) != call.secret {
		t.Fatalf("the current secret is not the one Zalo got: %v", err)
	}
	if got := actionsOf(st)[1:]; !slices.Equal(got, []string{domain.ActionZaloBotWebhookRequested, domain.ActionZaloBotWebhookSet}) {
		t.Fatalf("trail = %v", got)
	}
	noSecretIn(t, st, zbToken, call.secret)

	// Again: a NEW secret, the old current retired.
	if _, err := r.uc.SetSharedZaloBotWebhook(context.Background(), zbActor, ""); err != nil {
		t.Fatal(err)
	}
	if r.zalo.webhookCalls[1].secret == call.secret {
		t.Fatal("a second call reused a secret that was already current")
	}
}

func TestSetWebhookDefiniteRefusalClearsThePending(t *testing.T) {
	for _, o := range []zalobot.Outcome{zalobot.OutcomeTokenRejected, zalobot.OutcomeRejected, zalobot.OutcomeRateLimited} {
		r := configured(t)
		r.zalo.setWebhook = []zalobot.Outcome{o}
		out, err := r.uc.SetSharedZaloBotWebhook(context.Background(), zbActor, "")
		if err != nil || out.Outcome != outcomeValue(o) || !out.SetAt.IsZero() {
			t.Fatalf("%v: = %+v, %v", o, out, err)
		}
		if r.store.st.pending != nil || r.store.st.current != nil {
			t.Fatalf("%v: pending kept or promoted after a definite refusal", o)
		}
		if a := actionsOf(r.store.st); a[len(a)-1] != domain.ActionZaloBotWebhookRefused {
			t.Fatalf("%v: trail = %v", o, a)
		}
	}
}

// Ambiguous: the pending secret stays accepted, and the NEXT call sends the SAME secret (one pending
// slot — replacing it could 403 updates Zalo already signs with it), then promotes it.
func TestSetWebhookAmbiguousKeepsAndReusesThePending(t *testing.T) {
	r := configured(t)
	r.zalo.setWebhook = []zalobot.Outcome{zalobot.OutcomeUnavailable}
	out, err := r.uc.SetSharedZaloBotWebhook(context.Background(), zbActor, "")
	if err != nil || out.Outcome != domain.ZaloCallUnavailable {
		t.Fatalf("= %+v, %v", out, err)
	}
	if r.store.st.pending == nil {
		t.Fatal("an ambiguous answer discarded a secret Zalo may be using")
	}
	if a := actionsOf(r.store.st); a[len(a)-1] != domain.ActionZaloBotWebhookUnconfirmed {
		t.Fatalf("trail = %v", a)
	}
	out, err = r.uc.SetSharedZaloBotWebhook(context.Background(), zbActor, "")
	if err != nil || out.Outcome != domain.ZaloCallOK {
		t.Fatalf("retry: = %+v, %v", out, err)
	}
	if r.zalo.webhookCalls[0].secret != r.zalo.webhookCalls[1].secret {
		t.Fatal("the retry replaced a pending secret Zalo may already be using")
	}
	if r.store.st.pending != nil || r.store.st.current == nil {
		t.Fatal("the retry did not promote")
	}
}

func TestSetWebhookStepThreeFailureIsUnavailableAndKeepsThePending(t *testing.T) {
	r := configured(t)
	// Fail ONLY the third transaction: step 1 commits, Zalo answers OK, step 3 fails.
	r.zalo.setWebhook = nil
	orig := r.store
	wrapped := &failNthTx{fakeZaloStore: orig, failOn: 2}
	r.uc.store = wrapped
	if _, err := r.uc.SetSharedZaloBotWebhook(context.Background(), zbActor, ""); err == nil {
		t.Fatal("a failed step 3 must fail the RPC")
	}
	if orig.st.pending == nil {
		t.Fatal("the pending secret Zalo now uses was lost")
	}
}

type failNthTx struct {
	*fakeZaloStore
	n, failOn int
}

func (f *failNthTx) InTx(ctx context.Context, fn func(ZaloBotTx) error) error {
	f.n++
	if f.n == f.failOn {
		return errFakeCommit
	}
	return f.fakeZaloStore.InTx(ctx, fn)
}

func TestSetWebhookNotConfiguredAndNoHost(t *testing.T) {
	r := newZBRig(t)
	out, err := r.uc.SetSharedZaloBotWebhook(context.Background(), zbActor, "")
	if err != nil || out.Outcome != domain.ZaloCallNotConfigured || r.zalo.calls != 0 {
		t.Fatalf("= %+v, %v", out, err)
	}
	r = configured(t)
	r.uc.webhookHost = ""
	calls := r.zalo.calls
	if _, err := r.uc.SetSharedZaloBotWebhook(context.Background(), zbActor, ""); !errors.Is(err, ErrZaloBotWebhookHostNotConfigured) {
		t.Fatalf("err = %v", err)
	}
	if r.zalo.calls != calls || r.store.st.pending != nil {
		t.Fatal("without a host, Zalo was called or a secret stored")
	}
}

func TestGetWebhookComparesWithTheURLCommsWouldSet(t *testing.T) {
	r := configured(t)
	if _, err := r.uc.SetSharedZaloBotWebhook(context.Background(), zbActor, ""); err != nil {
		t.Fatal(err)
	}
	r.zalo.info = zalobot.WebhookInfo{URL: "https://bot.api.vigov.vn/api/v1/zalo-bot-updates"}
	out, err := r.uc.GetSharedZaloBotWebhook(context.Background())
	if err != nil || !out.URLMatches || out.SecretSetBy != zbActor || out.SecretSetAt.IsZero() {
		t.Fatalf("= %+v, %v", out, err)
	}
	r.zalo.info = zalobot.WebhookInfo{URL: "https://elsewhere.example/hook"}
	if out, _ := r.uc.GetSharedZaloBotWebhook(context.Background()); out.URLMatches {
		t.Fatal("another URL reported as matching")
	}
	trail := len(r.store.st.trail)
	_, _ = r.uc.GetSharedZaloBotWebhook(context.Background())
	if len(r.store.st.trail) != trail {
		t.Fatal("a read wrote a trail entry")
	}
}

// ---- the cross-commune read -------------------------------------------------------------------------------

func TestListZaloBotCommuneStatsIsAudited(t *testing.T) {
	r := newZBRig(t)
	r.store.st.links[communeX] = []string{"CB-001", "CB-002"}
	r.store.st.channelOn[communeY] = true
	out, err := r.uc.ListZaloBotCommuneStats(context.Background(), zbActor, "")
	if err != nil || len(out) != 2 || out[0].TenantID != communeX || out[0].LinkedStaffCount != 2 ||
		!out[1].ChannelEnabled || out[1].LinkedStaffCount != 0 {
		t.Fatalf("= %+v, %v", out, err)
	}
	if a := actionsOf(r.store.st); !slices.Equal(a, []string{domain.ActionZaloBotCommunesRead}) {
		t.Fatalf("trail = %v", a)
	}
	if _, err := r.uc.ListZaloBotCommuneStats(context.Background(), "", ""); !errors.Is(err, domain.ErrZaloBotActorCode) {
		t.Fatalf("an unattributed cross-commune read: %v", err)
	}
}

func TestNewWebhookSecretIsCSPRNGShaped(t *testing.T) {
	a, err := newWebhookSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := newWebhookSecret()
	if len(a) != 43 || bytes.Equal(a, b) || strings.ContainsAny(string(a), "+/=") {
		t.Fatalf("%d characters, distinct=%v", len(a), !bytes.Equal(a, b))
	}
}
