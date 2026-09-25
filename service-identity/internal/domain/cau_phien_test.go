package domain

import (
	"errors"
	"testing"
)

// One case per row of ADR 0045 §Chế độ và xã của phiên. The "active" half of each row is the use
// case's (it asks the registry); what is pinned here is the candidate and the rule for inactive.

const (
	xaA = "01JD8ZQK9M3NPXR7TVWYB2C4EA"
	xaB = "01JD8ZQK9M3NPXR7TVWYB2C4EB"
)

func TestChonXaAppRiengLayXaCuaAppVaBoQuaGoiY(t *testing.T) {
	for _, xacNhan := range []bool{false, true} {
		u, err := ChonXa(DauVaoChonXa{CheDo: CheDoAppRieng, XaCuaAppRieng: xaA, GoiY: xaB, DaXacNhan: xacNhan, XaDaNho: xaB})
		if err != nil {
			t.Fatalf("xác nhận=%v: %v", xacNhan, err)
		}
		if u.Xa != xaA || u.Nguon != NguonXaAppRieng {
			t.Fatalf("xác nhận=%v: %+v — app riêng phải theo xã của app, bỏ qua t= và xã đã nhớ", xacNhan, u)
		}
		if !u.TuChoiNeuNgung || u.NhoXa {
			t.Fatalf("app riêng: %+v — xã ngừng thì từ chối, và không ghi xã đã nhớ", u)
		}
	}
}

func TestChonXaAppRiengXaKhongHoatDongThiTuChoi(t *testing.T) {
	// The platform leaves the binding absent when the bound commune is inactive. No fallback to the
	// hint, the remembered commune, or a successor.
	_, err := ChonXa(DauVaoChonXa{CheDo: CheDoAppRieng, GoiY: xaB, DaXacNhan: true, XaDaNho: xaB})
	if !errors.Is(err, ErrAppRiengKhongCoXa) {
		t.Fatalf("err = %v, muốn ErrAppRiengKhongCoXa", err)
	}
}

func TestChonXaAppChinhDaXacNhanThiTheoQRVaNhoXa(t *testing.T) {
	u, err := ChonXa(DauVaoChonXa{CheDo: CheDoAppChinh, GoiY: xaB, DaXacNhan: true, XaDaNho: xaA})
	if err != nil {
		t.Fatal(err)
	}
	if u.Xa != xaB || u.Nguon != NguonXaQRXacNhan || !u.NhoXa || !u.TuChoiNeuNgung {
		t.Fatalf("%+v — QR đã xác nhận: xã của QR, nhớ xã ấy, xã ngừng thì từ chối", u)
	}
}

func TestChonXaAppChinhXacNhanMaKhongCoGoiYLaLoiNoiDay(t *testing.T) {
	_, err := ChonXa(DauVaoChonXa{CheDo: CheDoAppChinh, DaXacNhan: true, XaDaNho: xaA})
	if !errors.Is(err, ErrXacNhanKhongCoGoiY) {
		t.Fatalf("err = %v, muốn ErrXacNhanKhongCoGoiY", err)
	}
}

func TestChonXaAppChinhChuaXacNhanThiTheoXaDaNho(t *testing.T) {
	// With or without a hint: an unconfirmed QR moves nobody.
	for _, goiY := range []string{"", xaB} {
		u, err := ChonXa(DauVaoChonXa{CheDo: CheDoAppChinh, GoiY: goiY, XaDaNho: xaA})
		if err != nil {
			t.Fatal(err)
		}
		if u.Xa != xaA || u.Nguon != NguonXaNhoLai || u.NhoXa || u.TuChoiNeuNgung {
			t.Fatalf("t=%q: %+v — xã đã nhớ, không đổi xã đã nhớ, xã ngừng thì về không xã", goiY, u)
		}
	}
}

func TestChonXaAppChinhChuaXacNhanChuaNhoThiKhongCoXa(t *testing.T) {
	for _, goiY := range []string{"", xaB} {
		u, err := ChonXa(DauVaoChonXa{CheDo: CheDoAppChinh, GoiY: goiY})
		if err != nil {
			t.Fatal(err)
		}
		if u.Xa != "" || u.Nguon != NguonXaKhongCoXa || u.NhoXa {
			t.Fatalf("t=%q: %+v — không có xã nào, và TUYỆT ĐỐI không lấy t= làm xã (luật 1 cấm #2)", goiY, u)
		}
	}
}

func TestChonXaCheDoLaThiTuChoi(t *testing.T) {
	if _, err := ChonXa(DauVaoChonXa{CheDo: CheDoAppKhongRo, XaCuaAppRieng: xaA, GoiY: xaA, DaXacNhan: true}); !errors.Is(err, ErrCheDoAppKhongRo) {
		t.Fatalf("err = %v, muốn ErrCheDoAppKhongRo", err)
	}
}
