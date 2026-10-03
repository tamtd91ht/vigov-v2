package app

// The body images of an article (ADR 0067 §Sửa đổi 03/10/2026, H7, K2, K3, K7) over the same rig as the
// cover (content_cover_test.go): the REAL content store on the fake driver, in-memory file rows and
// object store. What lives here: the per-article cap read from platform's policy, the reservation of an
// unsaved article's id, the attach check on save, the retire of removed images, and the purpose-aware
// publish/withdraw that must never take a referenced body image down with the cover.
//
// "ANOTHER COMMUNE'S FILE" IS NOT A ROW HERE: every read of stored_file binds the commune to $1 (store,
// stored_file_pg_test.go), so to this layer another commune's id is indistinguishable from an unknown
// one — which is exactly the property, and why both cases expect the same error.

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const (
	bodyFileA       = "01JBBBBBBBBBBBBBBBBBBBBBBB"
	bodyFileB       = "01JCCCCCCCCCCCCCCCCCCCCCCC"
	bodyOtherItem   = "01JHHHHHHHHHHHHHHHHHHHHHHH"
	bodyNowhereFile = "01JZZZZZZZZZZZZZZZZZZZZZZZ" // no row: unknown, or another commune's
)

// bodyPolicy is platform's `content-body-image` row as seeded (41940cb4): the cap comes from HERE, the
// way the use case reads it, never from a constant of the test.
func bodyPolicy(max int) fakePolicies {
	return fakePolicies{ok: true, p: uploadpolicy.Policy{Purpose: storage.PurposeContentBodyImage, MaxBytes: 50 << 20,
		AllowedMIMETypes: []string{storage.MIMEJPEG, storage.MIMEPNG, storage.MIMEWebP},
		FileCountLimited: true, MaxFilesPerSubject: max}}
}

// readyBodyImage puts a ready body-image row for item into the fake, as a completion would leave it.
func readyBodyImage(files *fakeCoverFiles, id, item string) *domain.StoredFile {
	f := readyCover(files, id, item)
	f.Purpose = string(storage.PurposeContentBodyImage)
	f.ObjectKey = strings.Replace(f.ObjectKey, "/content-image/", "/content-body-image/", 1)
	return f
}

func publicKeyOf(f *domain.StoredFile) string {
	return strings.Replace(strings.Replace(f.ObjectKey, "content-source/", "public-media/", 1),
		"/original.png", "/thumb-1280.jpg", 1)
}

func figure(id string) string {
	return `<figure><img data-file-id="` + id + `" alt="Ảnh"><figcaption>Chú thích</figcaption></figure>`
}

func auditActions(k *khoNDGia) []string {
	var out []string
	for _, l := range k.cau("INSERT INTO audit_log") {
		for _, a := range l.args {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// --- upload: the cap (H7) and the reservation ----------------------------------------------------------

func TestBodyImageRequestRefusesThe21stFromThePolicyCount(t *testing.T) {
	r := newCoverRig(t, &khoNDGia{}) // the article is not saved yet: its id is a reservation
	r.uc.policies = bodyPolicy(20)
	for i := 0; i < 20; i++ {
		id := "01JQ" + strings.Repeat(string("0123456789ABCDEFGHJKMNPQRS"[i]), 22)
		readyBodyImage(r.files, id, coverItemID)
	}
	req := CoverUploadRequest{ContentItemID: coverItemID, FileName: "anh.jpg", ContentType: storage.MIMEJPEG, Size: 10}
	_, err := r.uc.RequestBodyImageUpload(r.ctx, req, nguoiSoanND())
	if !errors.Is(err, ErrCoverCountReached) {
		t.Fatalf("21st body image: err = %v, want ErrCoverCountReached", err)
	}
	if len(r.files.inserted) != 0 || r.k.daCommit != 0 {
		t.Errorf("a refused 21st wrote a row (%d) or committed (%d)", len(r.files.inserted), r.k.daCommit)
	}

	// A retired image (taken out of the body) frees its slot: the cap counts LIVE body images.
	for id := range r.files.rows {
		r.files.deleted[id] = true
		break
	}
	up, err := r.uc.RequestBodyImageUpload(r.ctx, req, nguoiSoanND())
	if err != nil {
		t.Fatalf("20 live after a retire: err = %v", err)
	}
	if up.File.Purpose != string(storage.PurposeContentBodyImage) || up.File.SubjectID != coverItemID ||
		!strings.Contains(up.File.ObjectKey, "/comms/content-body-image/") {
		t.Errorf("row = %+v", up.File)
	}
	acts := auditActions(r.k)
	if !containsString(acts, ActionBodyImageUploadRequested) || !containsString(acts, "noi-dung-mini-app/anh-than-bai/2026-10-01") {
		t.Errorf("the trail must name a BODY image, got %v", acts)
	}
}

// THE COUNT LOCK (H7 under concurrency): the per-subject lock is taken BEFORE the count on EVERY path —
// a reserved id (no article row to lock, the case that let two requests at 19 both insert), an existing
// article (row lock too) and a freshly minted id — and the row is inserted after it. The fake has no
// transactions, so "the same transaction" is shown as one lock per count, insert last; the PG test
// (store/stored_file_pg_test.go) is where the lock is shown to actually block.
func TestBodyImageAdmissionLocksTheSubjectBeforeCounting(t *testing.T) {
	const lock = "lock:" + coverItemID + ":content-body-image"
	const count, insert, reserved = "count:" + coverItemID, "insert:" + coverItemID, "reserved:" + coverItemID
	existing := func() *domain.NoiDungMiniApp {
		return &domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
			TrangThai: domain.TrangThaiAn, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND}
	}
	for _, tc := range []struct {
		name    string
		row     *domain.NoiDungMiniApp
		named   string
		reserve bool
		want    []string
	}{
		{"reserved id, no article row", nil, coverItemID, true, []string{reserved, lock, count, insert}},
		{"existing article", existing(), coverItemID, false, []string{lock, count, insert}},
		{"fresh id minted here", nil, "", false, []string{lock, count, insert}},
	} {
		t.Run("upload/"+tc.name, func(t *testing.T) {
			r := newCoverRig(t, &khoNDGia{dongHienCo: tc.row})
			r.uc.policies = bodyPolicy(20)
			if tc.reserve {
				readyBodyImage(r.files, bodyFileA, coverItemID)
			}
			r.files.admitCalls = nil
			_, err := r.uc.RequestBodyImageUpload(r.ctx, CoverUploadRequest{ContentItemID: tc.named, FileName: "a.jpg",
				ContentType: storage.MIMEJPEG, Size: 10}, nguoiSoanND())
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got := strings.Join(r.files.admitCalls, ","); got != strings.Join(tc.want, ",") {
				t.Errorf("calls = %s, want %s", got, strings.Join(tc.want, ","))
			}
			if tc.row != nil && !r.k.coCau("FOR UPDATE") {
				t.Error("an existing article was not locked by its row")
			}
		})
	}

	// FetchBodyImage: the pre-check transaction and the write transaction each take the lock before their
	// count; the row is inserted only in the second, after its own lock.
	for _, tc := range []struct {
		name  string
		row   *domain.NoiDungMiniApp
		named string
		want  []string
	}{
		{"reserved id, no article row", nil, coverItemID,
			[]string{reserved, lock, count, reserved, lock, count, insert}},
		{"existing article", existing(), coverItemID, []string{lock, count, lock, count, insert}},
		{"fresh id minted here", nil, "", []string{lock, count, lock, count, insert}},
	} {
		t.Run("fetch/"+tc.name, func(t *testing.T) {
			jpg := testJPEG(t, 400, 300, 0)
			r := newFetchRig(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(jpg) })
			r.k.dongHienCo = tc.row
			if tc.named != "" {
				r.uc.newID = func() (string, error) { return fetchedFile, nil } // no article id is minted
			}
			if tc.named != "" && tc.row == nil {
				readyBodyImage(r.files, bodyFileA, coverItemID)
			}
			r.files.admitCalls = nil
			if _, err := r.uc.FetchBodyImage(r.ctx, BodyImageFromURLRequest{URL: fetchURL, ContentItemID: tc.named},
				nguoiSoanND()); err != nil {
				t.Fatalf("err = %v", err)
			}
			if got := strings.Join(r.files.admitCalls, ","); got != strings.Join(tc.want, ",") {
				t.Errorf("calls = %s, want %s", got, strings.Join(tc.want, ","))
			}
		})
	}
}

func TestBodyImageCapDoesNotCountTheCover(t *testing.T) {
	r := newCoverRig(t, &khoNDGia{})
	r.uc.policies = bodyPolicy(1)
	readyCover(r.files, coverFileID, coverItemID) // the cover: another purpose, another count
	_, err := r.uc.RequestBodyImageUpload(r.ctx, CoverUploadRequest{ContentItemID: coverItemID, FileName: "a.jpg",
		ContentType: storage.MIMEJPEG, Size: 10}, nguoiSoanND())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestBodyImageRequestOnAnotherOfficersReservationIs404(t *testing.T) {
	r := newCoverRig(t, &khoNDGia{})
	r.uc.policies = bodyPolicy(20)
	f := readyBodyImage(r.files, bodyFileA, coverItemID)
	f.UploadedBy = "CB-2026-KHAC00"
	_, err := r.uc.RequestBodyImageUpload(r.ctx, CoverUploadRequest{ContentItemID: coverItemID, FileName: "a.jpg",
		ContentType: storage.MIMEJPEG, Size: 10}, nguoiSoanND())
	if !errors.Is(err, commsstore.ErrNoiDungKhongTonTai) {
		t.Fatalf("err = %v, want the 404 of an unknown article", err)
	}
	if len(r.files.inserted) != 0 {
		t.Error("a row was written under another officer's reservation")
	}
}

func TestBodyImageRequestWithoutItemMintsTheArticleID(t *testing.T) {
	r := newCoverRig(t, nil)
	r.uc.policies = bodyPolicy(20)
	up, err := r.uc.RequestBodyImageUpload(r.ctx, CoverUploadRequest{FileName: "a.jpg", ContentType: storage.MIMEJPEG,
		Size: 10}, nguoiSoanND())
	if err != nil {
		t.Fatal(err)
	}
	if up.File.ID != coverFileID || up.File.SubjectID != coverItemID {
		t.Errorf("file %q for article %q, want the server's two minted ids", up.File.ID, up.File.SubjectID)
	}
}

// --- completion --------------------------------------------------------------------------------------

func TestBodyImageCompletesLikeACoverAndRoutesDoNotCross(t *testing.T) {
	r := newCoverRig(t, nil)
	r.uc.policies = bodyPolicy(20)
	up, err := r.uc.RequestBodyImageUpload(r.ctx, CoverUploadRequest{FileName: "a.jpg", ContentType: storage.MIMEJPEG,
		Size: 10}, nguoiSoanND())
	if err != nil {
		t.Fatal(err)
	}
	r.objects.temp[up.Post.Fields["key"]] = testJPEG(t, 1600, 900, 0)

	// The cover's completion does not complete a body image: one purpose per route.
	if _, err := r.uc.Complete(r.ctx, up.File.ID, nguoiSoanND()); !errors.Is(err, ErrCoverFileNotFound) {
		t.Fatalf("cover completion of a body image: err = %v", err)
	}
	got, err := r.uc.CompleteBodyImageUpload(r.ctx, up.File.ID, nguoiSoanND())
	if err != nil {
		t.Fatalf("CompleteBodyImageUpload: %v", err)
	}
	if got.Status != domain.StoredFileReady || r.scanner.scanned != 1 {
		t.Fatalf("status=%s scanned=%d", got.Status, r.scanner.scanned)
	}
	deriv := strings.Replace(up.File.ObjectKey, "/original.jpg", "/thumb-1280.jpg", 1)
	if _, ok := r.objects.produced[deriv]; !ok {
		t.Errorf("no EXIF-free derivative at %q", deriv)
	}
	if !containsString(auditActions(r.k), ActionBodyImageStored) {
		t.Errorf("trail = %v", auditActions(r.k))
	}
}

// --- attach on save (K2) ------------------------------------------------------------------------------

func TestCreateWithBodyImageTakesTheReservedIDAndPublishesIt(t *testing.T) {
	k := &khoNDGia{}
	uc, files, objects, ctx := soanWithCovers(t, k)
	readyBodyImage(files, bodyFileA, coverItemID)

	yc := ycThemMau()
	yc.NoiDung, yc.DangLenMiniApp = "<p>Mở đầu</p>"+figure(bodyFileA)+"<p>Kết</p>", true
	moi, err := uc.Them(ctx, yc, nguoiSoanND())
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if moi.ID != coverItemID {
		t.Errorf("id = %q, want the body image's reserved %q", moi.ID, coverItemID)
	}
	if len(objects.published) != 1 || !strings.Contains(objects.published[0], "/content-body-image/") ||
		!strings.HasSuffix(objects.published[0], "/thumb-1280.jpg") {
		t.Errorf("published = %v", objects.published)
	}
	if k.daCommit != 1 {
		t.Errorf("commits = %d", k.daCommit)
	}
}

func TestSaveRefusesEveryForeignBodyImageWithTheSameError(t *testing.T) {
	type setup func(files *fakeCoverFiles)
	for name, tc := range map[string]struct {
		body  string
		setup setup
	}{
		"unknown id": {figure(bodyNowhereFile), func(*fakeCoverFiles) {}},
		// Another commune's file is not a row this commune can read — the same nil as unknown.
		"another commune's": {figure(bodyNowhereFile), func(*fakeCoverFiles) {}},
		"another item's": {figure(bodyFileA) + figure(bodyFileB), func(f *fakeCoverFiles) {
			readyBodyImage(f, bodyFileA, coverItemID)
			readyBodyImage(f, bodyFileB, bodyOtherItem)
		}},
		"the cover's file": {figure(bodyFileA) + figure(coverFileID), func(f *fakeCoverFiles) {
			readyBodyImage(f, bodyFileA, coverItemID)
			readyCover(f, coverFileID, coverItemID)
		}},
		"not finished": {figure(bodyFileA), func(f *fakeCoverFiles) {
			readyBodyImage(f, bodyFileA, coverItemID).Status = domain.StoredFilePending
		}},
	} {
		t.Run("create/"+name, func(t *testing.T) {
			k := &khoNDGia{}
			uc, files, objects, ctx := soanWithCovers(t, k)
			tc.setup(files)
			yc := ycThemMau()
			yc.NoiDung, yc.DangLenMiniApp = tc.body, true
			if _, err := uc.Them(ctx, yc, nguoiSoanND()); !errors.Is(err, domain.ErrBodyImageNotUsable) {
				t.Fatalf("err = %v, want ErrBodyImageNotUsable", err)
			}
			if k.coCau("INSERT INTO noi_dung_mini_app") || k.daCommit != 0 || len(objects.published) != 0 {
				t.Error("a refused body wrote the article or published a file")
			}
		})
		t.Run("edit/"+name, func(t *testing.T) {
			item := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
				TrangThai: domain.TrangThaiAn, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND}
			k := &khoNDGia{dongHienCo: &item}
			uc, files, _, ctx := soanWithCovers(t, k)
			tc.setup(files)
			body := tc.body
			if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{NoiDung: &body}, nguoiSoanND()); !errors.Is(err, domain.ErrBodyImageNotUsable) {
				t.Fatalf("err = %v, want ErrBodyImageNotUsable", err)
			}
			if k.coCau("UPDATE noi_dung_mini_app") || k.daCommit != 0 {
				t.Error("a refused body wrote the article")
			}
		})
	}
}

func TestCreateUnderAnExistingArticlesBodyImageIsRefused(t *testing.T) {
	existing := domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Đã có", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiAn, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND}
	k := &khoNDGia{dongHienCo: &existing}
	uc, files, _, ctx := soanWithCovers(t, k)
	readyBodyImage(files, bodyFileA, coverItemID)
	yc := ycThemMau()
	yc.NoiDung = figure(bodyFileA)
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); !errors.Is(err, domain.ErrBodyImageNotUsable) {
		t.Fatalf("err = %v", err)
	}
	if k.coCau("INSERT INTO noi_dung_mini_app") {
		t.Error("a second article was created under an existing id")
	}
}

// --- removed images and the purpose-aware settle -------------------------------------------------------

func publishedItemWithBody(body, cover string) domain.NoiDungMiniApp {
	return domain.NoiDungMiniApp{ID: coverItemID, Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: lucNDPinned,
		TrangThai: domain.TrangThaiDangHien, Nguon: domain.NguonThuCong, NguoiTaoMa: maCanBoSoanND,
		NoiDung: body, CoverImageFileID: cover, PublishedAt: lucNDPinned}
}

func TestReplacingTheCoverKeepsReferencedBodyImagesPublic(t *testing.T) {
	item := publishedItemWithBody(figure(bodyFileA), coverFileID)
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	oldCover := readyCover(files, coverFileID, coverItemID)
	oldCover.PublicObjectKey = publicKeyOf(oldCover)
	body := readyBodyImage(files, bodyFileA, coverItemID)
	body.PublicObjectKey = publicKeyOf(body)
	readyCover(files, coverOtherID, coverItemID)
	oldKey := oldCover.PublicObjectKey

	next := coverOtherID
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{CoverImageFileID: &next}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if len(objects.unpublished) != 1 || objects.unpublished[0] != oldKey {
		t.Fatalf("withdrawn = %v, want only the old cover %s", objects.unpublished, oldKey)
	}
	if files.get(bodyFileA).PublicObjectKey == "" {
		t.Error("the cover sweep withdrew a body image the article still shows")
	}
}

func TestEditRetiresRemovedBodyImagesAndWithdrawsThePublishedOne(t *testing.T) {
	item := publishedItemWithBody(figure(bodyFileA)+figure(bodyFileB), "")
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	a := readyBodyImage(files, bodyFileA, coverItemID)
	a.PublicObjectKey = publicKeyOf(a)
	b := readyBodyImage(files, bodyFileB, coverItemID)
	b.PublicObjectKey = publicKeyOf(b)
	bKey := b.PublicObjectKey

	keep := "<p>Chỉ còn một ảnh</p>" + figure(bodyFileA)
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{NoiDung: &keep}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if len(objects.unpublished) != 1 || objects.unpublished[0] != bKey {
		t.Fatalf("withdrawn = %v, want only the removed image %s", objects.unpublished, bKey)
	}
	if files.get(bodyFileA) == nil || files.get(bodyFileA).PublicObjectKey == "" {
		t.Error("the image still in the body lost its public copy")
	}
	// Withdrawn after commit, THEN retired in the withdrawal's own transaction, with its own entry.
	if !files.deleted[bodyFileB] || files.deleted[bodyFileA] {
		t.Errorf("retired = %v, want only %s", files.softDeleted, bodyFileB)
	}
	if !containsString(auditActions(k), ActionBodyImageWithdrawn) {
		t.Errorf("trail = %v", auditActions(k))
	}
}

func TestEditRetiresAnUnpublishedRemovedImageInItsOwnTransaction(t *testing.T) {
	item := publishedItemWithBody(figure(bodyFileA)+figure(bodyFileB), "")
	item.TrangThai, item.PublishedAt = domain.TrangThaiAn, lucNDPinned
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	readyBodyImage(files, bodyFileA, coverItemID)
	readyBodyImage(files, bodyFileB, coverItemID)
	readyCover(files, coverFileID, coverItemID) // NOT in the body: never touched by a body edit

	keep := figure(bodyFileA)
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{NoiDung: &keep}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if strings.Join(files.softDeleted, ",") != bodyFileB {
		t.Errorf("retired = %v, want [%s]", files.softDeleted, bodyFileB)
	}
	if len(objects.unpublished) != 0 || k.daCommit != 1 {
		t.Errorf("unpublished=%v commits=%d — nothing was public, one transaction", objects.unpublished, k.daCommit)
	}
	if !strings.Contains(auditDelta(t, k), "body_images_retired") {
		t.Errorf("the edit's entry must name the retired image: %s", auditDelta(t, k))
	}
}

func TestAnyEditRetiresReadyOrphansAndKeepsPendingUploads(t *testing.T) {
	// An image uploaded and completed but never placed in the body (the officer changed their mind before
	// saving) holds a slot of the per-article count until a save retires it. A PENDING upload may be in
	// flight while the officer saves: it stays.
	const orphan, inFlight = "01JDDDDDDDDDDDDDDDDDDDDDDD", "01JEEEEEEEEEEEEEEEEEEEEEEE"
	item := publishedItemWithBody(figure(bodyFileA), "")
	item.TrangThai = domain.TrangThaiAn
	k := &khoNDGia{dongHienCo: &item}
	uc, files, _, ctx := soanWithCovers(t, k)
	readyBodyImage(files, bodyFileA, coverItemID)
	readyBodyImage(files, orphan, coverItemID)
	readyBodyImage(files, inFlight, coverItemID).Status = domain.StoredFilePending
	purpose := string(storage.PurposeContentBodyImage)
	if n := files.liveCount(coverItemID, purpose, true); n != 3 {
		t.Fatalf("setup: live count = %d", n)
	}

	title := "Tiêu đề mới" // the body is not even sent: the rule runs on every save
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{TieuDe: &title}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if strings.Join(files.softDeleted, ",") != orphan {
		t.Fatalf("retired = %v, want only the ready orphan %s", files.softDeleted, orphan)
	}
	if files.get(inFlight) == nil || files.get(bodyFileA) == nil {
		t.Error("the in-flight upload or the placed image was retired")
	}
	if n := files.liveCount(coverItemID, purpose, true); n != 2 {
		t.Errorf("live count after the save = %d, want 2 — the orphan's slot is free", n)
	}
	if !strings.Contains(auditDelta(t, k), orphan) {
		t.Errorf("the edit's entry must name the retired orphan: %s", auditDelta(t, k))
	}
}

func TestCreateRetiresReadyOrphansOfItsReservedID(t *testing.T) {
	const orphan, inFlight = "01JDDDDDDDDDDDDDDDDDDDDDDD", "01JEEEEEEEEEEEEEEEEEEEEEEE"
	k := &khoNDGia{}
	uc, files, _, ctx := soanWithCovers(t, k)
	readyBodyImage(files, bodyFileA, coverItemID)
	readyBodyImage(files, orphan, coverItemID)
	readyBodyImage(files, inFlight, coverItemID).Status = domain.StoredFilePending

	yc := ycThemMau()
	yc.NoiDung = figure(bodyFileA)
	if _, err := uc.Them(ctx, yc, nguoiSoanND()); err != nil {
		t.Fatalf("Them: %v", err)
	}
	if strings.Join(files.softDeleted, ",") != orphan || files.get(inFlight) == nil {
		t.Errorf("retired = %v, want only %s; the pending upload must survive", files.softDeleted, orphan)
	}
}

func TestEditNeverRetiresAFileTheOldBodyNamedButIsNotItsBodyImage(t *testing.T) {
	// A body saved before the attach check existed could name ANY id — here the cover's. Taking it out
	// of the body must not retire the cover.
	item := publishedItemWithBody(figure(coverFileID), "")
	item.TrangThai = domain.TrangThaiAn
	k := &khoNDGia{dongHienCo: &item}
	uc, files, _, ctx := soanWithCovers(t, k)
	readyCover(files, coverFileID, coverItemID)

	plain := "<p>Không còn ảnh</p>"
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{NoiDung: &plain}, nguoiSoanND()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if len(files.softDeleted) != 0 {
		t.Errorf("retired %v — the cover is not a body image", files.softDeleted)
	}
}

func TestPublishingPublishesBodyImagesAndHidingWithdrawsWithoutRetiring(t *testing.T) {
	item := publishedItemWithBody(figure(bodyFileA), "")
	item.TrangThai = domain.TrangThaiAn
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	readyBodyImage(files, bodyFileA, coverItemID)

	publish := true
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{DangLenMiniApp: &publish}, nguoiSoanND()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(objects.published) != 1 || files.get(bodyFileA).PublicObjectKey == "" {
		t.Fatalf("published=%v key=%q", objects.published, files.get(bodyFileA).PublicObjectKey)
	}

	shown := *k.dongHienCo
	shown.TrangThai = domain.TrangThaiDangHien
	k.dongHienCo = &shown
	hide := false
	if _, err := uc.Sua(ctx, coverItemID, domain.YeuCauSuaNoiDung{DangLenMiniApp: &hide}, nguoiSoanND()); err != nil {
		t.Fatalf("hide: %v", err)
	}
	if len(objects.unpublished) != 1 || files.get(bodyFileA) == nil || files.get(bodyFileA).PublicObjectKey != "" {
		t.Errorf("unpublished=%v row=%+v — hidden: withdrawn, still the article's file", objects.unpublished, files.get(bodyFileA))
	}
	if len(files.softDeleted) != 0 {
		t.Error("hiding retired a body image the body still references")
	}
}

func TestDeletingTheArticleWithdrawsItsBodyImages(t *testing.T) {
	item := publishedItemWithBody(figure(bodyFileA), "")
	k := &khoNDGia{dongHienCo: &item}
	uc, files, objects, ctx := soanWithCovers(t, k)
	a := readyBodyImage(files, bodyFileA, coverItemID)
	a.PublicObjectKey = publicKeyOf(a)
	if err := uc.Delete(ctx, coverItemID, "Đăng nhầm bài", nguoiSoanND()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(objects.unpublished) != 1 || len(files.softDeleted) != 0 {
		t.Errorf("unpublished=%v retired=%v — withdrawn, the row stays with the record", objects.unpublished, files.softDeleted)
	}
}

// --- staff preview and public URLs ---------------------------------------------------------------------

func TestBodyImageViewsSignOnlyThisArticlesReadyBodyImages(t *testing.T) {
	r := newCoverRig(t, nil)
	readyBodyImage(r.files, bodyFileA, coverItemID)
	readyBodyImage(r.files, bodyFileB, coverItemID).Status = domain.StoredFilePending
	readyBodyImage(r.files, coverOtherID, bodyOtherItem) // another article's: never signed here

	views, err := r.uc.BodyImageViews(r.ctx, coverItemID, []string{bodyFileA, bodyFileB, coverOtherID, bodyNowhereFile})
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 4 {
		t.Fatalf("views = %+v", views)
	}
	if !strings.HasPrefix(views[0].PreviewURL.URL(), "https://signed.example/") ||
		!strings.HasSuffix(views[0].PreviewURL.URL(), "/thumb-1280.jpg") || views[0].Status != domain.StoredFileReady {
		t.Errorf("ready image: %+v", views[0])
	}
	for _, v := range views[1:] {
		if v.PreviewURL != "" {
			t.Errorf("%s got a preview URL: %+v", v.FileID, v)
		}
	}
}

func TestPublicBodyImageURLsOnlyPublishedBodyImagesOfThisArticle(t *testing.T) {
	r := newCoverRig(t, nil)
	pub := readyBodyImage(r.files, bodyFileA, coverItemID)
	pub.PublicObjectKey = publicKeyOf(pub)
	readyBodyImage(r.files, bodyFileB, coverItemID) // ready, not published
	other := readyBodyImage(r.files, coverOtherID, bodyOtherItem)
	other.PublicObjectKey = publicKeyOf(other) // published, another article
	cov := readyCover(r.files, coverFileID, coverItemID)
	cov.PublicObjectKey = publicKeyOf(cov) // published, the cover

	got, err := r.uc.PublicBodyImageURLs(r.ctx, coverItemID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[bodyFileA] != "https://media.example/"+pub.PublicObjectKey {
		t.Errorf("urls = %v", got)
	}
	r.uc.objects = nil
	if got, _ := r.uc.PublicBodyImageURLs(r.ctx, coverItemID); len(got) != 0 {
		t.Error("without object storage no URL may be built")
	}
}
