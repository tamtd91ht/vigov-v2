package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// bbFileID is a made-up id of ULID shape — no file anywhere carries it.
const bbFileID = "01JABCDEFGHJKMNPQRSTVWXYZ0"

// bbStaffBody holds every staff shape of ADR 0067 §Sửa đổi 03/10/2026.
const bbStaffBody = `<p>Mở đầu</p>` +
	`<figure><img data-file-id="` + bbFileID + `" alt="Lễ ra quân"><figcaption>Ảnh: <strong>xã</strong></figcaption></figure>` +
	`<blockquote><p>Câu một</p><p>Câu <em>hai</em></p></blockquote>` +
	`<p data-role="byline">Văn phòng UBND xã</p>`

func bbJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// No resolver (this card, before TASK-03): every image block is dropped, fail closed (K7, stop
// condition 4) — quote and byline still leave.
func TestBodyBlocksOutWithoutResolverDropsEveryImage(t *testing.T) {
	got := bbJSON(t, bodyBlocksOut(bbStaffBody, nil))
	want := `[{"kind":"paragraph","runs":[{"text":"Mở đầu"}]},` +
		`{"kind":"quote","paragraphs":[{"runs":[{"text":"Câu một"}]},{"runs":[{"text":"Câu "},{"text":"hai","italic":true}]}]},` +
		`{"kind":"byline","runs":[{"text":"Văn phòng UBND xã"}]}]`
	if got != want {
		t.Errorf("body_blocks =\n%s\nwant\n%s", got, want)
	}
}

func TestBodyBlocksOutImageOnlyWhenResolvedToHTTPS(t *testing.T) {
	const u = "https://cdn.vigov.vn/t_01JA/content-body-image/x.jpg"
	cases := []struct {
		name    string
		resolve bodyImageResolver
		image   bool
	}{
		{"resolved", func(id string) (string, bool) { return u, id == bbFileID }, true},
		{"not resolved", func(string) (string, bool) { return "", false }, false},
		{"resolved to http", func(string) (string, bool) { return "http://cdn.vigov.vn/x.jpg", true }, false},
		{"resolved to empty", func(string) (string, bool) { return "", true }, false},
	}
	for _, tc := range cases {
		out := bodyBlocksOut(bbStaffBody, tc.resolve)
		s := bbJSON(t, out)
		if strings.Contains(s, bbFileID) || strings.Contains(s, "file_id") {
			t.Errorf("%s: the file id reached the public wire: %s", tc.name, s)
		}
		var img *bodyBlockOut
		for i := range out {
			if out[i].Kind == "image" {
				img = &out[i]
			}
		}
		if (img != nil) != tc.image {
			t.Fatalf("%s: image block present = %v, want %v: %s", tc.name, img != nil, tc.image, s)
		}
		if img != nil {
			want := `{"kind":"image","src":"` + u + `","alt":"Lễ ra quân","caption":[{"text":"Ảnh: "},{"text":"xã","bold":true}]}`
			if got := bbJSON(t, img); got != want {
				t.Errorf("%s: image block =\n%s\nwant\n%s", tc.name, got, want)
			}
		}
	}
}

// A body holding nothing but images that do not resolve is no body_blocks at all — the client then
// shows `body`, as for any body without text.
func TestBodyBlocksOutOnlyUnresolvedImagesIsNil(t *testing.T) {
	if got := bodyBlocksOut(`<figure><img data-file-id="`+bbFileID+`"></figure>`, nil); got != nil {
		t.Errorf("got %#v, want nil", got)
	}
}

// A legacy row's raw `<img src>` never becomes an image, whatever the resolver says.
func TestBodyBlocksOutLegacyRawImgIsNoImage(t *testing.T) {
	legacy := `<p>Trước</p><img src="https://anh.example/a.png"><figure><img src="https://anh.example/b.png"></figure>`
	asked := false
	out := bodyBlocksOut(legacy, func(string) (string, bool) { asked = true; return "https://cdn.vigov.vn/x", true })
	for _, b := range out {
		if b.Kind == "image" {
			t.Errorf("raw <img src> became an image block: %+v", b)
		}
	}
	if asked {
		t.Error("the resolver was asked about a row that carries no file id")
	}
}

// End to end on the detail route: the staff shapes leave as structure, no image leaves while nothing
// resolves, and no file id, no HTML reaches the resident.
func TestPublicDetailStaffShapesWithoutResolver(t *testing.T) {
	nd := &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{xaA: {{
		ID: "n-staff", Loai: domain.LoaiThongBao, TieuDe: "Ra quân", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien,
		NoiDung: bbStaffBody,
	}}}}
	w := ckGoi(bannerServer(t, nd, nil), MauTinXa+"/n-staff", ckHostA)
	doiMa(t, w, http.StatusOK)
	ckKhongCoHTML(t, w)
	if strings.Contains(w.Body.String(), bbFileID) {
		t.Errorf("file id on the public wire: %s", w.Body.String())
	}
	var got struct {
		Body       string         `json:"body"`
		BodyBlocks []bodyBlockOut `json:"body_blocks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	kinds := make([]string, 0, len(got.BodyBlocks))
	for _, b := range got.BodyBlocks {
		kinds = append(kinds, b.Kind)
	}
	if k := strings.Join(kinds, ","); k != "paragraph,quote,byline" {
		t.Errorf("kinds = %s, want paragraph,quote,byline", k)
	}
	// K5: the plain body keeps caption, quote and byline text, drops the image.
	if want := "Mở đầu\n\nẢnh: xã\n\nCâu một\n\nCâu hai\n\nVăn phòng UBND xã"; got.Body != want {
		t.Errorf("body = %q, want %q", got.Body, want)
	}
}
