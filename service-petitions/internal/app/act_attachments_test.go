package app

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Files on the six status acts (09/10/2026) — act_attachments.go.
//
//	PROVED HERE   each act links the offered files to THE TIMELINE ROW IT WROTE, in its own committed
//	              transaction, and names the file ids (never names) in its audit delta · a file that is not
//	              this officer's completed log attachment of this petition refuses the WHOLE act — no
//	              UPDATE, no row, no trail · an empty or repeated id refuses before any transaction · the
//	              close gate still decides a closing on its own, and a log attachment cannot satisfy it ·
//	              an act without files reads no file and writes no `tep_dinh_kem`.
//	NOT PROVED    migration 0027's trigger (stored_file_pg_test.go, skipped without VIGOV_TEST_DSN).

var actFiles = []string{"f1", "f2"}

func actCandidates() map[string]domain.AttachCandidate {
	return map[string]domain.AttachCandidate{
		"f1": petitionLogCandidate("f1", idPhieuThu, maCanBoThu, domain.StoredFileStored, ""),
		"f2": petitionLogCandidate("f2", idPhieuThu, maCanBoThu, domain.StoredFileReady, ""),
	}
}

// inClassification is a petition waiting at `dang-phan-loai`, the state the two branches and the
// assignment leave from.
func inClassification() map[string]driver.Value {
	return dongPhieuMau(map[string]any{
		"trang_thai": string(domain.DangPhanLoai), "linh_vuc": "rac-thai",
		"han_xu_ly_xong": mocXuLyXongThu, "phan_loai_luc": mocThaoTac,
	})
}

type actCase struct {
	name string
	row  func() map[string]driver.Value
	run  func(uc *XuLyPhanAnh, ids []string) error
}

func actCases() []actCase {
	return []actCase{
		{"classification", func() map[string]driver.Value { return dongPhieuMau(nil) },
			func(uc *XuLyPhanAnh, ids []string) error {
				_, err := uc.ChotLinhVuc(ctxXa(xaThu), maPhieuThu, YeuCauChotLinhVuc{LinhVuc: "rac-thai", Attachments: ids},
					canBoThu(), khongQuyenHanChe)
				return err
			}},
		{"assignment", inClassification, func(uc *XuLyPhanAnh, ids []string) error {
			_, err := uc.PhanCong(ctxXa(xaThu), maPhieuThu, YeuCauPhanCong{BoPhan: "bp-001", Attachments: ids},
				canBoThu(), khongQuyenHanChe)
			return err
		}},
		{"status step", func() map[string]driver.Value { return phieuDaGiaoCho(maCanBoThu) },
			func(uc *XuLyPhanAnh, ids []string) error {
				_, err := uc.TienTrangThai(ctxXa(xaThu), maPhieuThu, "", canBoThu(), khongQuyenCaXa, khongQuyenHanChe, ids...)
				return err
			}},
		{"closure", func() map[string]driver.Value { return closablePetition().hang },
			func(uc *XuLyPhanAnh, ids []string) error {
				_, err := uc.Dong(ctxXa(xaThu), maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe, ids...)
				return err
			}},
		{"rejection", inClassification, func(uc *XuLyPhanAnh, ids []string) error {
			_, err := uc.KhongTiepNhan(ctxXa(xaThu), maPhieuThu, "Nội dung phản ánh không thuộc thẩm quyền giải quyết của xã.",
				"", canBoThu(), khongQuyenHanChe, ids...)
			return err
		}},
		{"referral", inClassification, func(uc *XuLyPhanAnh, ids []string) error {
			_, err := uc.ChuyenCapTren(ctxXa(xaThu), maPhieuThu, YeuCauChuyenCapTren{
				LyDo: "Sự cố lưới điện trung thế thuộc thẩm quyền điện lực.", CoQuanNhan: "Điện lực huyện",
				Attachments: ids}, canBoThu(), khongQuyenHanChe)
			return err
		}},
	}
}

// actAuditDelta decodes the act's audit entry.
func actAuditDelta(t *testing.T, k *khoPhieuXuLyGia) map[string]any {
	t.Helper()
	ins := k.cau("INSERT INTO audit_log")
	if len(ins) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(ins))
	}
	var d map[string]any
	if err := json.Unmarshal(ins[0].args[7].([]byte), &d); err != nil {
		t.Fatalf("delta: %v", err)
	}
	return d
}

func TestActsLinkFilesToTheRowTheyWrote(t *testing.T) {
	for _, c := range actCases() {
		t.Run(c.name, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = c.row()
			uc, _ := dungXuLy(t, k, hanXuLyThu())
			files := &staffFilesFake{photos: 1, cands: actCandidates()}
			uc.staffFiles = files
			if err := c.run(uc, actFiles); err != nil {
				t.Fatalf("act: %v", err)
			}
			rows := k.cau("INSERT INTO nhat_ky_phan_anh")
			if len(rows) != 1 {
				t.Fatalf("timeline rows = %d", len(rows))
			}
			if files.linkCalled != 1 || files.linkedTo != rows[0].args[1] ||
				strings.Join(files.linkedIDs, ",") != strings.Join(actFiles, ",") {
				t.Errorf("linked %v to %q, want %v to the row just written %v", files.linkedIDs, files.linkedTo, actFiles, rows[0].args[1])
			}
			if k.daCommit != 1 {
				t.Errorf("commit %d", k.daCommit)
			}
			got, _ := actAuditDelta(t, k)["tep_dinh_kem"].([]any)
			if len(got) != 2 || got[0] != "f1" || got[1] != "f2" {
				t.Errorf("tep_dinh_kem = %v", actAuditDelta(t, k)["tep_dinh_kem"])
			}
		})
	}
}

func TestActsRefuseAForeignFileAndWriteNothing(t *testing.T) {
	foreign := map[string]domain.AttachCandidate{
		"f1": petitionLogCandidate("f1", idPhieuThu, "CB-00999", domain.StoredFileStored, ""),
	}
	for _, c := range actCases() {
		t.Run(c.name, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = c.row()
			uc, _ := dungXuLy(t, k, hanXuLyThu())
			files := &staffFilesFake{photos: 1, cands: foreign}
			uc.staffFiles = files
			if err := c.run(uc, []string{"f1"}); !errors.Is(err, domain.ErrPetitionAttachmentNotUsable) {
				t.Fatalf("err = %v", err)
			}
			if k.coCau("UPDATE phieu_phan_anh") || k.coCau("INSERT INTO nhat_ky_phan_anh") ||
				k.coCau("INSERT INTO audit_log") || files.linkCalled != 0 || k.daCommit != 0 {
				t.Error("written although a file was refused")
			}
		})
	}
}

func TestActsRefuseABadListBeforeAnyTransaction(t *testing.T) {
	for _, c := range actCases() {
		for name, ids := range map[string][]string{"duplicate": {"f1", "f1"}, "blank": {" "}} {
			t.Run(c.name+"/"+name, func(t *testing.T) {
				k := khoPhieuMau()
				k.hang = c.row()
				uc, _ := dungXuLy(t, k, hanXuLyThu())
				uc.staffFiles = &staffFilesFake{photos: 1, cands: actCandidates()}
				if err := c.run(uc, ids); !errors.Is(err, domain.ErrAttachmentListInvalid) {
					t.Fatalf("err = %v", err)
				}
				if k.batDau != 0 {
					t.Error("a transaction opened for a refused list")
				}
			})
		}
	}
}

func TestActsWithoutFilesAreUnchanged(t *testing.T) {
	for _, c := range actCases() {
		t.Run(c.name, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = c.row()
			uc, _ := dungXuLy(t, k, hanXuLyThu())
			files := &staffFilesFake{photos: 1}
			uc.staffFiles = files
			if err := c.run(uc, nil); err != nil {
				t.Fatalf("act: %v", err)
			}
			if files.linkCalled != 0 {
				t.Error("linked although no file was offered")
			}
			if _, ok := actAuditDelta(t, k)["tep_dinh_kem"]; ok {
				t.Error("an act without files carries tep_dinh_kem")
			}
		})
	}
}

// THE CLOSE GATE IS NOT SATISFIED BY A LOG ATTACHMENT: switch on, no verification photo, a valid log
// attachment offered — the closing is refused for the photo and nothing is linked.
func TestCloseGateIgnoresLogAttachments(t *testing.T) {
	k := closablePetition()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	files := &staffFilesFake{photos: 0, cands: actCandidates()}
	uc.settings, uc.staffFiles = &closeSwitchFake{required: true}, files
	_, err := uc.Dong(ctx, maPhieuThu, ketQuaThat, "", canBoThu(), khongQuyenHanChe, actFiles...)
	if !errors.Is(err, domain.ErrVerificationPhotoRequired) {
		t.Fatalf("err = %v, want ErrVerificationPhotoRequired", err)
	}
	if files.linkCalled != 0 || k.coCau("UPDATE phieu_phan_anh") || k.daCommit != 0 {
		t.Error("closed or linked although the gate refused")
	}
	if len(files.counted) != 1 || !strings.Contains(files.counted[0], domain.PurposePetitionVerificationPhoto) {
		t.Errorf("gate counted %v — the verification purpose only", files.counted)
	}
}

// Files offered with no file store wired: a wiring fault, never the act recorded without them.
func TestActsWithFilesButNoStoreRefuse(t *testing.T) {
	k := khoPhieuMau()
	k.hang = phieuDaGiaoCho(maCanBoThu)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	uc.staffFiles = nil
	if _, err := uc.TienTrangThai(ctx, maPhieuThu, "", canBoThu(), khongQuyenCaXa, khongQuyenHanChe, "f1"); err == nil {
		t.Fatal("act recorded with its files silently dropped")
	}
	if k.batDau != 0 {
		t.Error("a transaction opened")
	}
}
