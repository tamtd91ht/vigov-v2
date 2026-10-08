package http

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The HTTP half of ADR 0050 §Sửa đổi 08/10/2026 (attach the session's verified phone when the box is
// empty). The decision of WHEN to ask lives in the use case (internal/app/contact_phone_test.go); this
// file proves the sid reaching it is the SESSION's and the two new refusals map to 401 / 503 with no code.

// A body naming another session cannot steer it: the sid handed down is the one the edge resolved.
func TestIntakeSessionIDComesFromSession(t *testing.T) {
	m := dungMayChuGui(t)

	body := `{"content":"Đống rác ở đầu ngõ.","session_id":"sid-NGUOI-KHAC","sid":"sid-NGUOI-KHAC"}`
	w := m.guiDuong(t, duongTapCongDan+"?session_id=sid-NGUOI-KHAC", body, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)

	if len(m.so.thaySession) != 1 || m.so.thaySession[0] != "sid-1" {
		t.Errorf("sid handed to the use case = %v, want the session's own %q (rule 4, invariant 2)",
			m.so.thaySession, "sid-1")
	}
}

func TestIntakeContactPhoneRefusals(t *testing.T) {
	cases := map[string]struct {
		err      error
		wantCode int
		wantKey  string
	}{
		"no session -> 401": {app.ErrContactPhoneNoSession, http.StatusUnauthorized, `"unauthorized"`},
		"identity down -> 503": {
			fmt.Errorf("%w cho xã %s: rpc error: code = Unavailable", app.ErrContactPhoneUnavailable, xaA),
			http.StatusServiceUnavailable, `"contact_phone_unavailable"`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuGui(t)
			m.so.loi = c.err

			w := m.gui(t, `{"content":"Đống rác ở đầu ngõ."}`, tokenCuaToi, khoaThu)
			doiMa(t, w, c.wantCode)

			body := w.Body.String()
			if !strings.Contains(body, c.wantKey) {
				t.Errorf("body = %s, want error key %s", body, c.wantKey)
			}
			if strings.Contains(body, domain.TienToMaTraCuu+"-") {
				t.Errorf("a lookup code left a refused intake: %s", body)
			}
			if len(m.so.theo[xaA]) != 0 {
				t.Errorf("stored %d petitions on a refused intake", len(m.so.theo[xaA]))
			}
			for _, internal := range []string{"identity", "rpc", "Unavailable", "sid-1"} {
				if strings.Contains(body, internal) {
					t.Errorf("internal detail %q in the citizen's body: %s", internal, body)
				}
			}
		})
	}
}
