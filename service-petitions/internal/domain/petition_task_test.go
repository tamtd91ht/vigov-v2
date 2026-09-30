package domain

import (
	"errors"
	"testing"
)

// The whole table of PetitionAcceptsTask (user decision 30/09/2026), status by status. Every one of the
// nine is listed, so a tenth status added to the lifecycle without a decision here shows up as the
// "unknown code" row's answer rather than as an accidental yes.
func TestPetitionAcceptsTask_EveryStatus(t *testing.T) {
	cases := map[TrangThai]error{
		DaTiepNhan:    ErrPetitionNotClassifiedForTask,
		DangPhanLoai:  ErrPetitionNotClassifiedForTask,
		DaChuyenXuLy:  nil,
		DangXuLy:      nil,
		DaXuLy:        nil,
		ChoDanXacNhan: nil,
		DaDong:        ErrPetitionClosedForTask,
		KhongTiepNhan: ErrPetitionClosedForTask,
		ChuyenCapTren: ErrPetitionClosedForTask,
		"da-huy":      ErrPetitionClosedForTask, // not one of the nine: fail closed
		"":            ErrPetitionClosedForTask,
	}
	if len(chuyenDuocSang) != 9 {
		t.Fatalf("vòng đời có %d trạng thái — bảng này chỉ quyết cho chín", len(chuyenDuocSang))
	}
	for status, want := range cases {
		t.Run(string(status), func(t *testing.T) {
			got := PetitionAcceptsTask(PhieuPhanAnh{TrangThai: status, LinhVuc: "rac-thai"})
			if !errors.Is(got, want) || (want == nil && got != nil) {
				t.Errorf("PetitionAcceptsTask(%q) = %v, muốn %v", status, got, want)
			}
		})
	}
}

// An allowed status with no settled field is data the lifecycle did not produce: refused as unclassified.
func TestPetitionAcceptsTask_AllowedStatusWithoutFieldIsNotClassified(t *testing.T) {
	for _, status := range []TrangThai{DaChuyenXuLy, DangXuLy, DaXuLy, ChoDanXacNhan} {
		if err := PetitionAcceptsTask(PhieuPhanAnh{TrangThai: status}); !errors.Is(err, ErrPetitionNotClassifiedForTask) {
			t.Errorf("%s không có lĩnh vực: %v, muốn ErrPetitionNotClassifiedForTask", status, err)
		}
	}
}

// `can-bo` is refused at EVERY status, including the ones that would otherwise accept.
func TestPetitionAcceptsTask_RestrictedFieldAtEveryStatus(t *testing.T) {
	for status := range chuyenDuocSang {
		err := PetitionAcceptsTask(PhieuPhanAnh{TrangThai: status, LinhVuc: LinhVucHanChe})
		if !errors.Is(err, ErrRestrictedFieldNoTask) {
			t.Errorf("%s + can-bo: %v, muốn ErrRestrictedFieldNoTask", status, err)
		}
	}
}
