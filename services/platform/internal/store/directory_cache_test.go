package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/pkg/tenant"
)

// dirGia is a stub directory that counts how often it is actually reached.
type dirGia struct {
	mu     sync.Mutex
	soLan  int
	ketQua map[string]tenant.Tenant
}

func (d *dirGia) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.soLan++
	t, ok := d.ketQua[host]
	return t, ok
}

const ulidThangBinh = "01JD8ZQK9M3NPXR7TVWYB2C4EF"

func dirMau() *dirGia {
	return &dirGia{ketQua: map[string]tenant.Tenant{
		"thangbinh.vigov.vn": {
			ID: tenant.ID(ulidThangBinh), Host: "thangbinh.vigov.vn",
			Name: "Xã Thăng Bình", Active: true,
		},
	}}
}

func TestCacheTraLoiTuBoNho(t *testing.T) {
	// This lookup runs on every request of every commune. If it reached the database each
	// time it would be the busiest query in the system, answering the same question all day.
	inner := dirMau()
	c := NewCachedDirectory(inner, time.Minute)

	for i := range 5 {
		got, ok := c.ByHost(context.Background(), "thangbinh.vigov.vn")
		if !ok || got.ID != tenant.ID(ulidThangBinh) {
			t.Fatalf("lần %d: ByHost trả %v, ok=%v", i, got.ID, ok)
		}
	}
	if inner.soLan != 1 {
		t.Errorf("chạm tầng dưới %d lần, muốn 1", inner.soLan)
	}
}

func TestCacheNhoCaLanTruot(t *testing.T) {
	// An unknown Host is the shape of a scan. If misses are not remembered, every probe
	// reaches the database and a scanner sets the query rate.
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
	// A deactivated commune may hold several hosts. Serving a dissolved commune is worse than
	// a moment of extra queries.
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
	// ttl <= 0 disables caching, which is what tests exercising the real query want.
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
