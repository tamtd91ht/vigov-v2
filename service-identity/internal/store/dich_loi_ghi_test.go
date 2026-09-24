package store

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/migrations"
)

// THE TWO TRANSLATORS OF DATABASE REFUSALS, WITHOUT A POSTGRESQL.
//
// They are the second layer of the catalogue tier rules and of the org chart's unique code: the use
// case refuses first, and when a race slips past it the constraint or trigger refuses, and these turn
// that into the same 409. Two ways to get them wrong, both silent until a race happens in production:
//
//   - the matched phrase drifts from the migration's RAISE text → a system-row delete that lost the
//     race answers 500 instead of `system_row` (the row is still protected — the refusal is just
//     unreadable), and nobody can tell from tests, because the pg suite is skipped here;
//   - an UNRECOGNISED error gets flattened into a business refusal → a real outage reads as "code
//     already used", and the administrator retries a different code forever.
//
// Partition names are the realistic input: both tables are PARTITION BY HASH, and PostgreSQL reports
// the leaf partition's copy of the key.

func TestDichLoiGhiDanhMucNhanDangVaKhongNuotLoiLa(t *testing.T) {
	cases := []struct {
		tho  string
		muon error
	}{
		{`ERROR: duplicate key value violates unique constraint "khoi_nhiem_vu_p07_tenant_id_ma_key"`, ErrMaDaTonTai},
		{`ERROR: catalogue loai_don_vi_dan_cu_p03: a system row is disabled, never deleted`, domain.ErrKhongXoaDuocMucHeThong},
		{`ERROR: catalogue khoi_nhiem_vu_p11: a tier-3 row cannot be taken out of use`, domain.ErrKhongTatDuocMucReNhanh},
	}
	for _, c := range cases {
		if got := dichLoiGhiDanhMuc("khoi_nhiem_vu", "ghi", errors.New(c.tho)); !errors.Is(got, c.muon) {
			t.Errorf("%q → %v, muốn %v", c.tho, got, c.muon)
		}
	}
	goc := errors.New("ERROR: connection reset by peer")
	got := dichLoiGhiDanhMuc("khoi_nhiem_vu", "ghi", goc)
	if !errors.Is(got, goc) {
		t.Errorf("lỗi lạ phải được bọc nguyên, nhận %v", got)
	}
	for _, sai := range []error{ErrMaDaTonTai, domain.ErrKhongXoaDuocMucHeThong, domain.ErrKhongTatDuocMucReNhanh, ErrDanhMucKhongTonTai} {
		if errors.Is(got, sai) {
			t.Errorf("lỗi hạ tầng bị dịch thành lỗi nghiệp vụ %v", sai)
		}
	}
}

func TestDichLoiGhiBoPhanNhanDangVaKhongNuotLoiLa(t *testing.T) {
	if got := dichLoiGhiBoPhan("chèn", errors.New(`duplicate key value violates unique constraint "bo_phan_p07_tenant_id_ma_key"`)); !errors.Is(got, ErrMaBoPhanDaDung) {
		t.Errorf("khoá duy nhất → %v, muốn ErrMaBoPhanDaDung", got)
	}
	if got := dichLoiGhiBoPhan("chèn", errors.New(`insert or update on table "bo_phan_p02" violates foreign key constraint "bo_phan_tenant_id_cha_id_fkey"`)); !errors.Is(got, ErrBoPhanChaKhongTonTai) {
		t.Errorf("khoá ngoại cha → %v, muốn ErrBoPhanChaKhongTonTai", got)
	}
	goc := errors.New("ERROR: canceling statement due to statement timeout")
	got := dichLoiGhiBoPhan("chèn", goc)
	if !errors.Is(got, goc) || errors.Is(got, ErrMaBoPhanDaDung) || errors.Is(got, ErrBoPhanChaKhongTonTai) {
		t.Errorf("lỗi lạ phải được bọc nguyên, không thành lỗi nghiệp vụ: %v", got)
	}
}

// THE PHRASES dichLoiGhiDanhMuc MATCHES ARE THE MIGRATION'S OWN RAISE TEXTS. Read from the embedded
// migration, so a reworded RAISE turns this red instead of silently turning a 409 into a 500.
func TestDichLoiGhiDanhMucKhopCauRaiseCuaMigration(t *testing.T) {
	b, err := fs.ReadFile(migrations.FS, "0005_don_vi_dan_cu_va_danh_muc.sql")
	if err != nil {
		t.Fatalf("đọc migration 0005: %v", err)
	}
	sql := string(b)
	for _, cau := range []string{
		"a system row is disabled, never deleted",
		"a tier-3 row cannot be taken out of use",
	} {
		if !strings.Contains(sql, "RAISE EXCEPTION 'catalogue %: "+cau+"'") {
			t.Errorf("migration 0005 không còn RAISE %q — dichLoiGhiDanhMuc sẽ trả 500 thay vì 409", cau)
		}
	}
}
