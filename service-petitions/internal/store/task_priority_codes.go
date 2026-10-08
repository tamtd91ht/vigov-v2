package store

// The read behind ResolveTaskPriorityCodes (proto/vigov/petitions/v1/petitions.proto): which of the
// asked codes a LIVE `muc_uu_tien_nhiem_vu` row of the commune in the context carries, and whether that
// row is switched on. SQL, and nothing else — the ceiling, the blank refusal and the de-duplication are
// the gRPC handler's, where the contract names them.
//
// Commune $1 from the context (rule 1, invariant 5); live rows only (rule 7, invariant 2).
// `dang_dung` is deliberately NOT a filter: a switched-off priority is answered with active=false, and
// the CALLER refuses it — "absent" and "switched off" are two different facts the contract keeps apart.

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// StatesByCode answers, for the codes given, the live priorities of this commune carrying exactly that
// `ma`. EXACT MATCH: no trim, no lowercase — the caller stores the code it sent, so a normalised match
// would approve a string other than the one written. A code no live row carries is simply absent.
//
// The SELECT list is `ma, dang_dung` and the Scan reads them in that order. NOT cotMucUuTien: that list
// carries `la_mac_dinh` beside `dang_dung`, the adjacent pair 9fff0136 found read swapped — a two-column
// list here has no neighbour to confuse it with.
//
// An empty list answers empty WITHOUT a statement: `ma IN ()` is a syntax error, and "no codes" must
// never be read as "every priority".
func (s *MucUuTienNhiemVuStore) StatesByCode(ctx context.Context, codes []string) ([]domain.TaskPriorityCodeState, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	// $1 is the commune, bound by Scoped.Query; the codes start at $2.
	args := make([]any, len(codes))
	marks := make([]string, len(codes))
	for i, c := range codes {
		args[i] = c
		marks[i] = "$" + strconv.Itoa(i+2)
	}
	tail := "AND deleted_at IS NULL AND ma IN (" + strings.Join(marks, ", ") + ")"

	rows, err := s.db.For(ctx).Query(ctx, "ma, dang_dung", "muc_uu_tien_nhiem_vu", tail, args...)
	if err != nil {
		return nil, fmt.Errorf("muc_uu_tien_nhiem_vu: tra trạng thái theo mã: %w", err)
	}
	defer rows.Close()

	out := make([]domain.TaskPriorityCodeState, 0, len(codes))
	for rows.Next() {
		var st domain.TaskPriorityCodeState
		if err := rows.Scan(&st.Code, &st.Active); err != nil {
			return nil, fmt.Errorf("muc_uu_tien_nhiem_vu: tra trạng thái theo mã: đọc dòng: %w", err)
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("muc_uu_tien_nhiem_vu: tra trạng thái theo mã: %w", err)
	}
	return out, nil
}
