package app

// Issuing an announcement rings each recipient's bell (`thong-bao.moi`, migration 0025, ADR 0086) — over
// the recording fake driver and the REAL stores: the bell rows share the issuing transaction, go through
// the bell's own store function, roll back with the act, exclude the publisher, carry the idempotent key,
// and leave the title out of the trail. What PostgreSQL decides (the unique key behind ON CONFLICT, the
// enqueue's CASE) is the store's pg suite.

import (
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

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const announcementSecretTitle = "TIEU-DE-THONG-BAO-KHONG-VAO-VET"

// newIssueUseCase builds the REAL use case over the REAL stores over f, ids and clock pinned.
func newIssueUseCase(t *testing.T, f *sqlFake, withZalo bool) (*SoanThongBaoNoiBo, context.Context) {
	t.Helper()
	db := sql.OpenDB(f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	uc := NewSoanThongBaoNoiBo(kho, commsstore.NewThongBaoNoiBoStore(kho), commsstore.NewStaffNotificationStore(kho))
	if withZalo {
		uc.WithZaloOutbox(commsstore.NewZaloLinkStore(kho), slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
	n := 0
	uc.sinhID = func() (string, error) { n++; return fmt.Sprintf("01JANNOUNCE%015d", n), nil }
	// Nanoseconds on purpose: the act must truncate to PostgreSQL's microseconds, or the Zalo enqueue
	// (which finds the bell rows by created_at) would match none of them.
	uc.bayGio = func() time.Time { return time.Date(2026, 10, 9, 2, 0, 0, 123456789, time.UTC) }
	return uc, tenant.Into(context.Background(), xaA)
}

func issueRequest(recipients ...string) domain.YeuCauSoanThongBao {
	return domain.YeuCauSoanThongBao{TieuDe: announcementSecretTitle, NoiDung: "Nội dung.", NguoiNhanMa: recipients}
}

// bellFake answers the bell INSERT with created(q) and fails the first statement containing fail.
func bellFake(created func(q string) int64, fail string) *sqlFake {
	return &sqlFake{exec: func(q string, _ []driver.Value) (int64, error) {
		if fail != "" && strings.Contains(q, fail) {
			return 0, errors.New("fake: broken on purpose")
		}
		if strings.Contains(q, "INSERT INTO staff_notification") {
			return created(q), nil
		}
		return 1, nil
	}}
}

func stmtIndex(f *sqlFake, sub string) int {
	for i, s := range f.stmts {
		if strings.Contains(s.sql, sub) {
			return i
		}
	}
	return -1
}

func TestPublishAnnouncement_WritesBellRowsInTheIssuingTransaction(t *testing.T) {
	f := bellFake(insertTuples, "")
	uc, ctx := newIssueUseCase(t, f, false)

	issued, err := uc.PhatHanh(ctx, issueRequest("CB-2026-AAAA11", maCanBoSoan, "CB-2026-BBBB22"), nguoiSoanMau())
	if err != nil {
		t.Fatal(err)
	}
	if f.begun != 1 || f.commits != 1 || f.rollbacks != 0 {
		t.Fatalf("begun %d commits %d rollbacks %d — one transaction for the whole act", f.begun, f.commits, f.rollbacks)
	}
	bell := f.with("INSERT INTO staff_notification")
	if len(bell) != 1 {
		t.Fatalf("bell statements = %d, want 1", len(bell))
	}
	// AddDelivery: $1 commune · $2 key · $3 kind · $4 title · $5 body · $6 link · $7 time, then (id, code).
	a := bell[0].args
	if a[0] != string(xaA) || a[1] != "thong-bao.moi:"+issued.ID || a[2] != "thong-bao.moi" ||
		a[3] != announcementSecretTitle || a[4] != "" || a[5] != "/thong-bao" {
		t.Errorf("bell args = %v", a[:6])
	}
	if at, _ := a[6].(time.Time); at.Nanosecond()%1000 != 0 || !at.Equal(issued.PhatHanhLuc) {
		t.Errorf("bell created_at = %v — the act's one clock reading, in microseconds", a[6])
	}
	// The publisher named themselves: on the book's list, but their own bell does not ring.
	if len(a) != 7+2*2 || a[8] != "CB-2026-AAAA11" || a[10] != "CB-2026-BBBB22" {
		t.Errorf("bell recipients = %v, want the two colleagues without the publisher", a[7:])
	}
	if l := f.with("INSERT INTO thong_bao_nguoi_nhan"); len(l) != 1 || len(l[0].args) != 2+3*2 {
		t.Error("the book's recipient list must still hold all three, publisher included")
	}
	if !(stmtIndex(f, "INSERT INTO thong_bao_nguoi_nhan") < stmtIndex(f, "INSERT INTO staff_notification") &&
		stmtIndex(f, "INSERT INTO staff_notification") < stmtIndex(f, "INSERT INTO audit_log")) {
		t.Error("statement order is not announcement → recipients → bell → trail")
	}
}

func TestPublishAnnouncement_TrailCountsTheBellAndNeverCarriesTheTitle(t *testing.T) {
	f := bellFake(insertTuples, "")
	uc, ctx := newIssueUseCase(t, f, false)
	if _, err := uc.PhatHanh(ctx, issueRequest("CB-2026-AAAA11", "CB-2026-BBBB22"), nguoiSoanMau()); err != nil {
		t.Fatal(err)
	}
	aud := f.with("INSERT INTO audit_log")
	if len(aud) != 1 {
		t.Fatalf("audit entries = %d, want ONE for the act", len(aud))
	}
	delta := string(aud[0].args[7].([]byte))
	if strings.Contains(delta, announcementSecretTitle) {
		t.Fatal("the title reached the trail — the ledger is never deleted (rule 3)")
	}
	for _, want := range []string{`"chuong":`, `"tao_moi":2`, `"da_co":0`, `"khoa":"thong-bao.moi:`} {
		if !strings.Contains(delta, want) {
			t.Errorf("delta lacks %s: %s", want, delta)
		}
	}
}

func TestPublishAnnouncement_BellFailureRollsBackTheWholeAct(t *testing.T) {
	f := bellFake(insertTuples, "INSERT INTO staff_notification")
	uc, ctx := newIssueUseCase(t, f, true)
	if _, err := uc.PhatHanh(ctx, issueRequest("CB-2026-AAAA11"), nguoiSoanMau()); err == nil {
		t.Fatal("the bell failed and the announcement still reported issued")
	}
	if f.commits != 0 || f.rollbacks != 1 {
		t.Errorf("commits %d rollbacks %d, want 0/1 — issued and told are one fact", f.commits, f.rollbacks)
	}
	if len(f.with("INSERT INTO audit_log")) != 0 || len(f.with("INSERT INTO zalo_delivery")) != 0 {
		t.Error("statements ran after the bell failed")
	}
}

func TestPublishAnnouncement_TrailFailureRollsBackTheBellRows(t *testing.T) {
	f := bellFake(insertTuples, "INSERT INTO audit_log")
	uc, ctx := newIssueUseCase(t, f, false)
	if _, err := uc.PhatHanh(ctx, issueRequest("CB-2026-AAAA11"), nguoiSoanMau()); err == nil {
		t.Fatal("the trail failed and the act still reported success")
	}
	if len(f.with("INSERT INTO staff_notification")) != 1 || f.commits != 0 || f.rollbacks != 1 {
		t.Errorf("bell written %d, commits %d rollbacks %d — the bell rows must go with the trail",
			len(f.with("INSERT INTO staff_notification")), f.commits, f.rollbacks)
	}
}

func TestPublishAnnouncement_QueuesZaloCopiesForTheBellKeyOnly(t *testing.T) {
	f := bellFake(insertTuples, "")
	uc, ctx := newIssueUseCase(t, f, true)
	issued, err := uc.PhatHanh(ctx, issueRequest("CB-2026-AAAA11"), nguoiSoanMau())
	if err != nil {
		t.Fatal(err)
	}
	enq := f.with("INSERT INTO zalo_delivery")
	if len(enq) != 1 || f.commits != 1 {
		t.Fatalf("enqueue statements = %d commits %d", len(enq), f.commits)
	}
	// $1 commune · $2 keys · $3 created_at · $4/$5 legacy map · $6 queueable kinds · $7 due-soon items.
	keys, _ := enq[0].args[1].([]string)
	if len(keys) != 1 || keys[0] != "thong-bao.moi:"+issued.ID {
		t.Errorf("keys = %v", enq[0].args[1])
	}
	if at, _ := enq[0].args[2].(time.Time); !at.Equal(issued.PhatHanhLuc) {
		t.Errorf("enqueue created_at = %v, want the bell's %v", enq[0].args[2], issued.PhatHanhLuc)
	}
	// The kind filter the statement uses must admit thong-bao.moi, or the row is silently never queued;
	// whether the commune ticked it is the statement's CASE ('loai-tat' otherwise), proved by the pg suite.
	if kinds, _ := enq[0].args[5].([]string); !strings.Contains(strings.Join(kinds, ","), "thong-bao.moi") {
		t.Errorf("queueable kinds = %v lack thong-bao.moi", kinds)
	}
	if enq[0].args[6] != "{}" {
		t.Errorf("due-soon items = %v, want none", enq[0].args[6])
	}
	if len(f.with("SAVEPOINT zalo_delivery_enqueue")) == 0 {
		t.Error("the enqueue is not inside its savepoint")
	}
}

func TestPublishAnnouncement_ZaloFailureStillIssuesAndSaysSo(t *testing.T) {
	f := bellFake(insertTuples, "INSERT INTO zalo_delivery")
	uc, ctx := newIssueUseCase(t, f, true)
	if _, err := uc.PhatHanh(ctx, issueRequest("CB-2026-AAAA11"), nguoiSoanMau()); err != nil {
		t.Fatalf("a Zalo failure failed the announcement: %v", err)
	}
	if f.commits != 1 || len(f.with("ROLLBACK TO SAVEPOINT zalo_delivery_enqueue")) != 1 {
		t.Errorf("commits %d — the Zalo failure must roll back to its savepoint only", f.commits)
	}
	if aud := f.with("INSERT INTO audit_log"); len(aud) != 1 ||
		!strings.Contains(string(aud[0].args[7].([]byte)), `"zalo_loi_xep_hang":true`) {
		t.Error("the trail does not say the Zalo copies were not queued")
	}
}

func TestPublishAnnouncement_OnlyThePublisherWritesNoBellRow(t *testing.T) {
	f := bellFake(insertTuples, "")
	uc, ctx := newIssueUseCase(t, f, true)
	if _, err := uc.PhatHanh(ctx, issueRequest(maCanBoSoan), nguoiSoanMau()); err != nil {
		t.Fatal(err)
	}
	if len(f.with("INSERT INTO staff_notification")) != 0 || len(f.with("INSERT INTO zalo_delivery")) != 0 {
		t.Error("the publisher's own bell rang for their own act")
	}
	if f.commits != 1 || len(f.with("INSERT INTO thong_bao_nguoi_nhan")) != 1 {
		t.Error("the announcement itself must still be issued")
	}
}

// The key is the announcement's id, so a redone write of the same act meets ON CONFLICT and creates
// nothing; the trail then counts what already existed, and no Zalo copy is queued twice.
func TestPublishAnnouncement_BellKeyIsIdempotent(t *testing.T) {
	f := bellFake(func(string) int64 { return 0 }, "")
	uc, ctx := newIssueUseCase(t, f, true)
	issued, err := uc.PhatHanh(ctx, issueRequest("CB-2026-AAAA11", "CB-2026-BBBB22"), nguoiSoanMau())
	if err != nil {
		t.Fatal(err)
	}
	bell := f.with("INSERT INTO staff_notification")
	if len(bell) != 1 || !strings.Contains(bell[0].sql, "ON CONFLICT (tenant_id, idempotency_key, recipient_code) DO NOTHING") {
		t.Fatal("the bell write does not deduplicate on (commune, key, recipient)")
	}
	if bell[0].args[1] != domain.ZaloKindAnnouncementPublished+":"+issued.ID {
		t.Errorf("key = %v, want thong-bao.moi:<announcement id>", bell[0].args[1])
	}
	if len(f.with("INSERT INTO zalo_delivery")) != 0 {
		t.Error("nothing created, yet a Zalo copy was queued")
	}
	delta := string(f.with("INSERT INTO audit_log")[0].args[7].([]byte))
	if !strings.Contains(delta, `"tao_moi":0`) || !strings.Contains(delta, `"da_co":2`) {
		t.Errorf("delta = %s", delta)
	}
}

func TestPublishAnnouncement_ClipsALongTitleForTheBellOnly(t *testing.T) {
	long := strings.Repeat("Đ", domain.TieuDeToiDa)
	f := bellFake(insertTuples, "")
	uc, ctx := newIssueUseCase(t, f, false)
	req := issueRequest("CB-2026-AAAA11")
	req.TieuDe = long
	if _, err := uc.PhatHanh(ctx, req, nguoiSoanMau()); err != nil {
		t.Fatalf("a 300-character title the book admits was refused by the bell: %v", err)
	}
	title, _ := f.with("INSERT INTO staff_notification")[0].args[3].(string)
	if n := len([]rune(title)); n != domain.MaxNotificationTitleLen || !strings.HasSuffix(title, "…") {
		t.Errorf("bell title is %d runes", n)
	}
	if book := f.with("INSERT INTO thong_bao\n"); len(book) != 1 || book[0].args[2] != long {
		t.Error("the announcement's own title was changed")
	}
}

func TestPublishAnnouncement_RefusesWithoutTheBellWired(t *testing.T) {
	f := bellFake(insertTuples, "")
	uc, ctx := newIssueUseCase(t, f, false)
	uc.bell = nil
	if _, err := uc.PhatHanh(ctx, issueRequest("CB-2026-AAAA11"), nguoiSoanMau()); !errors.Is(err, ErrBellNotWired) {
		t.Fatalf("err = %v, want ErrBellNotWired", err)
	}
	if f.begun != 0 {
		t.Error("a transaction opened for an act that cannot ring anybody's bell")
	}
}
