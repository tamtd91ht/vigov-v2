package app

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The typed / imported audiences (migration 0033; ADR 0099 §Sửa đổi S1–S7) over the same rig. What is
// proved: identity is never asked; every number is normalised server-side, invalid rows are kept masked and
// final, duplicates dropped; owed rows carry their number and message SEALED under the commune's DEK with
// the row-bound AAD (and nothing in clear anywhere else); the S1 limits; the SMS modes; the ZNS override;
// the mapping is stored; the audit entry holds counts, never a number or a row's text.

// The agreed fake numbers (rule 3, invariant 5), typed several ways.
const (
	typedPhoneA = "0900000000"
	typedPhoneB = "0900000001"
)

func typedSmsInput(mode string, rows ...NoticeRecipientInput) NoticeInput {
	return NoticeInput{Channel: domain.NoticeChannelSMS, SmsAccountID: noticeAccID, AudienceKind: domain.NoticeAudienceTypedList,
		SmsContentMode: mode, Recipients: rows, Title: "Nhắc lịch"}
}

func sharedRows() []NoticeRecipientInput {
	return []NoticeRecipientInput{
		{Phone: "+84 900 000 000", Values: map[string]string{"variable_1": "Bác An"}},
		{Phone: "abc-123"},
		{Phone: "090.000.0000", Values: map[string]string{"variable_1": "trùng"}}, // a duplicate of the first
		{Phone: "84900000001", Values: map[string]string{"variable_1": ""}},
	}
}

func TestTypedSmsSharedSendSealsOwedRowsAndKeepsInvalidOnes(t *testing.T) {
	r := newNoticeRig(t)
	in := typedSmsInput(domain.SmsContentShared, sharedRows()...)
	in.Content = "Chào {{variable_1}}, mời họp"
	got, err := r.uc.Send(r.ctx, in, noticeStaff())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.audience.asked) != 0 {
		t.Fatal("identity was asked for typed numbers")
	}
	if r.f.begun != 1 || r.f.commits != 1 || len(r.repo.campaigns) != 1 {
		t.Fatalf("tx %d/%d, campaigns %d", r.f.begun, r.f.commits, len(r.repo.campaigns))
	}
	c := r.repo.campaigns[0]
	if c.TemplateRef != "" || c.TemplateContent != in.Content || c.SmsContentMode != domain.SmsContentShared ||
		c.RecipientCount != 2 || c.InvalidCount != 1 || c.Audience.Kind != domain.NoticeAudienceTypedList || len(c.Audience.IDs) != 0 ||
		c.RecipientMapping != nil || c.UnitPrice != "350.00" || c.EstimatedCost != "700.00" || got.SourceName != "eSMS chính · XaMau" {
		t.Fatalf("campaign = %+v", c)
	}
	// The estimate priced the LONGEST text.
	if r.pricer.content[0] != "Chào Bác An, mời họp" {
		t.Fatalf("priced %q", r.pricer.content)
	}
	if len(r.repo.recipients) != 3 {
		t.Fatalf("%d rows, want 3 (the duplicate dropped)", len(r.repo.recipients))
	}
	wantText := map[string]string{typedPhoneA: "Chào Bác An, mời họp", typedPhoneB: "Chào , mời họp"}
	for _, row := range r.repo.recipients {
		if row.Kind != domain.NoticeRecipientTyped || row.Ref != row.ID || !domain.ValidMaskedPhone(row.PhoneMasked) {
			t.Fatalf("row = %+v", row)
		}
		if row.Status == domain.NoticeRecipientInvalid {
			if row.PhoneSealed != nil || row.PayloadSealed != nil || row.PhoneMasked != "***" {
				t.Fatalf("invalid row = %+v", row)
			}
			continue
		}
		// The seal opens ONLY under this row's own AAD, in this commune.
		phone, err := r.env.Open(r.ctx, row.PhoneSealed, noticePhoneAAD(r.ctx, row.ID))
		if err != nil {
			t.Fatalf("phone seal: %v", err)
		}
		if _, err := r.env.Open(r.ctx, row.PhoneSealed, noticePayloadAAD(r.ctx, row.ID)); err == nil {
			t.Fatal("a phone seal opened as a payload")
		}
		raw, err := r.env.Open(r.ctx, row.PayloadSealed, noticePayloadAAD(r.ctx, row.ID))
		if err != nil {
			t.Fatalf("payload seal: %v", err)
		}
		var p domain.NoticePayload
		if err := json.Unmarshal(raw, &p); err != nil || p.Text != wantText[string(phone)] {
			t.Fatalf("payload for %s = %+v %v", row.PhoneMasked, p, err)
		}
		if strings.Contains(string(row.PhoneSealed), string(phone)) {
			t.Fatal("the sealed bytes contain the number")
		}
	}
	// The trail: counts, never a number, a masked number or a row's text.
	aud := r.f.with("INSERT INTO audit_log")
	if len(aud) != 1 {
		t.Fatalf("%d audit entries", len(aud))
	}
	entry := argsText(aud[0].args)
	for _, banned := range []string{typedPhoneA, typedPhoneB, "09****", "Bác An", "mời họp"} {
		if strings.Contains(entry, banned) {
			t.Fatalf("the entry carries %q: %s", banned, entry)
		}
	}
	if !strings.Contains(entry, "invalid_count") || !strings.Contains(entry, noticeActor) {
		t.Fatalf("entry = %s", entry)
	}
	// Nothing in clear reached any statement.
	for _, st := range r.f.stmts {
		for _, p := range []string{typedPhoneA, typedPhoneB} {
			if strings.Contains(fmt.Sprint(st.args), p) {
				t.Fatalf("a full number reached the database in clear: %s", st.sql)
			}
		}
	}
}

func TestTypedPreviewCountsAndListsMissingVariables(t *testing.T) {
	r := newNoticeRig(t)
	in := typedSmsInput(domain.SmsContentShared, sharedRows()...)
	in.Content = "Chào {{variable_1}} hẹn {{variable_2}}"
	p, err := r.uc.Preview(r.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if p.RecipientCount != 2 || p.InvalidCount != 1 || p.DuplicateCount != 1 || p.SkippedCount != 0 ||
		strings.Join(p.MissingParams, ",") != "variable_2" || p.Content == nil || *p.Content != "Chào Bác An hẹn {{variable_2}}" ||
		p.UnitPrice != "" || len(r.pricer.content) != 0 {
		t.Fatalf("preview = %+v content %v", p, *p.Content)
	}
	if r.f.begun != 0 || len(r.audience.asked) != 0 {
		t.Fatal("the preview wrote or asked identity")
	}
	// The send refuses the same request with the names (400 family), writing nothing.
	_, err = r.uc.Send(r.ctx, in, noticeStaff())
	var missing *domain.NoticeMissingVariablesError
	if !errors.As(err, &missing) || !errors.Is(err, domain.ErrNoticeInvalid) || r.f.begun != 0 {
		t.Fatalf("send with a missing variable: %v", err)
	}
}

func TestTypedSmsPerRow(t *testing.T) {
	r := newNoticeRig(t)
	in := typedSmsInput(domain.SmsContentPerRow,
		NoticeRecipientInput{Phone: typedPhoneA, Content: "Nội dung riêng một"},
		NoticeRecipientInput{Phone: typedPhoneB, Content: "Nội dung riêng hai dài hơn"})
	in.AudienceKind = domain.NoticeAudienceImportedFile
	in.Mapping = &domain.NoticeRecipientMapping{PhoneColumn: "SĐT", ContentColumn: "Nội dung"}
	if _, err := r.uc.Send(r.ctx, in, noticeStaff()); err != nil {
		t.Fatal(err)
	}
	c := r.repo.campaigns[0]
	if c.TemplateContent != "" || c.TemplateName != "" || c.RecipientMapping == nil || c.RecipientMapping.ContentColumn != "Nội dung" {
		t.Fatalf("campaign = %+v", c)
	}
	if r.pricer.content[0] != "Nội dung riêng hai dài hơn" {
		t.Fatalf("priced %q", r.pricer.content)
	}
	// A valid number with an empty text refuses the whole request.
	r = newNoticeRig(t)
	in.Recipients[1].Content = "  "
	if _, err := r.uc.Send(r.ctx, in, noticeStaff()); !errors.Is(err, domain.ErrNoticeRowContent) || r.f.begun != 0 {
		t.Fatalf("empty row text: %v", err)
	}
}

func TestTypedRefusals(t *testing.T) {
	many := make([]NoticeRecipientInput, domain.MaxTypedNoticeRecipients+1)
	for i := range many {
		many[i] = NoticeRecipientInput{Phone: typedPhoneA}
	}
	cases := []struct {
		name string
		mut  func(*NoticeInput)
		want error
	}{
		{"no rows", func(in *NoticeInput) { in.Recipients = nil }, domain.ErrNoticeRecipients},
		{"over 1000 typed", func(in *NoticeInput) { in.Recipients = many }, domain.ErrNoticeRecipients},
		{"template on typed sms", func(in *NoticeInput) { in.TemplateRef = noticeTplRow }, domain.ErrNoticeContentMode},
		{"no mode", func(in *NoticeInput) { in.SmsContentMode = "" }, domain.ErrNoticeContentMode},
		{"params on typed sms", func(in *NoticeInput) { in.Params = map[string]string{"1": "x"} }, domain.ErrNoticeParams},
		{"shared text absent", func(in *NoticeInput) { in.SmsContentMode = domain.SmsContentShared }, domain.ErrNoticeContent},
		{"shared text on per-row", func(in *NoticeInput) { in.Content = "x" }, domain.ErrNoticeContent},
		{"mapping on danh-sach-so", func(in *NoticeInput) { in.Mapping = &domain.NoticeRecipientMapping{PhoneColumn: "p"} }, domain.ErrNoticeMapping},
		{"tep-nhap without mapping", func(in *NoticeInput) { in.AudienceKind = domain.NoticeAudienceImportedFile }, domain.ErrNoticeMapping},
		{"another commune's account", func(in *NoticeInput) { in.SmsAccountID = "01JOTHERCOMMUNEACC00000001" }, ErrNoticeSourceMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newNoticeRig(t)
			in := typedSmsInput(domain.SmsContentPerRow, NoticeRecipientInput{Phone: typedPhoneA, Content: "x"})
			tc.mut(&in)
			if _, err := r.uc.Preview(r.ctx, in); !errors.Is(err, tc.want) {
				t.Fatalf("= %v, want %v", err, tc.want)
			}
		})
	}
	// Rows for a directory audience are refused, as are a mode, a text and a mapping.
	for name, mut := range map[string]func(*NoticeInput){
		"rows":    func(in *NoticeInput) { in.Recipients = []NoticeRecipientInput{{Phone: typedPhoneA}} },
		"mode":    func(in *NoticeInput) { in.SmsContentMode = domain.SmsContentShared },
		"content": func(in *NoticeInput) { in.Content = "x" },
		"mapping": func(in *NoticeInput) { in.Mapping = &domain.NoticeRecipientMapping{PhoneColumn: "p"} },
	} {
		r := newNoticeRig(t)
		in := smsInput(map[string]string{"1": "a", "2": "b"})
		mut(&in)
		if _, err := r.uc.Preview(r.ctx, in); !errors.Is(err, domain.ErrNoticeInvalid) {
			t.Errorf("directory audience with %s: %v", name, err)
		}
	}
	// Only unreadable numbers: nothing to send, nothing written.
	r := newNoticeRig(t)
	in := typedSmsInput(domain.SmsContentPerRow, NoticeRecipientInput{Phone: "12", Content: "x"})
	if _, err := r.uc.Send(r.ctx, in, noticeStaff()); !errors.Is(err, ErrNoticeNoRecipients) || r.f.begun != 0 {
		t.Fatalf("all invalid: %v", err)
	}
	// No encryption key in the deployment: refused by name, nothing written.
	r = newNoticeRig(t)
	r.uc.sealer = nilEnvelope()
	in = typedSmsInput(domain.SmsContentPerRow, NoticeRecipientInput{Phone: typedPhoneA, Content: "x"})
	if _, err := r.uc.Send(r.ctx, in, noticeStaff()); err == nil || r.f.begun != 0 {
		t.Fatalf("no envelope: %v", err)
	}
}

func TestTypedZnsRowValuesOverrideFixedParams(t *testing.T) {
	r := newNoticeRig(t)
	in := NoticeInput{Channel: domain.NoticeChannelZNS, OARowID: noticeOAID, TemplateRef: "1001",
		AudienceKind: domain.NoticeAudienceImportedFile, Title: "Họp", Params: map[string]string{"su_kien": "Họp dân", "thoi_gian": "8h"},
		Mapping: &domain.NoticeRecipientMapping{PhoneColumn: "SĐT", Params: map[string]string{"su_kien": "Sự kiện"}},
		Recipients: []NoticeRecipientInput{
			{Phone: typedPhoneA, Values: map[string]string{"su_kien": "Họp tổ 3"}},
			{Phone: typedPhoneB, Values: map[string]string{"su_kien": ""}}, // empty cell keeps the fixed value
		}}
	if _, err := r.uc.Send(r.ctx, in, noticeStaff()); err != nil {
		t.Fatal(err)
	}
	c := r.repo.campaigns[0]
	if c.TemplateRef != "1001" || c.Params["su_kien"] != "Họp dân" || c.UnitPrice != "200.00" || c.EstimatedCost != "400.00" ||
		c.RecipientMapping.Params["su_kien"] != "Sự kiện" {
		t.Fatalf("campaign = %+v", c)
	}
	got := map[string]string{}
	for _, row := range r.repo.recipients {
		phone, _ := r.env.Open(r.ctx, row.PhoneSealed, noticePhoneAAD(r.ctx, row.ID))
		raw, _ := r.env.Open(r.ctx, row.PayloadSealed, noticePayloadAAD(r.ctx, row.ID))
		var p domain.NoticePayload
		_ = json.Unmarshal(raw, &p)
		got[string(phone)] = p.Params["su_kien"] + "|" + p.Params["thoi_gian"]
	}
	if got[typedPhoneA] != "Họp tổ 3|8h" || got[typedPhoneB] != "Họp dân|8h" {
		t.Fatalf("params = %v", got)
	}
	// A parameter with no fixed value and no column is missing (422 family), a value over the template's
	// bound is refused.
	r = newNoticeRig(t)
	in.Params = map[string]string{"su_kien": "Họp dân"}
	var missing *NoticeMissingParamsError
	if _, err := r.uc.Send(r.ctx, in, noticeStaff()); !errors.As(err, &missing) || missing.Missing[0] != "thoi_gian" {
		t.Fatalf("missing: %v", err)
	}
	r = newNoticeRig(t)
	in.Params = map[string]string{"thoi_gian": "8h"}
	in.Recipients[0].Values["su_kien"] = strings.Repeat("x", 31)
	if _, err := r.uc.Send(r.ctx, in, noticeStaff()); !errors.Is(err, domain.ErrNoticeParamLong) {
		t.Fatalf("too long: %v", err)
	}
}

func TestTypedImportLimitIsTenThousand(t *testing.T) {
	r := newNoticeRig(t)
	rows := make([]NoticeRecipientInput, domain.MaxImportedNoticeRecipients)
	for i := range rows {
		rows[i] = NoticeRecipientInput{Phone: fmt.Sprintf("09%08d", i), Content: "x"}
	}
	in := typedSmsInput(domain.SmsContentPerRow, rows...)
	in.AudienceKind = domain.NoticeAudienceImportedFile
	in.Mapping = &domain.NoticeRecipientMapping{PhoneColumn: "p", ContentColumn: "c"}
	p, err := r.uc.Preview(r.ctx, in)
	if err != nil || p.RecipientCount != domain.MaxImportedNoticeRecipients {
		t.Fatalf("%+v %v", p, err)
	}
	in.Recipients = append(in.Recipients, NoticeRecipientInput{Phone: typedPhoneA, Content: "x"})
	if _, err := r.uc.Preview(r.ctx, in); !errors.Is(err, domain.ErrNoticeRecipients) {
		t.Fatalf("10 001 rows: %v", err)
	}
}

func TestNoticeRecipientsHistoryPagesInTheCommune(t *testing.T) {
	r := newNoticeRig(t)
	r.repo.views = append(r.repo.views, sampleView("C1"))
	for i := 0; i < domain.NoticeRecipientPageSize+1; i++ {
		r.repo.recipients = append(r.repo.recipients, domain.NoticeRecipient{ID: fmt.Sprintf("R%04d", i), PhoneMasked: "09****0000"})
	}
	rows, next, err := r.uc.Recipients(r.ctx, "C1", "")
	if err != nil || len(rows) != domain.NoticeRecipientPageSize || next == "" {
		t.Fatalf("%d rows, next %q, %v", len(rows), next, err)
	}
	rows, next, err = r.uc.Recipients(r.ctx, "C1", next)
	if err != nil || len(rows) != 1 || next != "" || rows[0].ID != fmt.Sprintf("R%04d", domain.NoticeRecipientPageSize) {
		t.Fatalf("page 2: %+v %q %v", rows, next, err)
	}
	if _, _, err := r.uc.Recipients(r.ctx, "OTHER", ""); !errors.Is(err, ErrNoticeCampaignNotFound) {
		t.Fatalf("unknown campaign: %v", err)
	}
	if _, _, err := r.uc.Recipients(r.ctx, "C1", "!!"); !errors.Is(err, domain.ErrNoticeInvalid) {
		t.Fatalf("bad cursor: %v", err)
	}
}

// nilEnvelope is a deployment without SECRET_ENCRYPTION_KEYS: a nil commune envelope, which refuses by name.
func nilEnvelope() NoticeSealer {
	var e *crypto.Envelope
	return e
}

func sampleView(id string) docstore.NoticeCampaignView {
	return docstore.NoticeCampaignView{Campaign: domain.NoticeCampaign{ID: id, Channel: domain.NoticeChannelSMS,
		Status: domain.NoticeCampaignSending}}
}

// argsText renders statement arguments with []byte (a JSON delta) as text, so a test can read it.
func argsText(args []driver.Value) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		if b, ok := a.([]byte); ok {
			parts = append(parts, string(b))
			continue
		}
		parts = append(parts, fmt.Sprint(a))
	}
	return strings.Join(parts, " ")
}

// The shapes the web sends (coordinator, 17/10/2026): an empty phone cell is a row (kept khong-hop-le); an
// Excel numeric cell that lost its 0 is still the number; a mapped ZNS parameter arrives as "" in params
// and is not missing; an empty text names its row.
func TestTypedWebShapes(t *testing.T) {
	r := newNoticeRig(t)
	in := typedSmsInput(domain.SmsContentShared,
		NoticeRecipientInput{Phone: "900000000"}, // Excel dropped the 0
		NoticeRecipientInput{Phone: ""},          // empty cell
		NoticeRecipientInput{Phone: typedPhoneA}) // the same number as the first, once folded
	in.Content = "Mời họp tổ"
	p, err := r.uc.Preview(r.ctx, in)
	if err != nil || p.RecipientCount != 1 || p.InvalidCount != 1 || p.DuplicateCount != 1 {
		t.Fatalf("preview = %+v %v", p, err)
	}
	if _, err := r.uc.Send(r.ctx, in, noticeStaff()); err != nil {
		t.Fatal(err)
	}
	if r.repo.recipients[1].Status != domain.NoticeRecipientInvalid || r.repo.recipients[1].PhoneMasked != "*" {
		t.Fatalf("empty cell row = %+v", r.repo.recipients[1])
	}

	r = newNoticeRig(t)
	zns := NoticeInput{Channel: domain.NoticeChannelZNS, OARowID: noticeOAID, TemplateRef: "1001", Title: "Họp",
		AudienceKind: domain.NoticeAudienceImportedFile, Params: map[string]string{"su_kien": "", "thoi_gian": "8h"},
		Mapping:    &domain.NoticeRecipientMapping{PhoneColumn: "Cột A", Params: map[string]string{"su_kien": "Cột B"}},
		Recipients: []NoticeRecipientInput{{Phone: typedPhoneA, Values: map[string]string{"su_kien": ""}}}}
	if p, err := r.uc.Preview(r.ctx, zns); err != nil || len(p.MissingParams) != 0 {
		t.Fatalf("mapped param counted missing: %+v %v", p, err)
	}

	r = newNoticeRig(t)
	in = typedSmsInput(domain.SmsContentPerRow, NoticeRecipientInput{Phone: typedPhoneA, Content: "a"},
		NoticeRecipientInput{Phone: typedPhoneB, Content: ""})
	_, err = r.uc.Preview(r.ctx, in)
	var row *domain.NoticeRowContentError
	if !errors.As(err, &row) || row.Row != 2 || !errors.Is(err, domain.ErrNoticeRowContent) {
		t.Fatalf("empty text: %v", err)
	}
}
