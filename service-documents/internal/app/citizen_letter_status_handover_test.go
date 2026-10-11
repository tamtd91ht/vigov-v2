package app

// Row 44: a status change carrying "Bộ phận xử lý" / "Cán bộ phụ trách" (the prototype's status
// composer). WHAT IS PROVEN: the status move and the routing commit in ONE transaction with one log row
// and one audit entry each; the routing reason is the note or the fixed sentence; only a holder of
// `petition.create` may hand over (C13/C14) — the assignee may still move the status alone; the unit and
// the officer are checked live with identity BEFORE any transaction; an unchanged holder writes no
// routing row.

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/service-documents/internal/domain"
)

func TestMoveWithHandoverRoutesInTheSameTransaction(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, ""))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())

	l, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening, Note: "Giao địa chính",
		ToUnitID: "bp-dia-chinh", AssigneeCode: "CB-00777"}, clerk)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if l.Status != domain.LetterStatusScreening || l.HoldingUnitID != "bp-dia-chinh" || l.AssigneeCode != "CB-00777" {
		t.Fatalf("đơn trả về: trạng thái %s, bộ phận %s, cán bộ %s", l.Status, l.HoldingUnitID, l.AssigneeCode)
	}
	got := repo.rows["dt-1"]
	if got.Status != domain.LetterStatusScreening || got.HoldingUnitID != "bp-dia-chinh" || got.AssigneeCode != "CB-00777" {
		t.Fatalf("dòng lưu: %+v", got)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("giao dịch: mở %d, commit %d — đổi trạng thái và chuyển đơn phải chung MỘT giao dịch", k.batDau, k.daCommit)
	}
	if len(repo.logs) != 2 || repo.logs[0].Kind != domain.LetterLogStatusChange || repo.logs[1].Kind != domain.LetterLogRouting {
		t.Fatalf("muốn một dòng chuyen-trang-thai rồi một dòng luan-chuyen: %+v", repo.logs)
	}
	r := repo.logs[1]
	if r.FromUnitID != "bp-tu-phap" || r.ToUnitID != "bp-dia-chinh" || r.AssigneeCode != "CB-00777" || r.Content != "Giao địa chính" {
		t.Fatalf("dòng luan-chuyen: %+v", r)
	}
	checkLogRows(t, repo.logs)
	a := auditEntries(k)
	if len(a) != 2 || a[0].args[4] != ActionMoveCitizenLetter || a[1].args[4] != ActionRouteCitizenLetter {
		t.Fatalf("muốn hai vết: đổi trạng thái rồi chuyển đơn, được %d", len(a))
	}
	for i := range a {
		if a[i].args[1] != "CB-00123" {
			t.Fatalf("vết %d mang người %v, muốn mã cán bộ", i, a[i].args[1])
		}
	}
}

func TestMoveHandoverWithoutNoteUsesTheFixedSentence(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, ""))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening, ToUnitID: "bp-dia-chinh"}, clerk); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if len(repo.logs) != 2 || repo.logs[1].Content != "Phân công xử lý" || repo.logs[0].Content != "" {
		t.Fatalf("không ghi chú: dòng trạng thái rỗng, dòng chuyển mang câu cố định — được %+v", repo.logs)
	}
	checkLogRows(t, repo.logs)
}

func TestMoveHandoverNeedsPetitionCreate(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, "CB-00777"))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())

	_, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening, ToUnitID: "bp-dia-chinh"}, officer)
	if !errors.Is(err, domain.ErrLetterHandoverNotPermitted) {
		t.Fatalf("cán bộ được giao (không có petition.create) chuyển đơn qua ô đổi trạng thái mà lỗi = %v", err)
	}
	if k.batDau != 0 || repo.writes != 0 || len(auditEntries(k)) != 0 {
		t.Fatal("bị từ chối mà vẫn mở giao dịch hoặc ghi")
	}
	// The same officer may still move the status alone.
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening}, officer); err != nil {
		t.Fatalf("chỉ đổi trạng thái mà cán bộ được giao bị từ chối: %v", err)
	}
}

func TestMoveHandoverChecksUnitAndOfficerBeforeAnyTransaction(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, ""))
	dir := liveDirectory()
	uc, ctx := buildLetters(t, k, repo, dir)

	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening,
		ToUnitID: "bp-xa-khac"}, clerk); !errors.Is(err, domain.ErrLetterUnitNotLive) {
		t.Fatalf("bộ phận không thuộc xã mà lỗi = %v", err)
	}
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening,
		ToUnitID: "bp-dia-chinh", AssigneeCode: "CB-00999"}, clerk); !errors.Is(err, domain.ErrLetterAssigneeNotLive) {
		t.Fatalf("cán bộ không nhận việc ở xã mà lỗi = %v", err)
	}
	dir.err = errors.New("identity sập")
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening,
		ToUnitID: "bp-dia-chinh"}, clerk); !errors.Is(err, ErrLetterDirectoryUnavailable) {
		t.Fatalf("identity không trả lời mà lỗi = %v", err)
	}
	if k.batDau != 0 || repo.writes != 0 {
		t.Fatalf("kiểm chưa qua mà đã mở %d giao dịch / ghi %d lần", k.batDau, repo.writes)
	}
}

func TestMoveHandoverOfficerOnlyKeepsTheHoldingUnit(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, ""))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())
	l, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening, AssigneeCode: "CB-00777"}, clerk)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if l.HoldingUnitID != "bp-tu-phap" || l.AssigneeCode != "CB-00777" {
		t.Fatalf("không gửi bộ phận thì giữ bộ phận đang giữ: %+v", l)
	}
	if r := repo.logs[1]; r.FromUnitID != "bp-tu-phap" || r.ToUnitID != "bp-tu-phap" {
		t.Fatalf("dòng luan-chuyen: %+v", r)
	}
	checkLogRows(t, repo.logs)
}

func TestMoveHandoverOfficerOnlyOnAnUnheldLetterIsRefusedAndRolledBack(t *testing.T) {
	k := khoVBMau()
	unheld := letterIn("dt-1", domain.LetterStatusNew, "")
	unheld.HoldingUnitID = ""
	repo := newLetterRepo(unheld)
	uc, ctx := buildLetters(t, k, repo, liveDirectory())
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening,
		AssigneeCode: "CB-00777"}, clerk); !errors.Is(err, domain.ErrLetterUnitMissing) {
		t.Fatalf("đơn chưa bộ phận nào giữ, chỉ chọn cán bộ, mà lỗi = %v", err)
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — đổi trạng thái không được đứng một mình khi phần chuyển bị từ chối", k.daCommit, k.daRollback)
	}
}

// The prototype's chip: a group, from which the server takes C3's one arrow.
func TestMoveByGroupDerivesTheOnlyStepAndRefusesTheAmbiguousOne(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, ""))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())

	l, err := uc.Move(ctx, "dt-1", MoveLetterRequest{StatusGroup: domain.LetterGroupInProgress}, clerk)
	if err != nil || l.Status != domain.LetterStatusScreening {
		t.Fatalf("Mới vào sổ → Đang xử lý: %q, %v — muốn dang-xu-ly-don", l.Status, err)
	}
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{StatusGroup: domain.LetterGroupNotProcessed}, clerk); !errors.Is(err, domain.ErrLetterGroupAmbiguous) {
		t.Fatalf("Lưu, không thụ lý từ dang-xu-ly-don phải hỏi bước cụ thể, được %v", err)
	}
	if repo.rows["dt-1"].Status != domain.LetterStatusScreening {
		t.Fatal("bị từ chối mà trạng thái đã đổi")
	}
	// The explicit step stays accepted.
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusGuided}, clerk); err != nil {
		t.Fatalf("bước cụ thể bị từ chối: %v", err)
	}
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusFiled,
		StatusGroup: domain.LetterGroupNotProcessed}, clerk); !errors.Is(err, domain.ErrLetterGroupInvalid) {
		t.Fatalf("gửi cả hai mà lỗi = %v", err)
	}
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{}, clerk); !errors.Is(err, domain.ErrLetterGroupInvalid) {
		t.Fatalf("không gửi gì mà lỗi = %v", err)
	}
}

func TestMoveByGroupChecksWhoMayActFirst(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusScreening, "CB-00777"))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{StatusGroup: domain.LetterGroupNotProcessed}, reader); !errors.Is(err, domain.ErrLetterNotPermitted) {
		t.Fatalf("người không được giao mà lỗi = %v — muốn 403 trước khi nói nhóm có mấy bước", err)
	}
}

func TestMoveHandoverToTheSameHolderWritesNoRoutingRow(t *testing.T) {
	k := khoVBMau()
	repo := newLetterRepo(letterIn("dt-1", domain.LetterStatusNew, "CB-00777"))
	uc, ctx := buildLetters(t, k, repo, liveDirectory())
	if _, err := uc.Move(ctx, "dt-1", MoveLetterRequest{Status: domain.LetterStatusScreening,
		ToUnitID: "bp-tu-phap", AssigneeCode: "CB-00777"}, clerk); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if len(repo.logs) != 1 || len(auditEntries(k)) != 1 {
		t.Fatalf("không ai đổi tay mà vẫn ghi %d dòng nhật ký, %d vết", len(repo.logs), len(auditEntries(k)))
	}
}
