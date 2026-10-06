package store

// The two reads the Excel import of disbursement vouchers plans against (app/disbursement_import.go).
// The writes are the form's own statements on this store — ProjectForVoucherWrite, NguonVonConSong,
// Chen — so the import cannot write a row the form could not.
//
// BOTH TAKE THE CALLER'S TRANSACTION, the reason every method of chung_tu_giai_ngan.go does: the plan
// and the writes must see one state of the commune's projects and sources.

import (
	"context"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// projectsForVoucherImport is projectForVoucherWrite for MANY codes at once — the same `FOR SHARE`, for
// the same reason (chung_tu_giai_ngan.go, projectForVoucherWrite): no allocation edit or project
// removal can interleave between the plan that judged a row's source and the INSERT. Two imports, or an
// import and the form, do not queue behind each other: share locks are compatible.
//
// THE CODE IS MATCHED EXACTLY. `UNIQUE (tenant_id, ma)` (0004) compares exactly, and a project code
// keeps the case the commune typed off its paper decision (domain.ChuanHoaMaDuAn).
const projectsForVoucherImport = `SELECT id, ma FROM du_an
	WHERE tenant_id = $1 AND ma = ANY($2) AND deleted_at IS NULL
	ORDER BY ma
	FOR SHARE`

// allocatedSourcesOfProjects is allocatedSourcesOfProject for several projects — the same set the
// detail screen draws and the form's create decides on.
const allocatedSourcesOfProjects = `SELECT pb.du_an_id, pb.nguon_von_id
	FROM phan_bo_nguon_von pb
	JOIN nguon_von nv
	  ON nv.tenant_id = pb.tenant_id AND nv.tenant_id = $1 AND nv.id = pb.nguon_von_id
	WHERE pb.tenant_id = $1 AND pb.du_an_id = ANY($2) AND pb.deleted_at IS NULL
	  AND nv.deleted_at IS NULL
	ORDER BY pb.du_an_id, pb.nguon_von_id
	LIMIT $3`

// ProjectsForVoucherImport reads the LIVE projects of this commune whose code is in `codes`, each with
// the sources of its live allocation lines. A code with no live project in THIS commune is simply
// absent from the answer — another commune's project included, since $1 binds the commune (rule 1).
func (s *ChungTuGiaiNganStore) ProjectsForVoucherImport(ctx context.Context, tx *store.ScopedTx,
	codes []string) ([]domain.VoucherImportProject, error) {

	if len(codes) == 0 {
		return nil, nil
	}
	if len(codes) > domain.MaxVoucherImportRows {
		return nil, fmt.Errorf("chung_tu_giai_ngan: nhập Excel hỏi %d mã dự án, trần %d", len(codes), domain.MaxVoucherImportRows)
	}
	tid := string(tx.TenantID())
	rows, err := tx.Underlying().QueryContext(ctx, projectsForVoucherImport, tid, codes)
	if err != nil {
		return nil, fmt.Errorf("chung_tu_giai_ngan: khoá chia sẻ các dự án để nhập: %w", err)
	}
	var out []domain.VoucherImportProject
	index := make(map[string]int, len(codes))
	for rows.Next() {
		var p domain.VoucherImportProject
		if err := rows.Scan(&p.ID, &p.Code); err != nil {
			rows.Close()
			return nil, fmt.Errorf("chung_tu_giai_ngan: đọc dự án để nhập: %w", err)
		}
		index[p.ID] = len(out)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("chung_tu_giai_ngan: duyệt dự án để nhập: %w", err)
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}

	ids := make([]string, 0, len(out))
	for _, p := range out {
		ids = append(ids, p.ID)
	}
	ceiling := len(ids) * TranPhanBoMotDuAn
	lines, err := tx.Underlying().QueryContext(ctx, allocatedSourcesOfProjects, tid, ids, ceiling+1)
	if err != nil {
		return nil, fmt.Errorf("chung_tu_giai_ngan: đọc nguồn được phân bổ để nhập: %w", err)
	}
	defer lines.Close()
	n := 0
	for lines.Next() {
		var projectID, sourceID string
		if err := lines.Scan(&projectID, &sourceID); err != nil {
			return nil, fmt.Errorf("chung_tu_giai_ngan: đọc dòng nguồn được phân bổ để nhập: %w", err)
		}
		n++
		i, ok := index[projectID]
		if !ok {
			continue
		}
		out[i].Allocated = append(out[i].Allocated, sourceID)
		// REFUSED, NOT TRUNCATED — ProjectForVoucherWrite's rule: a short list could refuse a source the
		// project really draws on.
		if len(out[i].Allocated) > TranPhanBoMotDuAn {
			return nil, ErrQuaNhieuPhanBo
		}
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("chung_tu_giai_ngan: duyệt nguồn được phân bổ để nhập: %w", err)
	}
	if n > ceiling {
		return nil, ErrQuaNhieuPhanBo
	}
	return out, nil
}

// liveSourcesForVoucherImport is the commune's live catalogue, names as stored (trimmed by the
// `nguon_von_name_trimmed` CHECK, 0013).
const liveSourcesForVoucherImport = `SELECT id, ten FROM nguon_von
	WHERE tenant_id = $1 AND deleted_at IS NULL
	ORDER BY ten
	LIMIT $2`

// SourcesForVoucherImport reads the commune's LIVE funding sources. Over TranNguonVonMotNam is
// ErrQuaNhieuNguonVon — refused rather than truncated, or a name past the cut would read as unknown.
func (s *ChungTuGiaiNganStore) SourcesForVoucherImport(ctx context.Context, tx *store.ScopedTx) ([]domain.VoucherImportSource, error) {
	rows, err := tx.Underlying().QueryContext(ctx, liveSourcesForVoucherImport, string(tx.TenantID()), TranNguonVonMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("chung_tu_giai_ngan: đọc danh mục nguồn vốn để nhập: %w", err)
	}
	defer rows.Close()
	var out []domain.VoucherImportSource
	for rows.Next() {
		var src domain.VoucherImportSource
		if err := rows.Scan(&src.ID, &src.Name); err != nil {
			return nil, fmt.Errorf("chung_tu_giai_ngan: đọc dòng nguồn vốn để nhập: %w", err)
		}
		out = append(out, src)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chung_tu_giai_ngan: duyệt danh mục nguồn vốn để nhập: %w", err)
	}
	if len(out) > TranNguonVonMotNam {
		return nil, ErrQuaNhieuNguonVon
	}
	return out, nil
}
