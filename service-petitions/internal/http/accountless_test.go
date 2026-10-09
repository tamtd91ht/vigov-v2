package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// THE ACCOUNTLESS ROUTES — ADR 0083 (TEMPORARY), rule 5 invariant 7 adapted to authz.Public routes:
// there is no token to lack (so "no token" must WORK), no permission (so no 403), and the commune axis is
// the domain — wrong domain 404, another commune's code 404, an owned petition's code 404, limits 429.
//
//	PROVED HERE   no Authorization is needed on any of the three · the 201 and the lookup carry exactly
//	              their keys and no personal data · a retry with the same key replays the same shape and
//	              sends nothing twice · a malformed, unknown or inactive domain writes nothing · refused
//	              body fields (owner, channel, lat/lng) are 400 before anything is resolved · the lookup's
//	              404 is byte-identical for unknown domain, wrong code, another commune's code and an
//	              owned petition · 6th send / 31st lookup / limiter down = 429 / 429 / 503 · the commune
//	              ceiling and the unconfigured commune map to 429 commune_daily_limit / 503.
//
//	NOT PROVED    the SQL restricting the read to accountless rows (store/accountless_test.go), the
//	              ceiling count (app/accountless_intake_test.go).

var (
	acctXaA = tenant.ID("01JA" + strings.Repeat("A", 22))
	acctXaB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

const (
	acctHostA        = "xa-a.example.gov.vn"
	acctHostB        = "xa-b.example.gov.vn"
	acctHostInactive = "xa-cu.example.gov.vn"
	acctKey          = "9f86d081884c7d659a2feaa0c55ad015"
	acctCodeA        = "PA-ACCT-AAAA-0001"
	acctCodeB        = "PA-ACCT-BBBB-0001"
	acctOwnedA       = "PA-OWNED-AAA-0001" // a citizen's petition in commune A
)

type acctDirectory struct{ down bool }

// vi-name-ok: implements tenant.HostResolver, whose method name is platformclient's
func (d acctDirectory) XaTheoHost(_ context.Context, host string) (tenant.Tenant, bool, error) {
	if d.down {
		return tenant.Tenant{}, false, errors.New("platform unreachable")
	}
	switch host {
	case acctHostA:
		return tenant.Tenant{ID: acctXaA, Host: host, Active: true}, true, nil
	case acctHostB:
		return tenant.Tenant{ID: acctXaB, Host: host, Active: true}, true, nil
	case acctHostInactive:
		return tenant.Tenant{ID: tenant.ID("01JC" + strings.Repeat("C", 22)), Host: host, Active: false}, true, nil
	}
	return tenant.Tenant{}, false, nil
}

// acctIntake records what it was asked; err, when set, is returned instead.
type acctIntake struct {
	mu    sync.Mutex
	calls int
	got   []app.IntakeSender
	xa    []tenant.ID
	err   error
}

func (a *acctIntake) Gui(ctx context.Context, yc app.YeuCauGuiPhanAnh, s app.IntakeSender) (domain.PhieuPhanAnh, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	a.got = append(a.got, s)
	a.xa = append(a.xa, tenant.MustFrom(ctx))
	if a.err != nil {
		return domain.PhieuPhanAnh{}, a.err
	}
	return acctPetition(acctCodeA), nil
}

func acctPetition(code string) domain.PhieuPhanAnh {
	return domain.PhieuPhanAnh{MaTraCuu: code, Kenh: domain.KenhZaloMiniApp, TrangThai: domain.DaTiepNhan,
		NoiDung: "Đống rác ở đầu ngõ.", DiaChi: "Thôn Hà Lam", NguoiGuiHoTen: "Nguyễn Văn An",
		NguoiGuiDienThoai: "0900000000", HanTiepNhan: time.Date(2026, 10, 9, 2, 30, 0, 0, time.UTC)}
}

// acctReader holds accountless petitions per commune — like the store, an owned petition is NOT in it.
type acctReader struct{}

func (acctReader) AccountlessByCode(ctx context.Context, ma string) (domain.PhieuPhanAnh, error) {
	switch {
	case tenant.MustFrom(ctx) == acctXaA && ma == acctCodeA:
		return acctPetition(acctCodeA), nil
	case tenant.MustFrom(ctx) == acctXaB && ma == acctCodeB:
		return acctPetition(acctCodeB), nil
	}
	return domain.PhieuPhanAnh{}, petstore.ErrPhieuKhongTonTai
}

type acctFields struct{}

func (acctFields) CitizenCatalogue(ctx context.Context) ([]domain.PetitionFieldView, error) {
	_ = tenant.MustFrom(ctx)
	return []domain.PetitionFieldView{{Code: "rac-thai", Label: "Rác thải", Active: true, Enabled: true}}, nil
}

// acctCounter is a fixed-window counter per key, or always failing when down.
type acctCounter struct {
	mu   sync.Mutex
	n    map[string]int64
	down bool
}

func (c *acctCounter) Incr(_ context.Context, key string, _ time.Duration) (int64, time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.down {
		return 0, 0, errors.New("redis down")
	}
	if c.n == nil {
		c.n = map[string]int64{}
	}
	c.n[key]++
	return c.n[key], time.Minute, nil
}

// acctStore is an in-memory idem.Store.
type acctStore struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *acctStore) Claim(_ context.Context, k string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[k]; ok {
		return false, nil
	}
	s.m[k] = "1"
	return true, nil
}
func (s *acctStore) Get(_ context.Context, k string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[k], nil
}
func (s *acctStore) Complete(_ context.Context, k, v string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}
func (s *acctStore) Release(_ context.Context, k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
	return nil
}

type acctRig struct {
	h       http.Handler
	intake  *acctIntake
	counter *acctCounter
}

func newAcctRig(t *testing.T, dir acctDirectory) *acctRig {
	t.Helper()
	rig := &acctRig{intake: &acctIntake{}, counter: &acctCounter{}}
	lim := func(p ratelimit.Policy) *ratelimit.Limiter {
		l, err := ratelimit.New(rig.counter, p)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterAccountless(mux, DepsAccountless{
		Communes: dir, Intake: rig.intake, Petitions: acctReader{}, Fields: acctFields{},
		SendLimiter: lim(ratelimit.AccountlessSend), LookupLimiter: lim(ratelimit.AccountlessLookup),
		FieldsLimiter: lim(ratelimit.AccountlessFieldRead), Log: log,
	})
	rig.h = idem.Middleware(&acctStore{m: map[string]string{}}, log)(mux)
	return rig
}

// do sends a request with NO Authorization and NO cookie — the public chain's only shape.
func (r *acctRig) do(method, path, body, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "10.0.0.9:51000"
	if key != "" {
		req.Header.Set(idem.Header, key)
	}
	w := httptest.NewRecorder()
	r.h.ServeHTTP(w, req)
	return w
}

func acctBody(host string) string {
	return `{"host":"` + host + `","content":"Đống rác ở đầu ngõ.","address":"Thôn Hà Lam",` +
		`"reporter_name":"Nguyễn Văn An","reporter_phone":"0900000000","anonymous":false}`
}

func keysOf(t *testing.T, raw []byte) []string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not JSON: %s", raw)
	}
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var e struct{ Code string }
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	return e.Code
}

// --- send -------------------------------------------------------------------------------------

func TestAccountlessSend201WithoutAnyTokenAndExactBody(t *testing.T) {
	rig := newAcctRig(t, acctDirectory{})
	w := rig.do(http.MethodPost, AccountlessReportsPath, acctBody(acctHostA), acctKey)
	if w.Code != http.StatusCreated {
		t.Fatalf("send = %d %s", w.Code, w.Body.String())
	}
	if got := strings.Join(keysOf(t, w.Body.Bytes()), ","); got != "acknowledge_due,code,resolve_due,status" {
		t.Errorf("201 keys = %s, want exactly code,status,acknowledge_due,resolve_due", got)
	}
	for _, banned := range []string{"0900000000", "Nguyễn", "Đống rác", "Hà Lam"} {
		if strings.Contains(w.Body.String(), banned) {
			t.Errorf("201 carries %q", banned)
		}
	}
	if rig.intake.xa[0] != acctXaA || !rig.intake.got[0].Owner.IsAccountless() || rig.intake.got[0].IP == "" {
		t.Errorf("intake got commune %v sender %+v", rig.intake.xa[0], rig.intake.got[0])
	}
}

func TestAccountlessSendReplaysTheSameShapeForTheSameKey(t *testing.T) {
	rig := newAcctRig(t, acctDirectory{})
	first := rig.do(http.MethodPost, AccountlessReportsPath, acctBody(acctHostA), acctKey)
	again := rig.do(http.MethodPost, AccountlessReportsPath, acctBody(acctHostA), acctKey)
	if rig.intake.calls != 1 {
		t.Fatalf("intake ran %d times for one key", rig.intake.calls)
	}
	if again.Code != http.StatusCreated || again.Header().Get(idem.HeaderPhatLai) != "true" {
		t.Fatalf("replay = %d replay=%q", again.Code, again.Header().Get(idem.HeaderPhatLai))
	}
	if strings.TrimSpace(again.Body.String()) != strings.TrimSpace(first.Body.String()) {
		t.Errorf("replay body %s != first %s", again.Body.String(), first.Body.String())
	}
	// The same key in ANOTHER commune is a different key space: a fresh send there.
	rig.do(http.MethodPost, AccountlessReportsPath, acctBody(acctHostB), acctKey)
	if rig.intake.calls != 2 || rig.intake.xa[1] != acctXaB {
		t.Errorf("same key, commune B: calls %d", rig.intake.calls)
	}
}

func TestAccountlessSendRefusals(t *testing.T) {
	cases := []struct {
		name, body, key, code string
		status                int
	}{
		{"unknown domain", acctBody("khong-co.example.gov.vn"), acctKey, "commune_not_found", 404},
		{"inactive commune", acctBody(acctHostInactive), acctKey, "commune_not_found", 404},
		{"malformed domain", acctBody("https://xa-a.example.gov.vn"), acctKey, "invalid_host", 400},
		{"owner claimed", strings.Replace(acctBody(acctHostA), `{`, `{"cong_dan_id":"cd-1",`, 1), acctKey, "invalid_request", 400},
		{"tenant id is not a field", strings.Replace(acctBody(acctHostA), `"host":"`+acctHostA+`"`, `"tenant_id":"`+string(acctXaA)+`"`, 1), acctKey, "invalid_host", 400},
		{"scene location", strings.Replace(acctBody(acctHostA), `{`, `{"lat":15.7,"lng":108.3,`, 1), acctKey, "invalid_request", 400},
		// ADR 0088: the session intake takes a residential unit; this path refuses it (accountless.go).
		{"residential unit", strings.Replace(acctBody(acctHostA), `{`, `{"residential_unit_id":"01JUNITATEST",`, 1), acctKey, "invalid_request", 400},
		{"missing key", acctBody(acctHostA), "", "missing_idempotency_key", 400},
		{"short key", acctBody(acctHostA), "0123456789abcdef", "invalid_idempotency_key", 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rig := newAcctRig(t, acctDirectory{})
			w := rig.do(http.MethodPost, AccountlessReportsPath, c.body, c.key)
			if w.Code != c.status || errCode(t, w) != c.code {
				t.Errorf("= %d %s, want %d %s", w.Code, w.Body.String(), c.status, c.code)
			}
			if rig.intake.calls != 0 {
				t.Errorf("intake ran for a refused send")
			}
		})
	}
}

func TestAccountlessSendLimits(t *testing.T) {
	t.Run("6th send in the hour", func(t *testing.T) {
		rig := newAcctRig(t, acctDirectory{})
		var w *httptest.ResponseRecorder
		keys := []string{"00", "01", "02", "03", "04", "05"}
		for _, k := range keys {
			w = rig.do(http.MethodPost, AccountlessReportsPath, acctBody(acctHostA), acctKey[:30]+k)
		}
		if w.Code != http.StatusTooManyRequests || errCode(t, w) != "rate_limited" ||
			!strings.Contains(w.Body.String(), "Bộ phận tiếp nhận của Ủy ban nhân dân xã") {
			t.Errorf("6th = %d %s", w.Code, w.Body.String())
		}
		if rig.intake.calls != 5 {
			t.Errorf("intake ran %d times, want 5", rig.intake.calls)
		}
	})
	t.Run("commune daily ceiling", func(t *testing.T) {
		rig := newAcctRig(t, acctDirectory{})
		rig.intake.err = app.ErrAccountlessDailyLimit
		w := rig.do(http.MethodPost, AccountlessReportsPath, acctBody(acctHostA), acctKey)
		if w.Code != http.StatusTooManyRequests || errCode(t, w) != "commune_daily_limit" ||
			!strings.Contains(w.Body.String(), "Bộ phận tiếp nhận của Ủy ban nhân dân xã") {
			t.Errorf("= %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("commune not configured", func(t *testing.T) {
		rig := newAcctRig(t, acctDirectory{})
		rig.intake.err = app.ErrChuaAnDinhDuocHan
		if w := rig.do(http.MethodPost, AccountlessReportsPath, acctBody(acctHostA), acctKey); w.Code != 503 ||
			errCode(t, w) != "intake_not_configured" {
			t.Errorf("= %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("redis down", func(t *testing.T) {
		rig := newAcctRig(t, acctDirectory{})
		rig.counter.down = true
		if w := rig.do(http.MethodPost, AccountlessReportsPath, acctBody(acctHostA), acctKey); w.Code != 503 ||
			errCode(t, w) != "rate_limit_unavailable" || rig.intake.calls != 0 {
			t.Errorf("= %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("platform down", func(t *testing.T) {
		rig := newAcctRig(t, acctDirectory{down: true})
		if w := rig.do(http.MethodPost, AccountlessReportsPath, acctBody(acctHostA), acctKey); w.Code != 503 ||
			errCode(t, w) != "platform_unavailable" {
			t.Errorf("= %d %s", w.Code, w.Body.String())
		}
	})
}

// --- lookup -----------------------------------------------------------------------------------

func TestAccountlessLookup200HasOnlyTheDecidedFields(t *testing.T) {
	rig := newAcctRig(t, acctDirectory{})
	w := rig.do(http.MethodGet, AccountlessReportsPath+"/"+acctCodeA+"?host="+acctHostA, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("= %d %s", w.Code, w.Body.String())
	}
	if got := strings.Join(keysOf(t, w.Body.Bytes()), ","); got != "acknowledge_due,code,resolve_due,result,status" {
		t.Errorf("200 keys = %s", got)
	}
	for _, banned := range []string{"0900000000", "Nguyễn", "Đống rác", "Hà Lam"} {
		if strings.Contains(w.Body.String(), banned) {
			t.Errorf("lookup carries %q", banned)
		}
	}
}

func TestAccountlessLookup404IsIdenticalForEveryCause(t *testing.T) {
	rig := newAcctRig(t, acctDirectory{})
	var bodies []string
	for name, path := range map[string]string{
		"unknown domain":         "/" + acctCodeA + "?host=khong-co.example.gov.vn",
		"malformed domain":       "/" + acctCodeA + "?host=XA",
		"inactive commune":       "/" + acctCodeA + "?host=" + acctHostInactive,
		"wrong code":             "/PA-NONE-0000-0000?host=" + acctHostA,
		"another commune's code": "/" + acctCodeB + "?host=" + acctHostA,
		"a citizen's petition":   "/" + acctOwnedA + "?host=" + acctHostA,
	} {
		w := rig.do(http.MethodGet, AccountlessReportsPath+path, "", "")
		if w.Code != http.StatusNotFound {
			t.Errorf("%s = %d", name, w.Code)
		}
		bodies = append(bodies, w.Body.String())
	}
	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Errorf("404 bodies differ: %q vs %q", b, bodies[0])
		}
	}
}

func TestAccountlessLookup31stIs429(t *testing.T) {
	rig := newAcctRig(t, acctDirectory{})
	var w *httptest.ResponseRecorder
	for i := 0; i < ratelimit.AccountlessLookupLimit+1; i++ {
		// Varying the domain buys no budget: the key is the network alone.
		host := acctHostA
		if i%2 == 1 {
			host = acctHostB
		}
		w = rig.do(http.MethodGet, AccountlessReportsPath+"/"+acctCodeA+"?host="+host, "", "")
	}
	if w.Code != http.StatusTooManyRequests || errCode(t, w) != "rate_limited" {
		t.Errorf("31st = %d %s", w.Code, w.Body.String())
	}
}

// --- fields -----------------------------------------------------------------------------------

func TestAccountlessFields(t *testing.T) {
	rig := newAcctRig(t, acctDirectory{})
	w := rig.do(http.MethodGet, AccountlessFieldsPath+"?host="+acctHostA, "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `{"items":[{"code":"rac-thai","label":"Rác thải","icon":null,"tone":null}]}`) {
		t.Errorf("fields = %d %s", w.Code, w.Body.String())
	}
	for name, c := range map[string]struct {
		q      string
		status int
		code   string
	}{
		"unknown":   {"?host=khong-co.example.gov.vn", 404, "commune_not_found"},
		"inactive":  {"?host=" + acctHostInactive, 404, "commune_not_found"},
		"malformed": {"?host=a", 400, "invalid_host"},
		"missing":   {"", 400, "invalid_host"},
	} {
		if w := rig.do(http.MethodGet, AccountlessFieldsPath+c.q, "", ""); w.Code != c.status || errCode(t, w) != c.code {
			t.Errorf("%s = %d %s", name, w.Code, w.Body.String())
		}
	}
	down := newAcctRig(t, acctDirectory{})
	down.counter.down = true
	if w := down.do(http.MethodGet, AccountlessFieldsPath+"?host="+acctHostA, "", ""); w.Code != 503 {
		t.Errorf("redis down = %d, want 503 (fails closed)", w.Code)
	}
}

// --- staff DTO --------------------------------------------------------------------------------

func TestStaffDTOCarriesAccountless(t *testing.T) {
	for name, c := range map[string]struct {
		p    domain.PhieuPhanAnh
		want bool
	}{
		"accountless": {domain.PhieuPhanAnh{Kenh: domain.KenhZaloMiniApp}, true},
		"citizen":     {domain.PhieuPhanAnh{Kenh: domain.KenhZaloMiniApp, CongDanID: "cd-1"}, false},
		"staff":       {domain.PhieuPhanAnh{Kenh: domain.KenhCanBoNhapHo}, false},
	} {
		ra := phieuRaNgoai(c.p, "", false)
		if ra.Accountless == nil || *ra.Accountless != c.want {
			t.Errorf("%s: accountless = %v, want %v", name, ra.Accountless, c.want)
		}
	}
}
