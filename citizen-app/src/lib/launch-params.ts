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
export async function thamSoMoApp(): Promise<Record<string, string>> {
  try {
    const { getRouteParams } = await import("zmp-sdk");
    return getRouteParams() ?? {};
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
export function batChanDoan(tham_so: Record<string, string>): boolean {
  return "debug" in tham_so;
}
