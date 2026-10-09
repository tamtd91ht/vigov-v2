package app

import (
	"database/sql/driver"
	"strconv"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The `occurrence` of `petitions.status_changed.v1` (statusOccurrence) — over the REAL use cases, the REAL
// store and driver_gia_phieu_test.go's fake driver, whose petition_merge_event count follows the history
// rows it has seen inserted.
//
//	PROVED   a petition never unmerged sends EXACTLY the old number, so_lan_mo_lai + 1 — on a staff act
//	         and on a reopening by rating · a merged petition that followed its main into
//	         `cho-dan-xac-nhan`, was unmerged and came back to it sends occurrence 2 the second time, a key
//	         comms has not seen · the count runs inside the act's transaction, scoped by commune.
//	NOT PROVED   PostgreSQL's evaluation of the count — needs VIGOV_TEST_DSN.

// statusEvents lists "status:occurrence" of every status_changed row written for `code`, in order.
func statusEvents(t *testing.T, k *khoPhieuXuLyGia, code string) []string {
	t.Helper()
	var got []string
	for _, r := range k.cau("INSERT INTO su_kien_di") {
		if r.args[2] != "petitions.status_changed.v1" || r.args[3] != code {
			continue
		}
		b := thanSuKien(t, r.args)
		got = append(got, b["status"].(string)+":"+strconv.Itoa(int(b["occurrence"].(float64))))
	}
	return got
}

// assertUnmergeCountScoped: every count is in the transaction, filtered by the commune bound from the
// context ($1) and by the petition and the unmerge kind — never another commune's history.
func assertUnmergeCountScoped(t *testing.T, k *khoPhieuXuLyGia, petitionID string) {
	t.Helper()
	counts := k.cau("count(*) FROM petition_merge_event")
	if len(counts) == 0 {
		t.Fatal("the occurrence was computed without reading the unmerge history")
	}
	asked := false
	for _, c := range counts {
		if !c.trongGiaoDich || !strings.Contains(c.sql, "tenant_id = $1") || len(c.args) != 3 ||
			c.args[0] != string(xaThu) || c.args[2] != string(domain.MergeKindUnmerge) && c.args[2] != string(domain.MergeKindMerge) {
			t.Errorf("count = %q %v (in tx %v)", c.sql, c.args, c.trongGiaoDich)
		}
		asked = asked || c.args[1] == petitionID && c.args[2] == string(domain.MergeKindUnmerge)
	}
	if !asked {
		t.Errorf("no unmerge count for petition %s", petitionID)
	}
}

// Backward compatibility: with no unmerge the number is the one every notice already sent was keyed on.
func TestStatusOccurrenceWithoutUnmergeIsReopenCountPlusOne(t *testing.T) {
	for _, reopened := range []int64{0, 2} {
		t.Run(strconv.Itoa(int(reopened)), func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(map[string]any{"trang_thai": string(domain.DaXuLy), "linh_vuc": "rac-thai",
				"han_xu_ly_xong": mocXuLyXongThu, "phan_loai_luc": mocThaoTac, "xu_ly_xong_luc": mocThaoTac,
				"so_lan_mo_lai": reopened})
			k.latestReopen = mocThaoTac.Add(-1) // a reopened petition's timeline holds its reopen row
			uc, ctx := dungXuLy(t, k, hanXuLyThu())
			if _, err := uc.TienTrangThai(ctx, maPhieuThu, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe); err != nil {
				t.Fatalf("TienTrangThai: %v", err)
			}
			want := "cho-dan-xac-nhan:" + strconv.Itoa(int(reopened)+1)
			if got := strings.Join(statusEvents(t, k, maPhieuThu), ","); got != want {
				t.Errorf("events = %s, want %s", got, want)
			}
			assertUnmergeCountScoped(t, k, idPhieuThu)
		})
	}
}

// The reopening by rating keeps its number (new so_lan_mo_lai + 1) when nothing was unmerged, and moves
// past the round an unmerge took when something was.
func TestStatusOccurrenceOnReopenByRating(t *testing.T) {
	for _, c := range []struct {
		name     string
		unmerges int64
		want     string
	}{
		{"never unmerged", 0, "dang-xu-ly:3"}, // so_lan_mo_lai 1 → 2, + 1
		{"unmerged once", 1, "dang-xu-ly:4"},  // + the unmerge's round
	} {
		t.Run(c.name, func(t *testing.T) {
			k := khoPhieuMau()
			k.hang = dongPhieuMau(ratedPetition(domain.ChoDanXacNhan, nil))
			n := c.unmerges
			k.mergeCountOverride = &n
			uc, ctx := buildRating(t, k)
			if _, err := uc.Rate(ctx, maPhieuThu, RatingRequest{Stars: 1}, citizenActor()); err != nil {
				t.Fatalf("Rate: %v", err)
			}
			if got := strings.Join(statusEvents(t, k, maPhieuThu), ","); got != c.want {
				t.Errorf("events = %s, want %s", got, c.want)
			}
			assertUnmergeCountScoped(t, k, idPhieuThu)
		})
	}
}

// THE DEFECT: follow the main into `cho-dan-xac-nhan` (1), unmerge back to `dang-xu-ly`, work it to
// `da-xu-ly` and `cho-dan-xac-nhan` again. Under so_lan_mo_lai + 1 the second entry was 1 again and comms
// dropped it — the citizen was never asked to confirm the work actually done on THEIR petition.
func TestStatusOccurrenceAfterFollowUnmergeAndReentry(t *testing.T) {
	k := mergeFixtures(map[string]any{"trang_thai": string(domain.DaXuLy), "xu_ly_xong_luc": mocThaoTac}, nil)
	k.children = []map[string]driver.Value{childRow(idChild, codeChild, string(domain.DangXuLy), map[string]any{
		"linh_vuc": "rac-thai", "han_xu_ly_xong": mergeChildDue, "phan_loai_luc": mocThaoTac})}
	child := k.byKey[codeChild]
	for col, v := range map[string]any{"linh_vuc": "rac-thai", "phan_loai_luc": mocThaoTac,
		"merged_into": idMain, "merged_at": mocThaoTac, "merged_by": "CB-00001"} {
		child[col] = v
	}

	// 1. The main advances da-xu-ly → cho-dan-xac-nhan; the merged petition follows.
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.TienTrangThai(ctx, codeMain, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatalf("main advance: %v", err)
	}
	child["trang_thai"], child["xu_ly_xong_luc"] = string(domain.ChoDanXacNhan), mocThaoTac
	k.children = nil // the fake answers EVERY merged_into read with this list; nothing is merged into the child

	// 2. Unmerged out of cho-dan-xac-nhan → dang-xu-ly (not a reopening: so_lan_mo_lai stays 0).
	if _, err := uc.Unmerge(ctx, codeChild, "Không cùng vụ việc", canBoThu(), false); err != nil {
		t.Fatalf("Unmerge: %v", err)
	}
	child["trang_thai"], child["xu_ly_xong_luc"] = string(domain.DangXuLy), nil
	child["merged_into"], child["merged_at"], child["merged_by"] = nil, nil, nil

	// 3. Worked on its own: dang-xu-ly → da-xu-ly → cho-dan-xac-nhan.
	if _, err := uc.TienTrangThai(ctx, codeChild, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatalf("child to da-xu-ly: %v", err)
	}
	child["trang_thai"], child["xu_ly_xong_luc"] = string(domain.DaXuLy), mocThaoTac
	if _, err := uc.TienTrangThai(ctx, codeChild, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatalf("child to cho-dan-xac-nhan: %v", err)
	}

	want := "cho-dan-xac-nhan:1,dang-xu-ly:2,da-xu-ly:2,cho-dan-xac-nhan:2"
	if got := strings.Join(statusEvents(t, k, codeChild), ","); got != want {
		t.Errorf("events = %s\nwant      %s", got, want)
	}
	assertUnmergeCountScoped(t, k, idChild)
}
