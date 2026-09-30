package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the three attachment acts (task_attachment.go) over the REAL stores on the fake driver,
// with in-memory fakes for MinIO, clamd and platform's upload policy.
//
//	PROVED HERE   request: who may upload is who may write the log · platform's policy decides type,
//	              size and count, and its absence or outage refuses BEFORE any row · the pending row
//	              binds commune, task, uploader, records class and a key with no file name in it · the
//	              form is signed for that key, the policy's byte limit and the declared type · the row
//	              and its audit entry share ONE transaction, and the trail carries no file name.
//	              completion: only the uploader completes · the SNIFFED type decides, a mismatch with
//	              the declaration or a type outside the policy is rejected · a size above the CURRENT
//	              policy is rejected · infected is rejected with the temp object deleted and the
//	              signature in the trail · scanner down, or an object replaced mid-inspection, writes
//	              NOTHING and leaves the row pending · the stored path scans, hashes and promotes the
//	              same bytes, then writes scanning → stored and its trail in one transaction · a missing
//	              object is "not yet" inside the form's lifetime and `failed` after it · a destination
//	              already written is recovered, not duplicated · a stored file answers as stored again.
//	              download: another commune, another task, an unattached file of somebody else → 404 ·
//	              the link is signed for the private key, with the original name and the TTL ceiling.
//
//	NOT PROVED    MinIO, clamd and PostgreSQL themselves: core/storage and core/malwarescan carry their
//	              own suites (integration ones SKIP without a server), migration 0021's triggers are
//	              stored_file_pg_test.go's (SKIPS without VIGOV_TEST_DSN).

const fileIDThu = "01JTEPD1NHKEM0000000000001"

func dungTaskAttachments(t *testing.T, k *khoNhiemVuGia) (*TaskAttachments, *objectStoreFake, *scannerFake,
	*policyFake, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := pkgstore.New(db)
	obj := newObjectStoreFake()
	sc := &scannerFake{res: malwarescan.Result{Clean: true}}
	pol := taskAttachmentPolicy()
	uc := NewTaskAttachments(kho, petstore.NewNhiemVuStore(kho), petstore.NewStoredFileStore(kho), obj, sc, pol)
	uc.newID = func() (string, error) { return fileIDThu, nil }
	uc.now = func() time.Time { return mocThaoTacNV }
	return uc, obj, sc, pol, ctxXa(xaThu)
}

func uploadReq() AttachmentUploadRequest {
	return AttachmentUploadRequest{FileName: `C:\fakepath\Biên bản nghiệm thu.pdf`, ContentType: storage.MIMEPDF,
		Size: 48_213}
}

// --- a. request ------------------------------------------------------------------------------------

func TestRequestUpload_IssuesPendingRowFormAndTrailInOneTransaction(t *testing.T) {
	k := khoNVMau()
	uc, obj, _, _, ctx := dungTaskAttachments(t, k)

	up, err := uc.RequestUpload(ctx, maNVGoc, uploadReq(), staffActor(maNguoiThucHien), false)
	if err != nil {
		t.Fatalf("RequestUpload: %v", err)
	}

	wantKey := "records/t_" + strings.ToLower(string(xaThu)) + "/2026/09/petitions/task-attachment/" +
		strings.ToLower(fileIDThu) + "/original.pdf"
	f := up.File
	if f.ID != fileIDThu || f.ObjectKey != wantKey || f.SubjectID != idNVGoc || f.UploadedBy != maNguoiThucHien ||
		f.OriginalName != "Biên bản nghiệm thu.pdf" || f.Status != domain.StoredFilePending ||
		f.RetentionClass != "records" || f.Bucket != domain.StoredFileBucketPrivate {
		t.Errorf("tệp chờ = %+v", f)
	}
	// THE NAME IS NEVER IN THE KEY (ADR 0052 §3, rule 3 forbidden #4).
	if strings.Contains(strings.ToLower(f.ObjectKey), "bien") {
		t.Errorf("tên tệp lọt vào khoá đối tượng: %s", f.ObjectKey)
	}

	ins := k.cau("INSERT INTO stored_file")
	if len(ins) != 1 || ins[0].args[0] != string(xaThu) || ins[0].args[7] != idNVGoc || ins[0].args[9] != maNguoiThucHien {
		t.Fatalf("dòng chèn = %+v", ins)
	}
	chiGhiTrongGiaoDich(t, k)

	if len(obj.presigned) != 1 {
		t.Fatalf("ký %d lượt tải lên, muốn 1", len(obj.presigned))
	}
	p := obj.presigned[0]
	if p.key != "upload/"+wantKey || p.maxBytes != 10<<20 || p.contentType != storage.MIMEPDF || p.ttl != storage.UploadTTL {
		t.Errorf("lượt ký = %+v — muốn khoá upload/<khoá đích>, giới hạn của chính sách, kiểu khai báo, TTL 15 phút", p)
	}
	if up.Post.URL == "" || up.Post.Fields["key"] != "upload/"+wantKey {
		t.Errorf("biểu mẫu trả về = %v", up.Post.Fields)
	}

	vet := vetKiemToan(t, k)
	if vet.args[1] != maNguoiThucHien || vet.args[4] != ActionTaskAttachmentRequested || vet.args[5] != maNVGoc {
		t.Errorf("vết: chủ thể=%v hành vi=%v đối tượng=%v", vet.args[1], vet.args[4], vet.args[5])
	}
	delta := auditDeltaText(t, k)
	for _, want := range []string{`"tep_id":"` + fileIDThu + `"`, `"loai_khai_bao":"application/pdf"`,
		`"kich_thuoc_khai":48213`} {
		if !strings.Contains(delta, want) {
			t.Errorf("delta thiếu %s: %s", want, delta)
		}
	}
	if strings.Contains(delta, "Biên bản") || strings.Contains(delta, "nghiệm thu") {
		t.Errorf("tên tệp lọt vào vết kiểm toán: %s", delta)
	}
}

func TestRequestUpload_RelatedPersonAndTaskUpdateHolderMayUpload(t *testing.T) {
	for _, c := range []struct {
		code   string
		update TaskUpdateRight
	}{{"CB-00123", false}, {maLanhDao, false}, {outsiderCode, true}} {
		k := khoNVMau()
		uc, _, _, _, ctx := dungTaskAttachments(t, k)
		if _, err := uc.RequestUpload(ctx, maNVGoc, uploadReq(), staffActor(c.code), c.update); err != nil {
			t.Errorf("%s (update=%v): %v", c.code, c.update, err)
		}
	}
}

func TestRequestUpload_RefusalsWriteNothing(t *testing.T) {
	for _, c := range []struct {
		name   string
		mod    func(*TaskAttachments, *policyFake, *AttachmentUploadRequest)
		actor  string
		want   error
		signed bool // whether the refusal may come after the transaction opened
	}{
		{"người ngoài", nil, outsiderCode, domain.ErrNotTaskParticipant, true},
		{"chưa cấu hình chính sách", func(_ *TaskAttachments, p *policyFake, _ *AttachmentUploadRequest) { p.ok = false },
			maNguoiThucHien, ErrUploadNotConfigured, false},
		{"nền tảng không trả lời", func(_ *TaskAttachments, p *policyFake, _ *AttachmentUploadRequest) {
			p.err = uploadpolicy.ErrUnavailable
		}, maNguoiThucHien, ErrUploadLimitsUnavailable, false},
		{"chưa cấu hình kho", func(uc *TaskAttachments, _ *policyFake, _ *AttachmentUploadRequest) { uc.objects = nil },
			maNguoiThucHien, ErrUploadNotConfigured, false},
		{"chưa cấu hình máy quét", func(uc *TaskAttachments, _ *policyFake, _ *AttachmentUploadRequest) { uc.scanner = nil },
			maNguoiThucHien, ErrUploadNotConfigured, false},
		{"khai quá dung lượng", func(_ *TaskAttachments, _ *policyFake, r *AttachmentUploadRequest) { r.Size = 10<<20 + 1 },
			maNguoiThucHien, ErrAttachmentTooLarge, false},
		{"khai kiểu ngoài chính sách", func(_ *TaskAttachments, _ *policyFake, r *AttachmentUploadRequest) {
			r.ContentType = storage.MIMEWebP
		}, maNguoiThucHien, ErrAttachmentTypeNotAllowed, false},
		{"khai kiểu không có trong kho", func(_ *TaskAttachments, _ *policyFake, r *AttachmentUploadRequest) {
			r.ContentType = "text/html"
		}, maNguoiThucHien, ErrAttachmentTypeNotAllowed, false},
		{"tên rỗng", func(_ *TaskAttachments, _ *policyFake, r *AttachmentUploadRequest) { r.FileName = "  " },
			maNguoiThucHien, domain.ErrAttachmentNameInvalid, false},
		{"kích thước 0", func(_ *TaskAttachments, _ *policyFake, r *AttachmentUploadRequest) { r.Size = 0 },
			maNguoiThucHien, domain.ErrAttachmentSizeInvalid, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			uc, obj, _, pol, ctx := dungTaskAttachments(t, k)
			req := uploadReq()
			if c.mod != nil {
				c.mod(uc, pol, &req)
			}
			_, err := uc.RequestUpload(ctx, maNVGoc, req, staffActor(c.actor), false)
			if !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			khongGhiGi(t, k)
			if len(obj.presigned) != 0 {
				t.Error("đã ký biểu mẫu tải lên dù bị từ chối")
			}
			if !c.signed && k.batDau != 0 {
				t.Errorf("mở %d giao dịch cho một yêu cầu bị từ chối trước khi chạm dữ liệu", k.batDau)
			}
		})
	}
}

func TestRequestUpload_CountLimitFromPolicy(t *testing.T) {
	k := khoNVMau()
	uc, _, _, _, ctx := dungTaskAttachments(t, k)
	// Three live files already: stored, ready, and a pending one whose form is still valid.
	k.addStoredFile(storedFileRow(string(xaThu), "01JTEPCU00000000000000000A", idNVGoc, "CB-00412", domain.StoredFileStored, mocTaoNV))
	k.addStoredFile(storedFileRow(string(xaThu), "01JTEPCU00000000000000000B", idNVGoc, maLanhDao, domain.StoredFileReady, mocTaoNV))
	k.addStoredFile(storedFileRow(string(xaThu), "01JTEPCU00000000000000000C", idNVGoc, maNguoiThucHien,
		domain.StoredFilePending, mocThaoTacNV.Add(-5*time.Minute)))
	// These do NOT count: rejected, failed, an abandoned pending row, another task's, another commune's.
	k.addStoredFile(storedFileRow(string(xaThu), "01JTEPCU00000000000000000D", idNVGoc, maNguoiThucHien, domain.StoredFileRejected, mocTaoNV))
	k.addStoredFile(storedFileRow(string(xaThu), "01JTEPCU00000000000000000E", idNVGoc, maNguoiThucHien, domain.StoredFileFailed, mocTaoNV))
	k.addStoredFile(storedFileRow(string(xaThu), "01JTEPCU00000000000000000F", idNVGoc, maNguoiThucHien,
		domain.StoredFilePending, mocThaoTacNV.Add(-time.Hour)))
	k.addStoredFile(storedFileRow(string(xaThu), "01JTEPCU00000000000000000G", idNVCon, maNguoiThucHien, domain.StoredFileStored, mocTaoNV))
	k.addStoredFile(storedFileRow("01JB"+strings.Repeat("B", 22), "01JTEPCU00000000000000000H", idNVGoc, maNguoiThucHien,
		domain.StoredFileStored, mocTaoNV))

	_, err := uc.RequestUpload(ctx, maNVGoc, uploadReq(), staffActor(maNguoiThucHien), false)
	if !errors.Is(err, ErrAttachmentCountReached) {
		t.Fatalf("lỗi = %v, muốn ErrAttachmentCountReached", err)
	}
	khongGhiGi(t, k)

	// The same state under a limit of four passes — so the refusal above was the count, and exactly
	// three rows were counted.
	k2 := khoNVMau()
	uc2, _, _, pol2, ctx2 := dungTaskAttachments(t, k2)
	pol2.p.MaxFilesPerSubject = 4
	for _, r := range k.storedFiles {
		k2.addStoredFile(r)
	}
	if _, err := uc2.RequestUpload(ctx2, maNVGoc, uploadReq(), staffActor(maNguoiThucHien), false); err != nil {
		t.Errorf("ba tệp sống, giới hạn bốn: %v", err)
	}
}

// --- c. completion ---------------------------------------------------------------------------------

// seedPending puts a pending row of the fixture task for the assignee, and (unless data is nil) the
// bytes the browser posted into the temp bucket under its upload key.
func seedPending(k *khoNhiemVuGia, obj *objectStoreFake, created time.Time, data []byte) map[string]any {
	r := storedFileRow(string(xaThu), fileIDThu, idNVGoc, maNguoiThucHien, domain.StoredFilePending, created)
	k.addStoredFile(r)
	if data != nil {
		obj.put(storage.BucketTemp, "upload/"+r["object_key"].(string), data)
	}
	return map[string]any{"key": r["object_key"], "upload": "upload/" + r["object_key"].(string)}
}

func TestComplete_StoresScannedHashedPromotedInOneTransaction(t *testing.T) {
	k := khoNVMau()
	uc, obj, sc, _, ctx := dungTaskAttachments(t, k)
	keys := seedPending(k, obj, mocThaoTacNV.Add(-2*time.Minute), pdfBytes)

	f, err := uc.Complete(ctx, maNVGoc, fileIDThu, staffActor(maNguoiThucHien), false)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if f.Status != domain.StoredFileStored || f.MIMEType != storage.MIMEPDF || f.SizeBytes != int64(len(pdfBytes)) ||
		len(f.SHA256) != 64 {
		t.Errorf("tệp = %+v", f)
	}
	if string(sc.scanned) != string(pdfBytes) {
		t.Error("máy quét không thấy đúng các byte đã được lưu")
	}
	if obj.has(storage.BucketTemp, keys["upload"].(string)) || !obj.has(storage.BucketPrivate, keys["key"].(string)) {
		t.Error("tệp không được chép sang bucket private đúng khoá đích")
	}

	ups := k.cau("UPDATE stored_file")
	if len(ups) != 2 || ups[0].args[2] != "pending" || ups[0].args[3] != "scanning" ||
		!strings.Contains(ups[1].sql, "status = 'stored'") {
		t.Fatalf("chuyển trạng thái = %+v, muốn pending → scanning rồi scanning → stored", ups)
	}
	if got := k.storedFile(fileIDThu); got["status"] != "stored" {
		t.Errorf("dòng sau hoàn tất = %v", got["status"])
	}
	chiGhiTrongGiaoDich(t, k)
	vet := vetKiemToan(t, k)
	if vet.args[4] != ActionTaskAttachmentStored || vet.args[5] != maNVGoc {
		t.Errorf("vết: %v / %v", vet.args[4], vet.args[5])
	}
	delta := auditDeltaText(t, k)
	for _, want := range []string{`"loai_tep":"application/pdf"`, `"sha256":"` + f.SHA256 + `"`, `"khoi_phuc_tu_dich":false`} {
		if !strings.Contains(delta, want) {
			t.Errorf("delta thiếu %s: %s", want, delta)
		}
	}
	if strings.Contains(delta, "Biên bản") {
		t.Errorf("tên tệp lọt vào vết: %s", delta)
	}
}

func TestComplete_RejectionsDeleteTempAndAudit(t *testing.T) {
	for _, c := range []struct {
		name   string
		data   []byte
		mod    func(*scannerFake, *policyFake)
		reason string
		sig    string
	}{
		// Declared PDF (the key says .pdf), the bytes are a PNG: the policy allows PNG, the key does not.
		{"kiểu dò khác kiểu khai báo", pngBytes, nil, RejectTypeMismatch, ""},
		{"kiểu dò không thuộc danh sách", htmlBytes, nil, RejectTypeNotAllowed, ""},
		{"kiểu dò bị chính sách hiện hành bỏ", pdfBytes, func(_ *scannerFake, p *policyFake) {
			p.p.AllowedMIMETypes = []string{storage.MIMEJPEG}
		}, RejectTypeNotAllowed, ""},
		{"vượt dung lượng của chính sách lúc hoàn tất", pdfBytes, func(_ *scannerFake, p *policyFake) {
			p.p.MaxBytes = 10
		}, RejectTooLarge, ""},
		{"nhiễm mã độc", pdfBytes, func(s *scannerFake, _ *policyFake) {
			s.res = malwarescan.Result{Clean: false, Signature: "Eicar-Test-Signature"}
		}, RejectMalware, "Eicar-Test-Signature"},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			uc, obj, sc, pol, ctx := dungTaskAttachments(t, k)
			if c.mod != nil {
				c.mod(sc, pol)
			}
			keys := seedPending(k, obj, mocThaoTacNV.Add(-2*time.Minute), c.data)

			_, err := uc.Complete(ctx, maNVGoc, fileIDThu, staffActor(maNguoiThucHien), false)
			var rej *AttachmentRejection
			if !errors.As(err, &rej) || rej.Reason != c.reason || !errors.Is(err, ErrAttachmentRejected) {
				t.Fatalf("lỗi = %v, muốn từ chối %q", err, c.reason)
			}
			if obj.has(storage.BucketTemp, keys["upload"].(string)) {
				t.Error("tệp bị từ chối vẫn nằm ở bucket temp")
			}
			if obj.has(storage.BucketPrivate, keys["key"].(string)) || len(obj.promoted) != 0 {
				t.Error("tệp bị từ chối đã được chép sang bucket private")
			}
			if got := k.storedFile(fileIDThu); got["status"] != "rejected" {
				t.Errorf("trạng thái = %v, muốn rejected", got["status"])
			}
			if k.coCau("status = 'stored'") {
				t.Error("tệp bị từ chối mà vẫn ghi stored")
			}
			chiGhiTrongGiaoDich(t, k)
			delta := auditDeltaText(t, k)
			if vetKiemToan(t, k).args[4] != ActionTaskAttachmentRejected ||
				!strings.Contains(delta, `"ly_do":"`+c.reason+`"`) || !strings.Contains(delta, `"da_xoa_tep_tam":true`) {
				t.Errorf("vết từ chối: %s", delta)
			}
			if c.sig != "" && !strings.Contains(delta, c.sig) {
				t.Errorf("vết không mang tên chữ ký mã độc: %s", delta)
			}
			if c.reason != RejectMalware && c.reason != RejectCountReached && sc.calls != 0 {
				t.Errorf("quét %d lần một tệp đã bị loại trước khi quét", sc.calls)
			}
		})
	}
}

func TestComplete_FailedTempDeleteIsRecordedNotHidden(t *testing.T) {
	k := khoNVMau()
	uc, obj, sc, _, ctx := dungTaskAttachments(t, k)
	sc.res = malwarescan.Result{Signature: "Win.Test.EICAR_HDB-1"}
	obj.purgeErr = errors.New("minio: connection reset")
	seedPending(k, obj, mocThaoTacNV.Add(-time.Minute), pdfBytes)

	_, err := uc.Complete(ctx, maNVGoc, fileIDThu, staffActor(maNguoiThucHien), false)
	if !errors.Is(err, ErrAttachmentRejected) {
		t.Fatalf("lỗi = %v", err)
	}
	if delta := auditDeltaText(t, k); !strings.Contains(delta, `"da_xoa_tep_tam":false`) {
		t.Errorf("vết giấu việc xoá tệp tạm thất bại: %s", delta)
	}
}

func TestComplete_RetryableFailuresWriteNothing(t *testing.T) {
	for _, c := range []struct {
		name string
		mod  func(*objectStoreFake, *scannerFake, *policyFake)
		want error
	}{
		{"máy quét không tới được", func(_ *objectStoreFake, s *scannerFake, _ *policyFake) {
			s.err = malwarescan.ErrUnavailable
		}, ErrScanUnavailable},
		{"máy quét vượt giới hạn kích thước", func(_ *objectStoreFake, s *scannerFake, _ *policyFake) {
			s.err = malwarescan.ErrTooLarge
		}, ErrScanUnavailable},
		{"tệp bị thay trong lúc kiểm (ETag)", func(o *objectStoreFake, _ *scannerFake, _ *policyFake) {
			o.replaceAfterStat = true
		}, ErrUploadChanged},
		{"nền tảng không trả lời", func(_ *objectStoreFake, _ *scannerFake, p *policyFake) {
			p.err = uploadpolicy.ErrUnavailable
		}, ErrUploadLimitsUnavailable},
		{"chính sách bị gỡ", func(_ *objectStoreFake, _ *scannerFake, p *policyFake) { p.ok = false },
			ErrUploadNotConfigured},
		{"kho lỗi", func(o *objectStoreFake, _ *scannerFake, _ *policyFake) {
			o.statErr = errors.New("minio: 500")
		}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			uc, obj, sc, pol, ctx := dungTaskAttachments(t, k)
			keys := seedPending(k, obj, mocThaoTacNV.Add(-2*time.Minute), pdfBytes)
			c.mod(obj, sc, pol)

			_, err := uc.Complete(ctx, maNVGoc, fileIDThu, staffActor(maNguoiThucHien), false)
			if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			khongGhiGi(t, k)
			if got := k.storedFile(fileIDThu); got["status"] != "pending" {
				t.Errorf("trạng thái = %v, muốn vẫn pending để thử lại", got["status"])
			}
			// NEVER STORED UNSCANNED (ADR 0052 §9).
			if obj.has(storage.BucketPrivate, keys["key"].(string)) {
				t.Error("tệp đã vào bucket private dù chưa quét xong")
			}
		})
	}
}

func TestComplete_MissingObjectNotYetThenExpired(t *testing.T) {
	// Inside the form's lifetime: nothing arrived yet — nothing written.
	k := khoNVMau()
	uc, obj, _, _, ctx := dungTaskAttachments(t, k)
	seedPending(k, obj, mocThaoTacNV.Add(-5*time.Minute), nil)
	if _, err := uc.Complete(ctx, maNVGoc, fileIDThu, staffActor(maNguoiThucHien), false); !errors.Is(err, ErrUploadNotReceived) {
		t.Fatalf("lỗi = %v, muốn ErrUploadNotReceived", err)
	}
	khongGhiGi(t, k)

	// Past it: the row moves to `failed`, with its trail.
	k = khoNVMau()
	uc, obj, _, _, ctx = dungTaskAttachments(t, k)
	seedPending(k, obj, mocThaoTacNV.Add(-20*time.Minute), nil)
	if _, err := uc.Complete(ctx, maNVGoc, fileIDThu, staffActor(maNguoiThucHien), false); !errors.Is(err, ErrUploadExpired) {
		t.Fatalf("lỗi = %v, muốn ErrUploadExpired", err)
	}
	if got := k.storedFile(fileIDThu); got["status"] != "failed" {
		t.Errorf("trạng thái = %v, muốn failed", got["status"])
	}
	chiGhiTrongGiaoDich(t, k)
	if vetKiemToan(t, k).args[4] != ActionTaskAttachmentExpired {
		t.Error("hết hạn mà không để vết")
	}
}

// A previous completion promoted the object and then failed before its transaction committed: the
// retry finds the temp copy gone and the destination written, and records THAT object.
func TestComplete_RecoversFromDestination(t *testing.T) {
	k := khoNVMau()
	uc, obj, sc, _, ctx := dungTaskAttachments(t, k)
	keys := seedPending(k, obj, mocThaoTacNV.Add(-2*time.Minute), nil)
	obj.put(storage.BucketPrivate, keys["key"].(string), pdfBytes)

	f, err := uc.Complete(ctx, maNVGoc, fileIDThu, staffActor(maNguoiThucHien), false)
	if err != nil || f.Status != domain.StoredFileStored {
		t.Fatalf("Complete = %+v, %v", f, err)
	}
	if sc.calls != 0 {
		t.Error("quét lại một tệp đã ở bucket private")
	}
	if !strings.Contains(auditDeltaText(t, k), `"khoi_phuc_tu_dich":true`) {
		t.Error("vết không nói tệp được khôi phục từ đích")
	}
}

func TestComplete_AlreadyStoredAnswersTheSame(t *testing.T) {
	k := khoNVMau()
	uc, obj, sc, _, ctx := dungTaskAttachments(t, k)
	k.addStoredFile(storedFileRow(string(xaThu), fileIDThu, idNVGoc, maNguoiThucHien, domain.StoredFileStored, mocTaoNV))

	f, err := uc.Complete(ctx, maNVGoc, fileIDThu, staffActor(maNguoiThucHien), false)
	if err != nil || f.Status != domain.StoredFileStored {
		t.Fatalf("Complete = %+v, %v", f, err)
	}
	khongGhiGi(t, k)
	if sc.calls != 0 || len(obj.promoted) != 0 {
		t.Error("hoàn tất lần hai chạy lại việc quét/chép")
	}
}

func TestComplete_OnlyTheUploaderOnThisTask(t *testing.T) {
	for _, c := range []struct {
		name  string
		row   map[string]any
		actor string
		ma    string
		want  error
	}{
		{"cán bộ khác (có quyền ghi nhật ký)", nil, "CB-00412", maNVGoc, ErrAttachmentNotFound},
		{"qua đường của nhiệm vụ khác", nil, maNguoiThucHien, maNVCon, ErrAttachmentNotFound},
		{"mã tệp không có", map[string]any{"id": "01JKHONGCO0000000000000000"}, maNguoiThucHien, maNVGoc, ErrAttachmentNotFound},
		{"xã khác", map[string]any{"tenant_id": "01JB" + strings.Repeat("B", 22)}, maNguoiThucHien, maNVGoc, ErrAttachmentNotFound},
		{"đã bị từ chối", map[string]any{"status": "rejected"}, maNguoiThucHien, maNVGoc, ErrAttachmentNotPending},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			k.themCon(idNVCon, maNVCon, idNVGoc, domain.DangThucHien)
			uc, obj, sc, _, ctx := dungTaskAttachments(t, k)
			r := storedFileRow(string(xaThu), fileIDThu, idNVGoc, maNguoiThucHien, domain.StoredFilePending, mocThaoTacNV)
			for col, v := range c.row {
				r[col] = v
			}
			k.addStoredFile(r)
			obj.put(storage.BucketTemp, "upload/"+r["object_key"].(string), pdfBytes)

			_, err := uc.Complete(ctx, c.ma, fileIDThu, staffActor(c.actor), true)
			if !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			khongGhiGi(t, k)
			if sc.calls != 0 {
				t.Error("đã quét tệp của người khác")
			}
		})
	}
}

// --- download --------------------------------------------------------------------------------------

func TestDownloadLink(t *testing.T) {
	for _, c := range []struct {
		name   string
		row    map[string]any
		linked bool
		reader string
		ma     string
		ok     bool
	}{
		{"tệp đã gắn, người đọc bất kỳ", nil, true, outsiderCode, maNVGoc, true},
		{"tệp chưa gắn, chính người tải", nil, false, maNguoiThucHien, maNVGoc, true},
		{"tệp chưa gắn, người khác", nil, false, outsiderCode, maNVGoc, false},
		{"xã khác", map[string]any{"tenant_id": "01JB" + strings.Repeat("B", 22)}, true, outsiderCode, maNVGoc, false},
		{"qua đường của nhiệm vụ khác", nil, true, outsiderCode, maNVCon, false},
		{"chưa quét xong", map[string]any{"status": "pending"}, false, maNguoiThucHien, maNVGoc, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			k.themCon(idNVCon, maNVCon, idNVGoc, domain.DangThucHien)
			uc, obj, _, _, ctx := dungTaskAttachments(t, k)
			r := storedFileRow(string(xaThu), fileIDThu, idNVGoc, maNguoiThucHien, domain.StoredFileStored, mocTaoNV)
			for col, v := range c.row {
				r[col] = v
			}
			k.addStoredFile(r)
			if c.linked {
				k.fileLinks = map[string]string{fileIDThu: "nk-1"}
			}

			d, err := uc.DownloadLink(ctx, c.ma, fileIDThu, staffActor(c.reader))
			if !c.ok {
				if !errors.Is(err, ErrAttachmentNotFound) {
					t.Fatalf("lỗi = %v, muốn ErrAttachmentNotFound", err)
				}
				if len(obj.downloads) != 0 {
					t.Error("đã ký liên kết tải về dù bị từ chối")
				}
				return
			}
			if err != nil || d.URL == "" {
				t.Fatalf("DownloadLink = %+v, %v", d, err)
			}
			call := obj.downloads[0]
			if call.key != r["object_key"] || call.ttl != storage.MaxDownloadTTL || call.filename != "Biên bản nghiệm thu.pdf" {
				t.Errorf("lượt ký = %+v", call)
			}
			if !d.ExpiresAt.Equal(mocThaoTacNV.Add(storage.MaxDownloadTTL)) {
				t.Errorf("hết hạn = %v", d.ExpiresAt)
			}
			khongGhiGi(t, k)
		})
	}
}

func TestDownloadLink_StorageNotConfigured(t *testing.T) {
	k := khoNVMau()
	uc, _, _, _, ctx := dungTaskAttachments(t, k)
	uc.objects = nil
	if _, err := uc.DownloadLink(ctx, maNVGoc, fileIDThu, staffActor(maNguoiThucHien)); !errors.Is(err, ErrUploadNotConfigured) {
		t.Errorf("lỗi = %v, muốn ErrUploadNotConfigured", err)
	}
}
