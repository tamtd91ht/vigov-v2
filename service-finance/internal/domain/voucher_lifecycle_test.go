package domain

import (
	"errors"
	"strings"
	"testing"
)

// Task card G1+G2 (07/10/2026): the voucher lifecycle brought to the prototype on the owner's
// instruction. These are the domain halves: which state admits a lock, who may confirm or lock, and
// what a blank removal reason becomes. The SQL and transaction halves are in internal/app.

const (
	recorderCode = "CB-00123"
	otherCode    = "CB-00999"
)

func voucherIn(state TrangThaiChungTu) ChungTuGiaiNgan {
	return ChungTuGiaiNgan{ID: "ct", TrangThai: state, NguoiNhapID: recorderCode}
}

func TestVoucherLock_AdmitsDraftAndConfirmed(t *testing.T) {
	// `Kế toán nhập` IS ADMITTED NOW — it used to be ErrChuaXacNhanThiChuaKhoaDuoc. A lock from there
	// confirms in the same act (LockConfirmsToo), which is what keeps TrangThaiSauKhiMoKhoa exact.
	for _, tc := range []struct {
		state       TrangThaiChungTu
		want        error
		confirmsToo bool
	}{
		{ChungTuKeToanNhap, nil, true},
		{ChungTuDaXacNhan, nil, false},
		{ChungTuDaKhoa, ErrChungTuDaKhoaRoi, false},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			v := voucherIn(tc.state)
			if err := v.ChoKhoa(otherCode); !errors.Is(err, tc.want) {
				t.Fatalf("ChoKhoa = %v, muốn %v", err, tc.want)
			}
			if got := v.LockConfirmsToo(); got != tc.confirmsToo {
				t.Errorf("LockConfirmsToo = %v, muốn %v", got, tc.confirmsToo)
			}
		})
	}
}

func TestVoucherLock_UnknownStateRefused(t *testing.T) {
	// A fourth state is refused, never admitted by a default branch.
	if err := voucherIn("unknown-state").ChoKhoa(otherCode); err == nil {
		t.Fatal("trạng thái lạ mà vẫn cho khoá")
	}
}

func TestSelfConfirmation_RecorderAndBlankActorRefused(t *testing.T) {
	// The recorder may neither confirm nor lock; a BLANK actor is refused too (fail closed) — and it is
	// refused even when the recorder field is itself blank, which a plain equality would wave through.
	for _, tc := range []struct {
		name     string
		recorder string
		actor    string
		want     error
	}{
		{"recorder", recorderCode, recorderCode, ErrSelfConfirmation},
		{"blank actor", recorderCode, "", ErrSelfConfirmation},
		{"blank actor, blank recorder", "", "", ErrSelfConfirmation},
		{"another officer", recorderCode, otherCode, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, state := range []TrangThaiChungTu{ChungTuKeToanNhap, ChungTuDaXacNhan} {
				v := voucherIn(state)
				v.NguoiNhapID = tc.recorder
				if err := v.ChoKhoa(tc.actor); !errors.Is(err, tc.want) {
					t.Errorf("ChoKhoa(%s) = %v, muốn %v", state, err, tc.want)
				}
			}
			v := voucherIn(ChungTuKeToanNhap)
			v.NguoiNhapID = tc.recorder
			if err := v.ChoXacNhan(tc.actor); !errors.Is(err, tc.want) {
				t.Errorf("ChoXacNhan = %v, muốn %v", err, tc.want)
			}
		})
	}
}

func TestSelfConfirmation_StateAnsweredFirst(t *testing.T) {
	// As the prototype: a voucher already confirmed or locked answers THAT, whoever asks.
	if err := voucherIn(ChungTuDaXacNhan).ChoXacNhan(recorderCode); !errors.Is(err, ErrChungTuDaXacNhan) {
		t.Errorf("ChoXacNhan = %v, muốn ErrChungTuDaXacNhan", err)
	}
	if err := voucherIn(ChungTuDaKhoa).ChoKhoa(recorderCode); !errors.Is(err, ErrChungTuDaKhoaRoi) {
		t.Errorf("ChoKhoa = %v, muốn ErrChungTuDaKhoaRoi", err)
	}
}

func TestSelfConfirmation_MessageIsThePrototypes(t *testing.T) {
	// The web strips the `chung_tu: ` technical prefix and shows the rest; the rest is the sentence
	// the card fixes.
	if !strings.HasSuffix(ErrSelfConfirmation.Error(), "Không thể tự xác nhận khoản do chính mình nhập") {
		t.Fatalf("câu = %q", ErrSelfConfirmation.Error())
	}
}

func TestVoucherRemovalReason_BlankBecomesDefault(t *testing.T) {
	// Task card G2: optional on the way in, never empty in the row (rule 7, invariant 1).
	for _, blank := range []string{"", "   ", "\t"} {
		if got, err := ChuanHoaLyDoGo(blank); err != nil || got != VoucherRemovalDefaultReason {
			t.Errorf("ChuanHoaLyDoGo(%q) = %q, %v — muốn %q", blank, got, err, VoucherRemovalDefaultReason)
		}
	}
	if VoucherRemovalDefaultReason != "Gỡ khoản chi nhập nhầm" {
		t.Errorf("câu mặc định = %q", VoucherRemovalDefaultReason)
	}
	if got, err := ChuanHoaLyDoGo("  nhập trùng  "); err != nil || got != "nhập trùng" {
		t.Errorf("lý do có nội dung = %q, %v", got, err)
	}
	if _, err := ChuanHoaLyDoGo(strings.Repeat("a", LyDoGoChungTuToiDa+1)); !errors.Is(err, ErrLyDoGoQuaDai) {
		t.Errorf("lý do quá dài = %v, muốn ErrLyDoGoQuaDai", err)
	}
	if _, err := ChuanHoaLyDoGo("nhập\x07trùng"); !errors.Is(err, ErrThieuLyDoGo) {
		t.Errorf("ký tự điều khiển = %v, muốn ErrThieuLyDoGo", err)
	}
}
