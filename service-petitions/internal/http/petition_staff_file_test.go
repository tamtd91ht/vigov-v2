package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The staff files on a petition at the HTTP boundary (migration 0027).
//
//	PROVED HERE   every staff file route's four cases (rule 5, invariant 7): 401 no session · 403 a REAL
//	              neighbouring key · 401 right key WRONG COMMUNE (this package's convention: authz compares
//	              the commune before the key) · 2xx — the use case NOT reached in the first three · the
//	              uploads are ONE multipart request answered 201 with the stored file, the completion
//	              routes are gone · the write of verification photos needs `feedback.resolve`, not
//	              `feedback.read` · the
//	              business code reaches the use case · `can-bo` without `feedback.restricted` is the
//	              detail's 404 · a note carries its attachment ids down and its files back · the timeline
//	              renders attachments as an array · the close gate is 409 `after_photo_required` with the
//	              commune's sentence · the citizen's verification list: identity from the session, the
//	              GET's 404 byte for byte, never mounted on the staff mux.
//	NOT PROVED    the flows — internal/app/petition_staff_file_test.go.

const staffFileIDHTTP = "01JSTAFFFILEHTTP0000000001"

// verificationPhotosFake applies the commune and the restricted fact itself, so the isolation cases
// fail if the handler hands down the wrong ones.
type verificationPhotosFake struct {
	calls      int
	seenTenant tenant.ID
	actor      audit.Actor
	req        app.PhotoUploadRequest
	restricted bool
	err        error
}

func (f *verificationPhotosFake) admit(ctx context.Context, ma string, restricted bool) error {
	f.calls++
	f.seenTenant, f.restricted = tenant.MustFrom(ctx), restricted
	if f.err != nil {
		return f.err
	}
	switch {
	case tenant.MustFrom(ctx) == xaA && ma == maPhieuThuong:
	case tenant.MustFrom(ctx) == xaA && ma == maPhieuCanBo && restricted:
	default:
		return petstore.ErrPhieuKhongTonTai
	}
	return nil
}

func (f *verificationPhotosFake) MaxUploadBytes(context.Context) (int64, error) { return 10 << 20, nil }

func (f *verificationPhotosFake) Upload(ctx context.Context, ma string, req app.PhotoUploadRequest,
	body app.UploadBody, actor audit.Actor, restricted app.QuyenXemHanChe) (domain.StoredFile, error) {
	f.actor, f.req = actor, req
	if err := f.admit(ctx, ma, bool(restricted)); err != nil {
		return domain.StoredFile{}, err
	}
	if _, err := drainUpload(body); err != nil {
		return domain.StoredFile{}, err
	}
	return domain.StoredFile{ID: staffFileIDHTTP, MIMEType: storage.MIMEJPEG, SizeBytes: 400_000,
		Status: domain.StoredFileStored, CreatedAt: photoAtHTTP}, nil
}

func (f *verificationPhotosFake) ListPhotos(ctx context.Context, ma string, mayReadRestricted bool,
	reader audit.Actor) ([]app.PhotoLink, error) {
	f.actor = reader
	if err := f.admit(ctx, ma, mayReadRestricted); err != nil {
		return nil, err
	}
	return []app.PhotoLink{{File: domain.StoredFile{ID: staffFileIDHTTP, MIMEType: storage.MIMEJPEG, SizeBytes: 9},
		URL: storage.PresignedURL("https://s3.example.gov.vn/p?X-Amz-Signature=s"), ExpiresAt: photoAtHTTP}}, nil
}

// petitionLogAttachmentsFake is the three acts AND the timeline's batched read.
type petitionLogAttachmentsFake struct {
	calls      int
	seenTenant tenant.ID
	actor      audit.Actor
	noteRight  app.QuyenGhiChuCaXa
	restricted bool
	req        app.AttachmentUploadRequest
	err        error
	// removed records the (file id, reason) of the last Remove — citizen_report_figures_test.go.
	removedID, removedReason string
	removeResolve            app.QuyenXuLyCaXa

	byEntry   map[string][]domain.PetitionLogAttachment
	readIDs   []string
	readCalls int
}

func (f *petitionLogAttachmentsFake) admit(ctx context.Context, ma string, actor audit.Actor,
	restricted app.QuyenXemHanChe) error {
	f.calls++
	f.seenTenant, f.actor, f.restricted = tenant.MustFrom(ctx), actor, bool(restricted)
	if f.err != nil {
		return f.err
	}
	switch {
	case tenant.MustFrom(ctx) == xaA && ma == maPhieuThuong:
	case tenant.MustFrom(ctx) == xaA && ma == maPhieuCanBo && bool(restricted):
	default:
		return petstore.ErrPhieuKhongTonTai
	}
	return nil
}

func (f *petitionLogAttachmentsFake) MaxUploadBytes(context.Context) (int64, error) {
	return 50 << 20, nil
}

func (f *petitionLogAttachmentsFake) Upload(ctx context.Context, ma string, req app.AttachmentUploadRequest,
	body app.UploadBody, actor audit.Actor, noteRight app.QuyenGhiChuCaXa, restricted app.QuyenXemHanChe) (
	domain.StoredFile, error) {
	f.noteRight, f.req = noteRight, req
	if err := f.admit(ctx, ma, actor, restricted); err != nil {
		return domain.StoredFile{}, err
	}
	if _, err := drainUpload(body); err != nil {
		return domain.StoredFile{}, err
	}
	return domain.StoredFile{ID: staffFileIDHTTP, OriginalName: req.FileName, MIMEType: "application/pdf",
		SizeBytes: req.Size, Status: domain.StoredFileStored}, nil
}

func (f *petitionLogAttachmentsFake) DownloadLink(ctx context.Context, ma, id string, reader audit.Actor,
	restricted app.QuyenXemHanChe) (app.AttachmentDownload, error) {
	if err := f.admit(ctx, ma, reader, restricted); err != nil {
		return app.AttachmentDownload{}, err
	}
	return app.AttachmentDownload{URL: storage.PresignedURL("https://s3.example.gov.vn/d?X-Amz-Signature=s"),
		ExpiresAt: photoAtHTTP}, nil
}

func (f *petitionLogAttachmentsFake) PetitionAttachmentsByLogEntries(_ context.Context, ids []string) (
	map[string][]domain.PetitionLogAttachment, error) {
	f.readCalls++
	f.readIDs = ids
	if f.byEntry == nil {
		return map[string][]domain.PetitionLogAttachment{}, nil
	}
	return f.byEntry, nil
}

// newCitizenVerificationPhotosFake is the citizen scene-photo fake: the same ownership rule (code →
// citizen, per commune) and the same ListPhotos signature, as a SEPARATE instance per server.
func newCitizenVerificationPhotosFake() *citizenPhotosFake { return newCitizenPhotosFake() }

// --- rule 5, invariant 7: the four staff file routes ---------------------------------------------------

func verificationPhotosPath(code string) string { return duong(code) + "/verification-photos" }
func logAttachmentsPath(code string) string     { return duong(code) + "/log-attachments" }

func staffFileRoutes() []caTuyen {
	return []caTuyen{
		// The WRITE of the closing's evidence needs the closing key; the register's read key must not do.
		{"verification upload", http.MethodPost, verificationPhotosPath(maPhieuThuong), photoFile(),
			authz.Perm("feedback.resolve"), authz.Perm("feedback.read")},
		{"verification list", http.MethodGet, verificationPhotosPath(maPhieuThuong), nil,
			authz.Perm("feedback.read"), authz.Perm("feedback.resolve")},
		// The note route's gate: `feedback.read`; a commune-wide working key alone does not open it.
		{"log attachment upload", http.MethodPost, logAttachmentsPath(maPhieuThuong), attachmentFile(),
			authz.Perm("feedback.read"), authz.Perm("feedback.resolve")},
		{"log attachment download", http.MethodGet,
			logAttachmentsPath(maPhieuThuong) + "/" + staffFileIDHTTP + "/download", nil,
			authz.Perm("feedback.read"), authz.Perm("feedback.assign")},
	}
}

func (m *mayChu) staffFileCalls() int { return m.verificationPhotos.calls + m.petitionLogFiles.calls }

func TestStaffFileRoutes_NoSession401(t *testing.T) {
	for _, ca := range staffFileRoutes() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			doiMa(t, m.goiGhiNV(t, ca.method, hostA, ca.duong, nil, ca.than), http.StatusUnauthorized)
			if m.staffFileCalls() != 0 {
				t.Error("use case reached without a session")
			}
		})
	}
}

func TestStaffFileRoutes_WrongPermission403(t *testing.T) {
	for _, ca := range staffFileRoutes() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoaSai)
			doiMa(t, m.goiGhiNV(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than), http.StatusForbidden)
			if m.staffFileCalls() != 0 {
				t.Error("use case reached with the wrong permission")
			}
		})
	}
}

func TestStaffFileRoutes_RightPermissionWrongCommune401(t *testing.T) {
	for _, ca := range staffFileRoutes() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)
			path := strings.Replace(ca.duong, maPhieuThuong, maPhieuXaB, 1)
			doiMa(t, m.goiGhiNV(t, ca.method, hostB, path, canBoCuaXa(xaA), ca.than), http.StatusUnauthorized)
			if m.staffFileCalls() != 0 {
				t.Error("commune B reached with commune A's session — rò rỉ giữa hai cơ quan nhà nước")
			}
		})
	}
}

func TestStaffFileRoutes_RightPermissionRightCommune2xx(t *testing.T) {
	want := []int{http.StatusCreated, http.StatusOK, http.StatusCreated, http.StatusOK}
	for i, ca := range staffFileRoutes() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)
			w := m.goiGhiNV(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)
			doiMa(t, w, want[i])
			if m.staffFileCalls() != 1 {
				t.Fatalf("use case calls = %d, want 1", m.staffFileCalls())
			}
			for _, a := range []audit.Actor{m.verificationPhotos.actor, m.petitionLogFiles.actor} {
				if a.ID != "" && (a.ID != maCanBo || a.Kind != "staff") {
					t.Errorf("actor = %+v, want the BUSINESS CODE %q (rule 6, invariant 8)", a, maCanBo)
				}
			}
			if ca.method == http.MethodGet && w.Header().Get("Cache-Control") != "no-store" {
				t.Error("a reply carrying a bearer credential is cacheable")
			}
			if strings.Contains(w.Body.String(), maPhieuThuong) {
				t.Errorf("reply echoes the lookup code: %s", w.Body.String())
			}
		})
	}
}

// The restricted fact reaches every route, and a `can-bo` petition without it is the detail's 404.
func TestStaffFileRoutes_RestrictedFieldIsTheDetails404(t *testing.T) {
	for _, ca := range staffFileRoutes() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa, authz.Perm("feedback.read"))
			unknown := m.goi(t, http.MethodGet, hostA, duong(maKhongTonTai), canBoCuaXa(xaA))
			path := strings.Replace(ca.duong, maPhieuThuong, maPhieuCanBo, 1)
			w := m.goiGhiNV(t, ca.method, hostA, path, canBoCuaXa(xaA), ca.than)
			doiMa(t, w, http.StatusNotFound)
			if w.Body.String() != unknown.Body.String() {
				t.Errorf("restricted 404 differs from the unknown-code 404:\n %s\n %s", w.Body.String(), unknown.Body.String())
			}
			m.capQuyen(t, ca.khoa, authz.Perm("feedback.read"), QuyenHanChe)
			if w := m.goiGhiNV(t, ca.method, hostA, path, canBoCuaXa(xaA), ca.than); w.Code >= 300 {
				t.Errorf("with feedback.restricted: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

// The log-attachment acts take the NOTE RULE's fact: any of resolve / assign / classify.
func TestPetitionLogAttachment_NoteRightFact(t *testing.T) {
	for name, c := range map[string]struct {
		extra authz.Perm
		want  app.QuyenGhiChuCaXa
	}{
		"feedback.read only": {"", false},
		"+ feedback.assign":  {QuyenPhanCongPhieu, true},
		"+ feedback.unmask":  {QuyenXemDayDu, false},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			keys := []authz.Perm{"feedback.read"}
			if c.extra != "" {
				keys = append(keys, c.extra)
			}
			m.capQuyen(t, keys...)
			w := m.goiGhiNV(t, http.MethodPost, hostA, logAttachmentsPath(maPhieuThuong), canBoCuaXa(xaA),
				attachmentFile())
			doiMa(t, w, http.StatusCreated)
			if m.petitionLogFiles.noteRight != c.want {
				t.Errorf("note right handed down = %v, want %v", m.petitionLogFiles.noteRight, c.want)
			}
		})
	}
}

// The uploads answer the STORED file, take the declared fields, and the completion routes are gone.
func TestStaffFileUploads_StoredReplyAndDeclaredFields(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"), authz.Perm("feedback.resolve"))
	w := m.goiGhiNV(t, http.MethodPost, hostA, logAttachmentsPath(maPhieuThuong), canBoCuaXa(xaA), attachmentFile())
	doiMa(t, w, http.StatusCreated)
	var f taskAttachmentOut
	if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil || f.ID != staffFileIDHTTP || f.Status != "stored" ||
		f.FileName != "Biên bản nghiệm thu.pdf" {
		t.Errorf("reply = %s", w.Body.String())
	}
	if m.petitionLogFiles.req != (app.AttachmentUploadRequest{FileName: "Biên bản nghiệm thu.pdf",
		ContentType: storage.MIMEPDF, Size: 4096}) {
		t.Errorf("declared = %+v", m.petitionLogFiles.req)
	}
	w = m.goiGhiNV(t, http.MethodPost, hostA, verificationPhotosPath(maPhieuThuong), canBoCuaXa(xaA), photoFile())
	doiMa(t, w, http.StatusCreated)
	var p photoOut
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Status != "stored" ||
		m.verificationPhotos.req != (app.PhotoUploadRequest{ContentType: storage.MIMEPNG, Size: 2048}) {
		t.Errorf("reply = %s, declared = %+v", w.Body.String(), m.verificationPhotos.req)
	}
	for _, path := range []string{verificationPhotosPath(maPhieuThuong), logAttachmentsPath(maPhieuThuong)} {
		w := m.goiGhiNV(t, http.MethodPost, hostA, path+"/"+staffFileIDHTTP+"/completion", canBoCuaXa(xaA), nil)
		if w.Code < 400 {
			t.Errorf("%s/…/completion answered %d — the route must be gone", path, w.Code)
		}
	}
}

// --- the note carries attachments; the timeline renders them --------------------------------------------

func TestNoteWithAttachmentsCarriesIDsAndFilesBack(t *testing.T) {
	m := dungMayChu(t)
	w := m.goiGhiNV(t, http.MethodPost, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA),
		ghiChuPhieuVao{Note: "Đã gửi biên bản.", Attachments: []string{"f1", "f2"}})
	doiMa(t, w, http.StatusCreated)
	if fmt.Sprint(m.xuLy.attachmentIDs) != "[f1 f2]" {
		t.Errorf("ids reaching the use case = %v", m.xuLy.attachmentIDs)
	}
	var out nhatKyPhieuRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Attachments) != 2 || out.Attachments[0].ID != "f1" || out.Attachments[0].Status != "stored" {
		t.Errorf("attachments on the reply = %+v", out.Attachments)
	}
}

func TestNoteAttachmentRefusalsAre400WithoutTheCommune(t *testing.T) {
	for _, cause := range []error{domain.ErrAttachmentListInvalid, domain.ErrPetitionAttachmentNotUsable} {
		t.Run(cause.Error(), func(t *testing.T) {
			m := dungMayChu(t)
			m.xuLy.loi = fmt.Errorf("xu_ly_phan_anh: ghi chú cho xã %s: %w", xaA, cause)
			w := m.goiGhiNV(t, http.MethodPost, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA),
				ghiChuPhieuVao{Note: "x", Attachments: []string{"f1"}})
			doiMa(t, w, http.StatusBadRequest)
			if strings.Contains(w.Body.String(), string(xaA)) {
				t.Errorf("body echoes the commune id: %s", w.Body.String())
			}
		})
	}
}

func TestLogReadRendersAttachmentsOnePageOneRead(t *testing.T) {
	m := dungMayChu(t)
	m.petitionLogFiles.byEntry = map[string][]domain.PetitionLogAttachment{
		"nk-2": {{LogEntryID: "nk-2", FileID: "f9", OriginalName: "anh.jpg", MIMEType: storage.MIMEJPEG,
			SizeBytes: 5, Status: domain.StoredFileStored}},
	}
	w := m.goi(t, http.MethodGet, hostA, duongNhatKy(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if m.petitionLogFiles.readCalls != 1 || fmt.Sprint(m.petitionLogFiles.readIDs) != "[nk-2 nk-1]" {
		t.Errorf("batched read: %d calls, ids %v", m.petitionLogFiles.readCalls, m.petitionLogFiles.readIDs)
	}
	ra, raw := docTrangNhatKy(t, w.Body.Bytes())
	if len(ra.Items[0].Attachments) != 1 || ra.Items[0].Attachments[0].ID != "f9" {
		t.Errorf("entry nk-2 attachments = %+v", ra.Items[0].Attachments)
	}
	if v, ok := raw[1]["attachments"].([]any); !ok || len(v) != 0 {
		t.Errorf("an entry with no file must carry attachments: [] — got %#v", raw[1]["attachments"])
	}
}

// --- the close gate -----------------------------------------------------------------------------------

func TestClosureWithoutVerificationPhotoIs409WithTheCommunesSentence(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.resolve"))
	m.xuLy.loi = fmt.Errorf("xu_ly_phan_anh: đóng phiếu cho xã %s: %w", xaA, domain.ErrVerificationPhotoRequired)
	w := m.goiThan(t, http.MethodPost, hostA, duongDong(maPhieuThuong), canBoCuaXa(xaA),
		dongPhieuVao{Result: ketQuaThat})
	doiMa(t, w, http.StatusConflict)
	e := loiTra(t, w)
	if e.Code != "after_photo_required" {
		t.Errorf("code = %q", e.Code)
	}
	shipped, _ := domain.LookupShippedMessage(domain.KeyFeedbackAfterPhotoRequired)
	if want := shipped.DefaultText; e.Message != want || want == "" {
		t.Errorf("sentence = %q, want the shipped `feedback.after_photo_required` %q", e.Message, shipped.DefaultText)
	}
	if strings.Contains(w.Body.String(), string(xaA)) {
		t.Errorf("body echoes the commune id: %s", w.Body.String())
	}
}

// --- refusal mapping of the verification photo -----------------------------------------------------------

func TestVerificationPhotoRefusalMapping(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{app.ErrVerificationPhotoWindowClosed, http.StatusConflict, "petition_state"},
		{app.ErrVerificationPhotoCountReached, http.StatusConflict, "photo_limit"},
		{app.ErrVerificationPhotoNotFound, http.StatusNotFound, "not_found"},
		{app.ErrPhotoTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		{app.ErrPhotoTooLarge, http.StatusRequestEntityTooLarge, "file_too_large"},
		{&app.AttachmentRejection{Reason: app.RejectMalware}, http.StatusUnprocessableEntity, "photo_rejected"},
		{fmt.Errorf("tệp tải lên x: %w", httpx.ErrUploadTimeout), http.StatusRequestTimeout, "upload_timeout"},
		{app.ErrUploadNotReceived, http.StatusInternalServerError, "internal"},
		{app.ErrUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{app.ErrScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{errors.New("ảnh sau xử lý: x cho xã " + string(xaA) + ": db down"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(c.code+"/"+c.err.Error(), func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("feedback.resolve"))
			m.verificationPhotos.err = c.err
			w := m.goiGhiNV(t, http.MethodPost, hostA, verificationPhotosPath(maPhieuThuong), canBoCuaXa(xaA),
				photoFile())
			doiMa(t, w, c.status)
			if loiTra(t, w).Code != c.code {
				t.Errorf("code = %q, want %q", loiTra(t, w).Code, c.code)
			}
			if strings.Contains(w.Body.String(), string(xaA)) {
				t.Errorf("body echoes the commune id: %s", w.Body.String())
			}
		})
	}
}

func TestPetitionLogAttachmentRefusalMapping(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{app.ErrAttachmentNotFound, http.StatusNotFound, "not_found"},
		{app.ErrKhongPhaiNguoiDuocGiao, http.StatusForbidden, "forbidden"},
		{app.ErrPetitionLogAttachmentCountReached, http.StatusConflict, "attachment_limit"},
		{&app.AttachmentRejection{Reason: app.RejectTypeMismatch}, http.StatusUnprocessableEntity, "attachment_rejected"},
		{app.ErrAttachmentTooLarge, http.StatusRequestEntityTooLarge, "file_too_large"},
		{fmt.Errorf("tệp tải lên x: %w", httpx.ErrUploadMalformed), http.StatusBadRequest, "invalid_upload"},
		{app.ErrUploadLimitsUnavailable, http.StatusServiceUnavailable, "upload_limits_unavailable"},
		{errors.New("tệp: db down"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(c.code, func(t *testing.T) {
			m := dungMayChu(t)
			m.petitionLogFiles.err = c.err
			w := m.goiGhiNV(t, http.MethodPost, hostA, logAttachmentsPath(maPhieuThuong), canBoCuaXa(xaA),
				attachmentFile())
			doiMa(t, w, c.status)
			if loiTra(t, w).Code != c.code {
				t.Errorf("code = %q, want %q", loiTra(t, w).Code, c.code)
			}
		})
	}
}

// --- the citizen's read of verification photos -------------------------------------------------------------

func verificationPhotosMinePath(code string) string {
	return "/api/v1/my-citizen-reports/" + code + "/verification-photos"
}

func buildVerificationPhotoServer(t *testing.T) (*photoServer, *citizenPhotosFake) {
	t.Helper()
	scene := newCitizenPhotosFake()
	after := newCitizenVerificationPhotosFake()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu: phieuCuaToiMau(), GuiPhieu: soPhieuMoi(), Rating: newRatingFake(), NhanLinhVuc: nhanLinhVucMau(),
		CitizenFields: newFieldCatalogueFake(), Photos: scene, VerificationPhotos: after,
		PhotoLimiter: photoLimiterThu(), UploadSlots: uploadSlotsThu(), Log: log,
	})
	var h http.Handler = mux
	h = idem.Middleware(khoIdemMoi(), log)(h)
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(&soPhienCongDanGia{})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &photoServer{h: h, photos: scene}, after
}

func TestCitizenVerificationPhotos_NoSession401(t *testing.T) {
	s, after := buildVerificationPhotoServer(t)
	doiMa(t, s.do(t, http.MethodGet, verificationPhotosMinePath(maCuaToi), "", "", ""), http.StatusUnauthorized)
	if after.calls != 0 {
		t.Error("use case reached without a session")
	}
}

func TestCitizenVerificationPhotos_OwnPetition200FromTheSession(t *testing.T) {
	s, after := buildVerificationPhotoServer(t)
	w := s.do(t, http.MethodGet, verificationPhotosMinePath(maCuaToi)+"?citizen_id="+idNguoiKhac, "", tokenCuaToi, "")
	doiMa(t, w, http.StatusOK)
	if after.calls != 1 || after.seenTenant[0] != xaA || after.seenCitizen[0].ID != idToi {
		t.Fatalf("use case saw commune %v citizen %+v", after.seenTenant, after.seenCitizen)
	}
	if s.photos.calls != 0 {
		t.Error("the verification route reached the SCENE photo reader")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("signed links cacheable")
	}
	for _, internal := range []string{"object_key", "uploaded_by", "file_name", "subject", maCuaToi, idToi} {
		if strings.Contains(w.Body.String(), internal) {
			t.Errorf("%q on the citizen's reply", internal)
		}
	}
}

// Rule 4, forbidden #2: another citizen's code, another commune's and an unknown one are the GET's 404.
func TestCitizenVerificationPhotos_IdenticalNotFound(t *testing.T) {
	s, _ := buildVerificationPhotoServer(t)
	unknown := s.do(t, http.MethodGet, duongCuaToi(maKhongTonTai), "", tokenCuaToi, "")
	doiMa(t, unknown, http.StatusNotFound)
	for name, c := range map[string]struct{ code, token string }{
		"another citizen's code":          {maCuaNguoiKhac, tokenCuaToi},
		"own code, session in another xã": {maCuaToi, tokenXaB},
		"unknown code":                    {maKhongTonTai, tokenCuaToi},
	} {
		t.Run(name, func(t *testing.T) {
			w := s.do(t, http.MethodGet, verificationPhotosMinePath(c.code), "", c.token, "")
			doiMa(t, w, http.StatusNotFound)
			if w.Body.String() != unknown.Body.String() {
				t.Errorf("404 body differs:\n %s\n %s", w.Body.String(), unknown.Body.String())
			}
		})
	}
}

// The citizen path is NOT on the staff mux, and the staff list is NOT on the citizen mux (rule 4, inv. 5).
func TestVerificationPhotoSurfacesAreSeparate(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, verificationPhotosMinePath(maPhieuThuong), canBoCuaXa(xaA))
	if w.Code == http.StatusOK || m.verificationPhotos.calls != 0 {
		t.Errorf("citizen path answered %d on the staff mux", w.Code)
	}
	s, after := buildVerificationPhotoServer(t)
	w = s.do(t, http.MethodGet, verificationPhotosPath(maCuaToi), "", tokenCuaToi, "")
	if w.Code == http.StatusOK || after.calls != 0 {
		t.Errorf("staff path answered %d on the citizen mux", w.Code)
	}
	// Log attachments have NO citizen route at all.
	w = s.do(t, http.MethodGet, "/api/v1/my-citizen-reports/"+maCuaToi+"/log-attachments", "", tokenCuaToi, "")
	if w.Code == http.StatusOK {
		t.Error("a citizen route serves log attachments")
	}
}
