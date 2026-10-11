package http

// Migration 0028 on the wire: the typed video length reaches the use case on create and edit, comes back
// on the staff surface, and is published on commune-news only for a `video`.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-comms/internal/domain"
)

func TestCreatePassesVideoDuration(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goi(t, http.MethodPost, hostA, duongNoiDung,
		`{"type":"video","title":"Hướng dẫn","video_url":"https://video.example/v","video_duration_seconds":312}`, canBo(xaA))
	doiMa(t, w, http.StatusCreated)
	if got := m.ghi.themCuoi.VideoDurationSeconds; got != 312 {
		t.Errorf("video_duration_seconds reached the use case as %d", got)
	}
}

func TestPatchVideoDurationZeroClears(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"video_duration_seconds":0}`,
		canBo(xaA)), http.StatusOK)
	if p := m.ghi.suaCuoi.VideoDurationSeconds; p == nil || *p != 0 {
		t.Errorf("0 must reach the use case as a pointer to 0 (clear): %v", p)
	}
	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"title":"x"}`, canBo(xaA)), http.StatusOK)
	if m.ghi.suaCuoi.VideoDurationSeconds != nil {
		t.Error("an absent key must reach the use case as nil (leave alone)")
	}
}

func TestVideoDurationRefusalsMapTo422(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{domain.ErrVideoDurationInvalid, "invalid_video_duration"},
		{domain.ErrVideoDurationOnlyForVideo, "video_duration_only_for_video"},
	} {
		m := dungMayChuND(t)
		m.capQuyen(xaA, QuyenSuaNoiDung)
		m.ghi.loi = tc.err
		w := m.goi(t, http.MethodPatch, hostA, duongNoiDung+"/nd-001", `{"video_duration_seconds":99999}`, canBo(xaA))
		doiMa(t, w, http.StatusUnprocessableEntity)
		if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
			t.Errorf("%v: body %s", tc.err, w.Body.String())
		}
	}
}

func TestStaffListCarriesVideoDuration(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenDocNoiDung)
	n := noiDungMau()
	n.Loai, n.VideoURL, n.VideoDurationSeconds = domain.LoaiVideo, "https://video.example/v", 312
	m.so.ra.Items = append(m.so.ra.Items, n)
	w := m.goi(t, http.MethodGet, hostA, duongNoiDung, "", canBo(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"video_duration_seconds":312`) {
		t.Errorf("list: %s", w.Body.String())
	}
}

func TestPublicNewsCarriesVideoDurationOnlyOnVideo(t *testing.T) {
	nd, dm, audio, _ := publicAudioFixture()
	items := nd.theoXa[xaA]
	items[0].Loai, items[0].AudioFileID, items[0].AudioDurationSeconds = domain.LoaiVideo, "", 0
	items[0].VideoURL, items[0].VideoDurationSeconds = "https://video.example/v", 312
	nd.theoXa[xaA] = items
	h := newPublicAudioServer(t, nd, dm, audio)

	w := ckGoi(h, MauTinXa, ckHostA)
	got := ckDocTrang(t, w)
	if len(got.Items) != 1 || got.Items[0]["video_duration_seconds"] != float64(312) {
		t.Fatalf("the published video must carry its length: %v", got.Items)
	}

	// A length on a non-video row cannot pass 0028's CHECK; the type gate holds anyway.
	items[0].Loai = domain.LoaiTinTuc
	nd.theoXa[xaA] = items
	got = ckDocTrang(t, ckGoi(h, MauTinXa, ckHostA))
	if _, ok := got.Items[0]["video_duration_seconds"]; ok {
		t.Errorf("a news article must not carry a video length: %v", got.Items[0])
	}
}
