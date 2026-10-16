package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The Excel import over the fakes of citizen_directory_test.go: a preview names the row and the
// column and writes nothing; a commit is ALL OR NOTHING, every row carries the batch, ONE entry.

const phoneFourthFake = "0900000003"

func importUnits() []ImportResidentialUnit {
	return []ImportResidentialUnit{
		{ID: unitSampleID, Code: "thon-1", Name: "Thôn 1"},
		{ID: unitSecondID, Code: "thon-2", Name: "Thôn 2"},
	}
}

// sheetWithErrors: row 2 is fine; row 3 names a group the commune has not defined; row 4's number
// is already in the directory; row 5 names a unit the commune does not have.
func sheetWithErrors() [][]string {
	return [][]string{
		domain.CitizenContactImportColumns(),
		{nameSample, dienThoaiGia, domain.ResidentialUnitChoiceLabel("thon-1", "Thôn 1"), addressSample, "Tổ trưởng dân phố", "tổ trưởng", ""},
		{"Trần Thị B", phoneSecondFake, "Thôn 2", "", "Nhóm chưa lập", "", ""},
		{"Lê Văn C", phoneThirdFake, "", "", "", "", ""},
		{"Phạm Văn D", phoneFourthFake, "Thôn X", "", "", "", ""},
	}
}

func sheetOK() [][]string {
	return [][]string{
		domain.CitizenContactImportColumns(),
		{nameSample, dienThoaiGia, domain.ResidentialUnitChoiceLabel("thon-1", "Thôn 1"), addressSample, "Tổ trưởng dân phố", "tổ trưởng", ""},
		{"Trần Thị B", phoneSecondFake, " thôn 2 ", "", "Hộ kinh doanh, Tổ trưởng dân phố", "", "ghi chú"},
	}
}

func TestCitizenImportPreviewNamesRowAndColumnAndWritesNothing(t *testing.T) {
	h := newDirectoryHarness(t)
	h.contacts.takenPhones[phoneThirdFake] = true

	res, err := h.uc.ImportPreview(h.ctx, sheetWithErrors(), importUnits())
	if err != nil {
		t.Fatalf("ImportPreview: %v", err)
	}
	if res.RowCount != 4 {
		t.Errorf("RowCount = %d, want 4", res.RowCount)
	}
	want := map[int]string{
		3: domain.CitizenContactImportColGroups,
		4: domain.CitizenContactImportColPhone,
		5: domain.CitizenContactImportColResidentialUnit,
	}
	if len(res.Errors) != len(want) {
		t.Fatalf("errors = %+v", res.Errors)
	}
	for _, e := range res.Errors {
		if want[e.Row] != e.Column {
			t.Errorf("row %d: column %q, want %q", e.Row, e.Column, want[e.Row])
		}
		// No cell is echoed (rule 3, forbidden #3): not the number, not the typed group or unit.
		for _, cell := range []string{phoneThirdFake, "Nhóm chưa lập", "Thôn X", "Lê Văn C", "Phạm Văn D"} {
			if strings.Contains(e.Message, cell) {
				t.Errorf("row %d message echoes the cell: %s", e.Row, e.Message)
			}
		}
	}
	if len(h.contacts.inserted) != 0 || len(h.contacts.setGroups) != 0 || len(h.auditEntries()) != 0 {
		t.Error("a preview wrote")
	}
	chk := h.g.tim("FAKE citizen_contact phone taken")
	if chk == nil || h.g.ketThucCua(chk.tx) != "rollback" {
		t.Error("the preview's transaction did not roll back")
	}
}

func TestCitizenImportPreviewOfACleanFileHasNoErrors(t *testing.T) {
	h := newDirectoryHarness(t)

	res, err := h.uc.ImportPreview(h.ctx, sheetOK(), importUnits())
	if err != nil || len(res.Errors) != 0 || res.RowCount != 2 {
		t.Fatalf("res = %+v, err %v", res, err)
	}
	if len(h.contacts.inserted) != 0 || h.g.ketThucCua(1) != "rollback" {
		t.Error("a clean preview wrote, or committed")
	}
}

func TestCitizenImportCommitIsAllOrNothing(t *testing.T) {
	h := newDirectoryHarness(t)
	h.contacts.takenPhones[phoneThirdFake] = true

	_, err := h.uc.ImportCommit(h.ctx, nguoiThucHienGia(), sheetWithErrors(), importUnits())
	var rejected *CitizenContactImportRejected
	if !errors.As(err, &rejected) || len(rejected.Errors) != 3 {
		t.Fatalf("err = %v, want 3 row errors", err)
	}
	if len(h.contacts.inserted) != 0 || len(h.contacts.setGroups) != 0 || len(h.auditEntries()) != 0 {
		t.Error("a refused file inserted, set groups or audited — row 2 was fine and must still not land")
	}
	if h.g.ketThucCua(1) != "rollback" {
		t.Error("the transaction did not roll back")
	}

	// A store failure on the SECOND row: the first is rolled back with it, and no entry is written.
	h = newDirectoryHarness(t)
	h.g.loiTheo["FAKE citizen_contact_group set"] = errors.New("membership write failed")
	if _, err := h.uc.ImportCommit(h.ctx, nguoiThucHienGia(), sheetOK(), importUnits()); err == nil {
		t.Fatal("a failed row committed")
	}
	if len(h.auditEntries()) != 0 || h.g.ketThucCua(1) != "rollback" {
		t.Error("a failed import left an entry or committed")
	}
}

func TestCitizenImportCommitWritesEveryRowWithTheBatchAndOneEntry(t *testing.T) {
	h := newDirectoryHarness(t)

	res, err := h.uc.ImportCommit(h.ctx, nguoiThucHienGia(), sheetOK(), importUnits())
	if err != nil {
		t.Fatalf("ImportCommit: %v", err)
	}
	if res.RowCount != 2 || res.Batch != directoryIDPrefix+"A" {
		t.Errorf("res = %+v", res)
	}
	if len(h.contacts.inserted) != 2 {
		t.Fatalf("%d inserts, want 2", len(h.contacts.inserted))
	}
	first, second := h.contacts.inserted[0], h.contacts.inserted[1]
	for i, c := range []domain.CitizenContact{first, second} {
		if c.Source != domain.CitizenContactSourceExcel || c.ImportBatchID == nil || *c.ImportBatchID != res.Batch ||
			!c.IsActive || !c.CreatedAt.Equal(directoryInstant) {
			t.Errorf("row %d: source %q batch %v active %v", i+2, c.Source, c.ImportBatchID, c.IsActive)
		}
	}
	// Row 2 chose the dropdown value (code); row 3 typed the name with stray case and spaces.
	if first.ResidentialUnitID == nil || *first.ResidentialUnitID != unitSampleID ||
		second.ResidentialUnitID == nil || *second.ResidentialUnitID != unitSecondID {
		t.Errorf("units = %v / %v", first.ResidentialUnitID, second.ResidentialUnitID)
	}
	if len(h.contacts.setGroups) != 2 || h.contacts.setGroups[0].by != maCanBo ||
		len(h.contacts.setGroups[1].groupIDs) != 2 {
		t.Errorf("SetGroups = %+v", h.contacts.setGroups)
	}

	entries := h.auditEntries()
	if len(entries) != 1 {
		t.Fatalf("%d entries, want ONE for the run", len(entries))
	}
	e := entries[0]
	inserts := h.statements("FAKE citizen_contact insert")
	if len(inserts) != 2 || inserts[0].tx != e.tx || inserts[1].tx != e.tx || h.g.ketThucCua(e.tx) != "commit" {
		t.Fatalf("inserts in tx %d/%d, entry in tx %d, end %q", inserts[0].tx, inserts[1].tx, e.tx, h.g.ketThucCua(e.tx))
	}
	if e.args[1] != maCanBo || e.args[4] != ActionImportCitizenContacts || e.args[5] != res.Batch {
		t.Errorf("entry %v — want the STAFF CODE, the verb, the batch as subject", e.args[:6])
	}
	delta, raw := directoryDelta(t, e)
	assertNoPersonalData(t, "import delta", raw)
	if strings.Contains(raw, "Trần Thị B") {
		t.Errorf("the delta carries a name: %s", raw)
	}
	if delta["so_dong"] != float64(2) || delta["ma_lo"] != res.Batch || delta["nguon"] != "nhap-excel" {
		t.Errorf("delta = %s", raw)
	}
}

func TestCitizenImportCommitRefusesAnActorWithoutStaffCode(t *testing.T) {
	h := newDirectoryHarness(t)
	actor := nguoiThucHienGia()
	actor.Vet.ID = ""

	if _, err := h.uc.ImportCommit(h.ctx, actor, sheetOK(), importUnits()); err == nil {
		t.Fatal("an import without a staff code to attribute it to was allowed")
	}
	if h.g.soGiaoDich() != 0 {
		t.Error("a transaction was opened before the actor was checked")
	}
}
