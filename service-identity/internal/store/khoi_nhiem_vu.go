package store

import (
	"context"
	"errors"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// KhoiNhiemVuStore reads and writes the commune's task-bloc catalogue.
// GET / POST / PATCH / DELETE /api/v1/task-blocs
//
// THE TABLE IS IN THIS SERVICE ALTHOUGH THE ENTITY NAME CARRIES "Task" — the argument is on
// domain.KhoiNhiemVu and in migration 0005:300, and moving it to `petitions` is forbidden in
// advance by ADR 0024:130. What matters here: `petitions` reads these codes over the service
// contract, never by joining into this schema (rule 2, invariant 3).
//
// A FULL CATALOGUE since the user's decision of 2026-09-24 — same wrappers, same statements as
// LoaiDonViDanCuStore (danh_muc_ghi.go).
type KhoiNhiemVuStore struct {
	db *store.DB
}

func NewKhoiNhiemVuStore(db *store.DB) *KhoiNhiemVuStore { return &KhoiNhiemVuStore{db: db} }

// bangKhoiNhiemVu is the table name docDanhMuc interpolates. A constant, never a value from a
// request — see the contract on docDanhMuc.
const bangKhoiNhiemVu = "khoi_nhiem_vu"

// TranDanhMucKhoiNhiemVu is the hard upper bound on one commune's task-bloc catalogue.
//
// Same argument as TranDanhMucLoaiDonViDanCu, and the same number for the same reason: the list is
// `Khối Uỷ ban` · `Khối Đảng` · `Khác` (migration 0005:298) — three rows, describing which arm of
// the apparatus a task belongs to. A hundred blocs is not a large commune; it is data that has
// stopped being a list of blocs.
const TranDanhMucKhoiNhiemVu = 100

// ErrQuaNhieuKhoiNhiemVu says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE, and the cost of getting this wrong is paid in another service: the
// bloc picker sits on the task form in `petitions`, which holds the chosen code AS A VALUE. A
// silently short list is a task filed under the wrong arm of the apparatus — and the filter that
// later counts tasks per bloc reports a figure that is simply false, with nothing on any screen
// saying so.
var ErrQuaNhieuKhoiNhiemVu = errors.New("khoi_nhiem_vu: vượt trần danh mục")

// DanhSach reads the commune's whole task-bloc catalogue, ordered.
//
// The statement, the ordering, the soft-delete predicate and the refusal live in docDanhMuc — see
// the argument there for why the two catalogues share one read.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it from the context (rule 1,
// invariant 5).
func (s *KhoiNhiemVuStore) DanhSach(ctx context.Context) ([]domain.KhoiNhiemVu, error) {
	dong, err := docDanhMuc(ctx, s.db, bangKhoiNhiemVu,
		TranDanhMucKhoiNhiemVu, ErrQuaNhieuKhoiNhiemVu)
	if err != nil {
		return nil, err
	}

	ra := make([]domain.KhoiNhiemVu, 0, len(dong))
	for _, d := range dong {
		ra = append(ra, khoiNhiemVuTuDong(d))
	}
	return ra, nil
}

func khoiNhiemVuTuDong(d dongDanhMuc) domain.KhoiNhiemVu {
	return domain.KhoiNhiemVu{
		ID: d.ID, Ma: d.Ma, Nhan: d.Nhan, LaMacDinh: d.LaMacDinh, DangDung: d.DangDung,
		ThuTu: d.ThuTu, Nguon: d.Nguon, MaNguonReNhanh: d.MaNguonReNhanh,
	}
}

func dongTuKhoiNhiemVu(k domain.KhoiNhiemVu) dongDanhMuc {
	return dongDanhMuc{
		ID: k.ID, Ma: k.Ma, Nhan: k.Nhan, LaMacDinh: k.LaMacDinh, DangDung: k.DangDung,
		ThuTu: k.ThuTu, Nguon: k.Nguon, MaNguonReNhanh: k.MaNguonReNhanh,
	}
}

// --- the write path: typed wrappers, statements in danh_muc_ghi.go --------------------------------

func (s *KhoiNhiemVuStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.KhoiNhiemVu, error) {
	d, err := khoaDongDanhMuc(ctx, tx, bangKhoiNhiemVu, id)
	if err != nil {
		return domain.KhoiNhiemVu{}, err
	}
	return khoiNhiemVuTuDong(d), nil
}

func (s *KhoiNhiemVuStore) MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error) {
	return maDanhMucDaDung(ctx, tx, bangKhoiNhiemVu, ma)
}

func (s *KhoiNhiemVuStore) DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error) {
	return demDanhMucDangSong(ctx, tx, bangKhoiNhiemVu)
}

func (s *KhoiNhiemVuStore) Chen(ctx context.Context, tx *store.ScopedTx, k domain.KhoiNhiemVu) error {
	return chenDanhMuc(ctx, tx, bangKhoiNhiemVu, dongTuKhoiNhiemVu(k))
}

func (s *KhoiNhiemVuStore) BoMacDinhKhac(ctx context.Context, tx *store.ScopedTx, trongID string) error {
	return boMacDinhDanhMucKhac(ctx, tx, bangKhoiNhiemVu, trongID)
}

func (s *KhoiNhiemVuStore) CapNhat(ctx context.Context, tx *store.ScopedTx, k domain.KhoiNhiemVu) error {
	return capNhatDanhMuc(ctx, tx, bangKhoiNhiemVu, dongTuKhoiNhiemVu(k))
}

func (s *KhoiNhiemVuStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	return xoaMemDanhMuc(ctx, tx, bangKhoiNhiemVu, id, boi, lyDo)
}
