package store

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The petition-side statements migration 0027 adds, on the stored_file_test.go fake.
//
//	PROVED HERE   each petition statement names petition_log_attachment and NEVER the task table · the
//	              commune is $1 on both sides of every join · the verification read binds the purpose
//	              and excludes the citizen marker · the settings read inside a transaction is scoped.
//	NOT PROVED    PostgreSQL — the triggers of 0027 (stored_file_pg_test.go SKIPS without VIGOV_TEST_DSN).

func TestPetitionLinkStatementsUseThePetitionTable(t *testing.T) {
	linked := sfRow("f1", "stored")
	linked["a.log_entry_id"] = nil
	d := &sfDB{rows: []map[string]driver.Value{linked}}
	s, h := sfStore(d)
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		if _, err := s.PetitionAttachCandidates(ctx, tx, []string{"f1"}); err != nil {
			return err
		}
		return s.LinkToPetitionLogEntry(ctx, tx, "nkpa-1", []string{"f1"})
	}); err != nil {
		t.Fatalf("tx: %v", err)
	}
	if len(d.stmts) != 2 {
		t.Fatalf("%d statements, want 2", len(d.stmts))
	}
	cand, link := d.stmts[0], d.stmts[1]
	if !strings.Contains(cand.sql, "LEFT JOIN petition_log_attachment a ON a.tenant_id = $1") ||
		!strings.Contains(cand.sql, "WHERE f.tenant_id = $1") || strings.Contains(cand.sql, "task_log_attachment") {
		t.Errorf("candidate read: %s", cand.sql)
	}
	if !strings.HasPrefix(link.sql, "INSERT INTO petition_log_attachment (tenant_id, log_entry_id, stored_file_id)") ||
		link.args[0] != string(xaThu) || link.args[1] != "nkpa-1" || link.args[2] != "f1" {
		t.Errorf("link: %s %v", link.sql, link.args)
	}
}

func TestPetitionAttachmentReadsUseThePetitionTable(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{{"log_entry_id": "nkpa-9"}}}
	s, _ := sfStore(d)
	got, err := s.LinkedPetitionLogEntry(ctxXa(xaThu), "f1")
	if err != nil || got != "nkpa-9" {
		t.Fatalf("LinkedPetitionLogEntry = %q, %v", got, err)
	}
	if st := d.stmts[0]; !strings.Contains(st.sql, "FROM petition_log_attachment WHERE tenant_id = $1 AND stored_file_id = $2") {
		t.Errorf("linked read: %s", st.sql)
	}

	d.rows = nil
	if _, err := s.PetitionAttachmentsByLogEntries(ctxXa(xaThu), []string{"nkpa-1"}); err != nil {
		t.Fatalf("PetitionAttachmentsByLogEntries: %v", err)
	}
	st := d.stmts[len(d.stmts)-1]
	if !strings.Contains(st.sql, "FROM petition_log_attachment a") ||
		!strings.Contains(st.sql, "JOIN stored_file f ON f.tenant_id = $1") ||
		!strings.Contains(st.sql, "f.deleted_at IS NULL") || strings.Contains(st.sql, "task_log_attachment") {
		t.Errorf("batched read: %s", st.sql)
	}
}

// The task statements are untouched by the generalisation: still the task table.
func TestTaskLinkStatementsStillUseTheTaskTable(t *testing.T) {
	d := &sfDB{}
	s, h := sfStore(d)
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		return s.LinkToLogEntry(ctx, tx, "nk-1", []string{"f1"})
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.stmts[0].sql, "INSERT INTO task_log_attachment ") {
		t.Errorf("task link: %s", d.stmts[0].sql)
	}
}

// Migration 0027 question 4: the verification read binds ITS purpose, so a log attachment can never be
// handed to a citizen, and the citizen marker is excluded.
func TestVerificationPhotosBindsPurposeAndExcludesCitizen(t *testing.T) {
	row := sfRow("v1", "stored")
	row["purpose"], row["subject_type"], row["subject_id"] = domain.PurposePetitionVerificationPhoto, "petition", "pa-1"
	d := &sfDB{rows: []map[string]driver.Value{row}}
	s, _ := sfStore(d)
	got, err := s.VerificationPhotos(ctxXa(xaThu), "pa-1")
	if err != nil || len(got) != 1 || got[0].ID != "v1" {
		t.Fatalf("VerificationPhotos = %+v, %v", got, err)
	}
	st := d.stmts[0]
	if !strings.Contains(st.sql, "FROM stored_file WHERE tenant_id = $1 AND subject_type = $2 AND subject_id = $3 AND purpose = $4 AND uploaded_by <> $5") ||
		!strings.Contains(st.sql, "deleted_at IS NULL AND status IN ('stored', 'ready')") {
		t.Errorf("statement: %s", st.sql)
	}
	want := []driver.Value{string(xaThu), domain.StoredFileSubjectPetition, "pa-1",
		domain.PurposePetitionVerificationPhoto, domain.CitizenLogActor}
	for i, w := range want {
		if st.args[i] != w {
			t.Errorf("arg %d = %v, want %v", i, st.args[i], w)
		}
	}
}

// The closing act reads the switch inside its transaction: scoped, and no row is TRUE there too.
func TestVerificationPhotoRequiredTxNoRowIsTrueAndScoped(t *testing.T) {
	d := &sfDB{}
	_, h := sfStore(d)
	s := NewPetitionSettingsStore(h)
	var got bool
	if err := sfInTx(t, h, xaThu, func(ctx context.Context, tx *pkgstore.ScopedTx) error {
		var err error
		got, err = s.VerificationPhotoRequiredTx(ctx, tx)
		return err
	}); err != nil {
		t.Fatalf("tx: %v", err)
	}
	if !got {
		t.Fatal("no row inside the closing transaction must be TRUE (ADR 0008 decision 3)")
	}
	var seen bool
	for _, l := range d.stmts {
		if strings.Contains(l.sql, " FROM petition_settings WHERE tenant_id = $1 ") && len(l.args) == 1 &&
			l.args[0] == string(xaThu) {
			seen = true
		}
	}
	if !seen {
		t.Errorf("settings read not scoped to the commune: %+v", d.stmts)
	}
}
