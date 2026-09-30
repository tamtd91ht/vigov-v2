/**
 * Hai tuyến đọc của màn "Theo dõi giải ngân": `GET /api/v1/investment-projects` và
 * `GET /api/v1/investment-projects/{id}`. Máy chủ đòi `budget.read` trên cả hai.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `finance_danhSachDuAnRa` và `finance_duAnRa` đến từ
 * `schema.gen.ts`. Không tệp nào trong ứng dụng này mô tả lại mười tám trường của một dự án.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ✔ HAI KHIẾM KHUYẾT KHỐI NÀY TỪNG TẢ ĐỀU ĐÃ ĐÓNG, 24/09/2026. Giữ lại vì chúng là hai bài học
 * khác nhau, không phải vì còn đúng.
 *
 * 1. "Hợp đồng không khai `parameters` nào cho `GET /api/v1/investment-projects`" — ĐÃ SAI.
 *    `tools/apidoc` nay đọc tham số truy vấn thẳng từ AST handler (commit 5772109, mở rộng ở
 *    f5b6c63). Đo lại: hợp đồng khai `year` với `required: true` và `category` với
 *    `required: false`, đúng như `service-finance/internal/http/du_an.go` cư xử.
 *
 * 2. "Máy chủ cũng không có tuyến ghi" — ĐÃ SAI, và sai theo chiều đắt hơn: CHÍN tuyến ghi của
 *    phân hệ này có thật, và màn đã nối hết vào từ 24/09/2026 — xem `lib/api/giai-ngan.ts`.
 *    Tệp NÀY vẫn chỉ giữ hai tuyến ĐỌC, và đó là phân chia có chủ ý chứ không phải thiếu sót.
 *
 * VÌ SAO KHÔNG XOÁ TRẮNG: câu thứ hai là loại nguy hiểm nhất trong kho này — nó nói "thứ ấy
 * không tồn tại", nên người đọc tin nó sẽ đi DỰNG LẠI một thứ đã có, hoặc tệ hơn là kết luận
 * rằng phân hệ này chỉ đọc được. Phiên 23-24/09 gặp đúng lớp ấy sáu lần, và đây là lần thứ sáu.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { docJSON, type KetQua } from "./goi";
import type {
  finance_danhSachDuAnRa,
  finance_duAnRa,
  finance_get_investment_projects,
  finance_get_investment_projects_by_id,
} from "./schema.gen";

/**
 * Lọc danh sách dự án. `nam` KHÔNG tuỳ chọn — máy chủ trả 400 khi thiếu, và nó từ chối lấy năm
 * hiện tại làm mặc định vì một mặc định ở đó quyết định "báo cáo tiền của năm nào" một cách vô
 * hình (`du_an.go:158`). Bắt buộc ở đây để `tsc` chặn lời gọi thiếu năm ngay lúc dựng, thay vì
 * để nó thành một 400 lúc chạy.
 */
export type LocDuAn = {
  nam: number;
  /** Mã hạng mục kế hoạch vốn (ULID). Không đặt thì máy chủ trả mọi hạng mục. */
  hangMucId?: string;
};

/**
 * Dựng đường dẫn truy vấn. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`.
 *
 * KHÔNG CÓ `tenant_id` Ở ĐÂY, và không được thêm: xã suy từ `Host` ở rìa ngoài cùng, còn client
 * tự khai xã là client tự cấp quyền (luật 1, cấm #2).
 */
export function duongDanDanhSachDuAn(loc: LocDuAn): string {
  const duongDan: finance_get_investment_projects["duongDan"] = "/api/v1/investment-projects";
  const truyVan = new URLSearchParams();

  truyVan.set("year", String(loc.nam));
  // Chuỗi rỗng KHÔNG được gửi: máy chủ đọc `category` bằng `Query().Get`, nên `category=` rỗng
  // đi vào bộ lọc như một mã hạng mục rỗng thay vì như "mọi hạng mục".
  if (loc.hangMucId !== undefined && loc.hangMucId !== "") {
    truyVan.set("category", loc.hangMucId);
  }

  return `${duongDan}?${truyVan.toString()}`;
}

/**
 * GET /api/v1/investment-projects — dự án của xã trong MỘT năm ngân sách.
 *
 * KHÔNG PHÂN TRANG: tuyến trả cả danh sách hoặc hỏng. Vượt trần thì máy chủ TỪ CHỐI thay vì cắt
 * bớt, vì danh sách này được cộng tổng trên màn hình — một danh sách ngắn đi lặng lẽ là một tổng
 * đơn giản là quá nhỏ và trông hoàn toàn bình thường (`du_an.go:179`).
 */
export function layDanhSachDuAn(loc: LocDuAn): Promise<KetQua<finance_danhSachDuAnRa>> {
  return docJSON<finance_danhSachDuAnRa>(duongDanDanhSachDuAn(loc));
}

/**
 * GET /api/v1/investment-projects/{id} — một dự án.
 *
 * `id` mã hoá vào đường dẫn, không ghép thẳng. 404 ở đây là MỘT câu trả lời cho cả "không có dự
 * án ấy" lẫn "dự án của xã khác", và nó như thế vì kho không với tới được dự án của xã khác chứ
 * không vì handler so sánh gì — hai câu trả lời khác nhau sẽ nói cho người gọi biết một bản ghi
 * tồn tại bên trong một cơ quan họ không có việc gì để biết (`du_an.go:221`).
 */
export function layChiTietDuAn(id: string): Promise<KetQua<finance_duAnRa>> {
  const thamSo: finance_get_investment_projects_by_id["thamSo"] = { id };
  return docJSON<finance_duAnRa>(`/api/v1/investment-projects/${encodeURIComponent(thamSo.id)}`);
}
