package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
)

// GhiNhiemVu.TaoTuNguon — the source check runs INSIDE the task's transaction, BEFORE anything is
// written, and a refusal there leaves no task, no timeline row and no audit entry.

func TestTaoTuNguon_KiemNguonChayTrongGiaoDichTruocKhiGhi(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	var trongGiaoDich bool
	var soCauTruoc int
	_, err := uc.TaoTuNguon(ctx, taoMau(), canBoThu(), func(_ context.Context, tx *store.ScopedTx) error {
		trongGiaoDich = tx != nil && k.dangMo
		soCauTruoc = len(k.lenh)
		return nil
	})
	if err != nil {
		t.Fatalf("giao việc có kiểm nguồn: %v", err)
	}
	if !trongGiaoDich {
		t.Error("phép kiểm nguồn chạy ngoài giao dịch — quyết định trên một dòng người khác đổi được")
	}
	for _, l := range k.lenh[:soCauTruoc] {
		if strings.HasPrefix(l.sql, "INSERT") || strings.HasPrefix(l.sql, "UPDATE") {
			t.Errorf("đã ghi TRƯỚC phép kiểm nguồn: %q", l.sql)
		}
	}
}

func TestTaoTuNguon_KiemNguonTuChoiThiKhongGhiGi(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	tuChoi := errors.New("nguồn không còn nhận nhiệm vụ")
	_, err := uc.TaoTuNguon(ctx, taoMau(), canBoThu(), func(context.Context, *store.ScopedTx) error {
		return tuChoi
	})
	if !errors.Is(err, tuChoi) {
		t.Fatalf("lỗi = %v, muốn lỗi của phép kiểm nguồn đi nguyên vẹn ra ngoài", err)
	}
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "INSERT") || strings.HasPrefix(l.sql, "UPDATE") {
			t.Errorf("đã ghi dù phép kiểm nguồn từ chối: %q", l.sql)
		}
	}
	if k.daCommit != 0 {
		t.Errorf("commit %d lần dù bị từ chối", k.daCommit)
	}
}
