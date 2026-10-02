package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The four scene-photo routes at the HTTP boundary.
//
//	PROVED HERE   citizen routes over the REAL citizen chain (CitizenEdge → CitizenPrincipal →
//	              XaTuPhien → idem): 401 with no usable session, use case not reached · another
//	              citizen's code, another commune's and an unknown one answer BYTE FOR BYTE the GET's
//	              404 · the citizen and the commune reach the use case FROM THE SESSION · the declared
//	              body reaches it · replies carry the form / links with `no-store` · a replay of the
//	              upload request answers the file id and does not reach the use case twice · the
//	              per-citizen rate limit (429 + Retry-After, per citizen, 503 when its store is down) ·
//	              the refusal mapping (409 / 422 / 503) never echoes the code.
//	              Staff route, rule 5 invariant 7: 401 no session · 403 without `feedback.read` · 401
//	              right key WRONG COMMUNE (authz compares the commune first) · 200 · `can-bo` without
//	              `feedback.restricted` is the detail's 404.
//	NOT PROVED    the flow — internal/app/petition_photo_test.go.

const photoIDHTTP = "01JPH0THTTP000000000000001"

// citizenPhotosFake APPLIES THE OWNERSHIP RULE ITSELF, keyed by commune, so the isolation cases fail
// if the handler hands down the wrong identity or commune.
type citizenPhotosFake struct {
	mu          sync.Mutex
	owner       map[tenant.ID]map[string]string // code → citizen id
	calls       int
	seenCitizen []audit.Actor
	seenTenant  []tenant.ID
	seenReq     []app.PhotoUploadRequest
	err         error
}

func newCitizenPhotosFake() *citizenPhotosFake {
	return &citizenPhotosFake{owner: map[tenant.ID]map[string]string{
		xaA: {maCuaToi: idToi, maCuaNguoiKhac: idNguoiKhac},
		xaB: {},
	}}
}

func (f *citizenPhotosFake) check(ctx context.Context, ma string, c audit.Actor) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.seenCitizen = append(f.seenCitizen, c)
	f.seenTenant = append(f.seenTenant, tenant.MustFrom(ctx))
	if f.err != nil {
		return f.err
	}
	if f.owner[tenant.MustFrom(ctx)][ma] != c.ID || c.ID == "" {
		return petstore.ErrPhieuKhongTonTai
	}
	return nil
}

var photoAtHTTP = time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)

func (f *citizenPhotosFake) RequestUpload(ctx context.Context, ma string, req app.PhotoUploadRequest,
	c audit.Actor) (app.PhotoUpload, error) {
	if err := f.check(ctx, ma, c); err != nil {
		return app.PhotoUpload{}, err
	}
	f.mu.Lock()
	f.seenReq = append(f.seenReq, req)
	f.mu.Unlock()
	return app.PhotoUpload{
		File: domain.StoredFile{ID: photoIDHTTP, Status: domain.StoredFilePending, CreatedAt: photoAtHTTP},
		Post: storage.PresignedPost{URL: "https://s3.example.gov.vn/vigov-test-temp",
			Fields: map[string]string{"key": "upload/citizen-media/k/original.png", "policy": "p"}, ExpiresAt: photoAtHTTP.Add(15 * time.Minute)},
	}, nil
}

func (f *citizenPhotosFake) Complete(ctx context.Context, ma, id string, c audit.Actor) (domain.StoredFile, error) {
	if err := f.check(ctx, ma, c); err != nil {
		return domain.StoredFile{}, err
	}
	return domain.StoredFile{ID: id, MIMEType: storage.MIMEJPEG, SizeBytes: 481_000, Status: domain.StoredFileStored,
		CreatedAt: photoAtHTTP}, nil
}

func (f *citizenPhotosFake) ListPhotos(ctx context.Context, ma string, c audit.Actor) ([]app.PhotoLink, error) {
	if err := f.check(ctx, ma, c); err != nil {
		return nil, err
	}
	return []app.PhotoLink{{
		File: domain.StoredFile{ID: photoIDHTTP, MIMEType: storage.MIMEJPEG, SizeBytes: 481_000, CreatedAt: photoAtHTTP},
		URL:  storage.PresignedURL("https://s3.example.gov.vn/vigov-test-private/k?X-Amz-Signature=s"), ExpiresAt: photoAtHTTP.Add(15 * time.Minute),
	}}, nil
}

// staffPhotosFake is keyed by commune; the restricted petition needs the flag the handler computes.
type staffPhotosFake struct {
	calls      int
	seenTenant tenant.ID
	restricted bool
	reader     audit.Actor
	err        error
}

func newStaffPhotosFake() *staffPhotosFake { return &staffPhotosFake{} }

func (f *staffPhotosFake) ListPhotos(ctx context.Context, ma string, mayReadRestricted bool,
	reader audit.Actor) ([]app.PhotoLink, error) {
	f.calls++
	f.seenTenant, f.restricted, f.reader = tenant.MustFrom(ctx), mayReadRestricted, reader
	if f.err != nil {
		return nil, f.err
	}
	switch {
	case tenant.MustFrom(ctx) == xaA && ma == maPhieuThuong:
	case tenant.MustFrom(ctx) == xaA && ma == maPhieuCanBo && mayReadRestricted:
	default:
		return nil, petstore.ErrPhieuKhongTonTai
	}
	return []app.PhotoLink{{File: domain.StoredFile{ID: photoIDHTTP, MIMEType: storage.MIMEJPEG, SizeBytes: 9},
		URL: storage.PresignedURL("https://s3.example.gov.vn/p?X-Amz-Signature=s"), ExpiresAt: photoAtHTTP}}, nil
}

// photoCounter is an in-memory fixed window for the real ratelimit policy.
type photoCounter struct {
	mu   sync.Mutex
	n    map[string]int64
	keys []string
	fail error
}

func (c *photoCounter) Incr(_ context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail != nil {
		return 0, 0, c.fail
	}
	if c.n == nil {
		c.n = map[string]int64{}
	}
	c.keys = append(c.keys, key)
	c.n[key]++
	return c.n[key], window, nil
}

func photoLimiterOn(c ratelimit.Counter) *ratelimit.Limiter {
	l, err := ratelimit.New(c, ratelimit.CitizenPhotoUpload)
	if err != nil {
		panic(err)
	}
	return l
}

// photoLimiterThu is the limiter every other citizen suite mounts: the real policy, a fresh counter.
func photoLimiterThu() *ratelimit.Limiter { return photoLimiterOn(&photoCounter{}) }

type photoServer struct {
	h       http.Handler
	photos  *citizenPhotosFake
	counter *photoCounter
}

func buildPhotoServer(t *testing.T) *photoServer {
	t.Helper()
	f := newCitizenPhotosFake()
	c := &photoCounter{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu: phieuCuaToiMau(), GuiPhieu: soPhieuMoi(), Rating: newRatingFake(), NhanLinhVuc: nhanLinhVucMau(),
		CitizenFields: newFieldCatalogueFake(), Photos: f, VerificationPhotos: newCitizenVerificationPhotosFake(), PhotoLimiter: photoLimiterOn(c), Log: log,
	})
	var h http.Handler = mux
	h = idem.Middleware(khoIdemMoi(), log)(h)
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(&soPhienCongDanGia{})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &photoServer{h: h, photos: f, counter: c}
}

func photosPath(code string) string              { return "/api/v1/my-citizen-reports/" + code + "/photos" }
func photoCompletionPath(code, id string) string { return photosPath(code) + "/" + id + "/completion" }

func (s *photoServer) do(t *testing.T, method, path, body, token, idemKey string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "https://"+hostMiniApp+path, strings.NewReader(body))
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if idemKey != "" {
		r.Header.Set(idem.Header, idemKey)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const (
	photoBody    = `{"content_type":"image/png","size":812000}`
	photoIdemKey = "01JKHOAANHHIENTRUONGTHU001"
)

type photoRoute struct {
	name, method, path, body, key string
	ok                            int
}

func photoRoutes(code string) []photoRoute {
	return []photoRoute{
		{"upload slot", http.MethodPost, photosPath(code), photoBody, photoIdemKey, http.StatusCreated},
		{"completion", http.MethodPost, photoCompletionPath(code, photoIDHTTP), "", "", http.StatusOK},
		{"own list", http.MethodGet, photosPath(code), "", "", http.StatusOK},
	}
}

// --- citizen: 401 · 404 · 2xx -----------------------------------------------------------------------

func TestCitizenPhotoRoutesNoSession401(t *testing.T) {
	for _, c := range photoRoutes(maCuaToi) {
		for name, token := range map[string]string{"no header": "", "unknown token": "token-khong-ai-cap-BAO-GIO"} {
			t.Run(c.name+"/"+name, func(t *testing.T) {
				s := buildPhotoServer(t)
				doiMa(t, s.do(t, c.method, c.path, c.body, token, c.key), http.StatusUnauthorized)
				if s.photos.calls != 0 {
					t.Errorf("use case reached without a session (%d)", s.photos.calls)
				}
			})
		}
	}
}

func TestCitizenPhotoRoutesOtherCitizenOtherCommuneUnknownAreTheGets404(t *testing.T) {
	s := buildPhotoServer(t)
	unknown := s.do(t, http.MethodGet, duongCuaToi(maKhongTonTai), "", tokenCuaToi, "")
	doiMa(t, unknown, http.StatusNotFound)
	for _, cs := range []struct{ name, code, token string }{
		{"another citizen's code, same commune", maCuaNguoiKhac, tokenCuaToi},
		{"own code, session in another commune", maCuaToi, tokenXaB},
		{"unknown code", maKhongTonTai, tokenCuaToi},
	} {
		for i, c := range photoRoutes(cs.code) {
			t.Run(cs.name+"/"+c.name, func(t *testing.T) {
				key := ""
				if c.key != "" {
					key = c.key + string(rune('A'+i)) + strings.ReplaceAll(cs.code[3:7], "-", "")
				}
				w := s.do(t, c.method, c.path, c.body, cs.token, key)
				doiMa(t, w, http.StatusNotFound)
				if w.Body.String() != unknown.Body.String() {
					t.Errorf("404 body differs from the GET's unknown code (rule 4 forbidden #2):\n %s\n %s",
						w.Body.String(), unknown.Body.String())
				}
			})
		}
	}
}

func TestCitizenPhotoRoutesPassIdentityAndCommuneFromTheSession(t *testing.T) {
	for _, c := range photoRoutes(maCuaToi) {
		t.Run(c.name, func(t *testing.T) {
			s := buildPhotoServer(t)
			w := s.do(t, c.method, c.path+"?tenant_id=01JB&citizen_id="+idNguoiKhac, c.body, tokenCuaToi, c.key)
			doiMa(t, w, c.ok)
			f := s.photos
			if f.calls != 1 || f.seenTenant[0] != xaA || f.seenCitizen[0].ID != idToi || f.seenCitizen[0].Kind != "citizen" {
				t.Fatalf("use case saw commune %v citizen %+v (%d calls)", f.seenTenant, f.seenCitizen, f.calls)
			}
			if strings.Contains(w.Body.String(), maCuaToi) || strings.Contains(w.Body.String(), idToi) {
				t.Errorf("reply carries the lookup code or the citizen id: %s", w.Body.String())
			}
			if c.method == http.MethodGet || c.ok == http.StatusCreated {
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Error("a reply carrying a bearer credential is cacheable")
				}
			}
		})
	}
}

func TestCitizenPhotoUploadReplyAndReplay(t *testing.T) {
	s := buildPhotoServer(t)
	w := s.do(t, http.MethodPost, photosPath(maCuaToi), photoBody, tokenCuaToi, photoIdemKey)
	doiMa(t, w, http.StatusCreated)
	var out photoUploadOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Photo.ID != photoIDHTTP || out.Photo.Status != "pending" || out.Upload.URL == "" || out.Upload.Fields["key"] == "" {
		t.Errorf("reply = %+v", out)
	}
	if s.photos.seenReq[0] != (app.PhotoUploadRequest{ContentType: storage.MIMEPNG, Size: 812000}) {
		t.Errorf("declared body reaching the use case = %+v", s.photos.seenReq[0])
	}
	again := s.do(t, http.MethodPost, photosPath(maCuaToi), photoBody, tokenCuaToi, photoIdemKey)
	doiMa(t, again, http.StatusCreated)
	if again.Header().Get(idem.HeaderPhatLai) != "true" || !strings.Contains(again.Body.String(), photoIDHTTP) ||
		strings.Contains(again.Body.String(), "policy") {
		t.Errorf("replay = %s %v", again.Body.String(), again.Header())
	}
	if s.photos.calls != 1 {
		t.Errorf("a replay reached the use case again (%d calls) — a second file", s.photos.calls)
	}
}

func TestCitizenPhotoListAndCompletionReplies(t *testing.T) {
	s := buildPhotoServer(t)
	w := s.do(t, http.MethodGet, photosPath(maCuaToi), "", tokenCuaToi, "")
	doiMa(t, w, http.StatusOK)
	var list photoListOut
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].URL == "" || list.Items[0].ContentType != storage.MIMEJPEG ||
		list.Items[0].SizeBytes != 481000 || list.Items[0].URLExpiresAt.IsZero() {
		t.Errorf("list = %+v", list)
	}
	for _, internal := range []string{"object_key", "uploaded_by", "file_name", "subject"} {
		if strings.Contains(w.Body.String(), internal) {
			t.Errorf("internal field %q on the citizen's reply", internal)
		}
	}
	c := s.do(t, http.MethodPost, photoCompletionPath(maCuaToi, photoIDHTTP), "", tokenCuaToi, "")
	doiMa(t, c, http.StatusOK)
	var p photoOut
	_ = json.Unmarshal(c.Body.Bytes(), &p)
	if p.ID != photoIDHTTP || p.Status != "stored" || p.ContentType != storage.MIMEJPEG {
		t.Errorf("completion = %+v", p)
	}
}

// --- citizen: the per-citizen rate limit ------------------------------------------------------------

func TestCitizenPhotoWritesAreRateLimitedPerCitizen(t *testing.T) {
	s := buildPhotoServer(t)
	for i := 0; i < ratelimit.CitizenPhotoUploadLimit; i++ {
		if w := s.do(t, http.MethodPost, photoCompletionPath(maCuaToi, photoIDHTTP), "", tokenCuaToi, ""); w.Code != http.StatusOK {
			t.Fatalf("write %d: %d", i+1, w.Code)
		}
	}
	w := s.do(t, http.MethodPost, photosPath(maCuaToi), photoBody, tokenCuaToi, photoIdemKey)
	doiMa(t, w, http.StatusTooManyRequests)
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 without Retry-After")
	}
	if s.photos.calls != ratelimit.CitizenPhotoUploadLimit {
		t.Errorf("the refused write reached the use case (%d calls)", s.photos.calls)
	}
	// Another citizen of the same commune has a budget of their own.
	if w := s.do(t, http.MethodPost, photoCompletionPath(maCuaNguoiKhac, photoIDHTTP), "", tokenNguoiKhac, ""); w.Code != http.StatusOK {
		t.Errorf("another citizen throttled on this one's budget: %d", w.Code)
	}
	// The list is not a write and is not counted.
	if w := s.do(t, http.MethodGet, photosPath(maCuaToi), "", tokenCuaToi, ""); w.Code != http.StatusOK {
		t.Errorf("list throttled: %d", w.Code)
	}
	for _, k := range s.counter.keys {
		if !strings.HasPrefix(k, "t:"+string(xaA)+":") || strings.Contains(k, idToi) {
			t.Errorf("counter key %q is not commune-scoped or names the citizen", k)
		}
	}
}

func TestCitizenPhotoRateLimitStoreDownFailsClosed(t *testing.T) {
	s := buildPhotoServer(t)
	s.counter.fail = errors.New("redis: connection refused")
	w := s.do(t, http.MethodPost, photosPath(maCuaToi), photoBody, tokenCuaToi, photoIdemKey)
	doiMa(t, w, http.StatusServiceUnavailable)
	if loiTra(t, w).Code != "rate_limit_unavailable" || s.photos.calls != 0 {
		t.Errorf("code %q, %d calls", loiTra(t, w).Code, s.photos.calls)
	}
}

// --- citizen: refusal mapping -----------------------------------------------------------------------

func TestCitizenPhotoRefusalMapping(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{app.ErrPhotoWindowClosed, http.StatusConflict, "petition_state"},
		{app.ErrPhotoCountReached, http.StatusConflict, "photo_limit"},
		{app.ErrPhotoTypeNotAllowed, http.StatusBadRequest, "invalid_request"},
		{app.ErrPhotoTooLarge, http.StatusBadRequest, "invalid_request"},
		{&app.AttachmentRejection{Reason: app.RejectMalware}, http.StatusUnprocessableEntity, "photo_rejected"},
		{&app.AttachmentRejection{Reason: app.RejectUndecodable}, http.StatusUnprocessableEntity, "photo_rejected"},
		{app.ErrUploadNotReceived, http.StatusConflict, "upload_not_received"},
		{app.ErrPhotoNotFound, http.StatusNotFound, "not_found"},
		{app.ErrUploadNotConfigured, http.StatusServiceUnavailable, "storage_not_configured"},
		{app.ErrScanUnavailable, http.StatusServiceUnavailable, "malware_scan_unavailable"},
		{app.ErrUploadLimitsUnavailable, http.StatusServiceUnavailable, "upload_limits_unavailable"},
		{errors.New("xu_ly_phan_anh: x cho xã " + string(xaA) + ": db down"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(c.code+"/"+c.err.Error(), func(t *testing.T) {
			s := buildPhotoServer(t)
			s.photos.err = c.err
			w := s.do(t, http.MethodPost, photoCompletionPath(maCuaToi, photoIDHTTP), "", tokenCuaToi, "")
			doiMa(t, w, c.status)
			if loiTra(t, w).Code != c.code {
				t.Errorf("code = %q, want %q", loiTra(t, w).Code, c.code)
			}
			if strings.Contains(w.Body.String(), string(xaA)) || strings.Contains(w.Body.String(), maCuaToi) {
				t.Errorf("body echoes the commune id or the code: %s", w.Body.String())
			}
		})
	}
}

// --- staff: rule 5, invariant 7 -----------------------------------------------------------------------

func staffPhotosPath(code string) string { return "/api/v1/citizen-reports/" + code + "/photos" }

func TestStaffPetitionPhotos_NoSession401(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, staffPhotosPath(maPhieuThuong), nil), http.StatusUnauthorized)
	if m.staffPhotos.calls != 0 {
		t.Error("use case reached without a session")
	}
}

func TestStaffPetitionPhotos_WrongPermission403(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.read"), authz.Perm("feedback.classify"))
	doiMa(t, m.goi(t, http.MethodGet, hostA, staffPhotosPath(maPhieuThuong), canBoCuaXa(xaA)), http.StatusForbidden)
	if m.staffPhotos.calls != 0 {
		t.Error("use case reached with the wrong permission")
	}
}

func TestStaffPetitionPhotos_RightPermissionWrongCommune401(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))
	doiMa(t, m.goi(t, http.MethodGet, hostB, staffPhotosPath(maPhieuXaB), canBoCuaXa(xaA)), http.StatusUnauthorized)
	if m.staffPhotos.calls != 0 {
		t.Error("commune B's photos reached with commune A's session — rò rỉ giữa hai cơ quan nhà nước")
	}
}

func TestStaffPetitionPhotos_RightPermissionRightCommune200(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))
	w := m.goi(t, http.MethodGet, hostA, staffPhotosPath(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if m.staffPhotos.calls != 1 || m.staffPhotos.seenTenant != xaA || m.staffPhotos.restricted {
		t.Fatalf("use case: %+v", m.staffPhotos)
	}
	// The audited reader is the officer's BUSINESS CODE, never the internal id (rule 6, invariant 8).
	if r := m.staffPhotos.reader; r.ID != maCanBo || r.ID == idCanBo || r.Kind != "staff" || r.IP == "" {
		t.Errorf("reader handed to the use case = %+v, want ID %q kind staff with an IP", r, maCanBo)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("signed links cacheable")
	}
	var list photoListOut
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list.Items) != 1 || list.Items[0].URL == "" {
		t.Errorf("reply = %s", w.Body.String())
	}
}

func TestStaffPetitionPhotos_RestrictedFieldIsTheDetails404(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))
	unknown := m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-reports/"+maKhongTonTai, canBoCuaXa(xaA))
	w := m.goi(t, http.MethodGet, hostA, staffPhotosPath(maPhieuCanBo), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusNotFound)
	if w.Body.String() != unknown.Body.String() {
		t.Errorf("restricted 404 differs from the detail's unknown-code 404:\n %s\n %s", w.Body.String(), unknown.Body.String())
	}
	m.capQuyen(t, authz.Perm("feedback.read"), QuyenHanChe)
	doiMa(t, m.goi(t, http.MethodGet, hostA, staffPhotosPath(maPhieuCanBo), canBoCuaXa(xaA)), http.StatusOK)
}

func TestStaffPetitionPhotos_StorageMissingIs503(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))
	m.staffPhotos.err = app.ErrUploadNotConfigured
	w := m.goi(t, http.MethodGet, hostA, staffPhotosPath(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusServiceUnavailable)
	if loiTra(t, w).Code != "storage_not_configured" {
		t.Errorf("code = %q", loiTra(t, w).Code)
	}
}
