package event

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/vihat/vigov/core/events"
	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// WHAT THIS FILE PROVES, and each item is a failure that is SILENT in production:
//
//  1. An envelope with no commune is REFUSED — and refused because Receive goes through
//     core/events.Dispatch, not because a check was copied in here. Deleting the Dispatch call
//     turns this file red.
//  2. `occurrence` travels into `Round` untouched, and 0 is refused rather than defaulted. Without
//     it the SECOND closing of a reopened petition (ADR 0008) is swallowed as a duplicate, and
//     from inside the system that is indistinguishable from correct deduplication.
//  3. An absent `citizen_message` writes NOTHING and says nothing — no ledger row, no log line.
//     It is a normal outcome on the six transitions that owe the citizen no message.
//  4. Nothing on any path logs the recipient id or anything else that reads as a person (rule 3).
//
// NOT PROVED HERE: anything the ledger does. The row, its audit entry and the transaction they
// share belong to internal/app and are proved there; this file asserts only what is handed over.

const (
	testReportCode   = "PA-2026-7F3K9Q"
	testCitizenCode  = "01JCONGDANMAUTHUNGHIEM0001"
	testTemplateCode = "ZNS-PHAN-ANH-DOI-TRANG-THAI"
	testMilestone    = "da-chuyen-xu-ly"
)

var testTenant = tenant.ID("01JA" + strings.Repeat("A", 22))

// --- fakes --------------------------------------------------------------------------------------

type fakeLedger struct {
	received []app.RecordRequest
	tenants  []tenant.ID
	created  bool
	err      error
}

func (l *fakeLedger) Record(ctx context.Context, req app.RecordRequest) (app.RecordResult, error) {
	// The commune is read from the CONTEXT, which is the only place it may come from (rule 1,
	// invariant 4). Recording it here is what makes the missing-commune test prove something.
	l.tenants = append(l.tenants, tenant.MustFrom(ctx))
	l.received = append(l.received, req)
	if l.err != nil {
		return app.RecordResult{}, l.err
	}
	return app.RecordResult{ID: "01JTHONGBAOMAUTHUNGHIEM01", Created: l.created}, nil
}

type fakeTemplates struct {
	code  string
	err   error
	asked []string
}

func (f *fakeTemplates) TemplateCode(_ context.Context, milestone string) (string, error) {
	f.asked = append(f.asked, milestone)
	if f.err != nil {
		return "", f.err
	}
	return f.code, nil
}

// --- helpers ------------------------------------------------------------------------------------

func newTestConsumer(t *testing.T) (*CitizenReportStatusChanged, *fakeLedger, *fakeTemplates, *bytes.Buffer) {
	t.Helper()
	ledger := &fakeLedger{created: true}
	templates := &fakeTemplates{code: testTemplateCode}
	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, nil))
	return NewCitizenReportStatusChanged(ledger, templates, log), ledger, templates, &logBuf
}

// sampleMessage is a well-formed transition that DOES owe the citizen a message.
func sampleMessage() *petitionsv1.PetitionStatusChanged {
	return &petitionsv1.PetitionStatusChanged{
		LookupCode: testReportCode,
		Status:     testMilestone,
		Occurrence: 1,
		CitizenId:  testCitizenCode,
		CitizenMessage: &petitionsv1.CitizenMessage{
			StatusLabel: "Đã chuyển xử lý",
			NextStep:    "Bộ phận chuyên môn sẽ liên hệ trong thời hạn đã hẹn",
		},
	}
}

// envelopeOf encodes the payload EXACTLY as the contract says it travels: protojson with proto field
// names preserved. A test that hand-wrote the JSON would be testing its own spelling.
func envelopeOf(t *testing.T, msg *petitionsv1.PetitionStatusChanged) events.Envelope {
	t.Helper()
	b, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(msg)
	if err != nil {
		t.Fatalf("mã hoá payload: %v", err)
	}
	return events.Envelope{
		ID:       "01JPHONGBIMAUTHUNGHIEM001",
		Name:     CitizenReportStatusChangedEvent,
		TenantID: testTenant,
		At:       time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC),
		Payload:  b,
	}
}

// --- rule 1: no commune, no work ----------------------------------------------------------------

func TestEnvelopeWithoutTenantIsRefused(t *testing.T) {
	c, ledger, templates, logBuf := newTestConsumer(t)
	env := envelopeOf(t, sampleMessage())
	env.TenantID = ""

	err := c.Receive(context.Background(), env)
	if !errors.Is(err, events.ErrNoTenant) {
		t.Fatalf("muốn ErrNoTenant, nhận %v", err)
	}
	if len(ledger.received) != 0 {
		t.Fatalf("đã ghi sổ %d dòng cho một phong bì không có xã", len(ledger.received))
	}
	if len(templates.asked) != 0 {
		t.Fatalf("đã hỏi mẫu tin cho một phong bì không có xã")
	}
	if logBuf.Len() != 0 {
		t.Fatalf("có dòng nhật ký: %q", logBuf.String())
	}
}

func TestTenantFromEnvelopeReachesContext(t *testing.T) {
	c, ledger, _, _ := newTestConsumer(t)
	if err := c.Receive(context.Background(), envelopeOf(t, sampleMessage())); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(ledger.tenants) != 1 || ledger.tenants[0] != testTenant {
		t.Fatalf("xã trong context: %v, muốn %v", ledger.tenants, testTenant)
	}
}

// --- the contract's own refusals ----------------------------------------------------------------

func TestOtherEventNameIsRefused(t *testing.T) {
	c, ledger, _, _ := newTestConsumer(t)
	env := envelopeOf(t, sampleMessage())
	env.Name = "petitions.status_changed.v2"

	err := c.Receive(context.Background(), env)
	if !errors.Is(err, ErrWrongEventName) || !errors.Is(err, ErrNoRetry) {
		t.Fatalf("muốn ErrWrongEventName + ErrNoRetry, nhận %v", err)
	}
	if len(ledger.received) != 0 {
		t.Fatalf("đã ghi sổ cho một sự kiện không thuộc consumer này")
	}
}

func TestBrokenPayloadIsRefusedWithoutWrappingDecoderError(t *testing.T) {
	c, ledger, _, _ := newTestConsumer(t)
	env := envelopeOf(t, sampleMessage())
	// A body that fails to decode is, by definition, one that did not honour the contract — so it
	// is exactly the body that may carry what the contract forbids. `DAUVETRIENGBIET` stands in for
	// that content: the returned error must not repeat any of it (rule 3, forbidden #3).
	env.Payload = []byte(`{"occurrence": "DAUVETRIENGBIET"}`)

	err := c.Receive(context.Background(), env)
	if !errors.Is(err, ErrPayloadDecode) || !errors.Is(err, ErrNoRetry) {
		t.Fatalf("muốn ErrPayloadDecode + ErrNoRetry, nhận %v", err)
	}
	if strings.Contains(err.Error(), "DAUVETRIENGBIET") {
		t.Fatalf("lỗi trả về nhắc lại nội dung payload: %q", err.Error())
	}
	if len(ledger.received) != 0 {
		t.Fatalf("đã ghi sổ cho một payload hỏng")
	}
}

func TestNewFieldInPayloadStillPasses(t *testing.T) {
	// Rule 2, invariant 4: the publisher may add an OPTIONAL field to a live event. A consumer that
	// refused unknown fields would turn that permitted change into every message failing.
	c, ledger, _, _ := newTestConsumer(t)
	env := envelopeOf(t, sampleMessage())
	env.Payload = []byte(`{"lookup_code":"` + testReportCode + `","status":"` + testMilestone + `",` +
		`"occurrence":1,"citizen_id":"` + testCitizenCode + `",` +
		`"citizen_message":{"status_label":"Đã chuyển xử lý","next_step":"Bộ phận chuyên môn sẽ liên hệ"},` +
		`"previous_status":"dang-phan-loai"}`)

	if err := c.Receive(context.Background(), env); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(ledger.received) != 1 {
		t.Fatalf("ghi sổ %d dòng, muốn 1", len(ledger.received))
	}
}

func TestMissingRequiredFieldIsRefused(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*petitionsv1.PetitionStatusChanged)
		want   error
	}{
		{"thiếu mã tra cứu", func(m *petitionsv1.PetitionStatusChanged) { m.LookupCode = "" }, ErrMissingLookupCode},
		{"thiếu trạng thái", func(m *petitionsv1.PetitionStatusChanged) { m.Status = "" }, ErrMissingStatus},
		{"thiếu người nhận", func(m *petitionsv1.PetitionStatusChanged) { m.CitizenId = "" }, ErrMissingRecipient},
		{"lần bằng 0", func(m *petitionsv1.PetitionStatusChanged) { m.Occurrence = 0 }, ErrInvalidOccurrence},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, ledger, _, _ := newTestConsumer(t)
			msg := sampleMessage()
			tc.mutate(msg)

			err := c.Receive(context.Background(), envelopeOf(t, msg))
			if !errors.Is(err, tc.want) {
				t.Fatalf("muốn %v, nhận %v", tc.want, err)
			}
			if !errors.Is(err, ErrNoRetry) {
				t.Fatalf("một payload sai không tự đúng lên khi giao lại: %v", err)
			}
			if len(ledger.received) != 0 {
				t.Fatalf("đã ghi sổ cho một payload thiếu trường bắt buộc")
			}
		})
	}
}

// --- what gets handed to the ledger -------------------------------------------------------------

func TestRecordReceivesEveryField(t *testing.T) {
	c, ledger, templates, _ := newTestConsumer(t)
	if err := c.Receive(context.Background(), envelopeOf(t, sampleMessage())); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(ledger.received) != 1 {
		t.Fatalf("ghi sổ %d dòng, muốn 1", len(ledger.received))
	}
	want := app.RecordRequest{
		SubjectType:   domain.SubjectCitizenReport,
		SubjectCode:   testReportCode,
		Milestone:     testMilestone,
		Round:         1,
		Channel:       domain.ChannelZaloZNS,
		RecipientCode: testCitizenCode,
		TemplateCode:  testTemplateCode,
		Params: domain.NotificationParams{
			LookupCode:  testReportCode,
			StatusLabel: "Đã chuyển xử lý",
			NextStep:    "Bộ phận chuyên môn sẽ liên hệ trong thời hạn đã hẹn",
		},
	}
	if ledger.received[0] != want {
		t.Fatalf("yêu cầu ghi nợ:\n nhận %+v\n muốn %+v", ledger.received[0], want)
	}
	// Actor left zero: app.CitizenNotifications.Record turns it into the system principal (rule 6,
	// invariant 6). A consumer that filled in a staff id would be attributing the write to somebody who
	// did not do it.
	if ledger.received[0].Actor.ID != "" {
		t.Fatalf("người gây phải để trống cho app điền chủ thể hệ thống, nhận %q", ledger.received[0].Actor.ID)
	}
	if len(templates.asked) != 1 || templates.asked[0] != testMilestone {
		t.Fatalf("hỏi mẫu tin: %v", templates.asked)
	}
}

func TestSecondOccurrencePassesThroughUnchanged(t *testing.T) {
	// ADR 0008: a citizen may reopen a closed petition, so `da-dong` happens more than once and the
	// second closing owes a SECOND message. `occurrence` is what tells the two apart inside
	// domain.SendKey — passing 1 for both would swallow the second, and nothing would turn red
	// anywhere else.
	c, ledger, _, _ := newTestConsumer(t)
	msg := sampleMessage()
	msg.Status = "da-dong"
	msg.Occurrence = 2

	if err := c.Receive(context.Background(), envelopeOf(t, msg)); err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if ledger.received[0].Round != 2 {
		t.Fatalf("Round = %d, muốn 2", ledger.received[0].Round)
	}
}

func TestRedeliveryYieldsSameRequest(t *testing.T) {
	// Queues deliver at least once, and a producer that crashed after publishing republishes under
	// a FRESH envelope id. Neither may change what is handed to the ledger, because the
	// deduplication key is built from these fields — it is a key of the FACT, not of the message.
	c, ledger, _, _ := newTestConsumer(t)
	env := envelopeOf(t, sampleMessage())
	if err := c.Receive(context.Background(), env); err != nil {
		t.Fatalf("lần 1: %v", err)
	}
	env2 := envelopeOf(t, sampleMessage())
	env2.ID = "01JPHONGBIKHACMAUTHUNGHIE"
	ledger.created = false // the ledger reports "already recorded"
	if err := c.Receive(context.Background(), env2); err != nil {
		t.Fatalf("lần 2: %v", err)
	}
	if len(ledger.received) != 2 {
		t.Fatalf("gọi ghi sổ %d lần, muốn 2 — chống trùng là việc của sổ, không phải của consumer", len(ledger.received))
	}
	if ledger.received[0] != ledger.received[1] {
		t.Fatalf("hai lần giao cùng một sự thật cho hai yêu cầu khác nhau:\n %+v\n %+v", ledger.received[0], ledger.received[1])
	}
}

func TestLedgerErrorIsWrapped(t *testing.T) {
	c, ledger, _, _ := newTestConsumer(t)
	cause := errors.New("sổ đang bận")
	ledger.err = cause

	err := c.Receive(context.Background(), envelopeOf(t, sampleMessage()))
	if !errors.Is(err, cause) {
		t.Fatalf("muốn bọc lỗi của sổ, nhận %v", err)
	}
	if errors.Is(err, ErrNoRetry) {
		t.Fatalf("một lỗi ghi sổ PHẢI được giao lại, không được đánh dấu không thử lại")
	}
}

// --- an absent citizen_message is a fact -------------------------------------------------------

func TestAbsentCitizenMessageWritesAndSaysNothing(t *testing.T) {
	c, ledger, templates, logBuf := newTestConsumer(t)
	msg := sampleMessage()
	msg.CitizenMessage = nil

	if err := c.Receive(context.Background(), envelopeOf(t, msg)); err != nil {
		t.Fatalf("một bước nội bộ không phải lỗi: %v", err)
	}
	if len(ledger.received) != 0 {
		t.Fatalf("đã ghi %d dòng cho một chuyển trạng thái không nợ tin nào", len(ledger.received))
	}
	if len(templates.asked) != 0 {
		t.Fatalf("đã hỏi mẫu tin cho một chuyển trạng thái không gửi gì")
	}
	if logBuf.Len() != 0 {
		// It is the frequent, correct outcome on six of the nine transitions. Printing it as an
		// anomaly teaches whoever reads the channel to skip past it.
		t.Fatalf("đã log một ca hoàn toàn bình thường: %q", logBuf.String())
	}
}

// --- a commune with no OA degrades VISIBLY, and does not retry ---------------------------------

func TestChannelNotConfiguredDoesNotRetry(t *testing.T) {
	c, ledger, templates, logBuf := newTestConsumer(t)
	templates.err = ErrChannelNotConfigured

	err := c.Receive(context.Background(), envelopeOf(t, sampleMessage()))
	if !errors.Is(err, ErrChannelNotConfigured) || !errors.Is(err, ErrNoRetry) {
		t.Fatalf("muốn ErrChannelNotConfigured + ErrNoRetry, nhận %v", err)
	}
	if len(ledger.received) != 0 {
		t.Fatalf("đã ghi sổ dù chưa có mẫu tin nào được duyệt")
	}
	// ADR 0006 consequence 3: it must be VISIBLE, not silent.
	if !strings.Contains(logBuf.String(), testReportCode) {
		t.Fatalf("suy giảm phải nhìn thấy được, nhật ký: %q", logBuf.String())
	}
}

func TestOtherTemplateErrorIsStillRedelivered(t *testing.T) {
	c, _, templates, _ := newTestConsumer(t)
	templates.err = errors.New("kho cấu hình không với tới được")

	err := c.Receive(context.Background(), envelopeOf(t, sampleMessage()))
	if err == nil {
		t.Fatalf("muốn lỗi")
	}
	if errors.Is(err, ErrNoRetry) {
		t.Fatalf("một sự cố hạ tầng PHẢI được giao lại: %v", err)
	}
}

// --- rule 3: nothing on any path names a person -------------------------------------------------

func TestNoPathLogsCitizenIdentity(t *testing.T) {
	// The recipient id is opaque, which is why it may travel in the event at all — but a log line
	// pairing it with a record code is the first half of a map of who reported what (rule 6,
	// forbidden #4). Checked on BOTH paths that can log.
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"gửi được", nil},
		{"xã chưa cấu hình kênh", ErrChannelNotConfigured},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, templates, logBuf := newTestConsumer(t)
			templates.err = tc.err
			_ = c.Receive(context.Background(), envelopeOf(t, sampleMessage()))
			if strings.Contains(logBuf.String(), testCitizenCode) {
				t.Fatalf("mã công dân lọt vào nhật ký: %q", logBuf.String())
			}
		})
	}
}
