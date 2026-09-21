/**
 * Hai tuyến đọc của màn "Theo dõi giải ngân": `GET /api/v1/investment-projects` và
 * `GET /api/v1/investment-projects/{id}`. Máy chủ đòi `budget.read` trên cả hai.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `finance_danhSachDuAnRa` và `finance_duAnRa` đến từ
 * `schema.gen.ts`. Không tệp nào trong ứng dụng này mô tả lại mười tám trường của một dự án.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * HỢP ĐỒNG CÒN THIẾU HAI THAM SỐ TRUY VẤN CỦA TUYẾN DANH SÁCH, VÀ CHỖ NÀY LÀ NƠI PHẢI NÓI RA.
 *
 * `kb/20-contracts/openapi.json` không khai `parameters` nào cho `GET /api/v1/investment-projects`,
 * trong khi handler Go đọc **`year` (BẮT BUỘC)** và `category` (tuỳ chọn) —
 * `service-finance/internal/http/du_an.go:165,176`. Bộ sinh kiểu có hỗ trợ tham số truy vấn
 * (`scripts/gen-api-types.mjs`, khối "THAM SỐ TRUY VẤN"), và một tuyến khác của hợp đồng —
 * `GET /api/v1/staff` — có đủ bốn tham số của nó. Vậy lỗ hổng nằm ở `tools/apidoc`, không ở đây.
 *
 * HỆ QUẢ ĐO ĐƯỢC: `finance_get_investment_projects["truyVan"]` là `{}` rỗng, nên `tsc` KHÔNG
 * canh được hai tên tham số này. Chúng là chuỗi thường trong tệp này cho tới khi hợp đồng khai
 * chúng — đã báo lên, KHÔNG chép thầm vào một hằng số rồi để đó. Sửa ở `tools/apidoc` rồi
 * `make kb`, không sửa `openapi.json` bằng tay (luật 9, bất biến 8).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ HÀM GHI NÀO, và không có khung để về sau điền vào. Máy chủ cũng không có tuyến ghi:
 * "Nhập giải ngân" và "Ghi nhận khoản chi" đều ghi vào hồ sơ lưu trữ nên cần bản ghi nghiệp vụ
 * và vết kiểm toán trong CÙNG một giao dịch (`du_an.go`, khối đầu tệp).
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
