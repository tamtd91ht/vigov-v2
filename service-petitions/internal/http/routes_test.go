package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Harness and fakes for the petitions HTTP routes.
//
// NOTHING HERE IS A PostgreSQL. The properties under test are the permission declaration on each
// route, the commune boundary, and the shape and ORDER of each response — and none of those live in
// the database. There is also no PostgreSQL reachable from this build environment at all
// (VIGOV_TEST_DSN unset), so a test that needed one would be a test that never runs.
//
// WHAT THIS HARNESS DOES NOT COVER, SAID PLAINLY: this service has no session middleware yet. In
// service-identity the Principal is built by XacThuc from a signed cookie; here the harness puts it
// into the request context directly, the way that middleware will when it exists. So these tests
// assert what the ROUTE does with a principal — including refusing when there is none, and refusing
// one issued by another commune — and assert nothing about how a principal comes to be.

// --- fixtures -----------------------------------------------------------------------------------

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	// The internal staff id a principal carries. No personal data in a fixture (rule 3).
	idCanBo = "nd-01JCANBONOIBOCUAXA"

	// The BUSINESS CODE of the same person. A DIFFERENT STRING FROM idCanBo ON PURPOSE: the trail
	// records this one and authorisation joins on the other (rule 6, invariant 2; authz.Principal
	// argues why neither can do the other's job). One value for both would leave a test unable to
	// tell a correct write from the 2026-09-22 one.
	maCanBo = "CB-00123"
)

var (
	xaA = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

// canBoCuaXa is a signed-in member of staff OF ONE COMMUNE. The commune on the principal is what
// authz compares against the commune resolved from Host, so it is the field every isolation case
// below turns on.
func canBoCuaXa(xa tenant.ID) *authz.Principal {
	return &authz.Principal{ID: idCanBo, Ma: maCanBo, Kind: "staff", TenantID: xa}
}

// --- fakes --------------------------------------------------------------------------------------

type thuMucGia map[string]tenant.Tenant

func (m thuMucGia) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// thuMucMau is the platform registry for the two test communes. A NEW MAP PER CALL, so a test that
// rewrites one copy cannot change another's.
func thuMucMau() thuMucGia {
	return thuMucGia{
		hostA: {ID: xaA, Host: hostA, Name: "Xã Thăng Bình", Province: "Thành phố Đà Nẵng", Active: true},
		hostB: {ID: xaB, Host: hostB, Name: "Xã Bình Dương", Active: true},
	}
}

// loaiNhiemVuGia is the task-type catalogue, KEYED BY COMMUNE, reading the commune from the context
// exactly as *store.Scoped does. Keyed any other way, the isolation cases below would pass while
// proving nothing.
//
// `goi` counts the reads. The count is what proves the commune check happens BEFORE any store
// access — a route that refused only after reading would still have read another commune's rows.
type loaiNhiemVuGia struct {
	theo map[tenant.ID][]domain.LoaiNhiemVu
	loi  error
	goi  int
}

func (l *loaiNhiemVuGia) DanhSach(ctx context.Context) ([]domain.LoaiNhiemVu, error) {
	l.goi++
	if l.loi != nil {
		return nil, l.loi
	}
	return l.theo[tenant.MustFrom(ctx)], nil
}

// loaiNhiemVuMau gives commune A three rows and commune B one row with a DIFFERENT code and a
// DIFFERENT label. Two communes whose catalogues were named the same could not show a leak.
//
// THE ORDER IS THE STORE'S (ORDER BY thu_tu, ma) AND IS DELIBERATELY NOT ALPHABETICAL: `co-ban`
// sorts before `theo-van-ban` by code and "Theo văn bản" before "Việc cũ" by label, so any
// re-sorting in the handler — by code, by label, by id — turns the order assertion red.
//
// The third row is OUT OF USE. It is here because such rows must still be returned: a task recorded
// last year may hold that code, and a list that dropped it would leave the task showing a raw code
// with no label.
func loaiNhiemVuMau() *loaiNhiemVuGia {
	return &loaiNhiemVuGia{theo: map[tenant.ID][]domain.LoaiNhiemVu{
		xaA: {
			{ID: "lnv-001", Ma: "theo-van-ban", Nhan: "Theo văn bản", LaMacDinh: true, DangDung: true},
			{ID: "lnv-002", Ma: "co-ban", Nhan: "Cơ bản", DangDung: true},
			{ID: "lnv-003", Ma: "viec-cu", Nhan: "Việc cũ", DangDung: false},
		},
		xaB: {
			{ID: "lnv-b-001", Ma: "kiem-tra", Nhan: "Kiểm tra nội bộ xã B", DangDung: true},
		},
	}}
}

// mucUuTienGia is the priority scale, KEYED BY COMMUNE, reading the commune from the context the
// same way *store.Scoped does — see loaiNhiemVuGia.
type mucUuTienGia struct {
	theo map[tenant.ID][]domain.MucUuTienNhiemVu
	loi  error
	goi  int
}

func (m *mucUuTienGia) DanhSach(ctx context.Context) ([]domain.MucUuTienNhiemVu, error) {
	m.goi++
	if m.loi != nil {
		return nil, m.loi
	}
	return m.theo[tenant.MustFrom(ctx)], nil
}

// mucUuTienMau gives commune A the three shipped levels IN RANK ORDER, which is deliberately
// neither alphabetical by code (cao, khan, thuong) nor by label (Cao, Khẩn, Thường). That is the
// whole point of the fixture: a handler that sorted by anything at all would produce a plausible
// list in the wrong order, and a priority list in the wrong order is wrong in a way nobody reports
// as a bug.
func mucUuTienMau() *mucUuTienGia {
	return &mucUuTienGia{theo: map[tenant.ID][]domain.MucUuTienNhiemVu{
		xaA: {
			{ID: "uu-001", Ma: "khan", Nhan: "Khẩn", DangDung: true},
			{ID: "uu-002", Ma: "cao", Nhan: "Cao", DangDung: true},
			{ID: "uu-003", Ma: "thuong", Nhan: "Thường", LaMacDinh: true, DangDung: true},
		},
		xaB: {
			{ID: "uu-b-001", Ma: "binh-thuong", Nhan: "Bình thường xã B", LaMacDinh: true, DangDung: true},
		},
	}}
}

// vetXemGia is the full-view audit trail, RECORDING WHAT IT WAS ASKED TO WRITE.
//
// IT KEEPS THE COMMUNE FROM THE CONTEXT ON EVERY ENTRY, which the real use case also does (through
// store.Scoped) and which is the one property a fake keyed any other way could not show: an entry
// attributed to the wrong commune is useless to an inspection, and rule 6, invariant 2 lists the
// commune among the six things an entry must carry.
//
// `loi` makes the trail fail, which is the case the route has to treat as "do not disclose".
type vetXemGia struct {
	ghi []vetDaGhi
	loi error
}

type vetDaGhi struct {
	xa    tenant.ID
	ma    string
	nguoi audit.Actor
}

func (v *vetXemGia) GhiVet(ctx context.Context, ma string, nguoi audit.Actor) error {
	if v.loi != nil {
		// NOTHING IS RECORDED ON FAILURE, deliberately: the real use case commits or it does not,
		// and a fake that kept a half-entry would let a test pass that the database would fail.
		return v.loi
	}
	v.ghi = append(v.ghi, vetDaGhi{xa: tenant.MustFrom(ctx), ma: ma, nguoi: nguoi})
	return nil
}

// checkerGia grants permissions per commune and per staff id, reading the commune from the context
// exactly as the real query does. Both routes below are AnyAuthenticated, so what this fake is for
// is the opposite of the usual case: proving that an account holding NOTHING still gets 200.
type checkerGia struct {
	quyen map[tenant.ID]map[string]map[authz.Perm]bool
}

func (c checkerGia) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	if p.Kind != "staff" || p.ID == "" {
		return false
	}
	return c.quyen[tenant.MustFrom(ctx)][p.ID][perm]
}

// --- harness ------------------------------------------------------------------------------------

type mayChu struct {
	h          http.Handler
	d          Deps // kept so a test can rebuild the chain with ONE dependency swapped — see dungLai
	thuMuc     thuMucGia
	loai       *loaiNhiemVuGia
	uuTien     *mucUuTienGia
	phieu      *phieuGia
	nhan       *nhanLinhVucGia
	vet        *vetXemGia
	danhSach   *danhSachPhieuGia
	xuLy       *xuLyPhieuGia
	nhatKy     *nhatKyPhieuGia
	nhiemVu    *nhiemVuGia
	ghiNhiemVu *ghiNhiemVuGia
	deNghiCho  *deNghiChoDuyetGia
	bienBan    *bienBanGia
	ghiBienBan *ghiBienBanGia

	// petitionTasks is the task-from-petition act — petition_task_test.go.
	petitionTasks *petitionTaskFake
	// citizenLetterTasks is "Chuyển đơn thư thành nhiệm vụ" — citizen_letter_task_test.go.
	citizenLetterTasks *citizenLetterTaskFake

	// The leadership overview — summary_test.go.
	taskSummary   *taskSummaryFake
	reportSummary *citizenReportSummaryFake
	overdue       *overdueQueueFake
	// breakdown is the /phan-anh statistics read — citizen_report_figures_test.go.
	breakdown *citizenReportBreakdownFake

	// filterIdentity answers `soon=true` and `scope=related` — task_filter_identity_test.go.
	filterIdentity *taskFilterIdentityFake

	// registerExport is the Sổ theo dõi export — task_register_export_test.go.
	registerExport *registerExportFake

	// taskImport is the spreadsheet import — task_import_test.go.
	taskImport *taskImportFake

	// The task attachments — task_attachment_test.go.
	taskAttachments *taskAttachmentsFake
	logAttachments  *logAttachmentsFake

	// staffPhotos is the petition's scene photos, staff read — petition_photo_test.go.
	staffPhotos *staffPhotosFake

	// The petition's staff files — petition_staff_file_test.go.
	verificationPhotos *verificationPhotosFake
	petitionLogFiles   *petitionLogAttachmentsFake

	// fields is the petition field catalogue — petition_fields_test.go.
	fields *fieldCatalogueFake

	// staffIntake is "Nhập hộ phản ánh" — staff_intake_test.go.
	staffIntake *staffIntakeFake

	// unitNames is identity's residential-unit name lookup — residential_unit_test.go.
	unitNames *unitNamesFake

	// Merging duplicate petitions — citizen_report_merge_test.go.
	merge *mergeActsFake
	links *mergeLinksFake
	dups  *duplicateCandidatesFake
}

func dungMayChu(t *testing.T) *mayChu {
	t.Helper()

	loai := loaiNhiemVuMau()
	uuTien := mucUuTienMau()
	phieu := phieuMau()
	nhan := nhanLinhVucMau()
	vet := &vetXemGia{}
	// The register list is fed from the SAME fixtures as the single-petition reader, so commune A's
	// page really does contain a `can-bo` petition for the restricted-field cases to exclude, and
	// commune B's page contains only commune B's.
	danhSach := danhSachTuPhieuMau(phieu)
	xuLy := &xuLyPhieuGia{}
	// The staff intake — staff_intake_test.go.
	staffIntake := &staffIntakeFake{}
	// The processing logbook read, keyed by commune AND petition id — see nhatKyPhieuGia.
	nhatKy := nhatKyMau()
	// ONE fake for BOTH task read routes, keyed by commune — the same object the wiring in
	// cmd/server gives to both Deps fields, so a test cannot accidentally prove that two different
	// registers agree with each other.
	nhiemVu := nhiemVuMau()
	// The SIX write acts. A SEPARATE fake from the read one, exactly as the Deps field is separate:
	// a read is a store call and each of these opens a transaction, so one object answering both
	// would let a test prove that a write route "worked" by reading.
	ghiNhiemVu := &ghiNhiemVuGia{}
	// A task FROM a petition — petition_task_test.go.
	petitionTasks := &petitionTaskFake{}
	// A task FROM a citizen letter — citizen_letter_task_test.go.
	citizenLetterTasks := &citizenLetterTaskFake{}
	// The approval queue of extension requests, keyed by commune — see deNghiChoDuyetGia.
	deNghiCho := deNghiChoDuyetMau()
	// The meeting register. Its fixtures are SEPARATE from the task register's on purpose: the
	// counters on a card arrive already aggregated from the store, so a harness that derived them
	// from nhiemVuMau() would be asserting that two fakes agree with each other rather than that the
	// route renders what the store returned.
	bienBan := bienBanMau()
	// The THREE meeting-register write acts. A SEPARATE fake from the read one, exactly as the Deps
	// field is separate: a read is a store call and each of these opens a transaction, so one object
	// answering both would let a test prove a write route "worked" by reading.
	ghiBienBan := &ghiBienBanGia{}
	// The overview's three reads, keyed by commune — see summary_test.go.
	taskSummary := taskSummarySample()
	reportSummary := citizenReportSummarySample()
	overdue := overdueQueueSample()
	breakdown := citizenReportBreakdownSample()
	filterIdentity := taskFilterIdentitySample()
	registerExport := &registerExportFake{}
	taskImport := &taskImportFake{}
	taskAttachments := &taskAttachmentsFake{}
	logAttachments := &logAttachmentsFake{}
	// The petition's scene photos, staff read — petition_photo_test.go.
	staffPhotos := newStaffPhotosFake()
	// The petition's staff files — petition_staff_file_test.go.
	verificationPhotos := &verificationPhotosFake{}
	petitionLogFiles := &petitionLogAttachmentsFake{}
	// The field catalogue — petition_fields_test.go.
	fields := newFieldCatalogueFake()
	// The residential-unit names — residential_unit_test.go.
	unitNames := newUnitNamesFake()
	// Merging duplicate petitions — citizen_report_merge_test.go.
	merge, links, dups := &mergeActsFake{}, &mergeLinksFake{}, &duplicateCandidatesFake{}

	m := &mayChu{
		d: Deps{
			// A checker that grants commune A's account `feedback.read` — what the petition route
			// requires — plus one unrelated key, and grants commune B's account NOTHING. The
			// restricted key `feedback.restricted` AND the full-view key `feedback.unmask` are
			// deliberately absent from both, so the restricted-field case and the masking case
			// each have something real to fail on. A default harness that held every key would
			// turn the ordinary masked read — the common case in a commune — into the untested one.
			Checker: checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
				xaA: {idCanBo: {
					authz.Perm("task.read"):     true,
					authz.Perm("feedback.read"): true,
				}},
				xaB: {},
			}},
			LoaiNhiemVu: loai,
			MucUuTien:   uuTien,
			// The two write use cases, so Register accepts the Deps. NOTHING IN THIS FILE CALLS
			// THEM: the six write routes have their own four-case suites in
			// loai_nhiem_vu_ghi_test.go and muc_uu_tien_nhiem_vu_ghi_test.go, each with a fake that
			// records the commune and the acting person. Register refuses a nil dependency at
			// construction, so both have to be present — and a fake nothing invokes cannot answer
			// anything wrongly.
			GhiLoaiNhiemVu: &ghiDanhMucGia{},
			GhiMucUuTien:   &ghiDanhMucGiaUuTien{},
			// The two catalogue Excel imports: own suite in catalogue_import_test.go.
			TaskTypeImports:     taskTypeImportsFake(),
			TaskPriorityImports: taskPriorityImportsFake(),
			// Task-status wording: own suite in trang_thai_nhiem_vu_test.go; present because
			// Register refuses a nil dependency.
			TrangThaiNhiemVu:    docTrangThaiMau(),
			GhiTrangThaiNhiemVu: &ghiTrangThaiGia{},
			Phieu:               phieu,
			NhanLinhVuc:         nhan,
			PetitionFields:      fields,
			Vet:                 vet,
			DanhSachPhieu:       danhSach,
			XuLyPhieu:           xuLy,
			StaffIntake:         staffIntake,
			NhatKyPhieu:         nhatKy,
			NhiemVu:             nhiemVu,
			DanhSachNhiemVu:     nhiemVu,
			TaskFilterIdentity:  filterIdentity,
			TaskRegisterExport:  registerExport,
			TaskImport:          taskImport,
			DeNghiChoDuyet:      deNghiCho,
			GhiNhiemVu:          ghiNhiemVu,
			PetitionTasks:       petitionTasks,
			CitizenLetterTasks:  citizenLetterTasks,
			TaskAttachments:     taskAttachments,
			TaskLogAttachments:  logAttachments,
			PetitionPhotos:      staffPhotos,
			// The petition's staff files — petition_staff_file_test.go.
			VerificationPhotos:           verificationPhotos,
			PetitionLogAttachments:       petitionLogFiles,
			PetitionLogAttachmentsReader: petitionLogFiles,
			DanhSachBienBan:              bienBan,
			GhiBienBan:                   ghiBienBan,
			TaskSummary:                  taskSummary,
			CitizenReportSummary:         reportSummary,
			OverdueQueue:                 overdue,
			// The /phan-anh statistics — citizen_report_figures_test.go.
			CitizenReportBreakdown: breakdown,
			// identity's residential-unit names — residential_unit_test.go.
			ResidentialUnitNames: unitNames,
			// Merging duplicate petitions — citizen_report_merge_test.go.
			CitizenReportMerge:  merge,
			MergeLinks:          links,
			DuplicateCandidates: dups,
			// The audit-log reader: present because Register refuses a nil one; its suite is
			// audit_entries_test.go.
			AuditLog: &auditLogFake{},
			// "Lời hệ thống": the fake answers the SHIPPED DEFAULT unless a test sets an override, so
			// every refusal case in this package reads the sentence a commune that never touched the
			// screen reads. Its suite is system_messages_test.go.
			SystemMessages: &systemMessagesFake{},
			Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
		thuMuc:     thuMucMau(),
		loai:       loai,
		uuTien:     uuTien,
		phieu:      phieu,
		nhan:       nhan,
		vet:        vet,
		danhSach:   danhSach,
		xuLy:       xuLy,
		nhatKy:     nhatKy,
		nhiemVu:    nhiemVu,
		ghiNhiemVu: ghiNhiemVu,
		deNghiCho:  deNghiCho,
		bienBan:    bienBan,
		ghiBienBan: ghiBienBan,

		petitionTasks:      petitionTasks,
		citizenLetterTasks: citizenLetterTasks,

		taskSummary:   taskSummary,
		reportSummary: reportSummary,
		overdue:       overdue,
		breakdown:     breakdown,

		filterIdentity: filterIdentity,
		registerExport: registerExport,
		taskImport:     taskImport,

		taskAttachments: taskAttachments,
		logAttachments:  logAttachments,
		staffPhotos:     staffPhotos,

		verificationPhotos: verificationPhotos,
		petitionLogFiles:   petitionLogFiles,

		fields: fields,

		staffIntake: staffIntake,
		unitNames:   unitNames,

		merge: merge,
		links: links,
		dups:  dups,
	}
	m.dungLai(t, nil)
	return m
}

// dungLai rebuilds the edge chain, optionally with one dependency swapped first.
//
// The chain is the real one MINUS the session middleware this service does not have yet: commune
// resolution from Host, a panic guard, and the header strip. The principal is injected per request
// — see goi.
func (m *mayChu) dungLai(t *testing.T, sua func(d *Deps)) {
	t.Helper()
	if sua != nil {
		sua(&m.d)
	}

	mux := http.NewServeMux()
	Register(mux, m.d)

	var h http.Handler = mux
	h = httpx.TenantMiddleware(m.thuMuc)(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	m.h = h
}

// goi issues a request. A nil principal means "not signed in" — the 401 case.
//
// THE PRINCIPAL GOES INTO THE CONTEXT BEFORE THE EDGE RUNS, not after: TenantMiddleware adds the
// commune to whatever context the request already carries, so both values reach the guard exactly
// as they will in production. What it does NOT do is make the principal agree with the Host — which
// is the point, because that disagreement is the cross-commune case.
func (m *mayChu) goi(t *testing.T, method, host, path string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if p != nil {
		r = r.WithContext(authz.Into(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func doiMa(t *testing.T, w *httptest.ResponseRecorder, muon int) {
	t.Helper()
	if w.Code != muon {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, muon, w.Body.String())
	}
}

func loiTra(t *testing.T, w *httptest.ResponseRecorder) httpx.Error {
	t.Helper()
	var e httpx.Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("thân lỗi không phải JSON: %q", w.Body.String())
	}
	return e
}

// --- wiring -------------------------------------------------------------------------------------

// depsDay is a COMPLETE Deps. Every case below starts from it and removes exactly ONE thing.
//
// THE COMPLETENESS IS THE WHOLE POINT, and this test was rewritten for it: the earlier version
// built each case from a partial literal naming only one dependency, so once Deps grew from two
// fields to five, every case panicked over a field the case was not about. It stayed green while
// testing nothing — the failure shape that is indistinguishable from success.
func depsDay() Deps {
	return Deps{
		Checker:        checkerGia{},
		LoaiNhiemVu:    loaiNhiemVuMau(),
		MucUuTien:      mucUuTienMau(),
		GhiLoaiNhiemVu: &ghiDanhMucGia{},
		GhiMucUuTien:   &ghiDanhMucGiaUuTien{},
		// The two catalogue Excel imports.
		TaskTypeImports:     taskTypeImportsFake(),
		TaskPriorityImports: taskPriorityImportsFake(),
		// The task-status wording, read and write.
		TrangThaiNhiemVu:    docTrangThaiMau(),
		GhiTrangThaiNhiemVu: &ghiTrangThaiGia{},
		Phieu:               phieuMau(),
		NhanLinhVuc:         nhanLinhVucMau(),
		PetitionFields:      newFieldCatalogueFake(),
		Vet:                 &vetXemGia{},
		DanhSachPhieu:       danhSachTuPhieuMau(phieuMau()),
		XuLyPhieu:           &xuLyPhieuGia{},
		StaffIntake:         &staffIntakeFake{},
		NhatKyPhieu:         nhatKyMau(),
		// BOTH TASK FIELDS, from ONE fake — the same shape cmd/server wires.
		NhiemVu:            nhiemVuMau(),
		DanhSachNhiemVu:    nhiemVuMau(),
		TaskFilterIdentity: taskFilterIdentitySample(),
		TaskRegisterExport: &registerExportFake{},
		TaskImport:         &taskImportFake{},
		DeNghiChoDuyet:     deNghiChoDuyetMau(),
		GhiNhiemVu:         &ghiNhiemVuGia{},
		PetitionTasks:      &petitionTaskFake{},
		CitizenLetterTasks: &citizenLetterTaskFake{},
		TaskAttachments:    &taskAttachmentsFake{},
		TaskLogAttachments: &logAttachmentsFake{},
		PetitionPhotos:     &staffPhotosFake{},
		// The petition's staff files.
		VerificationPhotos:           &verificationPhotosFake{},
		PetitionLogAttachments:       &petitionLogAttachmentsFake{},
		PetitionLogAttachmentsReader: &petitionLogAttachmentsFake{},
		DanhSachBienBan:              bienBanMau(),
		GhiBienBan:                   &ghiBienBanGia{},
		// The leadership overview.
		TaskSummary:          taskSummarySample(),
		CitizenReportSummary: citizenReportSummarySample(),
		OverdueQueue:         overdueQueueSample(),
		// The /phan-anh statistics.
		CitizenReportBreakdown: citizenReportBreakdownSample(),
		AuditLog:               &auditLogFake{},
		SystemMessages:         &systemMessagesFake{},
		// Merging duplicate petitions.
		CitizenReportMerge:  &mergeActsFake{},
		MergeLinks:          &mergeLinksFake{},
		DuplicateCandidates: &duplicateCandidatesFake{},
	}
}

func TestRegisterThieuPhuThuocThiPanicNgayLucDung(t *testing.T) {
	// A route mounted without its store would accept requests it cannot answer, and the first
	// person to find out would be a member of staff in front of a government screen. Failing at
	// construction is loud, happens before any commune is served, and names the missing piece.
	//
	// THE Checker CASE IS THE ONE THAT CHANGED MEANING. It used to be correct for it to be
	// absent, because no route declared a permission; GET /api/v1/citizen-reports/{maTraCuu} now
	// does, and authz.RequirePermission(nil, …) panics when a member of staff CALLS it rather
	// than at startup.
	for ten, bo := range map[string]func(d *Deps){
		"thiếu kho loại nhiệm vụ": func(d *Deps) { d.LoaiNhiemVu = nil },
		"thiếu kho mức ưu tiên":   func(d *Deps) { d.MucUuTien = nil },
		// The task-status wording: a nil read is every Kanban header gone; a nil write is the
		// configuration screen's only save button panicking on a staff member's screen.
		"thiếu kho nhãn trạng thái nhiệm vụ": func(d *Deps) { d.TrangThaiNhiemVu = nil },
		"thiếu use case ghi nhãn trạng thái": func(d *Deps) { d.GhiTrangThaiNhiemVu = nil },
		"thiếu Checker":                    func(d *Deps) { d.Checker = nil },
		"thiếu kho phiếu":                  func(d *Deps) { d.Phieu = nil },
		"thiếu kho nhãn lĩnh vực":          func(d *Deps) { d.NhanLinhVuc = nil },
		"missing petition field catalogue": func(d *Deps) { d.PetitionFields = nil },
		"thiếu đường đọc danh sách phiếu":  func(d *Deps) { d.DanhSachPhieu = nil },
		// THE ONE THAT TAKES THE WHOLE PROCESSING PATH WITH IT. A nil here does not break one screen:
		// it breaks classify, assign, advance and close at once, which puts the service back in the
		// state it was in before these routes existed — petitions arriving and nothing able to move
		// them.
		"thiếu use case xử lý phiếu":      func(d *Deps) { d.XuLyPhieu = nil },
		"thiếu use case nhập hộ phản ánh": func(d *Deps) { d.StaffIntake = nil },
		// The timeline column of the petition drawer (migration 0013).
		"thiếu đường đọc nhật ký xử lý phiếu": func(d *Deps) { d.NhatKyPhieu = nil },
		// THE CASE WITH THE QUIETEST FAILURE MODE. A nil here does not break a screen: it breaks
		// only the branch that discloses a citizen's name and number, and only for an account
		// holding `feedback.unmask`. Without this case, a wiring line dropped in a refactor ships.
		"thiếu đường ghi vết xem đầy đủ": func(d *Deps) { d.Vet = nil },
		// THE TWO TASK READ ROUTES. Four other subsystems stand on this register, so a nil here is
		// not one screen: it is Nhiệm vụ, Sổ tay lãnh đạo, Biên bản họp and half of Tổng quan.
		"thiếu kho nhiệm vụ":                 func(d *Deps) { d.NhiemVu = nil },
		"thiếu đường đọc danh sách nhiệm vụ": func(d *Deps) { d.DanhSachNhiemVu = nil },
		"thiếu đường đọc hàng chờ lùi hạn":   func(d *Deps) { d.DeNghiChoDuyet = nil },
		// `soon=true` and `scope=related`: a nil here panics on the first officer who ticks the box.
		"thiếu đường hỏi identity cho bộ lọc nhiệm vụ": func(d *Deps) { d.TaskFilterIdentity = nil },
		"thiếu use case xuất sổ theo dõi":              func(d *Deps) { d.TaskRegisterExport = nil },
		"thiếu use case nhập nhiệm vụ":                 func(d *Deps) { d.TaskImport = nil },
		"thiếu use case nhập Excel loại nhiệm vụ":      func(d *Deps) { d.TaskTypeImports = nil },
		"thiếu use case nhập Excel mức ưu tiên":        func(d *Deps) { d.TaskPriorityImports = nil },
		// The meeting register. A nil here is the Biên bản họp screen, and with it the only place a
		// commune can see WHERE its tasks came from.
		"thiếu đường đọc danh sách biên bản": func(d *Deps) { d.DanhSachBienBan = nil },
		// The three meeting-register writes. A nil here leaves the register readable and unfillable —
		// and with it §3, the ONE path by which a conclusion becomes a task and keeps a back-link to
		// where it came from.
		"thiếu use case ghi biên bản": func(d *Deps) { d.GhiBienBan = nil },
		// §5.9's attachments: a nil use case panics on the first `📎 Đính kèm`; a nil reader panics on
		// EVERY timeline read, attachments or not.
		"thiếu use case tạo nhiệm vụ từ phiếu":     func(d *Deps) { d.PetitionTasks = nil },
		"missing citizen-letter task use case":     func(d *Deps) { d.CitizenLetterTasks = nil },
		"thiếu use case tệp đính kèm nhiệm vụ":     func(d *Deps) { d.TaskAttachments = nil },
		"thiếu đường đọc tệp đính kèm của nhật ký": func(d *Deps) { d.TaskLogAttachments = nil },
		"missing petition photo read":              func(d *Deps) { d.PetitionPhotos = nil },
		// The leadership overview: two tiles' worth of figures and the "Cần xử lý ngay" panel.
		"thiếu đường đếm tổng quan nhiệm vụ": func(d *Deps) { d.TaskSummary = nil },
		"thiếu đường đếm tổng quan phản ánh": func(d *Deps) { d.CitizenReportSummary = nil },
		"thiếu use case hàng đợi quá hạn":    func(d *Deps) { d.OverdueQueue = nil },
		// The audit-log reader (ADR 0054): a nil here panics on the first administrator opening it.
		"thiếu bộ đọc nhật ký hệ thống": func(d *Deps) { d.AuditLog = nil },
		// "Lời hệ thống": a nil here panics inside a REFUSAL — the answer meant to explain a mistake.
		"thiếu use case lời hệ thống": func(d *Deps) { d.SystemMessages = nil },
	} {
		t.Run(ten, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("dựng được route với phụ thuộc thiếu — lỗi sẽ nổ trên màn hình người dùng")
				}
			}()
			d := depsDay()
			bo(&d)
			Register(http.NewServeMux(), d)
		})
	}
}

// TestRegisterDuPhuThuocThiKhongPanic is the other half, and without it the test above proves
// only that Register panics — not that it panics for the reason claimed. A switch that panicked
// unconditionally would pass every case above.
func TestRegisterDuPhuThuocThiKhongPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Register panic dù đủ phụ thuộc: %v", r)
		}
	}()
	Register(http.NewServeMux(), depsDay())
}
