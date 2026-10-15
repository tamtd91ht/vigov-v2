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

// A scene photo is read whoever attached it — the citizen's own or the officer's at intake (owner
// decision 15/10/2026) — and never a file of another purpose, petition or subject.
func TestIsScenePhotoOf(t *testing.T) {
	for name, up := range map[string]string{"citizen": CitizenLogActor, "officer": "CB-00123"} {
		f := StoredFile{SubjectType: StoredFileSubjectPetition, SubjectID: "pa-1", Purpose: PurposePetitionPhoto,
			UploadedBy: up}
		if !IsScenePhotoOf(f, "pa-1") {
			t.Errorf("%s's scene photo of pa-1 refused", name)
		}
	}
	for name, f := range map[string]StoredFile{
		"another petition":   {SubjectType: StoredFileSubjectPetition, SubjectID: "pa-2", Purpose: PurposePetitionPhoto, UploadedBy: "CB-00123"},
		"a task file":        {SubjectType: StoredFileSubjectTask, SubjectID: "pa-1", Purpose: PurposePetitionPhoto, UploadedBy: "CB-00123"},
		"a verification":     {SubjectType: StoredFileSubjectPetition, SubjectID: "pa-1", Purpose: PurposePetitionVerificationPhoto, UploadedBy: "CB-00123"},
		"no uploader at all": {SubjectType: StoredFileSubjectPetition, SubjectID: "pa-1", Purpose: PurposePetitionPhoto},
	} {
		if IsScenePhotoOf(f, "pa-1") {
			t.Errorf("%s accepted as a scene photo of pa-1", name)
		}
	}
	if IsScenePhotoOf(StoredFile{SubjectType: StoredFileSubjectPetition, Purpose: PurposePetitionPhoto,
		UploadedBy: "CB-00123"}, "") {
		t.Error("an empty petition id matched")
	}
}
