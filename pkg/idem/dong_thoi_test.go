package idem

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

// Duplicate protection is only worth anything under CONCURRENCY. The existing tests send the
// second request after the first has finished, which is the easy case and not the one that
// happens in the field: a citizen double-taps, a Mini App retries on a slow network, a clerk
// clicks twice. Both requests are then in flight at the same moment.
//
// THE DEFECT CLASS: the handler running twice for one key. Rule 7 forbids deleting the
// duplicate, so a second petition with a second lookup code — already shown to the citizen —
// is permanent, and a second disbursement is real money. Run with -race.

func TestHaiYeuCauCungKhoaCungLucChiChayHandlerMotLan(t *testing.T) {
	var chay int32
	var batDau sync.WaitGroup
	batDau.Add(1)

	h := Required(DongKhiHong)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&chay, 1)
		batDau.Wait() // both requests are inside the middleware before either finishes
		RecordCode(r.Context(), "PA-7F3K9Q")
		w.WriteHeader(http.StatusCreated)
	}))
	s := newStoreGia()
	p := canBo(xaA, canBo1)

	ma := make([]int, 2)
	phatLaiLai := make([]string, 2)
	var wg sync.WaitGroup
	for i := range ma {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := goiNhu(t, h, s, xaA, khoaKhachHang, p)
			ma[i] = w.Code
			phatLaiLai[i] = w.Header().Get(HeaderPhatLai)
		}(i)
	}
	// Let the handler(s) proceed once both goroutines have had a chance to reach the middleware.
	batDau.Done()
	wg.Wait()

	if n := atomic.LoadInt32(&chay); n != 1 {
		t.Fatalf("handler chạy %d lần cho một khoá — bản ghi trùng là vĩnh viễn (luật 7)", n)
	}
	// Exactly one request is served as the first. The other is either told the first is still in
	// progress (409) or replayed — and a replay MUST say so in the header, otherwise the client
	// cannot tell a second creation from a repeat of the first.
	var lanDau int
	for i, m := range ma {
		switch {
		case m == http.StatusCreated && phatLaiLai[i] == "":
			lanDau++
		case m == http.StatusCreated && phatLaiLai[i] == "true": // replayed
		case m == http.StatusConflict:
		default:
			t.Fatalf("phản hồi lạ: mã %d, %s=%q", m, HeaderPhatLai, phatLaiLai[i])
		}
	}
	if lanDau != 1 {
		t.Fatalf("%d yêu cầu được phục vụ như lần đầu, muốn 1: mã %v, phát lại %v",
			lanDau, ma, phatLaiLai)
	}
	if s.soKhoa() != 1 {
		t.Fatalf("có %d khoá trong kho, muốn 1", s.soKhoa())
	}
}

func TestHaiXaCungLucCungKhoaKhachHangVanChayCaHai(t *testing.T) {
	// The commune prefix must hold under concurrency too: two communes whose clients generate the
	// same key are two unrelated requests, and refusing one of them would refuse a citizen of
	// commune B because somebody in commune A acted at the same second.
	var chay int32
	h := Required(MoKhiHong)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&chay, 1)
		RecordCode(r.Context(), "PA-7F3K9Q")
		w.WriteHeader(http.StatusCreated)
	}))
	s := newStoreGia()

	var wg sync.WaitGroup
	ma := map[string]int{}
	var mu sync.Mutex
	for _, xa := range []string{xaA, xaB} {
		wg.Add(1)
		go func(xa string) {
			defer wg.Done()
			w := goiNhu(t, h, s, xa, khoaKhachHang, canBo(xa, canBo1))
			mu.Lock()
			ma[xa] = w.Code
			mu.Unlock()
		}(xa)
	}
	wg.Wait()

	if atomic.LoadInt32(&chay) != 2 {
		t.Fatalf("handler chạy %d lần, muốn 2 — hai xã không được đụng khoá của nhau", chay)
	}
	for xa, m := range ma {
		if m != http.StatusCreated {
			t.Errorf("xã %s nhận mã %d, muốn 201", xa, m)
		}
	}
	if s.soKhoa() != 2 {
		t.Errorf("có %d khoá, muốn 2 (mỗi xã một khoá riêng)", s.soKhoa())
	}
}

func TestNhieuCanBoCungXaGuiDongThoiKhongDungKetQuaCuaNhau(t *testing.T) {
	// Inside ONE commune the key is still client-supplied, so two clerks can present the same
	// one at the same moment. The actor component of the key is what keeps each of them on their
	// own result — otherwise the second clerk is handed a lookup code for a petition they did not
	// create, and believes theirs exists. It does not.
	var mu sync.Mutex
	demTheoChuThe := map[string]int{}

	h := Required(DongKhiHong)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		demTheoChuThe[ChuThe(r.Context())]++
		mu.Unlock()
		RecordCode(r.Context(), "PA-7F3K9Q")
		w.WriteHeader(http.StatusCreated)
	}))
	s := newStoreGia()

	var wg sync.WaitGroup
	for _, id := range []string{canBo1, canBo2} {
		for lan := 0; lan < 5; lan++ {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				goiNhu(t, h, s, xaA, khoaKhachHang, canBo(xaA, id))
			}(id)
		}
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(demTheoChuThe) != 2 {
		t.Fatalf("có %d chủ thể chạy handler, muốn 2: %v", len(demTheoChuThe), demTheoChuThe)
	}
	for chuThe, n := range demTheoChuThe {
		if n != 1 {
			t.Errorf("chủ thể %s chạy handler %d lần, muốn 1", chuThe, n)
		}
	}
}
