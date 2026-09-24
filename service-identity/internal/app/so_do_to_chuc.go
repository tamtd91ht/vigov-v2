package app

// The use cases behind the WRITE surface of the commune's org chart (14-cau-hinh.md §1), under
// `admin.org` ("Quản lý sơ đồ tổ chức", migration 0001:282) — user decision 2026-09-24.
//
//	Them  add a unit, at the root or under a parent
//	Sua   rename it, move it under another parent (or to the root), change its rank
//
// THERE IS NO XOA, deliberately. Removing a unit must be refused while it still holds staff OR is
// named by records in `documents`, `petitions` or `comms`; this service can see the first and cannot
// see the other three without a cross-service contract nobody has designed (rule 2, stop condition
// #2). A delete that checked only the half it can see would orphan the other half silently.
//
// ANY KIND OF UNIT IS ACCEPTED — Đảng uỷ, HĐND, MTTQ as well as the UBND's own units (user decision
// 2026-09-24; the reason the resource is `org-units`, domain.BoPhan). There is no kind column and no
// check on one.
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with
// the business write, and core/audit.Write takes a *store.ScopedTx. Opening it is this layer's job.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// KhoBoPhan is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION, so the
// write and its audit entry cannot land in two.
type KhoBoPhan interface {
	KhoaBoPhan(ctx context.Context, tx *store.ScopedTx, id string) (domain.BoPhan, bool, error)
	MaCungGoc(ctx context.Context, tx *store.ScopedTx, goc string) ([]string, error)
	Chen(ctx context.Context, tx *store.ScopedTx, bp domain.BoPhan) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, bp domain.BoPhan) error
}

// The business verbs written into the trail.
const (
	HanhViThemBoPhan = "them_bo_phan"
	HanhViSuaBoPhan  = "sua_bo_phan"
)

// ErrCayBoPhanVongLap — the move would make a unit its own ancestor. 409: the caller holds
// `admin.org`; what is refused is the operation against the current shape of the tree.
var ErrCayBoPhanVongLap = errors.New("bo_phan: không thể dời một bộ phận vào dưới chính nó hay bộ phận con của nó")

// tranHauToMa is how many suffixes a generated slug tries (`-2` … `-100`) before answering 409.
// A commune with a hundred units deriving the same slug is not naming units; it is a loop.
const tranHauToMa = 100

// SoDoToChuc owns the write surface of one commune's org chart.
type SoDoToChuc struct {
	db  *store.DB
	kho KhoBoPhan

	// Injected so a test can pin it. In production: ulid.Moi.
	sinhID func() (string, error)
}

func NewSoDoToChuc(db *store.DB, kho KhoBoPhan) *SoDoToChuc {
	return &SoDoToChuc{db: db, kho: kho, sinhID: ulid.Moi}
}

// YeuCauThemBoPhan is the create request.
//
//	Ten    required; trimmed; case kept as typed (domain.ChuanHoaTenBoPhan)
//	ChaID  "" = the root
//	ThuTu  nil = 0, the column's own default
//	Ma     "" = derived from Ten, with `-2`, `-3`… on collision. NON-EMPTY = the code the person
//	       typed, used EXACTLY or refused with 409 — never silently suffixed, because a person who
//	       typed a code and got a different one has a code they did not choose, permanently.
type YeuCauThemBoPhan struct {
	Ten   string
	ChaID string
	ThuTu *int
	Ma    string
}

// Them adds one unit.
func (uc *SoDoToChuc) Them(ctx context.Context, yc YeuCauThemBoPhan, nguoi NguoiThucHien) (domain.BoPhan, error) {
	if err := nguoi.hopLe(); err != nil {
		return domain.BoPhan{}, err
	}
	// Shape first, outside the transaction: a request that fails its shape holds no lock.
	moi, err := dungBoPhanMoi(yc)
	if err != nil {
		return domain.BoPhan{}, err
	}
	maTuNhap := yc.Ma != ""

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if moi.ChaID != "" {
			// THE PARENT IS READ THROUGH THE SCOPED LOCK, so another commune's unit is simply not
			// found (rule 1). Held until commit so it cannot be moved away mid-flight.
			_, daXoa, err := uc.kho.KhoaBoPhan(ctx, tx, moi.ChaID)
			if errors.Is(err, idstore.ErrKhongTimThayBoPhan) || (err == nil && daXoa) {
				return idstore.ErrBoPhanChaKhongTonTai
			}
			if err != nil {
				return err
			}
		}

		dangCo, err := uc.kho.MaCungGoc(ctx, tx, moi.Ma)
		if err != nil {
			return err
		}
		ma, err := chonMaTrong(moi.Ma, dangCo, maTuNhap)
		if err != nil {
			return err
		}
		moi.Ma = ma

		if moi.ID, err = uc.sinhID(); err != nil {
			return fmt.Errorf("bo_phan: sinh id: %w", err)
		}
		if err := uc.kho.Chen(ctx, tx, moi); err != nil {
			return err
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). The subject is the unit's CODE, the
		// business identifier an inspection can read (rule 6, invariant 2); TenantID is filled by
		// audit.Write from the transaction.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViThemBoPhan,
			Subject: moi.Ma,
			Delta:   deltaBoPhan(map[string]any{"sau": vetBoPhan(moi)}),
		})
	})
	if err != nil {
		return domain.BoPhan{}, err
	}
	return moi, nil
}

func dungBoPhanMoi(yc YeuCauThemBoPhan) (domain.BoPhan, error) {
	ten, err := domain.ChuanHoaTenBoPhan(yc.Ten)
	if err != nil {
		return domain.BoPhan{}, err
	}
	if err := domain.KiemTraIDCha(yc.ChaID); err != nil {
		return domain.BoPhan{}, err
	}
	thuTu := 0
	if yc.ThuTu != nil {
		thuTu = *yc.ThuTu
	}
	if err := domain.KiemTraThuTuBoPhan(thuTu); err != nil {
		return domain.BoPhan{}, err
	}
	var ma string
	if yc.Ma != "" {
		ma, err = domain.ChuanHoaMaBoPhan(yc.Ma)
	} else {
		ma, err = domain.SinhMaBoPhan(ten)
	}
	if err != nil {
		return domain.BoPhan{}, err
	}
	return domain.BoPhan{Ten: ten, Ma: ma, ChaID: yc.ChaID, ThuTu: thuTu}, nil
}

// chonMaTrong picks the code to issue. A typed code is taken exactly or refused; a derived one takes
// the first free candidate of goc, goc-2, goc-3, … — "free" counting soft-deleted rows, because the
// unique key does.
func chonMaTrong(goc string, dangCo []string, maTuNhap bool) (string, error) {
	co := make(map[string]bool, len(dangCo))
	for _, m := range dangCo {
		co[m] = true
	}
	if maTuNhap {
		if co[goc] {
			return "", idstore.ErrMaBoPhanDaDung
		}
		return goc, nil
	}
	for n := 1; n <= tranHauToMa; n++ {
		if ung := domain.MaBoPhanThuN(goc, n); !co[ung] {
			return ung, nil
		}
	}
	return "", idstore.ErrMaBoPhanDaDung
}

// YeuCauSuaBoPhan is a PARTIAL edit: nil = leave alone.
//
// ChaID: nil = unchanged, "" = MOVE TO THE ROOT, an id = move under that unit. "" is the root
// because that is what the read route already prints for a root's `parent_id`, so a client writes
// back the value it read.
//
// THERE IS NO Ma. The code is an issued identifier (rule 7, invariant 3); the HTTP layer refuses a
// body that names it rather than ignoring it.
type YeuCauSuaBoPhan struct {
	Ten   *string
	ChaID *string
	ThuTu *int
}

// Sua renames, moves or re-ranks one unit.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — same discipline as SLA.Sua, and what makes the route
// honestly idempotent.
//
// =================================================================================================
// THE CYCLE CHECK. Moving X under P is refused when X is P or an ancestor of P. It walks UP from P,
// one row at a time, LOCKING each row (idstore.BoPhanStore.KhoaBoPhan), inside the same transaction
// as the UPDATE:
//
//   - upward and not downward: the chain from P to the root is a single path, bounded by the depth;
//     the subtree under X can be wide.
//   - row by row and not a recursive CTE: PostgreSQL refuses FOR UPDATE inside WITH RECURSIVE, and
//     an unlocked walk is a check with a gap through which two crossing moves both pass.
//   - bounded by idstore.TranDanhMucBoPhan steps: a chain longer than the whole commune's ceiling can
//     only be a loop ALREADY in the data (the schema cannot refuse one). That is answered as a
//     failure, never as "no cycle".
//
// X itself is locked first, so it cannot move while its would-be ancestors are read.
// =================================================================================================
func (uc *SoDoToChuc) Sua(ctx context.Context, id string, yc YeuCauSuaBoPhan, nguoi NguoiThucHien) (domain.BoPhan, error) {
	if err := nguoi.hopLe(); err != nil {
		return domain.BoPhan{}, err
	}
	if id == "" {
		return domain.BoPhan{}, idstore.ErrKhongTimThayBoPhan
	}
	var ten string
	if yc.Ten != nil {
		var err error
		if ten, err = domain.ChuanHoaTenBoPhan(*yc.Ten); err != nil {
			return domain.BoPhan{}, err
		}
	}
	if yc.ChaID != nil {
		if err := domain.KiemTraIDCha(*yc.ChaID); err != nil {
			return domain.BoPhan{}, err
		}
	}
	if yc.ThuTu != nil {
		if err := domain.KiemTraThuTuBoPhan(*yc.ThuTu); err != nil {
			return domain.BoPhan{}, err
		}
	}

	var sau domain.BoPhan
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		truoc, daXoa, err := uc.kho.KhoaBoPhan(ctx, tx, id)
		if err != nil {
			return err
		}
		if daXoa {
			return idstore.ErrKhongTimThayBoPhan
		}

		sau = truoc
		if yc.Ten != nil {
			sau.Ten = ten
		}
		if yc.ChaID != nil {
			sau.ChaID = *yc.ChaID
		}
		if yc.ThuTu != nil {
			sau.ThuTu = *yc.ThuTu
		}
		if sau == truoc {
			return nil
		}

		if sau.ChaID != truoc.ChaID && sau.ChaID != "" {
			if err := uc.kiemVongLap(ctx, tx, truoc.ID, sau.ChaID); err != nil {
				return err
			}
		}

		if err := uc.kho.CapNhat(ctx, tx, sau); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  HanhViSuaBoPhan,
			Subject: truoc.Ma,
			Delta: deltaBoPhan(map[string]any{
				"truoc": vetBoPhan(truoc),
				"sau":   vetBoPhan(sau),
			}),
		})
	})
	if err != nil {
		return domain.BoPhan{}, err
	}
	return sau, nil
}

// kiemVongLap walks from the new parent up to the root under row locks; see Sua.
func (uc *SoDoToChuc) kiemVongLap(ctx context.Context, tx *store.ScopedTx, idNut, idChaMoi string) error {
	cur := idChaMoi
	for buoc := 0; cur != ""; buoc++ {
		if cur == idNut {
			return ErrCayBoPhanVongLap
		}
		if buoc > idstore.TranDanhMucBoPhan {
			return fmt.Errorf("bo_phan: chuỗi bộ phận cha dài hơn %d bước — dữ liệu đã có vòng lặp", idstore.TranDanhMucBoPhan)
		}
		nut, daXoa, err := uc.kho.KhoaBoPhan(ctx, tx, cur)
		if buoc == 0 {
			// The new parent itself must be a LIVE unit of this commune. Another commune's id is
			// not found by the scoped lock (rule 1).
			if errors.Is(err, idstore.ErrKhongTimThayBoPhan) || (err == nil && daXoa) {
				return idstore.ErrBoPhanChaKhongTonTai
			}
		}
		if err != nil {
			// Past the first step a missing ancestor means the foreign key did not hold — not the
			// caller's fault, and never read as "no cycle".
			return fmt.Errorf("bo_phan: đọc bộ phận tổ tiên: %w", err)
		}
		cur = nut.ChaID
	}
	return nil
}

// vetBoPhan is the before/after payload: the three editable fields plus the code.
//
// `cha_id` IS THE PARENT'S ULID, and that is acceptable here where it would not be as a subject:
// `bo_phan` rows are never hard-deleted (rule 7), so the id stays resolvable to a name for as long
// as the trail is kept. NOTHING HERE IS PERSONAL DATA (rule 3, rule 6 forbidden #4).
func vetBoPhan(bp domain.BoPhan) map[string]any {
	return map[string]any{"ma": bp.Ma, "ten": bp.Ten, "cha_id": bp.ChaID, "thu_tu": bp.ThuTu}
}

// deltaBoPhan marshals the payload; a failure yields an explicit marker, never a nil delta.
func deltaBoPhan(v map[string]any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"loi":"khong_dung_duoc_delta"}`)
	}
	return b
}

// LaLoiDauVaoBoPhan reports whether this is a refusal of what the client sent (400). Listed
// explicitly rather than defaulting to 400, for the reason LaLoiDauVaoSLA gives.
func LaLoiDauVaoBoPhan(err error) bool {
	for _, e := range []error{
		domain.ErrThieuTenBoPhan, domain.ErrTenBoPhanQuaDai, domain.ErrTenBoPhanKyTuLa,
		domain.ErrMaBoPhanSaiDinhDang, domain.ErrMaBoPhanQuaDai, domain.ErrTenKhongSinhDuocMa,
		domain.ErrThuTuBoPhanAm, domain.ErrThuTuBoPhanQuaLon, domain.ErrIDChaQuaDai,
	} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
