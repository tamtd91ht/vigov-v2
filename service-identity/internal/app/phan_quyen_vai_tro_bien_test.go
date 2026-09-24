package app

import (
	"errors"
	"testing"
)

// AN AUDIT FAILURE TAKES THE GRANT CHANGE DOWN WITH IT (rule 6, invariant 3; ADR 0040).
//
// ADR 0040 accepted a HARD delete of grant cells on one condition: the audit entry naming every
// removed key commits in the same transaction. TestLuuPhanQuyenGhiVetCungGiaoDich proves they SHARE a
// transaction when all goes well; it cannot tell a use case that propagates the audit error from one
// that swallows it — both commit in the happy path. This is the case where they differ: cells gone,
// no record of who removed them.
//
// MUTATION THAT MUST TURN THIS RED: `_ = audit.Write(...); return nil` in PhanQuyenVaiTro.Luu.
func TestLuuPhanQuyenVetHongThiRollbackCaThemLanBo(t *testing.T) {
	b := dungBanThuPhanQuyen(t)
	b.ghi.loiTheo["audit_log"] = errors.New("ghi vết hỏng")

	if _, err := b.uc.Luu(ctxXa(xaThu), vaiTroDich, []string{"task.read"}, nguoiThucHienGia()); err == nil {
		t.Fatal("ghi vết hỏng mà lưu phân quyền vẫn báo thành công")
	}
	them, bo := b.ghi.tim("them-cap"), b.ghi.tim("bo-cap")
	if them == nil || bo == nil {
		t.Fatal("hai câu ghi phải đã chạy trước vết — không thì phép thử không phân biệt rollback với chưa chạy")
	}
	for ten, l := range map[string]*lenhGhi{"thêm ô": them, "gỡ ô": bo} {
		if got := b.ghi.ketThucCua(l.tx); got != "rollback" {
			t.Errorf("%s: giao dịch kết thúc bằng %q, muốn rollback — ô đã xoá cứng mà không có vết", ten, got)
		}
	}
}
