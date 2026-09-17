/**
 * Gọi hai tuyến đọc của danh bạ cán bộ: `GET /api/v1/staff` và `GET /api/v1/staff/{id}`.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `page_Result_identity_canBoTomTat` và
 * `identity_canBoTomTat` đến từ `schema.gen.ts`. Không tệp nào trong ứng dụng này mô tả lại
 * mười hai trường của một cán bộ — có bản thứ hai là có hai bản sẽ trôi (luật 9).
 *
 * KHÔNG CÓ TUYẾN GHI Ở ĐÂY, và cũng không có khung để về sau điền vào. Hợp đồng không có tuyến
 * nào tạo, sửa, khoá hay xoá cán bộ: cả bốn đang chờ khách chốt
 * `kb/00-foundation/open-questions.json` #9 (mật khẩu đầu tiên tới tay cán bộ mới bằng cách
 * nào), #10 (nghỉ hưu thì khoá hay xoá), #13 (có chặn việc xã mất người quản trị cuối cùng
 * không) và #14 (`admin.user` có được tác động lên chính tài khoản mình không). Một đường ghi
 * viết dở trông y hệt một quyết định đã có người ra.
 */

import { docJSON, type KetQua } from "./goi";
import type {
  identity_canBoTomTat,
  identity_get_staff,
  identity_get_staff_by_id,
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
