package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/store/crosstenant"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// ZALO GROUP CHATS (ADR 0092) over the recording fake driver and the REAL stores: the webhook branches of
// both bots, the staff use case's validation and trail, the enqueue's fan-out, the sender's decisions and
// the bot switch. What PostgreSQL decides — the platform-wide chat key, the triggers, the dedupe — is
// store/zalo_group_link_pg_test.go's and zalo_group_pg_test.go's.

const (
	unitA       = "01JBPHAN00000000000000000A"
	unitB       = "01JBPHAN00000000000000000B"
	groupCodeID = "01JGCODE000000000000000001"
	groupLinkID = "01JGLINKNEW000000000000001"
)

func groupLinkCols() []string {
	return []string{"id", "bot_ref", "org_unit_id", "kinds", "linked_at", "linked_by"}
}

func groupCodeCols() []string {
	return []string{"id", "issued_by", "org_unit_id", "kinds", "expires_at", "failed_attempts"}
}

// fakeUnits is identity's org-unit half: live ids and names; down = every call fails.
type fakeUnits struct {
	live  map[string]bool
	names map[string]identityclient.OrgUnitName
	down  bool
	asked int
}

func (u *fakeUnits) LiveOrgUnits(_ context.Context, ids []string) (map[string]struct{}, error) {
	u.asked++
	if u.down {
		return nil, identityclient.ErrIdentityUnavailable
	}
	out := map[string]struct{}{}
	for _, id := range ids {
		if u.live[id] {
			out[id] = struct{}{}
		}
	}
	return out, nil
}

func (u *fakeUnits) OrgUnitNames(_ context.Context, ids []string) (map[string]identityclient.OrgUnitName, error) {
	if u.down {
		return nil, identityclient.ErrIdentityUnavailable
	}
	out := map[string]identityclient.OrgUnitName{}
	for _, id := range ids {
		if n, ok := u.names[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func unitsWithA() *fakeUnits {
	return &fakeUnits{live: map[string]bool{unitA: true}, names: map[string]identityclient.OrgUnitName{
		unitA: {Name: "Văn phòng", Standing: identityv1.RecordStanding_RECORD_STANDING_LIVE}}}
}

// settingsRow is a zalo_channel_setting row: on, with kinds.
func settingsRow(on bool, kinds string) ([]string, [][]driver.Value) {
	return []string{"e", "k", "qs", "qe", "sd", "rd", "ua", "ub", "dsd"},
		[][]driver.Value{{on, kinds, "21:00", "06:00", int64(1), int64(1), botClock, "CB-1", nil}}
}

func entries(f *sqlFake) []sqlStmt { return f.with("INSERT INTO audit_log") }

func entryDelta(t *testing.T, s sqlStmt) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(s.args[7].([]byte), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// --- the shared bot's webhook in a GROUP -------------------------------------------------------------------

// groupWebhookFake answers the shared bot's group linking: no commune bot, the group code locked open.
func groupWebhookFake(expires time.Time) *sqlFake {
	f := &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "FROM zalo_group_pairing_code") {
			return groupCodeCols(), [][]driver.Value{{groupCodeID, "CB-00777", unitA, "nhiem-vu.giao-moi,phan-anh.phan-cong",
				expires, int64(0)}}, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	return f
}

func newGroupWebhook(t *testing.T, f *sqlFake, r *fakeResolver, units OrgUnitNamer) *ZaloWebhook {
	t.Helper()
	w := newWebhook(t, f, r, fakeLimiter{allow: true}).WithOrgUnitNames(units)
	w.now = func() time.Time { return botClock }
	w.newID = func() (string, error) { return groupLinkID, nil }
	return w
}

func inGroup(s string) zalobot.Update { return inChat(zalobot.ChatTypeGroup, s) }

func TestGroupWebhook_SharedBotLinksInTheCodesCommuneAsTheIssuer(t *testing.T) {
	f := groupWebhookFake(botClock.Add(5 * time.Minute))
	r := &fakeResolver{groupMatches: []crosstenant.PairingCodeMatch{{TenantID: xaB, CodeID: groupCodeID}}}
	w := newGroupWebhook(t, f, r, unitsWithA())
	got := w.Handle(context.Background(), inGroup("@Bot ViGov abcd-2345"), "203.0.113.9")
	if got != domain.ZaloReplyGroupLinked(false, "Văn phòng") || !strings.Contains(got, "bộ phận Văn phòng") {
		t.Fatalf("→ %q", got)
	}
	if f.commits != 1 || r.asked != 0 {
		t.Fatalf("commits %d, personal lookups %d", f.commits, r.asked)
	}
	ins := f.with("INSERT INTO zalo_group_link")
	if len(ins) != 1 || ins[0].args[0] != string(xaB) || ins[0].args[2] != domain.SharedZaloBotRef ||
		ins[0].args[3] != "group-1" || ins[0].args[4] != unitA || ins[0].args[8] != "CB-00777" {
		t.Fatalf("link = %+v — the CODE's commune, the shared bot, the code's scope, the issuer", ins)
	}
	if len(f.with("UPDATE zalo_group_pairing_code SET used_at")) != 1 {
		t.Error("the code was not marked used")
	}
	aud := entries(f)
	if len(aud) != 1 || aud[0].args[0] != string(xaB) || aud[0].args[1] != "CB-00777" || aud[0].args[2] != "staff" ||
		aud[0].args[3] != "203.0.113.9" || aud[0].args[4] != domain.ActionLinkZaloGroup {
		t.Fatalf("entry = %v — the ISSUER is the actor (rule 6 inv 8)", aud)
	}
	if strings.Contains(string(aud[0].args[7].([]byte)), "group-1") {
		t.Error("the group chat id reached the trail")
	}
}

func TestGroupWebhook_CommuneWideConfirmationAndUnknownUnitName(t *testing.T) {
	if got := domain.ZaloReplyGroupLinked(true, ""); got != "Đã gắn nhóm với ViGov — nhận nhắc việc của toàn xã." {
		t.Errorf("commune-wide → %q", got)
	}
	f := groupWebhookFake(botClock.Add(5 * time.Minute))
	w := newGroupWebhook(t, f, &fakeResolver{groupMatches: []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: groupCodeID}}},
		&fakeUnits{down: true})
	if got := w.Handle(context.Background(), inGroup("ABCD2345"), ""); got != domain.ZaloReplyGroupLinked(false, "") ||
		!strings.Contains(got, "bộ phận đã chọn") {
		t.Fatalf("names down → %q — the link committed; the name is never guessed", got)
	}
}

func TestGroupWebhook_RefusalsTouchNothing(t *testing.T) {
	one := []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: groupCodeID}}
	two := []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: "g1"}, {TenantID: xaB, CodeID: "g2"}}
	cases := []struct {
		name string
		r    *fakeResolver
		want string
	}{
		{"no group code (or a personal one)", &fakeResolver{matches: one}, domain.ZaloReplyGroupCodeRefused},
		{"two communes", &fakeResolver{groupMatches: two}, domain.ZaloReplyGroupCodeRefused},
		{"live as a group anywhere", &fakeResolver{groupMatches: one, groupLive: true}, domain.ZaloReplyGroupAlreadyLinked},
	}
	for _, c := range cases {
		f := groupWebhookFake(botClock.Add(5 * time.Minute))
		w := newGroupWebhook(t, f, c.r, nil)
		if got := w.Handle(context.Background(), inGroup("ABCD2345"), ""); got != c.want {
			t.Errorf("%s → %q", c.name, got)
		}
		if f.begun != 0 || c.r.asked != 0 {
			t.Errorf("%s: tx %d, personal lookups %d — nothing consumed, no personal read", c.name, f.begun, c.r.asked)
		}
	}
}

func TestGroupWebhook_SharedBotRefusesWhereTheCommuneHasItsOwnBot(t *testing.T) {
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
		&fakeResolver{groupMatches: []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: groupCodeID}}},
		fakeLimiter{allow: true}, fakeBot{}, fakeSend{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if got := w.Handle(context.Background(), inGroup("ABCD2345"), ""); got != domain.ZaloReplyGroupCodeRefused {
		t.Fatalf("→ %q", got)
	}
	if f.begun != 0 {
		t.Error("a linking transaction opened in a commune that uses its own bot")
	}
}

func TestGroupWebhook_ExpiredCodeRollsBack(t *testing.T) {
	f := groupWebhookFake(botClock.Add(-time.Second))
	w := newGroupWebhook(t, f, &fakeResolver{groupMatches: []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: groupCodeID}}}, nil)
	if got := w.Handle(context.Background(), inGroup("ABCD2345"), ""); got != domain.ZaloReplyGroupCodeRefused {
		t.Fatalf("→ %q", got)
	}
	if f.rollbacks != 1 || f.commits != 0 || len(f.with("INSERT INTO zalo_group_link")) != 0 {
		t.Fatalf("an expired code linked: commits %d rollbacks %d", f.commits, f.rollbacks)
	}
}

// What PostgreSQL refuses at the insert has its own sentence, and the whole linking rolls back (the code
// stays open).
func TestGroupWebhook_DatabaseRefusalsMapToTheirSentences(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"race: live meanwhile": {&pgconn.PgError{Code: "23505", ConstraintName: "zalo_group_link_one_live_per_chat"},
			domain.ZaloReplyGroupAlreadyLinked},
		"paired to a person": {&pgconn.PgError{Code: "P0001",
			Message: "zalo_group_link: this chat is paired to a person; it is not linked as a group"},
			domain.ZaloReplyGroupPairedPersonally},
		"bot not live": {&pgconn.PgError{Code: "P0001", Message: "zalo_group_link: bot_ref is not the commune's live bot"},
			domain.ZaloReplyGroupCodeRefused},
		"anything else": {errors.New("disk full"), domain.ZaloReplyUnavailable},
	}
	for name, c := range cases {
		f := groupWebhookFake(botClock.Add(5 * time.Minute))
		f.exec = func(q string, _ []driver.Value) (int64, error) {
			if strings.Contains(q, "INSERT INTO zalo_group_link") {
				return 0, c.err
			}
			return 1, nil
		}
		w := newGroupWebhook(t, f, &fakeResolver{groupMatches: []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: groupCodeID}}}, nil)
		if got := w.Handle(context.Background(), inGroup("ABCD2345"), ""); got != c.want {
			t.Errorf("%s → %q", name, got)
		}
		if f.commits != 0 || f.rollbacks != 1 {
			t.Errorf("%s: commits %d rollbacks %d", name, f.commits, f.rollbacks)
		}
	}
}

func TestGroupWebhook_LimiterBoundsGroupCodes(t *testing.T) {
	r := &fakeResolver{groupMatches: []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: groupCodeID}}}
	w := newWebhook(t, &sqlFake{}, r, fakeLimiter{allow: false})
	if got := w.Handle(context.Background(), inGroup("ABCD2345"), ""); got != domain.ZaloReplyPairingLimited {
		t.Errorf("over the limit → %q", got)
	}
	w = newWebhook(t, &sqlFake{}, r, fakeLimiter{err: errors.New("redis down")})
	if got := w.Handle(context.Background(), inGroup("ABCD2345"), ""); got != domain.ZaloReplyUnavailable {
		t.Errorf("limiter down → %q", got)
	}
	if r.groupAsked != 0 {
		t.Error("a group code was looked up past (or without) the per-chat limit")
	}
}

// A GROUP code typed in a private chat: guidance, nothing consumed (owner, 11/10/2026).
func TestGroupWebhook_GroupCodeInAPrivateChatIsGuidance(t *testing.T) {
	f := &sqlFake{}
	r := &fakeResolver{groupMatches: []crosstenant.PairingCodeMatch{{TenantID: xaA, CodeID: groupCodeID}}}
	w := newWebhook(t, f, r, fakeLimiter{allow: true})
	if got := w.Handle(context.Background(), text("ABCD2345"), ""); got != domain.ZaloReplyPrivateGroupCode {
		t.Fatalf("→ %q", got)
	}
	if f.begun != 0 {
		t.Error("a group code typed in private opened a transaction")
	}
}

// --- a commune bot's webhook in a GROUP ---------------------------------------------------------------------

func TestGroupWebhook_CommuneBotLinksThroughItsOwnBot(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.groupCodeID = groupCodeID
	w := newCommuneWebhook(t, r, fakeLimiter{allow: true}, nil)
	w.newID = func() (string, error) { return groupLinkID, nil }
	if got := w.Handle(authed(t, r, w), inGroup("ABCD2345"), "203.0.113.9"); got != domain.ZaloReplyGroupLinked(true, "") {
		t.Fatalf("→ %q", got)
	}
	ins := r.f.with("INSERT INTO zalo_group_link")
	if len(ins) != 1 || ins[0].args[0] != string(xaA) || ins[0].args[2] != "commune:bot-111" {
		t.Fatalf("link = %v — the Host's commune, the AUTHENTICATED bot", ins)
	}
	look := r.f.with("SELECT id FROM zalo_group_pairing_code")
	if len(look) != 1 || look[0].args[0] != string(xaA) {
		t.Fatalf("lookup = %v — scoped to this commune", look)
	}
	if len(r.f.with("FROM zalo_pairing_code")) != 0 {
		t.Error("a personal code was looked up from a group")
	}
}

func TestGroupWebhook_CommuneBotRefusals(t *testing.T) {
	for name, set := range map[string]func(*botRig){
		"no group code here":   func(*botRig) {},
		"chat live as a group": func(r *botRig) { r.groupCodeID, r.groupLive = groupCodeID, true },
	} {
		r := newBotRig(t)
		r.withLiveBot(t, "bot-111")
		set(r)
		w := newCommuneWebhook(t, r, fakeLimiter{allow: true}, nil)
		want := domain.ZaloReplyGroupCodeRefused
		if r.groupLive {
			want = domain.ZaloReplyGroupAlreadyLinked
		}
		if got := w.Handle(authed(t, r, w), inGroup("ABCD2345"), ""); got != want {
			t.Errorf("%s → %q", name, got)
		}
		if r.f.begun != 0 {
			t.Errorf("%s: a transaction opened", name)
		}
	}
	// And a group code of this commune typed to the commune bot in private: guidance.
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.groupCodeID = groupCodeID
	w := newCommuneWebhook(t, r, fakeLimiter{allow: true}, nil)
	if got := w.Handle(authed(t, r, w), text("ABCD2345"), ""); got != domain.ZaloReplyPrivateGroupCode {
		t.Errorf("private group code → %q", got)
	}
}

// --- the staff side: issue, list, edit, unlink -------------------------------------------------------------

type groupRig struct {
	f     *sqlFake
	uc    *ZaloGroups
	units *fakeUnits
	ctx   context.Context
	// settings: the commune's channel; link: the live group the lock reads (nil = none).
	on    bool
	kinds string
	link  []driver.Value
}

func newGroupRig(t *testing.T) *groupRig {
	t.Helper()
	g := &groupRig{units: unitsWithA(), ctx: tenant.Into(context.Background(), xaA), on: true,
		kinds: "nhiem-vu.giao-moi,phan-anh.phan-cong,phan-anh.qua-han,nhiem-vu.sap-den-han"}
	g.link = []driver.Value{groupLinkID, "shared", "", "nhiem-vu.giao-moi", botClock.Add(-time.Hour), "CB-00777"}
	g.f = &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "FROM zalo_channel_setting"):
			c, rows := settingsRow(g.on, g.kinds)
			return c, rows, nil
		case strings.Contains(q, "FROM zalo_group_link"):
			if g.link == nil {
				return groupLinkCols(), nil, nil
			}
			return groupLinkCols(), [][]driver.Value{g.link}, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	db := sql.OpenDB(g.f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	g.uc = NewZaloGroups(kho, docstore.NewZaloGroupStore(kho), docstore.NewZaloLinkStore(kho),
		fakeSource{bot: ActiveZaloBot{Ref: domain.SharedZaloBotRef, Configured: true}}, g.units, noNames{}, nil)
	g.uc.now = func() time.Time { return botClock }
	g.uc.newID = func() (string, error) { return groupCodeID, nil }
	g.uc.drawCode = func() (string, error) { return "ABCD2345", nil }
	return g
}

func TestZaloGroups_IssueValidatesScopeAndKindsBeforeWriting(t *testing.T) {
	cases := []struct {
		name string
		in   GroupPairingCodeInput
		set  func(*groupRig)
		want error
	}{
		{"unit not a ULID", GroupPairingCodeInput{OrgUnitID: "bp-1", Kinds: []string{"nhiem-vu.giao-moi"}}, nil,
			domain.ErrZaloGroupOrgUnitShape},
		{"unit not live", GroupPairingCodeInput{OrgUnitID: unitB, Kinds: []string{"nhiem-vu.giao-moi"}}, nil,
			domain.ErrZaloGroupOrgUnitNotLive},
		{"identity down", GroupPairingCodeInput{OrgUnitID: unitA, Kinds: []string{"nhiem-vu.giao-moi"}},
			func(g *groupRig) { g.units.down = true }, ErrOrgUnitsUnavailable},
		{"no kind", GroupPairingCodeInput{}, nil, domain.ErrZaloGroupKindsRequired},
		{"a per-person kind", GroupPairingCodeInput{Kinds: []string{"nhiem-vu.sap-den-han"}}, nil, domain.ErrZaloGroupKindUnknown},
		{"a mention", GroupPairingCodeInput{Kinds: []string{"nhiem-vu.nhac-ten"}}, nil, domain.ErrZaloGroupKindUnknown},
		{"a group kind the commune has not switched on", GroupPairingCodeInput{Kinds: []string{"van-ban.chuyen-toi"}}, nil,
			domain.ErrZaloGroupKindNotEnabled},
		{"channel off", GroupPairingCodeInput{Kinds: []string{"nhiem-vu.giao-moi"}}, func(g *groupRig) { g.on = false },
			ErrZaloChannelOff},
	}
	for _, c := range cases {
		g := newGroupRig(t)
		if c.set != nil {
			c.set(g)
		}
		if _, err := g.uc.IssueGroupPairingCode(g.ctx, c.in, staffActor); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
		if g.f.begun != 0 {
			t.Errorf("%s: a transaction opened", c.name)
		}
	}
}

func TestZaloGroups_IssueStoresTheHashAndScopeWithItsEntry(t *testing.T) {
	g := newGroupRig(t)
	v, err := g.uc.IssueGroupPairingCode(g.ctx, GroupPairingCodeInput{OrgUnitID: strings.ToLower(unitA),
		Kinds: []string{"phan-anh.phan-cong", "nhiem-vu.giao-moi", "nhiem-vu.giao-moi"}}, staffActor)
	if err != nil || v.Code != "ABCD2345" || !v.ExpiresAt.Equal(botClock.Add(domain.PairingCodeTTL)) {
		t.Fatalf("= %+v, %v", v, err)
	}
	ins := g.f.with("INSERT INTO zalo_group_pairing_code")
	if len(ins) != 1 || ins[0].args[0] != string(xaA) || ins[0].args[2] != staffActor.ID || ins[0].args[3] != unitA ||
		ins[0].args[4] != "nhiem-vu.giao-moi,phan-anh.phan-cong" {
		t.Fatalf("insert = %+v — the session's code, the unit upper-cased, the kinds canonical", ins)
	}
	if string(ins[0].args[5].([]byte)) == "ABCD2345" {
		t.Fatal("the code itself was stored")
	}
	aud := entries(g.f)
	if len(aud) != 1 || aud[0].args[4] != domain.ActionIssueZaloGroupPairingCode || aud[0].args[1] != staffActor.ID ||
		strings.Contains(string(aud[0].args[7].([]byte)), "ABCD2345") {
		t.Fatalf("entry = %v", aud)
	}
}

func TestZaloGroups_IssueRefusedWithoutABot(t *testing.T) {
	g := newGroupRig(t)
	g.uc.bots = fakeSource{}
	if _, err := g.uc.IssueGroupPairingCode(g.ctx, GroupPairingCodeInput{Kinds: []string{"nhiem-vu.giao-moi"}},
		staffActor); !errors.Is(err, ErrZaloBotNotConfigured) {
		t.Fatalf("err = %v", err)
	}
}

func TestZaloGroups_ListNamesUnitsAndIssuers(t *testing.T) {
	g := newGroupRig(t)
	g.link = []driver.Value{groupLinkID, "shared", unitA, "nhiem-vu.giao-moi", botClock, "CB-00777"}
	list, err := g.uc.Groups(g.ctx)
	if err != nil || len(list) != 1 || list[0].OrgUnitName != "Văn phòng" || list[0].OrgUnitRemoved ||
		list[0].LinkedBy != "CB-00777" {
		t.Fatalf("= %+v, %v", list, err)
	}
	g.units.down = true
	if _, err := g.uc.Groups(g.ctx); !errors.Is(err, ErrOrgUnitsUnavailable) {
		t.Fatalf("identity down → %v — never a page of blanks", err)
	}
}

func TestZaloGroups_EditWritesBeforeAndAfter(t *testing.T) {
	g := newGroupRig(t)
	if err := g.uc.EditGroup(g.ctx, groupLinkID, ZaloGroupPatch{}, staffActor); !errors.Is(err, domain.ErrZaloGroupNothingToChange) {
		t.Fatalf("empty patch → %v", err)
	}
	// The same values: nothing written, nothing recorded.
	if err := g.uc.EditGroup(g.ctx, groupLinkID, ZaloGroupPatch{OrgUnitSet: true, Kinds: []string{"nhiem-vu.giao-moi"}},
		staffActor); err != nil {
		t.Fatal(err)
	}
	if len(g.f.with("UPDATE zalo_group_link")) != 0 || len(entries(g.f)) != 0 {
		t.Fatal("a no-op edit wrote")
	}
	if err := g.uc.EditGroup(g.ctx, groupLinkID, ZaloGroupPatch{OrgUnitSet: true, OrgUnitID: unitA}, staffActor); err != nil {
		t.Fatal(err)
	}
	up := g.f.with("UPDATE zalo_group_link SET org_unit_id")
	if len(up) != 1 || up[0].args[0] != string(xaA) || up[0].args[2] != unitA || up[0].args[3] != "nhiem-vu.giao-moi" {
		t.Fatalf("update = %+v — scope moved, kinds kept", up)
	}
	d := entryDelta(t, entries(g.f)[0])
	if d["truoc"].(map[string]any)["bo_phan"] != "" || d["sau"].(map[string]any)["bo_phan"] != unitA {
		t.Fatalf("delta = %v", d)
	}
	// A unit not live, a kind outside: refused before the transaction.
	before := g.f.begun
	if err := g.uc.EditGroup(g.ctx, groupLinkID, ZaloGroupPatch{OrgUnitSet: true, OrgUnitID: unitB}, staffActor); !errors.Is(err,
		domain.ErrZaloGroupOrgUnitNotLive) {
		t.Errorf("unit not live → %v", err)
	}
	if err := g.uc.EditGroup(g.ctx, groupLinkID, ZaloGroupPatch{Kinds: []string{"ban-tin-tuan"}}, staffActor); !errors.Is(err,
		domain.ErrZaloGroupKindUnknown) {
		t.Errorf("digest kind → %v", err)
	}
	if g.f.begun != before {
		t.Error("a refused edit opened a transaction")
	}
	g.link = nil
	if err := g.uc.EditGroup(g.ctx, groupLinkID, ZaloGroupPatch{Kinds: []string{"nhiem-vu.giao-moi"}}, staffActor); !errors.Is(err,
		docstore.ErrZaloGroupNotFound) {
		t.Errorf("not live → %v", err)
	}
}

func TestZaloGroups_UnlinkIsSoftWithAReasonAndAnEntry(t *testing.T) {
	g := newGroupRig(t)
	if err := g.uc.UnlinkGroup(g.ctx, groupLinkID, "  ", staffActor); !errors.Is(err, domain.ErrZaloGroupUnlinkReason) {
		t.Fatalf("no reason → %v", err)
	}
	if err := g.uc.UnlinkGroup(g.ctx, groupLinkID, "Nhóm giải tán", staffActor); err != nil {
		t.Fatal(err)
	}
	end := g.f.with("UPDATE zalo_group_link SET unlinked_at")
	if len(end) != 1 || end[0].args[3] != staffActor.ID || end[0].args[4] != "Nhóm giải tán" {
		t.Fatalf("end = %+v", end)
	}
	aud := entries(g.f)
	if len(aud) != 1 || aud[0].args[4] != domain.ActionEndZaloGroupLink || aud[0].args[5] != domain.ZaloGroupSubject(groupLinkID) {
		t.Fatalf("entry = %v", aud)
	}
	if len(g.f.with("DELETE")) != 0 {
		t.Fatal("a hard delete")
	}
}

// --- the enqueue's fan-out ---------------------------------------------------------------------------------

func TestZaloGroupMatches(t *testing.T) {
	groups := []domain.ZaloGroupTarget{
		{ID: "wide", Kinds: []string{"nhiem-vu.giao-moi"}},
		{ID: "unit-a", OrgUnitID: unitA, Kinds: []string{"nhiem-vu.giao-moi", "phan-anh.phan-cong"}},
		{ID: "unit-b", OrgUnitID: unitB, Kinds: []string{"nhiem-vu.giao-moi"}},
	}
	n := domain.GroupNotice{Kind: "nhiem-vu.giao-moi", HoldingOrgUnitID: unitA, Posting: domain.GroupPostingAllowed}
	if got := domain.ZaloGroupMatches(n, groups); strings.Join(got, ",") != "wide,unit-a" {
		t.Errorf("unit A → %v", got)
	}
	n.HoldingOrgUnitID = ""
	if got := domain.ZaloGroupMatches(n, groups); strings.Join(got, ",") != "wide" {
		t.Errorf("no unit → %v — empty is never every unit", got)
	}
	n.Kind = "phan-anh.phan-cong"
	if got := domain.ZaloGroupMatches(n, groups); len(got) != 0 {
		t.Errorf("a kind the wide group lacks → %v", got)
	}
	for _, p := range []domain.GroupPosting{domain.GroupPostingUnspecified, domain.GroupPostingRestricted} {
		n := domain.GroupNotice{Kind: "nhiem-vu.giao-moi", Posting: p}
		if got := domain.ZaloGroupMatches(n, groups); len(got) != 0 {
			t.Errorf("posting %v → %v", p, got)
		}
	}
	if got := domain.ZaloGroupMatches(domain.GroupNotice{Kind: "nhiem-vu.sap-den-han", Posting: domain.GroupPostingAllowed},
		[]domain.ZaloGroupTarget{{ID: "x", Kinds: []string{"nhiem-vu.sap-den-han"}}}); len(got) != 0 {
		t.Errorf("a digest kind reached a group: %v", got)
	}
}

func groupNotices() []domain.NotificationDelivery {
	return []domain.NotificationDelivery{
		{IdempotencyKey: "task_assigned:01JA", Kind: domain.ZaloKindTaskAssigned, RecipientCodes: []string{"CB-1", "CB-2"},
			Title: "Việc mới NV-1", Link: "/nhiem-vu/1", HoldingOrgUnitID: unitA, GroupPosting: domain.GroupPostingAllowed},
		{IdempotencyKey: "petition_assigned:01JB", Kind: domain.ZaloKindPetitionAssigned, RecipientCodes: []string{"CB-3"},
			Title: "Phiếu PA-1", HoldingOrgUnitID: unitB, GroupPosting: domain.GroupPostingRestricted},
		{IdempotencyKey: "task_assigned:01JC", Kind: domain.ZaloKindTaskAssigned, RecipientCodes: []string{"CB-4"},
			Title: "Việc mới NV-2", GroupPosting: domain.GroupPostingUnspecified},
	}
}

func TestDeliver_QueuesOneGroupRowPerNoticeAndMatchingGroupInItsOwnSavepoint(t *testing.T) {
	f := &sqlFake{
		exec: func(q string, args []driver.Value) (int64, error) {
			if strings.Contains(q, "INSERT INTO staff_notification") {
				return insertTuples(q), nil
			}
			if strings.Contains(q, "INSERT INTO zalo_group_delivery") {
				return int64(len(args[1].([]string))), nil
			}
			return 1, nil
		},
		query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
			if strings.Contains(q, "FROM zalo_group_link") {
				return groupLinkCols(), [][]driver.Value{
					{"g-wide", "shared", "", "nhiem-vu.giao-moi,phan-anh.phan-cong", botClock, "CB-9"},
					{"g-a", "shared", unitA, "nhiem-vu.giao-moi", botClock, "CB-9"},
					{"g-b", "shared", unitB, "phan-anh.phan-cong", botClock, "CB-9"},
				}, nil
			}
			return nil, nil, fmt.Errorf("fake: no answer for %q", q)
		},
	}
	uc, _, ctx := newInboxUseCase(t, f)
	db := sql.OpenDB(f)
	t.Cleanup(func() { db.Close() })
	uc.WithZaloGroupOutbox(docstore.NewZaloGroupStore(store.New(db)))

	if _, err := uc.Deliver(ctx, groupNotices(), jobActor); err != nil {
		t.Fatal(err)
	}
	if f.commits != 1 || len(f.with("SAVEPOINT zalo_group_delivery_enqueue")) == 0 {
		t.Fatalf("commits %d — the group enqueue must share the bell's transaction, under its savepoint", f.commits)
	}
	ins := f.with("INSERT INTO zalo_group_delivery")
	if len(ins) != 1 {
		t.Fatalf("inserts = %d", len(ins))
	}
	links, keys := ins[0].args[2].([]string), ins[0].args[3].([]string)
	// ONE row per (notice, group) — two recipients still one row; RESTRICTED and UNSPECIFIED none.
	if strings.Join(links, ",") != "g-wide,g-a" || keys[0] != "task_assigned:01JA" || keys[1] != "task_assigned:01JA" {
		t.Fatalf("rows = %v %v", links, keys)
	}
	if ins[0].args[5].([]string)[0] != "Việc mới NV-1" || ins[0].args[0] != string(xaA) {
		t.Fatalf("snapshot = %v", ins[0].args)
	}
	d := entryDelta(t, entries(f)[0])
	if d["zalo_nhom_xep_hang"] != float64(2) || d["zalo_nhom_han_che"] != float64(1) {
		t.Fatalf("delta = %v", d)
	}
	if strings.Contains(string(entries(f)[0].args[7].([]byte)), "NV-1") {
		t.Error("the title reached the trail")
	}
}

func TestDeliver_GroupEnqueueFailureStillDeliversTheBell(t *testing.T) {
	f := &sqlFake{
		exec: func(q string, _ []driver.Value) (int64, error) {
			if strings.Contains(q, "INSERT INTO staff_notification") {
				return insertTuples(q), nil
			}
			return 1, nil
		},
		query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
			return nil, nil, errors.New("zalo_group_link broken")
		},
	}
	uc, _, ctx := newInboxUseCase(t, f)
	db := sql.OpenDB(f)
	t.Cleanup(func() { db.Close() })
	uc.WithZaloGroupOutbox(docstore.NewZaloGroupStore(store.New(db)))
	if _, err := uc.Deliver(ctx, groupNotices(), jobActor); err != nil {
		t.Fatalf("a group failure failed the bell: %v", err)
	}
	if f.commits != 1 || len(f.with("ROLLBACK TO SAVEPOINT zalo_group_delivery_enqueue")) != 1 {
		t.Fatalf("commits %d — the failed group enqueue must roll back to its own savepoint", f.commits)
	}
	if d := entryDelta(t, entries(f)[0]); d["zalo_nhom_loi_xep_hang"] != true {
		t.Fatalf("delta = %v", d)
	}
}

// A pure retry (every recipient already had the key) writes nothing — no group row either.
func TestDeliver_RetryQueuesNoGroupRow(t *testing.T) {
	f := &sqlFake{exec: func(q string, _ []driver.Value) (int64, error) {
		if strings.Contains(q, "INSERT INTO staff_notification") {
			return 0, nil
		}
		return 1, nil
	}}
	uc, _, ctx := newInboxUseCase(t, f)
	db := sql.OpenDB(f)
	t.Cleanup(func() { db.Close() })
	uc.WithZaloGroupOutbox(docstore.NewZaloGroupStore(store.New(db)))
	if _, err := uc.Deliver(ctx, groupNotices(), jobActor); err != nil {
		t.Fatal(err)
	}
	if len(f.with("zalo_group")) != 0 {
		t.Fatal("a retry touched the group outbox")
	}
}

// --- the sender ---------------------------------------------------------------------------------------------

func groupDueCols() []string {
	return []string{"id", "group_link_id", "kind", "attempts", "title", "body", "link", "live", "bot_ref", "kinds", "chat_id"}
}

// runGroups runs one tick for commune A (own bot "commune:bot-a") with the given owed group rows.
func runGroups(t *testing.T, now time.Time, settingsOn bool, rows ...[]driver.Value) (*sqlFake, *recordingSend) {
	t.Helper()
	f := &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "FROM zalo_group_delivery d"):
			return groupDueCols(), rows, nil
		case strings.Contains(q, "FROM zalo_delivery d"):
			return []string{"id", "staff_code", "kind", "attempts", "title", "body", "link", "due_soon_items"}, nil, nil
		case strings.Contains(q, "FROM zalo_channel_setting"):
			c, r := settingsRow(settingsOn, "nhiem-vu.giao-moi,phan-anh.qua-han")
			return c, r, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	send := &recordingSend{}
	z, err := NewZaloDispatcher(ZaloDispatcherDeps{DB: kho, Repo: docstore.NewZaloLinkStore(kho), Locks: oneCommuneLocks{},
		Registry: activeRegistry{}, Bots: perCommuneSource{}, Send: send, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	z.now = func() time.Time { return now }
	z.Tick(context.Background())
	return f, send
}

func groupDue(id, kind string, live bool, botRef, kinds string) []driver.Value {
	return []driver.Value{id, "g-1", kind, int64(0), "Việc mới NV-1", "", "/nhiem-vu/1", live, botRef, kinds, "group-chat-1"}
}

func TestGroupDispatcher_SendsThroughTheCommunesBotToTheGroup(t *testing.T) {
	f, send := runGroups(t, leadClock, true, groupDue("d1", "nhiem-vu.giao-moi", true, "commune:bot-a", "nhiem-vu.giao-moi"))
	if len(send.chats) != 1 || send.chats[0] != "group-chat-1" || send.tokens[0] != "token-of-A" {
		t.Fatalf("sent %v with %v", send.chats, send.tokens)
	}
	if len(f.with("UPDATE zalo_group_delivery SET status = 'da-gui'")) != 1 ||
		len(f.with("UPDATE zalo_group_delivery SET attempts = attempts + 1")) != 1 {
		t.Fatal("the row was not leased then marked sent")
	}
	for _, e := range entries(f) {
		if strings.Contains(string(e.args[7].([]byte)), "group-chat-1") || strings.Contains(string(e.args[7].([]byte)), "NV-1") {
			t.Fatal("a chat id or the text reached the trail")
		}
	}
}

func TestGroupDispatcher_SkipsWithTheirReasons(t *testing.T) {
	cases := map[string]struct {
		on   bool
		row  []driver.Value
		want string
	}{
		"channel off":         {false, groupDue("d", "nhiem-vu.giao-moi", true, "commune:bot-a", "nhiem-vu.giao-moi"), "kenh-tat"},
		"commune kind off":    {true, groupDue("d", "phan-anh.phan-cong", true, "commune:bot-a", "phan-anh.phan-cong"), "loai-tat"},
		"group dropped kind":  {true, groupDue("d", "nhiem-vu.giao-moi", true, "commune:bot-a", "phan-anh.qua-han"), "loai-tat"},
		"link ended":          {true, groupDue("d", "nhiem-vu.giao-moi", false, "commune:bot-a", "nhiem-vu.giao-moi"), "chua-lien-ket"},
		"link of another bot": {true, groupDue("d", "nhiem-vu.giao-moi", true, "shared", "nhiem-vu.giao-moi"), "chua-lien-ket"},
	}
	for name, c := range cases {
		f, send := runGroups(t, leadClock, c.on, c.row)
		skip := f.with("UPDATE zalo_group_delivery SET status = 'bo-qua'")
		if len(skip) != 1 || skip[0].args[2] != c.want || len(send.chats) != 0 {
			t.Errorf("%s: skips %v, sent %v", name, skip, send.chats)
		}
	}
}

func TestGroupDispatcher_QuietHoursPostpone(t *testing.T) {
	night := time.Date(2026, 10, 8, 16, 0, 0, 0, time.UTC) // 23:00 in Vietnam
	f, send := runGroups(t, night, true, groupDue("d1", "nhiem-vu.giao-moi", true, "commune:bot-a", "nhiem-vu.giao-moi"))
	post := f.with("UPDATE zalo_group_delivery SET next_attempt_at")
	if len(post) != 1 || len(send.chats) != 0 {
		t.Fatalf("postpone %v sent %v", post, send.chats)
	}
	if want := time.Date(2026, 10, 8, 23, 0, 0, 0, time.UTC); !post[0].args[2].(time.Time).Equal(want) {
		t.Errorf("postponed to %v, want the end of the window %v", post[0].args[2], want)
	}
}

// --- the bot switch ends group links too --------------------------------------------------------------------

func TestCommuneBotAdoptEndsSharedGroupLinksWithAnEntryEach(t *testing.T) {
	r := newBotRig(t)
	r.endedGroups = [][]driver.Value{{"01JGLINKOLD000000000000001", "shared", unitA, "nhiem-vu.giao-moi", botClock.Add(-time.Hour),
		"CB-00777"}}
	out, err := r.uc.Set(r.ctx, setInput(zbToken), staffActor)
	if err != nil || out.EndedGroupCount != 1 || out.EndedLinkCount != 0 {
		t.Fatalf("= %+v, %v", out, err)
	}
	end := r.f.with("UPDATE zalo_group_link SET unlinked_at")
	if len(end) != 1 || end[0].args[0] != string(xaA) || end[0].args[1] != domain.SharedZaloBotRef ||
		end[0].args[3] != staffActor.ID || end[0].args[4] != domain.ZaloGroupEndedByBotChange {
		t.Fatalf("end = %+v", end)
	}
	if r.f.commits != 1 || !strings.Contains(strings.Join(entryActions(r.f), ","), domain.ActionEndZaloGroupLink) {
		t.Fatalf("commits %d, entries %v", r.f.commits, entryActions(r.f))
	}
	if i, j := indexOfStmt(r.f, "UPDATE zalo_group_link SET unlinked_at"), indexOfStmt(r.f, "INSERT INTO zalo_commune_bot"); i < 0 || j < i {
		t.Error("the group links must end in the same transaction, before the new bot row")
	}
}

func TestCommuneBotRetireEndsItsGroupLinks(t *testing.T) {
	r := newBotRig(t)
	r.withLiveBot(t, "bot-111")
	r.endedGroups = [][]driver.Value{{"01JGLINKOLD000000000000001", "commune:bot-111", "", "nhiem-vu.giao-moi", botClock,
		"CB-00777"}}
	out, err := r.uc.Retire(r.ctx, "Quay về bot chung", staffActor)
	if err != nil || !out.Retired || out.EndedGroupCount != 1 {
		t.Fatalf("= %+v, %v", out, err)
	}
	end := r.f.with("UPDATE zalo_group_link SET unlinked_at")
	if len(end) != 1 || end[0].args[1] != "commune:bot-111" {
		t.Fatalf("end = %+v", end)
	}
	r.groupCount = 2
	v, err := r.uc.Current(r.ctx)
	if err != nil || v.LiveGroupCount != 2 {
		t.Fatalf("current = %+v %v", v, err)
	}
}
