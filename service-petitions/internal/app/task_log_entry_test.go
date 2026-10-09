package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the manual timeline entry (vigov-require a37ec96), over the REAL store on the fake driver.
//
//	PROVED HERE   who may write: the assignee and a `task.update` holder (full), the monitor, the
//	              assigning leader and the author (log), nobody else · the row and the audit entry share
//	              ONE transaction · the audit delta carries the LENGTH and the row id, never the text ·
//	              the actor is the staff business code · a refusal writes nothing · every status,
//	              `hoan-thanh` included, takes an entry · a blank entry never opens a transaction.
//
//	NOT PROVED    a37ec96's "officer of the task's unit" case — not implemented (no unit on the
//	              principal, no identity RPC); such an officer is refused, and that is asserted.

// The fixture task (dongNhiemVuGia): assignee CB-00311, assigner CB-00007, author CB-00123. No monitor:
// since ADR 0065 NV5 the monitoring officer IS the assignee. `outsiderCode` is none of them.
const outsiderCode = "CB-09999"

func staffActor(code string) audit.Actor { return audit.Actor{ID: code, Kind: "staff", IP: "10.0.0.7"} }

const logText = "Đã gửi công văn sang huyện, chờ phản hồi."

func TestAddLogEntry_WhoMayWrite(t *testing.T) {
	for _, c := range []struct {
		name     string
		code     string
		update   TaskUpdateRight
		wantCode string // quyen_ghi in the delta
	}{
		{"người thực hiện", maNguoiThucHien, false, "day-du"},
		{"lãnh đạo giao việc", maLanhDao, false, "ghi-nhat-ky"},
		{"người tạo", "CB-00123", false, "ghi-nhat-ky"},
		{"cán bộ có task.update", outsiderCode, true, "day-du"},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoNVMau()
			uc, ctx := dungGhiNhiemVu(t, k)

			row, _, err := uc.AddLogEntry(ctx, maNVGoc, "  "+logText+"  ", nil, nil, staffActor(c.code), c.update)
			if err != nil {
				t.Fatalf("ghi nhật ký: %v", err)
			}
			if row.NoiDung != logText || row.NguoiMa != c.code || row.TrangThaiTaiThoiDiem != domain.DangThucHien {
				t.Errorf("dòng trả về = %+v", row)
			}

			for _, stmt := range []string{"INSERT INTO nhat_ky_nhiem_vu", "INSERT INTO audit_log"} {
				if n := len(k.cau(stmt)); n != 1 {
					t.Errorf("câu %q chạy %d lần, muốn 1", stmt, n)
				}
			}
			if k.coCau("UPDATE nhiem_vu") {
				t.Error("ghi nhật ký mà đổi dòng nhiệm vụ")
			}
			chiGhiTrongGiaoDich(t, k)

			log := k.cau("INSERT INTO nhat_ky_nhiem_vu")[0]
			if log.args[4] != c.code || log.args[8] != logText {
				t.Errorf("dòng nhật ký: người=%v nội dung=%v", log.args[4], log.args[8])
			}

			vet := vetKiemToan(t, k)
			if vet.args[1] != c.code || vet.args[4] != ActionTaskLogEntry || vet.args[5] != maNVGoc {
				t.Errorf("vết: chủ thể=%v hành vi=%v đối tượng=%v", vet.args[1], vet.args[4], vet.args[5])
			}
			delta := auditDeltaText(t, k)
			for _, want := range []string{`"do_dai_noi_dung":`, `"nhat_ky_id":"`, `"quyen_ghi":"` + c.wantCode + `"`,
				`"trang_thai_tai_thoi_diem":"dang-thuc-hien"`} {
				if !strings.Contains(delta, want) {
					t.Errorf("delta thiếu %s: %s", want, delta)
				}
			}
			// THE TEXT NEVER ENTERS audit_log (rule 6, forbidden #4).
			if strings.Contains(delta, "công văn") {
				t.Errorf("nội dung tự do lọt vào vết kiểm toán: %s", delta)
			}
		})
	}
}

// TestAddLogEntry_OutsiderRefusedWritesNothing: `task.read` opened the route, but this officer is
// neither the assignee nor related, and holds no `task.update`.
func TestAddLogEntry_OutsiderRefusedWritesNothing(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	_, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, nil, nil, staffActor(outsiderCode), false)
	if !errors.Is(err, domain.ErrNotTaskParticipant) {
		t.Fatalf("lỗi = %v, muốn ErrNotTaskParticipant", err)
	}
	khongGhiGi(t, k)
}

// TestAddLogEntry_EmptyHolderMatchesNobody: a task with no assignee and no monitor must not make a
// caller whose code happens to be empty-equal its holder. (coCanBoThucHien refuses an empty actor
// first; this pins the domain half on its own.)
func TestAddLogEntry_EmptyHolderMatchesNobody(t *testing.T) {
	n := domain.NhiemVu{}
	if r := domain.TaskWorkRightFor(n, "", false); r != domain.TaskWorkNone {
		t.Errorf("mã rỗng trên nhiệm vụ rỗng = %v, muốn TaskWorkNone", r)
	}
	if r := domain.TaskWorkRightFor(n, outsiderCode, false); r != domain.TaskWorkNone {
		t.Errorf("người ngoài trên nhiệm vụ chưa phân công = %v, muốn TaskWorkNone", r)
	}
}

// TestAddLogEntry_FinishedTaskStillTakesAnEntry: appending moves nothing (rule 7 is about editing).
func TestAddLogEntry_FinishedTaskStillTakesAnEntry(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.HoanThanh)
	k.nhiemVu[idNVGoc]["ngay_hoan_thanh"] = mocThaoTacNV
	uc, ctx := dungGhiNhiemVu(t, k)

	row, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, nil, nil, staffActor(maNguoiThucHien), false)
	if err != nil {
		t.Fatalf("ghi nhật ký trên việc đã hoàn thành: %v", err)
	}
	if row.TrangThaiTaiThoiDiem != domain.HoanThanh {
		t.Errorf("trạng thái tại thời điểm = %s, muốn hoan-thanh", row.TrangThaiTaiThoiDiem)
	}
}

// TestAddLogEntry_BlankNeverOpensATransaction: refused before any row lock.
func TestAddLogEntry_BlankNeverOpensATransaction(t *testing.T) {
	for _, blank := range []string{"", "   \t\n"} {
		k := khoNVMau()
		uc, ctx := dungGhiNhiemVu(t, k)
		_, _, err := uc.AddLogEntry(ctx, maNVGoc, blank, nil, nil, staffActor(maNguoiThucHien), true)
		if !errors.Is(err, domain.ErrThieuNoiDungNhatKy) {
			t.Fatalf("nội dung %q: lỗi = %v, muốn ErrThieuNoiDungNhatKy", blank, err)
		}
		if k.batDau != 0 {
			t.Errorf("mở %d giao dịch cho nội dung rỗng", k.batDau)
		}
	}
}

// TestAddLogEntry_UnknownTaskIsNotFound: the same 404 sentinel every other act answers.
func TestAddLogEntry_UnknownTaskIsNotFound(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	_, _, err := uc.AddLogEntry(ctx, "NV404", logText, nil, nil, staffActor(maNguoiThucHien), true)
	if !errors.Is(err, petstore.ErrNhiemVuKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrNhiemVuKhongTonTai", err)
	}
	khongGhiGi(t, k)
}

// TestAddLogEntry_RetiredMonitorColumnGrantsNothing — ADR 0065 NV5: `chuyen_vien_theo_doi_ma` is never
// read. A pre-0025 row that still names somebody there gives that person NO right on the task; migration
// 0025 moved them into `nguoi_thuc_hien_ma` wherever that was empty, and where it was not, the assignee
// is the one holder. THE MUTATION THAT MUST TURN THIS RED: reading the retired column again.
func TestAddLogEntry_RetiredMonitorColumnGrantsNothing(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["chuyen_vien_theo_doi_ma"] = "CB-00412"
	uc, ctx := dungGhiNhiemVu(t, k)

	_, _, err := uc.AddLogEntry(ctx, maNVGoc, logText, nil, nil, staffActor("CB-00412"), false)
	if !errors.Is(err, domain.ErrNotTaskParticipant) {
		t.Fatalf("lỗi = %v, muốn ErrNotTaskParticipant — cột chuyên viên theo dõi đã nghỉ, không cấp quyền", err)
	}
	khongGhiGi(t, k)
}
