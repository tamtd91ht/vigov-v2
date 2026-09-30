package platformclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sync"
	"time"

	"google.golang.org/grpc"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/tenant"
)

// Tier 1 of the petition field catalogue — the closed code set every commune shares (ADR 0026) —
// read over PlatformService.ListPetitionFields and cached with the rules that RPC's comment sets for
// every caller (ADR 0060). A business service validates a field code HERE before it stores one:
// nothing downstream can (identity.proto, ResolveDeadlines answers a misspelled code with the default
// deadline).
//
// WHY IN platformclient ITSELF AND NOT A SUBPACKAGE (unlike uploadpolicy): this needs nothing beyond
// what platformclient already imports, so it adds no dependency to any service's build.

// PetitionFieldsTTL is how long one answer is served from memory.
//
// 60 SECONDS IS THE CONTRACT'S CEILING, not a tuning choice. There is no invalidation event; what
// makes that safe is that codes are only ever ADDED, so a stale answer can only refuse a new code,
// show an old default label, or accept a just-retired code that is still a real tier-1 code (ADR 0060
// §2). A longer TTL stretches all three, and an expired answer is never served.
const PetitionFieldsTTL = 60 * time.Second

var (
	// ErrPetitionFieldsUnavailable means the platform could not be asked and no answer younger than
	// PetitionFieldsTTL is held. The caller REFUSES with a retryable error (HTTP 503). It is never
	// "the code is valid" and never "the code is unknown".
	ErrPetitionFieldsUnavailable = errors.New("platformclient: petition field codes unavailable")

	// ErrPetitionFieldUnknown means the code is not a tier-1 code. Refuse the write (HTTP 400/422 —
	// the caller's choice); never store it, never map it to `khac` on the citizen's behalf.
	ErrPetitionFieldUnknown = errors.New("platformclient: not a petition field code")

	// ErrPetitionFieldRetired means the code exists but the platform retired it: refuse it for NEW
	// intake and classification. It is still a valid code on read paths — label it.
	ErrPetitionFieldRetired = errors.New("platformclient: petition field code retired")
)

// PetitionField is one usable tier-1 entry.
type PetitionField struct {
	Code         string
	DefaultLabel string
	SortOrder    int
	Icon         string // "" = not declared: render a neutral icon
	Tone         string // "" = not declared, or a tone this build does not know
	Active       bool
}

// PetitionFieldSet is one answer: every usable code, retired ones included, in platform order.
// The zero value holds no code.
type PetitionFieldSet struct {
	list   []PetitionField
	byCode map[string]PetitionField
}

// Lookup returns the entry for code, retired or not. ok=false: not a tier-1 code.
func (s PetitionFieldSet) Lookup(code string) (PetitionField, bool) {
	f, ok := s.byCode[code]
	return f, ok
}

// All returns every entry, retired ones included, ordered by SortOrder then Code. A copy: a caller
// reordering it for tier 2 must not change what the next caller reads.
func (s PetitionFieldSet) All() []PetitionField { return slices.Clone(s.list) }

// CheckForIntake is the membership check of a WRITE path — a citizen submitting, a staff member
// classifying, a commune adding an SLA or tier-2 row. nil means the code may be written. It says
// nothing about the commune's own tier-2 switch; the caller applies that on top.
func (s PetitionFieldSet) CheckForIntake(code string) error {
	f, ok := s.byCode[code]
	switch {
	case !ok:
		return ErrPetitionFieldUnknown
	case !f.Active:
		return ErrPetitionFieldRetired
	}
	return nil
}

// PetitionFieldsClient is the one RPC this reader makes. platformv1.PlatformServiceClient satisfies
// it — pass Directory.Client(), whose interceptors put "x-tenant-id" in metadata.
type PetitionFieldsClient interface {
	ListPetitionFields(ctx context.Context, in *platformv1.ListPetitionFieldsRequest,
		opts ...grpc.CallOption) (*platformv1.ListPetitionFieldsResponse, error)
}

// PetitionFields serves PetitionFieldSet from a cache.
//
// ONE ENTRY FOR EVERY COMMUNE, NOT ONE PER COMMUNE — deliberately unlike uploadpolicy. Tier 1 is the
// same set for every commune by definition (ADR 0026); a commune's own variation is tier 2, owned by
// service-petitions, never by the platform (ADR 0060 §2 and stop condition #3). Keying by commune
// would hold the same answer a few hundred times.
type PetitionFields struct {
	cl  PetitionFieldsClient
	log *slog.Logger
	now func() time.Time // injectable so tests do not sleep

	mu      sync.Mutex
	set     PetitionFieldSet
	expires time.Time // zero = nothing cached
}

// NewPetitionFields builds a reader. cl must be non-nil: a nil client would surface as a panic on the
// first intake, in production, instead of here.
func NewPetitionFields(cl PetitionFieldsClient, log *slog.Logger) *PetitionFields {
	if cl == nil {
		panic("platformclient: NewPetitionFields with a nil client")
	}
	if log == nil {
		log = slog.Default()
	}
	return &PetitionFields{cl: cl, log: log, now: time.Now}
}

// Set returns the tier-1 code set.
//
//	(set, nil)   an answer at most PetitionFieldsTTL old. It may hold no code — then every intake
//	             is refused (the contract: empty is "no code is valid", never "anything goes").
//	(_, err)     ctx carries no commune (a caller bug — refused locally, nothing is sent), or
//	             ErrPetitionFieldsUnavailable. Refuse; for the latter answer 503.
//
// THE COMMUNE IS REQUIRED EVEN ON A CACHE HIT, although the answer does not depend on it: a caller
// that forgot to put one in ctx must fail on every call, not only on the one call a minute that
// misses the cache.
func (r *PetitionFields) Set(ctx context.Context) (PetitionFieldSet, error) {
	if _, ok := tenant.From(ctx); !ok {
		return PetitionFieldSet{}, fmt.Errorf("platformclient: PetitionFields: %w", tenant.ErrNoTenant)
	}

	r.mu.Lock()
	set, expires := r.set, r.expires
	r.mu.Unlock()
	if !expires.IsZero() && r.now().Before(expires) {
		return set, nil
	}
	return r.fetch(ctx)
}

// fetch asks the platform and caches the answer. Two concurrent misses may both call; that costs one
// extra small RPC once a minute and needs no coordination. Errors are never cached, and a failed
// refresh never extends the old answer.
func (r *PetitionFields) fetch(ctx context.Context) (PetitionFieldSet, error) {
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	res, err := r.cl.ListPetitionFields(ctx, &platformv1.ListPetitionFieldsRequest{})
	if err != nil {
		return PetitionFieldSet{}, fmt.Errorf("%w: ListPetitionFields: %w", ErrPetitionFieldsUnavailable, err)
	}

	set := r.usable(ctx, res.GetFields())
	r.mu.Lock()
	r.set, r.expires = set, r.now().Add(PetitionFieldsTTL)
	r.mu.Unlock()
	return set, nil
}

// petitionFieldCode is the code shape service-platform's CHECK enforces (migration 0011): ADR 0011's
// enum-value spelling, at most 64 characters.
var petitionFieldCode = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var knownTones = map[string]bool{"blue": true, "green": true, "orange": true, "purple": true, "cyan": true, "red": true}

// usable keeps the entries a caller may rely on; anything else is dropped and so reads as "not a
// tier-1 code" — refused on every write. Dropping is the safe direction: the alternative, keeping
// an entry that breaks the contract, is letting a code nobody can vouch for into archival records.
func (r *PetitionFields) usable(ctx context.Context, in []*platformv1.PetitionField) PetitionFieldSet {
	count := make(map[string]int, len(in))
	for _, f := range in {
		count[f.GetCode()]++
	}

	set := PetitionFieldSet{byCode: make(map[string]PetitionField, len(in))}
	for _, f := range in {
		code := f.GetCode()
		switch {
		case !petitionFieldCode.MatchString(code) || len(code) > 64:
			r.skip(ctx, "code empty or malformed")
			continue
		case count[code] > 1:
			// "At most one entry per code" is the contract. Two entries give two labels and possibly
			// two active flags; picking one is guessing, so neither is served.
			r.skip(ctx, "code listed more than once")
			continue
		case f.GetDefaultLabel() == "":
			// A code with no label is a code a screen would show raw — the ve-sinh-moi-truong defect.
			r.skip(ctx, "empty default_label")
			continue
		case f.GetSortOrder() < 1:
			r.skip(ctx, "sort_order < 1")
			continue
		}
		tone := f.GetTone()
		if !knownTones[tone] {
			tone = "" // unknown or undeclared: the client renders neutral, never guesses
		}
		pf := PetitionField{Code: code, DefaultLabel: f.GetDefaultLabel(), SortOrder: int(f.GetSortOrder()),
			Icon: f.GetIcon(), Tone: tone, Active: f.GetActive()}
		set.list = append(set.list, pf)
		set.byCode[code] = pf
	}
	// The contract promises this order; sorting again costs nothing and means no caller depends on
	// the server keeping that promise.
	slices.SortStableFunc(set.list, func(a, b PetitionField) int {
		if a.SortOrder != b.SortOrder {
			return a.SortOrder - b.SortOrder
		}
		switch {
		case a.Code < b.Code:
			return -1
		case a.Code > b.Code:
			return 1
		}
		return 0
	})
	return set
}

// skip logs a dropped entry by reason only. Codes are platform configuration, not personal data, but
// a malformed one may be anything, so it is not echoed.
func (r *PetitionFields) skip(ctx context.Context, reason string) {
	r.log.WarnContext(ctx, "platformclient: petition field entry ignored, treated as not a tier-1 code",
		"reason", reason)
}
