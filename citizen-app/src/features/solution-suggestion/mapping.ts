/**
 * BA CÂU HỎI, VÀ BẢNG ÁNH XẠ TỪ CÂU TRẢ LỜI SANG DÒNG SẢN PHẨM CÓ THẬT.
 *
 * ⚠ KHÔNG CÓ DANH SÁCH SẢN PHẨM THỨ HAI Ở ĐÂY, VÀ ĐÓ LÀ ĐIỀU KIỆN ĐỂ MÀN NÀY KHÔNG TRÔI.
 *
 *   Bảng dưới chỉ giữ `SolutionId` — những khoá của `SOLUTIONS` trong `content/company-profile.ts`.
 *   Tên sản phẩm, câu headline, câu phụ: tất cả đọc ra từ danh sách ấy lúc vẽ. Chép chúng sang đây
 *   thì ngày ai đó sửa một câu đã công bố, màn này vẫn bày câu cũ — và câu cũ đứng tên một pháp
 *   nhân có thật.
 *
 *   `suggestedSolutions` lọc `SOLUTIONS`, nên một id GÕ SAI ở bảng dưới không đỏ lên ở đâu cả: nó chỉ
 *   lặng lẽ biến mất khỏi kết quả, và người dùng nhận một gợi ý thiếu — hoặc rỗng. Đó là lý do
 *   `solution-suggestion.test.tsx` có một ca khẳng định MỌI id trong bảng này tồn tại trong `SOLUTIONS`.
 *
 * ⚠ CHỈ CÂU HỎI THỨ BA QUYẾT ĐỊNH DANH SÁCH GỢI Ý, VÀ MÀN HÌNH NÓI RA ĐIỀU ĐÓ.
 *
 *   Một bảng "ngành bán lẻ thì hợp với eSMS" là một khẳng định về sự PHÙ HỢP mà không trang nào
 *   của ViHAT công bố, và không ai trong kho này duyệt. Thứ tra được từ nguồn là quan hệ giữa VIỆC
 *   ĐANG CẦN và câu headline đã công bố: "tổng đài đa kênh" trả lời cho gọi ra nhiều, "messaging"
 *   trả lời cho gửi thông báo hàng loạt, "Contact Center tích hợp CRM" trả lời cho quản lý quan hệ
 *   khách hàng.
 *
 *   Hai câu đầu — ngành và quy mô — KHÔNG bị bỏ đi, và cũng không giả vờ lọc: chúng được đọc lại
 *   trong câu tóm tắt ở cuối, thứ người dùng đọc cho người trực hotline nghe. Màn hình viết rõ
 *   điều đó (`WORDS.suggestion_basis`) thay vì để người dùng tưởng ba câu đều đang lọc. Một bộ chọn ba
 *   bước mà hai bước không đổi gì là một bộ chọn nói dối — trừ khi nó tự nói ra.
 */
import { type Solution, type SolutionId, SOLUTIONS } from "../../content/company-profile";

export type IndustryCode = "ban-le" | "tai-chinh" | "giao-duc-y-te" | "san-xuat-dich-vu";
export type ScaleCode = "duoi-10" | "10-50" | "50-200" | "tren-200";
export type TaskCode = "goi-ra" | "da-kenh" | "thong-bao" | "quan-he-khach";

/**
 * Một lựa chọn: khoá nội bộ, và chữ người dùng đọc. Khoá KHÔNG bao giờ hiện ra màn hình.
 * `ma` · `nhan` giữ tên cũ: bộ chọn của bề mặt yêu cầu (`features/yeu-cau`, nửa thương mại, ngoài
 * lượt đổi tên này) dùng chung hình dạng ấy.
 */
export type Option<T extends string> = { ma: T; nhan: string };

export const INDUSTRY_QUESTION = "Bạn đang làm trong lĩnh vực nào?";
export const SCALE_QUESTION = "Doanh nghiệp của bạn có bao nhiêu nhân sự?";
export const TASK_QUESTION = "Việc bạn đang cần làm là gì?";

export const INDUSTRIES: readonly Option<IndustryCode>[] = [
  { ma: "ban-le", nhan: "Bán lẻ, thương mại điện tử" },
  { ma: "tai-chinh", nhan: "Tài chính, bảo hiểm" },
  { ma: "giao-duc-y-te", nhan: "Giáo dục, y tế" },
  { ma: "san-xuat-dich-vu", nhan: "Sản xuất, dịch vụ khác" },
];

/**
 * QUY MÔ ĐO BẰNG SỐ NHÂN SỰ, KHÔNG BẰNG SỐ CUỘC GỌI MỖI NGÀY.
 *
 * Số cuộc gọi là con số người trả lời phải ước lượng — và ước lượng sai thì câu tóm tắt đọc cho
 * người trực hotline nghe cũng sai. Số nhân sự thì ai cũng biết chính xác về doanh nghiệp mình.
 */
export const SCALES: readonly Option<ScaleCode>[] = [
  { ma: "duoi-10", nhan: "Dưới 10 người" },
  { ma: "10-50", nhan: "Từ 10 đến 50 người" },
  { ma: "50-200", nhan: "Từ 50 đến 200 người" },
  { ma: "tren-200", nhan: "Trên 200 người" },
];

export const TASKS: readonly Option<TaskCode>[] = [
  { ma: "goi-ra", nhan: "Gọi ra nhiều" },
  { ma: "da-kenh", nhan: "Chăm sóc khách đa kênh" },
  { ma: "thong-bao", nhan: "Gửi thông báo hàng loạt" },
  { ma: "quan-he-khach", nhan: "Quản lý quan hệ khách hàng" },
];

/**
 * VIỆC ĐANG CẦN → DÒNG SẢN PHẨM. Mỗi dòng dưới đây tra được từ chính câu headline đã công bố:
 *
 *   | Việc | Dòng | Câu đã công bố dựa vào |
 *   |---|---|---|
 *   | Gọi ra nhiều | `voice-ai` | "Tổng đài đa kênh ứng dụng AI hàng đầu Việt Nam" |
 *   | Chăm sóc khách đa kênh | `voice-ai` + `messaging` | câu trên, và "…nâng cao trải nghiệm khách hàng đa kênh" |
 *   | Gửi thông báo hàng loạt | `messaging` | "Giải pháp CPaaS toàn cầu: Messaging & Voice" |
 *   | Quản lý quan hệ khách hàng | `crm` | "Contact Center tích hợp CRM" |
 *
 * `namecard` KHÔNG có mặt trong bảng, và đó là chủ đích: không việc nào trong bốn việc trên là
 * việc của một tấm danh thiếp số. Dòng ấy vào được từ ô menu "Danh thiếp" trên màn chủ — thêm nó
 * vào đây cho đủ bốn dòng là gợi ý một sản phẩm cho một nhu cầu nó không giải quyết.
 */
export const TASK_SOLUTION_MAP: Readonly<Record<TaskCode, readonly SolutionId[]>> = {
  "goi-ra": ["voice-ai"],
  "da-kenh": ["voice-ai", "messaging"],
  "thong-bao": ["messaging"],
  "quan-he-khach": ["crm"],
};

/**
 * Những dòng giải pháp gợi ý cho một việc, THEO ĐÚNG THỨ TỰ `SOLUTIONS` công bố.
 *
 * LỌC `SOLUTIONS`, KHÔNG TRA NGƯỢC TỪNG ID — hai lý do: thứ tự bày ra giống hệt màn Giải pháp, nên
 * người dùng gặp lại đúng thứ tự họ vừa thấy; và một id không còn tồn tại KHÔNG sinh ra một thẻ
 * rỗng, nó chỉ biến mất. Vế thứ hai là thứ khiến ca kiểm "mọi id trong bảng đều có thật" bắt buộc
 * phải tồn tại: không có nó thì lỗi ấy lặng lẽ.
 */
export function suggestedSolutions(task: TaskCode): readonly Solution[] {
  const ids = TASK_SOLUTION_MAP[task];
  return SOLUTIONS.filter((solution) => ids.includes(solution.id));
}

/** Chữ của một lựa chọn, tra từ chính danh sách vẽ ra nó. `null` nếu chưa chọn. */
export function labelOf<T extends string>(
  list: readonly Option<T>[],
  code: T | undefined,
): string | null {
  if (code === undefined) return null;
  return list.find((item) => item.ma === code)?.nhan ?? null;
}
