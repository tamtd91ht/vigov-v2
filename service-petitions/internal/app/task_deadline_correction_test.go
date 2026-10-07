package app

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Correcting a task's deadline through PATCH (`due_at`, vigov-require 93cff7f, user decision
// 28/09/2026), over the real store on the fake driver. The fixture task NV19 carries han_xu_ly =
// mocHanNV and han_ban_dau = mocHanGocNV.
//
//	PROVED HERE   both branches: no approved extension → `han_ban_dau` follows, written from the SAME
//	              value; an approved extension → only `han_xu_ly` moves · only APPROVED requests count
//	              (pending/rejected do not) and the count does not filter soft-deleted rows · the timeline
//	              row names the branch · the audit entry carries both deadlines before and after · the
//	              instant is stored as sent (no hour added) · a zero instant is refused before any
//	              transaction · a lost race writes nothing that commits.
//	NOT PROVED    that PostgreSQL's trigger (migration 0016) refuses the wrong forms — see
//	              internal/store/task_deadline_correction_pg_test.go (skips without VIGOV_TEST_DSN).

// correctedDue is the corrected deadline, sent with a +07:00 offset — the commune's wall clock — so a
// test can see it is stored as the SAME INSTANT, not re-read as a local hour or moved to 17:00.
var correctedDue = time.Date(2026, 7, 22, 16, 30, 0, 0, time.FixedZone("ICT", 7*3600))

func dueEdit(t time.Time) petstore.SuaNhiemVu { return petstore.SuaNhiemVu{DueAt: &t} }

func withRequest(k *khoNhiemVuGia, status string) {
	fields := map[string]driver.Value{"trang_thai": status}
	if status != "cho-duyet" {
		fields["nguoi_duyet_ma"] = maLanhDao
		fields["duyet_luc"] = mocThaoTacNV
	}
	k.deNghi[idDeNghi] = dongDeNghiGia(fields)
}

func deadlineUpdate(t *testing.T, k *khoNhiemVuGia) lenhPhieu {
	t.Helper()
	hit := k.cau("UPDATE nhiem_vu SET han_xu_ly")
	if len(hit) != 1 {
		t.Fatalf("chạy %d câu sửa hạn, muốn 1", len(hit))
	}
	return hit[0]
}

func TestCorrectDeadline_NoApprovedExtensionOriginalFollows(t *testing.T) {
	for _, pending := range []string{"", "cho-duyet", "tu-choi"} {
		t.Run("đề nghị: "+pending, func(t *testing.T) {
			k := khoNVMau()
			if pending != "" {
				withRequest(k, pending) // a pending or rejected request moved nothing
			}
			uc, ctx := dungGhiNhiemVu(t, k)

			after, err := uc.Sua(ctx, maNVGoc, dueEdit(correctedDue), canBoThu())
			if err != nil {
				t.Fatalf("sửa hạn: %v", err)
			}
			up := deadlineUpdate(t, k)
			if !strings.Contains(up.sql, "han_ban_dau = $3") {
				t.Errorf("chưa có gia hạn được duyệt mà hạn ban đầu không đi theo: %q", up.sql)
			}
			// $3 new, $4 the deadline read under the lock.
			if got, ok := up.args[2].(time.Time); !ok || !got.Equal(correctedDue) {
				t.Errorf("hạn mới = %v, muốn đúng thời điểm gửi %v", up.args[2], correctedDue)
			}
			if got, ok := up.args[3].(time.Time); !ok || !got.Equal(mocHanNV) {
				t.Errorf("hạn cũ trong WHERE = %v, muốn %v", up.args[3], mocHanNV)
			}
			if !after.HanXuLy.Equal(correctedDue) || !after.HanBanDau.Equal(correctedDue) {
				t.Errorf("phản hồi: hạn %v / gốc %v, muốn cả hai %v", after.HanXuLy, after.HanBanDau, correctedDue)
			}
			assertTimelineContains(t, k, "hạn ban đầu đi theo")
			delta := auditDeltaText(t, k)
			newUTC := correctedDue.UTC().Format(time.RFC3339)
			for _, want := range []string{
				`"han_xu_ly":"` + mocHanNV.Format(time.RFC3339) + `"`,
				`"han_ban_dau":"` + mocHanGocNV.Format(time.RFC3339) + `"`,
				`"han_xu_ly":"` + newUTC + `"`,
				`"han_ban_dau":"` + newUTC + `"`,
			} {
				if !strings.Contains(delta, want) {
					t.Errorf("vết kiểm toán thiếu %s: %s", want, delta)
				}
			}
			chiGhiTrongGiaoDich(t, k)
		})
	}
}

func TestCorrectDeadline_AfterApprovedExtensionOnlyCurrentMoves(t *testing.T) {
	k := khoNVMau()
	withRequest(k, "da-duyet")
	uc, ctx := dungGhiNhiemVu(t, k)

	after, err := uc.Sua(ctx, maNVGoc, dueEdit(correctedDue), canBoThu())
	if err != nil {
		t.Fatalf("sửa hạn: %v", err)
	}
	up := deadlineUpdate(t, k)
	if strings.Contains(up.sql, "han_ban_dau") {
		t.Errorf("đã có gia hạn được duyệt mà vẫn đổi hạn ban đầu — §11.3 mất mẫu số: %q", up.sql)
	}
	if !after.HanBanDau.Equal(mocHanGocNV) || !after.HanXuLy.Equal(correctedDue) {
		t.Errorf("phản hồi: hạn %v / gốc %v, muốn %v / %v", after.HanXuLy, after.HanBanDau, correctedDue, mocHanGocNV)
	}
	assertTimelineContains(t, k, "hạn ban đầu giữ nguyên")
	if d := auditDeltaText(t, k); !strings.Contains(d, `"han_ban_dau":"`+mocHanGocNV.Format(time.RFC3339)+`"`) {
		t.Errorf("vết kiểm toán không ghi hạn ban đầu giữ nguyên: %s", d)
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestCorrectDeadline_ApprovedCountIncludesSoftDeleted — the one read of `de_nghi_lui_han` that must
// NOT filter `deleted_at`: an approval moved the deadline whatever happened to the request row, and a
// soft delete must not re-open `han_ban_dau`. Asserted on the statement the store sends.
func TestCorrectDeadline_ApprovedCountIncludesSoftDeleted(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	if _, err := uc.Sua(ctx, maNVGoc, dueEdit(correctedDue), canBoThu()); err != nil {
		t.Fatalf("sửa hạn: %v", err)
	}
	var found bool
	for _, l := range k.cau("FROM de_nghi_lui_han") {
		// The reply's extension facts (store.attachExtensionFacts) name `deleted_at` inside the PENDING
		// item only; their approved item is asserted on its own in TestReplyCarriesExtensionFacts.
		if strings.Contains(l.sql, "GROUP BY nhiem_vu_id") {
			continue
		}
		if strings.Contains(l.sql, "'da-duyet'") {
			found = true
			if strings.Contains(l.sql, "deleted_at") {
				t.Errorf("câu đếm gia hạn đã duyệt lọc xoá mềm: %q", l.sql)
			}
			if !l.trongGiaoDich {
				t.Error("đếm gia hạn đã duyệt NGOÀI giao dịch — có thể quyết định theo trạng thái cũ")
			}
		}
	}
	if !found {
		t.Fatal("không hỏi đã có gia hạn được duyệt chưa")
	}
}

// TestCorrectDeadline_TaskWithoutDeadlineGetsOne — a task created with no deadline can be given one;
// both columns arrive together (the schema's pair CHECK), and the WHERE carries NULL for the old one.
func TestCorrectDeadline_TaskWithoutDeadlineGetsOne(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["han_xu_ly"] = nil
	k.nhiemVu[idNVGoc]["han_ban_dau"] = nil
	uc, ctx := dungGhiNhiemVu(t, k)

	after, err := uc.Sua(ctx, maNVGoc, dueEdit(correctedDue), canBoThu())
	if err != nil {
		t.Fatalf("đặt hạn cho việc chưa có hạn: %v", err)
	}
	up := deadlineUpdate(t, k)
	if !strings.Contains(up.sql, "han_ban_dau = $3") || up.args[3] != nil {
		t.Errorf("sql %q, hạn cũ %v — muốn cả hai cột, WHERE với NULL", up.sql, up.args[3])
	}
	if !after.HanBanDau.Equal(correctedDue) {
		t.Errorf("hạn ban đầu = %v, muốn %v", after.HanBanDau, correctedDue)
	}
	assertTimelineContains(t, k, "Sửa hạn xử lý: — → ")
}

// TestCorrectDeadline_SameInstantIsNoOp — the same instant in another offset, or differing only below
// the column's microsecond, changes nothing and writes nothing (idem.KhongCan on the route).
func TestCorrectDeadline_SameInstantIsNoOp(t *testing.T) {
	for name, due := range map[string]time.Time{
		"khác múi giờ":         mocHanNV.In(time.FixedZone("ICT", 7*3600)),
		"khác dưới micro giây": mocHanNV.Add(300 * time.Nanosecond),
	} {
		t.Run(name, func(t *testing.T) {
			k := khoNVMau()
			uc, ctx := dungGhiNhiemVu(t, k)
			if _, err := uc.Sua(ctx, maNVGoc, dueEdit(due), canBoThu()); err != nil {
				t.Fatalf("gửi lại đúng hạn: %v", err)
			}
			for _, l := range k.lenh {
				if strings.HasPrefix(l.sql, "UPDATE") || strings.HasPrefix(l.sql, "INSERT") {
					t.Errorf("ghi dù hạn không đổi: %q", l.sql)
				}
			}
		})
	}
}

func TestCorrectDeadline_ZeroInstantIs400BeforeTheTransaction(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	_, err := uc.Sua(ctx, maNVGoc, dueEdit(time.Time{}), canBoThu())
	if !errors.Is(err, domain.ErrDueAtEmpty) || !domain.LaLoiDauVaoNhiemVu(err) {
		t.Fatalf("lỗi = %v, muốn ErrDueAtEmpty (400)", err)
	}
	if k.batDau != 0 {
		t.Error("mở giao dịch dù đầu vào sai")
	}
}

// TestCorrectDeadline_RaceIsTaskState — the WHERE carries the deadline read under the lock; an
// extension approved in between moves it, zero rows match, and nothing commits.
func TestCorrectDeadline_RaceIsTaskState(t *testing.T) {
	k := khoNVMau()
	k.doiDong = 0
	uc, ctx := dungGhiNhiemVu(t, k)
	_, err := uc.Sua(ctx, maNVGoc, dueEdit(correctedDue), canBoThu())
	if !errors.Is(err, petstore.ErrNhiemVuDaChuyenTrang) {
		t.Fatalf("lỗi = %v, muốn ErrNhiemVuDaChuyenTrang", err)
	}
	if k.daCommit != 0 {
		t.Error("đã commit dù câu sửa hạn không khớp dòng nào")
	}
}

func assertTimelineContains(t *testing.T, k *khoNhiemVuGia, want string) {
	t.Helper()
	for _, l := range k.cau("INSERT INTO nhat_ky_nhiem_vu") {
		if s, ok := l.args[8].(string); ok && strings.Contains(s, want) {
			return
		}
	}
	t.Errorf("không có dòng nhật ký chứa %q", want)
}
