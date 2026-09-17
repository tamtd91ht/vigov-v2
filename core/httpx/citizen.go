package httpx

import (
	"context"
	"net/http"
	"strings"

	"github.com/vihat/vigov/core/tenant"
)

// --- Rìa HTTP của kênh công dân — ADR 0022 --------------------------------------------------
//
// MỘT RÌA RIÊNG, KHÔNG PHẢI MỘT NHÁNH TRONG TenantMiddleware.
//
// Mini App không có tên miền: nó gọi MỘT API host duy nhất và host đó không ứng với xã nào
// (ADR 0005). Chạy TenantMiddleware trên đường ấy thì mọi yêu cầu của mọi công dân đều 404.
// Bỏ nó đi mà không thay bằng gì thì store.For(ctx) panic — hoặc tệ hơn, có người "sửa" bằng
// cách nhận xã từ client, thứ luật 1 cấm #2.
//
// Nên: xã đến từ PHIÊN CÔNG DÂN do máy chủ phát hành. Không có đường thứ ba.
//
// LUẬT 1 BẤT BIẾN 8 KHÔNG CÓ ĐỐI ỨNG Ở ĐÂY, và đó là kết luận đã chốt chứ không phải chỗ còn
// thiếu. Bất biến ấy là: token mang tenant_id, mọi yêu cầu đối chiếu nó với xã suy từ `Host`,
// lệch thì 401 + báo động. Ở rìa này KHÔNG CÓ xã suy từ `Host` để mà đối chiếu — phiên là
// nguồn duy nhất, và một nguồn duy nhất không tự đối chiếu với chính nó được.
//
// Điều đó KHÔNG được "sửa" bằng cách đối chiếu phiên với bất cứ thứ gì client gửi lên (query
// `t=`, một header, một trường trong thân): làm thế là dựng lại đúng cái cửa luật 1 cấm #2 đã
// đóng, dưới dạng một phép kiểm tra trông có vẻ nghiêm ngặt hơn. Phép đối chiếu tương đương
// nằm TRONG LUỒNG chứ không ở rìa — ADR 0019, bất biến 8: xã của mã ghép phải khớp xã của
// phiên công dân.

// CitizenSession is what the edge learns about one citizen request. Three opaque identifiers,
// and there is no fourth field.
//
// VÌ SAO KHÔNG CÓ SỐ ĐIỆN THOẠI, TÊN, HAY BẤT KỲ DỮ LIỆU CÁ NHÂN NÀO: công dân được định danh
// trong nghiệp vụ bằng số điện thoại (ADR 0020), và số điện thoại là dữ liệu cá nhân theo Nghị
// định 13/2023 (luật 3). Rìa này chạy trên MỌI yêu cầu của kênh công dân — nó là chỗ dễ lọt
// vào log nhất trong cả hệ thống, vì một dòng gỡ lỗi ở đây in ra số điện thoại của cả xã. Rìa
// không cần biết công dân là ai, chỉ cần biết YÊU CẦU NÀY thuộc xã nào và phiên nào; nên kiểu
// dữ liệu này không mang nổi dữ liệu cá nhân, chứ không phải chỉ "không nên mang".
type CitizenSession struct {
	// ID là định danh phiên (sid) — cùng vai trò với Claims.Sid của cán bộ: nó là thứ làm phiên
	// THU HỒI ĐƯỢC (luật 5 bất biến 4). Dùng để ghi vết, không bao giờ để quyết định quyền.
	ID string

	// CitizenID là định danh mờ của công dân, do máy chủ phát, không suy ra được số điện thoại.
	// Nó tồn tại để luật 4 bất biến 2 thực hiện được: danh tính công dân đến TỪ PHIÊN, không
	// bao giờ từ tham số yêu cầu.
	CitizenID string

	// TenantID là xã của phiên.
	//
	// RỖNG LÀ MỘT CÂU TRẢ LỜI THẬT, KHÔNG PHẢI LỖI: ADR 0005 tách ba lớp, và ở lớp "khám phá"
	// công dân đã có phiên nhưng CHƯA chọn xã nào — đó chính là lúc màn hình chọn xã gọi lên.
	// Tuyến nghiệp vụ gặp giá trị rỗng thì XaTuPhien trả 401; không có mặc định nào được điền
	// vào đây (luật 1 cấm #1).
	TenantID tenant.ID
}

// CitizenSessions resolves a bearer token to the session the server issued for it.
//
// MỘT INTERFACE KHAI TRONG core, CÀI ĐẶT NẰM Ở DỊCH VỤ — đúng khuôn tenant.Directory. core
// không được biết phiên công dân lưu ở bảng nào, ký bằng gì, hết hạn ra sao: đó là quyết định
// của ADR 0020 và của dịch vụ sở hữu phiên, và nhét nó vào đây là buộc mọi dịch vụ dùng rìa
// này phải mang theo cách lưu trữ ấy.
//
// TraCuu TRẢ VỀ ok=false CHO MỌI TRƯỜNG HỢP KHÔNG DÙNG ĐƯỢC — token sai chữ ký, hết hạn, phiên
// đã thu hồi, phiên của một xã đã ngừng hoạt động. Một câu trả lời cho mọi trường hợp: phân
// biệt chúng ra phía ngoài là nói cho người đang dò biết họ dò tới đâu.
//
// ĐÂY LÀ ĐƯỜNG NÓNG: nó chạy trên mọi yêu cầu của mọi công dân, giống hệt tenant.Directory ở
// đường cán bộ. Cài đặt phải xử lý nó như đường nóng ngay từ đầu (đệm có TTL ngắn, vô hiệu khi
// thu hồi), chứ không phải sau khi đã chậm.
type CitizenSessions interface {
	// TraCuu returns the session for a bearer token, or ok=false.
	TraCuu(ctx context.Context, token string) (CitizenSession, bool)
}

type phienCongDanKey struct{}

// trangThaiLop ghi nhận một tuyến ĐÃ KHAI lớp xã hay chưa. Con trỏ, vì middleware lớp xã chạy
// BÊN TRONG rìa: rìa không thể biết trước tuyến này khai gì, nó chỉ biết được sau khi đã gọi
// xuống. Xem chanChuaKhaiLop để biết vì sao "biết sau" vẫn kịp.
type trangThaiLop struct{ daKhai bool }

type lopXaKey struct{}

// CitizenEdge resolves the commune of a citizen request from the session, never from the client.
//
// Nó KHÔNG tự đặt xã vào context. Việc đó chỉ XaTuPhien làm, và đó là điều làm nên bức tường
// thứ nhất của ADR 0022: một handler KhongThuocXa không có xã trong context, nên store.For(ctx)
// panic ngay lần chạy đầu — bức tường ấy là cơ chế, không phải quy ước ai đó phải nhớ.
//
// Nó cũng KHÔNG từ chối yêu cầu không có phiên: lớp KhongThuocXa hợp lệ khi chưa biết xã, và
// rìa chưa biết tuyến này thuộc lớp nào. Trục danh tính do authz.CitizenOnly trả lời, trục xã
// do lớp xã trả lời — hai trục vuông góc, mỗi trục từ chối phần của mình.
//
// PHIÊN ĐỰNG Ở `Authorization`, KHÔNG PHẢI COOKIE. Một host phục vụ mọi xã nghĩa là một cookie
// đặt ở đó được gửi kèm lưu lượng của MỌI xã theo đúng cấu tạo — chính hình dạng luật 1 cấm #3.
// Rìa này không đọc cookie, và không có chỗ nào để thêm vào.
func CitizenEdge(so CitizenSessions) func(http.Handler) http.Handler {
	if so == nil {
		// Không có sổ phiên thì không phân giải được xã cho bất kỳ tuyến nghiệp vụ nào. Dừng
		// lúc dựng máy chủ, chứ không phải 401 hàng loạt lúc chạy mà không ai hiểu vì sao.
		panic("httpx: CitizenEdge cần một CitizenSessions")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// StripTenantHeaders GỌI NGAY TRONG RÌA, không để người lắp máy chủ nhớ mắc thêm.
			// Ở đây nó nặng hơn đường cán bộ: rìa này không có `Host` để đối chiếu, nên một
			// header xã do client gửi lọt qua thì KHÔNG có gì mâu thuẫn với nó — không có phép
			// kiểm nào sau đó bắt được.
			goHeaderXa(r)

			ctx := r.Context()
			if tok, ok := tokenBearer(r.Header.Get("Authorization")); ok {
				// Token KHÔNG BAO GIỜ được ghi log, kể cả khi tra cứu hỏng: nó là thứ thay thế
				// được cho cả phiên.
				if p, ok := so.TraCuu(ctx, tok); ok {
					ctx = context.WithValue(ctx, phienCongDanKey{}, p)
				}
			}

			lop := &trangThaiLop{}
			ctx = context.WithValue(ctx, lopXaKey{}, lop)

			cw := &chanChuaKhaiLop{ResponseWriter: w, lop: lop}
			next.ServeHTTP(cw, r.WithContext(ctx))
			cw.ketThuc()
		})
	}
}

// CitizenSessionFrom reports the session the citizen edge resolved, if any.
//
// ok=false nghĩa là yêu cầu không mang phiên dùng được — KHÔNG phải "chưa chọn xã". Hai tình
// huống ấy khác nhau: phiên có mà TenantID rỗng là công dân đã đăng nhập nhưng chưa chọn xã.
func CitizenSessionFrom(ctx context.Context) (CitizenSession, bool) {
	p, ok := ctx.Value(phienCongDanKey{}).(CitizenSession)
	return p, ok
}

// XaTuPhien declares that this route belongs to ONE commune, taken from the citizen session.
//
// Đây là lớp của MỌI tuyến nghiệp vụ. Không có phiên, hoặc phiên chưa gắn xã nào, thì 401 —
// không có mặc định, không có "xã gần nhất", không có suy đoán từ bất cứ thứ gì client gửi.
func XaTuPhien() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ghiNhanDaKhaiLop(ctx)

			p, ok := CitizenSessionFrom(ctx)
			if !ok || !p.TenantID.Valid() {
				// MỘT câu trả lời cho cả ba trường hợp — không có phiên, phiên không dùng
				// được, phiên chưa gắn xã. Tách ra là nói cho người đang dò biết họ đang ở
				// bước nào.
				WriteError(w, http.StatusUnauthorized, "unauthorized",
					"Phiên không hợp lệ hoặc đã kết thúc. Vui lòng mở lại ứng dụng.", "")
				return
			}
			// tenant.Into, KHÔNG PHẢI IntoFull — và đó là điều đúng, không phải thiếu sót.
			//
			// Rìa này không phân giải `Host` nên nó KHÔNG BIẾT TÊN XÃ, chỉ biết tenant_id từ
			// phiên. Hệ quả: tenant.MustCurrent PANIC trong handler công dân. Đó là ý đồ. Ép
			// một Tenant vào đây thì phải bịa ra một bản ghi rỗng tên, và một tên rỗng đi tới
			// màn hình của một cơ quan nhà nước thì tệ hơn hẳn một lỗi.
			//
			// Tên xã hiện trên màn hình Mini App là dữ liệu app lấy bằng một lời gọi riêng và
			// giữ lại, không phải thứ rìa gắn kèm vào mọi yêu cầu.
			next.ServeHTTP(w, r.WithContext(tenant.Into(ctx, p.TenantID)))
		})
	}
}

// KhongThuocXa declares that this route belongs to NO commune. The reason is mandatory.
//
// CHỈ DÙNG CHO ĐƯỜNG PHÂN GIẢI XÃ: danh mục xã để công dân chọn, tra alias của QR xã đã sáp
// nhập. Một endpoint lớp này KHÔNG chạm được dữ liệu nghiệp vụ, và điều đó được giữ bằng hai
// bức tường độc lập (ADR 0022):
//
//  1. Không có xã trong context — store.For(ctx) gọi tenant.MustFrom, tức panic
//     (core/store/scoped.go). Không có API nào bỏ qua xã
//  2. Thứ duy nhất với tới được là sổ đăng ký xã, qua platformclient sang dịch vụ platform, và
//     ADR 0003 giữ dịch vụ ấy ở mức siêu dữ liệu
//
// Nếu một thiết kế cần endpoint vừa "không cần xã" vừa đọc dữ liệu của xã thì thiết kế ấy sai:
// đó là ĐIỀU KIỆN DỪNG, không phải chỗ để nới một trong hai tường.
//
// LÝ DO LÀ BẮT BUỘC VÀ PHẢI CỤ THỂ, đúng kỷ luật authz.Public(reason) — luật 5 cấm #4: một
// miễn trừ không có lý do là thứ sáu tháng sau không ai dám gỡ. "để hiển thị" không phải lý do.
func KhongThuocXa(lyDo string) func(http.Handler) http.Handler {
	if strings.TrimSpace(lyDo) == "" {
		panic("httpx: KhongThuocXa cần một lý do cụ thể")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ghiNhanDaKhaiLop(r.Context())
			// KHÔNG gọi tenant.Into, kể cả khi phiên có xã. Một tuyến khai "không thuộc xã
			// nào" mà vẫn thấy xã trong context là một tuyến có thể lặng lẽ đọc dữ liệu xã.
			next.ServeHTTP(w, r)
		})
	}
}

func ghiNhanDaKhaiLop(ctx context.Context) {
	if lop, ok := ctx.Value(lopXaKey{}).(*trangThaiLop); ok {
		lop.daKhai = true
	}
}

// chanChuaKhaiLop refuses to let a route that declared no commune class answer anything.
//
// VÌ SAO CẦN NÓ KHI apidoc ĐÃ TỪ CHỐI LÚC DỰNG: apidoc chỉ đọc được những tệp nó quét, và một
// tuyến đăng ký ngoài cây ấy (một handler lắp thẳng, một bộ định tuyến khác) lọt qua bộ sinh mà
// vẫn phục vụ thật. Thiếu khai là DENY chứ không phải allow (luật 5 bất biến 2), nên nó phải
// còn đúng cả khi bộ sinh không nhìn thấy tuyến đó.
//
// Chặn Ở CHỖ GHI chứ không đệm cả phản hồi: middleware lớp xã chạy bên trong rìa, nên rìa chỉ
// biết tuyến có khai hay không SAU khi đã gọi xuống — nhưng vẫn TRƯỚC byte đầu tiên handler
// ghi ra, vì handler chạy sau middleware lớp. Một phép so con trỏ mỗi lần ghi, không phải một
// bản sao phản hồi.
//
// NÓ CHỈ CHẶN PHẢN HỒI THÀNH CÔNG (2xx), và ranh giới ấy không tuỳ tiện. authz.CitizenOnly nằm
// NGOÀI lớp xã (ADR 0022 §khai trong cùng câu lệnh route), nên một yêu cầu không có phiên bị
// từ chối TRƯỚC khi middleware lớp kịp chạy. Chặn luôn cả phản hồi ấy thì mọi 401 của kênh
// công dân biến thành 500 — che mất câu trả lời đúng bằng một lỗi máy chủ. Thứ phải chặn là
// một tuyến chưa khai xã nào mà vẫn TRẢ LỜI THÀNH CÔNG, vì đó là lúc dữ liệu đi ra.
//
// Nó cố tình không chuyển tiếp http.Flusher: kênh công dân hôm nay không có tuyến truyền dòng
// nào, và một wrapper tự nhận là Flusher trong khi bên dưới không phải là một lỗi khó thấy hơn.
type chanChuaKhaiLop struct {
	http.ResponseWriter
	lop    *trangThaiLop
	daChan bool // đã chặn và tự trả lỗi
	daCho  bool // đã cho một phản hồi đi qua
}

func (w *chanChuaKhaiLop) WriteHeader(ma int) {
	if w.daChan {
		return
	}
	if !w.lop.daKhai && ma >= 200 && ma < 300 {
		w.tuChoi()
		return
	}
	w.daCho = true
	w.ResponseWriter.WriteHeader(ma)
}

func (w *chanChuaKhaiLop) Write(b []byte) (int, error) {
	if w.daChan {
		// Báo "đã ghi" cho handler: nó không sửa được lỗi này, và một lỗi ghi trả về sẽ thành
		// một vòng thử lại hoặc một dòng log chứa đúng thứ vừa bị chặn.
		return len(b), nil
	}
	if !w.daCho && !w.lop.daKhai {
		// Ghi thẳng không kèm WriteHeader là 200 ngầm định.
		w.tuChoi()
		return len(b), nil
	}
	w.daCho = true
	return w.ResponseWriter.Write(b)
}

// ketThuc bắt trường hợp handler không ghi gì cả — net/http khi ấy trả 200 rỗng, tức một tuyến
// chưa khai lớp vẫn trả lời thành công.
func (w *chanChuaKhaiLop) ketThuc() {
	if !w.lop.daKhai && !w.daCho && !w.daChan {
		w.tuChoi()
	}
}

func (w *chanChuaKhaiLop) tuChoi() {
	if w.daChan {
		return
	}
	w.daChan = true
	// 500 chứ không phải 403: đây là lỗi lắp ráp máy chủ, không phải lỗi của người gọi. Thông
	// điệp là thông điệp chung — công dân không có gì để làm với chi tiết bên trong (luật 3).
	WriteError(w.ResponseWriter, http.StatusInternalServerError, "lop_xa_chua_khai",
		"Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// tokenBearer tách token khỏi `Authorization: Bearer <token>`.
//
// Chỉ một khuôn duy nhất được chấp nhận. Không đọc cookie, không đọc query, không đọc thân yêu
// cầu: mỗi chỗ nhận thêm là một chỗ token bị ghi vào log truy cập của một proxy nào đó.
func tokenBearer(h string) (string, bool) {
	const tien = "bearer "
	if len(h) <= len(tien) || !strings.EqualFold(h[:len(tien)], tien) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(tien):])
	return tok, tok != ""
}
