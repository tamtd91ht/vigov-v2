package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// SenderPhoneBackfill (migration 0046, ADR 0102 #3).
//
//	PROVED HERE   every commune is keyed IN ITS OWN CONTEXT · each row gets core's national form · one
//	              audit entry per keyed row, by the system actor, subject = lookup code, in the SAME
//	              transaction as the write, masked delta · an unnormalisable number is left NULL, counted,
//	              and does not stop the run (the id cursor moves past it) · IDEMPOTENT: a second run keys
//	              nothing and writes no entry · RESUMABLE: a run stopped after a committed batch resumes
//	              without re-keying or re-auditing it · a row keyed meanwhile is skipped with no entry ·
//	              without the lock nothing is read · no number and no lookup code in a log line.
//	NOT PROVED    the SQL (internal/store/contact_petition_test.go) and PostgreSQL itself.

type backfillRow struct {
	id, code, raw, key string
}

// backfillStoreFake is the register of several communes, read and written as the scoped store would.
type backfillStoreFake struct {
	rows       map[tenant.ID][]*backfillRow
	batchCalls int
	failBatch  int // the batchCalls value on which the read fails; 0 = never
	setCommune []tenant.ID
	// keyedElsewhere: ids another writer keys between this job's read and its write.
	keyedElsewhere map[string]bool
}

func (f *backfillStoreFake) SenderPhoneBackfillBatch(ctx context.Context, afterID string, limit int) (
	[]petstore.SenderPhoneBackfillRow, error) {
	f.batchCalls++
	if f.failBatch != 0 && f.batchCalls == f.failBatch {
		return nil, errors.New("db blip")
	}
	rows := f.rows[tenant.MustFrom(ctx)]
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	var out []petstore.SenderPhoneBackfillRow
	for _, r := range rows {
		if r.key == "" && r.raw != "" && r.id > afterID && len(out) < limit {
			out = append(out, petstore.SenderPhoneBackfillRow{ID: r.id, LookupCode: r.code, RawPhone: r.raw})
		}
	}
	return out, nil
}

func (f *backfillStoreFake) SetSenderPhoneMatchKey(ctx context.Context, tx *pkgstore.ScopedTx, id, raw, key string) (bool, error) {
	f.setCommune = append(f.setCommune, tenant.MustFrom(ctx))
	// The statement runs on the transaction, so the fake driver records whether one was open.
	if _, err := tx.Exec(ctx, "UPDATE phieu_phan_anh SET sender_phone_normalized = $3", string(tx.TenantID()), id, key, raw); err != nil {
		return false, err
	}
	if f.keyedElsewhere[id] {
		return false, nil
	}
	for _, r := range f.rows[tenant.MustFrom(ctx)] {
		if r.id == id && r.key == "" && r.raw == raw {
			r.key = key
			return true, nil
		}
	}
	return false, nil
}

func backfillFixture() *backfillStoreFake {
	return &backfillStoreFake{rows: map[tenant.ID][]*backfillRow{
		xaThu: {
			{id: "pa-01", code: "PA-AAAA-AAAA-AAA1", raw: "0900000000"},
			{id: "pa-02", code: "PA-AAAA-AAAA-AAA2", raw: "+84 900 000 001"},
			{id: "pa-03", code: "PA-AAAA-AAAA-AAA3", raw: "not a number"},
			{id: "pa-04", code: "PA-AAAA-AAAA-AAA4", raw: "0900.000.002"},
			{id: "pa-05", code: "PA-AAAA-AAAA-AAA5", raw: "0900000003", key: "0900000003"}, // keyed at intake
		},
		xaKia: {
			{id: "pb-01", code: "PA-BBBB-BBBB-BBB1", raw: "0084900000004"},
		},
	}}
}

func newBackfillForTest(t *testing.T, k *khoGia, f *backfillStoreFake, locks JobLocker, communes []tenant.ID,
	logs *bytes.Buffer) *SenderPhoneBackfill {
	t.Helper()
	b, err := NewSenderPhoneBackfill(pkgstore.New(sql.OpenDB(k)), f, &fakeCommunes{ids: communes}, locks,
		slog.New(slog.NewTextHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	b.batch = 2 // several batches per commune
	return b
}

func backfillAuditEntries(k *khoGia) []lenhGia {
	var out []lenhGia
	for _, l := range k.lenh {
		if strings.Contains(l.sql, "INSERT INTO audit_log") {
			out = append(out, l)
		}
	}
	return out
}

func TestBackfillKeysEveryCommuneInItsOwnContextWithOneEntryPerRow(t *testing.T) {
	k, f, logs := &khoGia{}, backfillFixture(), &bytes.Buffer{}
	b := newBackfillForTest(t, k, f, &fakeLocks{}, []tenant.ID{xaThu, xaKia}, logs)

	done, err := b.Pass(context.Background())
	if !done || err != nil {
		t.Fatalf("done=%v err=%v", done, err)
	}
	want := map[string]string{"pa-01": "0900000000", "pa-02": "0900000001", "pa-03": "", "pa-04": "0900000002",
		"pa-05": "0900000003", "pb-01": "0900000004"}
	for _, rows := range f.rows {
		for _, r := range rows {
			if r.key != want[r.id] {
				t.Errorf("%s key = %q, want %q", r.id, r.key, want[r.id])
			}
		}
	}
	entries := backfillAuditEntries(k)
	if len(entries) != 4 {
		t.Fatalf("%d audit entries, want 4 (one per row keyed — not the unnormalisable one, not the pre-keyed one)", len(entries))
	}
	byCommune := map[string]int{}
	for _, e := range entries {
		if !e.trongGiaoDich {
			t.Error("an audit entry was written outside the transaction (rule 6, invariant 3)")
		}
		// $1 tenant · $2 actor · $3 kind · $4 ip · $5 action · $6 subject · $7 at · $8 delta
		if e.args[1] != "system" || e.args[2] != "system" || e.args[4] != AuditActionSenderPhoneKeyed ||
			!strings.HasPrefix(e.args[5].(string), "PA-") {
			t.Errorf("entry = %v", e.args[:6])
		}
		if delta := string(e.args[7].([]byte)); strings.Contains(delta, "0900000") {
			t.Errorf("delta carries the number unmasked: %s", delta)
		}
		byCommune[e.args[0].(string)]++
	}
	if byCommune[string(xaThu)] != 3 || byCommune[string(xaKia)] != 1 {
		t.Errorf("entries per commune = %v — each in its own commune", byCommune)
	}
	for _, l := range k.lenh {
		if !l.trongGiaoDich {
			t.Errorf("statement outside a transaction: %s", l.sql)
		}
	}
	for _, leak := range []string{"0900000", "PA-AAAA", "PA-BBBB", "not a number"} {
		if strings.Contains(logs.String(), leak) {
			t.Errorf("%q in the log: %s", leak, logs.String())
		}
	}
	if !strings.Contains(logs.String(), "khong_chuan_hoa_duoc=1") {
		t.Errorf("the unnormalisable count is not reported: %s", logs.String())
	}
}

func TestBackfillIsIdempotent(t *testing.T) {
	k, f := &khoGia{}, backfillFixture()
	b := newBackfillForTest(t, k, f, &fakeLocks{}, []tenant.ID{xaThu, xaKia}, &bytes.Buffer{})
	if done, err := b.Pass(context.Background()); !done || err != nil {
		t.Fatalf("first pass: %v %v", done, err)
	}
	first := len(backfillAuditEntries(k))
	if done, err := b.Pass(context.Background()); !done || err != nil {
		t.Fatalf("second pass: %v %v", done, err)
	}
	if n := len(backfillAuditEntries(k)); n != first {
		t.Errorf("second run wrote %d more entries — a re-run must key and audit nothing", n-first)
	}
}

// A run that stops after a committed batch resumes from there: nothing keyed twice, no entry twice.
func TestBackfillResumesAfterAFailedRun(t *testing.T) {
	k, f := &khoGia{}, backfillFixture()
	f.failBatch = 2 // commune A: batch 1 (pa-01, pa-02) commits, the second read fails
	b := newBackfillForTest(t, k, f, &fakeLocks{}, []tenant.ID{xaThu, xaKia}, &bytes.Buffer{})

	done, err := b.Pass(context.Background())
	if done || err == nil {
		t.Fatalf("done=%v err=%v — a commune that stopped must be retried", done, err)
	}
	if f.rows[xaKia][0].key == "" {
		t.Error("commune B was not keyed — one commune failing must not stop another")
	}
	afterFirst := len(backfillAuditEntries(k))
	if afterFirst != 3 {
		t.Fatalf("%d entries after the failed run, want 3 (pa-01, pa-02, pb-01)", afterFirst)
	}

	if done, err := b.Pass(context.Background()); !done || err != nil {
		t.Fatalf("resumed pass: %v %v", done, err)
	}
	subjects := map[string]int{}
	for _, e := range backfillAuditEntries(k) {
		subjects[e.args[5].(string)]++
	}
	for code, n := range subjects {
		if n != 1 {
			t.Errorf("%s audited %d times", code, n)
		}
	}
	if len(subjects) != 4 || f.rows[xaThu][3].key != "0900000002" {
		t.Errorf("subjects = %v, pa-04 key = %q", subjects, f.rows[xaThu][3].key)
	}
}

func TestBackfillSkipsRowKeyedMeanwhileWithoutAnEntry(t *testing.T) {
	k, f := &khoGia{}, backfillFixture()
	f.keyedElsewhere = map[string]bool{"pa-01": true}
	b := newBackfillForTest(t, k, f, &fakeLocks{}, []tenant.ID{xaThu}, &bytes.Buffer{})
	n, err := b.Commune(tenant.Into(context.Background(), xaThu))
	if err != nil {
		t.Fatal(err)
	}
	if n.Keyed != 2 || n.Skipped != 1 || n.Unnormalisable != 1 {
		t.Errorf("counts = %+v, want 2 keyed, 1 skipped, 1 unnormalisable", n)
	}
	for _, e := range backfillAuditEntries(k) {
		if e.args[5] == "PA-AAAA-AAAA-AAA1" {
			t.Error("an entry for a row this job did not write")
		}
	}
}

func TestBackfillWithoutTheLockReadsNothing(t *testing.T) {
	k, f := &khoGia{}, backfillFixture()
	b := newBackfillForTest(t, k, f, &fakeLocks{held: map[string]bool{}}, []tenant.ID{xaThu}, &bytes.Buffer{})
	done, err := b.Pass(context.Background())
	if !done || err != nil || f.batchCalls != 0 || len(k.lenh) != 0 {
		t.Fatalf("done=%v err=%v reads=%d statements=%d — another replica holds the job", done, err, f.batchCalls, len(k.lenh))
	}
}

func TestBackfillRunReturnsOnCancel(t *testing.T) {
	k, f := &khoGia{}, backfillFixture()
	f.failBatch = 1
	b := newBackfillForTest(t, k, f, &fakeLocks{}, []tenant.ID{xaThu}, &bytes.Buffer{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b.Run(ctx) // must return, not wait out the retry
}

func TestNewSenderPhoneBackfillRefusesMissingDependencies(t *testing.T) {
	if _, err := NewSenderPhoneBackfill(nil, nil, nil, nil, nil); err == nil {
		t.Fatal("built without dependencies")
	}
}
