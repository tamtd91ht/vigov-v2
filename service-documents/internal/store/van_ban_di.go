package store

// The SỔ VĂN BẢN ĐI register — SQL, and nothing else.
//
// THE FIVE PROPERTIES STATED AT THE TOP OF van_ban_den.go HOLD HERE TOO, word for word: the commune
// is $1 everywhere, nothing here opens a transaction, every read excludes soft-deleted rows, the
// issued number appears in no UPDATE, and there is no hard delete. They are not repeated; what
// follows is only what is DIFFERENT about the outgoing register.
//
// AND THE DIFFERENCE IS THE WEIGHT OF THE NUMBER. An incoming number is the commune's own
// bookkeeping. An OUTGOING number is printed on the document, sealed, and sent to a district office,
// a court or a citizen — so a duplicate is not a data problem inside this system, it is two
// documents nobody outside the commune can tell apart, one of which somebody has signed. The
// mechanisms are identical (day_so_van_ban.go); the consequence of a hole in them is not.
//
// ⚠ NO SPECIFICATION EXISTS FOR THIS REGISTER. docs/ui-ux/05-van-ban-don-thu.md describes the
// incoming register and the citizen-letter register and says nothing at all about outgoing
// documents. The columns below are this session's reading, reported as such — see migration 0004.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// VanBanDiStore reads and writes one commune's outgoing-document register.
type VanBanDiStore struct {
	db *store.DB
}

func NewVanBanDiStore(db *store.DB) *VanBanDiStore { return &VanBanDiStore{db: db} }

var (
	// ErrKhongThayVanBanDi — no such live entry in this commune. Same single answer for "does not
	// exist" and "belongs to another commune", for the same reason as the incoming register.
	ErrKhongThayVanBanDi = errors.New("van_ban_di: không có văn bản này trong xã")

	// ErrLoaiVanBanDiKhongDung — the type code names no live, in-use row of this commune's
	// catalogue. A SEPARATE SENTINEL from the incoming register's, so the handler's message names
	// the register the clerk is actually looking at.
	ErrLoaiVanBanDiKhongDung = errors.New("van_ban_di: loại văn bản không có trong danh mục của xã")
)

// SapXepVanBanDi — `number` descending by default, the newest issued document first. Same closed set
// and same exclusions as the incoming register: `noi_nhan` and `trich_yeu` are free text about a
// government matter and a sort key travels in a URL and an access log.
var SapXepVanBanDi = page.NewAllowlist(page.Desc,
	page.Col("number", "so_di", page.KindInt),
	page.Col("created_at", "tao_luc", page.KindTime),
)

// LocVanBanDi is the set of filters the list route accepts, already validated by the handler. Every
// field becomes a BOUND PARAMETER — see locVanBanDiThanhSQL.
type LocVanBanDi struct {
	Nam        int
	LoaiVanBan string
	Tim        string
}

// cotVanBanDi IS READ BY POSITION in the scans below.
//
// THE TRAP HERE IS `trich_yeu` / `noi_nhan` / `nguoi_ky` — three adjacent TEXT columns. Swapped, the
// register shows the recipient where the summary belongs and, worse, the signer's name in the
// recipient column: an outgoing document that reads as having been sent to the person who signed it.
// Nothing errors, and the row looks plausible.
const cotVanBanDi = `id, so_di, nam, ngay_van_ban, loai_van_ban, trich_yeu, noi_nhan, ` +
	`COALESCE(nguoi_ky, ''), nguoi_tao_ma, tao_luc, cap_nhat_luc`

func docMotDongVanBanDi(quet func(...any) error) (domain.VanBanDi, error) {
	var v domain.VanBanDi
	err := quet(&v.ID, &v.SoDi, &v.Nam, &v.NgayVanBan, &v.LoaiVanBan, &v.TrichYeu,
		&v.NoiNhan, &v.NguoiKy, &v.NguoiTaoMa, &v.TaoLuc, &v.CapNhatLuc)
	if err != nil {
		return domain.VanBanDi{}, err
	}
	return v, nil
}

// DanhSach reads ONE PAGE of the commune's outgoing register.
func (s *VanBanDiStore) DanhSach(ctx context.Context, loc LocVanBanDi,
	yc page.Request) (page.Result[domain.VanBanDi], error) {

	dieuKien, args := locVanBanDiThanhSQL(loc)

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotVanBanDi,
		Table:   "van_ban_di",
		Filter:  `AND deleted_at IS NULL` + dieuKien,
		Args:    args,
	}, yc, mocVanBanDi, func(rows *sql.Rows) (domain.VanBanDi, string, error) {
		v, err := docMotDongVanBanDi(rows.Scan)
		if err != nil {
			return domain.VanBanDi{}, "", err
		}
		return v, v.ID, nil
	})
}

func locVanBanDiThanhSQL(loc LocVanBanDi) (string, []any) {
	var (
		dieuKien string
		args     []any
	)
	them := func(mau string, gt any) {
		args = append(args, gt)
		dieuKien += fmt.Sprintf(mau, len(args)+1) // $1 is the commune
	}
	if loc.Nam != 0 {
		them(" AND nam = $%d", loc.Nam)
	}
	if loc.LoaiVanBan != "" {
		them(" AND loai_van_ban = $%d", loc.LoaiVanBan)
	}
	if loc.Tim != "" {
		args = append(args, "%"+loc.Tim+"%")
		dieuKien += fmt.Sprintf(" AND (trich_yeu ILIKE $%d OR noi_nhan ILIKE $%d)",
			len(args)+1, len(args)+1)
	}
	return dieuKien, args
}

var mocVanBanDi = store.NewMoc[domain.VanBanDi](SapXepVanBanDi,
	map[string]func(domain.VanBanDi) page.Key{
		"number":     func(v domain.VanBanDi) page.Key { return page.IntKey(int64(v.SoDi)) },
		"created_at": func(v domain.VanBanDi) page.Key { return page.TimeKey(v.TaoLuc) },
	})

// --- the write path ---------------------------------------------------------------------------

// TheoIDDeSua reads one live entry inside the transaction and holds it until the transaction ends.
// `FOR UPDATE` for the same reason as on the incoming register: every write is a read-decide-write.
func (s *VanBanDiStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.VanBanDi, error) {

	const stmt = `SELECT ` + cotVanBanDi + ` FROM van_ban_di ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	v, err := docMotDongVanBanDi(
		tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.VanBanDi{}, ErrKhongThayVanBanDi
	}
	if err != nil {
		return domain.VanBanDi{}, fmt.Errorf("van_ban_di: đọc văn bản để sửa: %w", err)
	}
	return v, nil
}

// LoaiVanBanConDung is the same check the incoming register makes, returning ITS OWN sentinel.
func (s *VanBanDiStore) LoaiVanBanConDung(ctx context.Context, tx *store.ScopedTx, ma string) error {
	const stmt = `SELECT 1 FROM loai_van_ban
		WHERE tenant_id = $1 AND ma = $2 AND dang_dung AND deleted_at IS NULL`

	var mot int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma).Scan(&mot)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLoaiVanBanDiKhongDung
	}
	if err != nil {
		return fmt.Errorf("van_ban_di: kiểm loại văn bản: %w", err)
	}
	return nil
}

// chenVanBanDi — `so_di` is a parameter and comes from DaySoStore.CapSo in this same transaction,
// under the counter's row lock. There is no other caller and no other source.
const chenVanBanDi = `INSERT INTO van_ban_di
	(tenant_id, id, so_di, nam, ngay_van_ban, loai_van_ban, trich_yeu, noi_nhan,
	 nguoi_ky, nguoi_tao_ma)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

func (s *VanBanDiStore) Chen(ctx context.Context, tx *store.ScopedTx, v domain.VanBanDi) error {
	_, err := tx.Exec(ctx, chenVanBanDi, string(tx.TenantID()),
		v.ID, v.SoDi, v.Nam, v.NgayVanBan, v.LoaiVanBan, v.TrichYeu, v.NoiNhan,
		rongThanhNil(v.NguoiKy), v.NguoiTaoMa)
	if err != nil {
		return fmt.Errorf("van_ban_di: chèn: %w", err)
	}
	return nil
}

// capNhatVanBanDi — `so_di` and `nam` APPEAR NOWHERE IN THIS STATEMENT. An issued outgoing number is
// on paper outside this commune; the trigger refuses to change it underneath, and its absence here
// is what makes that floor unreachable from this service.
const capNhatVanBanDi = `UPDATE van_ban_di
	SET ngay_van_ban = $3, loai_van_ban = $4, trich_yeu = $5, noi_nhan = $6, nguoi_ky = $7,
	    cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *VanBanDiStore) CapNhat(ctx context.Context, tx *store.ScopedTx, v domain.VanBanDi) error {
	kq, err := tx.Exec(ctx, capNhatVanBanDi, string(tx.TenantID()),
		v.ID, v.NgayVanBan, v.LoaiVanBan, v.TrichYeu, v.NoiNhan, rongThanhNil(v.NguoiKy))
	if err != nil {
		return fmt.Errorf("van_ban_di: cập nhật: %w", err)
	}
	return doiMotDongVanBanDi(kq, "cập nhật")
}

// xoaMemVanBanDi writes all THREE columns rule 7, invariant 1 names, in one statement.
//
// THE NUMBER IS NOT RETURNED TO THE SERIES — and on this register that sentence is the whole point.
// A document was issued under that number and has left the building; removing the row from the
// commune's screens cannot unsend it.
const xoaMemVanBanDi = `UPDATE van_ban_di
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *VanBanDiStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemVanBanDi, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("van_ban_di: xoá mềm: %w", err)
	}
	return doiMotDongVanBanDi(kq, "xoá mềm")
}

// doiMotDongVanBanDi is a SECOND COPY of doiMotDongVanBan on purpose, and the difference is the
// error it returns: that one answers ErrKhongThayVanBanDen, which the handler maps to "Không tìm
// thấy văn bản đến này." Sharing it would put a sentence about the incoming register in front of a
// clerk who was working in the outgoing one — and those are two different books on two different
// screens.
func doiMotDongVanBanDi(kq sql.Result, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("van_ban_di: %s: đọc số dòng: %w", viec, err)
	}
	if n == 0 {
		return ErrKhongThayVanBanDi
	}
	return nil
}
