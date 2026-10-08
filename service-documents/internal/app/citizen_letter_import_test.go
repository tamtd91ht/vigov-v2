package app

// The Excel import and the report export over the REAL DaySoStore and core/audit on the fake driver,
// with the letters in the in-memory repository of citizen_letter_test.go.
//
// WHAT IS PROVEN: every imported row goes through Book's own code — number from the commune's
// `don-thu` series, the deadline identity fixed, source `nhap-excel`, its own `vao_so_don_thu` entry —
// inside ONE transaction with one batch entry; a refused row or an identity failure writes NOTHING and
// opens no transaction; the preview writes nothing; the export is audited after rendering and not at
// all when identity cannot name the units; neither trail carries personal data.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

type unitCodesFake struct {
	byCode map[string]string
	err    error
	asked  [][]string
	tenant tenant.ID
}

func (u *unitCodesFake) LiveOrgUnitIDsByCode(ctx context.Context, codes []string) (map[string]string, error) {
	u.asked = append(u.asked, codes)
	u.tenant = tenant.MustFrom(ctx)
	if u.err != nil {
		return nil, u.err
	}
	out := map[string]string{}
	for _, c := range codes {
		if id, ok := u.byCode[c]; ok {
			out[c] = id
		}
	}
	return out, nil
}

func letterImportRows() []domain.LetterImportRow {
	day := time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)
	return []domain.LetterImportRow{
		{Row: 2, ReceivedDate: day, Type: domain.LetterTypeFeedback, SenderName: "Nguyễn Văn A",
			SenderPhone: "0900000000", SenderAddress: "Thôn Bình An", Summary: "Đề nghị sửa đường thôn"},
		{Row: 3, ReceivedDate: day, Type: domain.LetterTypeFeedback, SenderName: "",
			Summary: "Phản ánh đèn đường hỏng", UnitCode: "dia-chinh"},
	}
}

func buildImport(t *testing.T, k *khoVBGia, repo *letterRepoFake, dir *directoryFake, units *unitCodesFake) (
	*CitizenLetterImport, context.Context) {
	uc, ctx := buildLetters(t, k, repo, dir)
	return NewCitizenLetterImport(uc, units), ctx
}

func liveUnitCodes() *unitCodesFake {
	return &unitCodesFake{byCode: map[string]string{"dia-chinh": "bp-dia-chinh"}}
}

func TestImportBooksEveryRowThroughBookInOneTransaction(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo()
	dir := liveDirectory()
	due := time.Date(2026, 12, 30, 10, 0, 0, 0, time.UTC)
	dir.dueAt = &due
	units := liveUnitCodes()
	imp, ctx := buildImport(t, k, repo, dir, units)

	res, err := imp.Import(ctx, letterImportRows(), clerk)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Letters) != 2 || res.Letters[0].Number != 1 || res.Letters[1].Number != 2 || res.Letters[0].Year != 2027 {
		t.Fatalf("số cấp: %+v — muốn 1/2027 và 2/2027 theo dãy đơn thư của xã", res.Letters)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("giao dịch: mở %d, commit %d — muốn đúng MỘT giao dịch cho cả tệp", k.batDau, k.daCommit)
	}
	for _, l := range res.Letters {
		got := repo.rows[l.ID]
		if got.Source != domain.LetterSourceExcel || got.CreatedByCode != "CB-00123" || got.Status != domain.LetterStatusNew {
			t.Fatalf("đơn nhập: nguồn %q, người %q, trạng thái %q", got.Source, got.CreatedByCode, got.Status)
		}
		if !got.ProcessingDueAt.Equal(due) {
			t.Fatalf("hạn xử lý = %v, muốn hạn identity trả %v (ADR 0085 B)", got.ProcessingDueAt, due)
		}
	}
	second := repo.rows[res.Letters[1].ID]
	if second.HoldingUnitID != "bp-dia-chinh" || !second.SenderUnknown() || !res.Letters[1].SenderUnknown {
		t.Fatalf("dòng 3: bộ phận %q, không rõ người gửi %v", second.HoldingUnitID, second.SenderUnknown())
	}
	checkLogRows(t, repo.logs)
	if len(repo.logs) != 1 || repo.logs[0].Content != logAssignedAtBooking || repo.logs[0].LetterID != second.ID {
		t.Fatalf("nhật ký: %+v — muốn đúng một dòng 'Phân công xử lý' cho đơn có bộ phận", repo.logs)
	}
	// ONE IDENTITY ASK for two rows of the same type and day (memoDirectory), and NONE for the unit:
	// LiveOrgUnitIDsByCode already applied the live predicate.
	if len(dir.deadlineAsk) != 1 {
		t.Fatalf("hỏi hạn %d lần, muốn 1", len(dir.deadlineAsk))
	}
	if units.tenant != xaA || len(units.asked) != 1 {
		t.Fatalf("tra mã bộ phận: xã %q, %d lần", units.tenant, len(units.asked))
	}

	entries := auditEntries(k)
	if len(entries) != 3 {
		t.Fatalf("%d vết, muốn 3 (hai vao_so_don_thu + một nhap_don_thu_tu_excel)", len(entries))
	}
	if entries[0].args[4] != ActionBookCitizenLetter || entries[1].args[4] != ActionBookCitizenLetter ||
		entries[2].args[4] != ActionImportCitizenLetters {
		t.Fatalf("hành vi: %v %v %v", entries[0].args[4], entries[1].args[4], entries[2].args[4])
	}
	for i := range entries {
		if entries[i].args[1] != "CB-00123" {
			t.Fatalf("vết %d mang người %v, muốn mã cán bộ", i, entries[i].args[1])
		}
		d := auditDelta(k, i)
		for _, pii := range []string{"Nguyễn", "0900000000", "Bình An", "sửa đường", "đèn đường"} {
			if strings.Contains(d, pii) {
				t.Fatalf("vết %d chứa dữ liệu cá nhân %q", i, pii)
			}
		}
	}
	if !strings.Contains(auditDelta(k, 0), `"nguon":"nhap-excel"`) {
		t.Fatalf("vết vào sổ không ghi nguồn nhập Excel: %s", auditDelta(k, 0))
	}
	if d := auditDelta(k, 2); !strings.Contains(d, "DT-2027-0001") || !strings.Contains(d, "DT-2027-0002") {
		t.Fatalf("vết nhập không liệt kê số đã cấp: %s", d)
	}
}

func TestImportUnknownUnitCodeIsARowErrorAndWritesNothing(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo()
	imp, ctx := buildImport(t, k, repo, liveDirectory(), &unitCodesFake{byCode: map[string]string{}})

	_, err := imp.Import(ctx, letterImportRows(), clerk)
	var rej *LetterImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) != 1 || rej.Errors[0].Row != 3 ||
		rej.Errors[0].Column != domain.LetterImportColUnit {
		t.Fatalf("err = %v", err)
	}
	if k.batDau != 0 || repo.writes != 0 || len(auditEntries(k)) != 0 {
		t.Fatalf("dòng lỗi mà đã mở giao dịch (%d) / ghi (%d)", k.batDau, repo.writes)
	}
}

func TestImportIdentityFailureRefusesTheWholeFileUnchecked(t *testing.T) {
	for name, set := range map[string]func(*directoryFake, *unitCodesFake){
		"hạn xử lý":  func(d *directoryFake, _ *unitCodesFake) { d.dueErr = errors.New("identity down") },
		"mã bộ phận": func(_ *directoryFake, u *unitCodesFake) { u.err = errors.New("identity down") },
	} {
		t.Run(name, func(t *testing.T) {
			k := khoVBMau()
			repo := newLetterRepo()
			dir, units := liveDirectory(), liveUnitCodes()
			set(dir, units)
			imp, ctx := buildImport(t, k, repo, dir, units)
			_, err := imp.Import(ctx, letterImportRows(), clerk)
			if !errors.Is(err, ErrLetterImportUnchecked) {
				t.Fatalf("err = %v, muốn ErrLetterImportUnchecked", err)
			}
			if k.batDau != 0 || repo.writes != 0 || len(k.cau("INSERT INTO day_so_van_ban")) != 0 {
				t.Fatal("identity lỗi mà đã mở giao dịch hoặc lấy số")
			}
		})
	}
}

func TestImportUnusableDeadlineRuleIsARowError(t *testing.T) {
	k := khoVBMau()
	dir := liveDirectory()
	dir.dueErr = identityclient.ErrCitizenLetterDeadlineUnusable
	imp, ctx := buildImport(t, k, newLetterRepo(), dir, liveUnitCodes())
	_, err := imp.Import(ctx, letterImportRows(), clerk)
	var rej *LetterImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) != 2 || rej.Errors[0].Column != domain.LetterImportColType {
		t.Fatalf("err = %v", err)
	}
	if k.batDau != 0 {
		t.Fatal("đã mở giao dịch")
	}
}

func TestImportPreviewWritesNothingAndShowsTheDeadline(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo()
	dir := liveDirectory()
	due := time.Date(2026, 12, 30, 10, 0, 0, 0, time.UTC)
	dir.dueAt = &due
	imp, ctx := buildImport(t, k, repo, dir, liveUnitCodes())
	res, err := imp.Preview(ctx, letterImportRows(), clerk)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(res.Errors) != 0 || len(res.Letters) != 2 || res.Letters[0].ID != "" || res.Letters[0].Number != 0 ||
		!res.Letters[1].ProcessingDueAt.Equal(due) || res.Letters[1].HoldingUnitID != "bp-dia-chinh" {
		t.Fatalf("xem trước: %+v", res)
	}
	if k.batDau != 0 || repo.writes != 0 || len(auditEntries(k)) != 0 {
		t.Fatal("xem trước đã ghi")
	}
	if dir.deadlineAsk[0].kind != identityv1.CitizenLetterDeadlineKind_CITIZEN_LETTER_DEADLINE_KIND_PROCESSING {
		t.Fatalf("hỏi sai loại hạn: %v", dir.deadlineAsk[0].kind)
	}
}

func TestImportRefusesWithoutAnActor(t *testing.T) {
	k := khoVBMau()
	imp, ctx := buildImport(t, k, newLetterRepo(), liveDirectory(), liveUnitCodes())
	if _, err := imp.Import(ctx, letterImportRows(), LetterCaller{}); err == nil {
		t.Fatal("nhập không có mã cán bộ mà không bị từ chối")
	}
	if k.batDau != 0 {
		t.Fatal("đã mở giao dịch")
	}
}

// --- report export ------------------------------------------------------------------------------------

type unitNamesFake struct {
	names map[string]identityclient.OrgUnitName
	err   error
}

func (u *unitNamesFake) OrgUnitNames(_ context.Context, ids []string) (map[string]identityclient.OrgUnitName, error) {
	if u.err != nil {
		return nil, u.err
	}
	out := map[string]identityclient.OrgUnitName{}
	for _, id := range ids {
		if n, ok := u.names[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

// reportRepo answers ReportRows with letters carrying personal data.
type reportRepo struct {
	*letterRepoFake
	rows []domain.CitizenLetter
}

func (r *reportRepo) ReportRows(context.Context, int, time.Time) ([]domain.CitizenLetter, error) {
	return r.rows, nil
}

func TestReportExportRendersThenAuditsWithoutPersonalData(t *testing.T) {
	k := khoVBMau()
	at := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &reportRepo{letterRepoFake: newLetterRepo(), rows: []domain.CitizenLetter{
		{ID: "a", Year: 2027, Number: 1, Type: domain.LetterTypeComplaint, CreatedAt: at, ReceivedDate: at,
			SenderName: "Nguyễn Văn A", SenderPhone: "0900000000", Summary: "Khiếu nại", Status: domain.LetterStatusNew,
			HoldingUnitID: "bp-dia-chinh"},
	}}
	uc, ctx := buildLetters(t, k, repo.letterRepoFake, liveDirectory())
	uc.repo = repo
	exp := NewCitizenLetterReportExport(uc, &unitNamesFake{names: map[string]identityclient.OrgUnitName{
		"bp-dia-chinh": {Name: "Địa chính", Standing: identityv1.RecordStanding_RECORD_STANDING_LIVE}}})

	var seen LetterReportSheet
	file, err := exp.Export(ctx, 2027, clerk.Actor, func(s LetterReportSheet) ([]byte, error) {
		seen = s
		return []byte("xlsx"), nil
	})
	if err != nil || string(file) != "xlsx" {
		t.Fatalf("Export: %v", err)
	}
	if seen.Report.Received != 1 || seen.UnitNames["bp-dia-chinh"].Name != "Địa chính" || !seen.UnitNames["bp-dia-chinh"].Known {
		t.Fatalf("dữ liệu dựng tệp: %+v", seen)
	}
	entries := auditEntries(k)
	if len(entries) != 1 || entries[0].args[4] != ActionExportCitizenLetterReport || entries[0].args[1] != "CB-00123" ||
		entries[0].args[5] != "bao-cao-don-thu/2027" {
		t.Fatalf("vết xuất: %+v", entries)
	}
	for _, pii := range []string{"Nguyễn", "0900000000", "Khiếu nại"} {
		if strings.Contains(auditDelta(k, 0), pii) {
			t.Fatalf("vết xuất chứa %q", pii)
		}
	}
}

func TestReportExportWithoutUnitNamesProducesNoFileAndNoEntry(t *testing.T) {
	k := khoVBMau()
	repo := &reportRepo{letterRepoFake: newLetterRepo(), rows: []domain.CitizenLetter{
		{ID: "a", Year: 2027, Type: domain.LetterTypeFeedback, Status: domain.LetterStatusNew, HoldingUnitID: "bp-x",
			CreatedAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}}}
	uc, ctx := buildLetters(t, k, repo.letterRepoFake, liveDirectory())
	uc.repo = repo
	exp := NewCitizenLetterReportExport(uc, &unitNamesFake{err: errors.New("identity down")})
	rendered := false
	_, err := exp.Export(ctx, 2027, clerk.Actor, func(LetterReportSheet) ([]byte, error) {
		rendered = true
		return nil, nil
	})
	if !errors.Is(err, ErrLetterReportNamesUnavailable) || rendered || len(auditEntries(k)) != 0 {
		t.Fatalf("err = %v, dựng tệp %v, %d vết", err, rendered, len(auditEntries(k)))
	}
}
