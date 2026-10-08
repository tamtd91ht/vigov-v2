package store

// The read behind ResolveDocumentTypeCodes (proto/vigov/documents/v1/documents.proto): which of the
// asked codes a LIVE `loai_van_ban` row of the commune in the context carries, and whether that row
// is switched on. SQL, and nothing else — the ceiling, the blank refusal and the de-duplication are
// the gRPC handler's, where the contract names them.
//
// Commune $1 from the context (rule 1, invariant 5); live rows only (rule 7, invariant 2).
// `dang_dung` is deliberately NOT a filter: a switched-off type is answered with active=false, and
// the CALLER refuses it — "absent" and "switched off" are two different facts the contract keeps
// apart.

import (
	"context"
	"fmt"
	"strings"

	"github.com/vihat/vigov/service-documents/internal/domain"
)

// StatesByCode answers, for the codes given, the live rows of this commune carrying exactly that
// `ma`. EXACT MATCH: no trim, no lowercase — the caller stores the code it sent, so a normalised match
// would approve a string other than the one written. A code no live row carries is simply absent.
//
// An empty list answers empty WITHOUT a statement: `ma IN ()` is a syntax error, and "no codes" must
// never be read as "every type".
func (s *LoaiVanBanStore) StatesByCode(ctx context.Context, codes []string) ([]domain.DocumentTypeCodeState, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	var args []any
	bind := newBinder(&args)
	marks := make([]string, len(codes))
	for i, c := range codes {
		marks[i] = bind(c)
	}
	tail := "AND deleted_at IS NULL AND ma IN (" + strings.Join(marks, ", ") + ")"

	rows, err := s.db.For(ctx).Query(ctx, "ma, dang_dung", "loai_van_ban", tail, args...)
	if err != nil {
		return nil, fmt.Errorf("loai_van_ban: tra trạng thái theo mã: %w", err)
	}
	defer rows.Close()

	out := make([]domain.DocumentTypeCodeState, 0, len(codes))
	for rows.Next() {
		var st domain.DocumentTypeCodeState
		if err := rows.Scan(&st.Code, &st.Active); err != nil {
			return nil, fmt.Errorf("loai_van_ban: tra trạng thái theo mã: đọc dòng: %w", err)
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loai_van_ban: tra trạng thái theo mã: %w", err)
	}
	return out, nil
}
