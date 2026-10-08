package app

import (
	"context"
	"errors"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/documentsclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	"github.com/vihat/vigov/service-petitions/migrations"
)

// CitizenLetterTaskCreation on the REAL GhiNhiemVu over the task driver (dungGhiNhiemVu): what reaches
// the INSERT and the audit entry is what PostgreSQL would receive. documents is a fake resolver.

const (
	letterIDThu      = "01JDONTHU0000000000000000A"
	letterUnitThu    = "bp-dia-chinh"
	letterAssignee   = "CB-00012"
	letterSummaryThu = "Ông Nguyễn Văn A phản ánh tranh chấp ranh giới đất" // must never reach the trail
)

type letterResolverFake struct {
	answer  documentsclient.CitizenLetterForTask
	err     error
	calls   int
	asked   string
	commune tenant.ID
}

func (f *letterResolverFake) ResolveCitizenLetterForTask(ctx context.Context, id string) (
	documentsclient.CitizenLetterForTask, error) {
	f.calls++
	f.asked = id
	f.commune, _ = tenant.From(ctx)
	return f.answer, f.err
}

func eligibleLetter(due time.Time) *letterResolverFake {
	return &letterResolverFake{answer: documentsclient.CitizenLetterForTask{
		Eligibility: documentsclient.CitizenLetterEligible, LetterID: letterIDThu, Number: 12, Year: 2026,
		HoldingUnitID: letterUnitThu, AssigneeCode: letterAssignee, CurrentDueAt: due,
	}}
}

// buildLetterTask wires the act over the real create path, with every staff code assignable.
func buildLetterTask(t *testing.T, k *khoNhiemVuGia, letters CitizenLetterResolver) (
	*CitizenLetterTaskCreation, *giaoViecGia, context.Context) {
	t.Helper()
	tasks, ctx := dungGhiNhiemVu(t, k)
	staff := &giaoViecGia{duocTatCa: true}
	tasks.giaoViec = staff
	return NewCitizenLetterTaskCreation(letters, tasks), staff, ctx
}

// letterTaskRequest is the dialog: a title typed (pre-filled by the SCREEN from the summary, confirmed by
// a person), and a source pair + deadline a forged client might try — all three must be ignored.
func letterTaskRequest() YeuCauTaoNhiemVu {
	yc := taoMau()
	yc.NguonGiao = string(domain.NguonTrucTiep)
	yc.NguonID = "01JDONCUAXAKHAC000000000"
	yc.HanXuLy = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	return yc
}

func auditText(t *testing.T, k *khoNhiemVuGia) string {
	t.Helper()
	var b strings.Builder
	for _, a := range vetKiemToan(t, k).args {
		switch v := a.(type) {
		case []byte:
			b.Write(v)
		case string:
			b.WriteString(v)
		}
		b.WriteString(" ")
	}
	return b.String()
}

func TestCitizenLetterTask_BooksFromDocumentsAnswerInOneTransaction(t *testing.T) {
	k := khoNVMau()
	due := time.Date(2026, 10, 20, 16, 30, 0, 0, time.UTC) // 23:30 ICT on the 20th
	letters := eligibleLetter(due)
	uc, _, ctx := buildLetterTask(t, k, letters)

	n, err := uc.CreateTask(ctx, letterIDThu, letterTaskRequest(), canBoThu())
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if letters.calls != 1 || letters.asked != letterIDThu || letters.commune == "" {
		t.Errorf("documents asked %d times for %q in commune %q", letters.calls, letters.asked, letters.commune)
	}
	if n.NguonGiao != domain.SourceCitizenLetter || n.NguonID != letterIDThu {
		t.Errorf("source = (%q, %q), want (don-thu, the id documents echoed)", n.NguonGiao, n.NguonID)
	}
	ins := k.cau("INSERT INTO nhiem_vu ")
	if len(ins) != 1 {
		t.Fatalf("%d task INSERTs, want 1", len(ins))
	}
	args := ins[0].args
	if args[9] != "don-thu" || args[10] != letterIDThu {
		t.Errorf("INSERT nguon_giao/nguon_id = %v/%v", args[9], args[10])
	}
	wantDue := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC) // 17:00 ICT on the 20th
	for i, col := range []string{"han_xu_ly", "han_ban_dau"} {
		got, ok := args[14+i].(time.Time)
		if !ok || !got.Equal(wantDue) {
			t.Errorf("%s = %v, want %v (the Vietnam date of the letter's deadline at 17:00)", col, args[14+i], wantDue)
		}
	}
	// The dialog named nobody: the letter's holder, both halves.
	if args[11] != letterUnitThu || args[12] != letterAssignee {
		t.Errorf("holder = %v/%v, want the letter's %q/%q", args[11], args[12], letterUnitThu, letterAssignee)
	}
	// ONE transaction: task, timeline row and audit entry together.
	if k.batDau != 1 || k.daCommit != 1 {
		t.Errorf("transactions opened %d, committed %d — want 1/1", k.batDau, k.daCommit)
	}
	for _, s := range []string{"INSERT INTO nhiem_vu ", "INSERT INTO nhat_ky_nhiem_vu", "INSERT INTO audit_log"} {
		for _, l := range k.cau(s) {
			if !l.trongGiaoDich {
				t.Errorf("%q ran outside the transaction", s)
			}
		}
	}
	trail := auditText(t, k)
	if !strings.Contains(trail, `"don_thu":{"nam":2026,"so_vao_so":12}`) {
		t.Errorf("audit delta does not name the letter by number/year: %s", trail)
	}
	if !strings.Contains(trail, maCanBoThu) {
		t.Errorf("audit actor is not the staff business code %q", maCanBoThu)
	}
	if strings.Contains(trail, letterSummaryThu) {
		t.Error("the letter's summary reached audit_log")
	}
}

func TestCitizenLetterTask_NoLetterDeadlineGivesNoTaskDeadline(t *testing.T) {
	k := khoNVMau()
	uc, _, ctx := buildLetterTask(t, k, eligibleLetter(time.Time{}))
	if _, err := uc.CreateTask(ctx, letterIDThu, letterTaskRequest(), canBoThu()); err != nil {
		t.Fatal(err)
	}
	args := k.cau("INSERT INTO nhiem_vu ")[0].args
	if args[14] != nil || args[15] != nil {
		t.Errorf("deadline = %v/%v, want NULL — the client's 2030 date must not survive, nor any default", args[14], args[15])
	}
}

func TestCitizenLetterTask_BodyHolderWinsAndIsChecked(t *testing.T) {
	k := khoNVMau()
	uc, staff, ctx := buildLetterTask(t, k, eligibleLetter(time.Time{}))
	yc := letterTaskRequest()
	yc.NguoiThucHienMa = "CB-00077"
	if _, err := uc.CreateTask(ctx, letterIDThu, yc, canBoThu()); err != nil {
		t.Fatal(err)
	}
	args := k.cau("INSERT INTO nhiem_vu ")[0].args
	if args[11] != nil || args[12] != "CB-00077" {
		t.Errorf("holder = %v/%v — a dialog naming a person must not borrow the letter's unit", args[11], args[12])
	}
	if len(staff.daHoi) != 1 || staff.daHoi[0][0] != "CB-00077" {
		t.Errorf("assignee checks = %v, want one for CB-00077", staff.daHoi)
	}
}

func TestCitizenLetterTask_InheritedAssigneeIsCheckedToo(t *testing.T) {
	k := khoNVMau()
	uc, staff, ctx := buildLetterTask(t, k, eligibleLetter(time.Time{}))
	staff.duocTatCa = false // the letter's officer is no longer assignable (locked, left)
	_, err := uc.CreateTask(ctx, letterIDThu, letterTaskRequest(), canBoThu())
	if !errors.Is(err, ErrAssignmentStaffInvalid) {
		t.Fatalf("err = %v, want ErrAssignmentStaffInvalid", err)
	}
	khongMoGiaoDich(t, k)
}

// Every refusal: nothing written, no transaction even opened.
func TestCitizenLetterTask_RefusalsWriteNothing(t *testing.T) {
	down := errors.New("documents: UNAVAILABLE")
	for name, c := range map[string]struct {
		letters CitizenLetterResolver
		want    error
	}{
		"not found": {&letterResolverFake{answer: documentsclient.CitizenLetterForTask{
			Eligibility: documentsclient.CitizenLetterNotFound}}, ErrCitizenLetterNotFound},
		"denunciation": {&letterResolverFake{answer: documentsclient.CitizenLetterForTask{
			Eligibility: documentsclient.CitizenLetterDenunciation}}, ErrCitizenLetterDenunciation},
		"documents down":    {&letterResolverFake{err: down}, ErrCitizenLetterUnchecked},
		"unknown answer":    {&letterResolverFake{}, ErrCitizenLetterUnchecked},
		"documents unwired": {nil, ErrCitizenLetterUnchecked},
		"nobody to hold it": {&letterResolverFake{answer: documentsclient.CitizenLetterForTask{
			Eligibility: documentsclient.CitizenLetterEligible, LetterID: letterIDThu}}, domain.ErrCitizenLetterAssignmentRequired},
	} {
		t.Run(name, func(t *testing.T) {
			k := khoNVMau()
			uc, _, ctx := buildLetterTask(t, k, c.letters)
			_, err := uc.CreateTask(ctx, letterIDThu, letterTaskRequest(), canBoThu())
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			khongMoGiaoDich(t, k)
		})
	}
}

func TestCitizenLetterTask_BadIDAndNoActorNeverAskDocuments(t *testing.T) {
	k := khoNVMau()
	letters := eligibleLetter(time.Time{})
	uc, _, ctx := buildLetterTask(t, k, letters)
	if _, err := uc.CreateTask(ctx, "  ", letterTaskRequest(), canBoThu()); !errors.Is(err, domain.ErrCitizenLetterIDInvalid) {
		t.Errorf("blank id: err = %v", err)
	}
	if _, err := uc.CreateTask(ctx, letterIDThu, letterTaskRequest(), audit.Actor{Kind: "staff"}); err == nil {
		t.Error("no actor code: created")
	}
	if letters.calls != 0 {
		t.Errorf("documents asked %d times", letters.calls)
	}
	khongMoGiaoDich(t, k)
}

// --- the wall on the create path itself ------------------------------------------------------------------

func TestCreateFromSource_CitizenLetterSourceNeedsTheLetterFacts(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	yc := taoMau()
	yc.NguonGiao = string(domain.SourceCitizenLetter)
	yc.NguonID = letterIDThu

	if _, err := uc.Tao(ctx, yc, canBoThu()); !errors.Is(err, domain.ErrCitizenLetterSourceNotDirect) {
		t.Errorf("Tao: err = %v, want ErrCitizenLetterSourceNotDirect", err)
	}
	if _, err := uc.CreateFromSource(ctx, yc, canBoThu(), SourceSteps{
		AuditExtra: map[string]any{"khac": 1}}); !errors.Is(err, domain.ErrCitizenLetterSourceNotDirect) {
		t.Errorf("CreateFromSource without the letter key: err = %v", err)
	}
	khongMoGiaoDich(t, k)
}

// AuditExtra never overwrites a key create writes (createInTx's rule), so a door cannot rewrite the trail.
func TestCreateFromSource_AuditExtraCannotOverwriteCreateKeys(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	if _, err := uc.CreateFromSource(ctx, taoMau(), canBoThu(), SourceSteps{
		AuditExtra: map[string]any{"ma_tu_sinh": "GIA-MAO"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditText(t, k), "GIA-MAO") {
		t.Error("AuditExtra overwrote a key create writes")
	}
}

// --- the schema ----------------------------------------------------------------------------------------

// TestTaskSourcesAreAllowedByTheSchema: every NguonGiao the code can write is in the LATEST
// `nhiem_vu_nguon_giao_hop_le` (migration 0034 for `don-thu`). Without it the fake driver stays green while
// PostgreSQL rolls every letter task back.
func TestTaskSourcesAreAllowedByTheSchema(t *testing.T) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	decl := regexp.MustCompile(`(?is)nhiem_vu_nguon_giao_hop_le\s+CHECK\s*\(\s*nguon_giao\s+IN\s*\(([^)]*)\)`)
	var list string
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatal(err)
		}
		if ms := decl.FindAllStringSubmatch(string(b), -1); len(ms) > 0 {
			list = ms[len(ms)-1][1]
		}
	}
	if list == "" {
		t.Fatal("CHECK nhiem_vu_nguon_giao_hop_le not found — the reader went blind")
	}
	for _, s := range []domain.NguonGiao{domain.NguonTrucTiep, domain.NguonKetLuanHop, domain.NguonVanBanDen,
		domain.NguonPhanAnh, domain.SourceCitizenLetter} {
		if !strings.Contains(list, "'"+string(s)+"'") {
			t.Errorf("source %q is not in the latest CHECK — every write would roll back on PostgreSQL", s)
		}
	}
}
