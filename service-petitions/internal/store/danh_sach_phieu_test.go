package store

import (
	"database/sql/driver"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/textfold"
)

// The REGISTER LIST — what actually reaches the WHERE clause.
//
// # WHY THIS FILE EXISTS, AND IT IS A MEASUREMENT RATHER THAN A HABIT
//
// It was written after a MUTATION went unnoticed. Replacing `if !loc.ChoPhepHanChe` with `if false`
// in locPhieuThanhSQL — which removes the restricted field `can-bo` from the exclusion, i.e. opens a
// report ABOUT a member of staff to every colleague of theirs — left the whole repository GREEN.
//
// The reason is worth writing down because it is the trap this repository keeps meeting: the handler
// suite in internal/http proves the DECISION (an account without `feedback.restricted` sends
// ChoPhepHanChe=false), and its fake store honours the flag. Nothing anywhere proved that the REAL
// store turns that flag into a predicate. Two halves each tested against the other's assumption is
// indistinguishable from a tested whole, right up until the day it is not.
//
//	PROVED HERE   the commune is $1 and comes from the CONTEXT · soft-deleted rows are excluded ·
//	              the restricted field is excluded when the flag is closed and NOT excluded when it
//	              is open · every filter becomes a BOUND PARAMETER and none is concatenated as text ·
//	              the "late" filter is DERIVED from the deadline columns and reads no flag column.
//
//	NOT PROVED    anything PostgreSQL does with the statement. The fake builds each row from the
//	              column list the store itself handed it, so a column that does not exist in the real
//	              table passes here without a murmur.

// chayDanhSach runs one page through the real store on the fake driver and returns the statement.
func chayDanhSach(t *testing.T, loc LocPhieu) lenhGia {
	t.Helper()
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	yc, err := page.Parse(url.Values{}, SapXepPhieu)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	if _, err := s.DanhSach(ctxXa(xaThu), loc, yc); err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	return k.lenh[0]
}

// dieuKienHanChe is the predicate that keeps a report ABOUT a member of staff away from that person's
// colleagues. Written out here, not imported, so that changing the spelling in the store without
// changing it here turns this red rather than silently passing.
const dieuKienHanChe = `linh_vuc <> 'can-bo'`

func TestDanhSachLocDongThiLoaiLinhVucHanChe(t *testing.T) {
	l := chayDanhSach(t, LocPhieu{})

	if !strings.Contains(l.sql, dieuKienHanChe) {
		t.Fatalf("câu lệnh KHÔNG loại lĩnh vực hạn chế: %q\n\n"+
			"Một cán bộ thường sẽ đọc được phiếu tố cáo tác phong của chính đồng nghiệp mình — và "+
			"phiếu ấy còn nằm trong mọi số đếm suy ra từ việc lật trang (docs/ui-ux/09 §5, §14.5).",
			l.sql)
	}
	// NULL IS NOT EXCLUDED. An unclassified petition has no field yet, and `linh_vuc <> 'can-bo'` is
	// NULL — not TRUE — for a NULL column, so without the IS NULL branch every petition waiting to be
	// classified would vanish from the register of every ordinary officer.
	if !strings.Contains(l.sql, "linh_vuc IS NULL OR") {
		t.Errorf("phiếu CHƯA phân loại bị loại khỏi danh sách: %q", l.sql)
	}
}

func TestDanhSachLocMoThiKhongLoaiLinhVucHanChe(t *testing.T) {
	l := chayDanhSach(t, LocPhieu{ChoPhepHanChe: true})

	if strings.Contains(l.sql, dieuKienHanChe) {
		t.Fatalf("vẫn loại lĩnh vực hạn chế dù người gọi có feedback.restricted — một quyền cấp ra "+
			"mà không mở gì là một ô tích không làm gì trong màn Phân quyền: %q", l.sql)
	}
}

func TestDanhSachBuocXaVaLoaiDongDaXoa(t *testing.T) {
	l := chayDanhSach(t, LocPhieu{})

	// RULE 1, INVARIANTS 4 AND 5. The commune is not a parameter of DanhSach and cannot become one:
	// it arrives in the context and Scoped.Query binds it to $1.
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if len(l.args) == 0 || l.args[0] != string(xaThu) {
		t.Errorf("tham số đầu = %v, muốn xã từ context", l.args)
	}
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows — everywhere, always.
	if !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", l.sql)
	}
}

// TestDanhSachMoiBoLocLaThamSoRangBuoc — a filter assembled as TEXT is an injection point in a
// government register. Every value below must appear in `args` and NOWHERE in the statement.
func TestDanhSachMoiBoLocLaThamSoRangBuoc(t *testing.T) {
	loc := LocPhieu{
		TrangThai: "dang-phan-loai",
		LinhVuc:   "rac-thai",
		ThonID:    "thon-001",
		BoPhanID:  "bp-001",
		Kenh:      "zalo-oa",
		Tim:       "đầu ngõ",
	}
	l := chayDanhSach(t, loc)

	for _, gt := range []string{loc.TrangThai, loc.LinhVuc, loc.ThonID, loc.BoPhanID, loc.Kenh} {
		if strings.Contains(l.sql, gt) {
			t.Errorf("giá trị %q nằm TRONG câu lệnh thay vì là tham số: %q", gt, l.sql)
		}
		if !coThamSoChuoi(l.args, gt) {
			t.Errorf("giá trị %q không có trong tham số: %v", gt, l.args)
		}
	}
	if !coThamSoChuoi(l.args, "%"+loc.Tim+"%") {
		t.Errorf("chuỗi tìm không được truyền làm tham số: %v", l.args)
	}
	if strings.Contains(l.sql, loc.Tim) {
		t.Errorf("chuỗi tìm nằm TRONG câu lệnh: %q", l.sql)
	}
}

// TestDanhSachLocTreHanLaSUYRA — rule 10, invariant 3. The predicate compares the stored deadline
// with the database's own clock and with the recorded finishing instant; it reads no flag column,
// because there is none and there must never be one.
func TestDanhSachLocTreHanLaSUYRA(t *testing.T) {
	l := chayDanhSach(t, LocPhieu{ChiTreHan: true})

	if !strings.Contains(l.sql, "han_xu_ly_xong") || !strings.Contains(l.sql, "now()") {
		t.Fatalf("bộ lọc trễ hạn không so hạn đã lưu với đồng hồ: %q", l.sql)
	}
	if !strings.Contains(l.sql, "xu_ly_xong_luc") {
		t.Error("bộ lọc trễ hạn không xét thời điểm xử lý xong — việc làm TRỄ rồi xong sẽ tự biến " +
			"mất khỏi số liệu, và báo cáo quý trước đổi mỗi lần có người mở màn hình")
	}
	for _, cam := range []string{"is_overdue", "qua_han", "tre_han"} {
		if strings.Contains(l.sql, cam) {
			t.Errorf("câu lệnh đọc cột cờ %q — quá hạn là SUY RA, không phải một cột", cam)
		}
	}
	// A petition with no resolve deadline is NOT late: it has had no date promised to anybody, so
	// there is nothing to have missed.
	if !strings.Contains(l.sql, "han_xu_ly_xong IS NOT NULL") {
		t.Error("phiếu chưa phân loại lọt vào bộ lọc trễ hạn")
	}
}

// TestDanhSachLocGiaoChoToiLaThamSoRangBuoc — the "Giao cho tôi" code is a BOUND parameter against
// `can_bo_xu_ly_id`, composes with the restricted-field exclusion and the commune, and is absent from
// the statement entirely when not asked for.
func TestDanhSachLocGiaoChoToiLaThamSoRangBuoc(t *testing.T) {
	const ma = "CB-00123"
	l := chayDanhSach(t, LocPhieu{CanBoXuLyID: ma})

	if !strings.Contains(l.sql, "can_bo_xu_ly_id = $") {
		t.Fatalf("thiếu vị từ người được giao: %q", l.sql)
	}
	if strings.Contains(l.sql, ma) {
		t.Errorf("mã cán bộ nằm TRONG câu lệnh thay vì là tham số: %q", l.sql)
	}
	if !coThamSoChuoi(l.args, ma) {
		t.Errorf("mã cán bộ không có trong tham số: %v", l.args)
	}
	if !strings.Contains(l.sql, dieuKienHanChe) {
		t.Errorf("lọc Giao cho tôi làm mất điều kiện loại lĩnh vực hạn chế: %q", l.sql)
	}
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") || !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("lọc Giao cho tôi làm mất ràng buộc xã hoặc xoá mềm: %q", l.sql)
	}

	// The PREDICATE, not the column name: `can_bo_xu_ly_id` is also in the SELECT list.
	if l := chayDanhSach(t, LocPhieu{}); strings.Contains(l.sql, "can_bo_xu_ly_id = $") {
		t.Errorf("không yêu cầu mà vẫn lọc theo người được giao: %q", l.sql)
	}
}

// placeholderOf is the `$n` a value is bound to. args[0] is the commune, $1.
func placeholderOf(t *testing.T, args []driver.Value, v string) string {
	t.Helper()
	for i, a := range args {
		if a == v {
			return "$" + strconv.Itoa(i+1)
		}
	}
	t.Fatalf("%q không có trong tham số: %v", v, args)
	return ""
}

// TestSearchIsAccentInsensitive — bug sheet 10/10 row 8. The text and the location are matched FOLDED
// ON BOTH SIDES: the stored folds of migration 0041 against the term folded by the SAME textfold.SQL.
// The lookup code stays ILIKE. A raw `noi_dung ILIKE` here is a search where "nha van hoa" finds nothing.
func TestSearchIsAccentInsensitive(t *testing.T) {
	l := chayDanhSach(t, LocPhieu{Tim: "nha van hoa"})
	p := placeholderOf(t, l.args, "%nha van hoa%")
	for _, want := range []string{
		"ma_tra_cuu ILIKE " + p,
		"content_folded LIKE " + textfold.SQL(p),
		"address_folded LIKE " + textfold.SQL(p),
	} {
		if !strings.Contains(l.sql, want) {
			t.Errorf("thiếu %q trong %q", want, l.sql)
		}
	}
	for _, raw := range []string{"noi_dung ILIKE", "dia_chi,'') ILIKE", "noi_dung LIKE"} {
		if strings.Contains(l.sql, raw) {
			t.Errorf("còn so thô %q — không gập dấu: %q", raw, l.sql)
		}
	}
}

// TestRelatedScopePredicate — `scope=related`, the prototype's INVOLVED clause read against this schema
// (citizenReportRelatedCondition): I hold it · I wrote on its timeline (commune bound INSIDE the
// subquery) · one of my units holds it. Every value bound; ANDed after the restricted-field exclusion.
func TestRelatedScopePredicate(t *testing.T) {
	const me = "CB-00123"
	l := chayDanhSach(t, LocPhieu{Related: &CitizenReportRelatedScope{StaffCode: me, OrgUnits: []string{"bp-1", "bp-2"}}})

	p := placeholderOf(t, l.args, me)
	u1, u2 := placeholderOf(t, l.args, "bp-1"), placeholderOf(t, l.args, "bp-2")
	want := " AND (can_bo_xu_ly_id = " + p +
		" OR id IN (SELECT nk.phieu_phan_anh_id FROM nhat_ky_phan_anh nk WHERE nk.tenant_id = $1 AND nk.nguoi_ma = " + p + ")" +
		" OR bo_phan_id IN (" + u1 + ", " + u2 + "))"
	if !strings.Contains(l.sql, want) {
		t.Fatalf("vị từ liên quan sai:\n%q\nmuốn chứa\n%q", l.sql, want)
	}
	for _, v := range []string{me, "bp-1", "bp-2"} {
		if strings.Contains(l.sql, v) {
			t.Errorf("%q nằm TRONG câu lệnh thay vì là tham số", v)
		}
	}
	if !strings.Contains(l.sql, dieuKienHanChe) || strings.Index(l.sql, dieuKienHanChe) > strings.Index(l.sql, want) {
		t.Errorf("phạm vi liên quan làm mất (hoặc đứng trước) điều kiện loại lĩnh vực hạn chế: %q", l.sql)
	}
}

// No unit: the unit clause is ABSENT — never "every unit".
func TestRelatedScopeWithoutUnitHasNoUnitClause(t *testing.T) {
	l := chayDanhSach(t, LocPhieu{Related: &CitizenReportRelatedScope{StaffCode: "CB-00123"}})
	if strings.Contains(l.sql, "bo_phan_id IN") || !strings.Contains(l.sql, "can_bo_xu_ly_id = $") {
		t.Errorf("không bộ phận mà vị từ = %q", l.sql)
	}
}

// No code: refused before any statement — the tab would otherwise show only the unit's petitions.
func TestRelatedScopeWithoutCodeIsRefused(t *testing.T) {
	k := &khoGia{}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))
	yc, _ := page.Parse(url.Values{}, SapXepPhieu)
	loc := LocPhieu{Related: &CitizenReportRelatedScope{OrgUnits: []string{"bp-1"}}}
	if _, err := s.DanhSach(ctxXa(xaThu), loc, yc); !errors.Is(err, ErrCitizenReportRelatedNoStaff) {
		t.Errorf("DanhSach: err = %v", err)
	}
	if _, err := s.CountCitizenReports(ctxXa(xaThu), loc); !errors.Is(err, ErrCitizenReportRelatedNoStaff) {
		t.Errorf("CountCitizenReports: err = %v", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

func coThamSoChuoi(args []driver.Value, muon string) bool {
	for _, a := range args {
		if s, ok := a.(string); ok && s == muon {
			return true
		}
	}
	return false
}
