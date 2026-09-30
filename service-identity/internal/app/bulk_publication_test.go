package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: PublishMany — POST /api/v1/staff/publications (user decision 2026-09-30,
// option A). The properties that fail silently if broken: an unconfirmed row published anyway, a
// locked person published through the bulk path, a whole batch "succeeding" with one audit entry
// missing, and one transaction per row instead of one for the batch.
//
// Same fake driver as danh_ba_can_bo_test.go; the store below serves SEVERAL rows.

// multiRowStore is khoGia with a map of rows instead of one. Only the two methods PublishMany
// reaches are overridden; everything else is khoGia's.
type multiRowStore struct {
	*khoGia
	rows    map[string]domain.CanBoTomTat
	written []domain.CanBoTomTat
}

func (k *multiRowStore) TheoIDDeGhi(ctx context.Context, tx *store.ScopedTx, id string) (domain.CanBoTomTat, error) { // vi-name-ok: implements KhoDanhBaCanBo
	cb, ok := k.rows[id]
	if !ok {
		return domain.CanBoTomTat{}, idstore.ErrCanBoKhongTonTai
	}
	if _, err := tx.Exec(ctx, "SELECT doc-de-ghi FROM nguoi_dung", string(tx.TenantID()), id); err != nil {
		return domain.CanBoTomTat{}, err
	}
	return cb, nil
}

func (k *multiRowStore) DatCongKhai(ctx context.Context, tx *store.ScopedTx, cb domain.CanBoTomTat) error { // vi-name-ok: implements KhoDanhBaCanBo
	k.written = append(k.written, cb)
	_, err := tx.Exec(ctx, "UPDATE nguoi_dung SET cong-khai", cb.ID, cb.HienTrenMiniApp,
		cb.DongYCongKhaiLuc, cb.DongYCongKhaiGhiBoi, cb.ThuTuDanhBa)
	return err
}

const (
	idConfirmed   = "nd-01JBULKCONFIRMED0000000"
	idUnconfirmed = "nd-01JBULKUNCONFIRMED00000"
	idLocked      = "nd-01JBULKLOCKED000000000"
	idAlready     = "nd-01JBULKALREADY00000000"
	idUnknown     = "nd-01JBULKUNKNOWN00000000"
)

type bulkBench struct {
	uc    *DanhBaCanBo
	rows  *multiRowStore
	trace *ghiChep
}

func newBulkBench(t *testing.T) *bulkBench {
	t.Helper()
	b := dungBanThuDanhBa(t)
	row := func(id, code string) domain.CanBoTomTat {
		cb := canBoGia()
		cb.ID, cb.Ma = id, code
		return cb
	}
	locked := row(idLocked, "CB-LOCKED")
	locked.DangHoatDong = false
	already := row(idAlready, "CB-ALREADY")
	orig := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	already.HienTrenMiniApp = true
	already.DongYCongKhaiLuc = &orig
	already.DongYCongKhaiGhiBoi = "CB-2026-GHIGOC"
	already.ThuTuDanhBa = so(3)

	rows := &multiRowStore{khoGia: b.kho, rows: map[string]domain.CanBoTomTat{
		idConfirmed:   row(idConfirmed, "CB-CONFIRMED"),
		idUnconfirmed: row(idUnconfirmed, "CB-UNCONFIRMED"),
		idLocked:      locked,
		idAlready:     already,
	}}
	b.uc.kho = rows
	return &bulkBench{uc: b.uc, rows: rows, trace: b.ghi}
}

func mixedBatch() []BulkPublishItem {
	return []BulkPublishItem{
		{ID: idUnknown, ConsentConfirmed: true},
		{ID: idConfirmed, ConsentConfirmed: true, DisplayOrder: so(7)},
		{ID: idUnconfirmed, ConsentConfirmed: false},
		{ID: idLocked, ConsentConfirmed: true},
		{ID: idAlready, ConsentConfirmed: true},
	}
}

// THE MIXED BATCH: confirmed → published; unconfirmed → skipped consent; locked → skipped locked;
// unknown → skipped not found; already published → published, nothing written. Outcomes come back
// in REQUEST order even though rows are locked in id order.
//
// MUTATIONS THAT MUST TURN THIS RED: drop the per-row ConsentConfirmed gate; drop the lock check
// from applyPublicationInTx; abort the batch on ErrCanBoKhongTonTai.
func TestPublishManyMixedBatch(t *testing.T) {
	b := newBulkBench(t)

	out, err := b.uc.PublishMany(ctxXa(xaThu), mixedBatch(), nguoiThucHienGia())
	if err != nil {
		t.Fatalf("PublishMany: %v", err)
	}
	want := []struct {
		id        string
		published bool
		refusal   error
	}{
		{idUnknown, false, idstore.ErrCanBoKhongTonTai},
		{idConfirmed, true, nil},
		{idUnconfirmed, false, ErrChuaXacNhanDongY},
		{idLocked, false, ErrStaffLocked},
		{idAlready, true, nil},
	}
	if len(out) != len(want) {
		t.Fatalf("%d outcomes, want %d", len(out), len(want))
	}
	for i, w := range want {
		o := out[i]
		if o.ID != w.id || o.Published != w.published || !errors.Is(o.Refusal, w.refusal) ||
			(w.refusal == nil && o.Refusal != nil) {
			t.Errorf("outcome %d = %+v, want id=%s published=%v refusal=%v", i, o, w.id, w.published, w.refusal)
		}
	}

	// Exactly ONE row written — the confirmed one — with the consent marks and its order.
	if len(b.rows.written) != 1 {
		t.Fatalf("%d rows written, want 1: %+v", len(b.rows.written), b.rows.written)
	}
	g := b.rows.written[0]
	if g.ID != idConfirmed || !g.HienTrenMiniApp || g.DongYCongKhaiGhiBoi != maCanBo ||
		g.DongYCongKhaiLuc == nil || !g.DongYCongKhaiLuc.Equal(mocDongY) ||
		g.ThuTuDanhBa == nil || *g.ThuTuDanhBa != 7 {
		t.Errorf("written row wrong: %+v", g)
	}

	// ONE audit entry, for the one person whose state changed, naming them by code.
	entry := motVet(t, b.trace)
	if got := chuoiArg(t, entry, viTriHanhVi); got != HanhViCongKhaiMiniApp {
		t.Errorf("action = %q", got)
	}
	if got := chuoiArg(t, entry, viTriChuThe); got != "CB-CONFIRMED" {
		t.Errorf("subject = %q, want CB-CONFIRMED", got)
	}
	if got := chuoiArg(t, entry, viTriActor); got != maCanBo {
		t.Errorf("actor = %q, want staff code %q", got, maCanBo)
	}
	if strings.Contains(string(entry.args[viTriDelta].([]byte)), diDongGia) {
		t.Error("audit delta carries the raw mobile")
	}

	// ONE transaction for the whole batch, committed.
	if n := b.trace.soGiaoDich(); n != 1 {
		t.Errorf("%d transactions, want exactly 1 for the batch", n)
	}
	if end := b.trace.ketThucCua(entry.tx); end != "commit" {
		t.Errorf("transaction ended %q, want commit", end)
	}
}

// ONE AUDIT ROW PER PERSON PUBLISHED, all in the same transaction as their UPDATEs.
func TestPublishManyOneAuditPerPublishedPersonSameTx(t *testing.T) {
	b := newBulkBench(t)
	other := canBoGia()
	other.ID, other.Ma = "nd-01JBULKSECOND0000000000", "CB-SECOND"
	b.rows.rows[other.ID] = other

	out, err := b.uc.PublishMany(ctxXa(xaThu), []BulkPublishItem{
		{ID: idConfirmed, ConsentConfirmed: true},
		{ID: other.ID, ConsentConfirmed: true},
	}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("PublishMany: %v", err)
	}
	for _, o := range out {
		if !o.Published {
			t.Errorf("%s not published: %v", o.ID, o.Refusal)
		}
	}
	entries := vetDaGhi(b.trace)
	if len(entries) != 2 {
		t.Fatalf("%d audit entries, want 2 (one per person)", len(entries))
	}
	subjects := map[string]bool{}
	for _, e := range entries {
		subjects[chuoiArg(t, e, viTriChuThe)] = true
		if e.tx == 0 || e.tx != entries[0].tx {
			t.Errorf("audit entry outside the batch transaction (tx=%d)", e.tx)
		}
	}
	if !subjects["CB-CONFIRMED"] || !subjects["CB-SECOND"] {
		t.Errorf("subjects = %v", subjects)
	}
}

// AN AUDIT WRITE THAT FAILS ROLLS BACK THE WHOLE BATCH and the caller gets an error — never a list
// claiming people were published whose trail does not exist.
//
// MUTATION THAT MUST TURN THIS RED: treat the audit failure as a per-row skip.
func TestPublishManyRollsBackWhenAuditFails(t *testing.T) {
	b := newBulkBench(t)
	b.trace.loiTheo["audit_log"] = errors.New("audit down")

	out, err := b.uc.PublishMany(ctxXa(xaThu), []BulkPublishItem{
		{ID: idConfirmed, ConsentConfirmed: true},
	}, nguoiThucHienGia())
	if err == nil {
		t.Fatalf("audit failed but PublishMany succeeded: %+v", out)
	}
	if out != nil {
		t.Errorf("outcomes returned alongside an error: %+v", out)
	}
	upd := b.trace.tim("SET cong-khai")
	if upd == nil {
		t.Fatal("the UPDATE never ran — the test is not testing the rollback")
	}
	if end := b.trace.ketThucCua(upd.tx); end != "rollback" {
		t.Errorf("transaction ended %q, want rollback", end)
	}
}

// NOTHING CONFIRMED → no transaction at all, nothing written, every row reported.
func TestPublishManyNothingConfirmedOpensNoTransaction(t *testing.T) {
	b := newBulkBench(t)

	out, err := b.uc.PublishMany(ctxXa(xaThu), []BulkPublishItem{
		{ID: idConfirmed}, {ID: idLocked},
	}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("PublishMany: %v", err)
	}
	for _, o := range out {
		if o.Published || !errors.Is(o.Refusal, ErrChuaXacNhanDongY) {
			t.Errorf("%+v, want skipped consent_required", o)
		}
	}
	if n := b.trace.soGiaoDich(); n != 0 {
		t.Errorf("%d transactions opened for a batch with nothing confirmed", n)
	}
	khongCoGhi(t, b.trace)
}

// display_order ABSENT on an already-published person LEAVES their position alone (bulk is "turn
// on", not "reset"), and the original consent marks survive — so nothing is written at all.
func TestPublishManyKeepsOrderAndMarksOfAlreadyPublished(t *testing.T) {
	b := newBulkBench(t)

	if _, err := b.uc.PublishMany(ctxXa(xaThu), []BulkPublishItem{
		{ID: idAlready, ConsentConfirmed: true},
	}, nguoiThucHienGia()); err != nil {
		t.Fatalf("PublishMany: %v", err)
	}
	if len(b.rows.written) != 0 {
		t.Errorf("already published, no order sent, but a row was written: %+v", b.rows.written)
	}
	khongCoGhi(t, b.trace)
}

// A MALFORMED REQUEST IS REFUSED WHOLE, before any transaction.
func TestPublishManyMalformedRequestRefusedWhole(t *testing.T) {
	over := make([]BulkPublishItem, domain.MaxBulkPublication+1)
	for i := range over {
		over[i] = BulkPublishItem{ID: "nd-" + string(rune('A'+i%26)) + string(rune('a'+i/26)), ConsentConfirmed: true}
	}
	for _, c := range []struct {
		name  string
		items []BulkPublishItem
		want  error
	}{
		{"empty", nil, domain.ErrBulkPublicationEmpty},
		{"over the cap", over, domain.ErrBulkPublicationTooLarge},
		{"duplicate id", []BulkPublishItem{{ID: idConfirmed, ConsentConfirmed: true}, {ID: idConfirmed}}, domain.ErrBulkPublicationDuplicateID},
		{"empty id", []BulkPublishItem{{ID: "", ConsentConfirmed: true}}, domain.ErrBulkPublicationMissingID},
		{"negative order", []BulkPublishItem{{ID: idConfirmed, ConsentConfirmed: true, DisplayOrder: so(-1)}}, domain.ErrThuTuDanhBaAm},
	} {
		b := newBulkBench(t)
		out, err := b.uc.PublishMany(ctxXa(xaThu), c.items, nguoiThucHienGia())
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
		if out != nil {
			t.Errorf("%s: outcomes returned for a refused request", c.name)
		}
		if n := b.trace.soGiaoDich(); n != 0 {
			t.Errorf("%s: %d transactions opened", c.name, n)
		}
	}
}

// AN ACTOR WITH NO STAFF CODE IS REFUSED before anything — never replaced by the internal id.
func TestPublishManyActorWithoutCodeRefused(t *testing.T) {
	b := newBulkBench(t)
	actor := nguoiThucHienGia()
	actor.Vet.ID = ""
	if _, err := b.uc.PublishMany(ctxXa(xaThu), []BulkPublishItem{{ID: idConfirmed, ConsentConfirmed: true}}, actor); err == nil {
		t.Fatal("no staff code on the actor, yet PublishMany ran")
	}
	khongCoGhi(t, b.trace)
}
