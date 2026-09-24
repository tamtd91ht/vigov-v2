package http

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// PUT /api/v1/staff/{id}/publication — open question #12. Rule 5, invariant 7's four cases, plus
// what the handler alone decides: that `published` is required, that the actor reaching the use
// case carries the STAFF CODE, that the refusal of a missing consent reaches the client as its own
// code, and that the reply carries the new fields in their own places.
//
// The consent rule itself — "nothing written without consent_confirmed" — is proven against a
// transaction in app/danh_ba_mini_app_test.go; re-asserting it through a fake use case here would
// assert that the fake returns what it was told to.

func tuyenCongKhai(than string) tuyenGhi {
	return tuyenGhi{"công khai Mini App", "PUT", "/api/v1/staff/" + idNoiBo + "/publication", than, http.StatusOK}
}

const thanCongKhaiDung = `{"published":true,"consent_confirmed":true,"display_order":5}`

// coContentUpdate rebuilds the chain with a checker granting `content.update` in commune A ONLY.
// Commune B has the same account and grants nothing — which is what makes the wrong-commune case
// a case about the commune.
func coContentUpdate(t *testing.T, m *mayChu) {
	t.Helper()
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("content.update"): true}},
			xaB: {},
		}}
	})
}

func TestCongKhai_401KhongToken(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)

	w := m.goiGhi(t, tuyenCongKhai(thanCongKhaiDung), hostA, "")
	doiMa(t, w, http.StatusUnauthorized)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case đã chạy %d lần", n)
	}
}

// 403 WITH THE WRONG PERMISSION — and the wrong permission chosen is `admin.user`, the key every
// OTHER staff write route declares. Managing accounts does not include deciding what the public
// sees (user decision 2026-09-24).
func TestCongKhai_403CoAdminUserMaKhongCoContentUpdate(t *testing.T) {
	m := dungMayChu(t) // the shipped default: admin.user in commune A, nothing else

	w := m.goiGhi(t, tuyenCongKhai(thanCongKhaiDung), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusForbidden)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("chỉ có admin.user mà use case công khai đã chạy %d lần", n)
	}
}

// 403 WITH THE RIGHT PERMISSION IN THE WRONG COMMUNE — same person, `content.update` in A, properly
// signed in at B. Same expectation as every other staff write route (can_bo_ghi_test.go).
func TestCongKhai_403DungQuyenSaiXa(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)

	w := m.goiGhi(t, tuyenCongKhai(thanCongKhaiDung), hostB, m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusForbidden)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("sai xã mà use case đã chạy %d lần", n)
	}
}

// 200 — and the body reaches the use case field by field, with the STAFF CODE as the audit actor.
//
// MUTATION THAT MUST TURN THIS RED: build audit.Actor from p.ID in nguoiThucHienCanBo.
func TestCongKhai_200VaChuyenDungTruong(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)

	w := m.goiGhi(t, tuyenCongKhai(thanCongKhaiDung), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	g := m.ghiDanhBa
	if g.idCuoi != idNoiBo || g.xaCuoi != xaA {
		t.Errorf("id/xã tới use case = %q/%q", g.idCuoi, g.xaCuoi)
	}
	if !g.congKhai.CongKhai || !g.congKhai.DaXacNhanDongY ||
		g.congKhai.ThuTu == nil || *g.congKhai.ThuTu != 5 {
		t.Errorf("thân không tới use case đúng: %+v", g.congKhai)
	}
	if g.nguoiCuoi.Vet.ID != maCanBo {
		t.Errorf("người ghi đồng ý / chủ thể vết = %q, muốn MÃ CÁN BỘ %q", g.nguoiCuoi.Vet.ID, maCanBo)
	}
	if g.nguoiCuoi.ID != idNoiBo {
		t.Errorf("định danh quyết định = %q, muốn %q", g.nguoiCuoi.ID, idNoiBo)
	}
}

// `published` IS REQUIRED. Absent is refused rather than read as false: an accidental unpublish
// clears the consent marks and the person must be asked again.
func TestCongKhaiThieuPublishedBiTuChoi(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)

	w := m.goiGhi(t, tuyenCongKhai(`{"consent_confirmed":true}`), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusBadRequest)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("thiếu published mà use case vẫn chạy %d lần", n)
	}
}

// display_order ABSENT reaches the use case as nil ("no explicit order"), never as 0.
func TestCongKhaiThieuDisplayOrderLaNil(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)

	doiMa(t, m.goiGhi(t, tuyenCongKhai(`{"published":false}`), hostA, m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if m.ghiDanhBa.congKhai.ThuTu != nil {
		t.Errorf("display_order vắng mặt mà tới use case là %v", *m.ghiDanhBa.congKhai.ThuTu)
	}
	if m.ghiDanhBa.congKhai.CongKhai {
		t.Error("published=false tới use case thành true")
	}
}

// The refusals of this route reach the client as the right status and code.
func TestCongKhaiAnhXaLoi(t *testing.T) {
	for _, c := range []struct {
		ten    string
		loi    error
		status int
		ma     string
	}{
		{"thiếu xác nhận đồng ý", app.ErrChuaXacNhanDongY, http.StatusBadRequest, "consent_required"},
		{"thứ tự âm", domain.ErrThuTuDanhBaAm, http.StatusBadRequest, "invalid_request"},
		{"người khác xã / đã xoá mềm", idstore.ErrCanBoKhongTonTai, http.StatusNotFound, "staff_not_found"},
	} {
		m := dungMayChu(t)
		coContentUpdate(t, m)
		m.ghiDanhBa.loi = c.loi

		w := m.goiGhi(t, tuyenCongKhai(`{"published":true}`), hostA, m.tokenCho(t, xaA, sidA))
		if w.Code != c.status {
			t.Errorf("%s: mã = %d, muốn %d — thân: %s", c.ten, w.Code, c.status, w.Body.String())
			continue
		}
		if e := loiTra(t, w); e.Code != c.ma {
			t.Errorf("%s: code = %q, muốn %q", c.ten, e.Code, c.ma)
		}
	}
}

// AN EMPTY STAFF CODE ON THE PRINCIPAL REFUSES THE WRITE (500) BEFORE THE USE CASE RUNS — it is
// never replaced by the internal id (rule 6, invariant 8).
func TestCongKhaiMaCanBoRongThiTuChoi(t *testing.T) {
	m := dungMayChu(t)
	cb := canBoMau()
	cb.Ma = ""
	m.canBo.theo[idNoiBo] = cb
	coContentUpdate(t, m)

	w := m.goiGhi(t, tuyenCongKhai(thanCongKhaiDung), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("mã cán bộ rỗng mà use case vẫn chạy %d lần", n)
	}
}

// THE REPLY CARRIES THE NEW FIELDS IN THEIR OWN PLACES. has_zalo and published are given OPPOSITE
// values, so a swap in raNgoai — which would show "published" for somebody who merely has Zalo —
// cannot pass. display_order 0 checks that a real position does not become null.
func TestCongKhaiPhanHoiMangTruongMoiDungCho(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)
	luc := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	khong := 0
	m.ghiDanhBa.kq.CoZalo = false
	m.ghiDanhBa.kq.HienTrenMiniApp = true
	m.ghiDanhBa.kq.ThuTuDanhBa = &khong
	m.ghiDanhBa.kq.DongYCongKhaiLuc = &luc
	m.ghiDanhBa.kq.DongYCongKhaiGhiBoi = "CB-2026-GHI001"

	w := m.goiGhi(t, tuyenCongKhai(thanCongKhaiDung), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var ra map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	if ra["has_zalo"] != false || ra["published"] != true {
		t.Errorf("has_zalo=%v published=%v, muốn false/true — hai trường bị hoán đổi", ra["has_zalo"], ra["published"])
	}
	if ra["display_order"] != float64(0) {
		t.Errorf("display_order = %v, muốn 0", ra["display_order"])
	}
	if ra["consent_recorded_at"] != "2026-09-24T08:00:00Z" {
		t.Errorf("consent_recorded_at = %v", ra["consent_recorded_at"])
	}
	// #11 unchanged: the mobile stays unmasked on this internal surface.
	if ra["mobile"] != "0900000009" {
		t.Errorf("mobile = %v, muốn số đầy đủ (câu #11)", ra["mobile"])
	}
}

// Null on both nullable fields when there is no order and no consent.
func TestPhanHoiTruongMiniAppNullKhiKhongCo(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)

	w := m.goiGhi(t, tuyenCongKhai(`{"published":false}`), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	var ra map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &ra)
	for _, k := range []string{"display_order", "consent_recorded_at"} {
		v, co := ra[k]
		if !co || v != nil {
			t.Errorf("%s = %v (có=%v), muốn null", k, v, co)
		}
	}
}

// --- PATCH has_zalo -------------------------------------------------------------------------------

func TestSuaCanBoHasZalo(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	tg := moiTuyenGhi()[1]
	tg.than = `{"has_zalo":true}`
	doiMa(t, m.goiGhi(t, tg, hostA, tok), http.StatusOK)
	if z := m.ghiDanhBa.suaCuoi.CoZalo; z == nil || !*z {
		t.Errorf("has_zalo=true không tới use case: %v", z)
	}

	tg.than = `{"has_zalo":false}`
	doiMa(t, m.goiGhi(t, tg, hostA, tok), http.StatusOK)
	if z := m.ghiDanhBa.suaCuoi.CoZalo; z == nil || *z {
		t.Errorf("has_zalo=false phải tới use case là false, không phải vắng: %v", z)
	}

	tg.than = `{"position":"Chuyên viên"}`
	doiMa(t, m.goiGhi(t, tg, hostA, tok), http.StatusOK)
	if z := m.ghiDanhBa.suaCuoi.CoZalo; z != nil {
		t.Errorf("has_zalo không gửi mà tới use case là %v — sẽ ghi đè", *z)
	}
}
