package app

import (
	"context"
	"database/sql/driver"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Merge, unmerge and the follow-along step (petition_merge.go) — the REAL use case over the REAL store
// over driver_gia_phieu_test.go's fake driver.
//
//	PROVED HERE   the lock order (main, then merged) · the rules refuse BEFORE any write and roll back
//	              empty · the main's deadline moves to the earlier one BEFORE the link, never later, and not
//	              at all when nothing moves · the 0004 conflict is refused with nothing written · link,
//	              history row and both audit entries in ONE transaction, a failure on any of them commits
//	              nothing · the actor in the link, the history and the trail is the BUSINESS code · the
//	              reason never reaches the trail · unmerge needs its reason before any SQL and keeps the
//	              main's deadline · merged petitions follow their main into `cho-dan-xac-nhan` / `da-dong`
//	              in the SAME transaction, with the same result, each with its own trail, timeline row and
//	              its own outbox row carrying its OWN code · ended / already-there petitions stay put.
//	NOT PROVED    what PostgreSQL does — migration 0037's trigger and CHECKs are the floor
//	              (migrations/petition_merge_test.go checks them as text).

const (
	idMain       = "01JPHIEUCHINHTRONGTEST000"
	codeMain     = "PA-MAIN-7K2Q-9WXR"
	idChild      = "01JPHIEUPHUTRONGTEST00000"
	codeChild    = "PA-CHLD-4M8T-3HQZ"
	idChild2     = "01JPHIEUPHUTHUHAI00000000"
	codeChild2   = "PA-CHLD-2B6N-7PDW"
	citizenChild = "cd-01JCONGDANPHIEUPHU"
)

var (
	mergeMainDue  = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)  // the main petition's deadline
	mergeChildDue = time.Date(2026, 9, 29, 3, 30, 0, 0, time.UTC) // earlier — the main must take it
	mergeChildGoc = time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC)  // reported before the main
)

// mergeFixtures is a main petition being worked on and an unclassified merged-to-be one, both of commune
// xaThu, keyed by code AND id (the merge reads by code, the unmerge locks the main by id).
func mergeFixtures(main, child map[string]any) *khoPhieuXuLyGia {
	m := dongPhieuMau(map[string]any{
		"id": idMain, "ma_tra_cuu": codeMain, "trang_thai": string(domain.DangXuLy), "linh_vuc": "rac-thai",
		"han_xu_ly_xong": mergeMainDue, "phan_loai_luc": mocThaoTac,
	})
	c := dongPhieuMau(map[string]any{
		"id": idChild, "ma_tra_cuu": codeChild, "cong_dan_id": citizenChild,
		"han_xu_ly_xong": mergeChildDue, "goc_dem_han": mocGocThu, "vao_so_luc": mocGocThu,
	})
	for k, v := range main {
		m[k] = v
	}
	for k, v := range child {
		c[k] = v
	}
	k := khoPhieuMau()
	k.hang = nil
	k.byKey = map[string]map[string]driver.Value{codeMain: m, idMain: m, codeChild: c, idChild: c}
	return k
}

func mergeNow(t *testing.T, k *khoPhieuXuLyGia, req MergeRequest, restricted QuyenXemHanChe) (domain.PhieuPhanAnh, error) {
	t.Helper()
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	return uc.Merge(ctx, codeChild, req, canBoThu(), restricted)
}

func TestMergeWritesEverythingInOneTransactionInOrder(t *testing.T) {
	k := mergeFixtures(nil, nil)
	after, err := mergeNow(t, k, MergeRequest{MainCode: codeMain, Reason: "Cùng ổ gà trước nhà văn hoá thôn"}, false)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if after.MergedInto != idMain || after.MergedBy != maCanBoThu || !after.MergedAt.Equal(mocThaoTac) {
		t.Errorf("link = %q %q %v", after.MergedInto, after.MergedBy, after.MergedAt)
	}
	if after.HanXuLyXong != mergeChildDue {
		t.Error("the merged petition's own deadline changed")
	}

	// ORDER: lock main, lock child, chain check, deadline, link, history, two audit entries.
	var order []string
	for _, l := range k.lenh {
		if !l.trongGiaoDich {
			t.Errorf("statement outside the transaction: %s", l.sql)
		}
		switch {
		case strings.Contains(l.sql, "FOR UPDATE"):
			order = append(order, "lock:"+l.args[1].(string))
		case strings.Contains(l.sql, "SELECT id FROM phieu_phan_anh"):
			order = append(order, "chain")
		case strings.Contains(l.sql, "SET han_xu_ly_xong"):
			order = append(order, "deadline")
		case strings.Contains(l.sql, "SET merged_into = $3"):
			order = append(order, "link")
		case strings.Contains(l.sql, "INSERT INTO petition_merge_event"):
			order = append(order, "history")
		case strings.Contains(l.sql, "INSERT INTO audit_log"):
			order = append(order, "audit:"+l.args[5].(string))
		case strings.Contains(l.sql, "INSERT INTO nhat_ky_phan_anh"):
			order = append(order, "timeline:"+l.args[2].(string))
		case strings.Contains(l.sql, "count(*) FROM petition_merge_event"):
			order = append(order, "count")
		case strings.Contains(l.sql, "INSERT INTO su_kien_di"):
			order = append(order, "outbox")
		default:
			order = append(order, "?"+l.sql)
		}
	}
	want := "lock:" + codeMain + ",lock:" + codeChild + ",chain,deadline,link,history,audit:" + codeChild +
		",audit:" + codeMain + ",timeline:" + idChild + ",timeline:" + idMain + ",count,outbox"
	if got := strings.Join(order, ","); got != want {
		t.Errorf("order =\n %s\nwant\n %s", got, want)
	}
	if k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("commit=%d rollback=%d", k.daCommit, k.daRollback)
	}

	dl := k.cau("SET han_xu_ly_xong")[0]
	if dl.args[1] != idMain || dl.args[2] != mergeChildDue || dl.args[3] != mergeMainDue {
		t.Errorf("deadline args = %v — main takes the EARLIER, guarded by what was read", dl.args)
	}
	link := k.cau("SET merged_into = $3")[0]
	if link.args[1] != idChild || link.args[2] != idMain || link.args[4] != maCanBoThu || link.args[5] != string(domain.DaTiepNhan) {
		t.Errorf("link args = %v", link.args)
	}
	h := k.cau("INSERT INTO petition_merge_event")[0]
	if h.args[2] != idChild || h.args[3] != idMain || h.args[4] != "gop-phieu" || h.args[6] != maCanBoThu ||
		h.args[8] != mergeMainDue || h.args[9] != mergeChildDue {
		t.Errorf("history args = %v", h.args)
	}
	for _, a := range k.cau("INSERT INTO audit_log") {
		if a.args[1] != maCanBoThu {
			t.Errorf("audit actor = %v — the BUSINESS code (rule 6, invariant 8)", a.args[1])
		}
		if strings.Contains(string(a.args[7].([]byte)), "ổ gà") {
			t.Error("the reason text reached the append-only trail (rule 6, forbidden #4)")
		}
	}
	if !strings.Contains(string(k.cau("INSERT INTO audit_log")[0].args[7].([]byte)), codeMain) {
		t.Error("the merged petition's entry does not name the main petition")
	}
	assertMergeTimelineRows(t, k, "gop-phieu", string(domain.DaTiepNhan), string(domain.DangXuLy),
		"Gộp vào phiếu chính "+codeMain, "Nhận phiếu "+codeChild+" gộp vào", "ổ gà")
	assertMergeChangedRow(t, k.cau("INSERT INTO su_kien_di")[0], "gop-phieu", 1,
		"Ghép với phản ánh cùng vụ việc",
		"Phản ánh của anh/chị đã được ghép với phản ánh cùng vụ việc. Kết quả sẽ được báo khi xử lý xong.", "ổ gà")
	if n := len(k.cau("INSERT INTO su_kien_di")); n != 1 {
		t.Errorf("%d outbox rows, want ONE — the merged petition's merge_changed, never a status_changed", n)
	}
}

// assertMergeTimelineRows: one row on EACH petition, child first, each with its own status, no assignment
// pair, a fixed sentence naming the OTHER petition — and never the staff reason.
func assertMergeTimelineRows(t *testing.T, k *khoPhieuXuLyGia, action, childStatus, mainStatus,
	childText, mainText, reasonFragment string) {
	t.Helper()
	rows := k.cau("INSERT INTO nhat_ky_phan_anh")
	if len(rows) != 2 {
		t.Fatalf("%d timeline rows, want 2 — one on each petition", len(rows))
	}
	for i, w := range []struct{ id, status, text string }{{idChild, childStatus, childText}, {idMain, mainStatus, mainText}} {
		r := rows[i]
		if !r.trongGiaoDich || r.args[2] != w.id || r.args[4] != maCanBoThu || r.args[5] != action ||
			r.args[6] != w.status || r.args[7] != nil || r.args[8] != nil || r.args[9] != w.text {
			t.Errorf("timeline row %d = %v", i, r.args)
		}
		if s, _ := r.args[9].(string); reasonFragment != "" && strings.Contains(s, reasonFragment) {
			t.Error("the staff reason reached the timeline — it lives on petition_merge_event only")
		}
	}
}

// assertMergeChangedRow decodes one `petitions.merge_changed.v1` outbox row: the CHILD's code and citizen,
// kind, occurrence, the owner's wording — and nothing of the main petition, nothing of the reason.
func assertMergeChangedRow(t *testing.T, row lenhPhieu, kind string, occurrence float64, label, next, reasonFragment string) {
	t.Helper()
	if row.args[2] != "petitions.merge_changed.v1" || row.args[3] != codeChild || !row.trongGiaoDich {
		t.Errorf("outbox row = %v %v (in tx %v)", row.args[2], row.args[3], row.trongGiaoDich)
	}
	body := thanSuKien(t, row.args)
	msg, _ := body["citizen_message"].(map[string]any)
	if body["lookup_code"] != codeChild || body["kind"] != kind || body["occurrence"] != occurrence ||
		body["citizen_id"] != citizenChild || msg["status_label"] != label || msg["next_step"] != next || len(body) != 5 {
		t.Errorf("merge_changed body = %v", body)
	}
	raw := string(row.args[4].([]byte))
	if strings.Contains(raw, codeMain) || strings.Contains(raw, idMain) {
		t.Error("the main petition reached the merged petition's citizen (ADR 0087 §4)")
	}
	if reasonFragment != "" && strings.Contains(raw, reasonFragment) {
		t.Error("the staff reason reached the queue (rule 3)")
	}
}

// A petition with no citizen account behind it has nobody to tell: no outbox row — the timeline and the
// trail are still written.
func TestMergeWithoutCitizenWritesNoOutboxRow(t *testing.T) {
	k := mergeFixtures(nil, map[string]any{"cong_dan_id": nil})
	if _, err := mergeNow(t, k, MergeRequest{MainCode: codeMain}, false); err != nil {
		t.Fatal(err)
	}
	if k.coCau("INSERT INTO su_kien_di") || k.coCau("FROM petition_merge_event") {
		t.Error("an outbox row (or its count) for a petition nobody can be told about")
	}
	if len(k.cau("INSERT INTO nhat_ky_phan_anh")) != 2 || k.daCommit != 1 {
		t.Error("the act itself was not recorded")
	}
}

// OCCURRENCE is per (petition, kind), counted from the append-only history INCLUDING the act's own row:
// merge → unmerge → merge again is gop 1, tach 1, gop 2 — the second merge is a NEW notice, never a
// duplicate of the first in comms.
func TestMergeOccurrenceCountsPerKindAcrossActs(t *testing.T) {
	k := mergeFixtures(nil, nil)
	child := k.byKey[codeChild]
	link := func(on bool) {
		if on {
			child["merged_into"], child["merged_at"], child["merged_by"] = idMain, mocThaoTac, maCanBoThu
			return
		}
		child["merged_into"], child["merged_at"], child["merged_by"] = nil, nil, nil
	}
	if _, err := mergeNow(t, k, MergeRequest{MainCode: codeMain}, false); err != nil {
		t.Fatal(err)
	}
	link(true)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.Unmerge(ctx, codeChild, "Chọn nhầm", canBoThu(), false); err != nil {
		t.Fatal(err)
	}
	link(false)
	if _, err := mergeNow(t, k, MergeRequest{MainCode: codeMain}, false); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range k.cau("INSERT INTO su_kien_di") {
		b := thanSuKien(t, r.args)
		got = append(got, b["kind"].(string)+":"+strconv.Itoa(int(b["occurrence"].(float64))))
	}
	if strings.Join(got, ",") != "gop-phieu:1,tach-phieu:1,gop-phieu:2" {
		t.Errorf("occurrences = %v", got)
	}
}

// A count of 0 right after the append is a wiring fault: refused, the whole act rolled back — never sent
// as occurrence 0 or defaulted to 1.
func TestMergeOccurrenceZeroRollsBack(t *testing.T) {
	k := mergeFixtures(nil, nil)
	zero := int64(0)
	k.mergeCountOverride = &zero
	if _, err := mergeNow(t, k, MergeRequest{MainCode: codeMain}, false); !errors.Is(err, errMergeOccurrenceZero) {
		t.Fatalf("err = %v", err)
	}
	if k.daCommit != 0 || k.daRollback != 1 || k.coCau("INSERT INTO su_kien_di") {
		t.Errorf("commit=%d rollback=%d", k.daCommit, k.daRollback)
	}
}

func TestMergeDeadlineNullTakesTheOthersAndNothingMovesWhenAlreadyEarlier(t *testing.T) {
	// Main "chưa có" -> takes the merged petition's.
	k := mergeFixtures(map[string]any{"han_xu_ly_xong": nil, "trang_thai": string(domain.DaTiepNhan)}, nil)
	if _, err := mergeNow(t, k, MergeRequest{MainCode: codeMain}, false); err != nil {
		t.Fatal(err)
	}
	dl := k.cau("SET han_xu_ly_xong")
	if len(dl) != 1 || dl[0].args[2] != mergeChildDue || dl[0].args[3] != nil {
		t.Errorf("NULL main: %v", dl)
	}

	// Merged petition "chưa có" -> the main keeps its own; no deadline statement at all.
	k = mergeFixtures(nil, map[string]any{"han_xu_ly_xong": nil})
	if _, err := mergeNow(t, k, MergeRequest{MainCode: codeMain}, false); err != nil {
		t.Fatal(err)
	}
	if k.coCau("SET han_xu_ly_xong") {
		t.Error("the main's deadline was written although nothing moves")
	}
	if h := k.cau("INSERT INTO petition_merge_event")[0]; h.args[8] != mergeMainDue || h.args[9] != mergeMainDue {
		t.Errorf("history deadlines = %v %v", h.args[8], h.args[9])
	}

	// Main already earlier -> no deadline statement.
	k = mergeFixtures(map[string]any{"han_xu_ly_xong": mergeChildDue.Add(-time.Hour)}, nil)
	if _, err := mergeNow(t, k, MergeRequest{MainCode: codeMain}, false); err != nil {
		t.Fatal(err)
	}
	if k.coCau("SET han_xu_ly_xong") {
		t.Error("a later deadline was written onto the main")
	}
}

// Migration 0004's `han_xu_ly_xong >= goc_dem_han`: refused FIRST, with nothing written.
func TestMergeDeadlineBeforeMainOriginRefusedWithNothingWritten(t *testing.T) {
	k := mergeFixtures(map[string]any{"goc_dem_han": mergeChildDue.Add(time.Hour), "vao_so_luc": mergeChildDue.Add(time.Hour)},
		map[string]any{"goc_dem_han": mergeChildGoc, "vao_so_luc": mergeChildGoc})
	_, err := mergeNow(t, k, MergeRequest{MainCode: codeMain}, false)
	if !errors.Is(err, domain.ErrMergeDeadlineBeforeOrigin) {
		t.Fatalf("err = %v", err)
	}
	assertNothingWritten(t, k)
}

func assertNothingWritten(t *testing.T, k *khoPhieuXuLyGia) {
	t.Helper()
	for _, w := range []string{"UPDATE phieu_phan_anh", "INSERT INTO petition_merge_event", "INSERT INTO audit_log",
		"INSERT INTO su_kien_di", "INSERT INTO nhat_ky_phan_anh"} {
		if k.coCau(w) {
			t.Errorf("a refusal wrote %q", w)
		}
	}
	if k.daCommit != 0 {
		t.Errorf("commit=%d after a refusal", k.daCommit)
	}
}

func TestMergeRefusals(t *testing.T) {
	linked := map[string]any{"merged_into": "01JOTHER", "merged_at": mocThaoTac, "merged_by": "CB-1"}
	for name, c := range map[string]struct {
		main, child map[string]any
		children    bool
		restricted  QuyenXemHanChe
		req         MergeRequest
		want        error
	}{
		"main code missing":                   {req: MergeRequest{MainCode: "  "}, want: domain.ErrMergeMainMissing},
		"into itself":                         {req: MergeRequest{MainCode: codeChild}, want: domain.ErrMergeSelf},
		"main unknown or another commune's":   {req: MergeRequest{MainCode: "PA-XXXX-XXXX-XXXX"}, want: petstore.ErrPhieuKhongTonTai},
		"main resolved":                       {main: map[string]any{"trang_thai": string(domain.DaXuLy)}, want: domain.ErrMergeNotOpen},
		"child closed":                        {child: map[string]any{"trang_thai": string(domain.DaDong)}, want: domain.ErrMergeNotOpen},
		"child already merged":                {child: linked, want: domain.ErrMergeAlreadyMerged},
		"main is itself merged":               {main: linked, want: domain.ErrMergeTargetIsMerged},
		"child has merged petitions":          {children: true, want: domain.ErrMergeHasChildren},
		"can-bo, caller without the key":      {child: map[string]any{"linh_vuc": domain.LinhVucHanChe}, want: ErrPhieuHanChe},
		"can-bo main, caller without the key": {main: map[string]any{"linh_vuc": domain.LinhVucHanChe}, want: ErrPhieuHanChe},
		"can-bo, caller WITH the key":         {child: map[string]any{"linh_vuc": domain.LinhVucHanChe}, restricted: true, want: domain.ErrMergeStaffConduct},
		"reason too long": {req: MergeRequest{MainCode: codeMain, Reason: strings.Repeat("x", domain.MergeReasonMax+1)},
			want: domain.ErrMergeReasonTooLong},
	} {
		t.Run(name, func(t *testing.T) {
			k := mergeFixtures(c.main, c.child)
			if c.children {
				k.children = []map[string]driver.Value{{"id": "01JGRANDCHILD"}}
			}
			req := c.req
			if req.MainCode == "" {
				req.MainCode = codeMain
			}
			_, err := mergeNow(t, k, req, c.restricted)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			assertNothingWritten(t, k)
		})
	}
}

// ATOMICITY: a failure on the history row or on the trail takes the deadline and the link down with it.
func TestMergeFailureAnywhereCommitsNothing(t *testing.T) {
	for _, fail := range []string{"INSERT INTO petition_merge_event", "INSERT INTO audit_log", "SET merged_into = $3",
		"INSERT INTO nhat_ky_phan_anh", "count(*) FROM petition_merge_event", "INSERT INTO su_kien_di"} {
		t.Run(fail, func(t *testing.T) {
			k := mergeFixtures(nil, nil)
			k.loiSau = fail
			if _, err := mergeNow(t, k, MergeRequest{MainCode: codeMain}, false); err == nil {
				t.Fatal("no error")
			}
			if k.daCommit != 0 || k.daRollback != 1 {
				t.Errorf("commit=%d rollback=%d — a half-merged incident", k.daCommit, k.daRollback)
			}
		})
	}
}

// --- unmerge ---------------------------------------------------------------------------------------------

func linkedChild() map[string]any {
	return map[string]any{"merged_into": idMain, "merged_at": mocThaoTac, "merged_by": "CB-00001"}
}

func TestUnmergeReasonRequiredBeforeAnySQL(t *testing.T) {
	k := mergeFixtures(nil, linkedChild())
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.Unmerge(ctx, codeChild, " \n", canBoThu(), false); !errors.Is(err, domain.ErrUnmergeReasonMissing) {
		t.Fatalf("err = %v", err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("%d statements ran for a refused unmerge", len(k.lenh))
	}
}

func TestUnmergeClearsTheLinkKeepsTheDeadlineAndRecords(t *testing.T) {
	k := mergeFixtures(nil, linkedChild())
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	after, err := uc.Unmerge(ctx, codeChild, "Chọn nhầm phiếu chính", canBoThu(), false)
	if err != nil {
		t.Fatalf("Unmerge: %v", err)
	}
	if after.MergedInto != "" || after.MergedBy != "" || !after.MergedAt.IsZero() {
		t.Errorf("link still set: %+v", after)
	}
	locks := k.cau("FOR UPDATE")
	if len(locks) != 2 || locks[0].args[1] != idMain || locks[1].args[1] != codeChild {
		t.Errorf("lock order = %v — main first, by id; then the merged petition", locks)
	}
	if un := k.cau("SET merged_into = NULL"); len(un) != 1 || un[0].args[1] != idChild || un[0].args[2] != idMain {
		t.Errorf("unlink = %v", un)
	}
	h := k.cau("INSERT INTO petition_merge_event")[0]
	if h.args[4] != "tach-phieu" || h.args[7] != "Chọn nhầm phiếu chính" || h.args[8] != mergeMainDue || h.args[9] != mergeMainDue {
		t.Errorf("history = %v — reason kept on the history, main deadline NOT lengthened back", h.args)
	}
	audits := k.cau("INSERT INTO audit_log")
	if len(audits) != 2 || audits[0].args[4] != ActionUnmergePetition || audits[1].args[4] != ActionUnmergeFromMain {
		t.Errorf("audit = %v", audits)
	}
	if k.coCau("SET han_xu_ly_xong") {
		t.Error("unmerge moved a deadline")
	}
	if strings.Contains(k.cau("SET merged_into = NULL")[0].sql, "trang_thai = '") {
		t.Error("an open petition's status was rewritten by the unmerge")
	}
	assertMergeTimelineRows(t, k, "tach-phieu", string(domain.DaTiepNhan), string(domain.DangXuLy),
		"Tách khỏi phiếu chính "+codeMain, "Tách phiếu "+codeChild+" khỏi phiếu này", "nhầm")
	su := k.cau("INSERT INTO su_kien_di")
	if len(su) != 1 {
		t.Fatalf("%d outbox rows, want ONE merge_changed — no status moved", len(su))
	}
	assertMergeChangedRow(t, su[0], "tach-phieu", 1, "Xử lý riêng",
		"Phản ánh của anh/chị sẽ được xử lý riêng. Kết quả sẽ được báo khi xử lý xong.", "nhầm")
	if k.daCommit != 1 {
		t.Errorf("commit=%d", k.daCommit)
	}
}

// Owner, 09/10/2026 (c): a merged petition that followed its main into `cho-dan-xac-nhan` may still be
// unmerged — and returns to `dang-xu-ly` along the lifecycle's EXISTING edge, in the unlink's own
// statement. Not a reopening: the rating's counter is untouched and no reopen row is written. The deadline
// stays as stored; the borrowed finishing instant is cleared. The citizen is still told.
func TestUnmergeFromAwaitingConfirmationReturnsToProcessing(t *testing.T) {
	linked := linkedChild()
	linked["trang_thai"], linked["xu_ly_xong_luc"] = string(domain.ChoDanXacNhan), mocThaoTac
	k := mergeFixtures(nil, linked)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	after, err := uc.Unmerge(ctx, codeChild, "Không cùng vụ việc", canBoThu(), false)
	if err != nil {
		t.Fatalf("Unmerge: %v", err)
	}
	if after.TrangThai != domain.DangXuLy || !after.XuLyXongLuc.IsZero() || after.MergedInto != "" ||
		!after.HanXuLyXong.Equal(mergeChildDue) {
		t.Errorf("after = %s %v %q %v", after.TrangThai, after.XuLyXongLuc, after.MergedInto, after.HanXuLyXong)
	}
	un := k.cau("SET merged_into = NULL")
	if len(un) != 1 || !strings.Contains(un[0].sql, "trang_thai = 'dang-xu-ly'") ||
		!strings.Contains(un[0].sql, "xu_ly_xong_luc = NULL") || !strings.Contains(un[0].sql, "trang_thai = 'cho-dan-xac-nhan'") ||
		strings.Contains(un[0].sql, "so_lan_mo_lai") || strings.Contains(un[0].sql, " han_") ||
		len(un[0].args) != 3 || un[0].args[1] != idChild || un[0].args[2] != idMain {
		t.Fatalf("unlink = %v", un)
	}
	audits := k.cau("INSERT INTO audit_log")
	if d := string(audits[0].args[7].([]byte)); !strings.Contains(d, `"truoc":{"trang_thai":"cho-dan-xac-nhan"}`) ||
		!strings.Contains(d, `"sau":{"trang_thai":"dang-xu-ly"}`) {
		t.Errorf("the transition is not in the merged petition's trail: %s", d)
	}
	assertMergeTimelineRows(t, k, "tach-phieu", string(domain.DangXuLy), string(domain.DangXuLy),
		"Tách khỏi phiếu chính "+codeMain, "Tách phiếu "+codeChild+" khỏi phiếu này", "vụ việc")
	for _, r := range k.cau("INSERT INTO nhat_ky_phan_anh") {
		if r.args[5] == string(domain.LogActionReopenByRating) {
			t.Error("an unmerge was recorded as a reopening by rating")
		}
	}
	su := k.cau("INSERT INTO su_kien_di")
	if len(su) != 2 {
		t.Fatalf("%d outbox rows, want 2 — the transition's status_changed and the unmerge's merge_changed", len(su))
	}
	sc := thanSuKien(t, su[0].args)
	if su[0].args[2] != "petitions.status_changed.v1" || sc["status"] != string(domain.DangXuLy) || sc["citizen_message"] != nil {
		t.Errorf("status_changed = %v — entering dang-xu-ly is silent (ADR 0041 §Không báo)", sc)
	}
	assertMergeChangedRow(t, su[1], "tach-phieu", 1, "Xử lý riêng",
		"Phản ánh của anh/chị sẽ được xử lý riêng. Kết quả sẽ được báo khi xử lý xong.", "vụ việc")
	if k.daCommit != 1 {
		t.Errorf("commit=%d", k.daCommit)
	}
}

// A merged petition at `da-xu-ly` is unmerged where it stands — nothing moves.
func TestUnmergeAtResolvedKeepsTheStatus(t *testing.T) {
	linked := linkedChild()
	linked["trang_thai"], linked["xu_ly_xong_luc"] = string(domain.DaXuLy), mocThaoTac
	k := mergeFixtures(nil, linked)
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	after, err := uc.Unmerge(ctx, codeChild, "Lý do", canBoThu(), false)
	if err != nil {
		t.Fatal(err)
	}
	if after.TrangThai != domain.DaXuLy || after.XuLyXongLuc.IsZero() {
		t.Errorf("after = %s %v", after.TrangThai, after.XuLyXongLuc)
	}
	if un := k.cau("SET merged_into = NULL")[0]; strings.Contains(un.sql, "'dang-xu-ly'") || un.args[3] != string(domain.DaXuLy) {
		t.Errorf("unlink = %v", un)
	}
	if n := len(k.cau("INSERT INTO su_kien_di")); n != 1 {
		t.Errorf("%d outbox rows, want 1", n)
	}
}

// The unmerge's rollback takes the unlink, the timeline rows and the outbox rows down together.
func TestUnmergeFailureAnywhereCommitsNothing(t *testing.T) {
	for _, fail := range []string{"INSERT INTO petition_merge_event", "INSERT INTO audit_log",
		"INSERT INTO nhat_ky_phan_anh", "count(*) FROM petition_merge_event", "INSERT INTO su_kien_di"} {
		t.Run(fail, func(t *testing.T) {
			linked := linkedChild()
			linked["trang_thai"] = string(domain.ChoDanXacNhan)
			k := mergeFixtures(nil, linked)
			k.loiSau = fail
			uc, ctx := dungXuLy(t, k, hanXuLyThu())
			if _, err := uc.Unmerge(ctx, codeChild, "Lý do", canBoThu(), false); err == nil {
				t.Fatal("no error")
			}
			if k.daCommit != 0 || k.daRollback != 1 {
				t.Errorf("commit=%d rollback=%d — a half-unmerged petition", k.daCommit, k.daRollback)
			}
		})
	}
}

func TestUnmergeRefusals(t *testing.T) {
	ended := func(s domain.TrangThai) map[string]any {
		m := linkedChild()
		m["trang_thai"] = string(s)
		return m
	}
	for name, c := range map[string]struct {
		child map[string]any
		want  error
	}{
		"not merged":          {nil, domain.ErrNotMerged},
		"followed into close": {ended(domain.DaDong), domain.ErrUnmergeNotOpen},
		"refused on its own":  {ended(domain.KhongTiepNhan), domain.ErrUnmergeNotOpen},
		"referred on its own": {ended(domain.ChuyenCapTren), domain.ErrUnmergeNotOpen},
	} {
		t.Run(name, func(t *testing.T) {
			k := mergeFixtures(nil, c.child)
			uc, ctx := dungXuLy(t, k, hanXuLyThu())
			if _, err := uc.Unmerge(ctx, codeChild, "Lý do", canBoThu(), false); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			assertNothingWritten(t, k)
		})
	}
}

// --- follow-along --------------------------------------------------------------------------------------

func childRow(id, code, status string, extra map[string]any) map[string]driver.Value {
	r := dongPhieuMau(map[string]any{"id": id, "ma_tra_cuu": code, "trang_thai": status, "cong_dan_id": citizenChild,
		"merged_into": idMain, "merged_at": mocThaoTac, "merged_by": "CB-00001"})
	for k, v := range extra {
		r[k] = v
	}
	return r
}

func mainRow(status domain.TrangThai) map[string]driver.Value {
	return dongPhieuMau(map[string]any{"id": idMain, "ma_tra_cuu": codeMain, "trang_thai": string(status),
		"linh_vuc": "rac-thai", "han_xu_ly_xong": mergeMainDue, "phan_loai_luc": mocThaoTac, "xu_ly_xong_luc": mocThaoTac})
}

func TestMainIntoAwaitingConfirmationTakesItsMergedPetitions(t *testing.T) {
	k := khoPhieuMau()
	k.hang = mainRow(domain.DaXuLy)
	k.children = []map[string]driver.Value{
		childRow(idChild, codeChild, string(domain.DaTiepNhan), nil),
		childRow(idChild2, codeChild2, string(domain.KhongTiepNhan), map[string]any{
			"ly_do_ket_thuc_nhanh": "Không thuộc thẩm quyền", "ket_thuc_nhanh_luc": mocThaoTac}),
	}
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.TienTrangThai(ctx, codeMain, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatalf("TienTrangThai: %v", err)
	}
	follow := k.cau("SET trang_thai = 'cho-dan-xac-nhan'")
	if len(follow) != 1 || follow[0].args[1] != idChild || follow[0].args[2] != mocThaoTac ||
		follow[0].args[3] != idMain || follow[0].args[4] != string(domain.DaTiepNhan) {
		t.Fatalf("follow = %v — ONE merged petition (the refused one stays), guarded by the link and its status", follow)
	}
	if !follow[0].trongGiaoDich {
		t.Error("the merged petition moved outside the main's transaction")
	}
	// Two outbox rows: the main's and the merged petition's OWN, with ITS code and ITS citizen.
	su := k.cau("INSERT INTO su_kien_di")
	if len(su) != 2 {
		t.Fatalf("%d outbox rows, want 2 — each citizen is told about their own petition", len(su))
	}
	than := thanSuKien(t, su[1].args)
	if than["lookup_code"] != codeChild || than["citizen_id"] != citizenChild || than["status"] != string(domain.ChoDanXacNhan) {
		t.Errorf("merged petition's event = %v", than)
	}
	if strings.Contains(string(su[1].args[4].([]byte)), codeMain) {
		t.Error("the merged petition's citizen is told the MAIN petition's code (ADR 0087 §4)")
	}
	audits := k.cau("INSERT INTO audit_log")
	if len(audits) != 2 || audits[1].args[5] != codeChild || !strings.Contains(string(audits[1].args[7].([]byte)), codeMain) {
		t.Errorf("audit = %v — the merged petition's own entry, naming the main it followed", audits)
	}
	if len(k.cau("INSERT INTO nhat_ky_phan_anh")) != 2 {
		t.Error("the merged petition has no timeline row of its own")
	}
	if k.daCommit != 1 {
		t.Errorf("commit=%d", k.daCommit)
	}
}

func TestMainClosedClosesItsMergedPetitionsWithTheSameResult(t *testing.T) {
	k := khoPhieuMau()
	k.hang = mainRow(domain.ChoDanXacNhan)
	k.children = []map[string]driver.Value{
		childRow(idChild, codeChild, string(domain.ChoDanXacNhan), map[string]any{"xu_ly_xong_luc": mocThaoTac}),
		childRow(idChild2, codeChild2, string(domain.DaDong), map[string]any{"ket_qua_xu_ly": ketQuaThat, "dong_luc": mocThaoTac}),
	}
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.Dong(ctx, codeMain, ketQuaThat, "", canBoThu(), khongQuyenHanChe); err != nil {
		t.Fatalf("Dong: %v", err)
	}
	follow := k.cau("AND merged_into = $6")
	if len(follow) != 1 || !strings.Contains(follow[0].sql, "SET trang_thai = 'da-dong'") || follow[0].args[1] != idChild || follow[0].args[2] != ketQuaThat || follow[0].args[5] != idMain {
		t.Fatalf("follow = %v — the open merged petition closes with the SAME result; the closed one is left", follow)
	}
	su := k.cau("INSERT INTO su_kien_di")
	if len(su) != 2 || thanSuKien(t, su[1].args)["lookup_code"] != codeChild {
		t.Errorf("outbox = %d rows", len(su))
	}
	for _, a := range k.cau("INSERT INTO audit_log") {
		if strings.Contains(string(a.args[7].([]byte)), "thu gom") {
			t.Error("the result text reached the trail")
		}
	}
}

// A failure while a merged petition follows rolls the MAIN's act back too: one incident, never half-closed.
func TestFollowFailureRollsBackTheMainAct(t *testing.T) {
	k := khoPhieuMau()
	k.hang = mainRow(domain.ChoDanXacNhan)
	k.children = []map[string]driver.Value{childRow(idChild, codeChild, string(domain.DangXuLy), nil)}
	k.loiSau = "merged_into = $6"
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.Dong(ctx, codeMain, ketQuaThat, "", canBoThu(), khongQuyenHanChe); err == nil {
		t.Fatal("no error")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit=%d rollback=%d", k.daCommit, k.daRollback)
	}
}

// A MERGED petition moving on its own asks for no children (no chains) and moves nothing else.
func TestMergedPetitionMovingAloneFollowsNothing(t *testing.T) {
	k := khoPhieuMau()
	k.hang = dongPhieuMau(map[string]any{"trang_thai": string(domain.DaXuLy), "linh_vuc": "rac-thai",
		"han_xu_ly_xong": mergeMainDue, "phan_loai_luc": mocThaoTac, "xu_ly_xong_luc": mocThaoTac,
		"merged_into": idMain, "merged_at": mocThaoTac, "merged_by": "CB-1"})
	k.children = []map[string]driver.Value{childRow(idChild, codeChild, string(domain.DaTiepNhan), nil)}
	uc, ctx := dungXuLy(t, k, hanXuLyThu())
	if _, err := uc.TienTrangThai(ctx, maPhieuThu, "", canBoThu(), coQuyenCaXa, khongQuyenHanChe); err != nil {
		t.Fatal(err)
	}
	if k.coCau("merged_into = $2") || k.coCau("SET trang_thai = 'cho-dan-xac-nhan'") {
		t.Error("a merged petition looked for, or moved, petitions merged into it")
	}
}

// --- the suspected-duplicate search ------------------------------------------------------------------------

type dupReaderFake struct {
	p    domain.PhieuPhanAnh
	err  error
	rows []domain.PhieuPhanAnh
	q    *petstore.DuplicateQuery
}

func (f *dupReaderFake) TheoMaTraCuu(context.Context, string) (domain.PhieuPhanAnh, error) { // vi-name-ok: implements the existing store method
	return f.p, f.err
}

func (f *dupReaderFake) DuplicateCandidates(_ context.Context, q petstore.DuplicateQuery) ([]domain.PhieuPhanAnh, error) {
	f.q = &q
	return f.rows, nil
}

type thresholdsFake struct {
	t   petstore.DuplicateThresholds
	err error
}

func (f thresholdsFake) DuplicateThresholds(context.Context) (petstore.DuplicateThresholds, error) {
	return f.t, f.err
}

func ptr(f float64) *float64 { return &f }

func TestDuplicateCandidatesRefinesByDistanceClosestFirst(t *testing.T) {
	lat, lng := 15.880123, 108.335456
	src := domain.PhieuPhanAnh{ID: "src", TrangThai: domain.DaTiepNhan, Lat: ptr(lat), Lng: ptr(lng), GocDemHan: mocGocThu}
	near := domain.PhieuPhanAnh{ID: "near", Lat: ptr(lat + 0.0001), Lng: ptr(lng)}         // ~11 m
	mid := domain.PhieuPhanAnh{ID: "mid", Lat: ptr(lat + 0.0003), Lng: ptr(lng)}           // ~33 m
	far := domain.PhieuPhanAnh{ID: "far", Lat: ptr(lat + 0.00049), Lng: ptr(lng + 0.0002)} // in the box, outside 50 m
	r := &dupReaderFake{p: src, rows: []domain.PhieuPhanAnh{mid, far, near}}
	uc := NewDuplicateCandidates(r, thresholdsFake{t: petstore.DuplicateThresholds{RadiusMeters: 50, WindowDays: 7}})
	got, err := uc.Candidates(ctxXa(xaThu), "PA", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || got.Items[0].ID != "near" || got.Items[1].ID != "mid" {
		t.Errorf("items = %+v", got.Items)
	}
	if got.RadiusMeters != 50 || got.WindowDays != 7 || got.Truncated {
		t.Errorf("result = %+v", got)
	}
	if r.q.ExcludeID != "src" || r.q.WindowDays != 7 || !r.q.ReportedAt.Equal(mocGocThu) || r.q.Field != "" {
		t.Errorf("query = %+v", r.q)
	}
}

func TestDuplicateCandidatesEmptyOrRefused(t *testing.T) {
	located := func(p domain.PhieuPhanAnh) domain.PhieuPhanAnh { p.Lat, p.Lng = ptr(15.8), ptr(108.3); return p }
	th := thresholdsFake{t: petstore.DuplicateThresholds{RadiusMeters: 50, WindowDays: 7}}
	for name, p := range map[string]domain.PhieuPhanAnh{
		"no location":      {TrangThai: domain.DaTiepNhan},
		"resolved":         located(domain.PhieuPhanAnh{TrangThai: domain.DaXuLy}),
		"already merged":   located(domain.PhieuPhanAnh{TrangThai: domain.DaTiepNhan, MergedInto: "x"}),
		"can-bo, key held": located(domain.PhieuPhanAnh{TrangThai: domain.DaTiepNhan, LinhVuc: domain.LinhVucHanChe}),
	} {
		r := &dupReaderFake{p: p}
		got, err := NewDuplicateCandidates(r, th).Candidates(ctxXa(xaThu), "PA", true)
		if err != nil || len(got.Items) != 0 || got.Items == nil || r.q != nil {
			t.Errorf("%s: %+v, %v, searched=%v", name, got, err, r.q != nil)
		}
	}
	r := &dupReaderFake{p: located(domain.PhieuPhanAnh{TrangThai: domain.DaTiepNhan, LinhVuc: domain.LinhVucHanChe})}
	if _, err := NewDuplicateCandidates(r, th).Candidates(ctxXa(xaThu), "PA", false); !errors.Is(err, ErrPhieuHanChe) {
		t.Errorf("can-bo without the key: %v — want the unknown code's 404", err)
	}
	cause := errors.New("mất kết nối")
	r = &dupReaderFake{p: located(domain.PhieuPhanAnh{TrangThai: domain.DaTiepNhan})}
	if _, err := NewDuplicateCandidates(r, thresholdsFake{err: cause}).Candidates(ctxXa(xaThu), "PA", false); !errors.Is(err, cause) {
		t.Errorf("thresholds failure answered with a default: %v", err)
	}
}
