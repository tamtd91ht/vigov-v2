package app

// The broadcast audio acts (content_audio.go) and the audio half of Sua, over the REAL content store on
// the fake driver (transaction boundaries, the item UPDATE and the audit INSERT are real statements
// there) and the in-memory file and object fakes of content_cover_test.go.
//
// WHAT IT DOES NOT PROVE: MinIO, clamd, PostgreSQL — 0012's audio trigger and CHECKs, stored_file_guard.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// --- fixtures: real file shapes ----------------------------------------------------------------------

func isoBox(typ string, body ...[]byte) []byte {
	b := bytes.Join(body, nil)
	out := make([]byte, 8, 8+len(b))
	binary.BigEndian.PutUint32(out, uint32(8+len(b)))
	copy(out[4:], typ)
	return append(out, b...)
}

func isoTrack(handler string) []byte {
	hdlr := append(make([]byte, 8), handler...)
	hdlr = append(hdlr, make([]byte, 13)...)
	return isoBox("trak", isoBox("tkhd", make([]byte, 84)), isoBox("mdia", isoBox("mdhd", make([]byte, 24)),
		isoBox("hdlr", hdlr)))
}

// isoFile is ftyp(major, compatible…) + moov(tracks) + mdat.
func isoFile(major string, compatible []string, handlers ...string) []byte {
	ftyp := append([]byte(major), 0, 0, 0, 0)
	for _, c := range compatible {
		ftyp = append(ftyp, c...)
	}
	moov := [][]byte{isoBox("mvhd", make([]byte, 100))}
	for _, h := range handlers {
		moov = append(moov, isoTrack(h))
	}
	return bytes.Join([][]byte{isoBox("ftyp", ftyp), isoBox("moov", moov...), isoBox("mdat", make([]byte, 2048))}, nil)
}

// testMP3 is an ID3v2.3 tag then two MPEG-1 Layer III frames (128 kbit/s, 44.1 kHz: 417 bytes each).
func testMP3() []byte {
	b := append([]byte("ID3"), 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x14)
	b = append(b, make([]byte, 20)...)
	for i := 0; i < 2; i++ {
		f := make([]byte, 417)
		copy(f, []byte{0xFF, 0xFB, 0x90, 0x00})
		b = append(b, f...)
	}
	return b
}

func testM4A() []byte { return isoFile("M4A ", []string{"M4A ", "isom", "iso2"}, "soun") }

// --- fakes -------------------------------------------------------------------------------------------

// fakeAudioFiles is the cover file fake plus the audio-only calls and a settable count.
type fakeAudioFiles struct {
	*fakeCoverFiles
	liveCount, storedCount int
	softDeleted            []string // "id|by|reason"
}

func (f *fakeAudioFiles) CountForSubjectTx(context.Context, *store.ScopedTx, string, string, time.Time) (int, error) {
	return f.liveCount, nil
}
func (f *fakeAudioFiles) CountForSubject(context.Context, string, string) (int, error) {
	return f.storedCount, nil
}
func (f *fakeAudioFiles) ReadyObjectKeys(_ context.Context, purpose string, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range ids {
		if r := f.rows[id]; r != nil && r.Status == domain.StoredFileReady && r.Purpose == purpose {
			out[id] = r.ObjectKey
		}
	}
	return out, nil
}
func (f *fakeAudioFiles) SoftDelete(_ context.Context, _ *store.ScopedTx, id, by, reason string, _ time.Time) error {
	f.softDeleted = append(f.softDeleted, id+"|"+by+"|"+reason)
	delete(f.rows, id)
	return nil
}

// signingObjects records what PresignDownload was asked to sign.
type signingObjects struct {
	*fakeCoverObjects
	ttls      []time.Duration
	filenames []string
}

func (o *signingObjects) PresignDownload(ctx context.Context, b storage.Bucket, key string, ttl time.Duration,
	filename string) (storage.PresignedURL, error) {
	o.ttls, o.filenames = append(o.ttls, ttl), append(o.filenames, filename)
	return o.fakeCoverObjects.PresignDownload(ctx, b, key, ttl, filename)
}

// --- harness -----------------------------------------------------------------------------------------

const (
	audioFileID = "01JAAAAAAAAAAAAAAAAAAAAAAB"
	audioItemID = "01JTTTTTTTTTTTTTTTTTTTTTTT"
)

func audioPolicy() fakePolicies {
	return fakePolicies{ok: true, p: uploadpolicy.Policy{Purpose: storage.PurposeContentAudio, MaxBytes: 31457280,
		AllowedMIMETypes: []string{storage.MIMEMP3, storage.MIMEM4A}, FileCountLimited: true, MaxFilesPerSubject: 1}}
}

func broadcastItem() *domain.NoiDungMiniApp {
	return &domain.NoiDungMiniApp{ID: audioItemID, Loai: domain.LoaiTruyenThanh, TieuDe: "Bản tin sáng",
		NgayDang: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), TrangThai: domain.TrangThaiDangHien,
		Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND, TaoLuc: coverClock, CapNhatLuc: coverClock}
}

type audioRig struct {
	k       *khoNDGia
	uc      *ContentAudio
	files   *fakeAudioFiles
	objects *signingObjects
	scanner *fakeScanner
	ctx     context.Context
}

func newAudioRig(t *testing.T) *audioRig {
	t.Helper()
	k := &khoNDGia{dongHienCo: broadcastItem()}
	db := newFakeDB(t, k)
	files := &fakeAudioFiles{fakeCoverFiles: newFakeCoverFiles()}
	objects := &signingObjects{fakeCoverObjects: newFakeCoverObjects()}
	scanner := &fakeScanner{res: malwarescan.Result{Clean: true}}
	uc := NewContentAudio(db, commsstore.NewNoiDungMiniAppStore(db), files, objects, scanner, audioPolicy())
	uc.newID = func() (string, error) { return audioFileID, nil }
	uc.now = func() time.Time { return coverClock }
	return &audioRig{k: k, uc: uc, files: files, objects: objects, scanner: scanner,
		ctx: tenant.Into(context.Background(), xaA)}
}

// audioReq is one audio upload of data declared as mime, for the rig's broadcast, with a duration.
func audioReq(mime string, data []byte, seconds int) AudioUploadRequest {
	file, finish := streamOf(data, nil)
	return AudioUploadRequest{ContentItemID: audioItemID, FileName: "ban-tin-ong-nguyen-van-a.mp3",
		ContentType: mime, Size: int64(len(data)), DurationSeconds: seconds, File: file, Finish: finish}
}

// declared gives a declaration-only request a zero-filled stream of its size and a valid duration.
func declared(req AudioUploadRequest) AudioUploadRequest {
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 60
	}
	n := req.Size
	if n < 0 || n > 1<<20 {
		n = 0
	}
	req.File, req.Finish = streamOf(make([]byte, n), nil)
	return req
}

func (r *audioRig) upload(mime string, data []byte, seconds int) (AudioCompletion, error) {
	return r.uc.Upload(r.ctx, audioReq(mime, data, seconds), nguoiSoanND())
}

func (r *audioRig) itemUpdates() []lenhGhi { return r.k.cau("UPDATE noi_dung_mini_app") }

// --- a. admission ------------------------------------------------------------------------------------

func TestAudioUploadRefusesBeforeAnyTransaction(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  AudioUploadRequest
		want error
	}{
		{"no item", AudioUploadRequest{FileName: "a.mp3", ContentType: storage.MIMEMP3, Size: 10}, ErrAudioItemRequired},
		{"over 30 MiB", AudioUploadRequest{ContentItemID: audioItemID, FileName: "a.mp3", ContentType: storage.MIMEMP3,
			Size: 31457281}, ErrAudioTooLarge},
		{"declared video", AudioUploadRequest{ContentItemID: audioItemID, FileName: "a.mp4", ContentType: storage.MIMEMP4,
			Size: 10}, ErrAudioTypeNotAllowed},
		{"declared wav", AudioUploadRequest{ContentItemID: audioItemID, FileName: "a.wav", ContentType: "audio/wav",
			Size: 10}, ErrAudioTypeNotAllowed},
		{"declared x-m4a spelling", AudioUploadRequest{ContentItemID: audioItemID, FileName: "a.m4a",
			ContentType: "audio/x-m4a", Size: 10}, ErrAudioTypeNotAllowed},
		{"no size", AudioUploadRequest{ContentItemID: audioItemID, FileName: "a.mp3", ContentType: storage.MIMEMP3},
			domain.ErrCoverSizeInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newAudioRig(t)
			if _, err := r.uc.Upload(r.ctx, declared(tc.req), nguoiSoanND()); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if r.k.batDau != 0 || len(r.files.inserted) != 0 || r.objects.puts != 0 {
				t.Errorf("a refused declaration opened a transaction (%d), wrote a row (%d) or read the file (%d)",
					r.k.batDau, len(r.files.inserted), r.objects.puts)
			}
		})
	}
}

// THE DURATION IS CHECKED FIRST: a wrong figure refuses before a row, a transaction or a byte.
func TestAudioUploadDurationBounds(t *testing.T) {
	for _, d := range []int{0, -1, 21601} {
		r := newAudioRig(t)
		if _, err := r.upload(storage.MIMEMP3, testMP3(), d); !errors.Is(err, domain.ErrAudioDurationInvalid) {
			t.Errorf("duration %d: err = %v", d, err)
		}
		if r.k.batDau != 0 || r.objects.puts != 0 || r.scanner.scanned != 0 {
			t.Errorf("duration %d: refused AFTER work began", d)
		}
	}
	for _, d := range []int{1, 21600} {
		r := newAudioRig(t)
		if _, err := r.upload(storage.MIMEMP3, testMP3(), d); err != nil {
			t.Errorf("duration %d (a bound) refused: %v", d, err)
		}
	}
}

func TestAudioUploadFailsClosedWithoutPolicyOrStorage(t *testing.T) {
	r := newAudioRig(t)
	req := declared(AudioUploadRequest{ContentItemID: audioItemID, FileName: "a.mp3", ContentType: storage.MIMEMP3, Size: 10})
	r.uc.policies = fakePolicies{ok: false}
	if _, err := r.uc.Upload(r.ctx, req, nguoiSoanND()); !errors.Is(err, ErrAudioUploadNotConfigured) {
		t.Errorf("no policy: err = %v", err)
	}
	r.uc.policies = fakePolicies{err: uploadpolicy.ErrUnavailable}
	if _, err := r.uc.Upload(r.ctx, req, nguoiSoanND()); !errors.Is(err, ErrAudioLimitsUnavailable) {
		t.Errorf("platform down: err = %v", err)
	}
	if _, err := r.uc.UploadLimit(r.ctx); !errors.Is(err, ErrAudioLimitsUnavailable) {
		t.Errorf("platform down, limit: err = %v", err)
	}
	r.uc.objects = nil
	if _, err := r.uc.Upload(r.ctx, req, nguoiSoanND()); !errors.Is(err, ErrAudioUploadNotConfigured) {
		t.Errorf("no object store: err = %v", err)
	}
	if _, err := r.uc.UploadLimit(r.ctx); !errors.Is(err, ErrAudioUploadNotConfigured) {
		t.Errorf("no object store, limit: err = %v", err)
	}
}

func TestAudioUploadLimitIsThePolicys(t *testing.T) {
	r := newAudioRig(t)
	if n, err := r.uc.UploadLimit(r.ctx); err != nil || n != 31457280 {
		t.Fatalf("limit = %d, %v; want the policy's 30 MiB", n, err)
	}
}

func TestAudioUploadRefusesAnItemThatIsNotABroadcast(t *testing.T) {
	r := newAudioRig(t)
	r.k.dongHienCo.Loai = domain.LoaiTinTuc
	_, err := r.upload(storage.MIMEMP3, testMP3(), 60)
	if !errors.Is(err, domain.ErrAudioOnlyForBroadcast) {
		t.Fatalf("err = %v", err)
	}
	if len(r.files.inserted) != 0 || r.k.daRollback != 1 || r.k.coCau("INSERT INTO audit_log") || r.objects.puts != 0 {
		t.Errorf("inserted=%d rollback=%d puts=%d", len(r.files.inserted), r.k.daRollback, r.objects.puts)
	}
}

func TestAudioUploadForMissingItemIs404(t *testing.T) {
	r := newAudioRig(t)
	r.k.dongHienCo = nil
	req := audioReq(storage.MIMEMP3, testMP3(), 60)
	req.ContentItemID = "nd-khac"
	_, err := r.uc.Upload(r.ctx, req, nguoiSoanND())
	if !errors.Is(err, commsstore.ErrNoiDungKhongTonTai) || len(r.files.inserted) != 0 {
		t.Fatalf("err = %v inserted = %d", err, len(r.files.inserted))
	}
}

// ONE FILE PER ITEM (platform 0013): a second upload while one is live — or in flight — is refused.
func TestAudioUploadRefusesASecondLiveFile(t *testing.T) {
	r := newAudioRig(t)
	r.files.liveCount = 1
	_, err := r.upload(storage.MIMEMP3, testMP3(), 60)
	if !errors.Is(err, ErrAudioCountReached) || len(r.files.inserted) != 0 {
		t.Fatalf("err = %v inserted = %d", err, len(r.files.inserted))
	}
}

// --- b. the stream -----------------------------------------------------------------------------------

// A FAILED STREAM FREES THE ITEM'S ONE SLOT: the row goes to `failed` with the abandoned entry — left
// `pending`, it would answer the officer's retry with 409 `audio_limit` for UploadTTL.
func TestAudioStreamFailureMarksTheRowFailedAndAttachesNothing(t *testing.T) {
	r := newAudioRig(t)
	req := audioReq(storage.MIMEMP3, testMP3(), 60)
	req.File = readerFunc(func([]byte) (int, error) { return 0, errors.New("client hung up") })
	_, err := r.uc.Upload(r.ctx, req, nguoiSoanND())
	if !errors.Is(err, ErrAudioUploadIncomplete) {
		t.Fatalf("err = %v", err)
	}
	if st := r.files.get(audioFileID).Status; st != domain.StoredFileFailed {
		t.Errorf("status = %s, want failed", st)
	}
	if len(r.itemUpdates()) != 0 || r.scanner.scanned != 0 {
		t.Error("an incomplete upload was attached or scanned")
	}
	audits := r.k.cau("INSERT INTO audit_log")
	if len(audits) != 2 || !auditCarries(audits[1], CoverAbandonNotReceived) || !containsString(auditActions(r.k), ActionAudioExpired) {
		t.Errorf("trail = %v", auditActions(r.k))
	}
}

func TestAudioUploadWritesRowAndTrailBeforeTheBytes(t *testing.T) {
	r := newAudioRig(t)
	r.objects.putErr = errors.New("storage: put upload: unreachable")
	if _, err := r.upload(storage.MIMEMP3, testMP3(), 60); !errors.Is(err, ErrAudioUploadIncomplete) {
		t.Fatalf("err = %v", err)
	}
	if len(r.files.inserted) != 1 {
		t.Fatalf("inserted = %d", len(r.files.inserted))
	}
	f := r.files.inserted[0]
	if f.SubjectID != audioItemID || f.UploadedBy != maCanBoSoanND || f.Purpose != "content-audio" ||
		f.Bucket != domain.StoredFileBucketPrivate || f.RetentionClass != "content-source" || f.Status != domain.StoredFilePending {
		t.Errorf("row = %+v", f)
	}
	want := "content-source/t_" + strings.ToLower(string(xaA)) + "/2026/10/comms/content-audio/" +
		strings.ToLower(audioFileID) + "/original.mp3"
	if f.ObjectKey != want {
		t.Errorf("object key = %q, want %q", f.ObjectKey, want)
	}
	if r.objects.putMax != 31457280 {
		t.Errorf("write limit = %d, want the POLICY's 30 MiB", r.objects.putMax)
	}
	if !containsString(auditActions(r.k), ActionAudioUploadRequested) {
		t.Errorf("trail = %v", auditActions(r.k))
	}
	for _, l := range r.k.cau("INSERT INTO audit_log") {
		for _, a := range l.args {
			if s, ok := a.(string); ok && strings.Contains(s, "nguyen-van-a") {
				t.Errorf("the trail carries the file name: %s", s)
			}
		}
	}
}

// --- c. the completion -------------------------------------------------------------------------------

func TestAudioUploadMP3IsReadyAttachedAndAuditedInOneTransaction(t *testing.T) {
	r := newAudioRig(t)
	got, err := r.upload(storage.MIMEMP3, testMP3(), 754)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if got.File.Status != domain.StoredFileReady || got.File.MIMEType != storage.MIMEMP3 ||
		got.Item.AudioFileID != got.File.ID || got.Item.AudioDurationSeconds != 754 {
		t.Fatalf("completion = %+v", got)
	}
	want := []string{"pending>scanning", "scanning>stored", "stored>processing", "processing>ready"}
	if strings.Join(r.files.transitions, ",") != strings.Join(want, ",") {
		t.Errorf("transitions = %v, want %v", r.files.transitions, want)
	}
	if r.scanner.scanned != 1 || r.objects.promoted != 1 {
		t.Errorf("scanned=%d promoted=%d", r.scanner.scanned, r.objects.promoted)
	}
	// NO DERIVATIVE AND NOTHING PUBLIC (ADR 0067 §4.2).
	if len(r.objects.produced) != 0 || len(r.objects.published) != 0 || len(r.files.publicSet) != 0 {
		t.Errorf("produced=%v published=%v publicSet=%v", r.objects.produced, r.objects.published, r.files.publicSet)
	}
	// The item row and the stored entry: ONE transaction (the second; the first is the pending row).
	ups := r.itemUpdates()
	if len(ups) != 1 || ups[0].args[18] != got.File.ID || ups[0].args[19] != int64(754) {
		t.Fatalf("item update = %+v", ups)
	}
	if r.k.batDau != 2 || r.k.daCommit != 2 || len(r.k.cau("INSERT INTO audit_log")) != 2 {
		t.Errorf("tx: begin=%d commit=%d audit=%d", r.k.batDau, r.k.daCommit, len(r.k.cau("INSERT INTO audit_log")))
	}
	entry := r.k.cau("INSERT INTO audit_log")[1]
	var sawAction, sawActor bool
	for _, a := range entry.args {
		if a == ActionAudioStored {
			sawAction = true
		}
		if a == maCanBoSoanND {
			sawActor = true
		}
	}
	if !sawAction || !sawActor {
		t.Errorf("audit entry args = %v (want action %q and actor %q)", entry.args, ActionAudioStored, maCanBoSoanND)
	}
}

func TestAudioUploadM4AIsReady(t *testing.T) {
	r := newAudioRig(t)
	got, err := r.upload(storage.MIMEM4A, testM4A(), 60)
	if err != nil || got.File.MIMEType != storage.MIMEM4A || got.File.Status != domain.StoredFileReady {
		t.Fatalf("Upload = %+v, %v", got, err)
	}
}

// The file outcomes: each is a rejected row and a trail entry, the temp object purged, NOTHING promoted,
// NO item update.
func TestAudioUploadRejectsWhatIsNotAllowedAudio(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declared string
		data     []byte
		reason   string
	}{
		{"renamed mp4 video", storage.MIMEM4A, isoFile("isom", []string{"isom", "avc1", "mp41"}, "vide", "soun"),
			AudioRejectTypeNotAllowed},
		{"video track behind an M4A brand", storage.MIMEM4A, isoFile("M4A ", []string{"M4A "}, "soun", "vide"),
			AudioRejectNotAudio},
		{"wav", storage.MIMEMP3, append([]byte("RIFF\x24\x00\x00\x00WAVEfmt "), make([]byte, 64)...),
			AudioRejectTypeNotAllowed},
		{"ogg", storage.MIMEMP3, append([]byte("OggS"), make([]byte, 64)...), AudioRejectTypeNotAllowed},
		{"m4a declared as mp3", storage.MIMEMP3, testM4A(), AudioRejectTypeMismatch},
		{"id3 then video", storage.MIMEMP3,
			append(append([]byte("ID3"), 0x03, 0, 0, 0, 0, 0, 0x04, 0, 0, 0, 0), isoFile("isom", nil, "vide")...),
			AudioRejectNotAudio},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newAudioRig(t)
			_, err := r.upload(tc.declared, tc.data, 60)
			var rej *AudioRejection
			if !errors.As(err, &rej) || rej.Reason != tc.reason {
				t.Fatalf("err = %v, want rejection %q", err, tc.reason)
			}
			if r.objects.promoted != 0 || len(r.itemUpdates()) != 0 {
				t.Errorf("promoted=%d item updates=%d", r.objects.promoted, len(r.itemUpdates()))
			}
			if r.files.get(audioFileID).Status != domain.StoredFileRejected || len(r.objects.purged) != 1 {
				t.Errorf("status=%s purged=%v", r.files.get(audioFileID).Status, r.objects.purged)
			}
			acts := auditActions(r.k)
			if !containsString(acts, ActionAudioRejected) || containsString(acts, ActionAudioExpired) {
				t.Errorf("a rejection leaves the rejected entry and no abandoned one: %v", acts)
			}
		})
	}
}

func TestAudioUploadInfectedIsRejectedNeverStored(t *testing.T) {
	r := newAudioRig(t)
	r.scanner.res = malwarescan.Result{Clean: false, Signature: "Eicar-Test-Signature"}
	_, err := r.upload(storage.MIMEMP3, testMP3(), 60)
	var rej *AudioRejection
	if !errors.As(err, &rej) || rej.Reason != AudioRejectMalware {
		t.Fatalf("err = %v", err)
	}
	if r.objects.promoted != 0 || len(r.itemUpdates()) != 0 || r.files.get(audioFileID).Status != domain.StoredFileRejected {
		t.Errorf("an infected file went further than rejection")
	}
}

// UNSCANNABLE IS NEVER CLEAN — and the row is abandoned, not left holding the item's one slot.
func TestAudioUploadScannerDownStoresNothingAndFreesTheSlot(t *testing.T) {
	r := newAudioRig(t)
	r.scanner.err = errors.New("clamd unreachable")
	if _, err := r.upload(storage.MIMEMP3, testMP3(), 60); !errors.Is(err, ErrAudioScanUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if r.objects.promoted != 0 || len(r.itemUpdates()) != 0 || r.files.get(audioFileID).Status != domain.StoredFileFailed {
		t.Error("unscannable must store nothing, attach nothing, and leave the row failed (ADR 0052 §9)")
	}
	if !auditCarries(r.k.cau("INSERT INTO audit_log")[1], CoverAbandonNotInspected) {
		t.Errorf("trail = %v", auditActions(r.k))
	}
}

func TestAudioCompletionOverThePolicyIsTooLarge(t *testing.T) {
	r := newAudioRig(t)
	tight := fakePolicies{ok: true, p: uploadpolicy.Policy{Purpose: storage.PurposeContentAudio, MaxBytes: 100,
		AllowedMIMETypes: []string{storage.MIMEMP3}}}
	r.uc.policies = &seqPolicies{ps: []fakePolicies{audioPolicy(), tight}}
	_, err := r.upload(storage.MIMEMP3, testMP3(), 60)
	var rej *AudioRejection
	if !errors.As(err, &rej) || rej.Reason != AudioRejectTooLarge {
		t.Fatalf("err = %v", err)
	}
}

// The item changed type between the admission and the completion: nothing is attached, the row abandoned.
func TestAudioCompletionRefusesWhenTheItemIsNoLongerABroadcast(t *testing.T) {
	r := newAudioRig(t)
	r.scanner.onScan = func() { r.k.dongHienCo.Loai = domain.LoaiTinTuc }
	if _, err := r.upload(storage.MIMEMP3, testMP3(), 60); !errors.Is(err, domain.ErrAudioOnlyForBroadcast) {
		t.Fatalf("err = %v", err)
	}
	if len(r.itemUpdates()) != 0 || containsString(auditActions(r.k), ActionAudioStored) {
		t.Errorf("a refused completion attached something")
	}
	if st := r.files.get(audioFileID).Status; st != domain.StoredFileFailed {
		t.Errorf("status = %s, want failed", st)
	}
}

// The completion re-reads the row: another officer's upload answers 404, as it did as a route.
func TestAudioCompletionByAnotherOfficerIsNotFound(t *testing.T) {
	r := newAudioRig(t)
	r.files.rows[audioFileID] = &domain.StoredFile{ID: audioFileID, Purpose: string(audioPurpose),
		SubjectType: domain.StoredFileSubjectContentItem, SubjectID: audioItemID, Status: domain.StoredFilePending,
		UploadedBy: maCanBoSoanND}
	other := nguoiSoanND()
	other.ID = "CB-2026-KHAC00"
	if _, err := r.uc.complete(r.ctx, audioFileID, 60, other); !errors.Is(err, ErrAudioFileNotFound) {
		t.Fatalf("err = %v", err)
	}
}

// --- staff view, public links ------------------------------------------------------------------------

func TestAudioPublicURLsSignOnlyReadyFilesWithNoNameAndABoundedTTL(t *testing.T) {
	r := newAudioRig(t)
	r.files.rows["READY"] = &domain.StoredFile{ID: "READY", Purpose: "content-audio", Status: domain.StoredFileReady,
		ObjectKey: "content-source/t_x/2026/10/comms/content-audio/ready/original.mp3", OriginalName: "ong-nguyen-van-a.mp3"}
	r.files.rows["PENDING"] = &domain.StoredFile{ID: "PENDING", Purpose: "content-audio", Status: domain.StoredFilePending}
	r.files.rows["COVER"] = &domain.StoredFile{ID: "COVER", Purpose: "content-image", Status: domain.StoredFileReady}

	got, err := r.uc.PublicAudioURLs(r.ctx, []string{"READY", "PENDING", "COVER", "NONE"})
	if err != nil {
		t.Fatalf("PublicAudioURLs: %v", err)
	}
	if len(got) != 1 || got["READY"].URL == "" {
		t.Fatalf("signed = %v", got)
	}
	if !got["READY"].ExpiresAt.Equal(coverClock.Add(PublicAudioURLTTL)) {
		t.Errorf("expires at %v", got["READY"].ExpiresAt)
	}
	if PublicAudioURLTTL > storage.MaxDownloadTTL || PublicAudioURLTTL <= 0 {
		t.Errorf("PublicAudioURLTTL %v outside (0, %v]", PublicAudioURLTTL, storage.MaxDownloadTTL)
	}
	if len(r.objects.ttls) != 1 || r.objects.ttls[0] != PublicAudioURLTTL {
		t.Errorf("signed with ttl %v", r.objects.ttls)
	}
	// RULE 3: the officer's file name never reaches a resident's URL.
	if r.objects.filenames[0] != "" {
		t.Errorf("public link signed with a file name: %q", r.objects.filenames[0])
	}
	if strings.Contains(got["READY"].URL.URL(), "original.mp3") == false {
		t.Errorf("the link is not of the PRIVATE original: %s", got["READY"].URL.URL())
	}
}

func TestAudioPublicURLsEmptyWhenStorageNotConfigured(t *testing.T) {
	r := newAudioRig(t)
	r.uc.objects = nil
	got, err := r.uc.PublicAudioURLs(r.ctx, []string{"READY"})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestAudioViewSignsThePrivateOriginalForStaff(t *testing.T) {
	r := newAudioRig(t)
	r.files.rows["READY"] = &domain.StoredFile{ID: "READY", Purpose: "content-audio", Status: domain.StoredFileReady,
		MIMEType: storage.MIMEM4A, SizeBytes: 5000,
		ObjectKey: "content-source/t_x/2026/10/comms/content-audio/ready/original.m4a", OriginalName: "ban-tin.m4a"}
	v, err := r.uc.View(r.ctx, "READY")
	if err != nil || v.PreviewURL == "" || v.MIMEType != storage.MIMEM4A || v.SizeBytes != 5000 {
		t.Fatalf("view = %+v, %v", v, err)
	}
	if r.objects.ttls[0] > storage.MaxDownloadTTL {
		t.Errorf("staff preview ttl %v", r.objects.ttls[0])
	}
}

// --- the audio half of Sua ---------------------------------------------------------------------------

func newAudioEditRig(t *testing.T, item *domain.NoiDungMiniApp) (*SoanNoiDungMiniApp, *khoNDGia, *fakeAudioFiles, context.Context) {
	t.Helper()
	k := &khoNDGia{dongHienCo: item, coDong: true}
	db := newFakeDB(t, k)
	files := &fakeAudioFiles{fakeCoverFiles: newFakeCoverFiles()}
	uc := NewSoanNoiDungMiniApp(db, commsstore.NewNoiDungMiniAppStore(db), commsstore.NewDanhMucMiniAppStore(db)).
		WithAudioFiles(files)
	uc.bayGio = func() time.Time { return coverClock }
	return uc, k, files, tenant.Into(context.Background(), xaA)
}

func withAudio() *domain.NoiDungMiniApp {
	n := broadcastItem()
	n.AudioFileID, n.AudioDurationSeconds = audioFileID, 754
	return n
}

func ptr[T any](v T) *T { return &v }

func TestEditRemovingAudioClearsBothRetiresTheFileAndAuditsIt(t *testing.T) {
	uc, k, files, ctx := newAudioEditRig(t, withAudio())
	sau, err := uc.Sua(ctx, audioItemID, domain.YeuCauSuaNoiDung{AudioFileID: ptr("")}, nguoiSoanND())
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if sau.AudioFileID != "" || sau.AudioDurationSeconds != 0 {
		t.Errorf("after = %q / %d", sau.AudioFileID, sau.AudioDurationSeconds)
	}
	ups := k.cau("UPDATE noi_dung_mini_app")
	if len(ups) != 1 || ups[0].args[18] != nil || ups[0].args[19] != nil {
		t.Fatalf("update = %+v", ups)
	}
	if len(files.softDeleted) != 1 || files.softDeleted[0] != audioFileID+"|"+maCanBoSoanND+"|"+audioRetireReason {
		t.Errorf("retired = %v", files.softDeleted)
	}
	audits := k.cau("INSERT INTO audit_log")
	if len(audits) != 1 || k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("tx begin=%d commit=%d audits=%d", k.batDau, k.daCommit, len(audits))
	}
	if !auditCarries(audits[0], `"audio_file_retired":"`+audioFileID+`"`) ||
		!auditCarries(audits[0], `"audio_duration_seconds":754`) {
		t.Errorf("delta lacks the audio change: %v", audits[0].args)
	}
}

func TestEditTypeChangeAwayFromBroadcastRetiresTheAudio(t *testing.T) {
	uc, k, files, ctx := newAudioEditRig(t, withAudio())
	if _, err := uc.Sua(ctx, audioItemID, domain.YeuCauSuaNoiDung{Loai: ptr("tin-tuc")}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	ups := k.cau("UPDATE noi_dung_mini_app")
	if len(ups) != 1 || ups[0].args[18] != nil || ups[0].args[19] != nil || len(files.softDeleted) != 1 {
		t.Fatalf("update = %+v retired = %v", ups, files.softDeleted)
	}
}

func TestEditCorrectsTheDurationOnly(t *testing.T) {
	uc, k, files, ctx := newAudioEditRig(t, withAudio())
	if _, err := uc.Sua(ctx, audioItemID, domain.YeuCauSuaNoiDung{AudioDurationSeconds: ptr(300)}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	ups := k.cau("UPDATE noi_dung_mini_app")
	if len(ups) != 1 || ups[0].args[18] != audioFileID || ups[0].args[19] != int64(300) || len(files.softDeleted) != 0 {
		t.Fatalf("update = %+v retired = %v", ups, files.softDeleted)
	}
}

func TestEditAudioRefusalsWriteNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		item *domain.NoiDungMiniApp
		req  domain.YeuCauSuaNoiDung
		want error
	}{
		{"duration with no file", broadcastItem(), domain.YeuCauSuaNoiDung{AudioDurationSeconds: ptr(60)}, domain.ErrAudioAllOrNone},
		{"another file id", withAudio(), domain.YeuCauSuaNoiDung{AudioFileID: ptr("01JOTHERFILE00000000000000")}, domain.ErrAudioNotUsable},
		{"duration on news", func() *domain.NoiDungMiniApp { n := broadcastItem(); n.Loai = domain.LoaiTinTuc; return n }(),
			domain.YeuCauSuaNoiDung{AudioDurationSeconds: ptr(60)}, domain.ErrAudioOnlyForBroadcast},
		{"duration out of range", withAudio(), domain.YeuCauSuaNoiDung{AudioDurationSeconds: ptr(21601)}, domain.ErrAudioDurationInvalid},
		{"remove and type a duration", withAudio(), domain.YeuCauSuaNoiDung{AudioFileID: ptr(""), AudioDurationSeconds: ptr(60)},
			domain.ErrAudioAllOrNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, k, files, ctx := newAudioEditRig(t, tc.item)
			if _, err := uc.Sua(ctx, audioItemID, tc.req, nguoiSoanND()); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if k.coCau("UPDATE noi_dung_mini_app") || k.coCau("INSERT INTO audit_log") || len(files.softDeleted) != 0 {
				t.Error("a refused edit wrote something")
			}
		})
	}
}

func TestEditRemovingAudioWithoutAFileStoreIsRefused(t *testing.T) {
	uc, k, _, ctx := newAudioEditRig(t, withAudio())
	uc.audioFiles = nil
	if _, err := uc.Sua(ctx, audioItemID, domain.YeuCauSuaNoiDung{AudioFileID: ptr("")}, nguoiSoanND()); !errors.Is(err, ErrAudioUploadNotConfigured) {
		t.Fatalf("err = %v", err)
	}
	if k.coCau("UPDATE noi_dung_mini_app") {
		t.Error("the slot would stay held: the edit must not be written")
	}
}

func auditCarries(l lenhGhi, fragment string) bool {
	for _, a := range l.args {
		var s string
		switch v := a.(type) {
		case string:
			s = v
		case []byte:
			s = string(v)
		default:
			continue
		}
		if strings.Contains(s, fragment) {
			return true
		}
	}
	return false
}
