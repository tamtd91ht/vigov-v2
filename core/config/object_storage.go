package config

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/vihat/vigov/core/secret"
)

// ObjectStorage is the S3/MinIO connection of ADR 0052. core/storage.New consumes it.
//
// READ ONLY BY A SERVICE THAT DECLARES config.ObjectStore (and config.PublicMedia for
// PublicMediaBaseURL) — petitions today. Every other service never reads these values, so it is
// never stopped by them. A DECLARING service is refused in staging/prod without Endpoint,
// PublicEndpoint, the key pair and BucketPrefix (Region has a default); in dev they may be absent.
//
// WHAT "ABSENT IN DEV" DOES NOT MEAN: a default. Absent settings make storage.New return
// storage.ErrNotConfigured, and the caller refuses the upload (fail closed). Half the settings
// present is reported by CanhBao at every startup — the usual cause is a typo'd key in the
// ConfigMap or Secret, and the named refusal at the first upload is too late to be the only
// signal.
//
// A MALFORMED VALUE IS FATAL AT Load, unlike an absent one — the TRUSTED_PROXY_CIDRS
// precedent. An operator who wrote a value meant something; skipping it silently turns "I
// configured storage" into "uploads refused" with nothing pointing at the typo.
type ObjectStorage struct {
	// Endpoint is the S3 endpoint this process calls, inside the cluster: OBJECT_STORAGE_ENDPOINT,
	// a URL whose scheme decides TLS (`https://minio.internal:9000`). k8s CONFIGMAP.
	//
	// CLUSTER-SHAPED BUT EXACTLY ONE HOST (rule 11 invariant 5, forbidden #5). The variable is
	// parsed as a comma-separated list like every other address, and Load REFUSES more than one
	// entry, by name. The S3 client talks to one endpoint; distributed MinIO is reached through a
	// load balancer in front of it. Silently keeping the first host would look correct until the
	// day that host is drained (ADR 0052 §Hệ quả).
	//
	// No scheme variable (no OBJECT_STORAGE_USE_TLS): the scheme is already in the URL, and a
	// second source for the same fact is one that drifts (rule 9).
	Endpoint string

	// PublicEndpoint is the S3 endpoint a BROWSER sees: OBJECT_STORAGE_PUBLIC_ENDPOINT. Presigned
	// POST and GET are signed against this host, never Endpoint — the SigV4 signature covers the
	// host, and a URL signed for the internal name answers 403 to every browser (ADR 0052 §4).
	// Same shape and same single-host refusal as Endpoint. k8s CONFIGMAP.
	PublicEndpoint string

	// PublicMediaBaseURL is the base URL of the public bucket, for Mini App media:
	// OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL (a CDN may sit in front). May carry a path.
	// k8s CONFIGMAP.
	//
	// OPTIONAL EVEN WHEN THE REST IS SET: only the service that publishes derivatives (comms)
	// needs it. Absent means storage.PublicURL refuses with ErrNotConfigured.
	PublicMediaBaseURL string

	// AccessKey / SecretKey: OBJECT_STORAGE_ACCESS_KEY, OBJECT_STORAGE_SECRET_KEY. k8s SECRET,
	// one pair PER SERVICE (ADR 0052 §3: each service's IAM policy is scoped to `…/{service}/*`).
	// secret.Secret on both — the access key alone is not a credential, but pairing it with the
	// right secret key in an incident is half the work, and one type for the pair keeps it simple.
	AccessKey secret.Secret
	SecretKey secret.Secret

	// Region is OBJECT_STORAGE_REGION. k8s CONFIGMAP. DEFAULT "us-east-1", and the default is the
	// one place this struct has one, for a stated reason: minio-go only presigns OFFLINE when a
	// region is set — otherwise every presign starts with a GetBucketLocation call to the PUBLIC
	// endpoint, which from inside the cluster may not even resolve. "us-east-1" is what MinIO
	// itself uses when MINIO_SITE_REGION is unset. A wrong value fails closed: every signature is
	// rejected with 403, nothing is accepted that should not be.
	Region string

	// BucketPrefix is OBJECT_STORAGE_BUCKET_PREFIX: the three bucket names are
	// `{prefix}-private`, `{prefix}-public`, `{prefix}-temp` (ADR 0052 §2 — `vigov-prod`,
	// `vigov-stg`). k8s CONFIGMAP. One variable and not three bucket names, because ADR 0052 fixes
	// the suffixes; three variables would allow a deployment where "private" points at the public
	// bucket.
	BucketPrefix string
}

// ObjectStorageDefaultRegion is OBJECT_STORAGE_REGION when unset — see ObjectStorage.Region.
const ObjectStorageDefaultRegion = "us-east-1"

// ErrObjectStorageInvalid is a malformed object-storage value. Load refuses to start.
var ErrObjectStorageInvalid = errors.New("config: object storage setting is invalid")

// bucketPrefixPattern keeps `{prefix}-private` a valid S3 bucket name: lowercase letters,
// digits and hyphens, starting and ending with a letter or digit.
var bucketPrefixPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// maxBucketPrefix leaves room for the longest suffix, "-private" (8 characters), inside the
// 63-character S3 bucket-name limit.
const maxBucketPrefix = 63 - len("-private")

// Missing lists the settings storage.New cannot work without, by variable name. Empty means
// complete. PublicMediaBaseURL and Region are not in it: one is needed only by a publisher,
// the other has a default.
func (o ObjectStorage) Missing() []string {
	var out []string
	if o.Endpoint == "" {
		out = append(out, "OBJECT_STORAGE_ENDPOINT")
	}
	if o.PublicEndpoint == "" {
		out = append(out, "OBJECT_STORAGE_PUBLIC_ENDPOINT")
	}
	if o.AccessKey.Rong() {
		out = append(out, "OBJECT_STORAGE_ACCESS_KEY")
	}
	if o.SecretKey.Rong() {
		out = append(out, "OBJECT_STORAGE_SECRET_KEY")
	}
	if o.BucketPrefix == "" {
		out = append(out, "OBJECT_STORAGE_BUCKET_PREFIX")
	}
	return out
}

// Configured reports whether every setting storage.New needs is present.
func (o ObjectStorage) Configured() bool { return len(o.Missing()) == 0 }

// untouched reports whether nothing object-storage related was set at all — the state that
// deserves silence, as opposed to a half-configuration that deserves a warning.
func (o ObjectStorage) untouched() bool {
	return len(o.Missing()) == 5 && o.PublicMediaBaseURL == ""
}

// String never renders the keys. secret.Secret already refuses to, this only keeps a %v of
// the whole struct readable.
func (o ObjectStorage) String() string {
	return fmt.Sprintf("ObjectStorage{Endpoint:%s PublicEndpoint:%s PublicMediaBaseURL:%s "+
		"AccessKey:%s SecretKey:%s Region:%s BucketPrefix:%s}",
		o.Endpoint, o.PublicEndpoint, o.PublicMediaBaseURL,
		o.AccessKey, o.SecretKey, o.Region, o.BucketPrefix)
}

// parseObjectStorage validates the raw values read by Load. Every error names the variable and
// never a credential.
func parseObjectStorage(endpoint, publicEndpoint, mediaBase, accessKey, secretKey, region, prefix string) (ObjectStorage, error) {
	ep, err := ParseSingleEndpoint("OBJECT_STORAGE_ENDPOINT", endpoint)
	if err != nil {
		return ObjectStorage{}, err
	}
	pub, err := ParseSingleEndpoint("OBJECT_STORAGE_PUBLIC_ENDPOINT", publicEndpoint)
	if err != nil {
		return ObjectStorage{}, err
	}
	media, err := ParsePublicMediaBaseURL(mediaBase)
	if err != nil {
		return ObjectStorage{}, err
	}
	prefix = strings.TrimSpace(prefix)
	if prefix != "" {
		if err := CheckBucketPrefix(prefix); err != nil {
			return ObjectStorage{}, err
		}
	}
	region = strings.TrimSpace(region)
	if region == "" {
		region = ObjectStorageDefaultRegion
	}
	return ObjectStorage{
		Endpoint:           ep,
		PublicEndpoint:     pub,
		PublicMediaBaseURL: media,
		// Trimmed: a trailing newline pasted into a Secret makes every signature wrong, and
		// the 403 it produces names nothing.
		AccessKey:    secret.Secret(strings.TrimSpace(accessKey)),
		SecretKey:    secret.Secret(strings.TrimSpace(secretKey)),
		Region:       region,
		BucketPrefix: prefix,
	}, nil
}

// CheckBucketPrefix validates OBJECT_STORAGE_BUCKET_PREFIX so that `{prefix}-private` is a
// valid S3 bucket name. Exported for core/storage, same reason as ParseSingleEndpoint.
func CheckBucketPrefix(prefix string) error {
	if !bucketPrefixPattern.MatchString(prefix) || len(prefix) > maxBucketPrefix {
		return fmt.Errorf("%w: OBJECT_STORAGE_BUCKET_PREFIX %q must be lowercase letters, digits "+
			"and hyphens, at most %d characters (bucket names are {prefix}-private, -public, -temp)",
			ErrObjectStorageInvalid, prefix, maxBucketPrefix)
	}
	return nil
}

// ParseSingleEndpoint validates an S3 endpoint value: blank is allowed (returns ""), otherwise
// exactly ONE `http(s)://host[:port]` entry of a comma-separated list. Exported so core/storage
// applies the same rule to a Config built by hand, without a second copy of it.
//
// More than one entry is REFUSED, never trimmed to the first (rule 11 forbidden #5). Credentials
// in the URL are refused too: they belong in the Secret, and a URL is printed everywhere.
func ParseSingleEndpoint(name, raw string) (string, error) {
	entries := danhSach(raw)
	switch len(entries) {
	case 0:
		return "", nil
	case 1:
	default:
		return "", fmt.Errorf("%w: %s lists %d hosts; the S3 client talks to exactly ONE endpoint — "+
			"put a load balancer in front of distributed MinIO and give its address "+
			"(taking the first host silently is what rule 11 forbidden #5 exists to stop)",
			ErrObjectStorageInvalid, name, len(entries))
	}
	u, err := url.Parse(entries[0])
	if err != nil {
		return "", fmt.Errorf("%w: %s is not a URL: %w", ErrObjectStorageInvalid, name, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w: %s must start with http:// or https:// (the scheme decides TLS)",
			ErrObjectStorageInvalid, name)
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("%w: %s has no host", ErrObjectStorageInvalid, name)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w: %s carries credentials in the URL — they belong in "+
			"OBJECT_STORAGE_ACCESS_KEY / OBJECT_STORAGE_SECRET_KEY", ErrObjectStorageInvalid, name)
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%w: %s must be scheme://host[:port] only — an S3 endpoint has no path",
			ErrObjectStorageInvalid, name)
	}
	return u.Scheme + "://" + u.Host, nil
}

// ParsePublicMediaBaseURL validates OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL: blank allowed, else
// http(s), a host, an optional path, no query or credentials. The trailing slash is dropped so
// joining a key is one "/". Exported for core/storage, same reason as ParseSingleEndpoint.
func ParsePublicMediaBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.Contains(raw, ",") {
		return "", fmt.Errorf("%w: OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL is one base URL, not a list",
			ErrObjectStorageInvalid)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL is not a URL: %w",
			ErrObjectStorageInvalid, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%w: OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL must be http(s)://host[/path] "+
			"with no query or credentials", ErrObjectStorageInvalid)
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host+u.EscapedPath(), "/"), nil
}
