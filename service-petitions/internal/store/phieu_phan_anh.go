package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// PhieuPhanAnhStore is the only path to `phieu_phan_anh` (migration 0004).
//
// EVERY READ AND EVERY WRITE GOES THROUGH *store.Scoped, which binds `tenant_id` to $1 from the
// context. The commune is therefore not a parameter of any method here and cannot be made into
// one (rule 1, invariant 4 and 5) — a repository that can be built without a commune is a
// repository that can query across communes.
//
// THE WRITE METHODS TAKE *store.ScopedTx AND NOT A CONTEXT, deliberately. A petition is business
// data, so every write to it must leave an audit entry IN THE SAME TRANSACTION (rule 6,
// invariant 3). Taking the transaction as an argument makes "write the record now, write the
// trail afterwards if it works" impossible to express — which is the defect measured across the
// whole of the previous system.
type PhieuPhanAnhStore struct {
	db *store.DB
}

func NewPhieuPhanAnhStore(db *store.DB) *PhieuPhanAnhStore {
	return &PhieuPhanAnhStore{db: db}
}

// ErrPhieuKhongTonTai means no LIVE petition of THIS commune carries that lookup code.
//
// ONE ERROR FOR "no such code", "another commune's code" AND "soft deleted", and the caller
// answers 404 for all three. Telling them apart would confirm to somebody trying codes that a
// petition exists somewhere — and a petition's existence is itself information about a citizen
// (rule 4, forbidden #2 draws the same line between citizens).
var ErrPhieuKhongTonTai = errors.New("phieu_phan_anh: không có phiếu")

// cotPhieu IS READ BY POSITION in the Scan below. The dangerous neighbours are marked where they
// sit: `han_tiep_nhan` and `han_xu_ly_xong` are adjacent TIMESTAMPTZ columns whose NULLs mean
// OPPOSITE things, and swapping them produces no error at all — it produces a staff-booked
// petition that looks unclassified and a report that silently excludes the wrong rows.
const cotPhieu = `id, ma_tra_cuu, kenh_tiep_nhan, cong_dan_id, noi_dung, linh_vuc,
	dia_chi, thon_id, lat, lng, nguoi_gui_ho_ten, nguoi_gui_dien_thoai, an_danh,
	trang_thai, bo_phan_id, can_bo_xu_ly_id,
	goc_dem_han, vao_so_luc, han_tiep_nhan, han_xu_ly_xong,
	phan_loai_luc, xu_ly_xong_luc, dong_luc, hien_cong_khai, so_lan_mo_lai`

// TheoMaTraCuu reads one petition by the code the citizen was handed.
//
// `deleted_at IS NULL` IS ON THIS PATH AS IT IS ON EVERY OTHER (rule 7, invariant 2): lists,
// statistics, search, reports and background jobs alike. A soft-deleted petition is gone from
// every read path, and the row stays because it is an archival record.
func (s *PhieuPhanAnhStore) TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotPhieu, "phieu_phan_anh",
		`AND ma_tra_cuu = $2 AND deleted_at IS NULL`, ma)
	if err != nil {
		// The code is NOT in the wrapped message. It is the one string that opens a citizen's
		// petition, and an error travels into centralised logging (rule 3).
		return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: đọc theo mã tra cứu: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: đọc theo mã tra cứu: %w", err)
		}
		return domain.PhieuPhanAnh{}, ErrPhieuKhongTonTai
	}
	p, err := quetPhieu(rows)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: duyệt kết quả: %w", err)
	}
	return p, nil
}

// quangKiem is the row interface both Scan paths satisfy. It exists so quetPhieu can be shared
// without the store handing out *sql.Rows.
type quangKiem interface {
	Scan(dest ...any) error
}

func quetPhieu(r quangKiem) (domain.PhieuPhanAnh, error) {
	var (
		p    domain.PhieuPhanAnh
		kenh string
		tt   string

		// EVERY NULLABLE COLUMN IS READ THROUGH AN EXPLICIT NULL TYPE. Scanning a NULL straight
		// into a string or a time.Time is a runtime error in some drivers and a zero value in
		// others, and the second of those is how `han_tiep_nhan IS NULL` quietly becomes a
		// deadline in year 1.
		congDan, linhVuc, diaChi, thon, hoTen, dienThoai, boPhan, canBo sql.NullString
		lat, lng                                                        sql.NullFloat64
		hanTiepNhan, hanXuLyXong                                        sql.NullTime
		phanLoaiLuc, xuLyXongLuc, dongLuc                               sql.NullTime
	)

	// POSITIONAL — in lockstep with cotPhieu. See the note there.
	if err := r.Scan(
		&p.ID, &p.MaTraCuu, &kenh, &congDan, &p.NoiDung, &linhVuc,
		&diaChi, &thon, &lat, &lng, &hoTen, &dienThoai, &p.AnDanh,
		&tt, &boPhan, &canBo,
		&p.GocDemHan, &p.VaoSoLuc, &hanTiepNhan, &hanXuLyXong,
		&phanLoaiLuc, &xuLyXongLuc, &dongLuc, &p.HienCongKhai, &p.SoLanMoLai,
	); err != nil {
		return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: đọc dòng: %w", err)
	}

	p.Kenh = domain.KenhTiepNhan(kenh)
	p.TrangThai = domain.TrangThai(tt)
	p.CongDanID = congDan.String
	p.LinhVuc = linhVuc.String
	p.DiaChi = diaChi.String
	p.ThonID = thon.String
	p.NguoiGuiHoTen = hoTen.String
	p.NguoiGuiDienThoai = dienThoai.String
	p.BoPhanID = boPhan.String
	p.CanBoXuLyID = canBo.String
	if lat.Valid {
		v := lat.Float64
		p.Lat = &v
	}
	if lng.Valid {
		v := lng.Float64
		p.Lng = &v
	}
	// NULL -> the zero time.Time, and the MEANING of that zero differs per column. domain's
	// HanTiepNhanKhongApDung / ChuaChotHanXuLy are the only sanctioned way to ask.
	p.HanTiepNhan = hanTiepNhan.Time
	p.HanXuLyXong = hanXuLyXong.Time
	p.PhanLoaiLuc = phanLoaiLuc.Time
	p.XuLyXongLuc = xuLyXongLuc.Time
	p.DongLuc = dongLuc.Time
	return p, nil
}

// Tao inserts one petition INSIDE the caller's transaction.
//
// THE CALLER WRITES THE AUDIT ENTRY IN THAT SAME TRANSACTION. This method deliberately does not
// write it: the audit entry needs the actor and the IP, which are facts about the REQUEST and
// not about the record, and a store that invented them would produce a trail naming nobody.
//
// A ZERO time.Time BECOMES SQL NULL, and that translation is where the two opposite NULLs are
// actually produced. It is done once, here, so no caller assembles its own arguments and gets
// the direction wrong.
func (s *PhieuPhanAnhStore) Tao(ctx context.Context, tx *store.ScopedTx, p domain.PhieuPhanAnh) error {
	const stmt = `INSERT INTO phieu_phan_anh (
		tenant_id, id, ma_tra_cuu, kenh_tiep_nhan, cong_dan_id, noi_dung, linh_vuc,
		dia_chi, thon_id, lat, lng, nguoi_gui_ho_ten, nguoi_gui_dien_thoai, an_danh,
		trang_thai, goc_dem_han, vao_so_luc, han_tiep_nhan, han_xu_ly_xong, hien_cong_khai)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`

	_, err := tx.Exec(ctx, stmt,
		string(tx.TenantID()), p.ID, p.MaTraCuu, string(p.Kenh), rongThanhNull(p.CongDanID),
		p.NoiDung, rongThanhNull(p.LinhVuc),
		rongThanhNull(p.DiaChi), rongThanhNull(p.ThonID), p.Lat, p.Lng,
		rongThanhNull(p.NguoiGuiHoTen), rongThanhNull(p.NguoiGuiDienThoai), p.AnDanh,
		string(p.TrangThai), p.GocDemHan, p.VaoSoLuc,
		khongThanhNull(p.HanTiepNhan), khongThanhNull(p.HanXuLyXong),
		p.HienCongKhai)
	if err != nil {
		// NOT the content, NOT the reporter, NOT the lookup code — an INSERT error message can
		// carry the whole row on some drivers, and this row is citizen personal data (rule 3).
		return fmt.Errorf("phieu_phan_anh: ghi phiếu: %w", err)
	}
	return nil
}

// ChotLinhVuc records the classification: the field, the new status, the instant a human first
// touched the petition, and the resolve deadline that act fixes.
//
// ALL FOUR IN ONE STATEMENT, and that is the design rather than a convenience. They are one
// business act (ADR 0028, decision E): the act that stops the acknowledge clock IS the act that
// settles the field IS the act that fixes the resolve deadline. Split across statements, a
// failure between them leaves a petition classified with no resolve commitment — and nothing
// would ever come back to fill it, because the step that would have is the one that just ran.
//
// `han_xu_ly_xong` IS PASSED IN, ALREADY DECIDED. This method does not choose between "fix it"
// and "shorten it": that is ADR 0027 decision C versus ADR 0028 decision E, it depends on
// whether this is the first settling, and it belongs in the use case where the before/after
// values are also written to the audit trail.
//
// THE WHERE CLAUSE CARRIES THE EXPECTED STATUS, so two officers classifying the same petition at
// the same moment cannot both succeed. The second one matches no row, and the caller sees zero
// rows affected — a refusal, not a silent overwrite of the first one's field and deadline.
func (s *PhieuPhanAnhStore) ChotLinhVuc(ctx context.Context, tx *store.ScopedTx,
	id, linhVuc string, tuTrangThai domain.TrangThai, sangTrangThai domain.TrangThai,
	phanLoaiLuc, hanXuLyXong time.Time) error {

	const stmt = `UPDATE phieu_phan_anh
		SET linh_vuc = $3, trang_thai = $4, phan_loai_luc = $5, han_xu_ly_xong = $6,
		    cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND trang_thai = $7 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt,
		string(tx.TenantID()), id, linhVuc, string(sangTrangThai), phanLoaiLuc,
		khongThanhNull(hanXuLyXong), string(tuTrangThai))
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: chốt lĩnh vực: %w", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: chốt lĩnh vực, đếm dòng: %w", err)
	}
	if n == 0 {
		// The petition moved, was soft deleted, or never belonged to this commune. All three are
		// the same answer for the same reason as ErrPhieuKhongTonTai.
		return ErrPhieuKhongTonTai
	}
	return nil
}

// rongThanhNull turns an empty Go string into SQL NULL.
//
// WHY NOT STORE THE EMPTY STRING: `linh_vuc = ”` would satisfy the
// `phieu_phan_anh_nhap_ho_co_linh_vuc` constraint while naming no field at all, and a report
// grouping by field would grow a nameless bucket nobody can act on. NULL is the value that means
// "not known yet", and the constraint is written against it.
func rongThanhNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// khongThanhNull turns the zero time.Time into SQL NULL.
//
// THIS IS THE ONE FUNCTION THAT PRODUCES BOTH OF THE OPPOSITE NULLS, and it is deliberately
// blind to which is which: the MEANING is decided by the caller and by `kenh_tiep_nhan`, and a
// helper that tried to decide would need to know the channel — at which point it would be the
// business rule, in the wrong layer.
func khongThanhNull(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
