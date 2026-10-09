package app

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// SCENE PHOTOS ON AN UNVERIFIED PETITION — ADR 0080 decision 8: the owner is the Zalo account.
//
//	PROVED HERE   the account that owns the petition gets a slot, its trail actor is the account with
//	              Kind zalo-account · another account, and a CITIZEN whose id happens to be the same
//	              string, get the one not-found answer · the list signs links for the owner only.

const (
	ppZaloOwner = "01JZALOOWNERPHOTOTHU00000"
	ppZaloCode  = "PA-ZALO-OWNED-0001"
)

func zaloPhotoActor(id string) audit.Actor {
	// @actor-ok: the "who" of an unverified petition's act is `tai_khoan_zalo.id` with Kind
	// zalo-account (ADR 0080 decision 9); not a staff actor, so no business code exists.
	return audit.Actor{ID: id, Kind: audit.KindZaloAccount, IP: "10.0.0.9"}
}

func buildZaloPhotos(t *testing.T) *ppHarness {
	h := buildPhotos(t)
	h.pets.byCommune[xaThu][ppZaloCode] = domain.PhieuPhanAnh{ID: ppPetition, MaTraCuu: ppZaloCode,
		ZaloAccountID: ppZaloOwner, TrangThai: domain.DaTiepNhan, Kenh: domain.KenhZaloMiniApp}
	return h
}

func TestUnverifiedPhotoOwnerGetsSlotAndTrailNamesAccount(t *testing.T) {
	h := buildZaloPhotos(t)
	raw := ppJPEG(t, 20, 10, 1)
	if _, err := h.uc.Upload(h.ctx, ppZaloCode, PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: int64(len(raw))},
		uploadBodyOf(raw), zaloPhotoActor(ppZaloOwner)); err != nil {
		t.Fatalf("Upload của chủ phiếu là tài khoản Zalo: %v", err)
	}
	a := h.db.audits()
	if len(a) != 2 {
		t.Fatalf("vết = %v, muốn xin tải + lưu", a)
	}
	for _, e := range a {
		if e[0] != ppZaloOwner || e[1] != audit.KindZaloAccount || e[3] != ppZaloCode {
			t.Fatalf("vết = %v, muốn chủ thể %q kind %q", a, ppZaloOwner, audit.KindZaloAccount)
		}
	}
}

func TestUnverifiedPhotoRefusesAnyoneButTheOwningAccount(t *testing.T) {
	for name, actor := range map[string]audit.Actor{
		"another Zalo account": zaloPhotoActor("01JZALOOTHERACCOUNT000000"),
		// The SAME id string presented as a citizen must not reach the account's petition: the kind
		// chooses the column, never the string.
		"a citizen with the same id string": {ID: ppZaloOwner, Kind: "citizen", IP: "10.0.0.9"},
	} {
		t.Run(name, func(t *testing.T) {
			h := buildZaloPhotos(t)
			_, err := h.uc.Upload(h.ctx, ppZaloCode,
				PhotoUploadRequest{ContentType: storage.MIMEJPEG, Size: 400_000}, uploadBodyOf(make([]byte, 400_000)), actor)
			if !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
				t.Fatalf("err = %v, muốn ErrPhieuKhongTonTai (một câu 404 duy nhất)", err)
			}
			if len(h.files.inserted) != 0 || len(h.db.committed) != 0 {
				t.Errorf("ghi %d dòng / %d lệnh cho người không phải chủ phiếu", len(h.files.inserted), len(h.db.committed))
			}
			if _, err := h.uc.ListPhotos(h.ctx, ppZaloCode, actor); !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
				t.Errorf("ListPhotos err = %v, muốn ErrPhieuKhongTonTai", err)
			}
		})
	}
}

func TestUnverifiedPhotoListForOwner(t *testing.T) {
	h := buildZaloPhotos(t)
	h.addPhoto("01JPH0T0000000000000000009", domain.StoredFileStored, ppNow)
	links, err := h.uc.ListPhotos(h.ctx, ppZaloCode, zaloPhotoActor(ppZaloOwner))
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("%d liên kết, muốn 1", len(links))
	}
}
