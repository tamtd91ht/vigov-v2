package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// A status move carrying `handover` and/or `attachments` (user decision 07/10/2026), over the REAL
// store on the fake driver. Fixture task: `dang-thuc-hien`, unit bp-vpdu, assignee CB-00311.
//
//	PROVED HERE   the task lands in the CHOSEN status, never `moi-giao` · task.assign holder and the
//	              current assignee (without task.assign) may hand over; anybody else 403, nothing written
//	              · a finished task refused as Reassign · unit/assignee not accepted refused BEFORE any
//	              transaction · files linked to the STATUS row, never the handover row · a foreign or
//	              another uploader's file refused, nothing written · any failing step rolls back all ·
//	              two audit entries, actor = staff business code · a no-op handover writes nothing of
//	              its own · a plain move files the entry it filed before.
//	NOT PROVED    migration 0021's link trigger (stored_file_pg_test.go, needs VIGOV_TEST_DSN).

const handoverStatusLogID = "01JSINHRATRONGTEST000002" // 2nd id: the handover row takes the 1st

func handoverTo(unit, assignee string) *TaskAssignmentRequest {
	return &TaskAssignmentRequest{
		Change: domain.TaskAssignmentChange{Unit: unitPtr(unit), Assignee: unitPtr(assignee)},
		Note:   "Chuyển sang địa chính theo giao ban.",
	}
}

func lineText(l lenhPhieu) string {
	var b strings.Builder
	for _, a := range l.args {
		switch v := a.(type) {
		case []byte:
			b.Write(v)
		case string:
			b.WriteString(v)
		}
		b.WriteString(" ")
	}
	return b.String()
}

func TestStatusHandover_AssignHolderLandsInChosenStatus(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}

	sau, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
		TrangThai: string(domain.ChoDuyet), GhiChu: "Đã xong phần hồ sơ.",
		Handover: handoverTo("bp-dia-chinh", newAssignee), AssignRight: true,
	}, staffActor(outsiderCode), false, true)
	if err != nil {
		t.Fatalf("chuyển trạng thái kèm giao việc: %v", err)
	}
	if sau.TrangThai != domain.ChoDuyet || sau.NguoiThucHienMa != newAssignee || sau.BoPhanID != "bp-dia-chinh" {
		t.Errorf("sau = %s / %s / %s, muốn cho-duyet / %s / bp-dia-chinh",
			sau.TrangThai, sau.NguoiThucHienMa, sau.BoPhanID, newAssignee)
	}
	chiGhiTrongGiaoDich(t, k)

	upd := k.cau("UPDATE nhiem_vu")
	if len(upd) != 2 {
		t.Fatalf("chạy %d câu UPDATE nhiem_vu, muốn 2 (giao việc, rồi trạng thái)", len(upd))
	}
	// The handover UPDATE keeps the status — never moi-giao.
	if upd[0].args[4] != string(domain.DangThucHien) || upd[0].args[5] != string(domain.DangThucHien) {
		t.Errorf("UPDATE giao việc ghi trạng thái %v (chờ %v), muốn giữ dang-thuc-hien", upd[0].args[4], upd[0].args[5])
	}
	if upd[1].args[2] != string(domain.ChoDuyet) {
		t.Errorf("UPDATE trạng thái sang %v, muốn cho-duyet", upd[1].args[2])
	}
	for _, u := range upd {
		for _, a := range u.args {
			if a == string(domain.MoiGiao) {
				t.Errorf("nhiệm vụ bị đặt về moi-giao: %v", u.args)
			}
		}
	}

	logs := k.cau("INSERT INTO nhat_ky_nhiem_vu")
	if len(logs) != 2 {
		t.Fatalf("ghi %d dòng nhật ký, muốn 2", len(logs))
	}
	if txt, _ := logs[0].args[8].(string); !strings.Contains(txt, maNguoiThucHien+" → "+newAssignee) ||
		!strings.Contains(txt, "giao ban") {
		t.Errorf("dòng giao việc = %q", txt)
	}
	if logs[1].args[5] != string(domain.ChoDuyet) || logs[1].args[7] != newAssignee {
		t.Errorf("dòng trạng thái không mang trạng thái/người giữ MỚI: %v", logs[1].args)
	}

	vets := k.cau("INSERT INTO audit_log")
	if len(vets) != 2 {
		t.Fatalf("ghi %d vết, muốn 2", len(vets))
	}
	if vets[0].args[4] != ActionTaskAssignment || vets[1].args[4] != HanhViChuyenTrangNhiemVu {
		t.Errorf("hành vi = %v, %v", vets[0].args[4], vets[1].args[4])
	}
	for _, v := range vets {
		if v.args[1] != outsiderCode || v.args[5] != maNVGoc {
			t.Errorf("vết: chủ thể=%v đối tượng=%v, muốn mã cán bộ %s / %s", v.args[1], v.args[5], outsiderCode, maNVGoc)
		}
	}
	d0, d1 := lineText(vets[0]), lineText(vets[1])
	for _, want := range []string{`"dat_lai_trang_thai":false`, `"kem_chuyen_trang_thai":"cho-duyet"`,
		`"quyen_giao":"giao-nhiem-vu"`, `"nguoi_thuc_hien_ma":"` + newAssignee + `"`} {
		if !strings.Contains(d0, want) {
			t.Errorf("vết giao việc thiếu %s: %s", want, d0)
		}
	}
	if !strings.Contains(d1, `"kem_giao_lai":true`) {
		t.Errorf("vết trạng thái thiếu kem_giao_lai: %s", d1)
	}
	if strings.Contains(d0+d1, "giao ban") || strings.Contains(d0+d1, "hồ sơ") {
		t.Errorf("văn bản tự do lọt vào vết: %s | %s", d0, d1)
	}
}

func TestStatusHandover_CurrentAssigneeWithoutAssignKeyMayHandOver(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}

	sau, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
		TrangThai: string(domain.ChoDuyet), Handover: handoverTo("bp-dia-chinh", newAssignee),
	}, staffActor(maNguoiThucHien), false, false)
	if err != nil {
		t.Fatalf("người thực hiện giao tiếp việc: %v", err)
	}
	if sau.TrangThai != domain.ChoDuyet || sau.NguoiThucHienMa != newAssignee {
		t.Errorf("sau = %s / %s", sau.TrangThai, sau.NguoiThucHienMa)
	}
	if d := lineText(k.cau("INSERT INTO audit_log")[0]); !strings.Contains(d, `"quyen_giao":"nguoi-thuc-hien"`) {
		t.Errorf("vết giao việc không nêu cửa người thực hiện: %s", d)
	}
	chiGhiTrongGiaoDich(t, k)
}

func TestStatusHandover_OthersRefused(t *testing.T) {
	for _, c := range []struct {
		name   string
		code   string
		update TaskUpdateRight
		want   error
	}{
		// task.update lets them move the status, but neither task.assign nor being the assignee.
		{"lãnh đạo giao việc có task.update", maLanhDao, true, domain.ErrHandoverNotAllowed},
		{"cán bộ có task.update", outsiderCode, true, domain.ErrHandoverNotAllowed},
		// Not even the status move is theirs — the holder rule answers first.
		{"chỉ có task.read", outsiderCode, false, domain.ErrStatusNeedsHolder},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			uc, ctx := dungGhiNhiemVu(t, k)
			uc.giaoViec = &giaoViecGia{duocTatCa: true}
			_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
				TrangThai: string(domain.ChoDuyet), Handover: handoverTo("bp-dia-chinh", newAssignee),
			}, staffActor(c.code), true, c.update)
			if !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
			khongGhiGi(t, k)
		})
	}
}

// The new assignee is identity-checked, but who MAY hand over is the row's CURRENT assignee: naming
// yourself as the new assignee grants nothing.
func TestStatusHandover_PreviousOrFutureHolderIsNotTheAssignee(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
		TrangThai: string(domain.ChoDuyet), Handover: handoverTo("bp-dia-chinh", outsiderCode),
	}, staffActor(outsiderCode), false, true)
	if !errors.Is(err, domain.ErrHandoverNotAllowed) {
		t.Fatalf("lỗi = %v, muốn ErrHandoverNotAllowed", err)
	}
	khongGhiGi(t, k)
}

func TestStatusHandover_FinishedTaskRefusedAsReassign(t *testing.T) {
	k := khoNVHoanThanh()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
		TrangThai: string(domain.DangThucHien), GhiChu: "Mở lại để bổ sung số liệu.",
		Handover: handoverTo("bp-dia-chinh", newAssignee), AssignRight: true,
	}, staffActor(outsiderCode), true, true)
	if !errors.Is(err, domain.ErrTaskClosedForAssignment) {
		t.Fatalf("lỗi = %v, muốn ErrTaskClosedForAssignment", err)
	}
	khongGhiGi(t, k)
}

func TestStatusHandover_IdentityRefusalsBeforeTransaction(t *testing.T) {
	t.Run("người nhận không hợp lệ", func(t *testing.T) {
		k := khoNVMau()
		uc, ctx := dungGhiNhiemVu(t, k)
		uc.giaoViec = &giaoViecGia{duoc: map[string]struct{}{"CB-00001": {}}}
		_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
			TrangThai: string(domain.ChoDuyet), Handover: handoverTo("bp-dia-chinh", newAssignee), AssignRight: true,
		}, staffActor(maNguoiThucHien), false, false)
		if !errors.Is(err, ErrAssignmentStaffInvalid) {
			t.Fatalf("lỗi = %v, muốn ErrAssignmentStaffInvalid", err)
		}
		khongMoGiaoDich(t, k)
	})
	t.Run("bộ phận không còn trong xã", func(t *testing.T) {
		k := khoNVMau()
		uc, ctx := dungGhiNhiemVu(t, k)
		uc.giaoViec = &giaoViecGia{duocTatCa: true}
		uc.orgUnits = &orgUnitsFake{}
		_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
			TrangThai: string(domain.ChoDuyet), Handover: handoverTo("bp-da-xoa", newAssignee), AssignRight: true,
		}, staffActor(maNguoiThucHien), false, false)
		if !errors.Is(err, ErrOrgUnitNotLive) {
			t.Fatalf("lỗi = %v, muốn ErrOrgUnitNotLive", err)
		}
		khongMoGiaoDich(t, k)
	})
	t.Run("identity không trả lời", func(t *testing.T) {
		k := khoNVMau()
		uc, ctx := dungGhiNhiemVu(t, k) // giaoViec nil: fail closed
		_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
			TrangThai: string(domain.ChoDuyet), Handover: handoverTo("bp-dia-chinh", newAssignee), AssignRight: true,
		}, staffActor(maNguoiThucHien), false, false)
		if !errors.Is(err, ErrAssignmentStaffUnchecked) {
			t.Fatalf("lỗi = %v, muốn ErrAssignmentStaffUnchecked", err)
		}
		khongMoGiaoDich(t, k)
	})
	t.Run("thân giao việc rỗng", func(t *testing.T) {
		k := khoNVMau()
		uc, ctx := dungGhiNhiemVu(t, k)
		_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
			TrangThai: string(domain.ChoDuyet), Handover: &TaskAssignmentRequest{},
		}, staffActor(maNguoiThucHien), false, false)
		if !errors.Is(err, domain.ErrAssignmentEmpty) {
			t.Fatalf("lỗi = %v, muốn ErrAssignmentEmpty", err)
		}
		khongMoGiaoDich(t, k)
	})
}

func TestStatusAttachments_LinkedToTheStatusRow(t *testing.T) {
	t.Run("không giao việc", func(t *testing.T) {
		k := khoNVMau()
		k.addStoredFile(storedFileRow(string(xaThu), fileA, idNVGoc, maNguoiThucHien, domain.StoredFileStored, mocTaoNV))
		uc, ctx := dungGhiNhiemVu(t, k)
		if _, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
			TrangThai: string(domain.ChoDuyet), Attachments: []string{fileA},
		}, staffActor(maNguoiThucHien), false, false); err != nil {
			t.Fatalf("chuyển trạng thái kèm tệp: %v", err)
		}
		// The only id minted is the status row's.
		if k.fileLinks[fileA] != idMoiSinh {
			t.Errorf("tệp gắn vào %q, muốn dòng trạng thái %q", k.fileLinks[fileA], idMoiSinh)
		}
		d := auditDeltaText(t, k)
		if !strings.Contains(d, `"tep_dinh_kem":["`+fileA+`"]`) || !strings.Contains(d, `"nhat_ky_id":"`+idMoiSinh+`"`) {
			t.Errorf("vết không nêu tệp/dòng nhật ký: %s", d)
		}
		if strings.Contains(d, "Biên bản") {
			t.Errorf("tên tệp lọt vào vết: %s", d)
		}
		chiGhiTrongGiaoDich(t, k)
	})
	t.Run("kèm giao việc", func(t *testing.T) {
		k := khoNVMau()
		k.addStoredFile(storedFileRow(string(xaThu), fileA, idNVGoc, maNguoiThucHien, domain.StoredFileStored, mocTaoNV))
		uc, ctx := dungGhiNhiemVu(t, k)
		uc.giaoViec = &giaoViecGia{duocTatCa: true}
		if _, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
			TrangThai: string(domain.ChoDuyet), Attachments: []string{fileA},
			Handover: handoverTo("bp-dia-chinh", newAssignee),
		}, staffActor(maNguoiThucHien), false, false); err != nil {
			t.Fatalf("chuyển trạng thái kèm giao việc và tệp: %v", err)
		}
		if k.fileLinks[fileA] != handoverStatusLogID {
			t.Errorf("tệp gắn vào %q, muốn dòng TRẠNG THÁI %q (không phải dòng giao việc)",
				k.fileLinks[fileA], handoverStatusLogID)
		}
		chiGhiTrongGiaoDich(t, k)
	})
}

func TestStatusAttachments_UnusableFileRefusedWritesNothing(t *testing.T) {
	for _, c := range []struct {
		name     string
		row      map[string]any
		handover bool
	}{
		{"tệp của người khác tải", map[string]any{"uploaded_by": "CB-00412"}, false},
		{"tệp của nhiệm vụ khác", map[string]any{"subject_id": idNVCon}, false},
		{"tệp của xã khác", map[string]any{"tenant_id": "01JB" + strings.Repeat("B", 22)}, false},
		{"tệp của người khác tải, kèm giao việc", map[string]any{"uploaded_by": "CB-00412"}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			r := storedFileRow(string(xaThu), fileA, idNVGoc, maNguoiThucHien, domain.StoredFileStored, mocTaoNV)
			for col, v := range c.row {
				r[col] = v
			}
			k.addStoredFile(r)
			uc, ctx := dungGhiNhiemVu(t, k)
			uc.giaoViec = &giaoViecGia{duocTatCa: true}
			yc := YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet), Attachments: []string{fileA}}
			if c.handover {
				yc.Handover = handoverTo("bp-dia-chinh", newAssignee)
			}
			_, err := uc.DoiTrangThai(ctx, maNVGoc, yc, staffActor(maNguoiThucHien), false, false)
			if !errors.Is(err, domain.ErrAttachmentNotUsable) {
				t.Fatalf("lỗi = %v, muốn ErrAttachmentNotUsable", err)
			}
			khongGhiGi(t, k)
		})
	}
}

func TestStatusAttachments_BadListOrUnwiredNeverOpensATransaction(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
		TrangThai: string(domain.ChoDuyet), Attachments: []string{fileA, fileA},
	}, staffActor(maNguoiThucHien), false, false)
	if !errors.Is(err, domain.ErrAttachmentListInvalid) {
		t.Fatalf("lỗi = %v, muốn ErrAttachmentListInvalid", err)
	}
	khongMoGiaoDich(t, k)

	k = khoNVMau()
	uc, ctx = dungGhiNhiemVu(t, k)
	uc.files = nil
	if _, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
		TrangThai: string(domain.ChoDuyet), Attachments: []string{fileA},
	}, staffActor(maNguoiThucHien), false, false); !errors.Is(err, errAttachmentsNotWired) {
		t.Fatalf("lỗi = %v, muốn errAttachmentsNotWired", err)
	}
	khongMoGiaoDich(t, k)
}

// ONE TRANSACTION: whichever step fails — the handover's audit entry, the status UPDATE, the file
// link — nothing commits.
func TestStatusHandover_AnyFailingStepRollsBackEverything(t *testing.T) {
	for _, failing := range []string{
		"INSERT INTO audit_log",           // the handover's entry, the first one written
		"SET trang_thai = $3",             // the status UPDATE, after the handover's three writes
		"INSERT INTO task_log_attachment", // the link, after both rows
	} {
		t.Run(failing, func(t *testing.T) {
			k := khoNVMau()
			k.addStoredFile(storedFileRow(string(xaThu), fileA, idNVGoc, maNguoiThucHien, domain.StoredFileStored, mocTaoNV))
			k.loiSau = failing
			uc, ctx := dungGhiNhiemVu(t, k)
			uc.giaoViec = &giaoViecGia{duocTatCa: true}
			_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
				TrangThai: string(domain.ChoDuyet), Attachments: []string{fileA},
				Handover: handoverTo("bp-dia-chinh", newAssignee),
			}, staffActor(maNguoiThucHien), false, false)
			if err == nil {
				t.Fatal("bước hỏng mà không lỗi")
			}
			if !k.daNo {
				t.Fatalf("câu %q không chạy — phép thử không thử gì", failing)
			}
			if k.daCommit != 0 || k.daRollback != 1 {
				t.Errorf("commit %d, rollback %d — muốn 0 và 1", k.daCommit, k.daRollback)
			}
		})
	}
}

// A handover naming the holder the task already has writes nothing of its own; the move goes on.
func TestStatusHandover_NoChangeWritesOnlyTheMove(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	if _, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{
		TrangThai: string(domain.ChoDuyet), Handover: handoverTo("bp-vpdu", maNguoiThucHien),
	}, staffActor(maNguoiThucHien), false, false); err != nil {
		t.Fatalf("giao việc không đổi gì: %v", err)
	}
	for _, stmt := range []string{"UPDATE nhiem_vu", "INSERT INTO nhat_ky_nhiem_vu", "INSERT INTO audit_log"} {
		if n := len(k.cau(stmt)); n != 1 {
			t.Errorf("câu %q chạy %d lần, muốn 1", stmt, n)
		}
	}
	if d := auditDeltaText(t, k); strings.Contains(d, "kem_giao_lai") || !strings.Contains(d, HanhViChuyenTrangNhiemVu) {
		t.Errorf("vết = %s", d)
	}
}

// Neither field: the entry a plain move files carries none of the new keys.
func TestStatusMove_WithoutExtrasAuditUnchanged(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	if _, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet)},
		staffActor(maNguoiThucHien), false, false); err != nil {
		t.Fatalf("chuyển trạng thái: %v", err)
	}
	d := auditDeltaText(t, k)
	for _, key := range []string{"kem_giao_lai", "nhat_ky_id", "tep_dinh_kem"} {
		if strings.Contains(d, key) {
			t.Errorf("vết của lần chuyển thường có khoá mới %s: %s", key, d)
		}
	}
	if n := len(k.cau("UPDATE nhiem_vu")); n != 1 {
		t.Errorf("chạy %d câu UPDATE, muốn 1", n)
	}
}
