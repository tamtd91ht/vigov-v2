package store

import (
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
)

// The rating columns (ADR 0050 point 2) are read BY NAME from the store's own SELECT list, appended
// AFTER the branch columns — the one position where an added destination shifts nothing.
//
// NOT PROVED HERE: the two UPDATE statements. This fake has no transactions; they run over the
// transaction-capable driver in internal/app/petition_rating_test.go, through the real store.

func TestLookupReadsTheRating(t *testing.T) {
	at := time.Date(2026, 9, 27, 3, 41, 0, 0, time.UTC)
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(map[string]driver.Value{
		"trang_thai":     "cho-dan-xac-nhan",
		"diem_hai_long":  int64(2),
		"rating_comment": "Rác vẫn còn ở cuối ngõ.",
		"danh_gia_luc":   at,
		"so_lan_mo_lai":  int64(3),
	})}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	p, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
	if err != nil {
		t.Fatalf("đọc phiếu: %v", err)
	}
	if p.Rating != 2 || p.RatingComment != "Rác vẫn còn ở cuối ngõ." || !p.RatedAt.Equal(at) {
		t.Errorf("đánh giá đọc sai: sao=%d nhận xét=%q lúc=%v", p.Rating, p.RatingComment, p.RatedAt)
	}
	// The neighbours are unchanged — a destination inserted rather than appended would shift these.
	if p.SoLanMoLai != 3 || p.LyDoKetThucNhanh != "" || !p.KetThucNhanhLuc.IsZero() {
		t.Errorf("cột lân cận bị lệch: so_lan_mo_lai=%d nhánh=%q/%v", p.SoLanMoLai, p.LyDoKetThucNhanh,
			p.KetThucNhanhLuc)
	}
}

func TestLookupUnratedIsZero(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	s := NewPhieuPhanAnhStore(store.New(moKhoGia(k)))

	p, err := s.TheoMaTraCuu(ctxXa(xaThu), maThu)
	if err != nil {
		t.Fatalf("đọc phiếu: %v", err)
	}
	if p.Rating != 0 || p.RatingComment != "" || !p.RatedAt.IsZero() {
		t.Errorf("phiếu chưa chấm lại mang đánh giá: %d %q %v", p.Rating, p.RatingComment, p.RatedAt)
	}
}

// TestRatingMaxFilterIsBound — the low-rating slice is a BOUND parameter, excludes unrated petitions,
// and is absent when not asked for.
func TestRatingMaxFilterIsBound(t *testing.T) {
	sqlText, args := locPhieuThanhSQL(LocPhieu{RatingMax: 2, ChoPhepHanChe: true})
	if !strings.Contains(sqlText, "diem_hai_long IS NOT NULL AND diem_hai_long <= $2") {
		t.Errorf("thiếu điều kiện đánh giá thấp gắn tham số: %q", sqlText)
	}
	if len(args) != 1 || args[0] != 2 {
		t.Errorf("tham số = %v, muốn [2]", args)
	}

	sqlText, _ = locPhieuThanhSQL(LocPhieu{ChoPhepHanChe: true})
	if strings.Contains(sqlText, "diem_hai_long") {
		t.Errorf("không lọc sao mà câu lệnh vẫn có điều kiện đánh giá: %q", sqlText)
	}

	// Beside another filter, the placeholder numbers follow the argument order.
	sqlText, args = locPhieuThanhSQL(LocPhieu{TrangThai: "dang-xu-ly", RatingMax: 1})
	if !strings.Contains(sqlText, "trang_thai = $2") || !strings.Contains(sqlText, "diem_hai_long <= $3") ||
		len(args) != 2 || args[1] != 1 {
		t.Errorf("thứ tự tham số lệch: %q %v", sqlText, args)
	}
	// The restricted-field exclusion still applies (zero value of ChoPhepHanChe).
	if !strings.Contains(sqlText, "linh_vuc <> 'can-bo'") {
		t.Errorf("lọc đánh giá thấp làm mất điều kiện lĩnh vực hạn chế: %q", sqlText)
	}
}
