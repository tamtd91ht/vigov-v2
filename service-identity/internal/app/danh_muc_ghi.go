package app

// The use cases behind the WRITE surface of this service's two reference catalogues —
// `residential-unit-types` (loai_don_vi_dan_cu) and `task-blocs` (khoi_nhiem_vu), under
// `admin.lookup`. User decision 2026-09-24: both are FULL catalogues (add, relabel, reorder,
// disable/enable, soft-delete commune-added rows) — the same shape as the five catalogues already
// writable in documents, finance, comms and petitions. The rules below are theirs, copied, so the web
// can drive all seven with one piece of generic code.
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 requires the audit entry to share a transaction with the
// business write, and core/audit.Write takes a *store.ScopedTx. Opening it is this layer's job. And
// every write is read-decide-write under a row lock — read the row, derive its tier, refuse or apply
// — which needs the row, the rules and the transaction in one place.
//
// ONE GENERIC USE CASE, TWO INSTANTIATIONS. DanhMucGhi[domain.LoaiDonViDanCu] and
// DanhMucGhi[domain.KhoiNhiemVu] are different types, so a handler holding one cannot be handed the
// other — the property the two domain types exist for. What is shared is the logic, which is
// identical by the migration's own design (one trigger for both tables, 0005:124).
//
// WHAT THIS FILE DELIBERATELY DOES NOT DO: sow a commune's `nguon = 'he-thong'` rows. That is commune
// onboarding (migration 0005:38), which does not exist in this repository.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// KhoDanhMuc is the store, declared at the point of use. EVERY METHOD TAKES THE TRANSACTION, so the
// row and its audit entry cannot land in two. *idstore.LoaiDonViDanCuStore satisfies
// KhoDanhMuc[domain.LoaiDonViDanCu]; *idstore.KhoiNhiemVuStore satisfies KhoDanhMuc[domain.KhoiNhiemVu].
type KhoDanhMuc[T any] interface {
	TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (T, error)
	MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error)
	DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error)
	Chen(ctx context.Context, tx *store.ScopedTx, muc T) error
	BoMacDinhKhac(ctx context.Context, tx *store.ScopedTx, trongID string) error
	CapNhat(ctx context.Context, tx *store.ScopedTx, muc T) error
	XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error
}

// mucDanhMuc is the neutral view of one row this use case reasons over. Unexported: it is the shape
// of the RULES, not a business concept — each catalogue converts to and from its own domain type.
type mucDanhMuc struct {
	ID, Ma, Nhan        string
	ThuTu               int
	LaMacDinh, DangDung bool
	Nguon               string
	MaNguonReNhanh      bool
}

func (m mucDanhMuc) tang() domain.Tang { return domain.TangCua(m.Nguon, m.MaNguonReNhanh) }

// moTaDanhMuc is what differs between the two catalogues: the verbs in the trail, the ceiling, and
// the conversion. Nothing else is allowed to differ.
type moTaDanhMuc[T any] struct {
	ten                              string // table name, for wrapping errors — never a request value
	hanhViThem, hanhViSua, hanhViXoa string
	tran                             int
	sangMuc                          func(T) mucDanhMuc
	tuMuc                            func(mucDanhMuc) T
}

// DanhMucGhi owns adding, editing and retiring one commune's rows of ONE catalogue.
type DanhMucGhi[T any] struct {
	db  *store.DB
	kho KhoDanhMuc[T]
	mo  moTaDanhMuc[T]

	// sinhID is injected so a test can pin the id. In production: ulid.Moi.
	sinhID func() (string, error)
}

// The business verbs written into the trail. THE VERB NAMES THE CATALOGUE, not just the operation:
// `sua_danh_muc` across seven tables would leave the trail unable to say which list changed.
const (
	HanhViThemLoaiDonViDanCu = "them_loai_don_vi_dan_cu"
	HanhViSuaLoaiDonViDanCu  = "sua_loai_don_vi_dan_cu"
	HanhViXoaLoaiDonViDanCu  = "xoa_loai_don_vi_dan_cu"

	HanhViThemKhoiNhiemVu = "them_khoi_nhiem_vu"
	HanhViSuaKhoiNhiemVu  = "sua_khoi_nhiem_vu"
	HanhViXoaKhoiNhiemVu  = "xoa_khoi_nhiem_vu"
)

// NewDanhMucLoaiDonViDanCu builds the write use case of the residential-unit-type catalogue.
func NewDanhMucLoaiDonViDanCu(db *store.DB, kho KhoDanhMuc[domain.LoaiDonViDanCu]) *DanhMucGhi[domain.LoaiDonViDanCu] {
	return &DanhMucGhi[domain.LoaiDonViDanCu]{db: db, kho: kho, sinhID: ulid.Moi, mo: moTaDanhMuc[domain.LoaiDonViDanCu]{
		ten:        "loai_don_vi_dan_cu",
		hanhViThem: HanhViThemLoaiDonViDanCu, hanhViSua: HanhViSuaLoaiDonViDanCu, hanhViXoa: HanhViXoaLoaiDonViDanCu,
		tran: idstore.TranDanhMucLoaiDonViDanCu,
		sangMuc: func(l domain.LoaiDonViDanCu) mucDanhMuc {
			return mucDanhMuc{ID: l.ID, Ma: l.Ma, Nhan: l.Nhan, ThuTu: l.ThuTu, LaMacDinh: l.LaMacDinh,
				DangDung: l.DangDung, Nguon: l.Nguon, MaNguonReNhanh: l.MaNguonReNhanh}
		},
		tuMuc: func(m mucDanhMuc) domain.LoaiDonViDanCu {
			return domain.LoaiDonViDanCu{ID: m.ID, Ma: m.Ma, Nhan: m.Nhan, ThuTu: m.ThuTu, LaMacDinh: m.LaMacDinh,
				DangDung: m.DangDung, Nguon: m.Nguon, MaNguonReNhanh: m.MaNguonReNhanh}
		},
	}}
}

// NewDanhMucKhoiNhiemVu builds the write use case of the task-bloc catalogue.
func NewDanhMucKhoiNhiemVu(db *store.DB, kho KhoDanhMuc[domain.KhoiNhiemVu]) *DanhMucGhi[domain.KhoiNhiemVu] {
	return &DanhMucGhi[domain.KhoiNhiemVu]{db: db, kho: kho, sinhID: ulid.Moi, mo: moTaDanhMuc[domain.KhoiNhiemVu]{
		ten:        "khoi_nhiem_vu",
		hanhViThem: HanhViThemKhoiNhiemVu, hanhViSua: HanhViSuaKhoiNhiemVu, hanhViXoa: HanhViXoaKhoiNhiemVu,
		tran: idstore.TranDanhMucKhoiNhiemVu,
		sangMuc: func(k domain.KhoiNhiemVu) mucDanhMuc {
			return mucDanhMuc{ID: k.ID, Ma: k.Ma, Nhan: k.Nhan, ThuTu: k.ThuTu, LaMacDinh: k.LaMacDinh,
				DangDung: k.DangDung, Nguon: k.Nguon, MaNguonReNhanh: k.MaNguonReNhanh}
		},
		tuMuc: func(m mucDanhMuc) domain.KhoiNhiemVu {
			return domain.KhoiNhiemVu{ID: m.ID, Ma: m.Ma, Nhan: m.Nhan, ThuTu: m.ThuTu, LaMacDinh: m.LaMacDinh,
				DangDung: m.DangDung, Nguon: m.Nguon, MaNguonReNhanh: m.MaNguonReNhanh}
		},
	}}
}

// YeuCauThemDanhMuc is one new row, as it arrives from the handler.
//
// THERE IS NO `Nguon` FIELD AND THERE MUST NEVER BE ONE: provenance decides the tier. NO `DangDung`
// either: a row the commune has just added is in use; creating one disabled is POST then PATCH, and
// the second request is the one that leaves a trail saying somebody turned it off.
type YeuCauThemDanhMuc struct {
	Ma        string
	Nhan      string
	ThuTu     int
	LaMacDinh bool
}

// YeuCauSuaDanhMuc is a PARTIAL edit: nil = leave alone. Three of the four fields have a meaningful
// zero, which a struct of plain values could not tell from "not mentioned". THERE IS NO Ma.
type YeuCauSuaDanhMuc struct {
	Nhan      *string
	ThuTu     *int
	DangDung  *bool
	LaMacDinh *bool
}

// Them adds one row the commune owns (tier 1).
//
// ORDER OF THE REFUSALS: shape first (no lock), then the ceiling, then the duplicate code.
func (uc *DanhMucGhi[T]) Them(ctx context.Context, yc YeuCauThemDanhMuc, nguoi NguoiThucHien) (T, error) {
	var rong T
	if err := nguoi.hopLe(); err != nil {
		return rong, err
	}
	ma, err := domain.ChuanHoaMaDanhMuc(yc.Ma)
	if err != nil {
		return rong, err
	}
	nhan, err := domain.ChuanHoaNhanDanhMuc(yc.Nhan)
	if err != nil {
		return rong, err
	}
	if err := domain.KiemTraThuTuDanhMuc(yc.ThuTu); err != nil {
		return rong, err
	}
	id, err := uc.sinhID()
	if err != nil {
		return rong, fmt.Errorf("%s: sinh id: %w", uc.mo.ten, err)
	}

	moi := mucDanhMuc{
		ID: id, Ma: ma, Nhan: nhan, ThuTu: yc.ThuTu, LaMacDinh: yc.LaMacDinh, DangDung: true,
		// Set only so the value RETURNED describes the row written. The store writes both as literals.
		Nguon: domain.NguonDonVi, MaNguonReNhanh: false,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.kho.DemDangSong(ctx, tx)
		if err != nil {
			return err
		}
		if n >= uc.mo.tran {
			return idstore.ErrDanhMucDayTran
		}
		daDung, err := uc.kho.MaDaDung(ctx, tx, ma)
		if err != nil {
			return err
		}
		if daDung {
			return idstore.ErrMaDaTonTai
		}
		if moi.LaMacDinh {
			// BEFORE the insert: UNIQUE (tenant_id, moc_mac_dinh) admits one live default.
			if err := uc.kho.BoMacDinhKhac(ctx, tx, moi.ID); err != nil {
				return err
			}
		}
		if err := uc.kho.Chen(ctx, tx, uc.mo.tuMuc(moi)); err != nil {
			return err
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). The subject is the CODE; TenantID is
		// filled by audit.Write from the transaction. Nothing in the delta is personal data (rule 3).
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  uc.mo.hanhViThem,
			Subject: moi.Ma,
			Delta:   deltaDanhMuc(map[string]any{"sau": vetDanhMuc(moi)}),
		})
	})
	if err != nil {
		return rong, bocDanhMuc(ctx, uc.mo.ten, "thêm", err)
	}
	return uc.mo.tuMuc(moi), nil
}

// Sua applies a partial edit, refusing whatever this row's tier does not allow.
//
// THE TIER CHECK IS ON THE TRANSITION, not on the requested value: asking a tier-3 row to stay
// enabled is not an attempt to disable it. A NO-OP WRITES NOTHING AND AUDITS NOTHING — which is what
// makes the route's idem.KhongCan declaration true.
func (uc *DanhMucGhi[T]) Sua(ctx context.Context, id string, yc YeuCauSuaDanhMuc, nguoi NguoiThucHien) (T, error) {
	var rong T
	if err := nguoi.hopLe(); err != nil {
		return rong, err
	}
	if id == "" {
		return rong, idstore.ErrDanhMucKhongTonTai
	}
	var nhan string
	if yc.Nhan != nil {
		var err error
		if nhan, err = domain.ChuanHoaNhanDanhMuc(*yc.Nhan); err != nil {
			return rong, err
		}
	}
	if yc.ThuTu != nil {
		if err := domain.KiemTraThuTuDanhMuc(*yc.ThuTu); err != nil {
			return rong, err
		}
	}

	var sau mucDanhMuc
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		dong, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		truoc := uc.mo.sangMuc(dong)

		sau = truoc
		if yc.Nhan != nil {
			sau.Nhan = nhan
		}
		if yc.ThuTu != nil {
			sau.ThuTu = *yc.ThuTu
		}
		if yc.DangDung != nil {
			sau.DangDung = *yc.DangDung
		}
		if yc.LaMacDinh != nil {
			sau.LaMacDinh = *yc.LaMacDinh
		}

		if truoc.DangDung && !sau.DangDung {
			if err := truoc.tang().ChoTat(); err != nil {
				return err
			}
		}
		if sau == truoc {
			return nil
		}
		if sau.LaMacDinh && !truoc.LaMacDinh {
			if err := uc.kho.BoMacDinhKhac(ctx, tx, sau.ID); err != nil {
				return err
			}
		}
		if err := uc.kho.CapNhat(ctx, tx, uc.mo.tuMuc(sau)); err != nil {
			return err
		}
		// BEFORE AND AFTER, ONLY THE FIELDS THAT MOVED (rule 6, invariant 5).
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  uc.mo.hanhViSua,
			Subject: sau.Ma,
			Delta: deltaDanhMuc(map[string]any{
				"truoc": vetDoiDanhMuc(truoc, sau, true),
				"sau":   vetDoiDanhMuc(truoc, sau, false),
			}),
		})
	})
	if err != nil {
		return rong, bocDanhMuc(ctx, uc.mo.ten, "sửa", err)
	}
	return uc.mo.tuMuc(sau), nil
}

// Xoa soft deletes one row — tier 1 only. The row stays with `deleted_at`, `deleted_by` (the
// remover's STAFF CODE, rule 6 invariant 8) and `delete_reason`, and its `ma` stays taken forever.
func (uc *DanhMucGhi[T]) Xoa(ctx context.Context, id, lyDoTho string, nguoi NguoiThucHien) error {
	if err := nguoi.hopLe(); err != nil {
		return err
	}
	if id == "" {
		return idstore.ErrDanhMucKhongTonTai
	}
	lyDo, err := domain.ChuanHoaLyDoXoaDanhMuc(lyDoTho)
	if err != nil {
		return err
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		dong, err := uc.kho.TheoIDDeSua(ctx, tx, id)
		if err != nil {
			return err
		}
		truoc := uc.mo.sangMuc(dong)
		if err := truoc.tang().ChoXoaMem(); err != nil {
			return err
		}
		if err := uc.kho.XoaMem(ctx, tx, truoc.ID, nguoi.Vet.ID, lyDo); err != nil {
			return err
		}
		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN: the column is the row's state, the entry
		// is the append-only record of the act.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi.Vet,
			Action:  uc.mo.hanhViXoa,
			Subject: truoc.Ma,
			Delta: deltaDanhMuc(map[string]any{
				"truoc":   vetDanhMuc(truoc),
				"ly_do":   lyDo,
				"xoa_mem": true,
			}),
		})
	})
	if err != nil {
		return bocDanhMuc(ctx, uc.mo.ten, "xoá", err)
	}
	return nil
}

// vetDanhMuc is the audit view of one row. `nguon` is in it because it decides what may later be done
// to the row, and an inspection has no other way to know which tier the row was in at the time.
func vetDanhMuc(m mucDanhMuc) map[string]any {
	return map[string]any{
		"ma": m.Ma, "nhan": m.Nhan, "thu_tu": m.ThuTu,
		"dang_dung": m.DangDung, "la_mac_dinh": m.LaMacDinh, "nguon": m.Nguon,
	}
}

// vetDoiDanhMuc returns only the fields that moved, from whichever side is asked for.
func vetDoiDanhMuc(truoc, sau mucDanhMuc, benTruoc bool) map[string]any {
	chon := func(a, b any) any {
		if benTruoc {
			return a
		}
		return b
	}
	ra := map[string]any{}
	if truoc.Nhan != sau.Nhan {
		ra["nhan"] = chon(truoc.Nhan, sau.Nhan)
	}
	if truoc.ThuTu != sau.ThuTu {
		ra["thu_tu"] = chon(truoc.ThuTu, sau.ThuTu)
	}
	if truoc.DangDung != sau.DangDung {
		ra["dang_dung"] = chon(truoc.DangDung, sau.DangDung)
	}
	if truoc.LaMacDinh != sau.LaMacDinh {
		ra["la_mac_dinh"] = chon(truoc.LaMacDinh, sau.LaMacDinh)
	}
	return ra
}

// deltaDanhMuc marshals the payload; a failure yields an explicit marker, never a nil delta.
func deltaDanhMuc(v map[string]any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"loi":"khong_dung_duoc_delta"}`)
	}
	return b
}

// bocDanhMuc wraps a failure with the table, the commune and the operation — nothing else (no code,
// no label, no actor). %w keeps errors.Is working for the handler.
func bocDanhMuc(ctx context.Context, ten, viec string, err error) error {
	return fmt.Errorf("%s: %s cho xã %s: %w", ten, viec, tenant.MustFrom(ctx), err)
}

// LaLoiDauVaoDanhMuc reports whether this is a refusal of what the client sent (400). Listed
// explicitly rather than defaulting to 400: a default would turn a database outage into a 400.
func LaLoiDauVaoDanhMuc(err error) bool {
	for _, e := range []error{
		domain.ErrMaDanhMucTrong, domain.ErrMaDanhMucSaiDinhDang, domain.ErrMaDanhMucQuaDai,
		domain.ErrNhanDanhMucTrong, domain.ErrNhanDanhMucQuaDai,
		domain.ErrThuTuDanhMucNgoaiKhoang,
		domain.ErrThieuLyDoXoaDanhMuc, domain.ErrLyDoXoaDanhMucQuaDai,
		domain.ErrMaBatBien, domain.ErrNguonDoTuClient,
	} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
