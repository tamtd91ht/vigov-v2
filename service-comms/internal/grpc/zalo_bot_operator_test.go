package grpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/vihat/vigov/core/crypto"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

const fakeToken = "123456789:FAKE-zalo-bot-token-NOT-REAL"

// fakeZaloBotUC answers what the test sets and records the token it was handed.
type fakeZaloBotUC struct {
	err       error
	set       app.SetSharedZaloBotOutput
	bot       domain.SharedZaloBot
	found     bool
	check     app.CheckOutput
	webhook   app.SetWebhookOutput
	getHook   app.GetWebhookOutput
	stats     []domain.ZaloBotCommuneStat
	gotToken  string
	gotActor  string
	gotExpect int
}

func (f *fakeZaloBotUC) SetSharedZaloBot(_ context.Context, in app.SetSharedZaloBotInput) (app.SetSharedZaloBotOutput, error) {
	f.gotToken, f.gotActor, f.gotExpect = string(in.Token), in.ActorCode, in.ExpectedRelinkCount
	return f.set, f.err
}
func (f *fakeZaloBotUC) GetSharedZaloBot(context.Context) (domain.SharedZaloBot, bool, error) {
	return f.bot, f.found, f.err
}
func (f *fakeZaloBotUC) CheckSharedZaloBot(context.Context, string, string) (app.CheckOutput, error) {
	return f.check, f.err
}
func (f *fakeZaloBotUC) SetSharedZaloBotWebhook(context.Context, string, string) (app.SetWebhookOutput, error) {
	return f.webhook, f.err
}
func (f *fakeZaloBotUC) GetSharedZaloBotWebhook(context.Context) (app.GetWebhookOutput, error) {
	return f.getHook, f.err
}
func (f *fakeZaloBotUC) ListZaloBotCommuneStats(context.Context, string, string) ([]domain.ZaloBotCommuneStat, error) {
	return f.stats, f.err
}

func zbServer(uc ZaloBotOperator) (*ZaloBotServer, *bytes.Buffer) {
	var buf bytes.Buffer
	return NewZaloBotServer(uc, slog.New(slog.NewTextHandler(&buf, nil))), &buf
}

var setReq = &commsv1.SetSharedZaloBotRequest{Token: fakeToken, BotName: "Bot", ChatUrl: "https://zalo.me/1",
	ActorCode: "VH-00001", ActorIp: "10.0.0.5", ExpectedRelinkCount: 3}

func TestZaloBotStatusTable(t *testing.T) {
	cases := map[error]codes.Code{
		domain.ErrZaloBotActorCode: codes.InvalidArgument,
		domain.ErrZaloBotToken:     codes.InvalidArgument,
		fmt.Errorf("zalo bot: seal the token: %w", crypto.ErrNotConfigured): codes.FailedPrecondition,
		app.ErrZaloBotWebhookHostNotConfigured:                              codes.FailedPrecondition,
		errors.New("platformstore: commit: connection reset"):               codes.Unavailable,
	}
	for cause, want := range cases {
		s, logs := zbServer(&fakeZaloBotUC{err: cause})
		_, err := s.SetSharedZaloBot(context.Background(), setReq)
		if status.Code(err) != want {
			t.Errorf("%v: code = %v, want %v", cause, status.Code(err), want)
		}
		// Neither the status nor the log carries the token, ever.
		if strings.Contains(err.Error(), fakeToken) || strings.Contains(logs.String(), fakeToken) {
			t.Errorf("%v: the token leaked: %q / %q", cause, err, logs.String())
		}
	}
}

// The request's String() prints the token — the server must never log the request.
func TestZaloBotServerNeverLogsTheRequest(t *testing.T) {
	uc := &fakeZaloBotUC{err: errors.New("store down")}
	s, logs := zbServer(uc)
	_, _ = s.SetSharedZaloBot(context.Background(), setReq)
	if strings.Contains(logs.String(), fakeToken) || strings.Contains(logs.String(), "FAKE-zalo") {
		t.Fatalf("logged: %s", logs.String())
	}
	if uc.gotToken != fakeToken || uc.gotActor != "VH-00001" || uc.gotExpect != 3 {
		t.Fatalf("the use case got %q %q %d", uc.gotToken, uc.gotActor, uc.gotExpect)
	}
}

func TestSetSharedZaloBotTranslatesEachResult(t *testing.T) {
	at := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	s, _ := zbServer(&fakeZaloBotUC{set: app.SetSharedZaloBotOutput{Result: app.SetSaved, ZaloOutcome: domain.ZaloCallOK,
		EndedLinkCount: 2, Bot: domain.SharedZaloBot{BotName: "Bot", ChatURL: "https://zalo.me/1", SetAt: at, SetBy: "VH-00001"}}})
	res, err := s.SetSharedZaloBot(context.Background(), setReq)
	if err != nil || res.GetResult() != commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_SAVED ||
		res.GetEndedLinkCount() != 2 || !res.GetBot().GetHasToken() || res.GetBot().GetSetBy() != "VH-00001" ||
		res.GetZaloOutcome() != commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_OK {
		t.Fatalf("= %v, %v", res, err)
	}
	if strings.Contains(res.String(), fakeToken) {
		t.Fatal("the response carries the token")
	}

	s, _ = zbServer(&fakeZaloBotUC{set: app.SetSharedZaloBotOutput{Result: app.SetRelinkConfirmationRequired,
		ZaloOutcome: domain.ZaloCallOK, LiveLinkCount: 7}})
	res, _ = s.SetSharedZaloBot(context.Background(), setReq)
	if res.GetResult() != commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_RELINK_CONFIRMATION_REQUIRED ||
		res.GetLiveLinkCount() != 7 || res.GetBot() != nil {
		t.Fatalf("= %v", res)
	}

	s, _ = zbServer(&fakeZaloBotUC{set: app.SetSharedZaloBotOutput{Result: app.SetTokenCheckFailed, ZaloOutcome: domain.ZaloCallTokenRejected}})
	res, _ = s.SetSharedZaloBot(context.Background(), setReq)
	if res.GetResult() != commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_TOKEN_CHECK_FAILED ||
		res.GetZaloOutcome() != commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_TOKEN_REJECTED {
		t.Fatalf("= %v", res)
	}

	// A result the server does not know is never sent as UNSPECIFIED.
	s, _ = zbServer(&fakeZaloBotUC{})
	if _, err := s.SetSharedZaloBot(context.Background(), setReq); status.Code(err) != codes.Internal {
		t.Fatalf("unknown result: %v", err)
	}
}

func TestGetSharedZaloBotNeverConfiguredIsOK(t *testing.T) {
	s, _ := zbServer(&fakeZaloBotUC{})
	res, err := s.GetSharedZaloBot(context.Background(), &commsv1.GetSharedZaloBotRequest{})
	if err != nil || res.GetConfigured() || res.GetBot() != nil || res.GetLastCheck() != nil {
		t.Fatalf("= %v, %v", res, err)
	}
	at := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	s, _ = zbServer(&fakeZaloBotUC{found: true, bot: domain.SharedZaloBot{SetAt: at, SetBy: "VH-00001",
		LastCheckAt: at, LastCheckResult: domain.ZaloCallUnavailable}})
	res, _ = s.GetSharedZaloBot(context.Background(), &commsv1.GetSharedZaloBotRequest{})
	if !res.GetConfigured() || res.GetLastCheck().GetOutcome() != commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_UNAVAILABLE {
		t.Fatalf("= %v", res)
	}
}

func TestOutcomeVocabularyIsComplete(t *testing.T) {
	for _, v := range []string{domain.ZaloCallOK, domain.ZaloCallNotConfigured, domain.ZaloCallTokenRejected,
		domain.ZaloCallRateLimited, domain.ZaloCallUnavailable, domain.ZaloCallRejected, domain.ZaloCallMalformedResponse} {
		if outcomeToWire(v) == commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_UNSPECIFIED {
			t.Errorf("%q has no wire value", v)
		}
	}
	if outcomeToWire("") != commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_UNSPECIFIED {
		t.Error("an unknown value must never read as a known outcome")
	}
}

func TestGetWebhookFieldsOnlyWhenOK(t *testing.T) {
	s, _ := zbServer(&fakeZaloBotUC{getHook: app.GetWebhookOutput{Outcome: domain.ZaloCallTokenRejected, URL: "x", SecretSetBy: "VH-00001"}})
	res, _ := s.GetSharedZaloBotWebhook(context.Background(), &commsv1.GetSharedZaloBotWebhookRequest{})
	if res.GetUrl() != "" || res.GetSecretSetBy() != "" {
		t.Fatalf("fields set on a failure: %v", res)
	}
}

func TestListZaloBotCommuneStatsTranslates(t *testing.T) {
	s, _ := zbServer(&fakeZaloBotUC{stats: []domain.ZaloBotCommuneStat{{TenantID: "01JTESTCOMMUNEXXXXXXXXXXXX", ChannelEnabled: true, LinkedStaffCount: 4}}})
	res, err := s.ListZaloBotCommuneStats(context.Background(), &commsv1.ListZaloBotCommuneStatsRequest{ActorCode: "VH-00001"})
	if err != nil || len(res.GetCommunes()) != 1 || res.GetCommunes()[0].GetLinkedStaffCount() != 4 || !res.GetCommunes()[0].GetChannelEnabled() {
		t.Fatalf("= %v, %v", res, err)
	}
}
