package http

// The stand-ins for the two document registers, used by document_test.go.
//
// EVERY ONE OF THEM RECORDS THE COMMUNE IT WAS CALLED IN, read from the context exactly as
// *store.Scoped reads it. A fake that ignored the commune would let the "right permission, wrong
// commune" case of rule 5, invariant 7 pass while proving nothing at all — which is the trap
// fakeDocumentTypes avoids by keying its rows by commune, and the one that makes that case the
// easiest of the four to fake.

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// fakeIncomingLister is the incoming register's READ half, KEYED BY COMMUNE.
type fakeIncomingLister struct {
	byTenant map[tenant.ID][]domain.IncomingDocument
	err      error

	calls      int
	lastFilter docstore.IncomingDocumentFilter
	lastTenant tenant.ID
}

func (k *fakeIncomingLister) List(ctx context.Context, filter docstore.IncomingDocumentFilter,
	_ page.Request) (page.Result[domain.IncomingDocument], error) {

	k.calls++
	k.lastFilter = filter
	k.lastTenant = tenant.MustFrom(ctx)
	if k.err != nil {
		return page.Result[domain.IncomingDocument]{}, k.err
	}
	return page.Result[domain.IncomingDocument]{Items: k.byTenant[tenant.MustFrom(ctx)]}, nil
}

// fakeIncomingWriter stands in for the incoming register's write use case.
type fakeIncomingWriter struct {
	out domain.IncomingDocument
	err error

	registerCalls, updateCalls, removeCalls, routeCalls int
	lastTenant                                          tenant.ID
	lastActor                                           audit.Actor
	lastRegister                                        app.RegisterIncomingDocumentRequest
	lastUpdate                                          app.UpdateIncomingDocumentRequest
	lastRoute                                           app.RouteDocumentRequest
	lastID, lastReason                                  string
}

func (g *fakeIncomingWriter) note(ctx context.Context, actor audit.Actor) {
	g.lastTenant = tenant.MustFrom(ctx)
	g.lastActor = actor
}

func (g *fakeIncomingWriter) Register(ctx context.Context, req app.RegisterIncomingDocumentRequest,
	actor audit.Actor) (domain.IncomingDocument, error) {
	g.registerCalls++
	g.lastRegister = req
	g.note(ctx, actor)
	if g.err != nil {
		return domain.IncomingDocument{}, g.err
	}
	return g.out, nil
}

func (g *fakeIncomingWriter) Update(ctx context.Context, id string, req app.UpdateIncomingDocumentRequest,
	actor audit.Actor) (domain.IncomingDocument, error) {
	g.updateCalls++
	g.lastID, g.lastUpdate = id, req
	g.note(ctx, actor)
	if g.err != nil {
		return domain.IncomingDocument{}, g.err
	}
	return g.out, nil
}

func (g *fakeIncomingWriter) Remove(ctx context.Context, id, reason string, actor audit.Actor) error {
	g.removeCalls++
	g.lastID, g.lastReason = id, reason
	g.note(ctx, actor)
	return g.err
}

func (g *fakeIncomingWriter) Route(ctx context.Context, id string, req app.RouteDocumentRequest,
	actor audit.Actor) (domain.IncomingDocument, error) {
	g.routeCalls++
	g.lastID, g.lastRoute = id, req
	g.note(ctx, actor)
	if g.err != nil {
		return domain.IncomingDocument{}, g.err
	}
	return g.out, nil
}

func (g *fakeIncomingWriter) totalCalls() int {
	return g.registerCalls + g.updateCalls + g.removeCalls + g.routeCalls
}

// fakeIncomingReader stands in for the detail drawer's two reads, KEYED BY COMMUNE.
//
// IT MODELS WHAT THE STORE'S PREDICATE DOES, so the handler's 404 is exercised on all three causes:
// a row of another commune is simply absent from this commune's map, and a row in `removed` is
// present but removed — both return docstore.ErrIncomingDocumentNotFound, exactly as
// `tenant_id = $1 AND deleted_at IS NULL` would. The SQL itself is asserted in the app tests.
type fakeIncomingReader struct {
	byTenant map[tenant.ID][]domain.IncomingDocument
	removed  map[string]bool
	history  map[string][]domain.DocumentRouting // by document id; stored oldest first
	err      error

	detailCalls, historyCalls int
	lastTenant                tenant.ID
}

func (k *fakeIncomingReader) find(ctx context.Context, id string) (domain.IncomingDocument, error) {
	k.lastTenant = tenant.MustFrom(ctx)
	if k.err != nil {
		return domain.IncomingDocument{}, k.err
	}
	for _, d := range k.byTenant[tenant.MustFrom(ctx)] {
		if d.ID == id && !k.removed[id] {
			return d, nil
		}
	}
	return domain.IncomingDocument{}, docstore.ErrIncomingDocumentNotFound
}

func (k *fakeIncomingReader) Detail(ctx context.Context, id string) (domain.IncomingDocument, error) {
	k.detailCalls++
	return k.find(ctx, id)
}

func (k *fakeIncomingReader) RoutingHistory(ctx context.Context, id string) ([]domain.DocumentRouting, error) {
	k.historyCalls++
	d, err := k.find(ctx, id)
	if err != nil {
		return nil, err
	}
	return k.history[d.ID], nil
}

func (k *fakeIncomingReader) totalCalls() int { return k.detailCalls + k.historyCalls }

func sampleIncomingReader() *fakeIncomingReader {
	return &fakeIncomingReader{
		byTenant: sampleIncomingLister().byTenant,
		removed:  map[string]bool{},
		history: map[string][]domain.DocumentRouting{
			"vbd-a-001": {
				{ID: "ls-1", IncomingDocumentID: "vbd-a-001", RoutedAt: time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC),
					RoutedBy: "CB-00001", StatusAtTime: domain.IncomingStatusAssigned,
					ToOrgUnitID: "bp-van-phong", Instruction: "Chuyển văn phòng xem xét"},
				{ID: "ls-2", IncomingDocumentID: "vbd-a-001", RoutedAt: time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC),
					RoutedBy: "CB-00002", StatusAtTime: domain.IncomingStatusInProgress,
					FromOrgUnitID: "bp-van-phong", ToOrgUnitID: "bp-dia-chinh", AssigneeCode: "CB-00003",
					Instruction: "Thuộc thẩm quyền bộ phận Địa chính"},
			},
		},
	}
}

// fakeOutgoingLister is the outgoing register's READ half, KEYED BY COMMUNE.
type fakeOutgoingLister struct {
	byTenant map[tenant.ID][]domain.OutgoingDocument
	err      error

	calls      int
	lastFilter docstore.OutgoingDocumentFilter
	lastTenant tenant.ID
}

func (k *fakeOutgoingLister) List(ctx context.Context, filter docstore.OutgoingDocumentFilter,
	_ page.Request) (page.Result[domain.OutgoingDocument], error) {

	k.calls++
	k.lastFilter = filter
	k.lastTenant = tenant.MustFrom(ctx)
	if k.err != nil {
		return page.Result[domain.OutgoingDocument]{}, k.err
	}
	return page.Result[domain.OutgoingDocument]{Items: k.byTenant[tenant.MustFrom(ctx)]}, nil
}

// fakeOutgoingWriter stands in for the outgoing register's write use case.
type fakeOutgoingWriter struct {
	out domain.OutgoingDocument
	err error

	issueCalls, updateCalls, removeCalls int
	lastTenant                           tenant.ID
	lastActor                            audit.Actor
	lastIssue                            app.IssueOutgoingDocumentRequest
	lastUpdate                           app.UpdateOutgoingDocumentRequest
	lastID, lastReason                   string
}

func (g *fakeOutgoingWriter) note(ctx context.Context, actor audit.Actor) {
	g.lastTenant = tenant.MustFrom(ctx)
	g.lastActor = actor
}

func (g *fakeOutgoingWriter) IssueNumber(ctx context.Context, req app.IssueOutgoingDocumentRequest,
	actor audit.Actor) (domain.OutgoingDocument, error) {
	g.issueCalls++
	g.lastIssue = req
	g.note(ctx, actor)
	if g.err != nil {
		return domain.OutgoingDocument{}, g.err
	}
	return g.out, nil
}

func (g *fakeOutgoingWriter) Update(ctx context.Context, id string, req app.UpdateOutgoingDocumentRequest,
	actor audit.Actor) (domain.OutgoingDocument, error) {
	g.updateCalls++
	g.lastID, g.lastUpdate = id, req
	g.note(ctx, actor)
	if g.err != nil {
		return domain.OutgoingDocument{}, g.err
	}
	return g.out, nil
}

func (g *fakeOutgoingWriter) Remove(ctx context.Context, id, reason string, actor audit.Actor) error {
	g.removeCalls++
	g.lastID, g.lastReason = id, reason
	g.note(ctx, actor)
	return g.err
}

func (g *fakeOutgoingWriter) totalCalls() int { return g.issueCalls + g.updateCalls + g.removeCalls }

// The fixtures. TWO COMMUNES WITH DIFFERENT DATA, because two communes whose registers looked the
// same could not show a leak.
func sampleIncomingLister() *fakeIncomingLister {
	return &fakeIncomingLister{byTenant: map[tenant.ID][]domain.IncomingDocument{
		tenantA: {
			{ID: "vbd-a-002", ArrivalNo: 2, Year: 2026, IssuingBody: "Huyện uỷ",
				DocumentType: "cong-van", Summary: "Về việc rà soát hộ nghèo",
				Status: domain.IncomingStatusRegistered},
			{ID: "vbd-a-001", ArrivalNo: 1, Year: 2026, IssuingBody: "UBND huyện",
				DocumentType: "quyet-dinh", Summary: "Quyết định giao dự toán",
				Status: domain.IncomingStatusAssigned},
		},
		tenantB: {
			{ID: "vbd-b-001", ArrivalNo: 1, Year: 2026, IssuingBody: "Sở Nội vụ xã B",
				DocumentType: "thong-bao", Summary: "Thông báo của xã B",
				Status: domain.IncomingStatusRegistered},
		},
	}}
}

func sampleOutgoingLister() *fakeOutgoingLister {
	return &fakeOutgoingLister{byTenant: map[tenant.ID][]domain.OutgoingDocument{
		tenantA: {
			{ID: "vbdi-a-001", IssuedNo: 1, Year: 2026, DocumentType: "cong-van",
				Summary: "Trả lời đơn của công dân", Recipient: "UBND huyện"},
		},
		tenantB: {
			{ID: "vbdi-b-001", IssuedNo: 1, Year: 2026, DocumentType: "bao-cao",
				Summary: "Báo cáo của xã B", Recipient: "Huyện uỷ"},
		},
	}}
}

// --- the idempotency store ------------------------------------------------------------------------

// fakeIdemStore is idem.Store in a map.
//
// WHY THE REGISTER TESTS NEED A REAL ONE WHILE THE CATALOGUE TESTS DO NOT: the two routes that
// ISSUE A NUMBER declare idem.Required(idem.DongKhiHong), and a nil store is an UNREACHABLE store —
// which that declaration answers with 503, by design. With nil, every assertion about those routes
// would be an assertion about a missing cache.
//
// TTLs are ignored: nothing in these tests waits.
type fakeIdemStore struct {
	mu     sync.Mutex
	values map[string]string
}

func newFakeIdemStore() *fakeIdemStore { return &fakeIdemStore{values: map[string]string{}} }

func (k *fakeIdemStore) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.values[key]; ok {
		return false, nil
	}
	k.values[key] = "1"
	return true, nil
}

func (k *fakeIdemStore) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.values[key], nil
}

func (k *fakeIdemStore) Complete(_ context.Context, key, value string, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.values[key] = value
	return nil
}

func (k *fakeIdemStore) Release(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.values, key)
	return nil
}

// newTestServerWithIdem is newTestServer with a working idempotency store behind it.
func newTestServerWithIdem(t *testing.T) *testServer {
	t.Helper()
	m := newTestServer(t)
	m.idemStore = newFakeIdemStore()
	m.rebuild(t, nil)
	return m
}
