package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// Tests that need no database. The pg suite next door SKIPS without VIGOV_TEST_DSN while the
// package still prints `ok`, so everything decidable without a server is decided here.

const (
	xaKCD    = "01J0000000000000000000000C"
	xaKhacCD = "01J0000000000000000000000D"
	congDan1 = "01JCONGDAN0000000000000001"
)

var (
	mocKhai = time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)
	mocXet  = time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)
)

// dongQuanHe is one sample row, keyed by column name.
//
// EVERY VALUE IS DISTINGUISHABLE FROM EVERY OTHER, and two choices are deliberate:
//
//   - khai_cu_tru = tam_tru WITH trang_thai_xac_thuc = tu_choi. The two facts are independent,
//     so the fixture sets them to a combination that no single merged column could produce:
//     a declaration that says one thing and a review that says another.
//   - xet_duyet_boi and ly_do_tu_choi are the two adjacent plain-string columns. Their values
//     could not be mistaken for one another by a human — which is exactly what makes a swapped
//     Scan visible, because nothing else would catch it.
func dongQuanHe() map[string]driver.Value {
	return map[string]driver.Value{
		"cong_dan_id":         congDan1,
		"khai_cu_tru":         "tam_tru",
		"nguon_khai":          "can_bo",
		"khai_luc":            mocKhai,
		"trang_thai_xac_thuc": "tu_choi",
		"xet_duyet_boi":       "CB-XET-01",
		"xet_duyet_luc":       mocXet,
		"ly_do_tu_choi":       "Địa chỉ khai không thuộc địa bàn xã.",
	}
}

// nhieuDongQuanHe builds n rows with distinct citizen ids and increasing khai_luc, so a test
// about the bound is not also a test about ordering.
func nhieuDongQuanHe(n int) []map[string]driver.Value {
	ra := make([]map[string]driver.Value, 0, n)
	for i := 0; i < n; i++ {
		d := dongQuanHe()
		d["cong_dan_id"] = congDan1[:len(congDan1)-4] + string(rune('A'+i%26)) + "000"
		d["khai_luc"] = mocKhai.Add(time.Duration(i) * time.Minute)
		d["trang_thai_xac_thuc"] = "cho_xac_thuc"
		// EMPTY STRING, NOT nil, FOR THE TWO COALESCED COLUMNS — and the reason is a limit of
		// the fake worth naming: it does not evaluate SQL, so `coalesce(xet_duyet_boi,'')` is
		// not applied here. The sample has to be the value PostgreSQL would hand back. That
		// the coalesce is really in the statement is checked by the statement assertions; that
		// it really turns NULL into '' is checked only by the pg suite, against the real
		// schema.
		d["xet_duyet_boi"] = ""
		d["ly_do_tu_choi"] = ""
		// xet_duyet_luc keeps its NULL: it is NOT coalesced, because a zero time.Time would
		// read as 01/01/0001 on a screen, which is a date somebody would report.
		d["xet_duyet_luc"] = nil
		ra = append(ra, d)
	}
	return ra
}

// phienGia stands in for the citizen session registry so the edge can resolve one.
type phienGia struct{ p httpx.CitizenSession }

func (s phienGia) TraCuu(context.Context, string) (httpx.CitizenSession, bool) {
	return s.p, true
}

// ctxKenhCongDan builds the context a citizen request really carries, BY RUNNING THE REAL EDGE.
//
// WHY NOT context.WithValue DIRECTLY: core/httpx keeps the context key unexported on purpose,
// so there is no way for code outside the edge to forge a citizen session — which is the
// mechanism behind rule 4, invariant 2. A test that could forge one would be testing a path
// that does not exist in production. Running httpx.CitizenEdge is both honest and cheap.
//
// `gan` CHOOSES THE COMMUNE CLASS OF THE ROUTE (ADR 0022): XaTuPhien puts the session's commune
// into the context for a business route, KhongThuocXa declares a route that belongs to no
// commune — the discovery layer, where a citizen has signed in but not chosen a commune yet.
func ctxKenhCongDan(t *testing.T, p httpx.CitizenSession, gan bool) context.Context {
	t.Helper()
	var ra context.Context
	cuoi := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ra = r.Context()
		w.WriteHeader(http.StatusOK)
	})
	var trong http.Handler
	if gan {
		trong = httpx.XaTuPhien()(cuoi)
	} else {
		trong = httpx.KhongThuocXa("test: lớp khám phá, chưa chọn xã")(cuoi)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token-gia-khong-phai-token-that")
	rec := httptest.NewRecorder()
	httpx.CitizenEdge(phienGia{p: p})(trong).ServeHTTP(rec, req)
	if ra == nil {
		t.Fatalf("rìa kênh công dân không gọi tới handler, mã %d", rec.Code)
	}
	return ra
}

func ctxCongDanTrongXa(t *testing.T, xa, congDanID string) context.Context {
	t.Helper()
	return ctxKenhCongDan(t, httpx.CitizenSession{
		ID: "SID-01", CitizenID: congDanID, TenantID: tenant.ID(xa),
	}, true)
}

// --- rule 1: the commune, and where it comes from ----------------------------------------------

func TestQuanHeTheoPhienBuocXaVaoThamSoMotTuContext(t *testing.T) {
	// RULE 1, INVARIANTS 4 AND 5. The commune is not an argument and cannot become one: it
	// arrives in the context and the scoped repository binds it to $1.
	k := khoMoi()
	k.hang = []map[string]driver.Value{dongQuanHe()}

	if _, err := NewQuanHeCongDanXaStore(dbGia(k)).
		TheoPhienCongDan(ctxCongDanTrongXa(t, xaKCD, congDan1)); err != nil {
		t.Fatalf("TheoPhienCongDan lỗi: %v", err)
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}
	if len(l.args) < 1 || l.args[0] != xaKCD {
		t.Fatalf("$1 = %v, muốn xã trong context %q", l.args, xaKCD)
	}

	// The same store, another commune in the context, another $1. A store that cached the first
	// commune it saw would fail here.
	k2 := khoMoi()
	k2.hang = []map[string]driver.Value{dongQuanHe()}
	if _, err := NewQuanHeCongDanXaStore(dbGia(k2)).
		TheoPhienCongDan(ctxCongDanTrongXa(t, xaKhacCD, congDan1)); err != nil {
		t.Fatalf("TheoPhienCongDan lỗi: %v", err)
	}
	if k2.cuoi().args[0] != xaKhacCD {
		t.Errorf("$1 = %v, muốn %q", k2.cuoi().args[0], xaKhacCD)
	}
}

// --- rule 4: the citizen, and where THAT comes from --------------------------------------------

func TestQuanHeTheoPhienBuocCongDanTuPhienChuKhongPhaiThamSo(t *testing.T) {
	// RULE 4, INVARIANT 2. The method takes no citizen id, so $2 can only have come from the
	// session the server issued. If this ever became a parameter, changing one value in a URL
	// would read somebody else's declaration (rule 4, forbidden #1).
	k := khoMoi()
	k.hang = []map[string]driver.Value{dongQuanHe()}

	if _, err := NewQuanHeCongDanXaStore(dbGia(k)).
		TheoPhienCongDan(ctxCongDanTrongXa(t, xaKCD, congDan1)); err != nil {
		t.Fatalf("TheoPhienCongDan lỗi: %v", err)
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "cong_dan_id = $2") {
		t.Errorf("câu lệnh không lọc theo công dân: %q", l.sql)
	}
	if len(l.args) < 2 || l.args[1] != congDan1 {
		t.Fatalf("$2 = %v, muốn công dân của phiên %q", l.args, congDan1)
	}
}

func TestQuanHeTheoPhienKhongCoPhienThiTuChoiVaKhongChayCauLenhNao(t *testing.T) {
	// FAIL CLOSED. No session means there is no honest way to name the citizen, and every
	// fallback anyone would reach for takes the identity from something the client sent.
	k := khoMoi()
	ctx := tenant.Into(context.Background(), tenant.ID(xaKCD))

	_, err := NewQuanHeCongDanXaStore(dbGia(k)).TheoPhienCongDan(ctx)
	if !errors.Is(err, ErrThieuPhienCongDan) {
		t.Fatalf("lỗi = %v, muốn ErrThieuPhienCongDan", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("không có phiên mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

func TestQuanHeTheoPhienLechXaThiTuChoiVaKhongChayCauLenhNao(t *testing.T) {
	// The session says commune A, the request is scoped to commune B. On the citizen edge this
	// cannot happen — XaTuPhien is what puts the session's commune into the context — so it
	// only fires when some other code path put a different one there. That is the shape of a
	// citizen reading another commune's row, and it must not be silent.
	k := khoMoi()
	ctx := tenant.Into(ctxCongDanTrongXa(t, xaKCD, congDan1), tenant.ID(xaKhacCD))

	_, err := NewQuanHeCongDanXaStore(dbGia(k)).TheoPhienCongDan(ctx)
	if !errors.Is(err, ErrPhienLechXa) {
		t.Fatalf("lỗi = %v, muốn ErrPhienLechXa", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("phiên lệch xã mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

func TestQuanHeTheoPhienKhongCoXaTrongContextThiPanic(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A read that ran without a commune would reach every commune's rows
	// or none, and both are silent. httpx.Recover turns the panic into a traceable 500.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("đọc quan hệ khi context không có xã mà không panic")
		}
	}()
	k := khoMoi()
	ctx := ctxKenhCongDan(t, httpx.CitizenSession{
		ID: "SID-01", CitizenID: congDan1, TenantID: "",
	}, false)
	_, _ = NewQuanHeCongDanXaStore(dbGia(k)).TheoPhienCongDan(ctx)
}

// --- rule 7 and the shape of the statement ------------------------------------------------------

func TestQuanHeTheoPhienLocDongDaXoaMem(t *testing.T) {
	// RULE 7, INVARIANT 2: every read path excludes soft-deleted rows, everywhere. A
	// relationship row is kept rather than deleted precisely because it is the basis of a
	// citizen's access to their own files; kept AND invisible is the whole point of the pair.
	k := khoMoi()
	k.hang = []map[string]driver.Value{dongQuanHe()}

	if _, err := NewQuanHeCongDanXaStore(dbGia(k)).
		TheoPhienCongDan(ctxCongDanTrongXa(t, xaKCD, congDan1)); err != nil {
		t.Fatalf("TheoPhienCongDan lỗi: %v", err)
	}
	if !strings.Contains(k.cuoi().sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", k.cuoi().sql)
	}
}

func TestQuanHeLuonDocCaHaiSuThat(t *testing.T) {
	// THE DEFECT THIS TABLE EXISTS TO AVOID, pinned as a test. A read that returned the
	// declaration without the verification state — or the other way round — is the merge
	// arrived at by omission: the caller counts claims and reports registrations, the query
	// runs, the column holds a legal value, and the number reaching leadership is wrong with
	// nothing turning red.
	k := khoMoi()
	k.hang = []map[string]driver.Value{dongQuanHe()}
	s := NewQuanHeCongDanXaStore(dbGia(k))
	ctx := ctxCongDanTrongXa(t, xaKCD, congDan1)

	if _, err := s.TheoPhienCongDan(ctx); err != nil {
		t.Fatalf("TheoPhienCongDan lỗi: %v", err)
	}
	if _, err := s.HangChoXacThuc(ctx, 10); err != nil {
		t.Fatalf("HangChoXacThuc lỗi: %v", err)
	}
	for _, l := range k.lenh {
		if !strings.Contains(l.sql, "khai_cu_tru") || !strings.Contains(l.sql, "trang_thai_xac_thuc") {
			t.Errorf("câu lệnh đọc một sự thật mà thiếu sự thật kia: %q", l.sql)
		}
	}
}

// --- what comes back ----------------------------------------------------------------------------

func TestQuanHeTheoPhienDocDungTungCot(t *testing.T) {
	k := khoMoi()
	k.hang = []map[string]driver.Value{dongQuanHe()}

	q, err := NewQuanHeCongDanXaStore(dbGia(k)).
		TheoPhienCongDan(ctxCongDanTrongXa(t, xaKCD, congDan1))
	if err != nil {
		t.Fatalf("TheoPhienCongDan lỗi: %v", err)
	}
	if q.CongDanID != congDan1 {
		t.Errorf("CongDanID = %q, muốn %q", q.CongDanID, congDan1)
	}
	// The two facts, read independently. A merged column could not produce this pair.
	if q.Khai != domain.KhaiTamTru {
		t.Errorf("Khai = %q, muốn tam_tru", q.Khai)
	}
	if q.TrangThai != domain.TuChoi {
		t.Errorf("TrangThai = %q, muốn tu_choi", q.TrangThai)
	}
	if q.NguonKhai != domain.NguonKhaiCanBo {
		t.Errorf("NguonKhai = %q, muốn can_bo", q.NguonKhai)
	}
	if !q.KhaiLuc.Equal(mocKhai) {
		t.Errorf("KhaiLuc = %v, muốn %v", q.KhaiLuc, mocKhai)
	}
	// THE PAIR THAT NO TYPE CAN CATCH: both are plain strings and adjacent in the SELECT list.
	// Swapping them shows an officer's account id to the citizen as the reason their
	// declaration was refused, and files the reason as the name of the reviewer.
	if q.XetDuyetBoi != "CB-XET-01" {
		t.Errorf("XetDuyetBoi = %q — hai cột TEXT liền nhau đọc ngược?", q.XetDuyetBoi)
	}
	if !strings.HasPrefix(q.LyDoTuChoi, "Địa chỉ khai") {
		t.Errorf("LyDoTuChoi = %q — hai cột TEXT liền nhau đọc ngược?", q.LyDoTuChoi)
	}
	if q.XetDuyetLuc == nil || !q.XetDuyetLuc.Equal(mocXet) {
		t.Errorf("XetDuyetLuc = %v, muốn %v", q.XetDuyetLuc, mocXet)
	}
}

func TestQuanHeTheoPhienKhongCoDongThiBaoChuaCoQuanHe(t *testing.T) {
	// A legitimate answer, not a failure: the row is created by the citizen's first act toward
	// the commune (ADR 0005). It is a named error so the caller answers 404 rather than 500.
	k := khoMoi()

	_, err := NewQuanHeCongDanXaStore(dbGia(k)).
		TheoPhienCongDan(ctxCongDanTrongXa(t, xaKCD, congDan1))
	if !errors.Is(err, ErrKhongCoQuanHe) {
		t.Fatalf("lỗi = %v, muốn ErrKhongCoQuanHe", err)
	}
}

// --- the verification queue ----------------------------------------------------------------------

func TestHangChoXacThucLocDungTrangThaiVaSapXepOnDinh(t *testing.T) {
	k := khoMoi()
	k.hang = nhieuDongQuanHe(3)

	if _, err := NewQuanHeCongDanXaStore(dbGia(k)).
		HangChoXacThuc(ctxCongDanTrongXa(t, xaKCD, congDan1), 10); err != nil {
		t.Fatalf("HangChoXacThuc lỗi: %v", err)
	}
	l := k.cuoi()
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("hàng chờ không lọc theo xã: %q", l.sql)
	}
	if !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("hàng chờ không loại dòng đã xoá mềm: %q", l.sql)
	}
	if !strings.Contains(l.sql, "trang_thai_xac_thuc = $2") {
		t.Errorf("hàng chờ không lọc theo trạng thái xác thực: %q", l.sql)
	}
	// THE ORDER IS TOTAL: khai_luc is the queue's own order — longest wait first — and
	// cong_dan_id breaks ties. Without the tie-break two reads can return the same rows in a
	// different sequence and an officer's screen reshuffles under their finger.
	if !strings.Contains(l.sql, "ORDER BY khai_luc, cong_dan_id") {
		t.Errorf("thứ tự hàng chờ không ổn định: %q", l.sql)
	}
	if len(l.args) < 3 {
		t.Fatalf("thiếu tham số: %v", l.args)
	}
	if l.args[1] != string(domain.ChoXacThuc) {
		t.Errorf("$2 = %v, muốn %q", l.args[1], domain.ChoXacThuc)
	}
	// The bound is the caller's limit PLUS ONE — the extra row is the only way ConNua can be
	// known without counting rows over a 32-way partitioned table.
	if l.args[2] != int64(11) {
		t.Errorf("LIMIT = %v, muốn 11 (giới hạn + 1)", l.args[2])
	}
}

func TestHangChoXacThucDungGioiHanThiKhongBaoConNua(t *testing.T) {
	// Exactly the limit is a COMPLETE read, not a truncated one. An off-by-one here tells an
	// officer there is more work waiting when there is not.
	k := khoMoi()
	k.hang = nhieuDongQuanHe(3)

	ra, err := NewQuanHeCongDanXaStore(dbGia(k)).
		HangChoXacThuc(ctxCongDanTrongXa(t, xaKCD, congDan1), 3)
	if err != nil {
		t.Fatalf("HangChoXacThuc lỗi: %v", err)
	}
	if len(ra.Muc) != 3 {
		t.Fatalf("nhận %d dòng, muốn 3", len(ra.Muc))
	}
	if ra.ConNua {
		t.Error("đúng giới hạn mà vẫn báo còn nữa")
	}
}

func TestHangChoXacThucVuotGioiHanThiBaoConNuaVaKhongTraThua(t *testing.T) {
	// The limit+1-th row exists only as a signal. Returning it would hand the caller one row
	// more than it asked for — and an officer processing a queue would skip whatever fell into
	// the gap.
	k := khoMoi()
	k.hang = nhieuDongQuanHe(4)

	ra, err := NewQuanHeCongDanXaStore(dbGia(k)).
		HangChoXacThuc(ctxCongDanTrongXa(t, xaKCD, congDan1), 3)
	if err != nil {
		t.Fatalf("HangChoXacThuc lỗi: %v", err)
	}
	if len(ra.Muc) != 3 {
		t.Fatalf("nhận %d dòng, muốn đúng 3", len(ra.Muc))
	}
	if !ra.ConNua {
		t.Error("còn dòng chờ mà không báo")
	}
}

func TestHangChoXacThucRongVanLaCauTraLoiHopLe(t *testing.T) {
	// A commune with nobody waiting is the normal state, and it is a list — never a nil the
	// caller has to branch on.
	k := khoMoi()

	ra, err := NewQuanHeCongDanXaStore(dbGia(k)).
		HangChoXacThuc(ctxCongDanTrongXa(t, xaKCD, congDan1), 10)
	if err != nil {
		t.Fatalf("hàng chờ rỗng phải hợp lệ, nhận lỗi: %v", err)
	}
	if ra.Muc == nil {
		t.Fatal("trả nil thay vì lát rỗng")
	}
	if len(ra.Muc) != 0 || ra.ConNua {
		t.Errorf("nhận %d dòng, ConNua=%v", len(ra.Muc), ra.ConNua)
	}
}

func TestHangChoXacThucTuChoiGioiHanKhongHopLe(t *testing.T) {
	// No default is substituted: a default here answers, on behalf of a route that does not
	// exist yet, how much of a work queue an officer sees.
	k := khoMoi()
	s := NewQuanHeCongDanXaStore(dbGia(k))
	ctx := ctxCongDanTrongXa(t, xaKCD, congDan1)

	if _, err := s.HangChoXacThuc(ctx, 0); !errors.Is(err, ErrGioiHanKhongHopLe) {
		t.Errorf("giới hạn 0: lỗi = %v, muốn ErrGioiHanKhongHopLe", err)
	}
	if _, err := s.HangChoXacThuc(ctx, TranHangChoXacThuc+1); !errors.Is(err, ErrGioiHanVuotTran) {
		t.Errorf("trên trần: lỗi = %v, muốn ErrGioiHanVuotTran", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("giới hạn sai mà vẫn chạy %d câu lệnh", len(k.lenh))
	}
}

// --- failures --------------------------------------------------------------------------------

func TestQuanHeLoiKhoDuocBocChuKhongNuot(t *testing.T) {
	// Wrapped with %w, never swallowed with `_`: the caller tells a database outage apart from
	// "this citizen has no relationship here" with errors.Is, which needs the chain intact.
	goc := errors.New("cơ sở dữ liệu không phản hồi")
	k := khoMoi()
	k.loi = goc

	_, err := NewQuanHeCongDanXaStore(dbGia(k)).
		TheoPhienCongDan(ctxCongDanTrongXa(t, xaKCD, congDan1))
	if !errors.Is(err, goc) {
		t.Fatalf("lỗi = %v, muốn bọc %v", err, goc)
	}
	if errors.Is(err, ErrKhongCoQuanHe) {
		t.Error("lỗi kho bị nhận nhầm là chưa có quan hệ")
	}
	if strings.Contains(err.Error(), congDan1) {
		// Rule 3: the citizen identifier is the handle on a person, and an error travels into
		// logs and sometimes onto a screen.
		t.Errorf("định danh công dân lọt vào thông điệp lỗi: %v", err)
	}
}
