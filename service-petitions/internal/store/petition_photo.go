package store

// A petition's citizen scene photos (migration 0026; internal/app/petition_photo.go). SQL only: who
// may read them — the petition's own citizen, or staff with `feedback.read` — is decided above.

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// petitionPhotosTail selects the photos a reader can be shown: a citizen photo of THIS petition that
// REACHED THE DESTINATION (stored / ready) and is not soft-deleted (rule 7, invariant 2). Pending,
// rejected, failed and purged rows are upload attempts, not photos of the case.
//
// The subject type, purpose and uploader are BOUND from the domain constants rather than written as
// literals, so the shape migration 0026 forces and the shape read here have one source in Go.
const petitionPhotosTail = `AND subject_type = $2 AND subject_id = $3 AND purpose = $4 AND uploaded_by = $5
	AND deleted_at IS NULL AND status IN ('stored', 'ready')
	ORDER BY created_at, id`

// PetitionPhotos reads the citizen scene photos of ONE petition of this commune, oldest first.
//
// petitionID is the petition's INTERNAL id (`phieu_phan_anh.id`), which the caller obtained from a
// read that already applied its own isolation — the citizen's (cong_dan_id from the session) or the
// staff register's. The commune is $1 from the context (core/store Scoped.Query): another commune's
// id matches nothing. At most 5 rows exist by migration 0026's trigger, so there is no paging.
func (s *StoredFileStore) PetitionPhotos(ctx context.Context, petitionID string) ([]domain.StoredFile, error) {
	rows, err := s.db.For(ctx).Query(ctx, storedFileCols, "stored_file", petitionPhotosTail,
		domain.StoredFileSubjectPetition, petitionID, domain.PurposePetitionPhoto, domain.CitizenLogActor)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc ảnh hiện trường: %w", err)
	}
	defer rows.Close()
	out := []domain.StoredFile{}
	for rows.Next() {
		f, err := scanStoredFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stored_file: duyệt ảnh hiện trường: %w", err)
	}
	return out, nil
}

// verificationPhotosTail selects the verification photos a reader can be shown: purpose
// `petition-verification-photo` BOUND EXPLICITLY (migration 0027 question 4: never "every petition
// file", which would hand a citizen the staff-only log attachments), uploaded by staff — never the
// citizen marker — reached the destination and not soft-deleted (rule 7, invariant 2).
const verificationPhotosTail = `AND subject_type = $2 AND subject_id = $3 AND purpose = $4 AND uploaded_by <> $5
	AND deleted_at IS NULL AND status IN ('stored', 'ready')
	ORDER BY created_at, id`

// VerificationPhotos reads the staff verification photos ("SAU KHI XỬ LÝ") of ONE petition of this
// commune, oldest first. The caller's own read already applied its isolation — the citizen's identity
// filter or the staff register's — and the commune is $1 from the context. At most 5 rows exist by
// migration 0027's trigger, so there is no paging.
func (s *StoredFileStore) VerificationPhotos(ctx context.Context, petitionID string) ([]domain.StoredFile, error) {
	rows, err := s.db.For(ctx).Query(ctx, storedFileCols, "stored_file", verificationPhotosTail,
		domain.StoredFileSubjectPetition, petitionID, domain.PurposePetitionVerificationPhoto, domain.CitizenLogActor)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc ảnh sau xử lý: %w", err)
	}
	defer rows.Close()
	out := []domain.StoredFile{}
	for rows.Next() {
		f, err := scanStoredFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stored_file: duyệt ảnh sau xử lý: %w", err)
	}
	return out, nil
}
