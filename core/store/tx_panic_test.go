package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
)

// THE DEFECT CLASS THIS FILE CLOSES: a panic inside a transaction body.
//
// A nil map, an index out of range, a tenant.MustFrom deeper in the call — the transaction then
// reaches neither Commit nor Rollback, and stays open with its row locks until the request's
// context is cancelled. The business write is invisible to everyone else while the lock is held,
// and nothing in the logs says a transaction was left behind.
//
// The two halves are equally load-bearing: the rollback must happen, AND the panic must keep
// travelling up to httpx.Recover. A Tx that quietly turned a panic into an error would hide a
// defect the operator has to see.

func TestTxPanicThiRollbackVaPanicVanLanRa(t *testing.T) {
	db, g := moGhiChep()
	defer db.Close()

	ctx := tenant.Into(context.Background(), xaA)

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("panic bị nuốt — httpx.Recover không bao giờ thấy, lỗi lập trình biến mất")
			}
			if s, ok := r.(string); !ok || !strings.Contains(s, "hỏng giữa giao dịch") {
				t.Errorf("panic bị đổi nội dung: %v — phải lan lên nguyên vẹn", r)
			}
		}()
		_ = New(db).For(ctx).Tx(ctx, func(tx *ScopedTx) error {
			if _, err := tx.Exec(ctx, "INSERT INTO phien (tenant_id) VALUES ($1)", string(xaA)); err != nil {
				t.Fatal(err)
			}
			panic("hỏng giữa giao dịch")
		})
	}()

	phien := g.chua("INSERT INTO phien")
	if phien == nil {
		t.Fatal("câu lệnh nghiệp vụ không chạy")
	}
	if got := g.ketThucCua(phien.tx); got != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback — panic để lại giao dịch mở, giữ khoá dòng", got)
	}
}

func TestTxPanicRoiVanDungDuocKetNoiTiepTheo(t *testing.T) {
	// The consequence of NOT rolling back, made visible: the pool has one connection here, so a
	// transaction left open never gives it back and the next request blocks. This is what a
	// leaked transaction looks like from outside — not an error, a hang.
	db, g := moGhiChep()
	defer db.Close()

	ctx := tenant.Into(context.Background(), xaA)
	func() {
		defer func() { _ = recover() }()
		_ = New(db).For(ctx).Tx(ctx, func(*ScopedTx) error { panic("hỏng giữa giao dịch") })
	}()

	// If the first transaction were still open it would still hold the pool's single connection,
	// and this would wait for it. The deadline is what turns that wait into a readable failure
	// instead of a test that hangs — which is also exactly how it presents in production.
	ctx2, huy := context.WithTimeout(context.Background(), 2*time.Second)
	defer huy()
	ctx2 = tenant.Into(ctx2, xaB)
	err := New(db).For(ctx2).Tx(ctx2, func(tx *ScopedTx) error {
		_, err := tx.Exec(ctx2, "INSERT INTO audit_log (tenant_id) VALUES ($1)", string(xaB))
		return err
	})
	if err != nil {
		t.Fatalf("giao dịch kế tiếp không chạy được sau một panic: %v", err)
	}
	if vet := g.chua("INSERT INTO audit_log"); vet == nil || g.ketThucCua(vet.tx) != "commit" {
		t.Fatal("giao dịch kế tiếp không commit được")
	}
}
