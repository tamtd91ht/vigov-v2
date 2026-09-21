/**
 * Tra một phiếu phản ánh theo **mã tra cứu** — `GET /api/v1/citizen-reports/{maTraCuu}`.
 * Máy chủ đòi `feedback.read`.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `petitions_phieuPhanAnhRa` đến từ `schema.gen.ts`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * PHẢN HỒI ĐÃ CHE SẴN DỮ LIỆU CÁ NHÂN, VÀ MÀN HÌNH KHÔNG "LÀM ĐẸP" LẠI.
 *
 * `reporter_name` về dạng `Nguyễn V. A.`, `reporter_phone` về dạng `09****5678`, và cả hai RỖNG
 * khi người dân gửi ẩn danh — máy chủ che ở đường ra vì hôm nay chưa có khoá quyền nào mở xem
 * đầy đủ (`service-petitions/internal/http/phieu_phan_anh.go`, khối trên `phieuPhanAnhRa`).
 * Không có chỗ nào trong ứng dụng này ghép lại, đoán lại, hay hiện thêm chữ số nào.
 *
 * VÀ KHÔNG CÓ GHI CHÚ NỘI BỘ HAY LỊCH SỬ LUÂN CHUYỂN TRÊN PHẢN HỒI NÀY — đã đối chiếu từng
 * trường của hợp đồng. Nếu có ngày chúng xuất hiện ở đây thì đó là lỗi CỦA HỢP ĐỒNG và phải sửa
 * ở tuyến, chứ không phải chỗ để giao diện lọc bớt đi.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ TUYẾN DANH SÁCH. Hợp đồng không có `GET /api/v1/citizen-reports`, nên màn hình chỉ
 * tra được theo mã đã trả cho người dân — xem `features/phan-anh/tra-cuu-phieu.tsx`.
 */

import { docJSON, type KetQua } from "./goi";
import type {
  petitions_get_citizen_reports_by_maTraCuu,
  petitions_phieuPhanAnhRa,
} from "./schema.gen";

/**
 * GET /api/v1/citizen-reports/{maTraCuu}.
 *
 * MỘT MÃ 404 DUY NHẤT CHO BỐN TÌNH HUỐNG, và giao diện không được dựng lại sự phân biệt ấy: mã
 * không tồn tại · mã của xã khác · phiếu đã xoá mềm · phiếu thuộc lĩnh vực hạn chế (phản ánh về
 * tác phong cán bộ) mà tài khoản thiếu `feedback.restricted`. Máy chủ trả cùng một câu cho cả
 * bốn, có chủ ý: phân biệt được chúng là nói cho người đang thử mã biết họ gần tới đâu, và ở ca
 * thứ tư là nói cho một đồng nghiệp biết có người vừa phản ánh về họ (luật 4, cấm #2).
 *
 * Nên ở đây KHÔNG rẽ nhánh theo `code`, không đổi câu chữ, không thêm gợi ý nào — hiện đúng
 * `message` của máy chủ (`lib/api/goi.ts`).
 */
export function layPhieuPhanAnh(maTraCuu: string): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const thamSo: petitions_get_citizen_reports_by_maTraCuu["thamSo"] = { maTraCuu };
  return docJSON<petitions_phieuPhanAnhRa>(
    // Mã tra cứu là chuỗi người dân cầm trên tay và gõ lại — mã hoá vào đường dẫn, không ghép
    // thẳng, vì một dấu `/` hay khoảng trắng lọt vào sẽ đổi hẳn tuyến được gọi.
    `/api/v1/citizen-reports/${encodeURIComponent(thamSo.maTraCuu)}`,
  );
}
