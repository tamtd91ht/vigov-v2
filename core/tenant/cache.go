package tenant

import (
	"context"
	"sync"
	"time"
)

// CachedDirectory wraps a Directory with a short TTL.
//
// WHY THIS IS NOT PREMATURE: this lookup runs on EVERY request of EVERY commune. At 200+
// communes an uncached directory turns one round trip into the busiest call in the system, and
// it answers the same question all day — the Host -> commune mapping changes only when a
// commune is renamed or a domain is reassigned (ADR 0004, decision 5). For the seven services
// that are NOT the platform, that round trip is a gRPC call to another process, so the cost of
// missing is higher still.
//
// WHY THE TTL IS SHORT AND NOT ZERO: this cache is keyed by HOST, not by commune, so it holds
// no commune's business data and cannot serve one commune's rows to another — the failure mode
// rule 1 forbids. What it can do is keep serving a commune that was just deactivated, or miss a
// domain that was just reassigned. A short TTL bounds that window; Forget closes it on the
// events that matter.
//
// WHY IT LIVES IN pkg/ AND NOT IN THE PLATFORM SERVICE, which is where it used to live: it is a
// decorator over tenant.Directory with no platform logic in it at all, and every service edge
// needs one — the platform reads the registry from its own tables, the other seven read it over
// gRPC, and both wrap the result here. Left inside platform/internal it was unreachable
// from anywhere else (rule 2, forbidden #1), so the second service to need it would have written
// a second copy, and two caches with two invalidation rules is how a deactivated commune keeps
// being served in one process after it stopped being served in another.
//
// THE ONE THING THIS CACHE MUST NEVER BECOME: a cache keyed without the commune holding business
// data. That is rule 1, invariant 7, and it is a different thing from this. What makes THIS
// one safe is that the key IS the host and the value is only the registry answer.
type CachedDirectory struct {
	inner Directory
	ttl   time.Duration
	now   func() time.Time // injectable so tests do not sleep

	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	t      Tenant
	ok     bool // a miss is remembered too — see NewCachedDirectory
	hetHan time.Time
}

// NewCachedDirectory wraps inner. A ttl of zero disables caching, which is what tests that
// exercise the underlying lookup should use.
//
// MISSES ARE REMEMBERED ON PURPOSE: an unknown Host is the shape of a scan looking for communes
// that exist. Forgetting misses means every probe reaches the registry, and a scanner sets the
// query rate. The negative entry is cheap and bounded by the same TTL.
//
// THE COST OF THAT CHOICE, STATED: a lookup that failed for a TRANSPORT reason is indistinguish-
// able here from "no such commune" — Directory.ByHost returns a bool and nothing else — so a
// blip while the registry is unreachable is remembered as a miss for up to one TTL. That is
// fail-closed, which is the right direction (rule 1, invariant 3), and the implementation of
// ByHost is where the alarm has to be raised so the 404s are not silent. Telling the two apart
// inside the cache needs a Directory that reports why, and that is a wider change than this.
func NewCachedDirectory(inner Directory, ttl time.Duration) *CachedDirectory {
	return &CachedDirectory{
		inner:   inner,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]cacheEntry),
	}
}

func (c *CachedDirectory) ByHost(ctx context.Context, host string) (Tenant, bool) {
	if c.ttl <= 0 {
		return c.inner.ByHost(ctx, host)
	}

	c.mu.RLock()
	e, found := c.entries[host]
	c.mu.RUnlock()
	if found && c.now().Before(e.hetHan) {
		return e.t, e.ok
	}

	t, ok := c.inner.ByHost(ctx, host)

	c.mu.Lock()
	c.ghiCoTran(host, cacheEntry{t: t, ok: ok, hetHan: c.now().Add(c.ttl)})
	c.mu.Unlock()
	return t, ok
}

// TranMuc bounds the map. Reached only by something that is not ordinary traffic.
//
// VÌ SAO PHẢI CÓ TRẦN, dù chú thích trên NewCachedDirectory lập luận đúng rằng nhớ cả miss là
// một phòng thủ: `TenantMiddleware` tra thư mục cho MỌI `Host`, trước mọi xác thực, và khoá
// của map này chính là chuỗi kẻ gọi đặt. Không trần thì `curl -H 'Host: <ngẫu nhiên>'` lặp lại
// là một mục thường trú mỗi lần — không token, không cần biết xã nào tồn tại. Kẻ quét thôi đặt
// nhịp TRUY VẤN và bắt đầu đặt nhịp CẤP PHÁT BỘ NHỚ, và tiến trình hết bộ nhớ là 200+ xã cùng
// chết. Đây là rò khả dụng, không phải rò dữ liệu — nhưng nó hạ cả hệ thống.
//
// 4096 rộng gấp nhiều lần nhu cầu thật: mỗi xã có thể giữ vài tên miền cùng lúc (xã sáp nhập
// phải để địa chỉ cũ còn phân giải — luật 7), nên 200+ xã vẫn nằm sâu dưới ngưỡng. Con số này
// không ước lượng lưu lượng thật; nó là điểm mà nội dung map đã thôi là danh bạ tên miền.
const TranMuc = 4096

// ghiCoTran ghi một mục, và KHÔNG BAO GIỜ đẩy một mục còn hạn ra để lấy chỗ.
//
// Gọi khi đã giữ c.mu.
//
// THỨ TỰ Ở ĐÂY LÀ TOÀN BỘ NỘI DUNG: dọn các mục đã hết hạn trước, và chỉ khi vẫn đầy thì TỪ
// CHỐI NHỚ mục mới — phục vụ câu trả lời bình thường, chỉ là không cất nó.
//
// Vì sao không đuổi theo LRU hay ngẫu nhiên: đuổi một mục còn hạn nghĩa là đẩy một XÃ CÓ THẬT
// ra khỏi bộ đệm để lấy chỗ cho một host rác. Khi ấy kẻ tấn công đổi được một đòn bộ nhớ thành
// một đòn thrash — mọi yêu cầu thật quay lại gọi sang dịch vụ nền tảng, và thứ vừa được bảo vệ
// lại là thứ chịu thiệt. Từ chối nhớ cái mới giữ các mục thật luôn nóng.
//
// CÁI GIÁ, NÓI THẲNG: khi đã chạm trần, việc nhớ miss thôi tác dụng, nên kẻ quét lại đặt được
// nhịp truy vấn — đúng thứ chú thích trên NewCachedDirectory muốn chặn. Đánh đổi có chủ ý: mất
// một phòng thủ mềm (nhịp truy vấn, suy giảm dần) để giữ một ràng buộc cứng (bộ nhớ có biên,
// hỏng là chết cả tiến trình).
func (c *CachedDirectory) ghiCoTran(host string, e cacheEntry) {
	if len(c.entries) >= TranMuc {
		bayGio := c.now()
		for k, v := range c.entries {
			if !bayGio.Before(v.hetHan) {
				delete(c.entries, k)
			}
		}
	}
	// KHÔNG có nhánh riêng cho "mục này đã có sẵn". Bản nháp đầu có một nhánh như thế, với lý
	// do nghe hợp lý là ghi đè không làm map to thêm — nhưng nó là MÃ CHẾT: tới được đây nghĩa
	// là mục cũ hoặc không tồn tại, hoặc đã hết hạn (mục còn hạn đã trả về từ bộ đệm ở trên và
	// không bao giờ ghi lại), mà mục hết hạn thì vòng dọn ngay trên vừa xoá. Một đột biến gỡ
	// nhánh ấy đi không làm đỏ bài test nào — đó là cách nó lộ ra.
	if len(c.entries) < TranMuc {
		c.entries[host] = e
	}
}

// Forget drops one host from the cache. Call it when a domain is reassigned or a commune is
// renamed — the TTL is a safety net, not the mechanism.
//
// This touches an in-memory map only. Nothing here reaches storage, and no commune's record is
// affected: rule 7 is about archival data, and a cache holds none.
func (c *CachedDirectory) Forget(host string) {
	c.mu.Lock()
	delete(c.entries, host)
	c.mu.Unlock()
}

// ForgetAll empties the cache. Call it when a commune is deactivated: that commune may hold
// several hosts, and serving a dissolved commune is worse than a moment of extra queries.
func (c *CachedDirectory) ForgetAll() {
	c.mu.Lock()
	c.entries = make(map[string]cacheEntry)
	c.mu.Unlock()
}

// CommuneLookup answers "what does the registry say about THE COMMUNE IN ctx" — the by-commune twin
// of Directory. The commune is read from ctx, never passed as an argument (rule 1, invariant 4): the
// caller puts the commune it is asking about into the context, exactly as the citizen-session bridge
// does before platformclient.XaTrongNguCanh.
//
// THREE OUTCOMES, NOT TWO, AND THAT IS WHY IT IS NOT A METHOD ON Directory. ByHost collapses an
// outage into "no such host" and the edge answers 404, which is fail-closed and fine for a host. The
// caller of THIS lookup is the citizen-session check, where the same collapse would tell every
// citizen of every commune "your session ended" during a registry blip. err != nil is "could not
// ask"; ok=false is "the registry does not know this commune"; ok=true carries Active as stored.
type CommuneLookup interface {
	Current(ctx context.Context) (Tenant, bool, error)
}

// CachedCommune is CachedDirectory keyed by commune instead of host, with the SAME TTL
// (TENANT_CACHE_TTL) — so "a deactivated commune stops being served within one TTL" holds for every
// path that asks by commune, not only for Host resolution.
//
// ONLY ANSWERS ARE CACHED, NEVER ERRORS. A cached error would refuse a commune for a whole TTL after
// the registry recovered; an uncached one costs a retry on the next request, which is what an outage
// already costs. Negative answers ARE cached: the commune here comes from a server-issued session,
// never from a client, so "unknown" is a dissolved commune and asking again cannot change the answer
// sooner than the TTL.
//
// SAFE UNDER RULE 1 FOR THE SAME REASON CachedDirectory IS: the key is the commune and the value is
// only the registry's metadata answer — no business data of any commune is held.
type CachedCommune struct {
	inner CommuneLookup
	ttl   time.Duration
	now   func() time.Time // injectable so tests do not sleep

	mu      sync.RWMutex
	entries map[ID]cacheEntry
}

// NewCachedCommune wraps inner. A ttl of zero disables caching.
func NewCachedCommune(inner CommuneLookup, ttl time.Duration) *CachedCommune {
	if inner == nil {
		// A nil lookup would panic on the first citizen request, in another service's hot path.
		panic("tenant: NewCachedCommune cần một CommuneLookup")
	}
	return &CachedCommune{inner: inner, ttl: ttl, now: time.Now, entries: make(map[ID]cacheEntry)}
}

// Current implements CommuneLookup. No commune in ctx is a wiring fault and an error — never a
// lookup of "" and never a default (rule 1, forbidden #1).
func (c *CachedCommune) Current(ctx context.Context) (Tenant, bool, error) {
	id, ok := From(ctx)
	if !ok {
		return Tenant{}, false, ErrNoTenant
	}
	if c.ttl <= 0 {
		return c.inner.Current(ctx)
	}

	c.mu.RLock()
	e, found := c.entries[id]
	c.mu.RUnlock()
	if found && c.now().Before(e.hetHan) {
		return e.t, e.ok, nil
	}

	t, ok, err := c.inner.Current(ctx)
	if err != nil {
		return Tenant{}, false, err
	}

	c.mu.Lock()
	// The same ceiling as CachedDirectory, for a different reason: the keys come from server-issued
	// sessions, so a flood is not expected — but an unbounded map on a hot path is one bug away from
	// an out-of-memory that takes every commune down. Expired entries go first; when still full, the
	// answer is served without being remembered — a live commune is never evicted.
	if len(c.entries) >= TranMuc {
		bayGio := c.now()
		for k, v := range c.entries {
			if !bayGio.Before(v.hetHan) {
				delete(c.entries, k)
			}
		}
	}
	if len(c.entries) < TranMuc {
		c.entries[id] = cacheEntry{t: t, ok: ok, hetHan: c.now().Add(c.ttl)}
	}
	c.mu.Unlock()
	return t, ok, nil
}
