package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: the three calendar routes of migration 0006, read against the REAL edge
// chain and the REAL route table — Register mounts them, so every case below fails if a statement
// in routes.go loses its declaration, changes its path or stops being wired.
//
// THE PATHS ARE THE SHIPPED ONES. They were asked and answered by the user on 2026-09-20 and match
// the `@entity` marks in migration 0006; there is no stand-in left in this file. An earlier
// revision mounted these handlers at `/api/v1/test-probe/...` because the nouns were still open —
// a stand-in can only show that authz works, never that the statement being shipped declared it,
// which is exactly why it did not survive the names being settled.
//
// SIX PROPERTIES, each of which fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route — see
//     TestLichRoute_200KhongCoQuyenNaoVaKhongHoiChecker for what "403 wrong permission" becomes;
//  2. one commune's calendar never reaches another commune's caller;
//  3. an EMPTY calendar is answered 200 with a NAMED problem, never as ordinary office hours;
//  4. two overlapping sessions are SURFACED, and the rows still come back whole;
//  5. a date recorded as both a holiday and a swap day is REFUSED by BOTH date routes;
//  6. the mandatory `year` is parsed before any store is touched, and a repeated one is refused.

const (
	duongThuLichLamViec = "/api/v1/working-hours"
	duongThuNgayNghiLe  = "/api/v1/public-holidays"
	duongThuNgayLamBu   = "/api/v1/swap-working-days"
)

// checkerDem is authz.Checker that GRANTS NOTHING AND COUNTS BEING ASKED.
//
// THE COUNT IS THE ASSERTION, and it is the half of "AnyAuthenticated" that a permissive checker
// can never show. A route declared RequirePermission with a checker that happened to grant the key
// would also answer 200; only "the checker was never consulted" tells the two apart. Swap one of
// the three statements to RequirePermission and this fake turns the test red twice over — once on
// the status, once on the count.
type checkerDem struct{ goi int }

func (c *checkerDem) Allows(context.Context, authz.Principal, authz.Perm) bool {
	c.goi++
	return false
}

func docLich(t *testing.T, than []byte) danhSachCaLamViecRa {
	t.Helper()
	var ra danhSachCaLamViecRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

func docLamBu(t *testing.T, than []byte) danhSachCaLamBuRa {
	t.Helper()
	var ra danhSachCaLamBuRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases of rule 5, invariant 7, on the SHIPPED statements -----------------------
//
// All three routes are declared AnyAuthenticated, so the four cases read slightly differently from
// a guarded route's — and the two that change are the interesting ones:
//
//	401 no token                  unchanged
//	403 WRONG PERMISSION          becomes 200 WITH NO PERMISSION AT ALL, and the checker must never
//	                              be asked. That is the whole content of the declaration: a route
//	                              that merely happened to grant the key would also answer 200
//	403 right permission,         becomes 401 tenant_mismatch, refused at the TOKEN layer before
//	WRONG COMMUNE                 any store is touched
//	200 both correct              unchanged

func TestLichRoute_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	for _, duong := range []string{duongThuLichLamViec,
		duongThuNgayNghiLe + "?year=2026", duongThuNgayLamBu + "?year=2026"} {
		doiMa(t, m.goi(t, "GET", hostA, duong, "", ""), http.StatusUnauthorized)
	}
	// Not one store was touched. The global guard rejects before any handler runs, and asserting
	// the count is what proves the order rather than the outcome.
	if m.lichLamViec.goi != 0 || m.ngayNghiLe.goi != 0 || m.ngayLamBu.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc lịch của xã")
	}
}

func TestLichRoute_200KhongCoQuyenNaoVaKhongHoiChecker(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE, AS IT READS ON AN AnyAuthenticated ROUTE — and it is the
	// decision the three statements were declared with, so it is asserted rather than assumed.
	//
	// The account is rebuilt holding NOTHING, behind a checker that refuses everything AND COUNTS
	// BEING ASKED. All three routes must answer 200, and the count must stay at zero: office hours
	// and holidays sit under every deadline on every screen, so a configuration permission here
	// would not protect anything — it would empty those screens for everybody who is not an
	// administrator.
	//
	// If somebody later "tightens" one of these to RequirePermission, this goes red twice: the
	// status becomes 403 and the counter moves. That is the point.
	dem := &checkerDem{}
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) { d.Checker = dem })

	for ten, duong := range map[string]string{
		"working-hours":     duongThuLichLamViec,
		"public-holidays":   duongThuNgayNghiLe + "?year=2026",
		"swap-working-days": duongThuNgayLamBu + "?year=2026",
	} {
		t.Run(ten, func(t *testing.T) {
			w := m.goi(t, "GET", hostA, duong, "", m.tokenCho(t, xaA, sidA))
			doiMa(t, w, http.StatusOK)
		})
	}
	if dem.goi != 0 {
		t.Errorf("Checker bị hỏi %d lần — ba tuyến này khai AnyAuthenticated, không có quyền nào để kiểm", dem.goi)
	}
	// And the body is the real list, not an empty one: "200 with nothing in it" would satisfy the
	// status assertion while proving the opposite of what this test is for.
	w := m.goi(t, "GET", hostA, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA))
	if len(docLich(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận lịch rỗng — tuyến này phải trả đủ")
	}
}

func TestLichRoute_401XaKhac(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE as it reads here: there is no permission to be
	// right or wrong about, so what remains is a token issued by commune A presented at commune
	// B's domain. It is refused at the TOKEN layer, BEFORE any store is touched — which is what
	// makes another commune's calendar unreachable rather than merely unrequested.
	m := dungMayChu(t)

	for ten, duong := range map[string]string{
		"working-hours":     duongThuLichLamViec,
		"public-holidays":   duongThuNgayNghiLe + "?year=2026",
		"swap-working-days": duongThuNgayLamBu + "?year=2026",
	} {
		t.Run(ten, func(t *testing.T) {
			w := m.goi(t, "GET", hostB, duong, "", m.tokenCho(t, xaA, sidA))
			doiMa(t, w, http.StatusUnauthorized)
			if got := loiTra(t, w).Code; got != "tenant_mismatch" {
				t.Errorf("code = %q, muốn tenant_mismatch", got)
			}
		})
	}
	if m.lichLamViec.goi != 0 || m.ngayNghiLe.goi != 0 || m.ngayLamBu.goi != 0 {
		t.Error("token của xã khác mà vẫn đọc lịch của xã này")
	}
}

func TestLichRoute_200DuCaHai(t *testing.T) {
	// Signed in, at its own commune, holding the ordinary fixture's grants. All three answer 200
	// and each store is asked exactly once — a route mounted twice, or a handler reading a second
	// store "to enrich" the answer, moves that count.
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	doiMa(t, m.goi(t, "GET", hostA, duongThuNgayNghiLe+"?year=2026", "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	doiMa(t, m.goi(t, "GET", hostA, duongThuNgayLamBu+"?year=2026", "", m.tokenCho(t, xaA, sidA)), http.StatusOK)

	if m.lichLamViec.goi != 1 || m.ngayNghiLe.goi != 1 || m.ngayLamBu.goi != 1 {
		t.Errorf("số lần đọc kho = (%d, %d, %d), muốn (1, 1, 1)",
			m.lichLamViec.goi, m.ngayNghiLe.goi, m.ngayLamBu.goi)
	}
}

func TestLichKhongVuotSangXaKhac(t *testing.T) {
	// The same account, signed in properly at commune B. Commune A's working hours must not travel
	// with the person — nothing about the request is malformed, and this is the shape a leak
	// actually takes.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongThuLichLamViec, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "llv-001") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được ca làm việc của xã A: %s", w.Body.String())
	}
	ra := docLich(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "llv-b-001" || ra.Items[0].Weekday != 6 {
		t.Fatalf("xã B phải nhận đúng lịch của mình, nhận: %+v", ra.Items)
	}

	w = m.goi(t, "GET", hostB, duongThuNgayNghiLe+"?year=2026", "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "Tết Dương lịch") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được ngày nghỉ của xã A: %s", w.Body.String())
	}
}

// --- the weekly calendar -------------------------------------------------------------------------

func TestLichLamViec_200TraDungHinhDang(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docLich(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("nhận %d ca, muốn 2", len(ra.Items))
	}
	mot := ra.Items[0]
	if mot.ID != "llv-001" || mot.Weekday != 1 || mot.Note != "Buổi sáng" {
		t.Errorf("bản ghi đầu sai: %+v", mot)
	}
	// HH:MM:SS, fixed width, seconds always present — one shape on the wire.
	if mot.Start != "07:30:00" || mot.End != "11:30:00" {
		t.Errorf("giờ sai định dạng hoặc đảo chỗ: %q–%q", mot.Start, mot.End)
	}
	if ra.Items[1].Start != "13:30:00" || ra.Items[1].End != "17:00:00" {
		t.Errorf("ca chiều sai: %+v", ra.Items[1])
	}
	// A healthy week reports NO problems, and `problems` is [] rather than null. A fixture that
	// always carried a defect could not show this.
	if len(ra.Problems) != 0 {
		t.Errorf("tuần hợp lệ mà báo vấn đề: %+v", ra.Problems)
	}
	if !strings.Contains(w.Body.String(), `"problems":[]`) {
		t.Errorf("problems phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

func TestLichLamViecXaChuaCauHinhTra200VaNeuTenVanDe(t *testing.T) {
	// THE STATE EVERY COMMUNE IS IN TODAY — migration 0006 seeds nothing and the onboarding step
	// does not exist. Three things have to hold at once and each one matters:
	//
	//	200          the configuration screen that FIXES this must be able to load
	//	items: []    never null
	//	problems     names `empty_calendar`, so no client can read the empty list as ordinary
	//	             office hours. A default here — "Mon–Fri 08:00–17:00" — is a commitment
	//	             invented by software and told to a citizen (rule 10; migration 0006:56).
	m := dungMayChu(t)
	m.lichLamViec.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if !strings.Contains(than, `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", than)
	}
	ra := docLich(t, w.Body.Bytes())
	if len(ra.Problems) != 1 || ra.Problems[0].Kind != string(domain.VanDeLichTrong) {
		t.Fatalf("lịch rỗng phải được nêu tên, nhận: %+v", ra.Problems)
	}
	// `weekday` is null, not 0: an empty calendar belongs to no weekday, and "thứ 0" is a day
	// nobody configured.
	if ra.Problems[0].Weekday != nil {
		t.Errorf("weekday = %v, muốn null", *ra.Problems[0].Weekday)
	}
	if !strings.Contains(than, `"weekday":null`) {
		t.Errorf("weekday phải là null trên vấn đề lịch rỗng: %s", than)
	}
	if ra.Problems[0].Message == "" {
		t.Error("thiếu câu tiếng Việt cho người đọc màn hình cấu hình")
	}
}

func TestLichLamViecChongCaThiNeuRaChuKhongTinhHaiLan(t *testing.T) {
	// THE OVERLAP THE DATABASE CANNOT REFUSE: the EXCLUDE constraint needs `btree_gist`, and a
	// migration that fails for a missing extension stops the service (ADR 0013), so migration
	// 0006:109 hands the obligation to the read side. Two sessions of 07:30–11:30 and 09:00–12:00
	// are rows PostgreSQL accepts and two and a half hours counted twice.
	m := dungMayChu(t)
	m.lichLamViec.theo[xaA] = []domain.CaLamViec{
		{ID: "llv-001", Thu: 3, BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60, GhiChu: "Buổi sáng"},
		{ID: "llv-002", Thu: 3, BatDau: 9 * 3600, KetThuc: 12 * 3600, GhiChu: "Ca nhập nhầm"},
	}

	w := m.goi(t, "GET", hostA, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docLich(t, w.Body.Bytes())
	// THE ROWS STILL COME BACK WHOLE. The screen that fixes the overlap is the one showing both
	// sessions; hiding one of them would be the silent repair this refuses to perform.
	if len(ra.Items) != 2 {
		t.Fatalf("nhận %d ca, muốn 2 — vấn đề được NÊU, không phải tự sửa", len(ra.Items))
	}
	if len(ra.Problems) != 1 || ra.Problems[0].Kind != string(domain.VanDeCaChongNhau) {
		t.Fatalf("chồng ca không được nêu: %+v", ra.Problems)
	}
	if ra.Problems[0].Weekday == nil || *ra.Problems[0].Weekday != 3 {
		t.Errorf("weekday = %v, muốn 3 — màn hình cần biết sửa ngày nào", ra.Problems[0].Weekday)
	}
	if len(ra.Problems[0].SessionIDs) != 2 ||
		ra.Problems[0].SessionIDs[0] != "llv-001" || ra.Problems[0].SessionIDs[1] != "llv-002" {
		t.Errorf("session_ids = %v, muốn [llv-001 llv-002]", ra.Problems[0].SessionIDs)
	}
}

func TestLichLamViecVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// 500 rather than a short week, and it is the same argument as every other ceiling in this
	// service with a heavier consequence: a session missing from the calendar makes every deadline
	// computed afterwards longer than the commitment the commune actually made.
	m := dungMayChu(t)
	m.lichLamViec.loi = idstore.ErrQuaNhieuCaLamViec

	w := m.goi(t, "GET", hostA, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if e := loiTra(t, w); e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
	if strings.Contains(w.Body.String(), `"items"`) {
		t.Errorf("phản hồi từ chối vẫn kèm danh sách: %s", w.Body.String())
	}
}

func TestLichLamViecLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.lichLamViec.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

func TestLichLamViecTraDungNhungTruongCuaHopDong(t *testing.T) {
	// THE FIELDS THAT ARE ABSENT ARE THE DESIGN, asserted rather than assumed. `tenant_id` never
	// leaves this service (rule 1, invariant 4); the three soft-delete columns are the commune's
	// internal record of an administrative act, not part of a timetable.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	// NON-EMPTY FIRST: on an empty list the loop below runs zero times and passes having checked
	// nothing.
	if len(tho.Items) == 0 {
		t.Fatal("dữ liệu mẫu rỗng — phép kiểm sẽ xanh mà không kiểm gì")
	}
	muon := map[string]bool{"id": true, "weekday": true, "start": true, "end": true, "note": true}
	for _, mot := range tho.Items {
		for khoa := range mot {
			if !muon[khoa] {
				t.Errorf("trường ngoài hợp đồng lọt ra: %q — %s", khoa, w.Body.String())
			}
		}
		if len(mot) != len(muon) {
			t.Errorf("thiếu trường: có %v, muốn %v", mot, muon)
		}
	}
}

// --- the year parameter, shared by the two date routes -------------------------------------------

func TestNgayNghiLeThieuNamThiTuChoiTruocKhiChamKho(t *testing.T) {
	// "THIS YEAR" IS NOT A DEFAULT THIS CODE GETS TO PICK. A screen that never sent the parameter
	// would silently change window at midnight on 31/12, and nothing in any test would move.
	//
	// The store must not be touched: a rejected request runs no statement at all.
	m := dungMayChu(t)

	for ten, duong := range map[string]string{
		"thiếu hẳn":     duongThuNgayNghiLe,
		"rỗng":          duongThuNgayNghiLe + "?year=",
		"không phải số": duongThuNgayNghiLe + "?year=nam-nay",
		"quá xa":        duongThuNgayNghiLe + "?year=1800",
		// TWO VALUES IS ALSO A REFUSAL, and this is the one a `Get()` would have swallowed: it
		// returns the first and drops the rest, so the server would answer one of two years the
		// client asked for without saying which.
		"hai giá trị": duongThuNgayNghiLe + "?year=2026&year=2027",
	} {
		t.Run(ten, func(t *testing.T) {
			w := m.goi(t, "GET", hostA, duong, "", m.tokenCho(t, xaA, sidA))
			doiMa(t, w, http.StatusBadRequest)
			if got := loiTra(t, w).Code; got != "invalid_query" {
				t.Errorf("code = %q, muốn invalid_query", got)
			}
		})
	}
	if m.ngayNghiLe.goi != 0 {
		t.Errorf("tham số hỏng mà vẫn chạy %d truy vấn", m.ngayNghiLe.goi)
	}
}

func TestNgayNghiLeDocDungCuaSoNamNguoiGoiChon(t *testing.T) {
	// The year reaches the store, and a different year is a different answer. A handler that
	// dropped the parameter would pass every other test in this file.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThuNgayNghiLe+"?year=2026", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if m.ngayNghiLe.nam != 2026 {
		t.Errorf("kho nhận năm %d, muốn 2026", m.ngayNghiLe.nam)
	}
	var ra danhSachNgayNghiLeRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if len(ra.Items) != 2 || ra.Items[0].Date != "2026-01-01" || ra.Items[0].Name != "Tết Dương lịch" {
		t.Fatalf("năm 2026 trả sai: %+v", ra.Items)
	}

	w = m.goi(t, "GET", hostA, duongThuNgayNghiLe+"?year=2025", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if len(ra.Items) != 1 || ra.Items[0].ID != "nnl-2025" {
		t.Fatalf("cửa sổ năm không được tôn trọng: %+v", ra.Items)
	}
}

func TestNgayNghiLeNamChuaNhapTraMangRong(t *testing.T) {
	// [] AND NOT null, and it is NOT the same statement as an empty weekly calendar: a commune
	// with no holidays entered still has working hours, so there is no problem to report here.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThuNgayNghiLe+"?year=2030", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

func TestNgayNghiLeTraDungNhungTruongCuaHopDong(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThuNgayNghiLe+"?year=2026", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if len(tho.Items) == 0 {
		t.Fatal("dữ liệu mẫu rỗng — phép kiểm sẽ xanh mà không kiểm gì")
	}
	muon := map[string]bool{"id": true, "date": true, "name": true}
	for _, mot := range tho.Items {
		for khoa := range mot {
			if !muon[khoa] {
				t.Errorf("trường ngoài hợp đồng lọt ra: %q — %s", khoa, w.Body.String())
			}
		}
		if len(mot) != len(muon) {
			t.Errorf("thiếu trường: có %v, muốn %v", mot, muon)
		}
	}
}

// --- swap days -----------------------------------------------------------------------------------

func TestNgayLamBu_200TraCaGioLamViecCuaNgayDo(t *testing.T) {
	// A swap day carries a date AND the hours worked, because the weekday it falls on usually has
	// no session at all — the reason it is a second table rather than a flag on the holidays
	// (migration 0006:245). Two rows on one date is a swap day with a lunch break, and it is NOT
	// an overlap.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThuNgayLamBu+"?year=2026", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docLamBu(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("nhận %d ca làm bù, muốn 2", len(ra.Items))
	}
	mot := ra.Items[0]
	if mot.ID != "nlb-001" || mot.Date != "2026-02-21" || mot.Name != "Làm bù nghỉ Tết" {
		t.Errorf("bản ghi đầu sai: %+v", mot)
	}
	if mot.Start != "07:30:00" || mot.End != "11:30:00" {
		t.Errorf("giờ sai định dạng hoặc đảo chỗ: %q–%q", mot.Start, mot.End)
	}
	if len(ra.Problems) != 0 {
		t.Errorf("hai ca cách nhau bởi nghỉ trưa mà bị báo chồng: %+v", ra.Problems)
	}
}

func TestNgayLamBuChongCaThiNeuRa(t *testing.T) {
	// The same double count as the weekly calendar, on a table whose UNIQUE key also only stops
	// two sessions STARTING at the same minute.
	m := dungMayChu(t)
	m.ngayLamBu.theo[xaA][2026] = []domain.CaLamBu{
		{ID: "nlb-001", Ngay: "2026-02-21", BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60, Ten: "Làm bù"},
		{ID: "nlb-002", Ngay: "2026-02-21", BatDau: 9 * 3600, KetThuc: 12 * 3600, Ten: "Làm bù nhập nhầm"},
	}

	w := m.goi(t, "GET", hostA, duongThuNgayLamBu+"?year=2026", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docLamBu(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("vấn đề phải được NÊU chứ không tự sửa, nhận %d dòng", len(ra.Items))
	}
	if len(ra.Problems) != 1 || ra.Problems[0].Date != "2026-02-21" {
		t.Fatalf("chồng ca làm bù không được nêu: %+v", ra.Problems)
	}
	// The same `kind` string the weekly calendar publishes for the same defect, so a client writes
	// one branch rather than two.
	if ra.Problems[0].Kind != string(domain.VanDeCaChongNhau) {
		t.Errorf("kind = %q, muốn %q", ra.Problems[0].Kind, domain.VanDeCaChongNhau)
	}
}

// --- the refusal both date routes share ------------------------------------------------------------

func TestNgayVuaNghiVuaLamBuThiCaHaiTuyenDeuTuChoi(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one that looks like an over-reaction until the
	// alternative is written out: a date recorded as BOTH a closure and a working day is a
	// configuration error, and either winner makes one of two VISIBLE configuration rows do
	// nothing, with nothing on any screen to say which (migration 0006:254).
	//
	// BOTH ROUTES REFUSE. Refusing on one side only would leave the other screen looking healthy,
	// and the commune would fix nothing.
	xung := &domain.LoiNgayVuaNghiVuaLamBu{Ngay: []string{"2026-02-21"}}
	m := dungMayChu(t)
	m.ngayNghiLe.loi = xung
	m.ngayLamBu.loi = xung

	for ten, duong := range map[string]string{
		"ngày nghỉ lễ": duongThuNgayNghiLe + "?year=2026",
		"ngày làm bù":  duongThuNgayLamBu + "?year=2026",
	} {
		t.Run(ten, func(t *testing.T) {
			w := m.goi(t, "GET", hostA, duong, "", m.tokenCho(t, xaA, sidA))
			// 409: nothing failed and nothing about the request is wrong — the state on the server
			// contradicts itself.
			doiMa(t, w, http.StatusConflict)
			e := loiTra(t, w)
			if e.Code != "calendar_conflict" {
				t.Errorf("code = %q, muốn calendar_conflict", e.Code)
			}
			// The message names the day somebody has to go and correct. A date is not personal
			// data (rule 3), and a refusal nobody can act on is a refusal that gets worked around.
			if !strings.Contains(e.Message, "2026-02-21") {
				t.Errorf("thông báo không nêu ngày phải sửa: %q", e.Message)
			}
			// NO LIST COMES BACK WITH THE REFUSAL: a body carrying items beside the error is how a
			// refusal turns back into a client picking a winner.
			if strings.Contains(w.Body.String(), `"items"`) {
				t.Errorf("phản hồi từ chối vẫn kèm danh sách: %s", w.Body.String())
			}
		})
	}
}

func TestNgayLamBuVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	m := dungMayChu(t)
	m.ngayLamBu.loi = idstore.ErrQuaNhieuNgayLamBu

	w := m.goi(t, "GET", hostA, duongThuNgayLamBu+"?year=2026", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if e := loiTra(t, w); e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
}

func TestNgayNghiLeLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.ngayNghiLe.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongThuNgayNghiLe+"?year=2026", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}
