/**
 * Hai danh mục của xã: bộ phận (`GET /api/v1/org-units`) và vai trò (`GET /api/v1/roles`).
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `identity_danhSachBoPhanRa` và `identity_danhSachVaiTroRa`
 * đến từ `schema.gen.ts`. Không tệp nào ở đây mô tả lại một bộ phận hay một vai trò có những
 * trường gì (luật 9).
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: ba quy tắc
 * ấy đúng cho MỌI tuyến và được nói một lần ở `goi.ts`. Đọc tệp ấy trước khi sửa gì ở đây.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * KHÔNG CÓ BỘ NHỚ ĐỆM Ở TỆP NÀY, VÀ ĐÓ LÀ CHỦ Ý — đọc trước khi "tối ưu".
 *
 * Hai danh mục này là danh sách đóng và nhỏ (khoảng mười bộ phận, tám vai trò), nên một biến
 * ở mức module giữ lại kết quả trông rất hợp lý. Nó không hợp lý: danh mục là của MỘT xã. Một
 * bản đệm sống lâu hơn yêu cầu, trong một tiến trình phục vụ nhiều tên miền, là đúng hình dạng
 * của một lần tên bộ phận của xã này hiện trên màn hình xã khác — một vụ rò dữ liệu giữa hai cơ
 * quan nhà nước, không phải một lỗi hiển thị (luật 1, cấm #1; `skills/load-data-once`, cấm #4).
 *
 * Chỗ đúng để đọc một lần là PHẠM VI MỘT MÀN HÌNH: `docDanhMucDanhBa()` dưới đây đọc cả hai
 * danh mục đúng một lượt khi mở màn hình, rồi màn hình chuyền bảng tra xuống từng dòng. Mỗi
 * dòng tự gọi là mẫu N+1 — ở danh bạ hai mươi dòng thì thành hai mươi lời gọi, và con số ấy
 * chỉ đi lên (`skills/load-data-once`, dạng 1).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { docJSON, type KetQua } from "./goi";
import type {
  identity_danhSachBoPhanRa,
  identity_danhSachVaiTroRa,
  identity_get_org_units,
  identity_get_roles,
} from "./schema.gen";

/**
 * GET /api/v1/org-units — cây bộ phận của xã.
 *
 * KHÔNG PHÂN TRANG, và hợp đồng không có tham số nào để phân trang: tuyến trả NGUYÊN danh
 * sách (`items`). Đó là điều kiện để một bảng tra id → tên là đầy đủ; một danh mục trả về từng
 * trang sẽ cho ra những id "không tra được" chỉ vì chúng nằm ở trang sau.
 *
 * Tuyến là `any-authenticated`: mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ đọc được, vì tên bộ
 * phận xuất hiện ở ô phân công, luồng văn bản và mọi bộ lọc. Không chéo xã — máy chủ buộc
 * `tenant_id` ở tầng kho.
 */
export function layDanhMucBoPhan(): Promise<KetQua<identity_danhSachBoPhanRa>> {
  const duongDan: identity_get_org_units["duongDan"] = "/api/v1/org-units";
  return docJSON<identity_danhSachBoPhanRa>(duongDan);
}

/** GET /api/v1/roles — danh mục vai trò của xã. Cũng trả nguyên danh sách; xem hàm trên. */
export function layDanhMucVaiTro(): Promise<KetQua<identity_danhSachVaiTroRa>> {
  const duongDan: identity_get_roles["duongDan"] = "/api/v1/roles";
  return docJSON<identity_danhSachVaiTroRa>(duongDan);
}

/** Hai danh mục mà một màn hình danh bạ cần, đọc trong cùng một lượt. */
export type DanhMucDanhBa = {
  boPhan: KetQua<identity_danhSachBoPhanRa>;
  vaiTro: KetQua<identity_danhSachVaiTroRa>;
};

/**
 * Đọc cả hai danh mục — ĐÚNG HAI lời gọi, cho cả màn hình, dù danh bạ có bao nhiêu dòng.
 *
 * HAI KẾT QUẢ RỜI NHAU, KHÔNG GỘP THÀNH MỘT. Danh mục vai trò hỏng không có lý do gì làm cột
 * Bộ phận trống theo: hai cột, hai câu trả lời, và mỗi cột nói đúng chuyện của nó. Gộp lại là
 * biến một sự cố của một tuyến thành hai cột cùng im lặng.
 *
 * `Promise.all` chứ không phải hai lần `await` nối tiếp: hai tuyến không phụ thuộc nhau, nên
 * chờ tuần tự chỉ cộng thêm một vòng mạng vào thời gian mở màn hình. Cả hai hàm đều không ném
 * (`docJSON` gói lỗi vào `KetQua`), nên `Promise.all` ở đây không có nhánh `reject` nào.
 */
export function docDanhMucDanhBa(): Promise<DanhMucDanhBa> {
  return Promise.all([layDanhMucBoPhan(), layDanhMucVaiTro()]).then(([boPhan, vaiTro]) => ({
    boPhan,
    vaiTro,
  }));
}
