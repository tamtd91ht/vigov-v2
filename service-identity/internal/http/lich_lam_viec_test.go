package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR, AND WHAT IT DELIBERATELY CANNOT PROVE YET.
//
// The three calendar handlers are complete and the three stores are wired, but NO ROUTE IS
// MOUNTED: the URL resource names have no row in kb/00-foundation/ubiquitous-language.md and are
// being asked rather than guessed (ADR 0011, kb/INDEX.yaml `not_here`). So:
//
//	PROVED HERE   the handler's own behaviour, against the REAL edge chain — the commune check,
//	              the response shape, the empty-calendar problem, the overlap problem, the refusal
//	              of a self-contradicting calendar, the year parameter, the ceiling, the leak case.
//	NOT PROVED    that the SHIPPED route declares a permission. That is rule 5, invariant 7, and it
//	              can only be asserted against a `mux.Handle` statement in Register. The four cases
//	              land in the same turn as the three statements, and this file's stand-ins are
//	              exactly what they look like: a harness, not a contract.
//
// THE PATHS BELOW ARE HARNESS-ONLY AND ARE NOT PROPOSALS. They carry `test-probe` for the same
// reason duongThu does — so nobody reads them as a decided name, and so they cannot collide with a
// real route later.

const (
	duongThuLichLamViec = "/api/v1/test-probe/working-calendar"
	duongThuNgayNghiLe  = "/api/v1/test-probe/closure-days"
	duongThuNgayLamBu   = "/api/v1/test-probe/swap-days"
)

// mayChuLich is the ordinary harness with the three calendar handlers mounted behind the
// declaration the routes are INTENDED to carry — AnyAuthenticated, the call the user settled for
// catalogue reads on 2026-09-20.
//
// MOUNTING THEM HERE PROVES THE HANDLERS, NOT THE ROUTES, and the difference is the whole reason
// the note above exists: a stand-in can only show that authz works, never that the statement being
// shipped declared it.
func mayChuLich(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	m.them = func(mux *http.ServeMux, d Deps) {
		h := NewHandler(d)
		ly := authz.AnyAuthenticated("giờ làm việc và ngày nghỉ của xã hiện trên mọi màn hình có hạn xử lý — đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản không phải quản trị")
		mux.Handle("GET "+duongThuLichLamViec, ly(http.HandlerFunc(h.DanhSachCaLamViec)))
		mux.Handle("GET "+duongThuNgayNghiLe, ly(http.HandlerFunc(h.DanhSachNgayNghiLe)))
		mux.Handle("GET "+duongThuNgayLamBu, ly(http.HandlerFunc(h.DanhSachCaLamBu)))
	}
	m.dungLai(t, nil)
	return m
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

// --- the edge, on all three ---------------------------------------------------------------------

func TestLichLamViec_401KhongToken(t *testing.T) {
	m := mayChuLich(t)

	for _, duong := range []string{duongThuLichLamViec,
		duongThuNgayNghiLe + "?year=2026", duongThuNgayLamBu + "?year=2026"} {
		doiMa(t, m.goi(t, "GET", hostA, duong, "", ""), http.StatusUnauthorized)
	}
	if m.lichLamViec.goi != 0 || m.ngayNghiLe.goi != 0 || m.ngayLamBu.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc lịch của xã")
	}
}

func TestLichLamViec_401XaKhac(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE as it reads on an AnyAuthenticated route: there
	// is no permission to be right or wrong about, so what remains is a token issued by commune A
	// presented at commune B's domain. It is refused at the TOKEN layer, before any store is
	// touched — which is what makes another commune's calendar unreachable rather than merely
	// unrequested.
	m := mayChuLich(t)

	w := m.goi(t, "GET", hostB, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "tenant_mismatch" {
		t.Errorf("code = %q, muốn tenant_mismatch", got)
	}
	if m.lichLamViec.goi != 0 {
		t.Error("token của xã khác mà vẫn đọc lịch của xã này")
	}
}

func TestLichLamViec_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE DECISION THIS PINS: an account holding NOTHING still gets the calendar. Office hours and
	// holidays are on every screen that states a deadline, so a configuration permission here
	// would empty those screens for everybody who is not an administrator.
	m := mayChuLich(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{} // no grants at all, in any commune
	})

	w := m.goi(t, "GET", hostA, duongThuLichLamViec, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if len(docLich(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận lịch rỗng — tuyến này phải trả đủ")
	}
}

func TestLichKhongVuotSangXaKhac(t *testing.T) {
	// The same account, signed in properly at commune B. Commune A's working hours must not travel
	// with the person — nothing about the request is malformed, and this is the shape a leak
	// actually takes.
	m := mayChuLich(t)

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
	m := mayChuLich(t)

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
	m := mayChuLich(t)
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
	m := mayChuLich(t)
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
	m := mayChuLich(t)
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
	m := mayChuLich(t)
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
	m := mayChuLich(t)

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
	m := mayChuLich(t)

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
	m := mayChuLich(t)

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
	m := mayChuLich(t)

	w := m.goi(t, "GET", hostA, duongThuNgayNghiLe+"?year=2030", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

func TestNgayNghiLeTraDungNhungTruongCuaHopDong(t *testing.T) {
	m := mayChuLich(t)

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
	m := mayChuLich(t)

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
	m := mayChuLich(t)
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
	m := mayChuLich(t)
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
	m := mayChuLich(t)
	m.ngayLamBu.loi = idstore.ErrQuaNhieuNgayLamBu

	w := m.goi(t, "GET", hostA, duongThuNgayLamBu+"?year=2026", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if e := loiTra(t, w); e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
}

func TestNgayNghiLeLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := mayChuLich(t)
	m.ngayNghiLe.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongThuNgayNghiLe+"?year=2026", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}
