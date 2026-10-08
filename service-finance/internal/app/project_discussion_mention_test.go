package app

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/commsclient"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// The mention notice (ADR 0081 #5): sent AFTER the commit, once, to every mentioned colleague but the
// author; a comms failure or no comms at all never un-saves the comment.

type fakeMentionComms struct {
	k     *fakeDiscussionDB
	calls [][]commsclient.Notice

	committedAtCall int
	tenantAtCall    tenant.ID
	uncancellable   bool
	err             error
}

func (f *fakeMentionComms) DeliverStaffNotifications(ctx context.Context, n []commsclient.Notice) (map[string]commsclient.Delivery, error) {
	f.calls = append(f.calls, n)
	f.committedAtCall = f.k.committed
	f.tenantAtCall, _ = tenant.From(ctx)
	f.uncancellable = ctx.Done() == nil
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]commsclient.Delivery{}
	for _, x := range n {
		out[x.IdempotencyKey] = commsclient.Delivery{Created: uint32(len(x.RecipientMa))}
	}
	return out, nil
}

func buildMention(t *testing.T, comms *fakeMentionComms, k *fakeDiscussionDB, log *slog.Logger) (*ProjectDiscussion, context.Context) {
	t.Helper()
	uc, ctx := buildDiscussion(t, k)
	uc.newID = func() (string, error) { return idCommentNew, nil }
	if comms != nil {
		comms.k = k
		uc.NotifyMentionsThrough(comms, log)
	}
	return uc, ctx
}

func TestAddCommentNotifiesMentionedExceptAuthorAfterCommit(t *testing.T) {
	k := &fakeDiscussionDB{projectCode: "DA07", projectName: "Bê tông hoá đường"}
	comms := &fakeMentionComms{}
	uc, ctx := buildMention(t, comms, k, nil)

	// The author (clerk() = CB-00123) mentions themself; CB-00011 twice.
	if _, err := uc.AddComment(ctx, "da-001", ProjectCommentRequest{Body: "Đề nghị kiểm lại",
		MentionedStaffCodes: []string{"CB-00011", "CB-00123", " CB-00011 ", "CB-00012"}}, clerk()); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if len(comms.calls) != 1 || len(comms.calls[0]) != 1 {
		t.Fatalf("calls = %v, want exactly one call with one notice", comms.calls)
	}
	if comms.committedAtCall != 1 {
		t.Fatal("comms was called before the comment's transaction committed")
	}
	if comms.tenantAtCall != xaA {
		t.Fatalf("commune on the call = %q, want %q (rule 1, invariant 8)", comms.tenantAtCall, xaA)
	}
	n := comms.calls[0][0]
	if strings.Join(n.RecipientMa, ",") != "CB-00011,CB-00012" {
		t.Fatalf("recipients = %v — author excluded, duplicates collapsed", n.RecipientMa)
	}
	if n.Kind != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DISBURSEMENT_MENTION ||
		n.Title != "Bạn được nhắc trong trao đổi giải ngân: Bê tông hoá đường" ||
		n.Body != "Đề nghị kiểm lại" || n.Link != "/giai-ngan/du-an/da-001" ||
		n.IdempotencyKey != "giai-ngan.nhac-ten:"+idCommentNew {
		t.Fatalf("notice = %+v", n)
	}
}

func TestMentionNoticeKeyIsStableAndBoundsHold(t *testing.T) {
	long := strings.Repeat("ữ", 300)
	c := mkComment(long)
	a, _ := mentionNotice(c, strings.Repeat("Đ", 400))
	b, _ := mentionNotice(c, "khác")
	if a.IdempotencyKey != b.IdempotencyKey || a.IdempotencyKey != "giai-ngan.nhac-ten:"+idCommentNew {
		t.Fatalf("key not stable for one comment: %q / %q", a.IdempotencyKey, b.IdempotencyKey)
	}
	if got := []rune(a.Body); len(got) != 280 || string(got) != strings.Repeat("ữ", 280) {
		t.Fatalf("body = %d runes, want the first 280", len(got))
	}
	if len([]rune(a.Title)) != 200 || !strings.HasPrefix(a.Title, mentionTitlePrefix) {
		t.Fatalf("title = %d runes, want cut at comms' 200", len([]rune(a.Title)))
	}
	if short, _ := mentionNotice(mkComment("ngắn"), "X"); short.Body != "ngắn" {
		t.Fatalf("short body altered: %q", short.Body)
	}
}

func mkComment(body string) domain.ProjectComment {
	return domain.ProjectComment{ID: idCommentNew, ProjectID: "da-001", Body: body, AuthorCode: "CB-00123",
		MentionedStaffCodes: []string{"CB-00011"}}
}

func TestAddCommentNoCallWhenNobodyElseMentionedOrNoComms(t *testing.T) {
	for name, mentions := range map[string][]string{"no mention": nil, "only the author": {"CB-00123"}} {
		t.Run(name, func(t *testing.T) {
			k := &fakeDiscussionDB{projectCode: "DA07", projectName: "P"}
			comms := &fakeMentionComms{}
			uc, ctx := buildMention(t, comms, k, nil)
			if _, err := uc.AddComment(ctx, "da-001", ProjectCommentRequest{Body: "x", MentionedStaffCodes: mentions}, clerk()); err != nil {
				t.Fatalf("AddComment: %v", err)
			}
			if len(comms.calls) != 0 {
				t.Fatalf("comms called %d times with nobody to tell", len(comms.calls))
			}
		})
	}
	t.Run("comms not configured", func(t *testing.T) {
		k := &fakeDiscussionDB{projectCode: "DA07", projectName: "P"}
		uc, ctx := buildMention(t, nil, k, nil)
		if _, err := uc.AddComment(ctx, "da-001", ProjectCommentRequest{Body: "x",
			MentionedStaffCodes: []string{"CB-00011"}}, clerk()); err != nil || k.committed != 1 {
			t.Fatalf("AddComment with no comms: err %v, committed %d", err, k.committed)
		}
	})
}

func TestAddCommentCommsFailureKeepsCommentAndLogsNoText(t *testing.T) {
	k := &fakeDiscussionDB{projectCode: "DA07", projectName: "Dự án bí mật"}
	comms := &fakeMentionComms{err: fmt.Errorf("wrap: %w", commsclient.ErrCommsUnavailable)}
	var buf bytes.Buffer
	uc, ctx := buildMention(t, comms, k, slog.New(slog.NewJSONHandler(&buf, nil)))

	// A cancellable request context: the call must NOT inherit it (WithoutCancel) — a client hanging up
	// after the save must not drop the notice.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	got, err := uc.AddComment(ctx, "da-001", ProjectCommentRequest{Body: "Nội dung riêng tư 0900000000",
		MentionedStaffCodes: []string{"CB-00011"}}, clerk())
	if err != nil || got.ID != idCommentNew {
		t.Fatalf("a comms failure failed the request: %v", err)
	}
	if k.committed != 1 || len(comms.calls) != 1 {
		t.Fatalf("committed %d, calls %d — want 1/1", k.committed, len(comms.calls))
	}
	if !comms.uncancellable {
		t.Fatal("the call inherited the request's cancellation")
	}
	line := buf.String()
	if !strings.Contains(line, idCommentNew) || !strings.Contains(line, `"recipients":1`) ||
		!strings.Contains(line, `"comms_unavailable":true`) {
		t.Fatalf("failure not logged with id and count: %s", line)
	}
	for _, leaked := range []string{"riêng tư", "0900000000", "bí mật", "CB-00011"} {
		if strings.Contains(line, leaked) {
			t.Fatalf("log line carries %q: %s", leaked, line)
		}
	}
}
