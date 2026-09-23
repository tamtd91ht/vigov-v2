package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// manHinhGia builds a client layer out of literal TypeScript, so every case below states the
// exact source it is claiming something about.
func manHinhGia(tep map[string]string) manHinh {
	m := manHinh{nguon: map[string]string{}}
	for ten, s := range tep {
		m.nguon[ten] = boChuThichTS(s)
	}
	return m
}

// tuyenNV is the route the cases below argue about: it exists, it is staff-facing, and its
// operation id is petitions_get_tasks_by_ma.
var tuyenNV = func() tuyen {
	t := tuyenGia("GET", "/api/v1/tasks/{ma}")
	t.Service = "petitions"
	return t
}()

// --- BA CA BẮT BUỘC: có mã gọi · chỉ nằm trong chú thích · không ai nhắc ---------------------

// A real call, written the way this repository's client layer writes one.
func TestCoMaGoiThiCoiLaXong(t *testing.T) {
	mh := manHinhGia(map[string]string{
		"nhiem-vu.ts": `
import { docJSON } from "./goi";
export function layNhiemVu(ma: string) {
  const mau: petitions_get_tasks_by_ma["duongDan"] = "/api/v1/tasks/{ma}";
  return docJSON(mau.replace("{ma}", ma));
}
`,
	})
	if !mh.daDung(tuyenNV) {
		t.Fatal("tuyến có lời gọi thật mà không được coi là đã có màn hình")
	}
}

// THE CASE THIS FILE EXISTS FOR.
//
// The client layer here documents itself richly: nhiem-vu.ts really does open with a table of
// all eight task routes, paths, operation ids and permissions included. Counting that prose as
// eight finished screens is the same defect check_khoa_duy_nhat.py had just been fixed for —
// 17 of its 58 UNIQUE matches were words inside comments, and the threshold built on the
// inflated number sat above the real one. Both comment syntaxes are here because both are used.
func TestChiNamTrongChuThichThiKhongPhaiXong(t *testing.T) {
	mh := manHinhGia(map[string]string{
		"nhiem-vu.ts": `
/**
 * Tám tuyến của sổ Nhiệm vụ:
 *   GET /api/v1/tasks/{ma}   task.read   -> petitions_get_tasks_by_ma["duongDan"]
 */
// Sẽ làm sau: "/api/v1/tasks/{ma}" qua petitions_get_tasks_by_ma, gọi bằng "POST".
import { docJSON } from "./goi";
export function layKhac() {
  const mau: petitions_get_khac["duongDan"] = "/api/v1/khac";
  return docJSON(mau);
}
`,
	})
	if mh.daDung(tuyenNV) {
		t.Fatal("một đoạn văn NÓI VỀ tuyến bị đếm thành một lời gọi tuyến")
	}
}

func TestKhongAiNhacThiConMo(t *testing.T) {
	mh := manHinhGia(map[string]string{
		"thu-chi.ts": `
import { docJSON } from "./goi";
export function laySoThu() {
  const duongDan: finance_get_budget_sheets["duongDan"] = "/api/v1/budget-sheets";
  return docJSON(duongDan);
}
`,
	})
	if mh.daDung(tuyenNV) {
		t.Fatal("tuyến không ai gọi lại được coi là đã có màn hình")
	}
}

// --- hai dấu hiệu, và vì sao cần cả hai -----------------------------------------------------

// POST /api/v1/announcements is annotated `comms_get_announcements["duongDan"]` in the real
// client (thong-bao.ts:139) — the two routes share a path, so the write borrows the read's
// generated type. The operation-id signal alone leaves it open forever.
func TestChuoiDuongDanVaPhuongThucDuTrongKhiMaThaoTacVangMat(t *testing.T) {
	tb := tuyenGia("POST", "/api/v1/announcements")
	tb.Service = "comms"

	mh := manHinhGia(map[string]string{
		"thong-bao.ts": `
export function taoThongBao(than: unknown) {
  const duongDan: comms_get_announcements["duongDan"] = "/api/v1/announcements";
  return goiGhi(duongDan, "POST", than, 201);
}
`,
	})
	if !mh.daDung(tb) {
		t.Fatal("lời gọi ghi thật bị bỏ sót chỉ vì nó mượn kiểu của tuyến đọc cùng đường dẫn")
	}
}

// The other half of the same trade-off: a path literal with NO sign of the method is a read
// screen, and a read screen is not a write screen. POST /api/v1/investment-projects is the real
// case — du-an.ts holds the list screen and the word POST appears nowhere in it.
func TestChuoiDuongDanMaKhongCoPhuongThucGhiThiChuaXong(t *testing.T) {
	da := tuyenGia("POST", "/api/v1/investment-projects")
	da.Service = "finance"

	mh := manHinhGia(map[string]string{
		"du-an.ts": `
export function laySoDuAn() {
  const duongDan: finance_get_investment_projects["duongDan"] = "/api/v1/investment-projects";
  return docJSON(duongDan);
}
`,
	})
	if mh.daDung(da) {
		t.Fatal("một màn hình CHỈ ĐỌC bị đọc thành đã dựng cả tuyến ghi cùng đường dẫn")
	}
}

// GET is exempt from the method check: no fetch wrapper spells it, so demanding it would reject
// every read route in the repository.
func TestGETKhongCanChuPhuongThuc(t *testing.T) {
	g := tuyenGia("GET", "/api/v1/budget-indicators")
	g.Service = "finance"

	mh := manHinhGia(map[string]string{
		"thu-chi.ts": `export const d = "/api/v1/budget-indicators";`,
	})
	if !mh.daDung(g) {
		t.Fatal("tuyến đọc bị đòi chữ GET, thứ không client nào viết")
	}
}

// --- hai cách nhận nhầm hàng xóm ------------------------------------------------------------

// /api/v1/tasks and /api/v1/tasks/{ma} are two screens, and the nesting has to be rejected in
// BOTH DIRECTIONS — they fail for different reasons and one test cannot cover the other.
//
//	shorter literal, longer route   a substring search over the file finds the list path inside
//	                                nothing, so this direction is safe by accident today; it is
//	                                pinned so that "search the whole file for the path" never
//	                                comes back as a simplification.
//	longer literal, shorter route   THE ONE THAT SURVIVED THE MUTATION PASS. Dropping the closing
//	                                quote from the comparison leaves a prefix match, and then the
//	                                detail screen marks the list screen finished. The list screen
//	                                is then never built, silently — the exact direction of error
//	                                this whole mechanism must not have.
func TestDuongDanLongNhauKhongNhanNham(t *testing.T) {
	danhSach := tuyenGia("GET", "/api/v1/tasks")
	danhSach.Service = "petitions"

	ngan := manHinhGia(map[string]string{
		"nhiem-vu.ts": `export const d = "/api/v1/tasks";`,
	})
	if ngan.daDung(tuyenNV) {
		t.Error("màn hình danh sách bị tính là đã dựng cả màn hình chi tiết")
	}

	dai := manHinhGia(map[string]string{
		"nhiem-vu.ts": `export const d = "/api/v1/tasks/{ma}";`,
	})
	if dai.daDung(danhSach) {
		t.Error("màn hình chi tiết bị tính là đã dựng cả màn hình danh sách")
	}
}

// identity_get_staff must not be found inside identity_get_staff_by_id.
func TestMaThaoTacPhaiTronVenKhongPhaiTienTo(t *testing.T) {
	cb := tuyenGia("GET", "/api/v1/staff")
	if maOperation(cb) != "identity_get_staff" {
		t.Fatalf("ca này dựng trên mã thao tác %q — sửa ca nếu quy tắc đặt tên đổi", maOperation(cb))
	}
	mh := manHinhGia(map[string]string{
		"can-bo.ts": `export type X = identity_get_staff_by_id["duongDan"];`,
	})
	if mh.daDung(cb) {
		t.Fatal("mã thao tác của tuyến này khớp vào giữa tên của tuyến khác")
	}
}

// --- bóc chú thích: giữ nguyên vị trí -------------------------------------------------------

// Blanking instead of deleting is what keeps the result a faithful map of the file — the same
// property check_khoa_duy_nhat.py needs to compute a line number after stripping. A stripper
// that shortens the text passes every "is it gone" test and quietly breaks every offset.
func TestBoChuThichGiuNguyenDoDaiVaXuongDong(t *testing.T) {
	nguon := "const a = 1; // ghi chú\n/* nhiều\n   dòng */ const b = \"// không phải chú thích\";\n"
	ra := boChuThichTS(nguon)

	if len(ra) != len(nguon) {
		t.Fatalf("độ dài đổi: %d -> %d", len(nguon), len(ra))
	}
	if strings.Count(ra, "\n") != strings.Count(nguon, "\n") {
		t.Fatalf("số dòng đổi: %d -> %d", strings.Count(nguon, "\n"), strings.Count(ra, "\n"))
	}
	if strings.Contains(ra, "ghi chú") || strings.Contains(ra, "nhiều") {
		t.Errorf("chú thích còn sót: %q", ra)
	}
	if !strings.Contains(ra, "const a = 1;") || !strings.Contains(ra, "const b =") {
		t.Errorf("mã thật bị xoá mất: %q", ra)
	}
	// A // inside a string literal is text, not a comment. Blanking from there would swallow the
	// rest of the line — including, in this client layer, the path that follows on the next one.
	if !strings.Contains(ra, "// không phải chú thích") {
		t.Errorf("dấu // trong một chuỗi bị xử như chú thích: %q", ra)
	}
}

// --- docManHinh: phạm vi và hỏng-thì-đóng ---------------------------------------------------

func vietTep(t *testing.T, goc, rel, noiDung string) {
	t.Helper()
	p := filepath.Join(goc, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(noiDung), 0o644); err != nil {
		t.Fatal(err)
	}
}

// schema.gen.ts IS GENERATED FROM THE CONTRACT and names every route in the repository. Reading
// it answers "does this route exist", not "did anybody build it". With it in scope the detector
// reported 42 of 42 open tasks finished on 2026-09-24 — the exact lie it was written to end.
func TestSchemaGenBiLoaiTru(t *testing.T) {
	goc := t.TempDir()
	vietTep(t, goc, thuMucManHinh+"/schema.gen.ts", `
export type petitions_get_tasks_by_ma = { duongDan: "/api/v1/tasks/{ma}" };
`)
	vietTep(t, goc, thuMucManHinh+"/goi.ts", `export const CHUNG = {};`)

	mh, err := docManHinh(goc)
	if err != nil {
		t.Fatal(err)
	}
	if _, co := mh.nguon[filepath.ToSlash(thuMucManHinh)+"/schema.gen.ts"]; co {
		t.Fatal("tệp sinh ra từ hợp đồng vẫn được nạp")
	}
	if mh.daDung(tuyenNV) {
		t.Fatal("hợp đồng tự khai một tuyến bị đếm thành một màn hình đã dựng")
	}
}

// HỎNG THÌ ĐÓNG, VÀ ĐÓNG BẰNG CÁCH KÊU. An empty read is indistinguishable from "no screen was
// ever built", so it would freeze every task in open/ while the run prints success — which is
// the failure mode this whole change exists to remove, reintroduced one level down.
func TestThieuLopGoiAPIThiLaLoi(t *testing.T) {
	if _, err := docManHinh(t.TempDir()); err == nil {
		t.Fatal("thiếu hẳn web-admin/src/lib/api mà vẫn chạy tiếp")
	}

	goc := t.TempDir()
	if err := os.MkdirAll(filepath.Join(goc, thuMucManHinh), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := docManHinh(goc); err == nil {
		t.Fatal("thư mục không có tệp .ts nào mà vẫn chạy tiếp")
	}
}

// --- hàng đợi: bốn thư mục dưới dấu hiệu mới ------------------------------------------------

func mhNV() manHinh {
	return manHinhGia(map[string]string{
		"nhiem-vu.ts": `const mau: petitions_get_tasks_by_ma["duongDan"] = "/api/v1/tasks/{ma}";`,
	})
}

func TestViecOOpenCoManHinhThiChuyenSangDone(t *testing.T) {
	goc := filepath.Join(t.TempDir(), "web")
	ma := maViec(tuyenNV.Service, tuyenNV.Method, tuyenNV.Path)

	kq, err := dongBoViec(goc, []tuyen{tuyenNV}, mhNV())
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.DaCoManHinh) != 1 || kq.DaCoManHinh[0].ID != ma {
		t.Fatalf("mong đúng một việc được chuyển vì đã có màn hình, được %+v", kq.DaCoManHinh)
	}
	// Named, not counted: the report has to be able to say WHICH route it decided about.
	if kq.DaCoManHinh[0].Method != "GET" || kq.DaCoManHinh[0].Path != "/api/v1/tasks/{ma}" {
		t.Errorf("báo cáo không nêu được route: %+v", kq.DaCoManHinh[0])
	}
	if _, err := os.Stat(filepath.Join(goc, "done", ma+".json")); err != nil {
		t.Errorf("việc không nằm ở done/: %v", err)
	}
	if demTep(t, filepath.Join(goc, "open")) != 0 {
		t.Error("open/ phải rỗng")
	}
	// KHÔNG XOÁ, CHUYỂN. "Vì sao nó biến mất" là câu sẽ có người hỏi — đúng lý do stale/ tồn tại.
	if demTep(t, filepath.Join(goc, "done")) != 1 {
		t.Error("việc bị xoá thay vì được chuyển")
	}
}

// claimed/ BELONGS TO THE AGENT HOLDING IT, not to this generator. Taking the file out of their
// hands mid-build destroys the context at the moment they need it — and they are about to
// rename it themselves anyway.
func TestViecDangOClaimedCoManHinhVanKhongBiDung(t *testing.T) {
	goc := filepath.Join(t.TempDir(), "web")
	ma := maViec(tuyenNV.Service, tuyenNV.Method, tuyenNV.Path)
	if _, err := dongBoViec(goc, []tuyen{tuyenNV}, manHinh{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(
		filepath.Join(goc, "open", ma+".json"),
		filepath.Join(goc, "claimed", ma+".json")); err != nil {
		t.Fatal(err)
	}

	kq, err := dongBoViec(goc, []tuyen{tuyenNV}, mhNV())
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.DaCoManHinh) != 0 {
		t.Errorf("việc đang ở claimed/ bị đụng vào: %+v", kq.DaCoManHinh)
	}
	if _, err := os.Stat(filepath.Join(goc, "claimed", ma+".json")); err != nil {
		t.Errorf("tệp trong claimed/ không còn: %v", err)
	}
}

// A route that vanished from the server is stale/, full stop. A client call left behind in the
// web is not evidence the work is finished — it is evidence there is dead code to clean up, and
// filing it as done would hide exactly that.
//
// THIS CASE IS THE ONLY THING HOLDING THAT INVARIANT, and the mutation run says so precisely:
// dongBoViec guards it twice (the stale sweep runs first, and theoMa has no entry for a dead
// route). Removing either guard alone leaves this case green; removing both turns it red. So do
// not read a green suite as permission to delete one of the two.
func TestRouteBienMatThiStaleThangHonDaCoManHinh(t *testing.T) {
	goc := filepath.Join(t.TempDir(), "web")
	ma := maViec(tuyenNV.Service, tuyenNV.Method, tuyenNV.Path)
	if _, err := dongBoViec(goc, []tuyen{tuyenNV}, manHinh{}); err != nil {
		t.Fatal(err)
	}

	kq, err := dongBoViec(goc, []tuyen{tuyenGia("GET", "/api/v1/staff")}, mhNV())
	if err != nil {
		t.Fatal(err)
	}
	if kq.Stale != 1 || len(kq.DaCoManHinh) != 0 {
		t.Fatalf("mong stale=1 daCoManHinh=0, được stale=%d %+v", kq.Stale, kq.DaCoManHinh)
	}
	if _, err := os.Stat(filepath.Join(goc, "stale", ma+".json")); err != nil {
		t.Errorf("việc không vào stale/: %v", err)
	}
}

// One direction only. A screen that gets deleted, refactored, or renamed must never push a task
// back onto the board: done/ is history, and history is not recomputed.
func TestDaOTrongDoneThiKhongBaoGioQuayLaiOpen(t *testing.T) {
	goc := filepath.Join(t.TempDir(), "web")
	ma := maViec(tuyenNV.Service, tuyenNV.Method, tuyenNV.Path)
	if _, err := dongBoViec(goc, []tuyen{tuyenNV}, mhNV()); err != nil {
		t.Fatal(err)
	}

	// Lượt sau: lớp gọi API không còn nhắc tuyến này nữa.
	kq, err := dongBoViec(goc, []tuyen{tuyenNV}, manHinh{})
	if err != nil {
		t.Fatal(err)
	}
	if kq.Moi != 0 || kq.HoiSinh != 0 {
		t.Fatalf("việc đã xong bị dựng lại: %+v", kq)
	}
	if _, err := os.Stat(filepath.Join(goc, "done", ma+".json")); err != nil {
		t.Errorf("việc rời khỏi done/: %v", err)
	}
	if demTep(t, filepath.Join(goc, "open")) != 0 {
		t.Error("open/ phải rỗng")
	}
}

// --- trên mã THẬT của kho -------------------------------------------------------------------

// A FLOOR, NOT AN EXACT COUNT, and it is here for one reason: a scanner that silently reads
// nothing returns false for every route, which looks identical to "no screen has been built".
// Every case above would still pass, because every case above builds its own fixture.
//
// Measured 2026-09-24 over all 113 routes this generator scans: 100 already have a client call
// in the 35 files of the client layer. Of those, 2 are found ONLY by the operation id and 6
// ONLY by the path literal — which is the measurement behind keeping both signals rather than
// the tidier one.
//
// The floor sits below 100 with room for screens to be deleted or refactored, and far above the
// zero a dead scanner reports. It cannot be an equality: routes and screens are both added
// weekly, and a number that has to be edited on every legitimate change is a number people edit
// without reading.
const soTuyenDaCoManHinhToiThieu = 80

func TestTrenKhoThatDemDuocMotSoLuongHopLy(t *testing.T) {
	goc, err := timGoc()
	if err != nil {
		t.Fatal(err)
	}
	tuyens, err := quetTuyen(goc)
	if err != nil {
		t.Fatal(err)
	}
	mh, err := docManHinh(goc)
	if err != nil {
		t.Fatal(err)
	}

	dem := 0
	for _, x := range tuyens {
		if mh.daDung(x) {
			dem++
		}
	}
	if dem < soTuyenDaCoManHinhToiThieu {
		t.Fatalf("chỉ thấy %d/%d tuyến đã có màn hình, dưới ngưỡng %d — bộ quét đang đọc hụt, "+
			"đừng hạ ngưỡng trước khi biết vì sao", dem, len(tuyens), soTuyenDaCoManHinhToiThieu)
	}

	// Named routes, because a count alone cannot say WHICH routes were counted. Both are screens
	// that have been in done/ for weeks; they are not going to un-exist.
	theoKhoa := map[string]tuyen{}
	for _, x := range tuyens {
		theoKhoa[x.Method+" "+x.Path] = x
	}
	for _, khoa := range []string{"POST /api/v1/tasks", "GET /api/v1/tasks/{ma}"} {
		x, co := theoKhoa[khoa]
		if !co {
			t.Errorf("kho không còn tuyến %s — ca này cần cập nhật, đừng xoá nó đi", khoa)
			continue
		}
		if !mh.daDung(x) {
			t.Errorf("%s: màn hình đã dựng từ lâu mà không nhận ra", khoa)
		}
	}

	// THE OTHER HALF, AND IT IS THE HALF THE FLOOR CANNOT COVER: a detector that answered true
	// unconditionally would clear the floor above and empty the whole queue into done/.
	//
	// A SYNTHETIC PATH, NOT A REAL UNBUILT ONE. Naming a real route that nobody has built yet
	// makes this case go red on the day somebody builds it — a test that predictably breaks for a
	// good reason is a test people learn to edit without reading. Screens for real routes appear
	// weekly; this path never will.
	if mh.daDung(tuyenGia("GET", "/api/v1/tuyen-khong-bao-gio-ton-tai")) {
		t.Error("một tuyến không tồn tại vẫn được coi là đã có màn hình — phép suy đang trả true cho mọi thứ")
	}
}
