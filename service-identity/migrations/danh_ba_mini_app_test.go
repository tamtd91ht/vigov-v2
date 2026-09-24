package migrations

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Schema-TEXT checks for migration 0010, plus one cross-file check on the `quyen` seed.
//
// WHAT THIS IS AND IS NOT. It reads the SQL this binary embeds and asserts the constraints are
// written there. It does NOT prove PostgreSQL enforces them — that is
// internal/store/danh_ba_mini_app_pg_test.go, which needs VIGOV_TEST_DSN and SKIPS without it. This
// file exists because a pg suite that skips on most machines turns green whatever the SQL says: a
// deleted CHECK must go red somewhere that always runs, and this is that somewhere. It is the
// weaker half, and it is written down as the weaker half.

const tep0010 = "0010_danh_ba_mini_app_va_khoa_xoa_dong_trung.sql"

// maChay returns the executable part of a migration: `--` comments removed, whitespace collapsed,
// lower-cased. Comments are removed FIRST because 0010's REVERSAL section writes the constraint
// names out in prose — a test matching comments would stay green with the real DDL gone.
func maChay(t *testing.T, ten string) string {
	t.Helper()
	b, err := fs.ReadFile(FS, ten)
	if err != nil {
		t.Fatalf("đọc %s từ FS nhúng: %v", ten, err)
	}
	var dong []string
	for _, d := range strings.Split(string(b), "\n") {
		if i := strings.Index(d, "--"); i >= 0 {
			d = d[:i]
		}
		dong = append(dong, d)
	}
	return strings.ToLower(strings.Join(strings.Fields(strings.Join(dong, " ")), " "))
}

// THE MUTATION THAT MUST TURN THIS RED: delete (or weaken) any of the three ADD CONSTRAINT blocks
// in 0010 §3. The published⇒consent CHECK is the one open question #12 rests on; the
// unpublished⇒no-marks CHECK is "unpublishing clears the consent, the next publish asks again".
func TestMigration0010RangBuocCongKhaiCoDongY(t *testing.T) {
	sql := maChay(t, tep0010)

	can := []struct{ ten, bieuThuc string }{
		{"nguoi_dung_cong_khai_phai_co_dong_y",
			"check (not hien_tren_mini_app or (dong_y_cong_khai_luc is not null and dong_y_cong_khai_ghi_boi <> ''))"},
		{"nguoi_dung_rut_cong_khai_xoa_dong_y",
			"check (hien_tren_mini_app or (dong_y_cong_khai_luc is null and dong_y_cong_khai_ghi_boi = ''))"},
		{"nguoi_dung_thu_tu_danh_ba_khong_am",
			"check (thu_tu_danh_ba is null or thu_tu_danh_ba >= 0)"},
	}
	for _, c := range can {
		if !strings.Contains(sql, "add constraint "+c.ten+" "+c.bieuThuc) {
			t.Errorf("0010 KHÔNG CÒN ràng buộc %s với biểu thức %q — câu #12 chỉ còn được canh ở tầng "+
				"ứng dụng, tức một dòng psql là đủ đưa số di động lên kênh công khai không cần đồng ý",
				c.ten, c.bieuThuc)
		}
	}
}

// A CHECK passes when its expression is NULL. If the recorder column were nullable,
// "dong_y_cong_khai_ghi_boi is not the empty string" would be NULL for a NULL recorder and
// PostgreSQL would ACCEPT a published row naming nobody. NOT NULL with an empty-string default is
// what makes the CHECK above mean what it
// says; "tidying" it to a plain nullable TEXT must go red here.
func TestMigration0010NguoiGhiDongYKhongDuocNull(t *testing.T) {
	sql := maChay(t, tep0010)
	if !strings.Contains(sql, "add column if not exists dong_y_cong_khai_ghi_boi text not null default ''") {
		t.Error("dong_y_cong_khai_ghi_boi không còn NOT NULL DEFAULT '' — CHECK công khai⇒đồng ý " +
			"sẽ ĐỂ LỌT dòng có người ghi NULL, vì CHECK coi NULL là đạt")
	}
	// Fail closed: nothing is published by default (Decree 13/2023, #12).
	if !strings.Contains(sql, "add column if not exists hien_tren_mini_app boolean not null default false") {
		t.Error("hien_tren_mini_app không còn DEFAULT false — ngày tuyến Mini App đọc cột này, " +
			"mọi cán bộ bị công khai mà không ai được hỏi")
	}
}

// `admin.user.delete` is seeded (ADR 0035 / #27) — TASK-04's route checks it, and a key no
// migration seeds is a route that answers 403 to every account forever (rule 5, invariant 3c).
func TestMigration0010NapKhoaAdminUserDelete(t *testing.T) {
	sql := maChay(t, tep0010)
	if !strings.Contains(sql, "insert into quyen (ma, nhom, nhan, thu_tu) values ('admin.user.delete', 'quản trị',") {
		t.Error("0010 không còn nạp khoá admin.user.delete vào nhóm QUẢN TRỊ")
	}
}

// `quyen.thu_tu` has no unique constraint, so a colliding number raises nothing: the Phân quyền
// screen's order then depends on the order PostgreSQL returns rows (0007's header). This scans
// EVERY migration's quyen seed, so the next file that picks a taken number goes red here.
func TestQuyenThuTuKhongTrungGiuaCacMigration(t *testing.T) {
	dongQuyen := regexp.MustCompile(`\(\s*'([a-z][a-z0-9_.]*)'\s*,\s*'[^']*'\s*,\s*'[^']*'\s*,\s*(\d+)\s*\)`)
	chenQuyen := regexp.MustCompile(`(?is)insert\s+into\s+quyen\b(.*?)on\s+conflict`)

	tep, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(tep)

	theoSo := map[string]string{}
	dem := 0
	for _, ten := range tep {
		sql := maChay(t, ten)
		for _, khoi := range chenQuyen.FindAllStringSubmatch(sql, -1) {
			for _, m := range dongQuyen.FindAllStringSubmatch(khoi[1], -1) {
				dem++
				if cu, co := theoSo[m[2]]; co && cu != m[1] {
					t.Errorf("thu_tu %s dùng cho cả %q và %q (%s)", m[2], cu, m[1], ten)
				}
				theoSo[m[2]] = m[1]
			}
		}
	}
	// Fail closed: a regex that stopped matching would make this test vacuously green.
	if dem < 36 {
		t.Fatalf("chỉ đọc được %d dòng nạp quyen, mong ít nhất 36 — bộ phân tích đã mù", dem)
	}
}
