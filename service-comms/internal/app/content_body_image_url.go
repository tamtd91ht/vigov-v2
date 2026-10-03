package app

// A BODY IMAGE FROM A PASTED https LINK (ADR 0067 §Sửa đổi 03/10/2026, H1, H5, K6, K8):
//
//	POST /api/v1/content-items/body-images/from-url   {url, content_item_id?}   → a READY body image
//
// THE SERVER downloads the image (H5) — residents' phones only ever reach ViGov's public store — through
// internal/imagefetch (the SSRF boundary: URL shape before any network, the portal's AddrAllowed at every
// dial, ≤ 3 re-checked redirects, verified TLS, 30 s, no cookies), then through the SAME pipeline a portal
// cover takes (portal_image.go): sniff (the remote Content-Type is never read), the purpose's policy,
// ClamAV, the pixel budget, the EXIF-free `thumb-1280` re-encode, and recordPortalCover's guarded walk to
// `ready`. The article id follows requestUpload's rules exactly (admitSubject): none → reserved here; else
// a live article of this commune or an id THIS officer's earlier upload reserved.
//
// NO ORIGINAL IS STORED — DECIDED BY ADR 0052, NOT HERE. core/storage admits a `content-source` original
// only through Promote (a commune upload, scanned in temp) and PutServerProduced refuses to write one
// (ADR 0052 §Bổ sung 30/09 lần hai). A fetched image is not a commune upload: its re-encode is what is
// kept, exactly as for a portal cover; the sha256 of the bytes the site served is in the audit delta.
// Storing the fetched bytes as well would be a ninth core/storage operation — the owner's ADR change.
//
// WHAT NEVER LEAVES THIS FILE: the URL. Its path and query can carry a token or a person's data (rule
// 3), so the trail records the HOST only, and every error is a class (imagefetch.Error), never a quote.
//
// ORDER, AND WHY: the article and the count are checked BEFORE the download (a refused request costs no
// fetch, and the route is no probe for an officer without a usable article), the download and the
// derivative happen OUTSIDE any transaction (network and CPU of unbounded length), and the row and its
// audit entry are written in ONE short transaction that checks the article and the count AGAIN, under
// the per-subject count lock (admitSubject → LockSubjectCount) — two pastes, or a paste and an upload,
// racing for the 20th slot serialise there whether or not the article row exists yet. A transaction that fails after the derivative was
// stored deletes it (discardPortalCover); a failed delete leaves an unreferenced private object, never
// a row without its object.

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/imagefetch"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// ActionBodyImageFetched is the trail's verb for a body image the server downloaded from a pasted link —
// its OWN verb, as the upload request has one: an inspection must be able to tell "an officer uploaded
// this file" from "the server fetched it from <host> on an officer's request".
const ActionBodyImageFetched = "tai_anh_than_bai_noi_dung_tu_lien_ket"

// bodyImageFromURLName is stored_file.original_name of every fetched body image: the remote file name can
// carry a person's name (rule 3), and nobody typed it here.
const bodyImageFromURLName = "anh-than-bai-tu-lien-ket.jpg"

const (
	// bodyImageFetchMaxBytes is the MEMORY bound on one download, applied under the policy's own MaxBytes
	// (whichever is smaller wins). A VENDOR BOUND, not a customer number: the whole body is held in memory
	// for the sniff, the scan and the decode, and the policy's 50 MB per request would let a handful of
	// officers pasting at once exhaust the pod. 10 MiB is the portal's figure (portalImageMaxBytes) — a
	// news photo is far below it, and a larger one is a file to upload instead.
	bodyImageFetchMaxBytes = 10 << 20
	// bodyImageFetchSlots bounds the downloads in flight in this process: with the per-download bound
	// above, at most 4 × 10 MiB of fetched bytes are held at once. A request waiting for a slot waits
	// within its own context.
	bodyImageFetchSlots = 4
)

var (
	// ErrImageURLInvalid: the pasted link failed the shape check (https, port 443, no userinfo, a DNS
	// name, ≤ 2048 characters). Nothing left this process. 400.
	ErrImageURLInvalid = errors.New("ảnh thân bài: liên kết ảnh không hợp lệ")
	// ErrImageFetchFailed is what every *ImageFetchError matches. ONE answer for DNS, a refused address,
	// TLS, timeout, a non-200 and too many redirects (502): telling them apart to the caller would map the
	// cluster's internal addresses for them.
	ErrImageFetchFailed = errors.New("ảnh thân bài: không tải được ảnh từ liên kết")
)

// ImageFetchError is a download that failed. Class is imagefetch's (`dns`, `address-refused`, `http-404`
// …) — for the log line, never for the client. Refused reports that THIS side refused a destination: the
// security event `outbound_url_refused`.
type ImageFetchError struct {
	Class   string
	Refused bool
}

func (e *ImageFetchError) Error() string   { return ErrImageFetchFailed.Error() + ": " + e.Class }
func (e *ImageFetchError) Is(t error) bool { return t == ErrImageFetchFailed }

// ImageFetcher is *imagefetch.Client.
type ImageFetcher interface {
	Fetch(ctx context.Context, u *url.URL, max int64) ([]byte, error)
}

// WithImageFetcher wires the outbound client. Without it (nil) the route answers as "not configured"
// (503), like a missing store or scanner — fail closed.
func (uc *ContentCovers) WithImageFetcher(f ImageFetcher) *ContentCovers {
	uc.fetcher = f
	uc.fetchSlots = make(chan struct{}, bodyImageFetchSlots)
	return uc
}

// BodyImageFromURLRequest is what the editor sends: the pasted link, and the article like an upload.
type BodyImageFromURLRequest struct {
	URL           string
	ContentItemID string
}

// FetchBodyImage downloads one pasted image and stores it as a READY body image of the article (file
// header). The returned row's SubjectID is the article — the reserved id when the request named none.
func (uc *ContentCovers) FetchBodyImage(ctx context.Context, req BodyImageFromURLRequest, actor audit.Actor) (
	domain.StoredFile, error) {

	if actor.ID == "" {
		return domain.StoredFile{}, ErrThieuNguoiTaoNoiDung
	}
	u, err := imagefetch.ParseURL(req.URL)
	if err != nil {
		return domain.StoredFile{}, ErrImageURLInvalid
	}
	host := strings.ToLower(u.Hostname())
	if len(req.ContentItemID) > domain.MaxFileIDLen {
		return domain.StoredFile{}, commsstore.ErrNoiDungKhongTonTai
	}
	if !uc.uploadsConfigured() || uc.fetcher == nil {
		return domain.StoredFile{}, ErrCoverUploadNotConfigured
	}
	pol, err := uc.policyFor(ctx, bodyImagePurpose)
	if err != nil {
		return domain.StoredFile{}, err
	}
	subject := req.ContentItemID
	if subject == "" {
		// THE FUTURE ARTICLE'S ID, minted by the server — as RequestBodyImageUpload does.
		if subject, err = uc.newID(); err != nil {
			return domain.StoredFile{}, fmt.Errorf("ảnh thân bài: sinh mã mục nội dung: %w", err)
		}
	}
	now := uc.clock()

	// 1. Before any network: may this officer add a body image to this article at all?
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return uc.admitSubject(ctx, tx, bodyImagePurpose, pol, req.ContentItemID, subject, actor, now)
	})
	if err != nil {
		return domain.StoredFile{}, bocNoiDung(ctx, "tải ảnh thân bài từ liên kết", err)
	}

	// 2. The download, under the memory bound and a process-wide slot.
	limit := min(pol.MaxBytes, bodyImageFetchMaxBytes)
	data, err := uc.fetch(ctx, u, limit)
	if err != nil {
		return domain.StoredFile{}, err
	}

	// 3. The portal cover's pipeline, under this purpose and this officer.
	pc, err := uc.prepareFetchedImage(ctx, fetchedImage{
		purpose: bodyImagePurpose, pol: pol, maxBytes: limit, itemID: subject, data: data, now: now,
		uploadedBy: actor.ID, name: bodyImageFromURLName,
	})
	if err != nil {
		return domain.StoredFile{}, err
	}

	// 4. One transaction: the article and the count again, the row walked to `ready`, the trail.
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.admitSubject(ctx, tx, bodyImagePurpose, pol, req.ContentItemID, subject, actor, now); err != nil {
			return err
		}
		if err := uc.recordPortalCover(ctx, tx, pc); err != nil {
			return err
		}
		return writeCoverAudit(ctx, tx, actor, bodyImageVerbs, ActionBodyImageFetched, now, map[string]any{
			"tep_id":          pc.file.ID,
			"muc_noi_dung_id": subject,
			"muc_dich":        string(bodyImagePurpose),
			// THE HOST ONLY — never the path or the query (file header, rule 3).
			"may_chu_nguon": host,
			"loai_tep":      pc.file.MIMEType,
			"kich_thuoc":    pc.file.SizeBytes,
			"sha256":        pc.file.SHA256,
			"sha256_nguon":  pc.sourceSHA256,
			"ban_dan_xuat":  CoverDerivativeVariant,
		})
	})
	if err != nil {
		// The derivative is in the private bucket with no row naming it: delete it (the row is asked
		// first — a commit that reported an error may still have landed). A failure is not hidden: it is
		// joined to the error the caller logs; the object key carries no personal data (ADR 0052 §3).
		if _, derr := uc.discardPortalCover(ctx, pc.file); derr != nil {
			err = errors.Join(err, fmt.Errorf("ảnh thân bài: chưa xoá được bản dẫn xuất mồ côi %s: %w",
				pc.file.ObjectKey, derr))
		}
		return domain.StoredFile{}, bocNoiDung(ctx, "tải ảnh thân bài từ liên kết", err)
	}
	return pc.file, nil
}

// fetch runs one download in a process-wide slot and maps its failure: over the cap is a refusal of the
// FILE (422, like an upload over the policy); everything else is one ImageFetchError (502).
func (uc *ContentCovers) fetch(ctx context.Context, u *url.URL, limit int64) ([]byte, error) {
	select {
	case uc.fetchSlots <- struct{}{}:
		defer func() { <-uc.fetchSlots }()
	case <-ctx.Done():
		return nil, &ImageFetchError{Class: "canceled"}
	}
	data, err := uc.fetcher.Fetch(ctx, u, limit)
	switch {
	case err == nil:
		return data, nil
	case errors.Is(err, imagefetch.ErrTooLarge):
		return nil, &CoverRejection{Reason: CoverRejectTooLarge}
	default:
		return nil, &ImageFetchError{Class: imagefetch.ClassOf(err), Refused: imagefetch.IsRefusal(err)}
	}
}
