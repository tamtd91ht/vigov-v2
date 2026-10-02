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
	for _, frag := range []string{"WHERE tenant_id = $1", "subject_type = $2 AND subject_id = $3 AND purpose = $4 AND uploaded_by = $5",
		"deleted_at IS NULL", "status IN ('stored', 'ready')", "ORDER BY created_at, id"} {
		if !strings.Contains(st.sql, frag) {
			t.Errorf("statement lacks %q: %s", frag, st.sql)
		}
	}
	want := []driver.Value{string(xaThu), "petition", "pa-1", "petition-photo", "cong-dan"}
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

func TestPetitionPhotosWithoutCommunePanics(t *testing.T) {
	s, _ := sfStore(&sfDB{})
	defer func() {
		if recover() == nil {
			t.Error("no commune in the context did not panic")
		}
	}()
	_, _ = s.PetitionPhotos(context.Background(), "pa-1")
}
