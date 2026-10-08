package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// THE SESSION'S VERIFIED PHONE, ATTACHED WHEN THE BOX IS EMPTY — ADR 0050 §Sửa đổi 08/10/2026 point 2.
//
//	PROVED HERE   a verified citizen, not anonymous, with nothing typed gets the session's number stored
//	              in NguoiGuiDienThoai, normalised · identity is asked with the SESSION's sid and the
//	              owner's citizen id · the trail names `nguoi_gui_dien_thoai` without its value · a typed
//	              number, an anonymous send, a Zalo-account owner and an accountless send never ask · an
//	              empty answer is ErrContactPhoneNoSession and an outage ErrContactPhoneUnavailable, both
//	              with nothing written and no code · a failed deadline never reaches the phone question.

const sidThu = "01JSESSIONCUAPHIENCONGDAN"

// contactPhonesFake is identity's ResolveCitizenContactPhone, recording what it was asked.
type contactPhonesFake struct {
	phone string
	err   error

	calls       int
	seenSession []string
	seenCitizen []string
}

func (f *contactPhonesFake) ResolveCitizenContactPhone(_ context.Context, sessionID, citizenID string) (
	string, error) {
	f.calls++
	f.seenSession = append(f.seenSession, sessionID)
	f.seenCitizen = append(f.seenCitizen, citizenID)
	return f.phone, f.err
}

func verifiedCitizenWithSession() IntakeSender {
	s := congDanThu()
	s.SessionID = sidThu
	return s
}

func ycNoPhone() YeuCauGuiPhanAnh {
	yc := ycThu()
	yc.DienThoai = ""
	return yc
}

func buildWithPhones(k *khoGia, kho *khoPhieuGia, han HanTiepNhanDoc, phones CitizenContactPhones) *GuiPhanAnh {
	uc := dungGui(k, kho, han)
	uc.phones = phones
	return uc
}

func TestContactPhoneAttachedForVerifiedCitizenWithEmptyBox(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
	// Surrounding blanks prove the answer goes through domain.ChuanHoaDienThoai like a typed number.
	phones := &contactPhonesFake{phone: "  0900000000 "}

	p, err := buildWithPhones(k, kho, han, phones).Gui(ctxXa(xaThu), ycNoPhone(), verifiedCitizenWithSession())
	if err != nil {
		t.Fatalf("Gui: %v", err)
	}
	if phones.calls != 1 {
		t.Fatalf("asked identity %d times, want 1", phones.calls)
	}
	if phones.seenSession[0] != sidThu || phones.seenCitizen[0] != idCongDan {
		t.Errorf("asked with session %q citizen %q, want the session's sid %q and the owner %q",
			phones.seenSession[0], phones.seenCitizen[0], sidThu, idCongDan)
	}
	if len(kho.thay) != 1 || kho.thay[0].NguoiGuiDienThoai != "0900000000" {
		t.Fatalf("stored phone = %q, want the normalised verified number", kho.thay[0].NguoiGuiDienThoai)
	}
	if p.MaTraCuu == "" {
		t.Error("no lookup code returned on a successful intake")
	}

	// THE TRAIL NAMES THE FIELD, NEVER THE VALUE (rule 6, forbidden #4).
	raw, ok := k.lenh[1].args[7].([]byte)
	if !ok {
		t.Fatalf("delta = %T, want []byte", k.lenh[1].args[7])
	}
	if strings.Contains(string(raw), "0900000000") {
		t.Errorf("the attached number is in the audit delta: %s", raw)
	}
	var d struct {
		Fields []string `json:"truong_da_dien"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("delta is not JSON: %q", raw)
	}
	found := false
	for _, f := range d.Fields {
		found = found || f == "nguoi_gui_dien_thoai"
	}
	if !found {
		t.Errorf("truong_da_dien = %v, want nguoi_gui_dien_thoai named like a typed number", d.Fields)
	}
}

// A typed number, an anonymous send, a Zalo-account owner (ADR 0080) and an accountless send (ADR 0083)
// never ask identity — and the intake still succeeds with whatever was typed.
func TestContactPhoneNotAskedOutsideTheOneCase(t *testing.T) {
	anonymous := ycNoPhone()
	anonymous.AnDanh = true
	zalo := zaloThu()
	zalo.SessionID = sidThu

	cases := map[string]struct {
		yc        YeuCauGuiPhanAnh
		sender    IntakeSender
		wantPhone string
	}{
		"typed number wins":  {ycThu(), verifiedCitizenWithSession(), "0900000000"},
		"anonymous":          {anonymous, verifiedCitizenWithSession(), ""},
		"zalo-account owner": {ycNoPhone(), zalo, ""},
		"accountless":        {ycNoPhone(), AccountlessSender("10.0.0.9"), ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
			phones := &contactPhonesFake{phone: "0900000000"}

			if _, err := buildWithPhones(k, kho, han, phones).Gui(ctxXa(xaThu), c.yc, c.sender); err != nil {
				t.Fatalf("Gui: %v", err)
			}
			if phones.calls != 0 {
				t.Errorf("asked identity %d times — only a verified, non-anonymous citizen with an empty "+
					"box may be asked", phones.calls)
			}
			if got := kho.thay[0].NguoiGuiDienThoai; got != c.wantPhone {
				t.Errorf("stored phone = %q, want %q", got, c.wantPhone)
			}
		})
	}
}

func TestContactPhoneRefusalsWriteNothing(t *testing.T) {
	cases := map[string]struct {
		phones *contactPhonesFake
		want   error
	}{
		"empty answer -> no session": {&contactPhonesFake{phone: ""}, ErrContactPhoneNoSession},
		"blank answer -> no session": {&contactPhonesFake{phone: "   "}, ErrContactPhoneNoSession},
		"outage -> unavailable": {&contactPhonesFake{
			err: errors.New("identityclient: ResolveCitizenContactPhone: rpc error: code = Unavailable")},
			ErrContactPhoneUnavailable},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

			p, err := buildWithPhones(k, kho, han, c.phones).Gui(ctxXa(xaThu), ycNoPhone(),
				verifiedCitizenWithSession())
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if domain.LaLoiGuiPhanAnh(err) {
				t.Error("refusal classified as the citizen's input fault (400)")
			}
			if len(k.lenh) != 0 || len(kho.thay) != 0 {
				t.Errorf("wrote %d statements / %d rows on a refused intake", len(k.lenh), len(kho.thay))
			}
			if p.MaTraCuu != "" {
				t.Errorf("issued lookup code %q on a refused intake", p.MaTraCuu)
			}
			for _, leak := range []string{sidThu, idCongDan, "0900000000"} {
				if strings.Contains(err.Error(), leak) {
					t.Errorf("error carries %q (rule 3): %v", leak, err)
				}
			}
		})
	}
}

// A use case built without the source, reached in the one case that needs it, is a wiring fault —
// refused, never read as "no number" and never filed without one.
func TestContactPhoneMissingSourceIsRefused(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()

	_, err := dungGui(k, kho, han).Gui(ctxXa(xaThu), ycNoPhone(), verifiedCitizenWithSession())
	if err == nil {
		t.Fatal("intake succeeded without the verified-phone source")
	}
	if len(k.lenh) != 0 || len(kho.thay) != 0 {
		t.Errorf("wrote %d statements / %d rows", len(k.lenh), len(kho.thay))
	}
}

// The commune's commitment is asked first: an intake that fails on its deadline never causes a
// disclosure (identity audits every one).
func TestContactPhoneNotAskedWhenDeadlineFails(t *testing.T) {
	k, kho, han := &khoGia{}, &khoPhieuGia{}, hanThu()
	han.loi = errors.New("rpc error: code = FailedPrecondition")
	phones := &contactPhonesFake{phone: "0900000000"}

	_, err := buildWithPhones(k, kho, han, phones).Gui(ctxXa(xaThu), ycNoPhone(), verifiedCitizenWithSession())
	if !errors.Is(err, ErrChuaAnDinhDuocHan) {
		t.Fatalf("err = %v, want ErrChuaAnDinhDuocHan", err)
	}
	if phones.calls != 0 {
		t.Errorf("asked identity for the phone %d times on an intake already refused", phones.calls)
	}
}
