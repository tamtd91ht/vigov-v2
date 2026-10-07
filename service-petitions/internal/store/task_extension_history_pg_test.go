package store

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Integration tests for the per-task extension HISTORY read and for the decision writing its note
// (migration 0031) through the store's own statement.
//
// ⚠ THESE SKIP UNLESS VIGOV_TEST_DSN IS SET, and the package still prints `ok` — see the header of
// nhiem_vu_pg_test.go. Only these prove the column list and the UPDATE against the real schema and
// 0031's trigger.

// decideThroughStore runs DeNghiLuiHanStore.QuyetDinh in a scoped transaction — the statement the use
// case runs, not a hand-written copy of it.
func decideThroughStore(t *testing.T, s *DeNghiLuiHanStore, ctx context.Context, id string,
	to domain.TrangThaiDeNghi, note string) error {

	t.Helper()
	return s.db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		return s.QuyetDinh(ctx, tx, id, to, "CB-00007", time.Now().UTC(), note)
	})
}

// TestPgDecisionWritesNoteBlankIsNull — the decision's own UPDATE stores the note (0031's trigger lets it
// through only there), and "" is stored as NULL, never as an empty string (the CHECK refuses that).
func TestPgDecisionWritesNoteBlankIsNull(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	danhMucChoXa(t, db, commune)
	for _, task := range []struct{ id, code, req string }{
		{"nv-hn1", "NV71", "dn-hn1"}, {"nv-hn2", "NV72", "dn-hn2"},
	} {
		if err := themNhiemVu(db, commune, task.id, task.code, "theo-van-ban", "dang-thuc-hien",
			mocHanGocPg, mocHanGocPg, nil); err != nil {
			t.Fatalf("thêm nhiệm vụ: %v", err)
		}
		if err := themDeNghi(db, commune, task.req, task.id, "cho-duyet", nil, nil); err != nil {
			t.Fatalf("gửi đề nghị: %v", err)
		}
	}

	s := NewDeNghiLuiHanStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))
	if err := decideThroughStore(t, s, ctx, "dn-hn1", domain.TuChoiLuiHan, "Chưa đủ căn cứ."); err != nil {
		t.Fatalf("quyết định kèm ghi chú: %v", err)
	}
	if err := decideThroughStore(t, s, ctx, "dn-hn2", domain.DaDuyetLuiHan, ""); err != nil {
		t.Fatalf("quyết định không ghi chú: %v", err)
	}

	for id, want := range map[string]any{"dn-hn1": "Chưa đủ căn cứ.", "dn-hn2": nil} {
		var got any
		if err := db.QueryRow(`SELECT decision_note FROM de_nghi_lui_han WHERE tenant_id = $1 AND id = $2`,
			commune, id).Scan(&got); err != nil {
			t.Fatalf("đọc ghi chú %s: %v", id, err)
		}
		if b, ok := got.([]byte); ok {
			got = string(b)
		}
		if got != want {
			t.Errorf("decision_note của %s = %#v, muốn %#v", id, got, want)
		}
	}
}

// TestPgTaskHistoryEveryStatusLiveThisCommuneNewestFirst — every status of ONE task, newest first across
// a cursor, its note; soft-deleted requests, another task's, and another commune's twin all excluded.
func TestPgTaskHistoryEveryStatusLiveThisCommuneNewestFirst(t *testing.T) {
	db := moKetNoi(t)
	commune, other := xaRieng(t)
	danhMucChoXa(t, db, commune)
	danhMucChoXa(t, db, other)
	for _, c := range []struct{ commune, id, code string }{
		{commune, "nv-h1", "NV61"}, {commune, "nv-h2", "NV62"}, {other, "nv-h1", "NV61"},
	} {
		if err := themNhiemVu(db, c.commune, c.id, c.code, "theo-van-ban", "dang-thuc-hien",
			mocHanGocPg, mocHanGocPg, nil); err != nil {
			t.Fatalf("thêm nhiệm vụ %s: %v", c.id, err)
		}
	}

	// dn-h1 rejected with a note, dn-h2 soft-deleted (decided first so the pending slot frees),
	// dn-h3 pending — in that filing order; dn-hx on another task; dn-hk the other commune's twin.
	s := NewDeNghiLuiHanStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))
	if err := themDeNghi(db, commune, "dn-h1", "nv-h1", "cho-duyet", nil, nil); err != nil {
		t.Fatalf("gửi dn-h1: %v", err)
	}
	if err := decideThroughStore(t, s, ctx, "dn-h1", domain.TuChoiLuiHan, "Chưa đủ căn cứ."); err != nil {
		t.Fatalf("quyết định dn-h1: %v", err)
	}
	if err := themDeNghi(db, commune, "dn-h2", "nv-h1", "cho-duyet", nil, nil); err != nil {
		t.Fatalf("gửi dn-h2: %v", err)
	}
	if err := decideThroughStore(t, s, ctx, "dn-h2", domain.TuChoiLuiHan, ""); err != nil {
		t.Fatalf("quyết định dn-h2: %v", err)
	}
	if _, err := db.Exec(`UPDATE de_nghi_lui_han SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		WHERE tenant_id = $1 AND id = $2`, commune, "dn-h2", "CB-00007", "gửi nhầm"); err != nil {
		t.Fatalf("xoá mềm dn-h2: %v", err)
	}
	for _, r := range []struct{ commune, id, task string }{
		{commune, "dn-h3", "nv-h1"}, {commune, "dn-hx", "nv-h2"}, {other, "dn-hk", "nv-h1"},
	} {
		if err := themDeNghi(db, r.commune, r.id, r.task, "cho-duyet", nil, nil); err != nil {
			t.Fatalf("gửi %s: %v", r.id, err)
		}
	}

	req, err := page.Parse(url.Values{"limit": {"1"}}, TaskExtensionHistorySort)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	first, err := s.TaskHistory(ctx, "nv-h1", req)
	if err != nil {
		t.Fatalf("trang 1: %v", err)
	}
	req2, err := page.Parse(url.Values{"limit": {"1"}, "cursor": {first.NextCursor}}, TaskExtensionHistorySort)
	if err != nil {
		t.Fatalf("page.Parse trang 2: %v", err)
	}
	second, err := s.TaskHistory(ctx, "nv-h1", req2)
	if err != nil {
		t.Fatalf("trang 2: %v", err)
	}
	all := append(first.Items, second.Items...)
	if len(all) != 2 || all[0].ID != "dn-h3" || all[1].ID != "dn-h1" || second.HasMore {
		t.Fatalf("lịch sử = %+v (còn trang: %v), muốn [dn-h3 dn-h1] mới nhất trước", all, second.HasMore)
	}
	if all[0].TrangThai != domain.ChoDuyetLuiHan || all[0].DecisionNote != "" {
		t.Errorf("dn-h3 = %+v", all[0])
	}
	if all[1].TrangThai != domain.TuChoiLuiHan || all[1].DecisionNote != "Chưa đủ căn cứ." ||
		all[1].NguoiDuyetMa != "CB-00007" || all[1].DuyetLuc.IsZero() {
		t.Errorf("dn-h1 = %+v", all[1])
	}
}
