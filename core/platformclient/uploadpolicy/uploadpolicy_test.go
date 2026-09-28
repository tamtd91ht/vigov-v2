package uploadpolicy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
)

const (
	communeA = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EF")
	communeB = tenant.ID("01JD8ZQK9M3NPXR7TVWYB2C4EG")
)

// fakeClient answers from a settable response or error and counts calls, recording the commune the
// call carried so the per-commune key can be observed.
type fakeClient struct {
	mu      sync.Mutex
	res     *platformv1.ListUploadPoliciesResponse
	err     error
	calls   int
	tenants []tenant.ID
}

func (f *fakeClient) ListUploadPolicies(ctx context.Context, _ *platformv1.ListUploadPoliciesRequest,
	_ ...grpc.CallOption) (*platformv1.ListUploadPoliciesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	id, _ := tenant.From(ctx)
	f.tenants = append(f.tenants, id)
	if f.err != nil {
		return nil, f.err
	}
	return f.res, nil
}

func (f *fakeClient) set(res *platformv1.ListUploadPoliciesResponse, err error) {
	f.mu.Lock()
	f.res, f.err = res, err
	f.mu.Unlock()
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newReader(t *testing.T, f *fakeClient) (*Reader, *clock) {
	t.Helper()
	r := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := &clock{t: time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)}
	r.now = c.now
	return r, c
}

func ctxOf(id tenant.ID) context.Context { return tenant.Into(context.Background(), id) }

func photoPolicy() *platformv1.UploadPolicy {
	return &platformv1.UploadPolicy{
		Purpose:            platformv1.UploadPurpose_UPLOAD_PURPOSE_PETITION_PHOTO,
		MaxBytes:           10485760,
		AllowedMimeTypes:   []string{storage.MIMEJPEG, storage.MIMEPNG},
		MaxFilesPerSubject: proto.Int32(5),
	}
}

func answer(ps ...*platformv1.UploadPolicy) *platformv1.ListUploadPoliciesResponse {
	return &platformv1.ListUploadPoliciesResponse{Policies: ps}
}

// ---- drift: the proto enum and core/storage's closed list are one list ---------------------

// platform.proto: "A test in core compares this enum with storage's closed list in both
// directions". This is that test. A purpose added to one side only turns it red.
func TestEnumAndStoragePurposesMatchBothWays(t *testing.T) {
	fromEnum := map[storage.Purpose]bool{}
	for v, name := range platformv1.UploadPurpose_name {
		if v == 0 {
			continue
		}
		p, ok := purposeFromProto(platformv1.UploadPurpose(v))
		if !ok {
			t.Errorf("enum %s derives %q, which core/storage does not hold", name, p)
			continue
		}
		fromEnum[p] = true
	}
	for _, p := range storage.Purposes() {
		if !fromEnum[p] {
			t.Errorf("core/storage purpose %q has no UploadPurpose value in platform.proto", p)
		}
	}
	if len(fromEnum) != len(storage.Purposes()) {
		t.Errorf("enum gives %d purposes, storage holds %d", len(fromEnum), len(storage.Purposes()))
	}
}

func TestDerivationRuleSpelling(t *testing.T) {
	p, ok := purposeFromProto(platformv1.UploadPurpose_UPLOAD_PURPOSE_CONTENT_VIDEO)
	if !ok || p != storage.PurposeContentVideo {
		t.Fatalf("CONTENT_VIDEO -> %q, %v; want content-video", p, ok)
	}
	for _, v := range []platformv1.UploadPurpose{0, 999} {
		if p, ok := purposeFromProto(v); ok {
			t.Errorf("enum %d accepted as %q", v, p)
		}
	}
}

// ---- cache and fail-closed semantics -------------------------------------------------------

func TestMissingTenantRefusedLocally(t *testing.T) {
	f := &fakeClient{res: answer(photoPolicy())}
	r, _ := newReader(t, f)
	_, ok, err := r.Policy(context.Background(), storage.PurposePetitionPhoto)
	if !errors.Is(err, tenant.ErrNoTenant) || ok {
		t.Fatalf("ok=%v err=%v; want ErrNoTenant", ok, err)
	}
	if f.calls != 0 {
		t.Errorf("a call without a commune reached the platform (%d calls)", f.calls)
	}
}

func TestConfiguredPolicyAndCacheHit(t *testing.T) {
	f := &fakeClient{res: answer(photoPolicy())}
	r, c := newReader(t, f)

	p, ok, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if p.MaxBytes != 10485760 || !p.FileCountLimited || p.MaxFilesPerSubject != 5 ||
		!p.AllowsMIME(storage.MIMEJPEG) || p.AllowsMIME(storage.MIMEPDF) {
		t.Errorf("policy = %+v", p)
	}

	c.t = c.t.Add(TTL - time.Second)
	if _, ok, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto); err != nil || !ok {
		t.Fatalf("within TTL: ok=%v err=%v", ok, err)
	}
	if f.calls != 1 {
		t.Errorf("calls = %d within one TTL; want 1", f.calls)
	}
}

func TestAbsentPurposeIsNotConfigured(t *testing.T) {
	f := &fakeClient{res: answer(photoPolicy())}
	r, _ := newReader(t, f)
	_, ok, err := r.Policy(ctxOf(communeA), storage.PurposeContentVideo)
	if err != nil || ok {
		t.Fatalf("absent purpose: ok=%v err=%v; want not configured, no error", ok, err)
	}
	// An empty answer is ordinary: nothing is allowed anywhere.
	f.set(answer(), nil)
	r2, _ := newReader(t, f)
	if _, ok, err := r2.Policy(ctxOf(communeA), storage.PurposePetitionPhoto); err != nil || ok {
		t.Fatalf("empty answer: ok=%v err=%v", ok, err)
	}
}

// After the TTL a failed refresh must REFUSE — the expired answer is not served.
func TestExpiredAndRefreshFailsRefuses(t *testing.T) {
	f := &fakeClient{res: answer(photoPolicy())}
	r, c := newReader(t, f)
	if _, ok, _ := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto); !ok {
		t.Fatal("setup: not configured")
	}

	f.set(nil, status.Error(codes.Unavailable, "platform down"))
	c.t = c.t.Add(TTL)
	_, ok, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto)
	if !errors.Is(err, ErrUnavailable) || ok {
		t.Fatalf("expired + failure: ok=%v err=%v; want ErrUnavailable", ok, err)
	}
	if !strings.Contains(err.Error(), "platform down") {
		t.Errorf("cause lost from error: %v", err)
	}
}

// Errors are not cached: the next call asks again and recovers as soon as the platform does.
func TestErrorNotCached(t *testing.T) {
	f := &fakeClient{err: status.Error(codes.Unavailable, "down")}
	r, _ := newReader(t, f)
	if _, _, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	f.set(answer(photoPolicy()), nil)
	if _, ok, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto); err != nil || !ok {
		t.Fatalf("after recovery: ok=%v err=%v", ok, err)
	}
	if f.calls != 2 {
		t.Errorf("calls = %d; want 2", f.calls)
	}
}

// "Not configured" is cached for the same TTL and no longer: a purpose Vihat configures appears
// after one TTL.
func TestNotConfiguredReaskedAfterTTL(t *testing.T) {
	f := &fakeClient{res: answer()}
	r, c := newReader(t, f)
	if _, ok, _ := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto); ok {
		t.Fatal("setup: configured")
	}
	f.set(answer(photoPolicy()), nil)
	c.t = c.t.Add(TTL)
	if _, ok, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto); err != nil || !ok {
		t.Fatalf("after TTL: ok=%v err=%v", ok, err)
	}
}

// One commune's cached answer is never served to another: B asks the platform itself, with B in
// metadata.
func TestCacheKeyedPerCommune(t *testing.T) {
	f := &fakeClient{res: answer(photoPolicy())}
	r, _ := newReader(t, f)
	_, _, _ = r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto)
	_, _, _ = r.Policy(ctxOf(communeB), storage.PurposePetitionPhoto)
	if f.calls != 2 || f.tenants[0] != communeA || f.tenants[1] != communeB {
		t.Fatalf("calls=%d tenants=%v; want one call per commune, each carrying its own", f.calls, f.tenants)
	}
	if _, found := r.entries["t:"+communeA.String()]; !found {
		t.Errorf("cache keys = %v; want the t:<tenant_id> prefix", keys(r))
	}
}

func keys(r *Reader) []string {
	var out []string
	for k := range r.entries {
		out = append(out, k)
	}
	return out
}

func TestMIMENarrowedToStorageAllowList(t *testing.T) {
	pp := photoPolicy()
	pp.AllowedMimeTypes = []string{"image/svg+xml", storage.MIMEPNG, "text/html", storage.MIMEPNG}
	f := &fakeClient{res: answer(pp)}
	r, _ := newReader(t, f)
	p, ok, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if len(p.AllowedMIMETypes) != 1 || p.AllowedMIMETypes[0] != storage.MIMEPNG {
		t.Errorf("types = %v; want only image/png", p.AllowedMIMETypes)
	}
}

func TestUnusableEntriesAreNotConfigured(t *testing.T) {
	cases := map[string]func(*platformv1.UploadPolicy){
		"max_bytes zero":             func(p *platformv1.UploadPolicy) { p.MaxBytes = 0 },
		"max_bytes negative":         func(p *platformv1.UploadPolicy) { p.MaxBytes = -1 },
		"no types after narrowing":   func(p *platformv1.UploadPolicy) { p.AllowedMimeTypes = []string{"text/html"} },
		"no types at all":            func(p *platformv1.UploadPolicy) { p.AllowedMimeTypes = nil },
		"file count present but < 1": func(p *platformv1.UploadPolicy) { p.MaxFilesPerSubject = proto.Int32(0) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			pp := photoPolicy()
			mutate(pp)
			r, _ := newReader(t, &fakeClient{res: answer(pp)})
			if _, ok, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto); err != nil || ok {
				t.Fatalf("ok=%v err=%v; want not configured", ok, err)
			}
		})
	}
}

func TestUnspecifiedOrUnknownPurposeIgnored(t *testing.T) {
	unspecified := photoPolicy()
	unspecified.Purpose = platformv1.UploadPurpose_UPLOAD_PURPOSE_UNSPECIFIED
	unknown := photoPolicy()
	unknown.Purpose = platformv1.UploadPurpose(999)
	r, _ := newReader(t, &fakeClient{res: answer(unspecified, unknown)})
	for _, p := range storage.Purposes() {
		if _, ok, err := r.Policy(ctxOf(communeA), p); err != nil || ok {
			t.Errorf("%s: ok=%v err=%v; want not configured", p, ok, err)
		}
	}
}

// Two entries for one purpose break "at most one per purpose"; picking one would guess.
func TestDuplicatePurposeServesNeither(t *testing.T) {
	a, b := photoPolicy(), photoPolicy()
	b.MaxBytes = 1
	r, _ := newReader(t, &fakeClient{res: answer(a, b)})
	if _, ok, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto); err != nil || ok {
		t.Fatalf("ok=%v err=%v; want not configured", ok, err)
	}
}

// Absent max_files_per_subject is an explicit "no count limit", not zero files.
func TestNoFileCountLimit(t *testing.T) {
	pp := photoPolicy()
	pp.MaxFilesPerSubject = nil
	r, _ := newReader(t, &fakeClient{res: answer(pp)})
	p, ok, err := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto)
	if err != nil || !ok || p.FileCountLimited || p.MaxFilesPerSubject != 0 {
		t.Fatalf("p=%+v ok=%v err=%v", p, ok, err)
	}
}

func TestReturnedSliceIsACopy(t *testing.T) {
	r, _ := newReader(t, &fakeClient{res: answer(photoPolicy())})
	p, _, _ := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto)
	p.AllowedMIMETypes[0] = "text/html"
	q, _, _ := r.Policy(ctxOf(communeA), storage.PurposePetitionPhoto)
	if q.AllowedMIMETypes[0] == "text/html" {
		t.Fatal("a caller's edit changed the cached policy")
	}
}

func TestNewNilClientPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New(nil) did not panic")
		}
	}()
	_ = New(nil, nil)
}
