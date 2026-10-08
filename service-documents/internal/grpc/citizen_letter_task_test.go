package grpc

// ResolveCitizenLetterForTask (ADR 0085 A): commune-scoped read, NOT_FOUND for unknown / deleted /
// another commune's id, a denunciation answered with the bare eligibility, an eligible letter answered
// with codes, ids and one instant — and nothing personal, ever.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// fakeLetters holds live letters PER COMMUNE, read from the context as the store reads it. A
// soft-deleted letter is simply absent — the store's `deleted_at IS NULL` is proved against
// PostgreSQL; here the point is what the handler does with "not found".
type fakeLetters struct {
	byTenant map[tenant.ID]map[string]domain.CitizenLetter
	err      error
	calls    int
	gotID    string
}

func (f *fakeLetters) LiveByID(ctx context.Context, id string) (domain.CitizenLetter, bool, error) {
	f.calls++
	f.gotID = id
	if f.err != nil {
		return domain.CitizenLetter{}, false, f.err
	}
	l, ok := f.byTenant[tenant.MustFrom(ctx)][id]
	return l, ok, nil
}

const (
	pii1 = "Nguyễn Văn A"
	pii2 = "0900000000"
	pii3 = "Thôn Bình An"
	pii4 = "Đề nghị sửa đường liên thôn"
)

func letterBook() *fakeLetters {
	due := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	res := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	base := domain.CitizenLetter{SenderName: pii1, SenderPhone: pii2, SenderAddress: pii3, Summary: pii4,
		Year: 2026, HoldingUnitID: "bp-dia-chinh", AssigneeCode: "CB-00777", Source: domain.LetterSourceManual}
	mk := func(id string, n int, typ domain.LetterType, st domain.LetterStatus, mut func(*domain.CitizenLetter)) domain.CitizenLetter {
		l := base
		l.ID, l.Number, l.Type, l.Status = id, n, typ, st
		if mut != nil {
			mut(&l)
		}
		return l
	}
	return &fakeLetters{byTenant: map[tenant.ID]map[string]domain.CitizenLetter{
		testTenant: {
			"dt-new": mk("dt-new", 12, domain.LetterTypeFeedback, domain.LetterStatusNew, func(l *domain.CitizenLetter) {
				l.ProcessingDueAt = due
			}),
			"dt-admitted": mk("dt-admitted", 13, domain.LetterTypeComplaint, domain.LetterStatusResolving, func(l *domain.CitizenLetter) {
				l.ProcessingDueAt, l.ResolutionDueAt, l.AcceptedAt = due, res, due
			}),
			"dt-nodue": mk("dt-nodue", 14, domain.LetterTypeRequest, domain.LetterStatusFiled, func(l *domain.CitizenLetter) {
				l.HoldingUnitID, l.AssigneeCode = "", ""
			}),
			"dt-tc": mk("dt-tc", 15, domain.LetterTypeDenunciation, domain.LetterStatusScreening, func(l *domain.CitizenLetter) {
				l.ProcessingDueAt = due
			}),
		},
		otherTenant: {"dt-other": mk("dt-other", 1, domain.LetterTypeFeedback, domain.LetterStatusNew, nil)},
	}}
}

func lettersServer(f *fakeLetters) *Server {
	return NewServer(Deps{Incoming: &fakeCounter{}, DocumentTypes: &fakeTypes{}, Letters: f,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func askLetter(t *testing.T, f *fakeLetters, ctx context.Context, id string) (*documentsv1.ResolveCitizenLetterForTaskResponse, error) {
	t.Helper()
	return lettersServer(f).ResolveCitizenLetterForTask(ctx, &documentsv1.ResolveCitizenLetterForTaskRequest{LetterId: id})
}

// noPersonalData marshals the whole answer and refuses any sender or summary fragment in it.
func noPersonalData(t *testing.T, res *documentsv1.ResolveCitizenLetterForTaskResponse) {
	t.Helper()
	raw, err := proto.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{pii1, pii2, pii3, pii4, "nhap-tay"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("câu trả lời mang %q — không dữ liệu cá nhân nào qua ranh giới", leak)
		}
	}
}

func TestResolveCitizenLetterForTaskEligible(t *testing.T) {
	f := letterBook()
	ctx := tenant.Into(context.Background(), testTenant)

	res, err := askLetter(t, f, ctx, " dt-new ")
	if err != nil {
		t.Fatal(err)
	}
	if f.gotID != "dt-new" {
		t.Fatalf("id hỏi kho = %q", f.gotID)
	}
	src := res.GetLetter()
	if res.GetEligibility() != documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_ELIGIBLE || src == nil ||
		src.GetLetterId() != "dt-new" || src.GetNumber() != 12 || src.GetYear() != 2026 ||
		src.GetHoldingUnitId() != "bp-dia-chinh" || src.GetAssigneeCode() != "CB-00777" {
		t.Fatalf("= %+v", res)
	}
	if !src.GetCurrentDueAt().AsTime().Equal(time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("trước thụ lý: hạn = %v, muốn hạn xử lý đơn", src.GetCurrentDueAt())
	}
	noPersonalData(t, res)

	// Admitted: the resolution deadline.
	res, err = askLetter(t, f, ctx, "dt-admitted")
	if err != nil || !res.GetLetter().GetCurrentDueAt().AsTime().Equal(time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("sau thụ lý: %v %v, muốn hạn giải quyết", res, err)
	}
	// No deadline and no holder: unset and empty, never a default. Any status converts (no filter).
	res, err = askLetter(t, f, ctx, "dt-nodue")
	if err != nil || res.GetLetter() == nil || res.GetLetter().CurrentDueAt != nil ||
		res.GetLetter().GetHoldingUnitId() != "" || res.GetLetter().GetAssigneeCode() != "" {
		t.Fatalf("đơn không hạn: %+v %v", res, err)
	}
}

func TestResolveCitizenLetterForTaskDenunciationCarriesNothing(t *testing.T) {
	res, err := askLetter(t, letterBook(), tenant.Into(context.Background(), testTenant), "dt-tc")
	if err != nil {
		t.Fatal(err)
	}
	if res.GetEligibility() != documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_DENUNCIATION || res.Letter != nil {
		t.Fatalf("đơn tố cáo: %+v — chỉ được trả DENUNCIATION, không kèm gì (C9, ADR 0084 #6)", res)
	}
	noPersonalData(t, res)
}

func TestResolveCitizenLetterForTaskNotFoundIsOneAnswer(t *testing.T) {
	f := letterBook()
	ctx := tenant.Into(context.Background(), testTenant)
	// Unknown, and another commune's real id (a soft-deleted one is absent from the store's answer).
	for _, id := range []string{"khong-co", "dt-other"} {
		res, err := askLetter(t, f, ctx, id)
		if err != nil {
			t.Fatalf("%s: %v — NOT_FOUND là một câu trả lời OK, không phải lỗi", id, err)
		}
		if res.GetEligibility() != documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_NOT_FOUND || res.Letter != nil {
			t.Fatalf("%s: %+v", id, res)
		}
	}
}

func TestResolveCitizenLetterForTaskRefusals(t *testing.T) {
	f := letterBook()
	ctx := tenant.Into(context.Background(), testTenant)
	for _, id := range []string{"", "   ", strings.Repeat("a", 65)} {
		if _, err := askLetter(t, f, ctx, id); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("id %q: code = %v, want InvalidArgument", id, status.Code(err))
		}
	}
	if _, err := askLetter(t, f, ctx, strings.Repeat("a", 64)); err != nil {
		t.Fatalf("64 ký tự phải được nhận: %v", err)
	}
	calls := f.calls
	if _, err := askLetter(t, f, context.Background(), "dt-new"); status.Code(err) == codes.OK || f.calls != calls {
		t.Fatalf("không có xã: code %v, đọc kho %d lần", status.Code(err), f.calls-calls)
	}

	f.err = errors.New("pq: SELECT … password=secret")
	res, err := askLetter(t, f, ctx, "dt-new")
	if status.Code(err) != codes.Internal || res != nil {
		t.Fatalf("kho lỗi: res=%v code=%v — không bao giờ là NOT_FOUND hay ELIGIBLE", res, status.Code(err))
	}
	if st, _ := status.FromError(err); st.Message() != "lỗi nội bộ, vui lòng thử lại" {
		t.Fatalf("lộ nguyên nhân: %q", st.Message())
	}
}

func TestNewServerRefusesMissingLetters(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("built a server without the citizen-letter reader")
		}
	}()
	NewServer(Deps{Incoming: &fakeCounter{}, DocumentTypes: &fakeTypes{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}
