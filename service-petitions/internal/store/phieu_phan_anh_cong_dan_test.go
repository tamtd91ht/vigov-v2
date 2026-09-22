package store

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// What this file defends: the CITIZEN half of the petition register — rule 4, invariant 3, which
// wants BOTH axes on every citizen-path query, the session's identity AND the commune.
//
// READ THIS BEFORE TRUSTING A GREEN RUN. The fake driver hands back whatever rows the fixture
// holds, REGARDLESS OF THE WHERE CLAUSE (driver_gia_test.go §NOT PROVED). So a test shaped as
// "ask for another citizen's code, expect no row" would pass with the filter deleted — the worst
// trap in this repository, a guard that looks alive and is not. The property is therefore asserted
// where it is actually decidable without a PostgreSQL: THE STATEMENT AND ITS ARGUMENTS. Delete
// `AND cong_dan_id = $3` from the store and TestCuaCongDanTheoMaTraCuuLocCaHaiTruc turns red on
// two independent assertions.
//
//	PROVED HERE   the citizen identity reaches the statement as $3 · the commune still reaches it
//	              as $1 and still comes from the CONTEXT · the soft-delete predicate survives on
//	              this path too · an empty identity is REFUSED BEFORE any statement runs · no row
//	              is the same sentinel the staff path uses, so the route cannot tell "not yours"
//	              from "no such code" · neither the code nor the identifier is in a wrapped error.
//
//	NOT PROVED    that PostgreSQL applies the predicate. That needs a DSN — see
//	              phieu_phan_anh_pg_test.go, which skips without one.

const congDanThu = "cd-001" // the same opaque id dongPhieu writes into `cong_dan_id`

func TestCuaCongDanTheoMaTraCuuLocCaHaiTruc(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	if _, err := s.CuaCongDanTheoMaTraCuu(ctxXa(xaThu), congDanThu, maThu); err != nil {
		t.Fatalf("đọc phiếu của công dân: %v", err)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1", len(k.lenh))
	}
	l := k.lenh[0]

	// AXIS ONE — the commune (rule 1, invariants 4 and 5). Not a parameter, and it cannot become
	// one: Scoped.Query binds it to $1 from the context.
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu lệnh không lọc theo xã: %q", l.sql)
	}

	// AXIS TWO — the citizen (rule 4, invariants 1 and 3). THIS IS THE ASSERTION THE MUTATION TEST
	// AIMS AT: without it, a citizen holding any valid lookup code reads a petition that is not
	// theirs, and every other test in this package stays green.
	if !strings.Contains(l.sql, "cong_dan_id = $3") {
		t.Errorf("câu lệnh KHÔNG lọc theo danh tính công dân: %q", l.sql)
	}
	if !strings.Contains(l.sql, "ma_tra_cuu = $2") {
		t.Errorf("câu lệnh không tra theo mã tra cứu: %q", l.sql)
	}
	// RULE 7, INVARIANT 2 — every read path excludes soft-deleted rows, the citizen path included.
	if !strings.Contains(l.sql, "deleted_at IS NULL") {
		t.Errorf("thiếu điều kiện loại dòng đã xoá mềm: %q", l.sql)
	}

	// THE ARGUMENTS, IN ORDER. The statement carrying the right placeholder while the wrong value
	// is bound to it is a filter that reads as present and filters on nothing.
	if len(l.args) != 3 {
		t.Fatalf("tham số = %v, muốn đúng ba [xã, mã tra cứu, định danh công dân]", l.args)
	}
	if l.args[0] != string(xaThu) {
		t.Errorf("$1 = %v, muốn xã trong context %q", l.args[0], xaThu)
	}
	if l.args[1] != maThu {
		t.Errorf("$2 = %v, muốn mã tra cứu %q", l.args[1], maThu)
	}
	if l.args[2] != congDanThu {
		t.Errorf("$3 = %v, muốn định danh công dân của phiên %q", l.args[2], congDanThu)
	}
}

// TestCuaCongDanTheoMaTraCuuXaVanDenTuContext keeps the commune axis honest on THIS method too.
// One store, two contexts, two communes: a store that had captured a commune at construction — or
// taken one as an argument beside the citizen id — would bind the same $1 twice, and no test of a
// single commune could tell. This is the "phiên xã B hỏi phiếu xã A" case at the layer where it is
// decidable without a database.
func TestCuaCongDanTheoMaTraCuuXaVanDenTuContext(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	xaKhac := tenant.ID("01JB" + strings.Repeat("B", 22))
	if _, err := s.CuaCongDanTheoMaTraCuu(ctxXa(xaThu), congDanThu, maThu); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CuaCongDanTheoMaTraCuu(ctxXa(xaKhac), congDanThu, maThu); err != nil {
		t.Fatal(err)
	}
	if len(k.lenh) != 2 {
		t.Fatalf("chạy %d câu lệnh, muốn 2", len(k.lenh))
	}
	// THE ARITY IS CHECKED BEFORE THE VALUES, so that deleting the citizen filter fails this test
	// with a sentence a reader can act on instead of an index-out-of-range panic further down. A
	// mutation that crashes the harness is a mutation whose message nobody reads.
	for i, l := range k.lenh {
		if len(l.args) != 3 {
			t.Fatalf("lời gọi %d có %d tham số, muốn ba [xã, mã tra cứu, định danh công dân]: %v",
				i+1, len(l.args), l.args)
		}
	}
	if k.lenh[0].args[0] != string(xaThu) || k.lenh[1].args[0] != string(xaKhac) {
		t.Errorf("$1 của hai lời gọi = %v và %v — xã phải đi theo context, không theo kho",
			k.lenh[0].args[0], k.lenh[1].args[0])
	}
	// The citizen id is the SAME in both calls on purpose: one citizen may hold a session in
	// commune A and later in commune B (ADR 0005), and the commune must still decide.
	if k.lenh[1].args[2] != congDanThu {
		t.Errorf("$3 = %v, muốn %q", k.lenh[1].args[2], congDanThu)
	}
}

// TestCuaCongDanTheoMaTraCuuDinhDanhRongBiTuChoiTruocKhiChayLenh is the fail-closed case, and it
// asserts the REFUSAL AND THE ABSENCE OF A STATEMENT.
//
// Asserting only the error would pass on an implementation that ran the query first and rejected
// the result afterwards — which is one refactor away from running it and not rejecting anything.
// Nothing may reach the register on behalf of a citizen nobody identified.
func TestCuaCongDanTheoMaTraCuuDinhDanhRongBiTuChoiTruocKhiChayLenh(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	_, err := s.CuaCongDanTheoMaTraCuu(ctxXa(xaThu), "", maThu)
	if !errors.Is(err, ErrThieuDinhDanhCongDan) {
		t.Fatalf("lỗi = %v, muốn ErrThieuDinhDanhCongDan", err)
	}
	if len(k.lenh) != 0 {
		t.Fatalf("đã chạy %d câu lệnh với định danh rỗng — phải từ chối TRƯỚC khi chạm kho", len(k.lenh))
	}
	// AND IT MUST NOT BE THE 404 SENTINEL. A lost session identity answered as "không tìm thấy"
	// tells a citizen their petition is gone while the real fault is an isolation path that
	// stopped working — and nothing in any log says so.
	if errors.Is(err, ErrPhieuKhongTonTai) {
		t.Error("định danh rỗng bị gộp vào ErrPhieuKhongTonTai — một đường cách ly hỏng sẽ đội lốt 404")
	}
}

// TestCuaCongDanTheoMaTraCuuKhongCoDongDungSentinelCuaDuongCanBo pins the ONE answer rule 4,
// forbidden #2 requires: "không tồn tại", "của người khác", "xã khác" and "đã xoá mềm" must be
// indistinguishable to the caller. Sharing the sentinel with the staff path is what makes the
// handler physically unable to tell them apart and answer differently.
func TestCuaCongDanTheoMaTraCuuKhongCoDongDungSentinelCuaDuongCanBo(t *testing.T) {
	k := &khoGia{} // no rows — the driver's answer for every one of the four causes
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	_, err := s.CuaCongDanTheoMaTraCuu(ctxXa(xaThu), congDanThu, maThu)
	if !errors.Is(err, ErrPhieuKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrPhieuKhongTonTai — cùng một sentinel với đường cán bộ", err)
	}
}

func TestCuaCongDanTheoMaTraCuuLoiKhongMangMaCungKhongMangDinhDanh(t *testing.T) {
	k := &khoGia{loi: errors.New("pg: connection refused")}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	_, err := s.CuaCongDanTheoMaTraCuu(ctxXa(xaThu), congDanThu, maThu)
	if err == nil {
		t.Fatal("lỗi driver bị nuốt")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("lỗi gốc không được bọc bằng %%w: %v", err)
	}
	if strings.Contains(err.Error(), maThu) {
		t.Errorf("mã tra cứu lọt vào thông điệp lỗi: %v", err)
	}
	// THE IDENTIFIER NAMES A PERSON. It is opaque, but it is stable and it is the key that ties a
	// pile of log lines together into one citizen's activity (rule 3, invariants 1 and 2).
	if strings.Contains(err.Error(), congDanThu) {
		t.Errorf("định danh công dân lọt vào thông điệp lỗi: %v", err)
	}
}
