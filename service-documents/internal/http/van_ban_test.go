package http

// WHAT THIS FILE IS FOR: the NINE routes of the two document registers.
//
// FIVE THINGS, each of which fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, on EVERY one of the nine routes — and the third case
//     is the one that is easy to fake, see TestVanBan_403DungQuyenSaiXa;
//  2. each route asks for the key it is SUPPOSED to ask for, compared against a literal rather than
//     against a constant this file also defines — the only assertion here a fake checker cannot
//     satisfy by accident;
//  3. `number`, `status` and `due_at` are REFUSED, not ignored. The first of the three is the one
//     that matters: a client that could name its own register number could reissue a number already
//     printed on a sealed document (rule 7, invariant 3);
//  4. the commune and the acting person reach the use case, because they are what the audit entry is
//     filed under (rule 6, invariant 2) — and the register rows of one commune never reach another;
//  5. the register never answers with a commitment it invented: an SLA that is not configured is a
//     409 naming the screen that fixes it, not a document booked with a made-up deadline.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

const (
	duongVanBanDen = "/api/v1/incoming-documents"
	duongVanBanDi  = "/api/v1/outgoing-documents"
)

// The three keys these routes declare. They live HERE, in the test, and not in routes.go —
// tools/apidoc refuses a key that is not a string literal at the RequirePermission call site, so the
// routes spell them out (see the block above Register).
//
// A second spelling is a second place to drift, so the drift itself is what
// TestVanBan_KhoaQuyenDungChuoiCuaBangQuyen asserts: it reads the key the ROUTE asked for and
// compares it with a literal, not with these constants.
const (
	QuyenTaoVanBan    authz.Perm = "document.create"
	QuyenDocVanBan    authz.Perm = "document.read"
	QuyenChuyenVanBan authz.Perm = "document.route"
)

// thanVaoSo is the smallest valid booking body. Written out rather than marshalled from a struct so
// a test can send a field the Go type does not have — which is the whole point of case (3).
const thanVaoSo = `{"received_date":"2026-09-22","issuing_body":"Huyện uỷ",` +
	`"document_type":"cong-van","summary":"Về việc rà soát hộ nghèo"}`

const thanCapSoDi = `{"document_date":"2026-09-22","document_type":"cong-van",` +
	`"summary":"Trả lời đơn của công dân","recipient":"UBND huyện"}`

// motTuyenVanBan is one of the nine routes, so the four permission cases are asserted on ALL of
// them rather than on whichever one was written first.
type motTuyenVanBan struct {
	ten    string
	method string
	duong  string
	than   string
	khoa   authz.Perm // the key this route is SUPPOSED to declare
	ok     int        // the status a correctly-permitted call returns
}

func chinTuyenVanBan() []motTuyenVanBan {
	return []motTuyenVanBan{
		{"POST đến", http.MethodPost, duongVanBanDen, thanVaoSo, QuyenTaoVanBan, http.StatusCreated},
		{"PATCH đến", http.MethodPatch, duongVanBanDen + "/vbd-a-001", `{"summary":"Sửa trích yếu"}`,
			QuyenTaoVanBan, http.StatusOK},
		{"DELETE đến", http.MethodDelete, duongVanBanDen + "/vbd-a-001", `{"reason":"vào sổ nhầm"}`,
			QuyenTaoVanBan, http.StatusNoContent},
		{"POST chuyển", http.MethodPost, duongVanBanDen + "/vbd-a-001/routings",
			`{"to_unit":"bp-dia-chinh","reason":"Thuộc thẩm quyền bộ phận Địa chính"}`,
			QuyenChuyenVanBan, http.StatusOK},
		{"GET đến", http.MethodGet, duongVanBanDen, "", QuyenDocVanBan, http.StatusOK},

		{"POST đi", http.MethodPost, duongVanBanDi, thanCapSoDi, QuyenTaoVanBan, http.StatusCreated},
		{"PATCH đi", http.MethodPatch, duongVanBanDi + "/vbdi-a-001", `{"summary":"Sửa trích yếu"}`,
			QuyenTaoVanBan, http.StatusOK},
		{"DELETE đi", http.MethodDelete, duongVanBanDi + "/vbdi-a-001", `{"reason":"cấp số nhầm"}`,
			QuyenTaoVanBan, http.StatusNoContent},
		{"GET đi", http.MethodGet, duongVanBanDi, "", QuyenDocVanBan, http.StatusOK},
	}
}

func (m *mayChu) tongGoiVanBan() int { return m.ghiDen.tongGoi() + m.ghiDi.tongGoi() }
func (m *mayChu) tongDocVanBan() int { return m.den.goi + m.di.goi }

// --- (2) the permission keys themselves --------------------------------------------------------

func TestVanBan_KhoaQuyenDungChuoiCuaBangQuyen(t *testing.T) {
	// THE KEY EACH ROUTE ACTUALLY ASKS FOR, COMPARED AGAINST A LITERAL — and that is the whole point.
	//
	// Every other assertion in this file grants the key and then expects that same value to be
	// accepted, so changing a route's key to ANY string at all would leave them all green: the fake
	// checker grants whatever it was handed. That is not hypothetical. This repository carried three
	// invented keys — `finance.read`, `map.read`, `document.approve` — through several sessions with
	// every suite green, because a route holding a key the `quyen` table lacks answers 403 to EVERY
	// account, forever, and nothing says so (rule 5, invariant 3c).
	//
	// All three keys are seeded at service-identity/migrations/0001_init.sql:287-289 and listed in
	// the permission matrix at docs/ui-ux/14-cau-hinh.md:122-124. tools/check_quyen.py scans the
	// whole repository against that table on every `make check` and is the guard a fixture cannot
	// fool; this is the cheap half that turns red in `go test` too.
	for _, tc := range chinTuyenVanBan() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.goiThan(t, tc.method, hostA, tc.duong, canBoCua(xaA), tc.than)

			got := m.checker.hoiKhoaCuoi()
			muon := map[authz.Perm]string{
				QuyenTaoVanBan:    "document.create",
				QuyenDocVanBan:    "document.read",
				QuyenChuyenVanBan: "document.route",
			}[tc.khoa]
			if string(got) != muon {
				t.Fatalf("tuyến hỏi khoá %q, muốn %q — một khoá bảng `quyen` không có là một tuyến "+
					"trả 403 với MỌI tài khoản, mãi mãi, và không phép kiểm nào đỏ", got, muon)
			}
		})
	}
}

// --- (1) the four cases of rule 5, invariant 7 --------------------------------------------------

func TestVanBan_401KhongPhien(t *testing.T) {
	for _, tc := range chinTuyenVanBan() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, tc.khoa) // granted, and still refused: there is nobody to grant it to

			doiMa(t, m.goiThan(t, tc.method, hostA, tc.duong, nil, tc.than), http.StatusUnauthorized)
			if m.tongGoiVanBan() != 0 || m.tongDocVanBan() != 0 {
				t.Error("chưa đăng nhập mà sổ văn bản đã bị đụng tới")
			}
		})
	}
}

func TestVanBan_403SaiQuyen(t *testing.T) {
	// A signed-in account of the right commune holding a DIFFERENT permission. `admin.lookup` is
	// deliberately a real key of this service — the failure being guarded against is not "an account
	// with nothing", it is an administrator who may manage the type catalogue being able to book, to
	// remove, or to read the commune's correspondence.
	for _, tc := range chinTuyenVanBan() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, "admin.lookup")

			doiMa(t, m.goiThan(t, tc.method, hostA, tc.duong, canBoCua(xaA), tc.than),
				http.StatusForbidden)
			if m.tongGoiVanBan() != 0 || m.tongDocVanBan() != 0 {
				t.Error("sai quyền mà sổ văn bản vẫn chạy")
			}
			if m.checker.hoiKhoaCuoi() != tc.khoa {
				t.Errorf("tuyến hỏi khoá %q, muốn %q — một khoá khác là một quyền khác",
					m.checker.hoiKhoaCuoi(), tc.khoa)
			}
		})
	}
}

func TestVanBan_403DungQuyenSaiXa(t *testing.T) {
	// THE CASE THAT IS EASIEST TO FAKE AND HARDEST TO GET RIGHT, so read what it actually sets up.
	//
	// The account belongs to commune B and is signed in AT COMMUNE B: nothing about the request is
	// malformed, and authz's own commune comparison passes. What is wrong is the GRANT — the right
	// to work in the register was given in commune A. A checker that ignored the commune would answer
	// yes here, and commune A's clerk would be booking documents into commune B's register.
	//
	// That is rule 5, invariant 3 in one sentence: a permission missing its commune is cross-commune
	// escalation, not a lesser bug. And it answers 403 rather than 401 because the session is
	// perfectly valid — it is the authority that is absent.
	for _, tc := range chinTuyenVanBan() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, tc.khoa)

			doiMa(t, m.goiThan(t, tc.method, hostB, tc.duong, canBoCua(xaB), tc.than),
				http.StatusForbidden)
			if m.tongGoiVanBan() != 0 || m.tongDocVanBan() != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn vào được sổ của xã này")
			}
		})
	}
}

func TestVanBan_401PhienCuaXaKhac(t *testing.T) {
	// The other shape of "wrong commune": a principal issued by commune A presented at commune B's
	// domain. authz.RequirePermission compares the commune BEFORE the permission and answers 401,
	// not 403 — a browser does not send a cookie across hosts, so this is never an ordinary user
	// error. Asserted so that nobody "corrects" it to 403 and turns a deliberate probe into something
	// that reads like a permissions problem.
	for _, tc := range chinTuyenVanBan() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, tc.khoa)
			m.capQuyen(xaB, tc.khoa)

			doiMa(t, m.goiThan(t, tc.method, hostB, tc.duong, canBoCua(xaA), tc.than),
				http.StatusUnauthorized)
			if m.tongGoiVanBan() != 0 || m.tongDocVanBan() != 0 {
				t.Error("phiên của xã khác mà vẫn vào được sổ")
			}
		})
	}
}

func TestVanBan_DungQuyenDungXa(t *testing.T) {
	for _, tc := range chinTuyenVanBan() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuCoIdem(t)
			m.capQuyen(xaA, tc.khoa)

			w := m.goiThan(t, tc.method, hostA, tc.duong, canBoCua(xaA), tc.than)
			doiMa(t, w, tc.ok)

			// (4) THE COMMUNE AND THE ACTING PERSON REACHED THE LAYER BELOW. The audit entry is filed
			// under both (rule 6, invariant 2), and `actor_id` must hold the staff BUSINESS CODE —
			// `CB-00123` names somebody years later with no lookup still alive; a ULID names nobody
			// (rule 6, invariant 8).
			if tc.method == http.MethodGet {
				if m.tongDocVanBan() != 1 {
					t.Fatalf("tuyến đọc gọi kho %d lần, muốn 1", m.tongDocVanBan())
				}
				return
			}
			if m.tongGoiVanBan() != 1 {
				t.Fatalf("use case ghi chạy %d lần, muốn 1", m.tongGoiVanBan())
			}
			nguoi := m.ghiDen.nguoiCuoi
			xa := m.ghiDen.xaCuoi
			if m.ghiDi.tongGoi() == 1 {
				nguoi, xa = m.ghiDi.nguoiCuoi, m.ghiDi.xaCuoi
			}
			if xa != xaA {
				t.Errorf("use case chạy trong xã %q, muốn %q", xa, xaA)
			}
			if nguoi.ID != maCanBo {
				t.Errorf("chủ thể vết = %q, muốn MÃ cán bộ %q — `audit_log.actor_id` giữ mã nghiệp "+
					"vụ, không bao giờ giữ id nội bộ (luật 6 bất biến 8)", nguoi.ID, maCanBo)
			}
			if nguoi.IP == "" {
				t.Error("vết không có địa chỉ IP — luật 6 bất biến 2 đòi ai · làm gì · từ IP nào")
			}
		})
	}
}

// --- (3) the three facts a client may never state -----------------------------------------------

func TestVanBan_TuChoiSoDoClientDat(t *testing.T) {
	// THE MOST EXPENSIVE FIELD IN THE SERVICE. A client that could name its own register number
	// could put a number already printed on a sealed, delivered document onto a second one — and
	// rule 7, invariant 3 does not permit taking either of them back.
	//
	// REFUSED, NOT IGNORED, on all four write routes of both registers: a clerk who watched the
	// number they typed disappear would believe the register had accepted it.
	for _, tc := range []struct {
		ten    string
		method string
		duong  string
		than   string
	}{
		{"vào sổ đến", http.MethodPost, duongVanBanDen,
			`{"received_date":"2026-09-22","issuing_body":"Huyện uỷ","document_type":"cong-van","summary":"x","number":7}`},
		{"sửa đến", http.MethodPatch, duongVanBanDen + "/vbd-a-001", `{"number":7}`},
		{"cấp số đi", http.MethodPost, duongVanBanDi,
			`{"document_date":"2026-09-22","document_type":"cong-van","summary":"x","recipient":"y","number":7}`},
		{"sửa đi", http.MethodPatch, duongVanBanDi + "/vbdi-a-001", `{"number":7}`},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuCoIdem(t)
			m.capQuyen(xaA, QuyenTaoVanBan)

			w := m.goiThan(t, tc.method, hostA, tc.duong, canBoCua(xaA), tc.than)
			doiMa(t, w, http.StatusBadRequest)
			if m.tongGoiVanBan() != 0 {
				t.Fatal("thân mang `number` mà use case vẫn chạy — số vào sổ / số đi là của sổ cấp")
			}
		})
	}
}

func TestVanBan_TuChoiTrangThaiVaHanDoClientDat(t *testing.T) {
	// `status` — the state moves by ROUTING and by nothing else on this surface. A client that could
	// set it could create a document already `da-giai-quyet`: a record closed by nobody, counted as
	// done in every figure the commune reports.
	//
	// `due_at` — the commitment is this commune's SLA and this commune's calendar, computed once by
	// identity (rule 10, invariant 2). A client choosing its own deadline is a client choosing how
	// long the authority may take.
	for ten, than := range map[string]string{
		"status": `{"received_date":"2026-09-22","issuing_body":"H","document_type":"cong-van","summary":"x","status":"da-giai-quyet"}`,
		"due_at": `{"received_date":"2026-09-22","issuing_body":"H","document_type":"cong-van","summary":"x","due_at":"2030-01-01T00:00:00Z"}`,
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuCoIdem(t)
			m.capQuyen(xaA, QuyenTaoVanBan)

			doiMa(t, m.goiThan(t, http.MethodPost, hostA, duongVanBanDen, canBoCua(xaA), than),
				http.StatusBadRequest)
			if m.tongGoiVanBan() != 0 {
				t.Fatalf("thân mang `%s` mà use case vẫn chạy", ten)
			}
		})
	}
}

// --- (5) an SLA nobody configured is a refusal, never an invented deadline -----------------------

func TestVanBanDen_ChuaCauHinhSLAThiTuChoiVaNoiRoManHinh(t *testing.T) {
	// THE ORDINARY ANSWER IN EVERY COMMUNE TODAY, and it is the contract working rather than a bug:
	// `sla` is empty everywhere (service-identity migration 0008 seeds nothing) and the onboarding
	// step that fills it does not exist in this repository.
	//
	// WHAT THIS PINS IS THAT NOTHING FALLS BACK. A booked document with a deadline invented by
	// software is a commitment reported upward as though the authority made it (rule 10, forbidden
	// #3), and it would look completely normal on the screen. 409 with the name of the screen that
	// fixes it is the honest answer.
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, QuyenTaoVanBan)
	m.ghiDen.loi = app.ErrChuaAnDinhDuocHan

	w := m.goiThan(t, http.MethodPost, hostA, duongVanBanDen, canBoCua(xaA), thanVaoSo)
	doiMa(t, w, http.StatusConflict)

	e := loiTra(t, w)
	if e.Code != "sla_chua_cau_hinh" {
		t.Fatalf("mã lỗi = %q, muốn \"sla_chua_cau_hinh\"", e.Code)
	}
	if !strings.Contains(e.Message, "Thời hạn xử lý") {
		t.Errorf("thông báo không chỉ ra màn hình cần sửa: %q", e.Message)
	}
}

func TestVanBanDen_LoaiVanBanKhongConDungThiTraLoi409(t *testing.T) {
	// 409 AND NOT 400: the value is well formed and the caller is allowed to send it. What is wrong
	// is this code against the state of the commune's catalogue — the type may have been retired
	// between the form loading and the clerk pressing save, and the sentence has to say so or the
	// clerk retypes the same value.
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, QuyenTaoVanBan)
	m.ghiDen.loi = docstore.ErrLoaiVanBanKhongDung

	w := m.goiThan(t, http.MethodPost, hostA, duongVanBanDen, canBoCua(xaA), thanVaoSo)
	doiMa(t, w, http.StatusConflict)
	if e := loiTra(t, w); e.Code != "document_type_unknown" {
		t.Fatalf("mã lỗi = %q, muốn \"document_type_unknown\"", e.Code)
	}
}

func TestVanBanDen_ChuyenVanBanDaKetThucThiTraLoi409(t *testing.T) {
	// A document the commune has settled is not routed further: doing so would reopen a closed
	// record as a side effect of a routing form. 409 and not 403 — the caller HOLDS `document.route`;
	// what is refused is this act on THIS document.
	m := dungMayChu(t)
	m.capQuyen(xaA, QuyenChuyenVanBan)
	m.ghiDen.loi = domain.ErrVanBanDaKetThuc

	w := m.goiThan(t, http.MethodPost, hostA, duongVanBanDen+"/vbd-a-001/routings", canBoCua(xaA),
		`{"to_unit":"bp-dia-chinh","reason":"x"}`)
	doiMa(t, w, http.StatusConflict)
	if e := loiTra(t, w); e.Code != "document_state" {
		t.Fatalf("mã lỗi = %q, muốn \"document_state\"", e.Code)
	}
}

func TestVanBanDen_ChuyenThieuLyDoThiTuChoi(t *testing.T) {
	// THE REASON IS MANDATORY, and this assertion is what stops that from being quietly relaxed: the
	// timeline entry can never be edited afterwards (rule 7, forbidden #5), so a routing with no
	// reason is an instruction nobody can ever account for.
	//
	// It is refused by the USE CASE rather than by the handler, so this drives the real refusal
	// through the fake — see the app tests for the refusal itself.
	m := dungMayChu(t)
	m.capQuyen(xaA, QuyenChuyenVanBan)
	m.ghiDen.loi = domain.ErrThieuLyDoChuyen

	doiMa(t, m.goiThan(t, http.MethodPost, hostA, duongVanBanDen+"/vbd-a-001/routings",
		canBoCua(xaA), `{"to_unit":"bp-dia-chinh"}`), http.StatusBadRequest)
}

// --- (4) one commune's register never reaches another -------------------------------------------

func TestVanBan_DanhSachChiTraSoCuaXaTrongURL(t *testing.T) {
	// THE SECOND WALL, and the one this package owns. The first is the permission check; this is the
	// commune the QUERY ran in. The fake reads it from the context exactly as *store.Scoped reads it,
	// so a handler that passed a commune from anywhere else — a query parameter, a field on the
	// principal — turns this red.
	for _, tc := range []struct {
		ten   string
		duong string
		khoa  string // a value that appears ONLY in this commune's fixture
		cam   string // and one that appears only in the OTHER commune's
	}{
		{"đến", duongVanBanDen, "UBND huyện", "Sở Nội vụ xã B"},
		{"đi", duongVanBanDi, "Trả lời đơn của công dân", "Báo cáo của xã B"},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, QuyenDocVanBan)

			w := m.goi(t, http.MethodGet, hostA, tc.duong, canBoCua(xaA))
			doiMa(t, w, http.StatusOK)

			than := w.Body.String()
			if !strings.Contains(than, tc.khoa) {
				t.Fatalf("danh sách thiếu dòng của chính xã: %s", than)
			}
			if strings.Contains(than, tc.cam) {
				t.Fatalf("SỔ CỦA XÃ KHÁC LỌT SANG — rò rỉ giữa hai cơ quan nhà nước: %s", than)
			}
		})
	}
}

func TestVanBan_DanhSachRongTraMangRongChuKhongPhaiNull(t *testing.T) {
	// `items: []` AND NEVER `null`. A commune onboarded this morning has an empty register, and a
	// client that has to handle both shapes handles one of them wrong — usually by rendering nothing
	// and saying nothing.
	m := dungMayChu(t)
	m.capQuyen(xaA, QuyenDocVanBan)
	m.den.theo = nil

	w := m.goi(t, http.MethodGet, hostA, duongVanBanDen, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	var ra page.Result[vanBanDenRa]
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if ra.Items == nil {
		t.Fatal("`items` là null — phải là []")
	}
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("`items` không phải mảng rỗng: %s", w.Body.String())
	}
}

// --- the filters -------------------------------------------------------------------------------

func TestVanBanDen_LocKhongHopLeThiTuChoiChuKhongBoQua(t *testing.T) {
	// A FILTER SILENTLY DROPPED IS THE FAILURE THIS GUARDS. The screen asked for one slice of the
	// register and would be handed the whole of it, with nothing saying so — which on this surface
	// means a clerk believing they are looking at every document of one kind.
	for ten, truyVan := range map[string]string{
		"trạng thái lạ":     "?status=khong-co-that",
		"năm không phải số": "?year=hai-nghin",
		"năm ngoài khoảng":  "?year=1999",
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, QuyenDocVanBan)

			doiMa(t, m.goi(t, http.MethodGet, hostA, duongVanBanDen+truyVan, canBoCua(xaA)),
				http.StatusBadRequest)
			if m.den.goi != 0 {
				t.Error("bộ lọc sai mà vẫn chạy câu truy vấn")
			}
		})
	}
}

func TestVanBanDen_LocHopLeDiXuongKhoNguyenVen(t *testing.T) {
	// The values reach the store AS VALUES. Nothing between the query string and the SQL assembles
	// text: they become bound parameters (see store.locThanhSQL), which is what keeps a government
	// register free of an injection point.
	m := dungMayChu(t)
	m.capQuyen(xaA, QuyenDocVanBan)

	doiMa(t, m.goi(t, http.MethodGet, hostA,
		duongVanBanDen+"?year=2026&status=da-phan-cong&document_type=cong-van&holding_unit=bp-01&q=hộ%20nghèo",
		canBoCua(xaA)), http.StatusOK)

	muon := docstore.LocVanBanDen{
		Nam: 2026, TrangThai: "da-phan-cong", LoaiVanBan: "cong-van",
		BoPhan: "bp-01", Tim: "hộ nghèo",
	}
	if m.den.locCuo != muon {
		t.Fatalf("bộ lọc xuống kho = %+v, muốn %+v", m.den.locCuo, muon)
	}
}

// --- the response shape --------------------------------------------------------------------------

func TestVanBanDen_TraVeSoDoSoCapChuKhongPhaiSoClientGui(t *testing.T) {
	// The body sent carries no number at all; the response carries 8. That is the register issuing
	// it, which is the only way a number is ever produced.
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, QuyenTaoVanBan)

	w := m.goiThan(t, http.MethodPost, hostA, duongVanBanDen, canBoCua(xaA), thanVaoSo)
	doiMa(t, w, http.StatusCreated)

	var ra vanBanDenRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if ra.Number != 8 || ra.Year != 2026 {
		t.Fatalf("số/năm trả về = %d/%d, muốn 8/2026", ra.Number, ra.Year)
	}
	if ra.Status != string(domain.VanBanMoiVaoSo) {
		t.Fatalf("trạng thái = %q, muốn %q", ra.Status, domain.VanBanMoiVaoSo)
	}
}

func TestVanBanDen_KhongCoTruongOverdueTrongPhanHoi(t *testing.T) {
	// `overdue` IS NOT ON THE WIRE, AND THIS TEST IS WHAT KEEPS IT OFF. A boolean beside the deadline
	// is a second representation of one fact, and the two disagree the moment a response is cached:
	// `overdue: false` next to a deadline that has since passed, with the screen believing the
	// boolean. Rule 10, invariant 3 — overdue is DERIVED, here by the client from `due_at`, and on
	// the server by domain.VanBanDen.QuaHan.
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, QuyenTaoVanBan)

	w := m.goiThan(t, http.MethodPost, hostA, duongVanBanDen, canBoCua(xaA), thanVaoSo)
	doiMa(t, w, http.StatusCreated)

	var than map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &than); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	for _, cam := range []string{"overdue", "qua_han", "is_overdue"} {
		if _, co := than[cam]; co {
			t.Fatalf("phản hồi mang trường %q — quá hạn là SUY RA từ `due_at`, không phải một giá "+
				"trị thứ hai đi trên dây (luật 10 bất biến 3)", cam)
		}
	}
	if _, co := than["due_at"]; !co {
		t.Fatal("phản hồi thiếu `due_at` — không có nó thì client không suy ra được quá hạn")
	}
}

// --- idempotency ----------------------------------------------------------------------------------

func TestVanBan_TuyenCapSoDoiIdempotencyKey(t *testing.T) {
	// THE TWO ROUTES THAT ISSUE A NUMBER DECLARE idem.Required. A double-submitted form without the
	// header must be refused BEFORE it reaches the handler: the second submission would take THE NEXT
	// NUMBER, and that is not a duplicate row somebody can remove — it is a second entry in an
	// archival register whose number can never be given back (rule 7, invariant 3).
	for ten, tc := range map[string]struct {
		duong string
		than  string
	}{
		"vào sổ đến": {duongVanBanDen, thanVaoSo},
		"cấp số đi":  {duongVanBanDi, thanCapSoDi},
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuCoIdem(t)
			m.capQuyen(xaA, QuyenTaoVanBan)

			// Built by hand: the harness always sets the header, which is what makes every other
			// assertion about the route rather than about the header.
			r := httptest.NewRequest(http.MethodPost, "https://"+hostA+tc.duong,
				strings.NewReader(tc.than))
			r.Host = hostA
			r.RemoteAddr = "10.0.0.7:51000"
			r.Header.Set("Content-Type", "application/json")
			r = r.WithContext(context.WithValue(r.Context(), khoaChuTheThu{}, *canBoCua(xaA)))
			w := httptest.NewRecorder()
			m.h.ServeHTTP(w, r)

			if w.Code != http.StatusBadRequest && w.Code != http.StatusPreconditionRequired {
				t.Fatalf("thiếu Idempotency-Key mà vẫn được %d — tuyến cấp số phải đòi khoá", w.Code)
			}
			if m.tongGoiVanBan() != 0 {
				t.Error("thiếu Idempotency-Key mà use case cấp số vẫn chạy")
			}
		})
	}
}

func TestVanBan_KhongCoRedisThiTuyenCapSoDong(t *testing.T) {
	// THE DongKhiHong DECLARATION, PINNED. With no idempotency store reachable, the two routes that
	// ISSUE A NUMBER answer 503 and write nothing — and that is the choice, not an accident.
	//
	// MoKhiHong would let a double-submitted form through while the cache was down, and the second
	// submission would take THE NEXT NUMBER. That is not a duplicate row somebody can remove: it is a
	// second entry in an archival register, and rule 7, invariant 3 does not allow the number back.
	// Refusing to book while the cache is down is the cheaper failure, and it is visible.
	//
	// THE OTHER ROUTES ARE NOT AFFECTED — they declare idem.KhongCan — so a cache outage costs the
	// commune the ability to book and issue, not the whole register.
	m := dungMayChu(t) // no store, on purpose
	m.capQuyen(xaA, QuyenTaoVanBan, QuyenDocVanBan, QuyenChuyenVanBan)

	for ten, tc := range map[string]struct {
		method, duong, than string
		muon                int
	}{
		"vào sổ đến": {http.MethodPost, duongVanBanDen, thanVaoSo, http.StatusServiceUnavailable},
		"cấp số đi":  {http.MethodPost, duongVanBanDi, thanCapSoDi, http.StatusServiceUnavailable},
		"sửa đến":    {http.MethodPatch, duongVanBanDen + "/vbd-a-001", `{"summary":"x"}`, http.StatusOK},
		"chuyển":     {http.MethodPost, duongVanBanDen + "/vbd-a-001/routings", `{"to_unit":"bp","reason":"x"}`, http.StatusOK},
		"đọc sổ đến": {http.MethodGet, duongVanBanDen, "", http.StatusOK},
	} {
		t.Run(ten, func(t *testing.T) {
			doiMa(t, m.goiThan(t, tc.method, hostA, tc.duong, canBoCua(xaA), tc.than), tc.muon)
		})
	}
}
