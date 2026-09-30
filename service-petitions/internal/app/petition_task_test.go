package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// PetitionTaskCreation — a task FROM a petition (30/09/2026).
//
// THE PETITION HALF RUNS ON THE REAL STORE OVER THE PETITION FAKE DRIVER, so the locked read, the
// timeline INSERT and the audit INSERT are real statements the driver records with "inside a
// transaction or not". The TASK half is taskOnSameTx below: it opens the transaction on the SAME
// *store.DB the way GhiNhiemVu.CreateFromSource does and runs the two steps around a stand-in task
// INSERT. GhiNhiemVu's own side of the contract (Check first, Record last, both in its transaction) is
// pinned separately, on the task driver, at the bottom of this file.

const taskCodeFromPetition = "NV07"

// taskOnSameTx is the task register as this act sees it: ONE transaction, Check → task INSERT → Record.
type taskOnSameTx struct {
	db *pkgstore.DB

	calls int
	got   YeuCauTaoNhiemVu

	// beforeTx runs after the use case's unlocked read and before the transaction — the window in which
	// another officer can close the petition.
	beforeTx func()
}

func (f *taskOnSameTx) CreateFromSource(ctx context.Context, yc YeuCauTaoNhiemVu, _ audit.Actor,
	steps SourceSteps) (domain.NhiemVu, error) {

	f.calls++
	f.got = yc
	if f.beforeTx != nil {
		f.beforeTx()
	}
	n := domain.NhiemVu{ID: "nv-01", Ma: taskCodeFromPetition, TaoLuc: mocThaoTac}
	err := f.db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		if err := steps.Check(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO nhiem_vu (tenant_id) VALUES ($1)", string(tx.TenantID())); err != nil {
			return err
		}
		return steps.Record(ctx, tx, n)
	})
	if err != nil {
		return domain.NhiemVu{}, err
	}
	return n, nil
}

func buildPetitionTask(t *testing.T, k *khoPhieuXuLyGia) (*PetitionTaskCreation, *taskOnSameTx, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	handle := pkgstore.New(db)
	tasks := &taskOnSameTx{db: handle}
	uc := NewPetitionTaskCreation(petstore.NewPhieuPhanAnhStore(handle), tasks)
	uc.newID = func() (string, error) { return "01JNHATKYTAONHIEMVU000000", nil }
	return uc, tasks, ctxXa(xaThu)
}

func petitionTaskRequest() YeuCauTaoNhiemVu {
	return YeuCauTaoNhiemVu{TuSinhMa: true, Loai: "co-ban", TieuDe: "Dọn rác đầu ngõ thôn Hà Lam"}
}

func openPetition() *khoPhieuXuLyGia {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{"trang_thai": string(domain.DangXuLy), "linh_vuc": "rac-thai"})
	return k
}

func noWrites(t *testing.T, k *khoPhieuXuLyGia) {
	t.Helper()
	for _, l := range k.lenh {
		if strings.HasPrefix(strings.TrimSpace(l.sql), "INSERT") || strings.HasPrefix(strings.TrimSpace(l.sql), "UPDATE") {
			t.Errorf("đã ghi dù bị từ chối: %q", l.sql)
		}
	}
	if k.daCommit != 0 {
		t.Errorf("commit %d lần dù bị từ chối", k.daCommit)
	}
}

func TestPetitionTask_TaskLogAndBothAuditEntriesInOneTransaction(t *testing.T) {
	k := openPetition()
	uc, tasks, ctx := buildPetitionTask(t, k)

	n, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), false)
	if err != nil {
		t.Fatalf("tạo nhiệm vụ từ phiếu: %v", err)
	}
	if n.Ma != taskCodeFromPetition {
		t.Errorf("mã nhiệm vụ trả về = %q", n.Ma)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("giao dịch mở %d, commit %d — muốn đúng MỘT giao dịch cho cả nhiệm vụ lẫn nhật ký phiếu",
			k.batDau, k.daCommit)
	}

	locked := k.cau("FOR UPDATE")
	if len(locked) != 1 || !locked[0].trongGiaoDich {
		t.Fatal("phiếu không được đọc lại FOR UPDATE bên trong giao dịch của nhiệm vụ")
	}
	for _, want := range []string{"INSERT INTO nhiem_vu", "INSERT INTO nhat_ky_phan_anh", "INSERT INTO audit_log"} {
		rows := k.cau(want)
		if len(rows) != 1 {
			t.Fatalf("%d câu %q, muốn 1", len(rows), want)
		}
		if !rows[0].trongGiaoDich {
			t.Errorf("%q chạy NGOÀI giao dịch", want)
		}
	}

	logRow := k.cau("INSERT INTO nhat_ky_phan_anh")[0]
	if !coThamSo(logRow.args, string(domain.LogActionTaskCreated)) {
		t.Errorf("dòng nhật ký không mang hành vi %q: %v", domain.LogActionTaskCreated, logRow.args)
	}
	if !coThamSo(logRow.args, string(domain.DangXuLy)) {
		t.Error("dòng nhật ký không ghi trạng thái phiếu đang đứng — hành vi này không đổi trạng thái")
	}
	if !coThamSo(logRow.args, domain.TaskCreatedLogText(taskCodeFromPetition)) {
		t.Error("dòng nhật ký không nêu mã nhiệm vụ vừa cấp")
	}
	if !coThamSo(logRow.args, maCanBoThu) || !coThamSo(logRow.args, idPhieuThu) {
		t.Error("dòng nhật ký thiếu mã cán bộ (không phải id nội bộ) hoặc id phiếu")
	}

	entry := k.cau("INSERT INTO audit_log")[0]
	if !coThamSo(entry.args, AuditActionTaskFromPetition) || !coThamSo(entry.args, maPhieuThu) {
		t.Errorf("vết phía phiếu phải mang động từ %q và CHỦ ĐỀ là mã tra cứu: %v",
			AuditActionTaskFromPetition, entry.args)
	}
	for _, a := range entry.args {
		if s, ok := a.(string); ok && strings.Contains(s, "Dọn rác") {
			t.Error("tiêu đề nhiệm vụ lọt vào audit_log")
		}
	}
	if tasks.calls != 1 {
		t.Errorf("gọi đường tạo nhiệm vụ %d lần, muốn 1", tasks.calls)
	}
}

// The SERVER sets the source pair; whatever the request struct carried is overwritten.
func TestPetitionTask_SourceIsThePetitionNeverTheRequest(t *testing.T) {
	k := openPetition()
	uc, tasks, ctx := buildPetitionTask(t, k)

	yc := petitionTaskRequest()
	yc.NguonGiao = string(domain.NguonTrucTiep)
	yc.NguonID = "01JPHIEUCUAXAKHAC00000000"
	if _, err := uc.CreateTask(ctx, maPhieuThu, yc, canBoThu(), false); err != nil {
		t.Fatalf("tạo: %v", err)
	}
	if tasks.got.NguonGiao != string(domain.NguonPhanAnh) || tasks.got.NguonID != idPhieuThu {
		t.Errorf("nguồn = (%q, %q), muốn (phan-anh, %q)", tasks.got.NguonGiao, tasks.got.NguonID, idPhieuThu)
	}
}

// Both reads bind the commune FROM THE CONTEXT — which is what makes another commune's code the same
// "no such petition" as an unknown one.
func TestPetitionTask_ReadsAreScopedToTheContextCommune(t *testing.T) {
	k := openPetition()
	uc, _, ctx := buildPetitionTask(t, k)
	if _, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), false); err != nil {
		t.Fatalf("tạo: %v", err)
	}
	reads := k.cau("FROM phieu_phan_anh")
	if len(reads) != 2 {
		t.Fatalf("%d lần đọc phiếu, muốn 2 (đọc trước, đọc khoá)", len(reads))
	}
	for _, r := range reads {
		if !strings.Contains(r.sql, "tenant_id = $1") || !coThamSo(r.args, string(xaThu)) {
			t.Errorf("đọc phiếu không ràng theo xã trong ngữ cảnh: %q %v", r.sql, r.args)
		}
		if !strings.Contains(r.sql, "deleted_at IS NULL") {
			t.Errorf("đọc phiếu không loại dòng đã xoá mềm: %q", r.sql)
		}
	}
}

func TestPetitionTask_UnknownCodeWritesNothing(t *testing.T) {
	k := openPetition()
	k.hang = nil // unknown, another commune's, or soft-deleted: the scoped read finds no row
	uc, tasks, ctx := buildPetitionTask(t, k)

	_, err := uc.CreateTask(ctx, "PA-ZZZZ-ZZZZ-ZZZZ", petitionTaskRequest(), canBoThu(), false)
	if !errors.Is(err, petstore.ErrPhieuKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrPhieuKhongTonTai", err)
	}
	if tasks.calls != 0 || k.batDau != 0 {
		t.Error("đã đi tới đường tạo nhiệm vụ cho một phiếu không có")
	}
	noWrites(t, k)
}

func TestPetitionTask_RestrictedFieldWithoutKeyIsNotFound(t *testing.T) {
	for _, status := range []domain.TrangThai{domain.DangPhanLoai, domain.DangXuLy, domain.DaDong} {
		t.Run(string(status), func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(map[string]any{"linh_vuc": domain.LinhVucHanChe, "trang_thai": string(status)})
			uc, tasks, ctx := buildPetitionTask(t, k)

			// A CLOSED `can-bo` petition too: the 404 comes before the 409, or the 409 confirms it exists.
			_, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), false)
			if !errors.Is(err, ErrPhieuHanChe) {
				t.Fatalf("lỗi = %v, muốn ErrPhieuHanChe (404)", err)
			}
			if tasks.calls != 0 {
				t.Error("đã tạo nhiệm vụ từ phiếu hạn chế cho người không có feedback.restricted")
			}
			noWrites(t, k)
		})
	}
}

// User decision 30/09/2026: NO task from a `can-bo` petition at all — the key holder is told so by name
// (409 restricted_field_no_task), whatever the status, and nothing is written.
func TestPetitionTask_RestrictedFieldWithKeyIsRefusedByName(t *testing.T) {
	for _, status := range []domain.TrangThai{domain.DangPhanLoai, domain.DaChuyenXuLy, domain.DangXuLy,
		domain.DaXuLy, domain.ChoDanXacNhan, domain.DaDong} {
		t.Run(string(status), func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(map[string]any{"linh_vuc": domain.LinhVucHanChe, "trang_thai": string(status)})
			uc, tasks, ctx := buildPetitionTask(t, k)
			_, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), true)
			if !errors.Is(err, domain.ErrRestrictedFieldNoTask) {
				t.Fatalf("lỗi = %v, muốn ErrRestrictedFieldNoTask", err)
			}
			if tasks.calls != 0 {
				t.Error("đã đi tới đường tạo nhiệm vụ cho phiếu lĩnh vực can-bo")
			}
			noWrites(t, k)
		})
	}
}

// Received or being classified: the commune has not yet classified AND accepted it.
func TestPetitionTask_NotClassifiedIsRefused(t *testing.T) {
	for ten, row := range map[string]map[string]any{
		"da-tiep-nhan, chưa có lĩnh vực":        {"trang_thai": string(domain.DaTiepNhan)},
		"da-tiep-nhan, dân đã chọn lĩnh vực":    {"trang_thai": string(domain.DaTiepNhan), "linh_vuc": "rac-thai"},
		"dang-phan-loai (chưa quyết tiếp nhận)": {"trang_thai": string(domain.DangPhanLoai), "linh_vuc": "rac-thai"},
		"dang-xu-ly nhưng lĩnh vực trống (lạ)":  {"trang_thai": string(domain.DangXuLy)},
		"da-chuyen-xu-ly nhưng lĩnh vực trống":  {"trang_thai": string(domain.DaChuyenXuLy)},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(row)
			uc, tasks, ctx := buildPetitionTask(t, k)
			_, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), false)
			if !errors.Is(err, domain.ErrPetitionNotClassifiedForTask) {
				t.Fatalf("lỗi = %v, muốn ErrPetitionNotClassifiedForTask", err)
			}
			if tasks.calls != 0 {
				t.Error("đã đi tới đường tạo nhiệm vụ cho phiếu chưa phân loại")
			}
			noWrites(t, k)
		})
	}
}

func TestPetitionTask_ClosedPetitionIsRefused(t *testing.T) {
	for _, status := range []domain.TrangThai{domain.DaDong, domain.KhongTiepNhan, domain.ChuyenCapTren} {
		t.Run(string(status), func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(map[string]any{"trang_thai": string(status), "linh_vuc": "rac-thai"})
			uc, tasks, ctx := buildPetitionTask(t, k)

			_, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), false)
			if !errors.Is(err, domain.ErrPetitionClosedForTask) {
				t.Fatalf("lỗi = %v, muốn ErrPetitionClosedForTask", err)
			}
			if tasks.calls != 0 {
				t.Error("đã đi tới đường tạo nhiệm vụ cho một phiếu đã kết thúc")
			}
			noWrites(t, k)
		})
	}
}

// The four statuses after classification AND acceptance (user decision 30/09/2026) accept a task, with
// or without `feedback.restricted`.
func TestPetitionTask_AcceptedAndClassifiedStatusesAccept(t *testing.T) {
	for _, status := range []domain.TrangThai{domain.DaChuyenXuLy, domain.DangXuLy, domain.DaXuLy,
		domain.ChoDanXacNhan} {
		for _, restricted := range []QuyenXemHanChe{false, true} {
			t.Run(fmt.Sprintf("%s/restricted=%v", status, restricted), func(t *testing.T) {
				k := khoPhieuMau()
				k.hang = dongPhieuMau(map[string]any{"trang_thai": string(status), "linh_vuc": "rac-thai"})
				uc, tasks, ctx := buildPetitionTask(t, k)
				if _, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), restricted); err != nil {
					t.Fatalf("trạng thái %s bị từ chối: %v", status, err)
				}
				if tasks.calls != 1 || k.daCommit != 1 {
					t.Errorf("gọi %d lần, commit %d — muốn 1/1", tasks.calls, k.daCommit)
				}
			})
		}
	}
}

// Closed by another officer BETWEEN the unlocked read and the transaction: the locked read decides.
func TestPetitionTask_ClosedInTheWindowLosesTheRace(t *testing.T) {
	k := openPetition()
	uc, tasks, ctx := buildPetitionTask(t, k)
	tasks.beforeTx = func() {
		k.hang = dongPhieuMau(map[string]any{"trang_thai": string(domain.DaDong), "linh_vuc": "rac-thai"})
	}

	_, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), false)
	if !errors.Is(err, domain.ErrPetitionClosedForTask) {
		t.Fatalf("lỗi = %v, muốn ErrPetitionClosedForTask từ lần đọc khoá", err)
	}
	if k.daRollback != 1 {
		t.Errorf("rollback %d lần, muốn 1", k.daRollback)
	}
	noWrites(t, k)
}

// The other changes that can land in the window, each decided by the LOCKED read and each rolling back
// with nothing written: the row is no longer classified-and-accepted (a status or field the lifecycle
// cannot produce today, but the check must not trust that), or it was classified INTO `can-bo`.
func TestPetitionTask_RulesChangedInTheWindowLoseTheRace(t *testing.T) {
	for ten, ca := range map[string]struct {
		row        map[string]any
		restricted QuyenXemHanChe
		want       error
	}{
		"về dang-phan-loai": {map[string]any{"trang_thai": string(domain.DangPhanLoai), "linh_vuc": "rac-thai"},
			false, domain.ErrPetitionNotClassifiedForTask},
		"lĩnh vực bị xoá": {map[string]any{"trang_thai": string(domain.DangXuLy)},
			false, domain.ErrPetitionNotClassifiedForTask},
		"thành can-bo, có khoá": {map[string]any{"trang_thai": string(domain.DangXuLy), "linh_vuc": domain.LinhVucHanChe},
			true, domain.ErrRestrictedFieldNoTask},
		"thành can-bo, không khoá": {map[string]any{"trang_thai": string(domain.DangXuLy), "linh_vuc": domain.LinhVucHanChe},
			false, ErrPhieuHanChe},
	} {
		t.Run(ten, func(t *testing.T) {
			k := openPetition()
			uc, tasks, ctx := buildPetitionTask(t, k)
			tasks.beforeTx = func() { k.hang = dongPhieuMau(ca.row) }

			_, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), ca.restricted)
			if !errors.Is(err, ca.want) {
				t.Fatalf("lỗi = %v, muốn %v từ lần đọc khoá", err, ca.want)
			}
			if k.daRollback != 1 {
				t.Errorf("rollback %d lần, muốn 1", k.daRollback)
			}
			noWrites(t, k)
		})
	}
}

// The petition's audit entry failing takes the task and the timeline row down with it.
func TestPetitionTask_AuditFailureRollsEverythingBack(t *testing.T) {
	k := openPetition()
	k.loiSau = "INSERT INTO audit_log"
	uc, _, ctx := buildPetitionTask(t, k)

	if _, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), canBoThu(), false); err == nil {
		t.Fatal("vết hỏng mà vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit %d, rollback %d — muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestPetitionTask_NoActorCodeRefusedBeforeAnyRead(t *testing.T) {
	k := openPetition()
	uc, tasks, ctx := buildPetitionTask(t, k)
	_, err := uc.CreateTask(ctx, maPhieuThu, petitionTaskRequest(), audit.Actor{Kind: "staff"}, false)
	if err == nil {
		t.Fatal("không có mã cán bộ mà vẫn tạo")
	}
	if len(k.lenh) != 0 || tasks.calls != 0 {
		t.Error("đã chạm kho dù không có chủ thể")
	}
}

// --- GhiNhiemVu's side of the contract, on the task driver ---------------------------------------------

// A `phan-anh` source with a Check but no Record is still refused: the petition's timeline would never
// show the task.
func TestCreateFromSource_PetitionSourceNeedsBothSteps(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	yc := taoMau()
	yc.NguonGiao = string(domain.NguonPhanAnh)
	yc.NguonID = idPhieuThu

	_, err := uc.CreateFromSource(ctx, yc, canBoThu(), SourceSteps{
		Check: func(context.Context, *pkgstore.ScopedTx) error { return nil },
	})
	if !errors.Is(err, domain.ErrPetitionSourceNotDirect) {
		t.Fatalf("lỗi = %v, muốn ErrPetitionSourceNotDirect", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch cho một nguồn bị từ chối", k.batDau)
	}
}

// Record runs INSIDE the task's transaction, AFTER the task row, and sees the minted number.
func TestCreateFromSource_RecordRunsLastInTheSameTransaction(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	yc := taoMau()
	yc.NguonGiao = string(domain.NguonPhanAnh)
	yc.NguonID = idPhieuThu

	var inTx, afterInsert bool
	var sawCode string
	_, err := uc.CreateFromSource(ctx, yc, canBoThu(), SourceSteps{
		Check: func(context.Context, *pkgstore.ScopedTx) error { return nil },
		Record: func(_ context.Context, tx *pkgstore.ScopedTx, n domain.NhiemVu) error {
			inTx = tx != nil && k.dangMo
			afterInsert = k.coCau("INSERT INTO nhiem_vu") && k.coCau("INSERT INTO audit_log")
			sawCode = n.Ma
			return nil
		},
	})
	if err != nil {
		t.Fatalf("tạo: %v", err)
	}
	if !inTx || !afterInsert {
		t.Errorf("Record trong giao dịch = %v, sau khi ghi nhiệm vụ và vết = %v — muốn cả hai", inTx, afterInsert)
	}
	if sawCode == "" {
		t.Error("Record không thấy mã nhiệm vụ vừa cấp")
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Errorf("giao dịch mở %d, commit %d — muốn 1/1", k.batDau, k.daCommit)
	}
}

func TestCreateFromSource_RecordFailureRollsTheTaskBack(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	yc := taoMau()
	yc.NguonGiao = string(domain.NguonPhanAnh)
	yc.NguonID = idPhieuThu

	refused := errors.New("nhật ký phiếu hỏng")
	_, err := uc.CreateFromSource(ctx, yc, canBoThu(), SourceSteps{
		Check:  func(context.Context, *pkgstore.ScopedTx) error { return nil },
		Record: func(context.Context, *pkgstore.ScopedTx, domain.NhiemVu) error { return refused },
	})
	if !errors.Is(err, refused) {
		t.Fatalf("lỗi = %v, muốn lỗi của Record đi nguyên vẹn ra ngoài", err)
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit %d, rollback %d — muốn 0/1: nhiệm vụ không được sống sót khi phiếu không ghi được",
			k.daCommit, k.daRollback)
	}
}
