package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// om is a JSON object that marshals in INSERTION order.
//
// encoding/json sorts map keys, which is deterministic but puts `components` before the
// DO-NOT-EDIT notice and scatters `openapi`, `info`, `paths` alphabetically. A generated file
// whose first line is not its warning is a generated file somebody edits by hand. Insertion
// order costs thirty lines and buys a readable, stable document.
type om struct {
	khoa []string
	gt   map[string]any
}

func newOM() *om { return &om{gt: map[string]any{}} }

func (o *om) set(k string, v any) *om {
	if _, co := o.gt[k]; !co {
		o.khoa = append(o.khoa, k)
	}
	o.gt[k] = v
	return o
}

func (o *om) co(k string) bool {
	_, ok := o.gt[k]
	return ok
}

func (o *om) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.khoa {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := machJSON(k, "")
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		vb, err := machJSON(o.gt[k], "")
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// machJSON encodes WITHOUT HTML escaping.
//
// encoding/json escapes <, > and & by default, which is meaningless outside a <script> tag and
// turns the Vietnamese prose in these files into > soup — an arrow in a description should
// stay an arrow for the person reading the generated file. Deterministic either way; this is
// only about it being readable enough that somebody notices when it is wrong.
func machJSON(v any, thut string) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if thut != "" {
		enc.SetIndent("", thut)
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if thut == "" {
		return bytes.TrimRight(b.Bytes(), "\n"), nil
	}
	return b.Bytes(), nil
}

// --- the flat index the work queue reads --------------------------------------------------

type dongBeMat struct {
	Service     string            `json:"service"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	OperationID string            `json:"operation_id"`
	TaskID      string            `json:"task_id"`
	Summary     string            `json:"summary"`
	Screen      string            `json:"screen,omitempty"`
	QuyenKieu   string            `json:"permission_kind"`
	Quyen       string            `json:"permission,omitempty"`
	QuyenLyDo   string            `json:"permission_reason,omitempty"`
	Idem        string            `json:"idempotency"`
	IdemMode    string            `json:"idempotency_on_store_failure,omitempty"`
	IdemLyDo    string            `json:"idempotency_reason,omitempty"`
	Request     string            `json:"request_schema,omitempty"`
	Replies     map[string]string `json:"reply_schemas"`
	File        string            `json:"declared_in"`
}

type beMat struct {
	Canhbao string      `json:"_"`
	MoTa    string      `json:"_description"`
	HopDong string      `json:"_contract"`
	Routes  []dongBeMat `json:"routes"`
}

// --- assembling both documents ------------------------------------------------------------

func dungTaiLieu(tuyens []tuyen, gm *giaiMa) (*om, *beMat, error) {
	bs := moBoSchema(gm)

	// REPLIES FIRST, REQUESTS SECOND — the credential policy depends on it. A type reachable
	// from a reply is judged strictly; one reachable only from a request may carry the
	// password the sign-in form has to send. Building replies first makes "reachable from
	// both" resolve to strict without a second pass.
	replyRef := map[string]string{} // "pkgDir\x00Type" -> component
	requestRef := map[string]string{}

	for _, t := range tuyens {
		for _, r := range t.Replies {
			if r.Kieu == "" {
				continue
			}
			ten, err := bs.refTheoTen(t.pkgDir, r.Kieu, true)
			if err != nil {
				return nil, nil, fmt.Errorf("apidoc: %s: %s %s @reply %d: %w", t.File, t.Method, t.Path, r.Status, err)
			}
			replyRef[t.pkgDir+"\x00"+r.Kieu] = ten
		}
	}
	for _, t := range tuyens {
		if t.Request == "" {
			continue
		}
		ten, err := bs.refTheoTen(t.pkgDir, t.Request, false)
		if err != nil {
			return nil, nil, fmt.Errorf("apidoc: %s: %s %s @request: %w", t.File, t.Method, t.Path, err)
		}
		requestRef[t.pkgDir+"\x00"+t.Request] = ten
	}

	paths := newOM()
	surface := &beMat{
		Canhbao: canhBao,
		MoTa: "Bảng phẳng: route -> service, quyền, màn hình. Dùng để biết API nào đã có; " +
			"hình dạng dữ liệu nằm ở kb/20-contracts/openapi.json.",
		HopDong: "kb/20-contracts/openapi.json",
	}

	// tuyens is already sorted by (path, method), so paths are emitted in a stable order and
	// the two methods on /api/v1/sessions land in one path item.
	for _, t := range tuyens {
		muc, _ := paths.gt[t.Path].(*om)
		if muc == nil {
			muc = newOM()
			paths.set(t.Path, muc)
			if ps := thamSoDuongDan(t.Path); ps != nil {
				muc.set("parameters", ps)
			}
		}

		op := newOM()
		op.set("summary", t.Summary)
		op.set("operationId", maOperation(t))
		op.set("tags", []any{t.Service})
		if t.Screen != "" {
			op.set("x-vigov-screen", t.Screen)
		}
		op.set("x-vigov-permission", quyenJSON(t.Quyen))
		op.set("x-vigov-task", maViec(t.Service, t.Method, t.Path))
		if t.Idem.Kind == "required" {
			op.set("parameters", []any{
				newOM().set("name", "Idempotency-Key").
					set("in", "header").
					set("required", true).
					set("schema", newOM().set("type", "string")),
			})
			op.set("x-vigov-idempotency", idemJSON(t.Idem))
		}
		if t.Request != "" {
			op.set("requestBody", newOM().
				set("required", true).
				set("content", newOM().set("application/json", newOM().
					set("schema", newOM().set("$ref", "#/components/schemas/"+requestRef[t.pkgDir+"\x00"+t.Request])))))
		}

		resp := newOM()
		tra := map[string]string{}
		for _, r := range t.Replies {
			ma := strconv.Itoa(r.Status)
			one := newOM().set("description", moTaTrangThai(r.Status))
			if r.Kieu != "" {
				ref := replyRef[t.pkgDir+"\x00"+r.Kieu]
				one.set("content", newOM().set("application/json", newOM().
					set("schema", newOM().set("$ref", "#/components/schemas/"+ref))))
				tra[ma] = ref
			} else {
				tra[ma] = ""
			}
			resp.set(ma, one)
		}
		op.set("responses", resp)

		muc.set(strings.ToLower(t.Method), op)

		surface.Routes = append(surface.Routes, dongBeMat{
			Service:     t.Service,
			Method:      t.Method,
			Path:        t.Path,
			OperationID: maOperation(t),
			TaskID:      maViec(t.Service, t.Method, t.Path),
			Summary:     t.Summary,
			Screen:      t.Screen,
			QuyenKieu:   t.Quyen.Kind,
			Quyen:       t.Quyen.Key,
			QuyenLyDo:   t.Quyen.LyDo,
			Idem:        t.Idem.Kind,
			IdemMode:    t.Idem.Mode,
			IdemLyDo:    t.Idem.LyDo,
			Request:     requestRef[t.pkgDir+"\x00"+t.Request],
			Replies:     tra,
			File:        t.File,
		})
	}

	schemas := newOM()
	ten := make([]string, 0, len(bs.comps))
	for k := range bs.comps {
		ten = append(ten, k)
	}
	sort.Strings(ten)
	for _, k := range ten {
		schemas.set(k, bs.comps[k])
	}

	doc := newOM()
	doc.set("x-vigov-generated", canhBao)
	doc.set("openapi", "3.1.0")
	doc.set("info", newOM().
		set("title", "ViGov — REST API cho web quản trị").
		set("version", "v1").
		set("description", strings.Join([]string{
			"Bề mặt HTTP mà web quản trị gọi, trích từ khai báo route trong services/*/internal/.",
			"Xã được suy ra từ Host ở rìa ngoài cùng, nên KHÔNG có tenant_id trong đường dẫn hay query (luật 1).",
			"Mỗi operation khai quyền ở x-vigov-permission; thiếu khai báo là từ chối, không phải cho qua (luật 5).",
			"Phiên đi bằng cookie httpOnly do máy chủ đặt; client không tự gắn Authorization.",
		}, " ")))
	doc.set("x-vigov-source", "services/*/internal/ — chú thích @summary/@screen/@request/@reply ngay trên câu lệnh đăng ký route")
	doc.set("paths", paths)
	doc.set("components", newOM().set("schemas", schemas))

	return doc, surface, nil
}

// refTheoTen resolves an annotated type name and emits its component.
func (b *boSchema) refTheoTen(pkgDir, ten string, nghiem bool) (string, error) {
	k, err := b.gm.timKieu(pkgDir, ten)
	if err != nil {
		return "", err
	}
	if k.spec == nil {
		return "", fmt.Errorf("%s không phải kiểu có hình dạng đối tượng", ten)
	}
	return b.refCua(k, nghiem)
}

func quyenJSON(q quyenDecl) *om {
	o := newOM().set("kind", q.Kind)
	if q.Key != "" {
		o.set("key", q.Key)
	}
	if q.LyDo != "" {
		o.set("reason", q.LyDo)
	}
	return o
}

func idemJSON(i idemDecl) *om {
	hieuUng := "Store không tới được -> cho qua và ghi cảnh báo"
	if i.Mode == "DongKhiHong" {
		hieuUng = "Store không tới được -> trả 503"
	}
	return newOM().
		set("required", true).
		set("header", "Idempotency-Key").
		set("on_store_failure", i.Mode).
		set("on_store_failure_effect", hieuUng)
}

// thamSoDuongDan derives the path parameters from the template — {sid} and nothing invented.
func thamSoDuongDan(p string) []any {
	var out []any
	for _, seg := range strings.Split(p, "/") {
		if len(seg) > 2 && strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			ten := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			out = append(out, newOM().
				set("name", ten).
				set("in", "path").
				set("required", true).
				set("schema", newOM().set("type", "string")))
		}
	}
	return out
}

// maOperation is derived mechanically from the method and the path. Deliberately not a
// hand-picked nice name: an operationId is an identifier consumers generate clients from, and
// one person's taste is not a stable contract.
//
//	POST   /api/v1/sessions        -> identity_post_sessions
//	DELETE /api/v1/sessions/{sid}  -> identity_delete_sessions_by_sid
func maOperation(t tuyen) string {
	p := t.Path
	for _, v := range []string{"/api/v1/", "/api/v2/"} {
		p = strings.TrimPrefix(p, v)
	}
	phan := []string{t.Service, strings.ToLower(t.Method)}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			phan = append(phan, "by_"+strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}"))
			continue
		}
		phan = append(phan, strings.ReplaceAll(seg, "-", "_"))
	}
	return strings.Join(phan, "_")
}

// moTaTrangThai uses the standard reason phrase. OpenAPI requires a description on every
// response, and inventing Vietnamese prose per status would be writing a business meaning the
// handler never declared.
func moTaTrangThai(ma int) string {
	if s := http.StatusText(ma); s != "" {
		return s
	}
	return "HTTP " + strconv.Itoa(ma)
}

// vietJSON writes indented JSON with a trailing newline, creating the directory if needed.
func vietJSON(duongDan string, v any) error {
	if err := os.MkdirAll(filepath.Dir(duongDan), 0o755); err != nil {
		return err
	}
	b, err := machJSON(v, "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(duongDan, b, 0o644)
}
