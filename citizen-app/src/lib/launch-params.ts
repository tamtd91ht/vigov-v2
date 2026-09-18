/**
 * Đọc tham số mà Mini App được mở kèm theo.
 *
 * VÌ SAO TỆP NÀY TỒN TẠI, VÀ VÌ SAO NÓ CHƯA PHẢI GIAI ĐOẠN 2:
 *
 *   ADR 0018 ghi một câu hỏi ở mức chứng cứ THẤP và nó vẫn chưa được trả lời:
 *   *"tham số deep link có tới app trong MỌI đường mở không, và có giữ khi app đang chạy nền
 *   không"*. ADR 0005 dựng cả lớp khám phá lên trên giả định ấy — QR in ở trụ sở xã mang `t`,
 *   `src`, `v`, và app đọc chúng để chọn sẵn xã.
 *
 *   Không ai đo. Tài liệu Zalo render bằng JavaScript nên không đọc trực tiếp được, và một
 *   giả định chưa đo nằm dưới một kiến trúc đã chốt là chỗ đắt nhất để sai.
 *
 *   Nên tệp này KHÔNG chọn xã, KHÔNG gọi backend, KHÔNG lưu gì. Nó chỉ trả về những gì nền
 *   tảng đưa vào, nguyên si, để một màn chẩn đoán in ra trên máy thật.
 *
 * VÌ SAO NHẬP ĐỘNG CHỨ KHÔNG NHẬP TĨNH — đây là ràng buộc thật của SDK, đã đo:
 *
 *   `zmp-sdk` đụng `window` NGAY LÚC NHẬP MODULE. Một dòng `import … from "zmp-sdk"` ở phạm vi
 *   module làm mọi môi trường không phải trình duyệt ném `ReferenceError: window is not defined`
 *   — và nó không giết một hàm, nó giết cả cây module nhập vào nó. Lần đầu thêm, `screens.test`
 *   sập nguyên tệp dù chẳng test nào gọi tới SDK.
 *
 *   Nhập động bên trong hàm thì lỗi ở lại trong hàm, và `catch` bên dưới biến nó thành "không có
 *   tham số nào" — đúng câu trả lời khi app chạy ngoài Zalo.
 *
 * VÌ SAO BỌC TRY/CATCH: `getRouteParams` chỉ có thật khi chạy trong Zalo. Ở `npm run dev`, trong
 * vitest và trong `vite preview` nó vắng mặt hoặc ném lỗi. Một lỗi ở đây sẽ giết cây React trước
 * khi màn hình đầu tiên kịp vẽ — app trắng trơn vì một hàm chẩn đoán. Không đáng.
 *
 * GIỚI HẠN ĐÃ BIẾT: `@minimumVersion 2.11.0`. Máy có Zalo cũ hơn không có hàm này, và nhánh
 * catch là thứ giữ cho app vẫn chạy ở đó.
 */
export type KetQuaDo = {
  /** Theo SDK của Zalo. */
  sdk: Record<string, string>;
  /** Theo `location.search` của chính trang. */
  url: Record<string, string>;
};

/**
 * ĐO HAI NGUỒN, KHÔNG MỘT — và câu hỏi thứ hai đáng tiền hơn câu thứ nhất.
 *
 * `zmp-sdk` tốn **256 kB thô / 64 kB gzip** trong bundle vì nó kéo theo `zod`; đo được bằng cách
 * dựng hai lần, có và không có nó: 247,83 kB so với 504,34 kB. Tức nhập SDK chỉ để đọc tham số
 * làm app **tăng gấp đôi**.
 *
 * Nếu `location.search` mang đúng những tham số ấy thì cả lớp khám phá của ADR 0005 chạy được
 * mà không cần SDK, và 64 kB kia là tiền không phải trả — trên mạng di động của một người dùng
 * ở xã. Chưa ai biết, vì chưa ai đo. Nên bảng chẩn đoán in **cả hai** và để máy thật trả lời.
 */
export async function thamSoMoApp(): Promise<KetQuaDo> {
  return { sdk: await theoSdk(), url: theoUrl() };
}

async function theoSdk(): Promise<Record<string, string>> {
  try {
    const { getRouteParams } = await import("zmp-sdk");
    return getRouteParams() ?? {};
  } catch {
    return {};
  }
}

function theoUrl(): Record<string, string> {
  try {
    return Object.fromEntries(new URLSearchParams(window.location.search));
  } catch {
    return {};
  }
}

/**
 * Màn chẩn đoán chỉ bật khi mở kèm `debug`.
 *
 * Cổng này có chủ đích: người dân và người duyệt của Zalo KHÔNG bao giờ thấy nó. Một bảng kỹ
 * thuật hiện ra trong app của một đơn vị đang xin duyệt là thứ người duyệt sẽ hỏi, và câu trả
 * lời "đó là công cụ nội bộ" không giúp được gì ở vòng đó.
 */
export function batChanDoan(ket_qua: KetQuaDo): boolean {
  return "debug" in ket_qua.sdk || "debug" in ket_qua.url;
}
