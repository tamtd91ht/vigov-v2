package http

// `image_url` on GET /api/v1/commune-news and /{id} (ADR 0047 §6 (1), 2026-10-01): the PUBLISHED
// derivative's public URL, for published items only, resolved inside the commune the host names.

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

const publicCoverURL = "https://media.example/vigov-prod-public/public-media/t_x/2026/10/comms/content-image/y/thumb-1280.jpg"

func newPublicCoverServer(t *testing.T, nd *ckNoiDung, dm *ckDanhMuc, covers *fakePublicCovers) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Limiter: ckLimiter(), Xa: &ckNenTang{}, NoiDung: nd, DanhMuc: dm, CoverImages: covers, Audio: &fakePublicAudio{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return mux
}

func publicCoverFixture() (*ckNoiDung, *ckDanhMuc, *fakePublicCovers) {
	nd, dm := ckDuLieu()
	items := nd.theoXa[xaA]
	items[0].CoverImageFileID = "FILEPUBLISHED"
	// A DRAFT carrying a cover: its id must never even be asked about on the public surface.
	items[1].CoverImageFileID = "FILEDRAFT"
	nd.theoXa[xaA] = items
	covers := &fakePublicCovers{byTenant: map[tenant.ID]map[string]string{
		xaA: {"FILEPUBLISHED": publicCoverURL, "FILEDRAFT": "https://media.example/draft.jpg"},
	}}
	return nd, dm, covers
}

func TestPublicNewsImageURLOnlyForPublishedItems(t *testing.T) {
	nd, dm, covers := publicCoverFixture()
	h := newPublicCoverServer(t, nd, dm, covers)

	w := ckGoi(h, MauTinXa, ckHostA)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var got ckTrang
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0]["image_url"] != publicCoverURL {
		t.Fatalf("the published item must carry its public derivative URL: %v", got.Items)
	}
	for _, id := range covers.asked {
		if id == "FILEDRAFT" {
			t.Fatal("an UNPUBLISHED item's cover was resolved on the public surface")
		}
	}
	if covers.tenantID != xaA {
		t.Errorf("covers resolved in commune %q, want %q", covers.tenantID, xaA)
	}
}

func TestPublicNewsDetailCarriesImageURLAndOmitsWhenNoPublicCopy(t *testing.T) {
	nd, dm, covers := publicCoverFixture()
	h := newPublicCoverServer(t, nd, dm, covers)

	w := ckGoi(h, MauTinXa+"/nd-a-1", ckHostA)
	var one map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &one); err != nil || w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	if one["image_url"] != publicCoverURL {
		t.Errorf("detail must carry image_url: %v", one["image_url"])
	}

	// No public copy recorded (not published yet, or a withdrawal pending): the key is ABSENT.
	covers.byTenant[xaA] = map[string]string{}
	w = ckGoi(h, MauTinXa+"/nd-a-1", ckHostA)
	one = map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &one)
	if _, ok := one["image_url"]; ok {
		t.Errorf("no public copy, yet image_url is present: %s", w.Body.String())
	}
}

func TestPublicNewsImageReadFailureIs500(t *testing.T) {
	nd, dm, covers := publicCoverFixture()
	covers.err = errors.New("store down")
	h := newPublicCoverServer(t, nd, dm, covers)
	if w := ckGoi(h, MauTinXa, ckHostA); w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestCoverIDsSkipsUnpublishedAndDuplicates(t *testing.T) {
	ids := coverIDs([]domain.NoiDungMiniApp{
		{CoverImageFileID: "A", TrangThai: domain.TrangThaiDangHien},
		{CoverImageFileID: "A", TrangThai: domain.TrangThaiDangHien},
		{CoverImageFileID: "B", TrangThai: domain.TrangThaiAn},
		{CoverImageFileID: "C", TrangThai: domain.TrangThaiChoDuyet},
		{TrangThai: domain.TrangThaiDangHien},
	})
	if len(ids) != 1 || ids[0] != "A" {
		t.Errorf("coverIDs = %v, want [A]", ids)
	}
}
