package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// ADR 0079 lô 5 Q13 — the commune's Zalo "nhắc trước" lead, over the recording fake driver and the REAL
// store: the items ride the Zalo outbox only, and the sender narrows a due-soon digest to the lead, words
// it from the kept items, skips it when none is kept, and leaves every other case exactly as before.

// --- Deliver: the items go to the Zalo outbox, keyed by notice ---------------------------------------------

func TestDeliver_HandsDueSoonItemsToTheZaloOutboxOnly(t *testing.T) {
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

	in := twoNotices()
	in[0].DueSoonItems = []domain.DueSoonItem{{Code: "NV-0001", Deadline: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)}}
	if _, err := uc.Deliver(ctx, in, jobActor); err != nil {
		t.Fatal(err)
	}
	// The bell INSERT never carries them.
	for _, s := range f.with("INSERT INTO staff_notification") {
		if strings.Contains(s.sql, "due_soon") {
			t.Fatal("the bell row was given the items")
		}
		for _, a := range s.args {
			if v, ok := a.(string); ok && strings.Contains(v, "NV-0001") {
				t.Fatal("the bell row was given the items")
			}
		}
	}
	enq := f.with("INSERT INTO zalo_delivery")
	if len(enq) != 1 || len(enq[0].args) != 7 {
		t.Fatalf("enqueue = %+v", enq)
	}
	var byKey map[string][]map[string]any
	if err := json.Unmarshal([]byte(enq[0].args[6].(string)), &byKey); err != nil {
		t.Fatal(err)
	}
	if len(byKey) != 1 || len(byKey[in[0].IdempotencyKey]) != 1 ||
		byKey[in[0].IdempotencyKey][0]["code"] != "NV-0001" ||
		byKey[in[0].IdempotencyKey][0]["deadline"] != "2026-10-01T09:00:00Z" {
		t.Fatalf("$7 = %v — only the notice that carried items, as sent", byKey)
	}
	aud := f.with("INSERT INTO audit_log")
	if len(aud) != 1 || !strings.Contains(string(aud[0].args[7].([]byte)), `"so_muc_sap_den_han":1`) ||
		strings.Contains(string(aud[0].args[7].([]byte)), "NV-0001") {
		t.Error("the entry must count the items, never list them")
	}
}

// --- the settings save: the lead is written, and audited before/after ------------------------------------

func TestSaveSettings_WritesTheLeadWithBeforeAndAfter(t *testing.T) {
	f := &sqlFake{query: func(q string, _ []driver.Value) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "FROM zalo_channel_setting") {
			// Stored: same form, lead 7 days.
			return []string{"e", "k", "qs", "qe", "sd", "rd", "ua", "ub", "dsd"},
				[][]driver.Value{{true, "nhiem-vu.sap-den-han", "21:00", "06:00", nil, nil, botClock, "CB-1", int64(7)}}, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	uc := NewZaloLinks(kho, docstore.NewZaloLinkStore(kho), fakeSource{}, fakeSend{}, noNames{}, nil)
	ctx := tenant.Into(context.Background(), xaA)
	base := domain.ZaloChannelSetting{IsEnabled: true, Kinds: []string{domain.ZaloKindTaskDueSoon},
		QuietStartMinute: 21 * 60, QuietEndMinute: 6 * 60}

	// The same lead: a no-op — nothing written, nothing recorded.
	seven := 7
	same := base
	same.DueSoonDays = &seven
	if _, err := uc.SaveSettings(ctx, same, staffActor); err != nil {
		t.Fatal(err)
	}
	if len(f.with("INSERT INTO zalo_channel_setting")) != 0 || len(f.with("INSERT INTO audit_log")) != 0 {
		t.Fatal("an unchanged lead was written")
	}

	// Cleared: written as NULL, and the entry carries 7 → null.
	out, err := uc.SaveSettings(ctx, base, staffActor)
	if err != nil || out.DueSoonDays != nil {
		t.Fatalf("save = %+v %v", out, err)
	}
	ins := f.with("INSERT INTO zalo_channel_setting")
	if len(ins) != 1 || ins[0].args[9] != nil {
		t.Fatalf("written %+v — the lead is $10, NULL when cleared", ins)
	}
	aud := f.with("INSERT INTO audit_log")
	if len(aud) != 1 {
		t.Fatalf("entries = %d", len(aud))
	}
	var delta struct {
		Before map[string]any `json:"truoc"`
		After  map[string]any `json:"sau"`
	}
	if err := json.Unmarshal(aud[0].args[7].([]byte), &delta); err != nil {
		t.Fatal(err)
	}
	if delta.Before["due_soon_days"] != float64(7) || delta.After["due_soon_days"] != nil {
		t.Errorf("delta = %+v, want due_soon_days 7 → null", delta)
	}
	if _, ok := delta.After["due_soon_days"]; !ok {
		t.Error("the after half omits due_soon_days")
	}
}

// --- the sender ----------------------------------------------------------------------------------------------

type oneCommuneLocks struct{}

func (oneCommuneLocks) CommunesWithDueZaloDeliveries(context.Context, time.Time) ([]tenant.ID, error) {
	return []tenant.ID{xaA}, nil
}
func (oneCommuneLocks) TryLockScheduler(context.Context) (func(), bool, error) {
	return func() {}, true, nil
}

// textSend records what every send said.
type textSend struct{ texts []string }

func (s *textSend) SendMessage(_ context.Context, _ secret.Secret, _, text string) zalobot.Outcome {
	s.texts = append(s.texts, text)
	return zalobot.OutcomeOK
}

// Noon in Vietnam, outside the quiet window. A lead of 2 days keeps deadlines up to 2026-10-10 05:00 UTC.
var leadClock = time.Date(2026, 10, 8, 5, 0, 0, 0, time.UTC)

const (
	digestTitle = "Bạn có 3 nhiệm vụ sắp đến hạn xử lý"
	digestBody  = "Gồm: NV-0001, NV-0002, NV-0003."
	digestLink  = "/nhiem-vu?soon=true"
	digestURL   = "https://xa.example.gov.vn" + digestLink
)

// itemsJSON is zalo_delivery.due_soon_items as the store wrote it.
func itemsJSON(deadlines map[string]string) string {
	var parts []string
	for _, code := range []string{"NV-0001", "NV-0002", "NV-0003"} {
		if d, ok := deadlines[code]; ok {
			parts = append(parts, fmt.Sprintf(`{"code":%q,"deadline":%q}`, code, d))
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// runLead runs one tick for commune A with one owed due-soon digest. lead nil = no lead saved; items nil =
// the row carries none (an old producer).
func runLead(t *testing.T, kind string, lead, items any) (*sqlFake, *textSend) {
	t.Helper()
	f := &sqlFake{query: func(q string, args []driver.Value) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "FROM zalo_delivery d"):
			return []string{"id", "staff_code", "kind", "attempts", "title", "body", "link", "due_soon_items"},
				[][]driver.Value{{"01JDELIVLEAD", "CB-00001", kind, int64(0), digestTitle, digestBody, digestLink, items}}, nil
		case strings.Contains(q, "FROM zalo_channel_setting"):
			return []string{"e", "k", "qs", "qe", "sd", "rd", "ua", "ub", "dsd"},
				[][]driver.Value{{true, "nhiem-vu.sap-den-han,phan-anh.qua-han", "21:00", "06:00", int64(1), int64(1),
					botClock, "CB-1", lead}}, nil
		case strings.Contains(q, "FROM zalo_link"):
			return []string{"staff_code", "id", "chat_id"},
				[][]driver.Value{{"CB-00001", "01JLINKLEAD", "chat-lead"}}, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	send := &textSend{}
	z, err := NewZaloDispatcher(ZaloDispatcherDeps{DB: kho, Repo: docstore.NewZaloLinkStore(kho), Locks: oneCommuneLocks{},
		Registry: activeRegistry{}, Bots: perCommuneSource{}, Send: send,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	z.now = func() time.Time { return leadClock }
	z.Tick(context.Background())
	return f, send
}

var threeItems = itemsJSON(map[string]string{
	"NV-0001": "2026-10-08T09:00:00Z", // inside
	"NV-0002": "2026-10-10T05:00:00Z", // exactly at the cut-off: inside
	"NV-0003": "2026-10-12T00:00:00Z", // beyond two days: outside
})

func TestDispatcherNarrowsADueSoonDigestToTheLead(t *testing.T) {
	f, send := runLead(t, domain.ZaloKindTaskDueSoon, int64(2), threeItems)
	want := "Bạn có 2 nhiệm vụ sắp đến hạn xử lý\nGồm: NV-0001, NV-0002.\n" + digestURL
	if len(send.texts) != 1 || send.texts[0] != want {
		t.Fatalf("sent %q, want %q — worded from the kept items, never the bell's count", send.texts, want)
	}
	if len(f.with("status = 'bo-qua'")) != 0 {
		t.Error("a kept digest was skipped")
	}
}

func TestDispatcherSkipsADigestWithNothingInsideTheLead(t *testing.T) {
	far := itemsJSON(map[string]string{"NV-0003": "2026-10-12T00:00:00Z"})
	f, send := runLead(t, domain.ZaloKindTaskDueSoon, int64(2), far)
	if len(send.texts) != 0 {
		t.Fatalf("sent %q — nothing is inside the lead", send.texts)
	}
	skips := f.with("status = 'bo-qua'")
	if len(skips) != 1 || skips[0].args[2] != domain.ZaloSkipOutsideDueSoonLead {
		t.Fatalf("skips = %+v, want one %s", skips, domain.ZaloSkipOutsideDueSoonLead)
	}
	if len(f.with("attempts = attempts + 1")) != 0 {
		t.Error("a skipped digest counted an attempt")
	}
	aud := f.with("INSERT INTO audit_log")
	if len(aud) != 1 || !strings.Contains(string(aud[0].args[7].([]byte)), "bo-qua:"+domain.ZaloSkipOutsideDueSoonLead) {
		t.Error("the skip is not in the trail with its reason")
	}
}

// No lead saved, or no items (an old producer), or a kind that is not due soon: today's message — the
// bell's title and body, unfiltered. Never "send nothing".
func TestDispatcherKeepsTodaysMessageWithoutLeadOrItems(t *testing.T) {
	today := digestTitle + "\n" + digestBody + "\n" + digestURL
	for name, c := range map[string]struct {
		kind        string
		lead, items any
	}{
		"no lead saved":     {domain.ZaloKindTaskDueSoon, nil, threeItems},
		"no items":          {domain.ZaloKindTaskDueSoon, int64(2), nil},
		"no lead, no items": {domain.ZaloKindTaskDueSoon, nil, nil},
	} {
		t.Run(name, func(t *testing.T) {
			_, send := runLead(t, c.kind, c.lead, c.items)
			if len(send.texts) != 1 || send.texts[0] != today {
				t.Fatalf("sent %q, want today's %q", send.texts, today)
			}
		})
	}
	// A far-only list without a lead is still sent whole: the lead, not the items, decides narrowing.
	far := itemsJSON(map[string]string{"NV-0003": "2026-10-12T00:00:00Z"})
	if _, send := runLead(t, domain.ZaloKindTaskDueSoon, nil, far); len(send.texts) != 1 || send.texts[0] != today {
		t.Fatalf("no lead: sent %q", send.texts)
	}
}

// Overdue notices are never narrowed, whatever the lead: the lead is the due-soon box only.
func TestDispatcherNeverNarrowsAnOverdueNotice(t *testing.T) {
	_, send := runLead(t, domain.ZaloKindPetitionOverdue, int64(1), nil)
	if len(send.texts) != 1 || !strings.HasPrefix(send.texts[0], digestTitle) {
		t.Fatalf("sent %q", send.texts)
	}
}
