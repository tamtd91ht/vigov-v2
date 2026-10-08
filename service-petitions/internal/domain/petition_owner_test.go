package domain

import (
	"testing"
	"time"
)

// The day boundary of the unverified ceiling (ADR 0080 decision 7) is 00:00 Asia/Ho_Chi_Minh, whatever
// zone `now` arrives in — 16:59 UTC is still "yesterday" in Viet Nam, 17:00 UTC is "today".
func TestUnverifiedCountingDayStartIsVietnamMidnight(t *testing.T) {
	want := time.Date(2026, 10, 7, 17, 0, 0, 0, time.UTC) // 08/10 00:00 ICT
	for _, now := range []time.Time{
		time.Date(2026, 10, 7, 17, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 8, 16, 59, 59, 0, time.UTC),
		time.Date(2026, 10, 8, 1, 0, 0, 0, time.FixedZone("GIA-TAY", -8*3600)), // 16:00 ICT
	} {
		if got := UnverifiedCountingDayStart(now); !got.Equal(want) {
			t.Errorf("day start of %v = %v, want %v", now, got, want)
		}
	}
	if got := UnverifiedCountingDayStart(time.Date(2026, 10, 7, 16, 59, 59, 0, time.UTC)); got.Equal(want) {
		t.Errorf("16:59:59 UTC on 07/10 is 23:59:59 ICT — still the previous day, got %v", got)
	}
}

func TestPetitionOwnerValid(t *testing.T) {
	for _, c := range []struct {
		o    PetitionOwner
		want bool
	}{
		{PetitionOwner{Kind: OwnerCitizen, ID: "cd-1"}, true},
		{PetitionOwner{Kind: OwnerZaloAccount, ID: "01J"}, true},
		{PetitionOwner{Kind: OwnerZaloAccount, ID: " "}, false},
		{PetitionOwner{Kind: "staff", ID: "CB-1"}, false},
		{PetitionOwner{ID: "x"}, false},
	} {
		if got := c.o.Valid(); got != c.want {
			t.Errorf("%+v.Valid() = %v, want %v", c.o, got, c.want)
		}
	}
	if !(PhieuPhanAnh{ZaloAccountID: "01J"}).ContactUnverified() || (PhieuPhanAnh{CongDanID: "cd"}).ContactUnverified() {
		t.Error("ContactUnverified must follow the Zalo-account owner column alone")
	}
}
