package app

import (
	"database/sql/driver"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The write replies carry `extension_count` and `pending_extension` (store.attachExtensionFacts),
// through the same AttachTreeFactsTx the tree facts use — inside the act's transaction.
//
//	PROVED HERE   a status reply and a PATCH reply carry both facts · the approved count includes a
//	              SOFT-DELETED approval (migration 0016's predicate) · a withdrawn pending request is not
//	              pending · ONE statement per reply, inside the transaction · the approved item never
//	              names `deleted_at` · a task with no request reads 0 / false.
//	NOT PROVED    what PostgreSQL does with the FILTER clauses — store/task_extension_facts_pg_test.go.

// withExtensionRows files, against the root task: one live approval, one SOFT-DELETED approval, one
// withdrawn (soft-deleted) pending request and one rejected request — so exactly one fact of each kind
// is true only under the right predicate.
func withExtensionRows(k *khoNhiemVuGia, livePending bool) {
	decided := map[string]driver.Value{"nguoi_duyet_ma": maLanhDao, "duyet_luc": mocThaoTacNV}
	row := func(id, status string, deleted bool) {
		f := map[string]driver.Value{"id": id, "trang_thai": status}
		if status != "cho-duyet" {
			for col, v := range decided {
				f[col] = v
			}
		}
		if deleted {
			f["deleted_at"] = mocThaoTacNV
		}
		k.deNghi[id] = dongDeNghiGia(f)
	}
	row("dn-approved-live", "da-duyet", false)
	row("dn-approved-deleted", "da-duyet", true)
	row("dn-pending-withdrawn", "cho-duyet", true)
	row("dn-rejected", "tu-choi", false)
	if livePending {
		row("dn-pending-live", "cho-duyet", false)
	}
}

func assertOneExtensionFactsStatementInTx(t *testing.T, k *khoNhiemVuGia) {
	t.Helper()
	hits := k.cau("GROUP BY nhiem_vu_id")
	if len(hits) != 1 {
		t.Fatalf("chạy %d câu đọc số lần lùi hạn cho một phản hồi, muốn 1", len(hits))
	}
	l := hits[0]
	if !l.trongGiaoDich {
		t.Error("đọc số lần lùi hạn NGOÀI giao dịch — phản hồi có thể tả một trạng thái khác")
	}
	start := strings.Index(l.sql, "count(*) FILTER (WHERE trang_thai = 'da-duyet'")
	if start < 0 {
		t.Fatalf("không thấy mục đếm đã duyệt: %q", l.sql)
	}
	approvedItem := l.sql[start:]
	approvedItem = approvedItem[:strings.Index(approvedItem, "')")+2]
	if strings.Contains(approvedItem, "deleted_at") {
		t.Errorf("số lần lùi hạn lọc xoá mềm — lệch với trigger 0016: %q", approvedItem)
	}
	if !strings.Contains(l.sql, "tenant_id = $1") {
		t.Errorf("câu không buộc xã: %q", l.sql)
	}
}

func TestReplyCarriesExtensionFacts_StatusMove(t *testing.T) {
	k := khoNVMau()
	withExtensionRows(k, true)
	uc, ctx := dungGhiNhiemVu(t, k)

	after, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet)}, canBoThu(), false, true)
	if err != nil {
		t.Fatalf("đổi trạng thái: %v", err)
	}
	if after.ExtensionCount != 2 || !after.PendingExtension {
		t.Errorf("phản hồi: extension_count=%d pending=%v, muốn 2 (kể cả đã xoá mềm) và true",
			after.ExtensionCount, after.PendingExtension)
	}
	assertOneExtensionFactsStatementInTx(t, k)
}

func TestReplyCarriesExtensionFacts_PatchWithdrawnIsNotPending(t *testing.T) {
	k := khoNVMau()
	withExtensionRows(k, false)
	uc, ctx := dungGhiNhiemVu(t, k)

	after, err := uc.Sua(ctx, maNVGoc, dueEdit(correctedDue), canBoThu())
	if err != nil {
		t.Fatalf("sửa: %v", err)
	}
	if after.ExtensionCount != 2 || after.PendingExtension {
		t.Errorf("phản hồi: extension_count=%d pending=%v, muốn 2 và false (đề nghị chờ đã rút)",
			after.ExtensionCount, after.PendingExtension)
	}
	assertOneExtensionFactsStatementInTx(t, k)
}

func TestReplyCarriesExtensionFacts_NoRequestIsZero(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	after, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet)}, canBoThu(), false, true)
	if err != nil {
		t.Fatalf("đổi trạng thái: %v", err)
	}
	if after.ExtensionCount != 0 || after.PendingExtension {
		t.Errorf("phản hồi: extension_count=%d pending=%v, muốn 0 và false", after.ExtensionCount, after.PendingExtension)
	}
}
