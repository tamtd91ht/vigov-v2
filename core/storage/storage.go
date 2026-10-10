// Package storage is the object-storage library of ADR 0052: a thin wrapper over MinIO/S3 that
// does five things — write an upload into temp (PutUpload), stat, server-side copy, presigned
// GET, delete every version — plus the key scheme and the type sniffing those need.
//
// PLUS PUBLISH / UNPUBLISH (added 2026-09-30, recorded in ADR 0052 §1). ADR 0052 §11 says
// "publishing copies the derivative to the public bucket; unpublishing deletes the public copy".
// Both are compositions of the copy and delete above, but they are the ONLY way into and out of
// the public bucket, so they are named operations with their own refusals (PublishDerivative)
// rather than a bucket argument on Promote — a bucket argument is how an original reaches the
// public bucket, which is ADR 0052 §ĐIỀU KIỆN DỪNG #2.
//
// PLUS PutServerProduced (added 2026-09-30, ADR 0052 §1 and §6 of ADR 0047): the one way to write
// bytes the SERVER made — a re-encoded cover image derivative, a citizen photo re-encoded without
// EXIF — into the private bucket. Every other write path copies bytes a client uploaded; this one
// streams bytes the caller hands over, so it carries its own refusals rather than widening Promote.
//
// PLUS PutUpload (added 2026-10-09, ADR 0052 §Sửa đổi 09/10/2026 — the ninth operation): uploads
// now travel THROUGH the owning service as one multipart request, and the service streams the file
// into the temp bucket under an `upload/…` key. PutUpload is the ONLY upload path: the presigned
// POST (PresignUpload / PresignedPost) was removed on 2026-10-10 once no service called it, so no
// device writes to MinIO directly any more. The public endpoint now signs GET links only.
//
// IT IS A LIBRARY, NOT A SERVICE (ADR 0001:96). Metadata lives in each owning service's own
// `stored_file` table (rule 2 invariant 1); this package knows nothing about it, nothing about
// permissions and nothing about the audit trail. The caller checks permission / citizen session
// and commune BEFORE PutUpload and before asking for a presigned GET — the binding of a GET URL to
// a person happens at issue time, because it is a bearer credential for its whole TTL (ADR 0052
// §Cái giá).
//
// THE UPLOAD FLOW IT SERVES (ADR 0052 §1, as amended 09/10/2026):
//
//	a. service: receives the multipart request, Key{…}.UploadPath() → PutUpload
//	           (declared type + size limit checked before a byte is read)
//	b. service: Stat → ReadHead + SniffMIME → Open + malwarescan.Scan → SHA256 → Promote(etag)
//
// Every step of (b) is bound to the ETag PutUpload returned. Only the service writes the temp key
// now, but the binding stays: it is what guarantees Promote copies exactly the bytes that were
// scanned, whatever else might reach the temp bucket.
//
// WHAT IT NEVER LOGS: this package does not log at all. Callers may log an object key (keys
// carry no personal data by construction — ADR 0052 §3) but never a presigned URL
// (a bearer credential) and never an original file name (personal data, rule 3).
package storage

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/vihat/vigov/core/config"
)

// Config is config.ObjectStorage. An alias, not a copy: one struct, one owner (rule 9), and a
// service hands cfg.ObjectStorage straight to New.
type Config = config.ObjectStorage

// UploadTTL is how long a `pending` stored_file row counts against a purpose's file limit — 15
// minutes, the figure ADR 0052 §1a fixed for the old presigned form. The services still use it:
// a row a crash left `pending` stops holding a slot after UploadTTL, so a retry is not refused
// for ever (service-petitions app/upload_stream.go, service-comms, service-platform branding.go).
const UploadTTL = 15 * time.Minute

// MaxDownloadTTL caps a presigned GET. ADR 0052 says "short-lived" without a number; the cap
// reuses the UploadTTL figure so there is one number to reason about. A presigned GET is checked
// when the request starts, so a long video download does not need a longer TTL.
const MaxDownloadTTL = 15 * time.Minute

var (
	// ErrNotConfigured: the object-storage settings are absent. The caller refuses the upload.
	ErrNotConfigured = errors.New("storage: object storage is not configured")
	// ErrNotFound: no such object (or version).
	ErrNotFound = errors.New("storage: object not found")
	// ErrChanged: the object no longer has the ETag the caller checked — it was replaced after
	// Stat. The caller must restart the completion step, never proceed.
	ErrChanged = errors.New("storage: object changed since it was inspected")
	// ErrTypeNotAllowed: the sniffed type is not on the allow-list.
	ErrTypeNotAllowed = errors.New("storage: file type not allowed")
	// ErrInvalidArgument: a call that violates the contract of this package.
	ErrInvalidArgument = errors.New("storage: invalid argument")
	// ErrExists: the destination key is already taken. Objects are immutable (ADR 0052 §1).
	ErrExists = errors.New("storage: destination object already exists")
	// ErrTempCleanup: Promote succeeded but the temp object could not be deleted. The returned
	// Promoted value IS valid; the temp lifecycle rule removes the leftover within a day.
	ErrTempCleanup = errors.New("storage: promoted, but the temp object was not removed")
	// ErrRecordsNotPurgeable: purging a `records` object outside the temp bucket. Deleting an
	// administrative record is a STOP CONDITION (ADR 0052 §ĐIỀU KIỆN DỪNG #1, rule 7).
	ErrRecordsNotPurgeable = errors.New("storage: records are never purged by this library")
	// ErrNotPublishable: PublishDerivative was asked to make public something that must never be
	// public — an original, or anything of class citizen-media (ADR 0052 §ĐIỀU KIỆN DỪNG #2).
	// Not a retryable error and not a caller bug to route around: it is a STOP CONDITION.
	ErrNotPublishable = errors.New("storage: object may not be published")
	// ErrTooLarge: PutUpload was given a size above the caller's limit for the purpose. Refused
	// before any byte is read; a handler answers 413.
	ErrTooLarge = errors.New("storage: file exceeds the size limit")
	// ErrSizeMismatch: a reader yielded more or fewer bytes than the size declared for it. Always
	// returned together with ErrInvalidArgument (both match errors.Is), so callers of
	// PutServerProduced that test for ErrInvalidArgument keep working. Nothing is committed.
	ErrSizeMismatch = errors.New("storage: body length differs from the declared size")
)

// Bucket is one of the three functional buckets of ADR 0052 §2.
type Bucket int

const (
	BucketPrivate Bucket = iota + 1 // every private business file; versioned; presigned GET only
	BucketPublic                    // approved derivatives only; anonymous GetObject
	BucketTemp                      // upload/… (1 day) and export/… (7 days)
)

func (b Bucket) suffix() (string, bool) {
	switch b {
	case BucketPrivate:
		return "private", true
	case BucketPublic:
		return "public", true
	case BucketTemp:
		return "temp", true
	}
	return "", false
}

// Client talks to one MinIO deployment through two endpoints.
type Client struct {
	api       *minio.Client // internal endpoint: every call that reaches the server
	signer    *minio.Client // public endpoint: presigning only, never dialled (Region is set)
	prefix    string
	mediaBase string
}

// New builds a Client. Absent settings return ErrNotConfigured naming the missing variables; a
// malformed value (including more than one endpoint host) is refused with the config error.
// It opens no connection.
func New(cfg Config) (*Client, error) {
	if missing := cfg.Missing(); len(missing) > 0 {
		return nil, fmt.Errorf("%w: missing %s", ErrNotConfigured, strings.Join(missing, ", "))
	}
	// Re-validated here and not only in config.Load: a Config can be built by hand, and the
	// single-host rule has to hold for every path into this constructor.
	endpoint, err := config.ParseSingleEndpoint("OBJECT_STORAGE_ENDPOINT", cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	public, err := config.ParseSingleEndpoint("OBJECT_STORAGE_PUBLIC_ENDPOINT", cfg.PublicEndpoint)
	if err != nil {
		return nil, err
	}
	mediaBase, err := config.ParsePublicMediaBaseURL(cfg.PublicMediaBaseURL)
	if err != nil {
		return nil, err
	}
	if err := config.CheckBucketPrefix(cfg.BucketPrefix); err != nil {
		return nil, err
	}
	region := cfg.Region
	if region == "" {
		region = config.ObjectStorageDefaultRegion
	}
	api, err := newMinio(endpoint, region, cfg)
	if err != nil {
		return nil, err
	}
	signer, err := newMinio(public, region, cfg)
	if err != nil {
		return nil, err
	}
	return &Client{api: api, signer: signer, prefix: cfg.BucketPrefix, mediaBase: mediaBase}, nil
}

// newMinio builds one minio client for a validated `scheme://host[:port]`.
//
// PATH-STYLE ADDRESSING, always: MinIO behind one domain serves `host/bucket/key`; virtual-host
// style would need a wildcard DNS record and certificate per bucket that nobody has provisioned.
func newMinio(endpoint, region string, cfg Config) (*minio.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("storage: endpoint: %w", err)
	}
	c, err := minio.New(u.Host, &minio.Options{
		// .Lo() at the final point of consumption, per core/secret.
		Creds:        credentials.NewStaticV4(string(cfg.AccessKey.Lo()), string(cfg.SecretKey.Lo()), ""),
		Secure:       u.Scheme == "https",
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: client for %s: %w", u.Host, err)
	}
	return c, nil
}

// LogAttrs says WHERE this client goes, for one startup line: the endpoint the service itself
// reads and writes through, the endpoint view/download (GET) links are SIGNED for — "cua_cong_khai",
// the door a phone or browser opens a link through; uploads no longer use it (ADR 0052 §Sửa đổi
// 09/10/2026) — and the three bucket names. The two endpoints must reach ONE MinIO; when they do
// not, a file the service stored answers 404 to every signed link — this line is where that shows.
// Scheme and host only: no key, no secret, no object.
func (c *Client) LogAttrs() []any {
	name := func(b Bucket) string {
		n, _ := c.BucketName(b) // the three constants always have a suffix
		return n
	}
	return []any{
		"cua_noi_bo", c.api.EndpointURL().Scheme + "://" + c.api.EndpointURL().Host,
		"cua_cong_khai", c.signer.EndpointURL().Scheme + "://" + c.signer.EndpointURL().Host,
		"bucket_private", name(BucketPrivate), "bucket_temp", name(BucketTemp), "bucket_public", name(BucketPublic),
	}
}

// BucketName returns `{prefix}-{private|public|temp}`.
func (c *Client) BucketName(b Bucket) (string, error) {
	s, ok := b.suffix()
	if !ok {
		return "", fmt.Errorf("%w: unknown bucket %d", ErrInvalidArgument, int(b))
	}
	return c.prefix + "-" + s, nil
}

// checkKey validates that key has the shape that belongs in bucket — `upload/…` or `export/…`
// in temp, a destination key elsewhere — and returns the bucket name and the parsed key. Every
// call that names an object goes through it, so no operation can act on a free-form string.
func (c *Client) checkKey(b Bucket, key string) (string, Key, error) {
	name, err := c.BucketName(b)
	if err != nil {
		return "", Key{}, err
	}
	var k Key
	if b == BucketTemp {
		if !strings.HasPrefix(key, TempUploadPrefix) && !strings.HasPrefix(key, TempExportPrefix) {
			return "", Key{}, fmt.Errorf("%w: temp keys start with %s or %s", ErrInvalidKey, TempUploadPrefix, TempExportPrefix)
		}
		k, err = parseAnyKey(key)
	} else {
		k, err = ParseKey(key)
	}
	if err != nil {
		return "", Key{}, err
	}
	return name, k, nil
}

// PutUpload streams a client's file, received by the owning service, into the TEMP bucket under
// uploadKey — the ninth operation, added by ADR 0052 §Sửa đổi 09/10/2026 (owner decision): uploads
// go through the service, never straight from a device to MinIO. The completion step (b) of the
// package doc — Stat, sniff, malware scan, SHA256, Promote bound to the ETag — runs on what it
// wrote. The returned ETag is the one to bind (b) to.
//
// THE CALLER HAS ALREADY decided, before reading a byte of the body: session / permission (rules 4
// and 5), commune (rule 1), the per-purpose type and size policy (ADR 0052 §10), the per-pod
// concurrency slot and any rate limit. This function only refuses what breaks its own contract:
//   - uploadKey not an `upload/…` key (an `export/…` key, a destination key, a free-form string)
//     → ErrInvalidArgument together with ErrInvalidKey.
//   - contentType off the allow-list → ErrTypeNotAllowed; its extension not the key's →
//     ErrInvalidArgument. contentType is the DECLARED type:
//     it is never trusted downstream, because Promote stores the sniffed one.
//   - maxBytes <= 0 → ErrInvalidArgument. There is no default: the limit is platform policy.
//   - size <= 0 → ErrInvalidArgument (an empty file is not an upload); size > maxBytes →
//     ErrTooLarge (a handler answers 413). Both before any byte is read.
//   - a nil reader → ErrInvalidArgument.
//
// MEMORY: one PUT with the KNOWN size and DisableMultipart, read through a 64 KiB buffer
// (exactBufSize) — never size -1, with which minio-go buffers multipart parts of up to hundreds of
// MiB in a pod whose limit is 384 MiB (ADR 0052 §Sửa đổi, đánh đổi).
//
// r must yield EXACTLY size bytes; more or fewer fails with ErrSizeMismatch (and
// ErrInvalidArgument) while the request body is still one byte short, so the store never commits a
// truncated or padded object. Store errors pass through mapErr. No user metadata is written
// (ADR 0052 §3: the original file name is personal data and lives in stored_file).
//
// The temp key is fresh per upload (a new server-side ObjectID), so no existence check is made; the
// temp lifecycle rule removes anything the caller abandons within a day.
func (c *Client) PutUpload(ctx context.Context, uploadKey string, r io.Reader, size int64, maxBytes int64, contentType string) (ObjectInfo, error) {
	if !strings.HasPrefix(uploadKey, TempUploadPrefix) {
		return ObjectInfo{}, fmt.Errorf("%w: %w: only %s keys are written by PutUpload", ErrInvalidArgument, ErrInvalidKey, TempUploadPrefix)
	}
	name, k, err := c.checkKey(BucketTemp, uploadKey)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}
	ext, ok := ExtForMIME(contentType)
	if !ok {
		return ObjectInfo{}, fmt.Errorf("%w: declared type %q", ErrTypeNotAllowed, contentType)
	}
	if ext != k.Ext {
		return ObjectInfo{}, fmt.Errorf("%w: declared type %q does not match key extension %q", ErrInvalidArgument, contentType, k.Ext)
	}
	if maxBytes <= 0 {
		return ObjectInfo{}, fmt.Errorf("%w: maxBytes must be positive", ErrInvalidArgument)
	}
	if size <= 0 {
		return ObjectInfo{}, fmt.Errorf("%w: size must be positive", ErrInvalidArgument)
	}
	if size > maxBytes {
		return ObjectInfo{}, fmt.Errorf("%w: %d bytes, limit %d", ErrTooLarge, size, maxBytes)
	}
	if r == nil {
		return ObjectInfo{}, fmt.Errorf("%w: reader is nil", ErrInvalidArgument)
	}
	er := &exactReader{br: bufio.NewReaderSize(r, exactBufSize), remaining: size}
	info, err := c.api.PutObject(ctx, name, uploadKey, er, size, minio.PutObjectOptions{
		ContentType: contentType,
		// One request: a failed body can never leave a completed object behind.
		DisableMultipart: true,
	})
	if er.err != nil {
		return ObjectInfo{}, er.err
	}
	if err != nil {
		return ObjectInfo{}, mapErr("put upload", err)
	}
	return ObjectInfo{
		Key: uploadKey, Size: size, ETag: info.ETag, ContentType: contentType,
		VersionID: info.VersionID, LastModified: info.LastModified,
	}, nil
}

// ObjectInfo is what Stat reports.
type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	ContentType  string
	VersionID    string
	LastModified time.Time
}

// Stat reads an object's metadata.
func (c *Client) Stat(ctx context.Context, b Bucket, key string) (ObjectInfo, error) {
	name, _, err := c.checkKey(b, key)
	if err != nil {
		return ObjectInfo{}, err
	}
	st, err := c.api.StatObject(ctx, name, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, mapErr("stat", err)
	}
	return ObjectInfo{
		Key: st.Key, Size: st.Size, ETag: st.ETag, ContentType: st.ContentType,
		VersionID: st.VersionID, LastModified: st.LastModified,
	}, nil
}

// ReadHead returns the first n bytes of an object (fewer if it is shorter) with one range GET,
// for SniffMIME. ifMatchETag, when set, fails with ErrChanged if the object was replaced.
func (c *Client) ReadHead(ctx context.Context, b Bucket, key, ifMatchETag string, n int) ([]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("%w: n must be positive", ErrInvalidArgument)
	}
	name, _, err := c.checkKey(b, key)
	if err != nil {
		return nil, err
	}
	opts := minio.GetObjectOptions{}
	if err := opts.SetRange(0, int64(n)-1); err != nil {
		return nil, fmt.Errorf("storage: range: %w", err)
	}
	if ifMatchETag != "" {
		if err := opts.SetMatchETag(ifMatchETag); err != nil {
			return nil, fmt.Errorf("storage: if-match: %w", err)
		}
	}
	obj, err := c.api.GetObject(ctx, name, key, opts)
	if err != nil {
		return nil, mapErr("read head", err)
	}
	defer obj.Close()
	head, err := io.ReadAll(io.LimitReader(obj, int64(n)))
	if err != nil {
		// A zero-length object has no byte 0 to range over. PutUpload's refusal of size <= 0
		// makes this unreachable for uploads; answer "empty" rather than an error elsewhere.
		if s3Error(err).Code == "InvalidRange" {
			return []byte{}, nil
		}
		return nil, mapErr("read head", err)
	}
	return head, nil
}

// SHA256 streams an object through SHA-256 and returns the lowercase hex digest. Nothing is
// buffered beyond io.Copy's block. ifMatchETag as for ReadHead.
func (c *Client) SHA256(ctx context.Context, b Bucket, key, ifMatchETag string) (string, error) {
	name, _, err := c.checkKey(b, key)
	if err != nil {
		return "", err
	}
	opts := minio.GetObjectOptions{}
	if ifMatchETag != "" {
		if err := opts.SetMatchETag(ifMatchETag); err != nil {
			return "", fmt.Errorf("storage: if-match: %w", err)
		}
	}
	obj, err := c.api.GetObject(ctx, name, key, opts)
	if err != nil {
		return "", mapErr("sha256", err)
	}
	defer obj.Close()
	h := sha256.New()
	if _, err := io.Copy(h, obj); err != nil {
		return "", mapErr("sha256", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Open streams an object for reading in place — the malware scan of the complete step reads the
// temp object through it without buffering (core/malwarescan.Scan takes the reader and the size).
// The caller must Close the reader.
//
// ifMatchETag is REQUIRED, like Promote's: the bytes read here are vouched for (scanned) and then
// promoted, so they must be the bytes of the ETag the caller inspected. A replaced object fails
// with ErrChanged, a missing one with ErrNotFound — both before any byte is returned, because the
// request is made here rather than on the first Read. Errors during Read are mapped the same way.
func (c *Client) Open(ctx context.Context, b Bucket, key, ifMatchETag string) (io.ReadCloser, int64, error) {
	if ifMatchETag == "" {
		return nil, 0, fmt.Errorf("%w: ifMatchETag is required", ErrInvalidArgument)
	}
	name, _, err := c.checkKey(b, key)
	if err != nil {
		return nil, 0, err
	}
	opts := minio.GetObjectOptions{}
	if err := opts.SetMatchETag(ifMatchETag); err != nil {
		return nil, 0, fmt.Errorf("storage: if-match: %w", err)
	}
	obj, err := c.api.GetObject(ctx, name, key, opts)
	if err != nil {
		return nil, 0, mapErr("open", err)
	}
	// minio's GetObject is lazy; Stat sends the conditional GET now, so the precondition and
	// existence are settled before the caller starts streaming.
	st, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, 0, mapErr("open", err)
	}
	return &objectReader{obj: obj}, st.Size, nil
}

// objectReader maps read errors to this package's sentinels, like every other call here.
type objectReader struct{ obj *minio.Object }

func (r *objectReader) Read(p []byte) (int, error) {
	n, err := r.obj.Read(p)
	if err != nil && err != io.EOF {
		return n, mapErr("read", err)
	}
	return n, err
}

func (r *objectReader) Close() error { return r.obj.Close() }

// Promoted describes the object Promote wrote.
type Promoted struct {
	Key         string
	Size        int64
	ETag        string
	VersionID   string
	ContentType string // the SNIFFED type
}

// Promote copies an uploaded temp object to its final key server-side, then deletes the temp
// object. Call it only after the malware scan passed (ADR 0052 §9: nothing leaves temp unscanned).
//
//   - ifMatchETag is REQUIRED: the ETag from the Stat the caller scanned and hashed against. The
//     sniff and the copy are both conditional on it, so an object replaced after inspection fails
//     with ErrChanged instead of being stored unscanned.
//   - The destination must be the upload key's own destination — same class, commune, month,
//     service, purpose, object id and variant. Only the extension may differ, because it follows
//     the sniffed type while the upload key followed the declared one; a mismatch is refused.
//   - The type is sniffed here again and must match dst.Ext; the final object's Content-Type is
//     the sniffed type, never the client's. No user metadata is copied (ADR 0052 §3: the original
//     name lives in stored_file, not on the object).
//   - Only BucketPrivate is accepted. Putting an original into the public bucket is a STOP
//     CONDITION (ADR 0052 §ĐIỀU KIỆN DỪNG #2); publishing an approved derivative is the separate
//     operation PublishDerivative, which copies private → public and never from temp.
//   - An existing destination is refused (ErrExists): objects are immutable.
//
// Single-request server-side copy, so objects up to 5 GiB — above the 2 GB video ceiling of
// ADR 0052 §10.
func (c *Client) Promote(ctx context.Context, srcUploadKey, ifMatchETag string, dst Key, b Bucket) (Promoted, error) {
	if b != BucketPrivate {
		return Promoted{}, fmt.Errorf("%w: uploads are promoted into the private bucket only", ErrInvalidArgument)
	}
	if ifMatchETag == "" {
		return Promoted{}, fmt.Errorf("%w: ifMatchETag is required", ErrInvalidArgument)
	}
	src, err := ParseUploadKey(srcUploadKey)
	if err != nil {
		return Promoted{}, err
	}
	dstKey, err := dst.Path()
	if err != nil {
		return Promoted{}, err
	}
	if !sameObject(src, dst) {
		return Promoted{}, fmt.Errorf("%w: destination is not the upload's own destination", ErrInvalidArgument)
	}
	head, err := c.ReadHead(ctx, BucketTemp, srcUploadKey, ifMatchETag, SniffBytes)
	if err != nil {
		return Promoted{}, err
	}
	mime, ext, ok := SniffMIME(head)
	if !ok {
		return Promoted{}, ErrTypeNotAllowed
	}
	if ext != dst.Ext {
		return Promoted{}, fmt.Errorf("%w: destination extension %q, sniffed %q", ErrInvalidArgument, dst.Ext, ext)
	}
	tempName, err := c.BucketName(BucketTemp)
	if err != nil {
		return Promoted{}, err
	}
	dstName, err := c.BucketName(b)
	if err != nil {
		return Promoted{}, err
	}
	if _, err := c.api.StatObject(ctx, dstName, dstKey, minio.StatObjectOptions{}); err == nil {
		return Promoted{}, ErrExists
	} else if !errors.Is(mapErr("stat", err), ErrNotFound) {
		return Promoted{}, mapErr("stat destination", err)
	}
	if _, err := c.api.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: dstName, Object: dstKey, ContentType: mime, ReplaceMetadata: true},
		minio.CopySrcOptions{Bucket: tempName, Object: srcUploadKey, MatchETag: ifMatchETag},
	); err != nil {
		return Promoted{}, mapErr("copy", err)
	}
	st, err := c.api.StatObject(ctx, dstName, dstKey, minio.StatObjectOptions{})
	if err != nil {
		return Promoted{}, mapErr("stat promoted", err)
	}
	out := Promoted{Key: dstKey, Size: st.Size, ETag: st.ETag, VersionID: st.VersionID, ContentType: mime}
	if err := c.api.RemoveObject(ctx, tempName, srcUploadKey, minio.RemoveObjectOptions{}); err != nil {
		return out, fmt.Errorf("%w: %w", ErrTempCleanup, err)
	}
	return out, nil
}

// sameObject compares everything but the extension.
func sameObject(a, b Key) bool {
	return a.Class == b.Class &&
		strings.EqualFold(a.TenantID, b.TenantID) &&
		a.CreatedAt.UTC().Year() == b.CreatedAt.UTC().Year() &&
		a.CreatedAt.UTC().Month() == b.CreatedAt.UTC().Month() &&
		a.Service == b.Service && a.Purpose == b.Purpose &&
		strings.EqualFold(a.ObjectID, b.ObjectID) && a.Variant == b.Variant
}

// MaxServerProducedBytes caps PutServerProduced. The three flows it serves write re-encoded images,
// and the image ceiling of ADR 0052 §10 is 10 MB for what a client may upload; a server re-encode
// of such an image stays well below 32 MiB. The cap exists so a caller bug (a size taken from the
// wrong variable) fails here instead of streaming an unbounded body. A video transcode (ADR 0052
// §11) is larger than this and is NOT served by this operation — that needs its own decision.
const MaxServerProducedBytes = 32 << 20

// Produced describes the object PutServerProduced wrote: what the caller records in its
// stored_file row. SHA256 is computed over the exact bytes sent, while they were streamed.
type Produced struct {
	Key         string
	Size        int64
	ETag        string
	VersionID   string
	ContentType string // the SNIFFED type
	SHA256      string // lowercase hex
}

// PutServerProduced writes bytes the server itself produced to dst in the PRIVATE bucket. It is
// the only write path in this package whose bytes do not come from a client upload, and it serves
// exactly four flows (a and b: ADR 0047 §6, approved 2026-09-30; c: owner decision 02/10/2026;
// d: ADR 0069, 02/10/2026 — the commune logo `thumb-512.png` and web-admin banner `thumb-1600.jpg`
// of service-platform, class content-source, the same shape as (a) and published the same way):
//
//	a. news cover image: the promoted original is decoded, oriented, resized and re-encoded; the
//	   result is stored as a derivative (`thumb-1280`, class content-source) so PublishDerivative
//	   can publish it — the original itself is never published.
//	b. citizen scene photo: the temp upload is decoded, oriented and re-encoded WITHOUT any EXIF,
//	   and only these clean bytes are stored, as variant `original` of class citizen-media. The raw
//	   upload never reaches the private bucket; the caller then removes it from temp with
//	   PurgeAllVersions(ctx, BucketTemp, uploadKey) (allowed for every class in temp).
//
//	c. staff verification photo (petitions, `petition-verification-photo`, owner decision of
//	   02/10/2026): the same re-encode as (b), WITHOUT EXIF, because the citizen sees this photo on
//	   their own petition and a staff phone's EXIF carries GPS and device ids just as a citizen's does
//	   (rule 3). Stored as variant `original` of class records — see isVerificationPhotoOriginal.
//
// Refused before any byte is read:
//   - class records → ErrInvalidArgument, EXCEPT flow (c). An administrative record is what a person
//     filed; the server never manufactures one, and a re-encoded record would no longer be the
//     record. Flow (c) is the one exception the owner chose, knowingly: the clean re-encode IS the
//     record of that photo, exactly as in (b).
//   - class public-media → ErrInvalidArgument. Only PublishDerivative writes the public bucket.
//   - class content-source with variant `original` → ErrInvalidArgument. There the original is the
//     scanned client upload and arrives only through Promote; letting the server write one would
//     let a derivative silently replace what the commune actually uploaded. citizen-media is the
//     opposite on purpose: in flow (b) the raw upload must NEVER be stored (it carries EXIF — GPS
//     of the citizen's home, device ids; rule 3), so the clean re-encode IS the stored original.
//   - size <= 0 or > MaxServerProducedBytes, a nil reader, an invalid key → ErrInvalidArgument /
//     ErrInvalidKey.
//
// Then the first SniffBytes of r are sniffed: a type off the allow-list is ErrTypeNotAllowed, a
// type whose extension is not dst.Ext is ErrInvalidArgument. The Content-Type stored is the sniffed
// type; no user metadata is written (ADR 0052 §3).
//
// r must yield EXACTLY size bytes. More or fewer fails with ErrInvalidArgument, and the failure is
// raised while the request body is still incomplete, so the store never commits a truncated or
// padded object: the last bytes are only handed to the transport after checking nothing follows.
//
// IMMUTABLE: an existing dst is refused with ErrExists, as in Promote. The check is a stat before
// the write — the same window as Promote's, closed in practice by dst's fresh server-side ObjectID.
//
// Permission, commune, the malware scan of the upload the bytes were derived from (ADR 0052 §9),
// the stored_file row and the audit entry are the caller's (package doc).
func (c *Client) PutServerProduced(ctx context.Context, dst Key, r io.Reader, size int64) (Produced, error) {
	dstKey, err := dst.Path()
	if err != nil {
		return Produced{}, err
	}
	switch {
	case dst.Class == ClassRecords && !isVerificationPhotoOriginal(dst):
		return Produced{}, fmt.Errorf("%w: %s objects are never produced by the server", ErrInvalidArgument, ClassRecords)
	case dst.Class == ClassPublicMedia:
		return Produced{}, fmt.Errorf("%w: only PublishDerivative writes %s", ErrInvalidArgument, ClassPublicMedia)
	case dst.Class == ClassContentSource && dst.Variant == VariantOriginal:
		return Produced{}, fmt.Errorf("%w: a %s original comes only from Promote", ErrInvalidArgument, ClassContentSource)
	}
	if r == nil {
		return Produced{}, fmt.Errorf("%w: reader is nil", ErrInvalidArgument)
	}
	if size <= 0 || size > MaxServerProducedBytes {
		return Produced{}, fmt.Errorf("%w: size must be between 1 and %d bytes", ErrInvalidArgument, MaxServerProducedBytes)
	}
	br := bufio.NewReaderSize(r, exactBufSize)
	head, err := br.Peek(SniffBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return Produced{}, fmt.Errorf("storage: read head: %w", err)
	}
	mime, ext, ok := SniffMIME(head)
	if !ok {
		return Produced{}, ErrTypeNotAllowed
	}
	if ext != dst.Ext {
		return Produced{}, fmt.Errorf("%w: destination extension %q, sniffed %q", ErrInvalidArgument, dst.Ext, ext)
	}
	name, err := c.BucketName(BucketPrivate)
	if err != nil {
		return Produced{}, err
	}
	if _, err := c.api.StatObject(ctx, name, dstKey, minio.StatObjectOptions{}); err == nil {
		return Produced{}, ErrExists
	} else if !errors.Is(mapErr("stat", err), ErrNotFound) {
		return Produced{}, mapErr("stat destination", err)
	}
	er := &exactReader{br: br, remaining: size, h: sha256.New()}
	info, err := c.api.PutObject(ctx, name, dstKey, er, size, minio.PutObjectOptions{
		ContentType: mime,
		// One request: a failed body can then never leave a completed object behind, and the
		// stream is read exactly once, so the hash is the hash of what was sent.
		DisableMultipart: true,
	})
	if er.err != nil {
		return Produced{}, er.err
	}
	if err != nil {
		return Produced{}, mapErr("put", err)
	}
	return Produced{
		Key: dstKey, Size: size, ETag: info.ETag, VersionID: info.VersionID,
		ContentType: mime, SHA256: hex.EncodeToString(er.h.Sum(nil)),
	}, nil
}

// isVerificationPhotoOriginal is flow (c) of PutServerProduced, and the ONLY records key the server
// may produce: service petitions, purpose petition-verification-photo, variant original.
//
// NARROW ON PURPOSE. Widening the records refusal to "any image" would let the server rewrite a
// document scan or a task attachment — records whose bytes are what an officer filed. Every other
// records purpose stays refused (produce_test.go pins it).
//
// ⚠ COST: a records object is never purgeable outside temp (PurgeAllVersions), so a clean copy
// written here and then refused by the caller's transaction stays in the private bucket. The caller
// records that it remains; it is never silently deleted.
func isVerificationPhotoOriginal(k Key) bool {
	return k.Class == ClassRecords && k.Service == ServicePetitions &&
		k.Purpose == PurposePetitionVerificationPhoto && k.Variant == VariantOriginal
}

// exactBufSize is the read-ahead buffer of PutServerProduced; exactWindow is how far before the end
// exactReader starts peeking. exactWindow+1 must fit in exactBufSize (bufio.Reader.Peek).
const (
	exactBufSize = 64 << 10
	exactWindow  = 4 << 10
)

// exactReader yields exactly `remaining` bytes of br, hashing them when h is set, and fails — with
// a sticky ErrInvalidArgument + ErrSizeMismatch — if br is shorter or longer. The length check happens BEFORE the final bytes
// are returned: once the last byte is handed to the transport the store may commit, so "too long"
// has to be known while the body is still one byte short.
type exactReader struct {
	br        *bufio.Reader
	remaining int64
	h         hash.Hash
	err       error
}

func (e *exactReader) Read(p []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	if e.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > e.remaining {
		p = p[:e.remaining]
	}
	if e.remaining <= exactWindow {
		buf, err := e.br.Peek(int(e.remaining) + 1)
		switch {
		case int64(len(buf)) > e.remaining:
			e.err = fmt.Errorf("%w: %w: reader yields more than the declared size", ErrInvalidArgument, ErrSizeMismatch)
			return 0, e.err
		case err != nil && !errors.Is(err, io.EOF):
			e.err = fmt.Errorf("storage: read: %w", err)
			return 0, e.err
		case int64(len(buf)) < e.remaining:
			e.err = fmt.Errorf("%w: %w: reader yields less than the declared size", ErrInvalidArgument, ErrSizeMismatch)
			return 0, e.err
		}
	} else if limit := e.remaining - exactWindow; int64(len(p)) > limit {
		p = p[:limit] // never step into the final window without the peek above
	}
	n, err := e.br.Read(p)
	if e.h != nil { // PutUpload does not hash: step (b) hashes what the store holds
		e.h.Write(p[:n])
	}
	e.remaining -= int64(n)
	switch {
	case err == nil:
		return n, nil
	case errors.Is(err, io.EOF) && e.remaining == 0:
		return n, nil
	case errors.Is(err, io.EOF):
		e.err = fmt.Errorf("%w: %w: reader yields less than the declared size", ErrInvalidArgument, ErrSizeMismatch)
	default:
		e.err = fmt.Errorf("storage: read: %w", err)
	}
	return n, e.err
}

// PresignedURL is a presigned GET. A BEARER CREDENTIAL for its TTL: fmt and slog render "***";
// it is a string kind without MarshalJSON, so encoding/json writes the real URL — which is how a
// handler returns it. URL() hands it over explicitly.
type PresignedURL string

const redactedURL = "storage.PresignedURL{***}"

func (u PresignedURL) URL() string                   { return string(u) }
func (u PresignedURL) String() string                { return redactedURL }
func (u PresignedURL) GoString() string              { return redactedURL }
func (u PresignedURL) LogValue() slog.Value          { return slog.StringValue(redactedURL) }
func (u PresignedURL) Format(f fmt.State, verb rune) { _, _ = io.WriteString(f, redactedURL) }

// PresignDownload issues a presigned GET signed against the PUBLIC endpoint, offline.
//
// The response type and disposition are forced through the signed query: the type is the one
// the key's extension stands for (the extension came from the sniffed type), and the disposition
// is `inline` for media, `attachment` otherwise, with filename sanitised (ContentDisposition).
// filename is the stored original name — personal data; it is never logged here and must not be
// logged by the caller. ttl must be in (0, MaxDownloadTTL]; the caller chooses, there is no default.
func (c *Client) PresignDownload(ctx context.Context, b Bucket, key string, ttl time.Duration, filename string) (PresignedURL, error) {
	name, k, err := c.checkKey(b, key)
	if err != nil {
		return "", err
	}
	if ttl < time.Second || ttl > MaxDownloadTTL {
		return "", fmt.Errorf("%w: download ttl must be between 1s and %s", ErrInvalidArgument, MaxDownloadTTL)
	}
	mime := mimeByExt[k.Ext]
	params := url.Values{}
	params.Set("response-content-type", mime)
	params.Set("response-content-disposition", ContentDisposition(filename, mime))
	u, err := c.signer.PresignedGetObject(ctx, name, key, ttl, params)
	if err != nil {
		return "", fmt.Errorf("storage: presign download: %w", err)
	}
	return PresignedURL(u.String()), nil
}

// PublicURL is the anonymous URL of an object in the public bucket. Only `public-media` keys
// live there (ADR 0052 §2), so any other class is refused.
func (c *Client) PublicURL(key string) (string, error) {
	if c.mediaBase == "" {
		return "", fmt.Errorf("%w: OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL is empty", ErrNotConfigured)
	}
	k, err := ParseKey(key)
	if err != nil {
		return "", err
	}
	if k.Class != ClassPublicMedia {
		return "", fmt.Errorf("%w: only %s objects are public", ErrInvalidArgument, ClassPublicMedia)
	}
	return c.mediaBase + "/" + key, nil
}

// PublicCacheControl is set on every published object. ADR 0052 §4 names `Cache-Control:
// immutable` for the public media base URL; `immutable` only means something within a freshness
// lifetime, so it comes with the conventional one-year max-age. Sound because a public key never
// changes content: dst is derived 1:1 from an immutable private object (ADR 0052 §1).
//
// THE COST, for UnpublishDerivative: a CDN or browser that already fetched the object may keep
// serving it for up to that year after the origin copy is gone. Taking content down from caches
// is a CDN purge — outside this library and not yet provisioned.
const PublicCacheControl = "public, max-age=31536000, immutable"

// PublishDerivative copies an approved derivative from the private bucket to the public bucket,
// server-side. ADR 0052 §11: publishing copies the derivative; the original stays private.
//
// src is a key in the PRIVATE bucket; dst is its public twin and must be src with exactly one
// change — Class becomes public-media. Same commune (rule 1), service, purpose, month, object id,
// variant and extension. Requiring the twin instead of accepting any public-media key means a
// public key always names the private object it came from, and one service cannot publish into
// another commune's or another service's public tree.
//
// Refused before any network call:
//   - src of class citizen-media → ErrNotPublishable. Citizen media never becomes public,
//     derivative or not (ADR 0052 §ĐIỀU KIỆN DỪNG #2, rule 4).
//   - src variant `original` → ErrNotPublishable. Only derivatives are published (same stop).
//   - src of class public-media (that class lives only in the public bucket), dst of any class but
//     public-media, or dst not src's twin → ErrInvalidArgument.
//
// WHETHER THE DERIVATIVE IS APPROVED is the caller's decision, as are permission, commune and the
// audit entry (package doc). This function only refuses what can never be public.
//
// IDEMPOTENT: a dst that already exists returns nil without copying. Safe because only this
// function writes the public bucket and dst is fully derived from an immutable src — so an
// existing dst IS this derivative. A caller whose business transaction failed after the copy can
// therefore just retry.
//
// The public object gets the Content-Type its extension stands for (the extension came from the
// sniffed type), `inline`/`attachment` per ADR 0052 §4 with no file name (anonymous GETs carry no
// response overrides, so the disposition has to live on the object), PublicCacheControl, and no
// user metadata.
func (c *Client) PublishDerivative(ctx context.Context, src, dst Key) error {
	if src.Class == ClassCitizenMedia {
		return fmt.Errorf("%w: %s is never public", ErrNotPublishable, ClassCitizenMedia)
	}
	// Only what the commune itself publishes may go public. `records` are administrative records
	// (documents, attachments to tasks) — nothing in them is meant for an anonymous reader, so a
	// derivative of one reaching the public bucket would be a leak with a legal record behind it.
	if src.Class == ClassRecords {
		return fmt.Errorf("%w: %s is never public", ErrNotPublishable, ClassRecords)
	}
	if src.Variant == VariantOriginal {
		return fmt.Errorf("%w: an original is never public, only an approved derivative", ErrNotPublishable)
	}
	if src.Class == ClassPublicMedia {
		return fmt.Errorf("%w: source must be a private object, not %s", ErrInvalidArgument, ClassPublicMedia)
	}
	if dst.Class != ClassPublicMedia {
		return fmt.Errorf("%w: destination class must be %s", ErrInvalidArgument, ClassPublicMedia)
	}
	twin := src
	twin.Class = ClassPublicMedia
	if !sameObject(twin, dst) || twin.Ext != dst.Ext {
		return fmt.Errorf("%w: destination is not the source's public twin", ErrInvalidArgument)
	}
	srcKey, err := src.Path()
	if err != nil {
		return err
	}
	dstKey, err := dst.Path()
	if err != nil {
		return err
	}
	privName, err := c.BucketName(BucketPrivate)
	if err != nil {
		return err
	}
	pubName, err := c.BucketName(BucketPublic)
	if err != nil {
		return err
	}
	if _, err := c.api.StatObject(ctx, pubName, dstKey, minio.StatObjectOptions{}); err == nil {
		return nil
	} else if !errors.Is(mapErr("stat", err), ErrNotFound) {
		return mapErr("stat public", err)
	}
	mime := mimeByExt[dst.Ext]
	if _, err := c.api.CopyObject(ctx,
		minio.CopyDestOptions{
			Bucket: pubName, Object: dstKey, ReplaceMetadata: true,
			ContentType:        mime,
			ContentDisposition: ContentDisposition("", mime),
			CacheControl:       PublicCacheControl,
		},
		minio.CopySrcOptions{Bucket: privName, Object: srcKey},
	); err != nil {
		return mapErr("publish", err)
	}
	return nil
}

// UnpublishDerivative removes a published object from the public bucket — every version, so a
// bucket whose versioning was switched on against ADR 0052 §2 still loses the bytes. The private
// source is untouched (ADR 0052 §11: the original always stays private).
//
// IDEMPOTENT: an object already gone returns nil. Only public-media keys are accepted. Caches
// that already hold the object keep it until PublicCacheControl expires — see that constant.
func (c *Client) UnpublishDerivative(ctx context.Context, dst Key) error {
	if dst.Class != ClassPublicMedia {
		return fmt.Errorf("%w: only %s objects are unpublished", ErrInvalidArgument, ClassPublicMedia)
	}
	dstKey, err := dst.Path()
	if err != nil {
		return err
	}
	return c.PurgeAllVersions(ctx, BucketPublic, dstKey)
}

// PurgeAllVersions deletes every version and delete marker of one key. In the versioned private
// bucket a plain delete only adds a marker and the bytes stay (ADR 0052 §Cái giá), so a purge
// that is meant to remove data has to name each version.
//
// IDEMPOTENT: a key with no versions left returns nil, so a purge worker can retry freely.
// `records` objects are refused outside the temp bucket (ErrRecordsNotPurgeable). Whether a purge
// is due — retain_until, legal_hold — is the caller's decision; this function only executes it.
func (c *Client) PurgeAllVersions(ctx context.Context, b Bucket, key string) error {
	name, k, err := c.checkKey(b, key)
	if err != nil {
		return err
	}
	if b != BucketTemp && k.Class == ClassRecords {
		return ErrRecordsNotPurgeable
	}
	for obj := range c.api.ListObjectsIter(ctx, name, minio.ListObjectsOptions{Prefix: key, WithVersions: true}) {
		if obj.Err != nil {
			return mapErr("list versions", obj.Err)
		}
		if obj.Key != key {
			continue // a longer key sharing the prefix
		}
		err := c.api.RemoveObject(ctx, name, key, minio.RemoveObjectOptions{VersionID: obj.VersionID})
		if err != nil && !errors.Is(mapErr("purge", err), ErrNotFound) {
			return mapErr("purge", err)
		}
	}
	return nil
}

// mapErr turns S3 error codes into this package's sentinels, keeping the original wrapped.
//
// A MISSING BUCKET IS NOT ErrNotFound: it is a misconfigured prefix, and treating it as "object
// already gone" would let a purge worker mark every file purged against a bucket that never existed.
func mapErr(op string, err error) error {
	resp := s3Error(err)
	switch {
	case resp.Code == "NoSuchBucket":
		// The NAME, so the log says which of `{prefix}-private|-public|-temp` is missing: the operator's
		// fix is one `mc mb` of exactly that bucket, and "a bucket" sends them to read the ConfigMap.
		return fmt.Errorf("storage: %s: bucket %q does not exist: %w", op, resp.BucketName, err)
	case resp.Code == "NoSuchKey" || resp.Code == "NoSuchVersion" || resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("storage: %s: %w: %w", op, ErrNotFound, err)
	case resp.Code == "PreconditionFailed" || resp.StatusCode == http.StatusPreconditionFailed:
		return fmt.Errorf("storage: %s: %w: %w", op, ErrChanged, err)
	}
	return fmt.Errorf("storage: %s: %w", op, err)
}

// s3Error unwraps to minio's ErrorResponse. minio.ToErrorResponse only type-asserts, so an error
// wrapped once on its way here would read as "no code" and fall through every case above.
func s3Error(err error) minio.ErrorResponse {
	var resp minio.ErrorResponse
	if errors.As(err, &resp) {
		return resp
	}
	return minio.ErrorResponse{}
}
