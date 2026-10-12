package store

// A petition's citizen scene photos (migration 0026; internal/app/petition_photo.go). SQL only: who
// may read them — the petition's own citizen, or staff with `feedback.read` — is decided above.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/page"
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

// petitionCoverPhotosTail is petitionPhotosTail for MANY petitions at once — the same purpose, uploader,
// liveness and status, bound the same way — ordered so the FIRST row of each petition is its oldest
// photo, the one ListPhotos would list first.
const petitionCoverPhotosTail = `AND subject_type = $2 AND subject_id = ANY($3) AND purpose = $4 AND uploaded_by = $5
	AND deleted_at IS NULL AND status IN ('stored', 'ready')
	ORDER BY subject_id, created_at, id`

// errCoverPetitionList: more ids than one page holds, an empty id, or one id twice.
var errCoverPetitionList = errors.New("stored_file: danh sách phiếu đọc ảnh bìa không hợp lệ")

// PetitionCoverPhotos reads the COVER of each petition of one register page in ONE statement: the
// oldest citizen scene photo per petition, exactly the first item PetitionPhotos would return for it.
// A petition with no such photo is simply absent. Rows come back at most 5 per petition (migration
// 0026's trigger) and the first per petition is kept here — a DISTINCT ON would do the same in SQL,
// and is not used so the statement keeps Scoped.Query's `SELECT <cols> FROM` shape.
//
// petitionIDs are INTERNAL ids the caller has just read from the commune's register (which already
// applied the restricted-field filter); the commune is $1 from the context, so another commune's id
// matches nothing. At most page.MaxLimit ids, distinct and non-empty — anything else is not a page.
func (s *StoredFileStore) PetitionCoverPhotos(ctx context.Context, petitionIDs []string) ([]domain.StoredFile, error) {
	out := []domain.StoredFile{}
	if len(petitionIDs) == 0 {
		return out, nil
	}
	if len(petitionIDs) > page.MaxLimit || !distinctNonEmpty(petitionIDs) {
		return nil, errCoverPetitionList
	}
	rows, err := s.db.For(ctx).Query(ctx, storedFileCols, "stored_file", petitionCoverPhotosTail,
		domain.StoredFileSubjectPetition, petitionIDs, domain.PurposePetitionPhoto, domain.CitizenLogActor)
	if err != nil {
		return nil, fmt.Errorf("stored_file: đọc ảnh bìa phản ánh: %w", err)
	}
	defer rows.Close()
	seen := make(map[string]bool, len(petitionIDs))
	for rows.Next() {
		f, err := scanStoredFile(rows)
		if err != nil {
			return nil, err
		}
		if seen[f.SubjectID] {
			continue
		}
		seen[f.SubjectID] = true
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stored_file: duyệt ảnh bìa phản ánh: %w", err)
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
