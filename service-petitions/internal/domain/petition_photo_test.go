package domain

import "testing"

func TestPhotoUploadOpenOnlyWhileReceived(t *testing.T) {
	for _, s := range []TrangThai{DaTiepNhan, DangPhanLoai, DaChuyenXuLy, DangXuLy, DaXuLy, ChoDanXacNhan,
		DaDong, KhongTiepNhan, ChuyenCapTren} {
		if got, want := PhotoUploadOpen(s), s == DaTiepNhan; got != want {
			t.Errorf("PhotoUploadOpen(%s) = %v, want %v", s, got, want)
		}
	}
}

func TestIsCitizenPhotoOf(t *testing.T) {
	ok := StoredFile{SubjectType: StoredFileSubjectPetition, SubjectID: "pa-1", Purpose: PurposePetitionPhoto,
		UploadedBy: CitizenLogActor}
	if !IsCitizenPhotoOf(ok, "pa-1") {
		t.Fatal("a citizen photo of pa-1 refused")
	}
	for name, f := range map[string]StoredFile{
		"another petition": {SubjectType: StoredFileSubjectPetition, SubjectID: "pa-2", Purpose: PurposePetitionPhoto, UploadedBy: CitizenLogActor},
		"a task file":      {SubjectType: StoredFileSubjectTask, SubjectID: "pa-1", Purpose: PurposePetitionPhoto, UploadedBy: CitizenLogActor},
		"a staff upload":   {SubjectType: StoredFileSubjectPetition, SubjectID: "pa-1", Purpose: PurposePetitionPhoto, UploadedBy: "CB-00123"},
		"another purpose":  {SubjectType: StoredFileSubjectPetition, SubjectID: "pa-1", Purpose: "task-attachment", UploadedBy: CitizenLogActor},
	} {
		if IsCitizenPhotoOf(f, "pa-1") {
			t.Errorf("%s accepted as a citizen photo of pa-1", name)
		}
	}
	if IsCitizenPhotoOf(ok, "") {
		t.Error("an empty petition id matched")
	}
}
