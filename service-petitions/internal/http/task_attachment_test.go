package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for §5.9's four attachment routes (request, completion, download, remove), and the attachment half of the two log-entry routes, at the
// HTTP boundary.
//
//	PROVED HERE   each route's four cases (rule 5, invariant 7): 401 no session · 403 without
//	              `task.read` · 401 right key WRONG COMMUNE (authz compares the commune first) · 2xx —
//	              with the use case NOT REACHED in the first three · the commune the use case sees is the
//	              Host's · the actor is the staff business code · `task.update` is read from that key ·
//	              the declared body reaches the use case · the reply shapes (form, file, link) · every
//	              refusal's status, code and sentence, none of them carrying the commune id · the
//	              timeline reads a page's attachments in ONE call keyed by that page's entry ids.
//	NOT PROVED    the flow itself — internal/app/task_attachment_test.go, over the real stores.

const fileIDHTTP = "01JTEPHTTP0000000000000001"

// taskAttachmentsFake records the commune, the actor and the facts every call carried.
type taskAttachmentsFake struct {
	calls   int
	commune tenant.ID
	actor   audit.Actor
	code    string
	fileID  string
	update  app.TaskUpdateRight
	req     app.AttachmentUploadRequest
	reason  string
	err     error
}

func (f *taskAttachmentsFake) Remove(ctx context.Context, ma, id, reason string, actor audit.Actor,
	update app.TaskUpdateRight) error {
	f.record(ctx, ma, id, actor)
	f.reason, f.update = reason, update
	return f.err
}

func (f *taskAttachmentsFake) record(ctx context.Context, code, fileID string, actor audit.Actor) {
	f.calls++
	f.code, f.fileID, f.actor = code, fileID, actor
	f.commune = tenant.MustFrom(ctx)
}

func (f *taskAttachmentsFake) RequestUpload(ctx context.Context, ma string, req app.AttachmentUploadRequest,
	actor audit.Actor, update app.TaskUpdateRight) (app.AttachmentUpload, error) {
	f.record(ctx, ma, "", actor)
	f.req, f.update = req, update
	if f.err != nil {
		return app.AttachmentUpload{}, f.err
	}
	return app.AttachmentUpload{
		File: domain.StoredFile{ID: fileIDHTTP, OriginalName: req.FileName, Status: domain.StoredFilePending},
		Post: storage.PresignedPost{URL: "https://s3.example.gov.vn/vigov-test-temp",
			Fields:    map[string]string{"key": "upload/records/k/original.pdf", "policy": "p", "x-amz-signature": "s"},
			ExpiresAt: time.Date(2026, 9, 29, 3, 15, 0, 0, time.UTC)},
	}, nil
}

func (f *taskAttachmentsFake) Complete(ctx context.Context, ma, id string, actor audit.Actor,
	update app.TaskUpdateRight) (domain.StoredFile, error) {
	f.record(ctx, ma, id, actor)
	f.update = update
	if f.err != nil {
		return domain.StoredFile{}, f.err
	}
	return domain.StoredFile{ID: id, OriginalName: "Biên bản.pdf", MIMEType: storage.MIMEPDF, SizeBytes: 48213,
		Status: domain.StoredFileStored}, nil
}

func (f *taskAttachmentsFake) DownloadLink(ctx context.Context, ma, id string, reader audit.Actor) (
	app.AttachmentDownload, error) {
	f.record(ctx, ma, id, reader)
	if f.err != nil {
		return app.AttachmentDownload{}, f.err
	}
	return app.AttachmentDownload{URL: storage.PresignedURL("https://s3.example.gov.vn/vigov-test-private/k?X-Amz-Signature=s"),
		ExpiresAt: time.Date(2026, 9, 29, 3, 15, 0, 0, time.UTC)}, nil
}

// logAttachmentsFake is the batched timeline read, KEYED BY COMMUNE.
type logAttachmentsFake struct {
	byCommune map[tenant.ID]map[string][]domain.TaskLogAttachment
	calls     int
	ids       []string
	commune   tenant.ID
	err       error
}

func (l *logAttachmentsFake) AttachmentsByLogEntries(ctx context.Context, ids []string) (
	map[string][]domain.TaskLogAttachment, error) {
	l.calls++
	l.ids, l.commune = ids, tenant.MustFrom(ctx)
	if l.err != nil {
		return nil, l.err
	}
	out := map[string][]domain.TaskLogAttachment{}
	for _, id := range ids {
		if a, ok := l.byCommune[l.commune][id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

func attachmentsPath(ma string) string    { return duongNV(ma) + "/attachments" }
func completionPath(ma, id string) string { return attachmentsPath(ma) + "/" + id + "/completion" }
func attachmentDownloadPath(ma, id string) string {
	return attachmentsPath(ma) + "/" + id + "/download"
}

func uploadIn() taskAttachmentUploadIn {
	return taskAttachmentUploadIn{FileName: "Biên bản nghiệm thu.pdf", ContentType: storage.MIMEPDF, Size: 48213}
}

// --- rule 5, invariant 7 --------------------------------------------------------------------------

type attachmentRouteCase struct {
	name   string
	method string
	path   string
	body   any
	ok     int
}

func attachmentRouteCases() []attachmentRouteCase {
	return []attachmentRouteCase{
		{"request upload", http.MethodPost, attachmentsPath(maNVThu), uploadIn(), http.StatusCreated},
		{"completion", http.MethodPost, completionPath(maNVThu, fileIDHTTP), nil, http.StatusOK},
		{"download", http.MethodGet, attachmentDownloadPath(maNVThu, fileIDHTTP), nil, http.StatusOK},
		{"remove", http.MethodDelete, attachmentPath(maNVThu, fileIDHTTP), removeIn(), http.StatusNoContent},
	}
}

func attachmentPath(ma, id string) string { return attachmentsPath(ma) + "/" + id }

func removeIn() taskAttachmentRemoveIn {
	return taskAttachmentRemoveIn{Reason: "Tải nhầm tệp, đã thay bằng bản đúng."}
}

func TestAttachmentRoutes_NoSession401(t *testing.T) {
	for _, c := range attachmentRouteCases() {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			doiMa(t, m.goiGhiNV(t, c.method, hostA, c.path, nil, c.body), http.StatusUnauthorized)
			if m.taskAttachments.calls != 0 {
				t.Errorf("use case reached without a session (%d calls)", m.taskAttachments.calls)
			}
		})
	}
}

// A REAL key of this subsystem, just not the one: `task.create` and `task.update` without `task.read`.
func TestAttachmentRoutes_WrongPermission403(t *testing.T) {
	for _, c := range attachmentRouteCases() {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.create"), authz.Perm("task.update"))
			doiMa(t, m.goiGhiNV(t, c.method, hostA, c.path, canBoCuaXa(xaA), c.body), http.StatusForbidden)
			if m.taskAttachments.calls != 0 {
				t.Errorf("use case reached with the wrong permission (%d calls)", m.taskAttachments.calls)
			}
		})
	}
}

func TestAttachmentRoutes_RightPermissionWrongCommune401(t *testing.T) {
	for _, c := range attachmentRouteCases() {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.read"))
			doiMa(t, m.goiGhiNV(t, c.method, hostB, c.path, canBoCuaXa(xaA), c.body), http.StatusUnauthorized)
			if m.taskAttachments.calls != 0 {
				t.Errorf("commune B's files reached with commune A's session (%d calls) — rò rỉ giữa hai cơ quan nhà nước",
					m.taskAttachments.calls)
			}
		})
	}
}

func TestAttachmentRoutes_RightPermissionRightCommunePass(t *testing.T) {
	for _, c := range attachmentRouteCases() {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.read"))
			doiMa(t, m.goiGhiNV(t, c.method, hostA, c.path, canBoCuaXa(xaA), c.body), c.ok)
			f := m.taskAttachments
			if f.calls != 1 || f.commune != xaA || f.code != maNVThu {
				t.Fatalf("use case: %d calls, commune %q, code %q", f.calls, f.commune, f.code)
			}
			// Rule 6, invariant 8: the business code, never the internal id.
			if f.actor.ID != maCanBo || f.actor.Kind != "staff" || f.actor.IP == "" {
				t.Errorf("actor = %+v", f.actor)
			}
			if c.path != attachmentsPath(maNVThu) && f.fileID != fileIDHTTP {
				t.Errorf("file id reaching the use case = %q", f.fileID)
			}
		})
	}
}

// --- behaviour ------------------------------------------------------------------------------------

func TestRequestUpload_BodyAndFactsReachUseCaseReplyCarriesForm(t *testing.T) {
	for _, c := range []struct {
		perms []authz.Perm
		want  bool
	}{
		{[]authz.Perm{"task.read"}, false},
		{[]authz.Perm{"task.read", "task.approve"}, false},
		{[]authz.Perm{"task.read", "task.update"}, true},
	} {
		m := dungMayChu(t)
		m.capQuyen(t, c.perms...)
		w := m.goiGhiNV(t, http.MethodPost, hostA, attachmentsPath(maNVThu), canBoCuaXa(xaA), uploadIn())
		doiMa(t, w, http.StatusCreated)
		if bool(m.taskAttachments.update) != c.want {
			t.Errorf("%v: task.update fact = %v, want %v", c.perms, m.taskAttachments.update, c.want)
		}
		if m.taskAttachments.req != (app.AttachmentUploadRequest{FileName: "Biên bản nghiệm thu.pdf",
			ContentType: storage.MIMEPDF, Size: 48213}) {
			t.Errorf("body reaching the use case = %+v", m.taskAttachments.req)
		}
		var out taskAttachmentUploadOut
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("body: %s", w.Body.String())
		}
		if out.Attachment.ID != fileIDHTTP || out.Attachment.Status != "pending" ||
			out.Upload.URL == "" || out.Upload.Fields["policy"] == "" || out.Upload.ExpiresAt.IsZero() {
			t.Errorf("reply = %+v", out)
		}
	}
}

func TestCompleteAndDownloadReplies(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.read"))
	w := m.goiGhiNV(t, http.MethodPost, hostA, completionPath(maNVThu, fileIDHTTP), canBoCuaXa(xaA), nil)
	doiMa(t, w, http.StatusOK)
	var f taskAttachmentOut
	if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
		t.Fatalf("body: %s", w.Body.String())
	}
	if f.ID != fileIDHTTP || f.Status != "stored" || f.MIMEType != storage.MIMEPDF || f.SizeBytes != 48213 {
		t.Errorf("file = %+v", f)
	}

	w = m.goiGhiNV(t, http.MethodGet, hostA, attachmentDownloadPath(maNVThu, fileIDHTTP), canBoCuaXa(xaA), nil)
	doiMa(t, w, http.StatusOK)
	var d taskAttachmentDownloadOut
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatalf("body: %s", w.Body.String())
	}
	if !strings.HasPrefix(d.URL, "https://s3.example.gov.vn/") || d.ExpiresAt.IsZero() {
		t.Errorf("link = %+v", d)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store — the link is a bearer credential", w.Header().Get("Cache-Control"))
	}
}

func TestTaskAttachmentRefusalsMap(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"storage not configured", app.ErrUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{"platform unreachable", app.ErrUploadLimitsUnavailable, http.StatusServiceUnavailable, "upload_limits_unavailable"},
		{"scanner unreachable", app.ErrScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{"declared too large", app.ErrAttachmentTooLarge, http.StatusBadRequest, "invalid_request"},
		{"declared type refused", app.ErrAttachmentTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		{"file name", domain.ErrAttachmentNameInvalid, http.StatusBadRequest, "invalid_request"},
		{"infected", &app.AttachmentRejection{Reason: app.RejectMalware}, http.StatusUnprocessableEntity, "attachment_rejected"},
		{"sniff mismatch", &app.AttachmentRejection{Reason: app.RejectTypeMismatch}, http.StatusUnprocessableEntity, "attachment_rejected"},
		{"count reached", app.ErrAttachmentCountReached, http.StatusConflict, "attachment_limit"},
		{"not received", app.ErrUploadNotReceived, http.StatusConflict, "upload_not_received"},
		{"expired", app.ErrUploadExpired, http.StatusConflict, "upload_expired"},
		{"replaced mid-inspection", app.ErrUploadChanged, http.StatusConflict, "upload_changed"},
		{"no longer pending", app.ErrAttachmentNotPending, http.StatusConflict, "attachment_state"},
		{"file not found", app.ErrAttachmentNotFound, http.StatusNotFound, "not_found"},
		{"task not found", petstore.ErrNhiemVuKhongTonTai, http.StatusNotFound, "not_found"},
		{"not a participant", domain.ErrNotTaskParticipant, http.StatusForbidden, "forbidden"},
		{"system failure", errors.New("minio: 500"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.read"))
			m.taskAttachments.err = bocNhuApp(c.err)
			for _, rc := range attachmentRouteCases() {
				w := m.goiGhiNV(t, rc.method, hostA, rc.path, canBoCuaXa(xaA), rc.body)
				doiMa(t, w, c.want)
				if e := loiTra(t, w); e.Code != c.code || e.Message == "" {
					t.Errorf("%s: error = %q / %q, want code %q", rc.name, e.Code, e.Message, c.code)
				}
				if strings.Contains(w.Body.String(), xaBocThu) {
					t.Errorf("%s: body leaks the commune id: %s", rc.name, w.Body.String())
				}
			}
		})
	}
}

// The removal: the reason and the `task.update` fact reach the use case; 204 with no body.
func TestRemoveTaskAttachment_ReasonAndFactsReachUseCase(t *testing.T) {
	for _, c := range []struct {
		perms []authz.Perm
		want  bool
	}{
		{[]authz.Perm{"task.read"}, false},
		{[]authz.Perm{"task.read", "task.update"}, true},
	} {
		m := dungMayChu(t)
		m.capQuyen(t, c.perms...)
		w := m.goiGhiNV(t, http.MethodDelete, hostA, attachmentPath(maNVThu, fileIDHTTP), canBoCuaXa(xaA), removeIn())
		doiMa(t, w, http.StatusNoContent)
		f := m.taskAttachments
		if bool(f.update) != c.want || f.reason != removeIn().Reason || f.fileID != fileIDHTTP || f.code != maNVThu {
			t.Errorf("%v: update=%v reason=%q file=%q task=%q", c.perms, f.update, f.reason, f.fileID, f.code)
		}
		if w.Body.Len() != 0 {
			t.Errorf("204 with a body: %s", w.Body.String())
		}
	}
}

// The removal's own refusals: status, code, the domain's sentence, never the commune id.
func TestRemoveTaskAttachment_RefusalsMap(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want int
		code string
	}{
		{"reason missing", domain.ErrAttachmentRemovalReasonMissing, http.StatusBadRequest, "invalid_request"},
		{"reason too long", domain.ErrAttachmentRemovalReasonTooLong, http.StatusBadRequest, "invalid_request"},
		{"neither uploader nor task.update", domain.ErrAttachmentRemovalNotAllowed, http.StatusForbidden, "forbidden"},
		{"legal hold", domain.ErrAttachmentUnderLegalHold, http.StatusConflict, "legal_hold"},
		{"unknown / other task / already removed", app.ErrAttachmentNotFound, http.StatusNotFound, "not_found"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.read"))
			m.taskAttachments.err = bocNhuApp(c.err)
			w := m.goiGhiNV(t, http.MethodDelete, hostA, attachmentPath(maNVThu, fileIDHTTP), canBoCuaXa(xaA), removeIn())
			doiMa(t, w, c.want)
			e := loiTra(t, w)
			if e.Code != c.code || e.Message == "" {
				t.Errorf("error = %q / %q, want code %q", e.Code, e.Message, c.code)
			}
			if c.code != "not_found" && c.code != "legal_hold" && e.Message != c.err.Error() {
				t.Errorf("sentence = %q, want the domain's %q", e.Message, c.err.Error())
			}
			if strings.Contains(w.Body.String(), xaBocThu) {
				t.Errorf("body leaks the commune id: %s", w.Body.String())
			}
		})
	}
}

// Each rejection reason has its own sentence; malware is named as malware.
func TestRejectionSentenceNamesTheReason(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.read"))
	m.taskAttachments.err = &app.AttachmentRejection{Reason: app.RejectMalware}
	w := m.goiGhiNV(t, http.MethodPost, hostA, completionPath(maNVThu, fileIDHTTP), canBoCuaXa(xaA), nil)
	if e := loiTra(t, w); !strings.Contains(e.Message, "mã độc") {
		t.Errorf("sentence = %q", e.Message)
	}
	for _, reason := range []string{app.RejectMalware, app.RejectTypeNotAllowed, app.RejectTypeMismatch,
		app.RejectTooLarge, app.RejectCountReached} {
		if rejectionSentences[reason] == "" {
			t.Errorf("reason %q has no sentence", reason)
		}
	}
}

// --- the log-entry half ---------------------------------------------------------------------------

func TestAddTaskLogEntry_AttachmentsReachUseCaseAndReply(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.read"))
	w := m.goiGhiNV(t, http.MethodPost, hostA, taskLogEntriesPath(maNVThu), canBoCuaXa(xaA),
		taskLogEntryIn{Note: "Đã nghiệm thu.", Attachments: []string{"f1", "f2"}})
	doiMa(t, w, http.StatusCreated)
	if got := m.ghiNhiemVu.logAttachments; len(got) != 2 || got[0] != "f1" || got[1] != "f2" {
		t.Errorf("attachments reaching the use case = %v", got)
	}
	var row nhatKyNhiemVuRa
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil {
		t.Fatalf("body: %s", w.Body.String())
	}
	if len(row.Attachments) != 2 || row.Attachments[0].ID != "f1" || row.Attachments[0].Status != "stored" {
		t.Errorf("attachments in the reply = %+v", row.Attachments)
	}

	// Without attachments the reply still carries `attachments: []`, never null.
	w = m.goiGhiNV(t, http.MethodPost, hostA, taskLogEntriesPath(maNVThu), canBoCuaXa(xaA),
		taskLogEntryIn{Note: "Không kèm tệp."})
	doiMa(t, w, http.StatusCreated)
	if !strings.Contains(w.Body.String(), `"attachments":[]`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestAddTaskLogEntry_AttachmentRefusalsAre400WithSentence(t *testing.T) {
	for _, e := range []error{domain.ErrAttachmentNotUsable, domain.ErrAttachmentListInvalid} {
		m := dungMayChu(t)
		m.capQuyen(t, authz.Perm("task.read"))
		m.ghiNhiemVu.loi = bocNhuApp(e)
		w := m.goiGhiNV(t, http.MethodPost, hostA, taskLogEntriesPath(maNVThu), canBoCuaXa(xaA),
			taskLogEntryIn{Note: "x", Attachments: []string{"f1"}})
		doiMa(t, w, http.StatusBadRequest)
		if got := loiTra(t, w); got.Message != e.Error() {
			t.Errorf("sentence = %q, want %q", got.Message, e.Error())
		}
		if strings.Contains(w.Body.String(), xaBocThu) {
			t.Errorf("body leaks the commune id: %s", w.Body.String())
		}
	}
}

func TestTimelineReadsPageAttachmentsInOneCall(t *testing.T) {
	m := dungMayChuNhatKyNV(t)
	m.logAttachments.byCommune = map[tenant.ID]map[string][]domain.TaskLogAttachment{
		xaA: {"nknv-2": {{LogEntryID: "nknv-2", FileID: "f1", OriginalName: "Biên bản.pdf",
			MIMEType: storage.MIMEPDF, SizeBytes: 48213, Status: domain.StoredFileStored}}},
		// Commune B holds a file under the SAME entry id — a handler that lost the commune would show it.
		xaB: {"nknv-2": {{LogEntryID: "nknv-2", FileID: "f-b", OriginalName: "Của xã B.pdf"}}},
	}
	w := m.goi(t, http.MethodGet, hostA, duongNhatKyNV(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	l := m.logAttachments
	if l.calls != 1 || l.commune != xaA || len(l.ids) != 2 || l.ids[0] != "nknv-2" || l.ids[1] != "nknv-1" {
		t.Fatalf("attachment read: %d calls, commune %q, ids %v — want ONE call for the page", l.calls, l.commune, l.ids)
	}
	var page struct {
		Items []nhatKyNhiemVuRa `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("body: %s", w.Body.String())
	}
	if len(page.Items) != 2 || len(page.Items[0].Attachments) != 1 || page.Items[0].Attachments[0].ID != "f1" ||
		page.Items[0].Attachments[0].FileName != "Biên bản.pdf" || len(page.Items[1].Attachments) != 0 {
		t.Errorf("items = %+v", page.Items)
	}
	if strings.Contains(w.Body.String(), "Của xã B") {
		t.Error("body carries commune B's file")
	}
}

func TestTimelineAttachmentReadFailureIs500(t *testing.T) {
	m := dungMayChuNhatKyNV(t)
	m.logAttachments.err = errors.New("pg: timeout")
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongNhatKyNV(maNhiemVuA), canBoCuaXa(xaA)),
		http.StatusInternalServerError)
}
