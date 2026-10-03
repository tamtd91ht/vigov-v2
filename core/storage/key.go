package storage

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/ulid"
)

// Class is the first key segment: which retention class an object belongs to (ADR 0052 §3, §6).
// It leads the key so a per-class operation is one prefix inside one bucket. CLOSED LIST —
// adding a class is adding a retention rule, which is the customer's decision (ADR 0052 Còn mở #1).
type Class string

const (
	ClassRecords       Class = "records"        // administrative records, scans — never purged automatically
	ClassCitizenMedia  Class = "citizen-media"  // files a citizen uploads (petition photos)
	ClassContentSource Class = "content-source" // source files of Mini App content
	ClassPublicMedia   Class = "public-media"   // approved derivatives served from the public bucket
)

var knownClasses = map[Class]bool{
	ClassRecords: true, ClassCitizenMedia: true, ClassContentSource: true, ClassPublicMedia: true,
}

// Service is the owning service segment. Each service's MinIO access key is scoped by IAM to
// `…/{service}/*` (ADR 0052 §3), so this segment is an authorisation boundary, not a label.
type Service string

const (
	ServiceIdentity  Service = "identity"
	ServicePlatform  Service = "platform"
	ServiceDocuments Service = "documents"
	ServicePetitions Service = "petitions"
	ServiceFinance   Service = "finance"
	ServiceComms     Service = "comms"
	ServiceReporting Service = "reporting"
)

var knownServices = map[Service]bool{
	ServiceIdentity: true, ServicePlatform: true, ServiceDocuments: true, ServicePetitions: true,
	ServiceFinance: true, ServiceComms: true, ServiceReporting: true,
}

// Purpose says what the file is for inside its service. CLOSED LIST, and one that must NEVER
// contain a service name: an IAM wildcard `*` also matches `/`, so `…/*/comms/*` would match a
// key whose purpose is `comms` in another service's tree (ADR 0052 §Cái giá). TestClosedListsDoNotCollide
// enforces it for the constants and Key.validate enforces it again at runtime.
//
// The list holds the needs ADR 0052 names, and nothing speculative. A new purpose is one line
// here plus its size/type limit in platform (ADR 0052 §10).
type Purpose string

const (
	PurposeContentVideo      Purpose = "content-video"      // Mini App content, type video (comms)
	PurposeContentImage      Purpose = "content-image"      // Mini App content cover image (comms)
	PurposeContentAttachment Purpose = "content-attachment" // Mini App content attachment (comms)
	PurposeTenantLogo        Purpose = "tenant-logo"        // commune display-profile logo (platform)
	PurposePetitionPhoto     Purpose = "petition-photo"     // scene photo on a petition (petitions) — blocked on ADR 0052 §12
	PurposeDocumentScan      Purpose = "document-scan"      // scan of an administrative document (documents)
	// File attached to a task log entry (petitions; Nhiệm vụ §5.9 "Đính kèm"). Staff-uploaded work
	// evidence on an administrative task, so its key class is ClassRecords — never auto-purged.
	PurposeTaskAttachment Purpose = "task-attachment"
	// The broadcast audio of a Mini App `truyen-thanh` item (comms; ADR 0067 §4). Class
	// content-source, private bucket only: there is no public variant of audio (no transcoder yet,
	// ADR 0067 Còn mở #2), so it is delivered by a short-lived presigned GET, never a public key.
	PurposeContentAudio Purpose = "content-audio"
	// The "after processing" photo staff upload on a petition (petitions; docs/ui-ux/09 §8.4 "SAU KHI
	// XỬ LÝ", owner decision C of 02/10/2026). "verification" is the glossary's English for nghiệm thu
	// (kb/00-foundation/ubiquitous-language.md), the act this photo evidences and the name of the
	// per-commune switch that makes it mandatory. Staff-uploaded evidence that a public authority did
	// what it committed to, so its key class is ClassRecords like PurposeTaskAttachment — never
	// auto-purged, never public. NOT PurposePetitionPhoto: that is the citizen's, with its own count.
	PurposePetitionVerificationPhoto Purpose = "petition-verification-photo"
	// A file attached to a petition processing-log entry (petitions; docs/ui-ux/09 :197, :312, owner
	// decision B of 02/10/2026). Staff-only, never shown to a citizen. The petition counterpart of
	// PurposeTaskAttachment, and in its class for its reason: ClassRecords.
	PurposePetitionLogAttachment Purpose = "petition-log-attachment"
	// The commune's web-admin banner (platform; ADR 0069 #5): the strip under the topbar on every
	// web-admin page, uploaded by the commune itself. Its own purpose, not PurposeTenantLogo, because
	// its limits and normalisation differ (1600px wide vs a 512px square) and the platform policy row
	// is per purpose. NOT the Mini App `banner` content of comms (ADR 0067 §5): that one is
	// PurposeContentImage under comms, for citizens; ADR 0069 #6 keeps the two apart.
	PurposeTenantBanner Purpose = "tenant-banner"
	// An image inside a Mini App article body (comms; ADR 0067 amendment 03/10/2026, K3). Its own
	// purpose, not PurposeContentImage, because the platform policy row is per purpose and the counts
	// differ: one cover per item, up to 20 body images. Same class as the cover — ClassContentSource,
	// private original, public derivative.
	PurposeContentBodyImage Purpose = "content-body-image"
)

var knownPurposes = map[Purpose]bool{
	PurposeContentVideo: true, PurposeContentImage: true, PurposeContentAttachment: true,
	PurposeTenantLogo: true, PurposePetitionPhoto: true, PurposeDocumentScan: true,
	PurposeTaskAttachment: true, PurposeContentAudio: true,
	PurposePetitionVerificationPhoto: true, PurposePetitionLogAttachment: true,
	PurposeTenantBanner: true, PurposeContentBodyImage: true,
}

// Purposes returns the closed list, sorted, as a fresh slice the caller may keep.
//
// For code that meets a purpose it did not build from a constant: core/platformclient/uploadpolicy
// derives one from the platform's UploadPurpose enum and must refuse a spelling this build does not
// hold, and its drift test compares the two lists in both directions (platform.proto, UploadPurpose).
func Purposes() []Purpose {
	out := make([]Purpose, 0, len(knownPurposes))
	for p := range knownPurposes {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// Variants (ADR 0052 §3: `original`, `mp4-720p`, `poster`, `thumb-320`…). CLOSED, not merely
// `[a-z0-9-]+`: an open variant accepts a slugified file name (`don-nguyen-van-a`) and puts a
// person's name back into the key that §3 keeps names out of (rule 3 forbidden #4). The two
// families with a size in them are matched by exact shape: `mp4-<height>p`, `thumb-<width>`.
const (
	VariantOriginal = "original"
	VariantPoster   = "poster"
)

var variantPattern = regexp.MustCompile(`^(original|poster|mp4-[0-9]{3,4}p|thumb-[0-9]{2,4})$`)

// Temp-bucket prefixes. The MinIO lifecycle rules of the temp bucket are written against these
// two strings (1 day and 7 days, ADR 0052 §2); renaming one silently disables its expiry.
const (
	TempUploadPrefix = "upload/"
	TempExportPrefix = "export/"
)

// ErrInvalidKey is any key or key component that does not satisfy ADR 0052 §3.
var ErrInvalidKey = errors.New("storage: invalid object key")

// Key is the structured form of `{class}/t_{tenant_id}/{yyyy}/{mm}/{service}/{purpose}/{object_id}/{variant}.{ext}`.
//
// TENANT AND OBJECT IDS ARE ULIDs, HELD HERE IN THEIR CANONICAL UPPERCASE FORM AND WRITTEN
// LOWERCASE IN THE KEY. ADR 0052 §3 allows only `[a-z0-9_-./]` in a key, and Crockford base32
// decodes case-insensitively by specification, so lowercasing loses nothing. ParseKey returns
// the uppercase form, so a parsed TenantID compares equal to the tenant_id in the context
// (rule 1) without the caller remembering to fold case.
//
// The original file name is NEVER part of a key: a user-chosen name can carry a person's name
// (rule 3 forbidden #4, ADR 0052 §3). It lives in stored_file and in Content-Disposition only.
type Key struct {
	Class     Class
	TenantID  string    // ULID, any case on input
	CreatedAt time.Time // month of object creation, UTC — not a retention anchor
	Service   Service
	Purpose   Purpose
	ObjectID  string // ULID generated server-side — NewObjectID
	Variant   string
	Ext       string // from the SNIFFED type (SniffMIME), never from a file name
}

// NewObjectID returns a fresh server-side ULID for Key.ObjectID. The error is fatal to the
// upload: see ulid.Moi.
func NewObjectID() (string, error) {
	id, err := ulid.Moi()
	if err != nil {
		return "", fmt.Errorf("storage: object id: %w", err)
	}
	return id, nil
}

// Path renders the key after validating every segment.
func (k Key) Path() (string, error) {
	if err := k.validate(); err != nil {
		return "", err
	}
	t := k.CreatedAt.UTC()
	return fmt.Sprintf("%s/t_%s/%04d/%02d/%s/%s/%s/%s.%s",
		k.Class, strings.ToLower(k.TenantID), t.Year(), int(t.Month()),
		k.Service, k.Purpose, strings.ToLower(k.ObjectID), k.Variant, k.Ext), nil
}

// UploadPath is the temp-bucket key a presigned upload writes to: `upload/` + the destination
// key. ADR 0052 §2 requires the full destination shape after the prefix, so the per-service IAM
// scope `…/{service}/*` applies in the temp bucket too.
func (k Key) UploadPath() (string, error) {
	p, err := k.Path()
	if err != nil {
		return "", err
	}
	return TempUploadPrefix + p, nil
}

// ExportPath is the temp-bucket key of an export file: `export/` + the key shape, for the same
// IAM reason as UploadPath.
func (k Key) ExportPath() (string, error) {
	p, err := k.Path()
	if err != nil {
		return "", err
	}
	return TempExportPrefix + p, nil
}

func (k Key) validate() error {
	if !knownClasses[k.Class] {
		return fmt.Errorf("%w: unknown class %q", ErrInvalidKey, k.Class)
	}
	if !isULID(k.TenantID) {
		return fmt.Errorf("%w: tenant id is not a ULID", ErrInvalidKey)
	}
	if k.CreatedAt.IsZero() {
		return fmt.Errorf("%w: creation time is zero", ErrInvalidKey)
	}
	if y := k.CreatedAt.UTC().Year(); y < 2000 || y > 9999 {
		return fmt.Errorf("%w: creation year %d out of range", ErrInvalidKey, y)
	}
	if !knownServices[k.Service] {
		return fmt.Errorf("%w: unknown service %q", ErrInvalidKey, k.Service)
	}
	if !knownPurposes[k.Purpose] {
		return fmt.Errorf("%w: unknown purpose %q", ErrInvalidKey, k.Purpose)
	}
	if knownServices[Service(k.Purpose)] {
		return fmt.Errorf("%w: purpose %q equals a service name (IAM wildcard, ADR 0052)", ErrInvalidKey, k.Purpose)
	}
	if !isULID(k.ObjectID) {
		return fmt.Errorf("%w: object id is not a ULID", ErrInvalidKey)
	}
	if !variantPattern.MatchString(k.Variant) {
		return fmt.Errorf("%w: variant %q is not original, poster, mp4-<n>p or thumb-<n>", ErrInvalidKey, k.Variant)
	}
	if _, ok := mimeByExt[k.Ext]; !ok {
		return fmt.Errorf("%w: extension %q is not an allowed sniffed type", ErrInvalidKey, k.Ext)
	}
	return nil
}

// ParseKey reads a destination key back (purge, operations), with the same validation Path
// applies. Anything Path could not have produced is refused.
func ParseKey(s string) (Key, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 8 {
		return Key{}, fmt.Errorf("%w: expected 8 segments, got %d", ErrInvalidKey, len(parts))
	}
	tenant, ok := strings.CutPrefix(parts[1], "t_")
	if !ok || tenant != strings.ToLower(tenant) {
		return Key{}, fmt.Errorf("%w: tenant segment must be t_<lowercase ulid>", ErrInvalidKey)
	}
	year, err := fixedInt(parts[2], 4)
	if err != nil {
		return Key{}, err
	}
	month, err := fixedInt(parts[3], 2)
	if err != nil {
		return Key{}, err
	}
	if month < 1 || month > 12 {
		return Key{}, fmt.Errorf("%w: month %d", ErrInvalidKey, month)
	}
	if parts[6] != strings.ToLower(parts[6]) {
		return Key{}, fmt.Errorf("%w: object id must be lowercase in a key", ErrInvalidKey)
	}
	variant, ext, ok := strings.Cut(parts[7], ".")
	if !ok || strings.Contains(ext, ".") {
		return Key{}, fmt.Errorf("%w: last segment must be <variant>.<ext>", ErrInvalidKey)
	}
	k := Key{
		Class:     Class(parts[0]),
		TenantID:  strings.ToUpper(tenant),
		CreatedAt: time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC),
		Service:   Service(parts[4]),
		Purpose:   Purpose(parts[5]),
		ObjectID:  strings.ToUpper(parts[6]),
		Variant:   variant,
		Ext:       ext,
	}
	if err := k.validate(); err != nil {
		return Key{}, err
	}
	// Round-trip: rejects anything that validates but would not render identically.
	if p, _ := k.Path(); p != s {
		return Key{}, fmt.Errorf("%w: key is not in canonical form", ErrInvalidKey)
	}
	return k, nil
}

// ParseUploadKey reads an `upload/…` temp key back into its destination Key.
func ParseUploadKey(s string) (Key, error) {
	rest, ok := strings.CutPrefix(s, TempUploadPrefix)
	if !ok {
		return Key{}, fmt.Errorf("%w: not an %s key", ErrInvalidKey, TempUploadPrefix)
	}
	return ParseKey(rest)
}

// ParseExportKey reads an `export/…` temp key back into its Key.
func ParseExportKey(s string) (Key, error) {
	rest, ok := strings.CutPrefix(s, TempExportPrefix)
	if !ok {
		return Key{}, fmt.Errorf("%w: not an %s key", ErrInvalidKey, TempExportPrefix)
	}
	return ParseKey(rest)
}

// parseAnyKey accepts a destination key, an upload key or an export key — the three shapes
// that exist in the three buckets.
func parseAnyKey(s string) (Key, error) {
	switch {
	case strings.HasPrefix(s, TempUploadPrefix):
		return ParseUploadKey(s)
	case strings.HasPrefix(s, TempExportPrefix):
		return ParseExportKey(s)
	default:
		return ParseKey(s)
	}
}

func fixedInt(s string, width int) (int, error) {
	if len(s) != width {
		return 0, fmt.Errorf("%w: %q must be %d digits", ErrInvalidKey, s, width)
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("%w: %q must be digits", ErrInvalidKey, s)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %q: %w", ErrInvalidKey, s, err)
	}
	return n, nil
}

// isSegment is `[a-z0-9-]+`.
func isSegment(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// crockford is the ULID alphabet (core/ulid), accepted in either case.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// isULID accepts a 26-character Crockford ULID in either case whose first character is at most
// '7' (a larger one overflows 128 bits).
func isULID(s string) bool {
	if len(s) != ulid.Do {
		return false
	}
	up := strings.ToUpper(s)
	if up[0] > '7' {
		return false
	}
	for i := 0; i < len(up); i++ {
		if !strings.ContainsRune(crockford, rune(up[i])) {
			return false
		}
	}
	return true
}
