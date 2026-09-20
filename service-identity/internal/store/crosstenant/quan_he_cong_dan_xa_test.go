package crosstenant

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Tests that need no database, for the one cross-commune READ this package is allowed to hold
// on quan_he_cong_dan_xa. The pg suite next door SKIPS without VIGOV_TEST_DSN while the package
// still prints `ok`.

const (
	congDanCT = "01JCONGDAN0000000000000009"
	xaMotCT   = "01J0000000000000000000000E"
	xaHaiCT   = "01J0000000000000000000000F"
)

var mocKhaiCT = time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)

// dongXa is one sample row. The two facts are set to a combination no merged column could
// produce — a declaration saying one thing, a review saying another — and ly_do_tu_choi is the
// prose the citizen reads.
func dongXa(xa string) map[string]driver.Value {
	return map[string]driver.Value{
		"tenant_id":           xa,
		"khai_cu_tru":         "thuong_tru",
		"nguon_khai":          "cong_dan",
		"khai_luc":            mocKhaiCT,
		"trang_thai_xac_thuc": "tu_choi",
		"ly_do_tu_choi":       "Địa chỉ khai không thuộc địa bàn xã.",
	}
}

func phienCongDan(xa string) httpx.CitizenSession {
	return httpx.CitizenSession{ID: "SID-CT", CitizenID: congDanCT, TenantID: tenant.ID(xa)}
}

// --- rule 4: the citizen, and where it comes from ------------------------------------------------

func TestXaCuaCongDanKhoaTheoCongDanCuaPhien(t *testing.T) {
	// RULE 4, INVARIANT 2 AND THE WHOLE JUSTIFICATION OF THIS QUERY. It is keyed on ONE citizen
	// identity taken from the session the server issued — which is what makes it "a citizen
	// reading their own rows across communes" (ADR 0002) rather than the rollup doc.go forbids
	// while open question #4 is open. There is no signature here that takes an id.
	k := &khoCT{hang: []map[string]driver.Value{dongXa(xaMotCT)}}

	if _, err := NewQuanHeStore(dbGiaCT(k)).
		XaCuaCongDanTrongPhien(ctxKhamPha(t, phienCongDan(""))); err != nil {
		t.Fatalf("XaCuaCongDanTrongPhien lỗi: %v", err)
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "WHERE cong_dan_id = $1") {
		t.Errorf("câu lệnh không khoá theo công dân: %q", l.sql)
	}
	if len(l.args) < 1 || l.args[0] != congDanCT {
		t.Fatalf("$1 = %v, muốn công dân của phiên %q", l.args, congDanCT)
	}
}

func TestXaCuaCongDanKhongCoPhienThiTuChoiVaKhongChayCauLenhNao(t *testing.T) {
	// FAIL CLOSED. Without a session there is no honest way to name the citizen, and every
	// fallback anyone would reach for takes the identity from something the client sent — which
	// on THIS query would list another person's communes.
	k := &khoCT{}

	_, err := NewQuanHeStore(dbGiaCT(k)).XaCuaCongDanTrongPhien(context.Background())
	if !errors.Is(err, ErrThieuPhienCongDan) {
		t.Fatalf("lỗi = %v, muốn ErrThieuPhienCongDan", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("không có phiên mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

func TestXaCuaCongDanChayDuocKhiContextChuaCoXa(t *testing.T) {
	// THE POINT OF THE WHOLE PACKAGE, stated as a test: this read runs at the discovery layer,
	// where the citizen has signed in and has NOT chosen a commune (ADR 0005). A scoped
	// repository would panic here — core/store.For(ctx) requires a commune — so asking "which
	// communes am I known to" would require already knowing one.
	k := &khoCT{hang: []map[string]driver.Value{dongXa(xaMotCT), dongXa(xaHaiCT)}}

	ra, err := NewQuanHeStore(dbGiaCT(k)).
		XaCuaCongDanTrongPhien(ctxKhamPha(t, phienCongDan("")))
	if err != nil {
		t.Fatalf("XaCuaCongDanTrongPhien lỗi: %v", err)
	}
	if len(ra) != 2 {
		t.Fatalf("nhận %d xã, muốn 2", len(ra))
	}
	if ra[0].XaID != tenant.ID(xaMotCT) || ra[1].XaID != tenant.ID(xaHaiCT) {
		t.Errorf("mã xã đọc sai chỗ: %v, %v", ra[0].XaID, ra[1].XaID)
	}
}

// --- what the citizen may and may not see ----------------------------------------------------

func TestXaCuaCongDanKhongDocCanBoDaXetDuyet(t *testing.T) {
	// RULE 4, FORBIDDEN #5: internal fields do not cross into the citizen channel. The officer
	// who reviewed a declaration is routing information about the commune's own work.
	//
	// THE COLUMN IS NOT SELECTED AT ALL, which is stronger than dropping it in a handler: a
	// value that never leaves the database cannot be leaked by a DTO somebody added later.
	k := &khoCT{hang: []map[string]driver.Value{dongXa(xaMotCT)}}

	if _, err := NewQuanHeStore(dbGiaCT(k)).
		XaCuaCongDanTrongPhien(ctxKhamPha(t, phienCongDan(""))); err != nil {
		t.Fatalf("XaCuaCongDanTrongPhien lỗi: %v", err)
	}
	if strings.Contains(k.cuoi().sql, "xet_duyet_boi") {
		t.Errorf("câu lệnh đọc cả cán bộ xét duyệt — dữ liệu nội bộ ra kênh công dân: %q", k.cuoi().sql)
	}
	// ly_do_tu_choi IS the other half of the same decision: it is prose an officer wrote FOR
	// THE CITIZEN TO READ (ADR 0023 §A2). Without it a citizen cannot tell "nobody has looked
	// at this yet" from "this was refused".
	if !strings.Contains(k.cuoi().sql, "ly_do_tu_choi") {
		t.Errorf("câu lệnh bỏ mất lý do từ chối — công dân không đọc được vì sao: %q", k.cuoi().sql)
	}
}

func TestXaCuaCongDanLuonMangCaHaiSuThat(t *testing.T) {
	// The declaration and the verification state travel together, always. A screen that showed
	// "thường trú" without saying whether anyone had checked it is the merge the whole table
	// exists to avoid, performed on the citizen's own screen this time.
	k := &khoCT{hang: []map[string]driver.Value{dongXa(xaMotCT)}}

	ra, err := NewQuanHeStore(dbGiaCT(k)).
		XaCuaCongDanTrongPhien(ctxKhamPha(t, phienCongDan("")))
	if err != nil {
		t.Fatalf("XaCuaCongDanTrongPhien lỗi: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("nhận %d dòng, muốn 1", len(ra))
	}
	q := ra[0]
	if q.Khai != domain.KhaiThuongTru {
		t.Errorf("Khai = %q, muốn thuong_tru", q.Khai)
	}
	if q.TrangThai != domain.TuChoi {
		t.Errorf("TrangThai = %q, muốn tu_choi", q.TrangThai)
	}
	if q.NguonKhai != domain.NguonKhaiCongDan {
		t.Errorf("NguonKhai = %q, muốn cong_dan", q.NguonKhai)
	}
	if !q.KhaiLuc.Equal(mocKhaiCT) {
		t.Errorf("KhaiLuc = %v, muốn %v", q.KhaiLuc, mocKhaiCT)
	}
	// tenant_id and ly_do_tu_choi are the two plain strings in this row: a swapped Scan would
	// show the citizen a commune id where the refusal reason belongs, and file the reason as a
	// commune. Nothing but the column names can catch it.
	if !strings.HasPrefix(q.LyDoTuChoi, "Địa chỉ khai") {
		t.Errorf("LyDoTuChoi = %q — hai cột TEXT đọc ngược?", q.LyDoTuChoi)
	}
	if string(q.XaID) != xaMotCT {
		t.Errorf("XaID = %q — hai cột TEXT đọc ngược?", q.XaID)
	}
}

// --- rule 7, ordering and the bound -------------------------------------------------------------

func TestXaCuaCongDanLocDongDaXoaMemVaSapXepOnDinh(t *testing.T) {
	k := &khoCT{hang: []map[string]driver.Value{dongXa(xaMotCT)}}

	if _, err := NewQuanHeStore(dbGiaCT(k)).
		XaCuaCongDanTrongPhien(ctxKhamPha(t, phienCongDan(""))); err != nil {
		t.Fatalf("XaCuaCongDanTrongPhien lỗi: %v", err)
	}
	q := k.cuoi()
	// RULE 7, INVARIANT 2 — and the partial index behind this query carries the same predicate,
	// so dropping it would also stop using the index.
	if !strings.Contains(q.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", q.sql)
	}
	// TOTAL ORDER: most recent declaration first, tenant_id breaks ties. The pair is unique per
	// citizen, so the citizen's own list cannot reshuffle between two reads.
	if !strings.Contains(q.sql, "ORDER BY khai_luc DESC, tenant_id") {
		t.Errorf("thứ tự không ổn định: %q", q.sql)
	}
	if len(q.args) < 2 || q.args[1] != int64(TranXaCuaCongDan+1) {
		t.Errorf("LIMIT = %v, muốn %d (trần + 1)", q.args[1:], TranXaCuaCongDan+1)
	}
}

func TestXaCuaCongDanDungTranThiVanTraDu(t *testing.T) {
	// Exactly at the ceiling is a COMPLETE list. An off-by-one here refuses a citizen whose data
	// is perfectly valid, on the screen they open the app to see.
	k := &khoCT{hang: nhieuDongXa(TranXaCuaCongDan)}

	ra, err := NewQuanHeStore(dbGiaCT(k)).
		XaCuaCongDanTrongPhien(ctxKhamPha(t, phienCongDan("")))
	if err != nil {
		t.Fatalf("đúng trần mà bị từ chối: %v", err)
	}
	if len(ra) != TranXaCuaCongDan {
		t.Errorf("nhận %d dòng, muốn %d", len(ra), TranXaCuaCongDan)
	}
}

func TestXaCuaCongDanVuotTranThiTuChoiVaKhongTraDongNao(t *testing.T) {
	// REFUSE, DO NOT TRUNCATE — and the rows already read are DROPPED. A silently short list is
	// a commune missing from the citizen's own screen, where they have filed something and now
	// cannot find it. Handing back a list the caller might render anyway is how a refusal turns
	// back into a silent truncation, one careless `if err != nil { log }` later.
	k := &khoCT{hang: nhieuDongXa(TranXaCuaCongDan + 1)}

	ra, err := NewQuanHeStore(dbGiaCT(k)).
		XaCuaCongDanTrongPhien(ctxKhamPha(t, phienCongDan("")))
	if !errors.Is(err, ErrQuaNhieuXa) {
		t.Fatalf("lỗi = %v, muốn ErrQuaNhieuXa", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ra))
	}
}

func nhieuDongXa(n int) []map[string]driver.Value {
	ra := make([]map[string]driver.Value, 0, n)
	for i := 0; i < n; i++ {
		d := dongXa(xaMotCT)
		d["tenant_id"] = xaMotCT[:len(xaMotCT)-2] + string(rune('A'+i%26)) + string(rune('A'+(i/26)%26))
		d["khai_luc"] = mocKhaiCT.Add(time.Duration(i) * time.Minute)
		ra = append(ra, d)
	}
	return ra
}

// --- failures ------------------------------------------------------------------------------------

func TestXaCuaCongDanLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := &khoCT{loi: goc}

	ra, err := NewQuanHeStore(dbGiaCT(k)).
		XaCuaCongDanTrongPhien(ctxKhamPha(t, phienCongDan("")))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrQuaNhieuXa) {
		t.Error("lỗi kho bị nhận nhầm là vượt trần")
	}
	if ra != nil {
		t.Error("lỗi mà vẫn trả danh sách")
	}
	if strings.Contains(err.Error(), congDanCT) {
		// Rule 3: the citizen identifier is the handle on a person, and an error travels into
		// logs and sometimes onto a screen.
		t.Errorf("định danh công dân lọt vào thông điệp lỗi: %v", err)
	}
}
