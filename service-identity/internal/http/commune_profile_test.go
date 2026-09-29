package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/tenant"
)

// GET /api/v1/commune-profiles?host= — the commune display profile, read from the platform for the
// commune the host resolved to.
//
// Rule 5 invariant 7 adapted to Public: no principal, so no 401/403; "right permission, wrong commune"
// becomes "commune A's host never returns commune B's profile".

// profileReaderFake answers by the commune IN THE CONTEXT — a handler that put the wrong commune there,
// or none, is caught by what comes back (or by the panic).
type profileReaderFake struct {
	byCommune map[tenant.ID]platformclient.TenantProfile
	err       error
	calls     int
}

func (f *profileReaderFake) TenantProfile(ctx context.Context) (platformclient.TenantProfile, bool, error) {
	f.calls++
	id := tenant.MustFrom(ctx)
	if f.err != nil {
		return platformclient.TenantProfile{}, false, f.err
	}
	p, ok := f.byCommune[id]
	return p, ok, nil
}

// twoProfiles: commune QR and commune B, each with a full profile INCLUDING a logo and an
// introduction, so the test proves those two never leave.
func twoProfiles() *profileReaderFake {
	return &profileReaderFake{byCommune: map[tenant.ID]platformclient.TenantProfile{
		xaQR: {OfficeAddress: "Thôn 1, xã Quế Sơn", LogoURL: "https://anh.example/logo-qr.png",
			Hotline: "0900000000", OfficeHoursText: "Thứ 2–6: 7h30–11h30", Introduction: "Giới thiệu xã QR"},
		xaB: {OfficeAddress: "Địa chỉ xã B", LogoURL: "https://anh.example/logo-b.png",
			Hotline: "0900000000", OfficeHoursText: "Giờ xã B", Introduction: "Giới thiệu xã B"},
	}}
}

func profileServer(t *testing.T, nt *nenTangXaGia, pr *profileReaderFake, log *slog.Logger) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Xa: nt, DanhBa: &danhBaCongKhaiGia{}, Profile: pr, Log: log})
	return mux
}

func readProfiles(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	return out.Items
}

func TestCommuneProfileShapeAndNoLogoNoIdNoIntroduction(t *testing.T) {
	h := profileServer(t, &nenTangXaGia{}, twoProfiles(), nil)
	w := goiCongKhai(h, CommuneProfilesPath, qHost(hostQR))
	doiMa(t, w, http.StatusOK)

	items := readProfiles(t, w)
	if len(items) != 1 {
		t.Fatalf("items = %v, muốn đúng một hồ sơ", items)
	}
	var keys []string
	for k := range items[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// EXACT key set: logo, introduction, id, host are each a new publication question.
	if got := strings.Join(keys, ","); got != "hotline,name,office_address,office_hours_text" {
		t.Fatalf("trường = %s", got)
	}
	if items[0]["name"] != "Xã Quế Sơn" || items[0]["office_address"] != "Thôn 1, xã Quế Sơn" ||
		items[0]["hotline"] != "0900000000" || items[0]["office_hours_text"] != "Thứ 2–6: 7h30–11h30" {
		t.Fatalf("hồ sơ = %v", items[0])
	}
	body := w.Body.String()
	for _, banned := range []string{string(xaQR), hostQR, "logo", "Giới thiệu", "anh.example"} {
		if strings.Contains(body, banned) {
			t.Fatalf("phản hồi chứa %q: %s", banned, body)
		}
	}
}

func TestCommuneProfileOtherCommuneIsolation(t *testing.T) {
	h := profileServer(t, &nenTangXaGia{}, twoProfiles(), nil)

	wa := goiCongKhai(h, CommuneProfilesPath, qHost(hostQR))
	if strings.Contains(wa.Body.String(), "xã B") {
		t.Fatalf("RÒ RỈ GIỮA HAI XÃ: tên miền xã QR trả hồ sơ xã B: %s", wa.Body.String())
	}
	wb := goiCongKhai(h, CommuneProfilesPath, qHost(hostXaB))
	doiMa(t, wb, http.StatusOK)
	items := readProfiles(t, wb)
	if len(items) != 1 || items[0]["name"] != "Xã B" || items[0]["office_address"] != "Địa chỉ xã B" {
		t.Fatalf("xã B nhận %v", items)
	}
	if strings.Contains(wb.Body.String(), "Quế Sơn") {
		t.Fatalf("RÒ RỈ GIỮA HAI XÃ: tên miền xã B trả hồ sơ xã QR: %s", wb.Body.String())
	}
}

func TestCommuneProfileUnknownHostIsSameAnswerAsCommunes(t *testing.T) {
	pr := twoProfiles()
	h := profileServer(t, &nenTangXaGia{}, pr, nil)
	for _, host := range []string{hostKhongCo, hostDanhRieng, hostNgung} {
		w := goiCongKhai(h, CommuneProfilesPath, qHost(host))
		c := goiDanhMucXa(h, qHost(host))
		doiMa(t, w, http.StatusOK)
		if w.Body.String() != c.Body.String() || strings.TrimSpace(w.Body.String()) != `{"items":[]}` {
			t.Fatalf("%s: hồ sơ trả %s, /communes trả %s — muốn cùng {\"items\":[]}", host, w.Body.String(), c.Body.String())
		}
	}
	if pr.calls != 0 {
		// hostNgung resolves to a real, inactive commune id: reading its profile would publish a merged
		// commune's office details under a domain that no longer serves it.
		t.Fatalf("hồ sơ bị đọc %d lần cho tên miền không thuộc xã đang hoạt động nào", pr.calls)
	}
}

func TestCommuneProfileNotDeclaredIsNameWithEmptyFields(t *testing.T) {
	h := profileServer(t, &nenTangXaGia{}, &profileReaderFake{}, nil)
	w := goiCongKhai(h, CommuneProfilesPath, qHost(hostQR))
	doiMa(t, w, http.StatusOK)
	items := readProfiles(t, w)
	if len(items) != 1 || items[0]["name"] != "Xã Quế Sơn" || items[0]["office_address"] != "" ||
		items[0]["hotline"] != "" || items[0]["office_hours_text"] != "" {
		t.Fatalf("xã chưa khai hồ sơ: %v — muốn tên xã và ba trường rỗng, không mặc định", items)
	}
}

func TestCommuneProfilePlatformDownIs503(t *testing.T) {
	// Registry down: the profile is never asked.
	pr := twoProfiles()
	w := goiCongKhai(profileServer(t, &nenTangXaGia{chet: true}, pr, nil), CommuneProfilesPath, qHost(hostQR))
	doiMa(t, w, http.StatusServiceUnavailable)
	if pr.calls != 0 || strings.Contains(w.Body.String(), "items") {
		t.Fatalf("nền tảng chết: hồ sơ bị đọc %d lần, thân %s", pr.calls, w.Body.String())
	}

	// Registry up, profile read down: 503 too — never an empty profile.
	var logBuf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logBuf, nil))
	w = goiCongKhai(profileServer(t, &nenTangXaGia{}, &profileReaderFake{err: errors.New("rpc error: code = Unavailable")}, log),
		CommuneProfilesPath, qHost(hostQR))
	doiMa(t, w, http.StatusServiceUnavailable)
	if strings.Contains(w.Body.String(), "items") || strings.Contains(w.Body.String(), "rpc error") {
		t.Fatalf("503 mang danh sách hoặc lộ chi tiết: %s", w.Body.String())
	}
	if logBuf.Len() == 0 {
		t.Fatal("hồ sơ không đọc được mà không để lại dòng nhật ký nào")
	}
}

func TestCommuneProfileMalformedHostIs400BeforeAnyCall(t *testing.T) {
	nt, pr := &nenTangXaGia{}, twoProfiles()
	h := profileServer(t, nt, pr, nil)
	for _, q := range hostSaiHinhDang {
		if w := goiCongKhai(h, CommuneProfilesPath, q); w.Code != http.StatusBadRequest {
			t.Errorf("%q: mã = %d, muốn 400", q, w.Code)
		}
	}
	if nt.goi != 0 || pr.calls != 0 {
		t.Fatalf("host sai hình dạng: nền tảng bị hỏi %d lần, hồ sơ bị đọc %d lần", nt.goi, pr.calls)
	}
}
