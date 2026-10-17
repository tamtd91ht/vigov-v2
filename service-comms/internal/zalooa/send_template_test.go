package zalooa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/secret"
)

// message/template (block D-5). The number is the agreed fake (rule 3, invariant 5).
const testPhone = "0900000000"

func TestSendTemplateMessageShapeAndPhoneForm(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(http.StatusOK, `{"error":0,"message":"Success","data":{"msg_id":"abc123","sent_time":"1"}}`))
	id, o := c.SendTemplateMessage(context.Background(), secret.Secret(testAccess), testPhone, "1001",
		map[string]string{"ten_su_kien": "Hop dan"}, "01JROW0000000000000000001")
	if o != OutcomeOK || id != "abc123" {
		t.Fatalf("= %q %v", id, o)
	}
	if rc.paths[0] != "/message/template" || rc.headers[0].Get("access_token") != testAccess {
		t.Fatalf("path %q headers %v", rc.paths[0], rc.headers[0])
	}
	var body map[string]any
	if err := json.Unmarshal(rc.bodies[0], &body); err != nil {
		t.Fatal(err)
	}
	// Exactly the four declared fields (rule 3, invariant 6), the number in Zalo's 84… form.
	if len(body) != 4 || body["phone"] != "84900000000" || body["template_id"] != "1001" ||
		body["tracking_id"] != "01JROW0000000000000000001" {
		t.Fatalf("body = %v", body)
	}
}

func TestSendTemplateMessageClasses(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   Outcome
	}{
		{200, `{"error":-124,"message":"x"}`, OutcomeTokenRejected},
		{200, `{"error":-108,"message":"x"}`, OutcomeRecipientRejected},
		{200, `{"error":-118}`, OutcomeRecipientRejected},
		{200, `{"error":-1234}`, OutcomeRejected},
		{200, `{"error":0,"data":{}}`, OutcomeMalformedResponse},
		{200, `{"error":0,"data":{"msg_id":"has space"}}`, OutcomeMalformedResponse},
		{429, `{}`, OutcomeRateLimited},
		{503, `{}`, OutcomeUnavailable},
		{401, `{}`, OutcomeTokenRejected},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.status, tc.body), func(t *testing.T) {
			c, _, _ := start(t, jsonAnswer(tc.status, tc.body))
			if _, o := c.SendTemplateMessage(context.Background(), secret.Secret(testAccess), testPhone, "1001", nil, "R1"); o != tc.want {
				t.Fatalf("= %v, want %v", o, tc.want)
			}
		})
	}
}

func TestSendTemplateMessageRefusesLocallyWithoutCallingZalo(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(http.StatusOK, `{"error":0,"data":{"msg_id":"x"}}`))
	ctx := context.Background()
	if _, o := c.SendTemplateMessage(ctx, nil, testPhone, "1001", nil, "R1"); o != OutcomeTokenRejected {
		t.Errorf("no token = %v", o)
	}
	if _, o := c.SendTemplateMessage(ctx, secret.Secret(testAccess), "12345", "1001", nil, "R1"); o != OutcomeRecipientRejected {
		t.Errorf("bad number = %v", o)
	}
	if _, o := c.SendTemplateMessage(ctx, secret.Secret(testAccess), testPhone, "1001", nil, strings.Repeat("x", 49)); o != OutcomeRejected {
		t.Errorf("long tracking id = %v", o)
	}
	if len(rc.paths) != 0 {
		t.Fatalf("Zalo was asked %d times", len(rc.paths))
	}
}

func TestSendTemplateMessageNeverLogs(t *testing.T) {
	buf := captureLogs(t)
	c, _, _ := start(t, jsonAnswer(http.StatusInternalServerError, `{"error":"`+testPhone+`"}`))
	_, _ = c.SendTemplateMessage(context.Background(), secret.Secret(testAccess), testPhone, "1001", nil, "R1")
	if buf.Len() != 0 {
		t.Fatalf("the adapter logged: %s", buf.String())
	}
}

// SendTemplateMessageReport hands on Zalo's `message` for a non-zero `error` (ADR 0099 S5), with the access
// token cut out should Zalo quote it; nothing for an acceptance or a transport failure.
func TestSendTemplateMessageReportReturnsZalosMessage(t *testing.T) {
	c, _, _ := start(t, jsonAnswer(http.StatusOK, `{"error":-108,"message":"Phone number invalid `+testAccess+`"}`))
	_, o, text := c.SendTemplateMessageReport(context.Background(), secret.Secret(testAccess), testPhone, "1001", nil, "R1")
	if o != OutcomeRecipientRejected || text != "Phone number invalid ***" {
		t.Fatalf("= %v %q", o, text)
	}
	c, _, _ = start(t, jsonAnswer(http.StatusOK, `{"error":0,"message":"Success","data":{"msg_id":"m1"}}`))
	if _, o, text := c.SendTemplateMessageReport(context.Background(), secret.Secret(testAccess), testPhone, "1001", nil, "R1"); o != OutcomeOK || text != "" {
		t.Fatalf("ok: %v %q", o, text)
	}
	c, _, _ = start(t, jsonAnswer(http.StatusServiceUnavailable, `{"message":"down"}`))
	if _, o, text := c.SendTemplateMessageReport(context.Background(), secret.Secret(testAccess), testPhone, "1001", nil, "R1"); o != OutcomeUnavailable || text != "" {
		t.Fatalf("5xx: %v %q", o, text)
	}
}
