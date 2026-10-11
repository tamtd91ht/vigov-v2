package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// BASEMAP_URL — the object-storage prefix holding the self-hosted Vietnam basemap: the PMTiles archive,
// its glyph ranges and its sprite sheet (ADR 0072 §Trả lời 09/10/2026, S1). The SAME key of
// `common-config` web-admin's `/basemap/*` proxy reads (web-admin/src/lib/may-chu/basemap.ts), so the
// staff maps and the Mini App map draw the same bytes — one value, one refresh.
//
// WHO READS IT HERE: service-comms, group Basemap, for its public basemap route (ADR 0072 §"Sửa đổi
// 11/10/2026 (Mini App)" M1).
//
// OPTIONAL IN EVERY ENVIRONMENT, prod included — the OPERATOR_HOST kind of switch, not the "used ⇒
// required" kind. Operations sets it only AFTER the signed sovereignty check of the built archive (ADR
// 0072 §Còn mở sau sửa đổi 11/10/2026 #3, runbook kb/40-runbooks/basemap-pmtiles.md); making it required
// would stop every comms route — news, alerts, the staff register — to wait for a legal sign-off about
// one map. Empty = the route answers 503 with one sentence and the Mini App draws no basemap (§S1 "Thiếu
// tệp nền": never a fallback host).
//
// A PLATFORM-WIDE CONSTANT (rule 1 invariant 10): one archive of mainland Vietnam serves every commune.

// ErrBasemapURLInvalid is a BASEMAP_URL that is not https, carries credentials, a query or a fragment.
// Load refuses — the rule every declared group follows: malformed is fatal, never repaired.
var ErrBasemapURLInvalid = errors.New("config: BASEMAP_URL is invalid")

// parseBasemapURL validates the trimmed value and returns it with exactly one trailing "/" on the path,
// so a file name joins onto it as a child. "" stays "" (off).
//
// THE SAME CHECKS AS web-admin's basemapBaseUrl, on purpose — one key, one meaning:
//
//	https only     rule 13 invariant 1: traffic to a data store is encrypted and verified; plain http
//	               would be the one unencrypted channel to MinIO in the system
//	no user:pass   a credential in a ConfigMap is not a Secret (rule 8)
//	no ?query/#    a query is where a presigned signature would be pasted; the prefix is public-read
//	a path         allowed — the value names a bucket and a dated prefix, not a service origin
//
// The error names the variable and the broken property, NEVER the value: a malformed URL is where
// credentials get pasted (rule 8 invariant 3).
func parseBasemapURL(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	switch {
	case err != nil:
		return "", fmt.Errorf("%w: not a URL", ErrBasemapURLInvalid)
	case u.Scheme != "https":
		return "", fmt.Errorf("%w: must be https", ErrBasemapURLInvalid)
	case u.User != nil:
		return "", fmt.Errorf("%w: must not carry credentials", ErrBasemapURLInvalid)
	case u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(raw, "?#"):
		return "", fmt.Errorf("%w: must not carry a query or a fragment", ErrBasemapURLInvalid)
	case u.Host == "" || u.Opaque != "":
		return "", fmt.Errorf("%w: has no host", ErrBasemapURLInvalid)
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
		if u.RawPath != "" {
			u.RawPath += "/"
		}
	}
	return u.String(), nil
}
