package app

// The citizen-letter use cases, over the REAL DaySoStore and the REAL core/audit on the fake driver
// (driver_gia_van_ban_test.go) — so the number series and the audit INSERT are real statements inside
// a real transaction boundary — with the letter rows themselves in an in-memory repository.
//
// WHAT IS PROVEN: the number comes from series 'don-thu' of THIS commune for the year of the act,
// inside the booking transaction; every act writes its log row and one audit entry whose actor is the
// staff BUSINESS CODE; a refusal writes nothing; C3 and C13/C14 refuse; the log rows satisfy 0006's
// CHECKs; a denunciation read that discloses something is audited; the sender never enters the trail.

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// --- fakes ------------------------------------------------------------------------------------------

type letterRepoFake struct {
	rows   map[string]domain.CitizenLetter
	logs   []domain.LetterLogEntry
	writes int
	listF  docstore.CitizenLetterFilter
}

func newLetterRepo(rows ...domain.CitizenLetter) *letterRepoFake {
	r := &letterRepoFake{rows: map[string]domain.CitizenLetter{}}
	for _, l := range rows {
		r.rows[l.ID] = l
	}
	return r
}

func (r *letterRepoFake) get(id string) (domain.CitizenLetter, error) {
	l, ok := r.rows[id]
	if !ok {
		return l, docstore.ErrCitizenLetterNotFound
	}
	return l, nil
}
func (r *letterRepoFake) ByID(_ context.Context, _ *store.ScopedTx, id string) (domain.CitizenLetter, error) {
	return r.get(id)
}
func (r *letterRepoFake) ForUpdate(_ context.Context, _ *store.ScopedTx, id string) (domain.CitizenLetter, error) {
	return r.get(id)
}
func (r *letterRepoFake) Insert(_ context.Context, _ *store.ScopedTx, l domain.CitizenLetter) error {
	r.writes++
	r.rows[l.ID] = l
	return nil
}
func (r *letterRepoFake) UpdateHolder(_ context.Context, _ *store.ScopedTx, id, unit, assignee, _ string) error {
	r.writes++
	l := r.rows[id]
	l.HoldingUnitID, l.AssigneeCode = unit, assignee
	r.rows[id] = l
	return nil
}
func (r *letterRepoFake) UpdateStatus(_ context.Context, _ *store.ScopedTx, l domain.CitizenLetter, _ string) error {
	r.writes++
	r.rows[l.ID] = l
	return nil
}
func (r *letterRepoFake) UpdateResult(_ context.Context, _ *store.ScopedTx, l domain.CitizenLetter, _ string) error {
	r.writes++
	r.rows[l.ID] = l
	return nil
}
func (r *letterRepoFake) UpdateSender(_ context.Context, _ *store.ScopedTx, l domain.CitizenLetter, _ string) error {
	r.writes++
	r.rows[l.ID] = l
	return nil
}
func (r *letterRepoFake) InsertLog(_ context.Context, _ *store.ScopedTx, e domain.LetterLogEntry) error {
	r.writes++
	r.logs = append(r.logs, e)
	return nil
}
func (r *letterRepoFake) Log(_ context.Context, _ *store.ScopedTx, id string) ([]domain.LetterLogEntry, error) {
	var out []domain.LetterLogEntry
	for _, e := range r.logs {
		if e.LetterID == id {
			out = append(out, e)
		}
	}
	return out, nil
}
func (r *letterRepoFake) List(_ context.Context, f docstore.CitizenLetterFilter, _ page.Request) (page.Result[domain.CitizenLetter], error) {
	r.listF = f
	return page.NewResult[domain.CitizenLetter](), nil
}
func (r *letterRepoFake) DuplicateCandidates(context.Context, string, time.Time) ([]domain.CitizenLetter, error) {
	return nil, nil
}
func (r *letterRepoFake) ReportRows(context.Context, int, time.Time) ([]domain.CitizenLetter, error) {
	return nil, nil
}

type directoryFake struct {
	units, staff map[string]bool
	myUnits      []string
	err          error
	askedCode    string
}

func (d *directoryFake) LiveOrgUnits(_ context.Context, ids []string) (map[string]struct{}, error) {
	if d.err != nil {
		return nil, d.err
	}
	out := map[string]struct{}{}
	for _, id := range ids {
		if d.units[id] {
			out[id] = struct{}{}
		}
	}
	return out, nil
}

// vi-name-ok: implements the existing core/identityclient.Client method name
func (d *directoryFake) CanBoGiaoViecDuoc(_ context.Context, codes []string) (map[string]struct{}, error) {
	if d.err != nil {
		return nil, d.err
	}
	out := map[string]struct{}{}
	for _, c := range codes {
		if d.staff[c] {
			out[c] = struct{}{}
		}
	}
	return out, nil
}
func (d *directoryFake) StaffOrgUnits(_ context.Context, code string) ([]string, error) {
	d.askedCode = code
	return d.myUnits, d.err
}

// --- wiring -----------------------------------------------------------------------------------------

var (
	clerk   = LetterCaller{Actor: audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}, CanBook: true}
	officer = LetterCaller{Actor: audit.Actor{ID: "CB-00777", Kind: "staff", IP: "10.0.0.8"}}
	reader  = LetterCaller{Actor: audit.Actor{ID: "CB-00555", Kind: "staff", IP: "10.0.0.9"}}

	// 01:00 on 01/01/2027 in Vietnam, still 2026 in UTC: the series year must be 2027.
	newYearsNight = time.Date(2026, 12, 31, 18, 0, 0, 0, time.UTC)
)

func buildLetters(t *testing.T, k *khoVBGia, repo *letterRepoFake, dir *directoryFake) (*CitizenLetters, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	uc := NewCitizenLetters(kho, repo, docstore.NewDaySoStore(kho), dir)
	n := 0
	uc.newID = func() (string, error) {
		n++
		return "01JLETTER" + strings.Repeat("0", 14) + string(rune('A'+n)), nil
	}
	uc.now = func() time.Time { return newYearsNight }
	return uc, tenant.Into(context.Background(), xaA)
}

func liveDirectory() *directoryFake {
	return &directoryFake{units: map[string]bool{"bp-dia-chinh": true, "bp-tu-phap": true},
		staff: map[string]bool{"CB-00777": true}}
}

func bookingRequest() BookLetterRequest {
	return BookLetterRequest{ReceivedDate: time.Date(2026, 12, 30, 0, 0, 0, 0, time.UTC),
		Type: domain.LetterTypeComplaint, SenderName: " Nguyễn Văn A ", SenderPhone: "0900000000",
		SenderAddress: "Thôn Bình An", Summary: " Khiếu nại quyết định thu hồi đất "}
}

func letterIn(id string, status domain.LetterStatus, assignee string) domain.CitizenLetter {
	l := domain.CitizenLetter{ID: id, Number: 5, Year: 2026, Type: domain.LetterTypeComplaint,
		ReceivedDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Summary: "Khiếu nại",
		SenderName: "Nguyễn Văn A", SenderPhone: "0900000000", Status: status, AssigneeCode: assignee,
		HoldingUnitID: "bp-tu-phap"}
	if status == domain.LetterStatusAdmitted || status == domain.LetterStatusResolving {
		l.AcceptedAt = time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC)
	}
	return l
}

// auditEntries returns the audit INSERTs: args are (tenant, actor_id, actor_kind, actor_ip, action,
// subject, at, delta) — core/audit.Write's order.
func auditEntries(k *khoVBGia) []lenhGhi { return k.cau("INSERT INTO audit_log") }

func auditDelta(k *khoVBGia, i int) string { return string(auditEntries(k)[i].args[7].([]byte)) }

// checkLogRows asserts every log row satisfies 0006's CHECKs on `citizen_letter_log`.
func checkLogRows(t *testing.T, logs []domain.LetterLogEntry) {
	t.Helper()
	kinds := map[domain.LetterLogKind]bool{domain.LetterLogStatusChange: true, domain.LetterLogRouting: true,
		domain.LetterLogNote: true, domain.LetterLogResult: true, domain.LetterLogSenderCorrection: true}
	for _, e := range logs {
		if !kinds[e.Kind] || strings.TrimSpace(e.ActorCode) == "" {
			t.Fatalf("dòng nhật ký sai loại hoặc thiếu người: %+v", e)
		}
		if (e.FromStatus == "") != (e.ToStatus == "") || (e.FromStatus != "" && e.FromStatus == e.ToStatus) {
			t.Fatalf("cặp trạng thái không hợp CHECK status_pair: %+v", e)
		}
		switch e.Kind {
		case domain.LetterLogStatusChange:
			if e.ToStatus == "" {
				t.Fatalf("chuyen-trang-thai thiếu to_status: %+v", e)
			}
		case domain.LetterLogNote, domain.LetterLogSenderCorrection:
			if e.ToStatus != "" {
				t.Fatalf("%s không được mang trạng thái: %+v", e.Kind, e)
			}
		}
		if e.Kind == domain.LetterLogRouting {
			if e.ToUnitID == "" || strings.TrimSpace(e.Content) == "" {
				t.Fatalf("luan-chuyen thiếu bộ phận nhận hoặc lý do: %+v", e)
			}
		} else if e.FromUnitID != "" || e.ToUnitID != "" || e.AssigneeCode != "" {
			t.Fatalf("%s mang bộ ba luân chuyển: %+v", e.Kind, e)
		}
		if e.Kind == domain.LetterLogNote && strings.TrimSpace(e.Content) == "" {
			t.Fatalf("ghi-chu rỗng: %+v", e)
		}
	}
}

// --- booking ----------------------------------------------------------------------------------------

func TestBookTakesTheCommunesLetterSeriesNumberForTheYearOfTheAct(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo()
	uc, ctx := buildLetters(t, k, repo, liveDirectory())

	first, err := uc.Book(ctx, bookingRequest(), clerk)
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	second, err := uc.Book(ctx, bookingRequest(), clerk)
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	if first.Year != 2027 || first.Number != 1 || second.Number != 2 {
		t.Fatalf("số = %d/%d và %d/%d, muốn 1/2027 và 2/2027 (năm của hành vi vào sổ theo giờ Việt Nam)",
			first.Number, first.Year, second.Number, second.Year)
	}
	open := k.cau("INSERT INTO day_so_van_ban")[0]
	if open.args[0] != string(xaA) || open.args[1] != "don-thu" || open.args[2] != int64(2027) {
		t.Fatalf("dãy số mở với %v, muốn (xã A, don-thu, 2027)", open.args)
	}
	if k.soCuoi["don-thu|2027"] != 2 || k.soCuoi["den|2027"] != 0 {
		t.Fatalf("bộ đếm: %v — dãy đơn thư phải độc lập với văn bản đến", k.soCuoi)
	}
	if k.batDau != 2 || k.daCommit != 2 {
		t.Fatalf("giao dịch: mở %d, commit %d — muốn mỗi lần vào sổ đúng một giao dịch", k.batDau, k.daCommit)
	}
	got := repo.rows[first.ID]
	if got.SenderName != "Nguyễn Văn A" || got.Summary != "Khiếu nại quyết định thu hồi đất" || got.CreatedByCode != "CB-00123" {
		t.Fatalf("dòng lưu chưa được cắt khoảng trắng hoặc sai người tạo: %+v", got)
	}
	if !got.ProcessingDueAt.IsZero() || !got.ResolutionDueAt.IsZero() || got.Status != domain.LetterStatusNew {
		t.Fatal("đơn mới phải `moi-vao-so` và CHƯA có hạn (ADR 0078 #3)")
	}
}

func TestBookAuditsWithTheBusinessCodeAndNoPersonalData(t *testing.T) {
	k := khoVBMau()
	uc, ctx := buildLetters(t, k, newLetterRepo(), liveDirectory())
	if _, err := uc.Book(ctx, bookingRequest(), clerk); err != nil {
		t.Fatalf("Book: %v", err)
	}
	entries := auditEntries(k)
	if len(entries) != 1 {
		t.Fatalf("%d vết, muốn 1", len(entries))
	}
	a := entries[0].args
	if a[1] != "CB-00123" || a[4] != ActionBookCitizenLetter || a[5] != "DT-2027-0001" {
		t.Fatalf("vết: người %v, hành vi %v, đối tượng %v", a[1], a[4], a[5])
	}
	for _, pii := range []string{"Nguyễn", "0900000000", "Bình An", "thu hồi đất"} {
		if strings.Contains(auditDelta(k, 0), pii) {
			t.Fatalf("vết vào sổ chứa dữ liệu cá nhân %q", pii)
		}
	}
}

func TestBookRefusedRelatedLetterTakesNoNumber(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo()
	uc, ctx := buildLetters(t, k, repo, liveDirectory())
	req := bookingRequest()
	req.RelatedLetterID = "khong-co"
	if _, err := uc.Book(ctx, req, clerk); !errors.Is(err, domain.ErrLetterRelatedNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrLetterRelatedNotFound", err)
	}
	if k.coCau("day_so_van_ban") || repo.writes != 0 || len(auditEntries(k)) != 0 || k.daRollback != 1 {
		t.Fatal("đơn bị từ chối vẫn đụng tới dãy số, ghi dòng hoặc ghi vết")
	}
}

func TestBookRoutedAtBookingKeepsTheStatusAndLogsTheRouting(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo()
	dir := liveDirectory()
	uc, ctx := buildLetters(t, k, repo, dir)

	req := bookingRequest()
	req.HoldingUnitID = "bp-khong-ton-tai"
	if _, err := uc.Book(ctx, req, clerk); !errors.Is(err, domain.ErrLetterUnitNotLive) || k.batDau != 0 {
		t.Fatalf("bộ phận không còn: lỗi %v, giao dịch mở %d", err, k.batDau)
	}
	req.HoldingUnitID = "bp-dia-chinh"
	l, err := uc.Book(ctx, req, clerk)
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	if l.Status != domain.LetterStatusNew || l.HoldingUnitID != "bp-dia-chinh" {
		t.Fatalf("phân công là thuộc tính (C3): trạng thái %s, bộ phận %s", l.Status, l.HoldingUnitID)
	}
	if len(repo.logs) != 1 || repo.logs[0].Kind != domain.LetterLogRouting || repo.logs[0].ToUnitID != "bp-dia-chinh" {
		t.Fatalf("thiếu dòng luan-chuyen: %+v", repo.logs)
	}
	checkLogRows(t, repo.logs)

	dir.err = errors.New("identity sập")
	if _, err := uc.Book(ctx, req, clerk); !errors.Is(err, ErrLetterDirectoryUnavailable) {
		t.Fatalf("identity không trả lời mà lỗi = %v — không bao giờ ghi khi chưa kiểm được", err)
	}
}

func TestBookAuditFailureRollsEverythingBack(t *testing.T) {
	k := khoVBMau()
	k.loiSau = "INSERT INTO audit_log"
	uc, ctx := buildLetters(t, k, newLetterRepo(), liveDirectory())
	if _, err := uc.Book(ctx, bookingRequest(), clerk); err == nil {
		t.Fatal("vết hỏng mà vào sổ vẫn thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — vết và số phải cùng một giao dịch (luật 6 bất biến 3)", k.daCommit, k.daRollback)
	}
}

// --- routing ----------------------------------------------------------------------------------------

func TestRouteIsAnAttributeNotAStatus(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, ""))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())

	if _, err := uc.Route(ctx, "dt-1", RouteLetterRequest{ToUnitID: "bp-dia-chinh", AssigneeCode: "CB-00999",
		Reason: "Thuộc đất đai"}, clerk); !errors.Is(err, domain.ErrLetterAssigneeNotLive) {
		t.Fatalf("cán bộ không nhận việc được mà lỗi = %v", err)
	}
	l, err := uc.Route(ctx, "dt-1", RouteLetterRequest{ToUnitID: "bp-dia-chinh", AssigneeCode: "CB-00777",
		Reason: "Thuộc đất đai"}, clerk)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if l.Status != domain.LetterStatusNew || repo.rows["dt-1"].AssigneeCode != "CB-00777" {
		t.Fatalf("trạng thái %s, cán bộ %s — chuyển không đổi trạng thái", l.Status, repo.rows["dt-1"].AssigneeCode)
	}
	e := repo.logs[0]
	if e.FromUnitID != "bp-tu-phap" || e.ToUnitID != "bp-dia-chinh" || e.AssigneeCode != "CB-00777" || e.ToStatus != "" {
		t.Fatalf("dòng luan-chuyen: %+v", e)
	}
	checkLogRows(t, repo.logs)
	if a := auditEntries(k); len(a) != 1 || a[0].args[4] != ActionRouteCitizenLetter || a[0].args[1] != "CB-00123" {
		t.Fatal("chuyển đơn phải có đúng một vết, người là mã cán bộ")
	}

	repo.rows["dt-1"] = letterIn("dt-1", domain.LetterStatusFiled, "")
	if _, err := uc.Route(ctx, "dt-1", RouteLetterRequest{ToUnitID: "bp-dia-chinh", Reason: "x"}, clerk); !errors.Is(err, domain.ErrLetterFinished) {
		t.Fatalf("đơn đã lưu mà vẫn chuyển được: %v", err)
	}
}

// --- status -----------------------------------------------------------------------------------------

func TestMoveWhoMayAct(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, "CB-00777"))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())

	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening}, reader); !errors.Is(err, domain.ErrLetterNotPermitted) {
		t.Fatalf("chỉ có quyền xem, không được giao, mà lỗi = %v", err)
	}
	if repo.writes != 0 || len(auditEntries(k)) != 0 {
		t.Fatal("bị từ chối mà vẫn ghi")
	}
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening, Note: "Bắt đầu xem xét"}, officer); err != nil {
		t.Fatalf("cán bộ được giao bị từ chối: %v", err)
	}
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusAdmitted}, clerk); err != nil {
		t.Fatalf("người có quyền tiếp nhận bị từ chối: %v", err)
	}
	got := repo.rows["dt-1"]
	if got.Status != domain.LetterStatusAdmitted || !got.AcceptedAt.Equal(newYearsNight) {
		t.Fatalf("thụ lý phải ghi accepted_at: %+v", got)
	}
	if e := repo.logs[0]; e.Kind != domain.LetterLogStatusChange || e.FromStatus != domain.LetterStatusNew ||
		e.ToStatus != domain.LetterStatusScreening || e.Content != "Bắt đầu xem xét" {
		t.Fatalf("dòng chuyen-trang-thai: %+v", e)
	}
	checkLogRows(t, repo.logs)
	if a := auditEntries(k); len(a) != 2 || a[0].args[1] != "CB-00777" || a[1].args[1] != "CB-00123" {
		t.Fatal("mỗi lần đổi trạng thái đúng một vết, người là mã cán bộ thực hiện")
	}
}

func TestMoveRefusesArrowsC3DoesNotDraw(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, ""))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())
	_, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusAdmitted}, clerk)
	if !errors.Is(err, domain.ErrLetterTransitionRefused) || repo.writes != 0 {
		t.Fatalf("moi-vao-so → thu-ly phải bị từ chối và không ghi gì: %v", err)
	}
}

func TestResolvedNeedsTheResultFirst(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusResolving, "CB-00777"),
		letterIn("dt-2", domain.LetterStatusNew, "CB-00777"))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())

	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusResolved}, officer); !errors.Is(err, domain.ErrLetterNeedsResult) {
		t.Fatalf("đóng đơn khi chưa có kết quả: %v (C10 — không đóng im lặng)", err)
	}
	res := LetterResultRequest{DocumentNo: "12/TB-UBND", DocumentDate: time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
		Signer: "Chủ tịch UBND xã", Issuer: "UBND xã", Summary: "Đã trả lời bằng văn bản"}
	if _, err := uc.RecordResult(ctx, "dt-1", res, officer); err != nil {
		t.Fatalf("RecordResult: %v", err)
	}
	writes := repo.writes
	if _, err := uc.RecordResult(ctx, "dt-1", res, officer); err != nil || repo.writes != writes {
		t.Fatalf("ghi lại cùng kết quả phải không ghi gì: lỗi %v, ghi thêm %d", err, repo.writes-writes)
	}
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusResolved}, officer); err != nil {
		t.Fatalf("đã có kết quả mà không đóng được: %v", err)
	}
	got := repo.rows["dt-1"]
	if !got.ResolvedAt.Equal(newYearsNight) || got.Status != domain.LetterStatusResolved {
		t.Fatalf("đã giải quyết phải ghi resolved_at: %+v", got)
	}
	if len(repo.logs) != 2 || repo.logs[0].Kind != domain.LetterLogResult || repo.logs[1].Kind != domain.LetterLogStatusChange {
		t.Fatalf("nhật ký: %+v", repo.logs)
	}
	checkLogRows(t, repo.logs)
	if strings.Contains(auditDelta(k, 0), "Đã trả lời") {
		t.Fatal("tóm tắt kết quả lọt vào vết")
	}

	if _, err := uc.RecordResult(ctx, "dt-2", res, officer); !errors.Is(err, domain.ErrLetterResultNotAllowed) {
		t.Fatalf("ghi kết quả khi mới vào sổ: %v", err)
	}
}

// --- sender, notes ----------------------------------------------------------------------------------

func TestCorrectSenderNeverWritesValuesIntoLogOrTrail(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusScreening, ""))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())

	newName, clear := "Nguyễn Văn Bê", ""
	l, err := uc.CorrectSender(ctx, "dt-1", SenderCorrection{Name: &newName, Phone: &clear}, clerk)
	if err != nil {
		t.Fatalf("CorrectSender: %v", err)
	}
	if l.SenderName != newName || l.SenderPhone != "" {
		t.Fatalf("sửa người gửi: %+v", l)
	}
	if repo.logs[0].Kind != domain.LetterLogSenderCorrection || repo.logs[0].Content != "Sửa thông tin người gửi" {
		t.Fatalf("dòng sua-nguoi-gui: %+v", repo.logs[0])
	}
	delta := auditDelta(k, 0)
	for _, v := range []string{"Nguyễn", "0900000000"} {
		if strings.Contains(delta, v) {
			t.Fatalf("vết chứa giá trị người gửi %q", v)
		}
	}
	if !strings.Contains(delta, `"sender_name":true`) || !strings.Contains(delta, `"sender_address":false`) {
		t.Fatalf("vết phải ghi trường nào đổi: %s", delta)
	}
	checkLogRows(t, repo.logs)

	before := repo.writes
	if _, err := uc.CorrectSender(ctx, "dt-1", SenderCorrection{Name: &newName}, clerk); err != nil || repo.writes != before {
		t.Fatal("sửa không đổi gì mà vẫn ghi")
	}
}

func TestAddNote(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusScreening, "CB-00777"))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())
	if _, err := uc.AddNote(ctx, "dt-1", "Đã gọi điện cho người gửi", reader); !errors.Is(err, domain.ErrLetterNotPermitted) {
		t.Fatalf("người chỉ xem ghi được nhật ký: %v", err)
	}
	e, err := uc.AddNote(ctx, "dt-1", "  Đã gọi điện cho người gửi ", officer)
	if err != nil || e.Kind != domain.LetterLogNote || e.Content != "Đã gọi điện cho người gửi" {
		t.Fatalf("AddNote: %+v, %v", e, err)
	}
	if strings.Contains(auditDelta(k, 0), "gọi điện") {
		t.Fatal("nội dung ghi chú lọt vào vết")
	}
	checkLogRows(t, repo.logs)
}

// --- reads ------------------------------------------------------------------------------------------

func TestDetailOfADenunciationIsAuditedOnlyWhenItDiscloses(t *testing.T) {
	den := letterIn("dt-1", domain.LetterStatusScreening, "CB-00777")
	den.Type = domain.LetterTypeDenunciation
	k := khoVBMau()
	uc, ctx := buildLetters(t, k, newLetterRepo(den), liveDirectory())

	if _, show, err := uc.Detail(ctx, "dt-1", reader); err != nil || show.Identity || show.Summary || len(auditEntries(k)) != 0 {
		t.Fatalf("người chỉ xem: %+v, %v, %d vết", show, err, len(auditEntries(k)))
	}
	if _, show, err := uc.Detail(ctx, "dt-1", officer); err != nil || !show.Identity || len(auditEntries(k)) != 1 {
		t.Fatalf("cán bộ được giao: %+v, %v", show, err)
	}
	if a := auditEntries(k)[0].args; a[4] != ActionViewDenunciation || a[1] != "CB-00777" {
		t.Fatalf("vết xem tố cáo: %v", a)
	}

	k2 := khoVBMau()
	uc2, ctx2 := buildLetters(t, k2, newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, "")), liveDirectory())
	if _, _, err := uc2.Detail(ctx2, "dt-1", reader); err != nil || len(auditEntries(k2)) != 0 {
		t.Fatal("đọc đơn thường không cần vết")
	}
}

func TestListScopes(t *testing.T) {
	repo := newLetterRepo()
	dir := liveDirectory()
	dir.myUnits = []string{"bp-tu-phap"}
	uc, ctx := buildLetters(t, khoVBMau(), repo, dir)
	req, err := page.New(docstore.SortCitizenLetters, "", "", "", "")
	if err != nil {
		t.Fatalf("page.New: %v", err)
	}

	if _, err := uc.List(ctx, LetterListQuery{Scope: "mine", CallerCode: "CB-00777"}, req); err != nil || repo.listF.MineCode != "CB-00777" {
		t.Fatalf("mine: %v %+v", err, repo.listF)
	}
	if _, err := uc.List(ctx, LetterListQuery{Scope: "related", CallerCode: "CB-00777"}, req); err != nil {
		t.Fatalf("related: %v", err)
	}
	if dir.askedCode != "CB-00777" || repo.listF.Related == nil || repo.listF.Related.OrgUnits[0] != "bp-tu-phap" {
		t.Fatalf("related phải hỏi bộ phận của chính phiên: %+v", repo.listF.Related)
	}
	dir.err = errors.New("identity sập")
	if _, err := uc.List(ctx, LetterListQuery{Scope: "related", CallerCode: "CB-00777"}, req); !errors.Is(err, ErrLetterDirectoryUnavailable) {
		t.Fatalf("identity không trả lời mà tab vẫn chạy (bỏ mệnh đề bộ phận): %v", err)
	}
	if _, err := uc.List(ctx, LetterListQuery{Scope: "everyone"}, req); !errors.Is(err, ErrLetterScopeInvalid) {
		t.Fatal("phạm vi lạ phải bị từ chối")
	}
}
