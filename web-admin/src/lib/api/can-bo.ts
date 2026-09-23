/**
 * Gọi bảy tuyến của danh bạ cán bộ — hai tuyến đọc và **năm tuyến ghi**.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `page_Result_identity_canBoTomTat` và
 * `identity_canBoTomTat` đến từ `schema.gen.ts`. Không tệp nào trong ứng dụng này mô tả lại
 * mười ba trường của một cán bộ — có bản thứ hai là có hai bản sẽ trôi (luật 9).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * NĂM TUYẾN GHI, KHÔNG PHẢI MỘT — và sự tách ấy là của máy chủ, tệp này chỉ theo đúng nó
 * (`service-identity/internal/http/can_bo_ghi.go`, chú thích đầu tệp):
 *
 *   POST   /api/v1/staff                 thêm một dòng danh bạ
 *   PATCH  /api/v1/staff/{id}            sửa hồ sơ — KHÔNG đổi thẩm quyền của ai
 *   POST   /api/v1/staff/{id}/lockout    nghỉ hưu / chuyển công tác (#10)
 *   DELETE /api/v1/staff/{id}/lockout    quay lại làm việc
 *   PUT    /api/v1/staff/{id}/role       chuyển vai trò (#13, #14)
 *
 * Gộp lại thành một hàm `luuCanBo(...)` ở đây là dựng lại đúng thứ máy chủ vừa tách ra: một
 * lời gọi mang cả "sửa số điện thoại" lẫn "đưa người này vào vai trò điều hành xã" thì vết
 * kiểm toán chỉ còn một động từ, và một cuộc thanh tra phải tự đoán ra ý định từ delta.
 *
 * KHÔNG CÓ TUYẾN XOÁ, VÀ SỰ VẮNG MẶT ẤY LÀ MỘT PHÁT HIỆN CHỨ KHÔNG PHẢI MỘT THIẾU SÓT. Câu mở
 * #10 chốt XOÁ MỀM là thao tác riêng, mang quyền riêng; bảng `quyen` không có khoá nào nghĩa
 * là "xoá một dòng danh bạ nhập trùng", và dùng tạm `admin.user` cho nó chính là hình dạng #10
 * vừa từ chối — một quyền cho hai việc. Đây là phát hiện cho câu mở #27, không phải một dòng
 * `INSERT INTO quyen` (luật 5, bất biến 3c). Vì vậy tệp này không có `xoaCanBo`, và màn hình
 * không có nút Xoá.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./goi";
import type {
  identity_canBoTomTat,
  identity_datVaiTroVao,
  identity_delete_staff_by_id_lockout,
  identity_get_staff,
  identity_get_staff_by_id,
  identity_patch_staff_by_id,
  identity_post_staff,
  identity_post_staff_by_id_lockout,
  identity_put_staff_by_id_role,
  identity_suaCanBoVao,
  identity_themCanBoVao,
  page_Result_identity_canBoTomTat,
} from "./schema.gen";

/**
 * Khoá sắp xếp và chiều — **suy từ hợp đồng, không gõ tay**.
 *
 * Trước 2026-09-17 đây là một hằng `["code", "created_at"] as const` gõ tay, và là chỗ DUY
 * NHẤT trong cả ứng dụng chép một phần hợp đồng. Nguồn sự thật nằm cách đó hai module: danh
 * sách trắng `page.NewAllowlist` trong `service-identity/internal/store/can_bo_danh_sach.go`.
 * Một bản chép như thế không làm đỏ bài test nào lúc nó trôi — thêm một cột sắp xếp ở máy chủ
 * thì màn hình này không biết, gỡ một cột đi thì nó vẫn gửi và nhận 400, mà cán bộ chỉ thấy
 * "không tải được danh sách".
 *
 * Nay `tools/apidoc` đọc chính biến Go ấy (chú thích `@page idstore.SapXepCanBo`) và phát
 * `sort` kèm enum vào `openapi.json`, nên hai kiểu dưới đây lấy thẳng từ kiểu đã sinh. Thêm
 * hoặc bớt một cột ở máy chủ là `KhoaSapXep` đổi theo, và mọi chỗ dùng sai sẽ đỏ ở `tsc`.
 *
 * Sáu cột khác mà đặc tả vẽ mũi tên ⇅ lên (`docs/ui-ux/14-cau-hinh.md §3`) CỐ Ý không sắp xếp
 * được, mỗi cột một lý do đã ghi ngay trên danh sách trắng: họ tên và điện thoại là dữ liệu cá
 * nhân mà khoá sắp xếp thì đi vào URL, log truy cập và lịch sử trình duyệt (luật 3, cấm #4);
 * `dang_nhap_gan_nhat` cho phép NULL nên mọi người chưa từng đăng nhập sẽ rơi khỏi mọi trang
 * sau trang đầu — lặng lẽ.
 */
type TruyVanDanhSach = identity_get_staff["truyVan"];

/** Khoá sắp xếp máy chủ nhận. `NonNullable` vì tham số là tuỳ chọn trong hợp đồng. */
export type KhoaSapXep = NonNullable<TruyVanDanhSach["sort"]>;

/** Chiều sắp xếp. Máy chủ mặc định `asc` khi không gửi. */
export type ChieuSapXep = NonNullable<TruyVanDanhSach["order"]>;

/**
 * Danh sách khoá để dựng điều khiển sắp xếp trên giao diện.
 *
 * TypeScript xoá kiểu lúc chạy, nên một hợp của chuỗi hằng không tự liệt kê ra được — mảng này
 * là bản liệt kê ấy. Nó KHÔNG phải nguồn sự thật thứ hai, và HAI phép kiểm dưới đây giữ đúng
 * điều đó, mỗi phép bắt một chiều:
 *
 *   `satisfies` bắt khoá THỪA — một khoá máy chủ không nhận lọt vào mảng.
 *   `_duKhoa`  bắt khoá THIẾU — máy chủ thêm một cột mà mảng không có.
 *
 * Chỉ một trong hai là nửa vời, và nửa bị bỏ lại chính là nửa im lặng: một khoá thừa hỏng ngay
 * khi người dùng bấm (400), còn một khoá thiếu thì không ai thấy — cột ấy đơn giản không có
 * mũi tên, và không ai biết nó lẽ ra phải có.
 */
export const KHOA_SAP_XEP = ["code", "created_at"] as const satisfies readonly KhoaSapXep[];

/** Mọi khoá trong hợp đồng đều phải có mặt ở KHOA_SAP_XEP. Thiếu một khoá → `tsc` đỏ tại đây. */
type DuKhoaSapXep =
  Exclude<KhoaSapXep, (typeof KHOA_SAP_XEP)[number]> extends never ? true : never;
const _duKhoaSapXep: DuKhoaSapXep = true;
void _duKhoaSapXep;

/**
 * Một yêu cầu trang.
 *
 * `cursor` là **mờ đục**: một chuỗi máy chủ phát ra, client chỉ chuyền lại nguyên văn. Nó không
 * phải số thứ tự trang, và ứng dụng này không có chỗ nào dựng nên một con trỏ — dựng được con
 * trỏ là chọn được chỗ bắt đầu đọc.
 *
 * KHÔNG CÓ `offset`, và cũng không có `page`. Máy chủ đọc theo mốc (keyset) và CỐ Ý không đếm
 * tổng số: một `COUNT(*)` trên bảng đã phân mảnh 32 phần là cái giá phải trả trên mọi lần mở
 * màn hình, nên hợp đồng không trả `total` và giao diện không được bịa ra một con số nó không
 * có (`core/page/page.go`, chú thích đầu gói).
 */
export type ThamSoTrang = {
  limit?: number;
  sort?: KhoaSapXep;
  order?: ChieuSapXep;
  cursor?: string | null;
};

/**
 * Dựng đường dẫn truy vấn. Tách riêng khỏi lời gọi mạng để test được mà không cần thay `fetch`.
 *
 * Tham số nào không đặt thì KHÔNG xuất hiện trong URL — để máy chủ áp mặc định của nó
 * (`limit=20`, `sort=code`, `order=asc`), thay vì ứng dụng web giữ một bản sao thứ hai của các
 * mặc định ấy và trôi khỏi bản của máy chủ.
 */
export function duongDanDanhSachCanBo(thamSo: ThamSoTrang = {}): string {
  const duongDan: identity_get_staff["duongDan"] = "/api/v1/staff";
  const truyVan = new URLSearchParams();

  if (thamSo.limit !== undefined) truyVan.set("limit", String(thamSo.limit));
  if (thamSo.sort !== undefined) truyVan.set("sort", thamSo.sort);
  if (thamSo.order !== undefined) truyVan.set("order", thamSo.order);
  // Con trỏ rỗng nghĩa là trang đầu. Gửi `cursor=` rỗng thì máy chủ trả 400 "con trỏ không hợp
  // lệ" — đúng vào lần mở màn hình đầu tiên.
  if (thamSo.cursor !== undefined && thamSo.cursor !== null && thamSo.cursor !== "") {
    truyVan.set("cursor", thamSo.cursor);
  }

  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/** GET /api/v1/staff — một trang của danh bạ. Máy chủ đòi quyền `admin.user`; 403 nếu thiếu. */
export function layDanhSachCanBo(
  thamSo: ThamSoTrang = {},
): Promise<KetQua<page_Result_identity_canBoTomTat>> {
  return docJSON<page_Result_identity_canBoTomTat>(duongDanDanhSachCanBo(thamSo));
}

/**
 * GET /api/v1/staff/{id} — một cán bộ.
 *
 * `id` được mã hoá vào đường dẫn, không ghép thẳng. 404 ở đây là **một** câu trả lời cho ba
 * tình huống — id bịa ra, người đã bị xoá mềm, và cán bộ của xã khác — nên không tình huống nào
 * phân biệt được bằng cách thử (luật 4, cấm #2). Giao diện hiện đúng `message` của máy chủ chứ
 * không diễn giải thêm.
 */
export function layChiTietCanBo(id: string): Promise<KetQua<identity_canBoTomTat>> {
  const thamSo: identity_get_staff_by_id["thamSo"] = { id };
  return docJSON<identity_canBoTomTat>(`/api/v1/staff/${encodeURIComponent(thamSo.id)}`);
}

/* ---- năm tuyến ghi ------------------------------------------------------------------------- */

/**
 * Gửi một yêu cầu GHI của danh bạ và đọc `identity_canBoTomTat` trả về.
 *
 * NĂM TUYẾN ĐỀU TRẢ VỀ NGUYÊN DÒNG DANH BẠ SAU KHI GHI — 201 cho `POST /staff`, 200 cho bốn
 * tuyến còn lại, kể cả `DELETE .../lockout`. Đó là lý do hàm này có một kiểu trả về duy nhất:
 * màn hình vẽ lại đúng dòng vừa đổi bằng chính câu trả lời của lần ghi, không phải bằng một lần
 * đọc thứ hai — mà một lần đọc thứ hai có thể trả về dòng người khác vừa sửa, và đọc ra như thể
 * lần ghi vừa rồi đã làm một việc nó không làm (`can_bo_ghi.go`, chú thích trên `datKhoaCanBo`).
 *
 * KHÔNG RẼ NHÁNH THEO `code`, KHÔNG HIỆN `trace_id`, KHÔNG HIỆN SỐ HIỆU HTTP. `thongBaoLoi` lấy
 * đúng `message` máy chủ viết, và với các tuyến này đó là toàn bộ điểm: ba quy tắc khách chốt
 * 22/09/2026 đi tới người dùng NGUYÊN VĂN câu của máy chủ —
 *
 *   #13 → 409 "Xã phải luôn còn ít nhất một người quản trị. Hãy cấp quyền quản trị cho một cán
 *         bộ khác trước, rồi thực hiện lại thao tác này."
 *   #14 → 403 "Không thao tác được lên chính tài khoản của mình…" và 403 "Vai trò này mang
 *         quyền mà tài khoản của bạn không có…" — HAI câu khác nhau cho HAI ràng buộc khác nhau.
 *
 * Viết lại ba câu ấy ở client là dựng bản sao thứ hai của một quy tắc nghiệp vụ, và bản sao ấy
 * trôi mà không bài test nào đỏ (luật 9, cấm #2). Việc của màn hình là ĐƯA CÂU ẤY RA TRANG.
 *
 * KHÔNG GHI LOG GÌ KHI MẠNG HỎNG: thân yêu cầu mang họ tên, thư điện tử và hai số điện thoại của
 * một cán bộ (luật 3, bất biến 1).
 *
 * `than === undefined` THÌ KHÔNG CÓ THÂN VÀ KHÔNG CÓ `Content-Type`. Hai tuyến khoá/mở khoá cố ý
 * không nhận thân nào (`can_bo_ghi.go`: lược đồ không có cột nào giữ lý do, nên một ô lý do ở đây
 * sẽ rơi vào vết kiểm toán mà không màn hình nào đọc lại được). Gửi `{}` kèm `Content-Type` là
 * tuyên bố có một thân — thứ sẽ mời người sau điền vào.
 *
 * CÁCH GỌI MẠNG NẰM Ở `goi.ts`, MÓN NỢ ĐÃ TRẢ. Trước 2026-09-22 hàm này tự dựng lấy `fetch` — bản
 * thứ hai của một hình dạng `lib/api/danh-muc.ts` đã viết, và chú thích của chính bản ấy đặt sẵn
 * điều kiện: *"màn hình ghi thứ hai xuất hiện là lúc hàm này chuyển sang `goi.ts`"*. Màn hình thứ
 * hai là tệp này, nên cả hai bản đã gộp về `goiGhi`. Thứ CÒN LẠI ở đây là phần duy nhất của riêng
 * danh bạ: đọc thân thành `identity_canBoTomTat`.
 *
 * MÓN NỢ THỨ HAI CŨNG ĐÃ TRẢ, 24/09/2026: phép đọc thân từng nằm nguyên văn ở đây, và đó là bản
 * thứ MƯỜI MỘT của cùng một hàm trong `lib/api/` — năm tệp khác từng trỏ thẳng vào hàm này trong
 * chú thích của chúng, coi nó là bản gốc. Cả mười ba nay ở `goi.ts` (`docThanKetQua` ·
 * `docThanLoiGoi`), cạnh lý do vì sao `goiGhi` trả `Response` thô. Thứ CÒN LẠI ở đây là đúng một
 * việc: buộc kiểu trả về vào `identity_canBoTomTat` cho cả bốn tuyến ghi của danh bạ, để không chỗ
 * gọi nào tự khai kiểu.
 */
function goiGhiCanBo(
  duongDan: string,
  phuongThuc: "POST" | "PATCH" | "PUT" | "DELETE",
  than: unknown | undefined,
  maMongDoi: number,
  headerThem?: Readonly<Record<string, string>>,
): Promise<KetQua<identity_canBoTomTat>> {
  return docThanLoiGoi<identity_canBoTomTat>(
    goiGhi(duongDan, phuongThuc, than, maMongDoi, headerThem),
  );
}

/** Đường dẫn của một cán bộ cụ thể. `encodeURIComponent` vì id đi vào ĐƯỜNG DẪN, không vào thân. */
function duongDanMotCanBo(mau: string, id: string): string {
  return mau.replace("{id}", encodeURIComponent(id));
}

/**
 * POST /api/v1/staff — thêm một dòng danh bạ. 201, trả về dòng vừa tạo.
 *
 * KHÔNG CÓ `code` TRONG THÂN, VÀ KHÔNG CÓ CHỖ NÀO ĐỂ TRUYỀN VÀO. Câu mở #15 chốt 22/09/2026: mã
 * cán bộ do hệ thống sinh (`domain.SinhMaCanBo`), vì mã đã cấp thì không bao giờ cấp lại kể cả
 * sau xoá mềm (luật 7, bất biến 3) — một mã do client chọn là một mã có thể chỉ vào hồ sơ lưu
 * trữ của người khác. `identity_themCanBoVao` sinh từ hợp đồng không có trường ấy, nên form
 * không vẽ ô Mã được kể cả khi ai đó muốn: `tsc` đỏ ngay.
 *
 * KHÔNG CÓ `role_id` VÀ KHÔNG CÓ MẬT KHẨU. Gán vai trò là `PUT .../role` — tuyến mang hai ràng
 * buộc của #14; một `role_id` ở đây là đường vòng qua cả hai. Cấp tài khoản đăng nhập là luồng
 * khác (#9 — mật khẩu tạm, bắt đổi lần đầu), và máy chủ ghi `co_tai_khoan = false` làm hằng, nên
 * dòng vừa thêm là một dòng DANH BẠ chứ chưa phải một tài khoản.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ. Hợp đồng đòi `Idempotency-Key` bắt buộc trên
 * tuyến này. Sinh khoá bên trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới — tức
 * đúng cái khoá chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ đã tới máy chủ và đã tạo
 * người. Khoá do biểu mẫu giữ, sống bằng đời một lần mở form, và lần bấm lại dùng lại chính nó.
 */
export function themCanBo(
  than: identity_themCanBoVao,
  khoaChongTrung: string,
): Promise<KetQua<identity_canBoTomTat>> {
  const duongDan: identity_post_staff["duongDan"] = "/api/v1/staff";

  // DỰNG TỪNG TRƯỜNG, KHÔNG `...than`. Một phép trải ở đây là đường để một trường lạ — `code`,
  // `role_id`, `active` — đi lên máy chủ vào ngày ai đó truyền vào một dòng vừa đọc được.
  const thanGui: identity_themCanBoVao = {
    full_name: than.full_name,
    position: than.position,
    email: than.email,
    org_unit_id: than.org_unit_id,
    office_phone: than.office_phone,
    mobile: than.mobile,
  };

  return goiGhiCanBo(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung });
}

/**
 * PATCH /api/v1/staff/{id} — sửa hồ sơ. 200, trả về dòng sau khi sửa.
 *
 * SÁU TRƯỜNG, KHÔNG MỘT TRƯỜNG NÀO ĐỔI THẨM QUYỀN. Vai trò và trạng thái khoá không có mặt ở
 * đây vì chúng có tuyến riêng mang phép chặn riêng; một `role_id` lọt vào thân này là lối đi
 * vòng qua cả #13 lẫn #14.
 *
 * `null` NGHĨA LÀ "KHÔNG ĐỔI", không phải "xoá trắng". Máy chủ đọc sáu con trỏ và bỏ qua con trỏ
 * `nil` (`suaCanBoVao`), nên một màn hình chỉ sửa chức danh gửi `null` cho năm trường còn lại.
 * Chuỗi rỗng thì KHÁC HẲN: nó là "xoá nội dung ô này", một việc hợp lệ với chức danh, bộ phận và
 * hai số điện thoại. Lẫn hai thứ ấy là xoá trắng hai số điện thoại của một danh bạ công vụ mà
 * không ai báo gì.
 */
export function suaCanBo(
  id: string,
  than: identity_suaCanBoVao,
): Promise<KetQua<identity_canBoTomTat>> {
  const mau: identity_patch_staff_by_id["duongDan"] = "/api/v1/staff/{id}";

  const thanGui: identity_suaCanBoVao = {
    full_name: than.full_name,
    position: than.position,
    email: than.email,
    org_unit_id: than.org_unit_id,
    office_phone: than.office_phone,
    mobile: than.mobile,
  };

  return goiGhiCanBo(duongDanMotCanBo(mau, id), "PATCH", thanGui, 200);
}

/**
 * Khoá (`POST`) hoặc mở khoá (`DELETE`) tài khoản một cán bộ. 200 ở CẢ HAI chiều.
 *
 * MỘT HÀM CHO HAI CHIỀU, đúng như máy chủ có một `datKhoaCanBo` cho hai tuyến: hai bản sao là hai
 * chỗ để phép chặn #13 bị sửa ra khỏi một bản.
 *
 * KHOÁ KHÔNG PHẢI XOÁ (#10). Người bị khoá VẪN CÒN trong danh bạ, vẫn hiện trên mọi hồ sơ cũ, chỉ
 * là không đăng nhập được nữa — đó là điều xảy ra khi một cán bộ nghỉ hưu hay chuyển công tác.
 * Xoá mềm một dòng nhập trùng là việc khác, và tuyến cho nó chưa tồn tại (xem đầu tệp).
 */
export function datKhoaCanBo(id: string, khoa: boolean): Promise<KetQua<identity_canBoTomTat>> {
  const mauKhoa: identity_post_staff_by_id_lockout["duongDan"] = "/api/v1/staff/{id}/lockout";
  const mauMo: identity_delete_staff_by_id_lockout["duongDan"] = "/api/v1/staff/{id}/lockout";

  return goiGhiCanBo(
    duongDanMotCanBo(khoa ? mauKhoa : mauMo, id),
    khoa ? "POST" : "DELETE",
    undefined,
    200,
  );
}

/**
 * PUT /api/v1/staff/{id}/role — đổi vai trò. 200, trả về dòng sau khi đổi.
 *
 * `PUT` CHỨ KHÔNG `PATCH`, và thân mang TOÀN BỘ trạng thái của quan hệ chứ không một mệnh lệnh:
 * `role_id: ""` là "không giữ vai trò nào", một đích đến hợp lệ (`nguoi_dung.vai_tro_id` cho
 * NULL, và một người có thể ngồi trong sơ đồ tổ chức mà chưa cầm vai trò nào). Vì vậy hàm này
 * KHÔNG được có một nhánh "rỗng thì thôi không gửi" — nhánh ấy làm việc gỡ vai trò lặng lẽ không
 * xảy ra, mà màn hình vẫn báo đã lưu.
 *
 * ĐÂY LÀ TUYẾN MANG CẢ HAI RÀNG BUỘC CỦA #14 và đường thứ ba của #13 (hạ vai trò người quản trị
 * cuối cùng). Cả ba lần từ chối đều về đây dưới dạng một câu tiếng Việt do máy chủ viết.
 */
export function doiVaiTroCanBo(
  id: string,
  vaiTroID: string,
): Promise<KetQua<identity_canBoTomTat>> {
  const mau: identity_put_staff_by_id_role["duongDan"] = "/api/v1/staff/{id}/role";
  const thanGui: identity_datVaiTroVao = { role_id: vaiTroID };

  return goiGhiCanBo(duongDanMotCanBo(mau, id), "PUT", thanGui, 200);
}
