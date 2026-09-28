package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The four rule-5 cases of PUT …/publication are rows of caCacTuyen (xu_ly_phan_anh_test.go). This
// file proves what is particular to the route: the refusal mapping, the fields on the response, and
// that the principal reaches the use case as its business code with the restricted fact.

func publicationPath(code string) string { return duong(code) + "/publication" }

func TestPublicationResponseCarriesStatusAndDerivedPublic(t *testing.T) {
	for _, tc := range []struct {
		status string
		public bool
	}{{"cong-khai", true}, {"an", false}} {
		m := dungMayChu(t)
		m.capQuyen(t, "feedback.assign")

		w := m.goiThan(t, http.MethodPut, hostA, publicationPath(maPhieuThuong), canBoCuaXa(xaA),
			publicationIn{Status: tc.status})
		doiMa(t, w, http.StatusOK)

		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("giải mã: %v", err)
		}
		if body["publication_status"] != tc.status || body["public"] != tc.public {
			t.Errorf("%s: publication_status=%v public=%v, muốn %s/%v", tc.status,
				body["publication_status"], body["public"], tc.status, tc.public)
		}
		if m.xuLy.viec != "cong-khai" || m.xuLy.publication != tc.status || m.xuLy.nguoi.ID != maCanBo {
			t.Errorf("use case nhận việc=%q trạng thái=%q người=%q", m.xuLy.viec, m.xuLy.publication,
				m.xuLy.nguoi.ID)
		}
	}
}

func TestPublicationRefusalMapping(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code int
		key  string
		text string
	}{
		{"lĩnh vực tác phong cán bộ", fmt.Errorf("bọc xã 01JX: %w", domain.ErrNeverPublic),
			// The shipped default of `feedback.never_public` — the harness's fake has no override.
			http.StatusConflict, "never_public", "Phản ánh về thái độ, tác phong cán bộ không được hiển thị công khai."},
		{"giá trị sai", domain.ErrPublicationStatusInvalid, http.StatusBadRequest, "invalid_request", "cong-khai"},
		{"phiếu hạn chế", fmt.Errorf("bọc: %w", app.ErrPhieuHanChe), http.StatusNotFound, "", ""},
		{"không có phiếu", petstore.ErrPhieuKhongTonTai, http.StatusNotFound, "", ""},
		{"đua ghi", petstore.ErrPhieuDaChuyenTrang, http.StatusConflict, "petition_state", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "feedback.assign")
			m.xuLy.loi = tc.err

			w := m.goiThan(t, http.MethodPut, hostA, publicationPath(maPhieuThuong), canBoCuaXa(xaA),
				publicationIn{Status: "cong-khai"})
			doiMa(t, w, tc.code)
			if tc.key != "" && !strings.Contains(w.Body.String(), `"`+tc.key+`"`) {
				t.Errorf("thiếu mã lỗi %q: %s", tc.key, w.Body.String())
			}
			if tc.text != "" && !strings.Contains(w.Body.String(), tc.text) {
				t.Errorf("thiếu câu cố định %q: %s", tc.text, w.Body.String())
			}
			// Never the wrapped chain: it carries the commune id.
			if strings.Contains(w.Body.String(), "01JX") || strings.Contains(w.Body.String(), "phan_anh:") {
				t.Errorf("chuỗi lỗi nội bộ lọt ra dây: %s", w.Body.String())
			}
		})
	}
}

// The restricted fact is read from `feedback.restricted` and handed down, as on every write route.
func TestPublicationHandsDownRestrictedFact(t *testing.T) {
	for _, hold := range []bool{false, true} {
		m := dungMayChu(t)
		keys := []authz.Perm{"feedback.assign"}
		if hold {
			keys = append(keys, QuyenHanChe)
		}
		m.capQuyen(t, keys...)
		w := m.goiThan(t, http.MethodPut, hostA, publicationPath(maPhieuThuong), canBoCuaXa(xaA),
			publicationIn{Status: "an"})
		doiMa(t, w, http.StatusOK)
		if bool(m.xuLy.hanChe) != hold {
			t.Errorf("giữ khoá hạn chế=%v nhưng use case nhận %v", hold, m.xuLy.hanChe)
		}
	}
}
