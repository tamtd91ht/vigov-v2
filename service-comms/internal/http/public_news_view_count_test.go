package http

// THE VIEW COUNT (owner decision, ADR 0047 row 02/10/2026): the public DETAIL read adds one to
// `luot_xem`, best effort; the list, the banner strip and the categories never do; no count when the
// rate limit could not be consulted or on `no_view=1`. Which ROW the increment touches (this commune,
// published, not deleted) is the store's predicate, proven in internal/store; the fake here applies the
// same predicate so a handler that passed the wrong commune would show up.

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func viewServer(t *testing.T, nd *ckNoiDung, counter *memCounter, log *slog.Logger) http.Handler {
	t.Helper()
	_, dm := ckDuLieu()
	if counter == nil {
		counter = &memCounter{}
	}
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Limiter: ckLimiterOn(counter), Xa: &ckNenTang{}, NoiDung: nd, Views: nd,
		DanhMuc: dm, CoverImages: &fakePublicCovers{}, Audio: &fakePublicAudio{}, Log: log})
	return mux
}

func viewCountOf(t *testing.T, w *httptest.ResponseRecorder) float64 {
	t.Helper()
	var ra map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	n, ok := ra["view_count"].(float64)
	if !ok {
		t.Fatalf("view_count vắng mặt: %q", w.Body.String())
	}
	return n
}

// A list item never read yet must still carry `view_count: 0`: the Mini App reads an ABSENT field as
// "this server does not count" and hides the line, so `omitempty` would hide "0 lượt xem".
func TestPublicListSendsZeroViewCount(t *testing.T) {
	nd, _ := ckDuLieu()
	nd.theoXa[xaA][0].LuotXem = 0
	w := ckGoi(viewServer(t, nd, nil, nil), MauTinXa, ckHostA)
	doiMa(t, w, http.StatusOK)
	it := ckDocTrang(t, w).Items
	if len(it) != 1 {
		t.Fatalf("trang = %v, muốn một mục", it)
	}
	if v, ok := it[0]["view_count"]; !ok || v != float64(0) {
		t.Fatalf("view_count = %v (có mặt: %v), muốn 0 có mặt", v, ok)
	}
}

func TestPublicDetailCountsOneView(t *testing.T) {
	nd, _ := ckDuLieu() // nd-a-1 holds 9
	h := viewServer(t, nd, nil, nil)

	w := ckGoi(h, MauTinXa+"/nd-a-1", ckHostA)
	doiMa(t, w, http.StatusOK)
	if strings.Join(nd.views, ",") != string(xaA)+"/nd-a-1" {
		t.Fatalf("lượt tăng = %v, muốn đúng một lần cho nd-a-1 ở xã A", nd.views)
	}
	if got := viewCountOf(t, w); got != 10 {
		t.Fatalf("view_count = %v, muốn 10 (gồm lượt đang trả lời)", got)
	}
}

// A draft, an item awaiting approval, no such id (what a soft-deleted item is to the store), and B's
// id under A's host: ONE 404, byte-identical, and nothing counted.
func TestPublicDetailNotFoundIsIdenticalAndNotCounted(t *testing.T) {
	nd, _ := ckDuLieu()
	h := viewServer(t, nd, nil, nil)

	ref := ckGoi(h, MauTinXa+"/khong-co", ckHostA)
	doiMa(t, ref, http.StatusNotFound)
	for _, id := range []string{"nd-a-nhap", "nd-a-cho", "nd-b-1"} {
		w := ckGoi(h, MauTinXa+"/"+id, ckHostA)
		doiMa(t, w, http.StatusNotFound)
		if w.Body.String() != ref.Body.String() {
			t.Errorf("%s: 404 khác — %q / %q", id, w.Body.String(), ref.Body.String())
		}
	}
	if len(nd.views) != 0 {
		t.Fatalf("một 404 đã được đếm: %v", nd.views)
	}
}

func TestPublicDetailNoViewFlagIsNotCounted(t *testing.T) {
	nd, _ := ckDuLieu()
	h := viewServer(t, nd, nil, nil)

	w := ckGoi(h, MauTinXa+"/nd-a-1", ckHostA, "&no_view=1")
	doiMa(t, w, http.StatusOK)
	if len(nd.views) != 0 {
		t.Fatalf("no_view=1 vẫn đếm: %v", nd.views)
	}
	if got := viewCountOf(t, w); got != 9 {
		t.Fatalf("view_count = %v, muốn 9 (giá trị đã đọc)", got)
	}
	// Only exactly "1" skips: any other value counts normally.
	for _, v := range []string{"0", "true", "yes", ""} {
		ckGoi(h, MauTinXa+"/nd-a-1", ckHostA, "&no_view="+v)
	}
	if len(nd.views) != 4 {
		t.Fatalf("giá trị no_view khác \"1\" phải đếm như thường: %v", nd.views)
	}
}

// Redis down: the read is served (fail-open), but nothing bounds the caller, so nothing is counted.
func TestPublicDetailNotCountedWhenLimiterNotEnforced(t *testing.T) {
	nd, _ := ckDuLieu()
	h := viewServer(t, nd, &memCounter{fail: errors.New("dial tcp: connection refused")}, nil)

	w := ckGoi(h, MauTinXa+"/nd-a-1", ckHostA)
	doiMa(t, w, http.StatusOK)
	if len(nd.views) != 0 {
		t.Fatalf("giới hạn tần suất không kiểm được mà vẫn đếm: %v", nd.views)
	}
	if got := viewCountOf(t, w); got != 9 {
		t.Fatalf("view_count = %v, muốn 9", got)
	}
}

// Best effort: the increment fails → 200 with the article and the count as read; the log names the item
// id and never the host or the client address.
func TestPublicDetailIncrementFailureStillServes(t *testing.T) {
	nd, _ := ckDuLieu()
	nd.viewErr = errors.New("pq: connection reset")
	var buf bytes.Buffer
	h := viewServer(t, nd, nil, slog.New(slog.NewJSONHandler(&buf, nil)))

	r := httptest.NewRequest(http.MethodGet, "https://comms.api.vigov.vn"+MauTinXa+"/nd-a-1?host="+ckHostA, nil)
	r.RemoteAddr = "203.0.113.7:5000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	doiMa(t, w, http.StatusOK)
	if got := viewCountOf(t, w); got != 9 {
		t.Fatalf("view_count = %v, muốn 9 (giá trị đã đọc)", got)
	}
	var ra map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &ra)
	if ra["id"] != "nd-a-1" || ra["body"] == nil {
		t.Fatalf("tin không được trả đầy đủ: %v", ra)
	}
	logged := buf.String()
	if !strings.Contains(logged, "không tăng được lượt xem") || !strings.Contains(logged, `"id":"nd-a-1"`) {
		t.Fatalf("lỗi tăng lượt xem không được ghi với mã tin: %s", logged)
	}
	for _, cam := range []string{ckHostA, "203.0.113.7", "Tiêm chủng"} {
		if strings.Contains(logged, cam) {
			t.Fatalf("nhật ký chứa %q: %s", cam, logged)
		}
	}
}

// The list, the banner strip and the categories never count; the list carries the stored count.
func TestPublicListNeverCounts(t *testing.T) {
	nd, _ := ckDuLieu()
	h := viewServer(t, nd, nil, nil)

	w := ckGoi(h, MauTinXa, ckHostA)
	doiMa(t, w, http.StatusOK)
	if it := ckDocTrang(t, w).Items; len(it) != 1 || it[0]["view_count"] != float64(9) {
		t.Fatalf("trang = %v, muốn một mục view_count 9", it)
	}
	ckGoi(h, MauTinXa, ckHostA, "&type=banner")
	ckGoi(h, MauTinXa+"/categories", ckHostA)
	if len(nd.views) != 0 {
		t.Fatalf("tuyến danh sách đã đếm: %v", nd.views)
	}
}

// Rule 1: the increment runs in the commune the HOST resolved to — B's article is counted only under
// B's host, and never under A's.
func TestPublicDetailCountsInTheResolvedCommuneOnly(t *testing.T) {
	nd, _ := ckDuLieu()
	h := viewServer(t, nd, nil, nil)

	doiMa(t, ckGoi(h, MauTinXa+"/nd-b-1", ckHostA), http.StatusNotFound)
	doiMa(t, ckGoi(h, MauTinXa+"/nd-b-1", ckHostB), http.StatusOK)
	if strings.Join(nd.views, ",") != string(xaB)+"/nd-b-1" {
		t.Fatalf("lượt tăng = %v, muốn đúng [xã B/nd-b-1]", nd.views)
	}
	if nd.theoXa[xaA][0].LuotXem != 9 {
		t.Fatalf("tin của xã A bị đổi lượt xem: %d", nd.theoXa[xaA][0].LuotXem)
	}
}
