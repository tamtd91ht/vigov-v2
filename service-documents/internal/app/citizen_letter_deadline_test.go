package app

// ADR 0084 #3 / ADR 0085 B: the deadline a letter gets FROM THE COMMUNE'S RULE, asked of identity at
// the act that fixes it — PROCESSING at booking (from 00:00 Vietnam of the received date), RESOLUTION
// at `thu-ly` for a complaint / denunciation (from `accepted_at`).
//
// WHAT IS PROVEN: configured → stored as given and audited; not configured → NULL ("Không đặt hạn");
// ANY error → the act is refused with nothing written (no number, no status, no trail) — never NULL;
// a feedback letter / request never asks RESOLUTION; a clerk's later deadline still wins.

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

const (
	kindProcessing = identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_PROCESSING
	kindResolution = identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_RESOLUTION
)

// Not on a whole hour: anything that rounds or re-zones the instant identity gave shows.
var identityDue = time.Date(2027, 1, 12, 9, 30, 15, 0, time.UTC)

func configuredDirectory() *directoryFake {
	d := liveDirectory()
	due := identityDue
	d.dueAt = &due
	return d
}

func TestBookStoresTheProcessingDeadlineIdentityComputed(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo()
	dir := configuredDirectory()
	uc, ctx := buildLetters(t, k, repo, dir)

	l, err := uc.Book(ctx, bookingRequest(), clerk)
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	if len(dir.deadlineAsk) != 1 {
		t.Fatalf("hỏi hạn %d lần, muốn đúng 1", len(dir.deadlineAsk))
	}
	ask := dir.deadlineAsk[0]
	// bookingRequest: received 30/12/2026 → count from 00:00 of that day in Vietnam (= 17:00Z on 29/12).
	wantFrom := time.Date(2026, 12, 30, 0, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	if ask.letterType != identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_KHIEU_NAI || ask.kind != kindProcessing ||
		!ask.countFrom.Equal(wantFrom) {
		t.Fatalf("hỏi: %+v, muốn (khieu-nai, PROCESSING, %v)", ask, wantFrom)
	}
	got := repo.rows[l.ID]
	if !got.ProcessingDueAt.Equal(identityDue) || !got.ResolutionDueAt.IsZero() || !l.ProcessingDueAt.Equal(identityDue) {
		t.Fatalf("hạn lưu %v / %v, muốn đúng %v ở processing_due_at", got.ProcessingDueAt, got.ResolutionDueAt, identityDue)
	}
	if d := auditDelta(k, 0); !strings.Contains(d, `"han_xu_ly":"2027-01-12T09:30:15Z"`) {
		t.Fatalf("vết vào sổ thiếu hạn đã tính: %s", d)
	}
}

func TestBookNotConfiguredStoresNoDeadline(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo()
	dir := liveDirectory() // dueAt nil, dueErr nil = not configured
	uc, ctx := buildLetters(t, k, repo, dir)

	l, err := uc.Book(ctx, bookingRequest(), clerk)
	if err != nil {
		t.Fatalf("xã chưa cấu hình hạn mà không vào sổ được: %v", err)
	}
	if len(dir.deadlineAsk) != 1 || !repo.rows[l.ID].ProcessingDueAt.IsZero() {
		t.Fatalf("hỏi %d lần, hạn lưu %v — muốn hỏi 1 lần và không hạn", len(dir.deadlineAsk), repo.rows[l.ID].ProcessingDueAt)
	}
	if d := auditDelta(k, 0); !strings.Contains(d, `"han_xu_ly":""`) {
		t.Fatalf("vết vào sổ phải ghi 'không hạn': %s", d)
	}
}

func TestBookDenunciationAsksTheProcessingDeadlineOfToCao(t *testing.T) {
	dir := configuredDirectory()
	uc, ctx := buildLetters(t, khoVBMau(), newLetterRepo(), dir)
	req := bookingRequest()
	req.Type = domain.LetterTypeDenunciation
	if _, err := uc.Book(ctx, req, clerk); err != nil {
		t.Fatalf("Book: %v", err)
	}
	if len(dir.deadlineAsk) != 1 || dir.deadlineAsk[0].letterType != identityv1.CitizenLetterType_CITIZEN_LETTER_TYPE_TO_CAO ||
		dir.deadlineAsk[0].kind != kindProcessing {
		t.Fatalf("đơn tố cáo vào sổ phải hỏi hạn XỬ LÝ ĐƠN của to-cao: %+v", dir.deadlineAsk)
	}
}

// Every failure refuses the booking BEFORE the transaction: no number taken, no row, no trail — and
// never "no deadline", which the report would count as on time (ADR 0084 #4).
func TestBookRefusedWhenTheDeadlineCannotBeResolved(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"identity sập", fmt.Errorf("x: %w", identityclient.ErrIdentityUnavailable), ErrLetterDeadlineUnavailable},
		{"lỗi khác", errors.New("identity trả lỗi lạ"), ErrLetterDeadlineUnavailable},
		{"quy tắc hỏng", fmt.Errorf("x: %w", identityclient.ErrCitizenLetterDeadlineUnusable), ErrLetterDeadlineUnusable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k := khoVBMau()
			repo := newLetterRepo()
			dir := liveDirectory()
			dir.dueErr = c.err
			uc, ctx := buildLetters(t, k, repo, dir)

			_, err := uc.Book(ctx, bookingRequest(), clerk)
			if !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			if k.batDau != 0 || k.coCau("day_so_van_ban") || repo.writes != 0 || len(auditEntries(k)) != 0 {
				t.Fatalf("hỏi hạn hỏng mà vẫn mở giao dịch %d / lấy số / ghi %d / vết %d",
					k.batDau, repo.writes, len(auditEntries(k)))
			}
		})
	}
}

// --- thụ lý -----------------------------------------------------------------------------------------

func TestAdmittingAComplaintFixesTheResolutionDeadlineFromAcceptedAt(t *testing.T) {
	for _, typ := range []domain.LetterType{domain.LetterTypeComplaint, domain.LetterTypeDenunciation} {
		t.Run(string(typ), func(t *testing.T) {
			k := khoVBMau()
			l := letterIn("dt-1", domain.LetterStatusScreening, "")
			l.Type = typ
			repo := newLetterRepo(l)
			dir := configuredDirectory()
			uc, ctx := buildLetters(t, k, repo, dir)

			after, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusAdmitted}, clerk)
			if err != nil {
				t.Fatalf("Move: %v", err)
			}
			if len(dir.deadlineAsk) != 1 || dir.deadlineAsk[0].kind != kindResolution {
				t.Fatalf("thụ lý phải hỏi đúng một hạn GIẢI QUYẾT: %+v", dir.deadlineAsk)
			}
			got := repo.rows["dt-1"]
			if !dir.deadlineAsk[0].countFrom.Equal(got.AcceptedAt) || !got.AcceptedAt.Equal(newYearsNight) {
				t.Fatalf("gốc đếm %v, accepted_at %v — gốc đếm phải là accepted_at như đã lưu",
					dir.deadlineAsk[0].countFrom, got.AcceptedAt)
			}
			if !got.ResolutionDueAt.Equal(identityDue) || !after.ActiveDueAt().Equal(identityDue) {
				t.Fatalf("hạn giải quyết lưu %v, muốn %v", got.ResolutionDueAt, identityDue)
			}
			d := auditDelta(k, 0)
			if !strings.Contains(d, `"han_giai_quyet":""`) || !strings.Contains(d, `"han_giai_quyet":"2027-01-12T09:30:15Z"`) {
				t.Fatalf("vết thụ lý phải mang hạn giải quyết trước và sau: %s", d)
			}
		})
	}
}

func TestAdmittingAFeedbackLetterOrRequestNeverAsksResolution(t *testing.T) {
	for _, typ := range []domain.LetterType{domain.LetterTypeFeedback, domain.LetterTypeRequest} {
		t.Run(string(typ), func(t *testing.T) {
			l := letterIn("dt-1", domain.LetterStatusScreening, "")
			l.Type = typ
			repo := newLetterRepo(l)
			dir := configuredDirectory()
			dir.dueErr = errors.New("không được hỏi") // an ask would surface as a refusal
			uc, ctx := buildLetters(t, khoVBMau(), repo, dir)

			if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusAdmitted}, clerk); err != nil {
				t.Fatalf("Move: %v", err)
			}
			if len(dir.deadlineAsk) != 0 || !repo.rows["dt-1"].ResolutionDueAt.IsZero() {
				t.Fatalf("%s thụ lý mà hỏi hạn giải quyết: %+v", typ, dir.deadlineAsk)
			}
		})
	}
}

func TestAdmittingNotConfiguredStoresNoResolutionDeadline(t *testing.T) {
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusScreening, ""))
	dir := liveDirectory()
	uc, ctx := buildLetters(t, khoVBMau(), repo, dir)
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusAdmitted}, clerk); err != nil {
		t.Fatalf("xã chưa cấu hình mà không thụ lý được: %v", err)
	}
	got := repo.rows["dt-1"]
	if len(dir.deadlineAsk) != 1 || got.Status != domain.LetterStatusAdmitted || !got.ResolutionDueAt.IsZero() {
		t.Fatalf("hỏi %d lần, trạng thái %s, hạn %v", len(dir.deadlineAsk), got.Status, got.ResolutionDueAt)
	}
}

func TestAdmittingRefusedWhenTheDeadlineCannotBeResolved(t *testing.T) {
	for _, c := range []struct {
		err, want error
	}{
		{fmt.Errorf("x: %w", identityclient.ErrIdentityUnavailable), ErrLetterDeadlineUnavailable},
		{fmt.Errorf("x: %w", identityclient.ErrCitizenLetterDeadlineUnusable), ErrLetterDeadlineUnusable},
	} {
		k := khoVBMau()
		repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusScreening, ""))
		dir := liveDirectory()
		dir.dueErr = c.err
		uc, ctx := buildLetters(t, k, repo, dir)

		if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusAdmitted}, clerk); !errors.Is(err, c.want) {
			t.Fatalf("lỗi = %v, muốn %v", err, c.want)
		}
		got := repo.rows["dt-1"]
		if repo.writes != 0 || len(auditEntries(k)) != 0 || got.Status != domain.LetterStatusScreening || !got.AcceptedAt.IsZero() {
			t.Fatalf("hỏi hạn hỏng mà vẫn thụ lý: ghi %d, vết %d, trạng thái %s", repo.writes, len(auditEntries(k)), got.Status)
		}
	}
}

// A move refused on its own grounds answers with its own sentence, not a 503 — and asks nothing.
func TestAdmissionRefusalsComeBeforeTheIdentityCall(t *testing.T) {
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusScreening, ""), letterIn("dt-2", domain.LetterStatusNew, ""))
	dir := liveDirectory()
	dir.dueErr = fmt.Errorf("x: %w", identityclient.ErrIdentityUnavailable)
	uc, ctx := buildLetters(t, khoVBMau(), repo, dir)

	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusAdmitted}, reader); !errors.Is(err, domain.ErrLetterNotPermitted) {
		t.Fatalf("người không được giao: %v", err)
	}
	if _, err := uc.Move(ctx, "dt-2", MoveLetterRequest{Status: domain.LetterStatusAdmitted}, clerk); !errors.Is(err, domain.ErrLetterTransitionRefused) {
		t.Fatalf("mũi tên C3 không vẽ: %v", err)
	}
	if len(dir.deadlineAsk) != 0 || repo.writes != 0 {
		t.Fatalf("bị từ chối mà vẫn hỏi identity %d lần / ghi %d", len(dir.deadlineAsk), repo.writes)
	}
	// Other moves never ask a deadline at all.
	dir.dueErr = nil
	if _, err := uc.Move(ctx, "dt-2", MoveLetterRequest{Status: domain.LetterStatusScreening}, clerk); err != nil || len(dir.deadlineAsk) != 0 {
		t.Fatalf("chuyển sang dang-xu-ly-don mà hỏi hạn: %v, %d", err, len(dir.deadlineAsk))
	}
}

// ADR 0084 #3: "cán bộ vẫn sửa được" — the clerk's PATCH replaces the computed deadline.
func TestClerksDeadlineReplacesTheComputedOne(t *testing.T) {
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusScreening, ""))
	uc, ctx := buildLetters(t, khoVBMau(), repo, configuredDirectory())
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusAdmitted}, clerk); err != nil {
		t.Fatalf("Move: %v", err)
	}
	clerkDue := time.Date(2027, 2, 1, 10, 0, 0, 0, time.UTC)
	if _, err := uc.SetDeadline(ctx, "dt-1", clerkDue, clerk); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	if got := repo.rows["dt-1"].ResolutionDueAt; !got.Equal(clerkDue) {
		t.Fatalf("hạn sau khi cán bộ sửa = %v, muốn %v", got, clerkDue)
	}
}
