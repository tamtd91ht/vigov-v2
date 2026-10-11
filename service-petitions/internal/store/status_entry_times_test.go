package store

import (
	"database/sql/driver"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Migration 0043's two columns are read BY NAME (driver_gia_test.go), with two DISTINCT instants so a
// swap of the adjacent pair comes back as wrong data; NULL reads as the zero time ("not recorded").
func TestScanReadsTheStatusEntryTimes(t *testing.T) {
	forwarded := time.Date(2026, 10, 1, 2, 0, 0, 0, time.UTC)
	awaiting := time.Date(2026, 10, 2, 4, 0, 0, 0, time.UTC)
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(map[string]driver.Value{
		"forwarded_for_processing_at": forwarded, "awaiting_confirmation_at": awaiting})}}
	p, err := NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k))).TheoMaTraCuu(ctxXa(xaThu), maThu)
	if err != nil {
		t.Fatal(err)
	}
	if !p.ForwardedForProcessingAt.Equal(forwarded) || !p.AwaitingConfirmationAt.Equal(awaiting) {
		t.Errorf("times = %v / %v", p.ForwardedForProcessingAt, p.AwaitingConfirmationAt)
	}

	k = &khoGia{hangTheoCot: []map[string]driver.Value{dongPhieu(nil)}}
	p, err = NewPhieuPhanAnhStore(pkgstore.New(moKhoGia(k))).TheoMaTraCuu(ctxXa(xaThu), maThu)
	if err != nil {
		t.Fatal(err)
	}
	if !p.ForwardedForProcessingAt.IsZero() || !p.AwaitingConfirmationAt.IsZero() {
		t.Errorf("NULL read as %v / %v", p.ForwardedForProcessingAt, p.AwaitingConfirmationAt)
	}
}
