package domain

import (
	"testing"
	"time"
)

// The derivation rule, tested where it lives. It needs no database and no clock: `bayGio` is an
// argument precisely so the boundary case — the instant a code expires — is testable at all,
// and that is the case that decides whether a citizen standing at a counter is refused.

var mocGhep = time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)

func moc(d time.Duration) *time.Time {
	t := mocGhep.Add(d)
	return &t
}

func TestTrangThaiGhepBonTrangThai(t *testing.T) {
	hetHan := mocGhep.Add(120 * time.Second)

	ca := []struct {
		ten     string
		dungLuc *time.Time
		huyLuc  *time.Time
		bayGio  time.Time
		muon    TrangThaiMaGhep
	}{
		{"chưa dùng, còn hạn", nil, nil, mocGhep.Add(time.Second), ChoGhep},
		{"đã dùng", moc(10 * time.Second), nil, mocGhep.Add(20 * time.Second), DaGhep},
		{"đã huỷ", nil, moc(10 * time.Second), mocGhep.Add(20 * time.Second), DaHuy},
		{"quá hạn", nil, nil, mocGhep.Add(121 * time.Second), HetHan},
		// ĐÃ DÙNG THẮNG QUÁ HẠN: a session HAS been issued to a screen. Reporting "expired"
		// would tell an operator the pairing never happened, when the evidence says it did.
		{"đã dùng rồi mới quá hạn", moc(10 * time.Second), nil, mocGhep.Add(300 * time.Second), DaGhep},
		{"đã huỷ rồi mới quá hạn", nil, moc(10 * time.Second), mocGhep.Add(300 * time.Second), DaHuy},
	}

	for _, c := range ca {
		t.Run(c.ten, func(t *testing.T) {
			ra := TrangThaiGhep(hetHan, c.dungLuc, c.huyLuc, c.bayGio)
			if ra != c.muon {
				t.Errorf("trạng thái = %q, muốn %q", ra, c.muon)
			}
		})
	}
}

func TestTrangThaiGhepDungKhoanhKhacHetHanLaHetHan(t *testing.T) {
	// THE BOUNDARY, AND IT IS NOT A DETAIL. ADR 0019 caps the life of a code AT 120 seconds, and
	// the redeem statement compares `het_han_luc > now()` — which refuses at exactly the
	// boundary. Reporting ChoGhep here would tell a screen the code is still good while the
	// database will not let anyone spend it: two answers to one question, and the citizen at the
	// counter finds out first.
	hetHan := mocGhep.Add(120 * time.Second)

	if ra := TrangThaiGhep(hetHan, nil, nil, hetHan); ra != HetHan {
		t.Errorf("đúng khoảnh khắc hết hạn: trạng thái = %q, muốn %q", ra, HetHan)
	}
	if ra := TrangThaiGhep(hetHan, nil, nil, hetHan.Add(-time.Nanosecond)); ra != ChoGhep {
		t.Errorf("một nano trước hạn: trạng thái = %q, muốn %q", ra, ChoGhep)
	}
}

func TestTrangThaiGhepVuaDungVuaHuyThiBaoDaDung(t *testing.T) {
	// CONSTRAINT ghep_phien_khong_vua_dung_vua_huy makes this row impossible. It is decided here
	// anyway, because "impossible" is a statement about today's schema: if such a row ever
	// appeared, a session WAS issued to a screen, and that is the fact an operator has to be
	// told.
	hetHan := mocGhep.Add(120 * time.Second)

	if ra := TrangThaiGhep(hetHan, moc(5*time.Second), moc(6*time.Second), mocGhep.Add(10*time.Second)); ra != DaGhep {
		t.Errorf("trạng thái = %q, muốn %q", ra, DaGhep)
	}
}
