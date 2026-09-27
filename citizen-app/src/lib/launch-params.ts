/**
 * Đọc tham số mà Mini App được mở kèm theo — **chỉ từ `location.search`, không dùng SDK**.
 *
 * VÌ SAO KHÔNG NHẬP `zmp-sdk`, VÀ ĐÂY LÀ PHÉP ĐO CHỨ KHÔNG PHẢI SUY ĐOÁN:
 *
 *   Đo ngày 18/09/2026 trên bản thử nghiệm Version 6, mở nguội qua deep link: `location.search`
 *   mang **đúng** những gì `getRouteParams()` mang — cả tham số nền tảng (`env`, `version`) lẫn
 *   tham số riêng (`t`, `src`), đứng cạnh nhau trong cùng một chuỗi truy vấn.
 *
 *   `zmp-sdk` tốn **256 kB thô / 64 kB gzip** vì nó kéo theo `zod` — đo được bằng cách dựng hai
 *   lần, có và không có nó. Nhập SDK **chỉ để đọc tham số** làm app tăng gần gấp đôi, trên mạng
 *   di động của một người dân ở xã. `URLSearchParams` làm đúng việc ấy và tốn không gì.
 *
 * CHẾ ĐỘ HỎNG Ở ĐÂY LÀ CHẾ ĐỘ HỎNG LÀNH — và đó là thứ khiến đánh đổi trên an toàn:
 *
 *   Phép đo mới chạy trên MỘT đường mở. Nếu có một đường mở nào đó mà `location.search` rỗng
 *   trong khi `getRouteParams()` thì không, hậu quả là: **không có tham số ⇒ app giới thiệu bình
 *   thường, không gợi ý xã nào**. Không sai xã, không mất dữ liệu, không lỗi.
 *
 *   Đó đúng là đường mà ADR 0005 bắt buộc phải chạy được (*"app phải chạy đúng khi mở không có
 *   tham số nào"*), và cũng là đường phổ biến nhất từ lần mở thứ hai trở đi: mở từ danh sách app
 *   ghim, tìm trong Zalo, quay lại tuần sau — không đường nào mang tham số.
 *
 *   Đổi lại, nếu giữ SDK để phòng một chế độ hỏng lành, thì **mọi** người dùng trả 64 kB gzip
 *   trong **mọi** lần mở. Đó là cái giá chắc chắn trả cho một rủi ro chưa thấy.
 *
 * VÌ SAO BỌC TRY/CATCH: ngoài trình duyệt — trong vitest, trong lúc dựng — `window` không tồn
 * tại. Một lỗi ở đây sẽ giết cây React trước khi màn hình đầu tiên kịp vẽ, tức app trắng trơn vì
 * một hàm đọc tham số. Không đáng.
 */
export type KetQuaDo = {
  /** Theo `location.search` của chính trang. Nguồn duy nhất — xem chú thích đầu tệp. */
  url: Record<string, string>;
};

/**
 * ĐỒNG BỘ, KHÔNG BẤT ĐỒNG BỘ — và đó là hệ quả trực tiếp của việc bỏ SDK.
 *
 * Bản cũ phải `async` vì `zmp-sdk` chỉ nhập động được trong trình duyệt, nên lớp khám phá chỉ
 * xuất hiện ở lượt vẽ thứ hai: công dân quét QR và thấy màn giới thiệu công ty nhấp nháy một
 * nhịp trước khi màn xác nhận xã hiện ra. `location.search` có sẵn ngay lúc dựng, nên nhịp nhấp
 * nháy ấy biến mất cùng với SDK.
 */
export function thamSoMoApp(): KetQuaDo {
  return { url: theoUrl() };
}

function theoUrl(): Record<string, string> {
  try {
    return Object.fromEntries(new URLSearchParams(window.location.search));
  } catch {
    return {};
  }
}

/** Bảng tham số để dùng. Một nguồn, nên không còn gì phải gộp — xem chú thích đầu tệp. */
export function thamSo(ket_qua: KetQuaDo): Record<string, string> {
  return ket_qua.url;
}
