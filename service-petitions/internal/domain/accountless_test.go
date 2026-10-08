package domain

import "testing"

// The ONE accountless predicate (ADR 0083 row 11): Mini App channel AND no citizen AND no Zalo account.
func TestAccountlessIsChannelPlusNoOwner(t *testing.T) {
	for name, c := range map[string]struct {
		p    PhieuPhanAnh
		want bool
	}{
		"mini app, no owner":         {PhieuPhanAnh{Kenh: KenhZaloMiniApp}, true},
		"mini app, verified citizen": {PhieuPhanAnh{Kenh: KenhZaloMiniApp, CongDanID: "cd-1"}, false},
		"mini app, zalo account":     {PhieuPhanAnh{Kenh: KenhZaloMiniApp, ZaloAccountID: "01J"}, false},
		"staff-booked, no owner":     {PhieuPhanAnh{Kenh: KenhCanBoNhapHo}, false},
		"zalo OA, no owner":          {PhieuPhanAnh{Kenh: KenhZaloOA}, false},
	} {
		if got := c.p.Accountless(); got != c.want {
			t.Errorf("%s: Accountless() = %v, want %v", name, got, c.want)
		}
	}
}

// The accountless sender is never a Valid owner: an owner-filtered read handed it must refuse, never
// read "no owner" as "no filter".
func TestAccountlessOwnerIsNotAValidOwner(t *testing.T) {
	o := PetitionOwner{Kind: OwnerAccountless}
	if o.Valid() {
		t.Error("the accountless sender passes PetitionOwner.Valid — an owner filter could be switched off")
	}
	if !o.IsAccountless() {
		t.Error("IsAccountless() false for the accountless sender")
	}
	if (PetitionOwner{Kind: OwnerAccountless, ID: "x"}).IsAccountless() {
		t.Error("an accountless sender with an id is accepted")
	}
}

func TestAccountlessDailyCeilingIsTheOwnersFigure(t *testing.T) {
	if AccountlessDailyCeiling != 200 {
		t.Errorf("ceiling = %d, want 200 (ADR 0083 row 3) — changing it is a stop condition", AccountlessDailyCeiling)
	}
}
