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
    include: ["src/**/*.test.ts"],
  },
});
