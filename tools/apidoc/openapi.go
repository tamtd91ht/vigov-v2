package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
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
	XaLop       string            `json:"tenant_class,omitempty"`
	XaLyDo      string            `json:"tenant_class_reason,omitempty"`
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
		// Lớp xã CHỈ hiện trên tuyến công dân (ADR 0022). Trên tuyến cán bộ nó không tồn tại —
		// xã ở đó đến từ Host — nên không phát ra một khoá rỗng để người đọc phải đoán nghĩa.
		//
		// ĐÂY LÀ DANH SÁCH `KhongThuocXa` MÀ ADR 0022 YÊU CẦU LIỆT KÊ ĐƯỢC: nó SINH ra kèm lý
		// do, không ai giữ tay (luật 9). Người rà soát lọc `tenant_class` trong tệp này là thấy
		// hết, và cái thấy được không trôi khỏi mã nguồn.
		if t.Xa.Kind != "" {
			op.set("x-vigov-tenant-class", xaJSON(t.Xa))
		}
		op.set("x-vigov-task", maViec(t.Service, t.Method, t.Path))
		// MỘT danh sách `parameters` duy nhất, gom cả hai nguồn.
		//
		// Bản trước `op.set("parameters", ...)` cho Idempotency-Key, nên một tuyến vừa chống
		// lặp vừa phân trang sẽ mất một bên TRONG IM LẶNG — `om.set` ghi đè khoá cũ. Hôm nay
		// chưa tuyến nào có cả hai (chống lặp ở tuyến ghi, phân trang ở tuyến đọc), nên lỗi
		// này chưa có triệu chứng; nó chờ đúng tuyến đầu tiên có cả hai.
		var thamSo []any
		if t.Idem.Kind == "required" {
			thamSo = append(thamSo, newOM().set("name", "Idempotency-Key").
				set("in", "header").
				set("required", true).
				set("schema", newOM().set("type", "string")))
			op.set("x-vigov-idempotency", idemJSON(t.Idem))
		}
		// PHÂN TRANG: `@page` nếu tuyến có khai, NGƯỢC LẠI suy từ chính lời gọi `page.Parse`
		// trong handler. phantrang.go:sapXepSuyTuMa nói vì sao một chú thích viết tay không
		// đóng được lớp lỗi này — năm trong sáu tuyến phân trang của kho đã trôi mất dòng ấy.
		tenSapXep := t.Page
		if tenSapXep == "" {
			suy, err := gm.sapXepSuyTuMa(t.pkgDir, t.Handler)
			if err != nil {
				return nil, nil, fmt.Errorf("apidoc: %s: %s %s: %w", t.File, t.Method, t.Path, err)
			}
			tenSapXep = suy
		}
		if tenSapXep != "" {
			pt, err := gm.docPhanTrang(t.pkgDir, tenSapXep)
			if err != nil {
				return nil, nil, fmt.Errorf("apidoc: %s: %s %s: %w", t.File, t.Method, t.Path, err)
			}
			thamSo = append(thamSo, thamSoPhanTrang(pt)...)
		}
		// THAM SỐ TRUY VẤN ĐỌC THẲNG TỪ HANDLER — truyvan.go nói vì sao nó không phải một chú
		// thích. Gộp vào CÙNG danh sách `parameters`, sau phân trang, nên một tuyến vừa phân
		// trang vừa có tham số riêng không mất bên nào.
		if tv, err := gm.thamSoTruyVanCua(t.pkgDir, t.Handler); err != nil {
			return nil, nil, fmt.Errorf("apidoc: %s: %s %s: %w", t.File, t.Method, t.Path, err)
		} else if len(tv) > 0 {
			// Tên trùng thì GIỮ BẢN ĐÃ CÓ: `@page` mô tả `limit/cursor/sort/order` đầy đủ hơn —
			// kèm enum cột và trần trang — còn hai tham số cùng tên trong một operation là một
			// tài liệu OpenAPI không hợp lệ.
			daCo := map[string]bool{}
			for _, x := range thamSo {
				if o, ok := x.(*om); ok {
					if n, ok := o.gt["name"].(string); ok {
						daCo[n] = true
					}
				}
			}
			var con []thamSoTruyVan
			for _, x := range tv {
				if !daCo[x.Ten] {
					con = append(con, x)
				}
			}
			thamSo = append(thamSo, thamSoTruyVanJSON(con)...)
		}
		if len(thamSo) > 0 {
			op.set("parameters", thamSo)
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
			XaLop:       t.Xa.Kind,
			XaLyDo:      t.Xa.LyDo,
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
			"Bề mặt HTTP mà web quản trị gọi, trích từ khai báo route trong */internal/.",
			"Xã được suy ra từ Host ở rìa ngoài cùng, nên KHÔNG có tenant_id trong đường dẫn hay query (luật 1).",
			"Mỗi operation khai quyền ở x-vigov-permission; thiếu khai báo là từ chối, không phải cho qua (luật 5).",
			"Phiên đi bằng cookie httpOnly do máy chủ đặt; client không tự gắn Authorization.",
		}, " ")))
	doc.set("x-vigov-source", "*/internal/ — chú thích @summary/@screen/@request/@reply ngay trên câu lệnh đăng ký route")
	doc.set("paths", paths)
	doc.set("components", newOM().set("schemas", schemas))

	return doc, surface, nil
}

// refTheoTen resolves an annotated type name and emits its component.
func (b *boSchema) refTheoTen(pkgDir, ten string, nghiem bool) (string, error) {
	// PHÂN TÍCH tên kiểu chứ không cắt chuỗi. `@reply 200 page.Result[canBoTomTat]` là một
	// token không có khoảng trắng, nên bộ đọc chú thích trả nó về nguyên vẹn — nhưng cắt nó
	// tại dấu `.` cho ra `Result[canBoTomTat]`, một cái tên không có thật ở bất kỳ gói nào.
	// parser.ParseExpr đọc được cả ba dạng đang dùng: `canBoTomTat`, `httpx.Error`, và dạng
	// generic — bằng cùng một đường, nên không có dạng thứ tư nào bị xử lý riêng rồi trôi.
	e, err := parser.ParseExpr(ten)
	if err != nil {
		return "", fmt.Errorf("tên kiểu %q không đọc được: %w", ten, err)
	}

	switch t := e.(type) {
	case *ast.IndexExpr:
		k, err := b.kieuTongQuat(t.X, []ast.Expr{t.Index}, kieuGo{pkgDir: pkgDir})
		if err != nil {
			return "", err
		}
		return b.refCua(k, nghiem)
	case *ast.IndexListExpr:
		k, err := b.kieuTongQuat(t.X, t.Indices, kieuGo{pkgDir: pkgDir})
		if err != nil {
			return "", err
		}
		return b.refCua(k, nghiem)
	}

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

// xaJSON mô tả lớp xã bằng đúng từ ngữ của ADR 0022, kèm hệ quả — người rà soát đọc một chỗ
// thấy cả "khai gì" lẫn "điều đó nghĩa là gì trong context".
func xaJSON(x xaDecl) *om {
	o := newOM().set("kind", x.Kind)
	if x.Kind == "khong-thuoc-xa" {
		o.set("tenant_in_context", false)
		o.set("reason", x.LyDo)
		o.set("note", "Chỉ đường phân giải xã. Không có xã trong context nên không chạm được "+
			"dữ liệu nghiệp vụ (ADR 0022).")
		return o
	}
	o.set("tenant_in_context", true)
	o.set("source", "phiên công dân — không bao giờ từ Host, header hay query (luật 1 cấm #2)")
	if x.Kind == "tu-phien-chi-xem" {
		o.set("reason", x.LyDo)
		o.set("note", "Chỉ xem: nhận cả phiên CHƯA xác thực số điện thoại, nên không được đọc hay ghi "+
			"hồ sơ của chính công dân (ADR 0045).")
		return o
	}
	o.set("phone_verified_required", true)
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
