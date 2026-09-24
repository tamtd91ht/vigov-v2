package http

import (
	"net/http"
	"testing"
)

// THE CONSENT FLAG REACHES THE USE CASE AS THE CLIENT SENT IT — and absent or false means false.
//
// app/danh_ba_mini_app_test.go proves the use case refuses `CongKhai && !DaXacNhanDongY`. That proof
// is only worth something if the handler hands it the real flag. Every existing HTTP case sends
// `consent_confirmed: true`, so a handler that hard-codes the flag, or derives it from `published`,
// passed them all — and would put a personal mobile on the public Mini App with no consent recorded
// by anybody (#12, Decree 13/2023), while the use-case test stayed green.
//
// MUTATIONS THAT MUST TURN THIS RED (both verified 2026-09-24 to leave the suite green before this
// test existed):
//   - `DaXacNhanDongY: true,` in DatCongKhaiCanBo
//   - `DaXacNhanDongY: than.ConsentConfirmed || *than.Published,`
func TestCongKhaiCoXacNhanDongYToiUseCaseDungNhuClientGui(t *testing.T) {
	for _, c := range []struct {
		ten, than string
	}{
		{"vắng consent_confirmed", `{"published":true}`},
		{"consent_confirmed false", `{"published":true,"consent_confirmed":false}`},
		{"consent_confirmed null", `{"published":true,"consent_confirmed":null}`},
	} {
		m := dungMayChu(t)
		coContentUpdate(t, m)

		doiMa(t, m.goiGhi(t, tuyenCongKhai(c.than), hostA, m.tokenCho(t, xaA, sidA)), http.StatusOK)
		g := m.ghiDanhBa
		if g.soLanGoi() != 1 {
			t.Fatalf("%s: use case chạy %d lần, muốn 1 — bài kiểm không đọc được gì", c.ten, g.soLanGoi())
		}
		if !g.congKhai.CongKhai {
			t.Errorf("%s: published=true tới use case thành false — ca này không còn kiểm cổng đồng ý", c.ten)
		}
		if g.congKhai.DaXacNhanDongY {
			t.Errorf("%s: use case nhận DaXacNhanDongY = true dù client KHÔNG xác nhận — "+
				"công khai số di động không có đồng ý (#12)", c.ten)
		}
	}

	// And a string is not a confirmation: the decoder refuses it before the use case runs.
	m := dungMayChu(t)
	coContentUpdate(t, m)
	doiMa(t, m.goiGhi(t, tuyenCongKhai(`{"published":true,"consent_confirmed":"true"}`), hostA,
		m.tokenCho(t, xaA, sidA)), http.StatusBadRequest)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("consent_confirmed dạng chuỗi mà use case vẫn chạy %d lần", n)
	}
}
