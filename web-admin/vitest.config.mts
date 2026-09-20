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
