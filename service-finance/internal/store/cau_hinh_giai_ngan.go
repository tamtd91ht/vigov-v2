package store

// The commune's own disbursement configuration (`cau_hinh_giai_ngan`, migration 0005).
//
// TODAY THAT IS ONE VALUE: the slow-project warning threshold of §13 rule 5. The table exists
// because the value belongs to the commune and was living in the vendor's source as a constant —
// rule 1, invariant 10 read backwards, and open question #31.
//
// THERE IS NO WRITE PATH HERE, AND THAT IS WHY THE READ REPORTS ITS SOURCE. 0004's header made the
// right objection to a table nobody fills: it "reads as 'already configurable' while every commune
// silently gets the default". The answer is not to hide the table — it is to make this read unable
// to lie. NguongCanhBaoCham returns the figure AND where it came from, every caller has to handle
// both, and the API sends both out, so a commune sitting on the software's default can be told so
// by name. The screen that writes a row is the web's work and needs a permission decision that has
// not been made.

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// CauHinhGiaiNganStore reads one commune's disbursement configuration.
//
// It holds *store.DB and never a *sql.DB. The commune is taken from the context on every call
// through db.For(ctx), so there is no method here that could read another commune's configuration
// (rule 1, invariant 5). That matters more here than on an ordinary read: this value decides
// whether a commune's KPI card says "29 dự án chậm" or "0", so reading the wrong commune's row
// would produce a figure that is internally consistent and wrong.
type CauHinhGiaiNganStore struct {
	db *store.DB
}

func NewCauHinhGiaiNganStore(db *store.DB) *CauHinhGiaiNganStore {
	return &CauHinhGiaiNganStore{db: db}
}

// ErrNamNgoaiKhoang is a budget year outside the window the table admits.
//
// CHECKED HERE RATHER THAN LEFT TO `cau_hinh_giai_ngan_nam_hop_le`: a year of 1026 or 20226 reaching
// this read would simply find no row and return the default, which is indistinguishable from a
// commune that has set nothing. The caller's own year validation is upstream of this; this is the
// backstop that keeps a nonsense year from being answered with a plausible threshold.
var ErrNamNgoaiKhoang = errors.New("cau_hinh_giai_ngan: năm ngân sách ngoài khoảng 2000..2100")

// NguongCanhBaoCham reads the threshold in force for this commune in one budget year.
//
// NO ROW IS NOT AN ERROR — it is the ordinary state of every commune today, and it is answered with
// domain.MacDinhCuaPhanMem(): the software's 10 points, MARKED AS THE SOFTWARE'S. That second half
// is the whole point of the type; an ordinary `(PhanVan, error)` signature would make "the commune
// chose 10 points" and "nobody has chosen anything" the same answer, which is the failure this
// table was created to stop.
//
// THE STORED VALUE IS VALIDATED ON THE WAY OUT, not only on the way in. The CHECK constraint in
// migration 0005 is the floor, but this service will not be the only writer forever — an import, a
// support script, a future onboarding step — and a threshold wrong by a factor of a hundred changes
// a reported figure without anything looking broken. A row outside the range is REFUSED rather than
// silently replaced by the default: falling back would hide a misconfiguration behind a number that
// looks right, and the commune would never find out (fail closed).
func (s *CauHinhGiaiNganStore) NguongCanhBaoCham(ctx context.Context,
	nam int) (domain.NguongCanhBaoCham, error) {

	if nam < 2000 || nam > 2100 {
		return domain.NguongCanhBaoCham{}, ErrNamNgoaiKhoang
	}

	// Scoped.Query binds `tenant_id` to $1 from the context; the caller's own placeholders start at
	// $2. There is no signature here that omits the commune (rule 1, invariant 5).
	rows, err := s.db.For(ctx).Query(ctx, "nguong_canh_bao_cham", "cau_hinh_giai_ngan",
		"AND nam = $2", nam)
	if err != nil {
		return domain.NguongCanhBaoCham{}, fmt.Errorf("cau_hinh_giai_ngan: đọc ngưỡng: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.NguongCanhBaoCham{}, fmt.Errorf("cau_hinh_giai_ngan: đọc ngưỡng: %w", err)
		}
		// PRIMARY KEY (tenant_id, nam) admits at most one row, so "no row" is the only other case.
		return domain.MacDinhCuaPhanMem(), nil
	}

	var gia int64
	if err := rows.Scan(&gia); err != nil {
		return domain.NguongCanhBaoCham{}, fmt.Errorf("cau_hinh_giai_ngan: đọc dòng: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.NguongCanhBaoCham{}, fmt.Errorf("cau_hinh_giai_ngan: duyệt kết quả: %w", err)
	}
	if err := domain.KiemTraNguong(domain.PhanVan(gia)); err != nil {
		return domain.NguongCanhBaoCham{}, fmt.Errorf("cau_hinh_giai_ngan: %w", err)
	}
	return domain.CuaXa(domain.PhanVan(gia)), nil
}
