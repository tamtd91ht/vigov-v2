package store

import (
	"database/sql/driver"
	"testing"

	"github.com/vihat/vigov/core/store"
)

// TestReadCarriesUpdatedAt: `cap_nhat_luc` is at the tail of cotNhiemVu and lands in UpdatedAt — the
// PATCH precondition token (28/09/2026). The fixture gives it a value distinct from `tao_luc`, so a
// Scan that read one instant into the other field shows up here.
func TestReadCarriesUpdatedAt(t *testing.T) {
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(nil)}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))

	n, err := s.TheoMa(ctxXa(xaThu), maNhiemVuThu)
	if err != nil {
		t.Fatalf("đọc nhiệm vụ: %v", err)
	}
	if !n.UpdatedAt.Equal(mocSuaNVThu) {
		t.Errorf("UpdatedAt = %v, muốn %v", n.UpdatedAt, mocSuaNVThu)
	}
	if !n.TaoLuc.Equal(mocTaoNVThu) {
		t.Errorf("TaoLuc = %v, muốn %v — hai mốc bị đọc lẫn", n.TaoLuc, mocTaoNVThu)
	}
}
