package store

import (
	"errors"
	"testing"
)

// The unique-violation mapping is matched on the PARTITION's constraint name, which is the only name
// PostgreSQL reports for a hash-partitioned table (see translateRoleInsertError). The staff-code key
// of `nguoi_dung` shares the suffix and must NOT map to ErrRoleCodeTaken; any other error must pass
// through wrapped, never become a business refusal.
func TestTranslateRoleInsertError(t *testing.T) {
	dup := errors.New(`pq: duplicate key value violates unique constraint "vai_tro_p07_tenant_id_ma_key"`)
	if !errors.Is(translateRoleInsertError(dup), ErrRoleCodeTaken) {
		t.Error("vi phạm khoá (tenant_id, ma) của vai_tro không thành ErrRoleCodeTaken")
	}

	staff := errors.New(`pq: duplicate key value violates unique constraint "nguoi_dung_p07_tenant_id_ma_key"`)
	if errors.Is(translateRoleInsertError(staff), ErrRoleCodeTaken) {
		t.Error("khoá mã cán bộ của nguoi_dung bị đọc nhầm thành vai trò đã có")
	}

	conn := errors.New("driver: bad connection")
	got := translateRoleInsertError(conn)
	if errors.Is(got, ErrRoleCodeTaken) || !errors.Is(got, conn) {
		t.Errorf("lỗi kết nối phải được bọc nguyên vẹn, nhận %v", got)
	}
}
