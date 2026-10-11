package store

import (
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The staff-recorded rating read (migration 0042). The row is assembled BY NAME from the store's own
// SELECT list, so a reordered column list against its Scan comes back as wrong data.
//
// NOT PROVED HERE: Append and ForPetitionTx — this fake has no transactions; they run through the use case
// in internal/app/petition_staff_recorded_rating_test.go. PostgreSQL's CHECKs — migrations/ reads the text.

func TestStaffRatingReadIsScopedAndByName(t *testing.T) {
	at := time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)
	k := &khoGia{hangTheoCot: []map[string]driver.Value{{
		"id": "sr-1", "petition_id": "pa-1", "stars": int64(3), "channel": "tai-quay", "note": "Nói tại quầy.",
		"source": "staff_recorded", "recorded_by": "CB-00311", "recorded_at": at,
	}}}
	s := NewPetitionStaffRatingStore(store.New(moKhoGia(k)))

	r, ok, err := s.ForPetition(ctxXa(xaThu), "pa-1")
	if err != nil || !ok {
		t.Fatalf("đọc: ok=%v err=%v", ok, err)
	}
	if r.Stars != 3 || r.Channel != domain.StaffRatingAtCounter || r.Note != "Nói tại quầy." ||
		r.Source != domain.StaffRecordedSource || r.RecordedBy != "CB-00311" || !r.RecordedAt.Equal(at) {
		t.Errorf("đọc sai: %+v", r)
	}
	q := k.lenh[0]
	if !strings.Contains(q.sql, "FROM petition_staff_recorded_rating WHERE tenant_id = $1") ||
		!strings.Contains(q.sql, "AND petition_id = $2") || q.args[0] != string(xaThu) || q.args[1] != "pa-1" {
		t.Errorf("câu đọc không giới hạn theo xã và phiếu: %q %v", q.sql, q.args)
	}
	if strings.Contains(q.sql, "diem_hai_long") || strings.Contains(q.sql, "phieu_phan_anh") {
		t.Errorf("câu đọc chạm cột/bảng đánh giá của người dân: %q", q.sql)
	}
}

func TestStaffRatingReadNoneIsFalse(t *testing.T) {
	s := NewPetitionStaffRatingStore(store.New(moKhoGia(&khoGia{})))
	if _, ok, err := s.ForPetition(ctxXa(xaThu), "pa-1"); err != nil || ok {
		t.Errorf("không có bản ghi: ok=%v err=%v", ok, err)
	}
}
