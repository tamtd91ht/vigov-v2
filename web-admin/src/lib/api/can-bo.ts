/**
 * Gọi chín tuyến của danh bạ cán bộ — ba tuyến đọc và **sáu tuyến ghi**.
 *
 * BA TUYẾN ĐỌC: `GET /api/v1/staff` (một trang, lọc theo `unit` / `published` trên URL),
 * `GET /api/v1/staff/{id}`, và `POST /api/v1/staff/searches` — tìm theo chữ, CHỮ ĐI TRONG THÂN
 * (xem `timCanBo`).
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `page_Result_identity_canBoTomTat` và
 * `identity_canBoTomTat` đến từ `schema.gen.ts`. Không tệp nào trong ứng dụng này mô tả lại
 * mười ba trường của một cán bộ — có bản thứ hai là có hai bản sẽ trôi (luật 9).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * SÁU TUYẾN GHI, KHÔNG PHẢI MỘT — và sự tách ấy là của máy chủ, tệp này chỉ theo đúng nó
 * (`service-identity/internal/http/can_bo_ghi.go`, chú thích đầu tệp):
 *
 *   POST   /api/v1/staff                   thêm một dòng danh bạ
 *   PATCH  /api/v1/staff/{id}              sửa hồ sơ — KHÔNG đổi thẩm quyền của ai
 *   POST   /api/v1/staff/{id}/lockout      nghỉ hưu / chuyển công tác (#10)
 *   DELETE /api/v1/staff/{id}/lockout      quay lại làm việc
 *   PUT    /api/v1/staff/{id}/role         chuyển vai trò (#13, #14)
 *   PUT    /api/v1/staff/{id}/publication  công khai / rút khỏi danh bạ Mini App (#12), `content.update`
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

import { docJSON, docThanLoiGoi, goiGhi, thamSoTheoHopDong, type KetQua } from "./goi";
import type {
  identity_canBoTomTat,
  identity_datCongKhaiVao,
  identity_datVaiTroVao,
  identity_delete_staff_by_id_lockout,
  identity_get_staff,
  identity_get_staff_by_id,
  identity_patch_staff_by_id,
  identity_post_staff,
  identity_post_staff_by_id_lockout,
  identity_post_staff_searches,
  identity_put_staff_by_id_publication,
  identity_put_staff_by_id_role,
  identity_suaCanBoVao,
  identity_themCanBoVao,
  identity_timCanBoVao,
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
export type ThamSoTrang = LocCanBo & {
  limit?: number;
  sort?: KhoaSapXep;
  order?: ChieuSapXep;
  cursor?: string | null;
};

/**
 * Hai bộ lọc của danh bạ — MỘT bộ cho cả `GET /api/v1/staff` lẫn `POST /api/v1/staff/searches`,
 * đúng như máy chủ trao cả hai tuyến cho một `domain.LocCanBo`. Các bộ lọc có mặt kết hợp theo AND.
 *
 * Hai trường này KHÔNG phải dữ liệu cá nhân (một id bộ phận và một giá trị đúng/sai), nên chúng
 * được đi trên URL. Chữ tìm thì không — xem `timCanBo`.
 */
export type LocCanBo = {
  /** `unit` — id bộ phận. Rỗng hoặc vắng = mọi khối, kể cả người chưa thuộc khối nào. */
  boPhan?: string;
  /**
   * `published`. `null` hoặc vắng = cả hai. `false` là một bộ lọc THẬT ("chưa hiện trên Mini
   * App"), không phải "không lọc" — gộp hai nghĩa ấy làm một là ô "Chưa hiện" trả về cả xã.
   */
  congKhai?: boolean | null;
};

/**
 * Dựng đường dẫn truy vấn. Tách riêng khỏi lời gọi mạng để test được mà không cần thay `fetch`.
 *
 * MỌI TÊN THAM SỐ ĐI QUA `thamSoTheoHopDong<identity_get_staff["truyVan"]>`: máy chủ đổi tên
 * `unit` hay `published` thì `tsc` đỏ ở đúng dòng dưới đây, thay vì máy chủ bỏ qua một tham số lạ
 * và trả cả danh bạ trong khi cán bộ tin mình đang xem một khối.
 *
 * Tham số nào không đặt thì KHÔNG xuất hiện trong URL — để máy chủ áp mặc định của nó
 * (`limit=20`, `sort=code`, `order=asc`), thay vì ứng dụng web giữ một bản sao thứ hai của các
 * mặc định ấy và trôi khỏi bản của máy chủ.
 *
 * KHÔNG CÓ `q` VÀ KHÔNG CÓ CHỖ ĐỂ TRUYỀN NÓ: `ThamSoTrang` không có trường chữ tìm, và hợp đồng của
 * tuyến này cũng không có (`identity_get_staff["truyVan"]`). Chữ tìm thường là họ tên hoặc số điện
 * thoại; trên URL nó đi vào log truy cập và lịch sử trình duyệt (luật 3, cấm #4).
 */
export function duongDanDanhSachCanBo(thamSo: ThamSoTrang = {}): string {
  const duongDan: identity_get_staff["duongDan"] = "/api/v1/staff";
  const truyVan = new URLSearchParams();
  const dat = thamSoTheoHopDong<TruyVanDanhSach>(truyVan);

  dat("limit", thamSo.limit);
  dat("sort", thamSo.sort);
  dat("order", thamSo.order);
  dat("unit", thamSo.boPhan?.trim());
  // Máy chủ nhận ĐÚNG hai chữ `true` / `false` (`can_bo.go`, không dùng ParseBool). `null` thì không
  // gửi — cả hai.
  dat("published", chuCongKhai(thamSo.congKhai));
  // Con trỏ rỗng nghĩa là trang đầu — `dat` bỏ qua nó, vì `cursor=` rỗng là 400 "con trỏ không hợp
  // lệ" đúng vào lần mở màn hình đầu tiên.
  dat("cursor", thamSo.cursor);

  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

function chuCongKhai(congKhai: boolean | null | undefined): "true" | "false" | undefined {
  if (congKhai === true) return "true";
  if (congKhai === false) return "false";
  return undefined;
}

/** GET /api/v1/staff — một trang của danh bạ. Máy chủ đòi quyền `admin.user`; 403 nếu thiếu. */
export function layDanhSachCanBo(
  thamSo: ThamSoTrang = {},
): Promise<KetQua<page_Result_identity_canBoTomTat>> {
  return docJSON<page_Result_identity_canBoTomTat>(duongDanDanhSachCanBo(thamSo));
}

/* ---- tìm theo chữ --------------------------------------------------------------------------- */

/**
 * Giới hạn chữ tìm, tính bằng KÝ TỰ — cùng con số với `domain.TuKhoaTimCanBoToiDa`
 * (`service-identity/internal/domain/tim_can_bo.go`).
 *
 * KÝ TỰ, KHÔNG BYTE VÀ KHÔNG ĐƠN VỊ UTF-16. Một chữ Việt có dấu là hai hoặc ba byte ("ễ" là ba),
 * nên giới hạn theo byte từ chối một họ tên chừng bảy mươi chữ; còn `String.length` đếm đơn vị
 * UTF-16, nên một ký tự ngoài mặt phẳng cơ bản bị đếm là hai. Máy chủ đếm rune (điểm mã) —
 * `Array.from` đếm đúng thứ ấy.
 *
 * VÌ SAO CÓ BẢN THỨ HAI Ở CLIENT: để từ chối ngay tại ô nhập, trước khi chữ rời trình duyệt, chứ
 * không để chuyển cả chuỗi lên rồi nhận 400. Máy chủ VẪN là nơi quyết định — hai con số lệch nhau
 * thì câu của máy chủ hiện nguyên văn.
 */
export const TU_KHOA_TIM_TOI_DA = 200;

/** Chữ tìm đã chuẩn hoá và hợp lệ. Chỉ `chuanHoaTuKhoaTim` dựng ra được một giá trị kiểu này. */
export type TuKhoaHopLe = { readonly loai: "hopLe"; readonly tu: string };

export type TuKhoaTim = { readonly loai: "rong" } | { readonly loai: "quaDai" } | TuKhoaHopLe;

/**
 * Chuẩn hoá chữ tìm ĐÚNG như máy chủ làm (`domain.ChuanHoaTuKhoaTimCanBo`): cắt hai đầu, gộp mọi
 * dãy khoảng trắng bên trong thành một dấu cách, rồi đếm ký tự.
 *
 * GỘP KHOẢNG TRẮNG vì họ tên được lưu đã gộp: "Nguyễn  Văn" (hai dấu cách, dán từ Excel) phải được
 * tìm như "Nguyễn Văn", không thì không ra ai.
 *
 * `rong` KHÔNG PHẢI LỖI: ô tìm rỗng là "bỏ tìm, xem cả danh sách" — tức `GET /api/v1/staff`.
 */
export function chuanHoaTuKhoaTim(tho: string): TuKhoaTim {
  const tu = tho.split(/\s+/u).filter((phan) => phan !== "").join(" ");
  if (tu === "") return { loai: "rong" };
  if (Array.from(tu).length > TU_KHOA_TIM_TOI_DA) return { loai: "quaDai" };
  return { loai: "hopLe", tu };
}

/**
 * Thân của một lần tìm — dựng TỪNG TRƯỜNG theo `identity_timCanBoVao`.
 *
 * ĐỦ NĂM TRƯỜNG, KỂ CẢ KHI RỖNG, vì kiểu sinh từ hợp đồng đòi cả năm, và mỗi giá trị rỗng có nghĩa
 * đã khai ở máy chủ (`can_bo_tim.go`): `unit: ""` = mọi khối, `published: null` = cả hai,
 * `limit: null` = mặc định của máy chủ, `cursor: ""` = trang đầu.
 *
 * KHÔNG CÓ `sort` / `order`: tìm kiếm luôn phân trang theo thứ tự mặc định của danh bạ (mã, tăng
 * dần). Hợp đồng không có hai trường ấy, nên `tsc` đỏ nếu ai thêm vào.
 */
export function thanTimCanBo(
  tuKhoa: TuKhoaHopLe,
  thamSo: LocCanBo & { limit?: number; cursor?: string | null } = {},
): identity_timCanBoVao {
  return {
    q: tuKhoa.tu,
    unit: thamSo.boPhan?.trim() ?? "",
    published: thamSo.congKhai ?? null,
    limit: thamSo.limit ?? null,
    cursor: thamSo.cursor ?? "",
  };
}

/**
 * POST /api/v1/staff/searches — một trang kết quả tìm. 200, cùng hình dạng trang với
 * `GET /api/v1/staff`. Quyền `admin.user`, như danh sách.
 *
 * `POST` CHO MỘT PHÉP ĐỌC, VÀ ĐÓ LÀ CẢ LÝ DO TUYẾN NÀY TỒN TẠI: chữ tìm thường là họ tên hoặc số
 * điện thoại của một người. Trên URL, nó đi vào log truy cập của mọi proxy trên đường, lịch sử
 * trình duyệt của một máy dùng chung ở bộ phận một cửa, và mọi công cụ theo dõi ghi URL (luật 3,
 * cấm #4). Vì vậy đường dẫn dưới đây là một HẰNG — không có `?`, không có chỗ ghép thêm gì — và chữ
 * tìm chỉ đi trong thân. Con trỏ của trang sau cũng đi trong thân, nên đi tiếp các trang của một
 * lần tìm không bao giờ đưa chữ tìm lên URL.
 *
 * NHẬN `TuKhoaHopLe`, KHÔNG NHẬN `string`: chỉ `chuanHoaTuKhoaTim` dựng ra được kiểu ấy, nên không
 * chỗ gọi nào gửi được một chữ chưa cắt, rỗng, hay quá dài — `tsc` chặn trước khi máy chủ phải chặn.
 *
 * KHÔNG CẦN `Idempotency-Key`: lần gọi này không tạo gì, gửi lại hai lần là đọc hai lần.
 *
 * KHÔNG GHI LOG GÌ, kể cả khi hỏng — thân mang đúng thứ vừa nói ở trên (luật 3, bất biến 1).
 */
export function timCanBo(
  tuKhoa: TuKhoaHopLe,
  thamSo: LocCanBo & { limit?: number; cursor?: string | null } = {},
): Promise<KetQua<page_Result_identity_canBoTomTat>> {
  const duongDan: identity_post_staff_searches["duongDan"] = "/api/v1/staff/searches";
  return docThanLoiGoi<page_Result_identity_canBoTomTat>(
    goiGhi(duongDan, "POST", thanTimCanBo(tuKhoa, thamSo), 200),
  );
}

/**
 * Đọc một trang của danh bạ theo bộ lọc đang áp: có chữ tìm thì `POST .../searches`, không thì
 * `GET /api/v1/staff`. MỘT chỗ quyết định rẽ nhánh, để màn hình không tự viết lại phép ấy.
 *
 * `tuKhoa === null` là "không tìm" — không phải chuỗi rỗng, vì tuyến tìm từ chối chữ rỗng (400):
 * một lần tìm không chữ đã có tuyến của nó, và đó là danh sách.
 */
export function docTrangDanhBa(
  tuKhoa: TuKhoaHopLe | null,
  thamSo: LocCanBo & { cursor?: string | null } = {},
): Promise<KetQua<page_Result_identity_canBoTomTat>> {
  // KHÔNG TRUYỀN `limit`, `sort`, `order`: để máy chủ áp mặc định của chính nó.
  const loc = { boPhan: thamSo.boPhan, congKhai: thamSo.congKhai, cursor: thamSo.cursor };
  return tuKhoa === null ? layDanhSachCanBo(loc) : timCanBo(tuKhoa, loc);
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
  // `has_zalo` là trường TUỲ CHỌN của hợp đồng: vắng = không đổi. Chỉ chép lên khi bên gọi đặt nó
  // (`thanSua` chỉ đặt khi ô "Có Zalo" thật sự đổi), để một lần sửa chức danh không ghi đè cờ Zalo
  // mà người khác vừa đặt.
  if (than.has_zalo !== undefined && than.has_zalo !== null) thanGui.has_zalo = than.has_zalo;

  return goiGhiCanBo(duongDanMotCanBo(mau, id), "PATCH", thanGui, 200);
}

/**
 * Một yêu cầu đổi trạng thái công khai của MỘT người trên danh bạ Zalo Mini App (#12).
 *
 * `thuTu` KHÔNG TUỲ CHỌN Ở ĐÂY dù hợp đồng cho `null`: tuyến là PUT, thân mang TOÀN BỘ trạng thái,
 * nên `display_order` vắng hoặc `null` nghĩa là XOÁ thứ tự đang đặt (`can_bo_ghi.go`,
 * `datCongKhaiVao`). Buộc bên gọi truyền giá trị là buộc họ nghĩ tới nó — rút một người khỏi danh bạ
 * mà quên truyền thứ tự đang có thì người ấy mất luôn vị trí, và không ai được báo.
 */
export type YeuCauCongKhai = {
  readonly congKhai: boolean;
  /** Người quản trị đã tick "đã hỏi ý và người này đồng ý" CHO LẦN NÀY. */
  readonly daXacNhanDongY: boolean;
  readonly thuTu: number | null;
};

/**
 * Thân `PUT /api/v1/staff/{id}/publication` — dựng TỪNG TRƯỜNG theo `identity_datCongKhaiVao`.
 *
 * `consent_confirmed` CHỈ `true` KHI VỪA CÔNG KHAI VỪA ĐÃ TICK. Rút khỏi danh bạ luôn gửi `false`:
 * máy chủ bỏ qua trường ấy khi `published` là `false`, nhưng một thân nói "đã xác nhận đồng ý" cho
 * một lần rút là một dòng kiểm toán nói điều không ai làm.
 *
 * `published` LUÔN CÓ MẶT: máy chủ từ chối thân thiếu nó (400) thay vì đọc là `false`, vì rút nhầm
 * là xoá dấu đồng ý — và người ấy phải được hỏi lại từ đầu.
 */
export function thanCongKhai(yc: YeuCauCongKhai): identity_datCongKhaiVao {
  return {
    published: yc.congKhai,
    consent_confirmed: yc.congKhai && yc.daXacNhanDongY,
    display_order: yc.thuTu,
  };
}

/**
 * PUT /api/v1/staff/{id}/publication — công khai hoặc rút một cán bộ khỏi danh bạ Zalo Mini App.
 * 200, trả về dòng danh bạ sau khi đổi (kèm `consent_recorded_at`). Quyền `content.update`.
 *
 * MỘT NGƯỜI MỘT LẦN, KHÔNG CÓ BIẾN THỂ HÀNG LOẠT — câu mở #12 do khách chốt: công khai số di động
 * cá nhân phải hỏi ý TỪNG người. Một hàm nhận danh sách id ở đây là đường vòng qua quyết định ấy.
 *
 * Ba lần từ chối về tới giao diện NGUYÊN VĂN câu máy chủ: 400 `consent_required` (chưa xác nhận
 * đồng ý), 400 `invalid_request` (thứ tự âm, thân hỏng), 404 `staff_not_found`.
 */
export function datCongKhaiCanBo(
  id: string,
  yc: YeuCauCongKhai,
): Promise<KetQua<identity_canBoTomTat>> {
  const mau: identity_put_staff_by_id_publication["duongDan"] = "/api/v1/staff/{id}/publication";
  return goiGhiCanBo(duongDanMotCanBo(mau, id), "PUT", thanCongKhai(yc), 200);
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
