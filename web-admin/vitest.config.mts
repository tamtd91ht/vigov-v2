import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

/**
 * VÌ SAO CHỈ VITEST, KHÔNG JSDOM / TESTING-LIBRARY / PLAYWRIGHT Ở GIAI ĐOẠN NÀY:
 *
 *   Ba điều phải được giữ bằng test — một thông báo duy nhất cho mọi ca đăng nhập sai, client
 *   không đụng cookie, không có `tenant_id` trong bất kỳ yêu cầu nào — đều là tính chất của
 *   **lời gọi HTTP** và của **mã nguồn**, không phải của việc dựng DOM. Chúng kiểm được đầy đủ
 *   trong môi trường Node với `fetch` bị thay, và kiểm ở đó thì nhanh, không giòn, và hỏng đúng
 *   chỗ. Dựng thêm jsdom + testing-library lúc này là thêm bốn phụ thuộc để kiểm lại cùng một
 *   thứ qua một lớp mô phỏng trình duyệt.
 *
 *   Playwright sẽ cần — nhưng chỉ cần được khi có route trả cấu hình xã theo `Host`, vì trước
 *   đó ứng dụng chưa dựng nổi một trang thật để bấm vào (xem `lib/tenant.server.ts`).
 */
export default defineConfig({
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  test: {
    environment: "node",
    // MÚI GIỜ CỦA MÁY CHẠY TEST BỊ GHIM, VÀ ĐÓ LÀ MỘT PHÉP KIỂM CHỨ KHÔNG PHẢI MỘT TUỲ CHỌN.
    //
    // Đo 23/09/2026: xoá `timeZone: MUI_GIO` khỏi một phép định dạng mốc thời gian thì bốn ca đỏ
    // dưới `TZ=UTC` — nhưng trên máy trạm ở Việt Nam (+07) thì XANH HẾT, vì đầu ra ngẫu nhiên
    // trùng nhau. Tức một khiếm khuyết thật đi lọt qua mọi máy của người viết mã và chỉ lộ ra ở
    // CI, hoặc tệ hơn là không lộ ra ở đâu cả.
    //
    // Ghim UTC làm hai việc cùng lúc: mọi máy chạy ra cùng một kết quả, VÀ mọi lần ai đó bỏ quên
    // `timeZone` trong một phép định dạng đều đỏ ngay trên máy của chính người ấy.
    //
    // ĐẶT UTC CHỨ KHÔNG ĐẶT `Asia/Ho_Chi_Minh`: ghim vào +07 thì một phép định dạng thiếu
    // `timeZone` vẫn cho ra đúng con số, tức vẫn xanh, tức cái ghim không kiểm gì. Múi giờ ghim
    // phải KHÁC múi giờ nghiệp vụ thì nó mới nói được điều gì.
    //
    // Toàn bộ 883 ca đã chạy xanh dưới `TZ=UTC` trước khi dòng này được thêm, nên nó không sửa
    // một ca nào — nó chỉ chặn đường quay lại.
    env: { TZ: "UTC" },
    // `.tsx` IS COLLECTED, AND THE REASON IS A MEASURED HOLE RATHER THAN A PREFERENCE.
    //
    // While the Danh mục tab was being built, one mutation was run and NOT caught: blanking
    // `{nhanNhomRong(nhom.nhan)}` — the single most important sentence on that screen, the one
    // that tells a commune its catalogue is empty rather than broken — left all 167 tests green
    // and `tsc` clean. Every test guarded a DECISION in a pure module; nothing guarded that any
    // component ever put the decision on the page.
    //
    // This does NOT reverse the argument below. Closing it needs no jsdom, no testing-library
    // and no new dependency: `react-dom/server` renders a presentational component to a string
    // in plain Node, and a string is enough to ask "is the sentence there". What stays out of
    // scope is anything needing a browser — events, focus, layout — which is what the paragraph
    // below is actually about.
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
  },
});
