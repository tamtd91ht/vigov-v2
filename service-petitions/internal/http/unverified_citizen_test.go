package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// THE UNVERIFIED PETITION AT THE HTTP BOUNDARY — ADR 0080, over the REAL citizen chain.
//
// Citizen routes hold no permissions (rule 5, invariant 6), so rule 5 invariant 7's four cases read
// here as: 401 no usable session · 403 `chua_xac_thuc_so` for a Zalo-only session on a route that
// stays verified-phone only (the "wrong class" case) · the OTHER owner / other account gets the same
// 404 as a non-existent code (the "wrong commune / wrong owner" case) · 201/200 for the owner.
//
//	PROVED HERE   a Zalo-only session files a petition (201, owner = the account, kind zalo-account,
//	              contact_unverified true, no citizen id) · the 11th of the day is 429
//	              unverified_daily_limit and the use case wrote nothing · the lookup serves the owning
//	              account and answers another account, a citizen and an unknown code BYTE FOR BYTE alike
//	              · "Phản ánh của tôi", the rating and the verification photos stay 403 for that session
//	              · the scene-photo routes reach the use case with the account as actor · idem replays
//	              the same code to the same account and never to another · a verified citizen's 201 says
//	              contact_unverified false · the staff response carries contact_unverified and never the
//	              account id.
//	NOT PROVED    the store's SQL (store/petition_owner_test.go) or the use case's transaction
//	              (app/unverified_intake_test.go).

const (
	tokenZaloA = "token-phien-zalo-A-CHUA-CO-SO-GIA"
	tokenZaloB = "token-phien-zalo-B-CHUA-CO-SO-GIA"
	idZaloA    = "01JZALOACCOUNTAAAAAAAAAAAA"
	idZaloB    = "01JZALOACCOUNTBBBBBBBBBBBB"
)

func TestUnverifiedSessionFilesPetitionOwnedByAccount(t *testing.T) {
	m := dungMayChuGui(t)

	w := m.gui(t, thanThu, tokenZaloA, khoaThu)
	doiMa(t, w, http.StatusCreated)

	if len(m.so.thayCongDan) != 1 || m.so.thayCongDan[0] != idZaloA ||
		m.so.thayKind[0] != string(domain.OwnerZaloAccount) || m.so.thayIP[0] != "10.0.0.9" {
		t.Fatalf("owner to the use case = %v / %v / %v, want the session's account", m.so.thayCongDan, m.so.thayKind, m.so.thayIP)
	}
	ra := docPhieuCuaToi(t, w.Body.Bytes())
	if ra.ContactUnverified == nil || !*ra.ContactUnverified {
		t.Errorf("contact_unverified = %v, want true — the Mini App must tell the citizen they will not be notified", ra.ContactUnverified)
	}
	stored := m.so.theo[xaA][ra.Code]
	if stored.ZaloAccountID != idZaloA || stored.CongDanID != "" {
		t.Errorf("stored owner = (cong_dan_id %q, zalo_account_id %q)", stored.CongDanID, stored.ZaloAccountID)
	}
	if strings.Contains(w.Body.String(), idZaloA) {
		t.Errorf("the account id left the API: %s", w.Body.String())
	}
}

func TestVerifiedCitizenResponseSaysContactVerified(t *testing.T) {
	m := dungMayChuGui(t)
	w := m.gui(t, thanThu, tokenCuaToi, khoaThu)
	doiMa(t, w, http.StatusCreated)
	ra := docPhieuCuaToi(t, w.Body.Bytes())
	if ra.ContactUnverified == nil || *ra.ContactUnverified {
		t.Errorf("contact_unverified = %v, want false on a verified citizen's petition", ra.ContactUnverified)
	}
	if m.so.thayKind[0] != string(domain.OwnerCitizen) || m.so.thayCongDan[0] != idToi {
		t.Errorf("verified path changed: %v / %v", m.so.thayKind, m.so.thayCongDan)
	}
}

func TestUnverifiedEleventhPetitionOfTheDayIs429(t *testing.T) {
	m := dungMayChuGui(t)
	for i := 0; i < domain.UnverifiedDailyCeiling; i++ {
		doiMa(t, m.gui(t, thanThu, tokenZaloA, khoaThu+string(rune('A'+i))), http.StatusCreated)
	}
	before := len(m.so.theo[xaA])

	w := m.gui(t, thanThu, tokenZaloA, khoaThu+"-ELEVENTH")
	doiMa(t, w, http.StatusTooManyRequests)
	if !strings.Contains(w.Body.String(), `"unverified_daily_limit"`) {
		t.Errorf("body = %s, want the stable key unverified_daily_limit", w.Body.String())
	}
	if strings.Contains(w.Body.String(), domain.TienToMaTraCuu+"-") {
		t.Errorf("a lookup code left a refused intake: %s", w.Body.String())
	}
	if len(m.so.theo[xaA]) != before {
		t.Errorf("%d petitions written past the ceiling", len(m.so.theo[xaA])-before)
	}
	// Another account in the same commune is not affected by the first one's ceiling.
	doiMa(t, m.gui(t, thanThu, tokenZaloB, khoaThu+"-OTHER"), http.StatusCreated)
}

// TestUnverifiedLookupOnlyForTheOwningAccount — ADR 0080 decision 3 and stop condition #6: code AND
// same account; another account, a citizen and an unknown code are ONE body.
func TestUnverifiedLookupOnlyForTheOwningAccount(t *testing.T) {
	m := dungMayChuGui(t)
	w := m.gui(t, thanThu, tokenZaloA, khoaThu)
	doiMa(t, w, http.StatusCreated)
	ma := docPhieuCuaToi(t, w.Body.Bytes()).Code

	own := m.doc(t, duongTapCongDan+"/"+ma, tokenZaloA)
	doiMa(t, own, http.StatusOK)
	if !bytes.Equal(own.Body.Bytes(), w.Body.Bytes()) {
		t.Errorf("the 200 and the 201 differ:\n201: %s\n200: %s", w.Body.String(), own.Body.String())
	}

	unknown := m.doc(t, duongTapCongDan+"/"+maKhongTonTai, tokenZaloA)
	doiMa(t, unknown, http.StatusNotFound)
	for name, token := range map[string]string{"another Zalo account": tokenZaloB, "a verified citizen": tokenCuaToi} {
		r := m.doc(t, duongTapCongDan+"/"+ma, token)
		doiMa(t, r, http.StatusNotFound)
		if !bytes.Equal(r.Body.Bytes(), unknown.Body.Bytes()) {
			t.Errorf("%s: body differs from a non-existent code's — leaks the petition exists (rule 4, forbidden #2)\n got: %s\nwant: %s",
				name, r.Body.String(), unknown.Body.String())
		}
	}

	// And the other direction: an account cannot open a CITIZEN's petition with its code.
	c := m.gui(t, thanThu, tokenCuaToi, khoaThu+"-CITIZEN")
	doiMa(t, c, http.StatusCreated)
	r := m.doc(t, duongTapCongDan+"/"+docPhieuCuaToi(t, c.Body.Bytes()).Code, tokenZaloA)
	doiMa(t, r, http.StatusNotFound)
	if !bytes.Equal(r.Body.Bytes(), unknown.Body.Bytes()) {
		t.Errorf("a citizen's petition answered the account differently from an unknown code: %s", r.Body.String())
	}
}

// TestUnverifiedIdempotencyIsPerAccount — the same key from the same account replays the same code;
// from another account it is a different petition with a different code (core/idem keys by Kind:ID).
func TestUnverifiedIdempotencyIsPerAccount(t *testing.T) {
	m := dungMayChuGui(t)
	one := m.gui(t, thanThu, tokenZaloA, khoaThu)
	doiMa(t, one, http.StatusCreated)
	code := docPhieuCuaToi(t, one.Body.Bytes()).Code

	replay := m.gui(t, thanThu, tokenZaloA, khoaThu)
	if m.so.demGui != 1 || !strings.Contains(replay.Body.String(), code) {
		t.Fatalf("replay ran the use case %d times / did not carry %q: %s", m.so.demGui, code, replay.Body.String())
	}

	other := m.gui(t, thanThu, tokenZaloB, khoaThu)
	doiMa(t, other, http.StatusCreated)
	if m.so.demGui != 2 || docPhieuCuaToi(t, other.Body.Bytes()).Code == code {
		t.Errorf("another account with the same key was handed the first account's code %q", code)
	}
	// Nor does a verified citizen sharing the key get it.
	cit := m.gui(t, thanThu, tokenCuaToi, khoaThu)
	doiMa(t, cit, http.StatusCreated)
	if docPhieuCuaToi(t, cit.Body.Bytes()).Code == code {
		t.Errorf("a citizen with the same key was handed the account's code %q", code)
	}
}

// TestUnverifiedSessionStillRefusedOnVerifiedOnlyRoutes — "Phản ánh của tôi", the rating and the
// verification photos stay XaTuPhien (ADR 0080 decisions 3 and 8, stop condition #4): 403
// chua_xac_thuc_so, and nothing behind the gate runs.
func TestUnverifiedSessionStillRefusedOnVerifiedOnlyRoutes(t *testing.T) {
	for _, rt := range []struct{ name, method, path, body string }{
		{"list own petitions", http.MethodGet, "/api/v1/my-citizen-reports", ""},
		{"rate", http.MethodPost, duongCuaToi(maCuaToi) + "/rating", `{"stars":5}`},
		{"verification photos", http.MethodGet, duongCuaToi(maCuaToi) + "/verification-photos", ""},
	} {
		t.Run(rt.name, func(t *testing.T) {
			petitions, rating, verif := phieuCuaToiMau(), newRatingFake(), newCitizenVerificationPhotosFake()
			h := unverifiedChain(t, DepsCongDan{Phieu: petitions, GuiPhieu: soPhieuMoi(), Rating: rating,
				NhanLinhVuc: nhanLinhVucMau(), CitizenFields: newFieldCatalogueFake(), Photos: newCitizenPhotosFake(),
				VerificationPhotos: verif, PhotoLimiter: photoLimiterThu(), UploadSlots: uploadSlotsThu()})
			w := serveCitizen(h, rt.method, rt.path, rt.body, tokenZaloA)
			doiMa(t, w, http.StatusForbidden)
			if !strings.Contains(w.Body.String(), `"chua_xac_thuc_so"`) {
				t.Errorf("body = %s", w.Body.String())
			}
			if petitions.goi != 0 || rating.calls != 0 || verif.calls != 0 {
				t.Errorf("a later layer ran for a Zalo-only session (read %d · rating %d · verification %d)",
					petitions.goi, rating.calls, verif.calls)
			}
		})
	}
}

// TestUnverifiedSessionReachesScenePhotosAsAccount — the three scene-photo routes serve the account
// (decision 8), with the account as the actor and the session's commune.
func TestUnverifiedSessionReachesScenePhotosAsAccount(t *testing.T) {
	photos := newCitizenPhotosFake()
	const maZalo = "PA-ZALO-HTTP-0001"
	photos.owner[xaA][maZalo] = idZaloA
	h := unverifiedChain(t, DepsCongDan{Phieu: phieuCuaToiMau(), GuiPhieu: soPhieuMoi(), Rating: newRatingFake(),
		NhanLinhVuc: nhanLinhVucMau(), CitizenFields: newFieldCatalogueFake(), Photos: photos,
		VerificationPhotos: newCitizenVerificationPhotosFake(), PhotoLimiter: photoLimiterThu(), UploadSlots: uploadSlotsThu()})

	doiMa(t, serveCitizenUpload(t, h, photosPath(maZalo), photoFile(), tokenZaloA), http.StatusCreated)
	doiMa(t, serveCitizen(h, http.MethodGet, photosPath(maZalo), "", tokenZaloA), http.StatusOK)
	if len(photos.seenCitizen) != 2 {
		t.Fatalf("use case calls = %d, want the upload and the list", len(photos.seenCitizen))
	}
	for i, c := range photos.seenCitizen {
		if c.ID != idZaloA || c.Kind != audit.KindZaloAccount || photos.seenTenant[i] != xaA {
			t.Errorf("call %d actor = %+v in %q, want the account %q kind %q in commune A", i, c, photos.seenTenant[i],
				idZaloA, audit.KindZaloAccount)
		}
	}
	// Another account: the same 404 body as a non-existent code.
	other := serveCitizen(h, http.MethodGet, photosPath(maZalo), "", tokenZaloB)
	unknown := serveCitizen(h, http.MethodGet, photosPath(maKhongTonTai), "", tokenZaloB)
	doiMa(t, other, http.StatusNotFound)
	if !bytes.Equal(other.Body.Bytes(), unknown.Body.Bytes()) {
		t.Errorf("another account's photos answered differently from an unknown code: %s", other.Body.String())
	}
}

// TestStaffResponseCarriesContactUnverified — ADR 0080 decision 5: an explicit server field, both
// ways, and never the account id.
func TestStaffResponseCarriesContactUnverified(t *testing.T) {
	for _, c := range []struct {
		name string
		p    domain.PhieuPhanAnh
		want bool
	}{
		{"zalo-owned", domain.PhieuPhanAnh{MaTraCuu: "PA-Z", Kenh: domain.KenhZaloMiniApp, ZaloAccountID: idZaloA,
			TrangThai: domain.DaTiepNhan}, true},
		{"citizen-filed", domain.PhieuPhanAnh{MaTraCuu: "PA-C", Kenh: domain.KenhZaloMiniApp, CongDanID: idToi,
			TrangThai: domain.DaTiepNhan}, false},
		{"staff-booked", domain.PhieuPhanAnh{MaTraCuu: "PA-S", Kenh: domain.KenhCanBoNhapHo,
			TrangThai: domain.DaTiepNhan}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(phieuRaNgoai(c.p, "", false))
			if err != nil {
				t.Fatal(err)
			}
			var tho map[string]any
			if err := json.Unmarshal(b, &tho); err != nil {
				t.Fatal(err)
			}
			got, ok := tho["contact_unverified"].(bool)
			if !ok || got != c.want {
				t.Errorf("contact_unverified = %v (present %v), want %v: %s", tho["contact_unverified"], ok, c.want, b)
			}
			if strings.Contains(string(b), idZaloA) || strings.Contains(string(b), "zalo_account") {
				t.Errorf("the account id reached the staff surface: %s", b)
			}
			// A Zalo-owned petition has no citizen: it closes from da-xu-ly like a staff-booked one.
			if c.name == "zalo-owned" && tho["has_citizen"] != false {
				t.Errorf("has_citizen = %v, want false", tho["has_citizen"])
			}
		})
	}
}

func TestContactUnverifiedIsOptionalInTheContract(t *testing.T) {
	for _, v := range []any{phieuPhanAnhRa{}, phieuCuaToiRa{}} {
		b, _ := json.Marshal(v)
		if strings.Contains(string(b), `"contact_unverified"`) {
			t.Errorf("contact_unverified emitted when unset — lost omitempty, apidoc would declare it REQUIRED: %s", b)
		}
	}
}

// --- harness ---------------------------------------------------------------------------------------

func unverifiedChain(t *testing.T, d DepsCongDan) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d.Log = log
	mux := http.NewServeMux()
	RegisterCongDan(mux, d)
	var h http.Handler = mux
	h = idem.Middleware(khoIdemMoi(), log)(h)
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(&soPhienCongDanGia{})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	return httpx.StripTenantHeaders(h)
}

func serveCitizen(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://"+hostMiniApp+path, strings.NewReader(body))
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	r.Header.Set("Authorization", "Bearer "+token)
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(idem.Header, "01JKEYUNVERIFIED"+strings.NewReplacer("/", "", "{", "", "}", "").Replace(path))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// serveCitizenUpload is serveCitizen for a multipart file upload (upload.go).
func serveCitizenUpload(t *testing.T, h http.Handler, path string, u uploadThan, token string) *httptest.ResponseRecorder {
	t.Helper()
	b, ct := u.encode(t)
	r := httptest.NewRequest(http.MethodPost, "https://"+hostMiniApp+path, b)
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", ct)
	r.Header.Set(idem.Header, "01JKEYUNVERIFIED"+strings.NewReplacer("/", "", "{", "", "}", "").Replace(path))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
