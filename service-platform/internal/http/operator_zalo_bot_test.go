package http

// What these tests defend for `zalo-bots/shared…` (ADR 0074) — 401 / 403 / 200 per route live in
// TestGuardedRoutes (operator_routes_test.go); here:
//
//   - the token goes to comms verbatim and NOWHERE else: not a response, not a log line, not the
//     display row;
//   - the actor is the principal's business code and the edge's address, never the body's;
//   - every field refusal has its own code, decided before comms is called;
//   - comms' decisions keep their shapes: 409 + live_link_count, 422 + outcome;
//   - the bodyless POST/PUT pass the request guard with no body and no Content-Type;
//   - the communes table joins every registry commune with comms (absent = off/0/shared) and identity
//     (absent = 0), and shows the link count of the bot in use — never a sum.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// Fake material — says so in full (rule 8, forbidden #1).
const (
	zaloTokenFake   = "1234567890:zalo-bot-token-FAKE-NOT-REAL"
	zaloBotBodyFake = `{"token":"` + zaloTokenFake + `","bot_name":"  Bot nhắc việc ViGov  ","chat_url":"https://zalo.me/bot-FAKE"}`
	zaloKey         = "ops.zalo_bot.manage"
)

var (
	setAtFake  = time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)
	okOutcome  = commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_OK
	notConfigd = commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_NOT_CONFIGURED
)

// zaloBotFake is comms, absent. A nil answer field means the ordinary success answer.
type zaloBotFake struct {
	calls int
	err   error

	set      *commsv1.SetSharedZaloBotResponse
	get      *commsv1.GetSharedZaloBotResponse
	getErr   error
	check    *commsv1.CheckSharedZaloBotResponse
	whGet    *commsv1.GetSharedZaloBotWebhookResponse
	whSet    *commsv1.SetSharedZaloBotWebhookResponse
	stats    *commsv1.ListZaloBotCommuneStatsResponse
	sawSet   []*commsv1.SetSharedZaloBotRequest
	sawCheck []*commsv1.CheckSharedZaloBotRequest
	sawWhSet []*commsv1.SetSharedZaloBotWebhookRequest
	sawStats []*commsv1.ListZaloBotCommuneStatsRequest
	sawCtxT  []tenant.ID
}

func (f *zaloBotFake) hit(ctx context.Context) {
	f.calls++
	t, _ := tenant.From(ctx)
	f.sawCtxT = append(f.sawCtxT, t)
}

func botStatusFake() *commsv1.SharedZaloBotStatus {
	return &commsv1.SharedZaloBotStatus{SetAt: timestamppb.New(setAtFake), SetBy: opCodeFake, HasToken: true,
		BotName: "Bot nhắc việc ViGov", ChatUrl: "https://zalo.me/bot-FAKE"}
}

func (f *zaloBotFake) SetSharedZaloBot(ctx context.Context, in *commsv1.SetSharedZaloBotRequest, _ ...grpc.CallOption) (*commsv1.SetSharedZaloBotResponse, error) {
	f.hit(ctx)
	f.sawSet = append(f.sawSet, in)
	if f.err != nil {
		return nil, f.err
	}
	if f.set != nil {
		return f.set, nil
	}
	return &commsv1.SetSharedZaloBotResponse{Result: commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_SAVED,
		Bot: botStatusFake(), ZaloOutcome: okOutcome}, nil
}

func (f *zaloBotFake) GetSharedZaloBot(ctx context.Context, _ *commsv1.GetSharedZaloBotRequest, _ ...grpc.CallOption) (*commsv1.GetSharedZaloBotResponse, error) {
	f.hit(ctx)
	if f.err != nil {
		return nil, f.err
	}
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.get != nil {
		return f.get, nil
	}
	return &commsv1.GetSharedZaloBotResponse{Configured: true, Bot: botStatusFake(),
		LastCheck: &commsv1.SharedZaloBotCheck{CheckedAt: timestamppb.New(setAtFake), Outcome: okOutcome}}, nil
}

func (f *zaloBotFake) CheckSharedZaloBot(ctx context.Context, in *commsv1.CheckSharedZaloBotRequest, _ ...grpc.CallOption) (*commsv1.CheckSharedZaloBotResponse, error) {
	f.hit(ctx)
	f.sawCheck = append(f.sawCheck, in)
	if f.err != nil {
		return nil, f.err
	}
	if f.check != nil {
		return f.check, nil
	}
	return &commsv1.CheckSharedZaloBotResponse{Outcome: okOutcome, AccountName: "ViGov Bot",
		Check: &commsv1.SharedZaloBotCheck{CheckedAt: timestamppb.New(setAtFake), Outcome: okOutcome}}, nil
}

func (f *zaloBotFake) SetSharedZaloBotWebhook(ctx context.Context, in *commsv1.SetSharedZaloBotWebhookRequest, _ ...grpc.CallOption) (*commsv1.SetSharedZaloBotWebhookResponse, error) {
	f.hit(ctx)
	f.sawWhSet = append(f.sawWhSet, in)
	if f.err != nil {
		return nil, f.err
	}
	if f.whSet != nil {
		return f.whSet, nil
	}
	return &commsv1.SetSharedZaloBotWebhookResponse{Outcome: okOutcome, Url: "https://bot.api.example.invalid/api/v1/zalo-bot-updates",
		SetAt: timestamppb.New(setAtFake), SetBy: opCodeFake}, nil
}

func (f *zaloBotFake) GetSharedZaloBotWebhook(ctx context.Context, _ *commsv1.GetSharedZaloBotWebhookRequest, _ ...grpc.CallOption) (*commsv1.GetSharedZaloBotWebhookResponse, error) {
	f.hit(ctx)
	if f.err != nil {
		return nil, f.err
	}
	if f.whGet != nil {
		return f.whGet, nil
	}
	return &commsv1.GetSharedZaloBotWebhookResponse{Outcome: okOutcome, Url: "https://bot.api.example.invalid/api/v1/zalo-bot-updates",
		UrlMatches: true, SecretSetAt: timestamppb.New(setAtFake), SecretSetBy: opCodeFake}, nil
}

func (f *zaloBotFake) ListZaloBotCommuneStats(ctx context.Context, in *commsv1.ListZaloBotCommuneStatsRequest, _ ...grpc.CallOption) (*commsv1.ListZaloBotCommuneStatsResponse, error) {
	f.hit(ctx)
	f.sawStats = append(f.sawStats, in)
	if f.err != nil {
		return nil, f.err
	}
	if f.stats != nil {
		return f.stats, nil
	}
	return &commsv1.ListZaloBotCommuneStatsResponse{}, nil
}

// zaloLogFake is the display-row writer.
type zaloLogFake struct {
	acts   []domain.ZaloBotAct
	actors []domain.OperatorActor
	err    error
}

func (l *zaloLogFake) RecordZaloBotAct(_ context.Context, act domain.ZaloBotAct, by domain.OperatorActor) error {
	l.acts = append(l.acts, act)
	l.actors = append(l.actors, by)
	return l.err
}

func zaloHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.id.keys = []string{zaloKey}
	return h
}

func decode[T any](t *testing.T, body string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("not JSON: %v (%s)", err, body)
	}
	return v
}

// --- PUT zalo-bots/shared --------------------------------------------------------------------------------

func TestSetSharedZaloBotForwardsTokenAndNothingElse(t *testing.T) {
	h := zaloHarness(t)
	h.zalo.set = &commsv1.SetSharedZaloBotResponse{Result: commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_SAVED,
		Bot: botStatusFake(), ZaloOutcome: okOutcome, EndedLinkCount: 4}
	body := `{"token":"` + zaloTokenFake + `","bot_name":"  Bot nhắc việc ViGov  ","chat_url":" https://zalo.me/bot-FAKE ","expected_relink_count":4}`
	rec := h.do("PUT", "/api/v1/zalo-bots/shared", body, opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if len(h.zalo.sawSet) != 1 {
		t.Fatalf("comms called %d times", len(h.zalo.sawSet))
	}
	in := h.zalo.sawSet[0]
	if in.GetToken() != zaloTokenFake || in.GetBotName() != "Bot nhắc việc ViGov" || in.GetChatUrl() != "https://zalo.me/bot-FAKE" ||
		in.GetExpectedRelinkCount() != 4 {
		t.Errorf("request to comms: name=%q url=%q expected=%d token-verbatim=%v", in.GetBotName(), in.GetChatUrl(),
			in.GetExpectedRelinkCount(), in.GetToken() == zaloTokenFake)
	}
	if in.GetActorCode() != opCodeFake || in.GetActorIp() == "" {
		t.Errorf("actor = %q / %q, want the principal's code and the edge's address", in.GetActorCode(), in.GetActorIp())
	}
	if h.zalo.sawCtxT[0] != "" {
		t.Errorf("a commune travelled in ctx: %s", h.zalo.sawCtxT[0])
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	v := decode[map[string]any](t, rec.Body.String())
	if v["configured"] != true || v["ended_link_count"] != float64(4) || v["last_check"] == nil {
		t.Errorf("answer = %s", rec.Body)
	}
	bot, _ := v["bot"].(map[string]any)
	if bot["set_by"] != opCodeFake || bot["has_token"] != true || bot["set_at"] != "2026-10-11T03:00:00Z" {
		t.Errorf("bot = %v", bot)
	}
	if len(h.zlog.acts) != 1 || h.zlog.acts[0].Action != domain.ZaloBotActionTokenSet || h.zlog.acts[0].EndedLinkCount != 4 ||
		h.zlog.acts[0].Outcome != "thanh-cong" || h.zlog.actors[0].Code != opCodeFake {
		t.Errorf("display row = %+v by %+v", h.zlog.acts, h.zlog.actors)
	}
	for where, s := range map[string]string{"response": rec.Body.String(), "log": h.logs.String()} {
		if strings.Contains(s, zaloTokenFake) || strings.Contains(s, "zalo-bot-token-FAKE") {
			t.Errorf("the token reached the %s", where)
		}
	}
}

// The body cannot name the actor or anything else comms trusts: unknown fields are 400.
func TestSetSharedZaloBotActorIsNeverTheBody(t *testing.T) {
	h := zaloHarness(t)
	for _, extra := range []string{`"actor_code":"VH-99999"`, `"actor_ip":"198.51.100.1"`, `"tenant_id":"` + communeIDFake + `"`} {
		body := strings.TrimSuffix(zaloBotBodyFake, "}") + "," + extra + "}"
		if rec := h.do("PUT", "/api/v1/zalo-bots/shared", body, opCookie(t)); rec.Code != 400 {
			t.Errorf("%s: %d, want 400 invalid_body", extra, rec.Code)
		}
	}
	if h.zalo.calls != 0 {
		t.Fatalf("comms called %d times for refused bodies", h.zalo.calls)
	}
}

func TestSetSharedZaloBotFieldRefusals(t *testing.T) {
	long := strings.Repeat("a", 257)
	for name, c := range map[string]struct{ body, code string }{
		"empty token":        {`{"token":"","bot_name":"Bot","chat_url":"https://zalo.me/x"}`, "invalid_token"},
		"absent token":       {`{"bot_name":"Bot","chat_url":"https://zalo.me/x"}`, "invalid_token"},
		"token with space":   {`{"token":"abc def","bot_name":"Bot","chat_url":"https://zalo.me/x"}`, "invalid_token"},
		"token with slash":   {`{"token":"abc/def","bot_name":"Bot","chat_url":"https://zalo.me/x"}`, "invalid_token"},
		"token with percent": {`{"token":"abc%2F","bot_name":"Bot","chat_url":"https://zalo.me/x"}`, "invalid_token"},
		"token non-ascii":    {`{"token":"abcđ","bot_name":"Bot","chat_url":"https://zalo.me/x"}`, "invalid_token"},
		"token too long":     {`{"token":"` + long + `","bot_name":"Bot","chat_url":"https://zalo.me/x"}`, "invalid_token"},
		"token not a string": {`{"token":12,"bot_name":"Bot","chat_url":"https://zalo.me/x"}`, "invalid_body"},
		"blank name":         {`{"token":"abc","bot_name":"   ","chat_url":"https://zalo.me/x"}`, "invalid_bot_name"},
		"name too long":      {`{"token":"abc","bot_name":"` + strings.Repeat("ữ", 101) + `","chat_url":"https://zalo.me/x"}`, "invalid_bot_name"},
		"name control char":  {`{"token":"abc","bot_name":"Bot\u0007","chat_url":"https://zalo.me/x"}`, "invalid_bot_name"},
		"http url":           {`{"token":"abc","bot_name":"Bot","chat_url":"http://zalo.me/x"}`, "invalid_chat_url"},
		"userinfo url":       {`{"token":"abc","bot_name":"Bot","chat_url":"https://u:p@zalo.me/x"}`, "invalid_chat_url"},
		"relative url":       {`{"token":"abc","bot_name":"Bot","chat_url":"/x"}`, "invalid_chat_url"},
		"url too long":       {`{"token":"abc","bot_name":"Bot","chat_url":"https://zalo.me/` + strings.Repeat("x", 290) + `"}`, "invalid_chat_url"},
		"negative relink":    {`{"token":"abc","bot_name":"Bot","chat_url":"https://zalo.me/x","expected_relink_count":-1}`, "invalid_expected_relink_count"},
	} {
		t.Run(name, func(t *testing.T) {
			h := zaloHarness(t)
			rec := h.do("PUT", "/api/v1/zalo-bots/shared", c.body, opCookie(t))
			want := 422
			if c.code == "invalid_body" {
				want = 400
			}
			if rec.Code != want || decode[map[string]any](t, rec.Body.String())["code"] != c.code {
				t.Fatalf("%d %s, want %d %s", rec.Code, rec.Body, want, c.code)
			}
			if h.zalo.calls != 0 {
				t.Fatal("comms was called for a field refused here")
			}
		})
	}
}

func TestSetSharedZaloBotCommsDecisions(t *testing.T) {
	h := zaloHarness(t)
	h.zalo.set = &commsv1.SetSharedZaloBotResponse{Result: commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_RELINK_CONFIRMATION_REQUIRED,
		ZaloOutcome: okOutcome, LiveLinkCount: 7}
	rec := h.do("PUT", "/api/v1/zalo-bots/shared", zaloBotBodyFake, opCookie(t))
	v := decode[map[string]any](t, rec.Body.String())
	if rec.Code != 409 || v["code"] != "can-xac-nhan-ghep-lai" || v["live_link_count"] != float64(7) {
		t.Errorf("relink: %d %s", rec.Code, rec.Body)
	}
	if h.zalo.sawSet[0].GetExpectedRelinkCount() != 0 {
		t.Error("an absent expected_relink_count must travel as 0")
	}

	h.zalo.set = &commsv1.SetSharedZaloBotResponse{Result: commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_TOKEN_CHECK_FAILED,
		ZaloOutcome: commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_TOKEN_REJECTED}
	rec = h.do("PUT", "/api/v1/zalo-bots/shared", zaloBotBodyFake, opCookie(t))
	v = decode[map[string]any](t, rec.Body.String())
	if rec.Code != 422 || v["code"] != "token-khong-dung-duoc" || v["outcome"] != "token-bi-tu-choi" {
		t.Errorf("token refused: %d %s", rec.Code, rec.Body)
	}

	h.zalo.set = &commsv1.SetSharedZaloBotResponse{} // UNSPECIFIED: "not saved", fail closed
	if rec := h.do("PUT", "/api/v1/zalo-bots/shared", zaloBotBodyFake, opCookie(t)); rec.Code != 503 {
		t.Errorf("UNSPECIFIED result: %d, want 503", rec.Code)
	}
	if len(h.zlog.acts) != 0 {
		t.Errorf("display rows for acts that wrote nothing: %+v", h.zlog.acts)
	}
}

// SAVED is final: a failing display row or a failing re-read never turns it into an error.
func TestSetSharedZaloBotSavedSurvivesFollowUpFailures(t *testing.T) {
	h := zaloHarness(t)
	h.zlog.err = errors.New("db down (fake)")
	h.zalo.getErr = status.Error(codes.Unavailable, "x")
	rec := h.do("PUT", "/api/v1/zalo-bots/shared", zaloBotBodyFake, opCookie(t))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	v := decode[map[string]any](t, rec.Body.String())
	if v["bot"] == nil || v["last_check"] != nil || v["ended_link_count"] != float64(0) {
		t.Errorf("answer = %s", rec.Body)
	}
	if !strings.Contains(h.logs.String(), "zalo_bot.display_row_missing") {
		t.Error("the missing display row was not logged")
	}
	if strings.Contains(h.logs.String(), zaloTokenFake) {
		t.Error("the token reached the log")
	}
}

func TestZaloBotStatusMapping(t *testing.T) {
	for _, c := range []struct {
		code       codes.Code
		wantStatus int
		wantCode   string
	}{
		{codes.FailedPrecondition, 503, "zalo_bot_key_not_configured"},
		{codes.Unavailable, 503, "zalo_bot_unavailable"},
		{codes.DeadlineExceeded, 503, "zalo_bot_unavailable"},
		{codes.Unimplemented, 503, "zalo_bot_unavailable"},
		{codes.InvalidArgument, 422, "invalid_request"},
	} {
		t.Run(c.code.String(), func(t *testing.T) {
			h := zaloHarness(t)
			h.zalo.err = status.Error(c.code, "message naming "+zaloTokenFake)
			for _, rt := range []struct{ method, path, body string }{
				{"GET", "/api/v1/zalo-bots/shared", ""},
				{"PUT", "/api/v1/zalo-bots/shared", zaloBotBodyFake},
				{"POST", "/api/v1/zalo-bots/shared/check", ""},
				{"GET", "/api/v1/zalo-bots/shared/webhook", ""},
				{"PUT", "/api/v1/zalo-bots/shared/webhook", ""},
				{"GET", "/api/v1/zalo-bots/shared/communes", ""},
			} {
				rec := h.do(rt.method, rt.path, rt.body, opCookie(t))
				if rec.Code != c.wantStatus || decode[map[string]any](t, rec.Body.String())["code"] != c.wantCode {
					t.Errorf("%s %s: %d %s", rt.method, rt.path, rec.Code, rec.Body)
				}
			}
			if strings.Contains(h.logs.String(), zaloTokenFake) {
				t.Error("a status message reached the log")
			}
			if len(h.zlog.acts) != 0 {
				t.Errorf("display rows for failed calls: %+v", h.zlog.acts)
			}
		})
	}
}

// --- bodyless writes ---------------------------------------------------------------------------------------

// The console sends these with NO body and NO Content-Type (api.ts). The request guard admits exactly
// that, and the handler must not then ask for JSON.
func TestBodylessWritesAccepted(t *testing.T) {
	h := zaloHarness(t)
	hdr := http.Header{"Origin": []string{"https://" + operatorHostFake}}
	rec := h.doRaw("POST", "/api/v1/zalo-bots/shared/check", "", opCookie(t), hdr)
	v := decode[map[string]any](t, rec.Body.String())
	if rec.Code != 200 || v["outcome"] != "thanh-cong" || v["account_name"] != "ViGov Bot" {
		t.Errorf("check: %d %s", rec.Code, rec.Body)
	}
	rec = h.doRaw("PUT", "/api/v1/zalo-bots/shared/webhook", "", opCookie(t), hdr)
	v = decode[map[string]any](t, rec.Body.String())
	if rec.Code != 200 || v["outcome"] != "thanh-cong" || v["set_by"] != opCodeFake || v["url"] == "" {
		t.Errorf("webhook: %d %s", rec.Code, rec.Body)
	}
	if h.zalo.sawCheck[0].GetActorCode() != opCodeFake || h.zalo.sawWhSet[0].GetActorCode() != opCodeFake {
		t.Error("the actor is not the principal's business code")
	}
	if len(h.zlog.acts) != 2 || h.zlog.acts[0].Action != domain.ZaloBotActionChecked ||
		h.zlog.acts[1].Action != domain.ZaloBotActionWebhookSet || h.zlog.acts[1].WebhookURL == "" {
		t.Errorf("display rows = %+v", h.zlog.acts)
	}
}

func TestCheckAndWebhookOutcomes(t *testing.T) {
	h := zaloHarness(t)
	h.zalo.check = &commsv1.CheckSharedZaloBotResponse{Outcome: notConfigd}
	h.zalo.whSet = &commsv1.SetSharedZaloBotWebhookResponse{Outcome: notConfigd}
	rec := h.do("POST", "/api/v1/zalo-bots/shared/check", "", opCookie(t))
	if v := decode[map[string]any](t, rec.Body.String()); rec.Code != 200 || v["outcome"] != "chua-cau-hinh" {
		t.Errorf("check not configured: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do("PUT", "/api/v1/zalo-bots/shared/webhook", "", opCookie(t)); rec.Code != 200 {
		t.Errorf("webhook not configured: %d", rec.Code)
	}
	if len(h.zlog.acts) != 0 {
		t.Errorf("NOT_CONFIGURED wrote nothing in comms, so no display row: %+v", h.zlog.acts)
	}

	// A failed check: recorded by comms, so displayed; no account name.
	h.zalo.check = &commsv1.CheckSharedZaloBotResponse{Outcome: commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_RATE_LIMITED,
		AccountName: "must not show", Check: &commsv1.SharedZaloBotCheck{CheckedAt: timestamppb.New(setAtFake)}}
	rec = h.do("POST", "/api/v1/zalo-bots/shared/check", "", opCookie(t))
	if v := decode[map[string]any](t, rec.Body.String()); v["outcome"] != "gioi-han-tan-suat" || v["account_name"] != nil {
		t.Errorf("rate limited: %s", rec.Body)
	}
	// An ambiguous webhook result: displayed, no set_at.
	h.zalo.whSet = &commsv1.SetSharedZaloBotWebhookResponse{Outcome: commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_UNAVAILABLE,
		Url: "https://bot.api.example.invalid/api/v1/zalo-bot-updates", SetAt: timestamppb.New(setAtFake), SetBy: opCodeFake}
	rec = h.do("PUT", "/api/v1/zalo-bots/shared/webhook", "", opCookie(t))
	if v := decode[map[string]any](t, rec.Body.String()); v["outcome"] != "khong-kha-dung" || v["set_at"] != nil || v["url"] == "" {
		t.Errorf("ambiguous webhook: %s", rec.Body)
	}
	if len(h.zlog.acts) != 2 {
		t.Errorf("display rows = %+v", h.zlog.acts)
	}

	// UNSPECIFIED is never success.
	h.zalo.whGet = &commsv1.GetSharedZaloBotWebhookResponse{Url: "https://elsewhere.example.invalid", UrlMatches: true}
	rec = h.do("GET", "/api/v1/zalo-bots/shared/webhook", "", opCookie(t))
	if v := decode[map[string]any](t, rec.Body.String()); v["outcome"] != "khong-ro" || v["url_matches"] != false || v["url"] != "" {
		t.Errorf("unspecified webhook read: %s", rec.Body)
	}
}

func TestGetSharedZaloBot(t *testing.T) {
	h := zaloHarness(t)
	h.zalo.get = &commsv1.GetSharedZaloBotResponse{}
	rec := h.do("GET", "/api/v1/zalo-bots/shared", "", opCookie(t))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"configured":false}` {
		t.Errorf("never configured: %d %s", rec.Code, rec.Body)
	}
	h.zalo.get = &commsv1.GetSharedZaloBotResponse{Configured: true} // no bot: outside the contract
	if rec := h.do("GET", "/api/v1/zalo-bots/shared", "", opCookie(t)); rec.Code != 503 {
		t.Errorf("configured without bot: %d, want 503", rec.Code)
	}
	if len(h.zlog.acts) != 0 {
		t.Error("a read wrote a display row")
	}
}

// Dev without COMMS_GRPC_ADDR: 503 on every route, never a panic.
func TestZaloBotUnwired(t *testing.T) {
	hh := &operatorHandlers{d: OperatorDeps{Log: slog.Default()}}
	for name, fn := range map[string]http.HandlerFunc{
		"get": hh.getSharedZaloBot, "set": hh.setSharedZaloBot, "check": hh.checkSharedZaloBot,
		"webhook get": hh.getSharedZaloBotWebhook, "webhook set": hh.setSharedZaloBotWebhook, "communes": hh.listZaloBotCommunes,
	} {
		rec := httptest.NewRecorder()
		fn(rec, httptest.NewRequest("GET", "/", strings.NewReader(zaloBotBodyFake)))
		if rec.Code != 503 {
			t.Errorf("%s: %d, want 503", name, rec.Code)
		}
	}
}

// --- GET zalo-bots/shared/communes -------------------------------------------------------------------------

const (
	communeB = "01JD8ZQK9M3NPXR7TVWYB2C4EH"
	communeC = "01JD8ZQK9M3NPXR7TVWYB2C4EJ"
	communeX = "01JD8ZQK9M3NPXR7TVWYB2C4EK" // in comms, not in the registry
)

func TestZaloBotCommunesJoin(t *testing.T) {
	h := zaloHarness(t)
	h.reg.pages = [][]domain.Commune{
		{{ID: communeIDFake, Name: "Xã Thăng Bình"}, {ID: communeB, Name: "Xã Bình Lãnh"}},
		{{ID: communeC, Name: "Xã Bình Trị"}},
	}
	h.zalo.stats = &commsv1.ListZaloBotCommuneStatsResponse{Communes: []*commsv1.ZaloBotCommuneStats{
		// Own bot: the own-bot count, never the sum with the stale shared-bot count.
		{TenantId: communeIDFake, ChannelEnabled: true, LinkedStaffCount: 2, UsesOwnBot: true, OwnBotName: "Bot Thăng Bình", OwnBotLinkedStaffCount: 9},
		{TenantId: communeB, ChannelEnabled: true, LinkedStaffCount: 5, OwnBotLinkedStaffCount: 1},
		{TenantId: communeX, ChannelEnabled: true, LinkedStaffCount: 3},
	}}
	h.id.staffRes = &operatorclient.ListActiveStaffCountsResult{Outcome: operatorclient.OutcomeAccepted,
		Counts: map[string]uint32{communeIDFake: 20, communeB: 11, communeX: 4}}

	cookie := opCookie(t)
	rec := h.do("GET", "/api/v1/zalo-bots/shared/communes", "", cookie)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := decode[zaloBotCommuneListView](t, rec.Body.String())
	want := []zaloBotCommuneView{
		{TenantID: communeIDFake, CommuneName: "Xã Thăng Bình", ChannelEnabled: true, LinkedStaffCount: 9, UsesOwnBot: true,
			OwnBotName: "Bot Thăng Bình", TotalStaffCount: 20},
		{TenantID: communeB, CommuneName: "Xã Bình Lãnh", ChannelEnabled: true, LinkedStaffCount: 5, TotalStaffCount: 11},
		// Absent in comms AND identity: off, 0, shared bot, 0 total — still listed.
		{TenantID: communeC, CommuneName: "Xã Bình Trị"},
	}
	if len(got.Items) != len(want) {
		t.Fatalf("items = %+v", got.Items)
	}
	for i := range want {
		if got.Items[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got.Items[i], want[i])
		}
	}
	if !strings.Contains(rec.Body.String(), `"commune_name":"Xã Bình Trị","channel_enabled":false,"linked_staff_count":0,"uses_own_bot":false,"total_staff_count":0`) {
		t.Errorf("absent commune shape: %s", rec.Body)
	}
	// Identity is asked with the operator's own token and address; no commune anywhere.
	if len(h.id.sawStaffToken) != 1 || h.id.sawStaffToken[0] != cookie || h.id.sawStaffIP[0] == "" || h.id.sawStaffCtxT[0] != "" {
		t.Errorf("identity call: token-is-cookie=%v ip=%v ctx-tenant=%v", h.id.sawStaffToken[0] == cookie, h.id.sawStaffIP, h.id.sawStaffCtxT)
	}
	if s := h.zalo.sawStats[0]; s.GetActorCode() != opCodeFake || s.GetActorIp() == "" || h.zalo.sawCtxT[0] != "" {
		t.Errorf("comms call: actor %q ip %q ctx tenant %q", s.GetActorCode(), s.GetActorIp(), h.zalo.sawCtxT[0])
	}
	if len(h.zlog.acts) != 0 {
		t.Error("a read wrote a display row (comms and identity audit it)")
	}
}

func TestZaloBotCommunesRefusals(t *testing.T) {
	for name, c := range map[string]struct {
		res    *operatorclient.ListActiveStaffCountsResult
		err    error
		stats  *commsv1.ListZaloBotCommuneStatsResponse
		status int
		code   string
	}{
		"session ended at identity": {res: &operatorclient.ListActiveStaffCountsResult{Outcome: operatorclient.OutcomeSessionNotLive}, status: 401, code: "unauthorized"},
		"identity denies":           {res: &operatorclient.ListActiveStaffCountsResult{Outcome: operatorclient.OutcomePermissionDenied}, status: 403, code: "forbidden"},
		"identity down":             {err: errors.New("down (fake)"), status: 503, code: "staff_counts_unavailable"},
		"identity older":            {err: operatorclient.ErrNotSupported, status: 503, code: "staff_counts_unavailable"},
		"comms repeats a commune": {stats: &commsv1.ListZaloBotCommuneStatsResponse{Communes: []*commsv1.ZaloBotCommuneStats{
			{TenantId: communeIDFake}, {TenantId: communeIDFake}}}, status: 503, code: "zalo_bot_unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			h := zaloHarness(t)
			h.id.staffRes, h.id.staffErr, h.zalo.stats = c.res, c.err, c.stats
			rec := h.do("GET", "/api/v1/zalo-bots/shared/communes", "", opCookie(t))
			if rec.Code != c.status || decode[map[string]any](t, rec.Body.String())["code"] != c.code {
				t.Fatalf("%d %s, want %d %s", rec.Code, rec.Body, c.status, c.code)
			}
			if c.stats == nil && h.zalo.calls != 0 {
				t.Error("comms was asked although identity refused — its audited read would record a read nobody got")
			}
		})
	}
}
