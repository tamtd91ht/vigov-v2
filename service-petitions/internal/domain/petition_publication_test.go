package domain

import (
	"errors"
	"testing"
)

func TestCheckPublicationRequestAcceptsOnlyTheTwoActs(t *testing.T) {
	for _, ok := range []PublicationStatus{PublicationPublic, PublicationHidden} {
		got, err := CheckPublicationRequest(string(ok))
		if err != nil || got != ok {
			t.Errorf("CheckPublicationRequest(%q) = %q, %v — want accepted", ok, got, err)
		}
	}
	// `cho-duyet` is a stored value but not an act (see ErrPublicationStatusInvalid); the rest are
	// the spellings a careless client would send.
	for _, bad := range []string{"", "cho-duyet", "approved", "hidden", "CONG-KHAI", " an", "public"} {
		if _, err := CheckPublicationRequest(bad); !errors.Is(err, ErrPublicationStatusInvalid) {
			t.Errorf("CheckPublicationRequest(%q) err = %v, want ErrPublicationStatusInvalid", bad, err)
		}
	}
}

func TestPublicationAllowedRefusesOnlyPublishingStaffConduct(t *testing.T) {
	conduct := PhieuPhanAnh{LinhVuc: LinhVucHanChe, TrangThai: DangXuLy}
	if err := PublicationAllowed(conduct, PublicationPublic); !errors.Is(err, ErrNeverPublic) {
		t.Fatalf("publishing staff conduct: err = %v, want ErrNeverPublic", err)
	}
	if err := PublicationAllowed(conduct, PublicationHidden); err != nil {
		t.Errorf("hiding staff conduct refused: %v", err)
	}
	// No lifecycle status is refused (requirement service.py:764-793) — terminal ones included.
	for _, st := range []TrangThai{DaTiepNhan, DangPhanLoai, DaDong, KhongTiepNhan, ChuyenCapTren} {
		for _, field := range []string{"", "rac-thai"} {
			p := PhieuPhanAnh{LinhVuc: field, TrangThai: st}
			if err := PublicationAllowed(p, PublicationPublic); err != nil {
				t.Errorf("status %q field %q: publish refused: %v", st, field, err)
			}
		}
	}
}

func TestInitialPublicationStatus(t *testing.T) {
	if got := InitialPublicationStatus(LinhVucHanChe); got != PublicationHidden {
		t.Errorf("staff conduct born %q, want an (requirement service.py:301-307)", got)
	}
	for _, f := range []string{"", "rac-thai"} {
		if got := InitialPublicationStatus(f); got != PublicationPending {
			t.Errorf("field %q born %q, want cho-duyet", f, got)
		}
	}
}

func TestPublicationAfterClassification(t *testing.T) {
	for _, cur := range []PublicationStatus{PublicationPending, PublicationPublic, PublicationHidden} {
		if got := PublicationAfterClassification(cur, LinhVucHanChe); got != PublicationHidden {
			t.Errorf("%q classified into staff conduct -> %q, want an", cur, got)
		}
		if got := PublicationAfterClassification(cur, "rac-thai"); got != cur {
			t.Errorf("%q classified into rac-thai -> %q, want unchanged", cur, got)
		}
	}
}
