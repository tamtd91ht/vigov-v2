package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the assignment act (owner decision 28/09/2026), over the REAL store on the fake driver.
//
//	PROVED HERE   the UPDATE, the timeline row and the audit entry share ONE transaction and commit
//	              once · unit/assignee changed -> `moi-giao`, lead/monitor alone -> status kept · the
//	              UPDATE names no `han_*` column and the reply's deadlines are the row's · the timeline
//	              row carries the NEW holder and a from→to sentence · the audit actor is the STAFF
//	              BUSINESS CODE, the subject the register number, the delta both sides · terminal and
//	              no-op refused with nothing written · a staff code identity does not accept (unknown,
//	              another commune, locked) is ONE refusal before any transaction · identity down and
//	              missing wiring refuse as "unchecked" · a race answers the store's sentinel · a status
//	              move into `chuyen-tiep` is refused before any transaction.
//
//	NOT PROVED    whether a unit id exists in the commune — there is no RPC for it (see the header of
//	              task_assignment.go). Anything PostgreSQL does with the statement.

const newAssignee = "CB-00999"

func unitPtr(s string) *string { return &s }

func reassignReq(c domain.TaskAssignmentChange) TaskAssignmentRequest {
	return TaskAssignmentRequest{Change: c}
}

func TestReassign_HolderChangeThreeWritesOneTransaction(t *testing.T) {
	k := khoNVMau() // dang-thuc-hien, bp-vpdu, CB-00311
	uc, ctx := dungGhiNhiemVu(t, k)
	gv := &giaoViecGia{duoc: map[string]struct{}{newAssignee: {}}}
	uc.giaoViec = gv

	after, err := uc.Reassign(ctx, maNVGoc, TaskAssignmentRequest{
		Change: domain.TaskAssignmentChange{Unit: unitPtr("bp-dia-chinh"), Assignee: unitPtr(newAssignee)},
		Note:   "Chuyển theo chỉ đạo tại cuộc họp giao ban.",
	}, canBoThu())
	if err != nil {
		t.Fatalf("giao lại: %v", err)
	}

	for _, stmt := range []string{"UPDATE nhiem_vu", "INSERT INTO nhat_ky_nhiem_vu", "INSERT INTO audit_log"} {
		if n := len(k.cau(stmt)); n != 1 {
			t.Errorf("câu %q chạy %d lần, muốn 1", stmt, n)
		}
	}
	chiGhiTrongGiaoDich(t, k)

	upd := k.cau("UPDATE nhiem_vu")[0]
	if upd.args[2] != "bp-dia-chinh" || upd.args[3] != newAssignee {
		t.Errorf("UPDATE ghi bộ phận/người = %v/%v", upd.args[2], upd.args[3])
	}
	if upd.args[6] != string(domain.MoiGiao) || upd.args[7] != string(domain.DangThucHien) {
		t.Errorf("UPDATE trạng thái sau/đang chờ = %v/%v, muốn moi-giao/dang-thuc-hien", upd.args[6], upd.args[7])
	}
	if after.TrangThai != domain.MoiGiao {
		t.Errorf("phản hồi trạng thái = %q, muốn moi-giao", after.TrangThai)
	}

	// Identity was asked about the code being WRITTEN, in the context's commune, before the tx.
	if gv.goi != 1 || len(gv.daHoi[0]) != 1 || gv.daHoi[0][0] != newAssignee || gv.xa[0] != xaThu {
		t.Errorf("identity được hỏi %v trong xã %v", gv.daHoi, gv.xa)
	}

	log := k.cau("INSERT INTO nhat_ky_nhiem_vu")[0]
	if log.args[4] != maCanBoThu {
		t.Errorf("người ghi nhật ký = %v, muốn mã cán bộ %q", log.args[4], maCanBoThu)
	}
	if log.args[5] != string(domain.MoiGiao) || log.args[6] != "bp-dia-chinh" || log.args[7] != newAssignee {
		t.Errorf("dòng nhật ký không mang người giữ việc MỚI: %v", log.args)
	}
	text, _ := log.args[8].(string)
	for _, want := range []string{"CB-00311 → " + newAssignee, "bp-vpdu → bp-dia-chinh", "giao ban"} {
		if !strings.Contains(text, want) {
			t.Errorf("nội dung nhật ký %q thiếu %q", text, want)
		}
	}

	vet := vetKiemToan(t, k)
	if vet.args[1] != maCanBoThu || vet.args[4] != ActionTaskAssignment || vet.args[5] != maNVGoc {
		t.Errorf("vết: chủ thể=%v hành vi=%v đối tượng=%v", vet.args[1], vet.args[4], vet.args[5])
	}
	delta := auditDeltaText(t, k)
	for _, want := range []string{`"nguoi_thuc_hien_ma":"CB-00311"`, `"nguoi_thuc_hien_ma":"` + newAssignee + `"`,
		`"trang_thai":"dang-thuc-hien"`, `"trang_thai":"moi-giao"`, `"dat_lai_trang_thai":true`} {
		if !strings.Contains(delta, want) {
			t.Errorf("delta thiếu %s: %s", want, delta)
		}
	}
	// The note's TEXT is on the timeline, never in the permanent audit entry (rule 3).
	if strings.Contains(delta, "giao ban") {
		t.Errorf("ghi chú tự do lọt vào vết kiểm toán: %s", delta)
	}
}

func TestReassign_LeadAndMonitorOnlyKeepStatus(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}

	after, err := uc.Reassign(ctx, maNVGoc, reassignReq(domain.TaskAssignmentChange{
		LeadUnit: unitPtr("bp-tu-phap"), Monitor: unitPtr("CB-00500")}), canBoThu())
	if err != nil {
		t.Fatalf("giao lại: %v", err)
	}
	upd := k.cau("UPDATE nhiem_vu")[0]
	if upd.args[6] != string(domain.DangThucHien) {
		t.Errorf("trạng thái sau = %v, muốn giữ dang-thuc-hien (#7)", upd.args[6])
	}
	if after.TrangThai != domain.DangThucHien || after.BoPhanID != "bp-vpdu" {
		t.Errorf("phản hồi: %q, bộ phận %q", after.TrangThai, after.BoPhanID)
	}
	if !strings.Contains(auditDeltaText(t, k), `"dat_lai_trang_thai":false`) {
		t.Error("delta không nói trạng thái KHÔNG bị đặt lại")
	}
	chiGhiTrongGiaoDich(t, k)
}

func TestReassign_DeadlineUntouched(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	after, err := uc.Reassign(ctx, maNVGoc,
		reassignReq(domain.TaskAssignmentChange{Unit: unitPtr("bp-dia-chinh")}), canBoThu())
	if err != nil {
		t.Fatalf("giao lại: %v", err)
	}
	// THE STATEMENT NAMES NO DEADLINE COLUMN, so no value any caller passes could move one.
	for _, l := range k.lenh {
		if !strings.HasPrefix(l.sql, "UPDATE") {
			continue
		}
		// The column NAMES, not a `han_` prefix — `nguoi_thuc_hien_ma` contains that substring.
		for _, col := range []string{"han_xu_ly", "han_ban_dau", "ngay_hoan_thanh"} {
			if strings.Contains(l.sql, col) {
				t.Errorf("câu ghi của thao tác giao lại chạm cột %s: %q", col, l.sql)
			}
		}
	}
	if !after.HanXuLy.Equal(mocHanNV) || !after.HanBanDau.Equal(mocHanGocNV) {
		t.Errorf("hạn trong phản hồi = %v/%v, muốn giữ %v/%v", after.HanXuLy, after.HanBanDau, mocHanNV, mocHanGocNV)
	}
}

func TestReassign_TerminalRefusedWritesNothing(t *testing.T) {
	for _, status := range []domain.TrangThaiNhiemVu{domain.HoanThanh, domain.ChuyenTiep} {
		t.Run(string(status), func(t *testing.T) {
			k := khoNVMau()
			k.nhiemVu[idNVGoc]["trang_thai"] = string(status)
			uc, ctx := dungGhiNhiemVu(t, k)

			_, err := uc.Reassign(ctx, maNVGoc,
				reassignReq(domain.TaskAssignmentChange{Unit: unitPtr("bp-dia-chinh")}), canBoThu())
			if !errors.Is(err, domain.ErrTaskClosedForAssignment) {
				t.Fatalf("lỗi = %v, muốn ErrTaskClosedForAssignment", err)
			}
			khongGhiGi(t, k)
		})
	}
}

func TestReassign_NoChangeWritesNothing(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	_, err := uc.Reassign(ctx, maNVGoc,
		reassignReq(domain.TaskAssignmentChange{Unit: unitPtr("bp-vpdu"), LeadUnit: unitPtr("bp-vpdu")}), canBoThu())
	if !errors.Is(err, domain.ErrAssignmentNoChange) {
		t.Fatalf("lỗi = %v, muốn ErrAssignmentNoChange", err)
	}
	khongGhiGi(t, k)
}

// ONE refusal for unknown, another commune and locked — identity collapses them into "absent", and this
// layer must not tell them apart (rule 1: a code of another commune must not be seen to exist).
func TestReassign_ForeignOrUnknownStaffRefusedIdentically(t *testing.T) {
	for _, name := range []string{"không tồn tại", "thuộc xã khác", "đã khoá"} {
		t.Run(name, func(t *testing.T) {
			k := khoNVMau()
			uc, ctx := dungGhiNhiemVu(t, k)
			uc.giaoViec = &giaoViecGia{duoc: map[string]struct{}{"CB-00001": {}}}

			_, err := uc.Reassign(ctx, maNVGoc,
				reassignReq(domain.TaskAssignmentChange{Assignee: unitPtr(newAssignee)}), canBoThu())
			if !errors.Is(err, ErrAssignmentStaffInvalid) || errors.Is(err, ErrAssignmentStaffUnchecked) {
				t.Fatalf("lỗi = %v, muốn đúng ErrAssignmentStaffInvalid", err)
			}
			khongMoGiaoDich(t, k)
		})
	}
	// The MONITOR is checked the same way.
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duoc: map[string]struct{}{}}
	_, err := uc.Reassign(ctx, maNVGoc,
		reassignReq(domain.TaskAssignmentChange{Monitor: unitPtr("CB-00500")}), canBoThu())
	if !errors.Is(err, ErrAssignmentStaffInvalid) {
		t.Fatalf("chuyên viên theo dõi không hợp lệ: lỗi = %v", err)
	}
	khongMoGiaoDich(t, k)
}

func TestReassign_IdentityDownOrUnwiredRefusesUnchecked(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{loi: errors.New("rpc error: code = Unavailable")}
	_, err := uc.Reassign(ctx, maNVGoc,
		reassignReq(domain.TaskAssignmentChange{Assignee: unitPtr(newAssignee)}), canBoThu())
	if !errors.Is(err, ErrAssignmentStaffUnchecked) || errors.Is(err, ErrAssignmentStaffInvalid) {
		t.Fatalf("identity hỏng: lỗi = %v, muốn đúng ErrAssignmentStaffUnchecked", err)
	}
	khongMoGiaoDich(t, k)

	k = khoNVMau()
	uc, ctx = dungGhiNhiemVu(t, k) // giaoViec nil
	_, err = uc.Reassign(ctx, maNVGoc,
		reassignReq(domain.TaskAssignmentChange{Assignee: unitPtr(newAssignee)}), canBoThu())
	if !errors.Is(err, ErrAssignmentStaffUnchecked) {
		t.Fatalf("chưa nối dây: lỗi = %v, muốn ErrAssignmentStaffUnchecked (fail closed)", err)
	}
	khongMoGiaoDich(t, k)
}

func TestReassign_RaceAnswersStoreSentinel(t *testing.T) {
	k := khoNVMau()
	k.doiDong = 0 // somebody moved the row between the locking read and the UPDATE
	uc, ctx := dungGhiNhiemVu(t, k)
	_, err := uc.Reassign(ctx, maNVGoc,
		reassignReq(domain.TaskAssignmentChange{Unit: unitPtr("bp-dia-chinh")}), canBoThu())
	if !errors.Is(err, petstore.ErrNhiemVuDaChuyenTrang) {
		t.Fatalf("lỗi = %v, muốn ErrNhiemVuDaChuyenTrang", err)
	}
	if k.daCommit != 0 || k.coCau("INSERT INTO audit_log") {
		t.Error("đã commit hoặc ghi vết dù câu UPDATE không khớp dòng nào")
	}
}

// ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: a status move into `chuyen-tiep` used to be accepted from every
// working state. It is now refused with the sentence naming the assignment act, BEFORE any transaction.
func TestStatusMoveIntoForwardingRefused(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	_, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.ChuyenTiep)}, canBoThu(), true)
	if !errors.Is(err, domain.ErrForwardingIsAssignment) {
		t.Fatalf("lỗi = %v, muốn ErrForwardingIsAssignment", err)
	}
	khongMoGiaoDich(t, k)
}
