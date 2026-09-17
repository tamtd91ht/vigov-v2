package tenant

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Moved here with CachedDirectory itself, from platform/internal/store: the cache is a
// decorator over Directory that every service edge needs, and a test that moved with it is a
// test that keeps being run.

// dirGia is a stub directory that counts how often it is actually reached.
type dirGia struct {
	mu     sync.Mutex
	soLan  int
	ketQua map[string]Tenant
}

func (d *dirGia) ByHost(_ context.Context, host string) (Tenant, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.soLan++
	t, ok := d.ketQua[host]
	return t, ok
}

const ulidThangBinh = "01JD8ZQK9M3NPXR7TVWYB2C4EF"

func dirMau() *dirGia {
	return &dirGia{ketQua: map[string]Tenant{
		"thangbinh.vigov.vn": {
			ID: ID(ulidThangBinh), Host: "thangbinh.vigov.vn",
			Name: "Xã Thăng Bình", Active: true,
		},
	}}
}

func TestCacheTraLoiTuBoNho(t *testing.T) {
	// This lookup runs on every request of every commune. If it reached the registry each time
	// it would be the busiest call in the system, answering the same question all day.
	inner := dirMau()
	c := NewCachedDirectory(inner, time.Minute)

	for i := range 5 {
		got, ok := c.ByHost(context.Background(), "thangbinh.vigov.vn")
		if !ok || got.ID != ID(ulidThangBinh) {
			t.Fatalf("lần %d: ByHost trả %v, ok=%v", i, got.ID, ok)
		}
	}
	if inner.soLan != 1 {
		t.Errorf("chạm tầng dưới %d lần, muốn 1", inner.soLan)
	}
}

func TestCacheNhoCaLanTruot(t *testing.T) {
	// An unknown Host is the shape of a scan. If misses are not remembered, every probe reaches
	// the registry and a scanner sets the query rate.
	inner := dirMau()
	c := NewCachedDirectory(inner, time.Minute)

	for range 4 {
		if _, ok := c.ByHost(context.Background(), "khong-ton-tai.vigov.vn"); ok {
			t.Fatal("host không tồn tại mà trả ok=true")
		}
	}
	if inner.soLan != 1 {
		t.Errorf("chạm tầng dưới %d lần cho host không tồn tại, muốn 1", inner.soLan)
	}
}

func TestCacheHetHanThiHoiLai(t *testing.T) {
	// The TTL bounds how long a deactivated commune keeps being served.
	inner := dirMau()
	c := NewCachedDirectory(inner, time.Minute)

	gio := time.Now()
	c.now = func() time.Time { return gio }

	c.ByHost(context.Background(), "thangbinh.vigov.vn")
	gio = gio.Add(2 * time.Minute) // quá hạn
	c.ByHost(context.Background(), "thangbinh.vigov.vn")

	if inner.soLan != 2 {
		t.Errorf("chạm tầng dưới %d lần, muốn 2 (một trước và một sau khi hết hạn)", inner.soLan)
	}
}

func TestCacheForgetDongCuaSoNgay(t *testing.T) {
	// The TTL is a safety net; Forget is the mechanism when a domain is reassigned.
	inner := dirMau()
	c := NewCachedDirectory(inner, time.Hour)

	c.ByHost(context.Background(), "thangbinh.vigov.vn")
	c.Forget("thangbinh.vigov.vn")
	c.ByHost(context.Background(), "thangbinh.vigov.vn")

	if inner.soLan != 2 {
		t.Errorf("chạm tầng dưới %d lần, muốn 2 — Forget phải buộc hỏi lại ngay", inner.soLan)
	}
}

func TestCacheForgetAll(t *testing.T) {
	// A deactivated commune may hold several hosts. Serving a dissolved commune is worse than a
	// moment of extra queries.
	inner := dirMau()
	inner.ketQua["cu.vigov.vn"] = inner.ketQua["thangbinh.vigov.vn"]
	c := NewCachedDirectory(inner, time.Hour)

	c.ByHost(context.Background(), "thangbinh.vigov.vn")
	c.ByHost(context.Background(), "cu.vigov.vn")
	c.ForgetAll()
	c.ByHost(context.Background(), "thangbinh.vigov.vn")
	c.ByHost(context.Background(), "cu.vigov.vn")

	if inner.soLan != 4 {
		t.Errorf("chạm tầng dưới %d lần, muốn 4 — ForgetAll phải xoá mọi host", inner.soLan)
	}
}

func TestCacheTtlKhongThiKhongNho(t *testing.T) {
	// ttl <= 0 disables caching, which is what tests exercising the real lookup want.
	inner := dirMau()
	c := NewCachedDirectory(inner, 0)

	c.ByHost(context.Background(), "thangbinh.vigov.vn")
	c.ByHost(context.Background(), "thangbinh.vigov.vn")

	if inner.soLan != 2 {
		t.Errorf("chạm tầng dưới %d lần, muốn 2 — ttl=0 phải tắt cache", inner.soLan)
	}
}

func TestCacheAnToanKhiChayDongThoi(t *testing.T) {
	// The edge calls this concurrently on every request. A data race here is a crash in
	// production under exactly the load that matters.
	inner := dirMau()
	c := NewCachedDirectory(inner, time.Minute)

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			c.ByHost(context.Background(), "thangbinh.vigov.vn")
		})
	}
	wg.Wait()
}

// --- trần số mục ------------------------------------------------------------------------------

// BÀI QUAN TRỌNG NHẤT TỆP NÀY. `TenantMiddleware` tra thư mục cho MỌI `Host`, trước mọi xác
// thực, và khoá của map là chuỗi kẻ gọi đặt. Không trần thì `curl -H 'Host: <ngẫu nhiên>'` lặp
// lại là một mục thường trú mỗi lần — không token, không cần biết xã nào tồn tại — và tiến
// trình hết bộ nhớ là 200+ xã cùng chết.
func TestCacheKhongPhinhVoHanKhiBiQuetHost(t *testing.T) {
	d := dirMau()
	c := NewCachedDirectory(d, time.Minute)

	for i := 0; i < TranMuc+500; i++ {
		c.ByHost(context.Background(), fmt.Sprintf("rac-%d.example.gov.vn", i))
	}

	c.mu.RLock()
	n := len(c.entries)
	c.mu.RUnlock()

	if n > TranMuc {
		t.Fatalf("bộ đệm giữ %d mục, vượt trần %d — một kẻ quét đặt được nhịp cấp phát bộ nhớ", n, TranMuc)
	}
}

// Khi đầy, bộ đệm TỪ CHỐI nhớ host mới thay vì đuổi một mục đang còn hạn.
//
// Đuổi để lấy chỗ sẽ đổi một đòn bộ nhớ thành một đòn thrash: mỗi host rác đẩy một XÃ CÓ THẬT
// ra, mọi yêu cầu thật quay lại gọi sang dịch vụ nền tảng, và thứ vừa được bảo vệ lại là thứ
// chịu thiệt.
//
// BÀI NÀY TẤT ĐỊNH, và bản đầu của nó thì không: nó khẳng định một khoá cụ thể sống sót qua
// ~500 lần đuổi ngẫu nhiên, tức đúng khoảng 88% số lần chạy. Một bài test chập chờn tệ hơn
// không có bài nào — nó xanh ở máy mình rồi đỏ trong CI vào một buổi chẳng liên quan. Nên chỗ
// cần ghim là CHÍNH PHÉP TỪ CHỐI: điền đầy đúng trần bằng mục còn hạn, ghi thêm một host, rồi
// đòi rằng host ấy KHÔNG được nhớ.
func TestCacheDayThiTuChoiNhoThemChuKhongDuoiMucConHan(t *testing.T) {
	d := dirMau()
	c := NewCachedDirectory(d, time.Minute)
	moc := time.Now()
	c.now = func() time.Time { return moc }

	for i := 0; i < TranMuc; i++ {
		c.ByHost(context.Background(), fmt.Sprintf("rac-%d.example.gov.vn", i))
	}

	const moi = "host-moi.example.gov.vn"
	c.ByHost(context.Background(), moi)
	truoc := d.soLan

	// Hỏi lại ngay: nếu nó ĐÃ được nhớ thì thư mục không bị hỏi thêm. Phải bị hỏi lại.
	c.ByHost(context.Background(), moi)
	if d.soLan == truoc {
		t.Fatal("bộ đệm đầy vẫn nhớ host mới — tức nó đã đuổi một mục còn hạn để lấy chỗ")
	}

	c.mu.RLock()
	n := len(c.entries)
	c.mu.RUnlock()
	if n != TranMuc {
		t.Errorf("số mục = %d, muốn đúng %d — không mục còn hạn nào được phép bị đuổi", n, TranMuc)
	}
}

// Trần không được biến bộ đệm thành thứ chỉ điền một lần rồi đóng băng: mục hết hạn phải được
// dọn, nếu không một lượt quét sẽ khoá vĩnh viễn mọi xã onboard sau đó ra ngoài bộ đệm.
func TestCacheDayRoiVanNhoDuocHostMoiSauKhiHetHan(t *testing.T) {
	d := dirMau()
	c := NewCachedDirectory(d, time.Minute)
	moc := time.Now()
	c.now = func() time.Time { return moc }

	for i := 0; i < TranMuc+10; i++ {
		c.ByHost(context.Background(), fmt.Sprintf("rac-%d.example.gov.vn", i))
	}

	// Toàn bộ mục rác hết hạn.
	moc = moc.Add(2 * time.Minute)

	c.ByHost(context.Background(), "thangbinh.vigov.vn")
	truoc := d.soLan
	if _, ok := c.ByHost(context.Background(), "thangbinh.vigov.vn"); !ok {
		t.Fatal("không phân giải được xã thật")
	}
	if d.soLan != truoc {
		t.Error("xã mới KHÔNG được nhớ dù các mục cũ đã hết hạn — bộ đệm bị đóng băng sau một lượt quét")
	}
}
