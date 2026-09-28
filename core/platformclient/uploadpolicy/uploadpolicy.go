// Package uploadpolicy reads the per-purpose upload limits the platform owns (ADR 0052 §10;
// PlatformService.ListUploadPolicies in proto/vigov/platform/v1/platform.proto) and caches them
// per commune, with the fail-closed semantics that RPC's comment requires of every caller.
//
// WHY A SUBPACKAGE OF platformclient AND NOT A FILE IN IT: this code needs core/storage — the
// closed purpose list and the MIME allow-list a policy may only narrow — and core/storage imports
// minio-go. platformclient is imported by EVERY service edge; importing storage there would put
// minio-go into every service's build, and a service whose go.sum does not list it (six of seven
// today) would stop building in its Dockerfile, which runs without go.work. Only services that own
// files import this package, and those import core/storage anyway.
package uploadpolicy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
)

// TTL is how long one commune's answer is served from memory.
//
// 60 SECONDS IS THE CONTRACT'S CEILING, not a tuning choice: ListUploadPolicies says "at most 60
// seconds" because there is no invalidation event — a limit Vihat tightens is live everywhere within
// one TTL, and that promise is only true if no caller holds an answer longer. A longer value here
// would keep accepting uploads a tightened policy already refuses, silently.
const TTL = 60 * time.Second

// ErrUnavailable means the platform could not be asked and no answer younger than TTL is held.
// The caller REFUSES the upload with a retryable error (HTTP 503). It is never "not configured"
// and never "allowed" — the RPC's status table says so.
var ErrUnavailable = errors.New("uploadpolicy: upload limits unavailable")

// Policy is the usable limit set for one purpose, already narrowed to this build's core/storage
// allow-list.
type Policy struct {
	Purpose storage.Purpose

	// MaxBytes of ONE file. Always > 0: an entry with less is treated as not configured.
	MaxBytes int64

	// AllowedMIMETypes, sniffed types only, every one of them in core/storage's allow-list. Never
	// empty: an entry left with none after narrowing is treated as not configured.
	AllowedMIMETypes []string

	// FileCountLimited reports whether Vihat set a count limit for this purpose. false is an
	// explicit "no count limit" in the operations zone, NOT a default — MaxFilesPerSubject is then
	// 0 and must not be read as "zero files allowed".
	FileCountLimited   bool
	MaxFilesPerSubject int

	// UpdatedAt is for diagnostics only; nothing may be decided from it (platform.proto).
	UpdatedAt time.Time
}

// AllowsMIME reports whether a SNIFFED type is allowed. Pass the type storage.SniffMIME returned,
// never the client's declared Content-Type (ADR 0052 §1c).
func (p Policy) AllowsMIME(mime string) bool {
	return slices.Contains(p.AllowedMIMETypes, mime)
}

// Client is the one RPC this package makes. platformv1.PlatformServiceClient satisfies it; build
// that client with platformclient.Dial's interceptors so "x-tenant-id" travels in metadata.
type Client interface {
	ListUploadPolicies(ctx context.Context, in *platformv1.ListUploadPoliciesRequest,
		opts ...grpc.CallOption) (*platformv1.ListUploadPoliciesResponse, error)
}

// Reader serves Policy lookups from a per-commune cache.
//
// THE CACHE IS KEYED BY COMMUNE ("t:<tenant_id>", rule 1 invariant 7) EVEN THOUGH EVERY COMMUNE
// GETS THE SAME ANSWER TODAY. A per-commune override is open (platform.proto), and the contract is
// worded so that one ships as a server-side change only — provided no caller caches one global
// answer. A global cache would be correct today and silently wrong the day an override ships.
//
// BOUNDED WITHOUT A CAP: the commune comes from context, where only the edge (a resolved Host) or a
// verified session put it, so the key set is the set of real communes — a few hundred.
type Reader struct {
	cl  Client
	log *slog.Logger
	now func() time.Time // injectable so tests do not sleep

	mu      sync.Mutex
	entries map[string]entry
}

type entry struct {
	policies map[storage.Purpose]Policy
	expires  time.Time
}

// New builds a Reader. cl must be non-nil: a nil client would surface as a panic on the first
// upload, in production, instead of here.
func New(cl Client, log *slog.Logger) *Reader {
	if cl == nil {
		panic("uploadpolicy: New with a nil client")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Reader{cl: cl, log: log, now: time.Now, entries: make(map[string]entry)}
}

// Policy returns the limits for purpose in THE COMMUNE IN ctx.
//
//	(p, true, nil)    configured and usable — enforce p.
//	(_, false, nil)   NOT CONFIGURED (absent, or present but unusable): refuse every upload for
//	                  this purpose. Never substitute a default (ADR 0052 §10, stop condition #4).
//	(_, false, err)   no answer: ctx carries no commune (a caller bug — refused locally, nothing is
//	                  sent), or ErrUnavailable. Refuse; for ErrUnavailable answer 503.
//
// An expired answer is NEVER served, not even when the refresh fails: a platform outage longer than
// one TTL stops uploads — the price of fail-closed the contract states. Errors are not cached.
func (r *Reader) Policy(ctx context.Context, purpose storage.Purpose) (Policy, bool, error) {
	id, ok := tenant.From(ctx)
	if !ok {
		return Policy{}, false, fmt.Errorf("uploadpolicy: %w", tenant.ErrNoTenant)
	}
	key := "t:" + id.String()

	r.mu.Lock()
	e, found := r.entries[key]
	r.mu.Unlock()
	if !found || !r.now().Before(e.expires) {
		var err error
		if e, err = r.fetch(ctx, key); err != nil {
			return Policy{}, false, err
		}
	}

	p, ok := e.policies[purpose]
	if !ok {
		return Policy{}, false, nil
	}
	// A copy of the slice: a caller appending to it must not change what the next caller reads.
	p.AllowedMIMETypes = slices.Clone(p.AllowedMIMETypes)
	return p, true, nil
}

// fetch asks the platform and caches the answer. Two concurrent misses may both call; that costs
// one extra small RPC once a minute and needs no coordination.
func (r *Reader) fetch(ctx context.Context, key string) (entry, error) {
	ctx, cancel := context.WithTimeout(ctx, platformclient.HanGoi)
	defer cancel()

	res, err := r.cl.ListUploadPolicies(ctx, &platformv1.ListUploadPoliciesRequest{})
	if err != nil {
		// Wrapped with %w on both: the caller tests ErrUnavailable, an operator reads the cause.
		return entry{}, fmt.Errorf("%w: ListUploadPolicies: %w", ErrUnavailable, err)
	}

	e := entry{policies: r.usable(ctx, res.GetPolicies()), expires: r.now().Add(TTL)}
	r.mu.Lock()
	r.entries[key] = e
	r.mu.Unlock()
	return e, nil
}

// knownPurposes is this build's closed list, as a set.
var knownPurposes = func() map[storage.Purpose]bool {
	m := make(map[storage.Purpose]bool)
	for _, p := range storage.Purposes() {
		m[p] = true
	}
	return m
}()

// purposeFromProto derives the core/storage spelling from an enum value by the rule platform.proto
// states — strip "UPLOAD_PURPOSE_", lowercase, "_" -> "-" — and accepts it only if this build's
// storage holds it. UNSPECIFIED and values this build does not know are refused.
func purposeFromProto(v platformv1.UploadPurpose) (storage.Purpose, bool) {
	if v == platformv1.UploadPurpose_UPLOAD_PURPOSE_UNSPECIFIED {
		return "", false
	}
	name, ok := platformv1.UploadPurpose_name[int32(v)]
	if !ok {
		return "", false
	}
	p := storage.Purpose(strings.ReplaceAll(
		strings.ToLower(strings.TrimPrefix(name, "UPLOAD_PURPOSE_")), "_", "-"))
	return p, knownPurposes[p]
}

// usable keeps the entries a caller may enforce, exactly as the RPC comment lists them; anything
// else is dropped and so reads as NOT CONFIGURED. Each drop is logged by reason and enum number
// only — there is no personal data in this answer, but there is nothing to gain from echoing it.
func (r *Reader) usable(ctx context.Context, in []*platformv1.UploadPolicy) map[storage.Purpose]Policy {
	out := make(map[storage.Purpose]Policy, len(in))
	seen := make(map[storage.Purpose]int, len(in))
	for _, pp := range in {
		if purpose, ok := purposeFromProto(pp.GetPurpose()); ok {
			seen[purpose]++
		}
	}

	for _, pp := range in {
		purpose, ok := purposeFromProto(pp.GetPurpose())
		if !ok {
			r.skip(ctx, pp, "purpose unspecified or unknown to this build")
			continue
		}
		if seen[purpose] > 1 {
			// "At most one entry per purpose" is the contract. Two entries give two answers, and
			// picking one is guessing which limit Vihat meant — so neither is served.
			r.skip(ctx, pp, "purpose listed more than once")
			continue
		}
		if pp.GetMaxBytes() <= 0 {
			r.skip(ctx, pp, "max_bytes <= 0")
			continue
		}
		var mimes []string
		for _, m := range pp.GetAllowedMimeTypes() {
			// Narrow, never widen: a type storage cannot sniff would be refused at complete anyway.
			if _, allowed := storage.ExtForMIME(m); allowed && !slices.Contains(mimes, m) {
				mimes = append(mimes, m)
			}
		}
		if len(mimes) == 0 {
			r.skip(ctx, pp, "no allowed MIME type left after narrowing to the storage allow-list")
			continue
		}
		p := Policy{Purpose: purpose, MaxBytes: pp.GetMaxBytes(), AllowedMIMETypes: mimes}
		if pp.MaxFilesPerSubject != nil {
			if pp.GetMaxFilesPerSubject() < 1 {
				// Present means ">= 1" in the contract. Reading 0 as "no limit" would widen it.
				r.skip(ctx, pp, "max_files_per_subject present but < 1")
				continue
			}
			p.FileCountLimited = true
			p.MaxFilesPerSubject = int(pp.GetMaxFilesPerSubject())
		}
		if ts := pp.GetUpdatedAt(); ts != nil {
			p.UpdatedAt = ts.AsTime()
		}
		out[purpose] = p
	}
	return out
}

func (r *Reader) skip(ctx context.Context, pp *platformv1.UploadPolicy, reason string) {
	r.log.WarnContext(ctx, "uploadpolicy: platform entry ignored, purpose treated as not configured",
		"purpose_enum", int32(pp.GetPurpose()), "reason", reason)
}
