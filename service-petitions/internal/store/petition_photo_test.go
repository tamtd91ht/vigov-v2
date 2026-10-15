package store

import (
	"context"
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// PetitionPhotos on the recording driver: scoped to the context's commune as $1, bound to the
// petition, the citizen-photo shape of migration 0026, live rows that reached the destination, oldest
// first; the positional scan lines up. What PostgreSQL enforces (the 5-photo trigger, the CHECKs) is
// stored_file_pg_test.go's, and that SKIPS without VIGOV_TEST_DSN.
func TestPetitionPhotosScopedBoundAndLiveOnly(t *testing.T) {
	d := &sfDB{rows: []map[string]driver.Value{{
		"id": "f1", "bucket": "private", "object_key": "citizen-media/k/original.jpg", "retention_class": "citizen-media",
		"purpose": "petition-photo", "subject_type": "petition", "subject_id": "pa-1",
		"original_name": domain.PetitionPhotoName, "mime_type": "image/jpeg", "size_bytes": int64(48213), "sha256": "ab",
		"status": "stored", "uploaded_by": "cong-dan", "retain_until": nil, "legal_hold": false,
		"created_at": sfAt, "updated_at": sfAt,
	}}}
	s, _ := sfStore(d)
	got, err := s.PetitionPhotos(ctxXa(xaThu), "pa-1")
	if err != nil {
		t.Fatal(err)
	}
	st := d.stmts[0]
	if strings.Contains(st.sql, "uploaded_by =") {
		t.Errorf("statement still binds the uploader — an officer's scene photo would be invisible: %s", st.sql)
	}
	for _, frag := range []string{"WHERE tenant_id = $1", "subject_type = $2 AND subject_id = $3 AND purpose = $4",
		"deleted_at IS NULL", "status IN ('stored', 'ready')", "ORDER BY created_at, id"} {
		if !strings.Contains(st.sql, frag) {
			t.Errorf("statement lacks %q: %s", frag, st.sql)
		}
	}
	// NO UPLOADER BOUND (owner decision 15/10/2026): an officer's scene photo at intake is read too.
	want := []driver.Value{string(xaThu), "petition", "pa-1", "petition-photo"}
	if !reflect.DeepEqual(st.args, want) {
		t.Errorf("args = %v, want %v", st.args, want)
	}
	if len(got) != 1 || got[0].ID != "f1" || got[0].MIMEType != "image/jpeg" || got[0].SizeBytes != 48213 ||
		got[0].Status != domain.StoredFileStored {
		t.Errorf("got %+v", got)
	}

	d.rows = nil
	got, err = s.PetitionPhotos(ctxXa(xaThu), "pa-2")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("no photos: got %v (nil=%v), err %v — want an empty, non-nil list", got, got == nil, err)
	}
}

// PetitionCoverPhotos: ONE statement for the whole page, scoped to $1, the ids bound as ONE array, the
// same shape and liveness as PetitionPhotos, ordered oldest-first per petition — and only the FIRST row
// of each petition is returned. What PostgreSQL does with ANY($3) is stored_file_pg_test.go's territory.
func TestPetitionCoverPhotosOneStatementFirstPerPetition(t *testing.T) {
	row := func(id, petition string) map[string]driver.Value {
		return map[string]driver.Value{
			"id": id, "bucket": "private", "object_key": "citizen-media/k/" + id + "/original.jpg",
			"retention_class": "citizen-media", "purpose": "petition-photo", "subject_type": "petition",
			"subject_id": petition, "original_name": domain.PetitionPhotoName, "mime_type": "image/jpeg",
			"size_bytes": int64(9), "sha256": "ab", "status": "stored", "uploaded_by": "cong-dan",
			"retain_until": nil, "legal_hold": false, "created_at": sfAt, "updated_at": sfAt,
		}
	}
	// The driver returns rows in the statement's order: pa-1's oldest, pa-1's second, pa-2's only.
	d := &sfDB{rows: []map[string]driver.Value{row("f1", "pa-1"), row("f2", "pa-1"), row("f3", "pa-2")}}
	s, _ := sfStore(d)
	got, err := s.PetitionCoverPhotos(ctxXa(xaThu), []string{"pa-1", "pa-2", "pa-3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.stmts) != 1 {
		t.Fatalf("%d statements for one page — want ONE", len(d.stmts))
	}
	st := d.stmts[0]
	if strings.Contains(st.sql, "uploaded_by =") {
		t.Errorf("cover statement still binds the uploader: %s", st.sql)
	}
	for _, frag := range []string{"WHERE tenant_id = $1",
		"subject_type = $2 AND subject_id = ANY($3) AND purpose = $4",
		"deleted_at IS NULL", "status IN ('stored', 'ready')", "ORDER BY subject_id, created_at, id"} {
		if !strings.Contains(st.sql, frag) {
			t.Errorf("statement lacks %q: %s", frag, st.sql)
		}
	}
	want := []driver.Value{string(xaThu), "petition", []string{"pa-1", "pa-2", "pa-3"}, "petition-photo"}
	if !reflect.DeepEqual(st.args, want) {
		t.Errorf("args = %v, want %v", st.args, want)
	}
	if len(got) != 2 || got[0].ID != "f1" || got[0].SubjectID != "pa-1" || got[1].ID != "f3" {
		t.Errorf("got %+v — want f1 (pa-1's first) and f3", got)
	}

	t.Run("no ids: no statement", func(t *testing.T) {
		d := &sfDB{}
		s, _ := sfStore(d)
		got, err := s.PetitionCoverPhotos(ctxXa(xaThu), nil)
		if err != nil || got == nil || len(got) != 0 || len(d.stmts) != 0 {
			t.Errorf("got %v err %v stmts %d", got, err, len(d.stmts))
		}
	})
	t.Run("not a page: refused before SQL", func(t *testing.T) {
		d := &sfDB{}
		s, _ := sfStore(d)
		tooMany := make([]string, 101)
		for i := range tooMany {
			tooMany[i] = "pa-" + strings.Repeat("x", i+1)
		}
		for _, ids := range [][]string{{"pa-1", "pa-1"}, {""}, tooMany} {
			if _, err := s.PetitionCoverPhotos(ctxXa(xaThu), ids); err == nil {
				t.Errorf("ids %d accepted", len(ids))
			}
		}
		if len(d.stmts) != 0 {
			t.Error("a refused list reached the database")
		}
	})
}

func TestPetitionPhotosWithoutCommunePanics(t *testing.T) {
	s, _ := sfStore(&sfDB{})
	defer func() {
		if recover() == nil {
			t.Error("no commune in the context did not panic")
		}
	}()
	_, _ = s.PetitionPhotos(context.Background(), "pa-1")
}
