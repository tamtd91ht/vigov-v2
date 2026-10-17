package store

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/store/crosstenant"
)

// The commune OA's SQL (migration 0031) against a REAL PostgreSQL: column spellings, the two
// platform-wide unique keys, the guards the use case must satisfy (a reseal needs a new
// token_refreshed_at; a retired row is frozen), the one-template-per-milestone key, the callback's
// cross-commune lookup. SKIPPED WITHOUT VIGOV_TEST_DSN — a green run without the DSN means this file
// COMPILES, nothing more.

var oaAt = time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC)

func oaRig(t *testing.T) (*pkgstore.DB, *ZaloCommuneOAStore, *ZaloOAAuthorizationStore, *ZaloOATemplateStore,
	*crosstenant.ZaloOA, string, string) {
	t.Helper()
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	h := pkgstore.New(db)
	return h, NewZaloCommuneOAStore(h), NewZaloOAAuthorizationStore(h), NewZaloOATemplateStore(h), crosstenant.NewZaloOA(db), c1, c2
}

func liveOA(id, oaID string) domain.CommuneZaloOA {
	return domain.CommuneZaloOA{ID: id, OAID: oaID, OAName: "UBND Xã PG", AccessExpiresAt: oaAt.Add(25 * time.Hour),
		RefreshIssuedAt: oaAt, AuthorizedAt: oaAt, AuthorizedBy: "CB-00123", LastCheckAt: oaAt, LastCheckResult: domain.ZaloCallOK}
}

func TestPgCommuneOAOneLivePerOAAcrossCommunesAndResealGuard(t *testing.T) {
	h, oas, _, _, _, c1, c2 := oaRig(t)
	ctx1, ctx2 := ctxXa(tenant.ID(c1)), ctxXa(tenant.ID(c2))
	run := func(ctx interface{ Done() <-chan struct{} }, c string, fn func(tx *pkgstore.ScopedTx) error) error {
		cc := ctxXa(tenant.ID(c))
		return h.For(cc).Tx(cc, fn)
	}
	const id1 = "01JOAPG0000000000000000001"
	if err := run(ctx1, c1, func(tx *pkgstore.ScopedTx) error {
		return oas.InsertLive(ctx1, tx, liveOA(id1, "579745863508352884"), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, found, err := oas.LiveByID(ctx1, id1)
	if err != nil || !found || got.OAID != "579745863508352884" || got.AuthorizedBy != "CB-00123" ||
		!got.TokenRefreshedAt.Equal(oaAt) || got.LastCheckResult != domain.ZaloCallOK || got.NeedsReauthorization() {
		t.Fatalf("live = %+v %v %v", got, found, err)
	}
	// Commune 2 asking for commune 1's row id: not found — the scope is the context's commune.
	if _, found, err := oas.LiveByID(ctx2, id1); err != nil || found {
		t.Fatalf("commune 2 read commune 1's OA by id: %v %v", found, err)
	}
	if _, found, err := oas.SealedByID(ctx2, id1); err != nil || found {
		t.Fatalf("commune 2 read commune 1's sealed OA by id: %v %v", found, err)
	}
	// The same OA in ANOTHER commune: one generic error, nothing about where (0031 one_live_per_oa).
	err = run(ctx2, c2, func(tx *pkgstore.ScopedTx) error {
		return oas.InsertLive(ctx2, tx, liveOA("01JOAPG0000000000000000002", "579745863508352884"), fakeSealed, fakeSealed)
	})
	if !errors.Is(err, ErrZaloOATaken) {
		t.Fatalf("second commune, same OA: %v", err)
	}
	if list, err := oas.ListLive(ctx2); err != nil || len(list) != 0 {
		t.Fatalf("commune 2 sees %v (%v)", list, err)
	}
	// A second, DIFFERENT OA in the SAME commune is simply another live row (0032 dropped
	// one_live_per_commune; ADR 0099 #1). ListLive returns both, oldest first.
	if err := run(ctx1, c1, func(tx *pkgstore.ScopedTx) error {
		return oas.InsertLive(ctx1, tx, liveOA("01JOAPG0000000000000000003", "111"), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatalf("second OA in one commune: %v", err)
	}
	if list, err := oas.ListLive(ctx1); err != nil || len(list) != 2 || list[0].ID != id1 || list[1].OAID != "111" {
		t.Fatalf("list = %+v %v", list, err)
	}
	// The callback's lookup by Zalo's oa_id, in this commune only.
	if err := run(ctx1, c1, func(tx *pkgstore.ScopedTx) error {
		s, found, err := oas.LockLiveByZaloOAID(ctx1, tx, "111")
		if err != nil || !found || s.OA.ID != "01JOAPG0000000000000000003" {
			t.Fatalf("lock by oa_id = %+v %v %v", s.OA, found, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := run(ctx2, c2, func(tx *pkgstore.ScopedTx) error {
		if _, found, err := oas.LockLiveByZaloOAID(ctx2, tx, "111"); err != nil || found {
			t.Fatalf("commune 2 locked commune 1's OA by oa_id: %v %v", found, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Reseal guarded on the token_refreshed_at the caller read.
	later := oaAt.Add(23 * time.Hour)
	newSealed := bytes.Repeat([]byte{8}, 40)
	if err := run(ctx1, c1, func(tx *pkgstore.ScopedTx) error {
		return oas.Reseal(ctx1, tx, id1, newSealed, newSealed, later.Add(25*time.Hour), later, later, oaAt)
	}); err != nil {
		t.Fatalf("reseal: %v", err)
	}
	if err := run(ctx1, c1, func(tx *pkgstore.ScopedTx) error {
		return oas.Reseal(ctx1, tx, id1, fakeSealed, fakeSealed, later.Add(30*time.Hour), later, later.Add(time.Minute), oaAt)
	}); !errors.Is(err, ErrZaloRowChanged) {
		t.Fatalf("stale reseal: %v", err)
	}
	s, found, err := oas.SealedByID(ctx1, id1)
	if err != nil || !found || !bytes.Equal(s.AccessTokenSealed, newSealed) || !s.OA.TokenRefreshedAt.Equal(later) ||
		!s.OA.RefreshIssuedAt.Equal(later) {
		t.Fatalf("after reseal: %+v %v", s.OA, err)
	}
	// Flag, then retire; a retired row is frozen and no longer live.
	if err := run(ctx1, c1, func(tx *pkgstore.ScopedTx) error {
		return oas.MarkNeedsReauthorization(ctx1, tx, id1, domain.ReauthorizationRefreshTokenRejected, later)
	}); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if err := run(ctx1, c1, func(tx *pkgstore.ScopedTx) error {
		return oas.MarkNeedsReauthorization(ctx1, tx, id1, domain.ReauthorizationRefreshTokenExpired, later)
	}); !errors.Is(err, ErrZaloRowChanged) {
		t.Fatalf("second mark: %v", err)
	}
	if err := run(ctx1, c1, func(tx *pkgstore.ScopedTx) error {
		return oas.Retire(ctx1, tx, id1, "CB-00123", "Thử ngắt", later.Add(time.Hour))
	}); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if _, found, _ := oas.LiveByID(ctx1, id1); found {
		t.Fatal("a retired OA is still live")
	}
	if list, _ := oas.ListLive(ctx1); len(list) != 1 || list[0].ID == id1 {
		t.Fatalf("a retired OA is still listed: %+v", list)
	}
	if err := run(ctx1, c1, func(tx *pkgstore.ScopedTx) error {
		return oas.RecordCheck(ctx1, tx, id1, domain.ZaloCallOK, later)
	}); !errors.Is(err, ErrZaloRowChanged) {
		t.Fatalf("a retired row took a write: %v", err)
	}
	// The OA is free again for the other commune once retired here.
	if err := run(ctx2, c2, func(tx *pkgstore.ScopedTx) error {
		return oas.InsertLive(ctx2, tx, liveOA("01JOAPG0000000000000000004", "579745863508352884"), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatalf("after retire, other commune: %v", err)
	}
}

func TestPgAuthorizationStateHashLookupAndClose(t *testing.T) {
	h, _, auths, _, cross, c1, c2 := oaRig(t)
	ctx1 := ctxXa(tenant.ID(c1))
	state := bytes.Repeat([]byte{0x5a}, 32)
	hash := sha256.Sum256(state)
	const id = "01JAUTHPG00000000000000001"
	req := domain.ZaloOAAuthorization{ID: id, RequestedAt: time.Now().UTC(), RequestedBy: "CB-00123",
		ExpiresAt: time.Now().UTC().Add(domain.AuthorizationTTL)}
	if err := h.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		return auths.Insert(ctx1, tx, req, hash[:], fakeSealed)
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// The callback's lookup: hash → (commune, id), no commune in the question.
	tid, rowID, found, err := cross.OpenAuthorizationByStateHash(ctx1, hash[:])
	if err != nil || !found || string(tid) != c1 || rowID != id {
		t.Fatalf("lookup = %q %q %v %v", tid, rowID, found, err)
	}
	// The same hash cannot be issued again anywhere (total unique index).
	ctx2 := ctxXa(tenant.ID(c2))
	if err := h.For(ctx2).Tx(ctx2, func(tx *pkgstore.ScopedTx) error {
		return auths.Insert(ctx2, tx, domain.ZaloOAAuthorization{ID: "01JAUTHPG00000000000000002", RequestedAt: req.RequestedAt,
			RequestedBy: "CB-00001", ExpiresAt: req.ExpiresAt}, hash[:], fakeSealed)
	}); err == nil {
		t.Fatal("one state hash issued twice")
	}
	// Read in the commune, close as a Zalo refusal, then the lookup finds nothing (spent).
	if err := h.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		open, found, err := auths.LockOpenByID(ctx1, tx, id)
		if err != nil || !found || open.Auth.RequestedBy != "CB-00123" || !bytes.Equal(open.CodeVerifierSealed, fakeSealed) {
			t.Fatalf("lock = %+v %v %v", open.Auth, found, err)
		}
		return auths.Close(ctx1, tx, id, time.Now().UTC(), domain.AuthorizationZaloRefused, "")
	}); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, _, found, _ := cross.OpenAuthorizationByStateHash(ctx1, hash[:]); found {
		t.Fatal("a spent request is still found by its state")
	}
	if _, found, _ := auths.OpenByID(ctx1, id); found {
		t.Fatal("a spent request is still open by id")
	}
	// A spent request is frozen (0031 guard).
	if err := h.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		return auths.Close(ctx1, tx, id, time.Now().UTC(), domain.AuthorizationSucceeded, "")
	}); !errors.Is(err, ErrZaloRowChanged) {
		t.Fatalf("second close: %v", err)
	}
	// Expiry sweep: an open request past its expiry is closed as het-han, and listed per commune first.
	past := domain.ZaloOAAuthorization{ID: "01JAUTHPG00000000000000003", RequestedAt: oaAt, RequestedBy: "CB-00123",
		ExpiresAt: oaAt.Add(time.Minute)}
	h2 := sha256.Sum256(bytes.Repeat([]byte{0x5b}, 32))
	if err := h.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		return auths.Insert(ctx1, tx, past, h2[:], fakeSealed)
	}); err != nil {
		t.Fatal(err)
	}
	ids, err := cross.CommunesWithOpenAuthorizations(ctx1, time.Now().UTC())
	if err != nil || !containsTenant(ids, c1) {
		t.Fatalf("communes with expired requests = %v %v", ids, err)
	}
	var expired []string
	if err := h.For(ctx1).Tx(ctx1, func(tx *pkgstore.ScopedTx) error {
		expired, err = auths.ExpireOpen(ctx1, tx, time.Now().UTC(), time.Now().UTC())
		return err
	}); err != nil || len(expired) != 1 || expired[0] != past.ID {
		t.Fatalf("expired = %v %v", expired, err)
	}
}

func TestPgTemplatesOnePerMilestoneAndDetachOnRetire(t *testing.T) {
	h, oas, _, tpls, cross, c1, _ := oaRig(t)
	ctx := ctxXa(tenant.ID(c1))
	const oaRow = "01JOAPG0000000000000000011"
	run := func(fn func(tx *pkgstore.ScopedTx) error) error { return h.For(ctx).Tx(ctx, fn) }
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return oas.InsertLive(ctx, tx, liveOA(oaRow, "987"), fakeSealed, fakeSealed)
	}); err != nil {
		t.Fatal(err)
	}
	params := []domain.TemplateParam{{Name: "ma_tra_cuu", Required: true, Type: "STRING", MaxLength: 30},
		{Name: "moc_nhan", Type: "STRING"}, {Name: "viec_tiep_theo", Type: "STRING", MaxLength: 200}}
	t1 := domain.ZaloOATemplate{ID: "01JTPLPG00000000000000001", TemplateID: "1001", TemplateName: "Tiếp nhận",
		ZaloStatus: "ENABLE", ZaloQuality: "HIGH", ZaloTag: "TRANSACTION", Params: params, ParamsMatch: true}
	t2 := t1
	t2.ID, t2.TemplateID, t2.TemplateName = "01JTPLPG00000000000000002", "1002", "Đóng"
	if err := run(func(tx *pkgstore.ScopedTx) error {
		for _, tp := range []domain.ZaloOATemplate{t1, t2} {
			if err := tpls.UpsertFromSync(ctx, tx, oaRow, tp, oaAt); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	list, err := tpls.ListByOA(ctx, oaRow)
	if err != nil || len(list) != 2 || len(list[1].Params) != 3 || list[1].Params[0].MaxLength != 30 || !list[1].ParamsMatch {
		t.Fatalf("list = %+v %v", list, err)
	}
	if got, found, err := tpls.GetByZaloTemplateID(ctx, oaRow, "1002"); err != nil || !found || got.ID != t2.ID {
		t.Fatalf("by Zalo template id = %+v %v %v", got, found, err)
	}
	if _, found, err := tpls.GetByZaloTemplateID(ctx, "01JOAPGOTHERROW00000000001", "1002"); err != nil || found {
		t.Fatalf("a template read under another OA row: %v %v", found, err)
	}
	// Attach one; a second template on the same milestone is refused (one_per_milestone).
	m := domain.MilestoneReceived
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return tpls.SetMilestone(ctx, tx, t1.ID, &m, oaAt, "CB-00123")
	}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return tpls.SetMilestone(ctx, tx, t2.ID, &m, oaAt.Add(time.Minute), "CB-00123")
	}); !errors.Is(err, ErrMilestoneTaken) {
		t.Fatalf("second template, same milestone: %v", err)
	}
	// A re-sync rewrites Zalo's columns and keeps the milestone; the same id is NOT reused.
	t1b := t1
	t1b.ID, t1b.ZaloStatus, t1b.ParamsMatch = "01JTPLPG00000000000000009", "REJECT", false
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return tpls.UpsertFromSync(ctx, tx, oaRow, t1b, oaAt.Add(time.Hour))
	}); err != nil {
		t.Fatal(err)
	}
	got, found, err := tpls.Get(ctx, t1.ID)
	if err != nil || !found || got.ZaloStatus != "REJECT" || got.Milestone != m || got.MilestoneSetBy != "CB-00123" {
		t.Fatalf("after re-sync: %+v %v %v", got, found, err)
	}
	if att, found, err := tpls.AttachedForMilestone(ctx, m); err != nil || !found || att.Template.ID != t1.ID || att.OA.ID != oaRow ||
		att.Template.OARowID != oaRow || att.OA.OAID != "987" || !att.OA.RetiredAt.IsZero() {
		t.Fatalf("attached = %+v %v %v", att, found, err)
	}
	if c, err := tpls.CountsByOA(ctx); err != nil || c[oaRow] != (TemplateCounts{Templates: 2, Attached: 1}) {
		t.Fatalf("counts = %+v %v", c, err)
	}
	// Another commune sees nothing attached.
	if _, found, _ := tpls.AttachedForMilestone(ctxXa(tenant.ID("01JOTHERCOMMUNE0000000000X")), m); found {
		t.Fatal("another commune reads this commune's attachment")
	}
	// Retire + detach all in one transaction; the milestone is free for the next OA's template.
	var detached []string
	if err := run(func(tx *pkgstore.ScopedTx) error {
		if err := oas.Retire(ctx, tx, oaRow, "CB-00123", domain.CommuneZaloOAReauthorizedReason, oaAt.Add(2*time.Hour)); err != nil {
			return err
		}
		detached, err = tpls.DetachAllOfOA(ctx, tx, oaRow, oaAt.Add(2*time.Hour))
		return err
	}); err != nil || len(detached) != 1 || detached[0] != t1.ID {
		t.Fatalf("retire+detach: %v %v", detached, err)
	}
	if _, found, _ := tpls.AttachedForMilestone(ctx, m); found {
		t.Fatal("a detached milestone is still attached")
	}
	// MarkDeleted keeps the row.
	if err := run(func(tx *pkgstore.ScopedTx) error { return tpls.MarkDeleted(ctx, tx, t2.ID, oaAt.Add(3*time.Hour)) }); err != nil {
		t.Fatal(err)
	}
	if got, found, _ := tpls.Get(ctx, t2.ID); !found || got.ZaloStatus != "DELETE" {
		t.Fatalf("deleted = %+v %v", got, found)
	}
	// Refresh listing: a retired row is never listed.
	ids, err := cross.CommunesWithOAToRefresh(ctx, oaAt.Add(1000*time.Hour))
	if err != nil || containsTenant(ids, c1) {
		t.Fatalf("a retired OA is listed for refresh: %v %v", ids, err)
	}
}

func containsTenant(ids []tenant.ID, c string) bool {
	for _, id := range ids {
		if string(id) == c {
			return true
		}
	}
	return false
}
