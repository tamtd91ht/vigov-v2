package store

import (
	"net/url"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Integration test for the task progress-log READ (migration 0006, §5.9).
//
// ⚠ READ THIS BEFORE BELIEVING A GREEN RUN: it SKIPS unless VIGOV_TEST_DSN is set, and the package
// still prints `ok`. The harness (TestMain, moKetNoi, xaRieng) lives in danh_muc_nhiem_vu_pg_test.go.
// Worth writing while it cannot run: the column list is a string the fake driver accepts whatever it
// says, and only the real schema can refuse a misspelt column.

func dongNhatKyNVPg(id, nhiemVuID string, luc time.Time) domain.NhatKyNhiemVu {
	return domain.NhatKyNhiemVu{
		ID: id, NhiemVuID: nhiemVuID, ThoiDiem: luc, NguoiMa: "CB-00311",
		TrangThaiTaiThoiDiem: domain.DangThucHien, NoiDung: "Đã làm một phần.",
	}
}

// TestPgNhatKyNhiemVuDocMoiNhatTruocVaTheoXa — newest first, `id` as the tie-break, one task only,
// one commune only, and the page cursor carrying the reader to the rest.
func TestPgNhatKyNhiemVuDocMoiNhatTruocVaTheoXa(t *testing.T) {
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)
	kho := pkgstore.New(db)
	s := NewNhiemVuStore(kho)
	som := time.Date(2026, 9, 26, 1, 0, 0, 0, time.UTC)
	muon := time.Date(2026, 9, 26, 2, 0, 0, 0, time.UTC)

	ghi := func(x string, e domain.NhatKyNhiemVu) {
		t.Helper()
		ctx := ctxXa(tenant.ID(x))
		if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
			return s.GhiNhatKy(ctx, tx, e)
		}); err != nil {
			t.Fatalf("ghi %s: %v", e.ID, err)
		}
	}
	ghi(xa, dongNhatKyNVPg("nknv-d1", "nv-nk-1", som))
	// Two rows of ONE transaction share an instant; the id decides.
	ghi(xa, dongNhatKyNVPg("nknv-d2", "nv-nk-1", muon))
	ghi(xa, dongNhatKyNVPg("nknv-d3", "nv-nk-1", muon))
	// Another task of the same commune — must not appear.
	ghi(xa, dongNhatKyNVPg("nknv-d4", "nv-nk-2", muon))
	// The SAME task id in another commune — must not appear either (rule 1).
	ghi(xaKhac, dongNhatKyNVPg("nknv-d5", "nv-nk-1", muon))

	ctx := ctxXa(tenant.ID(xa))
	yc, err := page.Parse(url.Values{"limit": {"2"}}, SapXepNhatKyNhiemVu)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	trang1, err := s.NhatKyCuaNhiemVu(ctx, "nv-nk-1", yc)
	if err != nil {
		t.Fatalf("đọc trang 1: %v", err)
	}
	if !trang1.HasMore || trang1.NextCursor == "" {
		t.Fatalf("trang 1: has_more=%v cursor=%q, muốn còn trang sau", trang1.HasMore, trang1.NextCursor)
	}
	yc2, err := page.Parse(url.Values{"limit": {"2"}, "cursor": {trang1.NextCursor}}, SapXepNhatKyNhiemVu)
	if err != nil {
		t.Fatalf("page.Parse trang 2: %v", err)
	}
	trang2, err := s.NhatKyCuaNhiemVu(ctx, "nv-nk-1", yc2)
	if err != nil {
		t.Fatalf("đọc trang 2: %v", err)
	}

	var ids []string
	for _, e := range append(trang1.Items, trang2.Items...) {
		ids = append(ids, e.ID)
	}
	muonIDs := []string{"nknv-d3", "nknv-d2", "nknv-d1"}
	if len(ids) != len(muonIDs) {
		t.Fatalf("đọc được %v, muốn %v", ids, muonIDs)
	}
	for i := range muonIDs {
		if ids[i] != muonIDs[i] {
			t.Errorf("thứ tự = %v, muốn %v", ids, muonIDs)
			break
		}
	}
	if trang2.HasMore {
		t.Error("trang 2 báo còn trang sau dù đã hết")
	}
}
