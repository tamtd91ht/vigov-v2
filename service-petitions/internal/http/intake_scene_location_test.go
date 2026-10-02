package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
)

// The optional scene location (ADR 0050) on the citizen intake and on both read surfaces.
//
//	PROVED HERE   `lat`/`lng` are accepted, reach the use case, come back on the 201 and on the
//	              citizen's own GET · not sent means the keys are ABSENT, not null or 0 · one alone,
//	              out of range and non-number are 400 and create no petition · the intake log line
//	              carries neither coordinate · the staff read shows them under `feedback.read`, on an
//	              anonymous petition too (as `address` is), and omits them when there are none.
//	NOT PROVED    that the row and trail hold them correctly — internal/app and internal/store own
//	              that against their own fakes.

const bodyWithSceneLocation = `{"content":"Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",` +
	`"address":"Đầu ngõ thôn Hà Lam","lat":16.05441249,"lng":108.2022771}`

// intakeServerWithLog is dungMayChuGui with the log captured, so a test can read every line the
// route wrote.
func intakeServerWithLog(t *testing.T) (*mayChuGui, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	m := dungMayChuGui(t)

	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu: m.so, GuiPhieu: m.so, Rating: newRatingFake(), NhanLinhVuc: nhanLinhVucMau(), CitizenFields: newFieldCatalogueFake(), Photos: newCitizenPhotosFake(), PhotoLimiter: photoLimiterThu(), Log: log,
	})
	var h http.Handler = mux
	h = idem.Middleware(m.kho, log)(h)
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(&soPhienCongDanGia{})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	m.h = httpx.StripTenantHeaders(h)
	return m, &buf
}

func TestIntakeAcceptsSceneLocation(t *testing.T) {
	m := dungMayChuGui(t)

	w := m.gui(t, bodyWithSceneLocation, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)

	if yc := m.so.thayYeuCau[0]; yc.Lat == nil || *yc.Lat != 16.05441249 || yc.Lng == nil || *yc.Lng != 108.2022771 {
		t.Errorf("use case nhận toạ độ %v/%v, muốn đúng số công dân gửi", yc.Lat, yc.Lng)
	}
	ra := docPhieuCuaToi(t, w.Body.Bytes())
	if ra.Lat == nil || *ra.Lat != 16.054412 || ra.Lng == nil || *ra.Lng != 108.202277 {
		t.Errorf("201 trả toạ độ %v/%v, muốn 16.054412/108.202277 (đã làm tròn 6 chữ số)", ra.Lat, ra.Lng)
	}

	// The citizen's own GET echoes what they sent, like `address`.
	doc := m.doc(t, duongTapCongDan+"/"+ra.Code, tokenCuaToi)
	doiMa(t, doc, http.StatusOK)
	if lai := docPhieuCuaToi(t, doc.Body.Bytes()); lai.Lat == nil || *lai.Lat != *ra.Lat || *lai.Lng != *ra.Lng {
		t.Errorf("GET của công dân trả toạ độ %v/%v, khác 201", lai.Lat, lai.Lng)
	}
}

func TestIntakeWithoutSceneLocationOmitsKeys(t *testing.T) {
	m := dungMayChuGui(t)

	w := m.gui(t, thanThu, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	for _, key := range []string{"lat", "lng"} {
		if _, has := raw[key]; has {
			t.Errorf("khoá %q có mặt dù công dân không gửi vị trí — một ghim bịa ra: %s", key, w.Body.String())
		}
	}
}

func TestIntakeRefusesMalformedSceneLocation(t *testing.T) {
	const pre = `{"content":"Đống rác ở đầu ngõ.",`
	for name, c := range map[string]struct {
		body        string
		reachesCase bool // decoded fine, refused by the domain rule the use case applies
	}{
		"chỉ lat":         {pre + `"lat":16.05}`, true},
		"chỉ lng":         {pre + `"lng":108.2}`, true},
		"lat null lng số": {pre + `"lat":null,"lng":108.2}`, true},
		"lat > 90":        {pre + `"lat":91,"lng":108.2}`, true},
		"lng < -180":      {pre + `"lat":16.05,"lng":-181}`, true},
		"lat là chuỗi":    {pre + `"lat":"16.05","lng":108.2}`, false},
		"lng là bool":     {pre + `"lat":16.05,"lng":true}`, false},
		"lat tràn float":  {pre + `"lat":1e400,"lng":108.2}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuGui(t)
			w := m.gui(t, c.body, tokenCuaToi, khoaThu)
			doiMa(t, w, http.StatusBadRequest)
			if m.so.dem != 0 {
				t.Errorf("đã tạo %d phiếu dù vị trí sai hình dạng", m.so.dem)
			}
			if !c.reachesCase && m.so.demGui != 0 {
				t.Errorf("use case bị gọi %d lần cho một thân không giải mã được", m.so.demGui)
			}
			for _, leak := range []string{"16.05", "108.2", "1e400"} {
				if strings.Contains(w.Body.String(), leak) {
					t.Errorf("câu trả lời lặp lại toạ độ đã gửi (%q): %s", leak, w.Body.String())
				}
			}
		})
	}
}

func TestIntakeLogCarriesNoCoordinates(t *testing.T) {
	m, logged := intakeServerWithLog(t)

	w := m.gui(t, bodyWithSceneLocation, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)

	if !strings.Contains(logged.String(), "ma_tra_cuu") {
		t.Fatalf("không bắt được dòng log tiếp nhận — phép kiểm dưới đây sẽ xanh vì trống: %q", logged.String())
	}
	for _, leak := range []string{"16.05", "108.20", "16054", "108202"} {
		if strings.Contains(logged.String(), leak) {
			t.Errorf("log mang toạ độ (%q) — luật 3: %s", leak, logged.String())
		}
	}
}

// --- staff read --------------------------------------------------------------------------------

func withSceneLocation(m *mayChu, xa tenant.ID, code string, lat, lng float64) {
	p := m.phieu.theo[xa][code]
	p.Lat, p.Lng = &lat, &lng
	m.phieu.theo[xa][code] = p
}

func TestStaffReadShowsSceneLocation(t *testing.T) {
	m := dungMayChu(t) // grants feedback.read
	withSceneLocation(m, xaA, maPhieuThuong, 16.0544, 108.2022)

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docPhieu(t, w.Body.Bytes())
	if ra.Lat == nil || *ra.Lat != 16.0544 || ra.Lng == nil || *ra.Lng != 108.2022 {
		t.Errorf("lat/lng = %v/%v, muốn 16.0544/108.2022", ra.Lat, ra.Lng)
	}
}

func TestStaffReadOmitsSceneLocationWhenAbsent(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	for _, key := range []string{"lat", "lng"} {
		if _, has := raw[key]; has {
			t.Errorf("khoá %q có mặt trên phiếu không có vị trí: %s", key, w.Body.String())
		}
	}
}

// TestStaffReadShowsSceneLocationOnAnonymous: anonymity hides WHO filed it, not WHERE the problem
// is — the same treatment `address` gets. The reporter stays hidden beside it.
func TestStaffReadShowsSceneLocationOnAnonymous(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {
				authz.Perm("feedback.read"):       true,
				authz.Perm("feedback.restricted"): true,
			}},
		}}
	})
	withSceneLocation(m, xaA, maPhieuCanBo, 16.0544, 108.2022)

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuCanBo), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docPhieu(t, w.Body.Bytes())
	if !ra.Anonymous || ra.ReporterName != "" || ra.ReporterPhone != "" {
		t.Fatalf("fixture không còn là phiếu ẩn danh ẩn người gửi: %+v", ra)
	}
	if ra.Lat == nil || ra.Lng == nil {
		t.Errorf("phiếu ẩn danh mất vị trí hiện trường — `address` vẫn hiện, vị trí phải cùng cách: %s", w.Body.String())
	}
}
