package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-petitions/internal/app"
)

// The register list's `cover_photo` (owner decision 12/10/2026) at the HTTP boundary.
//
//	PROVED HERE   rule 5 invariant 7 on GET /api/v1/citizen-reports as it now hands out links: 401 no
//	              session · 403 wrong key · 401 right key wrong commune — the cover use case is NEVER
//	              reached on any of them · 200: the cover is on the item with a URL and `url_expires_at`,
//	              absent elsewhere, `no-store`, the reader is the officer's BUSINESS CODE · `can-bo`
//	              without `feedback.restricted` has no cover, with it has one · the use case's failure
//	              (the audit entry included) refuses the page with no link in the body · no object store
//	              or no business code serves the page without covers.
//	NOT PROVED    the selection, the signing and the audit entry — internal/app/petition_cover_test.go.

type listItemWithCover struct {
	Code       string        `json:"code"`
	CoverPhoto *photoLinkOut `json:"cover_photo"`
}

func listItems(t *testing.T, body []byte) map[string]listItemWithCover {
	t.Helper()
	var reply struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		t.Fatalf("reply not JSON: %s", body)
	}
	out := map[string]listItemWithCover{}
	for _, raw := range reply.Items {
		var it listItemWithCover
		if err := json.Unmarshal(raw, &it); err != nil {
			t.Fatal(err)
		}
		// ABSENT, not null: yesterday's clients must see the item exactly as before.
		if it.CoverPhoto == nil && strings.Contains(string(raw), `"cover_photo"`) {
			t.Errorf("item %s carries cover_photo without a cover: %s", it.Code, raw)
		}
		out[it.Code] = it
	}
	return out
}

func TestListCover_NoSession401(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongDanhSach, nil), http.StatusUnauthorized)
	if m.staffPhotos.coverCalls != 0 {
		t.Error("cover use case reached without a session")
	}
}

func TestListCover_WrongPermission403(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.read"))
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA)), http.StatusForbidden)
	if m.staffPhotos.coverCalls != 0 {
		t.Error("cover use case reached without feedback.read")
	}
}

func TestListCover_RightPermissionWrongCommune401(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))
	doiMa(t, m.goi(t, http.MethodGet, hostB, duongDanhSach, canBoCuaXa(xaA)), http.StatusUnauthorized)
	if m.staffPhotos.coverCalls != 0 {
		t.Error("commune B's covers reached with commune A's session — rò rỉ giữa hai cơ quan nhà nước")
	}
}

func TestListCover_PresentWithExpiry200(t *testing.T) {
	m := dungMayChu(t)
	m.staffPhotos.covers = map[string]bool{"pa-001": true}
	w := m.goi(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("a list carrying signed links is cacheable")
	}
	items := listItems(t, w.Body.Bytes())
	c := items[maPhieuThuong].CoverPhoto
	if c == nil || c.URL == "" || c.URLExpiresAt.IsZero() || c.ID == "" {
		t.Fatalf("cover on %s = %+v — want an id, a URL and its expiry", maPhieuThuong, c)
	}
	for code, it := range items {
		if code != maPhieuThuong && it.CoverPhoto != nil {
			t.Errorf("%s has a cover it was not given", code)
		}
	}
	// ONE call for the page; the audited reader is the BUSINESS CODE (rule 6, invariant 8).
	if m.staffPhotos.coverCalls != 1 || m.staffPhotos.seenTenant != xaA || m.staffPhotos.restricted {
		t.Fatalf("use case: %+v", m.staffPhotos)
	}
	if r := m.staffPhotos.reader; r.ID != maCanBo || r.Kind != "staff" || r.IP == "" {
		t.Errorf("reader = %+v, want %s / staff / an IP", r, maCanBo)
	}
}

func TestListCover_NoPhotoAbsent(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	for code, it := range listItems(t, w.Body.Bytes()) {
		if it.CoverPhoto != nil {
			t.Errorf("%s has a cover with no photo", code)
		}
	}
}

func TestListCover_RestrictedNeedsTheKey(t *testing.T) {
	m := dungMayChu(t)
	m.staffPhotos.covers = map[string]bool{"pa-001": true, "pa-002": true}
	w := m.goi(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if it, on := listItems(t, w.Body.Bytes())[maPhieuCanBo]; on && it.CoverPhoto != nil {
		t.Error("`can-bo` cover without feedback.restricted")
	}
	if m.staffPhotos.restricted {
		t.Error("the handler told the use case the caller holds feedback.restricted")
	}

	m.capQuyen(t, authz.Perm("feedback.read"), QuyenHanChe)
	w = m.goi(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if it := listItems(t, w.Body.Bytes())[maPhieuCanBo]; it.CoverPhoto == nil {
		t.Error("`can-bo` cover missing for a holder of feedback.restricted")
	}
}

func TestListCover_FailureRefusesThePage(t *testing.T) {
	m := dungMayChu(t)
	m.staffPhotos.covers = map[string]bool{"pa-001": true}
	m.staffPhotos.coverErr = errors.New("ảnh phiếu: ghi vết cán bộ xem ảnh cho xã x: audit: write: disk full")
	w := m.goi(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "X-Amz-Signature") || strings.Contains(w.Body.String(), maPhieuThuong) {
		t.Errorf("refused page leaks a link or a code: %s", w.Body.String())
	}
}

func TestListCover_NotConfiguredOrUnattributedServesWithout(t *testing.T) {
	for _, err := range []error{app.ErrUploadNotConfigured, app.ErrPhotoReaderUnattributed} {
		m := dungMayChu(t)
		m.staffPhotos.covers = map[string]bool{"pa-001": true}
		m.staffPhotos.coverErr = err
		w := m.goi(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA))
		doiMa(t, w, http.StatusOK)
		items := listItems(t, w.Body.Bytes())
		if len(items) == 0 {
			t.Fatalf("%v: page empty", err)
		}
		for code, it := range items {
			if it.CoverPhoto != nil {
				t.Errorf("%v: %s has a cover", err, code)
			}
		}
	}
}
