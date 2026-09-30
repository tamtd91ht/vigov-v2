/**
 * Đọc tham số mà Mini App được mở kèm theo — **chỉ từ `location.search`, không dùng SDK**.
 *
 * VÌ SAO KHÔNG NHẬP `zmp-sdk`, VÀ ĐÂY LÀ PHÉP ĐO CHỨ KHÔNG PHẢI SUY ĐOÁN:
 *
 *   Đo ngày 18/09/2026 trên bản thử nghiệm Version 6, mở nguội qua deep link: `location.search`
 *   mang **đúng** những gì `getRouteParams()` mang — cả tham số nền tảng (`env`, `version`) lẫn
 *   tham số riêng (hồi ấy là `t`, `src`; nay là `d`, `src` — ADR 0047), đứng cạnh nhau trong cùng
 *   một chuỗi truy vấn.
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

/**
 * MỨC TIN THEO NGUỒN — `src` ∈ {qr, zns} (ADR 0045:311, giữ nguyên bởi ADR 0047). Nằm ở đây, không ở
 * lớp khám phá, vì nó quyết định CÓ GỌI MẠNG hay không: một liên kết chuyển tay không được làm app
 * đi hỏi máy chủ về một xã. `features/kham-pha/goi-y.ts` đọc cùng bảng này — một nguồn, không hai.
 */
export const NGUON_CHON_SAN: ReadonlySet<string> = new Set(["qr", "zns"]);

/**
 * Khuôn tên miền, CHỈ ĐỂ CHẶN RÁC Ở GIAO DIỆN — máy chủ mới là bên quyết tên miền ấy có phải của một
 * xã không (`GET /api/v1/communes?host=`). Nhãn chữ thường ASCII, số, gạch nối (không ở đầu/cuối),
 * ít nhất hai nhãn, tổng ≤ 253. Không có cổng, không có đường dẫn, không có `@`: một chuỗi như thế
 * mà lọt vào `?host=` là để máy chủ phân tích một thứ không phải tên miền.
 */
const NHAN_TEN_MIEN = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

export function laTenMien(chuoi: string): boolean {
  if (chuoi.length === 0 || chuoi.length > 253) return false;
  const nhan = chuoi.split(".");
  return nhan.length >= 2 && nhan.every((n) => NHAN_TEN_MIEN.test(n));
}

/**
 * GỢI Ý XÃ TRÊN ĐƯỜNG LIÊN KẾT — `d` (tên miền xã) và `src`, theo quyết định 27/09/2026 (ADR 0047
 * §Trả lời). `t` và `v` đã bỏ: có mặt thì bị lờ đi, không đọc, không báo.
 *
 * `null` — mở như không có tham số — khi: không có `d`; `src` không nằm trong `NGUON_CHON_SAN`
 * (kể cả thiếu `src`: không có mặc định "coi như qr", luật 1 cấm #1); `d` không đúng khuôn tên miền.
 *
 * ⚠ KẾT QUẢ LÀ DỮ LIỆU CLIENT CUNG CẤP. Nó chỉ quyết app có HỎI máy chủ hay không; nó không chọn xã,
 * không vào phiên nếu công dân chưa bấm xác nhận (ADR 0047 điều kiện dừng #4), và không bao giờ được
 * vẽ ra như thể nó là tên xã.
 *
 * Chữ HOA trong `d` được hạ xuống: tên miền không phân biệt hoa thường, nên đây không phải đoán.
 */
export function thamSoXa(ket_qua: KetQuaDo): { ten_mien: string; nguon: string } | null {
  const p = thamSo(ket_qua);
  const nguon = p["src"] ?? "";
  if (!NGUON_CHON_SAN.has(nguon)) return null;
  const ten_mien = (p["d"] ?? "").toLowerCase();
  if (!laTenMien(ten_mien)) return null;
  return { ten_mien, nguon };
}
