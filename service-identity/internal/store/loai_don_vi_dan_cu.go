package store

import (
	"context"
	"errors"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// LoaiDonViDanCuStore reads the commune's residential-unit-type catalogue. GET
// /api/v1/residential-unit-types
//
// IT DECIDES NOTHING AND IT WRITES NOTHING. There is no write path here on purpose: whether a
// commune may edit the CODE LIST at all, or only its labels and order, is open question #21 and is
// unanswered. A half-written write path looks like a decision somebody made.
type LoaiDonViDanCuStore struct {
	db *store.DB
}

func NewLoaiDonViDanCuStore(db *store.DB) *LoaiDonViDanCuStore {
	return &LoaiDonViDanCuStore{db: db}
}

// bangLoaiDonViDanCu is the table name docDanhMuc interpolates. A constant, never a value from a
// request — see the contract on docDanhMuc.
const bangLoaiDonViDanCu = "loai_don_vi_dan_cu"

// TranDanhMucLoaiDonViDanCu is the hard upper bound on one commune's residential-unit-type
// catalogue.
//
// Same argument as TranDanhMucBoPhan: one process serves 200+ communes, so every list route is a
// shared resource and an unbounded one is forbidden (skills/rest-api-design §5); this route
// deliberately returns the WHOLE list, so the bound cannot come from a `limit` parameter and has to
// be a ceiling the commune's data is checked against.
//
// 100 IS FIFTY TIMES THE REAL FIGURE, AND THE REAL FIGURE MAY WELL BE EXACTLY TWO. The shipped
// codes are `thon` and `to-dan-pho` (migration 0005:216), and whether a commune has any business
// adding a third is the open question this table carries. The number is not a guess at how many
// there might be; it is the point past which the data has stopped being a classification of
// residential units — a loop that inserted rows, an import run twice, a test fixture on a live
// database.
const TranDanhMucLoaiDonViDanCu = 100

// ErrQuaNhieuLoaiDonViDanCu says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE: this list fills the `Loại` picker on the residential-unit form and
// the filter above the list. A silently short list is a classification that has disappeared from
// the picker, so the next unit created is classified wrongly or not at all — and `thon_to_dan_pho`
// carries that code onward to every screen that groups by it. A refusal breaks one commune's screen
// loudly and names itself in the log. Between a wrong answer nobody notices and no answer somebody
// fixes, this system chooses the second (fail closed).
var ErrQuaNhieuLoaiDonViDanCu = errors.New("loai_don_vi_dan_cu: vượt trần danh mục")

// DanhSach reads the commune's whole residential-unit-type catalogue, ordered.
//
// The statement, the ordering, the soft-delete predicate and the refusal all live in docDanhMuc —
// this catalogue and `khoi_nhiem_vu` are the same shape by the migration's own design, and the
// argument for reading them with one statement is stated there.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it from the context (rule 1,
// invariant 5), so this can only ever read the catalogue of the commune the request arrived in.
func (s *LoaiDonViDanCuStore) DanhSach(ctx context.Context) ([]domain.LoaiDonViDanCu, error) {
	dong, err := docDanhMuc(ctx, s.db, bangLoaiDonViDanCu,
		TranDanhMucLoaiDonViDanCu, ErrQuaNhieuLoaiDonViDanCu)
	if err != nil {
		return nil, err
	}

	ra := make([]domain.LoaiDonViDanCu, 0, len(dong))
	for _, d := range dong {
		ra = append(ra, domain.LoaiDonViDanCu{
			ID: d.ID, Ma: d.Ma, Nhan: d.Nhan, LaMacDinh: d.LaMacDinh, DangDung: d.DangDung,
		})
	}
	return ra, nil
}
